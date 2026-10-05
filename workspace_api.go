package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hasdev/forge-ade/internal/index"
	"github.com/hasdev/forge-ade/internal/workspace"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// ---------------------------------------------------------------------------
// Workspace API
// ---------------------------------------------------------------------------

// defaultFolder returns the folder new sessions/shells should be scoped to:
// the first folder of the open workspace, or the home directory. The process
// working directory is deliberately never used — a Finder-launched app runs
// at "/" which made the agent operate on the root filesystem and emit
// absolute paths in git diffs.
func (a *App) defaultFolder() string {
	if ws := a.workspaceMgr.Current(); ws != nil {
		if folders := ws.GetFolders(); len(folders) > 0 && !strings.HasPrefix(folders[0], "ssh://") {
			return folders[0]
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return ""
}

// OpenFolder opens a folder as a temporary workspace.
func (a *App) OpenFolder(folderPath string) (*workspace.Workspace, error) {
	if strings.HasPrefix(folderPath, "ssh://") {
		ws := workspace.NewTemporary(folderPath)
		a.explorer.SetRoots([]string{folderPath})
		return ws, nil
	}
	ws, err := a.workspaceMgr.OpenFolder(folderPath)
	if err != nil {
		return nil, err
	}
	a.onWorkspaceOpened(ws)
	return ws, nil
}

// OpenWorkspace opens a .workspace file.
func (a *App) OpenWorkspace(filePath string) (*workspace.Workspace, error) {
	ws, err := a.workspaceMgr.OpenWorkspace(filePath)
	if err != nil {
		return nil, err
	}
	a.onWorkspaceOpened(ws)

	return ws, nil
}

// SaveWorkspace saves the current workspace.
func (a *App) SaveWorkspace() error {
	return a.workspaceMgr.SaveCurrent()
}

// SaveWorkspaceAs saves the current workspace to a new file.
func (a *App) SaveWorkspaceAs(filePath string) error {
	return a.workspaceMgr.SaveCurrentAs(filePath)
}

// CloseWorkspace closes the current workspace.
func (a *App) CloseWorkspace() error {
	if ws := a.workspaceMgr.Current(); ws != nil {
		a.fileWatcher.Stop()
		a.sessionMgr.StopAll()
	}
	a.watchedRoots = nil
	a.workspaceMgr.Close()
	return nil
}

// AddFolderToWorkspace adds a folder to the current workspace and wires it up.
func (a *App) AddFolderToWorkspace(folderPath string) error {
	ws := a.workspaceMgr.Current()
	if ws == nil {
		return fmt.Errorf("no workspace open")
	}
	absPath, err := filepath.Abs(folderPath)
	if err != nil {
		return err
	}
	for _, f := range ws.GetFolders() {
		if f == absPath {
			return nil
		}
	}
	ws.AddFolder(absPath)
	folders := ws.GetFolders()
	a.explorer.SetRoots(folders)
	a.searchMgr.SetDirectories(folders)
	_ = a.fileWatcher.WatchDir(absPath)
	a.watchedRoots = append(a.watchedRoots, absPath)
	return nil
}

// RemoveFolderFromWorkspace removes a folder from the current workspace.
func (a *App) RemoveFolderFromWorkspace(folderPath string) error {
	ws := a.workspaceMgr.Current()
	if ws == nil {
		return fmt.Errorf("no workspace open")
	}
	ws.RemoveFolder(folderPath)
	folders := ws.GetFolders()
	a.explorer.SetRoots(folders)
	a.searchMgr.SetDirectories(folders)
	_ = a.fileWatcher.UnwatchDir(folderPath)
	for i, root := range a.watchedRoots {
		if root == folderPath {
			a.watchedRoots = append(a.watchedRoots[:i], a.watchedRoots[i+1:]...)
			break
		}
	}
	return nil
}

// GetCurrentWorkspace returns the current workspace.
func (a *App) GetCurrentWorkspace() *workspace.Workspace {
	return a.workspaceMgr.Current()
}

// GetRecentProjects returns recent workspaces and folders.
func (a *App) GetRecentProjects() []workspace.RecentEntry {
	return a.workspaceMgr.GetRecent()
}

// PinRecent pins a recent project.
func (a *App) PinRecent(path string, pinned bool) error {
	a.workspaceMgr.PinRecent(path, pinned)
	return nil
}

// RemoveRecent removes a recent project entry.
func (a *App) RemoveRecent(path string) error {
	a.workspaceMgr.RemoveRecent(path)
	return nil
}

// ---------------------------------------------------------------------------
// Native File Dialogs
// ---------------------------------------------------------------------------

// OpenFolderDialog opens a native directory picker and returns the selected path.
func (a *App) OpenFolderDialog() (string, error) {
	app := application.Get()
	if app == nil {
		return "", fmt.Errorf("app not initialized")
	}
	return app.Dialog.OpenFile().
		CanChooseDirectories(true).
		CanChooseFiles(false).
		SetTitle("Open Folder").
		PromptForSingleSelection()
}

// OpenWorkspaceDialog opens a native file picker for .workspace files.
func (a *App) OpenWorkspaceDialog() (string, error) {
	app := application.Get()
	if app == nil {
		return "", fmt.Errorf("app not initialized")
	}
	return app.Dialog.OpenFile().
		CanChooseFiles(true).
		SetTitle("Open Workspace").
		AddFilter("Workspace Files", "*.workspace").
		PromptForSingleSelection()
}

// OpenFileDialog opens a native file picker for any file.
func (a *App) OpenFileDialog() (string, error) {
	app := application.Get()
	if app == nil {
		return "", fmt.Errorf("app not initialized")
	}
	return app.Dialog.OpenFile().
		CanChooseFiles(true).
		SetTitle("Open File").
		PromptForSingleSelection()
}

// SaveWorkspaceDialog opens a native save dialog for .workspace files.
func (a *App) SaveWorkspaceDialog() (string, error) {
	app := application.Get()
	if app == nil {
		return "", fmt.Errorf("app not initialized")
	}
	return app.Dialog.SaveFile().
		SetMessage("Save Workspace As").
		SetFilename("my-project.workspace").
		AddFilter("Workspace Files", "*.workspace").
		PromptForSingleSelection()
}

// ---------------------------------------------------------------------------
// Internal
// ---------------------------------------------------------------------------

func (a *App) onWorkspaceOpened(ws *workspace.Workspace) {
	// onWorkspaceOpened runs from NewApp's restored workspace and from the
	// OpenFolder/OpenWorkspace bindings; serialize so a concurrent open can't
	// nil out state a previous call's goroutine is still using.
	a.workspaceMu.Lock()
	defer a.workspaceMu.Unlock()

	folders := ws.GetFolders()
	a.explorer.SetRoots(folders)
	a.searchMgr.SetDirectories(folders)

	// Unwatch the previous workspace first: without this, switching workspaces
	// kept streaming the old workspace's file events into the UI and let the
	// old workspace consume the watcher's directory budget.
	for _, root := range a.watchedRoots {
		_ = a.fileWatcher.UnwatchDir(root)
	}
	a.watchedRoots = a.watchedRoots[:0]

	// Rebuild the workspace symbol index (FWI). Indexes the first folder;
	// multi-root workspaces index each folder with its own store.
	if a.indexUnsub != nil {
		a.indexUnsub()
		a.indexUnsub = nil
	}
	a.indexStore = nil
	if len(folders) > 0 {
		a.agentMgr.SetDefaultFolder(folders[0])
		a.skillMgr.SetWorkspace(folders[0])
		if a.pluginMgr != nil {
			a.pluginMgr.SetWorkspace(folders[0])
		}
		if a.memoryMgr != nil {
			a.memoryMgr.SetWorkspace(folders[0])
		}
		idx := index.New(folders[0])
		_ = idx.Load()
		a.indexStore = idx
		a.indexUnsub = idx.Listen(a.bus)
		// Build the captured store, never a.indexStore: a re-open may nil or
		// replace the field while this build is still running.
		go func() {
			_ = idx.Build()
			_ = idx.Save()
		}()
	}

	// Start file watcher
	a.fileWatcher.Start()
	for _, folder := range folders {
		_ = a.fileWatcher.WatchDir(folder)
		a.watchedRoots = append(a.watchedRoots, folder)
	}

	// Start search index
	a.searchMgr.Start()
}
