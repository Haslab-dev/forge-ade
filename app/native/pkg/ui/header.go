package ui

import (
	"fmt"
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
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
	corestudio "github.com/hasdev/forge-ade/core/studio"
)

var (
	menuIconAgent    *mygo.MenuIcon
	menuIconEditor   *mygo.MenuIcon
	menuIconStudio   *mygo.MenuIcon
	menuIconFinder   *mygo.MenuIcon
	menuIconTerminal *mygo.MenuIcon
)

func init() {
	menuIconAgent, _ = mygo.NewMenuIconSVG(icons.TablerBot, true)
	menuIconEditor, _ = mygo.NewMenuIconSVG(icons.TablerCode, true)
	menuIconStudio, _ = mygo.NewMenuIconSVG(icons.TablerSparkles, true)
	menuIconFinder, _ = mygo.NewMenuIconSVG(icons.TablerFolder, true)
	menuIconTerminal, _ = mygo.NewMenuIconSVG(icons.TablerTerminal, true)
}

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
	a.setMode(ModeEditor)
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
	} else if a.Mode == ModeStudio {
		if a.Studio != nil && a.Studio.ActiveProject() != nil {
			title = "Studio: " + a.Studio.ActiveProject().Name
		} else {
			title = "Studio"
		}
	} else {
		if doc := a.Editor.ActiveDoc(); doc != nil {
			title = filepath.Base(doc.RelativePath)
		}
	}

	bar := c.TitleBar()
	leftPad := float32(12)
	showToggleInHeader := true
	if (a.Mode == ModeAgent || a.Mode == ModeStudio) && a.ShowLeftSidebar {
		// Full-height sidebar is visible on the left; navigation icons live in its header.
		showToggleInHeader = false
		leftPad = 14
	} else if bar.Left > 0 {
		// Clear macOS close/minimize/maximize traffic lights cleanly on the same Y baseline.
		leftPad = max(bar.Left+14, 88)
	} else if runtime.GOOS == "darwin" {
		leftPad = 88
	}

	// Keep leftPad tight in ModeAgent when the flush tab starts at the left edge.
	if a.Mode == ModeAgent && a.ShowLeftSidebar && !a.IsGitClientOpen && a.AgentViewMode == AgentViewChat {
		leftPad = 0
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

	hdrRow := ui.Row(c).Height(WorkspaceTopBarHeight).Gap(6).Padding(0, rightPad, 0, leftPad).
		AlignItems(ui.Center).
		Background(tc.Chrome).
		BorderWidth(0, 0, 1, 0).BorderColor(tc.ToolbarBorder)
	// Outside Agent Mode keep full-bar DragWindow; in Agent Mode the empty spacer
	// drags the window so right-click ContextMenu on session tabs is never swallowed.
	if a.Mode != ModeAgent {
		hdrRow.DragWindow()
	}
	hdrRow.Children(func() {
		// Left cluster (ZCode navigation & context controls)
		ui.Row(c).Grow(1).MinWidth(0).Gap(5).AlignItems(ui.Center).Clip().Children(func() {
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
			}

			// Project switcher button + active title (Zed style) - hidden in Agent mode
			if a.Mode != ModeAgent {
				projectBtn := ui.Row(c.Key("hdr-project")).Height(24).Radius(6).Padding(0, 6).Gap(5).
					AlignItems(ui.Center).Shrink(0).Cursor(ui.CursorPointer)
				projectBtn.Children(func() {
					icon := icons.TablerFolder
					if a.Workspace != nil && strings.HasPrefix(a.Workspace.RootDir, "ssh://") {
						icon = icons.TablerServer
					}
					ui.Icon(c, icon).Width(14).Height(14).TextColor(tc.IconMuted)
					ui.Text(c, projectName).FontSize(12).FontWeight(500).MaxWidth(110).SingleLine()
				})
				if projectBtn.Hovered() {
					projectBtn.Background(tc.ToolbarControlHover)
				}
				if projectBtn.Clicked() {
					a.OpenProjectSwitcherModal()
				}

				ui.Text(c, title).FontSize(13).FontWeight(600).MaxWidth(260).MinWidth(0).SingleLine()
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
					a.setMode(ModeAgent)
					a.IsGitClientOpen = false
					a.AgentViewMode = AgentViewChat
					a.ActiveTaskKind = "agent"
				}
			}

			// Orca-inspired multi-session closable tab strip in ModeAgent.
			if a.Mode == ModeAgent && !a.IsGitClientOpen && a.AgentViewMode == AgentViewChat {
				a.renderAgentHeaderTabStrip(c, tc)
			}

			// Studio Surface Quick-Switcher in ModeStudio: [Canvas | Image | Video]
			if a.Mode == ModeStudio {
				var project *corestudio.StudioProject
				if a.Studio != nil {
					project = a.Studio.ActiveProject()
				}
				ui.Row(c.Key("hdr-studio-surfaces")).Height(26).Gap(2).Padding(2).Radius(7).Shrink(0).
					Background(tc.ToolbarControl).BorderWidth(1).BorderColor(tc.ToolbarBorder).
					AlignItems(ui.Center).Children(func() {
					canvasActive := project == nil || (project.Type != corestudio.ArtifactTypeImage && project.Type != corestudio.ArtifactTypeVideo)
					headerSurfaceButton(c, tc, icons.TablerPresentation, "Canvas", 0, tc.Accent, canvasActive, func() {
						a.studioEnsureCanvasProject()
					})
					imageActive := project != nil && project.Type == corestudio.ArtifactTypeImage
					headerSurfaceButton(c, tc, icons.TablerPhoto, "Image", 0, tc.Accent, imageActive, func() {
						a.studioEnsureProjectForType(corestudio.ArtifactTypeImage)
					})
					videoActive := project != nil && project.Type == corestudio.ArtifactTypeVideo
					headerSurfaceButton(c, tc, icons.TablerSparkles, "Video", 0, tc.Accent, videoActive, func() {
						a.studioEnsureProjectForType(corestudio.ArtifactTypeVideo)
					})
				})
			}
		})

		// Live Agent Activity Pill when outside Chat (Editor, Board, Git, or CLI)
		if a.Agent != nil {
			sess := a.Agent.ActiveSession()
			needsApproval := a.sessionNeedsApproval(sess)
			isWorking := a.Agent.IsStreaming || (sess != nil && sess.Status == "running")
			outsideChat := (a.Mode != ModeAgent && a.Mode != ModeStudio) || a.AgentViewMode != AgentViewChat || a.IsGitClientOpen || a.ActiveTaskKind == "cli"
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
					a.setMode(ModeAgent)
					a.IsGitClientOpen = false
					a.AgentViewMode = AgentViewChat
					a.ActiveTaskKind = "agent"
				}
			}
		}

		winW, _ := c.Size()
		_ = winW

		// In Agent Mode with Split Pane active, global trailing window controls
		// live at the far right edge of the split pane header (Orca style),
		// keeping the left pane header pure ([tabs ✕] [+] [•••]).
		splitOpen := a.Mode == ModeAgent && a.AgentSplitSessionID != "" && !a.IsGitClientOpen && a.AgentViewMode == AgentViewChat

		if a.Mode == ModeAgent && !a.IsGitClientOpen && a.AgentViewMode == AgentViewChat {
			activeID := ""
			if a.Agent != nil {
				activeID = a.Agent.ActiveSessionID
			}
			activeIdx := -1
			for i, id := range a.AgentOpenTabs {
				if id == activeID {
					activeIdx = i
					break
				}
			}
			moreBtn := IconButton(c, tc, icons.TablerDots, 14).Shrink(0).Tooltip("Pane & Tab Actions")
			moreBtn.Menu(func(m *ui.Menu) {
				if activeID != "" {
					if m.Item("Hand off session…").Chosen() {
						a.OpenHandoffModal()
					}
					if m.Item("Spawn parallel worker…").Chosen() {
						a.OpenSpawnWorkerModal()
					}
					if m.Item(menuChecked(splitOpen, "Split Pane")).Chosen() {
						a.toggleAgentSplit()
					}
					m.Separator()
					if m.Item("Move Left").Disabled(activeIdx <= 0).Chosen() {
						a.moveAgentTab(activeID, -1)
					}
					if m.Item("Move Right").Disabled(activeIdx < 0 || activeIdx >= len(a.AgentOpenTabs)-1).Chosen() {
						a.moveAgentTab(activeID, 1)
					}
					m.Separator()
					if m.Item("Close").Chosen() {
						a.closeAgentTab(activeID)
					}
					if m.Item("Close Others").Disabled(len(a.AgentOpenTabs) <= 1).Chosen() {
						a.AgentOpenTabs = []string{activeID}
					}
					if m.Item("Close Left").Disabled(activeIdx <= 0).Chosen() {
						a.closeAgentTabsToLeft(activeID)
					}
					if m.Item("Close Right").Disabled(activeIdx < 0 || activeIdx >= len(a.AgentOpenTabs)-1).Chosen() {
						a.closeAgentTabsToRight(activeID)
					}
					if m.Item("Close All").Chosen() {
						a.closeAllAgentTabs()
					}
				} else {
					if m.Item("New task").Shortcut(ui.Cmd, ui.KeyN).Chosen() {
						a.handleNewTask()
					}
					if m.Item("Spawn parallel worker…").Chosen() {
						a.OpenSpawnWorkerModal()
					}
				}
			})
			if !splitOpen {
				if ToggleIconButton(c, tc, icons.TablerColumns, 14, false).Shrink(0).Tooltip("Split Pane (move active tab right)").Clicked() {
					a.toggleAgentSplit()
				}
			}
		}

		if !splitOpen {
			a.renderGlobalHeaderTrailingControls(c, tc)
		}
	})
}

// renderGlobalHeaderTrailingControls renders the window's far-right controls
// (Mode switcher, Settings when sidebar hidden, Bottom Terminal, Right Drawer).
func (a *AppUI) renderGlobalHeaderTrailingControls(c *ui.Context, tc theme.ThemeColors) {
	// Mode switcher dropdown: icon only for Agent, Editor, Studio
	modeIcon := icons.TablerBot
	switch a.Mode {
	case ModeEditor:
		modeIcon = icons.TablerCode
	case ModeStudio:
		modeIcon = icons.TablerSparkles
	default:
		modeIcon = icons.TablerBot
	}

	modeBtn := ui.Row(c.Key("hdr-mode-dropdown")).Height(24).Radius(6).Padding(0, 6).Gap(3).Shrink(0).
		AlignItems(ui.Center).Cursor(ui.CursorPointer)
	modeBtn.Children(func() {
		ui.Icon(c, modeIcon).Width(13).Height(13).TextColor(tc.IconMuted)
		ui.Icon(c, icons.TablerChevronDown).Width(10).Height(10).TextColor(tc.IconMuted)
	})
	if modeBtn.Hovered() {
		modeBtn.Background(tc.ToolbarControlHover)
	}
	modeBtn.Menu(func(m *ui.Menu) {
		if m.Item(menuChecked(a.Mode == ModeAgent, "Agent Chat Mode")).Shortcut(ui.Cmd, ui.Key1).Icon(menuIconAgent).Chosen() {
			a.setMode(ModeAgent)
			a.IsGitClientOpen = false
			a.AgentViewMode = AgentViewChat
			a.ActiveTaskKind = "agent"
		}
		if m.Item(menuChecked(a.Mode == ModeEditor, "Editor Mode")).Shortcut(ui.Cmd, ui.Key2).Icon(menuIconEditor).Chosen() {
			a.setMode(ModeEditor)
			a.IsGitClientOpen = false
		}
		if m.Item(menuChecked(a.Mode == ModeStudio, "Studio Mode")).Shortcut(ui.Cmd, ui.Key3).Icon(menuIconStudio).Chosen() {
			a.setMode(ModeStudio)
			a.IsGitClientOpen = false
		}
		m.Separator()
		if m.Item("Finder").Shortcut(ui.Cmd|ui.Alt, ui.KeyR).Icon(menuIconFinder).Chosen() {
			a.ExternalOpenTarget = "finder"
			a.openInFinder("")
		}
		if m.Item("Terminal (External)").Shortcut(ui.Cmd|ui.Alt, ui.KeyT).Icon(menuIconTerminal).Chosen() {
			a.ExternalOpenTarget = "terminal"
			a.openExternalTerminal("")
		}
	})

	if IconButton(c, tc, icons.TablerSettings, 14).Shrink(0).Tooltip("Settings (⌘,)").Clicked() {
		if a.Mode == ModeStudio {
			a.openSettings(settings.SettingsTabStudio)
		} else {
			a.openSettings(settings.SettingsTabGeneral)
		}
	}
	if ts := a.displayedTerminal(); ts != nil {
		if urls := a.termVisibleURLsCached(ts); len(urls) > 0 {
			if IconButton(c, tc, icons.TablerExternalLink, 14).Shrink(0).Tooltip("Open Terminal URLs (⇧⌘L)").Clicked() {
				fresh := ts.VisibleURLs()
				if len(fresh) == 0 {
					fresh = urls
				}
				a.OpenTerminalURLModal(fresh)
			}
		}
	}
	if ToggleIconButton(c, tc, icons.TablerTerminal, 14, a.isBottomTerminalVisible()).Shrink(0).Tooltip("Toggle Bottom Terminal (⌘`)").Clicked() {
		a.toggleBottomTerminal()
	}
	if !a.ShowRightDrawer && a.Mode != ModeEditor {
		if ToggleIconButton(c, tc, icons.TablerPanelRight, 14, a.ShowRightDrawer).Shrink(0).Tooltip("Toggle Right Drawer (⌘J)").Clicked() {
			a.ShowRightDrawer = !a.ShowRightDrawer
		}
	}
}

// headerSurfaceButton is one segment of the Agent Surface Quick-Switcher:
// icon + label with an optional count badge; the active segment is raised
// with a card surface.
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
