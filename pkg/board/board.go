package board

import (
	"sort"

	internalboard "github.com/hasdev/forge-ade/internal/board"
)

// BoardColumnDefs defines board columns in WorkflowBoardView order.
var BoardColumnDefs = []struct {
	Status string
	Name   string
}{
	{"todo", "To Do"},
	{"in_progress", "In Progress"},
	{"in_review", "In Review"},
	{"in_testing", "In Testing"},
	{"done", "Done"},
}

// BoardColumn represents one kanban lane with its tasks.
type BoardColumn struct {
	Name  string
	Tasks []*internalboard.Task
}

// BoardData holds all loaded columns in display order.
type BoardData struct {
	Columns []BoardColumn
}

// BoardManager manages loading the workspace's kanban board tasks.
type BoardManager struct {
	manager *internalboard.Manager
	RootDir string
	Data    *BoardData
}

// NewBoardManager creates a new BoardManager for the given workspace root.
func NewBoardManager(rootDir string) *BoardManager {
	return &BoardManager{
		manager: internalboard.NewManager(rootDir),
		RootDir: rootDir,
	}
}

// Refresh reloads tasks from the board manager and sorts them by recency.
func (b *BoardManager) Refresh() {
	if b == nil || b.manager == nil {
		return
	}
	tasks, err := b.manager.ListTasks(b.RootDir)
	data := &BoardData{Columns: make([]BoardColumn, 0, len(BoardColumnDefs))}
	if err == nil {
		byStatus := make(map[string][]*internalboard.Task, len(BoardColumnDefs))
		for i := range tasks {
			t := tasks[i]
			byStatus[t.Status] = append(byStatus[t.Status], &t)
		}
		for _, col := range BoardColumnDefs {
			list := byStatus[col.Status]
			sort.SliceStable(list, func(i, j int) bool { return list[i].UpdatedAt > list[j].UpdatedAt })
			data.Columns = append(data.Columns, BoardColumn{Name: col.Name, Tasks: list})
		}
	}
	b.Data = data
}
