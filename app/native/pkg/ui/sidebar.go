package ui

import (
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/agent"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/settings"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
	"github.com/hasdev/forge-ade/app/native/pkg/workspace"
)

// renderAgentSidebar renders the full-height task sessions sidebar with filter tabs and profile row.
func (a *AppUI) renderAgentSidebar(c *ui.Context) {
	t := c.Theme()
	tc := a.ThemeMgr.Colors()
	bar := c.TitleBar()
	topPadLeft := float32(14)
	if bar.Left > 0 {
		topPadLeft = max(bar.Left+8, 76)
	} else if runtime.GOOS == "darwin" {
		topPadLeft = 76
	}

	ui.Column(c).Grow(1).FillHeight().Background(tc.Sidebar).BorderWidth(0, 1, 0, 0).BorderColor(tc.SidebarBorder).Children(func() {
		// Top window controls strip (48px, macOS traffic-light inset) drags
		// the window, its collapse button stays clickable.
		ui.Row(c).Height(48).Padding(0, 12, 0, topPadLeft).AlignItems(ui.Center).DragWindow().Children(func() {
			if IconButton(c, tc, icons.TablerPanelLeft, 15).Clicked() {
				a.ShowLeftSidebar = false
			}
			ui.Row(c).Padding(0, 6).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, icons.ForgeSparkSVG).Width(15).Height(15).TextColor(tc.Accent)
				ui.Text(c, "ForgeADE").FontSize(13).FontWeight(600).LetterSpacing(-0.2)
			})
			ui.Box(c).Grow(1)
		})

		// forge-ade's leftSidebarView: the file explorer replaces the whole
		// tasks body ("Explore project files" swaps it in, "Back to tasks" out).
		if a.LeftSidebarView == "files" {
			a.renderSidebarFileExplorer(c, tc)
		} else {
			// Primary Action Buttons: + New task (⌘N), Search (⌘K)
			ui.Column(c).Padding(6, 10).Gap(2).Children(func() {
				isNewTaskActive := (a.Mode == ModeAgent && a.ActiveTaskKind == "agent" && (a.Agent == nil || a.Agent.ActiveSessionID == ""))
				newTask := SidebarActionButton(c, tc, icons.TablerPlus, "New task", "⌘N", isNewTaskActive)
				if newTask.Clicked() {
					a.OpenNewTaskModal()
				}
				search := SidebarActionButton(c, tc, icons.TablerSearch, "Search", "⌘K", false)
				if search.Clicked() {
					a.Palette.Toggle()
				}
			})

			activeWsPath := ""
			if a.Workspace != nil {
				activeWsPath = a.Workspace.RootDir
			}
			activeProjectName := resolveProjectName(activeWsPath)

			// Project selector dropdown + quick actions, as forge-ade's
			// "All projects ⌄" row: project filter + sort menu on the left,
			// open-folder / new-task / archive / collapse-others on the right.
			activeProject := resolveProjectName(activeWsPath)
			ui.Row(c).Padding(6, 14, 2, 14).AlignItems(ui.Center).Children(func() {
				filterLabel := "All projects"
				if a.ProjectFilter != "" && a.ProjectFilter != "all" {
					filterLabel = a.ProjectFilter
				}
				filterBtn := ui.Row(c).Gap(5).AlignItems(ui.Center).Cursor(ui.CursorPointer)
				filterBtn.Children(func() {
					ui.Text(c, filterLabel).FontSize(12).FontWeight(600).SingleLine()
					ui.Icon(c, icons.TablerChevronDown).Width(12).Height(12).TextColor(tc.SidebarMutedForeground)
				})
				filterBtn.Menu(func(m *ui.Menu) {
					if m.Item(menuChecked(a.ProjectFilter == "all", "All projects")).Chosen() {
						a.ProjectFilter = "all"
					}
					for _, name := range a.sidebarProjectNames(activeProject) {
						thisName := name
						if m.Item(menuChecked(a.ProjectFilter == thisName, thisName)).Chosen() {
							a.ProjectFilter = thisName
						}
					}
					m.Separator()
					if m.Item(menuChecked(a.TaskSortBy == "updated", "Updated (recent)")).Chosen() {
						a.TaskSortBy = "updated"
					}
					if m.Item(menuChecked(a.TaskSortBy == "created", "Created date")).Chosen() {
						a.TaskSortBy = "created"
					}
				})
				ui.Box(c).Grow(1)

				smallAction := func(key string, icon *ui.SVG, active bool, onPick func()) {
					btn := ui.Row(c.Key("hdract-"+key)).Size(20, 20).Radius(4).AlignItems(ui.Center).Justify(ui.Center)
					btn.Children(func() {
						ic := ui.Icon(c, icon).Width(14).Height(14)
						if active {
							ic.TextColor(tc.Text)
						} else {
							ic.TextColor(tc.SidebarMutedForeground)
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
				smallAction("open", icons.TablerArrowDownToLine, false, func() { a.openWorkspaceFolder() })
				smallAction("new", icons.TablerPlus, false, func() { a.NewAgentTask("") })
				smallAction("archive", icons.TablerArchive, a.Agent.ActiveFilterTab == "archived", func() {
					if a.Agent.ActiveFilterTab == "archived" {
						a.Agent.ActiveFilterTab = "active"
					} else {
						a.Agent.ActiveFilterTab = "archived"
					}
				})
				collapsedOthers := a.otherProjectsCollapsed(activeProject)
				if collapsedOthers {
					smallAction("expand", icons.TablerMaximize2, false, func() { a.expandAllProjects() })
				} else {
					smallAction("collapse", icons.TablerMinimize2, false, func() { a.collapseOtherProjects(activeProject) })
				}
			})

			// Search Input Filter
			ui.Row(c).Padding(2, 10, 4, 10).Children(func() {
				ui.TextInput(c, &a.Agent.SearchQuery).Placeholder("Search tasks...").Grow(1)
			})

			// Filter segmented tabs: Active, Pinned, Snoozed, Archived — the
			// selected tab is a foreground/15 pill; counts render as an
			// "(N)" suffix (Active never shows one), as forge-ade does.
			_, pinnedCount, snoozedCount, archivedCount := a.Agent.TabCounts()
			tabCounts := map[string]int{
				"active":   0,
				"pinned":   pinnedCount,
				"snoozed":  snoozedCount,
				"archived": archivedCount,
			}
			ui.Row(c).Gap(4).Padding(4, 10).AlignItems(ui.Center).Children(func() {
				capitalize := func(s string) string {
					if s == "" {
						return s
					}
					return strings.ToUpper(s[:1]) + s[1:]
				}
				for _, tab := range []string{"active", "pinned", "snoozed", "archived"} {
					thisTab := tab
					active := a.Agent.ActiveFilterTab == thisTab
					label := capitalize(thisTab)
					if n := tabCounts[thisTab]; n > 0 {
						label += fmt.Sprintf(" (%d)", n)
					}
					pill := ui.Row(c.Key("ftab-"+thisTab)).Radius(999).Padding(2, 8).AlignItems(ui.Center)
					pill.Children(func() {
						txt := ui.Text(c, label).FontSize(12)
						if active {
							txt.FontWeight(500)
						} else {
							txt.TextColor(tc.SidebarMutedForeground)
						}
					})
					if active {
						pill.Background(tc.Text.Alpha(0.15))
					} else if pill.Hovered() {
						pill.Background(tc.Text.Alpha(0.05))
					}
					if pill.Clicked() {
						a.Agent.ActiveFilterTab = thisTab
					}
				}
			})

			// Scrollable Task Sessions List grouped by Project/Folder
			type projectTaskGroup struct {
				ProjectName string
				FolderPath  string
				Sessions    []*agent.AgentSession
				CLISessions []*agent.AgentCLISession
			}

			groupMap := make(map[string]*projectTaskGroup)
			var groupOrder []string

			getOrCreateGroup := func(name, path string) *projectTaskGroup {
				if g, exists := groupMap[name]; exists {
					if g.FolderPath == "" && path != "" {
						g.FolderPath = path
					}
					return g
				}
				g := &projectTaskGroup{
					ProjectName: name,
					FolderPath:  path,
				}
				groupMap[name] = g
				groupOrder = append(groupOrder, name)
				return g
			}

			// Always seed active workspace first, then General
			getOrCreateGroup(activeProjectName, activeWsPath)
			if activeProjectName != "General" {
				getOrCreateGroup("General", "")
			}

			// 1. Group ACP Sessions
			sessions := a.Agent.FilteredSessions()
			for _, sess := range sessions {
				s := sess
				pName := resolveProjectName(s.WorkspacePath)
				if s.WorkspacePath == activeWsPath || (activeWsPath != "" && strings.HasPrefix(s.WorkspacePath, activeWsPath)) {
					pName = activeProjectName
				}
				g := getOrCreateGroup(pName, s.WorkspacePath)
				g.Sessions = append(g.Sessions, s)
			}

			// 2. Group CLI Sessions
			if a.CLIMgr != nil {
				for _, cliSess := range a.CLIMgr.Sessions {
					cs := cliSess
					if a.Agent.SearchQuery != "" {
						sq := strings.ToLower(a.Agent.SearchQuery)
						if !strings.Contains(strings.ToLower(cs.Title), sq) && !strings.Contains(strings.ToLower(cs.AgentName), sq) {
							continue
						}
					}
					if a.Agent.ActiveFilterTab == "archived" {
						continue
					}
					pName := resolveProjectName(cs.Cwd)
					if cs.Cwd == activeWsPath || (activeWsPath != "" && strings.HasPrefix(cs.Cwd, activeWsPath)) {
						pName = activeProjectName
					}
					g := getOrCreateGroup(pName, cs.Cwd)
					g.CLISessions = append(g.CLISessions, cs)
				}
			}

			ui.Scroll(c).Grow(1).Children(func() {
				ui.Column(c).Padding(2, 6).Gap(4).Children(func() {
					for _, name := range groupOrder {
						group := groupMap[name]
						// "All projects ⌄" filter: only the chosen project renders.
						if a.ProjectFilter != "all" && a.ProjectFilter != "" && group.ProjectName != a.ProjectFilter {
							continue
						}
						totalCount := len(group.Sessions) + len(group.CLISessions)

						// forge-ade renders no group at all when it has no
						// visible sessions (its groups derive from sessions).
						if totalCount == 0 {
							continue
						}

						// Sort within the group: updated (recent) or created date.
						sort.SliceStable(group.Sessions, func(i, j int) bool {
							if a.TaskSortBy == "created" {
								return group.Sessions[i].CreatedAt.After(group.Sessions[j].CreatedAt)
							}
							return group.Sessions[i].UpdatedAt.After(group.Sessions[j].UpdatedAt)
						})

						isCollapsed := a.CollapsedProjectGroups[group.ProjectName]

						// Project Group Header: chevron + folder + name, with the
						// hover actions (Explore project files / New task / Delete
						// workspace) that forge-ade shows on group hover.
						canExplore := group.ProjectName != "General" && group.FolderPath != ""
						pHeader := ui.Row(c.Key("pheader-"+group.ProjectName)).Height(28).Radius(6).
							Padding(0, 6).Gap(5).AlignItems(ui.Center)
						pHeader.Children(func() {
							if isCollapsed {
								ui.Icon(c, icons.TablerChevronRight).Width(12).Height(12).TextColor(tc.IconMuted)
								ui.Icon(c, icons.TablerFolder).Width(14).Height(14).TextColor(tc.IconMuted)
							} else {
								ui.Icon(c, icons.TablerChevronDown).Width(12).Height(12).TextColor(tc.IconMuted)
								ui.Icon(c, icons.TablerFolderOpen).Width(14).Height(14).TextColor(tc.Accent)
							}

							ui.Text(c, group.ProjectName).FontSize(12).FontWeight(600).Grow(1).SingleLine()

							// Hover-only actions, as forge-ade's opacity-0
							// group-hover actions.
							if pHeader.Hovered() {
								groupAction := func(key string, icon *ui.SVG, danger bool, onPick func()) {
									btn := ui.Row(c.Key("gact-"+key)).Size(20, 20).Radius(5).
										AlignItems(ui.Center).Justify(ui.Center)
									btn.Children(func() {
										ic := ui.Icon(c, icon).Width(13).Height(13)
										if danger {
											ic.TextColor(tc.Error)
										} else {
											ic.TextColor(tc.SidebarMutedForeground)
										}
									})
									if danger && btn.Hovered() {
										btn.Background(tc.Error.Alpha(0.12))
									} else if btn.Hovered() {
										btn.Background(tc.Text.Alpha(0.05))
									}
									if btn.Clicked() {
										onPick()
									}
								}

								if canExplore {
									groupAction("explore-"+group.ProjectName, icons.TablerFolderTree, false, func() {
										if group.FolderPath != "" && group.FolderPath != a.Workspace.RootDir {
											a.switchWorkspace(group.FolderPath)
										}
										a.Workspace.Refresh()
										a.LeftSidebarView = "files"
									})
								}
								groupAction("new-"+group.ProjectName, icons.TablerPlus, false, func() {
									if group.ProjectName == "General" || group.FolderPath == "" {
										a.OpenNewTaskModal()
									} else {
										a.OpenNewTaskModalFor(group.FolderPath)
									}
								})
								if canExplore {
									groupAction("delws-"+group.ProjectName, icons.TablerTrash, true, func() {
										a.deleteWorkspaceGroup(c, group.ProjectName, group.FolderPath)
									})
								}
							}
						})

						if pHeader.Hovered() {
							pHeader.Background(tc.SidebarRowHover)
						}
						if pHeader.Clicked() {
							a.CollapsedProjectGroups[group.ProjectName] = !isCollapsed
						}

						// Expanded Project Tasks — forge-ade renders empty groups
						// as just their header row, no placeholder text.
						if !isCollapsed && totalCount > 0 {
							ui.Column(c.Key("pbody-"+group.ProjectName)).Padding(0, 0, 4, 8).Gap(2).Children(func() {

								// 1. Agent ACP Sessions — progressive reveal with
								// "Show more", as forge-ade's groups do.
								visible := group.Sessions
								expandKey := "showmore-" + group.ProjectName
								if !a.ExpandedProjectGroups[expandKey] && len(visible) > 6 {
									visible = visible[:6]
								}
								for _, sess := range visible {
									s := sess
									isActive := (a.ActiveTaskKind == "agent" && s.ID == a.Agent.ActiveSessionID)
									added, removed := a.Agent.SessionDiffStats(s.ID)
									progress := float32(0)
									if s.Tasks != nil {
										if done, total := s.Tasks.Progress(); total > 0 {
											progress = float32(done) / float32(total)
										}
									}
									renderTaskRow(c, tc, taskRowData{
										Key:         "sess-" + s.ID,
										Title:       s.Title,
										Status:      s.Status,
										Active:      isActive,
										Pinned:      s.Pinned,
										Archived:    s.Archived,
										Snoozed:     s.Snoozed,
										AgentID:     s.AgentID,
										UpdatedTime: s.UpdatedAt,
										DiffAdded:   added,
										DiffRemoved: removed,
										Progress:    progress,
										OnPin:       func() { a.Agent.TogglePin(s.ID) },
										OnFork:      func() { a.Agent.ForkSessionByID(s.ID) },
										OnArchive:   func() { a.Agent.ToggleArchive(s.ID) },
										OnSnooze:    func() { a.Agent.ToggleSnooze(s.ID) },
										OnDelete:    func() { a.Agent.DeleteSession(s.ID) },
										OnOpen: func() {
											a.ActiveTaskKind = "agent"
											a.AgentViewMode = AgentViewChat
											a.Agent.SelectSession(s.ID)
										},
									})
								}
								if !a.ExpandedProjectGroups[expandKey] && len(group.Sessions) > 6 {
									showMore := ui.Row(c.Key("showmore-"+group.ProjectName)).Height(26).Radius(6).Padding(0, 8).Gap(6).AlignItems(ui.Center)
									showMore.Children(func() {
										ui.Text(c, fmt.Sprintf("Show more (%d)", len(group.Sessions)-6)).FontSize(11).TextColor(tc.Accent)
									})
									if showMore.Hovered() {
										showMore.Background(tc.SidebarRowHover)
									}
									if showMore.Clicked() {
										a.ExpandedProjectGroups[expandKey] = true
									}
								}

								// 2. Active CLI Sessions
								for _, cliSess := range group.CLISessions {
									cs := cliSess
									isActive := (a.ActiveTaskKind == "cli" && a.CLIMgr.ActiveID == cs.ID)
									isCLIRunning := cs.Status == "running" && cs.Terminal != nil && !cs.Terminal.IsExited()
									status := cs.Status
									if isCLIRunning {
										status = "running"
									} else if status == "running" {
										status = "completed"
									}
									renderTaskRow(c, tc, taskRowData{
										Key:         "cli-sess-" + cs.ID,
										Title:       cs.Title,
										Status:      status,
										Active:      isActive,
										AgentID:     "agent-agy",
										UpdatedTime: cs.StartedAt,
										OnStop:      func() { a.CLIMgr.StopSession(cs.ID) },
										OnRestart:   func() { a.CLIMgr.RestartSession(cs.ID) },
										OnDelete:    func() { a.CLIMgr.DeleteSession(cs.ID) },
										OnOpen: func() {
											a.ActiveTaskKind = "cli"
											a.AgentViewMode = AgentViewChat
											a.CLIMgr.ActiveID = cs.ID
										},
									})
								}
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
					a.OpenNewTaskModal()
				}
			})
			botRow := SidebarActionButton(c, tc, icons.TablerBot, "Add a bot or schedule", "", false)
			if botRow.Clicked() {
				// Automations manager, as forge-ade's AutomationsModal entry.
				a.Mode = ModeSettings
				if a.Settings != nil {
					a.Settings.IsOpen = true
					a.Settings.ActiveTab = settings.SettingsTabAutos
				}
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
				dockBtn(icons.TablerChartBar, a.ShowUsageModal, func() { a.openUsageModal() })
				dockBtn(icons.TablerLayoutKanban, a.Mode == ModeAgent && a.AgentViewMode == AgentViewBoard, func() {
					a.openBoardSurface()
				})
				dockBtn(icons.TablerGitBranch, a.Mode == ModeAgent && a.AgentViewMode == AgentViewGit, func() {
					a.openGitClientSurface()
					a.loadGitRemoteURL()
				})
				dockBtn(icons.TablerBook, false, func() { a.OpenRulesModal() })
				dockBtn(icons.TablerSettings, a.Mode == ModeSettings, func() {
					a.Mode = ModeSettings
					if a.Settings != nil {
						a.Settings.IsOpen = true
					}
				})
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

// renderSidebarFileExplorer draws forge-ade's SidebarFileExplorer: "← Back to
// tasks", the file search, the project header (reveal / collapse-all /
// refresh) and the workspace tree. Files open in the right sidebar's preview
// tabs, mirroring the original's side preview.
func (a *AppUI) renderSidebarFileExplorer(c *ui.Context, tc theme.ThemeColors) {
	// Back to tasks
	ui.Column(c).Padding(4, 8, 0, 8).Children(func() {
		back := ui.Row(c.Key("back-to-tasks")).Height(28).Radius(8).Padding(0, 8).Gap(8).AlignItems(ui.Center)
		back.Children(func() {
			ui.Icon(c, icons.TablerArrowLeft).Width(13).Height(13).TextColor(tc.Text.Alpha(0.85))
			ui.Text(c, "Back to tasks").FontSize(13).FontWeight(500).TextColor(tc.Text.Alpha(0.85))
		})
		if back.Hovered() {
			back.Background(tc.SidebarRowHover)
		}
		if back.Clicked() {
			a.LeftSidebarView = "tasks"
		}
	})

	// Search files
	ui.Row(c).Padding(8, 10, 6, 10).Children(func() {
		ui.TextInput(c, &a.Workspace.FilterQuery).Placeholder("Search files...").Grow(1)
	})

	// Project header: name + external link, collapse-all, refresh
	ui.Row(c).Padding(2, 14, 6, 14).AlignItems(ui.Center).Gap(4).Children(func() {
		projectName := resolveProjectName(a.Workspace.RootDir)
		nameBtn := ui.Row(c.Key("exp-project")).Gap(5).AlignItems(ui.Center)
		nameBtn.Children(func() {
			ui.Text(c, projectName).FontSize(13).FontWeight(500).TextColor(tc.SidebarMutedForeground).SingleLine()
			ui.Icon(c, icons.TablerExternalLink).Width(11).Height(11).TextColor(tc.SidebarMutedForeground)
		})
		if nameBtn.Clicked() {
			a.Palette.Toggle()
		}
		ui.Box(c).Grow(1)
		if IconButton(c, tc, icons.TablerChevronsDownUp, 12).Clicked() {
			for k := range a.ExpandedFileTreeFolders {
				a.ExpandedFileTreeFolders[k] = false
			}
		}
		if IconButton(c, tc, icons.TablerRefresh, 12).Clicked() {
			a.Workspace.Refresh()
			if a.Git != nil {
				go a.Git.Refresh()
			}
		}
	})

	// Tree — clicks open the side preview in the right sidebar.
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Padding(2, 8, 8, 8).Children(func() {
			a.renderExplorerTree(c, tc)
		})
	})
}

// resolveProjectName extracts a clean human-readable workspace project name.
func resolveProjectName(folderPath string) string {
	clean := strings.TrimSpace(folderPath)
	if clean == "" || clean == "general" || strings.HasSuffix(clean, "/.forge-ade/general") || strings.HasSuffix(clean, "/.forge-ade/general/") {
		return "General"
	}
	base := filepath.Base(filepath.Clean(clean))
	if base == "." || base == "/" || base == "" {
		return "General"
	}
	return base
}

// menuChecked prefixes a menu label with a check mark, matching forge-ade's
// role="menuitemradio" check indicator.
func menuChecked(checked bool, label string) string {
	if checked {
		return "✓ " + label
	}
	return label
}

// sidebarProjectNames lists the distinct project group names across all
// agent + CLI sessions, most recently updated first — the "All projects"
// filter menu entries.
func (a *AppUI) sidebarProjectNames(activeProject string) []string {
	seen := map[string]bool{activeProject: true, "General": true}
	type entry struct {
		name     string
		latestAt time.Time
	}
	var entries []entry
	touch := func(name string, at time.Time) {
		for i := range entries {
			if entries[i].name == name {
				if at.After(entries[i].latestAt) {
					entries[i].latestAt = at
				}
				return
			}
		}
		entries = append(entries, entry{name: name, latestAt: at})
	}
	if a.Agent != nil {
		for _, sess := range a.Agent.FilteredSessions() {
			name := resolveProjectName(sess.WorkspacePath)
			if sess.WorkspacePath == "" {
				name = "General"
			}
			at := sess.UpdatedAt
			if a.TaskSortBy == "created" {
				at = sess.CreatedAt
			}
			touch(name, at)
		}
	}
	if a.CLIMgr != nil {
		for _, cs := range a.CLIMgr.Sessions {
			touch(resolveProjectName(cs.Cwd), time.Now())
		}
	}
	delete(seen, activeProject) // active project is seeded by the caller order
	delete(seen, "General")
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].latestAt.After(entries[j].latestAt) })
	var names []string
	if activeProject != "" {
		names = append(names, activeProject)
	}
	names = append(names, "General")
	for _, e := range entries {
		if e.name == activeProject || e.name == "General" || seen[e.name] {
			continue
		}
		seen[e.name] = true
		names = append(names, e.name)
	}
	return names
}

// collapseOtherProjects collapses every sidebar project group except the
// active workspace's group, as forge-ade's Minimize2 toggle does.
func (a *AppUI) collapseOtherProjects(activeProject string) {
	for _, name := range a.sidebarProjectNames(activeProject) {
		if name != activeProject {
			a.CollapsedProjectGroups[name] = true
		}
	}
}

// expandAllProjects clears the per-project collapse state.
func (a *AppUI) expandAllProjects() {
	a.CollapsedProjectGroups = make(map[string]bool)
}

// otherProjectsCollapsed reports whether any non-active project group is
// currently collapsed, driving the Minimize2/Maximize2 toggle icon.
func (a *AppUI) otherProjectsCollapsed(activeProject string) bool {
	for _, name := range a.sidebarProjectNames(activeProject) {
		if name != activeProject && a.CollapsedProjectGroups[name] {
			return true
		}
	}
	return false
}

// FileTreeNode represents a node in the hierarchical file tree.
type FileTreeNode struct {
	Name         string
	RelativePath string
	FullPath     string
	IsDir        bool
	Children     []*FileTreeNode
}

// buildFileTree converts a flat list of workspace files into a hierarchical tree with sorted folders and files.
func buildFileTree(files []workspace.WorkspaceFile, rootDir string) []*FileTreeNode {
	rootMap := make(map[string]*FileTreeNode)
	var rootNodes []*FileTreeNode

	for _, f := range files {
		cleanRel := filepath.Clean(f.RelativePath)
		parts := strings.Split(cleanRel, string(filepath.Separator))
		if len(parts) == 0 {
			continue
		}

		if len(parts) == 1 {
			node := &FileTreeNode{
				Name:         parts[0],
				RelativePath: f.RelativePath,
				FullPath:     f.FullPath,
				IsDir:        f.IsDir,
			}
			rootMap[parts[0]] = node
			rootNodes = append(rootNodes, node)
			continue
		}

		// Navigate or create intermediate directories
		var curNode *FileTreeNode
		currentRel := parts[0]
		if existing, ok := rootMap[parts[0]]; ok {
			curNode = existing
		} else {
			curNode = &FileTreeNode{
				Name:         parts[0],
				RelativePath: currentRel,
				FullPath:     filepath.Join(rootDir, currentRel),
				IsDir:        true,
				Children:     make([]*FileTreeNode, 0),
			}
			rootMap[parts[0]] = curNode
			rootNodes = append(rootNodes, curNode)
		}

		for i := 1; i < len(parts)-1; i++ {
			part := parts[i]
			currentRel = filepath.Join(currentRel, part)
			var nextNode *FileTreeNode
			for _, child := range curNode.Children {
				if child.IsDir && child.Name == part {
					nextNode = child
					break
				}
			}
			if nextNode == nil {
				nextNode = &FileTreeNode{
					Name:         part,
					RelativePath: currentRel,
					FullPath:     filepath.Join(rootDir, currentRel),
					IsDir:        true,
					Children:     make([]*FileTreeNode, 0),
				}
				curNode.Children = append(curNode.Children, nextNode)
			}
			curNode = nextNode
		}

		// Leaf file node
		fileName := parts[len(parts)-1]
		fileNode := &FileTreeNode{
			Name:         fileName,
			RelativePath: f.RelativePath,
			FullPath:     f.FullPath,
			IsDir:        f.IsDir,
		}
		curNode.Children = append(curNode.Children, fileNode)
	}

	// Sort nodes: folders first (alphabetical), then files (alphabetical)
	var sortTree func(nodes []*FileTreeNode)
	sortTree = func(nodes []*FileTreeNode) {
		sort.Slice(nodes, func(i, j int) bool {
			if nodes[i].IsDir != nodes[j].IsDir {
				return nodes[i].IsDir
			}
			return strings.ToLower(nodes[i].Name) < strings.ToLower(nodes[j].Name)
		})
		for _, n := range nodes {
			if n.IsDir && len(n.Children) > 0 {
				sortTree(n.Children)
			}
		}
	}
	sortTree(rootNodes)

	return rootNodes
}

func (a *AppUI) isFolderExpanded(relPath string, depth int) bool {
	if a.Workspace.FilterQuery != "" {
		return true // Auto-expand all matching folders during search
	}
	if expanded, exists := a.ExpandedFileTreeFolders[relPath]; exists {
		return expanded
	}
	return depth == 0 // Top-level folders default to expanded
}

// renderFileTree renders the workspace file explorer: 35px header, filter
// input, collapsible folders and file tree with Tabler vector icons and dirty badges.
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

		if strings.HasPrefix(a.Workspace.RootDir, "ssh://") {
			unplugBtn := ui.Row(c).Size(22, 22).Radius(4).AlignItems(ui.Center).Justify(ui.Center)
			unplugBtn.Children(func() {
				ui.Icon(c, icons.TablerUnplug).Width(14).Height(14).TextColor(tc.Error)
			})
			if unplugBtn.Hovered() {
				unplugBtn.Background(tc.Error.Alpha(0.15))
			}
			if unplugBtn.Clicked() {
				a.DisconnectSSH()
			}
		}
	})

	// Shared hierarchical explorer with context menus (also rendered by the
	// right drawer's Files pane).
	a.renderExplorerTree(c, tc)
}
