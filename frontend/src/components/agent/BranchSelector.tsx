import React, { useState, useRef, useEffect } from 'react';
import { Check, ChevronDown, GitBranch, Network, Plus, Search } from 'lucide-react';
import { cn } from '../../lib/utils';
import { useWorkspace } from '../../stores/workspaceStore';
import { ApiBridge } from '../../services/apiBridge';

function useDropdownDismiss(onClose: () => void) {
  const ref = useRef<HTMLDivElement | null>(null);
  useEffect(() => {
    const onPointerDown = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) onClose();
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
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

export interface BranchSelectorProps {
  /**
   * "header": toolbar chip next to the workspace badge, menu opens down.
   * "composer": t3 context chip for the composer strip, menu opens up.
   */
  variant?: 'header' | 'composer';
  align?: 'start' | 'end';
  className?: string;
}

/**
 * Git branch selector — the t3 BranchToolbar chip cloned onto forge tokens:
 * small branch icon, middle-safe truncating label, dim chevron, glass menu
 * with the underline search, branch rows and the create/switch actions.
 * Self-contained: owns its menu state, loads branches on open.
 */
export const BranchSelector: React.FC<BranchSelectorProps> = ({
  variant = 'header',
  align = 'start',
  className,
}) => {
  const { activeWorkspacePath, gitBranch, gitFiles, refreshGitStatus, refreshGitLog } =
    useWorkspace();

  const [open, setOpen] = useState(false);
  const [branches, setBranches] = useState<string[]>([]);
  const [branchesLoading, setBranchesLoading] = useState(false);
  const [branchSearch, setBranchSearch] = useState('');
  const [creatingBranch, setCreatingBranch] = useState(false);
  const [newBranchName, setNewBranchName] = useState('');
  const [branchError, setBranchError] = useState('');
  const [gitGraphOpen, setGitGraphOpen] = useState(false);
  const searchInputRef = useRef<HTMLInputElement | null>(null);

  const menuRef = useDropdownDismiss(() => {
    setOpen(false);
    setCreatingBranch(false);
    setBranchError('');
  });

  const isGitWorkspace = Boolean(
    activeWorkspacePath && !activeWorkspacePath.startsWith('ssh://') && gitBranch,
  );

  const openMenu = async () => {
    const next = !open;
    setOpen(next);
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

  const refreshAfterCheckout = () => {
    refreshGitStatus();
    refreshGitLog();
  };

  useEffect(() => {
    if (open) {
      window.setTimeout(() => searchInputRef.current?.focus(), 50);
    }
  }, [open, creatingBranch]);

  if (!isGitWorkspace) return null;

  const filteredBranches = branches.filter(b =>
    b.toLowerCase().includes(branchSearch.toLowerCase()),
  );

  return (
    <div className={cn('relative shrink-0 select-none', className)} ref={menuRef}>
      <button
        type="button"
        onClick={() => void openMenu()}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label={`Git branch: ${gitBranch}`}
        className={cn(
          'inline-flex h-7 cursor-pointer items-center gap-1 rounded-md border border-transparent px-1.75 font-normal text-muted-foreground/70 transition-colors hover:bg-accent hover:text-foreground/80 sm:h-6',
          open && 'bg-accent text-foreground',
        )}
        title="Git branch"
      >
        <GitBranch className="size-3 shrink-0 opacity-70" />
        <span className="max-w-[180px] truncate">{gitBranch}</span>
        <ChevronDown className="size-3 shrink-0 opacity-50" />
      </button>

      {open && (
        <div
          role="menu"
          aria-label="Git branches"
          className={cn(
            'dropdown-glass z-50 w-80 animate-in rounded-lg p-1 text-ui-sm text-fg-primary shadow-[0_16px_40px_-18px_rgb(0_0_0/55%)] fade-in zoom-in-95 duration-100',
            variant === 'composer'
              ? align === 'end'
                ? 'absolute bottom-full right-0 mb-2'
                : 'absolute bottom-full left-0 mb-2'
              : align === 'end'
                ? 'absolute right-0 top-full mt-1.5'
                : 'absolute left-0 top-full mt-1.5',
          )}
        >
          <div className="border-b border-border-subtle px-3 pt-2.5 pb-1.5">
            <div className="relative -translate-y-px transition-colors focus-within:border-ring">
              <Search className="absolute left-0 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground/55" />
              <input
                ref={searchInputRef}
                type="text"
                value={branchSearch}
                onChange={e => setBranchSearch(e.target.value)}
                placeholder="Search branches"
                aria-label="Search branches"
                className="h-6.5 w-full border-0 bg-transparent pl-5 font-sans text-ui-sm text-fg-primary placeholder-fg-tertiary outline-none"
              />
            </div>
          </div>

          {creatingBranch ? (
            <div className="p-1.5">
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
                    setOpen(false);
                    refreshAfterCheckout();
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
                  className="w-full rounded-md border border-border-default bg-surface px-2.5 py-1.5 font-mono text-ui-xs text-fg-primary placeholder-fg-tertiary outline-none focus:border-accent-primary"
                  autoFocus
                />
              </form>
            </div>
          ) : (
            <div className="px-2 py-1 text-2xs font-semibold uppercase tracking-wider text-fg-tertiary">
              Branches
            </div>
          )}

          {branchError && (
            <p
              className="mb-1 break-words rounded-md bg-error-surface px-2 py-1.5 text-ui-xs text-destructive"
              role="alert"
            >
              {branchError}
            </p>
          )}

          {!creatingBranch && (
            <div className="max-h-64 space-y-0.5 overflow-y-auto p-1 pt-0">
              {branchesLoading ? (
                <p className="px-2 py-1.5 text-ui-sm text-fg-tertiary">Loading…</p>
              ) : branches.length === 0 ? (
                <p className="px-2 py-1.5 text-ui-sm text-fg-tertiary">No branches</p>
              ) : (
                filteredBranches.map(b => {
                  const isCurrent = b === gitBranch;
                  return (
                    <button
                      key={b}
                      type="button"
                      role="menuitemradio"
                      aria-checked={isCurrent}
                      onClick={async () => {
                        if (isCurrent || !activeWorkspacePath) {
                          setOpen(false);
                          return;
                        }
                        setBranchError('');
                        try {
                          await ApiBridge.gitCheckout(activeWorkspacePath, b);
                          setOpen(false);
                          refreshAfterCheckout();
                        } catch (err: any) {
                          // Uncommitted changes etc. keep the menu open and show why.
                          setBranchError(err?.message || String(err));
                        }
                      }}
                      className={cn(
                        'flex min-h-7 w-full items-start justify-between gap-2 rounded-sm px-2 py-1 text-left transition-colors hover:bg-accent',
                        isCurrent && 'bg-foreground/[0.08]',
                      )}
                    >
                      <span className="flex min-w-0 items-start gap-2">
                        <GitBranch className="mt-0.5 size-3.5 shrink-0 text-muted-foreground" />
                        <span className="min-w-0">
                          <span
                            className={cn(
                              'block truncate text-ui-sm',
                              isCurrent ? 'font-medium text-fg-primary' : 'text-fg-secondary',
                            )}
                          >
                            {b}
                          </span>
                          {isCurrent && gitFiles.length > 0 && (
                            <span className="mt-0.5 block truncate text-ui-xs text-fg-tertiary">
                              Uncommitted changes: {gitFiles.length} files
                            </span>
                          )}
                        </span>
                      </span>
                      {isCurrent && <Check className="mt-0.5 size-3.5 shrink-0 text-fg-primary" />}
                    </button>
                  );
                })
              )}
              {branches.length > 0 && filteredBranches.length === 0 && !branchesLoading && (
                <p className="px-2 py-1.5 text-ui-sm text-fg-tertiary">No branches found</p>
              )}
            </div>
          )}

          <div className="mx-1 my-1 h-px bg-border-subtle" />
          <button
            type="button"
            onClick={() => {
              setCreatingBranch(true);
              setBranchError('');
            }}
            className="flex min-h-7 w-full items-center gap-2 rounded-sm px-2 py-1 text-left text-ui-sm text-fg-primary transition-colors hover:bg-accent"
          >
            <Plus className="size-4 text-muted-foreground" />
            Create and switch to new branch…
          </button>
          <button
            type="button"
            onClick={() => {
              setOpen(false);
              setGitGraphOpen(true);
            }}
            className="flex min-h-7 w-full items-center gap-2 rounded-sm px-2 py-1 text-left text-ui-sm text-fg-primary transition-colors hover:bg-accent"
          >
            <Network className="size-4 text-muted-foreground" />
            Git Graph
          </button>
        </div>
      )}

      <GitGraphModalLazy open={gitGraphOpen} onClose={() => setGitGraphOpen(false)} />
    </div>
  );
};

/** Deferred so the heavy graph view only loads when actually opened. */
const GitGraphModalLazy: React.FC<{ open: boolean; onClose: () => void }> = props => {
  const [Modal, setModal] = useState<React.ComponentType<{
    open: boolean;
    onClose: () => void;
  }> | null>(null);
  useEffect(() => {
    if (!open || Modal) return;
    let active = true;
    import('../modals/GitGraphModal').then(m => {
      if (active) setModal(() => m.GitGraphModal);
    });
    return () => {
      active = false;
    };
  }, [open, Modal]);
  if (!Modal) return null;
  return <Modal {...props} />;
};
