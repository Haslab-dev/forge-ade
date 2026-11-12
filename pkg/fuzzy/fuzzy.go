package fuzzy

import (
	"strings"
	"unicode"
)

// VS Code-style fuzzy matcher, ported from ForgeADE's frontend/src/lib/fuzzy.ts.

// FuzzyMatch is a scored match; Positions are the indices of the target the
// query characters matched, ascending.
type FuzzyMatch struct {
	Score     float64
	Positions []int
}

var fuzzySeparators = map[byte]bool{
	'/': true, '\\': true, '_': true, '-': true, '.': true,
	' ': true, '@': true, ':': true, '$': true, '#': true,
}

// Match against the basename wins ties outright — this is what makes quick
// open feel like VS Code when many files share directory prefixes.
const (
	fuzzyBasenameBonus     = 60
	fuzzyConsecutiveBonus  = 8
	fuzzyBoundaryBonus     = 12
	fuzzyLeadingGapPenalty = 1.2
	fuzzyLengthPenalty     = 0.05
	fuzzyBaseScore         = 16
)

func isCamelHump(target string, idx int) bool {
	if idx == 0 {
		return false
	}
	prev := rune(target[idx-1])
	cur := rune(target[idx])
	return unicode.ToLower(prev) == prev && unicode.ToLower(cur) != cur
}

// fuzzyMatchIn greedily matches query against target in order.
func fuzzyMatchIn(query, target string) *FuzzyMatch {
	q := []rune(query)
	t := []rune(target)
	positions := make([]int, 0, len(q))
	score := float64(fuzzyBaseScore)
	prevIdx := -2
	searchFrom := 0

	for _, qc := range q {
		qc = unicode.ToLower(qc)
		idx := -1
		for i := searchFrom; i < len(t); i++ {
			if unicode.ToLower(t[i]) == qc {
				idx = i
				break
			}
		}
		if idx == -1 {
			return nil
		}

		if idx == prevIdx+1 {
			score += fuzzyConsecutiveBonus
		} else if prevIdx >= 0 {
			gap := idx - prevIdx - 1
			if gap > 10 {
				gap = 10
			}
			score -= float64(gap) * fuzzyLeadingGapPenalty
		}
		if idx == 0 || (idx > 0 && fuzzySeparators[byte(t[idx-1])]) || isCamelHump(target, idx) {
			score += fuzzyBoundaryBonus
		}

		positions = append(positions, idx)
		prevIdx = idx
		searchFrom = idx + 1
	}

	return &FuzzyMatch{Score: score - float64(len(t))*fuzzyLengthPenalty, Positions: positions}
}

// ScoreFilePath fuzzy-matches query against a file path, basename-aware.
// Nil when the query is not a subsequence of the path. An empty query
// matches everything with a neutral score.
func ScoreFilePath(query, filePath string) *FuzzyMatch {
	q := strings.TrimSpace(query)
	if q == "" {
		return &FuzzyMatch{}
	}
	sep := max(strings.LastIndex(filePath, "/"), strings.LastIndex(filePath, "\\"))
	if sep >= 0 {
		base := fuzzyMatchIn(q, filePath[sep+1:])
		if base != nil {
			for i, p := range base.Positions {
				base.Positions[i] = p + sep + 1
			}
			base.Score += fuzzyBasenameBonus
			return base
		}
	}
	return fuzzyMatchIn(q, filePath)
}

// ScoreText is a plain fuzzy match for labels (commands).
func ScoreText(query, text string) *FuzzyMatch {
	q := strings.TrimSpace(query)
	if q == "" {
		return &FuzzyMatch{}
	}
	return fuzzyMatchIn(q, text)
}
