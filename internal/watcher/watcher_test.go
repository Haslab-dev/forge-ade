package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hasdev/forge-ade/internal/events"
)

func newWatcher(t *testing.T) (*Watcher, *events.Bus, string) {
	t.Helper()
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
	return w, bus, dir
}

func collect(t *testing.T, bus *events.Bus, action func(), timeout time.Duration) []events.Event {
	t.Helper()
	got := make(chan events.Event, 16)
	for _, et := range []events.EventType{events.FileCreated, events.FileRenamed, events.FileDeleted, events.FileChanged} {
		bus.Subscribe(et, func(e events.Event) { got <- e })
	}
	action()
	var out []events.Event
	deadline := time.After(timeout)
	for {
		select {
		case e := <-got:
			out = append(out, e)
			if len(out) >= 8 {
				return out
			}
		case <-deadline:
			return out
		}
	}
}

func hasType(evs []events.Event, et events.EventType, path string) bool {
	for _, e := range evs {
		p, _ := e.Data["path"].(string)
		if e.Type == et && p == path {
			return true
		}
	}
	return false
}

// Renames must surface as a delete (old path) + create (new path) so the
// frontend can pair them by content and follow open tabs.
func TestRenameSurfacesAsDeleteAndCreate(t *testing.T) {
	_, bus, dir := newWatcher(t)
	old := filepath.Join(dir, "old.txt")
	newp := filepath.Join(dir, "new.txt")
	os.WriteFile(old, []byte("hello"), 0644)
	time.Sleep(300 * time.Millisecond)

	evs := collect(t, bus, func() { os.Rename(old, newp) }, 4*time.Second)
	if !hasType(evs, events.FileDeleted, old) {
		t.Fatalf("expected delete event for old path, got %+v", evs)
	}
	if !hasType(evs, events.FileCreated, newp) {
		t.Fatalf("expected create event for new path, got %+v", evs)
	}
}

func TestPureDelete(t *testing.T) {
	_, bus, dir := newWatcher(t)
	f := filepath.Join(dir, "gone.txt")
	os.WriteFile(f, []byte("x"), 0644)
	time.Sleep(300 * time.Millisecond)

	evs := collect(t, bus, func() { os.Remove(f) }, 2*time.Second)
	if !hasType(evs, events.FileDeleted, f) {
		t.Fatalf("expected delete event, got %+v", evs)
	}
}

func TestPureCreate(t *testing.T) {
	_, bus, dir := newWatcher(t)
	f := filepath.Join(dir, "fresh.txt")
	evs := collect(t, bus, func() { os.WriteFile(f, []byte("x"), 0644) }, 2*time.Second)
	if !hasType(evs, events.FileCreated, f) {
		t.Fatalf("expected create event, got %+v", evs)
	}
}

func TestModify(t *testing.T) {
	_, bus, dir := newWatcher(t)
	f := filepath.Join(dir, "edit.txt")
	os.WriteFile(f, []byte("x"), 0644)
	time.Sleep(300 * time.Millisecond)

	evs := collect(t, bus, func() { os.WriteFile(f, []byte("xyz"), 0644) }, 2*time.Second)
	if !hasType(evs, events.FileChanged, f) {
		t.Fatalf("expected change event, got %+v", evs)
	}
}

// Atomic saves (write temp, rename over target) must produce an event that
// points at the target so open tabs re-sync.
func TestAtomicSaveTargetEvent(t *testing.T) {
	_, bus, dir := newWatcher(t)
	target := filepath.Join(dir, "main.go")
	os.WriteFile(target, []byte("package main\n"), 0644)
	time.Sleep(300 * time.Millisecond)

	evs := collect(t, bus, func() {
		tmp := filepath.Join(dir, ".main.go.tmp")
		os.WriteFile(tmp, []byte("package main\n// new\n"), 0644)
		time.Sleep(100 * time.Millisecond)
		os.Rename(tmp, target)
	}, 4*time.Second)

	if !hasType(evs, events.FileCreated, target) && !hasType(evs, events.FileChanged, target) {
		t.Fatalf("expected event for target %s, got %+v", target, evs)
	}
}
