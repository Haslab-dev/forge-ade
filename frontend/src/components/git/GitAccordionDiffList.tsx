import React, { useState, useEffect, useMemo, useRef } from 'react';
import {
  ChevronDown,
  ChevronUp,
  RefreshCw,
  Plus,
  Minus,
  RotateCcw,
  Check,
  GitCommit,
  Search,
  X,
  FileCode,
  FileText,
  Braces,
  Atom,
  Code2,
  Binary,
  Layers,
  ChevronsUpDown,
  AlertCircle
} from 'lucide-react';
import { ApiBridge } from '../../services/apiBridge';
import { showToast } from '../../lib/toast';
import { parseUnifiedDiff, ParsedDiffLine } from '../diff/DiffViewer';

export interface GitFileItem {
  path: string;
  status: string;
  staging?: 'staged' | 'unstaged' | 'untracked';
  dir?: string;
  additions?: number;
  deletions?: number;
}

interface GitAccordionDiffListProps {
  cwd: string;
  gitBranch: string;
  gitFiles: GitFileItem[];
  onRefresh: () => Promise<void>;
  onStageFile: (path: string, e?: React.MouseEvent) => Promise<void>;
  onUnstageFile: (path: string, e?: React.MouseEvent) => Promise<void>;
  onDiscardFile: (path: string, e?: React.MouseEvent) => Promise<void>;
  onStageAll: () => Promise<void>;
  onUnstageAll: () => Promise<void>;
  onCommit: (message: string) => Promise<void>;
  isCommitting?: boolean;
}

import {
  getFileIcon,
  isBinaryFile,
  computeIntraLineSegments,
  CHUNK_SIZE
} from './gitDiffUtils';

/** Single file accordion card with on-demand diff loading, chunked rendering, and intra-line highlights */
interface FileAccordionCardProps {
  file: GitFileItem;
  cwd: string;
  isExpanded: boolean;
  onToggle: () => void;
  onStage: (path: string, e?: React.MouseEvent) => void;
  onUnstage: (path: string, e?: React.MouseEvent) => void;
  onDiscard: (path: string, e?: React.MouseEvent) => void;
  diffCache: Record<string, { loading: boolean; text: string; error?: string }>;
  fetchDiff: (file: GitFileItem) => void;
}

const FileAccordionCard: React.FC<FileAccordionCardProps> = ({
  file,
  cwd,
  isExpanded,
  onToggle,
  onStage,
  onUnstage,
  onDiscard,
  diffCache,
  fetchDiff
}) => {
  const filename = file.path.split('/').pop() || file.path;
  const dir = file.path.includes('/') ? file.path.substring(0, file.path.lastIndexOf('/') + 1) : '';
  const cacheKey = `${file.staging}:${file.path}`;
  const cached = diffCache[cacheKey];

  // Chunking state per file
  const [visibleCount, setVisibleCount] = useState<number>(CHUNK_SIZE);

  // Fetch diff when expanded if not already cached
  useEffect(() => {
    if (isExpanded && !cached) {
      fetchDiff(file);
    }
  }, [isExpanded, cached, file, fetchDiff]);

  // Parse lines when diff text is ready
  const { lines, isBinary, rawLineCount } = useMemo(() => {
    if (!cached || !cached.text) {
      return { lines: [], isBinary: isBinaryFile(file.path), rawLineCount: 0 };
    }
    if (isBinaryFile(file.path, cached.text)) {
      return { lines: [], isBinary: true, rawLineCount: 0 };
    }

    const { lines: parsedLines } = parseUnifiedDiff(cached.text);
    const content = parsedLines.filter(l => l.type !== 'file-header');
    return { lines: content, isBinary: false, rawLineCount: content.length };
  }, [cached, file.path]);

  // Pre-calculate paired line intra-line diffs for deleted lines immediately followed by added lines
  const pairedIntra = useMemo(() => {
    const pairs: Record<number, { delSegments: { text: string; changed: boolean }[]; addSegments: { text: string; changed: boolean }[] }> = {};
    for (let i = 0; i < lines.length - 1; i++) {
      if (lines[i].type === 'del' && lines[i + 1].type === 'add') {
        const { oldSegments, newSegments } = computeIntraLineSegments(lines[i].text, lines[i + 1].text);
        pairs[i] = { delSegments: oldSegments, addSegments: newSegments };
      }
    }
    return pairs;
  }, [lines]);

  const renderedLines = useMemo(() => {
    return lines.slice(0, visibleCount);
  }, [lines, visibleCount]);

  const isLargeDiff = rawLineCount > 1000;

  return (
    <div className="border-b border-border/80 bg-background overflow-hidden">
      {/* File Accordion Header Row */}
      <div
        role="button"
        tabIndex={0}
        onClick={onToggle}
        onKeyDown={e => {
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault();
            onToggle();
          }
        }}
        className="w-full px-3 py-2 bg-surface/50 hover:bg-surface-hover flex items-center justify-between cursor-pointer select-none transition-colors group"
      >
        {/* Left: Icon, Filename, Directory */}
        <div className="flex items-center gap-2 min-w-0">
          {getFileIcon(filename)}
          <span className="font-mono text-ui-xs font-semibold text-foreground truncate">
            {filename}
          </span>
          {dir && (
            <span className="font-mono text-[11px] text-foreground-subtle truncate">
              {dir}
            </span>
          )}
          {file.staging === 'staged' && (
            <span className="px-1.5 py-0.2 rounded text-[10px] font-bold bg-success/15 text-success">
              Staged
            </span>
          )}
          {isLargeDiff && (
            <span className="px-1.5 py-0.2 rounded text-[10px] font-semibold bg-amber-500/15 text-amber-500">
              {rawLineCount.toLocaleString()} lines
            </span>
          )}
        </div>

        {/* Right: Actions, Additions/Deletions, Chevron */}
        <div className="flex items-center gap-3 shrink-0">
          {/* Hover Actions: Stage / Unstage / Discard */}
          <div
            className="opacity-0 group-hover:opacity-100 flex items-center gap-1 transition-opacity"
            onClick={e => e.stopPropagation()}
          >
            {file.staging === 'staged' ? (
              <button
                type="button"
                onClick={e => onUnstage(file.path, e)}
                className="p-1 rounded hover:bg-surface border border-transparent hover:border-border text-foreground-subtle hover:text-foreground text-[10px] transition-colors"
                title="Unstage file"
              >
                <Minus className="size-3" />
              </button>
            ) : (
              <button
                type="button"
                onClick={e => onStage(file.path, e)}
                className="p-1 rounded hover:bg-surface border border-transparent hover:border-border text-emerald-500 hover:text-emerald-400 text-[10px] transition-colors"
                title="Stage file"
              >
                <Plus className="size-3" />
              </button>
            )}
            <button
              type="button"
              onClick={e => onDiscard(file.path, e)}
              className="p-1 rounded hover:bg-destructive/15 text-foreground-subtle hover:text-destructive text-[10px] transition-colors"
              title="Discard changes"
            >
              <RotateCcw className="size-3" />
            </button>
          </div>

          {/* Additions / Deletions */}
          <div className="flex items-center gap-1.5 font-mono text-[11px] font-semibold">
            <span className="text-emerald-500">+{file.additions ?? 0}</span>
            <span className="text-rose-500">-{file.deletions ?? 0}</span>
          </div>

          {/* Chevron */}
          {isExpanded ? (
            <ChevronUp className="size-4 text-foreground-subtle" />
          ) : (
            <ChevronDown className="size-4 text-foreground-subtle" />
          )}
        </div>
      </div>

      {/* Expanded Diff Body */}
      {isExpanded && (
        <div className="border-t border-border/40 bg-surface/10">
          {cached?.loading ? (
            <div className="flex items-center justify-center py-8 text-foreground-subtle text-ui-xs font-mono">
              <RefreshCw className="size-4 animate-spin mr-2 text-primary" />
              <span>Loading file diff…</span>
            </div>
          ) : cached?.error ? (
            <div className="p-4 text-destructive text-ui-xs font-mono">
              {cached.error}
            </div>
          ) : isBinary ? (
            <div className="flex flex-col items-center justify-center py-6 text-foreground-subtle text-center">
              <Binary className="size-8 text-foreground-subtle/50 mb-1" />
              <span className="text-ui-xs font-medium text-foreground">Binary file</span>
              <span className="text-[11px] text-foreground-subtle mt-0.5">
                Binary files differ. Differences cannot be displayed as text.
              </span>
            </div>
          ) : lines.length === 0 ? (
            <div className="py-4 text-center text-foreground-subtle text-ui-xs font-mono">
              (No differences detected)
            </div>
          ) : (
            <div>
              {/* Lines rendering */}
              <div className="font-mono text-ui-xs select-text overflow-x-auto divide-y divide-border/10">
                {renderedLines.map((line, idx) => {
                  const isAdd = line.type === 'add';
                  const isDel = line.type === 'del';
                  const isNormal = line.type === 'normal';
                  const isHeader = line.type === 'header';
                  const isNoNewline = line.text.includes('No newline at end of file');

                  if (isNoNewline) {
                    return (
                      <div key={idx} className="flex items-center text-foreground-subtle/70 italic text-[11px] py-1 px-3 pl-12 select-none bg-surface/20">
                        <span>{line.text}</span>
                      </div>
                    );
                  }

                  if (isHeader) {
                    const hunkMatch = line.text.match(/^(@@\s+-[0-9,]+\s+\+[0-9,]+\s+@@)(.*)$/);
                    const hunkRange = hunkMatch ? hunkMatch[1] : line.text;
                    const hunkContext = hunkMatch ? hunkMatch[2].trim() : '';

                    return (
                      <div
                        key={idx}
                        className="flex items-center px-3 py-0.5 bg-primary/10 text-primary border-y border-primary/20 select-none text-[11px]"
                      >
                        <span className="font-semibold shrink-0">{hunkRange}</span>
                        {hunkContext && (
                          <span className="ml-3 text-foreground-subtle font-sans truncate text-[11px]">
                            {hunkContext}
                          </span>
                        )}
                      </div>
                    );
                  }

                  // Intra-line diff highlighting for paired del/add
                  let intraSegments: { text: string; changed: boolean }[] | null = null;
                  if (isDel && pairedIntra[idx]) {
                    intraSegments = pairedIntra[idx].delSegments;
                  } else if (isAdd && pairedIntra[idx - 1]) {
                    intraSegments = pairedIntra[idx - 1].addSegments;
                  }

                  return (
                    <div
                      key={idx}
                      className={`flex items-start leading-[20px] transition-colors ${
                        isAdd
                          ? 'bg-emerald-950/25 dark:bg-emerald-950/35 text-emerald-200'
                          : isDel
                            ? 'bg-rose-950/25 dark:bg-rose-950/35 text-rose-200'
                            : 'text-foreground/85 hover:bg-surface/30'
                      }`}
                    >
                      {/* Line Number Gutter */}
                      <span
                        className={`w-10 text-right pr-3 select-none text-[11px] font-mono shrink-0 ${
                          isAdd
                            ? 'text-emerald-400 font-medium'
                            : isDel
                              ? 'text-rose-400 font-medium'
                              : 'text-foreground-subtle/50'
                        }`}
                      >
                        {isAdd ? (line.modLine ?? '') : isDel ? (line.origLine ?? '') : (line.modLine ?? line.origLine ?? '')}
                      </span>

                      {/* Code Content */}
                      <span className="flex-1 whitespace-pre pr-3 overflow-x-auto select-text font-mono">
                        {intraSegments ? (
                          intraSegments.map((seg, sIdx) => (
                            <span
                              key={sIdx}
                              className={
                                seg.changed
                                  ? isAdd
                                    ? 'bg-emerald-500/40 text-emerald-100 rounded-xs'
                                    : 'bg-rose-500/40 text-rose-100 rounded-xs'
                                  : undefined
                              }
                            >
                              {seg.text}
                            </span>
                          ))
                        ) : (
                          line.text
                        )}
                      </span>
                    </div>
                  );
                })}
              </div>

              {/* Chunk Controller Bar if diff has more lines than visibleCount */}
              {lines.length > visibleCount && (
                <div className="flex items-center justify-between px-4 py-2 bg-surface/90 border-t border-border/60 text-ui-xs select-none">
                  <div className="flex items-center gap-2 text-foreground-subtle">
                    <span>
                      Showing <strong className="text-foreground">{visibleCount.toLocaleString()}</strong> of <strong className="text-foreground">{lines.length.toLocaleString()}</strong> lines
                    </span>
                    {lines.length > 3000 && (
                      <span className="px-1.5 py-0.5 rounded text-[10px] bg-amber-500/15 text-amber-500 font-semibold">
                        Large Diff ({lines.length.toLocaleString()} lines)
                      </span>
                    )}
                  </div>
                  <div className="flex items-center gap-2">
                    <button
                      type="button"
                      onClick={() => setVisibleCount(c => Math.min(lines.length, c + CHUNK_SIZE))}
                      className="px-2.5 py-1 rounded bg-background hover:bg-surface-hover border border-border text-foreground text-[11px] font-medium transition-colors cursor-pointer"
                    >
                      +{CHUNK_SIZE} lines
                    </button>
                    {lines.length - visibleCount > 500 && (
                      <button
                        type="button"
                        onClick={() => setVisibleCount(c => Math.min(lines.length, c + 1000))}
                        className="px-2.5 py-1 rounded bg-background hover:bg-surface-hover border border-border text-foreground text-[11px] font-medium transition-colors cursor-pointer"
                      >
                        +1,000 lines
                      </button>
                    )}
                    <button
                      type="button"
                      onClick={() => setVisibleCount(lines.length)}
                      className="px-2.5 py-1 rounded hover:bg-surface-hover text-foreground-subtle hover:text-foreground text-[11px] transition-colors cursor-pointer"
                      title={lines.length > 5000 ? 'Rendering all lines may cause lag' : ''}
                    >
                      Show all
                    </button>
                  </div>
                </div>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  );
};

export const GitAccordionDiffList: React.FC<GitAccordionDiffListProps> = ({
  cwd,
  gitBranch,
  gitFiles,
  onRefresh,
  onStageFile,
  onUnstageFile,
  onDiscardFile,
  onStageAll,
  onUnstageAll,
  onCommit,
  isCommitting = false
}) => {
  // Staging filter: 'unstaged' (default, matches screenshot) | 'staged' | 'all'
  const [stagingFilter, setStagingFilter] = useState<'unstaged' | 'staged' | 'all'>('unstaged');
  const [filterMenuOpen, setFilterMenuOpen] = useState(false);
  const filterMenuRef = useRef<HTMLDivElement>(null);

  // Search filter
  const [searchQuery, setSearchQuery] = useState('');
  const [showSearch, setShowSearch] = useState(false);

  // Commit bar state
  const [commitSummary, setCommitSummary] = useState('');

  // Expand/collapse states for files: keyed by `${staging}:${path}`
  const [expandedFiles, setExpandedFiles] = useState<Record<string, boolean>>({});
  const [diffCache, setDiffCache] = useState<Record<string, { loading: boolean; text: string; error?: string }>>({});
  const [isRefreshing, setIsRefreshing] = useState(false);

  // Close filter dropdown on outside click
  useEffect(() => {
    const handleClick = (e: MouseEvent) => {
      if (filterMenuRef.current && !filterMenuRef.current.contains(e.target as Node)) {
        setFilterMenuOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, []);

  // Categorize files
  const stagedFiles = useMemo(() => gitFiles.filter(f => f.staging === 'staged'), [gitFiles]);
  const unstagedFiles = useMemo(() => gitFiles.filter(f => f.staging === 'unstaged' || f.staging === 'untracked'), [gitFiles]);

  const displayedFiles = useMemo(() => {
    let list: GitFileItem[] = [];
    if (stagingFilter === 'unstaged') {
      list = unstagedFiles;
    } else if (stagingFilter === 'staged') {
      list = stagedFiles;
    } else {
      list = gitFiles;
    }

    if (!searchQuery.trim()) return list;
    const q = searchQuery.toLowerCase();
    return list.filter(f => f.path.toLowerCase().includes(q));
  }, [stagingFilter, unstagedFiles, stagedFiles, gitFiles, searchQuery]);

  // Initial auto-expand: expand the first 1-2 files that are not binary or huge
  useEffect(() => {
    if (displayedFiles.length > 0) {
      setExpandedFiles(prev => {
        // If already has user interaction, preserve
        if (Object.keys(prev).length > 0) return prev;
        const initial: Record<string, boolean> = {};
        for (let i = 0; i < Math.min(2, displayedFiles.length); i++) {
          const f = displayedFiles[i];
          if (!isBinaryFile(f.path)) {
            initial[`${f.staging}:${f.path}`] = true;
          }
        }
        return initial;
      });
    }
  }, [displayedFiles]);

  const fetchDiff = async (f: GitFileItem) => {
    const key = `${f.staging}:${f.path}`;
    if (diffCache[key]?.text || diffCache[key]?.loading) return;

    setDiffCache(prev => ({ ...prev, [key]: { loading: true, text: '' } }));
    try {
      let text = '';
      if (f.staging === 'untracked') {
        const content = await ApiBridge.readFile(f.path);
        text = `--- /dev/null\n+++ b/${f.path}\n@@ -0,0 +1,${(content.match(/\n/g) || []).length + 1} @@\n` +
          content.split('\n').map(l => `+${l}`).join('\n');
      } else {
        text = await ApiBridge.gitDiff(f.path, cwd, f.staging === 'staged');
      }
      setDiffCache(prev => ({ ...prev, [key]: { loading: false, text: text || '(No differences detected)' } }));
    } catch (err: any) {
      setDiffCache(prev => ({ ...prev, [key]: { loading: false, text: '', error: err?.message || 'Failed to load diff' } }));
    }
  };

  const handleToggleFile = (f: GitFileItem) => {
    const key = `${f.staging}:${f.path}`;
    setExpandedFiles(prev => ({ ...prev, [key]: !prev[key] }));
  };

  const handleExpandAll = () => {
    const next: Record<string, boolean> = {};
    for (const f of displayedFiles) {
      next[`${f.staging}:${f.path}`] = true;
    }
    setExpandedFiles(next);
  };

  const handleCollapseAll = () => {
    setExpandedFiles({});
  };

  const handleRefreshClick = async () => {
    setIsRefreshing(true);
    setDiffCache({});
    try {
      await onRefresh();
    } finally {
      setIsRefreshing(false);
    }
  };

  const handleCommitSubmit = async () => {
    if (!commitSummary.trim()) {
      showToast('Please enter a commit message', 'warn');
      return;
    }
    await onCommit(commitSummary.trim());
    setCommitSummary('');
  };

  const allExpanded = displayedFiles.length > 0 && displayedFiles.every(f => expandedFiles[`${f.staging}:${f.path}`]);

  return (
    <div className="flex-1 flex flex-col h-full overflow-hidden bg-background">
      {/* ── Top Bar matching Screenshot ── */}
      <div className="h-11 min-h-11 px-3 border-b border-border bg-surface/60 flex items-center justify-between gap-3 shrink-0 select-none">
        {/* Left: Unstaged / Staged dropdown */}
        <div className="flex items-center gap-2">
          <div className="relative" ref={filterMenuRef}>
            <button
              type="button"
              onClick={() => setFilterMenuOpen(prev => !prev)}
              className="flex items-center gap-1.5 px-3 py-1 rounded-lg border border-border bg-background hover:bg-surface-hover text-ui-xs font-semibold text-foreground transition-colors shadow-xs cursor-pointer"
            >
              <span>
                {stagingFilter === 'unstaged'
                  ? 'Unstaged'
                  : stagingFilter === 'staged'
                    ? 'Staged'
                    : 'All Changes'}
              </span>
              <span className="text-[10px] text-foreground-subtle ml-0.5">
                (
                {stagingFilter === 'unstaged'
                  ? unstagedFiles.length
                  : stagingFilter === 'staged'
                    ? stagedFiles.length
                    : gitFiles.length}
                )
              </span>
              <ChevronDown className={`size-3.5 text-foreground-subtle transition-transform ${filterMenuOpen ? 'rotate-180' : ''}`} />
            </button>

            {filterMenuOpen && (
              <div className="absolute left-0 top-full mt-1 w-44 rounded-xl border border-border bg-surface shadow-xl z-50 py-1 font-mono text-ui-xs">
                <button
                  type="button"
                  onClick={() => {
                    setStagingFilter('unstaged');
                    setFilterMenuOpen(false);
                  }}
                  className={`w-full text-left px-3 py-1.5 flex items-center justify-between hover:bg-surface-hover transition-colors ${
                    stagingFilter === 'unstaged' ? 'text-primary font-semibold' : 'text-foreground'
                  }`}
                >
                  <span>Unstaged</span>
                  <span className="text-[10px] text-foreground-subtle font-sans font-medium">{unstagedFiles.length}</span>
                </button>
                <button
                  type="button"
                  onClick={() => {
                    setStagingFilter('staged');
                    setFilterMenuOpen(false);
                  }}
                  className={`w-full text-left px-3 py-1.5 flex items-center justify-between hover:bg-surface-hover transition-colors ${
                    stagingFilter === 'staged' ? 'text-primary font-semibold' : 'text-foreground'
                  }`}
                >
                  <span>Staged</span>
                  <span className="text-[10px] text-foreground-subtle font-sans font-medium">{stagedFiles.length}</span>
                </button>
                <div className="my-1 border-t border-border/50" />
                <button
                  type="button"
                  onClick={() => {
                    setStagingFilter('all');
                    setFilterMenuOpen(false);
                  }}
                  className={`w-full text-left px-3 py-1.5 flex items-center justify-between hover:bg-surface-hover transition-colors ${
                    stagingFilter === 'all' ? 'text-primary font-semibold' : 'text-foreground'
                  }`}
                >
                  <span>All Changes</span>
                  <span className="text-[10px] text-foreground-subtle font-sans font-medium">{gitFiles.length}</span>
                </button>
              </div>
            )}
          </div>

          {/* Quick Search Toggle */}
          {showSearch ? (
            <div className="flex items-center gap-1.5 px-2 py-0.5 rounded-lg border border-border bg-background">
              <Search className="size-3 text-foreground-subtle shrink-0" />
              <input
                type="text"
                value={searchQuery}
                onChange={e => setSearchQuery(e.target.value)}
                placeholder="Filter files…"
                className="w-36 bg-transparent text-[11px] outline-none text-foreground placeholder-foreground-subtle"
                autoFocus
              />
              <button
                type="button"
                onClick={() => {
                  setSearchQuery('');
                  setShowSearch(false);
                }}
                className="text-foreground-subtle hover:text-foreground"
              >
                <X className="size-3" />
              </button>
            </div>
          ) : (
            <button
              type="button"
              onClick={() => setShowSearch(true)}
              className="p-1 rounded-md text-foreground-subtle hover:text-foreground hover:bg-surface-hover transition-colors"
              title="Search files"
            >
              <Search className="size-3.5" />
            </button>
          )}
        </div>

        {/* Right: Actions */}
        <div className="flex items-center gap-2">
          {/* Stage All / Unstage All */}
          {stagingFilter === 'unstaged' && unstagedFiles.length > 0 && (
            <button
              type="button"
              onClick={onStageAll}
              className="flex items-center gap-1 px-2.5 py-1 rounded-lg border border-border bg-background hover:bg-surface-hover text-ui-xs font-medium text-foreground transition-colors cursor-pointer"
              title="Stage all modified files"
            >
              <Plus className="size-3 text-emerald-500" />
              <span>Stage All</span>
            </button>
          )}

          {stagingFilter === 'staged' && stagedFiles.length > 0 && (
            <button
              type="button"
              onClick={onUnstageAll}
              className="flex items-center gap-1 px-2.5 py-1 rounded-lg border border-border bg-background hover:bg-surface-hover text-ui-xs font-medium text-foreground transition-colors cursor-pointer"
              title="Unstage all files"
            >
              <Minus className="size-3 text-rose-500" />
              <span>Unstage All</span>
            </button>
          )}

          {/* Expand / Collapse all */}
          {displayedFiles.length > 0 && (
            <button
              type="button"
              onClick={allExpanded ? handleCollapseAll : handleExpandAll}
              className="flex items-center gap-1 px-2 py-1 rounded-lg border border-border bg-background hover:bg-surface-hover text-ui-xs text-foreground-subtle hover:text-foreground transition-colors cursor-pointer"
              title={allExpanded ? 'Collapse all diffs' : 'Expand all diffs'}
            >
              <ChevronsUpDown className="size-3" />
              <span>{allExpanded ? 'Collapse' : 'Expand'}</span>
            </button>
          )}

          {/* Refresh Button matching screenshot */}
          <button
            type="button"
            onClick={handleRefreshClick}
            disabled={isRefreshing}
            className="flex items-center gap-1.5 px-2.5 py-1 rounded-lg border border-border bg-background hover:bg-surface-hover text-ui-xs font-medium text-foreground transition-colors cursor-pointer disabled:opacity-50"
            title="Refresh Git status"
          >
            <RefreshCw className={`size-3.5 ${isRefreshing ? 'animate-spin text-primary' : 'text-foreground-subtle'}`} />
            <span>Refresh</span>
          </button>
        </div>
      </div>

      {/* ── Files Stream Area (Full Width, No Sidepanel) ── */}
      <div className="flex-1 overflow-y-auto">
        {displayedFiles.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-64 text-foreground-subtle text-center p-6">
            <Check className="size-10 text-emerald-500/50 mb-2" />
            <span className="text-ui-sm font-semibold text-foreground">
              {stagingFilter === 'staged' ? 'No staged changes' : 'Working tree is clean'}
            </span>
            <span className="text-ui-xs text-foreground-subtle mt-1 max-w-sm">
              {stagingFilter === 'staged'
                ? 'Stage changes from Unstaged to prepare your next commit.'
                : 'No unstaged files detected in this workspace.'}
            </span>
          </div>
        ) : (
          displayedFiles.map(file => {
            const key = `${file.staging}:${file.path}`;
            const isExpanded = !!expandedFiles[key];

            return (
              <FileAccordionCard
                key={key}
                file={file}
                cwd={cwd}
                isExpanded={isExpanded}
                onToggle={() => handleToggleFile(file)}
                onStage={onStageFile}
                onUnstage={onUnstageFile}
                onDiscard={onDiscardFile}
                diffCache={diffCache}
                fetchDiff={fetchDiff}
              />
            );
          })
        )}
      </div>

      {/* ── Bottom Commit Bar ── */}
      <div className="p-3 border-t border-border bg-surface/80 flex items-center gap-3 shrink-0 select-none">
        <div className="flex-1 flex items-center gap-2 px-3 py-1.5 rounded-lg border border-border bg-background focus-within:ring-1 focus-within:ring-primary shadow-xs">
          <GitCommit className="size-4 text-foreground-subtle shrink-0" />
          <input
            type="text"
            value={commitSummary}
            onChange={e => setCommitSummary(e.target.value)}
            onKeyDown={e => {
              if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
                handleCommitSubmit();
              }
            }}
            placeholder={
              stagedFiles.length > 0
                ? `Commit message to ${gitBranch || 'main'} (Cmd+Enter to commit)...`
                : 'Stage files above to commit changes...'
            }
            className="w-full bg-transparent text-ui-xs outline-none text-foreground placeholder-foreground-subtle"
          />
        </div>

        <button
          type="button"
          onClick={handleCommitSubmit}
          disabled={isCommitting || stagedFiles.length === 0 || !commitSummary.trim()}
          className="flex items-center gap-2 px-4 py-1.5 rounded-lg bg-primary text-primary-foreground font-medium text-ui-xs hover:bg-primary/90 transition-colors disabled:opacity-50 cursor-pointer shadow-xs shrink-0"
        >
          <Check className="size-4" />
          <span>{isCommitting ? 'Committing…' : `Commit (${stagedFiles.length})`}</span>
        </button>
      </div>
    </div>
  );
};
