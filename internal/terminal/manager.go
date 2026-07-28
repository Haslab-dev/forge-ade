package terminal

import (
	"fmt"
	"io"
	"sync"
	"syscall"

	"github.com/creack/pty"
	"github.com/google/uuid"
	"github.com/hasdev/forge-ade/internal/events"
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

// CreateShell creates and starts a shell session.
func (m *Manager) CreateShell(name, folder string) (*Session, error) {
	session := NewShell(name, folder)
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

// List returns all active sessions.
func (m *Manager) List() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		sessions = append(sessions, s)
	}
	return sessions
}

// ListByType returns sessions filtered by type.
func (m *Manager) ListByType(sessionType SessionType) []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Session
	for _, s := range m.sessions {
		if s.Type == sessionType {
			result = append(result, s)
		}
	}
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
		session.cmd.Process.Kill()
	}
	// Close PTY
	if session.pty != nil {
		session.pty.Close()
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

	return session, nil
}

func (m *Manager) readOutput(session *Session) {
	buf := make([]byte, 4096)
	for {
		n, err := session.pty.Read(buf)
		if err != nil {
			if err != io.EOF {
				session.mu.Lock()
				closed := session.closed
				session.mu.Unlock()
				if !closed {
					m.bus.Publish(events.Event{
						Type: events.TerminalClosed,
						Data: map[string]interface{}{
							"id":    session.ID,
							"error": err.Error(),
						},
					})
				}
			}
			return
		}

		if n > 0 {
			data := make([]byte, n)
			copy(data, buf[:n])

			// Fire session-level callback
			session.mu.Lock()
			if session.onData != nil {
				session.onData(string(data))
			}
			session.mu.Unlock()

			m.bus.Publish(events.Event{
				Type: events.TerminalOutput,
				Data: map[string]interface{}{
					"id":   session.ID,
					"data": string(data),
				},
			})
		}
	}
}
