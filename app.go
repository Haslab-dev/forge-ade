package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/hasdev/forge-ade/internal/ai"
	"github.com/hasdev/forge-ade/internal/events"
	"github.com/hasdev/forge-ade/internal/explorer"
	"github.com/hasdev/forge-ade/internal/git"
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
	terminalMgr  *terminal.Manager
	gitMgr       *git.Manager
	agentMgr     *ai.AgentManager
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
	termMgr := terminal.NewManager(bus)
	gm := git.NewManager(bus)
	am := ai.NewAgentManager(bus)
	sm := search.NewSearchManager(bus)

	app := &App{
		bus:          bus,
		workspaceMgr: wsMgr,
		explorer:     exp,
		terminalMgr:  termMgr,
		gitMgr:       gm,
		agentMgr:     am,
		searchMgr:    sm,
		fileWatcher:  fileWatcher,
		dataDir:      dataDir,
	}

	// Wire up event handlers
	app.setupEventHandlers()

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
func (a *App) CloseWorkspace() {
	if ws := a.workspaceMgr.Current(); ws != nil {
		a.fileWatcher.Stop()
		a.terminalMgr.CloseAll()
		a.agentMgr.StopAll()
	}
	a.workspaceMgr.Close()
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
func (a *App) PinRecent(path string, pinned bool) {
	a.workspaceMgr.PinRecent(path, pinned)
}

// RemoveRecent removes a recent project entry.
func (a *App) RemoveRecent(path string) {
	a.workspaceMgr.RemoveRecent(path)
}

// ---------------------------------------------------------------------------
// Workspace Info (serializable for frontend)
// ---------------------------------------------------------------------------

// WorkspaceInfo is a JSON-safe workspace summary for the frontend.
type WorkspaceInfo struct {
	Name        string   `json:"name"`
	Folders     []string `json:"folders"`
	IsTemporary bool     `json:"isTemporary"`
	FilePath    string   `json:"filePath,omitempty"`
	Theme       string   `json:"theme"`
}

// GetWorkspaceInfo returns a JSON-safe summary of the current workspace.
func (a *App) GetWorkspaceInfo() *WorkspaceInfo {
	ws := a.workspaceMgr.Current()
	if ws == nil {
		return nil
	}
	return &WorkspaceInfo{
		Name:        ws.GetName(),
		Folders:     ws.GetFolders(),
		IsTemporary: ws.IsTemporary,
		FilePath:    ws.FilePath,
		Theme:       ws.Settings.Theme,
	}
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

// ReadFile reads and returns a file's content as a string.
func (a *App) ReadFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}
	return string(data), nil
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

// ---------------------------------------------------------------------------
// Terminal API
// ---------------------------------------------------------------------------

// CreateTerminal creates a new terminal session.
func (a *App) CreateTerminal(name, shell, cwd string) (*terminal.Session, error) {
	return a.terminalMgr.Create(name, shell, cwd)
}

// WriteTerminal writes data to a terminal session.
func (a *App) WriteTerminal(id string, data string) error {
	_, err := a.terminalMgr.Write(id, []byte(data))
	return err
}

// ResizeTerminal resizes a terminal session.
func (a *App) ResizeTerminal(id string, rows, cols uint16) error {
	return a.terminalMgr.Resize(id, rows, cols)
}

// CloseTerminal closes a terminal session.
func (a *App) CloseTerminal(id string) error {
	return a.terminalMgr.Close(id)
}

// ListTerminals returns all active terminal sessions.
func (a *App) ListTerminals() []*terminal.Session {
	return a.terminalMgr.List()
}

// ---------------------------------------------------------------------------
// Git API
// ---------------------------------------------------------------------------

// DiscoverRepos discovers git repos in the workspace.
func (a *App) DiscoverRepos() {
	folders := a.workspaceMgr.Current().GetFolders()
	a.gitMgr.Discover(folders)
}

// GetRepoStatus returns the git status for all repositories.
func (a *App) GetRepoStatus() map[string][]git.StatusEntry {
	result := make(map[string][]git.StatusEntry)
	for _, repo := range a.gitMgr.ListRepos() {
		status, err := repo.GetStatus()
		if err == nil {
			result[repo.Path] = status
		}
	}
	return result
}

// GetBranches returns branches for a repository.
func (a *App) GetBranches(repoPath string) ([]git.Branch, error) {
	for _, repo := range a.gitMgr.ListRepos() {
		if repo.Path == repoPath {
			return repo.GetBranches()
		}
	}
	return nil, fmt.Errorf("repository not found: %s", repoPath)
}

// GetCommits returns commit history for a repository.
func (a *App) GetCommits(repoPath string, count int) ([]git.Commit, error) {
	for _, repo := range a.gitMgr.ListRepos() {
		if repo.Path == repoPath {
			return repo.GetCommits(count)
		}
	}
	return nil, fmt.Errorf("repository not found: %s", repoPath)
}

// GitStage stages files in a repository.
func (a *App) GitStage(repoPath string, paths ...string) error {
	for _, repo := range a.gitMgr.ListRepos() {
		if repo.Path == repoPath {
			return repo.Stage(paths...)
		}
	}
	return fmt.Errorf("repository not found: %s", repoPath)
}

// GitUnstage unstages files in a repository.
func (a *App) GitUnstage(repoPath string, paths ...string) error {
	for _, repo := range a.gitMgr.ListRepos() {
		if repo.Path == repoPath {
			return repo.Unstage(paths...)
		}
	}
	return fmt.Errorf("repository not found: %s", repoPath)
}

// GitCommit creates a commit in a repository.
func (a *App) GitCommit(repoPath string, message string) error {
	for _, repo := range a.gitMgr.ListRepos() {
		if repo.Path == repoPath {
			return repo.Commit(message)
		}
	}
	return fmt.Errorf("repository not found: %s", repoPath)
}

// GitStageAll stages all changes in a repository.
func (a *App) GitStageAll(repoPath string) error {
	for _, repo := range a.gitMgr.ListRepos() {
		if repo.Path == repoPath {
			return repo.StageAll()
		}
	}
	return fmt.Errorf("repository not found: %s", repoPath)
}

// GitRunCommand runs a raw git command in a repository.
func (a *App) GitRunCommand(repoPath string, args string) (string, error) {
	for _, repo := range a.gitMgr.ListRepos() {
		if repo.Path == repoPath {
			// Split args by spaces
			return repo.RunGitCommand(splitArgs(args)...)
		}
	}
	return "", fmt.Errorf("repository not found: %s", repoPath)
}

// ---------------------------------------------------------------------------
// AI Agent API
// ---------------------------------------------------------------------------

// StartAgent launches an AI agent.
func (a *App) StartAgent(name, provider, workspace string) (*ai.Agent, error) {
	return a.agentMgr.Start(name, ai.ProviderType(provider), workspace)
}

// StopAgent stops an AI agent.
func (a *App) StopAgent(id string) error {
	return a.agentMgr.Stop(id)
}

// ListAgents returns all AI agents.
func (a *App) ListAgents() []*ai.Agent {
	return a.agentMgr.List()
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

// SearchContent searches file contents (inverted index).
func (a *App) SearchContent(query string, limit int) ([]search.RankedResult, error) {
	return a.searchMgr.SearchContent(query, limit)
}

// ---------------------------------------------------------------------------
// Event Bus for Frontend
// ---------------------------------------------------------------------------

// Subscribe subscribes to a frontend event channel.
func (a *App) Subscribe(eventType string) {
	// The frontend uses Wails runtime.EventsOn for real-time updates
	// This method exists to configure which events to listen for
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

// ---------------------------------------------------------------------------
// Internal
// ---------------------------------------------------------------------------

func (a *App) onWorkspaceOpened(ws *workspace.Workspace) {
	folders := ws.GetFolders()
	a.explorer.SetRoots(folders)
	a.searchMgr.SetDirectories(folders)
	a.gitMgr.Discover(folders)

	// Start file watcher
	a.fileWatcher.Start()
	for _, folder := range folders {
		_ = a.fileWatcher.WatchDir(folder)
	}

	// Start search index
	a.searchMgr.Start()
}

func (a *App) setupEventHandlers() {
	// When files change, update search index incrementally
	a.bus.Subscribe(events.FileCreated, func(e events.Event) {
		path, _ := e.Data["path"].(string)
		if path != "" {
			a.searchMgr.IndexFile(path)
		}
	})
	a.bus.Subscribe(events.FileChanged, func(e events.Event) {
		path, _ := e.Data["path"].(string)
		if path != "" {
			a.searchMgr.RemoveFile(path)
			a.searchMgr.IndexFile(path)
		}
	})
	a.bus.Subscribe(events.FileDeleted, func(e events.Event) {
		path, _ := e.Data["path"].(string)
		if path != "" {
			a.searchMgr.RemoveFile(path)
		}
	})

	// Terminal output → forward to frontend
	a.bus.Subscribe(events.TerminalOutput, func(e events.Event) {
		// Wails runtime.EventsEmit will be used from main.go
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
	a.terminalMgr.CloseAll()
	a.agentMgr.StopAll()
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

func splitArgs(input string) []string {
	// Simple space-based split; doesn't handle quoted args
	var args []string
	current := ""
	inQuote := false
	for _, r := range input {
		switch {
		case r == '"':
			inQuote = !inQuote
		case r == ' ' && !inQuote:
			if current != "" {
				args = append(args, current)
				current = ""
			}
		default:
			current += string(r)
		}
	}
	if current != "" {
		args = append(args, current)
	}
	return args
}
