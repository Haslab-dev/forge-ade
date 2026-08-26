import React, { useState, useEffect, useRef, useMemo, useCallback } from "react";
import { Marked } from "marked";
import {
  IconList,
  IconSearch,
  IconX,
  IconArrowUp,
  IconArrowDown,
  IconFileText,
  IconClock,
  IconCode,
  IconLayoutSidebarRightCollapse,
  IconLayoutSidebarRight,
  IconBook,
} from "@tabler/icons-react";
import { EditorFile } from "../types";
import { useEditorStore } from "../hooks/store";
import { globalOpenFile } from "../panels/editor";
import { cn } from "../lib/utils";
import { BrowserOpenURL } from "../lib/native";

export interface HeadingItem {
  id: string;
  level: number;
  text: string;
  raw: string;
  index: number;
}

interface MarkdownViewerProps {
  file: EditorFile;
  isFocused?: boolean;
  onToggleMode?: () => void;
}

const LEVEL_COLORS: Record<number, { badge: string; text: string; bg: string; border: string }> = {
  1: {
    badge: "bg-sky-500/20 text-sky-400 border-sky-500/30",
    text: "text-[var(--fg-primary)] font-semibold",
    bg: "hover:bg-sky-500/10",
    border: "border-sky-500",
  },
  2: {
    badge: "bg-emerald-500/20 text-emerald-400 border-emerald-500/30",
    text: "text-[var(--fg-primary)] font-medium",
    bg: "hover:bg-emerald-500/10",
    border: "border-emerald-500",
  },
  3: {
    badge: "bg-purple-500/20 text-purple-400 border-purple-500/30",
    text: "text-[var(--fg-secondary)]",
    bg: "hover:bg-purple-500/10",
    border: "border-purple-500",
  },
  4: {
    badge: "bg-amber-500/20 text-amber-400 border-amber-500/30",
    text: "text-[var(--fg-secondary)]",
    bg: "hover:bg-amber-500/10",
    border: "border-amber-500",
  },
  5: {
    badge: "bg-rose-500/20 text-rose-400 border-rose-500/30",
    text: "text-[var(--fg-tertiary)]",
    bg: "hover:bg-rose-500/10",
    border: "border-rose-500",
  },
  6: {
    badge: "bg-zinc-500/20 text-zinc-400 border-zinc-500/30",
    text: "text-[var(--fg-tertiary)]",
    bg: "hover:bg-zinc-500/10",
    border: "border-zinc-500",
  },
};

export function MarkdownViewer({ file, isFocused: _isFocused, onToggleMode }: MarkdownViewerProps) {
  const content = file.content ?? "";
  const containerRef = useRef<HTMLDivElement>(null);
  const legendScrollRef = useRef<HTMLDivElement>(null);
  const activeItemRef = useRef<HTMLButtonElement | null>(null);

  const [activeHeadingId, setActiveHeadingId] = useState<string | null>(null);
  const [legendOpen, setLegendOpen] = useState<boolean>(true);
  const [legendSearch, setLegendSearch] = useState<string>("");
  const [selectedLevelFilter, setSelectedLevelFilter] = useState<number | null>(null);
  const [_copiedCodeId, setCopiedCodeId] = useState<string | null>(null);
  const [flashHeadingId, setFlashHeadingId] = useState<string | null>(null);

  const { setFiles } = useEditorStore();

  // Parse markdown and extract headings, task indices, code blocks, etc.
  const { html, headings, taskLines, wordCount, readingTimeMinutes } = useMemo(() => {
    const extractedHeadings: HeadingItem[] = [];
    const detectedTaskLines: number[] = [];

    // Find line numbers of task checkboxes in the source markdown
    content.split("\n").forEach((line, idx) => {
      if (/^\s*[-*+]\s+\[[ xX]\](?:\s|$)/.test(line)) {
        detectedTaskLines.push(idx);
      }
    });

    let headingCounter = 0;
    let taskCounter = 0;
    let codeBlockCounter = 0;

    const markedInstance = new Marked({
      gfm: true,
      breaks: false,
    });

    markedInstance.use({
      renderer: {
        heading({ tokens, depth, text }) {
          const idx = headingCounter++;
          const plainText = text.replace(/<[^>]*>/g, "").trim();
          const slug = plainText
            .toLowerCase()
            .replace(/[^\w\s-]/g, "")
            .replace(/\s+/g, "-")
            .slice(0, 50) || `section-${idx}`;
          const id = `heading-${idx}-${slug}`;

          extractedHeadings.push({
            id,
            level: depth,
            text: plainText || `Heading ${depth}`,
            raw: text,
            index: idx,
          });

          const inlineContent = this.parser.parseInline(tokens);

          return `<h${depth} id="${id}" class="markdown-heading group relative scroll-mt-6">
            <a href="#${id}" class="heading-anchor-link" data-heading-anchor="${id}" title="Direct link to this section" aria-label="Direct link to ${plainText}">#</a>
            <span class="heading-text">${inlineContent}</span>
          </h${depth}>\n`;
        },
        code({ text, lang }) {
          const codeId = `code-block-${codeBlockCounter++}`;
          const rawLang = (lang || "").trim().split(/\s+/)[0];
          const displayLang = rawLang ? rawLang.toUpperCase() : "CODE";
          const encoded = encodeURIComponent(text);

          return `<div class="code-block-wrapper my-4 rounded-lg overflow-hidden border border-[var(--border-default)] bg-[var(--bg-elevated)] shadow-sm" data-code-block="${codeId}">
            <div class="flex items-center justify-between px-3 py-1.5 bg-[var(--bg-panel)] border-b border-[var(--border-default)] text-[11px] font-mono text-[var(--fg-tertiary)] select-none">
              <span class="font-semibold uppercase tracking-wider text-[var(--fg-secondary)] text-[10px] flex items-center gap-1.5">
                <span class="inline-block size-2 rounded-full bg-[var(--accent-primary)] opacity-70"></span>
                ${displayLang}
              </span>
              <button class="copy-code-btn inline-flex items-center gap-1 px-2 py-0.5 rounded text-[10px] font-sans hover:bg-[var(--bg-surface-hover)] text-[var(--fg-secondary)] hover:text-[var(--fg-primary)] transition-colors cursor-pointer border border-transparent hover:border-[var(--border-default)]" data-code="${encoded}" data-btn-id="${codeId}">
                <span>Copy</span>
              </button>
            </div>
            <pre class="p-3.5 overflow-x-auto text-[12.5px] font-mono leading-relaxed bg-[var(--bg-app)] m-0 text-[var(--fg-primary)]"><code>${escapeHtml(text)}</code></pre>
          </div>\n`;
        },
        checkbox({ checked }) {
          const taskIdx = taskCounter++;
          return `<input type="checkbox" ${checked ? "checked" : ""} data-task-idx="${taskIdx}" class="task-checkbox mt-0.5 cursor-pointer accent-[var(--accent-primary)] rounded align-middle mr-2" />`;
        },
        blockquote(item) {
          const text = item.text || "";
          const body = this.parser.parse(item.tokens);
          // GitHub / Obsidian callout patterns: [!NOTE], [!TIP], [!IMPORTANT], [!WARNING], [!CAUTION]
          const match = text.match(/^\s*\[!(NOTE|TIP|IMPORTANT|WARNING|CAUTION)\]\s*(.*)/i);
          if (match) {
            const type = match[1].toUpperCase();
            const config = {
              NOTE: { border: "border-sky-500", bg: "bg-sky-500/10", title: "Note", color: "text-sky-400" },
              TIP: { border: "border-emerald-500", bg: "bg-emerald-500/10", title: "Tip", color: "text-emerald-400" },
              IMPORTANT: { border: "border-purple-500", bg: "bg-purple-500/10", title: "Important", color: "text-purple-400" },
              WARNING: { border: "border-amber-500", bg: "bg-amber-500/10", title: "Warning", color: "text-amber-400" },
              CAUTION: { border: "border-red-500", bg: "bg-red-500/10", title: "Caution", color: "text-red-400" },
            }[type] || { border: "border-[var(--border-default)]", bg: "bg-[var(--bg-panel)]", title: type, color: "text-[var(--fg-secondary)]" };

            const cleanBody = body
              .replace(/<p>\s*\[!(NOTE|TIP|IMPORTANT|WARNING|CAUTION)\]\s*(?:<br\s*\/?>)?/i, "<p>")
              .replace(/<p>\s*\[!(NOTE|TIP|IMPORTANT|WARNING|CAUTION)\]\s*<\/p>/i, "");

            return `<div class="callout-box my-3.5 p-3.5 rounded-lg border-l-4 ${config.border} ${config.bg} text-xs shadow-sm">
              <div class="font-bold text-[11px] uppercase tracking-wider ${config.color} mb-1.5 flex items-center gap-1.5">
                <span class="inline-block size-1.5 rounded-full ${config.color.replace("text-", "bg-")}"></span>
                ${config.title}
              </div>
              <div class="text-[var(--fg-secondary)] [&>p:last-child]:mb-0 leading-relaxed">${cleanBody}</div>
            </div>\n`;
          }

          return `<blockquote class="my-3.5 pl-4 border-l-3 border-[var(--border-default)] text-[var(--fg-secondary)] italic bg-[var(--bg-panel)]/30 py-1 rounded-r-md">${body}</blockquote>\n`;
        },
        table(token) {
          const header = token.header
            .map(
              (cell) =>
                `<th class="px-3.5 py-2 bg-[var(--bg-panel)] font-semibold text-left border border-[var(--border-default)] text-[var(--fg-primary)] text-xs uppercase tracking-wider">${this.parser.parseInline(cell.tokens)}</th>`
            )
            .join("");
          const rows = token.rows
            .map((row) => {
              const cells = row
                .map(
                  (cell) =>
                    `<td class="px-3.5 py-2 border border-[var(--border-default)] text-[var(--fg-secondary)] text-xs leading-normal">${this.parser.parseInline(cell.tokens)}</td>`
                )
                .join("");
              return `<tr class="hover:bg-[var(--bg-surface-hover)] transition-colors">${cells}</tr>`;
            })
            .join("");

          return `<div class="my-4 overflow-x-auto rounded-lg border border-[var(--border-default)] shadow-sm">
            <table class="w-full text-left border-collapse">${header ? `<thead><tr>${header}</tr></thead>` : ""}<tbody>${rows}</tbody></table>
          </div>\n`;
        },
        link({ href, title, tokens }) {
          const text = this.parser.parseInline(tokens);
          const isAnchor = (href || "").startsWith("#");
          const isExternal = /^(?:https?:\/\/|mailto:|tel:)/i.test(href || "");
          const titleAttr = title ? ` title="${escapeHtml(title)}"` : "";

          if (isAnchor) {
            return `<a href="${href}" class="markdown-anchor-link text-[var(--accent-primary)] hover:underline inline-flex items-center gap-0.5"${titleAttr}>${text}</a>`;
          }

          if (isExternal) {
            return `<a href="${href}" class="markdown-external-link text-[var(--accent-primary)] hover:underline inline-flex items-center gap-0.5 cursor-pointer" data-external="true" target="_blank" rel="noopener noreferrer"${titleAttr}>${text}</a>`;
          }

          return `<a href="${href}" class="markdown-file-link text-[var(--accent-primary)] hover:underline inline-flex items-center gap-0.5 cursor-pointer" data-file-link="${href}"${titleAttr}>${text}</a>`;
        },
      },
    });

    let parsedHtml = "";
    try {
      parsedHtml = markedInstance.parse(content) as string;
    } catch (e) {
      console.error("Markdown parse error:", e);
      parsedHtml = `<pre class="p-4 text-red-400 font-mono text-xs">${escapeHtml(content)}</pre>`;
    }

    // Word count & reading time
    const words = content.trim() ? content.trim().split(/\s+/).length : 0;
    const readMin = Math.max(1, Math.ceil(words / 200));

    return {
      html: parsedHtml,
      headings: extractedHeadings,
      taskLines: detectedTaskLines,
      wordCount: words,
      readingTimeMinutes: readMin,
    };
  }, [content]);

  // Heading counts by level (e.g. H1: 2, H2: 5)
  const headingLevelCounts = useMemo(() => {
    const counts: Record<number, number> = { 1: 0, 2: 0, 3: 0, 4: 0, 5: 0, 6: 0 };
    headings.forEach((h) => {
      if (counts[h.level] !== undefined) counts[h.level]++;
    });
    return counts;
  }, [headings]);

  // Filtered headings based on legend search and level filter
  const filteredHeadings = useMemo(() => {
    return headings.filter((h) => {
      if (selectedLevelFilter !== null && h.level > selectedLevelFilter) {
        return false;
      }
      if (legendSearch.trim()) {
        const query = legendSearch.toLowerCase().trim();
        return h.text.toLowerCase().includes(query);
      }
      return true;
    });
  }, [headings, legendSearch, selectedLevelFilter]);

  // Scroll spy: observe headings in markdown content to track active heading
  useEffect(() => {
    const container = containerRef.current;
    if (!container || headings.length === 0) return;

    const handleScroll = () => {
      const headingElements = headings
        .map((h) => ({ id: h.id, el: document.getElementById(h.id) }))
        .filter((item): item is { id: string; el: HTMLElement } => item.el !== null);

      if (headingElements.length === 0) return;

      const containerRect = container.getBoundingClientRect();
      const offsetTop = containerRect.top + 60; // buffer near the top

      let currentActive = headingElements[0].id;
      for (const item of headingElements) {
        const rect = item.el.getBoundingClientRect();
        if (rect.top <= offsetTop) {
          currentActive = item.id;
        } else {
          break;
        }
      }

      setActiveHeadingId(currentActive);
    };

    container.addEventListener("scroll", handleScroll, { passive: true });
    // Initial run
    handleScroll();

    return () => {
      container.removeEventListener("scroll", handleScroll);
    };
  }, [headings, html]);

  // Auto-scroll the active legend item into view in the legend panel
  useEffect(() => {
    if (activeHeadingId && activeItemRef.current && legendScrollRef.current) {
      activeItemRef.current.scrollIntoView({ behavior: "smooth", block: "nearest" });
    }
  }, [activeHeadingId]);

  // Handle clicking a heading in the legend
  const scrollToHeading = useCallback((headingId: string) => {
    const el = document.getElementById(headingId);
    if (el && containerRef.current) {
      el.scrollIntoView({ behavior: "smooth", block: "start" });
      setActiveHeadingId(headingId);

      // Trigger brief highlight animation
      setFlashHeadingId(headingId);
      setTimeout(() => {
        setFlashHeadingId((current) => (current === headingId ? null : current));
      }, 1500);
    }
  }, []);

  // Jump to top / bottom
  const scrollToTop = useCallback(() => {
    if (containerRef.current) {
      containerRef.current.scrollTo({ top: 0, behavior: "smooth" });
    }
  }, []);

  const scrollToBottom = useCallback(() => {
    if (containerRef.current) {
      containerRef.current.scrollTo({ top: containerRef.current.scrollHeight, behavior: "smooth" });
    }
  }, []);

  // Click delegation for interactive markdown items (task checkboxes, code copy, links)
  const handleContentClick = useCallback(
    (e: React.MouseEvent<HTMLDivElement>) => {
      const target = e.target as HTMLElement;

      // 1. Task list checkboxes
      const checkbox = target.closest<HTMLInputElement>("input[data-task-idx]");
      if (checkbox) {
        const taskIdx = parseInt(checkbox.getAttribute("data-task-idx") || "-1", 10);
        if (taskIdx >= 0 && taskIdx < taskLines.length) {
          const lineIdx = taskLines[taskIdx];
          const lines = content.split("\n");
          const targetLine = lines[lineIdx];
          if (targetLine) {
            const isChecked = /\[[xX]\]/.test(targetLine);
            const toggledLine = targetLine.replace(
              /^(\s*[-*+]\s+\[)[ xX](\])/,
              (_m, pre, post) => `${pre}${isChecked ? " " : "x"}${post}`
            );
            lines[lineIdx] = toggledLine;
            const newContent = lines.join("\n");

            // Update store
            setFiles((prev) =>
              prev.map((f) =>
                f.id === file.id
                  ? { ...f, content: newContent, modified: newContent !== (f.savedContent ?? "") }
                  : f
              )
            );
          }
        }
        return;
      }

      // 2. Code Block Copy Button
      const copyBtn = target.closest<HTMLButtonElement>(".copy-code-btn");
      if (copyBtn) {
        e.preventDefault();
        e.stopPropagation();
        const encoded = copyBtn.getAttribute("data-code") || "";
        const codeId = copyBtn.getAttribute("data-btn-id") || "";
        const codeText = decodeURIComponent(encoded);

        navigator.clipboard.writeText(codeText).then(() => {
          setCopiedCodeId(codeId);
          const span = copyBtn.querySelector("span");
          if (span) span.textContent = "Copied!";
          copyBtn.classList.add("text-emerald-400");
          setTimeout(() => {
            if (span) span.textContent = "Copy";
            copyBtn.classList.remove("text-emerald-400");
            setCopiedCodeId((cur) => (cur === codeId ? null : cur));
          }, 2000);
        });
        return;
      }

      // 3. Heading Anchor Links (#)
      const headingAnchor = target.closest<HTMLAnchorElement>("[data-heading-anchor]");
      if (headingAnchor) {
        e.preventDefault();
        const headingId = headingAnchor.getAttribute("data-heading-anchor");
        if (headingId) {
          scrollToHeading(headingId);
        }
        return;
      }

      // 4. Internal Anchor Links (href="#...")
      const anchorLink = target.closest<HTMLAnchorElement>("a.markdown-anchor-link");
      if (anchorLink) {
        const href = anchorLink.getAttribute("href");
        if (href && href.startsWith("#")) {
          e.preventDefault();
          const targetId = href.slice(1);
          scrollToHeading(targetId);
        }
        return;
      }

      // 5. External Links
      const externalLink = target.closest<HTMLAnchorElement>("a[data-external='true']");
      if (externalLink) {
        e.preventDefault();
        const href = externalLink.getAttribute("href");
        if (href) {
          try {
            BrowserOpenURL(href);
          } catch {
            window.open(href, "_blank", "noopener,noreferrer");
          }
        }
        return;
      }

      // 6. Relative File Links (e.g. ./foo.ts or doc.md)
      const fileLink = target.closest<HTMLAnchorElement>("a[data-file-link]");
      if (fileLink) {
        e.preventDefault();
        const relPath = fileLink.getAttribute("data-file-link");
        if (relPath) {
          // Resolve relative to current file's folder
          const currentDir = file.path.substring(0, file.path.lastIndexOf("/"));
          const cleanRel = relPath.replace(/^\.\//, "");
          const fullPath = currentDir ? `${currentDir}/${cleanRel}` : cleanRel;
          globalOpenFile(fullPath);
        }
        return;
      }
    },
    [content, file.id, file.path, scrollToHeading, setFiles, taskLines]
  );

  return (
    <div className="flex flex-col h-full w-full bg-[var(--bg-app)] text-[var(--fg-primary)] overflow-hidden select-text">
      {/* Top Header / Metadata Bar */}
      <div className="flex items-center justify-between px-3 py-1.5 bg-[var(--bg-sidebar)] border-b border-[var(--border-default)] text-[11px] text-[var(--fg-secondary)] shrink-0 select-none">
        {/* Left: Document info */}
        <div className="flex items-center gap-2.5 min-w-0">
          <span className="flex items-center gap-1.5 font-mono text-[var(--fg-primary)] font-medium truncate">
            <IconFileText className="size-3.5 text-sky-400 shrink-0" />
            <span className="truncate">{file.name}</span>
          </span>

          <span className="text-[var(--border-default)]">|</span>

          <span className="flex items-center gap-1 text-[var(--fg-tertiary)] font-mono text-[10.5px]">
            <span>{wordCount.toLocaleString()} words</span>
            <span>•</span>
            <span className="flex items-center gap-0.5">
              <IconClock className="size-3" />
              {readingTimeMinutes} min read
            </span>
          </span>

          {headings.length > 0 && (
            <>
              <span className="text-[var(--border-default)]">|</span>
              <span className="flex items-center gap-1 text-[var(--fg-tertiary)] font-mono text-[10.5px]">
                <IconList className="size-3 text-[var(--accent-primary)]" />
                <span>{headings.length} sections</span>
              </span>
            </>
          )}
        </div>

        {/* Right: Quick actions */}
        <div className="flex items-center gap-1.5 shrink-0">
          <button
            onClick={scrollToTop}
            title="Scroll to Top"
            className="p-1 rounded text-[var(--fg-tertiary)] hover:text-[var(--fg-primary)] hover:bg-[var(--bg-surface-hover)] transition-colors cursor-pointer"
          >
            <IconArrowUp className="size-3.5" />
          </button>
          <button
            onClick={scrollToBottom}
            title="Scroll to Bottom"
            className="p-1 rounded text-[var(--fg-tertiary)] hover:text-[var(--fg-primary)] hover:bg-[var(--bg-surface-hover)] transition-colors cursor-pointer"
          >
            <IconArrowDown className="size-3.5" />
          </button>

          <span className="w-[1px] h-3.5 bg-[var(--border-default)] mx-0.5" />

          {/* Toggle Legend Sidebar */}
          <button
            onClick={() => setLegendOpen((v) => !v)}
            title={legendOpen ? "Collapse Section Legend" : "Open Section Legend"}
            className={cn(
              "flex items-center gap-1 px-2 py-0.5 rounded text-[10.5px] border transition-colors cursor-pointer font-sans",
              legendOpen
                ? "bg-[var(--accent-primary)]/15 border-[var(--accent-primary)]/40 text-[var(--accent-primary)] font-medium"
                : "border-[var(--border-default)] text-[var(--fg-tertiary)] hover:text-[var(--fg-primary)] hover:bg-[var(--bg-surface-hover)]"
            )}
          >
            {legendOpen ? (
              <IconLayoutSidebarRightCollapse className="size-3.5" />
            ) : (
              <IconLayoutSidebarRight className="size-3.5" />
            )}
            <span>Legend</span>
            {headings.length > 0 && (
              <span className="px-1 py-0.2 rounded-full text-[9px] bg-black/20 font-mono">
                {headings.length}
              </span>
            )}
          </button>

        </div>
      </div>

      {/* Main Workspace Area (Markdown Document + Section Legend Navigation) */}
      <div className="flex-1 flex min-h-0 min-w-0 overflow-hidden relative">
        {/* Markdown Content Pane */}
        <div
          ref={containerRef}
          onClick={handleContentClick}
          className="flex-1 h-full overflow-y-auto px-6 py-6 scroll-smooth select-text"
        >
          <div className="max-w-4xl mx-auto pb-16">
            <style>{`
              .markdown-heading {
                position: relative;
                scroll-margin-top: 1.5rem;
                display: flex;
                align-items: baseline;
                gap: 0.5rem;
              }
              .markdown-heading:hover .heading-anchor-link {
                opacity: 1;
              }
              .heading-anchor-link {
                opacity: 0;
                color: var(--accent-primary);
                text-decoration: none;
                font-weight: 700;
                font-family: monospace;
                font-size: 0.85em;
                transition: opacity 0.15s ease;
                user-select: none;
                cursor: pointer;
              }
              .heading-flash {
                animation: headingPulse 1.5s ease-out;
              }
              @keyframes headingPulse {
                0% { background-color: color-mix(in srgb, var(--accent-primary) 30%, transparent); }
                100% { background-color: transparent; }
              }
            `}</style>

            <div
              className={cn(
                "markdown-body select-text",
                flashHeadingId && `[&_#${flashHeadingId}]:heading-flash`
              )}
              dangerouslySetInnerHTML={{ __html: html }}
            />
          </div>
        </div>

        {/* Section Legend Navigation Panel */}
        {legendOpen && (
          <div className="w-72 shrink-0 border-l border-[var(--border-default)] bg-[var(--bg-sidebar)] flex flex-col h-full overflow-hidden select-none">
            {/* Legend Header */}
            <div className="p-2.5 border-b border-[var(--border-default)] bg-[var(--bg-panel)]/50 space-y-2 shrink-0">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-1.5 text-xs font-semibold text-[var(--fg-primary)]">
                  <IconList className="size-3.5 text-[var(--accent-primary)]" />
                  <span>Section Legend</span>
                </div>
                <span className="px-1.5 py-0.5 text-[10px] font-mono rounded bg-[var(--bg-elevated)] border border-[var(--border-default)] text-[var(--fg-tertiary)]">
                  {headings.length} {headings.length === 1 ? "heading" : "headings"}
                </span>
              </div>

              {/* Heading Level Pills / Quick Filters */}
              <div className="flex items-center gap-1 flex-wrap pt-0.5">
                <button
                  onClick={() => setSelectedLevelFilter(null)}
                  className={cn(
                    "px-1.5 py-0.5 text-[9.5px] font-mono rounded border transition-colors cursor-pointer",
                    selectedLevelFilter === null
                      ? "bg-[var(--accent-primary)] text-[var(--fg-on-active,white)] border-[var(--accent-primary)] font-semibold"
                      : "bg-[var(--bg-app)] text-[var(--fg-tertiary)] border-[var(--border-default)] hover:text-[var(--fg-primary)]"
                  )}
                  title="Show all heading levels (H1-H6)"
                >
                  All
                </button>

                {[1, 2, 3, 4, 5, 6].map((lvl) => {
                  const count = headingLevelCounts[lvl];
                  if (count === 0) return null;
                  const isSelected = selectedLevelFilter === lvl;
                  const colors = LEVEL_COLORS[lvl];

                  return (
                    <button
                      key={lvl}
                      onClick={() => setSelectedLevelFilter(isSelected ? null : lvl)}
                      className={cn(
                        "px-1.5 py-0.5 text-[9.5px] font-mono rounded border transition-all cursor-pointer flex items-center gap-1",
                        isSelected
                          ? `${colors.badge} font-bold ring-1 ring-white/20`
                          : "bg-[var(--bg-app)] text-[var(--fg-tertiary)] border-[var(--border-default)] hover:text-[var(--fg-primary)]"
                      )}
                      title={`Filter up to H${lvl} (${count} headings)`}
                    >
                      <span>H{lvl}</span>
                      <span className="text-[8.5px] opacity-75">{count}</span>
                    </button>
                  );
                })}
              </div>

              {/* Search filter in legend */}
              <div className="relative">
                <IconSearch className="size-3 text-[var(--fg-tertiary)] absolute left-2 top-2 pointer-events-none" />
                <input
                  type="text"
                  value={legendSearch}
                  onChange={(e) => setLegendSearch(e.target.value)}
                  placeholder="Filter sections..."
                  className="w-full bg-[var(--bg-app)] border border-[var(--border-default)] pl-6 pr-6 py-1 text-[11px] rounded text-[var(--fg-primary)] placeholder:text-[var(--fg-tertiary)] focus:outline-none focus:border-[var(--accent-primary)] font-mono"
                />
                {legendSearch && (
                  <button
                    onClick={() => setLegendSearch("")}
                    className="absolute right-1.5 top-1.5 text-[var(--fg-tertiary)] hover:text-[var(--fg-primary)] cursor-pointer"
                  >
                    <IconX className="size-3" />
                  </button>
                )}
              </div>
            </div>

            {/* Legend List */}
            <div ref={legendScrollRef} className="flex-1 overflow-y-auto p-1.5 space-y-0.5">
              {filteredHeadings.length === 0 ? (
                <div className="p-4 text-center text-xs text-[var(--fg-tertiary)]">
                  {headings.length === 0 ? (
                    <div className="space-y-1">
                      <IconBook className="size-6 mx-auto stroke-[1.2] text-[var(--fg-disabled)] opacity-50" />
                      <p>No headings found</p>
                      <p className="text-[10px] text-[var(--fg-tertiary)]">
                        Add # H1, ## H2 in markdown to see sections
                      </p>
                    </div>
                  ) : (
                    <p>No sections matching &quot;{legendSearch}&quot;</p>
                  )}
                </div>
              ) : (
                filteredHeadings.map((h) => {
                  const isActive = activeHeadingId === h.id;
                  const colors = LEVEL_COLORS[h.level] || LEVEL_COLORS[6];

                  // Indentation based on heading level
                  const paddingLeft =
                    h.level === 1
                      ? "pl-2"
                      : h.level === 2
                      ? "pl-4"
                      : h.level === 3
                      ? "pl-6"
                      : h.level === 4
                      ? "pl-8"
                      : "pl-10";

                  return (
                    <button
                      key={h.id}
                      ref={isActive ? (el) => { activeItemRef.current = el; } : undefined}
                      onClick={() => scrollToHeading(h.id)}
                      className={cn(
                        "w-full text-left py-1.5 pr-2 rounded text-xs transition-all relative flex items-start gap-1.5 cursor-pointer group",
                        paddingLeft,
                        isActive
                          ? "bg-[var(--accent-primary)]/15 text-[var(--fg-primary)] font-semibold shadow-xs"
                          : "text-[var(--fg-secondary)] hover:bg-[var(--bg-surface-hover)] hover:text-[var(--fg-primary)]"
                      )}
                      title={`${h.text} (Level ${h.level})`}
                    >
                      {/* Active Indicator bar */}
                      {isActive && (
                        <span className="absolute left-0 top-1 bottom-1 w-1 rounded-r bg-[var(--accent-primary)]" />
                      )}

                      {/* Level Badge */}
                      <span
                        className={cn(
                          "px-1 py-0.2 text-[9px] font-mono rounded border uppercase shrink-0 mt-0.5",
                          colors.badge,
                          isActive && "ring-1 ring-[var(--accent-primary)]"
                        )}
                      >
                        H{h.level}
                      </span>

                      {/* Heading Text */}
                      <span className="truncate flex-1 leading-snug">
                        {h.text}
                      </span>
                    </button>
                  );
                })
              )}
            </div>

            {/* Legend Footer */}
            <div className="p-2 border-t border-[var(--border-default)] bg-[var(--bg-panel)]/40 flex items-center justify-between text-[10px] text-[var(--fg-tertiary)] shrink-0">
              <span className="font-mono">
                {activeHeadingId
                  ? headings.find((h) => h.id === activeHeadingId)?.text || "Section Navigation"
                  : "Overview"}
              </span>
              <button
                onClick={scrollToTop}
                className="hover:text-[var(--fg-primary)] underline cursor-pointer"
              >
                Top
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

function escapeHtml(text: string): string {
  return text
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#039;");
}
