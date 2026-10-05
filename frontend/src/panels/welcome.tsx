import React from "react";
import { RecentEntry } from "../types";
import { FolderOpen, FileText, Pin, Trash2, Code2 } from "lucide-react";
import { APP_VERSION } from "../lib/utils";

interface WelcomeProps {
  recentProjects: RecentEntry[];
  onOpenFolder: () => void;
  onOpenWorkspace: () => void;
  onOpenRecent: (entry: RecentEntry) => void;
  onPinRecent: (path: string, pinned: boolean) => void;
  onRemoveRecent: (path: string) => void;
}

export function Welcome({
  recentProjects,
  onOpenFolder,
  onOpenWorkspace,
  onOpenRecent,
  onPinRecent,
  onRemoveRecent,
}: WelcomeProps) {
  return (
    <div className="flex flex-col items-center justify-center h-full max-w-xl mx-auto px-6 py-12 select-none text-[var(--fg-secondary)] font-sans">
      <div className="flex flex-col items-center space-y-1.5 mb-8">
        <div className="flex items-center space-x-2.5">
          <Code2 className="size-8 text-[var(--accent-primary)] animate-pulse" />
          <span className="text-xl font-bold tracking-tight text-[var(--fg-primary)]">ForgeADE</span>
        </div>
        <span className="text-[10px] font-mono text-[var(--fg-tertiary)] bg-black/30 border border-[var(--border-default)] px-1.5 py-0.5 rounded">
          v{APP_VERSION}
        </span>
      </div>

      <div className="w-full space-y-4">
        {/* Core Actions */}
        <div className="grid grid-cols-2 gap-3">
          <button
            onClick={onOpenFolder}
            className="flex flex-col items-center justify-center p-5 bg-[var(--bg-sidebar)] border border-[var(--border-default)] hover:bg-[var(--bg-surface-hover)] hover:border-[var(--accent-primary)] text-center transition-all group cursor-pointer"
          >
            <FolderOpen className="size-6 text-amber-400 group-hover:scale-105 transition-transform mb-2" />
            <span className="text-xs font-semibold text-[var(--fg-primary)]">Open Project Folder</span>
          </button>

          <button
            onClick={onOpenWorkspace}
            className="flex flex-col items-center justify-center p-5 bg-[var(--bg-sidebar)] border border-[var(--border-default)] hover:bg-[var(--bg-surface-hover)] hover:border-[var(--accent-primary)] text-center transition-all group cursor-pointer"
          >
            <FileText className="size-6 text-blue-400 group-hover:scale-105 transition-transform mb-2" />
            <span className="text-xs font-semibold text-[var(--fg-primary)]">Open Workspace File</span>
          </button>
        </div>

        {/* Recent projects list */}
        <div className="bg-[var(--bg-sidebar)] border border-[var(--border-default)] p-4 flex flex-col space-y-3">
          <h3 className="text-xs font-bold uppercase tracking-wider text-[var(--fg-tertiary)]">Recent Projects</h3>

          {recentProjects.length === 0 ? (
            <div className="text-xs italic text-[var(--fg-tertiary)] py-4 text-center">
              No recent projects opened yet.
            </div>
          ) : (
            <div className="max-h-60 overflow-y-auto space-y-1">
              {recentProjects.map((entry) => (
                <div
                  key={entry.path}
                  className="flex items-center justify-between p-2 hover:bg-[var(--bg-panel)] rounded group transition-colors"
                >
                  <div
                    onClick={() => onOpenRecent(entry)}
                    className="flex-1 min-w-0 cursor-pointer"
                  >
                    <div className="text-xs font-semibold text-[var(--fg-primary)] truncate">
                      {entry.name}
                    </div>
                    <div className="text-[10px] text-[var(--fg-tertiary)] truncate">
                      {entry.path}
                    </div>
                  </div>

                  <div className="flex items-center space-x-1.5 opacity-0 group-hover:opacity-100 transition-opacity">
                    <button
                      onClick={() => onPinRecent(entry.path, !entry.pinned)}
                      className={`p-1 hover:bg-[var(--bg-surface-hover)] rounded ${
                        entry.pinned ? "text-amber-400" : "text-[var(--fg-tertiary)]"
                      }`}
                    >
                      <Pin className="size-3.5" />
                    </button>
                    <button
                      onClick={() => onRemoveRecent(entry.path)}
                      className="p-1 hover:bg-red-950/40 text-[var(--fg-tertiary)] hover:text-red-400 rounded"
                    >
                      <Trash2 className="size-3.5" />
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
