import React, { useState, useRef, useEffect } from 'react';
import { 
  Folder, 
  ChevronDown, 
  HelpCircle, 
  Terminal, 
  PanelRight, 
  Sparkles, 
  Rocket, 
  Info, 
  Plus, 
  Cloud, 
  FolderOpen, 
  MessageSquareOff, 
  GitBranch, 
  Check, 
  Search,
  Clock,
  Bug,
  Presentation,
  Moon
} from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { AgentTaskInputBar } from './AgentTaskInputBar';

interface AgentHomeViewProps {
  onToggleRightSidebar?: () => void;
  onOpenTerminal?: () => void;
}

export const AgentHomeView: React.FC<AgentHomeViewProps> = ({ 
  onToggleRightSidebar,
  onOpenTerminal 
}) => {
  const { 
    activeWorkspacePath, 
    setActiveWorkspacePath, 
    recentWorkspaces, 
    openFolder, 
    closeWorkspace, 
    gitBranch,
    createNewSession,
    openSettingsTab
  } = useWorkspace();

  const [isWorkspaceDropdownOpen, setIsWorkspaceDropdownOpen] = useState(false);
  const [workspaceSearch, setWorkspaceSearch] = useState('');
  const [isBranchDropdownOpen, setIsBranchDropdownOpen] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);

  const currentWorkspaceName = activeWorkspacePath 
    ? activeWorkspacePath.split('/').filter(Boolean).pop() || 'forge-ade'
    : 'forge-ade';

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setIsWorkspaceDropdownOpen(false);
        setIsBranchDropdownOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const filteredWorkspaces = recentWorkspaces.filter(ws => {
    const name = ws.split('/').filter(Boolean).pop() || ws;
    return name.toLowerCase().includes(workspaceSearch.toLowerCase());
  });

  const handleSelectWorkspace = (wsPath: string) => {
    setActiveWorkspacePath(wsPath);
    setIsWorkspaceDropdownOpen(false);
  };

  const handleSuggestion = (prompt: string) => {
    createNewSession(prompt);
  };

  return (
    <div className="flex-1 h-full bg-white dark:bg-[#181819] text-[#1f2937] dark:text-[#cccccc] flex flex-col relative overflow-hidden select-none font-sans transition-colors duration-150">
      
      {/* Stylized Center Watermark Logo */}
      <div className="absolute inset-0 flex items-center justify-center pointer-events-none select-none z-0">
        <div className="w-[480px] h-[480px] opacity-[0.035] dark:opacity-[0.05] flex items-center justify-center text-slate-800 dark:text-white">
          <svg viewBox="0 0 100 100" className="w-full h-full fill-current">
            <polygon points="50,15 75,29 75,57 50,71 25,57 25,29" fill="none" stroke="currentColor" strokeWidth="3" strokeLinejoin="round" />
            <polygon points="75,43 100,57 100,85 75,99 50,85 50,57" fill="currentColor" />
            <polygon points="25,43 50,57 50,85 25,99 0,85 0,57" fill="currentColor" />
            <circle cx="50" cy="43" r="4" fill="currentColor" />
          </svg>
        </div>
      </div>

      {/* Main Content Area */}
      <div className="flex-1 flex flex-col items-center justify-center px-4 z-10">
        <div className="w-full max-w-[660px] flex flex-col items-center space-y-5">
          
          {/* Main Greeting Headline */}
          <h1 className="text-2xl md:text-3xl font-medium text-[#111827] dark:text-[#dddddd] tracking-tight text-center">
            nice work today
          </h1>

          {/* Central Task Card */}
          <div className="w-full flex flex-col space-y-2 relative" ref={dropdownRef}>
            
            {/* Top Tag Pills: [✕ forge-ade ⌄] [⑂ main ⌄] */}
            <div className="flex items-center gap-2 pl-1">
              
              {/* Workspace Picker Pill */}
              <div className="relative">
                <button
                  type="button"
                  onClick={() => {
                    setIsWorkspaceDropdownOpen(prev => !prev);
                    setIsBranchDropdownOpen(false);
                  }}
                  className="flex items-center gap-1.5 px-3 py-1 rounded-full bg-[#f3f4f6] dark:bg-[#222225] hover:bg-[#e5e7eb] dark:hover:bg-[#28282c] border border-[#e5e7eb] dark:border-[#2d2d31] text-xs text-[#374151] dark:text-[#dddddd] hover:text-[#111827] dark:hover:text-white transition-colors cursor-pointer"
                >
                  <span className="text-[#9ca3af] hover:text-[#111827] dark:hover:text-white">✕</span>
                  <Folder className="w-3.5 h-3.5 text-[#d97706]" />
                  <span className="font-medium">{currentWorkspaceName}</span>
                  <ChevronDown className="w-3 h-3 text-[#9ca3af] dark:text-[#777777]" />
                </button>

                {/* Workspace Popover */}
                {isWorkspaceDropdownOpen && (
                  <div className="absolute left-0 top-full mt-1.5 w-64 rounded-2xl bg-white dark:bg-[#202022] shadow-2xl border border-[#e5e7eb] dark:border-[#333336] p-2 z-50 animate-in fade-in zoom-in-95 duration-100 text-xs">
                    {/* Search Input */}
                    <div className="flex items-center gap-2 px-2.5 py-1.5 rounded-lg bg-[#f9fafb] dark:bg-[#18181a] border border-[#e5e7eb] dark:border-[#2c2c2f] mb-1.5">
                      <Search className="w-3.5 h-3.5 text-[#9ca3af] dark:text-[#777777]" />
                      <input
                        type="text"
                        value={workspaceSearch}
                        onChange={e => setWorkspaceSearch(e.target.value)}
                        placeholder="Search workspaces"
                        className="bg-transparent border-0 text-xs text-[#111827] dark:text-white placeholder-[#9ca3af] dark:placeholder-[#777777] focus:outline-hidden w-full"
                        autoFocus
                      />
                    </div>

                    {/* Workspace list */}
                    <div className="space-y-0.5 max-h-44 overflow-y-auto py-1">
                      {filteredWorkspaces.map(ws => {
                        const name = ws.split('/').filter(Boolean).pop() || ws;
                        const isCurrent = ws === activeWorkspacePath || name === currentWorkspaceName;

                        return (
                          <button
                            key={ws}
                            type="button"
                            onClick={() => handleSelectWorkspace(ws)}
                            className={`w-full text-left px-2.5 py-1.5 rounded-lg flex items-center justify-between hover:bg-[#f3f4f6] dark:hover:bg-[#2a2a2d] transition-colors cursor-pointer ${
                              isCurrent ? 'bg-[#f3f4f6] dark:bg-[#2a2a2d] text-[#111827] dark:text-white font-medium' : 'text-[#4b5563] dark:text-[#cccccc]'
                            }`}
                          >
                            <div className="flex items-center gap-2 truncate">
                              <Folder className="w-3.5 h-3.5 text-[#d97706] shrink-0" />
                              <span className="truncate">{name}</span>
                            </div>
                            {isCurrent && <Check className="w-3.5 h-3.5 text-[#3b82f6] shrink-0" />}
                          </button>
                        );
                      })}
                    </div>

                    <div className="h-[1px] bg-[#e5e7eb] dark:bg-[#2a2a2d] my-1" />

                    {/* Action buttons */}
                    <div className="space-y-0.5 pt-0.5">
                      <button
                        type="button"
                        onClick={() => {
                          setIsWorkspaceDropdownOpen(false);
                          openFolder();
                        }}
                        className="w-full text-left px-2.5 py-1.5 rounded-lg hover:bg-[#f3f4f6] dark:hover:bg-[#2a2a2d] text-[#4b5563] dark:text-[#cccccc] hover:text-[#111827] dark:hover:text-white flex items-center gap-2 cursor-pointer"
                      >
                        <Plus className="w-3.5 h-3.5 text-[#6b7280] dark:text-[#888888]" />
                        <span>Open folder</span>
                      </button>

                      <button
                        type="button"
                        onClick={() => {
                          setIsWorkspaceDropdownOpen(false);
                          openSettingsTab('general');
                        }}
                        className="w-full text-left px-2.5 py-1.5 rounded-lg hover:bg-[#f3f4f6] dark:hover:bg-[#2a2a2d] text-[#4b5563] dark:text-[#cccccc] hover:text-[#111827] dark:hover:text-white flex items-center gap-2 cursor-pointer"
                      >
                        <Cloud className="w-3.5 h-3.5 text-[#6b7280] dark:text-[#888888]" />
                        <span>Remote connection</span>
                      </button>

                      <button
                        type="button"
                        onClick={() => {
                          setIsWorkspaceDropdownOpen(false);
                          closeWorkspace();
                        }}
                        className="w-full text-left px-2.5 py-1.5 rounded-lg hover:bg-[#f3f4f6] dark:hover:bg-[#2a2a2d] text-[#4b5563] dark:text-[#cccccc] hover:text-[#111827] dark:hover:text-white flex items-center gap-2 cursor-pointer"
                      >
                        <MessageSquareOff className="w-3.5 h-3.5 text-[#6b7280] dark:text-[#888888]" />
                        <span>Work outside a project</span>
                      </button>
                    </div>

                  </div>
                )}
              </div>

              {/* Git Branch Pill */}
              <div className="relative">
                <button
                  type="button"
                  onClick={() => {
                    setIsBranchDropdownOpen(prev => !prev);
                    setIsWorkspaceDropdownOpen(false);
                  }}
                  className="flex items-center gap-1.5 px-3 py-1 rounded-full bg-[#f3f4f6] dark:bg-[#222225] hover:bg-[#e5e7eb] dark:hover:bg-[#28282c] border border-[#e5e7eb] dark:border-[#2d2d31] text-xs text-[#374151] dark:text-[#dddddd] hover:text-[#111827] dark:hover:text-white transition-colors cursor-pointer"
                >
                  <GitBranch className="w-3.5 h-3.5 text-[#3b82f6]" />
                  <span className="font-medium">{gitBranch || 'main'}</span>
                  <ChevronDown className="w-3 h-3 text-[#9ca3af] dark:text-[#777777]" />
                </button>

                {isBranchDropdownOpen && (
                  <div className="absolute left-0 top-full mt-1.5 w-44 rounded-xl bg-white dark:bg-[#202022] shadow-xl border border-[#e5e7eb] dark:border-[#333336] p-1 z-50 text-xs text-[#374151] dark:text-[#cccccc]">
                    <div className="px-2.5 py-1 text-[10px] text-[#6b7280] dark:text-[#777777] uppercase font-semibold">
                      Git Branches
                    </div>
                    {['main', 'feat/agent-refactor', 'develop'].map(b => (
                      <button
                        key={b}
                        type="button"
                        onClick={() => setIsBranchDropdownOpen(false)}
                        className="w-full text-left px-2.5 py-1 rounded hover:bg-[#f3f4f6] dark:hover:bg-[#2a2a2d] hover:text-[#111827] dark:hover:text-white flex items-center justify-between cursor-pointer"
                      >
                        <span>{b}</span>
                        {b === (gitBranch || 'main') && <Check className="w-3 h-3 text-[#3b82f6]" />}
                      </button>
                    ))}
                  </div>
                )}
              </div>

            </div>

            {/* Input Bar Component */}
            <AgentTaskInputBar 
              placeholder="Ask anything, @ to add context, / for commands or capabilities"
              autoFocus={true}
            />

          </div>

          {/* Suggestion Pills below Card */}
          <div className="flex items-center justify-center gap-2 flex-wrap pt-2">
            {[
              { label: 'Weekly Summary', icon: Clock, prompt: 'Generate a comprehensive weekly summary of recent commits and task accomplishments.' },
              { label: 'Error Fix', icon: Bug, prompt: 'Inspect recent errors, analyze logs, and fix any pending bugs in the codebase.' },
              { label: 'PPT Creation', icon: Presentation, prompt: 'Generate an architecture outline and presentation slides for this project.' },
              { label: 'Idle-time task', icon: Moon, prompt: 'Run static analysis, optimize bundle dependencies, and clean unused code.' }
            ].map((sugg) => {
              const Icon = sugg.icon;
              return (
                <button
                  key={sugg.label}
                  type="button"
                  onClick={() => handleSuggestion(sugg.prompt)}
                  className="flex items-center gap-1.5 px-3 py-1.5 rounded-full bg-[#f9fafb] dark:bg-[#202023] hover:bg-[#f3f4f6] dark:hover:bg-[#28282c] border border-[#e5e7eb] dark:border-[#2b2b2e] text-xs text-[#4b5563] dark:text-[#bbbbbb] hover:text-[#111827] dark:hover:text-white transition-all cursor-pointer shadow-xs"
                >
                  <Icon className="w-3.5 h-3.5 text-[#6b7280] dark:text-[#888888]" />
                  <span>{sugg.label}</span>
                </button>
              );
            })}
          </div>

        </div>
      </div>

    </div>
  );
};
