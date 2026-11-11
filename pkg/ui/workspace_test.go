package ui

import (
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
