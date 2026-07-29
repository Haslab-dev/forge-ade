package search

import (
	"bufio"
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/hasdev/forge-ade/internal/gitignore"
)

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
		Path       struct {
			Text string `json:"text"`
		} `json:"path"`
		Lines      struct {
			Text string `json:"text"`
		} `json:"lines"`
		LineNumber int `json:"line_number"`
	} `json:"data"`
}

// SearchManager handles filename search (memory) and content search (ripgrep).
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

// SetDirectories sets workspace folders for indexing.
func (sm *SearchManager) SetDirectories(dirs []string) {
	sm.mu.Lock()
	sm.dirs = dirs
	sm.mu.Unlock()
}

// Start builds the filename index in background.
func (sm *SearchManager) Start() {
	go sm.buildInitialIndex()
}

// Stop is a no-op — nothing to stop.
func (sm *SearchManager) Stop() {}

// IndexFile adds a single file to the filename index.
func (sm *SearchManager) IndexFile(path string) {
	sm.filename.Insert(path)
}

// RemoveFile removes a file from the filename index.
func (sm *SearchManager) RemoveFile(path string) {
	sm.filename.Remove(path)
}

// SearchFilename performs instant filename search (memory, ~5-15MB for 50k files).
func (sm *SearchManager) SearchFilename(query string, limit int) []RankedResult {
	if query == "" {
		return nil
	}
	entries := sm.filename.FuzzySearch(query, limit)
	if len(entries) == 0 {
		entries = sm.filename.Search(query, limit)
	}
	if len(entries) == 0 {
		return nil
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

// SearchContent runs ripgrep on-demand. Zero memory when idle.
func (sm *SearchManager) SearchContent(query string, limit int) ([]RankedResult, error) {
	if query == "" {
		return nil, nil
	}

	sm.mu.RLock()
	dirs := make([]string, len(sm.dirs))
	copy(dirs, sm.dirs)
	sm.mu.RUnlock()

	if len(dirs) == 0 {
		return nil, nil
	}

	// ripgrep respects .gitignore natively
	args := []string{
		query,
		"--json",
		"--line-number",
		"--max-count", "50",
		"--follow",
		"--smart-case",
		"--trim",
		"--no-heading",
	}
	args = append(args, dirs...)

	cmd := exec.Command("rg", args...)
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
		if len(results) >= limit {
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

	cmd.Wait()
	return results, scanner.Err()
}

// Stats returns index statistics.
func (sm *SearchManager) Stats() map[string]int {
	return map[string]int{
		"files": sm.filename.Count(),
	}
}

func (sm *SearchManager) buildInitialIndex() {
	sm.mu.RLock()
	dirs := make([]string, len(sm.dirs))
	copy(dirs, sm.dirs)
	sm.mu.RUnlock()

	if len(dirs) == 0 {
		return
	}

	log.Printf("search: building filename index for %d directories", len(dirs))

	// Hard skip dirs to protect against index bloat when no .gitignore exists
	skipDirs := map[string]bool{
		"node_modules": true, ".git": true, ".svn": true,
		"pods": true, ".xcworkspace": true, ".xcodeproj": true,
		"deriveddata": true, ".build": true,
		".swiftpm": true, "carthage": true,
		"vendor": true, ".next": true, ".cache": true,
		"dist": true, "build": true, "coverage": true,
		"__pycache__": true, ".hg": true, ".bzr": true,
		".yarn": true,
	}

	var fileCount int
	for _, dir := range dirs {
		gi := gitignore.Load(dir)
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				n := strings.ToLower(info.Name())
				if skipDirs[n] {
					return filepath.SkipDir
				}
				if gi != nil && gi.MatchDir(info.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			sm.filename.Insert(path)
			fileCount++
			return nil
		})
	}

	log.Printf("search: filename index built — %d files", fileCount)
}
