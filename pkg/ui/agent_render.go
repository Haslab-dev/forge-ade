package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

// This file restyles the agent chat with ForgeADE's AgentActiveSessionView
// and AgentTaskInputBar recipes at their exact metrics: the centered chat
// lane (--chat-content-max-width), right-aligned rounded-2xl user bubbles,
// the "Working/Worked for Ns" pill with a bordered work block, tool traces,
// the turn-changes card with real diff counts, the 22px glass composer and
// the circular message-action send button.

// chatLaneInset computes the horizontal padding that centers the
// --chat-content-max-width lane (736/928px) in the viewport.
func (as *AgentState) chatLaneInset(c *ui.Context) (float32, float32) {
	lane := float32(736)
	if as.Theme != nil {
		lane = as.Theme.ChatLaneWidth()
	}
	if lane <= 0 {
		return 24, 24 // full width mode
	}
	w, _ := c.Size()
	inset := (w - lane) / 2
	if inset < 24 {
		inset = 24
	}
	return inset, inset
}

// RenderChat renders the conversation view, message bubbles, task checklist, and prompt composer.
func (as *AgentState) RenderChat(c *ui.Context) {
	tc := as.themeColors()
	sess := as.ActiveSession()

	if sess == nil || len(as.Messages) == 0 {
		as.renderAgentHome(c)
		if as.ShowModelPicker {
			as.renderModelPickerModal(c)
		}
		return
	}

	sessionTitle := "Agent Assistant"
	if sess != nil {
		sessionTitle = sess.Title
	}
	as.MdOpts.Theme = as.Theme.MarkdownTheme()

	as.ListState.Key = func(i int) any {
		if i < len(as.Messages) {
			return as.Messages[i].ID
		}
		return i
	}
	as.ListState.FollowEnd = true
	as.ListState.FollowAnimation = &ui.Smooth

	leftInset, rightInset := as.chatLaneInset(c)

	ui.Column(c).Fill().Background(tc.Canvas).Children(func() {
		// Session header: title, status pill, model, Fork
		ui.Row(c).Height(44).Gap(10).Padding(0, leftInset+8, 0, rightInset+8).AlignItems(ui.Center).
			BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
			ui.Text(c, sessionTitle).FontSize(13).FontWeight(600).MaxWidth(380)

			// Status pill
			pill := func(label string, color ui.Color) {
				p := ui.Row(c).Radius(999).Padding(2, 8).Gap(4).AlignItems(ui.Center).
					Background(color.Alpha(0.1)).BorderWidth(1).BorderColor(color.Alpha(0.25))
				p.Children(func() {
					ui.Box(c).Size(6, 6).Radius(3).Background(color)
					ui.Text(c, label).FontSize(11).TextColor(color)
				})
			}
			switch sess.Status {
			case "running":
				pill("Running", tc.Accent)
			case "completed":
				pill("Completed", tc.Success)
			default:
				pill("Idle", tc.SidebarMutedForeground)
			}

			ui.Box(c).Grow(1)
			ui.Text(c, as.ActiveModel).FontSize(11).TextColor(tc.TextMuted)
			if as.IsStreaming {
				ui.Text(c, "Streaming…").FontSize(11).TextColor(tc.Accent)
			}
			fork := composerControlChip(c, tc, TablerGitFork, "Fork", "", false)
			if fork.Clicked() && sess != nil {
				as.NewSession(fmt.Sprintf("%s (fork)", sess.Title))
			}
		})

		// Message list: the chat lane, right padding inset for the drawer
		ui.List(c, &as.ListState, len(as.Messages), func(i int) {
			if i >= len(as.Messages) {
				return
			}
			m := as.Messages[i]
			if m.Role == "user" {
				// User bubble: right-aligned max-w-[82%] rounded-2xl
				// border-border/50 bg-message px-3.5 py-2.5 shadow-2xs
				ui.Row(c.Key("urow")).Padding(8, leftInset, 8, 96).Justify(ui.End).Children(func() {
					bubble := ui.Column(c).Padding(10, 14).Radius(18).
						Background(tc.MessageSurface).BorderWidth(1).BorderColor(tc.Border.Alpha(0.5)).
						MaxWidth(600)
					bubble.Children(func() {
						ui.Text(c, m.Text).Selectable().FontSize(14).TextColor(tc.MessageForeground)
					})
					bubble.Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.06))
				})
			} else {
				ui.Column(c.Key("arow")).Padding(8, leftInset, 8, 96).Gap(6).MaxWidth(860).Children(func() {
					// "Working / Worked for Ns" pill over the work block
					hasWork := m.Thinking != "" || m.IsThinking || len(m.Tools) > 0
					if hasWork {
						as.renderWorkPill(c, tc, m)
						// Work block body: pl-2.5 border-l-2 ml-2
						work := ui.Column(c).Gap(4).Padding(0, 0, 0, 10).
							BorderWidth(0, 0, 0, 2).BorderColor(tc.Border.Alpha(0.6))
						work.Children(func() {
							if m.Thinking != "" || m.IsThinking {
								as.renderThinkingBlock(c, tc, m)
							}
							if len(m.Tools) > 0 {
								as.renderFoldedToolTraces(c, tc, m)
							}
						})
					}

					// Turn Changes Card
					if len(m.Changes) > 0 {
						as.renderTurnChangesCard(c, tc, m)
					}

					// Pending Approval Bar
					if m.PendingApproval != nil && !m.PendingApproval.Resolved {
						as.renderApprovalBar(c, tc, m)
					}

					// Response Markdown body
					if m.Text != "" {
						if m.Done {
							markdown.View(c, m.Text, &as.MdOpts)
						} else {
							markdown.Stream(c, m.Text, &as.MdOpts)
						}
					}
				})
			}
		}).Grow(1).Padding(16, 0, 16, 0).Gap(14).Justify(ui.End)

		// Floating Task Checklist (docked above composer)
		if as.TaskManager != nil {
			as.TaskManager.RenderChecklist(c)
		}

		// Mid-turn Steering Queue Tray
		if len(as.SteerQueue) > 0 {
			as.renderSteerQueueTray(c)
		}

		// Composer: the 22px glass card
		as.renderComposer(c, tc, sess)
	})
}

// renderWorkPill is the "Working/Worked for Ns" disclosure pill:
// rounded-lg border-border/60 bg-surface/60 px-2.5 py-1 text-ui-xs.
func (as *AgentState) renderWorkPill(c *ui.Context, tc ThemeColors, m *ChatMessage) {
	label := "Worked"
	running := !m.Done
	if running {
		label = "Working"
	} else if m.ThinkingTimeSec > 0 {
		label = fmt.Sprintf("Worked for %.0fs", m.ThinkingTimeSec)
	}
	summary := ""
	if len(m.Tools) > 0 {
		summary = summarizeTools(m.Tools)
	}

	open := m.ShowTools || running
	pill := ui.Row(c).Radius(8).Padding(4, 10).Gap(6).AlignItems(ui.Center).
		Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border.Alpha(0.6))
	pill.Children(func() {
		if running {
			ui.Icon(c, TablerLoader).Width(14).Height(14).TextColor(tc.Accent)
			ui.Text(c, label).FontSize(12).FontWeight(500).TextColor(tc.Accent)
		} else {
			done := ui.Box(c).Size(14, 14).Radius(7).AlignItems(ui.Center).Justify(ui.Center).
				Background(tc.Success.Alpha(0.15))
			done.Children(func() {
				ui.Icon(c, TablerCheck).Width(9).Height(9).TextColor(tc.Success)
			})
			ui.Text(c, label).FontSize(12).FontWeight(500)
		}
		if summary != "" {
			ui.Text(c, "· "+summary).FontSize(12).TextColor(tc.TextMuted)
		}
		ui.Box(c).Grow(1)
		chev := TablerChevronDown
		if open && !running {
			chev = TablerChevronUp
		}
		ui.Icon(c, chev).Width(12).Height(12).TextColor(tc.IconMuted)
	})
	if pill.Clicked() {
		m.ShowTools = !m.ShowTools
	}
}

// renderThinkingBlock renders the collapsible thinking trace: min-h-6
// rounded-md px-1 row with a mono 11px label, body in a bordered card.
func (as *AgentState) renderThinkingBlock(c *ui.Context, tc ThemeColors, m *ChatMessage) {
	label := "Thought Process"
	if m.ThinkingTimeSec > 0 {
		label = fmt.Sprintf("Thinking (%.1fs)", m.ThinkingTimeSec)
	}

	ui.Column(c).FillWidth().Children(func() {
		header := ui.Row(c).Radius(6).Padding(2, 4).Gap(6).AlignItems(ui.Center).MinHeight(24)
		header.Children(func() {
			ic := ui.Icon(c, TablerBrain).Width(14).Height(14).TextColor(tc.IconMuted)
			lbl := ui.Text(c, label).FontSize(11).TextColor(tc.SecondaryLabel)
			if m.IsThinking {
				ic.TextColor(tc.Accent)
				lbl.TextColor(tc.Accent)
			}
			ui.Box(c).Grow(1)
			chev := TablerChevronDown
			if m.ShowThinking {
				chev = TablerChevronUp
			}
			ui.Icon(c, chev).Width(12).Height(12).TextColor(tc.IconMuted)
		})
		if header.Hovered() {
			header.Background(tc.Text.Alpha(0.04))
		}
		if header.Clicked() {
			m.ShowThinking = !m.ShowThinking
		}

		if m.ShowThinking && m.Thinking != "" {
			ui.Box(c).Padding(10).Radius(8).Background(tc.Surface.Alpha(0.3)).BorderWidth(1).BorderColor(tc.Border.Alpha(0.5)).Children(func() {
				ui.Text(c, m.Thinking).FontSize(11).TextColor(tc.TextMuted)
			})
		}
	})
}

// renderFoldedToolTraces renders summarized tool calls (WorkLogRow recipe:
// min-h-6 rounded-md disclosure rows, mono details indented with a left border).
func (as *AgentState) renderFoldedToolTraces(c *ui.Context, tc ThemeColors, m *ChatMessage) {
	summary := summarizeTools(m.Tools)

	ui.Column(c).FillWidth().Gap(2).Children(func() {
		for _, tool := range m.Tools {
			thisTool := tool
			open := m.ShowTools
			row := ui.Row(c.Key("tool-"+fmt.Sprint(thisTool.ID))).Radius(6).Padding(2, 4).Gap(6).AlignItems(ui.Center).MinHeight(24)
			row.Children(func() {
				chev := TablerChevronRight
				if open {
					chev = TablerChevronDown
				}
				ui.Icon(c, chev).Width(12).Height(12).TextColor(tc.IconMuted)
				toolIcon := TablerTerminal
				nameLower := strings.ToLower(thisTool.ToolName)
				if strings.Contains(nameLower, "read") {
					toolIcon = TablerFileText
				} else if strings.Contains(nameLower, "edit") || strings.Contains(nameLower, "write") {
					toolIcon = TablerFilePen
				} else if strings.Contains(nameLower, "search") {
					toolIcon = TablerSearch
				}
				ui.Icon(c, toolIcon).Width(13).Height(13).TextColor(tc.IconMuted)
				ui.Text(c, thisTool.ToolName).FontSize(13)
				preview := thisTool.Command
				if len(preview) > 60 {
					preview = preview[:57] + "…"
				}
				if preview != "" {
					ui.Text(c, preview).FontSize(11).TextColor(tc.TextMuted).MaxWidth(380)
				}
				ui.Box(c).Grow(1)
				switch thisTool.Status {
				case "completed":
					ui.Text(c, "done").FontSize(10).TextColor(tc.Success)
				case "failed":
					ui.Text(c, "failed").FontSize(10).TextColor(tc.Error)
				default:
					ui.Text(c, "running").FontSize(10).TextColor(tc.Accent)
				}
			})
			if row.Hovered() {
				row.Background(tc.Text.Alpha(0.04))
			}
			if row.Clicked() {
				m.ShowTools = !m.ShowTools
			}

			if open && thisTool.Output != "" {
				ui.Box(c).Padding(6, 0, 6, 30).
					BorderWidth(0, 0, 0, 1).BorderColor(tc.Border.Alpha(0.5)).Children(func() {
					out := thisTool.Output
					if len(out) > 600 {
						out = out[:600] + "…"
					}
					ui.Text(c, out).FontSize(11).TextColor(tc.TextMuted)
				})
			}
		}
		_ = summary
	})
}

// renderTurnChangesCard is TurnChangesCard.tsx: rounded-lg border-border/70
// bg-surface/40 with a header, real +N/−N totals and per-file rows.
func (as *AgentState) renderTurnChangesCard(c *ui.Context, tc ThemeColors, m *ChatMessage) {
	// Compute the real diff counts of entries still unknown, once.
	as.mu.Lock()
	need := false
	keys := make([]string, 0, len(m.Changes))
	for _, ch := range m.Changes {
		if ch.Additions < 0 {
			key := fmt.Sprintf("%d/%s", m.ID, ch.Path)
			if !as.statsPending[key] {
				need = true
				as.statsPending[key] = true
				keys = append(keys, key)
			}
		}
	}
	as.mu.Unlock()
	if need {
		changes := m.Changes
		go func() {
			as.computeChangeStats(changes)
			if as.Invalidate != nil {
				as.Invalidate()
			}
		}()
	}

	totalAdd, totalDel := 0, 0
	for _, ch := range m.Changes {
		if ch.Additions > 0 {
			totalAdd += ch.Additions
		}
		if ch.Deletions > 0 {
			totalDel += ch.Deletions
		}
	}

	card := ui.Column(c).Radius(8).Background(tc.Surface.Alpha(0.4)).BorderWidth(1).BorderColor(tc.Border.Alpha(0.7))
	card.Children(func() {
		// Header
		ui.Row(c).Padding(6, 12).Gap(6).AlignItems(ui.Center).
			BorderWidth(0, 0, 1, 0).BorderColor(tc.Border.Alpha(0.4)).Children(func() {
			ui.Icon(c, TablerFileCode).Width(13).Height(13).TextColor(tc.Accent)
			ui.Text(c, fmt.Sprintf("Modified %d files in this turn", len(m.Changes))).FontSize(12).FontWeight(500)
			ui.Box(c).Grow(1)
			if totalAdd > 0 {
				ui.Text(c, fmt.Sprintf("+%d", totalAdd)).FontSize(11).TextColor(tc.DiffAdditionForeground)
			}
			if totalDel > 0 {
				ui.Text(c, fmt.Sprintf("−%d", totalDel)).FontSize(11).TextColor(tc.DiffDeletionForeground)
			}
		})
		// File rows
		ui.Column(c).Padding(8).Gap(4).Children(func() {
			for _, ch := range m.Changes {
				ui.Row(c.Key("tchg-"+ch.Path)).Radius(6).Padding(6, 10).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, FileIconSVG(ch.Path)).Width(13).Height(13).TextColor(tc.IconMuted)
					ui.Text(c, ch.Path).FontSize(12).Grow(1)
					if ch.Additions >= 0 {
						DiffStatText(c, tc, ch.Additions, ch.Deletions)
					}
				})
			}
		})
	})
}

// renderApprovalBar is PendingApprovalCard: rounded-xl border-warning/40
// bg-warning/5 with Approve/Deny, wired to the ACP permission request.
func (as *AgentState) renderApprovalBar(c *ui.Context, tc ThemeColors, m *ChatMessage) {
	appr := m.PendingApproval

	card := ui.Column(c).Radius(12).Padding(14).Gap(8).
		Background(tc.Warning.Alpha(0.05)).BorderWidth(1).BorderColor(tc.Warning.Alpha(0.4))
	card.Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, TablerShieldAlert).Width(16).Height(16).TextColor(tc.Warning)
			ui.Column(c).Grow(1).Children(func() {
				ui.Text(c, "Action Requires Confirmation").FontSize(13).FontWeight(600)
				ui.Text(c, appr.Summary).FontSize(12).TextColor(tc.TextMuted)
			})
		})
		ui.Row(c).Gap(6).Justify(ui.End).AlignItems(ui.Center).Children(func() {
			allow := chipRow(c, tc)
			allow.Background(tc.Warning).Radius(6).Height(26)
			allow.Children(func() {
				ui.Text(c, "Approve").FontSize(12).FontWeight(500)
			})
			if allow.Clicked() {
				appr.Resolved = true
				appr.Approved = true
				as.respondACPPermission(true)
			}
			always := composerControlChip(c, tc, TablerShieldCheck, "Always Allow", "", false)
			if always.Clicked() {
				appr.Resolved = true
				appr.Approved = true
				as.ExecutionMode = ModeBypass
				as.respondACPPermission(true)
			}
			deny := composerControlChip(c, tc, TablerX, "Deny", "", true)
			if deny.Clicked() {
				appr.Resolved = true
				appr.Approved = false
				as.respondACPPermission(false)
			}
		})
	})
}

// respondACPPermission answers a pending ACP permission request for the
// active session, if one is open (the real approval path).
func (as *AgentState) respondACPPermission(allow bool) {
	if as.ACPMgr == nil {
		return
	}
	as.ACPMgr.mu.Lock()
	req := as.ACPMgr.ActiveReq
	sessID := as.ActiveSessionID
	as.ACPMgr.mu.Unlock()
	if req != nil && (req.SessionID == sessID || sessID == "") {
		_ = as.ACPMgr.RespondPermission(req.SessionID, req.RequestID, allow)
		if as.Invalidate != nil {
			as.Invalidate()
		}
	}
}

// renderSteerQueueTray renders pending mid-turn steering instructions
// (rounded-2xl bg-panel/95 tray with grip rows).
func (as *AgentState) renderSteerQueueTray(c *ui.Context) {
	tc := as.themeColors()

	tray := ui.Column(c).Padding(8).Radius(16).Background(tc.SurfaceRaised).
		BorderWidth(1).BorderColor(tc.Border.Alpha(0.8)).Gap(4)
	tray.Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
			ui.Icon(c, TablerGripVertical).Width(13).Height(13).TextColor(tc.IconMuted)
			ui.Text(c, fmt.Sprintf("Queued Instructions (%d)", len(as.SteerQueue))).FontSize(12).FontWeight(500)
			ui.Text(c, "· auto-steers when turn completes").FontSize(11).TextColor(tc.TextMuted)
			ui.Box(c).Grow(1)
			clear := composerControlChip(c, tc, TablerX, "Clear all", "", false)
			if clear.Clicked() {
				as.SteerQueue = as.SteerQueue[:0]
			}
		})
		for idx, item := range as.SteerQueue {
			thisIdx, thisItem := idx, item
			ui.Row(c.Key(fmt.Sprintf("steer-%d", thisIdx))).Gap(6).Padding(2, 4).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, TablerGripVertical).Width(12).Height(12).TextColor(tc.IconMuted)
				ui.Text(c, thisItem).FontSize(12).MaxWidth(420).Grow(1)
				steerNow := composerControlChip(c, tc, TablerBolt, "Steer", "", false)
				if steerNow.Clicked() {
					as.SteerQueue = append(as.SteerQueue[:thisIdx], as.SteerQueue[thisIdx+1:]...)
					as.triggerAgentTurn(thisItem)
				}
				del := composerControlChip(c, tc, TablerX, "", "", true)
				if del.Clicked() {
					as.SteerQueue = append(as.SteerQueue[:thisIdx], as.SteerQueue[thisIdx+1:]...)
				}
			})
		}
	})
}

// renderComposer is AgentTaskInputBar: the 22px-radius glass card with the
// prompt field, control chips and the circular message-action send button.
func (as *AgentState) renderComposer(c *ui.Context, tc ThemeColors, sess *AgentSession) {
	inset, rightInset := as.chatLaneInset(c)

	ui.Column(c).Padding(0, inset, 10, rightInset).Gap(6).Children(func() {
		// Slash command pills above the card
		ui.Row(c).Gap(4).Wrap().Children(func() {
			for _, cmd := range []string{"/goal", "/plan", "/tasks", "/vibe", "/model", "/clear", "/status", "/help"} {
				thisCmd := cmd
				chip := ui.Row(c.Key("slash-"+thisCmd)).Radius(999).Padding(3, 10).AlignItems(ui.Center).
					BorderWidth(1).BorderColor(tc.Border.Alpha(0.6))
				chip.Children(func() {
					ui.Text(c, thisCmd).FontSize(11).TextColor(tc.TextMuted)
				})
				if chip.Hovered() {
					chip.Background(tc.Text.Alpha(0.04))
				}
				if chip.Clicked() {
					as.InputText = thisCmd + " "
				}
			}
			ui.Box(c).Grow(1)
			tokens, pct := as.ContextUsage()
			ui.Text(c, fmt.Sprintf("%.1fk tokens (%d%%)", float32(tokens)/1000.0, pct)).FontSize(10).TextColor(tc.TextMuted)
		})

		// The composer card: --composer-radius 22, border, big soft shadow
		card := ui.Column(c).Radius(22).Background(tc.Surface).
			BorderWidth(1).BorderColor(tc.Border).
			Shadow(0, 12, 28, -18, ui.RGBA(0, 0, 0, 0.4)).
			InsetShadow(0, 1, 0, 0, ui.RGBA(255, 255, 255, 0.06))
		card.Children(func() {
			// Prompt field
			ui.Row(c).Padding(10, 16, 6, 16).Children(func() {
				in := ui.TextInput(c, &as.InputText).Placeholder("Ask agent, enter instructions, or /help...").Grow(1)
				if in.Submitted() {
					as.SubmitMessage()
				}
			})

			// Control chips row
			ui.Row(c).Gap(6).Padding(6, 12, 12, 12).AlignItems(ui.Center).Children(func() {
				// Permission mode chip
				var modeIcon *ui.SVG
				modeLabel := ""
				modeColor := tc.Text
				switch as.ExecutionMode {
				case ModeAsk:
					modeIcon, modeLabel = TablerHand, "Ask"
				case ModeCode:
					modeIcon, modeLabel = TablerPencil, "Edits"
				case ModePlan:
					modeIcon, modeLabel = TablerShieldCheck, "Auto"
				default:
					modeIcon, modeLabel = TablerShieldAlert, "Full Access"
					modeColor = tc.FullAccess
				}
				modeBtn := chipRow(c, tc)
				modeBtn.Children(func() {
					ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, modeIcon).Width(13).Height(13).TextColor(modeColor)
						ui.Text(c, modeLabel).FontSize(12).FontWeight(500).TextColor(modeColor)
					})
				})
				if modeBtn.Clicked() {
					as.CycleExecutionMode()
				}

				// Reasoning effort chip
				effortLabel := "Med"
				switch as.ReasoningEffort {
				case EffortOff:
					effortLabel = "Off"
				case EffortLow:
					effortLabel = "Low"
				case EffortHigh:
					effortLabel = "High"
				case EffortMax:
					effortLabel = "Max"
				}
				effortBtn := composerControlChip(c, tc, TablerBrain, effortLabel, "", false)
				if effortBtn.Clicked() {
					as.CycleReasoningEffort()
				}

				// Model chip
				disp := as.ActiveModel
				if len(disp) > 18 {
					disp = disp[:15] + "…"
				}
				modelBtn := chipRow(c, tc)
				modelBtn.Children(func() {
					ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, TablerSparkles).Width(13).Height(13).TextColor(tc.Accent)
						ui.Text(c, disp).FontSize(12).FontWeight(500).SingleLine()
						ui.Icon(c, TablerChevronDown).Width(11).Height(11).TextColor(tc.IconMuted)
					})
				})
				if modelBtn.Clicked() {
					as.ShowModelPicker = !as.ShowModelPicker
				}

				// Auto-Verify chip
				verifyBtn := chipRow(c, tc)
				if as.AutoVerify {
					verifyBtn.Background(tc.Success.Alpha(0.1)).BorderWidth(1).BorderColor(tc.Success.Alpha(0.35))
					verifyBtn.Children(func() {
						ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
							ui.Icon(c, TablerShieldCheck).Width(13).Height(13).TextColor(tc.Success)
							ui.Text(c, "Verify").FontSize(12).FontWeight(500).TextColor(tc.Success)
						})
					})
				} else {
					verifyBtn.Children(func() {
						ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
							ui.Icon(c, TablerShieldCheck).Width(13).Height(13).TextColor(tc.TextMuted)
							ui.Text(c, "Verify").FontSize(12).FontWeight(500).TextColor(tc.Text)
						})
					})
				}
				if verifyBtn.Clicked() {
					as.AutoVerify = !as.AutoVerify
				}

				ui.Box(c).Grow(1)

				// Send / Queue / Stop: size-8 rounded-full bg-message-action
				send := ui.Row(c).Size(32, 32).Radius(999).AlignItems(ui.Center).Justify(ui.Center).
					Background(tc.MessageAction).
					Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.2)).
					InsetShadow(0, 1, 0, 0, ui.RGBA(255, 255, 255, 0.16))
				send.Children(func() {
					if as.IsStreaming {
						send.Background(tc.Error.Alpha(0.9))
						ui.Icon(c, TablerSquare).Width(13).Height(13).TextColor(tc.ErrorForeground)
					} else {
						ui.Icon(c, TablerChevronUp).Width(16).Height(16).TextColor(tc.MessageActionForeground)
					}
				})
				if send.Clicked() {
					if as.IsStreaming {
						as.mu.Lock()
						as.IsStreaming = false
						if len(as.Messages) > 0 {
							as.Messages[len(as.Messages)-1].Done = true
						}
						as.mu.Unlock()
					} else {
						as.SubmitMessage()
					}
				}
				_ = sess
			})
		})

		// Model picker popover
		if as.ShowModelPicker {
			as.renderModelPickerModal(c)
		}
	})
}

// chipRow is a bare ComposerControl container: rounded-lg h-7 px-2.5 with
// hover styling, for callers that build their children once.
func chipRow(c *ui.Context, tc ThemeColors) ui.Element {
	row := ui.Row(c).Height(28).Radius(8).Padding(0, 10).Gap(5).AlignItems(ui.Center)
	if row.Hovered() {
		row.Background(tc.Text.Alpha(0.05)).BorderWidth(1).BorderColor(tc.Border.Alpha(0.4))
	}
	return row
}

// composerControlChip is ComposerControl from composer-ui.tsx: rounded-lg
// border-transparent font-medium hover:bg-surface-hover h-7 px-2.5 text-ui-sm.
func composerControlChip(c *ui.Context, tc ThemeColors, icon *ui.SVG, label, kbd string, danger bool) ui.Element {
	row := ui.Row(c).Height(28).Radius(8).Padding(0, 10).Gap(5).AlignItems(ui.Center)
	row.Children(func() {
		if icon != nil && label != "" {
			color := tc.TextMuted
			if danger {
				color = tc.Error
			}
			ui.Icon(c, icon).Width(13).Height(13).TextColor(color)
		}
		if label != "" {
			color := tc.Text
			if danger {
				color = tc.Error
			}
			ui.Text(c, label).FontSize(12).FontWeight(500).TextColor(color)
		}
		if kbd != "" {
			ui.Text(c, kbd).FontSize(11).TextColor(tc.SecondaryLabel)
		}
	})
	if danger && row.Hovered() {
		row.Background(tc.Error.Alpha(0.1))
	} else if row.Hovered() {
		row.Background(tc.Text.Alpha(0.05)).BorderWidth(1).BorderColor(tc.Border.Alpha(0.4))
	}
	return row
}

// themeColors resolves the active ForgeADE roles; nil-safe when the theme
// manager is not attached yet.
func (as *AgentState) themeColors() ThemeColors {
	if as.Theme != nil {
		return as.Theme.Colors()
	}
	return PaletteColors(ThemePaletteDefault, true)
}

// renderAgentHome is AgentHomeView: the centered brand glyph, the
// dotted-underline workspace heading, the landing composer and the
// quick-start suggestion chips.
func (as *AgentState) renderAgentHome(c *ui.Context) {
	tc := as.themeColors()
	projectName := filepath.Base(as.workspacePath)
	if projectName == "" || projectName == "." {
		projectName = "General"
	}

	ui.Column(c).Grow(1).Fill().Center().Gap(20).Padding(24, 32).Background(tc.Canvas).Children(func() {
		// Brand glyph + hero heading: text-[28px]/[34px] font-semibold
		// tracking-[-0.03em]
		ui.Column(c).AlignItems(ui.Center).Gap(14).Children(func() {
			ui.Icon(c, ForgeSparkSVG).Width(54).Height(54).TextColor(tc.Text)
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "What should we do in").FontSize(28).FontWeight(600).LetterSpacing(-0.6)
				ui.Text(c, projectName+"?").FontSize(28).FontWeight(600).LetterSpacing(-0.6).TextColor(tc.Accent)
			})
		})

		// Landing composer: 680px, --composer-radius 22
		card := ui.Column(c).Width(680).Radius(22).BorderWidth(1).BorderColor(tc.Border).
			Background(tc.Surface).
			Shadow(0, 12, 28, -18, ui.RGBA(0, 0, 0, 0.4)).
			InsetShadow(0, 1, 0, 0, ui.RGBA(255, 255, 255, 0.06))
		card.Children(func() {
			// Landing context strip: workspace + branch chips
			ui.Row(c).Gap(8).Padding(12, 16, 0, 16).AlignItems(ui.Center).Children(func() {
				wsChip := ui.Row(c).Radius(8).Padding(4, 10).Gap(5).AlignItems(ui.Center).
					Background(tc.Text.Alpha(0.05))
				wsChip.Children(func() {
					ui.Icon(c, TablerFolder).Width(13).Height(13).TextColor(tc.IconMuted)
					ui.Text(c, projectName).FontSize(12).FontWeight(500)
				})
				if a := as.engine; a != nil && a.GitEngine != nil {
					branchChip := ui.Row(c).Radius(8).Padding(4, 10).Gap(5).AlignItems(ui.Center).
						Background(tc.Text.Alpha(0.05))
					branchChip.Children(func() {
						ui.Icon(c, TablerGitBranch).Width(12).Height(12).TextColor(tc.IconMuted)
						ui.Text(c, "main").FontSize(12).TextColor(tc.TextMuted)
					})
				}
				ui.Box(c).Grow(1)
			})

			// Input field
			ui.Row(c).Padding(8, 16).Children(func() {
				in := ui.TextInput(c, &as.InputText).Placeholder("Ask anything, @ threads or files, $ skills, or / commands").Grow(1)
				if in.Submitted() {
					as.SubmitMessage()
				}
			})

			// Bottom control row
			ui.Row(c).Gap(6).Padding(6, 12, 12, 12).AlignItems(ui.Center).Children(func() {
				var modeIcon *ui.SVG
				modeLabel := ""
				switch as.ExecutionMode {
				case ModeAsk:
					modeIcon, modeLabel = TablerHand, "Ask"
				case ModeCode:
					modeIcon, modeLabel = TablerPencil, "Edits"
				case ModePlan:
					modeIcon, modeLabel = TablerShieldCheck, "Auto"
				default:
					modeIcon, modeLabel = TablerShieldAlert, "Full Access"
				}
				execPill := composerControlChip(c, tc, modeIcon, modeLabel, "", false)
				if execPill.Clicked() {
					as.CycleExecutionMode()
				}

				reasonPill := composerControlChip(c, tc, TablerBrain, as.ReasoningEffortLabel(), "", false)
				if reasonPill.Clicked() {
					as.CycleReasoningEffort()
				}

				modelChip := chipRow(c, tc)
				modelChip.Children(func() {
					ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, TablerSparkles).Width(13).Height(13).TextColor(tc.Accent)
						ui.Text(c, as.ActiveModel).FontSize(12).FontWeight(500).MaxWidth(140).SingleLine()
					})
				})
				if modelChip.Clicked() {
					as.ShowModelPicker = true
				}

				ui.Box(c).Grow(1)

				send := ui.Row(c).Size(32, 32).Radius(999).AlignItems(ui.Center).Justify(ui.Center).
					Background(tc.MessageAction).
					Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.2)).
					InsetShadow(0, 1, 0, 0, ui.RGBA(255, 255, 255, 0.16))
				send.Children(func() {
					ui.Icon(c, TablerChevronUp).Width(16).Height(16).TextColor(tc.MessageActionForeground)
				})
				if send.Clicked() {
					as.SubmitMessage()
				}
			})
		})

		// Quick-start suggestion chips: rounded-full border-border/70
		// bg-surface/60 px-3 py-1.5 text-ui-xs
		ui.Row(c).Gap(8).Wrap().Justify(ui.Center).AlignItems(ui.Center).Children(func() {
			suggestions := []struct {
				icon   *ui.SVG
				label  string
				prompt string
			}{
				{TablerCompass, "Explore architecture", "Summarize the architecture of this repository, key packages, and entry points."},
				{TablerBug, "Find & fix a bug", "Help me diagnose and fix an issue in the codebase: "},
				{TablerFlask, "Run & improve tests", "Run the test suite, identify any failing or missing unit tests, and propose improvements."},
				{TablerGitPullRequest, "Review working tree", "Review uncommitted git changes in this workspace for bugs, edge cases, and style."},
			}
			for _, s := range suggestions {
				thisS := s
				chip := ui.Row(c.Key("sugg-"+thisS.label)).Radius(999).Padding(6, 12).Gap(6).AlignItems(ui.Center).
					Background(tc.Surface.Alpha(0.6)).BorderWidth(1).BorderColor(tc.Border.Alpha(0.7))
				chip.Children(func() {
					ui.Icon(c, thisS.icon).Width(14).Height(14).TextColor(tc.IconMuted)
					ui.Text(c, thisS.label).FontSize(12).FontWeight(500)
				})
				if chip.Hovered() {
					chip.Background(tc.Text.Alpha(0.06))
				}
				if chip.Clicked() {
					as.InputText = thisS.prompt
				}
			}
		})

		// Model picker popover
		if as.ShowModelPicker {
			as.renderModelPickerModal(c)
		}
	})
}

// renderModelPickerModal renders the model popover above the composer.
func (as *AgentState) renderModelPickerModal(c *ui.Context) {
	tc := as.themeColors()

	ui.Column(c).Absolute().Bottom(64).Right(inset64(as)).Width(300).Padding(6).Radius(12).
		Background(tc.SurfaceOverlay).BorderWidth(1).BorderColor(tc.Border).
		Shadow(0, 12, 32, -8, ui.RGBA(0, 0, 0, 0.35)).Children(func() {
		ui.Row(c).Padding(6, 8).AlignItems(ui.Center).Gap(6).Children(func() {
			ui.Text(c, "Select AI Model").FontSize(12).FontWeight(600)
			ui.Box(c).Grow(1)
		})
		for _, model := range as.AvailableModels {
			thisModel := model
			isSelected := (as.ActiveModel == thisModel)
			row := ui.Row(c.Key("model-"+thisModel)).Height(28).Radius(6).Padding(0, 8).Gap(6).AlignItems(ui.Center)
			row.Children(func() {
				if isSelected {
					ui.Icon(c, TablerCheck).Width(13).Height(13).TextColor(tc.Accent)
				} else {
					ui.Icon(c, TablerSparkles).Width(13).Height(13).TextColor(tc.IconMuted)
				}
				ui.Text(c, thisModel).FontSize(12).Grow(1)
			})
			if isSelected {
				row.Background(tc.Accent.Alpha(0.1))
			} else if row.Hovered() {
				row.Background(tc.Text.Alpha(0.04))
			}
			if row.Clicked() {
				as.ActiveModel = thisModel
				as.ShowModelPicker = false
			}
		}
	})
}

// inset64 is a small helper naming the model picker's anchor inset.
func inset64(as *AgentState) float32 {
	_ = as
	return 64
}
