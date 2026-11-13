package ui

import (
	termplugin "github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/terminal"
)

// renderBottomTerminal renders the bottom collapsible terminal drawer.
func (a *AppUI) renderBottomTerminal(c *ui.Context) {
	tc := a.ThemeMgr.Colors()

	ui.Column(c).Height(a.BottomTerminalHeight).Background(tc.Canvas).BorderWidth(1, 0, 0, 0).BorderColor(tc.Border).Children(func() {
		// Terminal Drawer Header
		ui.Row(c).Height(TerminalHeaderHeight).Padding(0, 12).AlignItems(ui.Center).
			BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, icons.TablerTerminal).Width(14).Height(14).TextColor(tc.IconMuted)
				ui.Text(c, "Terminal").FontSize(13).FontWeight(500)
			})
			ui.Box(c).Grow(1)
			restartBtn := ui.Row(c).Radius(4).Padding(2, 8).AlignItems(ui.Center)
			restartBtn.Children(func() {
				ui.Text(c, "Restart").FontSize(12).TextColor(tc.TextMuted)
			})
			if restartBtn.Hovered() {
				restartBtn.Background(tc.Text.Alpha(0.05))
			}
			if restartBtn.Clicked() && a.Terminal != nil {
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
