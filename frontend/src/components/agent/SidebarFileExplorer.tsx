import React, { useEffect, useMemo, useState } from 'react';
import {
  ChevronRight,
  ChevronDown,
  Folder,
  FolderOpen,
  RefreshCw,
  ExternalLink,
  Search,
  ArrowLeft,
  Copy,
  Eye,
  Code2
} from 'lucide-react';
import { cn } from '../../lib/utils';
import { useWorkspace } from '../../stores/workspaceStore';
import { getFileIcon } from '../../lib/file-icons';
import { ApiBridge } from '../../services/apiBridge';
import { FileItem } from '../../types';

/**
 * Workspace file explorer rendered inside the left sidebar: back-to-tasks,
 * fuzzy file search, collapsible tree with git decorations, and click-to-open
 * into the right side pane.
 */

const MAX_MATCHES = 200;

/** Parent directories of every changed path — drives the folder status dots. */
const buildChangedDirSet = (changedPaths: string[]): Set<string> => {
  const dirs = new Set<string>();
  for (const p of changedPaths) {
    const parts = p.split('/');
    parts.pop();
    let cur = '';
    for (const part of parts) {
      cur = cur ? `${cur}/${part}` : part;
      dirs.add(cur);
    }
  }
  return dirs;
};

const filterTree = (nodes: FileItem[], q: string): FileItem[] => {
  if (!q) return nodes;
  const lower = q.toLowerCase();
  const out: FileItem[] = [];
  for (const n of nodes) {
    const nName = (n?.name || '').toLowerCase();
    if (n.type === 'file') {
      if (nName.includes(lower)) out.push(n);
    } else {
      const kids = n.children ? filterTree(n.children, q) : [];
      if (kids.length > 0 || nName.includes(lower)) {
        out.push({ ...n, children: kids });
      }
    }
    if (out.length >= MAX_MATCHES) break;
  }
  return out;
};

export const SidebarFileExplorer: React.FC<{ onBack: () => void }> = ({ onBack }) => {
  const {
    files,
    refreshFiles,
    revalidateFileMutations,
    updateFolderChildren,
    gitFiles,
    refreshGitStatus,
    refreshGitLog,
    activeWorkspacePath,
    activeSideFile,
    openSideFile,
    openFileInEditor,
    setMode,
    setIsCommandPaletteOpen
  } = useWorkspace();

  const [query, setQuery] = useState('');
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});
  const [loadingDirs, setLoadingDirs] = useState<Record<string, boolean>>({});
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [ignoredSet, setIgnoredSet] = useState<Set<string>>(new Set());
  const [contextMenu, setContextMenu] = useState<{ x: number; y: number; node: FileItem } | null>(null);

  const projectName = (activeWorkspacePath || '').split('/').filter(Boolean).pop() || 'Project';

  // Ensure files & git status are loaded when explorer mounts or workspace changes
  useEffect(() => {
    if (activeWorkspacePath) {
      void refreshFiles();
      void refreshGitStatus();
      void refreshGitLog();
    }
  }, [activeWorkspacePath]);

  // Auto-expand non-hidden top-level folders on initial load if expanded is empty
  useEffect(() => {
    if (files && files.length > 0 && Object.keys(expanded).length === 0) {
      const topDirs: Record<string, boolean> = {};
      files.forEach(f => {
        if (f.type === 'folder' && !f.name.startsWith('.')) {
          topDirs[f.path] = true;
        }
      });
      setExpanded(topDirs);
    }
  }, [files]);

  // Close context menu on outside click or Escape
  useEffect(() => {
    const handleClick = () => setContextMenu(null);
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setContextMenu(null);
    };
    window.addEventListener('click', handleClick);
    window.addEventListener('keydown', handleKey);
    return () => {
      window.removeEventListener('click', handleClick);
      window.removeEventListener('keydown', handleKey);
    };
  }, []);

  // Paths ignored by .gitignore — rendered dimmed (folder rule dims its subtree).
  // Debounced: the walk collects up to 3000 paths and the backend check runs a
  // git process — firing it on every tree/stream tick stuttered the sidebar.
  useEffect(() => {
    let cancelled = false;
    const timer = window.setTimeout(() => {
      const all: string[] = [];
      const walk = (nodes?: FileItem[]) => {
        for (const n of nodes || []) {
          all.push(n.path);
          if (all.length >= 3000) return;
          if (n.children) walk(n.children);
        }
      };
      walk(files);
      if (all.length === 0 || activeWorkspacePath.startsWith('ssh://')) {
        setIgnoredSet(new Set());
        return;
      }
      ApiBridge.gitCheckIgnored(all, activeWorkspacePath || '')
        .then(ignored => {
          if (!cancelled) setIgnoredSet(new Set(ignored || []));
        })
        .catch(() => {});
    }, 350);
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [files, activeWorkspacePath]);

  const changedDirSet = useMemo(
    () => buildChangedDirSet(gitFiles.map(g => g.path)),
    [gitFiles]
  );
  const changedMap = useMemo(() => {
    const m = new Map<string, string>();
    for (const g of gitFiles) m.set(g.path, g.status || 'M');
    return m;
  }, [gitFiles]);

  const visibleTree = useMemo(() => filterTree(files || [], query.trim()), [files, query]);

  // VS Code-style visibility: modified amber, added/untracked green, deleted red.
  const statusTone = (status?: string): string => {
    switch (status) {
      case 'A':
      case '?':
        return 'text-success';
      case 'D':
        return 'text-destructive';
      case 'M':
      case 'R':
      case 'U':
        return 'text-warning';
      default:
        return '';
    }
  };

  const handleRefresh = async () => {
    setIsRefreshing(true);
    try {
      await revalidateFileMutations();
      await refreshGitStatus();
      await refreshGitLog();
    } finally {
      setIsRefreshing(false);
    }
  };

  const handleOpenInEditor = (filePath: string) => {
    setMode('editor');
    void openFileInEditor(filePath);
  };

  const toggleDir = async (path: string, node?: FileItem) => {
    const isOpening = !expanded[path];
    setExpanded(prev => ({ ...prev, [path]: isOpening }));

    if (isOpening && (!node?.children || node.children.length === 0)) {
      setLoadingDirs(prev => ({ ...prev, [path]: true }));
      try {
        const children = await ApiBridge.listDirectory(path);
        if (children && children.length > 0) {
          updateFolderChildren(path, children);
        }
      } catch (err) {
        console.warn('Failed to load children for folder:', path, err);
      } finally {
        setLoadingDirs(prev => ({ ...prev, [path]: false }));
      }
    }
  };

  const renderNode = (node: FileItem, depth: number, parentIgnored = false): React.ReactNode => {
    if (node.type === 'folder') {
      const isOpen = !!expanded[node.path] || !!query.trim();
      const hasChanges = changedDirSet.has(node.path);
      const isIgnored = parentIgnored || ignoredSet.has(node.path);
      const isLoadingThis = !!loadingDirs[node.path];
      const hasKids = node.children && node.children.length > 0;

      return (
        <div key={node.path}>
          <button
            type="button"
            onClick={() => toggleDir(node.path, node)}
            onContextMenu={(e) => {
              e.preventDefault();
              e.stopPropagation();
              setContextMenu({ x: e.clientX, y: e.clientY, node });
            }}
            title={isIgnored ? `${node.path} (gitignored)` : node.path}
            className={cn(
              'group/fdir flex h-7 w-full items-center gap-1.5 rounded-[6px] pr-2 text-left transition-colors hover:bg-surface-hover cursor-pointer',
              hasChanges && 'bg-warning/[0.05]',
              isIgnored && 'opacity-40'
            )}
            style={{ paddingLeft: 6 + depth * 12 }}
          >
            {isOpen ? (
              <ChevronDown className="size-3 shrink-0 text-foreground-subtlest" />
            ) : (
              <ChevronRight className="size-3 shrink-0 text-foreground-subtlest" />
            )}
            {isOpen ? (
              <FolderOpen className="size-3.5 shrink-0 text-foreground-subtle" />
            ) : (
              <Folder className="size-3.5 shrink-0 text-foreground-subtle" />
            )}
            <span className={cn('min-w-0 flex-1 truncate text-ui-sm', hasChanges ? 'text-warning/90' : 'text-foreground/85')}>
              {node.name}
            </span>
            {isLoadingThis && (
              <RefreshCw className="mr-1 size-3 shrink-0 animate-spin text-foreground-subtle" />
            )}
            {hasChanges && !isLoadingThis && <span className="mr-1 size-1.5 shrink-0 rounded-full bg-warning" aria-hidden="true" />}
          </button>
          {isOpen && (
            <div>
              {hasKids ? (
                node.children!.map(child => renderNode(child, depth + 1, isIgnored))
              ) : isLoadingThis ? (
                <div
                  className="h-6 flex items-center text-[11px] text-foreground-subtle/70 italic select-none"
                  style={{ paddingLeft: 6 + (depth + 1) * 12 }}
                >
                  Loading…
                </div>
              ) : (
                <div
                  className="h-6 flex items-center text-[11px] text-foreground-subtle/60 italic select-none"
                  style={{ paddingLeft: 6 + (depth + 1) * 12 }}
                >
                  (empty)
                </div>
              )}
            </div>
          )}
        </div>
      );
    }

    const status = changedMap.get(node.path);
    const isActive = activeSideFile === node.path;
    const isIgnored = parentIgnored || ignoredSet.has(node.path);
    const tone = statusTone(status);
    return (
      <button
        key={node.path}
        type="button"
        onClick={() => openSideFile(node.path)}
        onDoubleClick={() => handleOpenInEditor(node.path)}
        onContextMenu={(e) => {
          e.preventDefault();
          e.stopPropagation();
          setContextMenu({ x: e.clientX, y: e.clientY, node });
        }}
        title={isIgnored ? `${node.path} (gitignored)` : node.path}
        className={cn(
          'group/ffile flex h-7 w-full items-center gap-1.5 rounded-[6px] pr-2 text-left transition-colors cursor-pointer',
          isActive ? 'bg-elevated text-foreground' : 'hover:bg-surface-hover',
          tone && !isIgnored ? tone : !isIgnored ? 'text-foreground/85' : '',
          tone && !isIgnored && 'bg-warning/[0.05]',
          status === 'D' && !isIgnored && 'bg-destructive/[0.06] line-through decoration-foreground-subtle/60',
          isIgnored && 'opacity-40'
        )}
        style={{ paddingLeft: 6 + depth * 12 }}
      >
        {getFileIcon(node.name, 'size-3.5 shrink-0')}
        <span className="min-w-0 flex-1 truncate text-ui-sm">{node.name}</span>
        {status && (
          <span className="shrink-0 rounded border border-border px-1 text-ui-xs font-mono text-foreground-subtle">
            {status === '?' ? 'UNT' : status}
          </span>
        )}
      </button>
    );
  };

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      {/* Back to tasks */}
      <div className="flex items-center px-2 pb-1">
        <button
          type="button"
          onClick={onBack}
          className="flex h-7 w-full items-center gap-2 rounded-[8px] px-2 text-left text-ui-sm font-medium text-foreground/85 transition-colors hover:bg-surface-hover hover:text-foreground cursor-pointer"
        >
          <ArrowLeft className="size-3.5" />
          <span>Back to tasks</span>
        </button>
      </div>

      {/* Search files */}
      <div className="px-2 pb-2">
        <div className="flex h-8 items-center gap-2 rounded-lg border border-border bg-background px-2.5">
          <Search className="size-3.5 shrink-0 text-foreground-subtle" />
          <input
            type="text"
            value={query}
            onChange={e => setQuery(e.target.value)}
            onKeyDown={e => {
              if (e.key === 'Escape') setQuery('');
            }}
            placeholder="Search files..."
            className="w-full border-0 bg-transparent text-ui-sm text-foreground placeholder-foreground-subtlest outline-none"
          />
        </div>
      </div>

      {/* Project header */}
      <div className="flex items-center justify-between px-3 pb-1.5">
        <button
          type="button"
          onClick={() => setIsCommandPaletteOpen(true)}
          className="flex min-w-0 items-center gap-1.5 text-left cursor-pointer"
          title={activeWorkspacePath}
        >
          <span className="truncate text-ui-sm font-medium text-foreground-subtle">{projectName}</span>
          <ExternalLink className="size-3 shrink-0 text-foreground-subtlest" />
        </button>
        <button
          type="button"
          onClick={handleRefresh}
          aria-label="Refresh files"
          title="Refresh files"
          className="flex size-5 items-center justify-center rounded text-foreground-subtle hover:bg-surface-hover hover:text-foreground transition-colors cursor-pointer"
        >
          <RefreshCw className={cn('size-3', isRefreshing && 'animate-spin')} />
        </button>
      </div>

      {/* Tree */}
      <div className="flex min-h-0 flex-1 flex-col gap-0.5 overflow-y-auto px-2 pb-2">
        {visibleTree.length === 0 ? (
          <p className="px-2 py-6 text-center text-ui-xs text-foreground-subtlest">
            {query ? 'No matching files' : 'No files loaded'}
          </p>
        ) : (
          visibleTree.map(node => renderNode(node, 0))
        )}
      </div>

      {/* Right-click context menu */}
      {contextMenu && (
        <div
          role="menu"
          aria-label="File explorer actions"
          className="fixed z-50 min-w-48 rounded-lg border border-border bg-popover/95 p-1 text-ui-xs shadow-xl backdrop-blur-sm"
          style={{
            top: Math.min(contextMenu.y, window.innerHeight - 240),
            left: Math.min(contextMenu.x, window.innerWidth - 220),
          }}
          onClick={e => e.stopPropagation()}
        >
          {contextMenu.node.type === 'file' ? (
            <>
              <button
                type="button"
                role="menuitem"
                onClick={() => {
                  openSideFile(contextMenu.node.path);
                  setContextMenu(null);
                }}
                className="flex w-full items-center gap-2 rounded px-2.5 py-1.5 text-left text-foreground hover:bg-surface-hover cursor-pointer"
              >
                <Eye className="size-3.5 text-foreground-subtle" />
                <span>Open in Side Preview</span>
              </button>
              <button
                type="button"
                role="menuitem"
                onClick={() => {
                  handleOpenInEditor(contextMenu.node.path);
                  setContextMenu(null);
                }}
                className="flex w-full items-center gap-2 rounded px-2.5 py-1.5 text-left text-foreground hover:bg-surface-hover cursor-pointer"
              >
                <Code2 className="size-3.5 text-foreground-subtle" />
                <span>Open in Editor</span>
              </button>
              <div className="my-1 border-t border-border" />
            </>
          ) : (
            <>
              <button
                type="button"
                role="menuitem"
                onClick={() => {
                  toggleDir(contextMenu.node.path);
                  setContextMenu(null);
                }}
                className="flex w-full items-center gap-2 rounded px-2.5 py-1.5 text-left text-foreground hover:bg-surface-hover cursor-pointer"
              >
                {expanded[contextMenu.node.path] ? (
                  <>
                    <Folder className="size-3.5 text-foreground-subtle" />
                    <span>Collapse Folder</span>
                  </>
                ) : (
                  <>
                    <FolderOpen className="size-3.5 text-foreground-subtle" />
                    <span>Expand Folder</span>
                  </>
                )}
              </button>
              <div className="my-1 border-t border-border" />
            </>
          )}

          <button
            type="button"
            role="menuitem"
            onClick={() => {
              void navigator.clipboard.writeText(contextMenu.node.path);
              setContextMenu(null);
            }}
            className="flex w-full items-center gap-2 rounded px-2.5 py-1.5 text-left text-foreground hover:bg-surface-hover cursor-pointer"
          >
            <Copy className="size-3.5 text-foreground-subtle" />
            <span>Copy Path</span>
          </button>
          <button
            type="button"
            role="menuitem"
            onClick={() => {
              const rel = activeWorkspacePath && contextMenu.node.path.startsWith(activeWorkspacePath)
                ? contextMenu.node.path.slice(activeWorkspacePath.length).replace(/^\/+/, '')
                : contextMenu.node.path;
              void navigator.clipboard.writeText(rel);
              setContextMenu(null);
            }}
            className="flex w-full items-center gap-2 rounded px-2.5 py-1.5 text-left text-foreground hover:bg-surface-hover cursor-pointer"
          >
            <Copy className="size-3.5 text-foreground-subtle" />
            <span>Copy Relative Path</span>
          </button>

          <div className="my-1 border-t border-border" />

          <button
            type="button"
            role="menuitem"
            onClick={() => {
              void ApiBridge.openInFinder(contextMenu.node.path);
              setContextMenu(null);
            }}
            className="flex w-full items-center gap-2 rounded px-2.5 py-1.5 text-left text-foreground hover:bg-surface-hover cursor-pointer"
          >
            <ExternalLink className="size-3.5 text-foreground-subtle" />
            <span>Reveal in System</span>
          </button>
          <button
            type="button"
            role="menuitem"
            onClick={() => {
              void handleRefresh();
              setContextMenu(null);
            }}
            className="flex w-full items-center gap-2 rounded px-2.5 py-1.5 text-left text-foreground hover:bg-surface-hover cursor-pointer"
          >
            <RefreshCw className="size-3.5 text-foreground-subtle" />
            <span>Refresh</span>
          </button>
        </div>
      )}
    </div>
  );
};

export default SidebarFileExplorer;
