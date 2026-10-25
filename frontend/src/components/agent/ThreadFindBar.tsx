import React, { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react';
import { ChevronDown, ChevronUp, X } from 'lucide-react';
import { cn } from '../../lib/utils';

const FIND_HIGHLIGHT = 'forge-thread-find';
const ACTIVE_HIGHLIGHT = 'forge-thread-find-active';

interface TextOffset {
  node: Text;
  start: number;
  end: number;
}

/**
 * Walk the visible text nodes of the transcript and concatenate them so
 * matches spanning element boundaries (bold words, code chips) still highlight.
 */
function collectText(root: HTMLElement): { text: string; offsets: TextOffset[] } {
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
  const offsets: TextOffset[] = [];
  let text = '';
  let node: Node | null;
  while ((node = walker.nextNode())) {
    const value = (node as Text).nodeValue || '';
    if (value.length === 0) continue;
    offsets.push({ node: node as Text, start: text.length, end: text.length + value.length });
    text += value;
  }
  return { text, offsets };
}

function rangeForMatch(offsets: TextOffset[], start: number, end: number): Range | null {
  for (const offset of offsets) {
    if (offset.end <= start) continue;
    if (offset.start >= end) break;
    const range = document.createRange();
    range.setStart(offset.node, Math.max(0, start - offset.start));
    range.setEnd(offset.node, Math.min(offset.node.length, end - offset.start));
    return range;
  }
  return null;
}

function supportsHighlights(): boolean {
  return typeof CSS !== 'undefined' && 'highlights' in CSS && typeof Highlight !== 'undefined';
}

const HIGHLIGHT_STYLE_ID = 'forge-thread-find-styles';

function ensureHighlightStyles(): void {
  if (typeof document === 'undefined' || document.getElementById(HIGHLIGHT_STYLE_ID)) return;
  const style = document.createElement('style');
  style.id = HIGHLIGHT_STYLE_ID;
  style.textContent = `
    ::highlight(forge-thread-find) {
      color: var(--app-theme-search-match-foreground, oklch(0.216164 0.028283 77.423));
      background-color: var(--app-theme-search-match-background, oklch(0.915 0.11 85));
    }
    ::highlight(forge-thread-find-active) {
      color: var(--app-theme-search-match-active-foreground, oklch(0.216164 0.028283 77.423));
      background-color: var(--app-theme-search-match-active-background, oklch(0.774202 0.151946 74.331));
    }
  `;
  document.head.appendChild(style);
}

if (typeof window !== 'undefined' && supportsHighlights()) {
  ensureHighlightStyles();
}

/**
 * Find-in-thread: searches the transcript's text and paints
 * matches through the CSS Custom Highlight API, so rendering stays untouched
 * and copied text is unaffected. Matches are case-insensitive.
 */
export const ThreadFindBar: React.FC<{
  containerRef: React.RefObject<HTMLElement | null>;
  open: boolean;
  onClose: () => void;
}> = ({ containerRef, open, onClose }) => {
  const [query, setQuery] = useState('');
  const [matchCount, setMatchCount] = useState(0);
  const [activeIndex, setActiveIndex] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  // Bumped whenever the transcript DOM changes so stale highlights repaint.
  const [revision, setRevision] = useState(0);

  useEffect(() => {
    if (!open) return;
    inputRef.current?.focus();
    inputRef.current?.select();
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const container = containerRef.current;
    if (!container || !supportsHighlights()) return;
    const observer = new MutationObserver(() => setRevision((r) => r + 1));
    observer.observe(container, { childList: true, characterData: true, subtree: true });
    return () => observer.disconnect();
  }, [open, containerRef]);

  useLayoutEffect(() => {
    if (!open || !supportsHighlights()) return;
    const container = containerRef.current;
    CSS.highlights.delete(FIND_HIGHLIGHT);
    CSS.highlights.delete(ACTIVE_HIGHLIGHT);
    if (!container || query.trim().length === 0) {
      setMatchCount(0);
      return;
    }
    const needle = query.toLowerCase();
    const { text, offsets } = collectText(container);
    const matches: Array<[number, number]> = [];
    let idx = text.toLowerCase().indexOf(needle);
    while (idx !== -1 && matches.length < 2000) {
      matches.push([idx, idx + needle.length]);
      idx = text.toLowerCase().indexOf(needle, idx + needle.length);
    }
    setMatchCount(matches.length);

    const ranges: Range[] = [];
    for (const [start, end] of matches) {
      const range = rangeForMatch(offsets, start, end);
      if (range) ranges.push(range);
    }
    if (ranges.length > 0) {
      CSS.highlights.set(FIND_HIGHLIGHT, new Highlight(...ranges));
    }
    const active = ranges[Math.min(activeIndex, ranges.length - 1)];
    if (active) {
      CSS.highlights.set(ACTIVE_HIGHLIGHT, new Highlight(active));
      const rect = (active as Range).getBoundingClientRect();
      const containerRect = container.getBoundingClientRect();
      if (rect.top < containerRect.top || rect.bottom > containerRect.bottom) {
        (active as Range).startContainer.parentElement?.scrollIntoView({ block: 'center' });
      }
    }
    return () => {
      CSS.highlights.delete(FIND_HIGHLIGHT);
      CSS.highlights.delete(ACTIVE_HIGHLIGHT);
    };
  }, [open, query, activeIndex, revision, containerRef]);

  const step = useCallback(
    (delta: number) => {
      if (matchCount === 0) return;
      setActiveIndex((prev) => (prev + delta + matchCount) % matchCount);
    },
    [matchCount],
  );

  if (!open) return null;

  return (
    <div
      className="dropdown-glass absolute left-1/2 top-3 z-40 flex -translate-x-1/2 items-center gap-1 rounded-xl border border-border-default px-2 py-1.5 shadow-xl"
      role="search"
      onKeyDown={(e) => {
        if (e.key === 'Escape') {
          e.stopPropagation();
          onClose();
        } else if (e.key === 'Enter') {
          e.preventDefault();
          step(e.shiftKey ? -1 : 1);
        }
      }}
    >
      <input
        ref={inputRef}
        type="text"
        value={query}
        onChange={(e) => {
          setQuery(e.target.value);
          setActiveIndex(0);
        }}
        placeholder="Find in thread…"
        className={cn(
          'w-56 bg-transparent px-1.5 py-0.5 text-ui-sm text-fg-primary outline-none',
          'placeholder:text-fg-tertiary',
        )}
      />
      <span className="min-w-14 text-center font-mono text-2xs text-fg-tertiary tabular-nums">
        {query.trim() ? `${matchCount === 0 ? 0 : activeIndex + 1}/${matchCount}` : ''}
      </span>
      <button
        type="button"
        aria-label="Previous match"
        onClick={() => step(-1)}
        disabled={matchCount === 0}
        className="rounded p-1 text-fg-tertiary hover:bg-surface-hover hover:text-fg-primary disabled:opacity-40"
      >
        <ChevronUp className="size-3.5" />
      </button>
      <button
        type="button"
        aria-label="Next match"
        onClick={() => step(1)}
        disabled={matchCount === 0}
        className="rounded p-1 text-fg-tertiary hover:bg-surface-hover hover:text-fg-primary disabled:opacity-40"
      >
        <ChevronDown className="size-3.5" />
      </button>
      <div className="mx-0.5 h-4 w-px bg-border-subtle" />
      <button
        type="button"
        aria-label="Close find bar"
        onClick={onClose}
        className="rounded p-1 text-fg-tertiary hover:bg-surface-hover hover:text-fg-primary"
      >
        <X className="size-3.5" />
      </button>
    </div>
  );
};
