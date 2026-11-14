package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	termplugin "github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/terminal"
)

// renderBottomTerminal renders the bottom collapsible terminal drawer.
func (a *AppUI) renderBottomTerminal(c *ui.Context) {
	tc := a.ThemeMgr.Colors()

	ui.Column(c).Height(a.BottomTerminalHeight).Background(tc.Canvas).BorderWidth(1, 0, 0, 0).BorderColor(tc.Border).Children(func() {
		// Terminal Drawer Header — ZCode dock style: bold label, shell name,
		// session tab pill, then new/close on the right.
		ui.Row(c).Height(TerminalHeaderHeight).Padding(0, 12).Gap(10).AlignItems(ui.Center).
			BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
			ui.Text(c, "Terminal").FontSize(13).FontWeight(600)
			shellName := "zsh"
			if a.Settings != nil && a.Settings.DefaultShell != "" {
				shellName = filepath.Base(a.Settings.DefaultShell)
			}
			ui.Text(c, shellName).FontSize(12).TextColor(tc.TextMuted)

			title := "shell"
			if a.Terminal != nil {
				title = a.Terminal.Title
			}
			tab := ui.Row(c.Key("term-tab")).Height(24).Radius(6).Padding(0, 8).Gap(6).AlignItems(ui.Center).
				Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border)
			tab.Children(func() {
				ui.Text(c, title).FontSize(12).FontWeight(500)
				closeTab := ui.Row(c.Key("term-tab-x")).Size(14, 14).Radius(4).AlignItems(ui.Center).Justify(ui.Center)
				closeTab.Children(func() {
					ui.Icon(c, icons.TablerX).Width(9).Height(9).TextColor(tc.IconMuted)
				})
				if closeTab.Hovered() {
					closeTab.Background(tc.Text.Alpha(0.08))
				}
				if closeTab.Clicked() && a.Terminal != nil {
					_ = a.Terminal.Restart()
				}
			})

			ui.Box(c).Grow(1)

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
				_ = a.Terminal.Restart()
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
	if a.Agent != nil {
		a.Agent.InputText = prompt
	}
	if a.Window != nil {
		a.Window.Invalidate()
	}
}
