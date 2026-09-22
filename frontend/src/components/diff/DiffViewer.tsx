import React, { useState, useEffect, useMemo } from 'react';
import { 
  Check, 
  X, 
  Sparkles, 
  GitCompare,
  Copy,
  ExternalLink,
  CheckCheck,
  ChevronDown,
  ChevronRight,
  ChevronsDownUp,
  ChevronsUpDown,
  FoldVertical,
  FileText
} from 'lucide-react';
import { FileDiff } from '../../types';
import { useWorkspace } from '../../stores/workspaceStore';
import { ApiBridge } from '../../services/apiBridge';

interface DiffViewerProps {
  diff: FileDiff;
  onClose?: () => void;
  isInline?: boolean;
}

export interface ParsedDiffLine {
  type: 'header' | 'add' | 'del' | 'normal' | 'file-header';
  text: string;
  origLine?: number;
  modLine?: number;
}

export interface SplitDiffRow {
  type: 'row' | 'header' | 'file-header';
  headerText?: string;
  left?: {
    lineNum?: number;
    text: string;
    type: 'normal' | 'del';
  };
  right?: {
    lineNum?: number;
    text: string;
    type: 'normal' | 'add';
  };
}

// Unified Git Diff Parser (builds unified lines & split rows)
export function parseUnifiedDiff(rawText: string): { lines: ParsedDiffLine[]; splitRows: SplitDiffRow[] } {
  if (!rawText) return { lines: [], splitRows: [] };

  const rawLines = rawText.split('\n');
  const lines: ParsedDiffLine[] = [];
  const splitRows: SplitDiffRow[] = [];

  let origLineCount = 1;
  let modLineCount = 1;

  let pendingDels: Array<{ lineNum: number; text: string }> = [];
  let pendingAdds: Array<{ lineNum: number; text: string }> = [];

  const flushPending = () => {
    const maxLen = Math.max(pendingDels.length, pendingAdds.length);
    for (let i = 0; i < maxLen; i++) {
      const del = pendingDels[i];
      const add = pendingAdds[i];
      splitRows.push({
        type: 'row',
        left: del ? { lineNum: del.lineNum, text: del.text, type: 'del' } : undefined,
        right: add ? { lineNum: add.lineNum, text: add.text, type: 'add' } : undefined
      });
    }
    pendingDels = [];
    pendingAdds = [];
  };

  for (const l of rawLines) {
    if (
      l.startsWith('diff --git') ||
      l.startsWith('index ') ||
      l.startsWith('---') ||
      l.startsWith('+++') ||
      l.startsWith('new file') ||
      l.startsWith('deleted file') ||
      l.startsWith('similarity index') ||
      l.startsWith('rename from') ||
      l.startsWith('rename to') ||
      l.startsWith('Binary files') ||
      l.startsWith('\\ No newline')
    ) {
      flushPending();
      lines.push({ type: 'file-header', text: l });
      splitRows.push({ type: 'file-header', headerText: l });
    } else if (l.startsWith('@@')) {
      flushPending();
      const match = l.match(/@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@/);
      if (match) {
        origLineCount = parseInt(match[1], 10);
        modLineCount = parseInt(match[2], 10);
      }
      lines.push({ type: 'header', text: l });
      splitRows.push({ type: 'header', headerText: l });
    } else if (l.startsWith('+')) {
      const lineNum = modLineCount++;
      const text = l.substring(1);
      lines.push({ type: 'add', text, modLine: lineNum });
      pendingAdds.push({ lineNum, text });
    } else if (l.startsWith('-')) {
      const lineNum = origLineCount++;
      const text = l.substring(1);
      lines.push({ type: 'del', text, origLine: lineNum });
      pendingDels.push({ lineNum, text });
    } else {
      flushPending();
      const text = l.startsWith(' ') ? l.substring(1) : l;
      const oNum = origLineCount++;
      const mNum = modLineCount++;
      lines.push({ type: 'normal', text, origLine: oNum, modLine: mNum });
      splitRows.push({
        type: 'row',
        left: { lineNum: oNum, text, type: 'normal' },
        right: { lineNum: mNum, text, type: 'normal' }
      });
    }
  }

  flushPending();
  return { lines, splitRows };
}

export interface FileDiffSection {
  filePath: string;
  lines: ParsedDiffLine[];
  splitRows: SplitDiffRow[];
  additions: number;
  deletions: number;
}

// Partition unified git diff text into per-file sections with split rows & unified lines
export function parseUnifiedDiffByFiles(rawText: string, fallbackPath?: string): FileDiffSection[] {
  if (!rawText || !rawText.trim()) return [];

  const rawLines = rawText.split('\n');
  const sections: FileDiffSection[] = [];

  let currentPath = '';
  let currentRawLines: string[] = [];

  const flush = () => {
    if (currentRawLines.length > 0) {
      const { lines, splitRows } = parseUnifiedDiff(currentRawLines.join('\n'));
      const additions = lines.filter(l => l.type === 'add').length;
      const deletions = lines.filter(l => l.type === 'del').length;
      sections.push({
        filePath: currentPath || fallbackPath || 'Diff',
        lines,
        splitRows,
        additions,
        deletions
      });
    }
    currentRawLines = [];
  };

  for (let i = 0; i < rawLines.length; i++) {
    const l = rawLines[i];
    if (l.startsWith('diff --git')) {
      flush();
      const match = l.match(/diff --git a\/(.+?)\s+b\/(.+)$/);
      if (match) {
        currentPath = match[2];
      } else {
        const altMatch = l.match(/\sb\/(.+)$/);
        currentPath = altMatch ? altMatch[1] : (fallbackPath || '');
      }
      currentRawLines.push(l);
    } else {
      if (!currentPath && l.startsWith('+++ b/')) {
        currentPath = l.substring(6).trim();
      } else if (!currentPath && l.startsWith('--- a/')) {
        currentPath = l.substring(6).trim();
      }
      currentRawLines.push(l);
    }
  }

  flush();

  if (sections.length === 0 && rawLines.length > 0) {
    const { lines, splitRows } = parseUnifiedDiff(rawText);
    sections.push({
      filePath: fallbackPath || 'Diff',
      lines,
      splitRows,
      additions: lines.filter(l => l.type === 'add').length,
      deletions: lines.filter(l => l.type === 'del').length
    });
  }

  return sections;
}

// 2-File Diff Parser for Agent Proposed Changes (LCS based)
export function parseTwoFilesDiff(origText: string, modText: string): { lines: ParsedDiffLine[]; splitRows: SplitDiffRow[] } {
  const origLines = origText ? origText.split('\n') : [];
  const modLines = modText ? modText.split('\n') : [];

  const m = origLines.length;
  const n = modLines.length;

  // Handle empty cases quickly
  if (m === 0 && n === 0) return { lines: [], splitRows: [] };
  if (m === 0) {
    const lines: ParsedDiffLine[] = modLines.map((text, idx) => ({ type: 'add', text, modLine: idx + 1 }));
    const splitRows: SplitDiffRow[] = modLines.map((text, idx) => ({
      type: 'row',
      right: { lineNum: idx + 1, text, type: 'add' }
    }));
    return { lines, splitRows };
  }
  if (n === 0) {
    const lines: ParsedDiffLine[] = origLines.map((text, idx) => ({ type: 'del', text, origLine: idx + 1 }));
    const splitRows: SplitDiffRow[] = origLines.map((text, idx) => ({
      type: 'row',
      left: { lineNum: idx + 1, text, type: 'del' }
    }));
    return { lines, splitRows };
  }

  // Fast path: if files are identical
  if (origText === modText) {
    const lines: ParsedDiffLine[] = origLines.map((text, idx) => ({ type: 'normal', text, origLine: idx + 1, modLine: idx + 1 }));
    const splitRows: SplitDiffRow[] = origLines.map((text, idx) => ({
      type: 'row',
      left: { lineNum: idx + 1, text, type: 'normal' },
      right: { lineNum: idx + 1, text, type: 'normal' }
    }));
    return { lines, splitRows };
  }

  // Compute DP LCS table
  // Bound max dimensions to prevent lag on huge files (> 2000 lines)
  const maxLines = Math.max(m, n);
  if (maxLines > 2500) {
    // Fallback: line-by-line comparison
    const lines: ParsedDiffLine[] = [];
    const splitRows: SplitDiffRow[] = [];
    for (let idx = 0; idx < maxLines; idx++) {
      const origLine = origLines[idx];
      const modLine = modLines[idx];
      if (origLine === modLine) {
        lines.push({ type: 'normal', text: origLine, origLine: idx + 1, modLine: idx + 1 });
        splitRows.push({
          type: 'row',
          left: { lineNum: idx + 1, text: origLine, type: 'normal' },
          right: { lineNum: idx + 1, text: modLine, type: 'normal' }
        });
      } else {
        if (origLine !== undefined) {
          lines.push({ type: 'del', text: origLine, origLine: idx + 1 });
        }
        if (modLine !== undefined) {
          lines.push({ type: 'add', text: modLine, modLine: idx + 1 });
        }
        splitRows.push({
          type: 'row',
          left: origLine !== undefined ? { lineNum: idx + 1, text: origLine, type: 'del' } : undefined,
          right: modLine !== undefined ? { lineNum: idx + 1, text: modLine, type: 'add' } : undefined
        });
      }
    }
    return { lines, splitRows };
  }

  const dp: number[][] = Array.from({ length: m + 1 }, () => new Array(n + 1).fill(0));
  for (let i = 1; i <= m; i++) {
    for (let j = 1; j <= n; j++) {
      if (origLines[i - 1] === modLines[j - 1]) {
        dp[i][j] = dp[i - 1][j - 1] + 1;
      } else {
        dp[i][j] = Math.max(dp[i - 1][j], dp[i][j - 1]);
      }
    }
  }

  // Backtrack
  const diffOps: Array<{ type: 'del' | 'add' | 'normal'; text: string; origLine?: number; modLine?: number }> = [];
  let i = m;
  let j = n;
  while (i > 0 || j > 0) {
    if (i > 0 && j > 0 && origLines[i - 1] === modLines[j - 1]) {
      diffOps.unshift({ type: 'normal', text: origLines[i - 1], origLine: i, modLine: j });
      i--;
      j--;
    } else if (j > 0 && (i === 0 || dp[i][j - 1] >= dp[i - 1][j])) {
      diffOps.unshift({ type: 'add', text: modLines[j - 1], modLine: j });
      j--;
    } else if (i > 0 && (j === 0 || dp[i][j - 1] < dp[i - 1][j])) {
      diffOps.unshift({ type: 'del', text: origLines[i - 1], origLine: i });
      i--;
    }
  }

  const lines: ParsedDiffLine[] = [];
  const splitRows: SplitDiffRow[] = [];

  let pendingDels: Array<{ lineNum?: number; text: string }> = [];
  let pendingAdds: Array<{ lineNum?: number; text: string }> = [];

  const flush = () => {
    const maxLen = Math.max(pendingDels.length, pendingAdds.length);
    for (let k = 0; k < maxLen; k++) {
      const del = pendingDels[k];
      const add = pendingAdds[k];
      splitRows.push({
        type: 'row',
        left: del ? { lineNum: del.lineNum, text: del.text, type: 'del' } : undefined,
        right: add ? { lineNum: add.lineNum, text: add.text, type: 'add' } : undefined
      });
    }
    pendingDels = [];
    pendingAdds = [];
  };

  for (const op of diffOps) {
    if (op.type === 'normal') {
      flush();
      lines.push({ type: 'normal', text: op.text, origLine: op.origLine, modLine: op.modLine });
      splitRows.push({
        type: 'row',
        left: { lineNum: op.origLine, text: op.text, type: 'normal' },
        right: { lineNum: op.modLine, text: op.text, type: 'normal' }
      });
    } else if (op.type === 'del') {
      lines.push({ type: 'del', text: op.text, origLine: op.origLine });
      pendingDels.push({ lineNum: op.origLine, text: op.text });
    } else if (op.type === 'add') {
      lines.push({ type: 'add', text: op.text, modLine: op.modLine });
      pendingAdds.push({ lineNum: op.modLine, text: op.text });
    }
  }
  flush();

  return { lines, splitRows };
}

export const DiffViewer: React.FC<DiffViewerProps> = ({ diff, onClose, isInline = false }) => {
  const { acceptDiff, rejectDiff, createNewSession, openFileInEditor, activeWorkspacePath } = useWorkspace();
  
  const isGitReview = diff.kind === 'git';
  const [viewMode, setViewMode] = useState<'split' | 'unified'>('split');
  const [rawGitDiff, setRawGitDiff] = useState<string>('');
  const [isLoadingDiff, setIsLoadingDiff] = useState(false);
  const [copied, setCopied] = useState(false);

  // Check if content is unified diff format
  const isUnifiedDiff = useMemo(() => {
    if (diff.kind === 'git') return true;
    const mod = diff.modifiedContent || '';
    if (!diff.originalContent && mod) return true;
    if (mod.startsWith('diff --git') || mod.startsWith('@@ ') || mod.startsWith('--- ') || mod.includes('\n@@ -')) {
      return true;
    }
    return false;
  }, [diff.kind, diff.originalContent, diff.modifiedContent]);

  // Fetch real git diff if not provided
  useEffect(() => {
    let isMounted = true;
    const loadRealDiff = async () => {
      // Only fetch if we don't already have text
      if (!diff.modifiedContent && !diff.originalContent) {
        setIsLoadingDiff(true);
        try {
          let diffText = '';
          if (diff.id.startsWith('commitfile-')) {
            const rest = diff.id.slice('commitfile-'.length);
            const sep = rest.indexOf('|');
            diffText = await ApiBridge.gitCommitFileDiff(rest.slice(0, sep), rest.slice(sep + 1), activeWorkspacePath);
          } else if (diff.id.startsWith('commit-')) {
            const hash = diff.id.replace('commit-', '');
            diffText = await ApiBridge.gitCommitDiff(hash, activeWorkspacePath);
          } else {
            const isStaged = diff.id.includes('staged');
            const filePathParam = (diff.filePath === 'Working Tree Changes' || !diff.filePath) ? '' : diff.filePath;
            diffText = await ApiBridge.gitDiff(filePathParam, activeWorkspacePath, isStaged);
          }
          if (isMounted) {
            setRawGitDiff(diffText);
          }
        } catch (e) {
          console.warn('Failed to load git diff:', e);
        } finally {
          if (isMounted) setIsLoadingDiff(false);
        }
      }
    };
    loadRealDiff();
    return () => { isMounted = false; };
  }, [diff.id, diff.filePath, diff.originalContent, diff.modifiedContent, activeWorkspacePath]);

  const [collapsedFiles, setCollapsedFiles] = useState<Record<string, boolean>>({});

  // Parse diff into structured file sections
  const fileSections: FileDiffSection[] = useMemo(() => {
    if (isUnifiedDiff) {
      const text = diff.modifiedContent || rawGitDiff;
      const fallback = (diff.filePath && diff.filePath !== 'Working Tree Changes') ? diff.filePath : (diff.fileName || '');
      return parseUnifiedDiffByFiles(text, fallback);
    }
    const { lines, splitRows } = parseTwoFilesDiff(diff.originalContent || '', diff.modifiedContent || '');
    const additions = lines.filter(l => l.type === 'add').length;
    const deletions = lines.filter(l => l.type === 'del').length;
    return [{
      filePath: diff.filePath || diff.fileName || 'Proposed Changes',
      lines,
      splitRows,
      additions,
      deletions
    }];
  }, [isUnifiedDiff, diff.originalContent, diff.modifiedContent, rawGitDiff, diff.filePath, diff.fileName]);

  const totalAdditions = useMemo(() => {
    return fileSections.reduce((acc, s) => acc + s.additions, 0);
  }, [fileSections]);

  const totalDeletions = useMemo(() => {
    return fileSections.reduce((acc, s) => acc + s.deletions, 0);
  }, [fileSections]);

  const displayAdditions = totalAdditions || diff.additions || 0;
  const displayDeletions = totalDeletions || diff.deletions || 0;

  const toggleFile = (path: string) => {
    setCollapsedFiles(prev => ({
      ...prev,
      [path]: !prev[path]
    }));
  };

  const areAnyCollapsed = useMemo(() => {
    return fileSections.some(s => collapsedFiles[s.filePath]);
  }, [fileSections, collapsedFiles]);

  const handleToggleAll = () => {
    if (areAnyCollapsed) {
      setCollapsedFiles({});
    } else {
      const next: Record<string, boolean> = {};
      fileSections.forEach(s => {
        next[s.filePath] = true;
      });
      setCollapsedFiles(next);
    }
  };

  const handleAccept = () => {
    acceptDiff(diff.id);
  };

  const handleReject = () => {
    rejectDiff(diff.id);
  };

  const handleAskRefine = () => {
    createNewSession(`Please refine the changes in ${diff.filePath}`);
  };

  const handleCopy = () => {
    const textToCopy = diff.modifiedContent || rawGitDiff;
    if (textToCopy) {
      navigator.clipboard.writeText(textToCopy);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  return (
    <div className="w-full h-full min-h-0 flex flex-col bg-background border border-border rounded-[12px] overflow-hidden shadow-xs font-sans">
      {/* Header */}
      <div className="px-4 py-2.5 bg-surface border-b border-border flex items-center justify-between flex-wrap gap-2 text-xs select-none">
        <div className="flex items-center gap-2">
          <GitCompare className="w-4 h-4 text-success" />
          <span 
            onClick={() => {
              if (diff.filePath && diff.filePath !== 'Working Tree Changes' && fileSections.length === 1) {
                openFileInEditor(diff.filePath);
              }
            }}
            className={`font-bold text-foreground font-mono ${
              diff.filePath && diff.filePath !== 'Working Tree Changes' && fileSections.length === 1 ? 'hover:underline cursor-pointer' : ''
            }`}
          >
            {diff.fileName || diff.filePath}
          </span>
          {fileSections.length > 1 && (
            <span className="px-2 py-0.5 rounded-full text-[10px] font-semibold bg-primary/10 text-primary dark:bg-card dark:text-[#93C5FD]">
              {fileSections.length} files changed
            </span>
          )}
          <span className="flex items-center gap-1 font-mono text-[11px]">
            <span className="text-success font-semibold">+{displayAdditions}</span>
            <span className="text-destructive font-semibold">-{displayDeletions}</span>
          </span>
          <span className={`px-2 py-0.5 rounded-full text-[10px] font-semibold font-mono ${
            diff.status === 'accepted' 
              ? 'bg-success/10 text-success dark:bg-success/10 dark:text-success' 
              : diff.status === 'rejected'
              ? 'bg-destructive/10 text-destructive dark:bg-destructive/10 dark:text-destructive'
              : isGitReview
              ? 'bg-surface-hover text-foreground-subtle dark:bg-surface-hover dark:text-foreground-subtle'
              : 'bg-warning/10 text-warning dark:bg-warning/10 dark:text-warning'
          }`}>
            {isGitReview ? 'GIT REVIEW' : diff.status.toUpperCase()}
          </span>
        </div>

        {/* View Controls & Action Buttons */}
        <div className="flex items-center gap-2">
          {/* Collapse/Expand All Button */}
          {fileSections.length > 0 && (
            <button
              type="button"
              onClick={handleToggleAll}
              className="px-2.5 py-1 rounded-[6px] bg-surface-hover hover:bg-surface-hover dark:hover:bg-[#38383C] text-foreground border border-border dark:border-border transition-colors cursor-pointer flex items-center gap-1.5 text-[11px] font-medium shadow-2xs"
              title={areAnyCollapsed ? 'Expand all diffs' : 'Collapse all diffs'}
            >
              {areAnyCollapsed ? (
                <>
                  <ChevronsDownUp className="w-3.5 h-3.5" />
                  <span>Expand all diffs</span>
                </>
              ) : (
                <>
                  <ChevronsUpDown className="w-3.5 h-3.5" />
                  <span>Collapse all diffs</span>
                </>
              )}
            </button>
          )}

          {/* Split / Unified Toggle */}
          <div className="bg-border p-0.5 rounded-[7px] flex items-center text-[11px]">
            <button
              type="button"
              onClick={() => setViewMode('split')}
              className={`px-2.5 py-1 rounded-[5px] transition-colors cursor-pointer ${
                viewMode === 'split' ? 'bg-card text-foreground font-semibold shadow-2xs' : 'text-foreground-subtle'
              }`}
            >
              Split
            </button>
            <button
              type="button"
              onClick={() => setViewMode('unified')}
              className={`px-2.5 py-1 rounded-[5px] transition-colors cursor-pointer ${
                viewMode === 'unified' ? 'bg-card text-foreground font-semibold shadow-2xs' : 'text-foreground-subtle'
              }`}
            >
              Unified
            </button>
          </div>

          {/* Copy Button */}
          <button
            type="button"
            onClick={handleCopy}
            className="p-1.5 rounded-[6px] hover:bg-border text-foreground-subtle hover:text-foreground transition-colors cursor-pointer"
            title="Copy diff text"
          >
            {copied ? <CheckCheck className="w-3.5 h-3.5 text-success" /> : <Copy className="w-3.5 h-3.5" />}
          </button>

          {/* Open in editor tab */}
          {diff.filePath && diff.filePath !== 'Working Tree Changes' && fileSections.length === 1 && (
            <button
              type="button"
              onClick={() => openFileInEditor(diff.filePath)}
              className="flex items-center gap-1 px-2.5 py-1 rounded-[7px] bg-surface-hover hover:bg-surface-hover text-foreground-subtle dark:bg-surface-hover dark:hover:bg-surface-hover dark:text-foreground-subtle font-medium text-xs transition-colors cursor-pointer"
              title="Open full file in editor"
            >
              <ExternalLink className="w-3.5 h-3.5" />
              <span>Open</span>
            </button>
          )}

          {!isGitReview && (
            <button
              type="button"
              onClick={handleAskRefine}
              className="flex items-center gap-1 px-2.5 py-1 rounded-[7px] bg-primary/10 hover:bg-primary/10 text-primary dark:bg-card dark:hover:bg-[#28394F] dark:text-[#93C5FD] font-semibold text-xs transition-colors cursor-pointer"
              title="Ask Forge-ADE to adjust this diff"
            >
              <Sparkles className="w-3.5 h-3.5" />
              <span>Refine</span>
            </button>
          )}

          {!isGitReview && diff.status === 'pending' && (
            <>
              <button
                type="button"
                onClick={handleReject}
                className="flex items-center gap-1 px-2.5 py-1 rounded-[7px] bg-destructive/10 hover:bg-[#FECACA] text-destructive dark:bg-destructive/10 dark:hover:bg-[#5C0D0D] dark:text-destructive font-semibold text-xs transition-colors cursor-pointer"
                title="Reject changes"
              >
                <X className="w-3.5 h-3.5" />
                <span>Reject</span>
              </button>

              <button
                type="button"
                onClick={handleAccept}
                className="flex items-center gap-1 px-3 py-1 rounded-[7px] bg-[#16A34A] hover:bg-success text-white font-semibold text-xs transition-colors shadow-2xs cursor-pointer"
                title="Accept and write to workspace"
              >
                <Check className="w-3.5 h-3.5" />
                <span>Accept</span>
              </button>
            </>
          )}

          {onClose && (
            <button
              type="button"
              onClick={onClose}
              className="p-1 hover:bg-border rounded-[6px] text-foreground-subtle cursor-pointer"
            >
              <X className="w-4 h-4" />
            </button>
          )}
        </div>
      </div>

      {/* Diff Content Viewport */}
      <div className="flex-1 min-h-0 overflow-auto font-mono text-[12px] leading-[20px] select-text">
        {isLoadingDiff ? (
          <div className="p-8 text-center text-xs text-foreground-subtlest">
            Loading repository diff...
          </div>
        ) : fileSections.length === 0 ? (
          <div className="p-8 text-center text-xs text-foreground-subtlest">
            No difference found. Content matches cleanly.
          </div>
        ) : (
          <div className="p-3 space-y-3">
            {fileSections.map((sec, secIdx) => {
              const isCollapsed = !!collapsedFiles[sec.filePath];
              const secFileName = sec.filePath.split('/').pop() || sec.filePath;

              return (
                <div
                  key={sec.filePath || secIdx}
                  className="rounded-[10px] border border-border bg-background overflow-hidden shadow-xs"
                >
                  {/* Accordion File Header */}
                  <div
                    onClick={() => toggleFile(sec.filePath)}
                    className="px-3.5 py-2.5 bg-surface border-b border-border flex items-center justify-between gap-2 cursor-pointer select-none hover:bg-surface-hover dark:hover:bg-surface-hover transition-colors"
                  >
                    <div className="flex items-center gap-2 min-w-0">
                      {isCollapsed ? (
                        <ChevronRight className="w-4 h-4 text-foreground-subtle shrink-0" />
                      ) : (
                        <ChevronDown className="w-4 h-4 text-foreground-subtle shrink-0" />
                      )}
                      <FileText className="w-3.5 h-3.5 text-foreground-subtle shrink-0" />
                      <span className="font-semibold text-xs text-foreground truncate" title={sec.filePath}>
                        {sec.filePath}
                      </span>
                      <span className="flex items-center gap-1 text-[11px] shrink-0 font-sans">
                        {sec.additions > 0 && (
                          <span className="text-emerald-600 dark:text-emerald-400 font-semibold">+{sec.additions}</span>
                        )}
                        {sec.deletions > 0 && (
                          <span className="text-rose-600 dark:text-rose-400 font-semibold">-{sec.deletions}</span>
                        )}
                      </span>
                    </div>

                    <div className="flex items-center gap-1.5 shrink-0" onClick={(e) => e.stopPropagation()}>
                      <button
                        type="button"
                        onClick={() => toggleFile(sec.filePath)}
                        className="px-2 py-0.5 rounded bg-transparent hover:bg-border text-foreground-subtle text-[11px] font-sans font-medium cursor-pointer transition-colors"
                      >
                        {isCollapsed ? 'Expand diff' : 'Collapse diff'}
                      </button>

                      {sec.filePath && sec.filePath !== 'Working Tree Changes' && (
                        <button
                          type="button"
                          onClick={() => openFileInEditor(sec.filePath)}
                          className="flex items-center gap-1 px-2 py-0.5 rounded bg-surface-hover hover:bg-border dark:hover:bg-surface-hover text-foreground-subtle text-[11px] font-sans font-medium transition-colors cursor-pointer"
                          title="Open full file in editor"
                        >
                          <ExternalLink className="w-3 h-3" />
                          <span>Open</span>
                        </button>
                      )}
                    </div>
                  </div>

                  {/* Accordion File Body */}
                  {!isCollapsed && (
                    <div className="flex flex-col">
                      {viewMode === 'split' ? (
                        /* Split View Mode for File */
                        <div className="w-full flex flex-col divide-y divide-border">
                          {/* Split Column Headers */}
                          <div className="grid grid-cols-2 divide-x divide-border bg-surface border-b border-border text-[11px] font-bold text-foreground-subtle select-none sticky top-0 z-10">
                            <div className="px-3 py-1">Original</div>
                            <div className="px-3 py-1 text-success">Modified</div>
                          </div>

                          {sec.splitRows.length === 0 ? (
                            <div className="p-4 text-center text-xs text-foreground-subtlest">
                              No line changes in this file.
                            </div>
                          ) : (
                            sec.splitRows.map((row, rIdx) => {
                              if (row.type === 'header' || row.type === 'file-header') {
                                return (
                                  <div key={rIdx} className="w-full px-3 py-0.5 bg-surface-hover dark:bg-[#202022] text-primary dark:text-info font-bold text-[10px] select-none">
                                    {row.headerText}
                                  </div>
                                );
                              }

                              const left = row.left;
                              const right = row.right;
                              const isDel = left?.type === 'del';
                              const isAdd = right?.type === 'add';

                              return (
                                <div key={rIdx} className="grid grid-cols-2 divide-x divide-border hover:bg-surface/50 dark:hover:bg-surface-hover/30 transition-colors">
                                  {/* Left Pane (Original) */}
                                  <div className={`flex items-start overflow-hidden ${
                                    isDel ? 'bg-destructive/10/60 dark:bg-destructive/10/40 text-destructive dark:text-destructive' : 'text-foreground-subtle dark:text-foreground-secondary'
                                  }`}>
                                    <span className="w-10 text-right pr-2 text-foreground-subtle dark:text-[#555] select-none shrink-0 font-mono text-[11px]">
                                      {left?.lineNum ?? ''}
                                    </span>
                                    <span className="w-4 text-center select-none font-bold text-destructive shrink-0">
                                      {isDel ? '-' : ' '}
                                    </span>
                                    <span className="flex-1 whitespace-pre overflow-x-auto pr-2 font-mono">
                                      {left?.text ?? ''}
                                    </span>
                                  </div>

                                  {/* Right Pane (Modified) */}
                                  <div className={`flex items-start overflow-hidden ${
                                    isAdd ? 'bg-success/10/70 dark:bg-success/10/40 text-success dark:text-success' : 'text-foreground-subtle dark:text-foreground-secondary'
                                  }`}>
                                    <span className="w-10 text-right pr-2 text-foreground-subtle dark:text-[#555] select-none shrink-0 font-mono text-[11px]">
                                      {right?.lineNum ?? ''}
                                    </span>
                                    <span className="w-4 text-center select-none font-bold text-success shrink-0">
                                      {isAdd ? '+' : ' '}
                                    </span>
                                    <span className="flex-1 whitespace-pre overflow-x-auto pr-2 font-mono">
                                      {right?.text ?? ''}
                                    </span>
                                  </div>
                                </div>
                              );
                            })
                          )}
                        </div>
                      ) : (
                        /* Unified View Mode for File */
                        <div className="p-2 space-y-0.5 bg-background">
                          {sec.lines.length === 0 ? (
                            <div className="p-4 text-center text-xs text-foreground-subtlest">
                              No line changes in this file.
                            </div>
                          ) : (
                            sec.lines.map((l, lIdx) => {
                              if (l.type === 'header' || l.type === 'file-header') {
                                return (
                                  <div key={lIdx} className="px-3 py-0.5 bg-surface-hover dark:bg-[#202022] text-primary dark:text-info font-bold text-[10px] rounded-[4px] my-0.5 select-none">
                                    {l.text}
                                  </div>
                                );
                              }
                              const isAdd = l.type === 'add';
                              const isDel = l.type === 'del';
                              return (
                                <div
                                  key={lIdx}
                                  className={`flex items-start ${
                                    isAdd 
                                      ? 'bg-success/10/70 dark:bg-success/10/40 text-success dark:text-success' 
                                      : isDel 
                                      ? 'bg-destructive/10/60 dark:bg-destructive/10/40 text-destructive dark:text-destructive' 
                                      : 'text-foreground-subtle dark:text-foreground-secondary'
                                  }`}
                                >
                                  <span className="w-10 text-right pr-2 text-foreground-subtle dark:text-[#555] select-none shrink-0 font-mono text-[11px]">
                                    {l.origLine ?? ''}
                                  </span>
                                  <span className="w-10 text-right pr-2 text-foreground-subtle dark:text-[#555] select-none shrink-0 font-mono text-[11px]">
                                    {l.modLine ?? ''}
                                  </span>
                                  <span className={`w-4 text-center select-none font-bold ${isAdd ? 'text-success' : isDel ? 'text-destructive' : 'text-transparent'}`}>
                                    {isAdd ? '+' : isDel ? '-' : ' '}
                                  </span>
                                  <span className="flex-1 whitespace-pre overflow-x-auto font-mono">{l.text || ' '}</span>
                                </div>
                              );
                            })
                          )}
                        </div>
                      )}

                      {/* Bottom Collapse diff bar */}
                      <div className="px-3 py-1.5 bg-surface border-t border-border flex items-center justify-between text-[11px] text-foreground-subtle font-sans">
                        <span className="truncate max-w-[320px] font-mono text-[10px]">{secFileName}</span>
                        <button
                          type="button"
                          onClick={() => toggleFile(sec.filePath)}
                          className="hover:text-foreground cursor-pointer font-medium flex items-center gap-1"
                        >
                          <FoldVertical className="w-3.5 h-3.5" />
                          <span>Collapse diff</span>
                        </button>
                      </div>
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
