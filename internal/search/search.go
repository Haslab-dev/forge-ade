package search

import (
	"bufio"
	"bytes"
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"unicode"

	"github.com/hasdev/forge-ade/internal/gitignore"
)

// SearchOptions configures search behavior (matching VS Code options).
type SearchOptions struct {
	Query          string `json:"query"`
	MatchCase      bool   `json:"matchCase"`
	MatchWholeWord bool   `json:"matchWholeWord"`
	UseRegex       bool   `json:"useRegex"`
	Glob           string `json:"glob,omitempty"` // optional glob pattern (e.g. "*.tsx", "src/**/*.go")
	Limit          int    `json:"limit"`
}

// RankedResult is a search result with a score.
type RankedResult struct {
	Path     string  `json:"path"`
	Filename string  `json:"filename"`
	Score    float64 `json:"score"`
	Line     int     `json:"line,omitempty"`
	Content  string  `json:"content,omitempty"`
}

// rgLine represents one line of ripgrep JSON output.
type rgLine struct {
	Type string `json:"type"`
	Data struct {
		Path struct {
			Text string `json:"text"`
		} `json:"path"`
		Lines struct {
			Text string `json:"text"`
		} `json:"lines"`
		LineNumber int `json:"line_number"`
	} `json:"data"`
}

// SearchManager handles filename search (memory) and content search (ripgrep + pure Go fallback).
type SearchManager struct {
	filename *FilenameIndex
	dirs     []string
	mu       sync.RWMutex
}

// NewSearchManager creates a new search manager.
func NewSearchManager() *SearchManager {
	return &SearchManager{
		filename: NewFilenameIndex(),
	}
}

// SetDirectories sets workspace folders for indexing and rebuilds the index.
func (sm *SearchManager) SetDirectories(dirs []string) {
	sm.mu.Lock()
	sm.dirs = dirs
	sm.mu.Unlock()
	sm.buildInitialIndex()
}

// Start builds the filename index in background.
func (sm *SearchManager) Start() {
	go sm.buildInitialIndex()
}

// Stop resets the manager.
func (sm *SearchManager) Stop() {
	sm.filename.Clear()
}

// IndexFile adds a single file to the filename index.
func (sm *SearchManager) IndexFile(path string) {
	sm.mu.RLock()
	rootDir := ""
	if len(sm.dirs) > 0 {
		rootDir = sm.dirs[0]
	}
	sm.mu.RUnlock()
	sm.filename.Insert(path, rootDir)
}

// RemoveFile removes a file from the filename index.
func (sm *SearchManager) RemoveFile(path string) {
	sm.filename.Remove(path)
}

// SearchFilename performs instant filename search with options.
func (sm *SearchManager) SearchFilenameWithOptions(opts SearchOptions) []RankedResult {
	opts.Query = strings.TrimSpace(opts.Query)
	if opts.Query == "" {
		return []RankedResult{}
	}

	if sm.filename.Count() == 0 {
		sm.buildInitialIndex()
	}

	entries := sm.filename.SearchWithOptions(opts)
	if len(entries) == 0 {
		return []RankedResult{}
	}

	if g, err := compileGlob(opts.Glob); err == nil && g != nil {
		filtered := entries[:0]
		for _, e := range entries {
			if g.match(e.RelPath) || g.match(e.Name) {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}
	if len(entries) == 0 {
		return []RankedResult{}
	}

	results := make([]RankedResult, 0, len(entries))
	for _, e := range entries {
		results = append(results, RankedResult{
			Path:     e.Path,
			Filename: e.Name,
			Score:    100,
		})
	}
	return results
}

// SearchFilename legacy wrapper.
func (sm *SearchManager) SearchFilename(query string, limit int) []RankedResult {
	return sm.SearchFilenameWithOptions(SearchOptions{Query: query, Limit: limit})
}

// findRipgrepPath attempts to locate ripgrep binary across system paths.
func findRipgrepPath() string {
	if path, err := exec.LookPath("rg"); err == nil {
		return path
	}

	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(home, "homebrew", "bin", "rg"),
		"/opt/homebrew/bin/rg",
		"/usr/local/bin/rg",
		filepath.Join(home, ".cargo", "bin", "rg"),
		"/usr/bin/rg",
		`C:\Program Files\ripgrep\rg.exe`,
	}

	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c
		}
	}
	return ""
}

// SearchContentWithOptions searches file contents with full MatchCase, MatchWholeWord, and UseRegex options.
func (sm *SearchManager) SearchContentWithOptions(opts SearchOptions) ([]RankedResult, error) {
	opts.Query = strings.TrimSpace(opts.Query)
	if opts.Query == "" {
		return []RankedResult{}, nil
	}
	if opts.Limit <= 0 {
		opts.Limit = 50
	}

	sm.mu.RLock()
	dirs := make([]string, len(sm.dirs))
	copy(dirs, sm.dirs)
	sm.mu.RUnlock()

	if len(dirs) == 0 {
		return []RankedResult{}, nil
	}

	rgPath := findRipgrepPath()
	if rgPath != "" {
		results, err := sm.searchContentRipgrep(rgPath, opts, dirs)
		if err == nil && len(results) > 0 {
			return results, nil
		}
		if err != nil {
			log.Printf("search: ripgrep error (%v), falling back to pure-Go engine", err)
		}
	}

	// Fallback to high-speed multi-threaded pure Go search engine
	return sm.searchContentGo(opts, dirs)
}

// SearchContent legacy wrapper.
func (sm *SearchManager) SearchContent(query string, limit int) ([]RankedResult, error) {
	return sm.SearchContentWithOptions(SearchOptions{Query: query, Limit: limit})
}

func (sm *SearchManager) searchContentRipgrep(rgPath string, opts SearchOptions, dirs []string) ([]RankedResult, error) {
	args := []string{
		"--json",
		"--line-number",
		"--follow",
		"--trim",
		"--no-heading",
	}

	if opts.MatchCase {
		args = append(args, "--case-sensitive")
	} else {
		args = append(args, "--ignore-case")
	}

	if opts.MatchWholeWord {
		args = append(args, "--word-regexp")
	}

	if !opts.UseRegex {
		args = append(args, "--fixed-strings")
	}

	if strings.TrimSpace(opts.Glob) != "" {
		args = append(args, "--glob", opts.Glob)
	}

	args = append(args, "--regexp", opts.Query)
	args = append(args, dirs...)

	cmd := exec.Command(rgPath, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	var results []RankedResult
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*64), 1024*64)

	for scanner.Scan() {
		if len(results) >= opts.Limit {
			// Kill rg: once we stop reading the pipe it fills up (64KB) and rg
			// blocks forever in write(), deadlocking cmd.Wait() below.
			cmd.Process.Kill()
			break
		}
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var r rgLine
		if err := json.Unmarshal(line, &r); err != nil {
			continue
		}
		if r.Type != "match" {
			continue
		}
		results = append(results, RankedResult{
			Path:     r.Data.Path.Text,
			Filename: filepath.Base(r.Data.Path.Text),
			Line:     r.Data.LineNumber,
			Content:  strings.TrimSpace(r.Data.Lines.Text),
			Score:    100,
		})
	}

	_ = cmd.Wait()
	return results, nil
}

// ReplaceOptions configures a search-and-replace operation.
type ReplaceOptions struct {
	SearchOptions
	Replacement  string `json:"replacement"`
	PreserveCase bool   `json:"preserveCase"`
}

// ReplaceResult reports the outcome of a replace operation.
type ReplaceResult struct {
	FilesChanged      int      `json:"filesChanged"`
	TotalReplacements int      `json:"totalReplacements"`
	Files             []string `json:"files"`
}

func buildReplaceRegex(opts SearchOptions) (*regexp.Regexp, error) {
	pattern := opts.Query
	if !opts.UseRegex {
		pattern = regexp.QuoteMeta(pattern)
	}
	if opts.MatchWholeWord {
		pattern = `\b(?:` + pattern + `)\b`
	}
	if !opts.MatchCase {
		pattern = `(?i)` + pattern
	}
	return regexp.Compile(pattern)
}

// applyPreserveCase mirrors VS Code's preserve-case option: an ALL-CAPS match
// uppercases the replacement, a capitalized match capitalizes the replacement's
// first letter, anything else uses the replacement as typed.
func applyPreserveCase(match, replacement string) string {
	if match == strings.ToUpper(match) && strings.ToLower(match) != match {
		return strings.ToUpper(replacement)
	}
	r := []rune(match)
	if len(r) > 0 && unicode.IsUpper(r[0]) {
		rr := []rune(replacement)
		if len(rr) > 0 {
			rr[0] = unicode.ToUpper(rr[0])
		}
		return string(rr)
	}
	return replacement
}

// ReplaceAll replaces every occurrence of the query in every matching file and
// re-indexes the changed files. Writes go straight to disk, so the file
// watcher emits change events that keep the explorer and open tabs in sync.
func (sm *SearchManager) ReplaceAll(opts ReplaceOptions) (ReplaceResult, error) {
	opts.Query = strings.TrimSpace(opts.Query)
	if opts.Query == "" {
		return ReplaceResult{}, nil
	}
	re, err := buildReplaceRegex(opts.SearchOptions)
	if err != nil {
		return ReplaceResult{}, err
	}

	sm.mu.RLock()
	dirs := make([]string, len(sm.dirs))
	copy(dirs, sm.dirs)
	sm.mu.RUnlock()
	if len(dirs) == 0 {
		return ReplaceResult{}, nil
	}

	var result ReplaceResult
	skipDirs := map[string]bool{
		"node_modules": true, "vendor": true, ".git": true, "dist": true,
		"build": true, "coverage": true, "__pycache__": true, ".hg": true,
		".bzr": true,
	}
	for _, dir := range dirs {
		gi := gitignore.Load(dir)
		_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
			if err != nil || info == nil {
				return nil
			}
			if info.IsDir() {
				n := strings.ToLower(info.Name())
				if skipDirs[n] || (gi != nil && gi.MatchDir(info.Name())) {
					return filepath.SkipDir
				}
				return nil
			}
			if info.Size() > 5*1024*1024 || isBinaryExt(info.Name()) {
				return nil
			}
			content, rerr := os.ReadFile(p)
			if rerr != nil {
				return nil
			}
			if bytes.IndexByte(content, 0) != -1 || !re.Match(content) {
				return nil
			}
			s := string(content)
			out := re.ReplaceAllStringFunc(s, func(m string) string {
				if opts.PreserveCase {
					return applyPreserveCase(m, opts.Replacement)
				}
				return opts.Replacement
			})
			if out == s {
				return nil
			}
			if werr := os.WriteFile(p, []byte(out), info.Mode()); werr != nil {
				return nil
			}
			result.FilesChanged++
			result.TotalReplacements += len(re.FindAllString(s, -1))
			result.Files = append(result.Files, p)
			sm.IndexFile(p)
			return nil
		})
	}
	return result, nil
}

func createMatcher(opts SearchOptions) func(line string) bool {
	pattern := opts.Query
	if !opts.UseRegex {
		pattern = regexp.QuoteMeta(opts.Query)
	}
	if opts.MatchWholeWord {
		pattern = `\b` + pattern + `\b`
	}
	if !opts.MatchCase {
		pattern = `(?i)` + pattern
	}
	re, err := regexp.Compile(pattern)
	if err == nil {
		return func(line string) bool {
			return re.MatchString(line)
		}
	}

	lowerQ := strings.ToLower(opts.Query)
	return func(line string) bool {
		if opts.MatchCase {
			return strings.Contains(line, opts.Query)
		}
		return strings.Contains(strings.ToLower(line), lowerQ)
	}
}

// Pure Go concurrent content search engine (fast multi-threaded worker pool fallback)
func (sm *SearchManager) searchContentGo(opts SearchOptions, dirs []string) ([]RankedResult, error) {
	matcher := createMatcher(opts)
	g, _ := compileGlob(opts.Glob)
	skipDirs := map[string]bool{
		"node_modules": true, ".git": true, ".svn": true,
		"pods": true, ".xcworkspace": true, ".xcodeproj": true,
		"deriveddata": true, ".build": true, ".swiftpm": true,
		"vendor": true, ".next": true, ".cache": true,
		"dist": true, "build": true, "coverage": true,
		"__pycache__": true, ".hg": true, ".bzr": true,
	}

	type searchTask struct {
		path string
	}

	numWorkers := runtime.NumCPU()
	if numWorkers < 4 {
		numWorkers = 4
	}

	tasks := make(chan searchTask, 100)
	resultsChan := make(chan RankedResult, opts.Limit*2)
	done := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range tasks {
				matches := searchFileContentMatcher(task.path, matcher)
				for _, m := range matches {
					select {
					case resultsChan <- m:
					case <-done:
						return
					default:
					}
				}
			}
		}()
	}

	go func() {
		defer close(tasks)
		for _, dir := range dirs {
			gi := gitignore.Load(dir)
			_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
				if err != nil || info == nil {
					return nil
				}
				if info.IsDir() {
					n := strings.ToLower(info.Name())
					if skipDirs[n] || (gi != nil && gi.MatchDir(info.Name())) {
						return filepath.SkipDir
					}
					return nil
				}
				// Skip binary / large files
				if info.Size() > 5*1024*1024 || isBinaryExt(info.Name()) {
					return nil
				}
				if g != nil {
					if rel, rerr := filepath.Rel(dir, p); rerr == nil && !g.match(filepath.ToSlash(rel)) {
						return nil
					}
				}
				select {
				case tasks <- searchTask{path: p}:
				case <-done:
					return filepath.SkipDir
				}
				return nil
			})
		}
	}()

	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	var results []RankedResult
	for r := range resultsChan {
		results = append(results, r)
		if len(results) >= opts.Limit {
			close(done)
			break
		}
	}

	return results, nil
}

func isBinaryExt(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".ico", ".pdf", ".zip", ".tar", ".gz", ".7z",
		".exe", ".dll", ".so", ".dylib", ".bin", ".wasm", ".db", ".sqlite", ".pyc", ".class", ".o":
		return true
	}
	return false
}

func searchFileContentMatcher(filePath string, matcher func(string) bool) []RankedResult {
	f, err := os.Open(filePath)
	if err != nil {
		return nil
	}
	defer f.Close()

	// Peek first 512 bytes for null byte check (binary file test)
	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	if bytes.IndexByte(buf[:n], 0) != -1 {
		return nil
	}
	_, _ = f.Seek(0, 0)

	var matches []RankedResult
	scanner := bufio.NewScanner(f)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if matcher(line) {
			matches = append(matches, RankedResult{
				Path:     filePath,
				Filename: filepath.Base(filePath),
				Line:     lineNum,
				Content:  strings.TrimSpace(line),
				Score:    100,
			})
			if len(matches) >= 10 {
				break
			}
		}
	}
	return matches
}

func (sm *SearchManager) buildInitialIndex() {
	sm.mu.RLock()
	dirs := make([]string, len(sm.dirs))
	copy(dirs, sm.dirs)
	sm.mu.RUnlock()

	if len(dirs) == 0 {
		return
	}

	skipDirs := map[string]bool{
		"node_modules": true, ".git": true, ".svn": true,
		"pods": true, ".xcworkspace": true, ".xcodeproj": true,
		"deriveddata": true, ".build": true, ".swiftpm": true,
		"vendor": true, ".next": true, ".cache": true,
		"dist": true, "build": true, "coverage": true,
		"__pycache__": true, ".hg": true, ".bzr": true,
	}

	sm.filename.Clear()
	var fileCount int
	for _, dir := range dirs {
		gi := gitignore.Load(dir)
		_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info == nil {
				return nil
			}
			if info.IsDir() {
				n := strings.ToLower(info.Name())
				if skipDirs[n] || (gi != nil && gi.MatchDir(info.Name())) {
					return filepath.SkipDir
				}
				return nil
			}
			sm.filename.Insert(path, dir)
			fileCount++
			return nil
		})
	}
	log.Printf("search: filename index built — %d files across %d directories", fileCount, len(dirs))
}
