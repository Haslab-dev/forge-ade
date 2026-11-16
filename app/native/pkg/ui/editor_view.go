package ui

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	mygoeditor "github.com/egoist/mygo/ui/editor"
	"github.com/hasdev/forge-ade/app/native/pkg/editor"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/terminal"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
)

// renderEditorSurface renders the Editor mode layout: ActivityBar + Sidebar, Editor Tabs & Buffers, Bottom Terminal, Right Drawer.
func (a *AppUI) renderEditorSurface(c *ui.Context) {
	tc := a.ThemeMgr.Colors()

	// 1. Left Activity Bar + Sidebar
	if a.ShowLeftSidebar {
		activityButton := func(icon *ui.SVG, tab int, badgeCount int, onPick func()) {
			active := false
			if tab == ActivityGit {
				active = a.IsGitClientOpen
			} else if tab >= 0 {
				active = (!a.IsGitClientOpen && a.EditorActivity == tab)
			} else if tab == -1 && a.Settings != nil && a.Settings.IsOpen {
				active = true
			}
			btn := ui.Row(c).Size(ActivityBarWidth-8, 40).Radius(6).AlignItems(ui.Center).Justify(ui.Center)
			btn.Children(func() {
				ic := ui.Icon(c, icon).Width(20).Height(20)
				if active {
					ic.TextColor(tc.Text)
				} else {
					ic.TextColor(tc.IconMuted)
				}
				if badgeCount > 0 {
					badgeText := fmt.Sprintf("%d", badgeCount)
					if badgeCount > 99 {
						badgeText = "99+"
					}
					ui.Row(c).Absolute().Top(3).Right(3).Height(14).Radius(7).
						Padding(0, 4).Background(tc.Accent).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
						ui.Text(c, badgeText).FontSize(9).FontWeight(700).TextColor(tc.Canvas)
					})
				}
			})
			if active {
				btn.Background(tc.Text.Alpha(0.06))
			} else if btn.Hovered() {
				btn.Background(tc.Text.Alpha(0.04))
			}
			if btn.Clicked() {
				if onPick != nil {
					onPick()
				} else {
					a.EditorActivity = tab
				}
			}
			if active {
				ui.Box(c).Absolute().Left(0).Width(2).Top(8).Bottom(8).Background(tc.Text).Radius(0, 1, 1, 0)
			}
		}

		gitChanges := 0
		if a.Git != nil {
			// Render from a snapshot: Refresh goroutines write these fields.
			gm := a.Git.Snapshot()
			gitChanges = len(gm.Staged) + len(gm.Unstaged) + len(gm.Untracked)
		}

		ui.Column(c).Width(ActivityBarWidth).Background(tc.Canvas).BorderWidth(0, 1, 0, 0).BorderColor(tc.Border).
			Padding(8, 0).AlignItems(ui.Center).Gap(4).Children(func() {
			activityButton(icons.TablerFiles, ActivityExplorer, 0, func() {
				a.IsGitClientOpen = false
				a.EditorActivity = ActivityExplorer
			})
			activityButton(icons.TablerSearch, ActivitySearch, len(a.FileSearchHits), func() {
				a.IsGitClientOpen = false
				a.EditorActivity = ActivitySearch
			})
			activityButton(icons.TablerGitBranch, ActivityGit, gitChanges, func() {
				if a.IsGitClientOpen {
					a.IsGitClientOpen = false
				} else {
					a.IsGitClientOpen = true
					if a.Git != nil {
						go a.Git.Refresh()
					}
					a.loadGitRemoteURL()
				}
			})
			activityButton(icons.TablerDatabase, ActivityDatabase, 0, func() {
				a.IsGitClientOpen = false
				a.EditorActivity = ActivityDatabase
			})
			ui.Box(c).Grow(1)

			// Remote SSH Button on ActivityBar
			isSSH := strings.HasPrefix(a.Workspace.RootDir, "ssh://")
			sshBtn := ui.Row(c).Size(ActivityBarWidth-8, 40).Radius(6).AlignItems(ui.Center).Justify(ui.Center)
			sshBtn.Children(func() {
				ic := ui.Icon(c, icons.TablerServer).Width(20).Height(20)
				if isSSH {
					ic.TextColor(tc.Accent)
					ui.Box(c).Absolute().Top(4).Right(4).Size(6, 6).Radius(3).Background(tc.Success)
				} else {
					ic.TextColor(tc.IconMuted)
				}
			})
			if isSSH {
				sshBtn.Background(tc.Accent.Alpha(0.12))
			} else if sshBtn.Hovered() {
				sshBtn.Background(tc.Text.Alpha(0.04))
			}
			if sshBtn.Clicked() {
				if isSSH {
					a.ShowSSHQuickMenu = true
				} else {
					a.OpenSSHModal()
				}
			}

			activityButton(icons.TablerSettings, -1, 0, func() {
				a.openSettings()
			})
		})

		if !a.IsGitClientOpen {
			ui.Column(c).Width(FileTreeDefaultWidth).Background(tc.Canvas).BorderWidth(0, 1, 0, 0).BorderColor(tc.Border).Children(func() {
				switch a.EditorActivity {
				case ActivityExplorer:
					a.renderFileTree(c)
				case ActivitySearch:
					a.renderFileSearch(c)
				case ActivityGit:
					a.renderGitPane(c)
				case ActivityDatabase:
					a.renderDatabasePane(c)
				}
			})
		}
	}

	// 2. Center Column: Editor Tabs + Virtualized Code Buffer / Diff +
	// Collapsible Bottom Terminal. The wrapping Box clips so non-wrapping
	// rows inside cannot freeze the column's minimum width and push the
	// right drawer off-screen (same flex-shrink protection as agent mode).
	ui.Box(c).Grow(1).FillHeight().Clip().Children(func() {
		ui.Column(c).Grow(1).FillHeight().Children(func() {
			if a.IsGitClientOpen {
				a.renderGitClientSurface(c)
			} else {
				ui.Column(c).Grow(1).FillHeight().Children(func() {
					a.renderEditorView(c)
				})
			}

			hasActiveEditorTerminal := a.Editor != nil && strings.HasPrefix(a.Editor.ActivePath, "terminal://")
			if a.ShowBottomTerminal && !hasActiveEditorTerminal {
				TopResizeHandle(c, &a.BottomTerminalHeight, 120, 600, 240)
				a.renderBottomTerminal(c)
			}
		})
	})

	// 3. Collapsible Right Drawer with interactive drag resize handle
	if a.ShowRightDrawer {
		maxRight := float32(750)
		if winW, _ := c.Size(); winW > 0 && winW*0.6 < maxRight {
			maxRight = winW * 0.6
		}
		LeftResizeHandle(c, &a.RightDrawerWidth, 280, maxRight, RightDrawerDefault)
		a.renderRightDrawer(c)
	}
}

// renderEditorTabBar renders open tabs and buffer utility buttons.
func (a *AppUI) renderEditorTabBar(c *ui.Context) {
	em := a.Editor
	if em == nil {
		return
	}
	tc := a.ThemeMgr.Colors()

	ui.Row(c).Height(EditorTabBarHeight).Padding(0, 8).AlignItems(ui.Center).
		Background(tc.Surface).BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
		ui.ScrollHorizontal(c).Grow(1).Scrollbars(false).Children(func() {
			ui.Row(c).Height(EditorTabBarHeight).AlignItems(ui.Center).Children(func() {
				for _, tabPath := range em.OpenTabs() {
					path := tabPath
					isTerm := strings.HasPrefix(path, "terminal://")
					var doc *editor.OpenDocument
					if !isTerm {
						doc = em.GetDoc(path)
						if doc == nil {
							continue
						}
					}

					isActive := (path == em.ActivePath)
					isModified := false
					if doc != nil && doc.Editor != nil && doc.Editor.Buffer != nil {
						isModified = doc.Editor.Buffer.Modified()
					}
					baseName := ""
					if isTerm {
						baseName = strings.TrimPrefix(path, "terminal://")
						if baseName == "" {
							baseName = "Terminal"
						}
					} else {
						baseName = filepath.Base(doc.RelativePath)
					}
					isPinned := em.IsPinned(path)
					isReadOnly := em.IsReadOnly(path)
					closedThisTab := false

					tab := ui.Row(c.Key("tab-"+path)).Height(EditorTabBarHeight).Padding(0, 10).Gap(6).
						AlignItems(ui.Center).BorderWidth(0, 1, 0, 0).BorderColor(tc.Border)
					tab.Children(func() {
						if isTerm {
							ui.Icon(c, icons.TablerTerminal).Width(13).Height(13).TextColor(tc.Accent)
						} else {
							ui.Icon(c, icons.FileIconSVG(doc.RelativePath)).Width(13).Height(13).TextColor(tc.IconMuted)
						}
						if isPinned {
							ui.Icon(c, icons.TablerPinFilled).Width(9).Height(9).TextColor(tc.Accent)
						}
						label := ui.Text(c, baseName).FontSize(12)
						if isActive {
							label.FontWeight(500)
						}
						if isReadOnly {
							// Read-only tabs render an italic title, as forge-ade does.
							label.Italic().TextColor(tc.TextMuted)
						}
						closeBtn := ui.Row(c.Key("tab-close-"+path)).Size(16, 16).Radius(3).AlignItems(ui.Center).Justify(ui.Center)
						closeBtn.Children(func() {
							if isModified && !tab.Hovered() {
								ui.Box(c).Size(6, 6).Radius(3).Background(tc.Warning)
							} else {
								ui.Icon(c, icons.TablerX).Width(10).Height(10).TextColor(tc.IconMuted)
							}
						})
						if closeBtn.Hovered() {
							closeBtn.Background(tc.Text.Alpha(0.12))
						}
						if closeBtn.Clicked() {
							closedThisTab = true
							a.closeEditorTab(path)
						}
					})
					if isActive {
						tab.Background(tc.SurfaceOverlay)
					} else if tab.Hovered() {
						tab.Background(tc.Text.Alpha(0.04))
					}
					if !closedThisTab && tab.Clicked() {
						em.ActivePath = path
					}
					// Drag-to-reorder: while dragging horizontally, shift the tab in
					// the strip, as the webview's onMoveTab does.
					if _, dx, dragging := tab.Dragged(); dragging && dx != 0 {
						idx := em.TabIndex(path)
						if idx >= 0 {
							if dx > 24 && idx < len(em.OpenTabPaths)-1 {
								em.MoveTab(idx, idx+1)
							} else if dx < -24 && idx > 0 {
								em.MoveTab(idx, idx-1)
							}
						}
					}

					// Zed-style tab context menu, as forge-ade's tab bar has.
					tab.ContextMenu(func(m *ui.Menu) {
						if isTerm {
							if m.Item("Add Snippet...").Chosen() {
								sessID := ""
								if a.Terminal != nil {
									sessID = a.Terminal.ActiveSessionID
								}
								clipText := mygo.Clipboard.ReadText()
								a.OpenAddSnippetModal(sessID, clipText)
							}
							if m.Item("Terminal Snippets...").Shortcut(ui.Alt, ui.KeyS).Chosen() {
								sessID := ""
								if a.Terminal != nil {
									sessID = a.Terminal.ActiveSessionID
								}
								a.OpenTerminalSnippetsModal(sessID, "", "")
							}
							m.Separator()
							if m.Item("New Terminal Tab in Editor").Chosen() {
								a.openEditorTerminalTab("")
							}
							if a.Terminal != nil {
								if m.Item("Clear Terminal").Chosen() {
									a.Terminal.SendInput("\x0c")
								}
								if m.Item("Restart Shell").Chosen() {
									_ = a.Terminal.Restart()
								}
							}
							m.Separator()
							if len(em.Panes) > 1 {
								if m.Item("Move to Right Pane").Chosen() {
									em.MoveTabToPane(path, 1)
								}
								if m.Item("Move to Left Pane").Chosen() {
									em.MoveTabToPane(path, 0)
								}
							} else {
								if m.Item("Split Editor Right").Chosen() {
									em.SplitRight()
								}
							}
							m.Separator()
							if m.Item("Close Tab").Shortcut(ui.Ctrl, ui.KeyW).Chosen() {
								a.closeEditorTab(path)
							}
							if m.Item("Close Others").Chosen() {
								em.RequestCloseOthers(path)
							}
							if m.Item("Close All").Chosen() {
								em.RequestCloseAll()
							}
							return
						}

						pinLabel := "Pin Tab"
						if isPinned {
							pinLabel = "Unpin Tab"
						}
						if m.Item(pinLabel).Chosen() {
							em.TogglePin(path)
						}
						roLabel := "Set Read-Only"
						if isReadOnly {
							roLabel = "Set Editable"
						}
						if m.Item(roLabel).Chosen() {
							em.ToggleReadOnly(path)
						}
						m.Separator()
						if m.Item("Close").Shortcut(ui.Ctrl, ui.KeyW).Chosen() {
							a.closeEditorTab(path)
						}
						if m.Item("Close Others").Chosen() {
							em.RequestCloseOthers(path)
						}
						if m.Item("Close to the Left").Chosen() {
							em.RequestCloseLeft(path)
						}
						if m.Item("Close to the Right").Chosen() {
							em.RequestCloseRight(path)
						}
						if m.Item("Close All").Chosen() {
							em.RequestCloseAll()
						}
						m.Separator()
						if len(em.Panes) > 1 {
							if m.Item("Move to Right Pane").Chosen() {
								em.MoveTabToPane(path, 1)
							}
							if m.Item("Move to Left Pane").Chosen() {
								em.MoveTabToPane(path, 0)
							}
						} else {
							if m.Item("Split Editor Right").Chosen() {
								em.SplitRight()
							}
						}
						m.Separator()
						if m.Item("New Terminal Tab in Editor").Chosen() {
							a.openEditorTerminalTab("")
						}
						if m.Item("Add Snippet...").Chosen() {
							sessID := ""
							if a.Terminal != nil {
								sessID = a.Terminal.ActiveSessionID
							}
							clipText := mygo.Clipboard.ReadText()
							a.OpenAddSnippetModal(sessID, clipText)
						}
						if m.Item("Terminal Snippets...").Shortcut(ui.Alt, ui.KeyS).Chosen() {
							sessID := ""
							if a.Terminal != nil {
								sessID = a.Terminal.ActiveSessionID
							}
							a.OpenTerminalSnippetsModal(sessID, "", "")
						}
						m.Separator()
						if m.Item("Copy Path").Chosen() {
							mygo.Clipboard.WriteText(path)
						}
						if m.Item("Copy Relative Path").Chosen() {
							mygo.Clipboard.WriteText(doc.RelativePath)
						}
						if !strings.HasPrefix(path, "ssh://") {
							if m.Item("Reveal in Finder").Chosen() {
								mygo.Shell.ShowItemInFolder(path)
							}
						}
					})
				}

				if len(em.OpenTabPaths) == 0 {
					ui.Text(c, "No open files").FontSize(12).TextColor(tc.TextMuted)
				}
			})
		})

		actionBtn := func(icon *ui.SVG, label string, active bool, onPick func()) {
			btn := ui.Row(c).Height(24).Radius(6).Padding(0, 8).Gap(4).AlignItems(ui.Center)
			btn.Children(func() {
				ic := ui.Icon(c, icon).Width(13).Height(13).TextColor(tc.IconMuted)
				if active {
					ic.TextColor(tc.Accent)
				}
				if label != "" {
					ui.Text(c, label).FontSize(12).TextColor(tc.TextMuted)
				}
			})
			if active {
				btn.Background(tc.Accent.Alpha(0.1))
			} else if btn.Hovered() {
				btn.Background(tc.Text.Alpha(0.05))
			}
			if btn.Clicked() {
				onPick()
			}
		}

		actionBtn(icons.TablerTerminal, "Shell", strings.HasPrefix(em.ActivePath, "terminal://"), func() {
			a.openEditorTerminalTab("")
		})

		actionBtn(icons.TablerBook, "Snippets", false, func() {
			sessID := ""
			if a.Terminal != nil {
				sessID = a.Terminal.ActiveSessionID
			}
			a.OpenTerminalSnippetsModal(sessID, "", "")
		})

		splitLabel := "Split"
		if len(em.Panes) > 1 {
			splitLabel = "Single"
		}
		actionBtn(icons.TablerColumns, splitLabel, false, func() {
			if len(em.Panes) > 1 {
				em.Panes = nil
				em.PaneWeights = nil
				em.IsSplit = false
			} else {
				em.SplitRight()
			}
		})
		actionBtn(icons.TablerGitCompare, "Diff", em.IsDiffView, func() { em.IsDiffView = !em.IsDiffView })
		actionBtn(icons.TablerRefresh, "Wrap", em.SoftWrap, func() { em.SoftWrap = !em.SoftWrap })
		actionBtn(icons.TablerWand, "Format", false, func() { a.formatActiveDocument(c) })
		actionBtn(icons.TablerSparkles, "Edit with AI", a.InlineAI != nil && a.InlineAI.Open, func() { a.openInlineAI() })
		actionBtn(icons.TablerDeviceFloppy, "Save", false, func() {
			if err := em.SaveActiveFile(); err != nil {
				a.queueToast(ui.Toast{Title: "Save failed", Description: err.Error(), Type: "error"})
			}
		})
	})
}

// renderEditorView renders the tab bar, editor buffer(s) (split or single), or diff view.
func (a *AppUI) renderEditorView(c *ui.Context) {
	em := a.Editor
	if em == nil {
		return
	}

	ui.Column(c).Fill().Children(func() {
		a.renderEditorTabBar(c)
		a.renderEditorContent(c)
	})
}

// renderEditorContent renders diff, PDF, deck, terminal tab, multi-pane or single editor buffer.
func (a *AppUI) renderEditorContent(c *ui.Context) {
	em := a.Editor
	if em == nil {
		return
	}
	t := c.Theme()
	tc := a.ThemeMgr.Colors()

	if strings.HasPrefix(em.ActivePath, "terminal://") {
		a.renderEditorMainTerminal(c, tc, em.ActivePath)
		return
	}

	if em.ActivePath != "" && strings.HasSuffix(strings.ToLower(em.ActivePath), ".pdf") {
		full := em.ActivePath
		if a.Workspace != nil && !filepath.IsAbs(full) {
			full = a.Workspace.ResolveFullPath(full)
		}
		a.renderPdfViewerPane(c, tc, full, filepath.Base(em.ActivePath))
		return
	}

	if em.ActivePath != "" && isDeckOrMotionFile(em.ActivePath) {
		a.renderDeckPane(c)
		return
	}

	if em.ActivePath != "" && isImageOrSVG(em.ActivePath) {
		full := em.ActivePath
		if a.Workspace != nil && !filepath.IsAbs(full) {
			full = a.Workspace.ResolveFullPath(full)
		}
		doc := em.GetDoc(full)
		rel := em.ActivePath
		if a.Workspace != nil {
			if r, err := filepath.Rel(a.Workspace.RootDir, full); err == nil && !strings.HasPrefix(r, "..") {
				rel = r
			}
		}
		if a.SVGViewMode[full] != "code" {
			a.renderImageViewer(c, tc, full, rel, doc, false)
			return
		}
	}

	if em.IsDiffView {
		a.renderEditorDiff(c)
		return
	}

	// Multi-pane layout (EditorPaneState parity): N weighted panes with
	// 3px drag handles between them; the active pane keeps breadcrumbs,
	// minimap and the inline AI bar.
	em.EnsurePanes()
	if panes := em.PaneDocs(); panes != nil {
		ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
			for i, panePath := range panes {
				panePath, paneIdx := panePath, i
				if i > 0 {
					handle := ui.Box(c.Key(fmt.Sprintf("pane-handle-%d", i))).Width(4).FillHeight().Cursor(ui.CursorResizeEW)
					dx, _, held := handle.Dragged()
					if handle.DoubleClicked() {
						eq := 1.0 / float32(len(panes))
						for k := range panes {
							em.SetPaneWeight(k, eq)
						}
					} else if held && dx != 0 {
						delta := dx / 600.0
						em.SetPaneWeight(paneIdx-1, em.PaneWeight(paneIdx-1)+delta)
						em.SetPaneWeight(paneIdx, em.PaneWeight(paneIdx)-delta)
					}
					if held || handle.Hovered() {
						handle.Background(tc.Accent)
					} else {
						handle.Background(t.Border)
					}
				}
				ui.Column(c).Grow(em.PaneWeight(paneIdx)).FillHeight().Children(func() {
					if strings.HasPrefix(panePath, "terminal://") {
						a.renderEditorMainTerminal(c, tc, panePath)
						return
					}
					paneDoc := em.GetDoc(panePath)
					if paneDoc == nil {
						ui.Column(c).Grow(1).Center().Children(func() {
							ui.Text(c, "No file").FontSize(12).TextColor(tc.TextMuted)
						})
						return
					}
					paneDoc.Editor.SoftWrap = em.SoftWrap
					em.ApplyTheme(paneDoc.Editor)
					closedPane := false
					paneHdr := ui.Row(c.Key(fmt.Sprintf("pane-hdr-%d", paneIdx))).Height(28).Padding(0, 12).Gap(6).AlignItems(ui.Center).
						BorderWidth(0, 0, 1, 0).BorderColor(t.Border)
					paneHdr.Children(func() {
						if panePath == em.ActivePath {
							ui.Box(c).Size(6, 6).Radius(3).Background(tc.Accent)
						}
						ui.Text(c, paneDoc.RelativePath).FontSize(12).FontWeight(500).Grow(1).SingleLine()
						if IconButton(c, tc, icons.TablerX, 11).Clicked() {
							closedPane = true
							em.ClosePane(paneIdx)
						}
					})
					if !closedPane && paneHdr.Clicked() {
						em.ActivePane = paneIdx
						em.ActivePath = panePath
					}
					if panePath == em.ActivePath {
						a.renderEditorSingle(c)
					} else {
						if paneDoc != nil && isImageOrSVG(paneDoc.RelativePath) && a.SVGViewMode[paneDoc.FullPath] != "code" {
							a.renderImageViewer(c, tc, paneDoc.FullPath, paneDoc.RelativePath, paneDoc, false)
						} else {
							ui.Column(c).Grow(1).FillWidth().Children(func() {
								edView := mygoeditor.View(c, paneDoc.Editor).Grow(1).FillWidth()
								a.attachEditorContextMenu(c, edView, paneDoc)
							})
						}
					}
				})
			}
		})
		return
	}

	a.renderEditorSingle(c)
}

func (a *AppUI) renderEditorSingle(c *ui.Context) {
	em := a.Editor
	if em == nil {
		return
	}
	tc := a.ThemeMgr.Colors()
	if strings.HasPrefix(em.ActivePath, "terminal://") {
		a.renderEditorMainTerminal(c, tc, em.ActivePath)
		return
	}
	doc := em.ActiveDoc()
	if doc == nil {
		// No workspace opened: welcome pane with Open Folder + the recent
		// folders history (forge-ade's closeWorkspace flow, VS Code style).
		if a.Workspace.RootDir == "" {
			ui.Column(c).Grow(1).Center().Gap(14).Children(func() {
				ui.Icon(c, icons.ForgeSparkSVG).Width(52).Height(52).TextColor(tc.Text)
				ui.Text(c, "Welcome to ForgeADE").FontSize(19).FontWeight(600).TextColor(tc.Text)
				ui.Text(c, "Open a folder to start working, or pick one from your history.").FontSize(13).TextColor(tc.TextMuted)

				openBtn := ui.Row(c).Height(34).Radius(8).Padding(0, 16).Gap(7).AlignItems(ui.Center).Justify(ui.Center).
					Background(tc.Accent).Margin(6, 0, 0, 0)
				openBtn.Children(func() {
					ui.Icon(c, icons.TablerFolder).Width(15).Height(15).TextColor(tc.Canvas)
					ui.Text(c, "Open Folder…").FontSize(13).FontWeight(600).TextColor(tc.Canvas)
				})
				if openBtn.Clicked() {
					a.openWorkspaceFolder()
				}

				if len(a.RecentWorkspaces) > 0 {
					ui.Text(c, "RECENT FOLDERS").FontSize(10).FontWeight(700).LetterSpacing(0.6).TextColor(tc.TextMuted).Margin(10, 0, 0, 0)
					ui.Column(c).Width(420).Gap(2).Children(func() {
						for _, ws := range a.RecentWorkspaces {
							thisPath := ws
							row := ui.Row(c.Key("recent-ws-"+thisPath)).Height(30).Radius(6).Padding(0, 10).Gap(8).AlignItems(ui.Center)
							row.Children(func() {
								ui.Icon(c, icons.TablerFolder).Width(14).Height(14).TextColor(tc.IconMuted)
								ui.Text(c, filepath.Base(thisPath)).FontSize(12).FontWeight(500).TextColor(tc.Text)
								ui.Text(c, thisPath).FontSize(11).TextColor(tc.TextMuted).Grow(1).SingleLine()
							})
							if row.Hovered() {
								row.Background(tc.Text.Alpha(0.05))
							}
							if row.Clicked() {
								a.switchWorkspace(thisPath)
							}
						}
					})
				}

				ui.Row(c).Gap(6).AlignItems(ui.Center).Margin(8, 0, 0, 0).Children(func() {
					sshBtn := ui.Row(c).Height(26).Radius(6).Padding(0, 10).Gap(5).AlignItems(ui.Center).
						Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
					sshBtn.Children(func() {
						ui.Icon(c, icons.TablerServer).Width(12).Height(12).TextColor(tc.Accent)
						ui.Text(c, "Connect via SSH").FontSize(11).TextColor(tc.Text)
					})
					if sshBtn.Clicked() {
						a.OpenSSHModal()
					}
				})
			})
			return
		}
		if strings.HasPrefix(a.Workspace.RootDir, "ssh://") {
			_, remotePath, displayHost, _ := parseSSHWorkspaceURI(a.Workspace.RootDir)
			ui.Column(c).Grow(1).Center().Gap(10).Children(func() {
				ui.Row(c).Size(48, 48).Radius(12).Background(tc.Accent.Alpha(0.12)).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
					ui.Icon(c, icons.TablerServer).Width(24).Height(24).TextColor(tc.Accent)
				})
				ui.Text(c, fmt.Sprintf("Connected to %s", displayHost)).FontSize(15).FontWeight(600).TextColor(tc.Text)
				ui.Text(c, remotePath).FontSize(12).TextColor(tc.TextMuted)
				ui.Row(c).Gap(8).Padding(6, 0).Children(func() {
					termBtn := ui.Row(c).Height(30).Radius(6).Padding(0, 12).
						Background(tc.Accent).AlignItems(ui.Center).Justify(ui.Center).Gap(6)
					termBtn.Children(func() {
						ui.Icon(c, icons.TablerTerminal).Width(13).Height(13).TextColor(tc.Canvas)
						ui.Text(c, "Remote Terminal").FontSize(11).FontWeight(600).TextColor(tc.Canvas)
					})
					if termBtn.Clicked() {
						a.openEditorTerminalTab("")
					}

					uploadBtn := ui.Row(c).Height(30).Radius(6).Padding(0, 12).
						Background(tc.Surface).BorderWidth(1, 1, 1, 1).BorderColor(tc.Border).
						AlignItems(ui.Center).Justify(ui.Center).Gap(6).Cursor(ui.CursorPointer)
					uploadBtn.Children(func() {
						ui.Icon(c, icons.TablerUpload).Width(13).Height(13).TextColor(tc.Accent)
						ui.Text(c, "Upload Files (SFTP)").FontSize(11).FontWeight(500).TextColor(tc.Text)
					})
					if uploadBtn.Hovered() {
						uploadBtn.Background(tc.Text.Alpha(0.05))
					}
					if uploadBtn.Clicked() {
						a.OpenRemoteFileUploader("")
					}

					quickBtn := ui.Row(c).Height(30).Radius(6).Padding(0, 12).
						Background(tc.Surface).BorderWidth(1, 1, 1, 1).BorderColor(tc.Border).
						AlignItems(ui.Center).Justify(ui.Center).Gap(6)
					quickBtn.Children(func() {
						ui.Icon(c, icons.TablerServer).Width(13).Height(13).TextColor(tc.Accent)
						ui.Text(c, "Remote Actions").FontSize(11).FontWeight(500).TextColor(tc.Text)
					})
					if quickBtn.Clicked() {
						a.ShowSSHQuickMenu = true
					}
				})
			})
			return
		}
		ui.Column(c).Grow(1).Center().Gap(10).Children(func() {
			ui.Icon(c, icons.TablerFileCode).Width(40).Height(40).TextColor(tc.IconMuted.Alpha(0.6))
			ui.Text(c, "No file open").FontSize(16).FontWeight(600).TextColor(tc.Text)
			ui.Text(c, "Select a file from the workspace sidebar or press ⌘P to open.").FontSize(12).TextColor(tc.TextMuted)
			ui.Row(c).Gap(8).Margin(6, 0, 0, 0).Children(func() {
				openFileBtn := ui.Row(c).Height(30).Radius(6).Padding(0, 12).
					Background(tc.Accent).AlignItems(ui.Center).Justify(ui.Center).Gap(6).Cursor(ui.CursorPointer)
				openFileBtn.Children(func() {
					ui.Icon(c, icons.TablerSearch).Width(13).Height(13).TextColor(tc.Canvas)
					ui.Text(c, "Open File (⌘P)").FontSize(11).FontWeight(600).TextColor(tc.Canvas)
				})
				if openFileBtn.Clicked() {
					if a.Palette != nil {
						a.Palette.Toggle()
					}
				}

				shellBtn := ui.Row(c).Height(30).Radius(6).Padding(0, 12).
					Background(tc.Surface).BorderWidth(1, 1, 1, 1).BorderColor(tc.Border).
					AlignItems(ui.Center).Justify(ui.Center).Gap(6).Cursor(ui.CursorPointer)
				shellBtn.Children(func() {
					ui.Icon(c, icons.TablerTerminal).Width(13).Height(13).TextColor(tc.Accent)
					ui.Text(c, "Open Shell in Pane").FontSize(11).FontWeight(500).TextColor(tc.Text)
				})
				if shellBtn.Hovered() {
					shellBtn.Background(tc.Text.Alpha(0.05))
				}
				if shellBtn.Clicked() {
					a.openEditorTerminalTab("")
				}

				snipBtn := ui.Row(c).Height(30).Radius(6).Padding(0, 12).
					Background(tc.Surface).BorderWidth(1, 1, 1, 1).BorderColor(tc.Border).
					AlignItems(ui.Center).Justify(ui.Center).Gap(6).Cursor(ui.CursorPointer)
				snipBtn.Children(func() {
					ui.Icon(c, icons.TablerBook).Width(13).Height(13).TextColor(tc.IconMuted)
					ui.Text(c, "Snippets (⌥S)").FontSize(11).FontWeight(500).TextColor(tc.Text)
				})
				if snipBtn.Hovered() {
					snipBtn.Background(tc.Text.Alpha(0.05))
				}
				if snipBtn.Clicked() {
					sessID := ""
					if a.Terminal != nil {
						sessID = a.Terminal.ActiveSessionID
					}
					a.OpenTerminalSnippetsModal(sessID, "", "")
				}
			})
		})
		return
	}

	if isImageOrSVG(doc.RelativePath) && a.SVGViewMode[doc.FullPath] != "code" {
		a.renderImageViewer(c, tc, doc.FullPath, doc.RelativePath, doc, false)
		return
	}

	doc.Editor.SoftWrap = em.SoftWrap
	em.ApplyTheme(doc.Editor)

	ui.Column(c).Grow(1).FillWidth().Children(func() {
		// Breadcrumbs
		a.renderBreadcrumbs(c, doc)

		// Virtualized Code Editor View
		ui.Row(c).Grow(1).FillWidth().Children(func() {
			ui.Column(c).Grow(1).FillHeight().Children(func() {
				edView := mygoeditor.View(c, doc.Editor).Grow(1).FillWidth().AutoFocus()
				a.attachEditorContextMenu(c, edView, doc)
				// Inline AI edit bar floats over the buffer (⌘I).
				a.renderInlineAiBar(c, tc)
			})
			a.renderMinimap(c, tc, doc)
		})
	})
	_ = tc
}

func (a *AppUI) renderBreadcrumbs(c *ui.Context, doc *editor.OpenDocument) {
	t := c.Theme()
	tc := a.ThemeMgr.Colors()

	ui.Row(c).Height(28).Gap(6).Padding(0, 12).AlignItems(ui.Center).
		Background(tc.Canvas).BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
		ui.Icon(c, icons.TablerFolder).Width(13).Height(13).TextColor(tc.IconMuted)
		parts := strings.Split(doc.RelativePath, "/")
		for i, part := range parts {
			if i > 0 {
				ui.Icon(c, icons.TablerChevronRight).Width(10).Height(10).TextColor(tc.IconMuted)
			}
			isLast := (i == len(parts)-1)
			if isLast {
				ui.Icon(c, icons.FileIconSVG(doc.RelativePath)).Width(13).Height(13).TextColor(tc.IconMuted)
				ui.Text(c, part).FontSize(12).FontWeight(500)
			} else {
				ui.Text(c, part).FontSize(12).TextColor(t.TextMuted)
			}
		}
		ui.Box(c).Grow(1)
		if isSVG(doc.RelativePath) {
			svgToggle := ui.Row(c.Key("bc-svg-toggle")).Height(20).Radius(4).Padding(0, 6).Gap(4).AlignItems(ui.Center).
				Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
			svgToggle.Children(func() {
				ui.Icon(c, icons.TablerEye).Width(11).Height(11).TextColor(tc.Accent)
				ui.Text(c, "Preview Vector").FontSize(10).FontWeight(600).TextColor(tc.Accent)
			})
			if svgToggle.Hovered() {
				svgToggle.Background(tc.Accent.Alpha(0.12))
			}
			if svgToggle.Clicked() {
				a.SVGViewMode[doc.FullPath] = "preview"
			}
		}
		langName := "Plain Text"
		if l := doc.Editor.Buffer.Language(); l != nil {
			langName = l.Name
		}
		ui.Text(c, langName).FontSize(10).FontWeight(500).TextColor(t.TextMuted)
	})
}

func (a *AppUI) renderEditorDiff(c *ui.Context) {
	em := a.Editor
	if em == nil {
		return
	}
	t := c.Theme()
	tc := a.ThemeMgr.Colors()
	doc := em.ActiveDoc()
	if doc == nil {
		ui.Column(c).Grow(1).Center().Children(func() {
			ui.Text(c, "No file open").FontSize(17).FontWeight(600).TextColor(t.TextMuted)
		})
		return
	}

	d := doc.Diff
	d.Wrap = em.SoftWrap
	d.Layout = mygoeditor.DiffLayout(em.DiffLayout)

	switch em.DiffInline {
	case "Characters":
		d.Inline = mygoeditor.DiffInlineChar
	case "Off":
		d.Inline = mygoeditor.DiffInlineNone
	default:
		d.Inline = mygoeditor.DiffInlineWordAlt
	}
	em.ApplyDiffTheme(d)

	// Diff Controls Subheader
	ui.Row(c).Gap(12).Padding(6, 12).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
		ui.Text(c, fmt.Sprintf("Diff: %s", doc.RelativePath)).FontSize(13).FontWeight(500)
		ui.Segmented(c, &em.DiffLayout, "Split", "Unified")
		ui.Select(c, &em.DiffInline, []string{"Words", "Characters", "Off"})
		ui.Box(c).Grow(1)
		added, deleted := d.Stats()
		if added > 0 || deleted > 0 {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				if added > 0 {
					ui.Text(c, fmt.Sprintf("+%d", added)).FontSize(11).FontWeight(600).TextColor(tc.Success)
				}
				if deleted > 0 {
					ui.Text(c, fmt.Sprintf("−%d", deleted)).FontSize(11).FontWeight(600).TextColor(tc.Error)
				}
			})
		}
		ui.Text(c, "Wrap").FontSize(12).TextColor(t.TextMuted)
		ui.Switch(c, &em.SoftWrap).Label("Soft wrap")
	})

	if len(d.Hunks()) == 0 {
		ui.Column(c).Grow(1).Center().Gap(4).Children(func() {
			ui.Text(c, "No uncommitted changes").FontSize(16).FontWeight(600).TextColor(t.TextMuted)
			ui.Text(c, "Buffer matches the saved disk baseline.").FontSize(13).TextColor(t.TextMuted)
		})
		return
	}

	mygoeditor.DiffView(c, d).Grow(1).FillWidth()
}

// renderStatusBar draws the 26px editor status bar.
func (a *AppUI) renderStatusBar(c *ui.Context) {
	tc := a.ThemeMgr.Colors()
	// Same naming as the header: "General" when no folder is open (a bare
	// filepath.Base("") used to render a lone "." chip).
	projectName := resolveProjectName(a.Workspace.RootDir)

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

	statusText := func(s string) {
		ui.Text(c, s).FontSize(11).TextColor(tc.TextMuted)
	}
	ui.Row(c).Height(StatusBarHeight).Gap(12).Padding(0, 12).AlignItems(ui.Center).
		Background(tc.Canvas).BorderWidth(1, 0, 0, 0).BorderColor(tc.Border).Children(func() {
		// Remote SSH or Local folder indicator
		if strings.HasPrefix(a.Workspace.RootDir, "ssh://") {
			connID, _, displayHost, folderName := parseSSHWorkspaceURI(a.Workspace.RootDir)
			isConnected := a.Engine != nil && a.Engine.IsSSHConnected(connID)
			chipColor := tc.Accent
			statusDot := tc.Success
			statusExtra := ""
			if !isConnected {
				statusDot = tc.Warning
				statusExtra = " (Disconnected)"
			}
			remoteChip := ui.Row(c).Height(20).Radius(4).Padding(0, 6).Gap(5).AlignItems(ui.Center).
				Background(chipColor.Alpha(0.15)).BorderWidth(1, 1, 1, 1).BorderColor(chipColor.Alpha(0.3)).Cursor(ui.CursorPointer)
			remoteChip.Children(func() {
				ui.Icon(c, icons.TablerServer).Width(12).Height(12).TextColor(chipColor)
				ui.Box(c).Size(5, 5).Radius(2.5).Background(statusDot)
				ui.Text(c, fmt.Sprintf("SSH: %s%s", displayHost, statusExtra)).FontSize(11).FontWeight(600).TextColor(chipColor)
				ui.Text(c, fmt.Sprintf("(%s)", folderName)).FontSize(11).TextColor(tc.TextMuted)
			})
			if remoteChip.Hovered() {
				remoteChip.Background(chipColor.Alpha(0.25))
			}
			if remoteChip.Clicked() {
				a.ShowSSHQuickMenu = true
			}
			if !isConnected {
				reconBtn := ui.Row(c.Key("status-ssh-recon")).Height(20).Radius(4).Padding(0, 6).Gap(4).AlignItems(ui.Center).
					Background(tc.Warning.Alpha(0.18)).BorderWidth(1).BorderColor(tc.Warning.Alpha(0.4)).Cursor(ui.CursorPointer)
				reconBtn.Children(func() {
					ui.Icon(c, icons.TablerRefresh).Width(10).Height(10).TextColor(tc.Warning)
					ui.Text(c, "Reconnect").FontSize(10).FontWeight(600).TextColor(tc.Warning)
				})
				if reconBtn.Hovered() {
					reconBtn.Background(tc.Warning.Alpha(0.3))
				}
				if reconBtn.Clicked() {
					a.ReconnectSSH()
				}
			}
		} else {
			folderChip := ui.Row(c).Height(20).Radius(4).Padding(0, 5).Gap(4).AlignItems(ui.Center)
			folderChip.Children(func() {
				ui.Icon(c, icons.TablerFolder).Width(12).Height(12).TextColor(tc.IconMuted)
				statusText(projectName)
			})
			if folderChip.Hovered() {
				folderChip.Background(tc.Text.Alpha(0.06))
			}
			if folderChip.Clicked() {
				a.ShowLeftSidebar = true
				a.EditorActivity = ActivityExplorer
				if a.Workspace != nil && a.Workspace.RootDir != "" {
					_ = mygo.Shell.OpenExternal("file://" + a.Workspace.RootDir)
				}
			}

			sshChip := ui.Row(c).Height(20).Radius(4).Padding(0, 5).Gap(4).AlignItems(ui.Center)
			sshChip.Children(func() {
				ui.Icon(c, icons.TablerServer).Width(12).Height(12).TextColor(tc.IconMuted)
				statusText("SSH")
			})
			if sshChip.Hovered() {
				sshChip.Background(tc.Text.Alpha(0.06))
			}
			if sshChip.Clicked() {
				a.OpenSSHModal()
			}
		}

		if curPath != "" {
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, icons.FileIconSVG(curPath)).Width(12).Height(12).TextColor(tc.IconMuted)
				statusText(truncRunes(curPath, 60))
			})
		}
		ui.Box(c).Grow(1)

		// Ask Agent bridge: attach active file context and pivot straight to Agent Mode.
		askChip := ui.Row(c.Key("ed-ask-agent")).Height(20).Radius(4).Padding(0, 6).Gap(4).AlignItems(ui.Center).Cursor(ui.CursorPointer).
			Background(tc.Accent.Alpha(0.12))
		askChip.Children(func() {
			ui.Icon(c, icons.TablerSparkles).Width(11).Height(11).TextColor(tc.Accent)
			ui.Text(c, "Ask Agent").FontSize(11).FontWeight(500).TextColor(tc.Accent)
		})
		if askChip.Hovered() {
			askChip.Background(tc.Accent.Alpha(0.22))
		}
		if askChip.Clicked() {
			if a.Agent != nil && curPath != "" {
				a.Agent.AttachFile(curPath)
			}
			a.Mode = ModeAgent
			a.IsGitClientOpen = false
			a.AgentViewMode = AgentViewChat
			a.ActiveTaskKind = "agent"
		}

		// Follow Agent toggle: auto-jump the editor to the files the agent
		// edits during a turn (layoutStore.isFollowAgentMode parity).
		followChip := ui.Row(c).Height(20).Radius(4).Padding(0, 6).Gap(5).AlignItems(ui.Center)
		if a.FollowAgent {
			followChip.Background(tc.Accent.Alpha(0.14))
		}
		followChip.Children(func() {
			eyeColor := tc.TextMuted
			labelColor := tc.TextMuted
			if a.FollowAgent {
				eyeColor = tc.Accent
				labelColor = tc.Accent
			}
			ui.Icon(c, icons.TablerEye).Width(12).Height(12).TextColor(eyeColor)
			ui.Text(c, "Follow Agent").FontSize(11).TextColor(labelColor)
		})
		if followChip.Clicked() {
			a.FollowAgent = !a.FollowAgent
		}

		statusText(lineCol)
		statusText(fmt.Sprintf("%d lines · %s", lineCount, langLabel))
		statusText("UTF-8")

		if a.Git != nil {
			branch := a.Git.Snapshot().Branch
			if branch != "" {
				branchChip := ui.Row(c).Height(20).Radius(4).Padding(0, 6).Gap(4).AlignItems(ui.Center)
				branchChip.Children(func() {
					ui.Icon(c, icons.TablerGitBranch).Width(12).Height(12).TextColor(tc.Accent)
					ui.Text(c, branch).FontSize(11).FontWeight(500).TextColor(tc.Accent)
				})
				if branchChip.Hovered() {
					branchChip.Background(tc.Accent.Alpha(0.12))
				}
				if branchChip.Clicked() {
					a.ShowBranchModal = true
					a.BranchSearchQuery = ""
					a.Git.Refresh()
				}
			}
		}

		if a.Agent != nil {
			modelLabel := a.Agent.ActiveModel
			if modelLabel == "" {
				modelLabel = "No model"
			}
			modelChip := ui.Row(c).Height(20).Radius(4).Padding(0, 6).Gap(4).AlignItems(ui.Center)
			modelChip.Children(func() {
				ui.Icon(c, icons.TablerCpu).Width(12).Height(12).TextColor(tc.IconMuted)
				statusText(modelLabel)
			})
			if modelChip.Hovered() {
				modelChip.Background(tc.Text.Alpha(0.06))
			}
			if modelChip.Clicked() {
				a.ShowModelPalette = true
				a.ModelSearchQuery = ""
			}
		}
		statusText("⌘S Save · ⌘P Open · ⌘B Sidebar · ⌘1 Agent · ⌘2 Editor")
	})
}

// FormatLineCol formats cursor position.
func FormatLineCol(line, col int) string {
	return fmt.Sprintf("Ln %d, Col %d", line+1, col+1)
}

// renderFileSearch renders forge-ade's project search: a query over all
// workspace files with per-line hits that open the file at the match.

// renderMinimap draws the compact content map beside the buffer: one bar per
// line sized to its length, the visible viewport highlighted, click jumps —
// the webview CodeEditorPane's minimap.
func (a *AppUI) renderMinimap(c *ui.Context, tc theme.ThemeColors, doc *editor.OpenDocument) {
	if doc == nil {
		return
	}
	const (
		minimapWidth = float32(64)
		linePitch    = float32(2)
		maxBuckets   = 180
	)

	buckets := doc.MinimapSummary(maxBuckets)
	totalLines := doc.Editor.Buffer.LineCount()
	if totalLines < 1 {
		totalLines = 1
	}
	first, last := doc.Editor.VisibleLines()

	rail := ui.Column(c.Key("minimap")).Width(minimapWidth).FillHeight().
		Background(tc.SurfaceRaised.Alpha(0.35)).BorderWidth(1, 0, 0, 0).BorderColor(tc.Border).
		Padding(2, 2).Gap(0).Clip()
	rail.Children(func() {
		n := len(buckets)
		for i, b := range buckets {
			i := i
			startLine := (i * totalLines) / n
			endLine := ((i + 1) * totalLines) / n
			inView := endLine >= first && startLine <= last

			indent := (minimapWidth - 4) * b.IndentRatio * 0.4
			w := (minimapWidth - 4) * b.LengthRatio
			if w < 1 && b.LengthRatio > 0 {
				w = 1
			}
			if indent+w > minimapWidth-4 {
				w = minimapWidth - 4 - indent
				if w < 0 {
					w = 0
				}
			}

			bar := ui.Row(c.Key(fmt.Sprintf("mm-%d", i))).Height(linePitch).AlignItems(ui.Center)
			bar.Children(func() {
				if indent > 0 {
					ui.Box(c).Width(indent).Height(1)
				}
				if w > 0 {
					color := tc.Text.Alpha(0.18)
					if inView {
						color = tc.Text.Alpha(0.45)
					}
					ui.Box(c).Width(w).Height(1).Background(color)
				}
			})
			if bar.Clicked() {
				doc.Editor.GoTo(editorPosAtLine(doc, startLine))
			}
		}
	})
}

// editorPosAtLine builds a mygoeditor Pos for a 0-based line.
func editorPosAtLine(doc *editor.OpenDocument, line int) mygoeditor.Pos {
	return mygoeditor.Pos{Line: line, Col: 0}
}

// openEditorTerminalTab opens a terminal tab directly in the main editor pane
// (CodeEditorPane parity). Every tab owns a real, distinct PTY session: the
// terminal manager spawns a new session first and the editor tab binds to it;
// if the session cannot be created no tab is added.
func (a *AppUI) openEditorTerminalTab(title string) {
	if a.Editor == nil {
		return
	}
	isRemote := strings.HasPrefix(a.Workspace.RootDir, "ssh://")
	if title == "" {
		if isRemote {
			_, remotePath, _, folder := parseSSHWorkspaceURI(a.Workspace.RootDir)
			if folder == "" || folder == "remote-root" || folder == "." || folder == "/" {
				folder = filepath.Base(remotePath)
			}
			if folder == "" || folder == "." || folder == "/" {
				folder = "remote"
			}
			title = folder
		} else {
			folder := filepath.Base(a.Workspace.RootDir)
			if folder == "" || folder == "." || folder == "/" {
				folder = "terminal"
			}
			title = folder
		}
	}
	// Hide bottom terminal dock so the terminal is not duplicated on screen
	a.ShowBottomTerminal = false

	if a.Terminal == nil {
		a.queueToast(ui.Toast{Title: "Terminal unavailable", Description: "The terminal manager is not ready", Type: "error"})
		return
	}
	var sessionID int
	if isRemote {
		var prog string
		var args, env []string
		if a.Engine != nil && a.Engine.RemoteFS != nil {
			if client, remotePath, ok := a.Engine.RemoteFS.ParseRemotePath(a.Workspace.RootDir); ok && client != nil {
				prog, args, env = client.SSHCommand(remotePath)
			}
		}
		if prog == "" {
			connID, remotePath, _, _ := parseSSHWorkspaceURI(a.Workspace.RootDir)
			foundHost := connID
			for _, cfg := range a.SSHSavedConfigs {
				if cfg.ID == connID {
					user := cfg.User
					if user == "" {
						user = "root"
					}
					foundHost = fmt.Sprintf("%s@%s", user, cfg.Host)
					if cfg.Port > 0 && cfg.Port != 22 {
						args = append(args, "-p", strconv.Itoa(cfg.Port))
					}
					break
				}
			}
			prog = "ssh"
			args = append(args, "-o", "StrictHostKeyChecking=accept-new", foundHost, "-t", fmt.Sprintf("cd '%s' && exec $SHELL -l", remotePath))
		}

		if a.Editor.TerminalSessionCount() == 0 && a.Terminal.TabCount() == 1 {
			tabs, _ := a.Terminal.SnapshotTabs()
			if len(tabs) > 0 {
				sessionID = tabs[0].ID
				a.Terminal.Command = append([]string{prog}, args...)
				a.Terminal.Env = env
				a.Terminal.Title = title
				if a.Terminal.Term() == nil || a.Terminal.IsExited() {
					_ = a.Terminal.Restart()
				}
			}
		} else {
			if newTab, err := a.Terminal.NewTabCustom(title, a.Workspace.RootDir, append([]string{prog}, args...), env); err == nil && newTab != nil {
				sessionID = newTab.ID
			} else if err != nil {
				a.queueToast(ui.Toast{Title: "Could not open terminal", Description: err.Error(), Type: "error"})
				return
			}
		}
	} else {
		if a.Editor.TerminalSessionCount() == 0 && a.Terminal.TabCount() == 1 {
			tabs, _ := a.Terminal.SnapshotTabs()
			if len(tabs) > 0 {
				sessionID = tabs[0].ID
				if a.Terminal.Term() == nil || a.Terminal.IsExited() {
					_ = a.Terminal.Restart()
				}
			}
		} else {
			cwd := a.Workspace.RootDir
			if err := a.Terminal.NewTab(cwd); err == nil {
				if tabs, active := a.Terminal.SnapshotTabs(); len(tabs) > 0 && active >= 0 && active < len(tabs) {
					sessionID = tabs[active].ID
				}
			} else {
				a.queueToast(ui.Toast{Title: "Could not open terminal", Description: err.Error(), Type: "error"})
				return
			}
		}
	}

	tabID := a.Editor.OpenTerminalTab(title)
	if sessionID > 0 {
		a.Editor.BindTerminalSession(tabID, sessionID)
	}
	a.Mode = ModeEditor
	if a.Window != nil {
		a.Window.Invalidate()
	}
}

// restartOrReconnectTerminal refreshes or reconnects the active terminal session.
func (a *AppUI) restartOrReconnectTerminal() {
	if a.Terminal == nil {
		return
	}
	isRemote := strings.HasPrefix(a.Workspace.RootDir, "ssh://")
	if isRemote && a.Engine != nil {
		connID, _, _, _ := parseSSHWorkspaceURI(a.Workspace.RootDir)
		if connID != "" && !a.Engine.IsSSHConnected(connID) {
			a.ReconnectSSH()
			return
		}
	}
	_ = a.Terminal.Restart()
	if a.Window != nil {
		a.Window.Invalidate()
	}
}

// activeTerminalSessionID returns the TerminalState's active tab ID, 0 when
// none exists.
func (a *AppUI) activeTerminalSessionID() int {
	if a.Terminal == nil {
		return 0
	}
	tabs, active := a.Terminal.SnapshotTabs()
	if active < 0 || active >= len(tabs) {
		return 0
	}
	return tabs[active].ID
}

// renderEditorMainTerminal renders the interactive terminal directly in the main editor area.
// The editor tab bar above provides tab switching (1:1 with terminal sessions);
// the terminal viewport fills the pane cleanly with zero redundant headers or nested strips.
func (a *AppUI) renderEditorMainTerminal(c *ui.Context, tc theme.ThemeColors, tabPath string) {
	if a.Terminal == nil {
		ui.Column(c.Key("editor-main-terminal-" + tabPath)).Fill().Background(tc.Canvas).Center().Children(func() {
			ui.Text(c, "Terminal initializing...").FontSize(12).TextColor(tc.TextMuted)
		})
		return
	}

	if sessionID, ok := a.Editor.TerminalSessionFor(tabPath); ok && sessionID > 0 {
		a.selectTerminalSessionByID(a.Terminal, sessionID)
	}

	if a.Terminal.Term() == nil && a.Terminal.IsExited() && a.Terminal.TabCount() <= 1 {
		_ = a.Terminal.Restart()
	}

	ui.Column(c.Key("editor-main-terminal-" + tabPath)).Fill().Background(tc.Canvas).Children(func() {
		a.renderTerminal(c, a.Terminal)
	})
}

// attachEditorContextMenu wires Zed/VSCode-parity right-click actions onto the editor view.
func (a *AppUI) attachEditorContextMenu(c *ui.Context, el ui.Element, doc *editor.OpenDocument) ui.Element {
	if doc == nil || doc.Editor == nil {
		return el
	}
	return el.ContextMenu(func(m *ui.Menu) {
		sym := a.symbolUnderCursor(doc)
		wsDir := a.Workspace.RootDir
		em := a.Editor

		// Symbol scans walk the whole workspace, so they run off the render
		// thread and apply results on the UI thread (same panes as before).
		scanAsync := func(action, notFound string, scan func(symbol, wsDir string) []editor.PeekMatch) {
			if sym == "" {
				return
			}
			go func() {
				matches := scan(sym, wsDir)
				a.runOnUI(func() {
					a.applyPeekMatches(action, notFound, sym, matches)
				})
			}()
		}

		// 1. Navigation / LSP
		if m.Item("Go to Definition").Shortcut(0, ui.KeyF12).Chosen() {
			if sym != "" {
				scanAsync("Go to Definition", "No definition found for ", em.FindDefinitions)
			} else {
				doc.Editor.Run(mygoeditor.GoToDefinition)
			}
		}

		if m.Item("Go to Declaration").Shortcut(ui.Ctrl, ui.KeyF12).Chosen() {
			scanAsync("Go to Declaration", "No declaration found for ", em.FindDefinitions)
		}

		if m.Item("Go to Type Definition").Shortcut(ui.Shift, ui.KeyF12).Chosen() {
			scanAsync("Go to Type Definition", "No type definition found for ", em.FindTypeDefinitions)
		}

		if m.Item("Go to Implementation").Shortcut(ui.Cmd, ui.KeyF12).Chosen() {
			scanAsync("Go to Implementation", "No implementation found for ", em.FindImplementations)
		}

		if m.Item("Find All References").Shortcut(ui.Alt|ui.Shift, ui.KeyF12).Chosen() {
			scanAsync("Find All References", "No references found for ", em.FindReferences)
		}

		if m.Item("Find References").Shortcut(ui.Alt|ui.Shift, ui.KeyH).Chosen() {
			scanAsync("Find References", "No references found for ", em.FindReferences)
		}

		m.Separator()

		// 2. Refactoring & Formatting
		if m.Item("Rename Symbol").Shortcut(0, ui.KeyF2).Chosen() {
			if sym != "" {
				a.ShowRenameSymbolModal = true
				a.RenameSymbolTarget = sym
				a.RenameSymbolNewName = sym
			} else {
				c.AddToast(ui.Toast{Title: "Rename Symbol", Description: "Place cursor on a symbol to rename", Type: "info"})
			}
		}

		if m.Item("Format Buffer").Shortcut(ui.Alt|ui.Shift, ui.KeyF).Chosen() {
			a.formatDocument(c, doc)
		}

		if m.Item("Format Selections").Shortcut(ui.Cmd, ui.KeyK).Chosen() {
			a.formatDocumentSelection(c, doc)
		}

		if m.Item("Show Code Actions").Shortcut(ui.Cmd, ui.KeyPeriod).Chosen() {
			a.openInlineAI()
		}

		m.Separator()

		// 3. Clipboard & Selection
		if m.Item("Cut").Shortcut(ui.Cmd, ui.KeyX).Chosen() {
			doc.Editor.Run(mygoeditor.Cut)
		}

		if m.Item("Copy").Shortcut(ui.Cmd, ui.KeyC).Chosen() {
			doc.Editor.Run(mygoeditor.Copy)
		}

		if m.Item("Copy and Trim").Chosen() {
			sel := strings.TrimSpace(doc.Editor.SelectedText())
			if sel != "" {
				mygo.Clipboard.WriteText(sel)
				c.AddToast(ui.Toast{Title: "Copied & Trimmed", Description: sel, Type: "success"})
			}
		}

		if m.Item("Paste").Shortcut(ui.Cmd, ui.KeyV).Chosen() {
			doc.Editor.Run(mygoeditor.Paste)
		}

		m.Separator()

		// 4. File / System Integration
		if m.Item("Reveal in Finder").Shortcut(ui.Cmd|ui.Alt, ui.KeyR).Chosen() {
			mygo.Shell.ShowItemInFolder(doc.FullPath)
		}

		if m.Item("Open in Terminal").Chosen() {
			a.ShowBottomTerminal = true
			_ = a.Terminal.ResetShell(filepath.Dir(doc.FullPath))
		}

		if m.Item("Copy Permalink to Line").Chosen() {
			line := doc.Editor.CursorPos().Line + 1
			permalink := fmt.Sprintf("%s#L%d", doc.RelativePath, line)
			mygo.Clipboard.WriteText(permalink)
			c.AddToast(ui.Toast{Title: "Permalink Copied", Description: permalink, Type: "success"})
		}
	})
}

// applyPeekMatches applies a finished symbol scan exactly as the inline
// context-menu actions did: a single match opens the file at the match,
// multiple matches open the peek modal, none toasts.
func (a *AppUI) applyPeekMatches(action, notFoundPrefix, sym string, matches []editor.PeekMatch) {
	switch {
	case len(matches) == 1:
		a.openSearchResultChecked(matches[0].Path, matches[0].Line, "")
	case len(matches) > 1:
		a.ShowPeekModal = true
		a.PeekSymbol = sym
		a.PeekResults = matches
	default:
		a.queueToast(ui.Toast{Title: action, Description: notFoundPrefix + sym, Type: "info"})
	}
}

// closeEditorTab is the UI-facing close for every close control in the
// editor surface: editor-managed terminal tabs tear down their own PTY
// session, then the Request* path runs (discard confirmation aware).
func (a *AppUI) closeEditorTab(path string) {
	if strings.HasPrefix(path, "terminal://") && a.Terminal != nil {
		if sessionID, ok := a.Editor.TerminalSessionFor(path); ok {
			a.closeTerminalSessionByID(a.Terminal, sessionID)
		}
	}
	a.Editor.RequestClose(path)
}

// closeTerminalSessionByID closes the TerminalState tab with the given ID,
// if it still exists.
func (a *AppUI) closeTerminalSessionByID(ts *terminal.TerminalState, sessionID int) {
	tabs, _ := ts.SnapshotTabs()
	for _, tab := range tabs {
		if tab.ID == sessionID {
			ts.CloseTab(tab.Index)
			return
		}
	}
}

// selectTerminalSessionByID makes the bound session the TerminalState's
// active tab so the editor pane renders its own PTY, not a shared one.
func (a *AppUI) selectTerminalSessionByID(ts *terminal.TerminalState, sessionID int) {
	tabs, active := ts.SnapshotTabs()
	for _, tab := range tabs {
		if tab.ID == sessionID {
			if active != tab.Index {
				ts.SelectTab(tab.Index)
			}
			return
		}
	}
}
