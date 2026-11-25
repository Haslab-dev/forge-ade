package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Haslab-dev/RakitKode/agentclirc"
	"github.com/hasdev/forge-ade/app/native/pkg/terminal"
	"github.com/hasdev/forge-ade/core/agentsession"
)

// AgentCLIDefinition describes an agent CLI known to ForgeADE.
type AgentCLIDefinition struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Executable  string   `json:"executable"`
	Args        []string `json:"args"`
	ResumeArgs  []string `json:"resumeArgs,omitempty"`
	Description string   `json:"description"`
	Available   bool     `json:"available"`
}

// DefaultAgentCLIs registers the official fallback agent CLIs from ForgeADE.
var DefaultAgentCLIs = []AgentCLIDefinition{
	{ID: "antigravity", Name: "Antigravity CLI", Executable: "agy", Args: []string{}, ResumeArgs: []string{"--continue"}, Description: "Official Google Antigravity CLI agent"},
	{ID: "claude-code", Name: "Claude Code", Executable: "claude", Args: []string{}, ResumeArgs: []string{"--continue"}, Description: "Anthropic Claude Code terminal agent"},
	{ID: "codex", Name: "Codex", Executable: "codex", Args: []string{}, ResumeArgs: []string{"resume", "--last"}, Description: "OpenAI Codex CLI coding assistant"},
	{ID: "opencode", Name: "OpenCode", Executable: "opencode", Args: []string{}, ResumeArgs: []string{"--continue"}, Description: "OpenCode terminal coding assistant"},
	{ID: "commandcode", Name: "Command Code", Executable: "cmd", Args: []string{}, ResumeArgs: []string{"--resume"}, Description: "Command Code terminal agent with taste learning"},
	{ID: "ohmypi", Name: "OhMyPi", Executable: "omp", Args: []string{}, ResumeArgs: []string{"--continue"}, Description: "OhMyPi terminal agent framework"},
	{ID: "pi", Name: "Pi", Executable: "pi", Args: []string{}, ResumeArgs: []string{"--continue"}, Description: "Minimalist autonomous terminal agent"},
	{ID: "gemini", Name: "Gemini CLI", Executable: "gemini", Args: []string{}, ResumeArgs: []string{"--continue"}, Description: "Google Gemini terminal AI assistant"},
}

// AgentCLISession represents an interactive agent terminal task session.
type AgentCLISession struct {
	ID         string
	AgentID    string
	AgentName  string
	Title      string
	Command    string
	Args       []string
	Cwd        string
	Status     string // "running", "completed", "error"
	ExitCode   int
	StartedAt  time.Time
	EndedAt    time.Time
	Terminal   *terminal.TerminalState
	ResumeHint []string
}

// AgentCLIManager manages interactive Agent CLI sessions and on-disk CLI configuration discovery.
type AgentCLIManager struct {
	mu           sync.RWMutex
	Sessions     map[string]*AgentCLISession
	ActiveID     string
	Configs      []agentclirc.RuntimeConfig
	RootDir      string
	OnNeedRedraw func()
}

// NewAgentCLIManager constructs the CLI manager and scans for installed agent CLI configs.
func NewAgentCLIManager(rootDir string, onRedraw func()) *AgentCLIManager {
	mgr := &AgentCLIManager{
		Sessions:     make(map[string]*AgentCLISession),
		RootDir:      rootDir,
		OnNeedRedraw: onRedraw,
	}

	mgr.RefreshConfigs()
	mgr.LoadSessionsFromDisk()
	return mgr
}

// NewAgentCLIManagerLazy constructs the CLI manager without blocking startup
// on external CLI model probes (opencode, pi, etc.). Saved sessions load
// immediately from disk while CLI config discovery runs in the background.
func NewAgentCLIManagerLazy(rootDir string, onRedraw func()) *AgentCLIManager {
	mgr := &AgentCLIManager{
		Sessions:     make(map[string]*AgentCLISession),
		RootDir:      rootDir,
		OnNeedRedraw: onRedraw,
	}

	mgr.LoadSessionsFromDisk()
	go func() {
		mgr.RefreshConfigs()
		if mgr.OnNeedRedraw != nil {
			mgr.OnNeedRedraw()
		}
	}()
	return mgr
}

// LoadSessionsFromDisk reads persistent terminal CLI sessions from ~/.forge-ade/agentsessions/index.json.
func (mgr *AgentCLIManager) LoadSessionsFromDisk() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	indexPath := filepath.Join(home, ".forge-ade", "agentsessions", "index.json")
	bytes, err := os.ReadFile(indexPath)
	if err != nil {
		return
	}

	var records []struct {
		ID               string   `json:"id"`
		AgentID          string   `json:"agentId"`
		AgentName        string   `json:"agentName"`
		Title            string   `json:"title"`
		Executable       string   `json:"executable"`
		Args             []string `json:"args"`
		WorkingDirectory string   `json:"workingDirectory"`
		Status           string   `json:"status"`
		CreatedAt        int64    `json:"createdAt"`
		StartedAt        int64    `json:"startedAt"`
		EndedAt          int64    `json:"endedAt"`
		ResumeHint       []string `json:"resumeHint"`
	}
	if err := json.Unmarshal(bytes, &records); err != nil {
		return
	}

	// Smoke-test debris never loads: records pointing at OS temp workspaces
	// (unisolated go test runs used to write one "Test Agent Task" record per
	// run into the real store) are dropped, and the index is rewritten without
	// them so they stop resurrecting after every restart.
	purgedDebris := false
	for _, r := range records {
		if agentsession.IsTempTestPath(r.WorkingDirectory) {
			purgedDebris = true
			break
		}
	}
	if purgedDebris {
		kept := records[:0]
		for _, r := range records {
			if !agentsession.IsTempTestPath(r.WorkingDirectory) {
				kept = append(kept, r)
			}
		}
		records = kept
	}

	mgr.mu.Lock()

	for _, r := range records {
		started := time.UnixMilli(r.CreatedAt)
		if r.StartedAt > 0 {
			started = time.UnixMilli(r.StartedAt)
		}
		normCwd := agentsession.NormalizeWorkspacePath(r.WorkingDirectory)
		spawnCwd := normCwd
		if spawnCwd == "" {
			spawnCwd, _ = os.UserHomeDir()
		}

		// Restored sessions have no live process: build a deferred terminal
		// state (Restart relaunches the CLI on demand). The on-disk output log
		// is PTY output — never feed it back as input, the shell would try to
		// execute the raw escape sequences.
		term := terminal.NewTerminalStateDeferred(spawnCwd)
		if r.Executable != "" {
			term.Command = append([]string{r.Executable}, r.Args...)
		}

		status := r.Status
		if status == "terminated" || status == "exited" {
			status = "completed"
		}

		var ended time.Time
		if r.EndedAt > 0 {
			ended = time.UnixMilli(r.EndedAt)
		}

		mgr.Sessions[r.ID] = &AgentCLISession{
			ID:         r.ID,
			AgentID:    r.AgentID,
			AgentName:  r.AgentName,
			Title:      r.Title,
			Command:    r.Executable,
			Args:       r.Args,
			Cwd:        normCwd,
			Status:     status,
			StartedAt:  started,
			EndedAt:    ended,
			Terminal:   term,
			ResumeHint: r.ResumeHint,
		}
		if mgr.ActiveID == "" && status == "running" {
			mgr.ActiveID = r.ID
		}
	}
	mgr.mu.Unlock()

	// Rewrite the store without the purged debris (SaveSessionsToDisk takes
	// its own lock — it must run after the unlock above).
	if purgedDebris {
		mgr.SaveSessionsToDisk()
	}
}

type agentCLIDiskRecord struct {
	ID               string   `json:"id"`
	AgentID          string   `json:"agentId"`
	AgentName        string   `json:"agentName"`
	Title            string   `json:"title"`
	Executable       string   `json:"executable"`
	Args             []string `json:"args"`
	WorkingDirectory string   `json:"workingDirectory"`
	Status           string   `json:"status"`
	CreatedAt        int64    `json:"createdAt"`
	StartedAt        int64    `json:"startedAt"`
	EndedAt          int64    `json:"endedAt,omitempty"`
	ResumeHint       []string `json:"resumeHint,omitempty"`
}

// SaveSessionsToDisk persists CLI sessions to ~/.forge-ade/agentsessions/index.json.
func (mgr *AgentCLIManager) SaveSessionsToDisk() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	mgr.mu.RLock()
	records := make([]agentCLIDiskRecord, 0, len(mgr.Sessions))
	for _, s := range mgr.Sessions {
		if s == nil {
			continue
		}
		startedMs := s.StartedAt.UnixMilli()
		var endedMs int64
		if !s.EndedAt.IsZero() {
			endedMs = s.EndedAt.UnixMilli()
		}
		records = append(records, agentCLIDiskRecord{
			ID:               s.ID,
			AgentID:          s.AgentID,
			AgentName:        s.AgentName,
			Title:            s.Title,
			Executable:       s.Command,
			Args:             append([]string(nil), s.Args...),
			WorkingDirectory: s.Cwd,
			Status:           s.Status,
			CreatedAt:        startedMs,
			StartedAt:        startedMs,
			EndedAt:          endedMs,
			ResumeHint:       append([]string(nil), s.ResumeHint...),
		})
	}
	mgr.mu.RUnlock()

	dir := filepath.Join(home, ".forge-ade", "agentsessions")
	_ = os.MkdirAll(dir, 0o755)
	if data, err := json.MarshalIndent(records, "", "  "); err == nil {
		_ = os.WriteFile(filepath.Join(dir, "index.json"), data, 0o644)
	}
}

// RefreshConfigs reads on-disk configuration of all known agent CLIs (claude, agy, gemini, pi, codex, etc.).
func (mgr *AgentCLIManager) RefreshConfigs() {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	// Same enrichment the webview applies: per-CLI path fixes (opencode,
	// pi, omp) and the commandcode discovery fallback.
	mgr.Configs = agentsession.EnrichAllAgentRuntimeConfigs(agentclirc.List())
}

// InstalledConfigs returns the subset of agent CLIs that are currently installed on the host.
func (mgr *AgentCLIManager) InstalledConfigs() []agentclirc.RuntimeConfig {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	var installed []agentclirc.RuntimeConfig
	for _, c := range mgr.Configs {
		if c.Installed {
			installed = append(installed, c)
			continue
		}
		if _, err := lookPathEnriched(c.Executable); err == nil {
			c.Installed = true
			installed = append(installed, c)
		}
	}
	return installed
}

// GetAgentCLIs returns all known Agent CLI definitions with updated availability.
func (mgr *AgentCLIManager) GetAgentCLIs() []AgentCLIDefinition {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	var result []AgentCLIDefinition
	seen := make(map[string]bool)

	// Check standard defaults — enriched PATH so GUI launches detect CLIs.
	for _, def := range DefaultAgentCLIs {
		d := def
		if resolved, err := lookPathEnriched(d.Executable); err == nil {
			d.Available = true
			d.Executable = resolved
		}
		result = append(result, d)
		seen[d.ID] = true
	}

	// Add any runtime configs discovered by agentclirc
	for _, c := range mgr.Configs {
		if seen[c.ID] {
			continue
		}
		avail := c.Installed
		if !avail {
			if _, err := lookPathEnriched(c.Executable); err == nil {
				avail = true
			}
		}
		executable := c.Executable
		if resolved, err := lookPathEnriched(c.Executable); err == nil {
			executable = resolved
			avail = true
		}
		result = append(result, AgentCLIDefinition{
			ID:          c.ID,
			Name:        c.Name,
			Executable:  executable,
			Description: fmt.Sprintf("%s runtime CLI", c.Name),
			Available:   avail,
		})
		seen[c.ID] = true
	}

	return result
}

// CreateSessionWithTitle launches an agent CLI task session with a custom title.
func (mgr *AgentCLIManager) CreateSessionWithTitle(agentID, agentName, command string, args []string, cwd string, customTitle string) *AgentCLISession {
	sess := mgr.CreateSession(agentID, agentName, command, args, cwd)
	if sess != nil && strings.TrimSpace(customTitle) != "" {
		sess.Title = strings.TrimSpace(customTitle)
	}
	return sess
}

// CreateSession launches an interactive agent CLI task session. As forge-ade's
// backend does, the CLI executable itself becomes the PTY program — no wrapping
// shell, so its escape sequences never leak into a shell prompt.
func (mgr *AgentCLIManager) CreateSession(agentID, agentName, command string, args []string, cwd string) *AgentCLISession {
	mgr.mu.Lock()

	normCwd := agentsession.NormalizeWorkspacePath(cwd)
	if normCwd == "" {
		normCwd = agentsession.NormalizeWorkspacePath(mgr.RootDir)
	}
	spawnCwd := normCwd
	if spawnCwd == "" {
		spawnCwd, _ = os.UserHomeDir()
	}

	sessionID := fmt.Sprintf("cli-%s-%d", agentID, time.Now().UnixNano()%100000)
	title := fmt.Sprintf("%s Task", agentName)

	term := terminal.NewTerminalStateDeferred(spawnCwd)
	resolvedCmd, err := agentsession.LookPath(command)
	if err == nil && resolvedCmd != "" {
		term.Command = append([]string{resolvedCmd}, args...)
	} else {
		term.Command = append([]string{command}, args...)
	}
	status := "running"
	if err := term.Restart(); err != nil {
		status = "error"
	}

	sess := &AgentCLISession{
		ID:        sessionID,
		AgentID:   agentID,
		AgentName: agentName,
		Title:     title,
		Command:   command,
		Args:      args,
		Cwd:       normCwd,
		Status:    status,
		StartedAt: time.Now(),
		Terminal:  term,
	}

	mgr.Sessions[sessionID] = sess
	mgr.ActiveID = sessionID
	mgr.mu.Unlock()

	mgr.SaveSessionsToDisk()
	return sess
}

// ActiveSession returns the currently selected CLI session.
func (mgr *AgentCLIManager) ActiveSession() *AgentCLISession {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
	return mgr.Sessions[mgr.ActiveID]
}

// SortedSessions returns all CLI sessions in deterministic order (newest
// StartedAt first, tie-broken by ID descending).
func (mgr *AgentCLIManager) SortedSessions() []*AgentCLISession {
	if mgr == nil {
		return nil
	}
	mgr.mu.RLock()
	out := make([]*AgentCLISession, 0, len(mgr.Sessions))
	for _, s := range mgr.Sessions {
		if s != nil {
			out = append(out, s)
		}
	}
	mgr.mu.RUnlock()

	sort.Slice(out, func(i, j int) bool {
		if !out[i].StartedAt.Equal(out[j].StartedAt) {
			return out[i].StartedAt.After(out[j].StartedAt)
		}
		return out[i].ID > out[j].ID
	})
	return out
}

// StopSession stops a CLI session: Ctrl+C first (graceful for TUIs), then the
// process is killed if it survived the grace window.
func (mgr *AgentCLIManager) StopSession(sessionID string) {
	mgr.mu.RLock()
	sess := mgr.Sessions[sessionID]
	var term *terminal.TerminalState
	if sess != nil {
		term = sess.Terminal
	}
	mgr.mu.RUnlock()
	if term == nil {
		return
	}

	term.SendInput("\x03")
	go func() {
		time.Sleep(800 * time.Millisecond)
		if !term.IsExited() {
			term.Close()
		}
	}()

	mgr.mu.Lock()
	if sess, ok := mgr.Sessions[sessionID]; ok {
		sess.Status = "completed"
		sess.EndedAt = time.Now()
	}
	mgr.mu.Unlock()
	mgr.SaveSessionsToDisk()
}

// StopAll stops every running CLI session without the per-session grace
// window. Intended for app shutdown so agent PTYs are not orphaned.
func (mgr *AgentCLIManager) StopAll() {
	if mgr == nil {
		return
	}
	mgr.mu.RLock()
	terms := make([]*terminal.TerminalState, 0, len(mgr.Sessions))
	ids := make([]string, 0, len(mgr.Sessions))
	for id, s := range mgr.Sessions {
		if s == nil {
			continue
		}
		ids = append(ids, id)
		if s.Terminal != nil && !s.Terminal.IsExited() {
			terms = append(terms, s.Terminal)
		}
	}
	mgr.mu.RUnlock()
	for _, t := range terms {
		t.Close()
	}
	mgr.mu.Lock()
	for _, id := range ids {
		if s, ok := mgr.Sessions[id]; ok {
			s.Status = "completed"
			s.EndedAt = time.Now()
		}
	}
	mgr.mu.Unlock()
	mgr.SaveSessionsToDisk()
}

// getResumeArgs finds continuation flags for an agent CLI on restart.
func (mgr *AgentCLIManager) getResumeArgs(agentID, command string) []string {
	for _, def := range DefaultAgentCLIs {
		if strings.EqualFold(def.ID, agentID) || strings.EqualFold(def.Executable, command) {
			if len(def.ResumeArgs) > 0 {
				return def.ResumeArgs
			}
		}
	}

	norm := strings.ToLower(agentID + " " + command)
	if strings.Contains(norm, "codex") {
		return []string{"resume", "--last"}
	}
	if strings.Contains(norm, "commandcode") || strings.Contains(norm, "cmd") {
		return []string{"--resume"}
	}
	return []string{"--continue"}
}

// RestartSession relaunches the session's CLI within the same record, appending
// ResumeArgs (e.g. --continue) or captured ResumeHint so the conversation carries over in the same workspace.
func (mgr *AgentCLIManager) RestartSession(sessionID string) {
	mgr.mu.Lock()

	sess := mgr.Sessions[sessionID]
	if sess == nil || strings.TrimSpace(sess.Command) == "" {
		mgr.mu.Unlock()
		return
	}
	if sess.Terminal != nil {
		sess.Terminal.Close()
	} else {
		cwd := sess.Cwd
		if cwd == "" {
			cwd = agentsession.NormalizeWorkspacePath(mgr.RootDir)
		}
		if cwd == "" {
			cwd, _ = os.UserHomeDir()
		}
		sess.Terminal = terminal.NewTerminalStateDeferred(cwd)
	}

	resumeArgs := sess.ResumeHint
	if len(resumeArgs) == 0 {
		resumeArgs = mgr.getResumeArgs(sess.AgentID, sess.Command)
	}
	cmdArgs := append([]string{}, sess.Args...)
	cmdArgs = append(cmdArgs, resumeArgs...)

	resolvedCmd, err := agentsession.LookPath(sess.Command)
	if err == nil && resolvedCmd != "" {
		sess.Terminal.Command = append([]string{resolvedCmd}, cmdArgs...)
	} else {
		sess.Terminal.Command = append([]string{sess.Command}, cmdArgs...)
	}

	if err := sess.Terminal.Restart(); err != nil {
		sess.Status = "error"
		mgr.mu.Unlock()
		mgr.SaveSessionsToDisk()
		return
	}
	sess.Status = "running"
	sess.ExitCode = 0
	sess.EndedAt = time.Time{}
	sess.StartedAt = time.Now()
	mgr.mu.Unlock()
	mgr.SaveSessionsToDisk()
}

// RenameSession updates the title of a CLI session and persists it to disk.
func (mgr *AgentCLIManager) RenameSession(sessionID, title string) {
	trimmed := strings.TrimSpace(title)
	if trimmed == "" || mgr == nil {
		return
	}
	mgr.mu.Lock()
	if sess := mgr.Sessions[sessionID]; sess != nil {
		sess.Title = trimmed
	}
	mgr.mu.Unlock()
	mgr.SaveSessionsToDisk()
	if mgr.OnNeedRedraw != nil {
		mgr.OnNeedRedraw()
	}
}

// DeleteSession removes a CLI session and terminates its process.
func (mgr *AgentCLIManager) DeleteSession(sessionID string) {
	if mgr == nil || sessionID == "" {
		return
	}
	mgr.mu.Lock()
	sess := mgr.Sessions[sessionID]
	var term *terminal.TerminalState
	var deletedCwd string
	if sess != nil {
		term = sess.Terminal
		deletedCwd = sess.Cwd
		delete(mgr.Sessions, sessionID)
	}
	if mgr.ActiveID == sessionID {
		mgr.ActiveID = ""
		var best *AgentCLISession
		for _, cand := range mgr.Sessions {
			if cand == nil || cand.Cwd != deletedCwd {
				continue
			}
			if best == nil || cand.StartedAt.After(best.StartedAt) || (cand.StartedAt.Equal(best.StartedAt) && cand.ID > best.ID) {
				best = cand
			}
		}
		if best != nil {
			mgr.ActiveID = best.ID
		}
	}
	mgr.mu.Unlock()

	if term != nil {
		term.Close()
	}
	mgr.SaveSessionsToDisk()
	if mgr.OnNeedRedraw != nil {
		mgr.OnNeedRedraw()
	}
}

// SyncExitStatus flips a running session to completed once its PTY process
// has exited, so the UI reflects the real lifecycle.
func (mgr *AgentCLIManager) SyncExitStatus() {
	mgr.mu.Lock()
	changed := false
	for _, sess := range mgr.Sessions {
		if sess.Status != "running" || sess.Terminal == nil {
			continue
		}
		if sess.Terminal.IsExited() {
			sess.Status = "completed"
			sess.ExitCode = sess.Terminal.ExitStatus()
			sess.EndedAt = time.Now()
			changed = true
		}
	}
	mgr.mu.Unlock()
	if changed {
		mgr.SaveSessionsToDisk()
	}
}
