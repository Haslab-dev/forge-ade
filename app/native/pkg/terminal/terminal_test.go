package terminal

import (
	"os"
	"reflect"
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



