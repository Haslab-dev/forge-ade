package ui

// turn_status_card.go renders the ZCode-style floating turn-status card:
// a compact overlay above the composer summarizing the working turn —
// working-tree Changes with diffstat, the current Branch, a Commit-or-push
// jump, the live task Progress checklist, and the active Agents rows with
// per-agent elapsed time and Stop buttons.

import (
	"fmt"
	"strings"
	"time"

	rakitagent "github.com/Haslab-dev/RakitKode/agent"
	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/agent"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
)

// turnStatusCardWidth is the card's fixed width; it hugs the composer's
// trailing edge like ZCode's overlay.
const turnStatusCardWidth float32 = 400

// subagentRunning reports whether an RK session state means actively working.
func subagentRunning(state string) bool {
	switch strings.ToLower(fmt.Sprintf("%v", state)) {
	case "running", "working", "thinking", "executing":
		return true
	}
	return false
}

// renderTurnStatusToggle draws the slim strip shown above the composer when
// the card is closed and there is something worth showing (changes, task
// progress, or running agents). Clicking it opens the floating card.
func (a *AppUI) renderTurnStatusToggle(c *ui.Context, tc theme.ThemeColors, sessionID string) {
	if a.ShowTurnStatusCard {
		return
	}
	files, additions, deletions := 0, 0, 0
	hasGit := false
	if a.Git != nil && a.Workspace != nil && a.Workspace.RootDir != "" {
		files, additions, deletions = a.gitDirtySummary()
		hasGit = true
	}
	tasks := a.agentProgressTasks()
	runningSubs := a.activeSubagents(sessionID)
	goal := a.agentGoalText()

	hasWork := len(tasks) > 0 || len(runningSubs) > 0 || (hasGit && files > 0) || goal != ""
	if !hasWork {
		return
	}

	strip := ui.Row(c.Key("turn-status-toggle")).Height(26).Radius(8).Padding(0, 10).Gap(8).
		AlignItems(ui.Center).FillWidth().Justify(ui.Start).Cursor(ui.CursorPointer).
		Background(tc.SurfaceRaised.Alpha(0.6)).BorderWidth(1).BorderColor(tc.Border.Alpha(0.5))
	strip.Children(func() {
		ui.Icon(c, icons.TablerFileDiff).Width(13).Height(13).TextColor(tc.TextMuted)
		if hasGit && files > 0 {
			ui.Text(c, fmt.Sprintf("%d file%s", files, pluralS(files))).FontSize(11).TextColor(tc.TextMuted)
			ui.Text(c, fmt.Sprintf("+%d", additions)).FontSize(11).FontWeight(600).TextColor(tc.Success)
			ui.Text(c, fmt.Sprintf("-%d", deletions)).FontSize(11).FontWeight(600).TextColor(tc.Error)
		}
		if len(tasks) > 0 {
			done := 0
			for _, t := range tasks {
				if t.Status == agent.TaskCompleted {
					done++
				}
			}
			ui.Text(c, fmt.Sprintf("Progress %d/%d", done, len(tasks))).FontSize(11).FontWeight(500).TextColor(tc.TextMuted)
		}
		if n := len(runningSubs); n > 0 {
			ui.Text(c, fmt.Sprintf("Agents %d running", n)).FontSize(11).FontWeight(500).TextColor(tc.Accent)
		}
		ui.Box(c).Size(1, 12).Background(tc.Border.Alpha(0.5))
		ui.Icon(c, icons.TablerChevronUp).Width(12).Height(12).TextColor(tc.TextMuted)
	})
	if strip.Hovered() {
		strip.Background(tc.SurfaceRaised)
	}
	if strip.Clicked() {
		a.ShowTurnStatusCard = true
		a.invalidateUI()
	}
}

// agentGoalText returns the active session's plan/goal summary if set.
func (a *AppUI) agentGoalText() string {
	if a.Agent == nil {
		return ""
	}
	tm := a.Agent.ActiveTaskManager()
	if tm == nil {
		return ""
	}
	return strings.TrimSpace(tm.GoalText())
}

// agentProgressTasks returns a copy of the active session's checklist.
func (a *AppUI) agentProgressTasks() []agent.TaskItem {
	if a.Agent == nil {
		return nil
	}
	tm := a.Agent.ActiveTaskManager()
	if tm == nil {
		return nil
	}
	return tm.GetTasks()
}

// activeSubagents returns the active session's subagents, split into the
// running ones first. Also stamps first-seen times for elapsed rendering.
func (a *AppUI) activeSubagents(sessionID string) []*rakitagent.Session {
	if a.Agent == nil || sessionID == "" {
		return nil
	}
	subs := a.Agent.Subagents(sessionID)
	if len(subs) == 0 {
		return nil
	}
	var running []*rakitagent.Session
	now := time.Now()
	for _, s := range subs {
		if s == nil {
			continue
		}
		if subagentRunning(string(s.State)) {
			running = append(running, s)
		}
		if a.subagentFirstSeen == nil {
			a.subagentFirstSeen = make(map[string]time.Time)
		}
		if _, ok := a.subagentFirstSeen[s.ID]; !ok {
			a.subagentFirstSeen[s.ID] = now
		}
	}
	return running
}

// renderTurnStatusCard draws the floating card in the top-right corner of the
// chat pane: Changes header + diffstat, branch row, commit jump, Goal, task
// Progress checklist, and active Agents rows with Stop buttons.
func (a *AppUI) renderTurnStatusCard(c *ui.Context, tc theme.ThemeColors, sessionID string) {
	if !a.ShowTurnStatusCard {
		return
	}
	card := ui.Column(c.Key("turn-status-card")).Absolute().Top(12).Right(16).
		Width(turnStatusCardWidth).MaxHeight(520).Radius(14).
		Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border).
		Shadow(0, 24, 64, -24, ui.RGBA(0, 0, 0, 0.55)).Clip()
	card.Children(func() {
		ui.Scroll(c).MaxHeight(520).Children(func() {
			a.renderTurnStatusSections(c, tc, sessionID)
		})
	})
}

func (a *AppUI) renderTurnStatusSections(c *ui.Context, tc theme.ThemeColors, sessionID string) {
	// ── Changes header ────────────────────────────────────────────────
	files, additions, deletions := 0, 0, 0
	hasGit := false
	branch := ""
	if a.Git != nil && a.Workspace != nil && a.Workspace.RootDir != "" {
		files, additions, deletions = a.gitDirtySummary()
		snap := a.Git.Snapshot()
		branch = snap.Branch
		hasGit = true
	}

	header := ui.Row(c).Height(38).Padding(0, 14).Gap(8).AlignItems(ui.Center)
	header.Children(func() {
		ui.Icon(c, icons.TablerFileDiff).Width(15).Height(15).TextColor(tc.Text)
		ui.Text(c, "Changes").FontSize(13).FontWeight(600).TextColor(tc.Text).Grow(1)
		if hasGit && files > 0 {
			ui.Text(c, fmt.Sprintf("+%d", additions)).FontSize(12).FontWeight(600).TextColor(tc.Success)
			ui.Text(c, fmt.Sprintf("-%d", deletions)).FontSize(12).FontWeight(600).TextColor(tc.Error)
		}
		closeBtn := ui.Row(c.Key("turn-status-close")).Size(22, 22).Radius(6).AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
		closeBtn.Children(func() {
			ui.Icon(c, icons.TablerChevronDown).Width(13).Height(13).TextColor(tc.TextMuted)
		})
		if closeBtn.Hovered() {
			closeBtn.Background(tc.ToolbarControlHover)
		}
		if closeBtn.Clicked() {
			a.ShowTurnStatusCard = false
			a.invalidateUI()
		}
	})

	goal := a.agentGoalText()
	tasks := a.agentProgressTasks()
	runningSubs := a.activeSubagents(sessionID)
	hasAny := (hasGit && files > 0) || branch != "" || len(tasks) > 0 || goal != "" || len(runningSubs) > 0
	if !hasAny {
		ui.Column(c).Padding(10, 14).Children(func() {
			ui.Text(c, "Nothing to show yet — make a change or run a task.").FontSize(11).TextColor(tc.TextMuted)
		})
		return
	}

	// ── Branch row ────────────────────────────────────────────────────
	if hasGit && branch != "" {
		branchRow := ui.Row(c.Key("turn-status-branch")).Height(30).Padding(0, 14).Gap(8).AlignItems(ui.Center).Cursor(ui.CursorPointer)
		branchRow.Children(func() {
			ui.Icon(c, icons.TablerGitBranch).Width(13).Height(13).TextColor(tc.TextMuted)
			ui.Text(c, branch).FontSize(12).FontWeight(500).TextColor(tc.Text)
			ui.Icon(c, icons.TablerChevronDown).Width(11).Height(11).TextColor(tc.TextMuted)
		})
		if branchRow.Hovered() {
			branchRow.Background(tc.ToolbarControlHover)
		}
		if branchRow.Clicked() {
			a.ShowBranchModal = true
			a.BranchSearchQuery = ""
			if a.Git != nil {
				a.Git.RefreshAsync()
			}
		}

		// ── Commit or push row ────────────────────────────────────────
		commitRow := ui.Row(c.Key("turn-status-commit")).Height(30).Padding(0, 14).Gap(8).AlignItems(ui.Center).Cursor(ui.CursorPointer)
		commitRow.Children(func() {
			ui.Icon(c, icons.TablerGitCommit).Width(13).Height(13).TextColor(tc.TextMuted)
			ui.Text(c, "Commit or push").FontSize(12).TextColor(tc.Text)
		})
		if commitRow.Hovered() {
			commitRow.Background(tc.ToolbarControlHover)
		}
		if commitRow.Clicked() {
			a.ShowTurnStatusCard = false
			a.openGitClientSurface()
			a.invalidateUI()
		}
	}

	a.turnStatusDivider(c, tc)

	// ── Goal / Plan section ───────────────────────────────────────────
	if goal != "" {
		head := ui.Row(c).Height(26).Padding(0, 14).Gap(8).AlignItems(ui.Center)
		head.Children(func() {
			ui.Text(c, "Goal").FontSize(12).FontWeight(600).TextColor(tc.Text).Grow(1)
		})
		ui.Row(c).Padding(4, 14, 6, 14).Gap(8).AlignItems(ui.Start).Children(func() {
			ui.Icon(c, icons.TablerSparkles).Width(13).Height(13).TextColor(tc.Accent)
			ui.Text(c, goal).FontSize(12).TextColor(tc.Text).Grow(1)
		})
		a.turnStatusDivider(c, tc)
	}

	// ── Progress checklist ────────────────────────────────────────────
	if len(tasks) > 0 {
		done := 0
		for _, t := range tasks {
			if t.Status == agent.TaskCompleted {
				done++
			}
		}
		head := ui.Row(c).Height(26).Padding(0, 14).Gap(8).AlignItems(ui.Center)
		head.Children(func() {
			ui.Text(c, "Progress").FontSize(12).FontWeight(600).TextColor(tc.Text).Grow(1)
			ui.Text(c, fmt.Sprintf("%d/%d", done, len(tasks))).FontSize(12).TextColor(tc.TextMuted)
		})
		for i, t := range tasks {
			task := t
			row := ui.Row(c.Key(fmt.Sprintf("turn-status-task-%s-%d", task.ID, i))).Padding(6, 14).Gap(8).AlignItems(ui.Start)
			row.Children(func() {
				switch task.Status {
				case agent.TaskCompleted:
					ui.Box(c).Size(15, 15).Radius(8).AlignItems(ui.Center).Justify(ui.Center).
						Background(tc.Success.Alpha(0.15)).Children(func() {
						ui.Icon(c, icons.TablerCheck).Width(10).Height(10).TextColor(tc.Success)
					})
				case agent.TaskInProgress:
					ui.Icon(c, icons.TablerArrowRight).Width(15).Height(15).TextColor(tc.Accent)
				default:
					ui.Box(c).Size(14, 14).Radius(7).BorderWidth(1.5).BorderColor(tc.TextMuted.Alpha(0.5))
				}
				label := task.Title
				if task.Status == agent.TaskInProgress && task.ActiveForm != "" {
					label = task.ActiveForm
				}
				txt := ui.Text(c, label).FontSize(12)
				if task.Status == agent.TaskCompleted {
					txt.TextColor(tc.TextMuted)
				} else {
					txt.TextColor(tc.Text)
				}
			})
		}
		a.turnStatusDivider(c, tc)
	}

	// ── Agents section ────────────────────────────────────────────────
	if len(runningSubs) > 0 {
		head := ui.Row(c).Height(26).Padding(0, 14).Gap(8).AlignItems(ui.Center)
		head.Children(func() {
			ui.Text(c, "Agents").FontSize(12).FontWeight(600).TextColor(tc.Text).Grow(1)
			ui.Text(c, fmt.Sprintf("%d running", len(runningSubs))).FontSize(11).TextColor(tc.Accent)
		})
		for _, sub := range runningSubs {
			s := sub
			elapsed := time.Since(a.subagentFirstSeen[s.ID]).Truncate(time.Second)
			row := ui.Column(c.Key("turn-status-sub-"+s.ID)).Padding(6, 14).Gap(2)
			row.Children(func() {
				line := ui.Row(c).Gap(8).AlignItems(ui.Center).FillWidth()
				line.Children(func() {
					ui.Icon(c, icons.TablerBot).Width(14).Height(14).TextColor(tc.TextMuted)
					ui.Text(c, subDisplayName(s)).FontSize(12).FontWeight(500).TextColor(tc.Text).Grow(1).SingleLine()
					stopBtn := ui.Row(c.Key("turn-status-stop-"+s.ID)).Height(22).Radius(6).Padding(0, 8).Gap(4).
						AlignItems(ui.Center).Cursor(ui.CursorPointer).Background(tc.Surface)
					stopBtn.Children(func() {
						ui.Box(c).Size(8, 8).Radius(2).Background(tc.Error)
						ui.Text(c, "Stop").FontSize(10).FontWeight(600).TextColor(tc.Error)
					})
					if stopBtn.Hovered() {
						stopBtn.Background(tc.Error.Alpha(0.12))
					}
					if stopBtn.Clicked() {
						a.Agent.StopSubagent(sessionID, s.ID)
						a.queueToast(ui.Toast{Title: "Subagent Stopped", Description: subDisplayName(s), Type: "info"})
						a.invalidateUI()
					}
				})
				ui.Text(c, fmt.Sprintf("Running for %s", formatElapsedNice(elapsed))).FontSize(10.5).TextColor(tc.TextMuted)
			})
		}
	} else if len(tasks) == 0 {
		ui.Text(c, "No agents running.").FontSize(11).TextColor(tc.TextMuted)
	}
}

func (a *AppUI) turnStatusDivider(c *ui.Context, tc theme.ThemeColors) {
	ui.Box(c).Height(1).FillWidth().Margin(4, 0).Background(tc.Border.Alpha(0.45))
}

// subDisplayName picks the best display name for a subagent session.
func subDisplayName(s *rakitagent.Session) string {
	if s == nil {
		return "Agent"
	}
	if strings.TrimSpace(s.Name) != "" {
		return s.Name
	}
	return "Agent " + s.ID
}

// formatElapsedNice renders "5m 31s" / "42s" / "1h 04m".
func formatElapsedNice(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	total := int(d.Seconds())
	if total < 60 {
		return fmt.Sprintf("%ds", total)
	}
	if total < 3600 {
		return fmt.Sprintf("%dm %02ds", total/60, total%60)
	}
	return fmt.Sprintf("%dh %02dm", total/3600, (total%3600)/60)
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
