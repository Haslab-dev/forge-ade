import React from "react";
import { IconTerminal2 } from "@tabler/icons-react";

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
  return (
    <div className="h-6 px-3 bg-[var(--bg-sidebar)] border-t border-[var(--border-default)] flex items-center justify-between text-[10px] text-[var(--fg-tertiary)] shrink-0 select-none font-sans">
      <div className="flex items-center space-x-3">
        <span className="font-semibold text-[var(--fg-secondary)]">Workspace: {cwd || "No Project"}</span>
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
