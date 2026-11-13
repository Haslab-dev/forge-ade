package ui

// browser_viewer.go ports the right-drawer BrowserViewer tab: a native
// WKWebView-backed browser pane with URL bar, back/forward/reload and zoom —
// BrowserViewer.tsx parity, using MyGo's ui.WebView widget. Also hosts the
// Child Agents (subagent) pane reading agent.Manager.GetSubagents.

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
)

// Right drawer tabs beyond the fixed set (drawer.go).
const (
	RightTabBrowser  = 9
	RightTabSubagent = 10
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

	ui.Row(c).Height(36).Padding(0, 12).Gap(6).AlignItems(ui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
		ui.Icon(c, icons.TablerBot).Width(14).Height(14).TextColor(tc.Accent)
		ui.Text(c, "Child Agents").FontSize(12).FontWeight(600)
		ui.Box(c).Grow(1)
		if IconButton(c, tc, icons.TablerRefresh, 12).Clicked() && a.Window != nil {
			a.Window.Invalidate()
		}
	})

	sessionID := ""
	if a.Agent != nil {
		sessionID = a.Agent.ActiveSessionID
	}
	subs := a.Agent.Subagents(sessionID)
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
					Background(tc.SurfaceRaised.Alpha(0.4)).BorderWidth(1).BorderColor(tc.Border.Alpha(0.5))
				row.Children(func() {
					ui.Box(c).Size(7, 7).Radius(4).Background(statusColor)
					colRow := ui.Column(c).Gap(1)
					colRow.Children(func() {
						ui.Text(c, sub.Name).FontSize(12).FontWeight(500).SingleLine()
						ui.Text(c, fmt.Sprintf("%s · %d messages", statusLabel, len(sub.Messages))).FontSize(10).TextColor(tc.TextMuted)
					})
					ui.Box(c).Grow(1)
				})
				if row.Hovered() {
					row.Background(tc.Text.Alpha(0.05))
				}
			}
		})
	}).Grow(1)
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
