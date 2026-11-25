package ui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/agent"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
)

func (a *AppUI) findAgentSessionByID(id string) *agent.AgentSession {
	if a.Agent == nil || id == "" {
		return nil
	}
	for _, s := range a.Agent.Sessions {
		if s != nil && s.ID == id {
			return s
		}
	}
	return nil
}

// ensureAgentTab adds sessionID to the open header tab strip if not already present.
func (a *AppUI) ensureAgentTab(sessionID string) {
	if sessionID == "" {
		return
	}
	for _, id := range a.AgentOpenTabs {
		if id == sessionID {
			return
		}
	}
	a.AgentOpenTabs = append(a.AgentOpenTabs, sessionID)
}

// closeAgentTab closes one tab from the header strip (never deletes the session)
// and activates an adjacent tab if the closed tab was active.
func (a *AppUI) closeAgentTab(sessionID string) {
	idx := -1
	for i, id := range a.AgentOpenTabs {
		if id == sessionID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return
	}
	a.AgentOpenTabs = append(a.AgentOpenTabs[:idx], a.AgentOpenTabs[idx+1:]...)

	if a.Agent != nil && a.Agent.ActiveSessionID == sessionID {
		if len(a.AgentOpenTabs) > 0 {
			nextIdx := idx
			if nextIdx >= len(a.AgentOpenTabs) {
				nextIdx = len(a.AgentOpenTabs) - 1
			}
			if next := a.findAgentSessionByID(a.AgentOpenTabs[nextIdx]); next != nil {
				a.selectAgentSession(next)
				return
			}
		}
		a.handleNewTask()
	}
}

func (a *AppUI) closeAllAgentTabs() {
	a.AgentOpenTabs = nil
	a.handleNewTask()
}

func (a *AppUI) closeAgentTabsToLeft(sessionID string) {
	idx := -1
	for i, id := range a.AgentOpenTabs {
		if id == sessionID {
			idx = i
			break
		}
	}
	if idx <= 0 {
		return
	}
	a.AgentOpenTabs = append([]string(nil), a.AgentOpenTabs[idx:]...)
	if a.Agent != nil && a.Agent.ActiveSessionID != "" {
		found := false
		for _, id := range a.AgentOpenTabs {
			if id == a.Agent.ActiveSessionID {
				found = true
				break
			}
		}
		if !found {
			if s := a.findAgentSessionByID(sessionID); s != nil {
				a.selectAgentSession(s)
			}
		}
	}
}

func (a *AppUI) closeAgentTabsToRight(sessionID string) {
	idx := -1
	for i, id := range a.AgentOpenTabs {
		if id == sessionID {
			idx = i
			break
		}
	}
	if idx < 0 || idx >= len(a.AgentOpenTabs)-1 {
		return
	}
	a.AgentOpenTabs = append([]string(nil), a.AgentOpenTabs[:idx+1]...)
	if a.Agent != nil && a.Agent.ActiveSessionID != "" {
		found := false
		for _, id := range a.AgentOpenTabs {
			if id == a.Agent.ActiveSessionID {
				found = true
				break
			}
		}
		if !found {
			if s := a.findAgentSessionByID(sessionID); s != nil {
				a.selectAgentSession(s)
			}
		}
	}
}

func (a *AppUI) moveAgentTab(sessionID string, delta int) {
	idx := -1
	for i, id := range a.AgentOpenTabs {
		if id == sessionID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return
	}
	target := idx + delta
	if target < 0 || target >= len(a.AgentOpenTabs) {
		return
	}
	a.AgentOpenTabs[idx], a.AgentOpenTabs[target] = a.AgentOpenTabs[target], a.AgentOpenTabs[idx]
}

// openSessionInSplit opens sessionID in the side-by-side right pane (Zed style:
// if other tabs exist in the left pane, sessionID moves to the right pane and
// the left pane activates the adjacent tab).
func (a *AppUI) openSessionInSplit(sessionID string) {
	if sessionID == "" {
		return
	}
	a.AgentSplitSessionID = sessionID

	// If the left pane has other tabs besides sessionID, move sessionID out of
	// the left tab strip so each pane owns its own tab cleanly (Zed behavior).
	if len(a.AgentOpenTabs) > 1 {
		idx := -1
		for i, id := range a.AgentOpenTabs {
			if id == sessionID {
				idx = i
				break
			}
		}
		if idx >= 0 {
			a.AgentOpenTabs = append(a.AgentOpenTabs[:idx], a.AgentOpenTabs[idx+1:]...)
			if a.Agent != nil && a.Agent.ActiveSessionID == sessionID && len(a.AgentOpenTabs) > 0 {
				nextIdx := idx - 1
				if nextIdx < 0 {
					nextIdx = 0
				}
				if next := a.findAgentSessionByID(a.AgentOpenTabs[nextIdx]); next != nil {
					a.selectAgentSession(next)
				}
			}
		}
	}
	a.invalidateUI()
}

// closeAgentSplit closes the right split pane and returns its session tab to
// the left tab strip so the user never loses quick access to it.
func (a *AppUI) closeAgentSplit() {
	if a.AgentSplitSessionID != "" {
		a.ensureAgentTab(a.AgentSplitSessionID)
	}
	a.AgentSplitSessionID = ""
	a.AgentSplitWidth = 0
	a.invalidateUI()
}

// toggleAgentSplit opens or closes the side-by-side split session pane.
// Matches Zed's Split Right: moves the currently active session to the right
// pane (and reveals the previous open tab on the left, or mirrors if it is
// the only open tab).
func (a *AppUI) toggleAgentSplit() {
	if a.AgentSplitSessionID != "" {
		a.closeAgentSplit()
		return
	}
	if a.Agent == nil {
		return
	}
	activeID := a.Agent.ActiveSessionID
	if activeID != "" && a.findAgentSessionByID(activeID) != nil {
		a.openSessionInSplit(activeID)
		return
	}
	// Fallback if on empty New Task view: pick the most recent non-archived session.
	for _, s := range a.Agent.Sessions {
		if s != nil && !s.Archived {
			a.openSessionInSplit(s.ID)
			return
		}
	}
}

// reconcileAgentOpenTabs ensures the active session is in AgentOpenTabs and
// drops tabs whose backing sessions no longer exist.
func (a *AppUI) reconcileAgentOpenTabs() {
	if a.Agent == nil {
		return
	}
	if a.Agent.ActiveSessionID != "" {
		a.ensureAgentTab(a.Agent.ActiveSessionID)
	}
	kept := a.AgentOpenTabs[:0]
	for _, id := range a.AgentOpenTabs {
		if a.findAgentSessionByID(id) != nil {
			kept = append(kept, id)
		}
	}
	a.AgentOpenTabs = kept
	if a.AgentSplitSessionID != "" && a.findAgentSessionByID(a.AgentSplitSessionID) == nil {
		a.AgentSplitSessionID = ""
	}
}

// renderAgentHeaderTabStrip renders Orca's multi-session closable tab strip
// inside the 36px Agent Mode header bar.
func (a *AppUI) renderAgentHeaderTabStrip(c *ui.Context, tc theme.ThemeColors) {
	a.reconcileAgentOpenTabs()
	activeID := ""
	if a.Agent != nil {
		activeID = a.Agent.ActiveSessionID
	}

	ui.ScrollHorizontal(c).Grow(1).MinWidth(0).Scrollbars(false).Children(func() {
		ui.Row(c).Height(WorkspaceTopBarHeight).AlignItems(ui.Center).Children(func() {
			if len(a.AgentOpenTabs) == 0 {
				// Clean "New task" placeholder tab when no session is open yet.
				ui.Row(c.Key("hdr-tab-empty")).Height(WorkspaceTopBarHeight).Padding(0, 12).Gap(7).
					AlignItems(ui.Center).Shrink(0).
					Background(tc.Canvas).
					BorderWidth(0, 1, 0, 0).BorderColor(tc.ToolbarBorder).Children(func() {
					ui.Box(c).Size(6, 6).Radius(3).Background(tc.IconMuted.Alpha(0.5)).Shrink(0)
					ui.Text(c, "New task").FontSize(12).FontWeight(500).TextColor(tc.Text).SingleLine()
				})
			} else {
				for tabIdx, tabID := range a.AgentOpenTabs {
					ti := tabIdx
					id := tabID
					sess := a.findAgentSessionByID(id)
					if sess == nil {
						continue
					}
					isActive := id == activeID
					statusColor := tc.Success
					if a.sessionNeedsApproval(sess) {
						statusColor = tc.Warning
					} else if (isActive && a.Agent != nil && a.Agent.IsStreaming) || sess.Status == "running" {
						statusColor = tc.Accent
					}

					closeClicked := false
					tab := ui.Row(c.Key("hdr-tab-"+id)).Height(WorkspaceTopBarHeight).Padding(0, 8, 0, 10).Gap(6).
						AlignItems(ui.Center).Shrink(0).Cursor(ui.CursorPointer).
						BorderWidth(0, 1, 0, 0).BorderColor(tc.ToolbarBorder)
					if isActive {
						tab.Background(tc.Canvas)
					} else if tab.Hovered() {
						tab.Background(tc.ToolbarControlHover.Alpha(0.6))
					}

					tab.Children(func() {
						ui.Box(c).Size(6, 6).Radius(3).Background(statusColor).Shrink(0)
						if sess.AgentType == "internal" || strings.Contains(strings.ToLower(sess.AgentID), "internal") {
							ui.Icon(c, icons.ProviderForgeADE).Width(12).Height(12)
						} else {
							lookup := strings.TrimSpace(sess.AgentID + " " + sess.Model)
							if lookup != "" {
								icons.RenderProviderIcon(c, lookup, 12)
							}
						}
						txt := ui.Text(c, sess.Title).FontSize(12).MaxWidth(175).SingleLine()
						if isActive {
							txt.FontWeight(500).TextColor(tc.Text)
						} else {
							txt.FontWeight(400).TextColor(tc.TextMuted)
						}

						xBtn := ui.Row(c.Key("hdr-tab-x-"+id)).Size(16, 16).Radius(4).
							AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
						xBtn.Children(func() {
							xColor := tc.IconMuted.Alpha(0.55)
							if isActive || tab.Hovered() {
								xColor = tc.TextMuted
							}
							ui.Icon(c, icons.TablerX).Width(10).Height(10).TextColor(xColor)
						})
						if xBtn.Hovered() {
							xBtn.Background(tc.Text.Alpha(0.1))
						}
						if xBtn.Clicked() {
							closeClicked = true
							a.closeAgentTab(id)
						}
					})
					if tab.Clicked() && !closeClicked {
						a.selectAgentSession(sess)
					}
					tab.ContextMenu(func(m *ui.Menu) {
						if m.Item("Open in Split Pane").Chosen() {
							a.openSessionInSplit(id)
						}
						if m.Item("Hand off session…").Chosen() {
							a.selectAgentSession(sess)
							a.OpenHandoffModal()
						}
						m.Separator()
						if m.Item("Move Left").Disabled(ti <= 0).Chosen() {
							a.moveAgentTab(id, -1)
						}
						if m.Item("Move Right").Disabled(ti >= len(a.AgentOpenTabs)-1).Chosen() {
							a.moveAgentTab(id, 1)
						}
						m.Separator()
						if m.Item("Close").Chosen() {
							a.closeAgentTab(id)
						}
						if m.Item("Close Others").Disabled(len(a.AgentOpenTabs) <= 1).Chosen() {
							a.AgentOpenTabs = []string{id}
							a.selectAgentSession(sess)
						}
						if m.Item("Close Left").Disabled(ti <= 0).Chosen() {
							a.closeAgentTabsToLeft(id)
						}
						if m.Item("Close Right").Disabled(ti >= len(a.AgentOpenTabs)-1).Chosen() {
							a.closeAgentTabsToRight(id)
						}
						if m.Item("Close All").Chosen() {
							a.closeAllAgentTabs()
						}
					})
				}
			}

			// "+" button right next to the last tab (Orca style)
			newBtn := ui.Row(c.Key("hdr-agent-new")).Size(28, 28).Radius(6).Padding(0, 4).
				AlignItems(ui.Center).Justify(ui.Center).Shrink(0).Cursor(ui.CursorPointer)
			newBtn.Tooltip("New session tab (⌘N)")
			newBtn.Children(func() {
				ui.Icon(c, icons.TablerPlus).Width(13).Height(13).TextColor(tc.IconMuted)
			})
			if newBtn.Hovered() {
				newBtn.Background(tc.ToolbarControlHover)
			}
			if newBtn.Clicked() {
				a.handleNewTask()
			}

			// Remaining empty strip drags the window cleanly without swallowing tab right-clicks.
			ui.Box(c).Grow(1).FillHeight().DragWindow()
		})
	})
}

// renderSplitSessionPane renders the Orca-style secondary Agent session pane
// side-by-side next to the primary pane, with its own 36px tab header, live
// transcript, and inline composer.
func (a *AppUI) renderSplitSessionPane(c *ui.Context, tc theme.ThemeColors, sess *agent.AgentSession) {
	if sess == nil {
		return
	}
	statusColor := tc.Success
	isRunning := sess.Status == "running"
	if a.sessionNeedsApproval(sess) {
		statusColor = tc.Warning
	} else if isRunning {
		statusColor = tc.Accent
	}

	ui.Column(c.Key("split-pane-"+sess.ID)).Grow(1).FillHeight().Background(tc.Canvas).
		BorderWidth(0, 0, 0, 1).BorderColor(tc.ToolbarBorder).Clip().Children(func() {
		// 36px Split Pane Header (aligned with WorkspaceTopBarHeight).
		// DragWindow is placed on the empty spacer so right-click ContextMenu on spTab works on macOS.
		ui.Row(c).Height(WorkspaceTopBarHeight).Padding(0, 8, 0, 0).Gap(4).AlignItems(ui.Center).
			Background(tc.Chrome).BorderWidth(0, 0, 1, 0).BorderColor(tc.ToolbarBorder).Children(func() {
			// Active split tab (with right-click pane actions)
			spTab := ui.Row(c.Key("split-tab-"+sess.ID)).Height(WorkspaceTopBarHeight).Padding(0, 10).Gap(6).
				AlignItems(ui.Center).Background(tc.Canvas).
				BorderWidth(0, 1, 0, 0).BorderColor(tc.ToolbarBorder)
			spTab.Children(func() {
				ui.Box(c).Size(6, 6).Radius(3).Background(statusColor).Shrink(0)
				if sess.AgentType == "internal" || strings.Contains(strings.ToLower(sess.AgentID), "internal") {
					ui.Icon(c, icons.ProviderForgeADE).Width(12).Height(12)
				} else {
					lookup := strings.TrimSpace(sess.AgentID + " " + sess.Model)
					if lookup != "" {
						icons.RenderProviderIcon(c, lookup, 12)
					}
				}
				ui.Text(c, sess.Title).FontSize(12).FontWeight(500).TextColor(tc.Text).MaxWidth(200).SingleLine()
			})
			spTab.ContextMenu(func(m *ui.Menu) {
				if m.Item("Move Left").Chosen() {
					a.closeAgentSplit()
					a.selectAgentSession(sess)
				}
				if m.Item("Hand off session…").Chosen() {
					a.selectAgentSession(sess)
					a.OpenHandoffModal()
				}
				m.Separator()
				if m.Item("Close").Chosen() {
					a.closeAgentSplit()
				}
				if m.Item("Close All").Chosen() {
					a.AgentSplitSessionID = ""
					a.AgentSplitWidth = 0
					a.closeAllAgentTabs()
				}
			})

			// "+" session picker to switch which session is displayed in this split pane
			pickBtn := ui.Row(c.Key("split-pick")).Size(26, 26).Radius(6).
				AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
			pickBtn.Tooltip("Switch split session")
			pickBtn.Children(func() {
				ui.Icon(c, icons.TablerPlus).Width(13).Height(13).TextColor(tc.IconMuted)
			})
			if pickBtn.Hovered() {
				pickBtn.Background(tc.ToolbarControlHover)
			}
			pickBtn.Menu(func(m *ui.Menu) {
				if a.Agent != nil {
					for _, s := range a.Agent.Sessions {
						if s == nil || s.Archived {
							continue
						}
						cand := s
						if m.Item(menuChecked(cand.ID == sess.ID, cand.Title)).Chosen() {
							a.openSessionInSplit(cand.ID)
						}
					}
				}
			})

			ui.Box(c).Grow(1).FillHeight().DragWindow()

			// Pane actions menu (Move Left, Hand off, Close, Close All) + Close split + global controls
			spMore := IconButton(c, tc, icons.TablerDots, 14).Shrink(0).Tooltip("Split Pane Actions")
			spMore.Menu(func(m *ui.Menu) {
				if m.Item("Move Left").Chosen() {
					a.closeAgentSplit()
					a.selectAgentSession(sess)
				}
				if m.Item("Hand off session…").Chosen() {
					a.selectAgentSession(sess)
					a.OpenHandoffModal()
				}
				m.Separator()
				if m.Item("Close").Chosen() {
					a.closeAgentSplit()
				}
				if m.Item("Close All").Chosen() {
					a.AgentSplitSessionID = ""
					a.AgentSplitWidth = 0
					a.closeAllAgentTabs()
				}
			})
			if IconButton(c, tc, icons.TablerX, 13).Tooltip("Close split pane").Clicked() {
				a.closeAgentSplit()
			}
			ui.Box(c).Size(1, 14).Background(tc.ToolbarBorder).Shrink(0)
			a.renderGlobalHeaderTrailingControls(c, tc)
		})

		// Split Pane Transcript (100% parity with primary Agent chat:
		// thinking blocks, worked-for pills, tool traces, worker cards,
		// changes card, approval bars, and structured questions).
		a.setupMarkdownOptions(a.Agent, tc)
		lastAssistantID := 0
		for _, msg := range sess.Messages {
			if msg != nil && msg.Role != "user" {
				lastAssistantID = msg.ID
			}
		}

		ui.Scroll(c.Key("split-scroll-" + sess.ID)).Grow(1).Children(func() {
			ui.Column(c).Padding(12, 16, 16, 16).Gap(8).Children(func() {
				if sess.ParentSessionID != "" {
					if parent := a.findAgentSessionByID(sess.ParentSessionID); parent != nil {
						linkCard := ui.Row(c.Key("split-parent-"+parent.ID)).Radius(8).Padding(7, 10).Gap(6).
							AlignItems(ui.Center).Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).Cursor(ui.CursorPointer)
						linkCard.Children(func() {
							ui.Icon(c, icons.TablerGitFork).Width(12).Height(12).TextColor(tc.Accent)
							ui.Text(c, "Task from coordinator").FontSize(11.5).TextColor(tc.TextMuted)
							ui.Text(c, parent.Title).FontSize(11.5).FontWeight(600).TextColor(tc.Text).Grow(1).SingleLine()
							ui.Icon(c, icons.TablerExternalLink).Width(11).Height(11).TextColor(tc.IconMuted)
						})
						if linkCard.Clicked() {
							a.selectAgentSession(parent)
						}
					}
				}

				if len(sess.Messages) == 0 {
					ui.Column(c).Center().Padding(40, 16).Gap(4).Children(func() {
						ui.Text(c, sess.Title).FontSize(13).FontWeight(600).TextColor(tc.Text)
						ui.Text(c, "Send a message below to interact with this session in parallel.").FontSize(11.5).TextColor(tc.TextMuted)
					})
				}

				for _, msg := range sess.Messages {
					if msg == nil {
						continue
					}
					m := msg
					if m.Role == "user" {
						ui.Column(c.Key(fmt.Sprintf("sp-u-%d", m.ID))).Padding(8, 0, 4, 0).Gap(4).AlignItems(ui.End).Children(func() {
							a.renderUserMessageBubble(c, tc, sess.ID, m, 520)
							renderUserPromptCopyRow(c, tc, "sp-ucopy-"+fmt.Sprint(m.ID), m)
						})
						continue
					}

					ui.Column(c.Key(fmt.Sprintf("sp-a-%d", m.ID))).Padding(6, 0, 8, 0).Gap(6).Children(func() {
						a.renderAssistantResponseCard(c, tc, sess.ID, lastAssistantID, m)

						if m.Done && (m.Text != "" || len(m.Tools) > 0) {
							ui.Row(c).Gap(6).Padding(2, 0, 0, 0).AlignItems(ui.Center).Children(func() {
								acopy := ui.Row(c.Key("sp-copy-"+fmt.Sprint(m.ID))).Size(22, 22).Radius(4).AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
								acopy.Children(func() {
									ui.Icon(c, icons.TablerCopy).Width(13).Height(13).TextColor(tc.IconMuted)
								})
								if acopy.Hovered() {
									acopy.Background(tc.Text.Alpha(0.06))
								}
								if acopy.Clicked() {
									mygo.Clipboard.WriteText(messagePlainText(m))
									c.AddToast(ui.Toast{Title: "Copied", Type: "success"})
								}
								if !m.Timestamp.IsZero() {
									ui.Text(c, m.Timestamp.Format("3:04 PM")).FontSize(10.5).TextColor(tc.TextMuted)
								}
							})
						}
					})
				}
			})
		})

		// Split Pane Composer (matches primary Orca-style composer card)
		sendSplit := func() {
			prompt := strings.TrimSpace(a.AgentSplitInput)
			if prompt == "" {
				return
			}
			a.AgentSplitInput = ""
			_ = a.Agent.MessageWorker(sess.ID, prompt)
		}

		ui.Box(c).Padding(8, 16, 12, 16).Children(func() {
			ui.Column(c).Radius(12).Background(tc.Surface).
				BorderWidth(1).BorderColor(tc.Border).Clip().Children(func() {
				ui.Row(c).Padding(12, 16, 6, 16).Children(func() {
					in := ui.TextAreaBase(c, &a.AgentSplitInput).Lines(1, 6).Placeholder("Ask anything, @ threads or files, $ skills, or / commands").Grow(1)
					in.OnTextKey(func(k ui.TextKey) bool {
						if k.Key == ui.KeyEnter && k.Mods == 0 {
							sendSplit()
							k.SetText("")
							return true
						}
						return false
					})
				})
				ui.Row(c).Padding(4, 12, 10, 12).Gap(6).AlignItems(ui.Center).Children(func() {
					// Permission badge (mirrors main composer)
					modeLabel := "Code"
					modeColor := tc.TextMuted
					if a.Agent != nil {
						switch a.Agent.ExecutionMode {
						case agent.ModeBypass:
							modeLabel = "Full access"
							modeColor = tc.FullAccess
						case agent.ModePlan:
							modeLabel = "Plan"
							modeColor = tc.Accent
						case agent.ModeAsk:
							modeLabel = "Ask"
						}
					}
					permRow := ui.Row(c.Key("sp-perm")).Height(24).Radius(6).Padding(0, 7).Gap(5).AlignItems(ui.Center).Cursor(ui.CursorPointer)
					permRow.Children(func() {
						ui.Icon(c, icons.TablerShieldCheck).Width(13).Height(13).TextColor(modeColor)
						ui.Text(c, modeLabel).FontSize(11.5).FontWeight(500).TextColor(modeColor).SingleLine()
					})
					if permRow.Hovered() {
						permRow.Background(tc.Text.Alpha(0.06))
					}
					if permRow.Clicked() && a.Agent != nil {
						switch a.Agent.ExecutionMode {
						case agent.ModeBypass:
							a.Agent.ExecutionMode = agent.ModeCode
						case agent.ModeCode:
							a.Agent.ExecutionMode = agent.ModePlan
						case agent.ModePlan:
							a.Agent.ExecutionMode = agent.ModeAsk
						default:
							a.Agent.ExecutionMode = agent.ModeBypass
						}
					}

					ui.Box(c).Grow(1)

					// Provider icon + model label
					ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
						if sess.AgentType == "internal" || strings.Contains(strings.ToLower(sess.AgentID), "internal") {
							ui.Icon(c, icons.ProviderForgeADE).Width(12).Height(12)
						} else {
							lookup := strings.TrimSpace(sess.AgentID + " " + sess.Model)
							if lookup != "" {
								icons.RenderProviderIcon(c, lookup, 12)
							}
						}
						if mLbl := shortModelLabel(sess.Model); mLbl != "" {
							ui.Text(c, mLbl).FontSize(11.5).FontWeight(500).TextColor(tc.TextMuted).MaxWidth(150).SingleLine()
						}
					})

					if isRunning {
						stopBtn := ui.Row(c.Key("sp-stop")).Size(28, 28).Radius(14).
							AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer).
							Background(tc.Error.Alpha(0.18)).BorderWidth(1).BorderColor(tc.Error.Alpha(0.4))
						stopBtn.Children(func() {
							ui.Icon(c, icons.TablerSquare).Width(10).Height(10).TextColor(tc.Error)
						})
						if stopBtn.Clicked() {
							a.Agent.StopWorker(sess.ID)
						}
					} else {
						sendBtn := ui.Row(c.Key("sp-send")).Size(28, 28).Radius(14).
							AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer).
							Background(tc.MessageAction)
						sendBtn.Children(func() {
							ui.Icon(c, icons.TablerArrowUp).Width(14).Height(14).TextColor(tc.MessageActionForeground)
						})
						if sendBtn.Clicked() {
							sendSplit()
						}
					}
				})
			})
		})
	})
}
