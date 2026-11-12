package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	mygoeditor "github.com/egoist/mygo/ui/editor"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/editor"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/icons"
)

// renderEditorSurface renders the Editor mode layout: ActivityBar + Sidebar, Editor Tabs & Buffers, Bottom Terminal, Right Drawer.
func (a *AppUI) renderEditorSurface(c *ui.Context) {
	tc := a.ThemeMgr.Colors()

	// 1. Left Activity Bar + Sidebar
	if a.ShowLeftSidebar {
		activityButton := func(icon *ui.SVG, tab int, badgeCount int, onPick func()) {
			active := (tab >= 0 && a.EditorActivity == tab)
			if tab == -1 && a.Settings != nil && a.Settings.IsOpen {
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
			activityButton(icons.TablerFiles, ActivityExplorer, 0, nil)
			activityButton(icons.TablerGitBranch, ActivityGit, gitChanges, nil)
			activityButton(icons.TablerDatabase, ActivityDatabase, 0, nil)
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

		ui.Column(c).Width(FileTreeDefaultWidth).Background(tc.Canvas).BorderWidth(0, 1, 0, 0).BorderColor(tc.Border).Children(func() {
			switch a.EditorActivity {
			case ActivityExplorer:
				a.renderFileTree(c)
			case ActivityGit:
				a.renderGitPane(c)
			case ActivityDatabase:
				a.renderDatabasePane(c)
			}
		})
	}

	// 2. Center Column: Editor Tabs + Virtualized Code Buffer / Diff + Collapsible Bottom Terminal
	ui.Column(c).Grow(1).FillHeight().Children(func() {
		ui.Column(c).Grow(1).FillHeight().Children(func() {
			a.renderEditorView(c)
		})

		if a.ShowBottomTerminal {
			TopResizeHandle(c, &a.BottomTerminalHeight, 120, 600, 240)
			a.renderBottomTerminal(c)
		}
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

			tab := ui.Row(c.Key("tab-"+path)).Height(EditorTabBarHeight).Padding(0, 10).Gap(6).
				AlignItems(ui.Center).BorderWidth(0, 1, 0, 0).BorderColor(tc.Border)
			tab.Children(func() {
				ui.Icon(c, icons.FileIconSVG(doc.RelativePath)).Width(13).Height(13).TextColor(tc.IconMuted)
				label := ui.Text(c, baseName).FontSize(12)
				if isActive {
					label.FontWeight(500)
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
		if em.IsSplit {
			splitLabel = "Single"
		}
		actionBtn(icons.TablerColumns, splitLabel, false, func() { em.IsSplit = !em.IsSplit })
		actionBtn(icons.TablerGitCompare, "Diff", em.IsDiffView, func() { em.IsDiffView = !em.IsDiffView })
		actionBtn(icons.TablerRefresh, "Wrap", em.SoftWrap, func() { em.SoftWrap = !em.SoftWrap })
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

	ui.Column(c).Fill().Children(func() {
		a.renderEditorTabBar(c)

		if em.IsDiffView {
			a.renderEditorDiff(c)
			return
		}

		if em.IsSplit && len(em.OpenTabPaths) >= 2 {
			ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
				ui.Column(c).Grow(1).FillHeight().Children(func() {
					a.renderEditorSingle(c)
				})

				ui.Box(c).Width(3).FillHeight().Background(t.Border)

				var secondaryPath string
				for _, p := range em.OpenTabPaths {
					if p != em.ActivePath {
						secondaryPath = p
						break
					}
				}
				secondaryDoc := em.GetDoc(secondaryPath)
				ui.Column(c).Grow(1).FillHeight().Children(func() {
					if secondaryDoc != nil {
						secondaryDoc.Editor.SoftWrap = em.SoftWrap
						em.ApplyTheme(secondaryDoc.Editor)
						ui.Row(c).Height(28).Padding(0, 12).AlignItems(ui.Center).
							BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
							ui.Text(c, secondaryDoc.RelativePath).FontSize(12).FontWeight(500)
						})
						mygoeditor.View(c, secondaryDoc.Editor).Grow(1).FillWidth()
					}
				})
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
						a.ShowBottomTerminal = true
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
		mygoeditor.View(c, doc.Editor).Grow(1).FillWidth().AutoFocus()
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
			remoteChip := ui.Row(c).Height(20).Radius(4).Padding(0, 5).Gap(4).AlignItems(ui.Center)
			remoteChip.Children(func() {
				ui.Icon(c, icons.TablerServer).Width(12).Height(12).TextColor(tc.IconMuted)
				statusText("Remote")
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
