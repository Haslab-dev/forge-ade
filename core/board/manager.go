package board

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// NormalizeTasks standardizes a slice of tasks, ensuring valid keys, IDs,
// default values, and timestamps. Returns the normalized slice and whether
// any changes were made.
var globalTaskCounter int64

// maxADEKeyNumber scans task keys and returns the highest ADE-<n> number.
// The suffix is parsed with strconv (after trimming), so keys written by
// other tools or hand-edited files — "ade-12", "ADE- 7", " ADE-99 " — still
// count and newly generated keys never collide.
func maxADEKeyNumber(tasks []Task) int {
	maxNum := 0
	for _, t := range tasks {
		key := strings.ToUpper(strings.TrimSpace(t.Key))
		if !strings.HasPrefix(key, "ADE-") {
			continue
		}
		if n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(key, "ADE-"))); err == nil && n > maxNum {
			maxNum = n
		}
	}
	return maxNum
}

func NormalizeTasks(tasks []Task) ([]Task, bool) {
	modified := false
	now := time.Now().UnixMilli()

	maxNum := maxADEKeyNumber(tasks)

	result := make([]Task, len(tasks))
	for i, t := range tasks {
		task := t

		if task.Key == "" {
			maxNum++
			task.Key = fmt.Sprintf("ADE-%d", maxNum)
			modified = true
		}
		if task.ID == "" {
			count := atomic.AddInt64(&globalTaskCounter, 1)
			task.ID = fmt.Sprintf("task-%d-%d", time.Now().UnixNano(), count)
			modified = true
		}
		if task.Status == "" {
			task.Status = "todo"
			modified = true
		} else {
			lowStatus := strings.ToLower(task.Status)
			switch lowStatus {
			case "todo", "in_progress", "in_review", "in_testing", "done":
				if task.Status != lowStatus {
					task.Status = lowStatus
					modified = true
				}
			default:
				task.Status = "todo"
				modified = true
			}
		}

		if task.Priority == "" {
			task.Priority = "medium"
			modified = true
		} else {
			lowPri := strings.ToLower(task.Priority)
			switch lowPri {
			case "low", "medium", "high", "urgent":
				if task.Priority != lowPri {
					task.Priority = lowPri
					modified = true
				}
			default:
				task.Priority = "medium"
				modified = true
			}
		}

		if task.Assignee == "" {
			task.Assignee = "Pandu-Micro"
			modified = true
		}

		if task.CreatedAt == 0 {
			task.CreatedAt = now
			modified = true
		}
		if task.UpdatedAt == 0 {
			task.UpdatedAt = now
			modified = true
		}
		if task.Status == "done" && task.CompletedAt == 0 {
			task.CompletedAt = now
			modified = true
		}

		result[i] = task
	}

	return result, modified
}

type UpdateCallback func(tasks []Task, rawBytes []byte)

// Manager manages the persistence and lifecycle of workflow board tasks.
type Manager struct {
	defaultDir string
	mu         sync.Mutex
	counter    int64
	onUpdated  UpdateCallback
}

// NewManager creates a new workflow board domain Manager.
func NewManager(defaultDir string) *Manager {
	return &Manager{
		defaultDir: defaultDir,
	}
}

// SetOnUpdated sets the listener callback when tasks are mutated.
func (m *Manager) SetOnUpdated(cb UpdateCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onUpdated = cb
}

// ResolveBoardDir returns the directory where board_tasks.json lives.
func (m *Manager) ResolveBoardDir(wsFolder string) string {
	if wsFolder == "" {
		if m.defaultDir != "" {
			wsFolder = m.defaultDir
		} else {
			home, _ := os.UserHomeDir()
			return filepath.Join(home, ".forge-ade", "board")
		}
	}
	return filepath.Join(wsFolder, ".forge")
}

// GetBoardTasksFilePath returns the absolute path to board_tasks.json for a workspace.
func (m *Manager) GetBoardTasksFilePath(wsFolder string) string {
	dir := m.ResolveBoardDir(wsFolder)
	return filepath.Join(dir, "board_tasks.json")
}

// quarantineCorruptBoardFile renames a corrupt board_tasks.json aside with a
// nanosecond timestamp suffix so the raw bytes survive for manual recovery
// instead of being destroyed by the next write. Returns the backup path
// ("" on failure).
func quarantineCorruptBoardFile(filePath string) string {
	backup := fmt.Sprintf("%s.corrupt-%d", filePath, time.Now().UnixNano())
	if err := os.Rename(filePath, backup); err != nil {
		return ""
	}
	return backup
}

// ListTasks returns all tasks on the workflow board for the specified workspace.
func (m *Manager) ListTasks(wsFolder string) ([]Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	filePath := m.GetBoardTasksFilePath(wsFolder)
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// Migrate legacy path if exists
			legacyPath := filepath.Join(m.ResolveBoardDir(wsFolder), "jira_tasks.json")
			if legacyData, err2 := os.ReadFile(legacyPath); err2 == nil {
				var legacyTasks []Task
				if json.Unmarshal(legacyData, &legacyTasks) == nil {
					_ = os.WriteFile(filePath, legacyData, 0644)
					_ = os.Remove(legacyPath)
					return legacyTasks, nil
				}
			}
			return []Task{}, nil
		}
		return nil, err
	}

	var tasks []Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		// Corrupt or mid-write file: never treat it as an empty board (the
		// next write would wipe it). Quarantine the raw bytes for recovery.
		backup := quarantineCorruptBoardFile(filePath)
		if backup != "" {
			return []Task{}, fmt.Errorf("board file corrupt; preserved as %s: %w", backup, err)
		}
		return []Task{}, fmt.Errorf("board file corrupt (could not preserve): %w", err)
	}

	return tasks, nil
}

// GetTask retrieves a single task by ID or Key (e.g. ADE-1).
func (m *Manager) GetTask(wsFolder string, taskIDOrKey string) (Task, error) {
	tasks, err := m.ListTasks(wsFolder)
	if err != nil {
		return Task{}, err
	}
	for _, t := range tasks {
		if t.ID == taskIDOrKey || strings.EqualFold(t.Key, taskIDOrKey) {
			return t, nil
		}
	}
	return Task{}, fmt.Errorf("task not found: %s", taskIDOrKey)
}

// SaveTask inserts or updates a task on the workflow board. When the existing
// board file is corrupt, it is quarantined as .corrupt-<timestamp> and the
// save proceeds against a fresh list containing just the incoming task; a
// descriptive error tells the caller the previous file was preserved.
func (m *Manager) SaveTask(wsFolder string, task Task) (Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	dir := m.ResolveBoardDir(wsFolder)
	_ = os.MkdirAll(dir, 0755)
	filePath := filepath.Join(dir, "board_tasks.json")

	var tasks []Task
	var corruptBackup string
	if data, err := os.ReadFile(filePath); err == nil {
		if uerr := json.Unmarshal(data, &tasks); uerr != nil {
			// Never overwrite unreadable data: preserve the raw bytes, then
			// restart the board with the incoming task only.
			corruptBackup = quarantineCorruptBoardFile(filePath)
			tasks = nil
		}
	}

	now := time.Now().UnixMilli()

	// Normalize status
	switch strings.ToLower(task.Status) {
	case "todo", "in_progress", "in_review", "in_testing", "done":
		task.Status = strings.ToLower(task.Status)
	default:
		task.Status = "todo"
	}

	// Normalize priority
	switch strings.ToLower(task.Priority) {
	case "low", "medium", "high", "urgent":
		task.Priority = strings.ToLower(task.Priority)
	default:
		task.Priority = "medium"
	}

	if task.ID == "" {
		count := atomic.AddInt64(&globalTaskCounter, 1)
		task.ID = fmt.Sprintf("task-%d-%d", time.Now().UnixNano(), count)
	}

	if task.Key == "" {
		task.Key = fmt.Sprintf("ADE-%d", maxADEKeyNumber(tasks)+1)
	}

	if task.CreatedAt == 0 {
		task.CreatedAt = now
	}
	task.UpdatedAt = now
	if task.Status == "done" && task.CompletedAt == 0 {
		task.CompletedAt = now
	}

	if task.Assignee == "" {
		task.Assignee = "Pandu-Micro"
	}

	found := false
	for i, t := range tasks {
		if t.ID == task.ID || (t.Key != "" && strings.EqualFold(t.Key, task.Key)) {
			tasks[i] = task
			found = true
			break
		}
	}
	if !found {
		tasks = append([]Task{task}, tasks...)
	}

	bytes, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return task, err
	}

	if err := os.WriteFile(filePath, bytes, 0644); err != nil {
		return task, err
	}

	if m.onUpdated != nil {
		m.onUpdated(tasks, bytes)
	}

	if corruptBackup != "" {
		return task, fmt.Errorf("board file was corrupt and has been preserved as %s; board restarted with the saved task only", corruptBackup)
	}
	return task, nil
}

// DeleteTask removes a task by ID or Key.
func (m *Manager) DeleteTask(wsFolder string, taskIDOrKey string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	dir := m.ResolveBoardDir(wsFolder)
	filePath := filepath.Join(dir, "board_tasks.json")

	var tasks []Task
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := json.Unmarshal(data, &tasks); err != nil {
		// Corrupt file: quarantine it, never write over it.
		backup := quarantineCorruptBoardFile(filePath)
		if backup != "" {
			return fmt.Errorf("board file corrupt; preserved as %s: %w", backup, err)
		}
		return fmt.Errorf("board file corrupt (could not preserve): %w", err)
	}

	filtered := make([]Task, 0, len(tasks))
	for _, t := range tasks {
		if t.ID == taskIDOrKey || strings.EqualFold(t.Key, taskIDOrKey) {
			continue
		}
		filtered = append(filtered, t)
	}

	bytes, err := json.MarshalIndent(filtered, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(filePath, bytes, 0644); err != nil {
		return err
	}

	if m.onUpdated != nil {
		m.onUpdated(filtered, bytes)
	}

	return nil
}

// MoveTaskStatus updates the status of a task and invokes the update callback.
func (m *Manager) MoveTaskStatus(wsFolder string, taskIDOrKey string, status string) (Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	dir := m.ResolveBoardDir(wsFolder)
	filePath := filepath.Join(dir, "board_tasks.json")

	var tasks []Task
	data, err := os.ReadFile(filePath)
	if err != nil {
		return Task{}, err
	}
	if err := json.Unmarshal(data, &tasks); err != nil {
		// Corrupt file: quarantine it, never write over it.
		backup := quarantineCorruptBoardFile(filePath)
		if backup != "" {
			return Task{}, fmt.Errorf("board file corrupt; preserved as %s: %w", backup, err)
		}
		return Task{}, fmt.Errorf("board file corrupt (could not preserve): %w", err)
	}

	status = strings.ToLower(status)
	switch status {
	case "todo", "in_progress", "in_review", "in_testing", "done":
	default:
		return Task{}, fmt.Errorf("invalid status %q (allowed: todo, in_progress, in_review, in_testing, done)", status)
	}

	var updated Task
	found := false
	now := time.Now().UnixMilli()

	for i, t := range tasks {
		if t.ID == taskIDOrKey || strings.EqualFold(t.Key, taskIDOrKey) {
			tasks[i].Status = status
			tasks[i].UpdatedAt = now
			if status == "done" {
				tasks[i].CompletedAt = now
			}
			updated = tasks[i]
			found = true
			break
		}
	}

	if !found {
		return Task{}, fmt.Errorf("task not found: %s", taskIDOrKey)
	}

	bytes, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return updated, err
	}

	if err := os.WriteFile(filePath, bytes, 0644); err != nil {
		return updated, err
	}

	if m.onUpdated != nil {
		m.onUpdated(tasks, bytes)
	}

	return updated, nil
}
