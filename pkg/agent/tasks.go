package agent

import (
	"fmt"
	"sync"
)

// TaskStatus defines task execution states.
const (
	TaskPending    = "pending"
	TaskInProgress = "in_progress"
	TaskCompleted  = "completed"
)

// TaskItem represents an atomic task in the agent's goal checklist.
type TaskItem struct {
	ID         string
	Title      string
	Status     string
	ActiveForm string
}

// TaskManager coordinates live task decomposition and checklist state.
type TaskManager struct {
	mu         sync.RWMutex
	Goal       string
	Tasks      []TaskItem
	IsExpanded bool
}

// NewTaskManager creates an empty TaskManager.
func NewTaskManager() *TaskManager {
	return &TaskManager{
		IsExpanded: true,
		Tasks:      make([]TaskItem, 0),
	}
}

// SetGoal updates the high-level objective and tasks.
func (tm *TaskManager) SetGoal(goal string, tasks []TaskItem) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.Goal = goal
	tm.Tasks = tasks
	tm.IsExpanded = true
}

// ToggleTask flips a task between completed and pending.
func (tm *TaskManager) ToggleTask(id string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	for i := range tm.Tasks {
		if tm.Tasks[i].ID == id {
			if tm.Tasks[i].Status == TaskCompleted {
				tm.Tasks[i].Status = TaskPending
			} else {
				tm.Tasks[i].Status = TaskCompleted
			}
			return
		}
	}
}

// Clear removes all tasks and goal.
func (tm *TaskManager) Clear() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.Goal = ""
	tm.Tasks = nil
	tm.IsExpanded = false
}

// SetTasks replaces the current checklist with newly planned tasks.
func (tm *TaskManager) SetTasks(tasks []TaskItem) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.Tasks = tasks
}

// UpdateTaskStatus modifies a specific task item's status.
func (tm *TaskManager) UpdateTaskStatus(id, status string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	for i := range tm.Tasks {
		if tm.Tasks[i].ID == id {
			tm.Tasks[i].Status = status
			break
		}
	}
}

// Progress returns completed tasks count and total tasks count.
func (tm *TaskManager) Progress() (completed, total int) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	total = len(tm.Tasks)
	for _, t := range tm.Tasks {
		if t.Status == TaskCompleted {
			completed++
		}
	}
	return completed, total
}

// ProgressText returns a formatted completion string e.g. "3/5 tasks done".
func (tm *TaskManager) ProgressText() string {
	c, tot := tm.Progress()
	if tot == 0 {
		return ""
	}
	return fmt.Sprintf("%d/%d completed", c, tot)
}

// ToggleExpanded flips the collapsed state of the checklist.
func (tm *TaskManager) ToggleExpanded() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.IsExpanded = !tm.IsExpanded
}

// GetTasks returns a copy of current tasks.
func (tm *TaskManager) GetTasks() []TaskItem {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	out := make([]TaskItem, len(tm.Tasks))
	copy(out, tm.Tasks)
	return out
}
