import React, { useState, useMemo } from 'react';
import { IconFolder, IconFolderOpen, IconFile, IconChevronRight } from '@tabler/icons-react';
import { cn } from '../lib/utils';
import { getFileIcon } from '../lib/file-icons';

// ── Tree types ──────────────────────────────────────────────────────────────

interface TreeDir {
  kind: 'dir';
  name: string;
  path: string;
  children: TreeNode[];
}
interface TreeFile {
  kind: 'file';
  name: string;
  path: string;
}
type TreeNode = TreeDir | TreeFile;

// ── Build tree from flat path list ─────────────────────────────────────────

function buildTree(files: string[]): TreeNode[] {
  const root: TreeDir = { kind: 'dir', name: '', path: '', children: [] };

  for (const filePath of files) {
    const parts = filePath.split('/').filter(Boolean);
    let cur = root;
    let accumulated = '';
    for (let i = 0; i < parts.length; i++) {
      const part = parts[i];
      accumulated = accumulated ? `${accumulated}/${part}` : part;
      const isLast = i === parts.length - 1;
      if (isLast) {
        cur.children.push({ kind: 'file', name: part, path: accumulated });
      } else {
        let dir = cur.children.find(
          (c): c is TreeDir => c.kind === 'dir' && c.name === part,
        );
        if (!dir) {
          dir = { kind: 'dir', name: part, path: accumulated, children: [] };
          cur.children.push(dir);
        }
        cur = dir;
      }
    }
  }

  // Sort: dirs first, then files; both alphabetical.
  function sort(nodes: TreeNode[]): TreeNode[] {
    return nodes
      .sort((a, b) => {
        if (a.kind !== b.kind) return a.kind === 'dir' ? -1 : 1;
        return a.name.localeCompare(b.name);
      })
      .map(n => (n.kind === 'dir' ? { ...n, children: sort(n.children) } : n));
  }

  return sort(root.children);
}

// ── Single tree node row ────────────────────────────────────────────────────

function TreeNodeRow({
  node,
  depth,
  onClickFile,
}: {
  node: TreeNode;
  depth: number;
  onClickFile?: (path: string) => void;
}) {
  // All folders start collapsed by default.
  const [open, setOpen] = useState(false);
  const indent = depth * 12;

  if (node.kind === 'dir') {
    return (
      <>
        <button
          type="button"
          onClick={() => setOpen(v => !v)}
          className="w-full flex items-center gap-1 py-0.5 px-1 rounded hover:bg-[var(--bg-surface-hover)] text-left cursor-pointer select-none"
          style={{ paddingLeft: `${4 + indent}px` }}
        >
          <IconChevronRight
            className={cn(
              'w-3 h-3 text-[var(--fg-tertiary)] shrink-0 transition-transform duration-100',
              open && 'rotate-90',
            )}
          />
          {open ? (
            <IconFolderOpen className="w-3.5 h-3.5 text-[var(--graph-c3)] shrink-0" />
          ) : (
            <IconFolder className="w-3.5 h-3.5 text-[var(--graph-c3)] shrink-0" />
          )}
          <span className="text-[var(--fg-primary)] truncate">{node.name}</span>
          <span className="ml-auto text-[8px] text-[var(--fg-tertiary)] font-mono shrink-0 pr-1">
            {node.children.length}
          </span>
        </button>
        {open &&
          node.children.map(child => (
            <TreeNodeRow
              key={child.path}
              node={child}
              depth={depth + 1}
              onClickFile={onClickFile}
            />
          ))}
      </>
    );
  }

  // File node — getFileIcon returns a ReactNode, render directly.
  const fileIcon = getFileIcon(node.name, 'w-3.5 h-3.5');
  return (
    <button
      type="button"
      onClick={() => onClickFile?.(node.path)}
      className="w-full flex items-center gap-1.5 py-0.5 px-1 rounded hover:bg-[var(--bg-surface-hover)] text-left cursor-pointer select-none"
      style={{ paddingLeft: `${4 + indent + 16}px` }}
      title={node.path}
    >
      {fileIcon ?? <IconFile className="w-3.5 h-3.5 text-[var(--fg-tertiary)] shrink-0" />}
      <span className="text-[var(--fg-secondary)] truncate">{node.name}</span>
    </button>
  );
}

// ── Public component ────────────────────────────────────────────────────────

interface CommitFileTreeProps {
  files: string[];
  onClickFile?: (path: string) => void;
}

export function CommitFileTree({ files, onClickFile }: CommitFileTreeProps) {
  const [expanded, setExpanded] = useState(true);
  const tree = useMemo(() => buildTree(files), [files]);

  if (files.length === 0) return null;

  return (
    <div className="mb-4 border border-[var(--border-default)] rounded overflow-hidden">
      {/* Section header — click to collapse/expand the whole panel */}
      <button
        type="button"
        onClick={() => setExpanded(v => !v)}
        className="w-full flex items-center gap-2 px-3 py-1.5 bg-[var(--bg-sidebar)] border-b border-[var(--border-default)] hover:bg-[var(--bg-surface-hover)] transition-colors cursor-pointer select-none"
      >
        <IconChevronRight
          className={cn(
            'w-3.5 h-3.5 text-[var(--fg-tertiary)] transition-transform duration-100',
            expanded && 'rotate-90',
          )}
        />
        <span className="text-[10px] font-semibold uppercase tracking-wider text-[var(--fg-secondary)]">
          Changed Files
        </span>
        <span className="text-[10px] font-mono text-[var(--fg-tertiary)] ml-0.5">
          ({files.length})
        </span>
      </button>

      {/* Tree body */}
      {expanded && (
        <div className="max-h-56 overflow-y-auto p-1 bg-[var(--bg-panel)] text-xs font-mono">
          {tree.map(node => (
            <TreeNodeRow
              key={node.path}
              node={node}
              depth={0}
              onClickFile={onClickFile}
            />
          ))}
        </div>
      )}
    </div>
  );
}
