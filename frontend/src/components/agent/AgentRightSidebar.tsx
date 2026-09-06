import React, { useState, useEffect, useMemo, useRef } from 'react';
import { 
  GitCompare, 
  Terminal as TerminalIcon, 
  MessageSquare, 
  RefreshCw, 
  ChevronDown, 
  X, 
  FileCode, 
  Send, 
  Trash2,
  Sparkles,
  CheckCircle2,
  Bot,
  Loader2
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

  const [activeTab, setActiveTab] = useState<'review' | 'terminal' | 'sideChat'>('review');
  const [filterMode, setFilterMode] = useState<'unstaged' | 'staged' | 'all'>('unstaged');
  const [sideInput, setSideInput] = useState('');
  const [isSendingSide, setIsSendingSide] = useState(false);
  const [isFilterDropdownOpen, setIsFilterDropdownOpen] = useState(false);
  const [isModelDropdownOpen, setIsModelDropdownOpen] = useState(false);
  const [sideChatModel, setSideChatModel] = useState<string>(currentModel || '');

  // PTY Shell Session State
  const [terminalSessionId, setTerminalSessionId] = useState<string | null>(null);
  const [isInitializingTerminal, setIsInitializingTerminal] = useState(false);

  const filterDropdownRef = useRef<HTMLDivElement>(null);
  const modelDropdownRef = useRef<HTMLDivElement>(null);

  // Sync sideChatModel when currentModel changes and user hasn't overridden
  useEffect(() => {
    if (!sideChatModel && currentModel) {
      setSideChatModel(currentModel);
    }
  }, [currentModel, sideChatModel]);

  // Handle outside click for dropdowns
  useEffect(() => {
    const handleOutside = (e: MouseEvent) => {
      if (filterDropdownRef.current && !filterDropdownRef.current.contains(e.target as Node)) {
        setIsFilterDropdownOpen(false);
      }
      if (modelDropdownRef.current && !modelDropdownRef.current.contains(e.target as Node)) {
        setIsModelDropdownOpen(false);
      }
    };
    document.addEventListener('mousedown', handleOutside);
    return () => document.removeEventListener('mousedown', handleOutside);
  }, []);

  // Initialize Terminal PTY Session (Login shell)
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

  // Group models by provider for model selector
  const groupedModels = useMemo(() => {
    const map = new Map<string, { providerName: string; models: string[] }>();
    for (const p of providers) {
      if (!p.enabled) continue;
      const validModels = (p.selectedModels && p.selectedModels.length > 0)
        ? p.selectedModels
        : (p.models && p.models.length > 0)
        ? p.models
        : [];
      if (validModels.length > 0) {
        map.set(p.id, {
          providerName: p.name,
          models: validModels
        });
      }
    }
    return Array.from(map.values());
  }, [providers]);

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

  // Build Real Review Files list
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
    
    // Add from session diffs
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

    // Add from real git status
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

  return (
    <aside className="w-[340px] min-w-[340px] h-full bg-white dark:bg-[#181819] border-l border-[#e5e7eb] dark:border-[#242426] flex flex-col select-none font-sans text-xs text-[#374151] dark:text-[#cccccc] transition-colors">
      
      {/* Tab Header Bar */}
      <div className="h-[40px] min-h-[40px] px-2.5 bg-[#f9fafb] dark:bg-[#1a1a1c] border-b border-[#e5e7eb] dark:border-[#242426] flex items-center justify-between">
        <div className="flex items-center gap-1">
          {/* Review Tab */}
          <button
            type="button"
            onClick={() => setActiveTab('review')}
            className={`flex items-center gap-1.5 px-2.5 py-1 rounded-md transition-colors cursor-pointer text-xs ${
              activeTab === 'review'
                ? 'bg-[#e5e7eb] dark:bg-[#262629] text-[#111827] dark:text-white font-medium shadow-2xs'
                : 'text-[#6b7280] dark:text-[#888888] hover:text-[#111827] dark:hover:text-[#cccccc] hover:bg-[#f3f4f6] dark:hover:bg-[#202022]'
            }`}
          >
            <GitCompare className="w-3.5 h-3.5 text-[#2563eb] dark:text-[#3b82f6]" />
            <span>Review</span>
            {reviewFiles.length > 0 && (
              <span className="text-[10px] px-1 py-0.2 rounded-full bg-[#d1d5db] dark:bg-[#333336] text-[#374151] dark:text-[#aaaaaa] font-mono">
                {reviewFiles.length}
              </span>
            )}
          </button>

          {/* Terminal Tab */}
          <button
            type="button"
            onClick={() => setActiveTab('terminal')}
            className={`flex items-center gap-1.5 px-2.5 py-1 rounded-md transition-colors cursor-pointer text-xs ${
              activeTab === 'terminal'
                ? 'bg-[#e5e7eb] dark:bg-[#262629] text-[#111827] dark:text-white font-medium shadow-2xs'
                : 'text-[#6b7280] dark:text-[#888888] hover:text-[#111827] dark:hover:text-[#cccccc] hover:bg-[#f3f4f6] dark:hover:bg-[#202022]'
            }`}
          >
            <TerminalIcon className="w-3.5 h-3.5 text-[#d97706] dark:text-[#eab308]" />
            <span>Terminal</span>
          </button>

          {/* Side Conversation Tab */}
          <button
            type="button"
            onClick={() => setActiveTab('sideChat')}
            className={`flex items-center gap-1.5 px-2.5 py-1 rounded-md transition-colors cursor-pointer text-xs ${
              activeTab === 'sideChat'
                ? 'bg-[#e5e7eb] dark:bg-[#262629] text-[#111827] dark:text-white font-medium shadow-2xs'
                : 'text-[#6b7280] dark:text-[#888888] hover:text-[#111827] dark:hover:text-[#cccccc] hover:bg-[#f3f4f6] dark:hover:bg-[#202022]'
            }`}
          >
            <MessageSquare className="w-3.5 h-3.5 text-[#059669] dark:text-[#10b981]" />
            <span>Side chat</span>
            {sideMessages.length > 0 && (
              <span className="w-1.5 h-1.5 rounded-full bg-[#10b981]" />
            )}
          </button>
        </div>

        {/* Close Button */}
        {onClose && (
          <button
            type="button"
            onClick={onClose}
            className="p-1 rounded text-[#6b7280] dark:text-[#888888] hover:text-[#111827] dark:hover:text-white hover:bg-[#e5e7eb] dark:hover:bg-[#252528] transition-colors cursor-pointer"
            title="Close sidebar"
          >
            <X className="w-3.5 h-3.5" />
          </button>
        )}
      </div>

      {/* TAB 1: REVIEW (Git Changes) */}
      {activeTab === 'review' && (
        <div className="flex-1 flex flex-col overflow-hidden">
          {/* Sub-toolbar: Filter dropdown & Refresh */}
          <div className="px-3 py-2 border-b border-[#e5e7eb] dark:border-[#242426] flex items-center justify-between bg-[#f9fafb] dark:bg-[#19191a]">
            {/* Filter Dropdown */}
            <div className="relative" ref={filterDropdownRef}>
              <button
                type="button"
                onClick={() => setIsFilterDropdownOpen(prev => !prev)}
                className="flex items-center gap-1.5 text-xs text-[#374151] dark:text-[#dddddd] hover:text-[#111827] dark:hover:text-white font-medium cursor-pointer"
              >
                <span className="capitalize">{filterMode}</span>
                <ChevronDown className="w-3 h-3 text-[#9ca3af] dark:text-[#777777]" />
              </button>

              {isFilterDropdownOpen && (
                <div className="absolute left-0 top-full mt-1 w-32 rounded-lg bg-white dark:bg-[#222225] border border-[#e5e7eb] dark:border-[#333336] shadow-xl py-1 z-50 text-xs">
                  {(['unstaged', 'staged', 'all'] as const).map(mode => (
                    <button
                      key={mode}
                      type="button"
                      onClick={() => {
                        setFilterMode(mode);
                        setIsFilterDropdownOpen(false);
                      }}
                      className={`w-full text-left px-3 py-1.5 capitalize hover:bg-[#f3f4f6] dark:hover:bg-[#2b2b2e] cursor-pointer ${
                        filterMode === mode 
                          ? 'text-[#2563eb] dark:text-white font-semibold' 
                          : 'text-[#4b5563] dark:text-[#aaaaaa]'
                      }`}
                    >
                      {mode}
                    </button>
                  ))}
                </div>
              )}
            </div>

            {/* Refresh Button */}
            <button
              type="button"
              onClick={() => refreshGitStatus()}
              className="flex items-center gap-1 text-[11px] text-[#6b7280] dark:text-[#888888] hover:text-[#111827] dark:hover:text-white transition-colors cursor-pointer"
            >
              <RefreshCw className="w-3 h-3" />
              <span>Refresh</span>
            </button>
          </div>

          {/* Changed Files List */}
          <div className="flex-1 overflow-y-auto p-2 space-y-1">
            {reviewFiles.length === 0 ? (
              <div className="py-16 text-center text-[#9ca3af] dark:text-[#777777] space-y-2">
                <CheckCircle2 className="w-8 h-8 text-[#10b981] mx-auto opacity-75" />
                <p className="font-medium text-xs text-[#4b5563] dark:text-[#aaaaaa]">No changes detected</p>
                <p className="text-[11px] text-[#6b7280] dark:text-[#666669] max-w-[200px] mx-auto">
                  Workspace tree matches git index cleanly.
                </p>
              </div>
            ) : (
              reviewFiles.map((file, idx) => (
                <div
                  key={idx}
                  className="w-full p-2 rounded-lg bg-[#f9fafb] dark:bg-[#1e1e20] hover:bg-[#f3f4f6] dark:hover:bg-[#252528] border border-[#e5e7eb] dark:border-[#2b2b2e] hover:border-[#cbd5e1] dark:hover:border-[#3b3b40] transition-all cursor-pointer group flex items-center justify-between"
                  onClick={() => handleOpenFileDiff(file)}
                >
                  <div className="flex items-center gap-2 min-w-0 flex-1 pr-2">
                    <FileCode className="w-3.5 h-3.5 text-[#2563eb] dark:text-[#3b82f6] shrink-0" />
                    <div className="min-w-0">
                      <p className="text-xs font-medium text-[#1f2937] dark:text-[#dddddd] truncate group-hover:text-[#111827] dark:group-hover:text-white">
                        {file.name}
                      </p>
                      <p className="text-[10px] text-[#6b7280] dark:text-[#777777] truncate font-mono">
                        {file.dir}
                      </p>
                    </div>
                  </div>

                  <div className="flex items-center gap-2 shrink-0">
                    {(file.additions > 0 || file.deletions > 0) && (
                      <div className="flex items-center gap-1 font-mono text-[11px]">
                        {file.additions > 0 && <span className="text-[#16a34a] dark:text-[#22c55e]">+{file.additions}</span>}
                        {file.deletions > 0 && <span className="text-[#dc2626] dark:text-[#ef4444]">-{file.deletions}</span>}
                      </div>
                    )}
                    <ChevronDown className="w-3.5 h-3.5 text-[#9ca3af] dark:text-[#666666] group-hover:text-[#4b5563] dark:group-hover:text-[#aaaaaa]" />
                  </div>
                </div>
              ))
            )}
          </div>

          {/* Bottom Git Action Summary */}
          <div className="p-3 border-t border-[#e5e7eb] dark:border-[#242426] bg-[#f9fafb] dark:bg-[#161617] flex items-center justify-between text-[11px] text-[#6b7280] dark:text-[#888888]">
            <span>{reviewFiles.length} file{reviewFiles.length === 1 ? '' : 's'} changed</span>
            <button
              type="button"
              onClick={() => refreshGitStatus()}
              className="text-[#2563eb] dark:text-[#3b82f6] hover:underline cursor-pointer"
            >
              Sync status
            </button>
          </div>
        </div>
      )}

      {/* TAB 2: TERMINAL */}
      {activeTab === 'terminal' && (
        <div className="flex-1 flex flex-col overflow-hidden bg-white dark:bg-[#0c0c0c]">
          <div className="px-3 py-1.5 bg-[#f9fafb] dark:bg-[#161617] border-b border-[#e5e7eb] dark:border-[#242426] flex items-center justify-between text-[11px] text-[#6b7280] dark:text-[#888888]">
            <span className="font-medium">Workspace Login Shell</span>
            <span className="font-mono text-[10px] text-[#4b5563] dark:text-[#aaaaaa] px-1.5 py-0.5 rounded bg-[#e5e7eb] dark:bg-[#252528]">
              zsh -l
            </span>
          </div>
          <div className="flex-1 overflow-hidden relative">
            {isInitializingTerminal && !terminalSessionId ? (
              <div className="h-full flex items-center justify-center gap-2 text-xs text-[#6b7280] dark:text-[#888888]">
                <Loader2 className="w-4 h-4 animate-spin text-[#2563eb] dark:text-[#3b82f6]" />
                <span>Launching login shell...</span>
              </div>
            ) : terminalSessionId ? (
              <TerminalView isActive={true} sessionId={terminalSessionId} />
            ) : (
              <div className="h-full flex flex-col items-center justify-center p-4 text-center text-xs text-[#6b7280] dark:text-[#888888] space-y-2">
                <p>Shell session not available</p>
                <button
                  type="button"
                  onClick={async () => {
                    const created = await CreateShell('Agent Shell', activeWorkspacePath || '');
                    if (created?.id) setTerminalSessionId(created.id);
                  }}
                  className="px-3 py-1.5 rounded bg-[#2563eb] text-white hover:bg-[#1d4ed8] transition-colors cursor-pointer"
                >
                  Start Shell
                </button>
              </div>
            )}
          </div>
        </div>
      )}

      {/* TAB 3: SIDE CONVERSATION (Sub Chat Session) */}
      {activeTab === 'sideChat' && (
        <div className="flex-1 flex flex-col overflow-hidden">
          {/* Top Bar for Sub Chat with Model Selector */}
          <div className="px-3 py-2 border-b border-[#e5e7eb] dark:border-[#242426] flex items-center justify-between bg-[#f9fafb] dark:bg-[#19191a]">
            <div className="flex items-center gap-1.5">
              <Sparkles className="w-3.5 h-3.5 text-[#059669] dark:text-[#10b981]" />
              <span className="font-semibold text-xs text-[#111827] dark:text-white">Sub Chat</span>
            </div>

            <div className="flex items-center gap-2">
              {/* Model Selector Pill Dropdown */}
              <div className="relative" ref={modelDropdownRef}>
                <button
                  type="button"
                  onClick={() => setIsModelDropdownOpen(prev => !prev)}
                  className="flex items-center gap-1 text-[11px] px-2 py-0.5 rounded-full border border-[#d1d5db] dark:border-[#333336] bg-white dark:bg-[#222225] text-[#374151] dark:text-[#cccccc] hover:border-[#2563eb] dark:hover:border-[#3b82f6] cursor-pointer transition-colors max-w-[130px]"
                  title={sideChatModel || currentModel || 'Select Model'}
                >
                  <Bot className="w-3 h-3 text-[#2563eb] dark:text-[#3b82f6] shrink-0" />
                  <span className="truncate">{sideChatModel || currentModel || 'Model'}</span>
                  <ChevronDown className="w-2.5 h-2.5 text-[#9ca3af] dark:text-[#777777] shrink-0" />
                </button>

                {isModelDropdownOpen && (
                  <div className="absolute right-0 top-full mt-1 w-64 max-h-72 overflow-y-auto rounded-lg bg-white dark:bg-[#202023] border border-[#e5e7eb] dark:border-[#333336] shadow-xl py-1 z-50 text-xs">
                    <div className="px-3 py-1 text-[10px] font-semibold uppercase tracking-wider text-[#9ca3af] dark:text-[#666669] border-b border-[#f3f4f6] dark:border-[#2b2b2e]">
                      Select Model
                    </div>
                    {groupedModels.length === 0 ? (
                      <div className="px-3 py-2 text-xs text-[#6b7280] dark:text-[#888888]">
                        No configured models. Please configure a provider in Settings.
                      </div>
                    ) : (
                      groupedModels.map(group => (
                        <div key={group.providerName} className="py-1">
                          <div className="px-3 py-0.5 text-[10px] font-bold text-[#6b7280] dark:text-[#888888] uppercase tracking-wider">
                            {group.providerName}
                          </div>
                          {group.models.map(m => (
                            <button
                              key={m}
                              type="button"
                              onClick={() => {
                                setSideChatModel(m);
                                setIsModelDropdownOpen(false);
                              }}
                              className={`w-full text-left px-4 py-1.5 hover:bg-[#f3f4f6] dark:hover:bg-[#2a2a2e] cursor-pointer truncate flex items-center justify-between ${
                                (sideChatModel || currentModel) === m 
                                  ? 'text-[#2563eb] dark:text-blue-400 font-medium bg-[#eff6ff] dark:bg-[#1e293b]' 
                                  : 'text-[#374151] dark:text-[#cccccc]'
                              }`}
                            >
                              <span className="truncate">{m}</span>
                              {(sideChatModel || currentModel) === m && (
                                <span className="w-1.5 h-1.5 rounded-full bg-[#2563eb] dark:bg-blue-400 shrink-0 ml-2" />
                              )}
                            </button>
                          ))}
                        </div>
                      ))
                    )}
                  </div>
                )}
              </div>

              {/* Clear History Button */}
              {sideMessages.length > 0 && (
                <button
                  type="button"
                  onClick={clearSideConversation}
                  className="text-[11px] text-[#6b7280] dark:text-[#888888] hover:text-[#dc2626] dark:hover:text-[#ef4444] transition-colors cursor-pointer flex items-center gap-1"
                  title="Clear sub chat history"
                >
                  <Trash2 className="w-3 h-3" />
                </button>
              )}
            </div>
          </div>

          {/* Messages Stream */}
          <div className="flex-1 overflow-y-auto p-3 space-y-3">
            {sideMessages.length === 0 ? (
              <div className="py-12 text-center text-[#9ca3af] dark:text-[#777777] space-y-2">
                <MessageSquare className="w-8 h-8 text-[#cbd5e1] dark:text-[#555558] mx-auto" />
                <p className="font-medium text-xs text-[#4b5563] dark:text-[#aaaaaa]">Sub Chat Session</p>
                <p className="text-[11px] leading-relaxed max-w-[220px] mx-auto text-[#6b7280] dark:text-[#666669]">
                  Ask side questions or explore ideas with real LLM streaming without affecting the primary task thread.
                </p>
              </div>
            ) : (
              sideMessages.map(msg => (
                <div 
                  key={msg.id}
                  className={`flex flex-col ${msg.role === 'user' ? 'items-end' : 'items-start'}`}
                >
                  <div
                    className={`max-w-[88%] rounded-xl px-3 py-2 text-xs leading-relaxed ${
                      msg.role === 'user'
                        ? 'bg-[#2563eb] text-white font-sans'
                        : 'bg-[#f3f4f6] dark:bg-[#222225] text-[#1f2937] dark:text-[#dddddd] border border-[#e5e7eb] dark:border-[#333336]'
                    }`}
                  >
                    {msg.isThinking && !msg.content ? (
                      <span className="flex items-center gap-1.5 text-[#6b7280] dark:text-[#999999] italic text-[11px]">
                        <span className="w-1.5 h-1.5 rounded-full bg-[#10b981] animate-ping" />
                        Generating response...
                      </span>
                    ) : (
                      <p className="whitespace-pre-wrap select-text">{msg.content}</p>
                    )}
                  </div>
                  <span className="text-[9px] text-[#9ca3af] dark:text-[#666666] mt-0.5 px-1 font-mono">
                    {msg.timestamp}
                  </span>
                </div>
              ))
            )}
          </div>

          {/* Prompt Input Form */}
          <form onSubmit={handleSendSide} className="p-2.5 border-t border-[#e5e7eb] dark:border-[#242426] bg-[#f9fafb] dark:bg-[#161617] flex items-center gap-1.5">
            <input
              type="text"
              value={sideInput}
              onChange={e => setSideInput(e.target.value)}
              placeholder="Ask side question..."
              className="flex-1 bg-white dark:bg-[#202023] border border-[#d1d5db] dark:border-[#333336] rounded-lg px-3 py-1.5 text-xs text-[#111827] dark:text-white placeholder-[#9ca3af] dark:placeholder-[#777777] focus:outline-hidden focus:border-[#2563eb] dark:focus:border-[#3b82f6] transition-colors"
            />
            <button
              type="submit"
              disabled={!sideInput.trim() || isSendingSide}
              className={`p-1.5 rounded-lg transition-colors cursor-pointer ${
                sideInput.trim() && !isSendingSide
                  ? 'bg-[#2563eb] text-white hover:bg-[#1d4ed8]'
                  : 'bg-[#e5e7eb] dark:bg-[#262628] text-[#9ca3af] dark:text-[#666666] cursor-not-allowed'
              }`}
            >
              <Send className="w-3.5 h-3.5" />
            </button>
          </form>
        </div>
      )}

    </aside>

  );
};
