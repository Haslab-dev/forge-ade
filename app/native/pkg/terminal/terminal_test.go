package terminal

import (
	"os"
	"testing"
)

func TestTerminalStateInit(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "forge-native-term-test")
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
