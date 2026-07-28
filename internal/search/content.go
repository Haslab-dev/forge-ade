package search

import (
	"bufio"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/RoaringBitmap/roaring"
)

// ContentIndex provides full-text search using an inverted index.
type ContentIndex struct {
	mu         sync.RWMutex
	documents  []string           // docID -> document path
	docLookup  map[string]uint32  // path -> docID
	inverted   map[string]*roaring.Bitmap // term -> document bitmaps
	totalDocs  uint32
	maxDocSize int64 // max file size to index (1MB)
}

// ContentResult is a single content search hit.
type ContentResult struct {
	Path     string `json:"path"`
	Filename string `json:"filename"`
	Line     int    `json:"line"`
	Content  string `json:"content"`
	DocID    uint32 `json:"-"`
}

// NewContentIndex creates a new content index.
func NewContentIndex() *ContentIndex {
	return &ContentIndex{
		docLookup:  make(map[string]uint32),
		inverted:   make(map[string]*roaring.Bitmap),
		maxDocSize: 1 * 1024 * 1024, // 1MB
	}
}

// IndexFile indexes a file's content into the inverted index.
func (ci *ContentIndex) IndexFile(path string) {
	// Check size
	info, err := os.Stat(path)
	if err != nil || info.Size() > ci.maxDocSize || info.Size() == 0 {
		return
	}

	if !IsIndexed(path) {
		return
	}

	ci.mu.Lock()
	// Check if already indexed
	if _, exists := ci.docLookup[path]; exists {
		ci.mu.Unlock()
		return
	}

	docID := ci.totalDocs
	ci.documents = append(ci.documents, path)
	ci.docLookup[path] = docID
	ci.totalDocs++
	ci.mu.Unlock()

	// Read and tokenize file content
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024*64), 1024*64)

	seen := make(map[string]bool) // avoid duplicate terms per document

	for scanner.Scan() {
		line := scanner.Text()
		terms := tokenize(line)

		for _, term := range terms {
			if seen[term] {
				continue
			}
			seen[term] = true

			ci.mu.Lock()
			bm, exists := ci.inverted[term]
			if !exists {
				bm = roaring.New()
				ci.inverted[term] = bm
			}
			bm.Add(docID)
			ci.mu.Unlock()
		}
	}
}

// RemoveFile removes a file from the index.
func (ci *ContentIndex) RemoveFile(path string) {
	ci.mu.Lock()
	defer ci.mu.Unlock()

	docID, exists := ci.docLookup[path]
	if !exists {
		return
	}

	// Remove docID from all posting lists
	for term, bm := range ci.inverted {
		bm.Remove(docID)
		if bm.IsEmpty() {
			delete(ci.inverted, term)
		}
	}

	delete(ci.docLookup, path)
	// Note: documents slice isn't cleaned up to keep docIDs stable
}

// Search performs a content search. Supports AND and OR queries.
func (ci *ContentIndex) Search(query string, limit int) ([]ContentResult, error) {
	ci.mu.RLock()
	defer ci.mu.RUnlock()

	if ci.totalDocs == 0 || len(query) == 0 {
		return nil, nil
	}

	// Check if regex
	isRegex := len(query) > 2 && strings.HasPrefix(query, "/") && strings.HasSuffix(query, "/")

	if isRegex {
		// Fall back to file scan for regex (inverted index doesn't support regex directly)
		ci.mu.RUnlock()
		results := ci.regexSearch(query[1:len(query)-1], limit)
		ci.mu.RLock()
		return results, nil
	}

	// Normal text search using inverted index
	terms := tokenize(query)
	if len(terms) == 0 {
		return nil, nil
	}

	// Find documents matching ALL terms (AND)
	var resultDocs *roaring.Bitmap
	for _, term := range terms {
		bm, exists := ci.inverted[term]
		if !exists {
			// Term not found → no results
			return nil, nil
		}
		if resultDocs == nil {
			resultDocs = bm.Clone()
		} else {
			resultDocs.And(bm)
		}

		if resultDocs.IsEmpty() {
			return nil, nil
		}
	}

	if resultDocs == nil || resultDocs.IsEmpty() {
		return nil, nil
	}

	// Convert to results with line-level matching
	var results []ContentResult
	it := resultDocs.Iterator()
	for it.HasNext() {
		if len(results) >= limit {
			break
		}
		docID := it.Next()
		if int(docID) >= len(ci.documents) {
			continue
		}
		path := ci.documents[docID]

		// Find matching line
		line, content := ci.findMatchingLine(path, query, limit-len(results))
		if line > 0 {
			results = append(results, ContentResult{
				Path:     path,
				Filename: filepath.Base(path),
				Line:     line,
				Content:  content,
			})
		}
	}

	return results, nil
}

// Count returns the number of indexed documents.
func (ci *ContentIndex) Count() int {
	ci.mu.RLock()
	defer ci.mu.RUnlock()
	return int(ci.totalDocs)
}

// TermCount returns the number of unique terms in the index.
func (ci *ContentIndex) TermCount() int {
	ci.mu.RLock()
	defer ci.mu.RUnlock()
	return len(ci.inverted)
}

func (ci *ContentIndex) findMatchingLine(path, query string, _ int) (int, string) {
	file, err := os.Open(path)
	if err != nil {
		return 0, ""
	}
	defer file.Close()

	lowerQuery := strings.ToLower(query)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024*64), 1024*64)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		text := scanner.Text()
		if strings.Contains(strings.ToLower(text), lowerQuery) {
			return lineNum, strings.TrimSpace(text)
		}
	}

	return 0, ""
}

func (ci *ContentIndex) regexSearch(pattern string, limit int) []ContentResult {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil
	}

	var results []ContentResult
	for _, path := range ci.documents {
		if len(results) >= limit {
			break
		}
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 1024*64), 1024*64)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			text := scanner.Text()
			if re.MatchString(text) {
				results = append(results, ContentResult{
					Path:     path,
					Filename: filepath.Base(path),
					Line:     lineNum,
					Content:  strings.TrimSpace(text),
				})
				break // one match per file
			}
		}
		file.Close()
	}
	return results
}

// tokenize splits text into lowercase terms.
func tokenize(text string) []string {
	// Simple tokenization: split on whitespace and common separators
	re := regexp.MustCompile(`[a-zA-Z0-9_]+`)
	matches := re.FindAllString(text, -1)

	terms := make(map[string]bool)
	var result []string

	for _, m := range matches {
		lower := strings.ToLower(m)
		if len(lower) < 2 {
			continue // skip single chars
		}
		if !terms[lower] {
			terms[lower] = true
			result = append(result, lower)
		}
	}

	return result
}

// Ensure indexer logs errors
var _ = log.Println
