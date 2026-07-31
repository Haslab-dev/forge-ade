import React, { useState, useEffect, useMemo } from "react";
import {
  IconGitCommit,
  IconGitBranch,
  IconUser,
  IconCalendar,
  IconRefresh,
  IconFileDiff,
  IconCopy,
  IconCheck,
  IconCornerDownRight,
  IconCode,
} from "@tabler/icons-react";
import { cn } from "../lib/utils";
import { GetGitCommitGraph, GetGitCommitDiff, EventsOn } from "../lib/wails";
import { ResizableSplit } from "../components/resizable-split";

interface CommitNode {
  hash: string;
  short_hash: string;
  parents: string[];
  author_name: string;
  author_email: string;
  timestamp: string;
  message: string;
  graph_prefix: string;
}

interface CommitGraphResult {
  commits: CommitNode[];
  total_count: number;
  has_more: boolean;
  offset: number;
  limit: number;
}

export function GitGraphPanel() {
  const [graphData, setGraphData] = useState<CommitNode[]>([]);
  const [totalCount, setTotalCount] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [offset, setOffset] = useState(0);
  const [loading, setLoading] = useState(false);
  
  const [selectedCommit, setSelectedCommit] = useState<CommitNode | null>(null);
  const [commitDiff, setCommitDiff] = useState<string | null>(null);
  const [loadingDiff, setLoadingDiff] = useState(false);
  const [copiedHash, setCopiedHash] = useState(false);

  useEffect(() => {
    loadGraph(0, true);
  }, []);

  useEffect(() => {
    let timer: any = null;
    const unsubscribe = EventsOn("fs:changed", () => {
      if (timer) clearTimeout(timer);
      timer = setTimeout(() => {
        loadGraph(0, true);
      }, 600);
    });
    return () => {
      if (timer) clearTimeout(timer);
      if (typeof unsubscribe === "function") unsubscribe();
    };
  }, []);

  async function loadGraph(newOffset: number, reset = false) {
    setLoading(true);
    try {
      const res: CommitGraphResult = await GetGitCommitGraph("", newOffset, 50);
      if (res) {
        if (reset) {
          setGraphData(res.commits || []);
        } else {
          setGraphData((prev) => [...prev, ...(res.commits || [])]);
        }
        setTotalCount(res.total_count || 0);
        setHasMore(res.has_more || false);
        setOffset(newOffset);
      }
    } catch (err) {
      console.error("Failed to load git graph:", err);
    } finally {
      setLoading(false);
    }
  }

  async function handleSelectCommit(node: CommitNode) {
    setSelectedCommit(node);
    setLoadingDiff(true);
    try {
      const diffStr = await GetGitCommitDiff("", node.hash);
      setCommitDiff(diffStr);
    } catch (err) {
      setCommitDiff("Failed to load commit diff.");
    } finally {
      setLoadingDiff(false);
    }
  }

  function handleCopyHash(hash: string) {
    navigator.clipboard.writeText(hash);
    setCopiedHash(true);
    setTimeout(() => setCopiedHash(false), 1500);
  }

  // Parses patch diff into formatted color blocks
  const diffLines = useMemo(() => {
    if (!commitDiff) return [];
    const lines = commitDiff.split("\n");
    return lines.map((line, idx) => {
      let type: "file" | "hunk" | "add" | "del" | "meta" | "normal" = "normal";
      if (line.startsWith("diff --git")) type = "file";
      else if (line.startsWith("@@")) type = "hunk";
      else if (line.startsWith("+") && !line.startsWith("+++")) type = "add";
      else if (line.startsWith("-") && !line.startsWith("---")) type = "del";
      else if (line.startsWith("index ") || line.startsWith("---") || line.startsWith("+++")) type = "meta";

      return { id: idx, text: line, type };
    });
  }, [commitDiff]);

  const leftContent = (
    <div className="flex flex-col h-full bg-[var(--bg-app)] overflow-hidden font-sans">
      <div className="flex items-center justify-between px-4 py-2 border-b border-[var(--border-default)] bg-[var(--bg-panel)] shrink-0 select-none">
        <div className="flex items-center space-x-2 font-bold text-xs uppercase tracking-wider text-[var(--fg-tertiary)]">
          <IconGitBranch className="w-4 h-4 text-purple-400" />
          <span>Commit History Graph</span>
          <span className="text-[10px] text-[var(--fg-secondary)] font-mono">({totalCount} commits)</span>
        </div>
        <button
          onClick={() => loadGraph(0, true)}
          disabled={loading}
          className="p-1.5 rounded hover:bg-[var(--bg-surface-hover)] text-[var(--fg-secondary)] hover:text-white transition-all cursor-pointer disabled:opacity-50"
          title="Refresh Graph"
        >
          <IconRefresh className={cn("w-4 h-4", loading && "animate-spin")} />
        </button>
      </div>

      <div className="flex-1 overflow-y-auto font-mono text-[11px] select-text">
        <table className="w-full text-left border-collapse">
          <thead>
            <tr className="border-b border-[var(--border-default)] text-[var(--fg-secondary)] bg-black/20 uppercase text-[9px] font-bold tracking-wider sticky top-0 backdrop-blur-sm z-10 select-none">
              <th className="py-2 px-3 w-20">Graph</th>
              <th className="py-2 px-3 w-20">Hash</th>
              <th className="py-2 px-3">Message</th>
              <th className="py-2 px-3 w-32">Author</th>
              <th className="py-2 px-3 w-24">Date</th>
            </tr>
          </thead>
          <tbody>
            {graphData.map((node) => (
              <tr
                key={node.hash}
                onClick={() => handleSelectCommit(node)}
                className={cn(
                  "border-b border-[var(--border-default)]/40 cursor-pointer hover:bg-[var(--bg-surface-hover)] transition-colors group",
                  selectedCommit?.hash === node.hash && "bg-[var(--bg-surface-active)] font-semibold border-l-2 border-l-[var(--accent-primary)]"
                )}
              >
                <td className="py-2 px-3 text-purple-400 font-mono whitespace-pre text-[11px] select-none">
                  {node.graph_prefix || "●"}
                </td>
                <td className="py-2 px-3 font-semibold text-[var(--accent-primary)] group-hover:underline">{node.short_hash}</td>
                <td className="py-2 px-3 text-[var(--fg-primary)] truncate max-w-xs">{node.message}</td>
                <td className="py-2 px-3 text-[var(--fg-secondary)] truncate">{node.author_name}</td>
                <td className="py-2 px-3 text-[var(--fg-tertiary)] whitespace-nowrap">
                  {new Date(node.timestamp).toLocaleDateString()}
                </td>
              </tr>
            ))}
            {graphData.length === 0 && (
              <tr>
                <td colSpan={5} className="py-8 text-center text-[var(--fg-tertiary)] italic select-none">
                  No git commit history found.
                </td>
              </tr>
            )}
          </tbody>
        </table>

        {hasMore && (
          <div className="p-4 text-center select-none">
            <button
              onClick={() => loadGraph(offset + 50, false)}
              disabled={loading}
              className="px-4 py-1.5 bg-[var(--bg-panel)] border border-[var(--border-default)] hover:bg-[var(--bg-surface-hover)] text-[var(--fg-primary)] rounded text-xs font-semibold cursor-pointer disabled:opacity-50"
            >
              {loading ? "Loading..." : "Load More Commits"}
            </button>
          </div>
        )}
      </div>
    </div>
  );

  const rightContent = (
    <div className="flex flex-col h-full bg-[var(--bg-panel)] overflow-hidden font-sans">
      {selectedCommit ? (
        <>
          <div className="p-4 border-b border-[var(--border-default)] bg-[var(--bg-sidebar)] space-y-2 shrink-0 select-text">
            <div className="flex items-center justify-between select-none">
              <div className="flex items-center space-x-2 text-[var(--accent-primary)] font-bold text-xs font-mono">
                <IconGitCommit className="w-4 h-4 text-purple-400" />
                <span>{selectedCommit.hash}</span>
                <button
                  onClick={() => handleCopyHash(selectedCommit.hash)}
                  className="p-1 hover:bg-white/10 rounded text-[var(--fg-secondary)] hover:text-white transition-all cursor-pointer"
                  title="Copy Commit Hash"
                >
                  {copiedHash ? <IconCheck className="w-3.5 h-3.5 text-green-400" /> : <IconCopy className="w-3.5 h-3.5" />}
                </button>
              </div>
              <span className="text-[10px] text-[var(--fg-tertiary)] font-mono">
                {new Date(selectedCommit.timestamp).toLocaleString()}
              </span>
            </div>

            <h3 className="text-sm font-semibold text-[var(--fg-primary)] leading-snug">{selectedCommit.message}</h3>

            <div className="flex items-center space-x-4 text-[10px] text-[var(--fg-secondary)] pt-1 border-t border-[var(--border-default)]/50 select-none">
              <div className="flex items-center space-x-1.5">
                <IconUser className="w-3.5 h-3.5 text-purple-400" />
                <span>{selectedCommit.author_name} ({selectedCommit.author_email})</span>
              </div>
              {selectedCommit.parents && selectedCommit.parents.length > 0 && (
                <div className="flex items-center space-x-1 font-mono text-[10px] text-[var(--fg-tertiary)]">
                  <IconCornerDownRight className="w-3 h-3 text-purple-400" />
                  <span>Parents: {selectedCommit.parents.map((p) => p.slice(0, 7)).join(", ")}</span>
                </div>
              )}
            </div>
          </div>

          <div className="flex-1 overflow-y-auto p-3 font-mono text-[11px] space-y-0.5 select-text">
            {loadingDiff ? (
              <div className="text-[var(--fg-tertiary)] animate-pulse p-4 text-center">Loading patch diff...</div>
            ) : diffLines.length === 0 ? (
              <div className="text-[var(--fg-tertiary)] italic p-4 text-center">No changes in this commit.</div>
            ) : (
              diffLines.map((line) => {
                if (line.type === "file") {
                  return (
                    <div
                      key={line.id}
                      className="mt-3 mb-1 px-3 py-1.5 bg-blue-950/40 border border-blue-900/60 text-blue-300 rounded font-bold flex items-center space-x-2 text-[10px]"
                    >
                      <IconFileDiff className="w-4 h-4 shrink-0 text-blue-400" />
                      <span className="truncate">{line.text}</span>
                    </div>
                  );
                }
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
                      className="px-2.5 py-0.5 bg-emerald-950/30 text-emerald-300 border-l border-emerald-500 whitespace-pre"
                    >
                      {line.text}
                    </div>
                  );
                }
                if (line.type === "del") {
                  return (
                    <div
                      key={line.id}
                      className="px-2.5 py-0.5 bg-rose-950/30 text-rose-300 border-l border-rose-500 whitespace-pre"
                    >
                      {line.text}
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
                  <div key={line.id} className="px-2.5 py-0.5 text-[var(--fg-secondary)] whitespace-pre">
                    {line.text}
                  </div>
                );
              })
            )}
          </div>
        </>
      ) : (
        <div className="flex flex-col items-center justify-center h-full text-center space-y-2 text-[var(--fg-tertiary)] select-none">
          <IconFileDiff className="w-12 h-12 text-[var(--fg-disabled)] mb-1" />
          <span className="text-xs font-mono">Select a commit from the graph history to inspect interactive diff changes</span>
        </div>
      )}
    </div>
  );

  return (
    <ResizableSplit
      left={leftContent}
      right={rightContent}
      initialLeftWidth={550}
      minLeftWidth={300}
      maxLeftWidth={900}
    />
  );
}
