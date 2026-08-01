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
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/hasdev/forge-ade/internal/agent"
	"github.com/hasdev/forge-ade/internal/events"
	"github.com/hasdev/forge-ade/internal/explorer"
	"github.com/hasdev/forge-ade/internal/git"
	"github.com/hasdev/forge-ade/internal/llm"
	"github.com/hasdev/forge-ade/internal/mcp"
	"github.com/hasdev/forge-ade/internal/search"
	"github.com/hasdev/forge-ade/internal/skills"
	"github.com/hasdev/forge-ade/internal/terminal"
	"github.com/hasdev/forge-ade/internal/tools"
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

	llmClient *llm.LLMClient
	toolReg   *tools.Registry
	skillMgr  *skills.Manager
	mcpMgr    *mcp.Manager
	agentMgr  *agent.Manager
	gitEngine *git.Engine
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

	llmClient := llm.NewLLMClient(dataDir)
	toolReg := tools.NewRegistry(si)
	skillMgr := skills.NewManager()
	mcpMgr := mcp.NewManager(dataDir)
	agentMgr := agent.NewManager(llmClient, toolReg, skillMgr, mcpMgr, bus, dataDir)
	gitEngine := git.NewEngine()

	app := &App{
		bus:          bus,
		workspaceMgr: wsMgr,
		explorer:     exp,
		sessionMgr:   sm,
		searchMgr:    si,
		fileWatcher:  fileWatcher,
		dataDir:      dataDir,
		llmClient:    llmClient,
		toolReg:      toolReg,
		skillMgr:     skillMgr,
		mcpMgr:       mcpMgr,
		agentMgr:     agentMgr,
		gitEngine:    gitEngine,
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

// ResolvePath expands ~ and relative paths to an absolute, cleaned path.
// Returns the input unchanged if it cannot be resolved.
func (a *App) ResolvePath(path string) string {
	resolved, err := ResolvePath(path)
	if err != nil {
		return path
	}
	return resolved
}

// resolveWorkspacePath resolves relative paths (e.g. git file paths) against
// the first folder of the current workspace so file operations work from any CWD.
func (a *App) resolveWorkspacePath(path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	if ws := a.workspaceMgr.Current(); ws != nil && len(ws.GetFolders()) > 0 {
		joined := filepath.Join(ws.GetFolders()[0], path)
		if info, err := os.Stat(joined); err == nil && !info.IsDir() {
			return joined
		}
	}
	return path
}

// ReadFile reads and returns a file's content as a string.
func (a *App) ReadFile(path string) (string, error) {
	path = a.resolveWorkspacePath(path)
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

// DeleteFile removes a file or directory (recursively for directories).
func (a *App) DeleteFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	if info.IsDir() {
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("delete directory: %w", err)
		}
	} else {
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("delete file: %w", err)
		}
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
// Editor Language Tooling (lightweight — no LSP)
// ---------------------------------------------------------------------------

// SyntaxDiagnostic is a single error/warning from the lightweight syntax check.
type SyntaxDiagnostic struct {
	Line      int    `json:"line"`
	Column    int    `json:"column"`
	EndLine   int    `json:"end_line,omitempty"`
	EndColumn int    `json:"end_column,omitempty"`
	Message   string `json:"message"`
}

// findEsbuild resolves the esbuild binary. App-launched processes (from
// Finder/launchd) have a minimal PATH, so also probe common install locations.
func findEsbuild() string {
	if p, err := exec.LookPath("esbuild"); err == nil {
		return p
	}
	home, _ := os.UserHomeDir()
	candidates := []string{
		home + "/.bun/bin/esbuild",
		"/opt/homebrew/bin/esbuild",
		"/usr/local/bin/esbuild",
		"/usr/local/share/npm/bin/esbuild",
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c
		}
	}
	return ""
}

// CheckSyntax runs esbuild's parser over a JS/TS source string and returns
// syntax diagnostics (missing braces, invalid tokens, etc.). It does not do
// type checking — that requires a language server.
func (a *App) CheckSyntax(path, content string) ([]SyntaxDiagnostic, error) {
	// Only JS/TS-family files for now.
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs", ".mts", ".cts":
	default:
		return []SyntaxDiagnostic{}, nil
	}

	esbuildBin := findEsbuild()
	if esbuildBin == "" {
		return []SyntaxDiagnostic{}, nil // esbuild not installed — silently no-op
	}

	tmpFile, err := os.CreateTemp("", "forge-syntax-*"+ext)
	if err != nil {
		return nil, fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)
	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("write temp: %w", err)
	}
	tmpFile.Close()

	ctx, cancel := context.WithTimeout(a.ctx, 8*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, esbuildBin, tmpPath, "--format=esm", "--log-level=warning")
	out, _ := cmd.CombinedOutput()

	var diags []SyntaxDiagnostic
	// esbuild error shape:
	//   ✘ [ERROR] Expected identifier but found end of file
	//       ../var/folders/.../forge-syntax-123.ts:1:11:
	//         1 │ const x = {
	//           ╵            ^
	// Column in esbuild output is 1-based, but esbuild reports it as 0-based
	// in the "line:col:" marker — we subtract 1 to get the 0-based position.
	lines := strings.Split(string(out), "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if strings.Contains(line, "[ERROR]") {
			msg := strings.TrimSpace(strings.TrimPrefix(line, "✘ [ERROR]"))
			diags = append(diags, SyntaxDiagnostic{Message: msg})
			continue
		}
		if len(diags) == 0 || !strings.Contains(line, ":") {
			continue
		}
		// Location line: "<path>:<line>:<col>:"
		// Match the LAST two colon-separated numbers.
		re := regexp.MustCompile(`:(\d+):(\d+):\s*$`)
		m := re.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		ln, _ := strconv.Atoi(m[1])
		col, _ := strconv.Atoi(m[2])
		if ln > 0 {
			diags[len(diags)-1].Line = ln
			diags[len(diags)-1].Column = col
		}
	}
	return diags, nil
}

// findPrettier resolves the prettier binary. Prefers the project's local
// frontend/node_modules install, falling back to PATH.
func findPrettier() string {
	cwd, _ := os.Getwd()
	candidates := []string{
		cwd + "/frontend/node_modules/.bin/prettier",
		cwd + "/node_modules/.bin/prettier",
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c
		}
	}
	if p, err := exec.LookPath("prettier"); err == nil {
		return p
	}
	return ""
}

// findPrettierConfig resolves the project .prettierrc file.
func findPrettierConfig() string {
	cwd, _ := os.Getwd()
	for _, name := range []string{".prettierrc", ".prettierrc.js", ".prettierrc.cjs", ".prettierrc.json", ".prettierrc.yaml", "prettier.config.js", "prettier.config.cjs"} {
		p := filepath.Join(cwd, name)
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p
		}
	}
	return ""
}

// FormatCode formats a JS/TS source string using the project's prettier
// (with its .prettierrc config when present). Returns the formatted source,
// or the original content if prettier is unavailable.
func (a *App) FormatCode(path, content string) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	parser := ""
	switch ext {
	case ".js", ".mjs", ".cjs":
		parser = "babel"
	case ".jsx":
		parser = "babel"
	case ".ts", ".mts", ".cts":
		parser = "typescript"
	case ".tsx":
		parser = "typescript"
	case ".json":
		parser = "json"
	case ".css":
		parser = "css"
	case ".html":
		parser = "html"
	case ".md":
		parser = "markdown"
	default:
		return content, nil
	}

	prettierBin := findPrettier()
	if prettierBin == "" {
		return content, nil
	}

	tmpFile, err := os.CreateTemp("", "forge-format-*"+ext)
	if err != nil {
		return content, nil
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)
	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		return content, nil
	}
	tmpFile.Close()

	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Second)
	defer cancel()

	args := []string{tmpPath, "--write", "--parser", parser}
	if cfg := findPrettierConfig(); cfg != "" {
		args = append(args, "--config", cfg)
	}
	cmd := exec.CommandContext(ctx, prettierBin, args...)
	// Run from the frontend dir so prettier can resolve plugins installed
	// there (e.g. prettier-plugin-organize-imports).
	cwd, _ := os.Getwd()
	frontendDir := filepath.Join(cwd, "frontend")
	if info, err := os.Stat(frontendDir); err == nil && info.IsDir() {
		cmd.Dir = frontendDir
	} else {
		cmd.Dir = cwd
	}
	if err := cmd.Run(); err != nil {
		return content, nil
	}
	formatted, err := os.ReadFile(tmpPath)
	if err != nil {
		return content, nil
	}
	return string(formatted), nil
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
	a.bus.Subscribe(events.FileRenamed, func(e events.Event) {
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

	// Bridge agent updates to frontend
	a.bus.Subscribe("agent:updated", func(e events.Event) {
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "agent:updated", e.Data)
		}
	})
	a.bus.Subscribe("agent:config:changed", func(e events.Event) {
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "agent:config:changed", e.Data)
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

// ---------------------------------------------------------------------------
// Agent & LLM & Git API
// ---------------------------------------------------------------------------

// CreateAgentSession creates a new agent session with specified role filter.
func (a *App) CreateAgentSession(name string, role string, folder string) (*agent.Session, error) {
	return a.agentMgr.CreateSession(name, agent.RoleFilter(role), folder)
}

// ListAgentSessions returns all active agent sessions.
func (a *App) ListAgentSessions() []*agent.Session {
	return a.agentMgr.ListSessions()
}

// GetAgentSession returns a single agent session by ID.
func (a *App) GetAgentSession(id string) (*agent.Session, error) {
	sess, ok := a.agentMgr.GetSession(id)
	if !ok {
		return nil, fmt.Errorf("session not found")
	}
	return sess, nil
}

// UpdateAgentSession updates editable agent session fields (name, role, custom prompt, custom rules).
func (a *App) UpdateAgentSession(id string, name string, role string, customPrompt string, customRules string) (*agent.Session, error) {
	return a.agentMgr.UpdateSession(id, name, agent.RoleFilter(role), customPrompt, customRules)
}

// ListAgentDefinitions returns all pre-configured agent definitions.
func (a *App) ListAgentDefinitions() []agent.AgentDefinition {
	return a.agentMgr.ListAgentDefinitions()
}

// SaveAgentDefinition creates or updates a pre-configured agent definition.
func (a *App) SaveAgentDefinition(def agent.AgentDefinition) (agent.AgentDefinition, error) {
	return a.agentMgr.SaveAgentDefinition(def)
}

// DeleteAgentDefinition removes a pre-configured agent definition.
func (a *App) DeleteAgentDefinition(id string) error {
	return a.agentMgr.DeleteAgentDefinition(id)
}

// CreateAgentSessionFromDefinition creates a chat session from a pre-configured
// agent definition, scoped to the given project folder.
func (a *App) CreateAgentSessionFromDefinition(defID string, folder string) (*agent.Session, error) {
	return a.agentMgr.CreateSessionFromDefinition(defID, folder)
}

// SendAgentMessage sends a message to an agent session with optional @ file mentions.
func (a *App) SendAgentMessage(sessionID string, content string, mentionedPaths []string) error {
	return a.agentMgr.SendMessage(a.ctx, sessionID, content, mentionedPaths)
}

// RespondAgentApproval responds to a pending tool execution approval.
func (a *App) RespondAgentApproval(sessionID string, approve bool, autoApproveAll bool) error {
	return a.agentMgr.RespondApproval(a.ctx, sessionID, approve, autoApproveAll)
}

// DeleteAgentSession deletes an agent session.
func (a *App) DeleteAgentSession(id string) error {
	a.agentMgr.DeleteSession(id)
	return nil
}

// ToggleAgentTask toggles completion status of a task item.
func (a *App) ToggleAgentTask(sessionID string, taskID string, completed bool) error {
	a.agentMgr.ToggleTask(sessionID, taskID, completed)
	return nil
}

// GetProviderProfiles gets all configured provider profiles.
func (a *App) GetProviderProfiles() []llm.ProviderProfile {
	return a.llmClient.GetProviderProfiles()
}

// SaveProviderProfiles saves configured provider profiles.
func (a *App) SaveProviderProfiles(profiles []llm.ProviderProfile) error {
	err := a.llmClient.SaveProviderProfiles(profiles)
	if err != nil {
		return err
	}
	a.emitAgentConfigChanged()
	return nil
}

// emitAgentConfigChanged notifies open agent chats that provider/model/agent
// config changed so they refresh their model list and active model.
func (a *App) emitAgentConfigChanged() {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "agent:config:changed", map[string]interface{}{})
}

// FetchProviderModels fetches model list from provider endpoint.
func (a *App) FetchProviderModels(apiKey string, baseURL string) ([]string, error) {
	return a.llmClient.FetchModels(a.ctx, apiKey, baseURL)
}

// SetActiveModel sets active LLM model.
func (a *App) SetActiveModel(providerID string, model string) error {
	a.llmClient.SetActiveModel(providerID, model)
	return nil
}

// SaveLLMProfile updates active LLM provider profile.
func (a *App) SaveLLMProfile(providerID, apiKey, baseURL, model string) error {
	return a.llmClient.SaveProfile(providerID, apiKey, baseURL, model)
}

// GetLLMConfig gets active LLM profile config.
func (a *App) GetLLMConfig() llm.Profile {
	return a.llmClient.GetConfig()
}

// ListLLMProviders lists all supported LLM providers.
func (a *App) ListLLMProviders() []llm.ProviderConfig {
	return a.llmClient.ListProviders()
}

// ListSkills returns loaded SKILL.md skills.
func (a *App) ListSkills() []skills.Skill {
	return a.skillMgr.List()
}

// ListMCPTools returns loaded MCP tools.
func (a *App) ListMCPTools() []mcp.Tool {
	return a.mcpMgr.ListTools()
}

// ListMCPServers returns configured MCP servers.
func (a *App) ListMCPServers() []mcp.ServerConfig {
	return a.mcpMgr.ListServers()
}

// SaveMCPServer creates or updates an MCP server (GUI-configured).
func (a *App) SaveMCPServer(s mcp.ServerConfig) (mcp.ServerConfig, error) {
	return a.mcpMgr.SaveServer(s)
}

// DeleteMCPServer removes an MCP server by name.
func (a *App) DeleteMCPServer(name string) error {
	return a.mcpMgr.DeleteServer(name)
}

// GetGitCommitGraph streams lightweight paginated Git commits with graph prefix.
func (a *App) GetGitCommitGraph(repoPath string, offset int, limit int) (*git.CommitGraphResult, error) {
	if repoPath == "" {
		if ws := a.workspaceMgr.Current(); ws != nil && len(ws.GetFolders()) > 0 {
			repoPath = ws.GetFolders()[0]
		} else {
			cwd, _ := os.Getwd()
			repoPath = cwd
		}
	}
	return a.gitEngine.GetCommitGraph(a.ctx, repoPath, offset, limit)
}

// GetGitCommitDiff returns details and patch for a single commit.
func (a *App) GetGitCommitDiff(repoPath string, hash string) (string, error) {
	if repoPath == "" {
		if ws := a.workspaceMgr.Current(); ws != nil && len(ws.GetFolders()) > 0 {
			repoPath = ws.GetFolders()[0]
		} else {
			cwd, _ := os.Getwd()
			repoPath = cwd
		}
	}
	return a.gitEngine.GetCommitDiff(a.ctx, repoPath, hash)
}

// GetGitFileDiff returns the unified diff of a single working-tree file against HEAD.
func (a *App) GetGitFileDiff(repoPath string, path string) (string, error) {
	if repoPath == "" {
		if ws := a.workspaceMgr.Current(); ws != nil && len(ws.GetFolders()) > 0 {
			repoPath = ws.GetFolders()[0]
		} else {
			cwd, _ := os.Getwd()
			repoPath = cwd
		}
	}
	return a.gitEngine.GetFileDiff(a.ctx, repoPath, path)
}

// GetGitCommitFileDiff returns the unified diff of a single file within a commit.
func (a *App) GetGitCommitFileDiff(repoPath string, hash string, path string) (string, error) {
	if repoPath == "" {
		if ws := a.workspaceMgr.Current(); ws != nil && len(ws.GetFolders()) > 0 {
			repoPath = ws.GetFolders()[0]
		} else {
			cwd, _ := os.Getwd()
			repoPath = cwd
		}
	}
	return a.gitEngine.GetCommitFileDiff(a.ctx, repoPath, hash, path)
}

// GetGitFileContentAtCommit returns the raw file content at a given commit.
func (a *App) GetGitFileContentAtCommit(repoPath string, hash string, path string) (string, error) {
	if repoPath == "" {
		if ws := a.workspaceMgr.Current(); ws != nil && len(ws.GetFolders()) > 0 {
			repoPath = ws.GetFolders()[0]
		} else {
			cwd, _ := os.Getwd()
			repoPath = cwd
		}
	}
	return a.gitEngine.GetFileContentAtCommit(a.ctx, repoPath, hash, path)
}

// GetGitStatus returns lightweight git status (staged, unstaged, untracked).
func (a *App) GetGitStatus(repoPath string) (*git.GitStatusResult, error) {
	if repoPath == "" {
		if ws := a.workspaceMgr.Current(); ws != nil && len(ws.GetFolders()) > 0 {
			repoPath = ws.GetFolders()[0]
		} else {
			cwd, _ := os.Getwd()
			repoPath = cwd
		}
	}
	return a.gitEngine.GetStatus(a.ctx, repoPath)
}

// GitStage stages files.
func (a *App) GitStage(repoPath string, paths []string) error {
	if repoPath == "" {
		if ws := a.workspaceMgr.Current(); ws != nil && len(ws.GetFolders()) > 0 {
			repoPath = ws.GetFolders()[0]
		} else {
			cwd, _ := os.Getwd()
			repoPath = cwd
		}
	}
	return a.gitEngine.Stage(a.ctx, repoPath, paths)
}

// GitUnstage unstages files.
func (a *App) GitUnstage(repoPath string, paths []string) error {
	if repoPath == "" {
		if ws := a.workspaceMgr.Current(); ws != nil && len(ws.GetFolders()) > 0 {
			repoPath = ws.GetFolders()[0]
		} else {
			cwd, _ := os.Getwd()
			repoPath = cwd
		}
	}
	return a.gitEngine.Unstage(a.ctx, repoPath, paths)
}

// GitDiscard discards file changes.
func (a *App) GitDiscard(repoPath string, paths []string) error {
	if repoPath == "" {
		if ws := a.workspaceMgr.Current(); ws != nil && len(ws.GetFolders()) > 0 {
			repoPath = ws.GetFolders()[0]
		} else {
			cwd, _ := os.Getwd()
			repoPath = cwd
		}
	}
	return a.gitEngine.Discard(a.ctx, repoPath, paths)
}

// GitCommit commits staged changes.
func (a *App) GitCommit(repoPath string, message string) error {
	if repoPath == "" {
		if ws := a.workspaceMgr.Current(); ws != nil && len(ws.GetFolders()) > 0 {
			repoPath = ws.GetFolders()[0]
		} else {
			cwd, _ := os.Getwd()
			repoPath = cwd
		}
	}
	return a.gitEngine.Commit(a.ctx, repoPath, message)
}

// GitPush pushes commits to remote.
func (a *App) GitPush(repoPath string) error {
	if repoPath == "" {
		if ws := a.workspaceMgr.Current(); ws != nil && len(ws.GetFolders()) > 0 {
			repoPath = ws.GetFolders()[0]
		} else {
			cwd, _ := os.Getwd()
			repoPath = cwd
		}
	}
	return a.gitEngine.Push(a.ctx, repoPath)
}

// GenerateAICommitMessage generates a commit message using AI from staged diff with targeted provider/model.
func (a *App) GenerateAICommitMessage(repoPath string, providerID string, model string, instruction string) (string, error) {
	if repoPath == "" {
		if ws := a.workspaceMgr.Current(); ws != nil && len(ws.GetFolders()) > 0 {
			repoPath = ws.GetFolders()[0]
		} else {
			cwd, _ := os.Getwd()
			repoPath = cwd
		}
	}
	diffStat, err := a.gitEngine.GetStagedDiffStat(a.ctx, repoPath)
	if err != nil {
		diffStat = ""
	}

	diff, err := a.gitEngine.GetStagedDiff(a.ctx, repoPath)
	if err != nil || strings.TrimSpace(diff) == "" {
		return "", fmt.Errorf("no staged changes found to summarize. Please stage changes first (+)")
	}

	var promptContent string
	if len(diff) > 4000 {
		// Token efficient mode for large diffs / many files: send diffstat + first 2000 chars of diff
		truncatedDiff := diff
		if len(truncatedDiff) > 2000 {
			truncatedDiff = truncatedDiff[:2000] + "\n...[staged diff truncated for token efficiency]"
		}
		promptContent = fmt.Sprintf("Staged changes are large/have many files. Here is the summary of changed files:\n%s\n\nPartial staged diff sample:\n%s", diffStat, truncatedDiff)
	} else {
		// Small changes: send full details
		promptContent = fmt.Sprintf("Staged changes summary:\n%s\n\nStaged Diff:\n%s", diffStat, diff)
	}

	messages := []llm.LLMMessage{
		{
			Role:    llm.RoleSystem,
			Content: "CRITICAL: You are a Git commit message generator. Your output MUST be ONLY a concise 1 to 2 line Git commit message following conventional commits format (e.g., 'docs(readme): rewrite architecture guide and update tech stack'). DO NOT include any analysis, section headings, Markdown tables, or explanations. ONLY output the raw commit message text.",
		},
		{
			Role:    llm.RoleUser,
			Content: promptContent,
		},
	}

	// Optional user instruction appended as a follow-up so it overrides the default style.
	if strings.TrimSpace(instruction) != "" {
		messages = append(messages, llm.LLMMessage{
			Role:    llm.RoleUser,
			Content: "Additional instruction for the commit message: " + strings.TrimSpace(instruction),
		})
	}

	var resp *llm.LLMResponse
	if providerID != "" {
		resp, err = a.llmClient.ChatWithProvider(a.ctx, providerID, model, messages, nil)
	} else {
		resp, err = a.llmClient.Chat(a.ctx, messages, nil)
	}
	if err != nil {
		return "", fmt.Errorf("AI commit generation failed: %w", err)
	}

	result := strings.TrimSpace(resp.Content)

	// Clean up any extra Markdown codeblock fences if present
	result = strings.TrimPrefix(result, "```markdown")
	result = strings.TrimPrefix(result, "```git")
	result = strings.TrimPrefix(result, "```")
	result = strings.TrimSuffix(result, "```")
	result = strings.TrimSpace(result)

	// Filter out any leftover Markdown headers or analysis tables
	lines := strings.Split(result, "\n")
	var cleanLines []string
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "###") || strings.HasPrefix(trimmed, "|") || strings.HasPrefix(trimmed, "---") || strings.HasPrefix(trimmed, "**") {
			continue
		}
		if trimmed != "" {
			cleanLines = append(cleanLines, trimmed)
		}
		if len(cleanLines) >= 2 {
			break
		}
	}

	if len(cleanLines) > 0 {
		result = strings.Join(cleanLines, "\n")
	}

	return result, nil
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

// ResolvePath expands a leading ~ and resolves relative paths against the
// current working directory, returning a cleaned absolute path.
func ResolvePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("empty path")
	}
	if strings.HasPrefix(path, "~/") || path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if path == "~" {
			return home, nil
		}
		path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
	}
	return filepath.Abs(path)
}

// OpenNewWindow launches a new ForgeADE window as a separate OS process.
// Optional workspacePath opens the given folder or .workspace file in it.
func (a *App) OpenNewWindow(workspacePath string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("new window: resolve executable: %w", err)
	}

	args := []string{}
	if workspacePath != "" {
		resolved, err := ResolvePath(workspacePath)
		if err != nil {
			return fmt.Errorf("new window: resolve path: %w", err)
		}
		args = append(args, resolved)
	}

	cmd := exec.Command(exe, args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("new window: launch: %w", err)
	}
	// Detach from the parent so the new process outlives this one.
	_ = cmd.Process.Release()
	return nil
}
