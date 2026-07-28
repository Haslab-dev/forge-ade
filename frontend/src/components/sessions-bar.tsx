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

export function SessionsBar() {
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
      await CreateShell("Shell", "");
      refresh();
    } catch {
      // ignore
    }
  }, [refresh]);

  const handleNewAgent = useCallback(async () => {
    const name = prompt("Session name:")?.trim();
    if (!name) return;
    const provider = prompt("Provider:")?.trim() || "claude";
    try {
      await CreateAIAgent(name, provider, "");
      refresh();
    } catch {
      // ignore
    }
  }, [refresh]);

  const handleStop = useCallback(async (id: string) => {
    try {
      await StopSession(id);
      refresh();
    } catch {
      // ignore
    }
  }, [refresh]);

  if (sessions.length === 0) {
    return (
      <div className="flex items-center h-8 px-2 border-t bg-muted/20 text-xs text-muted-foreground gap-2">
        <Terminal className="size-3" />
        <span>No sessions</span>
        <button className="ml-auto p-1 hover:bg-accent rounded" onClick={handleNewShell} title="New Shell">
          <Shell className="size-3" />
        </button>
        <button className="p-1 hover:bg-accent rounded" onClick={handleNewAgent} title="New AI Agent">
          <Bot className="size-3" />
        </button>
      </div>
    );
  }

  return (
    <div className="flex items-center h-8 px-1 border-t bg-muted/20 text-xs shrink-0 overflow-x-auto">
      {sessions.map((s) => (
        <div
          key={s.id}
          className={cn(
            "flex items-center gap-1 px-2 py-1 border-r cursor-pointer hover:bg-accent/50 shrink-0",
            s.status === "running" ? "text-foreground" : "text-muted-foreground"
          )}
        >
          {s.type === "shell" ? (
            <Shell className="size-3 text-green-500" />
          ) : (
            <Bot className="size-3 text-cyan-500" />
          )}
          <span className="max-w-20 truncate">{s.name}</span>
          <span className={cn("w-1.5 h-1.5 rounded-full", s.status === "running" ? "bg-green-500" : "bg-muted-foreground")} />
          <button
            className="p-0.5 hover:bg-accent rounded ml-0.5 opacity-60 hover:opacity-100"
            onClick={(e) => { e.stopPropagation(); handleStop(s.id); }}
          >
            <X className="size-2.5" />
          </button>
        </div>
      ))}
      <button className="p-1 hover:bg-accent rounded ml-1 shrink-0" onClick={handleNewShell} title="New Shell">
        <Shell className="size-3" />
      </button>
      <button className="p-1 hover:bg-accent rounded shrink-0" onClick={handleNewAgent} title="New AI Agent">
        <Plus className="size-3" />
      </button>
    </div>
  );
}
