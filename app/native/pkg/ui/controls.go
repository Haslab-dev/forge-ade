package ui

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
)

// Shared controls replicating ForgeADE's Tailwind recipes at their exact
// metrics. All measurements are the px values of the original CSS
// (text-ui-xs 12, text-ui-sm 13, text-ui-base 14, h-7 28, h-7.5 30,
// size-7 28, rounded-md 6, rounded-lg 8, rounded-xl 12, rounded-2xl 16,
// --radius 10, --composer-radius 22).

// TablerIcon draws a Tabler SVG icon scaled to the target dimension.
func TablerIcon(c *ui.Context, svg *ui.SVG, size float32) {
	ui.Icon(c, svg).Width(size).Height(size)
}

// IconButton is WorkspaceHeader's size-7 rounded-md icon button: a 28×28
// hit area whose icon tints muted and brightens on hover, with an optional
// pressed/active look.
func IconButton(c *ui.Context, tc theme.ThemeColors, icon *ui.SVG, iconSize float32) ui.Element {
	row := ui.Row(c).Size(28, 28).Radius(6).AlignItems(ui.Center).Justify(ui.Center)
	row.Children(func() {
		ui.Icon(c, icon).Width(iconSize).Height(iconSize)
	})
	if row.Hovered() {
		row.Background(tc.ToolbarControlHover)
	}
	return row
}

// ToggleIconButton is an IconButton with an active (toggled) state, styled
// as the original's pressed controls (bg-surface-hover text-foreground).
func ToggleIconButton(c *ui.Context, tc theme.ThemeColors, icon *ui.SVG, iconSize float32, active bool) ui.Element {
	row := ui.Row(c).Size(28, 28).Radius(6).AlignItems(ui.Center).Justify(ui.Center)
	row.Children(func() {
		ic := ui.Icon(c, icon).Width(iconSize).Height(iconSize)
		if !active {
			ic.TextColor(tc.IconMuted)
		}
	})
	if active {
		row.Background(tc.ToolbarControlHover)
	} else if row.Hovered() {
		row.Background(tc.ToolbarControlHover)
	}
	return row
}

// SidebarActionButton is AgentSidebar's action row: h-7.5 (30px),
// rounded-[6px], px-2.5, text-ui-sm medium, hover bg-foreground/5, with an
// optional kbd hint at the trailing edge.
func SidebarActionButton(c *ui.Context, tc theme.ThemeColors, icon *ui.SVG, label, kbd string, active bool) ui.Element {
	row := ui.Row(c.Key("sb-btn-"+label)).Height(30).Radius(6).Padding(0, 10).Gap(10).AlignItems(ui.Center).Cursor(ui.CursorPointer)
	row.Children(func() {
		ic := ui.Icon(c, icon).Width(15).Height(15)
		if active {
			ic.TextColor(tc.Text)
		} else {
			ic.TextColor(tc.IconMuted)
		}
		ui.Text(c, label).FontSize(13).FontWeight(500)
		ui.Box(c).Grow(1)
		if kbd != "" {
			ui.Text(c, kbd).FontSize(12).TextColor(tc.SecondaryLabel)
		}
	})
	if active {
		row.Background(tc.Text.Alpha(0.08))
	} else if row.Hovered() {
		row.Background(tc.Text.Alpha(0.05))
	}
	return row
}

// SidebarDangerActionButton renders a danger/delete action row with red accents.
func SidebarDangerActionButton(c *ui.Context, tc theme.ThemeColors, icon *ui.SVG, label string) ui.Element {
	row := ui.Row(c.Key("sb-danger-"+label)).Height(30).Radius(6).Padding(0, 10).Gap(10).AlignItems(ui.Center).Cursor(ui.CursorPointer)
	labelColor := tc.Error
	row.Children(func() {
		ui.Icon(c, icon).Width(15).Height(15).TextColor(labelColor)
		ui.Text(c, label).FontSize(13).FontWeight(500).TextColor(labelColor)
	})
	if row.Hovered() {
		row.Background(tc.Error.Alpha(0.1))
	}
	return row
}

// SectionLabel renders the sidebar section headers: 11px semibold uppercase
// with wide tracking (text-[11px] font-semibold uppercase tracking-wider).
func SectionLabel(c *ui.Context, tc theme.ThemeColors, label string) {
	ui.Text(c, label).FontSize(11).FontWeight(600).TextColor(tc.SidebarMutedForeground).LetterSpacing(0.5)
}

// StatusDot is the task-row status dot: a small filled circle with the
// session state's color (running primary, completed subtle, idle muted).
func StatusDot(c *ui.Context, tc theme.ThemeColors, status string) {
	color := tc.SidebarMutedForeground
	switch status {
	case "running", "working":
		color = tc.Accent
	case "completed", "done":
		color = tc.Success
	case "awaiting-approval", "waiting":
		color = tc.Warning
	case "error", "failed":
		color = tc.Error
	case "idle":
		color = tc.SidebarMutedForeground
	}
	ui.Box(c).Size(7, 7).Radius(4).Background(color)
}

// DiffStatText renders the "+N −N" mono diff summary of a task row or file
// row (text-[10px] font-mono with diff-addition/deletion colors).
func DiffStatText(c *ui.Context, tc theme.ThemeColors, additions, deletions int) {
	if additions == 0 && deletions == 0 {
		return
	}
	ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
		if additions > 0 {
			ui.Text(c, formatDiffCount("+", additions)).FontSize(10).TextColor(tc.DiffAdditionForeground)
		}
		if deletions > 0 {
			ui.Text(c, formatDiffCount("−", deletions)).FontSize(10).TextColor(tc.DiffDeletionForeground)
		}
	})
}

// Chip renders the file/context pills: rounded-full px-2.5 py-1 text-ui-xs
// mono (composer file attachments, changed-files dock).
func Chip(c *ui.Context, tc theme.ThemeColors, label string) ui.Element {
	row := ui.Row(c).Radius(999).Padding(4, 10).AlignItems(ui.Center).
		Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
	row.Children(func() {
		ui.Text(c, label).FontSize(12)
	})
	return row
}

// ContrastCTA renders ForgeADE's primary action recipe: bg-foreground
// text-background ("Accept All", DatabaseViewer CTAs).
func ContrastCTA(c *ui.Context, tc theme.ThemeColors, label string) ui.Element {
	btn := ui.Row(c).Radius(6).Padding(4, 10).AlignItems(ui.Center).Justify(ui.Center).
		Background(tc.Text)
	btn.Children(func() {
		ui.Text(c, label).FontSize(12).FontWeight(500).TextColor(tc.Canvas)
	})
	if btn.Hovered() {
		btn.Opacity(0.9)
	}
	return btn
}

// formatDiffCount avoids a fmt import in hot paths: "+123".
func formatDiffCount(sign string, n int) string {
	if n == 0 {
		return ""
	}
	neg := ""
	if n < 0 {
		neg = "-"
		n = -n
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	if digits == "" {
		digits = "0"
	}
	return sign + neg + digits
}

// KbdText renders the keyboard hint chips (⌘N style): text-ui-xs mono subtle.
func KbdText(c *ui.Context, tc theme.ThemeColors, keys string) {
	ui.Text(c, keys).FontSize(12).TextColor(tc.SecondaryLabel)
}

// Badge renders the tiny status chips (10px uppercase tinted), as the diff
// status badge of DiffViewer's header and staged/unstaged badges.
func Badge(c *ui.Context, tc theme.ThemeColors, label string, color ui.Color) ui.Element {
	row := ui.Row(c).Radius(4).Padding(1, 6).AlignItems(ui.Center).
		Background(color.Alpha(0.1)).BorderWidth(1).BorderColor(color.Alpha(0.25))
	row.Children(func() {
		ui.Text(c, label).FontSize(10).FontWeight(500).TextColor(color)
	})
	return row
}

// taskRowData is one AgentSidebar task session row.
type taskRowData struct {
	Key         string
	Title       string
	Status      string
	Active      bool
	Pinned      bool
	Archived    bool
	Snoozed     bool
	AgentID     string
	UpdatedTime time.Time
	OnPin       func()
	OnSnooze    func()
	OnStop      func() // terminal sessions: stop while running
	OnRestart   func() // terminal sessions: relaunch when stopped
	OnDelete    func()
	OnOpen      func()
}

// relativeTime formats a timestamp the way forge-ade's task rows do:
// "now", "5m", "3h", "20h", "4d", then a date.
func relativeTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < 2*time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	default:
		return t.Format("Jan 2")
	}
}

// providerGlyph returns the small brand glyph of an agent harness, as
// forge-ade's ProviderIcon does for the built-in agents.
func providerGlyph(agentID string) (glyph rune, color ui.Color, ok bool) {
	switch agentID {
	case "agent-agy":
		return '◆', ui.Hex("#4285f4"), true
	case "agent-claude":
		return '✳', ui.Hex("#d97757"), true
	case "agent-codex":
		return '✦', ui.Hex("#10a37f"), true
	case "agent-gemini":
		return '✦', ui.Hex("#8e44ec"), true
	default:
		return 0, ui.Color{}, false
	}
}

// renderTaskRow draws AgentSidebar's TaskRow: rounded-[8px] py-1.5 pl-2.5
// pr-2 gap-2, active bg-foreground/10, provider glyph with a status dot
// pinned to its corner (running/awaiting only — idle rows show no dot),
// single-line title with inline pin, relative time, hover-only actions.
func renderTaskRow(c *ui.Context, tc theme.ThemeColors, d taskRowData) {
	row := ui.Row(c.Key(d.Key)).Radius(8).Padding(6, 8, 6, 10).Gap(8).AlignItems(ui.Center)
	isRunning := d.Status == "running" || d.Status == "working"
	isAwaiting := d.Status == "awaiting-approval" || d.Status == "waiting"
	row.Children(func() {
		// Provider glyph with the status dot overlaid at its top-right corner,
		// as forge-ade's ProviderIcon + state badge — unknown agents fall back
		// to the ForgeADE brand glyph, so the slot always renders.
		g := ui.Box(c).Size(16, 16).AlignItems(ui.Center).Justify(ui.Center)
		g.Children(func() {
			if glyph, color, ok := providerGlyph(d.AgentID); ok {
				ui.Text(c, string(glyph)).FontSize(10).TextColor(color)
			} else {
				ui.Icon(c, icons.ForgeSparkSVG).Width(12).Height(12).TextColor(tc.Accent)
			}
			if isAwaiting {
				ui.Box(c).Absolute().Top(-3).Right(-3).Size(8, 8).Radius(4).Background(tc.Warning)
			} else if isRunning {
				ui.Box(c).Absolute().Top(-2).Right(-2).Size(6, 6).Radius(3).Background(tc.Accent)
			}
		})
		// Title with the inline filled pin, as the original's title span.
		titleRow := ui.Row(c).Grow(1).Gap(5).AlignItems(ui.Center)
		titleRow.Children(func() {
			title := ui.Text(c, d.Title).FontSize(13).SingleLine()
			if d.Active {
				title.FontWeight(500).TextColor(tc.Text)
			}
			if d.Pinned {
				ui.Icon(c, icons.TablerPinFilled).Width(10).Height(10).TextColor(tc.Accent)
			}
		})

		// Time (with running pulse dot), hidden while the hover actions show.
		if !row.Hovered() {
			timeRow := ui.Row(c).Gap(4).AlignItems(ui.Center)
			timeRow.Children(func() {
				if isRunning {
					ui.Box(c).Size(6, 6).Radius(3).Background(tc.Accent)
				}
				if ts := relativeTime(d.UpdatedTime); ts != "" {
					ui.Text(c, ts).FontSize(11).TextColor(tc.SidebarMutedForeground)
				}
			})
		}

		// Hover actions: stop (running) / restart (stopped) for terminal
		// sessions, pin (always when pinned), snooze, delete — size-5 hit
		// areas, as forge-ade's group-hover buttons.
		showActions := row.Hovered() || d.Pinned || d.Snoozed
		if showActions && isRunning && d.OnStop != nil {
			stopBtn := ui.Row(c.Key("ract-stop-"+d.Key)).Size(20, 20).Radius(4).AlignItems(ui.Center).Justify(ui.Center)
			stopBtn.Children(func() {
				ui.Icon(c, icons.TablerSquare).Width(12).Height(12).TextColor(tc.SidebarMutedForeground)
			})
			if stopBtn.Hovered() {
				stopBtn.Background(tc.Text.Alpha(0.06))
			}
			if stopBtn.Clicked() {
				d.OnStop()
			}
		}
		if showActions && !isRunning && d.OnRestart != nil {
			restartBtn := ui.Row(c.Key("ract-restart-"+d.Key)).Size(20, 20).Radius(4).AlignItems(ui.Center).Justify(ui.Center)
			restartBtn.Children(func() {
				ui.Icon(c, icons.TablerRotate).Width(12).Height(12).TextColor(tc.SidebarMutedForeground)
			})
			if restartBtn.Hovered() {
				restartBtn.Background(tc.Text.Alpha(0.06))
			}
			if restartBtn.Clicked() {
				d.OnRestart()
			}
		}
		if showActions && d.OnPin != nil {
			pinBtn := ui.Row(c.Key("ract-pin-"+d.Key)).Size(20, 20).Radius(4).AlignItems(ui.Center).Justify(ui.Center)
			pinBtn.Children(func() {
				if d.Pinned {
					ui.Icon(c, icons.TablerPinFilled).Width(12).Height(12).TextColor(tc.Accent)
				} else {
					ui.Icon(c, icons.TablerPin).Width(12).Height(12).TextColor(tc.SidebarMutedForeground)
				}
			})
			if pinBtn.Hovered() {
				pinBtn.Background(tc.Text.Alpha(0.06))
			}
			if pinBtn.Clicked() {
				d.OnPin()
			}
		}
		if showActions && d.OnSnooze != nil {
			snoozeBtn := ui.Row(c.Key("ract-sn-"+d.Key)).Size(20, 20).Radius(4).AlignItems(ui.Center).Justify(ui.Center)
			snoozeBtn.Children(func() {
				bell := ui.Icon(c, icons.TablerBellOff).Width(12).Height(12)
				if d.Snoozed {
					bell.TextColor(tc.Warning)
				} else {
					bell.TextColor(tc.SidebarMutedForeground)
				}
			})
			if snoozeBtn.Hovered() {
				snoozeBtn.Background(tc.Text.Alpha(0.06))
			}
			if snoozeBtn.Clicked() {
				d.OnSnooze()
			}
		}
		if showActions && d.OnDelete != nil {
			delBtn := ui.Row(c.Key("ract-del-"+d.Key)).Size(20, 20).Radius(4).AlignItems(ui.Center).Justify(ui.Center)
			delBtn.Children(func() {
				ui.Icon(c, icons.TablerTrash).Width(13).Height(13).TextColor(tc.SidebarMutedForeground)
			})
			if delBtn.Hovered() {
				delBtn.Background(tc.Error.Alpha(0.12))
			}
			if delBtn.Clicked() {
				d.OnDelete()
			}
		}
	})
	if d.Active {
		row.Background(tc.Text.Alpha(0.1))
	} else if row.Hovered() {
		row.Background(tc.Text.Alpha(0.1))
	}
	if row.Clicked() && d.OnOpen != nil {
		d.OnOpen()
	}
	if d.Archived {
		row.Opacity(0.65)
	}
	if d.Snoozed {
		row.Opacity(0.5)
	}

	// Right-click actions, as forge-ade's row hover menu.
	row.ContextMenu(func(m *ui.Menu) {
		if m.Item("Open").Chosen() && d.OnOpen != nil {
			d.OnOpen()
		}
		if d.OnStop != nil && isRunning {
			if m.Item("Stop Session").Chosen() {
				d.OnStop()
			}
		}
		if d.OnRestart != nil && !isRunning {
			if m.Item("Restart Session").Chosen() {
				d.OnRestart()
			}
		}
		if d.OnPin != nil {
			pinLabel := "Pin"
			if d.Pinned {
				pinLabel = "Unpin"
			}
			if m.Item(pinLabel).Chosen() {
				d.OnPin()
			}
		}
		if d.OnSnooze != nil {
			snoozeLabel := "Snooze"
			if d.Snoozed {
				snoozeLabel = "Unsnooze"
			}
			if m.Item(snoozeLabel).Chosen() {
				d.OnSnooze()
			}
		}
		m.Separator()
		if d.OnDelete != nil {
			if m.Item("Delete").Danger().Chosen() {
				d.OnDelete()
			}
		}
	})
}
