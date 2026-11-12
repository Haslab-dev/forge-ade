package palette

import (
	"testing"

	"github.com/hasdev/forge-ade/forge-ade-native/pkg/workspace"
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
