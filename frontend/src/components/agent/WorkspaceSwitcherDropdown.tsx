import React, { useState, useRef, useEffect, useMemo } from 'react';
import {
  Laptop,
  Server,
  Check,
  X,
  ExternalLink,
  Search,
  FolderOpen,
  ChevronDown
} from 'lucide-react';
import { ApiBridge } from '../../services/apiBridge';
import { useWorkspace } from '../../stores/workspaceStore';
import type { WorkspaceEntry } from '../../types';

interface WorkspaceSwitcherDropdownProps {
  className?: string;
}

export const WorkspaceSwitcherDropdown: React.FC<WorkspaceSwitcherDropdownProps> = ({ className = '' }) => {
  const {
    activeWorkspacePath,
    activeWorkspaces,
    recentWorkspaces,
    switchWorkspace,
    closeWorkspaceFromWindow,
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
    const folderName = clean.split('/').filter(Boolean).pop() || 'forge-ade';
    return {
      path: activeWorkspacePath,
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
      w.name.toLowerCase().includes(query) ||
      w.path.toLowerCase().includes(query) ||
      (w.host && w.host.toLowerCase().includes(query))
    );
  }, [windowWorkspaces, query]);

  const filteredRecentWorkspaces = useMemo(() => {
    if (!query) return parsedRecentWorkspaces;
    return parsedRecentWorkspaces.filter(w =>
      w.name.toLowerCase().includes(query) ||
      w.path.toLowerCase().includes(query) ||
      (w.host && w.host.toLowerCase().includes(query))
    );
  }, [parsedRecentWorkspaces, query]);

  const formatPathForTooltip = (p: string) => {
    if (!p) return '';
    return p.replace(/^\/Users\/[^/]+/, '~');
  };

  return (
    <div className={`relative shrink-0 select-none ${className}`} ref={dropdownRef}>
      {/* ── Header Trigger Chip (Zed-style badge) ── */}
      <button
        type="button"
        onClick={() => {
          setIsOpen(prev => !prev);
          setSearch('');
        }}
        aria-haspopup="menu"
        aria-expanded={isOpen}
        title={activeWorkspacePath ? `Switch Workspace — ${activeWorkspacePath}` : 'Open Workspace'}
        className="flex h-6.5 cursor-pointer items-center gap-1.5 rounded-md border border-border/50 bg-surface px-2 text-ui-xs font-medium text-fg-secondary transition-colors hover:bg-surface-hover hover:text-fg-primary"
      >
        {currentWorkspace.type === 'remote' ? (
          <Server className="size-3.5 text-primary shrink-0" />
        ) : (
          <Laptop className="size-3.5 text-fg-secondary shrink-0" />
        )}

        {currentWorkspace.host && (
          <span className="font-mono text-ui-xs text-fg-primary font-semibold">
            {currentWorkspace.host}
          </span>
        )}

        <span className="max-w-32 truncate text-ui-xs text-fg-primary font-medium">
          {currentWorkspace.name}
        </span>

        <ChevronDown className="size-3 text-fg-tertiary ml-0.5" />
      </button>

      {/* ── Zed-Style Popover Menu ── */}
      {isOpen && (
        <div
          role="menu"
          aria-label="Workspace Switcher"
          className="absolute left-0 top-full z-50 mt-1.5 w-84 rounded-xl border border-border bg-popover p-1.5 shadow-2xl backdrop-blur-md animate-in fade-in zoom-in-95 duration-100"
        >
          {/* Search projects input */}
          <div className="relative mb-1.5 flex items-center rounded-lg border border-border/40 bg-surface px-2.5 py-1.5">
            <Search className="size-3.5 text-fg-secondary mr-2 shrink-0" />
            <input
              ref={searchInputRef}
              type="text"
              value={search}
              onChange={e => setSearch(e.target.value)}
              placeholder="Search projects..."
              aria-label="Search projects"
              className="w-full bg-transparent font-sans text-ui-sm text-fg-primary placeholder-fg-tertiary outline-none"
            />
          </div>

          <div className="max-h-80 overflow-y-auto space-y-3 py-1 pr-0.5">
            {/* Section: This Window */}
            {filteredWindowWorkspaces.length > 0 && (
              <div>
                <div className="px-2 py-1 text-ui-xs font-semibold text-fg-tertiary uppercase tracking-wider">
                  This Window
                </div>
                <div className="space-y-0.5">
                  {filteredWindowWorkspaces.map(ws => {
                    const isActive = ws.path === activeWorkspacePath;
                    return (
                      <div
                        key={ws.path}
                        className={`group relative flex items-center justify-between rounded-lg px-2.5 py-1.5 text-ui-sm transition-colors cursor-pointer ${
                          isActive
                            ? 'bg-surface-active text-fg-primary font-medium'
                            : 'text-fg-secondary hover:bg-surface-hover hover:text-fg-primary'
                        }`}
                        onClick={() => {
                          setIsOpen(false);
                          void switchWorkspace(ws.path);
                        }}
                      >
                        <div className="flex items-center gap-2 truncate min-w-0 flex-1 pr-2">
                          {ws.type === 'remote' ? (
                            <Server className="size-3.5 text-primary shrink-0" />
                          ) : (
                            <Laptop className="size-3.5 text-fg-secondary shrink-0" />
                          )}
                          <span className="truncate">
                            {ws.host ? `${ws.name} (${ws.host})` : ws.name}
                          </span>
                        </div>

                        <div className="flex items-center gap-1.5 shrink-0">
                          {isActive && (
                            <Check className="size-3.5 text-primary shrink-0" />
                          )}

                          {/* Action: Open in new native window */}
                          <button
                            type="button"
                            onClick={(e) => {
                              e.stopPropagation();
                              void openWorkspaceInNewWindow(ws.path);
                            }}
                            title="Open in New Window"
                            className="opacity-0 group-hover:opacity-100 p-0.5 rounded hover:bg-surface-active text-fg-secondary hover:text-fg-primary transition-all cursor-pointer"
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
                            className="opacity-0 group-hover:opacity-100 p-0.5 rounded hover:bg-destructive/20 text-fg-secondary hover:text-destructive transition-all cursor-pointer"
                          >
                            <X className="size-3" />
                          </button>
                        </div>

                        {/* Zed-style path badge preview */}
                        <div className="hidden group-hover:block absolute left-full ml-2 z-50 rounded bg-surface px-2 py-1 text-ui-xs font-mono text-fg-secondary shadow-lg border border-border/40 whitespace-nowrap pointer-events-none">
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
                <div className="px-2 py-1 text-ui-xs font-semibold text-fg-tertiary uppercase tracking-wider">
                  Recent Projects
                </div>
                <div className="space-y-0.5">
                  {filteredRecentWorkspaces.map(ws => (
                    <div
                      key={ws.path}
                      className="group relative flex items-center justify-between rounded-lg px-2.5 py-1.5 text-ui-sm text-fg-secondary hover:bg-surface-hover hover:text-fg-primary transition-colors cursor-pointer"
                      onClick={() => {
                        setIsOpen(false);
                        void switchWorkspace(ws.path);
                      }}
                    >
                      <div className="flex items-center gap-2 truncate min-w-0 flex-1 pr-2">
                        {ws.type === 'remote' ? (
                          <Server className="size-3.5 text-fg-secondary shrink-0" />
                        ) : (
                          <Laptop className="size-3.5 text-fg-secondary shrink-0" />
                        )}
                        <span className="truncate">
                          {ws.host ? `${ws.name} (${ws.host})` : ws.name}
                        </span>
                      </div>

                      {/* Open in New Window on hover */}
                      <button
                        type="button"
                        onClick={(e) => {
                          e.stopPropagation();
                          void openWorkspaceInNewWindow(ws.path);
                        }}
                        title="Open in New Window"
                        className="opacity-0 group-hover:opacity-100 p-0.5 rounded hover:bg-surface-active text-fg-secondary hover:text-fg-primary transition-all cursor-pointer"
                      >
                        <ExternalLink className="size-3" />
                      </button>

                      {/* Tooltip path */}
                      <div className="hidden group-hover:block absolute left-full ml-2 z-50 rounded bg-surface px-2 py-1 text-ui-xs font-mono text-fg-secondary shadow-lg border border-border/40 whitespace-nowrap pointer-events-none">
                        {formatPathForTooltip(ws.path)}
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>

          {/* Footer Actions (Zed-style: Open Local Folders / Open Remote Folder).
              The remote entry is hidden while the ForgeADE agent is active. */}
          <div className="mt-1 pt-1 border-t border-border/40 space-y-0.5">
            <button
              type="button"
              onClick={() => {
                setIsOpen(false);
                void openFolder();
              }}
              className="flex w-full cursor-pointer items-center gap-2 rounded-lg px-2.5 py-1.5 text-left text-ui-sm text-fg-secondary hover:bg-surface-hover hover:text-fg-primary transition-colors"
            >
              <FolderOpen className="size-3.5 text-fg-secondary" />
              <span>Open Local Folders</span>
            </button>
            {!forgeActive && (
              <button
                type="button"
                onClick={() => {
                  setIsOpen(false);
                  setIsSSHModalOpen(true);
                }}
                className="flex w-full cursor-pointer items-center gap-2 rounded-lg px-2.5 py-1.5 text-left text-ui-sm text-fg-secondary hover:bg-surface-hover hover:text-fg-primary transition-colors"
              >
                <Server className="size-3.5 text-primary" />
                <span>Open Remote Folder</span>
              </button>
            )}
          </div>
        </div>
      )}
    </div>
  );
};
