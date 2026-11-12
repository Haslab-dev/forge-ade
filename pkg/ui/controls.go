package ui

import (
	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/icons"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/theme"
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
	Key      string
	Title    string
	Subtitle string
	Status   string
	Active   bool
	Pinned   bool
	Archived bool
	OnPin    func()
	OnDelete func()
	OnOpen   func()
}

// renderTaskRow draws AgentSidebar's TaskRow: rounded-[8px] py-1.5 pl-2.5
// pr-2 gap-2, active bg-foreground/10, provider dot, title + subtitle,
// hover pin/delete actions.
func renderTaskRow(c *ui.Context, tc theme.ThemeColors, d taskRowData) {
	row := ui.Row(c.Key(d.Key)).Radius(8).Padding(6, 8, 6, 10).Gap(8).AlignItems(ui.Center)
	row.Children(func() {
		StatusDot(c, tc, d.Status)
		ui.Column(c).Grow(1).Gap(1).Children(func() {
			title := ui.Text(c, d.Title).FontSize(13).SingleLine()
			if d.Active {
				title.FontWeight(500)
			}
			if d.Subtitle != "" {
				ui.Text(c, d.Subtitle).FontSize(11).TextColor(tc.SidebarMutedForeground).SingleLine()
			}
		})
		if d.Pinned {
			ui.Icon(c, icons.TablerPinFilled).Width(13).Height(13).TextColor(tc.Accent)
		}
		if d.OnPin != nil {
			if IconButton(c, tc, icons.TablerPin, 12).Clicked() {
				d.OnPin()
			}
		}
		if d.OnDelete != nil {
			if IconButton(c, tc, icons.TablerTrash, 12).Clicked() {
				d.OnDelete()
			}
		}
	})
	if d.Active {
		row.Background(tc.Text.Alpha(0.1))
	} else if row.Hovered() {
		row.Background(tc.SidebarRowHover)
	}
	if row.Clicked() && d.OnOpen != nil {
		d.OnOpen()
	}
	if d.Archived {
		row.Opacity(0.65)
	}
}
