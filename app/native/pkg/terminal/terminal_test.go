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
