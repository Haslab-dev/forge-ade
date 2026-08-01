import React, { useMemo } from "react";
import { IconFileDiff, IconFileText, IconCode } from "@tabler/icons-react";

type DiffLineType = "file" | "hunk" | "add" | "del" | "meta" | "normal";

interface DiffLine {
  id: number;
  text: string;
  type: DiffLineType;
  oldLine?: number;
  newLine?: number;
}

interface DiffSection {
  path: string;
  lines: DiffLine[];
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
      sections.push({ path: currentPath, lines: current });
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

  if (!content || content.trim().length === 0) {
    return (
      <div className="flex flex-col items-center justify-center h-full text-center space-y-2 text-[var(--fg-tertiary)] select-none">
        <IconFileDiff className="w-10 h-10 text-[var(--fg-disabled)]" />
        <span className="text-xs font-mono italic">{emptyText || "No diff available."}</span>
      </div>
    );
  }

  return (
    <div className="font-mono text-[11px] space-y-0.5 select-text">
      {sections.map((section, si) => (
        <div key={si}>
          <div className="mt-3 mb-1 px-3 py-1.5 bg-blue-950/40 border border-blue-900/60 text-blue-300 rounded flex items-center space-x-2 text-[10px]">
            <IconFileDiff className="w-4 h-4 shrink-0 text-blue-400" />
            <span className="truncate flex-1">
              {section.path ? section.path : section.lines[0]?.text}
            </span>
            {(onOpenFile || onOpenDiff) && (
              <div className="flex items-center space-x-1 shrink-0">
                {onOpenDiff && (
                  <button
                    onClick={() => onOpenDiff(section.path)}
                    className="px-1.5 py-0.5 rounded bg-blue-900/50 border border-blue-700/60 hover:bg-blue-800/70 text-blue-200 hover:text-white transition-colors cursor-pointer flex items-center gap-1"
                    title="Open diff in editor"
                  >
                    <IconCode className="w-3 h-3" />
                    <span>Diff</span>
                  </button>
                )}
                {onOpenFile && (
                  <button
                    onClick={() => onOpenFile(section.path)}
                    className="px-1.5 py-0.5 rounded bg-blue-900/50 border border-blue-700/60 hover:bg-blue-800/70 text-blue-200 hover:text-white transition-colors cursor-pointer flex items-center gap-1"
                    title="Open file in editor"
                  >
                    <IconFileText className="w-3 h-3" />
                    <span>Open</span>
                  </button>
                )}
              </div>
            )}
          </div>
          {section.lines.map((line) => {
            if (line.type === "file") return null;

            const gutterOld = line.oldLine !== undefined ? (
              <span className="w-9 shrink-0 text-right pr-2 text-[var(--fg-tertiary)] select-none">{line.oldLine}</span>
            ) : (
              <span className="w-9 shrink-0 select-none" />
            );
            const gutterNew = line.newLine !== undefined ? (
              <span className="w-9 shrink-0 text-right pr-2 text-[var(--fg-tertiary)] select-none">{line.newLine}</span>
            ) : (
              <span className="w-9 shrink-0 select-none" />
            );

            if (line.type === "hunk") {
              return (
                <div
                  key={line.id}
                  className="my-1 px-2.5 py-1 bg-purple-950/40 text-purple-300 font-bold text-[10px] border-l-2 border-purple-500 rounded-r"
                >
                  {line.text}
                </div>
              );
            }
            if (line.type === "add") {
              return (
                <div
                  key={line.id}
                  className="flex bg-emerald-950/30 text-emerald-300 border-l border-emerald-500"
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
                  className="flex bg-rose-950/30 text-rose-300 border-l border-rose-500"
                >
                  {gutterOld}
                  {gutterNew}
                  <span className="px-2 py-0.5 flex-1 whitespace-pre">{line.text}</span>
                </div>
              );
            }
            if (line.type === "meta") {
              return (
                <div key={line.id} className="px-2.5 py-0.5 text-[var(--fg-tertiary)] italic">
                  {line.text}
                </div>
              );
            }
            return (
              <div key={line.id} className="flex text-[var(--fg-secondary)]">
                {gutterOld}
                {gutterNew}
                <span className="px-2 py-0.5 flex-1 whitespace-pre">{line.text}</span>
              </div>
            );
          })}
        </div>
      ))}
    </div>
  );
}
