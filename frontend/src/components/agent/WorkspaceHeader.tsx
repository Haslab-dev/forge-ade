import React, { useState, useRef, useEffect } from 'react';
import {
  Folder,
  Ellipsis,
  HelpCircle,
  SquareTerminal,
  PanelRight,
  GitBranch,
  MessageSquarePlus,
  Pencil,
  Archive,
  Trash2,
  Check,
  ChevronDown,
  Search,
  Plus,
  SquarePen,
  Code2
} from 'lucide-react';
import { cn } from '../../lib/utils';
import { useWorkspace } from '../../stores/workspaceStore';
import { ApiBridge } from '../../services/apiBridge';

/**
 * Workspace header ported from ZCode WorkspaceHeader: h-12 bar with
 * `border-transparent` in the draft variant and `border-border/50` in the
 * task variant. Title section: project finder dropdown, task title, branch
 * switcher dropdown, ellipsis menu. Action section: mode dropdown
 * (Agent/Editor), help, terminal toggle, side-pane toggle (rendered only
 * while the side pane is closed).
 */

interface WorkspaceHeaderProps {
  title: string;
  projectName: string;
  gitBranch?: string;
  changesPill?: React.ReactNode;
  /** draft = new-task view (no border), task = active session. */
  variant?: 'draft' | 'task';
  onToggleTerminal?: () => void;
  isSidePaneOpen?: boolean;
  onToggleSidePane?: () => void;
  onRenameSession?: () => void;
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
  changesPill,
  variant = 'draft',
  onToggleTerminal,
  isSidePaneOpen = false,
  onToggleSidePane,
  onRenameSession
}) => {
  const {
    mode,
    setMode,
    isRightActionDrawerOpen,
    setIsRightActionDrawerOpen,
    deleteSessionPermanently,
    activeSessionId,
    setIsCommandPaletteOpen,
    activeWorkspacePath,
    setActiveWorkspacePath,
    recentWorkspaces,
    openFolder
  } = useWorkspace();

  const [projectMenuOpen, setProjectMenuOpen] = useState(false);
  const [workspaceSearch, setWorkspaceSearch] = useState('');
  const projectMenuRef = useDropdown(() => setProjectMenuOpen(false));
  const [branchMenuOpen, setBranchMenuOpen] = useState(false);
  const branchMenuRef = useDropdown(() => setBranchMenuOpen(false));
  const [branches, setBranches] = useState<string[]>([]);
  const [branchesLoading, setBranchesLoading] = useState(false);
  const [modeMenuOpen, setModeMenuOpen] = useState(false);
  const modeMenuRef = useDropdown(() => setModeMenuOpen(false));
  const [taskMenuOpen, setTaskMenuOpen] = useState(false);
  const taskMenuRef = useDropdown(() => setTaskMenuOpen(false));

  const filteredWorkspaces = recentWorkspaces.filter(ws => {
    const name = ws.split('/').filter(Boolean).pop() || ws;
    return name.toLowerCase().includes(workspaceSearch.toLowerCase());
  });

  const openBranchMenu = async () => {
    const next = !branchMenuOpen;
    setBranchMenuOpen(next);
    setProjectMenuOpen(false);
    if (next) {
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

  const headerProjectButton = (
    <div className="relative shrink-0" ref={projectMenuRef}>
      <button
        type="button"
        onClick={() => {
          setProjectMenuOpen(v => !v);
          setBranchMenuOpen(false);
          setModeMenuOpen(false);
        }}
        className="flex h-8 items-center gap-1.5 rounded-lg px-2 text-ui-sm font-medium text-foreground transition-colors hover:bg-surface-hover"
        title={activeWorkspacePath || projectName}
      >
        <Folder className="size-4 text-foreground-subtle" />
        <span className="max-w-40 truncate">{projectName}</span>
        <ChevronDown className="size-3.5 text-foreground-subtle" />
      </button>
      {projectMenuOpen && (
        <div className="absolute left-0 top-full z-50 mt-1 w-64 rounded-xl border border-border bg-popover p-2 shadow-lg">
          <div className="mb-2 flex items-center gap-2 rounded-lg border border-border bg-background px-2.5 py-1.5">
            <Search className="size-3.5 text-foreground-subtle" />
            <input
              type="text"
              value={workspaceSearch}
              onChange={e => setWorkspaceSearch(e.target.value)}
              placeholder="Search projects"
              className="w-full border-0 bg-transparent font-mono text-xs text-foreground placeholder-foreground-subtlest outline-none"
              autoFocus
            />
          </div>
          <div className="max-h-52 space-y-0.5 overflow-y-auto">
            {filteredWorkspaces.map(ws => {
              const name = ws.split('/').filter(Boolean).pop() || ws;
              const isCurrent = ws === activeWorkspacePath || name === projectName;
              return (
                <button
                  key={ws}
                  type="button"
                  onClick={() => {
                    setActiveWorkspacePath(ws);
                    setProjectMenuOpen(false);
                  }}
                  className={cn(
                    'flex w-full items-center justify-between gap-2 rounded-lg px-2 py-1.5 text-left text-ui-sm transition-colors',
                    isCurrent
                      ? 'bg-surface-hover font-medium text-foreground'
                      : 'text-foreground-subtle hover:bg-surface-hover hover:text-foreground'
                  )}
                >
                  <span className="flex min-w-0 items-center gap-2">
                    <Folder className="size-3.5 shrink-0 text-foreground-subtle" />
                    <span className="truncate">{name}</span>
                  </span>
                  {isCurrent && <Check className="size-3.5 shrink-0 text-success" />}
                </button>
              );
            })}
            {filteredWorkspaces.length === 0 && (
              <p className="px-2 py-1.5 text-ui-sm text-foreground-subtlest">No projects found</p>
            )}
          </div>
          <div className="my-1 h-px bg-border" />
          <button
            type="button"
            onClick={() => {
              setProjectMenuOpen(false);
              openFolder();
            }}
            className="flex w-full items-center gap-2 rounded-lg px-2 py-1.5 text-left text-ui-sm text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground"
          >
            <Plus className="size-3.5" />
            Open local folder
          </button>
        </div>
      )}
    </div>
  );

  const headerBranchButton = gitBranch ? (
    <div className="relative shrink-0" ref={branchMenuRef}>
      <button
        type="button"
        onClick={openBranchMenu}
        className="flex h-8 items-center gap-1.5 rounded-lg px-2 text-ui-sm text-foreground-subtlest transition-colors hover:bg-surface-hover hover:text-foreground"
        title="Git branch"
      >
        <GitBranch className="size-3.5" />
        <span className="font-mono">{gitBranch}</span>
        <ChevronDown className="size-3" />
      </button>
      {branchMenuOpen && (
        <div className="absolute left-0 top-full z-50 mt-1 w-48 rounded-xl border border-border bg-popover p-1 shadow-lg">
          <div className="px-2 py-1 text-ui-xs font-semibold uppercase text-foreground-subtlest">
            Git Branches
          </div>
          {branchesLoading ? (
            <p className="px-2 py-1.5 text-ui-sm text-foreground-subtle">Loading…</p>
          ) : branches.length === 0 ? (
            <p className="px-2 py-1.5 text-ui-sm text-foreground-subtlest">No branches</p>
          ) : (
            branches.map(b => {
              const isCurrent = b === gitBranch;
              return (
                <button
                  key={b}
                  type="button"
                  onClick={async () => {
                    setBranchMenuOpen(false);
                    if (isCurrent || !activeWorkspacePath) return;
                    try {
                      await ApiBridge.gitCheckout(activeWorkspacePath, b);
                    } catch (e: any) {
                      alert(e?.message || String(e));
                    }
                  }}
                  className="flex w-full items-center justify-between gap-2 rounded-lg px-2 py-1.5 text-left font-mono text-ui-sm text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground"
                >
                  <span className="truncate">{b}</span>
                  {isCurrent && <Check className="size-3.5 shrink-0 text-success" />}
                </button>
              );
            })
          )}
        </div>
      )}
    </div>
  ) : null;

  const headerTaskMenu = activeSessionId ? (
    <div className="relative flex min-w-0 shrink-0 items-center" ref={taskMenuRef}>
      <button
        type="button"
        onClick={() => setTaskMenuOpen(v => !v)}
        className="flex h-8 w-8 items-center justify-center rounded-lg text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground"
        aria-label="More"
      >
        <Ellipsis className="size-4" />
      </button>
      {taskMenuOpen && (
        <div className="absolute left-0 top-full z-50 mt-1 w-52 rounded-xl border border-border bg-popover p-1 text-ui-sm shadow-lg">
          <button
            type="button"
            onClick={() => {
              setTaskMenuOpen(false);
              onRenameSession?.();
            }}
            className="flex w-full items-center gap-2 rounded-lg px-2 py-1.5 text-left text-foreground transition-colors hover:bg-surface-hover"
          >
            <Pencil className="size-4 text-foreground-subtle" />
            Rename task
          </button>
          <button
            type="button"
            onClick={() => setTaskMenuOpen(false)}
            className="flex w-full items-center gap-2 rounded-lg px-2 py-1.5 text-left text-foreground transition-colors hover:bg-surface-hover"
          >
            <Archive className="size-4 text-foreground-subtle" />
            Archive task
          </button>
          <div className="mx-1 my-1 h-px bg-border" />
          <button
            type="button"
            onClick={() => {
              setTaskMenuOpen(false);
              if (activeSessionId) deleteSessionPermanently(activeSessionId);
            }}
            className="flex w-full items-center gap-2 rounded-lg px-2 py-1.5 text-left text-destructive transition-colors hover:bg-destructive/10"
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
        '@container/workspace-header relative flex h-12 w-full shrink-0 border-b',
        variant === 'draft' ? 'border-transparent' : 'border-border/50'
      )}
    >
      <div className="titlebar-drag flex h-12 flex-1 min-w-0 items-center justify-between gap-2 p-2">
        {/* Title section (overflow visible so dropdowns can escape the bar) */}
        <div className="flex min-w-0 items-center gap-2">
          {headerProjectButton}
          {title ? (
            <h1
              className="flex min-w-12 max-w-100 shrink items-center gap-2 truncate text-ui-base font-semibold text-foreground"
              title={title}
            >
              <span className="min-w-0 truncate">{title}</span>
            </h1>
          ) : null}
          {headerBranchButton}
          {headerTaskMenu}
        </div>

        {/* Action section */}
        <div className="flex shrink-0 items-center gap-0.5">
          {changesPill}
          {/* Mode dropdown: Agent | Editor */}
          <div className="relative mr-1" ref={modeMenuRef}>
            <button
              type="button"
              onClick={() => {
                setModeMenuOpen(v => !v);
                setProjectMenuOpen(false);
              }}
              className="flex h-8 items-center gap-1.5 rounded-lg border border-border bg-surface px-2.5 text-ui-sm font-medium text-foreground transition-colors hover:bg-surface-hover"
              title="Switch workspace mode"
            >
              {mode === 'agent' ? (
                <SquarePen className="size-3.5 text-foreground-subtle" />
              ) : (
                <Code2 className="size-3.5 text-foreground-subtle" />
              )}
              <span className="capitalize">{mode}</span>
              <ChevronDown className="size-3 text-foreground-subtle" />
            </button>
            {modeMenuOpen && (
              <div className="absolute right-0 top-full z-50 mt-1 w-40 rounded-xl border border-border bg-popover p-1 shadow-lg">
                <button
                  type="button"
                  onClick={() => {
                    setMode('agent');
                    setModeMenuOpen(false);
                  }}
                  className="flex w-full items-center justify-between gap-2 rounded-lg px-2 py-1.5 text-left text-ui-sm text-foreground transition-colors hover:bg-surface-hover"
                >
                  <span className="flex items-center gap-2">
                    <SquarePen className="size-3.5 text-foreground-subtle" />
                    Agent
                  </span>
                  {mode === 'agent' && <Check className="size-3.5 text-foreground" />}
                </button>
                <button
                  type="button"
                  onClick={() => {
                    setMode('editor');
                    setModeMenuOpen(false);
                  }}
                  className="flex w-full items-center justify-between gap-2 rounded-lg px-2 py-1.5 text-left text-ui-sm text-foreground transition-colors hover:bg-surface-hover"
                >
                  <span className="flex items-center gap-2">
                    <Code2 className="size-3.5 text-foreground-subtle" />
                    Editor
                  </span>
                  {mode === 'editor' && <Check className="size-3.5 text-foreground" />}
                </button>
              </div>
            )}
          </div>
          <button
            type="button"
            onClick={() => setIsCommandPaletteOpen(true)}
            className="flex h-8 w-8 items-center justify-center rounded-lg text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground"
            title="Help"
          >
            <HelpCircle className="size-4" />
          </button>
          {variant === 'task' && onToggleTerminal ? (
            <button
              type="button"
              onClick={onToggleTerminal}
              className="flex h-8 w-8 items-center justify-center rounded-lg text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground"
              title="Terminal"
            >
              <SquareTerminal className="size-4" />
            </button>
          ) : null}
          {/* Side-pane toggle renders only while the pane is closed; the pane
              owns its own close action once open (ZCode behavior). */}
          {variant === 'task' && !isSidePaneOpen && onToggleSidePane ? (
            <button
              type="button"
              onClick={() => onToggleSidePane()}
              className="flex h-8 w-8 items-center justify-center rounded-lg text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground"
              title="Toggle side pane"
            >
              <PanelRight className="size-4" />
            </button>
          ) : null}
        </div>
      </div>
    </header>
  );
};

// Re-export for callers that rendered a MessageSquarePlus icon next to the title.
export { MessageSquarePlus };
