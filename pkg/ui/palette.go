package ui

import (
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"
)

// PaletteManager manages the Quick Open fuzzy file finder.
type PaletteManager struct {
	mu            sync.RWMutex
	IsOpen        bool
	Query         string
	SelectedIndex int
	Workspace     *WorkspaceState
	OnFileSelect  func(path string)
}

// NewPaletteManager creates a new Quick Open palette manager.
func NewPaletteManager(ws *WorkspaceState, onFileSelect func(path string)) *PaletteManager {
	return &PaletteManager{
		Workspace:    ws,
		OnFileSelect: onFileSelect,
	}
}

// Toggle flips palette visibility.
func (pm *PaletteManager) Toggle() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.IsOpen = !pm.IsOpen
	if pm.IsOpen {
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

// Filtered returns matching workspace files based on the fuzzy query.
func (pm *PaletteManager) Filtered() []WorkspaceFile {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	all := pm.Workspace.Files
	q := strings.ToLower(strings.TrimSpace(pm.Query))
	if q == "" {
		if len(all) > 15 {
			return all[:15]
		}
		return all
	}

	var results []WorkspaceFile
	for _, f := range all {
		if strings.Contains(strings.ToLower(f.RelativePath), q) {
			results = append(results, f)
			if len(results) >= 15 {
				break
			}
		}
	}
	return results
}

// RenderPalette draws the Quick Open modal if open.
func (pm *PaletteManager) RenderPalette(c *ui.Context) {
	pm.mu.RLock()
	open := pm.IsOpen
	pm.mu.RUnlock()

	if !open {
		return
	}

	t := c.Theme()
	matches := pm.Filtered()

	ui.Modal(c, &pm.IsOpen, func() {
		ui.Column(c).Width(540).Radius(10).Background(t.Surface).BorderWidth(1).BorderColor(t.Border).Padding(12, 16).Gap(8).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "Quick Open File").Bold().FontSize(13)
				ui.Box(c).Grow(1)
				if ui.Button(c, "✕").Clicked() {
					pm.Close()
				}
			})

			in := ui.TextInput(c, &pm.Query).Placeholder("Search files by name (e.g. main.go)...").Grow(1).AutoFocus()
			if in.Submitted() && len(matches) > 0 {
				selected := matches[0]
				pm.Close()
				if pm.OnFileSelect != nil {
					pm.OnFileSelect(selected.RelativePath)
				}
			}

			ui.Column(c).Gap(4).Padding(4, 0).Children(func() {
				for i, file := range matches {
					filePath := file.RelativePath
					row := ui.Row(c.Key("palette-"+filePath)).Padding(6, 10).Radius(6).AlignItems(ui.Center)
					if row.Hovered() || i == pm.SelectedIndex {
						row.Background(t.Accent).TextColor(t.AccentText)
					}

					ui.Text(c, filePath).FontSize(12).Grow(1)

					if row.Clicked() {
						pm.Close()
						if pm.OnFileSelect != nil {
							pm.OnFileSelect(filePath)
						}
					}
				}
				if len(matches) == 0 {
					ui.Text(c, "No matching files").TextColor(t.TextMuted).FontSize(12).Padding(8, 0)
				}
			})
		})
	})
}
