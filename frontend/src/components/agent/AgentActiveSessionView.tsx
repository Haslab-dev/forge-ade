import React, { useState, useRef, useEffect } from 'react';
import { 
  Folder, 
  ChevronDown, 
  ChevronRight, 
  HelpCircle, 
  Terminal, 
  PanelRight, 
  MoreHorizontal, 
  GitBranch, 
  Sparkles, 
  CheckCircle2, 
  Loader2, 
  Code2, 
  Undo2, 
  ExternalLink, 
  FileCode, 
  Info, 
  Rocket, 
  X,
  FileText,
  Copy,
  Check,
  BrainCircuit,
  MessageSquare
} from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { FileDiff } from '../../types';
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
    openFileInEditor,
    openSettingsTab,
    deleteSessionPermanently,
    isRightActionDrawerOpen,
    setIsRightActionDrawerOpen
  } = useWorkspace();

  const [expandedThoughts, setExpandedThoughts] = useState<Record<string, boolean>>({
    'th-1': false
  });
  const [activeInlineDiff, setActiveInlineDiff] = useState<FileDiff | null>(null);
  const [isMenuOpen, setIsMenuOpen] = useState(false);
  const chatBottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    chatBottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [activeSession?.messages]);

  if (!activeSession) return null;

  const toggleThought = (id: string) => {
    setExpandedThoughts(prev => ({
      ...prev,
      [id]: !prev[id]
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

  return (
    <div className="flex-1 h-full bg-white dark:bg-[#181819] text-[#1f2937] dark:text-[#cccccc] flex overflow-hidden select-none font-sans relative transition-colors duration-150">
      
      {/* Middle Chat & Task Stream Column */}
      <div className="flex-1 flex flex-col h-full overflow-hidden bg-white dark:bg-[#181819] relative">
        
        {/* Active Session Top Bar */}
        <div className="h-[42px] min-h-[42px] px-3.5 border-b border-[#e5e7eb] dark:border-[#242426] flex items-center justify-between bg-[#f9fafb] dark:bg-[#19191a] z-20">
          
          {/* Left Title & Project Badges */}
          <div className="flex items-center gap-2 min-w-0 max-w-[70%]">
            <h2 className="font-semibold text-xs text-[#111827] dark:text-[#dddddd] truncate max-w-[320px] select-text">
              {activeSession.title || 'Active Task Session'}
            </h2>

            {/* Project Folder Pill */}
            <div className="flex items-center gap-1 px-2 py-0.5 rounded-md bg-[#f3f4f6] dark:bg-[#222225] border border-[#e5e7eb] dark:border-[#2d2d31] text-[11px] text-[#4b5563] dark:text-[#aaaaaa] shrink-0">
              <Folder className="w-3 h-3 text-[#d97706]" />
              <span className="truncate max-w-[120px]">{currentProjectName}</span>
            </div>

            {/* Git Branch Pill */}
            <div className="flex items-center gap-1 px-2 py-0.5 rounded-md bg-[#f3f4f6] dark:bg-[#222225] border border-[#e5e7eb] dark:border-[#2d2d31] text-[11px] text-[#4b5563] dark:text-[#aaaaaa] shrink-0">
              <GitBranch className="w-3 h-3 text-[#3b82f6]" />
              <span>{gitBranch || 'main'}</span>
              <ChevronDown className="w-2.5 h-2.5 text-[#9ca3af] dark:text-[#777777]" />
            </div>

            {/* ... More Options Menu */}
            <div className="relative">
              <button
                type="button"
                onClick={() => setIsMenuOpen(prev => !prev)}
                className="p-1 rounded text-[#6b7280] dark:text-[#777777] hover:text-[#111827] dark:hover:text-white hover:bg-[#f3f4f6] dark:hover:bg-[#252528] transition-colors cursor-pointer"
              >
                <MoreHorizontal className="w-3.5 h-3.5" />
              </button>

              {isMenuOpen && (
                <div className="absolute left-0 top-full mt-1 w-44 rounded-xl bg-white dark:bg-[#222225] border border-[#e5e7eb] dark:border-[#333336] shadow-xl py-1 z-50 text-xs text-[#374151] dark:text-[#cccccc]">
                  <button
                    type="button"
                    onClick={() => {
                      setIsMenuOpen(false);
                      if (activeSessionId) deleteSessionPermanently(activeSessionId);
                    }}
                    className="w-full text-left px-3 py-1.5 hover:bg-[#fee2e2] dark:hover:bg-[#2b2b2e] text-[#ef4444] cursor-pointer"
                  >
                    Delete Session
                  </button>
                </div>
              )}
            </div>
          </div>

          {/* Right Action Icons */}
          <div className="flex items-center gap-1.5 text-[#6b7280] dark:text-[#888888]">
            <button
              type="button"
              onClick={() => setIsRightActionDrawerOpen(prev => !prev)}
              className="p-1.5 rounded hover:bg-[#f3f4f6] dark:hover:bg-[#252528] hover:text-[#111827] dark:hover:text-white transition-colors cursor-pointer"
              title="Toggle Terminal"
            >
              <Terminal className="w-4 h-4" />
            </button>

            <button
              type="button"
              onClick={() => setIsRightActionDrawerOpen(prev => !prev)}
              className={`p-1.5 rounded transition-colors cursor-pointer ${
                isRightActionDrawerOpen ? 'text-[#2563eb] dark:text-white bg-[#eff6ff] dark:bg-[#252528]' : 'hover:bg-[#f3f4f6] dark:hover:bg-[#252528] hover:text-[#111827] dark:hover:text-white'
              }`}
              title="Toggle Secondary Sidebar"
            >
              <PanelRight className="w-4 h-4" />
            </button>
          </div>

        </div>

        {/* Chat Transcript Area */}
        <div className="flex-1 overflow-y-auto p-4 md:px-8 space-y-4 select-text">
          
          {activeSession.messages.length === 0 ? (
            <div className="py-20 text-center text-[#777777] space-y-2">
              <Sparkles className="w-8 h-8 text-[#555558] mx-auto" />
              <p className="font-medium text-xs text-[#aaaaaa]">Ready for instructions</p>
              <p className="text-[11px] text-[#666669]">
                Type a follow-up prompt below to start modifying files or running commands.
              </p>
            </div>
          ) : (
            activeSession.messages.map((msg, index) => {
              const isUser = msg.role === 'user';

              return (
                <div key={msg.id || index} className="w-full space-y-3">
                  
                  {/* USER MESSAGE BUBBLE */}
                  {isUser && (
                    <div className="flex flex-col items-end">
                      <div className="max-w-[85%] rounded-2xl p-3.5 bg-[#f3f4f6] dark:bg-[#252528] text-[#111827] dark:text-[#f8fafc] text-xs leading-relaxed border border-[#e5e7eb] dark:border-[#333336] shadow-xs space-y-2">
                        <p className="whitespace-pre-wrap font-medium">{msg.content}</p>
                      </div>
                    </div>
                  )}

                  {/* AGENT RESPONSE & STEPS */}
                  {!isUser && (
                    <div className="flex flex-col items-start space-y-2.5 w-full">
                      
                      {/* Step Summary Text */}
                      {msg.content && (
                        <div className="text-[13px] text-[#1e293b] dark:text-[#e2e8f0] leading-relaxed select-text py-1 w-full">
                          <MarkdownRenderer content={msg.content} />
                        </div>
                      )}

                      {/* Tool Execution Step Pills (e.g. Commit, Layout verify) */}
                      {msg.toolExecutions && msg.toolExecutions.length > 0 && (
                        <div className="w-full space-y-1.5 py-1">
                          {msg.toolExecutions.map(tool => (
                            <div
                              key={tool.id}
                              className="flex items-center gap-2 px-3 py-1.5 rounded-xl bg-[#f9fafb] dark:bg-[#1e1e21] border border-[#e5e7eb] dark:border-[#2b2b2e] text-xs text-[#4b5563] dark:text-[#bbbbbb] w-fit shadow-2xs"
                            >
                              <CheckCircle2 className="w-3.5 h-3.5 text-[#10b981] shrink-0" />
                              <span className="font-mono text-[11px] text-[#111827] dark:text-[#dddddd]">
                                {tool.command || tool.toolName}
                              </span>
                              <span className="text-[10px] text-[#6b7280] dark:text-[#666666]">· Completed</span>
                            </div>
                          ))}
                        </div>
                      )}

                      {/* Accordion: ⚙ Thought · a few seconds */}
                      {msg.thoughts && msg.thoughts.length > 0 && (
                        <div className="w-full">
                          {msg.thoughts.map(th => {
                            const isOpen = expandedThoughts[th.id] !== undefined ? expandedThoughts[th.id] : false;
                            return (
                              <div key={th.id} className="w-fit">
                                <button
                                  type="button"
                                  onClick={() => toggleThought(th.id)}
                                  className="flex items-center gap-1.5 text-xs text-[#6b7280] dark:text-[#888888] hover:text-[#111827] dark:hover:text-[#cccccc] transition-colors cursor-pointer py-1"
                                >
                                  <BrainCircuit className="w-3.5 h-3.5 text-[#a855f7]" />
                                  <span>Thought · a few seconds</span>
                                  {isOpen ? (
                                    <ChevronDown className="w-3 h-3 text-[#6b7280] dark:text-[#666666]" />
                                  ) : (
                                    <ChevronRight className="w-3 h-3 text-[#6b7280] dark:text-[#666666]" />
                                  )}
                                </button>

                                {isOpen && (
                                  <div className="mt-1 p-3 rounded-xl bg-[#f9fafb] dark:bg-[#1c1c1e] border border-[#e5e7eb] dark:border-[#28282b] text-[11px] text-[#4b5563] dark:text-[#aaaaaa] leading-relaxed max-w-xl animate-in fade-in select-text">
                                    {th.thoughtText}
                                  </div>
                                )}
                              </div>
                            );
                          })}
                        </div>
                      )}

                      {/* Changed Files Preview Card */}
                      {sessionDiffs.length > 0 && (
                        <div className="w-full max-w-xl rounded-2xl bg-[#ffffff] dark:bg-[#1c1c1e] border border-[#e5e7eb] dark:border-[#2b2b2e] overflow-hidden shadow-xs my-2">
                          {/* Card Header */}
                          <div className="px-3.5 py-2.5 bg-[#f9fafb] dark:bg-[#202023] border-b border-[#e5e7eb] dark:border-[#2b2b2e] flex items-center justify-between text-xs">
                            <div className="flex items-center gap-2">
                              <ChevronDown className="w-3.5 h-3.5 text-[#6b7280] dark:text-[#888888]" />
                              <span className="font-semibold text-[#111827] dark:text-[#dddddd]">
                                {sessionDiffs.length} files changed
                              </span>
                              <span className="font-mono text-[11px]">
                                <span className="text-[#16a34a] dark:text-[#22c55e]">+{totalAdditions}</span>{' '}
                                <span className="text-[#dc2626] dark:text-[#ef4444]">-{totalDeletions}</span>
                              </span>
                            </div>

                            <button
                              type="button"
                              className="flex items-center gap-1 text-[11px] text-[#6b7280] dark:text-[#888888] hover:text-[#111827] dark:hover:text-white transition-colors cursor-pointer"
                            >
                              <Undo2 className="w-3 h-3" />
                              <span>Undo</span>
                            </button>
                          </div>

                          {/* Files List */}
                          <div className="p-2 space-y-1">
                            {sessionDiffs.map(d => {
                              const parts = d.filePath.split('/');
                              const fname = parts.pop() || d.fileName;
                              const dir = parts.join('/') || 'root';

                              return (
                                <div
                                  key={d.id}
                                  className="flex items-center justify-between p-2 rounded-xl bg-[#f9fafb] dark:bg-[#222225] hover:bg-[#f3f4f6] dark:hover:bg-[#28282c] transition-colors group border border-[#f3f4f6] dark:border-transparent"
                                >
                                  <div className="flex items-center gap-2 min-w-0 flex-1 pr-2">
                                    <Code2 className="w-3.5 h-3.5 text-[#3b82f6] shrink-0" />
                                    <span className="text-xs font-semibold text-[#111827] dark:text-white truncate">
                                      {fname}
                                    </span>
                                    <span className="text-[11px] text-[#6b7280] dark:text-[#777777] font-mono truncate">
                                      {dir}
                                    </span>
                                    <span className="font-mono text-[11px] ml-1 shrink-0">
                                      <span className="text-[#16a34a] dark:text-[#22c55e]">+{d.additions}</span>{' '}
                                      <span className="text-[#dc2626] dark:text-[#ef4444]">-{d.deletions}</span>
                                    </span>
                                  </div>

                                  <div className="flex items-center gap-1.5 shrink-0">
                                    <button
                                      type="button"
                                      onClick={() => setActiveInlineDiff(d)}
                                      className="px-2.5 py-1 rounded-md bg-[#e5e7eb] dark:bg-[#2d2d31] hover:bg-[#d1d5db] dark:hover:bg-[#38383e] text-[#111827] dark:text-white text-[11px] font-medium transition-colors cursor-pointer shadow-2xs"
                                    >
                                      Review
                                    </button>

                                    <button
                                      type="button"
                                      onClick={() => openDiffInEditor(d)}
                                      className="px-2 py-1 rounded-md hover:bg-[#e5e7eb] dark:hover:bg-[#2d2d31] text-[#6b7280] dark:text-[#aaaaaa] hover:text-[#111827] dark:hover:text-white text-[11px] transition-colors cursor-pointer flex items-center gap-0.5"
                                    >
                                      <span>Open</span>
                                      <ChevronDown className="w-3 h-3" />
                                    </button>
                                  </div>
                                </div>
                              );
                            })}
                          </div>
                        </div>
                      )}

                    </div>
                  )}

                </div>
              );
            })
          )}

          <div ref={chatBottomRef} />
        </div>

        {/* Bottom Follow-up Input Bar */}
        <div className="p-4 md:px-8 pt-2 pb-4">
          <AgentTaskInputBar
            placeholder="Ask for follow-up changes"
            autoFocus={true}
            isCompact={true}
          />
        </div>

      </div>

      {/* Right Sidebar (Review / Terminal / Side Chat) */}
      {isRightActionDrawerOpen && (
        <AgentRightSidebar 
          onClose={() => setIsRightActionDrawerOpen(false)}
          onOpenDiff={(d) => setActiveInlineDiff(d)}
        />
      )}

      {/* Inline Diff Overlay Modal if clicked */}
      {activeInlineDiff && (
        <div className="absolute inset-0 z-50 flex flex-col bg-white dark:bg-[#181819] animate-in fade-in duration-100">
          <div className="h-[40px] px-4 bg-[#f9fafb] dark:bg-[#202023] border-b border-[#e5e7eb] dark:border-[#2b2b2e] flex items-center justify-between">
            <div className="flex items-center gap-2 text-xs font-semibold text-[#111827] dark:text-white">
              <Code2 className="w-4 h-4 text-[#3b82f6]" />
              <span>Reviewing changes: {activeInlineDiff.fileName}</span>
              <span className="font-mono text-[11px] text-[#22c55e]">+{activeInlineDiff.additions}</span>
              <span className="font-mono text-[11px] text-[#ef4444]">-{activeInlineDiff.deletions}</span>
            </div>

            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={() => {
                  openDiffInEditor(activeInlineDiff);
                  setActiveInlineDiff(null);
                }}
                className="px-2.5 py-1 rounded bg-[#f3f4f6] dark:bg-[#2b2b2e] hover:bg-[#e5e7eb] dark:hover:bg-[#35353a] text-[#111827] dark:text-white text-xs font-medium flex items-center gap-1 transition-colors cursor-pointer"
              >
                <ExternalLink className="w-3.5 h-3.5" />
                <span>Open in full tab</span>
              </button>
              <button
                type="button"
                onClick={() => setActiveInlineDiff(null)}
                className="p-1 rounded hover:bg-[#f3f4f6] dark:hover:bg-[#2b2b2e] text-[#6b7280] dark:text-[#888888] hover:text-[#111827] dark:hover:text-white transition-colors cursor-pointer"
              >
                <X className="w-4 h-4" />
              </button>
            </div>
          </div>

          <div className="flex-1 p-4 overflow-hidden bg-[#f8fafc] dark:bg-[#141414]">
            <DiffViewer diff={activeInlineDiff} onClose={() => setActiveInlineDiff(null)} isInline={true} />
          </div>
        </div>
      )}

    </div>
  );
};
