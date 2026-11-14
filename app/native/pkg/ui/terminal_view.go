package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	termplugin "github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/terminal"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
)

// renderTerminalTabsStrip draws the clickable tab strip for a TerminalState.
func renderTerminalTabsStrip(c *ui.Context, tc theme.ThemeColors, ts *terminal.TerminalState, keyPrefix string) {
	if ts == nil {
		return
	}
	tabs, activeIdx := ts.SnapshotTabs()
	if len(tabs) == 0 {
		title := ts.GetTitle()
		if title == "" {
			title = "shell"
		}
		tabs = []terminal.TerminalTabSnapshot{{Index: 0, ID: 0, Title: title, Exited: ts.IsExited()}}
		activeIdx = 0
	}
	ui.ScrollHorizontal(c).Grow(1).Scrollbars(false).Children(func() {
		ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
			for _, tabSnap := range tabs {
				tabSnap := tabSnap
				isActive := tabSnap.Index == activeIdx
				title := tabSnap.Title
				if title == "" {
					title = "Terminal"
				}
				tab := ui.Row(c.Key(fmt.Sprintf("%s-tab-%d-%d", keyPrefix, tabSnap.ID, tabSnap.Index))).
					Height(24).Radius(6).Padding(0, 8).Gap(6).AlignItems(ui.Center).
					BorderWidth(1).Cursor(ui.CursorPointer)
				if isActive {
					tab.Background(tc.SurfaceRaised).BorderColor(tc.Border)
				} else {
					tab.Background(tc.Surface.Alpha(0.5)).BorderColor(tc.Border.Alpha(0.4))
					if tab.Hovered() {
						tab.Background(tc.SurfaceRaised.Alpha(0.7))
					}
				}
				closedThisTab := false
				tab.Children(func() {
					labelColor := tc.TextMuted
					if isActive {
						labelColor = tc.Text
					}
					ui.Text(c, title).FontSize(11).FontWeight(500).TextColor(labelColor).SingleLine().MaxWidth(120)
					closeTab := ui.Row(c.Key(fmt.Sprintf("%s-tab-x-%d-%d", keyPrefix, tabSnap.ID, tabSnap.Index))).
						Size(14, 14).Radius(4).AlignItems(ui.Center).Justify(ui.Center)
					closeTab.Children(func() {
						ui.Icon(c, icons.TablerX).Width(9).Height(9).TextColor(tc.IconMuted)
					})
					if closeTab.Hovered() {
						closeTab.Background(tc.Text.Alpha(0.08))
					}
					if closeTab.Clicked() {
						closedThisTab = true
						if ts.TabCount() > 0 {
							ts.CloseTab(tabSnap.Index)
						} else {
							_ = ts.Restart()
						}
					}
				})
				if !closedThisTab && tab.Clicked() && !isActive && ts.TabCount() > 0 {
					ts.SelectTab(tabSnap.Index)
				}
			}
		})
	})
}

// renderBottomTerminal renders the bottom collapsible terminal drawer.
func (a *AppUI) renderBottomTerminal(c *ui.Context) {
	tc := a.ThemeMgr.Colors()

	ui.Column(c).Height(a.BottomTerminalHeight).Background(tc.Canvas).BorderWidth(1, 0, 0, 0).BorderColor(tc.Border).Children(func() {
		// Terminal Drawer Header — ZCode dock style: bold label, shell name,
		// session tab strip, then new/close on the right.
		ui.Row(c).Height(TerminalHeaderHeight).Padding(0, 12).Gap(10).AlignItems(ui.Center).
			BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
			ui.Text(c, "Terminal").FontSize(13).FontWeight(600)
			shellName := "zsh"
			if a.Settings != nil && a.Settings.DefaultShell != "" {
				shellName = filepath.Base(a.Settings.DefaultShell)
			}
			ui.Text(c, shellName).FontSize(12).TextColor(tc.TextMuted)

			renderTerminalTabsStrip(c, tc, a.Terminal, "bottom-term")

			// Ask Agent diagnostic button (1:1 with webview BottomTerminal)
			askBtn := ui.Row(c.Key("term-ask-agent")).Height(24).Radius(5).Padding(0, 8).Gap(4).AlignItems(ui.Center).
				Background(tc.Accent.Alpha(0.12)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.35))
			askBtn.Children(func() {
				ui.Icon(c, icons.TablerSparkles).Width(11).Height(11).TextColor(tc.Accent)
				ui.Text(c, "Ask Agent").FontSize(11).FontWeight(600).TextColor(tc.Accent)
			})
			if askBtn.Clicked() {
				a.askAgentFromTerminal()
			}

			// Snippets modal button (⌥S)
			snipBtn := ui.Row(c.Key("term-snippets")).Height(24).Radius(5).Padding(0, 8).Gap(4).AlignItems(ui.Center).
				Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
			snipBtn.Children(func() {
				ui.Icon(c, icons.TablerBook).Width(11).Height(11).TextColor(tc.IconMuted)
				ui.Text(c, "Snippets").FontSize(11).TextColor(tc.Text)
			})
			if snipBtn.Clicked() {
				sessID := ""
				if a.Terminal != nil {
					sessID = a.Terminal.ActiveSessionID
				}
				a.OpenTerminalSnippetsModal(sessID, "", "")
			}

			// Clear (\x0c reset), Restart, close — ZCode terminal dock actions.
			if IconButton(c, tc, icons.TablerEraser, 13).Clicked() && a.Terminal != nil {
				a.Terminal.SendInput("\x0c")
			}
			if IconButton(c, tc, icons.TablerRefresh, 13).Clicked() && a.Terminal != nil {
				_ = a.Terminal.Restart()
			}
			if IconButton(c, tc, icons.TablerPlus, 13).Clicked() && a.Terminal != nil {
				_ = a.Terminal.NewTab("")
			}
			if IconButton(c, tc, icons.TablerX, 13).Clicked() {
				a.ShowBottomTerminal = false
			}
		})

		// Terminal Viewport
		if a.Terminal != nil {
			a.renderTerminal(c, a.Terminal)
		}
	})
}

// renderTerminal renders the Ghostty terminal viewport.
func (a *AppUI) renderTerminal(c *ui.Context, ts *terminal.TerminalState) {
	if ts == nil {
		return
	}
	t := c.Theme()

	term := ts.Term()
	if term != nil {
		termplugin.View(c, term).Fill().AutoFocus()
	} else {
		ui.Column(c).Grow(1).Center().Children(func() {
			ui.Text(c, "Terminal not available").TextColor(t.TextMuted)
			if ui.PrimaryButton(c, "Start Terminal").Clicked() {
				_ = ts.Restart()
			}
		})
	}
}

// askAgentFromTerminal captures recent output from the active terminal and sends a diagnostic prompt to the agent composer.
func (a *AppUI) askAgentFromTerminal() {
	output := ""
	if a.Terminal != nil {
		output = a.Terminal.RecentOutput(40)
	}
	if strings.TrimSpace(output) == "" {
		output = "No recent terminal output recorded."
	}
	prompt := fmt.Sprintf("Here is the recent terminal output:\n```\n%s\n```\n\nPlease diagnose any errors, explain what happened, and fix the issues.", strings.TrimSpace(output))
	a.Mode = ModeAgent
	a.AgentViewMode = AgentViewChat
	a.IsGitClientOpen = false
	a.ActiveTaskKind = "agent"
	if a.Agent != nil {
		a.Agent.InputText = prompt
	}
	if a.Window != nil {
		a.Window.Invalidate()
	}
}
