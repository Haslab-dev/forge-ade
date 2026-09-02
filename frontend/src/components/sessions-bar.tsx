import React, { useEffect, useState, useCallback } from "react";
import {
  IconTerminal2,
  IconGitBranch,
  IconChevronUp,
  IconChevronDown,
  IconPlus,
  IconX,
} from "@tabler/icons-react";
import { EventsOn, GetGitStatus, ListSessions, CreateShell } from "../lib/wails";
import { TerminalView } from "./terminal-view";
import { cn } from "../lib/utils";

interface SessionsBarProps {
  onSelectSession: (id: string | null) => void;
  cwd: string;
  onCreateShell: () => void;
  mode?: "agent" | "editor" | "git-graph";
}

export function SessionsBar({
  onSelectSession,
  cwd,
  onCreateShell,
  mode = "editor",
}: SessionsBarProps) {
  const [branch, setBranch] = useState("");
  const [isDrawerOpen, setIsDrawerOpen] = useState(false);
  const [shellSessions, setShellSessions] = useState<any[]>([]);
  const [activeShellId, setActiveShellId] = useState<string | null>(null);

  const loadShells = useCallback(async () => {
    try {
      const shells = await ListSessions();
      const list = Array.isArray(shells) ? shells : [];
      setShellSessions(list);
      if (list.length > 0) {
        setActiveShellId((prev) => (prev && list.some((s: any) => s.id === prev) ? prev : list[0].id));
      } else {
        setActiveShellId(null);
      }
    } catch {
      setShellSessions([]);
    }
  }, []);

  useEffect(() => {
    let cancelled = false;
    let timer: ReturnType<typeof setTimeout> | undefined;
    async function loadBranch() {
      try {
        const st = await GetGitStatus("");
        if (!cancelled && st && st.branch) {
          setBranch(st.branch);
        } else if (!cancelled && st && !st.branch) {
          setBranch("");
        }
      } catch { /* not a git repo */ }
    }
    loadBranch();
    loadShells();

    const unsubscribe = EventsOn("fs:changed", () => {
      if (timer) clearTimeout(timer);
      timer = setTimeout(loadBranch, 500);
    });

    const unsubs = [
      EventsOn("session:opened", () => loadShells()),
      EventsOn("session:closed", () => loadShells()),
    ];

    return () => {
      cancelled = true;
      if (timer) clearTimeout(timer);
      if (typeof unsubscribe === "function") unsubscribe();
      unsubs.forEach((u) => typeof u === "function" && u());
    };
  }, [cwd, loadShells]);

  const handleCreateNewShell = async () => {
    try {
      const name = `Terminal ${shellSessions.length + 1}`;
      const res = await CreateShell(name, cwd);
      await loadShells();
      if (res && res.id) {
        setActiveShellId(res.id);
        setIsDrawerOpen(true);
      }
    } catch (err) {
      console.error("Failed to create shell:", err);
    }
  };

  return (
    <div className="flex flex-col shrink-0 select-none font-sans border-t border-[var(--border-default)]">
      {/* Expandable Bottom Active Terminal Panel in Editor Mode */}
      {isDrawerOpen && mode === "editor" && (
        <div className="h-48 md:h-56 bg-[var(--terminal-background)] flex flex-col border-b border-[var(--border-default)] overflow-hidden">
          {/* Terminal Tabs Header */}
          <div className="h-7 bg-[var(--bg-sidebar)] border-b border-[var(--border-default)] flex items-center justify-between px-3 text-xs shrink-0">
            <div className="flex items-center gap-1.5 overflow-x-auto min-w-0 pr-1">
              <IconTerminal2 className="size-3.5 text-cyan-400 mr-1 shrink-0" />
              {shellSessions.map((shell) => {
                const active = shell.id === activeShellId;
                return (
                  <button
                    key={shell.id}
                    onClick={() => setActiveShellId(shell.id)}
                    className={cn(
                      "px-2 py-0.5 rounded text-[11px] font-mono transition-colors truncate max-w-28 cursor-pointer",
                      active
                        ? "bg-[var(--bg-app)] text-[var(--fg-primary)] border border-[var(--border-default)] font-semibold"
                        : "text-[var(--fg-tertiary)] hover:text-[var(--fg-primary)]"
                    )}
                  >
                    {shell.name || "Shell"}
                  </button>
                );
              })}
              <button
                onClick={handleCreateNewShell}
                className="p-1 hover:bg-[var(--bg-panel)] rounded text-[var(--fg-tertiary)] hover:text-white cursor-pointer shrink-0"
                title="New Terminal Shell (+)"
              >
                <IconPlus className="size-3" />
              </button>
            </div>

            <button
              onClick={() => setIsDrawerOpen(false)}
              className="p-1 text-[var(--fg-tertiary)] hover:text-white rounded cursor-pointer"
              title="Close terminal drawer"
            >
              <IconX className="size-3.5" />
            </button>
          </div>

          {/* Terminal Viewport */}
          <div className="flex-1 min-h-0 min-w-0 overflow-hidden relative">
            {shellSessions.length > 0 && activeShellId ? (
              shellSessions.map((shell) => (
                <div
                  key={shell.id}
                  className={cn(
                    "absolute inset-0 overflow-hidden",
                    shell.id === activeShellId ? "block z-10" : "hidden"
                  )}
                >
                  <TerminalView sessionId={shell.id} isActive={shell.id === activeShellId} />
                </div>
              ))
            ) : (
              <div className="flex flex-col items-center justify-center h-full p-4 text-center text-xs text-[var(--fg-tertiary)]">
                <p>No active terminal shells</p>
                <button
                  onClick={handleCreateNewShell}
                  className="mt-1.5 px-2.5 py-1 bg-[var(--bg-panel)] border border-[var(--border-default)] hover:bg-[var(--bg-surface-hover)] text-[var(--fg-primary)] rounded cursor-pointer text-xs"
                >
                  Launch Shell
                </button>
              </div>
            )}
          </div>
        </div>
      )}

      {/* Global Status Bar Strip */}
      <div className="h-6 px-3 bg-[var(--bg-sidebar)] flex items-center justify-between text-[10px] text-[var(--fg-tertiary)] shrink-0">
        <div className="flex items-center space-x-3 min-w-0">
          <span className="font-semibold text-[var(--fg-secondary)] truncate">Workspace: {cwd || "No Project"}</span>
          {branch && (
            <span className="flex items-center gap-1 font-mono text-[var(--fg-secondary)]">
              <IconGitBranch className="size-3 text-purple-400" />
              <span className="text-[var(--fg-primary)]">{branch}</span>
              <span className="text-[var(--fg-tertiary)]">· active branch</span>
            </span>
          )}
        </div>

        <div className="flex items-center space-x-2">
          {mode === "editor" ? (
            <button
              onClick={() => setIsDrawerOpen((prev) => !prev)}
              className={cn(
                "flex items-center gap-1.5 px-2 py-0.5 rounded cursor-pointer transition-colors font-medium",
                isDrawerOpen
                  ? "bg-[var(--bg-surface-active)] text-[var(--fg-primary)]"
                  : "hover:text-[var(--fg-primary)] text-[var(--fg-secondary)] hover:bg-[var(--bg-panel)]"
              )}
              title="Toggle Bottom Terminal Shell Panel"
            >
              <IconTerminal2 className="size-3 text-cyan-400" />
              <span>Terminal ({shellSessions.length})</span>
              {isDrawerOpen ? <IconChevronDown className="size-3" /> : <IconChevronUp className="size-3" />}
            </button>
          ) : (
            <button
              onClick={onCreateShell}
              className="flex items-center gap-1 hover:text-[var(--fg-primary)] cursor-pointer"
            >
              <IconTerminal2 className="size-3" />
              <span>New Shell</span>
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
