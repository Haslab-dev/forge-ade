package main

import "github.com/hasdev/forge-ade/internal/search"

// ---------------------------------------------------------------------------
// Search API
// ---------------------------------------------------------------------------

// SearchFilename searches files by name (instant, trie-based).
func (a *App) SearchFilename(query string, limit int) ([]search.RankedResult, error) {
	results := a.searchMgr.SearchFilename(query, limit)
	if results == nil {
		return []search.RankedResult{}, nil
	}
	return results, nil
}

// SearchFilenameWithOptions searches files with options.
func (a *App) SearchFilenameWithOptions(opts search.SearchOptions) ([]search.RankedResult, error) {
	results := a.searchMgr.SearchFilenameWithOptions(opts)
	if results == nil {
		return []search.RankedResult{}, nil
	}
	return results, nil
}

// SearchContent searches file contents via ripgrep (on-demand).
func (a *App) SearchContent(query string, limit int) ([]search.RankedResult, error) {
	return a.searchMgr.SearchContent(query, limit)
}

// SearchContentWithOptions searches file contents with options.
func (a *App) SearchContentWithOptions(opts search.SearchOptions) ([]search.RankedResult, error) {
	return a.searchMgr.SearchContentWithOptions(opts)
}

// SearchReplaceAll replaces all occurrences of the query across matching files.
func (a *App) SearchReplaceAll(opts search.ReplaceOptions) (search.ReplaceResult, error) {
	return a.searchMgr.ReplaceAll(opts)
}
