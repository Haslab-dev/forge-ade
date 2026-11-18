package ui

import (
	"fmt"
	"os"
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
	"github.com/hasdev/forge-ade/core/agentsession"
)

// renderAgentSidebar renders the full-height task sessions sidebar with filter tabs and profile row.
func (a *AppUI) renderAgentSidebar(c *ui.Context) {
	t := c.Theme()
	tc := a.ThemeMgr.Colors()
	bar := c.TitleBar()
	topPadLeft := float32(14)
	if bar.Left > 0 {
		topPadLeft = max(bar.Left+10, 78)
	} else if runtime.GOOS == "darwin" {
		topPadLeft = 78
	}

	ui.Column(c).Grow(1).FillHeight().Background(tc.Sidebar).BorderWidth(0, 1, 0, 0).BorderColor(tc.SidebarBorder).Children(func() {
		// Top window controls strip (aligned with WorkspaceTopBarHeight & macOS traffic lights).
		ui.Row(c).Height(WorkspaceTopBarHeight).Padding(0, 10, 0, topPadLeft).Gap(4).AlignItems(ui.Center).
			DragWindow().Children(func() {
			if IconButton(c, tc, icons.TablerPanelLeft, 15).Clicked() {
				a.ShowLeftSidebar = false
			}
			if IconButton(c, tc, icons.TablerArrowLeft, 14).Clicked() {
				a.cycleAgentSession(-1)
			}
			if IconButton(c, tc, icons.TablerArrowRight, 14).Clicked() {
				a.cycleAgentSession(1)
			}
			ui.Box(c).Grow(1)
		})

		// forge-ade's leftSidebarView: the file explorer replaces the whole
		// tasks body ("Explore project files" swaps it in, "Back to tasks" out).
		if a.LeftSidebarView == "files" {
			a.renderSidebarFileExplorer(c, tc)
		} else {
			// Primary action rows: New task, Automations, Plugin Marketplace — ZCode stack.
			ui.Column(c).Padding(10, 12, 6, 12).Gap(2).Children(func() {
				isNewTaskActive := (a.Mode == ModeAgent && a.ActiveTaskKind == "agent" && (a.Agent == nil || a.Agent.ActiveSessionID == ""))
				newTask := SidebarActionButton(c, tc, icons.TablerCirclePlus, "New task", "⌘N", isNewTaskActive)
				if newTask.Clicked() {
					a.handleNewTask()
				}
				autos := SidebarActionButton(c, tc, icons.TablerClock, "Automations", "", false)
				if autos.Clicked() {
					a.openSettings(settings.SettingsTabAutos)
				}
				plugins := SidebarActionButton(c, tc, icons.TablerPuzzle, "Plugin Marketplace", "", false)
				if plugins.Clicked() {
					a.openSettings(settings.SettingsTabPlugins)
				}
			})

			// "Task list" section header with trailing search + filter icons
			// (ZCode header); the search icon toggles the filter input.
			ui.Row(c).Padding(10, 14, 4, 14).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "Task list").FontSize(13).FontWeight(700)
				ui.Box(c).Grow(1)
				if IconButton(c, tc, icons.TablerSearch, 13).Clicked() {
					a.SidebarSearchOpen = !a.SidebarSearchOpen
					if !a.SidebarSearchOpen {
						a.Agent.SearchQuery = ""
					}
				}
				if IconButton(c, tc, icons.TablerFilter, 13).Clicked() {
					a.Palette.Toggle()
				}
			})

			activeWsPath := ""
			if a.Workspace != nil {
				activeWsPath = a.Workspace.RootDir
			}

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
					if m.Item(menuChecked(a.SidebarListView != "timeline", "Group by project")).Chosen() {
						a.SidebarListView = "project"
						a.SaveUIPrefs()
					}
					if m.Item(menuChecked(a.SidebarListView == "timeline", "Chronological timeline")).Chosen() {
						a.SidebarListView = "timeline"
						a.SaveUIPrefs()
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

			// Search Input Filter — toggled from the Task list header icon.
			if a.SidebarSearchOpen {
				ui.Row(c).Padding(2, 10, 4, 10).Children(func() {
					in := ui.TextInput(c, &a.Agent.SearchQuery).Placeholder("Search tasks...").Grow(1)
					if in.Submitted() {
						a.SidebarSearchOpen = false
						a.Agent.SearchQuery = ""
					}
				})
			}

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

			// Scrollable Task Sessions List grouped deterministically by Project/Folder.
			groups := a.sidebarProjectGroups()
			tab := "active"
			if a.Agent != nil && a.Agent.ActiveFilterTab != "" {
				tab = a.Agent.ActiveFilterTab
			}
			searching := a.Agent != nil && strings.TrimSpace(a.Agent.SearchQuery) != ""

			// Visible non-General project names (after the project filter and
			// the empty-group hiding rules) drive the drag-and-drop and
			// chevron reorder actions. Only meaningful in the grouped
			// "Group by project" list view; the timeline mode has no headers.
			reorderNames := make([]string, 0, len(groups))
			if a.SidebarListView != "timeline" {
				for _, g := range groups {
					if g.ProjectName == "General" {
						continue
					}
					if a.ProjectFilter != "all" && a.ProjectFilter != "" && g.ProjectName != a.ProjectFilter {
						continue
					}
					if len(g.Sessions)+len(g.CLISessions) == 0 && (tab != "active" || searching) {
						continue
					}
					reorderNames = append(reorderNames, g.ProjectName)
				}
			}

			ui.Scroll(c).Grow(1).Children(func() {
				ui.Column(c).Padding(2, 6).Gap(4).Children(func() {
					// Flat Chronological Timeline mode (1:1 with webview AgentSidebar timeline mode)
					if a.SidebarListView == "timeline" {
						type timelineEntry struct {
							Kind        string
							ID          string
							Session     *agent.AgentSession
							CLISession  *agent.AgentCLISession
							UpdatedTime time.Time
							Pinned      bool
							ProjectName string
						}
						var allEntries []timelineEntry
						for _, group := range groups {
							if a.ProjectFilter != "all" && a.ProjectFilter != "" && group.ProjectName != a.ProjectFilter {
								continue
							}
							for _, s := range group.Sessions {
								allEntries = append(allEntries, timelineEntry{
									Kind:        "agent",
									ID:          s.ID,
									Session:     s,
									UpdatedTime: s.UpdatedAt,
									Pinned:      s.Pinned,
									ProjectName: group.ProjectName,
								})
							}
							for _, cs := range group.CLISessions {
								allEntries = append(allEntries, timelineEntry{
									Kind:        "cli",
									ID:          cs.ID,
									CLISession:  cs,
									UpdatedTime: cs.StartedAt,
									Pinned:      false,
									ProjectName: group.ProjectName,
								})
							}
						}

						sort.SliceStable(allEntries, func(i, j int) bool {
							if tab == "active" && allEntries[i].Pinned != allEntries[j].Pinned {
								return allEntries[i].Pinned
							}
							iTime := allEntries[i].UpdatedTime
							jTime := allEntries[j].UpdatedTime
							if a.TaskSortBy == "created" {
								if allEntries[i].Session != nil {
									iTime = allEntries[i].Session.CreatedAt
								}
								if allEntries[j].Session != nil {
									jTime = allEntries[j].Session.CreatedAt
								}
							}
							if !iTime.Equal(jTime) {
								return iTime.After(jTime)
							}
							return allEntries[i].ID > allEntries[j].ID
						})

						if len(allEntries) == 0 {
							ui.Column(c).Center().Padding(30).Gap(4).Children(func() {
								ui.Text(c, "No sessions found").FontSize(12).TextColor(tc.TextMuted)
							})
						} else {
							for _, entry := range allEntries {
								if entry.Kind == "agent" {
									s := entry.Session
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
										ProjectName: entry.ProjectName,
										AgentID:     s.AgentID,
										Model:       s.Model,
										UpdatedTime: s.UpdatedAt,
										DiffAdded:   added,
										DiffRemoved: removed,
										Progress:    progress,
										OnPin:       func() { a.Agent.TogglePin(s.ID) },
										OnFork:      func() { a.Agent.ForkSessionByID(s.ID) },
										OnArchive:   func() { a.Agent.ToggleArchive(s.ID) },
										OnSnooze:    func() { a.Agent.ToggleSnooze(s.ID) },
										OnRename:    func() { a.openRenameSessionModal(s.ID, s.Title, false) },
										OnDelete:    func() { a.Agent.DeleteSession(s.ID) },
										OnOpen: func() {
											a.selectAgentSession(s)
										},
									})
								} else {
									cs := entry.CLISession
									isActive := (a.ActiveTaskKind == "cli" && a.CLIMgr.ActiveID == cs.ID)
									isCLIRunning := cs.Status == "running" && cs.Terminal != nil && !cs.Terminal.IsExited()
									status := cs.Status
									if isCLIRunning {
										status = "running"
									} else if status == "running" {
										status = "completed"
									}
									cliAgentID := cs.AgentID
									if cliAgentID == "" {
										cliAgentID = cs.Command
									}
									renderTaskRow(c, tc, taskRowData{
										Key:         "cli-sess-" + cs.ID,
										Title:       cs.Title,
										Status:      status,
										Active:      isActive,
										IsCLI:       true,
										ProjectName: entry.ProjectName,
										AgentID:     cliAgentID,
										UpdatedTime: cs.StartedAt,
										OnStop:      func() { a.CLIMgr.StopSession(cs.ID) },
										OnRestart:   func() { a.CLIMgr.RestartSession(cs.ID) },
										OnRename:    func() { a.openRenameSessionModal(cs.ID, cs.Title, true) },
										OnDelete:    func() { a.CLIMgr.DeleteSession(cs.ID) },
										OnOpen: func() {
											a.selectCLISession(cs)
										},
									})
								}
							}
						}
						return
					}

					renderedAnyGroup := false
					for _, group := range groups {
						// "All projects ⌄" filter: only the chosen project renders.
						if a.ProjectFilter != "all" && a.ProjectFilter != "" && group.ProjectName != a.ProjectFilter {
							continue
						}
						totalCount := len(group.Sessions) + len(group.CLISessions)

						// Empty groups stay visible on the Active tab (when not searching)
						// so General, the active folder, and recent folders can start tasks
						// directly; hide empty groups under Pinned/Snoozed/Archived/Search.
						if totalCount == 0 && (tab != "active" || searching) {
							continue
						}
						renderedAnyGroup = true

						isCollapsed := a.CollapsedProjectGroups[group.ProjectName]

						// Project Group Header: chevron + folder + name, with the
						// hover actions (Explore project files / New task / Delete
						// workspace) that forge-ade shows on group hover.
						canExplore := group.ProjectName != "General" && group.FolderPath != ""
						headerActionClicked := false
						pHeader := ui.Row(c.Key("pheader-"+group.ProjectName)).Height(28).Radius(6).
							Padding(0, 6).Gap(5).AlignItems(ui.Center)

						// Drag-and-drop reorder (MyGo same-window value
						// drag-drop): non-pinned project headers drag their
						// name; dropping one on another moves it there.
						dropOver := false
						if group.ProjectName != "General" {
							pHeader.Drag(projectDragValue{Name: group.ProjectName})
							if dropped, ok := ui.Drop[projectDragValue](pHeader); ok {
								a.dropProjectOn(dropped.Name, group.ProjectName, reorderNames)
							}
							if _, over := ui.DragOver[projectDragValue](pHeader); over {
								dropOver = true
							}
						}
						pHeader.Children(func() {
							if isCollapsed {
								ui.Icon(c, icons.TablerChevronRight).Width(12).Height(12).TextColor(tc.IconMuted)
								ui.Icon(c, icons.TablerFolder).Width(14).Height(14).TextColor(tc.IconMuted)
							} else {
								ui.Icon(c, icons.TablerChevronDown).Width(12).Height(12).TextColor(tc.IconMuted)
								ui.Icon(c, icons.TablerFolderOpen).Width(14).Height(14).TextColor(tc.Accent)
							}

							ui.Text(c, group.ProjectName).FontSize(12).FontWeight(600).Grow(1).MinWidth(0).SingleLine()

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
										headerActionClicked = true
										onPick()
									}
								}

								if canExplore {
									groupAction("explore-"+group.ProjectName, icons.TablerFolderTree, false, func() {
										curWs := ""
										if a.Workspace != nil {
											curWs = a.Workspace.RootDir
										}
										if group.FolderPath != "" && !sameWorkspacePath(curWs, group.FolderPath) {
											a.switchWorkspace(group.FolderPath)
										}
										a.LeftSidebarView = "files"
									})
								}
								groupAction("new-"+group.ProjectName, icons.TablerPlus, false, func() {
									a.NewAgentTaskInProject(group.ProjectName, group.FolderPath)
								})
								if canExplore {
									groupAction("delws-"+group.ProjectName, icons.TablerTrash, true, func() {
										a.deleteWorkspaceGroup(c, group.ProjectName, group.FolderPath)
									})
								}

								// Reorder chevrons: swap with the previous/next
								// visible project, faded at the edges ("General"
								// is pinned first and shows none).
								if group.ProjectName != "General" && len(reorderNames) > 1 {
									projIdx := -1
									for i, n := range reorderNames {
										if n == group.ProjectName {
											projIdx = i
											break
										}
									}
									reorderBtn := func(key string, icon *ui.SVG, enabled bool, onPick func()) {
										btn := ui.Row(c.Key("pmove-"+key)).Size(18, 18).Radius(4).
											AlignItems(ui.Center).Justify(ui.Center)
										btn.Children(func() {
											ic := ui.Icon(c, icon).Width(12).Height(12)
											if enabled {
												ic.TextColor(tc.SidebarMutedForeground)
											} else {
												ic.TextColor(tc.SidebarMutedForeground.Alpha(0.35))
											}
										})
										if enabled && btn.Hovered() {
											btn.Background(tc.Text.Alpha(0.05))
										}
										if enabled && btn.Clicked() {
											headerActionClicked = true
											onPick()
										}
									}
									if projIdx >= 0 {
										reorderBtn("up-"+group.ProjectName, icons.TablerChevronUp, projIdx > 0, func() {
											a.moveProjectGroup(group.ProjectName, -1, reorderNames)
										})
										reorderBtn("down-"+group.ProjectName, icons.TablerChevronDown, projIdx < len(reorderNames)-1, func() {
											a.moveProjectGroup(group.ProjectName, 1, reorderNames)
										})
									}
								}
							}
						})

						if dropOver {
							pHeader.Background(tc.Accent.Alpha(0.14))
						} else if pHeader.Hovered() {
							pHeader.Background(tc.SidebarRowHover)
						}
						if pHeader.Clicked() && !headerActionClicked {
							a.CollapsedProjectGroups[group.ProjectName] = !isCollapsed
						}
						pHeader.ContextMenu(func(m *ui.Menu) {
							if m.Item("New task in " + group.ProjectName).Chosen() {
								a.NewAgentTaskInProject(group.ProjectName, group.FolderPath)
							}
							if m.Item("New task with Agent / CLI…").Chosen() {
								if group.ProjectName == "General" || group.FolderPath == "" {
									a.OpenNewTaskModal()
								} else {
									a.OpenNewTaskModalFor(group.FolderPath)
								}
							}
							if canExplore {
								m.Separator()
								if m.Item("Explore project files").Chosen() {
									curWs := ""
									if a.Workspace != nil {
										curWs = a.Workspace.RootDir
									}
									if !sameWorkspacePath(curWs, group.FolderPath) {
										a.switchWorkspace(group.FolderPath)
									}
									a.LeftSidebarView = "files"
								}
								if m.Item("Open in Finder").Chosen() {
									mygo.Shell.ShowItemInFolder(group.FolderPath)
								}
								m.Separator()
								if m.Item("Remove workspace").Danger().Chosen() {
									a.deleteWorkspaceGroup(c, group.ProjectName, group.FolderPath)
								}
							}
						})

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
										Model:       s.Model,
										UpdatedTime: s.UpdatedAt,
										DiffAdded:   added,
										DiffRemoved: removed,
										Progress:    progress,
										OnPin:       func() { a.Agent.TogglePin(s.ID) },
										OnFork:      func() { a.Agent.ForkSessionByID(s.ID) },
										OnArchive:   func() { a.Agent.ToggleArchive(s.ID) },
										OnSnooze:    func() { a.Agent.ToggleSnooze(s.ID) },
										OnRename:    func() { a.openRenameSessionModal(s.ID, s.Title, false) },
										OnDelete:    func() { a.Agent.DeleteSession(s.ID) },
										OnOpen: func() {
											a.selectAgentSession(s)
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
									cliAgentID := cs.AgentID
									if cliAgentID == "" {
										cliAgentID = cs.Command
									}
									renderTaskRow(c, tc, taskRowData{
										Key:         "cli-sess-" + cs.ID,
										Title:       cs.Title,
										Status:      status,
										Active:      isActive,
										IsCLI:       true,
										AgentID:     cliAgentID,
										UpdatedTime: cs.StartedAt,
										OnStop:      func() { a.CLIMgr.StopSession(cs.ID) },
										OnRestart:   func() { a.CLIMgr.RestartSession(cs.ID) },
										OnRename:    func() { a.openRenameSessionModal(cs.ID, cs.Title, true) },
										OnDelete:    func() { a.CLIMgr.DeleteSession(cs.ID) },
										OnOpen: func() {
											a.selectCLISession(cs)
										},
									})
								}
							})
						}
					}
					if !renderedAnyGroup {
						ui.Column(c).Center().Padding(30).Gap(4).Children(func() {
							ui.Text(c, "No sessions found").FontSize(12).TextColor(tc.TextMuted)
						})
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
					a.openSettings(settings.SettingsTabAutos)
				}
			})
			botRow := SidebarActionButton(c, tc, icons.TablerBot, "Add a bot or schedule", "", false)
			if botRow.Clicked() {
				// Automations manager, as forge-ade's AutomationsModal entry.
				a.openSettings(settings.SettingsTabAutos)
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
				dockBtn(icons.TablerBook, false, func() { a.OpenRulesModal() })
				dockBtn(icons.TablerSettings, a.Mode == ModeSettings, func() {
					a.openSettings()
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
			a.collapseAllExplorerFolders()
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

// sidebarProjectGroup holds sessions grouped under one workspace project.
type sidebarProjectGroup struct {
	ProjectName string
	FolderPath  string
	Sessions    []*agent.AgentSession
	CLISessions []*agent.AgentCLISession
}

// sidebarProjectGroups returns deterministically ordered workspace groups:
// "General" is always first, followed by all other workspace groups sorted
// case-insensitively alphabetically. Within each group, sessions are sorted
// deterministically so selection, streaming, or reloads never shuffle rows.
func (a *AppUI) sidebarProjectGroups() []*sidebarProjectGroup {
	activeWsPath := ""
	if a.Workspace != nil {
		activeWsPath = a.Workspace.RootDir
		if !strings.HasPrefix(activeWsPath, "ssh://") {
			activeWsPath = agentsession.NormalizeWorkspacePath(activeWsPath)
		}
	}
	activeProjectName := resolveProjectName(activeWsPath)

	groupMap := make(map[string]*sidebarProjectGroup)
	getOrCreateGroup := func(name, path string) *sidebarProjectGroup {
		if strings.EqualFold(name, "General") || name == "" {
			name = "General"
			path = ""
		}
		normPath := path
		if normPath != "" && !strings.HasPrefix(normPath, "ssh://") {
			normPath = agentsession.NormalizeWorkspacePath(normPath)
		}
		if g, exists := groupMap[name]; exists {
			if g.FolderPath == "" && normPath != "" && name != "General" {
				g.FolderPath = normPath
			}
			return g
		}
		g := &sidebarProjectGroup{
			ProjectName: name,
			FolderPath:  normPath,
		}
		groupMap[name] = g
		return g
	}

	// Always seed General and the active workspace.
	getOrCreateGroup("General", "")
	getOrCreateGroup(activeProjectName, activeWsPath)

	tab := "active"
	searchQuery := ""
	if a.Agent != nil {
		if a.Agent.ActiveFilterTab != "" {
			tab = a.Agent.ActiveFilterTab
		}
		searchQuery = strings.TrimSpace(a.Agent.SearchQuery)
	}

	// On the Active tab without search, seed valid existing non-temp opened folders.
	if tab == "active" && searchQuery == "" {
		for _, rp := range a.RecentWorkspaces {
			normRP := rp
			if !strings.HasPrefix(normRP, "ssh://") {
				normRP = agentsession.NormalizeWorkspacePath(normRP)
			}
			if normRP == "" || agentsession.IsTempTestPath(normRP) {
				continue
			}
			if !strings.HasPrefix(normRP, "ssh://") {
				if info, err := os.Stat(normRP); err != nil || !info.IsDir() {
					continue
				}
			}
			getOrCreateGroup(resolveProjectName(normRP), normRP)
		}
	}

	// 1. Group filtered ACP Sessions.
	if a.Agent != nil {
		for _, s := range a.Agent.FilteredSessions() {
			if s == nil {
				continue
			}
			wsPath := s.WorkspacePath
			if !strings.HasPrefix(wsPath, "ssh://") {
				wsPath = agentsession.NormalizeWorkspacePath(wsPath)
			}
			pName := resolveProjectName(wsPath)
			g := getOrCreateGroup(pName, wsPath)
			g.Sessions = append(g.Sessions, s)
		}
	}

	// 2. Group CLI Sessions deterministically on the Active tab.
	if a.CLIMgr != nil && tab == "active" {
		sq := strings.ToLower(searchQuery)
		for _, cs := range a.CLIMgr.SortedSessions() {
			if cs == nil {
				continue
			}
			if sq != "" &&
				!strings.Contains(strings.ToLower(cs.Title), sq) &&
				!strings.Contains(strings.ToLower(cs.AgentName), sq) &&
				!strings.Contains(strings.ToLower(cs.Command), sq) {
				continue
			}
			wsPath := cs.Cwd
			if !strings.HasPrefix(wsPath, "ssh://") {
				wsPath = agentsession.NormalizeWorkspacePath(wsPath)
			}
			pName := resolveProjectName(wsPath)
			g := getOrCreateGroup(pName, wsPath)
			g.CLISessions = append(g.CLISessions, cs)
		}
	}

	names := make([]string, 0, len(groupMap))
	for name := range groupMap {
		names = append(names, name)
	}
	// Custom order (a.ProjectOrder, persisted via ui prefs) takes precedence:
	// "General" stays pinned first (existing invariant), ordered names follow
	// in their stored sequence, and names never moved keep the implicit
	// alphabetical order after them. TaskSortBy sorts sessions WITHIN each
	// group and is unaffected.
	orderIdx := make(map[string]int, len(a.ProjectOrder))
	for i, name := range a.ProjectOrder {
		if _, dup := orderIdx[name]; !dup {
			orderIdx[name] = i
		}
	}
	sort.SliceStable(names, func(i, j int) bool {
		if names[i] == "General" {
			return true
		}
		if names[j] == "General" {
			return false
		}
		oi, oki := orderIdx[names[i]]
		oj, okj := orderIdx[names[j]]
		if oki != okj {
			return oki
		}
		if oki && oi != oj {
			return oi < oj
		}
		li, lj := strings.ToLower(names[i]), strings.ToLower(names[j])
		if li != lj {
			return li < lj
		}
		return names[i] < names[j]
	})

	groups := make([]*sidebarProjectGroup, 0, len(names))
	for _, name := range names {
		g := groupMap[name]
		sort.SliceStable(g.Sessions, func(i, j int) bool {
			si, sj := g.Sessions[i], g.Sessions[j]
			if tab == "active" && si.Pinned != sj.Pinned {
				return si.Pinned
			}
			if a.TaskSortBy == "created" {
				if !si.CreatedAt.Equal(sj.CreatedAt) {
					return si.CreatedAt.After(sj.CreatedAt)
				}
			} else {
				if !si.UpdatedAt.Equal(sj.UpdatedAt) {
					return si.UpdatedAt.After(sj.UpdatedAt)
				}
			}
			return si.ID > sj.ID
		})
		groups = append(groups, g)
	}
	return groups
}

// projectDragValue is the value a sidebar project header drags during
// drag-and-drop reorder. A dedicated type makes project rows accept only
// other project rows (never files, tabs, or other future drag values).
type projectDragValue struct{ Name string }

// swapProjectOrder returns a copy of names with name moved one slot toward
// dir (-1 up, +1 down). "General" is pinned: it never moves and no other
// project may swap across it. Returns nil when the move is impossible
// (unknown name, at an edge, or the only neighbour is General).
func swapProjectOrder(names []string, name string, dir int) []string {
	if name == "" || name == "General" || (dir != -1 && dir != 1) {
		return nil
	}
	idx := -1
	for i, n := range names {
		if n == name {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil
	}
	j := idx + dir
	if j < 0 || j >= len(names) || names[j] == "General" {
		return nil
	}
	out := make([]string, len(names))
	copy(out, names)
	out[idx], out[j] = out[j], out[idx]
	return out
}

// moveProjectTo returns a copy of names with the dragged project moved to
// the drop-target project's position. "General" is pinned, so dragging it
// or dropping onto it is a no-op. Returns nil when the move is impossible.
func moveProjectTo(names []string, dragged, target string) []string {
	if dragged == "" || target == "" || dragged == target ||
		dragged == "General" || target == "General" {
		return nil
	}
	from, to := -1, -1
	for i, n := range names {
		if n == dragged && from < 0 {
			from = i
		}
		if n == target && to < 0 {
			to = i
		}
	}
	if from < 0 || to < 0 {
		return nil
	}
	out := make([]string, 0, len(names))
	out = append(out, names[:from]...)
	out = append(out, names[from+1:]...)
	if to > len(out) {
		to = len(out)
	}
	out = append(out[:to], append([]string{dragged}, out[to:]...)...)
	return out
}

// persistProjectOrder stores the full intended display order (General first,
// never stored itself) as the custom ProjectOrder. Previously-ordered names
// that are not currently visible (hidden by the filter, another tab, or an
// empty group) keep their relative order appended after the visible ones so
// a filtered reorder never erases hidden custom order. Saves prefs.
func (a *AppUI) persistProjectOrder(ordered []string) {
	seen := make(map[string]bool, len(ordered)+len(a.ProjectOrder))
	out := make([]string, 0, len(a.ProjectOrder)+len(ordered))
	for _, n := range ordered {
		if n == "" || n == "General" || seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, n)
	}
	for _, n := range a.ProjectOrder {
		if seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, n)
	}
	a.ProjectOrder = out
	a.SaveUIPrefs()
}

// moveProjectGroup swaps the named project one slot up (-1) or down (+1)
// within the given visible non-General project names and persists the
// custom order. "General" is pinned first.
func (a *AppUI) moveProjectGroup(name string, dir int, visible []string) {
	names := make([]string, 0, len(visible)+1)
	names = append(names, "General")
	names = append(names, visible...)
	swapped := swapProjectOrder(names, name, dir)
	if swapped == nil {
		return
	}
	a.persistProjectOrder(swapped[1:])
}

// dropProjectOn moves the dragged project to the dropped-on project's
// position within the visible non-General project names and persists it.
func (a *AppUI) dropProjectOn(dragged, target string, visible []string) {
	names := make([]string, 0, len(visible)+1)
	names = append(names, "General")
	names = append(names, visible...)
	moved := moveProjectTo(names, dragged, target)
	if moved == nil {
		return
	}
	a.persistProjectOrder(moved[1:])
}

// resolveProjectName extracts a clean human-readable workspace project name.
func resolveProjectName(folderPath string) string {
	clean := strings.TrimSpace(folderPath)
	if strings.HasPrefix(clean, "ssh://") {
		_, remotePath, displayHost, folderName := parseSSHWorkspaceURI(clean)
		if folderName != "" && folderName != "remote-root" && folderName != "." && folderName != "/" {
			return folderName
		}
		if displayHost != "" {
			return displayHost
		}
		base := filepath.Base(filepath.Clean(remotePath))
		if base != "." && base != "/" && base != "" {
			return base
		}
		return "remote"
	}
	norm := agentsession.NormalizeWorkspacePath(clean)
	if norm == "" {
		return "General"
	}
	base := filepath.Base(filepath.Clean(norm))
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

// sidebarProjectNames returns deterministic project filter options: "General"
// first, followed by all remaining projects sorted case-insensitively alphabetically.
func (a *AppUI) sidebarProjectNames(activeProject string) []string {
	seen := map[string]bool{"General": true}
	if activeProject != "" {
		seen[activeProject] = true
	}
	for _, rp := range a.RecentWorkspaces {
		normRP := rp
		if !strings.HasPrefix(normRP, "ssh://") {
			normRP = agentsession.NormalizeWorkspacePath(normRP)
		}
		if normRP == "" || agentsession.IsTempTestPath(normRP) {
			continue
		}
		if !strings.HasPrefix(normRP, "ssh://") {
			if info, err := os.Stat(normRP); err != nil || !info.IsDir() {
				continue
			}
		}
		seen[resolveProjectName(normRP)] = true
	}
	if a.Agent != nil {
		for _, sess := range a.Agent.AllSessions() {
			if sess == nil {
				continue
			}
			seen[resolveProjectName(sess.WorkspacePath)] = true
		}
	}
	if a.CLIMgr != nil {
		for _, cs := range a.CLIMgr.SortedSessions() {
			if cs == nil {
				continue
			}
			seen[resolveProjectName(cs.Cwd)] = true
		}
	}

	others := make([]string, 0, len(seen))
	for name := range seen {
		if name == "General" || name == "" {
			continue
		}
		others = append(others, name)
	}
	sort.Slice(others, func(i, j int) bool {
		li, lj := strings.ToLower(others[i]), strings.ToLower(others[j])
		if li != lj {
			return li < lj
		}
		return others[i] < others[j]
	})
	return append([]string{"General"}, others...)
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
			if _, exists := rootMap[parts[0]]; exists {
				continue // duplicate entry (e.g. dir + captured file)
			}
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

		// Leaf file node — skip when a folder node of the same name already
		// exists (a lazily-listed directory entry colliding with a captured
		// child file would otherwise create two identically-keyed rows).
		fileName := parts[len(parts)-1]
		dup := false
		for _, child := range curNode.Children {
			if child.Name == fileName {
				dup = true
				break
			}
		}
		if !dup {
			fileNode := &FileTreeNode{
				Name:         fileName,
				RelativePath: f.RelativePath,
				FullPath:     f.FullPath,
				IsDir:        f.IsDir,
			}
			curNode.Children = append(curNode.Children, fileNode)
		}
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
	return false // Folders default to collapsed
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
		treeBtn(icons.TablerChevronsDownUp, func() {
			a.collapseAllExplorerFolders()
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
			treeBtn(icons.TablerUpload, func() {
				a.OpenRemoteFileUploader("")
			})
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

	// Filter input
	ui.Row(c).Padding(6, 10).Children(func() {
		ui.TextInput(c, &a.Workspace.FilterQuery).Placeholder("Filter files...").Grow(1)
	})

	// Shared hierarchical explorer with context menus (also rendered by the
	// right drawer's Files pane).
	a.renderExplorerTree(c, tc)
}
