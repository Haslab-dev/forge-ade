import React, { useState, useEffect, useMemo, useRef, useCallback } from 'react';
import { DiffViewer } from '../diff/DiffViewer';
import { FileDiff as FileDiffIcon } from 'lucide-react';
import {
  upsertSingleton,
  mirrorFileTabs,
  nextActiveAfterClose,
  nextTerminalTitle,
  hasBuiltinSideChat,
  nextConversationTitle,
  findTerminalToFocus,
  closeTabsLeft,
  closeTabsRight,
  closeTabsOthers,
  nextActiveAfterBulkClose,
  type PaneTab
} from './paneTabs';
import {
  SquareTerminal,
  MessageSquare,
  Bot,
  Check,
  X,
  Plus,
  Terminal as TerminalIcon,
  RefreshCw,
  ExternalLink,
  Copy,
  Code2,
  Trash2,
  Send,
  Loader2,
  ChevronRight,
  ChevronDown,
  Brain,
  Globe,
  Cpu,
  Presentation,
  Eye,
  BarChart3,
  ShieldAlert
} from 'lucide-react';
import { cn } from '../../lib/utils';
import { useWorkspace, isZipFile, isBinaryFile, isImageFile, MAX_SIDE_PREVIEW_TEXT_BYTES } from '../../stores/workspaceStore';
import { TerminalView } from '../terminal-view';
import { ApiBridge } from '../../services/apiBridge';
import {
  CreateShell,
  StopSession,
  WriteSession,
  BrowserUseSetRect
} from '../../lib/wails';
import { MarkdownRenderer } from './MarkdownRenderer';
import { BrowserViewer } from './BrowserViewer';
import { DeckMotionViewer } from '../editor/DeckMotionViewer';
import { HtmlRenderFrame } from './HtmlRenderFrame';
import { getFileIcon } from '../../lib/file-icons';
import { highlightCodeLine } from '../../lib/diff-highlight';
import { AgentEngine } from '../../services/agentEngine';
import { AgentMessage } from '../../types';
import { SubagentChatPane } from './SubagentChatPane';

export type SidePaneTab = 'terminal' | 'sideChat' | 'browser' | 'subagent' | 'deck';

interface AgentRightSidebarProps {
  /** Tab to activate; bump `openSignal` to open it programmatically (header buttons). */
  initialTab?: SidePaneTab | null;
  openSignal?: number;
}

// Image extensions the side-pane viewer renders natively (base64 data URIs).
const SIDE_IMAGE_EXTS = new Set(['png', 'jpg', 'jpeg', 'gif', 'webp', 'bmp', 'ico', 'svg', 'avif']);

/** Diffs-panel: file list with +/- stats and an inline diff preview. */
const ChangesPane: React.FC<{
  diffs: any[];
  gitFiles?: Array<{ path: string; status: string; staging?: 'staged' | 'unstaged' | 'untracked'; dir?: string; additions?: number; deletions?: number }>;
  onOpenInEditor: (d: any) => void;
  onRefresh?: () => Promise<void>;
}> = ({ diffs, gitFiles = [], onOpenInEditor, onRefresh }) => {
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [isRefreshing, setIsRefreshing] = useState(false);

  // Combine session diffs with git working tree changes fallback
  const effectiveDiffs = useMemo(() => {
    if (diffs && diffs.length > 0) return diffs;
    if (!gitFiles || gitFiles.length === 0) return [];
    return gitFiles.map(g => ({
      id: g.staging === 'staged' ? `git-staged-${g.path}` : `git-${g.path}`,
      filePath: g.path,
      fileName: g.path.split('/').filter(Boolean).pop() || g.path,
      status: g.status,
      additions: g.additions ?? 0,
      deletions: g.deletions ?? 0,
      isGitFile: true
    }));
  }, [diffs, gitFiles]);

  const totalAdd = effectiveDiffs.reduce((a, d) => a + (d.additions || 0), 0);
  const totalDel = effectiveDiffs.reduce((a, d) => a + (d.deletions || 0), 0);
  const selected = effectiveDiffs.find(d => d.id === selectedId) ?? effectiveDiffs[0];

  const handleRefresh = async () => {
    if (!onRefresh) return;
    setIsRefreshing(true);
    try {
      await onRefresh();
    } finally {
      setIsRefreshing(false);
    }
  };

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden" data-right-panel-surface-content>
      <div className="surface-subheader gap-2 px-3">
        <span className="px-1 text-ui-sm font-medium text-foreground">Changes</span>
        {onRefresh && (
          <button
            type="button"
            onClick={handleRefresh}
            disabled={isRefreshing}
            aria-label="Refresh changes"
            title="Refresh changes"
            className="flex size-5 items-center justify-center rounded text-muted-foreground hover:bg-accent hover:text-foreground transition-colors cursor-pointer"
          >
            <RefreshCw className={cn('size-3', isRefreshing && 'animate-spin')} />
          </button>
        )}
        <span className="ml-auto flex items-center gap-2 font-mono text-2xs tabular-nums text-muted-foreground">
          {effectiveDiffs.length} {effectiveDiffs.length === 1 ? 'file' : 'files'}
          {(totalAdd > 0 || totalDel > 0) && (
            <>
              <span className="text-diff-addition-foreground">+{totalAdd}</span>
              <span className="text-diff-deletion-foreground">-{totalDel}</span>
            </>
          )}
        </span>
      </div>
      {effectiveDiffs.length === 0 ? (
        <div className="flex flex-1 flex-col items-center justify-center gap-2 p-6 text-center text-muted-foreground">
          <FileDiffIcon className="size-8 opacity-50" />
          <p className="text-ui-sm font-medium text-foreground">No changes yet</p>
          <p className="max-w-[15rem] text-2xs leading-relaxed text-muted-foreground">
            Files modified in the workspace or edited by the agent show up here with their diffs.
          </p>
          {onRefresh && (
            <button
              type="button"
              onClick={handleRefresh}
              disabled={isRefreshing}
              className="mt-2 flex items-center gap-1.5 rounded-md border border-border px-2.5 py-1 text-2xs text-foreground hover:bg-accent transition-colors cursor-pointer"
            >
              <RefreshCw className={cn('size-3', isRefreshing && 'animate-spin')} />
              <span>Refresh changes</span>
            </button>
          )}
        </div>
      ) : (
        <div className="flex min-h-0 flex-1 flex-col overflow-y-auto">
          <div className="space-y-0.5 p-2 pb-1">
            {effectiveDiffs.map(d => {
              const fname = d.filePath.split('/').filter(Boolean).pop() || d.fileName;
              const isSel = selected?.id === d.id;
              return (
                <button
                  key={d.id}
                  type="button"
                  onClick={() => setSelectedId(d.id)}
                  onDoubleClick={() => onOpenInEditor(d)}
                  className={cn(
                    'group flex h-7 w-full items-center gap-2 rounded-md px-2 text-left transition-colors hover:bg-accent/60',
                    isSel && 'bg-accent',
                  )}
                  title={`Open ${fname} diff in the editor (double-click)`}
                >
                  <span className="shrink-0 text-muted-foreground/70">{getFileIcon(fname, 'size-3.5')}</span>
                  <span className="min-w-0 flex-1 truncate font-mono text-xs text-foreground/85 group-hover:text-foreground">
                    {fname}
                  </span>
                  {d.status ? (
                    <span className="shrink-0 rounded border border-border px-1 font-mono text-3xs text-muted-foreground">
                      {d.status === '?' ? 'UNT' : d.status}
                    </span>
                  ) : (
                    <span className="shrink-0 font-mono text-3xs tabular-nums">
                      <span className="text-diff-addition-foreground">+{d.additions || 0}</span>{' '}
                      <span className="text-diff-deletion-foreground">-{d.deletions || 0}</span>
                    </span>
                  )}
                </button>
              );
            })}
          </div>
          <div className="min-h-[16rem] flex-1 border-t border-border-subtle">
            <DiffViewer diff={selected} onClose={() => {}} isInline />
          </div>
        </div>
      )}
    </div>
  );
};

// ---------------------------------------------------------------------------
// Global sidebar state lives at module scope so tabs (especially persistent
// isolated shell sessions) survive view unmounts, task changes, and project switches.
// ---------------------------------------------------------------------------
interface GlobalSidebarState {
  paneTabs: PaneTab[];
  activeTab: string | null;
  tabOpen: boolean;
  chatThreads: Record<string, AgentMessage[]>;
  chatInputs: Record<string, string>;
  chatBusy: Record<string, boolean>;
}

let globalSidebarState: GlobalSidebarState = {
  paneTabs: [],
  activeTab: null,
  tabOpen: false,
  chatThreads: {},
  chatInputs: {},
  chatBusy: {},
};

const sidebarListeners = new Set<() => void>();
function notifySidebarListeners() {
  sidebarListeners.forEach(emit => emit());
}

export const AgentRightSidebar: React.FC<AgentRightSidebarProps> = ({ initialTab, openSignal = 0 }) => {
  const { 
    gitFiles, 
    gitBranch,
    sideFileTabs,
    activeSideFile,
    activeSideFileLine,
    openSideFile,
    closeSideFile,
    closeSideFiles,
    revalidateFileMutations,
    setMode,
    openFileInEditor,
    activeSession,
    diffs,
    openDiffInEditor,
    sendSideConversationPrompt,
    clearSideConversation,
    activeWorkspacePath,
    currentModel,
    providers,
    rightPaneWidth,
    subagents,
    sendSubagentMessage
  } = useWorkspace();

  const [tabContextMenu, setTabContextMenu] = useState<{
    x: number;
    y: number;
    tab: PaneTab;
  } | null>(null);

  // Close tab context menu on click outside or Escape
  useEffect(() => {
    const handleCloseMenu = () => setTabContextMenu(null);
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setTabContextMenu(null);
    };
    window.addEventListener('click', handleCloseMenu);
    window.addEventListener('keydown', handleKey);
    return () => {
      window.removeEventListener('click', handleCloseMenu);
      window.removeEventListener('keydown', handleKey);
    };
  }, []);

  // Active tab id, or null when the pane is empty (nothing shown by default —
  // tabs are opened explicitly via the "+" menu or the empty-state shortcuts).
  // Backed by globalSidebarState so isolated shell sessions & tabs are persistent.
  const [activeTab, setActiveTabRaw] = useState<string | null>(() => globalSidebarState.activeTab);
  const [tabOpen, setTabOpenRaw] = useState<boolean>(() => globalSidebarState.tabOpen);
  const [paneTabs, setPaneTabsRaw] = useState<PaneTab[]>(() => globalSidebarState.paneTabs);
  const [chatThreads, setChatThreadsRaw] = useState<Record<string, AgentMessage[]>>(() => globalSidebarState.chatThreads);
  const [chatInputs, setChatInputsRaw] = useState<Record<string, string>>(() => globalSidebarState.chatInputs);
  const [chatBusy, setChatBusyRaw] = useState<Record<string, boolean>>(() => globalSidebarState.chatBusy);

  const setActiveTab = useCallback((val: string | null | ((prev: string | null) => string | null)) => {
    setActiveTabRaw(prev => {
      const next = typeof val === 'function' ? val(prev) : val;
      globalSidebarState.activeTab = next;
      notifySidebarListeners();
      return next;
    });
  }, []);

  const setTabOpen = useCallback((val: boolean | ((prev: boolean) => boolean)) => {
    setTabOpenRaw(prev => {
      const next = typeof val === 'function' ? val(prev) : val;
      globalSidebarState.tabOpen = next;
      notifySidebarListeners();
      return next;
    });
  }, []);

  const setPaneTabs = useCallback((val: PaneTab[] | ((prev: PaneTab[]) => PaneTab[])) => {
    setPaneTabsRaw(prev => {
      const next = typeof val === 'function' ? val(prev) : val;
      globalSidebarState.paneTabs = next;
      notifySidebarListeners();
      return next;
    });
  }, []);

  const setChatThreads = useCallback((val: Record<string, AgentMessage[]> | ((prev: Record<string, AgentMessage[]>) => Record<string, AgentMessage[]>)) => {
    setChatThreadsRaw(prev => {
      const next = typeof val === 'function' ? val(prev) : val;
      globalSidebarState.chatThreads = next;
      notifySidebarListeners();
      return next;
    });
  }, []);

  const setChatInputs = useCallback((val: Record<string, string> | ((prev: Record<string, string>) => Record<string, string>)) => {
    setChatInputsRaw(prev => {
      const next = typeof val === 'function' ? val(prev) : val;
      globalSidebarState.chatInputs = next;
      notifySidebarListeners();
      return next;
    });
  }, []);

  const setChatBusy = useCallback((val: Record<string, boolean> | ((prev: Record<string, boolean>) => Record<string, boolean>)) => {
    setChatBusyRaw(prev => {
      const next = typeof val === 'function' ? val(prev) : val;
      globalSidebarState.chatBusy = next;
      notifySidebarListeners();
      return next;
    });
  }, []);

  useEffect(() => {
    const listener = () => {
      setActiveTabRaw(globalSidebarState.activeTab);
      setTabOpenRaw(globalSidebarState.tabOpen);
      setPaneTabsRaw(globalSidebarState.paneTabs);
      setChatThreadsRaw(globalSidebarState.chatThreads);
      setChatInputsRaw(globalSidebarState.chatInputs);
      setChatBusyRaw(globalSidebarState.chatBusy);
    };
    sidebarListeners.add(listener);
    return () => {
      sidebarListeners.delete(listener);
    };
  }, []);

  const focusOrCreateSubagent = useCallback((subagentId: string) => {
    const tabId = `subagent:${subagentId}`;
    const sub = subagents.find(s => s.id === subagentId);
    const title = sub?.title || 'Sub-agent';
    setPaneTabs(prev => upsertSingleton(prev, { id: tabId, kind: 'subagent' as const, title, sessionId: subagentId }));
    setActiveTab(tabId);
    setTabOpen(true);
  }, [subagents]);

  useEffect(() => {
    const onOpenSubagent = (e: any) => {
      const subagentId = e?.detail?.subagentId;
      if (!subagentId) return;
      focusOrCreateSubagent(subagentId);
    };
    window.addEventListener('forge:open-subagent-tab', onOpenSubagent);
    return () => window.removeEventListener('forge:open-subagent-tab', onOpenSubagent);
  }, [focusOrCreateSubagent]);

  const [deckFilePath, setDeckFilePath] = useState<string>('');
  const [deckContent, setDeckContent] = useState<string>('');
  const [deckLoading, setDeckLoading] = useState(false);
  const [sideFilePreviewMode, setSideFilePreviewMode] = useState<Record<string, boolean>>({});

  const focusOrCreateDeck = useCallback(async (customPath?: string) => {
    setPaneTabs(prev => upsertSingleton(prev, { id: 'deck', kind: 'deck' as const, title: 'Presentation & Motion' }));
    setActiveTab('deck');
    setTabOpen(true);

    const target = customPath || (activeWorkspacePath ? `${activeWorkspacePath}/deck.html` : '');
    if (target) {
      setDeckFilePath(target);
      setDeckLoading(true);
      try {
        const content = await ApiBridge.readFile(target);
        setDeckContent(content);
      } catch {
        if (!customPath && activeWorkspacePath) {
          try {
            const motionPath = `${activeWorkspacePath}/motion.html`;
            const content = await ApiBridge.readFile(motionPath);
            setDeckFilePath(motionPath);
            setDeckContent(content);
          } catch {
            setDeckContent('');
          }
        } else {
          setDeckContent('');
        }
      } finally {
        setDeckLoading(false);
      }
    }
  }, [activeWorkspacePath]);

  useEffect(() => {
    const onOpenDeck = (e: any) => {
      const path = e?.detail?.path;
      void focusOrCreateDeck(path);
    };
    window.addEventListener('forge:open-sidepane-deck', onOpenDeck);
    return () => window.removeEventListener('forge:open-sidepane-deck', onOpenDeck);
  }, [focusOrCreateDeck]);

  useEffect(() => {
    const onOpenPreview = (e: any) => {
      const title = e?.detail?.title || 'Data Visualization';
      const content = e?.detail?.content || '';
      const url = e?.detail?.url || '';
      const id = e?.detail?.id || `preview:${Date.now()}`;
      setPaneTabs(prev => upsertSingleton(prev, { id, kind: 'preview' as const, title, content, url }));
      setActiveTab(id);
      setTabOpen(true);
    };
    window.addEventListener('forge:open-preview-tab', onOpenPreview);
    return () => window.removeEventListener('forge:open-preview-tab', onOpenPreview);
  }, []);

  useEffect(() => {
    const onOpenTerminalEvent = (e: any) => {
      const cwd = e?.detail?.cwd || activeWorkspacePath || '';
      const command = e?.detail?.command || '';
      const outputFile = e?.detail?.outputFile || '';
      const taskId = e?.detail?.taskId || '';
      void openTerminalTab(false, cwd, command, outputFile, taskId);
    };
    window.addEventListener('forge:open-sidepane-terminal', onOpenTerminalEvent);
    return () => window.removeEventListener('forge:open-sidepane-terminal', onOpenTerminalEvent);
  }, [activeWorkspacePath]);

  // Header controls (terminal toggle etc.) request a tab programmatically.
  const lastSignalRef = useRef(openSignal);
  useEffect(() => {
    if (openSignal !== lastSignalRef.current) {
      lastSignalRef.current = openSignal;
      if (initialTab === 'terminal') void openTerminalTab(true);
      else if (initialTab === 'sideChat') openConversationTab(true);
      else if (initialTab === 'browser') focusOrCreateBrowser();
      else if (initialTab === 'deck') void focusOrCreateDeck();
      else if (initialTab === 'subagent' && subagents.length > 0) focusOrCreateSubagent(subagents[0].id);
    }
  }, [openSignal, initialTab, subagents, focusOrCreateSubagent, focusOrCreateDeck]);


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

  // Side-pane tabs ("+" button): nothing is mounted until opened.
  // Review is a singleton; terminals and conversations can open multiple.
  // File tabs mirror the store's sideFileTabs (opened from the left explorer).
  const paneTabsRef = useRef(paneTabs);
  useEffect(() => { paneTabsRef.current = paneTabs; }, [paneTabs]);

  // Mirror files opened from the left explorer into side-pane tabs
  // (ordered after the built-in tabs)
  useEffect(() => {
    setPaneTabs(prev => mirrorFileTabs(prev, sideFileTabs));
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
  // Guard against zip archives, binaries, and text files > 2MB.
  const [fileContents, setFileContents] = useState<Record<string, { loading: boolean; content?: string; error?: string; isImage?: boolean }>>({});
  const activeSideFilePath = tabOpen && activeTab && activeTab.startsWith('file:') ? activeTab.slice(5) : null;
  useEffect(() => {
    if (!activeSideFilePath || fileContents[activeSideFilePath]) return;
    const ext = (activeSideFilePath.split('.').pop() || '').toLowerCase();
    const isImage = isImageFile(activeSideFilePath) || SIDE_IMAGE_EXTS.has(ext);
    const fileName = activeSideFilePath.split('/').pop() || activeSideFilePath;

    // 1. Guard against zip / archive files
    if (isZipFile(activeSideFilePath)) {
      setFileContents(prev => ({
        ...prev,
        [activeSideFilePath]: {
          loading: false,
          error: `"${fileName}" is an archive file and cannot be previewed in the right sidebar.`,
          isImage: false
        }
      }));
      return;
    }

    // 2. Guard against binaries (excluding images)
    if (!isImage && isBinaryFile(activeSideFilePath)) {
      setFileContents(prev => ({
        ...prev,
        [activeSideFilePath]: {
          loading: false,
          error: `"${fileName}" is a binary file and cannot be previewed in the right sidebar.`,
          isImage: false
        }
      }));
      return;
    }

    setFileContents(prev => ({ ...prev, [activeSideFilePath]: { loading: true } }));

    const loadContent = async () => {
      if (isImage) {
        const b64 = await ApiBridge.readFileBase64(activeSideFilePath);
        return {
          content: `data:${ext === 'svg' ? 'image/svg+xml' : ext === 'jpg' ? 'image/jpeg' : `image/${ext}`};base64,${b64}`,
          isImage: true
        };
      } else {
        // 3. Guard against files > 2MB (excluding images)
        const size = await ApiBridge.fileSize(activeSideFilePath);
        if (size > MAX_SIDE_PREVIEW_TEXT_BYTES) {
          const mb = (size / (1024 * 1024)).toFixed(1);
          throw new Error(`"${fileName}" is ${mb}MB — exceeds the 2MB limit for text preview.`);
        }
        const content = await ApiBridge.readFile(activeSideFilePath);
        return { content, isImage: false };
      }
    };

    loadContent()
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

  // Diff body: long runs of unmodified lines collapse behind an
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

  // ── Side-pane tabs ("+"): nothing opens until requested ──────────────────
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

  const focusOrCreateBrowser = () => {
    setPaneTabs(prev => upsertSingleton(prev, { id: 'browser', kind: 'browser' as const, title: 'Browser' }));
    setActiveTab('browser');
    setTabOpen(true);
  };

  const openTerminalTab = async (focusExisting = false, cwd?: string, command?: string, outputFile?: string, taskId?: string) => {
    const existing = findTerminalToFocus(paneTabsRef.current, { taskId, focusExisting });
    if (existing) {
      setActiveTab(existing.id);
      setTabOpen(true);
      return;
    }
    const title = nextTerminalTitle(paneTabsRef.current, command);
    const targetCwd = cwd
      ? `isolated:${cwd}`
      : (activeWorkspacePath ? `isolated:${activeWorkspacePath}` : '');
    try {
      const created = await CreateShell(title, targetCwd);
      if (created?.id) {
        const tab = { id: `term-${created.id}`, kind: 'terminal' as const, title, sessionId: created.id, taskId };
        setPaneTabs(prev => [...prev, tab]);
        setActiveTab(tab.id);
        setTabOpen(true);
        if (outputFile) {
          setTimeout(() => {
            void WriteSession(created.id, `tail -n 100 -f "${outputFile}"\n`);
          }, 200);
        }
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
    if (!hasBuiltinSideChat(paneTabsRef.current)) {
      setPaneTabs(prev => upsertSingleton(prev, { id: 'sideChat', kind: 'chat' as const, title: 'Side chat' }));
      setActiveTab('sideChat');
      setTabOpen(true);
      return;
    }
    const tab = { id: `chat-${Date.now()}`, kind: 'chat' as const, title: nextConversationTitle(paneTabsRef.current) };
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
      const next = nextActiveAfterClose(rest, tabId, activeTab);
      setActiveTab(next.activeTab);
      setTabOpen(next.tabOpen);
    }
  };

  const closeTabsBulk = async (tabsToClose: PaneTab[], nextActiveId: string | null) => {
    if (tabsToClose.length === 0) return;
    const closeIds = new Set(tabsToClose.map(t => t.id));
    const filePathsToClose: string[] = [];

    for (const t of tabsToClose) {
      if (t.kind === 'file' && t.path) {
        filePathsToClose.push(t.path);
      } else if (t.kind === 'browser') {
        void BrowserUseSetRect(0, 0, 0, 0, false);
      } else if (t.kind === 'terminal' && t.sessionId) {
        try { await StopSession(t.sessionId); } catch {}
      } else if (t.kind === 'chat' && t.id.startsWith('chat-')) {
        setChatThreads(prev => {
          const next = { ...prev };
          delete next[t.id];
          return next;
        });
      }
    }

    const remaining = paneTabsRef.current.filter(t => !closeIds.has(t.id));
    setPaneTabs(remaining);

    if (filePathsToClose.length > 0) {
      const nextActiveFile = nextActiveId?.startsWith('file:') ? nextActiveId.slice(5) : null;
      closeSideFiles(filePathsToClose, nextActiveFile);
    }

    if (remaining.length === 0 || !nextActiveId) {
      setActiveTab(null);
      setTabOpen(false);
    } else {
      setActiveTab(nextActiveId);
      setTabOpen(true);
      if (nextActiveId.startsWith('file:')) {
        const path = nextActiveId.slice(5);
        const tInfo = sideFileTabs.find(st => st.path === path);
        openSideFile(path, tInfo?.line);
      }
    }
  };

  const handleCloseOthers = async (tabId: string) => {
    const tabsToClose = paneTabsRef.current.filter(t => t.id !== tabId);
    await closeTabsBulk(tabsToClose, tabId);
  };

  const handleCloseLeft = async (tabId: string) => {
    const current = paneTabsRef.current;
    const idx = current.findIndex(t => t.id === tabId);
    if (idx <= 0) return;
    const tabsToClose = current.slice(0, idx);
    const remaining = closeTabsLeft(current, tabId);
    const next = nextActiveAfterBulkClose(remaining, tabId, activeTab);
    await closeTabsBulk(tabsToClose, next.activeTab);
  };

  const handleCloseRight = async (tabId: string) => {
    const current = paneTabsRef.current;
    const idx = current.findIndex(t => t.id === tabId);
    if (idx < 0 || idx >= current.length - 1) return;
    const tabsToClose = current.slice(idx + 1);
    const remaining = closeTabsRight(current, tabId);
    const next = nextActiveAfterBulkClose(remaining, tabId, activeTab);
    await closeTabsBulk(tabsToClose, next.activeTab);
  };

  const handleCloseAll = async () => {
    const current = paneTabsRef.current;
    await closeTabsBulk(current, null);
  };

  // One-shot: an explicit tab request that mounted the pane opens its tab.
  const mountedOpenRef = useRef(false);
  useEffect(() => {
    if (mountedOpenRef.current) return;
    mountedOpenRef.current = true;
    if (activeSideFile) {
      setActiveTab(`file:${activeSideFile}`);
      setTabOpen(true);
      return;
    }
    if (initialTab === 'sideChat') openConversationTab(true);
    else if (initialTab === 'browser') focusOrCreateBrowser();
    else if (initialTab === 'terminal' && openSignal > 0) void openTerminalTab(true);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    const onOpenBrowser = () => {
      focusOrCreateBrowser();
    };
    window.addEventListener('forge:open-sidepane-browser', onOpenBrowser);
    return () => {
      window.removeEventListener('forge:open-sidepane-browser', onOpenBrowser);
    };
  }, []);

  useEffect(() => {
    const onOpenSidePaneTab = (e: any) => {
      const tab = e?.detail?.tab;
      if (tab === 'terminal') void openTerminalTab(true);
      else if (tab === 'sideChat') openConversationTab(true);
      else if (tab === 'browser') focusOrCreateBrowser();
      else if (tab === 'deck') void focusOrCreateDeck();
      else if (tab === 'diffs') {
        setPaneTabs(prev => upsertSingleton(prev, { id: 'diffs', kind: 'diffs' as const, title: 'Changes' }));
        setActiveTab('diffs');
        setTabOpen(true);
      } else if (tab === 'subagent' && subagents.length > 0) {
        focusOrCreateSubagent(subagents[0].id);
      }
    };
    window.addEventListener('forge:open-sidepane-tab', onOpenSidePaneTab);
    return () => window.removeEventListener('forge:open-sidepane-tab', onOpenSidePaneTab);
  }, [focusOrCreateDeck, focusOrCreateSubagent, subagents]);

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
      
      {/* Top tab bar */}
      <div
        className="flex h-[var(--workspace-topbar-height)] min-h-[var(--workspace-topbar-height)] shrink-0 items-center justify-between gap-1 border-b border-border-subtle bg-background pr-2.5 pl-2 transition-colors"
        onContextMenu={(e) => {
          e.preventDefault();
        }}
      >
        <div role="tablist" aria-label="Side pane" className="flex min-w-0 flex-1 items-center gap-1 overflow-x-auto [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">

          {/* Tabs appear only after being opened via "+" (Changes/Browser/Deck
              are singletons; terminals and conversations can open multiple). */}
          {paneTabs.map(t => {
            const isActive = tabOpen && activeTab === t.id;
            return (
              <div
                key={t.id}
                data-active-tab={isActive ? 'true' : undefined}
                className={cn(
                  'group/panetab flex h-6 max-w-36 shrink-0 cursor-pointer items-center gap-0.5 rounded-md pr-2 pl-1.5 text-xs transition-colors',
                  isActive
                    ? 'bg-accent text-foreground'
                    : 'text-muted-foreground hover:bg-accent/60 hover:text-foreground'
                )}
                onMouseDown={(e) => {
                  if (e.button === 1) {
                    e.preventDefault();
                    e.stopPropagation();
                    void closePaneTab(t.id);
                  }
                }}
                onAuxClick={(e) => {
                  if (e.button === 1) {
                    e.preventDefault();
                    e.stopPropagation();
                    void closePaneTab(t.id);
                  }
                }}
                onContextMenu={(e) => {
                  e.preventDefault();
                  e.stopPropagation();
                  setTabContextMenu({ x: e.clientX, y: e.clientY, tab: t });
                }}
              >
                <button
                  type="button"
                  role="tab"
                  aria-selected={isActive}
                  onClick={() => openTab(t.id)}
                  onMouseDown={(e) => {
                    if (e.button === 1) {
                      e.preventDefault();
                      e.stopPropagation();
                      void closePaneTab(t.id);
                    }
                  }}
                  onAuxClick={(e) => {
                    if (e.button === 1) {
                      e.preventDefault();
                      e.stopPropagation();
                      void closePaneTab(t.id);
                    }
                  }}
                  onContextMenu={(e) => {
                    e.preventDefault();
                    e.stopPropagation();
                    setTabContextMenu({ x: e.clientX, y: e.clientY, tab: t });
                  }}
                  className="flex min-w-0 cursor-pointer items-center gap-1.5 truncate"
                  title={t.title}
                >
                  {t.kind === 'terminal' ? (
                    <SquareTerminal className="size-3 shrink-0" />
                  ) : t.kind === 'browser' ? (
                    <Globe className="size-3 shrink-0" />
                  ) : t.kind === 'subagent' ? (
                    <Bot className="size-3 shrink-0" />
                  ) : t.kind === 'deck' ? (
                    <Presentation className="size-3 shrink-0" />
                  ) : t.kind === 'diffs' ? (
                    <FileDiffIcon className="size-3 shrink-0" />
                  ) : t.kind === 'preview' ? (
                    <BarChart3 className="size-3 shrink-0" />
                  ) : t.kind === 'file' ? (
                    <span className="shrink-0">{getFileIcon(t.title || t.path || '', 'size-3')}</span>
                  ) : (
                    <MessageSquare className="size-3 shrink-0" />
                  )}
                  <span className="truncate">{t.title}</span>
                </button>
                {/* Close button: surface icon swaps to X on tab hover */}
                <button
                  type="button"
                  aria-label={`Close ${t.title}`}
                  title="Close tab"
                  onClick={() => void closePaneTab(t.id)}
                  className="group/close relative flex size-4 shrink-0 cursor-pointer items-center justify-center rounded-sm hover:bg-muted"
                >
                  <span className="relative flex size-3 items-center justify-center group-hover/panetab:hidden">
                    {t.kind === 'terminal' ? (
                      <SquareTerminal className="size-3 shrink-0" />
                    ) : t.kind === 'browser' ? (
                      <Globe className="size-3 shrink-0" />
                    ) : t.kind === 'diffs' ? (
                      <FileDiffIcon className="size-3 shrink-0" />
                    ) : t.kind === 'file' ? (
                      <span className="shrink-0">{getFileIcon(t.title || t.path || '', 'size-3')}</span>
                    ) : (
                      <MessageSquare className="size-3 shrink-0" />
                    )}
                  </span>
                  <X className="hidden size-3 group-hover/panetab:block" />
                </button>
              </div>
            );
          })}

        </div>

        {/* Right side controls: "+" new-tab menu and close.
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
                      setPaneTabs(prev => upsertSingleton(prev, { id: 'diffs', kind: 'diffs' as const, title: 'Changes' }));
                      setActiveTab('diffs');
                      setTabOpen(true);
                    }}
                    className="flex w-full items-center gap-2 rounded-lg px-2.5 py-1.5 text-left text-foreground transition-colors hover:bg-surface-hover"
                  >
                    <FileDiffIcon className="size-4 text-foreground-subtle" />
                    <span className="flex-1">Changes</span>
                    {paneTabs.some(t => t.id === 'diffs') && <Check className="size-3.5 text-foreground-subtlest" />}
                  </button>
                  <button
                    type="button"
                    role="menuitem"
                    onClick={() => {
                      setNewTabMenuOpen(false);
                      void focusOrCreateDeck();
                    }}
                    className="flex w-full items-center gap-2 rounded-lg px-2.5 py-1.5 text-left text-foreground transition-colors hover:bg-surface-hover"
                  >
                    <Presentation className="size-4 text-foreground-subtle" />
                    <span className="flex-1">Presentation & Motion</span>
                    {paneTabs.some(t => t.id === 'deck') && <Check className="size-3.5 text-foreground-subtlest" />}
                  </button>
                  <button
                    type="button"
                    role="menuitem"
                    onClick={() => {
                      setNewTabMenuOpen(false);
                      const id = `preview:${Date.now()}`;
                      setPaneTabs(prev => upsertSingleton(prev, {
                        id,
                        kind: 'preview' as const,
                        title: 'Data & HTML Preview',
                        content: '<div style="display:flex;flex-direction:column;align-items:center;justify-content:center;height:80vh;text-align:center;"><h2>ForgeADE Visualization & HTML Studio</h2><p style="color:var(--muted-foreground)">Live interactive rendering for data artifacts, SVG charts, and dashboards.</p></div>'
                      }));
                      setActiveTab(id);
                      setTabOpen(true);
                    }}
                    className="flex w-full items-center gap-2 rounded-lg px-2.5 py-1.5 text-left text-foreground transition-colors hover:bg-surface-hover"
                  >
                    <BarChart3 className="size-4 text-foreground-subtle" />
                    <span className="flex-1">Data & HTML preview</span>
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
                  {subagents.length > 0 && (
                    <>
                      <div className="my-1 border-t border-border/60" />
                      <div className="px-2.5 py-1 text-[10px] font-semibold uppercase tracking-wider text-foreground-subtlest">
                        Child Agents
                      </div>
                      {subagents.map((sub) => (
                        <button
                          key={sub.id}
                          type="button"
                          role="menuitem"
                          onClick={() => {
                            setNewTabMenuOpen(false);
                            focusOrCreateSubagent(sub.id);
                          }}
                          className="flex w-full items-center gap-2 rounded-lg px-2.5 py-1.5 text-left text-foreground transition-colors hover:bg-surface-hover"
                        >
                          <Bot className="size-4 text-primary" />
                          <span className="flex-1 truncate">{sub.title || 'Sub-agent'}</span>
                          {paneTabs.some((t) => t.id === `subagent:${sub.id}`) && (
                            <Check className="size-3.5 text-foreground-subtlest" />
                          )}
                        </button>
                      ))}
                    </>
                  )}
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
      {/* TAB: DECK & MOTION (in-app presentation & motion viewer)                  */}
      {/* ========================================================================= */}
      {tabOpen && activeTab === 'deck' && (
        <div className="flex-1 min-h-0 flex flex-col overflow-hidden bg-background">
          {deckLoading ? (
            <div className="flex-1 flex items-center justify-center gap-2 text-xs text-foreground-subtle">
              <Loader2 className="w-4 h-4 animate-spin text-primary" />
              <span>Loading presentation...</span>
            </div>
          ) : deckContent ? (
            <DeckMotionViewer
              filePath={deckFilePath}
              fileName={deckFilePath.split('/').pop() || 'presentation.html'}
              htmlContent={deckContent}
            />
          ) : (
            <div className="flex-1 flex flex-col items-center justify-center gap-3 p-6 text-center text-xs text-foreground-subtle">
              <Presentation className="w-10 h-10 text-primary/40" />
              <div className="font-medium text-foreground text-sm">No Presentation Loaded</div>
              <p className="max-w-xs leading-relaxed text-foreground-muted">
                Ask your agent to create a slide deck or run <code className="px-1.5 py-0.5 rounded bg-surface font-mono">scaffold_deck</code> to generate an Open Design presentation.
              </p>
              <button
                type="button"
                onClick={() => void focusOrCreateDeck()}
                className="mt-2 px-3 py-1.5 rounded-lg bg-primary text-primary-foreground text-xs font-medium hover:opacity-90 transition-opacity cursor-pointer"
              >
                Scan Workspace for Decks
              </button>
            </div>
          )}
        </div>
      )}

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
              { id: 'terminal', label: 'Terminal', Icon: SquareTerminal, open: () => void openTerminalTab() },
              { id: 'diffs', label: 'Changes', Icon: FileDiffIcon, open: () => { setPaneTabs(prev => upsertSingleton(prev, { id: 'diffs', kind: 'diffs' as const, title: 'Changes' })); setActiveTab('diffs'); setTabOpen(true); } },
              { id: 'browser', label: 'Browser viewer', Icon: Globe, open: () => focusOrCreateBrowser() },
              { id: 'deck', label: 'Presentation & Motion', Icon: Presentation, open: () => void focusOrCreateDeck() },
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
      {/* TAB: CHANGES: session diff list + inline diff */}
      {tabOpen && activeTab === 'diffs' && (
        <ChangesPane
          diffs={activeSession?.diffs?.length ? activeSession.diffs : (diffs || [])}
          gitFiles={gitFiles}
          onRefresh={revalidateFileMutations}
          onOpenInEditor={openDiffInEditor}
        />
      )}

      {/* TAB: FILE VIEWER (files opened from the left explorer) */}
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
        const isHtmlOrSvg = ext === 'html' || ext === 'htm' || ext === 'svg';
        const isDeckOrMotion = Boolean(
          data?.content && (
            data.content.includes('deck-stage') ||
            data.content.includes('class="slide') ||
            data.content.includes('data-composition-id') ||
            fileName.endsWith('.deck.html') ||
            fileName.endsWith('.motion.html')
          )
        );
        const showDeckViewer = isDeckOrMotion && (sideFilePreviewMode[filePath] ?? true);
        const showHtmlPreview = isHtmlOrSvg && !isDeckOrMotion && (sideFilePreviewMode[filePath] ?? true);

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
                {isDeckOrMotion && (
                  <button
                    type="button"
                    onClick={() => setSideFilePreviewMode(prev => ({ ...prev, [filePath]: !(prev[filePath] ?? true) }))}
                    className={cn(
                      "flex h-6 px-2 items-center gap-1 rounded text-ui-xs font-mono transition-colors cursor-pointer",
                      showDeckViewer
                        ? "bg-primary/20 text-primary border border-primary/30"
                        : "text-foreground-subtle hover:bg-surface-hover hover:text-foreground"
                    )}
                    title="Toggle Presentation / Code view"
                  >
                    <Presentation className="w-3.5 h-3.5" />
                    <span>{showDeckViewer ? 'Slides' : 'Code'}</span>
                  </button>
                )}
                {isHtmlOrSvg && !isDeckOrMotion && (
                  <button
                    type="button"
                    onClick={() => setSideFilePreviewMode(prev => ({ ...prev, [filePath]: !(prev[filePath] ?? true) }))}
                    className={cn(
                      "flex h-6 px-2 items-center gap-1 rounded text-ui-xs font-mono transition-colors cursor-pointer",
                      showHtmlPreview
                        ? "bg-info/20 text-info border border-info/30"
                        : "text-foreground-subtle hover:bg-surface-hover hover:text-foreground"
                    )}
                    title="Toggle Visual Preview / Code view"
                  >
                    <Eye className="w-3.5 h-3.5" />
                    <span>{showHtmlPreview ? 'Preview' : 'Code'}</span>
                  </button>
                )}
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
            ) : showDeckViewer ? (
              <DeckMotionViewer
                filePath={filePath}
                fileName={fileName}
                htmlContent={data?.content || ''}
              />
            ) : showHtmlPreview ? (
              <HtmlRenderFrame
                htmlContent={data?.content || ''}
                title={fileName}
                showControls={false}
                className="border-0 rounded-none size-full"
              />
            ) : (
            <div className="flex-1 min-h-0 overflow-auto font-mono text-ui-xs leading-[18px] select-text bg-background">
              {data?.loading ? (
                <div className="p-6 flex items-center justify-center gap-2 text-xs text-foreground-subtle">
                  <Loader2 className="w-3.5 h-3.5 animate-spin text-success" />
                  <span>Loading {fileName}...</span>
                </div>
              ) : data?.error ? (
                <div className="flex-1 min-h-[300px] flex flex-col items-center justify-center p-8 text-center gap-3">
                  <div className="size-12 rounded-full bg-destructive/10 flex items-center justify-center text-destructive">
                    <ShieldAlert className="size-6" />
                  </div>
                  <div className="space-y-1">
                    <h4 className="text-ui-base font-medium text-foreground">File Cannot Be Opened</h4>
                    <p className="text-ui-xs text-muted-foreground max-w-sm leading-relaxed">{data.error}</p>
                  </div>
                  <button
                    type="button"
                    onClick={() => ApiBridge.openInFinder(filePath)}
                    className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md border border-border bg-surface text-ui-xs text-foreground hover:bg-surface-hover transition-colors cursor-pointer mt-2"
                  >
                    <ExternalLink className="size-3.5" />
                    <span>Reveal in Finder</span>
                  </button>
                </div>
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
                                        <pre className="p-2 rounded bg-surface-hover dark:bg-surface text-ui-xs font-mono text-foreground text-fg-primary max-h-48 max-w-full overflow-x-auto overflow-y-auto whitespace-pre-wrap break-words [overflow-wrap:anywhere] select-text">
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

      {/* ========================================================================= */}
      {/* TAB 5: SUBAGENT CHAT (Minimal version of Main Agent Chat with parity)     */}
      {/* ========================================================================= */}
      {tabOpen && activeTab && activeTab.startsWith('subagent:') && (() => {
        const subId = activeTab.slice(9);
        const sub = subagents.find(s => s.id === subId);
        if (!sub) {
          return (
            <div className="flex-1 flex items-center justify-center text-foreground-subtle text-ui-xs">
              Subagent session loading…
            </div>
          );
        }

        const isRunning =
          sub.status === 'running' ||
          sub.goState === 'executing' ||
          sub.goState === 'thinking';

        return (
          <SubagentChatPane
            subagent={sub}
            isRunning={isRunning}
            messages={sub.messages || []}
            workspacePath={sub.workspacePath || activeWorkspacePath || ''}
            onSendMessage={(text) => sendSubagentMessage(sub.id, text)}
          />
        );
      })()}

      {/* ========================================================================= */}
      {/* TAB 6: PREVIEW & DATA VISUALIZATION                                       */}
      {/* ========================================================================= */}
      {tabOpen && activeTab && paneTabs.some(t => t.id === activeTab && t.kind === 'preview') && (() => {
        const tab = paneTabs.find(t => t.id === activeTab);
        if (!tab) return null;
        return (
          <div className="flex-1 min-h-0 flex flex-col overflow-hidden bg-background">
            <HtmlRenderFrame
              htmlContent={tab.content}
              src={tab.url}
              title={tab.title}
              showControls={true}
              className="border-0 rounded-none size-full"
            />
          </div>
        );
      })()}

      {/* Right-click tab context menu */}
      {tabContextMenu && (() => {
        const targetTab = tabContextMenu.tab;
        const targetIdx = paneTabs.findIndex(t => t.id === targetTab.id);
        const canCloseLeft = targetIdx > 0;
        const canCloseRight = targetIdx >= 0 && targetIdx < paneTabs.length - 1;
        const canCloseOthers = paneTabs.length > 1;

        return (
          <div
            className="fixed inset-0 z-50 select-none"
            onClick={() => setTabContextMenu(null)}
            onContextMenu={(e) => {
              e.preventDefault();
              setTabContextMenu(null);
            }}
          >
            <div
              role="menu"
              aria-label="Tab actions"
              className="absolute min-w-44 rounded-lg border border-border bg-popover/95 p-1 text-ui-xs shadow-xl backdrop-blur-sm"
              style={{
                top: Math.min(tabContextMenu.y, window.innerHeight - 260),
                left: Math.min(tabContextMenu.x, window.innerWidth - 200),
              }}
              onClick={e => e.stopPropagation()}
            >
            <button
              type="button"
              role="menuitem"
              onClick={() => {
                void closePaneTab(targetTab.id);
                setTabContextMenu(null);
              }}
              className="flex w-full items-center gap-2 rounded px-2.5 py-1.5 text-left text-foreground hover:bg-surface-hover cursor-pointer"
            >
              <X className="size-3.5 text-muted-foreground" />
              <span>Close</span>
            </button>
            <button
              type="button"
              role="menuitem"
              disabled={!canCloseOthers}
              onClick={() => {
                void handleCloseOthers(targetTab.id);
                setTabContextMenu(null);
              }}
              className={cn(
                'flex w-full items-center gap-2 rounded px-2.5 py-1.5 text-left transition-colors',
                canCloseOthers
                  ? 'text-foreground hover:bg-surface-hover cursor-pointer'
                  : 'text-muted-foreground/50 cursor-not-allowed'
              )}
            >
              <span>Close Others</span>
            </button>
            <button
              type="button"
              role="menuitem"
              disabled={!canCloseLeft}
              onClick={() => {
                void handleCloseLeft(targetTab.id);
                setTabContextMenu(null);
              }}
              className={cn(
                'flex w-full items-center gap-2 rounded px-2.5 py-1.5 text-left transition-colors',
                canCloseLeft
                  ? 'text-foreground hover:bg-surface-hover cursor-pointer'
                  : 'text-muted-foreground/50 cursor-not-allowed'
              )}
            >
              <span>Close to the Left</span>
            </button>
            <button
              type="button"
              role="menuitem"
              disabled={!canCloseRight}
              onClick={() => {
                void handleCloseRight(targetTab.id);
                setTabContextMenu(null);
              }}
              className={cn(
                'flex w-full items-center gap-2 rounded px-2.5 py-1.5 text-left transition-colors',
                canCloseRight
                  ? 'text-foreground hover:bg-surface-hover cursor-pointer'
                  : 'text-muted-foreground/50 cursor-not-allowed'
              )}
            >
              <span>Close to the Right</span>
            </button>
            <div className="my-1 border-t border-border" />
            <button
              type="button"
              role="menuitem"
              onClick={() => {
                void handleCloseAll();
                setTabContextMenu(null);
              }}
              className="flex w-full items-center gap-2 rounded px-2.5 py-1.5 text-left text-foreground hover:bg-surface-hover cursor-pointer"
            >
              <span>Close All</span>
            </button>

            {targetTab.kind === 'file' && targetTab.path && (
              <>
                <div className="my-1 border-t border-border" />
                <button
                  type="button"
                  role="menuitem"
                  onClick={() => {
                    setMode('editor');
                    void openFileInEditor(targetTab.path!);
                    setTabContextMenu(null);
                  }}
                  className="flex w-full items-center gap-2 rounded px-2.5 py-1.5 text-left text-foreground hover:bg-surface-hover cursor-pointer"
                >
                  <Code2 className="size-3.5 text-muted-foreground" />
                  <span>Open in Editor</span>
                </button>
                <button
                  type="button"
                  role="menuitem"
                  onClick={() => {
                    void navigator.clipboard.writeText(targetTab.path!);
                    setTabContextMenu(null);
                  }}
                  className="flex w-full items-center gap-2 rounded px-2.5 py-1.5 text-left text-foreground hover:bg-surface-hover cursor-pointer"
                >
                  <Copy className="size-3.5 text-muted-foreground" />
                  <span>Copy Path</span>
                </button>
                <button
                  type="button"
                  role="menuitem"
                  onClick={() => {
                    const rel = activeWorkspacePath && targetTab.path!.startsWith(activeWorkspacePath)
                      ? targetTab.path!.slice(activeWorkspacePath.length).replace(/^\/+/, '')
                      : targetTab.path!;
                    void navigator.clipboard.writeText(rel);
                    setTabContextMenu(null);
                  }}
                  className="flex w-full items-center gap-2 rounded px-2.5 py-1.5 text-left text-foreground hover:bg-surface-hover cursor-pointer"
                >
                  <Copy className="size-3.5 text-muted-foreground" />
                  <span>Copy Relative Path</span>
                </button>
                <button
                  type="button"
                  role="menuitem"
                  onClick={() => {
                    void ApiBridge.openInFinder(targetTab.path!);
                    setTabContextMenu(null);
                  }}
                  className="flex w-full items-center gap-2 rounded px-2.5 py-1.5 text-left text-foreground hover:bg-surface-hover cursor-pointer"
                >
                  <ExternalLink className="size-3.5 text-muted-foreground" />
                  <span>Reveal in System</span>
                </button>
              </>
            )}
            </div>
          </div>
        );
      })()}

    </aside>
  );
};
