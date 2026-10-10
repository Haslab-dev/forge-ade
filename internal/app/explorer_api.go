package app

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/hasdev/forge-ade/internal/explorer"
	"github.com/hasdev/forge-ade/internal/git"
)

// ---------------------------------------------------------------------------
// Explorer API
// ---------------------------------------------------------------------------

// GetFileTree returns the file tree for explorer roots.
func (a *App) GetFileTree(depth int) (string, error) {
	roots := a.explorer.GetRoots()
	if len(roots) > 0 && strings.HasPrefix(roots[0], "ssh://") {
		entries, isRemote, err := a.remoteFS.ListDirectory(roots[0])
		if isRemote {
			if err != nil {
				return "", err
			}
			data, _ := json.Marshal(entries)
			return string(data), nil
		}
	}
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
	if entries, isRemote, err := a.remoteFS.ListDirectory(dirPath); isRemote {
		if err != nil {
			return "", err
		}
		data, _ := json.Marshal(entries)
		return string(data), nil
	}
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
	if entries, isRemote, err := a.remoteFS.ListDirectory(targetPath); isRemote {
		if err != nil {
			return "", err
		}
		data, _ := json.Marshal(entries)
		return string(data), nil
	}
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
