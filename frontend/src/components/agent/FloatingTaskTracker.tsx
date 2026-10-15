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
  ChevronDown,
  Sparkles,
  Trash2,
  Terminal,
  SquareTerminal,
  Square,
  Clock,
  FileText,
  X
} from 'lucide-react';
import { cn } from '../../lib/utils';
import {
  SessionTaskItem,
  AgentSession,
  BackgroundTaskItem,
  AgentScheduleItem
} from '../../types';
import { GetBackgroundTaskOutput } from '../../lib/wails';

export interface FloatingTaskTrackerProps {
  tasks?: SessionTaskItem[];
  subagents?: AgentSession[];
  backgroundTasks?: BackgroundTaskItem[];
  schedules?: AgentScheduleItem[];
  onToggleTask?: (taskId: string, completed: boolean) => void;
  onClearTasks?: () => void;
  onOpenSubagent?: (subagentId: string) => void;
  onOpenTerminal?: (taskId?: string, cwd?: string, command?: string, outputFile?: string) => void;
  onStopBackgroundTask?: (taskId: string) => void;
  onCancelSchedule?: (scheduleId: string) => void;
  className?: string;
}

export const FloatingTaskTracker: React.FC<FloatingTaskTrackerProps> = ({
  tasks = [],
  subagents = [],
  backgroundTasks = [],
  schedules = [],
  onToggleTask,
  onClearTasks,
  onOpenSubagent,
  onOpenTerminal,
  onStopBackgroundTask,
  onCancelSchedule,
  className
}) => {
  const [isCollapsed, setIsCollapsed] = useState(false);
  const [menuOpen, setMenuOpen] = useState(false);
  const [copied, setCopied] = useState(false);
  const [expandedLogId, setExpandedLogId] = useState<string | null>(null);
  const [fetchedLogs, setFetchedLogs] = useState<Record<string, string>>({});
  const [, setTick] = useState(0);

  // Live countdown ticker only when active schedules/timers exist
  const hasActiveSchedules = schedules.some((s) => s.active);
  useEffect(() => {
    if (!hasActiveSchedules) return;
    const timer = setInterval(() => {
      setTick((t) => t + 1);
    }, 1000);
    return () => clearInterval(timer);
  }, [hasActiveSchedules]);

  // Compute checklist stats
  const { completedCount, totalCount, inProgressCount, pendingCount } =
    useMemo(() => {
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
        pendingCount: pending
      };
    }, [tasks]);

  const runningSubagentsCount = useMemo(() => {
    return subagents.filter(
      (s) =>
        s.status === 'running' ||
        s.goState === 'executing' ||
        s.goState === 'thinking'
    ).length;
  }, [subagents]);

  const runningBgCount = useMemo(() => {
    return backgroundTasks.filter((t) => t.status === 'running').length;
  }, [backgroundTasks]);

  const activeSchedules = useMemo(() => {
    return schedules.filter((s) => s.active);
  }, [schedules]);

  // Auto-expand tracker if new incomplete tasks or background tasks arrive
  const prevCountRef = useRef(totalCount);
  const prevBgCountRef = useRef(runningBgCount);
  useEffect(() => {
    if (
      (totalCount > 0 && (inProgressCount > 0 || pendingCount > 0)) ||
      runningBgCount > prevBgCountRef.current ||
      activeSchedules.length > 0
    ) {
      if (
        prevCountRef.current === 0 ||
        completedCount === prevCountRef.current ||
        runningBgCount > prevBgCountRef.current
      ) {
        setIsCollapsed(false);
      }
    }
    prevCountRef.current = totalCount;
    prevBgCountRef.current = runningBgCount;
  }, [
    totalCount,
    inProgressCount,
    pendingCount,
    completedCount,
    runningBgCount,
    activeSchedules.length
  ]);

  // Don't render anything if there are no tasks, subagents, background tasks, or active schedules
  if (
    tasks.length === 0 &&
    subagents.length === 0 &&
    backgroundTasks.length === 0 &&
    schedules.length === 0
  ) {
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

  const progressPercent =
    totalCount > 0 ? Math.round((completedCount / totalCount) * 100) : 0;

  // Format countdown for timers
  const formatCountdown = (nextRunStr: string) => {
    const diff = Math.max(
      0,
      Math.floor((new Date(nextRunStr).getTime() - Date.now()) / 1000)
    );
    if (diff <= 0) return 'Triggering now...';
    if (diff < 60) return `${diff}s remaining`;
    const m = Math.floor(diff / 60);
    const s = diff % 60;
    return `${m}m ${s}s remaining`;
  };

  // ── Collapsed Pill View ──────────────────────────────────────────────
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
          {runningBgCount > 0 ? (
            <span className="flex items-center gap-1.5 text-primary font-medium">
              <span className="size-2 rounded-full bg-primary animate-pulse" />
              <span>
                {runningBgCount === 1 ? '1 Server Active' : `${runningBgCount} Servers`}
              </span>
            </span>
          ) : activeSchedules.length > 0 ? (
            <span className="flex items-center gap-1.5 text-foreground-subtle font-medium">
              <Clock className="size-3.5 text-foreground-subtle animate-spin" />
              <span>Timer Active</span>
            </span>
          ) : runningSubagentsCount > 0 ? (
            <Loader2 className="size-3.5 text-primary animate-spin" />
          ) : completedCount === totalCount && totalCount > 0 ? (
            <CheckCircle2 className="size-3.5 text-success" />
          ) : (
            <Sparkles className="size-3.5 text-foreground-subtle group-hover:text-foreground" />
          )}

          {totalCount > 0 && (
            <span className="font-mono text-foreground-subtle">
              {completedCount}/{totalCount}
            </span>
          )}

          {runningSubagentsCount > 0 && (
            <span className="px-1.5 py-0.2 rounded-full bg-primary/15 text-primary text-ui-xs font-mono font-medium">
              {runningSubagentsCount} agent
            </span>
          )}

          <Maximize2 className="size-3 text-foreground-subtlest group-hover:text-foreground ml-1 transition-colors" />
        </button>
      </div>
    );
  }

  // ── Expanded Card View ──────────────────────────────────────────────
  return (
    <div
      className={cn(
        'absolute top-4 right-6 z-30 w-88 max-w-[calc(100%-2rem)] select-none animate-in fade-in zoom-in-95 duration-150',
        className
      )}
    >
      <div className="rounded-2xl border border-border/80 bg-panel/95 backdrop-blur-md shadow-2xl p-3.5 text-foreground flex flex-col gap-3">
        {/* Card Header */}
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <span className="text-ui-sm font-medium text-foreground">
              Runtime & Tasks
            </span>
            {totalCount > 0 && (
              <span className="text-ui-sm text-foreground-subtle font-mono">
                {completedCount}/{totalCount}
              </span>
            )}
            {runningBgCount > 0 && (
              <span className="px-1.5 py-0.5 rounded-md bg-surface text-foreground font-mono text-ui-xs flex items-center gap-1.5 border border-border/60">
                <span className="size-1.5 rounded-full bg-emerald-500 animate-pulse" />
                {runningBgCount} active
              </span>
            )}
            {completedCount === totalCount && totalCount > 0 && (
              <span className="px-1.5 py-0.5 rounded-md bg-surface text-foreground-subtle text-ui-xs font-medium flex items-center gap-1 border border-border/60">
                <CheckCircle2 className="size-3 text-foreground-subtle" /> Done
              </span>
            )}
          </div>

          <div className="flex items-center gap-1 relative">
            {/* 3-dots options menu */}
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
                <div className="absolute right-0 top-full mt-1 w-48 rounded-xl border border-border bg-popover p-1 shadow-xl z-50 animate-in fade-in zoom-in-95 duration-100">
                  {onOpenTerminal && (
                    <button
                      type="button"
                      onClick={() => {
                        onOpenTerminal();
                        setMenuOpen(false);
                      }}
                      className="w-full flex items-center gap-2 px-2.5 py-1.5 text-ui-xs text-foreground hover:bg-surface-hover rounded-lg text-left transition-colors cursor-pointer"
                    >
                      <SquareTerminal className="size-3.5 text-foreground-subtle" />
                      <span>Open Sidebar Terminal</span>
                    </button>
                  )}
                  {tasks.length > 0 && (
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
                  )}
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

            {/* Collapse button */}
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

        {/* Progress Bar */}
        {totalCount > 0 && (
          <div className="h-1 w-full bg-border/60 rounded-full overflow-hidden">
            <div
              className={cn(
                'h-full transition-all duration-300 rounded-full bg-foreground',
                completedCount === totalCount && 'opacity-80'
              )}
              style={{ width: `${progressPercent}%` }}
            />
          </div>
        )}

        {/* ── Section 1: Checklist Tasks ──────────────────────────────── */}
        {tasks.length > 0 && (
          <div className="space-y-2 py-0.5 max-h-52 overflow-y-auto pr-1">
            {tasks.map((task, idx) => {
              const isCompleted =
                task.status === 'completed' || !!task.completed;
              const isInProgress = task.status === 'in_progress';
              const displayText =
                isInProgress && task.active_form
                  ? task.active_form
                  : task.title;

              return (
                <div
                  key={task.id || `task-${idx}`}
                  className="group flex items-start gap-2.5 text-left py-0.5 select-text"
                >
                  <button
                    type="button"
                    onClick={() => onToggleTask?.(task.id, !isCompleted)}
                    className="shrink-0 mt-0.5 cursor-pointer text-left transition-transform active:scale-90"
                    title={isCompleted ? 'Mark pending' : 'Mark completed'}
                  >
                    {isCompleted ? (
                      <CheckCircle2 className="size-4 text-foreground-subtle" />
                    ) : isInProgress ? (
                      <ArrowRight className="size-4 text-foreground animate-pulse" />
                    ) : (
                      <Circle className="size-4 text-foreground-subtlest/60 hover:text-foreground-subtle transition-colors" />
                    )}
                  </button>

                  <div
                    className={cn(
                      'text-ui-xs line-clamp-2 leading-relaxed flex-1',
                      isCompleted
                        ? 'text-foreground-subtle line-through'
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

        {/* ── Section 2: Background Tasks & Servers ────────────────────── */}
        {backgroundTasks.length > 0 && (
          <div className="border-t border-border/60 pt-2.5 flex flex-col gap-2">
            <div className="flex items-center justify-between">
              <span className="text-ui-xs font-semibold text-foreground-subtle flex items-center gap-1.5">
                <Terminal className="size-3.5 text-foreground-subtle" />
                <span>Background Servers & Tasks</span>
              </span>
              <span className="text-ui-xs font-mono text-foreground-subtlest">
                {backgroundTasks.length} total
              </span>
            </div>

            <div className="space-y-1.5 max-h-48 overflow-y-auto pr-1">
              {backgroundTasks.map((bt) => {
                const taskId = bt.id || bt.task_id || '';
                const isRunning = bt.status === 'running';
                const isExpanded = expandedLogId === taskId;

                return (
                  <div
                    key={taskId}
                    className="rounded-xl border border-border/60 bg-surface/60 p-2 flex flex-col gap-1.5 text-ui-xs transition-colors"
                  >
                    <div className="flex items-center justify-between gap-2">
                      <div className="flex items-center gap-1.5 min-w-0 flex-1">
                        <span
                          className={cn(
                            'size-2 shrink-0 rounded-full',
                            isRunning
                              ? 'bg-emerald-500 animate-pulse'
                              : bt.status === 'completed'
                              ? 'bg-foreground-subtle'
                              : 'bg-destructive'
                          )}
                        />
                        <span
                          className="font-mono font-medium text-foreground truncate"
                          title={bt.command}
                        >
                          {bt.command}
                        </span>
                      </div>

                      <span className="font-mono text-[10px] px-1.5 py-0.5 rounded bg-background border border-border/50 text-foreground-subtle shrink-0 uppercase">
                        {bt.status}
                      </span>
                    </div>

                    {/* Action buttons row */}
                    <div className="flex items-center justify-between pt-1 border-t border-border/40 text-[11px]">
                      <button
                        type="button"
                        onClick={() => {
                          if (taskId) {
                            navigator.clipboard.writeText(taskId);
                          }
                        }}
                        className="text-foreground-subtlest hover:text-foreground font-mono truncate max-w-[120px] cursor-pointer transition-colors text-left"
                        title={`Click to copy task ID: ${taskId}`}
                      >
                        id: {taskId}
                      </button>

                      <div className="flex items-center gap-1 shrink-0">
                        {/* Open shell in right sidebar */}
                        {onOpenTerminal && (
                          <button
                            type="button"
                            onClick={() => onOpenTerminal(taskId, bt.cwd, bt.command, bt.output_file)}
                            className="flex items-center gap-1 px-2 py-0.5 rounded-md bg-surface-hover hover:bg-surface-active text-foreground transition-colors cursor-pointer"
                            title="Open interactive shell session in right sidebar"
                          >
                            <SquareTerminal className="size-3 text-foreground-subtle" />
                            <span>Shell</span>
                          </button>
                        )}

                        {/* Toggle output log */}
                        <button
                          type="button"
                          onClick={() => {
                            const next = isExpanded ? null : taskId;
                            setExpandedLogId(next);
                            if (next && !bt.output_tail) {
                              GetBackgroundTaskOutput(taskId)
                                .then(res => {
                                  if (res?.output_tail) {
                                    setFetchedLogs(prev => ({ ...prev, [taskId]: res.output_tail }));
                                  }
                                })
                                .catch(() => {});
                            }
                          }}
                          className="flex items-center gap-1 px-2 py-0.5 rounded-md hover:bg-surface-hover text-foreground-subtle hover:text-foreground transition-colors cursor-pointer"
                          title="View live log output"
                        >
                          <FileText className="size-3" />
                          <span>Logs</span>
                          {isExpanded ? (
                            <ChevronDown className="size-3" />
                          ) : (
                            <ChevronRight className="size-3" />
                          )}
                        </button>

                        {/* Stop button */}
                        {isRunning && onStopBackgroundTask && (
                          <button
                            type="button"
                            onClick={() => onStopBackgroundTask(taskId)}
                            className="flex items-center gap-1 px-1.5 py-0.5 rounded-md hover:bg-destructive/10 text-destructive transition-colors cursor-pointer"
                            title="Stop background process"
                          >
                            <Square className="size-3" />
                            <span>Stop</span>
                          </button>
                        )}
                      </div>
                    </div>

                    {/* Output Log Drawer */}
                    {isExpanded && (
                      <div className="mt-1 rounded-lg border border-border/80 bg-background p-2 font-mono text-[10.5px] leading-relaxed text-foreground-subtle max-h-36 overflow-y-auto whitespace-pre-wrap select-text">
                        {bt.output_tail ||
                          fetchedLogs[taskId] ||
                          (isRunning
                            ? 'Streaming stdout/stderr output from process...'
                            : 'No output captured')}
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          </div>
        )}

        {/* ── Section 3: Scheduled Checks & Crons ─────────────────────── */}
        {activeSchedules.length > 0 && (
          <div className="border-t border-border/60 pt-2.5 flex flex-col gap-2">
            <div className="flex items-center justify-between">
              <span className="text-ui-xs font-semibold text-foreground-subtle flex items-center gap-1.5">
                <Clock className="size-3.5 text-foreground-subtle" />
                <span>Scheduled Alarms & Crons</span>
              </span>
              <span className="text-ui-xs font-mono text-foreground-subtlest">
                {activeSchedules.length} active
              </span>
            </div>

            <div className="space-y-1.5 max-h-36 overflow-y-auto pr-1">
              {activeSchedules.map((sc) => (
                <div
                  key={sc.id}
                  className="rounded-xl border border-border/60 bg-surface/60 p-2 flex flex-col gap-1 text-ui-xs"
                >
                  <div className="flex items-center justify-between gap-2">
                    <div className="flex items-center gap-1.5 min-w-0 flex-1">
                      <Clock className="size-3.5 text-foreground-subtle shrink-0" />
                      <span className="text-ui-xs font-medium text-foreground truncate">
                        {sc.prompt}
                      </span>
                    </div>

                    {onCancelSchedule && (
                      <button
                        type="button"
                        onClick={() => onCancelSchedule(sc.id)}
                        className="p-1 rounded hover:bg-destructive/10 text-foreground-subtle hover:text-destructive transition-colors cursor-pointer shrink-0"
                        title="Cancel schedule"
                      >
                        <X className="size-3" />
                      </button>
                    )}
                  </div>

                  <div className="flex items-center justify-between text-[11px] text-foreground-subtle font-mono pt-0.5">
                    <span>{formatCountdown(sc.next_run)}</span>
                    {sc.task_id && (
                      <span className="text-foreground-subtlest">
                        task: {sc.task_id}
                      </span>
                    )}
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* ── Section 4: Child Agents ─────────────────────────────────── */}
        {subagents.length > 0 && (
          <div className="border-t border-border/60 pt-2 flex flex-col gap-2">
            <div className="flex items-center justify-between">
              <span className="text-ui-sm font-medium text-foreground-subtle">
                Agents
              </span>
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

            <div className="space-y-1.5 max-h-36 overflow-y-auto pr-1">
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
                        {isRunning
                          ? 'running'
                          : sub.goState || sub.status || 'idle'}
                      </span>
                      <ChevronRight className="size-3 text-foreground-subtlest group-hover:text-foreground transition-colors" />
                    </div>
                  </button>
                );
              })}
            </div>
          </div>
        )}
      </div>
    </div>
  );
};

export default FloatingTaskTracker;
