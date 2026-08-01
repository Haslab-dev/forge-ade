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
	Staging string `json:"staging"` // "staged", "unstaged", "untracked"
	Status  string `json:"status"`  // "M", "A", "D", "R", "?"
}

type GitStatusResult struct {
	Branch    string       `json:"branch"`
	Staged    []FileStatus `json:"staged"`
	Unstaged  []FileStatus `json:"unstaged"`
	Untracked []FileStatus `json:"untracked"`
}

// GetStatus returns lightweight git status using porcelain v2 format to avoid memory leaks.
func (e *Engine) GetStatus(ctx context.Context, repoPath string) (*GitStatusResult, error) {
	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain=v2", "-b")
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
				res.Untracked = append(res.Untracked, FileStatus{
					Path:    parts[1],
					Staging: "untracked",
					Status:  "?",
				})
			}
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
					Staging: "staged",
					Status:  stagedChar,
				})
			}
			if unstagedChar != "." {
				res.Unstaged = append(res.Unstaged, FileStatus{
					Path:    path,
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
					Staging: "staged",
					Status:  stagedChar,
				})
			}
			if unstagedChar != "." {
				res.Unstaged = append(res.Unstaged, FileStatus{
					Path:    path,
					Staging: "unstaged",
					Status:  unstagedChar,
				})
			}
		}
	}

	return res, nil
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

// Discard reverts modified/untracked files.
func (e *Engine) Discard(ctx context.Context, repoPath string, paths []string) error {
	if len(paths) == 0 {
		cmd1 := exec.CommandContext(ctx, "git", "restore", ".")
		cmd1.Dir = repoPath
		_ = cmd1.Run()

		cmd2 := exec.CommandContext(ctx, "git", "clean", "-fd")
		cmd2.Dir = repoPath
		return cmd2.Run()
	}

	args := append([]string{"restore", "--"}, paths...)
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoPath
	_ = cmd.Run()

	cleanArgs := append([]string{"clean", "-fd", "--"}, paths...)
	cleanCmd := exec.CommandContext(ctx, "git", cleanArgs...)
	cleanCmd.Dir = repoPath
	return cleanCmd.Run()
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

