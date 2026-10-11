import React, { useState, useMemo, useRef, useEffect } from 'react';
import {
  CheckCircle2,
  ArrowRight,
  Circle,
  MoreHorizontal,
  Minimize2,
  Maximize2,
  Bot,
  Loader2,
  Copy,
  Check,
  ChevronRight,
  Sparkles,
  Trash2,
} from 'lucide-react';
import { cn } from '../../lib/utils';
import { SessionTaskItem, AgentSession } from '../../types';

interface FloatingTaskTrackerProps {
  tasks?: SessionTaskItem[];
  subagents?: AgentSession[];
  onToggleTask?: (taskId: string, completed: boolean) => void;
  onClearTasks?: () => void;
  onOpenSubagent?: (subagentId: string) => void;
  className?: string;
}

export const FloatingTaskTracker: React.FC<FloatingTaskTrackerProps> = ({
  tasks = [],
  subagents = [],
  onToggleTask,
  onClearTasks,
  onOpenSubagent,
  className,
}) => {
  const [isCollapsed, setIsCollapsed] = useState(false);
  const [menuOpen, setMenuOpen] = useState(false);
  const [copied, setCopied] = useState(false);

  // Compute stats
  const { completedCount, totalCount, inProgressCount, pendingCount } = useMemo(() => {
    let completed = 0;
    let inProgress = 0;
    let pending = 0;
    for (const t of tasks) {
      if (t.status === 'completed' || t.completed) {
        completed++;
      } else if (t.status === 'in_progress') {
        inProgress++;
      } else {
        pending++;
      }
    }
    return {
      completedCount: completed,
      totalCount: tasks.length,
      inProgressCount: inProgress,
      pendingCount: pending,
    };
  }, [tasks]);

  const runningSubagentsCount = useMemo(() => {
    return subagents.filter(
      (s) => s.status === 'running' || s.goState === 'executing' || s.goState === 'thinking'
    ).length;
  }, [subagents]);

  // Auto-expand tracker if new incomplete tasks arrive after previously being done or empty
  const prevCountRef = useRef(totalCount);
  useEffect(() => {
    if (totalCount > 0 && (inProgressCount > 0 || pendingCount > 0)) {
      if (prevCountRef.current === 0 || completedCount === prevCountRef.current) {
        setIsCollapsed(false);
      }
    }
    prevCountRef.current = totalCount;
  }, [totalCount, inProgressCount, pendingCount, completedCount]);

  // Don't render anything if there are no tasks and no subagents
  if (tasks.length === 0 && subagents.length === 0) {
    return null;
  }

  const handleCopyChecklist = () => {
    const text = tasks
      .map((t) => {
        const check = t.status === 'completed' || t.completed ? '[x]' : '[ ]';
        return `${check} ${t.title}`;
      })
      .join('\n');
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
    setMenuOpen(false);
  };

  const progressPercent = totalCount > 0 ? Math.round((completedCount / totalCount) * 100) : 0;

  // Collapsed Pill View
  if (isCollapsed) {
    return (
      <div
        className={cn(
          'absolute top-4 right-6 z-30 select-none animate-in fade-in zoom-in-95 duration-150',
          className
        )}
      >
        <button
          type="button"
          onClick={() => setIsCollapsed(false)}
          className="flex items-center gap-2 px-3 py-1.5 rounded-xl bg-panel/95 hover:bg-panel border border-border/80 backdrop-blur-md shadow-xl text-ui-xs text-foreground cursor-pointer transition-all duration-200 group"
          title="Expand task tracker"
        >
          {runningSubagentsCount > 0 ? (
            <Loader2 className="size-3.5 text-primary animate-spin" />
          ) : completedCount === totalCount && totalCount > 0 ? (
            <CheckCircle2 className="size-3.5 text-success" />
          ) : (
            <Sparkles className="size-3.5 text-foreground-subtle group-hover:text-foreground" />
          )}
          <span className="font-medium text-foreground">Progress</span>
          <span className="font-mono text-foreground-subtle">
            {completedCount}/{totalCount}
          </span>
          {runningSubagentsCount > 0 && (
            <span className="px-1.5 py-0.2 rounded-full bg-primary/15 text-primary text-ui-xs font-mono font-medium">
              {runningSubagentsCount} active
            </span>
          )}
          <Maximize2 className="size-3 text-foreground-subtlest group-hover:text-foreground ml-1 transition-colors" />
        </button>
      </div>
    );
  }

  // Expanded Card View (matching screenshot design)
  return (
    <div
      className={cn(
        'absolute top-4 right-6 z-30 w-84 max-w-[calc(100%-2rem)] select-none animate-in fade-in zoom-in-95 duration-150',
        className
      )}
    >
      <div className="rounded-2xl border border-border/80 bg-panel/95 backdrop-blur-md shadow-2xl p-3.5 text-foreground flex flex-col gap-2.5">
        
        {/* Card Header */}
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <span className="text-ui-sm font-medium text-foreground">Progress</span>
            <span className="text-ui-sm text-foreground-subtle font-mono">
              {completedCount}/{totalCount}
            </span>
            {completedCount === totalCount && totalCount > 0 && (
              <span className="px-1.5 py-0.5 rounded-md bg-success/15 text-success text-ui-xs font-medium flex items-center gap-1">
                <CheckCircle2 className="size-3" /> Done
              </span>
            )}
          </div>

          <div className="flex items-center gap-1 relative">
            {/* 3-dots more menu */}
            <div className="relative">
              <button
                type="button"
                onClick={() => setMenuOpen((v) => !v)}
                className="p-1 rounded-md text-foreground-subtle hover:text-foreground hover:bg-surface transition-colors cursor-pointer"
                title="Options"
              >
                <MoreHorizontal className="size-4" />
              </button>

              {menuOpen && (
                <div className="absolute right-0 top-full mt-1 w-44 rounded-xl border border-border bg-popover p-1 shadow-xl z-50 animate-in fade-in zoom-in-95 duration-100">
                  <button
                    type="button"
                    onClick={handleCopyChecklist}
                    className="w-full flex items-center gap-2 px-2.5 py-1.5 text-ui-xs text-foreground hover:bg-surface-hover rounded-lg text-left transition-colors cursor-pointer"
                  >
                    {copied ? (
                      <Check className="size-3.5 text-success" />
                    ) : (
                      <Copy className="size-3.5 text-foreground-subtle" />
                    )}
                    <span>{copied ? 'Copied!' : 'Copy checklist'}</span>
                  </button>
                  {onClearTasks && totalCount > 0 && (
                    <button
                      type="button"
                      onClick={() => {
                        onClearTasks();
                        setMenuOpen(false);
                      }}
                      className="w-full flex items-center gap-2 px-2.5 py-1.5 text-ui-xs text-destructive hover:bg-destructive/10 rounded-lg text-left transition-colors cursor-pointer"
                    >
                      <Trash2 className="size-3.5" />
                      <span>Clear checklist</span>
                    </button>
                  )}
                  <button
                    type="button"
                    onClick={() => {
                      setIsCollapsed(true);
                      setMenuOpen(false);
                    }}
                    className="w-full flex items-center gap-2 px-2.5 py-1.5 text-ui-xs text-foreground hover:bg-surface-hover rounded-lg text-left transition-colors cursor-pointer"
                  >
                    <Minimize2 className="size-3.5 text-foreground-subtle" />
                    <span>Minimize tracker</span>
                  </button>
                </div>
              )}
            </div>

            {/* Collapse toggle button (diagonal minimize) */}
            <button
              type="button"
              onClick={() => setIsCollapsed(true)}
              className="p-1 rounded-md text-foreground-subtle hover:text-foreground hover:bg-surface transition-colors cursor-pointer"
              title="Collapse"
            >
              <Minimize2 className="size-3.5" />
            </button>
          </div>
        </div>

        {/* Subtle Progress Bar */}
        {totalCount > 0 && (
          <div className="h-1 w-full bg-border/60 rounded-full overflow-hidden">
            <div
              className={cn(
                'h-full transition-all duration-300 rounded-full',
                completedCount === totalCount ? 'bg-success' : 'bg-primary'
              )}
              style={{ width: `${progressPercent}%` }}
            />
          </div>
        )}

        {/* Checklist Items */}
        {tasks.length > 0 && (
          <div className="space-y-2 py-0.5 max-h-60 overflow-y-auto scrollbar-thin pr-1">
            {tasks.map((task, idx) => {
              const isCompleted = task.status === 'completed' || !!task.completed;
              const isInProgress = task.status === 'in_progress';
              const displayText =
                isInProgress && task.active_form ? task.active_form : task.title;

              return (
                <div
                  key={task.id || `task-${idx}`}
                  className="group flex items-start gap-2.5 text-left py-0.5 select-text"
                >
                  {/* Status Toggle Icon */}
                  <button
                    type="button"
                    onClick={() => onToggleTask?.(task.id, !isCompleted)}
                    className="shrink-0 mt-0.5 cursor-pointer text-left transition-transform active:scale-90"
                    title={isCompleted ? 'Mark pending' : 'Mark completed'}
                  >
                    {isCompleted ? (
                      <CheckCircle2 className="size-4 text-success" />
                    ) : isInProgress ? (
                      <ArrowRight className="size-4 text-foreground animate-pulse" />
                    ) : (
                      <Circle className="size-4 text-foreground-subtlest/60 hover:text-foreground-subtle transition-colors" />
                    )}
                  </button>

                  {/* Task Text */}
                  <div
                    className={cn(
                      'text-ui-xs line-clamp-2 leading-relaxed flex-1',
                      isCompleted
                        ? 'text-foreground-subtle'
                        : isInProgress
                        ? 'font-medium text-foreground'
                        : 'text-foreground-subtle'
                    )}
                  >
                    {displayText}
                  </div>
                </div>
              );
            })}
          </div>
        )}

        {/* Section Divider & Agents List */}
        <div className="border-t border-border/60 pt-2 flex flex-col gap-2">
          <div className="flex items-center justify-between">
            <span className="text-ui-sm font-medium text-foreground-subtle">Agents</span>
            {runningSubagentsCount > 0 ? (
              <span className="px-2 py-0.5 rounded-full bg-primary/15 text-primary text-ui-xs font-mono font-medium flex items-center gap-1.5">
                <span className="size-1.5 rounded-full bg-primary animate-ping" />
                {runningSubagentsCount} running
              </span>
            ) : (
              <span className="text-ui-xs font-mono text-foreground-subtlest">
                {subagents.length} total
              </span>
            )}
          </div>

          {/* Subagents Rows */}
          {subagents.length > 0 ? (
            <div className="space-y-1.5 max-h-36 overflow-y-auto scrollbar-thin pr-1">
              {subagents.map((sub) => {
                const isRunning =
                  sub.status === 'running' ||
                  sub.goState === 'executing' ||
                  sub.goState === 'thinking';

                return (
                  <button
                    key={sub.id}
                    type="button"
                    onClick={() => onOpenSubagent?.(sub.id)}
                    className="w-full flex items-center justify-between gap-2 p-1.5 rounded-lg bg-surface hover:bg-surface-hover border border-border/40 text-left transition-colors cursor-pointer group"
                    title={`Open ${sub.title || 'Subagent'}`}
                  >
                    <div className="flex items-center gap-2 min-w-0 flex-1">
                      <div className="size-5 rounded-md bg-panel flex items-center justify-center shrink-0 border border-border/50">
                        {isRunning ? (
                          <Loader2 className="size-3 text-primary animate-spin" />
                        ) : (
                          <Bot className="size-3 text-foreground-subtle group-hover:text-foreground transition-colors" />
                        )}
                      </div>
                      <span className="text-ui-xs font-medium text-foreground truncate">
                        {sub.title || 'Sub-agent'}
                      </span>
                    </div>

                    <div className="flex items-center gap-1 shrink-0">
                      <span
                        className={cn(
                          'text-ui-xs font-mono px-1.5 py-0.2 rounded-full uppercase tracking-wider',
                          isRunning
                            ? 'bg-primary/20 text-primary'
                            : 'bg-border/60 text-foreground-subtlest'
                        )}
                      >
                        {isRunning ? 'running' : sub.goState || sub.status || 'idle'}
                      </span>
                      <ChevronRight className="size-3 text-foreground-subtlest group-hover:text-foreground transition-colors" />
                    </div>
                  </button>
                );
              })}
            </div>
          ) : (
            <div className="text-ui-xs text-foreground-subtlest py-0.5">
              No active child agents
            </div>
          )}
        </div>

      </div>
    </div>
  );
};
