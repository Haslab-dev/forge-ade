package ui

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Haslab-dev/RakitKode/llm"
	"github.com/egoist/mygo/ui"
	mygoeditor "github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/hasdev/forge-ade/app/native/pkg/agent"
	"github.com/hasdev/forge-ade/app/native/pkg/git"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/settings"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
	internalgit "github.com/hasdev/forge-ade/core/git"
)

// gitOpsBusy guards git mutations (commit, stage/unstage all) triggered from
// the render path: only one runs at a time, and buttons render a busy state
// until the background goroutine publishes completion via runOnUI.
var gitOpsBusy atomic.Bool

func (a *AppUI) rightDrawerTabMeta(dirtyFiles int) (title string, icon *ui.SVG, count int) {
	switch a.RightDrawerTab {
	case RightTabFiles:
		return "Files", icons.TablerFolder, 0
	case RightTabBrowser:
		return "Browser", icons.TablerWorld, 0
	case RightTabTerminal:
		return "Terminal", icons.TablerTerminal, 0
	case RightTabChanges:
		return "Review", icons.TablerFileDiff, dirtyFiles
	case RightTabTasks:
		return "Plans", icons.TablerChecklist, 0
	case RightTabChat:
		return "Side conversation", icons.TablerMessage, 0
	case RightTabSubagent:
		subCount := 0
		if a.Agent != nil && a.Agent.ActiveSessionID != "" {
			subCount = len(a.Agent.Subagents(a.Agent.ActiveSessionID))
		}
		return "Agents", icons.TablerBot, subCount
	case RightTabDeck:
		return "Presentations", icons.TablerPresentation, 0
	case RightTabDatabase:
		return "Database", icons.TablerDatabase, 0
	case RightTabGit:
		return "Git", icons.TablerGitBranch, dirtyFiles
	case RightTabSideFile:
		name := filepath.Base(a.ActiveSideFile)
		if name == "" || name == "." {
			name = "File"
		}
		return name, icons.FileIconSVG(a.ActiveSideFile), 0
	default:
		return "New tab", icons.TablerFileText, 0
	}
}

// renderRightDrawer renders ZCode's full-height collapsible Right Sidebar with
// the 42px multi-tool tab strip and the New tab launcher / active tool view.
func (a *AppUI) renderRightDrawer(c *ui.Context) {
	tc := a.ThemeMgr.Colors()

	// Responsive clamping for small displays. The max matches the resize
	// handle's clamp (min(750, winW*0.6)) so dragging feels 1:1; the 280
	// floor keeps controls usable on tiny windows.
	drawerWidth := a.RightDrawerWidth
	if winW, _ := c.Size(); winW > 0 {
		maxAllowed := minF(750, winW*0.6)
		if drawerWidth > maxAllowed {
			drawerWidth = maxAllowed
		}
		if drawerWidth < 280 {
			drawerWidth = 280
		}
	}

	dirtyFiles, _, _ := a.gitDirtySummary()
	tabTitle, tabIcon, tabCount := a.rightDrawerTabMeta(dirtyFiles)

	bar := c.TitleBar()
	rightPad := float32(10)
	if bar.Right > 0 {
		rightPad = max(bar.Right+10, 135)
	} else if bar.Left == 0 && (runtime.GOOS == "linux" || runtime.GOOS == "windows") {
		rightPad = 135
	}

	ui.Column(c).Width(drawerWidth).Shrink(0).FillHeight().Background(tc.Canvas).
		BorderWidth(0, 0, 0, 1).BorderColor(tc.Border).Children(func() {
		// ZCode Right Sidebar Header Bar (42px, aligned with WorkspaceTopBarHeight)
		ui.Row(c).Height(WorkspaceTopBarHeight).Padding(0, rightPad, 0, 8).Gap(6).AlignItems(ui.Center).
			Background(tc.Chrome).BorderWidth(0, 0, 1, 0).BorderColor(tc.ToolbarBorder).DragWindow().Children(func() {
			if IconButton(c, tc, icons.TablerChevronsDownUp, 13).Shrink(0).Clicked() {
				a.RightDrawerTab = RightTabLauncher
			}

			// Active tab pill + "+" launcher shortcut inside horizontal strip
			ui.ScrollHorizontal(c).Grow(1).Scrollbars(false).Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					pill := ui.Row(c.Key("rtab-active")).Height(26).Radius(6).
						Padding(0, 8).Gap(6).AlignItems(ui.Center).
						Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border)
					pill.Children(func() {
						ui.Icon(c, tabIcon).Width(13).Height(13).TextColor(tc.IconMuted)
						ui.Text(c, tabTitle).FontSize(12).FontWeight(500).TextColor(tc.Text).MaxWidth(150).SingleLine()
						if tabCount > 0 {
							ui.Row(c).Height(15).Radius(4).Padding(0, 4).AlignItems(ui.Center).
								Background(tc.Accent.Alpha(0.15)).Children(func() {
								ui.Text(c, fmt.Sprintf("%d", tabCount)).FontSize(9).FontWeight(600).TextColor(tc.Accent)
							})
						}
						tabX := ui.Row(c.Key("rtab-x-active")).Size(16, 16).Radius(4).
							AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
						tabX.Children(func() {
							ui.Icon(c, icons.TablerX).Width(10).Height(10).TextColor(tc.TextMuted)
						})
						if tabX.Hovered() {
							tabX.Background(tc.Text.Alpha(0.1))
						}
						if tabX.Clicked() {
							switch a.RightDrawerTab {
							case RightTabSideFile:
								a.closeSideFile(a.ActiveSideFile)
								if len(a.SideFileTabs) == 0 {
									a.RightDrawerTab = RightTabLauncher
								}
							case RightTabBrowser:
								a.closeBrowserTab()
								a.RightDrawerTab = RightTabLauncher
							case RightTabLauncher:
								a.ShowRightDrawer = false
							default:
								a.RightDrawerTab = RightTabLauncher
							}
						}
					})

					if IconButton(c, tc, icons.TablerPlus, 13).Shrink(0).Clicked() {
						a.RightDrawerTab = RightTabLauncher
					}
				})
			})

			expandIcon := icons.TablerMaximize2
			if a.RightDrawerWidth > 420 {
				expandIcon = icons.TablerMinimize2
			}
			if IconButton(c, tc, expandIcon, 13).Shrink(0).Clicked() {
				if a.RightDrawerWidth > 420 {
					a.RightDrawerWidth = RightDrawerDefault
				} else {
					a.RightDrawerWidth = 540
				}
			}

			if IconButton(c, tc, icons.TablerPanelRight, 14).Shrink(0).Clicked() {
				a.ShowRightDrawer = false
			}
		})

		// Drawer Content
		ui.Column(c).Grow(1).FillHeight().Children(func() {
			switch a.RightDrawerTab {
			case RightTabChanges:
				a.renderChangesPane(c)
			case RightTabTerminal:
				if a.ShowBottomTerminal {
					ui.Column(c).Grow(1).Center().Padding(16).Gap(8).Children(func() {
						ui.Icon(c, icons.TablerTerminal).Width(24).Height(24).TextColor(tc.IconMuted)
						ui.Text(c, "Terminal open in bottom panel").FontSize(12).FontWeight(500).TextColor(tc.Text)
						moveBtn := ui.Row(c.Key("move-term-drawer")).Height(26).Radius(6).Padding(0, 10).AlignItems(ui.Center).
							Background(tc.Accent.Alpha(0.14)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.35)).Cursor(ui.CursorPointer)
						moveBtn.Children(func() {
							ui.Text(c, "Move to Right Drawer").FontSize(11).FontWeight(600).TextColor(tc.Accent)
						})
						if moveBtn.Clicked() {
							a.ShowBottomTerminal = false
						}
					})
					break
				}
				if a.Terminal != nil {
					ui.Row(c).Height(32).Padding(0, 10).Gap(6).AlignItems(ui.Center).
						BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
						a.renderTerminalTabsStrip(c, tc, a.Terminal, "drawer-term")
						if IconButton(c, tc, icons.TablerPlus, 11).Clicked() {
							_ = a.Terminal.NewTab("")
						}
						askBtn := ui.Row(c.Key("drawer-term-ask")).Height(22).Radius(4).Padding(0, 6).Gap(4).AlignItems(ui.Center).
							Background(tc.Accent.Alpha(0.12)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.35))
						askBtn.Children(func() {
							ui.Icon(c, icons.TablerSparkles).Width(10).Height(10).TextColor(tc.Accent)
							ui.Text(c, "Ask Agent").FontSize(10).FontWeight(600).TextColor(tc.Accent)
						})
						if askBtn.Clicked() {
							a.askAgentFromTerminal()
						}

						snipBtn := ui.Row(c.Key("drawer-term-snip")).Height(22).Radius(4).Padding(0, 6).Gap(4).AlignItems(ui.Center).
							Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
						snipBtn.Children(func() {
							ui.Icon(c, icons.TablerBook).Width(10).Height(10).TextColor(tc.IconMuted)
							ui.Text(c, "Snippets").FontSize(10).TextColor(tc.Text)
						})
						if snipBtn.Clicked() {
							sessID := ""
							if a.Terminal != nil {
								sessID = a.Terminal.ActiveSessionID
							}
							a.OpenTerminalSnippetsModal(sessID, "", "")
						}

						if IconButton(c, tc, icons.TablerEraser, 11).Clicked() {
							a.Terminal.SendInput("\x0c")
						}
						if IconButton(c, tc, icons.TablerRefresh, 11).Clicked() {
							_ = a.Terminal.Restart()
						}
					})
					a.renderTerminal(c, a.Terminal)
				}
			case RightTabChat:
				a.renderSideChatPane(c)
			case RightTabTasks:
				a.renderDrawerPlansPane(c, tc)
			case RightTabGit:
				a.renderGitPane(c)
			case RightTabDatabase:
				a.renderDatabasePane(c)
			case RightTabFiles:
				a.renderFilesPane(c)
			case RightTabSideFile:
				a.renderSideFilePane(c)
			case RightTabBrowser:
				a.renderBrowserPane(c)
			case RightTabSubagent:
				a.renderSubagentPane(c)
			case RightTabDeck:
				a.renderDeckPane(c)
			default:
				a.renderRightDrawerLauncher(c, tc)
			}
		})
	})
}

func (a *AppUI) renderDrawerPlansPane(c *ui.Context, tc theme.ThemeColors) {
	ui.Scroll(c).Grow(1).Padding(16, 18).Children(func() {
		ui.Column(c).Gap(10).Children(func() {
			a.renderInspectorBody(c, tc, false)
		})
	})
}

func (a *AppUI) renderRightDrawerLauncher(c *ui.Context, tc theme.ThemeColors) {
	ui.Scroll(c).Grow(1).Padding(16, 18).Children(func() {
		ui.Column(c).Gap(14).Children(func() {
			ui.Text(c, "New tab").FontSize(15).FontWeight(600)

			launcherCard := func(key, title, desc string, icon *ui.SVG, tileBg, tileFg ui.Color, bordered bool, onOpen func()) {
				row := ui.Row(c.Key("rlaunch-"+key)).Padding(6, 6).Radius(10).Gap(12).AlignItems(ui.Center).Cursor(ui.CursorPointer)
				if row.Hovered() {
					row.Background(tc.Text.Alpha(0.04))
				}
				openClicked := false
				row.Children(func() {
					tile := ui.Row(c).Size(36, 36).Radius(8).AlignItems(ui.Center).Justify(ui.Center).Shrink(0).Background(tileBg)
					if bordered {
						tile.BorderWidth(1).BorderColor(tc.Border)
					}
					tile.Children(func() {
						ui.Icon(c, icon).Width(17).Height(17).TextColor(tileFg)
					})
					ui.Column(c).Gap(2).Grow(1).MinWidth(0).Children(func() {
						ui.Text(c, title).FontSize(13).FontWeight(600).SingleLine()
						ui.Text(c, desc).FontSize(11).TextColor(tc.TextMuted).SingleLine()
					})
					openBtn := ui.Row(c.Key("rlaunch-open-"+key)).Height(26).Radius(999).Padding(0, 12).
						AlignItems(ui.Center).Justify(ui.Center).Shrink(0).
						Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border).Cursor(ui.CursorPointer)
					openBtn.Children(func() {
						ui.Text(c, "Open").FontSize(11).FontWeight(500).TextColor(tc.Text)
					})
					if openBtn.Hovered() {
						openBtn.Background(tc.Text.Alpha(0.08))
					}
					if openBtn.Clicked() {
						openClicked = true
						onOpen()
					}
				})
				if !openClicked && row.Clicked() {
					onOpen()
				}
			}

			ui.Column(c).Gap(4).Children(func() {
				launcherCard("files", "Files", "Select a file from the navigation", icons.TablerFolder, tc.SurfaceRaised, tc.IconMuted, true, func() {
					a.RightDrawerTab = RightTabFiles
				})
				launcherCard("browser", "Browser", "Browse websites and preview apps.", icons.TablerWorld, tc.SurfaceRaised, tc.IconMuted, true, func() {
					a.openBrowserTab("")
				})
				launcherCard("terminal", "Terminal", "Run commands and view output.", icons.TablerTerminal, tc.SurfaceRaised, tc.IconMuted, true, func() {
					a.ShowBottomTerminal = false
					a.RightDrawerTab = RightTabTerminal
				})
				launcherCard("review", "Review", "Review code changes and diffs.", icons.TablerFileDiff, tc.SurfaceRaised, tc.IconMuted, true, func() {
					a.RightDrawerTab = RightTabChanges
				})
				launcherCard("plans", "Plans & Progress", "View active plans, tasks, and sub-agents.", icons.TablerChecklist, tc.SurfaceRaised, tc.IconMuted, true, func() {
					a.RightDrawerTab = RightTabTasks
				})
				launcherCard("sidechat", "Side conversation", "Continue a conversation in the side pane.", icons.TablerMessage, tc.SurfaceRaised, tc.IconMuted, true, func() {
					a.RightDrawerTab = RightTabChat
				})
				launcherCard("subagents", "Child Agents", "Inspect and steer spawned sub-agents.", icons.TablerBot, tc.SurfaceRaised, tc.IconMuted, true, func() {
					a.RightDrawerTab = RightTabSubagent
				})
				launcherCard("ssh", "Remote SSH / SFTP", "Connect to remote server, manage remote files, and open remote terminal.", icons.TablerServer, tc.Accent.Alpha(0.12), tc.Accent, false, func() {
					a.OpenSSHModal()
				})
			})

			ui.Box(c).FillWidth().Height(1).Background(tc.Border.Alpha(0.6))

			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "App plugins").FontSize(13).FontWeight(600).Grow(1)
				viewAll := ui.Row(c.Key("rlaunch-plugins-all")).Gap(3).Radius(6).Padding(3, 6).AlignItems(ui.Center).Cursor(ui.CursorPointer)
				viewAll.Children(func() {
					ui.Text(c, "View all plugins").FontSize(11).TextColor(tc.TextMuted)
					ui.Icon(c, icons.TablerChevronRight).Width(12).Height(12).TextColor(tc.TextMuted)
				})
				if viewAll.Hovered() {
					viewAll.Background(tc.Text.Alpha(0.05))
				}
				if viewAll.Clicked() {
					a.openSettings(settings.SettingsTabPlugins)
				}
			})

			ui.Column(c).Gap(4).Children(func() {
				launcherCard("pdf", "PDF", "PDF", icons.TablerFileText, tc.Error.Alpha(0.14), tc.Error, false, func() {
					a.RightDrawerTab = RightTabFiles
				})
				launcherCard("presentations", "Presentations", "Presentations", icons.TablerPresentation, tc.Warning.Alpha(0.14), tc.Warning, false, func() {
					a.RightDrawerTab = RightTabDeck
				})
				launcherCard("database", "Database", "SQLite & DuckDB inspector", icons.TablerDatabase, tc.Info.Alpha(0.14), tc.Info, false, func() {
					a.RightDrawerTab = RightTabDatabase
				})
			})
		})
	})
}

// renderChangesPane renders working tree changes and unified diff viewer.
func (a *AppUI) renderChangesPane(c *ui.Context) {
	tc := a.ThemeMgr.Colors()
	gm := a.Git

	// No workspace opened — nothing for git to report.
	if a.Workspace.RootDir == "" {
		ui.Column(c).Grow(1).Center().Gap(8).Children(func() {
			ui.Icon(c, icons.TablerFolder).Width(22).Height(22).TextColor(tc.IconMuted)
			ui.Text(c, "No folder opened").FontSize(12).TextColor(tc.TextMuted)
		})
		return
	}

	var conflicts, staged, unstaged []internalgit.FileStatus
	var snap git.GitSnapshot
	if gm != nil {
		// Render from a point-in-time snapshot: Refresh/Stage/Commit goroutines
		// write the exported fields under the manager's lock.
		snap = gm.Snapshot()
		conflicts = snap.Conflicts
		staged = snap.Staged
		unstaged = append(unstaged, snap.Unstaged...)
		unstaged = append(unstaged, snap.Untracked...)
	}

	totalFiles := len(conflicts) + len(staged) + len(unstaged)

	ui.Row(c).Height(36).Padding(0, 12).AlignItems(ui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
		ui.Text(c, "Working Tree").FontSize(12).FontWeight(600)
		ui.Box(c).Grow(1)
		if gm != nil && gitOpsBusy.Load() {
			ui.Spinner(c).Size(12, 12).Label("Git")
		} else if IconButton(c, tc, icons.TablerRefresh, 12).Clicked() && gm != nil {
			gm.RefreshAsync()
		}
		ui.Text(c, fmt.Sprintf("%d changes", totalFiles)).FontSize(11).TextColor(tc.TextMuted)
	})

	if totalFiles == 0 {
		ui.Column(c).Grow(1).Center().Gap(8).Padding(20).Children(func() {
			ui.Icon(c, icons.TablerFileDiff).Width(32).Height(32).TextColor(tc.IconMuted)
			ui.Text(c, "No changes detected").FontSize(13).FontWeight(600).TextColor(tc.TextMuted)
			ui.Text(c, "Working tree is clean. Modified files will appear here.").FontSize(12).TextColor(tc.TextMuted).MaxWidth(240)
			if ContrastCTA(c, tc, "Refresh changes").Clicked() && gm != nil {
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

	ui.Column(c).Grow(1).FillHeight().Children(func() {
		// File lists (Conflicts, Staged & Unstaged)
		ui.Scroll(c).Height(minF(200, float32(totalFiles)*28+44)).Children(func() {
			ui.Column(c).Padding(4, 6).Gap(4).Children(func() {
				// 0. Merge Conflicts section
				if len(conflicts) > 0 {
					ui.Row(c).Padding(2, 4).AlignItems(ui.Center).Children(func() {
						ui.Text(c, fmt.Sprintf("MERGE CONFLICTS (%d)", len(conflicts))).FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.Error)
					})
					for _, f := range conflicts {
						f := f
						isSel := a.SelectedDiffPath == f.Path
						acted := false
						row := ui.Row(c.Key("conflict-"+f.Path)).Height(26).Radius(6).Padding(0, 6).Gap(4).AlignItems(ui.Center)
						row.Children(func() {
							ui.Icon(c, icons.TablerAlertTriangle).Width(12).Height(12).TextColor(tc.Error)
							ui.Text(c, filepath.Base(f.Path)).FontSize(11).Grow(1).SingleLine()
							oursBtn := ui.Row(c.Key("ours-"+f.Path)).Height(18).Radius(4).Padding(0, 5).AlignItems(ui.Center).
								Background(tc.Accent.Alpha(0.14)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.35))
							oursBtn.Children(func() {
								ui.Text(c, "Ours").FontSize(9).FontWeight(600).TextColor(tc.Accent)
							})
							if oursBtn.Clicked() && gm != nil {
								acted = true
								go func(p string) {
									_ = gm.ResolveConflict(p, "ours")
								}(f.Path)
							}
							theirsBtn := ui.Row(c.Key("theirs-"+f.Path)).Height(18).Radius(4).Padding(0, 5).AlignItems(ui.Center).
								Background(tc.Warning.Alpha(0.14)).BorderWidth(1).BorderColor(tc.Warning.Alpha(0.35))
							theirsBtn.Children(func() {
								ui.Text(c, "Theirs").FontSize(9).FontWeight(600).TextColor(tc.Warning)
							})
							if theirsBtn.Clicked() && gm != nil {
								acted = true
								go func(p string) {
									_ = gm.ResolveConflict(p, "theirs")
								}(f.Path)
							}
						})
						if isSel {
							row.Background(tc.Error.Alpha(0.14))
						} else if row.Hovered() {
							row.Background(tc.Text.Alpha(0.04))
						}
						if !acted && row.Clicked() {
							a.SelectedDiffPath = f.Path
						}
					}
				}
				// 1. Staged section
				if len(staged) > 0 {
					ui.Row(c).Padding(2, 4).AlignItems(ui.Center).Children(func() {
						ui.Text(c, fmt.Sprintf("STAGED CHANGES (%d)", len(staged))).FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted)
						ui.Box(c).Grow(1)
						unstageAllBtn := ui.Row(c).Radius(4).Padding(1, 6).AlignItems(ui.Center)
						unstageAllBtn.Children(func() {
							ui.Text(c, "Unstage All").FontSize(10).TextColor(tc.TextMuted)
						})
						if unstageAllBtn.Hovered() && !gitOpsBusy.Load() {
							unstageAllBtn.Background(tc.Text.Alpha(0.06))
						}
						if unstageAllBtn.Clicked() && gm != nil && gitOpsBusy.CompareAndSwap(false, true) {
							go func() {
								gm.UnstageAll()
								a.runOnUI(func() {
									gitOpsBusy.Store(false)
								})
							}()
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
						if stageAllBtn.Hovered() && !gitOpsBusy.Load() {
							stageAllBtn.Background(tc.Text.Alpha(0.06))
						}
						if stageAllBtn.Clicked() && gm != nil && gitOpsBusy.CompareAndSwap(false, true) {
							go func() {
								gm.StageAll()
								a.runOnUI(func() {
									gitOpsBusy.Store(false)
								})
							}()
						}
					})
					for _, f := range unstaged {
						f := f
						isSel := a.SelectedDiffPath == f.Path
						acted := false
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

							if gm != nil && snap.DiscardPendingPath == f.Path {
								confirmBtn := ui.Row(c.Key("discard-ok-"+f.Path)).Height(18).Radius(4).Padding(0, 5).AlignItems(ui.Center).
									Background(tc.Error.Alpha(0.18)).BorderWidth(1).BorderColor(tc.Error.Alpha(0.45))
								confirmBtn.Children(func() {
									ui.Text(c, "Sure?").FontSize(9).FontWeight(700).TextColor(tc.Error)
								})
								if confirmBtn.Clicked() {
									acted = true
									go gm.Discard(f.Path)
								}
								cancelBtn := ui.Row(c.Key("discard-no-"+f.Path)).Size(18, 18).Radius(4).AlignItems(ui.Center).Justify(ui.Center)
								cancelBtn.Children(func() {
									ui.Icon(c, icons.TablerX).Width(9).Height(9).TextColor(tc.IconMuted)
								})
								if cancelBtn.Clicked() {
									acted = true
									gm.DiscardPendingPath = ""
								}
							} else {
								discardBtn := ui.Row(c.Key("discard-"+f.Path)).Size(18, 18).Radius(4).AlignItems(ui.Center).Justify(ui.Center)
								discardBtn.Children(func() {
									ui.Icon(c, icons.TablerRotate).Width(10).Height(10).TextColor(tc.IconMuted)
								})
								if discardBtn.Hovered() {
									discardBtn.Background(tc.Error.Alpha(0.12))
								}
								if discardBtn.Clicked() && gm != nil {
									acted = true
									gm.DiscardPendingPath = f.Path
								}
							}

							stageBtn := ui.Row(c).Size(18, 18).Radius(4).AlignItems(ui.Center).Justify(ui.Center)
							stageBtn.Children(func() {
								ui.Icon(c, icons.TablerPlus).Width(10).Height(10).TextColor(tc.IconMuted)
							})
							if stageBtn.Hovered() {
								stageBtn.Background(tc.Text.Alpha(0.06))
							}
							if stageBtn.Clicked() && gm != nil {
								acted = true
								go gm.Stage(f.Path)
							}
						})
						if isSel {
							row.Background(tc.Accent.Alpha(0.14))
						} else if row.Hovered() {
							row.Background(tc.Text.Alpha(0.04))
						}
						if !acted && row.Clicked() {
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

					// Commit button. Stage/commit run on a background goroutine
					// (they can take seconds) guarded by gitOpsBusy; results
					// and toasts publish back through runOnUI.
					busy := gitOpsBusy.Load()
					canCommit := strings.TrimSpace(snap.CommitSummary) != "" && totalFiles > 0
					branchName := snap.Branch
					if branchName == "" {
						branchName = "main"
					}
					btnText := fmt.Sprintf("Commit to %s", branchName)
					commitBtn := ui.Row(c).Height(26).Radius(6).Padding(0, 10).AlignItems(ui.Center).Justify(ui.Center)
					switch {
					case busy:
						commitBtn.Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border.Alpha(0.5))
						commitBtn.Children(func() {
							ui.Spinner(c).Size(11, 11).Label("Committing…")
						})
					case canCommit:
						commitBtn.Background(tc.Accent).BorderWidth(1).BorderColor(tc.Accent)
						commitBtn.Children(func() {
							ui.Icon(c, icons.TablerCheck).Width(11).Height(11).TextColor(tc.AccentForeground)
							ui.Text(c, btnText).FontSize(11).FontWeight(600).TextColor(tc.AccentForeground)
						})
						if commitBtn.Clicked() && gitOpsBusy.CompareAndSwap(false, true) {
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
										gm.SetCommitMessage("", "", "")
										// Drop the stale selection so the next change
										// set auto-selects a file that still exists.
										a.SelectedDiffPath = ""
									}
								})
							}()
						}
					default:
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

			repoDir := a.Workspace.RootDir
			if repoDir == "" && gm != nil {
				repoDir = snap.RootDir
			}
			if repoDir == "" && a.Agent != nil {
				repoDir = a.Agent.WorkspacePath()
			}
			patch := ""
			diffReady := true
			if diffTitle != "" {
				if gm != nil {
					patch, diffReady = gm.GetCachedFileDiff(diffTitle)
				} else if a.Engine != nil {
					patch = a.Engine.GetGitDiff(repoDir, diffTitle)
				}
			}
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
	a.SideChatMd.Reveal = &markdown.Reveal{Unit: markdown.AtOnce, Fade: -1}

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
			// Quick-start prompts, as the webview side chat's empty state.
			ui.Column(c).Gap(4).Padding(6, 0, 0, 0).Children(func() {
				for _, qp := range []string{
					"Explain all changed files",
					"Check for potential edge cases",
					"How can I test the modified code?",
				} {
					qp := qp
					chip := ui.Row(c.Key("scq-"+qp)).Height(26).Radius(6).Padding(0, 9).AlignItems(ui.Center).
						Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
					chip.Children(func() {
						ui.Text(c, qp).FontSize(11)
					})
					if chip.Hovered() {
						chip.Background(tc.Text.Alpha(0.05))
					}
					if chip.Clicked() {
						a.SideChatInput = qp
						a.sendSideChat(qp)
					}
				}
			})
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
	root := a.Workspace.RootDir
	// The stream callback runs on an LLM goroutine: buffer the text locally
	// and publish every delta through runOnUI so render never races reply.Text.
	go func() {
		streamed := ""
		defer func() {
			final := streamed
			a.runOnUI(func() {
				reply.Text = final
				reply.Done = true
				a.SideChatStreaming = false
			})
		}()
		if engine == nil || engine.LLMClient == nil {
			streamed = "No LLM provider configured. Set one up in Settings → Models & Providers."
			return
		}
		msgs := []llm.LLMMessage{{
			Role:    "user",
			Content: "Workspace: " + root + "\n\n" + text,
		}}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		_, err := engine.LLMClient.ChatWithStream(ctx, msgs, nil, func(deltaContent, deltaReasoning string) {
			if deltaContent != "" {
				streamed += deltaContent
				partial := streamed
				a.runOnUI(func() {
					reply.Text = partial
				})
			}
		})
		if err != nil && streamed == "" {
			streamed = fmt.Sprintf("LLM request failed: %v", err)
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
		// Render from a point-in-time snapshot; Refresh/Fetch/Pull/Push run on
		// goroutines and publish under the manager's lock.
		snap := gm.Snapshot()

		// Header row with Branch pill + Remotes (Fetch, Pull, Push) + Refresh
		ui.Row(c).Height(40).Padding(0, 10).Gap(6).AlignItems(ui.Center).
			BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
			ui.Text(c, "Source Control").FontSize(12).FontWeight(600)

			if snap.Branch != "" {
				branchPill := ui.Row(c).Height(24).Radius(4).Padding(0, 6).Gap(4).AlignItems(ui.Center).
					Background(tc.Accent.Alpha(0.12)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.3))
				branchPill.Children(func() {
					ui.Icon(c, icons.TablerGitBranch).Width(11).Height(11).TextColor(tc.Accent)
					ui.Text(c, snap.Branch).FontSize(11).FontWeight(600).TextColor(tc.Accent)
				})
				if branchPill.Clicked() {
					a.ShowBranchModal = true
				}
			}

			ui.Box(c).Grow(1)

			// Remote op buttons: spinner while their goroutine is running.
			remoteBtn := func(label string, icon *ui.SVG, busy bool, run func() error, doneTitle, doneType string) {
				b := ui.Row(c).Height(24).Radius(4).Padding(0, 6).Gap(3).AlignItems(ui.Center).
					Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border.Alpha(0.6))
				b.Children(func() {
					if busy {
						ui.Spinner(c).Size(10, 10).Label(label)
					} else {
						ui.Icon(c, icon).Width(10).Height(10).TextColor(tc.IconMuted)
					}
					ui.Text(c, label).FontSize(10).TextColor(tc.Text)
				})
				if b.Hovered() && !busy {
					b.Background(tc.Text.Alpha(0.06))
				}
				if b.Clicked() && !busy {
					go func() {
						err := run()
						if err != nil {
							a.queueToast(ui.Toast{Title: label + " Error", Description: err.Error(), Type: "error"})
						} else {
							a.queueToast(ui.Toast{Title: doneTitle, Type: doneType})
						}
					}()
				}
			}
			remoteBtn("Fetch", icons.TablerArrowDown, snap.IsFetching, func() error { return gm.Fetch() }, "Fetched Remotes", "info")
			remoteBtn("Pull", icons.TablerArrowDown, snap.IsPulling, func() error { return gm.Pull() }, "Pull Completed", "success")
			remoteBtn("Push", icons.TablerArrowUp, snap.IsPushing, func() error { return gm.Push() }, "Pushed to Remote", "success")

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
				gm.RefreshAsync()
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
				{git.GitTabChanges, fmt.Sprintf("Changes (%d)", len(snap.Conflicts)+len(snap.Staged)+len(snap.Unstaged)+len(snap.Untracked)), icons.TablerFileDiff},
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
				ui.Scroll(c).Height(minF(220, float32(len(snap.Commits))*38+10)).Children(func() {
					ui.Column(c).Padding(4, 6).Gap(2).Children(func() {
						for _, commit := range snap.Commits {
							cm := commit
							isSel := snap.SelectedCommitHash == cm.Hash
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
				if snap.SelectedCommitHash != "" {
					ui.Column(c).Grow(1).FillHeight().Children(func() {
						ui.Row(c).Height(28).Padding(0, 8).Gap(6).AlignItems(ui.Center).
							Background(tc.SurfaceRaised).BorderWidth(1, 0, 1, 0).BorderColor(tc.Border.Alpha(0.5)).Children(func() {
							ui.Icon(c, icons.TablerGitCommit).Width(11).Height(11).TextColor(tc.Accent)
							shortH := snap.SelectedCommitHash
							if len(shortH) > 8 {
								shortH = shortH[:8]
							}
							ui.Text(c, fmt.Sprintf("Commit Diff (%s)", shortH)).FontSize(11).FontWeight(600).TextColor(tc.Text)
						})

						if snap.LoadingCommitDiff {
							ui.Column(c).Grow(1).Center().Children(func() {
								ui.Text(c, "Loading commit diff...").FontSize(11).TextColor(tc.TextMuted)
							})
						} else if snap.SelectedCommitDiff != "" {
							d := a.Editor.OpenPatchDiff("commit.diff", snap.SelectedCommitDiff, a.ThemeMgr)
							if d != nil {
								mygoeditor.DiffView(c, d).Grow(1).FillWidth()
							}
						} else {
							ui.Column(c).Grow(1).Center().Children(func() {
								ui.Text(c, "No diff in this commit").FontSize(11).TextColor(tc.TextMuted)
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
								a.queueToast(ui.Toast{Title: "Branch Error", Description: err.Error(), Type: "error"})
							} else {
								a.queueToast(ui.Toast{Title: "Branch Created", Description: name, Type: "success"})
							}
						}()
					}
				})
				ui.Scroll(c).Grow(1).Padding(8).Children(func() {
					ui.Column(c).Gap(2).Children(func() {
						for _, b := range snap.Branches {
							branch := b
							isCurrent := branch == snap.Branch
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

		// Database file switcher strip when workspace contains database files.
		if len(dm.Databases) > 0 {
			ui.Row(c).Height(32).Padding(0, 12).Gap(6).AlignItems(ui.Center).
				BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Background(t.Surface).Children(func() {
				ui.Text(c, "DB:").FontSize(10).Bold().TextColor(t.TextMuted)
				ui.ScrollHorizontal(c).Grow(1).Scrollbars(false).Children(func() {
					ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
						for _, dbPath := range dm.Databases {
							p := dbPath
							isSel := (p == dm.ActiveDBPath)
							pill := ui.Row(c.Key("db-switch-"+p)).Height(22).Radius(4).Padding(0, 8).AlignItems(ui.Center)
							pill.Children(func() {
								txt := ui.Text(c, filepath.Base(p)).FontSize(11)
								if isSel {
									txt.Bold().TextColor(a.ThemeMgr.Colors().AccentForeground)
								}
							})
							if isSel {
								pill.Background(t.Accent)
							} else if pill.Hovered() {
								pill.Background(t.Border)
							}
							pill.Tooltip(p)
							if pill.Clicked() && !isSel {
								go dm.Inspect(p)
							}
						}
					})
				})
			})
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
	gm := a.Git
	if gm == nil {
		return
	}
	go func() {
		if err := gm.SwitchBranch(branch); err == nil {
			// Publish state mutations on the UI thread; the workspace rescan
			// (potentially slow over remote file systems) stays off-thread.
			a.runOnUI(func() {
				a.Workspace.GitBranch = branch
			})
			a.Workspace.Refresh()
			if a.Window != nil {
				a.Window.Invalidate()
			}
		}
	}()
	a.ShowBranchModal = false
}

func (a *AppUI) createBranch(branch string) {
	gm := a.Git
	if gm == nil || strings.TrimSpace(branch) == "" {
		return
	}
	go func() {
		if err := gm.CreateBranch(strings.TrimSpace(branch)); err == nil {
			a.runOnUI(func() {
				a.Workspace.GitBranch = branch
			})
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

// renderSideFilePane draws the right sidebar's file preview tabs — files
// opened from the agent sidebar's explorer, as forge-ade's side preview:
// a tab strip plus the file in the MyGo editor view.
func (a *AppUI) renderSideFilePane(c *ui.Context) {
	tc := a.ThemeMgr.Colors()
	if len(a.SideFileTabs) == 0 {
		ui.Column(c).Grow(1).Center().Children(func() {
			ui.Text(c, "No file preview open").FontSize(12).TextColor(tc.TextMuted)
		})
		return
	}

	// File tab strip (forge-ade's sideFileTabs mirrored into pane tabs).
	ui.ScrollHorizontal(c).Height(34).Padding(0, 8).Gap(4).AlignItems(ui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
		for _, p := range a.SideFileTabs {
			thisPath := p
			active := a.ActiveSideFile == thisPath
			tab := ui.Row(c.Key("sidefile-"+thisPath)).Height(24).Radius(6).Padding(0, 8).Gap(5).AlignItems(ui.Center)
			tab.Children(func() {
				ui.Icon(c, icons.FileIconSVG(thisPath)).Width(12).Height(12).TextColor(tc.IconMuted)
				txt := ui.Text(c, filepath.Base(thisPath)).FontSize(11).SingleLine()
				if active {
					txt.FontWeight(600).TextColor(tc.Text)
				} else {
					txt.TextColor(tc.TextMuted)
				}
				// Right-click: close / close others / close to the right,
				// as AgentRightSidebar's tab context menu.
				tab.ContextMenu(func(m *ui.Menu) {
					if m.Item("Close").Chosen() {
						a.closeSideFile(thisPath)
					}
					if m.Item("Close Others").Chosen() {
						a.SideFileTabs = []string{thisPath}
						a.ActiveSideFile = thisPath
					}
					if m.Item("Close to the Right").Chosen() {
						for i, t := range a.SideFileTabs {
							if t == thisPath {
								a.SideFileTabs = a.SideFileTabs[:i+1]
								break
							}
						}
						found := false
						for _, t := range a.SideFileTabs {
							if t == a.ActiveSideFile {
								found = true
								break
							}
						}
						if !found {
							a.ActiveSideFile = thisPath
						}
					}
				})
				closeBtn := ui.Row(c.Key("sidefile-x-"+thisPath)).Size(14, 14).Radius(4).AlignItems(ui.Center).Justify(ui.Center)
				closeBtn.Children(func() {
					ui.Icon(c, icons.TablerX).Width(9).Height(9).TextColor(tc.IconMuted)
				})
				if closeBtn.Hovered() {
					closeBtn.Background(tc.Text.Alpha(0.08))
				}
				if closeBtn.Clicked() {
					a.closeSideFile(thisPath)
				}
			})
			if active {
				tab.Background(tc.Accent.Alpha(0.14))
			} else if tab.Hovered() {
				tab.Background(tc.Text.Alpha(0.05))
			}
			if tab.Clicked() {
				a.ActiveSideFile = thisPath
			}
		}
	})

	// Special preview modes (markdown) render instead of the buffer.
	if a.ActiveSideFile == a.SideFilePreviewMd {
		ui.Column(c).Grow(1).FillHeight().Children(func() {
			a.renderSideFilePreviewBody(c, tc, func() {
				ui.Column(c).Grow(1).Center().Children(func() {
					ui.Text(c, "File unavailable").FontSize(12).TextColor(tc.TextMuted)
				})
			})
		})
		return
	}

	full := ""
	if a.Workspace != nil {
		full = a.Workspace.ResolveFullPath(a.ActiveSideFile)
	}

	// Image and SVG viewer preview in right panel
	if isImageOrSVG(a.ActiveSideFile) && a.SVGViewMode[full] != "code" {
		doc := a.Editor.GetDoc(full)
		a.renderImageViewer(c, tc, full, a.ActiveSideFile, doc, true)
		return
	}

	// Native PDF viewer preview in side file tabs
	if strings.HasSuffix(strings.ToLower(a.ActiveSideFile), ".pdf") {
		a.renderPdfViewerPane(c, tc, full, filepath.Base(a.ActiveSideFile))
		return
	}

	// Presentation & Motion preview in side file tabs
	if isDeckOrMotionFile(a.ActiveSideFile) {
		a.renderDeckPane(c)
		return
	}

	// Editor view of the active file.
	doc := a.Editor.GetDoc(full)
	if doc == nil {
		ui.Column(c).Grow(1).Center().Children(func() {
			ui.Text(c, "File unavailable").FontSize(12).TextColor(tc.TextMuted)
		})
		return
	}
	// Diff mode: show the file's git working-tree diff instead of the
	// content, as forge-ade's sideFileDiffMode toggle does.
	diffMode := a.SideFileDiffMode[a.ActiveSideFile]
	ui.Column(c).Grow(1).FillHeight().Children(func() {
		if diffMode {
			repoDir := a.Workspace.RootDir
			if repoDir == "" && a.Git != nil {
				repoDir = a.Git.Snapshot().RootDir
			}
			if repoDir == "" && a.Agent != nil {
				repoDir = a.Agent.WorkspacePath()
			}
			patch := ""
			diffReady := true
			if a.Git != nil {
				patch, diffReady = a.Git.GetCachedFileDiff(a.ActiveSideFile)
			} else if a.Engine != nil {
				patch = a.Engine.GetGitDiff(repoDir, a.ActiveSideFile)
			}
			if !diffReady {
				ui.Column(c).Grow(1).Center().Children(func() {
					ui.Spinner(c).Size(14, 14).Label("Loading diff")
				})
			} else if patch == "" {
				ui.Column(c).Grow(1).Center().Children(func() {
					ui.Text(c, "No uncommitted changes").FontSize(12).TextColor(tc.TextMuted)
				})
			} else if d := a.Editor.OpenPatchDiff(a.ActiveSideFile, patch, a.ThemeMgr); d != nil {
				mygoeditor.DiffView(c, d).Grow(1).FillWidth()
			}
		} else {
			edView := mygoeditor.View(c, doc.Editor).Grow(1).FillWidth()
			a.attachEditorContextMenu(c, edView, doc)
		}
		// Keep the drawer preview in sync with the editor-mode active tab.
		ui.Row(c).Height(26).Padding(0, 12).Gap(6).AlignItems(ui.Center).
			BorderWidth(1, 0, 0, 0).BorderColor(tc.Border).Children(func() {
			ui.Text(c, a.ActiveSideFile).FontSize(10).TextColor(tc.TextMuted).SingleLine()
			ui.Box(c).Grow(1)
			diffToggle := ui.Text(c, "Diff")
			if diffMode {
				diffToggle.FontWeight(600).TextColor(tc.Accent)
			} else {
				diffToggle.FontSize(10).TextColor(tc.TextMuted)
			}
			if diffToggle.Clicked() {
				a.SideFileDiffMode[a.ActiveSideFile] = !diffMode
			}
			if ui.Text(c, "Open in Editor").FontSize(10).TextColor(tc.Accent).Padding(0, 0, 0, 10).Clicked() {
				_, _ = a.Editor.OpenFile(full, a.ActiveSideFile)
				a.Mode = ModeEditor
			}
		})
	})
}

// renderHunkRevertStrip lists the working-tree hunks of a file with per-hunk
// Revert buttons (GetGitFileDiffHunks / RevertGitHunk parity) above the diff.
func (a *AppUI) renderHunkRevertStrip(c *ui.Context, tc theme.ThemeColors, gm *git.GitManager, path, patch string) {
	if gm == nil || path == "" || patch == "" {
		return
	}
	var hunkHeaders []string
	for _, ln := range strings.Split(patch, "\n") {
		if strings.HasPrefix(ln, "@@ ") {
			hunkHeaders = append(hunkHeaders, strings.TrimSpace(ln))
		}
	}
	if len(hunkHeaders) == 0 {
		return
	}
	if len(a.HunkRevertPaths) == 0 {
		a.HunkRevertPaths = make(map[string]bool)
	}
	strip := ui.Column(c.Key("hunks-"+path)).Gap(2).Padding(4, 8).
		BorderWidth(1, 1, 0, 1).BorderColor(tc.Border.Alpha(0.5)).
		Background(tc.SurfaceRaised.Alpha(0.3))
	strip.Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, fmt.Sprintf("Hunks (%d)", len(hunkHeaders))).FontSize(10).FontWeight(700).TextColor(tc.TextMuted)
			ui.Box(c).Grow(1)
			ui.Text(c, "revert restores the hunk to HEAD").FontSize(9).TextColor(tc.SecondaryLabel)
		})
		for i, header := range hunkHeaders {
			i, header := i, header
			row := ui.Row(c.Key(fmt.Sprintf("hunk-%s-%d", path, i))).Height(24).Radius(6).Padding(0, 6).Gap(6).AlignItems(ui.Center)
			row.Children(func() {
				ui.Text(c, header).FontSize(10).Font("monospace").TextColor(tc.Accent).Grow(1).SingleLine()
				revertKey := path + "#" + fmt.Sprint(i)
				reverted := a.HunkRevertPaths[revertKey]
				if reverted {
					ui.Text(c, "reverted").FontSize(9).TextColor(tc.Success)
				} else {
					btn := ui.Row(c).Height(20).Radius(4).Padding(0, 7).AlignItems(ui.Center).
						Background(tc.Warning.Alpha(0.14)).BorderWidth(1).BorderColor(tc.Warning.Alpha(0.4))
					btn.Children(func() {
						ui.Text(c, "Revert").FontSize(9).FontWeight(600).TextColor(tc.Warning)
					})
					if btn.Clicked() {
						a.runOnUI(func() {
							a.HunkRevertPaths[revertKey] = true
						})
						go func() {
							if rerr := gm.RevertDiffHunk(path, i); rerr != nil {
								// Roll the marker back on the UI thread so the
								// render pass never races the revert goroutine.
								a.runOnUI(func() {
									a.HunkRevertPaths[revertKey] = false
								})
							}
						}()
					}
				}
			})
			if row.Hovered() {
				row.Background(tc.Text.Alpha(0.04))
			}
		}
	})
}
