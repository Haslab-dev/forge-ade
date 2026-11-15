package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Haslab-dev/RakitKode/events"
)

// TestForgePathsNeverFiltered verifies `.forge/` events fire even though the
// directory is gitignored (board_tasks.json is typically ignored, yet the
// board watcher depends on its change events).
func TestForgePathsNeverFiltered(t *testing.T) {
	dir := t.TempDir()
	forgeDir := filepath.Join(dir, ".forge")
	if err := os.MkdirAll(forgeDir, 0755); err != nil {
		t.Fatal(err)
	}
	// gitignore both .forge and node_modules.
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".forge/\nnode_modules/\n"), 0644); err != nil {
		t.Fatal(err)
	}

	bus := events.NewBus()
	w, err := New(bus)
	if err != nil {
		t.Fatal(err)
	}
	w.Start()
	if err := w.WatchDir(dir); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)

	boardFile := filepath.Join(forgeDir, "board_tasks.json")
	evs := collect(t, bus, func() {
		os.WriteFile(boardFile, []byte(`[]`), 0644)
	}, 3*time.Second)
	if !hasType(evs, events.FileCreated, boardFile) {
		t.Fatalf("expected create event for gitignored %s, got %+v", boardFile, evs)
	}

	// Writes to the pre-existing ignored file must surface as changes too.
	evs = collect(t, bus, func() {
		os.WriteFile(boardFile, []byte(`[{"id":"1"}]`), 0644)
	}, 3*time.Second)
	if !hasType(evs, events.FileChanged, boardFile) {
		t.Fatalf("expected change event for %s, got %+v", boardFile, evs)
	}

	// Sanity: other ignored locations stay filtered.
	junk := filepath.Join(dir, "node_modules", "pkg.js")
	os.MkdirAll(filepath.Join(dir, "node_modules"), 0755)
	time.Sleep(300 * time.Millisecond)
	evs = collect(t, bus, func() {
		os.WriteFile(junk, []byte("x"), 0644)
	}, 1500*time.Millisecond)
	if hasType(evs, events.FileCreated, junk) || hasType(evs, events.FileChanged, junk) {
		t.Fatalf("node_modules event leaked through filters: %+v", evs)
	}
}

// TestIsForgePath pins the matcher used by the filters.
func TestIsForgePath(t *testing.T) {
	cases := map[string]bool{
		"/ws/.forge/board_tasks.json": true,
		"/ws/.forge/snapshots/s/t/f":  true,
		"/ws/.forgery/file.txt":       false,
		"/ws/src/.forge":              true,
		"/ws/src/main.go":             false,
		"":                            false,
	}
	for path, want := range cases {
		if got := isForgePath(path); got != want {
			t.Errorf("isForgePath(%q) = %v, want %v", path, got, want)
		}
	}
}

// TestUnwatchDirStopsEvents verifies UnwatchDir removes the root and its
// subtree handles: events in the previously watched tree stop arriving.
func TestUnwatchDirStopsEvents(t *testing.T) {
	dir := t.TempDir()
	bus := events.NewBus()
	w, err := New(bus)
	if err != nil {
		t.Fatal(err)
	}
	w.Start()
	if err := w.WatchDir(dir); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)

	if err := w.UnwatchDir(dir); err != nil {
		t.Fatalf("UnwatchDir: %v", err)
	}
	time.Sleep(300 * time.Millisecond)

	f := filepath.Join(dir, "after-unwatch.txt")
	evs := collect(t, bus, func() {
		os.WriteFile(f, []byte("x"), 0644)
	}, 1200*time.Millisecond)
	if len(evs) != 0 {
		t.Fatalf("expected no events after UnwatchDir, got %+v", evs)
	}

	// Re-watching works (engine workspace switches go back and forth).
	if err := w.WatchDir(dir); err != nil {
		t.Fatalf("re-WatchDir: %v", err)
	}
	time.Sleep(300 * time.Millisecond)
	evs = collect(t, bus, func() {
		os.WriteFile(filepath.Join(dir, "after-rewatch.txt"), []byte("x"), 0644)
	}, 3*time.Second)
	if !hasType(evs, events.FileCreated, filepath.Join(dir, "after-rewatch.txt")) {
		t.Fatalf("expected events after re-watch, got %+v", evs)
	}
}
