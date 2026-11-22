package ui

// status_panel.go is the ZCode ConversationStatusPanel port: one floating
// status surface for the chat's top-right corner. Collapsed it is a capsule
// pill whose line follows a priority chain (Changes → Progress → Agents);
// expanded it is a 320pt rounded panel with collapsible Changes / Progress /
// Agents sections. It replaces the old live-changes pill, the task-tracker
// pill/card, and the turn-status card — those fed the same data through
// three separate surfaces.

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/agent"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
)

// statusPanelWidth matches ZCode's w-80 (320px) expanded panel.
const statusPanelWidth float32 = 320

// renderStatusPanel draws the capsule or the expanded panel depending on
// a.ShowTurnStatusCard ("panel expanded"). Hidden entirely when there is
// nothing to show — ZCode hides the panel when the model has no content.
func (a *AppUI) renderStatusPanel(c *ui.Context, tc theme.ThemeColors, msgs []*agent.ChatMessage) {
	sessionID := ""
	if a.Agent != nil {
		sessionID = a.Agent.SnapshotActiveSessionID()
	}
	st := a.collectStatusData(sessionID, msgs)
	if st.isEmpty() {
		return
	}
	if a.ShowTurnStatusCard {
		a.renderStatusPanelExpanded(c, tc, sessionID, st)
	} else {
		a.renderStatusCapsule(c, tc, st)
	}
}

// statusData snapshots everything the panel shows, gathered in one pass.
type statusData struct {
	add, del, files int
	hasChanges      bool
	tasks           []agent.TaskItem
	runningSubs     []*agentSubRow
}

// agentSubRow is a display row for one running subagent.
type agentSubRow struct {
	id      string
	name    string
	role    string
	elapsed time.Duration
}

func (st *statusData) isEmpty() bool {
	return !st.hasChanges && len(st.tasks) == 0 && len(st.runningSubs) == 0
}

// collectStatusData gathers changes (live working tree while running, else
// the last turn's Changes), the task checklist and running subagents.
func (a *AppUI) collectStatusData(sessionID string, msgs []*agent.ChatMessage) statusData {
	var st statusData
	add, del, files, ok := a.liveChangesStats(msgs)
	if ok {
		st.add, st.del, st.files, st.hasChanges = add, del, files, true
	}
	st.tasks = a.agentProgressTasks()
	for _, s := range a.activeSubagents(sessionID) {
		if s == nil {
			continue
		}
		elapsed := time.Duration(0)
		if seen, seenOK := a.subagentFirstSeen[s.ID]; seenOK {
			elapsed = time.Since(seen).Truncate(time.Second)
		}
		st.runningSubs = append(st.runningSubs, &agentSubRow{
			id:      s.ID,
			name:    subDisplayName(s),
			role:    strings.TrimSpace(string(s.RoleFilter)),
			elapsed: elapsed,
		})
	}
	return st
}

// statusSectionOpen returns the per-section collapse state (default open).
func (a *AppUI) statusSectionOpen(key string) bool {
	if a.StatusSectionOpen == nil {
		return true
	}
	return a.StatusSectionOpen[key]
}

func (a *AppUI) toggleStatusSection(key string) {
	if a.StatusSectionOpen == nil {
		a.StatusSectionOpen = make(map[string]bool)
	}
	a.StatusSectionOpen[key] = !a.statusSectionOpen(key)
}

// ── Capsule ──────────────────────────────────────────────────────────────

// renderStatusCapsule draws the collapsed pill. Its single line follows the
// ZCode priority chain: Git changes → current todo progress → agents.
func (a *AppUI) renderStatusCapsule(c *ui.Context, tc theme.ThemeColors, st statusData) {
	pill := ui.Row(c.Key("status-capsule")).Absolute().Top(12).Right(16).Height(30).Radius(999).
		Padding(0, 12).Gap(7).AlignItems(ui.Center).Cursor(ui.CursorPointer).
		Background(tc.SurfaceOverlay).BorderWidth(1).BorderColor(tc.Border).
		Shadow(0, 4, 12, 0, ui.RGBA(0, 0, 0, 0.18))
	pill.Children(func() {
		ui.Text(c, "Status").FontSize(11.5).FontWeight(600).TextColor(tc.Text)
		ui.Box(c).Size(1, 12).Background(tc.Border.Alpha(0.6))
		switch {
		case st.hasChanges:
			ui.Icon(c, icons.TablerFileDiff).Width(13).Height(13).TextColor(tc.TextMuted)
			ui.Text(c, fmt.Sprintf("+%d", st.add)).FontSize(11).FontWeight(600).TextColor(tc.DiffAdditionForeground)
			ui.Text(c, fmt.Sprintf("-%d", st.del)).FontSize(11).FontWeight(600).TextColor(tc.DiffDeletionForeground)
		case len(st.tasks) > 0:
			done := countDoneTasks(st.tasks)
			ui.Icon(c, icons.TablerChecklist).Width(13).Height(13).TextColor(tc.TextMuted)
			ui.Text(c, fmt.Sprintf("%d/%d", done, len(st.tasks))).FontSize(11).FontWeight(600).TextColor(tc.TextMuted)
		case len(st.runningSubs) > 0:
			faceName := "agents"
			agentFace(c, tc, faceName, "running")
			ui.Text(c, fmt.Sprintf("%d running", len(st.runningSubs))).FontSize(11).FontWeight(500).TextColor(tc.TextMuted)
		}
		ui.Icon(c, icons.TablerMaximize2).Width(12).Height(12).TextColor(tc.IconMuted.Alpha(0.7))
	})
	if pill.Hovered() {
		pill.Background(tc.SurfaceRaised)
	}
	// Hover swaps the trailing glyph to the expand affordance — ZCode's
	// hover-to-Maximize2 behavior. Layout is stable because the icon slot
	// is fixed size.
	if pill.Clicked() {
		a.ShowTurnStatusCard = true
		a.invalidateUI()
	}
}

// ── Expanded panel ───────────────────────────────────────────────────────

func (a *AppUI) renderStatusPanelExpanded(c *ui.Context, tc theme.ThemeColors, sessionID string, st statusData) {
	panel := ui.Column(c.Key("status-panel")).Absolute().Top(12).Right(16).
		Width(statusPanelWidth).MaxHeight(520).Radius(16).
		Background(tc.SurfaceOverlay).BorderWidth(1).BorderColor(tc.Border).
		Shadow(0, 24, 64, -24, ui.RGBA(0, 0, 0, 0.5)).Clip()
	panel.Children(func() {
		// Header: title + collapse-to-capsule button.
		header := ui.Row(c).Height(40).Padding(0, 14).Gap(8).AlignItems(ui.Center).
			BorderWidth(0, 0, 1, 0).BorderColor(tc.Border.Alpha(0.5))
		header.Children(func() {
			ui.Text(c, "Status").FontSize(13).FontWeight(600).TextColor(tc.Text).Grow(1)
			closeBtn := ui.Row(c.Key("status-panel-collapse")).Size(22, 22).Radius(6).
				AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
			closeBtn.Children(func() {
				ui.Icon(c, icons.TablerMinimize).Width(13).Height(13).TextColor(tc.TextMuted)
			})
			if closeBtn.Hovered() {
				closeBtn.Background(tc.ToolbarControlHover)
			}
			if closeBtn.Clicked() {
				a.ShowTurnStatusCard = false
				a.invalidateUI()
			}
		})

		ui.Scroll(c).MaxHeight(480).Children(func() {
			body := ui.Column(c).Padding(6, 6, 10, 6).Gap(2)
			body.Children(func() {
				if st.hasChanges {
					a.statusChangesSection(c, tc, st)
				}
				if len(st.tasks) > 0 {
					a.statusProgressSection(c, tc, st.tasks)
				}
				if len(st.runningSubs) > 0 {
					a.statusAgentsSection(c, tc, sessionID, st)
				}
			})
		})
	})
}

// statusSectionHeader draws one collapsible section header row.
func (a *AppUI) statusSectionHeader(c *ui.Context, tc theme.ThemeColors, key, title string, trailing func()) {
	head := ui.Row(c.Key("status-sec-"+key)).Height(30).Radius(8).Padding(0, 8).Gap(7).
		AlignItems(ui.Center).Cursor(ui.CursorPointer)
	head.Children(func() {
		chev := icons.TablerChevronRight
		if a.statusSectionOpen(key) {
			chev = icons.TablerChevronDown
		}
		ui.Icon(c, chev).Width(12).Height(12).TextColor(tc.IconMuted)
		ui.Text(c, title).FontSize(12).FontWeight(600).TextColor(tc.Text)
		ui.Box(c).Grow(1)
		if trailing != nil {
			trailing()
		}
	})
	if head.Hovered() {
		head.Background(tc.Text.Alpha(0.04))
	}
	if head.Clicked() {
		a.toggleStatusSection(key)
	}
}

// statusChangesSection: the "Diff" entry — diffstat row that opens the
// right drawer's Changes view.
func (a *AppUI) statusChangesSection(c *ui.Context, tc theme.ThemeColors, st statusData) {
	a.statusSectionHeader(c, tc, "changes", "Changes", func() {
		ui.Text(c, fmt.Sprintf("%d file%s", st.files, pluralS(st.files))).FontSize(11).TextColor(tc.TextMuted)
		ui.Text(c, fmt.Sprintf("+%d", st.add)).FontSize(11).FontWeight(600).TextColor(tc.DiffAdditionForeground)
		ui.Text(c, fmt.Sprintf("-%d", st.del)).FontSize(11).FontWeight(600).TextColor(tc.DiffDeletionForeground)
	})
	if !a.statusSectionOpen("changes") {
		return
	}
	row := ui.Row(c.Key("status-changes-row")).Height(30).Radius(8).Padding(0, 8).Gap(8).
		AlignItems(ui.Center).Cursor(ui.CursorPointer)
	row.Children(func() {
		ui.Icon(c, icons.TablerFileDiff).Width(14).Height(14).TextColor(tc.TextMuted)
		ui.Text(c, "Working tree changes").FontSize(12).TextColor(tc.Text).Grow(1).SingleLine()
		ui.Icon(c, icons.TablerChevronRight).Width(12).Height(12).TextColor(tc.IconMuted.Alpha(0.6))
	})
	if row.Hovered() {
		row.Background(tc.Text.Alpha(0.04))
	}
	if row.Clicked() {
		if a.Git != nil {
			a.Git.RefreshAsync()
		}
		a.RightDrawerTab = RightTabChanges
		a.ShowRightDrawer = true
		a.invalidateUI()
	}
}

// statusProgressSection: the task-todo checklist, ZCode PlanStatusSection
// style — check / arrow / circle glyphs, completed = struck-through muted.
func (a *AppUI) statusProgressSection(c *ui.Context, tc theme.ThemeColors, tasks []agent.TaskItem) {
	done := countDoneTasks(tasks)
	a.statusSectionHeader(c, tc, "progress", "Progress", func() {
		color := tc.TextMuted
		if done == len(tasks) && len(tasks) > 0 {
			color = tc.Success
		}
		ui.Text(c, fmt.Sprintf("%d/%d", done, len(tasks))).FontSize(11).FontWeight(600).TextColor(color)
	})
	if !a.statusSectionOpen("progress") {
		return
	}
	for i, t := range tasks {
		task := t
		row := ui.Row(c.Key(fmt.Sprintf("status-task-%s-%d", task.ID, i))).Padding(5, 8).Gap(8).
			AlignItems(ui.Start)
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
			txt := ui.Text(c, label).FontSize(12).Grow(1)
			switch task.Status {
			case agent.TaskCompleted:
				txt.TextColor(tc.TextMuted)
			case agent.TaskInProgress:
				txt.TextColor(tc.Text)
			default:
				txt.TextColor(tc.TextMuted.Alpha(0.85))
			}
		})
	}
}

// statusAgentsSection: running subagent rows with face tiles, elapsed and
// Stop; row click opens the child in the right drawer.
func (a *AppUI) statusAgentsSection(c *ui.Context, tc theme.ThemeColors, sessionID string, st statusData) {
	longest := time.Duration(0)
	for _, s := range st.runningSubs {
		if s.elapsed > longest {
			longest = s.elapsed
		}
	}
	a.statusSectionHeader(c, tc, "agents", "Agents", func() {
		if longest > 0 {
			ui.Text(c, formatWorkDuration(longest)).FontSize(11).TextColor(tc.TextMuted)
		}
		ui.Text(c, fmt.Sprintf("%d running", len(st.runningSubs))).FontSize(11).FontWeight(500).TextColor(tc.Accent)
	})
	if !a.statusSectionOpen("agents") {
		return
	}
	for _, sub := range st.runningSubs {
		s := sub
		row := ui.Column(c.Key("status-agent-"+s.id)).Padding(5, 8).Gap(2)
		row.Children(func() {
			line := ui.Row(c).Gap(8).AlignItems(ui.Center).FillWidth()
			line.Children(func() {
				ui.Box(c).Size(20, 20).Children(func() {
					agentFace(c, tc, s.name, "running")
				})
				ui.Text(c, s.name).FontSize(12).FontWeight(500).TextColor(tc.Text).Grow(1).SingleLine()
				stopBtn := ui.Row(c.Key("status-agent-stop-"+s.id)).Height(22).Radius(6).Padding(0, 8).Gap(4).
					AlignItems(ui.Center).Cursor(ui.CursorPointer).Background(tc.Surface)
				stopBtn.Children(func() {
					ui.Box(c).Size(8, 8).Radius(2).Background(tc.Error)
					ui.Text(c, "Stop").FontSize(10).FontWeight(600).TextColor(tc.Error)
				})
				if stopBtn.Hovered() {
					stopBtn.Background(tc.Error.Alpha(0.12))
				}
				if stopBtn.Clicked() {
					a.Agent.StopSubagent(sessionID, s.id)
					a.queueToast(ui.Toast{Title: "Subagent Stopped", Description: s.name, Type: "info"})
					a.invalidateUI()
				}
			})
			second := ui.Row(c).Padding(0, 0, 0, 28).Gap(6).AlignItems(ui.Center)
			second.Children(func() {
				if s.role != "" {
					ui.Text(c, s.role).FontSize(10.5).TextColor(tc.TextMuted).SingleLine()
				}
				ui.Text(c, fmt.Sprintf("for %s", formatWorkDuration(s.elapsed))).FontSize(10.5).TextColor(tc.TextMuted)
			})
		})
		// Whole-row open behavior, minus the Stop button's own clicks.
		if row.Clicked() {
			a.RightDrawerTab = RightTabSubagent
			a.ShowRightDrawer = true
			a.SelectedSubagentID = s.id
			a.invalidateUI()
		}
	}
}

// countDoneTasks counts checklist items in the completed state.
func countDoneTasks(tasks []agent.TaskItem) int {
	done := 0
	for _, t := range tasks {
		if t.Status == agent.TaskCompleted {
			done++
		}
	}
	return done
}
