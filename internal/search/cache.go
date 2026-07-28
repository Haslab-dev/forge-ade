package search

import (
	"sync"
	"time"
)

// CacheEntry caches a search query result.
type CacheEntry struct {
	Results   []RankedResult
	CreatedAt time.Time
	Query     string
}

// SearchCache caches recent search results for instant re-queries.
type SearchCache struct {
	mu       sync.RWMutex
	entries  map[string]*CacheEntry
	maxAge   time.Duration
	maxSize  int
}

// NewSearchCache creates a new search cache.
func NewSearchCache() *SearchCache {
	return &SearchCache{
		entries: make(map[string]*CacheEntry),
		maxAge:  30 * time.Second,
		maxSize: 100,
	}
}

// Get returns cached results if available.
func (sc *SearchCache) Get(query string) ([]RankedResult, bool) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	entry, exists := sc.entries[query]
	if !exists {
		return nil, false
	}

	if time.Since(entry.CreatedAt) > sc.maxAge {
		return nil, false
	}

	return entry.Results, true
}

// Set stores results for a query.
func (sc *SearchCache) Set(query string, results []RankedResult) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	// Evict oldest if at capacity
	if len(sc.entries) >= sc.maxSize {
		var oldestKey string
		var oldestTime time.Time
		for k, v := range sc.entries {
			if oldestKey == "" || v.CreatedAt.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.CreatedAt
			}
		}
		delete(sc.entries, oldestKey)
	}

	sc.entries[query] = &CacheEntry{
		Results:   results,
		CreatedAt: time.Now(),
		Query:     query,
	}
}

// Invalidate clears the cache.
func (sc *SearchCache) Invalidate() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.entries = make(map[string]*CacheEntry)
}

// Cleanup removes expired entries.
func (sc *SearchCache) Cleanup() {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	for k, v := range sc.entries {
		if time.Since(v.CreatedAt) > sc.maxAge {
			delete(sc.entries, k)
		}
	}
}
