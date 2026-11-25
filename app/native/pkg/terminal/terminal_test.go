package terminal

import (
	"os"
	"reflect"
	"strings"
	"testing"

	termplugin "github.com/egoist/mygo/plugins/terminal"
)

func TestTerminalStateInit(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "forge-ade-term-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	ts := &TerminalState{
		Cwd:   tmpDir,
		Title: "Terminal",
	}

	if ts.Cwd != tmpDir {
		t.Errorf("expected cwd %q, got %q", tmpDir, ts.Cwd)
	}

	title := ts.GetTitle()
	if title != "Terminal" {
		t.Errorf("expected title 'Terminal', got %q", title)
	}

	if ts.IsExited() {
		t.Errorf("expected not exited")
	}
}

func TestTerminalMultiTabState(t *testing.T) {
	tmpDir := t.TempDir()
	ts := &TerminalState{
		Cwd:   tmpDir,
		Title: "zsh",
		Tabs: []*TerminalTab{
			{ID: 1, Title: "zsh", Cwd: tmpDir},
			{ID: 2, Title: "ssh", Cwd: tmpDir},
			{ID: 3, Title: "logs", Cwd: tmpDir},
		},
		ActiveTabIdx: 0,
	}

	if got := ts.TabCount(); got != 3 {
		t.Fatalf("expected TabCount=3, got %d", got)
	}

	snaps, active := ts.SnapshotTabs()
	if len(snaps) != 3 || active != 0 || snaps[1].Title != "ssh" {
		t.Fatalf("unexpected SnapshotTabs: active=%d snaps=%+v", active, snaps)
	}

	ts.SelectTab(1)
	if ts.GetTitle() != "ssh" {
		t.Fatalf("expected active title 'ssh', got %q", ts.GetTitle())
	}
	// Out-of-bounds selections should be ignored.
	ts.SelectTab(-1)
	ts.SelectTab(99)
	if _, active = ts.SnapshotTabs(); active != 1 || ts.GetTitle() != "ssh" {
		t.Fatalf("expected active tab 1 ('ssh') unchanged, got active=%d title=%q", active, ts.GetTitle())
	}

	// Closing tab 0 shifts ActiveTabIdx down from 1 -> 0 while preserving "ssh".
	ts.CloseTab(0)
	if ts.TabCount() != 2 {
		t.Fatalf("expected TabCount=2 after closing tab 0, got %d", ts.TabCount())
	}
	snaps, active = ts.SnapshotTabs()
	if active != 0 || snaps[active].Title != "ssh" || ts.GetTitle() != "ssh" {
		t.Fatalf("expected active=0 ('ssh') after closing earlier tab, got active=%d snaps=%+v title=%q", active, snaps, ts.GetTitle())
	}

	// Closing non-active trailing tab preserves tab 0 ("ssh").
	ts.CloseTab(1)
	if ts.TabCount() != 1 || ts.GetTitle() != "ssh" {
		t.Fatalf("expected TabCount=1 and title 'ssh', got count=%d title=%q", ts.TabCount(), ts.GetTitle())
	}
}

func TestTerminalScrollReflect(t *testing.T) {
	termType := reflect.TypeOf(termplugin.Terminal{})
	field, ok := termType.FieldByName("term")
	if !ok {
		t.Fatalf("field 'term' not found in termplugin.Terminal")
	}
	t.Logf("field 'term' found: type=%v, offset=%d", field.Type, field.Offset)

	muField, ok := termType.FieldByName("mu")
	if !ok {
		t.Fatalf("field 'mu' not found in termplugin.Terminal")
	}
	t.Logf("field 'mu' found: type=%v, offset=%d", muField.Type, muField.Offset)

	// Check methods on field.Type (*vt.Terminal)
	for i := 0; i < field.Type.NumMethod(); i++ {
		m := field.Type.Method(i)
		t.Logf("method: %s", m.Name)
	}

	if _, ok := field.Type.MethodByName("ScrollToBottom"); !ok {
		t.Fatalf("method ScrollToBottom not found on %v", field.Type)
	}
	if _, ok := field.Type.MethodByName("ScrollBy"); !ok {
		t.Fatalf("method ScrollBy not found on %v", field.Type)
	}

	drawField, ok := termType.FieldByName("draw")
	if !ok {
		t.Fatalf("field 'draw' not found in termplugin.Terminal")
	}
	t.Logf("field 'draw' found: type=%v, offset=%d", drawField.Type, drawField.Offset)

	// Verify methods don't panic on nil terminal
	ts := &TerminalState{}
	ts.ScrollPageUp()
	ts.ScrollPageDown()
	ts.ScrollToTop()
	ts.ScrollToBottom()
}

func TestExtractTerminalURLsWrapped(t *testing.T) {
	// Exact layout from a wrapped OAuth login URL inside tmux (105 columns).
	screen := "" +
		"Open the URL below in your browser:\n" +
		"https://accounts.google.com/o/oauth2/auth?access_type=offline&client_id=1071006060591-tmhssin2h21lcre235vtolo\n" +
		"jh4g403ep.apps.googleusercontent.com&code_challenge=Kq1jePFLtbt1z7XMTT0XZTGupyCFwDUa9vOJKSt3wj0&code_challe\n" +
		"nge_method=S256&prompt=consent&redirect_uri=https%3A%2F%2Fantigravity.google%2Foauth-callback&response_type=\n" +
		"code&scope=https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fcloud-platform+https%3A%2F%2Fwww.googleapis.com%2Fauth%2\n" +
		"Fuserinfo.email+https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fuserinfo.profile+https%3A%2F%2Fwww.googleapis.com%2F\n" +
		"auth%2Fcclog+https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fexperimentsandconfigs+https%3A%2F%2Fwww.googleapis.com\n" +
		"%2Fauth%2Faicode+openid&state=u2u9UyapPgRozNNqFgrBxw\n" +
		"\n" +
		"After authenticating, copy the code displayed in the browser.\n"

	urls := ExtractTerminalURLs(screen, 170)
	if len(urls) != 1 {
		t.Fatalf("expected 1 unwrapped URL even in wide 170-col terminal, got %d: %v", len(urls), urls)
	}
	want := "https://accounts.google.com/o/oauth2/auth?access_type=offline&client_id=1071006060591-tmhssin2h21lcre235vtolo" +
		"jh4g403ep.apps.googleusercontent.com&code_challenge=Kq1jePFLtbt1z7XMTT0XZTGupyCFwDUa9vOJKSt3wj0&code_challe" +
		"nge_method=S256&prompt=consent&redirect_uri=https%3A%2F%2Fantigravity.google%2Foauth-callback&response_type=" +
		"code&scope=https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fcloud-platform+https%3A%2F%2Fwww.googleapis.com%2Fauth%2" +
		"Fuserinfo.email+https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fuserinfo.profile+https%3A%2F%2Fwww.googleapis.com%2F" +
		"auth%2Fcclog+https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fexperimentsandconfigs+https%3A%2F%2Fwww.googleapis.com" +
		"%2Fauth%2Faicode+openid&state=u2u9UyapPgRozNNqFgrBxw"
	if urls[0] != want {
		t.Fatalf("unwrapped URL mismatch:\n got: %s\nwant: %s", urls[0], want)
	}
	if host := URLHostLabel(urls[0]); host != "accounts.google.com" {
		t.Fatalf("expected host accounts.google.com, got %q", host)
	}

	// Short multi-line non-wrapped URLs must stay separate.
	two := "visit https://example.com/a.\nand https://github.com/hasdev/forge-ade)\n"
	gotTwo := ExtractTerminalURLs(two, 80)
	if len(gotTwo) != 2 || gotTwo[0] != "https://github.com/hasdev/forge-ade" || gotTwo[1] != "https://example.com/a" {
		t.Fatalf("expected 2 separate trimmed URLs newest-first, got %v", gotTwo)
	}

	// Exact 147-col layout from screenshot (ending with &c on row 1).
	screen147 := "" +
		"Open the URL below in your browser:\n" +
		"───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────\n" +
		"https://accounts.google.com/o/oauth2/auth?access_type=offline&client_id=1071006060591-tmhssin2h21lcre235vtolojh4g403ep.apps.googleusercontent.com&c\n" +
		"ode_challenge=LzfDF301hfCeY69oSCNYzLqv-rrgpQDT6TrNdxE7egQ&code_challenge_method=S256&prompt=consent&redirect_uri=https%3A%2F%2Fantigravity.google%2\n" +
		"Foauth-callback&response_type=code&scope=https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fcloud-platform+https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fuserinf\n" +
		"o.email+https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fuserinfo.profile+https%3A%2F%2Fwww.googleapis.com%2Fauth%2FccLog+https%3A%2F%2Fwww.googleapis.com\n" +
		"%2Fauth%2Fexperimentsandconfigs+https%3A%2F%2Fwww.googleapis.com%2Fauth%2Faicode+openid&state=nAVERxydJlaAjx2jwhhBjq\n" +
		"───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────\n"
	urls147 := ExtractTerminalURLs(screen147, 150)
	if len(urls147) != 1 || !strings.HasSuffix(urls147[0], "state=nAVERxydJlaAjx2jwhhBjq") {
		t.Fatalf("expected full 5-line URL ending in state=..., got %v", urls147)
	}

	// Exact 1-space indented 5-line OAuth URL from TUI box padding.
	indented := "" +
		" Open the URL below in your browser:\n" +
		" https://accounts.google.com/o/oauth2/auth?access_type=offline&client_id=1071006060591-tmhssin2h21lcre235vtolojh4g403ep.apps.googleusercontent.com&c\n" +
		" ode_challenge=v_BrJEm2QpiLYnWkKxvuo8fOrK-C4N0BcN3AFuGPNn0&code_challenge_method=S256&prompt=consent&redirect_uri=https%3A%2F%2Fantigravity.google%2\n" +
		" Foauth-callback&response_type=code&scope=https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fcloud-platform+https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fuserinf\n" +
		" o.email+https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fuserinfo.profile+https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fcclog+https%3A%2F%2Fwww.googleapis.com\n" +
		" %2Fauth%2Fexperimentsandconfigs+https%3A%2F%2Fwww.googleapis.com%2Fauth%2Faicode+openid&state=MQfDILMVMRwyheZ39stXgg\n" +
		" After authenticating, copy the code displayed in the browser.\n"
	urlsIndented := ExtractTerminalURLs(indented, 150)
	if len(urlsIndented) != 1 || !strings.HasSuffix(urlsIndented[0], "state=MQfDILMVMRwyheZ39stXgg") {
		t.Fatalf("expected full indented 5-line URL ending in state=MQfDILMVMRwyheZ39stXgg, got %v", urlsIndented)
	}
}
