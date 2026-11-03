import React, { useState, useEffect, useCallback, useMemo } from 'react';
import {
  ShieldAlert,
  Check,
  X,
  ChevronDown,
  ChevronUp,
  Loader2,
  Terminal,
  FilePen,
  FilePlus,
  Wrench,
  Search,
  CheckCheck
} from 'lucide-react';
import { cn } from '../../lib/utils';
import { RespondAgentApproval } from '../../lib/wails';

export interface PendingToolCall {
  tool_call_id?: string;
  id?: string;
  name?: string;
  arguments?: string | Record<string, any>;
  summary?: string;
}

interface AgentApprovalBarProps {
  goSessionId?: string;
  tools: PendingToolCall[];
  onResolved?: () => void;
  className?: string;
}

function summarizeTool(tool: PendingToolCall): { summary: string; icon: React.ReactNode; fullArgs: string } {
  const name = tool?.name || 'tool';
  let argsObj: any = {};
  let fullArgs = '';

  if (typeof tool?.arguments === 'string') {
    fullArgs = tool.arguments;
    try {
      argsObj = JSON.parse(tool.arguments);
    } catch {
      argsObj = {};
    }
  } else if (tool?.arguments && typeof tool.arguments === 'object') {
    argsObj = tool.arguments;
    fullArgs = JSON.stringify(tool.arguments, null, 2);
  }

  const path = argsObj.path || argsObj.filePath || argsObj.filepath || argsObj.file || argsObj.filename || '';
  const cmd = argsObj.command || argsObj.cmd || argsObj.script || argsObj.CommandLine || '';

  if (name.includes('terminal') || name.includes('bash') || name.includes('shell') || name.includes('command') || cmd) {
    const cleanCmd = cmd.trim().split('\n')[0] || name;
    return {
      summary: `Run command: ${cleanCmd.length > 80 ? cleanCmd.slice(0, 77) + '…' : cleanCmd}`,
      icon: <Terminal className="size-3.5 text-warning shrink-0" />,
      fullArgs
    };
  }

  if (name.includes('edit') || name.includes('patch') || name.includes('str_replace')) {
    const fileBasename = path.split(/[/\\]/).pop() || path || 'file';
    return {
      summary: `Edit ${fileBasename}`,
      icon: <FilePen className="size-3.5 text-warning shrink-0" />,
      fullArgs
    };
  }

  if (name.includes('write') || name.includes('create')) {
    const fileBasename = path.split(/[/\\]/).pop() || path || 'file';
    return {
      summary: `Write ${fileBasename}`,
      icon: <FilePlus className="size-3.5 text-warning shrink-0" />,
      fullArgs
    };
  }

  if (name.includes('search') || name.includes('grep') || name.includes('find')) {
    return {
      summary: `Search codebase: ${argsObj.query || path || name}`,
      icon: <Search className="size-3.5 text-warning shrink-0" />,
      fullArgs
    };
  }

  return {
    summary: `Execute ${name}`,
    icon: <Wrench className="size-3.5 text-warning shrink-0" />,
    fullArgs: fullArgs || JSON.stringify(argsObj, null, 2)
  };
}

export const AgentApprovalBar: React.FC<AgentApprovalBarProps> = ({
  goSessionId,
  tools = [],
  onResolved,
  className
}) => {
  const [busy, setBusy] = useState(false);
  const [expanded, setExpanded] = useState(false);
  const [detailIdx, setDetailIdx] = useState<number | null>(null);

  const handleResponse = useCallback(async (approve: boolean, autoAll: boolean) => {
    if (!goSessionId || busy) return;
    setBusy(true);
    try {
      await RespondAgentApproval(goSessionId, approve, autoAll);
      onResolved?.();
    } catch (err) {
      console.error('RespondAgentApproval failed:', err);
    } finally {
      setBusy(false);
    }
  }, [goSessionId, busy, onResolved]);

  // Keyboard shortcuts: ⌘+Y or ⌘+Shift+Enter to approve, Esc to deny
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (busy || tools.length === 0) return;
      if ((e.metaKey || e.ctrlKey) && (e.key === 'y' || (e.shiftKey && e.key === 'Enter'))) {
        e.preventDefault();
        void handleResponse(true, false);
      } else if (e.key === 'Escape' && !e.metaKey && !e.ctrlKey) {
        // Only deny if expanded is closed or second escape
        if (expanded) {
          setExpanded(false);
        } else {
          void handleResponse(false, false);
        }
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [busy, tools.length, expanded, handleResponse]);

  const summaries = useMemo(() => tools.map(summarizeTool), [tools]);

  if (!tools || tools.length === 0) return null;

  // Single approval request
  if (tools.length === 1) {
    const single = summaries[0];
    const isShowingDetails = detailIdx === 0;

    return (
      <div
        className={cn(
          'w-full bg-warning/10 border-t border-b border-warning/30 px-3.5 py-1.5 backdrop-blur-md transition-all select-none',
          className
        )}
        role="alert"
        aria-label="Agent tool approval requested"
      >
        <div className="flex items-center justify-between gap-3 text-ui-xs">
          <div className="flex items-center gap-2 min-w-0 flex-1">
            <ShieldAlert className="size-3.5 text-warning shrink-0" />
            <span className="font-semibold text-warning shrink-0">Approval Needed:</span>
            <div className="flex items-center gap-1.5 truncate text-foreground">
              {single.icon}
              <button
                type="button"
                onClick={() => setDetailIdx(isShowingDetails ? null : 0)}
                className="truncate font-mono hover:underline text-left cursor-pointer"
                title={single.summary}
              >
                {single.summary}
              </button>
            </div>
            {single.fullArgs && (
              <button
                type="button"
                onClick={() => setDetailIdx(isShowingDetails ? null : 0)}
                className="text-[10px] text-foreground-subtle hover:text-foreground px-1 py-0.5 rounded border border-border/50 shrink-0 cursor-pointer"
              >
                {isShowingDetails ? 'Hide details' : 'Details'}
              </button>
            )}
          </div>

          <div className="flex items-center gap-1.5 shrink-0">
            <button
              type="button"
              disabled={busy}
              onClick={() => handleResponse(false, false)}
              className="px-2 py-0.5 rounded text-ui-xs text-foreground-subtle hover:text-foreground hover:bg-surface-hover border border-border cursor-pointer disabled:opacity-50 flex items-center gap-1 transition-colors"
              title="Deny this action (Esc)"
            >
              <X className="size-3" />
              <span>Deny</span>
            </button>
            <button
              type="button"
              disabled={busy}
              onClick={() => handleResponse(true, true)}
              className="px-2 py-0.5 rounded text-ui-xs text-warning hover:bg-warning/20 border border-warning/30 cursor-pointer disabled:opacity-50 flex items-center gap-1 transition-colors"
              title="Always allow this and future tools in this session"
            >
              <CheckCheck className="size-3" />
              <span className="hidden sm:inline">Always allow</span>
            </button>
            <button
              type="button"
              disabled={busy}
              onClick={() => handleResponse(true, false)}
              className="px-2.5 py-0.5 rounded text-ui-xs font-semibold bg-warning text-black hover:bg-warning/90 cursor-pointer disabled:opacity-50 flex items-center gap-1 transition-all shadow-xs"
              title="Approve this action (⌘+Y)"
            >
              {busy ? <Loader2 className="size-3 animate-spin" /> : <Check className="size-3 stroke-[2.5]" />}
              <span>Approve</span>
            </button>
          </div>
        </div>

        {isShowingDetails && single.fullArgs && (
          <div className="mt-2 pt-2 border-t border-warning/20">
            <pre className="max-h-48 overflow-y-auto font-mono text-[11px] p-2 rounded bg-surface/80 border border-border text-foreground-subtle whitespace-pre-wrap break-all scrollbar-thin">
              {single.fullArgs}
            </pre>
          </div>
        )}
      </div>
    );
  }

  // Multiple approval requests (grouped hairline list)
  return (
    <div
      className={cn(
        'w-full bg-warning/10 border-t border-b border-warning/30 px-3.5 py-1.5 backdrop-blur-md transition-all select-none',
        className
      )}
      role="alert"
      aria-label={`${tools.length} agent tool approvals requested`}
    >
      <div className="flex items-center justify-between gap-3 text-ui-xs">
        <button
          type="button"
          onClick={() => setExpanded(!expanded)}
          className="flex items-center gap-2 min-w-0 flex-1 text-left cursor-pointer hover:opacity-85"
        >
          <ShieldAlert className="size-3.5 text-warning shrink-0" />
          <span className="font-semibold text-warning shrink-0">
            {tools.length} actions require approval:
          </span>
          <span className="truncate text-foreground-subtle">
            {summaries[0]?.summary} and {tools.length - 1} more
          </span>
          {expanded ? <ChevronUp className="size-3.5 text-warning shrink-0" /> : <ChevronDown className="size-3.5 text-warning shrink-0" />}
        </button>

        <div className="flex items-center gap-1.5 shrink-0">
          <button
            type="button"
            disabled={busy}
            onClick={() => handleResponse(false, false)}
            className="px-2 py-0.5 rounded text-ui-xs text-foreground-subtle hover:text-foreground hover:bg-surface-hover border border-border cursor-pointer disabled:opacity-50 flex items-center gap-1 transition-colors"
            title="Deny all requests"
          >
            <X className="size-3" />
            <span>Deny all</span>
          </button>
          <button
            type="button"
            disabled={busy}
            onClick={() => handleResponse(true, true)}
            className="px-2 py-0.5 rounded text-ui-xs text-warning hover:bg-warning/20 border border-warning/30 cursor-pointer disabled:opacity-50 flex items-center gap-1 transition-colors"
            title="Always allow all tools in this session"
          >
            <CheckCheck className="size-3" />
            <span className="hidden sm:inline">Always allow</span>
          </button>
          <button
            type="button"
            disabled={busy}
            onClick={() => handleResponse(true, false)}
            className="px-2.5 py-0.5 rounded text-ui-xs font-semibold bg-warning text-black hover:bg-warning/90 cursor-pointer disabled:opacity-50 flex items-center gap-1 transition-all shadow-xs"
            title="Approve all actions (⌘+Y)"
          >
            {busy ? <Loader2 className="size-3 animate-spin" /> : <Check className="size-3 stroke-[2.5]" />}
            <span>Approve all</span>
          </button>
        </div>
      </div>

      {expanded && (
        <div className="mt-2 pt-2 border-t border-warning/20 space-y-1.5 max-h-60 overflow-y-auto scrollbar-thin pr-1">
          {summaries.map((s, idx) => {
            const isDetailOpen = detailIdx === idx;
            return (
              <div
                key={tools[idx]?.id || tools[idx]?.tool_call_id || idx}
                className="flex flex-col rounded bg-surface/60 border border-border/60 p-1.5 text-[11px]"
              >
                <div className="flex items-center justify-between gap-2">
                  <div className="flex items-center gap-1.5 min-w-0 truncate">
                    {s.icon}
                    <span className="font-mono text-foreground truncate">{s.summary}</span>
                  </div>
                  {s.fullArgs && (
                    <button
                      type="button"
                      onClick={() => setDetailIdx(isDetailOpen ? null : idx)}
                      className="text-[10px] text-foreground-subtle hover:text-foreground underline cursor-pointer shrink-0"
                    >
                      {isDetailOpen ? 'Hide' : 'Inspect'}
                    </button>
                  )}
                </div>
                {isDetailOpen && s.fullArgs && (
                  <pre className="mt-1.5 p-1.5 rounded bg-surface border border-border text-[10px] font-mono text-foreground-subtle whitespace-pre-wrap break-all">
                    {s.fullArgs}
                  </pre>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
};
