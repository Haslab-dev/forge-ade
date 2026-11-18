package ui

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/Haslab-dev/RakitKode/tools"
	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
	"github.com/hasdev/forge-ade/core/terminal"
)

var ansiEscapeRe = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]|\x1b\][^\x07\x1b]*(\x07|\x1b\\)`)

type bgShellEntry struct {
	ID        string
	Name      string
	Cwd       string
	Preview   string
	StartedAt time.Time
	Source    string // "tool" | "pty"
}

// activeBackgroundShells returns running background shells spawned by the
// agent (both RakitKode detached bash tasks and core PTY shell sessions).
func (a *AppUI) activeBackgroundShells(sessionID string) []bgShellEntry {
	if a.Engine == nil {
		return nil
	}
	seen := make(map[string]bool)
	var out []bgShellEntry

	if a.Engine.ToolReg != nil {
		collectToolTasks := func(sid string) {
			for _, t := range a.Engine.ToolReg.ListTasks(sid) {
				if t == nil || t.Status != tools.TaskRunning || seen[t.ID] {
					continue
				}
				seen[t.ID] = true
				name := strings.TrimSpace(t.Command)
				if name == "" {
					name = "Background Shell"
				}
				preview := t.OutputTail
				if preview == "" && t.OutputFile != "" {
					preview = readTailFile(t.OutputFile, 4096)
				}
				out = append(out, bgShellEntry{
					ID:        t.ID,
					Name:      name,
					Cwd:       t.Cwd,
					Preview:   formatMiniShellPreview(preview),
					StartedAt: t.StartedAt,
					Source:    "tool",
				})
			}
		}
		if sessionID != "" {
			collectToolTasks(sessionID)
		}
		collectToolTasks("")
	}

	if a.Engine.ShellMgr != nil {
		for _, s := range a.Engine.ShellMgr.List() {
			if s == nil || s.Type != terminal.SessionShell || s.Status != "running" {
				continue
			}
			key := "pty:" + s.ID
			if seen[key] {
				continue
			}
			seen[key] = true
			name := strings.TrimSpace(s.Command)
			if name == "" {
				name = strings.TrimSpace(s.Name)
			}
			if name == "" {
				name = "Shell " + s.ID
			}
			out = append(out, bgShellEntry{
				ID:        s.ID,
				Name:      name,
				Cwd:       s.Folder,
				Preview:   formatMiniShellPreview(s.RecentOutput(4096)),
				StartedAt: s.CreatedAt,
				Source:    "pty",
			})
		}
	}

	return out
}

func (a *AppUI) stopBackgroundShell(entry bgShellEntry) {
	if a.Engine == nil {
		return
	}
	switch entry.Source {
	case "tool":
		if a.Engine.ToolReg != nil {
			a.Engine.ToolReg.StopTask(entry.ID)
		}
	case "pty":
		if a.Engine.ShellMgr != nil {
			_ = a.Engine.ShellMgr.Stop(entry.ID)
		}
	}
}

func readTailFile(path string, maxBytes int64) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return ""
	}
	size := info.Size()
	if size <= 0 {
		return ""
	}
	offset := int64(0)
	if size > maxBytes {
		offset = size - maxBytes
	}
	buf := make([]byte, size-offset)
	n, err := f.ReadAt(buf, offset)
	if n <= 0 && err != nil {
		return ""
	}
	return string(buf[:n])
}

func formatMiniShellPreview(raw string) string {
	if raw == "" {
		return ""
	}
	clean := ansiEscapeRe.ReplaceAllString(raw, "")
	clean = strings.ReplaceAll(clean, "\r\n", "\n")
	clean = strings.ReplaceAll(clean, "\r", "\n")
	lines := strings.Split(clean, "\n")
	// Trim trailing empty lines
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > 12 {
		lines = lines[len(lines)-12:]
	}
	return strings.Join(lines, "\n")
}

// renderBgShellPill renders the active background shell indicator pill
// (terminal icon + active count) in the composer strip. Clicking it toggles
// the background shells popover modal.
func (a *AppUI) renderBgShellPill(c *ui.Context, tc theme.ThemeColors, sessionID string) {
	shells := a.activeBackgroundShells(sessionID)
	if len(shells) == 0 {
		if a.ShowBgShellPopover {
			a.ShowBgShellPopover = false
		}
		return
	}

	pill := ui.Row(c.Key("composer-bg-shell-pill")).Height(28).Radius(8).Padding(0, 8).Gap(5).
		AlignItems(ui.Center).Cursor(ui.CursorPointer).
		Background(tc.Success.Alpha(0.14)).BorderWidth(1).BorderColor(tc.Success.Alpha(0.4))
	if a.ShowBgShellPopover {
		pill.Background(tc.Success.Alpha(0.22)).BorderColor(tc.Success.Alpha(0.65))
	}
	if pill.Hovered() {
		pill.Background(tc.Success.Alpha(0.22)).BorderColor(tc.Success.Alpha(0.6))
	}
	pill.Children(func() {
		ui.Box(c).Size(6, 6).Radius(3).Background(tc.Success)
		ui.Icon(c, icons.TablerTerminal).Width(13).Height(13).TextColor(tc.Success)
		ui.Text(c, fmt.Sprintf("%d", len(shells))).FontSize(12).FontWeight(600).TextColor(tc.Success)
	})
	if pill.Clicked() {
		a.ShowBgShellPopover = !a.ShowBgShellPopover
		a.invalidateUI()
	}
}

// renderBgShellPopover renders the centered modal listing active agent
// background shells, a live mini-shell preview, and an x button to stop each
// session.
func (a *AppUI) renderBgShellPopover(c *ui.Context, tc theme.ThemeColors, sessionID string) {
	if !a.ShowBgShellPopover {
		return
	}
	shells := a.activeBackgroundShells(sessionID)
	if len(shells) == 0 {
		a.ShowBgShellPopover = false
		return
	}

	a.modal(c, &a.ShowBgShellPopover, func() {
		card := ui.Column(c.Key("bg-shell-popover")).Width(520).MaxHeight(520).Radius(14).
			Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border).
			Shadow(0, 24, 64, -24, ui.RGBA(0, 0, 0, 0.55)).Clip()
		card.Children(func() {
			// Header
			header := ui.Row(c).Height(42).Padding(0, 16).Gap(8).AlignItems(ui.Center).
				BorderWidth(0, 0, 1, 0).BorderColor(tc.Border.Alpha(0.45))
			header.Children(func() {
				ui.Icon(c, icons.TablerTerminal).Width(14).Height(14).TextColor(tc.Success)
				ui.Text(c, "Background Shells").FontSize(13).FontWeight(600).TextColor(tc.Text).Grow(1)
				ui.Text(c, fmt.Sprintf("%d running", len(shells))).FontSize(11).FontWeight(500).TextColor(tc.Success)
				closeBtn := ui.Row(c.Key("bg-shell-close")).Size(24, 24).Radius(6).AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
				closeBtn.Children(func() {
					ui.Icon(c, icons.TablerX).Width(13).Height(13).TextColor(tc.TextMuted)
				})
				if closeBtn.Hovered() {
					closeBtn.Background(tc.ToolbarControlHover)
				}
				if closeBtn.Clicked() {
					a.ShowBgShellPopover = false
					a.invalidateUI()
				}
			})

			// Shell list + mini shell previews
			ui.Scroll(c).MaxHeight(460).Children(func() {
				ui.Column(c).Padding(12, 14).Gap(10).Children(func() {
					for _, entry := range shells {
						sh := entry
						elapsed := time.Since(sh.StartedAt).Truncate(time.Second)
						item := ui.Column(c.Key("bg-shell-item-"+sh.Source+"-"+sh.ID)).
							Radius(10).Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border.Alpha(0.6)).
							Padding(10, 12).Gap(8)
						item.Children(func() {
							// Session name + elapsed + x kill button
							top := ui.Row(c).Gap(8).AlignItems(ui.Center).FillWidth()
							top.Children(func() {
								ui.Box(c).Size(7, 7).Radius(4).Background(tc.Success)
								ui.Text(c, sh.Name).FontSize(12).Font("monospace").FontWeight(600).TextColor(tc.Text).Grow(1).SingleLine()
								if !sh.StartedAt.IsZero() {
									ui.Text(c, formatElapsedNice(elapsed)).FontSize(10).TextColor(tc.TextMuted)
								}
								killBtn := ui.Row(c.Key("bg-shell-kill-"+sh.Source+"-"+sh.ID)).Size(22, 22).Radius(6).
									AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer).
									Background(tc.SurfaceRaised.Alpha(0.8))
								killBtn.Children(func() {
									ui.Icon(c, icons.TablerX).Width(12).Height(12).TextColor(tc.TextMuted)
								})
								if killBtn.Hovered() {
									killBtn.Background(tc.Error.Alpha(0.15))
								}
								if killBtn.Clicked() {
									a.stopBackgroundShell(sh)
									a.queueToast(ui.Toast{Title: "Shell Stopped", Description: sh.Name, Type: "info"})
									a.invalidateUI()
								}
							})

							// Mini shell output preview
							previewText := sh.Preview
							previewBox := ui.Column(c).FillWidth().Radius(6).Padding(8, 10).
								Background(tc.Canvas).BorderWidth(1).BorderColor(tc.Border.Alpha(0.45))
							previewBox.Children(func() {
								if previewText == "" {
									ui.Text(c, "Waiting for output…").FontSize(10).Font("monospace").TextColor(tc.TextMuted)
								} else {
									ui.Scroll(c).MaxHeight(130).Children(func() {
										ui.Text(c, previewText).FontSize(10.5).Font("monospace").TextColor(tc.TextMuted).Selectable()
									})
								}
							})
						})
					}
				})
			})
		})
	})
}
