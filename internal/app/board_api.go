package app

import (
	"strings"

	"github.com/hasdev/forge-ade/internal/board"
)

// ---------------------------------------------------------------------------
// Workflow Board API (Wire contract adapter -> domain/board Manager)
// ---------------------------------------------------------------------------

// BoardTask represents a workflow board task item.
type BoardTask = board.Task

func (a *App) getBoardManager() *board.Manager {
	if a != nil && a.boardMgr != nil {
		return a.boardMgr
	}
	var defDir string
	if a != nil && a.workspaceMgr != nil {
		defDir = a.defaultFolder()
	}
	return board.NewManager(defDir)
}

func (a *App) resolveBoardDir(wsFolder string) string {
	if wsFolder == "" && a != nil && a.workspaceMgr != nil {
		wsFolder = a.defaultFolder()
	}
	return a.getBoardManager().ResolveBoardDir(wsFolder)
}

func (a *App) getBoardTasksFilePath(wsFolder string) string {
	if wsFolder == "" && a != nil && a.workspaceMgr != nil {
		wsFolder = a.defaultFolder()
	}
	return a.getBoardManager().GetBoardTasksFilePath(wsFolder)
}

// ListBoardTasks returns all tasks on the workflow board for the specified workspace.
func (a *App) ListBoardTasks(wsFolder string) ([]BoardTask, error) {
	if wsFolder == "" && a != nil && a.workspaceMgr != nil {
		wsFolder = a.defaultFolder()
	}
	return a.getBoardManager().ListTasks(wsFolder)
}

// GetBoardTask retrieves a single task by ID or Key (e.g. ADE-1).
func (a *App) GetBoardTask(wsFolder string, taskIDOrKey string) (BoardTask, error) {
	if wsFolder == "" && a != nil && a.workspaceMgr != nil {
		wsFolder = a.defaultFolder()
	}
	return a.getBoardManager().GetTask(wsFolder, taskIDOrKey)
}

// SaveBoardTask inserts or updates a task on the workflow board.
func (a *App) SaveBoardTask(wsFolder string, task BoardTask) (BoardTask, error) {
	if wsFolder == "" && a != nil && a.workspaceMgr != nil {
		wsFolder = a.defaultFolder()
	}
	mgr := a.getBoardManager()
	saved, err := mgr.SaveTask(wsFolder, task)
	if err != nil {
		return task, err
	}
	if a != nil {
		tasks, _ := mgr.ListTasks(wsFolder)
		a.emitEvent(EventBoardUpdated, tasks)
	}
	return saved, nil
}

// DeleteBoardTask removes a task by ID or Key.
func (a *App) DeleteBoardTask(wsFolder string, taskIDOrKey string) error {
	if wsFolder == "" && a != nil && a.workspaceMgr != nil {
		wsFolder = a.defaultFolder()
	}
	mgr := a.getBoardManager()
	if err := mgr.DeleteTask(wsFolder, taskIDOrKey); err != nil {
		return err
	}
	if a != nil {
		tasks, _ := mgr.ListTasks(wsFolder)
		a.emitEvent(EventBoardUpdated, tasks)
	}
	return nil
}

// MoveBoardTaskStatus updates the status of a task and emits a real-time event.
func (a *App) MoveBoardTaskStatus(wsFolder string, taskIDOrKey string, status string) (BoardTask, error) {
	if wsFolder == "" && a != nil && a.workspaceMgr != nil {
		wsFolder = a.defaultFolder()
	}
	status = strings.ToLower(status)
	mgr := a.getBoardManager()
	updated, err := mgr.MoveTaskStatus(wsFolder, taskIDOrKey, status)
	if err != nil {
		return BoardTask{}, err
	}
	if a != nil {
		tasks, _ := mgr.ListTasks(wsFolder)
		a.emitEvent(EventBoardUpdated, tasks)
	}
	return updated, nil
}
