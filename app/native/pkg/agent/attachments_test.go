package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAttachFilesValidatedLimits(t *testing.T) {
	dir := t.TempDir()
	as := &AgentState{workspacePath: dir}

	// 1. Reject directory
	subDir := filepath.Join(dir, "subdir")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := as.AttachFileValidated(subDir); err == nil || !strings.Contains(err.Error(), "directory") {
		t.Fatalf("expected directory error, got: %v", err)
	}

	// 2. Reject file exceeding MaxAttachmentSizeBytes
	bigFile := filepath.Join(dir, "huge.bin")
	f, err := os.Create(bigFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(MaxAttachmentSizeBytes + 1); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()
	if err := as.AttachFileValidated(bigFile); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected max size error, got: %v", err)
	}

	// 3. Enforce MaxAttachedFiles (10)
	var paths []string
	for i := 0; i < MaxAttachedFiles+2; i++ {
		p := filepath.Join(dir, fmt.Sprintf("img-%d.png", i))
		if err := os.WriteFile(p, []byte("png"), 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}
	added, errs := as.AttachFilesValidated(paths)
	if added != MaxAttachedFiles {
		t.Fatalf("expected %d added files, got %d", MaxAttachedFiles, added)
	}
	if len(errs) != 2 {
		t.Fatalf("expected 2 validation errors for overflow files, got %d: %v", len(errs), errs)
	}
}
