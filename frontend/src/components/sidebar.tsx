import { useState, useEffect, useCallback } from "react";
import {
  FolderTree,
  Search,
  GitBranch,
  Terminal,
  PanelLeftClose,
  PanelLeft,
  File,
  Shell,
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
} from "../../wailsjs/go/main/App";
import { terminal, search } from "../../wailsjs/go/models";
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
      {/* Icon bar */}
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
          {collapsed ? (
            <PanelLeft className="size-4" />
          ) : (
            <PanelLeftClose className="size-4" />
          )}
        </button>
      </div>

      {/* Panel content */}
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

// ---------------------------------------------------------------------------
// Search Panel
// ---------------------------------------------------------------------------
function SearchPanel() {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<search.RankedResult[]>([]);
  const [mode, setMode] = useState<"filename" | "content">("filename");

  const handleSearch = useCallback(async () => {
    if (!query.trim()) return;
    try {
      let res: search.RankedResult[];
      if (mode === "filename") {
        res = await SearchFilename(query, 50);
      } else {
        res = await SearchContent(query, 50);
      }
      setResults(Array.isArray(res) ? res : []);
    } catch (err) {
      console.error(err);
      setResults([]);
    }
  }, [query, mode]);

  return (
    <div className="flex flex-col h-full">
      <div className="p-2 border-b space-y-2">
        <div className="flex gap-1">
          <button
            className={cn(
              "flex-1 text-xs px-2 py-1 rounded",
              mode === "filename" ? "bg-accent" : "hover:bg-accent/50"
            )}
            onClick={() => setMode("filename")}
          >
            Files
          </button>
          <button
            className={cn(
              "flex-1 text-xs px-2 py-1 rounded",
              mode === "content" ? "bg-accent" : "hover:bg-accent/50"
            )}
            onClick={() => setMode("content")}
          >
            Content
          </button>
        </div>
        <div className="flex gap-1">
          <input
            className="flex-1 text-xs bg-muted rounded px-2 py-1 outline-none focus:ring-1 focus:ring-ring"
            placeholder={mode === "filename" ? "Search files..." : "Search content..."}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && handleSearch()}
          />
          <button
            className="text-xs px-2 py-1 bg-primary text-primary-foreground rounded hover:bg-primary/90"
            onClick={handleSearch}
          >
            Go
          </button>
        </div>
      </div>
      <ScrollArea className="flex-1">
        {results.length === 0 && (
          <p className="p-3 text-xs text-muted-foreground">
            {query ? "No results" : "Type a query to search"}
          </p>
        )}
        {results.map((r, i) => (
          <div
            key={i}
            className="px-3 py-1.5 text-xs hover:bg-accent cursor-pointer border-b border-border/50"
            onClick={() => globalOpenFile(r.path)}
          >
            <div className="flex items-center gap-1">
              <File className="size-3 shrink-0 text-muted-foreground" />
              <span className="truncate font-medium">{r.filename}</span>
            </div>
            <div className="truncate text-muted-foreground mt-0.5">{r.path}</div>
            {r.content && (
              <div className="truncate text-muted-foreground/70 mt-0.5 font-mono">
                {r.content}
              </div>
            )}
          </div>
        ))}
      </ScrollArea>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Git Panel
// ---------------------------------------------------------------------------
function GitPanel() {
  const [repos, setRepos] = useState<{ path: string; status: GitStatusEntry[] }[]>([]);
  const [loading, setLoading] = useState(false);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      await DiscoverRepos();
      const statusMap: Record<string, GitStatusEntry[]> = await GetRepoStatus();
      const entries = Object.entries(statusMap).map(([path, status]) => ({
        path,
        status: Array.isArray(status) ? status : [],
      }));
      setRepos(entries);
    } catch (err) {
      console.error(err);
    }
    setLoading(false);
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  return (
    <div className="flex flex-col h-full">
      <div className="p-2 border-b flex items-center justify-between">
        <span className="text-xs text-muted-foreground">
          {repos.length} repo{repos.length !== 1 ? "s" : ""}
        </span>
        <button
          className="text-xs px-2 py-0.5 hover:bg-accent rounded"
          onClick={refresh}
        >
          {loading ? "..." : "Refresh"}
        </button>
      </div>
      <ScrollArea className="flex-1">
        {repos.length === 0 && (
          <p className="p-3 text-xs text-muted-foreground">No repositories found</p>
        )}
        {repos.map((repo) => (
          <div key={repo.path} className="border-b border-border/50">
            <div className="flex items-center gap-1 px-3 py-1.5 text-xs font-medium">
              <GitBranch className="size-3.5 text-muted-foreground" />
              <span className="truncate">{repo.path.split("/").pop()}</span>
            </div>
            {repo.status.length === 0 && (
              <p className="px-3 pb-1.5 text-[10px] text-muted-foreground">Clean working tree</p>
            )}
            {repo.status.slice(0, 10).map((s, i) => (
              <div
                key={i}
                className="flex items-center gap-2 px-3 py-0.5 text-xs hover:bg-accent cursor-pointer"
                onClick={() => globalOpenFile(repo.path + "/" + s.path)}
              >
                <span className="text-red-400 w-4 text-center shrink-0">
                  {s.worktree || s.staging || " "}
                </span>
                <span className="truncate">{s.path}</span>
              </div>
            ))}
            {repo.status.length > 10 && (
              <p className="px-3 pb-1 text-[10px] text-muted-foreground">
                +{repo.status.length - 10} more
              </p>
            )}
          </div>
        ))}
      </ScrollArea>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Runtime Panel (Unified — Shell)
// ---------------------------------------------------------------------------
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
    if (onRefresh) {
      await onRefresh();
    } else {
      try {
        const list: terminal.Session[] = await ListSessions();
        setSessionsLocally(Array.isArray(list) ? list : []);
        setError("");
      } catch (err: unknown) {
        setError(String(err));
      }
    }
  }, [onRefresh]);

  const [localSessions, setSessionsLocally] = useState<terminal.Session[]>([]);
  const sessions = propSessions ?? localSessions;

  useEffect(() => {
    if (!propSessions) {
      refresh();
    }
  }, []);

  // Auto-clear feedback after 2s
  useEffect(() => {
    if (!feedback) return;
    const t = setTimeout(() => setFeedback(""), 2000);
    return () => clearTimeout(t);
  }, [feedback]);

  const handleNewShell = useCallback(async () => {
    setError("");
    try {
      const s = await CreateShell("Shell", cwd || "");
      refresh();
      if (onOpenSession && s?.id) onOpenSession(s.id);
    } catch (err: unknown) {
      setError(String(err));
    }
  }, [cwd, refresh, onOpenSession]);

  const handleStop = useCallback(async (id: string) => {
    setError("");
    try {
      console.log("STOP called for session:", id);
      await StopSession(id);
      setFeedback("Stopped ✓");
      await refresh();
    } catch (err: unknown) {
      console.error("STOP error:", err);
      setError(String(err));
    }
  }, [refresh]);

  const [renameTarget, setRenameTarget] = useState<terminal.Session | null>(null);

  const handleRename = useCallback((s: terminal.Session) => {
    setRenameTarget(s);
  }, []);

  const handleRenameConfirm = useCallback(async (newName: string) => {
    if (!renameTarget) return;
    setError("");
    try {
      await RenameSession(renameTarget.id, newName);
      await refresh();
    } catch (err: unknown) {
      setError(String(err));
    }
    setRenameTarget(null);
  }, [renameTarget, refresh]);

  const shells = sessions.filter((s) => s.type === "shell");

  return (
    <div className="flex flex-col h-full">
      <div className="p-2 border-b flex items-center justify-between gap-1">
        <span className="text-xs text-muted-foreground">{sessions.length} session{sessions.length !== 1 ? "s" : ""}</span>
        <div className="flex gap-1">
          <button className="text-xs px-2 py-0.5 hover:bg-accent rounded flex items-center gap-1 cursor-pointer" onClick={handleNewShell} title="New Shell">
            <Shell className="size-3" /> Shell
          </button>
        </div>
      </div>

      {error && (
        <div className="px-3 py-1.5 text-[10px] text-red-400 bg-red-500/10 border-b">{error}</div>
      )}
      {feedback && (
        <div className="px-3 py-1.5 text-[10px] text-green-400 bg-green-500/10 border-b">{feedback}</div>
      )}

      <ScrollArea className="flex-1">
        {sessions.length === 0 && !error && (
          <div className="p-4 text-xs text-muted-foreground text-center">
            <Terminal className="size-6 mx-auto mb-2 opacity-30" />
            <p>No sessions running</p>
          </div>
        )}

        {shells.length > 0 && (
          <div className="border-b border-border/50 pb-1">
            <div className="px-3 py-1 text-[10px] font-medium text-muted-foreground uppercase tracking-wider">Shell</div>
            {shells.map((s) => (
              <SessionRow key={s.id} session={s} onStop={handleStop} onRename={handleRename} onOpen={onOpenSession} />
            ))}
          </div>
        )}
      </ScrollArea>

      {renameTarget && (
        <SimpleModal
          open={true}
          title="Rename Session"
          defaultValue={renameTarget.name}
          onClose={() => setRenameTarget(null)}
          onSubmit={(newName) => {
            handleRenameConfirm(newName);
          }}
          submitLabel="Rename"
        />
      )}
    </div>
  );

}

function SessionRow({
  session,
  onStop,
  onRename,
  onOpen,
}: {
  session: terminal.Session;
  onStop: (id: string) => void;
  onRename: (s: terminal.Session) => void;
  onOpen?: (id: string) => void;
}) {
  const handleRowClick = (e: React.MouseEvent) => {
    // Check if the click target is the stop button or inside it
    const target = e.target as HTMLElement;
    if (target.closest('[data-action="stop"]')) {
      e.preventDefault();
      e.stopPropagation();
      onStop(session.id);
      return;
    }
    // Otherwise, open the session
    onOpen?.(session.id);
  };

  const handleCtxMenu = (e: React.MouseEvent) => {
    e.preventDefault();
    onRename(session);
  };

  return (
    <div
      className="flex items-center gap-2 px-3 py-1.5 text-xs hover:bg-accent group cursor-pointer select-none border-b border-border/20"
      onClick={handleRowClick}
      onContextMenu={handleCtxMenu}
    >
      {session.type === "shell" ? (
        <Shell className="size-3.5 shrink-0 text-green-500" />
      ) : (
        <Terminal className="size-3.5 shrink-0 text-cyan-500" />
      )}
      <div className="flex-1 min-w-0 pointer-events-none">
        <div className="flex items-center gap-1.5">
          <span className="font-medium truncate">{session.name}</span>
          <span className={cn("w-1.5 h-1.5 rounded-full shrink-0", session.status === "running" ? "bg-green-500" : "bg-muted-foreground")} />
          <span className="text-muted-foreground font-normal">
            · PID: {session.pid}
          </span>
        </div>
        <div className="text-muted-foreground truncate">
          {session.provider}
        </div>
      </div>
      <span
        data-action="stop"
        className="inline-flex items-center justify-center p-1 hover:bg-red-500/20 rounded text-red-400 border border-red-400/30 text-[10px] min-w-[18px] h-[18px]"
        title="Stop (kill process)"
      >
        ✕
      </span>
    </div>
  );
}
