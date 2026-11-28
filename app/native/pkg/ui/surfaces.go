package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"
	mygoeditor "github.com/egoist/mygo/ui/editor"
	"github.com/hasdev/forge-ade/app/native/pkg/git"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
	"github.com/hasdev/forge-ade/core/board"
	internalgit "github.com/hasdev/forge-ade/core/git"
)

// Full-window surfaces opened from the sidebar dock, as forge-ade opens the
// Workflow Board and the Git Client as the agent area's main view.

// Agent area view modes.
const (
	AgentViewChat  = "chat"
	AgentViewBoard = "board"
	AgentViewGit   = "git"
)

// openBoardSurface switches the agent area to the workflow board.
func (a *AppUI) openBoardSurface() {
	a.setMode(ModeAgent)
	a.AgentViewMode = AgentViewBoard
	a.RefreshBoard()
}

// openGitClientSurface switches the agent area or editor to the git client.
func (a *AppUI) openGitClientSurface() {
	if a.Mode == ModeEditor {
		a.IsGitClientOpen = true
	} else {
		a.setMode(ModeAgent)
		a.AgentViewMode = AgentViewGit
	}
	if a.Git != nil {
		a.Git.RefreshAsync()
	}
	a.loadGitRemoteURL()
}

// renderAgentView routes the agent area's main view, mirroring forge-ade's
// AgentContainer (chat / WorkflowBoardView / GitClientView).
func (a *AppUI) renderAgentView(c *ui.Context) {
	switch a.AgentViewMode {
	case AgentViewBoard:
		a.renderBoardSurface(c)
	case AgentViewGit:
		a.renderGitClientSurface(c)
	default:
		a.renderAgentChat(c)
	}
}

// ---------------------------------------------------------------------------
// Agent Workflow Board
// ---------------------------------------------------------------------------

// renderBoardSurface renders the full-width Agent Workflow Board kanban.
func (a *AppUI) renderBoardSurface(c *ui.Context) {
	tc := a.ThemeMgr.Colors()
	if a.Board == nil {
		a.RefreshBoard()
	}

	// Header: icon, title + Kanban badge, subtitle; toolbar row below.
	ui.Column(c).Fill().Background(tc.Canvas).Children(func() {
		ui.Row(c).Height(64).Padding(0, 24).Gap(14).AlignItems(ui.Center).Children(func() {
			tile := ui.Box(c).Size(40, 40).Radius(10).AlignItems(ui.Center).Justify(ui.Center).
				Background(tc.Accent.Alpha(0.12)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.25))
			tile.Children(func() {
				ui.Icon(c, icons.TablerLayoutKanban).Width(20).Height(20).TextColor(tc.Accent)
			})
			ui.Column(c).Gap(1).Children(func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "Agent Workflow Board").FontSize(16).FontWeight(700)
					Badge(c, tc, "Kanban", tc.TextMuted)
				})
				ui.Text(c, "Automated task pipeline orchestrated with Pandu-Micro & Agent Harness").FontSize(11).TextColor(tc.TextMuted)
			})
		})

		// Toolbar: search, Board/List toggle, refresh, + New Task
		ui.Row(c).Height(44).Padding(0, 24).Gap(8).AlignItems(ui.Center).
			BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
			ui.Row(c).Width(260).Height(30).Radius(6).Padding(0, 8).Gap(6).AlignItems(ui.Center).
				Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).Children(func() {
				ui.Icon(c, icons.TablerSearch).Width(12).Height(12).TextColor(tc.IconMuted)
				ui.TextInput(c, &a.BoardSearchQuery).Placeholder("Search tasks...").Grow(1)
			})

			seg := ui.Row(c).Height(30).Radius(6).Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).Gap(2).Padding(2)
			seg.Children(func() {
				segBtn := func(label string, icon *ui.SVG, active bool, onPick func()) {
					b := ui.Row(c).Height(24).Radius(4).Padding(0, 10).Gap(5).AlignItems(ui.Center).Justify(ui.Center)
					icColor, txtColor := tc.IconMuted, tc.Text
					weight := 500
					if active {
						b.Background(tc.Accent)
						icColor, txtColor = tc.AccentForeground, tc.AccentForeground
						weight = 600
					}
					b.Children(func() {
						ui.Icon(c, icon).Width(12).Height(12).TextColor(icColor)
						ui.Text(c, label).FontSize(11).FontWeight(weight).TextColor(txtColor)
					})
					if b.Clicked() {
						onPick()
					}
				}
				segBtn("Board", icons.TablerLayoutKanban, a.BoardViewMode != "list", func() { a.BoardViewMode = "" })
				segBtn("List", icons.TablerList, a.BoardViewMode == "list", func() { a.BoardViewMode = "list" })
			})

			refreshBtn := ui.Row(c).Size(30, 30).Radius(6).AlignItems(ui.Center).Justify(ui.Center).
				Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
			refreshBtn.Children(func() {
				ui.Icon(c, icons.TablerRefresh).Width(13).Height(13).TextColor(tc.IconMuted)
			})
			if refreshBtn.Hovered() {
				refreshBtn.Background(tc.Text.Alpha(0.05))
			}
			if refreshBtn.Clicked() {
				a.RefreshBoard()
			}

			ui.Box(c).Grow(1)

			newBtn := ui.Row(c).Height(30).Radius(6).Padding(0, 12).Gap(5).AlignItems(ui.Center).Justify(ui.Center).
				Background(tc.Accent)
			newBtn.Children(func() {
				ui.Icon(c, icons.TablerPlus).Width(13).Height(13).TextColor(tc.AccentForeground)
				ui.Text(c, "New Task").FontSize(12).FontWeight(600).TextColor(tc.AccentForeground)
			})
			if newBtn.Hovered() {
				newBtn.Background(tc.MessageActionHover)
			}
			if newBtn.Clicked() {
				a.NewBoardTaskTitle = ""
				a.NewBoardTaskDesc = ""
				a.NewBoardTaskPriority = "medium"
				a.NewBoardTaskStatus = "todo"
				a.ShowNewBoardTaskModal = true
			}

			// Close (X), always the last control of the toolbar
			closeBtn := ui.Row(c).Size(30, 30).Radius(6).AlignItems(ui.Center).Justify(ui.Center).
				Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
			closeBtn.Children(func() {
				ui.Icon(c, icons.TablerX).Width(13).Height(13).TextColor(tc.IconMuted)
			})
			if closeBtn.Hovered() {
				closeBtn.Background(tc.Text.Alpha(0.05))
			}
			if closeBtn.Clicked() {
				a.AgentViewMode = AgentViewChat
			}
		})

		// Body
		if a.Board == nil || a.Board.Data == nil || len(a.Board.Data.Columns) == 0 {
			ui.Column(c).Grow(1).Center().Gap(8).Children(func() {
				ui.Icon(c, icons.TablerLayoutKanban).Width(36).Height(36).TextColor(tc.IconMuted)
				ui.Text(c, "No board tasks").FontSize(14).FontWeight(600).TextColor(tc.TextMuted)
				ui.Text(c, "Tasks in .forge/board_tasks.json appear here.").FontSize(12).TextColor(tc.TextMuted)
				emptyNewBtn := ui.Row(c).Padding(6, 12).Radius(6).Background(tc.Accent).Gap(5).AlignItems(ui.Center).Cursor(ui.CursorPointer)
				emptyNewBtn.Children(func() {
					ui.Icon(c, icons.TablerPlus).Width(13).Height(13).TextColor(tc.AccentForeground)
					ui.Text(c, "New Task").FontSize(12).FontWeight(500).TextColor(tc.AccentForeground)
				})
				if emptyNewBtn.Clicked() {
					a.NewBoardTaskTitle = ""
					a.NewBoardTaskDesc = ""
					a.NewBoardTaskPriority = "medium"
					a.NewBoardTaskStatus = "todo"
					a.ShowNewBoardTaskModal = true
				}
			})
			return
		}

		if a.BoardViewMode == "list" {
			a.renderBoardList(c, tc)
		} else {
			a.renderBoardColumns(c, tc)
		}
	})

}

// renderBoardColumns draws the five-lane kanban with per-column create.
func (a *AppUI) renderBoardColumns(c *ui.Context, tc theme.ThemeColors) {
	query := strings.ToLower(strings.TrimSpace(a.BoardSearchQuery))

	ui.ScrollHorizontal(c).Grow(1).Padding(16, 24).Gap(14).AlignItems(ui.Stretch).Children(func() {
		for ci, col := range a.Board.Data.Columns {
			ci := ci
			tasks := col.Tasks
			if query != "" {
				filtered := make([]*board.Task, 0, len(tasks))
				for _, t := range tasks {
					if strings.Contains(strings.ToLower(t.Title), query) {
						filtered = append(filtered, t)
					}
				}
				tasks = filtered
			}

			ui.Column(c).Width(300).Radius(12).Background(tc.Surface.Alpha(0.5)).
				BorderWidth(1).BorderColor(tc.Border.Alpha(0.6)).Gap(0).Clip().Children(func() {
				// Column header: name + count + create
				ui.Row(c).Height(44).Padding(0, 12).Gap(6).AlignItems(ui.Center).
					BorderWidth(0, 0, 1, 0).BorderColor(tc.Border.Alpha(0.5)).Children(func() {
					colIcon := icons.TablerCircle
					colColor := tc.TextMuted
					switch ci {
					case 1:
						colIcon, colColor = icons.TablerClock, tc.Warning
					case 2:
						colIcon, colColor = icons.TablerEye, tc.Accent
					case 3:
						colIcon, colColor = icons.TablerFlask, tc.Error
					case 4:
						colIcon, colColor = icons.TablerCircleCheck, tc.Success
					}
					ui.Icon(c, colIcon).Width(14).Height(14).TextColor(colColor)
					ui.Text(c, col.Name).FontSize(12).FontWeight(600).Grow(1)
					ui.Box(c).Size(20, 16).Radius(8).AlignItems(ui.Center).Justify(ui.Center).
						Background(tc.Text.Alpha(0.08)).Children(func() {
						ui.Text(c, fmt.Sprintf("%d", len(tasks))).FontSize(10).TextColor(tc.TextMuted)
					})
					addBtn := ui.Row(c).Size(20, 20).Radius(4).AlignItems(ui.Center).Justify(ui.Center)
					addBtn.Children(func() {
						ui.Icon(c, icons.TablerPlus).Width(12).Height(12).TextColor(tc.IconMuted)
					})
					if addBtn.Hovered() {
						addBtn.Background(tc.Text.Alpha(0.06))
					}
					if addBtn.Clicked() {
						a.NewBoardTaskTitle = ""
						a.NewBoardTaskDesc = ""
						a.NewBoardTaskPriority = "medium"
						a.NewBoardTaskStatus = col.Status
						a.ShowNewBoardTaskModal = true
					}
				})

				// Cards
				ui.Scroll(c).Grow(1).Children(func() {
					ui.Column(c).Padding(8).Gap(8).Children(func() {
						if len(tasks) == 0 {
							empty := ui.Column(c).Grow(1).Center().Gap(4).Padding(16).
								Radius(8).BorderWidth(1).BorderColor(tc.Border.Alpha(0.4))
							empty.Children(func() {
								ui.Text(c, "No tasks").FontSize(11).TextColor(tc.TextMuted)
								create := ui.Row(c).Gap(2).AlignItems(ui.Center)
								create.Children(func() {
									ui.Text(c, "+ Create").FontSize(11).FontWeight(500).TextColor(tc.Accent)
								})
								if create.Clicked() {
									a.NewBoardTaskTitle = ""
									a.NewBoardTaskDesc = ""
									a.NewBoardTaskPriority = "medium"
									a.NewBoardTaskStatus = col.Status
									a.ShowNewBoardTaskModal = true
								}
							})
							_ = empty
							return
						}
						for _, task := range tasks {
							t := task
							card := ui.Column(c.Key("bcard-"+t.ID)).Radius(8).Padding(10, 12).Gap(6).
								Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
							card.Children(func() {
								ui.Text(c, t.Title).FontSize(12).Grow(1)
								ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
									if t.Key != "" {
										ui.Text(c, t.Key).FontSize(10).FontWeight(600).TextColor(tc.Accent)
									}
									if t.Priority != "" {
										Badge(c, tc, t.Priority, tc.Warning)
									}
									ui.Box(c).Grow(1)
									if t.Branch != "" {
										ui.Row(c).Gap(3).AlignItems(ui.Center).Children(func() {
											ui.Icon(c, icons.TablerGitBranch).Width(10).Height(10).TextColor(tc.IconMuted)
											ui.Text(c, t.Branch).FontSize(10).TextColor(tc.TextMuted).MaxWidth(90).SingleLine()
										})
									}
								})
							})
							// Move + delete actions
							card.ContextMenu(func(m *ui.Menu) {
								if ci > 0 {
									if m.Item(fmt.Sprintf("Move to %s", a.Board.Data.Columns[ci-1].Name)).Chosen() {
										go func() {
											_ = a.Board.MoveTaskStatus(t.ID, a.Board.Data.Columns[ci-1].Status)
											a.RefreshBoard()
											if a.Window != nil {
												a.Window.Invalidate()
											}
										}()
									}
								}
								if ci < len(a.Board.Data.Columns)-1 {
									if m.Item(fmt.Sprintf("Move to %s", a.Board.Data.Columns[ci+1].Name)).Chosen() {
										go func() {
											_ = a.Board.MoveTaskStatus(t.ID, a.Board.Data.Columns[ci+1].Status)
											a.RefreshBoard()
											if a.Window != nil {
												a.Window.Invalidate()
											}
										}()
									}
								}
								m.Separator()
								if m.Item("Delete").Danger().Chosen() {
									go func() {
										_ = a.Board.DeleteTask(t.ID)
										a.RefreshBoard()
										if a.Window != nil {
											a.Window.Invalidate()
										}
									}()
								}
							})
						}
					})
				})
			})
		}
	})
}

// renderBoardList draws the list view of all tasks.
func (a *AppUI) renderBoardList(c *ui.Context, tc theme.ThemeColors) {
	ui.Scroll(c).Grow(1).Padding(12, 24).Children(func() {
		ui.Column(c).Gap(4).Children(func() {
			for _, col := range a.Board.Data.Columns {
				for _, t := range col.Tasks {
					if a.BoardSearchQuery != "" && !strings.Contains(strings.ToLower(t.Title), strings.ToLower(a.BoardSearchQuery)) {
						continue
					}
					t := t
					row := ui.Row(c.Key("blist-"+t.ID)).Height(36).Radius(6).Padding(0, 10).Gap(8).AlignItems(ui.Center).
						Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border.Alpha(0.5))
					row.Children(func() {
						ui.Text(c, col.Name).FontSize(10).FontWeight(600).TextColor(tc.TextMuted).Width(90)
						if t.Key != "" {
							ui.Text(c, t.Key).FontSize(10).FontWeight(600).TextColor(tc.Accent)
						}
						ui.Text(c, t.Title).FontSize(12).Grow(1).SingleLine()
						if t.Priority != "" {
							Badge(c, tc, t.Priority, tc.Warning)
						}
					})
					if row.Hovered() {
						row.Background(tc.Text.Alpha(0.04))
					}
				}
			}
		})
	})
}

// ---------------------------------------------------------------------------
// Git Client
// ---------------------------------------------------------------------------

// renderGitClientSurface renders forge-ade's full GitClientView: toolbar with
// branch + remote ops, and Changes / History / Branches panes.
func (a *AppUI) renderGitClientSurface(c *ui.Context) {
	gm := a.Git
	if gm == nil {
		return
	}
	tc := a.ThemeMgr.Colors()
	// Render from a point-in-time snapshot: Refresh/Fetch/Pull/Push goroutines
	// write the manager's exported fields under its lock.
	snap := gm.Snapshot()

	ui.Column(c).Fill().Background(tc.Canvas).Children(func() {
		// Toolbar
		ui.Row(c).Height(52).Padding(0, 16).Gap(8).AlignItems(ui.Center).
			BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
			if snap.Branch != "" {
				branchPill := ui.Row(c).Height(30).Radius(8).Padding(0, 10).Gap(6).AlignItems(ui.Center).
					Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).Cursor(ui.CursorPointer).Tooltip("Switch Git Branch")
				branchPill.Children(func() {
					ui.Icon(c, icons.TablerGitBranch).Width(13).Height(13).TextColor(tc.Accent)
					ui.Text(c, snap.Branch).FontSize(12).FontWeight(600)
					ui.Icon(c, icons.TablerChevronDown).Width(11).Height(11).TextColor(tc.IconMuted)
				})
				if branchPill.Hovered() {
					branchPill.Background(tc.SurfaceRaised)
				}
				if branchPill.Clicked() {
					a.ShowBranchModal = true
				}
			}

			prLabel := "Pull Requests"
			if src, err := gm.RemoteSource(); err == nil && src != nil && src.IsGitLab {
				prLabel = "Merge Requests"
			}
			tabs := []struct {
				Tab  int
				Name string
			}{
				{git.GitTabChanges, fmt.Sprintf("Changes (%d)", len(snap.Staged)+len(snap.Unstaged)+len(snap.Untracked))},
				{git.GitTabHistory, "History"},
				{git.GitTabBranches, "Branches"},
				{git.GitTabPRs, prLabel},
				{git.GitTabWorktrees, "Worktrees"},
			}
			for _, tab := range tabs {
				t := tab
				active := gm.ActiveTab == t.Tab
				b := ui.Row(c.Key(fmt.Sprintf("gctab-%d", t.Tab))).Height(30).Radius(8).Padding(0, 12).
					AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
				b.Children(func() {
					txt := ui.Text(c, t.Name).FontSize(12)
					if active {
						txt.FontWeight(600).TextColor(tc.Accent)
					} else {
						txt.FontWeight(500).TextColor(tc.Text)
					}
				})
				if active {
					b.Background(tc.Accent.Alpha(0.15)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.3))
				} else if b.Hovered() {
					b.Background(tc.Text.Alpha(0.05))
				}
				if b.Clicked() {
					gm.ActiveTab = t.Tab
				}
			}

			ui.Box(c).Grow(1)

			// Remote badge
			if a.GitRemoteURL != "" {
				host := a.GitRemoteURL
				if i := strings.Index(host, "://"); i != -1 {
					host = host[i+3:]
				}
				if i := strings.IndexAny(host, "/:"); i != -1 {
					host = host[:i]
				}
				ui.Row(c).Height(26).Radius(999).Padding(0, 10).Gap(5).AlignItems(ui.Center).
					Background(tc.Success.Alpha(0.1)).BorderWidth(1).BorderColor(tc.Success.Alpha(0.3)).Children(func() {
					ui.Box(c).Size(6, 6).Radius(3).Background(tc.Success)
					ui.Text(c, host).FontSize(11).TextColor(tc.Success)
				})
			}

			remoteBtn := func(icon *ui.SVG, label string, busy bool, run func() error) {
				b := ui.Row(c).Height(30).Radius(8).Padding(0, 12).Gap(5).AlignItems(ui.Center).
					Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).Cursor(ui.CursorPointer)
				b.Children(func() {
					if busy {
						ui.Spinner(c).Size(12, 12).Label(label)
					} else {
						ui.Icon(c, icon).Width(13).Height(13).TextColor(tc.IconMuted)
					}
					ui.Text(c, label).FontSize(12).FontWeight(500)
				})
				if b.Hovered() {
					b.Background(tc.SurfaceRaised)
				}
				if b.Clicked() && !busy {
					go func() {
						if err := run(); err != nil {
							a.queueToast(ui.Toast{Title: label + " Failed", Description: err.Error(), Type: "error"})
						} else {
							a.queueToast(ui.Toast{Title: label + " Completed", Type: "success"})
						}
					}()
				}
			}
			remoteBtn(icons.TablerArrowDown, "Fetch Origin", snap.IsFetching, func() error { return gm.Fetch() })
			remoteBtn(icons.TablerArrowDown, "Pull", snap.IsPulling, func() error { return gm.Pull() })
			remoteBtn(icons.TablerArrowUp, "Push", snap.IsPushing, func() error { return gm.Push() })

			refreshBtn := ui.Row(c).Size(30, 30).Radius(8).AlignItems(ui.Center).Justify(ui.Center).
				Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).Cursor(ui.CursorPointer).Tooltip("Refresh Git Status")
			refreshBtn.Children(func() {
				ui.Icon(c, icons.TablerRefresh).Width(13).Height(13).TextColor(tc.IconMuted)
			})
			if refreshBtn.Hovered() {
				refreshBtn.Background(tc.SurfaceRaised)
			}
			if refreshBtn.Clicked() {
				gm.RefreshAsync()
				a.loadGitRemoteURL()
			}

			closeBtn := ui.Row(c).Size(30, 30).Radius(8).AlignItems(ui.Center).Justify(ui.Center).
				Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).Cursor(ui.CursorPointer).Tooltip("Close Git Client")
			closeBtn.Children(func() {
				ui.Icon(c, icons.TablerX).Width(13).Height(13).TextColor(tc.IconMuted)
			})
			if closeBtn.Hovered() {
				closeBtn.Background(tc.Text.Alpha(0.05))
			}
			if closeBtn.Clicked() {
				if a.Mode == ModeEditor {
					a.IsGitClientOpen = false
				} else {
					a.AgentViewMode = AgentViewChat
				}
			}
		})

		// Body per tab
		switch gm.ActiveTab {
		case git.GitTabHistory:
			a.renderGitHistorySurface(c, tc, gm)
		case git.GitTabBranches:
			a.renderGitBranchesSurface(c, tc, gm)
		case git.GitTabPRs:
			a.renderGitPRsSurface(c, tc, gm)
		case git.GitTabWorktrees:
			a.renderGitWorktreesSurface(c, tc, gm)
		default:
			a.renderGitChangesSurface(c, tc, gm)
		}
	})
}

// renderGitChangesSurface renders a full-width two-column working-tree view:
// file list + commit form on the left, wide diff viewer (Split/Unified) on the right.
func (a *AppUI) renderGitChangesSurface(c *ui.Context, tc theme.ThemeColors, gm *git.GitManager) {
	if a.Workspace.RootDir == "" {
		ui.Column(c).Grow(1).Center().Gap(8).Children(func() {
			ui.Icon(c, icons.TablerFolder).Width(24).Height(24).TextColor(tc.IconMuted)
			ui.Text(c, "No folder opened").FontSize(12.5).TextColor(tc.TextMuted)
		})
		return
	}

	snap := gm.Snapshot()
	conflicts := snap.Conflicts
	staged := snap.Staged
	unstaged := append(append([]internalgit.FileStatus(nil), snap.Unstaged...), snap.Untracked...)
	totalFiles := len(conflicts) + len(staged) + len(unstaged)

	if totalFiles == 0 {
		ui.Column(c).Grow(1).Center().Gap(8).Padding(24).Children(func() {
			ui.Icon(c, icons.TablerFileDiff).Width(34).Height(34).TextColor(tc.IconMuted)
			ui.Text(c, "Working tree is clean").FontSize(14).FontWeight(600).TextColor(tc.Text)
			ui.Text(c, "Modified, staged, and untracked files will appear here.").FontSize(12).TextColor(tc.TextMuted)
			if ContrastCTA(c, tc, "Refresh changes").Clicked() {
				gm.RefreshAsync()
			}
		})
		return
	}

	if a.SelectedDiffPath == "" {
		if len(conflicts) > 0 {
			a.SelectedDiffPath = conflicts[0].Path
		} else if len(staged) > 0 {
			a.SelectedDiffPath = staged[0].Path
		} else if len(unstaged) > 0 {
			a.SelectedDiffPath = unstaged[0].Path
		}
	}

	ui.Row(c).Grow(1).FillHeight().AlignItems(ui.Stretch).Children(func() {
		// Left Column (340px): Staged/Unstaged file list + Commit Form
		ui.Column(c).Width(340).Shrink(0).FillHeight().Background(tc.Sidebar).
			BorderWidth(0, 1, 0, 0).BorderColor(tc.SidebarBorder).Children(func() {
			ui.Row(c).Height(36).Padding(0, 12).Gap(6).AlignItems(ui.Center).
				BorderWidth(0, 0, 1, 0).BorderColor(tc.SidebarBorder).Children(func() {
				ui.Text(c, "Working Tree").FontSize(12).FontWeight(600).TextColor(tc.Text)
				ui.Box(c).Grow(1)
				ui.Text(c, fmt.Sprintf("%d files", totalFiles)).FontSize(11).TextColor(tc.TextMuted)
			})

			ui.Scroll(c).Grow(1).Children(func() {
				ui.Column(c).Padding(6, 8).Gap(4).Children(func() {
					if len(staged) > 0 {
						ui.Row(c).Padding(2, 4).AlignItems(ui.Center).Children(func() {
							ui.Text(c, fmt.Sprintf("STAGED (%d)", len(staged))).FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted)
							ui.Box(c).Grow(1)
							unstageAll := ui.Row(c.Key("gc-unstage-all")).Radius(4).Padding(2, 6).AlignItems(ui.Center).Cursor(ui.CursorPointer)
							unstageAll.Children(func() {
								ui.Text(c, "Unstage All").FontSize(10).TextColor(tc.TextMuted)
							})
							if unstageAll.Hovered() {
								unstageAll.Background(tc.SidebarRowHover)
							}
							if unstageAll.Clicked() && gitOpsBusy.CompareAndSwap(false, true) {
								go func() {
									gm.UnstageAll()
									a.runOnUI(func() { gitOpsBusy.Store(false) })
								}()
							}
						})
						for _, f := range staged {
							f := f
							isSel := a.SelectedDiffPath == f.Path
							acted := false
							row := ui.Row(c.Key("gc-staged-"+f.Path)).Height(28).Radius(6).Padding(0, 8).Gap(6).AlignItems(ui.Center).Cursor(ui.CursorPointer)
							row.Children(func() {
								icons.RenderFileIcon(c, f.Path, 13, tc.IconMuted)
								ui.Text(c, filepath.Base(f.Path)).FontSize(11.5).Grow(1).SingleLine()
								DiffStatText(c, tc, int(f.Additions), int(f.Deletions))
								Badge(c, tc, "S", tc.Success)
								uBtn := ui.Row(c.Key("gc-u-"+f.Path)).Size(18, 18).Radius(4).AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer).Tooltip("Unstage")
								uBtn.Children(func() { ui.Icon(c, icons.TablerMinus).Width(10).Height(10).TextColor(tc.IconMuted) })
								if uBtn.Hovered() {
									uBtn.Background(tc.ToolbarControlHover)
								}
								if uBtn.Clicked() {
									acted = true
									go gm.Unstage(f.Path)
								}
							})
							if isSel {
								row.Background(tc.SidebarRowSelected)
							} else if row.Hovered() {
								row.Background(tc.SidebarRowHover)
							}
							if !acted && row.Clicked() {
								a.SelectedDiffPath = f.Path
							}
						}
					}

					if len(unstaged) > 0 {
						ui.Row(c).Padding(6, 4, 2, 4).AlignItems(ui.Center).Children(func() {
							ui.Text(c, fmt.Sprintf("CHANGES (%d)", len(unstaged))).FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted)
							ui.Box(c).Grow(1)
							stageAll := ui.Row(c.Key("gc-stage-all")).Radius(4).Padding(2, 6).AlignItems(ui.Center).Cursor(ui.CursorPointer)
							stageAll.Children(func() {
								ui.Text(c, "Stage All").FontSize(10).TextColor(tc.TextMuted)
							})
							if stageAll.Hovered() {
								stageAll.Background(tc.SidebarRowHover)
							}
							if stageAll.Clicked() && gitOpsBusy.CompareAndSwap(false, true) {
								go func() {
									gm.StageAll()
									a.runOnUI(func() { gitOpsBusy.Store(false) })
								}()
							}
						})
						for _, f := range unstaged {
							f := f
							isSel := a.SelectedDiffPath == f.Path
							acted := false
							row := ui.Row(c.Key("gc-unstaged-"+f.Path)).Height(28).Radius(6).Padding(0, 8).Gap(6).AlignItems(ui.Center).Cursor(ui.CursorPointer)
							row.Children(func() {
								icons.RenderFileIcon(c, f.Path, 13, tc.IconMuted)
								ui.Text(c, filepath.Base(f.Path)).FontSize(11.5).Grow(1).SingleLine()
								DiffStatText(c, tc, int(f.Additions), int(f.Deletions))
								stLbl := string(f.Status)
								if stLbl == "" {
									stLbl = "M"
								}
								Badge(c, tc, stLbl, tc.Accent)
								sBtn := ui.Row(c.Key("gc-s-"+f.Path)).Size(18, 18).Radius(4).AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer).Tooltip("Stage")
								sBtn.Children(func() { ui.Icon(c, icons.TablerPlus).Width(10).Height(10).TextColor(tc.IconMuted) })
								if sBtn.Hovered() {
									sBtn.Background(tc.ToolbarControlHover)
								}
								if sBtn.Clicked() {
									acted = true
									go gm.Stage(f.Path)
								}
							})
							if isSel {
								row.Background(tc.SidebarRowSelected)
							} else if row.Hovered() {
								row.Background(tc.SidebarRowHover)
							}
							if !acted && row.Clicked() {
								a.SelectedDiffPath = f.Path
							}
						}
					}
				})
			})

			// Commit Form docked at bottom of left column
			ui.Column(c).Padding(10, 12).Gap(8).BorderWidth(1, 0, 0, 0).BorderColor(tc.SidebarBorder).
				Background(tc.Surface).Children(func() {
				ui.TextInput(c, &gm.CommitSummary).Placeholder("Commit summary (required)").Grow(1)
				ui.TextInput(c, &gm.CommitDesc).Placeholder("Description (optional)").Grow(1)
				ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
					aiBtn := ui.Row(c.Key("gc-ai-msg")).Height(28).Radius(6).Padding(0, 8).Gap(4).AlignItems(ui.Center).
						Background(tc.Canvas).BorderWidth(1).BorderColor(tc.Border).Cursor(ui.CursorPointer)
					aiBtn.Children(func() {
						ui.Icon(c, icons.TablerSparkles).Width(12).Height(12).TextColor(tc.Accent)
						ui.Text(c, "AI Message").FontSize(11).FontWeight(500).TextColor(tc.Text)
					})
					if aiBtn.Hovered() {
						aiBtn.Background(tc.SurfaceRaised)
					}
					if aiBtn.Clicked() {
						go gm.GenerateAICommit("")
					}
					ui.Box(c).Grow(1)
					busy := gitOpsBusy.Load()
					canCommit := strings.TrimSpace(snap.CommitSummary) != "" && totalFiles > 0
					branchName := snap.Branch
					if branchName == "" {
						branchName = "main"
					}
					btnText := fmt.Sprintf("Commit (%s)", branchName)
					cBtn := ui.Row(c.Key("gc-commit")).Height(28).Radius(6).Padding(0, 12).Gap(5).AlignItems(ui.Center).Justify(ui.Center)
					if busy {
						cBtn.Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border)
						cBtn.Children(func() { ui.Spinner(c).Size(11, 11).Label("Committing…") })
					} else if canCommit {
						cBtn.Background(tc.Accent).Cursor(ui.CursorPointer)
						cBtn.Children(func() {
							ui.Icon(c, icons.TablerCheck).Width(11).Height(11).TextColor(tc.AccentForeground)
							ui.Text(c, btnText).FontSize(11).FontWeight(600).TextColor(tc.AccentForeground)
						})
						if cBtn.Clicked() && gitOpsBusy.CompareAndSwap(false, true) {
							summary, desc := snap.CommitSummary, snap.CommitDesc
							stageFirst := len(staged) == 0 && len(unstaged) > 0
							go func() {
								if stageFirst {
									gm.StageAll()
								}
								fullMsg := summary
								if strings.TrimSpace(desc) != "" {
									fullMsg += "\n\n" + desc
								}
								err := gm.Commit(fullMsg)
								a.runOnUI(func() {
									gitOpsBusy.Store(false)
									if err != nil {
										a.queueToast(ui.Toast{Title: "Commit Error", Description: err.Error(), Type: "error"})
									} else {
										a.queueToast(ui.Toast{Title: "Committed", Description: summary, Type: "success"})
										gm.ClearCommitDraft()
										a.SelectedDiffPath = ""
									}
								})
							}()
						}
					} else {
						cBtn.Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border)
						cBtn.Children(func() { ui.Text(c, btnText).FontSize(11).TextColor(tc.TextMuted) })
					}
				})
			})
		})

		// Right Column (Grow 1): Wide Diff Viewer with Split/Unified toggle + Hunk Revert
		ui.Column(c).Grow(1).FillHeight().Clip().Children(func() {
			diffTitle := a.SelectedDiffPath
			ui.Row(c).Height(36).Padding(0, 14).Gap(8).AlignItems(ui.Center).
				BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Background(tc.Surface).Children(func() {
				ui.Icon(c, icons.TablerGitCompare).Width(13).Height(13).TextColor(tc.Accent)
				ui.Text(c, diffTitle).FontSize(12).FontWeight(600).TextColor(tc.Text).Grow(1).SingleLine()
				if a.Editor != nil {
					layoutLbl := "Unified"
					if a.Editor.DiffLayout == int(mygoeditor.DiffSplit) {
						layoutLbl = "Split"
					}
					lBtn := ui.Row(c.Key("gc-diff-layout")).Height(24).Radius(6).Padding(0, 8).AlignItems(ui.Center).
						Background(tc.Canvas).BorderWidth(1).BorderColor(tc.Border).Cursor(ui.CursorPointer).Tooltip("Toggle Unified / Split Diff")
					lBtn.Children(func() { ui.Text(c, layoutLbl).FontSize(11).FontWeight(500).TextColor(tc.Text) })
					if lBtn.Hovered() {
						lBtn.Background(tc.SurfaceRaised)
					}
					if lBtn.Clicked() {
						if a.Editor.DiffLayout == int(mygoeditor.DiffSplit) {
							a.Editor.DiffLayout = int(mygoeditor.DiffUnified)
						} else {
							a.Editor.DiffLayout = int(mygoeditor.DiffSplit)
						}
					}
				}
				openBtn := ui.Row(c.Key("gc-open-ed")).Height(24).Radius(6).Padding(0, 8).Gap(4).AlignItems(ui.Center).
					Background(tc.Canvas).BorderWidth(1).BorderColor(tc.Border).Cursor(ui.CursorPointer)
				openBtn.Children(func() {
					ui.Icon(c, icons.TablerCode).Width(11).Height(11).TextColor(tc.IconMuted)
					ui.Text(c, "Open in Editor").FontSize(11).TextColor(tc.Text)
				})
				if openBtn.Hovered() {
					openBtn.Background(tc.SurfaceRaised)
				}
				if openBtn.Clicked() {
					fullPath := filepath.Join(a.Workspace.RootDir, diffTitle)
					a.setMode(ModeEditor)
					a.IsGitClientOpen = false
					_, _ = a.Editor.OpenFile(fullPath, diffTitle)
				}
			})

			patch, diffReady := gm.GetCachedFileDiff(diffTitle)
			if !diffReady {
				ui.Column(c).Grow(1).Center().Gap(6).Children(func() {
					ui.Spinner(c).Size(14, 14).Label("Loading diff")
				})
			} else if patch == "" {
				ui.Column(c).Grow(1).Center().Children(func() {
					ui.Text(c, "No diff available for this file").FontSize(12).TextColor(tc.TextMuted)
				})
			} else {
				a.renderHunkRevertStrip(c, tc, gm, diffTitle, patch)
				if d := a.Editor.OpenPatchDiff(diffTitle, patch, a.ThemeMgr); d != nil {
					d.Layout = mygoeditor.DiffLayout(a.Editor.DiffLayout)
					mygoeditor.DiffView(c, d).Grow(1).FillWidth()
				}
			}
		})
	})
}

// renderGitBranchesSurface renders branch list + create at full width.
func (a *AppUI) renderGitBranchesSurface(c *ui.Context, tc theme.ThemeColors, gm *git.GitManager) {
	snap := gm.Snapshot()
	ui.Column(c).Grow(1).Children(func() {
		ui.Row(c).Padding(12, 16).Gap(8).AlignItems(ui.Center).Children(func() {
			// Safe to bind: NewBranchName is only ever written from the
			// render thread (this binding and the create-branch handler).
			ui.TextInput(c, &gm.NewBranchName).Placeholder("New branch name…").Width(260)
			createBtn := ui.Row(c).Height(30).Radius(6).Padding(0, 12).Gap(5).AlignItems(ui.Center).
				Background(tc.Accent.Alpha(0.12)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.3)).Cursor(ui.CursorPointer)
			createBtn.Children(func() {
				ui.Icon(c, icons.TablerPlus).Width(12).Height(12).TextColor(tc.Accent)
				ui.Text(c, "Create").FontSize(11).FontWeight(600).TextColor(tc.Accent)
			})
			if createBtn.Hovered() {
				createBtn.Background(tc.Accent.Alpha(0.2))
			}
			if createBtn.Clicked() && strings.TrimSpace(gm.NewBranchName) != "" {
				name := strings.TrimSpace(gm.NewBranchName)
				gm.NewBranchName = ""
				go func() {
					if err := gm.CreateBranch(name); err != nil {
						a.queueToast(ui.Toast{Title: "Branch Error", Description: err.Error(), Type: "error"})
					} else {
						a.queueToast(ui.Toast{Title: "Branch Created", Description: name, Type: "success"})
					}
				}()
			}
		})
		ui.Scroll(c).Grow(1).Padding(8, 16).Children(func() {
			ui.Column(c).Gap(2).Children(func() {
				for _, b := range snap.Branches {
					branch := b
					isCurrent := branch == snap.Branch
					row := ui.Row(c.Key("gbr-"+branch)).Height(34).Radius(6).Padding(0, 10).Gap(8).AlignItems(ui.Center).Cursor(ui.CursorPointer)
					row.Children(func() {
						ui.Icon(c, icons.TablerGitBranch).Width(13).Height(13).TextColor(tc.IconMuted)
						ui.Text(c, branch).FontSize(12).FontWeight(500).Grow(1).SingleLine()
						if isCurrent {
							Badge(c, tc, "HEAD", tc.Accent)
						}
					})
					if isCurrent {
						row.Background(tc.Accent.Alpha(0.12))
					} else if row.Hovered() {
						row.Background(tc.Text.Alpha(0.04))
					}
					if row.Clicked() && !isCurrent {
						a.checkoutBranch(branch)
					}
				}
			})
		})
	})
}

// renderGitHistorySurface renders the two-pane history: commit list with
// search on the left, selected commit detail with per-file diffs on the right.
func (a *AppUI) renderGitHistorySurface(c *ui.Context, tc theme.ThemeColors, gm *git.GitManager) {
	if a.GitHistoryWidth == 0 {
		a.GitHistoryWidth = 420
	}
	// Render from a point-in-time snapshot; SelectCommit's goroutine publishes
	// SelectedCommit* fields under the manager's lock.
	snap := gm.Snapshot()
	ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
		// Left: search + commit list (drag-resizable, GitClientView parity)
		ui.Column(c).Width(a.GitHistoryWidth).BorderWidth(0, 1, 0, 0).BorderColor(tc.Border).Children(func() {
			ui.Row(c).Padding(10, 12).Children(func() {
				ui.Row(c).Height(30).Radius(6).Padding(0, 8).Gap(6).AlignItems(ui.Center).
					Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).Grow(1).Children(func() {
					ui.Icon(c, icons.TablerSearch).Width(12).Height(12).TextColor(tc.IconMuted)
					// Safe to bind: SearchQuery is only ever written from the
					// render thread (this binding) — no goroutine touches it.
					ui.TextInput(c, &gm.SearchQuery).Placeholder("Search commits by message, author, SHA...").Grow(1)
				})
			})
			if len(snap.Commits) == 0 {
				ui.Column(c).Grow(1).Center().Gap(6).Children(func() {
					ui.Icon(c, icons.TablerGitCommit).Width(28).Height(28).TextColor(tc.IconMuted)
					ui.Text(c, "No commits yet").FontSize(12).TextColor(tc.TextMuted)
				})
				return
			}
			ui.Scroll(c).Grow(1).Children(func() {
				ui.Column(c).Padding(4, 8).Gap(2).Children(func() {
					query := strings.ToLower(strings.TrimSpace(gm.SearchQuery))
					for _, commit := range snap.Commits {
						cm := commit
						if query != "" &&
							!strings.Contains(strings.ToLower(cm.Message), query) &&
							!strings.Contains(strings.ToLower(cm.AuthorName), query) &&
							!strings.Contains(strings.ToLower(cm.Hash), query) {
							continue
						}
						isSel := snap.SelectedCommitHash == cm.Hash
						row := ui.Column(c.Key("gcm-"+cm.Hash)).Radius(8).Padding(8, 10).Gap(2).Cursor(ui.CursorPointer)
						row.Children(func() {
							ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
								renderGraphLanes(c, tc, cm.GraphPrefix)
								ui.Text(c, cm.Message).FontSize(12).FontWeight(500).Grow(1).SingleLine()
							})
							ui.Row(c).Gap(6).AlignItems(ui.Center).Padding(0, 16).Children(func() {
								ui.Text(c, cm.AuthorName).FontSize(11).TextColor(tc.TextMuted)
								ui.Box(c).Grow(1)
								if cm.Decorations != "" {
									ui.Text(c, cm.Decorations).FontSize(9).TextColor(tc.Success)
								}
								ui.Text(c, cm.ShortHash).FontSize(10).FontWeight(600).TextColor(tc.Accent)
							})
						})
						if isSel {
							row.Background(tc.Accent.Alpha(0.12)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.3))
						} else if row.Hovered() {
							row.Background(tc.Text.Alpha(0.04))
						}
						if row.Clicked() {
							gm.SelectCommit(cm.Hash)
						}
					}
				})
			})
		})

		// Drag resizer between the commit list and the detail pane.
		handle := ui.Box(c.Key("git-history-resizer")).Width(4).FillHeight().Background(tc.Border).Cursor(ui.CursorResizeEW)
		if handle.Hovered() {
			handle.Background(tc.Accent.Alpha(0.6))
		}
		if _, dx, dragging := handle.Dragged(); dragging {
			a.GitHistoryWidth += dx
			if winW, _ := c.Size(); winW > 0 {
				if max := winW * 0.6; a.GitHistoryWidth > max {
					a.GitHistoryWidth = max
				}
			}
			if a.GitHistoryWidth < 280 {
				a.GitHistoryWidth = 280
			}
		}

		// Right: commit detail
		ui.Column(c).Grow(1).Children(func() {
			if snap.SelectedCommitHash == "" {
				ui.Column(c).Grow(1).Center().Gap(6).Children(func() {
					ui.Icon(c, icons.TablerGitCommit).Width(32).Height(32).TextColor(tc.IconMuted)
					ui.Text(c, "Select a commit to inspect").FontSize(13).TextColor(tc.TextMuted)
				})
				return
			}

			// Detail header: message, author, hash, stats + diff layout / expand controls
			ui.Column(c).Padding(14, 16).Gap(6).BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					ui.Text(c, snap.SelectedCommitMessage).FontSize(14).FontWeight(700).Grow(1).SingleLine()
					shortH := snap.SelectedCommitHash
					if len(shortH) > 8 {
						shortH = shortH[:8]
					}
					Badge(c, tc, shortH, tc.Accent)
				})
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icons.TablerUser).Width(12).Height(12).TextColor(tc.IconMuted)
					ui.Text(c, snap.SelectedCommitAuthor).FontSize(12).TextColor(tc.TextMuted)
					if len(snap.SelectedCommitFiles) > 0 {
						add, del := 0, 0
						for _, f := range snap.SelectedCommitFiles {
							add += f.Additions
							del += f.Deletions
						}
						ui.Text(c, fmt.Sprintf("%d files changed", len(snap.SelectedCommitFiles))).FontSize(12).TextColor(tc.TextMuted)
						ui.Text(c, fmt.Sprintf("+%d", add)).FontSize(12).FontWeight(600).TextColor(tc.DiffAdditionForeground)
						ui.Text(c, fmt.Sprintf("-%d", del)).FontSize(12).FontWeight(600).TextColor(tc.DiffDeletionForeground)
					}
					ui.Box(c).Grow(1)
					if a.Editor != nil {
						layoutLbl := "Unified"
						if a.Editor.DiffLayout == int(mygoeditor.DiffSplit) {
							layoutLbl = "Split"
						}
						lBtn := ui.Row(c.Key("ghist-diff-layout")).Height(24).Radius(6).Padding(0, 8).AlignItems(ui.Center).
							Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).Cursor(ui.CursorPointer).Tooltip("Toggle Unified / Split Diff")
						lBtn.Children(func() {
							ui.Text(c, layoutLbl).FontSize(11).FontWeight(500).TextColor(tc.Text)
						})
						if lBtn.Hovered() {
							lBtn.Background(tc.SurfaceRaised)
						}
						if lBtn.Clicked() {
							if a.Editor.DiffLayout == int(mygoeditor.DiffSplit) {
								a.Editor.DiffLayout = int(mygoeditor.DiffUnified)
							} else {
								a.Editor.DiffLayout = int(mygoeditor.DiffSplit)
							}
						}
					}
					if len(snap.SelectedCommitFiles) > 1 {
						expBtn := ui.Row(c.Key("ghist-expand-all")).Height(24).Radius(6).Padding(0, 8).AlignItems(ui.Center).
							Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).Cursor(ui.CursorPointer)
						expBtn.Children(func() {
							ui.Text(c, "Expand All").FontSize(11).TextColor(tc.TextMuted)
						})
						if expBtn.Hovered() {
							expBtn.Background(tc.SurfaceRaised)
						}
						if expBtn.Clicked() {
							if a.gitHistoryExpanded == nil {
								a.gitHistoryExpanded = make(map[string]bool)
							}
							for _, f := range snap.SelectedCommitFiles {
								a.gitHistoryExpanded[f.Path] = true
							}
						}
						colBtn := ui.Row(c.Key("ghist-collapse-all")).Height(24).Radius(6).Padding(0, 8).AlignItems(ui.Center).
							Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).Cursor(ui.CursorPointer)
						colBtn.Children(func() {
							ui.Text(c, "Collapse All").FontSize(11).TextColor(tc.TextMuted)
						})
						if colBtn.Hovered() {
							colBtn.Background(tc.SurfaceRaised)
						}
						if colBtn.Clicked() {
							a.gitHistoryExpanded = make(map[string]bool)
							for _, f := range snap.SelectedCommitFiles {
								a.gitHistoryExpanded[f.Path] = false
							}
						}
					}
				})
			})

			// Per-file diffs (lazy & collapsible so large commits stay smooth)
			if snap.LoadingCommitDiff {
				ui.Column(c).Grow(1).Center().Gap(6).Children(func() {
					ui.Spinner(c).Size(16, 16).Label("Loading commit diff...")
				})
				return
			}
			if len(snap.SelectedCommitFiles) == 0 {
				ui.Column(c).Grow(1).Center().Gap(6).Children(func() {
					ui.Icon(c, icons.TablerGitCommit).Width(26).Height(26).TextColor(tc.IconMuted)
					ui.Text(c, "No file changes in this commit").FontSize(12).TextColor(tc.TextMuted)
				})
				return
			}
			if a.gitHistoryExpandedCommit != snap.SelectedCommitHash {
				a.gitHistoryExpandedCommit = snap.SelectedCommitHash
				a.gitHistoryExpanded = make(map[string]bool, len(snap.SelectedCommitFiles))
				autoLimit := 5
				if len(snap.SelectedCommitFiles) <= 8 {
					autoLimit = len(snap.SelectedCommitFiles)
				}
				for i, f := range snap.SelectedCommitFiles {
					a.gitHistoryExpanded[f.Path] = i < autoLimit
				}
			}
			ui.Scroll(c).Grow(1).Padding(12, 16).Children(func() {
				ui.Column(c).Gap(10).Children(func() {
					for i := range snap.SelectedCommitFiles {
						f := &snap.SelectedCommitFiles[i]
						isExpanded := a.gitHistoryExpanded[f.Path]
						ui.Column(c.Key("ghist-file-" + f.Path)).Radius(10).Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).Gap(0).Clip().Children(func() {
							hdr := ui.Row(c.Key("ghist-fhdr-"+f.Path)).Height(34).Padding(0, 12).Gap(8).AlignItems(ui.Center).Cursor(ui.CursorPointer)
							if isExpanded {
								hdr.BorderWidth(0, 0, 1, 0).BorderColor(tc.Border.Alpha(0.5))
							}
							if hdr.Hovered() {
								hdr.Background(tc.SurfaceRaised)
							}
							hdr.Children(func() {
								chev := icons.TablerChevronRight
								if isExpanded {
									chev = icons.TablerChevronDown
								}
								ui.Icon(c, chev).Width(12).Height(12).TextColor(tc.IconMuted)
								icons.RenderFileIcon(c, f.Path, 13, tc.IconMuted)
								ui.Text(c, filepath.Base(f.Path)).FontSize(12).FontWeight(600).Grow(1).SingleLine()
								dir := filepath.Dir(f.Path)
								if dir != "." {
									ui.Text(c, dir).FontSize(10).TextColor(tc.TextMuted).SingleLine()
								}
								if f.Additions > 0 {
									ui.Text(c, fmt.Sprintf("+%d", f.Additions)).FontSize(11).FontWeight(600).TextColor(tc.DiffAdditionForeground)
								}
								if f.Deletions > 0 {
									ui.Text(c, fmt.Sprintf("-%d", f.Deletions)).FontSize(11).FontWeight(600).TextColor(tc.DiffDeletionForeground)
								}
							})
							if hdr.Clicked() {
								a.gitHistoryExpanded[f.Path] = !isExpanded
							}
							if isExpanded {
								if f.Diff == nil && a.Editor != nil {
									key := "commit/" + snap.SelectedCommitHash + "/" + f.Path
									f.Diff = a.Editor.OpenPatchDiffKeyed(key, f.Patch, a.ThemeMgr)
								}
								if d, ok := f.Diff.(*mygoeditor.DiffState); ok && d != nil {
									if a.Editor != nil {
										d.Layout = mygoeditor.DiffLayout(a.Editor.DiffLayout)
										a.Editor.ApplyDiffTheme(d)
									}
									mygoeditor.DiffView(c, d).Height(340).FillWidth().Clip()
								}
							}
						})
					}
				})
			})
		})
	})
}

// renderGraphLanes draws the commit-graph lane glyphs with the per-lane
// colors of index.css (--graph-c0..c9): each non-space rune takes the color
// of its column, GitGraphPane parity for the history list.
func renderGraphLanes(c *ui.Context, tc theme.ThemeColors, prefix string) {
	if strings.TrimSpace(prefix) == "" {
		ui.Box(c).Size(8, 8).Radius(4).Background(tc.Accent.Alpha(0.7))
		return
	}
	runes := []rune(prefix)
	ui.Row(c).Gap(1).AlignItems(ui.Center).Children(func() {
		for col, r := range runes {
			r := r
			if r == ' ' {
				ui.Box(c).Width(6)
				continue
			}
			color := tc.GraphColors[col%len(tc.GraphColors)]
			if r == '│' || r == '|' || r == '/' || r == '\\' {
				ui.Text(c, string(r)).FontSize(10).Font("monospace").TextColor(color.Alpha(0.8))
				continue
			}
			ui.Text(c, string(r)).FontSize(10).Font("monospace").TextColor(color)
		}
	})
}
