import React, { useState, useEffect, useMemo, useRef } from 'react';
import {
  GitBranch,
  GitCommit,
  GitPullRequest,
  GitMerge,
  ArrowDown,
  ArrowUp,
  RefreshCw,
  Sparkles,
  Plus,
  Trash2,
  Check,
  CheckSquare,
  Square,
  MinusSquare,
  Clock,
  User,
  Folder,
  FileText,
  ChevronRight,
  ChevronDown,
  Layers,
  History,
  FileCode,
  RotateCcw,
  ExternalLink,
  Copy,
  X,
  Search,
  Upload,
  Download,
  AlertCircle,
  FileDiff as FileDiffIcon,
  Columns,
  Maximize2
} from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { ApiBridge } from '../../services/apiBridge';
import { showToast } from '../../lib/toast';
import { parseUnifiedDiff, parseUnifiedDiffByFiles, ParsedDiffLine } from '../diff/DiffViewer';
import { GitGraphLane } from '../editor/GitGraphLane';
import { GitDiffPane } from './GitDiffPane';
import { GitAccordionDiffList } from './GitAccordionDiffList';
import { GitCommitDiffView } from './GitCommitDiffView';
import { timeAgo } from './gitDiffUtils';

interface GitClientViewProps {
  onClose?: () => void;
  className?: string;
}

interface CommitNode {
  hash: string;
  short_hash: string;
  parents: string[];
  author_name: string;
  author_email: string;
  timestamp: string;
  message: string;
  graph_prefix: string;
  decorations: string;
  status?: string;
}

interface GitFileItem {
  path: string;
  status: string;
  staging?: 'staged' | 'unstaged' | 'untracked';
  dir?: string;
  additions?: number;
  deletions?: number;
}

export const GitClientView: React.FC<GitClientViewProps> = ({ onClose, className = '' }) => {
  const {
    activeWorkspacePath,
    gitBranch,
    gitFiles,
    gitCommits,
    refreshGitStatus,
    refreshGitLog
  } = useWorkspace();

  const cwd = activeWorkspacePath || '';

  // View state: 'changes' (GitHub Desktop style working tree) or 'history' (SourceTree log & graph)
  const [activeTab, setActiveTab] = useState<'changes' | 'history'>('changes');
  const [selectedFile, setSelectedFile] = useState<GitFileItem | null>(null);
  const [fileSearch, setFileSearch] = useState('');

  // Commit Form State
  const [commitSummary, setCommitSummary] = useState('');
  const [commitDesc, setCommitDesc] = useState('');
  const [isCommitting, setIsCommitting] = useState(false);
  const [isAiGenerating, setIsAiGenerating] = useState(false);

  // Remote Actions State
  const [isFetching, setIsFetching] = useState(false);
  const [isPulling, setIsPulling] = useState(false);
  const [isPushing, setIsPushing] = useState(false);
  const [isRefreshing, setIsRefreshing] = useState(false);

  // Branch Switcher State
  const [branchMenuOpen, setBranchMenuOpen] = useState(false);
  const [branches, setBranches] = useState<string[]>([]);
  const [branchSearch, setBranchSearch] = useState('');
  const [isCreatingBranch, setIsCreatingBranch] = useState(false);
  const [newBranchName, setNewBranchName] = useState('');
  const [isSwitchingBranch, setIsSwitchingBranch] = useState(false);
  const branchMenuRef = useRef<HTMLDivElement>(null);

  // Diff inspection
  const [activeDiffContent, setActiveDiffContent] = useState<string>('');
  const [diffLoading, setDiffLoading] = useState(false);
  const [splitDiffMode, setSplitDiffMode] = useState(false);

  // History / Commit Graph state
  const [historyCommits, setHistoryCommits] = useState<CommitNode[]>([]);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [selectedCommit, setSelectedCommit] = useState<CommitNode | null>(null);
  const [commitDiffFiles, setCommitDiffFiles] = useState<Array<{ path: string; additions: number; deletions: number; diff: string }>>([]);
  const [rawCommitDiff, setRawCommitDiff] = useState<string>('');
  const [loadingCommitDetails, setLoadingCommitDetails] = useState(false);
  const [historySearch, setHistorySearch] = useState('');

  // Resizable panel states
  const [changesLeftWidth, setChangesLeftWidth] = useState(340);
  const [historyLogWidth, setHistoryLogWidth] = useState(420);
  const [activeResizer, setActiveResizer] = useState<'changes-left' | 'history-log' | null>(null);
  const mainWorkAreaRef = useRef<HTMLDivElement>(null);
  const historyWorkAreaRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!activeResizer) return;

    const onMouseMove = (e: MouseEvent) => {
      if (activeResizer === 'changes-left') {
        if (!mainWorkAreaRef.current) return;
        const rect = mainWorkAreaRef.current.getBoundingClientRect();
        const relX = e.clientX - rect.left;
        const clamped = Math.max(240, Math.min(Math.min(650, rect.width - 300), relX));
        setChangesLeftWidth(clamped);
      } else if (activeResizer === 'history-log') {
        if (!historyWorkAreaRef.current) return;
        const rect = historyWorkAreaRef.current.getBoundingClientRect();
        const relX = e.clientX - rect.left;
        const clamped = Math.max(260, Math.min(Math.min(750, rect.width - 400), relX));
        setHistoryLogWidth(clamped);
      }
    };

    const onMouseUp = () => {
      setActiveResizer(null);
    };

    document.addEventListener('mousemove', onMouseMove);
    document.addEventListener('mouseup', onMouseUp);
    return () => {
      document.removeEventListener('mousemove', onMouseMove);
      document.removeEventListener('mouseup', onMouseUp);
    };
  }, [activeResizer]);

  // Close branch menu on outside click
  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (branchMenuRef.current && !branchMenuRef.current.contains(e.target as Node)) {
        setBranchMenuOpen(false);
        setIsCreatingBranch(false);
      }
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, []);

  // Load branches
  const loadBranches = async () => {
    if (!cwd) return;
    try {
      const list = await ApiBridge.getGitBranches(cwd);
      setBranches(Array.isArray(list) ? list : []);
    } catch {
      setBranches([]);
    }
  };

  useEffect(() => {
    loadBranches();
  }, [cwd, gitBranch]);

  // Load commit graph when switching to history tab
  const loadHistoryGraph = async () => {
    if (!cwd) return;
    setHistoryLoading(true);
    try {
      const res = await ApiBridge.getGitCommitGraph(cwd, 0, 100, '');
      if (res?.commits) {
        setHistoryCommits(res.commits);
        if (!selectedCommit && res.commits.length > 0) {
          selectCommitItem(res.commits[0]);
        }
      }
    } catch (e) {
      console.warn('Failed to load commit graph', e);
    } finally {
      setHistoryLoading(false);
    }
  };

  useEffect(() => {
    if (activeTab === 'history') {
      loadHistoryGraph();
    }
  }, [activeTab, cwd]);

  // Categorize files
  const stagedFiles = useMemo(() => gitFiles.filter(f => f.staging === 'staged'), [gitFiles]);
  const unstagedFiles = useMemo(() => gitFiles.filter(f => f.staging === 'unstaged' || f.staging === 'untracked'), [gitFiles]);

  const filteredStaged = useMemo(() => {
    if (!fileSearch.trim()) return stagedFiles;
    const q = fileSearch.toLowerCase();
    return stagedFiles.filter(f => f.path.toLowerCase().includes(q));
  }, [stagedFiles, fileSearch]);

  const filteredUnstaged = useMemo(() => {
    if (!fileSearch.trim()) return unstagedFiles;
    const q = fileSearch.toLowerCase();
    return unstagedFiles.filter(f => f.path.toLowerCase().includes(q));
  }, [unstagedFiles, fileSearch]);

  // Auto-select first file if nothing selected
  useEffect(() => {
    if (!selectedFile && gitFiles.length > 0) {
      const first = stagedFiles[0] || unstagedFiles[0];
      if (first) selectFileItem(first);
    } else if (selectedFile && !gitFiles.some(f => f.path === selectedFile.path)) {
      const first = stagedFiles[0] || unstagedFiles[0] || null;
      setSelectedFile(first);
      if (first) selectFileItem(first);
      else setActiveDiffContent('');
    }
  }, [gitFiles]);

  const selectFileItem = async (file: GitFileItem) => {
    setSelectedFile(file);
    setDiffLoading(true);
    try {
      let diff = '';
      if (file.staging === 'untracked') {
        const text = await ApiBridge.readFile(file.path);
        // format as pseudo diff
        diff = `--- /dev/null\n+++ b/${file.path}\n@@ -0,0 +1,${(text.match(/\n/g) || []).length + 1} @@\n` +
          text.split('\n').map(l => `+${l}`).join('\n');
      } else {
        diff = await ApiBridge.gitDiff(file.path, cwd, file.staging === 'staged');
      }
      setActiveDiffContent(diff || '(No diff detected)');
    } catch (err: any) {
      setActiveDiffContent(`Error loading diff: ${err?.message || String(err)}`);
    } finally {
      setDiffLoading(false);
    }
  };

  const selectCommitItem = async (commit: CommitNode) => {
    setSelectedCommit(commit);
    setLoadingCommitDetails(true);

    try {
      const raw = await ApiBridge.gitCommitDiff(commit.hash, cwd);
      setRawCommitDiff(raw || '');
      const sections = parseUnifiedDiffByFiles(raw || '');
      const files = sections
        .filter(s => (s.filePath && s.filePath !== 'Diff') || s.lines.length > 0)
        .map(s => ({
          path: s.filePath,
          additions: s.additions,
          deletions: s.deletions,
          diff: s.rawText || ''
        }));

      setCommitDiffFiles(files);
    } catch (err: any) {
      setRawCommitDiff(`Error: ${err?.message || 'Failed to inspect commit'}`);
      setCommitDiffFiles([]);
    } finally {
      setLoadingCommitDetails(false);
    }
  };

  // Stage / Unstage actions
  const handleStageFile = async (path: string, e?: React.MouseEvent) => {
    e?.stopPropagation();
    try {
      await ApiBridge.gitStage(path, cwd);
      await refreshGitStatus();
      if (selectedFile?.path === path) {
        selectFileItem({ ...selectedFile, staging: 'staged' });
      }
    } catch (err) {
      showToast(err instanceof Error ? err.message : 'Stage failed', 'error');
    }
  };

  const handleUnstageFile = async (path: string, e?: React.MouseEvent) => {
    e?.stopPropagation();
    try {
      await ApiBridge.gitUnstage(path, cwd);
      await refreshGitStatus();
      if (selectedFile?.path === path) {
        selectFileItem({ ...selectedFile, staging: 'unstaged' });
      }
    } catch (err) {
      showToast(err instanceof Error ? err.message : 'Unstage failed', 'error');
    }
  };

  const handleStageAll = async () => {
    try {
      await Promise.all(unstagedFiles.map(f => ApiBridge.gitStage(f.path, cwd)));
      await refreshGitStatus();
      showToast('All changes staged', 'info');
    } catch {
      showToast('Failed to stage all files', 'error');
    }
  };

  const handleUnstageAll = async () => {
    try {
      await Promise.all(stagedFiles.map(f => ApiBridge.gitUnstage(f.path, cwd)));
      await refreshGitStatus();
      showToast('All changes unstaged', 'info');
    } catch {
      showToast('Failed to unstage all files', 'error');
    }
  };

  const handleDiscardFile = async (path: string, e?: React.MouseEvent) => {
    e?.stopPropagation();
    if (!confirm(`Discard all changes in ${path}? This cannot be undone.`)) return;
    try {
      await ApiBridge.gitDiscard(path, cwd);
      await refreshGitStatus();
      showToast(`Discarded ${path}`, 'info');
    } catch (err) {
      showToast(err instanceof Error ? err.message : 'Discard failed', 'error');
    }
  };

  // Commit action
  const handleCommit = async () => {
    if (!commitSummary.trim()) {
      showToast('Please enter a commit summary', 'warn');
      return;
    }
    if (stagedFiles.length === 0) {
      showToast('No staged changes to commit', 'warn');
      return;
    }
    setIsCommitting(true);
    try {
      const fullMessage = commitDesc.trim()
        ? `${commitSummary.trim()}\n\n${commitDesc.trim()}`
        : commitSummary.trim();
      const res = await ApiBridge.gitCommit(fullMessage, cwd);
      if (res?.success) {
        showToast('Committed successfully!', 'info');
        setCommitSummary('');
        setCommitDesc('');
        await refreshGitStatus();
        void refreshGitLog();
      } else {
        showToast(res?.output || 'Commit failed', 'error');
      }
    } catch (err: any) {
      showToast(err?.message || 'Commit failed', 'error');
    } finally {
      setIsCommitting(false);
    }
  };

  // Generate AI commit message
  const handleGenerateAiMessage = async () => {
    setIsAiGenerating(true);
    try {
      const res = await ApiBridge.gitAiCommitMessage(cwd);
      if (res?.message) {
        const parts = res.message.split('\n\n');
        setCommitSummary(parts[0] || res.message);
        setCommitDesc(parts.slice(1).join('\n\n') || '');
        showToast('AI commit message generated', 'info');
      } else {
        showToast('Could not generate commit message', 'warn');
      }
    } catch (err: any) {
      showToast(err?.message || 'AI message failed', 'error');
    } finally {
      setIsAiGenerating(false);
    }
  };

  // Remote operations
  const handleFetch = async () => {
    setIsFetching(true);
    try {
      await ApiBridge.gitFetch(cwd);
      showToast('Fetched origin', 'info');
      await refreshGitStatus();
      void refreshGitLog();
    } catch (err: any) {
      showToast(err?.message || 'Fetch failed', 'error');
    } finally {
      setIsFetching(false);
    }
  };

  const handlePull = async () => {
    setIsPulling(true);
    try {
      const res = await ApiBridge.gitPull(cwd);
      showToast(res?.output || 'Pulled from remote', 'info');
      await refreshGitStatus();
      void refreshGitLog();
    } catch (err: any) {
      showToast(err?.message || 'Pull failed', 'error');
    } finally {
      setIsPulling(false);
    }
  };

  const handlePush = async () => {
    setIsPushing(true);
    try {
      const res = await ApiBridge.gitPush(gitBranch || 'main', cwd);
      showToast(res?.output || 'Pushed to remote', 'info');
      await refreshGitStatus();
      void refreshGitLog();
    } catch (err: any) {
      showToast(err?.message || 'Push failed', 'error');
    } finally {
      setIsPushing(false);
    }
  };

  const handleRefresh = async () => {
    setIsRefreshing(true);
    try {
      await refreshGitStatus();
      await refreshGitLog();
      if (activeTab === 'history') await loadHistoryGraph();
      showToast('Git status refreshed', 'info');
    } finally {
      setIsRefreshing(false);
    }
  };

  // Switch or create branch
  const handleCheckoutBranch = async (branch: string) => {
    if (branch === gitBranch) return;
    setIsSwitchingBranch(true);
    try {
      await ApiBridge.gitCheckout(branch, cwd);
      setBranchMenuOpen(false);
      showToast(`Switched to branch ${branch}`, 'info');
      await refreshGitStatus();
      void refreshGitLog();
    } catch (err: any) {
      showToast(err?.message || 'Switch branch failed', 'error');
    } finally {
      setIsSwitchingBranch(false);
    }
  };

  const handleCreateBranch = async (e: React.FormEvent) => {
    e.preventDefault();
    const name = newBranchName.trim();
    if (!name) return;
    try {
      await ApiBridge.gitCreateBranch(cwd, name);
      setNewBranchName('');
      setIsCreatingBranch(false);
      setBranchMenuOpen(false);
      showToast(`Created branch ${name}`, 'info');
      await refreshGitStatus();
      void refreshGitLog();
    } catch (err: any) {
      showToast(err?.message || 'Create branch failed', 'error');
    }
  };

  return (
    <div className={`flex flex-col h-full w-full bg-background select-none overflow-hidden text-foreground ${className}`}>
      {/* ── Top Header Toolbar (GitHub Desktop / SourceTree Style) ── */}
      <div className="flex h-12 items-center justify-between px-4 border-b border-border bg-surface shrink-0 z-10">
        <div className="flex items-center gap-3">
          {/* Branch Switcher Dropdown */}
          <div className="relative" ref={branchMenuRef}>
            <button
              type="button"
              onClick={() => {
                setBranchMenuOpen(v => !v);
                setBranchSearch('');
              }}
              className="flex items-center gap-2 px-2.5 py-1.5 rounded-lg border border-border bg-background hover:bg-surface-hover text-ui-sm font-medium transition-colors shadow-xs"
              title="Current Branch"
            >
              <GitBranch className="size-4 text-primary shrink-0" />
              <span className="font-mono text-ui-sm max-w-40 truncate">{gitBranch || 'main'}</span>
              <ChevronDown className="size-3.5 text-foreground-subtle ml-1" />
            </button>

            {branchMenuOpen && (
              <div className="absolute left-0 top-full mt-1.5 w-72 rounded-xl border border-border bg-popover p-2 shadow-xl z-50 animate-in fade-in zoom-in-95 duration-100">
                <div className="flex items-center gap-2 px-2.5 py-1.5 rounded-lg border border-border bg-background mb-2">
                  <Search className="size-3.5 text-foreground-subtle shrink-0" />
                  <input
                    type="text"
                    value={branchSearch}
                    onChange={e => setBranchSearch(e.target.value)}
                    placeholder="Filter branches…"
                    className="w-full bg-transparent text-ui-xs font-mono outline-none text-foreground placeholder-foreground-subtle"
                    autoFocus
                  />
                </div>

                {isCreatingBranch ? (
                  <form onSubmit={handleCreateBranch} className="p-1 mb-2">
                    <input
                      type="text"
                      value={newBranchName}
                      onChange={e => setNewBranchName(e.target.value)}
                      placeholder="New branch name…"
                      className="w-full px-2.5 py-1.5 rounded-lg border border-border bg-background text-ui-xs font-mono outline-none text-foreground mb-2"
                      autoFocus
                    />
                    <div className="flex justify-end gap-1.5">
                      <button
                        type="button"
                        onClick={() => setIsCreatingBranch(false)}
                        className="px-2.5 py-1 text-ui-xs rounded hover:bg-surface-hover text-foreground-subtle"
                      >
                        Cancel
                      </button>
                      <button
                        type="submit"
                        disabled={!newBranchName.trim()}
                        className="px-2.5 py-1 text-ui-xs rounded bg-primary text-primary-foreground font-medium disabled:opacity-50"
                      >
                        Create
                      </button>
                    </div>
                  </form>
                ) : (
                  <button
                    type="button"
                    onClick={() => setIsCreatingBranch(true)}
                    className="flex w-full items-center gap-2 px-2.5 py-1.5 mb-1 text-ui-xs rounded-lg hover:bg-surface-hover text-primary font-medium transition-colors"
                  >
                    <Plus className="size-3.5" />
                    Create New Branch…
                  </button>
                )}

                <div className="max-h-48 overflow-y-auto space-y-0.5 pt-1 border-t border-border/50">
                  {branches
                    .filter(b => b.toLowerCase().includes(branchSearch.toLowerCase()))
                    .map(b => {
                      const isCurrent = b === gitBranch;
                      return (
                        <button
                          key={b}
                          type="button"
                          onClick={() => handleCheckoutBranch(b)}
                          className={`flex w-full items-center justify-between px-2.5 py-1.5 rounded-md text-ui-xs font-mono text-left transition-colors ${
                            isCurrent
                              ? 'bg-primary/10 text-primary font-semibold'
                              : 'hover:bg-surface-hover text-foreground'
                          }`}
                        >
                          <span className="truncate">{b}</span>
                          {isCurrent && <Check className="size-3.5 shrink-0 ml-1" />}
                        </button>
                      );
                    })}
                </div>
              </div>
            )}
          </div>

          {/* Mode Switcher Tabs (Changes vs History) */}
          <div className="flex items-center rounded-lg bg-surface-hover/70 p-0.5 border border-border/60">
            <button
              type="button"
              onClick={() => setActiveTab('changes')}
              className={`flex items-center gap-1.5 px-3 py-1 rounded-md text-ui-xs font-medium transition-all ${
                activeTab === 'changes'
                  ? 'bg-background text-foreground shadow-xs'
                  : 'text-foreground-subtle hover:text-foreground'
              }`}
            >
              <FileDiffIcon className="size-3.5" />
              <span>Changes</span>
              {gitFiles.length > 0 && (
                <span className="ml-1 px-1.5 py-0.2 rounded-full text-[10px] font-bold bg-primary/20 text-primary">
                  {gitFiles.length}
                </span>
              )}
            </button>
            <button
              type="button"
              onClick={() => setActiveTab('history')}
              className={`flex items-center gap-1.5 px-3 py-1 rounded-md text-ui-xs font-medium transition-all ${
                activeTab === 'history'
                  ? 'bg-background text-foreground shadow-xs'
                  : 'text-foreground-subtle hover:text-foreground'
              }`}
            >
              <History className="size-3.5" />
              <span>History</span>
            </button>
          </div>
        </div>

        {/* Remote Sync Buttons: Fetch, Pull, Push, Refresh */}
        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={handleFetch}
            disabled={isFetching}
            className="flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg border border-border bg-background hover:bg-surface-hover text-ui-xs font-medium transition-colors disabled:opacity-50"
            title="Fetch latest from remote"
          >
            <RefreshCw className={`size-3.5 ${isFetching ? 'animate-spin text-primary' : 'text-foreground-subtle'}`} />
            <span>Fetch Origin</span>
          </button>

          <button
            type="button"
            onClick={handlePull}
            disabled={isPulling}
            className="flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg border border-border bg-background hover:bg-surface-hover text-ui-xs font-medium transition-colors disabled:opacity-50"
            title="Pull changes from remote"
          >
            <Download className={`size-3.5 ${isPulling ? 'animate-bounce text-primary' : 'text-foreground-subtle'}`} />
            <span>Pull</span>
          </button>

          <button
            type="button"
            onClick={handlePush}
            disabled={isPushing}
            className="flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg border border-border bg-background hover:bg-surface-hover text-ui-xs font-medium transition-colors disabled:opacity-50"
            title="Push commits to remote"
          >
            <Upload className={`size-3.5 ${isPushing ? 'animate-bounce text-primary' : 'text-foreground-subtle'}`} />
            <span>Push</span>
          </button>

          <div className="h-4 w-px bg-border mx-1" />

          <button
            type="button"
            onClick={handleRefresh}
            disabled={isRefreshing}
            className="p-1.5 rounded-lg text-foreground-subtle hover:text-foreground hover:bg-surface-hover transition-colors"
            title="Refresh Git status"
          >
            <RotateCcw className={`size-4 ${isRefreshing ? 'animate-spin text-primary' : ''}`} />
          </button>

          {onClose && (
            <button
              type="button"
              onClick={onClose}
              className="p-1.5 rounded-lg text-foreground-subtle hover:text-foreground hover:bg-surface-hover transition-colors"
              title="Close Git client"
            >
              <X className="size-4" />
            </button>
          )}
        </div>
      </div>

      {/* ── Main View Content ── */}
      {activeTab === 'changes' ? (
        <GitAccordionDiffList
          cwd={cwd}
          gitBranch={gitBranch || 'main'}
          gitFiles={gitFiles}
          onRefresh={handleRefresh}
          onStageFile={handleStageFile}
          onUnstageFile={handleUnstageFile}
          onDiscardFile={handleDiscardFile}
          onStageAll={handleStageAll}
          onUnstageAll={handleUnstageAll}
          onCommit={async (msg) => {
            setIsCommitting(true);
            try {
              const res = await ApiBridge.gitCommit(msg, cwd);
              if (res?.success) {
                showToast('Committed successfully!', 'info');
                await refreshGitStatus();
              } else {
                showToast(res?.output || 'Commit failed', 'error');
              }
            } catch (err: any) {
              showToast(err?.message || 'Commit failed', 'error');
            } finally {
              setIsCommitting(false);
            }
          }}
          isCommitting={isCommitting}
        />
      ) : (
        /* ═══════════════════════════════════════════════
           HISTORY VIEW: SourceTree Commit Graph & Log
           ═══════════════════════════════════════════════ */
        <div ref={historyWorkAreaRef} className={`flex flex-1 overflow-hidden ${activeResizer ? 'select-none cursor-col-resize' : ''}`}>
          {/* Left Column: Commit Graph List */}
          <div style={{ width: historyLogWidth }} className="border-r border-border flex flex-col bg-surface/30 shrink-0 overflow-hidden">
            {/* Search Commits */}
            <div className="p-2 border-b border-border/60">
              <div className="flex items-center gap-2 px-2.5 py-1 rounded-lg border border-border bg-background">
                <Search className="size-3.5 text-foreground-subtle shrink-0" />
                <input
                  type="text"
                  value={historySearch}
                  onChange={e => setHistorySearch(e.target.value)}
                  placeholder="Search commits by message, author, SHA…"
                  className="w-full bg-transparent text-ui-xs outline-none text-foreground placeholder-foreground-subtle"
                />
              </div>
            </div>

            {/* Commits List */}
            <div className="flex-1 overflow-y-auto">
              {historyLoading ? (
                <div className="flex items-center justify-center h-48 text-foreground-subtle">
                  <RefreshCw className="size-5 animate-spin mr-2" />
                  <span>Loading commit history…</span>
                </div>
              ) : historyCommits.length === 0 ? (
                <div className="p-6 text-center text-foreground-subtle text-ui-xs">
                  No commits found in this repository.
                </div>
              ) : (
                <div className="divide-y divide-border/40">
                  {historyCommits
                    .filter(c => {
                      if (!historySearch.trim()) return true;
                      const q = historySearch.toLowerCase();
                      return (
                        c.message.toLowerCase().includes(q) ||
                        c.author_name.toLowerCase().includes(q) ||
                        c.hash.toLowerCase().includes(q)
                      );
                    })
                    .map(commit => {
                      const isSelected = selectedCommit?.hash === commit.hash;
                      return (
                        <div
                          key={commit.hash}
                          onClick={() => selectCommitItem(commit)}
                          className={`flex items-start gap-2.5 px-3 py-2 cursor-pointer transition-colors ${
                            isSelected
                              ? 'bg-primary/15 text-primary'
                              : 'hover:bg-surface-hover text-foreground'
                          }`}
                        >
                          {/* Graph lane indicator */}
                          <div className="shrink-0 pt-0.5">
                            <GitGraphLane prefix={commit.graph_prefix} />
                          </div>

                          <div className="flex-1 min-w-0">
                            <div className="flex items-center justify-between gap-1 mb-0.5">
                              <span className="font-medium text-ui-xs truncate text-foreground">
                                {commit.message.split('\n')[0]}
                              </span>
                              <span className="font-mono text-[10px] text-foreground-subtle shrink-0">
                                {commit.short_hash || commit.hash.substring(0, 7)}
                              </span>
                            </div>

                            <div className="flex items-center justify-between text-[11px] text-foreground-subtle">
                              <span className="truncate max-w-40">{commit.author_name}</span>
                              <span>{timeAgo(commit.timestamp)}</span>
                            </div>
                          </div>
                        </div>
                      );
                    })}
                </div>
              )}
            </div>
          </div>

          {/* Drag Resizer Handle between Commit Graph and Commit Details */}
          <div
            onMouseDown={e => {
              e.preventDefault();
              setActiveResizer('history-log');
            }}
            className={`w-1 relative shrink-0 cursor-col-resize hover:bg-primary/50 transition-colors z-10 ${
              activeResizer === 'history-log' ? 'bg-primary' : 'bg-border/60 hover:bg-primary/40'
            }`}
            title="Drag to resize commit list"
          />

          {/* Right Column: Commit Details & Changed Files */}
          <div className="flex-1 flex flex-col overflow-hidden bg-background">
            {selectedCommit ? (
              <GitCommitDiffView
                commit={selectedCommit}
                files={commitDiffFiles}
                rawDiff={rawCommitDiff}
                isLoading={loadingCommitDetails}
              />
            ) : (
              <div className="flex-1 flex flex-col items-center justify-center text-foreground-subtle p-6 text-center">
                <History className="size-12 text-border mb-3" />
                <span className="text-ui-sm font-medium text-foreground">Select a commit</span>
                <span className="text-ui-xs text-foreground-subtle mt-1 max-w-sm">
                  Click on any commit node from the log graph on the left to inspect its commit message, author info, and code changes.
                </span>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
};
