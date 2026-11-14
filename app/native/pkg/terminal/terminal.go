package terminal

import (
	"strings"
	"sync"

	termplugin "github.com/egoist/mygo/plugins/terminal"
)

// TerminalTab holds state for one shell/SSH tab in TerminalState.
type TerminalTab struct {
	ID       int
	Title    string
	Cwd      string
	Exited   bool
	ExitCode int
	Term     *termplugin.Terminal
	Command  []string
	Env      []string
}

// TerminalTabSnapshot is an immutable snapshot of a terminal tab's metadata.
type TerminalTabSnapshot struct {
	Index  int
	ID     int
	Title  string
	Exited bool
}

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

	Tabs         []*TerminalTab
	ActiveTabIdx int
	nextTabID    int
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

func cloneStrings(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	out := make([]string, len(s))
	copy(out, s)
	return out
}

func (ts *TerminalState) ensureActiveTabLocked() *TerminalTab {
	if len(ts.Tabs) == 0 {
		ts.nextTabID++
		title := ts.Title
		if title == "" {
			title = "Terminal"
		}
		tab := &TerminalTab{
			ID:       ts.nextTabID,
			Title:    title,
			Cwd:      ts.Cwd,
			Exited:   ts.Exited,
			ExitCode: ts.ExitCode,
			Term:     ts.term,
			Command:  cloneStrings(ts.Command),
			Env:      cloneStrings(ts.Env),
		}
		ts.Tabs = []*TerminalTab{tab}
		ts.ActiveTabIdx = 0
	}
	if ts.ActiveTabIdx < 0 || ts.ActiveTabIdx >= len(ts.Tabs) {
		ts.ActiveTabIdx = 0
	}
	return ts.Tabs[ts.ActiveTabIdx]
}

func (ts *TerminalState) syncActiveLocked() {
	if len(ts.Tabs) == 0 {
		return
	}
	if ts.ActiveTabIdx < 0 || ts.ActiveTabIdx >= len(ts.Tabs) {
		ts.ActiveTabIdx = 0
	}
	tab := ts.Tabs[ts.ActiveTabIdx]
	ts.term = tab.Term
	ts.Cwd = tab.Cwd
	ts.Title = tab.Title
	ts.Exited = tab.Exited
	ts.ExitCode = tab.ExitCode
	ts.Command = cloneStrings(tab.Command)
	ts.Env = cloneStrings(tab.Env)
}

func (ts *TerminalState) spawnTabLocked(tab *TerminalTab) error {
	if tab.Term != nil {
		tab.Term.Close()
		tab.Term = nil
	}
	if tab.Title == "" {
		tab.Title = "Terminal"
	}
	opts := termplugin.Options{
		Dir: tab.Cwd,
		OnTitle: func(title string) {
			ts.mu.Lock()
			tab.Title = title
			if ts.ActiveTabIdx >= 0 && ts.ActiveTabIdx < len(ts.Tabs) && ts.Tabs[ts.ActiveTabIdx] == tab {
				ts.Title = title
			}
			ts.mu.Unlock()
		},
		OnExit: func(code int) {
			ts.mu.Lock()
			tab.Exited = true
			tab.ExitCode = code
			if ts.ActiveTabIdx >= 0 && ts.ActiveTabIdx < len(ts.Tabs) && ts.Tabs[ts.ActiveTabIdx] == tab {
				ts.Exited = true
				ts.ExitCode = code
			}
			ts.mu.Unlock()
		},
	}
	if len(tab.Command) > 0 {
		opts.Command = tab.Command
	}
	if len(tab.Env) > 0 {
		opts.Env = tab.Env
	}
	term, err := termplugin.New(opts)
	if err != nil {
		ts.syncActiveLocked()
		return err
	}
	tab.Term = term
	tab.Exited = false
	tab.ExitCode = 0
	ts.syncActiveLocked()
	return nil
}

// Restart spawns a new shell process in the active tab's working directory.
func (ts *TerminalState) Restart() error {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	tab := ts.ensureActiveTabLocked()
	if ts.term != nil && ts.term != tab.Term {
		ts.term.Close()
		ts.term = nil
	}
	tab.Cwd = ts.Cwd
	tab.Command = cloneStrings(ts.Command)
	tab.Env = cloneStrings(ts.Env)
	if ts.Title != "" {
		tab.Title = ts.Title
	}
	return ts.spawnTabLocked(tab)
}

// ResetShell clears any custom command/env (such as SSH) and restarts a local shell in cwd.
func (ts *TerminalState) ResetShell(cwd string) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if cwd != "" {
		ts.Cwd = cwd
	}
	ts.Command = nil
	ts.Env = nil
	ts.Title = "Terminal"
	tab := ts.ensureActiveTabLocked()
	if ts.term != nil && ts.term != tab.Term {
		ts.term.Close()
		ts.term = nil
	}
	tab.Cwd = ts.Cwd
	tab.Command = nil
	tab.Env = nil
	tab.Title = "Terminal"
	return ts.spawnTabLocked(tab)
}

// TabCount returns the number of open terminal tabs.
func (ts *TerminalState) TabCount() int {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return len(ts.Tabs)
}

// SnapshotTabs returns an immutable copy of open tabs and the active tab index.
func (ts *TerminalState) SnapshotTabs() ([]TerminalTabSnapshot, int) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if len(ts.Tabs) == 0 {
		return nil, 0
	}
	out := make([]TerminalTabSnapshot, len(ts.Tabs))
	for i, tab := range ts.Tabs {
		out[i] = TerminalTabSnapshot{
			Index:  i,
			ID:     tab.ID,
			Title:  tab.Title,
			Exited: tab.Exited,
		}
	}
	return out, ts.ActiveTabIdx
}

// NewTab spawns a new shell tab in cwd (or the active tab's Cwd when empty) and activates it.
func (ts *TerminalState) NewTab(cwd string) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	ts.ensureActiveTabLocked()
	if cwd == "" {
		cwd = ts.Cwd
	}
	ts.nextTabID++
	tab := &TerminalTab{
		ID:    ts.nextTabID,
		Title: "Terminal",
		Cwd:   cwd,
	}
	ts.Tabs = append(ts.Tabs, tab)
	ts.ActiveTabIdx = len(ts.Tabs) - 1
	return ts.spawnTabLocked(tab)
}

// SelectTab switches the active terminal tab to idx.
func (ts *TerminalState) SelectTab(idx int) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if idx < 0 || idx >= len(ts.Tabs) {
		return
	}
	ts.ActiveTabIdx = idx
	ts.syncActiveLocked()
}

// CloseTab closes tab idx. If it is the only tab, it restarts that tab with a fresh shell.
func (ts *TerminalState) CloseTab(idx int) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if idx < 0 || idx >= len(ts.Tabs) {
		return
	}
	if len(ts.Tabs) <= 1 {
		tab := ts.Tabs[0]
		tab.Command = nil
		tab.Env = nil
		tab.Title = "Terminal"
		_ = ts.spawnTabLocked(tab)
		return
	}
	victim := ts.Tabs[idx]
	if victim.Term != nil {
		victim.Term.Close()
		victim.Term = nil
	}
	ts.Tabs = append(ts.Tabs[:idx], ts.Tabs[idx+1:]...)
	if idx < ts.ActiveTabIdx {
		ts.ActiveTabIdx--
	}
	if ts.ActiveTabIdx >= len(ts.Tabs) {
		ts.ActiveTabIdx = len(ts.Tabs) - 1
	}
	ts.syncActiveLocked()
}

// Close terminates all running terminal processes.
func (ts *TerminalState) Close() {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	for _, tab := range ts.Tabs {
		if tab != nil && tab.Term != nil {
			tab.Term.Close()
			tab.Term = nil
		}
	}
	if ts.term != nil {
		ts.term.Close()
		ts.term = nil
	}
	ts.Tabs = nil
	ts.ActiveTabIdx = 0
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
