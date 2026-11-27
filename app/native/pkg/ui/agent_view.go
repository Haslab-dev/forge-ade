package ui

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	mygoeditor "github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/hasdev/forge-ade/app/native/pkg/agent"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/settings"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
	"github.com/hasdev/forge-ade/core/agentsession"
	"github.com/hasdev/forge-ade/core/quota"
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

	// Center Column: Main Chat Container (or CLI surface) + Collapsible Bottom
	// Terminal. The wrapping Box carries Clip so the column's automatic
	// minimum width is zero: non-wrapping composer rows would otherwise
	// freeze the column at its content width and push the right drawer
	// off-screen via flex-shrink.
	ui.Box(c).Grow(1).FillHeight().Clip().Children(func() {
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
			if a.ShowAgentBottomTerminal {
				TopResizeHandle(c, &a.BottomTerminalHeight, 120, 600, 240)
				a.renderBottomTerminal(c)
			}
		})
	})
}

// chatLaneInset returns the chat content insets, honoring the Appearance
// "Chat Lane Width" setting: comfortable/wide center a 736/928px lane in
// the chat column; "full" (default) is ZCode-style left-aligned content
// with a fixed 32px gutter.
func (a *AppUI) chatLaneInset(c *ui.Context) (float32, float32) {
	if a.Mode == ModeStudio {
		return 10, 10
	}
	if a.Mode == ModeAgent && a.AgentSplitSessionID != "" && !a.IsGitClientOpen && a.AgentViewMode == AgentViewChat {
		return 16, 16
	}
	const gutter = float32(32)
	colW := a.chatColumnWidth(c)
	lane := float32(0)
	if a.ThemeMgr != nil {
		lane = a.ThemeMgr.ChatLaneWidth()
	}
	if lane <= 0 || colW-lane < 128 {
		return gutter, gutter
	}
	inset := (colW - lane) / 2
	if inset < 24 {
		inset = 24
	}
	return inset, inset
}

// renderAssistantMessageBody renders the full rich assistant message body
// (worked-for pill, thinking blocks, tool traces/groups, worker cards,
// markdown/JSON output, turn changes card, approval bar, and structured
// questions). Shared by both the primary Agent chat pane and the side-by-side
// Split Session Pane for 100% visual & functional parity.
// renderBoundedMarkdown prevents multi-hundred-KB markdown payloads from
// saturating the UI thread on every frame/stream tick, while letting users
// expand completed large responses on demand.
func (a *AppUI) renderBoundedMarkdown(c *ui.Context, tc theme.ThemeColors, as *agent.AgentState, sessionID string, msgID int, text string, streaming bool) {
	const maxMarkdownRenderBytes = 48 * 1024
	if len(text) <= maxMarkdownRenderBytes {
		if streaming {
			markdown.Stream(c, text, &as.MdOpts)
		} else {
			markdown.View(c, text, &as.MdOpts)
		}
		return
	}

	if streaming {
		// During streaming, render only the tail window at a clean newline/UTF-8 boundary.
		start := len(text) - maxMarkdownRenderBytes
		for start < len(text) && (text[start]&0xC0) == 0x80 {
			start++
		}
		if nl := strings.IndexByte(text[start:], '\n'); nl >= 0 && nl < 1024 {
			start += nl + 1
		}
		tail := "… *(streaming large output — earlier text will appear when complete)*\n\n" + text[start:]
		markdown.Stream(c, tail, &as.MdOpts)
		return
	}

	expandKey := fmt.Sprintf("fullmd-%s-%d", sessionID, msgID)
	expanded := a.ToolGroupExpanded != nil && a.ToolGroupExpanded[expandKey]
	if expanded {
		markdown.View(c, text, &as.MdOpts)
		return
	}

	end := maxMarkdownRenderBytes
	for end > 0 && (text[end]&0xC0) == 0x80 {
		end--
	}
	if nl := strings.LastIndexByte(text[:end], '\n'); nl > maxMarkdownRenderBytes/2 {
		end = nl
	}
	markdown.View(c, text[:end]+"\n\n…", &as.MdOpts)
	btn := ui.Row(c.Key("btn-"+expandKey)).Height(24).Radius(6).Padding(0, 10).Gap(5).
		AlignItems(ui.Center).Background(tc.ToolbarControl).Cursor(ui.CursorPointer)
	btn.Children(func() {
		ui.Text(c, fmt.Sprintf("Show full response (%d KB)", len(text)/1024)).FontSize(11).FontWeight(500).TextColor(tc.Accent)
	})
	if btn.Hovered() {
		btn.Background(tc.ToolbarControlHover)
	}
	if btn.Clicked() {
		if a.ToolGroupExpanded == nil {
			a.ToolGroupExpanded = make(map[string]bool)
		}
		a.ToolGroupExpanded[expandKey] = true
	}
}

func (a *AppUI) renderAssistantMessageBody(c *ui.Context, tc theme.ThemeColors, sessionID string, lastAssistantID int, m *agent.ChatMessage) {
	as := a.Agent
	if as == nil || m == nil {
		return
	}
	if len(m.Blocks) > 0 {
		open := m.ShowTools || (!m.Done && (m.IsThinking || len(m.Tools) > 0))
		hasWork := m.Thinking != "" || m.IsThinking || len(m.Tools) > 0
		if hasWork {
			a.renderWorkPill(c, tc, sessionID, m)
		}
		hasThinkingBlocks := false
		for _, b := range m.Blocks {
			if b != nil && b.Kind == "thinking" {
				hasThinkingBlocks = true
				break
			}
		}
		finalTextIdx := -1
		for bi, blk := range m.Blocks {
			if blk != nil && blk.Kind == "text" {
				finalTextIdx = bi
			}
		}
		if finalTextIdx >= 0 {
			for _, blk := range m.Blocks[finalTextIdx+1:] {
				if blk != nil && (blk.Kind == "tool" || blk.Kind == "thinking") {
					finalTextIdx = -1
					break
				}
			}
		}
		renderTextBlock := func(blk *agent.MessageBlock, streaming bool) {
			trimmed := strings.TrimSpace(blk.Text)
			if trimmed == "" {
				return
			}
			isPureJSON := len(trimmed) <= 64*1024 &&
				((strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) ||
					(strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]")))
			if isPureJSON && json.Valid([]byte(trimmed)) {
				a.renderJSONResponseCard(c, tc, m, trimmed)
				return
			}
			a.renderBoundedMarkdown(c, tc, as, sessionID, m.ID, blk.Text, streaming)
		}
		if open {
			if !hasThinkingBlocks && (m.Thinking != "" || m.IsThinking) {
				a.renderThinkingBlock(c, tc, sessionID, m)
			}
			work := ui.Column(c).Gap(6).Padding(0, 0, 0, 14).
				BorderWidth(0, 0, 0, 1).BorderColor(tc.Border.Alpha(0.6))
			work.Children(func() {
				var toolByID map[string]*agent.ToolExecution
				if len(m.Tools) > 4 {
					toolByID = make(map[string]*agent.ToolExecution, len(m.Tools))
					for _, t := range m.Tools {
						if t != nil {
							toolByID[t.ID] = t
						}
					}
				}
				toolAt := func(idx int) *agent.ToolExecution {
					if idx >= len(m.Blocks) || m.Blocks[idx] == nil || m.Blocks[idx].Kind != "tool" {
						return nil
					}
					tid := m.Blocks[idx].ToolID
					if toolByID != nil {
						return toolByID[tid]
					}
					for _, t := range m.Tools {
						if t != nil && t.ID == tid {
							return t
						}
					}
					return nil
				}
				bi := 0
				for bi < len(m.Blocks) {
					blk := m.Blocks[bi]
					if blk == nil || bi == finalTextIdx {
						bi++
						continue
					}
					if blk.Kind == "thinking" {
						isLast := bi == len(m.Blocks)-1
						a.renderThinkingBlockSegment(c, tc, sessionID, m, bi, blk.Text, m.IsThinking && !m.Done && isLast)
						bi++
						continue
					}
					if blk.Kind == "tool" {
						first := toolAt(bi)
						if first == nil {
							bi++
							continue
						}
						fam := classifyTool(first).Family
						if fam != tfExplore && fam != tfRun && fam != tfChange {
							a.renderToolTraceRow(c, tc, sessionID, m, first)
							bi++
							continue
						}
						children := []groupChild{}
						j := bi
						for j < len(m.Blocks) && j != finalTextIdx {
							b := m.Blocks[j]
							if b == nil {
								j++
								continue
							}
							if b.Kind != "tool" {
								break
							}
							t := toolAt(j)
							if t == nil {
								j++
								continue
							}
							tf := classifyTool(t).Family
							if tf != tfExplore && tf != tfRun && tf != tfChange {
								break
							}
							children = append(children, groupChild{blockIdx: j, tool: t})
							j++
						}
						if len(children) >= 2 {
							a.renderConsecutiveSummaryGroup(c, tc, sessionID, m, children,
								fmt.Sprintf("%s-%d-%d", sessionID, m.ID, bi))
							bi = j
							continue
						}
						a.renderToolTraceRow(c, tc, sessionID, m, first)
						bi++
						continue
					}
					renderTextBlock(blk, false)
					bi++
				}
				if hasThinkingBlocks && m.IsThinking && !m.Done {
					last := m.Blocks[len(m.Blocks)-1]
					if last == nil || last.Kind != "thinking" {
						a.renderThinkingBlockSegment(c, tc, sessionID, m, len(m.Blocks), "", true)
					}
				}
			})
		}
		if finalTextIdx >= 0 {
			renderTextBlock(m.Blocks[finalTextIdx], !m.Done)
		}
	} else {
		hasWork := m.Thinking != "" || m.IsThinking || len(m.Tools) > 0
		if hasWork {
			a.renderWorkPill(c, tc, sessionID, m)
			open := m.ShowTools || (!m.Done && (m.IsThinking || len(m.Tools) > 0))
			if open {
				work := ui.Column(c).Gap(6).Padding(0, 0, 0, 14).
					BorderWidth(0, 0, 0, 2).BorderColor(tc.Border.Alpha(0.6))
				work.Children(func() {
					if m.Thinking != "" || m.IsThinking {
						a.renderThinkingBlock(c, tc, sessionID, m)
					}
					if len(m.Tools) > 0 {
						a.renderFoldedToolTraces(c, tc, sessionID, m)
					}
				})
			}
		}

		if m.Text != "" {
			trimmed := strings.TrimSpace(m.Text)
			isPureJSON := len(trimmed) <= 64*1024 &&
				((strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) ||
					(strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]")))
			if isPureJSON && json.Valid([]byte(trimmed)) {
				a.renderJSONResponseCard(c, tc, m, trimmed)
			} else {
				a.renderBoundedMarkdown(c, tc, as, sessionID, m.ID, m.Text, !m.Done)
			}
		}
	}

	if len(m.Changes) > 0 {
		a.renderTurnChangesCard(c, tc, sessionID, m.ID == lastAssistantID, m)
	}
	if m.Done && m.ID == lastAssistantID && isQuotaExceededMessage(m) {
		a.renderQuotaSwitchAccountBanner(c, tc, sessionID)
	}
	if m.PendingApproval != nil && !m.PendingApproval.Resolved {
		a.renderApprovalBar(c, tc, m)
	}
	if len(m.PendingQuestions) > 0 {
		a.renderPendingQuestionsCard(c, tc, sessionID, m)
	}
}

func isQuotaExceededMessage(m *agent.ChatMessage) bool {
	if m == nil {
		return false
	}
	low := strings.ToLower(m.Text)
	if !strings.Contains(low, "error") && !strings.Contains(low, "⚠️") && !strings.Contains(low, "429") {
		return false
	}
	for _, kw := range []string{"quota", "429", "rate limit", "rate_limit", "resource_exhausted", "usage limit", "limit reached"} {
		if strings.Contains(low, kw) {
			return true
		}
	}
	return false
}

func (a *AppUI) renderQuotaSwitchAccountBanner(c *ui.Context, tc theme.ThemeColors, sessionID string) {
	if a.Agent == nil || a.Settings == nil {
		return
	}
	sess := a.Agent.FindSession(sessionID)
	if sess == nil {
		return
	}
	aid := strings.ToLower(strings.TrimSpace(sess.AgentID))
	if aid != "agent-agy" && aid != "agy" && !strings.Contains(aid, "agy") && !strings.Contains(aid, "antigravity") {
		return
	}
	mgr := a.Settings.AntigravityAccountMgr()
	if mgr == nil {
		return
	}
	accounts := mgr.Accounts()
	var others []*quota.AntigravityAccount
	for _, acc := range accounts {
		if acc != nil && !acc.IsActive {
			others = append(others, acc)
		}
	}
	if len(others) == 0 {
		return
	}

	ui.Column(c.Key("quota-switch-banner")).FillWidth().Padding(10, 12).Radius(8).Gap(8).
		Background(tc.Warning.Alpha(0.08)).BorderWidth(1).BorderColor(tc.Warning.Alpha(0.35)).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icons.TablerRefresh).Width(14).Height(14).TextColor(tc.Warning)
			ui.Text(c, "Antigravity Quota Exceeded").FontSize(12).FontWeight(600).TextColor(tc.Text)
		})
		ui.Text(c, "Stop the current session and safely switch to another saved `agy` account:").
			FontSize(11).TextColor(tc.TextMuted)
		ui.Row(c).Gap(6).Wrap().AlignItems(ui.Center).Children(func() {
			for _, other := range others {
				target := other
				label := target.Email
				if target.Label != "" {
					label = fmt.Sprintf("%s (%s)", target.Label, target.Email)
				}
				btn := ui.Row(c.Key("qswitch-"+target.ID)).Height(28).Radius(6).Padding(0, 10).Gap(6).
					AlignItems(ui.Center).Background(tc.Accent).Cursor(ui.CursorPointer)
				btn.Children(func() {
					ui.Icon(c, icons.TablerUser).Width(12).Height(12).TextColor(tc.AccentForeground)
					ui.Text(c, "Switch to "+label).FontSize(11.5).FontWeight(600).TextColor(tc.AccentForeground)
				})
				if btn.Hovered() {
					btn.Background(tc.Accent.Alpha(0.85))
				}
				if btn.Clicked() {
					a.safeSwitchAntigravityAccountFromChat(target.ID, sessionID)
					c.AddToast(ui.Toast{
						Title:       "Switching Account",
						Description: fmt.Sprintf("Stopping session and switching to %s…", target.Email),
						Type:        "info",
					})
				}
			}
		})
	})
}

func hasAssistantVisibleBody(m *agent.ChatMessage) bool {
	if m == nil {
		return false
	}
	return len(m.Blocks) > 0 || m.Text != "" || m.Thinking != "" || m.IsThinking ||
		len(m.Tools) > 0 || len(m.Changes) > 0 ||
		(m.PendingApproval != nil && !m.PendingApproval.Resolved) ||
		len(m.PendingQuestions) > 0
}

func (a *AppUI) renderAssistantResponseCard(c *ui.Context, tc theme.ThemeColors, sessionID string, lastAssistantID int, m *agent.ChatMessage) {
	if !hasAssistantVisibleBody(m) {
		return
	}
	ui.Column(c).FillWidth().Padding(14, 16).Radius(12).Gap(8).
		Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).Children(func() {
		a.renderAssistantMessageBody(c, tc, sessionID, lastAssistantID, m)
	})
}

func renderUserPromptCopyRow(c *ui.Context, tc theme.ThemeColors, key string, m *agent.ChatMessage) {
	if m == nil {
		return
	}
	trimmed := strings.TrimSpace(m.Text)
	if strings.HasPrefix(trimmed, "## Session Handoff") || strings.HasPrefix(trimmed, "## Task from Coordinator") {
		return
	}
	ui.Row(c).FillWidth().Justify(ui.End).Padding(2, 2, 0, 0).Children(func() {
		copyBtn := ui.Row(c.Key(key)).Size(22, 22).Radius(4).AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
		copyBtn.Label("Copy prompt").Tooltip("Copy prompt")
		copyBtn.Children(func() {
			ui.Icon(c, icons.TablerCopy).Width(13).Height(13).TextColor(tc.IconMuted)
		})
		if copyBtn.Hovered() {
			copyBtn.Background(tc.Text.Alpha(0.06))
		}
		if copyBtn.Clicked() {
			c.WriteClipboard(m.Text)
			c.AddToast(ui.Toast{Title: "Copied", Type: "success"})
		}
	})
}

// renderUserMessageBubble renders a user turn: either a compact collapsible
// Orca-style Handoff / Coordinator Task card (so long handoff context never
// bloats the chat) or a clean flat neutral user bubble.
func (a *AppUI) renderUserMessageBubble(c *ui.Context, tc theme.ThemeColors, sessionID string, m *agent.ChatMessage, maxWidth float32) {
	trimmed := strings.TrimSpace(m.Text)
	isHandoff := strings.HasPrefix(trimmed, "## Session Handoff")
	isCoordTask := strings.HasPrefix(trimmed, "## Task from Coordinator")

	if isHandoff || isCoordTask {
		expandKey := fmt.Sprintf("handoff-%s-%d", sessionID, m.ID)
		expanded := a.ToolGroupExpanded != nil && a.ToolGroupExpanded[expandKey]

		// Extract concise header metadata from the structured markdown prompt.
		titlePrefix := "Handoff loaded"
		detailLabel := ""
		fileCount := 0
		if isCoordTask {
			titlePrefix = "Task from coordinator"
		}
		for _, line := range strings.Split(trimmed, "\n") {
			l := strings.TrimSpace(line)
			if strings.HasPrefix(l, "- **From:**") {
				detailLabel = strings.TrimSpace(strings.TrimPrefix(l, "- **From:**"))
				titlePrefix = "Handoff from"
			} else if strings.HasPrefix(l, "**Worker:**") {
				detailLabel = strings.Trim(strings.TrimSpace(strings.TrimPrefix(l, "**Worker:**")), "`")
			} else if strings.HasPrefix(l, "- `") && strings.HasSuffix(l, "`") {
				fileCount++
			}
		}

		card := ui.Column(c.Key("ho-card-" + expandKey)).FillWidth().Radius(10).
			Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).Clip()
		card.Children(func() {
			hdr := ui.Row(c.Key("ho-hdr-"+expandKey)).Height(36).Padding(0, 12).Gap(7).
				AlignItems(ui.Center).Cursor(ui.CursorPointer)
			hdr.Children(func() {
				ui.Icon(c, icons.TablerGitFork).Width(13).Height(13).TextColor(tc.Accent)
				ui.Text(c, titlePrefix).FontSize(12).TextColor(tc.TextMuted).SingleLine()
				if detailLabel != "" {
					ui.Text(c, detailLabel).FontSize(12).FontWeight(600).TextColor(tc.Text).Grow(1).MinWidth(0).SingleLine()
				} else {
					ui.Box(c).Grow(1)
				}
				if fileCount > 0 {
					ui.Row(c).Height(18).Radius(4).Padding(0, 6).AlignItems(ui.Center).
						Background(tc.ToolbarControl).Children(func() {
						ui.Text(c, fmt.Sprintf("%d files", fileCount)).FontSize(10).FontWeight(500).TextColor(tc.TextMuted)
					})
				} else {
					ui.Row(c).Height(18).Radius(4).Padding(0, 6).AlignItems(ui.Center).
						Background(tc.ToolbarControl).Children(func() {
						ui.Text(c, "Context loaded").FontSize(10).FontWeight(500).TextColor(tc.TextMuted)
					})
				}
				chev := icons.TablerChevronRight
				if expanded {
					chev = icons.TablerChevronDown
				}
				ui.Icon(c, chev).Width(12).Height(12).TextColor(tc.IconMuted)
			})
			if hdr.Hovered() {
				hdr.Background(tc.SurfaceRaised)
			}
			if hdr.Clicked() {
				if a.ToolGroupExpanded == nil {
					a.ToolGroupExpanded = make(map[string]bool)
				}
				a.ToolGroupExpanded[expandKey] = !expanded
			}

			if expanded {
				ui.Scroll(c.Key("ho-body-"+expandKey)).MaxHeight(220).
					BorderWidth(1, 0, 0, 0).BorderColor(tc.Border).Children(func() {
					ui.Box(c).Padding(10, 12).Children(func() {
						ui.Text(c, trimmed).Selectable().FontSize(11.5).LineHeight(1.45).TextColor(tc.TextMuted)
					})
				})
			}
		})
		return
	}

	ui.Row(c).Justify(ui.End).FillWidth().Children(func() {
		bubble := ui.Column(c).Padding(10, 14).Radius(10).
			Background(tc.MessageSurface).BorderWidth(1).BorderColor(tc.Border).
			MaxWidth(maxWidth)
		bubble.Children(func() {
			const maxUserPromptBytes = 16 * 1024
			displayPrompt := m.Text
			truncatedPrompt := false
			expandKey := fmt.Sprintf("fulluser-%s-%d", sessionID, m.ID)
			if len(displayPrompt) > maxUserPromptBytes && (a.ToolGroupExpanded == nil || !a.ToolGroupExpanded[expandKey]) {
				end := 8 * 1024
				for end > 0 && (displayPrompt[end]&0xC0) == 0x80 {
					end--
				}
				displayPrompt = displayPrompt[:end] + "\n…"
				truncatedPrompt = true
			}
			ui.Text(c, displayPrompt).Selectable().FontSize(13).LineHeight(1.45).TextColor(tc.MessageForeground)
			if truncatedPrompt {
				moreBtn := ui.Row(c.Key("btn-"+expandKey)).Padding(4, 0, 0, 0).Cursor(ui.CursorPointer)
				moreBtn.Children(func() {
					ui.Text(c, fmt.Sprintf("Show full prompt (%d KB)", len(m.Text)/1024)).FontSize(11).FontWeight(500).TextColor(tc.Accent)
				})
				if moreBtn.Clicked() {
					if a.ToolGroupExpanded == nil {
						a.ToolGroupExpanded = make(map[string]bool)
					}
					a.ToolGroupExpanded[expandKey] = true
				}
			}
			atts := userMessageAttachments(m.Text)
			if len(atts) > 0 {
				chipRow := ui.Row(c).Gap(5).Wrap().Padding(6, 0, 0, 0)
				chipRow.Children(func() {
					for _, att := range atts {
						att := att
						chip := ui.Row(c.Key("uatt-"+att)).Height(24).Radius(6).Padding(0, 8).Gap(5).AlignItems(ui.Center).
							Background(tc.Accent.Alpha(0.12)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.3))
						chip.Children(func() {
							icons.RenderFileIcon(c, att, 12, tc.Accent)
							ui.Text(c, filepath.Base(att)).FontSize(11).FontWeight(500)
						})
						if chip.Clicked() {
							a.openSidePreviewFile(c, att)
						}
					}
				})
			}
		})
	})
}

// modelEncodesEffort reports whether the harness or model identifier already
// encodes reasoning effort (e.g. Antigravity/AGY models ending in -low,
// -medium, -high, -xhigh), so showing a separate Reasoning Effort badge or
// toggle would be redundant.
func modelEncodesEffort(agentID, model string) bool {
	aid := strings.ToLower(strings.TrimSpace(agentID))
	if aid == "agent-agy" || strings.Contains(aid, "agy") || strings.Contains(aid, "antigravity") {
		return true
	}
	m := strings.ToLower(strings.TrimSpace(model))
	for _, suf := range []string{"-low", "-medium", "-high", "-xhigh", "-minimal", "-sonar"} {
		if strings.HasSuffix(m, suf) {
			return true
		}
	}
	return false
}

// shortHarnessLabel compresses verbose ACP/persona names for the clean
// composer control row (e.g. "Google Antigravity (AGY)" -> "AGY").
func shortHarnessLabel(name string) string {
	low := strings.ToLower(strings.TrimSpace(name))
	switch {
	case strings.Contains(low, "antigravity") || strings.Contains(low, "agy"):
		return "AGY"
	case strings.Contains(low, "claude"):
		return "Claude"
	case strings.Contains(low, "codex"):
		return "Codex"
	case strings.Contains(low, "gemini"):
		return "Gemini"
	case strings.Contains(low, "opencode"):
		return "OpenCode"
	case strings.Contains(low, "orchestrator"):
		return "Orchestrator"
	}
	s := strings.TrimSpace(strings.TrimSuffix(name, "Agent"))
	if runes := []rune(s); len(runes) > 14 {
		return string(runes[:13]) + "…"
	}
	return s
}

// shortModelLabel formats a concise model name for headers and chips
// (rune-safe truncation: model names may contain multibyte characters).
func shortModelLabel(model string) string {
	if model == "" {
		return "Select Model"
	}
	if idx := strings.Index(model, " ("); idx != -1 {
		model = model[:idx]
	}
	if runes := []rune(model); len(runes) > 24 {
		return string(runes[:22]) + "…"
	}
	return model
}

// openFileInEditor opens path (relative or absolute) in Editor Mode.
func (a *AppUI) openFileInEditor(path string) {
	if path == "" || a.Editor == nil {
		return
	}
	root := ""
	if a.Workspace != nil {
		root = a.Workspace.RootDir
	}
	fullPath := path
	relPath := path
	if filepath.IsAbs(path) {
		if root != "" {
			if rel, err := filepath.Rel(root, path); err == nil && !strings.HasPrefix(rel, "..") {
				relPath = rel
			}
		}
	} else if root != "" {
		fullPath = filepath.Join(root, path)
	}
	_, _ = a.Editor.OpenFile(fullPath, relPath)
	a.Mode = ModeEditor
	a.IsGitClientOpen = false
}

func (a *AppUI) showFloatingInspector(c *ui.Context) bool {
	return false
}

// renderAgentChat renders the active conversation view, message bubbles, task checklist, and prompt composer.
func (a *AppUI) renderAgentChat(c *ui.Context) {
	tc := a.ThemeMgr.Colors()
	as := a.Agent
	if as == nil {
		return
	}
	// One consistent per-frame copy of the transcript under the state lock:
	// bus goroutines mutate ChatMessage fields in place while streaming, so
	// the render must never read the live message pointers (or session
	// scalar fields) directly.
	msgs, sessionID, checkpointTurnID := as.ActiveTranscriptSnapshot()

	if sessionID == "" || len(msgs) == 0 {
		if a.Mode == ModeStudio {
			a.renderStudioAgentHome(c)
			return
		}
		a.renderAgentHome(c)
		return
	}

	a.setupMarkdownOptions(as, tc)

	as.ListState.Key = func(i int) any {
		if i < len(msgs) {
			return msgs[i].ID
		}
		return i
	}
	if a.LastFollowSessionID != sessionID {
		a.LastFollowSessionID = sessionID
		as.ListState.FollowEnd = true
	}
	as.ListState.FollowAnimation = &ui.Smooth

	// The checkpoint restore action is honest only on the message the active
	// checkpoint belongs to: RestoreLastTurnCheckpoint always reverts the
	// LAST turn, so only the last assistant message offers it (17b).
	lastAssistantID := 0
	for j := len(msgs) - 1; j >= 0; j-- {
		if msgs[j] != nil && msgs[j].Role == "assistant" {
			lastAssistantID = msgs[j].ID
			break
		}
	}

	leftInset, _ := a.chatLaneInset(c)
	rightInset := leftInset
	if a.showFloatingInspector(c) {
		rightInset = 324
	}

	// The chat fills a box so the ZCode floating inspector card can overlay it
	// at the top-right cleanly. Clip contains any overflow.
	ui.Box(c).Fill().Clip().Children(func() {
		ui.Column(c).Fill().Background(tc.Canvas).Children(func() {
			// Find-in-thread bar (⌘F), ThreadFindBar parity.
			if a.ShowThreadFind {
				a.renderThreadFindBar(c, tc, as)
			}

			// Turn scrubber + chat lane.
			transcriptWidth := a.chatColumnWidth(c)
			ui.Row(c).Grow(1).MaxWidth(transcriptWidth).Children(func() {
				a.renderTurnScrubber(c, tc, as, msgs)
				ui.List(c, &as.ListState, len(msgs), func(i int) {
					if i >= len(msgs) || msgs[i] == nil {
						return
					}
					m := msgs[i]
					if m.Role == "user" {
						ui.Column(c.Key("urow")).Padding(10, rightInset, 8, leftInset).Gap(4).AlignItems(ui.End).Children(func() {
							a.renderUserMessageBubble(c, tc, sessionID, m, 620)
							renderUserPromptCopyRow(c, tc, "ucopy-"+fmt.Sprint(m.ID), m)
						})
					} else {
						// Agent message: response card with work traces -> markdown ->
						// changes card, followed by bottom-left action icons + timestamp.
						ui.Column(c.Key("arow")).Padding(8, rightInset, 10, leftInset).Gap(6).Children(func() {
							a.renderAssistantResponseCard(c, tc, sessionID, lastAssistantID, m)

							// 5. ZCode Bottom-Left Action Icons + Timestamp
							if m.Done && (m.Text != "" || len(m.Tools) > 0) {
								ui.Row(c).Gap(6).Padding(2, 0, 0, 0).AlignItems(ui.Center).Children(func() {
									acopy := ui.Row(c.Key("acopy-"+fmt.Sprint(m.ID))).Size(22, 22).Radius(4).AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
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

									upBtn := ui.Row(c.Key("thumb-up-"+fmt.Sprint(m.ID))).Size(22, 22).Radius(4).AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
									upColor := tc.IconMuted
									if m.Feedback == "up" {
										upColor = tc.Success
									}
									upBtn.Children(func() {
										ui.Icon(c, icons.TablerThumbUp).Width(13).Height(13).TextColor(upColor)
									})
									if upBtn.Hovered() {
										upBtn.Background(tc.Text.Alpha(0.06))
									}
									if upBtn.Clicked() {
										// Toggle the live message under the lock (m is a
										// frame snapshot) and persist the session (17c).
										if as.ToggleMessageFeedback(sessionID, m.ID, "up") {
											c.AddToast(ui.Toast{Title: "Response rated helpful", Type: "success"})
										}
									}

									downBtn := ui.Row(c.Key("thumb-down-"+fmt.Sprint(m.ID))).Size(22, 22).Radius(4).AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
									downColor := tc.IconMuted
									if m.Feedback == "down" {
										downColor = tc.Error
									}
									downBtn.Children(func() {
										ui.Icon(c, icons.TablerThumbDown).Width(13).Height(13).TextColor(downColor)
									})
									if downBtn.Hovered() {
										downBtn.Background(tc.Text.Alpha(0.06))
									}
									if downBtn.Clicked() {
										if as.ToggleMessageFeedback(sessionID, m.ID, "down") {
											c.AddToast(ui.Toast{Title: "Response rated unhelpful", Type: "info"})
										}
									}

									if checkpointTurnID != "" && m.ID == lastAssistantID {
										cpBtn := ui.Row(c.Key("cp-restore-"+fmt.Sprint(m.ID))).Size(22, 22).Radius(4).AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
										cpBtn.Children(func() {
											ui.Icon(c, icons.TablerRotate).Width(12).Height(12).TextColor(tc.IconMuted)
										})
										if cpBtn.Hovered() {
											cpBtn.Background(tc.Text.Alpha(0.06))
										}
										if cpBtn.Clicked() {
											sid := sessionID
											go func() {
												err := as.RestoreLastTurnCheckpoint(sid)
												if a.Git != nil {
													a.Git.Refresh()
												}
												// Context.AddToast silently drops calls made
												// outside a frame — queue from goroutines.
												if err == nil {
													a.queueToast(ui.Toast{Title: "Checkpoint Restored", Type: "success"})
												} else {
													a.queueToast(ui.Toast{Title: "Restore Failed", Description: err.Error(), Type: "error"})
												}
												if a.Window != nil {
													a.Window.Invalidate()
												}
											}()
										}
									}

									if !m.Timestamp.IsZero() {
										ui.Text(c, m.Timestamp.Format("03:04 PM")).FontSize(11).TextColor(tc.TextMuted)
									}
								})
							}
						})
					}
				}).Grow(1).FillHeight().Padding(16, 0, 16, 0).Gap(14).Justify(ui.End).Clip()
			})

			// Mid-turn Steering Queue Tray
			if len(as.SteerQueueSnapshot()) > 0 {
				a.renderSteerQueueTray(c)
			}

			// Composer
			a.renderComposer(c, tc, nil)
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

		// Hide floating status capsule during Split Pane so it never overlaps
		// narrow split transcripts (diff stats remain visible in the bottom bar).
		if a.AgentSplitSessionID == "" {
			a.renderStatusPanel(c, tc, msgs)
		}
	})
}

// liveChangesStats returns the floating pill's totals (additions, deletions,
// changed files, whether anything changed). While a turn is running the git
// working tree is the live source (numstat polled by the streaming ticker);
// once the turn finalizes, the last assistant message's turn-scoped Changes win.
func (a *AppUI) liveChangesStats(msgs []*agent.ChatMessage) (add, del, files int, ok bool) {
	if a.Agent == nil {
		return 0, 0, 0, false
	}
	if a.Agent.HasRunningSession() && a.Git != nil {
		snap := a.Git.Snapshot()
		all := append(append(snap.Staged, snap.Unstaged...), snap.Untracked...)
		for _, fs := range all {
			files++
			if fs.Additions > 0 {
				add += fs.Additions
			}
			if fs.Deletions > 0 {
				del += fs.Deletions
			}
		}
		if files > 0 {
			return add, del, files, true
		}
		return 0, 0, 0, false
	}
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		if m == nil || m.Role != "assistant" {
			continue
		}
		for _, ch := range m.Changes {
			if ch == nil {
				continue
			}
			files++
			if ch.Additions > 0 {
				add += ch.Additions
			}
			if ch.Deletions > 0 {
				del += ch.Deletions
			}
		}
		break
	}
	return add, del, files, files > 0
}

func (a *AppUI) renderWorkPill(c *ui.Context, tc theme.ThemeColors, sessionID string, m *agent.ChatMessage) {
	running := !m.Done

	// Live elapsed while the turn runs; the app ticks once a second while
	// streaming, so this label counts up like ZCode's "Working for 12s".
	label := "Working"
	if running {
		if !m.TurnStartedAt.IsZero() {
			label = fmt.Sprintf("Working for %s", formatWorkDuration(time.Since(m.TurnStartedAt)))
		}
	} else {
		// forge-ade's workSecs: thought durations when known, otherwise an
		// estimate of two seconds per tool run — never "0s".
		durationSec := m.ThinkingTimeSec
		if durationSec <= 0 {
			durationSec = math.Max(float64(len(m.Tools))*2, 2)
		}
		label = fmt.Sprintf("Worked for %s", formatWorkDuration(time.Duration(durationSec*float64(time.Second))))
	}

	open := m.ShowTools || running
	// Orca-style ultra-minimal work summary line: "Worked for 32s >" + subtle hairline divider
	header := ui.Row(c).Height(26).Radius(6).Padding(0, 4).Gap(6).AlignItems(ui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(tc.Border.Alpha(0.7)).Cursor(ui.CursorPointer)
	header.Children(func() {
		if running {
			ui.Spinner(c).Size(11, 11).Label("")
			ui.Text(c, label).FontSize(12).FontWeight(500).TextColor(shimmerTextColor(tc))
		} else {
			ui.Text(c, label).FontSize(12).FontWeight(400).TextColor(tc.TextMuted)
		}
		chev := icons.TablerChevronRight
		if open {
			chev = icons.TablerChevronDown
		}
		chevColor := tc.IconMuted.Alpha(0.75)
		if header.Hovered() {
			chevColor = tc.Text
		}
		ui.Icon(c, chev).Width(12).Height(12).TextColor(chevColor)
		ui.Box(c).Grow(1)
	})
	if header.Hovered() {
		header.Background(tc.Text.Alpha(0.04))
	}
	if header.Clicked() {
		// m is a frame snapshot; flip the live message under the lock.
		a.Agent.MutateMessage(sessionID, m.ID, func(lm *agent.ChatMessage) {
			lm.ShowTools = !lm.ShowTools
		})
	}
}

func (a *AppUI) renderThinkingBlock(c *ui.Context, tc theme.ThemeColors, sessionID string, m *agent.ChatMessage) {
	a.renderThinkingBlockSegment(c, tc, sessionID, m, -1, m.Thinking, m.IsThinking && !m.Done)
}

func (a *AppUI) renderThinkingBlockSegment(c *ui.Context, tc theme.ThemeColors, sessionID string, m *agent.ChatMessage, blockIdx int, text string, isThinking bool) {
	// ZCode reasoning rows: collapsed by default even while streaming — the
	// header shimmers "Thinking" with a one-line preview of the newest
	// thought; completed turns read "Thought · 4.2s". Body is plain muted
	// text behind a left guide rail.
	label := "Thought"
	if isThinking {
		label = "Thinking"
	} else {
		// ZCode: sub-second thoughts read "a few seconds", never "0s".
		durationText := "a few seconds"
		if blockIdx < 0 && m.ThinkingTimeSec >= 1 {
			durationText = formatWorkDuration(time.Duration(m.ThinkingTimeSec * float64(time.Second)))
		} else if blockIdx >= 0 {
			// Per-block duration from the timeline stamps, falling back to
			// the message-level total for legacy sessions.
			if n := len(m.Blocks); blockIdx < n && m.Blocks[blockIdx] != nil && m.Blocks[blockIdx].StartedAtMs > 0 {
				blk := m.Blocks[blockIdx]
				endMs := blk.EndedAtMs
				if endMs == 0 {
					endMs = time.Now().UnixMilli()
				}
				if secs := float64(endMs-blk.StartedAtMs) / 1000; secs >= 1 {
					durationText = formatWorkDuration(time.Duration(maxInt64(endMs-blk.StartedAtMs, 0) * int64(time.Millisecond)))
				}
			} else if m.ThinkingTimeSec >= 1 {
				durationText = formatWorkDuration(time.Duration(m.ThinkingTimeSec * float64(time.Second)))
			}
		}
		label = "Thought · " + durationText
	}

	expandKey := fmt.Sprintf("%s-%d-%d", sessionID, m.ID, blockIdx)
	expanded := m.ShowThinking
	if blockIdx >= 0 && a.ThinkingBlockExpanded != nil {
		if v, ok := a.ThinkingBlockExpanded[expandKey]; ok {
			expanded = v
		}
	}

	ui.Column(c.Key("think-" + expandKey)).FillWidth().Gap(2).Children(func() {
		header := ui.Row(c).Radius(6).Padding(3, 8).Gap(8).AlignItems(ui.Center).MinHeight(30).Cursor(ui.CursorPointer)
		copyClicked := false
		header.Children(func() {
			if isThinking {
				ui.Spinner(c).Size(12, 12).Label("")
			} else {
				ui.Icon(c, icons.TablerBrain).Width(15).Height(15).TextColor(tc.IconMuted)
			}
			labelColor := tc.SecondaryLabel
			if isThinking {
				labelColor = shimmerTextColor(tc)
			}
			ui.Text(c, label).FontSize(13).TextColor(labelColor).FontWeight(500)
			if isThinking && !expanded {
				// Live one-line preview of the newest thought, clamped.
				if line := lastNonEmptyLine(text); line != "" {
					ui.Text(c, clampRunes(line, 70)).FontSize(13).TextColor(tc.TextMuted.Alpha(0.8)).Grow(1).SingleLine()
				} else {
					ui.Box(c).Grow(1)
				}
			} else {
				ui.Box(c).Grow(1)
			}

			if text != "" {
				// Copy affordance appears on hover only, like ZCode's
				// hover-revealed header actions.
				copyBtn := ui.Row(c.Key("think-copy-"+expandKey)).Size(20, 20).Radius(4).AlignItems(ui.Center).Justify(ui.Center).
					Cursor(ui.CursorPointer).Tooltip("Copy reasoning")
				copyBtn.Children(func() {
					copyColor := ui.RGBA(0, 0, 0, 0)
					if header.Hovered() {
						copyColor = tc.IconMuted
					}
					ui.Icon(c, icons.TablerCopy).Width(11).Height(11).TextColor(copyColor)
				})
				if copyBtn.Hovered() {
					copyBtn.Background(tc.Text.Alpha(0.06))
				}
				if copyBtn.Clicked() {
					copyClicked = true
					mygo.Clipboard.WriteText(text)
					c.AddToast(ui.Toast{Title: "Copied Reasoning", Description: "Chain of thought copied to clipboard", Type: "success"})
				}
			}

			chev := icons.TablerChevronRight
			if expanded {
				chev = icons.TablerChevronDown
			}
			chevColor := ui.RGBA(0, 0, 0, 0) // hidden until hover, ZCode-style
			if expanded || header.Hovered() {
				chevColor = tc.IconMuted
			}
			ui.Icon(c, chev).Width(12).Height(12).TextColor(chevColor)
		})
		if header.Hovered() {
			header.Background(tc.Text.Alpha(0.04))
		}
		if !copyClicked && header.Clicked() {
			if blockIdx >= 0 {
				if a.ThinkingBlockExpanded == nil {
					a.ThinkingBlockExpanded = make(map[string]bool)
				}
				a.ThinkingBlockExpanded[expandKey] = !expanded
			} else {
				a.Agent.MutateMessage(sessionID, m.ID, func(lm *agent.ChatMessage) {
					lm.ShowThinking = !lm.ShowThinking
				})
			}
		}

		// Collapsed by default — even mid-stream. Only an explicit toggle
		// opens the CoT body.
		if expanded && text != "" {
			if a.ThinkingScrollStates == nil {
				a.ThinkingScrollStates = make(map[string]*ui.ScrollState)
			}
			ss := a.ThinkingScrollStates[expandKey]
			if ss == nil {
				ss = &ui.ScrollState{Y: math.MaxFloat32}
				a.ThinkingScrollStates[expandKey] = ss
			}
			if isThinking {
				// While streaming, automatically follow newest thinking tokens to the bottom
				// unless the user intentionally scrolled up to read earlier reasoning.
				if ss.MaxY == 0 || ss.Y >= ss.MaxY-30 {
					ss.Y = math.MaxFloat32
				}
			}
			ui.Column(c).Padding(0, 0, 0, 14).BorderWidth(0, 0, 0, 1).BorderColor(tc.Border.Alpha(0.6)).Children(func() {
				ui.Scroll(c).TrackScroll(ss).MaxHeight(240).Children(func() {
					ui.Column(c).Padding(0, 0, 4, 10).Children(func() {
						displayText := text
						const maxThinkingRenderBytes = 24 * 1024
						if len(displayText) > maxThinkingRenderBytes {
							start := len(displayText) - maxThinkingRenderBytes
							for start < len(displayText) && (displayText[start]&0xC0) == 0x80 {
								start++
							}
							displayText = "… [earlier reasoning hidden; use Copy for full text]\n\n" + displayText[start:]
						}
						ui.Text(c, displayText).FontSize(11.5).TextColor(tc.TextMuted).Selectable()
					})
				})
			})
		}
	})
}

// maxInt64 returns the larger of two int64 values.
func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// renderFoldedToolTraces draws forge-ade's folded tool traces: a summary bar
// ("Ran 2 commands · Read 1 file · Edited 2 files" + Hide-N-steps), then
// per-tool rows — chevron, kind icon, [Write]/[Read]/[$] badge, mono label,
// output line count — with forge-ade's per-tool expanded views. Skill tools
// are excluded from the rows (counted in the summary only), as the original
// does with isSkillTool.
func (a *AppUI) renderFoldedToolTraces(c *ui.Context, tc theme.ThemeColors, sessionID string, m *agent.ChatMessage) {
	if len(m.Tools) == 0 {
		return
	}

	// All tools including skills, MCP calls, design tools, and subagents are rendered in traces
	ordinary := m.Tools

	ui.Column(c).FillWidth().Gap(3).Children(func() {
		// Group summary row for multi-tool execution
		if len(m.Tools) > 1 {
			a.renderToolGroupBar(c, tc, sessionID, m)
		}

		// Individual tool rows
		if m.ShowTools || (!m.Done && len(ordinary) > 0) {
			for _, tool := range ordinary {
				a.renderToolTraceRow(c, tc, sessionID, m, tool)
			}
		}
	})
}

// renderToolGroupBar draws the collapsible multi-tool summary row ("Running
// tools … Show N steps") shared by the folded traces list and the block
// timeline.
func (a *AppUI) renderToolGroupBar(c *ui.Context, tc theme.ThemeColors, sessionID string, m *agent.ChatMessage) {
	summary, hasRunning, hasFailed := summarizeTools(m.Tools)

	groupBar := ui.Row(c).Height(28).Radius(7).Padding(0, 8).Gap(7).AlignItems(ui.Center).
		Cursor(ui.CursorPointer)
	groupBar.Children(func() {
		if hasRunning {
			ui.Spinner(c).Size(12, 12).Label("Running tools")
		} else if hasFailed {
			ui.Icon(c, icons.TablerAlertTriangle).Width(13).Height(13).TextColor(tc.Error)
		} else {
			ui.Box(c).Size(6, 6).Radius(3).Background(tc.Success).Shrink(0)
		}

		summaryColor := tc.TextMuted
		if hasRunning {
			summaryColor = tc.Text
		} else if hasFailed {
			summaryColor = tc.Error
		}
		ui.Text(c, summary).FontSize(12).FontWeight(500).TextColor(summaryColor).SingleLine()

		stepLabel := fmt.Sprintf("%d steps", len(m.Tools))
		ui.Text(c, "· "+stepLabel).FontSize(11).TextColor(tc.TextMuted.Alpha(0.75)).SingleLine()

		chev := icons.TablerChevronRight
		if m.ShowTools {
			chev = icons.TablerChevronDown
		}
		ui.Icon(c, chev).Width(12).Height(12).TextColor(tc.IconMuted)
		ui.Box(c).Grow(1)
	})
	if groupBar.Hovered() {
		groupBar.Background(tc.Surface.Alpha(0.8))
	}
	if groupBar.Clicked() {
		a.Agent.MutateMessage(sessionID, m.ID, func(lm *agent.ChatMessage) {
			lm.ShowTools = !lm.ShowTools
		})
	}
}

// renderToolTraceRow renders one tool call as a ZCode-v4 summary row:
// muted icon + kind label (shimmer while running, no spinner) + summary
// text + "+N −N" diffstat + status, with the detail view expanding into an
// indented card below. Shared by the work lane, the consecutive-step groups
// and the folded legacy traces.
func (a *AppUI) renderToolTraceRow(c *ui.Context, tc theme.ThemeColors, sessionID string, m *agent.ChatMessage, thisTool *agent.ToolExecution) {
	a.renderToolRowEx(c, tc, sessionID, m, thisTool, false)
}

// renderToolRowEx is renderToolTraceRow with a nested flag: children inside
// a consecutive-step group drop the leading family icon (the rail already
// groups them) and tighten the row height, ZCode's "child icons hidden".
func (a *AppUI) renderToolRowEx(c *ui.Context, tc theme.ThemeColors, sessionID string, m *agent.ChatMessage, thisTool *agent.ToolExecution, nested bool) {
	v := classifyTool(thisTool)
	if v.IsSubagent {
		// ZCode renders Agent/Task calls as a single face-tile summary row
		// that opens the child timeline in the right drawer — never an
		// inline expandable card.
		a.renderAgentToolRow(c, tc, sessionID, m, thisTool, v)
		return
	}
	args := parseToolArgsCached(thisTool)
	failed := thisTool.Status == "failed" || strings.Contains(thisTool.Output, "<tool_use_error>")
	running := thisTool.Status == "running"
	isRead := v.Family == tfExplore && v.IsRead
	isEdit := v.IsEdit
	isBash := v.IsBash
	isSearch := v.Family == tfExplore && !v.IsRead

	// Output line count on the right — only when it carries information.
	// Meta-only read results ("4104 lines, 140821 bytes") render as one
	// line, which ZCode never shows.
	outLines := 0
	if thisTool.Output != "" && (isRead || isSearch) && !readMetaOnlyOutput(thisTool.Output) {
		outLines = strings.Count(thisTool.Output, "\n") + 1
	}
	rowHeight := float32(30)
	if nested {
		rowHeight = 28
	}

	ui.Column(c.Key("tool-" + thisTool.ID)).FillWidth().Children(func() {
		// ZCode ToolSummaryRow: family icon, kind label, content (filename
		// bright + directory muted / mono command), diffstat, then the
		// chevron trailing at the row's right edge, hidden until hover.
		row := ui.Row(c).Height(rowHeight).Radius(6).Padding(0, 8).Gap(8).AlignItems(ui.Center).Cursor(ui.CursorPointer)
		labelClicked := false
		row.Children(func() {
			if !nested {
				if running {
					ui.Spinner(c).Size(12, 12).Label("")
				} else {
					ui.Icon(c, v.Icon).Width(15).Height(15).TextColor(tc.IconMuted)
				}
			}
			kindColor := tc.SecondaryLabel
			if running {
				kindColor = shimmerTextColor(tc)
			}
			ui.Text(c, v.KindLabel).FontSize(13).FontWeight(500).TextColor(kindColor)
			// Source chip survives only where the kind label doesn't carry
			// the provenance: MCP server, skill, Forge Design.
			if v.Badge != "" && (v.Family == tfMCP || v.Family == tfSkill || v.Family == tfDesign) {
				ui.Box(c).Radius(4).Padding(1, 5).Background(tc.Text.Alpha(0.06)).
					BorderWidth(1).BorderColor(tc.Border.Alpha(0.5)).Children(func() {
					ui.Text(c, v.Badge).FontSize(10).Font("monospace").FontWeight(600).TextColor(tc.TextMuted)
				})
			}
			if v.FilePath != "" && v.Label != "" {
				// ZCode file content: workspace-relative leaf name in bright
				// text plus the parent directory muted — not one mono chip.
				// Click opens the file in the right drawer's side file pane.
				rel := a.displayToolPath(v.FilePath)
				leaf := filepath.Base(rel)
				dir := ""
				if d := filepath.Dir(rel); d != "." && d != "" && d != "/" {
					dir = d + "/"
				}
				fileChip := ui.Row(c.Key("tfile-"+thisTool.ID)).Gap(5).AlignItems(ui.Center).
					Radius(4).Padding(1, 4).MinWidth(0).Cursor(ui.CursorPointer)
				fileChip.Children(func() {
					icons.RenderFileIcon(c, leaf, 14, tc.IconMuted)
					ui.Text(c, leaf).FontSize(13).FontWeight(500).TextColor(tc.Text).SingleLine()
					if dir != "" {
						ui.Text(c, dir).FontSize(12).TextColor(tc.TextMuted).SingleLine()
					}
				})
				if fileChip.Hovered() {
					fileChip.Background(tc.Text.Alpha(0.07))
				}
				if fileChip.Clicked() {
					labelClicked = true
					if v.Family == tfChange {
						a.openTranscriptFileInSidePaneDiff(v.FilePath)
					} else {
						a.openTranscriptFileInSidePane(v.FilePath)
					}
				}
			} else if v.Label != "" {
				lbl := ui.Text(c, v.Label).FontSize(13).TextColor(tc.Text).SingleLine()
				if v.Family == tfRun || v.Family == tfMCP || isSearch {
					lbl.Font("monospace")
				}
			}
			// "+N −N" diffstat on edit rows, ZCode-style colored mono,
			// sitting right after the content like ZCode's Edit rows.
			if isEdit {
				if add, del, ok := toolDiffStatCached(thisTool); ok {
					ui.Text(c, fmt.Sprintf("+%d", add)).FontSize(12).Font("monospace").TextColor(tc.DiffAdditionForeground)
					ui.Text(c, fmt.Sprintf("-%d", del)).FontSize(12).Font("monospace").TextColor(tc.DiffDeletionForeground)
				}
			}
			if outLines > 1 {
				ui.Text(c, fmt.Sprintf("%d lines", outLines)).FontSize(11).Font("monospace").TextColor(tc.TextMuted)
			}
			if failed {
				ui.Text(c, "Failed").FontSize(12).FontWeight(600).TextColor(tc.Error)
			}
			// Trailing expand affordance: pushes to the row's right edge,
			// invisible until hover or expanded — ZCode's trailing chevron.
			ui.Box(c).Grow(1)
			chev := icons.TablerChevronRight
			if thisTool.Expanded {
				chev = icons.TablerChevronDown
			}
			chevColor := ui.RGBA(0, 0, 0, 0) // invisible until hover
			if thisTool.Expanded || row.Hovered() {
				chevColor = tc.IconMuted
			}
			ui.Icon(c, chev).Width(14).Height(14).TextColor(chevColor)
		})
		if row.Hovered() {
			row.Background(tc.Text.Alpha(0.04))
		}
		if row.Clicked() && !labelClicked {
			// thisTool is a per-frame snapshot copy — persist the toggle on
			// the live message so it survives the next frame.
			a.Agent.MutateMessage(sessionID, m.ID, func(lm *agent.ChatMessage) {
				for _, t := range lm.Tools {
					if t != nil && t.ID == thisTool.ID {
						t.Expanded = !t.Expanded
						break
					}
				}
			})
		}

		// Expanded detail views, mirroring forge-ade's tool blocks. ZCode
		// indents the body under the summary and lets each detail renderer
		// draw its own rounded card — no extra chrome here.
		if thisTool.Expanded {
			ui.Scroll(c).MaxHeight(340).Children(func() {
				ui.Column(c).Padding(6, 0, 4, 24).Gap(6).Children(func() {
					cleanOut := thisTool.Output
					if failed {
						cleanOut = strings.ReplaceAll(cleanOut, "<tool_use_error>", "")
						cleanOut = strings.ReplaceAll(cleanOut, "</tool_use_error>", "")
						cleanOut = strings.TrimSpace(cleanOut)
					}

					parsedPatch := extractDiffFromToolOutput(cleanOut, args.Path)
					switch {
					case v.Family == tfSkill:
						a.renderToolSkillBlock(c, tc, thisTool, cleanOut, failed)
					case v.Family == tfMCP && v.IsMCPLoad:
						a.renderToolMCPLoadBlock(c, tc, thisTool, cleanOut, failed)
					case v.Family == tfMCP:
						a.renderToolMCPCallBlock(c, tc, thisTool, cleanOut, failed)
					case v.Family == tfDesign:
						a.renderToolDesignBlock(c, tc, thisTool, cleanOut, failed)
					case isRead && args.Path != "" && readMetaOnlyOutput(cleanOut):
						// Harness returned only the size meta line — no content
						// to preview; a muted note beats a bloated editor card.
						ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
							ui.Icon(c, icons.TablerFileText).Width(12).Height(12).TextColor(tc.IconMuted)
							ui.Text(c, cleanOut).FontSize(11).TextColor(tc.TextMuted).SingleLine()
						})
					case isRead && args.Path != "":
						a.renderToolReadBlock(c, tc, args.Path, cleanOut)
					case isEdit && (args.OldString != "" || args.NewString != ""):
						a.renderToolEditDiff(c, tc, thisTool.ID, args.Path, args.OldString, args.NewString)
					case isEdit && args.Content != "" && args.Path != "":
						a.renderToolReadBlock(c, tc, args.Path, args.Content)
					case isEdit && len(args.Edits) > 0 && parsedPatch == "":
						for ei, ep := range args.Edits {
							a.renderToolEditDiff(c, tc, fmt.Sprintf("%s-%d", thisTool.ID, ei), args.Path, ep.OldString, ep.NewString)
						}
					case isSearch && cleanOut != "":
						ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
							ui.Icon(c, icons.TablerSearch).Width(12).Height(12).TextColor(tc.Accent)
							searchLbl := args.Query
							if searchLbl == "" {
								searchLbl = thisTool.Command
							}
							if args.Path != "" && args.Query != "" {
								searchLbl = fmt.Sprintf("%s  (%s)", args.Query, args.Path)
							}
							ui.Text(c, searchLbl).FontSize(11).Font("monospace").TextColor(tc.Text).Grow(1).SingleLine()
						})
						renderToolOutputBox(c, tc, cleanOut, failed)
					case parsedPatch != "":
						// Unified patch output (raw or inside JSON diff payload): parse into MyGo DiffView
						d := a.Editor.OpenPatchDiffKeyed("tool-patch-"+thisTool.ID, parsedPatch, a.ThemeMgr)
						if d != nil {
							d.FitContent = true
							d.Layout = mygoeditor.DiffUnified
							box := ui.Column(c).Radius(8).Background(tc.Canvas).BorderWidth(1).BorderColor(tc.Border.Alpha(0.5)).Clip()
							box.Children(func() {
								mygoeditor.DiffView(c, d).FillWidth()
							})
						} else if cleanOut != "" {
							renderToolOutputBox(c, tc, cleanOut, failed)
						}
					case isBash:
						cmdStr := args.Cmd
						if cmdStr == "" {
							cmdStr = thisTool.Command
						}
						ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
							ui.Text(c, "$").FontSize(11).FontWeight(700).TextColor(tc.Accent)
							ui.Text(c, cmdStr).FontSize(11).Font("monospace").TextColor(tc.Text).Grow(1).Selectable()
							if cleanOut != "" {
								copyBtn := ui.Row(c.Key("tbash-copy-"+thisTool.ID)).Size(20, 20).Radius(4).AlignItems(ui.Center).Justify(ui.Center).
									Cursor(ui.CursorPointer).Tooltip("Copy output")
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
			})
		}
	})
}

// renderAgentToolRow is the ZCode Agent/Task block: a face-tile identity,
// the agent role name, and the child's latest activity rolling in the
// summary line. The whole row is a button that opens the child timeline in
// the right drawer — matching ZCode's "full child timeline lives in the
// side pane" contract (no inline expand).
func (a *AppUI) renderAgentToolRow(c *ui.Context, tc theme.ThemeColors, sessionID string, m *agent.ChatMessage, thisTool *agent.ToolExecution, v toolVisual) {
	failed := thisTool.Status == "failed" || strings.Contains(thisTool.Output, "<tool_use_error>")
	running := thisTool.Status == "running"

	// Best-effort child match: same role while running, else the most
	// recent running child, else the latest finished one with that role.
	var matched *subagentSnapshot
	if a.Agent != nil {
		subs := a.snapshotSubagents(sessionID)
		role := strings.ToLower(v.SubRole)
		for i := len(subs) - 1; i >= 0; i-- {
			s := subs[i]
			if subagentRunning(s.state) && (role == "" || strings.Contains(strings.ToLower(s.roleFilter), role) || strings.Contains(strings.ToLower(s.name), role)) {
				matched = &subs[i]
				break
			}
		}
		if matched == nil {
			for i := len(subs) - 1; i >= 0; i-- {
				if subagentRunning(subs[i].state) {
					matched = &subs[i]
					break
				}
			}
		}
		if matched == nil {
			for i := len(subs) - 1; i >= 0; i-- {
				if role == "" || strings.Contains(strings.ToLower(subs[i].roleFilter), role) {
					matched = &subs[i]
					break
				}
			}
		}
	}

	faceStatus := "running"
	switch {
	case failed:
		faceStatus = "failed"
	case running && matched != nil && !subagentRunning(matched.state):
		faceStatus = "running"
	case running:
		faceStatus = "running"
	case matched != nil:
		faceStatus = subagentFaceState(matched.state)
	default:
		faceStatus = "done"
	}

	name := v.SubRole
	if matched != nil && strings.TrimSpace(matched.name) != "" {
		name = matched.name
	}

	// Live child activity, ZCode's getLatestAgentChildSummaryFromChildren.
	activity := ""
	if running && matched != nil {
		activity = subagentLiveSummary(matched)
	} else if v.SubPrompt != "" && v.Label == clampRunes(v.SubPrompt, 80) {
		activity = ""
	}
	if activity == "" {
		activity = clampRunes(v.SubPrompt, 60)
	}

	// Orca-inspired inline worker orchestration card:
	// "Started worker <name> ↗" while running, "Update from worker <name> ↗" when done.
	verb := "Update from worker"
	if running {
		verb = "Started worker"
	} else if failed {
		verb = "Worker failed"
	}

	row := ui.Column(c.Key("agent-tool-"+thisTool.ID)).Radius(10).Padding(9, 12).Gap(5).
		Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).FillWidth().Cursor(ui.CursorPointer)
	row.Children(func() {
		ui.Row(c).Gap(7).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(18, 18).Children(func() {
				agentFace(c, tc, name, faceStatus)
			})
			ui.Text(c, verb).FontSize(12).TextColor(tc.TextMuted).SingleLine()
			ui.Text(c, name).FontSize(12.5).FontWeight(600).TextColor(tc.Text).SingleLine()
			ui.Icon(c, icons.TablerExternalLink).Width(11).Height(11).TextColor(tc.IconMuted)
			ui.Box(c).Grow(1)
			if running {
				ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Size(6, 6).Radius(3).Background(tc.Accent)
					ui.Text(c, "Working").FontSize(11).FontWeight(500).TextColor(shimmerTextColor(tc))
				})
			} else if failed {
				ui.Text(c, "Failed").FontSize(11).FontWeight(600).TextColor(tc.Error)
			} else {
				ui.Text(c, "Done").FontSize(11).FontWeight(500).TextColor(tc.Success)
			}
			chevColor := tc.IconMuted.Alpha(0.7)
			if row.Hovered() {
				chevColor = tc.Text
			}
			ui.Icon(c, icons.TablerChevronRight).Width(12).Height(12).TextColor(chevColor)
		})
		if activity != "" {
			ui.Text(c, activity).FontSize(12).TextColor(tc.TextMuted).SingleLine()
		}
	})
	if row.Hovered() {
		row.Background(tc.SurfaceRaised)
	}
	if row.Clicked() {
		a.RightDrawerTab = RightTabSubagent
		a.ShowRightDrawer = true
		if matched != nil {
			a.SelectedSubagentID = matched.id
		}
		a.invalidateUI()
	}
}

// subagentFaceState maps an engine subagent state onto a face status.
func subagentFaceState(state string) string {
	if subagentRunning(state) {
		return "running"
	}
	switch strings.ToLower(state) {
	case "completed", "done", "success", "finished":
		return "done"
	case "error", "failed", "cancelled", "stopped":
		return "failed"
	default:
		return "idle"
	}
}

// subagentLiveSummary derives the child's rolling activity line from its
// snapshot: the newest tool call ("Reading src/main.go"), else the newest
// assistant text line, else "Working…".
func subagentLiveSummary(sub *subagentSnapshot) string {
	if sub == nil {
		return "Working…"
	}
	for i := len(sub.messages) - 1; i >= 0; i-- {
		msg := &sub.messages[i]
		if msg.role != "assistant" {
			continue
		}
		if msg.lastTool != "" {
			return agentActivityLabel(msg.lastTool, msg.lastToolDetail)
		}
		if line := lastNonEmptyLine(msg.text); line != "" {
			return clampRunes(line, 64)
		}
	}
	if len(sub.pendingTools) > 0 {
		return "Awaiting approval"
	}
	return "Working…"
}

// agentActivityLabel renders "<Verb> <object>" for the rolling summary.
func agentActivityLabel(toolName, detail string) string {
	nl := strings.ToLower(toolName)
	obj := clampRunes(detail, 56)
	switch {
	case strings.Contains(nl, "read"), strings.Contains(nl, "view"), strings.Contains(nl, "cat"):
		if obj == "" {
			return "Reading files"
		}
		return "Reading " + obj
	case strings.Contains(nl, "edit"), strings.Contains(nl, "write"), strings.Contains(nl, "patch"), strings.Contains(nl, "create"):
		if obj == "" {
			return "Editing files"
		}
		return "Editing " + obj
	case strings.Contains(nl, "bash"), strings.Contains(nl, "shell"), strings.Contains(nl, "exec"), strings.Contains(nl, "command"):
		if obj == "" {
			return "Running command"
		}
		return "Running " + obj
	case strings.Contains(nl, "search"), strings.Contains(nl, "grep"), strings.Contains(nl, "find"), strings.Contains(nl, "glob"), strings.Contains(nl, "list"):
		if obj == "" {
			return "Searching"
		}
		return "Searching " + obj
	default:
		if obj == "" {
			return "Running " + toolName
		}
		return "Running " + toolName + " · " + obj
	}
}

// groupChild is one entry of a consecutive-step group: a grouped tool call
// or an absorbed interleaved thinking block (nil tool).
type groupChild struct {
	blockIdx int
	tool     *agent.ToolExecution
}

// renderConsecutiveSummaryGroup collapses a run of consecutive
// explore/run/change tool calls into one ZCode statistics row — "Explored 3
// files · Ran 2 commands" — showing the active call's live summary with the
// shimmer while the group is still running, and expanding to an indented
// guide rail of nested child rows (absorbed thinking blocks render their
// collapsed headers inside the rail).
func (a *AppUI) renderConsecutiveSummaryGroup(c *ui.Context, tc theme.ThemeColors, sessionID string, m *agent.ChatMessage, children []groupChild, groupKey string) {
	nouns := groupNouns{}
	anyRunning := false
	var active *agent.ToolExecution
	var activeVisual toolVisual
	for _, ch := range children {
		if ch.tool == nil {
			continue
		}
		v := classifyTool(ch.tool)
		switch {
		case v.IsBash:
			nouns.command++
		case v.Family == tfChange:
			nouns.edit++
		case v.IsRead:
			nouns.file++
		default:
			nouns.search++
		}
		if ch.tool.Status == "running" {
			anyRunning = true
			active = ch.tool
			activeVisual = v
		}
	}

	expanded := a.ToolGroupExpanded[groupKey]
	kindWord := nouns.kind()
	stats := nouns.line()

	ui.Column(c.Key("tgroup-" + groupKey)).FillWidth().Children(func() {
		// ZCode ConsecutiveSummaryGroup header: "Explore · 1 search, 1 file"
		// — kind word bright, noun counts muted, chevron trailing.
		header := ui.Row(c).Height(30).Radius(6).Padding(0, 8).Gap(8).AlignItems(ui.Center)
		header.Children(func() {
			ui.Icon(c, groupRepresentativeIcon(kindWord)).Width(15).Height(15).TextColor(tc.IconMuted)
			if anyRunning && active != nil {
				// Rolling live summary of the in-flight call, ZCode-style.
				ui.Text(c, activeVisual.KindLabel).FontSize(13).FontWeight(500).TextColor(shimmerTextColor(tc)).SingleLine()
				ui.Text(c, clampRunes(activeVisual.Label, 72)).FontSize(13).TextColor(tc.Text).Grow(1).SingleLine()
			} else {
				ui.Text(c, kindWord).FontSize(13).FontWeight(500).TextColor(tc.Text).SingleLine()
				if stats != "" {
					ui.Text(c, "·  "+stats).FontSize(13).TextColor(tc.SecondaryLabel).Grow(1).SingleLine()
				} else {
					ui.Box(c).Grow(1)
				}
			}
			// Failure status lives on the failing child row, not the group —
			// matching ZCode's ConsecutiveSummaryGroup.
			chev := icons.TablerChevronRight
			if expanded {
				chev = icons.TablerChevronDown
			}
			chevColor := ui.RGBA(0, 0, 0, 0) // invisible until hover/expanded
			if expanded || header.Hovered() {
				chevColor = tc.IconMuted
			}
			ui.Icon(c, chev).Width(14).Height(14).TextColor(chevColor)
		})
		if header.Hovered() {
			header.Background(tc.Text.Alpha(0.04))
		}
		if header.Clicked() {
			if a.ToolGroupExpanded == nil {
				a.ToolGroupExpanded = make(map[string]bool)
			}
			a.ToolGroupExpanded[groupKey] = !expanded
		}

		if expanded {
			// Long runs scroll inside a capped rail, like ZCode's
			// max-h-[calc(16lh+30px)] container.
			drawRail := func() {
				rail := ui.Column(c).Padding(2, 0, 2, 16).Gap(2).
					BorderWidth(0, 0, 0, 1).BorderColor(tc.Border.Alpha(0.5))
				rail.Children(func() {
					for _, ch := range children {
						if ch.tool != nil {
							a.renderToolRowEx(c, tc, sessionID, m, ch.tool, true)
							continue
						}
						// Absorbed thinking: collapsed reasoning header inside
						// the rail; live shimmer only for the streaming tail.
						if ch.blockIdx < len(m.Blocks) && m.Blocks[ch.blockIdx] != nil {
							isLast := ch.blockIdx == len(m.Blocks)-1
							a.renderThinkingBlockSegment(c, tc, sessionID, m, ch.blockIdx,
								m.Blocks[ch.blockIdx].Text, m.IsThinking && !m.Done && isLast)
						}
					}
				})
			}
			if len(children) > 12 {
				ui.Scroll(c).MaxHeight(460).Children(drawRail)
			} else {
				drawRail()
			}
		}
	})
}

// displayToolPath shortens an absolute path to its workspace-relative form
// (or the bare filename outside the workspace) for ZCode-style file chips.
func (a *AppUI) displayToolPath(path string) string {
	if path == "" {
		return ""
	}
	if a.Workspace != nil && a.Workspace.RootDir != "" {
		if rel, err := filepath.Rel(a.Workspace.RootDir, path); err == nil && !strings.HasPrefix(rel, "..") && rel != "." {
			return rel
		}
	}
	return filepath.Base(path)
}

// readMetaOnlyOutput reports whether a read tool's output is just the
// harness's "4104 lines, 140821 bytes" meta line rather than file content —
// rendering that through the editor card is noise, ZCode hides it.
func readMetaOnlyOutput(out string) bool {
	s := strings.TrimSpace(out)
	if s == "" || strings.Contains(s, "\n") {
		return false
	}
	lower := strings.ToLower(s)
	if !strings.Contains(lower, "lines") || !strings.Contains(lower, "bytes") {
		return false
	}
	// Only digits, spaces, commas, and the two words — otherwise it's real
	// content that happens to mention both.
	for _, r := range lower {
		switch {
		case r >= '0' && r <= '9':
		case r == 'l' || r == 'i' || r == 'n' || r == 'e' || r == 's' || r == 'b' || r == 'y' || r == 't':
		case r == ' ' || r == ',':
		default:
			return false
		}
	}
	return true
}

// renderToolReadBlock draws forge-ade's ToolReadBlock: a file header with
// Open-in-Side-Viewer + Copy actions, then uses MyGo's internal read-only
// editor component for syntax highlighting, line numbers, and selections.
func fileExtBadgeLabel(path string) string {
	ext := strings.TrimPrefix(strings.ToUpper(filepath.Ext(path)), ".")
	switch ext {
	case "":
		return "FILE"
	case "MD", "MARKDOWN":
		return "M↓"
	default:
		if runes := []rune(ext); len(runes) > 4 {
			return string(runes[:4])
		}
		return ext
	}
}

func fileExtBadgeColor(path string, tc theme.ThemeColors) ui.Color {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go", ".rs", ".py":
		return tc.Success
	case ".md", ".ts", ".tsx", ".js", ".jsx":
		return tc.Info
	case ".json", ".yaml", ".yml", ".toml":
		return tc.Warning
	default:
		return tc.Accent
	}
}

// openTranscriptFileInSidePane opens a file referenced by an agent transcript
// row (tool file chips, change rows) in the right drawer's side file pane,
// resolving absolute paths to workspace-relative ones the pane stores.
// Paths outside the workspace — relative escapes (../..) or absolute files
// elsewhere on disk — are rejected: agent-controlled strings must not open
// arbitrary locations.
func (a *AppUI) openTranscriptFileInSidePane(path string) {
	a.openTranscriptFileInSidePaneOpts(path, false)
}

func (a *AppUI) openTranscriptFileInSidePaneDiff(path string) {
	a.openTranscriptFileInSidePaneOpts(path, true)
}

func (a *AppUI) openTranscriptFileInSidePaneOpts(path string, diff bool) {
	if path == "" || a.Workspace == nil {
		return
	}
	root := a.Workspace.RootDir
	if root == "" {
		return
	}
	clean := filepath.Clean(path)
	if strings.HasPrefix(clean, "ssh://") {
		if diff {
			a.SideFileDiffMode[clean] = true
		}
		a.openSidePreviewFile(nil, clean)
		return
	}
	if filepath.IsAbs(clean) {
		if root == "" || strings.HasPrefix(root, "ssh://") {
			return
		}
		r, err := filepath.Rel(root, clean)
		if err != nil || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
			return // absolute path outside the workspace
		}
		clean = r
	} else if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return // relative escape
	}
	if diff {
		a.SideFileDiffMode[clean] = true
		if a.Git != nil {
			a.Git.RefreshAsync()
		}
	}
	a.openSidePreviewFile(nil, clean)
}

func (a *AppUI) renderToolReadBlock(c *ui.Context, tc theme.ThemeColors, path, output string) {
	fileName := filepath.Base(path)
	extRaw := strings.TrimPrefix(strings.ToUpper(filepath.Ext(path)), ".")
	subLabel := "Document"
	if extRaw != "" {
		subLabel = "Document · " + extRaw
	}
	_, badgeColor := icons.FileIcon(path, tc.IconMuted)

	ui.Column(c).Radius(10).Background(tc.Surface).
		BorderWidth(1).BorderColor(tc.Border).Clip().Children(func() {
		// ZCode Document Header Card: 32x32 language icon badge + bold title + subtitle + [Open ▾]
		ui.Row(c).Padding(10, 12).Gap(10).AlignItems(ui.Center).
			Background(tc.SurfaceRaised.Alpha(0.35)).
			BorderWidth(0, 0, 1, 0).BorderColor(tc.Border.Alpha(0.5)).Children(func() {
			ui.Row(c).Size(32, 32).Radius(8).AlignItems(ui.Center).Justify(ui.Center).
				Background(badgeColor.Alpha(0.14)).BorderWidth(1).BorderColor(badgeColor.Alpha(0.28)).Children(func() {
				icons.RenderFileIcon(c, path, 18, tc.IconMuted)
			})
			ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
				ui.Text(c, fileName).FontSize(13).FontWeight(600).TextColor(tc.Text).SingleLine()
				ui.Text(c, subLabel).FontSize(11).TextColor(tc.TextMuted).SingleLine()
			})
			openPill := ui.Row(c.Key("read-open-"+path)).Height(26).Radius(6).Padding(0, 10).Gap(5).
				AlignItems(ui.Center).Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border).Cursor(ui.CursorPointer)
			openPill.Children(func() {
				ui.Text(c, "Open").FontSize(12).FontWeight(500).TextColor(tc.Text)
				ui.Icon(c, icons.TablerChevronDown).Width(11).Height(11).TextColor(tc.IconMuted)
			})
			if openPill.Hovered() {
				openPill.Background(tc.Text.Alpha(0.06))
			}
			openPill.Menu(func(m *ui.Menu) {
				if m.Item("Open in Side Viewer").Chosen() && a.Workspace != nil {
					rel, err := filepath.Rel(a.Workspace.RootDir, path)
					if err != nil || strings.HasPrefix(rel, "..") {
						rel = path
					}
					a.openSideFile(rel)
				}
				if m.Item("Open in Editor").Chosen() {
					a.openFileInEditor(path)
				}
				m.Separator()
				if m.Item("Copy Contents").Chosen() {
					mygo.Clipboard.WriteText(output)
					c.AddToast(ui.Toast{Title: "Copied", Description: fileName, Type: "success"})
				}
			})
		})

		// Internal MyGo Editor component (Read-only, syntax highlighted)
		if a.Editor != nil && output != "" {
			edState := a.Editor.GetOrCreateReadOnlyEditor("read-"+path, path, output, a.ThemeMgr)
			if edState != nil {
				ui.Box(c).Padding(4, 6).Children(func() {
					mygoeditor.View(c, edState).FillWidth()
				})
			}
		}
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
	if runes := []rune(outText); len(runes) > 1200 {
		outText = string(runes[:1200]) + "\n… [truncated]"
	}
	ui.Box(c).Padding(8).Radius(6).Background(boxBg).
		BorderWidth(1).BorderColor(boxBorder).Children(func() {
		ui.Text(c, outText).FontSize(11).Font("monospace").TextColor(textColor).Selectable()
	})
}

func (a *AppUI) renderTurnChangesCard(c *ui.Context, tc theme.ThemeColors, sessionID string, isCheckpointMsg bool, m *agent.ChatMessage) {
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

	// ZCode behavior: the card follows the live turn expanded, then folds to
	// just its header when the turn ends — an explicit user toggle wins.
	collapsed := m.Done
	if v, ok := a.ChangesCardCollapsed[m.ID]; ok {
		collapsed = v
	}

	card := ui.Column(c).Radius(10).Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).Clip()
	card.Children(func() {
		// ZCode Changes Header Row: ⌄ N files changed  +A -D        ↩ Undo
		hdr := ui.Row(c.Key("tchg-hdr-"+fmt.Sprint(m.ID))).Padding(8, 12).Gap(8).AlignItems(ui.Center)
		if !collapsed {
			hdr.BorderWidth(0, 0, 1, 0).BorderColor(tc.Border.Alpha(0.5))
		}
		hdr.Children(func() {
			chevBtn := ui.Row(c.Key("tchg-tog-" + fmt.Sprint(m.ID))).Gap(6).AlignItems(ui.Center).Cursor(ui.CursorPointer)
			chevBtn.Children(func() {
				chevIcon := icons.TablerChevronDown
				if collapsed {
					chevIcon = icons.TablerChevronRight
				}
				ui.Icon(c, chevIcon).Width(13).Height(13).TextColor(tc.IconMuted)
				ui.Text(c, fmt.Sprintf("%d files changed", len(m.Changes))).FontSize(12).FontWeight(600)
			})
			if chevBtn.Clicked() {
				if a.ChangesCardCollapsed == nil {
					a.ChangesCardCollapsed = map[int]bool{}
				}
				a.ChangesCardCollapsed[m.ID] = !collapsed
			}

			ui.Text(c, fmt.Sprintf("+%d", totalAdd)).FontSize(11).FontWeight(600).TextColor(tc.DiffAdditionForeground)
			ui.Text(c, fmt.Sprintf("-%d", totalDel)).FontSize(11).FontWeight(600).TextColor(tc.DiffDeletionForeground)
			ui.Box(c).Grow(1)

			if m.Done && isCheckpointMsg {
				undoBtn := ui.Row(c.Key("tchg-undo-"+fmt.Sprint(m.ID))).Height(22).Radius(6).Padding(0, 8).Gap(4).
					AlignItems(ui.Center).Cursor(ui.CursorPointer)
				undoBtn.Children(func() {
					ui.Icon(c, icons.TablerRotate).Width(12).Height(12).TextColor(tc.TextMuted)
					ui.Text(c, "Undo").FontSize(11).FontWeight(500).TextColor(tc.TextMuted)
				})
				if undoBtn.Hovered() {
					undoBtn.Background(tc.Text.Alpha(0.06))
				}
				if undoBtn.Clicked() {
					changes := append([]*agent.FileChange(nil), m.Changes...)
					session := sessionID
					msgID := m.ID
					go func() {
						// Only toast success once the undo actually happened:
						// checkpoint restore first, per-file git discard as the
						// fallback; with neither available, keep the list and
						// report the failure (queueToast — goroutine context).
						rerr := as.RestoreLastTurnCheckpoint(session)
						if rerr != nil && a.Git != nil {
							for _, ch := range changes {
								if ch != nil && ch.Path != "" {
									a.Git.Discard(ch.Path)
								}
							}
						}
						if a.Git != nil {
							a.Git.Refresh()
						}
						if rerr != nil && a.Git == nil {
							a.queueToast(ui.Toast{Title: "Undo Failed", Description: rerr.Error(), Type: "error"})
							if a.Window != nil {
								a.Window.Invalidate()
							}
							return
						}
						as.ClearMessageChanges(session, msgID)
						a.queueToast(ui.Toast{Title: "Turn Changes Undone", Type: "info"})
						if a.Window != nil {
							a.Window.Invalidate()
						}
					}()
				}
			}
		})

		if !collapsed {
			ui.Column(c).Padding(6, 8).Gap(4).Children(func() {
				for _, ch := range m.Changes {
					if ch == nil {
						continue
					}
					chPath := ch.Path
					base := filepath.Base(chPath)
					dir := filepath.Dir(chPath)
					if dir == "." {
						dir = ""
					}
					chRow := ui.Row(c.Key("tchg-"+chPath)).Radius(6).Padding(5, 8).Gap(8).AlignItems(ui.Center)
					chRow.Children(func() {
						icons.RenderFileIcon(c, chPath, 15, tc.IconMuted)
						ui.Text(c, base).FontSize(12).FontWeight(500).TextColor(tc.Text).SingleLine()
						if dir != "" {
							ui.Text(c, dir+"/").FontSize(11).TextColor(tc.TextMuted).SingleLine()
						}
						if ch.Additions > 0 {
							ui.Text(c, fmt.Sprintf("+%d", ch.Additions)).FontSize(11).TextColor(tc.DiffAdditionForeground)
						}
						if ch.Deletions > 0 {
							ui.Text(c, fmt.Sprintf("-%d", ch.Deletions)).FontSize(11).TextColor(tc.DiffDeletionForeground)
						}
						ui.Box(c).Grow(1)

						// [ Review ] pill
						reviewBtn := ui.Row(c.Key("tchg-rev-"+chPath)).Height(24).Radius(6).Padding(0, 10).
							AlignItems(ui.Center).Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border).Cursor(ui.CursorPointer)
						reviewBtn.Children(func() {
							ui.Text(c, "Review").FontSize(11).FontWeight(500).TextColor(tc.Text)
						})
						if reviewBtn.Hovered() {
							reviewBtn.Background(tc.Text.Alpha(0.06))
						}
						if reviewBtn.Clicked() {
							a.SelectedDiffPath = chPath
							a.ShowRightDrawer = true
							a.RightDrawerTab = RightTabChanges
						}

						// [ Open ▾ ] pill
						openBtn := ui.Row(c.Key("tchg-open-"+chPath)).Height(24).Radius(6).Padding(0, 8).Gap(4).
							AlignItems(ui.Center).Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border).Cursor(ui.CursorPointer)
						openBtn.Children(func() {
							ui.Text(c, "Open").FontSize(11).FontWeight(500).TextColor(tc.Text)
							ui.Icon(c, icons.TablerChevronDown).Width(10).Height(10).TextColor(tc.IconMuted)
						})
						if openBtn.Hovered() {
							openBtn.Background(tc.Text.Alpha(0.06))
						}
						openBtn.Menu(func(menu *ui.Menu) {
							if menu.Item("Open in Editor").Chosen() {
								a.openFileInEditor(chPath)
							}
							if menu.Item("Open in Side Viewer").Chosen() {
								a.openSidePreviewFile(c, chPath)
							}
						})
					})
					if chRow.Hovered() {
						chRow.Background(tc.Text.Alpha(0.04))
					}
				}
			})
		}
	})
}

// switchToInternalHarness activates the RakitKode internal harness and optional persona.
func (a *AppUI) switchToInternalHarness(personaID string) {
	as := a.Agent
	if as == nil {
		return
	}
	as.ActiveAgentID = "agent-internal"
	as.ActiveAgentType = "internal"
	a.SelectedHarnessID = "agent-internal"
	a.SelectedAgentID = "agent-internal"
	if sess := as.ActiveSession(); sess != nil {
		sess.AgentID = "agent-internal"
		sess.AgentType = "internal"
		sess.PersonaID = personaID
	}
	if personaID != "" {
		as.SetSessionPersona(as.ActiveSessionID, personaID)
	}
	a.invalidateUI()
}

// switchToACPHarness activates the given ACP external agent harness.
func (a *AppUI) switchToACPHarness(harnessID string) {
	as := a.Agent
	if as == nil {
		return
	}
	as.ActiveAgentID = harnessID
	as.ActiveAgentType = "acp"
	a.SelectedHarnessID = harnessID
	a.SelectedAgentID = harnessID
	if sess := as.ActiveSession(); sess != nil {
		sess.AgentID = harnessID
		sess.AgentType = "acp"
	}
	if a.ACPMgr != nil {
		as.ActiveModel = as.ResolveModelForAgent(harnessID, as.ActiveModel)
		a.SelectedAgentModel = as.ActiveModel
	}
	a.invalidateUI()
}

// respondApproval routes an approval decision to the right backend: internal
// sessions go through the RakitKode agent engine (RespondAgentApproval
// parity), ACP harnesses through the ACP manager. The error is propagated so
// the caller can keep the approval unresolved and let the user retry.
func (a *AppUI) respondApproval(sessionID, requestID string, approve, autoApproveAll bool) error {
	if a.Agent != nil && a.Agent.IsInternalSession(sessionID) {
		return a.Agent.RespondInternalApproval(sessionID, approve, autoApproveAll)
	}
	if a.ACPMgr != nil {
		return a.ACPMgr.RespondPermission(sessionID, requestID, approve)
	}
	return fmt.Errorf("no agent harness available to approve %s", requestID)
}

// resolvePendingApproval forwards the decision to the active agent harness
// first; only on success is the approval marked resolved and popped from the
// session's pending-approval queue. On error the bar stays up and the error
// is toasted so the user can retry.
func (a *AppUI) resolvePendingApproval(msg *agent.ChatMessage, approve, autoApproveAll bool) {
	if msg == nil || msg.PendingApproval == nil || msg.PendingApproval.Resolved {
		return
	}
	appr := *msg.PendingApproval // values from the frame snapshot
	if err := a.respondApproval(appr.SessionID, appr.ToolCallID, approve, autoApproveAll); err != nil {
		a.queueToast(ui.Toast{Title: "Approval Failed", Description: err.Error(), Type: "error"})
		return
	}
	if approve && autoApproveAll && a.Agent != nil {
		a.Agent.ExecutionMode = agent.ModeBypass
	}
	a.Agent.ResolveSessionApproval(appr.SessionID, appr.ToolCallID)
}

func (a *AppUI) renderApprovalBar(c *ui.Context, tc theme.ThemeColors, m *agent.ChatMessage) {
	appr := m.PendingApproval
	if appr == nil {
		return
	}

	// Orca-inspired "Needs you" inline decision card
	card := ui.Column(c).Radius(12).Padding(12, 14).Gap(10).
		Background(tc.Surface).BorderWidth(1).BorderColor(tc.Warning.Alpha(0.45))
	card.Children(func() {
		ui.Row(c).Gap(7).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(7, 7).Radius(3.5).Background(tc.Warning).Shrink(0)
			ui.Text(c, "Needs you").FontSize(11.5).FontWeight(600).TextColor(tc.Warning)
			ui.Text(c, "·").FontSize(11).TextColor(tc.TextMuted)
			ui.Text(c, "Action requires confirmation").FontSize(12).FontWeight(500).TextColor(tc.Text).Grow(1).SingleLine()
		})
		if strings.TrimSpace(appr.Summary) != "" {
			ui.Text(c, appr.Summary).FontSize(12).TextColor(tc.TextMuted)
		}
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			allow := chipRow(c, tc)
			allow.Background(tc.MessageAction).BorderWidth(0).Radius(7).Height(26).Padding(0, 10)
			allow.Children(func() {
				ui.Text(c, "Approve").FontSize(11.5).FontWeight(600).TextColor(tc.MessageActionForeground)
			})
			if allow.Clicked() {
				a.resolvePendingApproval(m, true, false)
			}
			always := composerControlChip(c, tc, icons.TablerShieldCheck, "Always Allow", "", false)
			if always.Clicked() {
				a.resolvePendingApproval(m, true, true)
			}
			deny := composerControlChip(c, tc, icons.TablerX, "Deny", "", true)
			if deny.Clicked() {
				a.resolvePendingApproval(m, false, false)
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

	// Render from a locked copy: the queue mutates from turn goroutines and
	// submit paths, and removals match by identity, not a captured index that
	// can shift.
	queue := as.SteerQueueSnapshot()
	streaming := as.StreamingSnapshot()

	// ZCode queued-prompt tray: compact rows — grip, text, then per-row
	// Steer (first row only, dashed into the running turn), edit and delete.
	tray := ui.Column(c).Padding(6).Radius(12).Background(tc.SurfaceRaised).
		BorderWidth(1).BorderColor(tc.Border.Alpha(0.8)).Gap(2)
	tray.Children(func() {
		for idx, item := range queue {
			thisItem := item
			// The snapshot index only keys the row for this frame; removals
			// match by identity, never by a captured index.
			row := ui.Row(c.Key(fmt.Sprintf("steer-%d", idx))).MinHeight(34).Radius(8).
				Padding(4, 8).Gap(8).AlignItems(ui.Center).
				Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border.Alpha(0.6))
			row.Children(func() {
				ui.Icon(c, icons.TablerGripVertical).Width(13).Height(13).TextColor(tc.IconMuted)
				ui.Text(c, thisItem).FontSize(12).Grow(1).SingleLine()
				if idx == 0 {
					steerBtn := ui.Row(c.Key("steer-now")).Height(26).Radius(7).Padding(0, 10).Gap(4).
						AlignItems(ui.Center).Background(tc.SurfaceOverlay).BorderWidth(1).BorderColor(tc.Border)
					steerBtn.Children(func() {
						ui.Icon(c, icons.TablerArrowUp).Width(12).Height(12).TextColor(tc.Text)
						ui.Text(c, "Steer").FontSize(12).FontWeight(600).TextColor(tc.Text)
					})
					if steerBtn.Clicked() && streaming {
						text := thisItem
						sid := as.SnapshotActiveSessionID()
						go func() {
							// Keep the queued item on failure so the text is
							// never silently lost; remove by identity.
							if err := as.SteerInternal(sid, text); err != nil {
								if errors.Is(err, agent.ErrSteerQueuedNextTurn) {
									a.queueToast(ui.Toast{Title: "Queued for Next Turn", Description: "This agent cannot accept mid-turn prompts — it will run when the current turn finishes", Type: "info"})
								} else {
									a.queueToast(ui.Toast{Title: "Steer Failed", Description: err.Error(), Type: "error"})
								}
								if a.Window != nil {
									a.Window.Invalidate()
								}
								return
							}
							as.RemoveSteerItem(text)
							as.SaveSteerQueues()
							if as.Invalidate != nil {
								as.Invalidate()
							}
						}()
					}
				}
				edit := ui.Row(c.Key("steer-edit")).Size(24, 24).Radius(6).AlignItems(ui.Center).Justify(ui.Center)
				edit.Children(func() {
					ui.Icon(c, icons.TablerPencil).Width(13).Height(13).TextColor(tc.TextMuted)
				})
				if edit.Clicked() {
					// TakeSteerItem locks internally (never wrap it in
					// as.Lock — sync.Mutex is not reentrant). InputText is
					// composer/UI-thread state.
					as.InputText = as.TakeSteerItem(thisItem)
					as.SaveSteerQueues()
				}
				del := ui.Row(c.Key("steer-del")).Size(24, 24).Radius(6).AlignItems(ui.Center).Justify(ui.Center)
				del.Children(func() {
					ui.Icon(c, icons.TablerTrash).Width(13).Height(13).TextColor(tc.TextMuted)
				})
				if del.Hovered() {
					del.Background(tc.Error.Alpha(0.12))
				}
				if del.Clicked() {
					as.RemoveSteerItem(thisItem)
					as.SaveSteerQueues()
				}
			})
			if row.Hovered() {
				row.Background(tc.SurfaceOverlay)
			}
		}
	})
}

func (a *AppUI) renderComposer(c *ui.Context, tc theme.ThemeColors, sess *agent.AgentSession) {
	as := a.Agent
	if as == nil {
		return
	}
	inset, rightInset := a.chatLaneInset(c)
	rightPad := inset
	if a.showFloatingInspector(c) {
		rightPad = maxF(inset, 324)
	}

	// Composer area: animated wait/progress strip above the composer when streaming.
	ui.Column(c).Padding(0, rightPad, 12, rightInset).Gap(6).Children(func() {
		a.renderStreamingLoaderStrip(c, tc, sess)
		a.renderComposerCard(c, tc, false)
	})
}

// renderStreamingLoaderStrip renders a compact Claude Code / ZCode-style animated
// progress loader above the composer with rotating playful status verbs, live
// elapsed timer, step counter, and current active tool hint.
func (a *AppUI) renderStreamingLoaderStrip(c *ui.Context, tc theme.ThemeColors, sess *agent.AgentSession) {
	if a.Agent == nil || !a.Agent.StreamingSnapshot() {
		return
	}
	sessID := a.Agent.ActiveSessionID
	glyph, phrase := streamingProgressPhrase(sessID, time.Now())

	var elapsedText, detailText string
	var stepCount int
	if sess != nil && len(sess.Messages) > 0 {
		for i := len(sess.Messages) - 1; i >= 0; i-- {
			m := sess.Messages[i]
			if m == nil || m.Role != "assistant" {
				continue
			}
			if !m.TurnStartedAt.IsZero() {
				elapsedText = formatWorkDuration(time.Since(m.TurnStartedAt))
			}
			stepCount = len(m.Tools)
			for j := len(m.Tools) - 1; j >= 0; j-- {
				t := m.Tools[j]
				if t != nil && t.Status == "running" {
					v := classifyTool(t)
					if v.Badge != "" && v.Label != "" {
						detailText = v.Badge + " · " + clampRunes(v.Label, 36)
					} else if v.Label != "" {
						detailText = v.KindLabel + " " + clampRunes(v.Label, 40)
					} else {
						detailText = v.KindLabel
					}
					break
				}
			}
			break
		}
	}

	ui.Row(c.Key("composer-stream-loader")).Height(26).Padding(0, 12).Gap(8).AlignItems(ui.Center).
		Radius(8).Background(tc.Surface.Alpha(0.75)).BorderWidth(1).BorderColor(tc.Border.Alpha(0.55)).Children(func() {
		ui.Text(c, glyph).FontSize(13).Font("monospace").FontWeight(700).TextColor(tc.Accent)
		ui.Text(c, phrase).FontSize(12).FontWeight(600).TextColor(shimmerTextColor(tc))
		if detailText != "" {
			ui.Text(c, "·").FontSize(11).TextColor(tc.TextMuted)
			ui.Text(c, detailText).FontSize(11).Font("monospace").TextColor(tc.TextMuted).Grow(1).SingleLine()
		} else {
			ui.Box(c).Grow(1)
		}
		if stepCount > 0 {
			lbl := fmt.Sprintf("%d step", stepCount)
			if stepCount > 1 {
				lbl = fmt.Sprintf("%d steps", stepCount)
			}
			ui.Text(c, lbl).FontSize(10.5).FontWeight(500).TextColor(tc.TextMuted)
		}
		if elapsedText != "" {
			ui.Text(c, elapsedText).FontSize(10.5).Font("monospace").FontWeight(600).TextColor(tc.TextMuted)
		}
	})
}

func (a *AppUI) renderComposerCard(c *ui.Context, tc theme.ThemeColors, isHome bool) {
	as := a.Agent
	if as == nil {
		return
	}

	card := ui.Column(c).Radius(12).Background(tc.Surface).
		BorderWidth(1).BorderColor(tc.Border).Clip()
	if isHome {
		// Landing card: the tray above owns the top corners; this card owns
		// the bottom ones — one continuous silhouette, width of the unit.
		card.Radius(0, 0, 12, 12).FillWidth()
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
						icons.RenderFileIcon(c, filePath, 12, tc.Accent)
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
			if as.StreamingSnapshot() {
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
				// Alt+S: stash non-empty prompt; otherwise open Terminal Snippets modal.
				if k.Key == ui.KeyS && k.Mods&ui.Alt != 0 {
					if strings.TrimSpace(as.InputText) != "" {
						as.StashPrompt()
						c.AddToast(ui.Toast{Title: "Prompt Stashed", Description: "Alt+S restores via the stash chip", Type: "info"})
					} else {
						sessID := ""
						if a.AgentTerminal != nil {
							sessID = a.AgentTerminal.ActiveSessionID
						}
						a.OpenTerminalSnippetsModal(sessID, "", "")
					}
					return true
				}
				// Prompt history: up/down while the composer is empty on a single line
				// or already browsing history (promptHistoryStore).
				if k.Key == ui.KeyUp || k.Key == ui.KeyDown {
					if !strings.Contains(as.InputText, "\n") && (strings.TrimSpace(as.InputText) == "" || as.PromptHistoryIdx != -1) {
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
					trimmed := strings.TrimSpace(as.InputText)
					if a.tryHandleSlashCommand(trimmed) {
						k.SetText("")
						return true
					}
					as.ListState.FollowEnd = true
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
		winW, _ := c.Size()
		isSlim := false
		if a.Mode == ModeStudio {
			isSlim = a.StudioLeftChatWidth == 0 || a.StudioLeftChatWidth < 520
		} else if winW > 0 && winW < 880 {
			isSlim = true
		}

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
				m.Separator()
				if m.Item("Skills Context (@skills)").Chosen() {
					if as.InputText != "" && !strings.HasSuffix(as.InputText, " ") {
						as.InputText += " "
					}
					as.InputText += "@skills "
				}
				if m.Item("MCP Tools Context (@mcp)").Chosen() {
					if as.InputText != "" && !strings.HasSuffix(as.InputText, " ") {
						as.InputText += " "
					}
					as.InputText += "@mcp "
				}
				if m.Item("Git Status & Diff (@git)").Chosen() {
					if as.InputText != "" && !strings.HasSuffix(as.InputText, " ") {
						as.InputText += " "
					}
					as.InputText += "@git "
				}
				if m.Item("Terminal Output (@terminal)").Chosen() {
					if as.InputText != "" && !strings.HasSuffix(as.InputText, " ") {
						as.InputText += " "
					}
					as.InputText += "@terminal "
				}
				m.Separator()
				if m.Item("Stash Current Prompt").Shortcut(ui.Alt, ui.KeyS).Chosen() {
					as.StashPrompt()
				}
			})

			// Permission Mode selector chip
			var modeIcon *ui.SVG
			modeLabel := ""
			slimLabel := ""
			modeColor := tc.Text
			switch as.ExecutionMode {
			case agent.ModeAsk:
				modeIcon, modeLabel, slimLabel = icons.TablerHand, "Ask", "Ask"
			case agent.ModeCode:
				modeIcon, modeLabel, slimLabel = icons.TablerPencil, "Edits", "Edit"
			case agent.ModePlan:
				modeIcon, modeLabel, slimLabel = icons.TablerShieldCheck, "Auto", "Auto"
			default:
				modeIcon, modeLabel, slimLabel = icons.TablerShieldAlert, "Full access", "Full"
				modeColor = tc.FullAccess
			}
			modeBtn := chipRow(c, tc)
			if isSlim {
				modeBtn.Padding(0, 6).Tooltip(fmt.Sprintf("Permission: %s", modeLabel))
			}
			modeBtn.Children(func() {
				ui.Row(c).Gap(3).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, modeIcon).Width(12).Height(12).TextColor(modeColor)
					lbl := modeLabel
					if isSlim {
						lbl = slimLabel
					}
					ui.Text(c, lbl).FontSize(11).FontWeight(500).TextColor(modeColor).SingleLine()
					ui.Icon(c, icons.TablerChevronDown).Width(9).Height(9).TextColor(modeColor.Alpha(0.7))
				})
			})
			if modeBtn.Clicked() {
				as.CycleExecutionMode()
			}

			// Spawned sub-agent / background shell indicator pill.
			// Default is hidden (0); only rendered when spawned subagents or extra background shells > 0.
			subCount := 0
			if as != nil && as.ActiveSession() != nil {
				subCount = len(as.Subagents(as.ActiveSession().ID))
			}
			spawnedShells := 0
			if a.AgentTerminal != nil {
				tabCount := a.AgentTerminal.TabCount()
				if tabCount > 1 {
					spawnedShells = tabCount - 1
				}
			}
			spawnedCount := subCount + spawnedShells

			if spawnedCount > 0 {
				termChip := ui.Row(c.Key("composer-spawned-pill")).Height(28).Radius(8).Padding(0, 8).Gap(5).AlignItems(ui.Center).Cursor(ui.CursorPointer)
				termColor := tc.Accent
				pillIcon := icons.TablerBot
				if subCount == 0 {
					pillIcon = icons.TablerTerminal
				}
				termChip.Background(tc.Accent.Alpha(0.12)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.35))
				if termChip.Hovered() {
					termChip.Background(tc.Accent.Alpha(0.18)).BorderColor(tc.Accent.Alpha(0.5))
				}
				termChip.Children(func() {
					ui.Icon(c, pillIcon).Width(13).Height(13).TextColor(termColor)
					ui.Text(c, fmt.Sprintf("%d", spawnedCount)).FontSize(12).FontWeight(600).TextColor(termColor)
				})
				termChip.ContextMenu(func(m *ui.Menu) {
					if subCount > 0 {
						if m.Item("View Subagents in Drawer").Chosen() {
							a.RightDrawerTab = RightTabSubagent
							a.ShowRightDrawer = true
						}
						m.Separator()
					}
					if m.Item("Terminal Snippets...").Shortcut(ui.Alt, ui.KeyS).Chosen() {
						sessID := ""
						if a.AgentTerminal != nil {
							sessID = a.AgentTerminal.ActiveSessionID
						}
						a.OpenTerminalSnippetsModal(sessID, "", "")
					}
					m.Separator()
					if m.Item(menuChecked(a.ShowAgentBottomTerminal, "Toggle Terminal Dock")).Shortcut(ui.Cmd, ui.KeyBackquote).Chosen() {
						a.ShowAgentBottomTerminal = !a.ShowAgentBottomTerminal
					}
					if a.AgentTerminal != nil {
						if m.Item("New Terminal Tab").Chosen() {
							_ = a.AgentTerminal.NewTab("")
							a.ShowAgentBottomTerminal = true
						}
						if m.Item("Clear Terminal").Chosen() {
							a.AgentTerminal.SendInput("\x0c")
						}
						if m.Item("Restart Shell").Chosen() {
							_ = a.AgentTerminal.Restart()
						}
					}
				})
				if termChip.Clicked() {
					if subCount > 0 {
						a.RightDrawerTab = RightTabSubagent
						a.ShowRightDrawer = !a.ShowRightDrawer
					} else {
						a.ShowAgentBottomTerminal = !a.ShowAgentBottomTerminal
					}
				}
			}

			// Active agent-spawned background shell indicator pill + popover trigger
			bgSessionID := ""
			if as != nil {
				bgSessionID = as.ActiveSessionID
			}
			a.renderBgShellPill(c, tc, bgSessionID)

			// ACP / Lead Persona Selector (Responsive: slim mode with model & acp selector)
			isInternalHarness := as.ActiveAgentID == "" || as.ActiveAgentID == "agent-internal" || as.ActiveAgentType == "internal"
			activeSess := as.ActiveSession()
			var personaID, personaName, shortPersona string
			var acpIcon *ui.SVG
			var acpColor, acpIconColor ui.Color

			if isInternalHarness {
				if a.Engine != nil && a.Engine.AgentMgr != nil {
					personaID, personaName = as.EffectivePersona(activeSess)
				}
				if personaName == "" {
					personaName = "Orchestrator"
				}
				shortPersona = strings.TrimSpace(strings.TrimSuffix(personaName, "Agent"))
				if shortPersona == "" {
					shortPersona = personaName
				}
				acpIcon = icons.TablerBot
				acpColor = tc.Text
				acpIconColor = tc.IconMuted
				if personaID == "orchestrator" || personaID == "" {
					acpColor = tc.Accent
					acpIconColor = tc.Accent
				}
			} else {
				// ACP External Harness
				acpIcon = icons.TablerPlug
				acpColor = tc.Accent
				acpIconColor = tc.Accent
				personaName = as.ActiveAgentID
				if a.ACPMgr != nil {
					for _, ag := range a.ACPMgr.Agents {
						if ag.ID == as.ActiveAgentID {
							personaName = ag.Name
							break
						}
					}
				}
				shortPersona = shortHarnessLabel(personaName)
			}

			displayPersona := shortHarnessLabel(personaName)
			if isSlim {
				if isInternalHarness {
					if personaID == "orchestrator" || personaID == "" {
						displayPersona = "Orch"
					} else if len(shortPersona) > 8 {
						displayPersona = string([]rune(shortPersona)[:7]) + "…"
					} else {
						displayPersona = shortPersona
					}
				} else {
					displayPersona = shortPersona
				}
			}

			personaBtn := ui.Row(c.Key("composer-harness")).Height(26).Radius(6).Padding(0, 7).Gap(4).
				AlignItems(ui.Center).Cursor(ui.CursorPointer)
			personaBtn.Tooltip(fmt.Sprintf("Agent / Harness: %s", personaName))
			personaBtn.Children(func() {
				ui.Icon(c, acpIcon).Width(12).Height(12).TextColor(acpIconColor)
				ui.Text(c, displayPersona).FontSize(11.5).FontWeight(500).TextColor(acpColor).SingleLine()
				ui.Icon(c, icons.TablerChevronDown).Width(9).Height(9).TextColor(tc.IconMuted)
			})
			if personaBtn.Hovered() {
				personaBtn.Background(tc.Text.Alpha(0.06))
			}
			personaBtn.Menu(func(m *ui.Menu) {
				m.Item("── Internal (Forge Engine) ──").Disabled(true)
				if m.Item(menuChecked(isInternalHarness && (activeSess == nil || activeSess.PersonaID == ""), "Auto (Orchestrator)")).Chosen() {
					a.switchToInternalHarness("")
				}
				if a.Engine != nil && a.Engine.AgentMgr != nil {
					for _, def := range a.Engine.AgentMgr.ListAgentDefinitions() {
						d := def
						isThisPersona := isInternalHarness && activeSess != nil && activeSess.PersonaID == d.ID
						it := m.Item(menuChecked(isThisPersona, d.Name))
						if d.Role != "" {
							it = it.Subtitle(string(d.Role))
						}
						if it.Chosen() {
							a.switchToInternalHarness(d.ID)
						}
					}
				}
				if a.ACPMgr != nil && len(a.ACPMgr.Agents) > 0 {
					m.Separator()
					m.Item("── ACP Agents (External) ──").Disabled(true)
					for _, ag := range a.ACPMgr.Agents {
						if ag.ID == "agent-internal" {
							continue
						}
						agentDef := ag
						isThisACP := !isInternalHarness && as.ActiveAgentID == agentDef.ID
						it := m.Item(menuChecked(isThisACP, agentDef.Name))
						if agentDef.Type != "" {
							it = it.Subtitle(agentDef.Type)
						}
						if it.Chosen() {
							a.switchToACPHarness(agentDef.ID)
						}
					}
				}
				m.Separator()
				if m.Item("Orchestration & Personas Settings...").Chosen() {
					a.openSettings(settings.SettingsTabSubagents)
				}
			})

			// Prompt stash chip: appears when entries exist; popover lists
			// stashed prompts (click restores into the composer).
			if len(as.StashedPrompts) > 0 {
				stashChip := ui.Row(c.Key("stash-chip")).Height(28).Radius(8).Padding(0, 9).Gap(5).AlignItems(ui.Center).
					Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
				stashChip.Children(func() {
					ui.Icon(c, icons.TablerBookmark).Width(12).Height(12).TextColor(tc.Accent)
					ui.Text(c, fmt.Sprintf("Stash %d", len(as.StashedPrompts))).FontSize(12).FontWeight(500)
				})
				stashChip.Menu(func(m *ui.Menu) {
					for i, st := range as.StashedPrompts {
						idx := i
						title := strings.ReplaceAll(st, "\n", " ")
						if runes := []rune(title); len(runes) > 60 {
							title = string(runes[:57]) + "…"
						}
						if m.Item(title).Chosen() {
							as.RestoreStash(idx)
							as.SavePromptStash()
						}
					}
					if len(as.StashedPrompts) > 0 {
						m.Separator()
						if m.Item("Clear stash").Danger().Chosen() {
							as.StashedPrompts = nil
							as.SavePromptStash()
						}
					}
				})
			}

			// (The former "Verify" chip was removed: it only toggled
			// as.AutoVerify, which no backend ever read — dead UI.)

			ui.Box(c).Grow(1)

			// Context Window Usage Ring (ZCode-style indicator next to model)
			_, pct := as.ContextUsage()
			gaugeBtn := ui.Row(c).Height(26).Radius(999).Gap(4).AlignItems(ui.Center).Cursor(ui.CursorPointer)
			if isSlim {
				gaugeBtn.Padding(0, 4)
			} else {
				gaugeBtn.Padding(0, 6)
			}
			gaugeBtn.Children(func() {
				ringColor := tc.TextMuted.Alpha(0.55)
				if pct >= 80 {
					ringColor = tc.Error
				} else if pct >= 60 {
					ringColor = tc.Warning
				} else if pct > 0 {
					ringColor = tc.Success.Alpha(0.8)
				}
				ui.Box(c).Size(14, 14).Radius(7).BorderWidth(2).BorderColor(ringColor)
				if !isSlim && !isHome && pct > 0 {
					ui.Text(c, fmt.Sprintf("%d%%", pct)).FontSize(11).FontWeight(500).TextColor(tc.TextMuted)
				}
			})
			gaugeBtn.Tooltip(fmt.Sprintf("%d%% context window used", pct))
			if gaugeBtn.Hovered() {
				gaugeBtn.Background(tc.Text.Alpha(0.04))
			}
			if gaugeBtn.Clicked() {
				a.openUsageModal()
			}

			// Model + Effort Selector Button (Triggers ModelProviderPaletteModal)
			disp := shortModelLabel(as.ActiveModel)
			if isSlim {
				if runes := []rune(disp); len(runes) > 15 {
					disp = string(runes[:14]) + "…"
				}
			}
			modelBtn := ui.Row(c.Key("composer-model")).Height(26).Radius(6).Padding(0, 7).Gap(5).
				AlignItems(ui.Center).Cursor(ui.CursorPointer)
			modelBtn.Tooltip(fmt.Sprintf("Model: %s", as.ActiveModel))
			modelBtn.Children(func() {
				icons.RenderProviderIcon(c, as.ActiveModel, 12)
				ui.Text(c, disp).FontSize(11.5).FontWeight(500).TextColor(tc.Text).SingleLine()
				if !isSlim && as.ReasoningEffort != agent.EffortOff && !modelEncodesEffort(as.ActiveAgentID, as.ActiveModel) {
					ui.Text(c, as.ReasoningEffortLabel()).FontSize(11).FontWeight(400).TextColor(tc.TextMuted)
				}
				ui.Icon(c, icons.TablerChevronDown).Width(9).Height(9).TextColor(tc.IconMuted)
			})
			if modelBtn.Hovered() {
				modelBtn.Background(tc.Text.Alpha(0.06))
			}
			if modelBtn.Clicked() {
				a.ShowModelPalette = true
				a.ModelSearchQuery = ""
			}

			// Send / Steer / Queue / Stop controls
			if as.StreamingSnapshot() {
				if strings.TrimSpace(as.InputText) != "" {
					queueBtn := ui.Row(c).Height(28).Radius(999).Padding(0, 10).Gap(4).AlignItems(ui.Center).
						Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
					queueBtn.Children(func() {
						ui.Icon(c, icons.TablerPlus).Width(11).Height(11).TextColor(tc.TextMuted)
						ui.Text(c, "Queue").FontSize(11).FontWeight(500)
					})
					if queueBtn.Clicked() {
						as.Lock()
						as.SteerQueue = append(as.SteerQueue, as.InputText)
						as.InputText = ""
						as.Unlock()
						as.SaveSteerQueues()
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
				stopBtn := ui.Row(c).Size(28, 28).Radius(999).AlignItems(ui.Center).Justify(ui.Center).
					Background(tc.Error.Alpha(0.9)).
					Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.2))
				stopBtn.Children(func() {
					ui.Icon(c, icons.TablerSquare).Width(12).Height(12).TextColor(tc.ErrorForeground)
				})
				if stopBtn.Clicked() {
					as.CancelStreaming()
				}
			} else {
				hasInput := strings.TrimSpace(as.InputText) != "" || len(as.AttachedFiles) > 0
				sendBtn := ui.Row(c).Size(28, 28).Radius(999).AlignItems(ui.Center).Justify(ui.Center)
				if hasInput {
					sendBtn.Background(tc.Text).
						Shadow(0, 1, 3, 0, ui.RGBA(0, 0, 0, 0.2))
					sendBtn.Children(func() {
						ui.Icon(c, icons.TablerArrowUp).Width(15).Height(15).TextColor(tc.Canvas)
					})
					if sendBtn.Clicked() {
						trimmed := strings.TrimSpace(as.InputText)
						if !a.tryHandleSlashCommand(trimmed) {
							as.ListState.FollowEnd = true
							as.SubmitMessage()
						}
					}
				} else {
					sendBtn.Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border.Alpha(0.6))
					sendBtn.Children(func() {
						ui.Icon(c, icons.TablerArrowUp).Width(14).Height(14).TextColor(tc.IconMuted)
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
// heading and header/tray project chips: recent workspaces + session workspaces
// (deduped), then "Open folder…" and "General (Work outside project)".
func (a *AppUI) workspaceMenuItems(m *ui.Menu) {
	current := ""
	if a.Workspace != nil {
		current = a.Workspace.RootDir
		if !strings.HasPrefix(current, "ssh://") {
			current = agentsession.NormalizeWorkspacePath(current)
		}
	}
	seen := make(map[string]bool)
	var projects []string
	for _, ws := range a.RecentWorkspaces {
		norm := ws
		if !strings.HasPrefix(norm, "ssh://") {
			norm = agentsession.NormalizeWorkspacePath(norm)
		}
		if norm == "" || agentsession.IsTempTestPath(norm) || seen[norm] {
			continue
		}
		seen[norm] = true
		projects = append(projects, norm)
	}
	for _, ws := range a.recentWorkspacePaths() {
		if ws == "" || seen[ws] {
			continue
		}
		seen[ws] = true
		projects = append(projects, ws)
	}
	for _, ws := range projects {
		path := ws
		label := resolveProjectName(path)
		if strings.HasPrefix(path, "ssh://") {
			_, remotePath, displayHost, folder := parseSSHWorkspaceURI(path)
			if folder == "" || folder == "remote-root" {
				folder = filepath.Base(remotePath)
			}
			label = fmt.Sprintf("SSH: %s (%s)", displayHost, folder)
		}
		if m.Item(menuChecked(sameWorkspacePath(path, current), label)).Chosen() {
			a.switchWorkspace(path)
		}
	}
	m.Separator()
	if m.Item("Switch Workspace...").Shortcut(ui.Cmd|ui.Alt, ui.KeyO).Chosen() {
		a.OpenProjectSwitcherModal()
	}
	if m.Item("Open folder…").Chosen() {
		a.openWorkspaceFolder()
	}
	if m.Item("Connect SSH Remote Folder...").Shortcut(ui.Cmd|ui.Shift, ui.KeyR).Chosen() {
		a.OpenSSHModal()
	}
	if m.Item(menuChecked(current == "", "General (Work outside project)")).Chosen() {
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
			normWs := agentsession.NormalizeWorkspacePath(sess.WorkspacePath)
			if normWs == "" || agentsession.IsTempTestPath(normWs) {
				continue
			}
			found := false
			for i := range entries {
				if entries[i].path == normWs {
					if sess.UpdatedAt.After(entries[i].latestAt) {
						entries[i].latestAt = sess.UpdatedAt
					}
					found = true
					break
				}
			}
			if !found {
				entries = append(entries, entry{path: normWs, latestAt: sess.UpdatedAt})
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
	// Fused to the composer card below: same width (FillWidth inside the
	// landing unit), shared top corners, one continuous card silhouette.
	ui.Row(c).FillWidth().Padding(7, 16).Gap(6).AlignItems(ui.Center).
		Background(tc.Surface).
		BorderWidth(1, 1, 0, 1).BorderColor(tc.Border).
		Radius(16, 16, 0, 0).Children(func() {
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

		// Git branch chip — only when an actual local git repository workspace is open.
		branch := ""
		if a.Workspace != nil && a.Workspace.RootDir != "" && !strings.HasPrefix(a.Workspace.RootDir, "ssh://") && a.Git != nil {
			// One locked snapshot per function: GitManager fields are written
			// from background refresh goroutines.
			branch = a.Git.Snapshot().Branch
		}
		if branch != "" {
			dirtyFiles, _, _ := a.gitDirtySummary()
			branchChip := ui.Row(c.Key("tray-branch")).Height(26).Radius(6).Padding(0, 7).Gap(5).AlignItems(ui.Center).Cursor(ui.CursorPointer)
			branchChip.Children(func() {
				ui.Icon(c, icons.TablerGitBranch).Width(12).Height(12).TextColor(tc.TextMuted.Alpha(0.7))
				ui.Text(c, branch).FontSize(12).TextColor(tc.TextMuted)
				if dirtyFiles > 0 {
					ui.Row(c).Height(16).Radius(4).Padding(0, 5).AlignItems(ui.Center).
						Background(tc.Warning.Alpha(0.14)).Children(func() {
						ui.Text(c, fmt.Sprintf("%d uncommitted", dirtyFiles)).FontSize(10).FontWeight(600).TextColor(tc.Warning)
					})
				}
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
	projectName := resolveProjectName(as.WorkspacePath())

	ui.Column(c).Grow(1).Fill().Center().Gap(24).Padding(24, 32).Background(tc.Canvas).Children(func() {
		ui.Column(c).AlignItems(ui.Center).Gap(12).Children(func() {
			ui.Box(c).Size(54, 54).Radius(16).AlignItems(ui.Center).Justify(ui.Center).
				Background(tc.Accent.Alpha(0.12)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.28)).
				Children(func() {
					ui.Icon(c, icons.ForgeSparkSVG).Width(30).Height(30).TextColor(tc.Accent)
				})
			// Heading: workspace name opens the workspace switcher
			ui.Row(c).Gap(0).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "What should we do in").FontSize(26).FontWeight(600).LetterSpacing(-0.7)
				ui.Box(c).Width(8)
				wsBtn := ui.Row(c).AlignItems(ui.Center).Cursor(ui.CursorPointer)
				wsBtn.Children(func() {
					ui.Text(c, projectName).FontSize(26).FontWeight(600).LetterSpacing(-0.7).
						DottedUnderline().DecorationColor(tc.Accent.Alpha(0.6)).UnderlineOffset(6)
				})
				wsBtn.Menu(func(m *ui.Menu) {
					a.workspaceMenuItems(m)
				})
				ui.Text(c, "?").FontSize(26).FontWeight(600).LetterSpacing(-0.7)
			})
			ui.Text(c, "Explore code, edit files, run tools, or plan multi-step tasks.").
				FontSize(12.5).TextColor(tc.TextMuted)
		})

		// Landing unit: 2x2 starter prompt cards + fused context tray & composer
		ui.Column(c).MaxWidth(768).FillWidth().Gap(16).AlignItems(ui.Center).Children(func() {
			type starterItem struct {
				icon   *ui.SVG
				label  string
				desc   string
				prompt string
			}
			row1 := []starterItem{
				{icons.TablerCompass, "Explore architecture", "Map key packages, entry points & data flow", "Summarize the architecture of this repository, key packages, and entry points."},
				{icons.TablerBug, "Find & fix a bug", "Diagnose root cause and implement a clean fix", "Help me diagnose and fix an issue in the codebase: "},
			}
			row2 := []starterItem{
				{icons.TablerFlask, "Run & improve tests", "Execute suite, fix failures & expand coverage", "Run the test suite, identify any failing or missing unit tests, and propose improvements."},
				{icons.TablerGitPullRequest, "Review working tree", "Audit uncommitted git changes for edge cases", "Review uncommitted git changes in this workspace for bugs, edge cases, and style."},
			}
			renderStarterRow := func(items []starterItem) {
				ui.Row(c).FillWidth().Gap(10).Children(func() {
					for _, s := range items {
						thisS := s
						card := ui.Row(c.Key("sugg-"+thisS.label)).Grow(1).MinWidth(0).Radius(12).Padding(11, 13).Gap(10).
							AlignItems(ui.Center).Cursor(ui.CursorPointer).
							Background(tc.Surface.Alpha(0.75)).BorderWidth(1).BorderColor(tc.Border.Alpha(0.75))
						card.Children(func() {
							ui.Box(c).Size(32, 32).Radius(8).Shrink(0).AlignItems(ui.Center).Justify(ui.Center).
								Background(tc.ToolbarControl).Children(func() {
								ui.Icon(c, thisS.icon).Width(15).Height(15).TextColor(tc.Accent)
							})
							ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
								ui.Text(c, thisS.label).FontSize(12.5).FontWeight(600).TextColor(tc.Text).SingleLine()
								ui.Text(c, thisS.desc).FontSize(11).TextColor(tc.TextMuted).SingleLine()
							})
						})
						if card.Hovered() {
							card.Background(tc.SurfaceRaised).BorderColor(tc.Accent.Alpha(0.45))
						}
						if card.Clicked() {
							as.InputText = thisS.prompt
							if as.Invalidate != nil {
								as.Invalidate()
							}
						}
					}
				})
			}
			ui.Column(c).FillWidth().Gap(10).Children(func() {
				renderStarterRow(row1)
				renderStarterRow(row2)
			})

			// Tray fused to the card with zero gap: same width, shared borders.
			ui.Column(c).FillWidth().Gap(0).Children(func() {
				a.renderLandingTray(c, tc, projectName)
				a.renderComposerCard(c, tc, true)
			})
		})

	})
}

func (a *AppUI) terminalSessionCount() int {
	if a.AgentTerminal == nil {
		return 0
	}
	count := a.AgentTerminal.TabCount()
	if count == 0 && a.AgentTerminal.ActiveSessionID != "" {
		return 1
	}
	return count
}

// renderInspectorBody renders the Plans, Progress, Terminals, and Agents sections
// shared between the floating ZCode card and the Right Sidebar Plans pane.
func (a *AppUI) renderInspectorBody(c *ui.Context, tc theme.ThemeColors, isFloating bool) {
	if a.Agent == nil {
		return
	}
	tm := a.Agent.TaskManager
	var tasks []agent.TaskItem
	var completed, total int
	if tm != nil {
		tasks = tm.GetTasks()
		completed, total = tm.Progress()
	}

	// 1. Plans header + active goal
	ui.Column(c).Gap(8).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Plans").FontSize(13).FontWeight(600).Grow(1)
			menuBtn := ui.Row(c.Key("insp-plans-more")).Size(22, 22).Radius(5).AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
			menuBtn.Children(func() {
				ui.Icon(c, icons.TablerDots).Width(13).Height(13).TextColor(tc.IconMuted)
			})
			if menuBtn.Hovered() {
				menuBtn.Background(tc.Text.Alpha(0.06))
			}
			menuBtn.Menu(func(m *ui.Menu) {
				if m.Item("Open Agents Drawer").Chosen() {
					a.ShowRightDrawer = true
					a.RightDrawerTab = RightTabSubagent
				}
				if tm != nil && len(tasks) > 0 {
					m.Separator()
					if m.Item("Clear tasks").Danger().Chosen() {
						tm.Clear()
					}
				}
			})

			if isFloating {
				collapseBtn := ui.Row(c.Key("insp-collapse")).Size(22, 22).Radius(5).AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
				collapseBtn.Children(func() {
					ui.Icon(c, icons.TablerMinimize2).Width(13).Height(13).TextColor(tc.IconMuted)
				})
				if collapseBtn.Hovered() {
					collapseBtn.Background(tc.Text.Alpha(0.06))
				}
				if collapseBtn.Clicked() {
					a.InspectorCollapsed = true
				}
			}
		})

		goal := ""
		if tm != nil {
			goal = tm.Goal
		}
		if goal == "" {
			if activeSess := a.Agent.ActiveSession(); activeSess != nil && activeSess.Title != "" && activeSess.Title != "New Task" {
				goal = activeSess.Title
			} else {
				goal = "No active plan"
			}
		}
		ui.Row(c).Gap(7).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icons.TablerList).Width(13).Height(13).TextColor(tc.IconMuted)
			ui.Text(c, goal).FontSize(12).TextColor(tc.Text).SingleLine().Grow(1)
		})
	})

	// 2. Progress section
	ui.Box(c).FillWidth().Height(1).Background(tc.Border.Alpha(0.6))
	ui.Column(c).Gap(8).Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Progress").FontSize(13).FontWeight(600)
			countColor := tc.Success
			if total == 0 {
				countColor = tc.TextMuted
			}
			ui.Text(c, fmt.Sprintf("%d/%d", completed, total)).FontSize(12).FontWeight(600).TextColor(countColor)
		})

		if len(tasks) == 0 {
			ui.Text(c, "No tasks tracked yet").FontSize(12).TextColor(tc.TextMuted)
		} else {
			ui.Column(c).Gap(6).Children(func() {
				for _, task := range tasks {
					tid := task.ID
					isDone := task.Status == agent.TaskCompleted
					isCurrent := task.Status == agent.TaskInProgress
					row := ui.Row(c.Key("trk-"+tid)).Gap(8).AlignItems(ui.Center).MinHeight(22).Padding(2, 4).Radius(6).Cursor(ui.CursorPointer)
					row.Children(func() {
						if isDone {
							ui.Icon(c, icons.TablerCircleCheck).Width(14).Height(14).TextColor(tc.Success)
						} else if isCurrent {
							ui.Icon(c, icons.TablerArrowRight).Width(14).Height(14).TextColor(tc.Accent)
						} else {
							ui.Icon(c, icons.TablerCircle).Width(14).Height(14).TextColor(tc.TextMuted)
						}
						txt := ui.Text(c, task.Title).FontSize(12).LineHeight(16).Grow(1)
						if isDone {
							txt.TextColor(tc.TextMuted)
						} else if isCurrent {
							txt.FontWeight(600).TextColor(tc.Accent)
						}
					})
					if row.Hovered() {
						row.Background(tc.Text.Alpha(0.04))
					}
					if row.Clicked() && tm != nil {
						tm.ToggleTask(tid)
					}
				}
			})
		}
	})

	// 3. Terminals section
	ui.Box(c).FillWidth().Height(1).Background(tc.Border.Alpha(0.6))
	termCount := a.terminalSessionCount()
	termRow := ui.Row(c.Key("insp-terminals")).Padding(4, 4).Radius(6).Gap(8).AlignItems(ui.Center).Cursor(ui.CursorPointer)
	termRow.Children(func() {
		ui.Text(c, "Terminals").FontSize(13).FontWeight(600).Grow(1)
		termMeta := "0 background"
		if termCount > 0 {
			termMeta = fmt.Sprintf("%d background", termCount)
		}
		ui.Text(c, termMeta).FontSize(12).TextColor(tc.TextMuted)
	})
	if termRow.Hovered() {
		termRow.Background(tc.Text.Alpha(0.04))
	}
	if termRow.Clicked() {
		a.ShowAgentBottomTerminal = !a.ShowAgentBottomTerminal
	}

	// 4. Agents / Orchestrator Personas section
	ui.Box(c).FillWidth().Height(1).Background(tc.Border.Alpha(0.6))
	subs := a.Agent.Subagents(a.Agent.ActiveSessionID)
	ui.Column(c).Gap(6).Children(func() {
		agentsHdr := ui.Row(c.Key("insp-agents-hdr")).Padding(2, 4).Radius(6).Gap(6).AlignItems(ui.Center).Cursor(ui.CursorPointer)
		agentsHdr.Children(func() {
			ui.Text(c, "Agents").FontSize(13).FontWeight(600).Grow(1)
			if len(subs) > 0 {
				ui.Text(c, fmt.Sprintf("%d", len(subs))).FontSize(12).TextColor(tc.TextMuted)
			}
		})
		if agentsHdr.Hovered() {
			agentsHdr.Background(tc.Text.Alpha(0.04))
		}
		if agentsHdr.Clicked() {
			if len(subs) > 0 {
				a.ShowRightDrawer = true
				a.RightDrawerTab = RightTabSubagent
			} else {
				a.openSettings(settings.SettingsTabSubagents)
			}
		}

		if len(subs) > 0 {
			limit := len(subs)
			if limit > 5 {
				limit = 5
			}
			for i := 0; i < limit; i++ {
				sub := subs[i]
				_, statusColor := subagentStatus(string(sub.State), tc)
				subRow := ui.Row(c.Key("insp-sub-"+sub.ID)).Padding(3, 6).Radius(6).Gap(6).AlignItems(ui.Center).Cursor(ui.CursorPointer)
				subRow.Children(func() {
					ui.Box(c).Size(6, 6).Radius(3).Background(statusColor)
					name := sub.Name
					if name == "" {
						name = "Subagent"
					}
					ui.Text(c, name).FontSize(12).SingleLine().Grow(1)
					if sub.RoleFilter != "" {
						ui.Text(c, string(sub.RoleFilter)).FontSize(11).TextColor(tc.TextMuted)
					}
				})
				if subRow.Hovered() {
					subRow.Background(tc.Text.Alpha(0.04))
				}
				if subRow.Clicked() {
					a.SelectedSubagentID = sub.ID
					a.ShowRightDrawer = true
					a.RightDrawerTab = RightTabSubagent
				}
			}
		} else {
			_, personaName := a.Agent.EffectivePersona(a.Agent.ActiveSession())
			if personaName == "" {
				personaName = "Orchestrator"
			}
			leadRow := ui.Row(c.Key("insp-lead-persona")).Padding(3, 6).Radius(6).Gap(6).AlignItems(ui.Center).Cursor(ui.CursorPointer)
			leadRow.Children(func() {
				ui.Icon(c, icons.TablerBot).Width(13).Height(13).TextColor(tc.IconMuted)
				ui.Text(c, personaName).FontSize(12).TextColor(tc.TextMuted).SingleLine().Grow(1)
				ui.Icon(c, icons.TablerChevronRight).Width(12).Height(12).TextColor(tc.IconMuted)
			})
			if leadRow.Hovered() {
				leadRow.Background(tc.Text.Alpha(0.04))
			}
			if leadRow.Clicked() {
				a.openSettings(settings.SettingsTabSubagents)
			}
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
		isRunning := sess.Status == "running" && sess.Terminal != nil && !sess.Terminal.IsExited()

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
		ui.Box(c).Grow(1).FillHeight().Children(func() {
			if sess.Terminal != nil && sess.Terminal.Term() != nil {
				a.renderTerminal(c, sess.Terminal)
			} else {
				// Empty state for restored / ended CLI session
				ui.Column(c).Grow(1).Fill().Center().Gap(14).Children(func() {
					ui.Box(c).Size(56, 56).Radius(14).Background(t.Surface).BorderWidth(1).BorderColor(t.Border).
						AlignItems(ui.Center).Justify(ui.Center).Children(func() {
						icons.RenderProviderIcon(c, sess.AgentID, 28)
					})
					ui.Column(c).AlignItems(ui.Center).Gap(4).Children(func() {
						ui.Text(c, sess.Title).FontSize(16).FontWeight(600).TextColor(t.Text)
						ui.Text(c, fmt.Sprintf("Agent CLI session in %s", sess.Cwd)).FontSize(12).TextColor(t.TextMuted)
					})
					resumeBtn := ui.Row(c.Key("cli-resume-btn")).Height(32).Radius(8).Padding(0, 16).Gap(6).
						AlignItems(ui.Center).Background(t.Accent).Cursor(ui.CursorPointer)
					resumeBtn.Children(func() {
						fg := a.ThemeMgr.Colors().AccentForeground
						ui.Icon(c, icons.TablerRotate).Width(14).Height(14).TextColor(fg)
						ui.Text(c, "Resume Session").FontSize(12).FontWeight(600).TextColor(fg)
					})
					if resumeBtn.Clicked() {
						a.CLIMgr.RestartSession(sess.ID)
					}
				})
			}

			// Floating Session Ended Overlay matching webview's SessionEndedOverlay
			if !isRunning {
				ui.Row(c).Absolute().Bottom(16).FillWidth().Justify(ui.Center).Children(func() {
					ui.Row(c.Key("cli-ended-overlay")).Radius(999).Padding(4, 12).Gap(8).AlignItems(ui.Center).
						Background(t.Surface.Alpha(0.95)).BorderWidth(1).BorderColor(t.Border).
						Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, 0.35)).Children(func() {
						statusText := "Session ended"
						if sess.ExitCode != 0 {
							statusText = fmt.Sprintf("Session ended · exit %d", sess.ExitCode)
						}
						ui.Text(c, statusText).FontSize(11).TextColor(t.TextMuted)
						reBtn := ui.Row(c.Key("overlay-re-btn")).Height(22).Radius(999).Padding(0, 10).Gap(4).
							AlignItems(ui.Center).Background(t.Accent).Cursor(ui.CursorPointer)
						reBtn.Children(func() {
							fg := a.ThemeMgr.Colors().AccentForeground
							ui.Icon(c, icons.TablerRotate).Width(11).Height(11).TextColor(fg)
							ui.Text(c, "Restart").FontSize(11).FontWeight(600).TextColor(fg)
						})
						if reBtn.Clicked() {
							a.CLIMgr.RestartSession(sess.ID)
						}
					})
				})
			}
		})
	})
}

func summarizeTools(tools []*agent.ToolExecution) (string, bool, bool) {
	commands, reads, edits, searches, skills, mcps, designs, others := 0, 0, 0, 0, 0, 0, 0, 0
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
		case strings.Contains(n, "skill"):
			skills++
		case strings.HasPrefix(n, "mcp__") || strings.HasPrefix(n, "mcp_") || strings.Contains(n, "mcp"):
			mcps++
		case strings.Contains(n, "deck") || strings.Contains(n, "motion") || strings.Contains(n, "opendesign") || strings.Contains(n, "open-design") || strings.Contains(n, "forge-design"):
			designs++
		case strings.Contains(n, "bash") || strings.Contains(n, "shell") || strings.Contains(n, "command") || strings.Contains(n, "exec") || strings.Contains(n, "terminal") || strings.Contains(n, "run_command"):
			commands++
		case strings.Contains(n, "read") || strings.Contains(n, "view") || strings.Contains(n, "cat"):
			reads++
		case strings.Contains(n, "edit") || strings.Contains(n, "write") || strings.Contains(n, "patch") || strings.Contains(n, "create"):
			edits++
		case strings.Contains(n, "search") || strings.Contains(n, "grep") || strings.Contains(n, "find") || strings.Contains(n, "glob"):
			searches++
		default:
			others++
		}
	}

	var parts []string
	if skills > 0 {
		if skills == 1 {
			parts = append(parts, "Loaded 1 skill")
		} else {
			parts = append(parts, fmt.Sprintf("Loaded %d skills", skills))
		}
	}
	if mcps > 0 {
		if mcps == 1 {
			parts = append(parts, "Ran 1 MCP tool")
		} else {
			parts = append(parts, fmt.Sprintf("Ran %d MCP tools", mcps))
		}
	}
	if designs > 0 {
		parts = append(parts, "Generated design block")
	}
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

func (a *AppUI) tryHandleSlashCommand(trimmed string) bool {
	as := a.Agent
	switch trimmed {
	case "/usage":
		a.openUsageModal()
		as.InputText = ""
		return true
	case "/settings", "/setup":
		a.openSettings()
		as.InputText = ""
		return true
	case "/model":
		a.ShowModelPalette = true
		a.ModelSearchQuery = ""
		as.InputText = ""
		return true
	case "/clear":
		// Shared locked implementation: nils both the session messages and
		// the active alias, resets streaming state, persists async.
		as.ClearConversation()
		as.InputText = ""
		return true
	}
	return false
}

type EditPair struct {
	OldString string
	NewString string
}

type ParsedToolArgs struct {
	Path      string
	Cmd       string
	Content   string
	OldString string
	NewString string
	Query     string
	Edits     []EditPair
}

func parseToolArgsCached(t *agent.ToolExecution) ParsedToolArgs {
	if t == nil {
		return ParsedToolArgs{}
	}
	if p, ok := t.ParsedArgs.(ParsedToolArgs); ok {
		return p
	}
	parsed := parseToolArgs(t.Command, t.ToolName)
	t.ParsedArgs = parsed
	return parsed
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
				case "old_string", "oldstring", "old", "old_text", "target":
					if res.OldString == "" {
						res.OldString = strVal
					}
				case "search":
					if isSearchTool(toolName) {
						if res.Query == "" {
							res.Query = strVal
						}
					} else if res.OldString == "" {
						res.OldString = strVal
					}
				case "query", "pattern", "regex", "glob", "search_query":
					if res.Query == "" {
						res.Query = strVal
					}
				case "new_string", "newstring", "new", "new_text", "replacement", "replace":
					if res.NewString == "" {
						res.NewString = strVal
					}
				}
			}
			for _, arrKey := range []string{"edits", "changes", "chunks"} {
				arr, ok := m[arrKey].([]interface{})
				if !ok {
					continue
				}
				for _, item := range arr {
					obj, ok := item.(map[string]interface{})
					if !ok {
						continue
					}
					var ep EditPair
					for ek, ev := range obj {
						sv, ok := ev.(string)
						if !ok {
							continue
						}
						switch strings.ToLower(ek) {
						case "old_string", "oldstring", "old", "old_text", "target", "search":
							if ep.OldString == "" {
								ep.OldString = sv
							}
						case "new_string", "newstring", "new", "new_text", "replacement", "replace":
							if ep.NewString == "" {
								ep.NewString = sv
							}
						}
					}
					if ep.OldString != "" || ep.NewString != "" {
						res.Edits = append(res.Edits, ep)
					}
				}
			}
			if len(res.Edits) == 1 && res.OldString == "" && res.NewString == "" {
				res.OldString = res.Edits[0].OldString
				res.NewString = res.Edits[0].NewString
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

func isSearchTool(name string) bool {
	nl := strings.ToLower(name)
	return strings.Contains(nl, "search") || strings.Contains(nl, "grep") || strings.Contains(nl, "glob") || strings.Contains(nl, "find")
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

// questionDraftsMu guards AppUI.QuestionDrafts: render reads/writes the map
// in-frame while submit goroutines clear it (a concurrent map write crashes).
// No other file touches the map, so a package-level lock here is sufficient.
var questionDraftsMu sync.Mutex

// questionDraft returns (creating if needed) the draft string for a key.
func (a *AppUI) questionDraft(key string) *string {
	questionDraftsMu.Lock()
	defer questionDraftsMu.Unlock()
	d, ok := a.QuestionDrafts[key]
	if !ok || d == nil {
		d = new(string)
		a.QuestionDrafts[key] = d
	}
	return d
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
					// SubmitQuestionAnswers restores the pending questions when
					// delivery fails, so only clear the drafts on success — the
					// user must be able to fix and retry (and AddToast from a
					// goroutine is silently dropped; use queueToast).
					if err := a.Agent.SubmitQuestionAnswers(session); err != nil {
						a.queueToast(ui.Toast{Title: "Couldn't Submit Answers", Description: err.Error(), Type: "error"})
						if a.Window != nil {
							a.Window.Invalidate()
						}
						return
					}
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
	draft := a.questionDraft(draftKey)

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

	// Only update & invalidate when the draft actually changed, avoiding a redraw loop every frame.
	if trimmed := strings.TrimSpace(*draft); trimmed != q.Custom {
		a.Agent.UpdateQuestion(sessionID, qi, func(qu *agent.AgentQuestion) {
			qu.Custom = trimmed
		})
	}
}

// clearQuestionDrafts drops the custom-answer drafts of a session.
func (a *AppUI) clearQuestionDrafts(sessionID string) {
	questionDraftsMu.Lock()
	defer questionDraftsMu.Unlock()
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

	a.modal(c, &a.ShowQuestionModal, func() {
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
					session := sessionID
					go func() {
						// Same restore-aware contract as the inline card: on
						// failure the questions come back and the modal stays
						// open so the user can retry; the modal closes only
						// after the answers were delivered.
						if err := a.Agent.SubmitQuestionAnswers(session); err != nil {
							a.queueToast(ui.Toast{Title: "Couldn't Submit Answers", Description: err.Error(), Type: "error"})
							return
						}
						a.clearQuestionDrafts(session)
						a.runOnUI(func() {
							a.ShowQuestionModal = false
							if a.Window != nil {
								a.Window.Invalidate()
							}
						})
					}()
				}
			})
		})
	})
}

// userMessageAttachments extracts the @path entries of a user message's
// "Attached files:" block (as SubmitMessage formats them).
func userMessageAttachments(text string) []string {
	idx := strings.Index(text, "Attached files:")
	if idx < 0 {
		return nil
	}
	// Back up to start of the line containing "Attached files:"
	lineStart := strings.LastIndexByte(text[:idx], '\n')
	if lineStart >= 0 {
		idx = lineStart + 1
	} else {
		idx = 0
	}
	var atts []string
	inBlock := false
	rest := text[idx:]
	for len(rest) > 0 {
		var line string
		if nl := strings.IndexByte(rest, '\n'); nl >= 0 {
			line = rest[:nl]
			rest = rest[nl+1:]
		} else {
			line = rest
			rest = ""
		}
		trimmed := strings.TrimSpace(line)
		if !inBlock {
			if strings.HasPrefix(trimmed, "Attached files:") {
				inBlock = true
			}
			continue
		}
		if trimmed == "" {
			break // blank line ends the attachment block
		}
		if strings.HasPrefix(trimmed, "@") {
			atts = append(atts, strings.TrimPrefix(trimmed, "@"))
		}
	}
	return atts
}

// extractDiffFromToolOutput extracts a unified diff from either raw unified
// diff text or a JSON tool output payload like {"additions":45,"diff":"@@ ..."}
// (including truncated JSON payloads ending in "... [truncated]").
func extractDiffFromToolOutput(out, filePath string) string {
	trimmed := strings.TrimSpace(out)
	if trimmed == "" {
		return ""
	}
	diffStr := ""
	if strings.HasPrefix(trimmed, "{") {
		var payload struct {
			Diff  string `json:"diff"`
			Patch string `json:"patch"`
		}
		if err := json.Unmarshal([]byte(trimmed), &payload); err == nil {
			if payload.Diff != "" {
				diffStr = payload.Diff
			} else if payload.Patch != "" {
				diffStr = payload.Patch
			}
		} else {
			// Handle truncated JSON where the closing quote/brace was cut off.
			for _, key := range []string{`"diff":"`, `"diff": "`, `"patch":"`, `"patch": "`} {
				if idx := strings.Index(trimmed, key); idx >= 0 {
					rest := trimmed[idx+len(key):]
					rest = strings.TrimSuffix(rest, "... [truncated]")
					rest = strings.TrimSuffix(rest, "… [truncated]")
					// Walk up to an unescaped closing quote if present, or end of prefix.
					var rawLit strings.Builder
					rawLit.WriteByte('"')
					escaped := false
					for i := 0; i < len(rest); i++ {
						ch := rest[i]
						if escaped {
							rawLit.WriteByte(ch)
							escaped = false
							continue
						}
						if ch == '\\' {
							// Avoid dangling backslash at the very end of a truncated string.
							if i == len(rest)-1 {
								break
							}
							rawLit.WriteByte(ch)
							escaped = true
							continue
						}
						if ch == '"' {
							break
						}
						rawLit.WriteByte(ch)
					}
					rawLit.WriteByte('"')
					var decoded string
					if err := json.Unmarshal([]byte(rawLit.String()), &decoded); err == nil {
						// Drop any incomplete trailing line after truncation.
						if lastNL := strings.LastIndex(decoded, "\n"); lastNL > 0 {
							decoded = decoded[:lastNL]
						}
						diffStr = decoded
					}
					break
				}
			}
		}
	} else if strings.Contains(trimmed, "@@ -") {
		diffStr = trimmed
	}

	diffStr = strings.TrimSpace(diffStr)
	if !strings.Contains(diffStr, "@@ -") {
		return ""
	}
	if !strings.Contains(diffStr, "--- ") || !strings.Contains(diffStr, "+++ ") {
		name := filePath
		if name == "" {
			name = "file"
		}
		diffStr = fmt.Sprintf("--- a/%s\n+++ b/%s\n%s\n", name, name, diffStr)
	}
	return diffStr
}

// renderToolEditDiff draws the old/new strings of an edit tool call as a
// native MyGo DiffView with syntax coloring, line numbers, word-level diff highlights,
// and quick Copy / Open in Editor actions.
func (a *AppUI) renderToolEditDiff(c *ui.Context, tc theme.ThemeColors, key, filePath, oldStr, newStr string) {
	if oldStr == "" && newStr == "" {
		return
	}
	d := a.Editor.GetOrCreateBufferDiffWithLang("tool-edit-"+key, filePath, oldStr, newStr, a.ThemeMgr)
	if d == nil {
		return
	}
	box := ui.Column(c).Radius(8).Background(tc.Canvas).BorderWidth(1).BorderColor(tc.Border.Alpha(0.5)).Clip()
	box.Children(func() {
		if filePath != "" {
			ui.Row(c).Height(26).Padding(0, 10).Gap(6).AlignItems(ui.Center).
				Background(tc.Surface).BorderWidth(0, 0, 1, 0).BorderColor(tc.Border.Alpha(0.45)).Children(func() {
				icons.RenderFileIcon(c, filePath, 13, tc.IconMuted)
				ui.Text(c, filepath.Base(filePath)).FontSize(11).FontWeight(600).TextColor(tc.Text).Grow(1).SingleLine()
				copyBtn := ui.Row(c.Key("tdiff-copy-"+key)).Height(18).Radius(4).Padding(0, 6).Gap(3).AlignItems(ui.Center).
					Cursor(ui.CursorPointer).Tooltip("Copy new content")
				copyBtn.Children(func() {
					ui.Icon(c, icons.TablerCopy).Width(10).Height(10).TextColor(tc.IconMuted)
					ui.Text(c, "Copy").FontSize(10).TextColor(tc.TextMuted)
				})
				if copyBtn.Hovered() {
					copyBtn.Background(tc.Text.Alpha(0.06))
				}
				if copyBtn.Clicked() {
					mygo.Clipboard.WriteText(newStr)
					c.AddToast(ui.Toast{Title: "Copied Diff Content", Type: "success"})
				}
				openBtn := ui.Row(c.Key("tdiff-open-"+key)).Height(18).Radius(4).Padding(0, 6).Gap(3).AlignItems(ui.Center).
					Cursor(ui.CursorPointer).Tooltip("Open in Editor")
				openBtn.Children(func() {
					ui.Icon(c, icons.TablerCode).Width(10).Height(10).TextColor(tc.IconMuted)
					ui.Text(c, "Open").FontSize(10).TextColor(tc.TextMuted)
				})
				if openBtn.Hovered() {
					openBtn.Background(tc.Text.Alpha(0.06))
				}
				if openBtn.Clicked() {
					a.openTranscriptFileInSidePane(filePath)
				}
			})
		}
		mygoeditor.DiffView(c, d).FillWidth()
	})
}

// chatColumnWidth computes the width actually available to the chat column
// (window minus optional left sidebar and right drawer), used to cap the
// transcript's intrinsic width.
func (a *AppUI) chatColumnWidth(c *ui.Context) float32 {
	if a.Mode == ModeStudio {
		if a.StudioLeftChatWidth > 0 {
			return a.StudioLeftChatWidth
		}
		return 420
	}
	w, _ := c.Size()
	if w <= 0 {
		return 800
	}
	if a.ShowLeftSidebar {
		w -= a.LeftSidebarWidth
	}
	if a.ShowRightDrawer {
		w -= a.RightDrawerWidth
		w -= 4 // resize handle
	}
	if a.Mode == ModeAgent && a.AgentSplitSessionID != "" && !a.IsGitClientOpen && a.AgentViewMode == AgentViewChat {
		if a.AgentSplitWidth > 0 && a.AgentSplitWidth < w {
			w = a.AgentSplitWidth
		} else {
			w = w / 2
		}
	}
	if w < 320 {
		w = 320
	}
	return w
}

// messagePlainText flattens a message for the clipboard: text + thinking
// label + per-tool lines, as forge-ade's copy-message action does.
func messagePlainText(m *agent.ChatMessage) string {
	var b strings.Builder
	b.WriteString(m.Text)
	for _, t := range m.Tools {
		b.WriteString("\n[" + t.ToolName + "]")
		if cmd := strings.TrimSpace(t.Command); cmd != "" {
			b.WriteString(" " + strings.ReplaceAll(cmd, "\n", " ⏎ "))
		}
		if out := strings.TrimSpace(t.Output); out != "" {
			b.WriteString("\n" + out)
		}
	}
	return strings.TrimSpace(b.String())
}
