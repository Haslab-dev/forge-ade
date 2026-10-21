import React, { useState, useRef, useEffect, useMemo } from 'react';
import {
  Laptop,
  Server,
  Check,
  X,
  ExternalLink,
  FolderOpen,
  ChevronDown,
  Trash2
} from 'lucide-react';
import { ApiBridge } from '../../services/apiBridge';
import { useWorkspace } from '../../stores/workspaceStore';
import type { WorkspaceEntry } from '../../types';
import {
  ComposerChevron,
  ComposerControl,
  ComposerMenu,
  ComposerMenuLabel,
  ComposerMenuSearch,
} from '../common/composer-ui';
import { cn } from '../../lib/utils';

interface WorkspaceSwitcherDropdownProps {
  className?: string;
  /**
   * "header" (default): toolbar chip in the workspace header, menu opens down.
   * "composer": t3 context chip for the composer footer tray, menu opens up.
   */
  variant?: 'header' | 'composer';
}

export const WorkspaceSwitcherDropdown: React.FC<WorkspaceSwitcherDropdownProps> = ({ className = '', variant = 'header' }) => {
  const {
    activeWorkspacePath,
    agentWorkspacePath,
    editorWorkspacePath,
    activeWorkspaces,
    recentWorkspaces,
    switchWorkspace,
    closeWorkspaceFromWindow,
    removeRecentWorkspace,
    openWorkspaceInNewWindow,
    openFolder,
    setIsSSHModalOpen,
    mode,
    activeTaskKind
  } = useWorkspace();

  // Remote (SSH) workspaces are hidden while the internal ForgeADE agent is
  // the active surface — it operates on the local machine only.
  const forgeActive = mode === 'agent' && activeTaskKind === 'forge';

  const [isOpen, setIsOpen] = useState(false);
  const [search, setSearch] = useState('');
  const [connectedSSHIds, setConnectedSSHIds] = useState<Set<string>>(new Set());
  const dropdownRef = useRef<HTMLDivElement | null>(null);
  const searchInputRef = useRef<HTMLInputElement | null>(null);

  // Poll / refresh active SSH connections when dropdown opens or on mount
  useEffect(() => {
    const refreshSSH = () => {
      ApiBridge.listSSHConnections()
        .then(conns => {
          const connected = new Set(
            (conns || []).filter(c => c.connected).map(c => c.id)
          );
          setConnectedSSHIds(connected);
        })
        .catch(() => {});
    };
    refreshSSH();
    if (isOpen) {
      refreshSSH();
    }
  }, [isOpen]);

  // Close on click outside or Escape
  useEffect(() => {
    const onPointerDown = (e: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setIsOpen(false);
      }
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setIsOpen(false);
    };
    if (isOpen) {
      document.addEventListener('mousedown', onPointerDown);
      document.addEventListener('keydown', onKey);
      window.setTimeout(() => searchInputRef.current?.focus(), 50);
    }
    return () => {
      document.removeEventListener('mousedown', onPointerDown);
      document.removeEventListener('keydown', onKey);
    };
  }, [isOpen]);

  // Current active workspace metadata
  const currentWorkspace: WorkspaceEntry = useMemo(() => {
    const isRemote = activeWorkspacePath.startsWith('ssh://');
    if (isRemote) {
      const rest = activeWorkspacePath.replace(/^ssh:\/\//, '');
      const slashIdx = rest.indexOf('/');
      const hostPart = slashIdx !== -1 ? rest.substring(0, slashIdx) : rest;
      const remoteDir = slashIdx !== -1 ? rest.substring(slashIdx) : '/';
      const folderName = remoteDir.split('/').filter(Boolean).pop() || 'remote-root';
      const cleanHost = hostPart.includes('@') ? hostPart.split('@')[1] : hostPart;
      return {
        path: activeWorkspacePath,
        name: folderName,
        type: 'remote',
        host: cleanHost.split(':')[0],
        lastActive: Date.now()
      };
    }
    const clean = activeWorkspacePath.replace(/\/+$/, '');
    const isGen = !clean || clean === 'general' || clean.endsWith('/.forge-ade/general');
    const folderName = isGen ? 'General' : (clean.split('/').filter(Boolean).pop() || 'General');
    return {
      path: activeWorkspacePath || '',
      name: folderName,
      type: 'local',
      lastActive: Date.now()
    };
  }, [activeWorkspacePath]);

  // Combined thisWindow entries ensuring current is included & disconnected SSH remotes are hidden
  const windowWorkspaces: WorkspaceEntry[] = useMemo(() => {
    if (!activeWorkspacePath && activeWorkspaces.length === 0) return [];
    const list = [...activeWorkspaces];
    if (activeWorkspacePath && !list.some(w => w.path === activeWorkspacePath)) {
      list.unshift(currentWorkspace);
    }
    // Filter out remote workspaces that are not connected (unless currently active);
    // hide all remote entries entirely while the ForgeADE agent is active.
    return list.filter(w => {
      if (w.type === 'remote') {
        if (forgeActive) return false;
        if (w.path === activeWorkspacePath) return true;
        const connId = w.path.replace(/^ssh:\/\//, '').split('/')[0];
        return connectedSSHIds.has(connId);
      }
      return true;
    });
  }, [activeWorkspaces, activeWorkspacePath, currentWorkspace, connectedSSHIds, forgeActive]);

  // Parse recent entries into WorkspaceEntry (filter out disconnected remote workspaces)
  const parsedRecentWorkspaces: WorkspaceEntry[] = useMemo(() => {
    const windowPaths = new Set(windowWorkspaces.map(w => w.path));
    return recentWorkspaces
      .filter(p => !windowPaths.has(p))
      .filter(p => {
        // Disconnected SSH remote should not be showed in recent projects;
        // no remote entries at all while the ForgeADE agent is active.
        if (p.startsWith('ssh://')) {
          if (forgeActive) return false;
          const connId = p.replace(/^ssh:\/\//, '').split('/')[0];
          return connectedSSHIds.has(connId);
        }
        return true;
      })
      .map(p => {
        const isRemote = p.startsWith('ssh://');
        if (isRemote) {
          const rest = p.replace(/^ssh:\/\//, '');
          const slashIdx = rest.indexOf('/');
          const hostPart = slashIdx !== -1 ? rest.substring(0, slashIdx) : rest;
          const remoteDir = slashIdx !== -1 ? rest.substring(slashIdx) : '/';
          const folderName = remoteDir.split('/').filter(Boolean).pop() || 'remote';
          const cleanHost = hostPart.includes('@') ? hostPart.split('@')[1] : hostPart;
          return {
            path: p,
            name: folderName,
            type: 'remote' as const,
            host: cleanHost.split(':')[0],
            lastActive: 0
          };
        }
        const clean = p.replace(/\/+$/, '');
        const folderName = clean.split('/').filter(Boolean).pop() || p;
        return {
          path: p,
          name: folderName,
          type: 'local' as const,
          lastActive: 0
        };
      });
  }, [recentWorkspaces, windowWorkspaces, connectedSSHIds, forgeActive]);

  // Search filter
  const query = search.trim().toLowerCase();
  const filteredWindowWorkspaces = useMemo(() => {
    if (!query) return windowWorkspaces;
    return windowWorkspaces.filter(w =>
      (w?.name || '').toLowerCase().includes(query) ||
      (w?.path || '').toLowerCase().includes(query) ||
      (w?.host && w.host.toLowerCase().includes(query))
    );
  }, [windowWorkspaces, query]);

  const filteredRecentWorkspaces = useMemo(() => {
    if (!query) return parsedRecentWorkspaces;
    return parsedRecentWorkspaces.filter(w =>
      (w?.name || '').toLowerCase().includes(query) ||
      (w?.path || '').toLowerCase().includes(query) ||
      (w?.host && w.host.toLowerCase().includes(query))
    );
  }, [parsedRecentWorkspaces, query]);

  const formatPathForTooltip = (p: string) => {
    if (!p) return '';
    return p.replace(/^\/Users\/[^/]+/, '~');
  };

  return (
    <div className={`relative shrink-0 select-none ${className}`} ref={dropdownRef}>
      {/* ── Trigger chip (t3 toolbar-control recipe) ── */}
      {variant === 'composer' ? (
        <ComposerControl
          size="xs"
          onClick={() => {
            setIsOpen(prev => !prev);
            setSearch('');
          }}
          aria-haspopup="menu"
          aria-expanded={isOpen}
          title={activeWorkspacePath ? `Switch Workspace — ${activeWorkspacePath}` : 'Open Workspace'}
          className="max-w-[220px] justify-start"
          pressed={isOpen}
        >
          {currentWorkspace.type === 'remote' ? (
            <Server className="size-3 shrink-0 opacity-70" />
          ) : (
            <Laptop className="size-3 shrink-0 opacity-70" />
          )}
          <span className="max-w-[160px] truncate">{currentWorkspace.name}</span>
          <ComposerChevron size="xs" />
        </ComposerControl>
      ) : (
        <button
          type="button"
          onClick={() => {
            setIsOpen(prev => !prev);
            setSearch('');
          }}
          aria-haspopup="menu"
          aria-expanded={isOpen}
          title={activeWorkspacePath ? `Switch Workspace — ${activeWorkspacePath}` : 'Open Workspace'}
          className="flex h-7 cursor-pointer items-center gap-1.5 rounded-md border border-transparent px-2 text-ui-xs font-medium text-secondary-label transition-colors hover:bg-accent hover:text-foreground"
        >
          {currentWorkspace.type === 'remote' ? (
            <Server className="size-3.5 shrink-0 text-muted-foreground" />
          ) : (
            <Laptop className="size-3.5 shrink-0 text-muted-foreground" />
          )}

          {currentWorkspace.host && (
            <span className="max-w-24 truncate font-mono text-ui-xs font-semibold text-fg-primary">
              {currentWorkspace.host}
            </span>
          )}

          <span className="max-w-32 truncate text-ui-xs text-fg-primary font-medium">
            {currentWorkspace.name}
          </span>

          <ChevronDown className="size-3 text-icon-muted" />
        </button>
      )}

      {/* ── Popover Menu (t3 glass panel + combobox rows) ── */}
      {isOpen && (
        <ComposerMenu
          label="Workspace Switcher"
          className={cn(
            'w-84 overflow-hidden',
            variant === 'composer'
              ? 'absolute bottom-full left-0 mb-2'
              : 'absolute left-0 top-full mt-1.5',
          )}
        >
          {/* Search projects input */}
          <ComposerMenuSearch
            value={search}
            onChange={setSearch}
            placeholder="Search projects..."
            inputRef={searchInputRef}
          />

          <div className="max-h-80 space-y-2 overflow-y-auto py-1 pr-0.5">
            {/* Section: This Window */}
            {filteredWindowWorkspaces.length > 0 && (
              <div>
                <ComposerMenuLabel>This Window</ComposerMenuLabel>
                <div className="space-y-0.5">
                  {filteredWindowWorkspaces.map(ws => {
                    const isActive = ws.path === activeWorkspacePath;
                    return (
                      <div
                        key={ws.path}
                        className={cn(
                          'group relative flex min-h-7 cursor-pointer items-center justify-between rounded-sm px-2 py-1 text-left text-ui-base transition-colors',
                          isActive
                            ? 'bg-foreground/[0.08] font-medium text-fg-primary'
                            : 'text-fg-secondary hover:bg-accent hover:text-fg-primary',
                        )}
                        onClick={() => {
                          setIsOpen(false);
                          void switchWorkspace(ws.path);
                        }}
                      >
                        <div className="flex min-w-0 flex-1 truncate items-center gap-2 pr-2">
                          {ws.type === 'remote' ? (
                            <Server className="size-3.5 shrink-0 text-muted-foreground" />
                          ) : (
                            <Laptop className="size-3.5 shrink-0 text-muted-foreground" />
                          )}
                          <span className="truncate">
                            {ws.host ? `${ws.name} (${ws.host})` : ws.name}
                          </span>
                        </div>

                        <div className="flex shrink-0 items-center gap-1">
                          {isActive && (
                            <Check className="size-3.5 shrink-0 text-fg-primary" />
                          )}

                          {/* Action: Open in new native window */}
                          <button
                            type="button"
                            onClick={(e) => {
                              e.stopPropagation();
                              void openWorkspaceInNewWindow(ws.path);
                            }}
                            title="Open in New Window"
                            className="cursor-pointer rounded p-0.5 text-fg-secondary opacity-0 transition-all hover:bg-accent hover:text-fg-primary group-hover:opacity-100"
                          >
                            <ExternalLink className="size-3" />
                          </button>

                          {/* Action: Close / Detach workspace from this window */}
                          <button
                            type="button"
                            onClick={(e) => {
                              e.stopPropagation();
                              void closeWorkspaceFromWindow(ws.path);
                            }}
                            title="Close Workspace"
                            className="cursor-pointer rounded p-0.5 text-fg-secondary opacity-0 transition-all hover:bg-destructive/20 hover:text-destructive group-hover:opacity-100"
                          >
                            <X className="size-3" />
                          </button>
                        </div>

                        {/* Zed-style path badge preview */}
                        <div className="pointer-events-none absolute left-full ml-2 hidden whitespace-nowrap rounded-md border border-border-subtle bg-popover px-2 py-1 font-mono text-ui-xs text-fg-secondary shadow-lg group-hover:block">
                          {formatPathForTooltip(ws.path)}
                        </div>
                      </div>
                    );
                  })}
                </div>
              </div>
            )}

            {/* Section: Recent Projects */}
            {filteredRecentWorkspaces.length > 0 && (
              <div>
                <ComposerMenuLabel>Recent Projects</ComposerMenuLabel>
                <div className="space-y-0.5">
                  {filteredRecentWorkspaces.map(ws => (
                    <div
                      key={ws.path}
                      className="group relative flex min-h-7 cursor-pointer items-center justify-between rounded-sm px-2 py-1 text-left text-ui-base text-fg-secondary transition-colors hover:bg-accent hover:text-fg-primary"
                      onClick={() => {
                        setIsOpen(false);
                        void switchWorkspace(ws.path);
                      }}
                    >
                      <div className="flex min-w-0 flex-1 truncate items-center gap-2 pr-2">
                        {ws.type === 'remote' ? (
                          <Server className="size-3.5 shrink-0 text-muted-foreground" />
                        ) : (
                          <Laptop className="size-3.5 shrink-0 text-muted-foreground" />
                        )}
                        <span className="truncate">
                          {ws.host ? `${ws.name} (${ws.host})` : ws.name}
                        </span>
                      </div>

                      <div className="flex shrink-0 items-center gap-1">
                        {/* Open in New Window on hover */}
                        <button
                          type="button"
                          onClick={(e) => {
                            e.stopPropagation();
                            void openWorkspaceInNewWindow(ws.path);
                          }}
                          title="Open in New Window"
                          className="cursor-pointer rounded p-0.5 text-fg-secondary opacity-0 transition-all hover:bg-accent hover:text-fg-primary group-hover:opacity-100"
                        >
                          <ExternalLink className="size-3" />
                        </button>

                        {/* Delete / Remove Project from Recent History */}
                        <button
                          type="button"
                          onClick={(e) => {
                            e.stopPropagation();
                            removeRecentWorkspace(ws.path);
                          }}
                          title="Delete Project (Remove from history)"
                          className="cursor-pointer rounded p-0.5 text-fg-secondary opacity-0 transition-all hover:bg-destructive/20 hover:text-destructive group-hover:opacity-100"
                        >
                          <Trash2 className="size-3" />
                        </button>
                      </div>

                      {/* Tooltip path */}
                      <div className="pointer-events-none absolute left-full ml-2 hidden whitespace-nowrap rounded-md border border-border-subtle bg-popover px-2 py-1 font-mono text-ui-xs text-fg-secondary shadow-lg group-hover:block">
                        {formatPathForTooltip(ws.path)}
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>

          {/* Footer Actions (Open Local Folders / Open Remote Folder).
              The remote entry is hidden while the ForgeADE agent is active. */}
          <div className="mt-1 space-y-0.5 border-t border-border-subtle pt-1">
            <button
              type="button"
              onClick={() => {
                setIsOpen(false);
                void openFolder();
              }}
              className="flex min-h-7 w-full cursor-pointer items-center gap-2 rounded-sm px-2 py-1 text-left text-ui-base text-fg-primary transition-colors hover:bg-accent"
            >
              <FolderOpen className="size-3.5 text-muted-foreground" />
              <span>Open Local Folders</span>
            </button>
            {!forgeActive && (
              <button
                type="button"
                onClick={() => {
                  setIsOpen(false);
                  setIsSSHModalOpen(true);
                }}
                className="flex min-h-7 w-full cursor-pointer items-center gap-2 rounded-sm px-2 py-1 text-left text-ui-base text-fg-primary transition-colors hover:bg-accent"
              >
                <Server className="size-3.5 text-muted-foreground" />
                <span>Open Remote Folder</span>
              </button>
            )}
          </div>
        </ComposerMenu>
      )}
    </div>
  );
};
