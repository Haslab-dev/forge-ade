package board

import (
	"os"
	"testing"
)

func TestBoardManager(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "forge-native-board-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	bm := NewBoardManager(tmpDir)
	if bm == nil {
		t.Fatal("expected non-nil BoardManager")
	}

	bm.Refresh()
	if bm.Data == nil {
		t.Fatal("expected non-nil BoardData")
	}

	if len(bm.Data.Columns) != len(BoardColumnDefs) {
		t.Errorf("expected %d columns, got %d", len(BoardColumnDefs), len(bm.Data.Columns))
	}
}
