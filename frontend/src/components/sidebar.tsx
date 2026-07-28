import { useState, useEffect, useCallback, useRef } from "react";
import {
  FolderTree,
  Search,
  GitBranch,
  Terminal,
  PanelLeftClose,
  PanelLeft,
  File,
  Shell,
  GitCommitHorizontal,
  Upload,
} from "lucide-react";
import { cn } from "../lib/utils";
import { Explorer } from "../panels/explorer";
import { ScrollArea } from "./ui/scroll-area";
import { SimpleModal } from "./simple-modal";
import { globalOpenFile } from "../panels/editor";
import {
  SearchContent,
  SearchFilename,
  DiscoverRepos,
  GetRepoStatus,
  ListSessions,
  CreateShell,
  StopSession,
  RenameSession,
  GitStage,
  GitUnstage,
  GitStageAll,
  GitCommit,
  GitRunCommand,
  GetCommitGraph,
  GetCommitDetail,
} from "../../wailsjs/go/main/App";
import { terminal, search, git } from "../../wailsjs/go/models";
import type { GitStatusEntry } from "../types";

interface SidebarProps {
  folders: string[];
  onOpenSession?: (id: string) => void;
  sessions?: terminal.Session[];
  onRefreshSessions?: () => void;
  cwd?: string;
}

const sidebarTabs = [
  { id: "explorer", icon: FolderTree, label: "Explorer" },
  { id: "search", icon: Search, label: "Search" },
  { id: "git", icon: GitBranch, label: "Git" },
  { id: "runtime", icon: Terminal, label: "Runtime" },
];

export function Sidebar({ folders, onOpenSession, sessions: propSessions, onRefreshSessions, cwd }: SidebarProps) {
  const [activeTab, setActiveTab] = useState("explorer");
  const [collapsed, setCollapsed] = useState(false);

  return (
    <div className="flex h-full">
      <div className="flex flex-col items-center gap-1 py-2 px-1 border-r bg-muted/30 w-12">
        {sidebarTabs.map((tab) => (
          <button
            key={tab.id}
            className={cn(
              "p-2 rounded-md transition-colors cursor-pointer",
              activeTab === tab.id
                ? "bg-accent text-accent-foreground"
                : "text-muted-foreground hover:text-foreground hover:bg-accent/50"
            )}
            onClick={() => setActiveTab(tab.id)}
            title={tab.label}
          >
            <tab.icon className="size-4" />
          </button>
        ))}
        <div className="flex-1" />
        <button
          className="p-2 rounded-md text-muted-foreground hover:text-foreground hover:bg-accent/50 cursor-pointer"
          onClick={() => setCollapsed(!collapsed)}
          title={collapsed ? "Expand" : "Collapse"}
        >
          {collapsed ? <PanelLeft className="size-4" /> : <PanelLeftClose className="size-4" />}
        </button>
      </div>

      {!collapsed && (
        <div className="w-64 border-r bg-background flex flex-col">
          <div className="flex items-center justify-between px-3 py-2 text-xs font-medium text-muted-foreground uppercase tracking-wider border-b shrink-0">
            <span>{sidebarTabs.find((t) => t.id === activeTab)?.label}</span>
          </div>
          <div className="flex-1 overflow-hidden">
            {activeTab === "explorer" && <Explorer roots={folders} />}
            {activeTab === "search" && <SearchPanel />}
            {activeTab === "git" && <GitPanel />}
            {activeTab === "runtime" && (
              <RuntimePanel
                onOpenSession={onOpenSession}
                sessions={propSessions}
                onRefresh={onRefreshSessions}
                cwd={cwd}
              />
            )}
          </div>
        </div>
      )}
    </div>
  );
}

function SearchPanel() {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<search.RankedResult[]>([]);
  const [mode, setMode] = useState<"filename" | "content">("filename");

  const handleSearch = useCallback(async () => {
    if (!query.trim()) return;
    try {
      let res: search.RankedResult[];
      if (mode === "filename") res = await SearchFilename(query, 50);
      else res = await SearchContent(query, 50);
      setResults(Array.isArray(res) ? res : []);
    } catch (err) { console.error(err); setResults([]); }
  }, [query, mode]);

  return (
    <div className="flex flex-col h-full">
      <div className="p-2 border-b space-y-2">
        <div className="flex gap-1">
          <button className={cn("flex-1 text-xs px-2 py-1 rounded", mode === "filename" ? "bg-accent" : "hover:bg-accent/50")} onClick={() => setMode("filename")}>Files</button>
          <button className={cn("flex-1 text-xs px-2 py-1 rounded", mode === "content" ? "bg-accent" : "hover:bg-accent/50")} onClick={() => setMode("content")}>Content</button>
        </div>
        <div className="flex gap-1">
          <input className="flex-1 text-xs bg-muted rounded px-2 py-1 outline-none focus:ring-1 focus:ring-ring" placeholder={mode === "filename" ? "Search files..." : "Search content..."} value={query} onChange={(e) => setQuery(e.target.value)} onKeyDown={(e) => e.key === "Enter" && handleSearch()} />
          <button className="text-xs px-2 py-1 bg-primary text-primary-foreground rounded hover:bg-primary/90" onClick={handleSearch}>Go</button>
        </div>
      </div>
      <ScrollArea className="flex-1">
        {results.length === 0 && <p className="p-3 text-xs text-muted-foreground">{query ? "No results" : "Type a query to search"}</p>}
        {results.map((r, i) => (
          <div key={i} className="px-3 py-1.5 text-xs hover:bg-accent cursor-pointer border-b border-border/50" onClick={() => globalOpenFile(r.path)}>
            <div className="flex items-center gap-1"><File className="size-3 shrink-0 text-muted-foreground" /><span className="truncate font-medium">{r.filename}</span></div>
            <div className="truncate text-muted-foreground mt-0.5">{r.path}</div>
            {r.content && <div className="truncate text-muted-foreground/70 mt-0.5 font-mono">{r.content}</div>}
          </div>
        ))}
      </ScrollArea>
    </div>
  );
}

function GitPanel() {
  const [repos, setRepos] = useState<{ path: string; status: GitStatusEntry[] }[]>([]);
  const [loading, setLoading] = useState(false);
  const [viewMode, setViewMode] = useState<"changes" | "graph">("changes");
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [commitMsg, setCommitMsg] = useState("");
  const [showPushConfirm, setShowPushConfirm] = useState(false);
  const [pushTarget, setPushTarget] = useState("");
  const [feedback, setFeedback] = useState("");
  const [graphEntries, setGraphEntries] = useState<Map<string, git.CommitGraphEntry[]>>(new Map());
  const [graphLoading, setGraphLoading] = useState(false);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      await DiscoverRepos();
      const statusMap: Record<string, GitStatusEntry[]> = await GetRepoStatus();
      const entries = Object.entries(statusMap).map(([path, status]) => ({ path, status: Array.isArray(status) ? status : [] }));
      setRepos(entries);
    } catch (err) { console.error(err); }
    setLoading(false);
  }, []);

  useEffect(() => { refresh(); }, [refresh]);

  useEffect(() => {
    const timer = setInterval(refresh, 3000);
    return () => clearInterval(timer);
  }, [refresh]);

  // Load graph data
  useEffect(() => {
    if (viewMode !== "graph") return;
    setGraphLoading(true);
    (async () => {
      const map = new Map<string, git.CommitGraphEntry[]>();
      for (const repo of repos) {
        try {
          const entries = await GetCommitGraph(repo.path, 50);
          map.set(repo.path, Array.isArray(entries) ? entries : []);
        } catch { }
      }
      setGraphEntries(map);
      setGraphLoading(false);
    })();
  }, [viewMode, repos.map(r => r.path).join(",")]);

  const toggleSelect = (path: string) => {
    setSelected(prev => { const next = new Set(prev); if (next.has(path)) next.delete(path); else next.add(path); return next; });
  };

  const currentRepo = repos[0];
  const repoPath = currentRepo?.path || "";

  const staged = currentRepo?.status.filter(s => s.staging && s.staging !== " ") || [];
  const unstaged = currentRepo?.status.filter(s => (!s.staging || s.staging === " ") && s.worktree && s.worktree !== " ") || [];
  const untracked = currentRepo?.status.filter(s => s.worktree === "?" || s.staging === "?") || [];

  const handleStageAll = async () => {
    if (!repoPath) return;
    try { await GitStageAll(repoPath); setFeedback("All changes staged ✓"); refresh(); } catch (e) { setFeedback(String(e)); }
  };

  const handleStageSelected = async () => {
    if (!repoPath || selected.size === 0) return;
    try { await GitStage(repoPath, [...selected]); setSelected(new Set()); setFeedback("Selected staged ✓"); refresh(); } catch (e) { setFeedback(String(e)); }
  };

  const handleUnstageSelected = async () => {
    if (!repoPath || selected.size === 0) return;
    try { await GitUnstage(repoPath, [...selected]); setSelected(new Set()); setFeedback("Selected unstaged ✓"); refresh(); } catch (e) { setFeedback(String(e)); }
  };

  const handleCommit = async () => {
    if (!repoPath || !commitMsg.trim()) return;
    try { await GitCommit(repoPath, commitMsg.trim()); setCommitMsg(""); setFeedback("Committed ✓"); refresh(); } catch (e) { setFeedback(String(e)); }
  };

  const handlePush = async () => {
    if (!repoPath) return;
    try {
      const target = pushTarget.trim() || "origin HEAD";
      await GitRunCommand(repoPath, "push " + target);
      setFeedback("Pushed ✓");
    } catch (e) { setFeedback(String(e)); }
    setShowPushConfirm(false);
  };

  useEffect(() => {
    if (!feedback) return;
    const t = setTimeout(() => setFeedback(""), 3000);
    return () => clearTimeout(t);
  }, [feedback]);

  return (
    <div className="flex flex-col h-full">
      <div className="p-2 border-b flex items-center justify-between gap-1">
        <div className="flex gap-1">
          <button className={cn("text-xs px-2 py-0.5 rounded", viewMode === "changes" ? "bg-accent" : "hover:bg-accent")} onClick={() => setViewMode("changes")}>Changes</button>
          <button className={cn("text-xs px-2 py-0.5 rounded", viewMode === "graph" ? "bg-accent" : "hover:bg-accent")} onClick={() => setViewMode("graph")}>Graph</button>
        </div>
        <button className="text-xs px-2 py-0.5 hover:bg-accent rounded" onClick={refresh}>{loading ? "..." : "Refresh"}</button>
      </div>

      {feedback && <div className="px-3 py-1 text-[10px] text-green-400 bg-green-500/10 border-b">{feedback}</div>}

      <ScrollArea className="flex-1">
        {!repoPath && <p className="p-3 text-xs text-muted-foreground">No repositories found</p>}

        {viewMode === "graph" && <GitGraphView entries={graphEntries.get(repoPath || "") || []} loading={graphLoading} repoPath={repoPath} />}

        {viewMode === "changes" && repoPath && (
          <div>
            {/* Action bar */}
            <div className="flex items-center gap-1 px-2 py-1 border-b border-border/30">
              <button className="text-[10px] px-1.5 py-0.5 bg-primary/10 text-primary rounded hover:bg-primary/20" onClick={handleStageAll} title="Stage All">+ All</button>
              <button className={cn("text-[10px] px-1.5 py-0.5 rounded", selected.size > 0 ? "bg-green-500/10 text-green-400 hover:bg-green-500/20" : "text-muted-foreground opacity-40")} disabled={selected.size === 0} onClick={handleStageSelected}>+ Sel</button>
              <button className={cn("text-[10px] px-1.5 py-0.5 rounded", selected.size > 0 ? "bg-orange-500/10 text-orange-400 hover:bg-orange-500/20" : "text-muted-foreground opacity-40")} disabled={selected.size === 0} onClick={handleUnstageSelected}>- Sel</button>
            </div>

            {/* Staged section */}
            {staged.length > 0 && (
              <div>
                <div className="px-3 py-1 text-[10px] font-medium text-green-400 uppercase tracking-wider border-b border-border/20">Staged ({staged.length})</div>
                {staged.map((s, i) => (
                  <GitFileRow key={"staged-" + i} entry={s} selected={selected.has(s.path)} onToggle={() => toggleSelect(s.path)} repoPath={repoPath} />
                ))}
              </div>
            )}

            {/* Unstaged section */}
            {unstaged.length > 0 && (
              <div>
                <div className="px-3 py-1 text-[10px] font-medium text-blue-400 uppercase tracking-wider border-b border-border/20">Changes ({unstaged.length})</div>
                {unstaged.map((s, i) => (
                  <GitFileRow key={"unstaged-" + i} entry={s} selected={selected.has(s.path)} onToggle={() => toggleSelect(s.path)} repoPath={repoPath} />
                ))}
              </div>
            )}

            {/* Untracked section */}
            {untracked.length > 0 && (
              <div>
                <div className="px-3 py-1 text-[10px] font-medium text-green-400/60 uppercase tracking-wider border-b border-border/20">Untracked ({untracked.length})</div>
                {untracked.map((s, i) => (
                  <GitFileRow key={"untracked-" + i} entry={s} selected={selected.has(s.path)} onToggle={() => toggleSelect(s.path)} repoPath={repoPath} />
                ))}
              </div>
            )}

            {staged.length === 0 && unstaged.length === 0 && untracked.length === 0 && (
              <p className="px-3 py-3 text-[10px] text-muted-foreground">Clean working tree</p>
            )}

            {/* Commit area */}
            {repoPath && (
              <div className="border-t border-border/30 p-2 space-y-1">
                <textarea
                  className="w-full text-xs bg-muted/50 rounded px-2 py-1 outline-none focus:ring-1 focus:ring-ring resize-none h-14"
                  placeholder="Commit message..."
                  value={commitMsg}
                  onChange={(e) => setCommitMsg(e.target.value)}
                  onKeyDown={(e) => { if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) handleCommit(); }}
                />
                <div className="flex gap-1">
                  <button
                    className={cn("flex-1 text-xs px-2 py-1 rounded flex items-center justify-center gap-1", commitMsg.trim() ? "bg-primary text-primary-foreground hover:bg-primary/90" : "bg-muted text-muted-foreground")}
                    disabled={!commitMsg.trim()}
                    onClick={handleCommit}
                  >
                    <GitCommitHorizontal className="size-3" /> Commit
                  </button>
                  <button
                    className="text-xs px-2 py-1 rounded bg-muted hover:bg-accent flex items-center gap-1"
                    onClick={() => { setPushTarget(""); setShowPushConfirm(true); }}
                    title="Push"
                  >
                    <Upload className="size-3" />
                  </button>
                </div>
              </div>
            )}
          </div>
        )}
      </ScrollArea>

      {showPushConfirm && (
        <SimpleModal
          open={true}
          title="Push"
          placeholder="origin HEAD"
          defaultValue="origin HEAD"
          onClose={() => setShowPushConfirm(false)}
          onSubmit={handlePush}
          submitLabel="Push"
        />
      )}
    </div>
  );
}

// Git file row with checkbox + status + click to open
function GitFileRow({ entry, selected, onToggle, repoPath }: { entry: GitStatusEntry; selected: boolean; onToggle: () => void; repoPath: string }) {
  const code = entry.staging && entry.staging !== " " ? entry.staging : entry.worktree && entry.worktree !== " " ? entry.worktree : "M";
  return (
    <div className="flex items-center gap-1 px-2 py-0.5 text-xs hover:bg-accent group cursor-pointer" onClick={() => globalOpenFile(repoPath + "/" + entry.path)}>
      <input type="checkbox" className="size-3 accent-primary shrink-0" checked={selected} onChange={(e) => { e.stopPropagation(); onToggle(); }} onClick={(e) => e.stopPropagation()} />
      <span className={cn(
        "w-5 text-center shrink-0 font-bold text-[10px]",
        code === "M" && "text-blue-400",
        code === "A" && "text-green-400",
        code === "D" && "text-red-400",
        code === "R" && "text-purple-400",
        code === "?" && "text-green-400/60",
      )}>{code}</span>
      <span className={cn("truncate flex-1", code === "D" && "line-through opacity-60")}>{entry.path}</span>
    </div>
  );
}

// Git graph / history view
function GitGraphView({ entries, loading, repoPath }: { entries: git.CommitGraphEntry[]; loading: boolean; repoPath: string }) {
  const [detailHash, setDetailHash] = useState<string | null>(null);
  const [detailContent, setDetailContent] = useState("");

  if (loading) return <p className="p-3 text-xs text-muted-foreground">Loading graph...</p>;
  if (entries.length === 0) return <p className="p-3 text-xs text-muted-foreground">No commits</p>;

  return (
    <div className="font-mono text-[11px]">
      {entries.map((e, i) => {
        const graphPart = renderGraphLine(e.graphLine);
        return (
          <div key={i}>
            <div
              className={cn(
                "flex items-center px-2 py-0.5 hover:bg-accent cursor-pointer",
                detailHash === e.hash && "bg-accent/50"
              )}
              onClick={async () => {
                if (detailHash === e.hash) { setDetailHash(null); return; }
                setDetailHash(e.hash);
                try {
                  const detail = await GetCommitDetail(repoPath, e.hash);
                  setDetailContent(detail || "");
                } catch { setDetailContent(""); }
              }}
            >
              <span className="shrink-0 mr-1 leading-4 whitespace-pre text-xs" style={{ minWidth: `${Math.max(entries.length > 0 ? entries[0].graphLine.length : 4, 4)}ch` }}>
                {graphPart}
              </span>
              <span className="text-yellow-400 shrink-0 mr-1">{e.hash}</span>
              <span className="truncate text-muted-foreground">{e.subject}</span>
            </div>
            {detailHash === e.hash && detailContent && (
              <div className="px-3 py-1 text-[10px] text-muted-foreground bg-muted/20 border-b border-border/20 whitespace-pre-wrap max-h-32 overflow-auto">
                {detailContent.slice(0, 1000)}
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}

function renderGraphLine(line: string): string {
  return line.replace(/\*/g, "●").replace(/[o]/g, "○").replace(/[|]/g, "│").replace(/[\/]/g, "╱").replace(/[\\]/g, "╲").replace(/[_]/g, "─");
}

function RuntimePanel({
  onOpenSession,
  sessions: propSessions,
  onRefresh,
  cwd,
}: {
  onOpenSession?: (id: string) => void;
  sessions?: terminal.Session[];
  onRefresh?: () => void;
  cwd?: string;
}) {
  const [error, setError] = useState("");
  const [feedback, setFeedback] = useState("");

  const refresh = useCallback(async () => {
    if (onRefresh) { await onRefresh(); }
    else {
      try { const list: terminal.Session[] = await ListSessions(); setSessionsLocally(Array.isArray(list) ? list : []); setError(""); }
      catch (err: unknown) { setError(String(err)); }
    }
  }, [onRefresh]);

  const [localSessions, setSessionsLocally] = useState<terminal.Session[]>([]);
  const sessions = propSessions ?? localSessions;

  useEffect(() => { if (!propSessions) refresh(); }, []);
  useEffect(() => { if (!feedback) return; const t = setTimeout(() => setFeedback(""), 2000); return () => clearTimeout(t); }, [feedback]);

  const handleNewShell = useCallback(async () => {
    setError("");
    try { const s = await CreateShell("Shell", cwd || ""); refresh(); if (onOpenSession && s?.id) onOpenSession(s.id); }
    catch (err: unknown) { setError(String(err)); }
  }, [cwd, refresh, onOpenSession]);

  const handleStop = useCallback(async (id: string) => {
    setError("");
    try { await StopSession(id); setFeedback("Stopped ✓"); await refresh(); }
    catch (err: unknown) { console.error(err); setError(String(err)); }
  }, [refresh]);

  const [renameTarget, setRenameTarget] = useState<terminal.Session | null>(null);
  const handleRename = useCallback((s: terminal.Session) => { setRenameTarget(s); }, []);
  const handleRenameConfirm = useCallback(async (newName: string) => {
    if (!renameTarget) return;
    setError("");
    try { await RenameSession(renameTarget.id, newName); await refresh(); }
    catch (err: unknown) { setError(String(err)); }
    setRenameTarget(null);
  }, [renameTarget, refresh]);
  const shells = sessions.filter((s) => s.type === "shell");

  return (
    <div className="flex flex-col h-full">
      <div className="p-2 border-b flex items-center justify-between gap-1">
        <span className="text-xs text-muted-foreground">{sessions.length} session{sessions.length !== 1 ? "s" : ""}</span>
        <div className="flex gap-1">
          <button className="text-xs px-2 py-0.5 hover:bg-accent rounded flex items-center gap-1 cursor-pointer" onClick={handleNewShell} title="New Shell"><Shell className="size-3" /> Shell</button>
        </div>
      </div>
      {error && <div className="px-3 py-1.5 text-[10px] text-red-400 bg-red-500/10 border-b">{error}</div>}
      {feedback && <div className="px-3 py-1.5 text-[10px] text-green-400 bg-green-500/10 border-b">{feedback}</div>}
      <ScrollArea className="flex-1">
        {sessions.length === 0 && !error && (
          <div className="p-4 text-xs text-muted-foreground text-center"><Terminal className="size-6 mx-auto mb-2 opacity-30" /><p>No sessions running</p></div>
        )}
        {shells.length > 0 && (
          <div className="border-b border-border/50 pb-1">
            <div className="px-3 py-1 text-[10px] font-medium text-muted-foreground uppercase tracking-wider">Shell</div>
            {shells.map((s) => (<SessionRow key={s.id} session={s} onStop={handleStop} onRename={handleRename} onOpen={onOpenSession} />))}
          </div>
        )}
      </ScrollArea>
      {renameTarget && (
        <SimpleModal open={true} title="Rename Session" defaultValue={renameTarget.name} onClose={() => setRenameTarget(null)} onSubmit={(newName) => { handleRenameConfirm(newName); }} submitLabel="Rename" />
      )}
    </div>
  );
}

function SessionRow({ session, onStop, onRename, onOpen }: { session: terminal.Session; onStop: (id: string) => void; onRename: (s: terminal.Session) => void; onOpen?: (id: string) => void; }) {
  const handleRowClick = (e: React.MouseEvent) => {
    const target = e.target as HTMLElement;
    if (target.closest('[data-action="stop"]')) { e.preventDefault(); e.stopPropagation(); onStop(session.id); return; }
    onOpen?.(session.id);
  };
  return (
    <div className="flex items-center gap-2 px-3 py-1.5 text-xs hover:bg-accent group cursor-pointer select-none border-b border-border/20" onClick={handleRowClick} onContextMenu={(e) => { e.preventDefault(); onRename(session); }}>
      {session.type === "shell" ? <Shell className="size-3.5 shrink-0 text-green-500" /> : <Terminal className="size-3.5 shrink-0 text-cyan-500" />}
      <div className="flex-1 min-w-0 pointer-events-none">
        <div className="flex items-center gap-1.5">
          <span className="font-medium truncate">{session.name}</span>
          <span className={cn("w-1.5 h-1.5 rounded-full shrink-0", session.status === "running" ? "bg-green-500" : "bg-muted-foreground")} />
          <span className="text-muted-foreground font-normal">· PID: {session.pid}</span>
        </div>
        <div className="text-muted-foreground truncate">{session.provider}</div>
      </div>
      <span data-action="stop" className="inline-flex items-center justify-center p-1 hover:bg-red-500/20 rounded text-red-400 border border-red-400/30 text-[10px] min-w-[18px] h-[18px]" title="Stop (kill process)">✕</span>
    </div>
  );
}
