package ui

import (
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/glass"
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

	outer := ui.Box(c).Grow(1).FillHeight()
	if a.glassBlurActive() {
		outer.Material(glass.Blur{Radius: 18})
	}
	outer.Children(func() {
		ui.Column(c).Fill().Background(tc.Sidebar).BorderWidth(0, 1, 0, 0).BorderColor(tc.SidebarBorder).Children(func() {
			// Top window controls strip (Orca style: traffic lights -> ForgeADE wordmark -> sidebar toggle).
			ui.Row(c).Height(WorkspaceTopBarHeight).Padding(0, 10, 0, topPadLeft).Gap(8).AlignItems(ui.Center).
				DragWindow().Children(func() {
				ui.Text(c, "ForgeADE").FontSize(12.5).FontWeight(600).TextColor(tc.SidebarForeground).SingleLine()
				if IconButton(c, tc, icons.TablerPanelLeft, 15).Tooltip("Hide Sidebar").Clicked() {
					a.ShowLeftSidebar = false
				}
				ui.Box(c).Grow(1)
			})

			// forge-ade's leftSidebarView: the file explorer replaces the whole
			// tasks body ("Explore project files" swaps it in, "Back to tasks" out).
			if a.LeftSidebarView == "files" {
				a.renderSidebarFileExplorer(c, tc)
			} else {
				// Orca-inspired compact Quick Nav Cluster: Search + Tasks (Board) + Automations + New Task
				ui.Column(c).Padding(8, 10, 6, 10).Gap(3).BorderWidth(0, 0, 1, 0).BorderColor(tc.SidebarBorder).Children(func() {
					// Inline Search bar (click to activate text input)
					if a.SidebarSearchOpen {
						ui.Row(c.Key("sb-search-open")).Height(28).Radius(7).Padding(0, 8).Gap(6).
							AlignItems(ui.Center).Background(tc.SidebarControlSurface).
							BorderWidth(1).BorderColor(tc.Focus.Alpha(0.6)).Children(func() {
							ui.Icon(c, icons.TablerSearch).Width(13).Height(13).TextColor(tc.SidebarMutedForeground)
							in := ui.TextInput(c, &a.Agent.SearchQuery).Placeholder("Search sessions...").Grow(1)
							if in.Submitted() {
								a.SidebarSearchOpen = false
								a.Agent.SearchQuery = ""
							}
							clr := ui.Row(c.Key("sb-search-close")).Size(16, 16).Radius(4).AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
							clr.Children(func() {
								ui.Icon(c, icons.TablerX).Width(10).Height(10).TextColor(tc.SidebarMutedForeground)
							})
							if clr.Hovered() {
								clr.Background(tc.Text.Alpha(0.08))
							}
							if clr.Clicked() {
								a.SidebarSearchOpen = false
								a.Agent.SearchQuery = ""
							}
						})
					} else {
						searchRow := ui.Row(c.Key("sb-search-trigger")).Height(28).Radius(7).Padding(0, 9).Gap(7).
							AlignItems(ui.Center).Cursor(ui.CursorPointer).
							Background(tc.SidebarControlSurface)
						searchRow.Children(func() {
							ui.Icon(c, icons.TablerSearch).Width(13).Height(13).TextColor(tc.SidebarMutedForeground)
							ui.Text(c, "Search").FontSize(12).TextColor(tc.SidebarMutedForeground).Grow(1)
						})
						if searchRow.Hovered() {
							searchRow.Background(tc.SidebarRowHover)
						}
						if searchRow.Clicked() {
							a.SidebarSearchOpen = true
						}
					}

					navRow := func(key string, icon *ui.SVG, label string, badge string, active bool, onClick func()) {
						r := ui.Row(c.Key("sb-nav-"+key)).Height(27).Radius(6).Padding(0, 8).Gap(8).
							AlignItems(ui.Center).Cursor(ui.CursorPointer)
						if active {
							r.Background(tc.SidebarRowActive)
						} else if r.Hovered() {
							r.Background(tc.SidebarRowHover)
						}
						r.Children(func() {
							ic := ui.Icon(c, icon).Width(14).Height(14)
							if active {
								ic.TextColor(tc.Accent)
							} else {
								ic.TextColor(tc.SidebarMutedForeground)
							}
							txt := ui.Text(c, label).FontSize(12.5).Grow(1).SingleLine()
							if active {
								txt.FontWeight(600).TextColor(tc.Text)
							} else {
								txt.FontWeight(500).TextColor(tc.SidebarForeground)
							}
							if badge != "" {
								ui.Row(c).Height(16).Radius(4).Padding(0, 5).AlignItems(ui.Center).
									Background(tc.ToolbarControl).Children(func() {
									ui.Text(c, badge).FontSize(10).FontWeight(500).TextColor(tc.SidebarMutedForeground)
								})
							}
						})
						if r.Clicked() {
							onClick()
						}
					}

					boardBadge := ""
					if n := a.boardTaskCount(); n > 0 {
						boardBadge = fmt.Sprintf("%d", n)
					}
					navRow("tasks", icons.TablerLayoutKanban, "Tasks", boardBadge, a.Mode == ModeAgent && a.AgentViewMode == AgentViewBoard, func() {
						a.IsGitClientOpen = false
						a.ActiveTaskKind = "agent"
						a.openBoardSurface()
					})
					navRow("automations", icons.TablerClock, "Automations", "", false, func() {
						a.openSettings(settings.SettingsTabAutos)
					})
				})

				// Collapsible Local Bots section (Orca Bots style)
				if a.Bots != nil {
					bots := a.Bots.List()
					ui.Column(c).Padding(6, 10, 4, 10).Gap(3).BorderWidth(0, 0, 1, 0).BorderColor(tc.SidebarBorder).Children(func() {
						ui.Row(c).Height(22).Gap(4).AlignItems(ui.Center).Children(func() {
							hdrToggle := ui.Row(c.Key("sb-bots-toggle")).Gap(4).AlignItems(ui.Center).Cursor(ui.CursorPointer)
							hdrToggle.Children(func() {
								chev := icons.TablerChevronDown
								if a.BotsSectionCollapsed {
									chev = icons.TablerChevronRight
								}
								ui.Icon(c, chev).Width(11).Height(11).TextColor(tc.SidebarMutedForeground)
								ui.Text(c, "Bots").FontSize(11.5).FontWeight(600).TextColor(tc.SidebarMutedForeground)
							})
							if hdrToggle.Clicked() {
								a.BotsSectionCollapsed = !a.BotsSectionCollapsed
							}
							ui.Box(c).Grow(1)
							addBotBtn := ui.Row(c.Key("sb-bots-add")).Size(20, 20).Radius(5).AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
							addBotBtn.Tooltip("New Local Bot")
							addBotBtn.Children(func() {
								ui.Icon(c, icons.TablerPlus).Width(12).Height(12).TextColor(tc.SidebarMutedForeground)
							})
							if addBotBtn.Hovered() {
								addBotBtn.Background(tc.Text.Alpha(0.06))
							}
							if addBotBtn.Clicked() {
								a.OpenNewBotModal()
							}
						})
						if !a.BotsSectionCollapsed {
							if len(bots) == 0 {
								emptyBot := ui.Row(c.Key("sb-bots-empty")).Height(26).Radius(6).Padding(0, 8).Gap(6).
									AlignItems(ui.Center).Cursor(ui.CursorPointer)
								emptyBot.Children(func() {
									ui.Icon(c, icons.TablerBot).Width(13).Height(13).TextColor(tc.SidebarMutedForeground)
									ui.Text(c, "Create your first bot…").FontSize(11.5).TextColor(tc.SidebarMutedForeground).Grow(1)
								})
								if emptyBot.Hovered() {
									emptyBot.Background(tc.SidebarRowHover)
								}
								if emptyBot.Clicked() {
									a.OpenNewBotModal()
								}
							} else {
								for _, b := range bots {
									bot := b
									botActive := a.Agent != nil && a.Agent.ActiveSession() != nil && a.Agent.ActiveSession().BotID == bot.ID
									workingCnt, needsCnt := a.botLiveCounts(bot)
									bRow := ui.Row(c.Key("sb-bot-"+bot.ID)).Height(42).Radius(8).Padding(0, 8).Gap(9).
										AlignItems(ui.Center).Cursor(ui.CursorPointer)
									if botActive {
										bRow.Background(tc.SidebarRowActive)
									} else if bRow.Hovered() {
										bRow.Background(tc.SidebarRowHover)
									}
									if bot.Paused {
										bRow.Opacity(0.55)
									}
									bRow.Children(func() {
										a.renderBotAvatar(c, tc, bot, 30, 9)
										ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
											ui.Text(c, bot.Name).FontSize(12.5).FontWeight(600).TextColor(tc.Text).SingleLine()
											sub := bot.Role
											subColor := tc.SidebarMutedForeground
											if needsCnt > 0 {
												sub = fmt.Sprintf("%d needs you", needsCnt)
												subColor = tc.Warning
											} else if workingCnt > 0 {
												if workingCnt == 1 {
													sub = "Running…"
												} else {
													sub = fmt.Sprintf("%d running…", workingCnt)
												}
												subColor = tc.Accent
											} else if bot.Paused {
												sub = "Paused"
											}
											if sub != "" {
												ui.Text(c, sub).FontSize(10.5).TextColor(subColor).SingleLine()
											}
										})
										if needsCnt > 0 {
											ui.Box(c).Size(7, 7).Radius(4).Background(tc.Warning).Shrink(0)
										} else if workingCnt > 0 {
											ui.Spinner(c).Size(12, 12).Label("")
										}
									})
									if bRow.Clicked() && a.Agent != nil {
										if home := a.Agent.OpenBotHomeSession(a.Bots, bot); home != nil {
											a.ensureAgentTab(home.ID)
										}
										a.IsGitClientOpen = false
										a.AgentViewMode = AgentViewChat
										a.ActiveTaskKind = "agent"
										a.ShowRightDrawer = true
										a.RightDrawerTab = RightTabBot
										if a.BotDrawerSubTab == "" {
											a.BotDrawerSubTab = "tasks"
										}
									}
									bRow.ContextMenu(func(m *ui.Menu) {
										if m.Item("Open Home Chat").Chosen() && a.Agent != nil {
											if home := a.Agent.OpenBotHomeSession(a.Bots, bot); home != nil {
												a.ensureAgentTab(home.ID)
											}
											a.ShowRightDrawer = true
											a.RightDrawerTab = RightTabBot
										}
										if m.Item("New Background Task…").Chosen() {
											a.OpenNewBotTaskModal(bot.ID)
										}
										if m.Item("Change Avatar…").Chosen() {
											a.OpenBotAvatarModal(bot.ID)
										}
										if a.Agent != nil && a.Agent.ActiveSession() != nil {
											if m.Item("Hand current session to " + bot.Name).Chosen() {
												src := a.Agent.ActiveSession()
												_, _ = a.Agent.HandToBot(bot, src.ID, src.Title, true)
											}
										}
										pauseLbl := "Pause bot"
										if bot.Paused {
											pauseLbl = "Resume bot"
										}
										if m.Item(pauseLbl).Chosen() {
											a.Bots.TogglePause(bot.ID)
										}
										m.Separator()
										if m.Item("Delete bot").Danger().Chosen() {
											a.Bots.Delete(bot.ID)
										}
									})
								}
							}
						}
					})
				}

				activeWsPath := ""
				if a.Workspace != nil {
					activeWsPath = a.Workspace.RootDir
				}

				// Orca-inspired single-row Projects Section Header (replaces separate
				// project-dropdown + Active/Pinned/Archived pill rows for cleaner density).
				activeProject := resolveProjectName(activeWsPath)
				_, pinnedCount, snoozedCount, archivedCount := a.Agent.TabCounts()
				curTab := a.Agent.ActiveFilterTab
				if curTab == "" {
					curTab = "active"
				}
				isFiltered := curTab != "active" || (a.ProjectFilter != "" && a.ProjectFilter != "all") || a.SidebarListView == "timeline"

				ui.Row(c).Padding(8, 10, 4, 12).Gap(4).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "Projects").FontSize(11.5).FontWeight(600).TextColor(tc.SidebarMutedForeground).SingleLine()

					// Show active filter badge only when non-default filter is active (click to reset)
					if curTab != "active" {
						tag := strings.ToUpper(curTab[:1]) + curTab[1:]
						chip := ui.Row(c.Key("sb-active-tab-chip")).Height(17).Radius(4).Padding(0, 5).Gap(3).
							AlignItems(ui.Center).Cursor(ui.CursorPointer).Background(tc.Accent.Alpha(0.14))
						chip.Tooltip("Click to return to Active sessions")
						chip.Children(func() {
							ui.Text(c, tag).FontSize(9.5).FontWeight(600).TextColor(tc.Accent)
							ui.Icon(c, icons.TablerX).Width(9).Height(9).TextColor(tc.Accent)
						})
						if chip.Clicked() {
							a.Agent.ActiveFilterTab = "active"
						}
					} else if a.ProjectFilter != "" && a.ProjectFilter != "all" {
						chip := ui.Row(c.Key("sb-active-proj-chip")).Height(17).Radius(4).Padding(0, 5).Gap(3).
							AlignItems(ui.Center).Cursor(ui.CursorPointer).Background(tc.Accent.Alpha(0.14))
						chip.Tooltip("Click to show all projects")
						chip.Children(func() {
							ui.Text(c, a.ProjectFilter).FontSize(9.5).FontWeight(600).TextColor(tc.Accent).MaxWidth(76).SingleLine()
							ui.Icon(c, icons.TablerX).Width(9).Height(9).TextColor(tc.Accent)
						})
						if chip.Clicked() {
							a.ProjectFilter = "all"
						}
					}

					ui.Box(c).Grow(1)

					smallAction := func(key, tip string, icon *ui.SVG, active bool, onPick func()) {
						btn := ui.Row(c.Key("hdract-"+key)).Size(22, 22).Radius(5).AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
						btn.Tooltip(tip)
						btn.Children(func() {
							ic := ui.Icon(c, icon).Width(13).Height(13)
							if active {
								ic.TextColor(tc.Accent)
							} else {
								ic.TextColor(tc.SidebarMutedForeground)
							}
						})
						if active {
							btn.Background(tc.Accent.Alpha(0.14))
						} else if btn.Hovered() {
							btn.Background(tc.Text.Alpha(0.06))
						}
						if btn.Clicked() {
							onPick()
						}
					}

					// Unified Filter, View & Sort Menu button
					filterBtn := ui.Row(c.Key("sb-proj-filter")).Size(22, 22).Radius(5).
						AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
					filterBtn.Tooltip("Filter, view & sort sessions")
					filterBtn.Children(func() {
						ic := ui.Icon(c, icons.TablerFilter).Width(13).Height(13)
						if isFiltered {
							ic.TextColor(tc.Accent)
						} else {
							ic.TextColor(tc.SidebarMutedForeground)
						}
					})
					if isFiltered {
						filterBtn.Background(tc.Accent.Alpha(0.14))
					} else if filterBtn.Hovered() {
						filterBtn.Background(tc.Text.Alpha(0.06))
					}
					filterBtn.Menu(func(m *ui.Menu) {
						if m.Item(menuChecked(curTab == "active", "Active")).Chosen() {
							a.Agent.ActiveFilterTab = "active"
						}
						pinLabel := "Pinned"
						if pinnedCount > 0 {
							pinLabel = fmt.Sprintf("Pinned (%d)", pinnedCount)
						}
						if m.Item(menuChecked(curTab == "pinned", pinLabel)).Chosen() {
							a.Agent.ActiveFilterTab = "pinned"
						}
						if snoozedCount > 0 || curTab == "snoozed" {
							if m.Item(menuChecked(curTab == "snoozed", fmt.Sprintf("Snoozed (%d)", snoozedCount))).Chosen() {
								a.Agent.ActiveFilterTab = "snoozed"
							}
						}
						archLabel := "Archived"
						if archivedCount > 0 {
							archLabel = fmt.Sprintf("Archived (%d)", archivedCount)
						}
						if m.Item(menuChecked(curTab == "archived", archLabel)).Chosen() {
							a.Agent.ActiveFilterTab = "archived"
						}
						m.Separator()
						if m.Item(menuChecked(a.ProjectFilter == "all" || a.ProjectFilter == "", "All projects")).Chosen() {
							a.ProjectFilter = "all"
						}
						for _, name := range a.sidebarProjectNames(activeProject) {
							thisName := name
							if m.Item(menuChecked(a.ProjectFilter == thisName, thisName)).Chosen() {
								a.ProjectFilter = thisName
							}
						}
						m.Separator()
						if m.Item(menuChecked(a.SidebarListView != "timeline", "Group by host & project")).Chosen() {
							a.SidebarListView = "project"
							a.SaveUIPrefs()
						}
						if m.Item(menuChecked(a.SidebarListView == "timeline", "Chronological timeline")).Chosen() {
							a.SidebarListView = "timeline"
							a.SaveUIPrefs()
						}
						m.Separator()
						if m.Item(menuChecked(a.TaskSortBy == "updated", "Sort by recent activity")).Chosen() {
							a.TaskSortBy = "updated"
						}
						if m.Item(menuChecked(a.TaskSortBy == "created", "Sort by created date")).Chosen() {
							a.TaskSortBy = "created"
						}
					})

					smallAction("newcp", "New custom project (multi-folder)", icons.TablerPlus, false, func() { a.openNewCustomProjectModal() })
					smallAction("open", "Open workspace folder", icons.TablerFolderPlus, false, func() { a.openWorkspaceFolder() })
					collapsedOthers := a.otherProjectsCollapsed(activeProject)
					if collapsedOthers {
						smallAction("expand", "Expand all projects", icons.TablerMaximize2, false, func() { a.expandAllProjects() })
					} else {
						smallAction("collapse", "Collapse other projects", icons.TablerMinimize2, false, func() { a.collapseOtherProjects(activeProject) })
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
											AgentType:   s.AgentType,
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
										a.renderSessionNestedChildren(c, tc, s)
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

						// Bucket visible project groups by execution host (Orca style: Local Mac / SSH host)
						type hostBucket struct {
							ID       string
							Label    string
							IsRemote bool
							Groups   []*sidebarProjectGroup
							Sessions int
						}
						hostMap := make(map[string]*hostBucket)
						var hostList []*hostBucket
						localLabel := "Local"
						if runtime.GOOS == "darwin" {
							localLabel = "Local Mac"
						}
						for _, group := range groups {
							if a.ProjectFilter != "all" && a.ProjectFilter != "" && group.ProjectName != a.ProjectFilter {
								continue
							}
							totalCount := len(group.Sessions) + len(group.CLISessions)
							if totalCount == 0 && (tab != "active" || searching) {
								continue
							}
							hID := "local"
							hLabel := localLabel
							isRem := false
							if strings.HasPrefix(group.FolderPath, "ssh://") {
								isRem = true
								rest := strings.TrimPrefix(group.FolderPath, "ssh://")
								if slash := strings.IndexByte(rest, '/'); slash > 0 {
									rest = rest[:slash]
								}
								if rest != "" {
									hID = "ssh:" + rest
									hLabel = rest
								}
							}
							hb, ok := hostMap[hID]
							if !ok {
								hb = &hostBucket{ID: hID, Label: hLabel, IsRemote: isRem}
								hostMap[hID] = hb
								hostList = append(hostList, hb)
							}
							hb.Groups = append(hb.Groups, group)
							hb.Sessions += totalCount
						}
						sort.SliceStable(hostList, func(i, j int) bool {
							if hostList[i].IsRemote != hostList[j].IsRemote {
								return !hostList[i].IsRemote // Local always first
							}
							return strings.ToLower(hostList[i].Label) < strings.ToLower(hostList[j].Label)
						})

						renderedAnyGroup := len(hostList) > 0
						for _, hb := range hostList {
							// Orca-style Host Header row: subtle pill [icon] Local Mac [N]
							hostIcon := icons.TablerCpu
							if hb.IsRemote {
								hostIcon = icons.TablerServer
							}
							ui.Row(c.Key("sb-host-"+hb.ID)).Height(24).Radius(6).Padding(0, 8).Gap(6).
								AlignItems(ui.Center).
								Background(tc.Surface.Alpha(0.5)).
								BorderWidth(1).BorderColor(tc.SidebarBorder.Alpha(0.7)).Children(func() {
								ui.Icon(c, hostIcon).Width(12).Height(12).TextColor(tc.SidebarMutedForeground)
								ui.Text(c, hb.Label).FontSize(11.5).FontWeight(600).TextColor(tc.SidebarForeground).SingleLine()
								if hb.Sessions > 0 {
									ui.Row(c).Height(15).Radius(4).Padding(0, 5).AlignItems(ui.Center).
										Background(tc.ToolbarControl).Children(func() {
										ui.Text(c, fmt.Sprintf("%d", hb.Sessions)).FontSize(9.5).FontWeight(600).TextColor(tc.SidebarMutedForeground)
									})
								}
								ui.Box(c).Grow(1)
							})

							for _, group := range hb.Groups {
								totalCount := len(group.Sessions) + len(group.CLISessions)
								isCollapsed := a.CollapsedProjectGroups[group.ProjectName]

								// Project Group Header: chevron + folder + name, with the
								// hover actions (Explore project files / New task / Delete
								// workspace) that forge-ade shows on group hover.
								canExplore := group.ProjectName != "General" && group.FolderPath != ""
								headerActionClicked := false
								pHeader := ui.Row(c.Key("pheader-"+group.ProjectName)).Height(32).Radius(6).
									Padding(0, 6).Gap(6).AlignItems(ui.Center)

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
									isRemote := strings.HasPrefix(group.FolderPath, "ssh://")
									if isCollapsed {
										ui.Icon(c, icons.TablerChevronRight).Width(12).Height(12).TextColor(tc.IconMuted)
									} else {
										ui.Icon(c, icons.TablerChevronDown).Width(12).Height(12).TextColor(tc.IconMuted)
									}
									a.renderProjectGroupIcon(c, tc, group, isCollapsed)

									ui.Text(c, group.ProjectName).FontSize(12).FontWeight(600).Grow(1).MinWidth(0).SingleLine()

									if isRemote && !pHeader.Hovered() {
										ui.Row(c).Height(15).Radius(4).Padding(0, 4).AlignItems(ui.Center).
											Background(tc.ToolbarControl).Children(func() {
											ui.Text(c, "SSH").FontSize(9).FontWeight(600).TextColor(tc.SidebarMutedForeground)
										})
									} else if group.IsCustom && len(group.Folders) > 1 && !pHeader.Hovered() {
										ui.Row(c).Height(15).Radius(4).Padding(0, 4).AlignItems(ui.Center).
											Background(tc.ToolbarControl).Children(func() {
											ui.Text(c, fmt.Sprintf("%d folders", len(group.Folders))).FontSize(9).FontWeight(600).TextColor(tc.SidebarMutedForeground)
										})
									} else if isCollapsed && totalCount > 0 && !pHeader.Hovered() {
										ui.Text(c, fmt.Sprintf("%d", totalCount)).FontSize(10.5).TextColor(tc.SidebarMutedForeground)
									}

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
									if group.IsCustom && len(group.Folders) > 0 {
										for _, f := range group.Folders {
											folder := f
											if m.Item("New task in " + filepath.Base(folder)).Chosen() {
												a.NewAgentTaskInProject(filepath.Base(folder), folder)
											}
										}
									}
									if m.Item("New task with Agent / CLI…").Chosen() {
										if group.ProjectName == "General" || group.FolderPath == "" {
											a.OpenNewTaskModal()
										} else {
											a.OpenNewTaskModalFor(group.FolderPath)
										}
									}
									if group.ProjectName != "General" {
										m.Separator()
										if m.Item("Change project icon…").Chosen() {
											a.openProjectIconModal(group.ProjectName)
										}
									}
									if group.IsCustom {
										if m.Item("Manage folders…").Chosen() {
											a.openEditCustomProjectModal(group.ProjectName)
										}
										if m.Item("Delete custom project").Danger().Chosen() {
											a.deleteCustomProject(group.ProjectName)
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

								// Expanded Project Tasks — non-General projects wrap sessions in an
								// Orca-style Worktree/Branch card showing status dot, primary badge,
								// and branch subtitle.
								if !isCollapsed && totalCount > 0 {
									renderProjectBody := func() {
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
												AgentType:   s.AgentType,
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
											a.renderSessionNestedChildren(c, tc, s)
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
									}

									if group.ProjectName != "General" {
										isPrimaryWs := group.FolderPath != "" && sameWorkspacePath(activeWsPath, group.FolderPath)
										branchName := ""
										if isPrimaryWs && a.Git != nil {
											branchName = a.Git.Snapshot().Branch
										}
										// Orca rule: ONLY the active/open or running workspace group gets
										// the subtle rounded border container; inactive groups stay flat!
										hasActiveOrRunning := false
										wtStatusColor := tc.Success
										for _, s := range group.Sessions {
											if a.Agent != nil && (s.ID == a.Agent.ActiveSessionID || s.ID == a.AgentSplitSessionID) {
												hasActiveOrRunning = true
											}
											// Also check nested workers & handoffs under this session
											for _, child := range a.Agent.WorkersForParent(s.ID) {
												if child.ID == a.Agent.ActiveSessionID || child.ID == a.AgentSplitSessionID || child.Status == "running" {
													hasActiveOrRunning = true
												}
											}
											for _, ho := range a.Agent.HandoffsFromParent(s.ID) {
												if ho.ID == a.Agent.ActiveSessionID || ho.ID == a.AgentSplitSessionID || ho.Status == "running" {
													hasActiveOrRunning = true
												}
											}
											if a.sessionNeedsApproval(s) {
												wtStatusColor = tc.Warning
												hasActiveOrRunning = true
											} else if s.Status == "running" || s.Status == "working" {
												wtStatusColor = tc.Accent
												hasActiveOrRunning = true
											}
										}

										wtCol := ui.Column(c.Key("pwt-"+group.ProjectName)).Padding(4, 4, 4, 6).Gap(2).Radius(8)
										if hasActiveOrRunning {
											wtCol.Background(tc.Surface.Alpha(0.55)).
												BorderWidth(1).BorderColor(tc.SidebarBorder)
										}
										wtCol.Children(func() {
											if branchName != "" || isPrimaryWs {
												ui.Row(c).Padding(2, 6, 3, 6).Gap(6).AlignItems(ui.Center).Children(func() {
													ui.Box(c).Size(6, 6).Radius(3).Background(wtStatusColor).Shrink(0)
													lbl := branchName
													if lbl == "" {
														lbl = "workspace"
													}
													ui.Text(c, lbl).FontSize(11).FontWeight(500).TextColor(tc.SidebarMutedForeground).SingleLine()
													if isPrimaryWs {
														ui.Row(c).Height(15).Radius(4).Padding(0, 5).AlignItems(ui.Center).
															Background(tc.ToolbarControl).Children(func() {
															ui.Text(c, "primary").FontSize(9.5).FontWeight(500).TextColor(tc.SidebarMutedForeground)
														})
													}
													ui.Box(c).Grow(1)
												})
											}
											renderProjectBody()
										})
									} else {
										ui.Column(c.Key("pbody-"+group.ProjectName)).Padding(0, 0, 4, 8).Gap(2).Children(renderProjectBody)
									}
								}
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

			// Minimal Sidebar Utility Row (non-redundant controls only: Settings, Plugins, Rules, Theme)
			// Automations lives in top Quick Nav; Usage & Quota lives in the full-width bottom bar.
			dockBtn := func(key, tip string, icon *ui.SVG, active bool, onPick func()) {
				btn := ui.Row(c.Key("dock-"+key)).Size(24, 24).Radius(5).AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
				btn.Tooltip(tip)
				btn.Children(func() {
					ic := ui.Icon(c, icon).Width(14).Height(14)
					if active {
						ic.TextColor(tc.Accent)
					} else {
						ic.TextColor(tc.IconMuted)
					}
				})
				if active {
					btn.Background(tc.Text.Alpha(0.1))
				} else if btn.Hovered() {
					btn.Background(tc.Text.Alpha(0.06))
				}
				if btn.Clicked() {
					onPick()
				}
			}
			ui.Row(c).Height(32).Padding(0, 10).Gap(4).AlignItems(ui.Center).
				BorderWidth(1, 0, 0, 0).BorderColor(tc.SidebarBorder.Alpha(0.6)).
				Children(func() {
					dockBtn("settings", "Settings (⌘,)", icons.TablerSettings, a.Mode == ModeSettings, func() {
						a.openSettings()
					})
					dockBtn("plugins", "Plugin Marketplace", icons.TablerPuzzle, false, func() { a.openSettings(settings.SettingsTabPlugins) })
					dockBtn("rules", "Rules & Instructions", icons.TablerBook, false, func() { a.OpenRulesModal() })
					ui.Box(c).Grow(1)
					if a.ThemeMgr.Appearance == "dark" {
						dockBtn("theme", "Switch to Light Mode (⌘/)", icons.TablerMoon, false, a.toggleAppearance)
					} else {
						dockBtn("theme", "Switch to Dark Mode (⌘/)", icons.TablerSun, false, a.toggleAppearance)
					}
				})
			_ = t
		})
	})
}

// renderSessionNestedChildren renders Orca-style nested tree children beneath
// a parent session row:
//
//	session
//	  │ -- subagent / hybrid worker
//	  │ -- handoff
func (a *AppUI) renderSessionNestedChildren(c *ui.Context, tc theme.ThemeColors, s *agent.AgentSession) {
	if s == nil || a.Agent == nil {
		return
	}
	subs := a.activeSubagents(s.ID)
	hybridWorkers := a.Agent.WorkersForParent(s.ID)
	handoffs := a.Agent.HandoffsFromParent(s.ID)
	if len(subs) == 0 && len(hybridWorkers) == 0 && len(handoffs) == 0 {
		return
	}
	ui.Column(c.Key("sb-subs-"+s.ID)).Padding(2, 0, 2, 12).Gap(1).
		BorderWidth(0, 0, 0, 1).BorderColor(tc.SidebarBorder).Children(func() {
		for _, sub := range subs {
			if sub == nil {
				continue
			}
			subItem := sub
			subName := subDisplayName(subItem)
			subRole := strings.TrimSpace(string(subItem.RoleFilter))
			wRow := ui.Row(c.Key("sb-sub-"+subItem.ID)).Height(24).Radius(6).Padding(0, 8).Gap(6).
				AlignItems(ui.Center).Cursor(ui.CursorPointer)
			wRow.Children(func() {
				ui.Box(c).Size(6, 6).Radius(3).Background(tc.Accent).Shrink(0)
				ui.Text(c, subName).FontSize(11.5).FontWeight(500).TextColor(tc.SidebarForeground).Grow(1).MinWidth(0).SingleLine()
				if subRole != "" {
					ui.Text(c, subRole).FontSize(10).TextColor(tc.SidebarMutedForeground).SingleLine()
				}
			})
			if wRow.Hovered() {
				wRow.Background(tc.SidebarRowHover)
			}
			if wRow.Clicked() {
				a.selectAgentSession(s)
				a.RightDrawerTab = RightTabSubagent
				a.ShowRightDrawer = true
			}
		}
		for _, hw := range hybridWorkers {
			if hw == nil {
				continue
			}
			worker := hw
			wName := worker.WorkerName
			if wName == "" {
				wName = worker.Title
			}
			wActive := a.ActiveTaskKind == "agent" && (worker.ID == a.Agent.ActiveSessionID || worker.ID == a.AgentSplitSessionID)
			dotColor := tc.Success
			if a.sessionNeedsApproval(worker) {
				dotColor = tc.Warning
			} else if worker.Status == "running" {
				dotColor = tc.Accent
			} else if worker.Status == "failed" {
				dotColor = tc.Error
			}
			wRow := ui.Row(c.Key("sb-hw-"+worker.ID)).Height(24).Radius(6).Padding(0, 7).Gap(6).
				AlignItems(ui.Center).Cursor(ui.CursorPointer)
			if wActive {
				wRow.Background(tc.SidebarRowActive)
			} else if wRow.Hovered() {
				wRow.Background(tc.SidebarRowHover)
			}
			wRow.Children(func() {
				ui.Box(c).Size(6, 6).Radius(3).Background(dotColor).Shrink(0)
				if worker.AgentType == "internal" {
					ui.Icon(c, icons.ProviderForgeADE).Width(11).Height(11)
				} else {
					icons.RenderProviderIcon(c, worker.AgentID+" "+worker.Model, 11)
				}
				ui.Text(c, wName).FontSize(11.5).FontWeight(500).TextColor(tc.SidebarForeground).Grow(1).MinWidth(0).SingleLine()
				if mLbl := shortSidebarModel(worker.Model); mLbl != "" {
					ui.Text(c, mLbl).FontSize(9.5).TextColor(tc.SidebarMutedForeground).SingleLine()
				}
			})
			if wRow.Clicked() {
				if a.Agent != nil && a.Agent.ActiveSessionID == s.ID {
					a.openSessionInSplit(worker.ID)
				} else {
					a.selectAgentSession(worker)
				}
			}
		}
		for _, ho := range handoffs {
			if ho == nil {
				continue
			}
			hs := ho
			hActive := a.ActiveTaskKind == "agent" && (hs.ID == a.Agent.ActiveSessionID || hs.ID == a.AgentSplitSessionID)
			dotColor := tc.Success
			if a.sessionNeedsApproval(hs) {
				dotColor = tc.Warning
			} else if hs.Status == "running" {
				dotColor = tc.Accent
			}
			hRow := ui.Row(c.Key("sb-ho-"+hs.ID)).Height(24).Radius(6).Padding(0, 7).Gap(5).
				AlignItems(ui.Center).Cursor(ui.CursorPointer)
			if hActive {
				hRow.Background(tc.SidebarRowActive)
			} else if hRow.Hovered() {
				hRow.Background(tc.SidebarRowHover)
			}
			hRow.Children(func() {
				ui.Icon(c, icons.TablerGitFork).Width(10).Height(10).TextColor(tc.SidebarMutedForeground)
				ui.Box(c).Size(5, 5).Radius(2.5).Background(dotColor).Shrink(0)
				if hs.AgentType == "internal" || strings.Contains(strings.ToLower(hs.AgentID), "internal") {
					ui.Icon(c, icons.ProviderForgeADE).Width(11).Height(11)
				} else {
					icons.RenderProviderIcon(c, hs.AgentID+" "+hs.Model, 11)
				}
				ui.Text(c, hs.Title).FontSize(11.5).FontWeight(500).TextColor(tc.SidebarForeground).Grow(1).MinWidth(0).SingleLine()
				if mLbl := shortSidebarModel(hs.Model); mLbl != "" {
					ui.Text(c, mLbl).FontSize(9.5).TextColor(tc.SidebarMutedForeground).SingleLine()
				}
			})
			if hRow.Clicked() {
				a.selectAgentSession(hs)
			}
			hRow.ContextMenu(func(m *ui.Menu) {
				if m.Item("Open in Split Pane").Chosen() {
					a.openSessionInSplit(hs.ID)
				}
				if m.Item("Delete").Danger().Chosen() {
					a.Agent.DeleteSession(hs.ID)
				}
			})
		}
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
	IsCustom    bool
	Folders     []string
	ColorIdx    int
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

	folderToCustom := make(map[string]string)
	for _, cp := range a.CustomProjects {
		if strings.TrimSpace(cp.Name) == "" {
			continue
		}
		for _, f := range cp.Folders {
			nf := agentsession.NormalizeWorkspacePath(f)
			if nf != "" {
				folderToCustom[nf] = cp.Name
			}
		}
	}
	resolveGroupName := func(wsPath string) string {
		if cpName, ok := folderToCustom[wsPath]; ok {
			return cpName
		}
		return resolveProjectName(wsPath)
	}
	activeProjectName := resolveGroupName(activeWsPath)

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

	// Always seed General, custom projects, and the active workspace.
	getOrCreateGroup("General", "")
	for _, cp := range a.CustomProjects {
		if strings.TrimSpace(cp.Name) == "" {
			continue
		}
		firstFolder := ""
		if len(cp.Folders) > 0 {
			firstFolder = cp.Folders[0]
		}
		g := getOrCreateGroup(cp.Name, firstFolder)
		g.IsCustom = true
		g.Folders = append([]string(nil), cp.Folders...)
	}
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
				if !a.isExistingDirCached(normRP) {
					continue
				}
			}
			getOrCreateGroup(resolveGroupName(normRP), normRP)
		}
	}

	// 1. Group filtered ACP/Internal Sessions (skip Bot-owned sessions on the
	// default Active view without search so they live solely under Bots).
	if a.Agent != nil {
		for _, s := range a.Agent.FilteredSessions() {
			if s == nil {
				continue
			}
			if tab == "active" && searchQuery == "" && s.BotID != "" {
				continue
			}
			if tab == "active" && searchQuery == "" && s.ParentSessionID != "" {
				continue
			}
			if tab == "active" && searchQuery == "" && s.HandoffFromID != "" && a.Agent.HasSessionID(s.HandoffFromID) {
				continue
			}
			wsPath := s.WorkspacePath
			if !strings.HasPrefix(wsPath, "ssh://") {
				wsPath = agentsession.NormalizeWorkspacePath(wsPath)
			}
			pName := resolveGroupName(wsPath)
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
			pName := resolveGroupName(wsPath)
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
	const paletteSize = 16
	usedColors := make([]bool, paletteSize)
	for _, name := range names {
		g := groupMap[name]
		idx := hashProjectName(name) % paletteSize
		for step := 0; step < paletteSize; step++ {
			cand := (idx + step) % paletteSize
			if !usedColors[cand] {
				usedColors[cand] = true
				idx = cand
				break
			}
		}
		g.ColorIdx = idx
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
	// Drop/chevron handlers run inside the MyGo build pass: never write disk
	// here (concurrent with the ticker's saver, and it blocks the frame).
	// Mark dirty — the existing ticker goroutine persists it.
	a.prefsDirty.Store(true)
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
			if !a.isExistingDirCached(normRP) {
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

// botLiveCounts returns the number of running tasks/chats and pending approvals for bot.
func (a *AppUI) botLiveCounts(bot *agent.BotProfile) (workingCnt, needsCnt int) {
	if a.Agent == nil || bot == nil {
		return 0, 0
	}
	homeRunning := false
	if bot.HomeSessionID != "" {
		if home := a.Agent.FindSession(bot.HomeSessionID); home != nil {
			if a.sessionNeedsApproval(home) {
				needsCnt++
			} else if home.Status == "running" {
				homeRunning = true
			}
		}
	}
	hasTurnRunning := false
	for _, tk := range a.Agent.BotTasks(a.Bots, bot.ID) {
		if tk.Status == "needs_approval" {
			needsCnt++
		} else if tk.Status == "running" {
			workingCnt++
			if strings.Contains(tk.SessionID, "#turn-") {
				hasTurnRunning = true
			}
		}
	}
	if homeRunning && !hasTurnRunning {
		workingCnt++
	}
	return workingCnt, needsCnt
}

// renderCollapsedLeftRail renders a compact 48px icon rail when the left
// sidebar is collapsed, showing project icons with a round pulsing indicator
// whenever a project has a running agent or CLI session.
func (a *AppUI) renderCollapsedLeftRail(c *ui.Context) {
	tc := a.ThemeMgr.Colors()

	ui.Column(c.Key("collapsed-left-rail")).Width(52).FillHeight().Shrink(0).
		Padding(10, 0, 8, 0).Gap(6).AlignItems(ui.Center).
		Background(tc.Sidebar).BorderWidth(0, 1, 0, 0).BorderColor(tc.SidebarBorder).
		Children(func() {
			// Project icons with running pulse badge
			activeWs := ""
			if a.Workspace != nil {
				activeWs = a.Workspace.RootDir
			}
			activeProj := resolveProjectName(activeWs)
			groups := a.sidebarProjectGroups()

			ui.Scroll(c.Key("rail-projects")).Grow(1).FillWidth().Scrollbars(false).Children(func() {
				ui.Column(c).FillWidth().AlignItems(ui.Center).Gap(6).Padding(2, 0).Children(func() {
					if a.Bots != nil {
						bots := a.Bots.List()
						if len(bots) > 0 {
							for _, b := range bots {
								bot := b
								botActive := a.Agent != nil && a.Agent.ActiveSession() != nil && a.Agent.ActiveSession().BotID == bot.ID
								workingCnt, needsCnt := a.botLiveCounts(bot)
								tip := bot.Name
								if needsCnt > 0 {
									tip += fmt.Sprintf(" · %d needs approval", needsCnt)
								} else if workingCnt > 0 {
									tip += fmt.Sprintf(" · %d working", workingCnt)
								} else if bot.Paused {
									tip += " · Paused"
								}
								bTile := ui.Box(c.Key("rail-bot-"+bot.ID)).Size(34, 34).Radius(9).
									AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer).Tooltip(tip)
								if botActive {
									bTile.Background(tc.SidebarRowActive).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.5))
								} else if bTile.Hovered() {
									bTile.Background(tc.SidebarRowHover)
								}
								if bot.Paused {
									bTile.Opacity(0.55)
								}
								bTile.Children(func() {
									a.renderBotAvatar(c, tc, bot, 28, 8)
									if needsCnt > 0 {
										ui.Box(c).Absolute().Top(1).Right(1).Size(8, 8).Radius(4).Background(tc.Warning)
									} else if workingCnt > 0 {
										p := shimmerPhase()
										t := p
										if t > 0.5 {
											t = 1 - t
										}
										pulseAlpha := float32(0.18 + 0.42*(t*2))
										ui.Box(c).Absolute().Top(1).Right(1).Size(10, 10).Radius(5).
											Background(tc.Accent.Alpha(pulseAlpha)).
											AlignItems(ui.Center).Justify(ui.Center).Children(func() {
											ui.Box(c).Size(6, 6).Radius(3).Background(tc.Accent)
										})
									}
								})
								if bTile.Clicked() && a.Agent != nil {
									if home := a.Agent.OpenBotHomeSession(a.Bots, bot); home != nil {
										a.ensureAgentTab(home.ID)
									}
									a.IsGitClientOpen = false
									a.AgentViewMode = AgentViewChat
									a.ActiveTaskKind = "agent"
									a.ShowRightDrawer = true
									a.RightDrawerTab = RightTabBot
									if a.BotDrawerSubTab == "" {
										a.BotDrawerSubTab = "tasks"
									}
								}
							}
							ui.Box(c).Width(24).Height(1).Background(tc.SidebarBorder.Alpha(0.7))
						}
					}

					for _, g := range groups {
						group := g
						if group == nil || (group.ProjectName == "General" && len(group.Sessions) == 0 && len(group.CLISessions) == 0) {
							continue
						}
						hasRunning := false
						var runningSess *agent.AgentSession
						for _, s := range group.Sessions {
							if s == nil {
								continue
							}
							isActiveStream := a.Agent != nil && a.Agent.ActiveSessionID == s.ID && a.Agent.IsStreaming
							if isActiveStream || s.Status == "running" || s.Status == "working" {
								hasRunning = true
								if runningSess == nil {
									runningSess = s
								}
							}
						}
						if !hasRunning {
							for _, cs := range group.CLISessions {
								if cs != nil && cs.Status == "running" && cs.Terminal != nil && !cs.Terminal.IsExited() {
									hasRunning = true
									break
								}
							}
						}

						isActiveProj := group.ProjectName == activeProj
						tip := group.ProjectName
						if hasRunning {
							tip += " · Agent running"
						}
						tile := ui.Box(c.Key("rail-proj-"+group.ProjectName)).Size(34, 34).Radius(8).
							AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer).Tooltip(tip)
						if isActiveProj {
							tile.Background(tc.SidebarRowActive).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.5))
						} else if tile.Hovered() {
							tile.Background(tc.SidebarRowHover)
						}
						tile.Children(func() {
							a.renderProjectGroupIcon(c, tc, group, false)
							if hasRunning {
								p := shimmerPhase()
								t := p
								if t > 0.5 {
									t = 1 - t
								}
								pulseAlpha := float32(0.18 + 0.42*(t*2))
								// Outer pulsing halo + inner solid round indicator
								ui.Box(c).Absolute().Top(1).Right(1).Size(10, 10).Radius(5).
									Background(tc.Accent.Alpha(pulseAlpha)).
									AlignItems(ui.Center).Justify(ui.Center).Children(func() {
									ui.Box(c).Size(6, 6).Radius(3).Background(tc.Accent)
								})
							}
						})
						if tile.Clicked() {
							if runningSess != nil {
								a.selectAgentSession(runningSess)
							} else if len(group.Sessions) > 0 && group.Sessions[0] != nil {
								a.selectAgentSession(group.Sessions[0])
							} else if group.FolderPath != "" {
								a.switchWorkspace(group.FolderPath)
							}
						}
					}
				})
			})

			ui.Box(c).Width(24).Height(1).Background(tc.SidebarBorder.Alpha(0.7))
			if IconButton(c, tc, icons.TablerPlus, 16).Tooltip("New Task (⌘N)").Clicked() {
				a.handleNewTask()
			}
			if IconButton(c, tc, icons.TablerSettings, 16).Tooltip("Settings (⌘,)").Clicked() {
				a.openSettings(0)
			}
		})
}
