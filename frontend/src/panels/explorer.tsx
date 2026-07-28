import { useState, useEffect } from "react";
import {
  ChevronRight,
  ChevronDown,
  File,
  Folder,
  FolderOpen,
  Plus,
  Trash2,
  FilePenLine,
} from "lucide-react";
import { ScrollArea } from "../components/ui/scroll-area";
import {
  GetFileTree,
  ExpandPath,
  CreateFile,
  DeleteFile,
} from "../../wailsjs/go/main/App";
import { globalOpenFile } from "../panels/editor";
import type { FileInfo } from "../types";

interface ExplorerProps {
  roots: string[];
  onRefresh?: () => void;
}

export function Explorer({ roots, onRefresh }: ExplorerProps) {
  const [refreshKey, setRefreshKey] = useState(0);

  const triggerRefresh = () => {
    setRefreshKey((k) => k + 1);
    onRefresh?.();
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
          />
        ))}
      </div>
    </ScrollArea>
  );
}

function FolderTree({
  rootPath,
  onRefresh,
}: {
  rootPath: string;
  onRefresh: () => void;
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
}: {
  node: FileInfo;
  depth: number;
  onRefresh: () => void;
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

  const handleCreateFile = async () => {
    const name = prompt("File name:");
    if (!name) return;
    const dir = node.isDir ? node.path : node.path.split("/").slice(0, -1).join("/");
    try {
      await CreateFile(dir + "/" + name);
      onRefresh();
      // Re-expand
      if (node.isDir && !expanded) {
        const result = await ExpandPath(node.path);
        setChildren(JSON.parse(result));
        setExpanded(true);
      }
    } catch (err) {
      console.error(err);
    }
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
        className="flex items-center gap-1 px-2 py-0.5 text-sm cursor-pointer rounded hover:bg-accent group relative"
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
                  const name = prompt("Folder name:");
                  if (name) {
                    CreateFile(node.path + "/" + name).then(onRefresh).catch(console.error);
                  }
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
            />
          ))}
        </div>
      )}
    </div>
  );
}
