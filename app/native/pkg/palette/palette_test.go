package palette

import (
	"testing"

	"github.com/hasdev/forge-ade/app/native/pkg/workspace"
)

func TestPaletteManager(t *testing.T) {
	ws := &workspace.WorkspaceState{
		Files: []workspace.WorkspaceFile{
			{RelativePath: "cmd/main.go"},
			{RelativePath: "pkg/ui/app.go"},
			{RelativePath: "pkg/engine/engine.go"},
			{RelativePath: "README.md"},
		},
	}

	pm := NewPaletteManager(ws, nil)

	// Mode files
	pm.Toggle()
	if !pm.IsOpen {
		t.Fatal("expected palette to be open")
	}
	if pm.Mode != PaletteFiles {
		t.Fatalf("expected Mode PaletteFiles, got %d", pm.Mode)
	}

	pm.Query = "main"
	matches := pm.FilteredFiles()
	if len(matches) == 0 {
		t.Fatal("expected at least 1 match for query 'main'")
	}
	if matches[0].File.RelativePath != "cmd/main.go" {
		t.Errorf("expected first match 'cmd/main.go', got %s", matches[0].File.RelativePath)
	}

	// Mode commands
	executed := false
	pm.SetCommands([]PaletteCommand{
		{
			ID:    "workbench.action.reload",
			Label: "Developer: Reload Window",
			Run:   func() { executed = true },
		},
	})

	pm.OpenCommands()
	if pm.Mode != PaletteCommands {
		t.Fatalf("expected Mode PaletteCommands, got %d", pm.Mode)
	}

	pm.Query = "reload"
	cmds := pm.FilteredCommands()
	if len(cmds) != 1 {
		t.Fatalf("expected 1 command match, got %d", len(cmds))
	}

	cmds[0].Run()
	if !executed {
		t.Fatal("expected command to execute")
	}
}

func TestPaletteEffectiveMode(t *testing.T) {
	pm := NewPaletteManager(&workspace.WorkspaceState{}, nil)
	pm.Mode = PaletteFiles

	pm.Query = ">Save"
	if got := pm.EffectiveMode(); got != PaletteCommands {
		t.Fatalf("expected PaletteCommands for '>' prefix, got %d", got)
	}
	if pm.Mode != PaletteFiles {
		t.Fatalf("EffectiveMode mutated base Mode to %d", pm.Mode)
	}

	pm.Query = "@Handle"
	if got := pm.EffectiveMode(); got != PaletteSymbols {
		t.Fatalf("expected PaletteSymbols for '@' prefix, got %d", got)
	}
	if pm.Mode != PaletteFiles {
		t.Fatalf("EffectiveMode mutated base Mode to %d", pm.Mode)
	}

	pm.Query = ":42:7"
	if got := pm.EffectiveMode(); got != PaletteLine {
		t.Fatalf("expected PaletteLine for ':' prefix, got %d", got)
	}
	if pm.Mode != PaletteFiles {
		t.Fatalf("EffectiveMode mutated base Mode to %d", pm.Mode)
	}

	// Clearing prefix restores base Mode without losing it.
	pm.Query = "main"
	if got := pm.EffectiveMode(); got != PaletteFiles {
		t.Fatalf("expected base PaletteFiles after prefix removed, got %d", got)
	}

	if line, col, ok := LineJumpSpec(":42:7"); !ok || line != 42 || col != 7 {
		t.Fatalf("expected (42, 7, true), got (%d, %d, %v)", line, col, ok)
	}
	if line, col, ok := LineJumpSpec(":10"); !ok || line != 10 || col != 0 {
		t.Fatalf("expected (10, 0, true), got (%d, %d, %v)", line, col, ok)
	}
	if _, _, ok := LineJumpSpec(":notanumber"); ok {
		t.Fatalf("expected ok=false for non-numeric line spec")
	}
}
