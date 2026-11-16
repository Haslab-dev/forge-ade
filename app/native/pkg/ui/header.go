package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/agent"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/settings"
)

func (a *AppUI) resolveExternalTargetDir(targetPath string) string {
	dir := targetPath
	if dir == "" && a.Workspace != nil {
		dir = a.Workspace.RootDir
	}
	if dir == "" || strings.HasPrefix(dir, "ssh://") {
		home, err := os.UserHomeDir()
		if err == nil {
			dir = home
		}
	}
	return dir
}

// openExternalTerminal opens the workspace folder in the OS terminal app.
func (a *AppUI) openExternalTerminal(targetPath string) {
	dir := a.resolveExternalTargetDir(targetPath)
	if runtime.GOOS == "darwin" {
		cmd := exec.Command("/usr/bin/open", "-a", "Terminal", dir)
		if err := cmd.Start(); err != nil {
			_ = exec.Command("/usr/bin/open", dir).Start()
		}
		return
	}
	if runtime.GOOS == "windows" {
		cmd := exec.Command("cmd", "/c", "start", "wt", "-d", dir)
		if err := cmd.Start(); err != nil {
			_ = exec.Command("cmd", "/c", "start", "cmd", "/K", "cd", "/d", dir).Start()
		}
		return
	}
	for _, term := range []string{"x-terminal-emulator", "gnome-terminal", "konsole", "alacritty", "kitty", "xterm"} {
		if path, err := exec.LookPath(term); err == nil {
			cmd := exec.Command(path)
			cmd.Dir = dir
			if err := cmd.Start(); err == nil {
				return
			}
		}
	}
}

// openInFinder reveals the workspace folder in macOS Finder.
func (a *AppUI) openInFinder(targetPath string) {
	dir := a.resolveExternalTargetDir(targetPath)
	mygo.Shell.ShowItemInFolder(dir)
}

// openExternalEditor opens the workspace folder in an external editor like Zed / VS Code.
func (a *AppUI) openExternalEditor(targetPath string) {
	dir := a.resolveExternalTargetDir(targetPath)
	if runtime.GOOS == "darwin" {
		if err := exec.Command("/usr/bin/open", "-a", "Zed", dir).Start(); err == nil {
			return
		}
		if err := exec.Command("/usr/bin/open", "-a", "Visual Studio Code", dir).Start(); err == nil {
			return
		}
	}
	a.Mode = ModeEditor
	a.IsGitClientOpen = false
}

// gitDirtySummary returns the working tree dirty file count and total +/- line counts.
func (a *AppUI) gitDirtySummary() (files, additions, deletions int) {
	if a.Git == nil || a.Workspace == nil || a.Workspace.RootDir == "" {
		return 0, 0, 0
	}
	snap := a.Git.Snapshot()
	files = len(snap.Conflicts) + len(snap.Staged) + len(snap.Unstaged) + len(snap.Untracked)
	for _, f := range snap.Staged {
		if f.Additions > 0 {
			additions += int(f.Additions)
		}
		if f.Deletions > 0 {
			deletions += int(f.Deletions)
		}
	}
	for _, f := range snap.Unstaged {
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

// renderWorkspaceHeader renders ZCode's minimal 42px center workspace header:
// navigation/new-task controls when the left sidebar is collapsed, project
// switcher + active title + actions menu, split open-in pill, and right-side
// copy, help, terminal, and right-sidebar toggle buttons.
func (a *AppUI) renderWorkspaceHeader(c *ui.Context) {
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

	bar := c.TitleBar()
	leftPad := float32(12)
	showToggleInHeader := true
	if a.Mode == ModeAgent && a.ShowLeftSidebar {
		// Full-height sidebar is visible on the left; navigation icons live in its header.
		showToggleInHeader = false
		leftPad = 14
	} else if bar.Left > 0 {
		// Clear macOS close/minimize/maximize traffic lights cleanly on the same Y baseline.
		leftPad = max(bar.Left+10, 78)
	} else if runtime.GOOS == "darwin" {
		leftPad = 78
	}

	rightPad := float32(10)
	if !(a.Mode == ModeAgent && a.ShowRightDrawer) {
		if bar.Right > 0 {
			rightPad = max(bar.Right+10, 135)
		} else if bar.Left == 0 && (runtime.GOOS == "linux" || runtime.GOOS == "windows") {
			// On Linux/Windows window buttons are on the right (minimize, maximize, close)
			rightPad = 135
		}
	}

	ui.Row(c).Height(WorkspaceTopBarHeight).Gap(6).Padding(0, rightPad, 0, leftPad).
		AlignItems(ui.Center).
		Background(tc.Chrome).
		BorderWidth(0, 0, 1, 0).BorderColor(tc.ToolbarBorder).
		DragWindow().
		Children(func() {
			// Left cluster (ZCode navigation & context controls)
			ui.Row(c).Grow(1).MinWidth(0).Gap(5).AlignItems(ui.Center).Clip().Children(func() {
				var activeSess *agent.AgentSession
				if a.Agent != nil {
					activeSess = a.Agent.ActiveSession()
				}

				if showToggleInHeader {
					if IconButton(c, tc, icons.TablerPanelLeft, 15).Shrink(0).Clicked() {
						a.ShowLeftSidebar = !a.ShowLeftSidebar
					}
					if IconButton(c, tc, icons.TablerArrowLeft, 14).Shrink(0).Clicked() {
						a.cycleAgentSession(-1)
					}
					if IconButton(c, tc, icons.TablerArrowRight, 14).Shrink(0).Clicked() {
						a.cycleAgentSession(1)
					}
					if IconButton(c, tc, icons.TablerCirclePlus, 15).Shrink(0).Clicked() {
						a.handleNewTask()
					}

					// Compact agent/turn status circle badge
					isStreaming := a.Agent != nil && a.Agent.IsStreaming
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
				}

				// Project switcher button + active title (Zed style)
				projectBtn := ui.Row(c.Key("hdr-project")).Height(24).Radius(6).Padding(0, 6).Gap(5).
					AlignItems(ui.Center).Shrink(0).Cursor(ui.CursorPointer)
				projectBtn.Children(func() {
					icon := icons.TablerFolder
					if a.Workspace != nil && strings.HasPrefix(a.Workspace.RootDir, "ssh://") {
						icon = icons.TablerServer
					}
					ui.Icon(c, icon).Width(14).Height(14).TextColor(tc.IconMuted)
					if a.Mode != ModeAgent {
						ui.Text(c, projectName).FontSize(12).FontWeight(500).MaxWidth(110).SingleLine()
					}
				})
				if projectBtn.Hovered() {
					projectBtn.Background(tc.ToolbarControlHover)
				}
				if projectBtn.Clicked() {
					a.OpenProjectSwitcherModal()
				}

				ui.Text(c, title).FontSize(13).FontWeight(600).MaxWidth(260).MinWidth(0).SingleLine()

				// Title "…" context menu (Task Actions, Git Branch, Board, Git, Editor)
				if a.Mode == ModeAgent {
					moreBtn := ui.Row(c.Key("hdr-more")).Size(24, 24).Radius(6).Shrink(0).
						AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
					moreBtn.Children(func() {
						ui.Icon(c, icons.TablerDots).Width(14).Height(14).TextColor(tc.IconMuted)
					})
					if moreBtn.Hovered() {
						moreBtn.Background(tc.ToolbarControlHover)
					}
					moreBtn.Menu(func(m *ui.Menu) {
						if a.Agent != nil && activeSess != nil {
							if m.Item("Task Actions (Rename, Archive, Delete)...").Chosen() {
								a.ShowTaskMenuModal = true
							}
							m.Separator()
						}
						if m.Item("Switch Git Branch...").Chosen() {
							a.ShowBranchModal = true
							a.BranchSearchQuery = ""
							if a.Git != nil {
								a.Git.RefreshAsync()
							}
						}
						if m.Item("Open Workflow Board").Chosen() {
							a.openBoardSurface()
						}
						if m.Item("Open Git Client").Chosen() {
							a.openGitClientSurface()
						}
						m.Separator()
						if m.Item("Switch to Editor Mode").Shortcut(ui.Cmd, ui.Key2).Chosen() {
							a.Mode = ModeEditor
							a.IsGitClientOpen = false
						}
					})
				}

				// One-click return pill when viewing Editor, Board, or Git Client
				if a.Mode == ModeEditor || a.AgentViewMode == AgentViewBoard || a.AgentViewMode == AgentViewGit || a.IsGitClientOpen {
					backPill := ui.Row(c.Key("hdr-back-chat")).Height(24).Radius(999).Padding(0, 9).Gap(4).
						AlignItems(ui.Center).Shrink(0).Cursor(ui.CursorPointer).
						Background(tc.ToolbarControl).BorderWidth(1).BorderColor(tc.ToolbarBorder)
					backPill.Children(func() {
						ui.Icon(c, icons.TablerArrowLeft).Width(12).Height(12).TextColor(tc.Text)
						ui.Text(c, "Agent Chat").FontSize(11).FontWeight(500).SingleLine()
					})
					if backPill.Hovered() {
						backPill.Background(tc.ToolbarControlHover)
					}
					if backPill.Clicked() {
						a.Mode = ModeAgent
						a.IsGitClientOpen = false
						a.AgentViewMode = AgentViewChat
						a.ActiveTaskKind = "agent"
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

			// ZCode Split Quick-Action Pill: [ Mode / Target | ▾ ]
			target := a.ExternalOpenTarget
			if target == "" {
				target = "editor"
			}
			targetIcon := icons.TablerCode
			switch target {
			case "finder":
				targetIcon = icons.TablerFolder
			case "terminal":
				targetIcon = icons.TablerTerminal
			default:
				targetIcon = icons.TablerCode
			}

			splitPill := ui.Row(c.Key("hdr-split-term")).Height(26).Radius(7).Shrink(0).AlignItems(ui.Center).
				Background(tc.ToolbarControl).BorderWidth(1).BorderColor(tc.ToolbarBorder)
			splitPill.Children(func() {
				termHalf := ui.Row(c.Key("hdr-split-left")).Height(24).Padding(0, 8).AlignItems(ui.Center).Cursor(ui.CursorPointer)
				termHalf.Children(func() {
					ui.Icon(c, targetIcon).Width(13).Height(13).TextColor(tc.ToolbarControlForeground)
				})
				if termHalf.Hovered() {
					termHalf.Background(tc.ToolbarControlHover)
				}
				if termHalf.Clicked() {
					switch target {
					case "finder":
						a.openInFinder("")
					case "terminal":
						a.openExternalTerminal("")
					default:
						a.Mode = ModeEditor
						a.IsGitClientOpen = false
					}
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
					// Mode Switcher in Header Dropdown
					if m.Item(menuChecked(a.Mode == ModeAgent, "Agent Chat Mode")).Shortcut(ui.Cmd, ui.Key1).Chosen() {
						a.Mode = ModeAgent
						a.IsGitClientOpen = false
						a.AgentViewMode = AgentViewChat
						a.ActiveTaskKind = "agent"
					}
					if m.Item(menuChecked(a.Mode == ModeEditor, "Editor Mode")).Shortcut(ui.Cmd, ui.Key2).Chosen() {
						a.Mode = ModeEditor
						a.IsGitClientOpen = false
					}
					m.Separator()

					// Quick Target Selectors / Launchers
					if m.Item(menuChecked(target == "editor", "Editor Mode")).Chosen() {
						a.ExternalOpenTarget = "editor"
						a.Mode = ModeEditor
						a.IsGitClientOpen = false
					}
					if m.Item(menuChecked(target == "finder", "Finder")).Chosen() {
						a.ExternalOpenTarget = "finder"
						a.openInFinder("")
					}
					if m.Item(menuChecked(target == "terminal", "Terminal (External)")).Chosen() {
						a.ExternalOpenTarget = "terminal"
						a.openExternalTerminal("")
					}
				})
			})

			// Trailing ZCode utility icons: Help, Terminal (internal bottom drawer), and Right Sidebar toggle
			if IconButton(c, tc, icons.TablerHelp, 14).Shrink(0).Clicked() {
				a.openSettings(settings.SettingsTabShortcuts)
			}
			if ToggleIconButton(c, tc, icons.TablerTerminal, 14, a.ShowBottomTerminal).Shrink(0).Clicked() {
				a.ShowBottomTerminal = !a.ShowBottomTerminal
			}
			if !(a.Mode == ModeAgent && a.ShowRightDrawer) {
				if ToggleIconButton(c, tc, icons.TablerPanelRight, 14, a.ShowRightDrawer).Shrink(0).Clicked() {
					a.ShowRightDrawer = !a.ShowRightDrawer
				}
			}
		})
}
