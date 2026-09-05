import React, { useState, useEffect, useRef } from 'react';
import {
  X,
  GitBranch,
  GitMerge,
  GitCommitHorizontal,
  RefreshCw,
  Copy,
  Check,
  User,
  CornerDownRight,
  Upload,
  Package,
  CloudUpload,
  FileDiff as FileDiffIcon
} from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import {
  GetGitCommitGraph,
  GetGitBranches,
  GetGitCommitDiff,
  GetGitCommitBody,
  GetGitStatus,
  GitFetch,
  GitMerge as WailsGitMerge
} from '../../lib/wails';
import { GitGraphLane } from './GitGraphLane';
import { parseGitDecorations } from '../../lib/gitDecorations';
import { DiffView } from '../diff-view';
import { ResizableSplit } from '../resizable-split';

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

interface CommitGraphResult {
  commits: CommitNode[];
  total_count: number;
  has_more: boolean;
  offset: number;
  limit: number;
}

const PAGE_SIZE = 50;

function formatCommitDate(ts?: string): string {
  if (!ts) return '';
  const d = new Date(ts);
  if (isNaN(d.getTime())) return '';
  return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: '2-digit' });
}

function StatusSign({ status }: { status?: string }) {
  if (status === 'pushed') {
    return <span title="Pushed" className="flex items-center shrink-0"><CloudUpload className="w-3.5 h-3.5 text-emerald-500" /></span>;
  }
  if (status === 'stash') {
    return <span title="Stash" className="flex items-center shrink-0"><Package className="w-3.5 h-3.5 text-amber-500" /></span>;
  }
  if (status === 'local') {
    return <span title="Local (not pushed)" className="flex items-center shrink-0"><Upload className="w-3.5 h-3.5 text-amber-500" /></span>;
  }
  return null;
}

// Full-area git graph pane: lane-based commit graph on the left, selected
// commit metadata + full patch on the right. Lives in the editor tab area
// (tab type 'git-graph'), so it can be opened, focused and closed like any tab.
export const GitGraphPane: React.FC = () => {
  const { closeTab, openDiffInEditor } = useWorkspace();

  const [commits, setCommits] = useState<CommitNode[]>([]);
  const [branches, setBranches] = useState<string[]>([]);
  const [selectedBranch, setSelectedBranch] = useState('');
  const [currentBranch, setCurrentBranch] = useState('');
  const [totalCount, setTotalCount] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [offset, setOffset] = useState(0);
  const [loading, setLoading] = useState(false);

  const [selectedCommit, setSelectedCommit] = useState<CommitNode | null>(null);
  const [commitDiff, setCommitDiff] = useState<string | null>(null);
  const [commitBody, setCommitBody] = useState<string | null>(null);
  const [loadingDiff, setLoadingDiff] = useState(false);
  const [copiedHash, setCopiedHash] = useState(false);
  const [merging, setMerging] = useState(false);
  const [mergeConfirmOpen, setMergeConfirmOpen] = useState(false);
  const selectedRef = useRef<CommitNode | null>(null);
  selectedRef.current = selectedCommit;

  useEffect(() => {
    GetGitBranches('').then(setBranches).catch(() => {});
    GetGitStatus('').then((st: any) => { if (st?.branch) setCurrentBranch(st.branch); }).catch(() => {});
    loadGraph(0, true);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function loadGraph(newOffset: number, reset = false, branchOverride?: string) {
    setLoading(true);
    try {
      const res: CommitGraphResult = await GetGitCommitGraph('', newOffset, PAGE_SIZE, branchOverride ?? selectedBranch);
      if (res) {
        setCommits(prev => (reset ? (res.commits || []) : [...prev, ...(res.commits || [])]));
        setTotalCount(res.total_count || 0);
        setHasMore(res.has_more || false);
        setOffset(newOffset);
      }
    } catch (err) {
      console.error('Failed to load git graph:', err);
    } finally {
      setLoading(false);
    }
  }

  async function handleRefresh() {
    setLoading(true);
    try {
      await GitFetch('');
    } catch { /* no remote is fine */ }
    try {
      await loadGraph(0, true);
      GetGitBranches('').then(setBranches).catch(() => {});
      GetGitStatus('').then((st: any) => { if (st?.branch) setCurrentBranch(st.branch); }).catch(() => {});
    } finally {
      setLoading(false);
    }
  }

  async function handleSelectCommit(node: CommitNode) {
    setSelectedCommit(node);
    setLoadingDiff(true);
    setCommitBody(null);
    try {
      const diffStr = await GetGitCommitDiff('', node.hash);
      setCommitDiff(diffStr || '');
      const body = await GetGitCommitBody('', node.hash);
      setCommitBody(body || null);
    } catch {
      setCommitDiff('');
    } finally {
      setLoadingDiff(false);
    }
  }

  function handleOpenCommitFileDiff(path: string) {
    const commit = selectedRef.current;
    if (!commit || !path) return;
    openDiffInEditor({
      id: `commitfile-${commit.hash}|${path}`,
      kind: 'git',
      filePath: path,
      fileName: `${path.split('/').pop() || path} @ ${commit.hash.slice(0, 7)}`,
      originalContent: '',
      modifiedContent: '',
      additions: 0,
      deletions: 0,
      status: 'pending',
      timestamp: new Date().toISOString()
    });
  }

  function handleCopyHash(hash: string) {
    navigator.clipboard.writeText(hash);
    setCopiedHash(true);
    setTimeout(() => setCopiedHash(false), 1500);
  }

  async function performMerge() {
    const commit = selectedRef.current;
    if (!commit) return;
    setMerging(true);
    try {
      const out = await WailsGitMerge('', commit.hash, false, false);
      console.log('merge:', out);
      setMergeConfirmOpen(false);
      await loadGraph(0, true);
    } catch (err: any) {
      console.error('merge failed:', err);
    } finally {
      setMerging(false);
    }
  }

  const bodyDesc = commitBody ? commitBody.split('\n').slice(1).join('\n').trim() : '';

  const leftContent = (
    <div className="flex flex-col h-full bg-[#f8fafc] dark:bg-[#181818] overflow-hidden font-sans">
      {/* Toolbar */}
      <div className="flex items-center justify-between px-3 py-2 border-b border-[#e5e7eb] dark:border-[#282828] bg-[#f9fafb] dark:bg-[#1e1e1e] shrink-0 select-none">
        <div className="flex items-center gap-2 text-[10px] font-bold uppercase tracking-wider text-[#6b7280] dark:text-[#9ca3af]">
          <GitBranch className="w-4 h-4 text-[#2563eb] dark:text-[#60a5fa]" />
          <span>Commit Graph</span>
          <span className="text-[10px] font-mono normal-case">({totalCount} commits)</span>
        </div>
        <div className="flex items-center gap-2">
          <select
            value={selectedBranch}
            onChange={(e) => {
              const b = e.target.value;
              setSelectedBranch(b);
              setSelectedCommit(null);
              setCommitDiff(null);
              setCommitBody(null);
              loadGraph(0, true, b);
            }}
            className="px-2 py-1 bg-white dark:bg-[#252528] border border-[#e5e7eb] dark:border-[#383838] text-[#374151] dark:text-[#d1d5db] text-[11px] font-mono rounded cursor-pointer focus:outline-none focus:border-[#2563eb]"
            title="Show commits for a branch (All = every branch)"
          >
            <option value="">All branches</option>
            {branches.map(b => (
              <option key={b} value={b}>{b}{b === currentBranch ? ' (HEAD)' : ''}</option>
            ))}
          </select>
          <button
            type="button"
            onClick={handleRefresh}
            disabled={loading}
            className="p-1.5 rounded hover:bg-[#e5e7eb] dark:hover:bg-[#333333] text-[#6b7280] dark:text-[#9ca3af] hover:text-[#111827] dark:hover:text-white transition-colors cursor-pointer disabled:opacity-50"
            title="Fetch from remote & refresh graph"
          >
            <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
          </button>
          <button
            type="button"
            onClick={() => closeTab('tab-git-graph')}
            className="p-1.5 rounded hover:bg-[#e5e7eb] dark:hover:bg-[#333333] text-[#6b7280] dark:text-[#9ca3af] hover:text-[#111827] dark:hover:text-white transition-colors cursor-pointer"
            title="Close Git Graph"
          >
            <X className="w-4 h-4" />
          </button>
        </div>
      </div>

      {/* Commit list */}
      <div className="flex-1 overflow-y-auto font-mono text-[11px] select-text">
        {commits.map(node => {
          const decorations = parseGitDecorations(node.decorations);
          const isHead = decorations.some(d => d.startsWith('HEAD'));
          const isSelected = selectedCommit?.hash === node.hash;
          return (
            <div
              key={node.hash}
              onClick={() => handleSelectCommit(node)}
              className={`flex items-start gap-2 px-3 py-1.5 cursor-pointer border-b border-[#f1f5f9] dark:border-[#222224] transition-colors group ${
                isSelected
                  ? 'bg-[#2563eb]/10 border-l-2 border-l-[#2563eb]'
                  : 'hover:bg-[#f1f5f9] dark:hover:bg-[#252528] border-l-2 border-l-transparent'
              }`}
            >
              <GitGraphLane prefix={node.graph_prefix} isMerge={(node.parents?.length ?? 0) > 1} isHead={isHead} />
              <div className="min-w-0 flex-1 py-0.5">
                <div className="flex items-center gap-1.5 min-w-0">
                  <StatusSign status={node.status} />
                  <span className="font-semibold text-[#2563eb] dark:text-[#60a5fa] shrink-0 group-hover:underline">{node.short_hash}</span>
                  <span className="truncate text-[#111827] dark:text-[#e2e8f0]">{node.message}</span>
                  {decorations.map(d => (
                    <span
                      key={d}
                      title={d}
                      className={`inline-block max-w-32 truncate px-1.5 py-0.5 rounded text-[9px] font-bold font-mono shrink-0 border ${
                        d.startsWith('HEAD')
                          ? 'bg-purple-500/15 border-purple-500/40 text-purple-600 dark:text-purple-300'
                          : 'bg-[#2563eb]/15 border-[#2563eb]/40 text-[#2563eb] dark:text-[#60a5fa]'
                      }`}
                    >
                      {d}
                    </span>
                  ))}
                </div>
                <div className="text-[10px] text-[#9ca3af] flex items-center gap-1.5 mt-0.5">
                  <span className="truncate">{node.author_name}</span>
                  <span>·</span>
                  <span className="shrink-0">{formatCommitDate(node.timestamp)}</span>
                </div>
              </div>
            </div>
          );
        })}
        {commits.length === 0 && !loading && (
          <div className="py-10 text-center text-[#9ca3af] italic select-none">
            No git commit history found.
          </div>
        )}
        {hasMore && (
          <div className="p-4 text-center select-none">
            <button
              type="button"
              onClick={() => loadGraph(offset + PAGE_SIZE, false)}
              disabled={loading}
              className="px-4 py-1.5 bg-white dark:bg-[#252528] border border-[#e5e7eb] dark:border-[#383838] hover:bg-[#f1f5f9] dark:hover:bg-[#2d2d30] text-[#111827] dark:text-white rounded text-xs font-semibold cursor-pointer disabled:opacity-50"
            >
              {loading ? 'Loading...' : 'Load More Commits'}
            </button>
          </div>
        )}
      </div>
    </div>
  );

  const rightContent = (
    <div className="flex flex-col h-full bg-white dark:bg-[#181818] overflow-hidden font-sans">
      {selectedCommit ? (
        <>
          {/* Commit metadata */}
          <div className="p-3 border-b border-[#e5e7eb] dark:border-[#282828] bg-[#f9fafb] dark:bg-[#1e1e1e] space-y-2 shrink-0 select-text">
            <div className="flex items-center justify-between select-none">
              <div className="flex items-center gap-2 text-[#2563eb] dark:text-[#60a5fa] font-bold text-xs font-mono min-w-0">
                <GitCommitHorizontal className="w-4 h-4 shrink-0" />
                <span className="truncate">{selectedCommit.hash}</span>
                <button
                  type="button"
                  onClick={() => handleCopyHash(selectedCommit.hash)}
                  className="p-1 hover:bg-[#e5e7eb] dark:hover:bg-[#333333] rounded text-[#6b7280] dark:text-[#9ca3af] hover:text-[#111827] dark:hover:text-white transition-colors cursor-pointer shrink-0"
                  title="Copy commit hash"
                >
                  {copiedHash ? <Check className="w-3.5 h-3.5 text-emerald-500" /> : <Copy className="w-3.5 h-3.5" />}
                </button>
              </div>
              <button
                type="button"
                onClick={() => { setSelectedCommit(null); setCommitDiff(null); setCommitBody(null); }}
                className="px-2 py-0.5 text-[10px] font-semibold border border-[#e5e7eb] dark:border-[#383838] rounded text-[#6b7280] dark:text-[#9ca3af] hover:text-[#111827] dark:hover:text-white hover:bg-[#f1f5f9] dark:hover:bg-[#2d2d30] transition-colors cursor-pointer shrink-0"
                title="Close commit detail"
              >
                Close
              </button>
            </div>

            <h3 className="text-sm font-semibold text-[#111827] dark:text-white leading-snug">{selectedCommit.message}</h3>

            {bodyDesc && (
              <div className="text-[11px] text-[#6b7280] dark:text-[#9ca3af] leading-relaxed whitespace-pre-wrap max-h-28 overflow-y-auto bg-white dark:bg-[#222224] border border-[#e5e7eb] dark:border-[#383838] rounded p-2">
                {bodyDesc}
              </div>
            )}

            <div className="flex items-center justify-between gap-2 pt-0.5 select-none">
              <div className="flex items-center gap-4 text-[10px] text-[#6b7280] dark:text-[#9ca3af] min-w-0">
                <div className="flex items-center gap-1.5 min-w-0">
                  <User className="w-3.5 h-3.5 shrink-0" />
                  <div className="flex flex-col min-w-0">
                    <span className="truncate">{selectedCommit.author_name}</span>
                    <span className="font-mono">{formatCommitDate(selectedCommit.timestamp)}</span>
                  </div>
                </div>
                {selectedCommit.parents && selectedCommit.parents.length > 0 && (
                  <div className="flex items-center gap-1 font-mono shrink-0">
                    <CornerDownRight className="w-3 h-3" />
                    <span>Parents: {selectedCommit.parents.map(p => p.slice(0, 7)).join(', ')}</span>
                  </div>
                )}
              </div>
              <button
                type="button"
                onClick={() => setMergeConfirmOpen(true)}
                disabled={merging}
                className="px-2.5 py-1 bg-purple-600 hover:bg-purple-700 text-white text-[10px] font-semibold rounded flex items-center gap-1 cursor-pointer disabled:opacity-50 shrink-0"
                title="Merge this commit into the current branch"
              >
                <GitMerge className="w-3.5 h-3.5" />
                <span>{merging ? 'Merging...' : 'Merge to current branch'}</span>
              </button>
            </div>
          </div>

          {/* Full patch */}
          <div className="flex-1 overflow-y-auto p-3">
            {loadingDiff ? (
              <div className="text-[#9ca3af] animate-pulse p-4 text-center text-xs">Loading patch diff...</div>
            ) : (
              <DiffView
                content={commitDiff || ''}
                emptyText="No changes in this commit (clean merges have no patch)."
                onOpenDiff={handleOpenCommitFileDiff}
              />
            )}
          </div>
        </>
      ) : (
        <div className="flex flex-col items-center justify-center h-full text-center gap-2 text-[#9ca3af] select-none">
          <FileDiffIcon className="w-12 h-12 text-[#d1d5db] dark:text-[#333333] mb-1" />
          <span className="text-xs font-mono">Select a commit from the graph to inspect its changes</span>
        </div>
      )}
    </div>
  );

  return (
    <div className="flex flex-col h-full overflow-hidden bg-white dark:bg-[#181818]">
      <div className="flex-1 min-h-0">
        {selectedCommit ? (
          <ResizableSplit
            left={leftContent}
            right={rightContent}
            initialLeftWidth={520}
            minLeftWidth={320}
            maxLeftWidth={900}
          />
        ) : (
          leftContent
        )}
      </div>

      {/* Merge confirmation modal */}
      {mergeConfirmOpen && selectedCommit && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className="bg-white dark:bg-[#222224] border border-[#e5e7eb] dark:border-[#383838] w-full max-w-md shadow-2xl p-4 flex flex-col gap-3 rounded-xl">
            <div className="flex items-center justify-between pb-2 border-b border-[#e5e7eb] dark:border-[#383838]">
              <span className="font-bold text-sm text-[#111827] dark:text-white flex items-center gap-2">
                <GitMerge className="w-4 h-4 text-purple-500" />
                Merge to current branch
              </span>
              <button
                type="button"
                onClick={() => setMergeConfirmOpen(false)}
                className="text-[#9ca3af] hover:text-[#111827] dark:hover:text-white cursor-pointer"
                title="Close"
              >
                <X className="w-4 h-4" />
              </button>
            </div>

            <div className="space-y-2 text-xs">
              <div className="flex items-center justify-between gap-2">
                <span className="text-[#6b7280] dark:text-[#9ca3af]">Merge commit:</span>
                <span className="font-mono text-[#2563eb] dark:text-[#60a5fa] font-semibold">{selectedCommit.short_hash}</span>
              </div>
              <div className="bg-[#f8fafc] dark:bg-[#1e1e1e] border border-[#e5e7eb] dark:border-[#383838] p-2 text-[#374151] dark:text-[#d1d5db] font-mono text-[10px] break-words max-h-24 overflow-y-auto rounded">
                {selectedCommit.message}
              </div>
              <div className="flex items-center justify-between gap-2">
                <span className="text-[#6b7280] dark:text-[#9ca3af]">Into current branch:</span>
                <span className="font-mono font-semibold text-purple-600 dark:text-purple-300">{currentBranch || '(unknown)'}</span>
              </div>
            </div>

            <div className="flex items-center justify-end gap-2 mt-2 pt-2 border-t border-[#e5e7eb] dark:border-[#383838]">
              <button
                type="button"
                onClick={() => setMergeConfirmOpen(false)}
                disabled={merging}
                className="px-3 py-1.5 text-xs text-[#6b7280] dark:text-[#9ca3af] hover:text-[#111827] dark:hover:text-white cursor-pointer disabled:opacity-50"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={performMerge}
                disabled={merging}
                className="px-4 py-1.5 text-xs font-semibold bg-purple-600 hover:bg-purple-700 text-white rounded flex items-center gap-1 cursor-pointer disabled:opacity-50"
              >
                <GitMerge className="w-3.5 h-3.5" />
                <span>{merging ? 'Merging...' : 'Merge'}</span>
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
