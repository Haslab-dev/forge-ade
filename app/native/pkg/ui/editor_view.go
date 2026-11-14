package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	mygoeditor "github.com/egoist/mygo/ui/editor"
	"github.com/hasdev/forge-ade/app/native/pkg/editor"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
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
			gitChanges = len(a.Git.Staged) + len(a.Git.Unstaged) + len(a.Git.Untracked)
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
				if a.Settings != nil {
					a.Settings.Toggle()
				}
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

			if a.ShowBottomTerminal {
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
		for _, tabPath := range em.OpenTabs() {
			path := tabPath
			doc := em.GetDoc(path)
			if doc == nil {
				continue
			}

			isActive := (path == em.ActivePath)
			isModified := doc.Editor.Buffer.Modified()
			baseName := filepath.Base(doc.RelativePath)
			isPinned := em.IsPinned(path)
			isReadOnly := em.IsReadOnly(path)

			tab := ui.Row(c.Key("tab-"+path)).Height(EditorTabBarHeight).Padding(0, 10).Gap(6).
				AlignItems(ui.Center).BorderWidth(0, 1, 0, 0).BorderColor(tc.Border)
			tab.Children(func() {
				ui.Icon(c, icons.FileIconSVG(doc.RelativePath)).Width(13).Height(13).TextColor(tc.IconMuted)
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
					em.CloseTab(path)
				}
			})
			if isActive {
				tab.Background(tc.SurfaceOverlay)
			} else if tab.Hovered() {
				tab.Background(tc.Text.Alpha(0.04))
			}
			if tab.Clicked() {
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
					em.CloseTab(path)
				}
				if m.Item("Close Others").Chosen() {
					em.CloseOthers(path)
				}
				if m.Item("Close to the Left").Chosen() {
					seen := false
					for _, p := range em.OpenTabs() {
						if p == path {
							seen = true
							continue
						}
						if !seen {
							em.CloseTab(p)
						}
					}
				}
				if m.Item("Close to the Right").Chosen() {
					em.CloseToRight(path)
				}
				if m.Item("Close All").Chosen() {
					em.CloseAll()
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

		ui.Box(c).Grow(1)

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
		actionBtn(icons.TablerDeviceFloppy, "Save", false, func() { _ = em.SaveActiveFile() })
	})
}

// renderEditorView renders the tab bar, editor buffer(s) (split or single), or diff view.
func (a *AppUI) renderEditorView(c *ui.Context) {
	em := a.Editor
	if em == nil {
		return
	}
	t := c.Theme()
	tc := a.ThemeMgr.Colors()

	ui.Column(c).Fill().Children(func() {
		a.renderEditorTabBar(c)

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
					if i > 0 {
						ui.Box(c.Key(fmt.Sprintf("pane-handle-%d", i))).Width(3).FillHeight().Background(t.Border)
					}
					panePath, paneIdx := panePath, i
					ui.Column(c).FillHeight().Children(func() {
						paneDoc := em.GetDoc(panePath)
						if paneDoc == nil {
							ui.Column(c).Grow(1).Center().Children(func() {
								ui.Text(c, "No file").FontSize(12).TextColor(tc.TextMuted)
							})
							return
						}
						paneDoc.Editor.SoftWrap = em.SoftWrap
						em.ApplyTheme(paneDoc.Editor)
						ui.Row(c).Height(28).Padding(0, 12).Gap(6).AlignItems(ui.Center).
							BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
							if panePath == em.ActivePath {
								ui.Box(c).Size(6, 6).Radius(3).Background(tc.Accent)
							}
							ui.Text(c, paneDoc.RelativePath).FontSize(12).FontWeight(500).Grow(1).SingleLine()
							if IconButton(c, tc, icons.TablerX, 11).Clicked() {
								em.ClosePane(paneIdx)
							}
						})
						if panePath == em.ActivePath {
							a.renderEditorSingle(c)
						} else {
							ui.Row(c).Grow(1).Children(func() {
								mygoeditor.View(c, paneDoc.Editor).Grow(1).FillWidth()
							})
						}
					})
				}
			})
			return
		}

		a.renderEditorSingle(c)
	})
}

func (a *AppUI) renderEditorSingle(c *ui.Context) {
	em := a.Editor
	if em == nil {
		return
	}
	t := c.Theme()
	tc := a.ThemeMgr.Colors()
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
						a.openRemoteTerminal()
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
		ui.Column(c).Grow(1).Center().Gap(6).Children(func() {
			ui.Text(c, "No file open").FontSize(17).FontWeight(600).TextColor(t.TextMuted)
			ui.Text(c, "Select a file from the workspace sidebar or press ⌘P to open.").FontSize(13).TextColor(t.TextMuted)
		})
		return
	}

	doc.Editor.SoftWrap = em.SoftWrap
	em.ApplyTheme(doc.Editor)

	ui.Column(c).Grow(1).FillWidth().Children(func() {
		// Breadcrumbs
		a.renderBreadcrumbs(c, doc)

		// Virtualized Code Editor View
		ui.Box(c).Fill().Children(func() {
			ui.Row(c).Grow(1).Children(func() {
				ui.Box(c).Grow(1).Children(func() {
					mygoeditor.View(c, doc.Editor).Grow(1).FillWidth().AutoFocus()
					// Inline AI edit bar floats over the buffer (⌘I).
					a.renderInlineAiBar(c, tc)
				})
				a.renderMinimap(c, tc, doc)
			})
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
	projectName := filepath.Base(a.Workspace.RootDir)

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
		// Remote SSH indicator
		if strings.HasPrefix(a.Workspace.RootDir, "ssh://") {
			_, _, displayHost, folderName := parseSSHWorkspaceURI(a.Workspace.RootDir)
			remoteChip := ui.Row(c).Height(20).Radius(4).Padding(0, 6).Gap(5).AlignItems(ui.Center).
				Background(tc.Accent.Alpha(0.15)).BorderWidth(1, 1, 1, 1).BorderColor(tc.Accent.Alpha(0.3))
			remoteChip.Children(func() {
				ui.Icon(c, icons.TablerServer).Width(12).Height(12).TextColor(tc.Accent)
				ui.Box(c).Size(5, 5).Radius(2.5).Background(tc.Success)
				ui.Text(c, fmt.Sprintf("SSH: %s", displayHost)).FontSize(11).FontWeight(600).TextColor(tc.Accent)
				ui.Text(c, fmt.Sprintf("(%s)", folderName)).FontSize(11).TextColor(tc.TextMuted)
			})
			if remoteChip.Hovered() {
				remoteChip.Background(tc.Accent.Alpha(0.25))
			}
			if remoteChip.Clicked() {
				a.ShowSSHQuickMenu = true
			}
		} else {
			// Local workspace indicator: folder name (General when empty).
			folderLabel := filepath.Base(a.Workspace.RootDir)
			if a.Workspace.RootDir == "" {
				folderLabel = "General"
			}
			remoteChip := ui.Row(c).Height(20).Radius(4).Padding(0, 5).Gap(4).AlignItems(ui.Center)
			remoteChip.Children(func() {
				ui.Icon(c, icons.TablerFolder).Width(12).Height(12).TextColor(tc.IconMuted)
				statusText(folderLabel)
			})
			if remoteChip.Hovered() {
				remoteChip.Background(tc.Text.Alpha(0.06))
			}
			if remoteChip.Clicked() {
				a.OpenSSHModal()
			}
		}

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

		if curPath != "" {
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, icons.FileIconSVG(curPath)).Width(12).Height(12).TextColor(tc.IconMuted)
				statusText(curPath)
			})
		}
		ui.Box(c).Grow(1)

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

		if a.Git != nil && a.Git.Branch != "" {
			branchChip := ui.Row(c).Height(20).Radius(4).Padding(0, 6).Gap(4).AlignItems(ui.Center)
			branchChip.Children(func() {
				ui.Icon(c, icons.TablerGitBranch).Width(12).Height(12).TextColor(tc.Accent)
				ui.Text(c, a.Git.Branch).FontSize(11).FontWeight(500).TextColor(tc.Accent)
			})
			if branchChip.Hovered() {
				branchChip.Background(tc.Accent.Alpha(0.12))
			}
			if branchChip.Clicked() {
				a.ShowBranchModal = true
				a.BranchSearchQuery = ""
				if a.Git != nil {
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
	_ = tc
	if doc == nil {
		return
	}
	const (
		minimapWidth  = float32(64)
		linePitch     = float32(2)
		maxRenderRows = 900
	)

	buf := doc.Editor.Buffer
	first, last := doc.Editor.VisibleLines()

	rail := ui.Column(c.Key("minimap")).Width(minimapWidth).FillHeight().
		Background(tc.SurfaceRaised.Alpha(0.35)).BorderWidth(1, 0, 0, 0).BorderColor(tc.Border).
		Padding(2, 2).Gap(0).Clip()
	rail.Children(func() {
		text := buf.String()
		lines := strings.Split(text, "\n")
		rows := len(lines)
		if rows > maxRenderRows {
			rows = maxRenderRows
		}
		// Scale bars to the widest visible line so the map stays legible.
		maxLen := 1
		for i := 0; i < rows; i++ {
			if l := len(lines[i]); l > maxLen {
				maxLen = l
			}
		}
		for i := 0; i < rows; i++ {
			i := i
			ln := len(lines[i])
			w := minimapWidth * float32(ln) / float32(maxLen)
			if w < 1 && ln > 0 {
				w = 1
			}
			inView := i >= first && i <= last
			bar := ui.Row(c.Key(fmt.Sprintf("mm-%d", i))).Height(linePitch).AlignItems(ui.Center)
			bar.Children(func() {
				color := tc.Text.Alpha(0.18)
				if inView {
					color = tc.Text.Alpha(0.4)
				}
				ui.Box(c).Width(w).Height(1).Background(color)
			})
			if bar.Clicked() {
				doc.Editor.GoTo(editorPosAtLine(doc, i))
			}
		}
	})
}

// editorPosAtLine builds a mygoeditor Pos for a 0-based line.
func editorPosAtLine(doc *editor.OpenDocument, line int) mygoeditor.Pos {
	return mygoeditor.Pos{Line: line, Col: 0}
}
