import React, { useState, useRef, useEffect } from 'react';
import { 
  Folder, 
  ChevronDown, 
  GitBranch, 
  Check, 
  Search, 
  Plus, 
  Cloud, 
  MessageSquareOff, 
  Sparkles 
} from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { WorkspaceHeader } from './WorkspaceHeader';
import { AgentTaskInputBar } from './AgentTaskInputBar';

interface AgentHomeViewProps {
  onToggleRightSidebar?: () => void;
  onOpenTerminal?: () => void;
}

export const AgentHomeView: React.FC<AgentHomeViewProps> = () => {
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

  return (
    <div className="flex-1 h-full bg-background text-foreground flex flex-col relative overflow-hidden select-none transition-colors">
      
      {/* Workspace header (draft variant) */}
      <div className="relative z-10">
        <WorkspaceHeader title="New task" projectName={activeWorkspacePath?.split('/').filter(Boolean).pop() || 'forge-ade'} />
      </div>

      {/* Background Watermark */}
      <div className="absolute inset-0 flex items-center justify-center pointer-events-none select-none z-0">
        <div className="w-[420px] h-[420px] opacity-[0.03] dark:opacity-[0.04] flex items-center justify-center text-slate-800 dark:text-white">
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
        <div className="w-full max-w-[700px] flex flex-col items-center space-y-6">
          
          {/* Main Greeting Headline */}
          <div className="text-center space-y-1">
            <h1 className="text-3xl font-medium text-foreground tracking-tight">
              nice work today
            </h1>
            <p className="text-sm text-foreground-subtle dark:text-foreground-subtlest">
              What would you like to build or automate next?
            </p>
          </div>

          {/* Central Task Box Container */}
          <div className="w-full flex flex-col space-y-3 relative" ref={dropdownRef}>
            
            {/* Top Project & Branch Pills */}
            <div className="flex items-center gap-2 pl-1">
              
              {/* Workspace Picker Pill */}
              <div className="relative">
                <button
                  type="button"
                  onClick={() => {
                    setIsWorkspaceDropdownOpen(prev => !prev);
                    setIsBranchDropdownOpen(false);
                  }}
                  className="flex items-center gap-2 px-3 py-1.5 rounded-[8px] bg-card dark:bg-surface-hover hover:bg-surface-hover dark:hover:bg-surface-hover border border-border text-[13px] font-mono text-foreground transition-colors cursor-pointer shadow-2xs"
                >
                  <Folder className="w-3.5 h-3.5 text-foreground-subtle" />
                  <span className="font-medium">{currentWorkspaceName}</span>
                  <ChevronDown className="w-3.5 h-3.5 text-foreground-subtle dark:text-foreground-subtle" />
                </button>

                {/* Workspace Popover */}
                {isWorkspaceDropdownOpen && (
                  <div className="absolute left-0 top-full mt-2 w-64 rounded-[10px] bg-card shadow-2xl border border-border p-2 z-50 text-xs text-foreground">
                    {/* Search Input */}
                    <div className="flex items-center gap-2 px-3 py-1.5 rounded-[6px] bg-surface dark:bg-background border border-border mb-2">
                      <Search className="w-3.5 h-3.5 text-foreground-subtle dark:text-foreground-subtle" />
                      <input
                        type="text"
                        value={workspaceSearch}
                        onChange={e => setWorkspaceSearch(e.target.value)}
                        placeholder="Search workspaces"
                        className="bg-transparent border-0 text-xs text-foreground placeholder-foreground-subtlest focus:outline-hidden w-full font-mono "
                        autoFocus
                      />
                    </div>

                    {/* Workspace list */}
                    <div className="space-y-1 max-h-44 overflow-y-auto py-1">
                      {filteredWorkspaces.map(ws => {
                        const name = ws.split('/').filter(Boolean).pop() || ws;
                        const isCurrent = ws === activeWorkspacePath || name === currentWorkspaceName;

                        return (
                          <button
                            key={ws}
                            type="button"
                            onClick={() => handleSelectWorkspace(ws)}
                            className={`w-full text-left px-2.5 py-1.5 rounded-[6px] flex items-center justify-between hover:bg-surface-hover transition-colors cursor-pointer font-mono ${
                              isCurrent ? 'bg-surface-hover text-foreground font-semibold' : 'text-foreground-subtle'
                            }`}
                          >
                            <div className="flex items-center gap-2 truncate">
                              <Folder className="w-3.5 h-3.5 text-foreground-subtle shrink-0" />
                              <span className="truncate">{name}</span>
                            </div>
                            {isCurrent && <Check className="w-3.5 h-3.5 text-success shrink-0" />}
                          </button>
                        );
                      })}
                    </div>

                    <div className="h-[1px] bg-border my-1" />

                    {/* Action buttons */}
                    <div className="space-y-1 pt-1">
                      <button
                        type="button"
                        onClick={() => {
                          setIsWorkspaceDropdownOpen(false);
                          openFolder();
                        }}
                        className="w-full text-left px-2.5 py-1.5 rounded-[6px] hover:bg-surface-hover text-foreground-subtle hover:text-foreground flex items-center gap-2 cursor-pointer transition-colors"
                      >
                        <Plus className="w-3.5 h-3.5" />
                        <span>Open local folder</span>
                      </button>

                      <button
                        type="button"
                        onClick={() => {
                          setIsWorkspaceDropdownOpen(false);
                          openSettingsTab('general');
                        }}
                        className="w-full text-left px-2.5 py-1.5 rounded-[6px] hover:bg-surface-hover text-foreground-subtle hover:text-foreground flex items-center gap-2 cursor-pointer transition-colors"
                      >
                        <Cloud className="w-3.5 h-3.5" />
                        <span>Remote connection</span>
                      </button>

                      <button
                        type="button"
                        onClick={() => {
                          setIsWorkspaceDropdownOpen(false);
                          closeWorkspace();
                        }}
                        className="w-full text-left px-2.5 py-1.5 rounded-[6px] hover:bg-surface-hover text-foreground-subtle hover:text-foreground flex items-center gap-2 cursor-pointer transition-colors"
                      >
                        <MessageSquareOff className="w-3.5 h-3.5" />
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
                  className="flex items-center gap-2 px-3 py-1.5 rounded-[8px] bg-card dark:bg-surface-hover hover:bg-surface-hover dark:hover:bg-surface-hover border border-border text-[13px] font-mono text-foreground transition-colors cursor-pointer shadow-2xs"
                >
                  <GitBranch className="w-3.5 h-3.5 text-foreground-subtle" />
                  <span>{gitBranch || 'main'}</span>
                  <ChevronDown className="w-3.5 h-3.5 text-foreground-subtle dark:text-foreground-subtle" />
                </button>

                {isBranchDropdownOpen && (
                  <div className="absolute left-0 top-full mt-2 w-44 rounded-[10px] bg-card shadow-xl border border-border p-1.5 z-50 text-xs font-mono text-foreground">
                    <div className="px-2.5 py-1 text-[10px] text-foreground-subtle dark:text-foreground-subtlest uppercase font-semibold">
                      Git Branches
                    </div>
                    {['main', 'develop'].map(b => (
                      <button
                        key={b}
                        type="button"
                        onClick={() => setIsBranchDropdownOpen(false)}
                        className="w-full text-left px-2.5 py-1.5 rounded-[6px] hover:bg-surface-hover text-foreground-subtle hover:text-foreground flex items-center justify-between cursor-pointer transition-colors"
                      >
                        <span>{b}</span>
                        {b === (gitBranch || 'main') && <Check className="w-3.5 h-3.5 text-success" />}
                      </button>
                    ))}
                  </div>
                )}
              </div>

            </div>

            {/* Input Bar Component */}
            <AgentTaskInputBar 
              placeholder="Ask anything, @ to add context, / for commands"
              autoFocus={true}
              isCompact={false}
            />

          </div>

          {/* Quick Suggestions Cards */}
          <div className="grid grid-cols-2 gap-3 w-full pt-2">
            {[
              { title: 'Explore codebase architecture', desc: 'Scan symbols, directories, and entry points' },
              { title: 'Run tests & verify diagnostics', desc: 'Execute test suite and report build errors' },
              { title: 'Refactor UI components', desc: 'Update styles, tokens, and responsive layout' },
              { title: 'Configure tools & MCPs', desc: 'Review active skills, plugins, and model access' }
            ].map((sug, i) => (
              <button
                key={i}
                type="button"
                onClick={() => createNewSession(sug.title)}
                className="p-3.5 rounded-[10px] bg-card hover:bg-surface dark:hover:bg-surface-hover border border-border text-left transition-colors cursor-pointer group space-y-1 shadow-2xs"
              >
                <div className="text-xs font-semibold text-foreground flex items-center justify-between">
                  <span>{sug.title}</span>
                  <Sparkles className="w-3 h-3 text-foreground-subtlest group-hover:text-primary dark:group-hover:text-success transition-colors" />
                </div>
                <p className="text-ui-xs text-foreground-subtle dark:text-foreground-subtlest leading-snug">
                  {sug.desc}
                </p>
              </button>
            ))}
          </div>

        </div>
      </div>

    </div>
  );
};
