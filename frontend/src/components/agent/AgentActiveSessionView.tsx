import React, { useState, useRef, useEffect, useMemo, useCallback } from 'react';
import { 
  Folder, 
  ChevronDown, 
  ChevronRight, 
  PanelRight, 
  MoreHorizontal, 
  GitBranch, 
  Sparkles, 
  CheckCircle2, 
  Bot,
  Loader2, 
  Code2, 
  ExternalLink, 
  FileCode, 
  X,
  Copy,
  Check,
  Brain,
  SquareTerminal,
  FilePen,
  FilePlus,
  Wrench,
  Search,
  ThumbsUp,
  ThumbsDown,
  AlertCircle,
  FileDiff as FileDiffIcon,
  Image as ImageIcon,
  RotateCcw,
  ArrowDown,
  ArrowRight,
  GitFork
} from 'lucide-react';
import { ProviderIcon } from './ProviderIcon';
import { WorkLogList, WorkLogBlock } from './WorkLog';
import { estimateTokenCount, formatElapsedSeconds } from './artifactsUtils';
import { useWorkspace } from '../../stores/workspaceStore';
import { cn } from '../../lib/utils';
import { 
  EventsOn, 
  RespondAgentApproval, 
  RespondAgentAsk,
  ACPRespondPermissionOption,
  ListBackgroundTasks,
  StopBackgroundTask,
  ListAgentSchedules,
  CancelAgentSchedule,
  RestoreSessionCheckpoint
} from '../../lib/wails';
import { AgentQuestionModal } from './AgentQuestionModal';
import { ApiBridge } from '../../services/apiBridge';
import { useAgentExecution, useAgentThreadSearch } from './hooks';
import { getChronologicalTimeline, type TimelineItem } from '../../lib/timeline';
import { AgentApprovalBar } from './AgentApprovalBar';
import { FoldedToolTraces } from './FoldedToolTraces';
import { TurnChangesCard } from './TurnChangesCard';
import { showToast } from '../../lib/toast';

// Chat message content is always a string in the frontend message model;
// coerce defensively so a block-shaped message (Go harness transcript) can
// never crash the renderer.
function chatText(content: unknown): string {
  if (typeof content === 'string') return content;
  if (Array.isArray(content)) {
    return content
      .filter((b: any) => b && b.type === 'text' && typeof b.text === 'string')
      .map((b: any) => b.text)
      .join('');
  }
  return content ? String(content) : '';
}

import { formatRelativeTime } from '../../lib/time';
import { FileDiff, ToolExecution, ThoughtStep, AgentMessage, BackgroundTaskItem, AgentScheduleItem } from '../../types';
import { getFileIcon } from '../../lib/file-icons';

// ---------------------------------------------------------------------------
// Go harness pause cards — shown when the backend agent loop pauses the turn
// for tool approval (mutating tools) or structured questions (`ask` tool).
// ---------------------------------------------------------------------------

function PendingApprovalCard({ goSessionId, tools }: { goSessionId: string; tools: any[] }) {
  const [busy, setBusy] = useState(false);
  const respond = async (approve: boolean, autoAll: boolean) => {
    if (busy) return;
    setBusy(true);
    try {
      await RespondAgentApproval(goSessionId, approve, autoAll);
    } catch (err) {
      console.error('approval failed:', err);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="w-full max-h-[min(65vh,480px)] flex flex-col rounded-xl border border-warning/40 bg-warning/5 p-3.5 shadow-md overflow-hidden">
      <div className="shrink-0 flex items-center justify-between pb-2 border-b border-warning/20">
        <div className="flex items-center gap-2 text-ui-sm font-semibold text-warning">
          <AlertCircle className="size-4 shrink-0" />
          <span>Permission required — the agent wants to run:</span>
        </div>
        <span className="text-ui-xs font-mono text-warning/80">
          {tools.length} {tools.length === 1 ? 'action' : 'actions'}
        </span>
      </div>
      <div className="flex-1 min-h-0 overflow-y-auto space-y-2 py-2.5 pr-1.5 scrollbar-thin">
        {tools.map((tc: any, i: number) => {
          const name = tc?.name || 'tool';
          let argsText = tc?.arguments ?? '{}';
          if (typeof argsText !== 'string') argsText = JSON.stringify(argsText, null, 2);
          return (
            <div key={tc?.tool_call_id || i} className="text-ui-xs font-mono bg-surface border border-border rounded-md p-2.5 overflow-x-auto whitespace-pre-wrap break-words">
              <div className="font-semibold text-warning mb-1">{name}</div>
              <div className="text-foreground-subtle">{argsText}</div>
            </div>
          );
        })}
      </div>
      <div className="shrink-0 flex items-center justify-between pt-2 border-t border-warning/20">
        <span className="text-ui-xs text-foreground-subtle hidden sm:inline">Review actions before continuing</span>
        <div className="flex items-center gap-2 ml-auto">
          <button
            type="button"
            onClick={() => respond(false, false)}
            disabled={busy}
            className="px-2.5 py-1 text-ui-xs text-foreground-subtle border border-border rounded-md hover:bg-surface-hover cursor-pointer disabled:opacity-50"
          >
            Deny
          </button>
          <button
            type="button"
            onClick={() => respond(true, true)}
            disabled={busy}
            title="Approve this and all future tool calls"
            className="px-2.5 py-1 text-ui-xs text-foreground-subtle border border-border rounded-md hover:bg-surface-hover cursor-pointer disabled:opacity-50"
          >
            Always allow
          </button>
          <button
            type="button"
            onClick={() => respond(true, false)}
            disabled={busy}
            className="px-3 py-1 text-ui-xs font-semibold bg-warning text-black rounded-md cursor-pointer disabled:opacity-50"
          >
            Approve
          </button>
        </div>
      </div>
    </div>
  );
}

function PendingQuestionsCard({
  goSessionId,
  acpSessionId,
  questions
}: {
  goSessionId?: string;
  acpSessionId?: string;
  questions: any[];
}) {
  const [answers, setAnswers] = useState<Record<string, any>>({});
  const [submitted, setSubmitted] = useState(false);
  const [showModal, setShowModal] = useState(false);

  const submit = async () => {
    setSubmitted(true);
    try {
      if (goSessionId) {
        await RespondAgentAsk(goSessionId, answers);
      } else if (acpSessionId) {
        const firstQ = questions[0];
        const reqId = firstQ?.acpRequestId || firstQ?.id || 'req-1';
        const val = answers[firstQ.id] || Object.values(answers)[0] || 'allow';
        await ACPRespondPermissionOption(acpSessionId, reqId, true, String(val));
      }
    } catch (err) {
      console.error('failed to submit answers:', err);
      setSubmitted(false);
    }
  };

  const qCount = questions.length;

  return (
    <>
      <div className="w-full max-h-[min(65vh,520px)] flex flex-col rounded-xl border border-info/40 bg-info/5 p-3.5 shadow-md overflow-hidden">
        <div className="shrink-0 flex items-center justify-between pb-2 border-b border-info/20">
          <div className="flex items-center gap-2 text-ui-sm font-semibold text-info">
            <Brain className="size-4 shrink-0" />
            <span>The agent needs your input</span>
          </div>
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={() => setShowModal(true)}
              className="text-ui-xs font-medium text-info hover:underline flex items-center gap-1 cursor-pointer"
            >
              Open dialog
            </button>
            <span className="text-ui-xs font-medium text-info/80">
              {qCount} {qCount === 1 ? 'question' : 'questions'}
            </span>
          </div>
        </div>
        <div className="flex-1 min-h-0 overflow-y-auto space-y-3.5 py-2.5 pr-1.5 scrollbar-thin">
          {questions.map((q: any, qi: number) => {
            const hasOptions = Array.isArray(q.options) && q.options.length > 0;
            return (
              <div key={q.id || qi} className="space-y-1.5 bg-surface/40 rounded-lg p-2.5 border border-border/50">
                <div className="text-ui-sm font-medium text-foreground">
                  <span className="text-info/70 font-mono text-ui-xs mr-1.5">{qi + 1}.</span>
                  {q.header && <span className="text-info font-semibold mr-1.5">[{q.header}]</span>}
                  {q.question}
                </div>
                {hasOptions ? (
                  <div className="space-y-2 pt-0.5">
                    <div className="flex flex-wrap gap-1.5">
                      {(q.options || []).map((optItem: any, oi: number) => {
                        const opt = typeof optItem === 'string' ? optItem : (optItem.label || optItem.name || optItem.id || String(oi + 1));
                        const selected = Array.isArray(answers[q.id])
                          ? answers[q.id].includes(opt)
                          : answers[q.id] === opt;
                        const toggle = () => {
                          if (q.multi) {
                            const cur: string[] = Array.isArray(answers[q.id]) ? answers[q.id] : [];
                            setAnswers(p => ({
                              ...p,
                              [q.id]: cur.includes(opt) ? cur.filter((c: string) => c !== opt) : [...cur, opt]
                            }));
                          } else {
                            setAnswers(p => ({ ...p, [q.id]: opt }));
                          }
                        };
                        const isRec = typeof optItem === 'object' && (optItem.isRecommended || optItem.recommended);
                        return (
                          <button
                            key={oi}
                            type="button"
                            onClick={toggle}
                            className={`px-2.5 py-1 text-ui-xs rounded-md border cursor-pointer transition-colors ${
                              selected
                                ? 'bg-info/20 border-info text-info font-medium shadow-2xs'
                                : 'bg-surface border-border text-foreground-subtle hover:bg-surface-hover hover:text-foreground'
                            }`}
                          >
                            {opt}
                            {(!q.multi && oi === (q.recommended ?? -1)) || isRec ? (
                              <span className="ml-1 opacity-70 font-normal">(Recommended)</span>
                            ) : null}
                          </button>
                        );
                      })}
                    </div>
                    <input
                      type="text"
                      placeholder="Or type custom answer..."
                      value={
                        Array.isArray(answers[q.id])
                          ? ''
                          : (q.options || []).some((o: any) => (typeof o === 'string' ? o : o.label) === answers[q.id])
                            ? ''
                            : answers[q.id] || ''
                      }
                      onChange={e => {
                        const val = e.target.value;
                        setAnswers(p => ({ ...p, [q.id]: val }));
                      }}
                      className="w-full px-2.5 py-1 text-ui-xs rounded-md border border-border bg-surface text-foreground placeholder:text-foreground-subtle/50 focus:outline-none focus:border-info"
                    />
                  </div>
                ) : (
                  <div className="pt-0.5">
                    <textarea
                      rows={2}
                      placeholder="Type your answer here..."
                      value={answers[q.id] || ''}
                      onChange={e => setAnswers(p => ({ ...p, [q.id]: e.target.value }))}
                      className="w-full px-2.5 py-1.5 text-ui-xs rounded-md border border-border bg-surface text-foreground placeholder:text-foreground-subtle/50 focus:outline-none focus:border-info resize-none"
                    />
                  </div>
                )}
              </div>
            );
          })}
        </div>
        <div className="shrink-0 flex items-center justify-between pt-2 border-t border-info/20">
          <span className="text-ui-xs text-foreground-subtle hidden sm:inline">Answer to resume agent turn</span>
          <button
            type="button"
            onClick={submit}
            disabled={submitted}
            className="px-3.5 py-1 text-ui-xs font-semibold bg-info text-black rounded-md cursor-pointer disabled:opacity-50 ml-auto transition-opacity"
          >
            {submitted ? 'Submitting…' : 'Submit answers'}
          </button>
        </div>
      </div>

      {showModal && (
        <AgentQuestionModal
          isOpen={true}
          onClose={() => setShowModal(false)}
          goSessionId={goSessionId}
          acpSessionId={acpSessionId}
          questions={questions}
        />
      )}
    </>
  );
}
import { AgentTaskInputBar } from './AgentTaskInputBar';
import type { SidePaneTab } from './AgentRightSidebar';
import { BottomTerminal } from './BottomTerminal';
import { DiffViewer } from '../diff/DiffViewer';
import { MarkdownRenderer } from './MarkdownRenderer';
import { ThreadFindBar } from './ThreadFindBar';
import { playNotificationSound } from '../../lib/notificationSounds';
import { cleanPiBanner } from '../../lib/utils';
import { ToolExecutionCard, ToolSkillBlock, parseSkillToolData, parseToolArgs, cleanArgPath } from './ToolBlockViews';
import { FloatingTaskTracker } from './FloatingTaskTracker';

const UserAttachmentThumbnail: React.FC<{
  att: { type: 'image' | 'file'; path: string; name: string };
  onOpen?: (path: string) => void;
}> = ({ att, onOpen }) => {
  const [imgSrc, setImgSrc] = useState<string | null>(null);
  const [loading, setLoading] = useState(att.type === 'image');
  const [error, setError] = useState(false);

  useEffect(() => {
    if (att.type !== 'image') return;
    let active = true;
    ApiBridge.readFileBase64(att.path)
      .then(b64 => {
        if (!active) return;
        if (b64) {
          const ext = att.path.split('.').pop()?.toLowerCase() || 'png';
          const mime = ext === 'png' ? 'image/png' : ext === 'jpg' || ext === 'jpeg' ? 'image/jpeg' : ext === 'webp' ? 'image/webp' : ext === 'gif' ? 'image/gif' : ext === 'svg' ? 'image/svg+xml' : 'image/png';
          const src = b64.startsWith('data:') ? b64 : `data:${mime};base64,${b64}`;
          setImgSrc(src);
        } else {
          setError(true);
        }
        setLoading(false);
      })
      .catch(() => {
        if (active) {
          setError(true);
          setLoading(false);
        }
      });
    return () => { active = false; };
  }, [att.type, att.path]);

  if (att.type === 'image' && !error) {
    return (
      <div
        onClick={() => onOpen?.(att.path)}
        className="group relative inline-flex flex-col rounded-lg overflow-hidden border border-border/60 bg-[var(--color-background-button-secondary)] hover:border-info/80 transition-all cursor-pointer max-w-[260px] shadow-sm select-none"
        title={`Click to view ${att.name} in right sidebar`}
      >
        {loading ? (
          <div className="w-44 h-28 flex flex-col items-center justify-center gap-1.5 text-muted-foreground bg-surface/50">
            <ImageIcon className="size-5 animate-pulse text-info" />
            <span className="text-ui-xs font-mono">Loading preview…</span>
          </div>
        ) : (
          <div className="relative overflow-hidden bg-black/10 dark:bg-black/40 flex items-center justify-center max-h-36">
            <img
              src={imgSrc || ''}
              alt={att.name}
              className="max-h-36 max-w-full object-contain rounded-t-lg transition-transform duration-200 group-hover:scale-[1.02]"
            />
          </div>
        )}
        <div className="flex items-center justify-between gap-1.5 px-2.5 py-1.5 bg-surface/90 text-foreground border-t border-border/40">
          <div className="flex items-center gap-1.5 min-w-0">
            <ImageIcon className="size-3.5 text-info shrink-0" />
            <span className="font-mono text-[11px] font-medium truncate max-w-[170px]">{att.name}</span>
          </div>
          <span className="text-[10px] text-muted-foreground group-hover:text-info font-mono shrink-0">View ↗</span>
        </div>
      </div>
    );
  }

  return (
    <button
      type="button"
      onClick={() => onOpen?.(att.path)}
      className="flex items-center gap-1.5 px-2.5 py-1 rounded-full bg-[var(--color-background-button-secondary)] hover:bg-[var(--color-background-button-secondary-hover)] text-ui-xs font-mono text-foreground max-w-full min-w-0 border border-border/40 transition-colors cursor-pointer"
      title={`Click to view ${att.name} in right sidebar`}
    >
      {att.type === 'image' ? (
        <ImageIcon className="size-3.5 text-info shrink-0" />
      ) : (
        <FileCode className="size-3.5 text-success shrink-0" />
      )}
      <span className="font-medium truncate max-w-[180px]">{att.name}</span>
    </button>
  );
};

export const AgentActiveSessionView: React.FC = () => {
  const { 
    agents,
    activeSession, 
    activeSessionId, 
    activeWorkspacePath, 
    diffs, 
    openDiffInEditor, 
    currentModel,
    deleteSessionPermanently,
    isRightActionDrawerOpen,
    setIsRightActionDrawerOpen,
    isBottomTerminalOpen,
    setIsBottomTerminalOpen,
    openSideFile,
    subagents,
    toggleTask,
    clearTasks,
    openSubagentInSidebar,
    openFileInEditor,
    skills,
    mode,
    isLeftSidebarOpen,
    setAgentViewMode
  } = useWorkspace();

  const [expandedThoughts, setExpandedThoughts] = useState<Record<string, boolean>>({});
  const [expandedTools, setExpandedTools] = useState<Record<string, boolean>>({});
  const [expandedWork, setExpandedWork] = useState<Record<string, boolean>>({});
  const [expandedExplore, setExpandedExplore] = useState<Record<string, boolean>>({});
  const [activeInlineDiff, setActiveInlineDiff] = useState<FileDiff | null>(null);
  const [isDiffCardExpanded, setIsDiffCardExpanded] = useState<boolean>(false);
  const [isScrolledUp, setIsScrolledUp] = useState<boolean>(false);
  const [restoringCheckpointId, setRestoringCheckpointId] = useState<string | null>(null);

  const sessionDiffs = activeSession?.diffs && activeSession.diffs.length > 0 
    ? activeSession.diffs 
    : diffs;

  const isSessionWorking = !!(
    activeSession?.status === 'running' ||
    activeSession?.goState === 'thinking' ||
    activeSession?.goState === 'executing'
  );

  const lastAgentMsg = activeSession?.messages?.slice().reverse().find(m => m.role === 'agent');
  const lastAgentMsgId = lastAgentMsg?.id;

  const { liveElapsed } = useAgentExecution(isSessionWorking, lastAgentMsgId);
  const { findOpen, setFindOpen } = useAgentThreadSearch();
  // Track chat column width dynamically to guarantee responsive clearance
  // for the timeline rail and hide it on very narrow views (<520px).
  const [chatColumnWidth, setChatColumnWidth] = useState(1200);
  const chatColumnRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const el = chatColumnRef.current;
    if (!el) return;
    const ro = new ResizeObserver((entries) => {
      for (const entry of entries) {
        setChatColumnWidth(entry.contentRect.width);
      }
    });
    ro.observe(el);
    return () => ro.disconnect();
  }, []);
  // Plain toggle opens the pane EMPTY; a tab argument requests that tab.
  const openSidePaneTab = useCallback((tab?: SidePaneTab) => {
    if (!tab) {
      setIsRightActionDrawerOpen(prev => !prev);
      return;
    }
    setIsRightActionDrawerOpen(true);
    window.dispatchEvent(new CustomEvent('forge:open-sidepane-tab', { detail: { tab } }));
  }, [setIsRightActionDrawerOpen]);
  // Auto-open the Browser viewer when the agent drives the in-app browser
  // (first navigation per session view — if the user closes the tab, later
  // navigations won't yank it back open).
  const autoBrowserOpenRef = useRef(false);
  useEffect(() => {
    const off = EventsOn('browser:status', (s: any) => {
      if (autoBrowserOpenRef.current) return;
      if (s && (s.state === 'running' || s.state === 'navigated') && s.url) {
        autoBrowserOpenRef.current = true;
        openSidePaneTab('browser');
      }
    });
    return off;
  }, [openSidePaneTab]);

  useEffect(() => {
    const onOpen = () => {
      openSidePaneTab('browser');
    };
    window.addEventListener('forge:open-sidepane-browser', onOpen);
    return () => window.removeEventListener('forge:open-sidepane-browser', onOpen);
  }, [openSidePaneTab]);

  useEffect(() => {
    const onOpenSubagent = () => {
      openSidePaneTab('subagent');
    };
    window.addEventListener('forge:open-subagent-tab', onOpenSubagent);
    return () => window.removeEventListener('forge:open-subagent-tab', onOpenSubagent);
  }, [openSidePaneTab]);

  const [backgroundTasks, setBackgroundTasks] = useState<BackgroundTaskItem[]>([]);
  const [schedules, setSchedules] = useState<AgentScheduleItem[]>([]);

  // Fetch background tasks and schedules on active session change
  const targetSessionId = activeSession?.goSessionId || activeSessionId;
  useEffect(() => {
    if (!targetSessionId) {
      setBackgroundTasks([]);
      setSchedules([]);
      return;
    }
    ListBackgroundTasks(targetSessionId)
      .then(tasks => {
        const mapped: BackgroundTaskItem[] = (tasks || []).map((t: any) => ({
          ...t,
          id: t.id || t.task_id || '',
          task_id: t.task_id || t.id || '',
        }));
        setBackgroundTasks(mapped);
      })
      .catch(err => console.error('Failed to list background tasks:', err));

    ListAgentSchedules(targetSessionId)
      .then(scheds => setSchedules(scheds || []))
      .catch(err => console.error('Failed to list agent schedules:', err));
  }, [targetSessionId]);

  // Listen to background task and schedule events from eventbus
  useEffect(() => {
    const offTask = EventsOn('agent:task_update', (data: any) => {
      if (!data) return;
      setBackgroundTasks(prev => {
        const id = data.task_id || data.id;
        const idx = prev.findIndex(t => (t.id || t.task_id) === id);
        const item: BackgroundTaskItem = {
          id,
          task_id: id,
          command: data.command || (idx >= 0 ? prev[idx].command : ''),
          cwd: data.cwd || (idx >= 0 ? prev[idx].cwd : ''),
          status: data.status || (idx >= 0 ? prev[idx].status : 'running'),
          exit_code: data.exit_code,
          started_at: data.started_at ? String(data.started_at) : (idx >= 0 ? prev[idx].started_at : new Date().toISOString()),
          finished_at: data.finished_at ? String(data.finished_at) : undefined,
          output_tail: data.output_tail || (idx >= 0 ? prev[idx].output_tail : ''),
          output_file: data.output_file || (idx >= 0 ? prev[idx].output_file : undefined),
          session_id: data.session_id,
        };
        if (idx >= 0) {
          const next = [...prev];
          next[idx] = { ...next[idx], ...item };
          return next;
        }
        return [...prev, item];
      });
    });

    const offSchedule = EventsOn('agent:schedule_update', (data: any) => {
      if (!data) return;
      setSchedules(prev => {
        const id = data.id || data.schedule_id;
        const idx = prev.findIndex(s => s.id === id);
        const item: AgentScheduleItem = {
          id,
          session_id: data.session_id || (idx >= 0 ? prev[idx].session_id : ''),
          prompt: data.prompt || (idx >= 0 ? prev[idx].prompt : ''),
          cron_expression: data.cron_expression || (idx >= 0 ? prev[idx].cron_expression : undefined),
          duration_seconds: data.duration_seconds || (idx >= 0 ? prev[idx].duration_seconds : undefined),
          task_id: data.task_id || (idx >= 0 ? prev[idx].task_id : undefined),
          status: data.status || (idx >= 0 ? prev[idx].status : undefined),
          active: data.active !== undefined ? Boolean(data.active) : (idx >= 0 ? prev[idx].active : true),
          created_at: data.created_at ? String(data.created_at) : (idx >= 0 ? prev[idx].created_at : new Date().toISOString()),
          next_run: data.next_run ? String(data.next_run) : (idx >= 0 ? prev[idx].next_run : new Date().toISOString()),
        };
        if (idx >= 0) {
          const next = [...prev];
          next[idx] = { ...next[idx], ...item };
          return next;
        }
        return [...prev, item];
      });
    });

    return () => {
      offTask();
      offSchedule();
    };
  }, []);

  const handleOpenTerminalForTask = useCallback((taskId?: string, cwd?: string, command?: string, outputFile?: string) => {
    setIsRightActionDrawerOpen(true);
    window.dispatchEvent(new CustomEvent('forge:open-sidepane-terminal', { detail: { taskId, cwd, command, outputFile } }));
  }, [setIsRightActionDrawerOpen]);

  const handleStopBackgroundTask = useCallback(async (taskId: string) => {
    try {
      await StopBackgroundTask(taskId);
      setBackgroundTasks(prev => prev.map(t => t.id === taskId ? { ...t, status: 'stopped' } : t));
    } catch (err) {
      console.error('Failed to stop background task:', err);
    }
  }, []);

  const handleCancelSchedule = useCallback(async (scheduleId: string) => {
    try {
      await CancelAgentSchedule(scheduleId);
      setSchedules(prev => prev.map(s => s.id === scheduleId ? { ...s, status: 'cancelled', active: false } : s));
    } catch (err) {
      console.error('Failed to cancel schedule:', err);
    }
  }, []);

  // Bottom dock terminal (AnimatedTerminalPanel) — state lives in the
  // workspace store; the full-width header in AgentContainer owns the toggle.
  const [copiedMsgId, setCopiedMsgId] = useState<string | null>(null);
  const [likedMsgs, setLikedMsgs] = useState<Record<string, 'up' | 'down' | null>>({});
  const chatBottomRef = useRef<HTMLDivElement>(null);
  const chatScrollRef = useRef<HTMLDivElement>(null);
  const chatContentRef = useRef<HTMLDivElement>(null);
  const userScrolledUpRef = useRef(false);
  const wasAtBottomRef = useRef(true);
  const isProgrammaticScrollRef = useRef(false);
  const programmaticScrollTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const lastMsgCountRef = useRef(0);
  const [activeTurnIndex, setActiveTurnIndex] = useState<number>(0);
  const [hoveredTurn, setHoveredTurn] = useState<{ index: number; top: number } | null>(null);

  // Group messages into timeline turns (each user prompt + agent response)
  const conversationTurns = useMemo(() => {
    const turns: Array<{
      index: number;
      userMessageId: string;
      userPrompt: string;
      agentPreview?: string;
      timestamp?: string;
    }> = [];
    const msgs = activeSession?.messages || [];
    let currentTurn: {
      index: number;
      userMessageId: string;
      userPrompt: string;
      agentPreview?: string;
      timestamp?: string;
    } | null = null;

    for (let i = 0; i < msgs.length; i++) {
      const msg = msgs[i];
      if (msg.role === 'user') {
        if (currentTurn) {
          turns.push(currentTurn);
        }
        currentTurn = {
          index: turns.length,
          userMessageId: msg.id || `msg-${i}`,
          userPrompt: chatText(msg.content) || 'Task prompt',
          timestamp: msg.timestamp
        };
      } else if (msg.role === 'agent' && currentTurn) {
        if (!currentTurn.agentPreview) {
          const thoughtPreview = msg.thoughts?.[0]?.thoughtText || '';
          currentTurn.agentPreview = chatText(msg.content) || thoughtPreview || 'Running tools...';
        }
      }
    }
    if (currentTurn) {
      turns.push(currentTurn);
    }
    return turns;
  }, [activeSession?.messages]);

  // Only display the left-edge turn scrubber when there are at least 2 turns
  // and the chat column has enough room so ticks never float as a stray dash.
  const showTurnTimeline = conversationTurns.length > 1 && chatColumnWidth >= 560;

  const setProgrammaticScroll = useCallback((durationMs = 550) => {
    isProgrammaticScrollRef.current = true;
    if (programmaticScrollTimerRef.current) {
      clearTimeout(programmaticScrollTimerRef.current);
    }
    programmaticScrollTimerRef.current = setTimeout(() => {
      isProgrammaticScrollRef.current = false;
    }, durationMs);
  }, []);

  const scrollToBottom = useCallback((behavior: ScrollBehavior = 'smooth') => {
    setProgrammaticScroll(behavior === 'smooth' ? 550 : 150);
    userScrolledUpRef.current = false;
    wasAtBottomRef.current = true;
    setIsScrolledUp(false);

    const performScroll = (b: ScrollBehavior) => {
      const el = chatScrollRef.current;
      if (!el) return;
      if (b === 'smooth') {
        el.scrollTo({
          top: el.scrollHeight,
          behavior: 'smooth'
        });
      } else {
        el.scrollTop = el.scrollHeight;
      }
    };

    // Staggered multi-stage scroll to ensure DOM layout, textarea resize, and fonts settle
    performScroll(behavior);
    requestAnimationFrame(() => performScroll(behavior));
    setTimeout(() => performScroll(behavior), 50);
    setTimeout(() => performScroll(behavior), 120);
    setTimeout(() => performScroll('auto'), 250);
    setTimeout(() => performScroll('auto'), 450);
  }, [setProgrammaticScroll]);

  // Update active turn based on scroll position and track user scroll intent
  const handleChatScroll = useCallback(() => {
    setHoveredTurn(null);
    if (!chatScrollRef.current) return;
    const { scrollTop, scrollHeight, clientHeight } = chatScrollRef.current;
    if (clientHeight === 0 || scrollHeight === 0) return;

    // User is considered at bottom if within 60px
    const distanceFromBottom = scrollHeight - (scrollTop + clientHeight);
    if (distanceFromBottom <= 60) {
      userScrolledUpRef.current = false;
      wasAtBottomRef.current = true;
      setIsScrolledUp(false);
    } else if (distanceFromBottom > 100 && !isProgrammaticScrollRef.current) {
      // Only trip scrolled up state if not during a programmatic auto-scroll
      userScrolledUpRef.current = true;
      wasAtBottomRef.current = false;
      setIsScrolledUp(true);
    }

    const containerRect = chatScrollRef.current.getBoundingClientRect();
    let currentActive = 0;

    for (let i = 0; i < conversationTurns.length; i++) {
      const turn = conversationTurns[i];
      const el = document.getElementById(`turn-${turn.userMessageId}`);
      if (el) {
        const elRect = el.getBoundingClientRect();
        if (elRect.top <= containerRect.top + 160) {
          currentActive = i;
        }
      }
    }
    setActiveTurnIndex(currentActive);
  }, [conversationTurns]);

  // Direct user wheel gesture detection
  const handleWheel = useCallback((e: React.WheelEvent<HTMLDivElement>) => {
    if (e.deltaY < 0) {
      // User explicitly scrolled wheel upwards: respect their intent immediately
      isProgrammaticScrollRef.current = false;
      if (programmaticScrollTimerRef.current) {
        clearTimeout(programmaticScrollTimerRef.current);
      }
      userScrolledUpRef.current = true;
      wasAtBottomRef.current = false;
      setIsScrolledUp(true);
    } else if (e.deltaY > 0) {
      const el = chatScrollRef.current;
      if (el) {
        const distanceFromBottom = el.scrollHeight - (el.scrollTop + el.clientHeight);
        if (distanceFromBottom <= 60) {
          userScrolledUpRef.current = false;
          wasAtBottomRef.current = true;
          setIsScrolledUp(false);
        }
      }
    }
  }, []);

  // Keep auto-scroll at bottom during streaming unless user manually scrolled up
  useEffect(() => {
    if (!chatScrollRef.current) return;
    if (userScrolledUpRef.current) return;

    // Use direct scroll assignment to eliminate smooth-scroll jank/flicker while streaming chunks
    chatScrollRef.current.scrollTop = chatScrollRef.current.scrollHeight;
  }, [activeSession?.messages, activeSession?.status]);

  // Snap to bottom when a new user message or turn begins
  useEffect(() => {
    const count = activeSession?.messages?.length || 0;
    if (count > lastMsgCountRef.current) {
      const newMessages = (activeSession?.messages || []).slice(lastMsgCountRef.current);
      const hasUserMsg = newMessages.some(m => m.role === 'user');
      if (hasUserMsg) {
        playNotificationSound('input');
        // Smoothly move up into view when a user message is sent
        scrollToBottom('smooth');
      } else if (!userScrolledUpRef.current) {
        scrollToBottom('auto');
      }
    }
    lastMsgCountRef.current = count;
  }, [activeSession?.messages?.length, activeSession?.messages, scrollToBottom]);

  // Immediate smooth scroll on Enter / prompt submission
  useEffect(() => {
    const handleChatSubmitted = () => {
      scrollToBottom('smooth');
    };
    window.addEventListener('forge:chat-submitted', handleChatSubmitted);
    return () => window.removeEventListener('forge:chat-submitted', handleChatSubmitted);
  }, [scrollToBottom]);

  // Maintain scroll-to-bottom on session switch, sidebar toggle, or returning from settings
  useEffect(() => {
    if (mode === 'agent' && chatScrollRef.current) {
      if (wasAtBottomRef.current || !userScrolledUpRef.current) {
        scrollToBottom('auto');
      }
    }
  }, [mode, isLeftSidebarOpen, activeSession?.id, scrollToBottom]);

  // Auto-scroll on container AND inner content resize if user was at bottom
  useEffect(() => {
    const el = chatScrollRef.current;
    const content = chatContentRef.current;
    if (!el) return;
    const ro = new ResizeObserver(() => {
      if (el.clientHeight > 0 && (wasAtBottomRef.current || !userScrolledUpRef.current)) {
        el.scrollTop = el.scrollHeight;
      }
    });
    ro.observe(el);
    if (content) {
      ro.observe(content);
    }
    return () => ro.disconnect();
  }, []);

  // Guard against ancestor container scroll displacement
  useEffect(() => {
    if (chatColumnRef.current && chatColumnRef.current.scrollTop !== 0) {
      chatColumnRef.current.scrollTop = 0;
    }
  });

  if (!activeSession) return null;

  const toggleThought = (id: string, currentlyOpen?: boolean) => {
    setExpandedThoughts(prev => ({
      ...prev,
      [id]: currentlyOpen !== undefined ? !currentlyOpen : !(prev[id] ?? false)
    }));
  };

  const toggleTool = (id: string, currentlyOpen?: boolean) => {
    setExpandedTools(prev => ({
      ...prev,
      [id]: currentlyOpen !== undefined ? !currentlyOpen : !(prev[id] ?? false)
    }));
  };

  const toggleExplore = (id: string) => {
    setExpandedExplore(prev => ({
      ...prev,
      [id]: prev[id] === undefined ? false : !prev[id]
    }));
  };

  const handleCopy = (text: string, msgId: string) => {
    navigator.clipboard.writeText(text);
    setCopiedMsgId(msgId);
    setTimeout(() => setCopiedMsgId(null), 2000);
  };

  const handleThumb = (msgId: string, type: 'up' | 'down') => {
    setLikedMsgs(prev => ({
      ...prev,
      [msgId]: prev[msgId] === type ? null : type
    }));
  };

  const totalAdditions = sessionDiffs.reduce((acc, d) => acc + (d.additions || 0), 0);
  const totalDeletions = sessionDiffs.reduce((acc, d) => acc + (d.deletions || 0), 0);

  const handleRestoreCheckpoint = async (turnId: string) => {
    if (!activeWorkspacePath || !activeSession?.id) return;
    setRestoringCheckpointId(turnId);
    try {
      await RestoreSessionCheckpoint(activeWorkspacePath, activeSession.id, turnId);
      showToast('Restored workspace checkpoint to this turn', 'success');
    } catch (err: any) {
      showToast(err?.message || 'Failed to restore checkpoint', 'error');
    } finally {
      setRestoringCheckpointId(null);
    }
  };

  const handleAcceptAllReview = () => {
    showToast(`Accepted changes across ${sessionDiffs.length} files`, 'success');
  };

  const handleRejectAllReview = async () => {
    if (lastAgentMsgId) {
      await handleRestoreCheckpoint(lastAgentMsgId);
    } else {
      showToast('Diff changes cleared', 'info');
    }
  };

  const extractExploreInfo = (t: ToolExecution, workspacePath?: string) => {
    let target = '';
    let action = 'Read';

    const toolLower = (t.toolName || '').toLowerCase();
    if (toolLower.includes('search') || toolLower.includes('grep') || toolLower.includes('find') || toolLower.includes('rg')) {
      action = 'Search';
    } else if (toolLower.includes('list') || toolLower.includes('dir')) {
      action = 'List';
    } else if (toolLower.includes('write') || toolLower.includes('create') || toolLower.includes('edit')) {
      action = 'Edit';
    }

    const parsedArgs = parseToolArgs(t.command || '', t.toolName || '');
    target = parsedArgs.path || '';

    if (!target && t.command) {
      const trimmed = t.command.trim();
      if (trimmed.startsWith('{') && trimmed.endsWith('}')) {
        try {
          const parsed = JSON.parse(trimmed);
          target = parsed.path || parsed.filePath || parsed.targetPath || parsed.query || parsed.pattern || parsed.command || parsed.file || '';
        } catch {
          const match = trimmed.match(/"(?:path|filePath|targetPath|file|query|pattern)":\s*"([^"]+)"/);
          if (match) target = match[1];
        }
      }
    }

    if (!target) {
      let clean = (t.command || t.toolName || '').trim();
      clean = clean.replace(/^[{"\s]*(?:path|filePath|targetPath|file)":\s*"?/i, '').replace(/["}\s]+$/g, '');
      clean = clean.replace(/^(?:read_file|read|cat|view_file|view|rg|grep|find|list_dir|ls)\s+/i, '');
      target = clean;
    }

    target = cleanArgPath(target).replace(/\\/g, '/');

    let displayPath = target;
    if (workspacePath) {
      const normWs = workspacePath.replace(/\\/g, '/').replace(/\/+$/, '');
      if (displayPath.startsWith(normWs)) {
        displayPath = displayPath.slice(normWs.length).replace(/^\/+/, '');
      }
    }

    const parts = displayPath.split('/').filter(Boolean);
    const filename = parts.pop() || displayPath;
    const dir = parts.join('/');

    return { action, filename, dir, fullPath: target };
  };

  const parseUserMessage = (rawContent: string) => {
    if (!rawContent) return { text: '', attachments: [] };
    rawContent = chatText(rawContent);
    if (!rawContent) return { text: '', attachments: [] };

    const attachments: { type: 'image' | 'file'; path: string; name: string }[] = [];
    const imageExtensions = ['png', 'jpg', 'jpeg', 'webp', 'gif', 'svg'];

    const attachmentRegex = /\[Attached (Image|File):\s*([^\]]+)\]/gi;
    let match: RegExpExecArray | null;
    while ((match = attachmentRegex.exec(rawContent)) !== null) {
      const rawPath = match[2].trim();
      const cleanPath = rawPath.replace(/\\/g, '/');
      const name = cleanPath.split('/').filter(Boolean).pop() || cleanPath;
      const ext = cleanPath.split('.').pop()?.toLowerCase() || '';
      const type = (match[1].toLowerCase() === 'image' || imageExtensions.includes(ext)) ? 'image' : 'file';
      if (!attachments.some(a => a.path === cleanPath)) {
        attachments.push({ type, path: cleanPath, name });
      }
    }

    // Also parse "Attached files:\n@path1\n@path2"
    const attachedFilesSectionRegex = /(?:^|\n+)Attached files:\s*([\s\S]*?)(?=(?:\n\n[^\n@]|\n[A-Z]|$))/i;
    const afMatch = attachedFilesSectionRegex.exec(rawContent);
    if (afMatch) {
      const pathsBlock = afMatch[1];
      const pathLines = pathsBlock.split('\n').map(l => l.trim()).filter(l => l.startsWith('@'));
      for (const line of pathLines) {
        const rawPath = line.replace(/^@/, '').trim();
        if (!rawPath) continue;
        const cleanPath = rawPath.replace(/\\/g, '/');
        const name = cleanPath.split('/').filter(Boolean).pop() || cleanPath;
        const ext = cleanPath.split('.').pop()?.toLowerCase() || '';
        const type = imageExtensions.includes(ext) ? 'image' : 'file';
        if (!attachments.some(a => a.path === cleanPath)) {
          attachments.push({ type, path: cleanPath, name });
        }
      }
    }

    let text = rawContent
      .replace(/\[Attached (?:Image|File):\s*[^\]]+\]/gi, '')
      .replace(/\[Attached File:[^\]]*\][\s\S]*?(?=\[Attached|$)/gi, '');
    if (afMatch) {
      text = text.replace(afMatch[0], '');
    }
    text = text.trim();

    return { text, attachments };
  };

  const isExploreTool = (t: ToolExecution) => {
    const name = (t.toolName || '').toLowerCase();
    // Mutating / command tools are never "explore" even when their args JSON
    // contains a "path" key (edit/write always do).
    if (name.includes('edit') || name.includes('write') || name.includes('create') ||
        name.includes('delet') || name.includes('bash') || name.includes('shell') ||
        name.includes('exec') || name.includes('run') || name.includes('todo') || name.includes('ask') ||
        name.includes('python') || name.includes('skill')) {
      return false;
    }
    const cmd = (t.command || '').toLowerCase();
    return name.includes('read') || name.includes('search') || name.includes('grep') ||
           name.includes('find') || name.includes('explore') || name.includes('list') ||
           name.includes('view') || cmd.includes('"path"') || cmd.includes('"query"') ||
           cmd.startsWith('read') || cmd.startsWith('rg') || cmd.startsWith('find') || cmd.startsWith('cat');
  };

  return (
    <div className="flex-1 h-full bg-background text-foreground flex overflow-hidden select-none relative transition-colors">
      
      {/* Middle Chat & Task Stream Column (divider is drawn by the right
          pane's border-l when open — no chat border-r to avoid a doubled,
          gapped line) */}
      <div
        ref={chatColumnRef}
        className="flex-1 flex flex-col h-full overflow-hidden bg-background relative min-w-0"
      >
        {/* Find in thread (⌘F) */}
        <ThreadFindBar containerRef={chatScrollRef} open={findOpen} onClose={() => setFindOpen(false)} />

        {/* Floating Collapsible Task Tracker per-session */}
        <FloatingTaskTracker
          tasks={activeSession.tasks}
          subagents={subagents}
          backgroundTasks={backgroundTasks}
          schedules={schedules}
          onToggleTask={toggleTask}
          onClearTasks={clearTasks}
          onOpenSubagent={openSubagentInSidebar}
          onOpenTerminal={handleOpenTerminalForTask}
          onStopBackgroundTask={handleStopBackgroundTask}
          onCancelSchedule={handleCancelSchedule}
        />

        {/* Turn Scrubber / Timeline Tracker (compact micro-minimap rail on the left edge) */}
        {showTurnTimeline && (
          <nav
            aria-label="Conversation turn timeline tracker"
            className="absolute left-1.5 sm:left-2 top-1/2 -translate-y-1/2 z-30 flex flex-col items-start gap-1 py-2 px-0.5 max-h-[min(70vh,480px)] overflow-y-auto no-scrollbar pointer-events-auto"
          >
            {conversationTurns.map((turn, idx) => {
              const isActive = activeTurnIndex === idx;

              return (
                <button
                  key={turn.userMessageId}
                  type="button"
                  onClick={() => {
                    const el = document.getElementById(`turn-${turn.userMessageId}`);
                    if (el && chatScrollRef.current) {
                      const containerRect = chatScrollRef.current.getBoundingClientRect();
                      const elRect = el.getBoundingClientRect();
                      const targetTop = chatScrollRef.current.scrollTop + (elRect.top - containerRect.top) - 24;
                      chatScrollRef.current.scrollTo({ top: Math.max(0, targetTop), behavior: 'smooth' });
                    }
                  }}
                  onMouseEnter={(e) => {
                    const rect = e.currentTarget.getBoundingClientRect();
                    const parentRect = chatColumnRef.current?.getBoundingClientRect();
                    const topOffset = parentRect ? rect.top - parentRect.top + rect.height / 2 : rect.top;
                    setHoveredTurn({ index: idx, top: topOffset });
                  }}
                  onMouseLeave={() => setHoveredTurn(null)}
                  className="group/turn-tick flex items-center py-1 px-0.5 cursor-pointer select-none"
                  aria-label={`Jump to turn ${idx + 1}`}
                >
                  <span
                    className={cn(
                      'h-[2px] rounded-full transition-all duration-150 block',
                      isActive
                        ? 'w-3 bg-foreground shadow-xs'
                        : 'w-1.5 bg-foreground-subtlest/60 group-hover/turn-tick:w-3 group-hover/turn-tick:bg-foreground-subtle'
                    )}
                  />
                </button>
              );
            })}
          </nav>
        )}

        {/* Turn Preview Pop-up Modal (rendered OUTSIDE <nav>, never clipped, single source of preview without native title tooltip) */}
        {hoveredTurn !== null && conversationTurns[hoveredTurn.index] && (
          <div
            style={{
              top: `${Math.max(24, Math.min(hoveredTurn.top, (chatColumnRef.current?.clientHeight ?? 600) - 130))}px`,
            }}
            className="absolute left-6 sm:left-7 z-50 w-72 rounded-[10px] bg-card/95 backdrop-blur-md border border-border/80 p-3 shadow-2xl text-left pointer-events-none -translate-y-1/2 animate-in fade-in zoom-in-95 duration-100"
          >
            <div className="flex items-center justify-between text-2xs font-mono uppercase tracking-wider text-foreground-subtle mb-1.5">
              <span className="font-semibold text-primary">Turn {hoveredTurn.index + 1}</span>
              {conversationTurns[hoveredTurn.index].timestamp && (
                <span>{formatRelativeTime(conversationTurns[hoveredTurn.index].timestamp)}</span>
              )}
            </div>
            <div className="text-ui-base font-semibold text-foreground line-clamp-2 leading-snug">
              {conversationTurns[hoveredTurn.index].userPrompt}
            </div>
            {conversationTurns[hoveredTurn.index].agentPreview && (
              <div className="text-ui-sm text-foreground-subtle line-clamp-3 leading-relaxed mt-1.5 font-normal">
                {conversationTurns[hoveredTurn.index].agentPreview}
              </div>
            )}
          </div>
        )}

        {/* Chat Transcript Area */}
        <div
          ref={chatScrollRef}
          onScroll={handleChatScroll}
          onWheel={handleWheel}
          data-agent-chat="true"
          className={cn(
            "flex-1 overflow-y-auto overflow-x-hidden pt-6 pb-6 space-y-6 select-text cursor-default relative w-full min-w-0 overscroll-y-contain",
            isRightActionDrawerOpen ? "px-3 sm:px-4" : "px-6 sm:px-8"
          )}
        >
          <div ref={chatContentRef} className="chat-content-lane space-y-6">
            {activeSession.messages.length === 0 ? (
              <div className="py-24 text-center text-foreground-subtlest space-y-3">
                <Sparkles className="w-9 h-9 text-foreground-subtlest mx-auto opacity-70" />
                <p className="font-medium text-sm text-foreground-subtle">Ready for instructions</p>
                <p className="text-xs text-foreground-subtlest max-w-sm mx-auto">
                  Type a task prompt below. The agent will explore files, execute tools, and propose changes.
                </p>
              </div>
            ) : (
              activeSession.messages.map((msg, index) => {
              const isUser = msg.role === 'user';
              const exploreTools = (msg.toolExecutions || []).filter(isExploreTool);
              const commandTools = (msg.toolExecutions || []).filter(t => !isExploreTool(t));
              const isMsgLiked = likedMsgs[msg.id];
              const isCopied = copiedMsgId === msg.id;

              return (
                <div 
                  key={msg.id || index} 
                  id={isUser ? `turn-${msg.id || index}` : undefined} 
                  className="group/msg w-full max-w-full min-w-0 space-y-4 scroll-mt-6"
                >
                  {/* PANDACODE PROVIDER HANDOFF CARD */}
                  {msg.role === 'handoff' && (() => {
                    let handoff = msg.handoff;
                    if (!handoff) {
                      try {
                        const parsed = JSON.parse(msg.content);
                        if (parsed && (parsed.fromProvider || parsed.toProvider)) handoff = parsed;
                      } catch {}
                    }
                    const fromProv = handoff?.fromProvider || activeSession.provider || 'internal';
                    const toProv = handoff?.toProvider || activeSession.provider || 'internal';
                    const summary = handoff?.summary || msg.content || 'Conversation carried forward into new agent engine.';

                    return (
                      <div className="flex justify-center my-4 select-none">
                        <details className="group/handoff w-full max-w-lg rounded-xl border border-border/70 bg-surface/50 hover:bg-surface/80 transition-colors p-2.5 shadow-xs">
                          <summary className="flex items-center justify-center gap-2 text-ui-xs text-foreground-subtle hover:text-foreground cursor-pointer select-none list-none [&::-webkit-details-marker]:hidden">
                            <ProviderIcon provider={fromProv} size={15} />
                            <ArrowRight className="size-3 text-foreground-subtlest" />
                            <ProviderIcon provider={toProv} size={15} />
                            <span className="font-semibold text-foreground mx-1">Conversation carried forward</span>
                            <ChevronDown className="size-3 text-foreground-subtlest transition-transform group-open/handoff:rotate-180" />
                          </summary>
                          <p className="mt-2.5 px-3 text-ui-xs text-foreground-subtle text-center leading-relaxed font-sans">
                            {summary}
                          </p>
                        </details>
                      </div>
                    );
                  })()}
                  
                  {/* USER MESSAGE BUBBLE (Kybern style: 80% width max, rounded squircle, subtle translucent background) */}
                  {isUser && (() => {
                    const { text, attachments } = parseUserMessage(msg.content);
                    const skillMatch = text.match(/(?:^|\s)\/skill:([^\s/]+)/i);
                    const skillName = skillMatch ? skillMatch[1] : null;
                    const matchedSkill = skillName
                      ? (skills || []).find(s => (s?.name || '').toLowerCase() === skillName.toLowerCase())
                      : undefined;
                    return (
                      <div className="group/user-row w-full max-w-full min-w-0 flex flex-col items-end gap-1">
                        <div className="relative max-w-[82%] rounded-2xl border border-border/50 bg-message px-3.5 py-2.5 text-message-foreground shadow-2xs select-text">
                        {matchedSkill && (
                          <div className="mb-1.5 inline-flex items-center gap-1 rounded-md border border-primary/25 bg-primary/10 px-2 py-0.5 text-2xs font-mono font-medium text-primary">
                            <Sparkles className="size-3" />
                            <span>/{matchedSkill.name}</span>
                          </div>
                        )}
                        {text && (
                          <p className="text-ui-base leading-relaxed text-message-foreground font-normal whitespace-pre-wrap break-words [overflow-wrap:anywhere] select-text min-w-0">
                            {text}
                          </p>
                        )}
                        {attachments.length > 0 && (
                          <div className="flex flex-wrap justify-end gap-2 max-w-full min-w-0 pt-1.5">
                            {attachments.map((att, i) => (
                              <UserAttachmentThumbnail
                                key={i}
                                att={att}
                                onOpen={(filePath) => void openSideFile(filePath)}
                              />
                            ))}
                          </div>
                        )}
                        </div>

                        <div className="flex w-full max-w-[82%] items-center justify-end gap-2 pe-1 text-2xs tabular-nums opacity-0 transition-opacity duration-200 group-hover/user-row:opacity-100">
                          <p className="text-fg-tertiary">{formatRelativeTime(msg.timestamp)}</p>
                          <button
                            type="button"
                            onClick={() => handleCopy(text, msg.id || `u-${index}`)}
                            className="rounded p-1 text-muted-foreground hover:bg-accent hover:text-foreground transition-colors cursor-pointer"
                            title="Copy message"
                          >
                            {copiedMsgId === (msg.id || `u-${index}`) ? <Check className="size-3 text-success" /> : <Copy className="size-3" />}
                          </button>
                        </div>
                      </div>
                    );
                  })()}

                  {/* AGENT RESPONSE & STREAMING STEPS (ForgeADE progress:
                      one "Working for Ns"/"Worked for Ns" row — only when
                      there is thinking or tool work to collapse) */}
                  {!isUser && msg.role !== 'handoff' && (() => {
                    const allTools = msg.toolExecutions || [];
                    // Is this a skill tool?
                    const isSkillTool = (t: ToolExecution) => {
                      const name = (t.toolName || '').toLowerCase();
                      return name.includes('skill');
                    };

                    // Extract and deduplicate skill tools for this message
                    const skillTools: ToolExecution[] = [];
                    const seenSkills = new Set<string>();
                    for (const t of allTools) {
                      if (isSkillTool(t)) {
                        const parsed = parseSkillToolData(t);
                        const key = (parsed.name || t.toolName || '').toLowerCase();
                        if (!seenSkills.has(key)) {
                          seenSkills.add(key);
                          skillTools.push(t);
                        }
                      }
                    }

                    // Sub-agent spawns (invoke_subagent / task / spawn_subagent) render as
                    // rich collapsible "SubAgent" cards instead of generic tools.
                    const isSubagentTool = (t: ToolExecution) => {
                      const name = (t.toolName || '').toLowerCase();
                      return name === 'invoke_subagent' || name === 'task' || name === 'spawn_subagent' || name === 'subagent';
                    };
                    const parseSubagentTool = (t: ToolExecution) => {
                      let args: any = {};
                      try { args = JSON.parse(t.command || '{}'); } catch { /* args not JSON */ }
                      let result: any = {};
                      try { result = typeof t.output === 'string' ? JSON.parse(t.output) : (t.output ?? {}); } catch { /* output not JSON */ }
                      const task = String(args.Prompt || args.prompt || args.task || args.instruction || '');
                      const role = String(
                        result.selected_subagent ||
                        result.subagent ||
                        args.Role ||
                        args.role ||
                        args.TypeName ||
                        args.typeName ||
                        args.subagent ||
                        'Subagent'
                      );
                      const rawOutput = typeof t.output === 'string' ? t.output : (t.output !== undefined && t.output !== null ? JSON.stringify(t.output, null, 2) : '');
                      return {
                        task,
                        requested: role,
                        name: role,
                        typeName: String(args.TypeName || args.typeName || ''),
                        model: String(args.Model || args.model || ''),
                        output: rawOutput,
                        failed: t.status === 'failed' || (t.output || '').includes('<tool_use_error>'),
                      };
                    };
                    const subagentTools: ToolExecution[] = [];
                    const seenSubagentTasks = new Set<string>();
                    for (const t of allTools) {
                      if (isSubagentTool(t)) {
                        // Dedupe: the same spawn can appear as both a
                        // streaming tool row and a persisted content block.
                        const key = `${t.id}|${parseSubagentTool(t).task}`;
                        if (!seenSubagentTasks.has(key)) {
                          seenSubagentTasks.add(key);
                          subagentTools.push(t);
                        }
                      }
                    }

                    // Timeline items exclude skill tools and sub-agent spawns
                    // so they aren't rendered twice
                    const timelineItems = getChronologicalTimeline(msg).filter(item => {
                      if (item.kind === 'tool' && item.tool && (isSkillTool(item.tool) || isSubagentTool(item.tool))) {
                        return false;
                      }
                      return true;
                    });

                    const isTurnRunning = !!(isSessionWorking && msg.id === lastAgentMsgId);
                    const isCurrentlyThinking = !!(msg.isThinking && isSessionWorking && msg.id === lastAgentMsgId);
                    const ordinaryTools = allTools.filter(t => !isSkillTool(t));
                    const hasOrdinaryTools = ordinaryTools.length > 0;
                    const hasThoughts = (msg.thoughts || []).length > 0;

                    // Identify the final response text item
                    // If turn is still running, there may not be a final text yet, or the last text is currently streaming.
                    let finalItemIndex = -1;
                    for (let i = timelineItems.length - 1; i >= 0; i--) {
                      if (timelineItems[i].kind === 'text') {
                        finalItemIndex = i;
                        break;
                      }
                    }

                    // Process items: everything in the timeline before the final response (or all thoughts & tools)
                    // If finalItemIndex >= 0 and not running (or completed turn), the final response is rendered outside.
                    // If turn is finished, processItems = all items except finalItem.
                    // If turn is running, during execution prior to final text, processItems are inside the working block.
                    const processItems: typeof timelineItems = [];
                    const finalItems: typeof timelineItems = [];

                    if (finalItemIndex >= 0) {
                      timelineItems.forEach((item, idx) => {
                        if (idx === finalItemIndex) {
                          finalItems.push(item);
                        } else {
                          processItems.push(item);
                        }
                      });
                    } else {
                      processItems.push(...timelineItems);
                    }

                    // Has collapsible work? Yes if there are thoughts, tools, or in-process intermediate texts.
                    const hasWork = processItems.length > 0;
                    const wbId = `wb-${msg.id || index}`;
                    // Worked group: open when in progress, close when done.
                    const workOpen = isTurnRunning
                      ? (expandedWork[wbId] !== false)
                      : (expandedWork[wbId] === true);

                    const totalThoughtSecs = (msg.thoughts || []).reduce((a, th) => a + (th.durationSeconds || 0), 0);
                    let workSecs = totalThoughtSecs;
                    if (workSecs <= 0) {
                      workSecs = Math.max(ordinaryTools.length * 2, 2);
                    }

                    // Detect if this session or message is driven by Google Antigravity
                    const sessionAgentId = activeSession?.agentId || '';
                    const matchedAgent = (agents || []).find((a: any) => a.id === sessionAgentId);
                    const isAntigravityAgent =
                      sessionAgentId === 'agent-agy' ||
                      sessionAgentId === 'antigravity' ||
                      sessionAgentId === 'agy' ||
                      sessionAgentId.includes('antigravity') ||
                      msg.agentId === 'agent-agy' ||
                      msg.authorProvider === 'agy' ||
                      (matchedAgent?.id === 'agent-agy' || (matchedAgent?.name || '').toLowerCase().includes('antigravity'));

                    // Calculate total steps for Antigravity: strictly for THIS task/message, NOT cumulative session steps!
                    let totalStepRuns = msg.totalSteps || msg.stepCount || 0;
                    if (!totalStepRuns) {
                      const toolCount = allTools.length;
                      const thoughtCount = (msg.thoughts || []).length;
                      if (toolCount > 0 || thoughtCount > 0) {
                        totalStepRuns = toolCount + thoughtCount;
                      }
                    }

                    // A task is considered stopped or completed only when it is no longer running or thinking
                    const isTaskStoppedOrDone = !isTurnRunning && !isCurrentlyThinking && !msg.isThinking && (
                      activeSession?.status !== 'running' || msg.id !== lastAgentMsgId
                    );

                    if (!hasWork && !isTurnRunning && finalItems.length === 0) {
                      return null;
                    }

                    const workingLabel = liveElapsed > 0 ? `Working for ${formatElapsedSeconds(liveElapsed)}` : 'Working…';

                    const renderTimelineItem = (item: (typeof timelineItems)[0]) => {
                      if (item.kind === 'thought') {
                        const th = item.thought!;
                        const isLastThought = th.id === msg.thoughts?.[msg.thoughts.length - 1]?.id;
                        const isStreamingThis = !!(isCurrentlyThinking && isLastThought);
                        const isOpen = expandedThoughts[th.id] !== undefined
                          ? expandedThoughts[th.id]
                          : (isStreamingThis || isLastThought);
                        const thoughtTokens = estimateTokenCount(th.thoughtText || '');
                        const durationText = th.durationSeconds && th.durationSeconds > 0
                          ? formatElapsedSeconds(th.durationSeconds)
                          : 'a few seconds';
                        const thoughtLabel = isStreamingThis
                          ? (liveElapsed > 0 ? `Thinking for ${formatElapsedSeconds(liveElapsed)}` : 'Thinking…')
                          : `Thought · ${durationText}${thoughtTokens > 0 ? ` · ${thoughtTokens} tokens` : ''}`;

                        return (
                          <div key={th.id} className="w-full max-w-full min-w-0">
                            <div className="flex items-center gap-1 w-full min-w-0">
                              <button
                                type="button"
                                aria-expanded={isOpen}
                                onClick={() => toggleThought(th.id, isOpen)}
                                className="group/timeline-row flex min-h-6 flex-1 max-w-full min-w-0 cursor-pointer items-center gap-1.5 overflow-hidden rounded-md px-1 py-0.5 text-ui-sm leading-relaxed transition-colors hover:bg-accent/70 select-none"
                              >
                                <span className="relative w-5 shrink-0 leading-0">
                                  <Brain className={cn("inline-flex size-3.5 shrink-0 items-center justify-center text-icon-muted stroke-2", isStreamingThis && "text-primary animate-pulse")} />
                                </span>
                                <span className={cn('block truncate text-secondary-label font-mono text-[11px]', isStreamingThis && 'live-tool-shine text-primary font-medium')}>
                                  {thoughtLabel}
                                </span>
                                <span className="flex size-4 shrink-0 items-center justify-center ml-auto">
                                  {isOpen ? (
                                    <ChevronDown className="size-3 text-icon-muted opacity-70 transition-transform duration-200" />
                                  ) : (
                                    <ChevronRight className="size-3 text-icon-muted opacity-70 transition-transform duration-200" />
                                  )}
                                </span>
                              </button>

                              {th.thoughtText && (
                                <button
                                  type="button"
                                  onClick={(e) => {
                                    e.stopPropagation();
                                    void navigator.clipboard.writeText(th.thoughtText);
                                  }}
                                  className="p-1 rounded text-foreground-subtlest hover:text-foreground hover:bg-surface-hover transition-colors cursor-pointer shrink-0"
                                  title="Copy raw reasoning"
                                >
                                  <Copy className="size-3" />
                                </button>
                              )}
                            </div>

                            {isOpen && (
                              <div className="ms-6 my-1 rounded-md border border-border/50 bg-surface/30 p-2.5 text-ui-sm leading-relaxed select-text scrollbar-thin max-h-[360px] overflow-y-auto overflow-x-hidden w-full max-w-full min-w-0 break-words [overflow-wrap:anywhere]">
                                <MarkdownRenderer content={th.thoughtText} muted />
                                <div className="mt-2 flex items-center justify-between text-[10px] text-foreground-subtlest font-mono border-t border-border/40 pt-1 select-none">
                                  <span>Chain of Thought (CoT)</span>
                                  <span>{thoughtTokens} tokens</span>
                                </div>
                              </div>
                            )}
                          </div>
                        );
                      }

                      if (item.kind === 'text') {
                        return (
                          <div key={item.id} className="text-ui-base/6 text-foreground select-text py-1 w-full max-w-full min-w-0 break-words [overflow-wrap:anywhere] leading-relaxed">
                            <MarkdownRenderer content={item.text!} />
                          </div>
                        );
                      }

                      // Tool execution card
                      const t = item.tool!;
                      const nameLower = (t.toolName || "").toLowerCase();
                      const isBash = nameLower.includes("bash") || nameLower.includes("shell") || nameLower.includes("exec") || nameLower.includes("command");
                      const isEdit = nameLower.includes("edit");
                      const isWrite = nameLower.includes("write") || nameLower.includes("create");
                      const isSkill = nameLower.includes("skill");
                      const isFailed = t.status === "failed" || (t.output || "").includes("<tool_use_error>");
                      const defaultOpen = false;
                      const isToolOpen = expandedTools[t.id] !== undefined ? expandedTools[t.id] : defaultOpen;

                      return (
                        <ToolExecutionCard
                          key={t.id}
                          tool={t}
                          isOpen={isToolOpen}
                          onToggle={() => toggleTool(t.id, isToolOpen)}
                          workspacePath={activeSession.workspacePath}
                        />
                      );
                    };

                    return (
                      <div className="flex flex-col items-start space-y-4 w-full max-w-full min-w-0">
                        {/* 0. SKILL BLOCK PINNED AT THE VERY TOP — stays open even after turn finishes */}
                        {skillTools.length > 0 && (
                          <div className="w-full max-w-full min-w-0 space-y-3">
                            {skillTools.map(st => {
                              const isSkillOpen = expandedTools[st.id] !== undefined ? expandedTools[st.id] : true;
                              return (
                                <ToolSkillBlock
                                  key={st.id}
                                  tool={st}
                                  isOpen={isSkillOpen}
                                  onToggle={() => toggleTool(st.id, isSkillOpen)}
                                  workspacePath={activeSession.workspacePath}
                                />
                              );
                            })}
                          </div>
                        )}

                        {subagentTools.length > 0 && (
                          <div className="w-full max-w-full min-w-0 space-y-2">
                            {subagentTools.map(st => {
                              const info = parseSubagentTool(st);
                              const isRunning = st.status === 'running';
                              const isSubOpen = expandedTools[st.id] !== undefined ? expandedTools[st.id] : true;
                              return (
                                <div
                                  key={st.id}
                                  className="w-full max-w-full min-w-0 rounded-2xl bg-secondary/70 border border-border/40 overflow-hidden text-ui-sm transition-all"
                                  data-testid="subagent-spawn-row"
                                >
                                  <div
                                    onClick={() => toggleTool(st.id, isSubOpen)}
                                    className="flex min-h-9 w-full max-w-full min-w-0 items-center gap-3 py-1.5 ps-4 pe-3 cursor-pointer select-none hover:bg-secondary/90 transition-colors"
                                  >
                                    <Bot className="size-4 shrink-0 text-muted-foreground" />
                                    <span className="shrink-0 font-medium text-foreground">{info.name}</span>
                                    {info.typeName && info.typeName !== info.name && (
                                      <span className="shrink-0 text-3xs font-mono px-1.5 py-0.5 rounded bg-surface border border-border/50 text-foreground-subtle">
                                        {info.typeName}
                                      </span>
                                    )}
                                    {info.task && (
                                      <span className="min-w-0 flex-1 truncate text-muted-foreground">
                                        · {info.task}
                                      </span>
                                    )}
                                    <span className="ml-auto shrink-0 flex items-center gap-2">
                                      {isRunning ? (
                                        <Loader2 className="h-3.5 w-3.5 animate-spin text-muted-foreground" />
                                      ) : info.failed ? (
                                        <X className="h-3.5 w-3.5 text-destructive" />
                                      ) : (
                                        <Check className="h-3.5 w-3.5 text-success" />
                                      )}
                                      <ChevronDown
                                        className={cn(
                                          "size-3.5 text-muted-foreground transition-transform duration-200",
                                          isSubOpen ? "rotate-0" : "-rotate-90"
                                        )}
                                      />
                                    </span>
                                  </div>

                                  {isSubOpen && (
                                    <div className="border-t border-border/40 bg-surface/40 p-3 space-y-2.5 text-xs">
                                      {info.task && (
                                        <div className="space-y-1">
                                          <div className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
                                            Instruction / Prompt
                                          </div>
                                          <div className="rounded-lg bg-surface p-2.5 font-mono text-xs text-foreground/90 whitespace-pre-wrap max-h-48 overflow-y-auto border border-border/40 select-text">
                                            {info.task}
                                          </div>
                                        </div>
                                      )}
                                      {isRunning && (
                                        <div className="flex items-center gap-2 py-1 text-muted-foreground">
                                          <Loader2 className="size-3.5 animate-spin text-primary" />
                                          <span>Sub-agent is executing in the background...</span>
                                        </div>
                                      )}
                                      {!isRunning && info.output && (
                                        <div className="space-y-1">
                                          <div className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
                                            Result / Output
                                          </div>
                                          <div className="rounded-lg bg-surface p-2.5 font-mono text-xs text-foreground/90 whitespace-pre-wrap max-h-64 overflow-y-auto border border-border/40 select-text">
                                            {info.output}
                                          </div>
                                        </div>
                                      )}
                                    </div>
                                  )}
                                </div>
                              );
                            })}
                          </div>
                        )}

                        {/* 1. WORKED-FOR HEADER & PROCESS GROUP — covers thoughts, in-process texts, and tools */}
                        {hasWork ? (
                          <div className="w-full max-w-full min-w-0 flex flex-col items-start space-y-2 pb-1 pt-0.5 select-none">
                            <button
                              type="button"
                              aria-expanded={workOpen}
                              onClick={() => setExpandedWork(prev => ({ ...prev, [wbId]: !workOpen }))}
                              className="group/wb inline-flex max-w-full items-center gap-2 rounded-lg border border-border/60 bg-surface/60 hover:bg-surface-hover px-2.5 py-1 text-ui-xs font-medium tabular-nums cursor-pointer select-none transition-colors shadow-2xs"
                            >
                              {isTurnRunning ? (
                                <Loader2 className="size-3 text-primary animate-spin shrink-0" />
                              ) : (
                                <span className="flex size-3.5 items-center justify-center rounded-full bg-success/15 text-success shrink-0">
                                  <Check className="size-2.5 stroke-[2.75]" />
                                </span>
                              )}
                              <div className="flex items-center gap-1.5 min-w-0">
                                <span
                                  role="status"
                                  aria-live="polite"
                                  className={`block truncate ${isTurnRunning ? 'live-tool-shine font-medium' : 'text-foreground-subtle group-hover/wb:text-foreground'}`}
                                >
                                  {isTurnRunning
                                    ? workingLabel
                                    : isTaskStoppedOrDone && totalStepRuns > 0
                                    ? `Worked for ${formatElapsedSeconds(workSecs)} · Done in ${totalStepRuns} ${totalStepRuns === 1 ? 'step' : 'steps'}`
                                    : `Worked for ${formatElapsedSeconds(workSecs)}`}
                                </span>
                              </div>
                              <span className="flex items-center gap-1 shrink-0 text-foreground-subtlest">
                                {workOpen ? (
                                  <ChevronDown className="w-3.5 h-3.5 text-icon-muted transition-transform duration-200" />
                                ) : (
                                  <ChevronRight className="w-3.5 h-3.5 text-icon-muted transition-transform duration-200" />
                                )}
                              </span>
                            </button>

                            {/* When work block is expanded: show all intermediate process items with grouped WorkLogList */}
                            {workOpen && (
                              <div className="w-full max-w-full min-w-0 space-y-2 my-1 pl-2.5 border-l-2 border-border/60 ml-2">
                                {(() => {
                                  const chunks: { kind: 'tools' | 'item'; items: typeof processItems }[] = [];
                                  let currentTools: typeof processItems = [];

                                  for (const it of processItems) {
                                    if (it.kind === 'tool') {
                                      currentTools.push(it);
                                    } else {
                                      if (currentTools.length > 0) {
                                        chunks.push({ kind: 'tools', items: currentTools });
                                        currentTools = [];
                                      }
                                      chunks.push({ kind: 'item', items: [it] });
                                    }
                                  }
                                  if (currentTools.length > 0) {
                                    chunks.push({ kind: 'tools', items: currentTools });
                                  }

                                  return chunks.map((chunk, cIdx) => {
                                    if (chunk.kind === 'tools') {
                                      const toolsList = chunk.items.map(it => it.tool!).filter(Boolean);
                                      const stableKey = toolsList[0]?.id ? `tools-group-${toolsList[0].id}` : `folded-tools-${cIdx}`;
                                      return (
                                        <FoldedToolTraces key={stableKey} tools={toolsList} defaultExpanded={true} className="w-full">
                                          <WorkLogList className="w-full">
                                            {chunk.items.map(it => renderTimelineItem(it))}
                                          </WorkLogList>
                                        </FoldedToolTraces>
                                      );
                                    }
                                    return chunk.items.map(it => renderTimelineItem(it));
                                  });
                                })()}
                              </div>
                            )}
                          </div>
                        ) : isTurnRunning ? (
                          <div className="flex flex-col items-start gap-1 w-fit select-none">
                            <div className="flex items-center gap-2">
                              <span role="status" aria-live="polite" className="font-medium animated-gradient-text py-1 text-ui-lg">
                                {workingLabel}
                              </span>
                              <Loader2 className="size-4 animate-spin text-foreground-subtle" />
                            </div>
                            {isCurrentlyThinking && !hasThoughts && (
                              <div className="flex items-center gap-2 text-ui-base text-trajectory-reasoning py-0.5 animate-pulse">
                                <Brain className="w-4 h-4 text-trajectory-reasoning shrink-0" />
                                <span className="font-medium">Thinking…</span>
                              </div>
                            )}
                          </div>
                        ) : null}

                        {/* 2. FINAL AGENT RESPONSE — always visible outside the collapsed work group */}
                        {finalItems.length > 0 && (
                          <div className="w-full max-w-full min-w-0 space-y-3">
                            {finalItems.map(item => renderTimelineItem(item))}
                          </div>
                        )}

                        {/* 2b. TURN FILE CHANGES ATTRIBUTION (PandaCode style) */}
                        {allTools.length > 0 && (
                          <TurnChangesCard
                            tools={allTools}
                            workspacePath={activeSession?.workspacePath}
                            onOpenDiff={(diff) => openDiffInEditor(diff)}
                            onOpenFile={(path) => void openSideFile(path)}
                          />
                        )}

                      {/* 6. ACTION FOOTER (assistant meta: hover-reveal, quiet) */}
                      <div className="flex w-full max-w-full min-w-0 items-center gap-0.5 px-1 py-0.5 opacity-0 transition-opacity duration-200 focus-within:opacity-100 group-hover/msg:opacity-100">
                        <button
                          type="button"
                          onClick={() => handleCopy(chatText(msg.content), msg.id)}
                          className="rounded p-1.5 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground cursor-pointer"
                          title="Copy response"
                        >
                          {isCopied ? (
                            <Check className="size-3.5 text-success" />
                          ) : (
                            <Copy className="size-3.5" />
                          )}
                        </button>

                        <button
                          type="button"
                          onClick={() => handleThumb(msg.id, 'up')}
                          className={`rounded p-1.5 transition-colors hover:bg-accent cursor-pointer ${
                            isMsgLiked === 'up' ? 'text-success' : 'text-muted-foreground hover:text-foreground'
                          }`}
                          title="Good response"
                        >
                          <ThumbsUp className="size-3.5" />
                        </button>

                        <button
                          type="button"
                          onClick={() => handleThumb(msg.id, 'down')}
                          className={`rounded p-1.5 transition-colors hover:bg-accent cursor-pointer ${
                            isMsgLiked === 'down' ? 'text-destructive' : 'text-muted-foreground hover:text-foreground'
                          }`}
                          title="Bad response"
                        >
                          <ThumbsDown className="size-3.5" />
                        </button>

                        <button
                          type="button"
                          onClick={() => void handleRestoreCheckpoint(msg.id)}
                          disabled={restoringCheckpointId === msg.id}
                          className="flex items-center gap-1 rounded px-1.5 py-1 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground cursor-pointer disabled:opacity-50"
                          title="Restore workspace checkpoint before this turn"
                        >
                          <RotateCcw className={cn("size-3.5", restoringCheckpointId === msg.id && "animate-spin text-primary")} />
                          <span className="text-[10px] hidden sm:inline">Checkpoint</span>
                        </button>

                        <div className="ml-1 text-2xs tabular-nums text-muted-foreground flex items-center gap-2">
                          <span>{formatRelativeTime(msg.timestamp)}</span>
                          {(activeSession?.model || matchedAgent?.name) && (
                            <span className="hidden sm:inline-flex items-center gap-1 px-1.5 py-0.5 rounded-md bg-surface/70 text-foreground-subtle border border-border/50 font-mono text-3xs select-none">
                              <ProviderIcon provider={matchedAgent?.icon || msg.authorProvider || activeSession?.provider || 'agent-internal'} size={10} />
                              <span className="truncate max-w-[130px]">{activeSession?.model || matchedAgent?.name}</span>
                            </span>
                          )}
                          {isTaskStoppedOrDone && totalStepRuns > 0 && (
                            <span
                              className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded-md bg-surface text-foreground-subtle border border-border/50 text-3xs font-mono font-medium select-none"
                              title={`${matchedAgent?.name || 'Agent'} completed in ${totalStepRuns} step runs`}
                            >
                              <Check className="size-2.5 text-success stroke-[2.5]" />
                              <span>Done in {totalStepRuns} {totalStepRuns === 1 ? 'step' : 'steps'}</span>
                            </span>
                          )}
                        </div>
                      </div>

                      {/* Turn end marker */}
                      {index === activeSession.messages.length - 1 && activeSession.status !== 'running' && (chatText(msg.content).includes('stopped by user') || chatText(msg.content).includes('Execution stopped')) && (
                        <div className="pt-1 text-ui-sm text-foreground-subtle">Stopped</div>
                      )}

                    </div>
                  );
                })()}

                </div>
              );
            })
          )}

          <div ref={chatBottomRef} className="h-28 sm:h-36 shrink-0" />
          </div>
        </div>

        {/* Bottom Follow-up Input Bar with Changed Files Card placed above.
            Shares the conversation reading measure (48rem, centered) with the
            message list so both align cleanly at medium–full widths. */}
        <div className={cn(
          "pt-1.5 pb-2.5 sm:pb-4 bg-background transition-colors w-full min-w-0 shrink-0",
          isRightActionDrawerOpen ? "px-2 sm:px-3" : "px-4 sm:px-6"
        )}>
          <div className="max-w-[var(--app-chat-max-width,48rem)] mx-auto w-full min-w-0 space-y-3">
            {/* Slim docked approval bar directly above input */}
            {activeSession.goSessionId && activeSession.goState === 'awaiting_approval' && (activeSession.pendingTools?.length ?? 0) > 0 && (
              <div className="w-full overflow-hidden rounded-xl border border-warning/30 shadow-xs">
                <AgentApprovalBar goSessionId={activeSession.goSessionId} tools={activeSession.pendingTools!} />
              </div>
            )}
            {activeSession.goState === 'awaiting_input' && (activeSession.pendingQuestions?.length ?? 0) > 0 && (
              <PendingQuestionsCard
                goSessionId={activeSession.goSessionId}
                acpSessionId={activeSession.acpSessionId}
                questions={activeSession.pendingQuestions!}
              />
            )}

            {/* Floating Jump to latest button */}
            {isScrolledUp && (
              <div className="flex justify-center w-full mb-2 pointer-events-none">
                <button
                  type="button"
                  onClick={() => scrollToBottom('smooth')}
                  className="pointer-events-auto flex items-center gap-1.5 rounded-full border border-border bg-popover/90 hover:bg-surface px-3 py-1 text-ui-xs font-medium text-foreground shadow-lg backdrop-blur-md transition-all cursor-pointer animate-in fade-in slide-in-from-bottom-2 duration-150 select-none"
                  title="Jump to latest message"
                >
                  <ArrowDown className="size-3.5 text-primary" />
                  <span>Jump to latest</span>
                  {activeSession?.status === 'running' && (
                    <span className="size-1.5 rounded-full bg-primary animate-pulse ml-0.5" />
                  )}
                </button>
              </div>
            )}

            {/* Unified Changed Files & Diff Review Dock */}
            {sessionDiffs.length > 0 && (
              <div className="@container/changed-files w-full rounded-xl border border-border/80 bg-panel/90 shadow-xs overflow-hidden transition-all">
                <div className="flex flex-wrap items-center justify-between gap-2 px-3 py-2 text-ui-xs select-none">
                  <div className="flex min-w-0 flex-1 items-center gap-2 overflow-x-auto scrollbar-none">
                    <button
                      type="button"
                      onClick={() => setIsDiffCardExpanded(prev => !prev)}
                      className="flex shrink-0 items-center gap-1.5 font-medium text-foreground cursor-pointer hover:text-primary transition-colors"
                    >
                      <ChevronRight
                        className={cn(
                          'size-3.5 shrink-0 text-muted-foreground/70 transition-transform',
                          isDiffCardExpanded ? 'rotate-90' : 'rotate-0'
                        )}
                      />
                      <span>
                        {sessionDiffs.length} {sessionDiffs.length === 1 ? 'file' : 'files'} changed
                      </span>
                      <span className="shrink-0 font-mono text-ui-xs tabular-nums">
                        <span className="text-diff-addition-foreground">+{totalAdditions}</span>{' '}
                        <span className="text-diff-deletion-foreground">-{totalDeletions}</span>
                      </span>
                    </button>

                    {!isDiffCardExpanded && (
                      <>
                        <div className="h-3.5 w-px bg-border/70 shrink-0" />
                        <div className="flex items-center gap-1.5 min-w-0">
                          {sessionDiffs.slice(0, 4).map(d => {
                            const fname = d.filePath.split('/').pop() || d.fileName;
                            return (
                              <button
                                key={d.id}
                                type="button"
                                onClick={() => openDiffInEditor(d)}
                                title={`Review ${d.filePath}`}
                                className="flex items-center gap-1.5 rounded-md border border-border/60 bg-surface px-2 py-0.5 text-ui-xs text-foreground-subtle hover:bg-surface-hover hover:text-foreground transition-colors cursor-pointer shrink-0"
                              >
                                <span className="shrink-0 text-muted-foreground/70">
                                  {getFileIcon(fname, 'size-3')}
                                </span>
                                <span className="truncate max-w-[140px] font-mono">{fname}</span>
                                <span className="font-mono text-2xs tabular-nums">
                                  <span className="text-diff-addition-foreground">+{d.additions}</span>{' '}
                                  <span className="text-diff-deletion-foreground">-{d.deletions}</span>
                                </span>
                              </button>
                            );
                          })}
                        </div>
                      </>
                    )}
                  </div>

                  <div className="flex shrink-0 items-center gap-1">
                    <button
                      type="button"
                      onClick={() => openDiffInEditor(sessionDiffs[0])}
                      className="flex items-center gap-1 rounded-md px-2 py-1 text-ui-xs text-secondary-label transition-colors hover:bg-accent hover:text-foreground cursor-pointer"
                      title="Open diff in the editor pane"
                    >
                      <FileDiffIcon className="size-3" />
                      <span className="hidden @[28rem]/changed-files:inline">Review diff</span>
                    </button>
                    <button
                      type="button"
                      onClick={() => void handleRejectAllReview()}
                      className="flex items-center gap-1 rounded-md px-2 py-1 text-ui-xs font-medium text-destructive transition-colors hover:bg-destructive/10 cursor-pointer"
                      title="Reject all changes (Restore pre-turn checkpoint)"
                    >
                      <X className="size-3" />
                      <span>Reject All</span>
                    </button>
                    <button
                      type="button"
                      onClick={handleAcceptAllReview}
                      className="flex items-center gap-1 rounded-md bg-foreground px-2.5 py-1 text-ui-xs font-medium text-background transition-opacity hover:opacity-90 cursor-pointer shadow-2xs"
                      title="Accept all changes"
                    >
                      <Check className="size-3" />
                      <span>Accept All</span>
                    </button>
                  </div>
                </div>

                {isDiffCardExpanded && (
                  <div className="max-h-56 space-y-0.5 overflow-y-auto border-t border-border/60 p-2">
                    {sessionDiffs.map(d => {
                      const parts = d.filePath.split('/');
                      const fname = parts.pop() || d.fileName;
                      const dir = parts.join('/') || 'root';
                      return (
                        <div
                          key={d.id}
                          className="group flex h-7 w-full cursor-pointer items-center gap-2 rounded-md py-1.5 pr-2 pl-2 text-left transition-colors hover:bg-accent/60"
                          onClick={() => {
                            if (d.filePath) {
                              openSideFile(d.filePath);
                            } else {
                              setActiveInlineDiff(d);
                            }
                          }}
                        >
                          <span className="shrink-0 text-muted-foreground/70">
                            {getFileIcon(fname, 'size-3.5')}
                          </span>
                          <span className="min-w-0 truncate font-mono text-ui-xs text-foreground/85 group-hover:text-foreground" title={d.filePath}>
                            {fname}
                          </span>
                          <span className="hidden truncate font-mono text-2xs text-muted-foreground/70 sm:inline" title={dir}>
                            {dir}
                          </span>
                          <span className="ml-auto shrink-0 font-mono text-2xs tabular-nums">
                            <span className="text-diff-addition-foreground">+{d.additions}</span>{' '}
                            <span className="text-diff-deletion-foreground">-{d.deletions}</span>
                          </span>
                        </div>
                      );
                    })}
                  </div>
                )}
              </div>
            )}

            <AgentTaskInputBar
              placeholder="Ask for follow-up changes"
              autoFocus={true}
              isCompact={true}
              fillWidth={isRightActionDrawerOpen}
            />
          </div>
        </div>

        {/* Bottom dock terminal (spans the conversation column) */}
        <BottomTerminal
          open={isBottomTerminalOpen}
          onToggle={() => setIsBottomTerminalOpen(false)}
          workspacePath={activeWorkspacePath || ''}
        />
      </div>

      {/* Inline Diff Overlay Modal if clicked */}
      {activeInlineDiff && (
        <div className="absolute inset-0 z-50 flex flex-col bg-background animate-in fade-in duration-100">
          <div className="h-[48px] px-6 bg-card border-b border-border flex items-center justify-between">
            <div className="flex items-center gap-2 text-sm font-semibold text-foreground">
              <Code2 className="w-4 h-4 text-success" />
              <span>Reviewing changes: {activeInlineDiff.fileName}</span>
              <span className="font-mono text-xs text-success">+{activeInlineDiff.additions}</span>
              <span className="font-mono text-xs text-destructive">-{activeInlineDiff.deletions}</span>
            </div>

            <div className="flex items-center gap-3">
              <button
                type="button"
                onClick={() => {
                  openDiffInEditor(activeInlineDiff);
                  setActiveInlineDiff(null);
                }}
                className="px-3 py-1.5 rounded-[8px] bg-surface-hover hover:bg-border text-foreground text-xs font-medium flex items-center gap-1.5 transition-colors cursor-pointer"
              >
                <ExternalLink className="w-3.5 h-3.5" />
                <span>Open in editor</span>
              </button>
              <button
                type="button"
                onClick={() => setActiveInlineDiff(null)}
                className="p-1.5 rounded-[6px] hover:bg-surface-hover text-foreground-subtle hover:text-foreground transition-colors cursor-pointer"
              >
                <X className="w-4 h-4" />
              </button>
            </div>
          </div>

          <div className="flex-1 min-h-0 p-4 flex flex-col overflow-hidden bg-card">
            <DiffViewer diff={activeInlineDiff} onClose={() => setActiveInlineDiff(null)} isInline={true} />
          </div>
        </div>
      )}

    </div>
  );
};
