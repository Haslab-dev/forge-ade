package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAgentSessions(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "agent-sess-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	engine := NewEngine(tempDir)
	as := NewAgentState(engine, tempDir, nil)

	if len(as.Sessions) == 0 {
		t.Fatalf("expected initial sessions, got 0")
	}

	active := as.ActiveSession()
	if active == nil {
		t.Fatalf("expected non-nil active session")
	}

	// Create new session
	sess := as.NewSession("Port UI to Go Native")
	if sess == nil || sess.Title != "Port UI to Go Native" {
		t.Fatalf("expected created session with title 'Port UI to Go Native', got %v", sess)
	}

	if as.ActiveSessionID != sess.ID {
		t.Fatalf("expected active session ID %s, got %s", sess.ID, as.ActiveSessionID)
	}

	// Pinning
	if sess.Pinned {
		t.Fatalf("expected initial session to be unpinned")
	}
	as.TogglePin(sess.ID)
	if !sess.Pinned {
		t.Fatalf("expected session to be pinned after TogglePin")
	}

	// Filter tabs
	as.ActiveFilterTab = "pinned"
	pinned := as.FilteredSessions()
	found := false
	for _, p := range pinned {
		if p.ID == sess.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected pinned session to appear in filtered sessions")
	}

	// Delete
	as.DeleteSession(sess.ID)
	for _, s := range as.Sessions {
		if s.ID == sess.ID {
			t.Fatalf("expected session %s to be deleted", sess.ID)
		}
	}
}

func TestEditorTabManagement(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ed-tabs-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	f1 := filepath.Join(tempDir, "file1.go")
	f2 := filepath.Join(tempDir, "file2.go")
	_ = os.WriteFile(f1, []byte("package one\n"), 0644)
	_ = os.WriteFile(f2, []byte("package two\n"), 0644)

	em := NewEditorManager()
	_, err = em.OpenFile(f1, "file1.go")
	if err != nil {
		t.Fatalf("failed to open f1: %v", err)
	}

	_, err = em.OpenFile(f2, "file2.go")
	if err != nil {
		t.Fatalf("failed to open f2: %v", err)
	}

	if len(em.OpenTabPaths) != 2 {
		t.Fatalf("expected 2 open tabs, got %d", len(em.OpenTabPaths))
	}
	if em.ActivePath != f2 {
		t.Fatalf("expected active path to be f2, got %s", em.ActivePath)
	}

	// Close active tab f2, should fall back to f1
	em.CloseTab(f2)
	if len(em.OpenTabPaths) != 1 {
		t.Fatalf("expected 1 open tab, got %d", len(em.OpenTabPaths))
	}
	if em.ActivePath != f1 {
		t.Fatalf("expected active path to fall back to f1, got %s", em.ActivePath)
	}
}

func TestAppUIModeSwitch(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "appui-mode-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	engine := NewEngine(tempDir)
	app := NewAppUI(engine, tempDir)

	if app.Mode != ModeAgent {
		t.Fatalf("expected default mode ModeAgent (%d), got %d", ModeAgent, app.Mode)
	}

	app.Mode = ModeEditor
	if app.Mode != ModeEditor {
		t.Fatalf("expected ModeEditor (%d), got %d", ModeEditor, app.Mode)
	}

	// Toggle collapsible sidebars & drawers
	app.ShowBottomTerminal = true
	if !app.ShowBottomTerminal {
		t.Fatalf("expected bottom terminal to be open")
	}

	app.ShowRightDrawer = true
	app.RightDrawerTab = RightTabGit
	if app.RightDrawerTab != RightTabGit {
		t.Fatalf("expected right drawer tab RightTabGit")
	}
}
