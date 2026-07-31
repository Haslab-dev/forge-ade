import React from "react";
import { IconRobot } from "@tabler/icons-react";

export function AgentScreen() {
  return (
    <div className="flex flex-col items-center justify-center h-full text-center p-6 text-[var(--fg-tertiary)] bg-[var(--bg-app)]">
      <IconRobot className="size-12 animate-pulse text-[var(--accent-primary)] mb-2" />
      <h3 className="text-sm font-semibold text-[var(--fg-secondary)]">Agent Workspace Panel</h3>
      <p className="text-xs max-w-xs mt-1">
        This session is docked in the bottom panel. Open the terminal panel (Ctrl+`) to access active agent chats.
      </p>
    </div>
  );
}
