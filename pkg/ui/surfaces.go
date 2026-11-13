package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"
	mygoeditor "github.com/egoist/mygo/ui/editor"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/git"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/icons"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/theme"
	"github.com/hasdev/forge-ade/internal/board"
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
	a.Mode = ModeAgent
	a.AgentViewMode = AgentViewBoard
	a.RefreshBoard()
}

// openGitClientSurface switches the agent area to the git client.
func (a *AppUI) openGitClientSurface() {
	a.Mode = ModeAgent
	a.AgentViewMode = AgentViewGit
	if a.Git != nil {
		go a.Git.Refresh()
	}
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
				a.OpenNewTaskModal()
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
						colIcon, colColor = icons.TablerClock, ui.Hex("#f59e0b")
					case 2:
						colIcon, colColor = icons.TablerEye, ui.Hex("#8b5cf6")
					case 3:
						colIcon, colColor = icons.TablerFlask, ui.Hex("#ec4899")
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

	ui.Column(c).Fill().Background(tc.Canvas).Children(func() {
		// Toolbar
		ui.Row(c).Height(52).Padding(0, 16).Gap(8).AlignItems(ui.Center).
			BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
			if gm.Branch != "" {
				branchPill := ui.Row(c).Height(30).Radius(8).Padding(0, 10).Gap(6).AlignItems(ui.Center).
					Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
				branchPill.Children(func() {
					ui.Icon(c, icons.TablerGitBranch).Width(13).Height(13).TextColor(tc.Accent)
					ui.Text(c, gm.Branch).FontSize(12).FontWeight(600)
					ui.Icon(c, icons.TablerChevronDown).Width(11).Height(11).TextColor(tc.IconMuted)
				})
				if branchPill.Clicked() {
					a.ShowBranchModal = true
				}
			}

			tabs := []struct {
				Tab  int
				Name string
			}{
				{git.GitTabChanges, fmt.Sprintf("Changes (%d)", len(gm.Staged)+len(gm.Unstaged)+len(gm.Untracked))},
				{git.GitTabHistory, "History"},
				{git.GitTabBranches, "Branches"},
			}
			for _, tab := range tabs {
				t := tab
				active := gm.ActiveTab == t.Tab
				b := ui.Row(c.Key("gctab-"+t.Name)).Height(30).Radius(8).Padding(0, 12).AlignItems(ui.Center).Justify(ui.Center)
				b.Children(func() {
					ui.Text(c, t.Name).FontSize(12).FontWeight(500)
				})
				if active {
					b.Background(tc.Accent.Alpha(0.15)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.3))
					b.Children(func() {
						ui.Text(c, t.Name).FontSize(12).FontWeight(600).TextColor(tc.Accent)
					})
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
					Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
				b.Children(func() {
					if busy {
						ui.Spinner(c).Size(12, 12).Label(label)
					} else {
						ui.Icon(c, icon).Width(13).Height(13).TextColor(tc.IconMuted)
					}
					ui.Text(c, label).FontSize(12).FontWeight(500)
				})
				if b.Hovered() {
					b.Background(tc.Text.Alpha(0.05))
				}
				if b.Clicked() && !busy {
					go func() {
						if err := run(); err != nil {
							c.AddToast(ui.Toast{Title: label + " Failed", Description: err.Error(), Type: "error"})
						} else {
							c.AddToast(ui.Toast{Title: label + " Completed", Type: "success"})
						}
						if a.Window != nil {
							a.Window.Invalidate()
						}
					}()
				}
			}
			remoteBtn(icons.TablerArrowDown, "Fetch Origin", gm.IsFetching, func() error { return gm.Fetch() })
			remoteBtn(icons.TablerArrowDown, "Pull", gm.IsPulling, func() error { return gm.Pull() })
			remoteBtn(icons.TablerArrowUp, "Push", gm.IsPushing, func() error { return gm.Push() })

			refreshBtn := ui.Row(c).Size(30, 30).Radius(8).AlignItems(ui.Center).Justify(ui.Center).
				Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
			refreshBtn.Children(func() {
				ui.Icon(c, icons.TablerRefresh).Width(13).Height(13).TextColor(tc.IconMuted)
			})
			if refreshBtn.Clicked() {
				go func() {
					gm.Refresh()
					a.loadGitRemoteURL()
				}()
			}

			closeBtn := ui.Row(c).Size(30, 30).Radius(8).AlignItems(ui.Center).Justify(ui.Center).
				Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
			closeBtn.Children(func() {
				ui.Icon(c, icons.TablerX).Width(13).Height(13).TextColor(tc.IconMuted)
			})
			if closeBtn.Clicked() {
				a.AgentViewMode = AgentViewChat
			}
		})

		// Body per tab
		switch gm.ActiveTab {
		case git.GitTabHistory:
			a.renderGitHistorySurface(c, tc, gm)
		case git.GitTabBranches:
			a.renderGitBranchesSurface(c, tc, gm)
		default:
			a.renderGitChangesSurface(c, tc, gm)
		}
	})
}

// renderGitChangesSurface renders the full-width working tree changes with
// the commit form.
func (a *AppUI) renderGitChangesSurface(c *ui.Context, tc theme.ThemeColors, gm *git.GitManager) {
	a.renderChangesPane(c)
}

// renderGitBranchesSurface renders branch list + create at full width.
func (a *AppUI) renderGitBranchesSurface(c *ui.Context, tc theme.ThemeColors, gm *git.GitManager) {
	ui.Column(c).Grow(1).Children(func() {
		ui.Row(c).Padding(12, 16).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.TextInput(c, &gm.NewBranchName).Placeholder("New branch name…").Width(260)
			createBtn := ui.Row(c).Height(30).Radius(6).Padding(0, 12).Gap(5).AlignItems(ui.Center).
				Background(tc.Accent.Alpha(0.12)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.3))
			createBtn.Children(func() {
				ui.Icon(c, icons.TablerPlus).Width(12).Height(12).TextColor(tc.Accent)
				ui.Text(c, "Create").FontSize(11).FontWeight(600).TextColor(tc.Accent)
			})
			if createBtn.Clicked() && strings.TrimSpace(gm.NewBranchName) != "" {
				name := strings.TrimSpace(gm.NewBranchName)
				gm.NewBranchName = ""
				go func() {
					if err := gm.CreateBranch(name); err != nil {
						c.AddToast(ui.Toast{Title: "Branch Error", Description: err.Error(), Type: "error"})
					} else {
						c.AddToast(ui.Toast{Title: "Branch Created", Description: name, Type: "success"})
					}
					if a.Window != nil {
						a.Window.Invalidate()
					}
				}()
			}
		})
		ui.Scroll(c).Grow(1).Padding(8, 16).Children(func() {
			ui.Column(c).Gap(2).Children(func() {
				for _, b := range gm.Branches {
					branch := b
					isCurrent := branch == gm.Branch
					row := ui.Row(c.Key("gbr-"+branch)).Height(34).Radius(6).Padding(0, 10).Gap(8).AlignItems(ui.Center)
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
	ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
		// Left: search + commit list
		ui.Column(c).Width(420).BorderWidth(0, 1, 0, 0).BorderColor(tc.Border).Children(func() {
			ui.Row(c).Padding(10, 12).Children(func() {
				ui.Row(c).Height(30).Radius(6).Padding(0, 8).Gap(6).AlignItems(ui.Center).
					Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).Grow(1).Children(func() {
					ui.Icon(c, icons.TablerSearch).Width(12).Height(12).TextColor(tc.IconMuted)
					ui.TextInput(c, &gm.SearchQuery).Placeholder("Search commits by message, author, SHA...").Grow(1)
				})
			})
			ui.Scroll(c).Grow(1).Children(func() {
				ui.Column(c).Padding(4, 8).Gap(2).Children(func() {
					query := strings.ToLower(strings.TrimSpace(gm.SearchQuery))
					for _, commit := range gm.Commits {
						cm := commit
						if query != "" &&
							!strings.Contains(strings.ToLower(cm.Message), query) &&
							!strings.Contains(strings.ToLower(cm.AuthorName), query) &&
							!strings.Contains(strings.ToLower(cm.Hash), query) {
							continue
						}
						isSel := gm.SelectedCommitHash == cm.Hash
						row := ui.Column(c.Key("gcm-"+cm.Hash)).Radius(8).Padding(8, 10).Gap(2)
						row.Children(func() {
							ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
								ui.Box(c).Size(8, 8).Radius(4).Background(tc.Accent.Alpha(0.7))
								ui.Text(c, cm.Message).FontSize(12).FontWeight(500).Grow(1).SingleLine()
								if cm.GraphPrefix != "" && strings.TrimSpace(cm.GraphPrefix) != "" {
									ui.Text(c, cm.GraphPrefix).FontSize(9).TextColor(tc.Accent.Alpha(0.6))
								}
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

		// Right: commit detail
		ui.Column(c).Grow(1).Children(func() {
			if gm.SelectedCommitHash == "" {
				ui.Column(c).Grow(1).Center().Gap(6).Children(func() {
					ui.Icon(c, icons.TablerGitCommit).Width(32).Height(32).TextColor(tc.IconMuted)
					ui.Text(c, "Select a commit to inspect").FontSize(13).TextColor(tc.TextMuted)
				})
				return
			}

			// Detail header: message, author, hash, stats
			ui.Column(c).Padding(14, 16).Gap(6).BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					ui.Text(c, gm.SelectedCommitMessage).FontSize(14).FontWeight(700).Grow(1).SingleLine()
					shortH := gm.SelectedCommitHash
					if len(shortH) > 8 {
						shortH = shortH[:8]
					}
					Badge(c, tc, shortH, tc.Accent)
				})
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icons.TablerUser).Width(12).Height(12).TextColor(tc.IconMuted)
					ui.Text(c, gm.SelectedCommitAuthor).FontSize(12).TextColor(tc.TextMuted)
					if len(gm.SelectedCommitFiles) > 0 {
						add, del := 0, 0
						for _, f := range gm.SelectedCommitFiles {
							add += f.Additions
							del += f.Deletions
						}
						ui.Text(c, fmt.Sprintf("%d files changed", len(gm.SelectedCommitFiles))).FontSize(12).TextColor(tc.TextMuted)
						ui.Text(c, fmt.Sprintf("+%d", add)).FontSize(12).FontWeight(600).TextColor(tc.DiffAdditionForeground)
						ui.Text(c, fmt.Sprintf("-%d", del)).FontSize(12).FontWeight(600).TextColor(tc.DiffDeletionForeground)
					}
				})
			})

			// Per-file diffs
			if len(gm.SelectedCommitFiles) == 0 {
				ui.Column(c).Grow(1).Center().Children(func() {
					ui.Text(c, "Loading commit diff...").FontSize(12).TextColor(tc.TextMuted)
				})
				return
			}
			ui.Scroll(c).Grow(1).Padding(12, 16).Children(func() {
				ui.Column(c).Gap(12).Children(func() {
					for _, f := range gm.SelectedCommitFiles {
						f := f
						ui.Column(c).Radius(10).Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).Gap(0).Clip().Children(func() {
							ui.Row(c).Height(34).Padding(0, 12).Gap(8).AlignItems(ui.Center).
								BorderWidth(0, 0, 1, 0).BorderColor(tc.Border.Alpha(0.5)).Children(func() {
								ui.Icon(c, icons.FileIconSVG(f.Path)).Width(13).Height(13).TextColor(tc.IconMuted)
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
							if f.Diff == nil && a.Editor != nil {
								key := "commit/" + gm.SelectedCommitHash + "/" + f.Path
								f.Diff = a.Editor.OpenPatchDiffKeyed(key, f.Patch, a.ThemeMgr)
							}
							if d, ok := f.Diff.(*mygoeditor.DiffState); ok && d != nil {
								mygoeditor.DiffView(c, d).Height(360).FillWidth().Clip()
							}
						})
					}
				})
			})
		})
	})
}
