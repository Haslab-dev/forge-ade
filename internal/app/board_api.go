package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ---------------------------------------------------------------------------
// Workflow Board API & Storage
// ---------------------------------------------------------------------------

// BoardTask represents a workflow board task item.
type BoardTask struct {
	ID            string   `json:"id"`
	Key           string   `json:"key"` // e.g. ADE-1
	Title         string   `json:"title"`
	Description   string   `json:"description,omitempty"`
	Status        string   `json:"status"`   // todo, in_progress, in_review, in_testing, done
	Priority      string   `json:"priority"` // low, medium, high, urgent
	Assignee      string   `json:"assignee,omitempty"`
	Labels        []string `json:"labels,omitempty"`
	SessionID     string   `json:"sessionId,omitempty"`
	TaskKind      string   `json:"taskKind,omitempty"`
	WorkspacePath string   `json:"workspacePath,omitempty"`
	CreatedAt     int64    `json:"createdAt"`
	UpdatedAt     int64    `json:"updatedAt"`
	CompletedAt   int64    `json:"completedAt,omitempty"`
	Branch        string   `json:"branch,omitempty"`
	FilesTouched  int      `json:"filesTouched,omitempty"`
}

var (
	boardMu      sync.Mutex
	boardCounter int64
)

func (a *App) resolveBoardDir(wsFolder string) string {
	if wsFolder == "" && a != nil && a.workspaceMgr != nil {
		wsFolder = a.defaultFolder()
	}
	if wsFolder == "" {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".forge", "board")
	}
	return filepath.Join(wsFolder, ".forge")
}

func (a *App) getBoardTasksFilePath(wsFolder string) string {
	dir := a.resolveBoardDir(wsFolder)
	return filepath.Join(dir, "board_tasks.json")
}

// ListBoardTasks returns all tasks on the workflow board for the specified workspace.
func (a *App) ListBoardTasks(wsFolder string) ([]BoardTask, error) {
	boardMu.Lock()
	defer boardMu.Unlock()

	filePath := a.getBoardTasksFilePath(wsFolder)
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// Migrate legacy path if exists
			legacyPath := filepath.Join(a.resolveBoardDir(wsFolder), "jira_tasks.json")
			if legacyData, err2 := os.ReadFile(legacyPath); err2 == nil {
				var legacyTasks []BoardTask
				if json.Unmarshal(legacyData, &legacyTasks) == nil {
					_ = os.WriteFile(filePath, legacyData, 0644)
					_ = os.Remove(legacyPath)
					return legacyTasks, nil
				}
			}
			return []BoardTask{}, nil
		}
		return nil, err
	}

	var tasks []BoardTask
	if err := json.Unmarshal(data, &tasks); err != nil {
		return []BoardTask{}, nil
	}

	return tasks, nil
}

// GetBoardTask retrieves a single task by ID or Key (e.g. ADE-1).
func (a *App) GetBoardTask(wsFolder string, taskIDOrKey string) (BoardTask, error) {
	tasks, err := a.ListBoardTasks(wsFolder)
	if err != nil {
		return BoardTask{}, err
	}
	for _, t := range tasks {
		if t.ID == taskIDOrKey || strings.EqualFold(t.Key, taskIDOrKey) {
			return t, nil
		}
	}
	return BoardTask{}, fmt.Errorf("task not found: %s", taskIDOrKey)
}

// SaveBoardTask inserts or updates a task on the workflow board.
func (a *App) SaveBoardTask(wsFolder string, task BoardTask) (BoardTask, error) {
	boardMu.Lock()
	defer boardMu.Unlock()

	dir := a.resolveBoardDir(wsFolder)
	_ = os.MkdirAll(dir, 0755)
	filePath := filepath.Join(dir, "board_tasks.json")

	var tasks []BoardTask
	if data, err := os.ReadFile(filePath); err == nil {
		_ = json.Unmarshal(data, &tasks)
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
		count := atomic.AddInt64(&boardCounter, 1)
		task.ID = fmt.Sprintf("task-%d-%d", now, count)
	}

	if task.Key == "" {
		maxNum := 0
		for _, t := range tasks {
			var num int
			if _, err := fmt.Sscanf(t.Key, "ADE-%d", &num); err == nil {
				if num > maxNum {
					maxNum = num
				}
			}
		}
		task.Key = fmt.Sprintf("ADE-%d", maxNum+1)
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
		tasks = append([]BoardTask{task}, tasks...)
	}

	bytes, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return task, err
	}
	if a != nil && a.boardWatcher != nil {
		a.boardWatcher.UpdateLastHash(bytes)
	}
	if err := os.WriteFile(filePath, bytes, 0644); err != nil {
		return task, err
	}

	a.emitEvent("board:updated", tasks)
	return task, nil
}

// DeleteBoardTask removes a task by ID or Key.
func (a *App) DeleteBoardTask(wsFolder string, taskIDOrKey string) error {
	boardMu.Lock()
	defer boardMu.Unlock()

	dir := a.resolveBoardDir(wsFolder)
	filePath := filepath.Join(dir, "board_tasks.json")

	var tasks []BoardTask
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	_ = json.Unmarshal(data, &tasks)

	filtered := make([]BoardTask, 0, len(tasks))
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
	if a != nil && a.boardWatcher != nil {
		a.boardWatcher.UpdateLastHash(bytes)
	}
	if err := os.WriteFile(filePath, bytes, 0644); err != nil {
		return err
	}

	a.emitEvent("board:updated", filtered)
	return nil
}

// MoveBoardTaskStatus updates the status of a task and emits a real-time event.
func (a *App) MoveBoardTaskStatus(wsFolder string, taskIDOrKey string, status string) (BoardTask, error) {
	boardMu.Lock()
	defer boardMu.Unlock()

	dir := a.resolveBoardDir(wsFolder)
	filePath := filepath.Join(dir, "board_tasks.json")

	var tasks []BoardTask
	data, err := os.ReadFile(filePath)
	if err != nil {
		return BoardTask{}, err
	}
	if err := json.Unmarshal(data, &tasks); err != nil {
		return BoardTask{}, err
	}

	status = strings.ToLower(status)
	switch status {
	case "todo", "in_progress", "in_review", "in_testing", "done":
	default:
		return BoardTask{}, fmt.Errorf("invalid status %q (allowed: todo, in_progress, in_review, in_testing, done)", status)
	}

	var updated BoardTask
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
		return BoardTask{}, fmt.Errorf("task not found: %s", taskIDOrKey)
	}

	bytes, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return updated, err
	}
	if a != nil && a.boardWatcher != nil {
		a.boardWatcher.UpdateLastHash(bytes)
	}
	if err := os.WriteFile(filePath, bytes, 0644); err != nil {
		return updated, err
	}

	a.emitEvent("board:updated", tasks)
	return updated, nil
}
