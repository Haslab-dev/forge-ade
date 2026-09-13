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
    '.commandcode',
    '.zcode'
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
  const [filterMode, setFilterMode] = useState<'unstaged' | 'staged' | 'all'>('all');
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
        <div className="p-4 flex items-center justify-center gap-2 text-xs text-[#6B7280] dark:text-[#9B9B9F]">
          <Loader2 className="w-3.5 h-3.5 animate-spin text-[#16A34A] dark:text-[#4ADE80]" />
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
      <div className="border-t border-[#E5E7EB] dark:border-[#333336] bg-[#F9FAFB] dark:bg-[#151516] flex flex-col">
        {/* Diff Toolbar */}
        <div className="px-3 py-1.5 bg-[#F3F4F6] dark:bg-[#1E1E20] border-b border-[#E5E7EB] dark:border-[#333336] flex items-center justify-between text-[11px]">
          <span className="font-mono text-[#6B7280] dark:text-[#9B9B9F] truncate max-w-[200px]" title={file.path}>
            {file.path}
          </span>
          <div className="flex items-center gap-2 shrink-0">
            <button
              type="button"
              onClick={(e) => {
                e.stopPropagation();
                handleOpenFileDiff(file);
              }}
              className="px-2 py-0.5 rounded bg-white dark:bg-[#2A2A2D] hover:bg-[#E5E7EB] dark:hover:bg-[#333336] text-[#6B7280] dark:text-[#9B9B9F] hover:text-[#111827] dark:hover:text-[#F2F2F2] transition-colors cursor-pointer flex items-center gap-1 font-sans"
              title="Open full diff"
            >
              <ExternalLink className="w-3 h-3" />
              <span>Full Diff</span>
            </button>
            <button
              type="button"
              onClick={(e) => toggleFileDiff(file, e)}
              className="px-2 py-0.5 rounded bg-[#E5E7EB] dark:bg-[#2A2A2D] hover:bg-[#D1D5DB] dark:hover:bg-[#38383C] text-[#374151] dark:text-[#E5E7EB] font-medium transition-colors cursor-pointer flex items-center gap-1 font-sans"
              title="Collapse this file diff"
            >
              <FoldVertical className="w-3 h-3" />
              <span>Collapse diff</span>
            </button>
          </div>
        </div>

        {/* Diff Content Viewport */}
        <div className="max-h-72 overflow-y-auto overflow-x-auto p-1 font-['JetBrains_Mono',monospace] text-[11px] leading-[18px] select-text">
          {parsedLines.length === 0 ? (
            <div className="p-3 text-center text-xs text-[#9CA3AF] dark:text-[#6B6B70]">
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
                      ? 'bg-[#DCFCE7]/70 dark:bg-[#064E3B]/40 text-[#166534] dark:text-[#86EFAC]'
                      : isDel
                      ? 'bg-[#FEE2E2]/70 dark:bg-[#450A0A]/40 text-[#991B1B] dark:text-[#FCA5A5]'
                      : isHdr
                      ? 'bg-[#E0E7FF]/40 dark:bg-[#1E1B4B]/30 text-[#2563EB] dark:text-[#60A5FA] font-bold'
                      : 'text-[#374151] dark:text-[#D1D5DB]'
                  }`}
                >
                  <span className="w-8 text-right pr-2 text-[#9CA3AF] dark:text-[#555558] select-none shrink-0 text-[10px]">
                    {line.origLine ?? (isAdd ? '+' : '')}
                  </span>
                  <span className="w-8 text-right pr-2 text-[#9CA3AF] dark:text-[#555558] select-none shrink-0 text-[10px]">
                    {line.modLine ?? (isDel ? '-' : '')}
                  </span>
                  <span className="w-4 text-center select-none font-bold shrink-0">
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
        <div className="px-3 py-1 bg-[#F9FAFB] dark:bg-[#18181A] border-t border-[#E5E7EB] dark:border-[#333336] flex items-center justify-between text-[10px] text-[#6B7280] dark:text-[#9B9B9F]">
          <div className="flex items-center gap-2">
            {file.additions > 0 && <span className="text-[#16A34A] dark:text-[#4ADE80]">+{file.additions} added</span>}
            {file.deletions > 0 && <span className="text-[#DC2626] dark:text-[#EF4444]">-{file.deletions} removed</span>}
          </div>
          <button
            type="button"
            onClick={(e) => toggleFileDiff(file, e)}
            className="hover:text-[#111827] dark:hover:text-[#F2F2F2] cursor-pointer font-medium"
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
      <div
        key={file.path}
        className="w-full rounded-[8px] bg-[#FFFFFF] dark:bg-[#1E1E20] border border-[#E5E7EB] dark:border-[#333336] overflow-hidden shadow-2xs transition-all"
      >
        {/* Accordion Header */}
        <div
          className="w-full p-2.5 hover:bg-[#F9FAFB] dark:hover:bg-[#252528] transition-colors cursor-pointer group flex items-center justify-between"
          onClick={(e) => toggleFileDiff(file, e)}
        >
          <div className="flex items-center gap-2 min-w-0 flex-1 pr-2">
            <span className="text-[#6B7280] dark:text-[#9B9B9F] shrink-0">
              {isExpanded ? <ChevronDown className="w-3.5 h-3.5" /> : <ChevronRight className="w-3.5 h-3.5" />}
            </span>
            <FileCode className="w-4 h-4 text-[#16A34A] dark:text-[#4ADE80] shrink-0" />
            
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-1.5 flex-wrap">
                <p className="text-xs font-semibold text-[#111827] dark:text-[#F2F2F2] truncate font-['JetBrains_Mono',monospace]">
                  {file.name}
                </p>

                {/* Status Badge */}
                <span className={`text-[10px] px-1.5 py-0.2 rounded font-bold font-['JetBrains_Mono',monospace] ${
                  statusChar === 'A' || statusChar === '?' 
                    ? 'bg-[#DCFCE7] text-[#166534] dark:bg-[#064E3B] dark:text-[#86EFAC]'
                    : statusChar === 'D'
                    ? 'bg-[#FEE2E2] text-[#991B1B] dark:bg-[#450A0A] dark:text-[#FCA5A5]'
                    : 'bg-[#FEF3C7] text-[#92400E] dark:bg-[#451A03] dark:text-[#FDE68A]'
                }`}>
                  {statusChar === '?' ? 'UNT' : statusChar}
                </span>

                {/* Staged Badge */}
                {isStaged && (
                  <span className="text-[9px] px-1.2 py-0.2 rounded bg-[#E0E7FF] text-[#3730A3] dark:bg-[#1E1B4B] dark:text-[#C7D2FE] font-medium font-['JetBrains_Mono',monospace]">
                    STAGED
                  </span>
                )}

                {/* Agent Badge */}
                {file.isAgentDiff && (
                  <span className="text-[9px] px-1.2 py-0.2 rounded bg-[#F3E8FF] text-[#6B21A8] dark:bg-[#3B0764] dark:text-[#E9D5FF] font-medium flex items-center gap-0.5">
                    <Sparkles className="w-2.5 h-2.5" />
                    <span>Agent</span>
                  </span>
                )}
              </div>

              <p className="text-[10.5px] text-[#6B7280] dark:text-[#6B6B70] truncate font-['JetBrains_Mono',monospace]">
                {file.dir}
              </p>
            </div>
          </div>

          {/* Right side: Additions/Deletions + Action Buttons */}
          <div className="flex items-center gap-2 shrink-0" onClick={e => e.stopPropagation()}>
            {(file.additions > 0 || file.deletions > 0) && (
              <div className="flex items-center gap-1 font-['JetBrains_Mono',monospace] text-[11px]">
                {file.additions > 0 && <span className="text-[#16A34A] dark:text-[#4ADE80]">+{file.additions}</span>}
                {file.deletions > 0 && <span className="text-[#DC2626] dark:text-[#EF4444]">-{file.deletions}</span>}
              </div>
            )}

            {/* Discard confirmation overlay on row */}
            {discardConfirmPath === file.path ? (
              <div className="flex items-center gap-1 bg-[#FEE2E2] dark:bg-[#450A0A] p-1 rounded-[6px]">
                <span className="text-[10px] text-[#DC2626] dark:text-[#FCA5A5] font-semibold px-1">Revert?</span>
                <button
                  type="button"
                  onClick={(e) => handleDiscardFile(e, file.path)}
                  className="px-1.5 py-0.5 rounded bg-[#DC2626] text-white text-[10px] font-bold cursor-pointer"
                >
                  Yes
                </button>
                <button
                  type="button"
                  onClick={(e) => { e.stopPropagation(); setDiscardConfirmPath(null); }}
                  className="px-1.5 py-0.5 rounded bg-white dark:bg-[#2A2A2D] text-[#6B7280] text-[10px] cursor-pointer"
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
                      className="p-1 rounded hover:bg-[#DCFCE7] dark:hover:bg-[#064E3B] text-[#16A34A] dark:text-[#4ADE80] transition-colors cursor-pointer"
                      title="Accept agent diff"
                    >
                      <Check className="w-3.5 h-3.5" />
                    </button>
                    <button
                      type="button"
                      onClick={() => rejectDiff(file.diffObj!.id)}
                      className="p-1 rounded hover:bg-[#FEE2E2] dark:hover:bg-[#450A0A] text-[#DC2626] dark:text-[#EF4444] transition-colors cursor-pointer"
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
                    className="p-1 rounded hover:bg-[#E5E7EB] dark:hover:bg-[#2A2A2D] text-[#6B7280] dark:text-[#9B9B9F] hover:text-[#111827] dark:hover:text-[#F2F2F2] transition-colors cursor-pointer"
                    title="Unstage changes"
                  >
                    <Minus className="w-3.5 h-3.5" />
                  </button>
                ) : (
                  <button
                    type="button"
                    onClick={(e) => handleStageFile(e, file.path)}
                    className="p-1 rounded hover:bg-[#E5E7EB] dark:hover:bg-[#2A2A2D] text-[#6B7280] dark:text-[#9B9B9F] hover:text-[#111827] dark:hover:text-[#F2F2F2] transition-colors cursor-pointer"
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
                  className="p-1 rounded hover:bg-[#FEE2E2] dark:hover:bg-[#450A0A] text-[#6B7280] dark:text-[#9B9B9F] hover:text-[#DC2626] dark:hover:text-[#EF4444] transition-colors cursor-pointer"
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
                  className="p-1 rounded hover:bg-[#E5E7EB] dark:hover:bg-[#2A2A2D] text-[#6B7280] dark:text-[#9B9B9F] hover:text-[#111827] dark:hover:text-[#F2F2F2] transition-colors cursor-pointer"
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
    <aside className="w-[540px] min-w-[440px] max-w-[660px] h-full bg-[#FFFFFF] dark:bg-[#161617] border-l border-[#E5E7EB] dark:border-[#333336] flex flex-col select-none font-[Inter,system-ui,sans-serif] text-xs text-[#111827] dark:text-[#F2F2F2] transition-colors">
      
      {/* Top Tabs Bar */}
      <div className="h-[52px] min-h-[52px] px-3 border-b border-[#E5E7EB] dark:border-[#333336] flex items-center justify-between bg-[#FFFFFF] dark:bg-[#161617] gap-1.5 transition-colors">
        <div className="flex items-center gap-1.5 overflow-x-auto">
          
          {/* Collapse Icon Button */}
          {onClose && (
            <button
              type="button"
              onClick={onClose}
              className="w-7 h-7 rounded-[6px] border border-[#E5E7EB] dark:border-[#333336] flex items-center justify-center text-[#6B7280] dark:text-[#9B9B9F] hover:text-[#111827] dark:hover:text-[#F2F2F2] hover:bg-[#F3F4F6] dark:hover:bg-[#2A2A2D] transition-colors cursor-pointer shrink-0"
              title="Collapse panel"
            >
              <ChevronsRight className="w-3.5 h-3.5" />
            </button>
          )}

          {/* Tab: Review */}
          <button
            type="button"
            onClick={() => setActiveTab('review')}
            className={`flex items-center gap-1.5 px-2.5 py-1.2 rounded-[7px] border transition-colors cursor-pointer text-[12px] shrink-0 ${
              activeTab === 'review'
                ? 'bg-[#EAEBED] dark:bg-[#2A2A2D] border-[#E5E7EB] dark:border-[#333336] text-[#111827] dark:text-[#F2F2F2] font-semibold shadow-2xs'
                : 'border-transparent text-[#6B7280] dark:text-[#9B9B9F] hover:text-[#111827] dark:hover:text-[#F2F2F2] hover:bg-[#F3F4F6] dark:hover:bg-[#1E1E20]'
            }`}
          >
            <GitCompare className="w-3.5 h-3.5 text-[#16A34A] dark:text-[#4ADE80]" />
            <span>Review</span>
            {allReviewFiles.length > 0 && (
              <span className="text-[10px] px-1.5 py-0.2 rounded-full bg-[#F3F4F6] dark:bg-[#1E1E20] border border-[#E5E7EB] dark:border-[#333336] text-[#6B7280] dark:text-[#9B9B9F] font-['JetBrains_Mono',monospace]">
                {allReviewFiles.length}
              </span>
            )}
          </button>

          {/* Tab: Findings */}
          <button
            type="button"
            onClick={() => setActiveTab('findings')}
            className={`flex items-center gap-1.5 px-2.5 py-1.2 rounded-[7px] border transition-colors cursor-pointer text-[12px] shrink-0 ${
              activeTab === 'findings'
                ? 'bg-[#EAEBED] dark:bg-[#2A2A2D] border-[#E5E7EB] dark:border-[#333336] text-[#111827] dark:text-[#F2F2F2] font-semibold shadow-2xs'
                : 'border-transparent text-[#6B7280] dark:text-[#9B9B9F] hover:text-[#111827] dark:hover:text-[#F2F2F2] hover:bg-[#F3F4F6] dark:hover:bg-[#1E1E20]'
            }`}
          >
            <Bot className="w-3.5 h-3.5 text-[#6B7280] dark:text-[#9B9B9F]" />
            <span>Findings</span>
          </button>

          {/* Tab: Terminal */}
          <button
            type="button"
            onClick={() => setActiveTab('terminal')}
            className={`flex items-center gap-1.5 px-2.5 py-1.2 rounded-[7px] border transition-colors cursor-pointer text-[12px] shrink-0 ${
              activeTab === 'terminal'
                ? 'bg-[#EAEBED] dark:bg-[#2A2A2D] border-[#E5E7EB] dark:border-[#333336] text-[#111827] dark:text-[#F2F2F2] font-semibold shadow-2xs'
                : 'border-transparent text-[#6B7280] dark:text-[#9B9B9F] hover:text-[#111827] dark:hover:text-[#F2F2F2] hover:bg-[#F3F4F6] dark:hover:bg-[#1E1E20]'
            }`}
          >
            <SquareTerminal className="w-3.5 h-3.5 text-[#6B7280] dark:text-[#9B9B9F]" />
            <span>Terminal</span>
          </button>

          {/* Tab: Side Chat */}
          <button
            type="button"
            onClick={() => setActiveTab('sideChat')}
            className={`flex items-center gap-1.5 px-2.5 py-1.2 rounded-[7px] border transition-colors cursor-pointer text-[12px] shrink-0 ${
              activeTab === 'sideChat'
                ? 'bg-[#EAEBED] dark:bg-[#2A2A2D] border-[#E5E7EB] dark:border-[#333336] text-[#111827] dark:text-[#F2F2F2] font-semibold shadow-2xs'
                : 'border-transparent text-[#6B7280] dark:text-[#9B9B9F] hover:text-[#111827] dark:hover:text-[#F2F2F2] hover:bg-[#F3F4F6] dark:hover:bg-[#1E1E20]'
            }`}
          >
            <MessageSquare className="w-3.5 h-3.5 text-[#6B7280] dark:text-[#9B9B9F]" />
            <span>Side chat</span>
            {sideMessages.length > 0 && (
              <span className="w-2 h-2 rounded-full bg-[#16A34A] dark:bg-[#4ADE80]" />
            )}
          </button>

        </div>
      </div>

      {/* ========================================================================= */}
      {/* TAB 1: REVIEW (Git & Session Diff Review with Complete Action Controls)   */}
      {/* ========================================================================= */}
      {activeTab === 'review' && (
        <div className="flex-1 min-h-0 flex flex-col overflow-hidden">
          
          {/* Sub-toolbar: Filters + Stage All / Unstage All + Collapse Diffs + Refresh */}
          <div className="px-3.5 py-2 border-b border-[#E5E7EB] dark:border-[#333336] flex items-center justify-between bg-[#F9FAFB] dark:bg-[#1A1A1C] gap-2 flex-wrap">
            
            {/* Filter Pills */}
            <div className="flex items-center bg-[#E5E7EB] dark:bg-[#262628] p-0.5 rounded-[7px] text-[11px]">
              <button
                type="button"
                onClick={() => setFilterMode('all')}
                className={`px-2 py-0.5 rounded-[5px] transition-colors cursor-pointer ${
                  filterMode === 'all'
                    ? 'bg-white dark:bg-[#1E1E20] text-[#111827] dark:text-[#F2F2F2] font-semibold shadow-2xs'
                    : 'text-[#6B7280] dark:text-[#9B9B9F]'
                }`}
              >
                All ({allReviewFiles.length})
              </button>
              <button
                type="button"
                onClick={() => setFilterMode('unstaged')}
                className={`px-2 py-0.5 rounded-[5px] transition-colors cursor-pointer ${
                  filterMode === 'unstaged'
                    ? 'bg-white dark:bg-[#1E1E20] text-[#111827] dark:text-[#F2F2F2] font-semibold shadow-2xs'
                    : 'text-[#6B7280] dark:text-[#9B9B9F]'
                }`}
              >
                Unstaged ({unstagedCount})
              </button>
              <button
                type="button"
                onClick={() => setFilterMode('staged')}
                className={`px-2 py-0.5 rounded-[5px] transition-colors cursor-pointer ${
                  filterMode === 'staged'
                    ? 'bg-white dark:bg-[#1E1E20] text-[#111827] dark:text-[#F2F2F2] font-semibold shadow-2xs'
                    : 'text-[#6B7280] dark:text-[#9B9B9F]'
                }`}
              >
                Staged ({stagedCount})
              </button>
            </div>

            {/* Quick Actions */}
            <div className="flex items-center gap-1.5">
              {/* Collapse / Expand Diffs button */}
              {filteredReviewFiles.length > 0 && (
                <button
                  type="button"
                  onClick={handleToggleAllDiffs}
                  className="px-2 py-1 rounded-[6px] bg-[#F3F4F6] hover:bg-[#E5E7EB] dark:bg-[#2A2A2D] dark:hover:bg-[#333336] text-[#111827] dark:text-[#F2F2F2] text-[11px] font-medium transition-colors cursor-pointer flex items-center gap-1"
                  title={areAnyDiffsExpanded ? "Collapse all diffs" : "Expand all diffs"}
                >
                  {areAnyDiffsExpanded ? (
                    <>
                      <ChevronsDownUp className="w-3.5 h-3.5 text-[#6B7280] dark:text-[#9B9B9F]" />
                      <span>Collapse diffs</span>
                    </>
                  ) : (
                    <>
                      <ChevronsUpDown className="w-3.5 h-3.5 text-[#6B7280] dark:text-[#9B9B9F]" />
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
                    ? 'bg-[#E5E7EB] dark:bg-[#2A2A2D] text-[#111827] dark:text-[#F2F2F2]'
                    : 'text-[#6B7280] dark:text-[#9B9B9F] hover:text-[#111827] dark:hover:text-[#F2F2F2]'
                }`}
                title={isGroupByFolder ? "Folder grouping ON (click for flat list)" : "Folder grouping OFF (click to group by folder)"}
              >
                <Folder className="w-3.5 h-3.5" />
              </button>

              {unstagedCount > 0 && filterMode !== 'staged' && (
                <button
                  type="button"
                  onClick={handleStageAll}
                  className="px-2 py-1 rounded-[6px] bg-[#F3F4F6] hover:bg-[#E5E7EB] dark:bg-[#2A2A2D] dark:hover:bg-[#333336] text-[#111827] dark:text-[#F2F2F2] text-[11px] font-medium transition-colors cursor-pointer"
                  title="Stage all unstaged changes"
                >
                  Stage All
                </button>
              )}

              {stagedCount > 0 && filterMode !== 'unstaged' && (
                <button
                  type="button"
                  onClick={handleUnstageAll}
                  className="px-2 py-1 rounded-[6px] bg-[#F3F4F6] hover:bg-[#E5E7EB] dark:bg-[#2A2A2D] dark:hover:bg-[#333336] text-[#111827] dark:text-[#F2F2F2] text-[11px] font-medium transition-colors cursor-pointer"
                  title="Unstage all staged changes"
                >
                  Unstage All
                </button>
              )}

              <button
                type="button"
                onClick={handleRefresh}
                className="p-1 rounded-[6px] text-[#6B7280] dark:text-[#9B9B9F] hover:text-[#111827] dark:hover:text-[#F2F2F2] hover:bg-[#E5E7EB] dark:hover:bg-[#2A2A2D] transition-colors cursor-pointer"
                title="Refresh Git status"
              >
                <RefreshCw className={`w-3.5 h-3.5 ${isRefreshingGit ? 'animate-spin text-[#16A34A]' : ''}`} />
              </button>
            </div>

          </div>

          {/* Changed Files List (with Folder Grouping & File Accordions) */}
          <div className="flex-1 min-h-0 overflow-y-auto p-3 space-y-2.5">
            {filteredReviewFiles.length === 0 ? (
              <div className="py-20 text-center text-[#9CA3AF] dark:text-[#6B6B70] space-y-2">
                <CheckCircle2 className="w-8 h-8 text-[#16A34A] dark:text-[#4ADE80] mx-auto opacity-75" />
                <p className="font-medium text-xs text-[#4B5563] dark:text-[#9B9B9F]">No changes detected</p>
                <p className="text-[11px] text-[#6B7280] dark:text-[#6B6B70] max-w-[220px] mx-auto">
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
                      className="w-full px-2.5 py-1.5 rounded-[6px] bg-[#F3F4F6]/80 dark:bg-[#1E1E20]/80 hover:bg-[#E5E7EB] dark:hover:bg-[#28282B] flex items-center justify-between text-left transition-colors cursor-pointer"
                    >
                      <div className="flex items-center gap-1.5 min-w-0 flex-1">
                        <span className="text-[#6B7280] dark:text-[#9B9B9F]">
                          {isFolderCollapsed ? <ChevronRight className="w-3.5 h-3.5" /> : <ChevronDown className="w-3.5 h-3.5" />}
                        </span>
                        {isFolderCollapsed ? (
                          <Folder className="w-3.5 h-3.5 text-[#2563EB] dark:text-[#60A5FA]" />
                        ) : (
                          <FolderOpen className="w-3.5 h-3.5 text-[#2563EB] dark:text-[#60A5FA]" />
                        )}
                        <span className="font-mono text-xs font-semibold text-[#111827] dark:text-[#F2F2F2] truncate">
                          {folder}
                        </span>
                        <span className="text-[10.5px] text-[#6B7280] dark:text-[#6B6B70]">
                          ({files.length} {files.length === 1 ? 'file' : 'files'})
                        </span>
                      </div>

                      <div className="flex items-center gap-2 shrink-0 font-['JetBrains_Mono',monospace] text-[10.5px]">
                        {folderAdditions > 0 && <span className="text-[#16A34A] dark:text-[#4ADE80]">+{folderAdditions}</span>}
                        {folderDeletions > 0 && <span className="text-[#DC2626] dark:text-[#EF4444]">-{folderDeletions}</span>}
                      </div>
                    </button>

                    {/* Files in Folder */}
                    {!isFolderCollapsed && (
                      <div className="pl-2 space-y-2 border-l-2 border-[#E5E7EB] dark:border-[#2A2A2D] ml-2">
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
          <div className="p-3 border-t border-[#E5E7EB] dark:border-[#333336] bg-[#FFFFFF] dark:bg-[#161617] space-y-2">
            {commitFeedback && (
              <div className="text-[11px] px-2 py-1 rounded bg-[#F3F4F6] dark:bg-[#202022] text-[#4B5563] dark:text-[#9B9B9F] font-mono">
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
                className="flex-1 bg-[#F9FAFB] dark:bg-[#1E1E20] border border-[#E5E7EB] dark:border-[#333336] rounded-[7px] px-2.5 py-1.5 text-xs text-[#111827] dark:text-[#F2F2F2] placeholder-[#9CA3AF] dark:placeholder-[#6B6B70] focus:outline-hidden font-['JetBrains_Mono',monospace]"
              />

              {/* AI Commit Generator */}
              <button
                type="button"
                onClick={handleGenerateAiCommit}
                disabled={isGeneratingAiCommit}
                className="p-1.5 rounded-[7px] bg-[#F3F4F6] hover:bg-[#E5E7EB] dark:bg-[#2A2A2D] dark:hover:bg-[#333336] text-[#2563EB] dark:text-[#60A5FA] transition-colors cursor-pointer shrink-0 disabled:opacity-40"
                title="Generate commit message with AI"
              >
                {isGeneratingAiCommit ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Sparkles className="w-3.5 h-3.5" />}
              </button>

              {/* Commit Button */}
              <button
                type="button"
                onClick={handleCommit}
                disabled={!commitMessage.trim() || isCommitting}
                className="px-3 py-1.5 rounded-[7px] bg-[#16A34A] hover:bg-[#15803D] text-white font-medium text-xs transition-colors cursor-pointer shrink-0 disabled:opacity-40 flex items-center gap-1"
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
                className="p-1.5 rounded-[7px] bg-[#F3F4F6] hover:bg-[#E5E7EB] dark:bg-[#2A2A2D] dark:hover:bg-[#333336] text-[#6B7280] dark:text-[#9B9B9F] hover:text-[#111827] dark:hover:text-[#F2F2F2] transition-colors cursor-pointer shrink-0 disabled:opacity-40"
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
      {activeTab === 'findings' && (
        <div className="flex-1 overflow-y-auto p-5 flex flex-col gap-4 select-text">
          
          {/* Status Banner */}
          <div className="flex items-center justify-between w-full">
            <div className="flex items-center gap-2">
              <span className={`w-2.5 h-2.5 rounded-full ${
                findingsData.isRunning ? 'bg-[#D97706] animate-pulse' : 'bg-[#16A34A]'
              }`} />
              <span className="font-semibold text-xs text-[#111827] dark:text-[#F2F2F2]">
                {findingsData.isRunning ? 'Execution in progress...' : 'Task execution complete'}
              </span>
            </div>

            <div className="text-[11px] font-['JetBrains_Mono',monospace] text-[#6B7280] dark:text-[#9B9B9F] px-2 py-0.5 rounded bg-[#F3F4F6] dark:bg-[#202022] border border-[#E5E7EB] dark:border-[#333336]">
              {activeSession?.model || currentModel || 'forge-agent'}
            </div>
          </div>

          {/* Goal / User Request Box */}
          <div className="w-full rounded-[10px] bg-[#F9FAFB] dark:bg-[#1E1E20] border border-[#E5E7EB] dark:border-[#333336] p-3 shadow-2xs">
            <div className="text-[11px] font-semibold text-[#6B7280] dark:text-[#9B9B9F] mb-1">TASK GOAL</div>
            <p className="text-xs text-[#374151] dark:text-[#CCCCCC] leading-relaxed whitespace-pre-wrap">
              {findingsData.initialUserPrompt}
            </p>
          </div>

          {/* Execution Metrics Grid */}
          <div className="grid grid-cols-4 gap-2">
            <div className="p-2.5 rounded-[8px] bg-[#FFFFFF] dark:bg-[#1E1E20] border border-[#E5E7EB] dark:border-[#333336] text-center shadow-2xs">
              <div className="text-[16px] font-bold text-[#111827] dark:text-[#F2F2F2] font-mono">
                {findingsData.turnCount}
              </div>
              <div className="text-[10px] text-[#6B7280] dark:text-[#9B9B9F]">Turns</div>
            </div>

            <div className="p-2.5 rounded-[8px] bg-[#FFFFFF] dark:bg-[#1E1E20] border border-[#E5E7EB] dark:border-[#333336] text-center shadow-2xs">
              <div className="text-[16px] font-bold text-[#16A34A] dark:text-[#4ADE80] font-mono">
                {findingsData.inspectedFiles.length}
              </div>
              <div className="text-[10px] text-[#6B7280] dark:text-[#9B9B9F]">Inspected</div>
            </div>

            <div className="p-2.5 rounded-[8px] bg-[#FFFFFF] dark:bg-[#1E1E20] border border-[#E5E7EB] dark:border-[#333336] text-center shadow-2xs">
              <div className="text-[16px] font-bold text-[#2563EB] dark:text-[#60A5FA] font-mono">
                {findingsData.terminalCommands.length}
              </div>
              <div className="text-[10px] text-[#6B7280] dark:text-[#9B9B9F]">Commands</div>
            </div>

            <div className="p-2.5 rounded-[8px] bg-[#FFFFFF] dark:bg-[#1E1E20] border border-[#E5E7EB] dark:border-[#333336] text-center shadow-2xs">
              <div className="text-[16px] font-bold text-[#D97706] dark:text-[#F5A623] font-mono">
                {findingsData.modifiedFiles.length}
              </div>
              <div className="text-[10px] text-[#6B7280] dark:text-[#9B9B9F]">Modified</div>
            </div>
          </div>

          {/* Latest Agent Summary */}
          {findingsData.latestResponse && (
            <div className="w-full space-y-1.5">
              <div className="text-xs font-semibold text-[#111827] dark:text-[#F2F2F2]">
                Agent Output & Findings
              </div>
              <div className="w-full rounded-[10px] bg-[#FFFFFF] dark:bg-[#1E1E20] border border-[#E5E7EB] dark:border-[#333336] p-3 text-xs leading-relaxed max-h-60 overflow-y-auto shadow-2xs">
                <MarkdownRenderer content={findingsData.latestResponse} />
              </div>
            </div>
          )}

          {/* Inspected Files List */}
          {findingsData.inspectedFiles.length > 0 && (
            <div className="w-full space-y-1.5">
              <div className="text-xs font-semibold text-[#111827] dark:text-[#F2F2F2] flex items-center justify-between">
                <span>Inspected Files ({findingsData.inspectedFiles.length})</span>
                <span className="text-[10px] text-[#6B7280] dark:text-[#9B9B9F]">Click to open</span>
              </div>
              <div className="space-y-1 max-h-40 overflow-y-auto">
                {findingsData.inspectedFiles.map((file, idx) => (
                  <div
                    key={idx}
                    onClick={() => openFileInEditor(file)}
                    className="flex items-center justify-between p-2 rounded-[6px] bg-[#FFFFFF] dark:bg-[#1E1E20] hover:bg-[#F3F4F6] dark:hover:bg-[#2A2A2D] border border-[#E5E7EB] dark:border-[#333336] cursor-pointer transition-colors"
                  >
                    <div className="flex items-center gap-2 truncate">
                      <FileCode className="w-3.5 h-3.5 text-[#2563EB] dark:text-[#60A5FA] shrink-0" />
                      <span className="font-['JetBrains_Mono',monospace] text-[11px] text-[#111827] dark:text-[#F2F2F2] truncate">
                        {file}
                      </span>
                    </div>
                    <ExternalLink className="w-3 h-3 text-[#9CA3AF] dark:text-[#6B6B70] shrink-0" />
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Executed Commands List */}
          {findingsData.terminalCommands.length > 0 && (
            <div className="w-full space-y-1.5">
              <div className="text-xs font-semibold text-[#111827] dark:text-[#F2F2F2]">
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
                    <div key={cmd.id} className="rounded-[6px] bg-[#FFFFFF] dark:bg-[#1E1E20] border border-[#E5E7EB] dark:border-[#333336] p-2 space-y-1">
                      <div 
                        className="flex items-center justify-between cursor-pointer"
                        onClick={() => setExpandedCommands(prev => ({ ...prev, [cmd.id]: !prev[cmd.id] }))}
                      >
                        <div className="flex items-center gap-1.5 truncate">
                          <TerminalIcon className="w-3.5 h-3.5 text-[#16A34A] dark:text-[#4ADE80] shrink-0" />
                          <span className="font-['JetBrains_Mono',monospace] text-[11px] text-[#111827] dark:text-[#F2F2F2] truncate">
                            {cmdStr}
                          </span>
                        </div>
                        <ChevronDown className={`w-3.5 h-3.5 text-[#9CA3AF] transition-transform ${isExpanded ? 'rotate-180' : ''}`} />
                      </div>

                      {isExpanded && cmd.output && (
                        <div className="mt-1 p-2 rounded bg-[#F3F4F6] dark:bg-[#121214] text-[10.5px] font-['JetBrains_Mono',monospace] max-h-32 overflow-y-auto whitespace-pre-wrap select-text">
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
              <div className="text-xs font-semibold text-[#111827] dark:text-[#F2F2F2] flex items-center justify-between">
                <span>Modified Files ({findingsData.modifiedFiles.length})</span>
                <button
                  type="button"
                  onClick={() => setActiveTab('review')}
                  className="text-xs text-[#16A34A] dark:text-[#4ADE80] hover:underline cursor-pointer"
                >
                  View in Review Tab
                </button>
              </div>
              <div className="space-y-1">
                {findingsData.modifiedFiles.map(d => (
                  <div
                    key={d.id}
                    onClick={() => onOpenDiff ? onOpenDiff(d) : openDiffInEditor(d)}
                    className="flex items-center justify-between p-2 rounded-[6px] bg-[#FFFFFF] dark:bg-[#1E1E20] hover:bg-[#F3F4F6] dark:hover:bg-[#2A2A2D] border border-[#E5E7EB] dark:border-[#333336] cursor-pointer transition-colors"
                  >
                    <div className="flex items-center gap-2 truncate">
                      <FileCode className="w-3.5 h-3.5 text-[#16A34A] dark:text-[#4ADE80] shrink-0" />
                      <span className="font-['JetBrains_Mono',monospace] text-[11px] text-[#111827] dark:text-[#F2F2F2] truncate">
                        {d.fileName}
                      </span>
                    </div>
                    <span className="font-['JetBrains_Mono',monospace] text-[11px]">
                      <span className="text-[#16A34A] dark:text-[#4ADE80]">+{d.additions}</span>{' '}
                      <span className="text-[#DC2626] dark:text-[#EF4444]">-{d.deletions}</span>
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
      <div className={activeTab === 'terminal' ? 'flex-1 flex flex-col overflow-hidden bg-[#FFFFFF] dark:bg-[#0C0C0D]' : 'hidden'}>
        {/* Terminal Header */}
        <div className="px-3.5 py-2 bg-[#F9FAFB] dark:bg-[#161617] border-b border-[#E5E7EB] dark:border-[#333336] flex items-center justify-between text-xs text-[#6B7280] dark:text-[#9B9B9F]">
          <div className="flex items-center gap-2">
            <span className="font-medium text-[#111827] dark:text-[#F2F2F2]">Workspace Shell</span>
            <span className="font-['JetBrains_Mono',monospace] text-[10.5px] text-[#6B7280] dark:text-[#9B9B9F] px-1.5 py-0.5 rounded-[4px] bg-[#F3F4F6] dark:bg-[#2A2A2D]">
              zsh -l
            </span>
          </div>

          <div className="flex items-center gap-1.5">
            <button
              type="button"
              onClick={handleClearTerminal}
              className="px-2 py-0.5 rounded-[5px] bg-[#F3F4F6] hover:bg-[#E5E7EB] dark:bg-[#2A2A2D] dark:hover:bg-[#333336] text-[#6B7280] dark:text-[#9B9B9F] hover:text-[#111827] dark:hover:text-[#F2F2F2] text-[11px] transition-colors cursor-pointer"
              title="Clear terminal screen"
            >
              Clear
            </button>
            <button
              type="button"
              onClick={handleRestartShell}
              className="px-2 py-0.5 rounded-[5px] bg-[#F3F4F6] hover:bg-[#E5E7EB] dark:bg-[#2A2A2D] dark:hover:bg-[#333336] text-[#6B7280] dark:text-[#9B9B9F] hover:text-[#111827] dark:hover:text-[#F2F2F2] text-[11px] transition-colors cursor-pointer flex items-center gap-1"
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
            <div className="h-full flex items-center justify-center gap-2 text-xs text-[#6B7280] dark:text-[#9B9B9F]">
              <Loader2 className="w-4 h-4 animate-spin text-[#16A34A] dark:text-[#4ADE80]" />
              <span>Launching shell session...</span>
            </div>
          ) : terminalSessionId ? (
            <TerminalView isActive={activeTab === 'terminal'} sessionId={terminalSessionId} />
          ) : (
            <div className="h-full flex flex-col items-center justify-center p-4 text-center text-xs text-[#6B7280] dark:text-[#9B9B9F] space-y-3">
              <p>Shell session not active</p>
              <button
                type="button"
                onClick={handleRestartShell}
                className="px-3.5 py-1.5 rounded-[8px] bg-[#F3F4F6] dark:bg-[#2A2A2D] text-[#111827] dark:text-[#F2F2F2] hover:bg-[#E5E7EB] dark:hover:bg-[#333336] transition-colors cursor-pointer"
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
      {activeTab === 'sideChat' && (
        <div className="flex-1 min-h-0 flex flex-col overflow-hidden">
          
          {/* Header */}
          <div className="px-3.5 py-2.5 border-b border-[#E5E7EB] dark:border-[#333336] flex items-center justify-between bg-[#F9FAFB] dark:bg-[#1A1A1C]">
            <div className="flex items-center gap-2">
              <Bot className="w-4 h-4 text-[#16A34A] dark:text-[#4ADE80]" />
              <span className="font-semibold text-xs text-[#111827] dark:text-[#F2F2F2]">Subagent Side Chat</span>
              <span className="text-[10px] font-mono text-[#6B7280] dark:text-[#9B9B9F] px-1.5 py-0.2 rounded bg-[#E5E7EB] dark:bg-[#2A2A2D]">
                {currentModel || 'active'}
              </span>
            </div>

            {sideMessages.length > 0 && (
              <button
                type="button"
                onClick={clearSideConversation}
                className="text-[#6B7280] dark:text-[#9B9B9F] hover:text-[#DC2626] dark:hover:text-[#EF4444] transition-colors cursor-pointer p-1 rounded"
                title="Clear side chat"
              >
                <Trash2 className="w-3.5 h-3.5" />
              </button>
            )}
          </div>

          {/* Messages Area */}
          <div className="flex-1 min-h-0 overflow-y-auto p-4 space-y-3.5 select-text">
            {sideMessages.length === 0 ? (
              <div className="py-12 text-center text-[#9CA3AF] dark:text-[#6B6B70] space-y-3">
                <MessageSquare className="w-8 h-8 text-[#9CA3AF] dark:text-[#6B6B70] mx-auto opacity-70" />
                <div>
                  <p className="text-xs font-semibold text-[#4B5563] dark:text-[#9B9B9F]">Side Assistant</p>
                  <p className="text-[11px] text-[#6B7280] dark:text-[#6B6B70] max-w-[240px] mx-auto mt-0.5">
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
                      className="p-2 rounded-[7px] bg-[#FFFFFF] dark:bg-[#1E1E20] hover:bg-[#F3F4F6] dark:hover:bg-[#252528] border border-[#E5E7EB] dark:border-[#333336] text-[11px] text-[#4B5563] dark:text-[#9B9B9F] hover:text-[#111827] dark:hover:text-[#F2F2F2] transition-colors text-left cursor-pointer"
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
                  <div key={m.id || i} className={`flex flex-col ${isUser ? 'items-end' : 'items-start'}`}>
                    <div className={`max-w-[95%] w-full rounded-[10px] p-3 text-xs leading-relaxed ${
                      isUser 
                        ? 'bg-[#EAEBED] dark:bg-[#2A2A2D] text-[#111827] dark:text-[#F2F2F2] border border-[#E5E7EB] dark:border-[#333336]' 
                        : 'bg-white dark:bg-[#1E1E20] text-[#111827] dark:text-[#F2F2F2] border border-[#E5E7EB] dark:border-[#333336] shadow-2xs'
                    }`}>
                      {isUser ? (
                        <p className="whitespace-pre-wrap">{m.content}</p>
                      ) : (
                        <>
                          {/* 1. Thoughts */}
                          {m.thoughts && m.thoughts.length > 0 && (
                            <div className="space-y-1.5 mb-2.5 w-full">
                              {m.thoughts.map(th => {
                                const isExpanded = expandedSideThoughts[th.id] ?? false;
                                return (
                                  <div key={th.id} className="rounded-[8px] bg-[#F3F4F6] dark:bg-[#202022] border border-[#E5E7EB] dark:border-[#333336] overflow-hidden text-xs">
                                    <button
                                      type="button"
                                      onClick={() => setExpandedSideThoughts(prev => ({ ...prev, [th.id]: !isExpanded }))}
                                      className="w-full px-2.5 py-1.5 flex items-center justify-between text-left hover:bg-[#EAEBED] dark:hover:bg-[#262628] transition-colors cursor-pointer"
                                    >
                                      <div className="flex items-center gap-1.5 text-[#4B5563] dark:text-[#9B9B9F]">
                                        <Brain className="w-3.5 h-3.5 text-[#6B7280] dark:text-[#9B9B9F]" />
                                        <span className="font-semibold text-[11px]">Thought</span>
                                        <span className="text-[#9CA3AF] dark:text-[#6B6B70]">·</span>
                                        <span className="text-[10.5px] text-[#6B7280] dark:text-[#6B6B70]">
                                          {th.durationSeconds ? `${th.durationSeconds}s` : 'a few seconds'}
                                        </span>
                                      </div>
                                      {isExpanded ? <ChevronDown className="w-3.5 h-3.5 text-[#6B7280]" /> : <ChevronRight className="w-3.5 h-3.5 text-[#6B7280]" />}
                                    </button>
                                    {isExpanded && (
                                      <div className="p-2.5 border-t border-[#E5E7EB] dark:border-[#333336] max-h-52 overflow-y-auto font-mono text-[11px] text-[#4B5563] dark:text-[#9B9B9F] whitespace-pre-wrap select-text leading-relaxed">
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
                            <div className="space-y-1.5 mb-2.5 w-full">
                              {m.toolExecutions.map(t => {
                                const isExpanded = expandedSideTools[t.id] ?? false;
                                const isRunning = t.status === 'running';
                                const isFailed = t.status === 'failed';
                                return (
                                  <div key={t.id} className="rounded-[8px] bg-[#F9FAFB] dark:bg-[#1A1A1C] border border-[#E5E7EB] dark:border-[#333336] overflow-hidden text-xs">
                                    <button
                                      type="button"
                                      onClick={() => setExpandedSideTools(prev => ({ ...prev, [t.id]: !isExpanded }))}
                                      className="w-full px-2.5 py-1.5 flex items-center justify-between text-left hover:bg-[#F3F4F6] dark:hover:bg-[#222225] transition-colors cursor-pointer"
                                    >
                                      <div className="flex items-center gap-2 min-w-0 flex-1 pr-2">
                                        {isRunning ? (
                                          <Loader2 className="w-3.5 h-3.5 animate-spin text-[#D97706] dark:text-[#F5A623] shrink-0" />
                                        ) : isFailed ? (
                                          <X className="w-3.5 h-3.5 text-[#DC2626] dark:text-[#EF4444] shrink-0" />
                                        ) : (
                                          <Check className="w-3.5 h-3.5 text-[#16A34A] dark:text-[#4ADE80] shrink-0" />
                                        )}
                                        <TerminalIcon className="w-3.5 h-3.5 text-[#6B7280] dark:text-[#9B9B9F] shrink-0" />
                                        <span className="font-mono text-[11px] font-semibold text-[#111827] dark:text-[#F2F2F2] truncate">
                                          {t.toolName || 'Tool Execution'}
                                        </span>
                                      </div>
                                      <div className="flex items-center gap-1.5 shrink-0 text-[#6B7280] dark:text-[#9B9B9F]">
                                        <span className={`text-[10px] font-mono font-medium px-1.5 py-0.2 rounded ${
                                          isRunning ? 'bg-[#FEF3C7] text-[#92400E] dark:bg-[#451A03] dark:text-[#FDE68A]' :
                                          isFailed ? 'bg-[#FEE2E2] text-[#991B1B] dark:bg-[#450A0A] dark:text-[#FCA5A5]' :
                                          'bg-[#DCFCE7] text-[#166534] dark:bg-[#064E3B] dark:text-[#86EFAC]'
                                        }`}>
                                          {t.status}
                                        </span>
                                        {isExpanded ? <ChevronDown className="w-3.5 h-3.5" /> : <ChevronRight className="w-3.5 h-3.5" />}
                                      </div>
                                    </button>
                                    {isExpanded && (
                                      <div className="p-2 border-t border-[#E5E7EB] dark:border-[#333336] bg-[#FFFFFF] dark:bg-[#121213] space-y-1.5">
                                        {t.command && (
                                          <div className="text-[10.5px] font-mono text-[#6B7280] dark:text-[#9B9B9F] truncate">
                                            <span className="font-semibold text-[#374151] dark:text-[#D1D5DB]">Args:</span> {t.command}
                                          </div>
                                        )}
                                        <pre className="p-2 rounded bg-[#F3F4F6] dark:bg-[#1C1C1E] text-[11px] font-mono text-[#111827] dark:text-[#E5E7EB] max-h-48 overflow-auto whitespace-pre-wrap select-text">
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
                            <div className="flex items-center gap-2 text-[#D97706] dark:text-[#F5A623] py-1 mt-1 font-mono text-[11px]">
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
          <form onSubmit={handleSendSide} className="p-3 border-t border-[#E5E7EB] dark:border-[#333336] bg-[#FFFFFF] dark:bg-[#161617] flex items-center gap-2">
            <input
              type="text"
              value={sideInput}
              onChange={e => setSideInput(e.target.value)}
              placeholder="Ask side question or review guidance..."
              className="flex-1 bg-[#F9FAFB] dark:bg-[#1E1E20] border border-[#E5E7EB] dark:border-[#333336] rounded-[8px] px-3 py-2 text-xs text-[#111827] dark:text-[#F2F2F2] placeholder-[#9CA3AF] dark:placeholder-[#6B6B70] focus:outline-hidden"
            />
            <button
              type="submit"
              disabled={!sideInput.trim() || isSendingSide}
              className="w-8 h-8 rounded-[8px] bg-[#F3F4F6] dark:bg-[#2A2A2D] hover:bg-[#E5E7EB] dark:hover:bg-[#333336] text-[#111827] dark:text-[#F2F2F2] flex items-center justify-center disabled:opacity-40 transition-colors cursor-pointer shrink-0"
            >
              {isSendingSide ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Send className="w-3.5 h-3.5" />}
            </button>
          </form>

        </div>
      )}

    </aside>
  );
};
