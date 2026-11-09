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
import { parseTwoFilesDiff } from '../diff/DiffViewer';
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
  workspacePath?: string,
  sessionDiffs?: FileDiff[]
): TurnFileChange[] {
  if (!tools || tools.length === 0) return [];

  const changeMap = new Map<string, TurnFileChange>();

  for (const t of tools) {
    const nameLower = (t.toolName || '').toLowerCase();
    const isEdit = nameLower.includes('edit') || nameLower.includes('patch') || nameLower.includes('replace');
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
    let diff: FileDiff | undefined = t.diff;

    // Check if sessionDiffs has a matching diff
    if (!diff && sessionDiffs && sessionDiffs.length > 0) {
      diff = sessionDiffs.find(d =>
        d.filePath === rawPath ||
        d.filePath.endsWith('/' + rawPath) ||
        rawPath.endsWith('/' + d.filePath)
      );
    }

    if (diff) {
      adds = typeof diff.additions === 'number' ? diff.additions : 0;
      dels = typeof diff.deletions === 'number' ? diff.deletions : 0;
      // Re-calculate real line additions and deletions if both are 0 but contents are present
      if (adds === 0 && dels === 0 && (diff.originalContent !== undefined || diff.modifiedContent !== undefined)) {
        const { lines } = parseTwoFilesDiff(diff.originalContent || '', diff.modifiedContent || '');
        adds = lines.filter(l => l.type === 'add').length;
        dels = lines.filter(l => l.type === 'del').length;
      }
    } else if (Boolean(parsedArgs.oldString || parsedArgs.newString)) {
      // Direct Target/Replacement or old_string/new_string tool execution
      const oldStr = parsedArgs.oldString || '';
      const newStr = parsedArgs.newString || '';
      const { lines } = parseTwoFilesDiff(oldStr, newStr);
      adds = lines.filter(l => l.type === 'add').length;
      dels = lines.filter(l => l.type === 'del').length;
      if (adds === 0 && dels === 0 && (oldStr || newStr)) {
        adds = newStr ? newStr.split('\n').length : 0;
        dels = oldStr ? oldStr.split('\n').length : 0;
      }
      diff = {
        id: `diff-turn-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`,
        filePath: rawPath,
        fileName,
        originalContent: oldStr,
        modifiedContent: newStr,
        additions: adds,
        deletions: dels,
        status: 'accepted',
        timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
        kind: 'agent'
      };
    } else if (isWrite) {
      const code = parsedArgs.content || '';
      adds = code ? code.split('\n').length : 1;
      dels = 0;
      diff = {
        id: `diff-turn-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`,
        filePath: rawPath,
        fileName,
        originalContent: '',
        modifiedContent: code,
        additions: adds,
        deletions: 0,
        status: 'accepted',
        timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
        kind: 'agent'
      };
    } else if (isEdit) {
      const diffText = t.output || '';
      if (diffText.includes('@@') || diffText.includes('--- ') || diffText.includes('+++ ')) {
        for (const l of diffText.split('\n')) {
          if (l.startsWith('+') && !l.startsWith('+++')) adds++;
          else if (l.startsWith('-') && !l.startsWith('---')) dels++;
        }
      }
      if (adds === 0 && dels === 0 && parsedArgs.content) {
        adds = parsedArgs.content.split('\n').length;
      }
    }

    const existing = changeMap.get(rawPath);
    if (existing) {
      existing.additions += adds;
      existing.deletions += dels;
      if (!existing.diff && diff) {
        existing.diff = diff;
      }
    } else {
      changeMap.set(rawPath, {
        filePath: rawPath,
        fileName,
        directory,
        additions: adds,
        deletions: dels,
        diff
      });
    }
  }

  return Array.from(changeMap.values());
}

interface TurnChangesCardProps {
  tools: ToolExecution[];
  workspacePath?: string;
  sessionDiffs?: FileDiff[];
  onOpenDiff?: (diff: FileDiff) => void;
  onOpenFile?: (filePath: string, diff?: FileDiff) => void;
  className?: string;
}

export const TurnChangesCard: React.FC<TurnChangesCardProps> = ({
  tools,
  workspacePath,
  sessionDiffs,
  onOpenDiff,
  onOpenFile,
  className
}) => {
  const [expanded, setExpanded] = useState(false);
  const [isReverting, setIsReverting] = useState(false);
  const [reverted, setReverted] = useState(false);
  const changes = useMemo(() => extractTurnChanges(tools, workspacePath, sessionDiffs), [tools, workspacePath, sessionDiffs]);

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
            <span className="text-diff-addition-foreground font-semibold">+{totalAdditions}</span>
            <span className="text-diff-deletion-foreground font-semibold">-{totalDeletions}</span>
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

          {(onOpenDiff || onOpenFile) && changes.length > 0 && (
            <button
              type="button"
              onClick={() => {
                const target = changes.find(c => c.diff) || changes[0];
                if (target) {
                  if (target.diff && onOpenDiff) {
                    onOpenDiff(target.diff);
                  } else if (onOpenFile) {
                    onOpenFile(target.filePath, target.diff);
                  }
                }
              }}
              className="flex items-center gap-1 px-2 py-0.5 rounded text-[11px] text-foreground-subtle hover:text-foreground hover:bg-surface-hover transition-colors cursor-pointer"
              title="Review turn diffs in sidebar"
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
                    onOpenFile(item.filePath, item.diff);
                  }
                }}
                className="flex items-center gap-2 min-w-0 truncate text-left cursor-pointer group-hover:text-primary transition-colors"
                title={`Open ${item.filePath} in sidebar diff mode`}
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
                <span className="font-mono text-2xs tabular-nums flex items-center gap-1">
                  <span className="text-diff-addition-foreground font-medium">+{item.additions}</span>
                  <span className="text-diff-deletion-foreground font-medium">-{item.deletions}</span>
                </span>
                {onOpenFile && (
                  <button
                    type="button"
                    onClick={(e) => {
                      e.stopPropagation();
                      onOpenFile(item.filePath, item.diff);
                    }}
                    className="p-1 rounded text-foreground-subtle hover:text-foreground opacity-0 group-hover:opacity-100 transition-opacity cursor-pointer"
                    title="Open file in sidebar diff mode"
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
