package search

import (
	"bufio"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var symbolDeclRegex = regexp.MustCompile(`(?i)\b(func|function|def|fn|type|class|interface|struct|enum|trait|const|let|var)\s+([A-Za-z0-9_]+)`)

// SearchSymbolsWithOptions searches for code symbol definitions with options.
func (sm *SearchManager) SearchSymbolsWithOptions(opts SearchOptions) ([]RankedResult, error) {
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

	subPattern := opts.Query
	if !opts.UseRegex {
		subPattern = regexp.QuoteMeta(opts.Query)
	}
	if opts.MatchWholeWord {
		subPattern = `\b` + subPattern + `\b`
	}

	casePrefix := "(?i)"
	if opts.MatchCase {
		casePrefix = ""
	}

	pattern := casePrefix + `\b(func|function|def|fn|type|class|interface|struct|enum|trait)\b.*` + subPattern

	rgPath := findRipgrepPath()
	if rgPath != "" {
		results, err := sm.searchSymbolRipgrep(rgPath, pattern, dirs, opts.Limit)
		if err == nil && len(results) > 0 {
			return results, nil
		}
	}

	return sm.searchSymbolGo(opts, dirs)
}

func (sm *SearchManager) SearchSymbols(query string, limit int) ([]RankedResult, error) {
	return sm.SearchSymbolsWithOptions(SearchOptions{Query: query, Limit: limit})
}

func (sm *SearchManager) searchSymbolRipgrep(rgPath, pattern string, dirs []string, limit int) ([]RankedResult, error) {
	args := []string{
		"--regexp", pattern,
		"--json",
		"--line-number",
		"--max-count", "50",
		"--follow",
		"--trim",
		"--no-heading",
	}
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

		content := strings.TrimSpace(r.Data.Lines.Text)
		results = append(results, RankedResult{
			Path:     r.Data.Path.Text,
			Filename: filepath.Base(r.Data.Path.Text),
			Line:     r.Data.LineNumber,
			Content:  content,
			Score:    100,
		})
	}

	_ = cmd.Wait()
	return results, nil
}

func (sm *SearchManager) searchSymbolGo(opts SearchOptions, dirs []string) ([]RankedResult, error) {
	matcher := createMatcher(opts)
	allResults, err := sm.searchContentGo(opts, dirs)
	if err != nil {
		return nil, err
	}

	var filtered []RankedResult
	for _, r := range allResults {
		if symbolDeclRegex.MatchString(r.Content) && matcher(r.Content) {
			filtered = append(filtered, r)
			if len(filtered) >= opts.Limit {
				break
			}
		}
	}

	return filtered, nil
}
