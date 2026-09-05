import React, { useState, useRef, useEffect, useMemo } from 'react';
import { 
  Plus, 
  X, 
  Brain, 
  Terminal, 
  ChevronDown, 
  ChevronRight, 
  FileText, 
  Settings, 
  GitCompare, 
  Square, 
  ArrowUp, 
  Mic, 
  Sparkles, 
  Laptop, 
  Folder, 
  PanelRightClose, 
  PanelRight,
  ExternalLink,
  CheckCircle2,
  Loader2,
  Code2,
  Copy,
  Check,
  GitBranch,
  BarChart2,
  Activity,
  Zap,
  Clock,
  Cpu,
  MessageSquare,
  Trash2
} from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { AgentSession } from '../../types';
import { AgentInputBar } from './AgentInputBar';
import { MarkdownRenderer } from './MarkdownRenderer';

export const AgentActiveSessionView: React.FC = () => {
  const { 
    activeSession, 
    activeSessionId, 
    sessions, 
    setActiveSessionId, 
    createNewSession, 
    deleteSession,
    sendAgentPrompt, 
    stopAgentExecution, 
    openFileInEditor, 
    openSettingsTab,
    openDiffInEditor,
    diffs,
    isRightActionDrawerOpen, 
    setIsRightActionDrawerOpen,
    setIsCommandPaletteOpen,
    savedSessions,
    openSessionFromHistory,
    deleteSessionPermanently,
    activeWorkspacePath,
    setMode,
    currentModel
  } = useWorkspace();

  const [copiedIndex, setCopiedIndex] = useState<number | null>(null);
  const [activeAnalyticsMsgId, setActiveAnalyticsMsgId] = useState<string | null>(null);

  const allHistorySessions = useMemo(() => {
    const map = new Map<string, AgentSession>();
    for (const s of savedSessions) {
      if (s && s.id) map.set(s.id, s);
    }
    for (const s of sessions) {
      if (s && s.id) map.set(s.id, s);
    }
    return Array.from(map.values()).sort((a, b) => {
      return (b.id || '').localeCompare(a.id || '');
    });
  }, [savedSessions, sessions]);

  const [expandedThoughts, setExpandedThoughts] = useState<Record<string, boolean>>({
    't-1': false,
    't-2': false,
    'subtasks-1': true
  });
  const streamEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    streamEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [activeSession?.messages]);

  if (!activeSession) return null;

  const toggleThought = (id: string, defaultOpen: boolean = false) => {
    setExpandedThoughts(prev => {
      const current = prev[id] !== undefined ? prev[id] : defaultOpen;
      return { ...prev, [id]: !current };
    });
  };

  const handleCopyMessage = (content: string, index: number) => {
    navigator.clipboard.writeText(content);
    setCopiedIndex(index);
    setTimeout(() => setCopiedIndex(null), 1500);
  };

  return (
    <div className="flex-1 flex h-[calc(100vh-68px)] overflow-hidden bg-white dark:bg-[#181818] select-text">
      
      {/* Left Chat / Step Stream Panel (60–65% width) */}
      <div className="flex-1 flex flex-col h-full border-r border-[#e5e7eb] dark:border-[#2b2b2b] overflow-hidden bg-white dark:bg-[#181818]">
        
        {/* Session Tab Bar (Image 1 replica) */}
        <div className="h-[38px] min-h-[38px] bg-[#fafafa] dark:bg-[#1f1f20] border-b border-[#e5e7eb] dark:border-[#2b2b2b] flex items-center justify-between px-2.5">
          <div className="flex items-center gap-1 overflow-x-auto">
            {sessions.map(s => {
              const isActive = s.id === activeSessionId;
              return (
                <div
                  key={s.id}
                  onClick={() => setActiveSessionId(s.id)}
                  className={`flex items-center gap-2 px-3 py-1.5 rounded-t-lg text-xs font-medium cursor-pointer transition-all border-b-2 ${
                    isActive
                      ? 'bg-white dark:bg-[#181818] border-[#2563eb] dark:border-[#60a5fa] text-[#111827] dark:text-white shadow-2xs font-semibold'
                      : 'border-transparent text-[#6b7280] dark:text-[#9ca3af] hover:text-[#111827] dark:hover:text-white hover:bg-black/5 dark:hover:bg-white/5'
                  }`}
                >
                  {/* Hexagon icon */}
                  <svg viewBox="0 0 100 100" className="w-3.5 h-3.5 fill-current text-[#4f46e5]">
                    <polygon points="50,15 75,29 75,57 50,71 25,57 25,29" />
                  </svg>
                  <span className="truncate max-w-[140px]">{s.title}</span>
                  <button
                    type="button"
                    onClick={(e) => {
                      e.stopPropagation();
                      deleteSession(s.id);
                    }}
                    className="p-0.5 hover:bg-[#e5e7eb] dark:hover:bg-[#333336] rounded text-[#9ca3af] hover:text-[#111827] dark:hover:text-white transition-colors"
                  >
                    <X className="w-3 h-3" />
                  </button>
                </div>
              );
            })}

            <button
              type="button"
              onClick={() => createNewSession()}
              className="p-1.5 hover:bg-[#e5e7eb] dark:hover:bg-[#333336] rounded-md text-[#6b7280] dark:text-[#9ca3af] hover:text-[#111827] dark:hover:text-white transition-colors ml-1"
              title="New Session (+)"
            >
              <Plus className="w-3.5 h-3.5" />
            </button>
          </div>

          {/* Toggle right drawer icon */}
          <button
            type="button"
            onClick={() => setIsRightActionDrawerOpen(prev => !prev)}
            className="p-1 hover:bg-[#e5e7eb] dark:hover:bg-[#333336] rounded text-[#6b7280] dark:text-[#9ca3af] hover:text-[#111827] dark:hover:text-white transition-colors"
            title="Toggle Action Drawer"
          >
            {isRightActionDrawerOpen ? (
              <PanelRightClose className="w-4 h-4" />
            ) : (
              <PanelRight className="w-4 h-4" />
            )}
          </button>
        </div>

        {/* Message / Thought / Tool Stream Body (Image 1 replica) */}
        <div className="flex-1 overflow-y-auto px-6 py-6 space-y-6">
          {activeSession.messages.map((msg, idx) => (
            <div key={msg.id || idx} className="space-y-3 animate-in fade-in duration-150">
              
              {/* User Bubble (Right-aligned, clean pill) */}
              {msg.role === 'user' && (
                <div className="flex justify-end">
                  <div className="bg-[#f3f4f6] dark:bg-[#262628] text-[#111827] dark:text-[#f3f4f6] px-4 py-2 rounded-2xl max-w-[80%] text-[13px] font-normal leading-relaxed shadow-2xs border border-[#e5e7eb]/60 dark:border-[#383838]">
                    {msg.content}
                  </div>
                </div>
              )}

              {/* Agent Message / Reasoning Stream */}
              {msg.role === 'agent' && (
                <div className="space-y-4 max-w-2xl">
                  
                  {/* Thought Telemetry Block(s) with Markdown Viewer - Auto Opens while thinking, auto collapses when done */}
                  {msg.thoughts?.map((thought) => {
                    const isCurrentlyThinking = msg.isThinking === true || (!msg.content && activeSession.status === 'running');
                    const isExpanded = expandedThoughts[thought.id] !== undefined
                      ? expandedThoughts[thought.id]
                      : isCurrentlyThinking;

                    return (
                      <div key={thought.id} className="space-y-1.5">
                        <button
                          type="button"
                          onClick={() => toggleThought(thought.id, isCurrentlyThinking)}
                          className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full bg-[#f3f4f6] dark:bg-[#242426] hover:bg-[#e5e7eb] dark:hover:bg-[#2e2e30] text-xs font-medium text-[#4b5563] dark:text-[#9ca3af] transition-colors cursor-pointer"
                        >
                          <Brain className="w-3.5 h-3.5 text-[#6366f1]" />
                          <span>Thought for {thought.durationSeconds}s</span>
                          {isExpanded ? (
                            <ChevronDown className="w-3.5 h-3.5 text-[#9ca3af]" />
                          ) : (
                            <ChevronRight className="w-3.5 h-3.5 text-[#9ca3af]" />
                          )}
                        </button>

                        {isExpanded && (
                          <div className="p-3 rounded-xl bg-[#f9fafb] dark:bg-[#1a1a1c] border border-[#e5e7eb] dark:border-[#2f2f31] text-xs text-[#4b5563] dark:text-[#a1a1aa] font-mono leading-relaxed">
                            <MarkdownRenderer content={thought.thoughtText} />
                          </div>
                        )}
                      </div>
                    );
                  })}

                  {/* Tool Invocation Card */}
                  {msg.toolExecutions?.map((tool) => (
                    <div
                      key={tool.id}
                      className="rounded-2xl border border-[#e5e7eb] dark:border-[#2f2f31] bg-white dark:bg-[#1e1e1e] shadow-2xs overflow-hidden"
                    >
                      {/* Tool Header Badge */}
                      <div className="px-3.5 py-2 bg-[#f9fafb] dark:bg-[#232325] border-b border-[#f0f0f2] dark:border-[#2b2b2b] flex items-center justify-between text-xs">
                        <div className="flex items-center gap-2 font-mono text-[#374151] dark:text-[#d1d5db]">
                          <Terminal className="w-3.5 h-3.5 text-[#3b82f6]" />
                          <span className="font-semibold">{tool.toolName}</span>
                        </div>
                        <span className="text-[10px] px-2 py-0.5 rounded-full bg-[#ecfdf5] dark:bg-[#064e3b] text-[#059669] dark:text-[#a7f3d0] font-mono font-medium">
                          {tool.status || 'completed'}
                        </span>
                      </div>

                        {/* Monospace Output Box */}
                        <div className="p-3 bg-white dark:bg-[#181818] font-mono text-[12px] space-y-2">
                          {tool.output && (
                            <div className="p-2.5 rounded-lg bg-[#fafafa] dark:bg-[#141414] border border-[#f0f0f2] dark:border-[#262626] text-[#4b5563] dark:text-[#a1a1aa] overflow-x-auto whitespace-pre leading-relaxed text-[11px]">
                              {tool.output}
                            </div>
                          )}

                          {tool.diff && (
                            <div className="pt-1 flex items-center justify-between">
                              <span className="text-[11px] text-[#6b7280] dark:text-[#94a3b8]">
                                Modified <code className="text-[#0f172a] dark:text-white font-semibold">{tool.diff.fileName}</code> (+{tool.diff.additions} -{tool.diff.deletions})
                              </span>
                              <button
                                type="button"
                                onClick={() => tool.diff && openDiffInEditor(tool.diff)}
                                className="px-2.5 py-1 rounded-lg bg-[#eff6ff] hover:bg-[#dbeafe] text-[#2563eb] dark:bg-[#1e293b] dark:hover:bg-[#27384e] dark:text-[#93c5fd] font-semibold text-xs flex items-center gap-1.5 transition-colors cursor-pointer"
                              >
                                <GitCompare className="w-3.5 h-3.5" />
                                <span>Review Diff</span>
                              </button>
                            </div>
                          )}
                        </div>

                        {/* Nested Sub-Tasks & File Reads */}
                        {tool.subtasks && tool.subtasks.length > 0 && (
                          <div className="border-t border-[#f0f0f2] dark:border-[#2b2b2b] p-3 bg-[#fafafa] dark:bg-[#1c1c1e] text-xs space-y-2">
                            <button
                              type="button"
                              onClick={() => toggleThought('subtasks-1')}
                              className="flex items-center gap-1 font-medium text-[#6b7280] dark:text-[#9ca3af] hover:text-[#111827] dark:hover:text-white transition-colors"
                            >
                              <ChevronDown className="w-3.5 h-3.5" />
                              <span>Thoughts</span>
                            </button>

                            <div className="pl-3 border-l-2 border-[#e5e7eb] dark:border-[#383838] space-y-1.5 text-xs">
                              {tool.subtasks.map((sub, sIdx) => {
                                const isFileLink = sub.startsWith('Read ');
                                const fileName = isFileLink ? sub.replace('Read ', '') : '';
                                return (
                                  <div
                                    key={sIdx}
                                    onClick={() => {
                                      if (isFileLink) {
                                        openFileInEditor(fileName.includes(' ') ? fileName.split(' ')[0] : fileName);
                                      }
                                    }}
                                    className={`flex items-center gap-2 py-0.5 ${
                                      isFileLink 
                                        ? 'text-[#2563eb] dark:text-[#60a5fa] hover:underline cursor-pointer font-medium' 
                                        : 'text-[#374151] dark:text-[#d1d5db]'
                                    }`}
                                  >
                                    {isFileLink ? (
                                      <FileText className="w-3.5 h-3.5 shrink-0" />
                                    ) : (
                                      <span className="w-1.5 h-1.5 rounded-full bg-[#9ca3af]" />
                                    )}
                                    <span>{sub}</span>
                                  </div>
                                );
                              })}
                            </div>
                          </div>
                        )}
                      </div>
                    ))}

                  {/* Formatted Agent Response Content with Markdown Viewer */}
                  {msg.content && (
                    <div className="space-y-2">
                      <MarkdownRenderer content={msg.content} />

                      {/* Action Reaction Toolbar */}
                      <div className="flex items-center gap-1.5 pt-2 text-[#9ca3af] select-none relative">
                        {/* Copy button */}
                        <button
                          type="button"
                          onClick={() => handleCopyMessage(msg.content, idx)}
                          className="p-1.5 rounded-lg hover:bg-[#f3f4f6] dark:hover:bg-[#252528] hover:text-[#111827] dark:hover:text-white transition-colors cursor-pointer"
                          title="Copy response"
                        >
                          {copiedIndex === idx ? <Check className="w-3.5 h-3.5 text-[#10b981]" /> : <Copy className="w-3.5 h-3.5" />}
                        </button>

                        {/* Branch conversation */}
                        <button
                          type="button"
                          className="p-1.5 rounded-lg hover:bg-[#f3f4f6] dark:hover:bg-[#252528] hover:text-[#111827] dark:hover:text-white transition-colors cursor-pointer"
                          title="Branch / Fork conversation"
                        >
                          <GitBranch className="w-3.5 h-3.5" />
                        </button>

                        {/* Telemetry / Analytics Stats */}
                        <div className="relative">
                          <button
                            type="button"
                            onClick={() => setActiveAnalyticsMsgId(prev => prev === msg.id ? null : msg.id)}
                            className={`p-1.5 rounded-lg transition-colors cursor-pointer ${
                              activeAnalyticsMsgId === msg.id 
                                ? 'bg-[#eff6ff] dark:bg-[#1e293b] text-[#2563eb] dark:text-[#38bdf8]' 
                                : 'hover:bg-[#f3f4f6] dark:hover:bg-[#252528] hover:text-[#111827] dark:hover:text-white'
                            }`}
                            title="View execution analytics (tokens in/out, timing, TPS)"
                          >
                            <BarChart2 className="w-3.5 h-3.5" />
                          </button>

                          {/* Analytics Popover Card */}
                          {activeAnalyticsMsgId === msg.id && (() => {
                            const prevMsg = idx > 0 ? activeSession.messages[idx - 1] : null;
                            const tokensIn = Math.max(12, Math.round((prevMsg?.content?.length || 40) / 4));
                            const tokensOut = Math.max(1, Math.round((msg.content?.length || 0) / 4));
                            const totalTokens = tokensIn + tokensOut;
                            const durationSec = msg.thoughts?.[0]?.durationSeconds || 1.35;
                            const tps = (tokensOut / durationSec).toFixed(1);

                            return (
                              <div className="absolute left-0 bottom-full mb-2 w-64 rounded-xl bg-[#0f172a] text-white p-3.5 shadow-2xl border border-[#334155] z-50 text-xs font-sans space-y-2.5 animate-in fade-in zoom-in-95">
                                <div className="flex items-center justify-between border-b border-[#334155] pb-1.5">
                                  <div className="flex items-center gap-1.5 font-semibold text-[#38bdf8]">
                                    <Activity className="w-3.5 h-3.5" />
                                    <span>Run Analytics</span>
                                  </div>
                                  <span className="text-[10px] px-1.5 py-0.5 rounded bg-[#1e293b] text-[#94a3b8] font-mono">
                                    {currentModel || 'My-ADE'}
                                  </span>
                                </div>

                                <div className="grid grid-cols-2 gap-2 text-[11px]">
                                  <div className="p-2 rounded-lg bg-[#1e293b]/70 border border-[#334155]/60 space-y-0.5">
                                    <span className="text-[#94a3b8] text-[10px] block">Input Tokens</span>
                                    <span className="font-mono font-bold text-white">{tokensIn.toLocaleString()}</span>
                                  </div>
                                  <div className="p-2 rounded-lg bg-[#1e293b]/70 border border-[#334155]/60 space-y-0.5">
                                    <span className="text-[#94a3b8] text-[10px] block">Output Tokens</span>
                                    <span className="font-mono font-bold text-[#10b981]">{tokensOut.toLocaleString()}</span>
                                  </div>
                                </div>

                                <div className="space-y-1 text-[11px] pt-0.5 border-t border-[#334155]/60">
                                  <div className="flex items-center justify-between text-[#94a3b8]">
                                    <div className="flex items-center gap-1">
                                      <Zap className="w-3 h-3 text-[#f59e0b]" />
                                      <span>Throughput (TPS):</span>
                                    </div>
                                    <span className="font-mono font-semibold text-white">{tps} tok/s</span>
                                  </div>
                                  <div className="flex items-center justify-between text-[#94a3b8]">
                                    <div className="flex items-center gap-1">
                                      <Clock className="w-3 h-3 text-[#a855f7]" />
                                      <span>Latency / Timing:</span>
                                    </div>
                                    <span className="font-mono font-semibold text-white">{durationSec.toFixed(2)}s</span>
                                  </div>
                                  <div className="flex items-center justify-between text-[#94a3b8]">
                                    <div className="flex items-center gap-1">
                                      <Cpu className="w-3 h-3 text-[#38bdf8]" />
                                      <span>Total Context:</span>
                                    </div>
                                    <span className="font-mono font-semibold text-white">{totalTokens.toLocaleString()} tokens</span>
                                  </div>
                                </div>
                              </div>
                            );
                          })()}
                        </div>
                      </div>

                    </div>
                  )}

                  {/* Animated Pulsing Thinking Indicator */}
                  {msg.isThinking && (
                    <div className="flex items-center gap-2 text-xs font-medium text-[#6b7280] dark:text-[#9ca3af] animate-pulse py-1">
                      <Loader2 className="w-3.5 h-3.5 animate-spin text-[#3b82f6]" />
                      <span>Thinking..</span>
                    </div>
                  )}

                </div>
              )}

            </div>
          ))}

          <div ref={streamEndRef} />
        </div>

        {/* Bottom Sticky Input Bar (Screenshot 1 replica) */}
        <div className="p-4 bg-white dark:bg-[#181818] border-t border-[#e5e7eb] dark:border-[#2b2b2b]">
          <AgentInputBar 
            onSendPrompt={sendAgentPrompt} 
            placeholder="Reply to My-ADE or ask for new changes..." 
          />
        </div>

      </div>

      {/* Right Quick-Action Drawer (35–40% width) (Screenshot 1 replica) */}
      {isRightActionDrawerOpen && (
        <div className="w-[320px] lg:w-[360px] bg-white dark:bg-[#1b1b1d] p-5 flex flex-col justify-between overflow-y-auto select-none border-l border-[#e5e7eb] dark:border-[#2b2b2b] animate-in slide-in-from-right-5 duration-150 font-sans">
          
          <div className="space-y-5">
            {/* Drawer top close button */}
            <div className="flex items-center justify-end pb-1">
              <button
                type="button"
                onClick={() => setIsRightActionDrawerOpen(false)}
                className="p-1 hover:bg-[#f3f4f6] dark:hover:bg-[#2a2a2c] rounded-lg text-[#9ca3af] hover:text-[#111827] dark:hover:text-white transition-colors"
                title="Close drawer"
              >
                <X className="w-4 h-4" />
              </button>
            </div>

            {/* Action Item Rows with Keyboard Shortcuts */}
            <div className="space-y-1.5">
              
              {/* + New session (⌥T) */}
              <button
                type="button"
                onClick={() => createNewSession()}
                className="w-full text-left p-3 rounded-xl hover:bg-[#f3f4f6] dark:hover:bg-[#252528] flex items-start justify-between transition-colors group cursor-pointer"
              >
                <div className="flex items-start gap-3">
                  <div className="p-1.5 rounded-lg bg-[#eff6ff] dark:bg-[#1e293b] text-[#2563eb] dark:text-[#60a5fa] group-hover:scale-105 transition-transform mt-0.5">
                    <Plus className="w-4 h-4" />
                  </div>
                  <div>
                    <p className="text-xs font-semibold text-[#111827] dark:text-white">
                      New session
                    </p>
                    <p className="text-[11px] text-[#6b7280] dark:text-[#9ca3af]">
                      Start a new session in this space
                    </p>
                  </div>
                </div>
                <kbd className="text-[10px] font-mono px-1.5 py-0.5 rounded bg-[#f3f4f6] dark:bg-[#2e2e32] text-[#6b7280] dark:text-[#9ca3af] border border-[#e5e7eb] dark:border-[#383838]">
                  ⌘ T
                </kbd>
              </button>

              {/* 📄 Open file (⌘P) */}
              <button
                type="button"
                onClick={() => setIsCommandPaletteOpen(true)}
                className="w-full text-left p-3 rounded-xl hover:bg-[#f3f4f6] dark:hover:bg-[#252528] flex items-start justify-between transition-colors group cursor-pointer"
              >
                <div className="flex items-start gap-3">
                  <div className="p-1.5 rounded-lg bg-[#fef3c7] dark:bg-[#2d2516] text-[#d97706] dark:text-[#fbbf24] group-hover:scale-105 transition-transform mt-0.5">
                    <FileText className="w-4 h-4" />
                  </div>
                  <div>
                    <p className="text-xs font-semibold text-[#111827] dark:text-white">
                      Open file
                    </p>
                    <p className="text-[11px] text-[#6b7280] dark:text-[#9ca3af]">
                      Open a file from the workspace
                    </p>
                  </div>
                </div>
                <kbd className="text-[10px] font-mono px-1.5 py-0.5 rounded bg-[#f3f4f6] dark:bg-[#2e2e32] text-[#6b7280] dark:text-[#9ca3af] border border-[#e5e7eb] dark:border-[#383838]">
                  ⌘ P
                </kbd>
              </button>

              {/* ⚙ Open customizations */}
              <button
                type="button"
                onClick={() => openSettingsTab('rules')}
                className="w-full text-left p-3 rounded-xl hover:bg-[#f3f4f6] dark:hover:bg-[#252528] flex items-start justify-between transition-colors group cursor-pointer"
              >
                <div className="flex items-start gap-3">
                  <div className="p-1.5 rounded-lg bg-[#f3f4f6] dark:bg-[#262628] text-[#4b5563] dark:text-[#d1d5db] group-hover:scale-105 transition-transform mt-0.5">
                    <Settings className="w-4 h-4" />
                  </div>
                  <div>
                    <p className="text-xs font-semibold text-[#111827] dark:text-white">
                      Open customizations
                    </p>
                    <p className="text-[11px] text-[#6b7280] dark:text-[#9ca3af]">
                      Manage rules, skills, MCPs and sub-agents
                    </p>
                  </div>
                </div>
              </button>

              {/* ± View diffs (only shown if any file changes exist) */}
              {diffs.length > 0 && (
                <button
                  type="button"
                  onClick={() => openDiffInEditor(diffs[0])}
                  className="w-full text-left p-3 rounded-xl hover:bg-[#f3f4f6] dark:hover:bg-[#252528] flex items-start justify-between transition-colors group cursor-pointer"
                >
                  <div className="flex items-start gap-3">
                    <div className="p-1.5 rounded-lg bg-[#ecfdf5] dark:bg-[#064e3b] text-[#059669] dark:text-[#a7f3d0] group-hover:scale-105 transition-transform mt-0.5">
                      <GitCompare className="w-4 h-4" />
                    </div>
                    <div>
                      <div className="flex items-center gap-1.5">
                        <p className="text-xs font-semibold text-[#111827] dark:text-white">
                          View diffs
                        </p>
                        {diffs.filter(d => d.status === 'pending').length > 0 && (
                          <span className="px-1.5 py-0.2 rounded-full bg-[#10b981] text-white text-[10px] font-mono font-bold">
                            {diffs.filter(d => d.status === 'pending').length}
                          </span>
                        )}
                      </div>
                      <p className="text-[11px] text-[#6b7280] dark:text-[#9ca3af]">
                        {diffs.length} file change(s) available
                      </p>
                    </div>
                  </div>
                </button>
              )}

            </div>

            {/* Recent Sessions History Section */}
            <div className="pt-3 border-t border-[#f3f4f6] dark:border-[#2b2b2b] space-y-2">
              <div className="flex items-center justify-between">
                <span className="text-[11px] font-semibold uppercase tracking-wider text-[#6b7280] dark:text-[#9ca3af]">
                  Recent Sessions ({allHistorySessions.length})
                </span>
                <button
                  type="button"
                  onClick={() => createNewSession()}
                  className="text-[11px] text-[#2563eb] dark:text-[#60a5fa] hover:underline font-medium flex items-center gap-1 cursor-pointer"
                >
                  <Plus className="w-3 h-3" />
                  <span>New</span>
                </button>
              </div>

              <div className="space-y-1 max-h-[220px] overflow-y-auto pr-1">
                {allHistorySessions.length === 0 ? (
                  <p className="text-[11px] text-[#9ca3af] italic py-2 text-center">
                    No recent sessions in this workspace
                  </p>
                ) : (
                  allHistorySessions.map(hs => {
                    const isActive = hs.id === activeSessionId;
                    const msgCount = hs.messages?.filter(m => m.role === 'user').length || 0;
                    return (
                      <div
                        key={hs.id}
                        onClick={() => openSessionFromHistory(hs)}
                        className={`group p-2 rounded-xl text-xs flex items-center justify-between cursor-pointer transition-all ${
                          isActive
                            ? 'bg-[#eff6ff] dark:bg-[#1e293b] text-[#2563eb] dark:text-[#93c5fd] font-medium'
                            : 'hover:bg-[#f3f4f6] dark:hover:bg-[#252528] text-[#374151] dark:text-[#d1d5db]'
                        }`}
                      >
                        <div className="flex items-center gap-2 min-w-0 pr-2">
                          <MessageSquare className="w-3.5 h-3.5 shrink-0 text-[#6b7280] dark:text-[#9ca3af]" />
                          <div className="min-w-0">
                            <p className="truncate text-xs leading-tight font-medium">
                              {hs.title || 'New Session'}
                            </p>
                            <p className="text-[10px] text-[#9ca3af] truncate">
                              {hs.createdAt || 'Recent'} • {msgCount} prompt{msgCount === 1 ? '' : 's'}
                            </p>
                          </div>
                        </div>

                        <div className="flex items-center gap-1 shrink-0">
                          {isActive && (
                            <span className="w-1.5 h-1.5 rounded-full bg-[#2563eb] dark:bg-[#60a5fa]" />
                          )}
                          <button
                            type="button"
                            onClick={(e) => {
                              e.stopPropagation();
                              deleteSessionPermanently(hs.id);
                            }}
                            className="p-1 opacity-0 group-hover:opacity-100 hover:bg-[#fee2e2] dark:hover:bg-[#450a0a] rounded text-[#ef4444] transition-all cursor-pointer"
                            title="Delete session"
                          >
                            <Trash2 className="w-3 h-3" />
                          </button>
                        </div>
                      </div>
                    );
                  })
                )}
              </div>
            </div>

            {/* Quick Switch to Editor Button */}
            <div className="pt-3 border-t border-[#f3f4f6] dark:border-[#2b2b2b]">
              <button
                type="button"
                onClick={() => setMode('editor')}
                className="w-full py-2 px-3 rounded-xl bg-[#eff6ff] hover:bg-[#dbeafe] dark:bg-[#1e293b] dark:hover:bg-[#273549] text-[#2563eb] dark:text-[#93c5fd] text-xs font-semibold flex items-center justify-center gap-2 transition-colors shadow-2xs"
              >
                <Code2 className="w-4 h-4" />
                <span>Switch to IDE / Code Editor (⌘2)</span>
              </button>
            </div>
          </div>

          {/* Bottom Space Info */}
          <div className="pt-4 border-t border-[#f3f4f6] dark:border-[#2b2b2b] text-[11px] text-[#9ca3af] flex items-center justify-between">
            <span>Runtime: Local Docker Engine</span>
            <span>v1.6.0</span>
          </div>

        </div>
      )}
    </div>
  );
};
