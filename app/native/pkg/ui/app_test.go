package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hasdev/forge-ade/app/native/pkg/agent"
	"github.com/hasdev/forge-ade/app/native/pkg/engine"
	"github.com/hasdev/forge-ade/core/agentsession"
)

func init() {
	_ = os.Setenv("SHELL_SESSIONS_DISABLE", "1")
	_ = os.Setenv("HISTFILE", "/dev/null")
}

func TestAppUIModeSwitchAndPanels(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "appui-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	eng := engine.NewEngine(tempDir)
	app := NewAppUI(eng, tempDir)
	defer app.Close()

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
	if app.LeftSidebarView != "" {
		t.Fatalf("expected initial left sidebar view (tasks), got %q", app.LeftSidebarView)
	}
	app.LeftSidebarView = "files"
	if app.LeftSidebarView != "files" {
		t.Fatalf("expected files view, got %q", app.LeftSidebarView)
	}
	app.LeftSidebarView = "tasks"
	if app.LeftSidebarView != "tasks" {
		t.Fatalf("expected tasks view, got %q", app.LeftSidebarView)
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
	defer app.Close()

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

func TestSidebarDeterministicProjectOrderAndGrouping(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	root := t.TempDir()
	dirAlpha := filepath.Join(root, "alpha")
	dirSub := filepath.Join(dirAlpha, "sub")
	dirBeta := filepath.Join(root, "Beta")
	dirZeta := filepath.Join(root, "zeta")
	for _, d := range []string{dirAlpha, dirSub, dirBeta, dirZeta} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}

	eng := engine.NewEngine(dirZeta)
	app := NewAppUI(eng, dirZeta)
	defer app.Close()

	now := time.Now()
	sGeneral := &agent.AgentSession{ID: "s-gen", Title: "Gen", WorkspacePath: "", CreatedAt: now, UpdatedAt: now}
	sAlpha := &agent.AgentSession{ID: "s-alpha", Title: "Alpha", WorkspacePath: dirAlpha, CreatedAt: now.Add(-3 * time.Minute), UpdatedAt: now.Add(-3 * time.Minute)}
	sSub := &agent.AgentSession{ID: "s-sub", Title: "Sub", WorkspacePath: dirSub, CreatedAt: now.Add(-1 * time.Minute), UpdatedAt: now.Add(5 * time.Minute)}
	sBeta := &agent.AgentSession{ID: "s-beta", Title: "Beta", WorkspacePath: dirBeta, CreatedAt: now.Add(-2 * time.Minute), UpdatedAt: now.Add(-2 * time.Minute)}
	app.Agent.ReplaceSessions([]*agent.AgentSession{sBeta, sSub, sAlpha, sGeneral})

	assertGroupOrder := func(want []string) {
		t.Helper()
		groups := app.sidebarProjectGroups()
		if len(groups) != len(want) {
			got := make([]string, len(groups))
			for i, g := range groups {
				got[i] = g.ProjectName
			}
			t.Fatalf("got group order %v, want %v", got, want)
		}
		for i, g := range groups {
			if g.ProjectName != want[i] {
				got := make([]string, len(groups))
				for k, gg := range groups {
					got[k] = gg.ProjectName
				}
				t.Fatalf("got group order %v, want %v", got, want)
			}
		}
	}

	wantOrder := []string{"General", "alpha", "Beta", "sub", "zeta"}
	assertGroupOrder(wantOrder)

	// Selecting sessions across projects (including parent alpha vs child sub)
	// must NEVER reorder groups or falsely merge sub into alpha.
	app.selectAgentSession(sAlpha)
	assertGroupOrder([]string{"General", "alpha", "Beta", "sub"})
	for _, g := range app.sidebarProjectGroups() {
		if g.ProjectName == "alpha" && len(g.Sessions) != 1 {
			t.Fatalf("expected alpha to have 1 session (sub must not merge into alpha), got %d", len(g.Sessions))
		}
		if g.ProjectName == "sub" && len(g.Sessions) != 1 {
			t.Fatalf("expected sub to remain its own group with 1 session, got %d", len(g.Sessions))
		}
	}

	app.selectAgentSession(sSub)
	assertGroupOrder([]string{"General", "alpha", "Beta", "sub"})

	app.selectAgentSession(sGeneral)
	assertGroupOrder([]string{"General", "alpha", "Beta", "sub"})
}

func TestSelectAgentSessionPreservesActiveIDAndSwitchesWorkspace(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	dirA := t.TempDir()
	dirB := t.TempDir()
	eng := engine.NewEngine(dirA)
	app := NewAppUI(eng, dirA)
	defer app.Close()

	now := time.Now()
	sA := &agent.AgentSession{ID: "sess-a", Title: "Task A", WorkspacePath: dirA, CreatedAt: now, UpdatedAt: now}
	sB := &agent.AgentSession{ID: "sess-b", Title: "Task B", WorkspacePath: dirB, CreatedAt: now, UpdatedAt: now}
	app.Agent.ReplaceSessions([]*agent.AgentSession{sA, sB})
	app.Agent.SelectSession(sA.ID)

	app.selectAgentSession(sB)
	if app.Agent.ActiveSessionID != sB.ID {
		t.Fatalf("expected ActiveSessionID=%q after selectAgentSession, got %q", sB.ID, app.Agent.ActiveSessionID)
	}
	if app.Workspace.RootDir != agentsession.NormalizeWorkspacePath(dirB) {
		t.Fatalf("expected workspace %q, got %q", agentsession.NormalizeWorkspacePath(dirB), app.Workspace.RootDir)
	}

	// Project "+ New task" targeting General must switch back to General workspace.
	app.NewAgentTaskInProject("General", "")
	if app.Workspace.RootDir != "" {
		t.Fatalf("expected General workspace RootDir=\"\", got %q", app.Workspace.RootDir)
	}
	if app.Agent.ActiveSession() != nil {
		t.Fatalf("expected no active session on NewAgentTaskInProject(\"General\")")
	}
}

func TestDeleteWorkspaceGroupClearsAllTabsAndRecentWorkspaces(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	wsRemove := "/non-temp-ws/remove-me"
	wsKeep := "/non-temp-ws/keep-me"
	eng := engine.NewEngine("")
	app := NewAppUI(eng, "")
	defer app.Close()

	now := time.Now()
	app.Agent.ReplaceSessions([]*agent.AgentSession{
		{ID: "r1", Title: "Active R", WorkspacePath: wsRemove, CreatedAt: now, UpdatedAt: now},
		{ID: "r2", Title: "Archived R", WorkspacePath: wsRemove, Archived: true, CreatedAt: now, UpdatedAt: now},
		{ID: "r3", Title: "Snoozed R", WorkspacePath: wsRemove, Snoozed: true, CreatedAt: now, UpdatedAt: now},
		{ID: "g1", Title: "General Task", WorkspacePath: "", CreatedAt: now, UpdatedAt: now},
	})
	app.RecentWorkspaces = []string{wsRemove, wsKeep}
	app.ProjectFilter = "remove-me"
	app.Workspace.RootDir = wsRemove

	app.deleteWorkspaceGroup(nil, "remove-me", wsRemove)

	for _, s := range app.Agent.AllSessions() {
		if s.WorkspacePath == wsRemove {
			t.Fatalf("expected session %q from deleted workspace to be removed across all tabs", s.ID)
		}
	}
	if len(app.Agent.AllSessions()) != 1 || app.Agent.AllSessions()[0].ID != "g1" {
		t.Fatalf("expected General session g1 to remain, got %v", app.Agent.AllSessions())
	}
	for _, rw := range app.RecentWorkspaces {
		if rw == wsRemove {
			t.Fatalf("expected %s removed from RecentWorkspaces", wsRemove)
		}
	}
	if len(app.RecentWorkspaces) != 1 || app.RecentWorkspaces[0] != wsKeep {
		t.Fatalf("expected RecentWorkspaces=[%s], got %v", wsKeep, app.RecentWorkspaces)
	}
	if app.ProjectFilter != "all" {
		t.Fatalf("expected ProjectFilter reset to \"all\", got %q", app.ProjectFilter)
	}
	if app.Workspace.RootDir != "" {
		t.Fatalf("expected active workspace reset to General \"\", got %q", app.Workspace.RootDir)
	}
}
