import { useState, useEffect, useCallback } from "react";
import {
  FolderTree,
  Search,
  GitBranch,
  Terminal,
  Bot,
  PanelLeftClose,
  PanelLeft,
  File,
} from "lucide-react";
import { cn } from "../lib/utils";
import { Explorer } from "../panels/explorer";
import { ScrollArea } from "../components/ui/scroll-area";
import {
  SearchContent,
  SearchFilename,
  DiscoverRepos,
  GetRepoStatus,
  ListTerminals,
  ListAgents,
  CreateTerminal,
  StartAgent,
} from "../../wailsjs/go/main/App";
import { search } from "../../wailsjs/go/models";
import type { GitStatusEntry, TerminalSession, Agent } from "../types";

interface SidebarProps {
  folders: string[];
}

const sidebarTabs = [
  { id: "explorer", icon: FolderTree, label: "Explorer" },
  { id: "search", icon: Search, label: "Search" },
  { id: "git", icon: GitBranch, label: "Git" },
  { id: "terminal", icon: Terminal, label: "Terminal" },
  { id: "agents", icon: Bot, label: "Agents" },
];

export function Sidebar({ folders }: SidebarProps) {
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
              "p-2 rounded-md transition-colors",
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
          className="p-2 rounded-md text-muted-foreground hover:text-foreground hover:bg-accent/50"
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
            {activeTab === "terminal" && <TerminalPanel />}
            {activeTab === "agents" && <AgentPanel />}
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
  const [repos, setRepos] = useState<
    { path: string; status: GitStatusEntry[] }[]
  >([]);
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
          className="text-xs px-2 py-0.5 hover:bg-accent rounded transition-colors"
          onClick={refresh}
        >
          {loading ? "..." : "Refresh"}
        </button>
      </div>
      <ScrollArea className="flex-1">
        {repos.length === 0 && (
          <p className="p-3 text-xs text-muted-foreground">
            No repositories found
          </p>
        )}
        {repos.map((repo) => (
          <div key={repo.path} className="border-b border-border/50">
            <div className="flex items-center gap-1 px-3 py-1.5 text-xs font-medium">
              <GitBranch className="size-3.5 text-muted-foreground" />
              <span className="truncate">
                {repo.path.split("/").pop()}
              </span>
            </div>
            {repo.status.length === 0 && (
              <p className="px-3 pb-1.5 text-[10px] text-muted-foreground">
                Clean working tree
              </p>
            )}
            {repo.status.slice(0, 10).map((s, i) => (
              <div
                key={i}
                className="flex items-center gap-2 px-3 py-0.5 text-xs hover:bg-accent"
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
// Terminal Panel
// ---------------------------------------------------------------------------
function TerminalPanel() {
  const [sessions, setSessions] = useState<TerminalSession[]>([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  const refresh = useCallback(async () => {
    try {
      const list: TerminalSession[] = await ListTerminals();
      setSessions(Array.isArray(list) ? list : []);
      setError("");
    } catch (err: unknown) {
      setError(String(err));
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  const handleNewTerminal = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      await CreateTerminal("shell", "", "");
      refresh();
    } catch (err: unknown) {
      setError(String(err));
    }
    setLoading(false);
  }, [refresh]);

  return (
    <div className="flex flex-col h-full">
      <div className="p-2 border-b flex items-center justify-between">
        <span className="text-xs text-muted-foreground">
          {sessions.length} session{sessions.length !== 1 ? "s" : ""}
        </span>
        <button
          className="text-xs px-2 py-0.5 hover:bg-accent rounded transition-colors disabled:opacity-50"
          onClick={handleNewTerminal}
          disabled={loading}
        >
          {loading ? "..." : "+ New"}
        </button>
      </div>
      {error && (
        <div className="px-3 py-1.5 text-[10px] text-red-400 bg-red-500/10 border-b">
          {error}
        </div>
      )}
      <ScrollArea className="flex-1">
        {sessions.length === 0 && !error && (
          <p className="p-3 text-xs text-muted-foreground">
            No terminal sessions. Click + New to create one.
          </p>
        )}
        {sessions.map((s) => (
          <div
            key={s.id}
            className="flex items-center gap-2 px-3 py-1.5 text-xs hover:bg-accent cursor-pointer"
          >
            <Terminal className="size-3.5 text-muted-foreground" />
            <span className="font-medium">{s.name ?? "shell"}</span>
            <span className="text-muted-foreground">PID: {s.pid}</span>
          </div>
        ))}
      </ScrollArea>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Agent Panel
// ---------------------------------------------------------------------------
function AgentPanel() {
  const [agents, setAgents] = useState<Agent[]>([]);
  const [error, setError] = useState("");
  const [starting, setStarting] = useState(false);

  const refresh = useCallback(async () => {
    try {
      const list: Agent[] = await ListAgents();
      setAgents(Array.isArray(list) ? list : []);
      setError("");
    } catch (err: unknown) {
      setError(String(err));
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  const handleStartAgent = useCallback(async () => {
    const name = prompt("Agent name:")?.trim();
    if (!name) return;
    const provider =
      prompt("Provider (claude/opencode/gemini/codex/aider):")?.trim() ||
      "claude";
    setStarting(true);
    setError("");
    try {
      await StartAgent(name, provider, "");
      refresh();
    } catch (err: unknown) {
      setError(String(err));
    }
    setStarting(false);
  }, [refresh]);

  return (
    <div className="flex flex-col h-full">
      <div className="p-2 border-b flex items-center justify-between">
        <span className="text-xs text-muted-foreground">
          {agents.length} agent{agents.length !== 1 ? "s" : ""}
        </span>
        <button
          className="text-xs px-2 py-0.5 hover:bg-accent rounded transition-colors disabled:opacity-50"
          onClick={handleStartAgent}
          disabled={starting}
        >
          {starting ? "..." : "+ Start"}
        </button>
      </div>
      {error && (
        <div className="px-3 py-1.5 text-[10px] text-red-400 bg-red-500/10 border-b">
          {error}
        </div>
      )}
      <ScrollArea className="flex-1">
        {agents.length === 0 && !error && (
          <p className="p-3 text-xs text-muted-foreground">
            No agents running. Start one to begin.
          </p>
        )}
        {agents.map((a) => (
          <div
            key={a.id}
            className="flex items-center gap-2 px-3 py-1.5 text-xs hover:bg-accent"
          >
            <Bot className="size-3.5 shrink-0 text-muted-foreground" />
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-1.5">
                <span className="font-medium truncate">{a.name}</span>
                <span
                  className={cn(
                    "w-1.5 h-1.5 rounded-full shrink-0",
                    a.status === "running"
                      ? "bg-green-500"
                      : a.status === "error"
                        ? "bg-red-500"
                        : "bg-muted-foreground"
                  )}
                />
              </div>
              <div className="text-muted-foreground truncate">
                {a.provider} · PID: {a.pid}
              </div>
            </div>
          </div>
        ))}
      </ScrollArea>
    </div>
  );
}
