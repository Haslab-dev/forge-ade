import React, { useState, useMemo } from 'react';
import { 
  ChevronDown, 
  ChevronUp, 
  ArrowRight,
  Folder,
  FolderOpen,
  Sparkles,
  Clock,
  Trash2,
  Settings,
  Sun,
  Moon,
  HardDrive
} from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { AgentInputBar } from './AgentInputBar';

export const AgentHomeView: React.FC = () => {
  const { 
    createNewSession, 
    setActiveSessionId, 
    deleteSession,
    sessions,
    activeWorkspacePath,
    openFolder,
    openSettingsTab,
    theme,
    toggleTheme
  } = useWorkspace();

  const [isRecentSessionsOpen, setIsRecentSessionsOpen] = useState(true);
  const [showAllWorkspaces, setShowAllWorkspaces] = useState(false);

  const workspaceSessions = useMemo(() => {
    if (showAllWorkspaces || !activeWorkspacePath) return sessions;
    return sessions.filter(s => !s.workspacePath || s.workspacePath === activeWorkspacePath);
  }, [sessions, activeWorkspacePath, showAllWorkspaces]);

  const handleSuggestionClick = (promptText: string) => {
    if (!activeWorkspacePath) {
      openFolder();
      return;
    }
    createNewSession(promptText);
  };

  return (
    <div className="flex-1 bg-white dark:bg-[#181818] flex flex-col items-center justify-between p-6 overflow-y-auto min-h-[calc(100vh-74px)] select-text">
      <div className="w-full max-w-2xl flex flex-col items-center pt-8 md:pt-14">
        
        {/* Geometric Hexagonal Agent Logo */}
        <div className="mb-6 flex items-center justify-center">
          <div className="relative w-14 h-14 flex items-center justify-center opacity-85 hover:opacity-100 transition-opacity">
            <svg viewBox="0 0 100 100" className="w-12 h-12 text-[#9ca3af] dark:text-[#6b7280] fill-current">
              <polygon points="50,15 75,29 75,57 50,71 25,57 25,29" fill="none" stroke="currentColor" strokeWidth="6" strokeLinejoin="round" />
              <polygon points="75,43 100,57 100,85 75,99 50,85 50,57" fill="currentColor" fillOpacity="0.25" />
              <polygon points="25,43 50,57 50,85 25,99 0,85 0,57" fill="currentColor" fillOpacity="0.15" />
              <circle cx="50" cy="43" r="5" fill="currentColor" />
            </svg>
          </div>
        </div>

        {/* Workspace Indicator / Picker Banner */}
        {activeWorkspacePath ? (
          <div className="mb-4 flex items-center gap-2 px-3 py-1.5 rounded-full bg-[#f1f5f9] dark:bg-[#252526] text-[11px] text-[#475569] dark:text-[#94a3b8] border border-[#e2e8f0] dark:border-[#383838]">
            <Folder className="w-3.5 h-3.5 text-[#2563eb] dark:text-[#38bdf8]" />
            <span className="font-semibold text-[#0f172a] dark:text-white">{activeWorkspacePath.split('/').pop()}</span>
            <span className="text-[#94a3b8] dark:text-[#555]">·</span>
            <span className="font-mono text-[10px] text-[#64748b] dark:text-[#737373] truncate max-w-[260px]">{activeWorkspacePath}</span>
            <button
              type="button"
              onClick={() => openFolder()}
              className="ml-1 text-[10px] text-[#2563eb] dark:text-[#60a5fa] hover:underline font-medium cursor-pointer"
            >
              Change
            </button>
          </div>
        ) : (
          <div className="mb-4 flex items-center gap-2 px-3.5 py-1.5 rounded-xl bg-[#eff6ff] dark:bg-[#1e293b]/80 border border-[#bfdbfe] dark:border-[#1e3a5f] text-xs text-[#1e40af] dark:text-[#93c5fd]">
            <FolderOpen className="w-4 h-4 text-[#2563eb] dark:text-[#38bdf8]" />
            <span>No workspace selected. Please choose a folder to start chatting:</span>
            <button
              type="button"
              onClick={() => openFolder()}
              className="px-2 py-0.5 rounded-md bg-[#2563eb] text-white text-[11px] font-semibold hover:bg-[#1d4ed8] transition-colors cursor-pointer shadow-2xs"
            >
              Choose Folder
            </button>
          </div>
        )}

        {/* Unified Input Bar */}
        <AgentInputBar 
          autoFocus={true} 
          placeholder={activeWorkspacePath ? `Ask about ${activeWorkspacePath.split('/').pop()}...` : 'Choose a workspace folder to start chatting...'} 
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
              onClick={() => handleSuggestionClick(suggestion)}
              className="px-2.5 py-1 rounded-full bg-[#f3f4f6] dark:bg-[#252526] hover:bg-[#e5e7eb] dark:hover:bg-[#333336] text-[#4b5563] dark:text-[#d1d5db] text-xs shrink-0 transition-colors cursor-pointer"
            >
              {suggestion}
            </button>
          ))}
        </div>

        {/* Recent Sessions Section */}
        <div className="w-full mt-10">
          <div className="flex items-center justify-between text-xs text-[#6b7280] dark:text-[#9ca3af] mb-2.5 px-1">
            <button
              type="button"
              onClick={() => setIsRecentSessionsOpen(prev => !prev)}
              className="flex items-center gap-1.5 font-semibold text-[#111827] dark:text-white hover:text-[#2563eb] dark:hover:text-[#60a5fa] transition-colors cursor-pointer"
            >
              <Clock className="w-3.5 h-3.5 text-[#6b7280]" />
              <span>Recent sessions</span>
              <span className="text-[10px] px-1.5 py-0.2 rounded-full bg-[#e5e7eb] dark:bg-[#333336] font-mono text-[#374151] dark:text-[#d1d5db]">
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
                className="text-[#2563eb] dark:text-[#60a5fa] hover:underline transition-colors cursor-pointer text-[11px]"
              >
                {showAllWorkspaces ? 'Show active workspace only' : `View all (${sessions.length})`}
              </button>
            )}
          </div>

          {isRecentSessionsOpen && (
            <div className="space-y-2">
              {workspaceSessions.length === 0 ? (
                <div className="py-10 text-center rounded-2xl border border-dashed border-[#e5e7eb] dark:border-[#2f2f31] p-6">
                  <Sparkles className="w-7 h-7 text-[#9ca3af] dark:text-[#555] mx-auto mb-2" />
                  <p className="text-xs font-medium text-[#6b7280] dark:text-[#9ca3af]">No recent sessions</p>
                  <p className="text-[11px] text-[#9ca3af] dark:text-[#666] mt-1">
                    {activeWorkspacePath ? 'Start a conversation in the input box above' : 'Open a workspace folder to start creating sessions'}
                  </p>
                </div>
              ) : (
                workspaceSessions.map(session => {
                  const sessionWorkspace = session.workspacePath?.split('/').pop() || '';
                  const isOtherWorkspace = session.workspacePath && session.workspacePath !== activeWorkspacePath;
                  return (
                    <div
                      key={session.id}
                      className="w-full p-3 rounded-xl border border-[#e5e7eb] dark:border-[#2f2f31] bg-white dark:bg-[#1e1e1e] hover:border-[#cbd5e1] dark:hover:border-[#444448] hover:shadow-xs transition-all flex items-center justify-between cursor-pointer group"
                      onClick={() => setActiveSessionId(session.id)}
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
                              <span className="text-[10px] px-1.5 py-0.2 rounded bg-[#fef3c7] dark:bg-[#422006] text-[#92400e] dark:text-[#fbbf24] font-medium">
                                {sessionWorkspace}
                              </span>
                            )}
                          </div>
                        </div>
                      </div>

                      <div className="flex items-center gap-2 shrink-0">
                        <span className="text-[10px] px-2 py-0.5 rounded-md bg-[#f3f4f6] dark:bg-[#252528] text-[#6b7280] dark:text-[#9ca3af] font-mono">
                          {session.model}
                        </span>
                        <button
                          type="button"
                          onClick={(e) => {
                            e.stopPropagation();
                            deleteSession(session.id);
                          }}
                          className="p-1 text-[#9ca3af] hover:text-[#ef4444] rounded opacity-0 group-hover:opacity-100 transition-opacity cursor-pointer"
                          title="Delete session"
                        >
                          <Trash2 className="w-3.5 h-3.5" />
                        </button>
                        <ArrowRight className="w-3.5 h-3.5 text-[#9ca3af] group-hover:text-[#2563eb] dark:group-hover:text-[#60a5fa] transition-colors" />
                      </div>
                    </div>
                  );
                })
              )}
            </div>
          )}
        </div>

      </div>

      {/* Clean Bottom Footer */}
      <footer className="w-full pt-8 pb-2 flex items-center justify-center gap-3 text-xs text-[#9ca3af] select-none">
        <span>Free</span>
        <span>•</span>
        <button
          type="button"
          onClick={() => openSettingsTab('agents')}
          className="hover:text-[#111827] dark:hover:text-white transition-colors cursor-pointer flex items-center gap-1"
        >
          <Settings className="w-3 h-3 text-[#3b82f6]" />
          <span>Settings</span>
        </button>
        <span>•</span>
        <button
          type="button"
          onClick={toggleTheme}
          className="hover:text-[#111827] dark:hover:text-white transition-colors cursor-pointer flex items-center gap-1"
          title={`Switch to ${theme === 'dark' ? 'Light' : 'Dark'} Mode`}
        >
          {theme === 'dark' ? <Sun className="w-3 h-3 text-[#eab308]" /> : <Moon className="w-3 h-3 text-[#8b5cf6]" />}
          <span>{theme === 'dark' ? 'Light' : 'Dark'}</span>
        </button>
      </footer>
    </div>
  );
};
