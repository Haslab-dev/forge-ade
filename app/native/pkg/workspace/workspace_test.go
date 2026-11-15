package workspace

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspaceState(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ws-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files
	_ = os.WriteFile(filepath.Join(tempDir, "main.go"), []byte("package main\n"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "README.md"), []byte("# Test\n"), 0644)
	_ = os.Mkdir(filepath.Join(tempDir, ".git"), 0755)
	_ = os.WriteFile(filepath.Join(tempDir, ".git", "config"), []byte("git config\n"), 0644)

	ws := NewWorkspaceState(tempDir)

	if len(ws.Files) != 2 {
		t.Fatalf("expected 2 files (ignoring .git), got %d", len(ws.Files))
	}

	ws.FilterQuery = "main"
	filtered := ws.FilteredFiles()
	if len(filtered) != 1 || filtered[0].RelativePath != "main.go" {
		t.Fatalf("expected 1 filtered file 'main.go', got %d", len(filtered))
	}

	resolved := ws.ResolveFullPath("main.go")
	if resolved != filepath.Join(tempDir, "main.go") {
		t.Fatalf("unexpected resolved path: %s", resolved)
	}

	// Idempotency: absolute paths must never be double-prefixed.
	absUnder := filepath.Join(tempDir, "pkg", "x.go")
	if got := ws.ResolveFullPath(absUnder); got != absUnder {
		t.Fatalf("absolute path under root must return unchanged: got %s want %s", got, absUnder)
	}
	if got := ws.ResolveFullPath(ws.ResolveFullPath(absUnder)); got != absUnder {
		t.Fatalf("ResolveFullPath is not idempotent: %s", got)
	}
	absOutside := "/tmp/elsewhere/y.go"
	if got := ws.ResolveFullPath(absOutside); got != absOutside {
		t.Fatalf("absolute path outside root must return as-is: got %s want %s", got, absOutside)
	}
}

func TestSearchFilesRespectsContextAndFindsHits(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ws-search-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)
	_ = os.WriteFile(filepath.Join(tempDir, "a.txt"), []byte("hello world\n"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "b.txt"), []byte("say hello back\n"), 0644)

	ws := NewWorkspaceState(tempDir)
	hits := ws.SearchFiles(context.Background(), "hello")
	if len(hits) != 2 {
		t.Fatalf("expected 2 hits for 'hello', got %d", len(hits))
	}
	if hits[0].Line != 1 {
		t.Fatalf("expected line 1, got %d", hits[0].Line)
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if got := ws.SearchFiles(cancelled, "hello"); len(got) != 0 {
		t.Fatalf("cancelled context must return no hits, got %d", len(got))
	}
}

func TestFilesUnderCoversChildren(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ws-under-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)
	_ = os.MkdirAll(filepath.Join(tempDir, "pkg", "sub"), 0755)
	_ = os.WriteFile(filepath.Join(tempDir, "pkg", "a.go"), []byte("package pkg\n"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "pkg", "sub", "b.go"), []byte("package sub\n"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "top.go"), []byte("package main\n"), 0644)

	ws := NewWorkspaceState(tempDir)
	under := ws.FilesUnder("pkg")
	found := map[string]bool{}
	for _, f := range under {
		found[f.RelativePath] = true
	}
	if !found["pkg/a.go"] || !found["pkg/sub/b.go"] {
		t.Fatalf("FilesUnder(pkg) must include nested children, got %v", found)
	}
	if found["top.go"] {
		t.Fatalf("FilesUnder(pkg) must not include siblings, got %v", found)
	}
	underFile := ws.FilesUnder("top.go")
	if len(underFile) != 1 || underFile[0].RelativePath != "top.go" {
		t.Fatalf("FilesUnder(file) must return the file itself, got %v", underFile)
	}
}

func TestWorkspaceFileOperations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ws-ops-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	ws := NewWorkspaceState(tempDir)

	// Create file
	if err := ws.CreateFile("pkg/hello.go"); err != nil {
		t.Fatalf("CreateFile failed: %v", err)
	}
	if len(ws.Files) != 1 || ws.Files[0].RelativePath != "pkg/hello.go" {
		t.Fatalf("expected 1 file 'pkg/hello.go', got %d", len(ws.Files))
	}

	// Delete file
	if err := ws.DeleteFile("pkg/hello.go"); err != nil {
		t.Fatalf("DeleteFile failed: %v", err)
	}
	if len(ws.Files) != 0 {
		t.Fatalf("expected 0 files after delete, got %d", len(ws.Files))
	}
}
