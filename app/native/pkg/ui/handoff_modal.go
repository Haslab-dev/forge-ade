package ui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/agent"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/settings"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
	"github.com/hasdev/forge-ade/core/quota"
)

type resolvedHandoffPersona struct {
	ID           string
	Name         string
	ShortName    string
	Role         string
	AgentID      string
	AgentType    string
	Model        string
	SystemPrompt string
	ReadOnly     bool
}

func modalOpaqueSurface(tc theme.ThemeColors) ui.Color {
	return tc.Surface.Alpha(0.97)
}

func (a *AppUI) modelsForHarness(agentID string) []string {
	targetID := strings.TrimSpace(agentID)
	if targetID == "" && a.Agent != nil {
		if sess := a.Agent.ActiveSession(); sess != nil && sess.AgentID != "" {
			targetID = sess.AgentID
		} else if a.Agent.ActiveAgentID != "" {
			targetID = a.Agent.ActiveAgentID
		}
	}
	if targetID == "" {
		targetID = "agent-internal"
	}

	seen := make(map[string]bool)
	var out []string
	add := func(m string) {
		m = strings.TrimSpace(m)
		if m != "" && !seen[m] {
			seen[m] = true
			out = append(out, m)
		}
	}

	if targetID == "agent-internal" {
		if a.Engine != nil && a.Engine.LLMClient != nil {
			add(a.Engine.LLMClient.GetConfig().Model)
		}
		if a.Settings != nil {
			add(a.Settings.Model)
		}
		if a.Agent != nil {
			add(a.Agent.ActiveModel)
			for _, m := range a.Agent.AvailableModels {
				add(m)
			}
		}
		return out
	}

	if a.ACPMgr != nil {
		for _, ag := range a.ACPMgr.Agents {
			if ag.ID != targetID {
				continue
			}
			for _, m := range ag.SupportedModels {
				add(m)
			}
			if ag.ID == "agent-agy" {
				a.ensureAgyModelsFetchedAsync()
				if a.Settings != nil {
					if acctMgr := a.Settings.AntigravityAccountMgr(); acctMgr != nil {
						if activeAcc := acctMgr.ActiveAccount(); activeAcc != nil {
							for _, m := range activeAcc.AvailableModels {
								add(m)
							}
							if len(out) == 0 && activeAcc.SupportsArgon() {
								for _, m := range quota.DefaultArgonModels() {
									add(m)
								}
							}
						}
					}
				}
			}
			add(ag.Model)
			break
		}
	}
	return out
}

func shortPersonaPillLabel(name string) string {
	s := strings.TrimSpace(strings.TrimSuffix(name, " Agent"))
	if s == "" {
		return name
	}
	return s
}

func sanitizeGoalTitle(raw string) string {
	s := strings.TrimSpace(raw)
	if idx := strings.Index(s, "\n"); idx >= 0 {
		s = strings.TrimSpace(s[:idx])
	}
	if strings.HasPrefix(s, "Attached files:") {
		return ""
	}
	if len(s) > 140 {
		s = strings.TrimSpace(s[:140]) + "…"
	}
	return s
}

func (a *AppUI) handoffPersonas() []resolvedHandoffPersona {
	var out []resolvedHandoffPersona
	if a.Engine != nil && a.Engine.AgentMgr != nil {
		for _, def := range a.Engine.AgentMgr.ListAgentDefinitions() {
			var hb settings.PersonaHarnessBinding
			if a.Settings != nil {
				hb = a.Settings.GetPersonaHarness(def.ID)
			}
			prompt := strings.TrimSpace(def.Prompt)
			if rules := strings.TrimSpace(def.Rules); rules != "" {
				if prompt != "" {
					prompt += "\n" + rules
				} else {
					prompt = rules
				}
			}
			if def.ReadOnly {
				if prompt != "" {
					prompt += "\n"
				}
				prompt += "Read-only: do NOT edit, create, or delete project files."
			}
			out = append(out, resolvedHandoffPersona{
				ID:           def.ID,
				Name:         def.Name,
				ShortName:    shortPersonaPillLabel(def.Name),
				Role:         string(def.Role),
				AgentID:      hb.AgentID,
				AgentType:    hb.AgentType,
				Model:        def.Model,
				SystemPrompt: prompt,
				ReadOnly:     def.ReadOnly,
			})
		}
	}
	return out
}

func (a *AppUI) applyHandoffPersona(p resolvedHandoffPersona) {
	a.HandoffPersonaID = p.ID
	a.HandoffPersonaName = p.Name
	a.HandoffPersonaPrompt = p.SystemPrompt
	a.HandoffTargetAgentID = p.AgentID
	a.HandoffTargetAgentType = p.AgentType
	a.HandoffTargetModel = p.Model
	a.HandoffEditAgentID = p.AgentID
	a.HandoffEditAgentType = p.AgentType
	a.HandoffEditModel = p.Model
	a.HandoffEditPrompt = p.SystemPrompt
}

// OpenHandoffModal prepares and opens the Persona-driven Session/ACP Handoff dialog,
// reading personas directly from Agent Personas.
func (a *AppUI) OpenHandoffModal() {
	if a.Agent == nil {
		return
	}
	sess := a.Agent.ActiveSession()
	if sess != nil {
		a.HandoffSourceID = sess.ID
		a.HandoffGoal = sanitizeGoalTitle(sess.Title)
	} else {
		a.HandoffSourceID = ""
		a.HandoffGoal = ""
	}
	a.HandoffNextSteps = ""
	a.HandoffCreateWorktree = false
	a.HandoffWorktreeBranch = ""
	a.HandoffAutoStart = true
	a.HandoffConfigOpen = false
	a.HandoffError = ""

	personas := a.handoffPersonas()
	if len(personas) > 0 {
		selected := personas[0]
		for _, p := range personas {
			if p.ID == a.HandoffPersonaID {
				selected = p
				break
			}
		}
		a.applyHandoffPersona(selected)
	}
	a.ShowHandoffModal = true
}

func (a *AppUI) resolveHandoffTargetLabel(agentID, model string) string {
	name := "Active Session (Auto)"
	if agentID != "" {
		if agentID == "agent-internal" {
			name = "ForgeADE Internal"
		} else {
			name = agentID
			if a.ACPMgr != nil {
				for _, def := range a.ACPMgr.Agents {
					if def.ID == agentID {
						name = def.Name
						break
					}
				}
			}
		}
	} else if a.Agent != nil {
		if sess := a.Agent.ActiveSession(); sess != nil && sess.AgentID != "" {
			return a.resolveHandoffTargetLabel(sess.AgentID, func() string {
				if model != "" {
					return model
				}
				return sess.Model
			}())
		}
	}
	if strings.TrimSpace(model) != "" {
		return name + " · " + strings.TrimSpace(model)
	}
	return name + " · Inherit model"
}

func (a *AppUI) renderHandoffModal(c *ui.Context) {
	if !a.ShowHandoffModal {
		return
	}
	tc := a.ThemeMgr.Colors()
	personas := a.handoffPersonas()

	type agentChoice struct {
		ID        string
		Name      string
		Type      string
		Model     string
		Available bool
	}
	choices := []agentChoice{
		{ID: "", Name: "Auto (Inherit)", Type: "", Available: true},
		{ID: "agent-internal", Name: "Internal", Type: "internal", Available: true},
	}
	if a.ACPMgr != nil {
		for _, def := range a.ACPMgr.Agents {
			if def.Type == "internal" || def.ID == "agent-internal" {
				continue
			}
			choices = append(choices, agentChoice{
				ID:        def.ID,
				Name:      shortHarnessLabel(def.Name),
				Type:      "acp",
				Model:     def.Model,
				Available: def.Available,
			})
		}
	}

	a.modal(c, &a.ShowHandoffModal, func() {
		modal := ui.Column(c).Width(540).MaxHeight(640).Radius(14).
			Background(modalOpaqueSurface(tc)).BorderWidth(1).BorderColor(tc.Border).
			Shadow(0, 20, 56, -16, ui.RGBA(0, 0, 0, 0.65)).Clip()

		modal.Children(func() {
			// Header
			ui.Row(c).Padding(14, 18).Gap(10).AlignItems(ui.Center).
				BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
				ui.Icon(c, icons.TablerGitFork).Width(16).Height(16).TextColor(tc.Accent)
				ui.Column(c).Grow(1).Gap(1).Children(func() {
					ui.Text(c, "Hand off session").FontSize(14).FontWeight(600).TextColor(tc.Text)
					ui.Text(c, "Pick a persona — harness & model auto-apply").FontSize(11).TextColor(tc.TextMuted)
				})
				closeBtn := ui.Row(c.Key("ho-close")).Size(24, 24).Radius(6).AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
				closeBtn.Children(func() {
					ui.Icon(c, icons.TablerX).Width(13).Height(13).TextColor(tc.IconMuted)
				})
				if closeBtn.Hovered() {
					closeBtn.Background(tc.Text.Alpha(0.08))
				}
				if closeBtn.Clicked() {
					a.ShowHandoffModal = false
				}
			})

			// Body
			ui.Scroll(c.Key("ho-modal-scroll")).Grow(1).Padding(16, 18).Children(func() {
				ui.Column(c).Gap(12).Children(func() {
					// 1. Persona selector
					ui.Column(c).Gap(6).Children(func() {
						ui.Row(c).AlignItems(ui.Center).Children(func() {
							ui.Text(c, "Persona").FontSize(11.5).FontWeight(600).TextColor(tc.TextMuted).Grow(1)
							mgrLink := ui.Row(c.Key("ho-open-orch-settings")).Radius(5).Padding(2, 6).Cursor(ui.CursorPointer)
							mgrLink.Children(func() {
								ui.Text(c, "Manage Personas →").FontSize(10.5).FontWeight(500).TextColor(tc.Accent)
							})
							if mgrLink.Hovered() {
								mgrLink.Background(tc.Accent.Alpha(0.1))
							}
							if mgrLink.Clicked() {
								a.ShowHandoffModal = false
								a.openSettings(settings.SettingsTabSubagents)
							}
						})
						ui.Row(c).Gap(6).Wrap().Children(func() {
							for _, pr := range personas {
								p := pr
								selected := a.HandoffPersonaID == p.ID
								pill := ui.Row(c.Key("ho-pr-"+p.ID)).Height(28).Radius(7).Padding(0, 10).Gap(5).
									AlignItems(ui.Center).Cursor(ui.CursorPointer).BorderWidth(1)
								if selected {
									pill.Background(tc.Accent.Alpha(0.18)).BorderColor(tc.Accent.Alpha(0.6))
								} else {
									pill.Background(tc.SurfaceRaised).BorderColor(tc.Border)
								}
								pill.Children(func() {
									ui.Icon(c, icons.TablerBot).Width(12).Height(12).TextColor(func() ui.Color {
										if selected {
											return tc.Accent
										}
										return tc.IconMuted
									}())
									txt := ui.Text(c, p.ShortName).FontSize(11.5).SingleLine()
									if selected {
										txt.FontWeight(600).TextColor(tc.Text)
									} else {
										txt.FontWeight(500).TextColor(tc.TextMuted)
									}
								})
								if pill.Hovered() && !selected {
									pill.Background(tc.ToolbarControlHover)
								}
								if pill.Clicked() {
									a.applyHandoffPersona(p)
								}
							}
						})

						// Resolved harness + model summary + Configure button
						targetDesc := a.resolveHandoffTargetLabel(a.HandoffTargetAgentID, a.HandoffTargetModel)
						ui.Row(c).Padding(7, 10).Gap(8).Radius(7).AlignItems(ui.Center).
							Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border).Children(func() {
							ui.Icon(c, icons.TablerSparkles).Width(12).Height(12).TextColor(tc.Accent)
							ui.Text(c, targetDesc).FontSize(11).FontWeight(500).TextColor(tc.Text).Grow(1).SingleLine()
							cfgBtn := ui.Row(c.Key("ho-cfg-toggle")).Height(22).Radius(5).Padding(0, 8).Gap(4).
								AlignItems(ui.Center).Cursor(ui.CursorPointer).
								Background(tc.ToolbarControl).BorderWidth(1).BorderColor(tc.Border)
							cfgBtn.Children(func() {
								ui.Icon(c, icons.TablerSettings).Width(11).Height(11).TextColor(tc.TextMuted)
								label := "Configure"
								if a.HandoffConfigOpen {
									label = "Close"
								}
								ui.Text(c, label).FontSize(10.5).FontWeight(500).TextColor(tc.Text)
							})
							if cfgBtn.Clicked() {
								a.HandoffConfigOpen = !a.HandoffConfigOpen
							}
						})
					})

					// Optional inline Persona Config editor (1-click Harness + Model pills)
					if a.HandoffConfigOpen && a.HandoffPersonaID != "" {
						editModels := a.modelsForHarness(a.HandoffEditAgentID)
						ui.Column(c).Padding(10, 12).Gap(8).Radius(8).
							Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.4)).Children(func() {
							ui.Text(c, fmt.Sprintf("Default Harness & Model for %s", a.HandoffPersonaName)).FontSize(11.5).FontWeight(600).TextColor(tc.Text)
							ui.Row(c).Gap(5).Wrap().Children(func() {
								for _, ch := range choices {
									opt := ch
									if !opt.Available && opt.ID != "" {
										continue
									}
									sel := a.HandoffEditAgentID == opt.ID
									pBtn := ui.Row(c.Key("ho-ed-ag-"+opt.ID)).Height(24).Radius(6).Padding(0, 8).
										AlignItems(ui.Center).Cursor(ui.CursorPointer).BorderWidth(1)
									if sel {
										pBtn.Background(tc.Accent.Alpha(0.18)).BorderColor(tc.Accent.Alpha(0.55))
									} else {
										pBtn.Background(tc.Surface).BorderColor(tc.Border)
									}
									pBtn.Children(func() {
										ui.Text(c, opt.Name).FontSize(11).TextColor(tc.Text)
									})
									if pBtn.Clicked() {
										a.HandoffEditAgentID = opt.ID
										a.HandoffEditAgentType = opt.Type
									}
								}
							})

							ui.Text(c, "Default Model").FontSize(10.5).FontWeight(600).TextColor(tc.TextMuted)
							ui.Row(c).Gap(5).Wrap().Children(func() {
								mChoices := append([]string{""}, editModels...)
								for _, mID := range mChoices {
									m := mID
									label := m
									if label == "" {
										label = "Inherit / Auto"
									}
									sel := strings.TrimSpace(a.HandoffEditModel) == m
									mBtn := ui.Row(c.Key("ho-ed-m-"+m)).Height(24).Radius(6).Padding(0, 8).
										AlignItems(ui.Center).Cursor(ui.CursorPointer).BorderWidth(1)
									if sel {
										mBtn.Background(tc.Accent.Alpha(0.18)).BorderColor(tc.Accent.Alpha(0.55))
									} else {
										mBtn.Background(tc.Surface).BorderColor(tc.Border)
									}
									mBtn.Children(func() {
										ui.Text(c, label).FontSize(10.5).TextColor(tc.Text)
									})
									if mBtn.Clicked() {
										a.HandoffEditModel = m
									}
								}
							})

							ui.Row(c).Justify(ui.End).Children(func() {
								saveBtn := ui.Row(c.Key("ho-ed-save")).Height(24).Radius(6).Padding(0, 10).
									AlignItems(ui.Center).Cursor(ui.CursorPointer).Background(tc.Accent)
								saveBtn.Children(func() {
									ui.Text(c, "Save Preset").FontSize(11).FontWeight(600).TextColor(tc.AccentForeground)
								})
								if saveBtn.Clicked() {
									if a.Settings != nil {
										a.Settings.SetPersonaHarness(a.HandoffPersonaID, a.HandoffEditAgentID, a.HandoffEditAgentType)
									}
									if a.Engine != nil && a.Engine.AgentMgr != nil {
										if def, ok := a.Engine.AgentMgr.FindDefinition(a.HandoffPersonaID); ok {
											def.Model = strings.TrimSpace(a.HandoffEditModel)
											_, _ = a.Engine.AgentMgr.SaveAgentDefinition(def)
										}
									}
									a.HandoffTargetAgentID = a.HandoffEditAgentID
									a.HandoffTargetAgentType = a.HandoffEditAgentType
									a.HandoffTargetModel = strings.TrimSpace(a.HandoffEditModel)
									a.HandoffConfigOpen = false
								}
							})
						})
					}

					// 2. Goal input
					ui.Column(c).Gap(4).Children(func() {
						ui.Text(c, "Goal / Done means").FontSize(11.5).FontWeight(600).TextColor(tc.TextMuted)
						ui.TextAreaBase(c, &a.HandoffGoal).Lines(2, 3).Placeholder("What should the receiving persona accomplish?").FillWidth()
					})

					// 3. Next instructions input
					ui.Column(c).Gap(4).Children(func() {
						ui.Text(c, "Next Instructions (optional)").FontSize(11.5).FontWeight(600).TextColor(tc.TextMuted)
						ui.TextAreaBase(c, &a.HandoffNextSteps).Lines(2, 4).Placeholder("Specific files, test commands, or constraints...").FillWidth()
					})

					// 4. Options
					ui.Column(c).Gap(6).Children(func() {
						wtToggle := ui.Row(c.Key("ho-wt-toggle")).Height(28).Radius(7).Padding(0, 8).Gap(8).
							AlignItems(ui.Center).Cursor(ui.CursorPointer).
							Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border)
						wtToggle.Children(func() {
							box := ui.Box(c).Size(14, 14).Radius(4).AlignItems(ui.Center).Justify(ui.Center).BorderWidth(1)
							if a.HandoffCreateWorktree {
								box.Background(tc.Accent).BorderColor(tc.Accent).Children(func() {
									ui.Icon(c, icons.TablerCheck).Width(10).Height(10).TextColor(tc.AccentForeground)
								})
							} else {
								box.BorderColor(tc.Border)
							}
							ui.Text(c, "Create isolated git worktree").FontSize(12).FontWeight(500).TextColor(tc.Text).Grow(1)
						})
						if wtToggle.Clicked() {
							a.HandoffCreateWorktree = !a.HandoffCreateWorktree
						}
						if a.HandoffCreateWorktree {
							ui.TextInput(c, &a.HandoffWorktreeBranch).Placeholder("Branch name (e.g. feature/handoff-review)").FillWidth()
						}

						autoToggle := ui.Row(c.Key("ho-auto-toggle")).Height(28).Radius(7).Padding(0, 8).Gap(8).
							AlignItems(ui.Center).Cursor(ui.CursorPointer).
							Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border)
						autoToggle.Children(func() {
							box := ui.Box(c).Size(14, 14).Radius(4).AlignItems(ui.Center).Justify(ui.Center).BorderWidth(1)
							if a.HandoffAutoStart {
								box.Background(tc.Accent).BorderColor(tc.Accent).Children(func() {
									ui.Icon(c, icons.TablerCheck).Width(10).Height(10).TextColor(tc.AccentForeground)
								})
							} else {
								box.BorderColor(tc.Border)
							}
							ui.Text(c, "Start running immediately (auto-send prompt)").FontSize(12).FontWeight(500).TextColor(tc.Text).Grow(1)
						})
						if autoToggle.Clicked() {
							a.HandoffAutoStart = !a.HandoffAutoStart
						}
					})

					if a.HandoffError != "" {
						ui.Text(c, a.HandoffError).FontSize(11.5).TextColor(tc.Error)
					}
				})
			})

			// Footer actions
			ui.Row(c).Padding(12, 18).Gap(8).Justify(ui.End).AlignItems(ui.Center).
				BorderWidth(1, 0, 0, 0).BorderColor(tc.Border).Children(func() {
				cancelBtn := ui.Row(c.Key("ho-cancel")).Height(30).Radius(7).Padding(0, 12).
					AlignItems(ui.Center).Cursor(ui.CursorPointer).
					Background(tc.ToolbarControl).BorderWidth(1).BorderColor(tc.Border)
				cancelBtn.Children(func() {
					ui.Text(c, "Cancel").FontSize(12).FontWeight(500).TextColor(tc.Text)
				})
				if cancelBtn.Clicked() {
					a.ShowHandoffModal = false
				}

				goBtn := ui.Row(c.Key("ho-submit")).Height(30).Radius(7).Padding(0, 14).Gap(6).
					AlignItems(ui.Center).Cursor(ui.CursorPointer).
					Background(tc.MessageAction)
				goBtn.Children(func() {
					ui.Text(c, "Hand off").FontSize(12).FontWeight(600).TextColor(tc.MessageActionForeground)
					ui.Icon(c, icons.TablerExternalLink).Width(12).Height(12).TextColor(tc.MessageActionForeground)
				})
				if goBtn.Clicked() {
					sess, err := a.Agent.ExecuteHandoff(agent.HandoffRequest{
						SourceSessionID:     a.HandoffSourceID,
						TargetAgentID:       a.HandoffTargetAgentID,
						TargetAgentType:     a.HandoffTargetAgentType,
						TargetModel:         strings.TrimSpace(a.HandoffTargetModel),
						PersonaName:         a.HandoffPersonaName,
						PersonaInstructions: a.HandoffPersonaPrompt,
						Goal:                strings.TrimSpace(a.HandoffGoal),
						NextSteps:           strings.TrimSpace(a.HandoffNextSteps),
						CreateWorktree:      a.HandoffCreateWorktree,
						WorktreeBranch:      strings.TrimSpace(a.HandoffWorktreeBranch),
						AutoStart:           a.HandoffAutoStart,
					})
					if err != nil {
						a.HandoffError = fmt.Sprintf("Handoff failed: %v", err)
					} else {
						a.ShowHandoffModal = false
						if sess != nil {
							a.ensureAgentTab(sess.ID)
							if sess.WorkspacePath != "" && a.Workspace != nil && !sameWorkspacePath(a.Workspace.RootDir, sess.WorkspacePath) {
								a.switchWorkspace(sess.WorkspacePath)
							}
						}
						toastDesc := "Prepared " + sess.Title + " (review before running)"
						if a.HandoffAutoStart {
							toastDesc = "Started " + sess.Title
						}
						c.AddToast(ui.Toast{Title: "Session Handed Off", Description: toastDesc, Type: "success"})
					}
				}
			})
		})
	})
}
