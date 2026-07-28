import { useState, useEffect, useCallback } from "react";
import { cn } from "../lib/utils";
import { GetFileDiff, GetRepoRoot, GetRelPath, StageDiffHunk } from "../../wailsjs/go/main/App";
import { git } from "../../wailsjs/go/models";

interface DiffViewProps {
  path: string;
  onClose?: () => void;
}

export function DiffView({ path, onClose }: DiffViewProps) {
  const [diff, setDiff] = useState<git.FileDiff | null>(null);
  const [loading, setLoading] = useState(true);
  const [viewMode, setViewMode] = useState<"side-by-side" | "unified">("unified");
  const [scrolledLines, setScrolledLines] = useState(0);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      setLoading(true);
      try {
        const repoPath = await GetRepoRoot(path);
        if (!repoPath) { setLoading(false); return; }
        const relPath = await GetRelPath(path);
        const fd = await GetFileDiff(repoPath, relPath);
        if (!cancelled) setDiff(fd || null);
      } catch { }
      if (!cancelled) setLoading(false);
    }
    load();
    return () => { cancelled = true; };
  }, [path]);

  const handleStageAll = useCallback(async () => {
    if (!diff) return;
    try {
      const repoPath = await GetRepoRoot(path);
      const relPath = await GetRelPath(path);
      for (let i = 0; i < diff.hunks.length; i++) {
        await StageDiffHunk(repoPath, relPath, i);
      }
    } catch (e) { console.error(e); }
  }, [diff, path]);

  const handleStageHunk = useCallback(async (hunkIdx: number) => {
    try {
      const repoPath = await GetRepoRoot(path);
      const relPath = await GetRelPath(path);
      await StageDiffHunk(repoPath, relPath, hunkIdx);
    } catch (e) { console.error(e); }
  }, [path]);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full text-sm text-muted-foreground">
        Loading diff...
      </div>
    );
  }

  if (!diff || diff.hunks.length === 0) {
    return (
      <div className="flex flex-col h-full">
        <div className="flex items-center justify-between px-3 py-1 border-b bg-muted/20 text-xs shrink-0">
          <span className="truncate">{path}</span>
          <div className="flex items-center gap-2">
            <span className="text-green-400">No changes</span>
            {onClose && (
              <button className="p-0.5 hover:bg-accent rounded" onClick={onClose}>✕</button>
            )}
          </div>
        </div>
        <div className="flex items-center justify-center h-full text-sm text-muted-foreground">
          No changes detected
        </div>
      </div>
    );
  }

  return (
    <div className="flex flex-col h-full bg-[#0d1117]">
      <div className="flex items-center justify-between px-3 py-1 border-b bg-muted/20 text-xs shrink-0">
        <span className="truncate font-mono">{path}</span>
        <div className="flex items-center gap-2">
          <button
            className={cn("px-2 py-0.5 rounded text-xs", viewMode === "unified" ? "bg-accent" : "hover:bg-accent")}
            onClick={() => setViewMode("unified")}
          >Unified</button>
          <button
            className={cn("px-2 py-0.5 rounded text-xs", viewMode === "side-by-side" ? "bg-accent" : "hover:bg-accent")}
            onClick={() => setViewMode("side-by-side")}
          >Side-by-Side</button>
          <button
            className="px-2 py-0.5 rounded hover:bg-accent text-green-400"
            onClick={handleStageAll}
            title="Stage all changes"
          >Stage All</button>
          {onClose && (
            <button className="p-0.5 hover:bg-accent rounded" onClick={onClose}>✕</button>
          )}
        </div>
      </div>

      <div className="flex-1 overflow-auto">
        {viewMode === "unified" ? (
          <UnifiedDiffView diff={diff} onStageHunk={handleStageHunk} />
        ) : (
          <SideBySideDiffView diff={diff} />
        )}
      </div>
    </div>
  );
}

function UnifiedDiffView({ diff, onStageHunk }: { diff: git.FileDiff; onStageHunk: (i: number) => void }) {
  return (
    <div className="font-mono text-xs">
      {diff.hunks.map((hunk, hi) => (
        <div key={hi}>
          <div className="flex items-center justify-between px-3 py-1 bg-muted/30 text-[10px] text-muted-foreground font-sans sticky top-0">
            <span>{hunk.header}</span>
            <button
              className="px-2 py-0.5 rounded hover:bg-accent text-green-400 text-[10px]"
              onClick={(e) => { e.stopPropagation(); onStageHunk(hi); }}
              title="Stage this hunk"
            >+ Stage Hunk</button>
          </div>
          {hunk.lines.map((line, li) => (
            <DiffLineRow key={li} line={line} />
          ))}
        </div>
      ))}
    </div>
  );
}

function SideBySideDiffView({ diff }: { diff: git.FileDiff }) {
  return (
    <div className="flex h-full font-mono text-xs">
      <div className="flex-1 border-r border-border/30">
        <div className="px-2 py-1 text-[10px] text-muted-foreground bg-muted/20 border-b sticky top-0">Original</div>
        {diff.hunks.map((hunk, hi) => (
          <div key={hi}>
            <div className="px-3 py-0.5 bg-muted/30 text-[10px] text-muted-foreground">{hunk.header}</div>
            {hunk.lines.map((line, li) => (
              <div key={li} className={cn(
                "px-3 py-0.5 leading-5 flex items-center",
                line.type === "deleted" && "bg-[rgba(239,68,68,.1)] text-red-400",
                line.type === "context" && "text-gray-400",
              )}>
                <span className="w-8 shrink-0 text-right mr-2 opacity-40">
                  {line.type !== "added" ? (line.oldLine > 0 ? line.oldLine : "") : ""}
                </span>
                <span className="flex-1 whitespace-pre">{line.type === "added" ? "" : line.content}</span>
              </div>
            ))}
          </div>
        ))}
      </div>
      <div className="flex-1">
        <div className="px-2 py-1 text-[10px] text-muted-foreground bg-muted/20 border-b sticky top-0">Modified</div>
        {diff.hunks.map((hunk, hi) => (
          <div key={hi}>
            <div className="px-3 py-0.5 bg-muted/30 text-[10px] text-muted-foreground">{hunk.header}</div>
            {hunk.lines.map((line, li) => (
              <div key={li} className={cn(
                "px-3 py-0.5 leading-5 flex items-center",
                line.type === "added" && "bg-[rgba(34,197,94,.1)] text-green-400",
                line.type === "context" && "text-gray-400",
              )}>
                <span className="w-8 shrink-0 text-right mr-2 opacity-40">
                  {line.type !== "deleted" ? (line.newLine > 0 ? line.newLine : "") : ""}
                </span>
                <span className="flex-1 whitespace-pre">{line.type === "deleted" ? "" : line.content}</span>
              </div>
            ))}
          </div>
        ))}
      </div>
    </div>
  );
}

function DiffLineRow({ line }: { line: git.DiffLine }) {
  return (
    <div className={cn(
      "px-4 py-0.5 leading-5 flex",
      line.type === "added" && "bg-[rgba(34,197,94,.1)] text-green-400",
      line.type === "deleted" && "bg-[rgba(239,68,68,.1)] text-red-400",
      line.type === "context" && "text-gray-400",
    )}>
      <span className="w-8 shrink-0 text-right mr-2 opacity-40 select-none">
        {line.oldLine > 0 ? line.oldLine : ""}
      </span>
      <span className="w-8 shrink-0 text-right mr-2 opacity-40 select-none">
        {line.newLine > 0 ? line.newLine : ""}
      </span>
      <span className="w-4 shrink-0 select-none">
        {line.type === "added" ? "+" : line.type === "deleted" ? "-" : " "}
      </span>
      <span className="flex-1 whitespace-pre">{line.content}</span>
    </div>
  );
}
