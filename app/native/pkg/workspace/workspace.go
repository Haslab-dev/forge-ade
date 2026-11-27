package workspace

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

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
	Version      uint64          // incremented whenever Files changes
	loadedDirs   map[string]bool // folders already listed lazily
}

// SnapshotVersion returns the current monotonic file-index version thread-safely.
func (ws *WorkspaceState) SnapshotVersion() uint64 {
	if ws == nil {
		return 0
	}
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	return ws.Version
}

// FirstFile returns the first indexed workspace file thread-safely.
func (ws *WorkspaceState) FirstFile() (WorkspaceFile, bool) {
	if ws == nil {
		return WorkspaceFile{}, false
	}
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	if len(ws.Files) == 0 {
		return WorkspaceFile{}, false
	}
	return ws.Files[0], true
}

// NewWorkspaceState creates and initializes a new workspace state for rootDir.
// An empty rootDir means NO workspace: files stay empty until the user opens
// a folder ("" is also what CWD=/ collapses to for Finder-launched bundles).
func NewWorkspaceState(rootDir string) *WorkspaceState {
	ws := NewWorkspaceStateLazy(rootDir)
	if !strings.HasPrefix(ws.RootDir, "ssh://") {
		ws.Refresh()
	}
	return ws
}

// NewWorkspaceStateLazy creates a workspace state without running the initial
// directory scan synchronously, allowing the caller to populate files in the
// background after the window is shown.
func NewWorkspaceStateLazy(rootDir string) *WorkspaceState {
	if rootDir == "/" {
		rootDir = ""
	}
	return &WorkspaceState{
		RootDir:    rootDir,
		loadedDirs: map[string]bool{"": true},
	}
}

// IsRemote reports whether the workspace is located on a remote SSH server.
func (ws *WorkspaceState) IsRemote() bool {
	if ws == nil {
		return false
	}
	return strings.HasPrefix(ws.RootDir, "ssh://")
}

// Refresh re-scans the workspace directory for editable files. The walk (and
// any remote SFTP listing) runs WITHOUT ws.mu held — only the final swap is
// lock-guarded — so a slow directory scan or network round-trip never blocks
// render reads or watcher-triggered refreshes (P1-3, GitManager pattern).
func (ws *WorkspaceState) Refresh() {
	ws.mu.Lock()
	root := ws.RootDir
	rfs := ws.RemoteFS
	ws.mu.Unlock()

	// No workspace opened yet — nothing to scan.
	if root == "" {
		ws.mu.Lock()
		if ws.RootDir == "" {
			ws.Files = nil
			ws.loadedDirs = map[string]bool{"": true}
			ws.Version++
		}
		ws.mu.Unlock()
		return
	}

	var files []WorkspaceFile
	maxFiles := 1200
	count := 0

	// Remote SSH directory scanning (depth 1 lazy scan — single SFTP request,
	// 0 lag). Lock-free: the listing is a network call.
	if strings.HasPrefix(root, "ssh://") && rfs != nil {
		entries, isRemote, err := rfs.ListDirectory(root)
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
			if entry.IsDir && (name == "node_modules" || name == ".git" || name == "dist" || name == "build" || name == ".next" || name == "vendor") {
				continue
			}
			rel := strings.TrimPrefix(entry.Path, root)
			rel = strings.TrimPrefix(rel, "/")
			files = append(files, WorkspaceFile{
				RelativePath: rel,
				FullPath:     entry.Path,
				IsDir:        entry.IsDir,
				Size:         entry.Size,
			})
		}

		sort.Slice(files, func(i, j int) bool {
			if files[i].IsDir != files[j].IsDir {
				return files[i].IsDir
			}
			return strings.ToLower(files[i].RelativePath) < strings.ToLower(files[j].RelativePath)
		})

		ws.mu.Lock()
		// A workspace switch during the scan must not overwrite the new
		// root's index with the old root's results.
		if ws.RootDir == root {
			ws.Files = files
			ws.loadedDirs = map[string]bool{"": true}
			ws.Version++
			if ws.SelectedPath == "" && len(files) > 0 {
				ws.SelectedPath = files[0].RelativePath
			}
		}
		ws.mu.Unlock()
		return
	}

	// Breadth-first walk so the budget spreads across the whole tree: a
	// depth-first cap would let the first big subtree (e.g. node_modules/…)
	// swallow the budget and leave the rest of the repo invisible.
	type dirEntry struct {
		path  string
		depth int
	}
	queue := []dirEntry{{path: root, depth: 0}}
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
			rel, err := filepath.Rel(root, filepath.Join(cur.path, name))
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

	ws.mu.Lock()
	if ws.RootDir == root {
		ws.Files = files
		ws.loadedDirs = map[string]bool{"": true}
		ws.Version++
		if ws.SelectedPath == "" && len(files) > 0 {
			ws.SelectedPath = files[0].RelativePath
		}
	}
	ws.mu.Unlock()
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
// LoadDir lists ONE directory live (local ~0.2ms, SFTP ~15ms) and merges it
// into the cached file index so expanding a folder in the explorer always shows its real children.
func (ws *WorkspaceState) LoadDir(rel string) {
	ws.mu.Lock()
	if ws.RootDir == "" {
		ws.mu.Unlock()
		return
	}
	if ws.loadedDirs == nil {
		ws.loadedDirs = map[string]bool{}
	}
	if ws.loadedDirs[rel] {
		ws.mu.Unlock()
		return
	}
	ws.loadedDirs[rel] = true
	rootDir := ws.RootDir
	rfs := ws.RemoteFS
	isRemote := strings.HasPrefix(rootDir, "ssh://")
	ws.mu.Unlock()

	if isRemote && rfs != nil {
		targetURI := rootDir
		if rel != "" && rel != "." {
			targetURI = strings.TrimRight(rootDir, "/") + "/" + rel
		}
		entries, ok, err := rfs.ListDirectory(targetURI)
		if !ok || err != nil {
			// Roll back loadedDirs so the folder can be retried after reconnecting.
			ws.mu.Lock()
			delete(ws.loadedDirs, rel)
			ws.mu.Unlock()
			return
		}
		var newFiles []WorkspaceFile
		for _, entry := range entries {
			if entry == nil {
				continue
			}
			name := filepath.Base(entry.Path)
			if strings.HasPrefix(name, ".") && name != ".env" {
				continue
			}
			if entry.IsDir && (name == "node_modules" || name == ".git" || name == "dist" || name == "build" || name == ".next" || name == "vendor") {
				continue
			}
			childRel := strings.TrimPrefix(entry.Path, rootDir)
			childRel = strings.TrimPrefix(childRel, "/")
			newFiles = append(newFiles, WorkspaceFile{
				RelativePath: childRel,
				FullPath:     entry.Path,
				IsDir:        entry.IsDir,
				Size:         entry.Size,
			})
		}
		ws.mu.Lock()
		known := map[string]bool{}
		for _, f := range ws.Files {
			known[f.RelativePath] = true
		}
		addedAny := false
		for _, nf := range newFiles {
			if !known[nf.RelativePath] {
				ws.Files = append(ws.Files, nf)
				addedAny = true
			}
		}
		if addedAny {
			ws.Version++
		}
		ws.mu.Unlock()
		return
	}

	root := rootDir
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
		return
	}
	ws.mu.Lock()
	known := map[string]bool{}
	for _, f := range ws.Files {
		known[f.RelativePath] = true
	}
	addedAny := false
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
				addedAny = true
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
		addedAny = true
	}
	if addedAny {
		ws.Version++
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
// It is idempotent: absolute paths already under RootDir return unchanged
// (search backends such as ripgrep return absolute paths, and joining them
// again produced /root/root/file double prefixes that broke every click
// navigation), absolute paths outside RootDir return as-is, and relative
// paths join under RootDir as before.
func (ws *WorkspaceState) ResolveFullPath(rel string) string {
	ws.mu.RLock()
	root := ws.RootDir
	ws.mu.RUnlock()

	if rel == "" {
		return root
	}
	if strings.HasPrefix(rel, "ssh://") {
		return rel
	}
	if strings.HasPrefix(root, "ssh://") {
		trimmedRel := strings.TrimPrefix(rel, "/")
		trimmedRel = strings.TrimPrefix(trimmedRel, "\\")
		return strings.TrimRight(root, "/") + "/" + trimmedRel
	}
	if filepath.IsAbs(rel) {
		return filepath.Clean(rel)
	}
	return filepath.Join(root, rel)
}

// FilesUnder returns the indexed files at rel itself or underneath it (for a
// folder: every recursive child), thread-safely.
func (ws *WorkspaceState) FilesUnder(rel string) []WorkspaceFile {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	rel = strings.TrimSuffix(strings.TrimSuffix(rel, "/"), string(filepath.Separator))
	var out []WorkspaceFile
	for _, f := range ws.Files {
		if f.RelativePath == rel || strings.HasPrefix(f.RelativePath, rel+"/") {
			out = append(out, f)
		}
	}
	return out
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
	if err := os.RemoveAll(fullPath); err != nil {
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
	searchWorkers      = 4
)

// DefaultSearchExcludeDirs lists dependency, build, cache, and VCS directories
// excluded by default (VS Code-style files.exclude / search.exclude).
var DefaultSearchExcludeDirs = map[string]bool{
	"node_modules":  true,
	".git":          true,
	".hg":           true,
	".svn":          true,
	"pods":          true,
	".gradle":       true,
	"build":         true,
	"dist":          true,
	"out":           true,
	"target":        true,
	"vendor":        true,
	".dart_tool":    true,
	".pub-cache":    true,
	".next":         true,
	".nuxt":         true,
	".output":       true,
	".turbo":        true,
	".cache":        true,
	".parcel-cache": true,
	".venv":         true,
	"venv":          true,
	"__pycache__":   true,
	".pytest_cache": true,
	".mypy_cache":   true,
	".ruff_cache":   true,
	"deriveddata":   true,
	".build":        true,
	".swiftpm":      true,
	"carthage":      true,
	".cargo":        true,
	"coverage":      true,
	"bin":           true,
	"obj":           true,
	".idea":         true,
	".vscode-test":  true,
	".bundle":       true,
}

// FileSearchOptions configures workspace file/content search with VS Code-style
// include/exclude globs and default dependency-folder exclusions.
type FileSearchOptions struct {
	Query              string
	Include            string
	Exclude            string
	UseDefaultExcludes bool
	MatchCase          bool
	UseRegex           bool
	MatchWholeWord     bool
	MaxHits            int
}

func splitSearchGlobs(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(filepath.ToSlash(p))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func matchSearchGlob(relSlash, base, pattern string) bool {
	if pattern == "" {
		return false
	}
	relLower := strings.ToLower(relSlash)
	baseLower := strings.ToLower(base)
	pat := strings.ToLower(strings.TrimPrefix(strings.TrimSuffix(pattern, "/"), "./"))
	if pat == "" {
		return false
	}
	// Exact segment match for plain folder/name patterns like "node_modules" or "Pods".
	if !strings.ContainsAny(pat, "*?/") {
		for _, seg := range strings.Split(relLower, "/") {
			if seg == pat {
				return true
			}
		}
	}
	// Strip leading "**/" for convenience when matching relative paths or basenames.
	patTrim := strings.TrimPrefix(pat, "**/")
	if !strings.Contains(patTrim, "/") {
		if ok, _ := path.Match(patTrim, baseLower); ok {
			return true
		}
	}
	if ok, _ := path.Match(pat, relLower); ok {
		return true
	}
	if ok, _ := path.Match(patTrim, relLower); ok {
		return true
	}
	// Handle "prefix/**" or "**/mid/**"
	if strings.HasSuffix(pat, "/**") {
		dir := strings.TrimSuffix(pat, "/**")
		dir = strings.TrimPrefix(dir, "**/")
		if dir != "" && (strings.HasPrefix(relLower, dir+"/") || strings.Contains(relLower, "/"+dir+"/")) {
			return true
		}
	}
	return false
}

// MatchesSearchFilters checks whether relPath passes default dependency-folder
// exclusions as well as custom include and exclude comma-separated glob patterns.
func MatchesSearchFilters(relPath, includeGlob, excludeGlob string, useDefaultExcludes bool) bool {
	relSlash := filepath.ToSlash(relPath)
	base := path.Base(relSlash)

	if useDefaultExcludes {
		for _, seg := range strings.Split(strings.ToLower(relSlash), "/") {
			if seg != "" && DefaultSearchExcludeDirs[seg] {
				return false
			}
		}
	}

	excludes := splitSearchGlobs(excludeGlob)
	for _, pat := range excludes {
		if matchSearchGlob(relSlash, base, pat) {
			return false
		}
	}

	includes := splitSearchGlobs(includeGlob)
	if len(includes) > 0 {
		matched := false
		for _, pat := range includes {
			if matchSearchGlob(relSlash, base, pat) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

// SearchFiles greps the workspace's tracked files for a case-insensitive
// substring using default dependency-folder exclusions.
func (ws *WorkspaceState) SearchFiles(ctx context.Context, query string) []FileSearchHit {
	return ws.SearchFilesWithOptions(ctx, FileSearchOptions{
		Query:              query,
		UseDefaultExcludes: true,
	})
}

// SearchFilesWithOptions performs a bounded, parallel, cancellable workspace search
// supporting VS Code-style include/exclude globs, default dependency excludes,
// case sensitivity, whole-word, and regex matching.
func (ws *WorkspaceState) SearchFilesWithOptions(ctx context.Context, opts FileSearchOptions) []FileSearchHit {
	rawQuery := strings.TrimSpace(opts.Query)
	if rawQuery == "" {
		return nil
	}
	maxHits := opts.MaxHits
	if maxHits <= 0 {
		maxHits = searchMaxHits
	}

	var re *regexp.Regexp
	if opts.UseRegex || opts.MatchWholeWord {
		pattern := rawQuery
		if !opts.UseRegex {
			pattern = regexp.QuoteMeta(pattern)
		}
		if opts.MatchWholeWord {
			pattern = `\b` + pattern + `\b`
		}
		if !opts.MatchCase {
			pattern = `(?i)` + pattern
		}
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			return nil
		}
		re = compiled
	}
	needle := rawQuery
	if !opts.MatchCase {
		needle = strings.ToLower(rawQuery)
	}

	// Snapshot & pre-filter the file list under RLock so excluded/dependency
	// folders never touch disk IO.
	ws.mu.RLock()
	candidates := make([]WorkspaceFile, 0, len(ws.Files))
	for _, f := range ws.Files {
		if f.IsDir {
			continue
		}
		if MatchesSearchFilters(f.RelativePath, opts.Include, opts.Exclude, opts.UseDefaultExcludes) {
			candidates = append(candidates, f)
		}
	}
	ws.mu.RUnlock()

	if len(candidates) == 0 {
		return nil
	}

	type indexedHits struct {
		idx  int
		hits []FileSearchHit
	}

	workers := searchWorkers
	if len(candidates) < workers {
		workers = len(candidates)
	}
	jobs := make(chan int, len(candidates))
	results := make(chan indexedHits, len(candidates))
	var totalHits atomic.Int32

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				if ctx.Err() != nil || int(totalHits.Load()) >= maxHits {
					return
				}
				f := candidates[idx]
				info, err := os.Stat(f.FullPath)
				if err != nil || info.IsDir() || info.Size() > searchMaxFileBytes {
					continue
				}
				data, err := os.ReadFile(f.FullPath)
				if err != nil || len(data) > searchMaxFileBytes || isBinary(data) {
					continue
				}

				fileHits := scanFileHits(f.RelativePath, data, needle, opts.MatchCase, re, searchMaxPerFile)
				if len(fileHits) > 0 {
					totalHits.Add(int32(len(fileHits)))
					results <- indexedHits{idx: idx, hits: fileHits}
				}
			}
		}()
	}

	for i := range candidates {
		if ctx.Err() != nil {
			break
		}
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	close(results)

	var collected []indexedHits
	for r := range results {
		collected = append(collected, r)
	}
	sort.Slice(collected, func(i, j int) bool {
		return collected[i].idx < collected[j].idx
	})

	var out []FileSearchHit
	for _, r := range collected {
		for _, h := range r.hits {
			out = append(out, h)
			if len(out) >= maxHits {
				return out
			}
		}
	}
	return out
}

func scanFileHits(relPath string, data []byte, needle string, matchCase bool, re *regexp.Regexp, maxPerFile int) []FileSearchHit {
	text := string(data)
	var hits []FileSearchHit
	if re != nil {
		locs := re.FindAllStringIndex(text, maxPerFile)
		if len(locs) == 0 {
			return nil
		}
		lastOff := 0
		line := 1
		for _, loc := range locs {
			abs := loc[0]
			line += strings.Count(text[lastOff:abs], "\n")
			lastOff = abs
			lineStart := strings.LastIndexByte(text[:abs], '\n') + 1
			lineEnd := len(text)
			if nl := strings.IndexByte(text[abs:], '\n'); nl != -1 {
				lineEnd = abs + nl
			}
			lineText := strings.TrimSpace(text[lineStart:lineEnd])
			if len(lineText) > 160 {
				lineText = truncationSafe(lineText, 160) + "…"
			}
			hits = append(hits, FileSearchHit{
				Path:      relPath,
				Line:      line,
				Text:      lineText,
				ByteStart: abs,
			})
		}
		return hits
	}

	hay := text
	if !matchCase {
		hay = strings.ToLower(text)
	}
	start := 0
	line := 1
	for len(hits) < maxPerFile {
		idx := strings.Index(hay[start:], needle)
		if idx == -1 {
			break
		}
		abs := start + idx
		line += strings.Count(hay[start:abs], "\n")
		lineStart := strings.LastIndexByte(hay[:abs], '\n') + 1
		lineEnd := len(text)
		if nl := strings.IndexByte(text[abs:], '\n'); nl != -1 {
			lineEnd = abs + nl
		}
		lineText := strings.TrimSpace(text[lineStart:lineEnd])
		if len(lineText) > 160 {
			lineText = truncationSafe(lineText, 160) + "…"
		}
		hits = append(hits, FileSearchHit{
			Path:      relPath,
			Line:      line,
			Text:      lineText,
			ByteStart: abs,
		})
		start = abs + len(needle)
	}
	return hits
}

// truncationSafe cuts s to at most max runes so multi-byte UTF-8 is never
// split mid-rune.
func truncationSafe(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
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
