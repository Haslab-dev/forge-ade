import React, { useState, useEffect, useMemo, useRef } from 'react';
import {
  GitPullRequest,
  GitMerge,
  GitBranch,
  Search,
  RefreshCw,
  Plus,
  MessageSquare,
  Clock,
  User,
  ExternalLink,
  CheckCircle2,
  XCircle,
  AlertCircle,
  ChevronRight,
  FileCode,
  FileText,
  CornerDownRight,
  ShieldCheck,
  Check,
  Send,
  Eye,
  Columns,
  Layers,
  ArrowRight,
  X,
  Sparkles,
  ChevronsUpDown
} from 'lucide-react';
import { ApiBridge } from '../../services/apiBridge';
import { showToast } from '../../lib/toast';
import {
  PullRequestItem,
  PullRequestDetail,
  PullRequestFile,
  PullRequestComment,
  PullRequestReview
} from '../../features/git/git.api';
import { GitDiffPane } from './GitDiffPane';
import { timeAgo, isBinaryFile } from './gitDiffUtils';
import { HistoryFileCard, CommitDiffFileItem } from './GitCommitDiffView';
import { parseUnifiedDiffByFiles } from '../diff/DiffViewer';

interface GitPullRequestsPaneProps {
  cwd: string;
  currentBranch?: string;
  onRefreshParent?: () => Promise<void>;
  className?: string;
}

export const GitPullRequestsPane: React.FC<GitPullRequestsPaneProps> = ({
  cwd,
  currentBranch = 'main',
  onRefreshParent,
  className = ''
}) => {
  // State
  const [prs, setPrs] = useState<PullRequestItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [stateFilter, setStateFilter] = useState<'all' | 'open' | 'closed' | 'merged'>('open');
  const [searchQuery, setSearchQuery] = useState('');

  // Selected PR state
  const [selectedPrNumber, setSelectedPrNumber] = useState<number | null>(null);
  const [prDetail, setPrDetail] = useState<PullRequestDetail | null>(null);
  const [prDiff, setPrDiff] = useState<string>('');
  const [loadingDetail, setLoadingDetail] = useState(false);
  const [detailTab, setDetailTab] = useState<'conversation' | 'files'>('conversation');
  const [selectedFileInPr, setSelectedFileInPr] = useState<string | null>(null);
  const [fileFilterQuery, setFileFilterQuery] = useState('');
  const [expandedFiles, setExpandedFiles] = useState<Record<string, boolean>>({});

  // Actions state
  const [commentText, setCommentText] = useState('');
  const [submittingComment, setSubmittingComment] = useState(false);
  const [reviewAction, setReviewAction] = useState<'approve' | 'request-changes' | 'comment' | null>(null);
  const [isCheckingOut, setIsCheckingOut] = useState(false);
  const [isMerging, setIsMerging] = useState(false);
  const [isClosingReopening, setIsClosingReopening] = useState(false);

  // Modals state
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [newPrTitle, setNewPrTitle] = useState('');
  const [newPrBody, setNewPrBody] = useState('');
  const [newPrBase, setNewPrBase] = useState('main');
  const [newPrHead, setNewPrHead] = useState(currentBranch || 'main');
  const [newPrDraft, setNewPrDraft] = useState(false);
  const [isCreatingPr, setIsCreatingPr] = useState(false);

  // Merge modal
  const [showMergeModal, setShowMergeModal] = useState(false);
  const [mergeMethod, setMergeMethod] = useState<'merge' | 'squash' | 'rebase'>('merge');

  // Resizable left column
  const [listWidth, setListWidth] = useState(290);
  const [isResizing, setIsResizing] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  // Load PR list
  const loadPullRequests = async () => {
    if (!cwd) return;
    setLoading(true);
    setError(null);
    try {
      const list = await ApiBridge.getPullRequests(cwd, stateFilter);
      setPrs(Array.isArray(list) ? list : []);
      // If we don't have a selected PR or current selected PR is not in list, pick the first
      if (list && list.length > 0) {
        if (!selectedPrNumber || !list.some(p => p.number === selectedPrNumber)) {
          setSelectedPrNumber(list[0].number);
        }
      } else {
        setSelectedPrNumber(null);
        setPrDetail(null);
        setPrDiff('');
      }
    } catch (err: any) {
      console.error('Failed to load pull requests:', err);
      setError(err?.message || 'Failed to load pull requests. Ensure GitHub CLI (gh) is authenticated.');
      setPrs([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadPullRequests();
  }, [cwd, stateFilter]);

  // Load PR detail and diff when selectedPrNumber changes
  const loadPrDetail = async (prNum: number) => {
    if (!cwd || !prNum) return;
    setLoadingDetail(true);
    try {
      const [detail, diff] = await Promise.all([
        ApiBridge.getPullRequestDetail(prNum, cwd),
        ApiBridge.getPullRequestDiff(prNum, cwd).catch(() => '')
      ]);
      setPrDetail(detail);
      setPrDiff(diff || '');
      setSelectedFileInPr(null);
    } catch (err: any) {
      console.warn('Failed to fetch PR detail:', err);
      showToast(err?.message || 'Failed to load PR details', 'error');
    } finally {
      setLoadingDetail(false);
    }
  };

  useEffect(() => {
    if (selectedPrNumber) {
      loadPrDetail(selectedPrNumber);
    }
  }, [selectedPrNumber, cwd]);

  // Filtered PR list
  const filteredPrs = useMemo(() => {
    if (!searchQuery.trim()) return prs;
    const q = searchQuery.toLowerCase();
    return prs.filter(
      p =>
        p.number.toString().includes(q) ||
        p.title.toLowerCase().includes(q) ||
        p.author?.login?.toLowerCase().includes(q) ||
        p.headRefName?.toLowerCase().includes(q) ||
        p.baseRefName?.toLowerCase().includes(q)
    );
  }, [prs, searchQuery]);

  // Counts by state
  const openCount = useMemo(() => prs.filter(p => p.state === 'OPEN').length, [prs]);

  // Resize handler
  useEffect(() => {
    if (!isResizing) return;
    const onMouseMove = (e: MouseEvent) => {
      if (!containerRef.current) return;
      const rect = containerRef.current.getBoundingClientRect();
      const newWidth = Math.max(280, Math.min(rect.width - 320, e.clientX - rect.left));
      setListWidth(newWidth);
    };
    const onMouseUp = () => setIsResizing(false);
    document.addEventListener('mousemove', onMouseMove);
    document.addEventListener('mouseup', onMouseUp);
    return () => {
      document.removeEventListener('mousemove', onMouseMove);
      document.removeEventListener('mouseup', onMouseUp);
    };
  }, [isResizing]);

  // Action: Add Comment
  const handleAddComment = async () => {
    if (!selectedPrNumber || !commentText.trim()) return;
    setSubmittingComment(true);
    try {
      await ApiBridge.createPullRequestComment(selectedPrNumber, commentText.trim(), cwd);
      setCommentText('');
      showToast('Comment submitted', 'info');
      await loadPrDetail(selectedPrNumber);
    } catch (err: any) {
      showToast(err?.message || 'Failed to post comment', 'error');
    } finally {
      setSubmittingComment(false);
    }
  };

  // Action: Review (Approve / Request Changes / Comment)
  const handleSubmitReview = async (action: 'approve' | 'request-changes' | 'comment') => {
    if (!selectedPrNumber) return;
    setReviewAction(action);
    try {
      await ApiBridge.reviewPullRequest(selectedPrNumber, action, commentText.trim(), cwd);
      setCommentText('');
      showToast(
        action === 'approve'
          ? 'Pull request approved'
          : action === 'request-changes'
          ? 'Changes requested'
          : 'Review submitted',
        'info'
      );
      await loadPrDetail(selectedPrNumber);
      await loadPullRequests();
    } catch (err: any) {
      showToast(err?.message || 'Review submission failed', 'error');
    } finally {
      setReviewAction(null);
    }
  };

  // Action: Checkout PR branch
  const handleCheckoutBranch = async () => {
    if (!selectedPrNumber) return;
    setIsCheckingOut(true);
    try {
      const output = await ApiBridge.checkoutPullRequest(selectedPrNumber, cwd);
      showToast(output ? `Checked out: ${output}` : `Checked out PR #${selectedPrNumber}`, 'info');
      if (onRefreshParent) await onRefreshParent();
    } catch (err: any) {
      showToast(err?.message || 'Checkout failed', 'error');
    } finally {
      setIsCheckingOut(false);
    }
  };

  // Action: Merge PR
  const handleMergePr = async () => {
    if (!selectedPrNumber) return;
    setIsMerging(true);
    try {
      await ApiBridge.mergePullRequest(selectedPrNumber, mergeMethod, cwd);
      setShowMergeModal(false);
      showToast(`Merged PR #${selectedPrNumber} via ${mergeMethod}`, 'info');
      await loadPrDetail(selectedPrNumber);
      await loadPullRequests();
      if (onRefreshParent) await onRefreshParent();
    } catch (err: any) {
      showToast(err?.message || 'Merge failed', 'error');
    } finally {
      setIsMerging(false);
    }
  };

  // Action: Close / Reopen PR
  const handleToggleCloseReopen = async () => {
    if (!selectedPrNumber || !prDetail) return;
    setIsClosingReopening(true);
    try {
      if (prDetail.state === 'OPEN') {
        await ApiBridge.closePullRequest(selectedPrNumber, cwd);
        showToast(`Closed PR #${selectedPrNumber}`, 'info');
      } else {
        await ApiBridge.reopenPullRequest(selectedPrNumber, cwd);
        showToast(`Reopened PR #${selectedPrNumber}`, 'info');
      }
      await loadPrDetail(selectedPrNumber);
      await loadPullRequests();
    } catch (err: any) {
      showToast(err?.message || 'Action failed', 'error');
    } finally {
      setIsClosingReopening(false);
    }
  };

  // Action: Create PR
  const handleCreatePrSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newPrTitle.trim()) {
      showToast('Title is required', 'error');
      return;
    }
    setIsCreatingPr(true);
    try {
      const created = await ApiBridge.createPullRequest(
        {
          title: newPrTitle.trim(),
          body: newPrBody.trim(),
          base: newPrBase.trim() || undefined,
          head: newPrHead.trim() || undefined,
          draft: newPrDraft
        },
        cwd
      );
      showToast(`Pull request created successfully!`, 'info');
      setShowCreateModal(false);
      setNewPrTitle('');
      setNewPrBody('');
      await loadPullRequests();
      if (created?.number) {
        setSelectedPrNumber(created.number);
      }
    } catch (err: any) {
      showToast(err?.message || 'Failed to create pull request', 'error');
    } finally {
      setIsCreatingPr(false);
    }
  };

  // Open external URL
  const handleOpenUrl = (url: string) => {
    if (!url) return;
    window.open(url, '_blank');
  };

  // Parse unified diff by files and merge with prDetail.files metadata (History diff style)
  const prDiffFiles = useMemo<CommitDiffFileItem[]>(() => {
    const parsedSections = parseUnifiedDiffByFiles(prDiff || '');
    const sectionMap = new Map<string, { additions: number; deletions: number; diff: string }>();

    for (const s of parsedSections) {
      if (s.filePath && s.filePath !== 'Diff') {
        sectionMap.set(s.filePath, {
          additions: s.additions,
          deletions: s.deletions,
          diff: s.rawText || ''
        });
      }
    }

    const items: CommitDiffFileItem[] = [];

    // If prDetail?.files exists, use it as canonical file list to ensure order and metadata
    if (prDetail?.files && prDetail.files.length > 0) {
      for (const f of prDetail.files) {
        const parsed = sectionMap.get(f.path);
        items.push({
          path: f.path,
          additions: parsed ? parsed.additions : (f.additions || 0),
          deletions: parsed ? parsed.deletions : (f.deletions || 0),
          diff: parsed ? parsed.diff : ''
        });
        sectionMap.delete(f.path);
      }
    }

    // Add any remaining parsed sections that weren't in prDetail.files
    for (const [path, data] of sectionMap.entries()) {
      items.push({
        path,
        additions: data.additions,
        deletions: data.deletions,
        diff: data.diff
      });
    }

    return items;
  }, [prDiff, prDetail?.files]);

  // Filtered files by search query
  const filteredPrFiles = useMemo(() => {
    if (!fileFilterQuery.trim()) return prDiffFiles;
    const q = fileFilterQuery.toLowerCase();
    return prDiffFiles.filter(f => f.path.toLowerCase().includes(q));
  }, [prDiffFiles, fileFilterQuery]);

  // Initial expand logic: expand first 2 non-binary files when PR changes
  useEffect(() => {
    if (prDiffFiles.length > 0) {
      const initial: Record<string, boolean> = {};
      const limit = prDiffFiles.length <= 3 ? prDiffFiles.length : 2;
      for (let i = 0; i < limit; i++) {
        if (!isBinaryFile(prDiffFiles[i].path)) {
          initial[prDiffFiles[i].path] = true;
        }
      }
      setExpandedFiles(initial);
    }
  }, [selectedPrNumber, prDiffFiles.length]);

  const handleToggleFile = (path: string) => {
    setExpandedFiles(prev => ({ ...prev, [path]: !prev[path] }));
  };

  const allFilesExpanded = filteredPrFiles.length > 0 && filteredPrFiles.every(f => expandedFiles[f.path]);

  const handleExpandAllFiles = () => {
    const next: Record<string, boolean> = {};
    for (const f of filteredPrFiles) {
      next[f.path] = true;
    }
    setExpandedFiles(next);
  };

  const handleCollapseAllFiles = () => {
    setExpandedFiles({});
  };

  // Render markdown text lines
  const renderMarkdownText = (text: string) => {
    if (!text) return <p className="text-foreground-subtle italic">No description provided.</p>;
    const lines = text.split('\n');
    return (
      <div className="space-y-2 text-ui-sm text-foreground leading-relaxed">
        {lines.map((line, idx) => {
          if (line.startsWith('### ')) {
            return (
              <h4 key={idx} className="text-ui-base font-bold text-foreground mt-3 mb-1">
                {line.slice(4)}
              </h4>
            );
          }
          if (line.startsWith('## ')) {
            return (
              <h3 key={idx} className="text-ui-lg font-bold text-foreground mt-4 mb-2">
                {line.slice(3)}
              </h3>
            );
          }
          if (line.startsWith('# ')) {
            return (
              <h2 key={idx} className="text-ui-xl font-bold text-foreground mt-4 mb-2">
                {line.slice(2)}
              </h2>
            );
          }
          if (line.startsWith('- ') || line.startsWith('* ')) {
            return (
              <li key={idx} className="ml-4 list-disc text-foreground">
                {line.slice(2)}
              </li>
            );
          }
          if (line.startsWith('> ')) {
            return (
              <blockquote
                key={idx}
                className="pl-3 border-l-2 border-primary/40 text-foreground-subtle italic my-1"
              >
                {line.slice(2)}
              </blockquote>
            );
          }
          if (line.trim() === '') {
            return <div key={idx} className="h-1" />;
          }
          return <p key={idx}>{line}</p>;
        })}
      </div>
    );
  };

  return (
    <div ref={containerRef} className={`flex h-full w-full overflow-hidden bg-background ${className}`}>
      {/* ── Left Column: Pull Requests List ── */}
      <div
        style={{ width: `${listWidth}px` }}
        className="flex flex-col h-full border-r border-border bg-surface shrink-0 select-none overflow-hidden"
      >
        {/* Header & New PR Button */}
        <div className="flex items-center justify-between px-3.5 py-2.5 border-b border-border bg-surface shrink-0">
          <div className="flex items-center gap-2">
            <GitPullRequest className="size-4 text-primary shrink-0" />
            <span className="text-ui-sm font-semibold text-foreground tracking-tight">Pull Requests</span>
            {openCount > 0 && (
              <span className="px-1.5 py-0.2 rounded-full text-[10px] font-bold bg-primary/20 text-primary">
                {openCount}
              </span>
            )}
          </div>
          <div className="flex items-center gap-1.5">
            <button
              type="button"
              onClick={loadPullRequests}
              disabled={loading}
              className="p-1.5 rounded-lg border border-border bg-background hover:bg-surface-hover text-foreground-subtle hover:text-foreground transition-colors disabled:opacity-50"
              title="Refresh pull requests"
            >
              <RefreshCw className={`size-3.5 ${loading ? 'animate-spin text-primary' : ''}`} />
            </button>
            <button
              type="button"
              onClick={() => {
                setNewPrHead(currentBranch || 'main');
                setShowCreateModal(true);
              }}
              className="flex items-center gap-1 px-2.5 py-1.2 rounded-lg bg-primary hover:bg-primary-hover text-primary-foreground text-ui-xs font-semibold shadow-xs transition-colors"
              title="Create new pull request"
            >
              <Plus className="size-3.5" />
              <span>New PR</span>
            </button>
          </div>
        </div>

        {/* State Filter Chips */}
        <div className="flex items-center gap-1 px-3 py-2 border-b border-border bg-surface shrink-0 overflow-x-auto no-scrollbar">
          {(['open', 'merged', 'closed', 'all'] as const).map(state => {
            const isActive = stateFilter === state;
            return (
              <button
                key={state}
                type="button"
                onClick={() => setStateFilter(state)}
                className={`px-2.5 py-1 rounded-md text-ui-xs font-medium capitalize transition-all shrink-0 ${
                  isActive
                    ? 'bg-foreground text-background font-semibold shadow-xs'
                    : 'text-foreground-subtle hover:text-foreground hover:bg-surface-hover'
                }`}
              >
                {state}
              </button>
            );
          })}
        </div>

        {/* Search input */}
        <div className="px-3 py-2 border-b border-border bg-surface shrink-0">
          <div className="flex items-center gap-2 px-2.5 py-1.5 rounded-lg border border-border bg-background focus-within:border-primary transition-colors">
            <Search className="size-3.5 text-foreground-subtle shrink-0" />
            <input
              type="text"
              value={searchQuery}
              onChange={e => setSearchQuery(e.target.value)}
              placeholder="Search by #number, title, author..."
              className="w-full bg-transparent text-ui-xs outline-none text-foreground placeholder-foreground-subtle"
            />
            {searchQuery && (
              <button
                type="button"
                onClick={() => setSearchQuery('')}
                className="text-foreground-subtle hover:text-foreground"
              >
                <X className="size-3" />
              </button>
            )}
          </div>
        </div>

        {/* List Content */}
        <div className="flex-1 overflow-y-auto min-h-0 divide-y divide-border/50">
          {error && (
            <div className="p-4 m-3 rounded-xl border border-destructive/30 bg-destructive/10 text-destructive text-ui-xs space-y-2">
              <div className="flex items-center gap-2 font-semibold">
                <AlertCircle className="size-4 shrink-0" />
                <span>CLI Notice</span>
              </div>
              <p className="leading-relaxed opacity-90">{error}</p>
              <button
                type="button"
                onClick={loadPullRequests}
                className="mt-2 px-2.5 py-1 rounded bg-destructive/20 hover:bg-destructive/30 text-destructive font-medium transition-colors"
              >
                Retry
              </button>
            </div>
          )}

          {loading && prs.length === 0 && (
            <div className="flex flex-col items-center justify-center p-8 text-foreground-subtle space-y-2">
              <RefreshCw className="size-5 animate-spin text-primary" />
              <span className="text-ui-xs">Loading pull requests…</span>
            </div>
          )}

          {!loading && !error && filteredPrs.length === 0 && (
            <div className="flex flex-col items-center justify-center p-8 text-center text-foreground-subtle space-y-2">
              <GitPullRequest className="size-8 opacity-30 text-foreground" />
              <p className="text-ui-sm font-medium text-foreground">No pull requests found</p>
              <p className="text-ui-xs text-foreground-subtle max-w-xs">
                {searchQuery
                  ? `No pull requests matching "${searchQuery}"`
                  : `There are no ${stateFilter} pull requests in this repository.`}
              </p>
            </div>
          )}

          {filteredPrs.map(pr => {
            const isSelected = selectedPrNumber === pr.number;
            const isMerged = pr.state === 'MERGED';
            const isClosed = pr.state === 'CLOSED';
            const isOpen = pr.state === 'OPEN';

            return (
              <div
                key={pr.number}
                onClick={() => setSelectedPrNumber(pr.number)}
                className={`flex flex-col p-3 cursor-pointer transition-colors border-l-2 ${
                  isSelected
                    ? 'bg-surface-hover border-primary'
                    : 'hover:bg-surface-hover/60 border-transparent'
                }`}
              >
                {/* Status + Number + Title */}
                <div className="flex items-start gap-2">
                  <div className="mt-0.5 shrink-0">
                    {isMerged ? (
                      <span className="flex items-center justify-center size-4 rounded-full bg-purple-500/15 text-purple-400">
                        <GitMerge className="size-3" />
                      </span>
                    ) : isClosed ? (
                      <span className="flex items-center justify-center size-4 rounded-full bg-rose-500/15 text-rose-400">
                        <XCircle className="size-3" />
                      </span>
                    ) : (
                      <span className="flex items-center justify-center size-4 rounded-full bg-emerald-500/15 text-emerald-400">
                        <GitPullRequest className="size-3" />
                      </span>
                    )}
                  </div>

                  <div className="flex-1 min-w-0">
                    <div className="flex items-center justify-between gap-2">
                      <span className="text-ui-xs font-mono text-foreground-subtle shrink-0">
                        #{pr.number}
                      </span>
                      <span className="text-[10px] text-foreground-subtle shrink-0">
                        {timeAgo(pr.updatedAt || pr.createdAt)}
                      </span>
                    </div>
                    <h4 className="text-ui-sm font-semibold text-foreground line-clamp-2 mt-0.5 leading-snug">
                      {pr.title}
                    </h4>
                  </div>
                </div>

                {/* Branches & Author */}
                <div className="flex items-center justify-between text-ui-xs text-foreground-subtle mt-2 pt-1 border-t border-border/30">
                  <div className="flex items-center gap-1 font-mono text-[11px] truncate max-w-[200px]" title={`${pr.headRefName} → ${pr.baseRefName}`}>
                    <GitBranch className="size-3 shrink-0 text-foreground-subtle" />
                    <span className="text-foreground truncate">{pr.headRefName}</span>
                    <ArrowRight className="size-2.5 shrink-0 opacity-40" />
                    <span className="opacity-70 truncate">{pr.baseRefName}</span>
                  </div>

                  <div className="flex items-center gap-2 shrink-0">
                    {pr.commentsCount > 0 && (
                      <span className="flex items-center gap-1 text-[11px] text-foreground-subtle">
                        <MessageSquare className="size-3" />
                        <span>{pr.commentsCount}</span>
                      </span>
                    )}
                    {pr.author?.login && (
                      <span className="flex items-center gap-1 text-[11px] font-medium text-foreground-subtle">
                        <span className="size-4 rounded-full bg-primary/20 text-primary flex items-center justify-center text-[9px] font-bold uppercase">
                          {pr.author.login.charAt(0)}
                        </span>
                        <span className="truncate max-w-[70px]">{pr.author.login}</span>
                      </span>
                    )}
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      </div>

      {/* Resizer Handle */}
      <div
        onMouseDown={() => setIsResizing(true)}
        className="w-1 cursor-col-resize hover:bg-primary transition-colors bg-border shrink-0 select-none z-10"
      />

      {/* ── Right Column: Selected Pull Request Detail & Review ── */}
      <div className="flex-1 flex flex-col h-full min-w-0 bg-background overflow-hidden">
        {!selectedPrNumber || !prDetail ? (
          <div className="flex-1 flex flex-col items-center justify-center p-8 text-center text-foreground-subtle space-y-3">
            {loadingDetail ? (
              <>
                <RefreshCw className="size-6 animate-spin text-primary" />
                <span className="text-ui-sm">Fetching pull request details…</span>
              </>
            ) : (
              <>
                <div className="size-16 rounded-2xl bg-surface border border-border flex items-center justify-center shadow-inner">
                  <GitPullRequest className="size-8 text-foreground-subtle opacity-40" />
                </div>
                <h3 className="text-ui-base font-semibold text-foreground">Select a pull request</h3>
                <p className="text-ui-xs text-foreground-subtle max-w-sm">
                  Choose a pull request from the list to view its conversation timeline, approve or request changes,
                  and inspect files changed with full diffs.
                </p>
                <button
                  type="button"
                  onClick={() => setShowCreateModal(true)}
                  className="mt-2 flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-primary text-primary-foreground text-ui-xs font-semibold shadow-xs hover:bg-primary-hover transition-colors"
                >
                  <Plus className="size-3.5" />
                  <span>Create Pull Request</span>
                </button>
              </>
            )}
          </div>
        ) : (
          <div className="flex-1 flex flex-col h-full min-w-0 overflow-hidden">
            {/* PR Header Toolbar */}
            <div className="flex flex-col gap-2 p-3 border-b border-border bg-surface shrink-0">
              <div className="flex items-center justify-between gap-3">
                <div className="flex items-center gap-2 min-w-0">
                  <span
                    className={`flex items-center gap-1.5 px-2 py-0.5 rounded-full text-ui-xs font-bold uppercase tracking-wider shrink-0 ${
                      prDetail.state === 'MERGED'
                        ? 'bg-purple-500/15 text-purple-400 border border-purple-500/30'
                        : prDetail.state === 'CLOSED'
                        ? 'bg-rose-500/15 text-rose-400 border border-rose-500/30'
                        : 'bg-emerald-500/15 text-emerald-400 border border-emerald-500/30'
                    }`}
                  >
                    {prDetail.state === 'MERGED' ? (
                      <GitMerge className="size-3" />
                    ) : prDetail.state === 'CLOSED' ? (
                      <XCircle className="size-3" />
                    ) : (
                      <GitPullRequest className="size-3" />
                    )}
                    <span>{prDetail.state}</span>
                  </span>

                  <h2 className="text-ui-sm font-bold text-foreground tracking-tight truncate leading-snug" title={prDetail.title}>
                    {prDetail.title}{' '}
                    <span className="font-normal font-mono text-foreground-subtle">
                      #{prDetail.number}
                    </span>
                  </h2>
                </div>

                {/* Header Action Buttons */}
                <div className="flex items-center gap-1.5 shrink-0">
                  {/* Checkout Branch */}
                  <button
                    type="button"
                    onClick={handleCheckoutBranch}
                    disabled={isCheckingOut}
                    className="flex items-center gap-1.5 px-2.5 py-1 rounded-lg border border-border bg-background hover:bg-surface-hover text-ui-xs font-semibold text-foreground transition-colors disabled:opacity-50"
                    title="Switch local repository to this pull request's branch"
                  >
                    <GitBranch className={`size-3.5 text-primary ${isCheckingOut ? 'animate-spin' : ''}`} />
                    <span>Checkout</span>
                  </button>

                  {/* Open in Browser */}
                  {prDetail.url && (
                    <button
                      type="button"
                      onClick={() => handleOpenUrl(prDetail.url)}
                      className="p-1.5 rounded-lg border border-border bg-background hover:bg-surface-hover text-foreground-subtle hover:text-foreground transition-colors"
                      title="Open pull request on GitHub"
                    >
                      <ExternalLink className="size-3.5" />
                    </button>
                  )}

                  {/* Merge PR Button (if OPEN) */}
                  {prDetail.state === 'OPEN' && (
                    <button
                      type="button"
                      onClick={() => setShowMergeModal(true)}
                      className="flex items-center gap-1.5 px-2.5 py-1 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white text-ui-xs font-semibold shadow-xs transition-colors"
                    >
                      <GitMerge className="size-3.5" />
                      <span>Merge PR</span>
                    </button>
                  )}

                  {/* Close / Reopen Button */}
                  <button
                    type="button"
                    onClick={handleToggleCloseReopen}
                    disabled={isClosingReopening}
                    className={`flex items-center gap-1 px-2 py-1 rounded-lg border text-ui-xs font-medium transition-colors disabled:opacity-50 ${
                      prDetail.state === 'OPEN'
                        ? 'border-destructive/30 hover:bg-destructive/10 text-destructive'
                        : 'border-emerald-500/30 hover:bg-emerald-500/10 text-emerald-400'
                    }`}
                  >
                    {prDetail.state === 'OPEN' ? 'Close' : 'Reopen'}
                  </button>
                </div>
              </div>

              {/* Branch and Author Info */}
              <div className="flex items-center gap-2 text-ui-xs text-foreground-subtle flex-wrap">
                <span className="font-medium text-foreground">{prDetail.author?.login}</span>
                <span>wants to merge into</span>
                <span className="px-1.5 py-0.2 rounded bg-surface-hover font-mono text-foreground">
                  {prDetail.baseRefName}
                </span>
                <span>from</span>
                <span className="px-1.5 py-0.2 rounded bg-surface-hover font-mono text-foreground">
                  {prDetail.headRefName}
                </span>
                <span>• {timeAgo(prDetail.createdAt)}</span>
              </div>

              {/* Sub-tabs: Conversation vs Files Changed */}
              <div className="flex items-center gap-2 pt-1.5 border-t border-border/50">
                <button
                  type="button"
                  onClick={() => setDetailTab('conversation')}
                  className={`flex items-center gap-2 px-3 py-1.5 rounded-lg text-ui-xs font-medium transition-colors ${
                    detailTab === 'conversation'
                      ? 'bg-primary/10 text-primary font-semibold'
                      : 'text-foreground-subtle hover:text-foreground hover:bg-surface-hover'
                  }`}
                >
                  <MessageSquare className="size-3.5" />
                  <span>Conversation</span>
                  {((prDetail.comments?.length || 0) + (prDetail.reviews?.length || 0)) > 0 && (
                    <span className="px-1.5 py-0.2 rounded-full text-[10px] bg-surface-hover text-foreground">
                      {(prDetail.comments?.length || 0) + (prDetail.reviews?.length || 0)}
                    </span>
                  )}
                </button>

                <button
                  type="button"
                  onClick={() => setDetailTab('files')}
                  className={`flex items-center gap-2 px-3 py-1.5 rounded-lg text-ui-xs font-medium transition-colors ${
                    detailTab === 'files'
                      ? 'bg-primary/10 text-primary font-semibold'
                      : 'text-foreground-subtle hover:text-foreground hover:bg-surface-hover'
                  }`}
                >
                  <FileCode className="size-3.5" />
                  <span>Files Changed</span>
                  {prDetail.files && prDetail.files.length > 0 && (
                    <span className="px-1.5 py-0.2 rounded-full text-[10px] bg-surface-hover text-foreground">
                      {prDetail.files.length}
                    </span>
                  )}
                  {prDetail.additions !== undefined && (
                    <span className="text-[11px] font-mono text-emerald-400">+{prDetail.additions}</span>
                  )}
                  {prDetail.deletions !== undefined && (
                    <span className="text-[11px] font-mono text-rose-400">-{prDetail.deletions}</span>
                  )}
                </button>
              </div>
            </div>

            {/* Sub-tab Body */}
            <div className="flex-1 min-h-0 overflow-y-auto">
              {detailTab === 'conversation' ? (
                /* ── Conversation & Reviews Timeline ── */
                <div className="p-6 max-w-4xl mx-auto space-y-6">
                  {/* PR Description Card */}
                  <div className="rounded-xl border border-border bg-surface shadow-xs overflow-hidden">
                    <div className="flex items-center justify-between px-4 py-2.5 border-b border-border bg-surface-hover/40">
                      <div className="flex items-center gap-2">
                        <span className="size-6 rounded-full bg-primary/20 text-primary flex items-center justify-center text-ui-xs font-bold uppercase">
                          {prDetail.author?.login?.charAt(0) || 'U'}
                        </span>
                        <span className="text-ui-xs font-bold text-foreground">
                          {prDetail.author?.login}
                        </span>
                        <span className="text-ui-xs text-foreground-subtle">
                          opened this pull request {timeAgo(prDetail.createdAt)}
                        </span>
                      </div>
                    </div>
                    <div className="p-4">
                      {renderMarkdownText(prDetail.body)}
                    </div>
                  </div>

                  {/* Reviews & Comments Timeline */}
                  {prDetail.reviews && prDetail.reviews.length > 0 && (
                    <div className="space-y-4">
                      <div className="flex items-center gap-2 text-ui-xs font-bold uppercase tracking-wider text-foreground-subtle">
                        <ShieldCheck className="size-4 text-primary" />
                        <span>Reviews & Feedback</span>
                      </div>

                      {prDetail.reviews.map((rev, idx) => {
                        const isApproved = rev.state === 'APPROVED';
                        const isChangesRequested = rev.state === 'CHANGES_REQUESTED';

                        return (
                          <div
                            key={rev.id || idx}
                            className={`rounded-xl border p-4 shadow-xs transition-colors ${
                              isApproved
                                ? 'border-emerald-500/30 bg-emerald-500/5'
                                : isChangesRequested
                                ? 'border-amber-500/30 bg-amber-500/5'
                                : 'border-border bg-surface'
                            }`}
                          >
                            <div className="flex items-center justify-between gap-2 mb-2">
                              <div className="flex items-center gap-2">
                                <span className="size-5 rounded-full bg-surface-hover flex items-center justify-center text-[10px] font-bold uppercase">
                                  {rev.author?.login?.charAt(0) || 'R'}
                                </span>
                                <span className="text-ui-xs font-bold text-foreground">
                                  {rev.author?.login}
                                </span>
                                <span className="text-ui-xs text-foreground-subtle">
                                  {isApproved
                                    ? 'approved these changes'
                                    : isChangesRequested
                                    ? 'requested changes'
                                    : 'submitted a review'}
                                </span>
                              </div>
                              <span
                                className={`flex items-center gap-1 px-2 py-0.5 rounded text-[10px] font-bold uppercase ${
                                  isApproved
                                    ? 'bg-emerald-500/20 text-emerald-400'
                                    : isChangesRequested
                                    ? 'bg-amber-500/20 text-amber-400'
                                    : 'bg-surface-hover text-foreground-subtle'
                                }`}
                              >
                                {isApproved && <Check className="size-3" />}
                                {isChangesRequested && <AlertCircle className="size-3" />}
                                <span>{rev.state}</span>
                              </span>
                            </div>
                            {rev.body && (
                              <div className="text-ui-sm text-foreground mt-2 pl-7">
                                {rev.body}
                              </div>
                            )}
                          </div>
                        );
                      })}
                    </div>
                  )}

                  {/* General Comments */}
                  {prDetail.comments && prDetail.comments.length > 0 && (
                    <div className="space-y-4">
                      <div className="flex items-center gap-2 text-ui-xs font-bold uppercase tracking-wider text-foreground-subtle">
                        <MessageSquare className="size-4 text-primary" />
                        <span>Discussion Comments ({prDetail.comments.length})</span>
                      </div>

                      {prDetail.comments.map((cmt, idx) => (
                        <div
                          key={cmt.id || idx}
                          className="rounded-xl border border-border bg-surface shadow-xs overflow-hidden"
                        >
                          <div className="flex items-center justify-between px-4 py-2 border-b border-border bg-surface-hover/30 text-ui-xs">
                            <div className="flex items-center gap-2">
                              <span className="size-5 rounded-full bg-primary/20 text-primary flex items-center justify-center text-[10px] font-bold uppercase">
                                {cmt.author?.login?.charAt(0) || 'C'}
                              </span>
                              <span className="font-bold text-foreground">{cmt.author?.login}</span>
                              <span className="text-foreground-subtle">
                                commented {timeAgo(cmt.createdAt)}
                              </span>
                            </div>
                          </div>
                          <div className="p-4 text-ui-sm text-foreground">
                            {renderMarkdownText(cmt.body)}
                          </div>
                        </div>
                      ))}
                    </div>
                  )}

                  {/* Add Comment & Review Form */}
                  <div className="rounded-xl border border-border bg-surface p-4 shadow-sm space-y-3">
                    <div className="flex items-center justify-between">
                      <label className="text-ui-xs font-bold text-foreground uppercase tracking-wider">
                        Leave a Comment or Review
                      </label>
                      <span className="text-[11px] text-foreground-subtle">
                        Markdown supported
                      </span>
                    </div>

                    <textarea
                      value={commentText}
                      onChange={e => setCommentText(e.target.value)}
                      placeholder="Write feedback, approval notes, or review comments…"
                      rows={4}
                      className="w-full px-3 py-2 rounded-lg border border-border bg-background text-ui-sm text-foreground placeholder-foreground-subtle outline-none focus:border-primary resize-y"
                    />

                    <div className="flex items-center justify-between pt-1">
                      <div className="text-[11px] text-foreground-subtle">
                        Tip: You can submit a regular comment or formal review approval.
                      </div>

                      <div className="flex items-center gap-2">
                        {/* Approve Button */}
                        <button
                          type="button"
                          onClick={() => handleSubmitReview('approve')}
                          disabled={reviewAction !== null}
                          className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white text-ui-xs font-semibold shadow-xs transition-colors disabled:opacity-50"
                        >
                          <CheckCircle2 className="size-3.5" />
                          <span>Approve</span>
                        </button>

                        {/* Request Changes */}
                        <button
                          type="button"
                          onClick={() => handleSubmitReview('request-changes')}
                          disabled={reviewAction !== null || !commentText.trim()}
                          className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg border border-amber-500/40 text-amber-400 hover:bg-amber-500/10 text-ui-xs font-semibold shadow-xs transition-colors disabled:opacity-50"
                          title="Specify requested changes in the comment box"
                        >
                          <AlertCircle className="size-3.5" />
                          <span>Request Changes</span>
                        </button>

                        {/* Regular Comment */}
                        <button
                          type="button"
                          onClick={handleAddComment}
                          disabled={submittingComment || !commentText.trim()}
                          className="flex items-center gap-1.5 px-3.5 py-1.5 rounded-lg bg-primary hover:bg-primary-hover text-primary-foreground text-ui-xs font-semibold shadow-xs transition-colors disabled:opacity-50"
                        >
                          <Send className="size-3.5" />
                          <span>Comment</span>
                        </button>
                      </div>
                    </div>
                  </div>
                </div>
              ) : (
                /* ── Files Changed Sub-tab (History Diff Style) ── */
                <div className="flex-1 flex flex-col h-full overflow-hidden bg-background">
                  {/* Top Filter & Expansion Toolbar (identical to History Diff) */}
                  <div className="h-10 min-h-10 px-4 border-b border-border bg-surface/50 flex items-center justify-between gap-3 shrink-0 select-none">
                    {/* Left: Search filter input and stats */}
                    <div className="flex items-center gap-2">
                      <div className="flex items-center gap-1.5 px-2.5 py-1 rounded-lg border border-border bg-background focus-within:border-primary transition-colors">
                        <Search className="size-3.5 text-foreground-subtle shrink-0" />
                        <input
                          type="text"
                          value={fileFilterQuery}
                          onChange={e => setFileFilterQuery(e.target.value)}
                          placeholder={`Filter files in PR (${prDiffFiles.length})…`}
                          className="w-48 sm:w-64 bg-transparent text-[11px] outline-none text-foreground placeholder-foreground-subtle"
                        />
                        {fileFilterQuery && (
                          <button
                            type="button"
                            onClick={() => setFileFilterQuery('')}
                            className="text-foreground-subtle hover:text-foreground cursor-pointer"
                          >
                            <X className="size-3" />
                          </button>
                        )}
                      </div>

                      <div className="hidden sm:flex items-center gap-2 font-mono text-[11px] text-foreground-subtle ml-2">
                        <span>
                          {prDiffFiles.length} {prDiffFiles.length === 1 ? 'file' : 'files'} changed
                        </span>
                        {prDetail.additions !== undefined && prDetail.additions > 0 && (
                          <span className="text-emerald-500 font-semibold">+{prDetail.additions}</span>
                        )}
                        {prDetail.deletions !== undefined && prDetail.deletions > 0 && (
                          <span className="text-rose-500 font-semibold">-{prDetail.deletions}</span>
                        )}
                      </div>
                    </div>

                    {/* Right: Expand / Collapse All */}
                    <div className="flex items-center gap-2">
                      {filteredPrFiles.length > 0 && (
                        <button
                          type="button"
                          onClick={allFilesExpanded ? handleCollapseAllFiles : handleExpandAllFiles}
                          className="flex items-center gap-1.5 px-2.5 py-1 rounded-lg border border-border bg-background hover:bg-surface-hover text-ui-xs text-foreground-subtle hover:text-foreground transition-colors cursor-pointer"
                          title={allFilesExpanded ? 'Collapse all files' : 'Expand all files'}
                        >
                          <ChevronsUpDown className="size-3.5" />
                          <span>{allFilesExpanded ? 'Collapse All' : 'Expand All'}</span>
                        </button>
                      )}
                    </div>
                  </div>

                  {/* Files Stream with HistoryFileCard */}
                  <div className="flex-1 overflow-y-auto">
                    {loadingDetail ? (
                      <div className="flex flex-col items-center justify-center h-64 text-foreground-subtle text-center">
                        <RefreshCw className="size-6 animate-spin text-primary mb-2" />
                        <span className="text-ui-xs font-medium">Loading pull request diffs…</span>
                      </div>
                    ) : filteredPrFiles.length === 0 ? (
                      <div className="flex flex-col items-center justify-center h-64 text-foreground-subtle text-center p-6">
                        <FileText className="size-10 text-border mb-2" />
                        <span className="text-ui-sm font-semibold text-foreground">
                          {prDiffFiles.length === 0 ? 'No files changed in this pull request' : 'No matching files'}
                        </span>
                      </div>
                    ) : (
                      filteredPrFiles.map(file => (
                        <HistoryFileCard
                          key={file.path}
                          file={file}
                          isExpanded={!!expandedFiles[file.path]}
                          onToggle={() => handleToggleFile(file.path)}
                        />
                      ))
                    )}
                  </div>
                </div>
              )}
            </div>
          </div>
        )}
      </div>

      {/* ── Create Pull Request Modal ── */}
      {showCreateModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4 animate-in fade-in duration-100">
          <div className="w-full max-w-xl rounded-2xl border border-border bg-surface p-6 shadow-2xl space-y-4">
            <div className="flex items-center justify-between border-b border-border pb-3">
              <div className="flex items-center gap-2">
                <GitPullRequest className="size-5 text-primary" />
                <h3 className="text-ui-base font-bold text-foreground">Open a Pull Request</h3>
              </div>
              <button
                type="button"
                onClick={() => setShowCreateModal(false)}
                className="p-1 rounded-lg text-foreground-subtle hover:text-foreground hover:bg-surface-hover"
              >
                <X className="size-4" />
              </button>
            </div>

            <form onSubmit={handleCreatePrSubmit} className="space-y-4">
              {/* Branch Source & Target */}
              <div className="grid grid-cols-2 gap-3 p-3 rounded-xl border border-border bg-background">
                <div>
                  <label className="text-[11px] font-semibold text-foreground-subtle uppercase tracking-wider block mb-1">
                    Base (Target) Branch
                  </label>
                  <input
                    type="text"
                    value={newPrBase}
                    onChange={e => setNewPrBase(e.target.value)}
                    placeholder="main"
                    className="w-full px-2.5 py-1.5 rounded-lg border border-border bg-surface font-mono text-ui-xs text-foreground outline-none focus:border-primary"
                  />
                </div>
                <div>
                  <label className="text-[11px] font-semibold text-foreground-subtle uppercase tracking-wider block mb-1">
                    Head (Source) Branch
                  </label>
                  <input
                    type="text"
                    value={newPrHead}
                    onChange={e => setNewPrHead(e.target.value)}
                    placeholder="feature-branch"
                    className="w-full px-2.5 py-1.5 rounded-lg border border-border bg-surface font-mono text-ui-xs text-foreground outline-none focus:border-primary"
                  />
                </div>
              </div>

              {/* Title */}
              <div>
                <label className="text-ui-xs font-semibold text-foreground block mb-1">
                  Title <span className="text-destructive">*</span>
                </label>
                <input
                  type="text"
                  value={newPrTitle}
                  onChange={e => setNewPrTitle(e.target.value)}
                  placeholder="e.g. feat: add pull request management in git client"
                  className="w-full px-3 py-2 rounded-lg border border-border bg-background text-ui-sm text-foreground outline-none focus:border-primary"
                  autoFocus
                  required
                />
              </div>

              {/* Description */}
              <div>
                <label className="text-ui-xs font-semibold text-foreground block mb-1">
                  Description
                </label>
                <textarea
                  value={newPrBody}
                  onChange={e => setNewPrBody(e.target.value)}
                  placeholder="Describe your changes, fixes, and motivation…"
                  rows={5}
                  className="w-full px-3 py-2 rounded-lg border border-border bg-background text-ui-sm text-foreground outline-none focus:border-primary resize-y"
                />
              </div>

              {/* Draft toggle */}
              <div className="flex items-center gap-2 pt-1">
                <input
                  type="checkbox"
                  id="draft-checkbox"
                  checked={newPrDraft}
                  onChange={e => setNewPrDraft(e.target.checked)}
                  className="rounded border-border text-primary focus:ring-primary"
                />
                <label htmlFor="draft-checkbox" className="text-ui-xs text-foreground cursor-pointer select-none">
                  Create as draft pull request (work in progress)
                </label>
              </div>

              {/* Action Buttons */}
              <div className="flex justify-end gap-2 pt-2 border-t border-border">
                <button
                  type="button"
                  onClick={() => setShowCreateModal(false)}
                  className="px-4 py-2 rounded-lg border border-border hover:bg-surface-hover text-ui-xs font-medium text-foreground transition-colors"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={isCreatingPr || !newPrTitle.trim()}
                  className="flex items-center gap-1.5 px-4 py-2 rounded-lg bg-primary hover:bg-primary-hover text-primary-foreground text-ui-xs font-semibold shadow-xs transition-colors disabled:opacity-50"
                >
                  {isCreatingPr && <RefreshCw className="size-3.5 animate-spin" />}
                  <span>Create Pull Request</span>
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* ── Merge Pull Request Modal ── */}
      {showMergeModal && prDetail && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4 animate-in fade-in duration-100">
          <div className="w-full max-w-md rounded-2xl border border-border bg-surface p-6 shadow-2xl space-y-4">
            <div className="flex items-center justify-between border-b border-border pb-3">
              <div className="flex items-center gap-2">
                <GitMerge className="size-5 text-emerald-400" />
                <h3 className="text-ui-base font-bold text-foreground">Merge Pull Request #{prDetail.number}</h3>
              </div>
              <button
                type="button"
                onClick={() => setShowMergeModal(false)}
                className="p-1 rounded-lg text-foreground-subtle hover:text-foreground hover:bg-surface-hover"
              >
                <X className="size-4" />
              </button>
            </div>

            <p className="text-ui-sm text-foreground-subtle">
              Choose a merge strategy to merge branch <span className="font-mono text-foreground font-semibold">{prDetail.headRefName}</span> into <span className="font-mono text-foreground font-semibold">{prDetail.baseRefName}</span>.
            </p>

            {/* Merge Strategy Options */}
            <div className="space-y-2">
              {[
                {
                  id: 'merge',
                  label: 'Create a merge commit',
                  desc: 'All commits from this branch will be added to the base branch via a merge commit.'
                },
                {
                  id: 'squash',
                  label: 'Squash and merge',
                  desc: 'The commits from this branch will be combined into one single commit in the base branch.'
                },
                {
                  id: 'rebase',
                  label: 'Rebase and merge',
                  desc: 'The commits from this branch will be rebased and added to the base branch without a merge commit.'
                }
              ].map(strat => (
                <label
                  key={strat.id}
                  onClick={() => setMergeMethod(strat.id as any)}
                  className={`flex items-start gap-3 p-3 rounded-xl border cursor-pointer transition-all ${
                    mergeMethod === strat.id
                      ? 'border-emerald-500/50 bg-emerald-500/10'
                      : 'border-border bg-background hover:bg-surface-hover/50'
                  }`}
                >
                  <input
                    type="radio"
                    name="mergeStrategy"
                    value={strat.id}
                    checked={mergeMethod === strat.id}
                    onChange={() => setMergeMethod(strat.id as any)}
                    className="mt-1 text-emerald-500 focus:ring-emerald-500"
                  />
                  <div>
                    <h5 className="text-ui-sm font-semibold text-foreground">{strat.label}</h5>
                    <p className="text-[11px] text-foreground-subtle mt-0.5 leading-snug">{strat.desc}</p>
                  </div>
                </label>
              ))}
            </div>

            {/* Action Buttons */}
            <div className="flex justify-end gap-2 pt-2 border-t border-border">
              <button
                type="button"
                onClick={() => setShowMergeModal(false)}
                className="px-4 py-2 rounded-lg border border-border hover:bg-surface-hover text-ui-xs font-medium text-foreground transition-colors"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={handleMergePr}
                disabled={isMerging}
                className="flex items-center gap-1.5 px-4 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white text-ui-xs font-semibold shadow-xs transition-colors disabled:opacity-50"
              >
                {isMerging && <RefreshCw className="size-3.5 animate-spin" />}
                <span>Confirm & Merge</span>
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
