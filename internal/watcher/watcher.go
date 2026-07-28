package watcher

import (
	"log"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/fsnotify/fsnotify"
	"github.com/hasdev/forge-ade/internal/events"
)

func init() {
	// Try to increase open file limit — fsnotify needs 1 FD per watched dir
	var rLimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit); err == nil {
		if rLimit.Cur < 10240 {
			rLimit.Cur = 10240
			syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
		}
	}
}

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

// WatchDir adds a directory and all its subdirectories to the watch list.
func (w *Watcher) WatchDir(root string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.watchRecursive(root)
}

func (w *Watcher) watchRecursive(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip inaccessible dirs
		}
		if info.IsDir() {
			if isSkipped(info.Name()) {
				return filepath.SkipDir
			}
			if err := w.watcher.Add(path); err != nil {
				log.Printf("watcher: skip %s (%v)", path, err)
				return nil
			}
			w.dirs[path] = true
		}
		return nil
	})
}

// UnwatchDir removes a directory from the watch list.
func (w *Watcher) UnwatchDir(root string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if err := w.watcher.Remove(path); err != nil {
				return nil
			}
			delete(w.dirs, path)
		}
		return nil
	})
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
		// Watch newly created directories
		if info, err := os.Stat(event.Name); err == nil && info.IsDir() && !isSkipped(info.Name()) {
			w.mu.Lock()
			w.watcher.Add(event.Name)
			w.dirs[event.Name] = true
			w.mu.Unlock()
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
		return
	}

	w.bus.Publish(evt)
}

var skipDirs = map[string]bool{
	"node_modules": true, ".git": true, ".svn": true,
	"vendor": true, ".next": true, ".cache": true,
	"dist": true, "build": true, "coverage": true,
	"__pycache__": true, ".hg": true,
}

func isSkipped(name string) bool {
	return skipDirs[name]
}
