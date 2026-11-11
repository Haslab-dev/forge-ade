package ui

import (
	"fmt"
	"sync"

	"github.com/egoist/mygo/ui"
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

// TaskManager coordinates live task decomposition and checklist rendering.
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
	}
}

// SetGoal updates the active goal and replaces existing tasks.
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
}

// Progress calculates completed / total ratio.
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

// RenderChecklist renders the floating task checklist card above the composer.
func (tm *TaskManager) RenderChecklist(c *ui.Context) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if len(tm.Tasks) == 0 {
		return
	}

	t := c.Theme()
	completed, total := tm.Progress()

	ui.Column(c).Margin(8, 16).Padding(10, 14).Radius(8).Background(t.Surface).BorderWidth(1).BorderColor(t.Border).Children(func() {
		// Header row with Goal & toggle
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				TablerIcon(c, TablerChecklist, 13)
				ui.Text(c, "ACTIVE GOAL").FontSize(10).Bold().TextColor(t.TextMuted)
			})
			ui.Box(c).Grow(1)
			ui.Text(c, fmt.Sprintf("%d/%d completed", completed, total)).FontSize(11).TextColor(t.Accent)
			toggleBtn := ui.Button(c, "")
			toggleBtn.Children(func() {
				if tm.IsExpanded {
					TablerIcon(c, TablerChevronUp, 12)
				} else {
					TablerIcon(c, TablerChevronDown, 12)
				}
			})
			if toggleBtn.Clicked() {
				tm.IsExpanded = !tm.IsExpanded
			}
		})

		if tm.Goal != "" {
			ui.Text(c, tm.Goal).Bold().FontSize(13).Padding(2, 0, 6, 0)
		}

		if tm.IsExpanded {
			ui.Column(c).Gap(4).Padding(4, 0).Children(func() {
				for _, task := range tm.Tasks {
					tid := task.ID
					isDone := task.Status == TaskCompleted
					isCurrent := task.Status == TaskInProgress

					ui.Row(c.Key("task-"+tid)).Gap(8).AlignItems(ui.Center).Children(func() {
						checkBtn := ui.Button(c, "")
						checkBtn.Children(func() {
							if isDone {
								TablerIcon(c, TablerCircleCheck, 13)
							} else if isCurrent {
								TablerIcon(c, TablerLoader, 13)
							} else {
								TablerIcon(c, TablerCircle, 13)
							}
						})
						if checkBtn.Clicked() {
							go tm.ToggleTask(tid)
						}

						textElement := ui.Text(c, task.Title).FontSize(12)
						if isDone {
							textElement.TextColor(t.TextMuted)
						} else if isCurrent {
							textElement.Bold().TextColor(t.Accent)
						}

						if isCurrent && task.ActiveForm != "" {
							ui.Box(c).Grow(1)
							ui.Text(c, task.ActiveForm).FontSize(11).TextColor(t.TextMuted)
						}
					})
				}
			})
		}
	})
}
