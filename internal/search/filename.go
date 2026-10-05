package search

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// FileEntry stores metadata about an indexed file.
type FileEntry struct {
	Path         string `json:"path"`
	RelPath      string `json:"relPath"`
	Name         string `json:"name"`
	Extension    string `json:"extension"`
	LowerName    string `json:"-"`
	LowerRelPath string `json:"-"`
	Dir          string `json:"dir"`
}

// FilenameIndex provides instant filename and path search using a high-performance in-memory index.
type FilenameIndex struct {
	mu    sync.RWMutex
	files map[string]*FileEntry // path -> entry
	count int
}

// NewFilenameIndex creates a new filename index.
func NewFilenameIndex() *FilenameIndex {
	return &FilenameIndex{
		files: make(map[string]*FileEntry),
	}
}

// Insert adds or updates a file in the index.
func (fi *FilenameIndex) Insert(path, rootDir string) {
	fi.mu.Lock()
	defer fi.mu.Unlock()

	rel := path
	if rootDir != "" {
		if r, err := filepath.Rel(rootDir, path); err == nil {
			rel = r
		}
	}

	name := filepath.Base(path)
	entry := &FileEntry{
		Path:         path,
		RelPath:      rel,
		Name:         name,
		Extension:    filepath.Ext(name),
		LowerName:    strings.ToLower(name),
		LowerRelPath: strings.ToLower(rel),
		Dir:          filepath.Dir(path),
	}

	if _, exists := fi.files[path]; !exists {
		fi.count++
	}
	fi.files[path] = entry
}

// Remove deletes a file from the index.
func (fi *FilenameIndex) Remove(path string) {
	fi.mu.Lock()
	defer fi.mu.Unlock()

	if _, ok := fi.files[path]; ok {
		delete(fi.files, path)
		fi.count--
	}
}

// Clear resets all entries.
func (fi *FilenameIndex) Clear() {
	fi.mu.Lock()
	defer fi.mu.Unlock()

	fi.files = make(map[string]*FileEntry)
	fi.count = 0
}

type scoredEntry struct {
	entry *FileEntry
	score int
}

// SearchWithOptions performs filename search considering MatchCase, MatchWholeWord, and UseRegex.
func (fi *FilenameIndex) SearchWithOptions(opts SearchOptions) []*FileEntry {
	fi.mu.RLock()
	defer fi.mu.RUnlock()

	query := strings.TrimSpace(opts.Query)
	if fi.count == 0 || query == "" {
		return nil
	}

	var re *regexp.Regexp
	if opts.UseRegex || opts.MatchWholeWord {
		pattern := query
		if !opts.UseRegex {
			pattern = regexp.QuoteMeta(query)
		}
		if opts.MatchWholeWord {
			pattern = `\b` + pattern + `\b`
		}
		if !opts.MatchCase {
			pattern = `(?i)` + pattern
		}
		var err error
		re, err = regexp.Compile(pattern)
		if err != nil {
			return nil
		}
	}

	var scoredResults []scoredEntry

	for _, entry := range fi.files {
		if re != nil {
			if re.MatchString(entry.Name) || re.MatchString(entry.RelPath) {
				scoredResults = append(scoredResults, scoredEntry{entry: entry, score: 1000})
			}
		} else {
			score := scoreFile(query, entry, opts.MatchCase)
			if score > 0 {
				scoredResults = append(scoredResults, scoredEntry{entry: entry, score: score})
			}
		}
	}

	if len(scoredResults) == 0 {
		return nil
	}

	sort.Slice(scoredResults, func(i, j int) bool {
		if scoredResults[i].score != scoredResults[j].score {
			return scoredResults[i].score > scoredResults[j].score
		}
		if len(scoredResults[i].entry.Path) != len(scoredResults[j].entry.Path) {
			return len(scoredResults[i].entry.Path) < len(scoredResults[j].entry.Path)
		}
		return scoredResults[i].entry.Name < scoredResults[j].entry.Name
	})

	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}
	if len(scoredResults) > limit {
		scoredResults = scoredResults[:limit]
	}

	results := make([]*FileEntry, len(scoredResults))
	for i, sr := range scoredResults {
		results[i] = sr.entry
	}
	return results
}

func (fi *FilenameIndex) Search(query string, limit int) []*FileEntry {
	return fi.SearchWithOptions(SearchOptions{
		Query: query,
		Limit: limit,
	})
}

func (fi *FilenameIndex) FuzzySearch(query string, limit int) []*FileEntry {
	return fi.Search(query, limit)
}

func (fi *FilenameIndex) Count() int {
	fi.mu.RLock()
	defer fi.mu.RUnlock()
	return fi.count
}

func scoreFile(query string, entry *FileEntry, matchCase bool) int {
	name := entry.Name
	relPath := entry.RelPath
	targetQuery := query

	if !matchCase {
		name = entry.LowerName
		relPath = entry.LowerRelPath
		targetQuery = strings.ToLower(query)
	}

	// Exact match
	if name == targetQuery {
		return 10000
	}
	if relPath == targetQuery {
		return 9500
	}

	// Prefix match
	if strings.HasPrefix(name, targetQuery) {
		return 8000 + (100 - len(name))
	}
	if strings.HasPrefix(relPath, targetQuery) {
		return 7000 + (100 - len(relPath))
	}

	// Substring (contains) match
	if idx := strings.Index(name, targetQuery); idx >= 0 {
		return 6000 - idx*10
	}
	if idx := strings.Index(relPath, targetQuery); idx >= 0 {
		return 5000 - idx*10
	}

	// No match. Search uses strict "contains" semantics: a file is returned only
	// when the query is an actual substring of its name or relative path
	// (case-insensitive by default). Fuzzy letter-diffusion matches are avoided
	// so results never appear when the query is not literally present.
	return 0
}
