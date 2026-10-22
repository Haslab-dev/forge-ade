package app

import (
	"github.com/hasdev/forge-ade/internal/board"
)

// BoardWatcher monitors .forge/board_tasks.json for changes made by external agents,
// scripts, or users, and triggers real-time UI updates on the Kanban board.
type BoardWatcher struct {
	watcher *board.Watcher
	app     *App
}

func newBoardWatcher(app *App) (*BoardWatcher, error) {
	w, err := board.NewWatcher(board.WatcherOptions{
		ResolveDir: func(wsFolder string) string {
			if app != nil {
				return app.resolveBoardDir(wsFolder)
			}
			return wsFolder
		},
		OnChanged: func(tasks []board.Task) {
			if app != nil {
				app.emitEvent(EventBoardUpdated, tasks)
			}
		},
	})
	if err != nil {
		return nil, err
	}
	return &BoardWatcher{watcher: w, app: app}, nil
}

// WatchWorkspace points the watcher to the workspace's .forge board directory.
func (bw *BoardWatcher) WatchWorkspace(folders []string) {
	if bw == nil || bw.watcher == nil {
		return
	}
	var defFolder string
	if bw.app != nil {
		defFolder = bw.app.defaultFolder()
	}
	bw.watcher.WatchWorkspace(folders, defFolder)
}

// UpdateLastHash records the hash of a write performed by internal Go APIs,
// preventing duplicate events or echo loops.
func (bw *BoardWatcher) UpdateLastHash(data []byte) {
	if bw == nil || bw.watcher == nil {
		return
	}
	bw.watcher.UpdateLastHash(data)
}

// Stop terminates the board watcher and releases file descriptors.
func (bw *BoardWatcher) Stop() {
	if bw == nil || bw.watcher == nil {
		return
	}
	bw.watcher.Stop()
}

// normalizeBoardTasks preserves backwards compatibility for internal tests.
func normalizeBoardTasks(tasks []BoardTask) ([]BoardTask, bool) {
	return board.NormalizeTasks(tasks)
}
