package ui

import (
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/agent"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
)

// gitDirtySummary returns the working tree dirty file count and total +/- line counts.
func (a *AppUI) gitDirtySummary() (files, additions, deletions int) {
	if a.Git == nil || a.Workspace == nil || a.Workspace.RootDir == "" {
		return 0, 0, 0
	}
	gm := a.Git
	files = len(gm.Conflicts) + len(gm.Staged) + len(gm.Unstaged) + len(gm.Untracked)
	for _, f := range gm.Staged {
		if f.Additions > 0 {
			additions += int(f.Additions)
		}
		if f.Deletions > 0 {
			deletions += int(f.Deletions)
		}
	}
	for _, f := range gm.Unstaged {
		if f.Additions > 0 {
			additions += int(f.Additions)
		}
		if f.Deletions > 0 {
			deletions += int(f.Deletions)
		}
	}
	return files, additions, deletions
}

// boardTaskCount returns the total number of tasks loaded on the workflow board.
func (a *AppUI) boardTaskCount() int {
	if a.Board == nil || a.Board.Data == nil {
		return 0
	}
	total := 0
	for _, col := range a.Board.Data.Columns {
		total += len(col.Tasks)
	}
	return total
}

// sessionNeedsApproval reports whether sess has an unresolved tool confirmation.
func (a *AppUI) sessionNeedsApproval(sess *agent.AgentSession) bool {
	if sess == nil {
		return false
	}
	if sess.Status == "awaiting-approval" || sess.Status == "waiting_permission" {
		return true
	}
	if a.Agent != nil && a.Agent.ActiveSessionID == sess.ID {
		for _, m := range a.Agent.Messages {
			if m.PendingApproval != nil && !m.PendingApproval.Resolved {
				return true
			}
		}
	}
	return false
}

// renderWorkspaceHeader renders ForgeADE's WorkspaceHeader: 52px tall, the
// chrome background, title, project/branch chips, agent surface switcher,
// live agent activity pill, segmented mode switcher, and 28px icon controls.
func (a *AppUI) renderWorkspaceHeader(c *ui.Context) {
	t := c.Theme()
	tc := a.ThemeMgr.Colors()
	projectName := filepath.Base(a.Workspace.RootDir)
	if a.Workspace.RootDir == "" {
		projectName = "General"
	}

	// Derive display title
	title := projectName
	if a.Mode == ModeAgent {
		switch {
		case a.AgentViewMode == AgentViewBoard:
			title = "Board"
		case a.IsGitClientOpen || a.AgentViewMode == AgentViewGit:
			title = "Git"
		case a.ActiveTaskKind == "cli" && a.CLIMgr != nil && a.CLIMgr.ActiveSession() != nil:
			title = a.CLIMgr.ActiveSession().Title
		case a.Agent != nil && a.Agent.ActiveSession() != nil:
			title = a.Agent.ActiveSession().Title
		default:
			title = "New task"
		}
	} else {
		if doc := a.Editor.ActiveDoc(); doc != nil {
			title = filepath.Base(doc.RelativePath)
		}
	}

	dirtyFiles, _, _ := a.gitDirtySummary()
	boardTasks := a.boardTaskCount()

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

	ui.Row(c).Height(WorkspaceTopBarHeight).Gap(8).Padding(0, rightPad, 0, leftPad).
		AlignItems(ui.Center).
		Background(tc.Chrome).
		BorderWidth(0, 0, 1, 0).BorderColor(tc.ToolbarBorder).
		DragWindow().
		Children(func() {
			// Left cluster (can shrink cleanly before crowding right controls)
			ui.Row(c).Grow(1).MinWidth(0).Gap(8).AlignItems(ui.Center).Clip().Children(func() {
				if showToggleInHeader {
					if IconButton(c, tc, icons.TablerPanelLeft, 16).Shrink(0).Clicked() {
						a.ShowLeftSidebar = !a.ShowLeftSidebar
					}
				}

				ui.Text(c, title).FontSize(13).FontWeight(600).MaxWidth(220).MinWidth(0).SingleLine()

				// Always-visible Project Switcher chip (both Agent & Editor modes)
				projectBtn := ui.Row(c.Key("hdr-project")).Height(26).Radius(6).Padding(0, 8).Gap(5).
					AlignItems(ui.Center).Shrink(0).Background(tc.ToolbarControl).Cursor(ui.CursorPointer)
				projectBtn.Children(func() {
					ui.Icon(c, icons.TablerFolder).Width(13).Height(13).TextColor(tc.IconMuted)
					ui.Text(c, projectName).FontSize(12).FontWeight(500).MaxWidth(130).SingleLine()
					ui.Icon(c, icons.TablerChevronDown).Width(11).Height(11).TextColor(tc.IconMuted)
				})
				if projectBtn.Hovered() {
					projectBtn.Background(tc.ToolbarControlHover)
				}
				projectBtn.Menu(func(m *ui.Menu) {
					a.workspaceMenuItems(m)
				})

				// Always-visible Git Branch Switcher chip (when a workspace is open)
				if a.Workspace != nil && a.Workspace.RootDir != "" {
					branchName := "main"
					if a.Git != nil && a.Git.Branch != "" {
						branchName = a.Git.Branch
					}
					branchBtn := ui.Row(c.Key("hdr-branch")).Height(26).Radius(6).Padding(0, 8).Gap(5).
						AlignItems(ui.Center).Shrink(0).Background(tc.ToolbarControl).Cursor(ui.CursorPointer)
					branchBtn.Children(func() {
						ui.Icon(c, icons.TablerGitBranch).Width(13).Height(13).TextColor(tc.Accent)
						ui.Text(c, branchName).FontSize(12).FontWeight(500).TextColor(tc.Accent).MaxWidth(110).SingleLine()
						if dirtyFiles > 0 {
							ui.Row(c).Height(16).Radius(4).Padding(0, 5).AlignItems(ui.Center).
								Background(tc.Warning.Alpha(0.16)).Children(func() {
								ui.Text(c, fmt.Sprintf("%d", dirtyFiles)).FontSize(10).FontWeight(600).TextColor(tc.Warning)
							})
						}
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
					if IconButton(c, tc, icons.TablerDots, 15).Shrink(0).Clicked() {
						a.ShowTaskMenuModal = true
					}
				}
			})

			// Live Agent Activity Pill when outside Chat (Editor, Board, Git, or CLI)
			if a.Agent != nil {
				sess := a.Agent.ActiveSession()
				needsApproval := a.sessionNeedsApproval(sess)
				isWorking := a.Agent.IsStreaming || (sess != nil && sess.Status == "running")
				outsideChat := a.Mode != ModeAgent || a.AgentViewMode != AgentViewChat || a.IsGitClientOpen || a.ActiveTaskKind == "cli"
				if outsideChat && (needsApproval || isWorking) {
					dotColor := tc.Accent
					pillLabel := "Agent working…"
					bgAlpha := float32(0.14)
					if needsApproval {
						dotColor = tc.Warning
						pillLabel = "Approval needed"
						bgAlpha = 0.16
					}
					livePill := ui.Row(c.Key("hdr-agent-live")).Height(26).Radius(999).Padding(0, 9).Gap(6).
						AlignItems(ui.Center).Shrink(0).Cursor(ui.CursorPointer).
						Background(dotColor.Alpha(bgAlpha)).BorderWidth(1).BorderColor(dotColor.Alpha(0.35))
					livePill.Children(func() {
						ui.Box(c).Size(6, 6).Radius(3).Background(dotColor)
						ui.Text(c, pillLabel).FontSize(11).FontWeight(600).TextColor(dotColor).SingleLine()
					})
					if livePill.Hovered() {
						livePill.Background(dotColor.Alpha(0.24))
					}
					if livePill.Clicked() {
						a.Mode = ModeAgent
						a.IsGitClientOpen = false
						a.AgentViewMode = AgentViewChat
						a.ActiveTaskKind = "agent"
					}
				}
			}

			// Agent Surface Quick-Switcher in ModeAgent: [Chat | Board | Git]
			if a.Mode == ModeAgent {
				ui.Row(c.Key("hdr-agent-surfaces")).Height(28).Gap(2).Padding(2).Radius(8).Shrink(0).
					Background(tc.ToolbarControl).BorderWidth(1).BorderColor(tc.ToolbarBorder).
					AlignItems(ui.Center).Children(func() {
					chatActive := !a.IsGitClientOpen && a.AgentViewMode == AgentViewChat && a.ActiveTaskKind != "cli"
					headerSurfaceButton(c, tc, icons.TablerMessage, "Chat", 0, tc.Accent, chatActive, func() {
						a.IsGitClientOpen = false
						a.AgentViewMode = AgentViewChat
						a.ActiveTaskKind = "agent"
					})
					boardActive := !a.IsGitClientOpen && a.AgentViewMode == AgentViewBoard
					headerSurfaceButton(c, tc, icons.TablerLayoutKanban, "Board", boardTasks, tc.Accent, boardActive, func() {
						a.IsGitClientOpen = false
						a.ActiveTaskKind = "agent"
						a.openBoardSurface()
					})
					gitActive := a.IsGitClientOpen || a.AgentViewMode == AgentViewGit
					headerSurfaceButton(c, tc, icons.TablerGitBranch, "Git", dirtyFiles, tc.Warning, gitActive, func() {
						a.IsGitClientOpen = false
						a.ActiveTaskKind = "agent"
						a.openGitClientSurface()
					})
				})
			}

			// Primary Segmented Mode Switcher: [✨ Agent] | [💻 Editor] (⌘1 / ⌘2)
			ui.Row(c).Height(28).Gap(2).Padding(2).Radius(8).Shrink(0).
				Background(tc.ToolbarControl).
				BorderWidth(1).BorderColor(tc.ToolbarBorder).
				AlignItems(ui.Center).Children(func() {
				segmentedModeButton(c, tc, icons.TablerSparkles, "Agent", a.Mode == ModeAgent, func() {
					a.Mode = ModeAgent
					a.IsGitClientOpen = false
					a.AgentViewMode = AgentViewChat
				})
				segmentedModeButton(c, tc, icons.TablerCode, "Editor", a.Mode == ModeEditor, func() {
					a.Mode = ModeEditor
					a.IsGitClientOpen = false
					a.AgentViewMode = AgentViewChat
				})
			})

			// 1px×16px divider
			ui.Box(c).Size(1, 16).Shrink(0).Background(tc.ToolbarBorder)

			// Workspace & panel controls: Reveal in Finder, Terminal, Right Pane, Appearance, Settings
			if IconButton(c, tc, icons.TablerFolder, 15).Shrink(0).Clicked() {
				mygo.Shell.ShowItemInFolder(a.Workspace.RootDir)
			}
			if ToggleIconButton(c, tc, icons.TablerTerminal, 15, a.ShowBottomTerminal).Shrink(0).Clicked() {
				a.ShowBottomTerminal = !a.ShowBottomTerminal
			}
			if ToggleIconButton(c, tc, icons.TablerPanelRight, 15, a.ShowRightDrawer).Shrink(0).Clicked() {
				a.ShowRightDrawer = !a.ShowRightDrawer
			}
			if IconButton(c, tc, icons.TablerPalette, 15).Shrink(0).Clicked() {
				a.ThemeMgr.Toggle()
			}
			if IconButton(c, tc, icons.TablerSettings, 15).Shrink(0).Clicked() {
				a.openSettings()
			}
			_ = t
		})
}

// headerSurfaceButton renders a compact surface pill with an optional count badge inside ModeAgent.
func headerSurfaceButton(c *ui.Context, tc theme.ThemeColors, icon *ui.SVG, label string, count int, badgeColor ui.Color, active bool, onPick func()) {
	btn := ui.Row(c.Key("hsurf-"+label)).Height(24).Radius(6).Padding(0, 7).Gap(4).
		AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
	btn.Children(func() {
		ic := ui.Icon(c, icon).Width(13).Height(13)
		txt := ui.Text(c, label).FontSize(11).FontWeight(500).SingleLine()
		if active {
			ic.TextColor(tc.Accent)
			txt.TextColor(tc.Text).FontWeight(600)
		} else {
			ic.TextColor(tc.ToolbarControlForeground)
			txt.TextColor(tc.ToolbarControlForeground)
		}
		if count > 0 {
			ui.Row(c).Height(15).Radius(4).Padding(0, 4).AlignItems(ui.Center).
				Background(badgeColor.Alpha(0.15)).Children(func() {
				ui.Text(c, fmt.Sprintf("%d", count)).FontSize(9).FontWeight(600).TextColor(badgeColor)
			})
		}
	})
	if active {
		btn.Background(tc.SurfaceOverlay).
			Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.16))
	} else if btn.Hovered() {
		btn.Background(tc.ToolbarControlHover)
	}
	if btn.Clicked() {
		onPick()
	}
}

// segmentedModeButton is one half of the header's segmented switch: h-6
// rounded-md px-2 font-medium; the active side is bg-card shadow-2xs with
// the accent icon.
func segmentedModeButton(c *ui.Context, tc theme.ThemeColors, icon *ui.SVG, label string, active bool, onPick func()) {
	btn := ui.Row(c).Height(24).Radius(6).Padding(0, 8).Gap(5).AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
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
