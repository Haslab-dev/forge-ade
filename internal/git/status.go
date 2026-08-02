package git

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type FileStatus struct {
	Path    string `json:"path"`
	Dir     string `json:"dir"`     // parent directory path, e.g. "src/lib"
	Staging string `json:"staging"` // "staged", "unstaged", "untracked"
	Status  string `json:"status"`  // "M", "A", "D", "R", "?"
}

type GitStatusResult struct {
	Branch    string       `json:"branch"`
	Staged    []FileStatus `json:"staged"`
	Unstaged  []FileStatus `json:"unstaged"`
	Untracked []FileStatus `json:"untracked"`
	Conflicts []FileStatus `json:"conflicts"`
}

// GetStatus returns lightweight git status using porcelain v2 format to avoid memory leaks.
func (e *Engine) GetStatus(ctx context.Context, repoPath string) (*GitStatusResult, error) {
	// -uall lists every untracked FILE individually (VS Code behavior) instead
	// of collapsing an untracked directory into a single "? dir/" entry whose
	// filename is empty and can't be staged/opened.
	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain=v2", "-b", "-uall")
	cmd.Dir = repoPath

	outBytes, err := cmd.CombinedOutput()
	if err != nil {
		return &GitStatusResult{
			Branch:    "unknown",
			Staged:    []FileStatus{},
			Unstaged:  []FileStatus{},
			Untracked: []FileStatus{},
		}, nil
	}

	res := &GitStatusResult{
		Branch:    "main",
		Staged:    make([]FileStatus, 0),
		Unstaged:  make([]FileStatus, 0),
		Untracked: make([]FileStatus, 0),
		Conflicts: make([]FileStatus, 0),
	}

	scanner := bufio.NewScanner(bytes.NewReader(outBytes))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "# branch.head ") {
			res.Branch = strings.TrimPrefix(line, "# branch.head ")
			continue
		}

		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}

		if parts[0] == "?" {
			// Untracked file: ? path
			if len(parts) >= 2 {
				path := parts[1]
				res.Untracked = append(res.Untracked, FileStatus{
					Path:    path,
					Dir:     dirOf(path),
					Staging: "untracked",
					Status:  "?",
				})
			}
			continue
		}

		if parts[0] == "u" && len(parts) >= 10 {
			// Unmerged (conflict) entry:
			// u <XY> <sub> <m1> <m2> <m3> <mW> <h1> <h2> <h3> <path>
			xy := parts[1]
			path := parts[10]
			res.Conflicts = append(res.Conflicts, FileStatus{
				Path:    path,
				Dir:     dirOf(path),
				Staging: "conflict",
				Status:  xy, // e.g. UU, AU, UA, DU, UD, AA, DD
			})
			continue
		}

		if parts[0] == "1" && len(parts) >= 9 {
			// Ordinary changed entry: 1 <XY> <sub> <mH> <mI> <mW> <hH> <hI> <path>
			xy := parts[1]
			path := parts[8]

			stagedChar := string(xy[0])
			unstagedChar := string(xy[1])

			if stagedChar != "." {
				res.Staged = append(res.Staged, FileStatus{
					Path:    path,
					Dir:     dirOf(path),
					Staging: "staged",
					Status:  stagedChar,
				})
			}
			if unstagedChar != "." {
				res.Unstaged = append(res.Unstaged, FileStatus{
					Path:    path,
					Dir:     dirOf(path),
					Staging: "unstaged",
					Status:  unstagedChar,
				})
			}
		} else if parts[0] == "2" && len(parts) >= 10 {
			// Renamed/copied entry: 2 <XY> <sub> <mH> <mI> <mW> <hH> <hI> <Xscore> <path>
			xy := parts[1]
			path := parts[9]

			stagedChar := string(xy[0])
			unstagedChar := string(xy[1])

			if stagedChar != "." {
				res.Staged = append(res.Staged, FileStatus{
					Path:    path,
					Dir:     dirOf(path),
					Staging: "staged",
					Status:  stagedChar,
				})
			}
			if unstagedChar != "." {
				res.Unstaged = append(res.Unstaged, FileStatus{
					Path:    path,
					Dir:     dirOf(path),
					Staging: "unstaged",
					Status:  unstagedChar,
				})
			}
		}
	}

	return res, nil
}

// dirOf returns the parent directory of a path, or "" for a top-level file.
func dirOf(path string) string {
	idx := strings.LastIndex(path, "/")
	if idx <= 0 {
		return ""
	}
	return path[:idx]
}

// Stage adds files to staging index.
func (e *Engine) Stage(ctx context.Context, repoPath string, paths []string) error {
	args := []string{"add"}
	if len(paths) == 0 {
		args = append(args, ".")
	} else {
		args = append(args, "--")
		args = append(args, paths...)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoPath
	return cmd.Run()
}

// Unstage removes files from staging index.
func (e *Engine) Unstage(ctx context.Context, repoPath string, paths []string) error {
	args := []string{"restore", "--staged"}
	if len(paths) == 0 {
		args = append(args, ".")
	} else {
		args = append(args, "--")
		args = append(args, paths...)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoPath
	return cmd.Run()
}

// Discard reverts file changes back to HEAD, mirroring VS Code's "Discard
// Changes": tracked files are fully reset (staged + worktree) with
// `git restore --staged --worktree`, and untracked files are removed with
// `git clean`. For each path only the command that applies is run — running
// `git clean` on a tracked file (or `restore` on an untracked one) errors, so
// each path falls back to the other command before being counted as failed.
func (e *Engine) Discard(ctx context.Context, repoPath string, paths []string) error {
	if len(paths) == 0 {
		cmd1 := exec.CommandContext(ctx, "git", "restore", "--staged", "--worktree", ".")
		cmd1.Dir = repoPath
		_ = cmd1.Run()

		cmd2 := exec.CommandContext(ctx, "git", "clean", "-fd")
		cmd2.Dir = repoPath
		return cmd2.Run()
	}

	var failures, processed int
	for _, p := range paths {
		if strings.TrimSpace(p) == "" {
			continue
		}
		processed++
		restoreCmd := exec.CommandContext(ctx, "git", "restore", "--staged", "--worktree", "--", p)
		restoreCmd.Dir = repoPath
		if err := restoreCmd.Run(); err != nil {
			// Not a tracked file (untracked, staged-new, or a new dir) — remove it.
			cleanCmd := exec.CommandContext(ctx, "git", "clean", "-fd", "--", p)
			cleanCmd.Dir = repoPath
			if cerr := cleanCmd.Run(); cerr != nil {
				failures++
			}
		}
	}

	// Only report an error when every path failed both restore AND clean.
	if processed > 0 && failures == processed {
		return fmt.Errorf("discard failed: no tracked or untracked changes to discard for the given paths")
	}
	return nil
}

// Commit creates a git commit.
func (e *Engine) Commit(ctx context.Context, repoPath string, message string) error {
	if strings.TrimSpace(message) == "" {
		return fmt.Errorf("commit message cannot be empty")
	}
	cmd := exec.CommandContext(ctx, "git", "commit", "-m", message)
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", string(out))
	}
	return nil
}

// Push pushes committed commits to remote.
func (e *Engine) Push(ctx context.Context, repoPath string) error {
	cmd := exec.CommandContext(ctx, "git", "push")
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", string(out))
	}
	return nil
}

// GetStagedDiff returns the staged diff for AI commit generation.
func (e *Engine) GetStagedDiff(ctx context.Context, repoPath string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "diff", "--staged")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// GetStagedDiffStat returns the diff stat summary for the staged changes.
func (e *Engine) GetStagedDiffStat(ctx context.Context, repoPath string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "diff", "--staged", "--stat")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// GetFileDiff returns the unified diff for a single file against HEAD
// (combines staged + unstaged working-tree changes). Untracked files have no diff.
func (e *Engine) GetFileDiff(ctx context.Context, repoPath string, path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("file path cannot be empty")
	}
	cmd := exec.CommandContext(ctx, "git", "diff", "HEAD", "--", path)
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git diff error: %w", err)
	}
	return string(out), nil
}

// Fetch updates remote-tracking branches from the default remote.
func (e *Engine) Fetch(ctx context.Context, repoPath string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "fetch", "--prune")
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// Merge merges the given source commit/branch into the current branch.
// When noFF is true a merge commit is always created; when squash is true
// changes are applied without creating a merge commit.
func (e *Engine) Merge(ctx context.Context, repoPath string, source string, noFF bool, squash bool) (string, error) {
	if strings.TrimSpace(source) == "" {
		return "", fmt.Errorf("merge source cannot be empty")
	}
	args := []string{"merge"}
	if noFF {
		args = append(args, "--no-ff")
	}
	if squash {
		args = append(args, "--squash")
	}
	args = append(args, "--")
	args = append(args, source)
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	return string(out), err
}

