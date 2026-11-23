package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/settings"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
	"github.com/hasdev/forge-ade/app/native/pkg/usage"
)

// renderSettingsTabUsage renders the comprehensive Usage Stats dashboard in Settings.
func (a *AppUI) renderSettingsTabUsage(c *ui.Context, tc theme.ThemeColors, sm *settings.SettingsManager) {
	if a.UsageMgr == nil {
		homeDir := a.Workspace.RootDir
		a.UsageMgr = usage.NewUsageManager(homeDir, nil)
	}

	// Initialize default filters if empty
	if sm.UsageTimeRange == "" {
		sm.UsageTimeRange = usage.TimeRange7D
	}
	if sm.UsageGroupBy == "" {
		sm.UsageGroupBy = "model"
	}
	if sm.UsageCustomStart == "" {
		sm.UsageCustomStart = time.Now().Add(-7 * 24 * time.Hour).Format("2006-01-02")
	}
	if sm.UsageCustomEnd == "" {
		sm.UsageCustomEnd = time.Now().Format("2006-01-02")
	}

	// Parse custom dates if manual date range selected
	var customStart, customEnd time.Time
	if sm.UsageTimeRange == usage.TimeRangeCustom {
		if t, err := time.Parse("2006-01-02", strings.TrimSpace(sm.UsageCustomStart)); err == nil {
			customStart = t
		}
		if t, err := time.Parse("2006-01-02", strings.TrimSpace(sm.UsageCustomEnd)); err == nil {
			// Include full end day
			customEnd = t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
		}
	}

	// Query aggregated usage
	qOpts := usage.QueryOptions{
		TimeRange:      sm.UsageTimeRange,
		CustomStart:    customStart,
		CustomEnd:      customEnd,
		FilterProvider: sm.UsageFilterProv,
		FilterModel:    sm.UsageFilterModel,
	}
	summary := a.UsageMgr.Query(qOpts)

	// 1. Top Section Header with Title, Description, and Refresh Action
	ui.Row(c).FillWidth().AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
		settingsSectionHeader(c, tc, "Usage & Token Stats",
			"Monitor token consumption, prompt inputs, completions, cache reads, and cache hit efficiency across models and providers.")

		refreshBtn := ui.Row(c).Height(30).Radius(6).Padding(0, 10).Gap(6).AlignItems(ui.Center).
			Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
		refreshBtn.Children(func() {
			ui.Icon(c, icons.TablerRefresh).Width(13).Height(13).TextColor(tc.IconMuted)
			ui.Text(c, "Refresh Stats").FontSize(12).FontWeight(500).TextColor(tc.Text)
		})
		if refreshBtn.Hovered() {
			refreshBtn.Background(tc.SurfaceRaised)
		}
		if refreshBtn.Clicked() {
			go a.UsageMgr.BackfillHistoricalSessions()
			c.AddToast(ui.Toast{Title: "Usage Stats Refreshed", Description: "Synchronized historical session tokens.", Type: "info"})
		}
	})

	// 2. Filter Controls Card (Time Range Presets, Custom Date Range, Model & Provider Selectors)
	settingsCard(c, tc, func() {
		ui.Column(c).Gap(14).Children(func() {
			// Row 1: Time Range Presets
			ui.Row(c).AlignItems(ui.Center).Gap(12).Wrap().Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icons.TablerClock).Width(14).Height(14).TextColor(tc.IconMuted)
					ui.Text(c, "Time Range:").FontSize(12).FontWeight(600).TextColor(tc.Text)
				})

				presets := []struct {
					ID    string
					Label string
				}{
					{usage.TimeRange1H, "Last 1h"},
					{usage.TimeRange1D, "Last 24h"},
					{usage.TimeRange7D, "Last 7d"},
					{usage.TimeRange1Month, "Last 1 Month"},
					{usage.TimeRangeCustom, "Custom Date Range"},
				}

				for _, p := range presets {
					thisP := p
					isSelected := (sm.UsageTimeRange == thisP.ID)
					btn := ui.Row(c.Key("range-btn-"+thisP.ID)).Height(28).Radius(6).Padding(0, 12).
						AlignItems(ui.Center).Justify(ui.Center)
					if isSelected {
						btn.Background(tc.Accent).BorderWidth(1).BorderColor(tc.Accent)
					} else {
						btn.Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
					}
					btn.Children(func() {
						txt := ui.Text(c, thisP.Label).FontSize(12).FontWeight(500)
						if isSelected {
							txt.TextColor(tc.AccentForeground).FontWeight(600)
						} else {
							txt.TextColor(tc.Text)
						}
					})
					if !isSelected && btn.Hovered() {
						btn.Background(tc.SurfaceRaised)
					}
					if btn.Clicked() {
						sm.UsageTimeRange = thisP.ID
					}
				}
			})

			// Row 2: Custom Date Range Inputs (Only shown when "custom" is selected)
			if sm.UsageTimeRange == usage.TimeRangeCustom {
				ui.Row(c).Padding(8, 12).Radius(6).Background(tc.Canvas).BorderWidth(1).BorderColor(tc.Border).
					Gap(16).AlignItems(ui.Center).Wrap().Children(func() {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Text(c, "From:").FontSize(12).FontWeight(600).TextColor(tc.TextMuted)
						ui.Row(c).Width(130).Height(28).Radius(4).Padding(0, 8).Background(tc.Surface).
							BorderWidth(1).BorderColor(tc.Border).AlignItems(ui.Center).Children(func() {
							ui.TextInput(c, &sm.UsageCustomStart).Placeholder("YYYY-MM-DD").Grow(1)
						})
					})

					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Text(c, "To:").FontSize(12).FontWeight(600).TextColor(tc.TextMuted)
						ui.Row(c).Width(130).Height(28).Radius(4).Padding(0, 8).Background(tc.Surface).
							BorderWidth(1).BorderColor(tc.Border).AlignItems(ui.Center).Children(func() {
							ui.TextInput(c, &sm.UsageCustomEnd).Placeholder("YYYY-MM-DD").Grow(1)
						})
					})

					ui.Text(c, "Format: YYYY-MM-DD (e.g. 2026-11-01)").FontSize(11).TextColor(tc.TextMuted)
				})
			}

			// Row 3: Provider & Model Filters + Grouping Mode
			ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Wrap().Gap(12).Children(func() {
				// Provider & Model dropdown filter controls
				ui.Row(c).Gap(16).AlignItems(ui.Center).Wrap().Children(func() {
					// Provider filter
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Text(c, "Provider:").FontSize(12).FontWeight(600).TextColor(tc.TextMuted)
						provOptions := append([]string{"All Providers"}, summary.AvailableProviders...)
						activeProv := sm.UsageFilterProv
						if activeProv == "" {
							activeProv = "All Providers"
						}
						ui.Select(c, &activeProv, provOptions)
						if activeProv == "All Providers" {
							sm.UsageFilterProv = ""
						} else {
							sm.UsageFilterProv = activeProv
						}
					})

					// Model filter
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Text(c, "Model:").FontSize(12).FontWeight(600).TextColor(tc.TextMuted)
						modelOptions := append([]string{"All Models"}, summary.AvailableModels...)
						activeModel := sm.UsageFilterModel
						if activeModel == "" {
							activeModel = "All Models"
						}
						ui.Select(c, &activeModel, modelOptions)
						if activeModel == "All Models" {
							sm.UsageFilterModel = ""
						} else {
							sm.UsageFilterModel = activeModel
						}
					})

					// Reset filters button if any filter is active
					if sm.UsageFilterProv != "" || sm.UsageFilterModel != "" {
						clearBtn := ui.Row(c).Height(26).Radius(4).Padding(0, 8).Gap(4).AlignItems(ui.Center).
							Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
						clearBtn.Children(func() {
							ui.Icon(c, icons.TablerX).Width(12).Height(12).TextColor(tc.IconMuted)
							ui.Text(c, "Clear Filters").FontSize(11).TextColor(tc.TextMuted)
						})
						if clearBtn.Clicked() {
							sm.UsageFilterProv = ""
							sm.UsageFilterModel = ""
						}
					}
				})

				// Grouping Mode Switcher
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "Group By:").FontSize(12).FontWeight(600).TextColor(tc.TextMuted)

					modelGrpBtn := ui.Row(c).Height(26).Radius(4).Padding(0, 10).AlignItems(ui.Center)
					if sm.UsageGroupBy == "model" {
						modelGrpBtn.Background(tc.Accent)
						ui.Text(c, "Model").FontSize(11).FontWeight(600).TextColor(tc.AccentForeground)
					} else {
						modelGrpBtn.Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
						ui.Text(c, "Model").FontSize(11).FontWeight(500).TextColor(tc.Text)
					}
					if modelGrpBtn.Clicked() {
						sm.UsageGroupBy = "model"
					}

					provGrpBtn := ui.Row(c).Height(26).Radius(4).Padding(0, 10).AlignItems(ui.Center)
					if sm.UsageGroupBy == "provider" {
						provGrpBtn.Background(tc.Accent)
						ui.Text(c, "Provider").FontSize(11).FontWeight(600).TextColor(tc.AccentForeground)
					} else {
						provGrpBtn.Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
						ui.Text(c, "Provider").FontSize(11).FontWeight(500).TextColor(tc.Text)
					}
					if provGrpBtn.Clicked() {
						sm.UsageGroupBy = "provider"
					}
				})
			})
		})
	})

	// 3. KPI Metric Cards Row (Input, Output, Cache, Cache Hit Rate %, Total Tokens)
	ui.Row(c).Gap(12).Children(func() {
		// Card 1: Input Tokens
		renderUsageKpiCard(c, tc, "Input Tokens", formatTokensCompact(summary.TotalInputTokens),
			formatExactTokens(summary.TotalInputTokens)+" prompt tokens", icons.TablerArrowDownLeft, tc.Text)

		// Card 2: Output Tokens
		renderUsageKpiCard(c, tc, "Output Tokens", formatTokensCompact(summary.TotalOutputTokens),
			formatExactTokens(summary.TotalOutputTokens)+" generated tokens", icons.TablerSparkles, tc.Accent)

		// Card 3: Cache Tokens
		renderUsageKpiCard(c, tc, "Cached Tokens", formatTokensCompact(summary.TotalCachedTokens),
			formatExactTokens(summary.TotalCachedTokens)+" cache read tokens", icons.TablerDatabase, tc.Text)

		// Card 4: Percentage Cache Hit
		hitRateStr := fmt.Sprintf("%.1f%%", summary.CacheHitRate)
		hitColor := tc.Text
		if summary.CacheHitRate >= 50.0 {
			hitColor = tc.Success
		} else if summary.CacheHitRate > 0.0 {
			hitColor = tc.Accent
		}
		renderUsageKpiCard(c, tc, "Cache Hit Rate", hitRateStr,
			"Context cache efficiency", icons.TablerActivity, hitColor)

		// Card 5: Total Tokens
		totalSub := fmt.Sprintf("%d turns · %d sessions", summary.TotalTurns, summary.TotalSessions)
		renderUsageKpiCard(c, tc, "Total Usage", formatTokensCompact(summary.TotalTokens),
			totalSub, icons.TablerChartBar, tc.Text)
	})

	// 4. Grouping Table (Group by Model OR Group by Provider)
	if sm.UsageGroupBy == "provider" {
		renderUsageByProviderTable(c, tc, summary)
	} else {
		renderUsageByModelTable(c, tc, summary)
	}
}

// renderUsageKpiCard renders a single metric summary card.
func renderUsageKpiCard(c *ui.Context, tc theme.ThemeColors, title, value, subtitle string, icon *ui.SVG, valColor ui.Color) {
	ui.Column(c).Grow(1).Padding(12, 14).Radius(8).Background(tc.Surface).
		BorderWidth(1).BorderColor(tc.Border).Gap(4).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
			ui.Text(c, strings.ToUpper(title)).FontSize(10).FontWeight(700).LetterSpacing(0.5).
				TextColor(tc.TextMuted)
			if icon != nil {
				ui.Icon(c, icon).Width(14).Height(14).TextColor(tc.IconMuted)
			}
		})
		ui.Text(c, value).FontSize(20).FontWeight(700).TextColor(valColor)
		ui.Text(c, subtitle).FontSize(11).TextColor(tc.TextMuted)
	})
}

// renderUsageByModelTable renders the table breakdown grouped per Model.
func renderUsageByModelTable(c *ui.Context, tc theme.ThemeColors, summary usage.UsageSummary) {
	settingsCard(c, tc, func() {
		ui.Column(c).Gap(10).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icons.TablerCpu).Width(15).Height(15).TextColor(tc.Accent)
					ui.Text(c, "Token Breakdown by Model").FontSize(13).FontWeight(600).TextColor(tc.Text)
				})
				ui.Text(c, fmt.Sprintf("%d models active in window", len(summary.ByModel))).FontSize(11).TextColor(tc.TextMuted)
			})

			if len(summary.ByModel) == 0 {
				ui.Row(c).Padding(24, 0).Justify(ui.Center).Children(func() {
					ui.Text(c, "No token usage recorded for the selected time range and filters.").FontSize(12).TextColor(tc.TextMuted)
				})
				return
			}

			// Table Header
			ui.Row(c).Height(28).Padding(0, 10).AlignItems(ui.Center).Background(tc.Canvas).
				Radius(4).Children(func() {
				ui.Text(c, "MODEL").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted).Grow(2)
				ui.Text(c, "PROVIDER").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted).Width(150)
				ui.Text(c, "INPUT").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted).Width(100)
				ui.Text(c, "OUTPUT").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted).Width(100)
				ui.Text(c, "CACHE READ").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted).Width(100)
				ui.Text(c, "CACHE HIT %").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted).Width(100)
				ui.Text(c, "TOTAL").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted).Width(100)
				ui.Text(c, "SHARE %").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted).Width(80)
			})

			// Table Rows
			for idx, mu := range summary.ByModel {
				thisMu := mu
				bg := tc.Surface
				if idx%2 == 1 {
					bg = tc.SurfaceRaised.Alpha(0.4)
				}

				row := ui.Row(c.Key("model-row-"+thisMu.Model)).Height(34).Padding(0, 10).
					AlignItems(ui.Center).Background(bg).Radius(4)
				row.Children(func() {
					// Model Name
					ui.Row(c).Grow(2).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, icons.TablerBot).Width(13).Height(13).TextColor(tc.Accent)
						ui.Text(c, thisMu.Model).FontSize(12).FontWeight(600).TextColor(tc.Text)
					})

					// Provider Badge
					ui.Row(c).Width(150).Children(func() {
						ui.Row(c).Padding(2, 6).Radius(4).Background(tc.Border).Children(func() {
							ui.Text(c, thisMu.Provider).FontSize(11).TextColor(tc.Text)
						})
					})

					// Input Tokens
					ui.Row(c).Width(100).Children(func() {
						ui.Text(c, formatExactTokens(thisMu.InputTokens)).FontSize(12).TextColor(tc.Text)
					})

					// Output Tokens
					ui.Row(c).Width(100).Children(func() {
						ui.Text(c, formatExactTokens(thisMu.OutputTokens)).FontSize(12).TextColor(tc.Accent)
					})

					// Cache Read Tokens
					ui.Row(c).Width(100).Children(func() {
						ui.Text(c, formatExactTokens(thisMu.CachedTokens)).FontSize(12).TextColor(tc.TextMuted)
					})

					// Cache Hit %
					ui.Row(c).Width(100).Children(func() {
						rateStr := fmt.Sprintf("%.1f%%", thisMu.CacheHitRate)
						rateColor := tc.TextMuted
						if thisMu.CacheHitRate >= 50.0 {
							rateColor = tc.Success
						} else if thisMu.CacheHitRate > 0.0 {
							rateColor = tc.Accent
						}
						ui.Text(c, rateStr).FontSize(12).FontWeight(600).TextColor(rateColor)
					})

					// Total Tokens
					ui.Row(c).Width(100).Children(func() {
						ui.Text(c, formatExactTokens(thisMu.TotalTokens)).FontSize(12).FontWeight(700).TextColor(tc.Text)
					})

					// Share %
					ui.Row(c).Width(80).Gap(6).AlignItems(ui.Center).Children(func() {
						shareStr := fmt.Sprintf("%.1f%%", thisMu.PercentShare)
						ui.Text(c, shareStr).FontSize(11).TextColor(tc.TextMuted)
					})
				})
			}
		})
	})
}

// renderUsageByProviderTable renders the table breakdown grouped per Provider.
func renderUsageByProviderTable(c *ui.Context, tc theme.ThemeColors, summary usage.UsageSummary) {
	settingsCard(c, tc, func() {
		ui.Column(c).Gap(10).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icons.TablerPackage).Width(15).Height(15).TextColor(tc.Accent)
					ui.Text(c, "Token Breakdown by Provider").FontSize(13).FontWeight(600).TextColor(tc.Text)
				})
				ui.Text(c, fmt.Sprintf("%d providers active in window", len(summary.ByProvider))).FontSize(11).TextColor(tc.TextMuted)
			})

			if len(summary.ByProvider) == 0 {
				ui.Row(c).Padding(24, 0).Justify(ui.Center).Children(func() {
					ui.Text(c, "No token usage recorded for the selected time range and filters.").FontSize(12).TextColor(tc.TextMuted)
				})
				return
			}

			// Table Header
			ui.Row(c).Height(28).Padding(0, 10).AlignItems(ui.Center).Background(tc.Canvas).
				Radius(4).Children(func() {
				ui.Text(c, "PROVIDER").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted).Grow(2)
				ui.Text(c, "MODELS COUNT").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted).Width(130)
				ui.Text(c, "INPUT").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted).Width(100)
				ui.Text(c, "OUTPUT").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted).Width(100)
				ui.Text(c, "CACHE READ").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted).Width(100)
				ui.Text(c, "CACHE HIT %").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted).Width(100)
				ui.Text(c, "TOTAL").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted).Width(100)
				ui.Text(c, "SHARE %").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted).Width(80)
			})

			// Table Rows
			for idx, pu := range summary.ByProvider {
				thisPu := pu
				bg := tc.Surface
				if idx%2 == 1 {
					bg = tc.SurfaceRaised.Alpha(0.4)
				}

				row := ui.Row(c.Key("prov-row-"+thisPu.Provider)).Height(34).Padding(0, 10).
					AlignItems(ui.Center).Background(bg).Radius(4)
				row.Children(func() {
					// Provider Name
					ui.Row(c).Grow(2).Gap(6).AlignItems(ui.Center).Children(func() {
						icons.RenderProviderIcon(c, thisPu.Provider, 15)
						ui.Text(c, thisPu.Provider).FontSize(12).FontWeight(600).TextColor(tc.Text)
					})

					// Models Count & badge
					ui.Row(c).Width(130).Children(func() {
						lbl := fmt.Sprintf("%d model", thisPu.ModelCount)
						if thisPu.ModelCount != 1 {
							lbl += "s"
						}
						ui.Row(c).Padding(2, 6).Radius(4).Background(tc.Border).Children(func() {
							ui.Text(c, lbl).FontSize(11).TextColor(tc.Text)
						})
					})

					// Input Tokens
					ui.Row(c).Width(100).Children(func() {
						ui.Text(c, formatExactTokens(thisPu.InputTokens)).FontSize(12).TextColor(tc.Text)
					})

					// Output Tokens
					ui.Row(c).Width(100).Children(func() {
						ui.Text(c, formatExactTokens(thisPu.OutputTokens)).FontSize(12).TextColor(tc.Accent)
					})

					// Cache Read Tokens
					ui.Row(c).Width(100).Children(func() {
						ui.Text(c, formatExactTokens(thisPu.CachedTokens)).FontSize(12).TextColor(tc.TextMuted)
					})

					// Cache Hit %
					ui.Row(c).Width(100).Children(func() {
						rateStr := fmt.Sprintf("%.1f%%", thisPu.CacheHitRate)
						rateColor := tc.TextMuted
						if thisPu.CacheHitRate >= 50.0 {
							rateColor = tc.Success
						} else if thisPu.CacheHitRate > 0.0 {
							rateColor = tc.Accent
						}
						ui.Text(c, rateStr).FontSize(12).FontWeight(600).TextColor(rateColor)
					})

					// Total Tokens
					ui.Row(c).Width(100).Children(func() {
						ui.Text(c, formatExactTokens(thisPu.TotalTokens)).FontSize(12).FontWeight(700).TextColor(tc.Text)
					})

					// Share %
					ui.Row(c).Width(80).Gap(6).AlignItems(ui.Center).Children(func() {
						shareStr := fmt.Sprintf("%.1f%%", thisPu.PercentShare)
						ui.Text(c, shareStr).FontSize(11).TextColor(tc.TextMuted)
					})
				})
			}
		})
	})
}

// formatExactTokens formats integer numbers with thousands comma separators (e.g. 1,234,567).
func formatExactTokens(n int) string {
	if n < 0 {
		return "-" + formatExactTokens(-n)
	}
	s := strconv.Itoa(n)
	if len(s) <= 3 {
		return s
	}
	var res []byte
	rem := len(s) % 3
	if rem > 0 {
		res = append(res, s[:rem]...)
		if len(s) > rem {
			res = append(res, ',')
		}
	}
	for i := rem; i < len(s); i += 3 {
		res = append(res, s[i:i+3]...)
		if i+3 < len(s) {
			res = append(res, ',')
		}
	}
	return string(res)
}

// formatTokensCompact formats integer numbers with k / M compact units for cards.
func formatTokensCompact(n int) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.2fM", float64(n)/1_000_000.0)
	}
	if n >= 10_000 {
		return fmt.Sprintf("%.1fk", float64(n)/1_000.0)
	}
	return formatTokens(n)
}
