package ui

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/internal/git"
)

// Agent sidebar view modes
const (
	AgentSidebarTasks = 0
	AgentSidebarFiles = 1
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
	AgentSidebarView    int    // AgentSidebarTasks (0) or AgentSidebarFiles (1)
	ShowBottomTerminal  bool    // Collapsible bottom terminal drawer
	BottomTerminalHeight float32 // Resizable terminal height
	ShowRightDrawer     bool    // Collapsible right side pane
	RightDrawerWidth    float32 // Resizable right pane width
	LeftSidebarWidth    float32 // Resizable left sidebar width
	RightDrawerTab      int     // RightTabChanges, RightTabTerminal, RightTabChat, RightTabTasks, RightTabGit, RightTabDatabase
	EditorActivity      int     // ActivityExplorer, ActivityGit, ActivityDatabase
	SelectedFile        string
	SelectedDiffPath    string
	SideChatMessages    []*ChatMessage
	SideChatInput       string
	SideChatModel       string
	SideChatMode        string // "full", "ask", "plan"
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
		Engine:               engine,
		Workspace:            NewWorkspaceState(rootDir),
		Editor:               NewEditorManager(),
		Terminal:             NewTerminalState(rootDir),
		ThemeMgr:             NewThemeManager(),
		Mode:                 ModeAgent,
		ActiveTaskKind:       "agent",
		ShowLeftSidebar:      true,
		LeftSidebarWidth:     260,
		RightDrawerWidth:     400,
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
	if c.Shortcut(ui.Cmd, ui.KeySlash) {
		a.ThemeMgr.Toggle()
	}
	if c.Shortcut(ui.Cmd, ui.KeyBackquote) {
		a.ShowBottomTerminal = !a.ShowBottomTerminal
	}

	bar := c.TitleBar()

	// Top window drag handle across the title bar region
	ui.Box(c).Absolute().Top(0).Left(0).Right(0).Height(max(bar.Height, 38)).DragWindow()

	// Root layout: Full-Height Left Sidebar (Agent Mode) + Main Column with interactive Split divider
	if a.Mode == ModeAgent && a.ShowLeftSidebar {
		ui.Split(c, &a.LeftSidebarWidth, func() {
			a.renderAgentSidebar(c)
		}, func() {
			a.renderMainColumn(c)
		}).SplitReset(260).Fill()
	} else {
		a.renderMainColumn(c)
	}

	// Overlays & Modals
	a.renderModals(c)
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

	ui.Row(c).Height(52).Gap(10).Padding(0, 14, 0, leftPad).AlignItems(ui.Center).Background(t.Surface).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
		// Left: Sidebar Toggle with Tabler Icon (only shown when full sidebar is collapsed)
		if showToggleInHeader {
			sidebarBtn := ui.Button(c, "")
			sidebarBtn.Children(func() {
				TablerIcon(c, TablerPanelLeft, 16)
			})
			if sidebarBtn.Clicked() {
				a.ShowLeftSidebar = !a.ShowLeftSidebar
			}
		}

		// Title
		ui.Text(c, title).Bold().FontSize(13).MaxWidth(280)

		// Project folder chip & Git Branch chip (only shown on editor surface, matches ForgeADE)
		if a.Mode == ModeEditor {
			projectChip := ui.Button(c, "")
			projectChip.Children(func() {
				ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
					TablerIcon(c, TablerFolder, 14)
					ui.Text(c, projectName).FontSize(11).Bold()
				})
			})
			if projectChip.Clicked() {
				a.Palette.Toggle()
			}

			if a.Git != nil && a.Git.Branch != "" {
				dirtyMarker := ""
				if len(a.Git.Staged)+len(a.Git.Unstaged)+len(a.Git.Untracked) > 0 {
					dirtyMarker = " *"
				}
				branchChip := ui.Button(c, "")
				branchChip.Children(func() {
					ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
						TablerIcon(c, TablerGitBranch, 13)
						ui.Text(c, fmt.Sprintf("%s%s", a.Git.Branch, dirtyMarker)).FontSize(11)
					})
				})
				if branchChip.Clicked() {
					a.ShowRightDrawer = true
					a.RightDrawerTab = RightTabGit
				}
			}
		}

		// Task "…" menu with TablerDots (shown in Agent mode with active session)
		if a.Mode == ModeAgent && a.Agent != nil && a.Agent.ActiveSession() != nil {
			moreBtn := ui.Button(c, "")
			moreBtn.Children(func() {
				TablerIcon(c, TablerDots, 15)
			})
			if moreBtn.Clicked() {
				a.ShowTaskMenuModal = true
			}
		}

		ui.Box(c).Grow(1)

		// Segmented Mode Switcher: [✨ Agent] | [💻 Editor] with smooth pill styling
		ui.Row(c).Gap(2).Padding(2).Radius(8).Background(t.Border).AlignItems(ui.Center).Children(func() {
			var agentBtn ui.Element
			if a.Mode == ModeAgent {
				agentBtn = ui.PrimaryButton(c, "")
			} else {
				agentBtn = ui.Button(c, "")
			}
			agentBtn.Children(func() {
				ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
					TablerIcon(c, TablerSparkles, 14)
					ui.Text(c, "Agent").FontSize(11).Bold()
				})
			})
			if agentBtn.Clicked() {
				a.Mode = ModeAgent
			}

			var editorBtn ui.Element
			if a.Mode == ModeEditor {
				editorBtn = ui.PrimaryButton(c, "")
			} else {
				editorBtn = ui.Button(c, "")
			}
			editorBtn.Children(func() {
				ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
					TablerIcon(c, TablerCode, 14)
					ui.Text(c, "Editor").FontSize(11).Bold()
				})
			})
			if editorBtn.Clicked() {
				a.Mode = ModeEditor
			}
		})

		ui.Box(c).Grow(1)

		// Workspace & panel controls: Reveal in Finder, Terminal Toggle, Side Pane Toggle, Theme Picker
		finderBtn := ui.Button(c, "")
		finderBtn.Children(func() {
			TablerIcon(c, TablerFolder, 15)
		})
		if finderBtn.Clicked() {
			_ = exec.Command("open", a.Workspace.RootDir).Start()
		}

		var termToggleBtn ui.Element
		if a.ShowBottomTerminal {
			termToggleBtn = ui.PrimaryButton(c, "")
		} else {
			termToggleBtn = ui.Button(c, "")
		}
		termToggleBtn.Children(func() {
			TablerIcon(c, TablerTerminal, 15)
		})
		if termToggleBtn.Clicked() {
			a.ShowBottomTerminal = !a.ShowBottomTerminal
		}

		var sideDrawerBtn ui.Element
		if a.ShowRightDrawer {
			sideDrawerBtn = ui.PrimaryButton(c, "")
		} else {
			sideDrawerBtn = ui.Button(c, "")
		}
		sideDrawerBtn.Children(func() {
			TablerIcon(c, TablerPanelRight, 15)
		})
		if sideDrawerBtn.Clicked() {
			a.ShowRightDrawer = !a.ShowRightDrawer
		}

		// Theme & Palette Picker button
		themeBtn := ui.Button(c, "")
		themeBtn.Children(func() {
			TablerIcon(c, TablerPalette, 15)
		})
		if themeBtn.Clicked() {
			a.ThemeMgr.Toggle()
		}
	})
}

// renderAgentSurface renders the center chat viewport, collapsible terminal & right drawer.
func (a *AppUI) renderAgentSurface(c *ui.Context) {
	// Center Column: Main Chat Container (or CLI surface) + Collapsible Bottom Terminal
	ui.Column(c).Grow(1).FillHeight().Children(func() {
		// Agent Main Container
		ui.Column(c).Grow(1).FillHeight().Children(func() {
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
		LeftResizeHandle(c, &a.RightDrawerWidth, 320, 750, 400)
		a.renderRightDrawer(c)
	}
}

// renderAgentSidebar renders the full-height task sessions sidebar with filter tabs and profile row.
func (a *AppUI) renderAgentSidebar(c *ui.Context) {
	t := c.Theme()
	bar := c.TitleBar()
	topPadLeft := float32(14)
	if bar.Left > 0 {
		topPadLeft = max(bar.Left, 76)
	}

	ui.Column(c).Grow(1).FillHeight().Background(t.Surface).BorderWidth(0, 1, 0, 0).BorderColor(t.Border).Children(func() {
		// Top window controls strip matching ForgeADE's full-height sidebar
		ui.Row(c).Padding(8, 12, 8, topPadLeft).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			collapseBtn := ui.Button(c, "")
			collapseBtn.Children(func() {
				TablerIcon(c, TablerPanelLeft, 14)
			})
			if collapseBtn.Clicked() {
				a.ShowLeftSidebar = false
			}
			ui.Text(c, "ForgeADE").Bold().FontSize(13)
			ui.Text(c, "Agent").FontSize(10).TextColor(t.TextMuted)
			ui.Box(c).Grow(1)
		})

		// Primary Action Buttons: + New Task (⌘N), Search (⌘P)
		ui.Column(c).Padding(8, 12).Gap(6).Children(func() {
			newTaskBtn := ui.PrimaryButton(c, "")
			newTaskBtn.Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					TablerIcon(c, TablerPlus, 14)
					ui.Text(c, "New Task (⌘N)").FontSize(11).Bold()
				})
			})
			if newTaskBtn.Clicked() {
				a.ShowNewTaskModal = true
				a.NewTaskTitle = ""
			}

			searchBtn := ui.Button(c, "")
			searchBtn.Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					TablerIcon(c, TablerSearch, 14)
					ui.Text(c, "Search (⌘P)").FontSize(11)
				})
			})
			if searchBtn.Clicked() {
				a.Palette.Toggle()
			}
		})

		// Sidebar Mode Switcher: [Tasks] vs [Files] (matches ForgeADE)
		ui.Row(c).Gap(4).Padding(2, 12).Children(func() {
			var tasksTab ui.Element
			if a.AgentSidebarView == AgentSidebarTasks {
				tasksTab = ui.PrimaryButton(c, "")
			} else {
				tasksTab = ui.Button(c, "")
			}
			tasksTab.Children(func() {
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					TablerIcon(c, TablerChecklist, 12)
					ui.Text(c, "Tasks").FontSize(10).Bold()
				})
			})
			if tasksTab.Clicked() {
				a.AgentSidebarView = AgentSidebarTasks
			}

			var filesTab ui.Element
			if a.AgentSidebarView == AgentSidebarFiles {
				filesTab = ui.PrimaryButton(c, "")
			} else {
				filesTab = ui.Button(c, "")
			}
			filesTab.Children(func() {
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					TablerIcon(c, TablerFiles, 12)
					ui.Text(c, "Files").FontSize(10).Bold()
				})
			})
			if filesTab.Clicked() {
				a.AgentSidebarView = AgentSidebarFiles
			}
		})

		if a.AgentSidebarView == AgentSidebarFiles {
			// Embedded Project File Explorer in Agent Sidebar
			ui.Column(c).Grow(1).Children(func() {
				a.renderFileTree(c)
			})
		} else {
			// Project & Sort Header
			ui.Row(c).Padding(4, 14).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "TASKS").FontSize(10).Bold().TextColor(t.TextMuted)
				ui.Box(c).Grow(1)
				plusBtn := ui.Button(c, "")
				plusBtn.Children(func() {
					TablerIcon(c, TablerPlus, 12)
				})
				if plusBtn.Clicked() {
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

						card := ui.Row(c.Key("sess-"+s.ID)).Padding(8, 10).Radius(6).Gap(8).AlignItems(ui.Center)
						if isActive {
							card.Background(t.Border)
						}
						card.Children(func() {
							if s.Status == "running" {
								TablerIcon(c, TablerLoader, 13)
							} else if s.Status == "completed" {
								TablerIcon(c, TablerCircleCheck, 13)
							} else {
								TablerIcon(c, TablerCircle, 13)
							}

							ui.Column(c).Grow(1).Children(func() {
								ui.Text(c, s.Title).Bold().FontSize(12).MaxWidth(150)
								ui.Row(c).Gap(6).Children(func() {
									ui.Text(c, s.Model).FontSize(10).TextColor(t.TextMuted)
								})
							})

							// Pin button with Tabler icon
							pinBtn := ui.Button(c, "")
							pinBtn.Children(func() {
								if s.Pinned {
									TablerIcon(c, TablerPinFilled, 13)
								} else {
									TablerIcon(c, TablerPin, 13)
								}
							})
							if pinBtn.Clicked() {
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
								TablerIcon(c, TablerTerminal, 14)
								ui.Column(c).Grow(1).Children(func() {
									ui.Text(c, cs.Title).Bold().FontSize(12).MaxWidth(150)
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
		}

		// Bots / Automations Section (matches ForgeADE)
		ui.Column(c).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Padding(6, 12).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "BOTS").FontSize(10).Bold().TextColor(t.TextMuted)
				ui.Box(c).Grow(1)
				addBotBtn := ui.Button(c, "")
				addBotBtn.Children(func() { TablerIcon(c, TablerPlus, 12) })
				if addBotBtn.Clicked() {
					a.ShowNewTaskModal = true
				}
			})
			botRowBtn := ui.Button(c, "")
			botRowBtn.Children(func() {
				ui.Row(c).Gap(6).Padding(4, 0).AlignItems(ui.Center).Children(func() {
					TablerIcon(c, TablerBot, 13)
					ui.Text(c, "Add a bot or schedule").FontSize(11).TextColor(t.TextMuted)
				})
			})
			if botRowBtn.Clicked() {
				a.ShowNewTaskModal = true
			}
		})

		// Bottom Utility Dock (matches ForgeADE AgentSidebar bottom utility dock)
		ui.Row(c).Padding(6, 10).AlignItems(ui.Center).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Background(t.Surface).Children(func() {
			// Documentation (TablerHelp)
			helpBtn := ui.Button(c, "")
			helpBtn.Children(func() { TablerIcon(c, TablerHelp, 14) })
			if helpBtn.Clicked() {
				_ = exec.Command("open", "https://github.com/hasdev/forge-ade").Start()
			}

			// Usage & Quotas (TablerChartBar)
			usageBtn := ui.Button(c, "")
			usageBtn.Children(func() { TablerIcon(c, TablerChartBar, 14) })
			if usageBtn.Clicked() {
				a.ShowRightDrawer = true
				a.RightDrawerTab = RightTabTasks
			}

			// Workflow Board (TablerLayoutKanban)
			kanbanBtn := ui.Button(c, "")
			kanbanBtn.Children(func() { TablerIcon(c, TablerLayoutKanban, 14) })
			if kanbanBtn.Clicked() {
				a.ShowRightDrawer = true
				a.RightDrawerTab = RightTabTasks
			}

			// Git Client (TablerGitBranch)
			gitBtn := ui.Button(c, "")
			gitBtn.Children(func() { TablerIcon(c, TablerGitBranch, 14) })
			if gitBtn.Clicked() {
				a.ShowRightDrawer = true
				a.RightDrawerTab = RightTabGit
			}

			// Project Rules (TablerBook)
			rulesBtn := ui.Button(c, "")
			rulesBtn.Children(func() { TablerIcon(c, TablerBook, 14) })
			if rulesBtn.Clicked() {
				a.ShowNewFileModal = true
				a.NewFileName = "AGENTS.md"
			}

			// Settings (TablerSettings)
			settingsBtn := ui.Button(c, "")
			settingsBtn.Children(func() { TablerIcon(c, TablerSettings, 14) })
			if settingsBtn.Clicked() {
				a.Settings.Toggle()
			}

			ui.Box(c).Grow(1)

			// Theme Toggle (TablerMoon / TablerSun)
			themeBtn := ui.Button(c, "")
			themeBtn.Children(func() {
				if a.ThemeMgr.Appearance == "dark" {
					TablerIcon(c, TablerMoon, 14)
				} else {
					TablerIcon(c, TablerSun, 14)
				}
			})
			if themeBtn.Clicked() {
				if a.ThemeMgr.Appearance == "dark" {
					a.ThemeMgr.Appearance = "light"
				} else {
					a.ThemeMgr.Appearance = "dark"
				}
			}
		})
	})
}

// renderEditorSurface renders the Editor mode: ActivityBar + FileTree, EditorView (tabs + buffer/diff), Bottom Terminal, Right Drawer.
func (a *AppUI) renderEditorSurface(c *ui.Context) {
	t := c.Theme()

	// 1. Left Activity Bar + Sidebar
	if a.ShowLeftSidebar {
		// Activity Bar (Explorer, Git, DB, Settings) with Tabler Icons
		ui.Column(c).Width(48).Background(t.Surface).BorderWidth(0, 1, 0, 0).BorderColor(t.Border).Padding(8, 0).AlignItems(ui.Center).Gap(8).Children(func() {
			var expBtn ui.Element
			if a.EditorActivity == ActivityExplorer {
				expBtn = ui.PrimaryButton(c, "")
			} else {
				expBtn = ui.Button(c, "")
			}
			expBtn.Children(func() { TablerIcon(c, TablerFiles, 16) })
			if expBtn.Clicked() {
				a.EditorActivity = ActivityExplorer
			}

			var gitBtn ui.Element
			if a.EditorActivity == ActivityGit {
				gitBtn = ui.PrimaryButton(c, "")
			} else {
				gitBtn = ui.Button(c, "")
			}
			gitBtn.Children(func() { TablerIcon(c, TablerGitBranch, 16) })
			if gitBtn.Clicked() {
				a.EditorActivity = ActivityGit
			}

			var dbBtn ui.Element
			if a.EditorActivity == ActivityDatabase {
				dbBtn = ui.PrimaryButton(c, "")
			} else {
				dbBtn = ui.Button(c, "")
			}
			dbBtn.Children(func() { TablerIcon(c, TablerDatabase, 16) })
			if dbBtn.Clicked() {
				a.EditorActivity = ActivityDatabase
			}

			ui.Box(c).Grow(1)

			settingsBtn := ui.Button(c, "")
			settingsBtn.Children(func() { TablerIcon(c, TablerSettings, 16) })
			if settingsBtn.Clicked() {
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

		// Collapsible Bottom Terminal with interactive drag resize handle
		if a.ShowBottomTerminal {
			TopResizeHandle(c, &a.BottomTerminalHeight, 120, 600, 240)
			a.renderBottomTerminal(c)
		}
	})

	// 3. Collapsible Right Drawer with interactive drag resize handle
	if a.ShowRightDrawer {
		LeftResizeHandle(c, &a.RightDrawerWidth, 320, 750, 400)
		a.renderRightDrawer(c)
	}
}

// renderFileTree renders the workspace file explorer tree.
func (a *AppUI) renderFileTree(c *ui.Context) {
	t := c.Theme()

	ui.Row(c).Padding(10, 12).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
		ui.Text(c, "FILES").FontSize(10).Bold().TextColor(t.TextMuted)
		ui.Box(c).Grow(1)
		newFileBtn := ui.Button(c, "")
		newFileBtn.Children(func() {
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				TablerIcon(c, TablerFilePlus, 13)
				ui.Text(c, "File").FontSize(11)
			})
		})
		if newFileBtn.Clicked() {
			a.ShowNewFileModal = true
			a.NewFileName = ""
		}
		newFolderBtn := ui.Button(c, "")
		newFolderBtn.Children(func() {
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				TablerIcon(c, TablerFolderPlus, 13)
				ui.Text(c, "Folder").FontSize(11)
			})
		})
		if newFolderBtn.Clicked() {
			a.ShowNewFolderModal = true
			a.NewFolderName = ""
		}
		refreshBtn := ui.Button(c, "")
		refreshBtn.Children(func() {
			TablerIcon(c, TablerRefresh, 13)
		})
		if refreshBtn.Clicked() {
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

	// File List with Tabler Vector SVG Icons
	filtered := a.Workspace.FilteredFiles()
	ui.Sidebar(c, &a.SelectedFile, func() {
		for _, f := range filtered {
			label := f.RelativePath
			doc := a.Editor.GetDoc(f.FullPath)
			if doc != nil && doc.Editor.Buffer.Modified() {
				label += " •"
			}
			item := ui.SidebarItem(c, f.RelativePath, FileIconSVG(f.RelativePath), label)
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

	ui.Column(c).Height(a.BottomTerminalHeight).Background(t.Surface).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
		// Terminal Drawer Header with Tabler icons
		ui.Row(c).Padding(6, 12).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				TablerIcon(c, TablerTerminal, 14)
				ui.Text(c, "Terminal (bash)").Bold().FontSize(12)
			})
			ui.Box(c).Grow(1)
			closeBtn := ui.Button(c, "")
			closeBtn.Children(func() {
				TablerIcon(c, TablerX, 13)
			})
			if closeBtn.Clicked() {
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

	ui.Column(c).Width(a.RightDrawerWidth).Background(t.Surface).BorderWidth(0, 0, 0, 1).BorderColor(t.Border).Children(func() {
		// Drawer Header Tabs with Tabler Icons (matching ForgeADE AgentRightSidebar)
		ui.Row(c).Gap(3).Padding(6, 8).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			tabs := []struct {
				Tab  int
				Name string
				Icon *ui.SVG
			}{
				{RightTabChanges, "Changes", TablerFileDiff},
				{RightTabTerminal, "Terminal", TablerTerminal},
				{RightTabChat, "Chat", TablerMessage},
				{RightTabTasks, "Tasks", TablerChecklist},
				{RightTabGit, "Git", TablerGitBranch},
				{RightTabDatabase, "DB", TablerDatabase},
			}

			for _, tab := range tabs {
				tTab := tab
				var btn ui.Element
				if a.RightDrawerTab == tTab.Tab {
					btn = ui.PrimaryButton(c, "")
				} else {
					btn = ui.Button(c, "")
				}
				btn.Children(func() {
					ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
						TablerIcon(c, tTab.Icon, 12)
						ui.Text(c, tTab.Name).FontSize(10).Bold()
					})
				})
				if btn.Clicked() {
					a.RightDrawerTab = tTab.Tab
				}
			}

			ui.Box(c).Grow(1)

			closeBtn := ui.Button(c, "")
			closeBtn.Children(func() {
				TablerIcon(c, TablerX, 13)
			})
			if closeBtn.Clicked() {
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

// renderChangesPane renders the git diff changes pane with interactive inline diff view.
func (a *AppUI) renderChangesPane(c *ui.Context) {
	t := c.Theme()

	var files []git.FileStatus
	if a.Git != nil {
		files = append(files, a.Git.Staged...)
		files = append(files, a.Git.Unstaged...)
		files = append(files, a.Git.Untracked...)
	}

	// Subheader
	ui.Row(c).Padding(6, 12).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Background(t.Surface).Children(func() {
		ui.Text(c, "Changes").Bold().FontSize(12)
		refreshBtn := ui.Button(c, "")
		refreshBtn.Children(func() {
			TablerIcon(c, TablerRefresh, 12)
		})
		if refreshBtn.Clicked() && a.Git != nil {
			a.Git.Refresh()
		}

		ui.Box(c).Grow(1)
		ui.Text(c, fmt.Sprintf("%d files", len(files))).FontSize(11).TextColor(t.TextMuted)
	})

	if len(files) == 0 {
		ui.Column(c).Grow(1).Center().Gap(8).Padding(20).Children(func() {
			TablerIcon(c, TablerFileDiff, 32)
			ui.Text(c, "No changes yet").Bold().FontSize(13).TextColor(t.TextMuted)
			ui.Text(c, "Files modified in the workspace show up here with their diffs.").FontSize(11).TextColor(t.TextMuted).MaxWidth(240)
			refreshBtn := ui.Button(c, "Refresh changes").FontSize(11)
			if refreshBtn.Clicked() && a.Git != nil {
				a.Git.Refresh()
			}
		})
		return
	}

	ui.Column(c).Grow(1).FillHeight().Children(func() {
		// File List
		ui.List(c, nil, len(files), func(i int) {
			if i >= len(files) {
				return
			}
			f := files[i]
			isSel := (a.SelectedDiffPath == f.Path) || (a.SelectedDiffPath == "" && i == 0)
			if a.SelectedDiffPath == "" && i == 0 {
				a.SelectedDiffPath = f.Path
			}

			fileRow := ui.Button(c, "")
			fileRow.Children(func() {
				ui.Row(c).Gap(6).Padding(4, 8).AlignItems(ui.Center).Children(func() {
					TablerIcon(c, FileIconSVG(f.Path), 13)
					ui.Text(c, filepath.Base(f.Path)).FontSize(11).Grow(1).Bold()
					badgeText := string(f.Status)
					if f.Staging == "staged" {
						badgeText = "STG"
					}
					ui.Text(c, badgeText).FontSize(9).TextColor(t.Accent).Padding(1, 4).Radius(4).Background(t.Border)
				})
			})
			if isSel {
				fileRow.Background(t.SurfaceHover)
			}
			if fileRow.Clicked() {
				a.SelectedDiffPath = f.Path
			}
		}).Height(160).BorderWidth(0, 0, 1, 0).BorderColor(t.Border)

		// Inline Diff Viewer for SelectedDiffPath
		ui.Column(c).Grow(1).FillHeight().Children(func() {
			diffTitle := a.SelectedDiffPath
			if diffTitle == "" && len(files) > 0 {
				diffTitle = files[0].Path
			}
			ui.Row(c).Padding(4, 10).AlignItems(ui.Center).Background(t.Surface).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
				ui.Text(c, filepath.Base(diffTitle)).FontSize(11).Bold()
				ui.Box(c).Grow(1)
				openEditorBtn := ui.Button(c, "Open in Editor").FontSize(10)
				if openEditorBtn.Clicked() {
					fullPath := filepath.Join(a.Workspace.RootDir, diffTitle)
					_, _ = a.Editor.OpenFile(fullPath, diffTitle)
					a.Mode = ModeEditor
				}
			})

			// Render diff lines
			diffContent := ""
			if a.Engine != nil && diffTitle != "" {
				diffContent = a.Engine.GetGitDiff(a.Workspace.RootDir, diffTitle)
			}
			if diffContent == "" {
				ui.Column(c).Grow(1).Center().Children(func() {
					ui.Text(c, "No diff available for this file").FontSize(11).TextColor(t.TextMuted)
				})
			} else {
				lines := strings.Split(diffContent, "\n")
				ui.List(c, nil, len(lines), func(i int) {
					if i >= len(lines) {
						return
					}
					line := lines[i]
					row := ui.Row(c).Padding(1, 6)
					txt := ui.Text(c, line).FontSize(11)
					if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
						row.Background(ui.RGBA(34, 197, 94, 0.15))
						txt.TextColor(ui.Hex("#22c55e"))
					} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
						row.Background(ui.RGBA(239, 68, 68, 0.15))
						txt.TextColor(ui.Hex("#ef4444"))
					} else if strings.HasPrefix(line, "@@") {
						txt.TextColor(ui.Hex("#3b82f6"))
					} else {
						txt.TextColor(t.TextMuted)
					}
				}).Grow(1).Padding(4)
			}
		})
	})
}

// renderSideChatPane renders an independent side conversation thread.
func (a *AppUI) renderSideChatPane(c *ui.Context) {
	t := c.Theme()

	// Side Chat Header
	ui.Row(c).Padding(6, 12).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Background(t.Surface).Children(func() {
		ui.Text(c, "Side Chat").Bold().FontSize(12)
		ui.Box(c).Grow(1)
		ui.Text(c, a.SideChatModel).FontSize(10).TextColor(t.TextMuted)
	})

	// Message list
	if len(a.SideChatMessages) == 0 {
		ui.Column(c).Grow(1).Center().Gap(8).Padding(20).Children(func() {
			TablerIcon(c, TablerMessage, 28)
			ui.Text(c, "Side Conversation").Bold().FontSize(13).TextColor(t.TextMuted)
			ui.Text(c, "Run quick queries or ask fast code questions without interrupting your main task session.").FontSize(11).TextColor(t.TextMuted).MaxWidth(240)
		})
	} else {
		ui.List(c, nil, len(a.SideChatMessages), func(i int) {
			if i >= len(a.SideChatMessages) {
				return
			}
			m := a.SideChatMessages[i]
			if m.Role == "user" {
				ui.Row(c).Padding(3, 8).Children(func() {
					ui.Spacer(c)
					ui.Text(c, m.Text).Selectable().Padding(6, 10).Radius(8).Background(t.Surface).MaxWidth(280)
				})
			} else {
				ui.Column(c).Padding(3, 8).MaxWidth(340).Children(func() {
					ui.Text(c, m.Text).Selectable().Padding(6, 10).Radius(8).Background(t.Border)
				})
			}
		}).Grow(1).Padding(8).Justify(ui.End)
	}

	// Bottom input composer
	ui.Row(c).Padding(8, 10).Gap(6).AlignItems(ui.Center).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Background(t.Surface).Children(func() {
		ui.TextInput(c, &a.SideChatInput).Placeholder("Ask side chat...").Grow(1)
		sendBtn := ui.PrimaryButton(c, "")
		sendBtn.Children(func() {
			TablerIcon(c, TablerSend, 12)
		})
		if sendBtn.Clicked() && strings.TrimSpace(a.SideChatInput) != "" {
			text := strings.TrimSpace(a.SideChatInput)
			a.SideChatInput = ""
			a.SideChatMessages = append(a.SideChatMessages, &ChatMessage{
				Role: "user",
				Text: text,
			})
			reply := fmt.Sprintf("Side Chat: processed '%s'. Codebase context loaded.", text)
			a.SideChatMessages = append(a.SideChatMessages, &ChatMessage{
				Role: "assistant",
				Text: reply,
			})
			if a.Window != nil {
				a.Window.Invalidate()
			}
		}
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
		ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
			TablerIcon(c, TablerFolder, 13)
			ui.Text(c, projectName).FontSize(11).TextColor(t.TextMuted)
		})
		if curPath != "" {
			ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
				TablerIcon(c, FileIconSVG(curPath), 13)
				ui.Text(c, curPath).FontSize(11)
			})
		}
		ui.Box(c).Grow(1)

		ui.Text(c, lineCol).FontSize(11).TextColor(t.TextMuted)
		ui.Text(c, fmt.Sprintf("%d lines · %s", lineCount, langLabel)).FontSize(11).TextColor(t.TextMuted)
		ui.Text(c, "UTF-8").FontSize(11).TextColor(t.TextMuted)

		if a.Git != nil && a.Git.Branch != "" {
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				TablerIcon(c, TablerGitBranch, 12)
				ui.Text(c, a.Git.Branch).FontSize(11).TextColor(t.Accent)
			})
		}

		ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
			TablerIcon(c, TablerCpu, 12)
			ui.Text(c, a.Agent.ActiveModel).FontSize(11).TextColor(t.TextMuted)
		})
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
				// Modal Header with Tabler icons
				ui.Row(c).AlignItems(ui.Center).Children(func() {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						TablerIcon(c, TablerSparkles, 16)
						ui.Text(c, "New Task Session").Bold().FontSize(15)
					})
					ui.Box(c).Grow(1)
					closeBtn := ui.Button(c, "")
					closeBtn.Children(func() {
						TablerIcon(c, TablerX, 13)
					})
					if closeBtn.Clicked() {
						a.ShowNewTaskModal = false
					}
				})

				// Task Objective Input
				ui.Text(c, "TASK OBJECTIVE").FontSize(10).Bold().TextColor(t.TextMuted)
				ui.TextInput(c, &a.NewTaskTitle).Placeholder("Describe task objective (e.g. Implement auth flow or fix bug)...").Grow(1).AutoFocus()

				// Agent Protocol Selector: ACP Agents vs Agent CLIs with Tabler Icons
				ui.Row(c).Gap(6).Children(func() {
					var acpTab ui.Element
					if a.NewTaskAgentType == "acp" {
						acpTab = ui.PrimaryButton(c, "")
					} else {
						acpTab = ui.Button(c, "")
					}
					acpTab.Children(func() {
						ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
							TablerIcon(c, TablerSparkles, 13)
							ui.Text(c, "ACP Agents").FontSize(11).Bold()
						})
					})
					if acpTab.Clicked() {
						a.NewTaskAgentType = "acp"
					}

					var cliTab ui.Element
					if a.NewTaskAgentType == "cli" {
						cliTab = ui.PrimaryButton(c, "")
					} else {
						cliTab = ui.Button(c, "")
					}
					cliTab.Children(func() {
						ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
							TablerIcon(c, TablerTerminal, 13)
							ui.Text(c, "Interactive Agent CLIs").FontSize(11).Bold()
						})
					})
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
										ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
											TablerIcon(c, TablerCircleCheck, 12)
											ui.Text(c, "Ready").FontSize(10).TextColor(t.Accent)
										})
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
									ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
										TablerIcon(c, TablerCircleCheck, 12)
										ui.Text(c, "Installed").FontSize(10).TextColor(t.Accent)
									})
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

				renameBtn := ui.Button(c, "")
				renameBtn.Children(func() {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						TablerIcon(c, TablerPencil, 13)
						ui.Text(c, "Rename Task").FontSize(11)
					})
				})
				if renameBtn.Clicked() {
					a.ShowTaskMenuModal = false
					a.ShowRenameModal = true
					if sess := a.Agent.ActiveSession(); sess != nil {
						a.RenameSessionTitle = sess.Title
					}
				}

				forkBtn := ui.Button(c, "")
				forkBtn.Children(func() {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						TablerIcon(c, TablerGitFork, 13)
						ui.Text(c, "Fork Session").FontSize(11)
					})
				})
				if forkBtn.Clicked() {
					a.ShowTaskMenuModal = false
					if sess := a.Agent.ActiveSession(); sess != nil {
						a.Agent.NewSession(fmt.Sprintf("%s (fork)", sess.Title))
					}
				}

				termBtn := ui.Button(c, "")
				termBtn.Children(func() {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						TablerIcon(c, TablerTerminal, 13)
						ui.Text(c, "Toggle Terminal").FontSize(11)
					})
				})
				if termBtn.Clicked() {
					a.ShowTaskMenuModal = false
					a.ShowBottomTerminal = !a.ShowBottomTerminal
				}

				tasksBtn := ui.Button(c, "")
				tasksBtn.Children(func() {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						TablerIcon(c, TablerChecklist, 13)
						ui.Text(c, "Toggle Tasks Drawer").FontSize(11)
					})
				})
				if tasksBtn.Clicked() {
					a.ShowTaskMenuModal = false
					a.ShowRightDrawer = true
					a.RightDrawerTab = RightTabTasks
				}

				delBtn := ui.Button(c, "")
				delBtn.Children(func() {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						TablerIcon(c, TablerTrash, 13)
						ui.Text(c, "Delete Session").FontSize(11)
					})
				})
				if delBtn.Clicked() {
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
