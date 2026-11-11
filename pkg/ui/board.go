package ui

import (
	"sort"

	"github.com/hasdev/forge-ade/internal/board"
)

// Board columns in WorkflowBoardView.tsx order.
var boardColumns = []struct {
	Status string
	Name   string
}{
	{"todo", "To Do"},
	{"in_progress", "In Progress"},
	{"in_review", "In Review"},
	{"in_testing", "In Testing"},
	{"done", "Done"},
}

// BoardColumn is one kanban lane with its tasks.
type BoardColumn struct {
	Name  string
	Tasks []*board.Task
}

// BoardData is the loaded board: columns in display order.
type BoardData struct {
	Columns []BoardColumn
}

// BoardManager loads the workspace's .forge board tasks through the
// original forge-ade board manager.
type BoardManager struct {
	manager *board.Manager
	RootDir string
	Data    *BoardData
}

// NewBoardManager wraps forge-ade's board.Manager for the workspace.
func NewBoardManager(rootDir string) *BoardManager {
	return &BoardManager{
		manager: board.NewManager(rootDir),
		RootDir: rootDir,
	}
}

// Refresh reloads the board tasks from .forge/board_tasks.json.
func (b *BoardManager) Refresh() {
	if b == nil || b.manager == nil {
		return
	}
	tasks, err := b.manager.ListTasks(b.RootDir)
	data := &BoardData{Columns: make([]BoardColumn, 0, len(boardColumns))}
	if err == nil {
		byStatus := make(map[string][]*board.Task, len(boardColumns))
		for i := range tasks {
			t := tasks[i]
			byStatus[t.Status] = append(byStatus[t.Status], &t)
		}
		for _, col := range boardColumns {
			list := byStatus[col.Status]
			sort.SliceStable(list, func(i, j int) bool { return list[i].UpdatedAt > list[j].UpdatedAt })
			data.Columns = append(data.Columns, BoardColumn{Name: col.Name, Tasks: list})
		}
	}
	b.Data = data
}
