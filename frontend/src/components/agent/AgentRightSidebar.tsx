import React, { useState, useEffect, useMemo, useRef } from 'react';
import { 
  GitCompare, 
  Terminal as TerminalIcon, 
  MessageSquare, 
  RefreshCw, 
  ChevronDown, 
  ChevronRight, 
  FileCode, 
  Send, 
  Trash2,
  CheckCircle2,
  Bot,
  Loader2,
  ChevronsRight,
  SquareTerminal
} from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { TerminalView } from '../terminal-view';
import { FileDiff } from '../../types';
import { ApiBridge } from '../../services/apiBridge';
import { CreateShell, ListSessions } from '../../lib/wails';

interface AgentRightSidebarProps {
  onClose?: () => void;
  onOpenDiff?: (diff: FileDiff) => void;
}

export const AgentRightSidebar: React.FC<AgentRightSidebarProps> = ({ onClose, onOpenDiff }) => {
  const { 
    diffs, 
    openDiffInEditor,
    gitFiles, 
    refreshGitStatus, 
    activeSession, 
    sendSideConversationPrompt, 
    clearSideConversation,
    activeWorkspacePath,
    openFileInEditor,
    currentModel,
    providers
  } = useWorkspace();

  const [activeTab, setActiveTab] = useState<'findings' | 'review' | 'terminal' | 'sideChat'>('findings');
  const [filterMode, setFilterMode] = useState<'unstaged' | 'staged' | 'all'>('unstaged');
  const [sideInput, setSideInput] = useState('');
  const [isSendingSide, setIsSendingSide] = useState(false);
  const [isFilterDropdownOpen, setIsFilterDropdownOpen] = useState(false);
  const [sideChatModel, setSideChatModel] = useState<string>(currentModel || '');

  // PTY Shell Session State
  const [terminalSessionId, setTerminalSessionId] = useState<string | null>(null);
  const [isInitializingTerminal, setIsInitializingTerminal] = useState(false);

  const filterDropdownRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!sideChatModel && currentModel) {
      setSideChatModel(currentModel);
    }
  }, [currentModel, sideChatModel]);

  useEffect(() => {
    const handleOutside = (e: MouseEvent) => {
      if (filterDropdownRef.current && !filterDropdownRef.current.contains(e.target as Node)) {
        setIsFilterDropdownOpen(false);
      }
    };
    document.addEventListener('mousedown', handleOutside);
    return () => document.removeEventListener('mousedown', handleOutside);
  }, []);

  // Initialize Terminal PTY Session
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

  const handleSendSide = async (e?: React.FormEvent) => {
    if (e) e.preventDefault();
    if (!sideInput.trim() || isSendingSide) return;
    const text = sideInput.trim();
    setSideInput('');
    setIsSendingSide(true);
    try {
      await sendSideConversationPrompt(text, sideChatModel || currentModel);
    } finally {
      setIsSendingSide(false);
    }
  };

  // Build Review Files list
  const reviewFiles = useMemo(() => {
    const list: Array<{ 
      path: string; 
      name: string; 
      dir: string; 
      additions: number; 
      deletions: number; 
      staged?: boolean;
      diffObj?: FileDiff 
    }> = [];
    
    for (const d of diffs) {
      const parts = d.filePath.split('/');
      const name = parts.pop() || d.fileName;
      const dir = parts.join('/') || 'root';
      list.push({
        path: d.filePath,
        name,
        dir,
        additions: d.additions,
        deletions: d.deletions,
        diffObj: d
      });
    }

    for (const gf of gitFiles) {
      const isStaged = gf.status === 'staged' || (gf as any).staged === true;
      if (filterMode === 'staged' && !isStaged) continue;
      if (filterMode === 'unstaged' && isStaged) continue;

      if (!list.some(item => item.path === gf.path)) {
        const parts = gf.path.split('/');
        const name = parts.pop() || gf.path;
        const dir = parts.join('/') || 'root';
        list.push({
          path: gf.path,
          name,
          dir,
          additions: 0,
          deletions: 0,
          staged: isStaged
        });
      }
    }

    return list;
  }, [diffs, gitFiles, filterMode]);

  const handleOpenFileDiff = async (file: typeof reviewFiles[0]) => {
    if (file.diffObj) {
      if (onOpenDiff) {
        onOpenDiff(file.diffObj);
      } else {
        openDiffInEditor(file.diffObj);
      }
      return;
    }

    try {
      const diffText = await ApiBridge.gitDiff(file.path, activeWorkspacePath, filterMode === 'staged');
      const constructedDiff: FileDiff = {
        id: `git-${filterMode}-${file.path}`,
        filePath: file.path,
        fileName: file.name,
        originalContent: '',
        modifiedContent: diffText || `(No text changes detected)`,
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

  const sideMessages = activeSession?.sideConversationMessages || [];
  const initialPrompt = activeSession?.messages?.find(m => m.role === 'user')?.content || activeSession?.title || 'Inspect and execute task';

  return (
    <aside className="w-[520px] min-w-[420px] max-w-[640px] h-full bg-[#FFFFFF] dark:bg-[#161617] border-l border-[#E5E7EB] dark:border-[#333336] flex flex-col select-none font-[Inter,system-ui,sans-serif] text-xs text-[#111827] dark:text-[#F2F2F2] transition-colors">
      
      {/* Top Tabs Bar */}
      <div className="h-[56px] min-h-[56px] px-4 border-b border-[#E5E7EB] dark:border-[#333336] flex items-center justify-between bg-[#FFFFFF] dark:bg-[#161617] gap-2 transition-colors">
        <div className="flex items-center gap-2 overflow-x-auto">
          
          {/* Collapse Icon Button */}
          {onClose && (
            <button
              type="button"
              onClick={onClose}
              className="w-8 h-8 rounded-[8px] border border-[#E5E7EB] dark:border-[#333336] flex items-center justify-center text-[#6B7280] dark:text-[#9B9B9F] hover:text-[#111827] dark:hover:text-[#F2F2F2] hover:bg-[#F3F4F6] dark:hover:bg-[#2A2A2D] transition-colors cursor-pointer shrink-0"
              title="Collapse findings panel"
            >
              <ChevronsRight className="w-4 h-4" />
            </button>
          )}

          {/* Tab: Findings */}
          <button
            type="button"
            onClick={() => setActiveTab('findings')}
            className={`flex items-center gap-2 px-3 py-1.5 rounded-[8px] border transition-colors cursor-pointer text-[13px] shrink-0 ${
              activeTab === 'findings'
                ? 'bg-[#EAEBED] dark:bg-[#2A2A2D] border-[#E5E7EB] dark:border-[#333336] text-[#111827] dark:text-[#F2F2F2] font-semibold shadow-xs'
                : 'border-transparent text-[#6B7280] dark:text-[#9B9B9F] hover:text-[#111827] dark:hover:text-[#F2F2F2] hover:bg-[#F3F4F6] dark:hover:bg-[#1E1E20]'
            }`}
          >
            <Bot className="w-4 h-4 text-[#6B7280] dark:text-[#9B9B9F]" />
            <span>Findings</span>
          </button>

          {/* Tab: Review */}
          <button
            type="button"
            onClick={() => setActiveTab('review')}
            className={`flex items-center gap-2 px-3 py-1.5 rounded-[8px] border transition-colors cursor-pointer text-[13px] shrink-0 ${
              activeTab === 'review'
                ? 'bg-[#EAEBED] dark:bg-[#2A2A2D] border-[#E5E7EB] dark:border-[#333336] text-[#111827] dark:text-[#F2F2F2] font-semibold shadow-xs'
                : 'border-transparent text-[#6B7280] dark:text-[#9B9B9F] hover:text-[#111827] dark:hover:text-[#F2F2F2] hover:bg-[#F3F4F6] dark:hover:bg-[#1E1E20]'
            }`}
          >
            <GitCompare className="w-4 h-4 text-[#16A34A] dark:text-[#4ADE80]" />
            <span>Review</span>
            {reviewFiles.length > 0 && (
              <span className="text-[11px] px-1.5 py-0.2 rounded-full bg-[#F3F4F6] dark:bg-[#1E1E20] border border-[#E5E7EB] dark:border-[#333336] text-[#6B7280] dark:text-[#9B9B9F] font-['JetBrains_Mono',monospace]">
                {reviewFiles.length}
              </span>
            )}
          </button>

          {/* Tab: Terminal */}
          <button
            type="button"
            onClick={() => setActiveTab('terminal')}
            className={`flex items-center gap-2 px-3 py-1.5 rounded-[8px] border transition-colors cursor-pointer text-[13px] shrink-0 ${
              activeTab === 'terminal'
                ? 'bg-[#EAEBED] dark:bg-[#2A2A2D] border-[#E5E7EB] dark:border-[#333336] text-[#111827] dark:text-[#F2F2F2] font-semibold shadow-xs'
                : 'border-transparent text-[#6B7280] dark:text-[#9B9B9F] hover:text-[#111827] dark:hover:text-[#F2F2F2] hover:bg-[#F3F4F6] dark:hover:bg-[#1E1E20]'
            }`}
          >
            <SquareTerminal className="w-4 h-4 text-[#6B7280] dark:text-[#9B9B9F]" />
            <span>Terminal</span>
          </button>

          {/* Tab: Side Chat */}
          <button
            type="button"
            onClick={() => setActiveTab('sideChat')}
            className={`flex items-center gap-2 px-3 py-1.5 rounded-[8px] border transition-colors cursor-pointer text-[13px] shrink-0 ${
              activeTab === 'sideChat'
                ? 'bg-[#EAEBED] dark:bg-[#2A2A2D] border-[#E5E7EB] dark:border-[#333336] text-[#111827] dark:text-[#F2F2F2] font-semibold shadow-xs'
                : 'border-transparent text-[#6B7280] dark:text-[#9B9B9F] hover:text-[#111827] dark:hover:text-[#F2F2F2] hover:bg-[#F3F4F6] dark:hover:bg-[#1E1E20]'
            }`}
          >
            <MessageSquare className="w-4 h-4 text-[#6B7280] dark:text-[#9B9B9F]" />
            <span>Side chat</span>
            {sideMessages.length > 0 && (
              <span className="w-2 h-2 rounded-full bg-[#16A34A] dark:bg-[#4ADE80]" />
            )}
          </button>

        </div>
      </div>

      {/* TAB 1: FINDINGS */}
      {activeTab === 'findings' && (
        <div className="flex-1 overflow-y-auto p-6 flex flex-col gap-5 select-text">
          
          {/* Status Pill */}
          <div className="flex items-center justify-end w-full">
            <div className="flex items-center gap-2.5 px-4 py-2 rounded-full bg-[#FFFFFF] dark:bg-[#1E1E20] border border-[#E5E7EB] dark:border-[#333336] text-[13px] text-[#111827] dark:text-[#F2F2F2] shadow-2xs">
              <CheckCircle2 className="w-4 h-4 text-[#16A34A] dark:text-[#4ADE80] shrink-0" />
              <span className="truncate max-w-[280px]">
                {activeSession?.status === 'running' ? 'Active execution in progress...' : 'Task execution report ready'}
              </span>
            </div>
          </div>

          {/* Divider with Model */}
          <div className="w-full flex items-center gap-3 my-1">
            <div className="flex-1 h-[1px] bg-[#E5E7EB] dark:bg-[#333336]"></div>
            <div className="text-[13px] font-['JetBrains_Mono',system-ui,sans-serif] text-[#6B7280] dark:text-[#6B6B70]">
              Using {activeSession?.model || currentModel || 'forge-agent'}
            </div>
            <div className="flex-1 h-[1px] bg-[#E5E7EB] dark:bg-[#333336]"></div>
          </div>

          {/* Prompt Box */}
          <div className="w-full rounded-[12px] bg-[#FFFFFF] dark:bg-[#1E1E20] border border-[#E5E7EB] dark:border-[#333336] p-[18px_20px] shadow-2xs">
            <p className="text-[15px]/[23px] text-[#4B5563] dark:text-[#9B9B9F] whitespace-pre-wrap">
              {initialPrompt}
            </p>
          </div>

          {/* Worked Duration */}
          <div className="flex items-center gap-2 text-[14px] text-[#6B7280] dark:text-[#9B9B9F]">
            <span>Worked in session</span>
            <ChevronRight className="w-4 h-4 text-[#9CA3AF] dark:text-[#6B6B70]" />
          </div>

          <div className="w-full h-[1px] bg-[#E5E7EB] dark:bg-[#333336]" />

          {/* Summary */}
          <div className="text-[16px] text-[#111827] dark:text-[#F2F2F2] font-normal">
            Session summary and findings:
          </div>

          {/* Bullets List */}
          <div className="flex flex-col gap-3.5">
            <div className="flex items-start gap-3">
              <span className="text-[#6B7280] dark:text-[#9B9B9F] text-[15px]">•</span>
              <p className="text-[15px]/[21px] text-[#4B5563] dark:text-[#9B9B9F]">
                Multi-session task streaming events routed to chat transcript with dedicated <span className="bg-[#F3F4F6] dark:bg-[#2A2A2D] text-[#111827] dark:text-[#F2F2F2] font-['JetBrains_Mono',monospace] px-2 py-0.5 rounded-[5px] text-[13px]">Thought</span> and <span className="bg-[#F3F4F6] dark:bg-[#2A2A2D] text-[#111827] dark:text-[#F2F2F2] font-['JetBrains_Mono',monospace] px-2 py-0.5 rounded-[5px] text-[13px]">Terminal</span> blocks.
              </p>
            </div>
            <div className="flex items-start gap-3">
              <span className="text-[#6B7280] dark:text-[#9B9B9F] text-[15px]">•</span>
              <p className="text-[15px]/[21px] text-[#4B5563] dark:text-[#9B9B9F]">
                Inspection tools categorized under <span className="bg-[#F3F4F6] dark:bg-[#2A2A2D] text-[#111827] dark:text-[#F2F2F2] font-['JetBrains_Mono',monospace] px-2 py-0.5 rounded-[5px] text-[13px]">Explore</span> with inspected file paths and status indicators.
              </p>
            </div>
            <div className="flex items-start gap-3">
              <span className="text-[#6B7280] dark:text-[#9B9B9F] text-[15px]">•</span>
              <p className="text-[15px]/[21px] text-[#4B5563] dark:text-[#9B9B9F]">
                Settings screen decoupled to dedicated viewport with full model provider configuration.
              </p>
            </div>
            <div className="flex items-start gap-3">
              <span className="text-[#6B7280] dark:text-[#9B9B9F] text-[15px]">•</span>
              <p className="text-[15px]/[21px] text-[#4B5563] dark:text-[#9B9B9F]">
                Findings panel defaults to collapsed state, accessible on demand via title bar or collapse button.
              </p>
            </div>
          </div>

          {/* Changed Files summary in findings */}
          {reviewFiles.length > 0 && (
            <div className="mt-3 pt-3 border-t border-[#E5E7EB] dark:border-[#333336]">
              <div className="text-sm font-semibold text-[#111827] dark:text-[#F2F2F2] mb-3 flex items-center justify-between">
                <span>Changed Files ({reviewFiles.length})</span>
                <button
                  type="button"
                  onClick={() => setActiveTab('review')}
                  className="text-xs text-[#16A34A] dark:text-[#4ADE80] hover:underline cursor-pointer font-normal"
                >
                  Open Review Tab
                </button>
              </div>
              <div className="space-y-1.5">
                {reviewFiles.slice(0, 4).map((f, i) => (
                  <div 
                    key={i} 
                    onClick={() => handleOpenFileDiff(f)}
                    className="p-2.5 rounded-[8px] bg-[#FFFFFF] dark:bg-[#1E1E20] border border-[#E5E7EB] dark:border-[#333336] hover:bg-[#F9FAFB] dark:hover:bg-[#2A2A2D] transition-colors cursor-pointer flex items-center justify-between shadow-2xs"
                  >
                    <div className="flex items-center gap-2 truncate">
                      <FileCode className="w-3.5 h-3.5 text-[#16A34A] dark:text-[#4ADE80] shrink-0" />
                      <span className="font-['JetBrains_Mono',monospace] text-xs text-[#111827] dark:text-[#F2F2F2] truncate">{f.name}</span>
                    </div>
                    {(f.additions > 0 || f.deletions > 0) && (
                      <span className="font-['JetBrains_Mono',monospace] text-xs shrink-0">
                        <span className="text-[#16A34A] dark:text-[#4ADE80]">+{f.additions}</span>{' '}
                        <span className="text-[#DC2626] dark:text-[#EF4444]">-{f.deletions}</span>
                      </span>
                    )}
                  </div>
                ))}
              </div>
            </div>
          )}

        </div>
      )}

      {/* TAB 2: REVIEW (Git Changes) */}
      {activeTab === 'review' && (
        <div className="flex-1 flex flex-col overflow-hidden">
          {/* Sub-toolbar */}
          <div className="px-4 py-3 border-b border-[#E5E7EB] dark:border-[#333336] flex items-center justify-between bg-[#FFFFFF] dark:bg-[#161617]">
            <div className="relative" ref={filterDropdownRef}>
              <button
                type="button"
                onClick={() => setIsFilterDropdownOpen(prev => !prev)}
                className="flex items-center gap-2 text-xs text-[#111827] dark:text-[#F2F2F2] font-semibold cursor-pointer"
              >
                <span className="capitalize">{filterMode} changes</span>
                <ChevronDown className="w-3.5 h-3.5 text-[#6B7280] dark:text-[#9B9B9F]" />
              </button>

              {isFilterDropdownOpen && (
                <div className="absolute left-0 top-full mt-1.5 w-36 rounded-[8px] bg-white dark:bg-[#1E1E20] border border-[#E5E7EB] dark:border-[#333336] shadow-2xl py-1 z-50 text-xs">
                  {(['unstaged', 'staged', 'all'] as const).map(mode => (
                    <button
                      key={mode}
                      type="button"
                      onClick={() => {
                        setFilterMode(mode);
                        setIsFilterDropdownOpen(false);
                      }}
                      className={`w-full text-left px-3 py-1.5 capitalize hover:bg-[#F3F4F6] dark:hover:bg-[#2A2A2D] cursor-pointer ${
                        filterMode === mode ? 'text-[#16A34A] dark:text-[#4ADE80] font-semibold' : 'text-[#6B7280] dark:text-[#9B9B9F]'
                      }`}
                    >
                      {mode}
                    </button>
                  ))}
                </div>
              )}
            </div>

            <button
              type="button"
              onClick={() => refreshGitStatus()}
              className="flex items-center gap-1.5 text-xs text-[#6B7280] dark:text-[#9B9B9F] hover:text-[#111827] dark:hover:text-[#F2F2F2] transition-colors cursor-pointer"
            >
              <RefreshCw className="w-3.5 h-3.5" />
              <span>Refresh</span>
            </button>
          </div>

          {/* Changed Files List */}
          <div className="flex-1 overflow-y-auto p-3 space-y-2">
            {reviewFiles.length === 0 ? (
              <div className="py-20 text-center text-[#9CA3AF] dark:text-[#6B6B70] space-y-2">
                <CheckCircle2 className="w-8 h-8 text-[#16A34A] dark:text-[#4ADE80] mx-auto opacity-75" />
                <p className="font-medium text-xs text-[#4B5563] dark:text-[#9B9B9F]">No changes detected</p>
                <p className="text-[11px] text-[#6B7280] dark:text-[#6B6B70] max-w-[220px] mx-auto">
                  Workspace tree matches git index cleanly.
                </p>
              </div>
            ) : (
              reviewFiles.map((file, idx) => (
                <div
                  key={idx}
                  className="w-full p-3 rounded-[8px] bg-[#FFFFFF] dark:bg-[#1E1E20] hover:bg-[#F9FAFB] dark:hover:bg-[#2A2A2D] border border-[#E5E7EB] dark:border-[#333336] transition-all cursor-pointer group flex items-center justify-between shadow-2xs"
                  onClick={() => handleOpenFileDiff(file)}
                >
                  <div className="flex items-center gap-2.5 min-w-0 flex-1 pr-2">
                    <FileCode className="w-4 h-4 text-[#16A34A] dark:text-[#4ADE80] shrink-0" />
                    <div className="min-w-0">
                      <p className="text-xs font-medium text-[#111827] dark:text-[#F2F2F2] truncate group-hover:text-black dark:group-hover:text-white font-['JetBrains_Mono',monospace]">
                        {file.name}
                      </p>
                      <p className="text-[11px] text-[#6B7280] dark:text-[#6B6B70] truncate font-['JetBrains_Mono',monospace]">
                        {file.dir}
                      </p>
                    </div>
                  </div>

                  <div className="flex items-center gap-2 shrink-0">
                    {(file.additions > 0 || file.deletions > 0) && (
                      <div className="flex items-center gap-1 font-['JetBrains_Mono',monospace] text-xs">
                        {file.additions > 0 && <span className="text-[#16A34A] dark:text-[#4ADE80]">+{file.additions}</span>}
                        {file.deletions > 0 && <span className="text-[#DC2626] dark:text-[#EF4444]">-{file.deletions}</span>}
                      </div>
                    )}
                    <ChevronRight className="w-4 h-4 text-[#9CA3AF] dark:text-[#6B6B70] group-hover:text-[#111827] dark:group-hover:text-[#9B9B9F]" />
                  </div>
                </div>
              ))
            )}
          </div>

          <div className="p-3 border-t border-[#E5E7EB] dark:border-[#333336] bg-[#FFFFFF] dark:bg-[#161617] flex items-center justify-between text-xs text-[#6B7280] dark:text-[#9B9B9F]">
            <span>{reviewFiles.length} file{reviewFiles.length === 1 ? '' : 's'} changed</span>
            <button
              type="button"
              onClick={() => refreshGitStatus()}
              className="text-[#16A34A] dark:text-[#4ADE80] hover:underline cursor-pointer"
            >
              Sync git status
            </button>
          </div>
        </div>
      )}

      {/* TAB 3: TERMINAL */}
      {activeTab === 'terminal' && (
        <div className="flex-1 flex flex-col overflow-hidden bg-[#FFFFFF] dark:bg-[#0C0C0D]">
          <div className="px-4 py-2 bg-[#F9FAFB] dark:bg-[#161617] border-b border-[#E5E7EB] dark:border-[#333336] flex items-center justify-between text-xs text-[#6B7280] dark:text-[#9B9B9F]">
            <span className="font-medium text-[#111827] dark:text-[#F2F2F2]">Workspace Terminal</span>
            <span className="font-mono text-[11px] text-[#6B7280] dark:text-[#9B9B9F] px-2 py-0.5 rounded-[4px] bg-[#F3F4F6] dark:bg-[#2A2A2D]">
              zsh -l
            </span>
          </div>
          <div className="flex-1 overflow-hidden relative">
            {isInitializingTerminal && !terminalSessionId ? (
              <div className="h-full flex items-center justify-center gap-2 text-xs text-[#6B7280] dark:text-[#9B9B9F]">
                <Loader2 className="w-4 h-4 animate-spin text-[#16A34A] dark:text-[#4ADE80]" />
                <span>Launching shell session...</span>
              </div>
            ) : terminalSessionId ? (
              <TerminalView isActive={true} sessionId={terminalSessionId} />
            ) : (
              <div className="h-full flex flex-col items-center justify-center p-4 text-center text-xs text-[#6B7280] dark:text-[#9B9B9F] space-y-3">
                <p>Shell session not active</p>
                <button
                  type="button"
                  onClick={async () => {
                    const created = await CreateShell('Agent Shell', activeWorkspacePath || '');
                    if (created?.id) setTerminalSessionId(created.id);
                  }}
                  className="px-3.5 py-1.5 rounded-[8px] bg-[#F3F4F6] dark:bg-[#2A2A2D] text-[#111827] dark:text-[#F2F2F2] hover:bg-[#E5E7EB] dark:hover:bg-[#333336] transition-colors cursor-pointer"
                >
                  Start Shell
                </button>
              </div>
            )}
          </div>
        </div>
      )}

      {/* TAB 4: SIDE CHAT */}
      {activeTab === 'sideChat' && (
        <div className="flex-1 flex flex-col overflow-hidden">
          <div className="px-4 py-3 border-b border-[#E5E7EB] dark:border-[#333336] flex items-center justify-between bg-[#FFFFFF] dark:bg-[#161617]">
            <div className="flex items-center gap-2">
              <Bot className="w-4 h-4 text-[#16A34A] dark:text-[#4ADE80]" />
              <span className="font-semibold text-xs text-[#111827] dark:text-[#F2F2F2]">Subagent Side Chat</span>
            </div>
            {sideMessages.length > 0 && (
              <button
                type="button"
                onClick={clearSideConversation}
                className="text-[#6B7280] dark:text-[#9B9B9F] hover:text-[#DC2626] dark:hover:text-[#EF4444] transition-colors cursor-pointer"
                title="Clear side chat"
              >
                <Trash2 className="w-3.5 h-3.5" />
              </button>
            )}
          </div>

          <div className="flex-1 overflow-y-auto p-4 space-y-3 select-text">
            {sideMessages.length === 0 ? (
              <div className="py-20 text-center text-[#9CA3AF] dark:text-[#6B6B70] space-y-2">
                <MessageSquare className="w-7 h-7 text-[#9CA3AF] dark:text-[#6B6B70] mx-auto opacity-70" />
                <p className="text-xs text-[#4B5563] dark:text-[#9B9B9F]">No side messages yet</p>
                <p className="text-[11px] text-[#6B7280] dark:text-[#6B6B70] max-w-[200px] mx-auto">
                  Ask secondary questions without cluttering the main task stream.
                </p>
              </div>
            ) : (
              sideMessages.map((m, i) => {
                const isUser = m.role === 'user';
                return (
                  <div key={m.id || i} className={`flex flex-col ${isUser ? 'items-end' : 'items-start'}`}>
                    <div className={`max-w-[88%] rounded-[10px] p-3 text-xs leading-relaxed ${
                      isUser 
                        ? 'bg-[#EAEBED] dark:bg-[#2A2A2D] text-[#111827] dark:text-[#F2F2F2] border border-[#E5E7EB] dark:border-[#333336]' 
                        : 'bg-white dark:bg-[#1E1E20] text-[#111827] dark:text-[#F2F2F2] border border-[#E5E7EB] dark:border-[#333336] shadow-2xs'
                    }`}>
                      <p className="whitespace-pre-wrap">{m.content}</p>
                    </div>
                  </div>
                );
              })
            )}
          </div>

          {/* Sub Chat Input */}
          <form onSubmit={handleSendSide} className="p-3 border-t border-[#E5E7EB] dark:border-[#333336] bg-[#FFFFFF] dark:bg-[#161617] flex items-center gap-2">
            <input
              type="text"
              value={sideInput}
              onChange={e => setSideInput(e.target.value)}
              placeholder="Ask side question..."
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
