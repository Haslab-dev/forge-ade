package ui

import (
	"path/filepath"
	"runtime"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
)

// renderWorkspaceHeader renders ForgeADE's WorkspaceHeader: 52px tall, the
// chrome background, title, project/branch chips, the segmented mode
// switcher and the 28px icon controls.
func (a *AppUI) renderWorkspaceHeader(c *ui.Context) {
	t := c.Theme()
	tc := a.ThemeMgr.Colors()
	projectName := filepath.Base(a.Workspace.RootDir)

	// Derive display title
	title := projectName
	if a.Mode == ModeAgent {
		if a.ActiveTaskKind == "cli" && a.CLIMgr != nil && a.CLIMgr.ActiveSession() != nil {
			title = a.CLIMgr.ActiveSession().Title
		} else if sess := a.Agent.ActiveSession(); sess != nil {
			title = sess.Title
		}
	} else {
		if doc := a.Editor.ActiveDoc(); doc != nil {
			title = filepath.Base(doc.RelativePath)
		}
	}

	bar := c.TitleBar()
	leftPad := float32(14)
	showToggleInHeader := true
	if a.Mode == ModeAgent && a.ShowLeftSidebar {
		// Full-height sidebar is visible on the left; header sits beside it
		showToggleInHeader = false
		leftPad = 12
	} else if bar.Left > 0 {
		// Sidebar is collapsed or in editor mode; clear traffic lights / window controls
		leftPad = max(bar.Left+12, 78)
	} else if runtime.GOOS == "darwin" {
		leftPad = 78
	}

	rightPad := float32(12)
	if bar.Right > 0 {
		rightPad = max(bar.Right+12, 135)
	} else if bar.Left == 0 && (runtime.GOOS == "linux" || runtime.GOOS == "windows") {
		// On Linux/Windows window buttons are on the right (minimize, maximize, close)
		rightPad = 135
	}

	ui.Row(c).Height(WorkspaceTopBarHeight).Gap(10).Padding(0, rightPad, 0, leftPad).
		AlignItems(ui.Center).
		Background(tc.Chrome).
		BorderWidth(0, 0, 1, 0).BorderColor(tc.ToolbarBorder).
		DragWindow().
		Children(func() {
			// Left: Sidebar Toggle (only shown when the full sidebar is collapsed)
			if showToggleInHeader {
				if IconButton(c, tc, icons.TablerPanelLeft, 16).Clicked() {
					a.ShowLeftSidebar = !a.ShowLeftSidebar
				}
			}

			// Title: text-ui-sm font-semibold max-w-80 truncate
			ui.Text(c, title).FontSize(13).FontWeight(600).MaxWidth(320)

			// In Editor surface: Project Chip & Git Branch Switcher (WorkspaceHeader parity)
			if a.Mode == ModeEditor {
				// Project folder chip
				projectBtn := ui.Row(c).Height(26).Radius(6).Padding(0, 8).Gap(5).AlignItems(ui.Center).
					Background(tc.ToolbarControl)
				projectBtn.Children(func() {
					ui.Icon(c, icons.TablerFolder).Width(13).Height(13).TextColor(tc.IconMuted)
					ui.Text(c, projectName).FontSize(12).FontWeight(500)
					ui.Icon(c, icons.TablerChevronDown).Width(11).Height(11).TextColor(tc.IconMuted)
				})
				if projectBtn.Hovered() {
					projectBtn.Background(tc.ToolbarControlHover)
				}
				if projectBtn.Clicked() {
					a.openWorkspaceFolder()
				}

				// Git Branch switcher chip
				branchName := "main"
				if a.Git != nil && a.Git.Branch != "" {
					branchName = a.Git.Branch
				}
				branchBtn := ui.Row(c).Height(26).Radius(6).Padding(0, 8).Gap(5).AlignItems(ui.Center).
					Background(tc.ToolbarControl)
				branchBtn.Children(func() {
					ui.Icon(c, icons.TablerGitBranch).Width(13).Height(13).TextColor(tc.Accent)
					ui.Text(c, branchName).FontSize(12).FontWeight(500).TextColor(tc.Accent)
					ui.Icon(c, icons.TablerChevronDown).Width(11).Height(11).TextColor(tc.IconMuted)
				})
				if branchBtn.Hovered() {
					branchBtn.Background(tc.ToolbarControlHover)
				}
				if branchBtn.Clicked() {
					a.ShowBranchModal = true
					a.BranchSearchQuery = ""
					if a.Git != nil {
						a.Git.Refresh()
					}
				}
			}

			// Task "…" menu (Agent mode with an active session)
			if a.Mode == ModeAgent && a.Agent != nil && a.Agent.ActiveSession() != nil {
				if IconButton(c, tc, icons.TablerDots, 15).Clicked() {
					a.ShowTaskMenuModal = true
				}
			}

			ui.Box(c).Grow(1)

			// Segmented Mode Switcher: [✨ Agent] | [💻 Editor]
			// h-7 rounded-lg border border-border-default/80 bg-surface/60 p-0.5
			ui.Row(c).Height(28).Gap(2).Padding(2).Radius(8).
				Background(tc.ToolbarControl).
				BorderWidth(1).BorderColor(tc.ToolbarBorder).
				AlignItems(ui.Center).Children(func() {
				segmentedModeButton(c, tc, icons.TablerSparkles, "Agent", a.Mode == ModeAgent, func() {
					a.Mode = ModeAgent
					a.AgentViewMode = AgentViewChat
				})
				segmentedModeButton(c, tc, icons.TablerCode, "Editor", a.Mode == ModeEditor, func() {
					a.Mode = ModeEditor
					a.AgentViewMode = AgentViewChat
				})
			})

			// 4px×16px divider (h-4 w-px bg-border/60)
			ui.Box(c).Size(1, 16).Background(tc.ToolbarBorder)

			// Workspace & panel controls: Reveal in Finder, Terminal, Right Pane, Appearance
			if IconButton(c, tc, icons.TablerFolder, 15).Clicked() {
				mygo.Shell.ShowItemInFolder(a.Workspace.RootDir)
			}
			if ToggleIconButton(c, tc, icons.TablerTerminal, 15, a.ShowBottomTerminal).Clicked() {
				a.ShowBottomTerminal = !a.ShowBottomTerminal
			}
			if ToggleIconButton(c, tc, icons.TablerPanelRight, 15, a.ShowRightDrawer).Clicked() {
				a.ShowRightDrawer = !a.ShowRightDrawer
			}
			if IconButton(c, tc, icons.TablerPalette, 15).Clicked() {
				a.ThemeMgr.Toggle()
			}
			_ = t
		})
}

// segmentedModeButton is one half of the header's segmented switch: h-6
// rounded-md px-2 font-medium; the active side is bg-card shadow-2xs with
// the accent icon.
func segmentedModeButton(c *ui.Context, tc theme.ThemeColors, icon *ui.SVG, label string, active bool, onPick func()) {
	btn := ui.Row(c).Height(24).Radius(6).Padding(0, 8).Gap(5).AlignItems(ui.Center).Justify(ui.Center)
	btn.Children(func() {
		ic := ui.Icon(c, icon).Width(14).Height(14)
		txt := ui.Text(c, label).FontSize(12).FontWeight(500)
		if active {
			ic.TextColor(tc.Accent)
			txt.TextColor(tc.Text)
		} else {
			ic.TextColor(tc.ToolbarControlForeground)
			txt.TextColor(tc.ToolbarControlForeground)
		}
	})
	if active {
		btn.Background(tc.SurfaceOverlay).
			Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.18))
	} else if btn.Hovered() {
		btn.Background(tc.ToolbarControlHover)
	}
	if btn.Clicked() {
		onPick()
	}
}
