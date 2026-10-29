import React, { useState, useMemo, useEffect, useRef } from 'react';
import {
  Check,
  Copy,
  Clock,
  User,
  Search,
  ChevronDown,
  ChevronUp,
  ChevronsUpDown,
  RefreshCw,
  Binary,
  FileCode,
  FileText,
  X
} from 'lucide-react';
import {
  getFileIcon,
  isBinaryFile,
  computeIntraLineSegments,
  timeAgo,
  CHUNK_SIZE
} from './gitDiffUtils';
import { parseUnifiedDiff, ParsedDiffLine } from '../diff/DiffViewer';

export interface CommitDiffFileItem {
  path: string;
  additions: number;
  deletions: number;
  diff: string;
}

interface GitCommitDiffViewProps {
  commit: {
    hash: string;
    author_name: string;
    author_email?: string;
    timestamp: string | number;
    message: string;
    short_hash?: string;
  };
  files: CommitDiffFileItem[];
  rawDiff?: string;
  isLoading?: boolean;
}

/** Individual file card for commit history with on-demand parsing, intra-line diff highlights, and chunked rendering */
export interface HistoryFileCardProps {
  file: CommitDiffFileItem;
  isExpanded: boolean;
  onToggle: () => void;
}

export const HistoryFileCard: React.FC<HistoryFileCardProps> = ({
  file,
  isExpanded,
  onToggle
}) => {
  const filename = file.path.split('/').pop() || file.path;
  const dir = file.path.includes('/') ? file.path.substring(0, file.path.lastIndexOf('/') + 1) : '';

  // Chunking state per file
  const [visibleCount, setVisibleCount] = useState<number>(CHUNK_SIZE);

  const { lines, isBinary, rawLineCount } = useMemo(() => {
    if (!file.diff || !file.diff.trim()) {
      return { lines: [], isBinary: isBinaryFile(file.path), rawLineCount: 0 };
    }
    if (isBinaryFile(file.path, file.diff)) {
      return { lines: [], isBinary: true, rawLineCount: 0 };
    }

    const { lines: parsedLines } = parseUnifiedDiff(file.diff);
    const content = parsedLines.filter(l => l.type !== 'file-header');
    return { lines: content, isBinary: false, rawLineCount: content.length };
  }, [file.diff, file.path]);

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
    <div id={`history-file-${file.path}`} className="border-b border-border/80 bg-background overflow-hidden">
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
          {isLargeDiff && (
            <span className="px-1.5 py-0.2 rounded text-[10px] font-semibold bg-amber-500/15 text-amber-500">
              {rawLineCount.toLocaleString()} lines
            </span>
          )}
        </div>

        {/* Right: Additions/Deletions, Chevron */}
        <div className="flex items-center gap-3 shrink-0">
          <div className="flex items-center gap-1.5 font-mono text-[11px] font-semibold">
            {file.additions > 0 && <span className="text-emerald-500">+{file.additions}</span>}
            {file.deletions > 0 && <span className="text-rose-500">-{file.deletions}</span>}
          </div>

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
          {isBinary ? (
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

export const GitCommitDiffView: React.FC<GitCommitDiffViewProps> = ({
  commit,
  files,
  rawDiff,
  isLoading = false
}) => {
  const [copied, setCopied] = useState(false);
  const [searchFilter, setSearchFilter] = useState('');
  const [showSearch, setShowSearch] = useState(false);
  const [expandedFiles, setExpandedFiles] = useState<Record<string, boolean>>({});

  const totalAdditions = useMemo(() => files.reduce((acc, f) => acc + (f.additions || 0), 0), [files]);
  const totalDeletions = useMemo(() => files.reduce((acc, f) => acc + (f.deletions || 0), 0), [files]);

  const filteredFiles = useMemo(() => {
    if (!searchFilter.trim()) return files;
    const q = searchFilter.toLowerCase();
    return files.filter(f => f.path.toLowerCase().includes(q));
  }, [files, searchFilter]);

  // Initial expand logic: expand first 2 files (unless huge), collapse others
  useEffect(() => {
    if (files.length > 0) {
      const initial: Record<string, boolean> = {};
      const limit = files.length <= 3 ? files.length : 2;
      for (let i = 0; i < limit; i++) {
        if (!isBinaryFile(files[i].path)) {
          initial[files[i].path] = true;
        }
      }
      setExpandedFiles(initial);
    }
  }, [commit.hash, files]);

  const handleCopyHash = () => {
    navigator.clipboard.writeText(commit.hash);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const handleToggle = (path: string) => {
    setExpandedFiles(prev => ({ ...prev, [path]: !prev[path] }));
  };

  const handleExpandAll = () => {
    const next: Record<string, boolean> = {};
    for (const f of filteredFiles) {
      next[f.path] = true;
    }
    setExpandedFiles(next);
  };

  const handleCollapseAll = () => {
    setExpandedFiles({});
  };

  const allExpanded = filteredFiles.length > 0 && filteredFiles.every(f => expandedFiles[f.path]);

  return (
    <div className="flex-1 flex flex-col h-full overflow-hidden bg-background">
      {/* ── Top Commit Metadata Card ── */}
      <div className="p-3.5 border-b border-border bg-surface shrink-0 space-y-2 select-none">
        <div className="flex items-start justify-between gap-3">
          <h3 className="text-ui-base font-semibold text-foreground leading-snug break-words">
            {commit.message}
          </h3>
          <button
            type="button"
            onClick={handleCopyHash}
            className="flex items-center gap-1.5 px-2.5 py-1 rounded-lg border border-border bg-background hover:bg-surface-hover text-ui-xs font-mono text-foreground-subtle transition-colors shrink-0 cursor-pointer shadow-xs"
            title="Copy full commit SHA"
          >
            {copied ? (
              <>
                <Check className="size-3 text-success" />
                <span className="text-success">Copied!</span>
              </>
            ) : (
              <>
                <Copy className="size-3" />
                <span>{commit.short_hash || commit.hash.substring(0, 8)}</span>
              </>
            )}
          </button>
        </div>

        <div className="flex flex-wrap items-center justify-between gap-2 text-ui-xs text-foreground-subtle pt-0.5">
          <div className="flex items-center gap-4">
            <div className="flex items-center gap-1.5">
              <User className="size-3.5 text-foreground-subtle" />
              <span className="font-medium text-foreground">{commit.author_name}</span>
              {commit.author_email && (
                <span className="text-foreground-subtlest">&lt;{commit.author_email}&gt;</span>
              )}
            </div>
            <div className="flex items-center gap-1.5">
              <Clock className="size-3.5 text-foreground-subtle" />
              <span>{new Date(commit.timestamp).toLocaleString()} ({timeAgo(commit.timestamp)})</span>
            </div>
          </div>

          <div className="flex items-center gap-2 font-mono text-ui-xs">
            <span className="text-foreground-subtle font-sans">
              {files.length} {files.length === 1 ? 'file' : 'files'} changed
            </span>
            {totalAdditions > 0 && (
              <span className="text-emerald-500 font-semibold">+{totalAdditions}</span>
            )}
            {totalDeletions > 0 && (
              <span className="text-rose-500 font-semibold">-{totalDeletions}</span>
            )}
          </div>
        </div>
      </div>

      {/* ── Filter / Navigation Toolbar ── */}
      <div className="h-10 min-h-10 px-3 border-b border-border bg-surface/50 flex items-center justify-between gap-3 shrink-0 select-none">
        <div className="flex items-center gap-2">
          {showSearch ? (
            <div className="flex items-center gap-1.5 px-2 py-0.5 rounded-lg border border-border bg-background">
              <Search className="size-3 text-foreground-subtle shrink-0" />
              <input
                type="text"
                value={searchFilter}
                onChange={e => setSearchFilter(e.target.value)}
                placeholder="Filter files in commit…"
                className="w-48 bg-transparent text-[11px] outline-none text-foreground placeholder-foreground-subtle"
                autoFocus
              />
              <button
                type="button"
                onClick={() => {
                  setSearchFilter('');
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
              className="flex items-center gap-1.5 px-2.5 py-1 rounded-lg border border-border bg-background hover:bg-surface-hover text-[11px] text-foreground-subtle hover:text-foreground transition-colors cursor-pointer"
            >
              <Search className="size-3" />
              <span>Filter files ({files.length})</span>
            </button>
          )}
        </div>

        {/* Right actions */}
        <div className="flex items-center gap-2">
          {filteredFiles.length > 0 && (
            <button
              type="button"
              onClick={allExpanded ? handleCollapseAll : handleExpandAll}
              className="flex items-center gap-1 px-2.5 py-1 rounded-lg border border-border bg-background hover:bg-surface-hover text-ui-xs text-foreground-subtle hover:text-foreground transition-colors cursor-pointer"
              title={allExpanded ? 'Collapse all files' : 'Expand all files'}
            >
              <ChevronsUpDown className="size-3" />
              <span>{allExpanded ? 'Collapse All' : 'Expand All'}</span>
            </button>
          )}
        </div>
      </div>

      {/* ── Full Width Files Stream (No Sidepanel Files) ── */}
      <div className="flex-1 overflow-y-auto">
        {isLoading ? (
          <div className="flex flex-col items-center justify-center h-64 text-foreground-subtle text-center">
            <RefreshCw className="size-6 animate-spin text-primary mb-2" />
            <span className="text-ui-xs font-medium">Loading commit diffs…</span>
          </div>
        ) : filteredFiles.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-64 text-foreground-subtle text-center p-6">
            <FileText className="size-10 text-border mb-2" />
            <span className="text-ui-sm font-semibold text-foreground">
              {files.length === 0 ? 'No files changed in this commit' : 'No matching files'}
            </span>
          </div>
        ) : (
          filteredFiles.map(file => (
            <HistoryFileCard
              key={file.path}
              file={file}
              isExpanded={!!expandedFiles[file.path]}
              onToggle={() => handleToggle(file.path)}
            />
          ))
        )}
      </div>
    </div>
  );
};
