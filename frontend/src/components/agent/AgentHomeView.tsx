import React, { useState, useRef, useEffect, type CSSProperties } from 'react';
import {
  Folder,
  ChevronDown,
  Check,
  Search,
  Plus,
  Cloud,
  MessageSquareOff,
  GitBranch
} from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { AgentTaskInputBar } from './AgentTaskInputBar';
import { AgentRightSidebar } from './AgentRightSidebar';
import { PanelResizeHandle } from './PanelResizeHandle';

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
    openSettingsTab,
    isLeftSidebarOpen,
    isRightActionDrawerOpen,
    setIsRightActionDrawerOpen,
    rightPaneWidth,
    setRightPaneWidth
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

  // The home composer is the internal ForgeADE agent — it works on local
  // folders only, so remote (ssh://) workspaces are not offered here.
  const filteredWorkspaces = recentWorkspaces
    .filter(ws => !ws.startsWith('ssh://'))
    .filter(ws => {
      const name = ws.split('/').filter(Boolean).pop() || ws;
      return name.toLowerCase().includes(workspaceSearch.toLowerCase());
    });

  const handleSelectWorkspace = (wsPath: string) => {
    setActiveWorkspacePath(wsPath);
    setIsWorkspaceDropdownOpen(false);
  };

  // Kybern Empty-Landing controls tray: stacked flush above the composer input
  const landingTray = (
    <div
      ref={dropdownRef}
      className="flex min-h-8 w-full flex-nowrap items-center gap-x-1.5 px-3 py-1 text-xs text-foreground-secondary"
    >
      {/* Project selector chip */}
      <div className="relative">
        <button
          type="button"
          onClick={() => {
            setIsWorkspaceDropdownOpen(prev => !prev);
            setIsBranchDropdownOpen(false);
          }}
          className="inline-flex cursor-pointer items-center gap-1.5 rounded-full px-2 py-1 text-[length:var(--app-font-size-ui-sm,11px)] font-normal text-foreground-secondary transition-colors hover:bg-[var(--color-background-button-secondary-hover)] hover:text-foreground"
        >
          <Folder className="size-3.5 opacity-70" />
          <span className="truncate max-w-[160px]">{currentWorkspaceName}</span>
          <ChevronDown className="size-3 opacity-60" />
        </button>

        {isWorkspaceDropdownOpen && (
          <div className="absolute bottom-full left-0 z-50 mb-2 w-64 rounded-xl border border-border bg-popover p-1.5 text-xs text-foreground shadow-xl animate-in fade-in zoom-in-95 duration-100">
            <div className="mb-1.5 flex items-center gap-2 rounded-lg border border-border bg-background px-2.5 py-1.5">
              <Search className="size-3.5 text-muted-foreground" />
              <input
                type="text"
                value={workspaceSearch}
                onChange={e => setWorkspaceSearch(e.target.value)}
                placeholder="Search projects…"
                aria-label="Search projects"
                className="w-full border-0 bg-transparent text-xs text-foreground placeholder:text-muted-foreground/60 focus:outline-hidden"
                autoFocus
              />
            </div>
            <div className="max-h-44 space-y-0.5 overflow-y-auto py-0.5">
              {filteredWorkspaces.map(ws => {
                const name = ws.split('/').filter(Boolean).pop() || ws;
                const isCurrent = ws === activeWorkspacePath || name === currentWorkspaceName;
                return (
                  <button
                    key={ws}
                    type="button"
                    onClick={() => handleSelectWorkspace(ws)}
                    className={`flex w-full cursor-pointer items-center justify-between rounded-lg px-2.5 py-1.5 text-left text-xs transition-colors ${
                      isCurrent ? 'bg-[var(--color-background-button-secondary)] font-medium text-foreground' : 'text-foreground/80 hover:bg-[var(--color-background-button-secondary-hover)] hover:text-foreground'
                    }`}
                  >
                    <span className="flex min-w-0 items-center gap-2 truncate">
                      <Folder className="size-3.5 shrink-0 opacity-70" />
                      <span className="truncate">{name}</span>
                    </span>
                    {isCurrent && <Check className="size-3.5 shrink-0 text-success" />}
                  </button>
                );
              })}
            </div>
            <div className="my-1 h-px bg-border" />
            <div className="space-y-0.5 pt-0.5">
              <button
                type="button"
                onClick={() => { setIsWorkspaceDropdownOpen(false); openFolder(); }}
                className="flex w-full cursor-pointer items-center gap-2 rounded-lg px-2.5 py-1.5 text-left text-xs text-foreground/80 hover:bg-[var(--color-background-button-secondary-hover)] hover:text-foreground transition-colors"
              >
                <Plus className="size-3.5 opacity-70" />
                <span>Open folder…</span>
              </button>
              <button
                type="button"
                onClick={() => { setIsWorkspaceDropdownOpen(false); closeWorkspace(); }}
                className="flex w-full cursor-pointer items-center gap-2 rounded-lg px-2.5 py-1.5 text-left text-xs text-foreground/80 hover:bg-[var(--color-background-button-secondary-hover)] hover:text-foreground transition-colors"
              >
                <MessageSquareOff className="size-3.5 opacity-70" />
                <span>Work outside a project</span>
              </button>
            </div>
          </div>
        )}
      </div>

      {/* Git Branch chip */}
      <div className="relative">
        <button
          type="button"
          onClick={() => {
            setIsBranchDropdownOpen(prev => !prev);
            setIsWorkspaceDropdownOpen(false);
          }}
          className="inline-flex cursor-pointer items-center gap-1.5 rounded-full px-2 py-1 text-[length:var(--app-font-size-ui-sm,11px)] font-mono text-foreground-secondary transition-colors hover:bg-[var(--color-background-button-secondary-hover)] hover:text-foreground"
        >
          <GitBranch className="size-3.5 opacity-70" />
          <span>{gitBranch || 'main'}</span>
          <ChevronDown className="size-3 opacity-60" />
        </button>

        {isBranchDropdownOpen && (
          <div className="absolute bottom-full left-0 z-50 mb-2 w-48 rounded-xl border border-border bg-popover p-1.5 text-xs text-foreground shadow-xl animate-in fade-in zoom-in-95 duration-100">
            <div className="px-2.5 py-1 text-ui-xs font-semibold uppercase tracking-wider text-muted-foreground">
              Branch
            </div>
            {['main', 'develop'].map(b => (
              <button
                key={b}
                type="button"
                onClick={() => setIsBranchDropdownOpen(false)}
                className="flex w-full cursor-pointer items-center justify-between rounded-lg px-2.5 py-1.5 text-left font-mono text-xs text-foreground/80 hover:bg-[var(--color-background-button-secondary-hover)] hover:text-foreground transition-colors"
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

      <div className="flex flex-1 min-h-0 w-full">

      {/* Centered Draft Body */}
      <div className="flex flex-1 flex-col items-center justify-center px-4 z-10 min-h-0">
        <div className="flex w-full max-w-[var(--app-chat-max-width,48rem)] flex-col items-center justify-center gap-6 my-auto">

          {/* Kybern Brand Glyph + "What should we do in {project}?" Heading */}
          <div className="flex flex-col items-center gap-4 text-center select-none w-full">
            {/* ForgeADE brand glyph — four-point forge spark (pure SVG, theme-aware) */}
            <div className="text-foreground">
              <svg width={56} height={56} viewBox="0 0 48 48" fill="currentColor" className="shrink-0" aria-hidden="true">
                <path d="M24 3c1.6 10.6 6.4 16.7 20 21-13.6 4.3-18.4 10.4-20 21-1.6-10.6-6.4-16.7-20-21 13.6-4.3 18.4-10.4 20-21Z" />
                <circle cx="39.5" cy="8.5" r="3.2" />
              </svg>
            </div>

            <h2 className="text-[26px] font-normal leading-[1.15] tracking-[-0.015em] text-foreground/95 sm:text-[30px]">
              What should we do in{' '}
              <button
                type="button"
                onClick={() => setIsWorkspaceDropdownOpen(prev => !prev)}
                className="cursor-pointer rounded-xs text-inherit underline decoration-dotted decoration-[1.5px] underline-offset-[6px] transition-colors duration-150 ease-out hover:text-foreground/70 focus-visible:outline-hidden"
              >
                {currentWorkspaceName}
              </button>
              ?
            </h2>
          </div>

          {/* Centered Kybern Composer with Landing Tray */}
          <div className="w-full pt-2">
            <AgentTaskInputBar
              placeholder="Ask anything, @ threads or files, $ skills, or / commands"
              autoFocus={true}
              isCompact={false}
              headerNode={landingTray}
            />
          </div>

        </div>
      </div>

      {/* Right side pane (file tabs opened from the explorer, Review, ...) */}
      {isRightActionDrawerOpen && (
        <>
          <PanelResizeHandle
            edge="left"
            ariaLabel="Resize side panel"
            getStartWidth={() => rightPaneWidth}
            onResize={setRightPaneWidth}
            min={360}
            max={() => Math.max(360, Math.floor(window.innerWidth * 0.6))}
            className="-mr-1"
          />
          <AgentRightSidebar initialTab={null} />
        </>
      )}
      </div>

    </div>
  );
};

export default AgentHomeView;
