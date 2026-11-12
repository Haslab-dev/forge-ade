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
