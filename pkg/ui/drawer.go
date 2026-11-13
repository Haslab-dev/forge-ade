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

	// Responsive clamping for small displays
	drawerWidth := a.RightDrawerWidth
	if winW, _ := c.Size(); winW > 0 {
		maxAllowed := winW * 0.55
		if drawerWidth > maxAllowed {
			drawerWidth = maxAllowed
		}
		if drawerWidth < 280 {
			drawerWidth = 280
		}
	}

	ui.Column(c).Width(drawerWidth).Background(tc.Canvas).BorderWidth(0, 0, 0, 1).BorderColor(tc.Border).Children(func() {
		// Drawer Header Tabs with ScrollHorizontal so it never overflows or cuts off
		ui.Row(c).Height(WorkspaceTopBarHeight).Padding(0, 6).AlignItems(ui.Center).
			BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
			tabs := []struct {
				Tab  int
				Name string
				Icon *ui.SVG
			}{
				{RightTabChanges, "Changes", icons.TablerFileDiff},
				{RightTabTerminal, "Terminal", icons.TablerTerminal},
				{RightTabChat, "Chat", icons.TablerMessage},
				{RightTabFiles, "Files", icons.TablerFolder},
			}

			ui.ScrollHorizontal(c).Grow(1).Gap(3).AlignItems(ui.Center).Children(func() {
				for _, tab := range tabs {
					tTab := tab
					active := a.RightDrawerTab == tTab.Tab
					pill := ui.Row(c.Key(fmt.Sprintf("rtab-%d", tTab.Tab))).Height(26).Radius(6).
						Padding(0, 8).Gap(4).AlignItems(ui.Center)
					pill.Children(func() {
						icColor := tc.IconMuted
						if active {
							icColor = tc.Accent
						}
						ui.Icon(c, tTab.Icon).Width(12).Height(12).TextColor(icColor)
						txt := ui.Text(c, tTab.Name).FontSize(11)
						if active {
							txt.FontWeight(600).TextColor(tc.Text)
						} else {
							txt.TextColor(tc.TextMuted)
						}
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
			})

			ui.Box(c).Width(4)

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
			case RightTabFiles:
				a.renderFilesPane(c)
			}
		})
	})
}

// renderChangesPane renders working tree changes and unified diff viewer.
func (a *AppUI) renderChangesPane(c *ui.Context) {
	tc := a.ThemeMgr.Colors()
	gm := a.Git

	var staged []internalgit.FileStatus
	var unstaged []internalgit.FileStatus
	if gm != nil {
		staged = gm.Staged
		unstaged = append(unstaged, gm.Unstaged...)
		unstaged = append(unstaged, gm.Untracked...)
	}

	totalFiles := len(staged) + len(unstaged)

	ui.Row(c).Height(36).Padding(0, 12).AlignItems(ui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
		ui.Text(c, "Working Tree").FontSize(12).FontWeight(600)
		ui.Box(c).Grow(1)
		if IconButton(c, tc, icons.TablerRefresh, 12).Clicked() && gm != nil {
			gm.Refresh()
		}
		ui.Text(c, fmt.Sprintf("%d changes", totalFiles)).FontSize(11).TextColor(tc.TextMuted)
	})

	if totalFiles == 0 {
		ui.Column(c).Grow(1).Center().Gap(8).Padding(20).Children(func() {
			ui.Icon(c, icons.TablerFileDiff).Width(32).Height(32).TextColor(tc.IconMuted)
			ui.Text(c, "No changes detected").FontSize(13).FontWeight(600).TextColor(tc.TextMuted)
			ui.Text(c, "Working tree is clean. Modified files will appear here.").FontSize(12).TextColor(tc.TextMuted).MaxWidth(240)
			if ContrastCTA(c, tc, "Refresh changes").Clicked() && gm != nil {
				gm.Refresh()
			}
		})
		return
	}

	if a.SelectedDiffPath == "" {
		if len(staged) > 0 {
			a.SelectedDiffPath = staged[0].Path
		} else if len(unstaged) > 0 {
			a.SelectedDiffPath = unstaged[0].Path
		}
	}

	ui.Column(c).Grow(1).FillHeight().Children(func() {
		// File lists (Staged & Unstaged)
		ui.Scroll(c).Height(minF(180, float32(totalFiles)*28+40)).Children(func() {
			ui.Column(c).Padding(4, 6).Gap(4).Children(func() {
				// 1. Staged section
				if len(staged) > 0 {
					ui.Row(c).Padding(2, 4).AlignItems(ui.Center).Children(func() {
						ui.Text(c, fmt.Sprintf("STAGED CHANGES (%d)", len(staged))).FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted)
						ui.Box(c).Grow(1)
						unstageAllBtn := ui.Row(c).Radius(4).Padding(1, 6).AlignItems(ui.Center)
						unstageAllBtn.Children(func() {
							ui.Text(c, "Unstage All").FontSize(10).TextColor(tc.TextMuted)
						})
						if unstageAllBtn.Hovered() {
							unstageAllBtn.Background(tc.Text.Alpha(0.06))
						}
						if unstageAllBtn.Clicked() && gm != nil {
							gm.UnstageAll()
						}
					})
					for _, f := range staged {
						f := f
						isSel := a.SelectedDiffPath == f.Path
						row := ui.Row(c.Key("staged-"+f.Path)).Height(26).Radius(6).Padding(0, 6).Gap(4).AlignItems(ui.Center)
						row.Children(func() {
							ui.Icon(c, icons.FileIconSVG(f.Path)).Width(12).Height(12).TextColor(tc.IconMuted)
							ui.Text(c, filepath.Base(f.Path)).FontSize(11).Grow(1).SingleLine()
							DiffStatText(c, tc, int(f.Additions), int(f.Deletions))
							Badge(c, tc, "S", tc.Success)

							unstageBtn := ui.Row(c).Size(18, 18).Radius(4).AlignItems(ui.Center).Justify(ui.Center)
							unstageBtn.Children(func() {
								ui.Icon(c, icons.TablerMinus).Width(10).Height(10).TextColor(tc.IconMuted)
							})
							if unstageBtn.Hovered() {
								unstageBtn.Background(tc.Text.Alpha(0.06))
							}
							if unstageBtn.Clicked() && gm != nil {
								go gm.Unstage(f.Path)
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
				}

				// 2. Unstaged / Untracked section
				if len(unstaged) > 0 {
					ui.Row(c).Padding(4, 4, 2, 4).AlignItems(ui.Center).Children(func() {
						ui.Text(c, fmt.Sprintf("CHANGES (%d)", len(unstaged))).FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted)
						ui.Box(c).Grow(1)
						stageAllBtn := ui.Row(c).Radius(4).Padding(1, 6).AlignItems(ui.Center)
						stageAllBtn.Children(func() {
							ui.Text(c, "Stage All").FontSize(10).TextColor(tc.TextMuted)
						})
						if stageAllBtn.Hovered() {
							stageAllBtn.Background(tc.Text.Alpha(0.06))
						}
						if stageAllBtn.Clicked() && gm != nil {
							gm.StageAll()
						}
					})
					for _, f := range unstaged {
						f := f
						isSel := a.SelectedDiffPath == f.Path
						row := ui.Row(c.Key("unstaged-"+f.Path)).Height(26).Radius(6).Padding(0, 6).Gap(4).AlignItems(ui.Center)
						row.Children(func() {
							ui.Icon(c, icons.FileIconSVG(f.Path)).Width(12).Height(12).TextColor(tc.IconMuted)
							ui.Text(c, filepath.Base(f.Path)).FontSize(11).Grow(1).SingleLine()
							DiffStatText(c, tc, int(f.Additions), int(f.Deletions))
							statusLabel := string(f.Status)
							if statusLabel == "" {
								statusLabel = "M"
							}
							Badge(c, tc, statusLabel, tc.Accent)

							stageBtn := ui.Row(c).Size(18, 18).Radius(4).AlignItems(ui.Center).Justify(ui.Center)
							stageBtn.Children(func() {
								ui.Icon(c, icons.TablerPlus).Width(10).Height(10).TextColor(tc.IconMuted)
							})
							if stageBtn.Hovered() {
								stageBtn.Background(tc.Text.Alpha(0.06))
							}
							if stageBtn.Clicked() && gm != nil {
								go gm.Stage(f.Path)
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
				}
			})
		})

		// Commit Form Card
		if gm != nil {
			ui.Column(c).Padding(8, 10).Gap(6).
				BorderWidth(1, 0, 1, 0).BorderColor(tc.Border.Alpha(0.6)).
				Background(tc.SurfaceRaised.Alpha(0.4)).Children(func() {
				ui.TextInput(c, &gm.CommitSummary).Placeholder("Summary (required)").Grow(1)
				ui.TextInput(c, &gm.CommitDesc).Placeholder("Description (optional)").Grow(1)

				ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
					// AI commit button
					aiBtn := ui.Row(c).Height(26).Radius(6).Padding(0, 8).Gap(4).AlignItems(ui.Center).
						Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
					aiBtn.Children(func() {
						ui.Icon(c, icons.TablerSparkles).Width(12).Height(12).TextColor(tc.Accent)
						ui.Text(c, "AI Message").FontSize(10).FontWeight(500).TextColor(tc.Text)
					})
					if aiBtn.Hovered() {
						aiBtn.Background(tc.Text.Alpha(0.06))
					}
					if aiBtn.Clicked() {
						go gm.GenerateAICommit("")
					}

					ui.Box(c).Grow(1)

					// Commit button
					canCommit := strings.TrimSpace(gm.CommitSummary) != "" && totalFiles > 0
					branchName := gm.Branch
					if branchName == "" {
						branchName = "main"
					}
					btnText := fmt.Sprintf("Commit to %s", branchName)
					commitBtn := ui.Row(c).Height(26).Radius(6).Padding(0, 10).AlignItems(ui.Center).Justify(ui.Center)
					if canCommit {
						commitBtn.Background(tc.Accent).BorderWidth(1).BorderColor(tc.Accent)
						commitBtn.Children(func() {
							ui.Icon(c, icons.TablerCheck).Width(11).Height(11).TextColor(ui.RGBA(255, 255, 255, 1.0))
							ui.Text(c, btnText).FontSize(11).FontWeight(600).TextColor(ui.RGBA(255, 255, 255, 1.0))
						})
						if commitBtn.Clicked() {
							if len(staged) == 0 && len(unstaged) > 0 {
								gm.StageAll()
							}
							fullMsg := gm.CommitSummary
							if strings.TrimSpace(gm.CommitDesc) != "" {
								fullMsg += "\n\n" + gm.CommitDesc
							}
							err := gm.Commit(fullMsg)
							if err != nil {
								c.AddToast(ui.Toast{Title: "Commit Error", Description: err.Error(), Type: "error"})
							} else {
								c.AddToast(ui.Toast{Title: "Committed", Description: gm.CommitSummary, Type: "success"})
								gm.CommitSummary = ""
								gm.CommitDesc = ""
							}
						}
					} else {
						commitBtn.Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border.Alpha(0.5))
						commitBtn.Children(func() {
							ui.Text(c, btnText).FontSize(11).TextColor(tc.TextMuted)
						})
					}
				})
			})
		}

		// Diff viewer for selected file
		ui.Column(c).Grow(1).FillHeight().Clip().Children(func() {
			diffTitle := a.SelectedDiffPath
			header := ui.Row(c).Height(30).Padding(0, 8).Gap(6).AlignItems(ui.Center).
				BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Background(tc.Surface)
			header.Children(func() {
				ui.Icon(c, icons.TablerGitCompare).Width(12).Height(12).TextColor(tc.IconMuted)
				ui.Text(c, filepath.Base(diffTitle)).FontSize(11).FontWeight(500).Grow(1).SingleLine()
				openBtn := ui.Row(c).Radius(4).Padding(2, 6).Gap(3).AlignItems(ui.Center)
				openBtn.Children(func() {
					ui.Icon(c, icons.TablerCode).Width(11).Height(11).TextColor(tc.TextMuted)
					ui.Text(c, "Open").FontSize(11).TextColor(tc.TextMuted)
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
						MaxWidth(maxF(180, a.RightDrawerWidth-48))
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
		side := ui.TextAreaBase(c, &a.SideChatInput).Lines(1, 5).Placeholder("Ask side chat...").Grow(1)
		side.OnTextKey(func(k ui.TextKey) bool {
			if k.Key == ui.KeyEnter && k.Mods == 0 {
				if strings.TrimSpace(a.SideChatInput) != "" {
					text := strings.TrimSpace(a.SideChatInput)
					a.SideChatInput = ""
					a.sendSideChat(text)
				}
				return true
			}
			return false
		})
		sendBtn := ui.Row(c).Size(28, 28).Radius(999).AlignItems(ui.Center).Justify(ui.Center).
			Background(tc.MessageAction)
		sendBtn.Children(func() {
			ui.Icon(c, icons.TablerSend).Width(13).Height(13).TextColor(tc.MessageActionForeground)
		})
		if sendBtn.Clicked() && strings.TrimSpace(a.SideChatInput) != "" {
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

// renderGitPane renders the full Source Control pane with Changes, History, and Branches tabs.
func (a *AppUI) renderGitPane(c *ui.Context) {
	gm := a.Git
	if gm == nil {
		return
	}
	tc := a.ThemeMgr.Colors()

	// Git operations run against the local working tree only, exactly as
	// forge-ade skips git for ssh:// workspaces.
	if strings.HasPrefix(a.Workspace.RootDir, "ssh://") {
		ui.Column(c).Grow(1).Center().Gap(8).Padding(20).Children(func() {
			ui.Icon(c, icons.TablerGitBranch).Width(32).Height(32).TextColor(tc.IconMuted)
			ui.Text(c, "Git is unavailable over SSH").FontSize(13).FontWeight(600).TextColor(tc.TextMuted)
			ui.Text(c, "Source control works on local workspaces only. Disconnect to use git.").FontSize(12).TextColor(tc.TextMuted).MaxWidth(240)
		})
		return
	}

	ui.Column(c).Fill().Children(func() {
		// Header row with Branch pill + Remotes (Fetch, Pull, Push) + Refresh
		ui.Row(c).Height(40).Padding(0, 10).Gap(6).AlignItems(ui.Center).
			BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
			ui.Text(c, "Source Control").FontSize(12).FontWeight(600)

			if gm.Branch != "" {
				branchPill := ui.Row(c).Height(24).Radius(4).Padding(0, 6).Gap(4).AlignItems(ui.Center).
					Background(tc.Accent.Alpha(0.12)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.3))
				branchPill.Children(func() {
					ui.Icon(c, icons.TablerGitBranch).Width(11).Height(11).TextColor(tc.Accent)
					ui.Text(c, gm.Branch).FontSize(11).FontWeight(600).TextColor(tc.Accent)
				})
				if branchPill.Clicked() {
					a.ShowBranchModal = true
				}
			}

			ui.Box(c).Grow(1)

			// Fetch button
			fetchBtn := ui.Row(c).Height(24).Radius(4).Padding(0, 6).Gap(3).AlignItems(ui.Center).
				Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border.Alpha(0.6))
			fetchBtn.Children(func() {
				ui.Icon(c, icons.TablerArrowDown).Width(10).Height(10).TextColor(tc.IconMuted)
				ui.Text(c, "Fetch").FontSize(10).TextColor(tc.Text)
			})
			if fetchBtn.Hovered() {
				fetchBtn.Background(tc.Text.Alpha(0.06))
			}
			if fetchBtn.Clicked() {
				go func() {
					_ = gm.Fetch()
					c.AddToast(ui.Toast{Title: "Fetched Remotes", Type: "info"})
				}()
			}

			// Pull button
			pullBtn := ui.Row(c).Height(24).Radius(4).Padding(0, 6).Gap(3).AlignItems(ui.Center).
				Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border.Alpha(0.6))
			pullBtn.Children(func() {
				ui.Icon(c, icons.TablerArrowDown).Width(10).Height(10).TextColor(tc.IconMuted)
				ui.Text(c, "Pull").FontSize(10).TextColor(tc.Text)
			})
			if pullBtn.Hovered() {
				pullBtn.Background(tc.Text.Alpha(0.06))
			}
			if pullBtn.Clicked() {
				go func() {
					err := gm.Pull()
					if err != nil {
						c.AddToast(ui.Toast{Title: "Pull Error", Description: err.Error(), Type: "error"})
					} else {
						c.AddToast(ui.Toast{Title: "Pull Completed", Type: "success"})
					}
				}()
			}

			// Push button
			pushBtn := ui.Row(c).Height(24).Radius(4).Padding(0, 6).Gap(3).AlignItems(ui.Center).
				Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border.Alpha(0.6))
			pushBtn.Children(func() {
				ui.Icon(c, icons.TablerArrowUp).Width(10).Height(10).TextColor(tc.IconMuted)
				ui.Text(c, "Push").FontSize(10).TextColor(tc.Text)
			})
			if pushBtn.Hovered() {
				pushBtn.Background(tc.Text.Alpha(0.06))
			}
			if pushBtn.Clicked() {
				go func() {
					err := gm.Push()
					if err != nil {
						c.AddToast(ui.Toast{Title: "Push Error", Description: err.Error(), Type: "error"})
					} else {
						c.AddToast(ui.Toast{Title: "Pushed to Remote", Type: "success"})
					}
				}()
			}

			// Refresh button
			refreshBtn := ui.Row(c).Size(24, 24).Radius(4).AlignItems(ui.Center).Justify(ui.Center).
				Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border.Alpha(0.6))
			refreshBtn.Children(func() {
				ui.Icon(c, icons.TablerRefresh).Width(11).Height(11).TextColor(tc.IconMuted)
			})
			if refreshBtn.Hovered() {
				refreshBtn.Background(tc.Text.Alpha(0.06))
			}
			if refreshBtn.Clicked() {
				go gm.Refresh()
			}
		})

		// Subtabs: Changes, History, Branches
		ui.Row(c).Height(34).Padding(0, 8).Gap(4).AlignItems(ui.Center).
			BorderWidth(0, 0, 1, 0).BorderColor(tc.Border.Alpha(0.6)).Children(func() {
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
				isSel := gm.ActiveTab == thisTab.Tab
				tabBtn := ui.Row(c.Key("subtab-"+thisTab.Label)).Height(24).Radius(4).Padding(0, 8).Gap(4).AlignItems(ui.Center)
				tabBtn.Children(func() {
					ui.Icon(c, thisTab.Icon).Width(11).Height(11).TextColor(tc.IconMuted)
					ui.Text(c, thisTab.Label).FontSize(11).FontWeight(500)
				})
				if isSel {
					tabBtn.Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border)
				}
				if tabBtn.Clicked() {
					gm.ActiveTab = thisTab.Tab
				}
			}
		})

		switch gm.ActiveTab {
		case git.GitTabChanges:
			a.renderChangesPane(c)
		case git.GitTabHistory:
			ui.Column(c).Grow(1).FillHeight().Children(func() {
				// Commit List
				ui.Scroll(c).Height(minF(220, float32(len(gm.Commits))*38+10)).Children(func() {
					ui.Column(c).Padding(4, 6).Gap(2).Children(func() {
						for _, commit := range gm.Commits {
							cm := commit
							isSel := gm.SelectedCommitHash == cm.Hash
							row := ui.Column(c.Key("commit-"+cm.Hash)).Radius(6).Padding(6, 8).Gap(2)
							row.Children(func() {
								ui.Text(c, cm.Message).FontSize(11).FontWeight(600).SingleLine().TextColor(tc.Text)
								ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
									ui.Text(c, cm.AuthorName).FontSize(10).TextColor(tc.TextMuted)
									ui.Box(c).Grow(1)
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

				// Commit Diff Viewer (when a commit is selected)
				if gm.SelectedCommitHash != "" {
					ui.Column(c).Grow(1).FillHeight().Children(func() {
						ui.Row(c).Height(28).Padding(0, 8).Gap(6).AlignItems(ui.Center).
							Background(tc.SurfaceRaised).BorderWidth(1, 0, 1, 0).BorderColor(tc.Border.Alpha(0.5)).Children(func() {
							ui.Icon(c, icons.TablerGitCommit).Width(11).Height(11).TextColor(tc.Accent)
							shortH := gm.SelectedCommitHash
							if len(shortH) > 8 {
								shortH = shortH[:8]
							}
							ui.Text(c, fmt.Sprintf("Commit Diff (%s)", shortH)).FontSize(11).FontWeight(600).TextColor(tc.Text)
						})

						if gm.SelectedCommitDiff != "" {
							d := a.Editor.OpenPatchDiff("commit.diff", gm.SelectedCommitDiff, a.ThemeMgr)
							if d != nil {
								mygoeditor.DiffView(c, d).Grow(1).FillWidth()
							}
						} else {
							ui.Column(c).Grow(1).Center().Children(func() {
								ui.Text(c, "Loading commit diff...").FontSize(11).TextColor(tc.TextMuted)
							})
						}
					})
				}
			})
		case git.GitTabBranches:
			ui.Column(c).Grow(1).Children(func() {
				// Create branch, as forge-ade's BranchSelector does
				ui.Row(c).Padding(8, 10, 4, 10).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.TextInput(c, &gm.NewBranchName).Placeholder("New branch name…").Grow(1)
					createBtn := ui.Row(c).Height(26).Radius(6).Padding(0, 10).Gap(4).AlignItems(ui.Center).
						Background(tc.Accent.Alpha(0.12)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.3))
					createBtn.Children(func() {
						ui.Icon(c, icons.TablerPlus).Width(11).Height(11).TextColor(tc.Accent)
						ui.Text(c, "Create").FontSize(10).FontWeight(600).TextColor(tc.Accent)
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
						}()
					}
				})
				ui.Scroll(c).Grow(1).Padding(8).Children(func() {
					ui.Column(c).Gap(2).Children(func() {
						for _, b := range gm.Branches {
							branch := b
							isCurrent := branch == gm.Branch
							row := ui.Row(c.Key("branch-"+branch)).Height(30).Radius(6).Padding(0, 8).Gap(6).AlignItems(ui.Center)
							row.Children(func() {
								ui.Icon(c, icons.TablerGitBranch).Width(12).Height(12).TextColor(tc.IconMuted)
								ui.Text(c, branch).FontSize(11).FontWeight(500).Grow(1).SingleLine()
								if isCurrent {
									Badge(c, tc, "HEAD", tc.Accent)
								}
							})
							if isCurrent {
								row.Background(tc.Accent.Alpha(0.12)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.3))
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

		renderSQLConsole := func() {
			ui.Column(c).Grow(1).FillHeight().Children(func() {
				ui.Column(c).Padding(8, 12).Gap(6).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
					in := ui.TextInput(c, &dm.SQLQuery).Placeholder("SELECT * FROM table LIMIT 25;").Grow(1)
					if in.Submitted() && !dm.IsExecuting {
						go dm.ExecuteQuery()
					}

					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						runBtn := ui.PrimaryButton(c, "")
						runBtn.Children(func() {
							ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
								TablerIcon(c, icons.TablerPlayerPlay, 12)
								ui.Text(c, "Execute").FontSize(11).Bold()
							})
						})
						if runBtn.Clicked() {
							go dm.ExecuteQuery()
						}
						ui.Box(c).Grow(1)
						if dm.StatusMsg != "" {
							ui.Text(c, dm.StatusMsg).FontSize(11).TextColor(t.TextMuted)
						}
					})
				})

				if dm.QueryResult != nil && len(dm.QueryResult.Columns) > 0 {
					ui.ScrollBoth(c).Grow(1).Padding(6, 10).Children(func() {
						ui.Row(c).Padding(4, 6).Background(t.Surface).Radius(4).Children(func() {
							for _, col := range dm.QueryResult.Columns {
								ui.Text(c, col).Bold().FontSize(11).Width(100)
							}
						})

						for i, r := range dm.QueryResult.Rows {
							ui.Row(c.Key(fmt.Sprintf("dbrow-%d", i))).Padding(3, 6).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
								for _, col := range dm.QueryResult.Columns {
									valStr := fmt.Sprintf("%v", r[col])
									if len(valStr) > 40 {
										valStr = valStr[:37] + "…"
									}
									ui.Text(c, valStr).FontSize(11).Width(100)
								}
							})
						}
					})
				}
			})
		}

		if a.RightDrawerWidth < 460 {
			// Stacked: table pills at top + SQL console below
			if dm.Info != nil && len(dm.Info.Tables) > 0 {
				ui.Row(c).Height(32).Padding(0, 8).AlignItems(ui.Center).Gap(6).
					BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Background(t.Surface).Children(func() {
					ui.Text(c, "TABLE:").FontSize(9).Bold().TextColor(t.TextMuted)
					ui.ScrollHorizontal(c).Grow(1).Gap(3).AlignItems(ui.Center).Children(func() {
						for _, tb := range dm.Info.Tables {
							tName := tb.Name
							isSel := (tName == dm.ActiveTable)
							pill := ui.Row(c.Key("db-pill-"+tName)).Height(22).Radius(4).Padding(0, 6).Gap(3).AlignItems(ui.Center)
							pill.Children(func() {
								ui.Text(c, tName).FontSize(11)
								ui.Text(c, fmt.Sprintf("%d", tb.RowCount)).FontSize(9).TextColor(t.TextMuted)
							})
							if isSel {
								pill.Background(t.Accent)
							} else if pill.Hovered() {
								pill.Background(t.Border)
							}
							if pill.Clicked() {
								go dm.SelectTable(tName)
							}
						}
					})
				})
			}
			renderSQLConsole()
		} else {
			// Side by side with compact tables column (130px)
			ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
				ui.Column(c).Width(130).Background(t.Surface).BorderWidth(0, 1, 0, 0).BorderColor(t.Border).Children(func() {
					ui.Row(c).Padding(8, 10).Children(func() {
						ui.Text(c, "TABLES").FontSize(10).Bold().TextColor(t.TextMuted)
					})

					if dm.Info != nil {
						ui.Scroll(c).Grow(1).Padding(2, 6).Children(func() {
							for _, tb := range dm.Info.Tables {
								tName := tb.Name
								row := ui.Row(c.Key("db-table-"+tName)).Padding(4, 6).Radius(4).Gap(4).AlignItems(ui.Center)
								row.Children(func() {
									TablerIcon(c, icons.TablerTable, 12)
									ui.Text(c, tName).FontSize(11).Grow(1).SingleLine()
									ui.Text(c, fmt.Sprintf("%d", tb.RowCount)).FontSize(10).TextColor(t.TextMuted)
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

				renderSQLConsole()
			})
		}
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

func maxF(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}
