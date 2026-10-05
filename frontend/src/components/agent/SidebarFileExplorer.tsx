import React, { useEffect, useMemo, useState } from 'react';
import {
  ChevronRight,
  ChevronDown,
  Folder,
  FolderOpen,
  RefreshCw,
  ExternalLink,
  Search,
  ArrowLeft
} from 'lucide-react';
import { cn } from '../../lib/utils';
import { useWorkspace } from '../../stores/workspaceStore';
import { getFileIcon } from '../../lib/file-icons';
import { ApiBridge } from '../../services/apiBridge';
import { FileItem } from '../../types';

/**
 * ZCode-style file explorer rendered inside the left sidebar: back-to-tasks,
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
    if (n.type === 'file') {
      if (n.name.toLowerCase().includes(lower)) out.push(n);
    } else {
      const kids = n.children ? filterTree(n.children, q) : [];
      if (kids.length > 0 || n.name.toLowerCase().includes(lower)) {
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
    gitFiles,
    activeWorkspacePath,
    activeSideFile,
    openSideFile,
    setIsCommandPaletteOpen
  } = useWorkspace();

  const [query, setQuery] = useState('');
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [ignoredSet, setIgnoredSet] = useState<Set<string>>(new Set());

  const projectName = (activeWorkspacePath || '').split('/').filter(Boolean).pop() || 'Project';

  // Paths ignored by .gitignore — rendered dimmed (folder rule dims its subtree).
  useEffect(() => {
    let cancelled = false;
    const all: string[] = [];
    const walk = (nodes?: FileItem[]) => {
      for (const n of nodes || []) {
        all.push(n.path);
        if (all.length >= 3000) return;
        if (n.children) walk(n.children);
      }
    };
    walk(files);
    if (all.length === 0) {
      setIgnoredSet(new Set());
      return;
    }
    ApiBridge.gitCheckIgnored(all, activeWorkspacePath || '')
      .then(ignored => {
        if (!cancelled) setIgnoredSet(new Set(ignored || []));
      })
      .catch(() => {});
    return () => {
      cancelled = true;
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
    } finally {
      setIsRefreshing(false);
    }
  };

  const toggleDir = (path: string) =>
    setExpanded(prev => ({ ...prev, [path]: !prev[path] }));

  const renderNode = (node: FileItem, depth: number, parentIgnored = false): React.ReactNode => {
    if (node.type === 'folder') {
      const isOpen = !!expanded[node.path] || !!query.trim();
      const hasChanges = changedDirSet.has(node.path);
      const isIgnored = parentIgnored || ignoredSet.has(node.path);
      return (
        <div key={node.path}>
          <button
            type="button"
            onClick={() => toggleDir(node.path)}
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
            <span className={cn('min-w-0 flex-1 truncate text-[12px]', hasChanges ? 'text-warning/90' : 'text-foreground/85')}>
              {node.name}
            </span>
            {hasChanges && <span className="mr-1 size-1.5 shrink-0 rounded-full bg-warning" aria-hidden="true" />}
          </button>
          {isOpen && node.children && (
            <div>{node.children.map(child => renderNode(child, depth + 1, isIgnored))}</div>
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
        title={isIgnored ? `${node.path} (gitignored)` : node.path}
        className={cn(
          'group/ffile flex h-7 w-full items-center gap-1.5 rounded-[6px] pr-2 text-left transition-colors cursor-pointer',
          isActive ? 'bg-[#232323] text-foreground' : 'hover:bg-surface-hover',
          tone && !isIgnored ? tone : !isIgnored ? 'text-foreground/85' : '',
          tone && !isIgnored && 'bg-warning/[0.05]',
          status === 'D' && !isIgnored && 'bg-destructive/[0.06] line-through decoration-foreground-subtle/60',
          isIgnored && 'opacity-40'
        )}
        style={{ paddingLeft: 6 + depth * 12 }}
      >
        {getFileIcon(node.name, 'size-3.5 shrink-0')}
        <span className="min-w-0 flex-1 truncate text-[12px]">{node.name}</span>
        {status && (
          <span className="shrink-0 rounded border border-border px-1 text-[9px] font-mono text-foreground-subtle">
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
          className="flex h-7 w-full items-center gap-2 rounded-[8px] px-2 text-left text-[12px] font-medium text-foreground/85 transition-colors hover:bg-surface-hover hover:text-foreground cursor-pointer"
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
            className="w-full border-0 bg-transparent text-[12px] text-foreground placeholder-foreground-subtlest outline-none"
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
          <span className="truncate text-[12px] font-medium text-foreground-subtle">{projectName}</span>
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
          <p className="px-2 py-6 text-center text-[11px] text-foreground-subtlest">
            {query ? 'No matching files' : 'No files loaded'}
          </p>
        ) : (
          visibleTree.map(node => renderNode(node, 0))
        )}
      </div>
    </div>
  );
};

export default SidebarFileExplorer;
