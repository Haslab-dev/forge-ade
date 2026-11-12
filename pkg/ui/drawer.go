package ui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/Haslab-dev/RakitKode/llm"
	"github.com/egoist/mygo/ui"
	mygoeditor "github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/agent"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/git"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/icons"
	internalgit "github.com/hasdev/forge-ade/internal/git"
)

// renderRightDrawer renders the collapsible side drawer: tabs and active pane.
func (a *AppUI) renderRightDrawer(c *ui.Context) {
	tc := a.ThemeMgr.Colors()

	ui.Column(c).Width(a.RightDrawerWidth).Background(tc.Canvas).BorderWidth(0, 0, 0, 1).BorderColor(tc.Border).Children(func() {
		// Drawer Header Tabs
		ui.Row(c).Height(WorkspaceTopBarHeight).Gap(4).Padding(0, 10).AlignItems(ui.Center).
			BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
			tabs := []struct {
				Tab  int
				Name string
				Icon *ui.SVG
			}{
				{RightTabChanges, "Changes", icons.TablerFileDiff},
				{RightTabTerminal, "Terminal", icons.TablerTerminal},
				{RightTabChat, "Chat", icons.TablerMessage},
				{RightTabTasks, "Tasks", icons.TablerChecklist},
				{RightTabBoard, "Board", icons.TablerLayoutKanban},
				{RightTabGit, "Git", icons.TablerGitBranch},
				{RightTabDatabase, "DB", icons.TablerDatabase},
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

			if IconButton(c, tc, icons.TablerX, 13).Clicked() {
				a.ShowRightDrawer = false
			}
		})

		// Drawer Content
		ui.Column(c).Grow(1).FillHeight().Children(func() {
			switch a.RightDrawerTab {
			case RightTabChanges:
				a.renderChangesPane(c)
			case RightTabTerminal:
				if a.Terminal != nil {
					a.renderTerminal(c, a.Terminal)
				}
			case RightTabChat:
				a.renderSideChatPane(c)
			case RightTabTasks:
				if a.Agent != nil && a.Agent.TaskManager != nil {
					a.renderTaskChecklist(c)
				}
			case RightTabBoard:
				a.renderBoardPane(c)
			case RightTabGit:
				a.renderGitPane(c)
			case RightTabDatabase:
				a.renderDatabasePane(c)
			}
		})
	})
}

// renderChangesPane renders working tree changes and unified diff viewer.
func (a *AppUI) renderChangesPane(c *ui.Context) {
	tc := a.ThemeMgr.Colors()

	var files []internalgit.FileStatus
	if a.Git != nil {
		files = append(files, a.Git.Staged...)
		files = append(files, a.Git.Unstaged...)
		files = append(files, a.Git.Untracked...)
	}

	ui.Row(c).Height(40).Padding(0, 12).AlignItems(ui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
		ui.Text(c, "Changes").FontSize(13).FontWeight(600)
		ui.Box(c).Grow(1)
		if IconButton(c, tc, icons.TablerRefresh, 12).Clicked() && a.Git != nil {
			a.Git.Refresh()
		}
		ui.Text(c, fmt.Sprintf("%d files", len(files))).FontSize(11).TextColor(tc.TextMuted)
	})

	if len(files) == 0 {
		ui.Column(c).Grow(1).Center().Gap(8).Padding(20).Children(func() {
			ui.Icon(c, icons.TablerFileDiff).Width(32).Height(32).TextColor(tc.IconMuted)
			ui.Text(c, "No changes yet").FontSize(13).FontWeight(600).TextColor(tc.TextMuted)
			ui.Text(c, "Files modified in the workspace show up here with their diffs.").FontSize(12).TextColor(tc.TextMuted).MaxWidth(240)
			if ContrastCTA(c, tc, "Refresh changes").Clicked() && a.Git != nil {
				a.Git.Refresh()
			}
		})
		return
	}

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

	ui.Column(c).Grow(1).FillHeight().Children(func() {
		ui.Scroll(c).Height(minF(220, float32(len(files))*28+8)).Children(func() {
			ui.Column(c).Padding(4, 8).Gap(1).Children(func() {
				for _, f := range files {
					f := f
					isSel := a.SelectedDiffPath == f.Path
					row := ui.Row(c.Key("chg-"+f.Path)).Height(28).Radius(6).Padding(0, 8).Gap(6).AlignItems(ui.Center)
					row.Children(func() {
						ui.Icon(c, icons.FileIconSVG(f.Path)).Width(13).Height(13).TextColor(tc.IconMuted)
						ui.Text(c, filepath.Base(f.Path)).FontSize(12).Grow(1).SingleLine()
						DiffStatText(c, tc, int(f.Additions), int(f.Deletions))
						badgeLabel := string(f.Status)
						badgeColor := tc.Accent
						if f.Staging == "staged" {
							badgeLabel = "S"
							badgeColor = tc.Success
						}
						Badge(c, tc, badgeLabel, badgeColor)

						stageBtn := ui.Row(c).Size(20, 20).Radius(4).AlignItems(ui.Center).Justify(ui.Center)
						stageBtn.Children(func() {
							if f.Staging == "staged" {
								ui.Icon(c, icons.TablerMinus).Width(12).Height(12).TextColor(tc.IconMuted)
							} else {
								ui.Icon(c, icons.TablerPlus).Width(12).Height(12).TextColor(tc.IconMuted)
							}
						})
						if stageBtn.Hovered() {
							stageBtn.Background(tc.Text.Alpha(0.06))
						}
						if stageBtn.Clicked() && a.Git != nil {
							if f.Staging == "staged" {
								go a.Git.Unstage(f.Path)
							} else {
								go a.Git.Stage(f.Path)
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

		// Diff viewer for selected file
		ui.Column(c).Grow(1).FillHeight().Children(func() {
			diffTitle := a.SelectedDiffPath
			header := ui.Row(c).Height(36).Padding(0, 10).Gap(6).AlignItems(ui.Center).
				BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Background(tc.Surface)
			header.Children(func() {
				ui.Icon(c, icons.TablerGitCompare).Width(13).Height(13).TextColor(tc.IconMuted)
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
					mygoeditor.DiffView(c, d).Grow(1).FillWidth()
				}
			}
		})
	})
}

// renderSideChatPane renders a secondary chat thread for quick questions.
func (a *AppUI) renderSideChatPane(c *ui.Context) {
	tc := a.ThemeMgr.Colors()
	a.SideChatMd.Theme = a.ThemeMgr.MarkdownTheme()

	ui.Row(c).Height(40).Padding(0, 12).AlignItems(ui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
		ui.Text(c, "Side Chat").FontSize(13).FontWeight(600)
		ui.Box(c).Grow(1)
		ui.Text(c, a.SideChatModel).FontSize(10).TextColor(tc.TextMuted)
	})

	if len(a.SideChatMessages) == 0 {
		ui.Column(c).Grow(1).Center().Gap(8).Padding(20).Children(func() {
			ui.Icon(c, icons.TablerMessage).Width(28).Height(28).TextColor(tc.IconMuted)
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

	ui.Row(c).Padding(8, 10).Gap(6).AlignItems(ui.Center).
		BorderWidth(1, 0, 0, 0).BorderColor(tc.Border).Children(func() {
		ui.TextInput(c, &a.SideChatInput).Placeholder("Ask side chat...").Grow(1)
		sendBtn := ui.Row(c).Size(28, 28).Radius(999).AlignItems(ui.Center).Justify(ui.Center).
			Background(tc.MessageAction)
		sendBtn.Children(func() {
			ui.Icon(c, icons.TablerSend).Width(13).Height(13).TextColor(tc.MessageActionForeground)
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

func (a *AppUI) sendSideChat(text string) {
	if a.SideChatStreaming {
		return
	}
	a.SideChatMessages = append(a.SideChatMessages, &agent.ChatMessage{Role: "user", Text: text})
	reply := &agent.ChatMessage{Role: "assistant", Text: "", Done: false}
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

// renderBoardPane renders the kanban workflow board.
func (a *AppUI) renderBoardPane(c *ui.Context) {
	tc := a.ThemeMgr.Colors()
	ui.Row(c).Height(40).Padding(0, 12).AlignItems(ui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
		ui.Text(c, "Workflow Board").FontSize(13).FontWeight(600)
		ui.Box(c).Grow(1)
		if IconButton(c, tc, icons.TablerRefresh, 12).Clicked() {
			a.RefreshBoard()
		}
	})
	if a.Board == nil {
		a.RefreshBoard()
	}
	if a.Board == nil || a.Board.Data == nil || len(a.Board.Data.Columns) == 0 {
		ui.Column(c).Grow(1).Center().Gap(8).Padding(20).Children(func() {
			ui.Icon(c, icons.TablerLayoutKanban).Width(32).Height(32).TextColor(tc.IconMuted)
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

// renderGitPane renders the full Source Control pane with Changes, History, and Branches tabs.
func (a *AppUI) renderGitPane(c *ui.Context) {
	gm := a.Git
	if gm == nil {
		return
	}
	t := c.Theme()

	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Gap(10).Padding(8, 14).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			ui.Text(c, "Source Control").Bold().FontSize(13)
			if gm.Branch != "" {
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					TablerIcon(c, icons.TablerGitBranch, 13)
					ui.Text(c, gm.Branch).FontSize(11).Bold().TextColor(t.Accent)
				})
			}
			ui.Box(c).Grow(1)
			refreshBtn := ui.Button(c, "")
			refreshBtn.Children(func() {
				TablerIcon(c, icons.TablerRefresh, 13)
			})
			if refreshBtn.Clicked() {
				go gm.Refresh()
			}
		})

		// Subtabs: Changes, History, Branches
		ui.Row(c).Gap(4).Padding(6, 12).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			subtabs := []struct {
				Tab   int
				Label string
				Icon  *ui.SVG
			}{
				{git.GitTabChanges, fmt.Sprintf("Changes (%d)", len(gm.Staged)+len(gm.Unstaged)+len(gm.Untracked)), icons.TablerFileDiff},
				{git.GitTabHistory, "History", icons.TablerHistory},
				{git.GitTabBranches, "Branches", icons.TablerGitBranch},
			}
			for _, st := range subtabs {
				thisTab := st
				var btn ui.Element
				if gm.ActiveTab == thisTab.Tab {
					btn = ui.PrimaryButton(c, "")
				} else {
					btn = ui.Button(c, "")
				}
				btn.Children(func() {
					ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
						TablerIcon(c, thisTab.Icon, 12)
						ui.Text(c, thisTab.Label).FontSize(11).Bold()
					})
				})
				if btn.Clicked() {
					gm.ActiveTab = thisTab.Tab
				}
			}
		})

		switch gm.ActiveTab {
		case git.GitTabChanges:
			a.renderChangesPane(c)
		case git.GitTabHistory:
			ui.Scroll(c).Grow(1).Padding(8).Children(func() {
				for _, commit := range gm.Commits {
					cm := commit
					ui.Column(c.Key("commit-"+cm.Hash)).Padding(6, 8).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
						ui.Text(c, cm.Message).FontSize(12).Bold().SingleLine()
						ui.Row(c).Gap(8).Children(func() {
							ui.Text(c, cm.AuthorName).FontSize(10).TextColor(t.TextMuted)
							ui.Text(c, cm.ShortHash).FontSize(10).TextColor(t.Accent)
						})
					})
				}
			})
		case git.GitTabBranches:
			ui.Scroll(c).Grow(1).Padding(8).Children(func() {
				for _, b := range gm.Branches {
					branch := b
					row := ui.Row(c.Key("branch-"+branch)).Padding(6, 8).Radius(6).Gap(6).AlignItems(ui.Center)
					row.Children(func() {
						TablerIcon(c, icons.TablerGitBranch, 13)
						ui.Text(c, branch).FontSize(12).Grow(1)
					})
					if branch == gm.Branch {
						row.Background(t.Accent)
					}
					if row.Clicked() {
						a.checkoutBranch(branch)
					}
				}
			})
		}
	})
}

// renderDatabasePane renders SQLite / DuckDB inspector and query console.
func (a *AppUI) renderDatabasePane(c *ui.Context) {
	dm := a.Database
	if dm == nil {
		return
	}
	t := c.Theme()

	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Gap(12).Padding(8, 16).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				TablerIcon(c, icons.TablerDatabase, 15)
				ui.Text(c, "Database Viewer").Bold().FontSize(14)
			})
			if dm.Info != nil {
				ui.Text(c, fmt.Sprintf("%s (%s)", dm.Info.FileName, dm.Info.Type)).FontSize(12).TextColor(t.Accent)
			}
			ui.Box(c).Grow(1)
			rescanBtn := ui.Button(c, "")
			rescanBtn.Children(func() {
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					TablerIcon(c, icons.TablerRefresh, 13)
					ui.Text(c, "Rescan").FontSize(11)
				})
			})
			if rescanBtn.Clicked() {
				go dm.ScanDatabases()
			}
		})

		if len(dm.Databases) == 0 {
			ui.Column(c).Grow(1).Center().Children(func() {
				TablerIcon(c, icons.TablerDatabase, 28)
				ui.Text(c, "No SQLite / DuckDB databases detected").FontSize(15).Bold().TextColor(t.TextMuted).Padding(6, 0, 0, 0)
				ui.Text(c, "Add a .sqlite, .sqlite3, .db, or .duckdb file to your workspace.").TextColor(t.TextMuted)
			})
			return
		}

		ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
			// Left: Table list
			ui.Column(c).Width(200).Background(t.Surface).BorderWidth(0, 1, 0, 0).BorderColor(t.Border).Children(func() {
				ui.Row(c).Padding(10, 12).Children(func() {
					ui.Text(c, "TABLES & VIEWS").FontSize(11).Bold().TextColor(t.TextMuted)
				})

				if dm.Info != nil {
					ui.Scroll(c).Grow(1).Padding(4, 8).Children(func() {
						for _, tb := range dm.Info.Tables {
							tName := tb.Name
							row := ui.Row(c.Key("db-table-"+tName)).Padding(6, 8).Radius(6).Gap(6).AlignItems(ui.Center)
							row.Children(func() {
								TablerIcon(c, icons.TablerTable, 13)
								ui.Text(c, tName).FontSize(12).Grow(1)
								ui.Text(c, fmt.Sprintf("%d", tb.RowCount)).FontSize(11).TextColor(t.TextMuted)
							})
							if tName == dm.ActiveTable {
								row.Background(t.Accent)
							} else if row.Hovered() {
								row.Background(t.Border)
							}
							if row.Clicked() {
								go dm.SelectTable(tName)
							}
						}
					})
				}
			})

			// Right: SQL console
			ui.Column(c).Grow(1).FillHeight().Children(func() {
				ui.Column(c).Padding(10, 16).Gap(8).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
					in := ui.TextInput(c, &dm.SQLQuery).Placeholder("SELECT * FROM table LIMIT 25;").Grow(1)
					if in.Submitted() && !dm.IsExecuting {
						go dm.ExecuteQuery()
					}

					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						runBtn := ui.PrimaryButton(c, "")
						runBtn.Children(func() {
							ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
								TablerIcon(c, icons.TablerPlayerPlay, 13)
								ui.Text(c, "Execute SQL").FontSize(11).Bold()
							})
						})
						if runBtn.Clicked() {
							go dm.ExecuteQuery()
						}
						ui.Box(c).Grow(1)
						if dm.StatusMsg != "" {
							ui.Text(c, dm.StatusMsg).FontSize(12).TextColor(t.TextMuted)
						}
					})
				})

				if dm.QueryResult != nil && len(dm.QueryResult.Columns) > 0 {
					ui.Scroll(c).Grow(1).Padding(8, 16).Children(func() {
						ui.Row(c).Padding(6, 8).Background(t.Surface).Radius(6).Children(func() {
							for _, col := range dm.QueryResult.Columns {
								ui.Text(c, col).Bold().FontSize(12).Width(120)
							}
						})

						for i, r := range dm.QueryResult.Rows {
							ui.Row(c.Key(fmt.Sprintf("dbrow-%d", i))).Padding(4, 8).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
								for _, col := range dm.QueryResult.Columns {
									valStr := fmt.Sprintf("%v", r[col])
									if len(valStr) > 40 {
										valStr = valStr[:37] + "…"
									}
									ui.Text(c, valStr).FontSize(12).Width(120)
								}
							})
						}
					})
				}
			})
		})
	})
}

func (a *AppUI) checkoutBranch(branch string) {
	if a.Git == nil {
		return
	}
	go func() {
		if err := a.Git.SwitchBranch(branch); err == nil {
			a.Workspace.GitBranch = branch
			a.Workspace.Refresh()
			if a.Window != nil {
				a.Window.Invalidate()
			}
		}
	}()
	a.ShowBranchModal = false
}

func (a *AppUI) createBranch(branch string) {
	if a.Git == nil || strings.TrimSpace(branch) == "" {
		return
	}
	go func() {
		if err := a.Git.CreateBranch(strings.TrimSpace(branch)); err == nil {
			a.Workspace.GitBranch = branch
			a.Workspace.Refresh()
			if a.Window != nil {
				a.Window.Invalidate()
			}
		}
	}()
	a.ShowBranchModal = false
}

func minF(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}
