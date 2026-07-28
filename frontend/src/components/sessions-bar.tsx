import { useState, useEffect, useCallback } from "react";
import { Shell, Terminal } from "lucide-react";
import { cn } from "../lib/utils";
import {
  ListSessions,
  CreateShell,
} from "../../wailsjs/go/main/App";
import { terminal } from "../../wailsjs/go/models";

interface SessionsBarProps {
  onSelectSession: (id: string) => void;
}

export function SessionsBar({ onSelectSession }: SessionsBarProps) {
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

  if (sessions.length === 0) {
    return (
      <div className="flex items-center h-9 px-3 border-t bg-[#1e1e2e] text-xs text-muted-foreground shrink-0 gap-2">
        <Terminal className="size-3" />
        <span>No sessions</span>
        <button
          className="ml-auto flex items-center gap-1 p-1 hover:bg-white/10 rounded text-muted-foreground hover:text-foreground cursor-pointer"
          onClick={handleNewShell}
          title="New Shell"
        >
          <Shell className="size-3" />
          <span>Shell</span>
        </button>
      </div>
    );
  }

  return (
    <div className="flex items-center h-9 px-1 border-t bg-[#1e1e2e] text-xs shrink-0 overflow-x-auto">
      {sessions.map((s) => (
        <button
          key={s.id}
          className={cn(
            "flex items-center gap-1.5 px-3 py-1 border-r shrink-0 transition-colors h-full cursor-pointer select-none",
            s.status === "running"
              ? "text-foreground hover:bg-white/10"
              : "text-muted-foreground hover:text-foreground hover:bg-white/10"
          )}
          onClick={() => onSelectSession(s.id)}
          title={`${s.name} (PID: ${s.pid})`}
        >
          <Shell className="size-3 text-green-500 shrink-0" />
          <span className="max-w-20 truncate">{s.name}</span>
          <span className={cn("w-1.5 h-1.5 rounded-full shrink-0", s.status === "running" ? "bg-green-500" : "bg-muted-foreground")} />
        </button>
      ))}
      <button
        className="flex items-center gap-1 p-1 hover:bg-white/10 rounded ml-1 shrink-0 text-muted-foreground hover:text-foreground cursor-pointer"
        onClick={handleNewShell}
        title="New Shell"
      >
        <Shell className="size-3" />
        <span>Shell</span>
      </button>
    </div>
  );
}
