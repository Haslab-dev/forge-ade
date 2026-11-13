package git

// extras.go exposes the remaining core/git surface the webview app offers but
// the native manager lacked: worktrees, conflict resolution, hunk revert,
// session checkpoints, graph pagination, commit bodies, and remote detection.
// Each is a thin, ctx-timeout wrapper over the shared core engine.

import (
	"context"
	"errors"
	"time"

	"github.com/hasdev/forge-ade/app/native/pkg/engine"
	internalgit "github.com/hasdev/forge-ade/core/git"
)

func ctxTimeout(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}

// ListWorktrees returns all linked and main worktrees for the repository.
func (gm *GitManager) ListWorktrees() ([]internalgit.GitWorktreeInfo, error) {
	ctx, cancel := ctxTimeout(10 * time.Second)
	defer cancel()
	return internalgit.ListWorktrees(ctx, gm.RootDir)
}

// CreateWorktree creates a new linked worktree at targetPath.
func (gm *GitManager) CreateWorktree(targetPath, branch string, createBranch bool) error {
	ctx, cancel := ctxTimeout(30 * time.Second)
	defer cancel()
	err := internalgit.CreateWorktree(ctx, gm.RootDir, targetPath, branch, createBranch)
	if err == nil && gm.engine != nil {
		gm.engine.GitEngine.Invalidate(gm.RootDir)
	}
	return err
}

// RemoveWorktree deletes a linked worktree.
func (gm *GitManager) RemoveWorktree(targetPath string, force bool) error {
	ctx, cancel := ctxTimeout(30 * time.Second)
	defer cancel()
	err := internalgit.RemoveWorktree(ctx, gm.RootDir, targetPath, force)
	if err == nil && gm.engine != nil {
		gm.engine.GitEngine.Invalidate(gm.RootDir)
	}
	return err
}

// PruneWorktrees removes stale worktree administrative records.
func (gm *GitManager) PruneWorktrees() error {
	ctx, cancel := ctxTimeout(15 * time.Second)
	defer cancel()
	err := internalgit.PruneWorktrees(ctx, gm.RootDir)
	if err == nil && gm.engine != nil {
		gm.engine.GitEngine.Invalidate(gm.RootDir)
	}
	return err
}

// GetConflictStageContent returns a conflict stage's blob content
// (stage 1 = base, 2 = ours, 3 = theirs).
func (gm *GitManager) GetConflictStageContent(path string, stage int) (string, error) {
	ctx, cancel := ctxTimeout(10 * time.Second)
	defer cancel()
	if gm.engine == nil {
		return "", errNoEngine
	}
	return gm.engine.GitEngine.GetConflictStageContent(ctx, gm.RootDir, path, stage)
}

// ResolveConflict resolves a conflicted file (action "ours" | "theirs").
func (gm *GitManager) ResolveConflict(path, action string) error {
	ctx, cancel := ctxTimeout(15 * time.Second)
	defer cancel()
	if gm.engine == nil {
		return errNoEngine
	}
	err := gm.engine.GitEngine.ResolveConflict(ctx, gm.RootDir, path, action)
	if err == nil {
		gm.Refresh()
	}
	return err
}

// FileDiffHunk is one hunk of a file's working-tree diff.
type FileDiffHunk = internalgit.DiffHunk

// GetFileDiffHunks returns the parsed hunks of a file's working-tree diff.
func (gm *GitManager) GetFileDiffHunks(path string) ([]FileDiffHunk, error) {
	ctx, cancel := ctxTimeout(10 * time.Second)
	defer cancel()
	if gm.engine == nil {
		return nil, errNoEngine
	}
	return gm.engine.GitEngine.GetFileDiffHunks(ctx, gm.RootDir, path)
}

// RevertDiffHunk restores a single hunk of a file to its HEAD content.
func (gm *GitManager) RevertDiffHunk(path string, hunkIndex int) error {
	ctx, cancel := ctxTimeout(15 * time.Second)
	defer cancel()
	if gm.engine == nil {
		return errNoEngine
	}
	err := gm.engine.GitEngine.RevertDiffHunk(ctx, gm.RootDir, path, hunkIndex)
	if err == nil {
		gm.Refresh()
	}
	return err
}

// CreateCheckpoint snapshots the workspace before an agent turn executes.
func (gm *GitManager) CreateCheckpoint(sessionID, turnID, description string) (*internalgit.CheckpointRecord, error) {
	ctx, cancel := ctxTimeout(20 * time.Second)
	defer cancel()
	return internalgit.CreateCheckpoint(ctx, gm.RootDir, sessionID, turnID, description)
}

// RestoreCheckpoint rolls the workspace files back to a checkpoint.
func (gm *GitManager) RestoreCheckpoint(sessionID, turnID string) error {
	ctx, cancel := ctxTimeout(30 * time.Second)
	defer cancel()
	err := internalgit.RestoreCheckpoint(ctx, gm.RootDir, sessionID, turnID)
	if err == nil && gm.engine != nil {
		gm.engine.GitEngine.Invalidate(gm.RootDir)
		gm.Refresh()
	}
	return err
}

// ListCheckpoints returns all checkpoints recorded for a session.
func (gm *GitManager) ListCheckpoints(sessionID string) ([]internalgit.CheckpointRecord, error) {
	return internalgit.ListCheckpoints(gm.RootDir, sessionID)
}

// GetCommitGraphPage returns a page of the commit graph with branch filter
// (GetGitCommitGraph binding parity: offset/limit/branch).
func (gm *GitManager) GetCommitGraphPage(offset, limit int, branch string) (*internalgit.CommitGraphResult, error) {
	ctx, cancel := ctxTimeout(15 * time.Second)
	defer cancel()
	if gm.engine == nil {
		return nil, errNoEngine
	}
	return gm.engine.GitEngine.GetCommitGraph(ctx, gm.RootDir, offset, limit, branch)
}

// GetCommitBody returns the full commit message body of a commit.
func (gm *GitManager) GetCommitBody(hash string) string {
	ctx, cancel := ctxTimeout(10 * time.Second)
	defer cancel()
	if gm.engine == nil {
		return ""
	}
	body, err := gm.engine.GitEngine.GetCommitBody(ctx, gm.RootDir, hash)
	if err != nil {
		return ""
	}
	return body
}

// GetFileContentAtCommit returns a file's content as of a commit (for
// reviewing historical versions).
func (gm *GitManager) GetFileContentAtCommit(path, hash string) string {
	ctx, cancel := ctxTimeout(10 * time.Second)
	defer cancel()
	if gm.engine == nil {
		return ""
	}
	content, err := gm.engine.GitEngine.GetFileContentAtCommit(ctx, gm.RootDir, path, hash)
	if err != nil {
		return ""
	}
	return content
}

// RemoteSource detects the active git remote and parses its host/provider
// (GitHub vs GitLab vs generic), used by the PRs pane.
func (gm *GitManager) RemoteSource() (*internalgit.GitRemoteSource, error) {
	ctx, cancel := ctxTimeout(10 * time.Second)
	defer cancel()
	if gm.engine == nil {
		return nil, errNoEngine
	}
	return gm.engine.GitEngine.GetRemoteSource(ctx, gm.RootDir)
}

// errNoEngine guards calls when no engine is wired (tests, detached state).
var errNoEngine = errors.New("git engine not initialized")

// Engine exposes the underlying composition root for UI panes that call the
// shared core/git surface (PRs, remote source) directly.
func (gm *GitManager) Engine() *engine.Engine {
	return gm.engine
}
