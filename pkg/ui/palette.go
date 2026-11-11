package ui

import (
	"sort"
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"
)

// Palette modes, matching CommandPaletteModal.tsx: files (⌘P/⌘K), commands
// (⇧⌘P).
const (
	PaletteFiles    = 0
	PaletteCommands = 1
)

// paletteCommand is one entry of the commands mode.
type paletteCommand struct {
	ID       string
	Label    string
	Detail   string
	Shortcut string
	Run      func()
}

// PaletteManager manages the Quick Open fuzzy file finder and the command
// palette.
type PaletteManager struct {
	mu            sync.RWMutex
	IsOpen        bool
	Mode          int // PaletteFiles, PaletteCommands
	Query         string
	SelectedIndex int
	Workspace     *WorkspaceState
	OnFileSelect  func(path string)
	Commands      []paletteCommand
}

// NewPaletteManager creates a new Quick Open palette manager.
func NewPaletteManager(ws *WorkspaceState, onFileSelect func(path string)) *PaletteManager {
	return &PaletteManager{
		Workspace:    ws,
		OnFileSelect: onFileSelect,
	}
}

// SetCommands attaches the command list (wired by AppUI so commands can
// reach its managers).
func (pm *PaletteManager) SetCommands(cmds []paletteCommand) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.Commands = cmds
}

// OpenCommands opens the palette in commands mode (⇧⌘P).
func (pm *PaletteManager) OpenCommands() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.IsOpen = true
	pm.Mode = PaletteCommands
	pm.Query = ""
	pm.SelectedIndex = 0
}

// Toggle flips palette visibility in files mode.
func (pm *PaletteManager) Toggle() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.IsOpen = !pm.IsOpen
	if pm.IsOpen {
		pm.Mode = PaletteFiles
		pm.Query = ""
		pm.SelectedIndex = 0
	}
}

// Close hides the palette.
func (pm *PaletteManager) Close() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.IsOpen = false
}

// filteredFiles ranks workspace files by the fuzzy query.
type fileMatch struct {
	file  WorkspaceFile
	score float64
}

func (pm *PaletteManager) filteredFiles() []fileMatch {
	all := pm.Workspace.Files
	q := pm.Query
	if strings.TrimSpace(q) == "" {
		out := make([]fileMatch, 0, len(all))
		for _, f := range all {
			out = append(out, fileMatch{file: f})
			if len(out) >= 15 {
				break
			}
		}
		return out
	}
	var results []fileMatch
	for _, f := range all {
		if m := ScoreFilePath(q, f.RelativePath); m != nil {
			results = append(results, fileMatch{file: f, score: m.Score})
		}
	}
	sort.Slice(results, func(i, j int) bool { return results[i].score > results[j].score })
	if len(results) > 15 {
		results = results[:15]
	}
	return results
}

func (pm *PaletteManager) filteredCommands() []paletteCommand {
	q := pm.Query
	var results []paletteCommand
	if strings.TrimSpace(q) == "" {
		return pm.Commands
	}
	type cmdMatch struct {
		cmd   paletteCommand
		score float64
	}
	var ms []cmdMatch
	for _, cmd := range pm.Commands {
		if m := ScoreText(q, cmd.Label+" "+cmd.Detail); m != nil {
			ms = append(ms, cmdMatch{cmd: cmd, score: m.Score})
		}
	}
	sort.Slice(ms, func(i, j int) bool { return ms[i].score > ms[j].score })
	for _, m := range ms {
		results = append(results, m.cmd)
	}
	return results
}

// RenderPalette draws the Quick Open modal if open, styled as
// CommandPaletteModal: max-w-xl rounded-2xl popover with a search row, a
// 320px list and a footer with mode switches.
func (pm *PaletteManager) RenderPalette(c *ui.Context, tc ThemeColors) {
	pm.mu.RLock()
	open := pm.IsOpen
	pm.mu.RUnlock()

	if !open {
		return
	}

	// Arrow navigation (works while the query input holds the keyboard
	// focus, as Shortcut polls where the focus is).
	if c.Shortcut(0, ui.KeyDown) {
		pm.mu.Lock()
		pm.SelectedIndex++
		limit := pm.selectionLimit()
		if pm.SelectedIndex >= limit {
			pm.SelectedIndex = limit - 1
		}
		pm.mu.Unlock()
	}
	if c.Shortcut(0, ui.KeyUp) {
		pm.mu.Lock()
		pm.SelectedIndex--
		if pm.SelectedIndex < 0 {
			pm.SelectedIndex = 0
		}
		pm.mu.Unlock()
	}
	if c.Shortcut(0, ui.KeyEscape) {
		pm.Close()
	}

	_ = c

	var matches []fileMatch
	var commands []paletteCommand
	pm.mu.RLock()
	if pm.Mode == PaletteFiles {
		matches = pm.filteredFiles()
	} else {
		commands = pm.filteredCommands()
	}
	sel := pm.SelectedIndex
	if sel >= len(matches)+len(commands) {
		sel = 0
		if pm.Mode == PaletteFiles {
			pm.mu.Lock()
			pm.SelectedIndex = 0
			pm.mu.Unlock()
		} else {
			pm.mu.Lock()
			pm.SelectedIndex = 0
			pm.mu.Unlock()
		}
	}
	mode := pm.Mode
	pm.mu.RUnlock()

	activate := func() {
		if mode == PaletteFiles {
			if sel < len(matches) {
				selected := matches[sel].file.RelativePath
				pm.Close()
				if pm.OnFileSelect != nil {
					pm.OnFileSelect(selected)
				}
			}
		} else {
			if sel < len(commands) {
				cmd := commands[sel]
				pm.Close()
				if cmd.Run != nil {
					cmd.Run()
				}
			}
		}
	}

	ui.Modal(c, &pm.IsOpen, func() {
		panel := ui.Column(c).Width(576).Radius(16).Background(tc.SurfaceOverlay).BorderWidth(1).BorderColor(tc.Border).
			Shadow(0, 24, 72, -20, ui.RGBA(0, 0, 0, 0.55))
		panel.Children(func() {
			// Search row: p-3.5 border-b with the mode icon and ESC hint
			ui.Row(c).Padding(14).Gap(8).AlignItems(ui.Center).
				BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
				icon := TablerSearch
				if mode == PaletteCommands {
					icon = TablerChevronRight
				}
				ui.Icon(c, icon).Width(15).Height(15).TextColor(tc.IconMuted)
				placeholder := "Search files by name..."
				if mode == PaletteCommands {
					placeholder = "Type a command..."
				}
				in := ui.TextInput(c, &pm.Query).Placeholder(placeholder).Grow(1).AutoFocus()
				if in.Submitted() {
					activate()
				}
				kbd := ui.Row(c).Radius(4).Padding(1, 6).Background(tc.ToolbarControl)
				kbd.Children(func() {
					ui.Text(c, "ESC").FontSize(12)
				})
			})

			// Result list: max-h-80 p-1.5 space-y-0.5; items px-3 py-2
			// rounded-xl text-xs; selected bg-primary/10 text-primary.
			ui.Scroll(c).MaxHeight(320).Children(func() {
				ui.Column(c).Padding(6).Gap(2).Children(func() {
					if mode == PaletteFiles {
						for i, m := range matches {
							i, path := i, m.file.RelativePath
							selected := i == sel
							row := ui.Row(c.Key("pal-"+path)).Radius(10).Padding(8, 12).Gap(8).AlignItems(ui.Center)
							row.Children(func() {
								ui.Icon(c, FileIconSVG(path)).Width(13).Height(13).TextColor(tc.IconMuted)
								ui.Text(c, path).FontSize(13).Grow(1)
							})
							if selected {
								row.Background(tc.Accent.Alpha(0.1))
								row.Children(func() {
									ui.Box(c).Grow(1)
									ui.Icon(c, TablerChevronRight).Width(12).Height(12).TextColor(tc.Accent)
								})
							} else if row.Hovered() {
								row.Background(tc.Text.Alpha(0.04))
							}
							if row.Clicked() {
								pm.Close()
								if pm.OnFileSelect != nil {
									pm.OnFileSelect(path)
								}
							}
						}
						if len(matches) == 0 {
							ui.Text(c, "No matching files").FontSize(13).TextColor(tc.TextMuted).Padding(8, 0)
						}
					} else {
						for i, cmd := range commands {
							i, cmd := i, cmd
							selected := i == sel
							row := ui.Row(c.Key("pal-cmd-"+cmd.ID)).Radius(10).Padding(8, 12).Gap(8).AlignItems(ui.Center)
							row.Children(func() {
								ui.Column(c).Grow(1).Children(func() {
									lbl := ui.Text(c, cmd.Label).FontSize(13)
									if selected {
										lbl.FontWeight(500)
									}
									if cmd.Detail != "" {
										ui.Text(c, cmd.Detail).FontSize(11).TextColor(tc.TextMuted)
									}
								})
								if cmd.Shortcut != "" {
									ui.Text(c, cmd.Shortcut).FontSize(11).TextColor(tc.SecondaryLabel)
								}
							})
							if selected {
								row.Background(tc.Accent.Alpha(0.1))
							} else if row.Hovered() {
								row.Background(tc.Text.Alpha(0.04))
							}
							if row.Clicked() {
								pm.Close()
								if cmd.Run != nil {
									cmd.Run()
								}
							}
						}
						if len(commands) == 0 {
							ui.Text(c, "No matching commands").FontSize(13).TextColor(tc.TextMuted).Padding(8, 0)
						}
					}
				})
			})

			// Footer: mode switches and hints
			ui.Row(c).Padding(8, 12).Gap(8).AlignItems(ui.Center).
				BorderWidth(1, 0, 0, 0).BorderColor(tc.Border).Children(func() {
				modeBtn := func(label string, m int) {
					active := mode == m
					btn := ui.Row(c).Radius(4).Padding(2, 8).AlignItems(ui.Center)
					btn.Children(func() {
						txt := ui.Text(c, label).FontSize(12)
						if active {
							txt.TextColor(tc.Accent).FontWeight(500)
						} else {
							txt.TextColor(tc.TextMuted)
						}
					})
					if active {
						btn.Background(tc.Accent.Alpha(0.1))
					} else if btn.Hovered() {
						btn.Background(tc.Text.Alpha(0.04))
					}
					if btn.Clicked() {
						pm.mu.Lock()
						pm.Mode = m
						pm.SelectedIndex = 0
						pm.mu.Unlock()
					}
				}
				modeBtn("Files", PaletteFiles)
				modeBtn("Commands", PaletteCommands)
				ui.Box(c).Grow(1)
				ui.Text(c, "↑↓ navigate · ↵ open").FontSize(12).TextColor(tc.TextMuted)
			})
		})
	})

}

// selectionLimit returns how many rows the current mode shows.
func (pm *PaletteManager) selectionLimit() int {
	if pm.Mode == PaletteFiles {
		return len(pm.filteredFiles())
	}
	return len(pm.filteredCommands())
}
