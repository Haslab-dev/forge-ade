package explorer

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/hasdev/forge-ade/internal/events"
	"github.com/hasdev/forge-ade/internal/gitignore"
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

// ListDirectory returns the contents of a directory at depth 1.
func (e *Explorer) ListDirectory(dirPath string) ([]*FileInfo, error) {
	gi := gitignore.Load(dirPath)
	return e.readDir(dirPath, false, 0, gi)
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
	if _, err := os.Stat(targetPath); err != nil {
		return nil, err
	}

	e.mu.RLock()
	showHidden := e.showHidden
	e.mu.RUnlock()

	gi := gitignore.Load(targetPath)
	return e.readDir(targetPath, showHidden, 0, gi)
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

// readDir lists a directory, recursing up to depth levels.
// gi is the single root-level Matcher shared across the whole tree walk;
// passing it down avoids re-loading .gitignore per subdirectory and ensures
// paths are always matched relative to the git root.
func (e *Explorer) readDir(dirPath string, showHidden bool, depth int, gi *gitignore.Matcher) ([]*FileInfo, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	var files []*FileInfo
	for _, entry := range entries {
		if !showHidden && isHidden(entry.Name()) {
			continue
		}

		fullPath := filepath.Join(dirPath, entry.Name())
		info, err := e.getEntryInfo(entry, fullPath, gi)
		if err != nil {
			continue
		}

		if info.IsDir && depth != 0 {
			children, err := e.readDir(fullPath, showHidden, depth-1, gi)
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

func (e *Explorer) getEntryInfo(entry fs.DirEntry, fullPath string, gi *gitignore.Matcher) (*FileInfo, error) {
	info, err := entry.Info()
	if err != nil {
		return nil, err
	}

	// MatchAbs resolves the path relative to the git root internally,
	// so patterns in root .gitignore correctly match files in subdirectories.
	ignored := gi.MatchAbs(fullPath, entry.IsDir())

	fi := &FileInfo{
		Name:       entry.Name(),
		Path:       fullPath,
		IsDir:      entry.IsDir(),
		Size:       info.Size(),
		Mode:       info.Mode().String(),
		ModTime:    info.ModTime().Format("2006-01-02 15:04:05"),
		Hidden:     isHidden(entry.Name()),
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

	gi := gitignore.Load(filepath.Dir(path))
	ignored := gi.MatchAbs(path, info.IsDir())

	return &FileInfo{
		Name:       filepath.Base(path),
		Path:       path,
		IsDir:      info.IsDir(),
		Size:       info.Size(),
		Mode:       info.Mode().String(),
		ModTime:    info.ModTime().Format("2006-01-02 15:04:05"),
		Hidden:     isHidden(filepath.Base(path)),
		GitIgnored: ignored,
	}, nil
}

func isHidden(name string) bool {
	return len(name) > 0 && name[0] == '.'
}
