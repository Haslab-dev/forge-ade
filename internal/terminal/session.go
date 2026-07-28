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
