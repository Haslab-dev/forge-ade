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
	goruntime "runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"github.com/hasdev/forge-ade/internal/agent"
	"github.com/hasdev/forge-ade/internal/events"
	"github.com/hasdev/forge-ade/internal/discovery"
	"github.com/hasdev/forge-ade/internal/explorer"
	"github.com/hasdev/forge-ade/internal/git"	
    "github.com/hasdev/forge-ade/internal/index"
	"github.com/hasdev/forge-ade/internal/llm"
	"github.com/hasdev/forge-ade/internal/mcp"
	"github.com/hasdev/forge-ade/internal/search"
	"github.com/hasdev/forge-ade/internal/skills"
	"github.com/hasdev/forge-ade/internal/terminal"
	"github.com/hasdev/forge-ade/internal/tools"
	"github.com/hasdev/forge-ade/internal/watcher"
	"github.com/hasdev/forge-ade/internal/workspace"
	"github.com/wailsapp/wails/v3/pkg/application"
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
	indexStore   *index.Store
	indexUnsub   func()
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
	agentMgr := agent.NewManager(llmClient, toolReg, skillMgr, mcpMgr, sm, bus, dataDir)
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
	a.sweepGitDirty()
	tree, err := a.explorer.GetTree(depth)
	if err != nil {
		return "", err
	}
	a.annotateGitStatus(tree)
	data, _ := json.Marshal(tree)
	return string(data), nil
}

// ListDirectory lists a directory's contents.
func (a *App) ListDirectory(dirPath string) (string, error) {
	a.sweepGitDirty()
	entries, err := a.explorer.ListDirectory(dirPath)
	if err != nil {
		return "", err
	}
	a.annotateGitStatus(entries)
	data, _ := json.Marshal(entries)
	return string(data), nil
}

// ExpandPath expands a directory in the tree and returns siblings.
func (a *App) ExpandPath(targetPath string) (string, error) {
	a.sweepGitDirty()
	entries, err := a.explorer.ExpandPath(targetPath)
	if err != nil {
		return "", err
	}
	a.annotateGitStatus(entries)
	data, _ := json.Marshal(entries)
	return string(data), nil
}

// ToggleHiddenFiles toggles hidden file visibility in the explorer.
func (a *App) ToggleHiddenFiles() bool {
	current := a.explorer.GetShowHidden()
	a.explorer.SetShowHidden(!current)
	return !current
}

// annotateGitStatus decorates file tree nodes with git status characters
// ("U" untracked/added, "M" modified, "D" deleted) resolved per repo root.
// Directories with any changed descendant are marked so the UI can show a dot.
// nodes may be workspace roots (all dirs) or a flat directory listing returned
// by ExpandPath/ListDirectory, so both file and dir entries are annotated.
func (a *App) annotateGitStatus(nodes []*explorer.FileInfo) {
	cache := make(map[string]map[string]string)
	for _, n := range nodes {
		// For a file entry, walk up from its parent dir to find the repo root.
		start := n.Path
		if !n.IsDir {
			start = filepath.Dir(n.Path)
		}
		repoRoot, ok := git.FindRepoRoot(start)
		if !ok {
			continue
		}
		statusMap, ok := cache[repoRoot]
		if !ok {
			statusMap, _ = a.gitEngine.StatusByPath(a.ctx, repoRoot)
			cache[repoRoot] = statusMap
		}
		if statusMap != nil {
			annotateNodeGitStatus(n, repoRoot, statusMap)
		}
	}
}

// annotateNodeGitStatus recursively marks each node with its git status and
// returns true when the node (or any descendant) has changes.
func annotateNodeGitStatus(node *explorer.FileInfo, repoRoot string, statusMap map[string]string) bool {
	relSlash := ""
	if rel, err := filepath.Rel(repoRoot, node.Path); err == nil {
		relSlash = filepath.ToSlash(rel)
	}
	sc := statusMap[relSlash]
	// A directory may hold changes whose paths no longer appear as children in
	// the tree (e.g. deleted files) — mark it dirty when any status entry nests
	// underneath it.
	if sc == "" && node.IsDir && relSlash != "" {
		prefix := relSlash + "/"
		for k := range statusMap {
			if strings.HasPrefix(k, prefix) {
				sc = "M"
				break
			}
		}
	}
	childDirty := false
	if node.Children != nil {
		for _, child := range node.Children {
			if annotateNodeGitStatus(child, repoRoot, statusMap) {
				childDirty = true
			}
		}
	}
	if sc != "" {
		node.GitStatus = sc
	} else if childDirty {
		node.GitStatus = "M"
	}
	return node.GitStatus != ""
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

// BrowserOpenURL opens a URL in the system default browser.
func (a *App) BrowserOpenURL(url string) error {
	if strings.TrimSpace(url) == "" {
		return fmt.Errorf("url cannot be empty")
	}
	cmd := exec.Command("open", url)
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

// resolveWorkspacePath resolves relative paths against the current workspace
// or explorer roots so file operations work seamlessly from any location.
func (a *App) resolveWorkspacePath(path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	if ws := a.workspaceMgr.Current(); ws != nil && len(ws.GetFolders()) > 0 {
		return filepath.Join(ws.GetFolders()[0], path)
	}
	if len(a.explorer.GetRoots()) > 0 {
		return filepath.Join(a.explorer.GetRoots()[0], path)
	}
	cwd, err := os.Getwd()
	if err == nil {
		return filepath.Join(cwd, path)
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
	path = a.resolveWorkspacePath(path)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file base64: %w", err)
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

// WriteFile writes content to a file, creating it if needed.
func (a *App) WriteFile(path string, content string) error {
	path = a.resolveWorkspacePath(path)
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

// CreateFolder creates a new directory (and parents if needed).
func (a *App) CreateFolder(path string) error {
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("create folder: %w", err)
	}
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

// GetClipboardFiles returns the absolute path of a single file/folder in the
// system clipboard (e.g. copied from Finder). Empty slice when the clipboard
// holds text, nothing, or multiple files (macOS only).
func (a *App) GetClipboardFiles() []string {
	if goruntime.GOOS != "darwin" {
		return nil
	}
	script := "try\nset u to the clipboard as «class furl»\nreturn POSIX path of u\non error\nreturn \"\"\nend try"
	cmd := exec.Command("osascript", "-e", script)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	p := strings.TrimSpace(string(out))
	// Guard: furl coercion is loose — text like "a: b" can turn into a
	// bogus HFS-style path. Only accept real, existing absolute paths.
	if p == "" || !filepath.IsAbs(p) {
		return nil
	}
	if _, err := os.Stat(p); err != nil {
		return nil
	}
	return []string{p}
}

// CopyPath copies a file or folder (recursively) from src to dst. dst is the
// full destination path (file for file, dir for folder).
func (a *App) CopyPath(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("copy stat: %w", err)
	}
	if !info.IsDir() {
		return a.CopyFile(src, dst)
	}
	if err := os.MkdirAll(dst, 0755); err != nil {
		return fmt.Errorf("copy mkdir: %w", err)
	}
	return filepath.Walk(src, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if fi.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0644)
	})
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

// SearchReplaceAll replaces all occurrences of the query across matching files.
func (a *App) SearchReplaceAll(opts search.ReplaceOptions) (search.ReplaceResult, error) {
	return a.searchMgr.ReplaceAll(opts)
}

// ---------------------------------------------------------------------------
// Workspace Index (FWI) — RFC-0001
// ---------------------------------------------------------------------------

// IndexStatus reports whether the workspace index is built and its size.
func (a *App) IndexStatus() map[string]interface{} {
	if a.indexStore == nil {
		return map[string]interface{}{"built": false}
	}
	syms := a.indexStore.Symbols()
	filesByLang, symsByLang := a.indexStore.LanguageStats()
	return map[string]interface{}{
		"built":            true,
		"symbols":          len(syms),
		"languages":        filesByLang,
		"symbols_language": symsByLang,
	}
}

// GetSymbols returns all indexed symbols.
func (a *App) GetSymbols() []index.Symbol {
	if a.indexStore == nil {
		return nil
	}
	return a.indexStore.Symbols()
}

// FindSymbol returns declarations matching name exactly (go-to-definition).
func (a *App) FindSymbol(name string) []index.Symbol {
	if a.indexStore == nil {
		return nil
	}
	return a.indexStore.Definition(name)
}

// SearchIndexSymbols finds symbols by exact/prefix/camel/fuzzy query (RFC §15).
func (a *App) SearchIndexSymbols(query string) []index.Symbol {
	if a.indexStore == nil {
		return nil
	}
	return a.indexStore.Search(query)
}

// GetCompletion returns completion candidates for a prefix (RFC §11),
// scoped to the language of `path` so suggestions never cross languages.
func (a *App) GetCompletion(prefix, path string) []index.Symbol {
	if a.indexStore == nil {
		return nil
	}
	return a.indexStore.Completion(prefix, index.DetectLanguage(path), path)
}

// GetMembers returns member suggestions for `instance.` (RFC §7): class
// members, object-literal keys, or function return shapes — scoped to the
// language of `path`.
func (a *App) GetMembers(instance, path string) []index.Symbol {
	if a.indexStore == nil {
		return nil
	}
	return a.indexStore.Members(instance, string(index.DetectLanguage(path)))
}

// GetOutline returns the symbols declared in a file, sorted by line (RFC §14).
func (a *App) GetOutline(file string) []index.Symbol {
	if a.indexStore == nil {
		return nil
	}
	return a.indexStore.Outline(a.resolveWorkspacePath(file))
}

// GetImports returns the import statements of a file.
func (a *App) GetImports(file string) []index.Import {
	if a.indexStore == nil {
		return nil
	}
	return a.indexStore.Imports(a.resolveWorkspacePath(file))
}

// GetExports returns the export statements of a file.
func (a *App) GetExports(file string) []index.Export {
	if a.indexStore == nil {
		return nil
	}
	return a.indexStore.Exports(a.resolveWorkspacePath(file))
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

// emitEvent forwards an event to the frontend via the Wails v3 event system.
func (a *App) emitEvent(name string, data interface{}) {
	if app := application.Get(); app != nil {
		app.Event.Emit(name, data)
	}
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
	folders := ws.GetFolders()
	a.explorer.SetRoots(folders)
	a.searchMgr.SetDirectories(folders)

	// Rebuild the workspace symbol index (FWI). Indexes the first folder;
	// multi-root workspaces index each folder with its own store.
	if a.indexUnsub != nil {
		a.indexUnsub()
		a.indexUnsub = nil
	}
	if len(folders) > 0 {
		a.indexStore = index.New(folders[0])
		_ = a.indexStore.Load()
		a.indexUnsub = a.indexStore.Listen(a.bus)
		go func() {
			_ = a.indexStore.Build()
			_ = a.indexStore.Save()
		}()
	}

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

// gitDirtyFlag is set by file-system events and swept lazily: the next git
// status consumer clears the whole status cache once (instead of per-event),
// so bursts of file changes (builds, installs) never re-run `git status`
// per event — only at most once per repo per TTL window, and only when
// something actually reads status.
var gitDirtyFlag int64

// sweepGitDirty invalidates the git status cache if any file event happened
// since the last sweep. Call at the top of every binding that consumes git
// status so reads always see fresh data without any background polling.
func (a *App) sweepGitDirty() {
	if atomic.SwapInt64(&gitDirtyFlag, 0) == 1 {
		a.gitEngine.InvalidateAll()
	}
}

func (a *App) setupEventHandlers() {
	a.bus.Subscribe(events.FileCreated, func(e events.Event) {
		path, _ := e.Data["path"].(string)
		if path != "" {
			a.searchMgr.IndexFile(path)
			atomic.StoreInt64(&gitDirtyFlag, 1)
			if a.ctx != nil {
				a.emitEvent("fs:changed", map[string]interface{}{
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
			atomic.StoreInt64(&gitDirtyFlag, 1)
			if a.ctx != nil {
				a.emitEvent("fs:changed", map[string]interface{}{
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
			atomic.StoreInt64(&gitDirtyFlag, 1)
			if a.ctx != nil {
				a.emitEvent("fs:changed", map[string]interface{}{
					"type": "deleted",
					"path": path,
				})
			}
		}
	})
	a.bus.Subscribe(events.FileRenamed, func(e events.Event) {
		path, _ := e.Data["path"].(string)
		oldPath, _ := e.Data["oldPath"].(string)
		if path == "" {
			return
		}
		atomic.StoreInt64(&gitDirtyFlag, 1)
		if oldPath != "" {
			// Paired rename (old + new known): tell the frontend to follow the
			// tab, and keep the search index in sync.
			a.searchMgr.RemoveFile(oldPath)
			a.searchMgr.IndexFile(path)
			if a.ctx != nil {
				a.emitEvent("fs:changed", map[string]interface{}{
					"type":    "renamed",
					"path":    path,
					"oldPath": oldPath,
				})
			}
			return
		}
		// Unpaired rename → classify by whether the path still exists.
		a.searchMgr.IndexFile(path)
		typ := "created"
		if _, err := os.Stat(path); err != nil {
			typ = "deleted"
			a.searchMgr.RemoveFile(path)
		}
		if a.ctx != nil {
			a.emitEvent("fs:changed", map[string]interface{}{
				"type": typ,
				"path": path,
			})
		}
	})

	// Bridge agent updates to frontend. All granular agent events
	// (turn/message/thinking/tool) are forwarded verbatim so the chat can
	// stream deltas instead of polling the whole session list.
	agentEvents := []events.EventType{
		"agent:updated",
		"agent:started",
		"agent:stopped",
		"agent:turn_start",
		"agent:turn_end",
		"agent:message_start",
		"agent:message_delta",
		"agent:message_end",
		"agent:thinking_start",
		"agent:thinking_delta",
		"agent:thinking_end",
		"agent:tool_start",
		"agent:tool_delta",
		"agent:tool_end",
		"agent:ask",
	}
	for _, evType := range agentEvents {
		evType := evType
		a.bus.Subscribe(evType, func(e events.Event) {
			if a.ctx != nil {
				a.emitEvent(string(evType), e.Data)
			}
		})
	}
	a.bus.Subscribe("agent:config:changed", func(e events.Event) {
		if a.ctx != nil {
			a.emitEvent("agent:config:changed", e.Data)
		}
	})

	// Bridge terminal output to frontend via Wails runtime events
	a.bus.Subscribe(events.TerminalOutput, func(e events.Event) {
		if a.ctx != nil {
			a.emitEvent("session:output", e.Data)
		}
	})
	a.bus.Subscribe(events.TerminalOpened, func(e events.Event) {
		if a.ctx != nil {
			a.emitEvent("session:opened", e.Data)
		}
	})
	a.bus.Subscribe(events.TerminalClosed, func(e events.Event) {
		if a.ctx != nil {
			a.emitEvent("session:closed", e.Data)
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

// ListAgentSessionsForFolder returns the agent sessions linked to the given
// project folder (and its subfolders) — the session history for the current
// project only, hiding sessions from other projects.
func (a *App) ListAgentSessionsForFolder(folder string) []*agent.Session {
	return a.agentMgr.ListSessionsForFolder(folder)
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

// StopAgentTurn cancels the currently running agent turn for a session.
func (a *App) StopAgentTurn(sessionID string) {
	a.agentMgr.StopTurn(sessionID)
}

// SetAgentDialect switches a session's tool-calling dialect ("" = native, "xml" = in-band).
func (a *App) SetAgentDialect(sessionID string, dialect string) error {
	return a.agentMgr.SetDialect(sessionID, dialect)
}

// RespondAgentAsk answers pending `ask` questions and resumes the agent turn.
func (a *App) RespondAgentAsk(sessionID string, answers map[string]any) error {
	return a.agentMgr.RespondAsk(sessionID, answers)
}

// SetAgentAutoApprove toggles yolo mode (always approve tool calls) for a session.
func (a *App) SetAgentAutoApprove(sessionID string, enabled bool) error {
	return a.agentMgr.SetAutoApprove(sessionID, enabled)
}

// ApplyAgentDefinitionToSession re-configures an existing session to use a
// pre-configured agent definition (role, prompt, rules, model) without
// creating a new session.
func (a *App) ApplyAgentDefinitionToSession(sessionID string, defID string) error {
	return a.agentMgr.ApplyDefinitionToSession(sessionID, defID)
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
	a.emitEvent("agent:config:changed", map[string]interface{}{})
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

// ---------------------------------------------------------------------------
// Multi-source discovery (MCP servers + skills from other agent tools)
// ---------------------------------------------------------------------------

// DiscoverMCPServers aggregates MCP servers from other agent tools' configs
// (Claude, Codex, Cursor, Windsurf, Gemini, opencode, Antigravity) plus our
// own entries (flagged Imported).
func (a *App) DiscoverMCPServers() []discovery.DiscoveredMCPServer {
	return discovery.DiscoverMCPServers(a.mcpMgr.ListServers())
}

// ImportDiscoveredMCPServers imports the named discovered servers into the
// app's own MCP config (enabled) and reconnects.
func (a *App) ImportDiscoveredMCPServers(names []string) error {
	if len(names) == 0 {
		return nil
	}
	found := map[string]discovery.DiscoveredMCPServer{}
	for _, s := range discovery.DiscoverMCPServers(a.mcpMgr.ListServers()) {
		found[strings.ToLower(s.Name)] = s
	}
	imported := 0
	for _, name := range names {
		s, ok := found[strings.ToLower(name)]
		if !ok {
			continue
		}
		if _, err := a.mcpMgr.SaveServer(s.ToServerConfig()); err != nil {
			return fmt.Errorf("import mcp %q: %w", s.Name, err)
		}
		imported++
	}
	if imported > 0 {
		go func() {
			if err := a.mcpMgr.ConnectAll(a.ctx); err != nil {
				log.Printf("mcp: reconnect after import: %v", err)
			}
			a.refreshMCPTools()
			a.emitEvent("agent:config:changed", map[string]interface{}{})
		}()
	}
	log.Printf("[discovery] imported %d MCP servers", imported)
	return nil
}

// DiscoverSkills aggregates skills from other agents' skill directories
// (Claude, AGENTS.md conventions, Antigravity) plus our own (flagged
// Imported).
func (a *App) DiscoverSkills() []discovery.DiscoveredSkill {
	return discovery.DiscoverSkills(a.skillMgr.List())
}

// ImportDiscoveredSkills copies the named discovered skills into the app's
// global skills directory and reloads the skill manager.
func (a *App) ImportDiscoveredSkills(names []string) error {
	if len(names) == 0 {
		return nil
	}
	found := map[string]discovery.DiscoveredSkill{}
	for _, s := range discovery.DiscoverSkills(a.skillMgr.List()) {
		found[strings.ToLower(s.Name)] = s
	}
	imported := 0
	for _, name := range names {
		s, ok := found[strings.ToLower(name)]
		if !ok || s.Path == "" {
			continue
		}
		if _, err := discovery.CopySkill(s.Path); err != nil {
			return fmt.Errorf("import skill %q: %w", s.Name, err)
		}
		imported++
	}
	if imported > 0 {
		a.skillMgr.Reload()
		a.emitEvent("agent:config:changed", map[string]interface{}{})
	}
	log.Printf("[discovery] imported %d skills", imported)
	return nil
}

// ListMCPTools returns loaded MCP tools.
func (a *App) ListMCPTools() []mcp.Tool {
	return a.mcpMgr.ListTools()
}

// ListConnectedMCPTools returns the tools discovered from live MCP connections
// (the actual tools the agent can call).
func (a *App) ListConnectedMCPTools() []mcp.Tool {
	return a.mcpMgr.ListConnectedTools()
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

// ReconnectMCP reconnects to all enabled MCP servers and refreshes the tool
// registry. Called after the user edits MCP server config.
func (a *App) ReconnectMCP() error {
	a.mcpMgr.DisconnectAll()
	if err := a.mcpMgr.ConnectAll(a.ctx); err != nil {
		return err
	}
	a.refreshMCPTools()
	return nil
}

// GetGitCommitGraph streams lightweight paginated Git commits with graph prefix.
// branch "" means all branches.
func (a *App) GetGitCommitGraph(repoPath string, offset int, limit int, branch string) (*git.CommitGraphResult, error) {
	if repoPath == "" {
		if ws := a.workspaceMgr.Current(); ws != nil && len(ws.GetFolders()) > 0 {
			repoPath = ws.GetFolders()[0]
		} else {
			cwd, _ := os.Getwd()
			repoPath = cwd
		}
	}
	return a.gitEngine.GetCommitGraph(a.ctx, repoPath, offset, limit, branch)
}

// GetGitBranches lists local branch names for the repo.
func (a *App) GetGitBranches(repoPath string) ([]string, error) {
	if repoPath == "" {
		if ws := a.workspaceMgr.Current(); ws != nil && len(ws.GetFolders()) > 0 {
			repoPath = ws.GetFolders()[0]
		} else {
			cwd, _ := os.Getwd()
			repoPath = cwd
		}
	}
	return a.gitEngine.GetBranches(a.ctx, repoPath)
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

// GetGitCommitBody returns the full commit message (subject + body) for a commit.
func (a *App) GetGitCommitBody(repoPath string, hash string) (string, error) {
	if repoPath == "" {
		if ws := a.workspaceMgr.Current(); ws != nil && len(ws.GetFolders()) > 0 {
			repoPath = ws.GetFolders()[0]
		} else {
			cwd, _ := os.Getwd()
			repoPath = cwd
		}
	}
	return a.gitEngine.GetCommitBody(a.ctx, repoPath, hash)
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

// GetGitFileDiffHunks returns the structured hunks of a file's diff against HEAD.
func (a *App) GetGitFileDiffHunks(repoPath string, path string) ([]git.DiffHunk, error) {
	if repoPath == "" {
		if ws := a.workspaceMgr.Current(); ws != nil && len(ws.GetFolders()) > 0 {
			repoPath = ws.GetFolders()[0]
		} else {
			cwd, _ := os.Getwd()
			repoPath = cwd
		}
	}
	return a.gitEngine.GetFileDiffHunks(a.ctx, repoPath, path)
}

// RevertGitHunk reverse-applies a single diff hunk, restoring it to HEAD state.
func (a *App) RevertGitHunk(repoPath string, path string, hunkIndex int) error {
	if repoPath == "" {
		if ws := a.workspaceMgr.Current(); ws != nil && len(ws.GetFolders()) > 0 {
			repoPath = ws.GetFolders()[0]
		} else {
			cwd, _ := os.Getwd()
			repoPath = cwd
		}
	}
	return a.gitEngine.RevertDiffHunk(a.ctx, repoPath, path, hunkIndex)
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
	a.sweepGitDirty()
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

// GetGitConflictStageContent returns a conflicted file's content at a merge
// stage: 1 = common ancestor, 2 = ours, 3 = theirs.
func (a *App) GetGitConflictStageContent(repoPath string, path string, stage int) (string, error) {
	if repoPath == "" {
		if ws := a.workspaceMgr.Current(); ws != nil && len(ws.GetFolders()) > 0 {
			repoPath = ws.GetFolders()[0]
		} else {
			cwd, _ := os.Getwd()
			repoPath = cwd
		}
	}
	return a.gitEngine.GetConflictStageContent(a.ctx, repoPath, path, stage)
}

// GitResolveConflict resolves a conflicted file. action: "ours", "theirs", or
// "mark" (stage the current working-tree content).
func (a *App) GitResolveConflict(repoPath string, path string, action string) error {
	if repoPath == "" {
		if ws := a.workspaceMgr.Current(); ws != nil && len(ws.GetFolders()) > 0 {
			repoPath = ws.GetFolders()[0]
		} else {
			cwd, _ := os.Getwd()
			repoPath = cwd
		}
	}
	return a.gitEngine.ResolveConflict(a.ctx, repoPath, path, action)
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

// GitFetch updates remote-tracking branches from the default remote.
func (a *App) GitFetch(repoPath string) (string, error) {
	if repoPath == "" {
		if ws := a.workspaceMgr.Current(); ws != nil && len(ws.GetFolders()) > 0 {
			repoPath = ws.GetFolders()[0]
		} else {
			cwd, _ := os.Getwd()
			repoPath = cwd
		}
	}
	return a.gitEngine.Fetch(a.ctx, repoPath)
}

// GitMerge merges the given source commit/branch into the current branch.
func (a *App) GitMerge(repoPath string, source string, noFF bool, squash bool) (string, error) {
	if repoPath == "" {
		if ws := a.workspaceMgr.Current(); ws != nil && len(ws.GetFolders()) > 0 {
			repoPath = ws.GetFolders()[0]
		} else {
			cwd, _ := os.Getwd()
			repoPath = cwd
		}
	}
	return a.gitEngine.Merge(a.ctx, repoPath, source, noFF, squash)
}

// GenerateAICommitMessage generates a commit message using AI from staged diff with targeted provider/model.
func (a *App) GenerateAICommitMessage(repoPath string, providerID string, model string, instruction string) (string, error) {
	log.Printf("[ai-commit] start: repo=%q provider=%q model=%q instruction=%q", repoPath, providerID, model, instruction)
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
	log.Printf("[ai-commit] repo=%q diffstat_len=%d err=%v", repoPath, len(diffStat), err)

	diff, err := a.gitEngine.GetStagedDiff(a.ctx, repoPath)
	if err != nil || strings.TrimSpace(diff) == "" {
		log.Printf("[ai-commit] no staged diff: err=%v diff_len=%d", err, len(diff))
		return "", fmt.Errorf("no staged changes found to summarize. Please stage changes first (+)")
	}
	log.Printf("[ai-commit] staged diff_len=%d (truncated mode=%v)", len(diff), len(diff) > 4000)

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
		log.Printf("[ai-commit] calling ChatWithProviderStream provider=%q model=%q", providerID, model)
		// Use the streaming path even for AI commit: some providers/proxies
		// (e.g. kilo-auto/free) only answer streaming requests and return an
		// empty 200 body to non-streaming calls.
		resp, err = a.llmClient.ChatWithProviderStream(a.ctx, providerID, model, messages, nil, nil, nil)
	} else {
		cfg := a.llmClient.GetConfig()
		log.Printf("[ai-commit] calling ChatWithStream (active provider=%q model=%q)", cfg.ProviderID, cfg.Model)
		resp, err = a.llmClient.ChatWithStream(a.ctx, messages, nil, nil)
	}
	if err != nil {
		log.Printf("[ai-commit] LLM call error: %v", err)
		return "", fmt.Errorf("AI commit generation failed: %w", err)
	}
	log.Printf("[ai-commit] LLM response: content_len=%d reasoning_len=%d tokens=%+v", len(resp.Content), len(resp.Reasoning), resp.TokenUsage)

	result := strings.TrimSpace(resp.Content)
	if result == "" {
		log.Printf("[ai-commit] WARNING: empty content after trim (reasoning only?)")
		return "", fmt.Errorf("AI commit generation returned an empty response. The provider may have failed silently — check the log")
	}

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

// ServiceStartup is called by Wails v3 when the service starts up.
func (a *App) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	a.ctx = ctx
	log.Println("ForgeADE started")

	// Connect to enabled MCP servers and register their tools into the tool
	// registry so the agent can call them (connect-on-startup).
	go func() {
		if err := a.mcpMgr.ConnectAll(ctx); err != nil {
			log.Printf("mcp: connect all: %v", err)
		}
		a.refreshMCPTools()
	}()
	return nil
}

// refreshMCPTools re-registers the tools discovered from live MCP connections.
func (a *App) refreshMCPTools() {
	for _, t := range a.mcpMgr.ListConnectedTools() {
		a.toolReg.RegisterMCPToolWithCaller(llm.MCPTool{
			ServerName:  t.ServerName,
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		}, a.mcpMgr)
	}
}

// ServiceShutdown is called by Wails v3 when the application shuts down.
func (a *App) ServiceShutdown() {
	a.fileWatcher.Stop()
	a.sessionMgr.StopAll()
	a.searchMgr.Stop()
	a.mcpMgr.DisconnectAll()
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
