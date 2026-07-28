package search

import (
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/hasdev/forge-ade/internal/events"
)

// SearchManager ties all indexes together.
type SearchManager struct {
	bus       *events.Bus
	filename  *FilenameIndex
	content   *ContentIndex
	ranking   *RankingEngine
	cache     *SearchCache
	dirs      []string
	mu        sync.RWMutex
	workerCh  chan string
	workerWg  sync.WaitGroup
	workerNum int
	running   bool
	stopCh    chan struct{}
}

// NewSearchManager creates a new search manager.
func NewSearchManager(bus *events.Bus) *SearchManager {
	sm := &SearchManager{
		bus:       bus,
		filename:  NewFilenameIndex(),
		content:   NewContentIndex(),
		ranking:   NewRankingEngine(),
		cache:     NewSearchCache(),
		workerCh:  make(chan string, 1000),
		workerNum: 16,
		stopCh:    make(chan struct{}),
	}
	return sm
}

// SetDirectories sets workspace folders for indexing.
func (sm *SearchManager) SetDirectories(dirs []string) {
	sm.mu.Lock()
	sm.dirs = dirs
	sm.ranking.SetWorkspaceDirs(dirs)
	sm.mu.Unlock()
}

// Start begins background indexing with a worker pool.
func (sm *SearchManager) Start() {
	sm.mu.Lock()
	if sm.running {
		sm.mu.Unlock()
		return
	}
	sm.running = true
	sm.mu.Unlock()

	// Start worker pool
	for i := 0; i < sm.workerNum; i++ {
		sm.workerWg.Add(1)
		go sm.worker()
	}

	// Start periodic cache cleanup
	go sm.cacheCleanup()

	// Initial index build
	go sm.buildInitialIndex()
}

// Stop stops the search manager.
func (sm *SearchManager) Stop() {
	sm.mu.Lock()
	if !sm.running {
		sm.mu.Unlock()
		return
	}
	sm.running = false
	close(sm.stopCh)
	close(sm.workerCh)
	sm.mu.Unlock()

	sm.workerWg.Wait()
}

// IndexFile queues a file for indexing.
func (sm *SearchManager) IndexFile(path string) {
	sm.mu.RLock()
	running := sm.running
	sm.mu.RUnlock()

	if !running {
		return
	}

	// Non-blocking send
	select {
	case sm.workerCh <- path:
	default:
		// Channel full, skip
	}
}

// RemoveFile removes a file from all indexes.
func (sm *SearchManager) RemoveFile(path string) {
	sm.filename.Remove(path)
	sm.content.RemoveFile(path)
}

// SearchFilename performs instant filename search.
func (sm *SearchManager) SearchFilename(query string, limit int) []RankedResult {
	if query == "" {
		return nil
	}

	// Check cache first
	if cached, ok := sm.cache.Get("fn:" + query); ok {
		return cached
	}

	// Try fuzzy search first (VS Code style)
	entries := sm.filename.FuzzySearch(query, limit*2)
	if len(entries) == 0 {
		// Fall back to prefix search
		entries = sm.filename.Search(query, limit*2)
	}

	if len(entries) == 0 {
		return nil
	}

	results := sm.ranking.RankFilename(query, entries, limit)

	// Cache result
	if len(results) > 0 {
		sm.cache.Set("fn:"+query, results)
	}

	return results
}

// SearchContent performs full-text search.
func (sm *SearchManager) SearchContent(query string, limit int) ([]RankedResult, error) {
	if query == "" {
		return nil, nil
	}

	// Check cache
	if cached, ok := sm.cache.Get("ct:" + query); ok {
		return cached, nil
	}

	results, err := sm.content.Search(query, limit*2)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}

	ranked := sm.ranking.RankContent(query, results, limit)

	// Cache
	if len(ranked) > 0 {
		sm.cache.Set("ct:"+query, ranked)
	}

	return ranked, nil
}

// MarkOpened records a file as recently opened for ranking.
func (sm *SearchManager) MarkOpened(path string) {
	sm.ranking.MarkOpened(path)
}

// MarkGitModified records git changes for ranking.
func (sm *SearchManager) MarkGitModified(paths []string) {
	sm.ranking.MarkGitModified(paths)
}

// Stats returns index statistics.
func (sm *SearchManager) Stats() map[string]int {
	return map[string]int{
		"files":       sm.filename.Count(),
		"documents":   sm.content.Count(),
		"terms":       sm.content.TermCount(),
		"cache_entries": len(sm.cache.entries),
	}
}

func (sm *SearchManager) worker() {
	defer sm.workerWg.Done()

	for path := range sm.workerCh {
		sm.indexFile(path)
	}
}

func (sm *SearchManager) indexFile(path string) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}

	if info.IsDir() {
		return
	}

	if !IsIndexed(path) {
		return
	}

	sm.filename.Insert(path)
	sm.content.IndexFile(path)
}

func (sm *SearchManager) buildInitialIndex() {
	sm.mu.RLock()
	dirs := make([]string, len(sm.dirs))
	copy(dirs, sm.dirs)
	sm.mu.RUnlock()

	log.Printf("search: building initial index for %d directories", len(dirs))

	var fileCount int
	for _, dir := range dirs {
		_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				if IsDirSkipped(info.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if !IsIndexed(path) {
				return nil
			}
			// Insert filename directly (instant, no worker delay)
			sm.filename.Insert(path)
			// Content indexing via worker (async)
			sm.IndexFile(path)
			fileCount++
			return nil
		})
	}

	log.Printf("search: index built — %d files indexed (content building async)", fileCount)
}

func (sm *SearchManager) cacheCleanup() {
	for {
		select {
		case <-sm.stopCh:
			return
		default:
			sm.cache.Cleanup()
			time.Sleep(30 * time.Second)
		}
	}
}
