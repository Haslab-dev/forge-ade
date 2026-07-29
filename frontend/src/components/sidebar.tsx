import { useState, useEffect, useCallback, useRef } from "react";
import {
  FolderTree,
  Search,
  Terminal,
  PanelLeftClose,
  PanelLeft,
  File,
  Shell,
  Code2,
  CaseSensitive,
  WholeWord,
  Regex,
} from "lucide-react";
import { cn } from "../lib/utils";
import { Explorer } from "../panels/explorer";
import { ScrollArea } from "./ui/scroll-area";
import { SimpleModal } from "./simple-modal";
import { globalOpenFile } from "../panels/editor";
import { EventsOn } from "../../wailsjs/runtime";
import {
  SearchContent,
  SearchFilename,
  SearchSymbols,
  SearchContentWithOptions,
  SearchFilenameWithOptions,
  SearchSymbolsWithOptions,
  ListSessions,
  CreateShell,
  StopSession,
  RenameSession,
} from "../../wailsjs/go/main/App";
import { terminal, search } from "../../wailsjs/go/models";

interface SidebarProps {
  folders: string[];
  onOpenSession?: (id: string) => void;
  sessions?: terminal.Session[];
  onRefreshSessions?: () => void;
  cwd?: string;
  onCreateShell?: () => void;
  onRefreshWorkspace?: () => void;
}

const sidebarTabs = [
  { id: "explorer", icon: FolderTree, label: "Explorer" },
  { id: "search", icon: Search, label: "Search" },
  { id: "runtime", icon: Terminal, label: "Runtime" },
];

export function Sidebar({ folders, onOpenSession, sessions: propSessions, onRefreshSessions, cwd, onCreateShell, onRefreshWorkspace }: SidebarProps) {
  const [activeTab, setActiveTab] = useState("explorer");
  const [collapsed, setCollapsed] = useState(false);

  const handleTabClick = (tabId: string) => {
    if (collapsed) {
      setActiveTab(tabId);
      setCollapsed(false);
    } else if (activeTab === tabId) {
      setCollapsed(true);
    } else {
      setActiveTab(tabId);
    }
  };

  return (
    <div className="flex h-full">
      <div className="flex flex-col items-center gap-1 py-2 px-1 border-r bg-muted/30 w-12 shrink-0">
        {sidebarTabs.map((tab) => (
          <button
            key={tab.id}
            className={cn(
              "p-2 rounded-md transition-colors cursor-pointer",
              !collapsed && activeTab === tab.id
                ? "bg-accent text-accent-foreground"
                : "text-muted-foreground hover:text-foreground hover:bg-accent/50"
            )}
            onClick={() => handleTabClick(tab.id)}
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
            {activeTab === "explorer" && <Explorer roots={folders} onRefresh={onRefreshWorkspace} />}
            {activeTab === "search" && <SearchPanel />}
            {activeTab === "runtime" && (
              <RuntimePanel
                onOpenSession={onOpenSession}
                sessions={propSessions}
                onRefresh={onRefreshSessions}
                cwd={cwd}
                onCreateShell={onCreateShell}
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
  const [mode, setMode] = useState<"filename" | "content" | "symbols">("filename");
  const [matchCase, setMatchCase] = useState(false);
  const [matchWholeWord, setMatchWholeWord] = useState(false);
  const [useRegex, setUseRegex] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const executeSearch = useCallback(
    async (
      q: string,
      m: "filename" | "content" | "symbols",
      mc: boolean,
      mw: boolean,
      rx: boolean
    ) => {
      const trimmed = q.trim();
      if (!trimmed) {
        setResults([]);
        setLoading(false);
        setError("");
        return;
      }
      setLoading(true);
      setError("");
      try {
        const opts = {
          query: trimmed,
          matchCase: mc,
          matchWholeWord: mw,
          useRegex: rx,
          limit: 50,
        };
        let res: search.RankedResult[];
        if (m === "filename") {
          res = await SearchFilenameWithOptions(opts);
        } else if (m === "symbols") {
          res = await SearchSymbolsWithOptions(opts);
        } else {
          res = await SearchContentWithOptions(opts);
        }
        setResults(Array.isArray(res) ? res : []);
      } catch (err: any) {
        console.error("Search error:", err);
        setError(String(err?.message || err));
        setResults([]);
      } finally {
        setLoading(false);
      }
    },
    []
  );

  useEffect(() => {
    const timer = setTimeout(() => {
      executeSearch(query, mode, matchCase, matchWholeWord, useRegex);
    }, 150);
    return () => clearTimeout(timer);
  }, [query, mode, matchCase, matchWholeWord, useRegex, executeSearch]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.altKey) {
      if (e.key.toLowerCase() === "c") {
        e.preventDefault();
        setMatchCase((prev) => !prev);
      } else if (e.key.toLowerCase() === "w") {
        e.preventDefault();
        setMatchWholeWord((prev) => !prev);
      } else if (e.key.toLowerCase() === "r") {
        e.preventDefault();
        setUseRegex((prev) => !prev);
      }
    }
  };

  return (
    <div className="flex flex-col h-full">
      <div className="p-2 border-b space-y-2">
        <div className="flex gap-1">
          <button
            className={cn(
              "flex-1 text-xs px-1.5 py-1 rounded transition-colors cursor-pointer",
              mode === "filename" ? "bg-accent text-foreground font-medium" : "text-muted-foreground hover:bg-accent/50"
            )}
            onClick={() => setMode("filename")}
          >
            Files
          </button>
          <button
            className={cn(
              "flex-1 text-xs px-1.5 py-1 rounded transition-colors cursor-pointer",
              mode === "content" ? "bg-accent text-foreground font-medium" : "text-muted-foreground hover:bg-accent/50"
            )}
            onClick={() => setMode("content")}
          >
            Content
          </button>
          <button
            className={cn(
              "flex-1 text-xs px-1.5 py-1 rounded transition-colors cursor-pointer flex items-center justify-center gap-1",
              mode === "symbols" ? "bg-accent text-foreground font-medium" : "text-muted-foreground hover:bg-accent/50"
            )}
            onClick={() => setMode("symbols")}
          >
            <Code2 className="size-3" />
            Symbols
          </button>
        </div>
        <div className="relative flex items-center">
          <input
            className="w-full text-xs bg-muted rounded px-2.5 py-1.5 outline-none focus:ring-1 focus:ring-ring pr-20"
            placeholder={
              mode === "filename"
                ? "Search files..."
                : mode === "symbols"
                ? "Search symbols..."
                : "Search in files..."
            }
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={handleKeyDown}
          />
          <div className="absolute right-1 flex items-center gap-0.5">
            {loading && (
              <span className="text-muted-foreground text-[10px] animate-pulse mr-1">...</span>
            )}
            <button
              className={cn(
                "p-1 rounded text-[11px] transition-colors cursor-pointer select-none",
                matchCase
                  ? "bg-cyan-500/20 text-cyan-400 border border-cyan-500/40 font-bold"
                  : "text-muted-foreground hover:text-foreground hover:bg-accent/50 opacity-60"
              )}
              onClick={() => setMatchCase((prev) => !prev)}
              title="Match Case (Alt+C)"
            >
              <CaseSensitive className="size-3.5" />
            </button>
            <button
              className={cn(
                "p-1 rounded text-[11px] transition-colors cursor-pointer select-none",
                matchWholeWord
                  ? "bg-cyan-500/20 text-cyan-400 border border-cyan-500/40 font-bold"
                  : "text-muted-foreground hover:text-foreground hover:bg-accent/50 opacity-60"
              )}
              onClick={() => setMatchWholeWord((prev) => !prev)}
              title="Match Whole Word (Alt+W)"
            >
              <WholeWord className="size-3.5" />
            </button>
            <button
              className={cn(
                "p-1 rounded text-[11px] transition-colors cursor-pointer select-none",
                useRegex
                  ? "bg-cyan-500/20 text-cyan-400 border border-cyan-500/40 font-bold"
                  : "text-muted-foreground hover:text-foreground hover:bg-accent/50 opacity-60"
              )}
              onClick={() => setUseRegex((prev) => !prev)}
              title="Use Regular Expression (Alt+R)"
            >
              <Regex className="size-3.5" />
            </button>
          </div>
        </div>
      </div>
      <ScrollArea className="flex-1">
        {error && (
          <div className="p-3 text-xs text-red-400 bg-red-500/10 border-b border-red-500/20">
            {error}
          </div>
        )}
        {!loading && query.trim() && results.length === 0 && !error && (
          <p className="p-3 text-xs text-muted-foreground">No matching {mode === "symbols" ? "symbols" : "results"} found</p>
        )}
        {!query.trim() && (
          <p className="p-3 text-xs text-muted-foreground opacity-70">
            Type to search {mode === "filename" ? "filenames" : mode === "symbols" ? "code symbols (functions, types, classes)" : "file contents"}...
          </p>
        )}
        {results.map((r, i) => (
          <div
            key={i}
            className="px-3 py-1.5 text-xs hover:bg-accent cursor-pointer border-b border-border/40 transition-colors group"
            onClick={() => globalOpenFile(r.path, r.line)}
          >
            <div className="flex items-center justify-between gap-1">
              <div className="flex items-center gap-1.5 min-w-0">
                <File className="size-3 shrink-0 text-cyan-400" />
                <span className="truncate font-medium text-foreground group-hover:text-cyan-300">
                  {r.filename}
                </span>
              </div>
              {r.line ? (
                <span className="text-[10px] px-1 py-0.2 rounded bg-muted/60 text-muted-foreground shrink-0 font-mono">
                  :{r.line}
                </span>
              ) : null}
            </div>
            <div className="truncate text-[11px] text-muted-foreground/70 mt-0.5 font-mono">
              {r.path}
            </div>
            {r.content && (
              <div className="truncate text-muted-foreground mt-1 font-mono text-[11px] bg-muted/20 px-1.5 py-0.5 rounded border border-border/30">
                {r.content}
              </div>
            )}
          </div>
        ))}
      </ScrollArea>
    </div>
  );
}

function RuntimePanel({
  onOpenSession,
  sessions: propSessions,
  onRefresh,
  cwd,
  onCreateShell,
}: {
  onOpenSession?: (id: string) => void;
  sessions?: terminal.Session[];
  onRefresh?: () => void;
  cwd?: string;
  onCreateShell?: () => void;
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
    if (onCreateShell) {
      onCreateShell();
      return;
    }
    setError("");
    try { const s = await CreateShell("Shell", cwd || ""); refresh(); if (onOpenSession && s?.id) onOpenSession(s.id); }
    catch (err: unknown) { setError(String(err)); }
  }, [cwd, refresh, onOpenSession, onCreateShell]);

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
