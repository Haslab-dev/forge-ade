package explorer

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Haslab-dev/RakitKode/events"
	"github.com/Haslab-dev/RakitKode/gitignore"
)

// FileInfo contains information about a file or directory entry.
type FileInfo struct {
	Name          string      `json:"name"`
	Path          string      `json:"path"`
	IsDir         bool        `json:"isDir"`
	Size          int64       `json:"size"`
	Mode          string      `json:"mode"`
	ModTime       string      `json:"modTime"`
	Symlink       bool        `json:"symlink"`
	SymlinkTarget string      `json:"symlinkTarget,omitempty"`
	Children      []*FileInfo `json:"children,omitempty"`
	Hidden        bool        `json:"hidden"`
	GitIgnored    bool        `json:"gitIgnored"`
	GitStatus     string      `json:"gitStatus,omitempty"` // "U", "M", "D", or "" when clean (set by app layer)
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

const maxExplorerDepth = 20

func isHeavyDir(name string) bool {
	switch strings.ToLower(name) {
	case ".git", "node_modules", ".zig-cache", "zig-out", ".next", "dist", ".cache", ".venv", "venv", "target", "vendor", ".turbo", "build", "coverage":
		return true
	}
	return false
}

// ListDirectory returns the contents of a directory at depth 1.
func (e *Explorer) ListDirectory(dirPath string) ([]*FileInfo, error) {
	return e.readDir(dirPath, false, 0, 0)
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
			nextDepth := depth
			if nextDepth > 0 {
				nextDepth--
			}
			children, err := e.readDir(root, showHidden, nextDepth, 0)
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
	children, err := e.readDir(targetPath, showHidden, 0, 0)
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

func (e *Explorer) readDir(dirPath string, showHidden bool, depth int, currentDepth int) ([]*FileInfo, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	gi := gitignore.Load(dirPath)

	files := make([]*FileInfo, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if !showHidden && isHidden(name) {
			continue
		}

		fullPath := filepath.Join(dirPath, name)
		info, err := e.getEntryInfo(entry, fullPath, gi)
		if err != nil {
			continue
		}

		if info.IsDir && !info.Symlink {
			if isHeavyDir(name) {
				// Don't deep-traverse heavy build/vendor directories
			} else if depth != 0 && currentDepth < maxExplorerDepth {
				nextDepth := depth
				if nextDepth > 0 {
					nextDepth--
				}
				children, err := e.readDir(fullPath, showHidden, nextDepth, currentDepth+1)
				if err == nil {
					info.Children = children
				}
			}
		}

		files = append(files, info)
	}

	slices.SortFunc(files, func(a, b *FileInfo) int {
		if a.IsDir != b.IsDir {
			if a.IsDir {
				return -1
			}
			return 1
		}
		return strings.Compare(a.Name, b.Name)
	})

	return files, nil
}

func (e *Explorer) getEntryInfo(entry fs.DirEntry, fullPath string, gi *gitignore.Matcher) (*FileInfo, error) {
	info, err := entry.Info()
	if err != nil {
		return nil, err
	}

	name := entry.Name()
	ignored := false
	if gi != nil {
		ignored = gi.Match([]string{name}, entry.IsDir())
	}

	fi := &FileInfo{
		Name:       name,
		Path:       fullPath,
		IsDir:      entry.IsDir(),
		Size:       info.Size(),
		Mode:       info.Mode().String(),
		ModTime:    info.ModTime().Format(time.DateTime),
		Hidden:     isHidden(name),
		GitIgnored: ignored,
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

	name := filepath.Base(path)
	ignored := false
	dir := filepath.Dir(path)
	gi := gitignore.Load(dir)
	if gi != nil {
		ignored = gi.Match([]string{name}, info.IsDir())
	}

	return &FileInfo{
		Name:       name,
		Path:       path,
		IsDir:      info.IsDir(),
		Size:       info.Size(),
		Mode:       info.Mode().String(),
		ModTime:    info.ModTime().Format(time.DateTime),
		Hidden:     isHidden(name),
		GitIgnored: ignored,
	}, nil
}

func isHidden(name string) bool {
	return len(name) > 0 && name[0] == '.'
}
