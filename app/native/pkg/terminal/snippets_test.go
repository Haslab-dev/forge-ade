package terminal

import (
	"os"
	"testing"
)

func TestSnippetManager(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "snippets-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	sm := NewSnippetManager(tmpDir)

	// Builtin snippets should be loaded
	all := sm.AllSnippets()
	if len(all) == 0 {
		t.Errorf("expected builtin snippets, got 0")
	}

	// Filter by category
	dockerSnips := sm.FilterSnippets("docker", "")
	if len(dockerSnips) == 0 {
		t.Errorf("expected docker snippets")
	}

	// Add custom snippet
	added := sm.AddSnippet("My Test Snippet", "echo hello world", "custom", "test description")
	if added.ID == "" || added.Title != "My Test Snippet" {
		t.Errorf("failed to add snippet: %+v", added)
	}

	// Query snippet
	found := sm.FilterSnippets("all", "hello world")
	if len(found) != 1 || found[0].ID != added.ID {
		t.Errorf("expected to find added snippet by query, got: %+v", found)
	}

	// Update snippet
	ok := sm.UpdateSnippet(added.ID, "Updated Title", "echo hello updated", "dev", "new desc")
	if !ok {
		t.Errorf("expected update to succeed")
	}

	// History operations
	sm.RecordHistory("git status", "manual")
	sm.RecordHistory("go build ./...", "manual")
	hist := sm.HistoryItems("")
	if len(hist) != 2 {
		t.Fatalf("expected 2 history items, got %d", len(hist))
	}
	if hist[0].Command != "go build ./..." {
		t.Errorf("expected most recent command first, got %s", hist[0].Command)
	}

	// Toggle favorite
	fav := sm.ToggleFavorite(hist[0].ID)
	if !fav {
		t.Errorf("expected favorite to be true")
	}

	// Delete snippet
	delOk := sm.DeleteSnippet(added.ID)
	if !delOk {
		t.Errorf("expected snippet deletion to succeed")
	}
}
