package terminal

import (
	"os"
	"os/exec"
	"sync"
	"time"
)

// SessionType categorizes what kind of process this session runs.
type SessionType string

const (
	SessionShell  SessionType = "shell"
	SessionAI     SessionType = "ai"
	SessionAgent  SessionType = "agent"
	SessionDocker SessionType = "docker"
	SessionSSH    SessionType = "ssh"
	SessionCustom SessionType = "custom"
)

// Session represents any executable process — shell, AI agent, Docker, etc.
type Session struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Type      SessionType `json:"type"`
	Provider  string      `json:"provider"`
	Folder    string      `json:"folder"`
	Command   string      `json:"command"`
	Status    string      `json:"status"` // running, stopped, error
	PID       int         `json:"pid"`
	CreatedAt time.Time   `json:"createdAt"`

	pty    *os.File    `json:"-"`
	cmd    *exec.Cmd   `json:"-"`
	mu     sync.Mutex  `json:"-"`
	closed bool        `json:"-"`
	onData func(data string)

	// outMu/out buffer recent output so the agent's persistent-shell Exec can
	// capture command results without racing the UI stream.
	outMu sync.Mutex `json:"-"`
	out   []byte     `json:"-"`
}

// appendOutput stores recent output (capped) for Exec capture.
func (s *Session) appendOutput(data string) {
	s.outMu.Lock()
	defer s.outMu.Unlock()
	s.out = append(s.out, []byte(data)...)
	if len(s.out) > 2<<20 { // 2 MiB cap
		s.out = s.out[len(s.out)-2<<20:]
	}
}

// resetOutput clears the capture buffer (called before each Exec).
func (s *Session) resetOutput() {
	s.outMu.Lock()
	s.out = s.out[:0]
	s.outMu.Unlock()
}

// snapshotOutput returns the current capture buffer.
func (s *Session) snapshotOutput() string {
	s.outMu.Lock()
	defer s.outMu.Unlock()
	return string(s.out)
}

// NewShell creates a new shell session.
func NewShell(name, folder string) *Session {
	if name == "" {
		name = "Shell"
	}
	return &Session{
		Type:      SessionShell,
		Name:      name,
		Provider:  "shell",
		Folder:    folder,
		Status:    "created",
		CreatedAt: time.Now(),
	}
}

// NewAIAgent creates a new AI agent session.
func NewAIAgent(name, provider, folder string) *Session {
	if name == "" {
		name = provider
	}
	return &Session{
		Type:      SessionAI,
		Name:      name,
		Provider:  provider,
		Folder:    folder,
		Status:    "created",
		CreatedAt: time.Now(),
	}
}

// OnData registers a callback for PTY output.
func (s *Session) OnData(fn func(data string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onData = fn
}
