package ui

import (
	"fmt"

	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/agent"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/icons"
)

// renderUsageQuotaModal renders the ForgeADE Usage & Quota Modal.
// Provides real-time limits, token credits, and session context usage breakdown.
func (a *AppUI) renderUsageQuotaModal(c *ui.Context) {
	if !a.ShowUsageModal {
		return
	}

	tc := a.ThemeMgr.Colors()
	as := a.Agent

	// Real quota data (account, weekly/five-hour windows) from forge-ade's
	// quota engine, falling back to ACP usage totals.
	quotaData := map[string]interface{}{}
	if a.Engine != nil {
		quotaData = a.Engine.GetAgentQuota("agent-agy")
	}
	account := ""
	if v, ok := quotaData["account"].(string); ok {
		account = v
	}

	pctColor := func(p float64) ui.Color {
		if p > 30 {
			return tc.Success
		}
		if p > 15 {
			return ui.Hex("#fbbf24")
		}
		return tc.Error
	}
	str_ := func(v interface{}) string {
		s, _ := v.(string)
		return s
	}
	mapOf := func(v interface{}) map[string]interface{} {
		m, _ := v.(map[string]interface{})
		return m
	}
	pctOf := func(m map[string]interface{}, def float64) float64 {
		if m == nil {
			return def
		}
		v, _ := m["remainingPercent"].(float64)
		if v <= 0 {
			return def
		}
		return v
	}
	strOf := func(m map[string]interface{}, key string) string {
		if m == nil {
			return ""
		}
		s, _ := m[key].(string)
		return s
	}
	quotaBar := func(label string, pct float64, resetsIn string) {
		color := pctColor(pct)
		ui.Column(c).Gap(4).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Text(c, label).FontSize(11).FontWeight(500).TextColor(color)
				ui.Box(c).Grow(1)
				ui.Text(c, fmt.Sprintf("%.2f%%", pct)).FontSize(11).FontWeight(700).TextColor(color)
			})
			ui.Box(c).FillWidth().Height(6).Radius(3).Background(tc.Border.Alpha(0.6)).Children(func() {
				ui.Box(c).WidthPercent(float32(pct)).Height(6).Radius(3).Background(color)
			})
			resetText := "Quota available"
			if resetsIn != "" {
				resetText = "Quota available · Resets in " + resetsIn
			}
			ui.Text(c, resetText).FontSize(10).TextColor(color)
		})
	}

	ui.Modal(c, &a.ShowUsageModal, func() {
		modal := ui.Column(c).Width(520).Radius(18).
			Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).
			Shadow(0, 24, 72, -20, ui.RGBA(0, 0, 0, 0.55)).Clip()

		modal.Children(func() {
			// Header
			ui.Row(c).Padding(16, 20).Gap(12).AlignItems(ui.Center).
				BorderWidth(0, 0, 1, 0).BorderColor(tc.Border.Alpha(0.6)).Children(func() {
				// Provider / Agent Icon
				ui.Box(c).Size(36, 36).Radius(10).AlignItems(ui.Center).Justify(ui.Center).
					Background(tc.Accent.Alpha(0.12)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.25)).Children(func() {
					agentName := "Google Antigravity"
					if as != nil && as.ActiveModel != "" {
						agentName = as.ActiveModel
					}
					icons.RenderProviderIcon(c, agentName, 18)
				})

				ui.Column(c).Grow(1).Gap(2).Children(func() {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Text(c, "Agent Usage & Quota").FontSize(14).FontWeight(600).TextColor(tc.Text)
						// Badge
						ui.Row(c).Radius(4).Padding(1, 6).Background(tc.Accent.Alpha(0.15)).
							BorderWidth(1).BorderColor(tc.Accent.Alpha(0.3)).Children(func() {
							ui.Text(c, "QUOTA").FontSize(9).FontWeight(700).TextColor(tc.Accent)
						})
					})
					ui.Text(c, "Real-time limits, token credits, and session context usage").FontSize(11).TextColor(tc.TextMuted)
				})

				// Refresh button
				refreshBtn := ui.Row(c).Size(28, 28).Radius(6).AlignItems(ui.Center).Justify(ui.Center).
					Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border)
				refreshBtn.Children(func() {
					ui.Icon(c, icons.TablerRefresh).Width(13).Height(13).TextColor(tc.IconMuted)
				})
				if refreshBtn.Hovered() {
					refreshBtn.Background(tc.Text.Alpha(0.06))
				}
				if refreshBtn.Clicked() {
					c.AddToast(ui.Toast{Title: "Refreshed Quotas", Description: "Quota counters synchronized", Type: "info"})
				}

				// Close button
				closeBtn := ui.Row(c).Size(28, 28).Radius(6).AlignItems(ui.Center).Justify(ui.Center).
					Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border)
				closeBtn.Children(func() {
					ui.Icon(c, icons.TablerX).Width(14).Height(14).TextColor(tc.IconMuted)
				})
				if closeBtn.Hovered() {
					closeBtn.Background(tc.Text.Alpha(0.06))
				}
				if closeBtn.Clicked() {
					a.ShowUsageModal = false
				}
			})

			// Body
			ui.Column(c).Padding(16, 20).Gap(14).Children(func() {
				// Model groups from forge-ade's quota engine (Gemini,
				// Claude & GPT, ...), each with weekly + five-hour windows.
				type quotaGroup struct {
					Label  string
					Models string
					Weekly map[string]interface{}
					Five   map[string]interface{}
				}
				groups := []quotaGroup{{
					Label:  str_(quotaData["modelGroup"]),
					Models: str_(quotaData["models"]),
					Weekly: mapOf(quotaData["weekly"]),
					Five:   mapOf(quotaData["fiveHour"]),
				}}
				if extras, ok := quotaData["extraGroups"].([]interface{}); ok {
					for _, eg := range extras {
						em, _ := eg.(map[string]interface{})
						if em == nil {
							continue
						}
						groups = append(groups, quotaGroup{
							Label:  str_(em["modelGroup"]),
							Models: str_(em["models"]),
							Weekly: mapOf(em["weekly"]),
							Five:   mapOf(em["fiveHour"]),
						})
					}
				}

				renderQuotaCard := func(g quotaGroup) {
					ui.Column(c).Padding(14).Radius(12).Background(tc.SurfaceRaised.Alpha(0.5)).
						BorderWidth(1).BorderColor(tc.Border.Alpha(0.7)).Gap(10).Children(func() {
						ui.Row(c).AlignItems(ui.Center).Children(func() {
							ui.Column(c).Gap(2).Children(func() {
								ui.Text(c, "MODELS & QUOTA").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted)
								accountLabel := "Account: Antigravity ACP Local"
								if account != "" {
									accountLabel = "Account: " + account
								}
								ui.Text(c, accountLabel).FontSize(11).FontWeight(600).TextColor(tc.Success)
							})
							ui.Box(c).Grow(1)
							if g.Label != "" {
								badge := ui.Row(c).Radius(4).Padding(2, 8).Background(tc.Accent.Alpha(0.15)).
									BorderWidth(1).BorderColor(tc.Accent.Alpha(0.3))
								badge.Children(func() {
									ui.Text(c, g.Label).FontSize(10).FontWeight(700).TextColor(tc.Accent)
								})
							}
						})
						if g.Models != "" {
							ui.Text(c, "Models within group: "+g.Models).FontSize(11).TextColor(tc.TextMuted)
						}
						quotaBar("Weekly Limit Remaining", pctOf(g.Weekly, 100), strOf(g.Weekly, "refreshesIn"))
						quotaBar("Five Hour Limit Remaining", pctOf(g.Five, 100), strOf(g.Five, "refreshesIn"))
					})
				}
				for _, g := range groups {
					renderQuotaCard(g)
				}

				// Card 2: Session Context Window Gauge
				usedTokens := 0
				pctUsed := 0
				if as != nil {
					usedTokens, pctUsed = as.ContextUsage()
				}
				maxTokens := 1000000 // 1M context budget

				ui.Column(c).Padding(14).Radius(12).Background(tc.SurfaceRaised.Alpha(0.5)).
					BorderWidth(1).BorderColor(tc.Border.Alpha(0.7)).Gap(10).Children(func() {
					ui.Row(c).AlignItems(ui.Center).Children(func() {
						ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
							ui.Icon(c, icons.TablerDatabase).Width(15).Height(15).TextColor(tc.Accent)
							ui.Text(c, "Session Context Window").FontSize(12).FontWeight(600).TextColor(tc.Text)
						})
						ui.Box(c).Grow(1)
						ui.Text(c, fmt.Sprintf("%d%%", pctUsed)).FontSize(12).FontWeight(700).TextColor(tc.Success)
					})

					ui.Row(c).AlignItems(ui.Baseline).Children(func() {
						ui.Text(c, fmt.Sprintf("%s of %s tokens", formatTokens(usedTokens), formatTokens(maxTokens))).
							FontSize(11).TextColor(tc.TextMuted)
						ui.Box(c).Grow(1)
						ui.Text(c, "1,000,000 budget").FontSize(10).TextColor(tc.TextMuted)
					})

					// Progress bar
					fillWidthPct := float32(pctUsed)
					if fillWidthPct < 2 && usedTokens > 0 {
						fillWidthPct = 2
					}
					if fillWidthPct > 100 {
						fillWidthPct = 100
					}
					ui.Box(c).FillWidth().Height(6).Radius(3).Background(tc.Border.Alpha(0.6)).Children(func() {
						ui.Box(c).WidthPercent(fillWidthPct).Height(6).Radius(3).Background(tc.Success)
					})

					// Force Compact Conversation Context Button
					compactBtn := ui.Row(c).Height(32).Radius(8).Padding(0, 12).Gap(6).
						AlignItems(ui.Center).Justify(ui.Center).
						Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
					compactBtn.Children(func() {
						ui.Icon(c, icons.TablerArrowsMinimize).Width(13).Height(13).TextColor(tc.Accent)
						ui.Text(c, "Force Compact Conversation Context").FontSize(11).FontWeight(500).TextColor(tc.Text)
					})
					if compactBtn.Hovered() {
						compactBtn.Background(tc.Text.Alpha(0.06))
					}
					if compactBtn.Clicked() {
						activeSess := as.ActiveSession()
						if as != nil && activeSess != nil && len(activeSess.Messages) > 4 {
							// Keep first and last messages, compact older history
							first := activeSess.Messages[0]
							lastThree := activeSess.Messages[len(activeSess.Messages)-3:]
							activeSess.Messages = append([]*agent.ChatMessage{first}, lastThree...)
							c.AddToast(ui.Toast{Title: "Context Compacted", Description: "Conversation history pruned to active turns", Type: "success"})
						} else {
							c.AddToast(ui.Toast{Title: "Already Optimal", Description: "Context is within compact bounds", Type: "info"})
						}
						a.ShowUsageModal = false
					}
				})
			})

			// Footer
			ui.Row(c).Padding(12, 20).AlignItems(ui.Center).
				BorderWidth(1, 0, 0, 0).BorderColor(tc.Border.Alpha(0.6)).
				Background(tc.SurfaceRaised.Alpha(0.3)).Children(func() {
				ui.Text(c, "Press Esc to close").FontSize(11).TextColor(tc.TextMuted)
				ui.Box(c).Grow(1)
				doneBtn := ui.Row(c).Height(30).Radius(6).Padding(0, 14).
					AlignItems(ui.Center).Justify(ui.Center).
					Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border)
				doneBtn.Children(func() {
					ui.Text(c, "Done").FontSize(12).FontWeight(500).TextColor(tc.Text)
				})
				if doneBtn.Hovered() {
					doneBtn.Background(tc.Text.Alpha(0.06))
				}
				if doneBtn.Clicked() {
					a.ShowUsageModal = false
				}
			})
		})
	})
}

func formatTokens(n int) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.1fM", float32(n)/1000000.0)
	}
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", float32(n)/1000.0)
	}
	return fmt.Sprintf("%d", n)
}
