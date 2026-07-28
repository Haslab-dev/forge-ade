package search

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// RankedResult is a search result with a score.
type RankedResult struct {
	Path     string  `json:"path"`
	Filename string  `json:"filename"`
	Score    float64 `json:"score"`
	Line     int     `json:"line,omitempty"`
	Content  string  `json:"content,omitempty"`
}

// RankingEngine scores search results based on multiple signals.
type RankingEngine struct {
	mu           sync.RWMutex
	openTimestamps map[string]time.Time // path -> last opened time
	gitModified    map[string]bool      // path -> modified in git
	workspaceDirs  []string             // current workspace roots
}

// NewRankingEngine creates a new ranking engine.
func NewRankingEngine() *RankingEngine {
	return &RankingEngine{
		openTimestamps: make(map[string]time.Time),
		gitModified:    make(map[string]bool),
	}
}

// SetWorkspaceDirs sets the current workspace directories for scoring.
func (re *RankingEngine) SetWorkspaceDirs(dirs []string) {
	re.mu.Lock()
	defer re.mu.Unlock()
	re.workspaceDirs = dirs
}

// MarkOpened records that a file was recently opened.
func (re *RankingEngine) MarkOpened(path string) {
	re.mu.Lock()
	defer re.mu.Unlock()
	re.openTimestamps[path] = time.Now()
}

// MarkGitModified marks files that have git changes.
func (re *RankingEngine) MarkGitModified(paths []string) {
	re.mu.Lock()
	defer re.mu.Unlock()
	for _, p := range paths {
		re.gitModified[p] = true
	}
}

// RankFilename scores filename search results and returns sorted results.
func (re *RankingEngine) RankFilename(query string, entries []*FileEntry, limit int) []RankedResult {
	if len(entries) == 0 {
		return nil
	}

	re.mu.RLock()
	defer re.mu.RUnlock()

	lowerQuery := strings.ToLower(query)
	type scored struct {
		result RankedResult
		score  int
	}

	var scoredResults []scored

	for _, entry := range entries {
		score := fuzzyScore(query, lowerQuery, entry.Name, entry.LowerName)
		if score <= 0 {
			continue
		}

		// Apply ranking signals
		score = re.applyRanking(score, entry.Path)

		scoredResults = append(scoredResults, scored{
			result: RankedResult{
				Path:     entry.Path,
				Filename: entry.Name,
				Score:    float64(score),
			},
			score: score,
		})
	}

	// Sort by score descending
	sort.Slice(scoredResults, func(i, j int) bool {
		if scoredResults[i].score != scoredResults[j].score {
			return scoredResults[i].score > scoredResults[j].score
		}
		return scoredResults[i].result.Filename < scoredResults[j].result.Filename
	})

	if len(scoredResults) > limit {
		scoredResults = scoredResults[:limit]
	}

	results := make([]RankedResult, len(scoredResults))
	for i, sr := range scoredResults {
		results[i] = sr.result
	}

	return results
}

// RankContent scores content search results.
func (re *RankingEngine) RankContent(query string, results []ContentResult, limit int) []RankedResult {
	if len(results) == 0 {
		return nil
	}

	re.mu.RLock()
	defer re.mu.RUnlock()

	lowerQuery := strings.ToLower(query)
	type scored struct {
		result RankedResult
		score  float64
	}

	var scoredResults []scored

	for _, r := range results {
		score := 100.0

		// Exact match bonus
		if strings.Contains(strings.ToLower(r.Content), lowerQuery) {
			score += 20
		}

		// Title/camelCase match bonus
		if strings.Contains(strings.ToLower(r.Filename), lowerQuery) {
			score += 15
		}

		// Line position bonus (matches in first few lines = likely important)
		if r.Line <= 5 {
			score += 10
		}

		// Apply ranking signals (convert to int for applyRanking, convert back)
		intScore := re.applyRanking(int(score*10), r.Path)
		score = float64(intScore) / 10.0

		scoredResults = append(scoredResults, scored{
			result: RankedResult{
				Path:     r.Path,
				Filename: r.Filename,
				Score:    score,
				Line:     r.Line,
				Content:  r.Content,
			},
			score: score,
		})
	}

	sort.Slice(scoredResults, func(i, j int) bool {
		if scoredResults[i].score != scoredResults[j].score {
			return scoredResults[i].score > scoredResults[j].score
		}
		return scoredResults[i].result.Filename < scoredResults[j].result.Filename
	})

	if len(scoredResults) > limit {
		scoredResults = scoredResults[:limit]
	}

	results2 := make([]RankedResult, len(scoredResults))
	for i, sr := range scoredResults {
		results2[i] = sr.result
	}

	return results2
}

func (re *RankingEngine) applyRanking(baseScore int, path string) int {
	score := baseScore

	// Recently opened files get a boost
	if ts, ok := re.openTimestamps[path]; ok {
		hoursAgo := time.Since(ts).Hours()
		if hoursAgo < 1 {
			score += 40 // opened within the last hour
		} else if hoursAgo < 24 {
			score += 20 // opened today
		}
	}

	// Git modified files get a boost
	if re.gitModified[path] {
		score += 20
	}

	// Files in workspace roots get a boost
	if len(re.workspaceDirs) > 0 {
		for _, dir := range re.workspaceDirs {
			if strings.HasPrefix(path, dir) {
				// Closer to root = higher score
				rel, err := filepath.Rel(dir, path)
				if err == nil {
					depth := len(strings.Split(rel, string(os.PathSeparator)))
					if depth <= 2 {
						score += 15
					} else if depth <= 4 {
						score += 5
					}
				}
				break
			}
		}
	}

	return score
}
