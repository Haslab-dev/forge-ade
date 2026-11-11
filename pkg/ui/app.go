package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// AppUI is the top-level native UI application state matching ForgeADE's Dual-Surface design.
type AppUI struct {
	Engine              *Engine
	Window              *mygo.Window
	Workspace           *WorkspaceState
	Editor              *EditorManager
	Terminal            *TerminalState
	Agent               *AgentState
	Git                 *GitManager
	Database            *DatabaseManager
	Palette             *PaletteManager
	Settings            *SettingsManager
	ThemeMgr            *ThemeManager
	ACPMgr              *ACPManager
	CLIMgr              *AgentCLIManager
	Mode                int    // ModeAgent (0) or ModeEditor (1)
	ActiveTaskKind      string // "agent" or "cli"
	ShowLeftSidebar     bool   // Left sidebar visibility
	ShowBottomTerminal  bool   // Collapsible bottom terminal drawer
	ShowRightDrawer     bool   // Collapsible right side pane
	RightDrawerTab      int    // RightTabGit, RightTabDatabase, RightTabTasks, RightTabAgent
	EditorActivity      int    // ActivityExplorer, ActivityGit, ActivityDatabase
	SelectedFile        string
	ShowNewFileModal    bool
	NewFileName         string
	ShowNewFolderModal  bool
	NewFolderName       string
	ShowNewTaskModal    bool
	NewTaskTitle        string
	NewTaskAgentType    string // "acp" or "cli"
	SelectedAgentID     string
	SelectedAgentModel  string
	ShowTaskMenuModal   bool
	ShowRenameModal     bool
	RenameSessionTitle  string
}

// NewAppUI constructs the complete AppUI state tree.
func NewAppUI(engine *Engine, rootDir string) *AppUI {
	appUI := &AppUI{
		Engine:              engine,
		Workspace:           NewWorkspaceState(rootDir),
		Editor:              NewEditorManager(),
		Terminal:            NewTerminalState(rootDir),
		ThemeMgr:            NewThemeManager(),
		Mode:                ModeAgent,
		ActiveTaskKind:      "agent",
		ShowLeftSidebar:     true,
		ShowBottomTerminal:  false,
		ShowRightDrawer:     false,
		RightDrawerTab:      RightTabGit,
		EditorActivity:      ActivityExplorer,
		NewTaskAgentType:    "acp",
		SelectedAgentID:     "agent-internal",
		SelectedAgentModel:  "Claude 3.7 Sonnet",
	}

	appUI.Agent = NewAgentState(engine, rootDir, func() {
		if appUI.Window != nil {
			appUI.Window.Invalidate()
		}
	})

	appUI.ACPMgr = NewACPManager(engine, func() {
		if appUI.Window != nil {
			appUI.Window.Invalidate()
		}
	})

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
		if showDiff {
			appUI.Editor.IsDiffView = true
		} else {
			appUI.Editor.IsDiffView = false
		}
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
	if c.Shortcut(ui.Cmd, ui.Key1) {
		a.Mode = ModeAgent
	}
	if c.Shortcut(ui.Cmd, ui.Key2) {
		a.Mode = ModeEditor
	}
	if c.Shortcut(ui.Cmd, ui.KeyJ) {
		a.ShowRightDrawer = !a.ShowRightDrawer
	}

	ui.Column(c).Fill().Children(func() {
		// 1. Top Unified Workspace Header (WorkspaceHeader)
		a.renderWorkspaceHeader(c)

		// 2. Middle Main Viewport (Agent or Editor + Sidebars + Drawers)
		ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
			if a.Mode == ModeAgent {
				a.renderAgentSurface(c)
			} else {
				a.renderEditorSurface(c)
			}
		})

		// 3. Status Bar (Always present in Editor mode)
		if a.Mode == ModeEditor {
			a.renderStatusBar(c)
		}

		// 4. Overlays & Modals
		a.renderModals(c)
	})
}

// renderWorkspaceHeader renders ForgeADE's WorkspaceHeader: title, project chip, branch chip, mode switcher, dock toggles.
func (a *AppUI) renderWorkspaceHeader(c *ui.Context) {
	t := c.Theme()
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

	ui.Row(c).Gap(10).Padding(7, 14).AlignItems(ui.Center).Background(t.Surface).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
		// Left: Sidebar Toggle
		sidebarBtn := ui.Button(c, map[bool]string{true: "◧", false: "◻"}[a.ShowLeftSidebar]).FontSize(12)
		if sidebarBtn.Clicked() {
			a.ShowLeftSidebar = !a.ShowLeftSidebar
		}

		// Title
		ui.Text(c, title).Bold().FontSize(13).MaxWidth(280)

		// Project folder chip
		projectChip := ui.Button(c, projectName).FontSize(11)
		if projectChip.Clicked() {
			a.Palette.Toggle()
		}

		// Git Branch chip
		if a.Git != nil && a.Git.Branch != "" {
			dirtyMarker := ""
			if len(a.Git.Staged)+len(a.Git.Unstaged)+len(a.Git.Untracked) > 0 {
				dirtyMarker = " *"
			}
			branchChip := ui.Button(c, fmt.Sprintf("%s%s", a.Git.Branch, dirtyMarker)).FontSize(11)
			if branchChip.Clicked() {
				a.ShowRightDrawer = true
				a.RightDrawerTab = RightTabGit
			}
		}

		// Task "…" menu
		moreBtn := ui.Button(c, "…").FontSize(12)
		if moreBtn.Clicked() {
			a.ShowTaskMenuModal = true
		}

		ui.Box(c).Grow(1)

		// Mode Switcher: [✨ Agent] | [💻 Editor]
		ui.Segmented(c, &a.Mode, "Agent", "Editor").Label("Surface")

		ui.Box(c).Grow(1)

		// Right controls: Search, Theme Picker, Bottom Terminal Toggle, Side Pane Toggle, Settings
		searchBtn := ui.Button(c, "Find (⌘P)").FontSize(11)
		if searchBtn.Clicked() {
			a.Palette.Toggle()
		}

		// Theme & Palette Picker button
		themeBtn := ui.Button(c, "Theme").FontSize(11)
		if themeBtn.Clicked() {
			a.ThemeMgr.Toggle()
		}

		var termToggleBtn ui.Element
		if a.ShowBottomTerminal {
			termToggleBtn = ui.PrimaryButton(c, "Terminal").FontSize(11)
		} else {
			termToggleBtn = ui.Button(c, "Terminal").FontSize(11)
		}
		if termToggleBtn.Clicked() {
			a.ShowBottomTerminal = !a.ShowBottomTerminal
		}

		var sideDrawerBtn ui.Element
		if a.ShowRightDrawer {
			sideDrawerBtn = ui.PrimaryButton(c, "Side Pane").FontSize(11)
		} else {
			sideDrawerBtn = ui.Button(c, "Side Pane").FontSize(11)
		}
		if sideDrawerBtn.Clicked() {
			a.ShowRightDrawer = !a.ShowRightDrawer
		}

		settingsBtn := ui.Button(c, "Settings").FontSize(11)
		if settingsBtn.Clicked() {
			a.Settings.Toggle()
		}
	})
}

// renderAgentSurface renders the Agent mode: Left AgentSidebar, Center AgentContainer, Collapsible Terminal & Right Drawer.
func (a *AppUI) renderAgentSurface(c *ui.Context) {
	// 1. Left Agent Sidebar
	if a.ShowLeftSidebar {
		a.renderAgentSidebar(c)
	}

	// 2. Center Column: Main Chat Container (or CLI surface) + Collapsible Bottom Terminal
	ui.Column(c).Grow(1).FillHeight().Children(func() {
		// Agent Main Container
		ui.Column(c).Grow(1).FillHeight().Children(func() {
			if a.ActiveTaskKind == "cli" && a.CLIMgr != nil && a.CLIMgr.ActiveSession() != nil {
				a.CLIMgr.RenderTerminalSessionSurface(c, a.CLIMgr.ActiveID)
			} else {
				a.Agent.RenderChat(c)
			}
		})

		// Collapsible Bottom Terminal
		if a.ShowBottomTerminal {
			a.renderBottomTerminal(c)
		}
	})

	// 3. Collapsible Right Drawer (Git, DB, Tasks)
	if a.ShowRightDrawer {
		a.renderRightDrawer(c)
	}
}

// renderAgentSidebar renders the full-height task sessions sidebar with filter tabs and profile row.
func (a *AppUI) renderAgentSidebar(c *ui.Context) {
	t := c.Theme()

	ui.Column(c).Width(260).Background(t.Surface).BorderWidth(0, 1, 0, 0).BorderColor(t.Border).Children(func() {
		// Top brand bar
		ui.Row(c).Padding(10, 14).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			ui.Text(c, "ForgeADE").Bold().FontSize(13)
			ui.Text(c, "Agent").FontSize(11).TextColor(t.TextMuted)
			ui.Box(c).Grow(1)
			if ui.Button(c, "◧").FontSize(11).Clicked() {
				a.ShowLeftSidebar = false
			}
		})

		// Primary Action Buttons: + New Task (⌘N), Search (⌘P)
		ui.Column(c).Padding(8, 12).Gap(6).Children(func() {
			if ui.PrimaryButton(c, "+ New Task (⌘N)").Clicked() {
				a.ShowNewTaskModal = true
				a.NewTaskTitle = ""
			}
			if ui.Button(c, "🔍 Search (⌘P)").Clicked() {
				a.Palette.Toggle()
			}
		})

		// Project & Sort Header
		ui.Row(c).Padding(4, 14).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "TASKS").FontSize(10).Bold().TextColor(t.TextMuted)
			ui.Box(c).Grow(1)
			if ui.Button(c, "+").FontSize(11).Clicked() {
				a.Agent.NewSession("")
				a.ActiveTaskKind = "agent"
			}
		})

		// Search Input Filter
		ui.Row(c).Padding(4, 12).Children(func() {
			ui.TextInput(c, &a.Agent.SearchQuery).Placeholder("Search tasks...").Grow(1)
		})

		// Filter Tabs: Active, Pinned, Archived with counts
		activeCount, pinnedCount, archivedCount := a.Agent.TabCounts()
		tabCounts := map[string]int{
			"active":   activeCount,
			"pinned":   pinnedCount,
			"archived": archivedCount,
		}
		ui.Row(c).Gap(4).Padding(6, 12).AlignItems(ui.Center).Children(func() {
			for _, tab := range []string{"active", "pinned", "archived"} {
				thisTab := tab
				label := fmt.Sprintf("%s (%d)", strings.Title(thisTab), tabCounts[thisTab])
				var btn ui.Element
				if a.Agent.ActiveFilterTab == thisTab {
					btn = ui.PrimaryButton(c, label).FontSize(10)
				} else {
					btn = ui.Button(c, label).FontSize(10)
				}
				if btn.Clicked() {
					a.Agent.ActiveFilterTab = thisTab
				}
			}
		})

		// Scrollable Task Sessions List
		sessions := a.Agent.FilteredSessions()
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Padding(4, 8).Gap(4).Children(func() {
				// 1. Agent ACP Sessions
				for _, sess := range sessions {
					s := sess
					isActive := (a.ActiveTaskKind == "agent" && s.ID == a.Agent.ActiveSessionID)
					statusIcon := "•"
					if s.Status == "running" {
						statusIcon = "●"
					} else if s.Status == "completed" {
						statusIcon = "✔"
					}

					card := ui.Row(c.Key("sess-"+s.ID)).Padding(8, 10).Radius(6).Gap(8).AlignItems(ui.Center)
					if isActive {
						card.Background(t.Border)
					}
					card.Children(func() {
						ui.Text(c, statusIcon).FontSize(10)
						ui.Column(c).Grow(1).Children(func() {
							ui.Text(c, s.Title).Bold().FontSize(12).MaxWidth(160)
							ui.Row(c).Gap(6).Children(func() {
								ui.Text(c, s.Model).FontSize(10).TextColor(t.TextMuted)
							})
						})

						// Pin button
						pinLabel := "☆"
						if s.Pinned {
							pinLabel = "★"
						}
						if ui.Button(c, pinLabel).FontSize(11).Clicked() {
							a.Agent.TogglePin(s.ID)
						}
					})
					if card.Clicked() {
						a.ActiveTaskKind = "agent"
						a.Agent.SelectSession(s.ID)
					}
				}

				// 2. Active CLI Sessions
				if a.CLIMgr != nil {
					for _, cliSess := range a.CLIMgr.Sessions {
						cs := cliSess
						isActive := (a.ActiveTaskKind == "cli" && a.CLIMgr.ActiveID == cs.ID)
						cliCard := ui.Row(c.Key("cli-sess-"+cs.ID)).Padding(8, 10).Radius(6).Gap(8).AlignItems(ui.Center)
						if isActive {
							cliCard.Background(t.Border)
						}
						cliCard.Children(func() {
							ui.Text(c, "CLI").FontSize(10).Bold().TextColor(t.Accent)
							ui.Column(c).Grow(1).Children(func() {
								ui.Text(c, cs.Title).Bold().FontSize(12).MaxWidth(160)
								ui.Text(c, fmt.Sprintf("%s · CLI Task", cs.AgentName)).FontSize(10).TextColor(t.TextMuted)
							})
						})
						if cliCard.Clicked() {
							a.ActiveTaskKind = "cli"
							a.CLIMgr.ActiveID = cs.ID
						}
					}
				}
			})
		})

		// Bottom Profile / Utility Row
		ui.Row(c).Padding(10, 14).AlignItems(ui.Center).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Background(t.Surface).Children(func() {
			ui.Column(c).Grow(1).Children(func() {
				ui.Text(c, "Model Profile").FontSize(10).Bold().TextColor(t.TextMuted)
				ui.Text(c, a.Agent.ActiveModel).FontSize(11).Bold()
			})

			// Utility buttons: Tasks, Git, Settings
			if ui.Button(c, "Tasks").FontSize(11).Clicked() {
				a.ShowRightDrawer = true
				a.RightDrawerTab = RightTabTasks
			}
			if ui.Button(c, "Git").FontSize(11).Clicked() {
				a.ShowRightDrawer = true
				a.RightDrawerTab = RightTabGit
			}
			if ui.Button(c, "Settings").FontSize(11).Clicked() {
				a.Settings.Toggle()
			}
		})
	})
}

// renderEditorSurface renders the Editor mode: ActivityBar + FileTree, EditorView (tabs + buffer/diff), Bottom Terminal, Right Drawer.
func (a *AppUI) renderEditorSurface(c *ui.Context) {
	t := c.Theme()

	// 1. Left Activity Bar + Sidebar
	if a.ShowLeftSidebar {
		// Activity Bar (Explorer, Git, DB, Settings)
		ui.Column(c).Width(48).Background(t.Surface).BorderWidth(0, 1, 0, 0).BorderColor(t.Border).Padding(8, 0).AlignItems(ui.Center).Gap(8).Children(func() {
			var expBtn ui.Element
			if a.EditorActivity == ActivityExplorer {
				expBtn = ui.PrimaryButton(c, "📁").FontSize(14)
			} else {
				expBtn = ui.Button(c, "📁").FontSize(14)
			}
			if expBtn.Clicked() {
				a.EditorActivity = ActivityExplorer
			}

			var gitBtn ui.Element
			if a.EditorActivity == ActivityGit {
				gitBtn = ui.PrimaryButton(c, "⎇").FontSize(14)
			} else {
				gitBtn = ui.Button(c, "⎇").FontSize(14)
			}
			if gitBtn.Clicked() {
				a.EditorActivity = ActivityGit
			}

			var dbBtn ui.Element
			if a.EditorActivity == ActivityDatabase {
				dbBtn = ui.PrimaryButton(c, "🗄").FontSize(14)
			} else {
				dbBtn = ui.Button(c, "🗄").FontSize(14)
			}
			if dbBtn.Clicked() {
				a.EditorActivity = ActivityDatabase
			}

			ui.Box(c).Grow(1)

			if ui.Button(c, "⚙").FontSize(14).Clicked() {
				a.Settings.Toggle()
			}
		})

		// File Tree or Activity Panel
		ui.Column(c).Width(230).Background(t.Surface).BorderWidth(0, 1, 0, 0).BorderColor(t.Border).Children(func() {
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

		// Collapsible Bottom Terminal
		if a.ShowBottomTerminal {
			a.renderBottomTerminal(c)
		}
	})

	// 3. Collapsible Right Drawer (Git, DB, Tasks, or Agent Chat)
	if a.ShowRightDrawer {
		a.renderRightDrawer(c)
	}
}

func fileIconBadge(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "🔵"
	case ".ts", ".tsx":
		return "🟦"
	case ".js", ".jsx":
		return "🟨"
	case ".json":
		return "🟧"
	case ".md":
		return "📝"
	case ".html":
		return "🌐"
	case ".css":
		return "🎨"
	case ".sql", ".db", ".sqlite", ".duckdb":
		return "🗄"
	case ".sh", ".bash", ".zsh":
		return "⚡"
	case ".yml", ".yaml", ".toml":
		return "⚙"
	default:
		return "📄"
	}
}

// renderFileTree renders the workspace file explorer tree.
func (a *AppUI) renderFileTree(c *ui.Context) {
	t := c.Theme()

	ui.Row(c).Padding(10, 12).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
		ui.Text(c, "FILES").FontSize(10).Bold().TextColor(t.TextMuted)
		ui.Box(c).Grow(1)
		if ui.Button(c, "+ File").FontSize(11).Clicked() {
			a.ShowNewFileModal = true
			a.NewFileName = ""
		}
		if ui.Button(c, "+ Folder").FontSize(11).Clicked() {
			a.ShowNewFolderModal = true
			a.NewFolderName = ""
		}
		if ui.Button(c, "↻").FontSize(11).Clicked() {
			a.Workspace.Refresh()
			if a.Git != nil {
				go a.Git.Refresh()
			}
			if a.Database != nil {
				go a.Database.ScanDatabases()
			}
		}
	})

	// Search filter
	ui.Row(c).Padding(4, 10).Children(func() {
		ui.TextInput(c, &a.Workspace.FilterQuery).Placeholder("Filter files...").Grow(1)
	})

	// File List
	filtered := a.Workspace.FilteredFiles()
	ui.Sidebar(c, &a.SelectedFile, func() {
		for _, f := range filtered {
			icon := fileIconBadge(f.RelativePath)
			label := fmt.Sprintf("%s %s", icon, f.RelativePath)
			doc := a.Editor.GetDoc(f.FullPath)
			if doc != nil && doc.Editor.Buffer.Modified() {
				label += " •"
			}
			item := ui.SidebarItem(c, f.RelativePath, nil, label)
			if item.Clicked() {
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
	}).Grow(1).Label("Files")
}

// renderBottomTerminal renders the embedded Ghostty terminal drawer.
func (a *AppUI) renderBottomTerminal(c *ui.Context) {
	t := c.Theme()

	ui.Column(c).Height(220).Background(t.Surface).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
		// Terminal Drawer Header
		ui.Row(c).Padding(6, 12).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			ui.Text(c, ">_ Terminal (bash)").Bold().FontSize(12)
			ui.Box(c).Grow(1)
			if ui.Button(c, "✕").FontSize(11).Clicked() {
				a.ShowBottomTerminal = false
			}
		})

		// Terminal Content
		a.Terminal.RenderTerminal(c)
	})
}

// renderRightDrawer renders the collapsible side drawer.
func (a *AppUI) renderRightDrawer(c *ui.Context) {
	t := c.Theme()

	ui.Column(c).Width(380).Background(t.Surface).BorderWidth(0, 0, 0, 1).BorderColor(t.Border).Children(func() {
		// Drawer Header Tabs
		ui.Row(c).Gap(4).Padding(6, 10).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			tabNames := []string{"⎇ Git", "🗄 DB", "📋 Tasks"}
			if a.Mode == ModeEditor {
				tabNames = append(tabNames, "✨ Agent")
			}
			for i, name := range tabNames {
				idx := i
				var btn ui.Element
				if a.RightDrawerTab == idx {
					btn = ui.PrimaryButton(c, name).FontSize(11)
				} else {
					btn = ui.Button(c, name).FontSize(11)
				}
				if btn.Clicked() {
					a.RightDrawerTab = idx
				}
			}
			ui.Box(c).Grow(1)
			if ui.Button(c, "✕").FontSize(11).Clicked() {
				a.ShowRightDrawer = false
			}
		})

		// Drawer Content
		ui.Column(c).Grow(1).FillHeight().Children(func() {
			switch a.RightDrawerTab {
			case RightTabGit:
				if a.Git != nil {
					a.Git.RenderGit(c)
				}
			case RightTabDatabase:
				if a.Database != nil {
					a.Database.RenderDatabase(c)
				}
			case RightTabTasks:
				if a.Agent != nil && a.Agent.TaskManager != nil {
					a.Agent.TaskManager.RenderChecklist(c)
				}
			case RightTabAgent:
				if a.Agent != nil {
					a.Agent.RenderChat(c)
				}
			}
		})
	})
}

// renderStatusBar renders Zed/ForgeADE-style bottom status bar for Editor mode.
func (a *AppUI) renderStatusBar(c *ui.Context) {
	t := c.Theme()
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

	ui.Row(c).Gap(16).Padding(4, 14).AlignItems(ui.Center).Background(t.Surface).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
		ui.Text(c, fmt.Sprintf("📁 %s", projectName)).FontSize(11).TextColor(t.TextMuted)
		if curPath != "" {
			ui.Text(c, curPath).FontSize(11)
		}
		ui.Box(c).Grow(1)

		ui.Text(c, lineCol).FontSize(11).TextColor(t.TextMuted)
		ui.Text(c, fmt.Sprintf("%d lines · %s", lineCount, langLabel)).FontSize(11).TextColor(t.TextMuted)
		ui.Text(c, "UTF-8").FontSize(11).TextColor(t.TextMuted)

		if a.Git != nil && a.Git.Branch != "" {
			ui.Text(c, fmt.Sprintf("⎇ %s", a.Git.Branch)).FontSize(11).TextColor(t.Accent)
		}

		ui.Text(c, a.Agent.ActiveModel).FontSize(11).TextColor(t.TextMuted)
		ui.Text(c, "⌘S Save · ⌘P Open · ⌘B Sidebar · ⌘1 Agent · ⌘2 Editor").FontSize(10).TextColor(t.TextMuted)
	})
}

// renderModals renders dialog overlays: Palette, Settings, ThemePicker, New File, New Task, Task Menu.
func (a *AppUI) renderModals(c *ui.Context) {
	t := c.Theme()

	// 1. Quick Open Palette
	if a.Palette != nil {
		a.Palette.RenderPalette(c)
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
			ui.Column(c).Width(400).Radius(10).Background(t.Surface).BorderWidth(1).BorderColor(t.Border).Padding(16, 18).Gap(10).Children(func() {
				ui.Text(c, "Create New File").Bold().FontSize(14)
				in := ui.TextInput(c, &a.NewFileName).Placeholder("filename.go (or path/to/file.ext)...").Grow(1).AutoFocus()
				if in.Submitted() && a.NewFileName != "" {
					_ = a.Workspace.CreateFile(a.NewFileName)
					a.ShowNewFileModal = false
					full := a.Workspace.ResolveFullPath(a.NewFileName)
					_, _ = a.Editor.OpenFile(full, a.NewFileName)
					a.SelectedFile = a.NewFileName
					a.Mode = ModeEditor
				}
				ui.Row(c).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Grow(1)
					if ui.Button(c, "Cancel").Clicked() {
						a.ShowNewFileModal = false
					}
					if ui.PrimaryButton(c, "Create").Clicked() && a.NewFileName != "" {
						_ = a.Workspace.CreateFile(a.NewFileName)
						a.ShowNewFileModal = false
						full := a.Workspace.ResolveFullPath(a.NewFileName)
						_, _ = a.Editor.OpenFile(full, a.NewFileName)
						a.SelectedFile = a.NewFileName
						a.Mode = ModeEditor
					}
				})
			})
		})
	}

	// 5b. New Folder Modal
	if a.ShowNewFolderModal {
		ui.Modal(c, &a.ShowNewFolderModal, func() {
			ui.Column(c).Width(400).Radius(10).Background(t.Surface).BorderWidth(1).BorderColor(t.Border).Padding(16, 18).Gap(10).Children(func() {
				ui.Text(c, "Create New Folder").Bold().FontSize(14)
				in := ui.TextInput(c, &a.NewFolderName).Placeholder("folder_name (or path/to/folder)...").Grow(1).AutoFocus()
				if in.Submitted() && a.NewFolderName != "" {
					_ = a.Workspace.CreateFolder(a.NewFolderName)
					a.ShowNewFolderModal = false
				}
				ui.Row(c).AlignItems(ui.Center).Children(func() {
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

	// 6. Unified New Task Modal (ACP Agents + Agent CLIs)
	if a.ShowNewTaskModal {
		ui.Modal(c, &a.ShowNewTaskModal, func() {
			ui.Column(c).Width(520).Radius(12).Background(t.Surface).BorderWidth(1).BorderColor(t.Border).Padding(16, 20).Gap(12).Children(func() {
				// Modal Header
				ui.Row(c).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "✨ New Task Session").Bold().FontSize(15)
					ui.Box(c).Grow(1)
					if ui.Button(c, "✕").FontSize(12).Clicked() {
						a.ShowNewTaskModal = false
					}
				})

				// Task Objective Input
				ui.Text(c, "TASK OBJECTIVE").FontSize(10).Bold().TextColor(t.TextMuted)
				ui.TextInput(c, &a.NewTaskTitle).Placeholder("Describe task objective (e.g. Implement auth flow or fix bug)...").Grow(1).AutoFocus()

				// Agent Protocol Selector: ACP Agents vs Agent CLIs
				ui.Row(c).Gap(6).Children(func() {
					var acpTab ui.Element
					if a.NewTaskAgentType == "acp" {
						acpTab = ui.PrimaryButton(c, "✨ ACP Agents").FontSize(11)
					} else {
						acpTab = ui.Button(c, "✨ ACP Agents").FontSize(11)
					}
					if acpTab.Clicked() {
						a.NewTaskAgentType = "acp"
					}

					var cliTab ui.Element
					if a.NewTaskAgentType == "cli" {
						cliTab = ui.PrimaryButton(c, "💻 Interactive Agent CLIs").FontSize(11)
					} else {
						cliTab = ui.Button(c, "💻 Interactive Agent CLIs").FontSize(11)
					}
					if cliTab.Clicked() {
						a.NewTaskAgentType = "cli"
					}
				})

				// Agent List Selection
				ui.Scroll(c).Height(180).Children(func() {
					ui.Column(c).Gap(4).Children(func() {
						if a.NewTaskAgentType == "acp" && a.ACPMgr != nil {
							for _, agent := range a.ACPMgr.Agents {
								ag := agent
								isSelected := (a.SelectedAgentID == ag.ID)
								row := ui.Row(c.Key("newtask-agent-"+ag.ID)).Padding(6, 10).Radius(6).Gap(8).AlignItems(ui.Center)
								if isSelected {
									row.Background(t.Border)
								}
								row.Children(func() {
									ui.Column(c).Grow(1).Children(func() {
										ui.Text(c, ag.Name).Bold().FontSize(12)
										ui.Text(c, ag.Description).FontSize(10).TextColor(t.TextMuted).MaxWidth(360)
									})
									if ag.Available {
										ui.Text(c, "✔ Ready").FontSize(10).TextColor(t.Accent)
									} else {
										ui.Text(c, "Not on PATH").FontSize(10).TextColor(t.TextMuted)
									}
								})
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
								ui.Text(c, "No installed agent CLIs detected on system.").FontSize(11).TextColor(t.TextMuted).Padding(12, 10)
							}
							for _, cli := range installed {
								cfg := cli
								isSelected := (a.SelectedAgentID == cfg.ID)
								row := ui.Row(c.Key("newtask-cli-"+cfg.ID)).Padding(6, 10).Radius(6).Gap(8).AlignItems(ui.Center)
								if isSelected {
									row.Background(t.Border)
								}
								row.Children(func() {
									ui.Column(c).Grow(1).Children(func() {
										ui.Text(c, cfg.Name).Bold().FontSize(12)
										ui.Text(c, fmt.Sprintf("Executable: %s · %s", cfg.Executable, cfg.ConfigRoot)).FontSize(10).TextColor(t.TextMuted)
									})
									ui.Text(c, "Installed").FontSize(10).TextColor(t.Accent)
								})
								if row.Clicked() {
									a.SelectedAgentID = cfg.ID
								}
							}
						}
					})
				})

				// Action buttons
				ui.Row(c).AlignItems(ui.Center).Children(func() {
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
	}

	// 7. Task Menu Modal
	if a.ShowTaskMenuModal {
		ui.Modal(c, &a.ShowTaskMenuModal, func() {
			ui.Column(c).Width(280).Radius(10).Background(t.Surface).BorderWidth(1).BorderColor(t.Border).Padding(12, 14).Gap(6).Children(func() {
				ui.Text(c, "Task Actions").Bold().FontSize(13).Padding(0, 0, 4, 0)

				if ui.Button(c, "✏ Rename Task").Clicked() {
					a.ShowTaskMenuModal = false
					a.ShowRenameModal = true
					if sess := a.Agent.ActiveSession(); sess != nil {
						a.RenameSessionTitle = sess.Title
					}
				}

				if ui.Button(c, "⎇ Fork Session").Clicked() {
					a.ShowTaskMenuModal = false
					if sess := a.Agent.ActiveSession(); sess != nil {
						a.Agent.NewSession(fmt.Sprintf("%s (fork)", sess.Title))
					}
				}

				if ui.Button(c, "⌨ Toggle Terminal").Clicked() {
					a.ShowTaskMenuModal = false
					a.ShowBottomTerminal = !a.ShowBottomTerminal
				}

				if ui.Button(c, "📋 Toggle Tasks Drawer").Clicked() {
					a.ShowTaskMenuModal = false
					a.ShowRightDrawer = true
					a.RightDrawerTab = RightTabTasks
				}

				if ui.Button(c, "🗑 Delete Session").Clicked() {
					a.ShowTaskMenuModal = false
					if sess := a.Agent.ActiveSession(); sess != nil {
						a.Agent.DeleteSession(sess.ID)
					}
				}

				ui.Box(c).Padding(2, 0)
				if ui.Button(c, "Close").Clicked() {
					a.ShowTaskMenuModal = false
				}
			})
		})
	}

	// 8. Rename Task Modal
	if a.ShowRenameModal {
		ui.Modal(c, &a.ShowRenameModal, func() {
			ui.Column(c).Width(360).Radius(10).Background(t.Surface).BorderWidth(1).BorderColor(t.Border).Padding(16, 18).Gap(10).Children(func() {
				ui.Text(c, "Rename Task").Bold().FontSize(14)
				in := ui.TextInput(c, &a.RenameSessionTitle).Grow(1).AutoFocus()
				if in.Submitted() && a.RenameSessionTitle != "" {
					if sess := a.Agent.ActiveSession(); sess != nil {
						sess.Title = a.RenameSessionTitle
						a.Agent.SaveSessionToDisk(sess)
					}
					a.ShowRenameModal = false
				}
				ui.Row(c).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Grow(1)
					if ui.Button(c, "Cancel").Clicked() {
						a.ShowRenameModal = false
					}
					if ui.PrimaryButton(c, "Save").Clicked() && a.RenameSessionTitle != "" {
						if sess := a.Agent.ActiveSession(); sess != nil {
							sess.Title = a.RenameSessionTitle
							a.Agent.SaveSessionToDisk(sess)
						}
						a.ShowRenameModal = false
					}
				})
			})
		})
	}
}
