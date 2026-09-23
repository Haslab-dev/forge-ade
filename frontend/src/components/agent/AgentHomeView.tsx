import React, { useState, useRef, useEffect, type CSSProperties } from 'react';
import {
  Folder,
  ChevronDown,
  GitBranch,
  Check,
  Search,
  Plus,
  Cloud,
  MessageSquareOff,
} from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { WorkspaceHeader } from './WorkspaceHeader';
import { AgentTaskInputBar } from './AgentTaskInputBar';

interface AgentHomeViewProps {
  onToggleRightSidebar?: () => void;
  onOpenTerminal?: () => void;
}

const SUGGESTED_PROMPTS = [
  'Explore codebase architecture',
  'Run tests & verify diagnostics',
  'Refactor UI components',
  'Configure tools & MCPs',
];

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

  // Draft context header row, rendered inside the composer card.
  const contextHeader = (
    <div ref={dropdownRef} className="flex min-w-0 flex-wrap items-center gap-0 p-1.5">
      {/* Workspace chip (ghost) */}
      <div className="relative">
        <button
          type="button"
          onClick={() => {
            setIsWorkspaceDropdownOpen(prev => !prev);
            setIsBranchDropdownOpen(false);
          }}
          className="flex h-8 items-center gap-1.5 rounded-lg px-2 text-ui-sm font-medium text-foreground hover:bg-surface-hover transition-colors cursor-pointer"
        >
          <Folder className="size-3.5 text-foreground-subtle" />
          <span>{currentWorkspaceName}</span>
          <ChevronDown className="size-3.5 text-foreground-subtle" />
        </button>

        {isWorkspaceDropdownOpen && (
          <div className="absolute bottom-full left-0 z-50 mb-2 w-64 rounded-2xl border border-popover-border bg-popover p-2 text-xs text-foreground shadow-md">
            <div className="mb-2 flex items-center gap-2 rounded-md border border-border bg-background px-3 py-1.5">
              <Search className="size-3.5 text-foreground-subtle" />
              <input
                type="text"
                value={workspaceSearch}
                onChange={e => setWorkspaceSearch(e.target.value)}
                placeholder="Search workspaces"
                className="w-full border-0 bg-transparent font-mono text-xs text-foreground placeholder-foreground-subtlest focus:outline-hidden"
                autoFocus
              />
            </div>
            <div className="max-h-44 space-y-1 overflow-y-auto py-1">
              {filteredWorkspaces.map(ws => {
                const name = ws.split('/').filter(Boolean).pop() || ws;
                const isCurrent = ws === activeWorkspacePath || name === currentWorkspaceName;
                return (
                  <button
                    key={ws}
                    type="button"
                    onClick={() => handleSelectWorkspace(ws)}
                    className={`flex w-full cursor-pointer items-center justify-between rounded-md px-2.5 py-1.5 text-left transition-colors ${
                      isCurrent ? 'bg-surface-hover font-medium text-foreground' : 'text-foreground-subtle hover:bg-surface-hover hover:text-foreground'
                    }`}
                  >
                    <span className="flex min-w-0 items-center gap-2 truncate">
                      <Folder className="size-3.5 shrink-0 text-foreground-subtle" />
                      <span className="truncate">{name}</span>
                    </span>
                    {isCurrent && <Check className="size-3.5 shrink-0 text-success" />}
                  </button>
                );
              })}
            </div>
            <div className="my-1 h-px bg-border" />
            <div className="space-y-1 pt-1">
              <button
                type="button"
                onClick={() => { setIsWorkspaceDropdownOpen(false); openFolder(); }}
                className="flex w-full cursor-pointer items-center gap-2 rounded-md px-2.5 py-1.5 text-left text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground"
              >
                <Plus className="size-3.5" />
                <span>Open local folder</span>
              </button>
              <button
                type="button"
                onClick={() => { setIsWorkspaceDropdownOpen(false); openSettingsTab('general'); }}
                className="flex w-full cursor-pointer items-center gap-2 rounded-md px-2.5 py-1.5 text-left text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground"
              >
                <Cloud className="size-3.5" />
                <span>Remote connection</span>
              </button>
              <button
                type="button"
                onClick={() => { setIsWorkspaceDropdownOpen(false); closeWorkspace(); }}
                className="flex w-full cursor-pointer items-center gap-2 rounded-md px-2.5 py-1.5 text-left text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground"
              >
                <MessageSquareOff className="size-3.5" />
                <span>Work outside a project</span>
              </button>
            </div>
          </div>
        )}
      </div>

      {/* Branch chip (ghost) */}
      <div className="relative">
        <button
          type="button"
          onClick={() => {
            setIsBranchDropdownOpen(prev => !prev);
            setIsWorkspaceDropdownOpen(false);
          }}
          className="flex h-8 items-center gap-1.5 rounded-lg px-2 text-ui-sm font-mono text-foreground hover:bg-surface-hover transition-colors cursor-pointer"
        >
          <GitBranch className="size-3.5 text-foreground-subtle" />
          <span>{gitBranch || 'main'}</span>
          <ChevronDown className="size-3.5 text-foreground-subtle" />
        </button>

        {isBranchDropdownOpen && (
          <div className="absolute bottom-full left-0 z-50 mb-2 w-44 rounded-2xl border border-popover-border bg-popover p-1.5 font-mono text-xs text-foreground shadow-md">
            <div className="px-2.5 py-1 text-ui-xs font-semibold uppercase text-foreground-subtlest">
              Git Branches
            </div>
            {['main', 'develop'].map(b => (
              <button
                key={b}
                type="button"
                onClick={() => setIsBranchDropdownOpen(false)}
                className="flex w-full cursor-pointer items-center justify-between rounded-md px-2.5 py-1.5 text-left text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground"
              >
                <span>{b}</span>
                {b === (gitBranch || 'main') && <Check className="size-3.5 text-success" />}
              </button>
            ))}
          </div>
        )}
      </div>
    </div>
  );

  return (
    <div className="flex-1 h-full bg-background text-foreground flex flex-col relative overflow-hidden select-none transition-colors">

      {/* Workspace header (draft variant) */}
      <div className="relative z-10">
        <WorkspaceHeader variant="draft" title="" projectName={currentWorkspaceName} />
      </div>

      {/* Main Content Area */}
      <div className="flex flex-1 flex-col items-center justify-center px-4 z-10">
        <div className="flex w-full max-w-2xl flex-col items-center justify-center gap-6">

          {/* Greeting over the watermark logo */}
          <div className="relative mb-4 flex w-full items-center justify-center">
            <div
              aria-hidden="true"
              className="pointer-events-none absolute left-1/2 top-1/2 aspect-[5/4] w-[min(72vw,25rem)] -translate-x-1/2 -translate-y-1/2 text-foreground-subtlest"
            >
              <svg
                viewBox="0 0 100 100"
                className="h-full w-full opacity-70"
                fill="none"
                stroke="currentColor"
                style={{
                  WebkitMaskImage: 'linear-gradient(to bottom, black 0%, transparent 70%, transparent 100%)',
                  maskImage: 'linear-gradient(to bottom, black 0%, transparent 70%, transparent 100%)',
                  WebkitMaskRepeat: 'no-repeat',
                  maskRepeat: 'no-repeat',
                  WebkitMaskSize: '100% 100%',
                  maskSize: '100% 100%',
                }}
              >
                <polygon points="50,15 75,29 75,57 50,71 25,57 25,29" strokeWidth="1.5" strokeLinejoin="round" />
                <polygon points="75,43 100,57 100,85 75,99 50,85 50,57" strokeWidth="1.5" strokeLinejoin="round" />
                <polygon points="25,43 50,57 50,85 25,99 0,85 0,57" strokeWidth="1.5" strokeLinejoin="round" />
                <circle cx="50" cy="43" r="4" strokeWidth="1.5" />
              </svg>
            </div>
            <p className="relative z-10 text-center text-3xl/[1.2] font-medium text-foreground">
              nice work today
            </p>
          </div>

          {/* Composer card with draft context header inside */}
          <div className="w-full">
            <AgentTaskInputBar
              placeholder="Ask anything, @ to add context, / for commands"
              autoFocus={true}
              isCompact={false}
              headerNode={contextHeader}
            />
          </div>

          {/* Suggested prompt chips (horizontal, waterfall stagger) */}
          <div className="w-full min-w-0 overflow-x-auto [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
            <div className="mx-auto flex w-max items-center gap-4">
              {SUGGESTED_PROMPTS.map((label, index) => (
                <button
                  key={label}
                  type="button"
                  aria-label={label}
                  title={label}
                  onClick={() => createNewSession(label)}
                  className="forge-draft-prompt-waterfall group/button flex h-8 min-w-0 cursor-pointer items-center justify-start gap-1.5 overflow-hidden rounded-lg border border-border px-3 text-left text-ui-base font-normal leading-4.5 text-foreground transition-colors hover:bg-surface-hover"
                  style={{ '--forge-draft-prompt-waterfall-delay': `${index * 65}ms` } as CSSProperties}
                >
                  <span className="flex size-4 shrink-0 items-center justify-center text-foreground opacity-70 transition-opacity group-hover/button:opacity-100">
                    <GitBranch className="size-4" />
                  </span>
                  <span className="min-w-0 max-w-64 truncate text-ui-base text-foreground opacity-70 transition-opacity group-hover/button:opacity-100">
                    {label}
                  </span>
                </button>
              ))}
            </div>
          </div>

        </div>
      </div>

    </div>
  );
};
