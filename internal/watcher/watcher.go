package watcher

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/fsnotify/fsnotify"
	"github.com/hasdev/forge-ade/internal/events"
	"github.com/hasdev/forge-ade/internal/gitignore"
	"github.com/hasdev/forge-ade/internal/ignore"
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
	mu       sync.RWMutex
	dirs     map[string]bool
	matchers map[string]*gitignore.Matcher
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
		bus:      bus,
		watcher:  w,
		dirs:     make(map[string]bool),
		matchers: make(map[string]*gitignore.Matcher),
		stopCh:   make(chan struct{}),
	}, nil
}

// Start begins watching directories and publishing events.
func (w *Watcher) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		return
	}

	// Recreate the fsnotify watcher in case it was stopped before
	if w.watcher != nil {
		_ = w.watcher.Close()
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("watcher: failed to recreate: %v", err)
		return
	}
	w.watcher = watcher
	w.stopCh = make(chan struct{})
	w.running = true

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
	_ = w.watcher.Close()
	w.dirs = make(map[string]bool)
	w.matchers = make(map[string]*gitignore.Matcher)
	w.running = false
}

// WatchDir adds a directory and all its subdirectories to the watch list.
func (w *Watcher) WatchDir(root string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	gi := gitignore.Load(root)
	if gi != nil {
		w.matchers[root] = gi
	}
	return w.watchRecursive(root, gi)
}

func (w *Watcher) watchRecursive(root string, gi *gitignore.Matcher) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			rel, _ := filepath.Rel(root, path)
			if rel != "." && strings.Count(rel, string(filepath.Separator)) >= maxWatchDepth {
				return filepath.SkipDir
			}
			if isPathSkipped(path) {
				return filepath.SkipDir
			}
			if gi != nil && rel != "." && rel != "" {
				parts := strings.Split(filepath.ToSlash(rel), "/")
				if gi.Match(parts, true) {
					return filepath.SkipDir
				}
			}
			if len(w.dirs) >= maxWatchedDirs {
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

	delete(w.matchers, root)
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

func (w *Watcher) isGitIgnored(path string, isDir bool) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()

	clean := filepath.Clean(path)
	for root, gi := range w.matchers {
		if gi == nil {
			continue
		}
		if strings.HasPrefix(clean, root) {
			rel, err := filepath.Rel(root, clean)
			if err == nil && rel != "." && rel != "" {
				parts := strings.Split(filepath.ToSlash(rel), "/")
				if gi.Match(parts, isDir) {
					return true
				}
			}
		}
	}
	return false
}

func (w *Watcher) handleEvent(event fsnotify.Event) {
	if isPathSkipped(event.Name) {
		return
	}

	fi, err := os.Stat(event.Name)
	isDir := err == nil && fi.IsDir()

	if w.isGitIgnored(event.Name, isDir) {
		return
	}

	evt := events.Event{
		Data: map[string]interface{}{
			"path": event.Name,
		},
	}

	switch {
	case event.Op&fsnotify.Create != 0:
		evt.Type = events.FileCreated
		// Watch newly created directories
		if isDir && !isPathSkipped(event.Name) && !w.isGitIgnored(event.Name, true) {
			w.mu.Lock()
			_ = w.watcher.Add(event.Name)
			w.dirs[event.Name] = true
			w.mu.Unlock()
		}

	case event.Op&fsnotify.Write != 0:
		evt.Type = events.FileChanged

	case event.Op&fsnotify.Remove != 0:
		evt.Type = events.FileDeleted
		w.mu.Lock()
		delete(w.dirs, event.Name)
		w.mu.Unlock()

	case event.Op&fsnotify.Rename != 0:
		// A rename where the path no longer exists is a rename-away (delete);
		// otherwise the file appeared here (create). The frontend pairs a
		// delete + create by content to follow open tabs across renames.
		if _, err := os.Stat(event.Name); err == nil {
			evt.Type = events.FileCreated
		} else {
			evt.Type = events.FileDeleted
		}
		w.mu.Lock()
		delete(w.dirs, event.Name)
		w.mu.Unlock()

	case event.Op&fsnotify.Chmod != 0:
		return
	}

	w.bus.Publish(evt)
}

// maxWatchDepth limits how deep we recurse when adding dirs.
const maxWatchDepth = 8

// maxWatchedDirs caps the total number of directories we watch to avoid FD exhaustion.
const maxWatchedDirs = 500

func isSkipped(name string) bool {
	return ignore.Name(name)
}

func isPathSkipped(path string) bool {
	if path == "" {
		return false
	}
	clean := filepath.ToSlash(path)
	parts := strings.Split(clean, "/")
	for _, part := range parts {
		if part != "" && isSkipped(part) {
			return true
		}
	}
	return false
}
