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
  Image as ImageIcon,
  RotateCcw
} from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { cn } from '../../lib/utils';
import { EventsOn, RespondAgentApproval, RespondAgentAsk } from '../../lib/wails';

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

function getFileExtensionBadge(path: string) {
  const ext = (path.split('.').pop() || '').toLowerCase();
  switch (ext) {
    case 'ts':
    case 'tsx':
      return { label: ext.toUpperCase(), bg: 'bg-blue-500/15 text-blue-400 border border-blue-500/25' };
    case 'js':
    case 'jsx':
      return { label: ext.toUpperCase(), bg: 'bg-yellow-500/15 text-yellow-400 border border-yellow-500/25' };
    case 'css':
    case 'scss':
      return { label: 'CSS', bg: 'bg-purple-500/15 text-purple-400 border border-purple-500/25' };
    case 'html':
      return { label: 'HTML', bg: 'bg-orange-500/15 text-orange-400 border border-orange-500/25' };
    case 'json':
      return { label: 'JSON', bg: 'bg-emerald-500/15 text-emerald-400 border border-emerald-500/25' };
    case 'py':
      return { label: 'PY', bg: 'bg-teal-500/15 text-teal-400 border border-teal-500/25' };
    case 'go':
      return { label: 'GO', bg: 'bg-cyan-500/15 text-cyan-400 border border-cyan-500/25' };
    case 'md':
      return { label: 'MD', bg: 'bg-slate-500/15 text-slate-300 border border-slate-500/25' };
    default:
      return { label: ext.slice(0, 3).toUpperCase() || 'FILE', bg: 'bg-zinc-500/15 text-zinc-300 border border-zinc-500/25' };
  }
}
import { formatRelativeTime } from '../../lib/time';
import { FileDiff, ToolExecution, ThoughtStep, AgentMessage } from '../../types';

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
    <div className="w-full rounded-xl border border-warning/40 bg-warning/5 p-3 space-y-2.5 shadow-xs">
      <div className="flex items-center gap-2 text-ui-sm font-semibold text-warning">
        <AlertCircle className="size-4 shrink-0" />
        <span>Permission required — the agent wants to run:</span>
      </div>
      <div className="space-y-1.5">
        {tools.map((tc: any, i: number) => {
          const name = tc?.name || 'tool';
          let argsText = tc?.arguments ?? '{}';
          if (typeof argsText !== 'string') argsText = JSON.stringify(argsText, null, 2);
          return (
            <div key={tc?.tool_call_id || i} className="text-ui-xs font-mono bg-surface border border-border rounded-md p-2 overflow-x-auto whitespace-pre-wrap break-words max-h-40 overflow-y-auto">
              <div className="font-semibold text-warning mb-0.5">{name}</div>
              <div className="text-foreground-subtle">{argsText}</div>
            </div>
          );
        })}
      </div>
      <div className="flex items-center justify-end gap-2 pt-0.5">
        <button
          type="button"
          onClick={() => respond(false, false)}
          className="px-2.5 py-1 text-ui-xs text-foreground-subtle border border-border rounded-md hover:bg-surface-hover cursor-pointer"
        >
          Deny
        </button>
        <button
          type="button"
          onClick={() => respond(true, true)}
          title="Approve this and all future tool calls"
          className="px-2.5 py-1 text-ui-xs text-foreground-subtle border border-border rounded-md hover:bg-surface-hover cursor-pointer"
        >
          Always allow
        </button>
        <button
          type="button"
          onClick={() => respond(true, false)}
          className="px-3 py-1 text-ui-xs font-semibold bg-warning text-black rounded-md cursor-pointer"
        >
          Approve
        </button>
      </div>
    </div>
  );
}

function PendingQuestionsCard({ goSessionId, questions }: { goSessionId: string; questions: any[] }) {
  const [answers, setAnswers] = useState<Record<string, any>>({});
  const [submitted, setSubmitted] = useState(false);

  const submit = async () => {
    setSubmitted(true);
    try {
      await RespondAgentAsk(goSessionId, answers);
    } catch (err) {
      console.error('failed to submit answers:', err);
      setSubmitted(false);
    }
  };

  return (
    <div className="w-full rounded-xl border border-info/40 bg-info/5 p-3 space-y-2.5 shadow-xs">
      <div className="flex items-center gap-2 text-ui-sm font-semibold text-info">
        <Brain className="size-4 shrink-0" />
        <span>The agent needs your input</span>
      </div>
      {questions.map((q: any, qi: number) => (
        <div key={q.id || qi} className="space-y-1.5">
          <div className="text-ui-sm font-medium text-foreground">{q.question}</div>
          <div className="flex flex-wrap gap-1.5">
            {(q.options || []).map((opt: string, oi: number) => {
              const selected = Array.isArray(answers[q.id]) ? answers[q.id].includes(opt) : answers[q.id] === opt;
              const toggle = () => {
                if (q.multi) {
                  const cur: string[] = Array.isArray(answers[q.id]) ? answers[q.id] : [];
                  setAnswers(p => ({ ...p, [q.id]: cur.includes(opt) ? cur.filter((c: string) => c !== opt) : [...cur, opt] }));
                } else {
                  setAnswers(p => ({ ...p, [q.id]: opt }));
                }
              };
              return (
                <button
                  key={oi}
                  type="button"
                  onClick={toggle}
                  className={`px-2 py-1 text-ui-xs rounded-md border cursor-pointer transition-colors ${
                    selected
                      ? 'bg-info/20 border-info text-info'
                      : 'bg-surface border-border text-foreground-subtle hover:bg-surface-hover'
                  }`}
                >
                  {opt}
                  {!q.multi && oi === (q.recommended ?? -1) && <span className="ml-1 opacity-70">(Recommended)</span>}
                </button>
              );
            })}
          </div>
        </div>
      ))}
      <div className="flex items-center justify-end pt-0.5">
        <button
          type="button"
          onClick={submit}
          disabled={submitted}
          className="px-3 py-1 text-ui-xs font-semibold bg-info text-black rounded-md cursor-pointer disabled:opacity-50"
        >
          Submit answers
        </button>
      </div>
    </div>
  );
}
import { AgentTaskInputBar } from './AgentTaskInputBar';
import { AgentRightSidebar, type SidePaneTab } from './AgentRightSidebar';
import { BottomTerminal } from './BottomTerminal';
import { PanelResizeHandle } from './PanelResizeHandle';
import { DiffViewer } from '../diff/DiffViewer';
import { MarkdownRenderer } from './MarkdownRenderer';
import { cleanPiBanner } from '../../lib/utils';

export const AgentActiveSessionView: React.FC = () => {
  const { 
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
    rightPaneWidth,
    setRightPaneWidth,
    openSideFile
  } = useWorkspace();

  const [expandedThoughts, setExpandedThoughts] = useState<Record<string, boolean>>({});
  const [expandedTools, setExpandedTools] = useState<Record<string, boolean>>({});
  const [expandedWork, setExpandedWork] = useState<Record<string, boolean>>({});
  const [expandedExplore, setExpandedExplore] = useState<Record<string, boolean>>({});
  const [activeInlineDiff, setActiveInlineDiff] = useState<FileDiff | null>(null);
  const [isDiffCardExpanded, setIsDiffCardExpanded] = useState<boolean>(false);
  // Side pane tab requested from outside (header terminal/side-pane buttons).
  const [sidePaneTab, setSidePaneTab] = useState<SidePaneTab | null>(null);
  const [sidePaneSignal, setSidePaneSignal] = useState(0);
  // The floating turn-scrubber rail overlaps the transcript in narrow panes —
  // only render it when the chat column has room (720px+).
  const [chatColumnWide, setChatColumnWide] = useState(true);
  const chatColumnRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const el = chatColumnRef.current;
    if (!el) return;
    const ro = new ResizeObserver((entries) => {
      for (const entry of entries) {
        setChatColumnWide(entry.contentRect.width >= 720);
      }
    });
    ro.observe(el);
    return () => ro.disconnect();
  }, []);
  // Plain toggle opens the pane EMPTY; a tab argument requests that tab
  // (signal bump opens it in the pane, including across remounts).
  const openSidePaneTab = useCallback((tab?: SidePaneTab) => {
    if (!tab) {
      setIsRightActionDrawerOpen(prev => !prev);
      return;
    }
    setSidePaneTab(tab);
    setSidePaneSignal(s => s + 1);
    setIsRightActionDrawerOpen(true);
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
  // Bottom dock terminal (ZCode AnimatedTerminalPanel) — state lives in the
  // workspace store; the full-width header in AgentContainer owns the toggle.
  const [copiedMsgId, setCopiedMsgId] = useState<string | null>(null);
  const [likedMsgs, setLikedMsgs] = useState<Record<string, 'up' | 'down' | null>>({});
  const chatBottomRef = useRef<HTMLDivElement>(null);
  const chatScrollRef = useRef<HTMLDivElement>(null);
  const userScrolledUpRef = useRef(false);
  const lastMsgCountRef = useRef(0);
  const [activeTurnIndex, setActiveTurnIndex] = useState<number>(0);
  const [hoveredTurnIndex, setHoveredTurnIndex] = useState<number | null>(null);

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

  // Update active turn based on scroll position and track user scroll intent
  const handleChatScroll = useCallback(() => {
    if (!chatScrollRef.current) return;
    const { scrollTop, scrollHeight, clientHeight } = chatScrollRef.current;

    // User is considered scrolled up if more than 100px from the bottom
    const distanceFromBottom = scrollHeight - (scrollTop + clientHeight);
    userScrolledUpRef.current = distanceFromBottom > 100;

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
      userScrolledUpRef.current = false;
      if (chatScrollRef.current) {
        chatScrollRef.current.scrollTop = chatScrollRef.current.scrollHeight;
      }
    }
    lastMsgCountRef.current = count;
  }, [activeSession?.messages?.length]);

  if (!activeSession) return null;

  const toggleThought = (id: string) => {
    setExpandedThoughts(prev => ({
      ...prev,
      [id]: prev[id] === undefined ? false : !prev[id]
    }));
  };

  const toggleTool = (id: string) => {
    setExpandedTools(prev => ({
      ...prev,
      [id]: !prev[id]
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

  const sessionDiffs = activeSession.diffs && activeSession.diffs.length > 0 
    ? activeSession.diffs 
    : diffs;

  const totalAdditions = sessionDiffs.reduce((acc, d) => acc + (d.additions || 0), 0);
  const totalDeletions = sessionDiffs.reduce((acc, d) => acc + (d.deletions || 0), 0);

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

    if (t.command) {
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

    target = target.replace(/\\/g, '/');

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
    const attachmentRegex = /\[Attached (Image|File):\s*([^\]]+)\]/gi;
    let match: RegExpExecArray | null;
    while ((match = attachmentRegex.exec(rawContent)) !== null) {
      const type = match[1].toLowerCase() === 'image' ? 'image' : 'file';
      const rawPath = match[2].trim();
      const cleanPath = rawPath.replace(/\\/g, '/');
      const name = cleanPath.split('/').filter(Boolean).pop() || cleanPath;
      attachments.push({ type, path: cleanPath, name });
    }

    let text = rawContent.replace(/\[Attached (?:Image|File):\s*[^\]]+\]/gi, '');
    text = text.replace(/\[Attached File:[^\]]*\][\s\S]*?(?=\[Attached|$)/gi, '');
    text = text.trim();

    return { text, attachments };
  };

  const isExploreTool = (t: ToolExecution) => {
    const name = (t.toolName || '').toLowerCase();
    // Mutating / command tools are never "explore" even when their args JSON
    // contains a "path" key (edit/write always do).
    if (name.includes('edit') || name.includes('write') || name.includes('create') ||
        name.includes('delet') || name.includes('bash') || name.includes('shell') ||
        name.includes('exec') || name.includes('run') || name.includes('todo') || name.includes('ask')) {
      return false;
    }
    const cmd = (t.command || '').toLowerCase();
    return name.includes('read') || name.includes('search') || name.includes('grep') ||
           name.includes('find') || name.includes('explore') || name.includes('list') ||
           name.includes('view') || cmd.includes('"path"') || cmd.includes('"query"') ||
           cmd.startsWith('read') || cmd.startsWith('rg') || cmd.startsWith('find') || cmd.startsWith('cat');
  };

  interface MessageTurnGroup {
    turn: number;
    thoughts: ThoughtStep[];
    exploreTools: ToolExecution[];
    commandTools: ToolExecution[];
    isThinking?: boolean;
  }

  const organizeMessageTurns = (msg: AgentMessage): MessageTurnGroup[] => {
    const thoughts = msg.thoughts || [];
    const tools = msg.toolExecutions || [];

    const extractTurn = (item: { turn?: number; id?: string }, fallback: number): number => {
      if (typeof item.turn === 'number' && item.turn > 0) return item.turn;
      if (item.id) {
        const match = item.id.match(/-(?:turn-)?(\d+)$/);
        if (match) {
          const parsed = parseInt(match[1], 10);
          if (parsed > 0) return parsed;
        }
      }
      return fallback;
    };

    const turnMap = new Map<number, {
      thoughts: ThoughtStep[];
      exploreTools: ToolExecution[];
      commandTools: ToolExecution[];
    }>();

    const getGroup = (turnNum: number) => {
      if (!turnMap.has(turnNum)) {
        turnMap.set(turnNum, { thoughts: [], exploreTools: [], commandTools: [] });
      }
      return turnMap.get(turnNum)!;
    };

    // 1. Group thoughts
    thoughts.forEach((th, idx) => {
      const tNum = extractTurn(th, idx + 1);
      getGroup(tNum).thoughts.push(th);
    });

    // 2. Group tools
    tools.forEach((tool) => {
      let tNum = extractTurn(tool, 0);
      if (tNum === 0) {
        if (thoughts.length > 1 && tool.createdAtMs && thoughts[1]?.createdAtMs) {
          tNum = tool.createdAtMs >= thoughts[1].createdAtMs ? 2 : 1;
        } else {
          tNum = 1;
        }
      }
      const group = getGroup(tNum);
      if (isExploreTool(tool)) {
        group.exploreTools.push(tool);
      } else {
        group.commandTools.push(tool);
      }
    });

    // 3. Active / streaming turn if currently thinking
    const activeTurn = msg.currentTurn || (turnMap.size > 0 ? Math.max(...Array.from(turnMap.keys())) : 1);
    if (msg.isThinking) {
      getGroup(activeTurn);
    }

    if (turnMap.size === 0) {
      getGroup(1);
    }

    const sortedNums = Array.from(turnMap.keys()).sort((a, b) => a - b);

    return sortedNums.map(turnNum => {
      const data = turnMap.get(turnNum)!;
      const isThisTurnThinking = !!(msg.isThinking && turnNum === activeTurn);
      return {
        turn: turnNum,
        thoughts: data.thoughts,
        exploreTools: data.exploreTools,
        commandTools: data.commandTools,
        isThinking: isThisTurnThinking
      };
    });
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

        {/* Turn Scrubber / Timeline Tracker (floating on the left edge —
            hidden in narrow panes where it would overlap the transcript) */}
        {conversationTurns.length > 0 && chatColumnWide && (
          <nav
            aria-label="Conversation turn timeline tracker"
            className="absolute left-4 md:left-5 top-1/2 -translate-y-1/2 z-30 flex flex-col items-start gap-3 py-3 px-1"
          >
            {conversationTurns.map((turn, idx) => {
              const isActive = activeTurnIndex === idx;
              const isHovered = hoveredTurnIndex === idx;

              return (
                <div key={turn.userMessageId} className="relative flex items-center">
                  {/* Tick line mark */}
                  <button
                    type="button"
                    onClick={() => {
                      const el = document.getElementById(`turn-${turn.userMessageId}`);
                      if (el) el.scrollIntoView({ behavior: 'smooth', block: 'start' });
                    }}
                    onMouseEnter={() => setHoveredTurnIndex(idx)}
                    onMouseLeave={() => setHoveredTurnIndex(null)}
                    className={`h-[2.5px] rounded-full transition-all duration-200 cursor-pointer block ${
                      isActive
                        ? 'w-5 bg-foreground shadow-xs'
                        : 'w-3.5 bg-foreground-subtlest hover:w-5 hover:bg-foreground-subtle'
                    }`}
                    title={`Turn ${idx + 1}: ${turn.userPrompt.slice(0, 40)}`}
                  />

                  {/* Hover Preview Card Popup */}
                  {isHovered && (
                    <div 
                      className="absolute left-8 top-1/2 -translate-y-1/2 w-72 rounded-[10px] bg-card border border-border p-3.5 shadow-2xl z-50 text-left pointer-events-none animate-in fade-in zoom-in-95 duration-100"
                    >
                      <div className="text-ui-base font-semibold text-foreground line-clamp-2 leading-snug">
                        {turn.userPrompt}
                      </div>
                      {turn.agentPreview && (
                        <div className="text-ui-sm text-foreground-subtle line-clamp-3 leading-relaxed mt-1.5 font-normal">
                          {turn.agentPreview}
                        </div>
                      )}
                    </div>
                  )}
                </div>
              );
            })}
          </nav>
        )}

        {/* Chat Transcript Area */}
        <div
          ref={chatScrollRef}
          onScroll={handleChatScroll}
          className="flex-1 overflow-y-auto overflow-x-hidden pt-6 pb-6 px-4 sm:px-6 space-y-6 select-text relative w-full min-w-0"
        >
          <div className="max-w-[var(--app-chat-max-width,48rem)] mx-auto w-full min-w-0 space-y-6">
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
                  className="w-full max-w-full min-w-0 space-y-4 scroll-mt-6"
                >
                  
                  {/* USER MESSAGE BUBBLE (Kybern style: 80% width max, rounded squircle, subtle translucent background) */}
                  {isUser && (() => {
                    const { text, attachments } = parseUserMessage(msg.content);
                    return (
                      <div className="group/user-row w-full max-w-full min-w-0 flex flex-col items-end">
                        <div className="flex max-w-[80%] flex-col gap-1.5 rounded-[var(--radius-user-message,1.2rem)] border border-transparent bg-[var(--app-user-message-background)] px-3.5 py-2 text-[length:var(--app-font-size-chat,13px)] text-foreground select-text">
                        {text && (
                          <p className="text-ui-base leading-[1.5] text-foreground font-normal whitespace-pre-wrap break-words [overflow-wrap:anywhere] select-text min-w-0">
                            {text}
                          </p>
                        )}
                        {attachments.length > 0 && (
                          <div className="flex flex-wrap justify-end gap-1.5 max-w-full min-w-0 pt-1">
                            {attachments.map((att, i) => (
                              <div
                                key={i}
                                className="flex items-center gap-1.5 px-2.5 py-1 rounded-full bg-[var(--color-background-button-secondary)] text-ui-xs font-mono text-foreground max-w-full min-w-0 border border-border/40"
                                title={att.path}
                              >
                                {att.type === 'image' ? (
                                  <ImageIcon className="size-3.5 text-info shrink-0" />
                                ) : (
                                  <FileCode className="size-3.5 text-success shrink-0" />
                                )}
                                <span className="font-medium truncate max-w-[180px]">{att.name}</span>
                              </div>
                            ))}
                          </div>
                        )}
                        </div>
                        <div className="mt-1 flex items-center gap-1.5 pr-1 opacity-0 transition-opacity group-hover/user-row:opacity-100">
                          <button
                            type="button"
                            onClick={() => handleCopy(text, msg.id || `u-${index}`)}
                            className="rounded p-1 text-muted-foreground hover:bg-[var(--color-background-button-secondary-hover)] hover:text-foreground transition-colors"
                            title="Copy message"
                          >
                            {copiedMsgId === (msg.id || `u-${index}`) ? <Check className="size-3.5 text-success" /> : <Copy className="size-3.5" />}
                          </button>
                        </div>
                      </div>
                    );
                  })()}

                  {/* AGENT RESPONSE & STREAMING STEPS */}
                  {!isUser && (() => {
                    const turns = organizeMessageTurns(msg);

                    return (
                      <div className="flex flex-col items-start space-y-4 w-full max-w-full min-w-0">
                        {turns.map(turnGroup => {
                          const hasThoughts = turnGroup.thoughts.length > 0;
                          const hasExplore = turnGroup.exploreTools.length > 0;
                          const hasCommands = turnGroup.commandTools.length > 0;
                          const showThinkingPlaceholder = turnGroup.isThinking && !hasThoughts;
                          const wbId = `wb-${msg.id || index}-${turnGroup.turn}`;
                          const workOpen = expandedWork[wbId] !== undefined ? expandedWork[wbId] : true;
                          const workSecs = turnGroup.thoughts.reduce((a, th) => a + (th.durationSeconds || 0), 0)
                            || (turnGroup.exploreTools.length + turnGroup.commandTools.length) || 3;
                          const hasWork = hasThoughts || hasExplore || hasCommands;

                          if (!hasThoughts && !hasExplore && !hasCommands && !showThinkingPlaceholder) {
                            return null;
                          }

                          return (
                            <div key={`turn-grp-${msg.id || index}-${turnGroup.turn}`} className="w-full max-w-full min-w-0 space-y-3">
                              {/* 0. WORKED-FOR HEADER (collapsible work block) */}
                              {hasWork && (
                                <button
                                  type="button"
                                  aria-expanded={workOpen}
                                  onClick={() => setExpandedWork(prev => ({ ...prev, [wbId]: !prev[wbId] }))}
                                  className="group/wb flex items-center gap-2 py-1 text-ui-lg cursor-pointer w-fit"
                                >
                                  <span
                                    role="status"
                                    aria-live="polite"
                                    className={`font-medium ${turnGroup.isThinking ? 'animated-gradient-text' : 'text-foreground-subtle group-hover/wb:text-foreground dark:group-hover/wb:text-foreground'}`}
                                  >
                                    {turnGroup.isThinking ? 'Working…' : `Worked for ${workSecs}s`}
                                  </span>
                                  {workOpen ? (
                                    <ChevronDown className="w-3.5 h-3.5 text-foreground-subtlest opacity-0 group-hover/wb:opacity-100 transition-opacity" />
                                  ) : (
                                    <ChevronRight className="w-3.5 h-3.5 text-foreground-subtlest" />
                                  )}
                                </button>
                              )}
                              {/* 1. THOUGHTS IN THIS TURN */}
                              {workOpen && hasThoughts ? (
                                <div className="w-full max-w-full min-w-0 space-y-2">
                                  {turnGroup.thoughts.map(th => {
                                    const isOpen = expandedThoughts[th.id] !== undefined ? expandedThoughts[th.id] : false;
                                    return (
                                      <div key={th.id} className="w-full max-w-full min-w-0">
                                        <button
                                          type="button"
                                          aria-expanded={isOpen}
                                          onClick={() => toggleThought(th.id)}
                                          className="flex items-center gap-2 text-ui-lg text-foreground-subtle hover:text-foreground transition-colors cursor-pointer py-1 group w-full max-w-full min-w-0"
                                        >
                                          <Brain className="w-4 h-4 text-foreground-subtle shrink-0" />
                                          <span className="font-medium text-foreground-subtle">Thought</span>
                                          <span className="text-foreground-subtlest">·</span>
                                          <span className="text-ui-lg text-foreground-subtlest">
                                            {th.durationSeconds && th.durationSeconds >= 10 ? `${th.durationSeconds}s` : 'a few seconds'}
                                          </span>
                                          {turnGroup.isThinking && (
                                            <span className="text-ui-sm text-warning animate-pulse font-mono">
                                              streaming...
                                            </span>
                                          )}
                                          {isOpen ? (
                                            <ChevronDown className="w-3.5 h-3.5 text-foreground-subtlest group-hover:text-foreground dark:group-hover:text-foreground-subtle transition-colors" />
                                          ) : (
                                            <ChevronRight className="w-3.5 h-3.5 text-foreground-subtlest group-hover:text-foreground dark:group-hover:text-foreground-subtle transition-colors" />
                                          )}
                                        </button>

                                        {isOpen && (
                                          <div className="border-l-2 border-border pl-4 py-1.5 my-1 text-ui-base/6 text-foreground-subtle select-text max-h-[280px] overflow-y-auto overflow-x-hidden pr-2 scrollbar-thin w-full max-w-full min-w-0 break-words [overflow-wrap:anywhere]">
                                            <MarkdownRenderer content={th.thoughtText} />
                                          </div>
                                        )}
                                      </div>
                                    );
                                  })}
                                </div>
                              ) : showThinkingPlaceholder ? (
                                <div className="flex items-center gap-2 text-ui-base text-warning py-1">
                                  <Brain className="w-4 h-4 text-warning animate-pulse shrink-0" />
                                  <span className="font-medium">Thinking...</span>
                                  <span className="text-foreground-subtlest">·</span>
                                  <span className="text-ui-lg text-foreground-subtlest">streaming</span>
                                </div>
                              ) : null}

                              {/* 2. EXPLORE TOOLS IN THIS TURN */}
                              {workOpen && hasExplore && (
                                <div className="w-full max-w-full min-w-0">
                                  {(() => {
                                    const exploreId = `explore-${msg.id || index}-${turnGroup.turn}`;
                                    const isExpOpen = expandedExplore[exploreId] !== undefined ? expandedExplore[exploreId] : true;

                                    return (
                                      <div className="w-full max-w-full min-w-0">
                                        <button
                                          type="button"
                                          onClick={() => toggleExplore(exploreId)}
                                          className="flex items-center gap-2 text-ui-lg text-foreground-subtle hover:text-foreground transition-colors cursor-pointer py-1 group w-full max-w-full min-w-0"
                                        >
                                          <Search className="w-4 h-4 text-foreground-subtle shrink-0" />
                                          <span className="font-medium text-foreground-subtle">Explore</span>
                                          <span className="text-foreground-subtlest">·</span>
                                          <span className="text-ui-lg text-foreground-subtlest">
                                            {turnGroup.exploreTools.length} {turnGroup.exploreTools.length === 1 ? 'file' : 'files'}
                                          </span>
                                          {isExpOpen ? (
                                            <ChevronDown className="w-3.5 h-3.5 text-foreground-subtlest group-hover:text-foreground dark:group-hover:text-foreground-subtle transition-colors" />
                                          ) : (
                                            <ChevronRight className="w-3.5 h-3.5 text-foreground-subtlest group-hover:text-foreground dark:group-hover:text-foreground-subtle transition-colors" />
                                          )}
                                        </button>

                                        {isExpOpen && (
                                          <div className="border-l-2 border-border pl-4 py-2 my-1 flex flex-col gap-2.5 w-full max-w-full min-w-0 overflow-hidden">
                                            {turnGroup.exploreTools.map(t => {
                                              const info = extractExploreInfo(t, activeSession.workspacePath);

                                              return (
                                                <div key={t.id} className="flex items-center gap-2.5 text-ui-lg w-full max-w-full min-w-0">
                                                  <span className="text-ui-lg text-foreground-subtlest w-12 shrink-0">
                                                    {info.action}
                                                  </span>
                                                  <FileCode className="w-3.5 h-3.5 text-success shrink-0" />
                                                  <span className="font-mono text-ui-lg text-foreground font-medium truncate max-w-[220px] shrink-0" title={info.fullPath}>
                                                    {info.filename}
                                                  </span>
                                                  {info.dir && (
                                                    <span className="font-mono text-ui-base text-foreground-subtlest truncate min-w-0 flex-1" title={info.fullPath}>
                                                      {info.dir}/
                                                    </span>
                                                  )}
                                                  {t.status === 'running' && (
                                                    <Loader2 className="w-3 h-3 animate-spin text-warning shrink-0 ml-1" />
                                                  )}
                                                </div>
                                              );
                                            })}
                                          </div>
                                        )}
                                      </div>
                                    );
                                  })()}
                                </div>
                              )}

                              {/* 3. COMMAND TOOLS IN THIS TURN */}
                              {workOpen && hasCommands && (
                                <div className="w-full max-w-full min-w-0 space-y-2">
                                  {turnGroup.commandTools.map(t => {
                                    const isToolOpen = expandedTools[t.id] || false;
                                    const isDone = t.status === 'completed';
                                    const isRunning = t.status === 'running';
                                    const isFailed = t.status === 'failed';

                                    const nameLower = (t.toolName || '').toLowerCase();
                                    const isBash = nameLower.includes('bash') || nameLower.includes('shell') || nameLower.includes('exec') || nameLower.includes('command');
                                    const isEdit = nameLower.includes('edit');
                                    const isWrite = nameLower.includes('write') || nameLower.includes('create');

                                    // Per-tool kind label + detail (matching ZCode ToolCallBlocks).
                                    let kind = 'Ran';
                                    let Icon = SquareTerminal;
                                    if (isEdit) { kind = 'Updated'; Icon = FilePen; }
                                    else if (isWrite) { kind = 'Created'; Icon = FilePlus; }
                                    else if (!isBash) { kind = (t.toolName || 'Tool').replace(/_/g, ' ').replace(/^\w/, c => c.toUpperCase()); Icon = Wrench; }

                                    let detail = t.command || '';
                                    if (isEdit || isWrite) {
                                      const raw = t.command || '';
                                      let target = '';
                                      if (raw.trim().startsWith('{')) {
                                        try { const p = JSON.parse(raw); target = p.file_path || p.filePath || p.path || ''; } catch { /* keep raw */ }
                                      }
                                      if (!target) target = t.diff?.filePath || raw;
                                      const parts = String(target).split('/').filter(Boolean);
                                      detail = parts.pop() || String(target);
                                    } else {
                                      const nl = detail.indexOf('\n');
                                      if (nl > 0) detail = detail.slice(0, nl) + '…';
                                    }
                                    const fileDir = (isEdit || isWrite && t.diff?.filePath)
                                      ? String(t.diff?.filePath || '').split('/').slice(0, -1).join('/') : '';

                                    return (
                                      <div key={t.id} className="w-full max-w-full min-w-0">
                                        <button
                                          type="button"
                                          onClick={() => { if (t.output || t.diff) toggleTool(t.id); }}
                                          className="group/tool-summary inline-flex max-w-full cursor-pointer items-center gap-2 self-start text-left text-ui-lg py-0.5 w-full"
                                        >
                                          <Icon className="w-4 h-4 shrink-0 text-foreground-subtlest" />
                                          <span className={`whitespace-nowrap font-medium shrink-0 ${isRunning ? 'animated-gradient-text' : 'text-foreground-subtlest'}`}>
                                            {kind}
                                          </span>
                                          <span className="truncate text-foreground-subtlest min-w-0 flex-1" title={t.command}>
                                            <span className="font-mono">{detail}</span>
                                            {fileDir ? <span className="text-foreground-subtlest"> {fileDir}/</span> : null}
                                          </span>
                                          {t.diff ? (
                                            <span className="shrink-0 font-mono text-ui-xs leading-none">
                                              <span className="text-success">+{t.diff.additions}</span>{' '}
                                              <span className="text-destructive">-{t.diff.deletions}</span>
                                            </span>
                                          ) : null}
                                          {isRunning ? (
                                            <span className="shrink-0 text-ui-sm text-foreground-subtlest">Running…</span>
                                          ) : null}
                                          {isDone ? (
                                            <span className="shrink-0 text-ui-sm text-foreground-subtlest">Done</span>
                                          ) : null}
                                          {isFailed ? (
                                            <span className="shrink-0 text-ui-sm text-destructive underline decoration-dotted underline-offset-2">Failed</span>
                                          ) : null}
                                          {(t.output || t.diff) ? (
                                            <ChevronRight className={`w-3.5 h-3.5 shrink-0 text-foreground-subtlest opacity-0 group-hover/tool-summary:opacity-100 transition-transform ${isToolOpen ? 'rotate-90' : ''}`} />
                                          ) : null}
                                        </button>

                                        {isToolOpen && t.output && (
                                          <div className="mt-1 ml-6 p-3 rounded-lg bg-card border border-white/10 dark:border-white/10 text-ui-sm font-mono text-foreground-secondary leading-relaxed max-h-60 max-w-[calc(100%-1.5rem)] overflow-y-auto overflow-x-auto whitespace-pre-wrap break-words [overflow-wrap:anywhere] select-text">
                                            {t.output}
                                          </div>
                                        )}
                                      </div>
                                    );
                                  })}
                                </div>
                              )}
                            </div>
                          );
                        })}

                      {/* 4. AGENT RESPONSE MARKDOWN */}
                      {chatText(msg.content) && cleanPiBanner(chatText(msg.content)) && (
                        <div className="text-ui-base/6 text-foreground select-text py-1 w-full max-w-full min-w-0 break-words [overflow-wrap:anywhere] leading-relaxed">
                          <MarkdownRenderer content={cleanPiBanner(chatText(msg.content))} />
                        </div>
                      )}

                      {/* 6. ACTION FOOTER */}
                      <div className="w-full max-w-full min-w-0 flex items-center gap-4 pt-1 text-foreground-subtle">
                        <button
                          type="button"
                          onClick={() => handleCopy(chatText(msg.content), msg.id)}
                          className="hover:text-foreground transition-colors cursor-pointer p-1"
                          title="Copy response"
                        >
                          {isCopied ? (
                            <Check className="w-4 h-4 text-success" />
                          ) : (
                            <Copy className="w-4 h-4" />
                          )}
                        </button>

                        <button
                          type="button"
                          onClick={() => handleThumb(msg.id, 'up')}
                          className={`transition-colors cursor-pointer p-1 ${
                            isMsgLiked === 'up' ? 'text-success' : 'hover:text-foreground'
                          }`}
                          title="Good response"
                        >
                          <ThumbsUp className="w-4 h-4" />
                        </button>

                        <button
                          type="button"
                          onClick={() => handleThumb(msg.id, 'down')}
                          className={`transition-colors cursor-pointer p-1 ${
                            isMsgLiked === 'down' ? 'text-destructive' : 'hover:text-foreground'
                          }`}
                          title="Bad response"
                        >
                          <ThumbsDown className="w-4 h-4" />
                        </button>

                        <div className="text-ui-base font-mono text-foreground-subtlest">
                          {formatRelativeTime(msg.timestamp)}
                        </div>
                      </div>

                      {/* Turn end marker */}
                      {index === activeSession.messages.length - 1 && activeSession.status !== 'running' && (
                        <div className="pt-1 text-ui-sm text-foreground-subtle">Stopped</div>
                      )}

                    </div>
                  );
                })()}

                </div>
              );
            })
          )}

          <div ref={chatBottomRef} />
          </div>
        </div>

        {/* Bottom Follow-up Input Bar with Changed Files Card placed above.
            Shares the conversation reading measure (48rem, centered) with the
            message list so both align like ZCode at medium–full widths. */}
        <div className="pt-2 pb-5 px-4 sm:px-6 bg-background transition-colors w-full min-w-0">
          <div className="max-w-[var(--app-chat-max-width,48rem)] mx-auto w-full min-w-0 space-y-3">
            {/* Go harness pause cards: approval gate + structured questions */}
            {activeSession.goSessionId && activeSession.goState === 'awaiting_approval' && (activeSession.pendingTools?.length ?? 0) > 0 && (
              <PendingApprovalCard goSessionId={activeSession.goSessionId} tools={activeSession.pendingTools!} />
            )}
            {activeSession.goSessionId && activeSession.goState === 'awaiting_input' && (activeSession.pendingQuestions?.length ?? 0) > 0 && (
              <PendingQuestionsCard goSessionId={activeSession.goSessionId} questions={activeSession.pendingQuestions!} />
            )}

            {/* Changed Files Summary Card (ZCode ConversationFileSummaryPanel matching) */}
            {sessionDiffs.length > 0 && (
              <div className="w-full rounded-xl border border-border bg-card shadow-xs overflow-hidden transition-all">
                <div className="flex h-10 items-center justify-between gap-3 px-3 transition-colors hover:bg-surface-hover">
                  <button
                    type="button"
                    onClick={() => setIsDiffCardExpanded(prev => !prev)}
                    className="flex h-full min-w-0 flex-1 items-center gap-2 text-left text-ui-base text-foreground cursor-pointer"
                  >
                    <ChevronRight
                      className={cn(
                        "size-3.5 shrink-0 text-foreground-subtlest transition-transform",
                        isDiffCardExpanded ? "rotate-90" : "rotate-0"
                      )}
                    />
                    <span className="min-w-0 truncate font-medium text-ui-sm">
                      {sessionDiffs.length} {sessionDiffs.length === 1 ? 'file' : 'files'} changed
                    </span>
                    <span className="shrink-0 font-mono text-ui-xs">
                      <span className="text-diff-added font-medium">+{totalAdditions}</span>{' '}
                      <span className="text-diff-removed font-medium">-{totalDeletions}</span>
                    </span>
                  </button>

                  <div className="flex items-center gap-1.5 shrink-0">
                    <button
                      type="button"
                      onClick={() => setActiveInlineDiff(sessionDiffs[0])}
                      className="flex items-center gap-1.5 px-2 py-1 rounded-md text-ui-xs text-foreground-subtle hover:text-foreground hover:bg-surface-hover transition-colors cursor-pointer"
                      title="Rewind / Undo changes"
                    >
                      <RotateCcw className="size-3" />
                      <span>Rewind</span>
                    </button>
                    <span className="text-foreground-subtlest text-ui-xs">·</span>
                    <button
                      type="button"
                      onClick={() => openSidePaneTab('review')}
                      className="px-2 py-1 rounded-md text-ui-xs text-foreground-subtle hover:text-foreground hover:bg-surface-hover transition-colors cursor-pointer"
                    >
                      Git
                    </button>
                  </div>
                </div>

                {isDiffCardExpanded && (
                  <div className="border-t border-border p-2 space-y-1 bg-surface/30 max-h-60 overflow-y-auto">
                    {sessionDiffs.map(d => {
                      const parts = d.filePath.split('/');
                      const fname = parts.pop() || d.fileName;
                      const dir = parts.join('/') || 'root';
                      const badge = getFileExtensionBadge(d.filePath);

                      return (
                        <div
                          key={d.id}
                          className="flex items-center justify-between p-2 rounded-lg hover:bg-surface-hover transition-colors group cursor-pointer"
                          onClick={() => {
                            if (d.filePath) {
                              openSideFile(d.filePath);
                            } else {
                              setActiveInlineDiff(d);
                            }
                          }}
                        >
                          <div className="flex items-center gap-2 min-w-0 flex-1 pr-2">
                            <span className={cn('flex size-5 items-center justify-center rounded text-ui-xs font-mono font-bold shrink-0', badge.bg)}>
                              {badge.label}
                            </span>
                            <span className="text-xs font-medium text-foreground font-mono truncate">
                              {fname}
                            </span>
                            <span className="text-ui-xs text-foreground-subtlest font-mono truncate">
                              {dir}/
                            </span>
                          </div>

                          <div className="flex items-center gap-2 shrink-0">
                            <span className="font-mono text-ui-xs">
                              <span className="text-diff-added">+{d.additions}</span>{' '}
                              <span className="text-diff-removed">-{d.deletions}</span>
                            </span>
                            <button
                              type="button"
                              onClick={(e) => {
                                e.stopPropagation();
                                openDiffInEditor(d);
                              }}
                              className="px-2 py-0.5 rounded-md bg-surface border border-border/50 text-foreground-subtle hover:text-foreground hover:bg-surface-hover text-ui-xs transition-colors"
                            >
                              Open
                            </button>
                          </div>
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
            />
          </div>
        </div>

        {/* Bottom dock terminal (spans the conversation column, ZCode-style) */}
        <BottomTerminal
          open={isBottomTerminalOpen}
          onToggle={() => setIsBottomTerminalOpen(false)}
          workspacePath={activeWorkspacePath || ''}
        />
      </div>

      {/* Right Sidebar (Findings Panel / Review / Terminal / Side Chat) */}
      {isRightActionDrawerOpen && (
        <>
          <PanelResizeHandle
            edge="left"
            ariaLabel="Resize side panel"
            getStartWidth={() => rightPaneWidth}
            onResize={setRightPaneWidth}
            min={360}
            max={() => Math.max(360, Math.floor(window.innerWidth * 0.6))}
            className="-mr-1"
          />
          <AgentRightSidebar
            onOpenDiff={(d) => setActiveInlineDiff(d)}
            initialTab={sidePaneTab}
            openSignal={sidePaneSignal}
          />
        </>
      )}

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
