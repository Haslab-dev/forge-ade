package git

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// DiffHunk is a single hunk of a unified diff.
type DiffHunk struct {
	OldStart int      `json:"oldStart"`
	OldLines int      `json:"oldLines"`
	NewStart int      `json:"newStart"`
	NewLines int      `json:"newLines"`
	Header   string   `json:"header"` // the "@@ ... @@" line
	Body     []string `json:"body"`   // hunk body lines (excluding the @@ header)
}

var hunkHeaderRe = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

// GetFileDiffHunks parses the unified diff of a single file against HEAD into
// structured hunks. Untracked files (no diff) return an empty slice.
func (e *Engine) GetFileDiffHunks(ctx context.Context, repoPath string, path string) ([]DiffHunk, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("file path cannot be empty")
	}
	cmd := exec.CommandContext(ctx, "git", "diff", "HEAD", "--", path)
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git diff error: %w", err)
	}

	var hunks []DiffHunk
	scanner := bufio.NewScanner(bytes.NewReader(out))
	var cur *DiffHunk
	flush := func() {
		if cur != nil {
			hunks = append(hunks, *cur)
			cur = nil
		}
	}
	for scanner.Scan() {
		line := scanner.Text()
		if m := hunkHeaderRe.FindStringSubmatch(line); m != nil {
			flush()
			oldLines, newLines := 1, 1
			if m[2] != "" {
				fmt.Sscanf(m[2], "%d", &oldLines)
			}
			if m[4] != "" {
				fmt.Sscanf(m[4], "%d", &newLines)
			}
			cur = &DiffHunk{
				OldStart: atoi(m[1]),
				OldLines: oldLines,
				NewStart: atoi(m[3]),
				NewLines: newLines,
				Header:   line,
			}
			continue
		}
		if cur != nil {
			cur.Body = append(cur.Body, line)
		}
	}
	flush()
	return hunks, nil
}

// RevertDiffHunk reverse-applies a single hunk of the file's working-tree diff,
// restoring that hunk region to its HEAD state. hunkIndex indexes the result of
// GetFileDiffHunks for the same path.
func (e *Engine) RevertDiffHunk(ctx context.Context, repoPath string, path string, hunkIndex int) error {
	hunks, err := e.GetFileDiffHunks(ctx, repoPath, path)
	if err != nil {
		return err
	}
	if hunkIndex < 0 || hunkIndex >= len(hunks) {
		return fmt.Errorf("hunk index %d out of range (0-%d)", hunkIndex, len(hunks)-1)
	}
	hunk := hunks[hunkIndex]

	var patch strings.Builder
	patch.WriteString("diff --git a/" + path + " b/" + path + "\n")
	patch.WriteString("--- a/" + path + "\n")
	patch.WriteString("+++ b/" + path + "\n")
	patch.WriteString(hunk.Header + "\n")
	for _, bodyLine := range hunk.Body {
		patch.WriteString(bodyLine + "\n")
	}

	apply := exec.CommandContext(ctx, "git", "apply", "-R", "--whitespace=nowarn", "-")
	apply.Dir = repoPath
	apply.Stdin = strings.NewReader(patch.String())
	out, err := apply.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git apply -R failed: %s", strings.TrimSpace(string(out)))
	}
	e.invalidate(repoPath)
	return nil
}

// StatusByPath returns a map of repo-relative path -> combined status character
// for all changed files. Characters: "U" (untracked/added), "M" (modified,
// including conflicts and renames), "D" (deleted). Priority: D > U > M.
func (e *Engine) StatusByPath(ctx context.Context, repoPath string) (map[string]string, error) {
	res, err := e.GetStatus(ctx, repoPath)
	if err != nil {
		return nil, err
	}
	prio := map[string]int{"M": 0, "U": 1, "D": 2}
	out := make(map[string]string)
	apply := func(entries []FileStatus) {
		for _, s := range entries {
			c := "M"
			if s.Status == "?" || s.Status == "A" {
				c = "U"
			} else if strings.Contains(s.Status, "D") {
				c = "D"
			}
			if cur, ok := out[s.Path]; !ok || prio[c] > prio[cur] {
				out[s.Path] = c
			}
		}
	}
	apply(res.Conflicts)
	apply(res.Staged)
	apply(res.Unstaged)
	apply(res.Untracked)
	return out, nil
}

// FindRepoRoot walks upward from dir to find the nearest git repository root.
// Returns false when no repository is found (e.g. inside a non-repo directory).
func FindRepoRoot(dir string) (string, bool) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", false
	}
	d := abs
	for {
		if fi, err := os.Stat(filepath.Join(d, ".git")); err == nil && fi != nil {
			return d, true
		}
		parent := filepath.Dir(d)
		if parent == d {
			return "", false
		}
		d = parent
	}
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}
