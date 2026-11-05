import React, { useState, useMemo } from 'react';
import {
  ChevronDown,
  ChevronRight,
  Loader2,
  Check,
  AlertTriangle
} from 'lucide-react';
import { cn } from '../../lib/utils';
import type { ToolExecution } from '../../types';

interface FoldedToolTracesProps {
  tools: ToolExecution[];
  children?: React.ReactNode;
  className?: string;
  defaultExpanded?: boolean;
}

export function summarizeToolsList(tools: ToolExecution[]): {
  summary: string;
  hasRunning: boolean;
  hasFailed: boolean;
} {
  let commands = 0;
  let reads = 0;
  let edits = 0;
  let searches = 0;
  let skills = 0;
  let others = 0;
  let hasRunning = false;
  let hasFailed = false;

  for (const t of tools) {
    if (t.status === 'running') hasRunning = true;
    if (t.status === 'failed' || (t.output || '').includes('<tool_use_error>')) hasFailed = true;

    const n = (t.toolName || '').toLowerCase();
    if (n.includes('bash') || n.includes('shell') || n.includes('command') || n.includes('exec') || n.includes('terminal')) {
      commands++;
    } else if (n.includes('read') || n.includes('view') || n.includes('cat')) {
      reads++;
    } else if (n.includes('edit') || n.includes('write') || n.includes('patch') || n.includes('create')) {
      edits++;
    } else if (n.includes('search') || n.includes('grep') || n.includes('find') || n.includes('glob')) {
      searches++;
    } else if (n.includes('skill')) {
      skills++;
    } else {
      others++;
    }
  }

  const parts: string[] = [];
  if (commands > 0) parts.push(`Ran ${commands} ${commands === 1 ? 'command' : 'commands'}`);
  if (reads > 0) parts.push(`Read ${reads} ${reads === 1 ? 'file' : 'files'}`);
  if (edits > 0) parts.push(`Edited ${edits} ${edits === 1 ? 'file' : 'files'}`);
  if (searches > 0) parts.push(`Ran ${searches} ${searches === 1 ? 'search' : 'searches'}`);
  if (skills > 0) parts.push(`Used ${skills} ${skills === 1 ? 'skill' : 'skills'}`);
  if (others > 0 && parts.length === 0) parts.push(`Executed ${others} ${others === 1 ? 'tool' : 'tools'}`);

  return {
    summary: parts.join(' · ') || `${tools.length} tool calls`,
    hasRunning,
    hasFailed
  };
}

export const FoldedToolTraces: React.FC<FoldedToolTracesProps> = ({
  tools,
  children,
  className,
  defaultExpanded = true
}) => {
  const { summary, hasRunning, hasFailed } = useMemo(() => summarizeToolsList(tools), [tools]);
  const [expanded, setExpanded] = useState(defaultExpanded || hasFailed);

  if (!tools || tools.length === 0) return null;

  // Single tool: render children directly without grouping wrapper
  if (tools.length === 1) {
    return <div className={cn('w-full my-1', className)}>{children}</div>;
  }

  return (
    <div className={cn('w-full my-1.5 rounded-xl border border-border/70 bg-panel/60 shadow-2xs overflow-hidden transition-all', className)}>
      <button
        type="button"
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center justify-between px-3 py-1.5 text-ui-xs font-mono text-foreground-subtle hover:text-foreground hover:bg-surface-hover/50 transition-colors select-none cursor-pointer"
        aria-expanded={expanded}
      >
        <div className="flex items-center gap-2 min-w-0 truncate">
          {hasRunning ? (
            <Loader2 className="size-3.5 text-primary animate-spin shrink-0" />
          ) : hasFailed ? (
            <AlertTriangle className="size-3.5 text-destructive shrink-0" />
          ) : (
            <span className="flex size-4 items-center justify-center rounded-full bg-success/15 text-success shrink-0">
              <Check className="size-2.5 stroke-[2.75]" />
            </span>
          )}
          <span className={cn('truncate font-medium text-foreground/90 select-none cursor-pointer', hasRunning && 'text-primary font-semibold live-tool-shine')}>
            {summary}
          </span>
        </div>

        <div className="flex items-center gap-1.5 shrink-0 text-foreground-subtlest select-none cursor-pointer">
          <span className="rounded bg-surface border border-border/60 px-1.5 py-0.5 text-3xs font-sans font-medium text-foreground-subtle select-none cursor-pointer">
            {expanded ? `Hide ${tools.length} steps` : `Show ${tools.length} steps`}
          </span>
          {expanded ? <ChevronDown className="size-3.5" /> : <ChevronRight className="size-3.5" />}
        </div>
      </button>

      {expanded && (
        <div className="px-2.5 py-1.5 border-t border-border/40 space-y-1 bg-surface/15 animate-in fade-in duration-100">
          {children}
        </div>
      )}
    </div>
  );
};
