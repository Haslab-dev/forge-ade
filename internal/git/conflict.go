package git

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)



// stageToRef maps a merge stage index to the git index ref.
//
//	1 = common ancestor, 2 = ours (current branch), 3 = theirs (merged branch)
func stageToRef(stage int) (string, error) {
	switch stage {
	case 1:
		return ":1:", nil
	case 2:
		return ":2:", nil
	case 3:
		return ":3:", nil
	default:
		return "", fmt.Errorf("invalid conflict stage %d (must be 1, 2 or 3)", stage)
	}
}

// GetConflictStageContent returns the content of a file at the given merge stage
// during a conflict. stage 1 = common ancestor, 2 = ours, 3 = theirs.
func (e *Engine) GetConflictStageContent(ctx context.Context, repoPath string, path string, stage int) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("file path cannot be empty")
	}
	ref, err := stageToRef(stage)
	if err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, "git", "show", ref+path)
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git show %s%s: %w", ref, path, err)
	}
	return string(out), nil
}

// ResolveConflict marks a conflicted file as resolved. action is one of:
//
//	"ours"   — accept the current branch version, stage it, and write to worktree
//	"theirs" — accept the incoming (merged) version, stage it, and write to worktree
//	"mark"   — keep the current working-tree content (whatever the user edited) and stage it
func (e *Engine) ResolveConflict(ctx context.Context, repoPath string, path string, action string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("file path cannot be empty")
	}

	switch strings.ToLower(strings.TrimSpace(action)) {
	case "ours":
		cmd := exec.CommandContext(ctx, "git", "checkout", "--ours", "--", path)
		cmd.Dir = repoPath
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("accept current failed: %s", string(out))
		}
	case "theirs":
		cmd := exec.CommandContext(ctx, "git", "checkout", "--theirs", "--", path)
		cmd.Dir = repoPath
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("accept incoming failed: %s", string(out))
		}
	case "mark":
		// Keep the working-tree content as-is (already edited by the user).
	default:
		return fmt.Errorf("unknown resolve action %q (expected ours, theirs or mark)", action)
	}

	// Stage the file to clear the conflict entry.
	addCmd := exec.CommandContext(ctx, "git", "add", "--", path)
	addCmd.Dir = repoPath
	if out, err := addCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git add failed: %s", string(out))
	}
	return nil
}
