package git

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GitWorktreeInfo represents a linked or main git worktree.
type GitWorktreeInfo struct {
	Path       string `json:"path"`
	Head       string `json:"head"`
	Branch     string `json:"branch"`
	IsBare     bool   `json:"isBare"`
	IsDetached bool   `json:"isDetached"`
	IsLocked   bool   `json:"isLocked"`
	LockReason string `json:"lockReason"`
	Prunable   string `json:"prunable"`
}

// ListWorktrees returns all linked and main worktrees for the repository.
func ListWorktrees(ctx context.Context, repoPath string) ([]GitWorktreeInfo, error) {
	out, err := gitCmd(ctx, repoPath, "worktree", "list", "--porcelain").Output()
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w", err)
	}

	var worktrees []GitWorktreeInfo
	var current GitWorktreeInfo

	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			if current.Path != "" {
				worktrees = append(worktrees, current)
				current = GitWorktreeInfo{}
			}
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		key := parts[0]
		val := ""
		if len(parts) > 1 {
			val = parts[1]
		}

		switch key {
		case "worktree":
			current.Path = val
		case "HEAD":
			current.Head = val
		case "branch":
			current.Branch = strings.TrimPrefix(val, "refs/heads/")
		case "bare":
			current.IsBare = true
		case "detached":
			current.IsDetached = true
		case "locked":
			current.IsLocked = true
			current.LockReason = val
		case "prunable":
			current.Prunable = val
		}
	}

	if current.Path != "" {
		worktrees = append(worktrees, current)
	}

	return worktrees, nil
}

// CreateWorktree creates a new linked worktree at targetPath.
func CreateWorktree(ctx context.Context, repoPath string, targetPath string, branch string, createBranch bool) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return fmt.Errorf("create worktree parent dir: %w", err)
	}

	args := []string{"worktree", "add"}
	if createBranch && branch != "" {
		args = append(args, "-b", branch, targetPath)
	} else if branch != "" {
		args = append(args, targetPath, branch)
	} else {
		args = append(args, targetPath)
	}

	out, err := gitCmd(ctx, repoPath, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree add failed: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// RemoveWorktree deletes a linked worktree.
func RemoveWorktree(ctx context.Context, repoPath string, targetPath string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, targetPath)

	out, err := gitCmd(ctx, repoPath, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree remove failed: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// PruneWorktrees cleans up stale worktree administrative data.
func PruneWorktrees(ctx context.Context, repoPath string) error {
	out, err := gitCmd(ctx, repoPath, "worktree", "prune").CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree prune failed: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}
