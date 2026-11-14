package terminal

import (
	"strings"
	"sync"

	termplugin "github.com/egoist/mygo/plugins/terminal"
)

// TerminalState manages the Ghostty terminal instance and shell lifecycle.
type TerminalState struct {
	mu              sync.Mutex
	term            *termplugin.Terminal
	Cwd             string
	Title           string
	ActiveSessionID string
	ExitCode        int
	Exited          bool
	// Command and Env override the spawned program; empty runs the user's
	// shell (a local shell, or an ssh remote shell for remote workspaces).
	Command []string
	Env     []string
}

// NewTerminalState creates a new terminal manager for the given working directory.
func NewTerminalState(cwd string) *TerminalState {
	ts := &TerminalState{
		Cwd:   cwd,
		Title: "Terminal",
	}
	_ = ts.Restart()
	return ts
}

// NewTerminalStateDeferred creates a terminal manager without spawning a
// process — Restart starts it on demand. Used for restored sessions whose
// process is long gone.
func NewTerminalStateDeferred(cwd string) *TerminalState {
	return &TerminalState{
		Cwd:    cwd,
		Title:  "Terminal",
		Exited: true,
	}
}

// Term returns the underlying terminal instance safely.
func (ts *TerminalState) Term() *termplugin.Terminal {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.term
}

// Restart spawns a new shell process in the working directory.
func (ts *TerminalState) Restart() error {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if ts.term != nil {
		ts.term.Close()
	}

	opts := termplugin.Options{
		Dir: ts.Cwd,
		OnTitle: func(title string) {
			ts.mu.Lock()
			ts.Title = title
			ts.mu.Unlock()
		},
		OnExit: func(code int) {
			ts.mu.Lock()
			ts.Exited = true
			ts.ExitCode = code
			ts.mu.Unlock()
		},
	}
	if len(ts.Command) > 0 {
		opts.Command = ts.Command
	}
	if len(ts.Env) > 0 {
		opts.Env = ts.Env
	}
	term, err := termplugin.New(opts)
	if err != nil {
		return err
	}

	ts.term = term
	ts.Exited = false
	return nil
}

// Close terminates the running terminal process.
func (ts *TerminalState) Close() {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if ts.term != nil {
		ts.term.Close()
		ts.term = nil
	}
}

// SendInput writes raw characters or commands to the running shell process.
func (ts *TerminalState) SendInput(data string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if ts.term != nil {
		ts.term.Send([]byte(data))
	}
}

// WriteInput delivers input command to the terminal process.
func (ts *TerminalState) WriteInput(sessionID, data string) error {
	ts.SendInput(data)
	return nil
}

// RecentOutput returns the most recent lines of terminal scrollback/screen text.
func (ts *TerminalState) RecentOutput(maxLines int) string {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if ts.term == nil {
		return ""
	}
	txt := ts.term.Text()
	if txt == "" {
		txt = string(ts.term.Snapshot())
	}
	if maxLines <= 0 {
		return txt
	}
	lines := strings.Split(txt, "\n")
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return strings.Join(lines, "\n")
}

// GetTitle returns the current window or tab title of the terminal.
func (ts *TerminalState) GetTitle() string {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.Title
}

// IsExited reports whether the subshell process has exited.
func (ts *TerminalState) IsExited() bool {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.Exited
}

// ExitStatus returns the process exit code once it has exited.
func (ts *TerminalState) ExitStatus() int {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.ExitCode
}
