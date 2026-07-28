import { useState, useEffect } from "react";
import {
  ChevronRight,
  ChevronDown,
  File,
  Folder,
  FolderOpen,
  Plus,
  Trash2,
} from "lucide-react";
import { ScrollArea } from "../components/ui/scroll-area";
import { SimpleModal } from "../components/simple-modal";
import { cn } from "../lib/utils";
import { globalOpenFile } from "../panels/editor";
import { EventsOn } from "../../wailsjs/runtime";
import {
  GetFileTree,
  ExpandPath,
  CreateFile,
  DeleteFile,
  GetFsChangeCount,
} from "../../wailsjs/go/main/App";
import type { FileInfo } from "../types";

interface ExplorerProps {
  roots: string[];
  onRefresh?: () => void;
}

export function Explorer({ roots, onRefresh }: ExplorerProps) {
  const [refreshKey, setRefreshKey] = useState(0);
  const [modal, setModal] = useState<{
    type: "createFile" | "createFolder" | "delete" | "rename";
    dir: string;
    oldName?: string;
    oldPath?: string;
  } | null>(null);

  // Listen for filesystem changes (Wails events)
  useEffect(() => {
    const dispose = EventsOn("fs:changed", () => {
      setRefreshKey((k) => k + 1);
      onRefresh?.();
    });
    return () => { if (dispose) dispose(); };
  }, [onRefresh]);

  // Poll fallback: check counter every second
  useEffect(() => {
    let lastCount = 0;
    GetFsChangeCount().then((c) => { lastCount = c; });
    const interval = setInterval(async () => {
      try {
        const count = await GetFsChangeCount();
        if (count !== lastCount) {
          lastCount = count;
          setRefreshKey((k) => k + 1);
          onRefresh?.();
        }
      } catch { /* ignore */ }
    }, 1000);
    return () => clearInterval(interval);
  }, [onRefresh]);

  const triggerRefresh = () => {
    setRefreshKey((k) => k + 1);
    onRefresh?.();
  };

  const handleModalSubmit = async (value: string) => {
    if (!modal) return;
    try {
      if (modal.type === "createFile") {
        await CreateFile(modal.dir + "/" + value);
      } else if (modal.type === "createFolder") {
        await CreateFile(modal.dir + "/" + value);
      }
      triggerRefresh();
    } catch (err) {
      console.error(err);
    }
    setModal(null);
  };

  if (roots.length === 0) {
    return (
      <div className="flex items-center justify-center h-full text-sm text-muted-foreground">
        No folders open
      </div>
    );
  }

  return (
    <ScrollArea className="h-full">
      <div className="py-1">
        {roots.map((root) => (
          <FolderTree
            key={root + refreshKey}
            rootPath={root}
            onRefresh={triggerRefresh}
            onShowModal={setModal}
          />
        ))}
      </div>

      <SimpleModal
        open={modal !== null}
        title={modal?.type === "createFile" ? "Create File" : modal?.type === "createFolder" ? "Create Folder" : ""}
        placeholder={modal?.type === "createFile" ? "filename.ts" : "folder-name"}
        onClose={() => setModal(null)}
        onSubmit={handleModalSubmit}
        submitLabel="Create"
      />
    </ScrollArea>
  );
}

function FolderTree({
  rootPath,
  onRefresh,
  onShowModal,
}: {
  rootPath: string;
  onRefresh: () => void;
  onShowModal: (m: { type: "createFile" | "createFolder"; dir: string } | null) => void;
}) {
  const [tree, setTree] = useState<FileInfo | null>(null);

  useEffect(() => {
    async function load() {
      try {
        const result = await GetFileTree(2);
        const parsed: FileInfo[] = JSON.parse(result);
        const root = parsed.find(
          (f) => f.path === rootPath || f.name === rootPath.split("/").pop()
        );
        setTree(root || null);
      } catch {
        // fallback
      }
    }
    load();
  }, [rootPath]);

  if (!tree) return null;

  return (
    <div>
      <div className="flex items-center gap-1 px-2 py-1 text-xs font-medium text-muted-foreground uppercase tracking-wider">
        <FolderOpen className="size-3.5" />
        <span>{tree.name}</span>
      </div>
      <div className="ml-2">
        {tree.children?.map((child) => (
          <FileNode
            key={child.path}
            node={child}
            depth={1}
            onRefresh={onRefresh}
            onShowModal={onShowModal}
          />
        ))}
      </div>
    </div>
  );
}

function FileNode({
  node,
  depth,
  onRefresh,
  onShowModal,
}: {
  node: FileInfo;
  depth: number;
  onRefresh: () => void;
  onShowModal: (m: { type: "createFile" | "createFolder"; dir: string } | null) => void;
}) {
  const [expanded, setExpanded] = useState(false);
  const [children, setChildren] = useState<FileInfo[] | undefined>(
    node.children
  );
  const [showMenu, setShowMenu] = useState(false);

  const handleToggle = async () => {
    if (node.isDir) {
      if (!expanded && !children) {
        try {
          const result = await ExpandPath(node.path);
          const parsed: FileInfo[] = JSON.parse(result);
          setChildren(parsed);
        } catch {
          // ignore
        }
      }
      setExpanded(!expanded);
    } else {
      // Open file in editor
      globalOpenFile(node.path);
    }
  };

  const handleContextMenu = (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setShowMenu(true);
    // Auto-hide after click outside
    setTimeout(() => setShowMenu(false), 3000);
  };

  const handleCreateFile = () => {
    const dir = node.isDir ? node.path : node.path.split("/").slice(0, -1).join("/");
    onShowModal({ type: "createFile", dir });
    setShowMenu(false);
  };

  const handleDelete = async () => {
    if (!confirm(`Delete "${node.name}"?`)) return;
    try {
      await DeleteFile(node.path);
      onRefresh();
    } catch (err) {
      console.error(err);
    }
    setShowMenu(false);
  };

  if (node.hidden) return null;

  return (
    <div>
      <div
        className={cn(
          "flex items-center gap-1 px-2 py-0.5 text-sm cursor-pointer rounded hover:bg-accent group relative",
          node.gitIgnored && "opacity-40"
        )}
        style={{ paddingLeft: `${depth * 16 + 8}px` }}
        onClick={handleToggle}
        onContextMenu={handleContextMenu}
      >
        {node.isDir ? (
          <>
            {expanded ? (
              <ChevronDown className="size-3.5 shrink-0 text-muted-foreground" />
            ) : (
              <ChevronRight className="size-3.5 shrink-0 text-muted-foreground" />
            )}
            {expanded ? (
              <FolderOpen className="size-4 shrink-0 text-amber-500" />
            ) : (
              <Folder className="size-4 shrink-0 text-amber-500" />
            )}
          </>
        ) : (
          <>
            <span className="w-3.5" />
            <File className="size-4 shrink-0 text-blue-400" />
          </>
        )}
        <span className="truncate flex-1">{node.name}</span>

        {/* Quick actions on hover */}
        <div className="hidden group-hover:flex items-center gap-0.5">
          {node.isDir && (
            <button
              className="p-0.5 hover:bg-accent rounded"
              onClick={(e) => {
                e.stopPropagation();
                handleCreateFile();
              }}
              title="New File"
            >
              <Plus className="size-3" />
            </button>
          )}
          <button
            className="p-0.5 hover:bg-accent rounded"
            onClick={(e) => {
              e.stopPropagation();
              handleDelete();
            }}
            title="Delete"
          >
            <Trash2 className="size-3 text-red-400" />
          </button>
        </div>

        {/* Context menu */}
        {showMenu && (
          <div
            className="absolute left-full top-0 z-50 bg-popover border rounded shadow-md py-1 min-w-32"
            onClick={(e) => e.stopPropagation()}
          >
            <button
              className="flex items-center gap-2 px-3 py-1.5 text-xs hover:bg-accent w-full text-left"
              onClick={handleCreateFile}
            >
              <Plus className="size-3" /> New File
            </button>
            {node.isDir && (
              <button
                className="flex items-center gap-2 px-3 py-1.5 text-xs hover:bg-accent w-full text-left"
                onClick={() => {
                  onShowModal({ type: "createFolder", dir: node.path });
                  setShowMenu(false);
                }}
              >
                <Plus className="size-3" /> New Folder
              </button>
            )}
            <div className="border-t my-1" />
            <button
              className="flex items-center gap-2 px-3 py-1.5 text-xs hover:bg-accent w-full text-left text-red-400"
              onClick={handleDelete}
            >
              <Trash2 className="size-3" /> Delete
            </button>
          </div>
        )}
      </div>
      {expanded && children && (
        <div>
          {children.map((child) => (
            <FileNode
              key={child.path}
              node={child}
              depth={depth + 1}
              onRefresh={onRefresh}
              onShowModal={onShowModal}
            />
          ))}
        </div>
      )}
    </div>
  );
}
