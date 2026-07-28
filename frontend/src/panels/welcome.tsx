import { FolderOpen, FileText, Pin, X } from "lucide-react";
import { Button } from "../components/ui/button";
import { ScrollArea } from "../components/ui/scroll-area";
import type { RecentEntry } from "../types";

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
  const pinned = recentProjects.filter((e) => e.pinned);
  const recent = recentProjects.filter((e) => !e.pinned).slice(0, 10);

  return (
    <div className="flex flex-col items-center justify-center h-full select-none">
      <div className="max-w-lg w-full px-8">
        {/* Logo / Brand */}
        <div className="text-center mb-10">
          <div className="inline-flex items-center justify-center w-16 h-16 rounded-2xl mb-4 overflow-hidden">
            <img src="/logo.png" className="w-full h-full object-cover" alt="ForgeADE" />
          </div>
          <h1 className="text-2xl font-semibold tracking-tight">ForgeADE</h1>
          <p className="text-sm text-muted-foreground mt-1">
            Native AI Development Workspace
          </p>
        </div>

        {/* Actions */}
        <div className="grid grid-cols-2 gap-3 mb-8">
          <Button
            variant="outline"
            className="h-20 flex-col gap-1"
            onClick={onOpenFolder}
          >
            <FolderOpen className="size-5" />
            <span className="text-xs">Open Folder</span>
          </Button>
          <Button
            variant="outline"
            className="h-20 flex-col gap-1"
            onClick={onOpenWorkspace}
          >
            <FileText className="size-5" />
            <span className="text-xs">Open Workspace</span>
          </Button>
        </div>

        {/* Recent Projects */}
        {recentProjects.length > 0 && (
          <div>
            <h2 className="text-xs font-medium text-muted-foreground uppercase tracking-wider mb-2">
              Recent
            </h2>
            <ScrollArea className="max-h-64">
              <div className="space-y-1">
                {pinned.map((entry) => (
                  <RecentRow
                    key={entry.path}
                    entry={entry}
                    onOpen={onOpenRecent}
                    onPin={onPinRecent}
                    onRemove={onRemoveRecent}
                  />
                ))}
                {recent.map((entry) => (
                  <RecentRow
                    key={entry.path}
                    entry={entry}
                    onOpen={onOpenRecent}
                    onPin={onPinRecent}
                    onRemove={onRemoveRecent}
                  />
                ))}
              </div>
            </ScrollArea>
          </div>
        )}
      </div>
    </div>
  );
}

function RecentRow({
  entry,
  onOpen,
  onPin,
  onRemove,
}: {
  entry: RecentEntry;
  onOpen: (e: RecentEntry) => void;
  onPin: (path: string, pinned: boolean) => void;
  onRemove: (path: string) => void;
}) {
  return (
    <div
      className="group flex items-center gap-3 px-3 py-2 rounded-md cursor-pointer hover:bg-accent text-sm"
      onClick={() => onOpen(entry)}
    >
      {entry.isWorkspace ? (
        <FileText className="size-4 shrink-0 text-muted-foreground" />
      ) : (
        <FolderOpen className="size-4 shrink-0 text-muted-foreground" />
      )}
      <div className="flex-1 min-w-0">
        <div className="truncate font-medium">{entry.name}</div>
        <div className="truncate text-xs text-muted-foreground">{entry.path}</div>
      </div>
      <div className="flex gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
        <button
          className="p-1 hover:text-foreground text-muted-foreground"
          onClick={(e) => {
            e.stopPropagation();
            onPin(entry.path, !entry.pinned);
          }}
        >
          <Pin className={`size-3.5 ${entry.pinned ? "fill-current" : ""}`} />
        </button>
        <button
          className="p-1 hover:text-foreground text-muted-foreground"
          onClick={(e) => {
            e.stopPropagation();
            onRemove(entry.path);
          }}
        >
          <X className="size-3.5" />
        </button>
      </div>
    </div>
  );
}
