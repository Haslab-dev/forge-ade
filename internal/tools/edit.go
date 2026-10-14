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
			path := resolveToolPath(ctx, argPath(args))
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

			effectiveNewStr := healIndentation(oldStr, match.text, newStr)
			updated := applyEdit(content, match.text, effectiveNewStr, replaceAll)
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

// SingleEdit represents one find-and-replace operation within a multiedit call.
type SingleEdit struct {
	OldString  string `json:"old_string"`
	NewString  string `json:"new_string"`
	ReplaceAll bool   `json:"replace_all"`
}

func multiEditTool() ToolSpec {
	return ToolSpec{
		Name: "multiedit",
		Description: "Apply multiple find-and-replace edits atomically to a single file. " +
			"All edits are verified in sequence before any change is written to disk. " +
			"If any edit fails to match, NONE of the edits are applied (transactional rollback). " +
			"Prefer multiedit over multiple sequential edit calls when changing multiple sections of a file.",
		Cost: "medium",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string", "description": "File to edit"},
				"edits": map[string]any{
					"type":        "array",
					"description": "List of find-and-replace edit blocks to apply in order",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"old_string":  map[string]any{"type": "string", "description": "Exact text to find"},
							"new_string":  map[string]any{"type": "string", "description": "Replacement text"},
							"replace_all": map[string]any{"type": "boolean", "description": "Replace all occurrences (default false)"},
						},
						"required": []string{"old_string", "new_string"},
					},
				},
			},
			"required": []string{"path", "edits"},
		},
		Handler: handleMultiEdit,
	}
}

func handleMultiEdit(ctx context.Context, args map[string]any) (any, error) {
	path := resolveToolPath(ctx, argPath(args))
	if path == "" {
		return nil, fmt.Errorf("path is required")
	}

	rawEdits, exists := args["edits"]
	if !exists {
		return nil, fmt.Errorf("edits parameter is required")
	}

	edits, err := parseSingleEdits(rawEdits)
	if err != nil {
		return nil, err
	}
	if len(edits) == 0 {
		return nil, fmt.Errorf("at least one edit operation is required")
	}

	tracker := FileStateFrom(ctx)
	if tracker != nil {
		if err := tracker.CheckFresh(path); err != nil {
			return nil, err
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	hadCRLF := strings.Contains(string(data), "\r\n")
	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	workingContent := content
	totalReplacements := 0

	for i, edit := range edits {
		oldStr := strings.ReplaceAll(edit.OldString, "\r\n", "\n")
		newStr := strings.ReplaceAll(edit.NewString, "\r\n", "\n")

		match := findEditMatch(workingContent, oldStr, edit.ReplaceAll)
		switch match.kind {
		case editNotFound:
			return nil, fmt.Errorf("transaction aborted (no changes written): edit #%d failed: string to replace not found in working content.\nString: %s\n%s", i+1, truncateForError(oldStr), editUsageHint)
		case editAmbiguous:
			return nil, fmt.Errorf("transaction aborted (no changes written): edit #%d failed: found %d matches of old_string, but replace_all is false. Please provide more context to uniquely identify the instance.\nString: %s", i+1, match.count, truncateForError(oldStr))
		}

		effectiveNewStr := healIndentation(oldStr, match.text, newStr)
		workingContent = applyEdit(workingContent, match.text, effectiveNewStr, edit.ReplaceAll)
		totalReplacements += match.count
	}

	finalContent := workingContent
	if hadCRLF {
		finalContent = strings.ReplaceAll(finalContent, "\n", "\r\n")
	}

	if err := os.WriteFile(path, []byte(finalContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to write %s: %w", path, err)
	}
	tracker.MarkEdited(path)

	res := map[string]any{
		"path":               path,
		"status":             "edited",
		"edits_applied":      len(edits),
		"total_replacements": totalReplacements,
	}

	diffContent := content
	if hadCRLF {
		diffContent = strings.ReplaceAll(content, "\n", "\r\n")
	}
	if d := unifiedDiff(diffContent, finalContent); d != "" {
		add, del := countDiffLines(d)
		res["diff"] = d
		res["additions"] = add
		res["deletions"] = del
	}
	return toolResult(res), nil
}

func parseSingleEdits(raw any) ([]SingleEdit, error) {
	arr, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("edits must be an array of edit objects")
	}
	var edits []SingleEdit
	for i, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("edit #%d must be an object with old_string and new_string", i+1)
		}
		oldStr := argString(m, "old_string")
		if oldStr == "" {
			oldStr = argString(m, "old")
		}
		newStr := argString(m, "new_string")
		if newStr == "" {
			if _, exists := m["new_string"]; !exists {
				newStr = argString(m, "new")
			}
		}
		if oldStr == "" {
			return nil, fmt.Errorf("edit #%d: old_string is required", i+1)
		}
		if oldStr == newStr {
			return nil, fmt.Errorf("edit #%d: old_string and new_string are exactly the same", i+1)
		}
		edits = append(edits, SingleEdit{
			OldString:  oldStr,
			NewString:  newStr,
			ReplaceAll: coerceReplaceAll(m),
		})
	}
	return edits, nil
}

// healIndentation adjusts the replacement text indentation if the model's
// search text had consistent indentation drift from the actual matched text in the file.
func healIndentation(search, matched, replacement string) string {
	if search == "" || matched == "" || replacement == "" {
		return replacement
	}
	sLines := strings.Split(search, "\n")
	mLines := strings.Split(matched, "\n")
	if len(sLines) != len(mLines) || len(sLines) == 0 {
		return replacement
	}

	var sIndent, mIndent string
	for i := 0; i < len(sLines); i++ {
		st := strings.TrimSpace(sLines[i])
		mt := strings.TrimSpace(mLines[i])
		if st != "" && mt != "" {
			sIndent = leadingWhitespace(sLines[i])
			mIndent = leadingWhitespace(mLines[i])
			break
		}
	}
	if sIndent == mIndent {
		return replacement
	}

	rLines := strings.Split(replacement, "\n")
	for i, rLine := range rLines {
		if strings.HasPrefix(rLine, sIndent) {
			rLines[i] = mIndent + strings.TrimPrefix(rLine, sIndent)
		}
	}
	return strings.Join(rLines, "\n")
}

func leadingWhitespace(s string) string {
	for i, c := range s {
		if c != ' ' && c != '\t' {
			return s[:i]
		}
	}
	return s
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
