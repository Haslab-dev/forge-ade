import React, { useState, useRef, useEffect } from 'react';
import {
  Folder,
  Ellipsis,
  SquareTerminal,
  PanelRight,
  PanelLeft,
  MessageSquarePlus,
  Pencil,
  Archive,
  Trash2,
  Plus,
  Code2,
  ClipboardCopy,
  ClipboardCheck,
  ExternalLink,
  LifeBuoy,
  Sparkles
} from 'lucide-react';
import { cn, formatDisplayTitle } from '../../lib/utils';
import { showToast } from '../../lib/toast';
import { useWorkspace } from '../../stores/workspaceStore';
import { ApiBridge } from '../../services/apiBridge';
import { WorkspaceSwitcherDropdown } from './WorkspaceSwitcherDropdown';
import { BranchSelector } from './BranchSelector';
import { ThemePicker } from '../common/ThemePicker';

/**
 * Workspace header, laid out like ZCode: title first (prominent), then the
 * project folder chip (click = reveal in Finder), branch switcher dropdown,
 * and the task "…" menu; the right cluster is a compact mode dropdown
 * (Agent/Editor) plus help / terminal / side-pane toggles.
 */

interface WorkspaceHeaderProps {
  title: string;
  projectName: string;
  gitBranch?: string;
  /** draft = new-task view (no border), task = active session. */
  variant?: 'draft' | 'task';
  onToggleTerminal?: () => void;
  isSidePaneOpen?: boolean;
  onToggleSidePane?: () => void;
  onRenameSession?: () => void;
  /** Sidebar visibility + toggle button pinned at the header's left corner. */
  sidebarOpen?: boolean;
  onToggleSidebar?: () => void;
  /** Left inset (px) so content clears the window-controls drag corner when
      the sidebar is collapsed (ZCode DesktopTopOverlay reserves this space). */
  leftSafeInset?: number;
  /** True when the full-height sidebar renders the sidebar toggle itself. */
  hideSidebarToggle?: boolean;
}

function useDropdown(onClose: () => void) {
  const ref = useRef<HTMLDivElement | null>(null);
  // Callers pass inline closures — keep them in a ref so the two document
  // listeners are registered once per dropdown instead of churned on every
  // header render (which happens on every store update).
  const onCloseRef = useRef(onClose);
  onCloseRef.current = onClose;
  useEffect(() => {
    const onPointerDown = (event: MouseEvent) => {
      if (ref.current && !ref.current.contains(event.target as Node)) onCloseRef.current();
    };
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onCloseRef.current();
    };
    document.addEventListener('mousedown', onPointerDown);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onPointerDown);
      document.removeEventListener('keydown', onKey);
    };
  }, []);
  return ref;
}

export const WorkspaceHeader: React.FC<WorkspaceHeaderProps> = ({
  title,
  projectName,
  gitBranch,
  variant = 'draft',
  onToggleTerminal,
  isSidePaneOpen = false,
  onToggleSidePane,
  onRenameSession,
  sidebarOpen,
  onToggleSidebar,
  leftSafeInset = 0,
  hideSidebarToggle = false
}) => {
  const {
    mode,
    deleteSessionPermanently,
    activeSessionId,
    setIsCommandPaletteOpen,
    activeWorkspacePath,
    archivedSessionIds,
    toggleArchiveSession,
    setMode,
    switchSurface,
    gitFiles,
  } = useWorkspace();

  const [copiedItem, setCopiedItem] = useState<string | null>(null);
  const [taskMenuOpen, setTaskMenuOpen] = useState(false);
  const taskMenuRef = useDropdown(() => setTaskMenuOpen(false));

  const copyText = async (key: string, text: string) => {
    try {
      await navigator.clipboard.writeText(text);
      setCopiedItem(key);
      window.setTimeout(() => setCopiedItem(cur => (cur === key ? null : cur)), 1500);
    } catch {
      // clipboard unavailable (permissions) — silently ignore
    }
  };

  // Agent surface parity: no dropdowns — the workspace and branch chips are
  // static badges; project switching lives in the sidebar and New Task flow.
  const isAgentSurface = mode === 'agent' || mode === 'automations';
  const wsPath = activeWorkspacePath || '';
  const wsIsRemote = wsPath.startsWith('ssh://');
  const wsRest = wsIsRemote ? wsPath.replace(/^ssh:\/\//, '') : '';
  const wsSlash = wsRest.indexOf('/');
  const wsHost = wsIsRemote ? (wsSlash !== -1 ? wsRest.substring(0, wsSlash) : wsRest).split(':')[0] : '';
  const isGeneralWorkspace = !wsPath || wsPath === 'general' || wsPath.endsWith('/.forge-ade/general') || wsPath.endsWith('/.forge-ade/general/');
  const wsName = wsIsRemote
    ? (wsSlash !== -1 ? wsRest.substring(wsSlash) : '/').split('/').filter(Boolean).pop() || 'remote-root'
    : isGeneralWorkspace
      ? 'General'
      : wsPath.replace(/\/+$/, '').split('/').filter(Boolean).pop() || 'General';

  const headerProjectBadge = <WorkspaceSwitcherDropdown />;

  const isGitWorkspace = Boolean(activeWorkspacePath && !activeWorkspacePath.startsWith('ssh://') && gitBranch);
  const headerBranchButton = (
    <BranchSelector variant="header" className="ml-0.5" />
  );

  const headerTaskMenu = activeSessionId ? (
    <div className="relative flex min-w-0 shrink-0 items-center" ref={taskMenuRef}>
      <button
        type="button"
        onClick={() => setTaskMenuOpen(v => !v)}
        aria-haspopup="menu"
        aria-expanded={taskMenuOpen}
        className="flex h-8 w-8 items-center justify-center rounded-lg text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground"
        aria-label="More"
      >
        <Ellipsis className="size-4" />
      </button>
      {taskMenuOpen && (
        <div role="menu" aria-label="Task actions" className="absolute left-0 top-full z-50 mt-1 w-64 rounded-xl border border-border bg-popover p-1.5 text-ui-sm shadow-lg">
          <button
            type="button"
            role="menuitem"
            onClick={() => {
              setTaskMenuOpen(false);
              onRenameSession?.();
            }}
            className="flex w-full items-center gap-2.5 rounded-lg px-2 py-1.5 text-left text-foreground transition-colors hover:bg-surface-hover"
          >
            <Pencil className="size-4 text-foreground-subtle" />
            Rename task
          </button>
          <button
            type="button"
            role="menuitem"
            onClick={() => {
              setTaskMenuOpen(false);
              if (activeSessionId) toggleArchiveSession(activeSessionId);
            }}
            className="flex w-full items-center gap-2.5 rounded-lg px-2 py-1.5 text-left text-foreground transition-colors hover:bg-surface-hover"
          >
            <Archive className="size-4 text-foreground-subtle" />
            {activeSessionId && archivedSessionIds.has(activeSessionId) ? 'Unarchive task' : 'Archive task'}
          </button>

          <button
            type="button"
            role="menuitem"
            onClick={() => {
              setTaskMenuOpen(false);
              onToggleTerminal?.();
            }}
            className="flex w-full items-center gap-2.5 rounded-lg px-2 py-1.5 text-left text-foreground transition-colors hover:bg-surface-hover"
          >
            <SquareTerminal className="size-4 text-foreground-subtle" />
            Toggle terminal
          </button>

          <div className="mx-1 my-1 h-px bg-border" />

          <button
            type="button"
            role="menuitem"
            onClick={() => {
              setTaskMenuOpen(false);
              if (activeWorkspacePath) ApiBridge.openInFinder(activeWorkspacePath);
            }}
            className="flex w-full items-center gap-2.5 rounded-lg px-2 py-1.5 text-left text-foreground transition-colors hover:bg-surface-hover"
          >
            <ExternalLink className="size-4 text-foreground-subtle" />
            Open in Finder
          </button>
          <button
            type="button"
            role="menuitem"
            onClick={() => {
              setTaskMenuOpen(false);
              void ApiBridge.getSandboxDir(activeWorkspacePath || '').then(dir => {
                if (dir) ApiBridge.openInFinder(dir);
                else showToast('Sandbox not available', 'warn');
              });
            }}
            className="flex w-full items-center gap-2.5 rounded-lg px-2 py-1.5 text-left text-foreground transition-colors hover:bg-surface-hover"
          >
            <Folder className="size-4 text-foreground-subtle" />
            Open sandbox folder
          </button>
          <button
            type="button"
            role="menuitem"
            onClick={() => {
              setTaskMenuOpen(false);
              void ApiBridge.getSandboxDir(activeWorkspacePath || '').then(dir => {
                if (dir) copyText('sandbox', dir);
              });
            }}
            className="flex w-full items-center gap-2.5 rounded-lg px-2 py-1.5 text-left text-foreground transition-colors hover:bg-surface-hover"
          >
            {copiedItem === 'sandbox' ? <ClipboardCheck className="size-4 text-success" /> : <ClipboardCopy className="size-4 text-foreground-subtle" />}
            {copiedItem === 'sandbox' ? 'Copied!' : 'Copy sandbox path'}
          </button>
          <button
            type="button"
            role="menuitem"
            onClick={() => activeWorkspacePath && copyText('path', activeWorkspacePath)}
            className="flex w-full items-center gap-2.5 rounded-lg px-2 py-1.5 text-left text-foreground transition-colors hover:bg-surface-hover"
          >
            {copiedItem === 'path' ? <ClipboardCheck className="size-4 text-success" /> : <ClipboardCopy className="size-4 text-foreground-subtle" />}
            {copiedItem === 'path' ? 'Copied!' : 'Copy path'}
          </button>
          <button
            type="button"
            role="menuitem"
            onClick={() =>
              activeSessionId &&
              activeWorkspacePath &&
              copyText('log', `${activeWorkspacePath}/.forge/sessions/${activeSessionId}.json`)
            }
            className="flex w-full items-center gap-2.5 rounded-lg px-2 py-1.5 text-left text-foreground transition-colors hover:bg-surface-hover"
          >
            {copiedItem === 'log' ? <ClipboardCheck className="size-4 text-success" /> : <ClipboardCopy className="size-4 text-foreground-subtle" />}
            {copiedItem === 'log' ? 'Copied!' : 'Copy log path'}
          </button>
          <button
            type="button"
            role="menuitem"
            onClick={() => activeSessionId && copyText('session', activeSessionId)}
            className="flex w-full items-center gap-2.5 rounded-lg px-2 py-1.5 text-left text-foreground transition-colors hover:bg-surface-hover"
          >
            {copiedItem === 'session' ? <ClipboardCheck className="size-4 text-success" /> : <ClipboardCopy className="size-4 text-foreground-subtle" />}
            {copiedItem === 'session' ? 'Copied!' : 'Copy session ID'}
          </button>

          <div className="mx-1 my-1 h-px bg-border" />

          <button
            type="button"
            role="menuitem"
            onClick={() => {
              setTaskMenuOpen(false);
              ApiBridge.browserOpenURL('https://github.com/hasdev/forge-ade/issues');
            }}
            className="flex w-full items-center gap-2.5 rounded-lg px-2 py-1.5 text-left text-foreground transition-colors hover:bg-surface-hover"
          >
            <LifeBuoy className="size-4 text-foreground-subtle" />
            Report issue
          </button>
          <button
            type="button"
            role="menuitem"
            onClick={() => {
              setTaskMenuOpen(false);
              if (activeSessionId) deleteSessionPermanently(activeSessionId);
            }}
            className="flex w-full items-center gap-2.5 rounded-lg px-2 py-1.5 text-left text-destructive transition-colors hover:bg-destructive/10"
          >
            <Trash2 className="size-4" />
            Delete task
          </button>
        </div>
      )}
    </div>
  ) : null;

  return (
    <header
      data-testid="workspace-header"
      className={cn(
        '@container/workspace-header relative flex h-[var(--workspace-topbar-height)] w-full shrink-0 border-b border-border/50 bg-chrome select-none'
      )}
    >
      {/* ── Single unified header row: toggle → title → chips → "…" ── */}
      <div className="titlebar-drag flex h-[var(--workspace-topbar-height)] flex-1 min-w-0 items-center justify-between gap-3 pl-[88px] pr-3">
        <div className="flex min-w-0 items-center gap-2 no-drag-region">
          {mode !== 'editor' && !hideSidebarToggle && onToggleSidebar && (
            <button
              type="button"
              onClick={onToggleSidebar}
              aria-label={sidebarOpen ? 'Hide sidebar' : 'Show sidebar'}
              title="Toggle sidebar (⌘B)"
              className="mr-1 flex size-7 shrink-0 items-center justify-center rounded-md text-foreground-subtle hover:bg-surface-hover hover:text-foreground transition-colors cursor-pointer"
            >
              <PanelLeft className="size-4" />
            </button>
          )}
          {title && title !== projectName ? (
            <span
              className="truncate text-ui-sm font-semibold text-foreground max-w-80"
              title={title}
            >
              {formatDisplayTitle(title)}
            </span>
          ) : null}
          {/* Workspace + branch selectors live in the composer's context strip
              on the agent surface — the header keeps them for the editor only. */}
          {!isAgentSurface && headerProjectBadge}
          {!isAgentSurface && headerBranchButton}
          {headerTaskMenu}
        </div>

        {/* Action section */}
        <div className="flex shrink-0 items-center gap-1 no-drag-region">
          {/* Surface toggle — Agent and Editor are two separate surfaces with
              their own workspace folders; switching carries the folder across. */}
          <button
            type="button"
            onClick={() => switchSurface(isAgentSurface ? 'editor' : 'agent')}
            aria-label={isAgentSurface ? 'Switch to Editor' : 'Switch to Agent'}
            title={isAgentSurface ? 'Open in Editor (carries this folder)' : 'Open in Agent (carries this folder)'}
            className="flex h-7 w-8 items-center justify-center rounded-md border border-border-default bg-toolbar-control transition-colors hover:bg-accent"
          >
            {isAgentSurface ? (
              <Code2 className="size-4 text-foreground-subtle" />
            ) : (
              <Sparkles className="size-4 text-primary" />
            )}
          </button>

          <button
            type="button"
            onClick={() => activeWorkspacePath && ApiBridge.openInFinder(activeWorkspacePath)}
            disabled={!activeWorkspacePath}
            className="flex h-8 w-8 items-center justify-center rounded-lg text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
            title="Open in Finder"
            aria-label="Open workspace in Finder"
          >
            <Folder className="size-4" />
          </button>
          {/* Bottom dock terminal toggle (all modes) */}
          {onToggleTerminal ? (
            <button
              type="button"
              onClick={onToggleTerminal}
              aria-pressed={false}
              className="flex h-8 w-8 items-center justify-center rounded-lg text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground cursor-pointer"
              title="Toggle terminal"
              aria-label="Toggle terminal"
            >
              <SquareTerminal className="size-4" />
            </button>
          ) : null}
          {/* Side-pane (Review/Terminal/Side chat) toggle — always visible in both Agent and Editor */}
          {onToggleSidePane ? (
            <button
              type="button"
              onClick={() => onToggleSidePane()}
              aria-pressed={isSidePaneOpen}
              className={cn(
                "flex h-8 w-8 items-center justify-center rounded-lg transition-colors cursor-pointer",
                isSidePaneOpen
                  ? "bg-surface-hover text-foreground"
                  : "text-foreground-subtle hover:bg-surface-hover hover:text-foreground"
              )}
              title={isSidePaneOpen ? "Close side panel" : "Open side panel (Review / Chat)"}
              aria-label="Toggle side pane"
            >
              <PanelRight className="size-4" />
            </button>
          ) : null}

          {/* Theme & Palette Picker */}
          <div className="ml-1">
            <ThemePicker compact />
          </div>
        </div>
      </div>

    </header>
  );
};

// Re-export for callers that rendered a MessageSquarePlus icon next to the title.
export { MessageSquarePlus };
