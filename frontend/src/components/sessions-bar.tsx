import { useState, useEffect, useCallback } from "react";
import { Shell, Bot, Plus, Terminal, Square } from "lucide-react";
import { cn } from "../lib/utils";
import {
  ListSessions,
  CreateShell,
  CreateAIAgent,
  StopSession,
} from "../../wailsjs/go/main/App";
import { terminal } from "../../wailsjs/go/models";

interface SessionsBarProps {
  onSelectSession: (id: string) => void;
  onStopSession: (id: string) => void;
}

export function SessionsBar({ onSelectSession, onStopSession }: SessionsBarProps) {
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
    const provider = prompt("Provider (claude/opencode/kilo):")?.trim() || "claude";
    try {
      const s = await CreateAIAgent(name, provider, "");
      refresh();
      onSelectSession(s.id);
    } catch {
      // ignore
    }
  }, [refresh, onSelectSession]);

  if (sessions.length === 0) {
    return (
      <div className="flex items-center h-9 px-3 border-t bg-[#1e1e2e] text-xs text-muted-foreground shrink-0 gap-2">
        <Terminal className="size-3" />
        <span>No sessions</span>
        <button className="ml-auto p-1 hover:bg-white/10 rounded" onClick={handleNewShell} title="New Shell">
          <Shell className="size-3" />
        </button>
        <button className="p-1 hover:bg-white/10 rounded" onClick={handleNewAgent} title="New AI Agent">
          <Plus className="size-3" />
        </button>
      </div>
    );
  }

  return (
    <div className="flex items-center h-9 px-1 border-t bg-[#1e1e2e] text-xs shrink-0 overflow-x-auto">
      {sessions.map((s) => (
        <div
          key={s.id}
          className={cn(
            "flex items-center gap-1.5 px-3 py-1 border-r shrink-0 transition-colors h-full",
            "hover:bg-white/10 text-muted-foreground hover:text-foreground group"
          )}
        >
          <button
            className="flex items-center gap-1.5 flex-1 min-w-0 h-full"
            onClick={() => onSelectSession(s.id)}
            title={`${s.name} (PID: ${s.pid})`}
          >
            {s.type === "shell" ? (
              <Shell className="size-3 text-green-500 shrink-0" />
            ) : (
              <Bot className="size-3 text-cyan-500 shrink-0" />
            )}
            <span className="max-w-20 truncate">{s.name}</span>
            <span className={cn("w-1.5 h-1.5 rounded-full shrink-0", s.status === "running" ? "bg-green-500" : "bg-muted-foreground")} />
          </button>
          <button
            className="p-0.5 hover:bg-red-500/20 rounded opacity-0 group-hover:opacity-100 transition-opacity shrink-0"
            onClick={() => onStopSession(s.id)}
            title="Stop session (kill process)"
          >
            <Square className="size-2.5 text-red-400" />
          </button>
        </div>
      ))}
      <button className="p-1 hover:bg-white/10 rounded ml-1 shrink-0 text-muted-foreground hover:text-foreground" onClick={handleNewShell} title="New Shell">
        <Shell className="size-3" />
      </button>
      <button className="p-1 hover:bg-white/10 rounded shrink-0 text-muted-foreground hover:text-foreground" onClick={handleNewAgent} title="New AI Agent">
        <Plus className="size-3" />
      </button>
    </div>
  );
}
