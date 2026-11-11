package ui

import (
	"fmt"
	"sync"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

// TerminalState manages the Ghostty terminal instance and shell lifecycle.
type TerminalState struct {
	mu       sync.Mutex
	term     *terminal.Terminal
	Cwd      string
	Title    string
	ExitCode int
	Exited   bool
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

// Restart spawns a new shell process in the working directory.
func (ts *TerminalState) Restart() error {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if ts.term != nil {
		ts.term.Close()
	}

	term, err := terminal.New(terminal.Options{
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
	})
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

// RenderTerminal draws the Ghostty terminal emulator viewport and toolbar.
func (ts *TerminalState) RenderTerminal(c *ui.Context) {
	t := c.Theme()

	ui.Row(c).Gap(8).Padding(6, 12).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
		ui.Text(c, fmt.Sprintf("Terminal: %s", ts.Title)).Bold()
		if ts.Exited {
			ui.Text(c, fmt.Sprintf("(Exited with code %d)", ts.ExitCode)).TextColor(t.TextMuted)
		}
		ui.Box(c).Grow(1)
		if ui.Button(c, "Restart Shell").Clicked() {
			_ = ts.Restart()
		}
	})

	if ts.term != nil {
		terminal.View(c, ts.term).Fill().AutoFocus()
	} else {
		ui.Column(c).Grow(1).Center().Children(func() {
			ui.Text(c, "Terminal not available").TextColor(t.TextMuted)
			if ui.PrimaryButton(c, "Start Terminal").Clicked() {
				_ = ts.Restart()
			}
		})
	}
}
