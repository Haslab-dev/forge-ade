import React, { useEffect, useRef } from 'react';
import { FileText, Folder } from 'lucide-react';
import type { FileItem } from '../../types';

export interface AtMentionItem {
  path: string;
  isDir: boolean;
}

export const AtMentionMenu: React.FC<{
  items: AtMentionItem[];
  index: number;
  onSelect: (item: AtMentionItem) => void;
}> = ({ items, index, onSelect }) => {
  const listRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const el = listRef.current?.children[index] as HTMLElement | undefined;
    el?.scrollIntoView({ block: 'nearest' });
  }, [index]);

  if (items.length === 0) return null;

  return (
    <div
      ref={listRef}
      className="absolute bottom-full left-3 right-3 mb-1 max-h-56 overflow-y-auto rounded-xl border border-border bg-white shadow-2xl dark:bg-[#222224] z-40"
    >
      {items.map((it, i) => (
        <button
          key={it.path}
          type="button"
          className={`flex w-full items-center gap-2.5 px-3 py-2 text-left text-xs transition-colors ${
            i === index ? 'bg-primary/10 dark:bg-card/60' : 'hover:bg-surface dark:hover:bg-surface-hover'
          }`}
          onMouseDown={(e) => {
            e.preventDefault();
            onSelect(it);
          }}
        >
          {it.isDir ? (
            <Folder className="size-3.5 shrink-0 text-info" />
          ) : (
            <FileText className="size-3.5 shrink-0 text-success" />
          )}
          <span className="min-w-0 truncate font-mono text-foreground">{it.path}</span>
          <span className="ml-auto shrink-0 text-[10px] uppercase text-foreground-subtle">
            {it.isDir ? 'folder' : 'file'}
          </span>
        </button>
      ))}
    </div>
  );
};

/** Flatten the workspace FileItem tree into mention candidates (files + folders). */
export function flattenFileTree(nodes: FileItem[], prefix = ''): AtMentionItem[] {
  const out: AtMentionItem[] = [];
  for (const n of nodes || []) {
    const rel = prefix ? `${prefix}/${n.name}` : n.name;
    out.push({ path: rel, isDir: n.type === 'folder' });
    if (n.type === 'folder' && n.children) {
      out.push(...flattenFileTree(n.children, rel));
    }
  }
  return out;
}

/** Extract @-mention tokens (paths without spaces) from the text before caret. */
export function extractAtQuery(textBeforeCaret: string): string | null {
  const match = /(?:^|\s)@([^\s]*)$/.exec(textBeforeCaret);
  return match ? match[1] : null;
}
