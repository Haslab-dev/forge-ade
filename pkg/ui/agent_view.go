package ui

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/agent"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/icons"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/theme"
)

// renderAgentSurface renders the center chat viewport, collapsible terminal & right drawer.
func (a *AppUI) renderAgentSurface(c *ui.Context) {
	tc := a.ThemeMgr.Colors()

	// Center Column: Main Chat Container (or CLI surface) + Collapsible Bottom Terminal
	ui.Column(c).Grow(1).FillHeight().Children(func() {
		// Agent Main Container
		ui.Column(c).Grow(1).FillHeight().Background(tc.Canvas).Children(func() {
			if a.ActiveTaskKind == "cli" && a.CLIMgr != nil && a.CLIMgr.ActiveSession() != nil {
				a.renderTerminalSessionSurface(c, a.CLIMgr.ActiveID)
			} else {
				a.renderAgentChat(c)
			}
		})

		// Collapsible Bottom Terminal with interactive drag resize handle
		if a.ShowBottomTerminal {
			TopResizeHandle(c, &a.BottomTerminalHeight, 120, 600, 240)
			a.renderBottomTerminal(c)
		}
	})

	// Collapsible Right Drawer with interactive drag resize handle
	if a.ShowRightDrawer {
		maxRight := float32(750)
		if winW, _ := c.Size(); winW > 0 && winW*0.6 < maxRight {
			maxRight = winW * 0.6
		}
		LeftResizeHandle(c, &a.RightDrawerWidth, 280, maxRight, RightDrawerDefault)
		a.renderRightDrawer(c)
	}
}

// chatLaneInset computes the horizontal padding that centers the chat lane in the viewport.
func (a *AppUI) chatLaneInset(c *ui.Context) (float32, float32) {
	lane := float32(736)
	if a.ThemeMgr != nil {
		lane = a.ThemeMgr.ChatLaneWidth()
	}
	if lane <= 0 {
		return 24, 24
	}
	w, _ := c.Size()
	inset := (w - lane) / 2
	if inset < 24 {
		inset = 24
	}
	return inset, inset
}

// renderAgentChat renders the active conversation view, message bubbles, task checklist, and prompt composer.
func (a *AppUI) renderAgentChat(c *ui.Context) {
	tc := a.ThemeMgr.Colors()
	as := a.Agent
	if as == nil {
		return
	}
	sess := as.ActiveSession()

	if sess == nil || len(as.Messages) == 0 {
		a.renderAgentHome(c)
		if as.ShowModelPicker {
			a.renderModelPickerModal(c)
		}
		return
	}

	sessionTitle := "Agent Assistant"
	if sess != nil {
		sessionTitle = sess.Title
	}
	as.MdOpts.Theme = a.ThemeMgr.MarkdownTheme()

	as.ListState.Key = func(i int) any {
		if i < len(as.Messages) {
			return as.Messages[i].ID
		}
		return i
	}
	as.ListState.FollowEnd = true
	as.ListState.FollowAnimation = &ui.Smooth

	leftInset, rightInset := a.chatLaneInset(c)

	// The chat fills a box so the floating task tracker can overlay it at
	// the top-right, as forge-ade's FloatingTaskTracker does.
	ui.Box(c).Fill().Children(func() {
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
					runningPill := ui.Row(c).Radius(999).Padding(2, 8).Gap(5).AlignItems(ui.Center).
						Background(tc.Accent.Alpha(0.1)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.25))
					runningPill.Children(func() {
						ui.Spinner(c).Size(10, 10).Label("Running")
						ui.Text(c, "Running").FontSize(11).TextColor(tc.Accent)
					})
				case "awaiting-approval", "waiting_permission":
					pill("Waiting for approval", tc.Warning)
				case "error":
					pill("Error", tc.Error)
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
				fork := composerControlChip(c, tc, icons.TablerGitFork, "Fork", "", false)
				if fork.Clicked() && sess != nil {
					forked := as.ForkSession(sess)
					if forked != nil {
						as.SaveSessionToDisk(forked)
					}
				}
			})

			// Message list: the chat lane
			ui.List(c, &as.ListState, len(as.Messages), func(i int) {
				if i >= len(as.Messages) {
					return
				}
				m := as.Messages[i]
				if m.Role == "user" {
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
						// "Working / Worked for Ns" pill
						hasWork := m.Thinking != "" || m.IsThinking || len(m.Tools) > 0
						if hasWork {
							a.renderWorkPill(c, tc, m)
							work := ui.Column(c).Gap(4).Padding(0, 0, 0, 10).
								BorderWidth(0, 0, 0, 2).BorderColor(tc.Border.Alpha(0.6))
							work.Children(func() {
								if m.Thinking != "" || m.IsThinking {
									a.renderThinkingBlock(c, tc, m)
								}
								if len(m.Tools) > 0 {
									a.renderFoldedToolTraces(c, tc, m)
								}
							})
						}

						// Turn Changes Card
						if len(m.Changes) > 0 {
							a.renderTurnChangesCard(c, tc, m)
						}

						// Pending Approval Bar
						if m.PendingApproval != nil && !m.PendingApproval.Resolved {
							a.renderApprovalBar(c, tc, m)
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

			// Mid-turn Steering Queue Tray
			if len(as.SteerQueue) > 0 {
				a.renderSteerQueueTray(c)
			}

			// Composer
			a.renderComposer(c, tc, sess)
		})

		// Floating Task Tracker overlay (pill <-> card), top-right
		a.renderTaskTracker(c, tc)
	})
}

func (a *AppUI) renderWorkPill(c *ui.Context, tc theme.ThemeColors, m *agent.ChatMessage) {
	running := !m.Done
	totalSteps := m.TotalSteps
	if totalSteps == 0 {
		totalSteps = len(m.Tools)
	}

	// Live elapsed while the turn runs; the app ticks once a second while
	// streaming, so this label counts up like forge-ade's "Working for 12s".
	elapsedSec := m.ThinkingTimeSec
	if running && !m.TurnStartedAt.IsZero() {
		elapsedSec = time.Since(m.TurnStartedAt).Seconds()
	}

	label := "Worked"
	if running {
		label = "Working…"
		if elapsedSec >= 1 {
			label = fmt.Sprintf("Working for %.0fs…", elapsedSec)
		}
	} else {
		durationSec := m.ThinkingTimeSec
		if durationSec <= 0 {
			durationSec = elapsedSec
		}
		if totalSteps > 0 {
			stepWord := "steps"
			if totalSteps == 1 {
				stepWord = "step"
			}
			label = fmt.Sprintf("Worked for %.0fs · Done in %d %s", durationSec, totalSteps, stepWord)
		} else {
			label = fmt.Sprintf("Worked for %.0fs", durationSec)
		}
	}

	open := m.ShowTools || running
	pill := ui.Row(c).Radius(8).Padding(4, 10).Gap(6).AlignItems(ui.Center).
		Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border.Alpha(0.6)).
		Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.04))
	pill.Children(func() {
		if running {
			ui.Spinner(c).Size(13, 13).Label("Working")
			ui.Text(c, label).FontSize(12).FontWeight(600).TextColor(tc.Accent)
		} else {
			ui.Box(c).Size(14, 14).Radius(7).AlignItems(ui.Center).Justify(ui.Center).
				Background(tc.Success.Alpha(0.15)).Children(func() {
				ui.Icon(c, icons.TablerCheck).Width(9).Height(9).TextColor(tc.Success)
			})
			ui.Text(c, label).FontSize(12).FontWeight(500).TextColor(tc.Text)
		}
		ui.Box(c).Grow(1)
		chev := icons.TablerChevronRight
		if open {
			chev = icons.TablerChevronDown
		}
		ui.Icon(c, chev).Width(12).Height(12).TextColor(tc.IconMuted)
	})
	if pill.Hovered() {
		pill.Background(tc.SurfaceRaised)
	}
	if pill.Clicked() {
		m.ShowTools = !m.ShowTools
	}
}

func (a *AppUI) renderThinkingBlock(c *ui.Context, tc theme.ThemeColors, m *agent.ChatMessage) {
	isThinking := m.IsThinking && !m.Done
	tokens := len(m.Thinking) / 4
	if tokens < 1 && m.Thinking != "" {
		tokens = 1
	}

	label := "Thought"
	if isThinking {
		label = "Thinking…"
		elapsed := m.ThinkingTimeSec
		if !m.TurnStartedAt.IsZero() {
			elapsed = time.Since(m.TurnStartedAt).Seconds()
		}
		if elapsed >= 1 {
			label = fmt.Sprintf("Thinking for %.0fs…", elapsed)
		}
	} else {
		durationText := "a few seconds"
		if m.ThinkingTimeSec > 0 {
			durationText = fmt.Sprintf("%.1fs", m.ThinkingTimeSec)
		}
		if tokens > 0 {
			label = fmt.Sprintf("Thought · %s · %d tokens", durationText, tokens)
		} else {
			label = fmt.Sprintf("Thought · %s", durationText)
		}
	}

	ui.Column(c).FillWidth().Gap(3).Children(func() {
		header := ui.Row(c).Radius(6).Padding(3, 6).Gap(6).AlignItems(ui.Center).MinHeight(26)
		header.Children(func() {
			if isThinking {
				ui.Spinner(c).Size(13, 13).Label("Thinking")
			} else {
				ui.Icon(c, icons.TablerBrain).Width(14).Height(14).TextColor(tc.IconMuted)
			}
			labelColor := tc.SecondaryLabel
			if isThinking {
				labelColor = tc.Accent
			}
			ui.Text(c, label).FontSize(11).TextColor(labelColor).FontWeight(500)
			ui.Box(c).Grow(1)

			if m.Thinking != "" {
				copyBtn := ui.Row(c).Size(20, 20).Radius(4).AlignItems(ui.Center).Justify(ui.Center)
				copyBtn.Children(func() {
					ui.Icon(c, icons.TablerCopy).Width(11).Height(11).TextColor(tc.IconMuted)
				})
				if copyBtn.Hovered() {
					copyBtn.Background(tc.Text.Alpha(0.06))
				}
				if copyBtn.Clicked() {
					mygo.Clipboard.WriteText(m.Thinking)
					c.AddToast(ui.Toast{Title: "Copied Reasoning", Description: "Chain of thought copied to clipboard", Type: "success"})
				}
			}

			chev := icons.TablerChevronRight
			if m.ShowThinking {
				chev = icons.TablerChevronDown
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
			card := ui.Column(c).Padding(10, 12).Radius(8).
				Background(tc.SurfaceRaised.Alpha(0.4)).BorderWidth(1).BorderColor(tc.Border.Alpha(0.6)).Gap(8)
			card.Children(func() {
				ui.Text(c, m.Thinking).FontSize(11).TextColor(tc.TextMuted).Selectable()
				ui.Row(c).Padding(6, 0, 0, 0).BorderWidth(1, 0, 0, 0).BorderColor(tc.Border.Alpha(0.3)).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "Chain of Thought (CoT)").FontSize(10).TextColor(tc.TextMuted)
					ui.Box(c).Grow(1)
					ui.Text(c, fmt.Sprintf("%d tokens", tokens)).FontSize(10).TextColor(tc.TextMuted)
				})
			})
		}
	})
}

func (a *AppUI) renderFoldedToolTraces(c *ui.Context, tc theme.ThemeColors, m *agent.ChatMessage) {
	if len(m.Tools) == 0 {
		return
	}

	summary, hasRunning, hasFailed := summarizeTools(m.Tools)

	ui.Column(c).FillWidth().Gap(3).Children(func() {
		// Group summary row for multi-tool execution
		if len(m.Tools) > 1 {
			groupBar := ui.Row(c).Height(30).Radius(8).Padding(0, 10).Gap(8).AlignItems(ui.Center).
				Background(tc.SurfaceRaised.Alpha(0.5)).BorderWidth(1).BorderColor(tc.Border.Alpha(0.6))
			groupBar.Children(func() {
				if hasRunning {
					ui.Spinner(c).Size(13, 13).Label("Running tools")
				} else if hasFailed {
					ui.Icon(c, icons.TablerAlertTriangle).Width(13).Height(13).TextColor(tc.Error)
				} else {
					ui.Box(c).Size(14, 14).Radius(7).AlignItems(ui.Center).Justify(ui.Center).
						Background(tc.Success.Alpha(0.15)).Children(func() {
						ui.Icon(c, icons.TablerCheck).Width(9).Height(9).TextColor(tc.Success)
					})
				}

				summaryColor := tc.Text
				if hasRunning {
					summaryColor = tc.Accent
				}
				ui.Text(c, summary).FontSize(11).FontWeight(500).TextColor(summaryColor)

				ui.Box(c).Grow(1)

				// Step toggle pill
				stepToggle := ui.Row(c).Radius(4).Padding(2, 6).Gap(4).AlignItems(ui.Center).
					Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border.Alpha(0.5))
				stepToggle.Children(func() {
					stepLabel := fmt.Sprintf("Show %d steps", len(m.Tools))
					if m.ShowTools {
						stepLabel = fmt.Sprintf("Hide %d steps", len(m.Tools))
					}
					ui.Text(c, stepLabel).FontSize(10).FontWeight(500).TextColor(tc.TextMuted)
					chev := icons.TablerChevronRight
					if m.ShowTools {
						chev = icons.TablerChevronDown
					}
					ui.Icon(c, chev).Width(11).Height(11).TextColor(tc.IconMuted)
				})
			})
			if groupBar.Hovered() {
				groupBar.Background(tc.SurfaceRaised)
			}
			if groupBar.Clicked() {
				m.ShowTools = !m.ShowTools
			}
		}

		// Individual tool rows
		if m.ShowTools || len(m.Tools) == 1 {
			for _, tool := range m.Tools {
				thisTool := tool
				args := parseToolArgs(thisTool.Command, thisTool.ToolName)
				nl := strings.ToLower(thisTool.ToolName)
				isBash := strings.Contains(nl, "bash") || strings.Contains(nl, "shell") || strings.Contains(nl, "exec") || strings.Contains(nl, "command") || strings.Contains(nl, "terminal") || strings.Contains(nl, "run_command")
				isRead := strings.Contains(nl, "read") || strings.Contains(nl, "view") || strings.Contains(nl, "cat")
				isEdit := strings.Contains(nl, "edit") || strings.Contains(nl, "write") || strings.Contains(nl, "patch") || strings.Contains(nl, "create")
				isSearch := strings.Contains(nl, "search") || strings.Contains(nl, "grep") || strings.Contains(nl, "find")

				toolIcon := icons.TablerTerminal
				if isRead {
					toolIcon = icons.TablerFileText
				} else if isEdit {
					toolIcon = icons.TablerFilePen
				} else if isSearch {
					toolIcon = icons.TablerSearch
				}

				displayLabel := thisTool.ToolName
				detailText := ""
				if isBash {
					cmdText := extractShellCommand(thisTool.Command, args.Cmd)
					if cmdText != "" {
						detailText = "$ " + cmdText
					}
				} else if (isRead || isEdit) && args.Path != "" {
					detailText = args.Path
				} else if args.Cmd != "" {
					detailText = args.Cmd
				} else if args.Path != "" {
					detailText = args.Path
				}

				if len(detailText) > 65 {
					detailText = detailText[:62] + "…"
				}

				toolCard := ui.Column(c.Key("tool-" + thisTool.ID)).Radius(8).
					Background(tc.SurfaceRaised.Alpha(0.35)).BorderWidth(1).BorderColor(tc.Border.Alpha(0.5)).Clip()

				toolCard.Children(func() {
					// Row header
					row := ui.Row(c).Height(30).Padding(0, 8).Gap(7).AlignItems(ui.Center)
					row.Children(func() {
						ui.Icon(c, toolIcon).Width(13).Height(13).TextColor(tc.IconMuted)
						ui.Text(c, displayLabel).FontSize(12).FontWeight(500).TextColor(tc.Text)
						if detailText != "" {
							ui.Text(c, detailText).FontSize(11).TextColor(tc.TextMuted).MaxWidth(420)
						}
						ui.Box(c).Grow(1)

						// Status indicator
						switch thisTool.Status {
						case "completed":
							ui.Row(c).Size(16, 16).Radius(8).AlignItems(ui.Center).Justify(ui.Center).
								Background(tc.Success.Alpha(0.15)).Children(func() {
								ui.Icon(c, icons.TablerCheck).Width(9).Height(9).TextColor(tc.Success)
							})
						case "failed":
							ui.Row(c).Size(16, 16).Radius(8).AlignItems(ui.Center).Justify(ui.Center).
								Background(tc.Error.Alpha(0.15)).Children(func() {
								ui.Icon(c, icons.TablerX).Width(9).Height(9).TextColor(tc.Error)
							})
						default:
							ui.Spinner(c).Size(12, 12).Label("Running")
						}

						chev := icons.TablerChevronRight
						if thisTool.Expanded {
							chev = icons.TablerChevronDown
						}
						ui.Icon(c, chev).Width(12).Height(12).TextColor(tc.IconMuted)
					})
					if row.Hovered() {
						row.Background(tc.Text.Alpha(0.04))
					}
					if row.Clicked() {
						thisTool.Expanded = !thisTool.Expanded
					}

					// Expanded detail view
					if thisTool.Expanded {
						ui.Column(c).Padding(8, 10).Gap(6).
							BorderWidth(1, 0, 0, 0).BorderColor(tc.Border.Alpha(0.4)).
							Background(tc.Canvas.Alpha(0.6)).Children(func() {
							// For bash tools, render terminal console style
							if isBash {
								cmdStr := args.Cmd
								if cmdStr == "" {
									cmdStr = thisTool.Command
								}
								ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
									ui.Text(c, "$").FontSize(11).FontWeight(700).TextColor(tc.Accent)
									ui.Text(c, cmdStr).FontSize(11).TextColor(tc.Text).Grow(1).Selectable()
									if thisTool.Output != "" {
										copyBtn := ui.Row(c).Size(20, 20).Radius(4).AlignItems(ui.Center).Justify(ui.Center)
										copyBtn.Children(func() {
											ui.Icon(c, icons.TablerCopy).Width(11).Height(11).TextColor(tc.IconMuted)
										})
										if copyBtn.Hovered() {
											copyBtn.Background(tc.Text.Alpha(0.06))
										}
										if copyBtn.Clicked() {
											mygo.Clipboard.WriteText(thisTool.Output)
											c.AddToast(ui.Toast{Title: "Copied Output", Type: "success"})
										}
									}
								})
							}

							if thisTool.Output != "" {
								outText := thisTool.Output
								isErr := strings.Contains(outText, "<tool_use_error>") || thisTool.Status == "failed"
								boxBg := tc.Surface
								boxBorder := tc.Border.Alpha(0.5)
								textColor := tc.TextMuted
								if isErr {
									boxBg = tc.Error.Alpha(0.08)
									boxBorder = tc.Error.Alpha(0.3)
									textColor = tc.Error
									outText = strings.ReplaceAll(outText, "<tool_use_error>", "")
									outText = strings.ReplaceAll(outText, "</tool_use_error>", "")
									outText = strings.TrimSpace(outText)
								}
								if len(outText) > 1200 {
									outText = outText[:1200] + "\n… [truncated]"
								}
								ui.Box(c).Padding(8).Radius(6).Background(boxBg).
									BorderWidth(1).BorderColor(boxBorder).Children(func() {
									ui.Text(c, outText).FontSize(11).TextColor(textColor).Selectable()
								})
							}
						})
					}
				})
			}
		}
	})
}

func (a *AppUI) renderTurnChangesCard(c *ui.Context, tc theme.ThemeColors, m *agent.ChatMessage) {
	if a.Agent == nil {
		return
	}
	as := a.Agent

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
		ui.Row(c).Padding(6, 12).Gap(6).AlignItems(ui.Center).
			BorderWidth(0, 0, 1, 0).BorderColor(tc.Border.Alpha(0.4)).Children(func() {
			ui.Icon(c, icons.TablerFileCode).Width(13).Height(13).TextColor(tc.Accent)
			ui.Text(c, fmt.Sprintf("Modified %d files in this turn", len(m.Changes))).FontSize(12).FontWeight(500)
			ui.Box(c).Grow(1)
			if totalAdd > 0 {
				ui.Text(c, fmt.Sprintf("+%d", totalAdd)).FontSize(11).TextColor(tc.DiffAdditionForeground)
			}
			if totalDel > 0 {
				ui.Text(c, fmt.Sprintf("−%d", totalDel)).FontSize(11).TextColor(tc.DiffDeletionForeground)
			}
		})
		ui.Column(c).Padding(8).Gap(4).Children(func() {
			for _, ch := range m.Changes {
				ui.Row(c.Key("tchg-"+ch.Path)).Radius(6).Padding(6, 10).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icons.FileIconSVG(ch.Path)).Width(13).Height(13).TextColor(tc.IconMuted)
					ui.Text(c, ch.Path).FontSize(12).Grow(1)
					if ch.Additions >= 0 {
						DiffStatText(c, tc, ch.Additions, ch.Deletions)
					}
				})
			}
		})
	})
	_ = as
}

func (a *AppUI) renderApprovalBar(c *ui.Context, tc theme.ThemeColors, m *agent.ChatMessage) {
	appr := m.PendingApproval
	if appr == nil {
		return
	}

	card := ui.Column(c).Radius(12).Padding(14).Gap(8).
		Background(tc.Warning.Alpha(0.05)).BorderWidth(1).BorderColor(tc.Warning.Alpha(0.4))
	card.Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icons.TablerShieldAlert).Width(16).Height(16).TextColor(tc.Warning)
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
				if a.ACPMgr != nil {
					_ = a.ACPMgr.RespondPermission(m.PendingApproval.SessionID, m.PendingApproval.ToolCallID, true)
				}
			}
			always := composerControlChip(c, tc, icons.TablerShieldCheck, "Always Allow", "", false)
			if always.Clicked() {
				appr.Resolved = true
				appr.Approved = true
				if a.Agent != nil {
					a.Agent.ExecutionMode = agent.ModeBypass
				}
				if a.ACPMgr != nil {
					_ = a.ACPMgr.RespondPermission(m.PendingApproval.SessionID, m.PendingApproval.ToolCallID, true)
				}
			}
			deny := composerControlChip(c, tc, icons.TablerX, "Deny", "", true)
			if deny.Clicked() {
				appr.Resolved = true
				appr.Approved = false
				if a.ACPMgr != nil {
					_ = a.ACPMgr.RespondPermission(m.PendingApproval.SessionID, m.PendingApproval.ToolCallID, false)
				}
			}
		})
	})
}

func (a *AppUI) renderSteerQueueTray(c *ui.Context) {
	tc := a.ThemeMgr.Colors()
	as := a.Agent
	if as == nil {
		return
	}

	tray := ui.Column(c).Padding(8).Radius(16).Background(tc.SurfaceRaised).
		BorderWidth(1).BorderColor(tc.Border.Alpha(0.8)).Gap(4)
	tray.Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
			ui.Icon(c, icons.TablerGripVertical).Width(13).Height(13).TextColor(tc.IconMuted)
			ui.Text(c, fmt.Sprintf("Queued Instructions (%d)", len(as.SteerQueue))).FontSize(12).FontWeight(500)
			ui.Text(c, "· auto-steers when turn completes").FontSize(11).TextColor(tc.TextMuted)
			ui.Box(c).Grow(1)
			clear := composerControlChip(c, tc, icons.TablerX, "Clear all", "", false)
			if clear.Clicked() {
				as.SteerQueue = as.SteerQueue[:0]
			}
		})
		for idx, item := range as.SteerQueue {
			thisIdx, thisItem := idx, item
			ui.Row(c.Key(fmt.Sprintf("steer-%d", thisIdx))).Gap(6).Padding(2, 4).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, icons.TablerGripVertical).Width(12).Height(12).TextColor(tc.IconMuted)
				ui.Text(c, thisItem).FontSize(12).MaxWidth(420).Grow(1)
				del := composerControlChip(c, tc, icons.TablerX, "", "", true)
				if del.Clicked() {
					as.SteerQueue = append(as.SteerQueue[:thisIdx], as.SteerQueue[thisIdx+1:]...)
				}
			})
		}
	})
}

func (a *AppUI) renderComposer(c *ui.Context, tc theme.ThemeColors, sess *agent.AgentSession) {
	as := a.Agent
	if as == nil {
		return
	}
	inset, rightInset := a.chatLaneInset(c)

	ui.Column(c).Padding(0, inset, 12, rightInset).Gap(6).Children(func() {
		// Glass composer card
		a.renderComposerCard(c, tc, false)

		// Context Strip below composer (workspace folder + git branch chips)
		ui.Row(c).Gap(8).Padding(2, 4, 0, 4).AlignItems(ui.Center).Children(func() {
			projectName := filepath.Base(a.Workspace.RootDir)
			wsChip := ui.Row(c).Height(24).Radius(6).Padding(0, 8).Gap(4).AlignItems(ui.Center).
				Background(tc.Surface.Alpha(0.4)).BorderWidth(1).BorderColor(tc.Border.Alpha(0.5))
			wsChip.Children(func() {
				ui.Icon(c, icons.TablerFolder).Width(12).Height(12).TextColor(tc.IconMuted)
				ui.Text(c, projectName).FontSize(11).FontWeight(500)
				ui.Icon(c, icons.TablerChevronDown).Width(10).Height(10).TextColor(tc.IconMuted)
			})
			if wsChip.Clicked() {
				a.openWorkspaceFolder()
			}

			branchName := "main"
			if a.Git != nil && a.Git.Branch != "" {
				branchName = a.Git.Branch
			}
			branchChip := ui.Row(c).Height(24).Radius(6).Padding(0, 8).Gap(4).AlignItems(ui.Center).
				Background(tc.Surface.Alpha(0.4)).BorderWidth(1).BorderColor(tc.Border.Alpha(0.5))
			branchChip.Children(func() {
				ui.Icon(c, icons.TablerGitBranch).Width(12).Height(12).TextColor(tc.Accent)
				ui.Text(c, branchName).FontSize(11).FontWeight(500).TextColor(tc.Accent)
				ui.Icon(c, icons.TablerChevronDown).Width(10).Height(10).TextColor(tc.IconMuted)
			})
			if branchChip.Clicked() {
				a.ShowBranchModal = true
				a.BranchSearchQuery = ""
				if a.Git != nil {
					a.Git.Refresh()
				}
			}
		})
	})
	_ = sess
}

func (a *AppUI) renderComposerCard(c *ui.Context, tc theme.ThemeColors, isHome bool) {
	as := a.Agent
	if as == nil {
		return
	}

	card := ui.Column(c).Radius(20).Background(tc.Surface).
		BorderWidth(1).BorderColor(tc.Border).
		Shadow(0, 12, 28, -18, ui.RGBA(0, 0, 0, 0.4)).
		InsetShadow(0, 1, 0, 0, ui.RGBA(255, 255, 255, 0.08))
	if isHome {
		card.Width(680)
	}

	card.Children(func() {
		// 1. Attached Files Strip (if any files attached)
		if len(as.AttachedFiles) > 0 {
			ui.Row(c).Padding(10, 14, 2, 14).Gap(6).Wrap().Children(func() {
				for i, file := range as.AttachedFiles {
					idx := i
					filePath := file
					fileName := filepath.Base(filePath)
					chip := ui.Row(c.Key("att-"+filePath)).Height(26).Radius(6).Padding(0, 8).Gap(5).AlignItems(ui.Center).
						Background(tc.Accent.Alpha(0.15)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.3))
					chip.Children(func() {
						ui.Icon(c, icons.FileIconSVG(filePath)).Width(12).Height(12).TextColor(tc.Accent)
						ui.Text(c, fileName).FontSize(11).FontWeight(500)
						delBtn := ui.Row(c.Key("att-del-"+filePath)).Size(14, 14).Radius(7).AlignItems(ui.Center).Justify(ui.Center)
						delBtn.Children(func() {
							ui.Icon(c, icons.TablerX).Width(9).Height(9).TextColor(tc.IconMuted)
						})
						if delBtn.Clicked() {
							as.RemoveAttachedFile(idx)
						}
					})
				}
			})
		}

		// 2. Multiline TextInput Area
		ui.Row(c).Padding(12, 16, 6, 16).Children(func() {
			placeholder := "Ask anything, @ threads or files, $ skills, or / commands"
			if as.IsStreaming {
				placeholder = "Keep typing to queue follow-up changes..."
			}
			in := ui.TextInput(c, &as.InputText).Placeholder(placeholder).Grow(1)
			if isHome {
				in.AutoFocus()
			}
			if in.Submitted() {
				as.SubmitMessage()
			}
		})

		// 3. Bottom Controls Tray
		ui.Row(c).Gap(6).Padding(4, 12, 12, 12).AlignItems(ui.Center).Children(func() {
			// [+] Add Context Button: attach picker, active file, or workspace file
			addBtn := ui.Row(c).Size(28, 28).Radius(8).AlignItems(ui.Center).Justify(ui.Center).
				Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border.Alpha(0.6))
			addBtn.Children(func() {
				ui.Icon(c, icons.TablerPlus).Width(13).Height(13).TextColor(tc.TextMuted)
			})
			if addBtn.Hovered() {
				addBtn.Background(tc.Text.Alpha(0.06))
			}
			addBtn.ContextMenu(func(m *ui.Menu) {
				if m.Item("Attach Files...").Chosen() {
					if a.Window != nil {
						win := a.Window
						go func() {
							paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
								Parent:   win,
								Title:    "Attach Files",
								Multiple: true,
							})
							if err != nil || len(paths) == 0 {
								return
							}
							as.AttachFiles(paths)
							if a.Window != nil {
								a.Window.Invalidate()
							}
						}()
					}
				}
				if doc := a.Editor.ActiveDoc(); doc != nil {
					if m.Item("Attach Active File").Subtitle(doc.RelativePath).Chosen() {
						as.AttachFile(doc.RelativePath)
					}
				}
				if m.Item("Attach Workspace File...").Chosen() {
					a.Palette.Toggle()
				}
			})

			// Permission Mode selector chip
			var modeIcon *ui.SVG
			modeLabel := ""
			modeColor := tc.Text
			switch as.ExecutionMode {
			case agent.ModeAsk:
				modeIcon, modeLabel = icons.TablerHand, "Ask"
			case agent.ModeCode:
				modeIcon, modeLabel = icons.TablerPencil, "Edits"
			case agent.ModePlan:
				modeIcon, modeLabel = icons.TablerShieldCheck, "Auto"
			default:
				modeIcon, modeLabel = icons.TablerShieldAlert, "Full"
				modeColor = tc.FullAccess
			}
			modeBtn := chipRow(c, tc)
			modeBtn.Children(func() {
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, modeIcon).Width(13).Height(13).TextColor(modeColor)
					ui.Text(c, modeLabel).FontSize(12).FontWeight(500).TextColor(modeColor)
					ui.Icon(c, icons.TablerChevronDown).Width(10).Height(10).TextColor(modeColor.Alpha(0.7))
				})
			})
			if modeBtn.Clicked() {
				as.CycleExecutionMode()
			}

			// Model Selector Button (Triggers full ModelProviderPaletteModal)
			disp := as.ActiveModel
			if disp == "" {
				disp = "Select Model"
			} else {
				if idx := strings.Index(disp, " ("); idx != -1 {
					disp = disp[:idx]
				}
				if len(disp) > 22 {
					disp = disp[:20] + "…"
				}
			}
			modelBtn := chipRow(c, tc)
			modelBtn.Children(func() {
				ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
					icons.RenderProviderIcon(c, as.ActiveModel, 13)
					ui.Text(c, disp).FontSize(12).FontWeight(500).SingleLine()
					if as.ReasoningEffort != agent.EffortOff {
						ui.Row(c).Radius(4).Padding(1, 4).Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border.Alpha(0.6)).Children(func() {
							ui.Text(c, as.ReasoningEffortLabel()).FontSize(9).FontWeight(600).TextColor(tc.TextMuted)
						})
					}
					ui.Icon(c, icons.TablerChevronDown).Width(10).Height(10).TextColor(tc.IconMuted)
				})
			})
			if modelBtn.Clicked() {
				a.ShowModelPalette = true
				a.ModelSearchQuery = ""
			}

			// Auto-Verify Button
			verifyBtn := chipRow(c, tc)
			if as.AutoVerify {
				verifyBtn.Background(tc.Success.Alpha(0.12)).BorderWidth(1).BorderColor(tc.Success.Alpha(0.35))
				verifyBtn.Children(func() {
					ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, icons.TablerShieldCheck).Width(13).Height(13).TextColor(tc.Success)
						ui.Text(c, "Verify").FontSize(12).FontWeight(500).TextColor(tc.Success)
					})
				})
			} else {
				verifyBtn.Children(func() {
					ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, icons.TablerShieldCheck).Width(13).Height(13).TextColor(tc.TextMuted)
						ui.Text(c, "Verify").FontSize(12).FontWeight(500).TextColor(tc.Text)
					})
				})
			}
			if verifyBtn.Clicked() {
				as.AutoVerify = !as.AutoVerify
			}

			ui.Box(c).Grow(1)

			// Context Window Usage Gauge (Right side)
			_, pct := as.ContextUsage()
			gaugeBtn := ui.Row(c).Height(24).Radius(999).Padding(0, 8).Gap(4).AlignItems(ui.Center)
			gaugeBtn.Children(func() {
				dotColor := tc.Success
				if pct >= 80 {
					dotColor = tc.Error
				} else if pct >= 60 {
					dotColor = tc.Warning
				}
				ui.Box(c).Size(6, 6).Radius(3).Background(dotColor)
				ui.Text(c, fmt.Sprintf("%d%%", pct)).FontSize(11).FontWeight(500).TextColor(tc.TextMuted)
			})
			if gaugeBtn.Hovered() {
				gaugeBtn.Background(tc.Text.Alpha(0.04))
			}
			if gaugeBtn.Clicked() {
				a.ShowUsageModal = true
			}

			// Send / Steer / Queue / Stop controls
			if as.IsStreaming {
				if strings.TrimSpace(as.InputText) != "" {
					queueBtn := ui.Row(c).Height(28).Radius(999).Padding(0, 10).Gap(4).AlignItems(ui.Center).
						Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
					queueBtn.Children(func() {
						ui.Icon(c, icons.TablerPlus).Width(11).Height(11).TextColor(tc.TextMuted)
						ui.Text(c, "Queue").FontSize(11).FontWeight(500)
					})
					if queueBtn.Clicked() {
						as.SteerQueue = append(as.SteerQueue, as.InputText)
						as.InputText = ""
						c.AddToast(ui.Toast{Title: "Queued", Description: "Prompt queued for next turn", Type: "info"})
					}

					steerBtn := ui.Row(c).Height(28).Radius(999).Padding(0, 10).Gap(4).AlignItems(ui.Center).
						Background(tc.MessageAction)
					steerBtn.Children(func() {
						ui.Icon(c, icons.TablerCompass).Width(12).Height(12).TextColor(tc.MessageActionForeground)
						ui.Text(c, "Steer").FontSize(11).FontWeight(600).TextColor(tc.MessageActionForeground)
					})
					if steerBtn.Clicked() {
						as.SubmitMessage()
					}
				}

				// Stop Button (Destructive Red)
				stopBtn := ui.Row(c).Size(32, 32).Radius(999).AlignItems(ui.Center).Justify(ui.Center).
					Background(tc.Error.Alpha(0.9)).
					Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.2))
				stopBtn.Children(func() {
					ui.Icon(c, icons.TablerSquare).Width(13).Height(13).TextColor(tc.ErrorForeground)
				})
				if stopBtn.Clicked() {
					as.CancelStreaming()
				}
			} else {
				hasInput := strings.TrimSpace(as.InputText) != "" || len(as.AttachedFiles) > 0
				sendBtn := ui.Row(c).Size(32, 32).Radius(999).AlignItems(ui.Center).Justify(ui.Center)
				if hasInput {
					sendBtn.Background(tc.MessageAction).
						Shadow(0, 1, 3, 0, ui.RGBA(0, 0, 0, 0.25)).
						InsetShadow(0, 1, 0, 0, ui.RGBA(255, 255, 255, 0.2))
					sendBtn.Children(func() {
						ui.Icon(c, icons.TablerArrowUp).Width(16).Height(16).TextColor(tc.MessageActionForeground)
					})
					if sendBtn.Clicked() {
						as.SubmitMessage()
					}
				} else {
					sendBtn.Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border.Alpha(0.6))
					sendBtn.Children(func() {
						ui.Icon(c, icons.TablerArrowUp).Width(15).Height(15).TextColor(tc.IconMuted)
					})
				}
			}
		})
	})
}

func chipRow(c *ui.Context, tc theme.ThemeColors) ui.Element {
	row := ui.Row(c).Height(28).Radius(8).Padding(0, 10).Gap(5).AlignItems(ui.Center)
	if row.Hovered() {
		row.Background(tc.Text.Alpha(0.05)).BorderWidth(1).BorderColor(tc.Border.Alpha(0.4))
	}
	return row
}

func composerControlChip(c *ui.Context, tc theme.ThemeColors, icon *ui.SVG, label, kbd string, danger bool) ui.Element {
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

func (a *AppUI) renderAgentHome(c *ui.Context) {
	tc := a.ThemeMgr.Colors()
	as := a.Agent
	if as == nil {
		return
	}
	projectName := filepath.Base(as.WorkspacePath())
	if projectName == "" || projectName == "." {
		projectName = "General"
	}

	ui.Column(c).Grow(1).Fill().Center().Gap(20).Padding(24, 32).Background(tc.Canvas).Children(func() {
		ui.Column(c).AlignItems(ui.Center).Gap(14).Children(func() {
			ui.Icon(c, icons.ForgeSparkSVG).Width(54).Height(54).TextColor(tc.Text)
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "What should we do in").FontSize(28).FontWeight(600).LetterSpacing(-0.6)
				ui.Text(c, projectName+"?").FontSize(28).FontWeight(600).LetterSpacing(-0.6).TextColor(tc.Accent)
			})
		})

		// Landing composer
		a.renderComposerCard(c, tc, true)

		// Suggestions
		ui.Row(c).Gap(8).Wrap().Justify(ui.Center).AlignItems(ui.Center).Children(func() {
			suggestions := []struct {
				icon   *ui.SVG
				label  string
				prompt string
			}{
				{icons.TablerCompass, "Explore architecture", "Summarize the architecture of this repository, key packages, and entry points."},
				{icons.TablerBug, "Find & fix a bug", "Help me diagnose and fix an issue in the codebase: "},
				{icons.TablerFlask, "Run & improve tests", "Run the test suite, identify any failing or missing unit tests, and propose improvements."},
				{icons.TablerGitPullRequest, "Review working tree", "Review uncommitted git changes in this workspace for bugs, edge cases, and style."},
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
					if as.Invalidate != nil {
						as.Invalidate()
					}
				}
			}
		})

		if as.ShowModelPicker {
			a.renderModelPickerModal(c)
		}
	})
}

func (a *AppUI) renderModelPickerModal(c *ui.Context) {
	tc := a.ThemeMgr.Colors()
	as := a.Agent
	if as == nil {
		return
	}

	ui.Column(c).Absolute().Bottom(64).Right(64).Width(300).Padding(6).Radius(12).
		Background(tc.SurfaceOverlay).BorderWidth(1).BorderColor(tc.Border).
		Shadow(0, 12, 32, -8, ui.RGBA(0, 0, 0, 0.35)).Children(func() {
		ui.Row(c).Padding(6, 8).AlignItems(ui.Center).Gap(6).Children(func() {
			ui.Text(c, "Select AI Model").FontSize(12).FontWeight(600)
			ui.Box(c).Grow(1)
		})
		if len(as.AvailableModels) == 0 {
			ui.Row(c).Padding(12, 8).Justify(ui.Center).Children(func() {
				ui.Text(c, "No models configured").FontSize(11).TextColor(tc.TextMuted)
			})
		}
		for _, model := range as.AvailableModels {
			thisModel := model
			isSelected := (as.ActiveModel == thisModel)
			row := ui.Row(c.Key("model-"+thisModel)).Height(28).Radius(6).Padding(0, 8).Gap(6).AlignItems(ui.Center)
			row.Children(func() {
				if isSelected {
					ui.Icon(c, icons.TablerCheck).Width(13).Height(13).TextColor(tc.Accent)
				} else {
					ui.Icon(c, icons.TablerSparkles).Width(13).Height(13).TextColor(tc.IconMuted)
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

// renderTaskTracker draws forge-ade's FloatingTaskTracker: a collapsed pill
// at the chat's top-right that expands into a floating progress card.
func (a *AppUI) renderTaskTracker(c *ui.Context, tc theme.ThemeColors) {
	tm := a.Agent.TaskManager
	if tm == nil {
		return
	}
	tasks := tm.GetTasks()
	if len(tasks) == 0 {
		return
	}

	completed, total := tm.Progress()
	var current *agent.TaskItem
	for i := range tasks {
		if tasks[i].Status == agent.TaskInProgress {
			current = &tasks[i]
			break
		}
	}
	allDone := completed == total && total > 0

	if !tm.IsExpanded {
		// Collapsed pill: status glyph, N/M, current task, chevron.
		pill := ui.Row(c).Absolute().Top(54).Right(20).Height(30).Radius(999).
			Padding(0, 12).Gap(6).AlignItems(ui.Center).
			Background(tc.SurfaceRaised.Alpha(0.95)).BorderWidth(1).BorderColor(tc.Border.Alpha(0.8)).
			Shadow(0, 4, 12, 0, ui.RGBA(0, 0, 0, 0.18))
		pill.Children(func() {
			if current != nil {
				ui.Spinner(c).Size(11, 11).Label("working")
			} else if allDone {
				ui.Box(c).Size(14, 14).Radius(7).AlignItems(ui.Center).Justify(ui.Center).
					Background(tc.Success.Alpha(0.15)).Children(func() {
					ui.Icon(c, icons.TablerCheck).Width(9).Height(9).TextColor(tc.Success)
				})
			} else {
				ui.Icon(c, icons.TablerChecklist).Width(12).Height(12).TextColor(tc.IconMuted)
			}
			ui.Text(c, fmt.Sprintf("%d/%d", completed, total)).FontSize(11).FontWeight(600)
			if current != nil {
				label := current.Title
				if current.ActiveForm != "" {
					label = current.ActiveForm
				}
				ui.Text(c, label).FontSize(11).TextColor(tc.TextMuted).MaxWidth(180).SingleLine()
			}
			ui.Icon(c, icons.TablerChevronDown).Width(12).Height(12).TextColor(tc.IconMuted)
		})
		if pill.Clicked() {
			tm.ToggleExpanded()
		}
		return
	}

	// Expanded floating card, anchored under the pill.
	card := ui.Column(c).Absolute().Top(92).Right(24).Width(352).Radius(16).
		Padding(14).Gap(10).
		Background(tc.SurfaceRaised.Alpha(0.97)).BorderWidth(1).BorderColor(tc.Border.Alpha(0.8)).
		Shadow(0, 12, 32, -8, ui.RGBA(0, 0, 0, 0.35))
	card.Children(func() {
		// Header: goal + progress count + collapse chevron
		header := ui.Row(c).Gap(8).AlignItems(ui.Center)
		header.Children(func() {
			ui.Icon(c, icons.TablerChecklist).Width(14).Height(14).TextColor(tc.IconMuted)
			goal := tm.Goal
			if goal == "" {
				goal = "Active Goal"
			}
			ui.Text(c, goal).FontSize(13).FontWeight(600).Grow(1).SingleLine()
			ui.Text(c, fmt.Sprintf("%d/%d", completed, total)).FontSize(11).FontWeight(600).TextColor(tc.Accent)
			ui.Icon(c, icons.TablerChevronUp).Width(13).Height(13).TextColor(tc.IconMuted)
		})
		if header.Hovered() {
			header.Background(tc.Text.Alpha(0.04)).Radius(6)
		}
		if header.Clicked() {
			tm.ToggleExpanded()
		}

		// Progress bar: fill = foreground, as forge-ade's tracker does.
		pct := float32(0)
		if total > 0 {
			pct = float32(completed) / float32(total) * 100
		}
		track := ui.Box(c).FillWidth().Height(4).Radius(2).Background(tc.Border.Alpha(0.6))
		track.Children(func() {
			ui.Box(c).WidthPercent(pct).Height(4).Radius(2).Background(tc.Text)
		})

		// Task rows
		ui.Column(c).Gap(6).Children(func() {
			for _, task := range tasks {
				tid := task.ID
				isDone := task.Status == agent.TaskCompleted
				isCurrent := task.Status == agent.TaskInProgress
				row := ui.Row(c.Key("trk-" + tid)).Gap(8).AlignItems(ui.Center).MinHeight(24)
				row.Children(func() {
					if isDone {
						ui.Icon(c, icons.TablerCircleCheck).Width(14).Height(14).TextColor(tc.TextMuted)
					} else if isCurrent {
						ui.Spinner(c).Size(13, 13).Label("current task")
					} else {
						ui.Icon(c, icons.TablerCircle).Width(14).Height(14).TextColor(tc.IconMuted)
					}
					txt := ui.Text(c, task.Title).FontSize(12).Grow(1).SingleLine()
					if isDone {
						txt.TextColor(tc.TextMuted)
					} else if isCurrent {
						txt.FontWeight(600).TextColor(tc.Accent)
					}
				})
				if row.Hovered() {
					row.Background(tc.Text.Alpha(0.03)).Radius(6)
				}
				if row.Clicked() {
					tm.ToggleTask(tid)
				}
			}
		})
	})
}

func (a *AppUI) renderTaskChecklist(c *ui.Context) {
	tm := a.Agent.TaskManager
	if tm == nil {
		return
	}
	tasks := tm.GetTasks()
	if len(tasks) == 0 {
		return
	}

	t := c.Theme()
	completed, total := tm.Progress()

	ui.Column(c).Margin(8, 16).Padding(10, 14).Radius(8).Background(t.Surface).BorderWidth(1).BorderColor(t.Border).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				TablerIcon(c, icons.TablerChecklist, 13)
				ui.Text(c, "ACTIVE GOAL").FontSize(10).Bold().TextColor(t.TextMuted)
			})
			ui.Box(c).Grow(1)
			ui.Text(c, fmt.Sprintf("%d/%d completed", completed, total)).FontSize(11).TextColor(t.Accent)
			toggleBtn := ui.Button(c, "")
			toggleBtn.Children(func() {
				if tm.IsExpanded {
					TablerIcon(c, icons.TablerChevronUp, 12)
				} else {
					TablerIcon(c, icons.TablerChevronDown, 12)
				}
			})
			if toggleBtn.Clicked() {
				tm.ToggleExpanded()
			}
		})

		if tm.Goal != "" {
			ui.Text(c, tm.Goal).Bold().FontSize(13).Padding(2, 0, 6, 0)
		}

		if tm.IsExpanded {
			ui.Column(c).Gap(4).Padding(4, 0).Children(func() {
				for _, task := range tasks {
					tid := task.ID
					isDone := task.Status == agent.TaskCompleted
					isCurrent := task.Status == agent.TaskInProgress

					ui.Row(c.Key("task-" + tid)).Gap(8).AlignItems(ui.Center).Children(func() {
						checkBtn := ui.Button(c, "")
						checkBtn.Children(func() {
							if isDone {
								TablerIcon(c, icons.TablerCircleCheck, 13)
							} else if isCurrent {
								TablerIcon(c, icons.TablerLoader, 13)
							} else {
								TablerIcon(c, icons.TablerCircle, 13)
							}
						})
						if checkBtn.Clicked() {
							tm.ToggleTask(tid)
						}

						textElement := ui.Text(c, task.Title).FontSize(12)
						if isDone {
							textElement.TextColor(t.TextMuted)
						} else if isCurrent {
							textElement.Bold().TextColor(t.Accent)
						}

						if isCurrent && task.ActiveForm != "" {
							ui.Box(c).Grow(1)
							ui.Text(c, task.ActiveForm).FontSize(11).TextColor(t.TextMuted)
						}
					})
				}
			})
		}
	})
}

func (a *AppUI) renderTerminalSessionSurface(c *ui.Context, sessionID string) {
	if a.CLIMgr == nil {
		return
	}
	t := c.Theme()
	sess := a.CLIMgr.ActiveSession()

	if sess == nil {
		ui.Column(c).Grow(1).Center().Children(func() {
			ui.Text(c, "No active CLI session").FontSize(16).Bold().TextColor(t.TextMuted)
		})
		return
	}

	ui.Column(c).Fill().Children(func() {
		// Session Top Bar
		ui.Row(c).Gap(10).Padding(8, 14).AlignItems(ui.Center).Background(t.Surface).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			TablerIcon(c, icons.TablerRobot, 16)
			ui.Text(c, sess.AgentName).Bold().FontSize(13)
			ui.Text(c, sess.Title).FontSize(12).TextColor(t.TextMuted)

			ui.Box(c).Grow(1)

			if sess.Status == "running" {
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					TablerIcon(c, icons.TablerLoader, 13)
					ui.Text(c, "Running").FontSize(11).TextColor(t.Accent)
				})
			} else {
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					TablerIcon(c, icons.TablerCircle, 13)
					ui.Text(c, fmt.Sprintf("Stopped (code %d)", sess.ExitCode)).FontSize(11).TextColor(t.TextMuted)
				})
			}

			elapsed := time.Since(sess.StartedAt).Truncate(time.Second)
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				TablerIcon(c, icons.TablerClock, 13)
				ui.Text(c, fmt.Sprintf("%s", elapsed)).FontSize(11).TextColor(t.TextMuted)
			})

			if sess.Status == "running" {
				stopBtn := ui.Button(c, "")
				stopBtn.Children(func() {
					ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
						TablerIcon(c, icons.TablerSquare, 12)
						ui.Text(c, "Stop").FontSize(11)
					})
				})
				if stopBtn.Clicked() {
					a.CLIMgr.StopSession(sess.ID)
				}
			}
		})

		// Terminal viewport
		ui.Column(c).Grow(1).FillHeight().Children(func() {
			if sess.Terminal != nil {
				a.renderTerminal(c, sess.Terminal)
			}
		})
	})
}

func summarizeTools(tools []*agent.ToolExecution) (string, bool, bool) {
	commands, reads, edits, searches, skills, others := 0, 0, 0, 0, 0, 0
	hasRunning := false
	hasFailed := false

	for _, t := range tools {
		if t.Status == "running" {
			hasRunning = true
		}
		if t.Status == "failed" || strings.Contains(t.Output, "<tool_use_error>") {
			hasFailed = true
		}

		n := strings.ToLower(t.ToolName)
		switch {
		case strings.Contains(n, "bash") || strings.Contains(n, "shell") || strings.Contains(n, "command") || strings.Contains(n, "exec") || strings.Contains(n, "terminal") || strings.Contains(n, "run_command"):
			commands++
		case strings.Contains(n, "read") || strings.Contains(n, "view") || strings.Contains(n, "cat"):
			reads++
		case strings.Contains(n, "edit") || strings.Contains(n, "write") || strings.Contains(n, "patch") || strings.Contains(n, "create"):
			edits++
		case strings.Contains(n, "search") || strings.Contains(n, "grep") || strings.Contains(n, "find") || strings.Contains(n, "glob"):
			searches++
		case strings.Contains(n, "skill"):
			skills++
		default:
			others++
		}
	}

	var parts []string
	if commands > 0 {
		if commands == 1 {
			parts = append(parts, "Ran 1 command")
		} else {
			parts = append(parts, fmt.Sprintf("Ran %d commands", commands))
		}
	}
	if reads > 0 {
		if reads == 1 {
			parts = append(parts, "Read 1 file")
		} else {
			parts = append(parts, fmt.Sprintf("Read %d files", reads))
		}
	}
	if edits > 0 {
		if edits == 1 {
			parts = append(parts, "Edited 1 file")
		} else {
			parts = append(parts, fmt.Sprintf("Edited %d files", edits))
		}
	}
	if searches > 0 {
		if searches == 1 {
			parts = append(parts, "Ran 1 search")
		} else {
			parts = append(parts, fmt.Sprintf("Ran %d searches", searches))
		}
	}
	if skills > 0 {
		if skills == 1 {
			parts = append(parts, "Used 1 skill")
		} else {
			parts = append(parts, fmt.Sprintf("Used %d skills", skills))
		}
	}
	if others > 0 && len(parts) == 0 {
		if others == 1 {
			parts = append(parts, "Executed 1 tool")
		} else {
			parts = append(parts, fmt.Sprintf("Executed %d tools", others))
		}
	}

	summary := strings.Join(parts, " · ")
	if summary == "" {
		summary = fmt.Sprintf("%d tool calls", len(tools))
	}
	return summary, hasRunning, hasFailed
}

type ParsedToolArgs struct {
	Path      string
	Cmd       string
	Content   string
	OldString string
	NewString string
}

func parseToolArgs(raw string, toolName string) ParsedToolArgs {
	var res ParsedToolArgs
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "{") {
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(raw), &m); err == nil {
			for k, v := range m {
				strVal, ok := v.(string)
				if !ok {
					continue
				}
				kl := strings.ToLower(k)
				switch kl {
				case "path", "filepath", "file_path", "file", "filename", "target_file", "targetfile", "target_path", "targetpath":
					if res.Path == "" {
						res.Path = cleanArgPath(strVal)
					}
				case "command", "cmd", "script", "commandline", "command_line":
					if res.Cmd == "" {
						res.Cmd = strVal
					}
				case "content", "text", "code", "body", "file_content":
					if res.Content == "" {
						res.Content = strVal
					}
				case "old_string", "oldstring", "old", "old_text", "target", "search":
					if res.OldString == "" {
						res.OldString = strVal
					}
				case "new_string", "newstring", "new", "new_text", "replacement", "replace":
					if res.NewString == "" {
						res.NewString = strVal
					}
				}
			}
		}
	}

	if res.Cmd == "" && res.Path == "" {
		if raw != "{}" && !strings.HasPrefix(raw, "{\"") {
			res.Cmd = raw
		}
	}

	if res.Path == "" {
		nl := strings.ToLower(toolName)
		if strings.Contains(nl, "read") || strings.Contains(nl, "view") || strings.Contains(nl, "edit") || strings.Contains(nl, "write") {
			cand := cleanArgPath(raw)
			if !strings.HasPrefix(cand, "{") && len(cand) < 260 {
				res.Path = cand
			}
		}
	}

	return res
}

func cleanArgPath(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, " \"'`{}[]\n\r\t")
	for _, p := range []string{"path:", "filepath:", "file:", "command:", "content:", "\"path\":", "\"command\":"} {
		if strings.HasPrefix(strings.ToLower(s), p) {
			s = strings.TrimSpace(s[len(p):])
			s = strings.Trim(s, " \"'`{}[]\n\r\t")
		}
	}
	return s
}

func extractShellCommand(raw, fallback string) string {
	candidate := strings.TrimSpace(fallback)
	if candidate == "" {
		candidate = strings.TrimSpace(raw)
	}
	if strings.HasPrefix(candidate, "{") {
		p := parseToolArgs(candidate, "bash")
		if p.Cmd != "" {
			candidate = p.Cmd
		}
	}
	lines := strings.Split(candidate, "\n")
	first := strings.TrimSpace(lines[0])
	if first != "" {
		return first
	}
	return candidate
}
