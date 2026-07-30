package explorer

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/hasdev/forge-ade/internal/events"
)

// FileInfo contains information about a file or directory entry.
type FileInfo struct {
	Name         string      `json:"name"`
	Path         string      `json:"path"`
	IsDir        bool        `json:"isDir"`
	Size         int64       `json:"size"`
	Mode         string      `json:"mode"`
	ModTime      string      `json:"modTime"`
	Symlink      bool        `json:"symlink"`
	SymlinkTarget string     `json:"symlinkTarget,omitempty"`
	Children     []*FileInfo `json:"children,omitempty"`
	Hidden       bool        `json:"hidden"`
	GitIgnored   bool        `json:"gitIgnored"`
}

// Explorer manages file tree browsing across workspace folders.
type Explorer struct {
	bus        *events.Bus
	mu         sync.RWMutex
	roots      []string
	showHidden bool
}

// New creates a new Explorer.
func New(bus *events.Bus) *Explorer {
	return &Explorer{
		bus:        bus,
		showHidden: true,
	}
}

// SetRoots sets the workspace root folders.
func (e *Explorer) SetRoots(roots []string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.roots = roots
}

// GetRoots returns the current root folders.
func (e *Explorer) GetRoots() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	roots := make([]string, len(e.roots))
	copy(roots, e.roots)
	return roots
}

// ListDirectory returns the contents of a directory at depth 1.
func (e *Explorer) ListDirectory(dirPath string) ([]*FileInfo, error) {
	return e.readDir(dirPath, false, 0)
}

// GetTree returns the full file tree for all root folders.
// depth controls how deep to recurse (0 = single level, -1 = full tree).
func (e *Explorer) GetTree(depth int) ([]*FileInfo, error) {
	e.mu.RLock()
	roots := e.roots
	showHidden := e.showHidden
	e.mu.RUnlock()

	var tree []*FileInfo
	for _, root := range roots {
		info, err := e.getFileInfo(root)
		if err != nil {
			continue
		}
		if depth != 0 {
			children, err := e.readDir(root, showHidden, depth-1)
			if err == nil {
				info.Children = children
			}
		}
		tree = append(tree, info)
	}
	return tree, nil
}

// ExpandPath returns the children of the given directory.
func (e *Explorer) ExpandPath(targetPath string) ([]*FileInfo, error) {
	// Ensure the path exists
	if _, err := os.Stat(targetPath); err != nil {
		return nil, err
	}

	e.mu.RLock()
	showHidden := e.showHidden
	e.mu.RUnlock()

	// Return the directory's own children, not its siblings
	children, err := e.readDir(targetPath, showHidden, 0)
	if err != nil {
		return nil, err
	}
	return children, nil
}

// SetShowHidden toggles hidden file visibility.
func (e *Explorer) SetShowHidden(show bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.showHidden = show
}

// GetShowHidden returns whether hidden files are shown.
func (e *Explorer) GetShowHidden() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.showHidden
}

func (e *Explorer) readDir(dirPath string, showHidden bool, depth int) ([]*FileInfo, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	gitignorePatterns := loadGitignore(dirPath)

	var files []*FileInfo
	for _, entry := range entries {
		if !showHidden && isHidden(entry.Name()) {
			continue
		}

		fullPath := filepath.Join(dirPath, entry.Name())
		info, err := e.getEntryInfo(entry, fullPath, gitignorePatterns)
		if err != nil {
			continue
		}

		if info.IsDir && depth != 0 {
			children, err := e.readDir(fullPath, showHidden, depth-1)
			if err == nil {
				info.Children = children
			}
		}

		files = append(files, info)
	}

	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDir != files[j].IsDir {
			return files[i].IsDir // directories first
		}
		return files[i].Name < files[j].Name
	})

	return files, nil
}

func (e *Explorer) getEntryInfo(entry fs.DirEntry, fullPath string, gitignorePatterns []string) (*FileInfo, error) {
	info, err := entry.Info()
	if err != nil {
		return nil, err
	}

	fi := &FileInfo{
		Name:    entry.Name(),
		Path:    fullPath,
		IsDir:   entry.IsDir(),
		Size:    info.Size(),
		Mode:    info.Mode().String(),
		ModTime: info.ModTime().Format("2006-01-02 15:04:05"),
		Hidden:  isHidden(entry.Name()),
		GitIgnored: isGitignored(fullPath, gitignorePatterns),
	}

	if entry.Type()&os.ModeSymlink != 0 {
		fi.Symlink = true
		if target, err := os.Readlink(fullPath); err == nil {
			fi.SymlinkTarget = target
		}
	}

	return fi, nil
}

func (e *Explorer) getFileInfo(path string) (*FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	return &FileInfo{
		Name:    filepath.Base(path),
		Path:    path,
		IsDir:   info.IsDir(),
		Size:    info.Size(),
		Mode:    info.Mode().String(),
		ModTime: info.ModTime().Format("2006-01-02 15:04:05"),
		Hidden:  isHidden(filepath.Base(path)),
		GitIgnored: isGitignored(path, loadGitignore(filepath.Dir(path))),
	}, nil
}

func isHidden(name string) bool {
	return len(name) > 0 && name[0] == '.'
}

func loadGitignore(dir string) []string {
	raw, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		return nil
	}
	var patterns []string
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}
	return patterns
}

func isGitignored(path string, patterns []string) bool {
	for _, pattern := range patterns {
		if pattern == "" {
			continue
		}
		name := filepath.Base(path)
		// Simple glob match: support * and ? and leading /
		if strings.HasPrefix(pattern, "/") {
			// Absolute from repo root
			if matched, _ := filepath.Match(pattern[1:], name); matched {
				return true
			}
		} else if strings.HasSuffix(pattern, "/") {
			// Directory pattern
			if info, err := os.Stat(path); err == nil && info.IsDir() {
				if matched, _ := filepath.Match(pattern[:len(pattern)-1], name); matched {
					return true
				}
			}
		} else {
			if matched, _ := filepath.Match(pattern, name); matched {
				return true
			}
			// Also check if path contains the pattern (for dir/** patterns)
			if strings.Contains(path, "/"+pattern) {
				return true
			}
		}
	}
	return false
}
