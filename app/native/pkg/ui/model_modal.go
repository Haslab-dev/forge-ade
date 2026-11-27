package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/Haslab-dev/RakitKode/llm"
	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/agent"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/settings"
	"github.com/hasdev/forge-ade/core/quota"
)

// ModelMetadata describes a model option for selection.
type ModelMetadata struct {
	ID            string
	DisplayName   string
	Provider      string
	ProviderID    string
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
	seenInternal := make(map[string]bool)
	addInternalModel := func(m ModelMetadata) {
		key := fmt.Sprintf("%s::%s", m.ProviderID, m.ID)
		if m.ProviderID == "" {
			key = fmt.Sprintf("%s::%s", m.Provider, m.ID)
		}
		if !seenInternal[key] {
			seenInternal[key] = true
			internalModels = append(internalModels, m)
		}
	}

	// 1. Load models from all Custom Provider Profiles configured in Settings (e.g. Kenari, Zhipu, Ollama, OpenAI, Anthropic)
	if a.Settings != nil {
		for _, prof := range a.Settings.GetProfiles() {
			if !prof.Enabled && len(a.Settings.GetProfiles()) > 1 {
				continue
			}
			proto := "openai"
			if strings.Contains(strings.ToLower(prof.BaseURL), "anthropic") || strings.EqualFold(prof.ID, "anthropic") {
				proto = "anthropic"
			}
			provName := prof.Name
			if provName == "" {
				provName = prof.ID
			}
			// Load both selected models and available models from this profile
			for _, mName := range prof.SelectedModels {
				addInternalModel(ModelMetadata{
					ID:            mName,
					DisplayName:   mName,
					Provider:      provName,
					ProviderID:    prof.ID,
					ProviderKey:   proto,
					HarnessID:     "agent-internal",
					HarnessName:   "ForgeADE Internal",
					ContextWindow: provName,
				})
			}
			for _, mName := range prof.AvailableModels {
				addInternalModel(ModelMetadata{
					ID:            mName,
					DisplayName:   mName,
					Provider:      provName,
					ProviderID:    prof.ID,
					ProviderKey:   proto,
					HarnessID:     "agent-internal",
					HarnessName:   "ForgeADE Internal",
					ContextWindow: provName,
				})
			}
		}
	}

	// 2. Active configured model in Settings
	if a.Settings != nil && a.Settings.Model != "" {
		provName := a.Settings.Provider
		if provName == "" {
			provName = "Configured"
		}
		provID := a.Settings.SelectedProviderID
		if provID == "" {
			provID = a.Settings.Provider
		}
		addInternalModel(ModelMetadata{
			ID:            a.Settings.Model,
			DisplayName:   a.Settings.Model,
			Provider:      provName,
			ProviderID:    provID,
			ProviderKey:   a.Settings.Provider,
			HarnessID:     "agent-internal",
			HarnessName:   "ForgeADE Internal",
			ContextWindow: "Configured",
		})
	}

	// 3. Engine LLMClient active config model
	if a.Engine != nil && a.Engine.LLMClient != nil {
		cfg := a.Engine.LLMClient.GetConfig()
		if cfg.Model != "" {
			addInternalModel(ModelMetadata{
				ID:            cfg.Model,
				DisplayName:   cfg.Model,
				Provider:      cfg.ProviderID,
				ProviderID:    cfg.ProviderID,
				ProviderKey:   cfg.ProviderID,
				HarnessID:     "agent-internal",
				HarnessName:   "ForgeADE Internal",
				ContextWindow: "Active",
			})
		}
	}

	// 4. Agent available models
	if a.Agent != nil {
		for _, m := range a.Agent.AvailableModels {
			addInternalModel(ModelMetadata{
				ID:            m,
				DisplayName:   m,
				Provider:      "custom",
				ProviderID:    "custom",
				ProviderKey:   "custom",
				HarnessID:     "agent-internal",
				HarnessName:   "ForgeADE Internal",
				ContextWindow: "Custom",
			})
		}
	}
	for _, m := range defaultInternalModels {
		addInternalModel(m)
	}

	// If internal models are empty and user hasn't explicitly picked a harness, default to first ACP harness
	if len(internalModels) == 0 && (a.SelectedHarnessID == "" || a.SelectedHarnessID == "agent-internal") && len(harnesses) > 1 {
		a.SelectedHarnessID = harnesses[1].ID
		for i := range harnesses {
			harnesses[i].IsCurrent = (harnesses[i].ID == a.SelectedHarnessID)
		}
	}

	getAgentModels := func(ag agent.ACPAgentDefinition) []string {
		models := ag.SupportedModels
		if ag.ID == "agent-agy" {
			a.ensureAgyModelsFetchedAsync()
			if a.Settings != nil {
				if acctMgr := a.Settings.AntigravityAccountMgr(); acctMgr != nil {
					if activeAcc := acctMgr.ActiveAccount(); activeAcc != nil {
						if len(activeAcc.AvailableModels) > len(models) {
							models = activeAcc.AvailableModels
						} else if activeAcc.SupportsArgon() && len(quota.DefaultArgonModels()) > len(models) {
							models = quota.DefaultArgonModels()
						}
					}
				}
			}
		}
		return models
	}

	if isSearching {
		for _, m := range internalModels {
			if strings.Contains(strings.ToLower(m.DisplayName), search) ||
				strings.Contains(strings.ToLower(m.ID), search) ||
				strings.Contains(strings.ToLower(m.Provider), search) ||
				strings.Contains(strings.ToLower(m.ProviderID), search) {
				visibleModels = append(visibleModels, m)
			}
		}
		if a.ACPMgr != nil {
			for _, ag := range a.ACPMgr.Agents {
				if !ag.Available {
					continue
				}
				for _, sm := range getAgentModels(ag) {
					if strings.Contains(strings.ToLower(sm), search) ||
						strings.Contains(strings.ToLower(ag.Name), search) {
						visibleModels = append(visibleModels, ModelMetadata{
							ID:            sm,
							DisplayName:   sm,
							Provider:      ag.Name,
							ProviderID:    ag.ID,
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
		tabLower := strings.ToLower(a.SelectedProviderTab)
		for _, m := range internalModels {
			mProv := strings.ToLower(m.Provider)
			mPID := strings.ToLower(m.ProviderID)
			mKey := strings.ToLower(m.ProviderKey)
			mID := strings.ToLower(m.ID)
			matches := tabLower == "all" ||
				strings.EqualFold(m.Provider, a.SelectedProviderTab) ||
				strings.EqualFold(m.ProviderID, a.SelectedProviderTab) ||
				strings.EqualFold(m.ProviderKey, a.SelectedProviderTab) ||
				(tabLower == "anthropic" && (strings.Contains(mID, "claude") || strings.Contains(mProv, "anthropic") || strings.Contains(mPID, "anthropic"))) ||
				(tabLower == "openai" && (strings.Contains(mID, "gpt") || strings.Contains(mKey, "openai") || strings.Contains(mProv, "openai") || strings.Contains(mPID, "openai"))) ||
				(tabLower == "google" || tabLower == "gemini") && (strings.Contains(mID, "gemini") || strings.Contains(mProv, "gemini") || strings.Contains(mProv, "google") || strings.Contains(mPID, "google")) ||
				(tabLower == "deepseek" && (strings.Contains(mID, "deepseek") || strings.Contains(mProv, "deepseek") || strings.Contains(mPID, "deepseek"))) ||
				(tabLower == "ollama" && (strings.Contains(mProv, "ollama") || strings.Contains(mPID, "ollama") || m.IsLocal))
			if matches {
				visibleModels = append(visibleModels, m)
			}
		}
	} else {
		// Selected an external ACP agent
		if a.ACPMgr != nil {
			for _, ag := range a.ACPMgr.Agents {
				if ag.ID == a.SelectedHarnessID {
					models := getAgentModels(ag)
					for _, sm := range models {
						visibleModels = append(visibleModels, ModelMetadata{
							ID:            sm,
							DisplayName:   sm,
							Provider:      ag.Name,
							ProviderID:    ag.ID,
							ProviderKey:   ag.Type,
							HarnessID:     ag.ID,
							HarnessName:   ag.Name,
							ContextWindow: "ACP",
						})
					}
					if len(models) == 0 && ag.Model != "" {
						visibleModels = append(visibleModels, ModelMetadata{
							ID:            ag.Model,
							DisplayName:   ag.Model,
							Provider:      ag.Name,
							ProviderID:    ag.ID,
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

	selectModel := func(m ModelMetadata) {
		prevModel := a.Agent.ActiveModel
		a.Agent.ActiveModel = m.DisplayName
		a.SelectedAgentModel = m.DisplayName
		if m.HarnessID != "" {
			a.SelectedHarnessID = m.HarnessID
			a.SelectedAgentID = m.HarnessID
			a.Agent.ActiveAgentID = m.HarnessID
			if m.HarnessID == "agent-internal" {
				a.Agent.ActiveAgentType = "internal"
			} else {
				a.Agent.ActiveAgentType = "acp"
			}
		}
		if sess := a.Agent.ActiveSession(); sess != nil {
			oldSessModel := sess.Model
			if oldSessModel == "" {
				oldSessModel = prevModel
			}
			sess.Model = m.DisplayName
			if m.HarnessID != "" {
				sess.AgentID = m.HarnessID
				if m.HarnessID != "agent-internal" {
					sess.AgentType = "acp"
				} else {
					sess.AgentType = "internal"
				}
			}
			if m.HarnessID == "agent-internal" {
				if m.ProviderID != "" {
					sess.Provider = m.ProviderID
				} else if m.Provider != "" {
					sess.Provider = m.Provider
				}
			}
			if oldSessModel != "" && oldSessModel != m.DisplayName {
				a.Agent.RecordModelChange(sess.ID, oldSessModel, m.DisplayName)
			} else {
				a.Agent.SaveSessionToDisk(sess)
			}
		}
		if m.HarnessID == "agent-internal" {
			if m.ProviderID != "" {
				a.Agent.ActiveProvider = m.ProviderID
			} else if m.Provider != "" {
				a.Agent.ActiveProvider = m.Provider
			}
			if a.Settings != nil {
				a.Settings.Model = m.DisplayName
				// Sync provider profile if found
				var targetProf *llm.ProviderProfile
				for _, prof := range a.Settings.GetProfiles() {
					if m.ProviderID != "" && prof.ID == m.ProviderID {
						p := prof
						targetProf = &p
						break
					}
					if prof.Name == m.Provider || prof.ID == m.Provider {
						p := prof
						targetProf = &p
						break
					}
				}
				if targetProf == nil {
					// Fallback to model match if ProviderID wasn't set
					for _, prof := range a.Settings.GetProfiles() {
						matched := false
						for _, am := range prof.AvailableModels {
							if am == m.DisplayName {
								matched = true
								break
							}
						}
						if !matched {
							for _, sm := range prof.SelectedModels {
								if sm == m.DisplayName {
									matched = true
									break
								}
							}
						}
						if matched {
							p := prof
							targetProf = &p
							break
						}
					}
				}
				if targetProf != nil {
					a.Settings.SelectedProviderID = targetProf.ID
					a.Settings.Provider = targetProf.ID
					a.Settings.BaseURL = targetProf.BaseURL
					a.Settings.APIKey = targetProf.APIKey
					if a.Engine != nil && a.Engine.LLMClient != nil {
						a.Engine.LLMClient.SetActiveModel(targetProf.ID, m.DisplayName)
					}
				} else if a.Engine != nil && a.Engine.LLMClient != nil {
					prov := a.Engine.LLMClient.FindProviderForModel(m.DisplayName)
					if prov != "" {
						a.Engine.LLMClient.SetActiveModel(prov, m.DisplayName)
					} else {
						a.Engine.LLMClient.SetActiveModelByID(m.DisplayName)
					}
				}
			} else if a.Engine != nil && a.Engine.LLMClient != nil {
				if m.ProviderID != "" {
					a.Engine.LLMClient.SetActiveModel(m.ProviderID, m.DisplayName)
				} else {
					a.Engine.LLMClient.SetActiveModelByID(m.DisplayName)
				}
			}
		}
		a.ShowModelPalette = false
		c.AddToast(ui.Toast{
			Title:       "Model Selected",
			Description: fmt.Sprintf("%s · %s (%s)", m.HarnessName, m.DisplayName, m.Provider),
			Type:        "info",
		})
		if a.Window != nil {
			a.Window.Invalidate()
		}
	}

	a.modal(c, &a.ShowModelPalette, func() {
		// 580px popup card matching ForgeADE ModelProviderPaletteModal metrics
		ui.Column(c).Width(580).Height(480).Radius(16).
			Background(tc.SurfaceOverlay).BorderWidth(1).BorderColor(tc.Border).
			Shadow(0, 24, 64, -20, ui.RGBA(0, 0, 0, 0.55)).Clip().Children(func() {

			// 1. Search Bar Header
			ui.Row(c).Height(44).Padding(0, 12).Gap(8).AlignItems(ui.Center).
				BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
				ui.Icon(c, icons.TablerSearch).Width(15).Height(15).TextColor(tc.IconMuted)
				in := ui.TextInput(c, &a.ModelSearchQuery).Placeholder("Search models, providers, or harnesses...").Grow(1).AutoFocus()
				if in.Submitted() && len(visibleModels) > 0 {
					selectModel(visibleModels[0])
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
					a.openSettings(settings.SettingsTabModels)
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
							if thisH.ID != "agent-internal" && a.Agent != nil {
								model := a.Agent.SelectACPAgent(thisH.ID)
								a.SelectedAgentID = thisH.ID
								if model != "" {
									a.SelectedAgentModel = model
								}
								if sess := a.Agent.ActiveSession(); sess != nil {
									sess.AgentID = thisH.ID
									sess.AgentType = "acp"
									if model != "" {
										sess.Model = model
									}
									a.Agent.SaveSessionToDisk(sess)
								}
							}
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
							if a.Settings != nil {
								for _, prof := range a.Settings.GetProfiles() {
									pName := prof.Name
									if pName == "" {
										pName = prof.ID
									}
									dup := false
									for _, p := range pills {
										if strings.EqualFold(p.Label, pName) || strings.EqualFold(p.ID, prof.ID) {
											dup = true
											break
										}
									}
									if !dup && (len(prof.AvailableModels) > 0 || len(prof.SelectedModels) > 0) {
										proto := "openai"
										if strings.Contains(strings.ToLower(prof.BaseURL), "anthropic") || strings.EqualFold(prof.ID, "anthropic") {
											proto = "anthropic"
										}
										pills = append(pills, struct {
											ID    string
											Label string
											Icon  string
										}{prof.ID, pName, proto})
									}
								}
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

					// Group models by Provider
					type modelGroup struct {
						Key         string
						Title       string
						Icon        string
						HarnessName string
						Models      []ModelMetadata
					}

					var modelGroups []modelGroup
					grpMap := make(map[string]int)

					for _, m := range visibleModels {
						grpKey := m.Provider
						if grpKey == "" {
							grpKey = m.ProviderID
						}
						if grpKey == "" {
							grpKey = "Other"
						}
						if isSearching && m.HarnessID != "agent-internal" {
							grpKey = m.HarnessName
						}

						idx, exists := grpMap[grpKey]
						if !exists {
							idx = len(modelGroups)
							grpMap[grpKey] = idx
							icon := m.ProviderKey
							if icon == "" {
								icon = "openai"
							}
							modelGroups = append(modelGroups, modelGroup{
								Key:         grpKey,
								Title:       grpKey,
								Icon:        icon,
								HarnessName: m.HarnessName,
								Models:      nil,
							})
						}
						modelGroups[idx].Models = append(modelGroups[idx].Models, m)
					}

					// Scrollable Models List Grouped by Provider
					ui.Scroll(c).Grow(1).Children(func() {
						ui.Column(c).Gap(6).Children(func() {
							if len(visibleModels) == 0 {
								ui.Column(c).Grow(1).Padding(48, 16).Center().Gap(10).Children(func() {
									if isSearching {
										ui.Text(c, fmt.Sprintf("No models found matching \"%s\"", a.ModelSearchQuery)).FontSize(12).TextColor(tc.TextMuted)
										customBtn := ui.PrimaryButton(c, fmt.Sprintf("+ Use custom \"%s\"", a.ModelSearchQuery)).FontSize(12)
										if customBtn.Clicked() {
											selectModel(ModelMetadata{
												ID:          a.ModelSearchQuery,
												DisplayName: a.ModelSearchQuery,
												HarnessID:   a.SelectedHarnessID,
												HarnessName: "Custom",
												Provider:    "Custom",
											})
										}
									} else {
										ui.Icon(c, icons.TablerCpu).Width(32).Height(32).TextColor(tc.IconMuted)
										ui.Text(c, "No providers or models configured").FontSize(13).FontWeight(600).TextColor(tc.Text)
										ui.Text(c, "Configure API keys or local LLM endpoints in Settings.").
											FontSize(11).TextColor(tc.TextMuted)
										cfgBtn := ui.PrimaryButton(c, "Configure Providers").FontSize(12)
										if cfgBtn.Clicked() {
											a.ShowModelPalette = false
											a.openSettings(settings.SettingsTabModels)
										}
									}
								})
								return
							}

							for _, grp := range modelGroups {
								thisGrp := grp
								ui.Column(c.Key("grp-col-" + thisGrp.Key)).Gap(2).Children(func() {
									// Section Header for Provider
									grpHeader := ui.Row(c.Key("grp-hdr-"+thisGrp.Key)).Height(24).Padding(6, 6, 2, 6).Gap(6).AlignItems(ui.Center)
									grpHeader.Children(func() {
										icons.RenderProviderIcon(c, thisGrp.Icon, 12)
										ui.Text(c, strings.ToUpper(thisGrp.Title)).FontSize(10).FontWeight(700).LetterSpacing(0.6).TextColor(tc.TextMuted)
										ui.Box(c).Grow(1)
										countStr := fmt.Sprintf("%d", len(thisGrp.Models))
										if len(thisGrp.Models) == 1 {
											countStr += " model"
										} else {
											countStr += " models"
										}
										ui.Text(c, countStr).FontSize(9).FontWeight(500).TextColor(tc.TextMuted.Alpha(0.6))
									})

									// Rows for each model in this provider group
									for _, model := range thisGrp.Models {
										m := model
										active := a.Agent.ActiveModel
										isModelSelected := false
										if a.SelectedHarnessID == m.HarnessID {
											if m.HarnessID == "agent-internal" {
												isModelSelected = active != "" &&
													(active == m.DisplayName || strings.HasPrefix(active, m.DisplayName+" "))
												if a.Settings != nil && a.Settings.SelectedProviderID != "" && m.ProviderID != "" {
													isModelSelected = isModelSelected && (m.ProviderID == a.Settings.SelectedProviderID || m.Provider == a.Settings.Provider)
												}
											} else {
												isModelSelected = (a.SelectedAgentModel == m.DisplayName || active == m.DisplayName)
											}
										}

										rowKey := fmt.Sprintf("mod-row-%s-%s-%s", m.HarnessID, m.ProviderID, m.ID)
										row := ui.Row(c.Key(rowKey)).Height(30).Radius(8).Padding(0, 8).Gap(8).AlignItems(ui.Center)
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

											if m.Provider != "" {
												ui.Text(c, m.Provider).FontSize(10).TextColor(tc.TextMuted)
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
											selectModel(m)
										}
									}
								})
							}
						})
					})
				})
			})

			// 3. Bottom Footer: Local LLM Status + Segmented Reasoning Effort Control
			ui.Row(c).Height(44).Padding(0, 12).AlignItems(ui.Center).
				Background(tc.Surface.Alpha(0.4)).BorderWidth(1, 0, 0, 0).BorderColor(tc.Border).Children(func() {

				// Local LLM & ACP Status — reflects real agent availability
				// rather than an unconditional green dot.
				ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
					acpReady := false
					if a.ACPMgr != nil {
						for _, ag := range a.ACPMgr.Agents {
							if ag.ID != "agent-internal" && ag.Available {
								acpReady = true
								break
							}
						}
					}
					if acpReady {
						ui.Box(c).Size(6, 6).Radius(3).Background(tc.Success)
						ui.Text(c, "ACP Agents & Local LLMs ready").FontSize(11).TextColor(tc.TextMuted)
					} else {
						ui.Box(c).Size(6, 6).Radius(3).Background(tc.TextMuted.Alpha(0.5))
						ui.Text(c, "No external ACP agents detected — internal engine active").FontSize(11).TextColor(tc.TextMuted)
					}
				})

				ui.Box(c).Grow(1)

				// Segmented Reasoning Effort Control — hidden for Antigravity/AGY
				// where effort (-low / -medium / -high / -xhigh) is already part of each model ID.
				if modelEncodesEffort(a.SelectedHarnessID, "") {
					ui.Text(c, "Effort encoded in model").FontSize(11).TextColor(tc.TextMuted)
				} else {
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
				}
			})
		})
	})
}

func (a *AppUI) ensureAgyModelsFetchedAsync() {
	if a.isFetchingAntigravityModels || a.agyModelsFetched || a.ACPMgr == nil {
		return
	}
	a.isFetchingAntigravityModels = true
	go func() {
		models, err := quota.FetchAgyCLIModels(context.Background())
		a.runOnUI(func() {
			a.isFetchingAntigravityModels = false
			a.agyModelsFetched = true
			if err == nil && len(models) > 0 {
				a.ACPMgr.UpdateAgentSupportedModels("agent-agy", models)
				a.invalidateUI()
			}
		})
	}()
}
