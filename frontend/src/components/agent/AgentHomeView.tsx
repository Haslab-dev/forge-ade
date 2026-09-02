import React, { useState, useMemo } from 'react';
import { 
  ChevronDown, 
  ChevronUp, 
  ArrowRight,
  Folder,
  Sparkles,
  Clock,
  Search
} from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { AgentInputBar } from './AgentInputBar';

export const AgentHomeView: React.FC = () => {
  const { 
    createNewSession, 
    setActiveSessionId, 
    sessions,
    activeWorkspacePath
  } = useWorkspace();

  const [isRecentSessionsOpen, setIsRecentSessionsOpen] = useState(true);
  const [showAllWorkspaces, setShowAllWorkspaces] = useState(false);

  const workspaceSessions = useMemo(() => {
    if (showAllWorkspaces) return sessions;
    return sessions.filter(s => !s.workspacePath || s.workspacePath === activeWorkspacePath);
  }, [sessions, activeWorkspacePath, showAllWorkspaces]);

  const otherSessions = useMemo(() => {
    return sessions.filter(s => s.workspacePath && s.workspacePath !== activeWorkspacePath);
  }, [sessions, activeWorkspacePath]);

  return (
    <div className="flex-1 bg-white dark:bg-[#181818] flex flex-col items-center justify-between p-6 overflow-y-auto min-h-[calc(100vh-74px)] select-text">
      <div className="w-full max-w-2xl flex flex-col items-center pt-8 md:pt-14">
        
        {/* Geometric Hexagonal Agent Logo */}
        <div className="mb-8 flex items-center justify-center">
          <div className="relative w-14 h-14 flex items-center justify-center opacity-85 hover:opacity-100 transition-opacity">
            <svg viewBox="0 0 100 100" className="w-12 h-12 text-[#9ca3af] dark:text-[#6b7280] fill-current">
              <polygon points="50,15 75,29 75,57 50,71 25,57 25,29" fill="none" stroke="currentColor" strokeWidth="6" strokeLinejoin="round" />
              <polygon points="75,43 100,57 100,85 75,99 50,85 50,57" fill="currentColor" fillOpacity="0.25" />
              <polygon points="25,43 50,57 50,85 25,99 0,85 0,57" fill="currentColor" fillOpacity="0.15" />
              <circle cx="50" cy="43" r="5" fill="currentColor" />
            </svg>
          </div>
        </div>

        {/* Workspace Badge */}
        {activeWorkspacePath && (
          <div className="mb-4 flex items-center gap-1.5 px-3 py-1.5 rounded-full bg-[#f1f5f9] dark:bg-[#252526] text-[11px] text-[#64748b] dark:text-[#94a3b8] border border-[#e2e8f0] dark:border-[#383838]">
            <Folder className="w-3 h-3 text-[#2563eb] dark:text-[#38bdf8]" />
            <span className="font-medium">{activeWorkspacePath.split('/').pop()}</span>
            <span className="text-[#94a3b8] dark:text-[#555]">·</span>
            <span className="font-mono text-[10px]">{activeWorkspacePath}</span>
          </div>
        )}

        {/* Unified Input Bar */}
        <AgentInputBar 
          autoFocus={true} 
          placeholder={`Ask about ${activeWorkspacePath?.split('/').pop() || 'your project'}...`} 
        />

        {/* Quick prompt templates */}
        <div className="w-full mt-4 flex items-center gap-2 overflow-x-auto pb-1 text-xs">
          <span className="text-[#9ca3af] text-[11px] uppercase tracking-wider shrink-0">Try:</span>
          {[
            'Explain this codebase architecture',
            'Find and fix potential bugs',
            'Run project tests and inspect build status',
            'Review recent git changes'
          ].map((suggestion, i) => (
            <button
              key={i}
              type="button"
              onClick={() => createNewSession(suggestion)}
              className="px-2.5 py-1 rounded-full bg-[#f3f4f6] dark:bg-[#252526] hover:bg-[#e5e7eb] dark:hover:bg-[#333336] text-[#4b5563] dark:text-[#d1d5db] text-xs shrink-0 transition-colors cursor-pointer"
            >
              {suggestion}
            </button>
          ))}
        </div>

        {/* Recent Sessions Section */}
        <div className="w-full mt-10">
          <div className="flex items-center justify-between text-xs text-[#6b7280] dark:text-[#9ca3af] mb-2 px-1">
            <button
              type="button"
              onClick={() => setIsRecentSessionsOpen(prev => !prev)}
              className="flex items-center gap-1 font-medium hover:text-[#111827] dark:hover:text-white transition-colors cursor-pointer"
            >
              <Clock className="w-3.5 h-3.5" />
              <span>Sessions in this workspace</span>
              <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-[#e2e8f0] dark:bg-[#333] font-mono">
                {workspaceSessions.length}
              </span>
              {isRecentSessionsOpen ? (
                <ChevronUp className="w-3.5 h-3.5" />
              ) : (
                <ChevronDown className="w-3.5 h-3.5" />
              )}
            </button>

            {sessions.length > workspaceSessions.length && (
              <button
                type="button"
                onClick={() => setShowAllWorkspaces(prev => !prev)}
                className="hover:text-[#111827] dark:hover:text-white hover:underline transition-colors cursor-pointer text-[11px]"
              >
                {showAllWorkspaces ? 'Show this workspace only' : `Show all (${sessions.length})`}
              </button>
            )}
          </div>

          {isRecentSessionsOpen && (
            <div className="space-y-1.5">
              {workspaceSessions.length === 0 && (
                <div className="py-8 text-center">
                  <Sparkles className="w-8 h-8 text-[#d1d5db] dark:text-[#444] mx-auto mb-2" />
                  <p className="text-xs text-[#9ca3af]">No sessions yet in this workspace</p>
                  <p className="text-[11px] text-[#d1d5db] dark:text-[#555] mt-1">Start a conversation above to create one</p>
                </div>
              )}
              {workspaceSessions.map(session => {
                const sessionWorkspace = session.workspacePath?.split('/').pop() || '';
                const isOtherWorkspace = session.workspacePath && session.workspacePath !== activeWorkspacePath;
                return (
                  <div
                    key={session.id}
                    onClick={() => setActiveSessionId(session.id)}
                    className="w-full p-3 rounded-xl border border-[#e5e7eb] dark:border-[#2f2f31] bg-white dark:bg-[#1e1e1e] hover:border-[#cbd5e1] dark:hover:border-[#444448] hover:shadow-xs transition-all flex items-center justify-between cursor-pointer group"
                  >
                    <div className="flex items-center gap-3 min-w-0 flex-1">
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2">
                          <span className="font-medium text-xs text-[#111827] dark:text-white group-hover:text-[#2563eb] dark:group-hover:text-[#60a5fa] transition-colors truncate">
                            {session.title}
                          </span>
                          {session.status === 'running' && (
                            <span className="w-1.5 h-1.5 rounded-full bg-[#22c55e] animate-pulse shrink-0" />
                          )}
                        </div>
                        <div className="flex items-center gap-2 mt-0.5">
                          <span className="text-[11px] text-[#9ca3af]">{session.updatedAt}</span>
                          {isOtherWorkspace && sessionWorkspace && (
                            <span className="text-[10px] px-1.5 py-0.5 rounded bg-[#fef3c7] dark:bg-[#422006] text-[#92400e] dark:text-[#fbbf24] font-medium">
                              {sessionWorkspace}
                            </span>
                          )}
                        </div>
                      </div>
                    </div>

                    <div className="flex items-center gap-2 opacity-0 group-hover:opacity-100 transition-opacity shrink-0">
                      <span className="text-[11px] text-[#6b7280] dark:text-[#9ca3af] font-mono">
                        {session.model}
                      </span>
                      <ArrowRight className="w-3.5 h-3.5 text-[#2563eb] dark:text-[#60a5fa]" />
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>

      </div>
    </div>
  );
};
