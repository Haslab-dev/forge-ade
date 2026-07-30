package explorer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hasdev/forge-ade/internal/events"
)

func TestExplorerShowHiddenByDefault(t *testing.T) {
	bus := events.NewBus()
	exp := New(bus)

	if !exp.GetShowHidden() {
		t.Errorf("expected GetShowHidden() to be true by default, got false")
	}

	tempDir := t.TempDir()
	dotFile := filepath.Join(tempDir, ".gitignore")
	dotDir := filepath.Join(tempDir, ".build")
	normalFile := filepath.Join(tempDir, "main.go")

	if err := os.WriteFile(dotFile, []byte("build/"), 0644); err != nil {
		t.Fatalf("failed to create dot file: %v", err)
	}
	if err := os.Mkdir(dotDir, 0755); err != nil {
		t.Fatalf("failed to create dot dir: %v", err)
	}
	if err := os.WriteFile(normalFile, []byte("package main"), 0644); err != nil {
		t.Fatalf("failed to create normal file: %v", err)
	}

	exp.SetRoots([]string{tempDir})

	tree, err := exp.GetTree(1)
	if err != nil {
		t.Fatalf("GetTree failed: %v", err)
	}

	if len(tree) != 1 {
		t.Fatalf("expected 1 root in tree, got %d", len(tree))
	}

	children := tree[0].Children
	foundDotFile := false
	foundDotDir := false
	foundNormalFile := false

	for _, child := range children {
		if child.Name == ".gitignore" {
			foundDotFile = true
		}
		if child.Name == ".build" {
			foundDotDir = true
		}
		if child.Name == "main.go" {
			foundNormalFile = true
		}
	}

	if !foundDotFile {
		t.Errorf("expected .gitignore to be present in tree when showHidden is true")
	}
	if !foundDotDir {
		t.Errorf("expected .build to be present in tree when showHidden is true")
	}
	if !foundNormalFile {
		t.Errorf("expected main.go to be present in tree")
	}

	// Test disabling showHidden
	exp.SetShowHidden(false)
	treeWithoutHidden, err := exp.GetTree(1)
	if err != nil {
		t.Fatalf("GetTree failed: %v", err)
	}

	for _, child := range treeWithoutHidden[0].Children {
		if child.Name == ".gitignore" || child.Name == ".build" {
			t.Errorf("did not expect dot item %s in tree when showHidden is false", child.Name)
		}
	}
}
