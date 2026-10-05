import React, { useState, useEffect, useMemo, useRef, useCallback } from 'react';
import {
  GitCompare,
  SquareTerminal,
  MessageSquare,
  Bot,
  ChevronsRight,
  CheckCircle2,
  Check,
  X,
  Plus,
  Minus,
  RotateCcw,
  FileCode,
  Search,
  Terminal as TerminalIcon,
  Sparkles,
  RefreshCw,
  ExternalLink,
  Copy,
  Trash2,
  Send,
  Loader2,
  AlertCircle,
  ChevronRight,
  ChevronDown,
  UploadCloud,
  Brain,
  Folder,
  FolderOpen,
  ChevronsDownUp,
  ChevronsUpDown,
  FoldVertical,
  PanelRight,
  Globe
} from 'lucide-react';
import { cn } from '../../lib/utils';
import { useWorkspace } from '../../stores/workspaceStore';
import { TerminalView } from '../terminal-view';
import { FileDiff } from '../../types';
import { ApiBridge } from '../../services/apiBridge';
import { parseUnifiedDiff, parseTwoFilesDiff, ParsedDiffLine } from '../diff/DiffViewer';
import {
  CreateShell,
  StopSession,
  GitStage,
  GitUnstage,
  GitDiscard,
  WriteSession,
  BrowserUseSetRect
} from '../../lib/wails';
import { MarkdownRenderer } from './MarkdownRenderer';
import { BrowserViewer } from './BrowserViewer';
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
function isGitignored(filePath: string, patterns: string[], ignoredSet: Set<string>): boolean {
  if (!filePath) return true;
  const norm = filePath.replace(/\\/g, '/').replace(/^\.\//, '');

  if (ignoredSet.has(norm) || ignoredSet.has(filePath)) return true;

  // Standard ignored directories and files
  const defaultIgnores = [
    'node_modules',
    '.git',
    'dist',
    '.forge-ade',
    '.cortex',
    '.kilo',
    '.workspace',
    '.idea',
    '.vscode',
    '.DS_Store',
    'build/bin',
    '.task',
    'archive-fe',
    '.commandcode'
  ];

  for (const def of defaultIgnores) {
    if (norm === def || norm.startsWith(def + '/') || norm.includes('/' + def + '/') || norm.endsWith('/' + def)) {
      return true;
    }
  }

  // Match .gitignore lines
  for (let pat of patterns) {
    pat = pat.trim();
    if (!pat || pat.startsWith('#')) continue;
    const isDir = pat.endsWith('/');
    const clean = isDir ? pat.slice(0, -1) : pat;

    if (clean.startsWith('*.')) {
      const ext = clean.slice(1);
      if (norm.endsWith(ext)) return true;
      continue;
    }

    if (norm === clean || norm.startsWith(clean + '/') || norm.includes('/' + clean + '/') || norm.endsWith('/' + clean)) {
      return true;
    }
  }

  return false;
}

export const AgentRightSidebar: React.FC<AgentRightSidebarProps> = ({ onOpenDiff, initialTab, openSignal = 0 }) => {
  const { 
    diffs, 
    openDiffInEditor,
    gitFiles, 
    gitBranch,
    sideFileTabs,
    activeSideFile,
    activeSideFileLine,
    openSideFile,
    closeSideFile,
    refreshGitStatus, 
    activeSession, 
    sendSideConversationPrompt, 
    clearSideConversation,
    activeWorkspacePath,
    openFileInEditor,
    currentModel,
    acceptDiff,
    rejectDiff,
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
  const [filterMode, setFilterMode] = useState<'unstaged' | 'staged' | 'all'>('all');
  const [sourceMenuOpen, setSourceMenuOpen] = useState(false);
  const [isRefreshingGit, setIsRefreshingGit] = useState(false);
  const [discardConfirmPath, setDiscardConfirmPath] = useState<string | null>(null);

  // Quick Commit State
  const [commitMessage, setCommitMessage] = useState('');
  const [isGeneratingAiCommit, setIsGeneratingAiCommit] = useState(false);
  const [isCommitting, setIsCommitting] = useState(false);
  const [isPushing, setIsPushing] = useState(false);
  const [commitFeedback, setCommitFeedback] = useState<string | null>(null);

  // Side Chat State
  const [sideInput, setSideInput] = useState('');
  const [isSendingSide, setIsSendingSide] = useState(false);
  const sideChatBottomRef = useRef<HTMLDivElement>(null);

  // Review Accordion & Grouping State
  const [expandedFileDiffs, setExpandedFileDiffs] = useState<Record<string, boolean>>({});
  const [loadedDiffs, setLoadedDiffs] = useState<Record<string, { loading: boolean; text: string }>>({});
  const [collapsedFolders, setCollapsedFolders] = useState<Record<string, boolean>>({});
  const [isGroupByFolder, setIsGroupByFolder] = useState<boolean>(true);

  // Side Chat thoughts & tool execution accordion state
  const [expandedSideThoughts, setExpandedSideThoughts] = useState<Record<string, boolean>>({});
  const [expandedSideTools, setExpandedSideTools] = useState<Record<string, boolean>>({});

  // Side-pane tabs (ZCode "+" button): nothing is mounted until opened.
  // Review is a singleton; terminals and conversations can open multiple.
  // File tabs mirror the store's sideFileTabs (opened from the left explorer).
  const [paneTabs, setPaneTabs] = useState<Array<{
    id: string;
    kind: 'review' | 'terminal' | 'chat' | 'file' | 'browser';
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

  // .gitignore patterns & ignored paths set
  const [gitignorePatterns, setGitignorePatterns] = useState<string[]>([]);
  const [gitIgnoredSet, setGitIgnoredSet] = useState<Set<string>>(new Set());

  const loadGitignore = useCallback(async () => {
    if (!activeWorkspacePath) return;
    try {
      const content = await ApiBridge.readFile(`${activeWorkspacePath}/.gitignore`);
      if (content) {
        setGitignorePatterns(content.split('\n'));
      }
    } catch {}

    try {
      const candidatePaths = diffs.map(d => d.filePath);
      if (candidatePaths.length > 0) {
        const ignored = await ApiBridge.gitCheckIgnored(candidatePaths, activeWorkspacePath);
        if (ignored && ignored.length > 0) {
          setGitIgnoredSet(prev => {
            const next = new Set(prev);
            ignored.forEach(p => next.add(p));
            return next;
          });
        }
      }
    } catch {}
  }, [activeWorkspacePath, diffs]);

  useEffect(() => {
    loadGitignore();
  }, [loadGitignore]);

  // Handle Manual Refresh
  const handleRefresh = async () => {
    setIsRefreshingGit(true);
    try {
      await refreshGitStatus();
      await loadGitignore();
    } finally {
      setIsRefreshingGit(false);
    }
  };

  // Build Master Review Files List (Respecting .gitignore)
  const { allReviewFiles, filteredReviewFiles, unstagedCount, stagedCount } = useMemo(() => {
    const list: ReviewFileItem[] = [];

    // 1. Ingest Agent session diffs (filter out gitignored files!)
    for (const d of diffs) {
      if (isGitignored(d.filePath, gitignorePatterns, gitIgnoredSet)) {
        continue;
      }
      const parts = d.filePath.split('/');
      const name = parts.pop() || d.fileName;
      const dir = parts.join('/') || 'root';

      // Check if gitFiles has a corresponding entry
      const matchGit = gitFiles.find(gf => gf.path === d.filePath || gf.path.endsWith(d.filePath) || d.filePath.endsWith(gf.path));
      const staging = matchGit?.staging || (matchGit?.status === 'staged' ? 'staged' : 'unstaged');
      const status = matchGit?.status || 'M';

      list.push({
        path: d.filePath,
        name,
        dir,
        status,
        staging,
        additions: d.additions,
        deletions: d.deletions,
        isAgentDiff: true,
        diffObj: d
      });
    }

    // 2. Ingest git files not already in list (filter out gitignored files!)
    for (const gf of gitFiles) {
      if (isGitignored(gf.path, gitignorePatterns, gitIgnoredSet)) {
        continue;
      }
      if (!list.some(item => item.path === gf.path || item.path.endsWith(gf.path) || gf.path.endsWith(item.path))) {
        const parts = gf.path.split('/');
        const name = parts.pop() || gf.path;
        const dir = gf.dir || parts.join('/') || 'root';
        const staging = gf.staging || (gf.status === 'staged' ? 'staged' : 'unstaged');

        list.push({
          path: gf.path,
          name,
          dir,
          status: gf.status || 'M',
          staging,
          additions: gf.additions || 0,
          deletions: gf.deletions || 0,
          isAgentDiff: false
        });
      }
    }

    const uCount = list.filter(f => f.staging !== 'staged').length;
    const sCount = list.filter(f => f.staging === 'staged').length;

    const filtered = list.filter(f => {
      if (filterMode === 'staged') return f.staging === 'staged';
      if (filterMode === 'unstaged') return f.staging !== 'staged';
      return true;
    });

    return {
      allReviewFiles: list,
      filteredReviewFiles: filtered,
      unstagedCount: uCount,
      stagedCount: sCount
    };
  }, [diffs, gitFiles, filterMode]);

  // Stage single file
  const handleStageFile = async (e: React.MouseEvent, path: string) => {
    e.stopPropagation();
    try {
      await GitStage(activeWorkspacePath || '', [path]);
      await refreshGitStatus();
    } catch (err) {
      console.error('Stage failed:', err);
    }
  };

  // Unstage single file
  const handleUnstageFile = async (e: React.MouseEvent, path: string) => {
    e.stopPropagation();
    try {
      await GitUnstage(activeWorkspacePath || '', [path]);
      await refreshGitStatus();
    } catch (err) {
      console.error('Unstage failed:', err);
    }
  };

  // Discard file changes
  const handleDiscardFile = async (e: React.MouseEvent, path: string) => {
    e.stopPropagation();
    try {
      await GitDiscard(activeWorkspacePath || '', [path]);
      setDiscardConfirmPath(null);
      await refreshGitStatus();
    } catch (err) {
      console.error('Discard failed:', err);
    }
  };

  // Stage All Files
  const handleStageAll = async () => {
    const paths = allReviewFiles.filter(f => f.staging !== 'staged').map(f => f.path);
    if (paths.length === 0) return;
    try {
      await GitStage(activeWorkspacePath || '', paths);
      await refreshGitStatus();
    } catch (err) {
      console.error('Stage all failed:', err);
    }
  };

  // Unstage All Files
  const handleUnstageAll = async () => {
    const paths = allReviewFiles.filter(f => f.staging === 'staged').map(f => f.path);
    if (paths.length === 0) return;
    try {
      await GitUnstage(activeWorkspacePath || '', paths);
      await refreshGitStatus();
    } catch (err) {
      console.error('Unstage all failed:', err);
    }
  };

  // Open file diff
  const handleOpenFileDiff = async (file: ReviewFileItem) => {
    if (file.diffObj) {
      if (onOpenDiff) {
        onOpenDiff(file.diffObj);
      } else {
        openDiffInEditor(file.diffObj);
      }
      return;
    }

    try {
      const isStaged = file.staging === 'staged';
      const diffText = await ApiBridge.gitDiff(file.path, activeWorkspacePath, isStaged);
      const constructedDiff: FileDiff = {
        id: `git-${file.staging}-${file.path}`,
        filePath: file.path,
        fileName: file.name,
        originalContent: '',
        modifiedContent: diffText || `(No difference detected)`,
        additions: file.additions,
        deletions: file.deletions,
        status: 'pending',
        timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
        kind: 'git'
      };

      if (onOpenDiff) {
        onOpenDiff(constructedDiff);
      } else {
        openDiffInEditor(constructedDiff);
      }
    } catch (err) {
      console.error('Failed to load git diff, opening normal file:', err);
      openFileInEditor(file.path);
    }
  };

  // Grouping by folder
  const groupedReviewFiles = useMemo(() => {
    const groups: Record<string, ReviewFileItem[]> = {};
    for (const f of filteredReviewFiles) {
      const folder = f.dir || 'root';
      if (!groups[folder]) groups[folder] = [];
      groups[folder].push(f);
    }
    return groups;
  }, [filteredReviewFiles]);

  const areAnyDiffsExpanded = useMemo(() => {
    return filteredReviewFiles.some(f => expandedFileDiffs[f.path]);
  }, [filteredReviewFiles, expandedFileDiffs]);

  const toggleFileDiff = async (file: ReviewFileItem, e?: React.MouseEvent) => {
    e?.stopPropagation();
    const nextState = !expandedFileDiffs[file.path];
    setExpandedFileDiffs(prev => ({ ...prev, [file.path]: nextState }));

    if (nextState && !loadedDiffs[file.path]) {
      if (file.diffObj?.modifiedContent) {
        setLoadedDiffs(prev => ({
          ...prev,
          [file.path]: { loading: false, text: file.diffObj!.modifiedContent }
        }));
        return;
      }

      setLoadedDiffs(prev => ({
        ...prev,
        [file.path]: { loading: true, text: '' }
      }));

      try {
        const isStaged = file.staging === 'staged';
        const text = await ApiBridge.gitDiff(file.path, activeWorkspacePath, isStaged);
        setLoadedDiffs(prev => ({
          ...prev,
          [file.path]: { loading: false, text: text || '(No difference detected)' }
        }));
      } catch (err: any) {
        setLoadedDiffs(prev => ({
          ...prev,
          [file.path]: { loading: false, text: `Error: ${err.message || 'Failed to load diff'}` }
        }));
      }
    }
  };

  const handleToggleAllDiffs = async () => {
    if (areAnyDiffsExpanded) {
      setExpandedFileDiffs({});
    } else {
      const nextExp: Record<string, boolean> = {};
      for (const f of filteredReviewFiles) {
        nextExp[f.path] = true;
      }
      setExpandedFileDiffs(nextExp);

      for (const f of filteredReviewFiles) {
        if (!loadedDiffs[f.path]) {
          if (f.diffObj?.modifiedContent) {
            setLoadedDiffs(prev => ({
              ...prev,
              [f.path]: { loading: false, text: f.diffObj!.modifiedContent }
            }));
          } else {
            ApiBridge.gitDiff(f.path, activeWorkspacePath, f.staging === 'staged')
              .then(text => {
                setLoadedDiffs(prev => ({
                  ...prev,
                  [f.path]: { loading: false, text: text || '(No difference detected)' }
                }));
              })
              .catch(err => {
                setLoadedDiffs(prev => ({
                  ...prev,
                  [f.path]: { loading: false, text: `Error: ${err.message || 'Failed to load diff'}` }
                }));
              });
          }
        }
      }
    }
  };

  const toggleFolder = (folder: string) => {
    setCollapsedFolders(prev => ({
      ...prev,
      [folder]: !prev[folder]
    }));
  };

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
  const UNMODIFIED_COLLAPSE_THRESHOLD = 6;
  const [expandedContexts, setExpandedContexts] = useState<Record<string, boolean>>({});

  const renderFileDiffView = (file: ReviewFileItem) => {
    const diffData = loadedDiffs[file.path];
    if (!diffData || diffData.loading) {
      return (
        <div className="p-4 flex items-center justify-center gap-2 text-xs text-foreground-subtle">
          <Loader2 className="w-3.5 h-3.5 animate-spin text-success" />
          <span>Loading diff for {file.name}...</span>
        </div>
      );
    }

    const raw = diffData.text;
    let parsedLines: ParsedDiffLine[] = [];

    if (file.isAgentDiff && file.diffObj && file.diffObj.originalContent !== undefined && !raw.startsWith('diff --git')) {
      parsedLines = parseTwoFilesDiff(file.diffObj.originalContent, file.diffObj.modifiedContent).lines;
    } else {
      parsedLines = parseUnifiedDiff(raw).lines;
    }

    // Drop git plumbing headers and group consecutive unmodified lines so
    // they can collapse behind a single "N unmodified lines" bar.
    const contentLines = parsedLines.filter(l => l.type !== 'file-header');
    const segments: Array<{ collapsed: boolean; count: number; lines: ParsedDiffLine[] }> = [];
    let run: ParsedDiffLine[] = [];
    const flushRun = () => {
      if (run.length === 0) return;
      segments.push({ collapsed: run.length > UNMODIFIED_COLLAPSE_THRESHOLD, count: run.length, lines: run });
      run = [];
    };
    for (const l of contentLines) {
      if (l.type === 'normal') run.push(l);
      else {
        flushRun();
        segments.push({ collapsed: false, count: 1, lines: [l] });
      }
    }
    flushRun();

    const fileExt = file.name.split('.').pop();
    const renderRows = (lines: ParsedDiffLine[], keyPrefix: string) =>
      lines.map((line, lIdx) => {
        const isAdd = line.type === 'add';
        const isDel = line.type === 'del';
        const isHdr = line.type === 'header';
        if (isHdr) {
          return (
            <div
              key={`${keyPrefix}-${lIdx}`}
              className="px-3 py-0.5 text-[10px] font-mono text-foreground-subtlest bg-surface-hover/60 select-none"
            >
              {line.text}
            </div>
          );
        }
        return (
          <div
            key={`${keyPrefix}-${lIdx}`}
            className={`flex items-start transition-colors ${
              isAdd
                ? 'bg-success/10 dark:bg-success/15'
                : isDel
                ? 'bg-destructive/10 dark:bg-destructive/15'
                : ''
            }`}
          >
            <span className="w-10 text-right pr-2 text-[10px] leading-[18px] text-foreground-subtlest select-none shrink-0">
              {line.origLine ?? ''}
            </span>
            <span className="w-10 text-right pr-2 text-[10px] leading-[18px] text-foreground-subtlest select-none shrink-0">
              {line.modLine ?? ''}
            </span>
            <span className={`flex-1 whitespace-pre pr-2 leading-[18px] ${isDel ? 'text-foreground/70' : 'text-foreground/90'}`}>
              {highlightCodeLine(line.text, fileExt)}
            </span>
          </div>
        );
      });

    return (
      <div className="border-t border-border bg-surface dark:bg-[#151516] flex flex-col">
        {/* Diff Toolbar */}
        <div className="px-3 py-1.5 bg-surface-hover border-b border-border flex items-center justify-between text-ui-xs">
          <span className="font-mono text-foreground-subtle truncate max-w-[200px]" title={file.path}>
            {file.path}
          </span>
          <div className="flex items-center gap-2 shrink-0">
            <button
              type="button"
              onClick={(e) => {
                e.stopPropagation();
                handleOpenFileDiff(file);
              }}
              className="px-2 py-0.5 rounded bg-surface-hover hover:bg-border text-foreground-subtle hover:text-foreground transition-colors cursor-pointer flex items-center gap-1 font-sans"
              title="Open full diff"
            >
              <ExternalLink className="w-3 h-3" />
              <span>Full Diff</span>
            </button>
            <button
              type="button"
              onClick={(e) => toggleFileDiff(file, e)}
              className="px-2 py-0.5 rounded bg-border hover:bg-[#D1D5DB] dark:hover:bg-[#38383C] text-foreground-subtle dark:text-[#E5E7EB] font-medium transition-colors cursor-pointer flex items-center gap-1 font-sans"
              title="Collapse this file diff"
            >
              <FoldVertical className="w-3 h-3" />
              <span>Collapse diff</span>
            </button>
          </div>
        </div>

        {/* Diff Content Viewport (ZCode style: collapsible unmodified context) */}
        <div className="max-h-72 overflow-y-auto overflow-x-auto py-1 font-mono text-ui-xs leading-[18px] select-text">
          {contentLines.length === 0 ? (
            <div className="p-3 text-center text-xs text-foreground-subtlest">
              {raw || 'No diff detected.'}
            </div>
          ) : (
            segments.map((seg, sIdx) => {
              const key = `${file.path}#${sIdx}`;
              if (seg.collapsed && !expandedContexts[key]) {
                return (
                  <button
                    key={key}
                    type="button"
                    onClick={(e) => {
                      e.stopPropagation();
                      setExpandedContexts(prev => ({ ...prev, [key]: true }));
                    }}
                    className="w-[calc(100%-16px)] mx-2 flex items-center gap-2 px-3 py-1 my-0.5 rounded-md bg-surface-hover hover:bg-border text-foreground-subtle text-[10px] font-sans cursor-pointer select-none transition-colors text-left"
                    title="Expand unmodified lines"
                  >
                    <ChevronDown className="w-3 h-3 shrink-0" />
                    <span>{seg.count} unmodified lines</span>
                  </button>
                );
              }
              return <React.Fragment key={key}>{renderRows(seg.lines, `${sIdx}`)}</React.Fragment>;
            })
          )}
        </div>

        {/* Diff Footer */}
        <div className="px-3 py-1 bg-surface dark:bg-[#18181A] border-t border-border flex items-center justify-between text-[10px] text-foreground-subtle">
          <div className="flex items-center gap-2">
            {file.additions > 0 && <span className="text-success">+{file.additions} added</span>}
            {file.deletions > 0 && <span className="text-destructive">-{file.deletions} removed</span>}
          </div>
          <button
            type="button"
            onClick={(e) => toggleFileDiff(file, e)}
            className="hover:text-foreground cursor-pointer font-medium"
          >
            Collapse diff
          </button>
        </div>
      </div>
    );
  };

  const renderFileAccordion = (file: ReviewFileItem) => {
    const isExpanded = !!expandedFileDiffs[file.path];
    const isStaged = file.staging === 'staged';
    const statusChar = file.status.toUpperCase();
    const isPendingAgentDiff = file.isAgentDiff && file.diffObj?.status === 'pending';

    return (
      <div key={file.path} className="w-full min-w-0">
        {/* Row header: flat h-8 row */}
        <div
          className="flex h-8 w-full items-center gap-3 px-3 text-left transition-colors hover:bg-surface-hover cursor-pointer group"
          onClick={(e) => toggleFileDiff(file, e)}
        >
          <div className="flex items-center gap-2 min-w-0 flex-1 pr-2">
            <span className="text-foreground-subtle shrink-0">
              {isExpanded ? <ChevronDown className="w-3.5 h-3.5" /> : <ChevronRight className="w-3.5 h-3.5" />}
            </span>
            {getFileIcon(file.name, "w-4 h-4 shrink-0", true)}
            
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-1.5 flex-wrap">
                <p className="truncate text-ui-base text-foreground">{file.name}</p>
                  <span className="truncate text-ui-base text-foreground-subtlest">{file.dir}</span>

                {/* Status Badge — single subtle tone keeps the list calm */}
                <span className="text-[10px] px-1.5 py-0.2 rounded font-medium font-mono bg-border/40 text-foreground-subtle dark:bg-border/40 dark:text-foreground-subtle">
                  {statusChar === '?' ? 'UNT' : statusChar}
                </span>

                {/* Staged Badge */}
                {isStaged && (
                  <span className="text-[9px] px-1.2 py-0.2 rounded border border-border text-foreground-subtle font-medium font-mono">
                    STAGED
                  </span>
                )}
              </div>
            </div>
          </div>

          {/* Right side: Additions/Deletions + Action Buttons */}
          <div className="flex items-center gap-2 shrink-0" onClick={e => e.stopPropagation()}>
            {(file.additions > 0 || file.deletions > 0) && (
              <div className="shrink-0 whitespace-nowrap text-ui-base">
                <span className="text-diff-added">+{file.additions}</span>
                <span className="ml-2 text-diff-removed">-{file.deletions}</span>
              </div>
            )}

            {/* Discard confirmation overlay on row */}
            {discardConfirmPath === file.path ? (
              <div className="flex items-center gap-1 bg-destructive/10 p-1 rounded-[6px]">
                <span className="text-[10px] text-destructive dark:text-destructive font-semibold px-1">Revert?</span>
                <button
                  type="button"
                  onClick={(e) => handleDiscardFile(e, file.path)}
                  className="px-1.5 py-0.5 rounded bg-[#DC2626] text-white text-[10px] font-medium cursor-pointer"
                >
                  Yes
                </button>
                <button
                  type="button"
                  onClick={(e) => { e.stopPropagation(); setDiscardConfirmPath(null); }}
                  className="px-1.5 py-0.5 rounded bg-surface-hover text-foreground-subtle text-[10px] cursor-pointer"
                >
                  No
                </button>
              </div>
            ) : (
              <div className="flex items-center gap-1 opacity-80 group-hover:opacity-100 transition-opacity">
                {/* Agent Diff Accept / Reject */}
                {isPendingAgentDiff && file.diffObj && (
                  <>
                    <button
                      type="button"
                      onClick={() => acceptDiff(file.diffObj!.id)}
                      className="p-1 rounded hover:bg-success/10 text-success transition-colors cursor-pointer"
                      title="Accept agent diff"
                    >
                      <Check className="w-3.5 h-3.5" />
                    </button>
                    <button
                      type="button"
                      onClick={() => rejectDiff(file.diffObj!.id)}
                      className="p-1 rounded hover:bg-destructive/10 text-destructive transition-colors cursor-pointer"
                      title="Reject agent diff"
                    >
                      <X className="w-3.5 h-3.5" />
                    </button>
                  </>
                )}

                {/* Stage / Unstage Button */}
                {isStaged ? (
                  <button
                    type="button"
                    onClick={(e) => handleUnstageFile(e, file.path)}
                    className="p-1 rounded hover:bg-border text-foreground-subtle hover:text-foreground transition-colors cursor-pointer"
                    title="Unstage changes"
                  >
                    <Minus className="w-3.5 h-3.5" />
                  </button>
                ) : (
                  <button
                    type="button"
                    onClick={(e) => handleStageFile(e, file.path)}
                    className="p-1 rounded hover:bg-border text-foreground-subtle hover:text-foreground transition-colors cursor-pointer"
                    title="Stage changes"
                  >
                    <Plus className="w-3.5 h-3.5" />
                  </button>
                )}

                {/* Discard button */}
                <button
                  type="button"
                  onClick={(e) => {
                    e.stopPropagation();
                    setDiscardConfirmPath(file.path);
                  }}
                  className="p-1 rounded hover:bg-destructive/10 text-foreground-subtle hover:text-destructive transition-colors cursor-pointer"
                  title="Discard changes"
                >
                  <RotateCcw className="w-3.5 h-3.5" />
                </button>

                {/* Open in full editor button */}
                <button
                  type="button"
                  onClick={(e) => {
                    e.stopPropagation();
                    handleOpenFileDiff(file);
                  }}
                  className="p-1 rounded hover:bg-border text-foreground-subtle hover:text-foreground transition-colors cursor-pointer"
                  title="Open full diff"
                >
                  <ExternalLink className="w-3.5 h-3.5" />
                </button>
              </div>
            )}
          </div>
        </div>

        {/* Accordion Body: File Diff View */}
        {isExpanded && renderFileDiffView(file)}
      </div>
    );
  };

  // AI Commit Message Generator
  const handleGenerateAiCommit = async () => {
    setIsGeneratingAiCommit(true);
    setCommitFeedback(null);
    try {
      const res = await ApiBridge.gitAiCommitMessage(activeWorkspacePath);
      if (res && res.message) {
        setCommitMessage(res.message);
      }
    } catch (err: any) {
      setCommitFeedback(`Error: ${err.message || 'Failed to generate commit'}`);
    } finally {
      setIsGeneratingAiCommit(false);
    }
  };

  // Commit Staged/All Changes
  const handleCommit = async () => {
    if (!commitMessage.trim() || isCommitting) return;
    setIsCommitting(true);
    setCommitFeedback(null);
    try {
      const res = await ApiBridge.gitCommit(commitMessage.trim(), activeWorkspacePath);
      if (res && res.success) {
        setCommitMessage('');
        setCommitFeedback('✓ Committed successfully');
        setTimeout(() => setCommitFeedback(null), 3000);
        await refreshGitStatus();
      } else {
        setCommitFeedback(`Commit output: ${res?.output || 'Failed'}`);
      }
    } catch (err: any) {
      setCommitFeedback(`Commit error: ${err.message || 'Failed'}`);
    } finally {
      setIsCommitting(false);
    }
  };

  // Push Changes
  const handlePush = async () => {
    setIsPushing(true);
    setCommitFeedback(null);
    try {
      const res = await ApiBridge.gitPush(gitBranch || 'main', activeWorkspacePath);
      if (res && res.success) {
        setCommitFeedback('✓ Pushed to remote');
        setTimeout(() => setCommitFeedback(null), 3000);
      } else {
        setCommitFeedback(`Push failed: ${res?.output || 'Error'}`);
      }
    } catch (err: any) {
      setCommitFeedback(`Push error: ${err.message || 'Failed'}`);
    } finally {
      setIsPushing(false);
    }
  };

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
      await sendSideConversationPrompt(text, currentModel);
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
    setPaneTabs(prev => (prev.some(t => t.id === 'review') ? prev : [...prev, { id: 'review', kind: 'review' as const, title: 'Review' }]));
    setActiveTab('review');
    setTabOpen(true);
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
        currentModel,
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
        }
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
                  {t.kind === 'review' && allReviewFiles.length > 0 && (
                    <span className="shrink-0 rounded-full bg-[var(--color-background-button-secondary-hover)] px-1 text-[10px] font-mono opacity-80">
                      {allReviewFiles.length}
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
              { id: 'review', label: 'Review', Icon: GitCompare, open: () => focusOrCreateReview() },
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

      {tabOpen && activeTab === 'review' && (
        <div className="flex-1 min-h-0 flex flex-col overflow-hidden">
          
          {/* Sub-toolbar: Filters + Stage All / Unstage All + Collapse Diffs + Refresh */}
          <div className="px-3.5 py-2 border-b border-border flex items-center justify-between bg-surface dark:bg-[#1A1A1C] gap-2 flex-wrap">
            
            {/* Filter Pills */}
            <div className="relative">
              <button
                type="button"
                onClick={() => setSourceMenuOpen(v => !v)}
                className="flex h-9 min-w-40 items-center justify-between gap-2 rounded-lg border border-border bg-background px-3 text-ui-sm text-foreground hover:bg-surface-hover transition-colors cursor-pointer"
              >
                <span className="truncate">
                  {filterMode === 'all' ? `All (${allReviewFiles.length})` : filterMode === 'unstaged' ? `Unstaged (${unstagedCount})` : `Staged (${stagedCount})`}
                </span>
                <ChevronDown className="size-3.5 text-foreground-subtle" />
              </button>
              {sourceMenuOpen && (
                <>
                  <div className="fixed inset-0 z-40" onClick={() => setSourceMenuOpen(false)} />
                  <div className="absolute left-0 top-full z-50 mt-1 w-44 rounded-lg border border-border bg-popover py-1 shadow-2xl text-ui-sm">
                    {[['all', `All (${allReviewFiles.length})`], ['unstaged', `Unstaged (${unstagedCount})`], ['staged', `Staged (${stagedCount})`]].map(([id, label]) => (
                      <button
                        key={id}
                        type="button"
                        onClick={() => { setFilterMode(id as typeof filterMode); setSourceMenuOpen(false); }}
                        className={`w-full cursor-pointer px-3 py-1.5 text-left transition-colors ${
                          filterMode === id ? 'bg-selected text-foreground' : 'text-foreground-subtle hover:bg-surface-hover hover:text-foreground'
                        }`}
                      >
                        {label}
                      </button>
                    ))}
                  </div>
                </>
              )}
            </div>
            {/* Quick Actions */}
            <div className="flex items-center gap-1.5">
              {/* Collapse / Expand Diffs button */}
              {filteredReviewFiles.length > 0 && (
                <button
                  type="button"
                  onClick={handleToggleAllDiffs}
                  className="px-2 py-1 rounded-[6px] bg-surface-hover hover:bg-border dark:hover:bg-surface-hover text-foreground text-ui-xs font-medium transition-colors cursor-pointer flex items-center gap-1"
                  title={areAnyDiffsExpanded ? "Collapse all diffs" : "Expand all diffs"}
                >
                  {areAnyDiffsExpanded ? (
                    <>
                      <ChevronsDownUp className="w-3.5 h-3.5 text-foreground-subtle" />
                      <span>Collapse diffs</span>
                    </>
                  ) : (
                    <>
                      <ChevronsUpDown className="w-3.5 h-3.5 text-foreground-subtle" />
                      <span>Expand diffs</span>
                    </>
                  )}
                </button>
              )}

              {/* Group by folder toggle */}
              <button
                type="button"
                onClick={() => setIsGroupByFolder(!isGroupByFolder)}
                className={`p-1 rounded-[6px] transition-colors cursor-pointer ${
                  isGroupByFolder
                    ? 'bg-border text-foreground'
                    : 'text-foreground-subtle hover:text-foreground'
                }`}
                title={isGroupByFolder ? "Folder grouping ON (click for flat list)" : "Folder grouping OFF (click to group by folder)"}
              >
                <Folder className="w-3.5 h-3.5" />
              </button>

              {unstagedCount > 0 && filterMode !== 'staged' && (
                <button
                  type="button"
                  onClick={handleStageAll}
                  className="px-2 py-1 rounded-[6px] bg-surface-hover hover:bg-border dark:hover:bg-surface-hover text-foreground text-ui-xs font-medium transition-colors cursor-pointer"
                  title="Stage all unstaged changes"
                >
                  Stage All
                </button>
              )}

              {stagedCount > 0 && filterMode !== 'unstaged' && (
                <button
                  type="button"
                  onClick={handleUnstageAll}
                  className="px-2 py-1 rounded-[6px] bg-surface-hover hover:bg-border dark:hover:bg-surface-hover text-foreground text-ui-xs font-medium transition-colors cursor-pointer"
                  title="Unstage all staged changes"
                >
                  Unstage All
                </button>
              )}

              <button
                type="button"
                onClick={handleRefresh}
                className="p-1 rounded-[6px] text-foreground-subtle hover:text-foreground hover:bg-border transition-colors cursor-pointer"
                title="Refresh Git status"
              >
                <RefreshCw className={`w-3.5 h-3.5 ${isRefreshingGit ? 'animate-spin text-success' : ''}`} />
              </button>
            </div>

          </div>

          {/* Changed Files List (with Folder Grouping & File Accordions) */}
          <div className="flex-1 min-h-0 overflow-y-auto p-3 space-y-2.5">
            {filteredReviewFiles.length === 0 ? (
              <div className="py-20 text-center text-foreground-subtlest space-y-2">
                <CheckCircle2 className="w-8 h-8 text-success mx-auto opacity-75" />
                <p className="font-medium text-xs text-foreground-subtle">No changes detected</p>
                <p className="text-ui-xs text-foreground-subtle dark:text-foreground-subtlest max-w-[220px] mx-auto">
                  {filterMode === 'staged' 
                    ? 'No files are currently staged.' 
                    : filterMode === 'unstaged'
                    ? 'No unstaged working tree changes.'
                    : 'Workspace tree matches git index cleanly.'}
                </p>
              </div>
            ) : isGroupByFolder ? (
              Object.entries(groupedReviewFiles).map(([folder, files]) => {
                const isFolderCollapsed = !!collapsedFolders[folder];
                const folderAdditions = files.reduce((acc, f) => acc + (f.additions || 0), 0);
                const folderDeletions = files.reduce((acc, f) => acc + (f.deletions || 0), 0);

                return (
                  <div key={folder} className="space-y-1.5">
                    {/* Folder Group Header */}
                    <button
                      type="button"
                      onClick={() => toggleFolder(folder)}
                      className="w-full px-2.5 py-1.5 rounded-[6px] bg-surface-hover/80 dark:bg-card/80 hover:bg-surface-hover dark:hover:bg-[#28282B] flex items-center justify-between text-left transition-colors cursor-pointer"
                    >
                      <div className="flex items-center gap-1.5 min-w-0 flex-1">
                        <span className="text-foreground-subtle">
                          {isFolderCollapsed ? <ChevronRight className="w-3.5 h-3.5" /> : <ChevronDown className="w-3.5 h-3.5" />}
                        </span>
                        {isFolderCollapsed ? (
                          <Folder className="w-3.5 h-3.5 text-foreground-subtle" />
                        ) : (
                          <FolderOpen className="w-3.5 h-3.5 text-foreground-subtle" />
                        )}
                        <span className="font-mono text-xs font-semibold text-foreground truncate">
                          {folder}
                        </span>
                        <span className="text-ui-xs text-foreground-subtle dark:text-foreground-subtlest">
                          ({files.length} {files.length === 1 ? 'file' : 'files'})
                        </span>
                      </div>

                      <div className="flex items-center gap-2 shrink-0 font-mono text-ui-xs">
                        {folderAdditions > 0 && <span className="text-success">+{folderAdditions}</span>}
                        {folderDeletions > 0 && <span className="text-destructive">-{folderDeletions}</span>}
                      </div>
                    </button>

                    {/* Files in Folder */}
                    {!isFolderCollapsed && (
                      <div className="pl-2 space-y-2 border-l-2 border-border ml-2">
                        {files.map(renderFileAccordion)}
                      </div>
                    )}
                  </div>
                );
              })
            ) : (
              filteredReviewFiles.map(renderFileAccordion)
            )}
          </div>

          {/* Quick Commit / Push Bar */}
          <div className="p-3 border-t border-border bg-background space-y-2">
            {commitFeedback && (
              <div className="text-ui-xs px-2 py-1 rounded bg-surface-hover dark:bg-[#202022] text-foreground-subtle font-mono">
                {commitFeedback}
              </div>
            )}

            <div className="flex items-center gap-1.5">
              <input
                type="text"
                value={commitMessage}
                onChange={e => setCommitMessage(e.target.value)}
                placeholder="Commit message..."
                onKeyDown={e => {
                  if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
                    handleCommit();
                  }
                }}
                className="flex-1 bg-surface border border-border rounded-[7px] px-2.5 py-1.5 text-xs text-foreground placeholder-foreground-subtlest focus:outline-hidden font-mono "
              />

              {/* AI Commit Generator */}
              <button
                type="button"
                onClick={handleGenerateAiCommit}
                disabled={isGeneratingAiCommit}
                className="p-1.5 rounded-[7px] bg-surface-hover hover:bg-border dark:hover:bg-surface-hover text-primary dark:text-info transition-colors cursor-pointer shrink-0 disabled:opacity-40"
                title="Generate commit message with AI"
              >
                {isGeneratingAiCommit ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Sparkles className="w-3.5 h-3.5" />}
              </button>

              {/* Commit Button */}
              <button
                type="button"
                onClick={handleCommit}
                disabled={!commitMessage.trim() || isCommitting}
                className="px-3 py-1.5 rounded-[7px] bg-[#16A34A] hover:bg-success text-white font-medium text-xs transition-colors cursor-pointer shrink-0 disabled:opacity-40 flex items-center gap-1"
                title="Commit staged changes"
              >
                {isCommitting ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Check className="w-3.5 h-3.5" />}
                <span>Commit</span>
              </button>

              {/* Push Button */}
              <button
                type="button"
                onClick={handlePush}
                disabled={isPushing}
                className="p-1.5 rounded-[7px] bg-surface-hover hover:bg-border dark:hover:bg-surface-hover text-foreground-subtle hover:text-foreground transition-colors cursor-pointer shrink-0 disabled:opacity-40"
                title={`Push to origin/${gitBranch || 'main'}`}
              >
                {isPushing ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <UploadCloud className="w-3.5 h-3.5" />}
              </button>
            </div>
          </div>

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
            <div className="px-3 py-2 bg-surface dark:bg-[#1A1A1C] border-b border-border flex items-center justify-between gap-2 shrink-0">
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
                          "w-10 shrink-0 select-none pr-2 text-right text-[10px] leading-[18px]",
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
                    <div className="px-3 py-2 text-[10px] text-foreground-subtlest">
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
          <div className="px-3.5 py-2.5 border-b border-border flex items-center justify-between bg-surface dark:bg-[#1A1A1C]">
            <div className="flex items-center gap-2">
              <Bot className="w-4 h-4 text-success" />
              <span className="font-semibold text-xs text-foreground">Subagent Side Chat</span>
              <span className="text-[10px] font-mono text-foreground-subtle px-1.5 py-0.2 rounded bg-border">
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
                  <p className="text-ui-xs text-foreground-subtle dark:text-foreground-subtlest max-w-[240px] mx-auto mt-0.5">
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
                                  <div key={th.id} className="rounded-[8px] bg-surface-hover dark:bg-[#202022] border border-border overflow-hidden text-xs max-w-full min-w-0">
                                    <button
                                      type="button"
                                      onClick={() => setExpandedSideThoughts(prev => ({ ...prev, [th.id]: !isExpanded }))}
                                      className="w-full px-2.5 py-1.5 flex items-center justify-between text-left hover:bg-[#EAEBED] dark:hover:bg-[#262628] transition-colors cursor-pointer"
                                    >
                                      <div className="flex items-center gap-1.5 text-foreground-subtle">
                                        <Brain className="w-3.5 h-3.5 text-foreground-subtle" />
                                        <span className="font-semibold text-ui-xs">Thought</span>
                                        <span className="text-foreground-subtlest">·</span>
                                        <span className="text-ui-xs text-foreground-subtle dark:text-foreground-subtlest">
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
                                  <div key={t.id} className="rounded-[8px] bg-surface dark:bg-[#1A1A1C] border border-border overflow-hidden text-xs max-w-full min-w-0">
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
                                        <span className={`text-[10px] font-mono font-medium px-1.5 py-0.2 rounded ${
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
                                            <span className="font-semibold text-foreground-subtle dark:text-foreground-secondary">Args:</span> {t.command}
                                          </div>
                                        )}
                                        <pre className="p-2 rounded bg-surface-hover dark:bg-[#1C1C1E] text-ui-xs font-mono text-foreground dark:text-[#E5E7EB] max-h-48 max-w-full overflow-x-auto overflow-y-auto whitespace-pre-wrap break-words [overflow-wrap:anywhere] select-text">
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
