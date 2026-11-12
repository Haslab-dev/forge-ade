package ui

import (
	"fmt"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/agent"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/board"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/database"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/editor"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/engine"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/git"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/palette"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/settings"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/terminal"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/theme"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/workspace"
)

// Application surface modes
const (
	ModeAgent  = 0
	ModeEditor = 1
)

// Agent sidebar view modes
const (
	AgentSidebarTasks = 0
	AgentSidebarFiles = 1
)

// Editor activity bar tabs
const (
	ActivityExplorer = 0
	ActivityGit      = 1
	ActivityDatabase = 2
)

// Right drawer tab constants
const (
	RightTabChanges  = 0
	RightTabTerminal = 1
	RightTabChat     = 2
	RightTabTasks    = 3
	RightTabBoard    = 4
	RightTabGit      = 5
	RightTabDatabase = 6
)

// Layout metrics of ForgeADE's CSS
const (
	WorkspaceTopBarHeight = float32(52)
	RightDrawerDefault    = float32(420)
	LeftSidebarDefault    = float32(260)
	TerminalHeaderHeight  = float32(36)
	StatusBarHeight       = float32(24)
	ActivityBarWidth      = float32(48)
	FileTreeDefaultWidth  = float32(230)
	FileTreeRowHeight     = float32(22)
	EditorTabBarHeight    = float32(35)
)

// AppUI is the top-level native UI application state coordinating the modular domain engines.
type AppUI struct {
	Engine               *engine.Engine
	Window               *mygo.Window
	Workspace            *workspace.WorkspaceState
	Editor               *editor.EditorManager
	Terminal             *terminal.TerminalState
	Agent                *agent.AgentState
	Git                  *git.GitManager
	Database             *database.DatabaseManager
	Palette              *palette.PaletteManager
	Settings             *settings.SettingsManager
	ThemeMgr             *theme.ThemeManager
	ACPMgr               *agent.ACPManager
	CLIMgr               *agent.AgentCLIManager
	Board                *board.BoardManager
	Mode                 int     // ModeAgent or ModeEditor
	ActiveTaskKind       string  // "agent" or "cli"
	ShowLeftSidebar      bool    // Left sidebar visibility
	AgentSidebarView     int     // AgentSidebarTasks or AgentSidebarFiles
	ShowBottomTerminal   bool    // Collapsible bottom terminal drawer
	BottomTerminalHeight float32 // Resizable terminal height
	ShowRightDrawer      bool    // Collapsible right side pane
	RightDrawerWidth     float32 // Resizable right pane width
	LeftSidebarWidth     float32 // Resizable left sidebar width
	RightDrawerTab       int     // RightTabChanges, RightTabTerminal, RightTabChat, etc.
	EditorActivity       int     // ActivityExplorer, ActivityGit, ActivityDatabase
	SelectedFile         string
	SelectedDiffPath     string
	SideChatMessages     []*agent.ChatMessage
	SideChatInput        string
	SideChatModel        string
	SideChatMode         string // "full", "ask", "plan"
	SideChatStreaming    bool
	SideChatMd           markdown.Options
	ShowNewFileModal     bool
	NewFileName          string
	ShowNewFolderModal   bool
	NewFolderName        string
	ShowNewTaskModal        bool
	NewTaskTitle            string
	NewTaskAgentType        string // "acp" or "cli"
	NewTaskSearchQuery      string
	NewTaskFilterTab        string // "all", "acp", "cli"
	NewTaskSelectedKind     string // "acp", "cli"
	NewTaskSelectedAgentID  string
	NewTaskSelectedName     string
	NewTaskSelectedExec     string
	NewTaskSelectedArgs     []string
	NewTaskUninstalledOpen  bool
	NewTaskStep             string // "agent" or "workspace"
	SelectedAgentID         string
	SelectedAgentModel   string
	ShowTaskMenuModal    bool
	ShowRenameModal      bool
	RenameSessionTitle   string
	ShowBranchModal      bool
	BranchSearchQuery    string
	ShowModelPalette     bool
	ModelSearchQuery     string
	SelectedHarnessID       string
	SelectedProviderTab     string
	CollapsedProjectGroups  map[string]bool
	ExpandedFileTreeFolders map[string]bool
}

// NewAppUI constructs the complete AppUI state tree.
func NewAppUI(engine *engine.Engine, rootDir string) *AppUI {
	appUI := &AppUI{
		Engine:                  engine,
		Workspace:               workspace.NewWorkspaceState(rootDir),
		Editor:                  editor.NewEditorManager(),
		Terminal:                terminal.NewTerminalState(rootDir),
		ThemeMgr:                theme.NewThemeManager(),
		Mode:                    ModeAgent,
		ActiveTaskKind:          "agent",
		ShowLeftSidebar:         true,
		LeftSidebarWidth:        LeftSidebarDefault,
		RightDrawerWidth:        RightDrawerDefault,
		BottomTerminalHeight:    240,
		ShowBottomTerminal:      false,
		ShowRightDrawer:         false,
		RightDrawerTab:          RightTabChanges,
		EditorActivity:          ActivityExplorer,
		NewTaskAgentType:        "acp",
		NewTaskFilterTab:        "all",
		NewTaskStep:             "agent",
		SelectedAgentID:         "agent-internal",
		SelectedAgentModel:      "Claude 3.7 Sonnet",
		SelectedHarnessID:       "agent-internal",
		SelectedProviderTab:     "all",
		CollapsedProjectGroups:  make(map[string]bool),
		ExpandedFileTreeFolders: make(map[string]bool),
		SideChatModel:        "Claude 3.7 Sonnet",
		SideChatMode:         "full",
		SideChatMessages:     make([]*agent.ChatMessage, 0),
	}

	appUI.Agent = agent.NewAgentState(engine, rootDir, func() {
		if appUI.Window != nil {
			appUI.Window.Invalidate()
		}
	})
	appUI.Agent.Theme = appUI.ThemeMgr

	appUI.ACPMgr = agent.NewACPManager(engine, func() {
		if appUI.Window != nil {
			appUI.Window.Invalidate()
		}
	})
	appUI.Agent.ACPMgr = appUI.ACPMgr

	appUI.ACPMgr.OnTokenChunk = func(sessionID, text string) {
		appUI.Agent.Lock()
		for _, s := range appUI.Agent.Sessions {
			if s.ID == sessionID {
				if len(s.Messages) > 0 {
					last := s.Messages[len(s.Messages)-1]
					if last.Role == "assistant" && !last.Done {
						last.Text += text
					}
				}
				break
			}
		}
		appUI.Agent.Unlock()
		if appUI.Window != nil {
			appUI.Window.Invalidate()
		}
	}

	appUI.CLIMgr = agent.NewAgentCLIManager(rootDir, func() {
		if appUI.Window != nil {
			appUI.Window.Invalidate()
		}
	})

	appUI.Editor.Theme = appUI.ThemeMgr

	appUI.Settings = settings.NewSettingsManager(engine, func(provider, model string) {
		appUI.Agent.ActiveModel = model
	})
	if appUI.Settings.Model != "" {
		appUI.Agent.ActiveModel = appUI.Settings.Model
	}
	appUI.Settings.ThemeMgr = appUI.ThemeMgr

	appUI.Database = database.NewDatabaseManager(appUI.Workspace)
	appUI.Database.OnNeedRedraw = func() {
		if appUI.Window != nil {
			appUI.Window.Invalidate()
		}
	}

	appUI.Git = git.NewGitManager(engine, rootDir, func(path string, showDiff bool) {
		fullPath := appUI.Workspace.ResolveFullPath(path)
		_, _ = appUI.Editor.OpenFile(fullPath, path)
		appUI.SelectedFile = path
		appUI.Editor.IsDiffView = showDiff
		appUI.Mode = ModeEditor
	})
	appUI.Git.OnNeedRedraw = func() {
		if appUI.Window != nil {
			appUI.Window.Invalidate()
		}
	}

	if engine != nil {
		engine.StartWatcher(rootDir, func() {
			appUI.Workspace.Refresh()
			if appUI.Git != nil {
				appUI.Git.Refresh()
			}
			if appUI.Window != nil {
				appUI.Window.Invalidate()
			}
		})
	}

	appUI.Palette = palette.NewPaletteManager(appUI.Workspace, func(path string) {
		fullPath := appUI.Workspace.ResolveFullPath(path)
		_, _ = appUI.Editor.OpenFile(fullPath, path)
		appUI.SelectedFile = path
		appUI.Mode = ModeEditor
	})
	appUI.Palette.SetCommands(appUI.paletteCommands())

	if len(appUI.Workspace.Files) > 0 {
		first := appUI.Workspace.Files[0]
		appUI.SelectedFile = first.RelativePath
		_, _ = appUI.Editor.OpenFile(first.FullPath, first.RelativePath)
	}

	if appUI.Git != nil && appUI.Git.Branch != "" {
		appUI.Workspace.GitBranch = appUI.Git.Branch
		appUI.Workspace.GitDirty = len(appUI.Git.Staged)+len(appUI.Git.Unstaged)+len(appUI.Git.Untracked) > 0
	}

	return appUI
}

func (a *AppUI) paletteCommands() []palette.PaletteCommand {
	return []palette.PaletteCommand{
		{ID: "new-task", Label: "New Task", Detail: "Start a new agent task (open New Agent UI)", Shortcut: "⌘N", Run: func() {
			a.NewAgentTask("")
		}},
		{ID: "new-task-agent", Label: "New Task with Agent / CLI...", Detail: "Select ACP Agent or CLI harness", Shortcut: "⇧⌘N", Run: func() {
			a.OpenNewTaskModal()
		}},
		{ID: "select-model", Label: "Select Model / Harness", Detail: "Choose AI model or ACP harness", Shortcut: "⌘M", Run: func() {
			a.ShowModelPalette = true
			a.ModelSearchQuery = ""
		}},
		{ID: "agent-mode", Label: "Switch to Agent Mode", Shortcut: "⌘1", Run: func() { a.Mode = ModeAgent }},
		{ID: "editor-mode", Label: "Switch to Editor Mode", Shortcut: "⌘2", Run: func() { a.Mode = ModeEditor }},
		{ID: "toggle-terminal", Label: "Toggle Terminal", Detail: "Bottom terminal drawer", Shortcut: "⌘`", Run: func() {
			a.ShowBottomTerminal = !a.ShowBottomTerminal
		}},
		{ID: "toggle-sidebar", Label: "Toggle Sidebar", Shortcut: "⌘B", Run: func() { a.ShowLeftSidebar = !a.ShowLeftSidebar }},
		{ID: "toggle-drawer", Label: "Toggle Right Drawer", Shortcut: "⌘J", Run: func() { a.ShowRightDrawer = !a.ShowRightDrawer }},
		{ID: "save", Label: "Save File", Shortcut: "⌘S", Run: func() { _ = a.Editor.SaveActiveFile() }},
		{ID: "settings", Label: "Open Settings", Shortcut: "⌘,", Run: func() { a.Settings.Toggle() }},
		{ID: "appearance", Label: "Toggle Appearance", Detail: "Dark / light", Shortcut: "⌘/", Run: a.toggleAppearance},
		{ID: "theme", Label: "Pick Theme Palette", Run: func() { a.ThemeMgr.Toggle() }},
		{ID: "open-folder", Label: "Open Workspace Folder", Shortcut: "⌘O", Run: a.openWorkspaceFolder},
		{ID: "board", Label: "Show Workflow Board", Run: func() {
			a.ShowRightDrawer = true
			a.RightDrawerTab = RightTabBoard
			a.RefreshBoard()
		}},
		{ID: "changes", Label: "Show Git Changes", Run: func() {
			a.ShowRightDrawer = true
			a.RightDrawerTab = RightTabChanges
		}},
		{ID: "refresh-files", Label: "Refresh Workspace Files", Run: func() {
			a.Workspace.Refresh()
			if a.Git != nil {
				go a.Git.Refresh()
			}
		}},
	}
}

// RefreshBoard reloads the workflow board from .forge/board_tasks.json.
func (a *AppUI) RefreshBoard() {
	if a.Board == nil {
		a.Board = board.NewBoardManager(a.Workspace.RootDir)
	} else {
		a.Board.RootDir = a.Workspace.RootDir
	}
	a.Board.Refresh()
}

// SetWindow attaches the native window reference for redraw invalidations.
func (a *AppUI) SetWindow(w *mygo.Window) {
	a.Window = w
}

// View builds the entire native GPU frame matching ForgeADE layout.
func (a *AppUI) View(c *ui.Context) {
	if a.ThemeMgr != nil {
		c.SetTheme(a.ThemeMgr.ActiveTheme())
	}

	// Global Keyboard Shortcuts
	if c.Shortcut(ui.Cmd, ui.KeyS) {
		if err := a.Editor.SaveActiveFile(); err == nil {
			doc := a.Editor.ActiveDoc()
			name := "File"
			if doc != nil {
				name = doc.RelativePath
			}
			c.AddToast(ui.Toast{
				Title:       "Saved",
				Description: fmt.Sprintf("%s saved to disk", name),
				Type:        "success",
			})
		}
	}
	if c.Shortcut(ui.Cmd, ui.KeyP) {
		a.Palette.Toggle()
	}
	if c.Shortcut(ui.Cmd|ui.Shift, ui.KeyP) {
		a.Palette.OpenCommands()
	}
	if c.Shortcut(ui.Cmd|ui.Shift, ui.KeyF) {
		a.Palette.Toggle()
	}
	if c.Shortcut(ui.Cmd|ui.Shift, ui.KeyE) {
		a.Mode = ModeEditor
		a.ShowLeftSidebar = true
		a.EditorActivity = ActivityExplorer
	}
	if c.Shortcut(ui.Cmd|ui.Shift, ui.KeyG) {
		a.Mode = ModeEditor
		a.ShowLeftSidebar = true
		a.EditorActivity = ActivityGit
	}
	if c.Shortcut(ui.Cmd|ui.Shift, ui.KeyJ) {
		a.ShowRightDrawer = !a.ShowRightDrawer
	}
	if c.Shortcut(ui.Cmd, ui.KeyComma) {
		a.Settings.Toggle()
	}
	if c.Shortcut(ui.Cmd, ui.KeyB) {
		a.ShowLeftSidebar = !a.ShowLeftSidebar
	}
	if c.Shortcut(ui.Cmd, ui.KeyN) {
		a.NewAgentTask("")
	}
	if c.Shortcut(ui.Cmd|ui.Shift, ui.KeyN) {
		a.OpenNewTaskModal()
	}
	if c.Shortcut(ui.Cmd, ui.KeyM) {
		a.ShowModelPalette = !a.ShowModelPalette
		a.ModelSearchQuery = ""
	}
	if c.Shortcut(ui.Cmd, ui.KeyO) {
		a.openWorkspaceFolder()
	}
	if c.Shortcut(ui.Cmd, ui.Key1) {
		a.Mode = ModeAgent
	}
	if c.Shortcut(ui.Cmd, ui.Key2) {
		a.Mode = ModeEditor
	}
	if c.Shortcut(ui.Cmd, ui.KeyJ) {
		a.ShowRightDrawer = !a.ShowRightDrawer
	}
	if c.Shortcut(ui.Cmd, ui.KeySlash) {
		a.toggleAppearance()
	}
	if c.Shortcut(ui.Cmd, ui.KeyBackquote) {
		a.ShowBottomTerminal = !a.ShowBottomTerminal
	}
	if c.Shortcut(ui.Ctrl, ui.KeyTab) {
		a.Editor.CycleTab(1)
	}
	if c.Shortcut(ui.Ctrl|ui.Shift, ui.KeyTab) {
		a.Editor.CycleTab(-1)
	}
	if c.Shortcut(ui.Cmd, ui.KeyW) {
		if a.Mode == ModeEditor && a.Editor.ActivePath != "" {
			a.Editor.CloseTab(a.Editor.ActivePath)
		}
	}

	// Root layout
	if a.Mode == ModeAgent && a.ShowLeftSidebar {
		ui.Split(c, &a.LeftSidebarWidth, func() {
			a.renderAgentSidebar(c)
		}, func() {
			a.renderMainColumn(c)
		}).SplitReset(LeftSidebarDefault).Fill()
	} else {
		a.renderMainColumn(c)
	}

	// Overlays & Modals
	a.renderModals(c)
}

func (a *AppUI) toggleAppearance() {
	if a.ThemeMgr.Appearance == "dark" {
		a.ThemeMgr.Appearance = "light"
	} else {
		a.ThemeMgr.Appearance = "dark"
	}
}

// NewAgentTask switches to Agent Mode and opens the New Agent UI ("What should we do in <project>?")
// in the specified workspace (or active workspace if workspacePath is empty).
func (a *AppUI) NewAgentTask(workspacePath string) {
	a.Mode = ModeAgent
	a.ActiveTaskKind = "agent"
	a.ShowNewTaskModal = false

	if workspacePath != "" && workspacePath != a.Workspace.RootDir {
		a.switchWorkspace(workspacePath)
	}

	if a.Agent != nil {
		a.Agent.ClearActiveSession()
	}

	if a.Window != nil {
		a.Window.Invalidate()
	}
}

func (a *AppUI) OpenNewTaskModal() {
	a.ShowNewTaskModal = true
	a.NewTaskTitle = ""
	a.NewTaskSearchQuery = ""
	a.NewTaskFilterTab = "all"
	a.NewTaskStep = "agent"
	a.NewTaskSelectedKind = ""
	a.NewTaskSelectedAgentID = ""
	a.NewTaskSelectedName = ""
	a.NewTaskSelectedExec = ""
	a.NewTaskSelectedArgs = nil
	a.NewTaskUninstalledOpen = false
	if a.ACPMgr != nil {
		a.ACPMgr.RefreshAvailability()
	}
	if a.CLIMgr != nil {
		a.CLIMgr.RefreshConfigs()
	}
}

func (a *AppUI) openWorkspaceFolder() {
	if a.Window == nil {
		return
	}
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:    a.Window,
			Title:     "Open Workspace Folder",
			Directory: true,
		})
		if err != nil || len(paths) == 0 || paths[0] == "" {
			return
		}
		a.switchWorkspace(paths[0])
	}()
}

func (a *AppUI) switchWorkspace(dir string) {
	a.Workspace = workspace.NewWorkspaceState(dir)
	a.Agent.SetWorkspacePath(dir)
	a.Terminal.Cwd = dir
	a.Editor.CloseAll()
	a.SelectedFile = ""
	a.SelectedDiffPath = ""
	if a.Database != nil {
		a.Database.Workspace = a.Workspace
		go a.Database.ScanDatabases()
	}
	if a.Git != nil {
		a.Git = git.NewGitManager(a.Engine, dir, a.Git.OnFileSelect)
		a.Git.OnNeedRedraw = func() {
			if a.Window != nil {
				a.Window.Invalidate()
			}
		}
		go a.Git.Refresh()
	}
	if a.Engine != nil {
		a.Engine.StartWatcher(dir, func() {
			a.Workspace.Refresh()
			if a.Git != nil {
				a.Git.Refresh()
			}
			if a.Window != nil {
				a.Window.Invalidate()
			}
		})
	}
	if len(a.Workspace.Files) > 0 {
		first := a.Workspace.Files[0]
		a.SelectedFile = first.RelativePath
		_, _ = a.Editor.OpenFile(first.FullPath, first.RelativePath)
	}
	if a.Window != nil {
		a.Window.Invalidate()
	}
}

func (a *AppUI) renderMainColumn(c *ui.Context) {
	ui.Column(c).Grow(1).FillHeight().Children(func() {
		a.renderWorkspaceHeader(c)

		ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
			if a.Mode == ModeAgent {
				a.renderAgentSurface(c)
			} else {
				a.renderEditorSurface(c)
			}
		})

		if a.Mode == ModeEditor {
			a.renderStatusBar(c)
		}
	})
}
