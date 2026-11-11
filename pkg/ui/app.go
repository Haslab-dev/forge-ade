package ui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/Haslab-dev/RakitKode/llm"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/hasdev/forge-ade/internal/git"
)

// Agent sidebar view modes
const (
	AgentSidebarTasks = 0
	AgentSidebarFiles = 1
)

// Layout metrics of ForgeADE's CSS (--workspace-topbar-height 52px, drawer
// defaults 260/420, h-9 terminal header, 24px status bar).
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

// AppUI is the top-level native UI application state matching ForgeADE's Dual-Surface design.
type AppUI struct {
	Engine               *Engine
	Window               *mygo.Window
	Workspace            *WorkspaceState
	Editor               *EditorManager
	Terminal             *TerminalState
	Agent                *AgentState
	Git                  *GitManager
	Database             *DatabaseManager
	Palette              *PaletteManager
	Settings             *SettingsManager
	ThemeMgr             *ThemeManager
	ACPMgr               *ACPManager
	CLIMgr               *AgentCLIManager
	Board                *BoardManager
	Mode                 int     // ModeAgent (0) or ModeEditor (1)
	ActiveTaskKind       string  // "agent" or "cli"
	ShowLeftSidebar      bool    // Left sidebar visibility
	AgentSidebarView     int     // AgentSidebarTasks (0) or AgentSidebarFiles (1)
	ShowBottomTerminal   bool    // Collapsible bottom terminal drawer
	BottomTerminalHeight float32 // Resizable terminal height
	ShowRightDrawer      bool    // Collapsible right side pane
	RightDrawerWidth     float32 // Resizable right pane width
	LeftSidebarWidth     float32 // Resizable left sidebar width
	RightDrawerTab       int     // RightTabChanges, RightTabTerminal, RightTabChat, RightTabTasks, RightTabGit, RightTabDatabase
	EditorActivity       int     // ActivityExplorer, ActivityGit, ActivityDatabase
	SelectedFile         string
	SelectedDiffPath     string
	SideChatMessages     []*ChatMessage
	SideChatInput        string
	SideChatModel        string
	SideChatMode         string // "full", "ask", "plan"
	SideChatStreaming    bool
	SideChatMd           markdown.Options
	ShowNewFileModal     bool
	NewFileName          string
	ShowNewFolderModal   bool
	NewFolderName        string
	ShowNewTaskModal     bool
	NewTaskTitle         string
	NewTaskAgentType     string // "acp" or "cli"
	SelectedAgentID      string
	SelectedAgentModel   string
	ShowTaskMenuModal    bool
	ShowRenameModal      bool
	RenameSessionTitle   string
}

// NewAppUI constructs the complete AppUI state tree.
func NewAppUI(engine *Engine, rootDir string) *AppUI {
	appUI := &AppUI{
		Engine:               engine,
		Workspace:            NewWorkspaceState(rootDir),
		Editor:               NewEditorManager(),
		Terminal:             NewTerminalState(rootDir),
		ThemeMgr:             NewThemeManager(),
		Mode:                 ModeAgent,
		ActiveTaskKind:       "agent",
		ShowLeftSidebar:      true,
		LeftSidebarWidth:     LeftSidebarDefault,
		RightDrawerWidth:     RightDrawerDefault,
		BottomTerminalHeight: 240,
		ShowBottomTerminal:   false,
		ShowRightDrawer:      false,
		RightDrawerTab:       RightTabChanges,
		EditorActivity:       ActivityExplorer,
		NewTaskAgentType:     "acp",
		SelectedAgentID:      "agent-internal",
		SelectedAgentModel:   "Claude 3.7 Sonnet",
		SideChatModel:        "Claude 3.7 Sonnet",
		SideChatMode:         "full",
		SideChatMessages:     make([]*ChatMessage, 0),
	}

	appUI.Agent = NewAgentState(engine, rootDir, func() {
		if appUI.Window != nil {
			appUI.Window.Invalidate()
		}
	})

	appUI.Agent.Theme = appUI.ThemeMgr

	appUI.ACPMgr = NewACPManager(engine, func() {
		if appUI.Window != nil {
			appUI.Window.Invalidate()
		}
	})

	appUI.Agent.ACPMgr = appUI.ACPMgr

	appUI.ACPMgr.OnTokenChunk = func(sessionID, text string) {
		appUI.Agent.mu.Lock()
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
		appUI.Agent.mu.Unlock()
		if appUI.Window != nil {
			appUI.Window.Invalidate()
		}
	}

	appUI.CLIMgr = NewAgentCLIManager(rootDir, func() {
		if appUI.Window != nil {
			appUI.Window.Invalidate()
		}
	})

	appUI.Editor.Theme = appUI.ThemeMgr

	appUI.Settings = NewSettingsManager(engine, func(provider, model string) {
		appUI.Agent.ActiveModel = model
	})
	appUI.Settings.ThemeMgr = appUI.ThemeMgr
	appUI.Settings.ACPMgr = appUI.ACPMgr

	appUI.Database = NewDatabaseManager(engine, appUI.Workspace)

	appUI.Git = NewGitManager(engine, rootDir, func(path string, showDiff bool) {
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

	// Attach live file system watcher to auto-refresh file tree and git on disk mutations
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

	appUI.Palette = NewPaletteManager(appUI.Workspace, func(path string) {
		fullPath := appUI.Workspace.ResolveFullPath(path)
		_, _ = appUI.Editor.OpenFile(fullPath, path)
		appUI.SelectedFile = path
		appUI.Mode = ModeEditor
	})
	appUI.Palette.SetCommands(appUI.paletteCommands())

	// Pre-open first file in workspace
	if len(appUI.Workspace.Files) > 0 {
		first := appUI.Workspace.Files[0]
		appUI.SelectedFile = first.RelativePath
		_, _ = appUI.Editor.OpenFile(first.FullPath, first.RelativePath)
	}

	// Initial Git status sync
	if appUI.Git != nil && appUI.Git.Branch != "" {
		appUI.Workspace.GitBranch = appUI.Git.Branch
		appUI.Workspace.GitDirty = len(appUI.Git.Staged)+len(appUI.Git.Unstaged)+len(appUI.Git.Untracked) > 0
	}

	return appUI
}

// paletteCommands is the ⇧⌘P command list (CommandPaletteModal parity).
func (a *AppUI) paletteCommands() []paletteCommand {
	return []paletteCommand{
		{ID: "new-task", Label: "New Task", Detail: "Start a task session", Shortcut: "⌘N", Run: func() {
			a.ShowNewTaskModal = true
			a.NewTaskTitle = ""
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
		a.Board = NewBoardManager(a.Workspace.RootDir)
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
	// Apply dynamic theme palette and appearance
	if a.ThemeMgr != nil {
		c.SetTheme(a.ThemeMgr.ActiveTheme())
	}

	// Global Keyboard Shortcuts (workspaceLifecycle.ts parity)
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
		a.ShowNewTaskModal = true
		a.NewTaskTitle = ""
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

	// Root layout: Full-Height Left Sidebar (Agent Mode) + Main Column with interactive Split divider
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

// toggleAppearance flips dark/light, as ⇧⌘[] does in the original.
func (a *AppUI) toggleAppearance() {
	if a.ThemeMgr.Appearance == "dark" {
		a.ThemeMgr.Appearance = "light"
	} else {
		a.ThemeMgr.Appearance = "dark"
	}
}

// openWorkspaceFolder shows the system folder picker (⌘O).
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

// switchWorkspace re-roots the app at another folder, carrying the mode and
// the surfaces across, as WorkspaceSwitcherDropdown does.
func (a *AppUI) switchWorkspace(dir string) {
	a.Workspace = NewWorkspaceState(dir)
	a.Agent.workspacePath = dir
	a.Terminal.Cwd = dir
	a.Editor.CloseAll()
	a.SelectedFile = ""
	a.SelectedDiffPath = ""
	if a.Database != nil {
		a.Database.Workspace = a.Workspace
		go a.Database.ScanDatabases()
	}
	if a.Git != nil {
		a.Git = NewGitManager(a.Engine, dir, a.Git.OnFileSelect)
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

// renderMainColumn renders the header, surface viewport, and status bar.
func (a *AppUI) renderMainColumn(c *ui.Context) {
	ui.Column(c).Grow(1).FillHeight().Children(func() {
		// Unified Workspace Header
		a.renderWorkspaceHeader(c)

		// Surface Viewport
		ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
			if a.Mode == ModeAgent {
				a.renderAgentSurface(c)
			} else {
				a.renderEditorSurface(c)
			}
		})

		// Status Bar (Always present in Editor mode)
		if a.Mode == ModeEditor {
			a.renderStatusBar(c)
		}
	})
}

// renderWorkspaceHeader renders ForgeADE's WorkspaceHeader: 52px tall, the
// chrome background, title, project/branch chips, the segmented mode
// switcher and the 28px icon controls.
func (a *AppUI) renderWorkspaceHeader(c *ui.Context) {
	t := c.Theme()
	tc := a.ThemeMgr.Colors()
	projectName := filepath.Base(a.Workspace.RootDir)

	// Derive display title
	title := projectName
	if a.Mode == ModeAgent {
		if a.ActiveTaskKind == "cli" && a.CLIMgr != nil && a.CLIMgr.ActiveSession() != nil {
			title = a.CLIMgr.ActiveSession().Title
		} else if sess := a.Agent.ActiveSession(); sess != nil {
			title = sess.Title
		}
	} else {
		if doc := a.Editor.ActiveDoc(); doc != nil {
			title = filepath.Base(doc.RelativePath)
		}
	}

	bar := c.TitleBar()
	leftPad := float32(14)
	showToggleInHeader := true
	if a.Mode == ModeAgent && a.ShowLeftSidebar {
		// Full-height sidebar is visible on the left; header sits beside it
		showToggleInHeader = false
		leftPad = 12
	} else if bar.Left > 0 {
		// Sidebar is collapsed or in editor mode; clear traffic lights
		leftPad = max(bar.Left, 78)
	}

	ui.Row(c).Height(WorkspaceTopBarHeight).Gap(10).Padding(0, 12, 0, leftPad).
		AlignItems(ui.Center).
		Background(tc.Chrome).
		BorderWidth(0, 0, 1, 0).BorderColor(tc.ToolbarBorder).
		DragWindow().
		Children(func() {
			// Left: Sidebar Toggle (only shown when the full sidebar is collapsed)
			if showToggleInHeader {
				if IconButton(c, tc, TablerPanelLeft, 16).Clicked() {
					a.ShowLeftSidebar = !a.ShowLeftSidebar
				}
			}

			// Title: text-ui-sm font-semibold max-w-80 truncate
			ui.Text(c, title).FontSize(13).FontWeight(600).MaxWidth(320)

			// Task "…" menu (Agent mode with an active session)
			if a.Mode == ModeAgent && a.Agent != nil && a.Agent.ActiveSession() != nil {
				if IconButton(c, tc, TablerDots, 15).Clicked() {
					a.ShowTaskMenuModal = true
				}
			}

			ui.Box(c).Grow(1)

			// Segmented Mode Switcher: [✨ Agent] | [💻 Editor]
			// h-7 rounded-lg border border-border-default/80 bg-surface/60 p-0.5
			ui.Row(c).Height(28).Gap(2).Padding(2).Radius(8).
				Background(tc.ToolbarControl).
				BorderWidth(1).BorderColor(tc.ToolbarBorder).
				AlignItems(ui.Center).Children(func() {
				segmentedModeButton(c, tc, TablerSparkles, "Agent", a.Mode == ModeAgent, func() { a.Mode = ModeAgent })
				segmentedModeButton(c, tc, TablerCode, "Editor", a.Mode == ModeEditor, func() { a.Mode = ModeEditor })
			})

			// 4px×16px divider (h-4 w-px bg-border/60)
			ui.Box(c).Size(1, 16).Background(tc.ToolbarBorder)

			// Workspace & panel controls: Reveal in Finder, Terminal, Right Pane, Appearance
			if IconButton(c, tc, TablerFolder, 15).Clicked() {
				mygo.Shell.ShowItemInFolder(a.Workspace.RootDir)
			}
			if ToggleIconButton(c, tc, TablerTerminal, 15, a.ShowBottomTerminal).Clicked() {
				a.ShowBottomTerminal = !a.ShowBottomTerminal
			}
			if ToggleIconButton(c, tc, TablerPanelRight, 15, a.ShowRightDrawer).Clicked() {
				a.ShowRightDrawer = !a.ShowRightDrawer
			}
			if IconButton(c, tc, TablerPalette, 15).Clicked() {
				a.ThemeMgr.Toggle()
			}
			_ = t
		})
}

// segmentedModeButton is one half of the header's segmented switch: h-6
// rounded-md px-2 font-medium; the active side is bg-card shadow-2xs with
// the accent icon.
func segmentedModeButton(c *ui.Context, tc ThemeColors, icon *ui.SVG, label string, active bool, onPick func()) {
	btn := ui.Row(c).Height(24).Radius(6).Padding(0, 8).Gap(5).AlignItems(ui.Center).Justify(ui.Center)
	btn.Children(func() {
		ic := ui.Icon(c, icon).Width(14).Height(14)
		txt := ui.Text(c, label).FontSize(12).FontWeight(500)
		if active {
			ic.TextColor(tc.Accent)
			txt.TextColor(tc.Text)
		} else {
			ic.TextColor(tc.ToolbarControlForeground)
			txt.TextColor(tc.ToolbarControlForeground)
		}
	})
	if active {
		btn.Background(tc.SurfaceOverlay).
			Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.18))
	} else if btn.Hovered() {
		btn.Background(tc.ToolbarControlHover)
	}
	if btn.Clicked() {
		onPick()
	}
}

// renderAgentSurface renders the center chat viewport, collapsible terminal & right drawer.
func (a *AppUI) renderAgentSurface(c *ui.Context) {
	// Center Column: Main Chat Container (or CLI surface) + Collapsible Bottom Terminal
	ui.Column(c).Grow(1).FillHeight().Children(func() {
		// Agent Main Container
		ui.Column(c).Grow(1).FillHeight().Background(a.ThemeMgr.Colors().Canvas).Children(func() {
			if a.ActiveTaskKind == "cli" && a.CLIMgr != nil && a.CLIMgr.ActiveSession() != nil {
				a.CLIMgr.RenderTerminalSessionSurface(c, a.CLIMgr.ActiveID)
			} else {
				a.Agent.RenderChat(c)
			}
		})

		// Collapsible Bottom Terminal with interactive drag resize handle
		if a.ShowBottomTerminal {
			TopResizeHandle(c, &a.BottomTerminalHeight, 120, 600, 240)
			a.renderBottomTerminal(c)
		}
	})

	// Collapsible Right Drawer with interactive drag resize handle
	if a.ShowRightDrawer {
		LeftResizeHandle(c, &a.RightDrawerWidth, 320, 750, RightDrawerDefault)
		a.renderRightDrawer(c)
	}
}

// renderAgentSidebar renders the full-height task sessions sidebar with filter tabs and profile row.
func (a *AppUI) renderAgentSidebar(c *ui.Context) {
	t := c.Theme()
	tc := a.ThemeMgr.Colors()
	bar := c.TitleBar()
	topPadLeft := float32(14)
	if bar.Left > 0 {
		topPadLeft = max(bar.Left, 76)
	}

	ui.Column(c).Grow(1).FillHeight().Background(tc.Sidebar).BorderWidth(0, 1, 0, 0).BorderColor(tc.SidebarBorder).Children(func() {
		// Top window controls strip (48px, macOS traffic-light inset) drags
		// the window, its collapse button stays clickable.
		ui.Row(c).Height(48).Padding(0, 12, 0, topPadLeft).AlignItems(ui.Center).DragWindow().Children(func() {
			if IconButton(c, tc, TablerPanelLeft, 15).Clicked() {
				a.ShowLeftSidebar = false
			}
			ui.Row(c).Padding(0, 6).Gap(4).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "ForgeADE").FontSize(13).FontWeight(600).LetterSpacing(-0.2)
			})
			ui.Box(c).Grow(1)
		})

		// Primary Action Buttons: + New Task (⌘N), Search (⌘P)
		ui.Column(c).Padding(6, 10).Gap(2).Children(func() {
			newTask := SidebarActionButton(c, tc, TablerPlus, "New Task", "⌘N", false)
			if newTask.Clicked() {
				a.ShowNewTaskModal = true
				a.NewTaskTitle = ""
			}
			search := SidebarActionButton(c, tc, TablerSearch, "Search", "⌘P", false)
			if search.Clicked() {
				a.Palette.Toggle()
			}
		})

		// Sidebar View Switcher: Tasks | Files
		ui.Row(c).Gap(4).Padding(2, 10).Children(func() {
			pill := func(label string, icon *ui.SVG, active bool, onPick func()) {
				btn := ui.Row(c).Height(24).Radius(999).Padding(0, 10).Gap(5).AlignItems(ui.Center)
				btn.Children(func() {
					ui.Icon(c, icon).Width(12).Height(12).TextColor(tc.SidebarForeground)
					ui.Text(c, label).FontSize(12)
					if active {
						ui.Text(c, "").FontSize(12)
					}
				})
				if active {
					btn.Background(tc.Text.Alpha(0.15))
					btn.Children(func() {})
				} else if btn.Hovered() {
					btn.Background(tc.SidebarRowHover)
				}
				if btn.Clicked() {
					onPick()
				}
			}
			pill("Tasks", TablerChecklist, a.AgentSidebarView == AgentSidebarTasks, func() { a.AgentSidebarView = AgentSidebarTasks })
			pill("Files", TablerFiles, a.AgentSidebarView == AgentSidebarFiles, func() { a.AgentSidebarView = AgentSidebarFiles })
		})

		if a.AgentSidebarView == AgentSidebarFiles {
			// Embedded Project File Explorer in Agent Sidebar
			ui.Column(c).Grow(1).Children(func() {
				a.renderFileTree(c)
			})
		} else {
			// Section header: PROJECTS
			ui.Row(c).Padding(10, 14, 4, 14).AlignItems(ui.Center).Children(func() {
				SectionLabel(c, tc, "PROJECTS")
				ui.Box(c).Grow(1)
				if IconButton(c, tc, TablerPlus, 12).Clicked() {
					a.Agent.NewSession("")
					a.ActiveTaskKind = "agent"
				}
			})

			// Search Input Filter
			ui.Row(c).Padding(2, 10, 4, 10).Children(func() {
				ui.TextInput(c, &a.Agent.SearchQuery).Placeholder("Search tasks...").Grow(1)
			})

			// Filter Tabs: Active, Pinned, Archived with counts
			activeCount, pinnedCount, archivedCount := a.Agent.TabCounts()
			tabCounts := map[string]int{
				"active":   activeCount,
				"pinned":   pinnedCount,
				"archived": archivedCount,
			}
			ui.Row(c).Gap(4).Padding(4, 10).AlignItems(ui.Center).Children(func() {
				capitalize := func(s string) string {
					if s == "" {
						return s
					}
					return strings.ToUpper(s[:1]) + s[1:]
				}
				for _, tab := range []string{"active", "pinned", "archived"} {
					thisTab := tab
					active := a.Agent.ActiveFilterTab == thisTab
					pill := ui.Row(c).Radius(999).Padding(2, 8).AlignItems(ui.Center).Gap(4)
					pill.Children(func() {
						ui.Text(c, capitalize(thisTab)).FontSize(12)
						if n := tabCounts[thisTab]; n > 0 {
							ui.Text(c, fmt.Sprintf("%d", n)).FontSize(10).TextColor(tc.SidebarMutedForeground)
						}
					})
					if active {
						pill.Background(tc.Text.Alpha(0.15))
					} else if pill.Hovered() {
						pill.Background(tc.SidebarRowHover)
					}
					if pill.Clicked() {
						a.Agent.ActiveFilterTab = thisTab
					}
				}
			})

			// Scrollable Task Sessions List
			sessions := a.Agent.FilteredSessions()
			ui.Scroll(c).Grow(1).Children(func() {
				ui.Column(c).Padding(2, 8).Gap(2).Children(func() {
					// 1. Agent ACP Sessions
					for _, sess := range sessions {
						s := sess
						isActive := (a.ActiveTaskKind == "agent" && s.ID == a.Agent.ActiveSessionID)
						renderTaskRow(c, tc, taskRowData{
							Key:      "sess-" + s.ID,
							Title:    s.Title,
							Subtitle: s.Model,
							Status:   s.Status,
							Active:   isActive,
							Pinned:   s.Pinned,
							Archived: s.Archived,
							OnPin:    func() { a.Agent.TogglePin(s.ID) },
							OnDelete: func() { a.Agent.DeleteSession(s.ID) },
							OnOpen:   func() { a.ActiveTaskKind = "agent"; a.Agent.SelectSession(s.ID) },
						})
					}

					// 2. Active CLI Sessions
					if a.CLIMgr != nil {
						for _, cliSess := range a.CLIMgr.Sessions {
							cs := cliSess
							isActive := (a.ActiveTaskKind == "cli" && a.CLIMgr.ActiveID == cs.ID)
							renderTaskRow(c, tc, taskRowData{
								Key:      "cli-sess-" + cs.ID,
								Title:    cs.Title,
								Subtitle: cs.AgentName + " · CLI Task",
								Status:   "running",
								Active:   isActive,
								OnOpen:   func() { a.ActiveTaskKind = "cli"; a.CLIMgr.ActiveID = cs.ID },
							})
						}
					}
				})
			})
		}

		// Bots / Automations Section
		ui.Column(c).BorderWidth(1, 0, 0, 0).BorderColor(tc.SidebarBorder).Padding(6, 10).Gap(2).Children(func() {
			ui.Row(c).Padding(2, 4).AlignItems(ui.Center).Children(func() {
				SectionLabel(c, tc, "BOTS")
				ui.Box(c).Grow(1)
				if IconButton(c, tc, TablerPlus, 12).Clicked() {
					a.ShowNewTaskModal = true
				}
			})
			botRow := SidebarActionButton(c, tc, TablerBot, "Add a bot or schedule", "", false)
			if botRow.Clicked() {
				a.ShowNewTaskModal = true
			}
		})

		// Bottom Utility Dock
		dockBtn := func(icon *ui.SVG, active bool, onPick func()) {
			btn := ui.Row(c).Size(28, 28).Radius(6).AlignItems(ui.Center).Justify(ui.Center)
			btn.Children(func() {
				ic := ui.Icon(c, icon).Width(15).Height(15)
				if active {
					ic.TextColor(tc.Accent)
				} else {
					ic.TextColor(tc.IconMuted)
				}
			})
			if active {
				btn.Background(tc.Text.Alpha(0.1))
			} else if btn.Hovered() {
				btn.Background(tc.Text.Alpha(0.05))
			}
			if btn.Clicked() {
				onPick()
			}
		}
		ui.Row(c).Padding(6, 10).Gap(2).AlignItems(ui.Center).
			BorderWidth(1, 0, 0, 0).BorderColor(tc.SidebarBorder).
			Background(tc.Chrome.Alpha(0.4)).
			Children(func() {
				dockBtn(TablerHelp, false, func() { mygo.Shell.OpenExternal("https://github.com/hasdev/forge-ade") })
				dockBtn(TablerChartBar, false, func() {
					a.ShowRightDrawer = true
					a.RightDrawerTab = RightTabTasks
				})
				dockBtn(TablerLayoutKanban, a.RightDrawerTab == RightTabBoard && a.ShowRightDrawer, func() {
					a.ShowRightDrawer = true
					a.RightDrawerTab = RightTabBoard
				})
				dockBtn(TablerGitBranch, a.RightDrawerTab == RightTabGit && a.ShowRightDrawer, func() {
					a.ShowRightDrawer = true
					a.RightDrawerTab = RightTabGit
				})
				dockBtn(TablerBook, false, func() {
					a.ShowNewFileModal = true
					a.NewFileName = "AGENTS.md"
				})
				dockBtn(TablerSettings, false, func() { a.Settings.Toggle() })
				ui.Box(c).Grow(1)
				if a.ThemeMgr.Appearance == "dark" {
					dockBtn(TablerMoon, false, a.toggleAppearance)
				} else {
					dockBtn(TablerSun, false, a.toggleAppearance)
				}
			})
		_ = t
	})
}

// renderEditorSurface renders the Editor mode: ActivityBar + FileTree, EditorView (tabs + buffer/diff), Bottom Terminal, Right Drawer.
func (a *AppUI) renderEditorSurface(c *ui.Context) {
	tc := a.ThemeMgr.Colors()

	// 1. Left Activity Bar + Sidebar
	if a.ShowLeftSidebar {
		// Activity Bar (Explorer, Git, DB) + bottom Settings: 48px wide,
		// 2px left active indicator, 20px icons.
		activityButton := func(icon *ui.SVG, tab int) {
			active := a.EditorActivity == tab
			btn := ui.Row(c).Size(ActivityBarWidth-8, 40).Radius(6).AlignItems(ui.Center).Justify(ui.Center)
			btn.Children(func() {
				ic := ui.Icon(c, icon).Width(20).Height(20)
				if active {
					ic.TextColor(tc.Text)
				} else {
					ic.TextColor(tc.IconMuted)
				}
			})
			if active {
				btn.Background(tc.Text.Alpha(0.06))
			} else if btn.Hovered() {
				btn.Background(tc.Text.Alpha(0.04))
			}
			if btn.Clicked() {
				a.EditorActivity = tab
			}
			// Active indicator: absolute left bar
			if active {
				ui.Box(c).Absolute().Left(0).Width(2).Top(8).Bottom(8).Background(tc.Text).Radius(0, 1, 1, 0)
			}
		}
		ui.Column(c).Width(ActivityBarWidth).Background(tc.Canvas).BorderWidth(0, 1, 0, 0).BorderColor(tc.Border).
			Padding(8, 0).AlignItems(ui.Center).Gap(4).Children(func() {
			activityButton(TablerFiles, ActivityExplorer)
			activityButton(TablerGitBranch, ActivityGit)
			activityButton(TablerDatabase, ActivityDatabase)
			ui.Box(c).Grow(1)
			activityButton(TablerSettings, -1)
			_ = activityButton
		})

		// File Tree or Activity Panel
		ui.Column(c).Width(FileTreeDefaultWidth).Background(tc.Canvas).BorderWidth(0, 1, 0, 0).BorderColor(tc.Border).Children(func() {
			switch a.EditorActivity {
			case ActivityExplorer:
				a.renderFileTree(c)
			case ActivityGit:
				if a.Git != nil {
					a.Git.RenderGit(c)
				}
			case ActivityDatabase:
				if a.Database != nil {
					a.Database.RenderDatabase(c)
				}
			}
		})
	}

	// 2. Center Column: Editor Tabs + Virtualized Code Buffer / Diff + Collapsible Bottom Terminal
	ui.Column(c).Grow(1).FillHeight().Children(func() {
		// Main Editor View
		ui.Column(c).Grow(1).FillHeight().Children(func() {
			a.Editor.RenderEditorView(c)
		})

		// Collapsible Bottom Terminal with interactive drag resize handle
		if a.ShowBottomTerminal {
			TopResizeHandle(c, &a.BottomTerminalHeight, 120, 600, 240)
			a.renderBottomTerminal(c)
		}
	})

	// 3. Collapsible Right Drawer with interactive drag resize handle
	if a.ShowRightDrawer {
		LeftResizeHandle(c, &a.RightDrawerWidth, 320, 750, RightDrawerDefault)
		a.renderRightDrawer(c)
	}
}

// renderFileTree renders the workspace file explorer: 35px header, filter
// input, 22px rows with file icons and dirty badges.
func (a *AppUI) renderFileTree(c *ui.Context) {
	tc := a.ThemeMgr.Colors()

	ui.Row(c).Height(35).Padding(0, 12).AlignItems(ui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
		ui.Text(c, "EXPLORER").FontSize(12).FontWeight(600).LetterSpacing(0.4)
		ui.Box(c).Grow(1)
		treeBtn := func(icon *ui.SVG, onPick func()) {
			btn := ui.Row(c).Size(22, 22).Radius(4).AlignItems(ui.Center).Justify(ui.Center)
			btn.Children(func() {
				ui.Icon(c, icon).Width(14).Height(14).TextColor(tc.IconMuted)
			})
			if btn.Hovered() {
				btn.Background(tc.Text.Alpha(0.05))
			}
			if btn.Clicked() {
				onPick()
			}
		}
		treeBtn(TablerFilePlus, func() {
			a.ShowNewFileModal = true
			a.NewFileName = ""
		})
		treeBtn(TablerFolderPlus, func() {
			a.ShowNewFolderModal = true
			a.NewFolderName = ""
		})
		treeBtn(TablerRefresh, func() {
			a.Workspace.Refresh()
			if a.Git != nil {
				go a.Git.Refresh()
			}
			if a.Database != nil {
				go a.Database.ScanDatabases()
			}
		})
	})

	// Search filter
	ui.Row(c).Padding(4, 10).Children(func() {
		ui.TextInput(c, &a.Workspace.FilterQuery).Placeholder("Filter files...").Grow(1)
	})

	// File List with Tabler vector icons and dirty badges
	filtered := a.Workspace.FilteredFiles()
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Padding(2, 6).Gap(0).Children(func() {
			for _, f := range filtered {
				f := f
				selected := a.SelectedFile == f.RelativePath
				dirty := false
				if doc := a.Editor.GetDoc(f.FullPath); doc != nil && doc.Editor.Buffer.Modified() {
					dirty = true
				}
				row := ui.Row(c.Key("ftree-"+f.RelativePath)).Height(FileTreeRowHeight).Radius(4).
					Padding(0, 8).Gap(6).AlignItems(ui.Center)
				row.Children(func() {
					ui.Icon(c, FileIconSVG(f.RelativePath)).Width(14).Height(14).TextColor(tc.IconMuted)
					ui.Text(c, f.RelativePath).FontSize(12).Grow(1).SingleLine()
					if dirty {
						ui.Box(c).Size(6, 6).Radius(3).Background(tc.Warning)
					}
				})
				if selected {
					row.Background(tc.Accent.Alpha(0.12))
				} else if row.Hovered() {
					row.Background(tc.Text.Alpha(0.04))
				}
				if row.Clicked() {
					a.SelectedFile = f.RelativePath
					fullPath := a.Workspace.ResolveFullPath(f.RelativePath)

					// Jump to database tab if database clicked
					ext := filepath.Ext(f.RelativePath)
					if ext == ".sqlite" || ext == ".db" || ext == ".duckdb" {
						if a.Database != nil {
							a.Database.Inspect(fullPath)
							a.EditorActivity = ActivityDatabase
						}
						return
					}

					_, _ = a.Editor.OpenFile(fullPath, f.RelativePath)
				}
			}
		})
	})
}

// renderBottomTerminal renders the embedded Ghostty terminal drawer: a 36px
// header and the terminal body.
func (a *AppUI) renderBottomTerminal(c *ui.Context) {
	tc := a.ThemeMgr.Colors()

	ui.Column(c).Height(a.BottomTerminalHeight).Background(tc.Canvas).BorderWidth(1, 0, 0, 0).BorderColor(tc.Border).Children(func() {
		// Terminal Drawer Header
		ui.Row(c).Height(TerminalHeaderHeight).Padding(0, 12).AlignItems(ui.Center).
			BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, TablerTerminal).Width(14).Height(14).TextColor(tc.IconMuted)
				ui.Text(c, "Terminal").FontSize(13).FontWeight(500)
			})
			ui.Box(c).Grow(1)
			restartBtn := ui.Row(c).Radius(4).Padding(2, 8).AlignItems(ui.Center)
			restartBtn.Children(func() {
				ui.Text(c, "Restart").FontSize(12).TextColor(tc.TextMuted)
			})
			if restartBtn.Hovered() {
				restartBtn.Background(tc.Text.Alpha(0.05))
			}
			if restartBtn.Clicked() {
				_ = a.Terminal.Restart()
			}
			if IconButton(c, tc, TablerX, 13).Clicked() {
				a.ShowBottomTerminal = false
			}
		})

		// Terminal Content
		a.Terminal.RenderTerminal(c)
	})
}

// renderRightDrawer renders the collapsible side drawer: a 52px tab bar
// with pane pills and the active pane below.
func (a *AppUI) renderRightDrawer(c *ui.Context) {
	tc := a.ThemeMgr.Colors()

	ui.Column(c).Width(a.RightDrawerWidth).Background(tc.Canvas).BorderWidth(0, 0, 0, 1).BorderColor(tc.Border).Children(func() {
		// Drawer Header Tabs (AgentRightSidebar pane tabs: h-6 pills)
		ui.Row(c).Height(WorkspaceTopBarHeight).Gap(4).Padding(0, 10).AlignItems(ui.Center).
			BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
			tabs := []struct {
				Tab  int
				Name string
				Icon *ui.SVG
			}{
				{RightTabChanges, "Changes", TablerFileDiff},
				{RightTabTerminal, "Terminal", TablerTerminal},
				{RightTabChat, "Chat", TablerMessage},
				{RightTabTasks, "Tasks", TablerChecklist},
				{RightTabBoard, "Board", TablerLayoutKanban},
				{RightTabGit, "Git", TablerGitBranch},
				{RightTabDatabase, "DB", TablerDatabase},
			}

			for _, tab := range tabs {
				tTab := tab
				active := a.RightDrawerTab == tTab.Tab
				pill := ui.Row(c.Key(fmt.Sprintf("rtab-%d", tTab.Tab))).Height(24).Radius(6).
					Padding(0, 6).Gap(3).AlignItems(ui.Center)
				pill.Children(func() {
					ui.Icon(c, tTab.Icon).Width(11).Height(11).TextColor(tc.IconMuted)
					ui.Text(c, tTab.Name).FontSize(11)
				})
				if active {
					pill.Background(tc.Text.Alpha(0.1))
				} else if pill.Hovered() {
					pill.Background(tc.Text.Alpha(0.05))
				}
				if pill.Clicked() {
					a.RightDrawerTab = tTab.Tab
				}
			}

			ui.Box(c).Grow(1)

			if IconButton(c, tc, TablerX, 13).Clicked() {
				a.ShowRightDrawer = false
			}
		})

		// Drawer Content
		ui.Column(c).Grow(1).FillHeight().Children(func() {
			switch a.RightDrawerTab {
			case RightTabChanges:
				a.renderChangesPane(c)
			case RightTabTerminal:
				a.Terminal.RenderTerminal(c)
			case RightTabChat:
				a.renderSideChatPane(c)
			case RightTabTasks:
				if a.Agent != nil && a.Agent.TaskManager != nil {
					a.Agent.TaskManager.RenderChecklist(c)
				}
			case RightTabBoard:
				a.renderBoardPane(c)
			case RightTabGit:
				if a.Git != nil {
					a.Git.RenderGit(c)
				}
			case RightTabDatabase:
				if a.Database != nil {
					a.Database.RenderDatabase(c)
				}
			}
		})
	})
}

// renderChangesPane renders the git changes pane: h-7 file rows with real
// +N/−N stats, stage buttons, and the MyGo internal diff viewer below.
func (a *AppUI) renderChangesPane(c *ui.Context) {
	tc := a.ThemeMgr.Colors()

	var files []git.FileStatus
	if a.Git != nil {
		files = append(files, a.Git.Staged...)
		files = append(files, a.Git.Unstaged...)
		files = append(files, a.Git.Untracked...)
	}

	// Subheader (surface-subheader: h-10 border-b bg-background)
	ui.Row(c).Height(40).Padding(0, 12).AlignItems(ui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
		ui.Text(c, "Changes").FontSize(13).FontWeight(600)
		ui.Box(c).Grow(1)
		if IconButton(c, tc, TablerRefresh, 12).Clicked() && a.Git != nil {
			a.Git.Refresh()
		}
		ui.Text(c, fmt.Sprintf("%d files", len(files))).FontSize(11).TextColor(tc.TextMuted)
	})

	if len(files) == 0 {
		ui.Column(c).Grow(1).Center().Gap(8).Padding(20).Children(func() {
			ui.Icon(c, TablerFileDiff).Width(32).Height(32).TextColor(tc.IconMuted)
			ui.Text(c, "No changes yet").FontSize(13).FontWeight(600).TextColor(tc.TextMuted)
			ui.Text(c, "Files modified in the workspace show up here with their diffs.").FontSize(12).TextColor(tc.TextMuted).MaxWidth(240)
			if ContrastCTA(c, tc, "Refresh changes").Clicked() && a.Git != nil {
				a.Git.Refresh()
			}
		})
		return
	}

	// Track selection (first file selected by default, as the original does)
	if a.SelectedDiffPath == "" {
		a.SelectedDiffPath = files[0].Path
	}
	found := false
	for _, f := range files {
		if f.Path == a.SelectedDiffPath {
			found = true
			break
		}
	}
	if !found {
		a.SelectedDiffPath = files[0].Path
	}

	// File rows: h-7 rounded-md px-2 hover:bg-accent/60
	ui.Column(c).Grow(1).FillHeight().Children(func() {
		ui.Scroll(c).Height(minF(220, float32(len(files))*28+8)).Children(func() {
			ui.Column(c).Padding(4, 8).Gap(1).Children(func() {
				for _, f := range files {
					f := f
					isSel := a.SelectedDiffPath == f.Path
					row := ui.Row(c.Key("chg-"+f.Path)).Height(28).Radius(6).Padding(0, 8).Gap(6).AlignItems(ui.Center)
					row.Children(func() {
						ui.Icon(c, FileIconSVG(f.Path)).Width(13).Height(13).TextColor(tc.IconMuted)
						ui.Text(c, filepath.Base(f.Path)).FontSize(12).Grow(1).SingleLine()
						DiffStatText(c, tc, int(f.Additions), int(f.Deletions))
						badgeLabel := string(f.Status)
						badgeColor := tc.Accent
						if f.Staging == "staged" {
							badgeLabel = "S"
							badgeColor = tc.Success
						}
						Badge(c, tc, badgeLabel, badgeColor)
						// Stage/Unstage hover action
						stageBtn := ui.Row(c).Size(20, 20).Radius(4).AlignItems(ui.Center).Justify(ui.Center)
						stageBtn.Children(func() {
							if f.Staging == "staged" {
								ui.Icon(c, TablerMinus).Width(12).Height(12).TextColor(tc.IconMuted)
							} else {
								ui.Icon(c, TablerPlus).Width(12).Height(12).TextColor(tc.IconMuted)
							}
						})
						if stageBtn.Hovered() {
							stageBtn.Background(tc.Text.Alpha(0.06))
						}
						if stageBtn.Clicked() && a.Engine != nil {
							paths := []string{f.Path}
							if f.Staging == "staged" {
								go func() {
									_ = a.Engine.GitUnstage(a.Workspace.RootDir, paths)
									a.Git.Refresh()
								}()
							} else {
								go func() {
									_ = a.Engine.GitStage(a.Workspace.RootDir, paths)
									a.Git.Refresh()
								}()
							}
						}
					})
					if isSel {
						row.Background(tc.Accent.Alpha(0.14))
					} else if row.Hovered() {
						row.Background(tc.Text.Alpha(0.04))
					}
					if row.Clicked() {
						a.SelectedDiffPath = f.Path
					}
				}
			})
		})

		// MyGo internal diff viewer for the selected file
		ui.Column(c).Grow(1).FillHeight().Children(func() {
			diffTitle := a.SelectedDiffPath
			header := ui.Row(c).Height(36).Padding(0, 10).Gap(6).AlignItems(ui.Center).
				BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Background(tc.Surface)
			header.Children(func() {
				ui.Icon(c, TablerGitCompare).Width(13).Height(13).TextColor(tc.IconMuted)
				ui.Text(c, filepath.Base(diffTitle)).FontSize(12).FontWeight(500)
				ui.Box(c).Grow(1)
				openBtn := ui.Row(c).Radius(4).Padding(2, 8).AlignItems(ui.Center)
				openBtn.Children(func() {
					ui.Text(c, "Open in Editor").FontSize(11).TextColor(tc.TextMuted)
				})
				if openBtn.Hovered() {
					openBtn.Background(tc.Text.Alpha(0.05))
				}
				if openBtn.Clicked() {
					fullPath := filepath.Join(a.Workspace.RootDir, diffTitle)
					_, _ = a.Editor.OpenFile(fullPath, diffTitle)
					a.Mode = ModeEditor
				}
			})

			patch := ""
			if a.Engine != nil && diffTitle != "" {
				patch = a.Engine.GetGitDiff(a.Workspace.RootDir, diffTitle)
			}
			if patch == "" {
				ui.Column(c).Grow(1).Center().Children(func() {
					ui.Text(c, "No diff available for this file").FontSize(12).TextColor(tc.TextMuted)
				})
			} else {
				d := a.Editor.OpenPatchDiff(diffTitle, patch, a.ThemeMgr)
				if d != nil {
					editor.DiffView(c, d).Grow(1).FillWidth()
				}
			}
		})
	})
}

// renderSideChatPane renders an independent side conversation thread,
// streaming replies from the workspace LLM profile.
func (a *AppUI) renderSideChatPane(c *ui.Context) {
	tc := a.ThemeMgr.Colors()
	a.SideChatMd.Theme = a.ThemeMgr.MarkdownTheme()

	// Header
	ui.Row(c).Height(40).Padding(0, 12).AlignItems(ui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
		ui.Text(c, "Side Chat").FontSize(13).FontWeight(600)
		ui.Box(c).Grow(1)
		ui.Text(c, a.SideChatModel).FontSize(10).TextColor(tc.TextMuted)
	})

	// Message list
	if len(a.SideChatMessages) == 0 {
		ui.Column(c).Grow(1).Center().Gap(8).Padding(20).Children(func() {
			ui.Icon(c, TablerMessage).Width(28).Height(28).TextColor(tc.IconMuted)
			ui.Text(c, "Side Conversation").FontSize(13).FontWeight(600).TextColor(tc.TextMuted)
			ui.Text(c, "Run quick queries or ask fast code questions without interrupting your main task session.").FontSize(12).TextColor(tc.TextMuted).MaxWidth(240)
		})
	} else {
		ui.List(c, &ui.ListState{FollowEnd: true}, len(a.SideChatMessages), func(i int) {
			if i >= len(a.SideChatMessages) {
				return
			}
			m := a.SideChatMessages[i]
			if m.Role == "user" {
				ui.Row(c.Key("side-user-"+fmt.Sprint(i))).Padding(3, 8).Justify(ui.End).Children(func() {
					bubble := ui.Column(c).Padding(8, 12).Radius(14).
						Background(tc.MessageSurface).BorderWidth(1).BorderColor(tc.Border.Alpha(0.5)).
						MaxWidth(300)
					bubble.Children(func() {
						ui.Text(c, m.Text).FontSize(13).TextColor(tc.MessageForeground)
					})
				})
			} else {
				ui.Column(c.Key("side-asst-"+fmt.Sprint(i))).Padding(3, 8).Children(func() {
					if m.Done {
						markdown.View(c, m.Text, &a.SideChatMd)
					} else {
						markdown.Stream(c, m.Text, &a.SideChatMd)
					}
				})
			}
		}).Grow(1).Padding(8)
	}

	// Bottom input composer
	ui.Row(c).Padding(8, 10).Gap(6).AlignItems(ui.Center).
		BorderWidth(1, 0, 0, 0).BorderColor(tc.Border).Children(func() {
		ui.TextInput(c, &a.SideChatInput).Placeholder("Ask side chat...").Grow(1)
		sendBtn := ui.Row(c).Size(28, 28).Radius(999).AlignItems(ui.Center).Justify(ui.Center).
			Background(tc.MessageAction)
		sendBtn.Children(func() {
			ui.Icon(c, TablerSend).Width(13).Height(13).TextColor(tc.MessageActionForeground)
		})
		pressed := sendBtn.Clicked()
		in := a.SideChatInput
		if strings.TrimSpace(in) != "" && (pressed || in == "\n") {
			text := strings.TrimSpace(a.SideChatInput)
			a.SideChatInput = ""
			a.sendSideChat(text)
		}
	})
}

// sendSideChat streams a side-conversation reply from the workspace LLM.
func (a *AppUI) sendSideChat(text string) {
	if a.SideChatStreaming {
		return
	}
	a.SideChatMessages = append(a.SideChatMessages, &ChatMessage{Role: "user", Text: text})
	reply := &ChatMessage{Role: "assistant", Text: "", Done: false}
	a.SideChatMessages = append(a.SideChatMessages, reply)
	a.SideChatStreaming = true
	if a.Window != nil {
		a.Window.Invalidate()
	}

	engine := a.Engine
	go func() {
		defer func() {
			a.SideChatStreaming = false
			reply.Done = true
			if a.Window != nil {
				a.Window.Invalidate()
			}
		}()
		if engine == nil || engine.LLMClient == nil {
			reply.Text = "No LLM provider configured. Set one up in Settings → Models & Providers."
			return
		}
		msgs := []llm.LLMMessage{{
			Role:    "user",
			Content: "Workspace: " + a.Workspace.RootDir + "\n\n" + text,
		}}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		_, err := engine.LLMClient.ChatWithStream(ctx, msgs, nil, func(deltaContent, deltaReasoning string) {
			if deltaContent != "" {
				reply.Text += deltaContent
			}
			if a.Window != nil {
				a.Window.Invalidate()
			}
		})
		if err != nil && reply.Text == "" {
			reply.Text = fmt.Sprintf("LLM request failed: %v", err)
		}
	}()
}

// renderStatusBar renders the 24px editor status bar: project, file,
// line/col, lines, language, UTF-8, branch and model.
func (a *AppUI) renderStatusBar(c *ui.Context) {
	tc := a.ThemeMgr.Colors()
	projectName := filepath.Base(a.Workspace.RootDir)

	curPath := ""
	lineCol := "Ln 1, Col 1"
	langLabel := "Plain Text"
	lineCount := 0

	if doc := a.Editor.ActiveDoc(); doc != nil {
		curPath = doc.RelativePath
		pos := doc.Editor.CursorPos()
		lineCol = FormatLineCol(pos.Line, pos.Col)
		lineCount = doc.Editor.Buffer.LineCount()
		if l := doc.Editor.Buffer.Language(); l != nil {
			langLabel = l.Name
		}
	}

	statusText := func(s string) {
		ui.Text(c, s).FontSize(11).TextColor(tc.TextMuted)
	}
	ui.Row(c).Height(StatusBarHeight).Gap(16).Padding(0, 12).AlignItems(ui.Center).
		Background(tc.Canvas).BorderWidth(1, 0, 0, 0).BorderColor(tc.Border).Children(func() {
		ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, TablerFolder).Width(12).Height(12).TextColor(tc.IconMuted)
			statusText(projectName)
		})
		if curPath != "" {
			ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, FileIconSVG(curPath)).Width(12).Height(12).TextColor(tc.IconMuted)
				statusText(curPath)
			})
		}
		ui.Box(c).Grow(1)

		statusText(lineCol)
		statusText(fmt.Sprintf("%d lines · %s", lineCount, langLabel))
		statusText("UTF-8")

		if a.Git != nil && a.Git.Branch != "" {
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, TablerGitBranch).Width(12).Height(12).TextColor(tc.Accent)
				ui.Text(c, a.Git.Branch).FontSize(11).TextColor(tc.Accent)
			})
		}

		ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, TablerCpu).Width(12).Height(12).TextColor(tc.IconMuted)
			statusText(a.Agent.ActiveModel)
		})
		statusText("⌘S Save · ⌘P Open · ⌘B Sidebar · ⌘1 Agent · ⌘2 Editor")
	})
}

// renderModals renders dialog overlays: Palette, Settings, ThemePicker, New File, New Task, Task Menu.
func (a *AppUI) renderModals(c *ui.Context) {
	t := c.Theme()
	tc := a.ThemeMgr.Colors()

	// 1. Quick Open Palette
	if a.Palette != nil {
		a.Palette.RenderPalette(c, a.ThemeMgr.Colors())
	}

	// 2. Settings Modal
	if a.Settings != nil {
		a.Settings.RenderSettings(c)
	}

	// 3. Theme & Appearance Picker Modal
	if a.ThemeMgr != nil {
		a.ThemeMgr.RenderThemePicker(c)
	}

	// 4. ACP Permission Approval Modal
	if a.ACPMgr != nil {
		a.ACPMgr.RenderPermissionModal(c)
	}

	// 5. New File Modal
	if a.ShowNewFileModal {
		ui.Modal(c, &a.ShowNewFileModal, func() {
			ui.Column(c).Width(420).Radius(12).Background(tc.SurfaceOverlay).BorderWidth(1).BorderColor(tc.Border).Padding(16, 18).Gap(10).
				Shadow(0, 24, 64, -24, ui.RGBA(0, 0, 0, 0.45)).
				Children(func() {
					ui.Text(c, "Create New File").FontSize(14).FontWeight(600)
					in := ui.TextInput(c, &a.NewFileName).Placeholder("filename.go (or path/to/file.ext)...").Grow(1).AutoFocus()
					create := func() {
						if a.NewFileName == "" {
							return
						}
						_ = a.Workspace.CreateFile(a.NewFileName)
						a.ShowNewFileModal = false
						full := a.Workspace.ResolveFullPath(a.NewFileName)
						_, _ = a.Editor.OpenFile(full, a.NewFileName)
						a.SelectedFile = a.NewFileName
						a.Mode = ModeEditor
					}
					if in.Submitted() {
						create()
					}
					ui.Row(c).AlignItems(ui.Center).Gap(6).Justify(ui.End).Children(func() {
						ui.Box(c).Grow(1)
						if ui.Button(c, "Cancel").Clicked() {
							a.ShowNewFileModal = false
						}
						if ui.PrimaryButton(c, "Create").Clicked() {
							create()
						}
					})
				})
		})
	}

	// 5b. New Folder Modal
	if a.ShowNewFolderModal {
		ui.Modal(c, &a.ShowNewFolderModal, func() {
			ui.Column(c).Width(420).Radius(12).Background(tc.SurfaceOverlay).BorderWidth(1).BorderColor(tc.Border).Padding(16, 18).Gap(10).
				Shadow(0, 24, 64, -24, ui.RGBA(0, 0, 0, 0.45)).
				Children(func() {
					ui.Text(c, "Create New Folder").FontSize(14).FontWeight(600)
					in := ui.TextInput(c, &a.NewFolderName).Placeholder("folder_name (or path/to/folder)...").Grow(1).AutoFocus()
					if in.Submitted() && a.NewFolderName != "" {
						_ = a.Workspace.CreateFolder(a.NewFolderName)
						a.ShowNewFolderModal = false
					}
					ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
						ui.Box(c).Grow(1)
						if ui.Button(c, "Cancel").Clicked() {
							a.ShowNewFolderModal = false
						}
						if ui.PrimaryButton(c, "Create").Clicked() && a.NewFolderName != "" {
							_ = a.Workspace.CreateFolder(a.NewFolderName)
							a.ShowNewFolderModal = false
						}
					})
				})
		})
	}

	// 6. Unified New Task Modal (ACP Agents + Agent CLIs) — 580px, like
	// NewTaskModal.tsx's w-[580px].
	if a.ShowNewTaskModal {
		ui.Modal(c, &a.ShowNewTaskModal, func() {
			ui.Column(c).Width(580).Radius(12).Background(tc.Sidebar).BorderWidth(1).BorderColor(tc.Border).Padding(0, 0, 12, 0).Gap(10).
				Shadow(0, 24, 72, -20, ui.RGBA(0, 0, 0, 0.5)).
				Children(func() {
					// Modal Header
					ui.Row(c).Height(48).Padding(0, 16).AlignItems(ui.Center).
						BorderWidth(0, 0, 1, 0).BorderColor(tc.Border.Alpha(0.6)).Children(func() {
						ui.Text(c, "New Task").FontSize(14).FontWeight(500)
						ui.Box(c).Grow(1)
						if IconButton(c, tc, TablerX, 13).Clicked() {
							a.ShowNewTaskModal = false
						}
					})

					ui.Column(c).Padding(4, 16).Gap(10).Children(func() {
						// Task Objective Input
						ui.Text(c, "TASK OBJECTIVE").FontSize(10).FontWeight(600).TextColor(tc.TextMuted)
						ui.TextInput(c, &a.NewTaskTitle).Placeholder("Describe task objective (e.g. Implement auth flow or fix bug)...").Grow(1).AutoFocus()

						// Agent Protocol Selector
						ui.Row(c).Gap(6).Children(func() {
							protoTab := func(label string, icon *ui.SVG, active bool, onPick func()) {
								btn := ui.Row(c).Height(28).Radius(6).Padding(0, 10).Gap(5).AlignItems(ui.Center)
								btn.Children(func() {
									ui.Icon(c, icon).Width(13).Height(13).TextColor(tc.IconMuted)
									ui.Text(c, label).FontSize(12).FontWeight(500)
								})
								if active {
									btn.Background(tc.Text.Alpha(0.1))
								} else if btn.Hovered() {
									btn.Background(tc.Text.Alpha(0.05))
								}
								if btn.Clicked() {
									onPick()
								}
							}
							protoTab("ACP Agents", TablerSparkles, a.NewTaskAgentType == "acp", func() { a.NewTaskAgentType = "acp" })
							protoTab("Interactive Agent CLIs", TablerTerminal, a.NewTaskAgentType == "cli", func() { a.NewTaskAgentType = "cli" })
						})

						// Agent List Selection: rows with 32px icon tiles
						ui.Scroll(c).Height(200).Children(func() {
							ui.Column(c).Gap(2).Children(func() {
								if a.NewTaskAgentType == "acp" && a.ACPMgr != nil {
									for _, agent := range a.ACPMgr.Agents {
										ag := agent
										isSelected := (a.SelectedAgentID == ag.ID)
										row := ui.Row(c.Key("newtask-agent-"+ag.ID)).Radius(8).Padding(8, 10).Gap(10).AlignItems(ui.Center)
										row.Children(func() {
											tile := ui.Box(c).Size(32, 32).Radius(8).AlignItems(ui.Center).Justify(ui.Center).
												Background(tc.Canvas).BorderWidth(1).BorderColor(tc.Border)
											tile.Children(func() {
												ui.Icon(c, TablerSparkles).Width(16).Height(16).TextColor(tc.Accent)
											})
											ui.Column(c).Grow(1).Children(func() {
												ui.Text(c, ag.Name).FontSize(13).FontWeight(500)
												ui.Text(c, ag.Description).FontSize(11).TextColor(tc.TextMuted).MaxWidth(380)
											})
											if ag.Available {
												Badge(c, tc, "READY", tc.Accent)
											} else {
												Badge(c, tc, "NOT INSTALLED", tc.TextMuted)
											}
										})
										if isSelected {
											row.Background(tc.Text.Alpha(0.06))
										} else if row.Hovered() {
											row.Background(tc.Text.Alpha(0.04))
										}
										if row.Clicked() {
											a.SelectedAgentID = ag.ID
											if len(ag.SupportedModels) > 0 {
												a.SelectedAgentModel = ag.SupportedModels[0]
											}
										}
									}
								} else if a.NewTaskAgentType == "cli" && a.CLIMgr != nil {
									installed := a.CLIMgr.InstalledConfigs()
									if len(installed) == 0 {
										ui.Text(c, "No installed agent CLIs detected on system.").FontSize(12).TextColor(tc.TextMuted).Padding(12, 10)
									}
									for _, cli := range installed {
										cfg := cli
										isSelected := (a.SelectedAgentID == cfg.ID)
										row := ui.Row(c.Key("newtask-cli-"+cfg.ID)).Radius(8).Padding(8, 10).Gap(10).AlignItems(ui.Center)
										row.Children(func() {
											tile := ui.Box(c).Size(32, 32).Radius(8).AlignItems(ui.Center).Justify(ui.Center).
												Background(tc.Canvas).BorderWidth(1).BorderColor(tc.Border)
											tile.Children(func() {
												ui.Icon(c, TablerTerminal).Width(16).Height(16).TextColor(tc.Accent)
											})
											ui.Column(c).Grow(1).Children(func() {
												ui.Text(c, cfg.Name).FontSize(13).FontWeight(500)
												ui.Text(c, fmt.Sprintf("Executable: %s · %s", cfg.Executable, cfg.ConfigRoot)).FontSize(11).TextColor(tc.TextMuted).MaxWidth(380)
											})
											Badge(c, tc, "INSTALLED", tc.Success)
										})
										if isSelected {
											row.Background(tc.Text.Alpha(0.06))
										} else if row.Hovered() {
											row.Background(tc.Text.Alpha(0.04))
										}
										if row.Clicked() {
											a.SelectedAgentID = cfg.ID
										}
									}
								}
							})
						})

						// Action buttons
						ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
							ui.Box(c).Grow(1)
							if ui.Button(c, "Cancel").Clicked() {
								a.ShowNewTaskModal = false
							}
							if ui.PrimaryButton(c, "Start Session").Clicked() {
								title := a.NewTaskTitle
								if title == "" {
									title = "New Task"
								}

								if a.NewTaskAgentType == "cli" && a.CLIMgr != nil {
									// Start interactive CLI task
									a.CLIMgr.CreateSession(a.SelectedAgentID, a.SelectedAgentID, a.SelectedAgentID, []string{}, a.Workspace.RootDir)
									a.ActiveTaskKind = "cli"
								} else {
									// Start ACP Agent session
									sess := a.Agent.NewSession(title)
									sess.AgentID = a.SelectedAgentID
									sess.Model = a.SelectedAgentModel
									a.Agent.SaveSessionToDisk(sess)
									a.ActiveTaskKind = "agent"
								}

								a.ShowNewTaskModal = false
								a.Mode = ModeAgent
							}
						})
					})
				})
		})
	}

	// 7. Task Menu Modal
	if a.ShowTaskMenuModal {
		ui.Modal(c, &a.ShowTaskMenuModal, func() {
			ui.Column(c).Width(280).Radius(12).Background(tc.SurfaceOverlay).BorderWidth(1).BorderColor(tc.Border).Padding(6).Gap(2).
				Shadow(0, 24, 64, -24, ui.RGBA(0, 0, 0, 0.45)).
				Children(func() {
					menuItem := func(icon *ui.SVG, label string, danger bool, onPick func()) {
						btn := SidebarActionButton(c, tc, icon, label, "", danger)
						if btn.Clicked() {
							onPick()
						}
					}
					menuItem(TablerPencil, "Rename Task", false, func() {
						a.ShowTaskMenuModal = false
						a.ShowRenameModal = true
						if sess := a.Agent.ActiveSession(); sess != nil {
							a.RenameSessionTitle = sess.Title
						}
					})
					menuItem(TablerGitFork, "Fork Session", false, func() {
						a.ShowTaskMenuModal = false
						if sess := a.Agent.ActiveSession(); sess != nil {
							a.Agent.NewSession(fmt.Sprintf("%s (fork)", sess.Title))
						}
					})
					menuItem(TablerTerminal, "Toggle Terminal", false, func() {
						a.ShowTaskMenuModal = false
						a.ShowBottomTerminal = !a.ShowBottomTerminal
					})
					menuItem(TablerChecklist, "Toggle Tasks Drawer", false, func() {
						a.ShowTaskMenuModal = false
						a.ShowRightDrawer = true
						a.RightDrawerTab = RightTabTasks
					})
					menuItem(TablerTrash, "Delete Session", true, func() {
						a.ShowTaskMenuModal = false
						if sess := a.Agent.ActiveSession(); sess != nil {
							a.Agent.DeleteSession(sess.ID)
						}
					})
				})
		})
	}

	// 8. Rename Task Modal
	if a.ShowRenameModal {
		ui.Modal(c, &a.ShowRenameModal, func() {
			ui.Column(c).Width(380).Radius(12).Background(tc.SurfaceOverlay).BorderWidth(1).BorderColor(tc.Border).Padding(16, 18).Gap(10).
				Shadow(0, 24, 64, -24, ui.RGBA(0, 0, 0, 0.45)).
				Children(func() {
					ui.Text(c, "Rename Task").FontSize(14).FontWeight(600)
					in := ui.TextInput(c, &a.RenameSessionTitle).Grow(1).AutoFocus()
					save := func() {
						if a.RenameSessionTitle == "" {
							return
						}
						if sess := a.Agent.ActiveSession(); sess != nil {
							sess.Title = a.RenameSessionTitle
							a.Agent.SaveSessionToDisk(sess)
						}
						a.ShowRenameModal = false
					}
					if in.Submitted() {
						save()
					}
					ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
						ui.Box(c).Grow(1)
						if ui.Button(c, "Cancel").Clicked() {
							a.ShowRenameModal = false
						}
						if ui.PrimaryButton(c, "Save").Clicked() {
							save()
						}
					})
				})
		})
	}
	_ = t
}

// renderBoardPane renders the workflow board drawer pane (board.tasks.json).
func (a *AppUI) renderBoardPane(c *ui.Context) {
	tc := a.ThemeMgr.Colors()
	ui.Row(c).Height(40).Padding(0, 12).AlignItems(ui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
		ui.Text(c, "Workflow Board").FontSize(13).FontWeight(600)
		ui.Box(c).Grow(1)
		if IconButton(c, tc, TablerRefresh, 12).Clicked() {
			a.RefreshBoard()
		}
	})
	if a.Board == nil {
		a.RefreshBoard()
	}
	if a.Board == nil || a.Board.Data == nil || len(a.Board.Data.Columns) == 0 {
		ui.Column(c).Grow(1).Center().Gap(8).Padding(20).Children(func() {
			ui.Icon(c, TablerLayoutKanban).Width(32).Height(32).TextColor(tc.IconMuted)
			ui.Text(c, "No board tasks").FontSize(13).FontWeight(600).TextColor(tc.TextMuted)
			ui.Text(c, "Tasks in .forge/board_tasks.json appear here.").FontSize(12).TextColor(tc.TextMuted).MaxWidth(240)
		})
		return
	}
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Padding(8, 10).Gap(10).Children(func() {
			for _, col := range a.Board.Data.Columns {
				ui.Column(c).Gap(4).Children(func() {
					ui.Row(c).AlignItems(ui.Center).Gap(6).Padding(0, 2).Children(func() {
						ui.Text(c, col.Name).FontSize(11).FontWeight(600).TextColor(tc.TextMuted)
						ui.Box(c).Size(18, 16).Radius(8).AlignItems(ui.Center).Justify(ui.Center).
							Background(tc.Text.Alpha(0.08)).Children(func() {
							ui.Text(c, fmt.Sprintf("%d", len(col.Tasks))).FontSize(10).TextColor(tc.TextMuted)
						})
					})
					for _, task := range col.Tasks {
						card := ui.Column(c.Key("board-"+task.ID)).Radius(8).Padding(8, 10).Gap(4).
							Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
						card.Children(func() {
							ui.Text(c, task.Title).FontSize(12)
							if task.Status != "" {
								Badge(c, tc, task.Status, tc.Accent)
							}
						})
					}
				})
			}
		})
	})
}

// minF is a float min helper for layout math.
func minF(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}
