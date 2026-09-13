import React, { useState, useEffect, useMemo } from 'react';
import { 
  Check, 
  X, 
  Sparkles, 
  GitCompare,
  Copy,
  ExternalLink,
  CheckCheck
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
    if (l.startsWith('diff --git') || l.startsWith('index ') || l.startsWith('---') || l.startsWith('+++')) {
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

  // Parse diff into structured lines & split rows
  const { lines, splitRows } = useMemo(() => {
    if (isUnifiedDiff) {
      const text = diff.modifiedContent || rawGitDiff;
      return parseUnifiedDiff(text);
    }
    return parseTwoFilesDiff(diff.originalContent || '', diff.modifiedContent || '');
  }, [isUnifiedDiff, diff.originalContent, diff.modifiedContent, rawGitDiff]);

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
    <div className="w-full h-full min-h-0 flex flex-col bg-white dark:bg-[#161617] border border-[#E5E7EB] dark:border-[#333336] rounded-[12px] overflow-hidden shadow-xs font-sans">
      {/* Header */}
      <div className="px-4 py-2.5 bg-[#F9FAFB] dark:bg-[#1E1E20] border-b border-[#E5E7EB] dark:border-[#333336] flex items-center justify-between flex-wrap gap-2 text-xs select-none">
        <div className="flex items-center gap-2">
          <GitCompare className="w-4 h-4 text-[#16A34A] dark:text-[#4ADE80]" />
          <span 
            onClick={() => {
              if (diff.filePath && diff.filePath !== 'Working Tree Changes') {
                openFileInEditor(diff.filePath);
              }
            }}
            className="font-bold text-[#111827] dark:text-[#F2F2F2] hover:underline cursor-pointer font-['JetBrains_Mono',monospace]"
          >
            {diff.fileName || diff.filePath}
          </span>
          <span className="flex items-center gap-1 font-['JetBrains_Mono',monospace] text-[11px]">
            <span className="text-[#16A34A] dark:text-[#4ADE80] font-semibold">+{diff.additions || 0}</span>
            <span className="text-[#DC2626] dark:text-[#EF4444] font-semibold">-{diff.deletions || 0}</span>
          </span>
          <span className={`px-2 py-0.5 rounded-full text-[10px] font-semibold font-['JetBrains_Mono',monospace] ${
            diff.status === 'accepted' 
              ? 'bg-[#DCFCE7] text-[#166534] dark:bg-[#064E3B] dark:text-[#86EFAC]' 
              : diff.status === 'rejected'
              ? 'bg-[#FEE2E2] text-[#991B1B] dark:bg-[#450A0A] dark:text-[#FCA5A5]'
              : isGitReview
              ? 'bg-[#F3F4F6] text-[#4B5563] dark:bg-[#2A2A2D] dark:text-[#9B9B9F]'
              : 'bg-[#FEF3C7] text-[#92400E] dark:bg-[#451A03] dark:text-[#FDE68A]'
          }`}>
            {isGitReview ? 'GIT REVIEW' : diff.status.toUpperCase()}
          </span>
        </div>

        {/* View Controls & Action Buttons */}
        <div className="flex items-center gap-2">
          {/* Split / Unified Toggle */}
          <div className="bg-[#E5E7EB] dark:bg-[#2A2A2D] p-0.5 rounded-[7px] flex items-center text-[11px]">
            <button
              type="button"
              onClick={() => setViewMode('split')}
              className={`px-2.5 py-1 rounded-[5px] transition-colors cursor-pointer ${
                viewMode === 'split' ? 'bg-white dark:bg-[#1E1E20] text-[#111827] dark:text-[#F2F2F2] font-semibold shadow-2xs' : 'text-[#6B7280] dark:text-[#9B9B9F]'
              }`}
            >
              Split
            </button>
            <button
              type="button"
              onClick={() => setViewMode('unified')}
              className={`px-2.5 py-1 rounded-[5px] transition-colors cursor-pointer ${
                viewMode === 'unified' ? 'bg-white dark:bg-[#1E1E20] text-[#111827] dark:text-[#F2F2F2] font-semibold shadow-2xs' : 'text-[#6B7280] dark:text-[#9B9B9F]'
              }`}
            >
              Unified
            </button>
          </div>

          {/* Copy Button */}
          <button
            type="button"
            onClick={handleCopy}
            className="p-1.5 rounded-[6px] hover:bg-[#E5E7EB] dark:hover:bg-[#2A2A2D] text-[#6B7280] dark:text-[#9B9B9F] hover:text-[#111827] dark:hover:text-[#F2F2F2] transition-colors cursor-pointer"
            title="Copy diff text"
          >
            {copied ? <CheckCheck className="w-3.5 h-3.5 text-[#16A34A] dark:text-[#4ADE80]" /> : <Copy className="w-3.5 h-3.5" />}
          </button>

          {/* Open in editor tab */}
          {diff.filePath && diff.filePath !== 'Working Tree Changes' && (
            <button
              type="button"
              onClick={() => openFileInEditor(diff.filePath)}
              className="flex items-center gap-1 px-2.5 py-1 rounded-[7px] bg-[#F3F4F6] hover:bg-[#E5E7EB] text-[#4B5563] dark:bg-[#2A2A2D] dark:hover:bg-[#333336] dark:text-[#9B9B9F] font-medium text-xs transition-colors cursor-pointer"
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
              className="flex items-center gap-1 px-2.5 py-1 rounded-[7px] bg-[#EFF6FF] hover:bg-[#DBEAFE] text-[#2563EB] dark:bg-[#1E293B] dark:hover:bg-[#28394F] dark:text-[#93C5FD] font-semibold text-xs transition-colors cursor-pointer"
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
                className="flex items-center gap-1 px-2.5 py-1 rounded-[7px] bg-[#FEE2E2] hover:bg-[#FECACA] text-[#DC2626] dark:bg-[#450A0A] dark:hover:bg-[#5C0D0D] dark:text-[#FCA5A5] font-semibold text-xs transition-colors cursor-pointer"
                title="Reject changes"
              >
                <X className="w-3.5 h-3.5" />
                <span>Reject</span>
              </button>

              <button
                type="button"
                onClick={handleAccept}
                className="flex items-center gap-1 px-3 py-1 rounded-[7px] bg-[#16A34A] hover:bg-[#15803D] text-white font-semibold text-xs transition-colors shadow-2xs cursor-pointer"
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
              className="p-1 hover:bg-[#E5E7EB] dark:hover:bg-[#2A2A2D] rounded-[6px] text-[#6B7280] dark:text-[#9B9B9F] cursor-pointer"
            >
              <X className="w-4 h-4" />
            </button>
          )}
        </div>
      </div>

      {/* Diff Content Viewport */}
      <div className="flex-1 min-h-0 overflow-auto font-['JetBrains_Mono',monospace] text-[12px] leading-[20px] select-text">
        {isLoadingDiff ? (
          <div className="p-8 text-center text-xs text-[#9CA3AF] dark:text-[#6B6B70]">
            Loading repository diff...
          </div>
        ) : viewMode === 'split' ? (
          /* Split View Mode */
          <div className="w-full flex flex-col divide-y divide-[#E5E7EB] dark:divide-[#333336]">
            {/* Split Column Headers */}
            <div className="grid grid-cols-2 divide-x divide-[#E5E7EB] dark:divide-[#333336] bg-[#F9FAFB] dark:bg-[#1E1E20] border-b border-[#E5E7EB] dark:border-[#333336] text-[11px] font-bold text-[#6B7280] dark:text-[#9B9B9F] select-none sticky top-0 z-10">
              <div className="px-3 py-1">Original (Workspace)</div>
              <div className="px-3 py-1 text-[#16A34A] dark:text-[#4ADE80]">Proposed / Working Tree</div>
            </div>

            {splitRows.length === 0 ? (
              <div className="p-8 text-center text-xs text-[#9CA3AF] dark:text-[#6B6B70]">
                No difference found. Content matches cleanly.
              </div>
            ) : (
              splitRows.map((row, idx) => {
                if (row.type === 'header' || row.type === 'file-header') {
                  return (
                    <div key={idx} className="w-full px-3 py-1 bg-[#F3F4F6] dark:bg-[#202022] text-[#2563EB] dark:text-[#60A5FA] font-bold text-[11px] select-none">
                      {row.headerText}
                    </div>
                  );
                }

                const left = row.left;
                const right = row.right;
                const isDel = left?.type === 'del';
                const isAdd = right?.type === 'add';

                return (
                  <div key={idx} className="grid grid-cols-2 divide-x divide-[#E5E7EB] dark:divide-[#333336] hover:bg-[#F9FAFB]/50 dark:hover:bg-[#2A2A2D]/30 transition-colors">
                    {/* Left Pane (Original) */}
                    <div className={`flex items-start overflow-hidden ${
                      isDel ? 'bg-[#FEE2E2]/60 dark:bg-[#450A0A]/40 text-[#991B1B] dark:text-[#FCA5A5]' : 'text-[#374151] dark:text-[#CCCCCC]'
                    }`}>
                      <span className="w-10 text-right pr-2 text-[#9CA3AF] dark:text-[#555] select-none shrink-0 font-mono text-[11px]">
                        {left?.lineNum ?? ''}
                      </span>
                      <span className="w-4 text-center select-none font-bold text-[#DC2626] shrink-0">
                        {isDel ? '-' : ' '}
                      </span>
                      <span className="flex-1 whitespace-pre overflow-x-auto pr-2 font-mono">
                        {left?.text ?? ''}
                      </span>
                    </div>

                    {/* Right Pane (Modified) */}
                    <div className={`flex items-start overflow-hidden ${
                      isAdd ? 'bg-[#DCFCE7]/70 dark:bg-[#064E3B]/40 text-[#166534] dark:text-[#86EFAC]' : 'text-[#374151] dark:text-[#CCCCCC]'
                    }`}>
                      <span className="w-10 text-right pr-2 text-[#9CA3AF] dark:text-[#555] select-none shrink-0 font-mono text-[11px]">
                        {right?.lineNum ?? ''}
                      </span>
                      <span className="w-4 text-center select-none font-bold text-[#16A34A] shrink-0">
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
          /* Unified View Mode */
          <div className="p-2 space-y-0.5 bg-white dark:bg-[#161617]">
            {lines.length === 0 ? (
              <div className="p-8 text-center text-xs text-[#9CA3AF] dark:text-[#6B6B70]">
                No difference found. Working tree matches cleanly.
              </div>
            ) : (
              lines.map((l, idx) => {
                if (l.type === 'header' || l.type === 'file-header') {
                  return (
                    <div key={idx} className="px-3 py-1 bg-[#F3F4F6] dark:bg-[#202022] text-[#2563EB] dark:text-[#60A5FA] font-bold text-[11px] rounded-[4px] my-1 select-none">
                      {l.text}
                    </div>
                  );
                }
                const isAdd = l.type === 'add';
                const isDel = l.type === 'del';
                return (
                  <div
                    key={idx}
                    className={`flex items-start ${
                      isAdd 
                        ? 'bg-[#DCFCE7]/70 dark:bg-[#064E3B]/40 text-[#166534] dark:text-[#86EFAC]' 
                        : isDel 
                        ? 'bg-[#FEE2E2]/60 dark:bg-[#450A0A]/40 text-[#991B1B] dark:text-[#FCA5A5]' 
                        : 'text-[#374151] dark:text-[#CCCCCC]'
                    }`}
                  >
                    <span className="w-10 text-right pr-2 text-[#9CA3AF] dark:text-[#555] select-none shrink-0 font-mono text-[11px]">
                      {l.origLine ?? ''}
                    </span>
                    <span className="w-10 text-right pr-2 text-[#9CA3AF] dark:text-[#555] select-none shrink-0 font-mono text-[11px]">
                      {l.modLine ?? ''}
                    </span>
                    <span className={`w-4 text-center select-none font-bold ${isAdd ? 'text-[#16A34A]' : isDel ? 'text-[#DC2626]' : 'text-transparent'}`}>
                      {isAdd ? '+' : isDel ? '-' : ' '}
                    </span>
                    <span className="flex-1 whitespace-pre overflow-x-auto font-mono">{l.text || ' '}</span>
                  </div>
                );
              })
            )}
          </div>
        )}
      </div>
    </div>
  );
};
