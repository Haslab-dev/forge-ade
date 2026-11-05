import React, { useState, useMemo } from 'react';
import {
  FileCode2,
  ChevronRight,
  ChevronDown,
  Eye,
  ExternalLink,
  FileText,
  RotateCcw,
  Loader2,
  Check
} from 'lucide-react';
import { cn } from '../../lib/utils';
import type { ToolExecution, FileDiff } from '../../types';
import { parseToolArgs, cleanArgPath } from './ToolBlockViews';
import { ApiBridge } from '../../services/apiBridge';
import { showToast } from '../../lib/toast';

export interface TurnFileChange {
  filePath: string;
  fileName: string;
  directory: string;
  additions: number;
  deletions: number;
  diff?: FileDiff;
}

export function extractTurnChanges(
  tools: ToolExecution[],
  workspacePath?: string
): TurnFileChange[] {
  if (!tools || tools.length === 0) return [];

  const changeMap = new Map<string, TurnFileChange>();

  for (const t of tools) {
    const nameLower = (t.toolName || '').toLowerCase();
    const isEdit = nameLower.includes('edit') || nameLower.includes('patch');
    const isWrite = nameLower.includes('write') || nameLower.includes('create');

    if (!isEdit && !isWrite && !t.diff) continue;

    const parsedArgs = parseToolArgs(t.command || '', t.toolName || '');
    let rawPath = t.diff?.filePath || parsedArgs.path || '';
    if (!rawPath) continue;

    if (workspacePath && rawPath.startsWith(workspacePath)) {
      rawPath = rawPath.slice(workspacePath.length).replace(/^\/+/, '');
    }
    rawPath = cleanArgPath(rawPath);
    if (!rawPath) continue;

    const parts = rawPath.split('/').filter(Boolean);
    const fileName = parts.pop() || rawPath;
    const directory = parts.join('/') || 'root';

    // Calculate additions and deletions
    let adds = 0;
    let dels = 0;

    if (t.diff) {
      adds = t.diff.additions || 0;
      dels = t.diff.deletions || 0;
    } else if (isWrite) {
      const code = parsedArgs.content || '';
      adds = code ? code.split('\n').length : 1;
    } else if (isEdit) {
      const diffText = t.output || '';
      if (diffText.includes('@@')) {
        for (const l of diffText.split('\n')) {
          if (l.startsWith('+') && !l.startsWith('+++')) adds++;
          else if (l.startsWith('-') && !l.startsWith('---')) dels++;
        }
      }
      if (adds === 0 && dels === 0) {
        adds = 1;
      }
    }

    const existing = changeMap.get(rawPath);
    if (existing) {
      existing.additions += adds;
      existing.deletions += dels;
      if (!existing.diff && t.diff) {
        existing.diff = t.diff;
      }
    } else {
      changeMap.set(rawPath, {
        filePath: rawPath,
        fileName,
        directory,
        additions: adds,
        deletions: dels,
        diff: t.diff
      });
    }
  }

  return Array.from(changeMap.values());
}

interface TurnChangesCardProps {
  tools: ToolExecution[];
  workspacePath?: string;
  onOpenDiff?: (diff: FileDiff) => void;
  onOpenFile?: (filePath: string) => void;
  className?: string;
}

export const TurnChangesCard: React.FC<TurnChangesCardProps> = ({
  tools,
  workspacePath,
  onOpenDiff,
  onOpenFile,
  className
}) => {
  const [expanded, setExpanded] = useState(false);
  const [isReverting, setIsReverting] = useState(false);
  const [reverted, setReverted] = useState(false);
  const changes = useMemo(() => extractTurnChanges(tools, workspacePath), [tools, workspacePath]);

  if (changes.length === 0) return null;

  const totalAdditions = changes.reduce((sum, c) => sum + c.additions, 0);
  const totalDeletions = changes.reduce((sum, c) => sum + c.deletions, 0);

  const handleRevertTurn = async () => {
    if (isReverting || reverted) return;
    setIsReverting(true);
    try {
      let revertedCount = 0;
      for (const change of changes) {
        if (change.diff?.originalContent !== undefined) {
          const targetPath = workspacePath ? `${workspacePath}/${change.filePath}` : change.filePath;
          await ApiBridge.writeFile(targetPath, change.diff.originalContent);
          revertedCount++;
        }
      }
      setReverted(true);
      showToast(`Reverted changes in ${revertedCount} file(s)`, 'success');
    } catch (err: any) {
      showToast(`Failed to revert: ${err?.message || 'Unknown error'}`, 'error');
    } finally {
      setIsReverting(false);
    }
  };

  return (
    <div
      data-testid="turn-changes-card"
      className={cn(
        'w-full my-2 rounded-lg border border-border/70 bg-surface/40 overflow-hidden shadow-xs transition-all',
        className
      )}
    >
      <div className="flex items-center justify-between px-3 py-1.5 bg-surface/60 border-b border-border/40 select-none">
        <button
          type="button"
          onClick={() => setExpanded(!expanded)}
          className="flex items-center gap-2 text-ui-xs font-medium text-foreground hover:text-primary transition-colors cursor-pointer min-w-0"
          aria-expanded={expanded}
        >
          {expanded ? <ChevronDown className="size-3.5 text-foreground-subtle shrink-0" /> : <ChevronRight className="size-3.5 text-foreground-subtle shrink-0" />}
          <FileCode2 className="size-3.5 text-primary shrink-0" />
          <span className="truncate">
            Modified {changes.length} {changes.length === 1 ? 'file' : 'files'} in this turn
          </span>
          <div className="flex items-center gap-1 font-mono text-2xs tabular-nums shrink-0 ml-1">
            {totalAdditions > 0 && (
              <span className="text-diff-addition-foreground font-semibold">+{totalAdditions}</span>
            )}
            {totalDeletions > 0 && (
              <span className="text-diff-deletion-foreground font-semibold">-{totalDeletions}</span>
            )}
          </div>
        </button>

        <div className="flex items-center gap-1 shrink-0">
          {changes.some(c => c.diff?.originalContent !== undefined) && (
            <button
              type="button"
              disabled={isReverting || reverted}
              onClick={handleRevertTurn}
              className={cn(
                "flex items-center gap-1 px-2 py-0.5 rounded text-2xs transition-colors cursor-pointer",
                reverted
                  ? "text-success bg-success/10 cursor-default"
                  : "text-foreground-subtle hover:text-destructive hover:bg-destructive/10"
              )}
              title={reverted ? "Turn reverted" : "Revert all file modifications in this turn"}
            >
              {isReverting ? (
                <Loader2 className="size-3 animate-spin" />
              ) : reverted ? (
                <Check className="size-3" />
              ) : (
                <RotateCcw className="size-3" />
              )}
              <span>{reverted ? 'Reverted' : 'Revert Turn'}</span>
            </button>
          )}

          {onOpenDiff && changes.some(c => c.diff) && (
            <button
              type="button"
              onClick={() => {
                const target = changes.find(c => c.diff);
                if (target?.diff) onOpenDiff(target.diff);
              }}
              className="flex items-center gap-1 px-2 py-0.5 rounded text-[11px] text-foreground-subtle hover:text-foreground hover:bg-surface-hover transition-colors cursor-pointer"
              title="Review turn diffs"
            >
              <Eye className="size-3" />
              <span>Review Diff</span>
            </button>
          )}
        </div>
      </div>

      {expanded && (
        <div className="p-2 space-y-1 bg-surface/20 animate-in fade-in duration-100">
          {changes.map(item => (
            <div
              key={item.filePath}
              className="flex items-center justify-between gap-2 px-2.5 py-1.5 rounded-md text-ui-xs hover:bg-surface-hover/80 transition-colors group"
            >
              <button
                type="button"
                onClick={() => {
                  if (item.diff && onOpenDiff) {
                    onOpenDiff(item.diff);
                  } else if (onOpenFile) {
                    onOpenFile(item.filePath);
                  }
                }}
                className="flex items-center gap-2 min-w-0 truncate text-left cursor-pointer group-hover:text-primary transition-colors"
                title={item.filePath}
              >
                <FileText className="size-3.5 text-foreground-subtle shrink-0" />
                <span className="font-mono text-xs font-medium text-foreground truncate">
                  {item.fileName}
                </span>
                <span className="font-mono text-[10px] text-foreground-subtlest truncate hidden sm:inline">
                  {item.directory}
                </span>
              </button>

              <div className="flex items-center gap-2 shrink-0">
                <span className="font-mono text-2xs tabular-nums">
                  {item.additions > 0 && (
                    <span className="text-diff-addition-foreground">+{item.additions} </span>
                  )}
                  {item.deletions > 0 && (
                    <span className="text-diff-deletion-foreground">-{item.deletions}</span>
                  )}
                </span>
                {onOpenFile && (
                  <button
                    type="button"
                    onClick={() => onOpenFile(item.filePath)}
                    className="p-1 rounded text-foreground-subtle hover:text-foreground opacity-0 group-hover:opacity-100 transition-opacity cursor-pointer"
                    title="Open file in editor"
                  >
                    <ExternalLink className="size-3" />
                  </button>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
};
