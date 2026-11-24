package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/settings"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
	"github.com/hasdev/forge-ade/core/quota"
)

// renderSettingsTabAntigravityQuota renders the multi-account Antigravity Quota Tracker in Settings.
func (a *AppUI) renderSettingsTabAntigravityQuota(c *ui.Context, tc theme.ThemeColors, sm *settings.SettingsManager) {
	mgr := sm.AntigravityAccountMgr()
	accounts := mgr.Accounts()

	// 1. Top Section Header with Title & Action Buttons
	ui.Row(c).FillWidth().AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
		settingsSectionHeader(c, tc, "Antigravity Quota Tracker",
			"Monitor 5-hour and weekly quotas, subscription tiers, and switch accounts across Google Antigravity profiles.")

		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			// Refresh All button with refresh icon
			refreshAllBtn := ui.Row(c).Height(30).Radius(6).Padding(0, 10).Gap(6).AlignItems(ui.Center).
				Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).Cursor(ui.CursorPointer)
			refreshAllBtn.Children(func() {
				ui.Icon(c, icons.TablerRefresh).Width(13).Height(13).TextColor(tc.IconMuted)
				if sm.IsRefreshingQuota {
					ui.Text(c, "Refreshing...").FontSize(12).FontWeight(500).TextColor(tc.TextMuted)
				} else {
					ui.Text(c, "Refresh All").FontSize(12).FontWeight(500).TextColor(tc.Text)
				}
			})
			if refreshAllBtn.Hovered() {
				refreshAllBtn.Background(tc.SurfaceRaised)
			}
			if refreshAllBtn.Clicked() && !sm.IsRefreshingQuota {
				a.refreshAllAntigravityAccountsAsync()
				c.AddToast(ui.Toast{Title: "Refreshing Quotas", Description: "Synchronizing all Antigravity accounts...", Type: "info"})
			}

			// Import Local Account button
			importBtn := ui.Row(c).Height(30).Radius(6).Padding(0, 10).Gap(6).AlignItems(ui.Center).
				Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).Cursor(ui.CursorPointer)
			importBtn.Children(func() {
				ui.Icon(c, icons.TablerUser).Width(13).Height(13).TextColor(tc.IconMuted)
				ui.Text(c, "Import Local").FontSize(12).FontWeight(500).TextColor(tc.Text)
			})
			if importBtn.Hovered() {
				importBtn.Background(tc.SurfaceRaised)
			}
			if importBtn.Clicked() {
				a.importLocalAntigravityAccountAsync()
			}

			// Add Account button
			addBtn := ui.Row(c).Height(30).Radius(6).Padding(0, 10).Gap(6).AlignItems(ui.Center).
				Background(tc.Accent).BorderWidth(1).BorderColor(tc.Accent).Cursor(ui.CursorPointer)
			addBtn.Children(func() {
				ui.Icon(c, icons.TablerPlus).Width(13).Height(13).TextColor(tc.AccentForeground)
				ui.Text(c, "Add Account").FontSize(12).FontWeight(600).TextColor(tc.AccentForeground)
			})
			if addBtn.Hovered() {
				addBtn.Background(tc.Accent.Alpha(0.85))
			}
			if addBtn.Clicked() {
				sm.IsAddingAccount = !sm.IsAddingAccount
				sm.AccountStatusMessage = ""
			}
		})
	})

	// 2. Expandable Add Account Form
	if sm.IsAddingAccount {
		settingsCard(c, tc, func() {
			ui.Column(c).Gap(14).Children(func() {
				// Header
				ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, icons.TablerPlus).Width(15).Height(15).TextColor(tc.Accent)
						ui.Text(c, "Add Antigravity Account").FontSize(14).FontWeight(600).TextColor(tc.Text)
					})
					closeFormBtn := ui.Row(c).Size(22, 22).Radius(4).AlignItems(ui.Center).Justify(ui.Center)
					closeFormBtn.Children(func() {
						ui.Icon(c, icons.TablerX).Width(12).Height(12).TextColor(tc.IconMuted)
					})
					if closeFormBtn.Clicked() {
						a.cancelAntigravityOAuth()
						sm.IsAddingAccount = false
						sm.AccountStatusMessage = ""
					}
				})

				// Mode Switcher Tabs (OAuth vs Manual)
				ui.Row(c).Gap(8).Children(func() {
					// Tab 1: Automatic OAuth
					isOAuth := (sm.AddAccountMode == settings.AddAccountModeOAuth)
					tabOAuth := ui.Row(c).Height(30).Radius(6).Padding(0, 12).Gap(6).AlignItems(ui.Center)
					if isOAuth {
						tabOAuth.Background(tc.Accent.Alpha(0.15)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.4))
					} else {
						tabOAuth.Background(tc.Canvas).BorderWidth(1).BorderColor(tc.Border)
					}
					tabOAuth.Children(func() {
						icons.RenderProviderIcon(c, "antigravity", 14)
						txt := ui.Text(c, "Automatic (Google OAuth)").FontSize(12).FontWeight(500)
						if isOAuth {
							txt.TextColor(tc.Accent).FontWeight(600)
						} else {
							txt.TextColor(tc.TextMuted)
						}
					})
					if tabOAuth.Clicked() {
						sm.AddAccountMode = settings.AddAccountModeOAuth
						sm.AccountStatusMessage = ""
					}

					// Tab 2: Manual
					isManual := (sm.AddAccountMode == settings.AddAccountModeManual)
					tabManual := ui.Row(c).Height(30).Radius(6).Padding(0, 12).Gap(6).AlignItems(ui.Center)
					if isManual {
						tabManual.Background(tc.Accent.Alpha(0.15)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.4))
					} else {
						tabManual.Background(tc.Canvas).BorderWidth(1).BorderColor(tc.Border)
					}
					tabManual.Children(func() {
						ui.Icon(c, icons.TablerKey).Width(13).Height(13).TextColor(tc.IconMuted)
						txt := ui.Text(c, "Manual (Token / JSON)").FontSize(12).FontWeight(500)
						if isManual {
							txt.TextColor(tc.Accent).FontWeight(600)
						} else {
							txt.TextColor(tc.TextMuted)
						}
					})
					if tabManual.Clicked() {
						sm.AddAccountMode = settings.AddAccountModeManual
						sm.AccountStatusMessage = ""
					}
				})

				// Content for Tab 1: Automatic OAuth
				if sm.AddAccountMode == settings.AddAccountModeOAuth {
					ui.Column(c).Gap(10).Children(func() {
						ui.Text(c, "Authorize directly with Google in your web browser. ForgeADE automatically receives the credentials and tracks your quotas without any manual token copying.").
							FontSize(11).TextColor(tc.TextMuted)

						// Optional Nickname
						ui.Column(c).Gap(4).Children(func() {
							ui.Text(c, "Account Nickname (Optional)").FontSize(11).FontWeight(500).TextColor(tc.Text)
							ui.Row(c).Height(32).Radius(6).Padding(0, 8).Background(tc.Canvas).BorderWidth(1).BorderColor(tc.Border).Children(func() {
								ui.TextInput(c, &sm.NewAccountLabel).Placeholder("e.g. Work, Personal Pro, Backup").Grow(1)
							})
						})

						if !sm.IsOAuthFlowActive {
							ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
								// Sign in with Google Button
								loginBtn := ui.Row(c).Height(34).Radius(8).Padding(0, 16).Gap(8).AlignItems(ui.Center).Justify(ui.Center).
									Background(tc.Accent).Cursor(ui.CursorPointer)
								loginBtn.Children(func() {
									icons.RenderProviderIcon(c, "antigravity", 16)
									ui.Text(c, "Sign in with Google (OAuth)").FontSize(12).FontWeight(600).TextColor(tc.AccentForeground)
								})
								if loginBtn.Hovered() {
									loginBtn.Background(tc.Accent.Alpha(0.85))
								}
								if loginBtn.Clicked() {
									a.startAntigravityOAuthAsync(sm.NewAccountLabel)
								}

								cancelBtn := ui.Button(c, "Cancel")
								if cancelBtn.Clicked() {
									sm.IsAddingAccount = false
									sm.AccountStatusMessage = ""
								}
							})
						} else {
							// Active OAuth waiting state
							ui.Column(c).Padding(14, 16).Radius(8).Background(tc.Canvas).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.4)).Gap(10).Children(func() {
								ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
									ui.Icon(c, icons.TablerRefresh).Width(15).Height(15).TextColor(tc.Accent)
									ui.Text(c, "Waiting for Google authorization in browser...").FontSize(13).FontWeight(600).TextColor(tc.Accent)
								})
								ui.Text(c, "A browser tab has opened for Google OAuth sign-in. Grant permissions to complete setup. Once authorized, the account will be added automatically.").
									FontSize(11).TextColor(tc.TextMuted)

								ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
									if sm.OAuthFlowURL != "" {
										reopenBtn := ui.Row(c).Height(28).Radius(6).Padding(0, 10).Gap(5).AlignItems(ui.Center).
											Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
										reopenBtn.Children(func() {
											ui.Icon(c, icons.TablerExternalLink).Width(12).Height(12).TextColor(tc.IconMuted)
											ui.Text(c, "Reopen Browser").FontSize(11).FontWeight(500).TextColor(tc.Text)
										})
										if reopenBtn.Hovered() {
											reopenBtn.Background(tc.SurfaceRaised)
										}
										if reopenBtn.Clicked() {
											_ = quota.OpenSystemBrowser(sm.OAuthFlowURL)
										}

										copyBtn := ui.Row(c).Height(28).Radius(6).Padding(0, 10).Gap(5).AlignItems(ui.Center).
											Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
										copyBtn.Children(func() {
											ui.Icon(c, icons.TablerFiles).Width(12).Height(12).TextColor(tc.IconMuted)
											ui.Text(c, "Copy Link").FontSize(11).FontWeight(500).TextColor(tc.Text)
										})
										if copyBtn.Hovered() {
											copyBtn.Background(tc.SurfaceRaised)
										}
										if copyBtn.Clicked() {
											mygo.Clipboard.WriteText(sm.OAuthFlowURL)
											c.AddToast(ui.Toast{Title: "Copied Link", Description: "Authorization link copied to clipboard", Type: "info"})
										}
									}

									abortBtn := ui.Button(c, "Cancel")
									if abortBtn.Clicked() {
										a.cancelAntigravityOAuth()
									}
								})
							})
						}
					})
				} else {
					// Content for Tab 2: Manual
					ui.Column(c).Gap(10).Children(func() {
						ui.Text(c, "Paste a Google OAuth refresh token (starts with 1//...) or JSON credentials from ~/.gemini/oauth_creds.json").
							FontSize(11).TextColor(tc.TextMuted)

						// Token or JSON input
						ui.Column(c).Gap(4).Children(func() {
							ui.Text(c, "Refresh Token or Credentials JSON").FontSize(11).FontWeight(500).TextColor(tc.Text)
							ui.Row(c).Height(32).Radius(6).Padding(0, 8).Background(tc.Canvas).BorderWidth(1).BorderColor(tc.Border).Children(func() {
								ui.TextInput(c, &sm.NewAccountInput).Placeholder("Paste refresh token (1//...) or JSON object").Grow(1)
							})
						})

						// Nickname / Label input
						ui.Column(c).Gap(4).Children(func() {
							ui.Text(c, "Account Nickname (Optional)").FontSize(11).FontWeight(500).TextColor(tc.Text)
							ui.Row(c).Height(32).Radius(6).Padding(0, 8).Background(tc.Canvas).BorderWidth(1).BorderColor(tc.Border).Children(func() {
								ui.TextInput(c, &sm.NewAccountLabel).Placeholder("e.g. Work, Personal Pro, Backup").Grow(1)
							})
						})

						// Form action buttons
						ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
							cancelBtn := ui.Button(c, "Cancel")
							if cancelBtn.Clicked() {
								sm.IsAddingAccount = false
								sm.AccountStatusMessage = ""
							}

							saveBtn := ui.Row(c).Height(28).Radius(6).Padding(0, 14).AlignItems(ui.Center).Justify(ui.Center).
								Background(tc.Accent).Cursor(ui.CursorPointer)
							saveBtn.Children(func() {
								ui.Text(c, "Save & Track Quota").FontSize(12).FontWeight(600).TextColor(tc.AccentForeground)
							})
							if saveBtn.Hovered() {
								saveBtn.Background(tc.Accent.Alpha(0.85))
							}
							if saveBtn.Clicked() {
								input := strings.TrimSpace(sm.NewAccountInput)
								if input == "" {
									sm.AccountStatusMessage = "Please enter a refresh token or JSON credential"
								} else {
									label := strings.TrimSpace(sm.NewAccountLabel)
									a.addAntigravityAccountAsync(input, label)
								}
							}
						})
					})
				}

				// Error / Status Message
				if sm.AccountStatusMessage != "" {
					ui.Row(c).Padding(6, 10).Radius(6).Background(tc.Error.Alpha(0.1)).BorderWidth(1).BorderColor(tc.Error.Alpha(0.25)).Children(func() {
						ui.Text(c, sm.AccountStatusMessage).FontSize(11).TextColor(tc.Error)
					})
				}
			})
		})
	}

	// 3. Accounts List or Empty State
	if len(accounts) == 0 {
		settingsCard(c, tc, func() {
			ui.Column(c).Padding(24, 16).AlignItems(ui.Center).Gap(10).Children(func() {
				ui.Box(c).Size(48, 48).Radius(12).AlignItems(ui.Center).Justify(ui.Center).
					Background(tc.Accent.Alpha(0.12)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.25)).Children(func() {
					icons.RenderProviderIcon(c, "antigravity", 24)
				})
				ui.Text(c, "No Antigravity Accounts Tracked Yet").FontSize(14).FontWeight(600).TextColor(tc.Text)
				ui.Text(c, "Sign in with Google using OAuth or import your local Antigravity credentials to monitor quotas.").
					FontSize(12).TextColor(tc.TextMuted)

				ui.Row(c).Padding(8, 0, 0, 0).Gap(10).Children(func() {
					loginNowBtn := ui.Row(c).Height(32).Radius(6).Padding(0, 14).Gap(6).AlignItems(ui.Center).
						Background(tc.Accent).Cursor(ui.CursorPointer)
					loginNowBtn.Children(func() {
						icons.RenderProviderIcon(c, "antigravity", 14)
						ui.Text(c, "Sign in with Google (OAuth)").FontSize(12).FontWeight(600).TextColor(tc.AccentForeground)
					})
					if loginNowBtn.Hovered() {
						loginNowBtn.Background(tc.Accent.Alpha(0.85))
					}
					if loginNowBtn.Clicked() {
						sm.IsAddingAccount = true
						sm.AddAccountMode = settings.AddAccountModeOAuth
						a.startAntigravityOAuthAsync("")
					}

					importNowBtn := ui.Row(c).Height(32).Radius(6).Padding(0, 14).Gap(6).AlignItems(ui.Center).
						Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
					importNowBtn.Children(func() {
						ui.Icon(c, icons.TablerUser).Width(13).Height(13).TextColor(tc.Text)
						ui.Text(c, "Import Local Account").FontSize(12).FontWeight(500).TextColor(tc.Text)
					})
					if importNowBtn.Hovered() {
						importNowBtn.Background(tc.SurfaceRaised)
					}
					if importNowBtn.Clicked() {
						a.importLocalAntigravityAccountAsync()
					}
				})
			})
		})
		return
	}

	// 4. Render Account Cards
	ui.Column(c).Gap(14).Children(func() {
		for _, acc := range accounts {
			thisAcc := acc
			a.renderAccountQuotaCard(c, tc, sm, thisAcc)
		}
	})
}

// renderAccountQuotaCard renders an individual account's credentials, tier, and live quota gauges.
func (a *AppUI) renderAccountQuotaCard(c *ui.Context, tc theme.ThemeColors, sm *settings.SettingsManager, acc *quota.AntigravityAccount) {
	cardBorder := tc.Border
	if acc.IsActive {
		cardBorder = tc.Accent.Alpha(0.6)
	}

	ui.Column(c).Padding(16, 18).Radius(12).Background(tc.Surface.Alpha(0.6)).
		BorderWidth(1).BorderColor(cardBorder).Gap(14).Children(func() {

		// Header Row: Avatar, Email, Tier, Status, and Action Buttons (Refresh Icon, Switch, Delete)
		ui.Row(c).FillWidth().AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
			// Left side: Icon + Identity info
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				// Antigravity Avatar Icon
				ui.Box(c).Size(36, 36).Radius(10).AlignItems(ui.Center).Justify(ui.Center).
					Background(tc.Accent.Alpha(0.12)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.25)).Children(func() {
					icons.RenderProviderIcon(c, "antigravity", 20)
				})

				ui.Column(c).Gap(2).Children(func() {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Text(c, acc.Email).FontSize(13).FontWeight(600).TextColor(tc.Text)

						// Active Badge
						if acc.IsActive {
							Badge(c, tc, "ACTIVE", tc.Accent)
						}

						// Tier Badge (FREE, PRO, ULTRA, ENTERPRISE)
						tier := strings.ToUpper(strings.TrimSpace(acc.Tier))
						switch tier {
						case "ULTRA":
							Badge(c, tc, "ULTRA", tc.Warning)
						case "PRO":
							Badge(c, tc, "PRO", tc.Accent)
						case "ENTERPRISE":
							Badge(c, tc, "ENTERPRISE", tc.Success)
						default:
							Badge(c, tc, "FREE", tc.TextMuted)
						}

						// Argon Capability Badge
						if acc.Quota != nil && (acc.Quota.ArgonWeeklyPercent > 0 || acc.Quota.ArgonWeeklyReset != "") {
							Badge(c, tc, "ARGON", tc.Success)
						}

						// Status Badge if not OK
						if acc.Status == "expired" {
							Badge(c, tc, "EXPIRED", tc.Error)
						} else if acc.Status == "forbidden" {
							Badge(c, tc, "FORBIDDEN", tc.Error)
						} else if sm.RefreshingAccountID == acc.ID {
							Badge(c, tc, "SYNCING...", tc.Warning)
						}
					})

					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						if acc.Label != "" {
							ui.Text(c, acc.Label).FontSize(11).FontWeight(500).TextColor(tc.Text)
							ui.Text(c, "·").FontSize(11).TextColor(tc.TextMuted)
						}
						ui.Text(c, formatLastUpdated(acc.LastUpdated)).FontSize(11).TextColor(tc.TextMuted)
					})
				})
			})

			// Right side action buttons: Refresh Icon, Set Active, Delete
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				// Refresh Icon Button
				refBtn := ui.Row(c).Size(28, 28).Radius(6).AlignItems(ui.Center).Justify(ui.Center).
					Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border)
				refBtn.Children(func() {
					ui.Icon(c, icons.TablerRefresh).Width(13).Height(13).TextColor(tc.IconMuted)
				})
				if refBtn.Hovered() {
					refBtn.Background(tc.Text.Alpha(0.08))
				}
				if refBtn.Clicked() {
					a.refreshSingleAntigravityAccountAsync(acc.ID)
					c.AddToast(ui.Toast{Title: "Refreshing Quota", Description: fmt.Sprintf("Updating %s...", acc.Email), Type: "info"})
				}

				// Switch / Set Active Button
				if !acc.IsActive {
					switchBtn := ui.Row(c).Height(28).Radius(6).Padding(0, 10).Gap(4).AlignItems(ui.Center).
						Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border)
					switchBtn.Children(func() {
						ui.Icon(c, icons.TablerCheck).Width(12).Height(12).TextColor(tc.IconMuted)
						ui.Text(c, "Set Active").FontSize(11).FontWeight(500).TextColor(tc.Text)
					})
					if switchBtn.Hovered() {
						switchBtn.Background(tc.Text.Alpha(0.08))
					}
					if switchBtn.Clicked() {
						a.switchAntigravityAccountAsync(acc.ID)
					}
				}

				// Delete Account Button
				delBtn := ui.Row(c).Size(28, 28).Radius(6).AlignItems(ui.Center).Justify(ui.Center).
					Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border)
				delBtn.Children(func() {
					ui.Icon(c, icons.TablerTrash).Width(13).Height(13).TextColor(tc.IconMuted)
				})
				if delBtn.Hovered() {
					delBtn.Background(tc.Error.Alpha(0.12))
				}
				if delBtn.Clicked() {
					a.removeAntigravityAccount(acc.ID)
					c.AddToast(ui.Toast{Title: "Account Removed", Description: fmt.Sprintf("Removed %s from tracker", acc.Email), Type: "info"})
				}
			})
		})

		// Quotas Body: Gemini & Claude/GPT Window Gauges
		q := acc.Quota
		hasQuota := (q != nil && q.QuotaKnown)

		ui.Row(c).FillWidth().Gap(14).Children(func() {
			// Box 1: Gemini Models
			ui.Column(c).Grow(1).Padding(12, 14).Radius(8).Background(tc.Canvas).
				BorderWidth(1).BorderColor(tc.Border.Alpha(0.6)).Gap(10).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
					ui.Text(c, "GEMINI MODELS").FontSize(10).FontWeight(700).LetterSpacing(0.5).
						TextColor(tc.SidebarMutedForeground)
					ui.Text(c, "Gemini Flash & Pro").FontSize(10).TextColor(tc.TextMuted)
				})

				gemini5h := 100.0
				gemini5hReset := "Quota available"
				geminiWeekly := 100.0
				geminiWeeklyReset := "Quota available"
				if hasQuota {
					gemini5h = q.Gemini5hPercent
					gemini5hReset = q.Gemini5hReset
					geminiWeekly = q.GeminiWeeklyPercent
					geminiWeeklyReset = q.GeminiWeeklyReset
				}

				renderQuotaProgressBar(c, tc, "5-Hour Window", gemini5h, gemini5hReset, hasQuota)
				renderQuotaProgressBar(c, tc, "Weekly Limit", geminiWeekly, geminiWeeklyReset, hasQuota)
			})

			// Box 2: Claude and GPT Models
			ui.Column(c).Grow(1).Padding(12, 14).Radius(8).Background(tc.Canvas).
				BorderWidth(1).BorderColor(tc.Border.Alpha(0.6)).Gap(10).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
					ui.Text(c, "CLAUDE AND GPT MODELS").FontSize(10).FontWeight(700).LetterSpacing(0.5).
						TextColor(tc.SidebarMutedForeground)
					ui.Text(c, "Claude Sonnet, Opus & GPT").FontSize(10).TextColor(tc.TextMuted)
				})

				claude5h := 100.0
				claude5hReset := "Quota available"
				claudeWeekly := 100.0
				claudeWeeklyReset := "Quota available"
				if hasQuota {
					claude5h = q.Claude5hPercent
					claude5hReset = q.Claude5hReset
					claudeWeekly = q.ClaudeWeeklyPercent
					claudeWeeklyReset = q.ClaudeWeeklyReset
				}

				renderQuotaProgressBar(c, tc, "5-Hour Window", claude5h, claude5hReset, hasQuota)
				renderQuotaProgressBar(c, tc, "Weekly Limit", claudeWeekly, claudeWeeklyReset, hasQuota)
			})

			// Box 3: Gemini 4 Argon Models (if authorized for account)
			if hasQuota && (q.ArgonWeeklyPercent > 0 || q.ArgonWeeklyReset != "") {
				ui.Column(c).Grow(1).Padding(12, 14).Radius(8).Background(tc.Canvas).
					BorderWidth(1).BorderColor(tc.Border.Alpha(0.6)).Gap(10).Children(func() {
					ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
						ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
							ui.Text(c, "GEMINI 4 ARGON").FontSize(10).FontWeight(700).LetterSpacing(0.5).
								TextColor(tc.SidebarMutedForeground)
							ui.Box(c).Padding(2, 6).Radius(4).Background(tc.Accent.Alpha(0.15)).Children(func() {
								ui.Text(c, "ARGON TIER").FontSize(9).FontWeight(700).TextColor(tc.Accent)
							})
						})
						ui.Text(c, "Reasoning & Coding").FontSize(10).TextColor(tc.TextMuted)
					})

					renderQuotaProgressBar(c, tc, "Weekly Quota", q.ArgonWeeklyPercent, q.ArgonWeeklyReset, true)

					if acc.IsActive {
						modelPaletteBtn := ui.Row(c).Height(24).Radius(5).Padding(0, 8).Gap(4).AlignItems(ui.Center).
							Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
						if modelPaletteBtn.Hovered() {
							modelPaletteBtn.Background(tc.ToolbarControlHover)
						}
						modelPaletteBtn.Children(func() {
							ui.Icon(c, icons.TablerSparkles).Width(12).Height(12).TextColor(tc.Accent)
							ui.Text(c, "Available in Model Palette (Cmd+P)").FontSize(10).FontWeight(500).TextColor(tc.Text)
						})
						if modelPaletteBtn.Clicked() {
							a.ShowModelPalette = true
							a.SelectedHarnessID = "agent-agy"
							a.syncAntigravityModelsToACP()
							a.invalidateUI()
						}
					}
				})
			}
		})

		// Error Message Banner (if any)
		if acc.StatusMessage != "" && acc.Status != "ok" {
			ui.Row(c).Padding(8, 12).Radius(6).Background(tc.Error.Alpha(0.1)).BorderWidth(1).BorderColor(tc.Error.Alpha(0.25)).Children(func() {
				ui.Icon(c, icons.TablerAlertTriangle).Width(13).Height(13).TextColor(tc.Error)
				ui.Text(c, acc.StatusMessage).FontSize(11).TextColor(tc.Error)
			})
		}
	})
}

// renderQuotaProgressBar renders an individual quota gauge with percent fill and countdown text.
func renderQuotaProgressBar(c *ui.Context, tc theme.ThemeColors, label string, pct float64, resetsIn string, known bool) {
	if !known {
		ui.Column(c).Gap(3).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
				ui.Text(c, label).FontSize(11).FontWeight(500).TextColor(tc.TextMuted)
				ui.Text(c, "Unavailable").FontSize(11).FontWeight(600).TextColor(tc.TextMuted)
			})
			ui.Box(c).FillWidth().Height(5).Radius(3).Background(tc.Border.Alpha(0.4))
			ui.Text(c, "Refresh to synchronize quota limits").FontSize(10).TextColor(tc.TextMuted)
		})
		return
	}

	color := tc.Success
	if pct <= 15 {
		color = tc.Error
	} else if pct <= 30 {
		color = tc.Warning
	}

	subtext := "Quota available"
	if pct <= 0 {
		color = tc.Error
		subtext = "Quota exhausted"
	} else if resetsIn != "" && !strings.HasPrefix(resetsIn, "Quota") {
		subtext = resetsIn
	}

	ui.Column(c).Gap(3).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
			ui.Text(c, label).FontSize(11).FontWeight(500).TextColor(tc.Text)
			ui.Text(c, fmt.Sprintf("%.1f%%", pct)).FontSize(11).FontWeight(700).TextColor(color)
		})
		ui.Box(c).FillWidth().Height(5).Radius(3).Background(tc.Border.Alpha(0.6)).Children(func() {
			ui.Box(c).WidthPercent(float32(pct)).Height(5).Radius(3).Background(color)
		})
		ui.Text(c, subtext).FontSize(10).TextColor(color)
	})
}

// formatLastUpdated prints a human-readable duration since last update.
func formatLastUpdated(ts int64) string {
	if ts <= 0 {
		return "Never updated"
	}
	diff := time.Since(time.Unix(ts, 0))
	if diff < time.Minute {
		return "Updated just now"
	}
	if diff < time.Hour {
		return fmt.Sprintf("Updated %dm ago", int(diff.Minutes()))
	}
	if diff < 24*time.Hour {
		return fmt.Sprintf("Updated %dh ago", int(diff.Hours()))
	}
	return fmt.Sprintf("Updated %dd ago", int(diff.Hours()/24))
}

// ---------------------------------------------------------------------------
// Async Controller Actions
// ---------------------------------------------------------------------------

func (a *AppUI) refreshAllAntigravityAccountsAsync() {
	if a.Settings == nil {
		return
	}
	sm := a.Settings
	sm.IsRefreshingQuota = true
	a.invalidateUI()

	go func() {
		mgr := sm.AntigravityAccountMgr()
		_, _ = mgr.RefreshAll()
		a.runOnUI(func() {
			sm.IsRefreshingQuota = false
			a.invalidateUI()
		})
	}()
}

func (a *AppUI) refreshSingleAntigravityAccountAsync(accountID string) {
	if a.Settings == nil {
		return
	}
	sm := a.Settings
	sm.RefreshingAccountID = accountID
	a.invalidateUI()

	go func() {
		mgr := sm.AntigravityAccountMgr()
		_, _ = mgr.RefreshAccount(accountID)
		a.runOnUI(func() {
			if sm.RefreshingAccountID == accountID {
				sm.RefreshingAccountID = ""
			}
			a.invalidateUI()
		})
	}()
}

func (a *AppUI) importLocalAntigravityAccountAsync() {
	if a.Settings == nil {
		return
	}
	sm := a.Settings
	sm.IsRefreshingQuota = true
	a.invalidateUI()

	go func() {
		mgr := sm.AntigravityAccountMgr()
		acc, err := mgr.ImportLocalAccount()
		a.runOnUI(func() {
			sm.IsRefreshingQuota = false
			if err != nil {
				sm.AccountStatusMessage = fmt.Sprintf("Import failed: %v", err)
			} else if acc != nil {
				sm.AccountStatusMessage = ""
			}
			a.invalidateUI()
		})
	}()
}

func (a *AppUI) switchAntigravityAccountAsync(accountID string) {
	if a.Settings == nil {
		return
	}
	sm := a.Settings
	mgr := sm.AntigravityAccountMgr()
	if mgr == nil {
		return
	}
	sm.IsRefreshingQuota = true
	a.invalidateUI()

	go func() {
		_ = mgr.SetActive(accountID)
		a.runOnUI(func() {
			a.syncAntigravityModelsToACP()
			a.restartActiveAntigravitySession()
			a.refreshQuotaAsync()
			sm.IsRefreshingQuota = false
			a.invalidateUI()
		})
	}()
}

// syncAntigravityModelsToACP synchronizes the available models from the active Antigravity
// account to the ACP agent definition, so the model picker palette immediately displays
// all supported models (including Gemini 4 Argon, Neon, etc.) without requiring a restart.
func (a *AppUI) syncAntigravityModelsToACP() {
	if a.Settings == nil || a.ACPMgr == nil {
		return
	}
	mgr := a.Settings.AntigravityAccountMgr()
	if mgr == nil {
		return
	}
	activeAcc := mgr.ActiveAccount()
	if activeAcc == nil {
		return
	}
	models := activeAcc.AvailableModels
	if len(models) == 0 {
		if activeAcc.SupportsArgon() {
			models = quota.DefaultArgonModels()
		} else {
			models = quota.DefaultAntigravityModels()
		}
	}
	if len(models) > 0 {
		a.ACPMgr.UpdateAgentSupportedModels("agent-agy", models)
	}
}

func (a *AppUI) restartActiveAntigravitySession() {
	// 1. Restart CLI agent if running Antigravity (agy)
	if a.ActiveTaskKind == "cli" && a.CLIMgr != nil {
		if sess := a.CLIMgr.ActiveSession(); sess != nil {
			id := strings.ToLower(sess.AgentID)
			cmd := strings.ToLower(sess.Command)
			if strings.Contains(id, "agy") || strings.Contains(id, "antigravity") || strings.Contains(cmd, "agy") {
				sessID := sess.ID
				go a.CLIMgr.RestartSession(sessID)
			}
		}
	}

	// 2. Restart ACP agent session if running Antigravity (agent-agy / agy / antigravity)
	if a.Agent != nil {
		if sess := a.Agent.ActiveSession(); sess != nil {
			id := strings.ToLower(sess.AgentID)
			if id == "agent-agy" || id == "agy" || strings.Contains(id, "agy") || strings.Contains(id, "antigravity") || sess.AgentType == "acp" {
				sessID := sess.ID
				go func() {
					_ = a.Agent.RestartACPSession(sessID)
				}()
			}
		}
	}
}

// isQuotaExhaustedError checks if an error message represents a rate limit / quota exhaustion error.
func isQuotaExhaustedError(errStr string) bool {
	if errStr == "" {
		return false
	}
	lower := strings.ToLower(errStr)
	return strings.Contains(lower, "429") ||
		strings.Contains(lower, "resource_exhausted") ||
		strings.Contains(lower, "quota") ||
		strings.Contains(lower, "rate limit") ||
		strings.Contains(lower, "rate_limit") ||
		strings.Contains(lower, "insufficient_quota") ||
		strings.Contains(lower, "quotaresetdelay")
}

func (a *AppUI) removeAntigravityAccount(accountID string) {
	if a.Settings == nil {
		return
	}
	sm := a.Settings
	mgr := sm.AntigravityAccountMgr()
	_ = mgr.RemoveAccount(accountID)
	a.invalidateUI()
}

func (a *AppUI) addAntigravityAccountAsync(input, label string) {
	if a.Settings == nil {
		return
	}
	sm := a.Settings
	sm.IsRefreshingQuota = true
	a.invalidateUI()

	go func() {
		mgr := sm.AntigravityAccountMgr()
		acc, err := mgr.AddAccount(input, label)
		a.runOnUI(func() {
			sm.IsRefreshingQuota = false
			if err != nil {
				sm.AccountStatusMessage = fmt.Sprintf("Failed to add account: %v", err)
			} else {
				sm.NewAccountInput = ""
				sm.NewAccountLabel = ""
				sm.IsAddingAccount = false
				sm.AccountStatusMessage = ""
				if acc != nil && acc.IsActive {
					a.refreshQuotaAsync()
				}
			}
			a.invalidateUI()
		})
	}()
}

func (a *AppUI) startAntigravityOAuthAsync(label string) {
	if a.Settings == nil {
		return
	}
	sm := a.Settings
	mgr := sm.AntigravityAccountMgr()
	if mgr == nil {
		return
	}

	sm.IsOAuthFlowActive = true
	sm.OAuthFlowURL = ""
	sm.OAuthFlowStatus = "Starting browser sign-in..."
	sm.AccountStatusMessage = ""
	a.invalidateUI()

	ctx := context.Background()
	go func() {
		acc, err := mgr.StartOAuthLogin(ctx, label, func(authURL string) {
			a.runOnUI(func() {
				sm.OAuthFlowURL = authURL
				sm.OAuthFlowStatus = "Waiting for authorization in browser..."
				a.invalidateUI()
			})
		})

		a.runOnUI(func() {
			sm.IsOAuthFlowActive = false
			sm.OAuthFlowURL = ""
			sm.OAuthFlowStatus = ""
			if err != nil {
				if err != context.Canceled {
					sm.AccountStatusMessage = fmt.Sprintf("OAuth error: %v", err)
				}
			} else if acc != nil {
				sm.IsAddingAccount = false
				sm.NewAccountLabel = ""
				sm.AccountStatusMessage = ""
				if acc.IsActive {
					a.refreshQuotaAsync()
				}
			}
			a.invalidateUI()
		})
	}()
}

func (a *AppUI) cancelAntigravityOAuth() {
	if a.Settings == nil {
		return
	}
	sm := a.Settings
	mgr := sm.AntigravityAccountMgr()
	if mgr != nil {
		mgr.CancelOAuth()
	}
	sm.IsOAuthFlowActive = false
	sm.OAuthFlowURL = ""
	sm.OAuthFlowStatus = ""
	a.invalidateUI()
}
