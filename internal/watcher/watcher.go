package watcher

import (
	"log"
	"os"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/hasdev/forge-ade/internal/events"
)

// Watcher monitors the filesystem for changes and publishes events.
type Watcher struct {
	bus      *events.Bus
	watcher  *fsnotify.Watcher
	mu       sync.Mutex
	dirs     map[string]bool
	stopCh   chan struct{}
	running  bool
}

// New creates a new file watcher.
func New(bus *events.Bus) (*Watcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &Watcher{
		bus:     bus,
		watcher: w,
		dirs:    make(map[string]bool),
		stopCh:  make(chan struct{}),
	}, nil
}

// Start begins watching directories and publishing events.
func (w *Watcher) Start() {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.running = true
	w.mu.Unlock()

	go w.loop()
}

func (w *Watcher) loop() {
	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			w.handleEvent(event)

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("file watcher error: %v", err)

		case <-w.stopCh:
			return
		}
	}
}

// Stop stops the file watcher.
func (w *Watcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	close(w.stopCh)
	w.watcher.Close()
	w.running = false
}

// WatchDir adds a directory to the watch list (non-recursive).
// On macOS, FSEvents provides recursive coverage from the root watch.
func (w *Watcher) WatchDir(root string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.watcher.Add(root); err != nil {
		return err
	}
	w.dirs[root] = true
	return nil
}

// UnwatchDir removes a directory from the watch list.
func (w *Watcher) UnwatchDir(root string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.watcher.Remove(root); err != nil {
		return err
	}
	delete(w.dirs, root)
	return nil
}

func (w *Watcher) handleEvent(event fsnotify.Event) {
	evt := events.Event{
		Data: map[string]interface{}{
			"path": event.Name,
		},
	}

	switch {
	case event.Op&fsnotify.Create != 0:
		evt.Type = events.FileCreated
		// If a new directory is created, watch it too
		if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
			w.mu.Lock()
			w.watcher.Add(event.Name)
			w.dirs[event.Name] = true
			w.mu.Unlock()
			evt.Type = events.FileCreated
		}

	case event.Op&fsnotify.Write != 0:
		evt.Type = events.FileChanged

	case event.Op&fsnotify.Remove != 0:
		evt.Type = events.FileDeleted
		delete(w.dirs, event.Name)

	case event.Op&fsnotify.Rename != 0:
		evt.Type = events.FileRenamed
		delete(w.dirs, event.Name)

	case event.Op&fsnotify.Chmod != 0:
		// Permission changes, ignore for now
		return
	}

	w.bus.Publish(evt)
}
