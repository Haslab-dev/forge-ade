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
// Jira Board API & Storage
// ---------------------------------------------------------------------------

// JiraTask represents a workflow board task item.
type JiraTask struct {
	ID            string   `json:"id"`
	Key           string   `json:"key"` // e.g. ADE-1
	Title         string   `json:"title"`
	Description   string   `json:"description,omitempty"`
	Status        string   `json:"status"` // todo, in_progress, in_review, in_testing, done
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
	jiraMu      sync.Mutex
	jiraCounter int64
)

func (a *App) resolveJiraDir(wsFolder string) string {
	if wsFolder == "" && a != nil && a.workspaceMgr != nil {
		wsFolder = a.defaultFolder()
	}
	if wsFolder == "" {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".forge", "jira")
	}
	return filepath.Join(wsFolder, ".forge")
}

func (a *App) getJiraTasksFilePath(wsFolder string) string {
	dir := a.resolveJiraDir(wsFolder)
	return filepath.Join(dir, "jira_tasks.json")
}

// ListJiraTasks returns all tasks on the Jira board for the specified workspace.
func (a *App) ListJiraTasks(wsFolder string) ([]JiraTask, error) {
	jiraMu.Lock()
	defer jiraMu.Unlock()

	filePath := a.getJiraTasksFilePath(wsFolder)
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []JiraTask{}, nil
		}
		return nil, err
	}

	var tasks []JiraTask
	if err := json.Unmarshal(data, &tasks); err != nil {
		return []JiraTask{}, nil
	}

	return tasks, nil
}

// GetJiraTask retrieves a single task by ID or Key (e.g. ADE-1).
func (a *App) GetJiraTask(wsFolder string, taskIDOrKey string) (JiraTask, error) {
	tasks, err := a.ListJiraTasks(wsFolder)
	if err != nil {
		return JiraTask{}, err
	}
	for _, t := range tasks {
		if t.ID == taskIDOrKey || strings.EqualFold(t.Key, taskIDOrKey) {
			return t, nil
		}
	}
	return JiraTask{}, fmt.Errorf("task not found: %s", taskIDOrKey)
}

// SaveJiraTask inserts or updates a task on the Jira board.
func (a *App) SaveJiraTask(wsFolder string, task JiraTask) (JiraTask, error) {
	jiraMu.Lock()
	defer jiraMu.Unlock()

	dir := a.resolveJiraDir(wsFolder)
	_ = os.MkdirAll(dir, 0755)
	filePath := filepath.Join(dir, "jira_tasks.json")

	var tasks []JiraTask
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
		count := atomic.AddInt64(&jiraCounter, 1)
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
		// New task placed at the beginning
		tasks = append([]JiraTask{task}, tasks...)
	}

	bytes, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return task, err
	}
	if err := os.WriteFile(filePath, bytes, 0644); err != nil {
		return task, err
	}

	a.emitEvent("jira:updated", tasks)
	return task, nil
}

// DeleteJiraTask removes a task by ID or Key.
func (a *App) DeleteJiraTask(wsFolder string, taskIDOrKey string) error {
	jiraMu.Lock()
	defer jiraMu.Unlock()

	dir := a.resolveJiraDir(wsFolder)
	filePath := filepath.Join(dir, "jira_tasks.json")

	var tasks []JiraTask
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	_ = json.Unmarshal(data, &tasks)

	filtered := make([]JiraTask, 0, len(tasks))
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

	a.emitEvent("jira:updated", filtered)
	return nil
}

// MoveJiraTaskStatus updates the status of a task and emits a real-time event.
func (a *App) MoveJiraTaskStatus(wsFolder string, taskIDOrKey string, status string) (JiraTask, error) {
	jiraMu.Lock()
	defer jiraMu.Unlock()

	dir := a.resolveJiraDir(wsFolder)
	filePath := filepath.Join(dir, "jira_tasks.json")

	var tasks []JiraTask
	data, err := os.ReadFile(filePath)
	if err != nil {
		return JiraTask{}, err
	}
	if err := json.Unmarshal(data, &tasks); err != nil {
		return JiraTask{}, err
	}

	status = strings.ToLower(status)
	switch status {
	case "todo", "in_progress", "in_review", "in_testing", "done":
	default:
		return JiraTask{}, fmt.Errorf("invalid status %q (allowed: todo, in_progress, in_review, in_testing, done)", status)
	}

	var updated JiraTask
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
		return JiraTask{}, fmt.Errorf("task not found: %s", taskIDOrKey)
	}

	bytes, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return updated, err
	}
	if err := os.WriteFile(filePath, bytes, 0644); err != nil {
		return updated, err
	}

	a.emitEvent("jira:updated", tasks)
	return updated, nil
}
