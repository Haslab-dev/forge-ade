package ui

import (
	termplugin "github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/terminal"
	"path/filepath"
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
