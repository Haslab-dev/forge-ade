package terminal

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Haslab-dev/RakitKode/events"
	"github.com/creack/pty"
	"github.com/google/uuid"
)

// Manager manages all session types (shell, AI agents, etc.).
type Manager struct {
	bus      *events.Bus
	mu       sync.RWMutex
	sessions map[string]*Session
}

// NewManager creates a new unified session manager.
func NewManager(bus *events.Bus) *Manager {
	return &Manager{
		bus:      bus,
		sessions: make(map[string]*Session),
	}
}

// CreateShell creates and starts a shell session. An "isolated:" folder prefix
// (or empty folder) creates an isolated global shell that survives workspace close/switches.
func (m *Manager) CreateShell(name, folder string) (*Session, error) {
	isolated := false
	if strings.HasPrefix(folder, "isolated:") {
		folder = strings.TrimPrefix(folder, "isolated:")
		isolated = true
	} else if folder == "" {
		isolated = true
	}
	session := NewShell(name, folder)
	session.Isolated = isolated
	return m.start(session)
}

// CreateRemoteShell starts a shell session that runs a local ssh process
// under the PTY to reach a remote host. folder is the ssh:// URI for display;
// env carries SSH_ASKPASS wiring from the connection.
func (m *Manager) CreateRemoteShell(name, folder, exe string, args, env []string) (*Session, error) {
	session := NewRemoteShell(name, folder, exe, args, env)
	return m.start(session)
}

// CreateAIAgent creates and starts an AI agent session.
func (m *Manager) CreateAIAgent(name, provider, folder string) (*Session, error) {
	session := NewAIAgent(name, provider, folder)
	return m.start(session)
}

// Get returns a session by ID.
func (m *Manager) Get(id string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[id]
	return s, ok
}

// List returns all active sessions, ordered by creation time (stable order —
// Go map iteration is randomized, which caused the frontend session tabs to
// reorder randomly on every poll).
func (m *Manager) List() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		sessions = append(sessions, s)
	}
	sort.SliceStable(sessions, func(i, j int) bool {
		if !sessions[i].CreatedAt.Equal(sessions[j].CreatedAt) {
			return sessions[i].CreatedAt.Before(sessions[j].CreatedAt)
		}
		return sessions[i].Name < sessions[j].Name
	})
	return sessions
}

// ListByType returns sessions filtered by type, in stable creation order.
func (m *Manager) ListByType(sessionType SessionType) []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Session
	for _, s := range m.sessions {
		if s.Type == sessionType {
			result = append(result, s)
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		if !result[i].CreatedAt.Equal(result[j].CreatedAt) {
			return result[i].CreatedAt.Before(result[j].CreatedAt)
		}
		return result[i].Name < result[j].Name
	})
	return result
}

// Write sends data to a session's stdin.
func (m *Manager) Write(id string, data []byte) (int, error) {
	m.mu.RLock()
	session, ok := m.sessions[id]
	m.mu.RUnlock()

	if !ok {
		return 0, fmt.Errorf("session not found: %s", id)
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if session.closed {
		return 0, fmt.Errorf("session closed: %s", id)
	}

	return session.pty.Write(data)
}

// Resize resizes a session's PTY.
func (m *Manager) Resize(id string, rows, cols uint16) error {
	m.mu.RLock()
	session, ok := m.sessions[id]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("session not found: %s", id)
	}

	if err := pty.Setsize(session.pty, &pty.Winsize{Rows: rows, Cols: cols}); err != nil {
		return err
	}

	m.bus.Publish(events.Event{
		Type: events.TerminalResized,
		Data: map[string]interface{}{
			"id":   id,
			"rows": rows,
			"cols": cols,
		},
	})

	return nil
}

// Exec runs a command in an existing persistent shell session and waits for a
// completion marker, returning the captured output and exit code. On timeout
// the shell is killed and recreated under the same id so the caller stays
// ExecContext runs a command in an existing persistent shell session and waits for a
// completion marker, respecting context cancellation for immediate stopping.
func (m *Manager) ExecContext(ctx context.Context, id, command string, timeout time.Duration) (string, int, error) {
	m.mu.RLock()
	sess, ok := m.sessions[id]
	m.mu.RUnlock()
	if !ok {
		return "", 1, fmt.Errorf("terminal session %s not found", id)
	}
	startMarker := "__FORGE_START_" + uuid.NewString()[:8] + "__"
	endMarker := "__FORGE_END_" + uuid.NewString()[:8] + "__"
	sess.resetOutput()
	cmdLine := fmt.Sprintf("printf \"\\n%%s\\n\" %q\n%s\nprintf \"\\n%%s:$?\\n\" %q\n", startMarker, command, endMarker)
	if _, err := sess.pty.WriteString(cmdLine); err != nil {
		return "", 1, fmt.Errorf("write to terminal: %w", err)
	}
	markerRe := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(endMarker) + `:(\d+)\s*$`)
	deadline := time.Now().Add(timeout)
	for {
		if ctx != nil && ctx.Err() != nil {
			// Interrupt running process in PTY immediately (SIGINT / Ctrl-C)
			_, _ = sess.pty.Write([]byte{0x03, '\n'})
			return strings.TrimSpace(stripTerminalAnsi(sess.snapshotOutput())), 130, ctx.Err()
		}
		out := sess.snapshotOutput()
		if match := markerRe.FindStringSubmatch(out); match != nil {
			code, _ := strconv.Atoi(match[1])

			// Extract only the output between the line after startMarker and the line before endMarker.
			// This completely eliminates echoed command text, ZLE line-wrapping duplicate characters, and prompts.
			var realOutput string
			if startIdx := strings.LastIndex(out, startMarker); startIdx != -1 {
				afterStart := out[startIdx+len(startMarker):]
				if nlIdx := strings.Index(afterStart, "\n"); nlIdx != -1 {
					afterStart = afterStart[nlIdx+1:]
				}
				if endIdx := strings.Index(afterStart, endMarker); endIdx != -1 {
					realOutput = afterStart[:endIdx]
				} else {
					realOutput = afterStart
				}
			} else {
				var cleaned []string
				for _, ln := range strings.Split(out, "\n") {
					if !strings.Contains(ln, endMarker) && !strings.Contains(ln, startMarker) {
						cleaned = append(cleaned, ln)
					}
				}
				realOutput = strings.Join(cleaned, "\n")
			}

			cleanedOut := strings.ReplaceAll(realOutput, "\r\n", "\n")
			cleanedOut = strings.ReplaceAll(cleanedOut, "\r", "")
			return strings.TrimSpace(stripTerminalAnsi(cleanedOut)), code, nil
		}
		if time.Now().After(deadline) {
			_ = m.Stop(id)
			if sh, err := m.CreateShell(sess.Name, sess.Folder); err == nil {
				m.mu.Lock()
				delete(m.sessions, sh.ID) // drop the auto-assigned key
				sh.ID = id                // reuse the id so callers stay valid
				m.sessions[id] = sh
				m.mu.Unlock()
			}
			return strings.TrimSpace(out), 1, fmt.Errorf("command timed out after %s; shell restarted", timeout.Round(time.Second))
		}
		time.Sleep(40 * time.Millisecond)
	}
}

// Exec runs a command in an existing persistent shell session.
func (m *Manager) Exec(id, command string, timeout time.Duration) (string, int, error) {
	return m.ExecContext(context.Background(), id, command, timeout)
}

// Stop kills the process and removes the session from the list.
func (m *Manager) Stop(id string) error {
	m.mu.Lock()
	session, ok := m.sessions[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("session not found: %s", id)
	}
	delete(m.sessions, id)
	m.mu.Unlock()

	session.mu.Lock()
	session.closed = true
	session.Status = "stopped"
	pid := session.PID
	session.mu.Unlock()

	// Kill by PID
	if pid > 0 {
		syscall.Kill(pid, syscall.SIGKILL)
	}
	// Also via cmd.Process
	if session.cmd != nil && session.cmd.Process != nil {
		_ = session.cmd.Process.Kill()
		_ = session.cmd.Wait()
	}
	// Close PTY
	if session.pty != nil {
		_ = session.pty.Close()
	}

	m.bus.Publish(events.Event{
		Type: events.TerminalClosed,
		Data: map[string]interface{}{
			"id": id,
		},
	})

	return nil
}

// Rename changes a session's display name.
func (m *Manager) Rename(id, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return fmt.Errorf("session not found: %s", id)
	}
	s.Name = name
	return nil
}

// StopByFolder terminates every session whose folder is one of the given
// workspace folders (or a subfolder of it). Closing a workspace must only
// stop the shells that were opened for it: Agent and Editor surfaces keep
// independent folders, so a remote shell opened from the editor has to
// survive the agent surface closing its own (unrelated) workspace.
func (m *Manager) StopByFolder(folders []string) {
	cleanFolders := make([]string, 0, len(folders))
	for _, f := range folders {
		if f != "" {
			cleanFolders = append(cleanFolders, strings.TrimRight(f, "/"))
		}
	}
	if len(cleanFolders) == 0 {
		return
	}

	m.mu.RLock()
	ids := make([]string, 0, len(m.sessions))
	for id, s := range m.sessions {
		if s.Isolated {
			continue
		}
		sFolder := strings.TrimRight(s.Folder, "/")
		for _, f := range cleanFolders {
			if sFolder == f || strings.HasPrefix(sFolder, f+"/") {
				ids = append(ids, id)
				break
			}
		}
	}
	m.mu.RUnlock()

	for _, id := range ids {
		_ = m.Stop(id)
	}
}

// StopAll terminates all sessions.
func (m *Manager) StopAll() {
	m.mu.RLock()
	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	m.mu.RUnlock()

	for _, id := range ids {
		_ = m.Stop(id)
	}
}

func (m *Manager) start(session *Session) (*Session, error) {
	session.ID = uuid.New().String()
	session.Status = "starting"

	cmd, err := BuildCommand(session)
	if err != nil {
		return nil, fmt.Errorf("build command: %w", err)
	}

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 24, Cols: 80})
	if err != nil {
		return nil, fmt.Errorf("start pty: %w", err)
	}

	session.pty = ptmx
	session.cmd = cmd
	session.PID = cmd.Process.Pid
	session.Status = "running"

	m.mu.Lock()
	m.sessions[session.ID] = session
	m.mu.Unlock()

	// Publish event
	m.bus.Publish(events.Event{
		Type: events.TerminalOpened,
		Data: map[string]interface{}{
			"id":       session.ID,
			"name":     session.Name,
			"type":     string(session.Type),
			"provider": session.Provider,
		},
	})

	// Read output in background
	go m.readOutput(session)
	go m.reap(session)

	return session, nil
}

func (m *Manager) readOutput(session *Session) {
	buf := make([]byte, 65536)
	// Stateful UTF-8 decode across reads. A PTY read can split a multi-byte
	// UTF-8 sequence (spinner glyphs, box-drawing chars) across chunk
	// boundaries. Converting each chunk independently with string(data)
	// corrupts the split character into U+FFFD (the ��� seen in the
	// terminal) AND, worse, desyncs the byte stream for xterm's parser.
	//
	// The decoder holds back any trailing incomplete sequence and prepends
	// it to the next read — the combined bytes are decoded exactly once,
	// with zero replacement characters and byte-exact output.
	decoder := &utf8CarryDecoder{}
	for {
		n, err := session.pty.Read(buf)
		if err != nil {
			return
		}
		if n > 0 {
			decoded := decoder.Decode(buf[:n])
			if len(decoded) > 0 {
				session.appendOutput(decoded)
				m.bus.Publish(events.Event{
					Type: events.TerminalOutput,
					Data: map[string]interface{}{
						"id":   session.ID,
						"data": decoded,
					},
				})
			}
		}
	}
}

func (m *Manager) reap(session *Session) {
	if session.cmd != nil {
		_ = session.cmd.Wait()
	}

	session.mu.Lock()
	if session.closed {
		session.mu.Unlock()
		return
	}
	session.closed = true
	session.Status = "stopped"
	session.mu.Unlock()

	if session.pty != nil {
		_ = session.pty.Close()
	}

	m.mu.Lock()
	delete(m.sessions, session.ID)
	m.mu.Unlock()

	m.bus.Publish(events.Event{
		Type: events.TerminalClosed,
		Data: map[string]interface{}{
			"id": session.ID,
		},
	})
}

// utf8CarryDecoder decodes a UTF-8 byte stream incrementally, holding back
// trailing incomplete sequences so they are never split mid-character and
// never replaced with U+FFFD. The carry bytes are prepended to the next
// Decode call, so every byte is decoded exactly once.
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

	// Find the longest valid UTF-8 prefix by scanning from the end for a
	// clean boundary (ASCII byte or a complete multi-byte sequence).
	tailStart := len(combined)
	for i := len(combined) - 1; i >= 0; i-- {
		c := combined[i]
		if c&0x80 == 0 {
			tailStart = i + 1 // ASCII: everything up to and including i is clean
			break
		}
		if c&0xC0 == 0xC0 {
			// Lead byte at i — the sequence is complete if enough bytes follow.
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
				tailStart = i + need // complete sequence — boundary after it
			} else {
				tailStart = i // incomplete — hold from the lead byte
			}
			break
		}
		// continuation byte without a lead — keep scanning left
	}

	valid := combined[:tailStart]
	tail := combined[tailStart:]
	if len(tail) > 0 {
		d.carry = append([]byte{}, tail...)
	}
	return string(valid)
}

func (m *Manager) publishOutput(id string, data []byte) {
	out := make([]byte, len(data))
	copy(out, data)
	m.bus.Publish(events.Event{
		Type: events.TerminalOutput,
		Data: map[string]interface{}{
			"id":   id,
			"data": string(out),
		},
	})
}

// stripTerminalAnsi removes ANSI escape codes (CSI, OSC, two-char escapes)
// so output captured from PTY terminals doesn't pollute model transcripts or UI cards.
func stripTerminalAnsi(s string) string {
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
			for j < len(s) && s[j] != '\x07' && s[j] != '\x1b' {
				j++
			}
			if j < len(s) && s[j] == '\x1b' && j+1 < len(s) && s[j+1] == '\\' {
				j++
			}
			i = j + 1
		default:
			i += 2
		}
	}
	return b.String()
}
