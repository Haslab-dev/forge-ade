package terminal

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/google/uuid"
	"github.com/hasdev/forge-ade/internal/events"
	"golang.org/x/term"
)

// Session represents a single terminal PTY session.
type Session struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Shell     string    `json:"shell"`
	CWD       string    `json:"cwd"`
	CreatedAt time.Time `json:"createdAt"`
	PID       int       `json:"pid"`

	pty    *os.File     `json:"-"`
	cmd    *exec.Cmd    `json:"-"`
	mu     sync.Mutex   `json:"-"`
	closed bool         `json:"-"`
	winCh  chan Winsize `json:"-"`
}

// Winsize represents terminal dimensions.
type Winsize struct {
	Rows uint16 `json:"rows"`
	Cols uint16 `json:"cols"`
}

// Manager manages multiple terminal PTY sessions.
type Manager struct {
	bus      *events.Bus
	mu       sync.RWMutex
	sessions map[string]*Session
}

// NewManager creates a new terminal manager.
func NewManager(bus *events.Bus) *Manager {
	return &Manager{
		bus:      bus,
		sessions: make(map[string]*Session),
	}
}

// Create creates a new terminal session.
func (m *Manager) Create(name, shell, cwd string) (*Session, error) {
	if shell == "" {
		shell = detectShell()
	}
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	cmd := exec.Command(shell)
	cmd.Dir = cwd

	// Set environment
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("TERM=xterm-256color"))

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: 24,
		Cols: 80,
	})
	if err != nil {
		return nil, fmt.Errorf("start pty: %w", err)
	}

	// Set raw mode on the pty
	if fd := int(ptmx.Fd()); fd >= 0 {
		_, _ = term.MakeRaw(fd)
	}

	session := &Session{
		ID:        uuid.New().String(),
		Name:      name,
		Shell:     shell,
		CWD:       cwd,
		CreatedAt: time.Now(),
		PID:       cmd.Process.Pid,
		pty:       ptmx,
		cmd:       cmd,
		winCh:     make(chan Winsize, 10),
	}

	m.mu.Lock()
	m.sessions[session.ID] = session
	m.mu.Unlock()

	// Start reading output
	go m.readOutput(session)

	// Publish event
	m.bus.Publish(events.Event{
		Type: events.TerminalOpened,
		Data: map[string]interface{}{
			"id":   session.ID,
			"name": session.Name,
		},
	})

	return session, nil
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

// Write sends data to a terminal session (input from user).
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

// Resize resizes a terminal session.
func (m *Manager) Resize(id string, rows, cols uint16) error {
	m.mu.RLock()
	session, ok := m.sessions[id]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("session not found: %s", id)
	}

	if err := pty.Setsize(session.pty, &pty.Winsize{
		Rows: rows,
		Cols: cols,
	}); err != nil {
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

// Close terminates a terminal session.
func (m *Manager) Close(id string) error {
	m.mu.RLock()
	session, ok := m.sessions[id]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("session not found: %s", id)
	}

	session.mu.Lock()
	if session.closed {
		session.mu.Unlock()
		return nil
	}
	session.closed = true
	session.mu.Unlock()

	if err := session.cmd.Process.Kill(); err != nil {
		return err
	}

	_ = session.pty.Close()

	m.mu.Lock()
	delete(m.sessions, id)
	m.mu.Unlock()

	m.bus.Publish(events.Event{
		Type: events.TerminalClosed,
		Data: map[string]interface{}{
			"id": id,
		},
	})

	return nil
}

// CloseAll closes all terminal sessions.
func (m *Manager) CloseAll() {
	m.mu.RLock()
	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	m.mu.RUnlock()

	for _, id := range ids {
		_ = m.Close(id)
	}
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

func detectShell() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}
	return shell
}
