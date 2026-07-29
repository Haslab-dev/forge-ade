package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"

	"github.com/hasdev/forge-ade/internal/events"
	"github.com/hasdev/forge-ade/internal/explorer"
	"github.com/hasdev/forge-ade/internal/search"
	"github.com/hasdev/forge-ade/internal/terminal"
	"github.com/hasdev/forge-ade/internal/watcher"
	"github.com/hasdev/forge-ade/internal/workspace"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the main application struct that exposes methods to the frontend via Wails.
type App struct {
	ctx          context.Context
	bus          *events.Bus
	workspaceMgr *workspace.Manager
	explorer     *explorer.Explorer
	sessionMgr   *terminal.Manager
	searchMgr    *search.SearchManager
	fileWatcher  *watcher.Watcher
	dataDir      string
}

// NewApp creates a new App and initializes all subsystems.
func NewApp() *App {
	dataDir := getDataDir()

	bus := events.NewBus()

	wsMgr, err := workspace.NewManager(dataDir)
	if err != nil {
		log.Fatalf("failed to init workspace manager: %v", err)
	}

	fileWatcher, err := watcher.New(bus)
	if err != nil {
		log.Fatalf("failed to init file watcher: %v", err)
	}

	exp := explorer.New(bus)
	sm := terminal.NewManager(bus)
	si := search.NewSearchManager()

	app := &App{
		bus:          bus,
		workspaceMgr: wsMgr,
		explorer:     exp,
		sessionMgr:   sm,
		searchMgr:    si,
		fileWatcher:  fileWatcher,
		dataDir:      dataDir,
	}

	// Wire up event handlers
	app.setupEventHandlers()

	if ws := wsMgr.Current(); ws != nil {
		app.onWorkspaceOpened(ws)
	}

	return app
}

// ---------------------------------------------------------------------------
// Workspace API
// ---------------------------------------------------------------------------

// OpenFolder opens a folder as a temporary workspace.
func (a *App) OpenFolder(folderPath string) (*workspace.Workspace, error) {
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
// Explorer API
// ---------------------------------------------------------------------------

// GetFileTree returns the file tree for explorer roots.
func (a *App) GetFileTree(depth int) (string, error) {
	tree, err := a.explorer.GetTree(depth)
	if err != nil {
		return "", err
	}
	data, _ := json.Marshal(tree)
	return string(data), nil
}

// ListDirectory lists a directory's contents.
func (a *App) ListDirectory(dirPath string) (string, error) {
	entries, err := a.explorer.ListDirectory(dirPath)
	if err != nil {
		return "", err
	}
	data, _ := json.Marshal(entries)
	return string(data), nil
}

// ExpandPath expands a directory in the tree and returns siblings.
func (a *App) ExpandPath(targetPath string) (string, error) {
	entries, err := a.explorer.ExpandPath(targetPath)
	if err != nil {
		return "", err
	}
	data, _ := json.Marshal(entries)
	return string(data), nil
}

// ToggleHiddenFiles toggles hidden file visibility in the explorer.
func (a *App) ToggleHiddenFiles() bool {
	current := a.explorer.GetShowHidden()
	a.explorer.SetShowHidden(!current)
	return !current
}

// ---------------------------------------------------------------------------
// File Operations (Read, Write, Create, Delete)
// ---------------------------------------------------------------------------

// GetHomeDir returns the user's home directory.
func (a *App) GetHomeDir() string {
	home, _ := os.UserHomeDir()
	return home
}

// OpenInFinder opens a path in macOS Finder (selects file or opens directory).
func (a *App) OpenInFinder(path string) error {
	cmd := exec.Command("open", "-R", path)
	return cmd.Run()
}

// IsDir checks if a path is a directory.
func (a *App) IsDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// ReadFile reads and returns a file's content as a string.
func (a *App) ReadFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}
	return string(data), nil
}

// ReadFileBase64 reads a binary file and returns base64-encoded content.
func (a *App) ReadFileBase64(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

// WriteFile writes content to a file, creating it if needed.
func (a *App) WriteFile(path string, content string) error {
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

// CreateFile creates a new empty file.
func (a *App) CreateFile(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	f.Close()
	return nil
}

// DeleteFile removes a file or empty directory.
func (a *App) DeleteFile(path string) error {
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("delete file: %w", err)
	}
	return nil
}

// RenameFile renames or moves a file.
func (a *App) RenameFile(oldPath, newPath string) error {
	if err := os.Rename(oldPath, newPath); err != nil {
		return fmt.Errorf("rename file: %w", err)
	}
	return nil
}

// CopyFile copies a file from src to dst.
func (a *App) CopyFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("copy file read: %w", err)
	}
	if err := os.WriteFile(dst, input, 0644); err != nil {
		return fmt.Errorf("copy file write: %w", err)
	}
	return nil
}

// MoveFile moves a file from src to dst (alias for RenameFile).
func (a *App) MoveFile(src, dst string) error {
	return os.Rename(src, dst)
}

// ---------------------------------------------------------------------------
// Session API (Unified — Shell + AI Agents + Docker, etc.)
// ---------------------------------------------------------------------------

// CreateShell creates a new shell session.
func (a *App) CreateShell(name, folder string) (*terminal.Session, error) {
	return a.sessionMgr.CreateShell(name, folder)
}

// CreateAIAgent creates a new AI agent session.
func (a *App) CreateAIAgent(name, provider, folder string) (*terminal.Session, error) {
	return a.sessionMgr.CreateAIAgent(name, provider, folder)
}

// WriteSession writes data to a session's stdin.
func (a *App) WriteSession(id string, data string) error {
	_, err := a.sessionMgr.Write(id, []byte(data))
	return err
}

// ResizeSession resizes a session's PTY.
func (a *App) ResizeSession(id string, rows, cols uint16) error {
	return a.sessionMgr.Resize(id, rows, cols)
}

// StopSession terminates a session.
func (a *App) StopSession(id string) error {
	return a.sessionMgr.Stop(id)
}

// RenameSession renames a session.
func (a *App) RenameSession(id, name string) error {
	return a.sessionMgr.Rename(id, name)
}

// ListSessions returns all active sessions.
func (a *App) ListSessions() []*terminal.Session {
	return a.sessionMgr.List()
}

// ListShells returns only shell sessions.
func (a *App) ListShells() []*terminal.Session {
	return a.sessionMgr.ListByType(terminal.SessionShell)
}

// ListAIAgents returns only AI agent sessions.
func (a *App) ListAIAgents() []*terminal.Session {
	return a.sessionMgr.ListByType(terminal.SessionAI)
}

// ---------------------------------------------------------------------------
// Search API
// ---------------------------------------------------------------------------

// SearchFilename searches files by name (instant, trie-based).
func (a *App) SearchFilename(query string, limit int) ([]search.RankedResult, error) {
	results := a.searchMgr.SearchFilename(query, limit)
	if results == nil {
		return []search.RankedResult{}, nil
	}
	return results, nil
}

// SearchFilenameWithOptions searches files with options.
func (a *App) SearchFilenameWithOptions(opts search.SearchOptions) ([]search.RankedResult, error) {
	results := a.searchMgr.SearchFilenameWithOptions(opts)
	if results == nil {
		return []search.RankedResult{}, nil
	}
	return results, nil
}

// SearchContent searches file contents via ripgrep (on-demand).
func (a *App) SearchContent(query string, limit int) ([]search.RankedResult, error) {
	return a.searchMgr.SearchContent(query, limit)
}

// SearchContentWithOptions searches file contents with options.
func (a *App) SearchContentWithOptions(opts search.SearchOptions) ([]search.RankedResult, error) {
	return a.searchMgr.SearchContentWithOptions(opts)
}

// SearchSymbols searches code symbols (functions, types, structs, classes, interfaces).
func (a *App) SearchSymbols(query string, limit int) ([]search.RankedResult, error) {
	return a.searchMgr.SearchSymbols(query, limit)
}

// SearchSymbolsWithOptions searches code symbols with options.
func (a *App) SearchSymbolsWithOptions(opts search.SearchOptions) ([]search.RankedResult, error) {
	return a.searchMgr.SearchSymbolsWithOptions(opts)
}

// ---------------------------------------------------------------------------
// Event Bus for Frontend
// ---------------------------------------------------------------------------

// Subscribe subscribes to a frontend event channel.
func (a *App) Subscribe(eventType string) error {
	return nil
}

// ---------------------------------------------------------------------------
// Native File Dialogs
// ---------------------------------------------------------------------------

// OpenFolderDialog opens a native directory picker and returns the selected path.
func (a *App) OpenFolderDialog() (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("app not initialized")
	}
	path, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Open Folder",
	})
	if err != nil {
		return "", err
	}
	return path, nil
}

// OpenWorkspaceDialog opens a native file picker for .workspace files.
func (a *App) OpenWorkspaceDialog() (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("app not initialized")
	}
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Open Workspace",
		Filters: []runtime.FileFilter{
			{
				Pattern:     "*.workspace",
				DisplayName: "Workspace Files",
			},
		},
	})
	if err != nil {
		return "", err
	}
	return path, nil
}

// OpenFileDialog opens a native file picker for any file.
func (a *App) OpenFileDialog() (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("app not initialized")
	}
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Open File",
	})
	if err != nil {
		return "", err
	}
	return path, nil
}

// SaveWorkspaceDialog opens a native save dialog for .workspace files.
func (a *App) SaveWorkspaceDialog() (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("app not initialized")
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Save Workspace As",
		DefaultFilename: "my-project.workspace",
		Filters: []runtime.FileFilter{
			{
				Pattern:     "*.workspace",
				DisplayName: "Workspace Files",
			},
		},
	})
	if err != nil {
		return "", err
	}
	return path, nil
}

// ---------------------------------------------------------------------------
// Internal
// ---------------------------------------------------------------------------

func (a *App) onWorkspaceOpened(ws *workspace.Workspace) {
	folders := ws.GetFolders()
	a.explorer.SetRoots(folders)
	a.searchMgr.SetDirectories(folders)

	// Start file watcher
	a.fileWatcher.Start()
	for _, folder := range folders {
		_ = a.fileWatcher.WatchDir(folder)
	}

	// Start search index
	a.searchMgr.Start()
}

// ---------------------------------------------------------------------------
// File Sync — used by frontend to detect external file changes
// ---------------------------------------------------------------------------

var fsChangeCounter int64

// GetFsChangeCount returns a counter that increments on every fs change.
// Frontend polls this to detect external file changes.
func (a *App) GetFsChangeCount() int64 {
	return atomic.LoadInt64(&fsChangeCounter)
}

func (a *App) setupEventHandlers() {
	a.bus.Subscribe(events.FileCreated, func(e events.Event) {
		path, _ := e.Data["path"].(string)
		if path != "" {
			a.searchMgr.IndexFile(path)
			atomic.AddInt64(&fsChangeCounter, 1)
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "fs:changed", map[string]interface{}{
					"type": "created",
					"path": path,
				})
			}
		}
	})
	a.bus.Subscribe(events.FileChanged, func(e events.Event) {
		path, _ := e.Data["path"].(string)
		if path != "" {
			a.searchMgr.RemoveFile(path)
			a.searchMgr.IndexFile(path)
			atomic.AddInt64(&fsChangeCounter, 1)
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "fs:changed", map[string]interface{}{
					"type": "modified",
					"path": path,
				})
			}
		}
	})
	a.bus.Subscribe(events.FileDeleted, func(e events.Event) {
		path, _ := e.Data["path"].(string)
		if path != "" {
			a.searchMgr.RemoveFile(path)
			atomic.AddInt64(&fsChangeCounter, 1)
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "fs:changed", map[string]interface{}{
					"type": "deleted",
					"path": path,
				})
			}
		}
	})

	// Bridge terminal output to frontend via Wails runtime events
	a.bus.Subscribe(events.TerminalOutput, func(e events.Event) {
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "session:output", e.Data)
		}
	})
	a.bus.Subscribe(events.TerminalOpened, func(e events.Event) {
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "session:opened", e.Data)
		}
	})
	a.bus.Subscribe(events.TerminalClosed, func(e events.Event) {
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "session:closed", e.Data)
		}
	})
}

// Startup is called when the application starts up.
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	log.Println("ForgeADE started")
}

// Shutdown cleanly shuts down all subsystems.
func (a *App) Shutdown() {
	a.fileWatcher.Stop()
	a.sessionMgr.StopAll()
	a.searchMgr.Stop()
}

func getDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".forge-ade")
	}
	dir := filepath.Join(home, ".forge-ade")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return filepath.Join(".", ".forge-ade")
	}
	return dir
}
