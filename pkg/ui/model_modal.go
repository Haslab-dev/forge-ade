package ui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/agent"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/icons"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/settings"
)

// ModelMetadata describes a model option for selection.
type ModelMetadata struct {
	ID            string
	DisplayName   string
	Provider      string
	ProviderKey   string
	HarnessID     string
	HarnessName   string
	ContextWindow string
	IsLocal       bool
}

// Built-in Internal Engine Models. Left empty so only configured providers and models are used.
var defaultInternalModels = []ModelMetadata{}

// renderModelProviderPaletteModal renders ForgeADE's full ModelProviderPaletteModal dialog.
func (a *AppUI) renderModelProviderPaletteModal(c *ui.Context) {
	if !a.ShowModelPalette {
		return
	}
	tc := a.ThemeMgr.Colors()

	if a.SelectedHarnessID == "" {
		a.SelectedHarnessID = "agent-internal"
	}
	if a.SelectedProviderTab == "" {
		a.SelectedProviderTab = "all"
	}

	if c.Shortcut(0, ui.KeyEscape) {
		a.ShowModelPalette = false
		return
	}

	// Harness definitions
	type harnessDef struct {
		ID        string
		Name      string
		Icon      string
		IsCurrent bool
	}
	harnesses := []harnessDef{
		{ID: "agent-internal", Name: "ForgeADE Internal", Icon: "forge-ade", IsCurrent: a.SelectedHarnessID == "agent-internal"},
	}
	if a.ACPMgr != nil {
		for _, ag := range a.ACPMgr.Agents {
			if ag.ID != "agent-internal" {
				harnesses = append(harnesses, harnessDef{
					ID:        ag.ID,
					Name:      ag.Name,
					Icon:      ag.Type,
					IsCurrent: a.SelectedHarnessID == ag.ID,
				})
			}
		}
	}

	// Filter models by search or selected harness & provider
	search := strings.TrimSpace(strings.ToLower(a.ModelSearchQuery))
	var visibleModels []ModelMetadata
	isSearching := search != ""

	// Build configured internal models
	var internalModels []ModelMetadata
	if a.Settings != nil && a.Settings.Model != "" {
		provName := a.Settings.Provider
		if provName == "" {
			provName = "Configured"
		}
		internalModels = append(internalModels, ModelMetadata{
			ID:            a.Settings.Model,
			DisplayName:   a.Settings.Model,
			Provider:      provName,
			ProviderKey:   a.Settings.Provider,
			HarnessID:     "agent-internal",
			HarnessName:   "ForgeADE Internal",
			ContextWindow: "Configured",
		})
	}
	if a.Agent != nil {
		for _, m := range a.Agent.AvailableModels {
			internalModels = append(internalModels, ModelMetadata{
				ID:            m,
				DisplayName:   m,
				Provider:      "custom",
				ProviderKey:   "custom",
				HarnessID:     "agent-internal",
				HarnessName:   "ForgeADE Internal",
				ContextWindow: "Custom",
			})
		}
	}
	for _, m := range defaultInternalModels {
		internalModels = append(internalModels, m)
	}

	// If internal models are empty and user hasn't explicitly picked a harness, default to first ACP harness
	if len(internalModels) == 0 && (a.SelectedHarnessID == "" || a.SelectedHarnessID == "agent-internal") && len(harnesses) > 1 {
		a.SelectedHarnessID = harnesses[1].ID
		for i := range harnesses {
			harnesses[i].IsCurrent = (harnesses[i].ID == a.SelectedHarnessID)
		}
	}

	if isSearching {
		for _, m := range internalModels {
			if strings.Contains(strings.ToLower(m.DisplayName), search) ||
				strings.Contains(strings.ToLower(m.ID), search) ||
				strings.Contains(strings.ToLower(m.Provider), search) {
				visibleModels = append(visibleModels, m)
			}
		}
		if a.ACPMgr != nil {
			for _, ag := range a.ACPMgr.Agents {
				if !ag.Available {
					continue
				}
				for _, sm := range ag.SupportedModels {
					if strings.Contains(strings.ToLower(sm), search) {
						visibleModels = append(visibleModels, ModelMetadata{
							ID:            sm,
							DisplayName:   sm,
							Provider:      ag.Name,
							ProviderKey:   ag.Type,
							HarnessID:     ag.ID,
							HarnessName:   ag.Name,
							ContextWindow: "ACP",
						})
					}
				}
			}
		}
	} else if a.SelectedHarnessID == "agent-internal" {
		for _, m := range internalModels {
			if a.SelectedProviderTab == "all" || strings.EqualFold(m.Provider, a.SelectedProviderTab) {
				visibleModels = append(visibleModels, m)
			}
		}
	} else {
		// Selected an external ACP agent
		if a.ACPMgr != nil {
			for _, ag := range a.ACPMgr.Agents {
				if ag.ID == a.SelectedHarnessID {
					for _, sm := range ag.SupportedModels {
						visibleModels = append(visibleModels, ModelMetadata{
							ID:            sm,
							DisplayName:   sm,
							Provider:      ag.Name,
							ProviderKey:   ag.Type,
							HarnessID:     ag.ID,
							HarnessName:   ag.Name,
							ContextWindow: "ACP",
						})
					}
					if len(ag.SupportedModels) == 0 && ag.Model != "" {
						visibleModels = append(visibleModels, ModelMetadata{
							ID:            ag.Model,
							DisplayName:   ag.Model,
							Provider:      ag.Name,
							ProviderKey:   ag.Type,
							HarnessID:     ag.ID,
							HarnessName:   ag.Name,
							ContextWindow: "ACP",
						})
					}
					break
				}
			}
		}
	}

	selectModel := func(modelID, displayName, harnessID string) {
		a.Agent.ActiveModel = displayName
		a.SelectedAgentModel = displayName
		if harnessID != "" {
			a.SelectedHarnessID = harnessID
		}
		if sess := a.Agent.ActiveSession(); sess != nil {
			sess.Model = displayName
			if harnessID != "" {
				sess.AgentID = harnessID
				if harnessID != "agent-internal" {
					sess.AgentType = "acp"
				}
			}
		}
		a.ShowModelPalette = false
		c.AddToast(ui.Toast{
			Title:       "Model Selected",
			Description: fmt.Sprintf("%s · %s", harnessID, displayName),
			Type:        "info",
		})
		if a.Window != nil {
			a.Window.Invalidate()
		}
	}

	ui.Modal(c, &a.ShowModelPalette, func() {
		// 540px popup card matching ForgeADE ModelProviderPaletteModal metrics
		ui.Column(c).Width(540).Height(450).Radius(16).
			Background(tc.SurfaceOverlay).BorderWidth(1).BorderColor(tc.Border).
			Shadow(0, 24, 64, -20, ui.RGBA(0, 0, 0, 0.55)).Children(func() {

			// 1. Search Bar Header
			ui.Row(c).Height(44).Padding(0, 12).Gap(8).AlignItems(ui.Center).
				BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
				ui.Icon(c, icons.TablerSearch).Width(15).Height(15).TextColor(tc.IconMuted)
				in := ui.TextInput(c, &a.ModelSearchQuery).Placeholder("Search models, providers, or harnesses...").Grow(1).AutoFocus()
				if in.Submitted() && len(visibleModels) > 0 {
					selectModel(visibleModels[0].ID, visibleModels[0].DisplayName, visibleModels[0].HarnessID)
				}

				if a.ModelSearchQuery != "" {
					clearBtn := ui.Row(c).Size(20, 20).Radius(10).AlignItems(ui.Center).Justify(ui.Center)
					clearBtn.Children(func() {
						ui.Icon(c, icons.TablerX).Width(12).Height(12).TextColor(tc.IconMuted)
					})
					if clearBtn.Clicked() {
						a.ModelSearchQuery = ""
					}
				}

				// Providers button to jump to settings
				provBtn := ui.Row(c).Height(24).Radius(6).Padding(0, 8).Gap(4).AlignItems(ui.Center).
					Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
				provBtn.Children(func() {
					ui.Icon(c, icons.TablerSettings).Width(12).Height(12).TextColor(tc.IconMuted)
					ui.Text(c, "Providers").FontSize(11).FontWeight(500)
				})
				if provBtn.Clicked() {
					a.ShowModelPalette = false
					a.Settings.ActiveTab = settings.SettingsTabModels
					a.Settings.IsOpen = true
				}
			})

			// 2. Main Two-Column Split Body
			ui.Row(c).Grow(1).FillWidth().Children(func() {
				// Left Rail: Harnesses List (150px)
				ui.Column(c).Width(150).FillHeight().Background(tc.Surface.Alpha(0.35)).
					BorderWidth(0, 1, 0, 0).BorderColor(tc.Border).Padding(8, 6).Gap(2).Children(func() {
					ui.Row(c).Padding(2, 6, 6, 6).Children(func() {
						ui.Text(c, "HARNESSES").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.SidebarMutedForeground)
					})

					for _, h := range harnesses {
						thisH := h
						isSel := (a.SelectedHarnessID == thisH.ID)
						btn := ui.Row(c.Key("harness-"+thisH.ID)).Height(28).Radius(8).Padding(0, 8).Gap(6).AlignItems(ui.Center)
						btn.Children(func() {
							icons.RenderProviderIcon(c, thisH.Icon, 13)
							ui.Text(c, thisH.Name).FontSize(12).FontWeight(func() int {
								if isSel {
									return 600
								}
								return 400
							}()).Grow(1).SingleLine()

							if thisH.IsCurrent {
								ui.Box(c).Size(5, 5).Radius(3).Background(tc.Accent)
							}
						})
						if isSel {
							btn.Background(tc.Accent.Alpha(0.12))
						} else if btn.Hovered() {
							btn.Background(tc.Text.Alpha(0.04))
						}
						if btn.Clicked() {
							a.SelectedHarnessID = thisH.ID
							a.SelectedProviderTab = "all"
							a.ModelSearchQuery = ""
						}
					}
				})

				// Right Column: Models for active harness or search results
				ui.Column(c).Grow(1).FillHeight().Padding(8, 10).Children(func() {
					if !isSearching && a.SelectedHarnessID == "agent-internal" && len(internalModels) > 0 {
						// Provider Filter Pills Row
						ui.Row(c).Gap(4).Padding(0, 0, 6, 0).Wrap().Children(func() {
							pills := []struct {
								ID    string
								Label string
								Icon  string
							}{
								{"all", "All", ""},
								{"Anthropic", "Anthropic", "claude"},
								{"OpenAI", "OpenAI", "openai"},
								{"Google", "Gemini", "gemini"},
								{"DeepSeek", "DeepSeek", "deepseek"},
								{"Ollama", "Ollama", "ollama"},
							}
							for _, pill := range pills {
								thisPill := pill
								isPillActive := (a.SelectedProviderTab == thisPill.ID)
								pBtn := ui.Row(c.Key("prov-pill-"+thisPill.ID)).Height(22).Radius(6).Padding(0, 7).Gap(4).AlignItems(ui.Center)
								pBtn.Children(func() {
									if thisPill.Icon != "" {
										icons.RenderProviderIcon(c, thisPill.Icon, 11)
									}
									ui.Text(c, thisPill.Label).FontSize(11).FontWeight(func() int {
										if isPillActive {
											return 600
										}
										return 400
									}())
								})
								if isPillActive {
									pBtn.Background(tc.Text).TextColor(tc.Canvas)
								} else {
									pBtn.Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
									if pBtn.Hovered() {
										pBtn.Background(tc.Surface.Alpha(0.8))
									}
								}
								if pBtn.Clicked() {
									a.SelectedProviderTab = thisPill.ID
								}
							}
						})
					}

					// Scrollable Models List
					ui.Scroll(c).Grow(1).Children(func() {
						ui.Column(c).Gap(2).Children(func() {
							if len(visibleModels) == 0 {
								ui.Column(c).Grow(1).Padding(48, 16).Center().Gap(10).Children(func() {
									if isSearching {
										ui.Text(c, fmt.Sprintf("No models found matching \"%s\"", a.ModelSearchQuery)).FontSize(12).TextColor(tc.TextMuted)
										customBtn := ui.PrimaryButton(c, fmt.Sprintf("+ Use custom \"%s\"", a.ModelSearchQuery)).FontSize(12)
										if customBtn.Clicked() {
											selectModel(a.ModelSearchQuery, a.ModelSearchQuery, a.SelectedHarnessID)
										}
									} else {
										ui.Icon(c, icons.TablerCpu).Width(32).Height(32).TextColor(tc.IconMuted)
										ui.Text(c, "No providers or models configured").FontSize(13).FontWeight(600).TextColor(tc.Text)
										ui.Text(c, "Configure API keys or local LLM endpoints in Settings.").
											FontSize(11).TextColor(tc.TextMuted)
										cfgBtn := ui.PrimaryButton(c, "Configure Providers").FontSize(12)
										if cfgBtn.Clicked() {
											a.ShowModelPalette = false
											if a.Settings != nil {
												a.Settings.ActiveTab = settings.SettingsTabModels
												a.Settings.IsOpen = true
											}
										}
									}
								})
							}

							for _, model := range visibleModels {
								m := model
								isModelSelected := (a.Agent.ActiveModel == m.DisplayName || strings.HasPrefix(a.Agent.ActiveModel, m.DisplayName))
								row := ui.Row(c.Key("mod-row-"+m.ID)).Height(30).Radius(8).Padding(0, 8).Gap(8).AlignItems(ui.Center)
								row.Children(func() {
									icons.RenderProviderIcon(c, m.ProviderKey, 14)
									ui.Text(c, m.DisplayName).FontSize(12).FontWeight(func() int {
										if isModelSelected {
											return 600
										}
										return 400
									}()).Grow(1).SingleLine()

									if m.IsLocal {
										badge := ui.Row(c).Radius(4).Padding(1, 5).Background(tc.Success.Alpha(0.15))
										badge.Children(func() {
											ui.Text(c, "Local").FontSize(9).FontWeight(600).TextColor(tc.Success)
										})
									}

									if m.ContextWindow != "" {
										ui.Text(c, m.ContextWindow).FontSize(10).TextColor(tc.TextMuted)
									}

									if isModelSelected {
										ui.Icon(c, icons.TablerCheck).Width(13).Height(13).TextColor(tc.Accent)
									}
								})

								if isModelSelected {
									row.Background(tc.Accent.Alpha(0.12)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.3))
								} else if row.Hovered() {
									row.Background(tc.Text.Alpha(0.04))
								}
								if row.Clicked() {
									selectModel(m.ID, m.DisplayName, m.HarnessID)
								}
							}
						})
					})
				})
			})

			// 3. Bottom Footer: Local LLM Status + Segmented Reasoning Effort Control
			ui.Row(c).Height(44).Padding(0, 12).AlignItems(ui.Center).
				Background(tc.Surface.Alpha(0.4)).BorderWidth(1, 0, 0, 0).BorderColor(tc.Border).Children(func() {

				// Local LLM & ACP Status
				ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Size(6, 6).Radius(3).Background(tc.Success)
					ui.Text(c, "ACP Agents & Local LLMs ready").FontSize(11).TextColor(tc.TextMuted)
				})

				ui.Box(c).Grow(1)

				// Segmented Reasoning Effort Control (Off, Low, Med, High)
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "Reasoning").FontSize(11).FontWeight(500).TextColor(tc.TextMuted)
					efforts := []struct {
						ID    agent.ReasoningEffort
						Label string
					}{
						{agent.EffortOff, "Off"},
						{agent.EffortLow, "Low"},
						{agent.EffortMedium, "Med"},
						{agent.EffortHigh, "High"},
					}
					seg := ui.Row(c).Height(24).Radius(6).Padding(2).Gap(2).Background(tc.Surface).
						BorderWidth(1).BorderColor(tc.Border).AlignItems(ui.Center)
					seg.Children(func() {
						for _, ef := range efforts {
							thisEf := ef
							isEfActive := (a.Agent.ReasoningEffort == thisEf.ID)
							btn := ui.Row(c.Key("effort-"+string(thisEf.ID))).Height(20).Radius(4).Padding(0, 6).
								AlignItems(ui.Center).Justify(ui.Center)
							btn.Children(func() {
								ui.Text(c, thisEf.Label).FontSize(10).FontWeight(func() int {
									if isEfActive {
										return 600
									}
									return 400
								}())
							})
							if isEfActive {
								btn.Background(tc.Text).TextColor(tc.Canvas)
							} else if btn.Hovered() {
								btn.Background(tc.Text.Alpha(0.04))
							}
							if btn.Clicked() {
								a.Agent.ReasoningEffort = thisEf.ID
								c.AddToast(ui.Toast{
									Title:       "Reasoning Effort",
									Description: fmt.Sprintf("Set reasoning effort to %s", thisEf.Label),
									Type:        "info",
								})
							}
						}
					})
				})
			})
		})
	})
}
