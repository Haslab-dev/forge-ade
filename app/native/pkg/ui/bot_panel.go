package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/agent"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
)

// activeBotProfile resolves the BotProfile owning the currently active session,
// or falls back to the first configured bot if any.
func (a *AppUI) activeBotProfile() *agent.BotProfile {
	if a.Bots == nil {
		return nil
	}
	if a.Agent != nil {
		if sess := a.Agent.ActiveSession(); sess != nil && sess.BotID != "" {
			if b := a.Bots.Get(sess.BotID); b != nil {
				return b
			}
		}
	}
	list := a.Bots.List()
	if len(list) > 0 {
		return list[0]
	}
	return nil
}

// renderBotRightPane renders the MuseAI-style right inspector for the active Bot:
// Bot header + 3 sub-tabs (Tasks grouped by day, Approvals, Persona).
func (a *AppUI) renderBotRightPane(c *ui.Context, tc theme.ThemeColors) {
	bot := a.activeBotProfile()
	if bot == nil {
		ui.Column(c).Grow(1).Center().Padding(24).Gap(10).Children(func() {
			ui.Icon(c, icons.TablerBot).Width(28).Height(28).TextColor(tc.IconMuted)
			ui.Text(c, "No Bot Selected").FontSize(13).FontWeight(600).TextColor(tc.Text)
			ui.Text(c, "Create or select a bot in the left sidebar to inspect its background tasks and approvals.").
				FontSize(11.5).TextColor(tc.TextMuted)
			btn := ui.Row(c.Key("bp-new-bot")).Height(28).Radius(7).Padding(0, 12).Gap(5).
				AlignItems(ui.Center).Cursor(ui.CursorPointer).Background(tc.Accent)
			btn.Children(func() {
				ui.Icon(c, icons.TablerPlus).Width(12).Height(12).TextColor(tc.AccentForeground)
				ui.Text(c, "Create Bot").FontSize(11.5).FontWeight(600).TextColor(tc.AccentForeground)
			})
			if btn.Clicked() {
				a.OpenNewBotModal()
			}
		})
		return
	}

	var tasks []agent.BotTaskSummary
	if a.Agent != nil {
		tasks = a.Agent.BotTasks(a.Bots, bot.ID)
	}
	var allApprovals []agent.PendingApproval
	runningCount := 0
	for _, t := range tasks {
		if t.Status == "running" {
			runningCount++
		}
		allApprovals = append(allApprovals, t.PendingApprovals...)
	}

	subTab := a.BotDrawerSubTab
	if subTab == "" {
		subTab = "tasks"
	}

	ui.Column(c).Grow(1).FillHeight().Children(func() {
		// 1. Bot Header
		ui.Column(c).Padding(14, 16, 12, 16).Gap(10).
			BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				avBtn := ui.Box(c.Key("bp-avatar-edit")).Radius(12).Cursor(ui.CursorPointer)
				avBtn.Tooltip("Change bot avatar")
				avBtn.Children(func() {
					a.renderBotAvatar(c, tc, bot, 44, 12)
				})
				if avBtn.Clicked() {
					a.OpenBotAvatarModal(bot.ID)
				}
				ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
					ui.Text(c, bot.Name).FontSize(14).FontWeight(600).TextColor(tc.Text).SingleLine()
					statusText := "Connected"
					statusColor := tc.Success
					if bot.Paused {
						statusText = "Paused"
						statusColor = tc.TextMuted
					} else if len(allApprovals) > 0 {
						statusText = fmt.Sprintf("%d waiting approval", len(allApprovals))
						statusColor = tc.Warning
					} else if runningCount > 0 {
						statusText = fmt.Sprintf("%d task running", runningCount)
						statusColor = tc.Accent
					}
					ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
						ui.Box(c).Size(6, 6).Radius(3).Background(statusColor)
						ui.Text(c, statusText).FontSize(11).FontWeight(500).TextColor(statusColor).SingleLine()
					})
				})
				newTaskBtn := ui.Row(c.Key("bp-new-task")).Height(26).Radius(6).Padding(0, 9).Gap(4).
					AlignItems(ui.Center).Cursor(ui.CursorPointer).
					Background(tc.Accent.Alpha(0.14)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.35))
				newTaskBtn.Children(func() {
					ui.Icon(c, icons.TablerPlus).Width(11).Height(11).TextColor(tc.Accent)
					ui.Text(c, "Task").FontSize(11).FontWeight(600).TextColor(tc.Accent)
				})
				if newTaskBtn.Hovered() {
					newTaskBtn.Background(tc.Accent.Alpha(0.22))
				}
				if newTaskBtn.Clicked() {
					a.OpenNewBotTaskModal(bot.ID)
				}
			})

			// 2. Sub-tabs: Tasks | Approvals | Persona
			ui.Row(c).Gap(6).Children(func() {
				tabs := []struct {
					id    string
					label string
					badge int
				}{
					{"tasks", "Tasks", len(tasks)},
					{"approvals", "Approvals", len(allApprovals)},
					{"persona", "Persona", 0},
				}
				for _, tb := range tabs {
					tItem := tb
					active := subTab == tItem.id
					pill := ui.Row(c.Key("bp-tab-"+tItem.id)).Height(26).Radius(6).Padding(0, 10).Gap(5).
						AlignItems(ui.Center).Cursor(ui.CursorPointer).BorderWidth(1)
					if active {
						pill.Background(tc.SurfaceRaised).BorderColor(tc.Border)
					} else {
						pill.BorderColor(tc.Border.Alpha(0))
					}
					pill.Children(func() {
						txt := ui.Text(c, tItem.label).FontSize(11.5).FontWeight(500).TextColor(tc.TextMuted)
						if active {
							txt.TextColor(tc.Text).FontWeight(600)
						}
						if tItem.badge > 0 {
							bg := tc.Text.Alpha(0.08)
							fg := tc.TextMuted
							if tItem.id == "approvals" {
								bg = tc.Warning.Alpha(0.2)
								fg = tc.Warning
							}
							ui.Box(c).Radius(8).Padding(1, 5).Background(bg).Children(func() {
								ui.Text(c, fmt.Sprintf("%d", tItem.badge)).FontSize(10).FontWeight(600).TextColor(fg)
							})
						}
					})
					if pill.Hovered() && !active {
						pill.Background(tc.Text.Alpha(0.05))
					}
					if pill.Clicked() {
						a.BotDrawerSubTab = tItem.id
					}
				}
			})
		})

		// 3. Tab Body
		ui.Scroll(c).Grow(1).Padding(12, 14).Children(func() {
			switch subTab {
			case "approvals":
				a.renderBotApprovalsTab(c, tc, allApprovals)
			case "persona":
				a.renderBotPersonaTab(c, tc, bot)
			default:
				a.renderBotTasksTab(c, tc, bot, tasks)
			}
		})
	})
}

func (a *AppUI) renderBotTasksTab(c *ui.Context, tc theme.ThemeColors, bot *agent.BotProfile, tasks []agent.BotTaskSummary) {
	if len(tasks) == 0 {
		ui.Column(c).Padding(16, 8).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icons.TablerChecklist).Width(22).Height(22).TextColor(tc.IconMuted)
			ui.Text(c, "No background tasks yet").FontSize(12).FontWeight(600).TextColor(tc.Text)
			ui.Text(c, "Tasks spawned by "+bot.Name+" (via RakitKode or ACP) appear here so the main chat stays clean.").
				FontSize(11).TextColor(tc.TextMuted)
		})
		return
	}

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	yesterdayStart := todayStart.AddDate(0, 0, -1)

	var today, yesterday, earlier []agent.BotTaskSummary
	for _, t := range tasks {
		ts := t.UpdatedAt
		if ts.IsZero() {
			ts = t.CreatedAt
		}
		if !ts.Before(todayStart) {
			today = append(today, t)
		} else if !ts.Before(yesterdayStart) {
			yesterday = append(yesterday, t)
		} else {
			earlier = append(earlier, t)
		}
	}

	renderGroup := func(label string, items []agent.BotTaskSummary) {
		if len(items) == 0 {
			return
		}
		ui.Column(c).Gap(6).Padding(0, 0, 10, 0).Children(func() {
			ui.Text(c, label).FontSize(11).FontWeight(600).TextColor(tc.TextMuted).Padding(0, 2)
			for _, item := range items {
				tk := item
				selected := a.SelectedBotTaskID == tk.SessionID && a.ShowBotTaskModal
				card := ui.Row(c.Key("bt-card-"+tk.SessionID)).Padding(9, 10).Gap(9).Radius(9).
					AlignItems(ui.Start).Cursor(ui.CursorPointer).BorderWidth(1)
				if selected {
					card.Background(tc.Accent.Alpha(0.12)).BorderColor(tc.Accent.Alpha(0.45))
				} else {
					card.Background(tc.SurfaceRaised).BorderColor(tc.Border)
				}
				card.Children(func() {
					// Status icon
					stIcon := icons.TablerCheck
					stColor := tc.Success
					switch tk.Status {
					case "running":
						stIcon = icons.TablerRefresh
						stColor = tc.Accent
					case "needs_approval":
						stIcon = icons.TablerAlertTriangle
						stColor = tc.Warning
					case "failed":
						stIcon = icons.TablerX
						stColor = tc.Error
					}
					ui.Box(c).Size(18, 18).Radius(9).AlignItems(ui.Center).Justify(ui.Center).
						Background(stColor.Alpha(0.16)).Children(func() {
						ui.Icon(c, stIcon).Width(11).Height(11).TextColor(stColor)
					})

					ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
						ui.Text(c, tk.Title).FontSize(12).FontWeight(600).TextColor(tc.Text).SingleLine()
						sub := tk.Summary
						if tk.Status == "running" && tk.CurrentStep != "" {
							sub = tk.CurrentStep
						} else if tk.Status == "needs_approval" {
							sub = fmt.Sprintf("Waiting for approval (%d)", len(tk.PendingApprovals))
						}
						if sub == "" {
							sub = tk.HarnessLabel
						}
						ui.Text(c, sub).FontSize(11).TextColor(tc.TextMuted).SingleLine()
						ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
							ui.Text(c, tk.HarnessLabel).FontSize(10).TextColor(tc.TextMuted)
							ui.Text(c, "·").FontSize(10).TextColor(tc.TextMuted)
							ui.Text(c, tk.UpdatedAt.Format("3:04 PM")).FontSize(10).TextColor(tc.TextMuted)
						})
					})
				})
				if card.Hovered() && !selected {
					card.Background(tc.Text.Alpha(0.05))
				}
				if card.Clicked() {
					a.SelectedBotTaskID = tk.SessionID
					a.ShowBotTaskModal = true
				}
			}
		})
	}

	renderGroup("Today", today)
	renderGroup("Yesterday", yesterday)
	renderGroup("Earlier", earlier)
}

func (a *AppUI) renderBotApprovalsTab(c *ui.Context, tc theme.ThemeColors, approvals []agent.PendingApproval) {
	if len(approvals) == 0 {
		ui.Column(c).Padding(16, 8).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icons.TablerShieldCheck).Width(22).Height(22).TextColor(tc.IconMuted)
			ui.Text(c, "No pending approvals").FontSize(12).FontWeight(600).TextColor(tc.Text)
			ui.Text(c, "When a background task needs permission to run a command or edit a file, it will appear here.").
				FontSize(11).TextColor(tc.TextMuted)
		})
		return
	}

	ui.Column(c).Gap(8).Children(func() {
		for _, ap := range approvals {
			appr := ap
			card := ui.Column(c.Key("bp-ap-"+appr.ToolCallID)).Padding(10, 12).Gap(6).Radius(9).
				Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Warning.Alpha(0.45))
			card.Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icons.TablerAlertTriangle).Width(13).Height(13).TextColor(tc.Warning)
					ui.Text(c, appr.ToolName).FontSize(12).FontWeight(600).TextColor(tc.Text).Grow(1).SingleLine()
				})
				if appr.Summary != "" {
					ui.Text(c, appr.Summary).FontSize(11).TextColor(tc.TextMuted)
				}
				if appr.Arguments != "" {
					ui.Box(c).Padding(6, 8).Radius(6).Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).Children(func() {
						ui.Text(c, appr.Arguments).FontSize(10.5).TextColor(tc.Text)
					})
				}
				ui.Row(c).Gap(6).Justify(ui.End).Children(func() {
					denyBtn := ui.Row(c.Key("bp-deny-"+appr.ToolCallID)).Height(24).Radius(6).Padding(0, 10).
						AlignItems(ui.Center).Cursor(ui.CursorPointer).
						Background(tc.ToolbarControl).BorderWidth(1).BorderColor(tc.Border)
					denyBtn.Children(func() {
						ui.Text(c, "Deny").FontSize(11).FontWeight(500).TextColor(tc.Text)
					})
					if denyBtn.Clicked() {
						if err := a.respondApproval(appr.SessionID, appr.ToolCallID, false, false); err != nil {
							a.queueToast(ui.Toast{Title: "Approval Failed", Description: err.Error(), Type: "error"})
						} else if a.Agent != nil {
							a.Agent.ResolveSessionApproval(appr.SessionID, appr.ToolCallID)
						}
					}

					allowBtn := ui.Row(c.Key("bp-allow-"+appr.ToolCallID)).Height(24).Radius(6).Padding(0, 10).
						AlignItems(ui.Center).Cursor(ui.CursorPointer).Background(tc.Accent)
					allowBtn.Children(func() {
						ui.Text(c, "Allow").FontSize(11).FontWeight(600).TextColor(tc.AccentForeground)
					})
					if allowBtn.Clicked() {
						if err := a.respondApproval(appr.SessionID, appr.ToolCallID, true, false); err != nil {
							a.queueToast(ui.Toast{Title: "Approval Failed", Description: err.Error(), Type: "error"})
						} else if a.Agent != nil {
							a.Agent.ResolveSessionApproval(appr.SessionID, appr.ToolCallID)
						}
					}
				})
			})
		}
	})
}

func (a *AppUI) syncBotPersonaDraft(bot *agent.BotProfile) {
	if bot == nil {
		return
	}
	a.BotPersonaEditID = bot.ID
	a.BotPersonaName = bot.Name
	a.BotPersonaRole = bot.Role
	a.BotPersonaPrompt = bot.SystemPrompt
	a.BotPersonaAgentID = bot.AgentID
	if a.BotPersonaAgentID == "" {
		a.BotPersonaAgentID = "agent-internal"
	}
	a.BotPersonaAgentType = bot.AgentType
	if a.BotPersonaAgentType == "" {
		if a.BotPersonaAgentID == "agent-internal" {
			a.BotPersonaAgentType = "internal"
		} else {
			a.BotPersonaAgentType = "acp"
		}
	}
}

func (a *AppUI) renderBotPersonaTab(c *ui.Context, tc theme.ThemeColors, bot *agent.BotProfile) {
	if bot == nil {
		return
	}
	if a.BotPersonaEditID != bot.ID {
		a.syncBotPersonaDraft(bot)
	}

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

	dirty := strings.TrimSpace(a.BotPersonaName) != strings.TrimSpace(bot.Name) ||
		strings.TrimSpace(a.BotPersonaRole) != strings.TrimSpace(bot.Role) ||
		strings.TrimSpace(a.BotPersonaPrompt) != strings.TrimSpace(bot.SystemPrompt) ||
		a.BotPersonaAgentID != bot.AgentID

	ui.Column(c).Gap(10).Children(func() {
		infoCard := ui.Column(c).Padding(12, 12).Gap(10).Radius(9).
			Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border)
		infoCard.Children(func() {
			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, "Bot Name").FontSize(10.5).FontWeight(600).TextColor(tc.TextMuted)
				ui.TextInput(c, &a.BotPersonaName).Placeholder("Bot name").FillWidth()
			})

			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, "Role").FontSize(10.5).FontWeight(600).TextColor(tc.TextMuted)
				ui.TextInput(c, &a.BotPersonaRole).Placeholder("e.g. Bug triage & PR fixer").FillWidth()
			})

			ui.Column(c).Gap(5).Children(func() {
				ui.Text(c, "Default Harness").FontSize(10.5).FontWeight(600).TextColor(tc.TextMuted)
				ui.Row(c).Gap(6).Wrap().Children(func() {
					for _, o := range opts {
						opt := o
						sel := a.BotPersonaAgentID == opt.ID
						pill := ui.Row(c.Key("bp-ag-"+bot.ID+"-"+opt.ID)).Height(26).Radius(6).Padding(0, 8).Gap(5).
							AlignItems(ui.Center).Cursor(ui.CursorPointer).BorderWidth(1)
						if sel {
							pill.Background(tc.Accent.Alpha(0.16)).BorderColor(tc.Accent.Alpha(0.55))
						} else {
							pill.Background(tc.Surface).BorderColor(tc.Border)
						}
						pill.Children(func() {
							if opt.Type == "internal" {
								ui.Icon(c, icons.ProviderForgeADE).Width(11).Height(11)
							} else {
								icons.RenderProviderIcon(c, opt.ID+" "+opt.Name, 11)
							}
							ui.Text(c, opt.Name).FontSize(11).FontWeight(500).TextColor(tc.Text)
						})
						if pill.Clicked() {
							a.BotPersonaAgentID = opt.ID
							a.BotPersonaAgentType = opt.Type
						}
					}
				})
			})

			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, "Standing Instructions").FontSize(10.5).FontWeight(600).TextColor(tc.TextMuted)
				ui.TextAreaBase(c, &a.BotPersonaPrompt).Lines(4, 8).
					Placeholder("Standing instructions when this bot handles chats or background tasks...").
					FillWidth()
			})

			ui.Row(c).Gap(6).Justify(ui.End).AlignItems(ui.Center).Padding(2, 0, 0, 0).Children(func() {
				if dirty {
					resetBtn := ui.Row(c.Key("bp-persona-reset-"+bot.ID)).Height(26).Radius(6).Padding(0, 10).
						AlignItems(ui.Center).Cursor(ui.CursorPointer).
						Background(tc.ToolbarControl).BorderWidth(1).BorderColor(tc.Border)
					resetBtn.Children(func() {
						ui.Text(c, "Reset").FontSize(11).FontWeight(500).TextColor(tc.Text)
					})
					if resetBtn.Clicked() {
						a.syncBotPersonaDraft(bot)
					}
				}

				saveBtn := ui.Row(c.Key("bp-persona-save-"+bot.ID)).Height(26).Radius(6).Padding(0, 12).Gap(5).
					AlignItems(ui.Center).Cursor(ui.CursorPointer)
				if dirty {
					saveBtn.Background(tc.Accent)
				} else {
					saveBtn.Background(tc.ToolbarControl).BorderWidth(1).BorderColor(tc.Border).Opacity(0.65)
				}
				saveBtn.Children(func() {
					fg := tc.TextMuted
					if dirty {
						fg = tc.AccentForeground
					}
					ui.Text(c, "Save Persona").FontSize(11).FontWeight(600).TextColor(fg)
				})
				if saveBtn.Clicked() && dirty && a.Bots != nil {
					updated := a.Bots.UpdatePersona(
						bot.ID,
						a.BotPersonaName,
						a.BotPersonaRole,
						a.BotPersonaPrompt,
						a.BotPersonaAgentID,
						a.BotPersonaAgentType,
					)
					if updated != nil {
						a.syncBotPersonaDraft(updated)
						if a.Agent != nil && updated.HomeSessionID != "" {
							a.Agent.RenameSession(updated.HomeSessionID, updated.Name+" · Home")
						}
						a.queueToast(ui.Toast{Title: "Persona Saved", Description: updated.Name + " updated", Type: "success"})
					}
				}
			})
		})
	})
}

// renderBotTaskModal renders the 2-column MuseAI-style Task Detail Modal:
// Left column = step & tool-call trace; Right column = Summary & Changed files/Artifacts.
func (a *AppUI) renderBotTaskModal(c *ui.Context) {
	if !a.ShowBotTaskModal || a.SelectedBotTaskID == "" || a.Agent == nil {
		return
	}
	bot := a.activeBotProfile()
	if bot == nil {
		a.ShowBotTaskModal = false
		return
	}
	var task *agent.BotTaskSummary
	for _, t := range a.Agent.BotTasks(a.Bots, bot.ID) {
		if t.SessionID == a.SelectedBotTaskID {
			cp := t
			task = &cp
			break
		}
	}
	if task == nil {
		a.ShowBotTaskModal = false
		return
	}

	tc := a.ThemeMgr.Colors()
	a.modal(c, &a.ShowBotTaskModal, func() {
		modal := ui.Column(c).Width(780).MaxHeight(560).Radius(14).
			Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).
			Shadow(0, 20, 56, -16, ui.RGBA(0, 0, 0, 0.55)).Clip()
		modal.Children(func() {
			// Header
			ui.Row(c).Padding(14, 18).Gap(10).AlignItems(ui.Center).
				BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
				badgeLabel := "Complete"
				badgeColor := tc.Success
				switch task.Status {
				case "running":
					badgeLabel = "Running"
					badgeColor = tc.Accent
				case "needs_approval":
					badgeLabel = "Needs approval"
					badgeColor = tc.Warning
				case "failed":
					badgeLabel = "Failed"
					badgeColor = tc.Error
				}
				ui.Box(c).Radius(6).Padding(2, 8).Background(badgeColor.Alpha(0.16)).Children(func() {
					ui.Text(c, badgeLabel).FontSize(11).FontWeight(600).TextColor(badgeColor)
				})
				ui.Text(c, task.Title).FontSize(14).FontWeight(600).TextColor(tc.Text).Grow(1).SingleLine()
				ui.Text(c, task.HarnessLabel).FontSize(11).TextColor(tc.TextMuted)
				xBtn := ui.Row(c.Key("btm-close")).Size(24, 24).Radius(6).AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
				xBtn.Children(func() {
					ui.Icon(c, icons.TablerX).Width(13).Height(13).TextColor(tc.IconMuted)
				})
				if xBtn.Hovered() {
					xBtn.Background(tc.Text.Alpha(0.08))
				}
				if xBtn.Clicked() {
					a.ShowBotTaskModal = false
				}
			})

			// 2-Column Body
			ui.Row(c).Grow(1).MinHeight(360).Children(func() {
				// Left column: Execution steps / tool calls
				ui.Scroll(c).Width(310).FillHeight().Padding(14, 16).
					BorderWidth(0, 1, 0, 0).BorderColor(tc.Border).Children(func() {
					ui.Column(c).Gap(8).Children(func() {
						ui.Text(c, "EXECUTION TRACE").FontSize(10.5).FontWeight(600).TextColor(tc.TextMuted)
						if len(task.Steps) == 0 {
							ui.Text(c, "No tool calls recorded yet.").FontSize(11.5).TextColor(tc.TextMuted)
						} else {
							for i, st := range task.Steps {
								step := st
								ic := icons.TablerCheck
								col := tc.Success
								if step.Status == "running" {
									ic = icons.TablerRefresh
									col = tc.Accent
								} else if step.Status == "failed" {
									ic = icons.TablerX
									col = tc.Error
								}
								ui.Row(c.Key(fmt.Sprintf("btm-st-%d-%s", i, step.ID))).Gap(8).AlignItems(ui.Start).Padding(4, 0).Children(func() {
									ui.Icon(c, ic).Width(13).Height(13).TextColor(col)
									ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
										ui.Text(c, step.Label).FontSize(11.5).FontWeight(600).TextColor(tc.Text).SingleLine()
										if step.Detail != "" {
											ui.Text(c, step.Detail).FontSize(10.5).TextColor(tc.TextMuted).SingleLine()
										}
									})
								})
							}
						}
					})
				})

				// Right column: Summary & Artifacts
				ui.Scroll(c).Grow(1).FillHeight().Padding(16, 18).Children(func() {
					ui.Column(c).Gap(14).Children(func() {
						ui.Column(c).Gap(4).Children(func() {
							ui.Text(c, "Summary").FontSize(13).FontWeight(600).TextColor(tc.Text)
							sum := task.Summary
							if sum == "" {
								sum = "Task is currently running in the background."
							}
							ui.Text(c, sum).FontSize(12).TextColor(tc.Text)
						})

						if len(task.Changes) > 0 {
							ui.Column(c).Gap(6).Children(func() {
								ui.Text(c, fmt.Sprintf("Touched Files (%d)", len(task.Changes))).FontSize(12).FontWeight(600).TextColor(tc.Text)
								for _, ch := range task.Changes {
									fc := ch
									ui.Row(c.Key("btm-fc-"+fc.Path)).Padding(6, 10).Gap(8).Radius(7).
										Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border).AlignItems(ui.Center).Children(func() {
										ui.Icon(c, icons.TablerFileText).Width(13).Height(13).TextColor(tc.IconMuted)
										ui.Text(c, fc.Path).FontSize(11.5).TextColor(tc.Text).Grow(1).SingleLine()
										if fc.Additions > 0 {
											ui.Text(c, fmt.Sprintf("+%d", fc.Additions)).FontSize(11).TextColor(tc.Success)
										}
										if fc.Deletions > 0 {
											ui.Text(c, fmt.Sprintf("-%d", fc.Deletions)).FontSize(11).TextColor(tc.Error)
										}
									})
								}
							})
						}
					})
				})
			})

			// Footer
			ui.Row(c).Padding(10, 18).Gap(8).Justify(ui.End).AlignItems(ui.Center).
				BorderWidth(1, 0, 0, 0).BorderColor(tc.Border).Children(func() {
				openBtn := ui.Row(c.Key("btm-open-sess")).Height(28).Radius(7).Padding(0, 12).
					AlignItems(ui.Center).Cursor(ui.CursorPointer).
					Background(tc.ToolbarControl).BorderWidth(1).BorderColor(tc.Border)
				openBtn.Children(func() {
					ui.Text(c, "Open Full Session").FontSize(11.5).FontWeight(500).TextColor(tc.Text)
				})
				if openBtn.Clicked() {
					a.ShowBotTaskModal = false
					sessID := task.SessionID
					if idx := strings.Index(sessID, "#"); idx > 0 {
						sessID = sessID[:idx]
					}
					a.Agent.SelectSession(sessID)
				}
			})
		})
	})
}

// renderChatProcessBar renders a live processing indicator above the composer
// whenever the active chat is running (summary step, active tool call, elapsed time)
// or when the active Bot has background tasks running / awaiting approval.
func (a *AppUI) renderChatProcessBar(c *ui.Context, tc theme.ThemeColors, sess *agent.AgentSession) {
	if a.Agent == nil || sess == nil {
		return
	}
	st := a.Agent.ActiveTurnStatus(a.Bots, sess.ID)
	if !st.Active && st.BackgroundRunning == 0 && st.BackgroundApprovals == 0 {
		return
	}

	bar := ui.Row(c.Key("chat-proc-bar-"+sess.ID)).Padding(6, 12).Gap(8).Radius(8).
		AlignItems(ui.Center).Cursor(ui.CursorPointer).
		Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border)
	bar.Children(func() {
		dotCol := tc.Accent
		if st.PendingApproval || st.BackgroundApprovals > 0 {
			dotCol = tc.Warning
		}
		ui.Box(c).Size(7, 7).Radius(4).Background(dotCol)

		var parts []string
		if st.Active {
			if st.CurrentTool != "" {
				lbl := "Running " + st.CurrentTool
				if st.CurrentToolCommand != "" {
					cmd := st.CurrentToolCommand
					if len(cmd) > 44 {
						cmd = cmd[:44] + "…"
					}
					lbl += " (" + cmd + ")"
				}
				parts = append(parts, lbl)
			} else if st.IsThinking {
				parts = append(parts, "Thinking…")
			} else {
				parts = append(parts, "Processing…")
			}
			if st.StepCount > 0 || st.CompletedTools > 0 {
				steps := st.StepCount
				if st.CompletedTools > steps {
					steps = st.CompletedTools
				}
				parts = append(parts, fmt.Sprintf("%d steps", steps))
			}
			if st.ElapsedSec > 0 {
				parts = append(parts, fmt.Sprintf("%ds", st.ElapsedSec))
			}
		}
		if st.BackgroundRunning > 0 {
			parts = append(parts, fmt.Sprintf("%d background task(s) running", st.BackgroundRunning))
		}
		if st.BackgroundApprovals > 0 {
			parts = append(parts, fmt.Sprintf("%d approval needed", st.BackgroundApprovals))
		}

		ui.Text(c, strings.Join(parts, " · ")).FontSize(11.5).FontWeight(500).TextColor(tc.Text).Grow(1).SingleLine()

		if sess.BotID != "" {
			ui.Text(c, "Inspect →").FontSize(11).FontWeight(600).TextColor(tc.Accent)
		}
	})
	if bar.Hovered() {
		bar.Background(tc.Text.Alpha(0.05))
	}
	if bar.Clicked() && sess.BotID != "" {
		a.ShowRightDrawer = true
		a.RightDrawerTab = RightTabBot
		if st.BackgroundApprovals > 0 {
			a.BotDrawerSubTab = "approvals"
		} else {
			a.BotDrawerSubTab = "tasks"
		}
	}
}

// renderBotEmptyState renders the dedicated Bot Home Chat landing view when a
// Bot's Home Chat has no messages yet (instead of the generic project coding landing).
func (a *AppUI) renderBotEmptyState(c *ui.Context, tc theme.ThemeColors, sess *agent.AgentSession) {
	var bot *agent.BotProfile
	if a.Bots != nil {
		bot = a.Bots.Get(sess.BotID)
	}
	botName := "Bot"
	botRole := "Autonomous assistant"
	harness := "RakitKode"
	if bot != nil {
		botName = bot.Name
		if bot.Role != "" {
			botRole = bot.Role
		}
		if bot.AgentType == "acp" {
			harness = "ACP · " + strings.TrimPrefix(bot.AgentID, "agent-")
		}
	}

	ui.Column(c).Grow(1).Fill().Center().Gap(22).Padding(24, 32).Background(tc.Canvas).Children(func() {
		ui.Column(c).AlignItems(ui.Center).Gap(10).Children(func() {
			avBox := ui.Box(c.Key("be-avatar")).Radius(20).Cursor(ui.CursorPointer)
			if bot != nil {
				avBox.Tooltip("Change bot avatar")
			}
			avBox.Children(func() {
				a.renderBotAvatar(c, tc, bot, 76, 20)
			})
			if avBox.Clicked() && bot != nil {
				a.OpenBotAvatarModal(bot.ID)
			}
			ui.Text(c, botName).FontSize(26).FontWeight(600).LetterSpacing(-0.6).TextColor(tc.Text)
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, botRole).FontSize(12.5).TextColor(tc.TextMuted)
				ui.Text(c, "·").FontSize(12.5).TextColor(tc.TextMuted)
				ui.Text(c, harness).FontSize(12).FontWeight(500).TextColor(tc.Accent)
			})
			ui.Text(c, "Chat stays focused on Q&A and decisions. Heavy tasks run in the background (RakitKode or ACP).").
				FontSize(12).TextColor(tc.TextMuted)
		})

		ui.Column(c).MaxWidth(720).FillWidth().Gap(14).AlignItems(ui.Center).Children(func() {
			ui.Row(c).Gap(8).Wrap().Justify(ui.Center).Children(func() {
				if bot != nil {
					newTaskBtn := ui.Row(c.Key("be-new-task")).Height(30).Radius(8).Padding(0, 12).Gap(6).
						AlignItems(ui.Center).Cursor(ui.CursorPointer).
						Background(tc.Accent.Alpha(0.14)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.4))
					newTaskBtn.Children(func() {
						ui.Icon(c, icons.TablerPlus).Width(13).Height(13).TextColor(tc.Accent)
						ui.Text(c, "Run Background Task").FontSize(12).FontWeight(600).TextColor(tc.Accent)
					})
					if newTaskBtn.Clicked() {
						a.OpenNewBotTaskModal(bot.ID)
					}
				}
				inspectBtn := ui.Row(c.Key("be-inspect")).Height(30).Radius(8).Padding(0, 12).Gap(6).
					AlignItems(ui.Center).Cursor(ui.CursorPointer).
					Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border)
				inspectBtn.Children(func() {
					ui.Icon(c, icons.TablerChecklist).Width(13).Height(13).TextColor(tc.IconMuted)
					ui.Text(c, "Inspect Tasks").FontSize(12).FontWeight(500).TextColor(tc.Text)
				})
				if inspectBtn.Clicked() {
					a.ShowRightDrawer = true
					a.RightDrawerTab = RightTabBot
					a.BotDrawerSubTab = "tasks"
				}
			})

			ui.Column(c).FillWidth().Gap(6).Children(func() {
				a.renderChatProcessBar(c, tc, sess)
				a.renderComposerCard(c, tc, true)
			})
		})
	})
}
