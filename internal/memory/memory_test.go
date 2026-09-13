package memory

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMemoryLifecycle(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "memory_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr := New(tmpDir)
	if len(mgr.List()) != 0 {
		t.Fatalf("expected 0 entries initially, got %d", len(mgr.List()))
	}

	err = mgr.Save(Entry{
		Key:      "database_convention",
		Content:  "Always use PostgreSQL with snake_case table and column names.",
		Category: "architecture",
		Scope:    "workspace",
	})
	if err != nil {
		t.Fatalf("failed to save memory: %v", err)
	}

	list := mgr.List()
	if len(list) != 1 {
		t.Fatalf("expected 1 memory entry, got %d", len(list))
	}
	if list[0].Key != "database_convention" {
		t.Errorf("unexpected key: %s", list[0].Key)
	}

	// Verify file was written
	jsonPath := filepath.Join(tmpDir, ".forge", "memory.json")
	if _, err := os.Stat(jsonPath); err != nil {
		t.Errorf("expected memory.json to exist at %s", jsonPath)
	}

	// Search
	results := mgr.Search("postgresql")
	if len(results) != 1 {
		t.Fatalf("expected 1 search result, got %d", len(results))
	}

	// FormatPrompt
	prompt := mgr.FormatPrompt()
	if prompt == "" {
		t.Errorf("expected non-empty format prompt")
	}

	// Delete
	err = mgr.Delete("database_convention")
	if err != nil {
		t.Fatalf("failed to delete memory: %v", err)
	}
	if len(mgr.List()) != 0 {
		t.Fatalf("expected 0 entries after delete, got %d", len(mgr.List()))
	}
}
