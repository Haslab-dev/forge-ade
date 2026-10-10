import React, { useState, useEffect, useMemo, useRef, useCallback } from 'react';
import {
  GitCompare,
  SquareTerminal,
  MessageSquare,
  Bot,
  
  
  Check,
  X,
  Plus,
  
  
  
  
  Terminal as TerminalIcon,
  
  RefreshCw,
  ExternalLink,
  
  Trash2,
  Send,
  Loader2,
  ChevronRight,
  ChevronDown,
  
  Brain,
  
  
  
  
  
  Globe,
  Cpu,
  Eye, FileCode } from 'lucide-react';
import { cn } from '../../lib/utils';
import { useWorkspace } from '../../stores/workspaceStore';
import { TerminalView } from '../terminal-view';
import { FileDiff } from '../../types';
import { ApiBridge } from '../../services/apiBridge';
import {
  CreateShell,
  StopSession,
  
  
  
  WriteSession,
  BrowserUseSetRect
} from '../../lib/wails';
import { MarkdownRenderer } from './MarkdownRenderer';
import { BrowserViewer } from './BrowserViewer';
import { GitControlPanel } from '../editor/GitControlPanel';
import { DiffViewer } from '../diff/DiffViewer';
import { getFileIcon } from '../../lib/file-icons';
import { highlightCodeLine } from '../../lib/diff-highlight';
import { AgentEngine } from '../../services/agentEngine';
import { AgentMessage } from '../../types';

export type SidePaneTab = 'review' | 'terminal' | 'sideChat' | 'browser';

interface AgentRightSidebarProps {
  onOpenDiff?: (diff: FileDiff) => void;
  /** Tab to activate; bump `openSignal` to open it programmatically (header buttons). */
  initialTab?: SidePaneTab | null;
  openSignal?: number;
}

// Image extensions the side-pane viewer renders natively (base64 data URIs).
const SIDE_IMAGE_EXTS = new Set(['png', 'jpg', 'jpeg', 'gif', 'webp', 'bmp', 'ico', 'svg', 'avif']);

export interface ReviewFileItem {
  path: string;
  name: string;
  dir: string;
  status: string; // 'M', 'A', 'D', '?', 'R', etc.
  staging: 'staged' | 'unstaged' | 'untracked';
  additions: number;
  deletions: number;
  isAgentDiff: boolean;
  diffObj?: FileDiff;
}

// Helper to determine if a file is gitignored
export const AgentRightSidebar: React.FC<AgentRightSidebarProps> = ({ initialTab, openSignal = 0 }) => {
  const { 
    diffs,
    gitFiles, 
    gitBranch,
    sideFileTabs,
    activeSideFile,
    activeSideFileLine,
    openSideFile,
    closeSideFile,
    activeSession,
    sendSideConversationPrompt,
    clearSideConversation,
    activeWorkspacePath,
    currentModel,
    providers,
    rightPaneWidth
  } = useWorkspace();

  // Active tab id, or null when the pane is empty (nothing shown by default —
  // tabs are opened explicitly via the "+" menu or the empty-state shortcuts).
  const [activeTab, setActiveTab] = useState<string | null>(null);
  const [tabOpen, setTabOpen] = useState(false);

  // Header controls (terminal toggle etc.) request a tab programmatically.
  const lastSignalRef = useRef(openSignal);
  useEffect(() => {
    if (openSignal !== lastSignalRef.current) {
      lastSignalRef.current = openSignal;
      if (initialTab === 'review') focusOrCreateReview();
      else if (initialTab === 'terminal') void openTerminalTab(true);
      else if (initialTab === 'sideChat') openConversationTab(true);
      else if (initialTab === 'browser') focusOrCreateBrowser();
    }
  }, [openSignal, initialTab]);


  // Side Chat State
  const [sideInput, setSideInput] = useState('');
  const [isSendingSide, setIsSendingSide] = useState(false);
  const sideChatBottomRef = useRef<HTMLDivElement>(null);
  // Side-chat model & posture: '' follows the main task's model.
  const [sideModel, setSideModel] = useState('');
  const [sideModelMenuOpen, setSideModelMenuOpen] = useState(false);
  const [sideMode, setSideMode] = useState<'ask' | 'plan' | 'full'>('full');
  const [sideModeMenuOpen, setSideModeMenuOpen] = useState(false);
  const sideModelMenuRef = useRef<HTMLDivElement>(null);
  const sideModeMenuRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const onPointerDown = (e: MouseEvent) => {
      if (sideModelMenuRef.current && !sideModelMenuRef.current.contains(e.target as Node)) setSideModelMenuOpen(false);
      if (sideModeMenuRef.current && !sideModeMenuRef.current.contains(e.target as Node)) setSideModeMenuOpen(false);
    };
    document.addEventListener('mousedown', onPointerDown);
    return () => document.removeEventListener('mousedown', onPointerDown);
  }, []);
  // Models offered to the side chat: every model selected in the enabled
  // providers (same source as the main composer's picker).
  const sideModelOptions = useMemo(() => {
    const list: string[] = [];
    const seen = new Set<string>();
    for (const p of providers) {
      if (!p.enabled) continue;
      const models = (p.selectedModels && p.selectedModels.length > 0 ? p.selectedModels : p.models) || [];
      for (const m of models) {
        if (!seen.has(m)) {
          seen.add(m);
          list.push(m);
        }
      }
    }
    return list;
  }, [providers]);
  const effectiveSideModel = sideModel || currentModel;


  // Side Chat thoughts & tool execution accordion state
  const [expandedSideThoughts, setExpandedSideThoughts] = useState<Record<string, boolean>>({});
  const [expandedSideTools, setExpandedSideTools] = useState<Record<string, boolean>>({});

  // Side-pane tabs (ZCode "+" button): nothing is mounted until opened.
  // Review is a singleton; terminals and conversations can open multiple.
  // File tabs mirror the store's sideFileTabs (opened from the left explorer).
  const [paneTabs, setPaneTabs] = useState<Array<{
    id: string;
    kind: 'review' | 'gitdiff' | 'terminal' | 'chat' | 'file' | 'browser';
    title: string;
    sessionId?: string;
    path?: string;
  }>>([]);
  const paneTabsRef = useRef(paneTabs);
  useEffect(() => { paneTabsRef.current = paneTabs; }, [paneTabs]);

  // Mirror files opened from the left explorer into side-pane tabs
  // (ordered after the built-in tabs, ZCode-style)
  useEffect(() => {
    setPaneTabs(prev => {
      const fileTabs = sideFileTabs.map(t => ({
        id: `file:${t.path}`,
        kind: 'file' as const,
        title: t.name,
        path: t.path
      }));
      const nonFile = prev.filter(t => t.kind !== 'file');
      if (
        prev.length === fileTabs.length + nonFile.length &&
        fileTabs.every((t, i) => prev[nonFile.length + i]?.id === t.id)
      ) {
        return prev;
      }
      return [...nonFile, ...fileTabs];
    });
  }, [sideFileTabs]);

  // When activeSideFile changes in the store, activate its tab
  useEffect(() => {
    if (activeSideFile) {
      setActiveTab(`file:${activeSideFile}`);
      setTabOpen(true);
    } else if (activeTab && activeTab.startsWith('file:')) {
      const rest = paneTabsRef.current.filter(t => t.kind !== 'file');
      if (rest.length > 0) {
        setActiveTab(rest[rest.length - 1].id);
        setTabOpen(true);
      } else {
        setActiveTab(null);
        setTabOpen(false);
      }
    }
  }, [activeSideFile]);
  const [chatThreads, setChatThreads] = useState<Record<string, AgentMessage[]>>({});
  const [chatInputs, setChatInputs] = useState<Record<string, string>>({});
  const [chatBusy, setChatBusy] = useState<Record<string, boolean>>({});
  const [newTabMenuOpen, setNewTabMenuOpen] = useState(false);
  const extraEngineRef = useRef<AgentEngine>(new AgentEngine());
  const extraChatBottomRef = useRef<HTMLDivElement>(null);

  // Auto-scroll side chat on update
  const sideMessages = activeSession?.sideConversationMessages || [];
  useEffect(() => {
    if (activeTab === 'sideChat') {
      sideChatBottomRef.current?.scrollIntoView({ behavior: 'smooth' });
    } else if (activeTab?.startsWith('chat-')) {
      extraChatBottomRef.current?.scrollIntoView({ behavior: 'smooth' });
    }
  }, [sideMessages.length, chatThreads, activeTab, tabOpen]);

  // Terminal PTY sessions are created on demand per terminal tab (see
  // openTerminalTab) — no shell is spawned until the user opens one.


  // Opened-file viewer: lazy content cache per path (read via ApiBridge).
  // Images (png/jpeg/svg/...) load as base64 data URIs and render as <img>.
  const [fileContents, setFileContents] = useState<Record<string, { loading: boolean; content?: string; error?: string; isImage?: boolean }>>({});
  const activeSideFilePath = tabOpen && activeTab && activeTab.startsWith('file:') ? activeTab.slice(5) : null;
  useEffect(() => {
    if (!activeSideFilePath || fileContents[activeSideFilePath]) return;
    const ext = (activeSideFilePath.split('.').pop() || '').toLowerCase();
    const isImage = SIDE_IMAGE_EXTS.has(ext);
    setFileContents(prev => ({ ...prev, [activeSideFilePath]: { loading: true } }));
    const load = isImage
      ? ApiBridge.readFileBase64(activeSideFilePath).then(b64 => ({
          content: `data:${ext === 'svg' ? 'image/svg+xml' : ext === 'jpg' ? 'image/jpeg' : `image/${ext}`};base64,${b64}`,
          isImage: true
        }))
      : ApiBridge.readFile(activeSideFilePath).then(content => ({ content, isImage: false }));
    load
      .then(({ content, isImage: img }) => {
        setFileContents(prev => ({ ...prev, [activeSideFilePath]: { loading: false, content, isImage: img } }));
      })
      .catch((err: any) => {
        setFileContents(prev => ({
          ...prev,
          [activeSideFilePath]: { loading: false, error: String(err?.message || err) }
        }));
      });
  }, [activeSideFilePath, fileContents]);

  const highlightedLineRef = useRef<HTMLDivElement | null>(null);
  useEffect(() => {
    if (activeSideFileLine && highlightedLineRef.current) {
      highlightedLineRef.current.scrollIntoView({ behavior: 'smooth', block: 'center' });
    }
  }, [activeSideFilePath, activeSideFileLine, fileContents]);

  // ZCode-style diff body: long runs of unmodified lines collapse behind an
  // expandable "N unmodified lines" bar (default collapsed).

  // Side Chat Handlers
  // Which conversation is active: 'main' = built-in side chat, otherwise an
  // extra conversation tab id. One JSX block serves both.
  const activeChatId: string | null =
    tabOpen && activeTab && (activeTab === 'sideChat' || activeTab.startsWith('chat-'))
      ? activeTab === 'sideChat' ? 'main' : activeTab
      : null;
  const activeMessages: AgentMessage[] = activeChatId === 'main'
    ? sideMessages
    : (chatThreads[activeChatId ?? ''] || []);
  const activeChatInput = activeChatId === 'main'
    ? sideInput
    : (chatInputs[activeChatId ?? ''] || '');

  const handleSendSide = async (e?: React.FormEvent, customText?: string) => {
    if (e) e.preventDefault();
    const text = (customText || sideInput).trim();
    if (!text || isSendingSide) return;
    setSideInput('');
    setIsSendingSide(true);
    try {
      await sendSideConversationPrompt(text, sideModel || undefined, sideMode);
    } finally {
      setIsSendingSide(false);
    }
  };

  // ── Side-pane tabs (ZCode "+"): nothing opens until requested ─────────────
  // Review is a singleton; terminals and conversations open new instances.

  const openTab = (tab: string) => {
    setActiveTab(tab);
    setTabOpen(true);
    if (tab.startsWith('file:')) {
      const filePath = tab.slice(5);
      const tabInfo = sideFileTabs.find(t => t.path === filePath);
      openSideFile(filePath, tabInfo?.line);
    }
  };

  const handleReloadFile = useCallback((filePath: string) => {
    const ext = (filePath.split('.').pop() || '').toLowerCase();
    const isImage = SIDE_IMAGE_EXTS.has(ext);
    setFileContents(prev => ({ ...prev, [filePath]: { loading: true } }));
    const load = isImage
      ? ApiBridge.readFileBase64(filePath).then(b64 => ({
          content: `data:${ext === 'svg' ? 'image/svg+xml' : ext === 'jpg' ? 'image/jpeg' : `image/${ext}`};base64,${b64}`,
          isImage: true
        }))
      : ApiBridge.readFile(filePath).then(content => ({ content, isImage: false }));
    load
      .then(({ content, isImage: img }) => {
        setFileContents(prev => ({ ...prev, [filePath]: { loading: false, content, isImage: img } }));
      })
      .catch((err: any) => {
        setFileContents(prev => ({
          ...prev,
          [filePath]: { loading: false, error: String(err?.message || err) }
        }));
      });
  }, []);

  const focusOrCreateReview = () => {
    setPaneTabs(prev => (prev.some(t => t.id === 'review') ? prev : [...prev, { id: 'review', kind: 'review' as const, title: 'Git' }]));
    setActiveTab('review');
    setTabOpen(true);
  };

  // Full-diff pane tab: git-panel diffs render INSIDE the side pane instead of
  // spawning tabs in the main editor.
  const [gitDiff, setGitDiff] = useState<FileDiff | null>(null);
  const focusOrCreateGitDiff = (diff: FileDiff) => {
    setGitDiff(diff);
    setPaneTabs(prev => (prev.some(t => t.id === 'gitdiff') ? prev : [...prev, { id: 'gitdiff', kind: 'gitdiff' as const, title: 'Diff' }]));
    setActiveTab('gitdiff');
    setTabOpen(true);
  };
  const closeGitDiff = () => {
    setGitDiff(null);
    void closePaneTab('gitdiff');
  };

  // Browser viewer is a singleton tab hosting the in-app browser.
  const focusOrCreateBrowser = () => {
    setPaneTabs(prev => (prev.some(t => t.id === 'browser') ? prev : [...prev, { id: 'browser', kind: 'browser' as const, title: 'Browser' }]));
    setActiveTab('browser');
    setTabOpen(true);
  };

  const openTerminalTab = async (focusExisting = false) => {
    if (focusExisting) {
      const existing = paneTabsRef.current.find(t => t.kind === 'terminal');
      if (existing) {
        setActiveTab(existing.id);
        setTabOpen(true);
        return;
      }
    }
    const title = `zsh ${paneTabsRef.current.filter(t => t.kind === 'terminal').length + 1}`;
    try {
      const created = await CreateShell(title, activeWorkspacePath || '');
      if (created?.id) {
        const tab = { id: `term-${created.id}`, kind: 'terminal' as const, title, sessionId: created.id };
        setPaneTabs(prev => [...prev, tab]);
        setActiveTab(tab.id);
        setTabOpen(true);
      }
    } catch (err) {
      console.error('Failed to create side terminal:', err);
    }
  };

  const openConversationTab = (focusExisting = false) => {
    if (focusExisting) {
      const existing = paneTabsRef.current.find(t => t.kind === 'chat');
      if (existing) {
        setActiveTab(existing.id);
        setTabOpen(true);
        return;
      }
    }
    // The first conversation binds to the session's built-in side chat;
    // additional ones are independent threads.
    if (!paneTabsRef.current.some(t => t.id === 'sideChat')) {
      setPaneTabs(prev => (prev.some(t => t.id === 'sideChat') ? prev : [...prev, { id: 'sideChat', kind: 'chat' as const, title: 'Side chat' }]));
      setActiveTab('sideChat');
      setTabOpen(true);
      return;
    }
    const tab = { id: `chat-${Date.now()}`, kind: 'chat' as const, title: `Conversation ${paneTabsRef.current.filter(t => t.kind === 'chat').length + 1}` };
    setPaneTabs(prev => [...prev, tab]);
    setChatThreads(prev => ({ ...prev, [tab.id]: [] }));
    setActiveTab(tab.id);
    setTabOpen(true);
  };

  const closePaneTab = async (tabId: string) => {
    const tab = paneTabsRef.current.find(t => t.id === tabId);
    if (tab?.kind === 'file' && tab.path) {
      closeSideFile(tab.path);
      return;
    }
    if (tab?.kind === 'browser') {
      // Hide the native browser view immediately — don't rely solely on the
      // viewer's unmount cleanup.
      void BrowserUseSetRect(0, 0, 0, 0, false);
    }
    if (tab?.kind === 'terminal' && tab.sessionId) {
      try { await StopSession(tab.sessionId); } catch {}
    }
    if (tab?.kind === 'chat' && tabId.startsWith('chat-')) {
      setChatThreads(prev => {
        const next = { ...prev };
        delete next[tabId];
        return next;
      });
    }
    setPaneTabs(prev => prev.filter(t => t.id !== tabId));
    if (activeTab === tabId) {
      const rest = paneTabsRef.current.filter(t => t.id !== tabId);
      if (rest.length > 0) {
        setActiveTab(rest[rest.length - 1].id);
        setTabOpen(true);
      } else {
        setActiveTab(null);
        setTabOpen(false);
      }
    }
  };

  // One-shot: an explicit tab request that mounted the pane opens its tab.
  const mountedOpenRef = useRef(false);
  useEffect(() => {
    if (mountedOpenRef.current) return;
    mountedOpenRef.current = true;
    if (initialTab === 'review') focusOrCreateReview();
    else if (initialTab === 'terminal') void openTerminalTab(true);
    else if (initialTab === 'sideChat') openConversationTab(true);
    else if (initialTab === 'browser') focusOrCreateBrowser();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Extra-conversation send: streams into the tab's local thread via its own
  // AgentEngine instance — independent from the session's built-in side chat.
  const handleSendExtraChat = async (tabId: string, customText?: string) => {
    const text = (customText || chatInputs[tabId] || '').trim();
    if (!text || chatBusy[tabId]) return;
    setChatInputs(prev => ({ ...prev, [tabId]: '' }));
    setChatBusy(prev => ({ ...prev, [tabId]: true }));

    const userMsg: AgentMessage = { id: `u-${Date.now()}`, role: 'user', content: text, timestamp: new Date().toISOString() };
    const agentMsgId = `a-${Date.now()}`;
    const agentMsg: AgentMessage = { id: agentMsgId, role: 'agent', content: '', timestamp: new Date().toISOString(), isThinking: true };
    setChatThreads(prev => ({ ...prev, [tabId]: [...(prev[tabId] || []), userMsg, agentMsg] }));

    const patch = (fn: (m: AgentMessage) => AgentMessage) => {
      setChatThreads(prev => ({
        ...prev,
        [tabId]: (prev[tabId] || []).map(m => (m.id === agentMsgId ? fn(m) : m))
      }));
    };

    try {
      const history = [
        ...(chatThreads[tabId] || []).map(m => ({ role: m.role === 'user' ? 'user' : 'assistant', content: m.content })),
        { role: 'user', content: text }
      ];
      await extraEngineRef.current.streamSideChat(
        sideModel || currentModel,
        history,
        {
          workspacePath: activeWorkspacePath || '',
          activeTaskTitle: activeSession?.title,
          diffs: activeSession?.diffs || [],
          gitFiles,
          gitBranch
        },
        {
          onChunk: chunk => {
            setChatThreads(prev => ({
              ...prev,
              [tabId]: (prev[tabId] || []).map(m => {
                if (m.id !== agentMsgId) return m;
                return { ...m, content: m.content + chunk, isThinking: false };
              })
            }));
          },
          onThought: thought => {
            patch(m => {
              const thoughts = m.thoughts || [];
              const idx = thoughts.findIndex(t => t.id === thought.id);
              const nextThoughts = idx >= 0
                ? thoughts.map((t, i) => (i === idx ? { ...t, ...thought } : t))
                : [...thoughts, thought];
              return { ...m, thoughts: nextThoughts, isThinking: true };
            });
          },
          onToolStart: tool => {
            patch(m => {
              const tools = m.toolExecutions || [];
              const idx = tools.findIndex(t => t.id === tool.id);
              const nextTools = idx >= 0
                ? tools.map((t, i) => (i === idx ? { ...t, ...tool } : t))
                : [...tools, tool];
              return { ...m, toolExecutions: nextTools, isThinking: false };
            });
          },
          onToolComplete: tool => {
            patch(m => {
              const tools = m.toolExecutions || [];
              const idx = tools.findIndex(t => t.id === tool.id);
              const nextTools = idx >= 0
                ? tools.map((t, i) => (i === idx ? { ...t, ...tool } : t))
                : [...tools, tool];
              return { ...m, toolExecutions: nextTools };
            });
          },
          onToolStatus: status => {
            patch(m => ({ ...m, toolStatus: status || undefined, isThinking: !!status }));
          }
        },
        undefined,
        sideMode
      );
    } catch (err: any) {
      patch(m => ({
        ...m,
        content: `⚠️ **Error**: ${err?.message || String(err)}`,
        isThinking: false
      }));
    } finally {
      setChatBusy(prev => ({ ...prev, [tabId]: false }));
    }
  };

  // Extra terminal tabs: clear / restart operate on that tab's own session.
  const handleClearExtraTerminal = async (sessionId?: string) => {
    if (sessionId) {
      try { await WriteSession(sessionId, '\x0c'); } catch {}
    }
  };

  const handleRestartExtraTerminal = async (tabId: string, sessionId?: string) => {
    const tab = paneTabsRef.current.find(t => t.id === tabId);
    if (!tab) return;
    try {
      if (sessionId) { try { await StopSession(sessionId); } catch {} }
      const created = await CreateShell(tab.title, activeWorkspacePath || '');
      if (created?.id) {
        setPaneTabs(prev => prev.map(t => (t.id === tabId ? { ...t, sessionId: created.id, id: `term-${created.id}` } : t)));
        setActiveTab(`term-${created.id}`);
      }
    } catch (err) {
      console.error('Failed to restart side terminal:', err);
    }
  };

  return (
    <aside
      style={{ width: `${rightPaneWidth}px` }}
      className="h-full min-w-0 shrink-0 border-l border-border bg-background flex flex-col select-none text-xs text-foreground overflow-hidden transition-colors"
    >
      
      {/* Top Tabs Bar (Kybern 46px surface chips) */}
      <div className="h-[46px] min-h-[46px] shrink-0 px-2.5 border-b border-border/50 flex items-center justify-between bg-background gap-1.5 transition-colors">
        <div role="tablist" aria-label="Side pane" className="flex items-center gap-1 overflow-x-auto min-w-0 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">

          {/* Tabs appear only after being opened via "+" (Review = singleton,
              terminals and conversations can open multiple). */}
          {paneTabs.map(t => {
            const isActive = tabOpen && activeTab === t.id;
            return (
              <div
                key={t.id}
                className={cn(
                  'group/panetab flex h-7 shrink-0 items-center gap-1 rounded-lg pl-2 pr-1 text-[length:var(--app-font-size-ui,12px)] transition-colors',
                  isActive
                    ? 'bg-[var(--color-background-button-secondary)] text-foreground font-medium'
                    : 'text-muted-foreground hover:bg-[var(--color-background-button-secondary-hover)] hover:text-foreground'
                )}
              >
                <button
                  type="button"
                  role="tab"
                  aria-selected={isActive}
                  onClick={() => openTab(t.id)}
                  className="flex max-w-44 cursor-pointer items-center gap-1.5 truncate"
                  title={t.title}
                >
                  {t.kind === 'review' ? (
                    <GitCompare className="size-3.5 shrink-0 text-success opacity-70" />
                  ) : t.kind === 'gitdiff' ? (
                    <FileCode className="size-3.5 shrink-0 text-info opacity-70" />
                  ) : t.kind === 'terminal' ? (
                    <SquareTerminal className="size-3.5 shrink-0 opacity-70" />
                  ) : t.kind === 'browser' ? (
                    <Globe className="size-3.5 shrink-0 opacity-70" />
                  ) : t.kind === 'file' ? (
                    <span className="shrink-0">{getFileIcon(t.title || t.path || '', 'size-3.5')}</span>
                  ) : (
                    <MessageSquare className="size-3.5 shrink-0 opacity-70" />
                  )}
                  <span className="truncate">{t.title}</span>
                  {t.kind === 'review' && gitFiles.length > 0 && (
                    <span className="shrink-0 rounded-full bg-[var(--color-background-button-secondary-hover)] px-1 text-ui-xs font-mono opacity-80">
                      {gitFiles.length}
                    </span>
                  )}
                </button>
                <button
                  type="button"
                  aria-label={`Close ${t.title}`}
                  title="Close tab"
                  onClick={() => void closePaneTab(t.id)}
                  className="flex size-4 shrink-0 items-center justify-center rounded text-muted-foreground opacity-0 transition-opacity hover:text-foreground group-hover/panetab:opacity-100 focus-visible:opacity-100 cursor-pointer"
                >
                  <X className="size-3" />
                </button>
              </div>
            );
          })}

        </div>

        {/* Right side controls: "+" new-tab menu (ZCode side-pane +) and close.
            Lives OUTSIDE the scrollable tablist — an overflow-x-auto ancestor
            clips the dropdown menu. */}
        <div className="flex shrink-0 items-center gap-0.5">
          <div className="relative">
            <button
              type="button"
              onClick={() => setNewTabMenuOpen(v => !v)}
              aria-haspopup="menu"
              aria-expanded={newTabMenuOpen}
              aria-label="New side conversation or terminal"
              title="New conversation or terminal"
              className="flex size-6 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-[var(--color-background-button-secondary-hover)] hover:text-foreground cursor-pointer"
            >
              <Plus className="size-4" />
            </button>
            {newTabMenuOpen && (
              <>
                <div className="fixed inset-0 z-40" onClick={() => setNewTabMenuOpen(false)} />
                <div role="menu" aria-label="New tab" className="absolute right-0 top-full z-50 mt-1.5 w-52 rounded-xl border border-border bg-popover p-1 text-ui-sm shadow-lg">
                  <button
                    type="button"
                    role="menuitem"
                    onClick={() => {
                      setNewTabMenuOpen(false);
                      focusOrCreateReview();
                    }}
                    className="flex w-full items-center gap-2 rounded-lg px-2.5 py-1.5 text-left text-foreground transition-colors hover:bg-surface-hover"
                  >
                    <GitCompare className="size-4 text-foreground-subtle" />
                    <span className="flex-1">Review</span>
                    {paneTabs.some(t => t.id === 'review') && <Check className="size-3.5 text-foreground-subtlest" />}
                  </button>
                  <button
                    type="button"
                    role="menuitem"
                    onClick={() => {
                      setNewTabMenuOpen(false);
                      void openTerminalTab();
                    }}
                    className="flex w-full items-center gap-2 rounded-lg px-2.5 py-1.5 text-left text-foreground transition-colors hover:bg-surface-hover"
                  >
                    <SquareTerminal className="size-4 text-foreground-subtle" />
                    <span className="flex-1">New terminal</span>
                  </button>
                  <button
                    type="button"
                    role="menuitem"
                    onClick={() => {
                      setNewTabMenuOpen(false);
                      focusOrCreateBrowser();
                    }}
                    className="flex w-full items-center gap-2 rounded-lg px-2.5 py-1.5 text-left text-foreground transition-colors hover:bg-surface-hover"
                  >
                    <Globe className="size-4 text-foreground-subtle" />
                    <span className="flex-1">Browser viewer</span>
                    {paneTabs.some(t => t.id === 'browser') && <Check className="size-3.5 text-foreground-subtlest" />}
                  </button>
                  <button
                    type="button"
                    role="menuitem"
                    onClick={() => {
                      setNewTabMenuOpen(false);
                      openConversationTab();
                    }}
                    className="flex w-full items-center gap-2 rounded-lg px-2.5 py-1.5 text-left text-foreground transition-colors hover:bg-surface-hover"
                  >
                    <MessageSquare className="size-4 text-foreground-subtle" />
                    <span className="flex-1">New conversation</span>
                  </button>
                </div>
              </>
            )}
          </div>
        </div>
      </div>

      {/* ========================================================================= */}
      {/* TAB: BROWSER (in-app browser viewer)                                      */}
      {/* ========================================================================= */}
      {tabOpen && activeTab === 'browser' && <BrowserViewer />}

      {/* ========================================================================= */}
      {/* TAB 1: REVIEW (Git & Session Diff Review with Complete Action Controls)   */}
      {/* ========================================================================= */}
      {/* Empty state: choose a tab (reference side pane) */}
      {!tabOpen && (
        <div className="flex flex-1 flex-col items-center justify-center gap-5 px-8">
          <div className="text-center">
            <h2 className="text-ui-xl font-semibold text-foreground">Open tab</h2>
            <p className="mt-1 text-ui-sm text-foreground-subtle">Choose a tab to open in the side pane.</p>
          </div>
          <div className="flex w-full max-w-xs flex-col gap-2.5">
            {([
              { id: 'sideChat', label: 'Side conversation', Icon: MessageSquare, open: () => openConversationTab() },
              { id: 'review', label: 'Git', Icon: GitCompare, open: () => focusOrCreateReview() },
              { id: 'terminal', label: 'Terminal', Icon: SquareTerminal, open: () => void openTerminalTab() },
              { id: 'browser', label: 'Browser viewer', Icon: Globe, open: () => focusOrCreateBrowser() },
            ] as const).map(({ id, label, Icon, open }) => (
              <button
                key={id}
                type="button"
                onClick={open}
                className="flex h-12 w-full items-center gap-3 rounded-xl border border-border px-4 text-left text-ui-base text-foreground hover:bg-surface-hover transition-colors cursor-pointer"
              >
                <Icon className="size-4 text-foreground-subtle" />
                <span>{label}</span>
              </button>
            ))}
          </div>
        </div>
      )}

      {tabOpen && activeTab === 'review' && <GitControlPanel onOpenFullDiff={focusOrCreateGitDiff} />}

      {/* TAB: GIT DIFF (full diff rendered inside the side pane) */}
      {tabOpen && activeTab === 'gitdiff' && gitDiff && (
        <div className="flex-1 min-h-0 flex flex-col overflow-hidden">
          <DiffViewer diff={gitDiff} isInline onClose={closeGitDiff} />
        </div>
      )}

      {/* ========================================================================= */}
      {/* TAB: FILE VIEWER (files opened from the left explorer, ZCode-style)       */}
      {/* ========================================================================= */}
      {tabOpen && activeSideFilePath && (() => {
        const filePath = activeSideFilePath;
        const data = fileContents[filePath];
        const fileName = filePath.split('/').pop() || filePath;
        const relPath = activeWorkspacePath && filePath.startsWith(activeWorkspacePath + '/')
          ? filePath.slice(activeWorkspacePath.length + 1)
          : filePath;
        const projectLeaf = (activeWorkspacePath || '').split('/').filter(Boolean).pop() || 'Project';
        const ext = fileName.includes('.') ? fileName.split('.').pop() : '';
        const MAX_LINES = 5000;
        const lines = data?.content ? data.content.split('\n') : [];
        const visibleLines = lines.slice(0, MAX_LINES);
        return (
          <div className="flex-1 min-h-0 flex flex-col overflow-hidden">
            {/* Breadcrumb header */}
            <div className="px-3 py-2 bg-surface bg-panel border-b border-border flex items-center justify-between gap-2 shrink-0">
              <div className="flex min-w-0 items-center gap-1.5 text-ui-xs font-mono text-foreground-subtle">
                <span className="shrink-0">{projectLeaf}</span>
                <span className="text-foreground-subtlest">›</span>
                <span className="truncate text-foreground" title={filePath}>{relPath}</span>
              </div>
              <div className="flex items-center gap-1 shrink-0">
                <button
                  type="button"
                  onClick={() => handleReloadFile(filePath)}
                  className="flex size-6 shrink-0 items-center justify-center rounded-md text-foreground-subtle hover:bg-surface-hover hover:text-foreground transition-colors cursor-pointer"
                  title="Reload file from disk"
                  aria-label="Reload file"
                >
                  <RefreshCw className={cn("w-3.5 h-3.5", data?.loading && "animate-spin text-info")} />
                </button>
                <button
                  type="button"
                  onClick={() => ApiBridge.openInFinder(filePath)}
                  className="flex size-6 shrink-0 items-center justify-center rounded-md text-foreground-subtle hover:bg-surface-hover hover:text-foreground transition-colors cursor-pointer"
                  title="Reveal in Finder"
                  aria-label="Reveal in Finder"
                >
                  <ExternalLink className="w-3.5 h-3.5" />
                </button>
              </div>
            </div>

            {/* Content */}
            {data?.isImage ? (
              <div
                className="flex-1 min-h-0 overflow-auto flex items-center justify-center p-6 bg-surface"
                style={{
                  backgroundImage:
                    'repeating-conic-gradient(rgba(255,255,255,0.05) 0% 25%, transparent 0% 50%)',
                  backgroundSize: '16px 16px'
                }}
              >
                <img
                  src={data.content}
                  alt={fileName}
                  draggable={false}
                  className="max-w-full max-h-full object-contain shadow-lg"
                />
              </div>
            ) : (
            <div className="flex-1 min-h-0 overflow-auto font-mono text-ui-xs leading-[18px] select-text bg-background">
              {data?.loading ? (
                <div className="p-6 flex items-center justify-center gap-2 text-xs text-foreground-subtle">
                  <Loader2 className="w-3.5 h-3.5 animate-spin text-success" />
                  <span>Loading {fileName}...</span>
                </div>
              ) : data?.error ? (
                <div className="p-6 text-center text-xs text-destructive">{data.error}</div>
              ) : lines.length === 0 ? (
                <div className="p-6 text-center text-xs text-foreground-subtlest">Empty file</div>
              ) : (
                <>
                  {visibleLines.map((line, i) => {
                    const lineNum = i + 1;
                    const isTargetLine = activeSideFileLine === lineNum;
                    return (
                      <div
                        key={i}
                        id={`side-file-line-${lineNum}`}
                        ref={isTargetLine ? highlightedLineRef : undefined}
                        className={cn(
                          "flex items-start hover:bg-surface-hover/40 transition-colors",
                          isTargetLine && "bg-info/15 border-l-2 border-info"
                        )}
                      >
                        <span className={cn(
                          "w-10 shrink-0 select-none pr-2 text-right text-ui-xs leading-[18px]",
                          isTargetLine ? "text-info font-bold" : "text-foreground-subtlest"
                        )}>
                          {lineNum}
                        </span>
                        <span className={cn(
                          "flex-1 whitespace-pre pr-3 leading-[18px]",
                          isTargetLine ? "text-foreground font-medium" : "text-foreground/90"
                        )}>
                          {highlightCodeLine(line, ext)}
                        </span>
                      </div>
                    );
                  })}
                  {lines.length > MAX_LINES && (
                    <div className="px-3 py-2 text-ui-xs text-foreground-subtlest">
                      File truncated — showing first {MAX_LINES.toLocaleString()} of {lines.length.toLocaleString()} lines
                    </div>
                  )}
                </>
              )}
            </div>
            )}
          </div>
        );
      })()}

      {/* Terminal tabs — one persistent shell per tab, opened via "+"
          (the built-in default shell was removed: terminals open on demand). */}
      {paneTabs.filter(t => t.kind === 'terminal').map(t => (
        <div key={t.id} className={tabOpen && activeTab === t.id ? 'flex-1 min-h-0 flex flex-col overflow-hidden bg-card' : 'hidden'}>
          <div className="px-3.5 py-2 bg-surface dark:bg-background border-b border-border flex items-center justify-between text-xs text-foreground-subtle shrink-0">
            <div className="flex items-center gap-2">
              <span className="font-medium text-foreground">{t.title}</span>
              <span className="font-mono text-ui-xs text-foreground-subtle px-1.5 py-0.5 rounded-[4px] bg-surface-hover">
                zsh -l
              </span>
            </div>
            <div className="flex items-center gap-1.5">
              <button
                type="button"
                onClick={() => void handleClearExtraTerminal(t.sessionId)}
                className="px-2 py-0.5 rounded-[5px] bg-surface-hover hover:bg-border dark:hover:bg-surface-hover text-foreground-subtle hover:text-foreground text-ui-xs transition-colors cursor-pointer"
                title="Clear terminal screen"
              >
                Clear
              </button>
              <button
                type="button"
                onClick={() => void handleRestartExtraTerminal(t.id, t.sessionId)}
                className="px-2 py-0.5 rounded-[5px] bg-surface-hover hover:bg-border dark:hover:bg-surface-hover text-foreground-subtle hover:text-foreground text-ui-xs transition-colors cursor-pointer flex items-center gap-1"
                title="Restart terminal shell"
              >
                <RefreshCw className="w-3 h-3" />
                <span>Restart</span>
              </button>
            </div>
          </div>
          <div className="flex-1 min-h-0 overflow-hidden relative">
            {t.sessionId ? (
              <TerminalView isActive={activeTab === t.id} sessionId={t.sessionId} />
            ) : (
              <div className="h-full flex items-center justify-center text-xs text-foreground-subtle">
                Starting shell…
              </div>
            )}
          </div>
        </div>
      ))}

      {/* ========================================================================= */}
      {/* TAB 4: SIDE CHAT (Context-Aware Assistant with Markdown & Auto-scroll)    */}
      {/* Serves the built-in side chat and every extra conversation tab.           */}
      {/* ========================================================================= */}
      {activeChatId !== null && (
        <div className="flex-1 min-h-0 flex flex-col overflow-hidden">
          
          {/* Header */}
          <div className="px-3.5 py-2.5 border-b border-border flex items-center justify-between bg-surface bg-panel">
            <div className="flex items-center gap-2">
              <Bot className="w-4 h-4 text-success" />
              <span className="font-semibold text-xs text-foreground">Subagent Side Chat</span>
              <span className="text-ui-xs font-mono text-foreground-subtle px-1.5 py-0.2 rounded bg-border">
                {currentModel || 'active'}
              </span>
            </div>

            {activeMessages.length > 0 && (
              <button
                type="button"
                onClick={() => {
                  if (activeChatId === 'main') {
                    clearSideConversation();
                  } else {
                    setChatThreads(prev => ({ ...prev, [activeChatId]: [] }));
                  }
                }}
                className="text-foreground-subtle hover:text-destructive transition-colors cursor-pointer p-1 rounded shrink-0"
                title="Clear side chat"
              >
                <Trash2 className="w-3.5 h-3.5" />
              </button>
            )}
          </div>

          {/* Messages Area */}
          <div className="flex-1 min-h-0 overflow-y-auto overflow-x-hidden p-4 space-y-3.5 select-text w-full min-w-0">
            {activeMessages.length === 0 ? (
              <div className="py-12 text-center text-foreground-subtlest space-y-3">
                <MessageSquare className="w-8 h-8 text-foreground-subtlest mx-auto opacity-70" />
                <div>
                  <p className="text-xs font-semibold text-foreground-subtle">Side Assistant</p>
                  <p className="text-ui-xs text-foreground-subtlest max-w-[240px] mx-auto mt-0.5">
                    Ask questions, verify changes, or explore alternative designs without polluting the primary task stream.
                  </p>
                </div>

                {/* Quick Prompts */}
                <div className="flex flex-col gap-1.5 pt-2 max-w-[280px] mx-auto text-left">
                  {[
                    'Explain all changed files in this session',
                    'Check for potential edge cases or bugs',
                    'How can I test the modified code?'
                  ].map((prompt, i) => (
                    <button
                      key={i}
                      type="button"
                      onClick={() => {
                        if (activeChatId === 'main') void handleSendSide(undefined, prompt);
                        else void handleSendExtraChat(activeChatId, prompt);
                      }}
                      className="p-2 rounded-[7px] bg-card hover:bg-surface-hover dark:hover:bg-surface-hover border border-border text-ui-xs text-foreground-subtle hover:text-foreground transition-colors text-left cursor-pointer"
                    >
                      {prompt}
                    </button>
                  ))}
                </div>
              </div>
            ) : (
              activeMessages.map((m, i) => {
                const isUser = m.role === 'user';
                return (
                  <div key={m.id || i} className={`flex flex-col ${isUser ? 'items-end' : 'items-start'} w-full max-w-full min-w-0`}>
                    <div className={`max-w-[95%] w-full rounded-[10px] p-3 text-xs leading-relaxed min-w-0 ${
                      isUser 
                        ? 'bg-selected text-foreground border border-border' 
                        : 'bg-card text-foreground border border-border shadow-2xs'
                    }`}>
                      {isUser ? (
                        <p className="whitespace-pre-wrap break-words [overflow-wrap:anywhere]">{m.content}</p>
                      ) : (
                        <>
                          {/* 1. Thoughts */}
                          {m.thoughts && m.thoughts.length > 0 && (
                            <div className="space-y-1.5 mb-2.5 w-full max-w-full min-w-0">
                              {m.thoughts.map(th => {
                                const isExpanded = expandedSideThoughts[th.id] ?? false;
                                return (
                                  <div key={th.id} className="rounded-[8px] bg-surface-hover bg-surface border border-border overflow-hidden text-xs max-w-full min-w-0">
                                    <button
                                      type="button"
                                      onClick={() => setExpandedSideThoughts(prev => ({ ...prev, [th.id]: !isExpanded }))}
                                      className="w-full px-2.5 py-1.5 flex items-center justify-between text-left hover:bg-surface-hover transition-colors cursor-pointer"
                                    >
                                      <div className="flex items-center gap-1.5 text-foreground-subtle">
                                        <Brain className="w-3.5 h-3.5 text-foreground-subtle" />
                                        <span className="font-semibold text-ui-xs">Thought</span>
                                        <span className="text-foreground-subtlest">·</span>
                                        <span className="text-ui-xs text-foreground-subtlest">
                                          {th.durationSeconds ? `${th.durationSeconds}s` : 'a few seconds'}
                                        </span>
                                      </div>
                                      {isExpanded ? <ChevronDown className="w-3.5 h-3.5 text-foreground-subtle" /> : <ChevronRight className="w-3.5 h-3.5 text-foreground-subtle" />}
                                    </button>
                                    {isExpanded && (
                                      <div className="p-2.5 border-t border-border max-h-52 overflow-y-auto overflow-x-hidden font-mono text-ui-xs text-foreground-subtle whitespace-pre-wrap break-words [overflow-wrap:anywhere] select-text leading-relaxed">
                                        {th.thoughtText}
                                      </div>
                                    )}
                                  </div>
                                );
                              })}
                            </div>
                          )}

                          {/* 2. Tool Executions */}
                          {m.toolExecutions && m.toolExecutions.length > 0 && (
                            <div className="space-y-1.5 mb-2.5 w-full max-w-full min-w-0">
                              {m.toolExecutions.map(t => {
                                const isExpanded = expandedSideTools[t.id] ?? false;
                                const isRunning = t.status === 'running';
                                const isFailed = t.status === 'failed';
                                return (
                                  <div key={t.id} className="rounded-[8px] bg-surface bg-panel border border-border overflow-hidden text-xs max-w-full min-w-0">
                                    <button
                                      type="button"
                                      onClick={() => setExpandedSideTools(prev => ({ ...prev, [t.id]: !isExpanded }))}
                                      className="w-full px-2.5 py-1.5 flex items-center justify-between text-left hover:bg-surface-hover dark:hover:bg-[#222225] transition-colors cursor-pointer"
                                    >
                                      <div className="flex items-center gap-2 min-w-0 flex-1 pr-2">
                                        {isRunning ? (
                                          <Loader2 className="w-3.5 h-3.5 animate-spin text-warning shrink-0" />
                                        ) : isFailed ? (
                                          <X className="w-3.5 h-3.5 text-destructive shrink-0" />
                                        ) : (
                                          <Check className="w-3.5 h-3.5 text-success shrink-0" />
                                        )}
                                        <TerminalIcon className="w-3.5 h-3.5 text-foreground-subtle shrink-0" />
                                        <span className="font-mono text-ui-xs font-semibold text-foreground truncate">
                                          {t.toolName || 'Tool Execution'}
                                        </span>
                                      </div>
                                      <div className="flex items-center gap-1.5 shrink-0 text-foreground-subtle">
                                        <span className={`text-ui-xs font-mono font-medium px-1.5 py-0.2 rounded ${
                                          isRunning ? 'bg-warning/10 text-warning dark:bg-warning/10 dark:text-warning' :
                                          isFailed ? 'bg-destructive/10 text-destructive dark:bg-destructive/10 dark:text-destructive' :
                                          'bg-success/10 text-success dark:bg-success/10 dark:text-success'
                                        }`}>
                                          {t.status}
                                        </span>
                                        {isExpanded ? <ChevronDown className="w-3.5 h-3.5" /> : <ChevronRight className="w-3.5 h-3.5" />}
                                      </div>
                                    </button>
                                    {isExpanded && (
                                      <div className="p-2 border-t border-border bg-card space-y-1.5 max-w-full min-w-0">
                                        {t.command && (
                                          <div className="text-ui-xs font-mono text-foreground-subtle truncate">
                                            <span className="font-semibold text-foreground-subtle">Args:</span> {t.command}
                                          </div>
                                        )}
                                        <pre className="p-2 rounded bg-surface-hover dark:bg-[#1C1C1E] text-ui-xs font-mono text-foreground text-fg-primary max-h-48 max-w-full overflow-x-auto overflow-y-auto whitespace-pre-wrap break-words [overflow-wrap:anywhere] select-text">
                                          {t.output || '(No output)'}
                                        </pre>
                                      </div>
                                    )}
                                  </div>
                                );
                              })}
                            </div>
                          )}

                          {/* 3. Assistant Content */}
                          {m.content && <MarkdownRenderer content={m.content} />}

                          {/* 4. Live Thinking status */}
                          {m.isThinking && (
                            <div className="flex items-center gap-2 text-warning py-1 mt-1 font-mono text-ui-xs">
                              <Loader2 className="w-3.5 h-3.5 animate-spin" />
                              <span className="font-medium">{m.toolStatus || 'Side assistant is working...'}</span>
                            </div>
                          )}
                        </>
                      )}
                    </div>
                  </div>
                );
              })
            )}
            {activeChatId === 'main' ? (
              <div ref={sideChatBottomRef} />
            ) : (
              <div ref={extraChatBottomRef} />
            )}
          </div>

          {/* Side chat pickers: model + posture (ask / plan / full access) */}
          <div className="shrink-0 flex items-center gap-1.5 border-t border-border bg-background px-3 pt-2">
            {/* Model picker */}
            <div className="relative" ref={sideModelMenuRef}>
              <button
                type="button"
                onClick={() => { setSideModelMenuOpen(v => !v); setSideModeMenuOpen(false); }}
                aria-haspopup="menu"
                aria-expanded={sideModelMenuOpen}
                className="flex h-6 max-w-[190px] items-center gap-1 rounded-md border border-border bg-surface px-1.5 text-ui-xs text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground cursor-pointer"
                title="Side chat model"
              >
                <Cpu className="w-3 h-3 shrink-0 text-primary" />
                <span className="truncate font-mono">{effectiveSideModel || 'Model'}</span>
                <ChevronDown className="w-2.5 h-2.5 shrink-0 opacity-60" />
              </button>
              {sideModelMenuOpen && (
                <div role="menu" aria-label="Side chat models" className="absolute bottom-full left-0 z-50 mb-1.5 max-h-64 w-64 overflow-y-auto rounded-xl border border-border bg-popover p-1 shadow-xl">
                  <div className="px-2.5 py-1 text-ui-xs font-semibold uppercase tracking-wider text-foreground-subtlest">
                    Side chat model
                  </div>
                  {sideModelOptions.length === 0 && (
                    <p className="px-2.5 py-2 text-ui-xs text-foreground-subtle">Configure models in Settings → Model settings</p>
                  )}
                  {sideModelOptions.map(m => (
                    <button
                      key={m}
                      type="button"
                      role="menuitemradio"
                      aria-checked={m === effectiveSideModel}
                      onClick={() => { setSideModel(m); setSideModelMenuOpen(false); }}
                      className="flex w-full items-center justify-between rounded-lg px-2.5 py-1.5 text-left transition-colors hover:bg-surface-hover cursor-pointer"
                    >
                      <span className="truncate font-mono text-ui-xs text-foreground">{m}</span>
                      {m === effectiveSideModel && <Check className="w-3.5 h-3.5 shrink-0 text-success" />}
                    </button>
                  ))}
                </div>
              )}
            </div>

            {/* Mode picker: ask / plan / full access */}
            <div className="relative" ref={sideModeMenuRef}>
              <button
                type="button"
                onClick={() => { setSideModeMenuOpen(v => !v); setSideModelMenuOpen(false); }}
                aria-haspopup="menu"
                aria-expanded={sideModeMenuOpen}
                className="flex h-6 items-center gap-1 rounded-md border border-border bg-surface px-1.5 text-ui-xs text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground cursor-pointer"
                title="Side chat mode"
              >
                {sideMode === 'ask' ? (
                  <MessageSquare className="w-3 h-3 shrink-0 text-foreground-subtle" />
                ) : sideMode === 'plan' ? (
                  <Brain className="w-3 h-3 shrink-0 text-info" />
                ) : (
                  <Eye className="w-3 h-3 shrink-0 text-success" />
                )}
                <span className="capitalize">{sideMode === 'full' ? 'Full access' : sideMode}</span>
                <ChevronDown className="w-2.5 h-2.5 shrink-0 opacity-60" />
              </button>
              {sideModeMenuOpen && (
                <div role="menu" aria-label="Side chat mode" className="absolute bottom-full left-0 z-50 mb-1.5 w-52 rounded-xl border border-border bg-popover p-1 shadow-xl">
                  {([
                    { id: 'ask', label: 'Ask', desc: 'Q&A about the code and changes' },
                    { id: 'plan', label: 'Plan', desc: 'Implementation plans, no rewrites' },
                    { id: 'full', label: 'Full access', desc: 'Active pair programmer' }
                  ] as const).map(opt => (
                    <button
                      key={opt.id}
                      type="button"
                      role="menuitemradio"
                      aria-checked={sideMode === opt.id}
                      onClick={() => { setSideMode(opt.id); setSideModeMenuOpen(false); }}
                      className="flex w-full items-start justify-between gap-2 rounded-lg px-2.5 py-1.5 text-left transition-colors hover:bg-surface-hover cursor-pointer"
                    >
                      <span className="min-w-0">
                        <span className="block text-ui-xs font-medium text-foreground">{opt.label}</span>
                        <span className="block text-ui-xs text-foreground-subtlest">{opt.desc}</span>
                      </span>
                      {sideMode === opt.id && <Check className="mt-0.5 w-3.5 h-3.5 shrink-0 text-success" />}
                    </button>
                  ))}
                </div>
              )}
            </div>
          </div>

          {/* Sub Chat Input */}
          <form
            onSubmit={e => {
              e.preventDefault();
              if (activeChatId === 'main') {
                void handleSendSide();
              } else {
                void handleSendExtraChat(activeChatId);
              }
            }}
            className="shrink-0 p-3 border-t border-border bg-background flex items-center gap-2"
          >
            <input
              type="text"
              value={activeChatInput}
              onChange={e => {
                const v = e.target.value;
                if (activeChatId === 'main') setSideInput(v);
                else setChatInputs(prev => ({ ...prev, [activeChatId]: v }));
              }}
              placeholder="Ask side question or review guidance..."
              className="flex-1 bg-surface border border-border rounded-[8px] px-3 py-2 text-xs text-foreground placeholder-foreground-subtlest focus:outline-hidden"
            />
            <button
              type="submit"
              disabled={(!activeChatInput.trim()) || (activeChatId === 'main' ? isSendingSide : !!chatBusy[activeChatId])}
              className="w-8 h-8 rounded-[8px] bg-surface-hover hover:bg-border text-foreground flex items-center justify-center disabled:opacity-40 transition-colors cursor-pointer shrink-0"
            >
              {(activeChatId === 'main' ? isSendingSide : !!chatBusy[activeChatId]) ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Send className="w-3.5 h-3.5" />}
            </button>
          </form>

        </div>
      )}

    </aside>
  );
};
