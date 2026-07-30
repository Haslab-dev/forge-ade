import { useState, useEffect, useCallback, useRef } from "react";
import {
  IconFolder,
  IconSearch,
  IconTerminal2,
  IconRobot,
  IconGitBranch,
  IconLayoutSidebarLeftCollapse,
  IconLayoutSidebarLeftExpand,
  IconSettings,
  IconFileCode,
  IconLetterCase,
  IconMist,
  IconRegex,
  IconPlus,
  IconX,
  IconCheck,
  IconTrash,
} from "@tabler/icons-react";
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
  ListAgentSessions,
  CreateAgentSession,
  DeleteAgentSession,
} from "../../wailsjs/go/main/App";
import { terminal, search } from "../../wailsjs/go/models";
import { GlobalSettingsModal } from "./global-settings-modal";

interface SidebarProps {
  folders: string[];
  onOpenSession?: (id: string) => void;
  sessions?: terminal.Session[];
  onRefreshSessions?: () => void;
  cwd?: string;
  onCreateShell?: () => void;
  onRefreshWorkspace?: () => void;
  collapsed?: boolean;
  onToggleCollapse?: (collapsed: boolean) => void;
}

import { GitSidebarPanel } from "../panels/git-sidebar-panel";

const sidebarTabs = [
  { id: "explorer", icon: IconFolder, label: "Explorer" },
  { id: "search", icon: IconSearch, label: "Search" },
  { id: "git", icon: IconGitBranch, label: "Source Control" },
  { id: "runtime", icon: IconTerminal2, label: "Runtime" },
];

export function Sidebar({
  folders,
  onOpenSession,
  sessions: propSessions,
  onRefreshSessions,
  cwd,
  onCreateShell,
  onRefreshWorkspace,
  collapsed = false,
  onToggleCollapse,
}: SidebarProps) {
  const [activeTab, setActiveTab] = useState("explorer");
  const [showSettingsModal, setShowSettingsModal] = useState(false);

  const handleTabClick = (tabId: string) => {
    setActiveTab(tabId);
    if (collapsed && onToggleCollapse) {
      onToggleCollapse(false);
    }
  };

  const handleTabDoubleClick = (tabId: string) => {
    setActiveTab(tabId);
    if (onToggleCollapse) {
      onToggleCollapse(!collapsed);
    }
  };

  return (
    <div className="flex h-full w-full overflow-hidden">
      {/* Icon strip */}
      <div className="flex flex-col items-center gap-1 py-2 px-1 border-r bg-muted/30 w-12 shrink-0 select-none">
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
            onDoubleClick={() => handleTabDoubleClick(tab.id)}
            title={`${tab.label} (Double-click to toggle sidebar)`}
          >
            <tab.icon className="size-4" />
          </button>
        ))}
        <div className="flex-1" />

        {/* Global Settings button */}
        <button
          className="p-2 rounded-md text-muted-foreground hover:text-foreground hover:bg-accent/50 cursor-pointer"
          onClick={() => setShowSettingsModal(true)}
          title="Global Settings (Providers, MCP, Skills, Themes)"
        >
          <IconSettings className="size-4" />
        </button>

        {/* Collapse toggle */}
        <button
          className="p-2 rounded-md text-muted-foreground hover:text-foreground hover:bg-accent/50 cursor-pointer"
          onClick={() => onToggleCollapse?.(!collapsed)}
          title={collapsed ? "Expand Sidebar" : "Collapse Sidebar"}
        >
          {collapsed ? <IconLayoutSidebarLeftExpand className="size-4" /> : <IconLayoutSidebarLeftCollapse className="size-4" />}
        </button>
      </div>

      {/* Dynamic resizable body */}
      {!collapsed && (
        <div className="flex-1 border-r bg-background flex flex-col min-w-0 h-full overflow-hidden">
          <div className="flex items-center justify-between px-3 py-2 text-xs font-medium text-muted-foreground uppercase tracking-wider border-b shrink-0">
            <span>{sidebarTabs.find((t) => t.id === activeTab)?.label}</span>
          </div>
          <div className="flex-1 overflow-hidden">
            {activeTab === "explorer" && <Explorer roots={folders} onRefresh={onRefreshWorkspace} />}
            {activeTab === "search" && <SearchPanel />}
            {activeTab === "git" && <GitSidebarPanel />}
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

      {/* Global Settings Modal */}
      <GlobalSettingsModal open={showSettingsModal} onClose={() => setShowSettingsModal(false)} />
    </div>
  );
}

function SearchPanel() {
  const [query, setQuery] = useState("");
  const [mode, setMode] = useState<"filename" | "content" | "symbols">("filename");
  const [caseSensitive, setCaseSensitive] = useState(false);
  const [wholeWord, setWholeWord] = useState(false);
  const [useRegex, setUseRegex] = useState(false);

  const [results, setResults] = useState<search.RankedResult[]>([]);
  const [searching, setSearching] = useState(false);

  const doSearch = useCallback(async () => {
    if (!query.trim()) {
      setResults([]);
      return;
    }
    setSearching(true);
    try {
      let res: search.RankedResult[] = [];
      const opts = new search.SearchOptions({
        query,
        caseSensitive,
        wholeWord,
        regex: useRegex,
        limit: 50,
      });

      if (mode === "filename") {
        res = await SearchFilenameWithOptions(opts);
      } else if (mode === "content") {
        res = await SearchContentWithOptions(opts);
      } else if (mode === "symbols") {
        res = await SearchSymbolsWithOptions(opts);
      }
      setResults(res ?? []);
    } catch (err) {
      console.error(err);
      setResults([]);
    } finally {
      setSearching(false);
    }
  }, [query, mode, caseSensitive, wholeWord, useRegex]);

  useEffect(() => {
    const timer = setTimeout(doSearch, 200);
    return () => clearTimeout(timer);
  }, [doSearch]);

  return (
    <div className="flex flex-col h-full p-2 gap-2 text-xs">
      <div className="flex gap-1">
        <button
          className={cn("flex-1 py-1 px-2 rounded font-medium", mode === "filename" ? "bg-primary text-primary-foreground" : "bg-muted text-muted-foreground")}
          onClick={() => setMode("filename")}
        >
          Files
        </button>
        <button
          className={cn("flex-1 py-1 px-2 rounded font-medium", mode === "content" ? "bg-primary text-primary-foreground" : "bg-muted text-muted-foreground")}
          onClick={() => setMode("content")}
        >
          Content
        </button>
        <button
          className={cn("flex-1 py-1 px-2 rounded font-medium", mode === "symbols" ? "bg-primary text-primary-foreground" : "bg-muted text-muted-foreground")}
          onClick={() => setMode("symbols")}
        >
          Symbols
        </button>
      </div>

      <div className="relative">
        <input
          type="text"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder={`Search ${mode}...`}
          className="w-full bg-background border rounded px-2.5 py-1 text-xs focus:outline-none focus:border-primary"
        />
      </div>

      <div className="flex items-center gap-1">
        <button
          className={cn("p-1 rounded text-xs border", caseSensitive ? "bg-accent text-foreground" : "text-muted-foreground")}
          onClick={() => setCaseSensitive(!caseSensitive)}
          title="Match Case"
        >
          <IconLetterCase className="size-3.5" />
        </button>
        <button
          className={cn("p-1 rounded text-xs border", useRegex ? "bg-accent text-foreground" : "text-muted-foreground")}
          onClick={() => setUseRegex(!useRegex)}
          title="Use Regex"
        >
          <IconRegex className="size-3.5" />
        </button>
      </div>

      <div className="flex-1 overflow-y-auto space-y-1">
        {searching && <div className="text-muted-foreground p-2">Searching...</div>}
        {!searching && results.length === 0 && query && <div className="text-muted-foreground p-2">No results</div>}
        {results.map((r, i) => (
          <div
            key={i}
            onClick={() => globalOpenFile(r.path ?? (r as any).Path)}
            className="p-1.5 rounded hover:bg-accent/50 cursor-pointer flex flex-col gap-0.5"
          >
            <span className="font-medium truncate text-foreground">{r.path ?? (r as any).Path}</span>
            {((r.line ?? (r as any).Line) || 0) > 0 && (
              <span className="text-[10px] text-muted-foreground">
                Line {r.line ?? (r as any).Line}: {(r as any).snippet ?? (r as any).Snippet}
              </span>
            )}
          </div>
        ))}
      </div>
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
  const [agentSessions, setAgentSessions] = useState<any[]>([]);

  const loadAgents = useCallback(async () => {
    try {
      const list = await ListAgentSessions();
      setAgentSessions(Array.isArray(list) ? list : []);
    } catch { /* ignore */ }
  }, []);

  useEffect(() => {
    loadAgents();
    const unsubAgent = EventsOn("agent:updated", () => loadAgents());
    const unsubTerm = EventsOn("terminal:closed", () => {
      if (onRefresh) onRefresh();
    });
    return () => {
      if (typeof unsubAgent === "function") unsubAgent();
      if (typeof unsubTerm === "function") unsubTerm();
    };
  }, [loadAgents, onRefresh]);

  async function handleKillShell(id: string, e: React.MouseEvent) {
    e.stopPropagation();
    try {
      await StopSession(id);
      if (onRefresh) onRefresh();
    } catch (err) {
      console.error("Failed to kill shell session:", err);
    }
  }

  async function handleKillAgent(id: string, e: React.MouseEvent) {
    e.stopPropagation();
    try {
      await DeleteAgentSession(id);
      loadAgents();
    } catch (err) {
      console.error("Failed to kill agent session:", err);
    }
  }

  async function handleCreateAgent() {
    try {
      const created: any = await CreateAgentSession("New Agent", "coding", "");
      loadAgents();
      if (created && created.id && onOpenSession) {
        onOpenSession(created.id);
      }
    } catch (err) {
      console.error(err);
    }
  }

  return (
    <div className="flex flex-col h-full p-2 gap-2 text-xs">
      <div className="flex items-center justify-between border-b border-[var(--color-border)] pb-2">
        <span className="font-semibold text-muted-foreground uppercase text-[10px]">Active Sessions</span>
        <div className="flex items-center gap-1">
          <button
            onClick={onCreateShell}
            className="px-2 py-0.5 rounded bg-green-600/20 text-green-300 hover:bg-green-600 hover:text-white text-[10px] font-semibold flex items-center gap-0.5 cursor-pointer"
            title="Launch New Shell"
          >
            <IconPlus className="size-3" />
            <span>Shell</span>
          </button>
          <button
            onClick={handleCreateAgent}
            className="px-2 py-0.5 rounded bg-blue-600/20 text-blue-300 hover:bg-blue-600 hover:text-white text-[10px] font-semibold flex items-center gap-0.5 cursor-pointer"
            title="Launch New Agent"
          >
            <IconPlus className="size-3" />
            <span>Agent</span>
          </button>
        </div>
      </div>

      <div className="flex-1 overflow-y-auto space-y-1.5">
        {/* Shell Sessions */}
        {(propSessions ?? []).map((s) => {
          const pidNum = s.pid || (s as any).PID || (s as any).pid;
          return (
            <div
              key={s.id}
              onClick={() => onOpenSession?.(s.id)}
              className="p-2 rounded border border-[var(--color-border)] bg-[var(--color-bg-primary)] hover:bg-accent cursor-pointer flex items-center justify-between group"
            >
              <div className="flex items-center space-x-1.5 min-w-0">
                <IconTerminal2 className="size-3.5 text-green-400 shrink-0" />
                <span className="font-medium truncate text-foreground">{s.name}</span>
                {pidNum && (
                  <span className="font-mono text-[10px] text-gray-400 bg-black/40 px-1 py-0.5 rounded shrink-0">
                    PID: {pidNum}
                  </span>
                )}
              </div>
              <div className="flex items-center space-x-1.5 shrink-0">
                <span className={cn("w-2 h-2 rounded-full", s.status === "running" ? "bg-green-500" : "bg-muted-foreground")} />
                <button
                  onClick={(e) => handleKillShell(s.id, e)}
                  className="p-1 rounded text-gray-400 hover:text-rose-400 hover:bg-rose-500/10 cursor-pointer"
                  title="Kill Shell Session"
                >
                  <IconTrash className="size-3.5" />
                </button>
              </div>
            </div>
          );
        })}

        {/* Agent Sessions */}
        {agentSessions.map((a) => (
          <div
            key={a.id}
            onClick={() => onOpenSession?.(a.id)}
            className="p-2 rounded border border-[var(--color-border)] bg-[var(--color-bg-primary)] hover:bg-accent cursor-pointer flex items-center justify-between group"
          >
            <div className="flex items-center space-x-1.5 min-w-0">
              <IconRobot className="size-3.5 text-blue-400 shrink-0" />
              <span className="font-medium truncate text-foreground">{a.name}</span>
              <span className="font-mono text-[10px] text-blue-400 bg-blue-950/60 border border-blue-800/60 px-1 py-0.5 rounded shrink-0 uppercase">
                Agent
              </span>
            </div>
            <div className="flex items-center space-x-1.5 shrink-0">
              <span className="w-2 h-2 rounded-full bg-blue-500 animate-pulse" />
              <button
                onClick={(e) => handleKillAgent(a.id, e)}
                className="p-1 rounded text-gray-400 hover:text-rose-400 hover:bg-rose-500/10 cursor-pointer"
                title="Kill Agent Session"
              >
                <IconTrash className="size-3.5" />
              </button>
            </div>
          </div>
        ))}

        {(propSessions ?? []).length === 0 && agentSessions.length === 0 && (
          <div className="text-[11px] text-muted-foreground p-3 text-center italic">
            No active shell or agent sessions running.
          </div>
        )}
      </div>
    </div>
  );
}
