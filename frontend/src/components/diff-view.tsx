import React, { useMemo, useState } from "react";
import { IconFileDiff, IconFileText, IconCode } from "@tabler/icons-react";
import { 
  ChevronDown, 
  ChevronRight, 
  ChevronsDownUp, 
  ChevronsUpDown, 
  FoldVertical 
} from "lucide-react";

type DiffLineType = "file" | "hunk" | "add" | "del" | "meta" | "normal";

interface DiffLine {
  id: number;
  text: string;
  type: DiffLineType;
  oldLine?: number;
  newLine?: number;
}

export interface DiffSection {
  path: string;
  lines: DiffLine[];
  additions: number;
  deletions: number;
}

function extractPathFromDiffLine(line: string): string {
  const match = line.match(/\sb\/(.+)$/);
  return match ? match[1] : "";
}

function extractPathFromBanner(line: string): string {
  const match = line.match(/^(?:\+\+\+|---)\s+b\/(.+)$/);
  return match ? match[1] : "";
}

function parseHunkHeader(line: string): { oldStart: number; newStart: number } {
  const m = line.match(/@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@/);
  if (!m) return { oldStart: 0, newStart: 0 };
  return { oldStart: parseInt(m[1], 10), newStart: parseInt(m[2], 10) };
}

function parseDiff(content: string): DiffSection[] {
  const lines = content.split("\n");
  const sections: DiffSection[] = [];
  let current: DiffLine[] = [];
  let currentPath = "";
  let oldLine = 0;
  let newLine = 0;

  const flush = () => {
    if (current.length > 0) {
      const adds = current.filter(l => l.type === "add").length;
      const dels = current.filter(l => l.type === "del").length;
      sections.push({ 
        path: currentPath, 
        lines: current, 
        additions: adds, 
        deletions: dels 
      });
    }
    current = [];
    currentPath = "";
  };

  lines.forEach((line, idx) => {
    if (line.startsWith("diff --git")) {
      flush();
      currentPath = extractPathFromDiffLine(line);
      oldLine = 0;
      newLine = 0;
      current.push({ id: idx, text: line, type: "file" });
      return;
    }

    let type: DiffLineType = "normal";
    let lOld: number | undefined;
    let lNew: number | undefined;

    if (line.startsWith("@@")) {
      type = "hunk";
      const { oldStart, newStart } = parseHunkHeader(line);
      oldLine = oldStart;
      newLine = newStart;
    } else if (line.startsWith("+") && !line.startsWith("+++")) {
      type = "add";
      lNew = newLine;
      newLine += 1;
    } else if (line.startsWith("-") && !line.startsWith("---")) {
      type = "del";
      lOld = oldLine;
      oldLine += 1;
    } else if (
      line.startsWith("index ") ||
      line.startsWith("---") ||
      line.startsWith("+++") ||
      line.startsWith("new file") ||
      line.startsWith("deleted file") ||
      line.startsWith("similarity index") ||
      line.startsWith("rename from") ||
      line.startsWith("rename to") ||
      line.startsWith("Binary files") ||
      line.startsWith("\\ No newline")
    ) {
      type = "meta";
      if (!currentPath) {
        const p = extractPathFromBanner(line);
        if (p) currentPath = p;
      }
    } else if (oldLine > 0 || newLine > 0) {
      // Context line inside a hunk
      lOld = oldLine;
      lNew = newLine;
      oldLine += 1;
      newLine += 1;
    }

    current.push({ id: idx, text: line, type, oldLine: lOld, newLine: lNew });
  });
  flush();
  return sections;
}

interface DiffViewProps {
  content: string;
  onOpenFile?: (path: string) => void;
  onOpenDiff?: (path: string) => void;
  emptyText?: string;
}

export function DiffView({ content, onOpenFile, onOpenDiff, emptyText }: DiffViewProps) {
  const sections = useMemo(() => parseDiff(content), [content]);
  const [collapsedSections, setCollapsedSections] = useState<Record<string, boolean>>({});

  const totalAdditions = useMemo(() => sections.reduce((acc, s) => acc + s.additions, 0), [sections]);
  const totalDeletions = useMemo(() => sections.reduce((acc, s) => acc + s.deletions, 0), [sections]);

  const areAnyCollapsed = useMemo(() => {
    return sections.some((s, idx) => collapsedSections[s.path || idx]);
  }, [sections, collapsedSections]);

  const handleToggleAll = () => {
    if (areAnyCollapsed) {
      // Expand all
      setCollapsedSections({});
    } else {
      // Collapse all
      const next: Record<string, boolean> = {};
      sections.forEach((s, idx) => {
        next[s.path || idx] = true;
      });
      setCollapsedSections(next);
    }
  };

  const toggleSection = (key: string | number) => {
    setCollapsedSections(prev => ({
      ...prev,
      [key]: !prev[key]
    }));
  };

  if (!content || content.trim().length === 0) {
    return (
      <div className="flex flex-col items-center justify-center h-full text-center space-y-2 text-[var(--fg-tertiary)] select-none">
        <IconFileDiff className="w-10 h-10 text-[var(--fg-disabled)]" />
        <span className="text-xs font-mono italic">{emptyText || "No diff available."}</span>
      </div>
    );
  }

  return (
    <div className="font-mono text-[11px] space-y-2 select-text">
      {/* Diff Accordions Toolbar */}
      {sections.length > 0 && (
        <div className="flex items-center justify-between px-3 py-2 rounded-[8px] bg-surface-hover dark:bg-[#1E1E22] border border-border text-xs">
          <div className="flex items-center gap-2">
            <IconFileDiff className="w-4 h-4 text-primary dark:text-info" />
            <span className="font-semibold text-foreground">
              {sections.length} {sections.length === 1 ? "file changed" : "files changed"}
            </span>
            <span className="text-[11px] font-mono">
              {totalAdditions > 0 && (
                <span className="text-emerald-600 dark:text-emerald-400 font-semibold">+{totalAdditions}</span>
              )}{" "}
              {totalDeletions > 0 && (
                <span className="text-rose-600 dark:text-rose-400 font-semibold">-{totalDeletions}</span>
              )}
            </span>
          </div>

          <button
            type="button"
            onClick={handleToggleAll}
            className="px-2.5 py-1 rounded-[6px] bg-surface-hover hover:bg-surface-hover dark:hover:bg-[#38383C] text-foreground border border-border dark:border-border transition-colors cursor-pointer flex items-center gap-1.5 text-[11px] font-sans font-medium shadow-2xs"
            title={areAnyCollapsed ? "Expand all diffs" : "Collapse all diffs"}
          >
            {areAnyCollapsed ? (
              <>
                <ChevronsUpDown className="w-3.5 h-3.5 text-foreground-subtle" />
                <span>Expand all diffs</span>
              </>
            ) : (
              <>
                <ChevronsDownUp className="w-3.5 h-3.5 text-foreground-subtle" />
                <span>Collapse all diffs</span>
              </>
            )}
          </button>
        </div>
      )}

      {/* File Accordions */}
      {sections.map((section, si) => {
        const secKey = section.path || si;
        const isCollapsed = !!collapsedSections[secKey];
        const fileName = section.path ? section.path.split("/").pop() : section.lines[0]?.text;

        return (
          <div
            key={si}
            className="rounded-[8px] bg-white dark:bg-[#18181A] border border-border overflow-hidden shadow-2xs"
          >
            {/* Accordion File Header */}
            <div
              onClick={() => toggleSection(secKey)}
              className="px-3 py-2 bg-surface dark:bg-[#1F1F22] hover:bg-surface-hover dark:hover:bg-[#26262A] border-b border-border flex items-center justify-between cursor-pointer select-none transition-colors"
            >
              <div className="flex items-center gap-2 min-w-0 flex-1 pr-2">
                <span className="text-foreground-subtle shrink-0">
                  {isCollapsed ? <ChevronRight className="w-3.5 h-3.5" /> : <ChevronDown className="w-3.5 h-3.5" />}
                </span>
                <IconFileDiff className="w-4 h-4 shrink-0 text-primary dark:text-info" />
                <span className="font-semibold text-xs text-foreground truncate font-mono">
                  {section.path ? section.path : section.lines[0]?.text}
                </span>
              </div>

              <div className="flex items-center gap-2 shrink-0 font-sans" onClick={e => e.stopPropagation()}>
                {(section.additions > 0 || section.deletions > 0) && (
                  <div className="flex items-center gap-1 font-mono text-[11px] pr-1">
                    {section.additions > 0 && (
                      <span className="text-emerald-600 dark:text-emerald-400 font-semibold">+{section.additions}</span>
                    )}
                    {section.deletions > 0 && (
                      <span className="text-rose-600 dark:text-rose-400 font-semibold">-{section.deletions}</span>
                    )}
                  </div>
                )}

                <button
                  type="button"
                  onClick={() => toggleSection(secKey)}
                  className="px-2 py-0.5 rounded bg-surface-hover hover:bg-surface-hover dark:hover:bg-[#38383C] text-foreground-subtle dark:text-foreground-secondary border border-border dark:border-border transition-colors cursor-pointer flex items-center gap-1 text-[10.5px]"
                  title={isCollapsed ? "Expand diff" : "Collapse diff"}
                >
                  <FoldVertical className="w-3 h-3" />
                  <span>{isCollapsed ? "Expand diff" : "Collapse diff"}</span>
                </button>

                {onOpenDiff && section.path && (
                  <button
                    type="button"
                    onClick={() => onOpenDiff(section.path)}
                    className="px-2 py-0.5 rounded bg-primary/10 dark:bg-card hover:bg-primary/10 dark:hover:bg-[#2D3E56] text-primary dark:text-[#93C5FD] border border-[#BFDBFE] dark:border-[#3B82F6]/30 transition-colors cursor-pointer flex items-center gap-1 text-[10.5px]"
                    title="Open diff in editor"
                  >
                    <IconCode className="w-3 h-3" />
                    <span>Diff</span>
                  </button>
                )}

                {onOpenFile && section.path && (
                  <button
                    type="button"
                    onClick={() => onOpenFile(section.path)}
                    className="px-2 py-0.5 rounded bg-surface-hover hover:bg-surface-hover dark:hover:bg-[#38383C] text-foreground-subtle dark:text-foreground-secondary border border-border dark:border-border transition-colors cursor-pointer flex items-center gap-1 text-[10.5px]"
                    title="Open file in editor"
                  >
                    <IconFileText className="w-3 h-3" />
                    <span>Open</span>
                  </button>
                )}
              </div>
            </div>

            {/* Accordion Lines */}
            {!isCollapsed && (
              <div className="overflow-x-auto">
                {section.lines.map((line) => {
                  if (line.type === "file") return null;

                  const gutterOld = line.oldLine !== undefined ? (
                    <span className="w-9 shrink-0 text-right pr-2 text-foreground-subtle dark:text-foreground-subtle select-none text-[10px]">
                      {line.oldLine}
                    </span>
                  ) : (
                    <span className="w-9 shrink-0 select-none" />
                  );
                  const gutterNew = line.newLine !== undefined ? (
                    <span className="w-9 shrink-0 text-right pr-2 text-foreground-subtle dark:text-foreground-subtle select-none text-[10px]">
                      {line.newLine}
                    </span>
                  ) : (
                    <span className="w-9 shrink-0 select-none" />
                  );

                  if (line.type === "hunk") {
                    return (
                      <div
                        key={line.id}
                        className="my-0.5 px-2.5 py-0.5 bg-primary/10 dark:bg-primary/10/40 text-primary dark:text-info font-bold text-[10px] border-l-2 border-blue-500"
                      >
                        {line.text}
                      </div>
                    );
                  }
                  if (line.type === "add") {
                    return (
                      <div
                        key={line.id}
                        className="flex bg-success/10/70 dark:bg-success/10/40 text-success dark:text-success border-l-2 border-[#16A34A] leading-[18px]"
                      >
                        {gutterOld}
                        {gutterNew}
                        <span className="px-2 py-0.5 flex-1 whitespace-pre">{line.text}</span>
                      </div>
                    );
                  }
                  if (line.type === "del") {
                    return (
                      <div
                        key={line.id}
                        className="flex bg-destructive/10/70 dark:bg-destructive/10/40 text-destructive dark:text-destructive border-l-2 border-[#DC2626] leading-[18px]"
                      >
                        {gutterOld}
                        {gutterNew}
                        <span className="px-2 py-0.5 flex-1 whitespace-pre">{line.text}</span>
                      </div>
                    );
                  }
                  if (line.type === "meta") {
                    return (
                      <div key={line.id} className="px-2.5 py-0.5 text-foreground-subtle italic text-[10px]">
                        {line.text}
                      </div>
                    );
                  }
                  return (
                    <div key={line.id} className="flex text-foreground-subtle dark:text-foreground-secondary leading-[18px]">
                      {gutterOld}
                      {gutterNew}
                      <span className="px-2 py-0.5 flex-1 whitespace-pre">{line.text}</span>
                    </div>
                  );
                })}

                {/* Bottom Collapse diff bar */}
                <div className="px-3 py-1 bg-surface dark:bg-[#18181A] border-t border-border flex items-center justify-between text-[10px] text-foreground-subtle font-sans">
                  <span className="truncate max-w-[300px]">{fileName}</span>
                  <button
                    type="button"
                    onClick={() => toggleSection(secKey)}
                    className="hover:text-foreground cursor-pointer font-medium"
                  >
                    Collapse diff
                  </button>
                </div>
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}
