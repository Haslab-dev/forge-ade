import { useState, useEffect, useMemo } from "react";
import { Shell, X, Plus, Maximize2, Columns3, Grid3x3 } from "lucide-react";
import { TerminalView } from "../components/terminal-view";
import { cn } from "../lib/utils";
import { RenameSession } from "../../wailsjs/go/main/App";
import { terminal } from "../../wailsjs/go/models";

interface ShellScreenProps {
  sessions: terminal.Session[];
  onCreateShell: () => void;
  onStopSession: (id: string) => void;
  onRenameSession: (id: string, name: string) => void;
  initialSessionId?: string | null;
}

export function ShellScreen({
  sessions,
  onCreateShell,
  onStopSession,
  onRenameSession,
  initialSessionId,
}: ShellScreenProps) {
  const [layoutMode, setLayoutMode] = useState<"single" | "horizontal" | "grid4">("single");
  const [selectedSessionId, setSelectedSessionId] = useState<string | null>(
    initialSessionId ?? sessions[0]?.id ?? null
  );

  // Sync initialSessionId when it changes externally
  useEffect(() => {
    if (initialSessionId && sessions.find((s) => s.id === initialSessionId)) {
      setSelectedSessionId(initialSessionId);
    }
  }, [initialSessionId, sessions]);

  // If selected session disappears, pick another
  useEffect(() => {
    if (selectedSessionId && !sessions.find((s) => s.id === selectedSessionId)) {
      setSelectedSessionId(sessions[0]?.id ?? null);
    }
  }, [sessions, selectedSessionId]);

  const handleRename = useMemo(
    () => (s: terminal.Session) => {
      const newName = prompt("Rename session:", s.name);
      if (newName && newName !== s.name) {
        RenameSession(s.id, newName)
          .then(() => onRenameSession(s.id, newName))
          .catch(console.error);
      }
    },
    [onRenameSession]
  );

  /** Render a single cell: either a terminal or a "New Shell" placeholder */
  const renderCell = (session: terminal.Session | null, key: string) => {
    if (!session) {
      return (
        <div
          key={key}
          className="flex items-center justify-center border-r border-b bg-muted/5 h-full"
        >
          <button
            className="flex items-center gap-1.5 px-3 py-2 text-xs text-muted-foreground hover:text-foreground hover:bg-accent/50 rounded transition-colors cursor-pointer"
            onClick={onCreateShell}
          >
            <Plus className="size-3.5" />
            New Shell
          </button>
        </div>
      );
    }

    return (
      <div
        key={session.id}
        className="flex flex-col border-r border-b overflow-hidden min-w-0 h-full"
      >
        {/* Cell header */}
        <div className="flex items-center justify-between px-2 py-0.5 text-[11px] text-muted-foreground bg-muted/20 border-b shrink-0">
          <div className="flex items-center gap-1 min-w-0">
            <Shell className="size-2.5 text-green-500 shrink-0" />
            <span
              className="truncate cursor-pointer hover:text-foreground"
              onContextMenu={(e) => {
                e.preventDefault();
                handleRename(session);
              }}
              title="Right-click to rename"
            >
              {session.name}
            </span>
            <span
              className={cn(
                "w-1.5 h-1.5 rounded-full shrink-0",
                session.status === "running" ? "bg-green-500" : "bg-muted-foreground"
              )}
            />
          </div>
          <button
            className="p-0.5 hover:bg-accent rounded shrink-0 ml-1 cursor-pointer"
            onClick={() => onStopSession(session.id)}
            title="Stop session"
          >
            <X className="size-2.5" />
          </button>
        </div>
        {/* Terminal area */}
        <div className="flex-1 min-h-0 overflow-hidden">
          <TerminalView sessionId={session.id} />
        </div>
      </div>
    );
  };

  /** Chunk sessions into groups of `size` with null padding, filling up to count */
  const chunked = useMemo(() => {
    const cellsPerGroup = layoutMode === "horizontal" ? 3 : 4;
    const fillCount = Math.max(sessions.length, cellsPerGroup);
    const chunks: (terminal.Session | null)[][] = [];
    for (let i = 0; i < fillCount; i += cellsPerGroup) {
      const chunk: (terminal.Session | null)[] = [];
      for (let j = 0; j < cellsPerGroup; j++) {
        chunk.push(sessions[i + j] ?? null);
      }
      chunks.push(chunk);
    }
    return chunks;
  }, [layoutMode, sessions]);

  // ---------- Filter buttons component ----------
  const filterBar = (
    <div className="flex items-center gap-0.5">
      <button
        className={cn(
          "p-1.5 rounded transition-colors cursor-pointer",
          layoutMode === "single"
            ? "bg-accent text-foreground"
            : "text-muted-foreground hover:text-foreground hover:bg-accent/50"
        )}
        onClick={() => setLayoutMode("single")}
        title="Single terminal"
      >
        <Maximize2 className="size-3.5" />
      </button>
      <button
        className={cn(
          "p-1.5 rounded transition-colors cursor-pointer",
          layoutMode === "horizontal"
            ? "bg-accent text-foreground"
            : "text-muted-foreground hover:text-foreground hover:bg-accent/50"
        )}
        onClick={() => setLayoutMode("horizontal")}
        title="Horizontal group (3 per row)"
      >
        <Columns3 className="size-3.5" />
      </button>
      <button
        className={cn(
          "p-1.5 rounded transition-colors cursor-pointer",
          layoutMode === "grid4"
            ? "bg-accent text-foreground"
            : "text-muted-foreground hover:text-foreground hover:bg-accent/50"
        )}
        onClick={() => setLayoutMode("grid4")}
        title="4-grid layout (2x2)"
      >
        <Grid3x3 className="size-3.5" />
      </button>
    </div>
  );

  // ---------- Empty state ----------
  if (sessions.length === 0) {
    return (
      <div className="flex flex-col h-full">
        <div className="flex items-center justify-end px-3 py-1.5 border-b bg-muted/20 shrink-0">
          {filterBar}
        </div>
        <div className="flex-1 flex items-center justify-center">
          <div className="text-center">
            <Shell className="size-10 mx-auto mb-3 opacity-30" />
            <p className="text-sm text-muted-foreground mb-3">No sessions open</p>
            <button
              className="inline-flex items-center gap-1.5 px-4 py-2 text-xs font-medium bg-foreground/10 hover:bg-foreground/20 rounded transition-colors cursor-pointer"
              onClick={onCreateShell}
            >
              <Plus className="size-3.5" />
              New Shell
            </button>
          </div>
        </div>
      </div>
    );
  }

  // ---------- Single mode (one terminal, full area) ----------
  if (layoutMode === "single") {
    const activeSession = sessions.find((s) => s.id === selectedSessionId) ?? sessions[0];

    return (
      <div className="flex flex-col h-full">
        <div className="flex items-center justify-between px-3 py-0 border-b bg-muted/20 shrink-0">
          {/* Session selector tabs */}
          <div className="flex items-center overflow-x-auto">
            {sessions.map((s) => (
              <div
                key={s.id}
                className={cn(
                  "flex items-center gap-1 px-3 py-1.5 text-xs border-r cursor-pointer whitespace-nowrap shrink-0 select-none",
                  selectedSessionId === s.id
                    ? "bg-background border-b-2 border-b-cyan-500"
                    : "hover:bg-accent/50"
                )}
                onClick={() => setSelectedSessionId(s.id)}
                onContextMenu={(e) => {
                  e.preventDefault();
                  handleRename(s);
                }}
                title="Right-click to rename"
              >
                <Shell className="size-3 text-green-500 shrink-0" />
                <span className="truncate max-w-28">{s.name}</span>
                <span
                  className={cn(
                    "w-1.5 h-1.5 rounded-full",
                    s.status === "running" ? "bg-green-500" : "bg-muted-foreground"
                  )}
                />
              </div>
            ))}
          </div>
          {filterBar}
        </div>
        <div className="flex-1 overflow-hidden">
          <TerminalView sessionId={activeSession.id} />
        </div>
      </div>
    );
  }

  // ---------- Horizontal mode (up to 3 side by side, full height, horizontally scrollable) ----------
  if (layoutMode === "horizontal") {
    return (
      <div className="flex flex-col h-full">
        <div className="flex items-center justify-end px-3 py-1.5 border-b bg-muted/20 shrink-0">
          {filterBar}
        </div>
        <div className="flex-1 min-h-0 flex overflow-x-auto">
          {chunked.map((group, gi) => (
            <div key={gi} className="flex flex-shrink-0 h-full" style={{ minWidth: "100%" }}>
                {group.map((s, ci) => (
                  <div key={s ? s.id : `empty-${gi}-${ci}`} className="flex-1 min-w-0">
                    {renderCell(s, `h-${gi}-${ci}`)}
                  </div>
                ))}
            </div>
          ))}
        </div>
      </div>
    );
  }

  // ---------- Grid4 mode (2x2 per page, fills full width, horizontally scrollable) ----------
  return (
    <div className="flex flex-col h-full">
      <div className="flex items-center justify-end px-3 py-1.5 border-b bg-muted/20 shrink-0">
        {filterBar}
      </div>
      <div className="flex-1 min-h-0 flex overflow-x-auto">
        {chunked.map((page, pi) => (
          <div
            key={pi}
            className="grid grid-cols-2 grid-rows-2 h-full flex-shrink-0"
            style={{ minWidth: "100%" }}
          >
            {page.map((s, ci) => renderCell(s, `g-${pi}-${ci}`))}
          </div>
        ))}
      </div>
    </div>
  );
}
