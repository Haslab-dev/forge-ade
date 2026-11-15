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
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Type     SessionType `json:"type"`
	Provider string      `json:"provider"`
	Folder   string      `json:"folder"`
	Command  string      `json:"command"`
	Isolated bool        `json:"isolated,omitempty"`
	// Executable/Args, when set, override the provider entirely — used by
	// remote shells, which run a local ssh process under the PTY.
	Executable string   `json:"executable,omitempty"`
	Args       []string `json:"args,omitempty"`
	// Extra environment for the override command (SSH_ASKPASS wiring for
	// password-authenticated remote shells).
	Env       []string  `json:"-"`
	Status    string    `json:"status"` // running, stopped, error
	PID       int       `json:"pid"`
	CreatedAt time.Time `json:"createdAt"`

	pty      *os.File   `json:"-"`
	cmd      *exec.Cmd  `json:"-"`
	mu       sync.Mutex `json:"-"`
	closed   bool       `json:"-"`
	waitOnce sync.Once  `json:"-"`
	ptyOnce  sync.Once  `json:"-"`
	onData   func(data string)

	// execMu serializes ExecContext runs on this session: concurrent execs
	// share one capture buffer and one completion marker stream, so two
	// interleaved runs would read each other's output and spuriously time out.
	execMu sync.Mutex `json:"-"`

	// outMu/out buffer recent output so the agent's persistent-shell Exec can
	// capture command results without racing the UI stream.
	outMu sync.Mutex `json:"-"`
	out   []byte     `json:"-"`
}

func (s *Session) waitCmd() {
	s.waitOnce.Do(func() {
		if s.cmd != nil {
			_ = s.cmd.Wait()
		}
	})
}

func (s *Session) closePTY() {
	s.ptyOnce.Do(func() {
		if s.pty != nil {
			_ = s.pty.Close()
		}
	})
}

// GetID satisfies the agent engine's ShellSession interface.
func (s *Session) GetID() string { return s.ID }

// rebuildForRestart returns a fresh, unstarted Session carrying the exact
// launch identity of this one (provider, folder, command override, argv and
// env overrides, isolation flag). A timeout restart uses it so a remote/SSH
// or AI session is recreated as itself — never silently replaced by a plain
// local shell.
func (s *Session) rebuildForRestart() *Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	return &Session{
		Name:       s.Name,
		Type:       s.Type,
		Provider:   s.Provider,
		Folder:     s.Folder,
		Command:    s.Command,
		Isolated:   s.Isolated,
		Executable: s.Executable,
		Args:       append([]string(nil), s.Args...),
		Env:        append([]string(nil), s.Env...),
		CreatedAt:  time.Now(),
	}
}

// appendOutput stores recent output (capped) for Exec capture.
func (s *Session) appendOutput(data string) {
	s.outMu.Lock()
	defer s.outMu.Unlock()
	s.out = append(s.out, []byte(data)...)
	const maxCap = 512 * 1024 // 512 KiB cap
	if len(s.out) > maxCap {
		// Allocate a fresh slice so the old oversized backing array can be
		// reclaimed by GC rather than held in heap by slice headers.
		newOut := make([]byte, maxCap)
		copy(newOut, s.out[len(s.out)-maxCap:])
		s.out = newOut
	}
}

// resetOutput clears the capture buffer (called before each Exec).
func (s *Session) resetOutput() {
	s.outMu.Lock()
	s.out = nil
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

// NewRemoteShell creates a shell session that runs a local ssh process under
// the PTY to reach a remote host. folder keeps the ssh:// URI for display;
// env carries SSH_ASKPASS wiring so password-authenticated connections do not
// prompt again inside the terminal.
func NewRemoteShell(name, folder, exe string, args, env []string) *Session {
	if name == "" {
		name = "Remote Shell"
	}
	return &Session{
		Type:       SessionShell,
		Name:       name,
		Provider:   "shell",
		Folder:     folder,
		Executable: exe,
		Args:       args,
		Env:        env,
		Status:     "created",
		CreatedAt:  time.Now(),
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
