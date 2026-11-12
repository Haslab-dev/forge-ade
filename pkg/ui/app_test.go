package ui

import (
	"os"
	"testing"

	"github.com/hasdev/forge-ade/forge-ade-native/pkg/engine"
)

func TestAppUIModeSwitchAndPanels(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "appui-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	eng := engine.NewEngine(tempDir)
	app := NewAppUI(eng, tempDir)

	if app.Mode != ModeAgent {
		t.Fatalf("expected default mode ModeAgent (%d), got %d", ModeAgent, app.Mode)
	}

	// Switch to Editor mode
	app.Mode = ModeEditor
	if app.Mode != ModeEditor {
		t.Fatalf("expected ModeEditor (%d), got %d", ModeEditor, app.Mode)
	}

	// Toggle collapsible bottom terminal drawer
	app.ShowBottomTerminal = true
	if !app.ShowBottomTerminal {
		t.Fatalf("expected bottom terminal to be open")
	}

	// Toggle collapsible right drawer and tabs
	app.ShowRightDrawer = true
	app.RightDrawerTab = RightTabGit
	if app.RightDrawerTab != RightTabGit {
		t.Fatalf("expected right drawer tab RightTabGit")
	}

	// Check layout metrics
	if app.LeftSidebarWidth != LeftSidebarDefault {
		t.Errorf("expected default left sidebar width %v, got %v", LeftSidebarDefault, app.LeftSidebarWidth)
	}
	if app.RightDrawerWidth != RightDrawerDefault {
		t.Errorf("expected default right drawer width %v, got %v", RightDrawerDefault, app.RightDrawerWidth)
	}

	// Agent sidebar view modes
	if app.AgentSidebarView != AgentSidebarTasks {
		t.Fatalf("expected initial AgentSidebarTasks, got %d", app.AgentSidebarView)
	}
	app.AgentSidebarView = AgentSidebarFiles
	if app.AgentSidebarView != AgentSidebarFiles {
		t.Fatalf("expected AgentSidebarFiles, got %d", app.AgentSidebarView)
	}
}
