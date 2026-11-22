package ui

// browser_viewer.go ports the right-drawer BrowserViewer tab: a native
// WKWebView-backed browser pane with URL bar, back/forward/reload and zoom —
// BrowserViewer.tsx parity, using MyGo's ui.WebView widget. Also hosts the
// Child Agents (subagent) pane reading agent.Manager.GetSubagents.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
)

// openBrowserTab creates the native web view (singleton, as the webview's
// browser tab) and activates the Browser drawer tab.
func (a *AppUI) openBrowserTab(startURL string) {
	if a.BrowserWebView != nil && !a.BrowserWebView.IsDestroyed() {
		a.RightDrawerTab = RightTabBrowser
		a.ShowRightDrawer = true
		if startURL != "" {
			a.navigateBrowser(startURL)
		}
		return
	}
	if a.Window == nil || !mygo.WebViewAvailable() {
		if a.Window != nil {
			a.Window.Invalidate()
		}
		return
	}
	wv, err := a.Window.NewWebView(mygo.WebViewOptions{URL: startURL})
	if err != nil {
		return
	}
	a.BrowserWebView = wv
	a.BrowserURLCurrent = startURL
	a.BrowserURLInput = startURL
	a.BrowserZoom = 1.0
	a.RightDrawerTab = RightTabBrowser
	a.ShowRightDrawer = true
	if a.Window != nil {
		a.Window.Invalidate()
	}
}

// closeBrowserTab destroys the web view and leaves the tab.
func (a *AppUI) closeBrowserTab() {
	if a.BrowserWebView != nil && !a.BrowserWebView.IsDestroyed() {
		a.BrowserWebView.Destroy()
	}
	a.BrowserWebView = nil
	if a.RightDrawerTab == RightTabBrowser {
		a.RightDrawerTab = RightTabChanges
	}
	if a.Window != nil {
		a.Window.Invalidate()
	}
}

// navigateBrowser loads a URL, defaulting the scheme like a browser bar does.
func (a *AppUI) navigateBrowser(raw string) {
	if a.BrowserWebView == nil || a.BrowserWebView.IsDestroyed() {
		return
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return
	}
	if !strings.Contains(raw, "://") && !strings.HasPrefix(raw, "about:") {
		if strings.Contains(raw, " ") || !strings.Contains(raw, ".") {
			raw = "https://duckduckgo.com/?q=" + strings.ReplaceAll(raw, " ", "+")
		} else {
			raw = "https://" + raw
		}
	}
	if err := a.BrowserWebView.Page().LoadURL(raw); err == nil {
		a.BrowserURLCurrent = raw
		a.BrowserURLInput = raw
	}
	if a.Window != nil {
		a.Window.Invalidate()
	}
}

// renderBrowserPane draws the browser tab: toolbar + native web view.
func (a *AppUI) renderBrowserPane(c *ui.Context) {
	tc := a.ThemeMgr.Colors()
	if a.BrowserWebView == nil || a.BrowserWebView.IsDestroyed() {
		ui.Column(c).Grow(1).Center().Gap(10).Padding(20).Children(func() {
			ui.Icon(c, icons.TablerWorld).Width(32).Height(32).TextColor(tc.IconMuted)
			ui.Text(c, "Browser viewer").FontSize(13).FontWeight(600).TextColor(tc.TextMuted)
			ui.Text(c, "Native web view for pages and previews the agent opens.").FontSize(12).TextColor(tc.TextMuted).MaxWidth(240)
			if ContrastCTA(c, tc, "Open browser").Clicked() {
				a.openBrowserTab("")
			}
		})
		return
	}

	page := a.BrowserWebView.Page()

	// The web engine exposes no navigation-changed callback, but it does
	// expose the current URL and history state: sync the address bar (links,
	// redirects) and gate back/forward on real history every frame.
	if cur := page.URL(); cur != "" && cur != a.BrowserURLCurrent {
		a.BrowserURLCurrent = cur
		if a.BrowserURLInput != cur {
			a.BrowserURLInput = cur
		}
	}
	canGoBack := page.CanGoBack()
	canGoForward := page.CanGoForward()

	// Toolbar: back / forward / reload, URL bar, zoom controls.
	ui.Row(c).Height(36).Padding(0, 8).Gap(5).AlignItems(ui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
		backBtn := IconButton(c, tc, icons.TablerArrowLeft, 13)
		if canGoBack {
			if backBtn.Clicked() {
				page.GoBack()
			}
		} else {
			backBtn.Tooltip("No previous page")
		}
		fwdBtn := IconButton(c, tc, icons.TablerArrowRight, 13)
		if canGoForward {
			if fwdBtn.Clicked() {
				page.GoForward()
			}
		} else {
			fwdBtn.Tooltip("No next page")
		}
		if IconButton(c, tc, icons.TablerRefresh, 13).Clicked() {
			page.Reload()
		}
		urlBox := ui.Row(c).Height(26).Radius(6).Padding(0, 8).Gap(6).AlignItems(ui.Center).
			Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).Grow(1)
		urlBox.Children(func() {
			ui.Icon(c, icons.TablerLock).Width(11).Height(11).TextColor(tc.IconMuted)
			urlIn := ui.TextInput(c, &a.BrowserURLInput).Placeholder("Search or enter address").Grow(1)
			if urlIn.Submitted() {
				a.navigateBrowser(a.BrowserURLInput)
			}
		})
		if IconButton(c, tc, icons.TablerZoomOut, 13).Clicked() {
			a.BrowserZoom = clampZoom(a.BrowserZoom - 0.1)
			a.BrowserWebView.Page().SetZoomFactor(a.BrowserZoom)
		}
		if IconButton(c, tc, icons.TablerZoomIn, 13).Clicked() {
			a.BrowserZoom = clampZoom(a.BrowserZoom + 0.1)
			a.BrowserWebView.Page().SetZoomFactor(a.BrowserZoom)
		}
		close := IconButton(c, tc, icons.TablerX, 13)
		close.ContextMenu(func(m *ui.Menu) {
			if m.Item("Close browser tab").Chosen() {
				a.closeBrowserTab()
			}
		})
		if close.Clicked() {
			a.closeBrowserTab()
		}
	})

	// The native web view fills the pane.
	ui.WebView(c, a.BrowserWebView).Fill()
}

func clampZoom(z float64) float64 {
	if z < 0.3 {
		return 0.3
	}
	if z > 3.0 {
		return 3.0
	}
	return z
}

// subagentSnapshot is a render-local copy of the displayed fields of one
// engine subagent session. Agent.Subagents → Manager.GetSubagents releases
// the engine's read lock before returning live *Session pointers, and LLM
// goroutines keep mutating those sessions mid-turn (appending streaming
// message content, updating State and PendingTools), so the render must
// never touch the live sessions: it would race the writers for a whole
// frame and can crash on a torn slice header or a map iterated during a
// concurrent write. snapshotSubagents copies everything the pane displays
// in one tight, bounds-checked pass immediately after the locked accessor
// returns. That copy pass itself still runs just outside the engine's lock
// (no wider lock is reachable from the ui package) — the residual race
// window is one bounded pass per frame, versus unbounded whole-frame
// iteration of live state before; every access below is length-captured,
// bounds-checked and nil-defensive.
type subagentSnapshot struct {
	id           string
	name         string
	roleFilter   string
	state        string
	model        string
	messages     []subagentMessageSnapshot
	pendingTools []subagentPendingToolSnapshot
}

// subagentMessageSnapshot holds the message fields the chat view renders,
// flattened at copy time (text/thinking concatenated, tool-call names
// collected — mirroring AgentMessage.Text/Thinking/ToolCallBlocks without
// re-reading the live slice once per call).
type subagentMessageSnapshot struct {
	role      string
	timestamp time.Time
	thinking  string
	text      string
	toolCalls []string
	// lastTool/lastToolDetail feed the parent-side rolling activity line
	// ("Reading src/main.go"): the newest tool_call's name plus a defensive
	// copy of its most useful argument (path / command / query).
	lastTool       string
	lastToolDetail string
}

// subagentPendingToolSnapshot is one pending approval request with its
// arguments JSON-marshaled at copy time from a defensively key-copied map,
// so the render never iterates a live session map.
type subagentPendingToolSnapshot struct {
	name      string
	arguments string
}

// snapshotSubagents copies the displayed state of a session's subagents out
// of the live engine sessions. See subagentSnapshot for the concurrency
// rationale.
func (a *AppUI) snapshotSubagents(sessionID string) []subagentSnapshot {
	if a.Agent == nil {
		return nil
	}
	live := a.Agent.Subagents(sessionID)
	if len(live) == 0 {
		return nil
	}
	out := make([]subagentSnapshot, 0, len(live))
	for _, sub := range live {
		if sub == nil {
			continue
		}
		snap := subagentSnapshot{
			id:         sub.ID,
			name:       sub.Name,
			roleFilter: string(sub.RoleFilter),
			state:      fmt.Sprintf("%v", sub.State),
			model:      sub.Model,
		}

		// Messages: capture the length once and bounds-check every element;
		// a mid-turn append can grow the slice while we copy.
		n := len(sub.Messages)
		for i := 0; i < n; i++ {
			if i >= len(sub.Messages) {
				break
			}
			msg := sub.Messages[i]
			sm := subagentMessageSnapshot{role: msg.Role, timestamp: msg.Timestamp}
			blocks := msg.Content
			bn := len(blocks)
			for j := 0; j < bn; j++ {
				if j >= len(blocks) {
					break
				}
				b := blocks[j]
				switch b.Type {
				case "text":
					sm.text += b.Text
				case "thinking":
					sm.thinking += b.Text
				case "tool_call":
					sm.toolCalls = append(sm.toolCalls, b.Name)
					sm.lastTool = b.Name
					sm.lastToolDetail = summarizeToolArgsCopy(b.Arguments)
				}
			}
			snap.messages = append(snap.messages, sm)
		}

		// Pending tool approvals: bounded copy pass; JSON-encode a key-copy
		// of each arguments map so render never marshals a live map.
		pn := len(sub.PendingTools)
		for i := 0; i < pn; i++ {
			if i >= len(sub.PendingTools) {
				break
			}
			pt := sub.PendingTools[i]
			pts := subagentPendingToolSnapshot{name: pt.Name}
			if len(pt.Arguments) > 0 {
				argsCopy := make(map[string]interface{}, len(pt.Arguments))
				for k, v := range pt.Arguments {
					argsCopy[k] = v
				}
				if raw, err := json.Marshal(argsCopy); err == nil {
					pts.arguments = string(raw)
				}
			}
			snap.pendingTools = append(snap.pendingTools, pts)
		}

		out = append(out, snap)
	}
	return out
}

// summarizeToolArgsCopy defensively copies a tool-call arguments map (same
// bounded pass as the PendingTools copy) and returns the most useful single
// value — path, command, or query — for the parent-side activity line.
// Reading values from a live racing map is never done; only the bounded
// key-copy is touched.
func summarizeToolArgsCopy(args map[string]any) string {
	if len(args) == 0 {
		return ""
	}
	copied := make(map[string]string, 16)
	n := 0
	for k, v := range args {
		if n >= 16 {
			break
		}
		n++
		if s, ok := v.(string); ok {
			copied[k] = s
		}
	}
	for _, keys := range [][]string{
		{"path", "file_path", "filePath", "file"},
		{"command", "cmd", "script"},
		{"query", "pattern", "url"},
	} {
		for _, k := range keys {
			if s := copied[k]; s != "" {
				return clampRunes(s, 56)
			}
		}
	}
	return ""
}

// renderSubagentPane lists the child agents of the active session
// (GetSubagents), SubagentChatPane parity.
func (a *AppUI) renderSubagentPane(c *ui.Context) {
	tc := a.ThemeMgr.Colors()

	sessionID := ""
	var subs []subagentSnapshot
	if a.Agent != nil {
		// The nil guard covers every Agent access below; the ID read goes
		// through the locked accessor.
		sessionID = a.Agent.SnapshotActiveSessionID()
		subs = a.snapshotSubagents(sessionID)
	}

	// If a specific subagent is selected, display its chat/steering view
	if a.SelectedSubagentID != "" {
		var activeSub *subagentSnapshot
		for i := range subs {
			if subs[i].id == a.SelectedSubagentID {
				activeSub = &subs[i]
				break
			}
		}
		if activeSub != nil {
			a.renderSubagentChatView(c, activeSub)
			return
		}
		a.SelectedSubagentID = ""
	}

	ui.Row(c).Height(36).Padding(0, 12).Gap(6).AlignItems(ui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
		ui.Icon(c, icons.TablerBot).Width(14).Height(14).TextColor(tc.Accent)
		ui.Text(c, "Child Agents").FontSize(12).FontWeight(600)
		ui.Box(c).Grow(1)
		if IconButton(c, tc, icons.TablerRefresh, 12).Clicked() && a.Window != nil {
			a.Window.Invalidate()
		}
	})

	if len(subs) == 0 {
		ui.Column(c).Grow(1).Center().Gap(8).Padding(20).Children(func() {
			ui.Icon(c, icons.TablerBot).Width(30).Height(30).TextColor(tc.IconMuted)
			ui.Text(c, "No child agents").FontSize(13).FontWeight(600).TextColor(tc.TextMuted)
			ui.Text(c, "Subagents the agent spawns for delegated tasks appear here.").FontSize(12).TextColor(tc.TextMuted).MaxWidth(240)
		})
		return
	}

	scroll := ui.Scroll(c)
	scroll.Children(func() {
		col := ui.Column(c).Gap(2).Padding(8)
		col.Children(func() {
			for i := range subs {
				sub := subs[i]
				// ZCode SubagentDirectory row: face tile + name + role badge,
				// a one-line summary of the latest activity, and a relative
				// timestamp on the trailing edge.
				row := ui.Row(c.Key("sub-"+sub.id)).Radius(8).Padding(8, 10).Gap(9).
					AlignItems(ui.Center).Cursor(ui.CursorPointer)
				row.Children(func() {
					faceStatus := subagentFaceState(sub.state)
					ui.Box(c).Size(20, 20).Children(func() {
						agentFace(c, tc, sub.name, faceStatus)
					})
					colRow := ui.Column(c).Gap(2).Grow(1)
					colRow.Children(func() {
						ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
							ui.Text(c, sub.name).FontSize(12).FontWeight(500).SingleLine()
							if sub.roleFilter != "" {
								Badge(c, tc, sub.roleFilter, tc.Info)
							}
						})
						ui.Text(c, subagentDirectorySummary(&sub)).FontSize(10.5).TextColor(tc.TextMuted).SingleLine()
					})
					ui.Text(c, subagentRelativeTime(&sub)).FontSize(10).TextColor(tc.IconMuted)
				})
				if row.Hovered() {
					row.Background(tc.Text.Alpha(0.05))
				}
				if row.Clicked() {
					a.SelectedSubagentID = sub.id
					if a.Window != nil {
						a.Window.Invalidate()
					}
				}
			}
		})
	}).Grow(1)
}

// subagentDirectorySummary returns the latest activity line for a directory
// row: newest tool activity or assistant text, falling back to a message
// count.
func subagentDirectorySummary(sub *subagentSnapshot) string {
	if line := subagentLiveSummary(sub); line != "Working…" {
		return line
	}
	if len(sub.messages) > 0 {
		return fmt.Sprintf("%d messages", len(sub.messages))
	}
	return "No activity yet"
}

// subagentRelativeTime renders the newest message timestamp as "3m ago".
func subagentRelativeTime(sub *subagentSnapshot) string {
	for i := len(sub.messages) - 1; i >= 0; i-- {
		ts := sub.messages[i].timestamp
		if ts.IsZero() {
			continue
		}
		d := time.Since(ts)
		switch {
		case d < time.Minute:
			return "just now"
		case d < time.Hour:
			return fmt.Sprintf("%dm ago", int(d.Minutes()))
		case d < 24*time.Hour:
			return fmt.Sprintf("%dh ago", int(d.Hours()))
		default:
			return fmt.Sprintf("%dd ago", int(d.Hours()/24))
		}
	}
	return ""
}

// renderSubagentChatView renders the detailed subagent conversation,
// thoughts, tool calls, and steering input — entirely from the per-frame
// snapshot; no live engine session is touched after the copy pass.
func (a *AppUI) renderSubagentChatView(c *ui.Context, sub *subagentSnapshot) {
	tc := a.ThemeMgr.Colors()

	// Top bar: back button, name, status dot, model badge, and stop turn button
	statusLabel, statusColor := subagentStatus(sub.state, tc)
	ui.Row(c).Height(36).Padding(0, 8).Gap(6).AlignItems(ui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
		backBtn := ui.Row(c).Height(24).Radius(4).Padding(0, 6).Gap(4).AlignItems(ui.Center).
			Background(tc.SurfaceRaised.Alpha(0.6)).Cursor(ui.CursorPointer)
		backBtn.Children(func() {
			ui.Icon(c, icons.TablerArrowLeft).Width(12).Height(12).TextColor(tc.TextMuted)
			ui.Text(c, "All agents").FontSize(11).TextColor(tc.TextMuted)
		})
		if backBtn.Hovered() {
			backBtn.Background(tc.SurfaceRaised)
		}
		if backBtn.Clicked() {
			a.SelectedSubagentID = ""
			if a.Window != nil {
				a.Window.Invalidate()
			}
			return
		}

		ui.Box(c).Size(20, 20).Children(func() {
			agentFace(c, tc, sub.name, subagentFaceState(sub.state))
		})
		nameText := sub.name
		if nameText == "" {
			nameText = "Child Agent"
		}
		ui.Text(c, nameText).FontSize(12).FontWeight(600).SingleLine().Grow(1)
		if sub.roleFilter != "" {
			Badge(c, tc, sub.roleFilter, tc.Info)
		}
		ui.Text(c, statusLabel).FontSize(10).TextColor(statusColor)

		if sub.model != "" {
			ui.Row(c).Height(18).Radius(3).Padding(0, 5).AlignItems(ui.Center).
				Background(tc.Border.Alpha(0.3)).Children(func() {
				ui.Text(c, sub.model).FontSize(9).TextColor(tc.TextMuted).SingleLine()
			})
		}

		isWorking := sub.state == "running" || sub.state == "working"
		if isWorking {
			stopBtn := ui.Row(c).Height(22).Radius(4).Padding(0, 6).Gap(4).AlignItems(ui.Center).
				Background(tc.Error.Alpha(0.15)).BorderWidth(1).BorderColor(tc.Error.Alpha(0.3)).Cursor(ui.CursorPointer)
			stopBtn.Children(func() {
				ui.Icon(c, icons.TablerPlayerStop).Width(10).Height(10).TextColor(tc.Error)
				ui.Text(c, "Stop").FontSize(10).FontWeight(600).TextColor(tc.Error)
			})
			if stopBtn.Clicked() && a.Engine != nil && a.Engine.AgentMgr != nil {
				a.Engine.AgentMgr.StopTurn(sub.id)
				if a.Window != nil {
					a.Window.Invalidate()
				}
			}
		}
	})

	ui.Column(c).Grow(1).Children(func() {
		// Tool approval request if pending
		if len(sub.pendingTools) > 0 {
			ui.Column(c).Padding(8, 10).Gap(6).Background(tc.Warning.Alpha(0.1)).
				BorderWidth(0, 0, 1, 0).BorderColor(tc.Warning.Alpha(0.3)).Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icons.TablerAlertTriangle).Width(13).Height(13).TextColor(tc.Warning)
					ui.Text(c, "Action requires approval").FontSize(11).FontWeight(600).TextColor(tc.Warning)
				})
				for _, pt := range sub.pendingTools {
					ptName := pt.name
					if ptName == "" {
						ptName = "Tool Action"
					}
					ui.Text(c, ptName).FontSize(11).FontWeight(500)
					if pt.arguments != "" {
						ui.Text(c, pt.arguments).FontSize(10).TextColor(tc.TextMuted).SingleLine()
					}
				}
				ui.Row(c).Gap(8).Children(func() {
					appr := ContrastCTA(c, tc, "Approve")
					if appr.Clicked() && a.Engine != nil && a.Engine.AgentMgr != nil {
						go func() {
							_ = a.Engine.AgentMgr.RespondApproval(context.Background(), sub.id, true, false)
							if a.Window != nil {
								a.Window.Invalidate()
							}
						}()
					}
					deny := ui.Button(c, "Deny")
					if deny.Clicked() && a.Engine != nil && a.Engine.AgentMgr != nil {
						go func() {
							_ = a.Engine.AgentMgr.RespondApproval(context.Background(), sub.id, false, false)
							if a.Window != nil {
								a.Window.Invalidate()
							}
						}()
					}
				})
			})
		}

		// Message history
		scroll := ui.Scroll(c)
		scroll.Children(func() {
			ui.Column(c).Gap(10).Padding(10).Children(func() {
				if len(sub.messages) == 0 {
					ui.Column(c).Center().Padding(20).Gap(6).Children(func() {
						ui.Icon(c, icons.TablerMessage).Width(24).Height(24).TextColor(tc.IconMuted)
						ui.Text(c, "No messages yet").FontSize(12).TextColor(tc.TextMuted)
					})
				}
				for i, msg := range sub.messages {
					msgKey := fmt.Sprintf("submsg-%d", i)
					isUser := msg.role == "user"
					if isUser {
						// User prompts: right-aligned bubble like the main chat.
						ui.Row(c.Key(msgKey)).Justify(ui.End).Children(func() {
							bubble := ui.Column(c).MaxWidth(420).Radius(14).Padding(8, 12).Gap(4).
								Background(tc.MessageSurface).BorderWidth(1).BorderColor(tc.Border.Alpha(0.5))
							bubble.Children(func() {
								ui.Text(c, msg.text).FontSize(12).TextColor(tc.MessageForeground)
							})
						})
						continue
					}
					// Assistant: ZCode-style column — reasoning behind a guide
					// rail, tool activity as summary lines, markdown body.
					assist := ui.Column(c.Key(msgKey)).Gap(4).Padding(2, 0)
					assist.Children(func() {
						if msg.thinking != "" {
							ui.Column(c).Padding(0, 0, 0, 10).BorderWidth(0, 0, 0, 1).
								BorderColor(tc.Border.Alpha(0.6)).Children(func() {
								ui.Text(c, clampRunes(msg.thinking, 600)).FontSize(10.5).TextColor(tc.TextMuted)
							})
						}
						for ti, toolName := range msg.toolCalls {
							detail := ""
							if ti == len(msg.toolCalls)-1 {
								detail = msg.lastToolDetail
							}
							toolRow := ui.Row(c).Height(20).Padding(0, 2).Gap(6).AlignItems(ui.Center)
							toolRow.Children(func() {
								ui.Text(c, agentActivityLabel(toolName, detail)).FontSize(11).
									TextColor(tc.TextMuted).Grow(1).SingleLine()
							})
						}
						txt := strings.TrimSpace(msg.text)
						if txt != "" {
							a.SideChatMd.Theme = a.ThemeMgr.MarkdownTheme()
							markdown.View(c, txt, &a.SideChatMd)
						}
					})
				}
			})
		}).Grow(1)

		// Steering bottom bar
		ui.Row(c).Height(40).Padding(6, 8).Gap(6).AlignItems(ui.Center).
			BorderWidth(1, 0, 0, 0).BorderColor(tc.Border).Background(tc.Surface).Children(func() {
			input := ui.TextInput(c, &a.SubagentChatInput).Placeholder("Steer or message subagent...").Grow(1)
			sendAction := func() {
				text := strings.TrimSpace(a.SubagentChatInput)
				if text == "" || a.Engine == nil || a.Engine.AgentMgr == nil {
					return
				}
				a.SubagentChatInput = ""
				// Working state from the frame snapshot: consistent with
				// what the user saw when they hit Send.
				working := sub.state == "running" || sub.state == "working"
				go func() {
					if working {
						if err := a.Engine.AgentMgr.SteerMessage(sub.id, text); err != nil {
							_ = a.Engine.AgentMgr.SendMessage(context.Background(), sub.id, text, nil)
						}
					} else {
						_ = a.Engine.AgentMgr.SendMessage(context.Background(), sub.id, text, nil)
					}
					if a.Window != nil {
						a.Window.Invalidate()
					}
				}()
			}
			if input.Submitted() {
				sendAction()
			}
			sendBtn := ContrastCTA(c, tc, "Send")
			if sendBtn.Clicked() {
				sendAction()
			}
		})
	})
}

// subagentStatus maps the engine's session state (already stringified into
// the snapshot) to a label + color.
func subagentStatus(state string, tc theme.ThemeColors) (string, ui.Color) {
	switch state {
	case "running", "working":
		return "Running", tc.Accent
	case "error":
		return "Error", tc.Error
	case "completed", "done":
		return "Completed", tc.Success
	default:
		return "Idle", tc.SidebarMutedForeground
	}
}
