import { useState, useEffect, useMemo } from "react";
import { Shell, X, Plus, Maximize2, Columns3, Grid3x3 } from "lucide-react";
import { TerminalView } from "../components/terminal-view";
import { cn } from "../lib/utils";
import { RenameSession } from "../../wailsjs/go/main/App";
import { terminal } from "../../wailsjs/go/models";

interface ShellScreenProps {
  sessions: terminal.Session[];
  onCreateShell: () => void;
  onCloseSession: (id: string) => void;
  onStopSession?: (id: string) => void;
  onRenameSession: (id: string, name: string) => void;
  initialSessionId?: string | null;
}

type LayoutMode = "single" | "horizontal" | "grid4";

export function ShellScreen({
  sessions,
  onCreateShell,
  onCloseSession,
  onStopSession,
  onRenameSession,
  initialSessionId,
}: ShellScreenProps) {
  const [layoutMode, setLayoutMode] = useState<LayoutMode>("single");
  const [hideEmptySlots, setHideEmptySlots] = useState(false);
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
          className="relative flex items-center justify-center border-r border-b bg-muted/5 h-full group"
        >
          <button
            className="absolute top-1.5 right-1.5 p-1 text-muted-foreground hover:text-foreground hover:bg-accent rounded transition-colors cursor-pointer"
            onClick={() => setHideEmptySlots(true)}
            title="Remove empty shell slot"
          >
            <X className="size-3.5" />
          </button>
          <button
            className="flex items-center gap-1.5 px-3 py-2 text-xs text-muted-foreground hover:text-foreground hover:bg-accent/50 rounded transition-colors cursor-pointer"
            onClick={() => {
              setHideEmptySlots(false);
              onCreateShell();
            }}
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
        className="flex flex-col border-r border-b overflow-hidden min-w-0 min-h-0 h-full w-full"
      >
        {/* Cell header */}
        <div className="flex items-center justify-between px-2 py-0.5 text-[11px] text-muted-foreground bg-muted/20 border-b shrink-0 select-none">
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
            onClick={() => onCloseSession(session.id)}
            title="Close view (session remains running)"
          >
            <X className="size-2.5" />
          </button>
        </div>
        {/* Terminal area */}
        <div className="flex-1 min-h-0 min-w-0 overflow-hidden h-full w-full">
          <TerminalView sessionId={session.id} isActive={true} />
        </div>
      </div>
    );
  };

  const cellsPerGroup = useMemo(() => {
    switch (layoutMode) {
      case "horizontal":
        return 3;
      case "grid4":
        return 4;
      default:
        return 1;
    }
  }, [layoutMode]);

  /** Chunk sessions into groups of `size` with optional null padding */
  const chunked = useMemo(() => {
    if (layoutMode === "single") return [];

    const fillCount = hideEmptySlots
      ? sessions.length
      : Math.max(sessions.length, cellsPerGroup);

    const chunks: (terminal.Session | null)[][] = [];
    for (let i = 0; i < fillCount; i += cellsPerGroup) {
      const chunk: (terminal.Session | null)[] = [];
      for (let j = 0; j < cellsPerGroup; j++) {
        const idx = i + j;
        if (idx < sessions.length) {
          chunk.push(sessions[idx]);
        } else if (!hideEmptySlots && idx < fillCount) {
          chunk.push(null);
        }
      }
      if (chunk.length > 0) {
        chunks.push(chunk);
      }
    }
    return chunks;
  }, [layoutMode, sessions, cellsPerGroup, hideEmptySlots]);

  // ---------- Filter / Layout mode buttons component ----------
  const filterBar = (
    <div className="flex items-center gap-0.5">
      <button
        className={cn(
          "p-1.5 rounded transition-colors cursor-pointer",
          layoutMode === "single"
            ? "bg-accent text-foreground"
            : "text-muted-foreground hover:text-foreground hover:bg-accent/50"
        )}
        onClick={() => {
          setLayoutMode("single");
          setHideEmptySlots(false);
        }}
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
        onClick={() => {
          setLayoutMode("horizontal");
          setHideEmptySlots(false);
        }}
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
        onClick={() => {
          setLayoutMode("grid4");
          setHideEmptySlots(false);
        }}
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
        <div className="flex items-center justify-between px-3 py-1 border-b bg-muted/20 shrink-0">
          <div className="flex items-center gap-1">
            <button
              className="flex items-center gap-1 px-2 py-1 text-xs rounded bg-accent/40 hover:bg-accent text-foreground transition-colors cursor-pointer"
              onClick={() => {
                setHideEmptySlots(false);
                onCreateShell();
              }}
              title="New Shell"
            >
              <Plus className="size-3.5" />
              <span className="hidden sm:inline">New Shell</span>
            </button>
          </div>
          {filterBar}
        </div>
        <div className="flex-1 flex items-center justify-center">
          <div className="text-center">
            <Shell className="size-10 mx-auto mb-3 opacity-30" />
            <p className="text-sm text-muted-foreground mb-3">No sessions open</p>
            <button
              className="inline-flex items-center gap-1.5 px-4 py-2 text-xs font-medium bg-foreground/10 hover:bg-foreground/20 rounded transition-colors cursor-pointer"
              onClick={() => {
                setHideEmptySlots(false);
                onCreateShell();
              }}
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
      <div className="flex flex-col h-full min-h-0 min-w-0">
        <div className="flex items-center justify-between px-3 py-0 border-b bg-muted/20 shrink-0">
          {/* Session selector tabs */}
          <div className="flex items-center overflow-x-auto">
            {sessions.map((s) => (
              <div
                key={s.id}
                className={cn(
                  "flex items-center gap-1 px-3 py-1.5 text-xs border-r cursor-pointer whitespace-nowrap shrink-0 select-none group",
                  activeSession?.id === s.id
                    ? "bg-background border-b-2 border-b-cyan-500 text-foreground font-medium"
                    : "text-muted-foreground hover:bg-accent/50"
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
                    "w-1.5 h-1.5 rounded-full shrink-0",
                    s.status === "running" ? "bg-green-500" : "bg-muted-foreground"
                  )}
                />
                <button
                  className="p-0.5 hover:bg-accent rounded shrink-0 ml-1 opacity-0 group-hover:opacity-100 transition-opacity cursor-pointer"
                  onClick={(e) => {
                    e.stopPropagation();
                    onCloseSession(s.id);
                  }}
                  title="Close view (session remains running)"
                >
                  <X className="size-2.5" />
                </button>
              </div>
            ))}
            <button
              className="flex items-center justify-center px-2 py-1.5 text-xs text-muted-foreground hover:text-foreground hover:bg-accent/50 border-r cursor-pointer transition-colors shrink-0"
              onClick={() => {
                setHideEmptySlots(false);
                onCreateShell();
              }}
              title="New Shell"
            >
              <Plus className="size-3.5" />
            </button>
          </div>
          {filterBar}
        </div>
        <div className="flex-1 overflow-hidden relative h-full w-full min-h-0 min-w-0">
          {sessions.map((s) => (
            <div
              key={s.id}
              className={cn("h-full w-full flex flex-col min-h-0 min-w-0", s.id === activeSession?.id ? "flex" : "hidden")}
            >
              <TerminalView sessionId={s.id} isActive={s.id === activeSession?.id} />
            </div>
          ))}
        </div>
      </div>
    );
  }

  // ---------- Horizontal mode (up to 3 side by side, full height, horizontally scrollable) ----------
  if (layoutMode === "horizontal") {
    return (
      <div className="flex flex-col h-full min-h-0 min-w-0">
        <div className="flex items-center justify-between px-3 py-1 border-b bg-muted/20 shrink-0">
          <div className="flex items-center gap-1">
            <button
              className="flex items-center gap-1 px-2 py-1 text-xs rounded bg-accent/40 hover:bg-accent text-foreground transition-colors cursor-pointer"
              onClick={() => {
                setHideEmptySlots(false);
                onCreateShell();
              }}
              title="New Shell"
            >
              <Plus className="size-3.5" />
              <span className="hidden sm:inline">New Shell</span>
            </button>
          </div>
          {filterBar}
        </div>
        <div className="flex-1 min-h-0 min-w-0 flex overflow-x-auto">
          {chunked.map((group, gi) => (
            <div key={gi} className="flex flex-shrink-0 h-full w-full min-h-0 min-w-0" style={{ minWidth: "100%" }}>
              {group.map((s, ci) => (
                <div key={s ? s.id : `empty-${gi}-${ci}`} className="flex-1 min-w-[300px] h-full flex flex-col min-h-0 min-w-0">
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
    <div className="flex flex-col h-full min-h-0 min-w-0">
      <div className="flex items-center justify-between px-3 py-1 border-b bg-muted/20 shrink-0">
        <div className="flex items-center gap-1">
          <button
            className="flex items-center gap-1 px-2 py-1 text-xs rounded bg-accent/40 hover:bg-accent text-foreground transition-colors cursor-pointer"
            onClick={() => {
              setHideEmptySlots(false);
              onCreateShell();
            }}
            title="New Shell"
          >
            <Plus className="size-3.5" />
            <span className="hidden sm:inline">New Shell</span>
          </button>
        </div>
        {filterBar}
      </div>
      <div className="flex-1 min-h-0 min-w-0 flex overflow-x-auto">
        {chunked.map((page, pi) => (
          <div
            key={pi}
            className="grid grid-cols-2 grid-rows-2 h-full flex-shrink-0 w-full min-h-0 min-w-0"
            style={{ minWidth: "100%" }}
          >
            {page.map((s, ci) => renderCell(s, `g-${pi}-${ci}`))}
          </div>
        ))}
      </div>
    </div>
  );
}



