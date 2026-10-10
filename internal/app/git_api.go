package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/hasdev/forge-ade/internal/commitmsg"
	"github.com/hasdev/forge-ade/internal/git"
)

func (a *App) gitContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	base := a.ctx
	if base == nil {
		base = context.Background()
	}
	return context.WithTimeout(base, timeout)
}

func (a *App) resolveGitRepoPath(repoPath string) string {
	repoPath = strings.TrimSpace(repoPath)
	if repoPath == "" {
		repoPath = a.defaultFolder()
	}
	cur := repoPath
	if fi, err := os.Stat(cur); err == nil && !fi.IsDir() {
		cur = filepath.Dir(cur)
	}
	for {
		if _, err := os.Stat(filepath.Join(cur, ".git")); err == nil {
			return cur
		}
		parent := filepath.Dir(cur)
		if parent == cur || parent == "." || parent == "/" {
			break
		}
		cur = parent
	}
	return repoPath
}

// GetGitCommitGraph streams lightweight paginated Git commits with graph prefix.
// branch "" means all branches.
func (a *App) GetGitCommitGraph(repoPath string, offset int, limit int, branch string) (*git.CommitGraphResult, error) {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(30 * time.Second)
	defer cancel()
	return a.gitEngine.GetCommitGraph(ctx, repoPath, offset, limit, branch)
}

// GetGitBranches lists local branch names for the repo.
func (a *App) GetGitBranches(repoPath string) ([]string, error) {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(15 * time.Second)
	defer cancel()
	return a.gitEngine.GetBranches(ctx, repoPath)
}

// GitCheckout switches the working tree to the given branch.
func (a *App) GitCheckout(repoPath string, branch string) error {
	repoPath = a.resolveGitRepoPath(repoPath)
	if branch == "" {
		return fmt.Errorf("branch name is required")
	}
	ctx, cancel := a.gitContext(30 * time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "checkout", branch)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("checkout %s failed: %s", branch, strings.TrimSpace(string(out)))
	}
	a.emitEvent("git:changed", repoPath)
	return nil
}

// GitCheckoutNewBranch creates a new branch from HEAD and switches to it
// (git checkout -b). Used by the header branch switcher's
// "Create and switch to new branch…" action.
func (a *App) GitCheckoutNewBranch(repoPath string, name string) error {
	repoPath = a.resolveGitRepoPath(repoPath)
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("branch name is required")
	}
	ctx, cancel := a.gitContext(30 * time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "checkout", "-b", name)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("create branch %s failed: %s", name, strings.TrimSpace(string(out)))
	}
	a.emitEvent("git:changed", repoPath)
	return nil
}

// GetGitCommitDiff returns details and patch for a single commit.
func (a *App) GetGitCommitDiff(repoPath string, hash string) (string, error) {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(30 * time.Second)
	defer cancel()
	return a.gitEngine.GetCommitDiff(ctx, repoPath, hash)
}

// GetGitCommitBody returns the full commit message (subject + body) for a commit.
func (a *App) GetGitCommitBody(repoPath string, hash string) (string, error) {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(15 * time.Second)
	defer cancel()
	return a.gitEngine.GetCommitBody(ctx, repoPath, hash)
}

// GetGitFileDiff returns the unified diff of a single working-tree file against HEAD.
func (a *App) GetGitFileDiff(repoPath string, path string) (string, error) {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(30 * time.Second)
	defer cancel()
	return a.gitEngine.GetFileDiff(ctx, repoPath, path)
}

// GetGitCommitFileDiff returns the unified diff of a single file within a commit.
func (a *App) GetGitCommitFileDiff(repoPath string, hash string, path string) (string, error) {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(30 * time.Second)
	defer cancel()
	return a.gitEngine.GetCommitFileDiff(ctx, repoPath, hash, path)
}

// GetGitFileDiffHunks returns the structured hunks of a file's diff against HEAD.
func (a *App) GetGitFileDiffHunks(repoPath string, path string) ([]git.DiffHunk, error) {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(30 * time.Second)
	defer cancel()
	return a.gitEngine.GetFileDiffHunks(ctx, repoPath, path)
}

// RevertGitHunk reverse-applies a single diff hunk, restoring it to HEAD state.
func (a *App) RevertGitHunk(repoPath string, path string, hunkIndex int) error {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(15 * time.Second)
	defer cancel()
	return a.gitEngine.RevertDiffHunk(ctx, repoPath, path, hunkIndex)
}

// GetGitFileContentAtCommit returns the raw file content at a given commit.
func (a *App) GetGitFileContentAtCommit(repoPath string, hash string, path string) (string, error) {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(30 * time.Second)
	defer cancel()
	return a.gitEngine.GetFileContentAtCommit(ctx, repoPath, hash, path)
}

// GetGitStatus returns lightweight git status (staged, unstaged, untracked).
func (a *App) GetGitStatus(repoPath string) (*git.GitStatusResult, error) {
	a.sweepGitDirty()
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(30 * time.Second)
	defer cancel()
	return a.gitEngine.GetStatus(ctx, repoPath)
}

// GitStage stages files.
func (a *App) GitStage(repoPath string, paths []string) error {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(30 * time.Second)
	defer cancel()
	return a.gitEngine.Stage(ctx, repoPath, paths)
}

// GitUnstage unstages files.
func (a *App) GitUnstage(repoPath string, paths []string) error {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(30 * time.Second)
	defer cancel()
	return a.gitEngine.Unstage(ctx, repoPath, paths)
}

// GitDiscard discards file changes.
func (a *App) GitDiscard(repoPath string, paths []string) error {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(30 * time.Second)
	defer cancel()
	return a.gitEngine.Discard(ctx, repoPath, paths)
}

// GitCheckIgnored returns which of the given paths match gitignore rules.
func (a *App) GitCheckIgnored(repoPath string, paths []string) ([]string, error) {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(15 * time.Second)
	defer cancel()
	return a.gitEngine.CheckIgnored(ctx, repoPath, paths)
}

// GetGitConflictStageContent returns a conflicted file's content at a merge
// stage: 1 = common ancestor, 2 = ours, 3 = theirs.
func (a *App) GetGitConflictStageContent(repoPath string, path string, stage int) (string, error) {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(15 * time.Second)
	defer cancel()
	return a.gitEngine.GetConflictStageContent(ctx, repoPath, path, stage)
}

// GitResolveConflict resolves a conflicted file. action: "ours", "theirs", or
// "mark" (stage the current working-tree content).
func (a *App) GitResolveConflict(repoPath string, path string, action string) error {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(15 * time.Second)
	defer cancel()
	return a.gitEngine.ResolveConflict(ctx, repoPath, path, action)
}

// GitCommit commits staged changes.
func (a *App) GitCommit(repoPath string, message string) error {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(30 * time.Second)
	defer cancel()
	return a.gitEngine.Commit(ctx, repoPath, message)
}

// GitPush pushes commits to remote.
func (a *App) GitPush(repoPath string) error {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(60 * time.Second)
	defer cancel()
	return a.gitEngine.Push(ctx, repoPath)
}

// GitFetch updates remote-tracking branches from the default remote.
func (a *App) GitFetch(repoPath string) (string, error) {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(60 * time.Second)
	defer cancel()
	return a.gitEngine.Fetch(ctx, repoPath)
}

// GitMerge merges the given source commit/branch into the current branch.
func (a *App) GitMerge(repoPath string, source string, noFF bool, squash bool) (string, error) {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(30 * time.Second)
	defer cancel()
	return a.gitEngine.Merge(ctx, repoPath, source, noFF, squash)
}

// GenerateAICommitMessage generates a commit message using AI from staged diff with targeted provider/model.
func (a *App) GenerateAICommitMessage(repoPath string, providerID string, model string, instruction string) (string, error) {
	gen := &commitmsg.Generator{Git: a.gitEngine, LLM: a.llmClient}
	ctx, cancel := a.gitContext(45 * time.Second)
	defer cancel()
	return gen.Generate(ctx, a.resolveGitRepoPath(repoPath), providerID, model, instruction)
}
