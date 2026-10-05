package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ---------------------------------------------------------------------------
// edit — exact string replacement in a file (the harness "Edit" tool).
//
// Semantics cloned from the ZCode reference harness:
//   - old_string must match the file exactly (including indentation) and be
//     unique unless replace_all is set.
//   - A fallback match ladder repairs the near-misses models actually produce:
//     curly-quote normalization, Read-style line-number prefixes, and
//     per-line whitespace drift.
//   - Replaces the FIRST occurrence unless replace_all; replacing "" deletes
//     the matched text (consuming one trailing newline).
//   - The result text returned to the model confirms success — the file on
//     disk is already updated when the model sees the result.
// ---------------------------------------------------------------------------

const editUsageHint = "Your edit old_string/new_string will be rejected if they do not match the file exactly. Use the Read tool first, and match whitespace exactly."

type editMatchKind int

const (
	editNotFound editMatchKind = iota
	editAmbiguous
	editFound
)

type editMatch struct {
	kind editMatchKind
	// text is the actual file slice to replace (may differ from old_string
	// when a fuzzy strategy matched). count is the number of occurrences.
	text  string
	count int
}

// normalizeCurlyQuotes maps typographic quotes to their ASCII equivalents.
func normalizeCurlyQuotes(s string) string {
	r := strings.NewReplacer("‘", "'", "’", "'", "“", `"`, "”", `"`, "–", "-", "—", "-")
	return r.Replace(s)
}

// stripLineNumberPrefixes removes Read-style prefixes ("12: ", "12\t", "12: ")
// from every line of s, so an old_string copy-pasted from numbered Read
// output still matches the raw file text.
func stripLineNumberPrefixes(s string) string {
	lines := strings.Split(s, "\n")
	changed := false
	for i, l := range lines {
		if idx := strings.IndexAny(l, ":\t"); idx > 0 && isAllDigits(l[:idx]) {
			rest := l[idx+1:]
			rest = strings.TrimPrefix(rest, " ")
			lines[i] = rest
			changed = true
		}
	}
	if !changed {
		return s
	}
	return strings.Join(lines, "\n")
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// lineOffsets returns the byte offset at which each line of content starts.
func lineOffsets(content string) []int {
	offsets := []int{0}
	for i := 0; i < len(content); i++ {
		if content[i] == '\n' {
			offsets = append(offsets, i+1)
		}
	}
	return offsets
}

// lineTrimmedMatch implements the whitespace-flexible block match: every line
// of the search block must equal the corresponding file line after
// TrimSpace. A trailing empty search line is dropped (models often add one).
func lineTrimmedMatch(content, search string) []string {
	searchLines := strings.Split(search, "\n")
	for len(searchLines) > 0 && strings.TrimSpace(searchLines[len(searchLines)-1]) == "" {
		searchLines = searchLines[:len(searchLines)-1]
	}
	if len(searchLines) == 0 {
		return nil
	}

	var matches []string
	contentLines := strings.Split(content, "\n")
	offsets := lineOffsets(content)
	for start := 0; start+len(searchLines) <= len(contentLines); start++ {
		ok := true
		for j, sl := range searchLines {
			if strings.TrimSpace(contentLines[start+j]) != strings.TrimSpace(sl) {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		// Map the matched line block back to the actual file slice.
		from := offsets[start]
		endLine := start + len(searchLines) - 1
		var to int
		if endLine+1 < len(offsets) {
			to = offsets[endLine+1] - 1 // exclude the trailing newline
		} else {
			to = len(content)
		}
		matches = append(matches, content[from:to])
	}
	return matches
}

// findEditMatch runs the match ladder: exact → quote-normalized →
// line-number-prefix-stripped → line-trimmed. The first strategy producing at
// least one candidate wins; multiple candidates mean ambiguous unless
// replace_all is set (in which case any identical-text candidates collapse).
func findEditMatch(content, search string, replaceAll bool) editMatch {
	if search == "" {
		return editMatch{kind: editFound, text: "", count: 1}
	}

	// 1. Exact match.
	if n := strings.Count(content, search); n > 0 {
		if n > 1 && !replaceAll {
			return editMatch{kind: editAmbiguous, count: n}
		}
		return editMatch{kind: editFound, text: search, count: n}
	}

	// 2. Curly-quote / dash normalization (search in a normalized copy, but
	// replace the actual file slice so the file keeps its original glyphs).
	normContent := normalizeCurlyQuotes(content)
	normSearch := normalizeCurlyQuotes(search)
	if normSearch != search {
		if n := strings.Count(normContent, normSearch); n > 0 {
			if n > 1 && !replaceAll {
				return editMatch{kind: editAmbiguous, count: n}
			}
			idx := strings.Index(normContent, normSearch)
			return editMatch{kind: editFound, text: content[idx : idx+len(normSearch)], count: n}
		}
	}

	// 3. Read-style line-number prefixes.
	stripped := stripLineNumberPrefixes(search)
	if stripped != search {
		if n := strings.Count(content, stripped); n > 0 {
			if n > 1 && !replaceAll {
				return editMatch{kind: editAmbiguous, count: n}
			}
			return editMatch{kind: editFound, text: stripped, count: n}
		}
	}

	// 4. Per-line whitespace drift (block match ignoring leading/trailing
	// whitespace per line). Skip when replace_all: slices may differ per
	// occurrence, which would rewrite inconsistent whitespace.
	if cands := lineTrimmedMatch(content, search); len(cands) > 0 {
		uniq := make([]string, 0, len(cands))
		seen := make(map[string]bool)
		for _, c := range cands {
			if !seen[c] {
				seen[c] = true
				uniq = append(uniq, c)
			}
		}
		if len(uniq) > 1 || (len(cands) > 1 && !replaceAll) {
			return editMatch{kind: editAmbiguous, count: len(cands)}
		}
		return editMatch{kind: editFound, text: uniq[0], count: len(cands)}
	}

	return editMatch{kind: editNotFound}
}

// applyEdit replaces the matched text. An empty replacement also consumes one
// trailing newline so deleting a whole line doesn't leave a blank one behind.
func applyEdit(content, matched, replacement string, replaceAll bool) string {
	if replacement != "" {
		if replaceAll {
			return strings.ReplaceAll(content, matched, replacement)
		}
		return strings.Replace(content, matched, replacement, 1)
	}
	search := matched
	if !strings.HasSuffix(matched, "\n") {
		if n := strings.Count(content, matched+"\n"); n > 0 {
			search = matched + "\n"
		}
	}
	if replaceAll {
		return strings.ReplaceAll(content, search, replacement)
	}
	return strings.Replace(content, search, replacement, 1)
}

// coerceReplaceAll accepts booleans plus the string forms models emit
// ("true", "1", "yes", "on").
func coerceReplaceAll(args map[string]any) bool {
	switch v := args["replace_all"].(type) {
	case bool:
		return v
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "1", "yes", "y", "on":
			return true
		}
	case float64:
		return v != 0
	}
	return false
}

func editTool() ToolSpec {
	return ToolSpec{
		Name: "edit",
		Description: "Performs exact string replacement in a file (sed-style patch, NOT a full rewrite). " +
			"- You must Read the file in this conversation before editing, or the call will fail.\n" +
			"- `old_string` must match the file exactly, including indentation, and be unique — the edit fails otherwise. Strip the Read line prefix (line number + tab) before matching.\n" +
			"- `replace_all: true` replaces every occurrence instead.\n" +
			"- Prefer edit over write for existing files: pass the smallest old_string that uniquely identifies the change.",
		Cost: "medium",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":        map[string]any{"type": "string", "description": "File to edit"},
				"old_string":  map[string]any{"type": "string", "description": "Exact text to find (must be unique unless replace_all)"},
				"new_string":  map[string]any{"type": "string", "description": "Replacement text (empty deletes the matched text)"},
				"replace_all": map[string]any{"type": "boolean", "description": "Replace every occurrence (default false)"},
			},
			"required": []string{"path", "old_string", "new_string"},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			path := resolveToolPath(ctx, argString(args, "path"))
			oldStr := argString(args, "old_string")
			if oldStr == "" {
				oldStr = argString(args, "old")
			}
			newStr := argString(args, "new_string")
			if newStr == "" {
				if _, exists := args["new_string"]; !exists {
					newStr = argString(args, "new")
				}
			}
			replaceAll := coerceReplaceAll(args)

			if path == "" {
				return nil, fmt.Errorf("path is required")
			}
			if oldStr == newStr {
				return nil, fmt.Errorf("No changes to make: old_string and new_string are exactly the same.")
			}

			tracker := FileStateFrom(ctx)
			_, statErr := os.Stat(path)
			fileExists := statErr == nil

			// Create mode: empty old_string on a missing file creates it.
			if !fileExists {
				if oldStr == "" {
					if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
						return nil, fmt.Errorf("failed to create directory: %w", err)
					}
					if err := os.WriteFile(path, []byte(newStr), 0644); err != nil {
						return nil, fmt.Errorf("failed to write file: %w", err)
					}
					tracker.MarkEdited(path)
					res := map[string]any{"path": path, "status": "created", "replacements": 1}
					if d := unifiedDiff("", newStr); d != "" {
						res["diff"] = d
						add := 0
						for _, l := range strings.Split(d, "\n") {
							if strings.HasPrefix(l, "+") && !strings.HasPrefix(l, "+++") {
								add++
							}
						}
						res["additions"] = add
					}
					return toolResult(res), nil
				}
				return nil, fmt.Errorf("File does not exist: %s", path)
			}

			// Read-before-edit + staleness gate (session-backed calls only).
			if tracker != nil {
				if err := tracker.CheckFresh(path); err != nil {
					return nil, err
				}
			}

			if oldStr == "" {
				return nil, fmt.Errorf("Cannot create new file - file already exists.")
			}

			data, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("failed to read %s: %w", path, err)
			}
			hadCRLF := strings.Contains(string(data), "\r\n")
			content := strings.ReplaceAll(string(data), "\r\n", "\n")
			oldStr = strings.ReplaceAll(oldStr, "\r\n", "\n")
			newStr = strings.ReplaceAll(newStr, "\r\n", "\n")

			match := findEditMatch(content, oldStr, replaceAll)
			switch match.kind {
			case editNotFound:
				return nil, fmt.Errorf("String to replace not found in file.\nString: %s\n%s", truncateForError(oldStr), editUsageHint)
			case editAmbiguous:
				return nil, fmt.Errorf("Found %d matches of the string to replace, but replace_all is false. To replace all occurrences, set replace_all to true. To replace only one occurrence, please provide more context to uniquely identify the instance.\nString: %s", match.count, truncateForError(oldStr))
			}

			updated := applyEdit(content, match.text, newStr, replaceAll)
			if hadCRLF {
				updated = strings.ReplaceAll(updated, "\n", "\r\n")
			}
			if err := os.WriteFile(path, []byte(updated), 0644); err != nil {
				return nil, fmt.Errorf("failed to write %s: %w", path, err)
			}
			tracker.MarkEdited(path)

			res := map[string]any{
				"path":         path,
				"status":       "edited",
				"replacements": match.count,
				"replace_all":  replaceAll,
			}
			diffContent := content
			if hadCRLF {
				diffContent = strings.ReplaceAll(content, "\n", "\r\n")
			}
			if d := unifiedDiff(diffContent, updated); d != "" {
				add, del := countDiffLines(d)
				res["diff"] = d
				res["additions"] = add
				res["deletions"] = del
			}
			return toolResult(res), nil
		},
	}
}

func truncateForError(s string) string {
	if len(s) <= 500 {
		return s
	}
	return s[:500] + "…"
}

func countDiffLines(d string) (add int, del int) {
	for _, l := range strings.Split(d, "\n") {
		switch {
		case strings.HasPrefix(l, "+") && !strings.HasPrefix(l, "+++"):
			add++
		case strings.HasPrefix(l, "-") && !strings.HasPrefix(l, "---"):
			del++
		}
	}
	return add, del
}
