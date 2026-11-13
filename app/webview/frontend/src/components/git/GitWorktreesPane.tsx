import React, { useState, useEffect, useCallback } from 'react';
import {
  GitFork,
  Plus,
  Trash2,
  ExternalLink,
  RefreshCw,
  Folder,
  GitBranch,
  ShieldAlert,
  Loader2,
  CheckCircle2
} from 'lucide-react';
import { GetGitWorktrees, CreateGitWorktree, RemoveGitWorktree, PruneGitWorktrees, GitWorktreeInfo } from '../../lib/wails';
import { useWorkspace } from '../../stores/workspaceStore';
import { showToast } from '../../lib/toast';

interface GitWorktreesPaneProps {
  repoPath: string;
}

export const GitWorktreesPane: React.FC<GitWorktreesPaneProps> = ({ repoPath }) => {
  const [worktrees, setWorktrees] = useState<GitWorktreeInfo[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [isCreating, setIsCreating] = useState(false);
  const [newWorktreeName, setNewWorktreeName] = useState('');
  const [newBranchName, setNewBranchName] = useState('');
  const { openFolder, activeWorkspacePath } = useWorkspace();

  const loadWorktrees = useCallback(async () => {
    if (!repoPath) return;
    setIsLoading(true);
    try {
      const list = await GetGitWorktrees(repoPath);
      setWorktrees(list || []);
    } catch (err: any) {
      showToast(err?.message || 'Failed to list worktrees', 'error');
    } finally {
      setIsLoading(false);
    }
  }, [repoPath]);

  useEffect(() => {
    void loadWorktrees();
  }, [loadWorktrees]);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    const name = newWorktreeName.trim();
    if (!name) return;
    const branch = newBranchName.trim() || `worktree/${name}`;
    const targetPath = `${repoPath}/.forge/worktrees/${name}`;

    try {
      await CreateGitWorktree(repoPath, targetPath, branch, true);
      showToast(`Worktree "${name}" created on branch "${branch}"`, 'info');
      setNewWorktreeName('');
      setNewBranchName('');
      setIsCreating(false);
      await loadWorktrees();
    } catch (err: any) {
      showToast(err?.message || 'Failed to create worktree', 'error');
    }
  };

  const handleRemove = async (targetPath: string) => {
    if (!window.confirm(`Remove worktree at ${targetPath}?`)) return;
    try {
      await RemoveGitWorktree(repoPath, targetPath, true);
      showToast('Worktree removed', 'info');
      await loadWorktrees();
    } catch (err: any) {
      showToast(err?.message || 'Failed to remove worktree', 'error');
    }
  };

  const handlePrune = async () => {
    try {
      await PruneGitWorktrees(repoPath);
      showToast('Worktrees pruned', 'info');
      await loadWorktrees();
    } catch (err: any) {
      showToast(err?.message || 'Failed to prune worktrees', 'error');
    }
  };

  return (
    <div className="flex-1 flex flex-col h-full overflow-hidden bg-background">
      {/* Toolbar Header */}
      <div className="flex h-10 items-center justify-between px-4 border-b border-border bg-surface shrink-0">
        <div className="flex items-center gap-2 font-mono text-ui-xs text-foreground-subtle">
          <GitFork className="size-3.5 text-primary" />
          <span className="font-semibold text-foreground">Isolated Git Worktrees</span>
          <span className="rounded bg-surface-hover px-1.5 py-0.5 text-[10px] uppercase font-semibold text-foreground-subtlest">
            {worktrees.length} total
          </span>
        </div>

        <div className="flex items-center gap-1.5">
          <button
            type="button"
            onClick={handlePrune}
            className="flex h-6 items-center gap-1 px-2 rounded border border-border text-[11px] text-foreground-subtle hover:bg-surface-hover hover:text-foreground cursor-pointer"
            title="Prune stale worktree metadata"
          >
            Prune
          </button>
          <button
            type="button"
            onClick={() => void loadWorktrees()}
            className="flex size-6 items-center justify-center rounded border border-border text-foreground-subtle hover:bg-surface-hover hover:text-foreground cursor-pointer"
            title="Refresh worktrees"
          >
            <RefreshCw className={`size-3.5 ${isLoading ? 'animate-spin' : ''}`} />
          </button>
          <button
            type="button"
            onClick={() => setIsCreating(v => !v)}
            className="flex h-6 items-center gap-1 px-2 rounded bg-primary text-primary-foreground font-medium text-[11px] hover:bg-primary/90 cursor-pointer shadow-xs"
          >
            <Plus className="size-3" />
            New Worktree
          </button>
        </div>
      </div>

      {/* Create Worktree Modal / Panel */}
      {isCreating && (
        <form onSubmit={handleCreate} className="p-4 border-b border-border bg-surface/70 space-y-3 shrink-0">
          <div className="text-ui-xs font-semibold text-foreground">Create Isolated Subagent Worktree</div>
          <p className="text-[11px] text-foreground-subtle">
            Allows parallel agent test runs, refactoring, and code modifications without dirtying your main workspace.
          </p>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            <div>
              <label className="block text-[11px] font-mono text-foreground-subtle mb-1">Worktree Subfolder Name</label>
              <input
                type="text"
                placeholder="e.g. subagent-tests or feature-auth"
                value={newWorktreeName}
                onChange={e => setNewWorktreeName(e.target.value)}
                className="w-full px-2.5 py-1.5 rounded border border-border bg-background text-ui-xs font-mono outline-none text-foreground placeholder-foreground-subtle"
                autoFocus
              />
            </div>
            <div>
              <label className="block text-[11px] font-mono text-foreground-subtle mb-1">New Branch Name (optional)</label>
              <input
                type="text"
                placeholder="Leave blank for worktree/{name}"
                value={newBranchName}
                onChange={e => setNewBranchName(e.target.value)}
                className="w-full px-2.5 py-1.5 rounded border border-border bg-background text-ui-xs font-mono outline-none text-foreground placeholder-foreground-subtle"
              />
            </div>
          </div>
          <div className="flex justify-end gap-2 pt-1">
            <button
              type="button"
              onClick={() => setIsCreating(false)}
              className="px-3 py-1 rounded text-ui-xs text-foreground-subtle hover:bg-surface-hover cursor-pointer"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={!newWorktreeName.trim()}
              className="px-3 py-1 rounded bg-primary text-primary-foreground font-medium text-ui-xs disabled:opacity-50 cursor-pointer"
            >
              Create Worktree
            </button>
          </div>
        </form>
      )}

      {/* Worktrees List */}
      <div className="flex-1 overflow-auto p-4 space-y-2.5">
        {isLoading && worktrees.length === 0 ? (
          <div className="flex items-center justify-center p-12 text-foreground-subtle gap-2 text-ui-xs">
            <Loader2 className="size-4 animate-spin text-primary" />
            <span>Loading worktrees…</span>
          </div>
        ) : worktrees.length === 0 ? (
          <div className="p-12 text-center text-foreground-subtle text-ui-xs space-y-2">
            <GitFork className="size-8 mx-auto text-foreground-subtlest" />
            <p>No linked git worktrees found.</p>
            <p className="text-[11px] text-foreground-subtlest">
              Create an isolated worktree to let autonomous agents run tests in parallel.
            </p>
          </div>
        ) : (
          worktrees.map((wt, idx) => {
            const isMain = idx === 0;
            const isCurrent = wt.path === activeWorkspacePath;

            return (
              <div
                key={wt.path}
                className="flex flex-col md:flex-row md:items-center justify-between p-3 rounded-lg border border-border bg-surface/50 hover:bg-surface transition-colors gap-3"
              >
                <div className="flex items-start gap-3 min-w-0">
                  <div className="p-2 rounded-md bg-panel border border-border/60 shrink-0 mt-0.5">
                    <Folder className="size-4 text-primary" />
                  </div>
                  <div className="min-w-0 space-y-1">
                    <div className="flex items-center gap-2 flex-wrap">
                      <span className="font-mono text-ui-sm font-semibold text-foreground truncate">
                        {wt.path.split('/').filter(Boolean).pop()}
                      </span>
                      {isMain && (
                        <span className="rounded bg-primary/20 text-primary text-[10px] px-1.5 py-0.2 font-semibold uppercase">
                          Main Repo
                        </span>
                      )}
                      {isCurrent && (
                        <span className="flex items-center gap-1 rounded bg-success/20 text-success text-[10px] px-1.5 py-0.2 font-semibold uppercase">
                          <CheckCircle2 className="size-2.5" />
                          Active Workspace
                        </span>
                      )}
                      {wt.isLocked && (
                        <span className="flex items-center gap-1 rounded bg-warning/20 text-warning text-[10px] px-1.5 py-0.2 font-semibold uppercase">
                          <ShieldAlert className="size-2.5" />
                          Locked
                        </span>
                      )}
                    </div>
                    <div className="flex items-center gap-3 text-ui-xs font-mono text-foreground-subtle">
                      <span className="flex items-center gap-1 truncate text-foreground-secondary">
                        <GitBranch className="size-3 text-primary shrink-0" />
                        {wt.branch || '(detached)'}
                      </span>
                      <span className="text-foreground-subtlest">•</span>
                      <span className="truncate text-[11px]" title={wt.path}>
                        {wt.path}
                      </span>
                      {wt.head && (
                        <>
                          <span className="text-foreground-subtlest">•</span>
                          <span className="text-foreground-subtlest">{wt.head.slice(0, 7)}</span>
                        </>
                      )}
                    </div>
                  </div>
                </div>

                <div className="flex items-center gap-1.5 shrink-0 self-end md:self-center">
                  {!isCurrent && (
                    <button
                      type="button"
                      onClick={() => void openFolder(wt.path)}
                      className="flex h-7 items-center gap-1 px-2.5 rounded border border-border text-ui-xs text-foreground-subtle hover:bg-surface-hover hover:text-foreground cursor-pointer transition-colors"
                      title="Open this worktree as active workspace"
                    >
                      <ExternalLink className="size-3" />
                      Open
                    </button>
                  )}
                  {!isMain && (
                    <button
                      type="button"
                      onClick={() => void handleRemove(wt.path)}
                      className="flex size-7 items-center justify-center rounded border border-border text-destructive/80 hover:bg-destructive/10 hover:text-destructive cursor-pointer transition-colors"
                      title="Delete this worktree"
                    >
                      <Trash2 className="size-3" />
                    </button>
                  )}
                </div>
              </div>
            );
          })
        )}
      </div>
    </div>
  );
};
