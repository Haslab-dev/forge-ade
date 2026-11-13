package database

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hasdev/forge-ade/app/native/pkg/workspace"
)

func TestDatabaseManager(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "db-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.sqlite")
	if err := os.WriteFile(dbPath, []byte("SQLite format 3\x00"), 0644); err != nil {
		t.Fatalf("write sqlite file: %v", err)
	}

	ws := workspace.NewWorkspaceState(tempDir)
	dm := NewDatabaseManager(ws)

	if len(dm.Databases) != 1 {
		t.Fatalf("expected 1 detected database, got %d", len(dm.Databases))
	}

	if dm.Databases[0] != dbPath {
		t.Fatalf("expected dbPath %s, got %s", dbPath, dm.Databases[0])
	}
}
