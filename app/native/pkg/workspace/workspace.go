package workspace

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/hasdev/forge-ade/core/remotefs"
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
	// macOS system / cloud-provider trees: enormous, irrelevant as project
	// files, and (iCloud "Mobile Documents" in particular) their opens can
	// block indefinitely when files are not downloaded.
	"Library":          true,
	"Applications":     true,
	"Mobile Documents": true,
	"CloudStorage":     true,
	"Volumes":          true,
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
	RemoteFS     *remotefs.Manager
	loadedDirs   map[string]bool // folders already listed lazily
}

// NewWorkspaceState creates and initializes a new workspace state for rootDir.
// An empty rootDir means NO workspace: files stay empty until the user opens
// a folder ("" is also what CWD=/ collapses to for Finder-launched bundles).
func NewWorkspaceState(rootDir string) *WorkspaceState {
	if rootDir == "/" {
		rootDir = ""
	}
	ws := &WorkspaceState{
		RootDir:    rootDir,
		loadedDirs: map[string]bool{"": true},
	}
	ws.Refresh()
	return ws
}

// IsRemote reports whether the workspace is located on a remote SSH server.
func (ws *WorkspaceState) IsRemote() bool {
	if ws == nil {
		return false
	}
	return strings.HasPrefix(ws.RootDir, "ssh://")
}

// Refresh re-scans the workspace directory for editable files.
func (ws *WorkspaceState) Refresh() {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	var files []WorkspaceFile
	maxFiles := 1200
	ws.loadedDirs = map[string]bool{"": true}
	count := 0

	// No workspace opened yet — nothing to scan.
	if ws.RootDir == "" {
		ws.Files = nil
		return
	}

	// Handle remote SSH directory scanning
	if strings.HasPrefix(ws.RootDir, "ssh://") && ws.RemoteFS != nil {
		var walkRemote func(dirURI string, depth int)
		walkRemote = func(dirURI string, depth int) {
			if depth > 3 || count >= maxFiles {
				return
			}
			entries, isRemote, err := ws.RemoteFS.ListDirectory(dirURI)
			if !isRemote || err != nil {
				return
			}
			for _, entry := range entries {
				if entry == nil {
					continue
				}
				name := filepath.Base(entry.Path)
				if strings.HasPrefix(name, ".") && name != ".env" {
					continue
				}
				if entry.IsDir && (name == "node_modules" || name == ".git" || name == "dist" || name == "build" || name == ".next") {
					continue
				}
				rel := strings.TrimPrefix(entry.Path, ws.RootDir)
				rel = strings.TrimPrefix(rel, "/")
				files = append(files, WorkspaceFile{
					RelativePath: rel,
					FullPath:     entry.Path,
					IsDir:        entry.IsDir,
					Size:         entry.Size,
				})
				count++
				if entry.IsDir {
					walkRemote(entry.Path, depth+1)
				}
			}
		}
		walkRemote(ws.RootDir, 0)
		ws.Files = files
		return
	}

	// Breadth-first walk so the budget spreads across the whole tree: a
	// depth-first cap would let the first big subtree (e.g. app/webview/…)
	// swallow the budget and leave the rest of the repo invisible.
	type dirEntry struct {
		path  string
		depth int
	}
	queue := []dirEntry{{path: ws.RootDir, depth: 0}}
	const (
		maxDepth  = 6
		perDirCap = 120
	)
	for len(queue) > 0 && count < maxFiles {
		cur := queue[0]
		queue = queue[1:]
		if cur.depth > maxDepth {
			continue
		}
		entries, err := os.ReadDir(cur.path)
		if err != nil {
			continue
		}
		added := 0
		for _, e := range entries {
			if count >= maxFiles || added >= perDirCap {
				break
			}
			name := e.Name()
			if e.IsDir() {
				if IgnoredDirectoryNames[name] || (strings.HasPrefix(name, ".") && name != ".") {
					continue
				}
				queue = append(queue, dirEntry{path: filepath.Join(cur.path, name), depth: cur.depth + 1})
				continue
			}
			if strings.HasPrefix(name, ".") || binarySuffix(name) {
				continue
			}
			rel, err := filepath.Rel(ws.RootDir, filepath.Join(cur.path, name))
			if err != nil {
				rel = name
			}
			info, err := e.Info()
			size := int64(0)
			if err == nil {
				size = info.Size()
			}
			files = append(files, WorkspaceFile{
				RelativePath: rel,
				FullPath:     filepath.Join(cur.path, name),
				IsDir:        false,
				Size:         size,
			})
			count++
			added++
		}
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].RelativePath < files[j].RelativePath
	})

	ws.Files = files
	if ws.SelectedPath == "" && len(files) > 0 {
		ws.SelectedPath = files[0].RelativePath
	}
}

// binarySuffix reports whether a filename looks like a binary asset the
// explorer should skip.
func binarySuffix(name string) bool {
	for _, suf := range []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".ico",
		".exe", ".so", ".dylib", ".dll", ".bin", ".tar.gz", ".zip", ".icns",
		".woff", ".woff2", ".ttf", ".otf", ".mp4", ".mov", ".mp3", ".pdf"} {
		if strings.HasSuffix(name, suf) {
			return true
		}
	}
	return false
}

// loadedDirs remembers which folders have been lazily listed.
// LoadDir lists ONE directory live (webview GetDirectoryTree parity,
// ~0.2ms) and merges it into the cached file index so expanding a folder
// in the explorer always shows its real children, even when the initial
// snapshot budget never reached that depth.
func (ws *WorkspaceState) LoadDir(rel string) {
	ws.mu.Lock()
	if ws.loadedDirs == nil {
		ws.loadedDirs = map[string]bool{}
	}
	if ws.loadedDirs[rel] {
		ws.mu.Unlock()
		return
	}
	ws.loadedDirs[rel] = true

	root := ws.RootDir
	dir := rel
	if rel == "." || rel == "" {
		dir = ""
	}
	full := root
	if dir != "" {
		full = filepath.Join(root, dir)
	}
	entries, err := os.ReadDir(full)
	if err != nil {
		ws.mu.Unlock()
		return
	}
	known := map[string]bool{}
	for _, f := range ws.Files {
		known[f.RelativePath] = true
	}
	for _, e := range entries {
		name := e.Name()
		childRel := name
		if dir != "" {
			childRel = dir + "/" + name
		}
		if e.IsDir() {
			if IgnoredDirectoryNames[name] || (strings.HasPrefix(name, ".") && name != ".") {
				continue
			}
			if !known[childRel] {
				ws.Files = append(ws.Files, WorkspaceFile{RelativePath: childRel, FullPath: filepath.Join(full, name), IsDir: true})
			}
			continue
		}
		if strings.HasPrefix(name, ".") || binarySuffix(name) {
			continue
		}
		if known[childRel] {
			continue
		}
		size := int64(0)
		if info, err := e.Info(); err == nil {
			size = info.Size()
		}
		ws.Files = append(ws.Files, WorkspaceFile{RelativePath: childRel, FullPath: filepath.Join(full, name), Size: size})
	}
	ws.mu.Unlock()
}

// NeedsLoad reports whether an expanded folder's children were never listed.
func (ws *WorkspaceState) NeedsLoad(rel string) bool {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	if ws.loadedDirs == nil {
		return true
	}
	return !ws.loadedDirs[rel]
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
	if strings.HasPrefix(fullPath, "ssh://") {
		if ws.RemoteFS == nil {
			return fmt.Errorf("not connected")
		}
		_, err := ws.RemoteFS.WriteFile(fullPath, []byte(""))
		ws.Refresh()
		return err
	}
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
	if strings.HasPrefix(fullPath, "ssh://") {
		if ws.RemoteFS == nil {
			return fmt.Errorf("not connected")
		}
		_, err := ws.RemoteFS.CreateFolder(fullPath)
		ws.Refresh()
		return err
	}
	if err := os.MkdirAll(fullPath, 0755); err != nil {
		return err
	}
	ws.Refresh()
	return nil
}

// DeleteFile deletes a file at relPath and refreshes the workspace index.
func (ws *WorkspaceState) DeleteFile(relPath string) error {
	fullPath := ws.ResolveFullPath(relPath)
	if strings.HasPrefix(fullPath, "ssh://") {
		if ws.RemoteFS == nil {
			return fmt.Errorf("not connected")
		}
		_, err := ws.RemoteFS.DeleteFile(fullPath)
		ws.Refresh()
		return err
	}
	if err := os.Remove(fullPath); err != nil {
		return err
	}
	ws.Refresh()
	return nil
}

// FileSearchHit is one match of a text search across workspace files.
type FileSearchHit struct {
	Path      string // relative path
	Line      int    // 1-based line number
	Text      string // the matched line, trimmed
	ByteStart int    // match start offset in the file (for editor selection)
}

const (
	searchMaxFileBytes = 512 << 10 // skip files above 512 KB
	searchMaxHits      = 200
	searchMaxPerFile   = 20
)

// SearchFiles greps the workspace's tracked files for a case-insensitive
// substring and returns up to 200 hits, mirroring forge-ade's project
// search. Binary-looking files are skipped.
func (ws *WorkspaceState) SearchFiles(query string) []FileSearchHit {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return nil
	}
	var hits []FileSearchHit
	for _, f := range ws.Files {
		if f.IsDir || len(hits) >= searchMaxHits {
			break
		}
		data, err := os.ReadFile(f.FullPath)
		if err != nil || len(data) > searchMaxFileBytes {
			continue
		}
		if isBinary(data) {
			continue
		}
		lower := strings.ToLower(string(data))
		start := 0
		perFile := 0
		line := 1
		for perFile < searchMaxPerFile && len(hits) < searchMaxHits {
			idx := strings.Index(lower[start:], query)
			if idx == -1 {
				break
			}
			abs := start + idx
			line += strings.Count(lower[start:abs], "\n")
			lineStart := strings.LastIndexByte(lower[:abs], '\n') + 1
			lineEnd := lineStart
			if lineEnd < len(lower) {
				if nl := strings.IndexByte(lower[abs:], '\n'); nl != -1 {
					lineEnd = abs + nl
				} else {
					lineEnd = len(lower)
				}
			}
			text := strings.TrimSpace(string(data)[lineStart:lineEnd])
			if len(text) > 160 {
				text = text[:160] + "…"
			}
			hits = append(hits, FileSearchHit{
				Path:      f.RelativePath,
				Line:      line,
				Text:      text,
				ByteStart: abs,
			})
			perFile++
			start = abs + len(query)
		}
	}
	return hits
}

// isBinary reports whether data looks binary (NUL byte in the first 8 KB).
func isBinary(data []byte) bool {
	n := len(data)
	if n > 8192 {
		n = 8192
	}
	return bytes.IndexByte(data[:n], 0) != -1
}

// RenameFile renames or moves a file/folder in the workspace, locally or
// over SFTP for remote workspaces.
func (ws *WorkspaceState) RenameFile(oldRel, newRel string) error {
	oldPath := ws.ResolveFullPath(oldRel)
	newPath := ws.ResolveFullPath(newRel)
	if strings.HasPrefix(oldPath, "ssh://") {
		if ws.RemoteFS == nil {
			return fmt.Errorf("not connected")
		}
		_, err := ws.RemoteFS.RenameFile(oldPath, newPath)
		ws.Refresh()
		return err
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		return err
	}
	ws.Refresh()
	return nil
}
