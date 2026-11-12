package terminal

import (
	"sync"

	termplugin "github.com/egoist/mygo/plugins/terminal"
)

// TerminalState manages the Ghostty terminal instance and shell lifecycle.
type TerminalState struct {
	mu       sync.Mutex
	term     *termplugin.Terminal
	Cwd      string
	Title    string
	ExitCode int
	Exited   bool
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
