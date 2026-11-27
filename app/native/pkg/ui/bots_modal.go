package ui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/agent"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
)

// OpenNewBotModal opens the Local ForgeADE Bot creation dialog.
func (a *AppUI) OpenNewBotModal() {
	a.NewBotTemplateID = ""
	a.NewBotName = ""
	a.NewBotRole = ""
	a.NewBotPrompt = ""
	a.NewBotAvatar = defaultBotAvatarPreset
	if a.NewBotAgentID == "" {
		a.NewBotAgentID = "agent-internal"
		a.NewBotAgentType = "internal"
	}
	a.ShowNewBotModal = true
}

// OpenNewBotTaskModal opens the background task launcher for botID (RakitKode or ACP).
func (a *AppUI) OpenNewBotTaskModal(botID string) {
	a.NewBotTaskBotID = botID
	a.NewBotTaskTitle = ""
	a.NewBotTaskPrompt = ""
	a.NewBotTaskWorktree = false
	a.NewBotTaskError = ""
	if a.Bots != nil {
		if b := a.Bots.Get(botID); b != nil && b.AgentID != "" {
			a.NewBotTaskAgentID = b.AgentID
			a.NewBotTaskAgentType = b.AgentType
		}
	}
	if a.NewBotTaskAgentID == "" {
		a.NewBotTaskAgentID = "agent-internal"
		a.NewBotTaskAgentType = "internal"
	}
	a.ShowNewBotTaskModal = true
}

// OpenSpawnWorkerModal opens the Hybrid Worker (Internal or ACP) launch dialog.
func (a *AppUI) OpenSpawnWorkerModal() {
	a.WorkerNameInput = ""
	a.WorkerTaskInput = ""
	a.WorkerUseWorktree = false
	a.WorkerError = ""
	if a.WorkerAgentID == "" {
		a.WorkerAgentID = "agent-internal"
		a.WorkerAgentType = "internal"
	}
	a.ShowSpawnWorkerModal = true
}

func (a *AppUI) renderNewBotModal(c *ui.Context) {
	if !a.ShowNewBotModal {
		return
	}
	tc := a.ThemeMgr.Colors()

	type botAgentOpt struct {
		ID   string
		Name string
		Type string
	}
	opts := []botAgentOpt{
		{ID: "agent-internal", Name: "ForgeADE Internal", Type: "internal"},
	}
	if a.ACPMgr != nil {
		for _, def := range a.ACPMgr.Agents {
			if def.Type == "internal" || def.ID == "agent-internal" {
				continue
			}
			opts = append(opts, botAgentOpt{ID: def.ID, Name: def.Name, Type: "acp"})
		}
	}

	a.modal(c, &a.ShowNewBotModal, func() {
		modal := ui.Column(c).Width(460).Radius(14).
			Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).
			Shadow(0, 20, 56, -16, ui.RGBA(0, 0, 0, 0.55)).Clip()
		modal.Children(func() {
			ui.Row(c).Padding(14, 18).Gap(10).AlignItems(ui.Center).
				BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
				ui.Icon(c, icons.TablerBot).Width(16).Height(16).TextColor(tc.Accent)
				ui.Column(c).Grow(1).Gap(1).Children(func() {
					ui.Text(c, "New Local Bot").FontSize(14).FontWeight(600).TextColor(tc.Text)
					ui.Text(c, "Persistent persona with Home Chat & autonomous handoff").FontSize(11).TextColor(tc.TextMuted)
				})
				xBtn := ui.Row(c.Key("nb-close")).Size(24, 24).Radius(6).AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
				xBtn.Children(func() {
					ui.Icon(c, icons.TablerX).Width(13).Height(13).TextColor(tc.IconMuted)
				})
				if xBtn.Hovered() {
					xBtn.Background(tc.Text.Alpha(0.08))
				}
				if xBtn.Clicked() {
					a.ShowNewBotModal = false
				}
			})

			ui.Column(c).Padding(16, 18).Gap(11).Children(func() {
				// Template picker (Blank or built-in preset)
				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, "Template").FontSize(11.5).FontWeight(600).TextColor(tc.TextMuted)
					ui.Row(c).Gap(6).Wrap().Children(func() {
						blankSel := a.NewBotTemplateID == ""
						blankPill := ui.Row(c.Key("nb-tpl-blank")).Height(26).Radius(6).Padding(0, 9).
							AlignItems(ui.Center).Cursor(ui.CursorPointer).BorderWidth(1)
						if blankSel {
							blankPill.Background(tc.Accent.Alpha(0.16)).BorderColor(tc.Accent.Alpha(0.55))
						} else {
							blankPill.Background(tc.SurfaceRaised).BorderColor(tc.Border)
						}
						blankPill.Children(func() {
							ui.Text(c, "Custom").FontSize(11).FontWeight(500).TextColor(tc.Text)
						})
						if blankPill.Clicked() {
							a.NewBotTemplateID = ""
						}
						for _, tpl := range agent.DefaultBotTemplates() {
							t := tpl
							sel := a.NewBotTemplateID == t.ID
							pill := ui.Row(c.Key("nb-tpl-"+t.ID)).Height(26).Radius(6).Padding(0, 9).
								AlignItems(ui.Center).Cursor(ui.CursorPointer).BorderWidth(1)
							if sel {
								pill.Background(tc.Accent.Alpha(0.16)).BorderColor(tc.Accent.Alpha(0.55))
							} else {
								pill.Background(tc.SurfaceRaised).BorderColor(tc.Border)
							}
							pill.Children(func() {
								ui.Text(c, t.Title).FontSize(11).FontWeight(500).TextColor(tc.Text)
							})
							if pill.Clicked() {
								a.NewBotTemplateID = t.ID
								a.NewBotName = t.Title
								a.NewBotRole = t.Role
								a.NewBotPrompt = t.SystemPrompt
								a.NewBotAgentID = t.AgentID
								a.NewBotAgentType = t.AgentType
								if t.Avatar != "" {
									a.NewBotAvatar = t.Avatar
								}
							}
						}
					})
				})

				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					avPick := ui.Box(c.Key("nb-avatar-pick")).Radius(12).Cursor(ui.CursorPointer)
					avPick.Tooltip("Change avatar")
					avPick.Children(func() {
						a.renderBotAvatarImg(c, tc, a.NewBotAvatar, 44, 12)
					})
					if avPick.Clicked() {
						a.OpenBotAvatarModal("")
					}
					ui.Column(c).Grow(1).Gap(4).Children(func() {
						ui.Text(c, "Bot Name").FontSize(11.5).FontWeight(600).TextColor(tc.TextMuted)
						ui.TextInput(c, &a.NewBotName).Placeholder("e.g. Scout, CI Watch, Review Buddy").FillWidth()
					})
				})
				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, "Role").FontSize(11.5).FontWeight(600).TextColor(tc.TextMuted)
					ui.TextInput(c, &a.NewBotRole).Placeholder("e.g. Bug triage & PR fixer").FillWidth()
				})
				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, "Default Agent").FontSize(11.5).FontWeight(600).TextColor(tc.TextMuted)
					ui.Row(c).Gap(6).Wrap().Children(func() {
						for _, o := range opts {
							opt := o
							sel := a.NewBotAgentID == opt.ID
							pill := ui.Row(c.Key("nb-ag-"+opt.ID)).Height(28).Radius(7).Padding(0, 9).Gap(5).
								AlignItems(ui.Center).Cursor(ui.CursorPointer).BorderWidth(1)
							if sel {
								pill.Background(tc.Accent.Alpha(0.16)).BorderColor(tc.Accent.Alpha(0.55))
							} else {
								pill.Background(tc.SurfaceRaised).BorderColor(tc.Border)
							}
							pill.Children(func() {
								if opt.Type == "internal" {
									ui.Icon(c, icons.ProviderForgeADE).Width(12).Height(12)
								} else {
									icons.RenderProviderIcon(c, opt.ID+" "+opt.Name, 12)
								}
								ui.Text(c, opt.Name).FontSize(11.5).FontWeight(500).TextColor(tc.Text)
							})
							if pill.Clicked() {
								a.NewBotAgentID = opt.ID
								a.NewBotAgentType = opt.Type
							}
						}
					})
				})
				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, "Instructions (optional)").FontSize(11.5).FontWeight(600).TextColor(tc.TextMuted)
					ui.TextAreaBase(c, &a.NewBotPrompt).Lines(2, 4).Placeholder("Standing instructions when this bot takes over a task...").FillWidth()
				})
			})

			ui.Row(c).Padding(12, 18).Gap(8).Justify(ui.End).AlignItems(ui.Center).
				BorderWidth(1, 0, 0, 0).BorderColor(tc.Border).Children(func() {
				cancelBtn := ui.Row(c.Key("nb-cancel")).Height(30).Radius(7).Padding(0, 12).
					AlignItems(ui.Center).Cursor(ui.CursorPointer).
					Background(tc.ToolbarControl).BorderWidth(1).BorderColor(tc.Border)
				cancelBtn.Children(func() {
					ui.Text(c, "Cancel").FontSize(12).FontWeight(500).TextColor(tc.Text)
				})
				if cancelBtn.Clicked() {
					a.ShowNewBotModal = false
				}

				createBtn := ui.Row(c.Key("nb-create")).Height(30).Radius(7).Padding(0, 14).Gap(5).
					AlignItems(ui.Center).Cursor(ui.CursorPointer).Background(tc.MessageAction)
				createBtn.Children(func() {
					ui.Text(c, "Create Bot").FontSize(12).FontWeight(600).TextColor(tc.MessageActionForeground)
				})
				if createBtn.Clicked() && a.Bots != nil {
					ws := ""
					if a.Workspace != nil {
						ws = a.Workspace.RootDir
					}
					bot := a.Bots.Create(agent.BotProfile{
						Name:          strings.TrimSpace(a.NewBotName),
						Role:          strings.TrimSpace(a.NewBotRole),
						SystemPrompt:  strings.TrimSpace(a.NewBotPrompt),
						AgentID:       a.NewBotAgentID,
						AgentType:     a.NewBotAgentType,
						Avatar:        a.NewBotAvatar,
						WorkspacePath: ws,
					})
					a.ShowNewBotModal = false
					if bot != nil && a.Agent != nil {
						if home := a.Agent.OpenBotHomeSession(a.Bots, bot); home != nil {
							a.ensureAgentTab(home.ID)
						}
						a.ShowRightDrawer = true
						a.RightDrawerTab = RightTabBot
						a.BotDrawerSubTab = "tasks"
						c.AddToast(ui.Toast{Title: "Bot Created", Description: bot.Name + " is ready", Type: "success"})
					}
				}
			})
		})
	})
}

func (a *AppUI) renderSpawnWorkerModal(c *ui.Context) {
	if !a.ShowSpawnWorkerModal {
		return
	}
	tc := a.ThemeMgr.Colors()

	type workerAgentOpt struct {
		ID   string
		Name string
		Type string
	}
	opts := []workerAgentOpt{
		{ID: "agent-internal", Name: "ForgeADE Internal", Type: "internal"},
	}
	if a.ACPMgr != nil {
		for _, def := range a.ACPMgr.Agents {
			if def.Type == "internal" || def.ID == "agent-internal" {
				continue
			}
			opts = append(opts, workerAgentOpt{ID: def.ID, Name: def.Name, Type: "acp"})
		}
	}

	a.modal(c, &a.ShowSpawnWorkerModal, func() {
		modal := ui.Column(c).Width(480).Radius(14).
			Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).
			Shadow(0, 20, 56, -16, ui.RGBA(0, 0, 0, 0.55)).Clip()
		modal.Children(func() {
			ui.Row(c).Padding(14, 18).Gap(10).AlignItems(ui.Center).
				BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
				ui.Icon(c, icons.TablerCpu).Width(16).Height(16).TextColor(tc.Accent)
				ui.Column(c).Grow(1).Gap(1).Children(func() {
					ui.Text(c, "Spawn Parallel Worker").FontSize(14).FontWeight(600).TextColor(tc.Text)
					ui.Text(c, "Run an Internal or ACP worker alongside this session").FontSize(11).TextColor(tc.TextMuted)
				})
				xBtn := ui.Row(c.Key("sw-close")).Size(24, 24).Radius(6).AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
				xBtn.Children(func() {
					ui.Icon(c, icons.TablerX).Width(13).Height(13).TextColor(tc.IconMuted)
				})
				if xBtn.Hovered() {
					xBtn.Background(tc.Text.Alpha(0.08))
				}
				if xBtn.Clicked() {
					a.ShowSpawnWorkerModal = false
				}
			})

			ui.Column(c).Padding(16, 18).Gap(11).Children(func() {
				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, "Worker Name").FontSize(11.5).FontWeight(600).TextColor(tc.TextMuted)
					ui.TextInput(c, &a.WorkerNameInput).Placeholder("e.g. tax-rules, promo-tests, code-review").FillWidth()
				})
				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, "Worker Engine (Internal or ACP)").FontSize(11.5).FontWeight(600).TextColor(tc.TextMuted)
					ui.Row(c).Gap(6).Wrap().Children(func() {
						for _, o := range opts {
							opt := o
							sel := a.WorkerAgentID == opt.ID
							pill := ui.Row(c.Key("sw-ag-"+opt.ID)).Height(28).Radius(7).Padding(0, 9).Gap(5).
								AlignItems(ui.Center).Cursor(ui.CursorPointer).BorderWidth(1)
							if sel {
								pill.Background(tc.Accent.Alpha(0.16)).BorderColor(tc.Accent.Alpha(0.55))
							} else {
								pill.Background(tc.SurfaceRaised).BorderColor(tc.Border)
							}
							pill.Children(func() {
								if opt.Type == "internal" {
									ui.Icon(c, icons.ProviderForgeADE).Width(12).Height(12)
								} else {
									icons.RenderProviderIcon(c, opt.ID+" "+opt.Name, 12)
								}
								ui.Text(c, opt.Name).FontSize(11.5).FontWeight(500).TextColor(tc.Text)
							})
							if pill.Clicked() {
								a.WorkerAgentID = opt.ID
								a.WorkerAgentType = opt.Type
							}
						}
					})
				})
				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, "Task Prompt").FontSize(11.5).FontWeight(600).TextColor(tc.TextMuted)
					ui.TextAreaBase(c, &a.WorkerTaskInput).Lines(3, 5).Placeholder("Describe the scoped task for this parallel worker...").FillWidth()
				})

				wtRow := ui.Row(c.Key("sw-wt-toggle")).Height(28).Radius(7).Padding(0, 8).Gap(8).
					AlignItems(ui.Center).Cursor(ui.CursorPointer).
					Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border)
				wtRow.Children(func() {
					box := ui.Box(c).Size(14, 14).Radius(4).AlignItems(ui.Center).Justify(ui.Center).BorderWidth(1)
					if a.WorkerUseWorktree {
						box.Background(tc.Accent).BorderColor(tc.Accent).Children(func() {
							ui.Icon(c, icons.TablerCheck).Width(10).Height(10).TextColor(tc.AccentForeground)
						})
					} else {
						box.BorderColor(tc.Border)
					}
					ui.Text(c, "Isolate in a dedicated git worktree").FontSize(12).FontWeight(500).TextColor(tc.Text).Grow(1)
				})
				if wtRow.Clicked() {
					a.WorkerUseWorktree = !a.WorkerUseWorktree
				}

				if a.WorkerError != "" {
					ui.Text(c, a.WorkerError).FontSize(11.5).TextColor(tc.Error)
				}
			})

			ui.Row(c).Padding(12, 18).Gap(8).Justify(ui.End).AlignItems(ui.Center).
				BorderWidth(1, 0, 0, 0).BorderColor(tc.Border).Children(func() {
				cancelBtn := ui.Row(c.Key("sw-cancel")).Height(30).Radius(7).Padding(0, 12).
					AlignItems(ui.Center).Cursor(ui.CursorPointer).
					Background(tc.ToolbarControl).BorderWidth(1).BorderColor(tc.Border)
				cancelBtn.Children(func() {
					ui.Text(c, "Cancel").FontSize(12).FontWeight(500).TextColor(tc.Text)
				})
				if cancelBtn.Clicked() {
					a.ShowSpawnWorkerModal = false
				}

				launchBtn := ui.Row(c.Key("sw-launch")).Height(30).Radius(7).Padding(0, 14).Gap(6).
					AlignItems(ui.Center).Cursor(ui.CursorPointer).Background(tc.MessageAction)
				launchBtn.Children(func() {
					ui.Text(c, "Start Worker").FontSize(12).FontWeight(600).TextColor(tc.MessageActionForeground)
				})
				if launchBtn.Clicked() && a.Agent != nil {
					wsMode := "shared"
					if a.WorkerUseWorktree {
						wsMode = "worktree"
					}
					parentID := a.Agent.SnapshotActiveSessionID()
					rec, err := a.Agent.SpawnWorker(agent.WorkerSpec{
						ParentSessionID: parentID,
						Name:            strings.TrimSpace(a.WorkerNameInput),
						AgentID:         a.WorkerAgentID,
						AgentType:       a.WorkerAgentType,
						Task:            strings.TrimSpace(a.WorkerTaskInput),
						WorkspaceMode:   wsMode,
						AutoStart:       true,
					})
					if err != nil {
						a.WorkerError = fmt.Sprintf("Failed: %v", err)
					} else {
						a.ShowSpawnWorkerModal = false
						c.AddToast(ui.Toast{Title: "Worker Started", Description: rec.Name + " is running in parallel", Type: "success"})
					}
				}
			})
		})
	})
}

func (a *AppUI) renderNewBotTaskModal(c *ui.Context) {
	if !a.ShowNewBotTaskModal {
		return
	}
	tc := a.ThemeMgr.Colors()

	var bot *agent.BotProfile
	if a.Bots != nil {
		bot = a.Bots.Get(a.NewBotTaskBotID)
	}
	botName := "Bot"
	if bot != nil {
		botName = bot.Name
	}

	type harnessOpt struct {
		ID   string
		Name string
		Type string
	}
	opts := []harnessOpt{
		{ID: "agent-internal", Name: "ForgeADE Internal (RakitKode)", Type: "internal"},
	}
	if a.ACPMgr != nil {
		for _, def := range a.ACPMgr.Agents {
			if def.Type == "internal" || def.ID == "agent-internal" {
				continue
			}
			opts = append(opts, harnessOpt{ID: def.ID, Name: def.Name, Type: "acp"})
		}
	}

	a.modal(c, &a.ShowNewBotTaskModal, func() {
		modal := ui.Column(c).Width(480).Radius(14).
			Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).
			Shadow(0, 20, 56, -16, ui.RGBA(0, 0, 0, 0.55)).Clip()
		modal.Children(func() {
			ui.Row(c).Padding(14, 18).Gap(10).AlignItems(ui.Center).
				BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
				ui.Icon(c, icons.TablerCpu).Width(16).Height(16).TextColor(tc.Accent)
				ui.Column(c).Grow(1).Gap(1).Children(func() {
					ui.Text(c, "New Background Task · "+botName).FontSize(14).FontWeight(600).TextColor(tc.Text)
					ui.Text(c, "Runs in background via RakitKode or ACP without cluttering chat").FontSize(11).TextColor(tc.TextMuted)
				})
				xBtn := ui.Row(c.Key("nbt-close")).Size(24, 24).Radius(6).AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
				xBtn.Children(func() {
					ui.Icon(c, icons.TablerX).Width(13).Height(13).TextColor(tc.IconMuted)
				})
				if xBtn.Hovered() {
					xBtn.Background(tc.Text.Alpha(0.08))
				}
				if xBtn.Clicked() {
					a.ShowNewBotTaskModal = false
				}
			})

			ui.Column(c).Padding(16, 18).Gap(11).Children(func() {
				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, "Task Title (optional)").FontSize(11.5).FontWeight(600).TextColor(tc.TextMuted)
					ui.TextInput(c, &a.NewBotTaskTitle).Placeholder("e.g. Fix flaky login test").FillWidth()
				})
				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, "Harness (Internal RakitKode or ACP)").FontSize(11.5).FontWeight(600).TextColor(tc.TextMuted)
					ui.Row(c).Gap(6).Wrap().Children(func() {
						for _, o := range opts {
							opt := o
							sel := a.NewBotTaskAgentID == opt.ID
							pill := ui.Row(c.Key("nbt-ag-"+opt.ID)).Height(28).Radius(7).Padding(0, 9).Gap(5).
								AlignItems(ui.Center).Cursor(ui.CursorPointer).BorderWidth(1)
							if sel {
								pill.Background(tc.Accent.Alpha(0.16)).BorderColor(tc.Accent.Alpha(0.55))
							} else {
								pill.Background(tc.SurfaceRaised).BorderColor(tc.Border)
							}
							pill.Children(func() {
								if opt.Type == "internal" {
									ui.Icon(c, icons.ProviderForgeADE).Width(12).Height(12)
								} else {
									icons.RenderProviderIcon(c, opt.ID+" "+opt.Name, 12)
								}
								ui.Text(c, opt.Name).FontSize(11.5).FontWeight(500).TextColor(tc.Text)
							})
							if pill.Clicked() {
								a.NewBotTaskAgentID = opt.ID
								a.NewBotTaskAgentType = opt.Type
							}
						}
					})
				})
				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, "Task Instructions").FontSize(11.5).FontWeight(600).TextColor(tc.TextMuted)
					ui.TextAreaBase(c, &a.NewBotTaskPrompt).Lines(3, 5).Placeholder("Describe what the bot should execute in the background...").FillWidth()
				})

				wtRow := ui.Row(c.Key("nbt-wt-toggle")).Height(28).Radius(7).Padding(0, 8).Gap(8).
					AlignItems(ui.Center).Cursor(ui.CursorPointer).
					Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border)
				wtRow.Children(func() {
					box := ui.Box(c).Size(14, 14).Radius(4).AlignItems(ui.Center).Justify(ui.Center).BorderWidth(1)
					if a.NewBotTaskWorktree {
						box.Background(tc.Accent).BorderColor(tc.Accent).Children(func() {
							ui.Icon(c, icons.TablerCheck).Width(10).Height(10).TextColor(tc.AccentForeground)
						})
					} else {
						box.BorderColor(tc.Border)
					}
					ui.Text(c, "Isolate in a dedicated git worktree").FontSize(12).FontWeight(500).TextColor(tc.Text).Grow(1)
				})
				if wtRow.Clicked() {
					a.NewBotTaskWorktree = !a.NewBotTaskWorktree
				}

				if a.NewBotTaskError != "" {
					ui.Text(c, a.NewBotTaskError).FontSize(11.5).TextColor(tc.Error)
				}
			})

			ui.Row(c).Padding(12, 18).Gap(8).Justify(ui.End).AlignItems(ui.Center).
				BorderWidth(1, 0, 0, 0).BorderColor(tc.Border).Children(func() {
				cancelBtn := ui.Row(c.Key("nbt-cancel")).Height(30).Radius(7).Padding(0, 12).
					AlignItems(ui.Center).Cursor(ui.CursorPointer).
					Background(tc.ToolbarControl).BorderWidth(1).BorderColor(tc.Border)
				cancelBtn.Children(func() {
					ui.Text(c, "Cancel").FontSize(12).FontWeight(500).TextColor(tc.Text)
				})
				if cancelBtn.Clicked() {
					a.ShowNewBotTaskModal = false
				}

				launchBtn := ui.Row(c.Key("nbt-launch")).Height(30).Radius(7).Padding(0, 14).Gap(6).
					AlignItems(ui.Center).Cursor(ui.CursorPointer).Background(tc.MessageAction)
				launchBtn.Children(func() {
					ui.Text(c, "Run Task").FontSize(12).FontWeight(600).TextColor(tc.MessageActionForeground)
				})
				if launchBtn.Clicked() && a.Agent != nil && bot != nil {
					sess, err := a.Agent.SpawnBotTask(a.Bots, bot, a.NewBotTaskTitle, a.NewBotTaskPrompt, a.NewBotTaskAgentID, a.NewBotTaskAgentType, a.NewBotTaskWorktree)
					if err != nil {
						a.NewBotTaskError = fmt.Sprintf("Failed: %v", err)
					} else {
						a.ShowNewBotTaskModal = false
						a.ShowRightDrawer = true
						a.RightDrawerTab = RightTabBot
						a.BotDrawerSubTab = "tasks"
						if sess != nil {
							a.SelectedBotTaskID = sess.ID
						}
						c.AddToast(ui.Toast{Title: "Background Task Started", Description: bot.Name + " is running the task", Type: "success"})
					}
				}
			})
		})
	})
}
