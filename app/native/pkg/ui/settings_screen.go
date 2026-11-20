package ui

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/Haslab-dev/RakitKode/llm"
	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/settings"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
)

// renderSettingsScreen renders ForgeADE's full Settings Screen experience.
func (a *AppUI) renderSettingsScreen(c *ui.Context) {
	sm := a.Settings
	if sm == nil || !sm.IsOpen {
		return
	}
	tc := a.ThemeMgr.Colors()

	if c.Shortcut(0, ui.KeyEscape) {
		a.closeSettings()
		return
	}

	// ForgeADE's settings is a full-screen surface with its own nav.
	{
		ui.Column(c).Fill().Background(tc.SurfaceOverlay).Children(func() {

			bar := c.TitleBar()
			leftPad := float32(16)
			if bar.Left > 0 {
				leftPad = max(bar.Left+10, 78)
			} else if runtime.GOOS == "darwin" {
				leftPad = 78
			}
			rightPad := float32(16)
			if bar.Right > 0 {
				rightPad = max(bar.Right+12, 135)
			} else if bar.Left == 0 && (runtime.GOOS == "linux" || runtime.GOOS == "windows") {
				rightPad = 135
			}

			// 1. Header Bar: Back Button, Breadcrumb, Search Bar, Close Button
			ui.Row(c).Height(WorkspaceTopBarHeight).Padding(0, rightPad, 0, leftPad).Gap(12).AlignItems(ui.Center).
				BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).DragWindow().Children(func() {

				backBtn := ui.Row(c).Height(28).Radius(6).Padding(0, 8).Gap(6).AlignItems(ui.Center).
					Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
				backBtn.Children(func() {
					ui.Icon(c, icons.TablerArrowLeft).Width(13).Height(13).TextColor(tc.IconMuted)
					ui.Text(c, "Back to workspace").FontSize(12).FontWeight(500)
				})
				if backBtn.Clicked() {
					a.closeSettings()
				}

				// Breadcrumb
				sectionName := settingsSectionTitle(sm.ActiveTab)
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "Settings").FontSize(13).TextColor(tc.TextMuted)
					ui.Icon(c, icons.TablerChevronRight).Width(11).Height(11).TextColor(tc.IconMuted)
					ui.Text(c, sectionName).FontSize(13).FontWeight(600)
				})

				ui.Box(c).Grow(1)

				// Search input
				ui.Row(c).Width(220).Height(28).Radius(6).Padding(0, 8).Gap(6).AlignItems(ui.Center).
					Background(tc.Canvas).BorderWidth(1).BorderColor(tc.Border).Children(func() {
					ui.Icon(c, icons.TablerSearch).Width(12).Height(12).TextColor(tc.IconMuted)
					ui.TextInput(c, &sm.SearchQuery).Placeholder("Search settings...").Grow(1)
				})

				closeBtn := ui.Row(c).Size(24, 24).Radius(6).AlignItems(ui.Center).Justify(ui.Center)
				closeBtn.Children(func() {
					ui.Icon(c, icons.TablerX).Width(13).Height(13).TextColor(tc.IconMuted)
				})
				if closeBtn.Clicked() {
					a.closeSettings()
				}
			})

			// 2. Main Body: Split Sidebar & Content Area
			ui.Row(c).Grow(1).FillWidth().Children(func() {
				// Left Navigation Sidebar (210px)
				ui.Column(c).Width(210).FillHeight().Background(tc.Sidebar).
					BorderWidth(0, 1, 0, 0).BorderColor(tc.SidebarBorder).
					Padding(12, 8).Gap(14).Children(func() {

					// Group 1: Models & Agents
					ui.Column(c).Gap(2).Children(func() {
						ui.Row(c).Padding(2, 8, 4, 8).Children(func() {
							ui.Text(c, "MODELS & AGENTS").FontSize(10).FontWeight(700).LetterSpacing(0.5).
								TextColor(tc.SidebarMutedForeground)
						})

						a.renderSettingsNavItem(c, tc, sm, settings.SettingsTabModels, "Providers & Models", icons.TablerPackage, "")
						a.renderSettingsNavItem(c, tc, sm, settings.SettingsTabAgents, "ACP & Agent CLIs", icons.TablerBot, "ACP")
						a.renderSettingsNavItem(c, tc, sm, settings.SettingsTabAntigravityQuota, "Antigravity Quota", icons.TablerChartBar, "MULTI")
						a.renderSettingsNavItem(c, tc, sm, settings.SettingsTabMCP, "MCP Servers", icons.TablerPlug, "")
						a.renderSettingsNavItem(c, tc, sm, settings.SettingsTabSkills, "Skills", icons.TablerWand, "")
						a.renderSettingsNavItem(c, tc, sm, settings.SettingsTabPlugins, "Plugins", icons.TablerPuzzle, "")
						a.renderSettingsNavItem(c, tc, sm, settings.SettingsTabSubagents, "Orchestration & Roles", icons.TablerBot, "MULTI")
						a.renderSettingsNavItem(c, tc, sm, settings.SettingsTabCommands, "Slash Commands", icons.TablerCode, "")
					})

					// Group 2: Workspace & Capabilities
					ui.Column(c).Gap(2).Children(func() {
						ui.Row(c).Padding(2, 8, 4, 8).Children(func() {
							ui.Text(c, "WORKSPACE & CAPABILITIES").FontSize(10).FontWeight(700).LetterSpacing(0.5).
								TextColor(tc.SidebarMutedForeground)
						})

						a.renderSettingsNavItem(c, tc, sm, settings.SettingsTabGeneral, "General", icons.TablerSettings, "")
						a.renderSettingsNavItem(c, tc, sm, settings.SettingsTabAppearance, "Appearance", icons.TablerPalette, "")
						a.renderSettingsNavItem(c, tc, sm, settings.SettingsTabMemory, "Memory", icons.TablerBrain, "")
						a.renderSettingsNavItem(c, tc, sm, settings.SettingsTabShortcuts, "Shortcuts & Keys", icons.TablerKeyboard, "")
						a.renderSettingsNavItem(c, tc, sm, settings.SettingsTabBrowser, "Browser Use", icons.TablerWorld, "")
						a.renderSettingsNavItem(c, tc, sm, settings.SettingsTabComputer, "Computer Use", icons.TablerCpu, "")
						a.renderSettingsNavItem(c, tc, sm, settings.SettingsTabIndexing, "Codebase Indexing", icons.TablerDatabase, "")
					})

					// Group 3: Automation
					ui.Column(c).Gap(2).Children(func() {
						ui.Row(c).Padding(2, 8, 4, 8).Children(func() {
							ui.Text(c, "AUTOMATION").FontSize(10).FontWeight(700).LetterSpacing(0.5).
								TextColor(tc.SidebarMutedForeground)
						})
						a.renderSettingsNavItem(c, tc, sm, settings.SettingsTabAutos, "Automations", icons.TablerClock, "")
						a.renderSettingsNavItem(c, tc, sm, settings.SettingsTabHooks, "Lifecycle Hooks", icons.TablerGitFork, "")
					})
				})

				// Right Content Pane
				ui.Column(c).Grow(1).FillHeight().Padding(16, 24).Children(func() {
					ui.Scroll(c).Grow(1).Children(func() {
						ui.Column(c).Gap(18).Children(func() {
							switch sm.ActiveTab {
							case settings.SettingsTabModels:
								a.renderSettingsTabModels(c, tc, sm)
							case settings.SettingsTabAgents:
								a.renderSettingsTabAgents(c, tc, sm)
							case settings.SettingsTabAntigravityQuota:
								a.renderSettingsTabAntigravityQuota(c, tc, sm)
							case settings.SettingsTabMCP:
								a.renderSettingsTabMCPReal(c, tc, sm)
							case settings.SettingsTabSkills:
								a.renderSettingsTabSkillsReal(c, tc, sm)
							case settings.SettingsTabPlugins:
								a.renderSettingsTabPlugins(c, tc, sm)
							case settings.SettingsTabMemory:
								a.renderSettingsTabMemory(c, tc, sm)
							case settings.SettingsTabAutos:
								a.renderSettingsTabAutomations(c, tc, sm)
							case settings.SettingsTabSubagents:
								a.renderSettingsTabSubagents(c, tc, sm)
							case settings.SettingsTabCommands:
								a.renderSettingsTabCommands(c, tc, sm)
							case settings.SettingsTabGeneral:
								a.renderSettingsTabGeneral(c, tc, sm)
							case settings.SettingsTabAppearance:
								a.renderSettingsTabAppearance(c, tc, sm)
							case settings.SettingsTabShortcuts:
								a.renderSettingsTabShortcuts(c, tc, sm)
							case settings.SettingsTabBrowser:
								a.renderSettingsTabBrowser(c, tc, sm)
							case settings.SettingsTabComputer:
								a.renderSettingsTabComputer(c, tc, sm)
							case settings.SettingsTabIndexing:
								a.renderSettingsTabIndexing(c, tc, sm)
							case settings.SettingsTabHooks:
								a.renderSettingsTabHooks(c, tc, sm)
							}
						})
					})
				})
			})
		})
	}
}

func settingsSectionTitle(tab int) string {
	switch tab {
	case settings.SettingsTabModels:
		return "Providers & Models"
	case settings.SettingsTabAgents:
		return "ACP & Agent CLIs"
	case settings.SettingsTabAntigravityQuota:
		return "Antigravity Quota Tracker"
	case settings.SettingsTabMCP:
		return "MCP Servers"
	case settings.SettingsTabSkills:
		return "Skills"
	case settings.SettingsTabPlugins:
		return "Plugins"
	case settings.SettingsTabMemory:
		return "Memory"
	case settings.SettingsTabAutos:
		return "Automations"
	case settings.SettingsTabSubagents:
		return "Orchestration & Sub-Agent Personas"
	case settings.SettingsTabCommands:
		return "Slash Commands"
	case settings.SettingsTabGeneral:
		return "General"
	case settings.SettingsTabAppearance:
		return "Appearance"
	case settings.SettingsTabShortcuts:
		return "Shortcuts & Keys"
	case settings.SettingsTabBrowser:
		return "Browser Use"
	case settings.SettingsTabComputer:
		return "Computer Use"
	case settings.SettingsTabIndexing:
		return "Codebase Indexing"
	case settings.SettingsTabHooks:
		return "Lifecycle Hooks"
	default:
		return "Settings"
	}
}

func (a *AppUI) renderSettingsNavItem(c *ui.Context, tc theme.ThemeColors, sm *settings.SettingsManager, tab int, label string, icon *ui.SVG, badge string) {
	// Search filter hides non-matching sections (SettingsScreen parity).
	if q := strings.ToLower(strings.TrimSpace(sm.SearchQuery)); q != "" && !strings.Contains(strings.ToLower(label), q) {
		return
	}

	isActive := (sm.ActiveTab == tab)
	btn := ui.Row(c.Key("nav-tab-"+label)).Height(30).Radius(8).Padding(0, 10).Gap(8).AlignItems(ui.Center)
	btn.Children(func() {
		ic := ui.Icon(c, icon).Width(15).Height(15)
		if isActive {
			ic.TextColor(tc.Text)
		} else {
			ic.TextColor(tc.IconMuted)
		}

		txt := ui.Text(c, label).FontSize(12).Grow(1)
		if isActive {
			txt.FontWeight(600).TextColor(tc.Text)
		} else {
			txt.TextColor(tc.SidebarForeground)
		}

		if badge != "" {
			b := ui.Row(c).Radius(4).Padding(1, 5).Background(tc.Accent.Alpha(0.15))
			b.Children(func() {
				ui.Text(c, badge).FontSize(9).FontWeight(700).TextColor(tc.Accent)
			})
		}
	})

	if isActive {
		btn.Background(tc.SidebarRowSelected)
	} else if btn.Hovered() {
		btn.Background(tc.SidebarRowHover)
	}
	if btn.Clicked() {
		sm.ActiveTab = tab
	}
}

// ---------------------------------------------------------------------------
// Settings Sections
// ---------------------------------------------------------------------------

func providerProtocol(p llm.ProviderProfile) string {
	idLower := strings.ToLower(p.ID)
	urlLower := strings.ToLower(p.BaseURL)
	if strings.Contains(idLower, "anthropic") || strings.Contains(urlLower, "anthropic") {
		return "anthropic"
	}
	return "openai"
}

func (a *AppUI) renderSettingsTabModels(c *ui.Context, tc theme.ThemeColors, sm *settings.SettingsManager) {
	settingsSectionHeader(c, tc, "Providers & Models", "Configure LLM provider API credentials, endpoints, and default model selection.")

	// UI-owned deep copy: GetProviderProfiles returns the manager's live
	// slice, so every edit below lands on the copy and persistence happens
	// exclusively through SaveProfiles / DeleteProfile (atomic swaps under
	// the manager lock). Re-synced each frame so fetch results show up.
	profiles := settings.CopyProfiles(sm.GetProfiles())
	if sm.SelectedProviderID == "" && len(profiles) > 0 {
		sm.SelectedProviderID = profiles[0].ID
	}

	var selectedProfile *llm.ProviderProfile
	var selectedIdx int = -1
	for i := range profiles {
		if profiles[i].ID == sm.SelectedProviderID {
			selectedProfile = &profiles[i]
			selectedIdx = i
			break
		}
	}
	if selectedProfile == nil && len(profiles) > 0 {
		selectedProfile = &profiles[0]
		selectedIdx = 0
		sm.SelectedProviderID = profiles[0].ID
	}
	if selectedProfile != nil && sm.EditingProfileID != selectedProfile.ID {
		sm.EditingProfileID = selectedProfile.ID
		sm.EditProfileName = selectedProfile.Name
		sm.EditBaseURL = selectedProfile.BaseURL
		sm.EditAPIKey = selectedProfile.APIKey
	}

	// Two-column container matching ForgeADE SettingsScreen
	ui.Row(c).Radius(12).BorderWidth(1).BorderColor(tc.Border).Background(tc.Surface).Clip().Children(func() {
		// Left: Custom Provider List (240px)
		ui.Column(c).Width(240).BorderWidth(0, 1, 0, 0).BorderColor(tc.Border).
			Background(tc.Surface.Alpha(0.5)).Padding(12, 10).Gap(6).Children(func() {
			ui.Text(c, "CUSTOM PROVIDERS").FontSize(10).FontWeight(700).LetterSpacing(0.6).TextColor(tc.TextMuted)

			if len(profiles) == 0 {
				ui.Text(c, "No providers configured. Add a custom OpenAI or Anthropic endpoint.").
					FontSize(11).TextColor(tc.TextMuted).LineHeight(1.4)
			} else {
				for _, p := range profiles {
					prof := p
					isSelected := !sm.IsAddingProvider && selectedProfile != nil && selectedProfile.ID == prof.ID
					hasKey := strings.TrimSpace(prof.APIKey) != ""
					proto := providerProtocol(prof)
					modelCount := len(prof.SelectedModels)
					if modelCount == 0 {
						modelCount = len(prof.AvailableModels)
					}

					btn := ui.Row(c.Key("pcard-"+prof.ID)).Radius(8).Padding(6, 8).Gap(8).AlignItems(ui.Center)
					if isSelected {
						btn.Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border)
					} else if btn.Hovered() {
						btn.Background(tc.Text.Alpha(0.04))
					}

					btn.Children(func() {
						ui.Box(c).Size(24, 24).Radius(6).Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border.Alpha(0.6)).
							AlignItems(ui.Center).Justify(ui.Center).Children(func() {
							icons.RenderProviderIcon(c, proto, 14)
						})

						ui.Column(c).Grow(1).Gap(1).Children(func() {
							dispName := prof.Name
							if strings.TrimSpace(dispName) == "" {
								dispName = prof.ID
							}
							ui.Text(c, dispName).FontSize(12).FontWeight(500).SingleLine()

							keyLabel := "No key"
							if hasKey {
								keyLabel = "Key set"
							}
							sub := fmt.Sprintf("%s · %d models · %s", strings.ToUpper(proto), modelCount, keyLabel)
							ui.Text(c, sub).FontSize(10).TextColor(tc.TextMuted).SingleLine()
						})

						// Status indicator dot
						dotColor := tc.TextMuted.Alpha(0.4)
						if prof.Enabled {
							if hasKey {
								dotColor = tc.Success
							} else {
								dotColor = tc.Warning
							}
						}
						ui.Box(c).Size(8, 8).Radius(4).Background(dotColor)
					})

					if btn.Clicked() {
						sm.SelectedProviderID = prof.ID
						sm.EditingProfileID = prof.ID
						sm.EditProfileName = prof.Name
						sm.EditBaseURL = prof.BaseURL
						sm.EditAPIKey = prof.APIKey
						sm.IsAddingProvider = false
						sm.ModelSearchQuery = ""
					}
				}
			}

			// Add Provider Button
			addBtn := ui.Row(c.Key("add-prov-btn")).Height(32).Radius(8).Padding(0, 10).Gap(6).
				AlignItems(ui.Center).BorderWidth(1).BorderColor(tc.Border.Alpha(0.7))
			if addBtn.Hovered() {
				addBtn.Background(tc.Text.Alpha(0.04))
			}
			addBtn.Children(func() {
				ui.Icon(c, icons.TablerPlus).Width(13).Height(13).TextColor(tc.TextMuted)
				ui.Text(c, "Add Custom Provider").FontSize(11).FontWeight(600)
			})
			if addBtn.Clicked() {
				sm.IsAddingProvider = true
				sm.NewProviderProtocol = "openai"
				sm.NewProviderName = ""
				sm.NewProviderBaseURL = ""
				sm.NewProviderAPIKey = ""
				sm.NewProviderInitialModel = ""
			}
		})

		// Right: Detail / Form Panel
		ui.Column(c).Grow(1).Padding(16, 20).Gap(14).Children(func() {
			if sm.IsAddingProvider {
				// ADD CUSTOM PROVIDER FORM
				ui.Column(c).Gap(12).Children(func() {
					ui.Column(c).Gap(2).Children(func() {
						ui.Text(c, "Add Custom Provider").FontSize(16).FontWeight(600)
						ui.Text(c, "Configure a custom OpenAI-compatible or Anthropic-compatible API endpoint.").
							FontSize(11).TextColor(tc.TextMuted)
					})

					// Protocol Selector
					ui.Column(c).Gap(4).Children(func() {
						ui.Text(c, "API Protocol").FontSize(11).FontWeight(600).TextColor(tc.TextMuted)
						ui.Row(c).Gap(8).Children(func() {
							isOpenAI := sm.NewProviderProtocol == "openai"
							isAnthropic := sm.NewProviderProtocol == "anthropic"

							pOpenAI := ui.Row(c.Key("proto-openai")).Grow(1).Radius(8).Padding(8, 12).Gap(8).AlignItems(ui.Center)
							if isOpenAI {
								pOpenAI.Background(tc.Accent.Alpha(0.12)).BorderWidth(1).BorderColor(tc.Accent)
							} else {
								pOpenAI.Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
							}
							pOpenAI.Children(func() {
								icons.RenderProviderIcon(c, "openai", 16)
								ui.Column(c).Gap(1).Children(func() {
									ui.Text(c, "OpenAI Compatible").FontSize(11).FontWeight(600)
									ui.Text(c, "/v1/chat/completions").FontSize(9).TextColor(tc.TextMuted)
								})
							})
							if pOpenAI.Clicked() {
								sm.NewProviderProtocol = "openai"
							}

							pAnth := ui.Row(c.Key("proto-anthropic")).Grow(1).Radius(8).Padding(8, 12).Gap(8).AlignItems(ui.Center)
							if isAnthropic {
								pAnth.Background(tc.Accent.Alpha(0.12)).BorderWidth(1).BorderColor(tc.Accent)
							} else {
								pAnth.Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
							}
							pAnth.Children(func() {
								icons.RenderProviderIcon(c, "anthropic", 16)
								ui.Column(c).Gap(1).Children(func() {
									ui.Text(c, "Anthropic Compatible").FontSize(11).FontWeight(600)
									ui.Text(c, "/v1/messages").FontSize(9).TextColor(tc.TextMuted)
								})
							})
							if pAnth.Clicked() {
								sm.NewProviderProtocol = "anthropic"
							}
						})
					})

					// Provider Name
					ui.Column(c).Gap(4).Children(func() {
						ui.Text(c, "Provider Name").FontSize(11).FontWeight(600).TextColor(tc.TextMuted)
						ui.TextInput(c, &sm.NewProviderName).Placeholder("e.g. Kenari Gateway, OpenRouter, Local Ollama").Grow(1)
					})

					// Base URL
					ui.Column(c).Gap(4).Children(func() {
						ui.Text(c, "Base URL").FontSize(11).FontWeight(600).TextColor(tc.TextMuted)
						defURL := "https://api.openai.com/v1"
						if sm.NewProviderProtocol == "anthropic" {
							defURL = "https://api.anthropic.com/v1"
						}
						ui.TextInput(c, &sm.NewProviderBaseURL).Placeholder(defURL).Grow(1)
					})

					// API Key
					ui.Column(c).Gap(4).Children(func() {
						ui.Text(c, "API Key").FontSize(11).FontWeight(600).TextColor(tc.TextMuted)
						ui.TextInput(c, &sm.NewProviderAPIKey).Placeholder("sk-... (optional for local endpoints)").Grow(1)
					})

					// Initial Models
					ui.Column(c).Gap(4).Children(func() {
						ui.Text(c, "Initial Models (comma-separated, or click Fetch Models after saving)").FontSize(11).FontWeight(600).TextColor(tc.TextMuted)
						ui.TextInput(c, &sm.NewProviderInitialModel).Placeholder("e.g. mimo-v2-6-flash, gpt-4o, claude-3-7-sonnet").Grow(1)
					})

					// Form Actions
					ui.Row(c).Gap(8).Padding(4, 0, 0, 0).Children(func() {
						saveBtn := ui.PrimaryButton(c, "Save Custom Provider")
						if saveBtn.Clicked() {
							name := strings.TrimSpace(sm.NewProviderName)
							if name == "" {
								name = "Custom Gateway"
							}
							bURL := strings.TrimSpace(sm.NewProviderBaseURL)
							if bURL == "" {
								if sm.NewProviderProtocol == "anthropic" {
									bURL = "https://api.anthropic.com/v1"
								} else {
									bURL = "https://api.openai.com/v1"
								}
							}
							var models []string
							if sm.NewProviderInitialModel != "" {
								for _, m := range strings.Split(sm.NewProviderInitialModel, ",") {
									if tm := strings.TrimSpace(m); tm != "" {
										models = append(models, tm)
									}
								}
							}
							newID := fmt.Sprintf("prov-%d", time.Now().UnixMilli())
							newProf := llm.ProviderProfile{
								ID:              newID,
								Name:            name,
								APIKey:          strings.TrimSpace(sm.NewProviderAPIKey),
								BaseURL:         bURL,
								Enabled:         true,
								AvailableModels: models,
								SelectedModels:  models,
							}
							allProfs := append(settings.CopyProfiles(sm.GetProfiles()), newProf)
							if err := sm.SaveProfiles(allProfs); err != nil {
								c.AddToast(ui.Toast{Title: "Save Failed", Description: err.Error(), Type: "error"})
							} else {
								sm.SelectedProviderID = newID
								sm.IsAddingProvider = false
								if len(models) > 0 {
									a.Agent.ActiveModel = models[0]
									if a.Engine != nil && a.Engine.LLMClient != nil {
										a.Engine.LLMClient.SetActiveModel(newID, models[0])
									}
								}
								c.AddToast(ui.Toast{Title: "Provider Added", Description: name, Type: "success"})
							}
						}

						cancelBtn := ui.Button(c, "Cancel")
						if cancelBtn.Clicked() {
							sm.IsAddingProvider = false
						}
					})
				})
			} else if selectedProfile != nil {
				// DETAIL VIEW FOR SELECTED PROVIDER
				proto := providerProtocol(*selectedProfile)
				hasKey := strings.TrimSpace(selectedProfile.APIKey) != ""

				// Header
				ui.Row(c).Gap(10).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).
					BorderColor(tc.Border.Alpha(0.6)).Padding(0, 0, 10, 0).Children(func() {
					ui.Box(c).Size(32, 32).Radius(8).Background(tc.SurfaceRaised).
						BorderWidth(1).BorderColor(tc.Border).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
						icons.RenderProviderIcon(c, proto, 18)
					})

					ui.Column(c).Grow(1).Gap(2).Children(func() {
						ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
							ui.Text(c, selectedProfile.Name).FontSize(14).FontWeight(600)
							Badge(c, tc, strings.ToUpper(proto), tc.Accent)

							// Enabled / Disabled Toggle Pill
							enBtn := ui.Row(c.Key("entog-"+selectedProfile.ID)).Radius(999).Padding(1, 8).AlignItems(ui.Center)
							if selectedProfile.Enabled {
								enBtn.Background(tc.Success.Alpha(0.12)).BorderWidth(1).BorderColor(tc.Success.Alpha(0.3))
								enBtn.Children(func() {
									ui.Text(c, "Enabled").FontSize(10).FontWeight(600).TextColor(tc.Success)
								})
							} else {
								enBtn.Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border)
								enBtn.Children(func() {
									ui.Text(c, "Disabled").FontSize(10).TextColor(tc.TextMuted)
								})
							}
							if enBtn.Clicked() {
								// Mutate the UI-owned copy, then persist the whole list.
								selectedProfile.Enabled = !selectedProfile.Enabled
								if err := sm.SaveProfiles(profiles); err != nil {
									selectedProfile.Enabled = !selectedProfile.Enabled // revert on failure
									c.AddToast(ui.Toast{Title: "Save Failed", Description: err.Error(), Type: "error"})
								}
							}
						})

						ui.Text(c, selectedProfile.BaseURL).FontSize(11).TextColor(tc.TextMuted)
					})

					// Key status badge
					if hasKey {
						Badge(c, tc, "KEY CONFIGURED", tc.Success)
					} else {
						Badge(c, tc, "KEY MISSING", tc.Warning)
					}

					// Delete Provider Button
					delBtn := ui.Row(c.Key("delprov-"+selectedProfile.ID)).Radius(6).Padding(4, 8).Gap(4).AlignItems(ui.Center)
					if delBtn.Hovered() {
						delBtn.Background(tc.Error.Alpha(0.1))
					}
					delBtn.Children(func() {
						ui.Icon(c, icons.TablerTrash).Width(12).Height(12).TextColor(tc.Error)
						ui.Text(c, "Delete").FontSize(11).TextColor(tc.Error)
					})
					if delBtn.Clicked() {
						if err := sm.DeleteProfile(selectedProfile.ID); err != nil {
							c.AddToast(ui.Toast{Title: "Delete Failed", Description: err.Error(), Type: "error"})
						} else {
							newProfs := settings.CopyProfiles(sm.GetProfiles())
							sm.EditingProfileID = ""
							if len(newProfs) > 0 {
								sm.SelectedProviderID = newProfs[0].ID
							} else {
								sm.SelectedProviderID = ""
							}
							c.AddToast(ui.Toast{Title: "Provider Deleted", Description: selectedProfile.Name, Type: "info"})
						}
					}
				})

				// Edit credentials form (bound to persistent SettingsManager draft fields)
				ui.Column(c).Gap(8).Children(func() {
					ui.Column(c).Gap(4).Children(func() {
						ui.Text(c, "Display Name").FontSize(11).FontWeight(600).TextColor(tc.TextMuted)
						ui.TextInput(c, &sm.EditProfileName).Placeholder("Provider display name").Grow(1)
					})
					ui.Row(c).Gap(12).Children(func() {
						// Base URL
						ui.Column(c).Grow(1).Gap(4).Children(func() {
							ui.Text(c, "Base URL").FontSize(11).FontWeight(600).TextColor(tc.TextMuted)
							ui.TextInput(c, &sm.EditBaseURL).Grow(1)
						})

						// API Key with Show/Hide
						ui.Column(c).Grow(1).Gap(4).Children(func() {
							ui.Row(c).AlignItems(ui.Center).Children(func() {
								ui.Text(c, "API Key").FontSize(11).FontWeight(600).TextColor(tc.TextMuted)
								ui.Box(c).Grow(1)
								showKeyBtn := ui.Row(c).Gap(3).AlignItems(ui.Center)
								showKeyBtn.Children(func() {
									ic := icons.TablerEye
									txt := "Show"
									if sm.ShowAPIKey {
										ic = icons.TablerEyeOff
										txt = "Hide"
									}
									ui.Icon(c, ic).Width(11).Height(11).TextColor(tc.IconMuted)
									ui.Text(c, txt).FontSize(10).TextColor(tc.TextMuted)
								})
								if showKeyBtn.Clicked() {
									sm.ShowAPIKey = !sm.ShowAPIKey
								}
							})
							keyInput := ui.TextInput(c, &sm.EditAPIKey).Placeholder("Enter API Key...").Grow(1)
							if !sm.ShowAPIKey {
								keyInput.Password()
							}
						})
					})
				})

				// Save button
				ui.Row(c).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Grow(1)
					saveBtn := ui.Button(c, "Save Credentials")
					if saveBtn.Clicked() {
						if selectedIdx >= 0 && selectedIdx < len(profiles) {
							if n := strings.TrimSpace(sm.EditProfileName); n != "" {
								profiles[selectedIdx].Name = n
							}
							profiles[selectedIdx].BaseURL = strings.TrimSpace(sm.EditBaseURL)
							profiles[selectedIdx].APIKey = strings.TrimSpace(sm.EditAPIKey)
							if err := sm.SaveProfiles(profiles); err != nil {
								c.AddToast(ui.Toast{Title: "Save Failed", Description: err.Error(), Type: "error"})
							} else {
								if sm.Provider == profiles[selectedIdx].ID {
									sm.BaseURL = profiles[selectedIdx].BaseURL
									sm.APIKey = profiles[selectedIdx].APIKey
								}
								c.AddToast(ui.Toast{Title: "Saved", Description: profiles[selectedIdx].Name, Type: "success"})
							}
						}
					}
				})

				// Available Models Section
				ui.Column(c).Gap(8).Padding(6, 0, 0, 0).BorderWidth(1, 0, 0, 0).BorderColor(tc.Border.Alpha(0.4)).Children(func() {
					ui.Row(c).AlignItems(ui.Center).Children(func() {
						ui.Text(c, "Available Models").FontSize(13).FontWeight(600)
						ui.Box(c).Grow(1)

						// Fetch Models Button
						fetchBtn := ui.Row(c.Key("fetch-models-btn")).Radius(6).Padding(3, 8).Gap(4).
							AlignItems(ui.Center).Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border)
						if fetchBtn.Hovered() {
							fetchBtn.Background(tc.Text.Alpha(0.04))
						}
						fetchBtn.Children(func() {
							if sm.IsFetchingModels {
								ui.Spinner(c).Size(11, 11).Label("Fetching")
							} else {
								ui.Icon(c, icons.TablerRefresh).Width(11).Height(11).TextColor(tc.Success)
							}
							ui.Text(c, "Fetch models").FontSize(11).FontWeight(500)
						})
						if fetchBtn.Clicked() && !sm.IsFetchingModels {
							sm.IsFetchingModels = true
							go func(profID, apiKey, baseURL string) {
								var (
									models   []string
									fetchErr error
								)
								if a.Engine != nil && a.Engine.LLMClient != nil {
									ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
									defer cancel()
									models, fetchErr = a.Engine.LLMClient.FetchModels(ctx, apiKey, baseURL)
								}
								// Publish everything — spinner, toasts, and the
								// persisted profile list — on the UI thread; the
								// goroutine never touches the manager's live slice.
								a.runOnUI(func() {
									sm.IsFetchingModels = false
									switch {
									case fetchErr != nil:
										a.queueToast(ui.Toast{Title: "Fetch Error", Description: fetchErr.Error(), Type: "error"})
									case len(models) == 0:
										a.queueToast(ui.Toast{Title: "No Models", Description: "The provider returned no models", Type: "error"})
									default:
										allP := settings.CopyProfiles(sm.GetProfiles())
										for idx := range allP {
											if allP[idx].ID == profID {
												allP[idx].AvailableModels = models
												break
											}
										}
										if err := sm.SaveProfiles(allP); err != nil {
											a.queueToast(ui.Toast{Title: "Save Failed", Description: err.Error(), Type: "error"})
										} else {
											a.queueToast(ui.Toast{Title: "Models Fetched", Description: fmt.Sprintf("Found %d models", len(models)), Type: "success"})
										}
									}
									a.invalidateUI()
								})
							}(selectedProfile.ID, sm.EditAPIKey, sm.EditBaseURL)
						}
					})

					// Search models
					ui.Row(c).Height(30).Radius(6).Padding(0, 8).Gap(6).AlignItems(ui.Center).
						Background(tc.Canvas).BorderWidth(1).BorderColor(tc.Border).Children(func() {
						ui.Icon(c, icons.TablerSearch).Width(12).Height(12).TextColor(tc.IconMuted)
						ui.TextInput(c, &sm.ModelSearchQuery).Placeholder("Search models (e.g. flash, gpt, claude, qwen)...").Grow(1)
					})

					// Filtered models
					modelsList := selectedProfile.AvailableModels
					if len(modelsList) == 0 {
						modelsList = selectedProfile.SelectedModels
					}
					var filtered []string
					q := strings.TrimSpace(strings.ToLower(sm.ModelSearchQuery))
					for _, m := range modelsList {
						if q == "" || strings.Contains(strings.ToLower(m), q) {
							filtered = append(filtered, m)
						}
					}

					// Quick actions: select all / deselect all
					saveProfiles := func() {
						if selectedIdx >= 0 && selectedIdx < len(profiles) {
							profiles[selectedIdx] = *selectedProfile
							if err := sm.SaveProfiles(profiles); err != nil {
								c.AddToast(ui.Toast{Title: "Save Failed", Description: err.Error(), Type: "error"})
							}
						}
					}
					ui.Row(c).AlignItems(ui.Center).Children(func() {
						ui.Text(c, fmt.Sprintf("%d models", len(filtered))).FontSize(11).TextColor(tc.TextMuted)
						ui.Box(c).Grow(1)

						selAll := ui.Text(c, "Select all").FontSize(11).FontWeight(500).TextColor(tc.Success)
						if selAll.Clicked() {
							selectedProfile.SelectedModels = append([]string(nil), filtered...)
							saveProfiles()
						}
						ui.Text(c, " · ").FontSize(11).TextColor(tc.TextMuted)
						deselAll := ui.Text(c, "Deselect all").FontSize(11).TextColor(tc.TextMuted)
						if deselAll.Clicked() {
							selectedProfile.SelectedModels = nil
							saveProfiles()
						}
					})

					// Scrollable list
					ui.Scroll(c).Height(180).Children(func() {
						ui.Column(c).Gap(4).Children(func() {
							if len(filtered) == 0 {
								ui.Text(c, "No models found. Click Fetch Models above or type a model name below.").
									FontSize(11).TextColor(tc.TextMuted).Padding(10, 0)
							} else {
								for _, mName := range filtered {
									m := mName
									isSelected := false
									for _, smm := range selectedProfile.SelectedModels {
										if smm == m {
											isSelected = true
											break
										}
									}
									isCurrentActive := a.Agent.ActiveModel == m

									mRow := ui.Row(c.Key("modrow-"+m)).Height(28).Radius(6).Padding(0, 8).Gap(8).AlignItems(ui.Center)
									if isCurrentActive {
										mRow.Background(tc.Success.Alpha(0.08)).BorderWidth(1).BorderColor(tc.Success.Alpha(0.25))
									} else {
										mRow.Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border.Alpha(0.5))
									}

									mRow.Children(func() {
										// Checkbox
										chk := ui.Row(c.Key("chk-"+m)).Size(14, 14).Radius(3).AlignItems(ui.Center).Justify(ui.Center)
										if isSelected {
											chk.Background(tc.Success).Children(func() {
												ui.Icon(c, icons.TablerCheck).Width(10).Height(10).TextColor(tc.SuccessForeground)
											})
										} else {
											chk.BorderWidth(1).BorderColor(tc.Border)
										}
										if chk.Clicked() {
											if isSelected {
												var nsm []string
												for _, smm := range selectedProfile.SelectedModels {
													if smm != m {
														nsm = append(nsm, smm)
													}
												}
												selectedProfile.SelectedModels = nsm
											} else {
												selectedProfile.SelectedModels = append(selectedProfile.SelectedModels, m)
											}
											if selectedIdx >= 0 && selectedIdx < len(profiles) {
												profiles[selectedIdx] = *selectedProfile
												if err := sm.SaveProfiles(profiles); err != nil {
													c.AddToast(ui.Toast{Title: "Save Failed", Description: err.Error(), Type: "error"})
												}
											}
										}

										setActiveModel := func(targetModel string) {
											a.Agent.ActiveModel = targetModel
											a.SelectedAgentModel = targetModel
											if a.Settings != nil {
												a.Settings.Model = targetModel
												a.Settings.Provider = selectedProfile.ID
												a.Settings.BaseURL = selectedProfile.BaseURL
												a.Settings.APIKey = selectedProfile.APIKey
											}
											if a.Engine != nil && a.Engine.LLMClient != nil {
												a.Engine.LLMClient.SetActiveModel(selectedProfile.ID, targetModel)
											}
											if sess := a.Agent.ActiveSession(); sess != nil {
												sess.Model = targetModel
												sess.Provider = selectedProfile.ID
												sess.AgentID = "agent-internal"
												sess.AgentType = "internal"
												a.Agent.SaveSessionToDisk(sess)
											}
											a.Agent.ActiveProvider = selectedProfile.ID
											a.Agent.ActiveAgentID = "agent-internal"
											a.Agent.ActiveAgentType = "internal"
											a.SelectedHarnessID = "agent-internal"
											a.SelectedAgentID = "agent-internal"
											c.AddToast(ui.Toast{Title: "Active Model Set", Description: fmt.Sprintf("%s (%s)", targetModel, selectedProfile.Name), Type: "info"})
											if a.Window != nil {
												a.Window.Invalidate()
											}
										}

										// Model Name
										txt := ui.Text(c, m).FontSize(11).FontWeight(500).Grow(1).SingleLine()
										if isCurrentActive {
											txt.TextColor(tc.Success).FontWeight(600)
										}
										if txt.Clicked() {
											setActiveModel(m)
										}

										// Active Badge / Set Active button
										if isCurrentActive {
											Badge(c, tc, "ACTIVE", tc.Success)
										} else {
											setActBtn := ui.Row(c.Key("setact-"+m)).Radius(4).Padding(1, 6).AlignItems(ui.Center)
											if setActBtn.Hovered() {
												setActBtn.Background(tc.Text.Alpha(0.06))
											}
											setActBtn.Children(func() {
												ui.Text(c, "Set Active").FontSize(10).TextColor(tc.TextMuted)
											})
											if setActBtn.Clicked() {
												setActiveModel(m)
											}
										}

										// Delete from list button
										delMBtn := ui.Row(c.Key("delm-"+m)).Size(16, 16).Radius(4).AlignItems(ui.Center).Justify(ui.Center)
										delMBtn.Children(func() {
											ui.Icon(c, icons.TablerX).Width(10).Height(10).TextColor(tc.IconMuted)
										})
										if delMBtn.Clicked() {
											var nam []string
											for _, am := range selectedProfile.AvailableModels {
												if am != m {
													nam = append(nam, am)
												}
											}
											selectedProfile.AvailableModels = nam
											var nsm []string
											for _, smm := range selectedProfile.SelectedModels {
												if smm != m {
													nsm = append(nsm, smm)
												}
											}
											selectedProfile.SelectedModels = nsm
											if selectedIdx >= 0 && selectedIdx < len(profiles) {
												profiles[selectedIdx] = *selectedProfile
												if err := sm.SaveProfiles(profiles); err != nil {
													c.AddToast(ui.Toast{Title: "Save Failed", Description: err.Error(), Type: "error"})
												}
											}
										}
									})
								}
							}
						})
					})

					// Add Custom Model Row
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.TextInput(c, &sm.NewModelName).Placeholder("Add model ID manually (e.g. qwen-2.5-coder-32b)...").Grow(1)
						addMBtn := ui.Button(c, "Add Model")
						if addMBtn.Clicked() && strings.TrimSpace(sm.NewModelName) != "" {
							mNew := strings.TrimSpace(sm.NewModelName)
							selectedProfile.AvailableModels = append(selectedProfile.AvailableModels, mNew)
							selectedProfile.SelectedModels = append(selectedProfile.SelectedModels, mNew)
							sm.NewModelName = ""
							if selectedIdx >= 0 && selectedIdx < len(profiles) {
								profiles[selectedIdx] = *selectedProfile
								if err := sm.SaveProfiles(profiles); err != nil {
									c.AddToast(ui.Toast{Title: "Save Failed", Description: err.Error(), Type: "error"})
								} else {
									c.AddToast(ui.Toast{Title: "Model Added", Description: mNew, Type: "success"})
								}
							} else {
								c.AddToast(ui.Toast{Title: "Model Added", Description: mNew, Type: "success"})
							}
						}
					})
				})
			} else {
				ui.Column(c).Grow(1).Center().Gap(8).Children(func() {
					ui.Icon(c, icons.TablerPackage).Width(24).Height(24).TextColor(tc.IconMuted)
					ui.Text(c, "No provider selected").FontSize(13).FontWeight(600).TextColor(tc.TextMuted)
					ui.Text(c, "Select a provider from the list or click Add Custom Provider.").FontSize(11).TextColor(tc.TextMuted)
				})
			}
		})
	})

	// Local LLMs & Ollama Status — probed for real against the well-known
	// local endpoints (cached 30s), never a hardcoded ONLINE.
	settingsCard(c, tc, func() {
		ui.Column(c).Gap(6).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "Local LLMs (Ollama / LM Studio)").FontSize(12).FontWeight(600)
				ui.Box(c).Grow(1)
				if a.localLLMOnline() {
					Badge(c, tc, "ONLINE", tc.Success)
				} else {
					Badge(c, tc, "OFFLINE", tc.TextMuted)
				}
			})
			ui.Text(c, "ForgeADE automatically routes local prompts to Ollama (http://localhost:11434) and LM Studio (http://localhost:1234) when active.").
				FontSize(11).TextColor(tc.TextMuted)
		})
	})
}

func (a *AppUI) renderSettingsTabAgents(c *ui.Context, tc theme.ThemeColors, sm *settings.SettingsManager) {
	settingsSectionHeader(c, tc, "ACP & Agent CLIs", "External autonomous agents conforming to the Agent Client Protocol (ACP).")

	if a.ACPMgr == nil {
		return
	}

	ui.Column(c).Gap(8).Children(func() {
		for _, ag := range a.ACPMgr.Agents {
			thisAg := ag
			settingsCard(c, tc, func() {
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					icons.RenderProviderIcon(c, thisAg.Type, 20)
					ui.Column(c).Grow(1).Gap(2).Children(func() {
						ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
							ui.Text(c, thisAg.Name).FontSize(13).FontWeight(600)
							if thisAg.Model != "" {
								ui.Text(c, thisAg.Model).FontSize(10).TextColor(tc.TextMuted)
							}
						})
						ui.Text(c, thisAg.Description).FontSize(11).TextColor(tc.TextMuted)
						if thisAg.Command != "" {
							ui.Text(c, "Command: "+thisAg.Command).FontSize(10).FontWeight(500).TextColor(tc.IconMuted)
						}
					})

					if thisAg.Available {
						Badge(c, tc, "AVAILABLE", tc.Success)
					} else {
						Badge(c, tc, "NOT FOUND", tc.TextMuted)
					}
				})
			})
		}

		ui.Row(c).Padding(6, 0).Children(func() {
			scanBtn := ui.Button(c, "Scan System PATH for Agents")
			if scanBtn.Clicked() {
				go func() {
					// Probe off-thread, then apply by index on the UI thread —
					// range copies would drop the writes, and publishing via
					// runOnUI avoids racing concurrent render reads.
					type probe struct {
						idx int
						ok  bool
					}
					agents := a.ACPMgr.Agents
					results := make([]probe, 0, len(agents))
					for i, ag := range agents {
						if ag.ID == "agent-internal" || ag.Type == "internal" || strings.TrimSpace(ag.Command) == "" {
							continue // built-in engine: availability is not PATH-derived
						}
						_, err := exec.LookPath(ag.Command)
						results = append(results, probe{idx: i, ok: err == nil})
					}
					a.runOnUI(func() {
						for _, r := range results {
							if r.idx >= 0 && r.idx < len(a.ACPMgr.Agents) {
								a.ACPMgr.Agents[r.idx].Available = r.ok
							}
						}
						a.invalidateUI()
					})
				}()
				c.AddToast(ui.Toast{Title: "Scanning", Description: "Probed PATH for installed ACP binaries", Type: "info"})
			}
		})
	})
}

func (a *AppUI) renderSettingsTabGeneral(c *ui.Context, tc theme.ThemeColors, sm *settings.SettingsManager) {
	settingsSectionHeader(c, tc, "General Workspace", "Workspace preferences, file persistence, and notifications.")

	settingsCard(c, tc, func() {
		ui.Column(c).Gap(10).Children(func() {
			ui.Text(c, "Workspace Root Directory").FontSize(12).FontWeight(600)
			ui.Text(c, a.Workspace.RootDir).FontSize(11).TextColor(tc.TextMuted)

			ui.Row(c).Gap(8).Children(func() {
				if ui.Button(c, "Choose Folder...").Clicked() {
					a.openWorkspaceFolder()
				}
			})
		})
	})

	settingsCard(c, tc, func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).Gap(2).Children(func() {
				ui.Text(c, "Auto Save Files").FontSize(13).FontWeight(600)
				ui.Text(c, "Automatically persist modified buffers to disk every 30 seconds.").FontSize(11).TextColor(tc.TextMuted)
			})
			ui.Switch(c, &sm.AutoSave)
		})
	})
}

func (a *AppUI) renderSettingsTabAppearance(c *ui.Context, tc theme.ThemeColors, sm *settings.SettingsManager) {
	settingsSectionHeader(c, tc, "Appearance & Themes", "Customize UI color palettes, dark/light theme, font sizes, and layout lane width.")
	tm := a.ThemeMgr

	// 1. Dark / Light Toggle
	settingsCard(c, tc, func() {
		ui.Column(c).Gap(8).Children(func() {
			ui.Text(c, "APPEARANCE MODE").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted)
			ui.Row(c).Gap(8).Children(func() {
				darkBtn := func() ui.Element {
					if tm.Appearance == "dark" {
						return ui.PrimaryButton(c, "")
					}
					return ui.Button(c, "")
				}()
				darkBtn.Children(func() {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, icons.TablerMoon).Width(13).Height(13)
						ui.Text(c, "Dark Mode").FontSize(12).FontWeight(600)
					})
				})
				if darkBtn.Clicked() {
					tm.Appearance = "dark"
				}

				lightBtn := func() ui.Element {
					if tm.Appearance == "light" {
						return ui.PrimaryButton(c, "")
					}
					return ui.Button(c, "")
				}()
				lightBtn.Children(func() {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, icons.TablerSun).Width(13).Height(13)
						ui.Text(c, "Light Mode").FontSize(12).FontWeight(600)
					})
				})
				if lightBtn.Clicked() {
					tm.Appearance = "light"
				}
			})
		})
	})

	// 2. Palette Swatches
	settingsCard(c, tc, func() {
		ui.Column(c).Gap(8).Children(func() {
			ui.Text(c, fmt.Sprintf("COLOR PALETTE (%d BUILT-IN PACKS)", len(tm.AvailablePalettes()))).FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted)
			ui.Row(c).Gap(8).Wrap().Children(func() {
				for _, p := range tm.AvailablePalettes() {
					thisP := p
					isSel := (tm.PaletteID == thisP.ID)
					card := ui.Row(c.Key("pal-card-"+thisP.ID)).Radius(8).Padding(8, 12).Gap(8).AlignItems(ui.Center)
					card.Children(func() {
						ui.Box(c).Size(12, 12).Radius(6).Background(theme.SwatchColor(thisP.SwatchHex))
						ui.Text(c, thisP.Name).FontSize(12).FontWeight(func() int {
							if isSel {
								return 600
							}
							return 400
						}())
						if isSel {
							ui.Icon(c, icons.TablerCheck).Width(13).Height(13).TextColor(tc.Accent)
						}
					})
					if isSel {
						card.Background(tc.Accent.Alpha(0.12)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.3))
					} else {
						card.Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
						if card.Hovered() {
							card.Background(tc.Surface.Alpha(0.8))
						}
					}
					if card.Clicked() {
						tm.PaletteID = thisP.ID
					}
				}
			})
		})
	})

	// 3. Chat Lane Width & Diff Color Scheme
	settingsCard(c, tc, func() {
		ui.Row(c).Gap(24).Children(func() {
			ui.Column(c).Grow(1).Gap(6).Children(func() {
				ui.Text(c, "Chat Lane Width").FontSize(12).FontWeight(600)
				ui.Row(c).Gap(6).Children(func() {
					for _, opt := range []struct{ ID, Label string }{
						{"comfortable", "Comfortable (736px)"},
						{"wide", "Wide (928px)"},
						{"full", "Full"},
					} {
						thisOpt := opt
						btn := func() ui.Element {
							if tm.ChatWidth == thisOpt.ID {
								return ui.PrimaryButton(c, thisOpt.Label).FontSize(11)
							}
							return ui.Button(c, thisOpt.Label).FontSize(11)
						}()
						if btn.Clicked() {
							tm.ChatWidth = thisOpt.ID
						}
					}
				})
			})

			ui.Column(c).Grow(1).Gap(6).Children(func() {
				ui.Text(c, "Diff Scheme").FontSize(12).FontWeight(600)
				ui.Row(c).Gap(6).Children(func() {
					for _, opt := range []struct{ ID, Label string }{
						{"classic", "Classic (Green/Red)"},
						{"modern", "Modern (Blue/Orange)"},
					} {
						thisOpt := opt
						btn := func() ui.Element {
							if tm.DiffScheme == thisOpt.ID {
								return ui.PrimaryButton(c, thisOpt.Label).FontSize(11)
							}
							return ui.Button(c, thisOpt.Label).FontSize(11)
						}()
						if btn.Clicked() {
							tm.DiffScheme = thisOpt.ID
						}
					}
				})
			})
		})
	})
}

func (a *AppUI) renderSettingsTabShortcuts(c *ui.Context, tc theme.ThemeColors, sm *settings.SettingsManager) {
	settingsSectionHeader(c, tc, "Terminal & Shortcuts", "Shell configuration and keybinding shortcuts reference.")

	settingsCard(c, tc, func() {
		ui.Column(c).Gap(6).Children(func() {
			ui.Text(c, "Default Shell").FontSize(12).FontWeight(600)
			ui.Select(c, &sm.DefaultShell, []string{"/bin/zsh", "/bin/bash", "/bin/sh"})
		})
	})

	shortcuts := []struct{ Key, Desc string }{
		{"⌘S", "Save current active buffer to disk"},
		{"⌘P", "Quick open file by fuzzy name"},
		{"⇧⌘P", "Open command palette"},
		{"⌘M", "Select AI Model & Harness"},
		{"⌘1", "Switch to Autonomous Agent Mode"},
		{"⌘2", "Switch to Code Editor Mode"},
		{"⌘B", "Toggle left sidebar"},
		{"⌘J", "Toggle right drawer"},
		{"⌘`", "Toggle collapsible bottom terminal"},
		{"⌘N", "Start a new task session"},
		{"⌘,", "Open Settings"},
		{"⌘/", "Toggle Dark / Light appearance"},
		{"⌘W", "Close active editor tab"},
		{"Ctrl+Tab", "Next editor tab"},
		{"⇧Ctrl+Tab", "Previous editor tab"},
	}

	settingsCard(c, tc, func() {
		ui.Column(c).Gap(6).Children(func() {
			ui.Text(c, "KEYBOARD SHORTCUTS REFERENCE").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted)
			for _, sc := range shortcuts {
				s := sc
				ui.Row(c.Key("sc-"+s.Key)).Height(26).Padding(0, 8).AlignItems(ui.Center).Children(func() {
					kbd := ui.Row(c).Radius(4).Padding(2, 6).Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
					kbd.Children(func() {
						ui.Text(c, s.Key).FontSize(11).FontWeight(600).TextColor(tc.Accent)
					})
					ui.Box(c).Width(12)
					ui.Text(c, s.Desc).FontSize(12).TextColor(tc.Text)
				})
			}
		})
	})
}

// localLLMOnline reports whether a local LLM server (Ollama or LM Studio)
// answers on its well-known port. Probed off the render path with a 30s
// cache; until the first probe returns it shows the previous result.
func (a *AppUI) localLLMOnline() bool {
	if time.Since(a.localLLMCheckedAt) < 30*time.Second {
		return a.localLLMOnlineFlag
	}
	a.localLLMCheckedAt = time.Now()
	go func() {
		online := probeLocalEndpoint("http://localhost:11434/api/tags") ||
			probeLocalEndpoint("http://localhost:1234/v1/models")
		a.runOnUI(func() {
			a.localLLMOnlineFlag = online
			a.invalidateUI()
		})
	}()
	return a.localLLMOnlineFlag
}

// probeLocalEndpoint performs a short HEAD/GET reachability check.
func probeLocalEndpoint(url string) bool {
	client := &http.Client{Timeout: 1200 * time.Millisecond}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode < 500
}

func settingsSectionHeader(c *ui.Context, tc theme.ThemeColors, title, desc string) {
	ui.Column(c).Gap(3).Children(func() {
		ui.Text(c, title).FontSize(18).FontWeight(700)
		ui.Text(c, desc).FontSize(12).TextColor(tc.TextMuted)
	})
}

func settingsCard(c *ui.Context, tc theme.ThemeColors, children func()) {
	ui.Column(c).Padding(14, 16).Radius(10).Background(tc.Surface.Alpha(0.5)).
		BorderWidth(1).BorderColor(tc.Border).Children(children)
}
