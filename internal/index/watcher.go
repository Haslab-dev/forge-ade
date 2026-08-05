package index

import (
	"path/filepath"
	"strings"

	"github.com/hasdev/forge-ade/internal/events"
)

// Listen subscribes the store to workspace file events so the index stays in
// sync without full rescans (RFC §5.2): CREATE/UPDATE → parse & replace,
// DELETE → remove. Returns an unsubscribe function.
func (s *Store) Listen(bus *events.Bus) func() {
	unsub := make([]func(), 0, 3)
	for _, evt := range []events.EventType{events.FileCreated, events.FileChanged, events.FileDeleted} {
		unsub = append(unsub, bus.Subscribe(evt, s.onFileEvent))
	}
	return func() {
		for _, u := range unsub {
			u()
		}
	}
}

func (s *Store) onFileEvent(e events.Event) {
	path, _ := e.Data["path"].(string)
	if path == "" || !withinRoot(s.root, path) {
		return
	}
	switch e.Type {
	case events.FileCreated, events.FileChanged:
		_ = s.Update(path)
	case events.FileDeleted:
		_ = s.Remove(path)
	}
}

// withinRoot reports whether path is inside root (or equal to it).
func withinRoot(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
