import { useState, useEffect, useCallback } from "react";
import { Terminal, Shell, Bot, X, Plus } from "lucide-react";
import { cn } from "../lib/utils";
import {
  ListSessions,
  CreateShell,
  CreateAIAgent,
  StopSession,
} from "../../wailsjs/go/main/App";
import { terminal } from "../../wailsjs/go/models";

interface SessionsBarProps {
  activeSessionId: string | null;
  onSelectSession: (id: string | null) => void;
}

export function SessionsBar({ activeSessionId, onSelectSession }: SessionsBarProps) {
  const [sessions, setSessions] = useState<terminal.Session[]>([]);

  const refresh = useCallback(async () => {
    try {
      const list: terminal.Session[] = await ListSessions();
      setSessions(Array.isArray(list) ? list : []);
    } catch {
      // ignore
    }
  }, []);

  useEffect(() => {
    refresh();
    const interval = setInterval(refresh, 2000);
    return () => clearInterval(interval);
  }, [refresh]);

  const handleNewShell = useCallback(async () => {
    try {
      const s = await CreateShell("Shell", "");
      refresh();
      onSelectSession(s.id);
    } catch {
      // ignore
    }
  }, [refresh, onSelectSession]);

  const handleNewAgent = useCallback(async () => {
    const name = prompt("Session name:")?.trim();
    if (!name) return;
    const provider = prompt("Provider (claude/opencode/kilo/gemini/codex/aider):")?.trim() || "claude";
    try {
      const s = await CreateAIAgent(name, provider, "");
      refresh();
      onSelectSession(s.id);
    } catch {
      // ignore
    }
  }, [refresh, onSelectSession]);

  const handleStop = useCallback(async (id: string) => {
    try {
      await StopSession(id);
      if (activeSessionId === id) onSelectSession(null);
      refresh();
    } catch {
      // ignore
    }
  }, [activeSessionId, onSelectSession, refresh]);

  const handleSelect = useCallback((id: string) => {
    onSelectSession(activeSessionId === id ? null : id);
  }, [activeSessionId, onSelectSession]);

  return (
    <div className="flex items-center h-9 px-1 border-t bg-[#1e1e2e] text-xs shrink-0 overflow-x-auto">
      {sessions.length === 0 && (
        <span className="px-2 text-muted-foreground flex items-center gap-1">
          <Terminal className="size-3" />
          No sessions
        </span>
      )}
      {sessions.map((s) => (
        <div
          key={s.id}
          className={cn(
            "flex items-center gap-1.5 px-3 py-1 border-r cursor-pointer shrink-0 transition-colors",
            activeSessionId === s.id
              ? "bg-[#181825] text-foreground border-t-2 border-t-cyan-500"
              : "text-muted-foreground hover:text-foreground hover:bg-[#181825]/50"
          )}
          onClick={() => handleSelect(s.id)}
        >
          {s.type === "shell" ? (
            <Shell className="size-3 text-green-500" />
          ) : (
            <Bot className="size-3 text-cyan-500" />
          )}
          <span className="max-w-24 truncate">{s.name}</span>
          <span className={cn("w-1.5 h-1.5 rounded-full", s.status === "running" ? "bg-green-500" : "bg-muted-foreground")} />
          <button
            className="p-0.5 hover:bg-accent rounded ml-0.5 opacity-60 hover:opacity-100 transition-opacity"
            onClick={(e) => { e.stopPropagation(); handleStop(s.id); }}
          >
            <X className="size-2.5" />
          </button>
        </div>
      ))}
      <button className="p-1 hover:bg-accent rounded ml-1 shrink-0 text-muted-foreground hover:text-foreground" onClick={handleNewShell} title="New Shell">
        <Shell className="size-3" />
      </button>
      <button className="p-1 hover:bg-accent rounded shrink-0 text-muted-foreground hover:text-foreground" onClick={handleNewAgent} title="New AI Agent">
        <Plus className="size-3" />
      </button>
    </div>
  );
}
