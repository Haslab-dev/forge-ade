package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/icons"
)

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
			if IconButton(c, tc, icons.TablerPanelLeft, 15).Clicked() {
				a.ShowLeftSidebar = false
			}
			ui.Row(c).Padding(0, 6).Gap(4).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "ForgeADE").FontSize(13).FontWeight(600).LetterSpacing(-0.2)
			})
			ui.Box(c).Grow(1)
		})

		// Primary Action Buttons: + New Task (⌘N), Search (⌘P)
		ui.Column(c).Padding(6, 10).Gap(2).Children(func() {
			newTask := SidebarActionButton(c, tc, icons.TablerPlus, "New Task", "⌘N", false)
			if newTask.Clicked() {
				a.ShowNewTaskModal = true
				a.NewTaskTitle = ""
			}
			search := SidebarActionButton(c, tc, icons.TablerSearch, "Search", "⌘P", false)
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
				})
				if active {
					btn.Background(tc.Text.Alpha(0.15))
				} else if btn.Hovered() {
					btn.Background(tc.SidebarRowHover)
				}
				if btn.Clicked() {
					onPick()
				}
			}
			pill("Tasks", icons.TablerChecklist, a.AgentSidebarView == AgentSidebarTasks, func() { a.AgentSidebarView = AgentSidebarTasks })
			pill("Files", icons.TablerFiles, a.AgentSidebarView == AgentSidebarFiles, func() { a.AgentSidebarView = AgentSidebarFiles })
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
				if IconButton(c, tc, icons.TablerPlus, 12).Clicked() {
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
				if IconButton(c, tc, icons.TablerPlus, 12).Clicked() {
					a.ShowNewTaskModal = true
				}
			})
			botRow := SidebarActionButton(c, tc, icons.TablerBot, "Add a bot or schedule", "", false)
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
				dockBtn(icons.TablerHelp, false, func() { mygo.Shell.OpenExternal("https://github.com/hasdev/forge-ade") })
				dockBtn(icons.TablerChartBar, false, func() {
					a.ShowRightDrawer = true
					a.RightDrawerTab = RightTabTasks
				})
				dockBtn(icons.TablerLayoutKanban, a.RightDrawerTab == RightTabBoard && a.ShowRightDrawer, func() {
					a.ShowRightDrawer = true
					a.RightDrawerTab = RightTabBoard
				})
				dockBtn(icons.TablerGitBranch, a.RightDrawerTab == RightTabGit && a.ShowRightDrawer, func() {
					a.ShowRightDrawer = true
					a.RightDrawerTab = RightTabGit
				})
				dockBtn(icons.TablerBook, false, func() {
					a.ShowNewFileModal = true
					a.NewFileName = "AGENTS.md"
				})
				dockBtn(icons.TablerSettings, false, func() { a.Settings.Toggle() })
				ui.Box(c).Grow(1)
				if a.ThemeMgr.Appearance == "dark" {
					dockBtn(icons.TablerMoon, false, a.toggleAppearance)
				} else {
					dockBtn(icons.TablerSun, false, a.toggleAppearance)
				}
			})
		_ = t
	})
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
		treeBtn(icons.TablerFilePlus, func() {
			a.ShowNewFileModal = true
			a.NewFileName = ""
		})
		treeBtn(icons.TablerFolderPlus, func() {
			a.ShowNewFolderModal = true
			a.NewFolderName = ""
		})
		treeBtn(icons.TablerRefresh, func() {
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
					ui.Icon(c, icons.FileIconSVG(f.RelativePath)).Width(14).Height(14).TextColor(tc.IconMuted)
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
