package editor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEditorManager(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ed-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	filePath := filepath.Join(tempDir, "sample.go")
	initialContent := "package main\n\nfunc main() {}\n"
	if err := os.WriteFile(filePath, []byte(initialContent), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	em := NewEditorManager()
	doc, err := em.OpenFile(filePath, "sample.go")
	if err != nil {
		t.Fatalf("OpenFile failed: %v", err)
	}

	if doc == nil || doc.Editor == nil || doc.Editor.Buffer == nil {
		t.Fatalf("expected valid document and buffer")
	}

	if doc.Editor.Buffer.Modified() {
		t.Fatalf("newly opened file should not be modified")
	}

	// Active document
	active := em.ActiveDoc()
	if active != doc {
		t.Fatalf("expected active document to match opened document")
	}

	// Word completions
	comps := WordCompletions(doc.Editor.Buffer, "ma")
	if len(comps) == 0 {
		t.Fatalf("expected completions for 'ma', got 0")
	}

	// Word at offset
	word, start, end := WordAtOffset(doc.Editor.Buffer, 3) // inside "package"
	if word != "package" || start != 0 || end != 7 {
		t.Fatalf("expected word 'package' at [0:7], got '%s' at [%d:%d]", word, start, end)
	}

	word2, start2, end2 := WordAtOffset(doc.Editor.Buffer, 9) // inside "main"
	if word2 != "main" || start2 != 8 || end2 != 12 {
		t.Fatalf("expected word 'main' at [8:12], got '%s' at [%d:%d]", word2, start2, end2)
	}
}

func TestEditorMinimapSplitAndRename(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ed-reg-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fileA := filepath.Join(tempDir, "a.go")
	fileB := filepath.Join(tempDir, "b.go")
	fileC := filepath.Join(tempDir, "c.go")
	renamedB := filepath.Join(tempDir, "b_renamed.go")
	for _, p := range []string{fileA, fileB, fileC} {
		if err := os.WriteFile(p, []byte("package main\n\nfunc run() {}\n"), 0644); err != nil {
			t.Fatalf("write file %s: %v", p, err)
		}
	}

	em := NewEditorManager()
	docA, err := em.OpenFile(fileA, "a.go")
	if err != nil {
		t.Fatalf("OpenFile a.go: %v", err)
	}
	if _, err := em.OpenFile(fileB, "b.go"); err != nil {
		t.Fatalf("OpenFile b.go: %v", err)
	}
	if _, err := em.OpenFile(fileC, "c.go"); err != nil {
		t.Fatalf("OpenFile c.go: %v", err)
	}

	// MinimapSummary caches by buffer version and refreshes on edit.
	buckets1 := docA.MinimapSummary(180)
	if len(buckets1) == 0 {
		t.Fatalf("expected non-empty minimap buckets")
	}
	docA.Editor.Buffer.Set("package main\nfunc alpha() {}\nfunc beta() {}\nfunc gamma() {}\n")
	buckets2 := docA.MinimapSummary(180)
	if len(buckets2) <= len(buckets1) {
		t.Errorf("expected minimap bucket count to increase after inserting lines (%d -> %d)", len(buckets1), len(buckets2))
	}

	// Split panes, weight clamping, and close pane.
	_, _ = em.OpenFile(fileB, "b.go")
	em.SplitRight()
	if !em.IsSplit || len(em.PaneDocs()) != 2 {
		t.Fatalf("expected 2 split panes, IsSplit=%v panes=%v", em.IsSplit, em.PaneDocs())
	}
	em.SetPaneWeight(0, 0.01)
	if w := em.PaneWeight(0); w != 0.1 {
		t.Errorf("expected pane weight clamped to 0.1, got %v", w)
	}
	em.ClosePane(1)
	if em.IsSplit || em.PaneDocs() != nil {
		t.Errorf("expected single pane after ClosePane, IsSplit=%v panes=%v", em.IsSplit, em.PaneDocs())
	}

	// RenameOpenTab preserves OpenTabPaths order, pins, and ActivePath.
	em.TogglePin(fileB)
	_, _ = em.OpenFile(fileB, "b.go")
	em.RenameOpenTab(fileB, renamedB, "b_renamed.go")

	if len(em.OpenTabPaths) != 3 || em.OpenTabPaths[0] != fileA || em.OpenTabPaths[1] != renamedB || em.OpenTabPaths[2] != fileC {
		t.Errorf("expected tab order [%s %s %s], got %v", fileA, renamedB, fileC, em.OpenTabPaths)
	}
	if !em.IsPinned(renamedB) || em.IsPinned(fileB) {
		t.Errorf("expected pin transferred to %s (old=%v new=%v)", renamedB, em.IsPinned(fileB), em.IsPinned(renamedB))
	}
	if em.ActivePath != renamedB {
		t.Errorf("expected ActivePath=%s, got %s", renamedB, em.ActivePath)
	}
}
