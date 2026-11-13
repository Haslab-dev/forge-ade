package board

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const BoardInstructionsMarkdown = `# Forge ADE Workflow Kanban Board

Forge ADE includes an integrated project Workflow Kanban Board for tracking tasks, bugs, features, and backlogs.
The board state is stored in this repository at:
` + "`.forge/board_tasks.json`" + `

## How to Manage Tasks
Forge ADE monitors ` + "`.forge/board_tasks.json`" + ` with a real-time file watcher. Any edits you make to this file will immediately reflect on the user's Kanban board UI.

- **To view tasks**: Read ` + "`.forge/board_tasks.json`" + `.
- **To create a task**: Append a new task object to the JSON array in ` + "`.forge/board_tasks.json`" + `.
- **To move/update a task**: Find the task by ` + "`key`" + ` (e.g. ` + "`ADE-1`" + `) or ` + "`id`" + ` and change its ` + "`status`" + `, ` + "`priority`" + `, ` + "`title`" + `, or ` + "`description`" + `.
- **To complete a task**: Set its ` + "`status`" + ` to ` + "`\"done\"`" + `.
- **To delete a task**: Remove the task object from the array in ` + "`.forge/board_tasks.json`" + `.

## Task Object Schema
| Field | Type | Required | Values / Description |
|---|---|---|---|
| ` + "`title`" + ` | string | **Yes** | Clear, concise title of the task |
| ` + "`description`" + ` | string | No | Detailed description, requirements, or steps |
| ` + "`status`" + ` | string | No | ` + "`\"todo\"`" + ` (default), ` + "`\"in_progress\"`" + `, ` + "`\"in_review\"`" + `, ` + "`\"in_testing\"`" + `, ` + "`\"done\"`" + ` |
| ` + "`priority`" + ` | string | No | ` + "`\"low\"`" + `, ` + "`\"medium\"`" + ` (default), ` + "`\"high\"`" + `, ` + "`\"urgent\"`" + ` |
| ` + "`assignee`" + ` | string | No | Assignee name (default: ` + "`\"Pandu-Micro\"`" + `) |
| ` + "`labels`" + ` | string[] | No | Array of tags, e.g. ` + "`[\"bug\", \"frontend\"]`" + ` |
| ` + "`key`" + ` | string | No | E.g. ` + "`\"ADE-1\"`" + `. Auto-generated if omitted |
| ` + "`id`" + ` | string | No | Unique ID. Auto-generated if omitted |

## Example Task Entry
` + "```json" + `
[
  {
    "key": "ADE-1",
    "title": "Fix navbar dropdown on mobile screens",
    "description": "Dropdown menu overflows viewport on screens < 768px",
    "status": "todo",
    "priority": "high",
    "assignee": "Pandu-Micro",
    "labels": ["bug", "ui"]
  }
]
` + "```" + `
`

// WatcherOptions defines the configuration for a board Watcher.
type WatcherOptions struct {
	ResolveDir func(wsFolder string) string
	OnChanged  func(tasks []Task)
}

// Watcher monitors .forge/board_tasks.json for changes made by external agents,
// scripts, or users, and triggers real-time updates.
type Watcher struct {
	resolveDir    func(wsFolder string) string
	onChanged     func(tasks []Task)
	watcher       *fsnotify.Watcher
	mu            sync.Mutex
	watchedDir    string
	watchedPath   string
	lastHash      string
	stopCh        chan struct{}
	debounceTimer *time.Timer
	isClosed      bool
}

// NewWatcher creates and starts a new board file-system watcher.
func NewWatcher(opts WatcherOptions) (*Watcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create board fsnotify watcher: %w", err)
	}

	resolve := opts.ResolveDir
	if resolve == nil {
		resolve = func(wsFolder string) string {
			if wsFolder == "" {
				home, _ := os.UserHomeDir()
				return filepath.Join(home, ".forge", "board")
			}
			return filepath.Join(wsFolder, ".forge")
		}
	}

	bw := &Watcher{
		resolveDir: resolve,
		onChanged:  opts.OnChanged,
		watcher:    w,
		stopCh:     make(chan struct{}),
	}

	go bw.run()
	return bw, nil
}

func (bw *Watcher) run() {
	for {
		select {
		case <-bw.stopCh:
			return
		case event, ok := <-bw.watcher.Events:
			if !ok {
				return
			}
			bw.handleEvent(event)
		case err, ok := <-bw.watcher.Errors:
			if !ok {
				return
			}
			if err != nil {
				log.Printf("[board_watcher] error: %v", err)
			}
		}
	}
}

func (bw *Watcher) handleEvent(event fsnotify.Event) {
	bw.mu.Lock()
	targetPath := bw.watchedPath
	isClosed := bw.isClosed
	bw.mu.Unlock()

	if isClosed || targetPath == "" {
		return
	}

	// We only monitor changes to board_tasks.json
	base := filepath.Base(event.Name)
	if base != "board_tasks.json" {
		return
	}

	if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) == 0 {
		return
	}

	bw.mu.Lock()
	defer bw.mu.Unlock()

	if bw.debounceTimer != nil {
		bw.debounceTimer.Stop()
	}
	bw.debounceTimer = time.AfterFunc(150*time.Millisecond, func() {
		bw.processFileChange()
	})
}

// WatchWorkspace points the watcher to the workspace's .forge board directory.
func (bw *Watcher) WatchWorkspace(folders []string, fallbackDir string) {
	if bw == nil || bw.watcher == nil {
		return
	}
	bw.mu.Lock()
	defer bw.mu.Unlock()

	if bw.isClosed {
		return
	}

	var wsFolder string
	if len(folders) > 0 && folders[0] != "" {
		wsFolder = folders[0]
	} else {
		wsFolder = fallbackDir
	}

	dir := bw.resolveDir(wsFolder)
	if dir == "" {
		return
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("[board_watcher] failed to mkdir %s: %v", dir, err)
		return
	}

	filePath := filepath.Join(dir, "board_tasks.json")
	bw.ensureBoardFiles(dir, filePath)

	// If already watching this directory, just update hash
	if bw.watchedDir == dir {
		if data, err := os.ReadFile(filePath); err == nil {
			bw.lastHash = HashBytes(data)
		}
		return
	}

	// Unwatch previous directory if exists
	if bw.watchedDir != "" {
		_ = bw.watcher.Remove(bw.watchedDir)
		bw.watchedDir = ""
		bw.watchedPath = ""
	}

	// Add watch to the directory
	if err := bw.watcher.Add(dir); err != nil {
		log.Printf("[board_watcher] failed to watch %s: %v", dir, err)
		return
	}

	bw.watchedDir = dir
	bw.watchedPath = filePath

	if data, err := os.ReadFile(filePath); err == nil {
		bw.lastHash = HashBytes(data)
	}
	log.Printf("[board_watcher] watching %s for board changes", filePath)
}

func (bw *Watcher) ensureBoardFiles(dir string, filePath string) {
	// 1. Ensure board_tasks.json exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		legacyPath := filepath.Join(dir, "jira_tasks.json")
		if legacyData, err2 := os.ReadFile(legacyPath); err2 == nil {
			_ = os.WriteFile(filePath, legacyData, 0644)
			_ = os.Remove(legacyPath)
		} else {
			_ = os.WriteFile(filePath, []byte("[]\n"), 0644)
		}
	}

	// 2. Ensure board_instructions.md exists
	instructionsPath := filepath.Join(dir, "board_instructions.md")
	if _, err := os.Stat(instructionsPath); os.IsNotExist(err) {
		_ = os.WriteFile(instructionsPath, []byte(BoardInstructionsMarkdown), 0644)
	}
}

// UpdateLastHash records the hash of a write performed by internal Go APIs,
// preventing duplicate events or echo loops.
func (bw *Watcher) UpdateLastHash(data []byte) {
	if bw == nil {
		return
	}
	bw.mu.Lock()
	bw.lastHash = HashBytes(data)
	bw.mu.Unlock()
}

func (bw *Watcher) processFileChange() {
	bw.mu.Lock()
	filePath := bw.watchedPath
	lastHash := bw.lastHash
	bw.mu.Unlock()

	if filePath == "" {
		return
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		time.Sleep(50 * time.Millisecond)
		data, err = os.ReadFile(filePath)
		if err != nil {
			return
		}
	}

	currentHash := HashBytes(data)
	if currentHash == lastHash {
		// Content unchanged or matches internal write
		return
	}

	trimmed := strings.TrimSpace(string(data))
	var tasks []Task
	if trimmed == "" || trimmed == "[]" {
		tasks = []Task{}
	} else {
		if err := json.Unmarshal(data, &tasks); err != nil {
			// Retry once after 100ms in case write is in-progress
			time.Sleep(100 * time.Millisecond)
			if retryData, retryErr := os.ReadFile(filePath); retryErr == nil {
				if json.Unmarshal(retryData, &tasks) != nil {
					log.Printf("[board_watcher] invalid JSON in %s: %v", filePath, err)
					return
				}
				data = retryData
				currentHash = HashBytes(data)
			} else {
				return
			}
		}
	}

	// Normalize tasks (auto-assign ADE-X key, id, timestamps, defaults if omitted by agent)
	normalized, modified := NormalizeTasks(tasks)
	if modified {
		if formatted, err := json.MarshalIndent(normalized, "", "  "); err == nil {
			newHash := HashBytes(formatted)
			bw.mu.Lock()
			bw.lastHash = newHash
			bw.mu.Unlock()

			_ = os.WriteFile(filePath, formatted, 0644)
			tasks = normalized
		} else {
			bw.mu.Lock()
			bw.lastHash = currentHash
			bw.mu.Unlock()
		}
	} else {
		bw.mu.Lock()
		bw.lastHash = currentHash
		bw.mu.Unlock()
	}

	if bw.onChanged != nil {
		log.Printf("[board_watcher] external change detected, notifying callback with %d tasks", len(tasks))
		bw.onChanged(tasks)
	}
}

// Stop terminates the board watcher and releases file descriptors.
func (bw *Watcher) Stop() {
	if bw == nil {
		return
	}
	bw.mu.Lock()
	if bw.isClosed {
		bw.mu.Unlock()
		return
	}
	bw.isClosed = true
	if bw.debounceTimer != nil {
		bw.debounceTimer.Stop()
	}
	bw.mu.Unlock()

	close(bw.stopCh)
	if bw.watcher != nil {
		_ = bw.watcher.Close()
	}
}

// HashBytes computes SHA-256 hash string for byte slice.
func HashBytes(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
