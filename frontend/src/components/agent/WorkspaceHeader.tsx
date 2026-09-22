import React, { useState } from 'react';
import {
  Folder,
  Ellipsis,
  HelpCircle,
  SquareTerminal,
  PanelRight,
  GitBranch,
} from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';

/**
 * Workspace header: h-12 bar, title section (folder context button,
 * task title, ellipsis menu), action section (mode switch, help, terminal
 * toggle, side-pane toggle).
 */
interface WorkspaceHeaderProps {
  title: string;
  projectName: string;
  gitBranch?: string;
  changesPill?: React.ReactNode;
}

export const WorkspaceHeader: React.FC<WorkspaceHeaderProps> = ({
  title,
  projectName,
  gitBranch,
  changesPill,
}) => {
  const {
    mode,
    setMode,
    isRightActionDrawerOpen,
    setIsRightActionDrawerOpen,
    isLeftSidebarOpen,
    deleteSessionPermanently,
    activeSessionId,
    setIsCommandPaletteOpen,
  } = useWorkspace();
  const [menuOpen, setMenuOpen] = useState(false);

  return (
    <header className="relative flex h-12 w-full shrink-0 border-b border-border/50">
      <div className="flex h-12 flex-1 min-w-0 items-center justify-between gap-2 overflow-hidden p-2">
        {/* Title section */}
        <div className="flex min-w-0 items-center gap-2 overflow-hidden">
          <button
            type="button"
            className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg text-foreground-subtle hover:bg-surface-hover hover:text-foreground transition-colors cursor-pointer"
            title={projectName}
          >
            <Folder className="size-4" />
          </button>
          <h1
            className="flex min-w-12 max-w-100 shrink items-center gap-2 truncate font-semibold text-foreground text-ui-base"
            title={title}
          >
            <span className="min-w-0 truncate">{title}</span>
          </h1>
          {gitBranch ? (
            <span className="flex min-w-0 shrink-0 items-center gap-1.5 text-ui-sm text-foreground-subtlest">
              <GitBranch className="size-3.5" />
              <span className="font-mono">{gitBranch}</span>
            </span>
          ) : null}
          <div className="relative flex min-w-0 shrink-0 items-center gap-1">
            <button
              type="button"
              onClick={() => setMenuOpen((v) => !v)}
              className="flex h-8 w-8 items-center justify-center rounded-lg text-foreground-subtle hover:bg-surface-hover hover:text-foreground transition-colors cursor-pointer"
              aria-label="More"
            >
              <Ellipsis className="size-4" />
            </button>
            {menuOpen && (
              <>
                <div className="fixed inset-0 z-40" onClick={() => setMenuOpen(false)} />
                <div className="absolute left-0 top-full z-50 mt-1 w-48 rounded-lg border border-border bg-popover py-1 shadow-2xl text-ui-sm">
                  <button
                    type="button"
                    onClick={() => {
                      setMenuOpen(false);
                      if (activeSessionId) deleteSessionPermanently(activeSessionId);
                    }}
                    className="w-full cursor-pointer px-3 py-1.5 text-left text-destructive hover:bg-surface-hover transition-colors"
                  >
                    Delete session
                  </button>
                </div>
              </>
            )}
          </div>
        </div>

        {/* Action section */}
        <div className="flex shrink-0 items-center gap-0.5">
          {changesPill}
          {/* Mode selection: Agent | Editor */}
          <div className="mr-1 flex items-center gap-0.5 rounded-lg bg-surface p-0.5">
            <button
              type="button"
              onClick={() => setMode('agent')}
              className={`rounded-md px-2 py-1 text-ui-sm transition-colors cursor-pointer ${
                mode === 'agent'
                  ? 'bg-selected text-foreground font-medium'
                  : 'text-foreground-subtle hover:text-foreground'
              }`}
            >
              Agent
            </button>
            <button
              type="button"
              onClick={() => setMode('editor')}
              className={`rounded-md px-2 py-1 text-ui-sm transition-colors cursor-pointer ${
                mode === 'editor'
                  ? 'bg-selected text-foreground font-medium'
                  : 'text-foreground-subtle hover:text-foreground'
              }`}
            >
              Editor
            </button>
          </div>
          <button
            type="button"
            onClick={() => setIsCommandPaletteOpen(true)}
            className="flex h-8 w-8 items-center justify-center rounded-lg text-foreground-subtle hover:bg-surface-hover hover:text-foreground transition-colors cursor-pointer"
            title="Help"
          >
            <HelpCircle className="size-4" />
          </button>
          <button
            type="button"
            className="flex h-8 w-8 items-center justify-center rounded-lg text-foreground-subtle hover:bg-surface-hover hover:text-foreground transition-colors cursor-pointer"
            title="Terminal"
          >
            <SquareTerminal className="size-4" />
          </button>
          <button
            type="button"
            onClick={() => setIsRightActionDrawerOpen(!isRightActionDrawerOpen)}
            className={`flex h-8 w-8 items-center justify-center rounded-lg transition-colors cursor-pointer ${
              isRightActionDrawerOpen
                ? 'bg-selected text-foreground'
                : 'text-foreground-subtle hover:bg-surface-hover hover:text-foreground'
            }`}
            title="Toggle side pane"
          >
            <PanelRight className="size-4" />
          </button>
        </div>
      </div>
    </header>
  );
};
