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

func TestNewAgentTaskLandingScreen(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "appui-newtask-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	eng := engine.NewEngine(tempDir)
	app := NewAppUI(eng, tempDir)

	// Ensure there is an initial session
	sess := app.Agent.NewSession("Initial Existing Session")
	if app.Agent.ActiveSessionID != sess.ID {
		t.Fatalf("expected ActiveSessionID == %s, got %s", sess.ID, app.Agent.ActiveSessionID)
	}

	// Switch to Editor mode and open NewTaskModal
	app.Mode = ModeEditor
	app.OpenNewTaskModal()
	if !app.ShowNewTaskModal {
		t.Fatalf("expected ShowNewTaskModal == true")
	}

	// Trigger NewAgentTask ("+ New Task" / ⌘N)
	app.NewAgentTask("")

	if app.Mode != ModeAgent {
		t.Fatalf("expected Mode == ModeAgent (%d), got %d", ModeAgent, app.Mode)
	}
	if app.ActiveTaskKind != "agent" {
		t.Fatalf("expected ActiveTaskKind == 'agent', got '%s'", app.ActiveTaskKind)
	}
	if app.ShowNewTaskModal {
		t.Fatalf("expected ShowNewTaskModal == false")
	}
	if app.Agent.ActiveSessionID != "" {
		t.Fatalf("expected Agent.ActiveSessionID == '', got '%s'", app.Agent.ActiveSessionID)
	}
	if app.Agent.ActiveSession() != nil {
		t.Fatalf("expected ActiveSession() == nil on New Agent UI landing screen")
	}

	// Now simulate user typing a prompt in the New Agent UI composer
	prompt := "Explain repository layout"
	app.Agent.InputText = prompt
	app.Agent.SubmitMessage()

	// Verify a fresh session was created
	newSess := app.Agent.ActiveSession()
	if newSess == nil {
		t.Fatalf("expected ActiveSession() != nil after submitting prompt from New Agent UI")
	}
	if len(newSess.Messages) == 0 {
		t.Fatalf("expected messages to contain user prompt")
	}
	if newSess.Messages[0].Text != prompt {
		t.Fatalf("expected first message '%s', got '%s'", prompt, newSess.Messages[0].Text)
	}
}
