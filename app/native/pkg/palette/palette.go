package palette

import (
	"sort"
	"strings"
	"sync"

	"github.com/hasdev/forge-ade/forge-ade-native/pkg/fuzzy"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/workspace"
)

// Palette modes: files (⌘P/⌘K), commands (⇧⌘P).
const (
	PaletteFiles    = 0
	PaletteCommands = 1
)

// PaletteCommand is one entry of the commands mode.
type PaletteCommand struct {
	ID       string
	Label    string
	Detail   string
	Shortcut string
	Run      func()
}

// FileMatch ranks a workspace file with its fuzzy score.
type FileMatch struct {
	File  workspace.WorkspaceFile
	Score float64
}

// PaletteManager manages Quick Open fuzzy file finding and command execution.
type PaletteManager struct {
	mu            sync.RWMutex
	IsOpen        bool
	Mode          int // PaletteFiles, PaletteCommands
	Query         string
	SelectedIndex int
	Workspace     *workspace.WorkspaceState
	OnFileSelect  func(path string)
	Commands      []PaletteCommand
}

// NewPaletteManager creates a new Quick Open palette manager.
func NewPaletteManager(ws *workspace.WorkspaceState, onFileSelect func(path string)) *PaletteManager {
	return &PaletteManager{
		Workspace:    ws,
		OnFileSelect: onFileSelect,
	}
}

// SetCommands attaches the command list.
func (pm *PaletteManager) SetCommands(cmds []PaletteCommand) {
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

// MoveSelection steps the highlighted row, clamped to the current mode's
// results (arrow-key navigation).
func (pm *PaletteManager) MoveSelection(dir int) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	limit := 0
	if pm.Mode == PaletteFiles {
		limit = len(pm.FilteredFiles())
	} else {
		limit = len(pm.FilteredCommands())
	}
	if limit == 0 {
		pm.SelectedIndex = 0
		return
	}
	pm.SelectedIndex += dir
	if pm.SelectedIndex < 0 {
		pm.SelectedIndex = 0
	}
	if pm.SelectedIndex >= limit {
		pm.SelectedIndex = limit - 1
	}
}

// FilteredFiles ranks workspace files by the fuzzy query.
func (pm *PaletteManager) FilteredFiles() []FileMatch {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if pm.Workspace == nil {
		return nil
	}

	all := pm.Workspace.Files
	q := pm.Query
	if strings.TrimSpace(q) == "" {
		out := make([]FileMatch, 0, len(all))
		for _, f := range all {
			out = append(out, FileMatch{File: f})
			if len(out) >= 15 {
				break
			}
		}
		return out
	}

	var results []FileMatch
	for _, f := range all {
		if m := fuzzy.ScoreFilePath(q, f.RelativePath); m != nil {
			results = append(results, FileMatch{File: f, Score: m.Score})
		}
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	if len(results) > 15 {
		results = results[:15]
	}
	return results
}

// FilteredCommands ranks registered commands by the fuzzy query.
func (pm *PaletteManager) FilteredCommands() []PaletteCommand {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	q := pm.Query
	var results []PaletteCommand
	if strings.TrimSpace(q) == "" {
		return pm.Commands
	}

	type cmdMatch struct {
		cmd   PaletteCommand
		score float64
	}
	var ms []cmdMatch
	for _, cmd := range pm.Commands {
		if m := fuzzy.ScoreText(q, cmd.Label+" "+cmd.Detail); m != nil {
			ms = append(ms, cmdMatch{cmd: cmd, score: m.Score})
		}
	}
	sort.Slice(ms, func(i, j int) bool { return ms[i].score > ms[j].score })
	for _, m := range ms {
		results = append(results, m.cmd)
	}
	return results
}
