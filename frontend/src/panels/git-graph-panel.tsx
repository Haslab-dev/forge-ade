import React, { useState, useEffect } from "react";
import {
  IconGitCommit,
  IconGitBranch,
  IconGitMerge,
  IconUser,
  IconRefresh,
  IconFileDiff,
  IconCopy,
  IconCheck,
  IconCornerDownRight,
  IconX,
  IconUpload,
  IconCloudUpload,
  IconPackage,
} from "@tabler/icons-react";
import { cn } from "../lib/utils";
import { GetGitCommitGraph, GetGitCommitDiff, GetGitCommitBody, GetGitCommitFileDiff, GetGitFileContentAtCommit, GetGitStatus, GitFetch, GitMerge, GetGitBranches } from "../lib/wails";
import { globalOpenFile, globalOpenDiff } from "./editor";
import { useToast } from "../lib/toast";
import { useWorkspace } from "../stores/workspaceStore";
import { ResizableSplit } from "../components/resizable-split";
import { DiffView } from "../components/diff-view";

interface CommitNode {
  hash: string;
  short_hash: string;
  parents: string[];
  author_name: string;
  author_email: string;
  timestamp: string;
  message: string;
  graph_prefix: string;
  decorations: string;
  status?: string;
}

interface CommitGraphResult {
  commits: CommitNode[];
  total_count: number;
  has_more: boolean;
  offset: number;
  limit: number;
}

// Colorful "train lines" palette — one color per branch column in the graph.
const BRANCH_PALETTE = Array.from({ length: 10 }, (_, i) => `var(--graph-c${i})`);

function branchColorForColumn(col: number): string {
  return BRANCH_PALETTE[col % BRANCH_PALETTE.length];
}

// Lane-based SVG renderer — mimics the VS Code "Git Graph" extension:
// round commit dots with colored halos, thin branch-colored lines, and small
// junction dots where branches converge (merge indicators).
const LANE_W = 16;
const ROW_H = 20;

function GraphPrefix({ prefix, isMerge, isHead }: { prefix: string; isMerge?: boolean; isHead?: boolean }) {
  if (!prefix) {
    return <svg width={LANE_W} height={ROW_H} className="block select-none">{commitDot(0, ROW_H / 2, branchColorForColumn(0), isMerge, isHead)}</svg>;
  }
  const lanes = Math.ceil(prefix.length / 2);
  const cx = (lane: number) => lane * LANE_W + LANE_W / 2;
  const color = (lane: number) => branchColorForColumn(lane);

  const lines: React.ReactNode[] = [];
  const dots: { x: number; y: number; color: string; r: number; merge: boolean; head: boolean }[] = [];

  for (let L = 0; L < lanes; L++) {
    const i0 = L * 2;
    const i1 = L * 2 + 1;
    const c0 = i0 < prefix.length ? prefix[i0] : " ";
    const c1 = i1 < prefix.length ? prefix[i1] : " ";
    const col = color(L);

    if (c0 === "|" || c1 === "|") {
      lines.push(<line key={`v${L}`} x1={cx(L)} y1={0} x2={cx(L)} y2={ROW_H} stroke={col} strokeWidth={1.5} />);
    }
    if (c0 === "\\" || c1 === "\\") {
      // Branch leaving this lane down-right into the next lane.
      lines.push(<line key={`br${L}`} x1={cx(L)} y1={0} x2={cx(L + 1)} y2={ROW_H} stroke={col} strokeWidth={1.5} />);
    }
    if (c0 === "/" || c1 === "/") {
      // Incoming line from the right lane converging down-left (keeps its own lane color).
      lines.push(<line key={`bl${L}`} x1={cx(L + 1)} y1={0} x2={cx(L)} y2={ROW_H} stroke={color(L + 1)} strokeWidth={1.5} />);
    }
    if (c0 === "_" || c1 === "_") {
      // Horizontal jog before a diagonal.
      lines.push(<line key={`h${L}`} x1={cx(L)} y1={1.5} x2={cx(L) + LANE_W} y2={1.5} stroke={col} strokeWidth={1.5} />);
    }

    const isCommitCell = c0 === "*" || c1 === "*" || c0 === "@" || c1 === "@";
    if (isCommitCell) {
      dots.push({ x: cx(L), y: ROW_H / 2, color: col, r: 3.4, merge: !!isMerge && L === 0, head: !!isHead });
    }

    // Junction dot where a diagonal meets a vertical — the merge indicator.
    const hasVert = c0 === "|" || c1 === "|";
    const hasDiag = c0 === "/" || c1 === "/" || c0 === "\\" || c1 === "\\";
    if (hasVert && hasDiag) {
      const y = (c0 === "/" || c1 === "/") ? ROW_H : 0;
      dots.push({ x: cx(L), y, color: col, r: 2.2, merge: false, head: false });
    }
  }

  return (
    <svg width={lanes * LANE_W} height={ROW_H} className="block select-none">
      {lines}
      {dots.map((d, i) => commitDot(d.x, d.y, d.color, d.merge, d.head, i))}
    </svg>
  );
}

function commitDot(x: number, y: number, col: string, isMerge = false, isHead = false, key?: number) {
  return (
    <g key={key}>
      {/* halo ring */}
      <circle cx={x} cy={y} r={isMerge ? 5.5 : 5} fill={col} fillOpacity={0.22} />
      {/* main dot */}
      <circle cx={x} cy={y} r={isMerge ? 3.6 : 3.2} fill={col} stroke="rgba(0,0,0,0.55)" strokeWidth={0.75} />
      {/* inner highlight */}
      <circle cx={x} cy={y} r={1.2} fill={isHead ? "#fff" : "rgba(255,255,255,0.75)"} />
    </g>
  );
}

// Parses git decorations like " (HEAD -> main, origin/main)" into chips.
function parseDecorations(dec: string): string[] {
  if (!dec) return [];
  return dec
    .replace(/^\(|\)$/g, "")
    .split(",")
    .map((s) => s.trim())
    .filter(Boolean);
}

// Sign shown per commit: local (unpushed), pushed, or stash.
function CommitStatusSign({ status }: { status?: string }) {
  if (status === "pushed") {
    return (
      <span
        title="Pushed"
        className="text-[var(--status-success)] flex items-center shrink-0"
      >
        <IconCloudUpload className="size-3.5" />
      </span>
    );
  }
  if (status === "stash") {
    return (
      <span
        title="Stash"
        className="text-[var(--graph-c6)] flex items-center shrink-0"
      >
        <IconPackage className="size-3.5" />
      </span>
    );
  }
  if (status === "local") {
    return (
      <span
        title="Local (not pushed)"
        className="text-[var(--status-warning)] flex items-center shrink-0"
      >
        <IconUpload className="size-3.5" />
      </span>
    );
  }
  return null;
}

export function GitGraphPanel() {
  const { toast } = useToast();
  const { activeWorkspacePath } = useWorkspace();
  const ws = activeWorkspacePath || "";

  const [graphData, setGraphData] = useState<CommitNode[]>([]);
  const [branches, setBranches] = useState<string[]>([]);
  const [selectedBranch, setSelectedBranch] = useState<string>("");
  const [totalCount, setTotalCount] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [offset, setOffset] = useState(0);
  const [loading, setLoading] = useState(false);

  const [selectedCommit, setSelectedCommit] = useState<CommitNode | null>(null);
  const [commitDiff, setCommitDiff] = useState<string | null>(null);
  const [commitBody, setCommitBody] = useState<string | null>(null);
  const [loadingDiff, setLoadingDiff] = useState(false);
  const [copiedHash, setCopiedHash] = useState(false);
  const [merging, setMerging] = useState(false);
  const [mergeConfirmOpen, setMergeConfirmOpen] = useState(false);
  const [currentBranch, setCurrentBranch] = useState("main");

  useEffect(() => {
    GetGitBranches(ws).then(setBranches).catch(() => {});
    loadGraph(0, true);
    refreshBranch();
  }, [ws]);

  async function refreshBranch() {
    try {
      const st = await GetGitStatus(ws);
      if (st && st.branch) setCurrentBranch(st.branch);
    } catch { /* ignore */ }
  }

  async function loadGraph(newOffset: number, reset = false, branchOverride?: string) {
    setLoading(true);
    try {
      const res: CommitGraphResult = await GetGitCommitGraph(ws, newOffset, 50, branchOverride ?? selectedBranch);
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

  async function handleRefresh() {
    setLoading(true);
    try {
      await GitFetch(ws);
    } catch (err) {
      console.error("git fetch failed (may have no remote):", err);
    }
    try {
      await loadGraph(0, true);
      await refreshBranch();
      GetGitBranches(ws).then(setBranches).catch(() => {});
    } finally {
      setLoading(false);
    }
  }

  async function handleSelectCommit(node: CommitNode) {
    setSelectedCommit(node);
    setLoadingDiff(true);
    setCommitBody(null);
    try {
      const diffStr = await GetGitCommitDiff(ws, node.hash);
      setCommitDiff(diffStr);
      const body = await GetGitCommitBody(ws, node.hash);
      setCommitBody(body || null);
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

  const handleCloseCommitDetail = () => {
    setSelectedCommit(null);
    setCommitDiff(null);
    setCommitBody(null);
    setLoadingDiff(false);
    setCopiedHash(false);
  };

  async function handleOpenCommitDiff(path: string) {
    if (!selectedCommit || !path) return;
    try {
      const diff = await GetGitCommitFileDiff(ws, selectedCommit.hash, path);
      globalOpenDiff(path, diff || "", { diffHash: selectedCommit.hash });
      toast(`Opened diff for ${path.split("/").pop()}`, "success");
    } catch (err: any) {
      toast("Failed to open commit diff: " + err, "danger");
    }
  }

  async function handleOpenCommitFile(path: string) {
    if (!selectedCommit || !path) return;
    try {
      const content = await GetGitFileContentAtCommit(ws, selectedCommit.hash, path);
      const base = path.split(/[/\\]/).pop() || "file";
      await globalOpenFile(path, {
        id: `commit:${selectedCommit.hash}:${path}`,
        name: `${base} @ ${selectedCommit.hash.slice(0, 7)}`,
        content,
      });
    } catch (err: any) {
      toast("Failed to open file at commit: " + err, "danger");
    }
  }

  function handleMergeCommit() {
    if (!selectedCommit) return;
    setMergeConfirmOpen(true);
  }

  async function performMerge() {
    if (!selectedCommit) return;
    setMerging(true);
    try {
      const out = await GitMerge(ws, selectedCommit.hash, false, false);
      toast(out.trim().split("\n")[0] || "Merge completed", "success");
      setMergeConfirmOpen(false);
      loadGraph(0, true);
      refreshBranch();
    } catch (err: any) {
      toast("Merge failed: " + err, "danger");
    } finally {
      setMerging(false);
    }
  }

  const leftContent = (
    <div className="flex flex-col h-full bg-[var(--bg-app)] overflow-hidden font-sans">
      <div className="flex items-center justify-between px-4 py-2 border-b border-[var(--border-default)] bg-[var(--bg-panel)] shrink-0 select-none">
        <div className="flex items-center space-x-2 font-bold text-xs uppercase tracking-wider text-[var(--fg-tertiary)]">
          <IconGitBranch className="w-4 h-4 text-[var(--graph-c6)]" />
          <span>Commit History Graph</span>
          <span className="text-[10px] text-[var(--fg-secondary)] font-mono">({totalCount} commits)</span>
        </div>
        <div className="flex items-center space-x-2">
          <select
            value={selectedBranch}
            onChange={(e) => {
              const b = e.target.value;
              setSelectedBranch(b);
              setSelectedCommit(null);
              setCommitDiff(null);
              setCommitBody(null);
              loadGraph(0, true, b);
            }}
            className="px-2 py-1 bg-[var(--bg-panel)] border border-[var(--border-default)] text-[var(--fg-secondary)] text-[11px] font-mono rounded cursor-pointer focus:outline-none focus:border-[var(--accent-primary)]"
            title="Show commits for a branch (All = every branch)"
          >
            <option value="">All branches</option>
            {branches.map((b) => (
              <option key={b} value={b}>{b}{b === currentBranch ? " (HEAD)" : ""}</option>
            ))}
          </select>
          <button
            onClick={handleRefresh}
            disabled={loading}
            className="p-1.5 rounded hover:bg-[var(--bg-surface-hover)] text-[var(--fg-secondary)] hover:text-white transition-all cursor-pointer disabled:opacity-50"
            title="Fetch from remote & refresh graph"
          >
            <IconRefresh className={cn("w-4 h-4", loading && "animate-spin")} />
          </button>
        </div>
      </div>

      <div className="flex-1 overflow-y-auto font-mono text-[11px] select-text">
        <table className="w-full text-left border-collapse">
          <thead>
            <tr className="border-b border-[var(--border-default)] text-[var(--fg-secondary)] bg-black/20 uppercase text-[9px] font-bold tracking-wider sticky top-0 backdrop-blur-sm z-10 select-none">
              <th className="py-2 px-3 w-24">Graph</th>
              <th className="py-2 px-3 w-44">Hash</th>
              <th className="py-2 px-3 w-60">Message</th>
              <th className="py-2 px-3 w-32">Author</th>
              <th className="py-2 px-3 w-24">Date</th>
            </tr>
          </thead>
          <tbody>
            {graphData.map((node) => {
              const decorations = parseDecorations(node.decorations);
              const isHead = decorations.some((d) => d.startsWith("HEAD"));
              return (
                <tr
                  key={node.hash}
                  onClick={() => handleSelectCommit(node)}
                  className={cn(
                    "border-b border-[var(--border-default)]/40 cursor-pointer hover:bg-[var(--bg-surface-hover)] transition-colors group",
                    selectedCommit?.hash === node.hash && "bg-[var(--bg-surface-active)] font-semibold border-l-2 border-l-[var(--accent-primary)]"
                  )}
                >
                  <td className="py-1.5 px-3 select-none">
                    <GraphPrefix
                      prefix={node.graph_prefix}
                      isMerge={node.parents && node.parents.length > 1}
                      isHead={isHead}
                    />
                  </td>
                  <td className="py-2 px-3">
                    <div className="flex items-center gap-1 min-w-0">
                      <CommitStatusSign status={node.status} />
                      <span className="font-semibold text-[var(--accent-primary)] group-hover:underline shrink-0">{node.short_hash}</span>
                      {decorations.map((d) => (
                        <span
                          key={d}
                          title={d}
                          className={cn(
                            "inline-block max-w-28 truncate px-1.5 py-0.5 rounded text-[9px] font-bold font-mono shrink-0 border",
                            isHead
                              ? "bg-purple-950/60 border-purple-700/60 text-purple-300"
                              : "bg-sky-950/50 border-sky-800/60 text-sky-300"
                          )}
                        >
                          {d}
                        </span>
                      ))}
                    </div>
                  </td>
                  <td className="py-2 px-3 text-[var(--fg-primary)] overflow-hidden">
                    <span className="block truncate">{node.message}</span>
                  </td>
                  <td className="py-2 px-3 text-[var(--fg-secondary)] truncate">{node.author_name}</td>
                  <td className="py-2 px-3 text-[var(--fg-tertiary)] whitespace-nowrap">
                    {new Date(node.timestamp).toLocaleDateString()}
                  </td>
                </tr>
              );
            })}
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

  const bodyDesc = commitBody ? commitBody.split("\n").slice(1).join("\n").trim() : "";

  const rightContent = (
    <div className="flex flex-col h-full bg-[var(--bg-panel)] overflow-hidden font-sans">
      {selectedCommit ? (
        <>
          <div className="p-4 border-b border-[var(--border-default)] bg-[var(--bg-sidebar)] space-y-2 shrink-0 select-text">
            <div className="flex items-center justify-between select-none">
              <div className="flex items-center space-x-2 text-[var(--accent-primary)] font-bold text-xs font-mono">
                <IconGitCommit className="w-4 h-4 text-[var(--graph-c6)]" />
                <span>{selectedCommit.hash}</span>
                <button
                  onClick={() => handleCopyHash(selectedCommit.hash)}
                  className="p-1 hover:bg-white/10 rounded text-[var(--fg-secondary)] hover:text-white transition-all cursor-pointer"
                  title="Copy Commit Hash"
                >
                  {copiedHash ? <IconCheck className="w-3.5 h-3.5 text-[var(--status-success)]" /> : <IconCopy className="w-3.5 h-3.5" />}
                </button>
                <button
                  onClick={handleCloseCommitDetail}
                  className="px-2 py-0.5 text-[10px] font-semibold border border-[var(--border-default)] rounded text-[var(--fg-secondary)] hover:text-white hover:bg-[var(--bg-surface-hover)] transition-all cursor-pointer"
                  title="Close commit detail"
                >
                  Close
                </button>
              </div>
            </div>

            <h3 className="text-sm font-semibold text-[var(--fg-primary)] leading-snug">{selectedCommit.message}</h3>

            {bodyDesc && (
              <div className="text-[11px] text-[var(--fg-secondary)] leading-relaxed whitespace-pre-wrap max-h-32 overflow-y-auto bg-[var(--bg-panel)]/50 border border-[var(--border-default)]/50 rounded p-2">
                {bodyDesc}
              </div>
            )}

            {parseDecorations(selectedCommit.decorations).length > 0 && (
              <div className="flex items-center gap-1.5 flex-wrap pt-0.5 select-none">
                <IconGitBranch className="w-3 h-3 text-[var(--graph-c6)] shrink-0" />
                {parseDecorations(selectedCommit.decorations).map((d) => (
                  <span
                    key={d}
                    title={d}
                    className={cn(
                      "px-1.5 py-0.5 rounded text-[9px] font-bold font-mono border",
                      d.startsWith("HEAD")
                        ? "bg-purple-950/60 border-purple-700/60 text-purple-300"
                        : "bg-sky-950/50 border-sky-800/60 text-sky-300"
                    )}
                  >
                    {d}
                  </span>
                ))}
              </div>
            )}

            <div className="flex items-center justify-between gap-2 pt-1 select-none">
              <div className="flex items-center space-x-4 text-[10px] text-[var(--fg-secondary)]">
                <div className="flex items-start space-x-1.5">
                  <IconUser className="w-3.5 h-3.5 text-[var(--graph-c6)] mt-px" />
                  <div className="flex flex-col space-y-0.5">
                    <span>{selectedCommit.author_name}</span>
                    <span className="text-[var(--fg-tertiary)] font-mono">
                      {new Date(selectedCommit.timestamp).toLocaleString()}
                    </span>
                  </div>
                </div>
                {selectedCommit.parents && selectedCommit.parents.length > 0 && (
                  <div className="flex items-center space-x-1 font-mono text-[10px] text-[var(--fg-tertiary)]">
                    <IconCornerDownRight className="w-3 h-3 text-[var(--graph-c6)]" />
                    <span>Parents: {selectedCommit.parents.map((p) => p.slice(0, 7)).join(", ")}</span>
                  </div>
                )}
              </div>
              <button
                onClick={handleMergeCommit}
                disabled={merging}
                className="px-2.5 py-1 bg-purple-600 hover:bg-purple-700 text-white text-[10px] font-semibold rounded flex items-center gap-1 cursor-pointer disabled:opacity-50"
                title="Merge this commit into the current branch"
              >
                <IconGitMerge className="w-3.5 h-3.5" />
                <span>{merging ? "Merging..." : "Merge to current branch"}</span>
              </button>
            </div>
          </div>

          <div className="flex-1 overflow-y-auto p-3">
            {loadingDiff ? (
              <div className="text-[var(--fg-tertiary)] animate-pulse p-4 text-center">Loading patch diff...</div>
            ) : (
              <DiffView
                content={commitDiff || ""}
                emptyText="No changes in this commit."
                onOpenDiff={handleOpenCommitDiff}
                onOpenFile={handleOpenCommitFile}
              />
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
    <div className="flex flex-col h-full overflow-hidden">
      <div className="flex-1 min-h-0">
        {selectedCommit ? (
          <ResizableSplit
            left={leftContent}
            right={rightContent}
            initialLeftWidth={550}
            minLeftWidth={300}
            maxLeftWidth={900}
          />
        ) : (
          leftContent
        )}
      </div>

      {/* Merge confirmation modal */}
      {mergeConfirmOpen && selectedCommit && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className="bg-[var(--bg-sidebar)] border border-[var(--border-default)] w-full max-w-md shadow-2xl p-4 flex flex-col gap-3">
            <div className="flex items-center justify-between pb-2 border-b border-[var(--border-default)]">
              <span className="font-bold text-sm text-[var(--fg-primary)] flex items-center gap-2">
                <IconGitMerge className="size-4 text-[var(--graph-c6)]" />
                Merge to current branch
              </span>
              <button
                onClick={() => setMergeConfirmOpen(false)}
                className="text-[var(--fg-tertiary)] hover:text-[var(--fg-primary)] cursor-pointer"
                title="Close"
              >
                <IconX className="size-4" />
              </button>
            </div>

            <div className="space-y-2 text-xs">
              <div className="flex items-center justify-between gap-2">
                <span className="text-[var(--fg-tertiary)]">Merge commit:</span>
                <span className="font-mono text-[var(--accent-primary)] font-semibold">{selectedCommit.short_hash}</span>
              </div>
              <div className="bg-[var(--bg-panel)] border border-[var(--border-default)] p-2 text-[var(--fg-secondary)] font-mono text-[10px] break-words max-h-24 overflow-y-auto">
                {selectedCommit.message}
              </div>
              <div className="flex items-center justify-between gap-2">
                <span className="text-[var(--fg-tertiary)]">Into current branch:</span>
                <span className="font-mono font-semibold text-purple-300">{currentBranch}</span>
              </div>
            </div>

            <div className="flex items-center justify-between mt-2 pt-2 border-t border-[var(--border-default)]">
              <div />
              <div className="flex items-center gap-2">
                <button
                  onClick={() => setMergeConfirmOpen(false)}
                  disabled={merging}
                  className="px-3 py-1.5 text-xs text-[var(--fg-secondary)] hover:text-[var(--fg-primary)] cursor-pointer disabled:opacity-50"
                >
                  Cancel
                </button>
                <button
                  onClick={performMerge}
                  disabled={merging}
                  className="px-4 py-1.5 text-xs font-semibold bg-purple-600 hover:bg-purple-700 text-white rounded flex items-center gap-1 cursor-pointer disabled:opacity-50"
                >
                  <IconGitMerge className="size-3.5" />
                  <span>{merging ? "Merging..." : "Merge"}</span>
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
