package ui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/agent"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
)

// AgentBottomBarHeight is the slim Orca-inspired status & quota strip height.
const AgentBottomBarHeight = float32(26)

// attentionSummary counts all sessions currently awaiting approval/input and
// returns the first one so the user can jump directly to it.
func (a *AppUI) attentionSummary() (int, *agent.AgentSession) {
	if a.Agent == nil {
		return 0, nil
	}
	count := 0
	var first *agent.AgentSession
	for _, s := range a.Agent.Sessions {
		if s == nil || s.Archived {
			continue
		}
		if a.sessionNeedsApproval(s) {
			count++
			if first == nil {
				first = s
			}
		}
	}
	return count, first
}

// renderAgentBottomBar draws the Orca-inspired slim bottom status & live quota
// strip in Agent Mode: per-window quota meters on the left, and host / branch /
// running-workers / "Needs you" indicators on the right.
func (a *AppUI) renderAgentBottomBar(c *ui.Context) {
	tc := a.ThemeMgr.Colors()

	ui.Row(c.Key("agent-bottom-bar")).Height(AgentBottomBarHeight).Shrink(0).
		Padding(0, 12).Gap(10).AlignItems(ui.Center).
		Background(tc.Chrome).
		BorderWidth(1, 0, 0, 0).BorderColor(tc.ToolbarBorder).
		Children(func() {
			// Left cluster: Live quota meters (click to open full Quota modal)
			a.renderBottomQuotaStrip(c, tc)

			ui.Box(c).Grow(1)

			// Right cluster: "Needs you" attention pill, running workers, git branch, host
			attnCount, attnSess := a.attentionSummary()
			if attnCount > 0 && attnSess != nil {
				label := fmt.Sprintf("%d needs you", attnCount)
				pill := ui.Row(c.Key("bot-needs-you")).Height(18).Radius(999).Padding(0, 7).Gap(5).
					AlignItems(ui.Center).Cursor(ui.CursorPointer).
					Background(tc.Warning.Alpha(0.15)).BorderWidth(1).BorderColor(tc.Warning.Alpha(0.35))
				pill.Children(func() {
					ui.Box(c).Size(6, 6).Radius(3).Background(tc.Warning)
					ui.Text(c, label).FontSize(10.5).FontWeight(600).TextColor(tc.Warning).SingleLine()
				})
				if pill.Hovered() {
					pill.Background(tc.Warning.Alpha(0.24))
				}
				if pill.Clicked() {
					a.selectAgentSession(attnSess)
				}
			}

			// Active subagents / workers count
			if a.Agent != nil {
				if subs := a.activeSubagents(a.Agent.SnapshotActiveSessionID()); len(subs) > 0 {
					wBtn := ui.Row(c.Key("bot-workers")).Height(18).Radius(5).Padding(0, 6).Gap(4).
						AlignItems(ui.Center).Cursor(ui.CursorPointer)
					wBtn.Children(func() {
						ui.Box(c).Size(5, 5).Radius(2.5).Background(tc.Accent)
						ui.Text(c, fmt.Sprintf("%d working", len(subs))).FontSize(10.5).FontWeight(500).TextColor(tc.TextMuted).SingleLine()
					})
					if wBtn.Hovered() {
						wBtn.Background(tc.Text.Alpha(0.06))
					}
					if wBtn.Clicked() {
						a.RightDrawerTab = RightTabSubagent
						a.ShowRightDrawer = true
					}
				}
			}

			// Git branch & dirty count (click opens Git Client surface)
			gitLabel := "Git"
			dirty := 0
			if a.Git != nil && a.Workspace != nil && a.Workspace.RootDir != "" {
				snap := a.Git.Snapshot()
				if snap.Branch != "" {
					gitLabel = snap.Branch
				}
				dirty, _, _ = a.gitDirtySummary()
			}
			gBtn := ui.Row(c.Key("bot-git")).Height(18).Radius(5).Padding(0, 6).Gap(4).
				AlignItems(ui.Center).Cursor(ui.CursorPointer)
			gBtn.Tooltip("Open Git Client")
			gBtn.Children(func() {
				ui.Icon(c, icons.TablerGitBranch).Width(11).Height(11).TextColor(tc.IconMuted)
				ui.Text(c, gitLabel).FontSize(10.5).FontWeight(500).TextColor(tc.TextMuted).MaxWidth(140).SingleLine()
				if dirty > 0 {
					ui.Text(c, fmt.Sprintf("• %d", dirty)).FontSize(10).FontWeight(600).TextColor(tc.Warning)
				}
			})
			if gBtn.Hovered() {
				gBtn.Background(tc.Text.Alpha(0.06))
			}
			if gBtn.Clicked() {
				a.IsGitClientOpen = false
				a.ActiveTaskKind = "agent"
				a.openGitClientSurface()
			}

			// Execution host badge (Local vs SSH remote)
			hostLabel := "Local"
			hostIcon := icons.TablerCpu
			if a.Workspace != nil && strings.HasPrefix(a.Workspace.RootDir, "ssh://") {
				rest := strings.TrimPrefix(a.Workspace.RootDir, "ssh://")
				if slash := strings.IndexByte(rest, '/'); slash > 0 {
					hostLabel = rest[:slash]
				} else if rest != "" {
					hostLabel = rest
				}
				hostIcon = icons.TablerServer
			}
			hBtn := ui.Row(c.Key("bot-host")).Height(18).Radius(5).Padding(0, 6).Gap(4).
				AlignItems(ui.Center).Cursor(ui.CursorPointer)
			hBtn.Tooltip("Switch workspace or SSH host")
			hBtn.Children(func() {
				ui.Icon(c, hostIcon).Width(11).Height(11).TextColor(tc.IconMuted)
				ui.Text(c, hostLabel).FontSize(10.5).FontWeight(500).TextColor(tc.TextMuted).MaxWidth(120).SingleLine()
			})
			if hBtn.Hovered() {
				hBtn.Background(tc.Text.Alpha(0.06))
			}
			if hBtn.Clicked() {
				a.OpenProjectSwitcherModal()
			}
		})
}

func (a *AppUI) renderBottomQuotaStrip(c *ui.Context, tc theme.ThemeColors) {
	quotaData := a.QuotaData
	quotaKnown := false
	if quotaData != nil {
		if v, ok := quotaData["quotaKnown"].(bool); ok {
			quotaKnown = v
		} else if _, hasFive := quotaData["fiveHour"]; hasFive {
			quotaKnown = true
		} else if _, hasWeekly := quotaData["weekly"]; hasWeekly {
			quotaKnown = true
		}
	}

	pctColor := func(p float64) ui.Color {
		if p > 30 {
			return tc.TextMuted
		}
		if p > 15 {
			return tc.Warning
		}
		return tc.Error
	}

	renderMeter := func(key, label string, m map[string]interface{}) bool {
		if m == nil {
			return false
		}
		pct, ok := m["remainingPercent"].(float64)
		if !ok || pct < 0 {
			return false
		}
		if pct > 100 {
			pct = 100
		}
		resetsIn, _ := m["resetsIn"].(string)
		color := pctColor(pct)

		item := ui.Row(c.Key("qmeter-"+key)).Height(18).Radius(4).Padding(0, 5).Gap(5).
			AlignItems(ui.Center).Cursor(ui.CursorPointer)
		item.Tooltip("Click for detailed Usage & Quota")
		item.Children(func() {
			ui.Text(c, label).FontSize(10).FontWeight(600).TextColor(tc.TextMuted)
			ui.Box(c).Width(34).Height(4).Radius(2).Background(tc.Border).Clip().Children(func() {
				fill := tc.Accent
				if pct <= 15 {
					fill = tc.Error
				} else if pct <= 30 {
					fill = tc.Warning
				}
				ui.Box(c).WidthPercent(float32(pct)).Height(4).Radius(2).Background(fill)
			})
			val := fmt.Sprintf("%.0f%%", pct)
			if resetsIn != "" && !strings.HasPrefix(resetsIn, "Quota") {
				val += " " + resetsIn
			}
			ui.Text(c, val).FontSize(10).FontWeight(500).TextColor(color).SingleLine()
		})
		if item.Hovered() {
			item.Background(tc.Text.Alpha(0.06))
		}
		if item.Clicked() {
			a.openUsageModal()
		}
		return true
	}

	renderedAny := false
	if quotaKnown && quotaData != nil {
		if five, ok := quotaData["fiveHour"].(map[string]interface{}); ok {
			if renderMeter("5h", "5h", five) {
				renderedAny = true
			}
		}
		if weekly, ok := quotaData["weekly"].(map[string]interface{}); ok {
			if renderMeter("7d", "7d", weekly) {
				renderedAny = true
			}
		}
	}

	if !renderedAny {
		qBtn := ui.Row(c.Key("bot-quota-open")).Height(18).Radius(4).Padding(0, 5).Gap(4).
			AlignItems(ui.Center).Cursor(ui.CursorPointer)
		qBtn.Tooltip("Open Usage & Quota")
		qBtn.Children(func() {
			ui.Icon(c, icons.TablerChartBar).Width(11).Height(11).TextColor(tc.IconMuted)
			ui.Text(c, "Usage & Quota").FontSize(10.5).FontWeight(500).TextColor(tc.TextMuted)
		})
		if qBtn.Hovered() {
			qBtn.Background(tc.Text.Alpha(0.06))
		}
		if qBtn.Clicked() {
			a.openUsageModal()
		}
	} else {
		refBtn := ui.Row(c.Key("bot-quota-refresh")).Size(18, 18).Radius(4).
			AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
		refBtn.Tooltip("Refresh quota")
		refBtn.Children(func() {
			ui.Icon(c, icons.TablerRefresh).Width(10).Height(10).TextColor(tc.IconMuted)
		})
		if refBtn.Hovered() {
			refBtn.Background(tc.Text.Alpha(0.06))
		}
		if refBtn.Clicked() {
			a.refreshQuotaAsync()
		}
	}
}
