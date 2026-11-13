package ui

import (
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/hasdev/forge-ade/app/native/pkg/agent"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
)

// renderAgentSurface renders the center chat viewport, collapsible terminal & right drawer.
func (a *AppUI) renderAgentSurface(c *ui.Context) {
	// The agent area hosts three main views, as forge-ade's AgentContainer:
	// the chat, the Workflow Board, and the Git Client.
	if a.AgentViewMode == AgentViewBoard || a.AgentViewMode == AgentViewGit {
		a.renderAgentView(c)
		return
	}
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

	leftInset, _ := a.chatLaneInset(c)

	// The chat fills a box so the floating task tracker can overlay it at
	// the top-right, as forge-ade's FloatingTaskTracker does. Clip contains
	// any overflow: nothing may paint over the workspace header.
	ui.Box(c).Fill().Clip().Children(func() {
		ui.Column(c).Fill().Background(tc.Canvas).Children(func() {
			// Session header: title, status pill, model, Fork
			ui.Row(c).Height(44).Gap(10).Padding(0, leftInset+8, 0, leftInset+8).AlignItems(ui.Center).
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

			// Find-in-thread bar (⌘F), ThreadFindBar parity.
			if a.ShowThreadFind {
				a.renderThreadFindBar(c, tc, as)
			}

			// Turn scrubber + chat lane, as forge-ade's left minimap rail.
			// The row takes the column's remaining height; the List bounds
			// itself to that height (FillHeight + Clip) so the transcript
			// virtualizes and scrolls instead of overflowing the header.
			ui.Row(c).Grow(1).Children(func() {
				a.renderTurnScrubber(c, tc, as)
				ui.List(c, &as.ListState, len(as.Messages), func(i int) {
					if i >= len(as.Messages) {
						return
					}
					m := as.Messages[i]
					if m.Role == "user" {
						// User bubble: right-aligned inside the centered lane
						// (Padding is CSS order top,right,bottom,left — the lane
						// inset belongs on BOTH sides).
						ui.Row(c.Key("urow")).Padding(8, leftInset, 8, leftInset).Justify(ui.End).Children(func() {
							bubble := ui.Column(c).Padding(10, 14).Radius(18).
								Background(tc.MessageSurface).BorderWidth(1).BorderColor(tc.Border.Alpha(0.5)).
								MaxWidth(600)
							bubble.Children(func() {
								ui.Text(c, m.Text).Selectable().FontSize(14).TextColor(tc.MessageForeground)
								// Attachment chips for the "Attached files:" block,
								// as the webview's UserAttachmentThumbnail pills.
								atts := userMessageAttachments(m.Text)
								if len(atts) > 0 {
									chipRow := ui.Row(c).Gap(5).Wrap().Padding(6, 0, 0, 0)
									chipRow.Children(func() {
										for _, att := range atts {
											att := att
											chip := ui.Row(c.Key("uatt-"+att)).Height(24).Radius(6).Padding(0, 8).Gap(5).AlignItems(ui.Center).
												Background(tc.Accent.Alpha(0.12)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.3))
											chip.Children(func() {
												ui.Icon(c, icons.FileIconSVG(att)).Width(12).Height(12).TextColor(tc.Accent)
												ui.Text(c, filepath.Base(att)).FontSize(11).FontWeight(500)
											})
											if chip.Clicked() {
												a.openSideFile(att)
											}
										}
									})
								}
							})
							bubble.Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.06))
						})
					} else {
						// Agent message: full lane width (centered via the insets),
						// work blocks and markdown spanning the lane like forge-ade.
						ui.Column(c.Key("arow")).Padding(8, leftInset, 8, leftInset).Gap(6).Children(func() {
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

							// Structured Questions Card (ask tool) — option chips,
							// custom answers, open-in-dialog.
							if len(m.PendingQuestions) > 0 {
								a.renderPendingQuestionsCard(c, tc, sess.ID, m)
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
				}).Grow(1).FillHeight().Padding(16, 0, 16, 0).Gap(14).Justify(ui.End).Clip()
			})

			// Mid-turn Steering Queue Tray
			if len(as.SteerQueue) > 0 {
				a.renderSteerQueueTray(c)
			}

			// Composer
			a.renderComposer(c, tc, sess)
		})

		// Jump to latest: shown when the user scrolled the transcript up,
		// as forge-ade's "Jump to latest" pill does.
		if !as.ListState.AtEnd() {
			ui.Row(c).Absolute().Bottom(120).Left(0).Right(0).Justify(ui.Center).Children(func() {
				inner := ui.Row(c).Height(30).Radius(999).Padding(0, 14).Gap(6).AlignItems(ui.Center).
					Background(tc.SurfaceOverlay).BorderWidth(1).BorderColor(tc.Border).
					Shadow(0, 4, 12, 0, ui.RGBA(0, 0, 0, 0.2))
				inner.Children(func() {
					ui.Box(c).Size(6, 6).Radius(3).Background(tc.Accent)
					ui.Text(c, "Jump to latest").FontSize(12).FontWeight(500)
				})
				if inner.Clicked() {
					as.ListState.ScrollToEnd()
				}
			})
		}

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
		// forge-ade's workSecs: thought durations when known, otherwise an
		// estimate of two seconds per tool run — never "0s".
		durationSec := m.ThinkingTimeSec
		if durationSec <= 0 {
			durationSec = math.Max(float64(len(m.Tools))*2, 2)
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

// renderFoldedToolTraces draws forge-ade's folded tool traces: a summary bar
// ("Ran 2 commands · Read 1 file · Edited 2 files" + Hide-N-steps), then
// per-tool rows — chevron, kind icon, [Write]/[Read]/[$] badge, mono label,
// output line count — with forge-ade's per-tool expanded views. Skill tools
// are excluded from the rows (counted in the summary only), as the original
// does with isSkillTool.
func (a *AppUI) renderFoldedToolTraces(c *ui.Context, tc theme.ThemeColors, m *agent.ChatMessage) {
	if len(m.Tools) == 0 {
		return
	}

	summary, hasRunning, hasFailed := summarizeTools(m.Tools)

	// Skill tools never render as rows (forge-ade filters isSkillTool).
	ordinary := make([]*agent.ToolExecution, 0, len(m.Tools))
	for _, t := range m.Tools {
		if !strings.Contains(strings.ToLower(t.ToolName), "skill") {
			ordinary = append(ordinary, t)
		}
	}

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
				ui.Text(c, summary).FontSize(11).FontWeight(500).TextColor(summaryColor).Grow(1).SingleLine()

				// Step toggle pill — fixed content, never wraps.
				stepLabel := fmt.Sprintf("Show %d steps", len(m.Tools))
				if m.ShowTools {
					stepLabel = fmt.Sprintf("Hide %d steps", len(m.Tools))
				}
				stepToggle := ui.Row(c.Key("steps-toggle")).Height(20).Radius(4).Padding(0, 6).Gap(4).AlignItems(ui.Center).
					Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border.Alpha(0.5))
				stepToggle.Children(func() {
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
		if m.ShowTools || len(ordinary) == 1 {
			for _, tool := range ordinary {
				thisTool := tool
				args := parseToolArgs(thisTool.Command, thisTool.ToolName)
				nl := strings.ToLower(thisTool.ToolName)
				isBash := strings.Contains(nl, "bash") || strings.Contains(nl, "shell") || strings.Contains(nl, "exec") || strings.Contains(nl, "terminal") || strings.Contains(nl, "run_command")
				isRead := strings.Contains(nl, "read") || strings.Contains(nl, "view") || strings.Contains(nl, "cat")
				isEdit := strings.Contains(nl, "edit") || strings.Contains(nl, "write") || strings.Contains(nl, "patch") || strings.Contains(nl, "create")
				isSearch := strings.Contains(nl, "search") || strings.Contains(nl, "grep") || strings.Contains(nl, "find")
				failed := thisTool.Status == "failed" || strings.Contains(thisTool.Output, "<tool_use_error>")

				toolIcon := icons.TablerTerminal
				if isRead {
					toolIcon = icons.TablerFileText
				} else if isEdit {
					toolIcon = icons.TablerFilePen
				} else if isSearch {
					toolIcon = icons.TablerSearch
				}

				// Row label + kind badge, as forge-ade's [Write]/[Read]/[$] chips.
				badge := ""
				displayLabel := thisTool.ToolName
				if isEdit {
					badge = "Write"
					if nl != "write" && (strings.Contains(nl, "edit") || strings.Contains(nl, "patch")) {
						badge = "Edit"
					}
					displayLabel = args.Path
				} else if isRead {
					badge = "Read"
					displayLabel = args.Path
				} else if isBash {
					badge = "$"
					if cmd := extractShellCommand(thisTool.Command, args.Cmd); cmd != "" {
						displayLabel = cmd
					}
				} else if args.Path != "" {
					displayLabel = args.Path
				}
				if displayLabel == "" {
					displayLabel = thisTool.ToolName
				}
				if len(displayLabel) > 72 {
					displayLabel = displayLabel[:69] + "…"
				}

				// Output line count on the right, like the original's read rows.
				outLines := 0
				if thisTool.Output != "" && (isRead || isSearch) {
					outLines = strings.Count(thisTool.Output, "\n") + 1
				}

				toolCard := ui.Column(c.Key("tool-" + thisTool.ID)).Radius(8).
					Background(tc.SurfaceRaised.Alpha(0.35)).BorderWidth(1).BorderColor(tc.Border.Alpha(0.5)).Clip()
				if failed {
					toolCard.BorderColor(tc.Error.Alpha(0.4))
				}

				toolCard.Children(func() {
					// Row header: chevron left, icon, badge, mono label, extras right.
					row := ui.Row(c).Height(30).Padding(0, 8).Gap(7).AlignItems(ui.Center)
					row.Children(func() {
						chev := icons.TablerChevronRight
						if thisTool.Expanded {
							chev = icons.TablerChevronDown
						}
						ui.Icon(c, chev).Width(12).Height(12).TextColor(tc.IconMuted)
						ui.Icon(c, toolIcon).Width(13).Height(13).TextColor(tc.IconMuted)
						if badge != "" {
							ui.Box(c).Radius(4).Padding(1, 5).Background(tc.Text.Alpha(0.06)).
								BorderWidth(1).BorderColor(tc.Border.Alpha(0.5)).Children(func() {
								ui.Text(c, badge).FontSize(10).Font("monospace").FontWeight(600).TextColor(tc.TextMuted)
							})
						}
						ui.Text(c, displayLabel).FontSize(12).Font("monospace").TextColor(tc.Text).Grow(1).SingleLine()
						if outLines > 0 {
							ui.Text(c, fmt.Sprintf("%d lines", outLines)).FontSize(10).Font("monospace").TextColor(tc.TextMuted)
						}
						if thisTool.Status == "running" {
							ui.Spinner(c).Size(12, 12).Label("Running")
						} else if failed {
							ui.Box(c).Radius(4).Padding(1, 5).Background(tc.Error.Alpha(0.12)).Children(func() {
								ui.Text(c, "Failed").FontSize(10).FontWeight(600).TextColor(tc.Error)
							})
						}
					})
					if row.Hovered() {
						row.Background(tc.Text.Alpha(0.04))
					}
					if row.Clicked() {
						thisTool.Expanded = !thisTool.Expanded
					}

					// Expanded detail views, mirroring forge-ade's tool blocks.
					if thisTool.Expanded {
						ui.Column(c).Padding(8, 10).Gap(6).
							BorderWidth(1, 0, 0, 0).BorderColor(tc.Border.Alpha(0.4)).
							Background(tc.Canvas.Alpha(0.6)).Children(func() {
							cleanOut := thisTool.Output
							if failed {
								cleanOut = strings.ReplaceAll(cleanOut, "<tool_use_error>", "")
								cleanOut = strings.ReplaceAll(cleanOut, "</tool_use_error>", "")
								cleanOut = strings.TrimSpace(cleanOut)
							}

							switch {
							case isRead && args.Path != "":
								a.renderToolReadBlock(c, tc, args.Path, cleanOut)
							case isEdit && (args.OldString != "" || args.NewString != ""):
								// Edit tool: colored old/new snippet, as the
								// webview tool view's diff body.
								renderToolEditDiff(c, tc, args.OldString, args.NewString)
							case isBash:
								cmdStr := args.Cmd
								if cmdStr == "" {
									cmdStr = thisTool.Command
								}
								ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
									ui.Text(c, "$").FontSize(11).FontWeight(700).TextColor(tc.Accent)
									ui.Text(c, cmdStr).FontSize(11).Font("monospace").TextColor(tc.Text).Grow(1).Selectable()
									if cleanOut != "" {
										copyBtn := ui.Row(c).Size(20, 20).Radius(4).AlignItems(ui.Center).Justify(ui.Center)
										copyBtn.Children(func() {
											ui.Icon(c, icons.TablerCopy).Width(11).Height(11).TextColor(tc.IconMuted)
										})
										if copyBtn.Hovered() {
											copyBtn.Background(tc.Text.Alpha(0.06))
										}
										if copyBtn.Clicked() {
											mygo.Clipboard.WriteText(cleanOut)
											c.AddToast(ui.Toast{Title: "Copied Output", Type: "success"})
										}
									}
								})
								if cleanOut != "" {
									renderToolOutputBox(c, tc, cleanOut, failed)
								}
							default:
								if cleanOut != "" {
									renderToolOutputBox(c, tc, cleanOut, failed)
								}
							}
						})
					}
				})
			}
		}
	})
}

// renderToolReadBlock draws forge-ade's ToolReadBlock: a file header with
// Open-in-Side-Viewer + Copy actions, then line-numbered content.
func (a *AppUI) renderToolReadBlock(c *ui.Context, tc theme.ThemeColors, path, output string) {
	fileName := filepath.Base(path)

	ui.Column(c).Radius(8).Background(tc.Surface.Alpha(0.8)).
		BorderWidth(1).BorderColor(tc.Border.Alpha(0.5)).Clip().Children(func() {
		// File header: icon + name + truncated path + actions
		ui.Row(c).Height(30).Padding(0, 10).Gap(8).AlignItems(ui.Center).
			Background(tc.SurfaceRaised.Alpha(0.6)).
			BorderWidth(0, 0, 1, 0).BorderColor(tc.Border.Alpha(0.4)).Children(func() {
			ui.Icon(c, icons.TablerFileText).Width(13).Height(13).TextColor(tc.Accent)
			ui.Text(c, fileName).FontSize(11).Font("monospace").FontWeight(600).TextColor(tc.Text).SingleLine()
			ui.Text(c, path).FontSize(11).Font("monospace").TextColor(tc.TextMuted).Grow(1).SingleLine()
			sideBtn := ui.Row(c.Key("read-side")).Gap(4).Radius(4).Padding(2, 6).AlignItems(ui.Center)
			sideBtn.Children(func() {
				ui.Icon(c, icons.TablerExternalLink).Width(11).Height(11).TextColor(tc.TextMuted)
				ui.Text(c, "Open in Side Viewer").FontSize(11).TextColor(tc.TextMuted)
			})
			if sideBtn.Hovered() {
				sideBtn.Background(tc.Text.Alpha(0.05))
			}
			if sideBtn.Clicked() && a.Workspace != nil {
				rel, err := filepath.Rel(a.Workspace.RootDir, path)
				if err != nil || strings.HasPrefix(rel, "..") {
					rel = path
				}
				a.openSideFile(rel)
			}
			copyBtn := ui.Row(c.Key("read-copy")).Gap(4).Radius(4).Padding(2, 6).AlignItems(ui.Center)
			copyBtn.Children(func() {
				ui.Icon(c, icons.TablerCopy).Width(11).Height(11).TextColor(tc.TextMuted)
				ui.Text(c, "Copy").FontSize(11).TextColor(tc.TextMuted)
			})
			if copyBtn.Hovered() {
				copyBtn.Background(tc.Text.Alpha(0.05))
			}
			if copyBtn.Clicked() {
				mygo.Clipboard.WriteText(output)
				c.AddToast(ui.Toast{Title: "Copied", Description: fileName, Type: "success"})
			}
		})

		// Line-numbered content (up to 40 lines), like the original's gutter.
		ui.Column(c).Padding(6, 8).Children(func() {
			lines := strings.Split(output, "\n")
			maxLines := 40
			shown := lines
			truncated := false
			if len(lines) > maxLines {
				shown = lines[:maxLines]
				truncated = true
			}
			for i, line := range shown {
				idx := i
				text := line
				ui.Row(c.Key(fmt.Sprintf("rl-%d", idx))).Padding(1, 0).Gap(10).AlignItems(ui.Start).Children(func() {
					ui.Text(c, fmt.Sprintf("%d", idx+1)).Width(24).FontSize(10).Font("monospace").
						TextColor(tc.TextMuted.Alpha(0.6))
					ui.Text(c, text).FontSize(11).Font("monospace").TextColor(tc.Text).Grow(1).Selectable()
				})
			}
			if truncated {
				ui.Row(c).Padding(6, 2, 0, 2).Justify(ui.Center).Children(func() {
					ui.Text(c, fmt.Sprintf("Truncated (+%d lines)", len(lines)-maxLines)).FontSize(10).TextColor(tc.TextMuted)
				})
			}
		})
	})
}

// renderToolOutputBox draws the generic mono output card with error styling
// and the original's truncation marker.
func renderToolOutputBox(c *ui.Context, tc theme.ThemeColors, outText string, failed bool) {
	boxBg := tc.Surface
	boxBorder := tc.Border.Alpha(0.5)
	textColor := tc.TextMuted
	if failed {
		boxBg = tc.Error.Alpha(0.08)
		boxBorder = tc.Error.Alpha(0.3)
		textColor = tc.Error
	}
	if len(outText) > 1200 {
		outText = outText[:1200] + "\n… [truncated]"
	}
	ui.Box(c).Padding(8).Radius(6).Background(boxBg).
		BorderWidth(1).BorderColor(boxBorder).Children(func() {
		ui.Text(c, outText).FontSize(11).Font("monospace").TextColor(textColor).Selectable()
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
			// Multi-file review bar: accept dismisses, reject restores the
			// pre-turn checkpoint (RestoreSessionCheckpoint parity).
			if m.Done {
				acceptBtn := ui.Row(c.Key("tchg-accept")).Height(22).Radius(6).Padding(0, 8).AlignItems(ui.Center).
					Background(tc.Success.Alpha(0.14)).BorderWidth(1).BorderColor(tc.Success.Alpha(0.35))
				acceptBtn.Children(func() {
					ui.Text(c, "Accept all").FontSize(10).FontWeight(600).TextColor(tc.Success)
				})
				if acceptBtn.Clicked() {
					m.Changes = nil
				}
				rejectBtn := ui.Row(c.Key("tchg-reject")).Height(22).Radius(6).Padding(0, 8).AlignItems(ui.Center).
					Background(tc.Error.Alpha(0.12)).BorderWidth(1).BorderColor(tc.Error.Alpha(0.35))
				rejectBtn.Children(func() {
					ui.Text(c, "Revert all").FontSize(10).FontWeight(600).TextColor(tc.Error)
				})
				if rejectBtn.Clicked() {
					m.Changes = nil
					session := as.ActiveSessionID
					go func() {
						if rerr := as.RestoreLastTurnCheckpoint(session); rerr != nil {
							// No checkpoint recorded (e.g. ACP turn); git discard fallback.
						}
						if a.Window != nil {
							a.Window.Invalidate()
						}
					}()
				}
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

// respondApproval routes an approval decision to the right backend: internal
// sessions go through the RakitKode agent engine (RespondAgentApproval
// parity), ACP harnesses through the ACP manager.
func (a *AppUI) respondApproval(sessionID, requestID string, approve, autoApproveAll bool) {
	if a.Agent != nil && a.Agent.IsInternalSession(sessionID) {
		_ = a.Agent.RespondInternalApproval(sessionID, approve, autoApproveAll)
		return
	}
	if a.ACPMgr != nil {
		_ = a.ACPMgr.RespondPermission(sessionID, requestID, approve)
	}
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
				a.respondApproval(m.PendingApproval.SessionID, m.PendingApproval.ToolCallID, true, false)
			}
			always := composerControlChip(c, tc, icons.TablerShieldCheck, "Always Allow", "", false)
			if always.Clicked() {
				appr.Resolved = true
				appr.Approved = true
				if a.Agent != nil {
					a.Agent.ExecutionMode = agent.ModeBypass
				}
				a.respondApproval(m.PendingApproval.SessionID, m.PendingApproval.ToolCallID, true, true)
			}
			deny := composerControlChip(c, tc, icons.TablerX, "Deny", "", true)
			if deny.Clicked() {
				appr.Resolved = true
				appr.Approved = false
				a.respondApproval(m.PendingApproval.SessionID, m.PendingApproval.ToolCallID, false, false)
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

	card := ui.Column(c).Radius(22).Background(tc.Surface).
		BorderWidth(1).BorderColor(tc.Border).
		Shadow(0, 12, 28, -18, ui.RGBA(0, 0, 0, 0.4)).
		InsetShadow(0, 1, 0, 0, ui.RGBA(255, 255, 255, 0.08))
	if isHome {
		// Landing composer: max-w-[48rem], but grow-and-shrink so a narrow
		// window (or the right drawer) never gets pushed off-screen — a fixed
		// width here would raise the chat column's min width.
		card.Grow(1).MaxWidth(768)
	}

	card.Children(func() {
		// Autocomplete panels float above the input: "/" opens the slash
		// catalog, a trailing "@token" opens the file mention menu —
		// AgentTaskInputBar.tsx parity.
		slashItems, slashActive := a.composerSlashState(as)
		if slashActive {
			if a.SlashSelected >= len(slashItems) {
				a.SlashSelected = 0
			}
			a.renderComposerMenu(c, tc, slashItems, a.SlashSelected, "slash")
		}
		mentionItems, _, mentionActive := a.composerMentionState(as)
		if mentionActive {
			if a.MentionSelected >= len(mentionItems) {
				a.MentionSelected = 0
			}
			a.renderComposerMenu(c, tc, mentionItems, a.MentionSelected, "mention")
		}

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

		// 2. Growing multiline prompt: one line at rest, up to eight as the
		// message wraps or a long text is pasted, as forge-ade's composer does.
		ui.Row(c).Padding(12, 16, 6, 16).Children(func() {
			placeholder := "Ask anything, @ threads or files, $ skills, or / commands"
			if as.IsStreaming {
				placeholder = "Keep typing to queue follow-up changes..."
			}
			in := ui.TextAreaBase(c, &as.InputText).Lines(1, 8).Placeholder(placeholder).Grow(1)
			if isHome {
				in.AutoFocus()
			}
			in.OnTextKey(func(k ui.TextKey) bool {
				// Autocomplete navigation consumes Up/Down/Enter/Esc while a
				// menu is active, as the webview slash / mention menus do.
				if slashActive && len(slashItems) > 0 {
					switch k.Key {
					case ui.KeyDown:
						a.SlashSelected = (a.SlashSelected + 1) % len(slashItems)
						return true
					case ui.KeyUp:
						a.SlashSelected = (a.SlashSelected - 1 + len(slashItems)) % len(slashItems)
						return true
					case ui.KeyEnter:
						if k.Mods == 0 {
							a.applyComposerSuggestion(slashItems[a.SlashSelected])
							k.SetText(a.Agent.InputText)
							return true
						}
					case ui.KeyEscape:
						a.SlashDismissedAt = a.Agent.InputText
						return true
					}
				}
				if mentionActive && len(mentionItems) > 0 {
					switch k.Key {
					case ui.KeyDown:
						a.MentionSelected = (a.MentionSelected + 1) % len(mentionItems)
						return true
					case ui.KeyUp:
						a.MentionSelected = (a.MentionSelected - 1 + len(mentionItems)) % len(mentionItems)
						return true
					case ui.KeyEnter:
						if k.Mods == 0 {
							a.applyComposerSuggestion(mentionItems[a.MentionSelected])
							k.SetText(a.Agent.InputText)
							return true
						}
					case ui.KeyEscape:
						a.MentionDismissedAt = a.Agent.InputText
						return true
					}
				}
				// Prompt history: up/down while the composer is single-line
				// and no autocomplete menu is active (promptHistoryStore).
				if k.Key == ui.KeyUp || k.Key == ui.KeyDown {
					if !strings.Contains(as.InputText, "\n") && strings.TrimSpace(as.InputText) != "" || (k.Key == ui.KeyDown && as.PromptHistoryIdx != -1) {
						delta := -1
						if k.Key == ui.KeyDown {
							delta = 1
						}
						if text, ok := as.PromptHistoryStep(delta, as.InputText); ok {
							k.SetText(text)
							return true
						}
					}
					return false
				}
				if k.Key == ui.KeyEnter && k.Mods == 0 {
					as.SubmitMessage()
					k.SetText("")
					return true
				}
				return false // Shift+Enter breaks the line
			})
			in.OnPaste(func(text string) bool {
				if strings.TrimSpace(text) == "" {
					if png := mygo.Clipboard.ReadImage(); len(png) > 0 {
						if path, err := as.SavePastedImage(png); err == nil {
							as.AttachFile(path)
							c.AddToast(ui.Toast{Title: "Image Attached", Description: filepath.Base(path), Type: "success"})
							return true
						}
					}
				}
				return false
			})
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

			// Context Window Usage Gauge (Right side) — landing shows the
			// status dot only; the in-chat gauge adds the percent label.
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
				if !isHome {
					ui.Text(c, fmt.Sprintf("%d%%", pct)).FontSize(11).FontWeight(500).TextColor(tc.TextMuted)
				}
			})
			if gaugeBtn.Hovered() {
				gaugeBtn.Background(tc.Text.Alpha(0.04))
			}
			if gaugeBtn.Clicked() {
				a.openUsageModal()
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

// workspaceMenuItems builds the workspace switcher menu shared by the landing
// heading and the tray project chip: recent session workspaces, then
// "Open folder…" and "General (Work outside project)", as AgentHomeView does.
func (a *AppUI) workspaceMenuItems(m *ui.Menu) {
	current := ""
	if a.Workspace != nil {
		current = a.Workspace.RootDir
	}
	recents := a.RecentWorkspaces // persisted opened-folder history
	if len(recents) == 0 {
		recents = a.recentWorkspacePaths() // fall back to session-derived
	}
	for _, ws := range recents {
		path := ws
		if m.Item(menuChecked(path == current, resolveProjectName(path))).Chosen() {
			a.switchWorkspace(path)
		}
	}
	m.Separator()
	if m.Item("Open folder…").Chosen() {
		a.openWorkspaceFolder()
	}
	if m.Item("General (Work outside project)").Chosen() {
		a.switchWorkspace("")
	}
}

// recentWorkspacePaths lists distinct session workspace paths, most recently
// updated first (remote ssh:// paths excluded — the internal agent works on
// local folders only, as forge-ade's landing filter does).
func (a *AppUI) recentWorkspacePaths() []string {
	type entry struct {
		path     string
		latestAt time.Time
	}
	var entries []entry
	if a.Agent != nil {
		for _, sess := range a.Agent.FilteredSessions() {
			if sess.WorkspacePath == "" || strings.HasPrefix(sess.WorkspacePath, "ssh://") {
				continue
			}
			found := false
			for i := range entries {
				if entries[i].path == sess.WorkspacePath {
					if sess.UpdatedAt.After(entries[i].latestAt) {
						entries[i].latestAt = sess.UpdatedAt
					}
					found = true
					break
				}
			}
			if !found {
				entries = append(entries, entry{path: sess.WorkspacePath, latestAt: sess.UpdatedAt})
			}
		}
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].latestAt.After(entries[j].latestAt) })
	paths := make([]string, 0, len(entries))
	for _, e := range entries {
		paths = append(paths, e.path)
	}
	return paths
}

// renderLandingTray draws the Kybern stacked strip attached above the landing
// composer: the project selector chip + git branch chip (git repos only).
func (a *AppUI) renderLandingTray(c *ui.Context, tc theme.ThemeColors, projectName string) {
	// w-14/15 of the composer above which it stacks — grows/shrinks with the
	// composer instead of imposing a fixed min width on the chat column.
	ui.Row(c).Grow(1).MaxWidth(717).Padding(5, 14).Gap(6).AlignItems(ui.Center).
		Background(tc.Surface.Alpha(0.5)).
		BorderWidth(1, 1, 0, 1).BorderColor(tc.Border.Alpha(0.55)).
		Radius(10, 10, 0, 0).Children(func() {
		// Project selector chip
		projChip := ui.Row(c.Key("tray-project")).Height(26).Radius(6).Padding(0, 7).Gap(5).AlignItems(ui.Center).Cursor(ui.CursorPointer)
		projChip.Children(func() {
			ui.Icon(c, icons.TablerFolder).Width(13).Height(13).TextColor(tc.TextMuted)
			ui.Text(c, projectName).FontSize(12).TextColor(tc.TextMuted)
			ui.Icon(c, icons.TablerChevronDown).Width(10).Height(10).TextColor(tc.TextMuted.Alpha(0.5))
		})
		if projChip.Hovered() {
			projChip.Background(tc.Text.Alpha(0.06))
		}
		projChip.Menu(func(m *ui.Menu) {
			a.workspaceMenuItems(m)
		})

		// Git branch chip — only when the workspace is a local git repository.
		branch := ""
		if a.Git != nil && a.Git.Branch != "" && (a.Workspace == nil || !strings.HasPrefix(a.Workspace.RootDir, "ssh://")) {
			branch = a.Git.Branch
		}
		if branch != "" {
			branchChip := ui.Row(c.Key("tray-branch")).Height(26).Radius(6).Padding(0, 7).Gap(5).AlignItems(ui.Center).Cursor(ui.CursorPointer)
			branchChip.Children(func() {
				ui.Icon(c, icons.TablerGitBranch).Width(12).Height(12).TextColor(tc.TextMuted.Alpha(0.7))
				ui.Text(c, branch).FontSize(12).TextColor(tc.TextMuted)
				ui.Icon(c, icons.TablerChevronDown).Width(10).Height(10).TextColor(tc.TextMuted.Alpha(0.5))
			})
			if branchChip.Hovered() {
				branchChip.Background(tc.Text.Alpha(0.06))
			}
			if branchChip.Clicked() {
				a.ShowBranchModal = true
				a.BranchSearchQuery = ""
				a.Git.Refresh()
			}
		}
	})
}

func (a *AppUI) renderAgentHome(c *ui.Context) {
	tc := a.ThemeMgr.Colors()
	as := a.Agent
	if as == nil {
		return
	}
	projectName := filepath.Base(as.WorkspacePath())
	if projectName == "" || projectName == "." || projectName == "/" {
		projectName = "General"
	}

	ui.Column(c).Grow(1).Fill().Center().Gap(24).Padding(24, 32).Background(tc.Canvas).Children(func() {
		ui.Column(c).AlignItems(ui.Center).Gap(16).Children(func() {
			ui.Icon(c, icons.ForgeSparkSVG).Width(54).Height(54).TextColor(tc.Text)
			// Heading: the workspace name is a dotted-underline button that
			// opens the workspace switcher, as AgentHomeView's h2 does.
			ui.Row(c).Gap(0).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "What should we do in").FontSize(28).FontWeight(600).LetterSpacing(-0.84)
				ui.Box(c).Width(8)
				wsBtn := ui.Row(c).AlignItems(ui.Center).Cursor(ui.CursorPointer)
				wsBtn.Children(func() {
					ui.Text(c, projectName).FontSize(28).FontWeight(600).LetterSpacing(-0.84).
						DottedUnderline().DecorationColor(tc.TextMuted.Alpha(0.55)).UnderlineOffset(6)
				})
				wsBtn.Menu(func(m *ui.Menu) {
					a.workspaceMenuItems(m)
				})
				ui.Text(c, "?").FontSize(28).FontWeight(600).LetterSpacing(-0.84)
			})
		})

		// Landing composer with the stacked context tray above it
		ui.Column(c).FillWidth().AlignItems(ui.Center).Gap(0).Children(func() {
			a.renderLandingTray(c, tc, projectName)
			a.renderComposerCard(c, tc, true)
		})

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
			ui.Column(c).Gap(1).Grow(1).Children(func() {
				goal := tm.Goal
				ui.Text(c, "Runtime & Tasks").FontSize(13).FontWeight(600).SingleLine()
				if goal != "" {
					ui.Text(c, goal).FontSize(11).TextColor(tc.TextMuted).SingleLine()
				}
			})
			ui.Text(c, fmt.Sprintf("%d/%d", completed, total)).FontSize(11).FontWeight(600).TextColor(tc.Accent)
			menuBtn := ui.Row(c).Size(22, 22).Radius(4).AlignItems(ui.Center).Justify(ui.Center)
			menuBtn.Children(func() {
				ui.Icon(c, icons.TablerDots).Width(13).Height(13).TextColor(tc.IconMuted)
			})
			if menuBtn.Hovered() {
				menuBtn.Background(tc.Text.Alpha(0.06))
			}
			menuBtn.ContextMenu(func(m *ui.Menu) {
				if m.Item("Clear tasks").Danger().Chosen() {
					tm.Clear()
				}
			})
			ui.Icon(c, icons.TablerMinimize).Width(13).Height(13).TextColor(tc.IconMuted)
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
						ui.Icon(c, icons.TablerArrowRight).Width(14).Height(14).TextColor(tc.Accent)
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
		a.CLIMgr.SyncExitStatus()

		// Session Top Bar: one title, real status, elapsed, Stop/Restart.
		ui.Row(c).Gap(10).Padding(8, 14).AlignItems(ui.Center).Background(t.Surface).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			TablerIcon(c, icons.TablerTerminal, 16)
			title := sess.Title
			if title == "" || title == sess.AgentName {
				title = sess.AgentName
				ui.Text(c, title).Bold().FontSize(13)
			} else {
				ui.Text(c, sess.AgentName).Bold().FontSize(13)
				ui.Text(c, title).FontSize(12).TextColor(t.TextMuted)
			}

			ui.Box(c).Grow(1)

			isRunning := sess.Status == "running" && sess.Terminal != nil && !sess.Terminal.IsExited()
			if isRunning {
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Size(7, 7).Radius(4).Background(t.Accent)
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

			if isRunning {
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
			} else {
				restartBtn := ui.Button(c, "")
				restartBtn.Children(func() {
					ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, icons.TablerRotate).Width(12).Height(12)
						ui.Text(c, "Restart").FontSize(11)
					})
				})
				if restartBtn.Clicked() {
					a.CLIMgr.RestartSession(sess.ID)
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

// ── Structured questions (ask tool) ────────────────────────────────────────
// Port of PendingQuestionsCard / AgentQuestionModal: per-question option
// chips (single/multi select, "(Recommended)" badge), custom free-text
// answers (drafted in AppUI.QuestionDrafts), submit back through
// RespondAgentAsk, plus an "Open dialog" affordance.

// questionDraftKey is the composer-draft key of one question's custom answer.
func questionDraftKey(sessionID string, qi int, inDialog bool) string {
	return fmt.Sprintf("q:%s:%d:%v", sessionID, qi, inDialog)
}

func (a *AppUI) renderPendingQuestionsCard(c *ui.Context, tc theme.ThemeColors, sessionID string, m *agent.ChatMessage) {
	card := ui.Column(c.Key("qcard-" + sessionID)).Radius(12).Padding(14).Gap(10).
		Background(tc.Accent.Alpha(0.05)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.35))
	card.Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icons.TablerHelp).Width(16).Height(16).TextColor(tc.Accent)
			ui.Text(c, "Agent has questions").FontSize(13).FontWeight(600)
			ui.Box(c).Grow(1)
			openDlg := composerControlChip(c, tc, icons.TablerExternalLink, "Open dialog", "", false)
			if openDlg.Clicked() {
				a.ShowQuestionModal = true
			}
		})

		for qi := range m.PendingQuestions {
			a.renderQuestionBlock(c, tc, sessionID, qi, false)
		}

		actions := ui.Row(c).Gap(6).Justify(ui.End).AlignItems(ui.Center)
		actions.Children(func() {
			submit := ui.Row(c).Height(28).Radius(8).Padding(0, 14).Gap(6).AlignItems(ui.Center).Background(tc.Accent)
			submit.Children(func() {
				ui.Text(c, "Submit answers").FontSize(12).FontWeight(600).TextColor(tc.AccentForeground)
			})
			if submit.Clicked() {
				session := sessionID
				go func() {
					_ = a.Agent.SubmitQuestionAnswers(session)
					a.clearQuestionDrafts(session)
					if a.Window != nil {
						a.Window.Invalidate()
					}
				}()
			}
		})
	})
}

// renderQuestionBlock draws one question: header chip, prompt, option chips
// with recommended badge, multi-select hint, and the custom answer input.
func (a *AppUI) renderQuestionBlock(c *ui.Context, tc theme.ThemeColors, sessionID string, qi int, inDialog bool) {
	qs := a.Agent.PendingQuestions(sessionID)
	if qi >= len(qs) {
		return
	}
	q := qs[qi]
	draftKey := questionDraftKey(sessionID, qi, inDialog)
	draft, ok := a.QuestionDrafts[draftKey]
	if !ok || draft == nil {
		draft = new(string)
		a.QuestionDrafts[draftKey] = draft
	}

	block := ui.Column(c.Key(fmt.Sprintf("q-%d-%v", qi, inDialog))).Padding(8, 10).Radius(10).Gap(8).
		Background(tc.Surface.Alpha(0.5)).BorderWidth(1).BorderColor(tc.Border.Alpha(0.5))
	block.Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			if q.Header != "" {
				hdr := ui.Row(c).Height(18).Radius(5).Padding(0, 6).AlignItems(ui.Center).Background(tc.Accent.Alpha(0.14))
				hdr.Children(func() {
					ui.Text(c, strings.ToUpper(q.Header)).FontSize(9).FontWeight(700).TextColor(tc.Accent)
				})
			}
			ui.Text(c, q.Question).FontSize(12).FontWeight(500).Grow(1)
			if q.Multi {
				ui.Text(c, "multi-select").FontSize(10).TextColor(tc.SecondaryLabel)
			}
		})

		// Option chips: click toggles; single-select clears the others.
		ui.Row(c).Gap(6).Wrap().Children(func() {
			for oi, opt := range q.Options {
				oi, opt := oi, opt
				selected := slices.Contains(q.Selected, opt)
				chip := ui.Row(c.Key(fmt.Sprintf("qopt-%d-%d-%v", qi, oi, inDialog))).Height(26).Radius(8).Padding(0, 10).Gap(5).AlignItems(ui.Center)
				chip.Children(func() {
					if selected {
						ui.Icon(c, icons.TablerCheck).Width(12).Height(12).TextColor(tc.Accent)
					}
					ui.Text(c, opt).FontSize(12)
					if oi == q.Recommended {
						badge := ui.Row(c).Height(15).Radius(4).Padding(0, 5).AlignItems(ui.Center).Background(tc.Success.Alpha(0.16))
						badge.Children(func() {
							ui.Text(c, "(Recommended)").FontSize(9).FontWeight(600).TextColor(tc.Success)
						})
					}
				})
				if selected {
					chip.Background(tc.Accent.Alpha(0.16)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.5))
				} else {
					chip.Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
				}
				if chip.Clicked() {
					a.Agent.UpdateQuestion(sessionID, qi, func(qu *agent.AgentQuestion) {
						if qu.Multi {
							if slices.Contains(qu.Selected, opt) {
								qu.Selected = slices.DeleteFunc(qu.Selected, func(s string) bool { return s == opt })
							} else {
								qu.Selected = append(qu.Selected, opt)
							}
						} else if selected {
							qu.Selected = nil
						} else {
							qu.Selected = []string{opt}
						}
					})
				}
			}
		})

		// Custom free-text answer, drafted in AppUI state so the input owns
		// live text between frames.
		customRow := ui.Row(c).Gap(6).AlignItems(ui.Center)
		customRow.Children(func() {
			ui.Text(c, "Other:").FontSize(11).TextColor(tc.TextMuted)
			ui.TextInput(c, draft).Placeholder("Type a custom answer…").Grow(1)
		})
	})

	// The draft feeds the live question on each frame; SubmitQuestionAnswers
	// reads from the live questions.
	a.Agent.UpdateQuestion(sessionID, qi, func(qu *agent.AgentQuestion) {
		qu.Custom = strings.TrimSpace(*draft)
	})
}

// clearQuestionDrafts drops the custom-answer drafts of a session.
func (a *AppUI) clearQuestionDrafts(sessionID string) {
	for k := range a.QuestionDrafts {
		if strings.HasPrefix(k, "q:"+sessionID+":") {
			delete(a.QuestionDrafts, k)
		}
	}
}

// renderQuestionModal is the full AgentQuestionModal dialog: roomier blocks
// with dedicated custom-answer inputs per question.
func (a *AppUI) renderQuestionModal(c *ui.Context) {
	if a.Agent == nil || !a.ShowQuestionModal {
		return
	}
	tc := a.ThemeMgr.Colors()
	sessionID := a.Agent.ActiveSessionID

	ui.Modal(c, &a.ShowQuestionModal, func() {
		panel := ui.Column(c).Width(520).MaxHeight(560).Radius(16).Background(tc.SurfaceOverlay).
			BorderWidth(1).BorderColor(tc.Border).Padding(18).Gap(12).Clip().
			Shadow(0, 24, 72, -20, ui.RGBA(0, 0, 0, 0.55))
		panel.Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, icons.TablerHelp).Width(18).Height(18).TextColor(tc.Accent)
				ui.Text(c, "Agent Questions").FontSize(15).FontWeight(600)
				ui.Box(c).Grow(1)
				if IconButton(c, tc, icons.TablerX, 14).Clicked() {
					a.ShowQuestionModal = false
				}
			})

			scroll := ui.Scroll(c)
			scroll.Children(func() {
				col := ui.Column(c).Gap(10)
				qs := a.Agent.PendingQuestions(sessionID)
				col.Children(func() {
					for qi := range qs {
						a.renderQuestionBlock(c, tc, sessionID, qi, true)
					}
				})
			}).Grow(1).MaxHeight(380)

			actions := ui.Row(c).Gap(8).Justify(ui.End)
			actions.Children(func() {
				cancel := composerControlChip(c, tc, icons.TablerX, "Cancel", "", false)
				if cancel.Clicked() {
					a.ShowQuestionModal = false
				}
				submit := ui.Row(c).Height(30).Radius(8).Padding(0, 16).Gap(6).AlignItems(ui.Center).Background(tc.Accent)
				submit.Children(func() {
					ui.Text(c, "Submit").FontSize(12).FontWeight(600).TextColor(tc.AccentForeground)
				})
				if submit.Clicked() {
					a.ShowQuestionModal = false
					go func() {
						_ = a.Agent.SubmitQuestionAnswers(sessionID)
						a.clearQuestionDrafts(sessionID)
						if a.Window != nil {
							a.Window.Invalidate()
						}
					}()
				}
			})
		})
	})
}

// userMessageAttachments extracts the @path entries of a user message's
// "Attached files:" block (as SubmitMessage formats them).
func userMessageAttachments(text string) []string {
	var atts []string
	inBlock := false
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Attached files:") {
			inBlock = true
			continue
		}
		if inBlock {
			if trimmed == "" {
				break // blank line ends the attachment block
			}
			if strings.HasPrefix(trimmed, "@") {
				atts = append(atts, strings.TrimPrefix(trimmed, "@"))
			}
		}
	}
	return atts
}

// renderToolEditDiff draws the old/new strings of an edit tool call as a
// colored diff snippet: red − lines for removed text, green + lines for
// added text, capped like the webview's inline diff bodies.
func renderToolEditDiff(c *ui.Context, tc theme.ThemeColors, oldStr, newStr string) {
	const maxRows = 24
	rows := 0
	truncated := false

	box := ui.Column(c).Radius(8).Background(tc.Canvas).BorderWidth(1).BorderColor(tc.Border.Alpha(0.5)).Clip()
	box.Children(func() {
		emit := func(prefix string, text string, color ui.Color) {
			if rows >= maxRows {
				truncated = true
				return
			}
			lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
			for _, ln := range lines {
				if rows >= maxRows {
					truncated = true
					break
				}
				shown := ln
				if len(shown) > 160 {
					shown = shown[:157] + "…"
				}
				rowText := shown
				row := ui.Row(c.Key(fmt.Sprintf("ed-%s-%d", prefix, rows))).Padding(1, 8).Background(color.Alpha(0.08))
				row.Children(func() {
					ui.Text(c, prefix+rowText).FontSize(11).Font("monospace").TextColor(color).SingleLine().Selectable()
				})
				rows++
			}
		}
		if oldStr != "" {
			emit("-", oldStr, tc.DiffDeletionForeground)
		}
		if newStr != "" {
			emit("+", newStr, tc.DiffAdditionForeground)
		}
		if truncated {
			ui.Row(c).Padding(2, 8).Children(func() {
				ui.Text(c, fmt.Sprintf("… diff truncated at %d lines", maxRows)).FontSize(10).TextColor(tc.TextMuted)
			})
		}
	})
}
