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

	rakitagent "github.com/Haslab-dev/RakitKode/agent"
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

	// Toolbar: back / forward / reload, URL bar, zoom controls.
	ui.Row(c).Height(36).Padding(0, 8).Gap(5).AlignItems(ui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
		if IconButton(c, tc, icons.TablerArrowLeft, 13).Clicked() {
			a.BrowserWebView.Page().GoBack()
		}
		if IconButton(c, tc, icons.TablerArrowRight, 13).Clicked() {
			a.BrowserWebView.Page().GoForward()
		}
		if IconButton(c, tc, icons.TablerRefresh, 13).Clicked() {
			a.BrowserWebView.Page().Reload()
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

// renderSubagentPane lists the child agents of the active session
// (GetSubagents), SubagentChatPane parity.
func (a *AppUI) renderSubagentPane(c *ui.Context) {
	tc := a.ThemeMgr.Colors()

	sessionID := ""
	if a.Agent != nil {
		sessionID = a.Agent.ActiveSessionID
	}
	subs := a.Agent.Subagents(sessionID)

	// If a specific subagent is selected, display its live chat/steering view
	if a.SelectedSubagentID != "" {
		var activeSub *rakitagent.Session
		for _, s := range subs {
			if s.ID == a.SelectedSubagentID {
				activeSub = s
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
		col := ui.Column(c).Gap(4).Padding(8)
		col.Children(func() {
			for _, sub := range subs {
				sub := sub
				statusLabel, statusColor := subagentStatus(sub.State, tc)
				row := ui.Row(c.Key("sub-"+sub.ID)).Radius(8).Padding(8, 10).Gap(8).AlignItems(ui.Center).
					Background(tc.SurfaceRaised.Alpha(0.4)).BorderWidth(1).BorderColor(tc.Border.Alpha(0.5)).Cursor(ui.CursorPointer)
				row.Children(func() {
					ui.Box(c).Size(7, 7).Radius(4).Background(statusColor)
					colRow := ui.Column(c).Gap(1)
					colRow.Children(func() {
						ui.Text(c, sub.Name).FontSize(12).FontWeight(500).SingleLine()
						ui.Text(c, fmt.Sprintf("%s · %d messages", statusLabel, len(sub.Messages))).FontSize(10).TextColor(tc.TextMuted)
					})
					ui.Box(c).Grow(1)
					ui.Icon(c, icons.TablerChevronRight).Width(12).Height(12).TextColor(tc.IconMuted)
				})
				if row.Hovered() {
					row.Background(tc.Text.Alpha(0.05))
				}
				if row.Clicked() {
					a.SelectedSubagentID = sub.ID
					if a.Window != nil {
						a.Window.Invalidate()
					}
				}
			}
		})
	}).Grow(1)
}

// renderSubagentChatView renders the detailed subagent conversation, thoughts, tool calls, and steering input.
func (a *AppUI) renderSubagentChatView(c *ui.Context, sub *rakitagent.Session) {
	tc := a.ThemeMgr.Colors()

	// Top bar: back button, name, status dot, model badge, and stop turn button
	statusLabel, statusColor := subagentStatus(sub.State, tc)
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

		ui.Box(c).Size(6, 6).Radius(3).Background(statusColor)
		nameText := sub.Name
		if nameText == "" {
			nameText = "Child Agent"
		}
		ui.Text(c, nameText).FontSize(12).FontWeight(600).SingleLine().Grow(1)
		ui.Text(c, statusLabel).FontSize(10).TextColor(statusColor)

		if sub.Model != "" {
			ui.Row(c).Height(18).Radius(3).Padding(0, 5).AlignItems(ui.Center).
				Background(tc.Border.Alpha(0.3)).Children(func() {
				ui.Text(c, sub.Model).FontSize(9).TextColor(tc.TextMuted).SingleLine()
			})
		}

		isWorking := sub.State == "running" || sub.State == "working"
		if isWorking {
			stopBtn := ui.Row(c).Height(22).Radius(4).Padding(0, 6).Gap(4).AlignItems(ui.Center).
				Background(tc.Error.Alpha(0.15)).BorderWidth(1).BorderColor(tc.Error.Alpha(0.3)).Cursor(ui.CursorPointer)
			stopBtn.Children(func() {
				ui.Icon(c, icons.TablerPlayerStop).Width(10).Height(10).TextColor(tc.Error)
				ui.Text(c, "Stop").FontSize(10).FontWeight(600).TextColor(tc.Error)
			})
			if stopBtn.Clicked() && a.Engine != nil && a.Engine.AgentMgr != nil {
				a.Engine.AgentMgr.StopTurn(sub.ID)
				if a.Window != nil {
					a.Window.Invalidate()
				}
			}
		}
	})

	ui.Column(c).Grow(1).Children(func() {
		// Tool approval request if pending
		if len(sub.PendingTools) > 0 {
			ui.Column(c).Padding(8, 10).Gap(6).Background(tc.Warning.Alpha(0.1)).
				BorderWidth(0, 0, 1, 0).BorderColor(tc.Warning.Alpha(0.3)).Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icons.TablerAlertTriangle).Width(13).Height(13).TextColor(tc.Warning)
					ui.Text(c, "Action requires approval").FontSize(11).FontWeight(600).TextColor(tc.Warning)
				})
				for _, pt := range sub.PendingTools {
					ptName := pt.Name
					if ptName == "" {
						ptName = "Tool Action"
					}
					ui.Text(c, ptName).FontSize(11).FontWeight(500)
					if len(pt.Arguments) > 0 {
						if raw, err := json.Marshal(pt.Arguments); err == nil {
							ui.Text(c, string(raw)).FontSize(10).TextColor(tc.TextMuted).SingleLine()
						}
					}
				}
				ui.Row(c).Gap(8).Children(func() {
					appr := ContrastCTA(c, tc, "Approve")
					if appr.Clicked() && a.Engine != nil && a.Engine.AgentMgr != nil {
						go func() {
							_ = a.Engine.AgentMgr.RespondApproval(context.Background(), sub.ID, true, false)
							if a.Window != nil {
								a.Window.Invalidate()
							}
						}()
					}
					deny := ui.Button(c, "Deny")
					if deny.Clicked() && a.Engine != nil && a.Engine.AgentMgr != nil {
						go func() {
							_ = a.Engine.AgentMgr.RespondApproval(context.Background(), sub.ID, false, false)
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
				if len(sub.Messages) == 0 {
					ui.Column(c).Center().Padding(20).Gap(6).Children(func() {
						ui.Icon(c, icons.TablerMessage).Width(24).Height(24).TextColor(tc.IconMuted)
						ui.Text(c, "No messages yet").FontSize(12).TextColor(tc.TextMuted)
					})
				}
				for i, msg := range sub.Messages {
					msgKey := fmt.Sprintf("submsg-%d", i)
					isUser := msg.Role == "user"
					bubble := ui.Column(c.Key(msgKey)).Radius(8).Padding(8, 10).Gap(4)
					if isUser {
						bubble.Background(tc.Accent.Alpha(0.1)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.25))
					} else {
						bubble.Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border.Alpha(0.6))
					}
					bubble.Children(func() {
						ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
							roleLabel := "Assistant"
							roleColor := tc.TextMuted
							if isUser {
								roleLabel = "You"
								roleColor = tc.Accent
							}
							ui.Text(c, roleLabel).FontSize(10).FontWeight(600).TextColor(roleColor)
							if !msg.Timestamp.IsZero() {
								ui.Text(c, msg.Timestamp.Format("15:04")).FontSize(9).TextColor(tc.TextMuted)
							}
						})

						thinking := msg.Thinking()
						if thinking != "" {
							ui.Column(c).Radius(4).Padding(6).Gap(2).Background(tc.Surface.Alpha(0.6)).
								BorderWidth(1).BorderColor(tc.Border.Alpha(0.4)).Children(func() {
								ui.Text(c, "Thought Process").FontSize(9).FontWeight(600).TextColor(tc.TextMuted)
								ui.Text(c, thinking).FontSize(10).TextColor(tc.TextMuted)
							})
						}

						txt := strings.TrimSpace(msg.Text())
						if txt != "" {
							markdown.View(c, txt, &a.SideChatMd)
						}

						for _, tb := range msg.ToolCallBlocks() {
							toolRow := ui.Row(c).Height(22).Radius(4).Padding(0, 6).Gap(5).AlignItems(ui.Center).
								Background(tc.Surface.Alpha(0.8)).BorderWidth(1).BorderColor(tc.Border.Alpha(0.4))
							toolRow.Children(func() {
								ui.Icon(c, icons.TablerWrench).Width(10).Height(10).TextColor(tc.Accent)
								ui.Text(c, tb.Name).FontSize(10).FontWeight(500)
							})
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
				go func() {
					_ = a.Engine.AgentMgr.SendMessage(context.Background(), sub.ID, text, nil)
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

// subagentStatus maps the engine's session state to a label + color.
func subagentStatus(state interface{}, tc theme.ThemeColors) (string, ui.Color) {
	switch fmt.Sprintf("%v", state) {
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
