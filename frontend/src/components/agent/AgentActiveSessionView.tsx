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
  Search,
  ThumbsUp,
  ThumbsDown,
  AlertCircle,
  Image as ImageIcon
} from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { FileDiff, ToolExecution, ThoughtStep, AgentMessage } from '../../types';
import { AgentTaskInputBar } from './AgentTaskInputBar';
import { AgentRightSidebar } from './AgentRightSidebar';
import { DiffViewer } from '../diff/DiffViewer';
import { MarkdownRenderer } from './MarkdownRenderer';

export const AgentActiveSessionView: React.FC = () => {
  const { 
    activeSession, 
    activeSessionId, 
    activeWorkspacePath, 
    gitBranch, 
    diffs, 
    openDiffInEditor, 
    currentModel,
    deleteSessionPermanently,
    isRightActionDrawerOpen,
    setIsRightActionDrawerOpen
  } = useWorkspace();

  const [expandedThoughts, setExpandedThoughts] = useState<Record<string, boolean>>({});
  const [expandedTools, setExpandedTools] = useState<Record<string, boolean>>({});
  const [expandedExplore, setExpandedExplore] = useState<Record<string, boolean>>({});
  const [activeInlineDiff, setActiveInlineDiff] = useState<FileDiff | null>(null);
  const [isMenuOpen, setIsMenuOpen] = useState(false);
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
          userPrompt: msg.content || 'Task prompt',
          timestamp: msg.timestamp
        };
      } else if (msg.role === 'agent' && currentTurn) {
        if (!currentTurn.agentPreview) {
          const thoughtPreview = msg.thoughts?.[0]?.thoughtText || '';
          currentTurn.agentPreview = msg.content || thoughtPreview || 'Running tools...';
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

  const currentProjectName = activeSession.workspacePath 
    ? activeSession.workspacePath.split('/').filter(Boolean).pop() || 'General'
    : activeWorkspacePath ? activeWorkspacePath.split('/').filter(Boolean).pop() || 'forge-ade' : 'forge-ade';

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
    <div className="flex-1 h-full bg-[#F8F9FA] dark:bg-[#161617] text-[#111827] dark:text-[#F2F2F2] flex overflow-hidden select-none font-[Inter,system-ui,sans-serif] relative transition-colors">
      
      {/* Middle Chat & Task Stream Column */}
      <div className="flex-1 flex flex-col h-full overflow-hidden bg-[#F8F9FA] dark:bg-[#161617] border-r border-[#E5E7EB] dark:border-[#333336] relative">
        
        {/* Active Session Top Bar */}
        <div className="h-[56px] min-h-[56px] px-6 border-b border-[#E5E7EB] dark:border-[#333336] flex items-center justify-between bg-[#FFFFFF] dark:bg-[#161617] z-20 transition-colors">
          
          {/* Left Title & Project Badges */}
          <div className="flex items-center gap-3 min-w-0 max-w-[70%]">
            <h2 className="font-semibold text-[17px] text-[#111827] dark:text-[#F2F2F2] truncate max-w-[340px] select-text">
              {activeSession.title || 'Active Task Session'}
            </h2>

            {/* Project Folder Pill */}
            <div className="flex items-center gap-1.5 px-3 py-1.5 rounded-[8px] bg-[#F3F4F6] dark:bg-[#2A2A2D] text-[13px] font-['JetBrains_Mono',system-ui,sans-serif] text-[#111827] dark:text-[#F2F2F2] shrink-0 border border-[#E5E7EB] dark:border-[#333336]">
              <Folder className="w-3.5 h-3.5 text-[#6B7280] dark:text-[#9B9B9F]" />
              <span className="truncate max-w-[140px]">{currentProjectName}</span>
            </div>

            {/* Git Branch Pill */}
            <div className="flex items-center gap-1.5 px-3 py-1.5 rounded-[8px] bg-[#F3F4F6] dark:bg-[#2A2A2D] text-[13px] font-['JetBrains_Mono',system-ui,sans-serif] text-[#111827] dark:text-[#F2F2F2] shrink-0 border border-[#E5E7EB] dark:border-[#333336]">
              <GitBranch className="w-3.5 h-3.5 text-[#6B7280] dark:text-[#9B9B9F]" />
              <span>{gitBranch || 'main'}</span>
              <ChevronDown className="w-3 h-3 text-[#6B7280] dark:text-[#9B9B9F]" />
            </div>

            {/* More Options Menu */}
            <div className="relative">
              <button
                type="button"
                onClick={() => setIsMenuOpen(prev => !prev)}
                className="p-1.5 rounded-[6px] text-[#6B7280] dark:text-[#9B9B9F] hover:text-[#111827] dark:hover:text-[#F2F2F2] hover:bg-[#F3F4F6] dark:hover:bg-[#2A2A2D] transition-colors cursor-pointer"
                title="More options"
              >
                <MoreHorizontal className="w-4 h-4" />
              </button>

              {isMenuOpen && (
                <div className="absolute left-0 top-full mt-1.5 w-44 rounded-[10px] bg-white dark:bg-[#1E1E20] border border-[#E5E7EB] dark:border-[#333336] shadow-2xl py-1 z-50 text-xs text-[#111827] dark:text-[#F2F2F2]">
                  <button
                    type="button"
                    onClick={() => {
                      setIsMenuOpen(false);
                      if (activeSessionId) deleteSessionPermanently(activeSessionId);
                    }}
                    className="w-full text-left px-3.5 py-2 hover:bg-[#FEE2E2] dark:hover:bg-[#2A2A2D] text-[#DC2626] dark:text-[#EF4444] cursor-pointer transition-colors"
                  >
                    Delete Session
                  </button>
                </div>
              )}
            </div>
          </div>

          {/* Right Action Icons */}
          <div className="flex items-center gap-2 text-[#6B7280] dark:text-[#9B9B9F]">
            {/* Status Indicator */}
            {activeSession.status === 'running' ? (
              <div className="flex items-center gap-2 px-3 py-1 rounded-full bg-[#FEF3C7] dark:bg-[#2A2A2D] border border-[#FDE68A] dark:border-[#333336] text-[12px] text-[#D97706] dark:text-[#F5A623]">
                <Loader2 className="w-3.5 h-3.5 animate-spin text-[#D97706] dark:text-[#F5A623]" />
                <span>Working...</span>
              </div>
            ) : (
              <div className="flex items-center gap-1.5 px-3 py-1 rounded-full bg-white dark:bg-[#1E1E20] border border-[#E5E7EB] dark:border-[#333336] text-[12px] text-[#16A34A] dark:text-[#4ADE80]">
                <CheckCircle2 className="w-3.5 h-3.5 text-[#16A34A] dark:text-[#4ADE80]" />
                <span className="text-[#6B7280] dark:text-[#9B9B9F]">Ready</span>
              </div>
            )}

            <button
              type="button"
              onClick={() => setIsRightActionDrawerOpen(!isRightActionDrawerOpen)}
              className={`p-2 rounded-[8px] transition-colors cursor-pointer ${
                isRightActionDrawerOpen 
                  ? 'text-[#111827] dark:text-[#F2F2F2] bg-[#EAEBED] dark:bg-[#2A2A2D]' 
                  : 'hover:bg-[#F3F4F6] dark:hover:bg-[#2A2A2D] text-[#6B7280] dark:text-[#9B9B9F] hover:text-[#111827] dark:hover:text-[#F2F2F2]'
              }`}
              title="Toggle Findings Panel"
            >
              <PanelRight className="w-4 h-4" />
            </button>
          </div>

        </div>

        {/* Turn Scrubber / Timeline Tracker (Floating on left edge) */}
        {conversationTurns.length > 0 && (
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
                        ? 'w-5 bg-[#111827] dark:bg-[#F2F2F2] shadow-xs'
                        : 'w-3.5 bg-[#9CA3AF] dark:bg-[#6B6B70] hover:w-5 hover:bg-[#4B5563] dark:hover:bg-[#9B9B9F]'
                    }`}
                    title={`Turn ${idx + 1}: ${turn.userPrompt.slice(0, 40)}`}
                  />

                  {/* Hover Preview Card Popup */}
                  {isHovered && (
                    <div 
                      className="absolute left-8 top-1/2 -translate-y-1/2 w-72 rounded-[10px] bg-white dark:bg-[#1E1E20] border border-[#E5E7EB] dark:border-[#333336] p-3.5 shadow-2xl z-50 text-left pointer-events-none animate-in fade-in zoom-in-95 duration-100"
                    >
                      <div className="text-[13px] font-semibold text-[#111827] dark:text-[#F2F2F2] line-clamp-2 leading-snug">
                        {turn.userPrompt}
                      </div>
                      {turn.agentPreview && (
                        <div className="text-[12px] text-[#6B7280] dark:text-[#9B9B9F] line-clamp-3 leading-relaxed mt-1.5 font-normal">
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
          className="flex-1 overflow-y-auto overflow-x-hidden pt-6 pb-6 px-6 sm:px-10 md:px-14 lg:px-20 space-y-6 select-text relative w-full min-w-0"
        >
          <div className="max-w-4xl mx-auto w-full min-w-0 space-y-6">
            {activeSession.messages.length === 0 ? (
              <div className="py-24 text-center text-[#9CA3AF] dark:text-[#6B6B70] space-y-3">
                <Sparkles className="w-9 h-9 text-[#9CA3AF] dark:text-[#6B6B70] mx-auto opacity-70" />
                <p className="font-medium text-sm text-[#4B5563] dark:text-[#9B9B9F]">Ready for instructions</p>
                <p className="text-xs text-[#6B7280] dark:text-[#6B6B70] max-w-sm mx-auto">
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
                  
                  {/* USER MESSAGE BUBBLE */}
                  {isUser && (() => {
                    const { text, attachments } = parseUserMessage(msg.content);
                    return (
                      <div className="w-full max-w-full min-w-0 bg-[#FFFFFF] dark:bg-[#1E1E20] border border-[#E5E7EB] dark:border-[#333336] rounded-[12px] p-[18px_20px] transition-all shadow-2xs space-y-3 overflow-hidden">
                        {text && (
                          <p className="text-[15px]/[24px] text-[#111827] dark:text-[#F2F2F2] font-normal whitespace-pre-wrap break-words [overflow-wrap:anywhere] select-text min-w-0">
                            {text}
                          </p>
                        )}
                        {attachments.length > 0 && (
                          <div className="flex flex-wrap gap-2 pt-1 border-t border-[#F3F4F6] dark:border-[#2A2A2D] first:border-t-0 first:pt-0 max-w-full min-w-0">
                            {attachments.map((att, i) => (
                              <div 
                                key={i} 
                                className="flex items-center gap-2 px-3 py-1.5 rounded-[8px] bg-[#F3F4F6] dark:bg-[#2A2A2D] text-[13px] font-['JetBrains_Mono',system-ui,sans-serif] text-[#111827] dark:text-[#F2F2F2] border border-[#E5E7EB] dark:border-[#333336] max-w-full min-w-0"
                                title={att.path}
                              >
                                {att.type === 'image' ? (
                                  <ImageIcon className="w-4 h-4 text-[#3B82F6] dark:text-[#60A5FA] shrink-0" />
                                ) : (
                                  <FileCode className="w-3.5 h-3.5 text-[#16A34A] dark:text-[#4ADE80] shrink-0" />
                                )}
                                <span className="font-medium truncate max-w-[200px]">{att.name}</span>
                                {att.path && att.path !== att.name && (
                                  <span className="text-[11px] text-[#6B7280] dark:text-[#9B9B9F] truncate max-w-[160px]">
                                    {att.path}
                                  </span>
                                )}
                              </div>
                            ))}
                          </div>
                        )}
                      </div>
                    );
                  })()}

                  {/* MODEL DIVIDER */}
                  {isUser && index < activeSession.messages.length - 1 && (
                    <div className="w-full max-w-full min-w-0 flex items-center gap-3 my-3">
                      <div className="flex-1 h-[1px] bg-[#E5E7EB] dark:bg-[#333336]"></div>
                      <div className="text-[13px] font-['JetBrains_Mono',system-ui,sans-serif] text-[#6B7280] dark:text-[#6B6B70]">
                        Using {activeSession.model || currentModel || 'forge-agent'}
                      </div>
                      <div className="flex-1 h-[1px] bg-[#E5E7EB] dark:bg-[#333336]"></div>
                    </div>
                  )}

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

                          if (!hasThoughts && !hasExplore && !hasCommands && !showThinkingPlaceholder) {
                            return null;
                          }

                          return (
                            <div key={`turn-grp-${msg.id || index}-${turnGroup.turn}`} className="w-full max-w-full min-w-0 space-y-3">
                              {/* 1. THOUGHTS IN THIS TURN */}
                              {hasThoughts ? (
                                <div className="w-full max-w-full min-w-0 space-y-2">
                                  {turnGroup.thoughts.map(th => {
                                    const isOpen = expandedThoughts[th.id] !== undefined ? expandedThoughts[th.id] : true;
                                    return (
                                      <div key={th.id} className="w-full max-w-full min-w-0">
                                        <button
                                          type="button"
                                          onClick={() => toggleThought(th.id)}
                                          className="flex items-center gap-2 text-[15px] text-[#4B5563] dark:text-[#9B9B9F] hover:text-[#111827] dark:hover:text-[#F2F2F2] transition-colors cursor-pointer py-1 group w-full max-w-full min-w-0"
                                        >
                                          <Brain className="w-4 h-4 text-[#6B7280] dark:text-[#9B9B9F] shrink-0" />
                                          <span className="font-bold text-[#4B5563] dark:text-[#9B9B9F]">Thought</span>
                                          <span className="text-[#9CA3AF] dark:text-[#6B6B70]">·</span>
                                          <span className="text-[14px] text-[#6B7280] dark:text-[#6B6B70]">
                                            {th.durationSeconds ? `${th.durationSeconds}s` : 'a few seconds ago'}
                                          </span>
                                          {turnGroup.isThinking && (
                                            <span className="text-[12px] text-[#D97706] dark:text-[#F5A623] animate-pulse font-mono">
                                              streaming...
                                            </span>
                                          )}
                                          {isOpen ? (
                                            <ChevronDown className="w-3.5 h-3.5 text-[#9CA3AF] dark:text-[#6B6B70] group-hover:text-[#111827] dark:group-hover:text-[#9B9B9F] transition-colors" />
                                          ) : (
                                            <ChevronRight className="w-3.5 h-3.5 text-[#9CA3AF] dark:text-[#6B6B70] group-hover:text-[#111827] dark:group-hover:text-[#9B9B9F] transition-colors" />
                                          )}
                                        </button>

                                        {isOpen && (
                                          <div className="border-l-2 border-[#E5E7EB] dark:border-[#333336] pl-4 py-1.5 my-1 text-[15px]/[24px] text-[#4B5563] dark:text-[#9B9B9F] select-text max-h-[280px] overflow-y-auto overflow-x-hidden pr-2 scrollbar-thin w-full max-w-full min-w-0 break-words [overflow-wrap:anywhere]">
                                            <MarkdownRenderer content={th.thoughtText} />
                                          </div>
                                        )}
                                      </div>
                                    );
                                  })}
                                </div>
                              ) : showThinkingPlaceholder ? (
                                <div className="flex items-center gap-2 text-[15px] text-[#D97706] dark:text-[#F5A623] py-1">
                                  <Brain className="w-4 h-4 text-[#D97706] dark:text-[#F5A623] animate-pulse shrink-0" />
                                  <span className="font-bold">Thinking...</span>
                                  <span className="text-[#9CA3AF] dark:text-[#6B6B70]">·</span>
                                  <span className="text-[14px] text-[#6B7280] dark:text-[#6B6B70]">streaming</span>
                                </div>
                              ) : null}

                              {/* 2. EXPLORE TOOLS IN THIS TURN */}
                              {hasExplore && (
                                <div className="w-full max-w-full min-w-0">
                                  {(() => {
                                    const exploreId = `explore-${msg.id || index}-${turnGroup.turn}`;
                                    const isExpOpen = expandedExplore[exploreId] !== undefined ? expandedExplore[exploreId] : true;

                                    return (
                                      <div className="w-full max-w-full min-w-0">
                                        <button
                                          type="button"
                                          onClick={() => toggleExplore(exploreId)}
                                          className="flex items-center gap-2 text-[15px] text-[#4B5563] dark:text-[#9B9B9F] hover:text-[#111827] dark:hover:text-[#F2F2F2] transition-colors cursor-pointer py-1 group w-full max-w-full min-w-0"
                                        >
                                          <Search className="w-4 h-4 text-[#6B7280] dark:text-[#9B9B9F] shrink-0" />
                                          <span className="font-bold text-[#4B5563] dark:text-[#9B9B9F]">Explore</span>
                                          <span className="text-[#9CA3AF] dark:text-[#6B6B70]">·</span>
                                          <span className="text-[14px] text-[#6B7280] dark:text-[#6B6B70]">
                                            {turnGroup.exploreTools.length} {turnGroup.exploreTools.length === 1 ? 'file' : 'files'}
                                          </span>
                                          {isExpOpen ? (
                                            <ChevronDown className="w-3.5 h-3.5 text-[#9CA3AF] dark:text-[#6B6B70] group-hover:text-[#111827] dark:group-hover:text-[#9B9B9F] transition-colors" />
                                          ) : (
                                            <ChevronRight className="w-3.5 h-3.5 text-[#9CA3AF] dark:text-[#6B6B70] group-hover:text-[#111827] dark:group-hover:text-[#9B9B9F] transition-colors" />
                                          )}
                                        </button>

                                        {isExpOpen && (
                                          <div className="border-l-2 border-[#E5E7EB] dark:border-[#333336] pl-4 py-2 my-1 flex flex-col gap-2.5 w-full max-w-full min-w-0 overflow-hidden">
                                            {turnGroup.exploreTools.map(t => {
                                              const info = extractExploreInfo(t, activeSession.workspacePath);

                                              return (
                                                <div key={t.id} className="flex items-center gap-2.5 text-[14px] w-full max-w-full min-w-0">
                                                  <span className="text-[14px] text-[#6B7280] dark:text-[#6B6B70] w-12 shrink-0">
                                                    {info.action}
                                                  </span>
                                                  <FileCode className="w-3.5 h-3.5 text-[#16A34A] dark:text-[#4ADE80] shrink-0" />
                                                  <span className="font-['JetBrains_Mono',system-ui,sans-serif] text-[14px] text-[#111827] dark:text-[#F2F2F2] font-medium truncate max-w-[220px] shrink-0" title={info.fullPath}>
                                                    {info.filename}
                                                  </span>
                                                  {info.dir && (
                                                    <span className="font-['JetBrains_Mono',system-ui,sans-serif] text-[13px] text-[#6B7280] dark:text-[#6B6B70] truncate min-w-0 flex-1" title={info.fullPath}>
                                                      {info.dir}/
                                                    </span>
                                                  )}
                                                  {t.status === 'running' && (
                                                    <Loader2 className="w-3 h-3 animate-spin text-[#D97706] dark:text-[#F5A623] shrink-0 ml-1" />
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
                              {hasCommands && (
                                <div className="w-full max-w-full min-w-0 space-y-2">
                                  {turnGroup.commandTools.map(t => {
                                    const isToolOpen = expandedTools[t.id] || false;
                                    const isDone = t.status === 'completed';
                                    const isRunning = t.status === 'running';
                                    const isFailed = t.status === 'failed';

                                    return (
                                      <div key={t.id} className="w-full max-w-full min-w-0">
                                        <div className="flex items-center gap-2.5 py-1 w-full max-w-full min-w-0">
                                          <SquareTerminal className="w-4 h-4 text-[#6B7280] dark:text-[#9B9B9F] shrink-0" />
                                          <span className="text-[15px] font-bold text-[#4B5563] dark:text-[#9B9B9F] shrink-0">Terminal</span>
                                          <div className="text-[13px] font-['JetBrains_Mono',system-ui,sans-serif] text-[#6B7280] dark:text-[#6B6B70] truncate flex-1 select-text min-w-0">
                                            {t.command || t.toolName}
                                          </div>

                                          {isRunning && (
                                            <div className="flex items-center gap-1 text-[12px] text-[#D97706] dark:text-[#F5A623] shrink-0">
                                              <Loader2 className="w-3.5 h-3.5 animate-spin" />
                                            </div>
                                          )}
                                          {isDone && (
                                            <div className="flex items-center gap-1 text-[12px] text-[#16A34A] dark:text-[#4ADE80] shrink-0">
                                              <CheckCircle2 className="w-3.5 h-3.5 text-[#16A34A] dark:text-[#4ADE80]" />
                                            </div>
                                          )}
                                          {isFailed && (
                                            <div className="flex items-center gap-1 text-[12px] text-[#DC2626] dark:text-[#EF4444] shrink-0">
                                              <AlertCircle className="w-3.5 h-3.5 text-[#DC2626] dark:text-[#EF4444]" />
                                            </div>
                                          )}

                                          {t.output && (
                                            <button
                                              type="button"
                                              onClick={() => toggleTool(t.id)}
                                              className="text-[#9CA3AF] dark:text-[#6B6B70] hover:text-[#111827] dark:hover:text-[#9B9B9F] p-1 rounded cursor-pointer transition-colors"
                                              title="Toggle console output"
                                            >
                                              {isToolOpen ? (
                                                <ChevronDown className="w-3.5 h-3.5" />
                                              ) : (
                                                <ChevronRight className="w-3.5 h-3.5" />
                                              )}
                                            </button>
                                          )}
                                        </div>

                                        {isToolOpen && t.output && (
                                          <div className="mt-1 ml-4 sm:ml-6 p-3 rounded-[8px] bg-[#F3F4F6] dark:bg-[#0E0E10] border border-[#E5E7EB] dark:border-[#333336] text-[12px] font-['JetBrains_Mono',monospace] text-[#111827] dark:text-[#CCCCCC] leading-relaxed max-h-60 max-w-[calc(100%-1rem)] sm:max-w-[calc(100%-1.5rem)] overflow-y-auto overflow-x-auto whitespace-pre-wrap break-words [overflow-wrap:anywhere] select-text">
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

                      {/* 4. CHANGED FILES CARD */}
                      {sessionDiffs.length > 0 && (
                        <div className="w-full max-w-xl min-w-0 rounded-[12px] bg-[#FFFFFF] dark:bg-[#1E1E20] border border-[#E5E7EB] dark:border-[#333336] overflow-hidden my-2 shadow-2xs">
                          <div className="px-4 py-3 border-b border-[#E5E7EB] dark:border-[#333336] flex items-center justify-between text-xs bg-[#F9FAFB] dark:bg-[#1E1E20]">
                            <div className="flex items-center gap-2 font-medium text-[#111827] dark:text-[#F2F2F2]">
                              <Code2 className="w-4 h-4 text-[#16A34A] dark:text-[#4ADE80]" />
                              <span>{sessionDiffs.length} files changed</span>
                              <span className="font-['JetBrains_Mono',system-ui,sans-serif] text-[12px]">
                                <span className="text-[#16A34A] dark:text-[#4ADE80]">+{totalAdditions}</span>{' '}
                                <span className="text-[#DC2626] dark:text-[#EF4444]">-{totalDeletions}</span>
                              </span>
                            </div>

                            <button
                              type="button"
                              onClick={() => setActiveInlineDiff(sessionDiffs[0])}
                              className="text-[12px] text-[#6B7280] dark:text-[#9B9B9F] hover:text-[#111827] dark:hover:text-[#F2F2F2] transition-colors cursor-pointer"
                            >
                              Review all
                            </button>
                          </div>

                          <div className="p-2 space-y-1">
                            {sessionDiffs.slice(0, 5).map(d => {
                              const parts = d.filePath.split('/');
                              const fname = parts.pop() || d.fileName;
                              const dir = parts.join('/') || 'root';

                              return (
                                <div
                                  key={d.id}
                                  className="flex items-center justify-between p-2 rounded-[8px] hover:bg-[#F3F4F6] dark:hover:bg-[#2A2A2D] transition-colors group cursor-pointer"
                                  onClick={() => setActiveInlineDiff(d)}
                                >
                                  <div className="flex items-center gap-2 min-w-0 flex-1 pr-2">
                                    <FileCode className="w-3.5 h-3.5 text-[#16A34A] dark:text-[#4ADE80] shrink-0" />
                                    <span className="text-xs font-medium text-[#111827] dark:text-[#F2F2F2] font-['JetBrains_Mono',system-ui,sans-serif] truncate">
                                      {fname}
                                    </span>
                                    <span className="text-[11px] text-[#6B7280] dark:text-[#6B6B70] font-['JetBrains_Mono',system-ui,sans-serif] truncate">
                                      {dir}
                                    </span>
                                  </div>

                                  <div className="flex items-center gap-2 shrink-0">
                                    <span className="font-['JetBrains_Mono',system-ui,sans-serif] text-[11px]">
                                      <span className="text-[#16A34A] dark:text-[#4ADE80]">+{d.additions}</span>{' '}
                                      <span className="text-[#DC2626] dark:text-[#EF4444]">-{d.deletions}</span>
                                    </span>
                                    <button
                                      type="button"
                                      onClick={(e) => {
                                        e.stopPropagation();
                                        openDiffInEditor(d);
                                      }}
                                      className="px-2 py-0.5 rounded-[5px] bg-[#F3F4F6] dark:bg-[#2A2A2D] text-[#6B7280] dark:text-[#9B9B9F] hover:text-[#111827] dark:hover:text-[#F2F2F2] hover:bg-[#E5E7EB] dark:hover:bg-[#333336] text-[11px] transition-colors"
                                    >
                                      Open
                                    </button>
                                  </div>
                                </div>
                              );
                            })}
                          </div>
                        </div>
                      )}

                      {/* 5. AGENT RESPONSE MARKDOWN */}
                      {msg.content && (
                        <div className="text-[16px]/[25px] text-[#111827] dark:text-[#F2F2F2] font-[Inter,system-ui,sans-serif] select-text py-1 w-full max-w-full min-w-0 break-words [overflow-wrap:anywhere] leading-relaxed">
                          <MarkdownRenderer content={msg.content} />
                        </div>
                      )}

                      {/* 6. ACTION FOOTER */}
                      <div className="w-full max-w-full min-w-0 flex items-center gap-4 pt-1 text-[#6B7280] dark:text-[#9B9B9F]">
                        <button
                          type="button"
                          onClick={() => handleCopy(msg.content, msg.id)}
                          className="hover:text-[#111827] dark:hover:text-[#F2F2F2] transition-colors cursor-pointer p-1"
                          title="Copy response"
                        >
                          {isCopied ? (
                            <Check className="w-4 h-4 text-[#16A34A] dark:text-[#4ADE80]" />
                          ) : (
                            <Copy className="w-4 h-4" />
                          )}
                        </button>

                        <button
                          type="button"
                          onClick={() => handleThumb(msg.id, 'up')}
                          className={`transition-colors cursor-pointer p-1 ${
                            isMsgLiked === 'up' ? 'text-[#16A34A] dark:text-[#4ADE80]' : 'hover:text-[#111827] dark:hover:text-[#F2F2F2]'
                          }`}
                          title="Good response"
                        >
                          <ThumbsUp className="w-4 h-4" />
                        </button>

                        <button
                          type="button"
                          onClick={() => handleThumb(msg.id, 'down')}
                          className={`transition-colors cursor-pointer p-1 ${
                            isMsgLiked === 'down' ? 'text-[#DC2626] dark:text-[#EF4444]' : 'hover:text-[#111827] dark:hover:text-[#F2F2F2]'
                          }`}
                          title="Bad response"
                        >
                          <ThumbsDown className="w-4 h-4" />
                        </button>

                        <div className="text-[13px] font-['JetBrains_Mono',system-ui,sans-serif] text-[#9CA3AF] dark:text-[#6B6B70]">
                          {msg.timestamp || 'now'}
                        </div>
                      </div>

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

        {/* Bottom Follow-up Input Bar */}
        <div className="pt-2 pb-5 px-6 sm:px-10 md:px-14 lg:px-20 bg-[#F8F9FA] dark:bg-[#161617] transition-colors w-full min-w-0">
          <div className="max-w-4xl mx-auto w-full min-w-0">
            <AgentTaskInputBar
              placeholder="Ask for follow-up changes"
              autoFocus={true}
              isCompact={true}
            />
          </div>
        </div>

      </div>

      {/* Right Sidebar (Findings Panel / Review / Terminal / Side Chat) */}
      {isRightActionDrawerOpen && (
        <AgentRightSidebar 
          onClose={() => setIsRightActionDrawerOpen(false)}
          onOpenDiff={(d) => setActiveInlineDiff(d)}
        />
      )}

      {/* Inline Diff Overlay Modal if clicked */}
      {activeInlineDiff && (
        <div className="absolute inset-0 z-50 flex flex-col bg-[#F8F9FA] dark:bg-[#161617] animate-in fade-in duration-100">
          <div className="h-[48px] px-6 bg-[#FFFFFF] dark:bg-[#1E1E20] border-b border-[#E5E7EB] dark:border-[#333336] flex items-center justify-between">
            <div className="flex items-center gap-2 text-sm font-semibold text-[#111827] dark:text-[#F2F2F2]">
              <Code2 className="w-4 h-4 text-[#16A34A] dark:text-[#4ADE80]" />
              <span>Reviewing changes: {activeInlineDiff.fileName}</span>
              <span className="font-['JetBrains_Mono',system-ui,sans-serif] text-xs text-[#16A34A] dark:text-[#4ADE80]">+{activeInlineDiff.additions}</span>
              <span className="font-['JetBrains_Mono',system-ui,sans-serif] text-xs text-[#DC2626] dark:text-[#EF4444]">-{activeInlineDiff.deletions}</span>
            </div>

            <div className="flex items-center gap-3">
              <button
                type="button"
                onClick={() => {
                  openDiffInEditor(activeInlineDiff);
                  setActiveInlineDiff(null);
                }}
                className="px-3 py-1.5 rounded-[8px] bg-[#F3F4F6] dark:bg-[#2A2A2D] hover:bg-[#E5E7EB] dark:hover:bg-[#333336] text-[#111827] dark:text-[#F2F2F2] text-xs font-medium flex items-center gap-1.5 transition-colors cursor-pointer"
              >
                <ExternalLink className="w-3.5 h-3.5" />
                <span>Open in editor</span>
              </button>
              <button
                type="button"
                onClick={() => setActiveInlineDiff(null)}
                className="p-1.5 rounded-[6px] hover:bg-[#F3F4F6] dark:hover:bg-[#2A2A2D] text-[#6B7280] dark:text-[#9B9B9F] hover:text-[#111827] dark:hover:text-[#F2F2F2] transition-colors cursor-pointer"
              >
                <X className="w-4 h-4" />
              </button>
            </div>
          </div>

          <div className="flex-1 min-h-0 p-4 flex flex-col overflow-hidden bg-white dark:bg-[#121213]">
            <DiffViewer diff={activeInlineDiff} onClose={() => setActiveInlineDiff(null)} isInline={true} />
          </div>
        </div>
      )}

    </div>
  );
};
