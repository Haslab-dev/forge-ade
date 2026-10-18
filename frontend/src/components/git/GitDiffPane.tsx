import React, { useState, useMemo } from 'react';
import {
  AlignJustify,
  Columns,
  Copy,
  Check,
  FileCode,
  Binary,
  ChevronDown,
  ChevronRight,
  RefreshCw,
  FileText
} from 'lucide-react';
import { parseUnifiedDiff, parseUnifiedDiffByFiles, ParsedDiffLine, SplitDiffRow } from '../diff/DiffViewer';

export interface GitDiffPaneProps {
  diffText: string;
  filePath?: string;
  title?: string;
  badge?: React.ReactNode;
  headerActions?: React.ReactNode;
  additions?: number;
  deletions?: number;
  isLoading?: boolean;
  emptyMessage?: string;
  isMultiFile?: boolean;
}

export const GitDiffPane: React.FC<GitDiffPaneProps> = ({
  diffText,
  filePath = '',
  title,
  badge,
  headerActions,
  additions,
  deletions,
  isLoading = false,
  emptyMessage = 'No differences detected',
  isMultiFile = false
}) => {
  const [viewMode, setViewMode] = useState<'unified' | 'split'>('unified');
  const [copied, setCopied] = useState(false);
  const [collapsedFiles, setCollapsedFiles] = useState<Record<string, boolean>>({});

  // Parse diff for multi-file commit diffs or single-file diffs
  const sections = useMemo(() => {
    if (!diffText || !diffText.trim()) return [];
    if (isMultiFile) {
      return parseUnifiedDiffByFiles(diffText);
    }
    const { lines, splitRows } = parseUnifiedDiff(diffText);
    const adds = additions ?? lines.filter(l => l.type === 'add').length;
    const dels = deletions ?? lines.filter(l => l.type === 'del').length;
    return [{
      filePath: filePath || 'Diff',
      lines,
      splitRows,
      additions: adds,
      deletions: dels
    }];
  }, [diffText, isMultiFile, filePath, additions, deletions]);

  const isBinary = useMemo(() => {
    return diffText.includes('Binary files') && diffText.includes('differ');
  }, [diffText]);

  const handleCopy = () => {
    if (!diffText) return;
    navigator.clipboard.writeText(diffText);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const toggleFile = (path: string) => {
    setCollapsedFiles(prev => ({ ...prev, [path]: !prev[path] }));
  };

  const displayTitle = title || (filePath ? filePath.split('/').pop() || filePath : 'Diff');
  const dirPath = filePath && filePath.includes('/') ? filePath.substring(0, filePath.lastIndexOf('/')) : '';

  return (
    <div className="flex-1 flex flex-col h-full overflow-hidden bg-background">
      {/* Top Diff Header Bar */}
      <div className="h-10 min-h-10 px-4 border-b border-border bg-surface/50 flex items-center justify-between gap-3 shrink-0 select-none">
        {/* File information */}
        <div className="flex items-center gap-2 min-w-0">
          <FileCode className="size-4 text-primary shrink-0" />
          <div className="flex items-baseline gap-1.5 truncate">
            <span className="font-mono text-ui-xs font-semibold text-foreground truncate" title={filePath || displayTitle}>
              {displayTitle}
            </span>
            {dirPath && (
              <span className="font-mono text-[11px] text-foreground-subtle truncate">
                {dirPath}
              </span>
            )}
          </div>

          {badge && <div className="shrink-0">{badge}</div>}

          {/* Stats */}
          {((additions ?? 0) > 0 || (deletions ?? 0) > 0) && (
            <div className="flex items-center gap-1 font-mono text-[11px] shrink-0 font-medium">
              {(additions ?? 0) > 0 && <span className="text-emerald-500">+{additions}</span>}
              {(deletions ?? 0) > 0 && <span className="text-rose-500">-{deletions}</span>}
            </div>
          )}
        </div>

        {/* View Mode & Actions */}
        <div className="flex items-center gap-2 shrink-0">
          {/* Unified / Split toggle */}
          <div className="flex items-center p-0.5 rounded-lg border border-border bg-background/80 text-ui-xs">
            <button
              type="button"
              onClick={() => setViewMode('unified')}
              className={`flex items-center gap-1 px-2 py-0.5 rounded text-[11px] font-medium transition-colors ${
                viewMode === 'unified'
                  ? 'bg-surface-hover text-foreground shadow-xs'
                  : 'text-foreground-subtle hover:text-foreground'
              }`}
              title="Unified inline diff view"
            >
              <AlignJustify className="size-3" />
              <span>Unified</span>
            </button>
            <button
              type="button"
              onClick={() => setViewMode('split')}
              className={`flex items-center gap-1 px-2 py-0.5 rounded text-[11px] font-medium transition-colors ${
                viewMode === 'split'
                  ? 'bg-surface-hover text-foreground shadow-xs'
                  : 'text-foreground-subtle hover:text-foreground'
              }`}
              title="Side-by-side split diff view"
            >
              <Columns className="size-3" />
              <span>Split</span>
            </button>
          </div>

          {/* Copy diff button */}
          <button
            type="button"
            onClick={handleCopy}
            className="flex items-center gap-1 px-2 py-1 rounded-md border border-border bg-background hover:bg-surface-hover text-foreground-subtle hover:text-foreground text-[11px] font-medium transition-colors cursor-pointer"
            title="Copy diff to clipboard"
          >
            {copied ? (
              <>
                <Check className="size-3 text-success" />
                <span className="text-success">Copied</span>
              </>
            ) : (
              <>
                <Copy className="size-3" />
                <span>Copy</span>
              </>
            )}
          </button>

          {headerActions && <div className="flex items-center gap-1.5">{headerActions}</div>}
        </div>
      </div>

      {/* Diff Viewport */}
      <div className="flex-1 overflow-auto bg-surface/10 p-3">
        {isLoading ? (
          <div className="flex flex-col items-center justify-center h-56 text-foreground-subtle text-center">
            <RefreshCw className="size-6 animate-spin text-primary mb-2" />
            <span className="text-ui-xs font-medium">Loading diff…</span>
          </div>
        ) : isBinary ? (
          <div className="flex flex-col items-center justify-center h-56 p-8 text-center bg-surface/30 border border-border/60 rounded-xl max-w-md mx-auto my-8">
            <Binary className="size-10 text-foreground-subtle/50 mb-2" />
            <span className="text-ui-sm font-semibold text-foreground">Binary file</span>
            <span className="text-ui-xs text-foreground-subtle mt-1">
              Binary files differ. Differences cannot be displayed as text.
            </span>
          </div>
        ) : sections.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-56 text-foreground-subtle text-center">
            <Check className="size-8 text-emerald-500/50 mb-2" />
            <span className="text-ui-sm font-medium text-foreground">{emptyMessage}</span>
            <span className="text-ui-xs text-foreground-subtle mt-1">The file contents match cleanly.</span>
          </div>
        ) : (
          <div className="space-y-4">
            {sections.map((sec, secIdx) => {
              const isCollapsed = !!collapsedFiles[sec.filePath];
              const secFileName = sec.filePath.split('/').pop() || sec.filePath;
              const secDirPath = sec.filePath.includes('/') ? sec.filePath.substring(0, sec.filePath.lastIndexOf('/')) : '';

              // Filter out git plumbing lines (diff --git, index, etc.) from direct code rendering
              const contentLines = sec.lines.filter(l => l.type !== 'file-header');
              const contentSplitRows = sec.splitRows.filter(r => r.type !== 'file-header');

              return (
                <div
                  key={sec.filePath || secIdx}
                  className="rounded-xl border border-border/80 bg-background overflow-hidden shadow-xs"
                >
                  {/* File Header when in multi-file mode */}
                  {isMultiFile && (
                    <div
                      role="button"
                      tabIndex={0}
                      onClick={() => toggleFile(sec.filePath)}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter' || e.key === ' ') {
                          e.preventDefault();
                          toggleFile(sec.filePath);
                        }
                      }}
                      className="px-3.5 py-2 bg-surface/80 border-b border-border flex items-center justify-between gap-2 cursor-pointer select-none hover:bg-surface-hover transition-colors"
                    >
                      <div className="flex items-center gap-2 min-w-0">
                        {isCollapsed ? (
                          <ChevronRight className="size-4 text-foreground-subtle shrink-0" />
                        ) : (
                          <ChevronDown className="size-4 text-foreground-subtle shrink-0" />
                        )}
                        <FileText className="size-3.5 text-foreground-subtle shrink-0" />
                        <span className="font-mono text-ui-xs font-semibold text-foreground truncate">
                          {secFileName}
                        </span>
                        {secDirPath && (
                          <span className="font-mono text-[10px] text-foreground-subtle truncate">
                            {secDirPath}
                          </span>
                        )}
                        <div className="flex items-center gap-1 text-[10px] font-mono shrink-0 ml-1">
                          {sec.additions > 0 && <span className="text-emerald-500 font-semibold">+{sec.additions}</span>}
                          {sec.deletions > 0 && <span className="text-rose-500 font-semibold">-{sec.deletions}</span>}
                        </div>
                      </div>

                      <span className="text-[11px] text-foreground-subtle">
                        {isCollapsed ? 'Expand' : 'Collapse'}
                      </span>
                    </div>
                  )}

                  {!isCollapsed && (
                    <div className="overflow-x-auto">
                      {viewMode === 'unified' ? (
                        /* Unified Diff View */
                        <div className="divide-y divide-border/20 font-mono text-ui-xs">
                          {contentLines.length === 0 ? (
                            <div className="p-6 text-center text-ui-xs text-foreground-subtle">
                              No code changes in this file.
                            </div>
                          ) : (
                            contentLines.map((line, lIdx) => {
                              if (line.type === 'header') {
                                // Extract hunk range @@ ... @@ and function context
                                const hunkMatch = line.text.match(/^(@@\s+-[0-9,]+\s+\+[0-9,]+\s+@@)(.*)$/);
                                const hunkRange = hunkMatch ? hunkMatch[1] : line.text;
                                const hunkContext = hunkMatch ? hunkMatch[2].trim() : '';

                                return (
                                  <div
                                    key={lIdx}
                                    className="flex items-center px-3 py-1 bg-primary/10 text-primary border-y border-primary/20 my-0.5 select-none text-[11px]"
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

                              const isAdd = line.type === 'add';
                              const isDel = line.type === 'del';

                              return (
                                <div
                                  key={lIdx}
                                  className={`flex items-start leading-[22px] transition-colors ${
                                    isAdd
                                      ? 'bg-emerald-500/10 text-emerald-950 dark:text-emerald-200 border-l-2 border-emerald-500'
                                      : isDel
                                        ? 'bg-rose-500/10 text-rose-950 dark:text-rose-200 border-l-2 border-rose-500'
                                        : 'text-foreground/85 border-l-2 border-transparent hover:bg-surface/40'
                                  }`}
                                >
                                  {/* Old Line # */}
                                  <span className="w-11 text-right pr-2 select-none text-[10px] text-foreground-subtle/50 font-mono shrink-0">
                                    {line.origLine ?? ''}
                                  </span>

                                  {/* New Line # */}
                                  <span className="w-11 text-right pr-2 select-none text-[10px] text-foreground-subtle/50 font-mono shrink-0">
                                    {line.modLine ?? ''}
                                  </span>

                                  {/* Sign Symbol */}
                                  <span className={`w-5 text-center select-none font-bold text-xs shrink-0 ${
                                    isAdd ? 'text-emerald-500' : isDel ? 'text-rose-500' : 'text-transparent'
                                  }`}>
                                    {isAdd ? '+' : isDel ? '-' : ' '}
                                  </span>

                                  {/* Code Text */}
                                  <span className="flex-1 whitespace-pre pr-3 select-text overflow-x-auto">
                                    {line.text}
                                  </span>
                                </div>
                              );
                            })
                          )}
                        </div>
                      ) : (
                        /* Split (Side-by-Side) Diff View */
                        <div className="divide-y divide-border/20 font-mono text-ui-xs">
                          {/* Column Headers */}
                          <div className="grid grid-cols-2 divide-x divide-border bg-surface/70 border-b border-border text-[11px] font-semibold text-foreground-subtle select-none sticky top-0 z-10">
                            <div className="px-3 py-1">Original</div>
                            <div className="px-3 py-1 text-emerald-600 dark:text-emerald-400">Modified</div>
                          </div>

                          {contentSplitRows.length === 0 ? (
                            <div className="p-6 text-center text-ui-xs text-foreground-subtle">
                              No code changes in this file.
                            </div>
                          ) : (
                            contentSplitRows.map((row, rIdx) => {
                              if (row.type === 'header') {
                                return (
                                  <div
                                    key={rIdx}
                                    className="col-span-2 px-3 py-1 bg-primary/10 text-primary border-y border-primary/20 my-0.5 select-none text-[11px] font-semibold"
                                  >
                                    {row.headerText}
                                  </div>
                                );
                              }

                              const left = row.left;
                              const right = row.right;
                              const isDel = left?.type === 'del';
                              const isAdd = right?.type === 'add';

                              return (
                                <div key={rIdx} className="grid grid-cols-2 divide-x divide-border/50 hover:bg-surface/30 transition-colors">
                                  {/* Left (Original) */}
                                  <div className={`flex items-start leading-[22px] overflow-hidden ${
                                    isDel ? 'bg-rose-500/10 text-rose-950 dark:text-rose-200 border-l-2 border-rose-500' : 'text-foreground/80'
                                  }`}>
                                    <span className="w-11 text-right pr-2 select-none text-[10px] text-foreground-subtle/50 font-mono shrink-0">
                                      {left?.lineNum ?? ''}
                                    </span>
                                    <span className={`w-5 text-center select-none font-bold text-xs shrink-0 ${
                                      isDel ? 'text-rose-500' : 'text-transparent'
                                    }`}>
                                      {isDel ? '-' : ' '}
                                    </span>
                                    <span className="flex-1 whitespace-pre pr-2 overflow-x-auto select-text">
                                      {left?.text ?? ''}
                                    </span>
                                  </div>

                                  {/* Right (Modified) */}
                                  <div className={`flex items-start leading-[22px] overflow-hidden ${
                                    isAdd ? 'bg-emerald-500/10 text-emerald-950 dark:text-emerald-200 border-l-2 border-emerald-500' : 'text-foreground/80'
                                  }`}>
                                    <span className="w-11 text-right pr-2 select-none text-[10px] text-foreground-subtle/50 font-mono shrink-0">
                                      {right?.lineNum ?? ''}
                                    </span>
                                    <span className={`w-5 text-center select-none font-bold text-xs shrink-0 ${
                                      isAdd ? 'text-emerald-500' : 'text-transparent'
                                    }`}>
                                      {isAdd ? '+' : ' '}
                                    </span>
                                    <span className="flex-1 whitespace-pre pr-2 overflow-x-auto select-text">
                                      {right?.text ?? ''}
                                    </span>
                                  </div>
                                </div>
                              );
                            })
                          )}
                        </div>
                      )}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
};
