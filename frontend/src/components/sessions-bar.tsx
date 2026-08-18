import React, { useEffect, useState, useMemo } from "react";
import {
  IconTerminal2,
  IconGitBranch,
  IconGraph,
  IconServer,
  IconCheck,
  IconAlertTriangle,
  IconX,
} from "@tabler/icons-react";
import { EventsOn, GetGitStatus } from "../lib/wails";
import { useLSPStore } from "../lib/lsp-store";
import { useEditorStore } from "../hooks/store";
import { getLanguageMeta } from "../lib/languages";

interface SessionsBarProps {
  onSelectSession: (id: string | null) => void;
  cwd: string;
  onCreateShell: () => void;
  onOpenGitGraph?: () => void;
  onOpenLSPModal?: (pos?: { x: number; y: number }) => void;
}
export function SessionsBar({
  onSelectSession,
  cwd,
  onCreateShell,
  onOpenGitGraph,
  onOpenLSPModal,
}: SessionsBarProps) {
  const [branch, setBranch] = useState("");
  const { servers, diagnostics } = useLSPStore();
  const { files, activeFileIndex } = useEditorStore();
  const activeFile = activeFileIndex >= 0 && activeFileIndex < files.length ? files[activeFileIndex] : null;

  const totalErrors = useMemo(() => {
    let count = 0;
    for (const d of Object.values(diagnostics)) {
      count += d.errors || 0;
    }
    return count;
  }, [diagnostics]);

  const totalWarnings = useMemo(() => {
    let count = 0;
    for (const d of Object.values(diagnostics)) {
      count += d.warnings || 0;
    }
    return count;
  }, [diagnostics]);

  const activeLangMeta = useMemo(() => {
    if (!activeFile || activeFile.type !== "file") return null;
    return getLanguageMeta(activeFile.path);
  }, [activeFile]);

  const activeServerForFile = useMemo(() => {
    if (!activeLangMeta) return null;
    return servers.find((s) => s.languageId === activeLangMeta.id);
  }, [servers, activeLangMeta]);

  const runningServersCount = useMemo(() => {
    return servers.filter((s) => s.status === "running").length;
  }, [servers]);
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
    // No polling: refresh the branch badge only when files change (debounced).
    // GetGitStatus is TTL-cached, so this spawns `git status` at most ~1x/5s.
    const unsubscribe = EventsOn("fs:changed", () => {
      if (timer) clearTimeout(timer);
      timer = setTimeout(loadBranch, 500);
    });
    return () => {
      cancelled = true;
      if (timer) clearTimeout(timer);
      if (typeof unsubscribe === "function") unsubscribe();
    };
  }, [cwd]);

  return (
    <div className="h-6 px-3 bg-[var(--bg-sidebar)] border-t border-[var(--border-default)] flex items-center justify-between text-[10px] text-[var(--fg-tertiary)] shrink-0 select-none font-sans">
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

      <div className="flex items-center space-x-3">
        {/* Language Server Status Indicator */}
        <button
          onClick={(e) => {
            const rect = e.currentTarget.getBoundingClientRect();
            onOpenLSPModal?.({ x: rect.left, y: rect.top });
          }}
          className="flex items-center gap-1.5 px-2 py-0.5 rounded hover:bg-[var(--bg-surface-hover)] hover:text-[var(--fg-primary)] cursor-pointer transition-colors"
          title="Manage Language Servers & Diagnostics"
        >
          <IconServer className={`size-3 ${runningServersCount > 0 ? "text-cyan-400" : "text-[var(--fg-tertiary)]"}`} />
          <span className="font-mono">
            {activeServerForFile
              ? `${activeServerForFile.name} (${activeServerForFile.status})`
              : activeLangMeta
              ? `${activeLangMeta.name} LSP`
              : runningServersCount > 0
              ? `${runningServersCount} LSP active`
              : "LSP: Ready"}
          </span>

          {/* Diagnostics badge in footer */}
          {totalErrors > 0 ? (
            <span className="flex items-center gap-0.5 text-red-400 font-bold ml-1">
              <span className="inline-block size-1.5 rounded-full bg-red-400 animate-pulse" />
              {totalErrors}
            </span>
          ) : totalWarnings > 0 ? (
            <span className="flex items-center gap-0.5 text-amber-400 font-bold ml-1">
              <span className="inline-block size-1.5 rounded-full bg-amber-400" />
              {totalWarnings}
            </span>
          ) : runningServersCount > 0 ? (
            <span className="flex items-center text-emerald-400 ml-1">
              <span className="inline-block size-1.5 rounded-full bg-emerald-400" />
            </span>
          ) : null}
        </button>

        <button
          onClick={onOpenGitGraph}
          className="flex items-center gap-1 hover:text-[var(--fg-primary)] cursor-pointer"
          title="Open Git Graph"
        >
          <IconGraph className="size-3 text-purple-400" />
          <span>Git Graph</span>
        </button>
        <button
          onClick={onCreateShell}
          className="flex items-center gap-1 hover:text-[var(--fg-primary)] cursor-pointer"
        >
          <IconTerminal2 className="size-3" />
          <span>New Shell</span>
        </button>
      </div>
    </div>
  );
}
