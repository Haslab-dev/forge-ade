import React, { useEffect, useState } from "react";
import { IconTerminal2, IconGitBranch } from "@tabler/icons-react";
import { GetGitStatus } from "../lib/wails";

interface SessionsBarProps {
  onSelectSession: (id: string | null) => void;
  cwd: string;
  onCreateShell: () => void;
}

export function SessionsBar({
  onSelectSession,
  cwd,
  onCreateShell,
}: SessionsBarProps) {
  const [branch, setBranch] = useState("");

  useEffect(() => {
    let cancelled = false;
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
    const interval = setInterval(loadBranch, 5000);
    return () => {
      cancelled = true;
      clearInterval(interval);
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

      <div className="flex items-center space-x-2">
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
