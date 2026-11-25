package terminal

import (
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"unicode"
	"unsafe"

	"github.com/egoist/mygo"
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
	gen      uint64
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

	// DefaultShell is the executable used for new local shell tabs when set
	// (Settings → Default Shell); empty runs the user's login shell.
	defaultShell string

	Tabs         []*TerminalTab
	ActiveTabIdx int
	nextTabID    int
}

// SetDefaultShell configures the shell executable spawned by new local shell
// tabs (Settings → Default Shell). Empty falls back to the user's login
// shell. Safe from any goroutine.
func (ts *TerminalState) SetDefaultShell(shell string) {
	ts.mu.Lock()
	ts.defaultShell = shell
	ts.mu.Unlock()
}

// DefaultUserShell returns the detected login shell executable for the current user.
// On macOS / Unix, it checks $SHELL, /bin/zsh, /bin/bash, /bin/sh.
func DefaultUserShell() string {
	if sh := strings.TrimSpace(os.Getenv("SHELL")); sh != "" {
		if _, err := os.Stat(sh); err == nil {
			return sh
		}
	}
	if runtime.GOOS == "darwin" {
		if _, err := os.Stat("/bin/zsh"); err == nil {
			return "/bin/zsh"
		}
	}
	for _, candidate := range []string{"/bin/zsh", "/bin/bash", "/bin/sh"} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return "/bin/sh"
}

// defaultTabTitle returns a friendly title based on cwd (folder name or remote path).
func defaultTabTitle(cwd string) string {
	if cwd == "" || cwd == "/" || cwd == "." {
		return "Terminal"
	}
	if strings.HasPrefix(cwd, "ssh://") {
		trimmed := strings.TrimPrefix(cwd, "ssh://")
		slashIdx := strings.Index(trimmed, "/")
		if slashIdx != -1 {
			rem := trimmed[slashIdx:]
			b := filepath.Base(rem)
			if b != "" && b != "/" && b != "." {
				return b
			}
		}
		return "remote"
	}
	b := filepath.Base(cwd)
	if b == "" || b == "/" || b == "." {
		return "Terminal"
	}
	return b
}

// localShellCommand resolves the spawn command for a local shell tab: the
// configured DefaultShell when set, otherwise DefaultUserShell().
// Automatically includes the login shell flag (-l) so environment variables
// (.zprofile, .zshrc, .bash_profile, PATH) are properly loaded.
// Caller holds ts.mu.
func (ts *TerminalState) localShellCommand() []string {
	sh := strings.TrimSpace(ts.defaultShell)
	if sh == "" {
		sh = DefaultUserShell()
	}
	parts := strings.Fields(sh)
	if len(parts) == 0 {
		return []string{DefaultUserShell(), "-l"}
	}
	hasLoginFlag := false
	for _, p := range parts {
		if p == "-l" || p == "--login" {
			hasLoginFlag = true
			break
		}
	}
	if !hasLoginFlag {
		parts = append(parts, "-l")
	}
	return parts
}

// NewTerminalState creates a new terminal manager for the given working directory.
func NewTerminalState(cwd string) *TerminalState {
	ts := &TerminalState{
		Cwd:   cwd,
		Title: defaultTabTitle(cwd),
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

func closeTerms(terms ...*termplugin.Terminal) {
	for _, t := range terms {
		if t != nil {
			t.Close()
		}
	}
}

func sanitizeCwd(cwd string) string {
	if cwd == "" || strings.HasPrefix(cwd, "ssh://") {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
		return "."
	}
	if stat, err := os.Stat(cwd); err != nil || !stat.IsDir() {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
		return "."
	}
	return cwd
}

func (ts *TerminalState) spawnTabLocked(tab *TerminalTab) (*termplugin.Terminal, error) {
	oldTerm := tab.Term
	tab.Term = nil
	tab.gen++
	gen := tab.gen
	if tab.Title == "" {
		tab.Title = defaultTabTitle(tab.Cwd)
	}
	opts := termplugin.Options{
		Dir:         sanitizeCwd(tab.Cwd),
		Transparent: true,
		OpenLink: func(link string) bool {
			trimmed := strings.TrimSpace(link)
			lower := strings.ToLower(trimmed)
			if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "mailto:") {
				resolved := ts.ResolveURL(trimmed)
				go func() { _ = mygo.Shell.OpenExternal(resolved) }()
				return true
			}
			return false
		},
		OnTitle: func(title string) {
			ts.mu.Lock()
			if tab.gen == gen {
				tab.Title = title
				if ts.ActiveTabIdx >= 0 && ts.ActiveTabIdx < len(ts.Tabs) && ts.Tabs[ts.ActiveTabIdx] == tab {
					ts.Title = title
				}
			}
			ts.mu.Unlock()
		},
		OnExit: func(code int) {
			ts.mu.Lock()
			if tab.gen == gen {
				tab.Exited = true
				tab.ExitCode = code
				if ts.ActiveTabIdx >= 0 && ts.ActiveTabIdx < len(ts.Tabs) && ts.Tabs[ts.ActiveTabIdx] == tab {
					ts.Exited = true
					ts.ExitCode = code
				}
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
		return oldTerm, err
	}
	tab.Term = term
	tab.Exited = false
	tab.ExitCode = 0
	ts.syncActiveLocked()
	return oldTerm, nil
}

// Restart spawns a new shell process in the active tab's working directory.
func (ts *TerminalState) Restart() error {
	var toClose []*termplugin.Terminal
	ts.mu.Lock()
	tab := ts.ensureActiveTabLocked()
	if ts.term != nil && ts.term != tab.Term {
		toClose = append(toClose, ts.term)
		ts.term = nil
	}
	tab.Cwd = ts.Cwd
	tab.Command = cloneStrings(ts.Command)
	if len(tab.Command) == 0 {
		// No override (SSH etc.): honor the configured Default Shell.
		tab.Command = ts.localShellCommand()
	}
	tab.Env = cloneStrings(ts.Env)
	if ts.Title != "" {
		tab.Title = ts.Title
	}
	old, err := ts.spawnTabLocked(tab)
	if old != nil {
		toClose = append(toClose, old)
	}
	ts.mu.Unlock()
	closeTerms(toClose...)
	return err
}

// ResetShell clears any custom command/env (such as SSH) and restarts a local shell in cwd.
func (ts *TerminalState) ResetShell(cwd string) error {
	var toClose []*termplugin.Terminal
	ts.mu.Lock()
	if cwd != "" {
		ts.Cwd = cwd
	}
	ts.Command = nil
	ts.Env = nil
	ts.Title = "Terminal"
	tab := ts.ensureActiveTabLocked()
	if ts.term != nil && ts.term != tab.Term {
		toClose = append(toClose, ts.term)
		ts.term = nil
	}
	tab.Cwd = ts.Cwd
	tab.Command = ts.localShellCommand()
	tab.Env = nil
	tab.Title = "Terminal"
	old, err := ts.spawnTabLocked(tab)
	if old != nil {
		toClose = append(toClose, old)
	}
	ts.mu.Unlock()
	closeTerms(toClose...)
	return err
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

// NewTabCustom spawns a new shell tab with explicit title, working directory,
// command and environment, and activates it.
func (ts *TerminalState) NewTabCustom(title, cwd string, command, env []string) (*TerminalTab, error) {
	ts.mu.Lock()
	ts.ensureActiveTabLocked()
	if title == "" {
		title = "Terminal"
	}
	ts.nextTabID++
	tab := &TerminalTab{
		ID:      ts.nextTabID,
		Title:   title,
		Cwd:     cwd,
		Command: cloneStrings(command),
		Env:     cloneStrings(env),
	}
	ts.Tabs = append(ts.Tabs, tab)
	ts.ActiveTabIdx = len(ts.Tabs) - 1
	old, err := ts.spawnTabLocked(tab)
	ts.mu.Unlock()
	closeTerms(old)
	return tab, err
}

// NewTab spawns a new shell tab in cwd (or the active tab's Cwd when empty) and activates it.
// If the active tab was running a remote SSH command or custom command, the new tab
// inherits that command and environment so remote workspaces spawn remote shells.
func (ts *TerminalState) NewTab(cwd string) error {
	ts.mu.Lock()
	active := ts.ensureActiveTabLocked()
	if cwd == "" {
		cwd = ts.Cwd
	}
	ts.nextTabID++
	title := defaultTabTitle(cwd)
	var cmd, env []string
	if active != nil && len(active.Command) > 0 {
		cmd = cloneStrings(active.Command)
		env = cloneStrings(active.Env)
		title = active.Title
	} else if len(ts.Command) > 0 {
		cmd = cloneStrings(ts.Command)
		env = cloneStrings(ts.Env)
		title = ts.Title
	} else {
		cmd = ts.localShellCommand()
	}
	tab := &TerminalTab{
		ID:      ts.nextTabID,
		Title:   title,
		Cwd:     cwd,
		Command: cmd,
		Env:     env,
	}
	ts.Tabs = append(ts.Tabs, tab)
	ts.ActiveTabIdx = len(ts.Tabs) - 1
	old, err := ts.spawnTabLocked(tab)
	ts.mu.Unlock()
	closeTerms(old)
	return err
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
	var old *termplugin.Terminal
	ts.mu.Lock()
	if idx < 0 || idx >= len(ts.Tabs) {
		ts.mu.Unlock()
		return
	}
	if len(ts.Tabs) <= 1 {
		tab := ts.Tabs[0]
		if !strings.HasPrefix(tab.Cwd, "ssh://") && !strings.HasPrefix(ts.Cwd, "ssh://") {
			tab.Command = ts.localShellCommand()
			tab.Env = nil
			tab.Title = "Terminal"
		}
		old, _ = ts.spawnTabLocked(tab)
		ts.mu.Unlock()
		closeTerms(old)
		return
	}
	victim := ts.Tabs[idx]
	victim.gen++
	old = victim.Term
	victim.Term = nil
	ts.Tabs = append(ts.Tabs[:idx], ts.Tabs[idx+1:]...)
	if idx < ts.ActiveTabIdx {
		ts.ActiveTabIdx--
	}
	if ts.ActiveTabIdx >= len(ts.Tabs) {
		ts.ActiveTabIdx = len(ts.Tabs) - 1
	}
	ts.syncActiveLocked()
	ts.mu.Unlock()
	closeTerms(old)
}

// Close terminates all running terminal processes.
func (ts *TerminalState) Close() {
	var toClose []*termplugin.Terminal
	ts.mu.Lock()
	for _, tab := range ts.Tabs {
		if tab != nil {
			tab.gen++
			if tab.Term != nil {
				toClose = append(toClose, tab.Term)
				tab.Term = nil
			}
		}
	}
	if ts.term != nil {
		toClose = append(toClose, ts.term)
		ts.term = nil
	}
	ts.Tabs = nil
	ts.ActiveTabIdx = 0
	ts.mu.Unlock()
	closeTerms(toClose...)
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

var (
	termFieldOffset uintptr
	termFieldFound  bool
	termMuOffset    uintptr
	termMuFound     bool
	drawFieldOffset uintptr
	drawFieldFound  bool
)

func init() {
	termType := reflect.TypeOf(termplugin.Terminal{})
	if f, ok := termType.FieldByName("term"); ok {
		termFieldOffset = f.Offset
		termFieldFound = true
	}
	if f, ok := termType.FieldByName("mu"); ok {
		termMuOffset = f.Offset
		termMuFound = true
	}
	if f, ok := termType.FieldByName("draw"); ok {
		drawFieldOffset = f.Offset
		drawFieldFound = true
	}
}

// scrollTerminal moves the terminal scrollback buffer and triggers an immediate view redraw.
func scrollTerminal(t *termplugin.Terminal, action string) bool {
	if t == nil || !termFieldFound {
		return false
	}
	tPtr := unsafe.Pointer(t)
	if termMuFound {
		mu := (*sync.Mutex)(unsafe.Pointer(uintptr(tPtr) + termMuOffset))
		mu.Lock()
		defer mu.Unlock()
	}
	vtPtr := *(*unsafe.Pointer)(unsafe.Pointer(uintptr(tPtr) + termFieldOffset))
	if vtPtr == nil {
		return false
	}
	f, _ := reflect.TypeOf(termplugin.Terminal{}).FieldByName("term")
	vtVal := reflect.NewAt(f.Type.Elem(), vtPtr)

	if alt := vtVal.MethodByName("AltScreen"); alt.IsValid() {
		res := alt.Call(nil)
		if len(res) > 0 && res[0].Bool() {
			return false
		}
	}

	switch action {
	case "to_top":
		if m := vtVal.MethodByName("ScrollToTop"); m.IsValid() {
			m.Call(nil)
		}
	case "to_bottom":
		if m := vtVal.MethodByName("ScrollToBottom"); m.IsValid() {
			m.Call(nil)
		}
	case "page_up":
		rows := 24
		if sz := vtVal.MethodByName("Size"); sz.IsValid() {
			res := sz.Call(nil)
			if len(res) == 2 {
				rows = int(res[1].Int())
			}
		}
		page := rows - 1
		if page < 1 {
			page = 1
		}
		if m := vtVal.MethodByName("ScrollBy"); m.IsValid() {
			m.Call([]reflect.Value{reflect.ValueOf(-page)})
		}
	case "page_down":
		rows := 24
		if sz := vtVal.MethodByName("Size"); sz.IsValid() {
			res := sz.Call(nil)
			if len(res) == 2 {
				rows = int(res[1].Int())
			}
		}
		page := rows - 1
		if page < 1 {
			page = 1
		}
		if m := vtVal.MethodByName("ScrollBy"); m.IsValid() {
			m.Call([]reflect.Value{reflect.ValueOf(page)})
		}
	default:
		return false
	}

	if drawFieldFound {
		drawPtr := (*atomic.Pointer[func()])(unsafe.Pointer(uintptr(tPtr) + drawFieldOffset))
		if f := drawPtr.Load(); f != nil {
			(*f)()
		}
	}
	return true
}

// ScrollPageUp scrolls the active tab's viewport up by one display page.
func (ts *TerminalState) ScrollPageUp() {
	ts.mu.Lock()
	term := ts.term
	ts.mu.Unlock()
	if term != nil {
		scrollTerminal(term, "page_up")
	}
}

// ScrollPageDown scrolls the active tab's viewport down by one display page.
func (ts *TerminalState) ScrollPageDown() {
	ts.mu.Lock()
	term := ts.term
	ts.mu.Unlock()
	if term != nil {
		scrollTerminal(term, "page_down")
	}
}

// ScrollToTop scrolls the active tab's viewport to the very top of the scrollback.
func (ts *TerminalState) ScrollToTop() {
	ts.mu.Lock()
	term := ts.term
	ts.mu.Unlock()
	if term != nil {
		scrollTerminal(term, "to_top")
	}
}

// ScrollToBottom scrolls the active tab's viewport to the bottom (active prompt).
func (ts *TerminalState) ScrollToBottom() {
	ts.mu.Lock()
	term := ts.term
	ts.mu.Unlock()
	if term != nil {
		scrollTerminal(term, "to_bottom")
	}
}

func isURLBodyChar(b byte) bool {
	if b <= ' ' || b >= 0x7f {
		return false
	}
	switch b {
	case '<', '>', '"', '`', '|', '\\', '^', '{', '}':
		return false
	}
	return true
}

func trimTrailingURLPunct(u string) string {
	for len(u) > 0 {
		switch u[len(u)-1] {
		case '.', ',', ';', ':', '!', '?', '\'', '"':
			u = u[:len(u)-1]
		case ')':
			if strings.Count(u, ")") > strings.Count(u, "(") {
				u = u[:len(u)-1]
			} else {
				return u
			}
		case ']':
			if strings.Count(u, "]") > strings.Count(u, "[") {
				u = u[:len(u)-1]
			} else {
				return u
			}
		default:
			return u
		}
	}
	return u
}

func isURLContinuationPrefix(s string) bool {
	if s == "" || s[0] == ' ' || s[0] == '\t' {
		return false
	}
	low := strings.ToLower(s)
	if strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") {
		return false
	}
	switch s[0] {
	case '-', '*', '#', '>', '$', '[', '|':
		return false
	}
	// Reject UTF-8 box-drawing / divider lines (─, ━, ═, │, etc.).
	for _, r := range s {
		if r >= 0x2500 && r <= 0x257f {
			return false
		}
		break
	}
	return true
}

func hasURLSyntaxChar(s string) bool {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '=', '&', '%', '?', '/', '+', '_', '-', '.', ':':
			return true
		}
	}
	return false
}

// ExtractTerminalURLs scans terminal text and reconstructs http:// and https://
// URLs even when wrapped across multiple lines (e.g. inside tmux, narrow panes,
// or CLI output that hard-wraps at 80–120 columns). Results are returned
// newest-first and deduplicated.
func ExtractTerminalURLs(text string, cols int) []string {
	if text == "" {
		return nil
	}
	rawLines := strings.Split(text, "\n")
	lines := make([]string, len(rawLines))
	for i, l := range rawLines {
		lines[i] = strings.TrimRightFunc(l, func(r rune) bool {
			return unicode.IsSpace(r) || r == 0 || r == '\u200b'
		})
	}

	wrapThresh := 40
	if cols > 0 && cols-4 < wrapThresh {
		wrapThresh = max(cols-4, 20)
	}

	var found []string
	seen := make(map[string]bool)

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		searchFrom := 0
		for searchFrom < len(line) {
			low := strings.ToLower(line[searchFrom:])
			idxHTTP := strings.Index(low, "http://")
			idxHTTPS := strings.Index(low, "https://")
			rel := -1
			if idxHTTP >= 0 && (idxHTTPS < 0 || idxHTTP < idxHTTPS) {
				rel = idxHTTP
			} else if idxHTTPS >= 0 {
				rel = idxHTTPS
			}
			if rel < 0 {
				break
			}
			start := searchFrom + rel
			end := start
			for end < len(line) && isURLBodyChar(line[end]) {
				end++
			}
			var sb strings.Builder
			sb.WriteString(line[start:end])

			// If the URL reached the end of a wrapped row, join continuation lines.
			curIdx := i
			curEnd := end
			curLine := line
			maxIndent := max(start+2, 4)
			for curEnd == len(curLine) && len([]rune(curLine)) >= wrapThresh && curIdx+1 < len(lines) {
				rawNext := lines[curIdx+1]
				indent := 0
				for indent < len(rawNext) && rawNext[indent] == ' ' {
					indent++
				}
				if indent > maxIndent {
					break
				}
				next := rawNext[indent:]
				if !isURLContinuationPrefix(next) {
					break
				}
				k := 0
				for k < len(next) && isURLBodyChar(next[k]) {
					k++
				}
				if k == 0 {
					break
				}
				// Join if the entire next line is unbroken URL characters, OR its
				// leading token clearly contains URL syntax (not a prose word).
				if k < len(next) && !hasURLSyntaxChar(next[:k]) {
					break
				}
				sb.WriteString(next[:k])
				curIdx++
				curLine = rawNext
				curEnd = indent + k
				i = curIdx
			}

			u := trimTrailingURLPunct(sb.String())
			if parsed, err := url.Parse(u); err == nil && parsed.Host != "" && (parsed.Scheme == "http" || parsed.Scheme == "https") {
				if !seen[u] {
					seen[u] = true
					found = append(found, u)
				}
			}
			line = curLine
			searchFrom = curEnd
		}
	}

	// Reverse so newest URL appears first.
	for l, r := 0, len(found)-1; l < r; l, r = l+1, r-1 {
		found[l], found[r] = found[r], found[l]
	}
	return found
}

// VisibleURLs returns http/https URLs found in the active terminal's visible
// screen and recent scrollback, newest first.
func (ts *TerminalState) VisibleURLs() []string {
	if ts == nil {
		return nil
	}
	ts.mu.Lock()
	term := ts.term
	ts.mu.Unlock()
	if term == nil {
		return nil
	}
	cols, rows := term.Size()
	txt := term.Text()
	if txt == "" {
		return nil
	}
	lines := strings.Split(txt, "\n")
	keep := rows + 16
	if keep < 40 {
		keep = 40
	}
	if len(lines) > keep {
		lines = lines[len(lines)-keep:]
	}
	return ExtractTerminalURLs(strings.Join(lines, "\n"), cols)
}

// ResolveURL expands a truncated single-row URL fragment against wrapped URLs
// currently visible in the terminal.
func (ts *TerminalState) ResolveURL(fragment string) string {
	fragment = trimTrailingURLPunct(strings.TrimSpace(fragment))
	if fragment == "" || ts == nil {
		return fragment
	}
	for _, full := range ts.VisibleURLs() {
		if strings.HasPrefix(full, fragment) && len(full) > len(fragment) {
			return full
		}
	}
	return fragment
}

// URLHostLabel returns a short display label for a URL (e.g. "accounts.google.com").
func URLHostLabel(raw string) string {
	if u, err := url.Parse(strings.TrimSpace(raw)); err == nil && u.Host != "" {
		return u.Host
	}
	if len(raw) > 32 {
		return raw[:29] + "..."
	}
	return raw
}
