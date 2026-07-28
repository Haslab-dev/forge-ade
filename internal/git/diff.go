package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// DiffLineType indicates what kind of change a line represents.
type DiffLineType string

const (
	DiffAdded    DiffLineType = "added"
	DiffModified DiffLineType = "modified"
	DiffDeleted  DiffLineType = "deleted"
	DiffContext  DiffLineType = "context"
)

// DiffLine represents a single line in a diff.
type DiffLine struct {
	Type    DiffLineType `json:"type"`
	Content string       `json:"content"`
	OldLine int          `json:"oldLine"`
	NewLine int          `json:"newLine"`
}

// DiffHunk represents a contiguous block of changed lines.
type DiffHunk struct {
	OldStart int        `json:"oldStart"`
	OldCount int        `json:"oldCount"`
	NewStart int        `json:"newStart"`
	NewCount int        `json:"newCount"`
	Header   string     `json:"header"`
	Lines    []DiffLine `json:"lines"`
}

// FileDiff represents the diff for a single file.
type FileDiff struct {
	OldPath string     `json:"oldPath"`
	NewPath string     `json:"newPath"`
	Hunks   []DiffHunk `json:"hunks"`
}

// DiffCache caches diffs for open files.
type DiffCache struct {
	mu    sync.RWMutex
	cache map[string]*FileDiff
}

var globalDiffCache = &DiffCache{cache: make(map[string]*FileDiff)}

// GetFileDiff computes or retrieves a cached diff for a file in the repo.
func (r *Repository) GetFileDiff(relPath string) (*FileDiff, error) {
	fullPath := filepath.Join(r.Path, relPath)

	globalDiffCache.mu.RLock()
	if cached, ok := globalDiffCache.cache[fullPath]; ok {
		globalDiffCache.mu.RUnlock()
		return cached, nil
	}
	globalDiffCache.mu.RUnlock()

	fd, err := r.computeFileDiff(relPath)
	if err != nil {
		return nil, err
	}

	globalDiffCache.mu.Lock()
	globalDiffCache.cache[fullPath] = fd
	globalDiffCache.mu.Unlock()

	return fd, nil
}

// ClearDiffCache clears all cached diffs.
func ClearDiffCache() {
	globalDiffCache.mu.Lock()
	globalDiffCache.cache = make(map[string]*FileDiff)
	globalDiffCache.mu.Unlock()
}

// InvalidateDiffCache removes cached diff for a specific file.
func InvalidateDiffCache(path string) {
	globalDiffCache.mu.Lock()
	delete(globalDiffCache.cache, path)
	globalDiffCache.mu.Unlock()
}

func (r *Repository) computeFileDiff(relPath string) (*FileDiff, error) {
	fullPath := filepath.Join(r.Path, relPath)

	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return r.computeDeletedFileDiff(relPath)
	}

	cmd := exec.Command("git", "diff", "--no-color", "-U999999", relPath)
	cmd.Dir = r.Path
	output, err := cmd.CombinedOutput()
	if err != nil {
		if len(output) == 0 {
			return &FileDiff{OldPath: relPath, NewPath: relPath}, nil
		}
		return nil, fmt.Errorf("git diff: %w", err)
	}

	if len(output) == 0 {
		cmdStaged := exec.Command("git", "diff", "--cached", "--no-color", "-U999999", relPath)
		cmdStaged.Dir = r.Path
		stagedOutput, _ := cmdStaged.CombinedOutput()
		if len(stagedOutput) == 0 {
			return &FileDiff{OldPath: relPath, NewPath: relPath}, nil
		}
		output = stagedOutput
	}

	return parseUnifiedDiff(string(output), relPath)
}

func (r *Repository) computeDeletedFileDiff(relPath string) (*FileDiff, error) {
	cmd := exec.Command("git", "show", "HEAD:"+relPath)
	cmd.Dir = r.Path
	oldContent, err := cmd.CombinedOutput()
	if err != nil {
		return &FileDiff{OldPath: relPath, NewPath: relPath}, nil
	}

	lines := strings.Split(string(oldContent), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	var hunks []DiffHunk
	var diffLines []DiffLine
	for i, line := range lines {
		diffLines = append(diffLines, DiffLine{
			Type:    DiffDeleted,
			Content: line,
			OldLine: i + 1,
			NewLine: -1,
		})
	}
	if len(diffLines) > 0 {
		hunks = append(hunks, DiffHunk{
			OldStart: 1,
			OldCount: len(lines),
			NewStart: 0,
			NewCount: 0,
			Header:   "@@ -1," + fmt.Sprintf("%d", len(lines)) + " +0,0 @@",
			Lines:    diffLines,
		})
	}

	return &FileDiff{
		OldPath: relPath,
		NewPath: relPath,
		Hunks:   hunks,
	}, nil
}

func parseUnifiedDiff(diffOutput, relPath string) (*FileDiff, error) {
	fd := &FileDiff{
		OldPath: relPath,
		NewPath: relPath,
	}

	lines := strings.Split(diffOutput, "\n")
	var currentHunk *DiffHunk

	for _, line := range lines {
		if strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ ") {
			continue
		}

		if strings.HasPrefix(line, "@@") {
			if currentHunk != nil {
				fd.Hunks = append(fd.Hunks, *currentHunk)
			}
			currentHunk = &DiffHunk{Lines: []DiffLine{}, Header: line}
			fmt.Sscanf(line, "@@ -%d,%d +%d,%d @@",
				&currentHunk.OldStart, &currentHunk.OldCount,
				&currentHunk.NewStart, &currentHunk.NewCount)
			if currentHunk.OldCount == 0 {
				currentHunk.OldCount = 1
			}
			if currentHunk.NewCount == 0 {
				currentHunk.NewCount = 1
			}
			continue
		}

		if currentHunk == nil {
			continue
		}

		dl := DiffLine{Content: line}
		oldLine := currentHunk.OldStart + len(currentHunk.Lines)
		newLine := currentHunk.NewStart + len(currentHunk.Lines)

		switch {
		case strings.HasPrefix(line, "+"):
			dl.Type = DiffAdded
			dl.OldLine = -1
			dl.NewLine = newLine
			dl.Content = strings.TrimPrefix(line, "+")
		case strings.HasPrefix(line, "-"):
			dl.Type = DiffDeleted
			dl.OldLine = oldLine
			dl.NewLine = -1
			dl.Content = strings.TrimPrefix(line, "-")
		default:
			dl.Type = DiffContext
			dl.OldLine = oldLine
			dl.NewLine = newLine
			dl.Content = strings.TrimPrefix(line, " ")
		}
		currentHunk.Lines = append(currentHunk.Lines, dl)
	}

	if currentHunk != nil {
		fd.Hunks = append(fd.Hunks, *currentHunk)
	}

	return fd, nil
}

// GetDiffForOpenFiles gets diffs for multiple files at once.
func (r *Repository) GetDiffForOpenFiles(paths []string) (map[string]*FileDiff, error) {
	results := make(map[string]*FileDiff)
	for _, p := range paths {
		fd, err := r.GetFileDiff(p)
		if err != nil {
			continue
		}
		results[p] = fd
	}
	return results, nil
}

// StageDiffHunk stages a specific hunk from the diff.
func (r *Repository) StageDiffHunk(relPath string, hunkIdx int) error {
	fd, err := r.GetFileDiff(relPath)
	if err != nil {
		return err
	}
	if hunkIdx < 0 || hunkIdx >= len(fd.Hunks) {
		return fmt.Errorf("invalid hunk index: %d", hunkIdx)
	}

	hunk := fd.Hunks[hunkIdx]
	var patchLines []string
	patchLines = append(patchLines, hunk.Header)
	for _, line := range hunk.Lines {
		switch line.Type {
		case DiffAdded:
			patchLines = append(patchLines, "+"+line.Content)
		case DiffDeleted:
			patchLines = append(patchLines, "-"+line.Content)
		case DiffContext:
			patchLines = append(patchLines, " "+line.Content)
		}
	}

	patchContent := strings.Join(patchLines, "\n")
	if patchContent == "" {
		return nil
	}

	cmd := exec.Command("git", "apply", "--cached", "--unidiff-zero", "-")
	cmd.Dir = r.Path
	cmd.Stdin = strings.NewReader(patchContent)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("stage hunk: %s: %w", string(output), err)
	}

	return nil
}
