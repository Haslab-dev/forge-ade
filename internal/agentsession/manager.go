package agentsession

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"

	"github.com/creack/pty"
	"github.com/google/uuid"
	"github.com/hasdev/forge-ade/internal/events"
)

const (
	// outputRingCap bounds the in-memory terminal scrollback kept per session
	// for replay after an app restart ("closing ForgeADE must not erase
	// session history").
	outputRingCap = 512 << 10
	// stopGrace is how long Stop waits after SIGTERM before escalating to
	// SIGKILL — agent CLIs get a chance to clean up their TTY state.
	stopGrace = 3 * time.Second
)

// Manager owns all terminal sessions: their records (metadata + history) and
// their live PTY handles. It is agnostic of what the agent CLI does — output
// is forwarded verbatim on the bus for whatever terminal renderer is mounted.
type Manager struct {
	bus     *events.Bus
	dataDir string
	configs *ConfigStore

	mu      sync.RWMutex
	records map[string]*Session
	handles map[string]*handle

	recordsPath string
}

// handle is the live process side of a session. Fields are guarded by mu.
type handle struct {
	id      string
	mu      sync.Mutex
	pty     *os.File
	pid     int
	killing bool // user/app-initiated stop in progress → final status terminated
	dirty   bool // output ring changed since the last disk flush
	ring    []byte

	// First-input auto-title capture: raw keystrokes accumulate until the
	// first complete line, which becomes the session title.
	lineBuf   []byte
	titleDone bool
}

// NewManager creates the manager and loads persisted session history.
func NewManager(bus *events.Bus, dataDir string, configs *ConfigStore) *Manager {
	m := &Manager{
		bus:         bus,
		dataDir:     dataDir,
		configs:     configs,
		records:     make(map[string]*Session),
		handles:     make(map[string]*handle),
		recordsPath: filepath.Join(dataDir, "agentsessions", "index.json"),
	}
	m.loadRecords()
	go m.flushLoop()
	return m
}

// ---------------------------------------------------------------------------
// Persistence
// ---------------------------------------------------------------------------

func (m *Manager) loadRecords() {
	raw, err := os.ReadFile(m.recordsPath)
	if err != nil {
		return
	}
	var recs []*Session
	if err := json.Unmarshal(raw, &recs); err != nil {
		log.Printf("agentsession: parse %s: %v", m.recordsPath, err)
		return
	}
	now := time.Now().UnixMilli()
	dirty := false
	for _, r := range recs {
		if r == nil || r.ID == "" {
			continue
		}
		// Processes cannot outlive the app: anything that still claims to be
		// running when history is loaded died with the previous launch.
		if r.Running() {
			r.Status = StatusTerminated
			r.EndedAt = now
			dirty = true
		}
		m.records[r.ID] = r
	}
	if dirty {
		m.persistRecordsLocked()
	}
}

func (m *Manager) persistRecordsLocked() {
	if err := os.MkdirAll(filepath.Dir(m.recordsPath), 0755); err != nil {
		log.Printf("agentsession: mkdir records dir: %v", err)
		return
	}
	recs := make([]*Session, 0, len(m.records))
	for _, r := range m.records {
		recs = append(recs, r)
	}
	sort.Slice(recs, func(i, j int) bool { return recs[i].CreatedAt > recs[j].CreatedAt })
	raw, err := json.MarshalIndent(recs, "", "  ")
	if err != nil {
		return
	}
	if err := os.WriteFile(m.recordsPath, raw, 0644); err != nil {
		log.Printf("agentsession: write records: %v", err)
	}
}

func (m *Manager) outputLogPath(id string) string {
	return filepath.Join(m.dataDir, "agentsessions", "output", id+".log")
}

// flushLoop periodically persists dirty output rings so a crash or force-quit
// still leaves readable history behind.
func (m *Manager) flushLoop() {
	ticker := time.NewTicker(2 * time.Second)
	for range ticker.C {
		m.mu.RLock()
		type pending struct {
			id   string
			ring []byte
		}
		var pend []pending
		for id, h := range m.handles {
			h.mu.Lock()
			if h.dirty {
				h.dirty = false
				pend = append(pend, pending{id, append([]byte{}, h.ring...)})
			}
			h.mu.Unlock()
		}
		m.mu.RUnlock()
		for _, p := range pend {
			m.writeOutputLogFor(p.id, p.ring)
		}
	}
}

// writeOutputLogFor writes a session's ring to its output log atomically.
func (m *Manager) writeOutputLogFor(id string, ring []byte) {
	path := m.outputLogPath(id)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, ring, 0644); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}

// ---------------------------------------------------------------------------
// Listing / lookup
// ---------------------------------------------------------------------------

// List returns all session records, newest first (running sessions first).
func (m *Manager) List() []Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Session, 0, len(m.records))
	for _, r := range m.records {
		out = append(out, *r)
	}
	sort.SliceStable(out, func(i, j int) bool {
		ri, rj := out[i].Running(), out[j].Running()
		if ri != rj {
			return ri
		}
		return out[i].CreatedAt > out[j].CreatedAt
	})
	return out
}

// Get returns a copy of one session record.
func (m *Manager) Get(id string) (Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.records[id]
	if !ok {
		return Session{}, false
	}
	return *r, true
}

// OutputTail returns the session's persisted terminal output (raw VT bytes)
// for replay into a terminal renderer.
func (m *Manager) OutputTail(id string) string {
	m.mu.RLock()
	h := m.handles[id]
	m.mu.RUnlock()
	if h != nil {
		h.mu.Lock()
		ring := string(h.ring)
		h.mu.Unlock()
		if ring != "" {
			return ring
		}
	}
	raw, err := os.ReadFile(m.outputLogPath(id))
	if err != nil {
		return ""
	}
	if len(raw) > outputRingCap {
		raw = raw[len(raw)-outputRingCap:]
	}
	return string(raw)
}

// ---------------------------------------------------------------------------
// Session lifecycle
// ---------------------------------------------------------------------------

// Create spawns a new agent CLI process in its own PTY and records the
// session. workspacePath scopes the session (metadata + default cwd); title
// is the user-facing task name (defaults to the workspace folder name).
func (m *Manager) Create(agentID, workspacePath, title string) (Session, error) {
	cfg, err := m.configs.Get(agentID)
	if err != nil {
		return Session{}, err
	}
	if !cfg.Enabled {
		return Session{}, fmt.Errorf("%s is disabled in Settings → Agent CLIs", cfg.Name)
	}

	rec := &Session{
		ID:            uuid.New().String(),
		AgentID:       cfg.ID,
		AgentName:     cfg.Name,
		Title:         title,
		WorkspacePath: workspacePath,
		Executable:    cfg.Executable,
		Args:          append([]string{}, cfg.Args...),
		CreatedAt:     time.Now().UnixMilli(),
		Status:        StatusStarting,
	}
	if rec.Title == "" {
		rec.Title = folderName(workspacePath)
		rec.TitleAuto = true // first user input may rename the session
	}
	rec.WorkingDirectory = m.resolveWorkingDir(cfg, workspacePath)

	if err := m.spawn(rec, cfg, nil); err != nil {
		rec.Status = StatusFailed
		rec.Error = err.Error()
		rec.EndedAt = time.Now().UnixMilli()
		m.mu.Lock()
		m.records[rec.ID] = rec
		m.persistRecordsLocked()
		m.mu.Unlock()
		m.publishUpdated(*rec)
		return *rec, err
	}
	return *rec, nil
}

// resolveWorkingDir picks the session cwd: explicit per-session choice beats
// per-agent config beats workspace root beats home (never "/" — GUI-launched
// apps start there).
func (m *Manager) resolveWorkingDir(cfg AgentCLIConfig, workspacePath string) string {
	if cfg.WorkingDirectory != "" {
		return cfg.WorkingDirectory
	}
	if workspacePath != "" {
		return workspacePath
	}
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return ""
}

// spawn starts the PTY process for rec (config supplies env) and installs the
// read/reap goroutines. seed, when non-nil, pre-fills the output history (the
// previous generation's terminal content across a restart).
func (m *Manager) spawn(rec *Session, cfg AgentCLIConfig, seed []byte) error {
	exe, err := lookPathEnriched(cfg.Executable)
	if err != nil {
		return fmt.Errorf("executable %q not found — configure its path in Settings → Agent CLIs", cfg.Executable)
	}

	cmd := exec.Command(exe, cfg.Args...)
	cmd.Dir = rec.WorkingDirectory
	cmd.Env = buildEnvironment(cfg.Environment)

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 24, Cols: 80})
	if err != nil {
		return fmt.Errorf("start %s: %w", cfg.Name, err)
	}

	h := &handle{id: rec.ID, pty: ptmx, pid: cmd.Process.Pid}
	if len(seed) > 0 {
		if len(seed) > outputRingCap {
			seed = seed[len(seed)-outputRingCap:]
		}
		h.ring = append(h.ring, seed...) // already persisted — not dirty
	}
	rec.PID = h.pid
	rec.Status = StatusRunning
	rec.StartedAt = time.Now().UnixMilli()
	rec.EndedAt = 0
	rec.ExitCode = nil
	rec.Error = ""

	m.mu.Lock()
	m.records[rec.ID] = rec
	m.handles[rec.ID] = h
	m.persistRecordsLocked()
	m.mu.Unlock()

	m.bus.Publish(events.Event{
		Type: events.TerminalOpened,
		Data: map[string]interface{}{
			"id":       rec.ID,
			"name":     rec.Title,
			"type":     "agent",
			"provider": rec.AgentID,
		},
	})
	m.publishUpdated(*rec)

	go m.readOutput(rec.ID, h)
	go m.reap(rec.ID, cmd, h)
	return nil
}

// resume-hint patterns the CLIs themselves print. Antigravity, for example,
// ends with:  Resume with -c (or command below): agy --conversation=<uuid>
var (
	resumeConversationRe = regexp.MustCompile(`--conversation[ =]([A-Za-z0-9][A-Za-z0-9._-]{7,})`)
	resumeIDRe           = regexp.MustCompile(`--resume[ =]([A-Za-z0-9][A-Za-z0-9._-]{7,})`)
)

// resumeHintFromOutput extracts the latest resume args printed by the CLI, or
// nil. Tolerates chunk splits via the caller-provided carry.
func resumeHintFromOutput(text string) []string {
	if m := resumeConversationRe.FindStringSubmatch(text); m != nil {
		return []string{"--conversation=" + m[1]}
	}
	if m := resumeIDRe.FindStringSubmatch(text); m != nil {
		return []string{"--resume=" + m[1]}
	}
	return nil
}

// readOutput forwards PTY bytes verbatim to the bus and the session ring, and
// watches for the CLI's printed resume command.
func (m *Manager) readOutput(id string, h *handle) {
	decoder := &utf8CarryDecoder{}
	buf := make([]byte, 65536)
	// Sliding window so a resume hint split across PTY reads still matches.
	hintWindow := ""
	for {
		n, err := h.pty.Read(buf)
		if err != nil {
			return // reaper publishes the close path
		}
		if n > 0 {
			decoded := decoder.Decode(buf[:n])
			if decoded != "" {
				h.mu.Lock()
				h.ring = append(h.ring, decoded...)
				if len(h.ring) > outputRingCap {
					h.ring = append([]byte{}, h.ring[len(h.ring)-outputRingCap:]...)
				}
				h.dirty = true
				h.mu.Unlock()
				m.bus.Publish(events.Event{
					Type: events.TerminalOutput,
					Data: map[string]interface{}{"id": id, "data": decoded},
				})
				// Resume-hint watch (best effort, cheap): scan a bounded
				// window that bridges chunk boundaries.
				hintWindow = (hintWindow + decoded)
				if len(hintWindow) > 4096 {
					hintWindow = hintWindow[len(hintWindow)-2048:]
				}
				if hint := resumeHintFromOutput(hintWindow); hint != nil {
					m.applyResumeHint(id, hint)
				}
			}
		}
	}
}

// applyResumeHint stores the CLI's printed resume command on the session.
func (m *Manager) applyResumeHint(id string, hint []string) {
	m.mu.Lock()
	rec := m.records[id]
	if rec == nil || strings.Join(rec.ResumeHint, " ") == strings.Join(hint, " ") {
		m.mu.Unlock()
		return
	}
	rec.ResumeHint = hint
	snapshot := *rec
	m.persistRecordsLocked()
	m.mu.Unlock()
	m.publishUpdated(snapshot)
}

// reap waits for the process to exit and finalizes the session record. A
// generation guard protects against restart races: once a newer process owns
// the session, this reaper must not touch the handle map, the record, or the
// close events.
func (m *Manager) reap(id string, cmd *exec.Cmd, h *handle) {
	err := cmd.Wait()

	h.mu.Lock()
	killing := h.killing
	ring := append([]byte{}, h.ring...)
	h.dirty = false
	h.mu.Unlock()
	_ = h.pty.Close()

	m.mu.Lock()
	if m.handles[id] != h {
		// Restart already replaced this generation — the new owner finalizes
		// everything.
		m.mu.Unlock()
		return
	}
	delete(m.handles, id)
	m.mu.Unlock()

	// Flush the terminal history before the record flips to not-running so
	// "Get(id).Running() == false" implies the output log is already on disk.
	m.writeOutputLogFor(id, ring)

	m.mu.Lock()
	// A restart may have installed a new generation while the log was being
	// written — it owns the record now; skip finalizing and publishing close.
	if cur := m.handles[id]; cur != nil && cur != h {
		m.mu.Unlock()
		return
	}
	rec := m.records[id]
	if rec == nil {
		m.mu.Unlock()
		return
	}
	rec.EndedAt = time.Now().UnixMilli()
	if killing {
		rec.Status = StatusTerminated
	} else {
		rec.Status = StatusExited
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				code := exitErr.ExitCode()
				rec.ExitCode = &code
			}
		} else {
			code := 0
			rec.ExitCode = &code
		}
	}
	snapshot := *rec
	m.persistRecordsLocked()
	m.mu.Unlock()

	m.bus.Publish(events.Event{
		Type: events.TerminalClosed,
		Data: map[string]interface{}{"id": id},
	})
	m.publishUpdated(snapshot)
}

// Write sends raw input (keystrokes, paste) to the session's PTY. The first
// complete line a user types also names the session (when the title is still
// automatic) — see captureTitle.
func (m *Manager) Write(id string, data []byte) (int, error) {
	m.mu.RLock()
	h := m.handles[id]
	rec := m.records[id]
	m.mu.RUnlock()
	if h == nil {
		return 0, fmt.Errorf("session not running: %s", id)
	}
	n, err := h.pty.Write(data)
	if err == nil && rec != nil && rec.TitleAuto {
		m.captureTitle(id, h, data)
	}
	return n, err
}

// captureTitle accumulates raw input until the first complete line and, if it
// looks like a real task (not a CLI command or a password entry), names the
// session after it. Best effort by design — naming must never disturb input.
func (m *Manager) captureTitle(id string, h *handle, data []byte) {
	var line string

	h.mu.Lock()
	if h.titleDone {
		h.mu.Unlock()
		return
	}
	h.lineBuf = append(h.lineBuf, data...)
	if len(h.lineBuf) > 1024 {
		h.lineBuf = h.lineBuf[len(h.lineBuf)-1024:]
	}
	for {
		term := bytes.IndexAny(h.lineBuf, "\r\n")
		if term < 0 {
			h.mu.Unlock()
			return // keep accumulating — no complete line yet
		}
		line = string(h.lineBuf[:term])
		h.lineBuf = h.lineBuf[term+1:]
		if title, ok := deriveTitle(line, tailString(h.ring, 300)); ok {
			h.titleDone = true
			h.lineBuf = nil
			h.mu.Unlock()
			m.applyTitle(id, title)
			return
		}
	}
}

// applyTitle renames the session record and broadcasts the change.
func (m *Manager) applyTitle(id, title string) {
	m.mu.Lock()
	rec := m.records[id]
	if rec == nil || !rec.TitleAuto {
		m.mu.Unlock()
		return
	}
	rec.Title = title
	rec.TitleAuto = false // first input claimed the name — later input never renames
	snapshot := *rec
	m.persistRecordsLocked()
	m.mu.Unlock()
	m.publishUpdated(snapshot)
}

// Resize resizes the session's PTY.
func (m *Manager) Resize(id string, rows, cols uint16) error {
	m.mu.RLock()
	h := m.handles[id]
	m.mu.RUnlock()
	if h == nil {
		return fmt.Errorf("session not running: %s", id)
	}
	return pty.Setsize(h.pty, &pty.Winsize{Rows: rows, Cols: cols})
}

// Stop terminates a running session (SIGTERM, then SIGKILL after a grace
// period). The session record is kept — history survives.
func (m *Manager) Stop(id string) error {
	m.mu.RLock()
	h := m.handles[id]
	m.mu.RUnlock()
	if h == nil {
		return fmt.Errorf("session not running: %s", id)
	}

	h.mu.Lock()
	already := h.killing
	h.killing = true
	pid := h.pid
	h.mu.Unlock()
	if already {
		return nil
	}

	// User-initiated: reflect immediately so the UI flips to Terminated even
	// if the CLI takes the full grace period to die.
	m.mu.Lock()
	if rec := m.records[id]; rec != nil && rec.Running() {
		rec.Status = StatusTerminated
		rec.EndedAt = time.Now().UnixMilli()
		snapshot := *rec
		m.persistRecordsLocked()
		m.mu.Unlock()
		m.publishUpdated(snapshot)
	} else {
		m.mu.Unlock()
	}

	if pid > 0 {
		_ = syscall.Kill(pid, syscall.SIGTERM)
		go func() {
			time.Sleep(stopGrace)
			h.mu.Lock()
			stillKilling := h.killing
			pid := h.pid
			h.mu.Unlock()
			if stillKilling {
				// Still alive after the grace period — hard kill (negative pid
				// targets the whole process group; the PTY child is its leader).
				_ = syscall.Kill(-pid, syscall.SIGKILL)
				_ = syscall.Kill(pid, syscall.SIGKILL)
			}
		}()
	}
	// Closing the master unblocks the read loop.
	_ = h.pty.Close()
	return nil
}

// Restart relaunches the session's agent CLI (same agent config and working
// directory) within the same session record. The CLI receives the config's
// ResumeArgs (e.g. --continue) so the conversation carries over. A running
// process is stopped first, and Restart waits for it to fully exit — the old
// generation's reaper must never touch the new one.
func (m *Manager) Restart(id string) (Session, error) {
	m.mu.Lock()
	rec := m.records[id]
	if rec == nil {
		m.mu.Unlock()
		return Session{}, fmt.Errorf("session not found: %s", id)
	}
	snapshot := *rec
	m.mu.Unlock()

	hadHistory := ""
	if snapshot.Running() {
		// Snapshot the history before stopping: the reaper will overwrite the
		// output log with the final ring; either way we seed the new
		// generation with it so the terminal story continues.
		hadHistory = m.OutputTail(id)
		_ = m.Stop(id)
		// Wait for the old generation's reaper to release the session (bounded
		// by the stop grace window). Without this, the old reaper can delete
		// the NEW handle and flip the record to terminated.
		deadline := time.Now().Add(stopGrace + 2*time.Second)
		for time.Now().Before(deadline) {
			m.mu.RLock()
			h := m.handles[id]
			m.mu.RUnlock()
			if h == nil {
				break
			}
			time.Sleep(40 * time.Millisecond)
		}
	} else {
		hadHistory = m.OutputTail(id)
	}

	cfg, err := m.configs.Get(snapshot.AgentID)
	if err != nil {
		return snapshot, err
	}
	// Restart always relaunches in the directory the session originally ran
	// in — the workspace a session is bound to does not follow later edits to
	// the agent config or the app's active workspace.
	if snapshot.WorkingDirectory != "" {
		cfg.WorkingDirectory = snapshot.WorkingDirectory
	}
	// Note: ResumeArgs need no handling here — configs saved before the field
	// existed adopt the built-in default in the config store's merge.

	m.mu.Lock()
	rec = m.records[id]
	if rec != nil {
		rec.Status = StatusStarting
		rec.Error = ""
	}
	m.mu.Unlock()

	spawnCfg := cfg
	// Resume strategy: the CLI's own printed hint (its exact conversation id)
	// wins; otherwise the config's ResumeArgs; an explicitly empty ResumeArgs
	// disables continuation entirely.
	if len(cfg.ResumeArgs) > 0 && len(snapshot.ResumeHint) > 0 {
		spawnCfg.Args = append(append([]string{}, cfg.Args...), snapshot.ResumeHint...)
	} else if len(cfg.ResumeArgs) > 0 {
		spawnCfg.Args = append(append([]string{}, cfg.Args...), cfg.ResumeArgs...)
	}

	if err := m.spawn(rec, spawnCfg, []byte(hadHistory)); err != nil {
		m.mu.Lock()
		if rec != nil {
			rec.Status = StatusFailed
			rec.Error = err.Error()
			rec.EndedAt = time.Now().UnixMilli()
			snapshot := *rec
			m.persistRecordsLocked()
			m.mu.Unlock()
			m.publishUpdated(snapshot)
			return snapshot, err
		}
		m.mu.Unlock()
		return snapshot, err
	}
	m.mu.RLock()
	out := *m.records[id]
	m.mu.RUnlock()
	return out, nil
}

// Delete removes a session and its output history. A running session is
// stopped first.
func (m *Manager) Delete(id string) error {
	if _, ok := m.Get(id); !ok {
		return fmt.Errorf("session not found: %s", id)
	}
	if _, running := m.runningHandle(id); running {
		_ = m.Stop(id)
		deadline := time.Now().Add(stopGrace)
		for time.Now().Before(deadline) {
			if _, running := m.runningHandle(id); !running {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
	}

	m.mu.Lock()
	delete(m.records, id)
	delete(m.handles, id)
	m.persistRecordsLocked()
	m.mu.Unlock()

	_ = os.Remove(m.outputLogPath(id))
	m.bus.Publish(events.Event{
		Type: events.AgentSessionDeleted,
		Data: map[string]interface{}{"id": id},
	})
	return nil
}

func (m *Manager) runningHandle(id string) (*handle, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	h, ok := m.handles[id]
	return h, ok
}

// Shutdown stops all live sessions and flushes pending output. Called on app
// quit — agent CLI processes cannot outlive ForgeADE.
func (m *Manager) Shutdown() {
	m.mu.RLock()
	ids := make([]string, 0, len(m.handles))
	for id := range m.handles {
		ids = append(ids, id)
	}
	m.mu.RUnlock()
	for _, id := range ids {
		_ = m.Stop(id)
	}
	// Wait (bounded) for the reapers to finalize every handle.
	deadline := time.Now().Add(stopGrace + 2*time.Second)
	for time.Now().Before(deadline) {
		m.mu.RLock()
		n := len(m.handles)
		m.mu.RUnlock()
		if n == 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	m.mu.Lock()
	m.persistRecordsLocked()
	m.mu.Unlock()
}

// publishUpdated broadcasts a session metadata change on the bus.
func (m *Manager) publishUpdated(rec Session) {
	m.bus.Publish(events.Event{
		Type: events.AgentSessionUpdated,
		Data: map[string]interface{}{"session": rec},
	})
}

// ---------------------------------------------------------------------------
// Environment
// ---------------------------------------------------------------------------

// enrichedPath returns a PATH with the common CLI install locations prepended
// so Homebrew/npm/Bun/Cargo/Go-installed agent CLIs resolve when the app is
// launched from Finder (which inherits a minimal PATH).
func enrichedPath() string {
	home, _ := os.UserHomeDir()
	extra := []string{
		"/opt/homebrew/bin",
		"/opt/homebrew/sbin",
		home + "/homebrew/bin",
		home + "/.cargo/bin",
		home + "/go/bin",
		home + "/.bun/bin",
		home + "/.local/bin",
		home + "/node_modules/.bin",
		"/usr/local/bin",
	}
	path := os.Getenv("PATH")
	for _, p := range extra {
		if p != "" && !strings.Contains(path, p) {
			path = p + ":" + path
		}
	}
	return path
}

// lookPathEnriched resolves an executable against the enriched PATH (and
// directly, when it is already an absolute path).
func lookPathEnriched(executable string) (string, error) {
	if strings.Contains(executable, "/") {
		if info, err := os.Stat(executable); err == nil && !info.IsDir() {
			return executable, nil
		}
		return "", fmt.Errorf("not found: %s", executable)
	}
	for _, dir := range filepath.SplitList(enrichedPath()) {
		if dir == "" {
			continue
		}
		candidate := filepath.Join(dir, executable)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() && info.Mode()&0111 != 0 {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("not found in PATH: %s", executable)
}

// buildEnvironment assembles the child environment: the app's environment,
// an enriched PATH, terminal variables, then the agent config's overrides.
func buildEnvironment(extra map[string]string) []string {
	envMap := make(map[string]string)
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}
	envMap["PATH"] = enrichedPath()
	envMap["TERM"] = "xterm-256color"
	envMap["COLORTERM"] = "truecolor"
	envMap["LANG"] = "en_US.UTF-8"
	envMap["LC_ALL"] = "en_US.UTF-8"
	for k, v := range extra {
		if k == "" {
			continue
		}
		if v == "" {
			delete(envMap, k)
			continue
		}
		envMap[k] = v
	}
	envList := make([]string, 0, len(envMap))
	for k, v := range envMap {
		envList = append(envList, k+"="+v)
	}
	return envList
}

// DetectExecutable reports whether an executable resolves on this machine.
func DetectExecutable(executable string) bool {
	_, err := lookPathEnriched(executable)
	return err == nil
}

func folderName(path string) string {
	name := strings.TrimSuffix(strings.TrimRight(path, "/"), "")
	if name == "" {
		return ""
	}
	if idx := strings.LastIndexAny(name, "/\\"); idx >= 0 {
		name = name[idx+1:]
	}
	return name
}

// tailString returns the last n bytes of b as a string.
func tailString(b []byte, n int) string {
	if len(b) > n {
		return string(b[len(b)-n:])
	}
	return string(b)
}

// deriveTitle turns a raw terminal input line into a session title, rejecting
// lines that are not plausibly a task statement: CLI commands (/x, !bash,
// $python), control-key noise, too-short or letterless input, and anything
// typed while a password/passphrase prompt was on screen (secret hygiene —
// keystrokes still flow through Write).
func deriveTitle(line, recentOutput string) (string, bool) {
	line = stripANSI(line)
	line = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 { // control chars, DEL
			return -1
		}
		return r
	}, line)
	line = strings.TrimSpace(line)
	// If the line is or contains a file/screenshot path (e.g. dragged/dropped image), extract the clean filename
	cleanPath := strings.Trim(line, "'\" ")
	if strings.Contains(cleanPath, "/") || strings.Contains(cleanPath, "\\") {
		base := filepath.Base(cleanPath)
		ext := strings.ToLower(filepath.Ext(base))
		if strings.HasPrefix(cleanPath, "/var/") || strings.HasPrefix(cleanPath, "/tmp/") || strings.Contains(cleanPath, "TemporaryItems") || ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".webp" {
			line = strings.TrimSuffix(base, filepath.Ext(base))
		}
	}
	if len([]rune(line)) < 4 {
		return "", false
	}
	hasLetter := false
	for _, r := range line {
		if unicode.IsLetter(r) {
			hasLetter = true
			break
		}
	}
	if !hasLetter {
		return "", false
	}
	if strings.HasPrefix(line, "/") || strings.HasPrefix(line, "!") || strings.HasPrefix(line, "$") {
		return "", false
	}
	lowerRecent := strings.ToLower(recentOutput)
	if strings.Contains(lowerRecent, "password") || strings.Contains(lowerRecent, "passphrase") {
		return "", false
	}

	line = strings.Join(strings.Fields(line), " ")
	runes := []rune(line)
	if len(runes) > 48 {
		line = string(runes[:48])
		if idx := strings.LastIndex(line, " "); idx > 12 {
			line = line[:idx]
		}
		line = strings.TrimSpace(line) + "…"
	}
	r := []rune(line)
	r[0] = unicode.ToUpper(r[0])
	return string(r), true
}

// stripANSI removes ANSI escape sequences (CSI, OSC, two-char escapes) so
// arrow keys / editing keys never pollute a derived title.
func stripANSI(s string) string {
	if !strings.ContainsRune(s, '\x1b') {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] != '\x1b' {
			b.WriteByte(s[i])
			i++
			continue
		}
		if i+1 >= len(s) {
			break
		}
		switch s[i+1] {
		case '[': // CSI: params then a final letter byte
			j := i + 2
			for j < len(s) && (s[j] < 0x40 || s[j] > 0x7e) {
				j++
			}
			i = j + 1
		case ']': // OSC: terminated by BEL or ESC \
			j := i + 2
			for j < len(s) && s[j] != '\a' {
				if s[j] == '\x1b' && j+1 < len(s) && s[j+1] == '\\' {
					j++
					break
				}
				j++
			}
			i = j + 1
		default: // two-character escape
			i += 2
		}
	}
	return b.String()
}

// utf8CarryDecoder decodes a UTF-8 byte stream incrementally, holding back
// trailing incomplete sequences so they are never split mid-character and
// never replaced with U+FFFD. The carry bytes are prepended to the next
// Decode call, so every byte is decoded exactly once. (PTY reads can split a
// multi-byte sequence across chunk boundaries.)
type utf8CarryDecoder struct {
	carry []byte
}

// Decode consumes b plus any bytes held from a previous call, returning the
// complete decoded prefix. Trailing incomplete bytes are kept for the next
// call.
func (d *utf8CarryDecoder) Decode(b []byte) string {
	combined := make([]byte, 0, len(d.carry)+len(b))
	combined = append(combined, d.carry...)
	combined = append(combined, b...)
	d.carry = nil

	tailStart := len(combined)
	for i := len(combined) - 1; i >= 0; i-- {
		c := combined[i]
		if c&0x80 == 0 {
			tailStart = i + 1
			break
		}
		if c&0xC0 == 0xC0 {
			need := 1
			switch {
			case c&0xE0 == 0xC0:
				need = 2
			case c&0xF0 == 0xE0:
				need = 3
			case c&0xF8 == 0xF0:
				need = 4
			}
			if len(combined)-i >= need {
				tailStart = i + need
			} else {
				tailStart = i
			}
			break
		}
	}

	valid := combined[:tailStart]
	tail := combined[tailStart:]
	if len(tail) > 0 {
		d.carry = append([]byte{}, tail...)
	}
	return string(valid)
}
