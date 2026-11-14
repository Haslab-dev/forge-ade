package ui

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/agent"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/settings"
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
	projectName := resolveProjectName(a.Workspace.RootDir)

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
	leftPad := float32(12)
	showToggleInHeader := true
	if a.Mode == ModeAgent && a.ShowLeftSidebar {
		// Full-height sidebar is visible on the left; header sits beside it.
		showToggleInHeader = false
		leftPad = 10
	} else if bar.Left > 0 {
		// Clear macOS close/minimize/maximize traffic lights cleanly on the same Y baseline.
		leftPad = max(bar.Left+10, 78)
	} else if runtime.GOOS == "darwin" {
		leftPad = 78
	}

	rightPad := float32(10)
	if bar.Right > 0 {
		rightPad = max(bar.Right+10, 135)
	} else if bar.Left == 0 && (runtime.GOOS == "linux" || runtime.GOOS == "windows") {
		// On Linux/Windows window buttons are on the right (minimize, maximize, close)
		rightPad = 135
	}

	ui.Row(c).Height(WorkspaceTopBarHeight).Gap(6).Padding(0, rightPad, 0, leftPad).
		AlignItems(ui.Center).
		Background(tc.Chrome).
		BorderWidth(0, 0, 1, 0).BorderColor(tc.ToolbarBorder).
		DragWindow().
		Children(func() {
			// Left cluster (ZCode navigation & context controls)
			ui.Row(c).Grow(1).MinWidth(0).Gap(5).AlignItems(ui.Center).Clip().Children(func() {
				if showToggleInHeader {
					if IconButton(c, tc, icons.TablerPanelLeft, 15).Shrink(0).Clicked() {
						a.ShowLeftSidebar = !a.ShowLeftSidebar
					}
				}

				// Back / Forward session history buttons
				if IconButton(c, tc, icons.TablerArrowLeft, 14).Shrink(0).Clicked() {
					a.cycleAgentSession(-1)
				}
				if IconButton(c, tc, icons.TablerArrowRight, 14).Shrink(0).Clicked() {
					a.cycleAgentSession(1)
				}

				// New Task (+) circle button
				if IconButton(c, tc, icons.TablerCirclePlus, 15).Shrink(0).Clicked() {
					a.handleNewTask()
				}

				// Compact agent/turn status circle badge
				isStreaming := a.Agent != nil && a.Agent.IsStreaming
				var activeSess *agent.AgentSession
				if a.Agent != nil {
					activeSess = a.Agent.ActiveSession()
				}
				needsApproval := a.sessionNeedsApproval(activeSess)
				badgeBg := tc.Success.Alpha(0.18)
				badgeColor := tc.Success
				if needsApproval {
					badgeBg = tc.Warning.Alpha(0.18)
					badgeColor = tc.Warning
				} else if isStreaming {
					badgeBg = tc.Accent.Alpha(0.18)
					badgeColor = tc.Accent
				}
				statusCircle := ui.Row(c.Key("hdr-status-circle")).Size(18, 18).Radius(9).
					Shrink(0).AlignItems(ui.Center).Justify(ui.Center).Background(badgeBg).Cursor(ui.CursorPointer)
				statusCircle.Children(func() {
					if isStreaming {
						ui.Spinner(c).Size(10, 10).Label("running")
					} else if needsApproval {
						ui.Icon(c, icons.TablerAlertTriangle).Width(10).Height(10).TextColor(badgeColor)
					} else {
						ui.Icon(c, icons.TablerArrowDown).Width(10).Height(10).TextColor(badgeColor)
					}
				})
				if statusCircle.Clicked() {
					a.Mode = ModeAgent
					a.IsGitClientOpen = false
					a.AgentViewMode = AgentViewChat
				}

				// Project switcher button + active title
				projectBtn := ui.Row(c.Key("hdr-project")).Height(24).Radius(6).Padding(0, 6).Gap(5).
					AlignItems(ui.Center).Shrink(0).Cursor(ui.CursorPointer)
				projectBtn.Children(func() {
					ui.Icon(c, icons.TablerFolder).Width(14).Height(14).TextColor(tc.IconMuted)
					if a.Mode != ModeAgent {
						ui.Text(c, projectName).FontSize(12).FontWeight(500).MaxWidth(110).SingleLine()
					}
				})
				if projectBtn.Hovered() {
					projectBtn.Background(tc.ToolbarControlHover)
				}
				projectBtn.Menu(func(m *ui.Menu) {
					a.workspaceMenuItems(m)
				})

				ui.Text(c, title).FontSize(13).FontWeight(600).MaxWidth(220).MinWidth(0).SingleLine()

				// Task "…" menu (Agent mode with an active session)
				if a.Mode == ModeAgent && a.Agent != nil && activeSess != nil {
					if IconButton(c, tc, icons.TablerDots, 14).Shrink(0).Clicked() {
						a.ShowTaskMenuModal = true
					}
				}

				// Compact Git Branch badge (when a workspace is open)
				if a.Workspace != nil && a.Workspace.RootDir != "" {
					branchName := "main"
					if a.Git != nil && a.Git.Branch != "" {
						branchName = a.Git.Branch
					}
					branchBtn := ui.Row(c.Key("hdr-branch")).Height(24).Radius(6).Padding(0, 6).Gap(4).
						AlignItems(ui.Center).Shrink(0).Background(tc.ToolbarControl).Cursor(ui.CursorPointer)
					branchBtn.Children(func() {
						ui.Icon(c, icons.TablerGitBranch).Width(12).Height(12).TextColor(tc.Accent)
						ui.Text(c, branchName).FontSize(11).FontWeight(500).TextColor(tc.Accent).MaxWidth(90).SingleLine()
						if dirtyFiles > 0 {
							ui.Row(c).Height(15).Radius(4).Padding(0, 4).AlignItems(ui.Center).
								Background(tc.Warning.Alpha(0.16)).Children(func() {
								ui.Text(c, fmt.Sprintf("%d", dirtyFiles)).FontSize(9).FontWeight(600).TextColor(tc.Warning)
							})
						}
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
					livePill := ui.Row(c.Key("hdr-agent-live")).Height(24).Radius(999).Padding(0, 8).Gap(5).
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

			// ZCode Split Quick-Action Pill: [ >_ | ▾ ]
			splitPill := ui.Row(c.Key("hdr-split-term")).Height(26).Radius(7).Shrink(0).AlignItems(ui.Center).
				Background(tc.ToolbarControl).BorderWidth(1).BorderColor(tc.ToolbarBorder)
			splitPill.Children(func() {
				termHalf := ui.Row(c.Key("hdr-split-left")).Height(24).Padding(0, 8).AlignItems(ui.Center).Cursor(ui.CursorPointer)
				termIconColor := tc.ToolbarControlForeground
				if a.ShowBottomTerminal {
					termIconColor = tc.Accent
				}
				termHalf.Children(func() {
					ui.Icon(c, icons.TablerTerminal).Width(13).Height(13).TextColor(termIconColor)
				})
				if termHalf.Hovered() {
					termHalf.Background(tc.ToolbarControlHover)
				}
				if termHalf.Clicked() {
					a.ShowBottomTerminal = !a.ShowBottomTerminal
				}

				ui.Box(c).Size(1, 14).Background(tc.ToolbarBorder)

				dropHalf := ui.Row(c.Key("hdr-split-drop")).Height(24).Padding(0, 6).AlignItems(ui.Center).Cursor(ui.CursorPointer)
				dropHalf.Children(func() {
					ui.Icon(c, icons.TablerChevronDown).Width(11).Height(11).TextColor(tc.IconMuted)
				})
				if dropHalf.Hovered() {
					dropHalf.Background(tc.ToolbarControlHover)
				}
				dropHalf.Menu(func(m *ui.Menu) {
					if m.Item("Toggle Bottom Terminal").Shortcut(ui.Cmd, ui.KeyBackquote).Chosen() {
						a.ShowBottomTerminal = !a.ShowBottomTerminal
					}
					if m.Item("Terminal Snippets...").Shortcut(ui.Alt, ui.KeyS).Chosen() {
						sessID := ""
						if a.Terminal != nil {
							sessID = a.Terminal.ActiveSessionID
						}
						a.OpenTerminalSnippetsModal(sessID, "", "")
					}
					m.Separator()
					if m.Item("Switch to Editor Mode").Shortcut(ui.Cmd, ui.Key2).Chosen() {
						a.Mode = ModeEditor
						a.IsGitClientOpen = false
					}
					if m.Item("Switch to Agent Chat").Shortcut(ui.Cmd, ui.Key1).Chosen() {
						a.Mode = ModeAgent
						a.IsGitClientOpen = false
						a.AgentViewMode = AgentViewChat
					}
					if m.Item("Open Workflow Board").Chosen() {
						a.openBoardSurface()
					}
					if m.Item("Open Git Client").Chosen() {
						a.openGitClientSurface()
					}
					if a.Workspace != nil && a.Workspace.RootDir != "" {
						m.Separator()
						if m.Item("Reveal Project in Finder").Chosen() {
							mygo.Shell.ShowItemInFolder(a.Workspace.RootDir)
						}
					}
				})
			})

			// Agent Surface Quick-Switcher in ModeAgent: [Chat | Board | Git]
			if a.Mode == ModeAgent {
				ui.Row(c.Key("hdr-agent-surfaces")).Height(26).Gap(2).Padding(2).Radius(7).Shrink(0).
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
			ui.Row(c).Height(26).Gap(2).Padding(2).Radius(7).Shrink(0).
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
			ui.Box(c).Size(1, 14).Shrink(0).Background(tc.ToolbarBorder)

			// Trailing ZCode utility icons: Export/Copy, Help, Terminal, Inspector/Drawer, Palette, Settings
			if IconButton(c, tc, icons.TablerUpload, 14).Shrink(0).Clicked() {
				if a.Agent != nil {
					var sb strings.Builder
					for _, msg := range a.Agent.Messages {
						if msg != nil && msg.Text != "" {
							sb.WriteString(msg.Role)
							sb.WriteString(": ")
							sb.WriteString(msg.Text)
							sb.WriteString("\n\n")
						}
					}
					if out := strings.TrimSpace(sb.String()); out != "" {
						mygo.Clipboard.WriteText(out)
						c.AddToast(ui.Toast{Title: "Thread Copied", Description: title, Type: "success"})
					}
				}
			}
			if IconButton(c, tc, icons.TablerHelp, 14).Shrink(0).Clicked() {
				a.openSettings(settings.SettingsTabShortcuts)
			}
			if ToggleIconButton(c, tc, icons.TablerTerminal, 14, a.ShowBottomTerminal).Shrink(0).Clicked() {
				a.ShowBottomTerminal = !a.ShowBottomTerminal
			}
			inspectorActive := a.ShowRightDrawer || (a.Mode == ModeAgent && !a.InspectorCollapsed)
			if ToggleIconButton(c, tc, icons.TablerPanelRight, 14, inspectorActive).Shrink(0).Clicked() {
				if a.Mode == ModeAgent && a.AgentViewMode == AgentViewChat && !a.ShowRightDrawer {
					a.InspectorCollapsed = !a.InspectorCollapsed
				} else {
					a.ShowRightDrawer = !a.ShowRightDrawer
				}
			}
			if IconButton(c, tc, icons.TablerPalette, 14).Shrink(0).Clicked() {
				a.ThemeMgr.Toggle()
			}
			if IconButton(c, tc, icons.TablerSettings, 14).Shrink(0).Clicked() {
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
