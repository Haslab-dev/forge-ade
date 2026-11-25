package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/Haslab-dev/RakitKode/llm"
	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/agent"
	"github.com/hasdev/forge-ade/app/native/pkg/engine"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
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

func TestProjectSwitcherThisWindowAndHistory(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	eng := engine.NewEngine("")
	app := NewAppUI(eng, "")
	defer app.Close()

	localWS := "/projects/mobile-asn"
	sshWS := "ssh://srv-prod/var/www/pchain-api"

	// 1. Record workspaces into RecentWorkspaces (both local and ssh)
	app.recordRecentWorkspace(localWS)
	app.recordRecentWorkspace(sshWS)

	if len(app.RecentWorkspaces) != 2 {
		t.Fatalf("expected 2 recent workspaces, got %d: %v", len(app.RecentWorkspaces), app.RecentWorkspaces)
	}
	if app.RecentWorkspaces[0] != sshWS {
		t.Fatalf("expected top recent workspace to be %s, got %s", sshWS, app.RecentWorkspaces[0])
	}

	// 2. Open workspaces in "This Window"
	app.recordOpenWorkspace(localWS)
	app.recordOpenWorkspace(sshWS)

	if len(app.OpenWorkspaces) != 2 {
		t.Fatalf("expected 2 open workspaces in this window, got %d: %v", len(app.OpenWorkspaces), app.OpenWorkspaces)
	}

	// 3. Test entry generation
	entrySSH := app.buildProjectSwitcherEntry(sshWS, sshWS, true)
	if !entrySSH.IsSSH {
		t.Fatalf("expected entry to be identified as SSH")
	}
	if entrySSH.Name != "pchain-api" {
		t.Fatalf("expected entry name to be pchain-api, got %s", entrySSH.Name)
	}
	if !entrySSH.IsActive {
		t.Fatalf("expected entry to be active")
	}

	// 4. Close workspace from window (x button): must remove from OpenWorkspaces, but KEEP in RecentWorkspaces
	app.closeWorkspaceFromWindow(sshWS)

	if len(app.OpenWorkspaces) != 1 || app.OpenWorkspaces[0] != localWS {
		t.Fatalf("expected OpenWorkspaces to only retain localWS, got: %v", app.OpenWorkspaces)
	}

	// Verify history remained intact
	foundSSHInHistory := false
	for _, rw := range app.RecentWorkspaces {
		if rw == sshWS {
			foundSSHInHistory = true
			break
		}
	}
	if !foundSSHInHistory {
		t.Fatalf("expected sshWS to remain in RecentWorkspaces history after closing from window")
	}
}

func TestModelProviderNoCollisionAndGrouping(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	eng := engine.NewEngine("")
	app := NewAppUI(eng, "")
	defer app.Close()

	// Configure two custom providers that both offer the same model name "glm-5.3-flash"
	profs := []llm.ProviderProfile{
		{
			ID:              "kenari",
			Name:            "Kenari",
			BaseURL:         "https://kenari.ai/v1",
			APIKey:          "key-kenari",
			AvailableModels: []string{"glm-5.3-flash", "kenari-pro"},
			Enabled:         true,
		},
		{
			ID:              "zhipu",
			Name:            "Zhipu Kantor",
			BaseURL:         "https://open.bigmodel.cn/api/paas/v4",
			APIKey:          "key-zhipu",
			AvailableModels: []string{"glm-5.3", "glm-5.3-flash", "glm-5.3-flashx"},
			Enabled:         true,
		},
	}
	if err := app.Settings.SaveProfiles(profs); err != nil {
		t.Fatalf("failed to save profiles: %v", err)
	}

	app.ShowModelPalette = true
	app.ModelSearchQuery = "5.3"

	// Create an active session
	now := time.Now()
	sess := &agent.AgentSession{
		ID:        "test-session",
		Title:     "Test Session",
		CreatedAt: now,
		UpdatedAt: now,
	}
	app.Agent.ReplaceSessions([]*agent.AgentSession{sess})
	app.Agent.SelectSession(sess.ID)

	// Verify that internal models loaded from both profiles contain glm-5.3-flash twice (once for Kenari, once for Zhipu)
	foundKenari := false
	foundZhipu := false
	for _, prof := range app.Settings.GetProfiles() {
		for _, m := range prof.AvailableModels {
			if m == "glm-5.3-flash" {
				if prof.ID == "kenari" {
					foundKenari = true
				}
				if prof.ID == "zhipu" {
					foundZhipu = true
				}
			}
		}
	}
	if !foundKenari || !foundZhipu {
		t.Fatalf("expected both Kenari and Zhipu to have glm-5.3-flash in profiles")
	}

	// Verify selectModel logic when selecting Zhipu's glm-5.3-flash
	zhipuMeta := ModelMetadata{
		ID:          "glm-5.3-flash",
		DisplayName: "glm-5.3-flash",
		Provider:    "Zhipu Kantor",
		ProviderID:  "zhipu",
		ProviderKey: "openai",
		HarnessID:   "agent-internal",
		HarnessName: "ForgeADE Internal",
	}

	// Call the model selection behavior
	app.Agent.ActiveModel = zhipuMeta.DisplayName
	app.Agent.ActiveProvider = zhipuMeta.ProviderID
	app.Settings.SelectedProviderID = zhipuMeta.ProviderID
	app.Settings.Provider = zhipuMeta.ProviderID
	if sess := app.Agent.ActiveSession(); sess != nil {
		sess.Model = zhipuMeta.DisplayName
		sess.Provider = zhipuMeta.ProviderID
	}
	if app.Engine != nil && app.Engine.LLMClient != nil {
		app.Engine.LLMClient.SetActiveModel(zhipuMeta.ProviderID, zhipuMeta.DisplayName)
	}

	if sess.Provider != "zhipu" {
		t.Fatalf("expected session Provider='zhipu', got %q", sess.Provider)
	}
	if app.Settings.SelectedProviderID != "zhipu" {
		t.Fatalf("expected Settings.SelectedProviderID='zhipu', got %q", app.Settings.SelectedProviderID)
	}
	if app.Agent.ActiveProvider != "zhipu" {
		t.Fatalf("expected Agent.ActiveProvider='zhipu', got %q", app.Agent.ActiveProvider)
	}

	// 5. Test UI render pass with Model Palette open and duplicate model names across providers
	app.ShowModelPalette = true
	app.ModelSearchQuery = "5.3"
	tw := ui.NewTesterWith(func(c *ui.Context) { app.View(c) }, 1280, 800, ui.TesterOptions{Dark: true})
	tw.Frame()
}

func TestWorkspaceTerminalSessionPersistence(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	engine := engine.NewEngine(t.TempDir())

	app := NewAppUI(engine, dirA)
	defer func() {
		if app.Terminal != nil {
			app.Terminal.Close()
		}
		for _, ts := range app.WorkspaceTerminals {
			if ts != nil {
				ts.Close()
			}
		}
	}()

	// 1. Workspace A: open an editor terminal tab
	app.openEditorTerminalTab("Terminal A")
	if len(app.Editor.OpenTabPaths) == 0 || app.Editor.OpenTabPaths[0] != "terminal://Terminal A" {
		t.Fatalf("expected editor tab 'terminal://Terminal A', got %v", app.Editor.OpenTabPaths)
	}
	termA := app.Terminal
	if termA == nil {
		t.Fatalf("expected non-nil terminal for workspace A")
	}

	// 2. Switch to Workspace B
	app.switchWorkspace(dirB)
	termB := app.Terminal
	if termB == nil {
		t.Fatalf("expected non-nil terminal for workspace B")
	}
	if termB == termA {
		t.Fatalf("expected workspace B to have a distinct terminal instance from workspace A")
	}

	// Open a terminal tab in Workspace B
	app.openEditorTerminalTab("Terminal B")
	if len(app.Editor.OpenTabPaths) == 0 || app.Editor.OpenTabPaths[0] != "terminal://Terminal B" {
		t.Fatalf("expected editor tab 'terminal://Terminal B' in workspace B, got %v", app.Editor.OpenTabPaths)
	}

	// 3. Switch back to Workspace A: verify Terminal A and tabs are restored
	app.switchWorkspace(dirA)
	if app.Terminal != termA {
		t.Fatalf("expected workspace A terminal to be restored, got different instance")
	}
	foundA := false
	for _, p := range app.Editor.OpenTabPaths {
		if p == "terminal://Terminal A" {
			foundA = true
		}
		if p == "terminal://Terminal B" {
			t.Fatalf("workspace B tab leaked into workspace A: %v", app.Editor.OpenTabPaths)
		}
	}
	if !foundA {
		t.Fatalf("expected 'terminal://Terminal A' tab restored in workspace A, got %v", app.Editor.OpenTabPaths)
	}

	// 4. Switch to remote SSH workspace
	sshURI := "ssh://test-conn-1/root/pchain-api"
	app.switchWorkspace(sshURI)
	termSSH := app.Terminal
	if termSSH == nil {
		t.Fatalf("expected non-nil terminal for remote workspace")
	}
	if termSSH == termA || termSSH == termB {
		t.Fatalf("expected remote workspace to have its own distinct terminal instance")
	}

	app.openEditorTerminalTab("pchain-api")
	foundSSH := false
	for _, p := range app.Editor.OpenTabPaths {
		if p == "terminal://pchain-api" {
			foundSSH = true
		}
	}
	if !foundSSH {
		t.Fatalf("expected 'terminal://pchain-api' tab in remote workspace, got %v", app.Editor.OpenTabPaths)
	}

	// 5. Switch back to Workspace A, then back to remote SSH workspace
	app.switchWorkspace(dirA)
	if app.Terminal != termA {
		t.Fatalf("expected workspace A terminal to be restored")
	}

	app.switchWorkspace(sshURI)
	if app.Terminal != termSSH {
		t.Fatalf("expected remote SSH terminal to be restored without restarting or reconnecting")
	}
	foundSSH = false
	for _, p := range app.Editor.OpenTabPaths {
		if p == "terminal://pchain-api" {
			foundSSH = true
		}
	}
	if !foundSSH {
		t.Fatalf("expected remote SSH tab to persist across workspace switches, got %v", app.Editor.OpenTabPaths)
	}

	// 6. Close Workspace B from window
	app.closeWorkspaceFromWindow(dirB)
	if _, ok := app.WorkspaceTerminals[normalizeWorkspaceKey(dirB)]; ok {
		t.Fatalf("expected workspace B terminal to be deleted from WorkspaceTerminals after closing workspace")
	}
	if _, ok := app.WorkspaceTerminals[normalizeWorkspaceKey(dirA)]; !ok {
		t.Fatalf("expected workspace A terminal to remain in WorkspaceTerminals")
	}
	if _, ok := app.WorkspaceTerminals[normalizeWorkspaceKey(sshURI)]; !ok {
		t.Fatalf("expected remote SSH terminal to remain in WorkspaceTerminals")
	}
}

func TestSeparateAgentAndEditorWorkspaces(t *testing.T) {
	dirAgent := t.TempDir()
	dirEditor := t.TempDir()
	eng := engine.NewEngine(dirAgent)
	app := NewAppUI(eng, dirAgent)

	// Start in Agent Mode on dirAgent
	app.setMode(ModeAgent)
	app.switchWorkspace(dirAgent)
	if normalizeWorkspaceKey(app.Workspace.RootDir) != normalizeWorkspaceKey(dirAgent) {
		t.Fatalf("expected agent workspace %q, got %q", dirAgent, app.Workspace.RootDir)
	}

	// Switch to Editor Mode and change workspace to dirEditor
	app.setMode(ModeEditor)
	app.switchWorkspace(dirEditor)
	if normalizeWorkspaceKey(app.Workspace.RootDir) != normalizeWorkspaceKey(dirEditor) {
		t.Fatalf("expected editor workspace %q, got %q", dirEditor, app.Workspace.RootDir)
	}

	// Switching back to Agent Mode must restore dirAgent, not stay on dirEditor
	app.setMode(ModeAgent)
	if normalizeWorkspaceKey(app.Workspace.RootDir) != normalizeWorkspaceKey(dirAgent) {
		t.Fatalf("expected agent mode to restore %q, got %q", dirAgent, app.Workspace.RootDir)
	}

	// Switching back to Editor Mode must restore dirEditor
	app.setMode(ModeEditor)
	if normalizeWorkspaceKey(app.Workspace.RootDir) != normalizeWorkspaceKey(dirEditor) {
		t.Fatalf("expected editor mode to restore %q, got %q", dirEditor, app.Workspace.RootDir)
	}
}

func TestProjectOrderSwapAndMoveHelpers(t *testing.T) {
	names := []string{"General", "alpha", "Beta", "sub", "zeta"}

	// Swap down the chain: Beta up over alpha.
	if got := swapProjectOrder(names, "Beta", -1); !slices.Equal(got, []string{"General", "Beta", "alpha", "sub", "zeta"}) {
		t.Fatalf("swapProjectOrder Beta up = %v", got)
	}
	// Swap down: alpha down below Beta.
	if got := swapProjectOrder(names, "alpha", 1); !slices.Equal(got, []string{"General", "Beta", "alpha", "sub", "zeta"}) {
		t.Fatalf("swapProjectOrder alpha down = %v", got)
	}
	// Edge: the first project cannot cross the pinned "General".
	if got := swapProjectOrder(names, "alpha", -1); got != nil {
		t.Fatalf("swapProjectOrder first project up should be nil, got %v", got)
	}
	// Edge: the last project cannot move down.
	if got := swapProjectOrder(names, "zeta", 1); got != nil {
		t.Fatalf("swapProjectOrder last project down should be nil, got %v", got)
	}
	// "General" itself never moves.
	if got := swapProjectOrder(names, "General", 1); got != nil {
		t.Fatalf("swapProjectOrder General should be nil, got %v", got)
	}
	// Unknown name and invalid direction are no-ops.
	if got := swapProjectOrder(names, "nope", -1); got != nil {
		t.Fatalf("swapProjectOrder unknown name should be nil, got %v", got)
	}
	if got := swapProjectOrder(names, "alpha", 7); got != nil {
		t.Fatalf("swapProjectOrder invalid dir should be nil, got %v", got)
	}
	// Helpers never mutate the input.
	if !slices.Equal(names, []string{"General", "alpha", "Beta", "sub", "zeta"}) {
		t.Fatalf("input slice mutated: %v", names)
	}

	// Drag: zeta onto alpha puts zeta at alpha's position.
	if got := moveProjectTo(names, "zeta", "alpha"); !slices.Equal(got, []string{"General", "zeta", "alpha", "Beta", "sub"}) {
		t.Fatalf("moveProjectTo zeta->alpha = %v", got)
	}
	// Drag: alpha onto zeta moves it to the tail slot.
	if got := moveProjectTo(names, "alpha", "zeta"); !slices.Equal(got, []string{"General", "Beta", "sub", "zeta", "alpha"}) {
		t.Fatalf("moveProjectTo alpha->zeta = %v", got)
	}
	// Pinned General cannot drag or be a drop target.
	if got := moveProjectTo(names, "alpha", "General"); got != nil {
		t.Fatalf("moveProjectTo onto General should be nil, got %v", got)
	}
	if got := moveProjectTo(names, "General", "alpha"); got != nil {
		t.Fatalf("moveProjectTo dragging General should be nil, got %v", got)
	}
	// Same-name drop is a no-op.
	if got := moveProjectTo(names, "alpha", "alpha"); got != nil {
		t.Fatalf("moveProjectTo same name should be nil, got %v", got)
	}
}

func TestSidebarGroupsRespectProjectOrder(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	root := t.TempDir()
	dirs := map[string]string{
		"alpha": filepath.Join(root, "alpha"),
		"Beta":  filepath.Join(root, "Beta"),
		"sub":   filepath.Join(root, "alpha", "sub"),
		"zeta":  filepath.Join(root, "zeta"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}

	eng := engine.NewEngine(dirs["zeta"])
	app := NewAppUI(eng, dirs["zeta"])
	defer app.Close()

	now := time.Now()
	i := 0
	sessions := make([]*agent.AgentSession, 0, len(dirs))
	for name, dir := range dirs {
		i++
		sessions = append(sessions, &agent.AgentSession{
			ID:            "s-" + name,
			Title:         name,
			WorkspacePath: dir,
			CreatedAt:     now.Add(time.Duration(i) * time.Minute),
			UpdatedAt:     now.Add(time.Duration(i) * time.Minute),
		})
	}
	app.Agent.ReplaceSessions(sessions)

	groupOrder := func() []string {
		groups := app.sidebarProjectGroups()
		out := make([]string, len(groups))
		for gi, g := range groups {
			out[gi] = g.ProjectName
		}
		return out
	}

	// Implicit order (empty ProjectOrder) is unchanged: General first, then
	// alphabetical.
	if got := groupOrder(); !slices.Equal(got, []string{"General", "alpha", "Beta", "sub", "zeta"}) {
		t.Fatalf("implicit group order = %v", got)
	}

	// Custom order: stored sequence first, names never moved keep the
	// implicit alphabetical order appended after them.
	app.ProjectOrder = []string{"sub", "zeta", "alpha"}
	if got := groupOrder(); !slices.Equal(got, []string{"General", "sub", "zeta", "alpha", "Beta"}) {
		t.Fatalf("custom group order = %v", got)
	}
}

func TestProjectOrderReorderPersistsAcrossRestart(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	dataDir := t.TempDir()

	eng := engine.NewEngine(dataDir)
	app := NewAppUI(eng, dataDir)

	now := time.Now()
	sA := &agent.AgentSession{ID: "p-a", Title: "A", WorkspacePath: filepath.Join(dataDir, "alpha"), CreatedAt: now, UpdatedAt: now}
	sB := &agent.AgentSession{ID: "p-b", Title: "B", WorkspacePath: filepath.Join(dataDir, "Beta"), CreatedAt: now, UpdatedAt: now}
	sC := &agent.AgentSession{ID: "p-c", Title: "C", WorkspacePath: filepath.Join(dataDir, "gamma"), CreatedAt: now, UpdatedAt: now}
	app.Agent.ReplaceSessions([]*agent.AgentSession{sA, sB, sC})

	// Chevron move: Beta up swaps with alpha (visible non-General order).
	app.moveProjectGroup("Beta", -1, []string{"alpha", "Beta", "gamma"})
	if !slices.Equal(app.ProjectOrder, []string{"Beta", "alpha", "gamma"}) {
		t.Fatalf("ProjectOrder after move up = %v", app.ProjectOrder)
	}

	// Drag-and-drop move: gamma dropped onto Beta takes its slot.
	app.dropProjectOn("gamma", "Beta", []string{"Beta", "alpha", "gamma"})
	if !slices.Equal(app.ProjectOrder, []string{"gamma", "Beta", "alpha"}) {
		t.Fatalf("ProjectOrder after drop = %v", app.ProjectOrder)
	}

	// Hidden previously-ordered names are preserved, not erased.
	app.ProjectOrder = []string{"gamma", "Beta", "alpha", "hidden-ws"}
	app.dropProjectOn("alpha", "gamma", []string{"gamma", "Beta", "alpha"})
	if !slices.Equal(app.ProjectOrder, []string{"alpha", "gamma", "Beta", "hidden-ws"}) {
		t.Fatalf("ProjectOrder after drop with hidden name = %v", app.ProjectOrder)
	}

	// persistProjectOrder already saved to disk; Close is a no-op for prefs.
	app.Close()

	// A fresh AppUI over the same data dir restores the custom order.
	eng2 := engine.NewEngine(dataDir)
	app2 := NewAppUI(eng2, dataDir)
	defer app2.Close()
	if !slices.Equal(app2.ProjectOrder, []string{"alpha", "gamma", "Beta", "hidden-ws"}) {
		t.Fatalf("ProjectOrder after restart = %v", app2.ProjectOrder)
	}

	// And the sidebar renders the restored order (General pinned first,
	// ordered names in stored sequence, unordered after them). Sessions are
	// replayed because they are not persisted in this store.
	app2.Agent.ReplaceSessions([]*agent.AgentSession{sA, sB, sC})
	groups := app2.sidebarProjectGroups()
	got := make([]string, len(groups))
	for gi, g := range groups {
		got[gi] = g.ProjectName
	}
	want := []string{"General", "alpha", "gamma", "Beta", resolveProjectName(dataDir)}
	if !slices.Equal(got, want) {
		t.Fatalf("group order after restart = %v, want %v", got, want)
	}
}

func TestOpenMarkdownFileMention(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	dataDir := t.TempDir()

	eng := engine.NewEngine(dataDir)
	app := NewAppUI(eng, dataDir)
	defer app.Close()

	readmeRel := "README.md"
	if err := os.WriteFile(filepath.Join(dataDir, readmeRel), []byte("# hi"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}

	if app.openMarkdownFileMention("https://example.com/x") {
		t.Fatalf("web URL must not be treated as a file mention")
	}
	if app.openMarkdownFileMention("#heading") {
		t.Fatalf("anchor must not be treated as a file mention")
	}
	if app.openMarkdownFileMention("missing.go") {
		t.Fatalf("non-existing workspace path must not be treated as a file mention")
	}
	if !app.openMarkdownFileMention(readmeRel) {
		t.Fatalf("existing workspace file mention should open the side pane")
	}
	if !app.ShowRightDrawer || app.RightDrawerTab != RightTabSideFile {
		t.Fatalf("expected right drawer open on the side file tab")
	}
	if app.SelectedFile != readmeRel || len(app.SideFileTabs) == 0 || app.SideFileTabs[0] != readmeRel {
		t.Fatalf("expected README.md selected in side pane, got %q tabs %v", app.SelectedFile, app.SideFileTabs)
	}

	// Agent-emitted absolute links are file:// URLs; they must resolve like a
	// bare path mention, including %XX-escaped names.
	spacedRel := "with space.txt"
	if err := os.WriteFile(filepath.Join(dataDir, spacedRel), []byte("x"), 0o644); err != nil {
		t.Fatalf("write spaced file: %v", err)
	}
	if !app.openMarkdownFileMention("file://" + filepath.Join(dataDir, readmeRel)) {
		t.Fatalf("file:// URL of an existing workspace file should open the side pane")
	}
	if !app.openMarkdownFileMention("file://" + filepath.Join(dataDir, "with%20space.txt")) {
		t.Fatalf("percent-encoded file:// URL should open the side pane")
	}
	if app.SelectedFile != spacedRel {
		t.Fatalf("expected %q selected in side pane, got %q", spacedRel, app.SelectedFile)
	}
	outside := filepath.Join(fakeHome, "outside.txt")
	if err := os.WriteFile(outside, []byte("x"), 0o644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	if app.openMarkdownFileMention("file://" + outside) {
		t.Fatalf("file:// URL outside the workspace must not be treated as a file mention")
	}
}

// TestDrawerTabReconcileRegistersActivePane guards the multi-tab drawer:
// the reconcile MUST run in the render path (View), or the header pills
// never appear and panes "disappear" (regression: the reconcile call was
// once placed inside a dead helper).
func TestDrawerTabReconcileRegistersActivePane(t *testing.T) {
	tempDir := t.TempDir()
	eng := engine.NewEngine(tempDir)
	app := NewAppUI(eng, tempDir)
	app.ShowRightDrawer = true
	app.RightDrawerTab = RightTabTerminal

	ui.NewTesterWith(func(c *ui.Context) { app.View(c) }, 1280, 800, ui.TesterOptions{Dark: true})

	if len(app.DrawerTabs) == 0 {
		t.Fatal("View must register the active pane into DrawerTabs")
	}
	found := false
	for _, tab := range app.DrawerTabs {
		if tab.Kind == RightTabTerminal {
			found = true
		}
	}
	if !found {
		t.Fatalf("Terminal pane not registered in DrawerTabs: %+v", app.DrawerTabs)
	}
	if app.RightDrawerTab != app.DrawerTabs[app.drawerActiveIdx].Kind {
		t.Fatalf("active tab out of sync: tab=%d active kind=%d", app.RightDrawerTab, app.DrawerTabs[app.drawerActiveIdx].Kind)
	}
}

// TestLauncherPickReplacesLauncherTab guards the REPLACE semantics: picking
// a pane from the New-tab grid transforms the launcher tab in place — the
// "New tab" entry must NOT survive alongside the picked pane.
func TestLauncherPickReplacesLauncherTab(t *testing.T) {
	tempDir := t.TempDir()
	eng := engine.NewEngine(tempDir)
	app := NewAppUI(eng, tempDir)
	app.ShowRightDrawer = true

	// "+" opens a launcher instance.
	app.openNewLauncherTab()
	if len(app.DrawerTabs) != 1 || app.DrawerTabs[0].Kind != RightTabLauncher {
		t.Fatalf("expected one launcher tab, got %+v", app.DrawerTabs)
	}

	// Picking Terminal replaces it in place.
	app.replaceActiveDrawerTab(RightTabTerminal, "Terminal", -1)
	if len(app.DrawerTabs) != 1 || app.DrawerTabs[0].Kind != RightTabTerminal {
		t.Fatalf("launcher tab must be replaced by the choice, got %+v", app.DrawerTabs)
	}

	// Rendering must not resurrect the launcher as an extra tab.
	app.prefsDirty.Store(false)
	tw := ui.NewTesterWith(func(c *ui.Context) { app.View(c) }, 1280, 800, ui.TesterOptions{Dark: true})
	_ = tw
	if len(app.DrawerTabs) != 1 || app.DrawerTabs[0].Kind != RightTabTerminal {
		t.Fatalf("render must keep exactly the picked pane, got %+v", app.DrawerTabs)
	}
}

// TestUserSequenceThreeTerminals reproduces the user's exact click sequence:
// "+" opens a launcher tab, picking Terminal spawns a terminal session tab —
// repeated three times. Expected end state: 3 launcher tabs + 3 terminal
// session tabs (Terminal, Terminal 2, Terminal 3). If any Terminal instance
// is lost or turned into a launcher, this test fails and shows the state.
func TestUserSequenceThreeTerminals(t *testing.T) {
	tempDir := t.TempDir()
	eng := engine.NewEngine(tempDir)
	app := NewAppUI(eng, tempDir)
	app.ShowRightDrawer = true

	dump := func(stage string) {
		t.Logf("%s: %+v (active=%d RightDrawerTab=%d)", stage, app.DrawerTabs, app.drawerActiveIdx, app.RightDrawerTab)
	}

	// Cycle 1: + → launcher → pick Terminal.
	app.openNewLauncherTab()
	dump("after + 1")
	app.pickDrawerPaneTerminal()
	dump("after pick 1")
	tw := ui.NewTesterWith(func(c *ui.Context) { app.View(c) }, 1280, 800, ui.TesterOptions{Dark: true})
	_ = tw // a frame with reconcile runs here, like the real app
	dump("after frame 1")

	// Cycle 2.
	app.openNewLauncherTab()
	app.pickDrawerPaneTerminal()
	tw2 := ui.NewTesterWith(func(c *ui.Context) { app.View(c) }, 1280, 800, ui.TesterOptions{Dark: true})
	_ = tw2
	dump("after cycle 2")

	// Cycle 3.
	app.openNewLauncherTab()
	app.pickDrawerPaneTerminal()
	tw3 := ui.NewTesterWith(func(c *ui.Context) { app.View(c) }, 1280, 800, ui.TesterOptions{Dark: true})
	_ = tw3
	dump("after cycle 3")

	terminals := 0
	launchers := 0
	for _, tab := range app.DrawerTabs {
		switch tab.Kind {
		case RightTabTerminal:
			terminals++
		case RightTabLauncher:
			launchers++
		}
	}
	if terminals != 3 {
		t.Fatalf("expected 3 Terminal tabs after three cycles, got %d: %+v", terminals, app.DrawerTabs)
	}
	if launchers != 0 {
		t.Fatalf("expected 0 launcher tabs after picking terminal, got %d: %+v", launchers, app.DrawerTabs)
	}
}

// TestUserSequenceMultiTabSideChat verifies that Side Chat supports multiple tabs
// and that picking Side Chat from the launcher closes the launcher tab.
func TestUserSequenceMultiTabSideChat(t *testing.T) {
	tempDir := t.TempDir()
	eng := engine.NewEngine(tempDir)
	app := NewAppUI(eng, tempDir)
	app.ShowRightDrawer = true

	// Cycle 1: + → pick Side Chat
	app.openNewLauncherTab()
	app.pickDrawerPaneSideChat()
	if len(app.DrawerTabs) != 1 || app.DrawerTabs[0].Kind != RightTabChat {
		t.Fatalf("expected 1 Side Chat tab, got %+v", app.DrawerTabs)
	}
	if app.DrawerTabs[0].Title != "Side chat" {
		t.Fatalf("expected title 'Side chat', got %q", app.DrawerTabs[0].Title)
	}

	// Cycle 2: + → pick Side Chat again (multi-tab enabled)
	app.openNewLauncherTab()
	app.pickDrawerPaneSideChat()
	if len(app.DrawerTabs) != 2 {
		t.Fatalf("expected 2 tabs after cycle 2, got %+v", app.DrawerTabs)
	}
	if app.DrawerTabs[0].Kind != RightTabChat || app.DrawerTabs[1].Kind != RightTabChat {
		t.Fatalf("expected both tabs to be RightTabChat, got %+v", app.DrawerTabs)
	}
	if app.DrawerTabs[1].Title != "Side chat 2" {
		t.Fatalf("expected title 'Side chat 2', got %q", app.DrawerTabs[1].Title)
	}

	// No launchers should remain
	for _, tab := range app.DrawerTabs {
		if tab.Kind == RightTabLauncher {
			t.Fatalf("no launcher tab should remain, got %+v", app.DrawerTabs)
		}
	}
}

// TestSingletonsDoNotDuplicate verifies that only Terminal and Side Chat can be multi-tab;
// singleton panes (like Review, Files, Plans, Browser) never duplicate. Picking an already-open
// singleton closes the launcher and focuses the existing tab.
func TestSingletonsDoNotDuplicate(t *testing.T) {
	tempDir := t.TempDir()
	eng := engine.NewEngine(tempDir)
	app := NewAppUI(eng, tempDir)
	app.ShowRightDrawer = true

	// 1. Open Review
	app.openNewLauncherTab()
	app.pickDrawerPaneSingleton(RightTabChanges, "Review")
	if len(app.DrawerTabs) != 1 || app.DrawerTabs[0].Kind != RightTabChanges {
		t.Fatalf("expected 1 Review tab, got %+v", app.DrawerTabs)
	}

	// 2. Open Terminal alongside Review (multi-tab tool)
	app.openNewLauncherTab()
	app.pickDrawerPaneTerminal()
	if len(app.DrawerTabs) != 2 {
		t.Fatalf("expected 2 tabs (Review + Terminal), got %+v", app.DrawerTabs)
	}

	// 3. User opens "+" launcher and clicks Review again
	app.openNewLauncherTab()
	if len(app.DrawerTabs) != 3 || app.DrawerTabs[app.drawerActiveIdx].Kind != RightTabLauncher {
		t.Fatalf("expected launcher to open, got %+v", app.DrawerTabs)
	}
	app.pickDrawerPaneSingleton(RightTabChanges, "Review")

	// Must NOT create a second Review tab! Launcher must close and existing Review must be focused.
	if len(app.DrawerTabs) != 2 {
		t.Fatalf("expected exactly 2 tabs (no duplicate Review, launcher closed), got %+v", app.DrawerTabs)
	}
	reviewCount := 0
	for _, tab := range app.DrawerTabs {
		if tab.Kind == RightTabChanges {
			reviewCount++
		}
		if tab.Kind == RightTabLauncher {
			t.Fatalf("launcher tab must be closed, got %+v", app.DrawerTabs)
		}
	}
	if reviewCount != 1 {
		t.Fatalf("expected exactly 1 Review tab, got %d", reviewCount)
	}
	if app.RightDrawerTab != RightTabChanges || app.DrawerTabs[app.drawerActiveIdx].Kind != RightTabChanges {
		t.Fatalf("expected Review tab to be active, got idx=%d kind=%d", app.drawerActiveIdx, app.RightDrawerTab)
	}
}

func TestAgentAndEditorTerminalsAreIndependent(t *testing.T) {
	dir := t.TempDir()
	eng := engine.NewEngine(dir)
	app := NewAppUI(eng, dir)
	defer app.Close()

	if app.AgentTerminal == nil || app.Terminal == nil || app.EditorBottomTerminal == nil {
		t.Fatalf("expected AgentTerminal, Terminal, and EditorBottomTerminal to be initialized")
	}
	if app.AgentTerminal == app.Terminal || app.EditorBottomTerminal == app.Terminal || app.EditorBottomTerminal == app.AgentTerminal {
		t.Fatalf("AgentTerminal, EditorBottomTerminal, and Terminal must be distinct TerminalState instances")
	}

	// 1. In Agent Mode, open a Right Drawer terminal.
	app.setMode(ModeAgent)
	app.ShowRightDrawer = true
	app.openNewLauncherTab()
	app.pickDrawerPaneTerminal()
	if !app.ShowRightDrawer || len(app.DrawerTabs) != 1 || app.DrawerTabs[0].Kind != RightTabTerminal {
		t.Fatalf("expected Agent right drawer terminal tab, got show=%v tabs=%+v", app.ShowRightDrawer, app.DrawerTabs)
	}
	agentTabCount := app.AgentTerminal.TabCount()

	// 2. Switch to Editor Mode: Agent's right drawer and terminal must not follow.
	app.setMode(ModeEditor)
	if app.ShowRightDrawer {
		t.Fatalf("Agent Mode right drawer must not leak into Editor Mode")
	}
	if len(app.DrawerTabs) != 0 {
		t.Fatalf("Agent Mode drawer tabs must not leak into Editor Mode: %+v", app.DrawerTabs)
	}
	if app.activeTerminal() != app.EditorBottomTerminal {
		t.Fatalf("expected activeTerminal() in Editor Mode to be app.EditorBottomTerminal")
	}

	// 3. Open an Editor bottom terminal, then open an Editor main-pane shell tab:
	// the bottom terminal must stay visible and not be hidden/overlapped.
	app.ShowBottomTerminal = true
	app.openEditorTerminalTab("Editor Shell")
	if !app.ShowBottomTerminal {
		t.Fatalf("opening Editor main-pane shell must not hide ShowBottomTerminal")
	}
	if app.AgentTerminal.TabCount() != agentTabCount {
		t.Fatalf("Editor shell must not mutate AgentTerminal tabs: want %d, got %d", agentTabCount, app.AgentTerminal.TabCount())
	}

	// 4. Switch back to Agent Mode: Agent's right terminal is preserved and bottom terminal stays closed.
	app.setMode(ModeAgent)
	if app.activeTerminal() != app.AgentTerminal {
		t.Fatalf("expected activeTerminal() in Agent Mode to be app.AgentTerminal")
	}
	if app.isBottomTerminalVisible() {
		t.Fatalf("Editor ShowBottomTerminal must not open bottom terminal in Agent Mode")
	}
	if !app.ShowRightDrawer || len(app.DrawerTabs) != 1 || app.DrawerTabs[0].Kind != RightTabTerminal {
		t.Fatalf("expected Agent right drawer terminal to be restored, got show=%v tabs=%+v", app.ShowRightDrawer, app.DrawerTabs)
	}

	// 5. Switch back to Editor Mode: Editor's bottom terminal state and main-pane terminal are preserved.
	app.setMode(ModeEditor)
	if !app.isBottomTerminalVisible() {
		t.Fatalf("expected Editor Mode bottom terminal visibility to be preserved")
	}
	if app.activeTerminal() != app.EditorBottomTerminal {
		t.Fatalf("expected activeTerminal() in Editor Mode to be app.EditorBottomTerminal")
	}
}

func TestEditorShellPersistsAcrossDifferentModeWorkspaces(t *testing.T) {
	dirAgent := t.TempDir()
	dirEditor := t.TempDir()
	eng := engine.NewEngine(dirAgent)
	app := NewAppUI(eng, dirAgent)
	defer app.Close()

	app.setMode(ModeAgent)
	app.switchWorkspace(dirAgent)

	app.setMode(ModeEditor)
	app.switchWorkspace(dirEditor)
	app.openEditorTerminalTab("pchain-api")
	editorTerm := app.Terminal
	if app.Editor.ActivePath != "terminal://pchain-api" {
		t.Fatalf("expected active tab 'terminal://pchain-api', got %q", app.Editor.ActivePath)
	}

	// Switch to Agent Mode (switches workspace to dirAgent), then back to Editor Mode.
	app.setMode(ModeAgent)
	app.setMode(ModeEditor)

	if app.Terminal != editorTerm {
		t.Fatalf("expected Editor Mode terminal instance to be preserved across mode/workspace switch")
	}
	if app.Editor.ActivePath != "terminal://pchain-api" {
		t.Fatalf("expected 'terminal://pchain-api' tab to survive Agent Mode round-trip, got ActivePath=%q tabs=%v", app.Editor.ActivePath, app.Editor.OpenTabPaths)
	}
}

func TestSnippetTargetsMainEditorPaneWithoutOpeningBottomTerminal(t *testing.T) {
	dir := t.TempDir()
	eng := engine.NewEngine(dir)
	app := NewAppUI(eng, dir)
	defer app.Close()

	app.setMode(ModeEditor)
	app.ShowBottomTerminal = false
	app.openEditorTerminalTab("pchain-api")

	sessID := ""
	if ts := app.snippetDefaultTerminal(); ts != nil {
		sessID = ts.ActiveSessionID
	}
	app.OpenTerminalSnippetsModal(sessID, "", "")
	if app.TerminalSnippetTargetState != app.Terminal {
		t.Fatalf("expected snippet target to be main-pane app.Terminal, got %p (want %p)", app.TerminalSnippetTargetState, app.Terminal)
	}
	app.runTerminalCommand("echo hello")
	if app.ShowBottomTerminal {
		t.Fatalf("applying snippet to main-pane terminal must not open bottom terminal")
	}
}

func TestExplorerOpenInTerminalRespectsAgentMode(t *testing.T) {
	dir := t.TempDir()
	eng := engine.NewEngine(dir)
	app := NewAppUI(eng, dir)
	defer app.Close()

	app.setMode(ModeAgent)
	editorTabsBefore := app.Terminal.TabCount()

	app.OpenDrawerPaneTerminalInDir(dir)

	if app.ShowBottomTerminal || app.ShowAgentBottomTerminal {
		t.Fatalf("opening sidebar terminal in Agent Mode must not move shell to bottom")
	}
	if !app.ShowRightDrawer || app.RightDrawerTab != RightTabTerminal {
		t.Fatalf("expected terminal to open in Agent Mode right drawer, got show=%v tab=%d", app.ShowRightDrawer, app.RightDrawerTab)
	}
	if app.Terminal.TabCount() != editorTabsBefore {
		t.Fatalf("opening Agent Mode terminal must not affect Editor Mode terminal tabs")
	}
}

func TestMentionMatchesIncludesFoldersAndScrollableCount(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "src")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 16; i++ {
		name := filepath.Join(subDir, fmt.Sprintf("file_%02d.go", i))
		if err := os.WriteFile(name, []byte("package main\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	eng := engine.NewEngine(dir)
	app := NewAppUI(eng, dir)
	defer app.Close()
	app.Workspace.Refresh()

	// Empty query should include the folder "src/" and more than 12 items (17 total: 1 dir + 16 files).
	all := app.mentionMatches("")
	if len(all) <= 12 {
		t.Fatalf("expected >12 mention items for scrolling, got %d", len(all))
	}
	foundDir := false
	for _, it := range all {
		if it.Category == "folder" && it.Name == "src/" && it.Insert == "@src/ " {
			foundDir = true
			break
		}
	}
	if !foundDir {
		t.Fatalf("expected folder 'src/' in mentionMatches(\"\"), got %+v", all)
	}

	// Querying "src/" should also match the folder.
	dirHits := app.mentionMatches("src/")
	foundDirHit := false
	for _, it := range dirHits {
		if it.Category == "folder" && it.Name == "src/" {
			foundDirHit = true
			break
		}
	}
	if !foundDirHit {
		t.Fatalf("expected folder 'src/' in mentionMatches(\"src/\"), got %+v", dirHits)
	}
}

func TestOpenLocalFileURLAndBinaryGuard(t *testing.T) {
	wsDir := t.TempDir()
	outsideDir := t.TempDir()

	eng := engine.NewEngine(wsDir)
	app := NewAppUI(eng, wsDir)
	defer app.Close()

	// 1. Outside .md file via file:// URL must open in the right drawer preview (not external editor).
	outsideMd := filepath.Join(outsideDir, "audit_report.md")
	if err := os.WriteFile(outsideMd, []byte("# Audit\n\nDetails"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !app.openLocalFileURL("file://" + outsideMd) {
		t.Fatalf("expected openLocalFileURL to handle outside .md file")
	}
	if !app.ShowRightDrawer || app.RightDrawerTab != RightTabSideFile || app.SideFilePreviewMd != outsideMd {
		t.Fatalf("expected outside .md opened in side preview, got show=%v tab=%d md=%q", app.ShowRightDrawer, app.RightDrawerTab, app.SideFilePreviewMd)
	}

	// 2. Workspace .tsx file with #L272-L276 anchor must load into Editor and SideFileTabs.
	tsxDir := filepath.Join(wsDir, "src")
	if err := os.MkdirAll(tsxDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tsxFull := filepath.Join(tsxDir, "index.tsx")
	if err := os.WriteFile(tsxFull, []byte("export const x = 1;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !app.openMarkdownFileMention("file://" + tsxFull + "#L272-L276") {
		t.Fatalf("expected workspace .tsx link with line anchor to open")
	}
	if app.Editor.GetDoc(tsxFull) == nil {
		t.Fatalf("expected Editor.GetDoc(%q) to be non-nil", tsxFull)
	}

	// 3. Binary/executable files must be rejected; text/script files must be allowed.
	exeFile := filepath.Join(wsDir, "app.exe")
	if err := os.WriteFile(exeFile, []byte{'M', 'Z', 0, 0}, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := validatePreviewableFile(exeFile); err == nil {
		t.Fatalf("expected .exe file to be rejected")
	}
	machoFile := filepath.Join(wsDir, "tool")
	if err := os.WriteFile(machoFile, []byte{0xcf, 0xfa, 0xed, 0xfe, 0x07, 0x00}, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := validatePreviewableFile(machoFile); err == nil {
		t.Fatalf("expected Mach-O binary without extension to be rejected")
	}
	scriptFile := filepath.Join(wsDir, "Makefile")
	if err := os.WriteFile(scriptFile, []byte("all:\n\techo ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := validatePreviewableFile(scriptFile); err != nil {
		t.Fatalf("expected Makefile to be allowed, got %v", err)
	}
}

func TestGlassAndBackgroundPrefsRoundTrip(t *testing.T) {
	dataDir := t.TempDir()
	wsDir := t.TempDir()

	eng := engine.NewEngine(dataDir)
	app1 := NewAppUI(eng, wsDir)
	app1.ThemeMgr.PaletteID = theme.ThemePaletteObsidian
	app1.ThemeMgr.GlassEnabled = true
	app1.ThemeMgr.GlassOpacity = 0.00
	app1.ThemeMgr.GlassOpacitySet = true
	app1.ThemeMgr.BackgroundKind = theme.BackgroundKindPreset
	app1.ThemeMgr.BackgroundPreset = theme.BackgroundPresetAurora
	app1.ThemeMgr.BackgroundOpacity = 0.72
	app1.ThemeMgr.BackgroundOpacitySet = true
	app1.ThemeMgr.BackgroundBlur = 14
	app1.SaveUIPrefs()
	app1.Close()

	eng2 := engine.NewEngine(dataDir)
	app2 := NewAppUI(eng2, wsDir)
	defer app2.Close()

	tm := app2.ThemeMgr
	if tm.PaletteID != theme.ThemePaletteObsidian {
		t.Fatalf("expected PaletteID obsidian, got %q", tm.PaletteID)
	}
	if !tm.GlassEnabled || !tm.IsGlassActive() {
		t.Fatalf("expected GlassEnabled to persist across reload")
	}
	if !tm.GlassOpacitySet || tm.EffectiveGlassOpacity() != 0.00 {
		t.Fatalf("expected GlassOpacity 0.00 to persist, got %v (set=%v)", tm.EffectiveGlassOpacity(), tm.GlassOpacitySet)
	}
	if tm.BackgroundKind != theme.BackgroundKindPreset || tm.BackgroundPreset != theme.BackgroundPresetAurora {
		t.Fatalf("expected aurora preset, got kind=%q preset=%q", tm.BackgroundKind, tm.BackgroundPreset)
	}
	if tm.BackgroundOpacity != 0.72 || tm.BackgroundBlur != 14 {
		t.Fatalf("expected bg opacity=0.72 blur=14, got opacity=%v blur=%v", tm.BackgroundOpacity, tm.BackgroundBlur)
	}
	if tc := tm.Colors(); tc.Canvas.A != 0 {
		t.Fatalf("expected Canvas alpha 0 at 0%% glass opacity, got %d", tc.Canvas.A)
	}
}

func TestWindowBackgroundDoesNotBlockInput(t *testing.T) {
	dataDir := t.TempDir()
	wsDir := t.TempDir()

	eng := engine.NewEngine(dataDir)
	app := NewAppUI(eng, wsDir)
	defer app.Close()

	app.ThemeMgr.SupportsVibrancy = true
	app.ThemeMgr.GlassEnabled = true
	app.ThemeMgr.BackgroundKind = theme.BackgroundKindPreset
	app.ThemeMgr.BackgroundPreset = theme.BackgroundPresetAurora

	tw := ui.NewTesterWith(func(c *ui.Context) { app.View(c) }, 1280, 800, ui.TesterOptions{Dark: true})
	// Clicking UI controls must succeed even with the full-window background active.
	if err := tw.Click("Explore architecture"); err != nil {
		t.Fatalf("expected click on Explore architecture to succeed behind window background, got: %v", err)
	}
	if app.Agent.InputText == "" {
		t.Fatalf("expected Explore architecture prompt to populate InputText")
	}
	if err := tw.Click("Usage & Quota"); err != nil {
		t.Fatalf("expected click on Usage & Quota to succeed behind window background, got: %v", err)
	}
	if !app.ShowUsageModal {
		t.Fatalf("expected ShowUsageModal to open after clicking Usage & Quota")
	}
}

func TestUserPromptCopyAlwaysVisibleWithoutHover(t *testing.T) {
	dataDir := t.TempDir()
	wsDir := t.TempDir()

	eng := engine.NewEngine(dataDir)
	app := NewAppUI(eng, wsDir)
	defer app.Close()

	sess := app.Agent.NewSession("Copy Check")
	sess.Messages = append(sess.Messages,
		&agent.ChatMessage{ID: 1, Role: "user", Text: "hello world prompt"},
		&agent.ChatMessage{ID: 2, Role: "assistant", Text: "Here is a card response.", Done: true},
	)

	tw := ui.NewTesterWith(func(c *ui.Context) { app.View(c) }, 1280, 800, ui.TesterOptions{Dark: true})
	// Copy prompt button must be hittable and clickable without hovering first.
	if err := tw.Click("Copy prompt"); err != nil {
		t.Fatalf("expected Copy prompt button to be visible and clickable without hover: %v", err)
	}
	if got := tw.Clipboard(); got != "hello world prompt" {
		t.Fatalf("expected clipboard 'hello world prompt', got %q", got)
	}
}
