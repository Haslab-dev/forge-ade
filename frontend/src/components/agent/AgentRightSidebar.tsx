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
  FoldVertical
} from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { TerminalView } from '../terminal-view';
import { FileDiff } from '../../types';
import { ApiBridge } from '../../services/apiBridge';
import { parseUnifiedDiff, parseTwoFilesDiff, ParsedDiffLine } from '../diff/DiffViewer';
import { 
  CreateShell, 
  ListSessions, 
  StopSession, 
  GitStage, 
  GitUnstage, 
  GitDiscard,
  WriteSession
} from '../../lib/wails';
import { MarkdownRenderer } from './MarkdownRenderer';

interface AgentRightSidebarProps {
  onClose?: () => void;
  onOpenDiff?: (diff: FileDiff) => void;
}

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

export const AgentRightSidebar: React.FC<AgentRightSidebarProps> = ({ onClose, onOpenDiff }) => {
  const { 
    diffs, 
    openDiffInEditor,
    gitFiles, 
    gitBranch,
    refreshGitStatus, 
    activeSession, 
    sendSideConversationPrompt, 
    clearSideConversation,
    activeWorkspacePath,
    openFileInEditor,
    currentModel,
    acceptDiff,
    rejectDiff
  } = useWorkspace();

  const [activeTab, setActiveTab] = useState<'findings' | 'review' | 'terminal' | 'sideChat'>('review');
  // Reference side pane starts empty: tabs open into the pane, clicking the
  // active tab closes it back to the 'Open tab' empty state.
  const [tabOpen, setTabOpen] = useState(false);
  const openTab = (tab: 'findings' | 'review' | 'terminal' | 'sideChat') => {
    if (tabOpen && activeTab === tab) {
      setTabOpen(false);
    } else {
      setActiveTab(tab);
      setTabOpen(true);
    }
  };
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

  // PTY Shell Session State
  const [terminalSessionId, setTerminalSessionId] = useState<string | null>(null);
  const [isInitializingTerminal, setIsInitializingTerminal] = useState(false);

  // Expanded tool states in Findings tab
  const [expandedCommands, setExpandedCommands] = useState<Record<string, boolean>>({});

  // Review Accordion & Grouping State
  const [expandedFileDiffs, setExpandedFileDiffs] = useState<Record<string, boolean>>({});
  const [loadedDiffs, setLoadedDiffs] = useState<Record<string, { loading: boolean; text: string }>>({});
  const [collapsedFolders, setCollapsedFolders] = useState<Record<string, boolean>>({});
  const [isGroupByFolder, setIsGroupByFolder] = useState<boolean>(true);

  // Side Chat thoughts & tool execution accordion state
  const [expandedSideThoughts, setExpandedSideThoughts] = useState<Record<string, boolean>>({});
  const [expandedSideTools, setExpandedSideTools] = useState<Record<string, boolean>>({});

  // Auto-scroll side chat on update
  const sideMessages = activeSession?.sideConversationMessages || [];
  useEffect(() => {
    if (activeTab === 'sideChat') {
      sideChatBottomRef.current?.scrollIntoView({ behavior: 'smooth' });
    }
  }, [sideMessages.length, activeTab]);

  // Terminal PTY Initialization
  useEffect(() => {
    let isCancelled = false;
    const initTerminal = async () => {
      setIsInitializingTerminal(true);
      try {
        const existing = await ListSessions();
        const found = (existing || []).find((s: any) => s.name === 'Agent Shell' || s.name === 'Agent Terminal');
        if (found && !isCancelled) {
          setTerminalSessionId(found.id);
          setIsInitializingTerminal(false);
          return;
        }
        const created = await CreateShell('Agent Shell', activeWorkspacePath || '');
        if (created && created.id && !isCancelled) {
          setTerminalSessionId(created.id);
        }
      } catch (err) {
        console.error('Failed to initialize Agent terminal session:', err);
      } finally {
        if (!isCancelled) setIsInitializingTerminal(false);
      }
    };

    initTerminal();
    return () => {
      isCancelled = true;
    };
  }, [activeWorkspacePath]);

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
          additions: 0,
          deletions: 0,
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

        {/* Diff Content Viewport */}
        <div className="max-h-72 overflow-y-auto overflow-x-auto p-1 font-mono text-ui-xs leading-[18px] select-text">
          {parsedLines.length === 0 ? (
            <div className="p-3 text-center text-xs text-foreground-subtlest">
              {raw || 'No diff detected.'}
            </div>
          ) : (
            parsedLines.map((line, lIdx) => {
              const isAdd = line.type === 'add';
              const isDel = line.type === 'del';
              const isHdr = line.type === 'header' || line.type === 'file-header';

              return (
                <div
                  key={lIdx}
                  className={`flex items-start px-1.5 py-0.5 transition-colors ${
                    isAdd
                      ? 'bg-success/10/70 dark:bg-success/10/40 text-success dark:text-success'
                      : isDel
                      ? 'bg-destructive/10/70 dark:bg-destructive/10/40 text-destructive dark:text-destructive'
                      : isHdr
                      ? 'bg-primary/10/40 dark:bg-primary/10/30 text-primary dark:text-info font-medium'
                      : 'text-foreground-subtle dark:text-foreground-secondary'
                  }`}
                >
                  <span className="w-8 text-right pr-2 text-foreground-subtle dark:text-[#555558] select-none shrink-0 text-[10px]">
                    {line.origLine ?? (isAdd ? '+' : '')}
                  </span>
                  <span className="w-8 text-right pr-2 text-foreground-subtle dark:text-[#555558] select-none shrink-0 text-[10px]">
                    {line.modLine ?? (isDel ? '-' : '')}
                  </span>
                  <span className="w-4 text-center select-none font-medium shrink-0">
                    {isAdd ? '+' : isDel ? '-' : ' '}
                  </span>
                  <span className="flex-1 whitespace-pre pr-2">
                    {line.text}
                  </span>
                </div>
              );
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
            <FileCode className="w-4 h-4 text-success shrink-0" />
            
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-1.5 flex-wrap">
                <p className="truncate text-ui-base text-foreground">{file.name}</p>
                  <span className="truncate text-ui-base text-foreground-subtlest">{file.dir}</span>

                {/* Status Badge */}
                <span className={`text-[10px] px-1.5 py-0.2 rounded font-medium font-mono ${
                  statusChar === 'A' || statusChar === '?' 
                    ? 'bg-success/10 text-success dark:bg-success/10 dark:text-success'
                    : statusChar === 'D'
                    ? 'bg-destructive/10 text-destructive dark:bg-destructive/10 dark:text-destructive'
                    : 'bg-warning/10 text-warning dark:bg-warning/10 dark:text-warning'
                }`}>
                  {statusChar === '?' ? 'UNT' : statusChar}
                </span>

                {/* Staged Badge */}
                {isStaged && (
                  <span className="text-[9px] px-1.2 py-0.2 rounded bg-primary/10 text-info dark:bg-primary/10 dark:text-info font-medium font-mono ">
                    STAGED
                  </span>
                )}

                {/* Agent Badge */}
                {file.isAgentDiff && (
                  <span className="text-[9px] px-1.2 py-0.2 rounded bg-primary/10 text-primary dark:bg-primary/10 dark:text-primary font-medium flex items-center gap-0.5">
                    <Sparkles className="w-2.5 h-2.5" />
                    <span>Agent</span>
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

  // Dynamic Findings Extraction
  const findingsData = useMemo(() => {
    const msgs = activeSession?.messages || [];
    const agentMsgs = msgs.filter(m => m.role === 'agent');
    const userMsgs = msgs.filter(m => m.role === 'user');

    const initialUserPrompt = userMsgs[0]?.content || activeSession?.title || 'Inspect workspace';

    // Tool executions aggregated
    const allTools = agentMsgs.flatMap(m => m.toolExecutions || []);
    
    // Inspected files (Explore)
    const inspectedFiles = new Set<string>();
    allTools.forEach(t => {
      if (t.readFiles) t.readFiles.forEach(f => inspectedFiles.add(f));
      try {
        const parsed = JSON.parse(t.command || '{}');
        const p = parsed.path || parsed.filePath || parsed.targetFile;
        if (p) inspectedFiles.add(p);
      } catch {}
    });

    // Executed commands (Terminal)
    const terminalCommands = allTools.filter(t => {
      const name = t.toolName.toLowerCase();
      return name.startsWith('bash') || name.startsWith('command') || name.startsWith('run_command') || name.startsWith('terminal');
    });

    // Latest Agent response summary
    const lastAgentMsg = agentMsgs[agentMsgs.length - 1];
    const latestResponse = lastAgentMsg?.content || '';

    // Status label
    const isRunning = activeSession?.status === 'running';

    return {
      initialUserPrompt,
      turnCount: agentMsgs.length,
      inspectedFiles: Array.from(inspectedFiles),
      terminalCommands,
      modifiedFiles: diffs,
      latestResponse,
      isRunning
    };
  }, [activeSession, diffs]);

  // Restart Shell Session
  const handleRestartShell = async () => {
    setIsInitializingTerminal(true);
    try {
      if (terminalSessionId) {
        try { await StopSession(terminalSessionId); } catch {}
      }
      const created = await CreateShell('Agent Shell', activeWorkspacePath || '');
      if (created?.id) setTerminalSessionId(created.id);
    } catch (err) {
      console.error('Failed to restart shell:', err);
    } finally {
      setIsInitializingTerminal(false);
    }
  };

  // Clear Terminal
  const handleClearTerminal = async () => {
    if (terminalSessionId) {
      try {
        await WriteSession(terminalSessionId, '\x0c');
      } catch {}
    }
  };

  return (
    <aside className="w-[540px] min-w-[440px] max-w-[660px] h-full bg-background border-l border-border flex flex-col select-none text-xs text-foreground transition-colors">
      
      {/* Top Tabs Bar */}
      <div className="h-[52px] min-h-[52px] px-3 border-b border-border flex items-center justify-between bg-background gap-1.5 transition-colors">
        <div className="flex items-center gap-1.5 overflow-x-auto">
          
          {/* Collapse Icon Button */}
          {onClose && (
            <button
              type="button"
              onClick={onClose}
              className="w-7 h-7 rounded-[6px] border border-border flex items-center justify-center text-foreground-subtle hover:text-foreground hover:bg-surface-hover transition-colors cursor-pointer shrink-0"
              title="Collapse panel"
            >
              <ChevronsRight className="w-3.5 h-3.5" />
            </button>
          )}

          {/* Tab: Review */}
          <button
            type="button"
            onClick={() => openTab('review')}
            className={`flex items-center gap-1.5 px-2.5 py-1.2 rounded-[7px] border transition-colors cursor-pointer text-[12px] shrink-0 ${
              tabOpen && activeTab === 'review'
                ? 'bg-selected border-border text-foreground font-semibold shadow-2xs'
                : 'border-transparent text-foreground-subtle hover:text-foreground hover:bg-surface-hover'
            }`}
          >
            <GitCompare className="w-3.5 h-3.5 text-success" />
            <span>Review</span>
            {allReviewFiles.length > 0 && (
              <span className="text-[10px] px-1.5 py-0.2 rounded-full bg-surface-hover border border-border text-foreground-subtle font-mono ">
                {allReviewFiles.length}
              </span>
            )}
          </button>

          {/* Tab: Findings */}
          <button
            type="button"
            onClick={() => openTab('findings')}
            className={`flex items-center gap-1.5 px-2.5 py-1.2 rounded-[7px] border transition-colors cursor-pointer text-[12px] shrink-0 ${
              tabOpen && activeTab === 'findings'
                ? 'bg-selected border-border text-foreground font-semibold shadow-2xs'
                : 'border-transparent text-foreground-subtle hover:text-foreground hover:bg-surface-hover'
            }`}
          >
            <Bot className="w-3.5 h-3.5 text-foreground-subtle" />
            <span>Findings</span>
          </button>

          {/* Tab: Terminal */}
          <button
            type="button"
            onClick={() => openTab('terminal')}
            className={`flex items-center gap-1.5 px-2.5 py-1.2 rounded-[7px] border transition-colors cursor-pointer text-[12px] shrink-0 ${
              tabOpen && activeTab === 'terminal'
                ? 'bg-selected border-border text-foreground font-semibold shadow-2xs'
                : 'border-transparent text-foreground-subtle hover:text-foreground hover:bg-surface-hover'
            }`}
          >
            <SquareTerminal className="w-3.5 h-3.5 text-foreground-subtle" />
            <span>Terminal</span>
          </button>

          {/* Tab: Side Chat */}
          <button
            type="button"
            onClick={() => openTab('sideChat')}
            className={`flex items-center gap-1.5 px-2.5 py-1.2 rounded-[7px] border transition-colors cursor-pointer text-[12px] shrink-0 ${
              tabOpen && activeTab === 'sideChat'
                ? 'bg-selected border-border text-foreground font-semibold shadow-2xs'
                : 'border-transparent text-foreground-subtle hover:text-foreground hover:bg-surface-hover'
            }`}
          >
            <MessageSquare className="w-3.5 h-3.5 text-foreground-subtle" />
            <span>Side chat</span>
            {sideMessages.length > 0 && (
              <span className="w-2 h-2 rounded-full bg-success" />
            )}
          </button>

        </div>
      </div>

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
              { id: 'sideChat', label: 'Side conversation', Icon: MessageSquare },
              { id: 'review', label: 'Review', Icon: GitCompare },
              { id: 'terminal', label: 'Terminal', Icon: SquareTerminal },
            ] as const).map(({ id, label, Icon }) => (
              <button
                key={id}
                type="button"
                onClick={() => openTab(id)}
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
                          <Folder className="w-3.5 h-3.5 text-primary dark:text-info" />
                        ) : (
                          <FolderOpen className="w-3.5 h-3.5 text-primary dark:text-info" />
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
      {/* TAB 2: FINDINGS (Dynamic Execution Metrics, Summary & Inspected Assets)    */}
      {/* ========================================================================= */}
      {tabOpen && activeTab === 'findings' && (
        <div className="flex-1 overflow-y-auto p-5 flex flex-col gap-4 select-text">
          
          {/* Status Banner */}
          <div className="flex items-center justify-between w-full">
            <div className="flex items-center gap-2">
              <span className={`w-2.5 h-2.5 rounded-full ${
                findingsData.isRunning ? 'bg-[#D97706] animate-pulse' : 'bg-[#16A34A]'
              }`} />
              <span className="font-semibold text-xs text-foreground">
                {findingsData.isRunning ? 'Execution in progress...' : 'Task execution complete'}
              </span>
            </div>

            <div className="text-ui-xs font-mono text-foreground-subtle px-2 py-0.5 rounded bg-surface-hover dark:bg-[#202022] border border-border">
              {activeSession?.model || currentModel || 'forge-agent'}
            </div>
          </div>

          {/* Goal / User Request Box */}
          <div className="w-full rounded-[10px] bg-surface border border-border p-3 shadow-2xs">
            <div className="text-ui-xs font-semibold text-foreground-subtle mb-1">TASK GOAL</div>
            <p className="text-xs text-foreground-subtle dark:text-foreground-secondary leading-relaxed whitespace-pre-wrap">
              {findingsData.initialUserPrompt}
            </p>
          </div>

          {/* Execution Metrics Grid */}
          <div className="grid grid-cols-4 gap-2">
            <div className="p-2.5 rounded-[8px] bg-card border border-border text-center shadow-2xs">
              <div className="text-[16px] font-medium text-foreground font-mono">
                {findingsData.turnCount}
              </div>
              <div className="text-[10px] text-foreground-subtle">Turns</div>
            </div>

            <div className="p-2.5 rounded-[8px] bg-card border border-border text-center shadow-2xs">
              <div className="text-[16px] font-medium text-success font-mono">
                {findingsData.inspectedFiles.length}
              </div>
              <div className="text-[10px] text-foreground-subtle">Inspected</div>
            </div>

            <div className="p-2.5 rounded-[8px] bg-card border border-border text-center shadow-2xs">
              <div className="text-[16px] font-medium text-primary dark:text-info font-mono">
                {findingsData.terminalCommands.length}
              </div>
              <div className="text-[10px] text-foreground-subtle">Commands</div>
            </div>

            <div className="p-2.5 rounded-[8px] bg-card border border-border text-center shadow-2xs">
              <div className="text-[16px] font-medium text-warning font-mono">
                {findingsData.modifiedFiles.length}
              </div>
              <div className="text-[10px] text-foreground-subtle">Modified</div>
            </div>
          </div>

          {/* Latest Agent Summary */}
          {findingsData.latestResponse && (
            <div className="w-full space-y-1.5">
              <div className="text-xs font-semibold text-foreground">
                Agent Output & Findings
              </div>
              <div className="w-full rounded-[10px] bg-card border border-border p-3 text-xs leading-relaxed max-h-60 overflow-y-auto shadow-2xs">
                <MarkdownRenderer content={findingsData.latestResponse} />
              </div>
            </div>
          )}

          {/* Inspected Files List */}
          {findingsData.inspectedFiles.length > 0 && (
            <div className="w-full space-y-1.5">
              <div className="text-xs font-semibold text-foreground flex items-center justify-between">
                <span>Inspected Files ({findingsData.inspectedFiles.length})</span>
                <span className="text-[10px] text-foreground-subtle">Click to open</span>
              </div>
              <div className="space-y-1 max-h-40 overflow-y-auto">
                {findingsData.inspectedFiles.map((file, idx) => (
                  <div
                    key={idx}
                    onClick={() => openFileInEditor(file)}
                    className="flex items-center justify-between p-2 rounded-[6px] bg-card hover:bg-surface-hover border border-border cursor-pointer transition-colors"
                  >
                    <div className="flex items-center gap-2 truncate">
                      <FileCode className="w-3.5 h-3.5 text-primary dark:text-info shrink-0" />
                      <span className="font-mono text-ui-xs text-foreground truncate">
                        {file}
                      </span>
                    </div>
                    <ExternalLink className="w-3 h-3 text-foreground-subtlest shrink-0" />
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Executed Commands List */}
          {findingsData.terminalCommands.length > 0 && (
            <div className="w-full space-y-1.5">
              <div className="text-xs font-semibold text-foreground">
                Executed Commands ({findingsData.terminalCommands.length})
              </div>
              <div className="space-y-1.5 max-h-44 overflow-y-auto">
                {findingsData.terminalCommands.map((cmd) => {
                  let cmdStr = cmd.toolName;
                  try {
                    const parsed = JSON.parse(cmd.command || '{}');
                    cmdStr = parsed.command || parsed.cmd || cmdStr;
                  } catch {}
                  const isExpanded = !!expandedCommands[cmd.id];

                  return (
                    <div key={cmd.id} className="rounded-[6px] bg-card border border-border p-2 space-y-1">
                      <div 
                        className="flex items-center justify-between cursor-pointer"
                        onClick={() => setExpandedCommands(prev => ({ ...prev, [cmd.id]: !prev[cmd.id] }))}
                      >
                        <div className="flex items-center gap-1.5 truncate">
                          <TerminalIcon className="w-3.5 h-3.5 text-success shrink-0" />
                          <span className="font-mono text-ui-xs text-foreground truncate">
                            {cmdStr}
                          </span>
                        </div>
                        <ChevronDown className={`w-3.5 h-3.5 text-foreground-subtle transition-transform ${isExpanded ? 'rotate-180' : ''}`} />
                      </div>

                      {isExpanded && cmd.output && (
                        <div className="mt-1 p-2 rounded bg-surface-hover dark:bg-[#121214] text-ui-xs font-mono max-h-32 overflow-y-auto whitespace-pre-wrap select-text">
                          {cmd.output}
                        </div>
                      )}
                    </div>
                  );
                })}
              </div>
            </div>
          )}

          {/* Modified Files Section */}
          {findingsData.modifiedFiles.length > 0 && (
            <div className="w-full space-y-1.5">
              <div className="text-xs font-semibold text-foreground flex items-center justify-between">
                <span>Modified Files ({findingsData.modifiedFiles.length})</span>
                <button
                  type="button"
                  onClick={() => openTab('review')}
                  className="text-xs text-success hover:underline cursor-pointer"
                >
                  View in Review Tab
                </button>
              </div>
              <div className="space-y-1">
                {findingsData.modifiedFiles.map(d => (
                  <div
                    key={d.id}
                    onClick={() => onOpenDiff ? onOpenDiff(d) : openDiffInEditor(d)}
                    className="flex items-center justify-between p-2 rounded-[6px] bg-card hover:bg-surface-hover border border-border cursor-pointer transition-colors"
                  >
                    <div className="flex items-center gap-2 truncate">
                      <FileCode className="w-3.5 h-3.5 text-success shrink-0" />
                      <span className="font-mono text-ui-xs text-foreground truncate">
                        {d.fileName}
                      </span>
                    </div>
                    <span className="font-mono text-ui-xs">
                      <span className="text-success">+{d.additions}</span>{' '}
                      <span className="text-destructive">-{d.deletions}</span>
                    </span>
                  </div>
                ))}
              </div>
            </div>
          )}

        </div>
      )}

      {/* ========================================================================= */}
      {/* TAB 3: TERMINAL (Persistent Shell Session with Controls)                  */}
      {/* ========================================================================= */}
      <div className={tabOpen && activeTab === 'terminal' ? 'flex-1 flex flex-col overflow-hidden bg-card' : 'hidden'}>
        {/* Terminal Header */}
        <div className="px-3.5 py-2 bg-surface dark:bg-background border-b border-border flex items-center justify-between text-xs text-foreground-subtle">
          <div className="flex items-center gap-2">
            <span className="font-medium text-foreground">Workspace Shell</span>
            <span className="font-mono text-ui-xs text-foreground-subtle px-1.5 py-0.5 rounded-[4px] bg-surface-hover">
              zsh -l
            </span>
          </div>

          <div className="flex items-center gap-1.5">
            <button
              type="button"
              onClick={handleClearTerminal}
              className="px-2 py-0.5 rounded-[5px] bg-surface-hover hover:bg-border dark:hover:bg-surface-hover text-foreground-subtle hover:text-foreground text-ui-xs transition-colors cursor-pointer"
              title="Clear terminal screen"
            >
              Clear
            </button>
            <button
              type="button"
              onClick={handleRestartShell}
              className="px-2 py-0.5 rounded-[5px] bg-surface-hover hover:bg-border dark:hover:bg-surface-hover text-foreground-subtle hover:text-foreground text-ui-xs transition-colors cursor-pointer flex items-center gap-1"
              title="Restart terminal shell"
            >
              <RefreshCw className="w-3 h-3" />
              <span>Restart</span>
            </button>
          </div>
        </div>

        {/* Terminal Screen */}
        <div className="flex-1 overflow-hidden relative">
          {isInitializingTerminal && !terminalSessionId ? (
            <div className="h-full flex items-center justify-center gap-2 text-xs text-foreground-subtle">
              <Loader2 className="w-4 h-4 animate-spin text-success" />
              <span>Launching shell session...</span>
            </div>
          ) : terminalSessionId ? (
            <TerminalView isActive={activeTab === 'terminal'} sessionId={terminalSessionId} />
          ) : (
            <div className="h-full flex flex-col items-center justify-center p-4 text-center text-xs text-foreground-subtle space-y-3">
              <p>Shell session not active</p>
              <button
                type="button"
                onClick={handleRestartShell}
                className="px-3.5 py-1.5 rounded-[8px] bg-surface-hover text-foreground hover:bg-border transition-colors cursor-pointer"
              >
                Start Shell
              </button>
            </div>
          )}
        </div>
      </div>

      {/* ========================================================================= */}
      {/* TAB 4: SIDE CHAT (Context-Aware Assistant with Markdown & Auto-scroll)    */}
      {/* ========================================================================= */}
      {tabOpen && activeTab === 'sideChat' && (
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

            {sideMessages.length > 0 && (
              <button
                type="button"
                onClick={clearSideConversation}
                className="text-foreground-subtle hover:text-destructive transition-colors cursor-pointer p-1 rounded"
                title="Clear side chat"
              >
                <Trash2 className="w-3.5 h-3.5" />
              </button>
            )}
          </div>

          {/* Messages Area */}
          <div className="flex-1 min-h-0 overflow-y-auto overflow-x-hidden p-4 space-y-3.5 select-text w-full min-w-0">
            {sideMessages.length === 0 ? (
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
                      onClick={() => handleSendSide(undefined, prompt)}
                      className="p-2 rounded-[7px] bg-card hover:bg-surface-hover dark:hover:bg-surface-hover border border-border text-ui-xs text-foreground-subtle hover:text-foreground transition-colors text-left cursor-pointer"
                    >
                      {prompt}
                    </button>
                  ))}
                </div>
              </div>
            ) : (
              sideMessages.map((m, i) => {
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
            <div ref={sideChatBottomRef} />
          </div>

          {/* Sub Chat Input */}
          <form onSubmit={handleSendSide} className="p-3 border-t border-border bg-background flex items-center gap-2">
            <input
              type="text"
              value={sideInput}
              onChange={e => setSideInput(e.target.value)}
              placeholder="Ask side question or review guidance..."
              className="flex-1 bg-surface border border-border rounded-[8px] px-3 py-2 text-xs text-foreground placeholder-foreground-subtlest focus:outline-hidden"
            />
            <button
              type="submit"
              disabled={!sideInput.trim() || isSendingSide}
              className="w-8 h-8 rounded-[8px] bg-surface-hover hover:bg-border text-foreground flex items-center justify-center disabled:opacity-40 transition-colors cursor-pointer shrink-0"
            >
              {isSendingSide ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Send className="w-3.5 h-3.5" />}
            </button>
          </form>

        </div>
      )}

    </aside>
  );
};
