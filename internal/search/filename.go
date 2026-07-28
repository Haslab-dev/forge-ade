package search

import (
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/armon/go-radix"
)

// FileEntry stores metadata about an indexed file.
type FileEntry struct {
	Path      string
	Name      string
	Extension string
	LowerName string // lowercase for case-insensitive matching
	Dir       string
}

// FilenameIndex provides instant filename search using a radix tree.
type FilenameIndex struct {
	mu     sync.RWMutex
	tree   *radix.Tree
	files  map[string]*FileEntry // path -> entry
	count  int
}

// NewFilenameIndex creates a new filename index.
func NewFilenameIndex() *FilenameIndex {
	return &FilenameIndex{
		tree:  radix.New(),
		files: make(map[string]*FileEntry),
	}
}

// Insert adds a file to the index.
func (fi *FilenameIndex) Insert(path string) {
	fi.mu.Lock()
	defer fi.mu.Unlock()

	name := filepath.Base(path)
	entry := &FileEntry{
		Path:      path,
		Name:      name,
		Extension: filepath.Ext(name),
		LowerName: strings.ToLower(name),
		Dir:       filepath.Dir(path),
	}

	fi.files[path] = entry
	fi.count++

	// Insert into trie with all possible query forms:
	// 1. The full name (for exact matching)
	// 2. Lowercase version (for case-insensitive)
	fi.tree.Insert(entry.LowerName, path)
}

// Remove deletes a file from the index.
func (fi *FilenameIndex) Remove(path string) {
	fi.mu.Lock()
	defer fi.mu.Unlock()

	if entry, ok := fi.files[path]; ok {
		fi.tree.Delete(entry.LowerName)
		delete(fi.files, path)
		fi.count--
	}
}

// Search returns files whose names match the query prefix.
// Uses the radix tree for O(len(query)) lookup.
func (fi *FilenameIndex) Search(query string, limit int) []*FileEntry {
	fi.mu.RLock()
	defer fi.mu.RUnlock()

	if fi.count == 0 {
		return nil
	}

	lowerQuery := strings.ToLower(query)
	var results []*FileEntry

	// Walk the trie using the query as prefix
	fi.tree.WalkPrefix(lowerQuery, func(key string, value interface{}) bool {
		path := value.(string)
		if entry, ok := fi.files[path]; ok {
			results = append(results, entry)
		}
		return len(results) >= limit
	})

	// Also check for substring matches (for cases where prefix doesn't match but substring does)
	if len(results) < limit && len(query) >= 2 {
		fi.walkAll(func(entry *FileEntry) bool {
			if len(results) >= limit {
				return true
			}
			if strings.Contains(entry.LowerName, lowerQuery) {
				// Check if already added
				for _, r := range results {
					if r.Path == entry.Path {
						return false
					}
				}
				results = append(results, entry)
			}
			return false
		})
	}

	return results
}

// FuzzySearch implements VS Code-like fuzzy matching.
// Query "wm" matches "WorkspaceManager.go" (Word boundary + CamelCase).
func (fi *FilenameIndex) FuzzySearch(query string, limit int) []*FileEntry {
	fi.mu.RLock()
	defer fi.mu.RUnlock()

	if fi.count == 0 || len(query) == 0 {
		return nil
	}

	type scored struct {
		entry *FileEntry
		score int
	}

	lowerQuery := strings.ToLower(query)
	var scoredResults []scored

	fi.walkAll(func(entry *FileEntry) bool {
		score := fuzzyScore(query, lowerQuery, entry.Name, entry.LowerName)

		if score > 0 {
			scoredResults = append(scoredResults, scored{entry, score})
		}

		return false // continue walking
	})

	// Sort by score descending, then by name
	sort.Slice(scoredResults, func(i, j int) bool {
		if scoredResults[i].score != scoredResults[j].score {
			return scoredResults[i].score > scoredResults[j].score
		}
		return scoredResults[i].entry.Name < scoredResults[j].entry.Name
	})

	// Apply limit
	if len(scoredResults) > limit {
		scoredResults = scoredResults[:limit]
	}

	results := make([]*FileEntry, len(scoredResults))
	for i, sr := range scoredResults {
		results[i] = sr.entry
	}

	return results
}

// Count returns the number of indexed files.
func (fi *FilenameIndex) Count() int {
	fi.mu.RLock()
	defer fi.mu.RUnlock()
	return fi.count
}

// GetAll returns all indexed entries (for serialization / rebuild).
func (fi *FilenameIndex) GetAll() []*FileEntry {
	fi.mu.RLock()
	defer fi.mu.RUnlock()

	entries := make([]*FileEntry, 0, fi.count)
	for _, entry := range fi.files {
		entries = append(entries, entry)
	}
	return entries
}

func (fi *FilenameIndex) walkAll(fn func(entry *FileEntry) bool) {
	for _, entry := range fi.files {
		if fn(entry) {
			return
		}
	}
}

// fuzzyScore returns a score for how well `query` matches `name`.
// Higher is better. Returns 0 if no match.
// Implements: consecutive char bonus, word boundary bonus, camelCase bonus,
// filename bonus, exact prefix bonus.
func fuzzyScore(query, lowerQuery, name, lowerName string) int {
	// Exact match → perfect score
	if lowerName == lowerQuery {
		return 1000
	}

	// Prefix match → high score
	if strings.HasPrefix(lowerName, lowerQuery) {
		return 800 + len(query)
	}

	// Check if all query chars exist in order in the name
	qIdx := 0
	score := 0
	prevMatched := false

	for i := 0; i < len(lowerName) && qIdx < len(lowerQuery); i++ {
		if lowerName[i] == lowerQuery[qIdx] {
			// Character matched
			bonus := 10 // base

			// Consecutive character bonus
			if prevMatched {
				bonus += 5
			}

			// Word boundary bonus (after separator or start)
			if i == 0 || lowerName[i-1] == '/' || lowerName[i-1] == '_' ||
				lowerName[i-1] == '-' || lowerName[i-1] == '.' {
				bonus += 20
			}

			// CamelCase bonus (uppercase letter in original name)
			if i > 0 && name[i] >= 'A' && name[i] <= 'Z' {
				bonus += 15
			}

			// Path separator bonus (after /)
			if i > 0 && lowerName[i-1] == '/' {
				bonus += 30
			}

			score += bonus
			prevMatched = true
			qIdx++
		} else {
			prevMatched = false
		}
	}

	// All characters matched?
	if qIdx == len(lowerQuery) {
		// Bonus for filename matching (not path)
		if !strings.Contains(name, "/") {
			score += 40
		}

		// Bonus for matching extension
		ext := filepath.Ext(name)
		if strings.Contains(ext, lowerQuery) {
			score += 10
		}

		return score
	}

	return 0 // not all chars matched
}
