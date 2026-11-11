package ui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// IgnoredDirectoryNames holds names of directories to skip during workspace scans.
var IgnoredDirectoryNames = map[string]bool{
	".git":         true,
	".forge":       true,
	".task":        true,
	"node_modules": true,
	"dist":         true,
	"build":        true,
	".next":        true,
	".cache":       true,
	"target":       true,
	"vendor":       true,
}

// WorkspaceFile represents an indexed file in the workspace.
type WorkspaceFile struct {
	RelativePath string
	FullPath     string
	IsDir        bool
	Size         int64
}

// WorkspaceState manages workspace files and repository metadata.
type WorkspaceState struct {
	mu           sync.RWMutex
	RootDir      string
	Files        []WorkspaceFile
	SelectedPath string
	GitBranch    string
	GitDirty     bool
	FilterQuery  string
}

// NewWorkspaceState creates and initializes a new workspace state for rootDir.
func NewWorkspaceState(rootDir string) *WorkspaceState {
	if rootDir == "" {
		rootDir, _ = os.Getwd()
	}
	ws := &WorkspaceState{
		RootDir: rootDir,
	}
	ws.Refresh()
	return ws
}

// Refresh re-scans the workspace directory for editable files.
func (ws *WorkspaceState) Refresh() {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	var files []WorkspaceFile
	maxFiles := 300
	count := 0

	_ = filepath.Walk(ws.RootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || count >= maxFiles {
			return nil
		}
		name := info.Name()
		if info.IsDir() {
			if IgnoredDirectoryNames[name] || (strings.HasPrefix(name, ".") && name != ".") {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip common binary and hidden files
		if strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".png") ||
			strings.HasSuffix(name, ".jpg") || strings.HasSuffix(name, ".exe") ||
			strings.HasSuffix(name, ".so") || strings.HasSuffix(name, ".dylib") ||
			strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".zip") {
			return nil
		}

		rel, err := filepath.Rel(ws.RootDir, path)
		if err != nil {
			rel = name
		}

		files = append(files, WorkspaceFile{
			RelativePath: rel,
			FullPath:     path,
			IsDir:        false,
			Size:         info.Size(),
		})
		count++
		return nil
	})

	sort.Slice(files, func(i, j int) bool {
		return files[i].RelativePath < files[j].RelativePath
	})

	ws.Files = files
	if ws.SelectedPath == "" && len(files) > 0 {
		ws.SelectedPath = files[0].RelativePath
	}
}

// FilteredFiles returns files matching the current filter query.
func (ws *WorkspaceState) FilteredFiles() []WorkspaceFile {
	ws.mu.RLock()
	defer ws.mu.RUnlock()

	if strings.TrimSpace(ws.FilterQuery) == "" {
		return ws.Files
	}

	q := strings.ToLower(strings.TrimSpace(ws.FilterQuery))
	var out []WorkspaceFile
	for _, f := range ws.Files {
		if strings.Contains(strings.ToLower(f.RelativePath), q) {
			out = append(out, f)
		}
	}
	return out
}

// ResolveFullPath converts a relative workspace path to an absolute path.
func (ws *WorkspaceState) ResolveFullPath(rel string) string {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	return filepath.Join(ws.RootDir, rel)
}

// CreateFile creates a new empty file at relPath and refreshes the workspace index.
func (ws *WorkspaceState) CreateFile(relPath string) error {
	fullPath := ws.ResolveFullPath(relPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(fullPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	_ = f.Close()
	ws.Refresh()
	return nil
}

// CreateFolder creates a new directory at relPath and refreshes the workspace index.
func (ws *WorkspaceState) CreateFolder(relPath string) error {
	fullPath := ws.ResolveFullPath(relPath)
	if err := os.MkdirAll(fullPath, 0755); err != nil {
		return err
	}
	ws.Refresh()
	return nil
}

// DeleteFile deletes a file at relPath and refreshes the workspace index.
func (ws *WorkspaceState) DeleteFile(relPath string) error {
	fullPath := ws.ResolveFullPath(relPath)
	if err := os.Remove(fullPath); err != nil {
		return err
	}
	ws.Refresh()
	return nil
}
