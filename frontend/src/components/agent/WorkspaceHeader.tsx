import React, { useState, useRef, useEffect } from 'react';
import {
  Folder,
  Ellipsis,
  HelpCircle,
  SquareTerminal,
  PanelRight,
  PanelLeft,
  GitBranch,
  MessageSquarePlus,
  Pencil,
  Archive,
  Trash2,
  Check,
  ChevronDown,
  Search,
  Plus,
  Code2,
  Network,
  ClipboardCopy,
  ClipboardCheck,
  ExternalLink,
  LifeBuoy,
  Sparkles
} from 'lucide-react';
import { cn, formatDisplayTitle } from '../../lib/utils';
import { useWorkspace } from '../../stores/workspaceStore';
import { useSessionStore } from '../../stores/sessionStore';
import { ApiBridge } from '../../services/apiBridge';
import { GitGraphModal } from '../modals/GitGraphModal';

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
}

function useDropdown(onClose: () => void) {
  const ref = useRef<HTMLDivElement | null>(null);
  useEffect(() => {
    const onPointerDown = (event: MouseEvent) => {
      if (ref.current && !ref.current.contains(event.target as Node)) onClose();
    };
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose();
    };
    document.addEventListener('mousedown', onPointerDown);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onPointerDown);
      document.removeEventListener('keydown', onKey);
    };
  }, [onClose]);
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
  leftSafeInset = 0
}) => {
  const {
    mode,
    previousMode,
    deleteSessionPermanently,
    activeSessionId,
    setIsCommandPaletteOpen,
    activeWorkspacePath,
    archivedSessionIds,
    toggleArchiveSession,
    setMode,
    gitFiles,
    refreshGitStatus,
    refreshGitLog,
    leftSidebarWidth
  } = useWorkspace();

  const [branchMenuOpen, setBranchMenuOpen] = useState(false);
  const branchMenuRef = useDropdown(() => {
    setBranchMenuOpen(false);
    setCreatingBranch(false);
    setBranchError('');
  });
  const [branches, setBranches] = useState<string[]>([]);
  const [branchesLoading, setBranchesLoading] = useState(false);
  const [branchSearch, setBranchSearch] = useState('');
  const [creatingBranch, setCreatingBranch] = useState(false);
  const [newBranchName, setNewBranchName] = useState('');
  const [branchError, setBranchError] = useState('');
  const [gitGraphOpen, setGitGraphOpen] = useState(false);
  const [copiedItem, setCopiedItem] = useState<string | null>(null);
  const [taskMenuOpen, setTaskMenuOpen] = useState(false);
  const taskMenuRef = useDropdown(() => setTaskMenuOpen(false));
  const [modeMenuOpen, setModeMenuOpen] = useState(false);
  const modeMenuRef = useDropdown(() => setModeMenuOpen(false));

  const copyText = async (key: string, text: string) => {
    try {
      await navigator.clipboard.writeText(text);
      setCopiedItem(key);
      window.setTimeout(() => setCopiedItem(cur => (cur === key ? null : cur)), 1500);
    } catch {
      // clipboard unavailable (permissions) — silently ignore
    }
  };

  const refreshGitAfterCheckout = () => {
    refreshGitStatus();
    refreshGitLog();
  };

  const openBranchMenu = async () => {
    const next = !branchMenuOpen;
    setBranchMenuOpen(next);
    if (next) {
      setBranchSearch('');
      setBranchError('');
      setCreatingBranch(false);
      setBranchesLoading(true);
      try {
        const list = await ApiBridge.getGitBranches(activeWorkspacePath || '');
        setBranches(Array.isArray(list) ? list : []);
      } catch {
        setBranches([]);
      } finally {
        setBranchesLoading(false);
      }
    }
  };

  const headerProjectBadge = (
    <button
      type="button"
      onClick={() => {
        if (activeWorkspacePath) ApiBridge.openInFinder(activeWorkspacePath);
      }}
      className="flex h-6.5 shrink-0 cursor-pointer items-center gap-1.5 rounded-md border border-border/40 bg-surface px-2 text-ui-xs font-medium text-foreground-subtle select-none transition-colors hover:bg-surface-hover hover:text-foreground"
      title={activeWorkspacePath ? `Reveal in Finder — ${activeWorkspacePath}` : projectName}
    >
      <Folder className="size-3.5 text-foreground-subtle" />
      <span className="max-w-36 truncate">{projectName}</span>
    </button>
  );

  const headerBranchButton = gitBranch ? (
    <div className="relative shrink-0" ref={branchMenuRef}>
      <button
        type="button"
        onClick={openBranchMenu}
        aria-haspopup="menu"
        aria-expanded={branchMenuOpen}
        aria-label={`Git branch: ${gitBranch}`}
        className="flex h-6.5 items-center gap-1.5 rounded-md border border-border/40 bg-surface px-2 text-ui-xs font-medium text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground"
        title="Git branch"
      >
        <GitBranch className="size-3.5 text-foreground-subtle" />
        <span className="font-mono text-ui-xs">{gitBranch}</span>
        <ChevronDown className="size-3 text-foreground-subtlest" />
      </button>
      {branchMenuOpen && (
        <div role="menu" aria-label="Git branches" className="absolute left-0 top-full z-50 mt-1.5 w-80 rounded-xl border border-border bg-popover p-2 shadow-lg">
          <div className="mb-1.5 flex items-center gap-2 rounded-lg border border-border bg-background px-2.5 py-1.5">
            <Search className="size-3.5 text-foreground-subtle" />
            <input
              type="text"
              value={branchSearch}
              onChange={e => setBranchSearch(e.target.value)}
              placeholder="Search branches"
              aria-label="Search branches"
              className="w-full border-0 bg-transparent font-mono text-xs text-foreground placeholder-foreground-subtlest outline-none"
              autoFocus
            />
          </div>

          {creatingBranch ? (
            <form
              onSubmit={async e => {
                e.preventDefault();
                const name = newBranchName.trim();
                if (!name) return;
                setBranchError('');
                try {
                  await ApiBridge.gitCreateBranch(activeWorkspacePath || '', name);
                  setCreatingBranch(false);
                  setNewBranchName('');
                  setBranchMenuOpen(false);
                  refreshGitAfterCheckout();
                } catch (err: any) {
                  setBranchError(err?.message || String(err));
                }
              }}
            >
              <input
                type="text"
                value={newBranchName}
                onChange={e => setNewBranchName(e.target.value)}
                onKeyDown={e => {
                  if (e.key === 'Escape') {
                    e.stopPropagation();
                    setCreatingBranch(false);
                    setNewBranchName('');
                    setBranchError('');
                  }
                }}
                placeholder="New branch name"
                aria-label="New branch name"
                className="w-full rounded-lg border border-border bg-background px-2.5 py-1.5 font-mono text-xs text-foreground placeholder-foreground-subtlest outline-none"
                autoFocus
              />
            </form>
          ) : null}

          {!creatingBranch && (
            <div className="px-2 py-1 text-ui-xs font-semibold uppercase text-foreground-subtlest">
              Branches
            </div>
          )}

          {branchError && (
            <p className="mb-1 break-words rounded-lg bg-destructive/10 px-2 py-1.5 text-ui-xs text-destructive" role="alert">
              {branchError}
            </p>
          )}

          {!creatingBranch && (
            <div className="max-h-64 space-y-0.5 overflow-y-auto">
              {branchesLoading ? (
                <p className="px-2 py-1.5 text-ui-sm text-foreground-subtle">Loading…</p>
              ) : branches.length === 0 ? (
                <p className="px-2 py-1.5 text-ui-sm text-foreground-subtlest">No branches</p>
              ) : (
                branches
                  .filter(b => b.toLowerCase().includes(branchSearch.toLowerCase()))
                  .map(b => {
                    const isCurrent = b === gitBranch;
                    return (
                      <button
                        key={b}
                        type="button"
                        role="menuitemradio"
                        aria-checked={isCurrent}
                        onClick={async () => {
                          if (isCurrent || !activeWorkspacePath) {
                            setBranchMenuOpen(false);
                            return;
                          }
                          setBranchError('');
                          try {
                            await ApiBridge.gitCheckout(activeWorkspacePath, b);
                            setBranchMenuOpen(false);
                            refreshGitAfterCheckout();
                          } catch (err: any) {
                            // Uncommitted changes etc. keep the menu open and show why.
                            setBranchError(err?.message || String(err));
                          }
                        }}
                        className={cn(
                          'flex w-full items-start justify-between gap-2 rounded-lg px-2 py-1.5 text-left transition-colors',
                          isCurrent
                            ? 'bg-surface-hover'
                            : 'hover:bg-surface-hover'
                        )}
                      >
                        <span className="flex min-w-0 items-start gap-2">
                          <GitBranch className="mt-0.5 size-3.5 shrink-0 text-foreground-subtle" />
                          <span className="min-w-0">
                            <span className={cn('block truncate font-mono text-ui-sm', isCurrent ? 'font-medium text-foreground' : 'text-foreground-subtle')}>
                              {b}
                            </span>
                            {isCurrent && gitFiles.length > 0 && (
                              <span className="mt-0.5 block truncate text-ui-xs text-foreground-subtlest">
                                Uncommitted changes: {gitFiles.length} files
                              </span>
                            )}
                          </span>
                        </span>
                        {isCurrent && <Check className="mt-0.5 size-3.5 shrink-0 text-success" />}
                      </button>
                    );
                  })
              )}
              {branches.length > 0 && branches.filter(b => b.toLowerCase().includes(branchSearch.toLowerCase())).length === 0 && !branchesLoading && (
                <p className="px-2 py-1.5 text-ui-sm text-foreground-subtlest">No branches found</p>
              )}
            </div>
          )}

          <div className="my-1.5 h-px bg-border" />
          <button
            type="button"
            onClick={() => {
              setCreatingBranch(true);
              setBranchError('');
            }}
            className="flex w-full items-center gap-2 rounded-lg px-2 py-1.5 text-left text-ui-sm text-foreground transition-colors hover:bg-surface-hover"
          >
            <Plus className="size-4 text-foreground-subtle" />
            Create and switch to new branch…
          </button>
          <button
            type="button"
            onClick={() => {
              setBranchMenuOpen(false);
              setGitGraphOpen(true);
            }}
            className="flex w-full items-center gap-2 rounded-lg px-2 py-1.5 text-left text-ui-sm text-foreground transition-colors hover:bg-surface-hover"
          >
            <Network className="size-4 text-foreground-subtle" />
            Git Graph
          </button>
        </div>
      )}
    </div>
  ) : null;

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

  const isAgentMode = mode === 'agent' || mode === 'terminal' || mode === 'automations';
  const ModeIcon = isAgentMode ? Sparkles : Code2;

  const switchToAgentMode = () => {
    if (isAgentMode) return;
    const def = useSessionStore.getState().appSettings.defaultMode;
    setMode(previousMode === 'terminal' || previousMode === 'agent' ? previousMode : (def === 'agent-ui' ? 'agent' : 'terminal'));
  };

  return (
    <header
      data-testid="workspace-header"
      className={cn(
        '@container/workspace-header relative flex h-12 w-full shrink-0 border-b border-border/50 bg-background select-none'
      )}
    >
      {/* ── Left Sidebar Header Area (aligns with sidebar width when open) ── */}
      {sidebarOpen ? (
        <div
          style={{ width: `${leftSidebarWidth}px` }}
          className="titlebar-drag flex h-12 shrink-0 items-center pl-[88px] pr-2.5 border-r border-border"
        >
          {onToggleSidebar && (
            <button
              type="button"
              onClick={onToggleSidebar}
              aria-label="Hide sidebar"
              title="Hide sidebar (⌘B)"
              className="no-drag-region flex size-7 items-center justify-center rounded-md text-foreground-subtle hover:bg-surface-hover hover:text-foreground transition-colors cursor-pointer"
            >
              <PanelLeft className="size-4" />
            </button>
          )}
        </div>
      ) : (
        <div className="titlebar-drag flex h-12 shrink-0 items-center pl-[88px] pr-2">
          {onToggleSidebar && (
            <button
              type="button"
              onClick={onToggleSidebar}
              aria-label="Show sidebar"
              title="Show sidebar (⌘B)"
              className="no-drag-region flex size-7 items-center justify-center rounded-md text-foreground-subtle hover:bg-surface-hover hover:text-foreground transition-colors cursor-pointer"
            >
              <PanelLeft className="size-4" />
            </button>
          )}
        </div>
      )}

      {/* ── Main Pane Header Area (Title → folder chip → branch → "…") ── */}
      <div className="titlebar-drag flex h-12 flex-1 min-w-0 items-center justify-between gap-3 px-3">
        <div className="flex min-w-0 items-center gap-2 no-drag-region">
          {title && title !== projectName ? (
            <span
              className="truncate text-ui-sm font-semibold text-foreground max-w-80"
              title={title}
            >
              {formatDisplayTitle(title)}
            </span>
          ) : null}
          {headerProjectBadge}
          {headerBranchButton}
          {headerTaskMenu}
        </div>

        {/* Action section */}
        <div className="flex shrink-0 items-center gap-1 no-drag-region">
          {/* Mode dropdown: Agent | Editor (compact, ZCode-style) */}
          <div className="relative shrink-0" ref={modeMenuRef}>
            <button
              type="button"
              onClick={() => setModeMenuOpen(v => !v)}
              aria-haspopup="menu"
              aria-expanded={modeMenuOpen}
              aria-label="Switch interface mode"
              title="Switch interface mode"
              className={cn(
                'flex h-7 items-center gap-1.5 rounded-lg border border-border bg-surface pl-2 pr-1.5 text-ui-xs font-medium transition-colors hover:bg-surface-hover',
                isAgentMode ? 'text-foreground' : 'text-foreground-subtle'
              )}
            >
              <ModeIcon className={cn('size-3.5', isAgentMode ? 'text-primary' : 'text-foreground-subtle')} />
              <span>{isAgentMode ? 'Agent' : 'Editor'}</span>
              <ChevronDown className="size-3 text-foreground-subtlest" />
            </button>

            {modeMenuOpen && (
              <div role="menu" aria-label="Interface mode" className="absolute right-0 top-full z-50 mt-1.5 w-44 rounded-xl border border-border bg-popover p-1 shadow-lg">
                <button
                  type="button"
                  role="menuitemradio"
                  aria-checked={isAgentMode}
                  onClick={() => {
                    setModeMenuOpen(false);
                    switchToAgentMode();
                  }}
                  className="flex w-full cursor-pointer items-center gap-2 rounded-lg px-2 py-1.5 text-left transition-colors hover:bg-surface-hover"
                >
                  <Sparkles className="size-3.5 text-primary" />
                  <span className="flex-1 text-ui-xs font-medium text-foreground">Agent</span>
                  {isAgentMode && <Check className="size-3.5 text-success" />}
                </button>
                <button
                  type="button"
                  role="menuitemradio"
                  aria-checked={mode === 'editor'}
                  onClick={() => {
                    setModeMenuOpen(false);
                    setMode('editor');
                  }}
                  className="flex w-full cursor-pointer items-center gap-2 rounded-lg px-2 py-1.5 text-left transition-colors hover:bg-surface-hover"
                >
                  <Code2 className="size-3.5 text-foreground-subtle" />
                  <span className="flex-1 text-ui-xs font-medium text-foreground">Editor</span>
                  {mode === 'editor' && <Check className="size-3.5 text-success" />}
                </button>
              </div>
            )}
          </div>

          <button
            type="button"
            onClick={() => setIsCommandPaletteOpen(true)}
            className="flex h-8 w-8 items-center justify-center rounded-lg text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground cursor-pointer"
            title="Help & Shortcuts (⌘K)"
            aria-label="Help and shortcuts"
          >
            <HelpCircle className="size-4" />
          </button>
          {/* Bottom dock terminal toggle (only in agent task view, NOT in terminal mode) */}
          {variant === 'task' && mode !== 'terminal' && onToggleTerminal ? (
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
        </div>
      </div>

      {/* Git Graph modal (branch menu entry) */}
      <GitGraphModal open={gitGraphOpen} onClose={() => setGitGraphOpen(false)} />
    </header>
  );
};

// Re-export for callers that rendered a MessageSquarePlus icon next to the title.
export { MessageSquarePlus };
