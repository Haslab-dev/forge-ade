import { useState, useEffect, useCallback, useRef } from "react";
import {
  ChevronRight,
  ChevronDown,
  File,
  Folder,
  FolderOpen,
  Plus,
  Trash2,
  Copy,
  ClipboardCopy,
  Pencil,
  Move,
  FolderPlus,
  FolderX,
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
  RenameFile,
  CopyFile,
  MoveFile,
  OpenInFinder,
  AddFolderToWorkspace,
  RemoveFolderFromWorkspace,
  OpenFolderDialog,
  ReadFile,
  ReadFileBase64,
} from "../../wailsjs/go/main/App";
import type { FileInfo } from "../types";

interface ExplorerProps {
  roots: string[];
  onRefresh?: () => void;
}

export function Explorer({ roots, onRefresh }: ExplorerProps) {
  const [refreshKey, setRefreshKey] = useState(0);
  const [modal, setModal] = useState<{
    type: "createFile" | "createFolder" | "confirmDelete" | "rename" | "copy" | "move";
    dir: string;
    oldName?: string;
    oldPath?: string;
  } | null>(null);
  const [showHidden, setShowHidden] = useState(false);

  useEffect(() => {
    const dispose = EventsOn("fs:changed", () => {
      setRefreshKey((k) => k + 1);
      onRefresh?.();
    });
    return () => { if (dispose) dispose(); };
  }, [onRefresh]);

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
      } catch { }
    }, 1000);
    return () => clearInterval(interval);
  }, [onRefresh]);

  const triggerRefresh = () => {
    setRefreshKey((k) => k + 1);
    onRefresh?.();
  };

  const handleAddFolder = async () => {
    const path = await OpenFolderDialog();
    if (!path) return;
    try {
      await AddFolderToWorkspace(path);
      triggerRefresh();
    } catch (err) {
      console.error("Failed to add folder:", err);
    }
  };

  const handleRemoveFolder = async (path: string) => {
    if (roots.length <= 1) {
      // Removing the last folder makes no sense — close workspace instead
      return;
    }
    try {
      await RemoveFolderFromWorkspace(path);
      triggerRefresh();
    } catch (err) {
      console.error("Failed to remove folder:", err);
    }
  };

  const handleModalSubmit = async (value: string) => {
    if (!modal) return;
    try {
      if (modal.type === "createFile") {
        await CreateFile(modal.dir + "/" + value);
      } else if (modal.type === "createFolder") {
        await CreateFile(modal.dir + "/" + value);
      } else if (modal.type === "confirmDelete" && modal.oldPath) {
        await DeleteFile(modal.oldPath);
      } else if (modal.type === "rename" && modal.oldPath) {
        const dir = modal.dir;
        await RenameFile(modal.oldPath, dir + "/" + value);
      } else if (modal.type === "copy" && modal.oldPath) {
        const dir = modal.dir;
        const ext = modal.oldPath.includes(".") ? "." + modal.oldPath.split(".").pop() : "";
        await CopyFile(modal.oldPath, dir + "/" + value);
      } else if (modal.type === "move" && modal.oldPath) {
        const dir = modal.dir;
        await MoveFile(modal.oldPath, dir + "/" + value);
      }
      triggerRefresh();
    } catch (err) {
      console.error(err);
    }
    setModal(null);
  };

  if (roots.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center h-full text-sm text-muted-foreground">
        <p className="mb-3">No folders open</p>
        <button
          className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs bg-foreground/10 hover:bg-foreground/20 rounded transition-colors cursor-pointer"
          onClick={handleAddFolder}
        >
          <FolderPlus className="size-3.5" />
          Add Folder
        </button>
      </div>
    );
  }

  return (
    <ScrollArea className="h-full">
      <div className="flex items-center justify-between px-3 py-1 border-b sticky top-0 bg-background z-10">
        <span className="text-xs text-muted-foreground">Folders</span>
        <button
          className="text-xs px-2 py-0.5 hover:bg-accent rounded cursor-pointer text-muted-foreground"
          onClick={() => setShowHidden(!showHidden)}
          title={showHidden ? "Hide dot files" : "Show dot files"}
        >
          {showHidden ? "Hide ." : "Show ."}
        </button>
      </div>
      <div className="py-1">
        {roots.map((root) => (
          <FolderGroup
            key={root + refreshKey}
            rootPath={root}
            onRefresh={triggerRefresh}
            onShowModal={setModal}
            onRemove={roots.length > 1 ? handleRemoveFolder : undefined}
            showHidden={showHidden}
          />
        ))}
      </div>

      {/* Add Folder button at the bottom */}
      <div className="px-3 py-2 border-t mt-1">
        <button
          className="flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground hover:bg-accent/50 rounded px-2 py-1 w-full transition-colors cursor-pointer"
          onClick={handleAddFolder}
        >
          <FolderPlus className="size-3.5" />
          Add Folder
        </button>
      </div>

      <SimpleModal
        open={modal !== null}
        title={
          modal?.type === "createFile" ? "Create File" :
          modal?.type === "createFolder" ? "Create Folder" :
          modal?.type === "rename" ? "Rename" :
          modal?.type === "copy" ? "Copy As" :
          modal?.type === "move" ? "Move To" :
          modal?.type === "confirmDelete" ? "Delete" : ""
        }
        defaultValue={modal?.oldName}
        placeholder={
          modal?.type === "createFile" ? "filename.ts" :
          modal?.type === "createFolder" ? "folder-name" :
          modal?.type === "rename" && modal?.oldName ? modal.oldName :
          modal?.type === "copy" ? "copy-filename.ts" :
          "filename"
        }
        destructive={modal?.type === "confirmDelete"}
        onClose={() => setModal(null)}
        onSubmit={handleModalSubmit}
        submitLabel={
          modal?.type === "rename" ? "Rename" :
          modal?.type === "copy" ? "Copy" :
          modal?.type === "move" ? "Move" :
          modal?.type === "confirmDelete" ? "Delete" :
          "Create"
        }
      />
    </ScrollArea>
  );
}

// Accordion-style folder group: collapsible root with remove button
function FolderGroup({
  rootPath,
  onRefresh,
  onShowModal,
  onRemove,
  showHidden,
}: {
  rootPath: string;
  onRefresh: () => void;
  onShowModal: (m: any) => void;
  onRemove?: (path: string) => void;
  showHidden?: boolean;
}) {
  const [collapsed, setCollapsed] = useState(false);
  const [tree, setTree] = useState<FileInfo | null>(null);

  const handleDeleteItem = () => {
    if (!tree) return;
    onShowModal({ type: "confirmDelete", dir: rootPath, oldName: tree.name, oldPath: tree.path });
  };

  const handleCreateFolder = () => {
    onShowModal({ type: "createFolder", dir: rootPath });
  };

  useEffect(() => {
    async function load() {
      try {
        const result = await GetFileTree(2);
        const parsed: FileInfo[] = JSON.parse(result);
        const root = parsed.find(
          (f) => f.path === rootPath || f.name === rootPath.split("/").pop()
        );
        setTree(root || null);
      } catch { }
    }
    load();
  }, [rootPath]);

  return (
    <div className="border-b border-border/40">
      {/* Accordion header */}
      <div
        className="flex items-center gap-1 px-2 py-1.5 text-xs font-medium text-muted-foreground hover:bg-accent/30 cursor-pointer select-none group"
        onClick={() => setCollapsed(!collapsed)}
      >
        {collapsed ? (
          <ChevronRight className="size-3.5 shrink-0" />
        ) : (
          <ChevronDown className="size-3.5 shrink-0" />
        )}
        <FolderOpen className="size-3.5 shrink-0 text-amber-500" />
        <span className="truncate flex-1">{tree?.name ?? rootPath.split("/").pop()}</span>

        <div className="hidden group-hover:flex items-center gap-0.5">
          <button
            className="p-0.5 hover:bg-accent rounded cursor-pointer"
            onClick={(e) => { e.stopPropagation(); onShowModal({ type: "createFile", dir: rootPath }); }}
            title="New File"
          >
            <Plus className="size-3" />
          </button>
          <button
            className="p-0.5 hover:bg-accent rounded cursor-pointer"
            onClick={(e) => { e.stopPropagation(); handleCreateFolder(); }}
            title="New Folder"
          >
            <FolderPlus className="size-3" />
          </button>
          <button
            className="p-0.5 hover:bg-accent rounded cursor-pointer"
            onClick={(e) => { e.stopPropagation(); handleDeleteItem(); }}
            title="Delete"
          >
            <Trash2 className="size-3 text-red-400" />
          </button>
        </div>

        {onRemove && (
          <button
            className="p-0.5 hover:bg-accent rounded ml-1 cursor-pointer"
            onClick={(e) => { e.stopPropagation(); onRemove(rootPath); }}
            title="Remove folder from workspace"
          >
            <FolderX className="size-3 text-red-400" />
          </button>
        )}
      </div>
      {/* Children */}
      {!collapsed && tree && (
        <div className="ml-0">
          {(tree.children ?? [])
            .filter((child) => showHidden || !child.name.startsWith("."))
            .map((child) => (
              <FileNode
                key={child.path}
                node={child}
                depth={1}
                onRefresh={onRefresh}
                onShowModal={onShowModal}
                showHidden={showHidden}
              />
            ))}
        </div>
      )}
    </div>
  );
}

function FileNode({
  node,
  depth,
  onRefresh,
  onShowModal,
  parentPath,
  showHidden,
}: {
  node: FileInfo;
  depth: number;
  onRefresh: () => void;
  onShowModal: (m: { type: "createFile" | "createFolder" | "confirmDelete" | "rename" | "copy" | "move"; dir: string; oldName?: string; oldPath?: string } | null) => void;
  parentPath?: string;
  showHidden?: boolean;
}) {
  const [expanded, setExpanded] = useState(false);
  const [children, setChildren] = useState<FileInfo[] | undefined>(node.children);
  const [showMenu, setShowMenu] = useState(false);
  const [menuPos, setMenuPos] = useState({ x: 0, y: 0 });
  const [dragOver, setDragOver] = useState(false);
  const nodeRef = useRef<HTMLDivElement>(null);

  const handleToggle = async () => {
    if (node.isDir) {
      if (!expanded && !children) {
        try {
          const result = await ExpandPath(node.path);
          const parsed: FileInfo[] = JSON.parse(result);
          setChildren(parsed);
        } catch { }
      }
      setExpanded(!expanded);
    } else {
      globalOpenFile(node.path);
    }
  };

  const handleContextMenu = (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setMenuPos({ x: e.clientX, y: e.clientY });
    setShowMenu(true);
  };

  const closeMenu = () => setShowMenu(false);

  useEffect(() => {
    if (!showMenu) return;
    const handler = () => closeMenu();
    window.addEventListener("click", handler);
    return () => window.removeEventListener("click", handler);
  }, [showMenu]);

  const handleCreateFile = () => {
    const dir = node.isDir ? node.path : node.path.split("/").slice(0, -1).join("/");
    onShowModal({ type: "createFile", dir });
    closeMenu();
  };

  const handleDelete = () => {
    const dir = node.path.split("/").slice(0, -1).join("/");
    onShowModal({ type: "confirmDelete", dir, oldName: node.name, oldPath: node.path });
    closeMenu();
  };

  const handleRename = () => {
    const dir = node.path.split("/").slice(0, -1).join("/");
    onShowModal({ type: "rename", dir, oldName: node.name, oldPath: node.path });
    closeMenu();
  };

  const handleCopyFile = () => {
    const dir = node.path.split("/").slice(0, -1).join("/");
    const base = node.name.includes(".") ? node.name.split(".").slice(0, -1).join(".") + "-copy." + node.name.split(".").pop() : node.name + "-copy";
    onShowModal({ type: "copy", dir, oldName: base, oldPath: node.path });
    closeMenu();
  };

  const handleMove = () => {
    const dir = node.path.split("/").slice(0, -1).join("/");
    onShowModal({ type: "move", dir, oldName: node.name, oldPath: node.path });
    closeMenu();
  };

  const handleCopyRelative = () => {
    navigator.clipboard.writeText(node.path);
    closeMenu();
  };

  const handleCopyFullPath = () => {
    navigator.clipboard.writeText(node.path);
    closeMenu();
  };

  const handleCopyContent = async () => {
    try {
      const content = await ReadFile(node.path);
      navigator.clipboard.writeText(content);
    } catch { /* ignore */ }
    closeMenu();
  };

  const handleCopyImage = async () => {
    try {
      const b64 = await ReadFileBase64(node.path);
      const ext = node.name.split(".").pop()?.toLowerCase() || "png";
      navigator.clipboard.writeText(`data:image/${ext === "jpg" ? "jpeg" : ext};base64,${b64}`);
    } catch { /* ignore */ }
    closeMenu();
  };

  const isImage = ["png", "jpg", "jpeg", "gif", "svg", "webp", "ico"].includes(
    node.name.split(".").pop()?.toLowerCase() ?? ""
  );
  const isText = !isImage && !node.isDir && !["pdf", "bin", "exe", "dll", "so", "dylib"].includes(
    node.name.split(".").pop()?.toLowerCase() ?? ""
  );

  const handleOpenInFinder = () => {
    OpenInFinder(node.path).catch(() => {});
    closeMenu();
  };

  // DnD handlers
  const handleDragStart = (e: React.DragEvent) => {
    e.dataTransfer.setData("text/plain", node.path);
    e.dataTransfer.setData("application/forge-path", node.path);
    e.dataTransfer.effectAllowed = "move";
  };

  const handleDragOver = (e: React.DragEvent) => {
    if (!node.isDir) return;
    e.preventDefault();
    e.dataTransfer.dropEffect = "move";
    setDragOver(true);
  };

  const handleDragLeave = () => setDragOver(false);

  const handleDrop = async (e: React.DragEvent) => {
    e.preventDefault();
    setDragOver(false);
    if (!node.isDir) return;
    const srcPath = e.dataTransfer.getData("application/forge-path") || e.dataTransfer.getData("text/plain");
    if (!srcPath || srcPath === node.path) return;
    const name = srcPath.split("/").pop();
    try {
      await MoveFile(srcPath, node.path + "/" + name);
      onRefresh();
    } catch (err) {
      console.error(err);
    }
  };


  return (
    <div>
      <div
        ref={nodeRef}
        draggable
        className={cn(
          "flex items-center gap-1 px-2 py-0.5 text-sm cursor-pointer rounded group relative",
          node.gitIgnored && "opacity-40",
          dragOver && "bg-accent/50 ring-1 ring-primary"
        )}
        style={{ paddingLeft: `${depth * 16 + 8}px` }}
        onClick={handleToggle}
        onContextMenu={handleContextMenu}
        onDragStart={handleDragStart}
        onDragOver={handleDragOver}
        onDragLeave={handleDragLeave}
        onDrop={handleDrop}
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

        <div className="hidden group-hover:flex items-center gap-0.5">
          {node.isDir && (
            <button
              className="p-0.5 hover:bg-accent rounded"
              onClick={(e) => { e.stopPropagation(); handleCreateFile(); }}
              title="New File"
            >
              <Plus className="size-3" />
            </button>
          )}
          <button
            className="p-0.5 hover:bg-accent rounded"
            onClick={(e) => { e.stopPropagation(); handleDelete(); }}
            title="Delete"
          >
            <Trash2 className="size-3 text-red-400" />
          </button>
        </div>
      </div>

      {showMenu && (
        <div
          className="fixed z-50 bg-popover border rounded shadow-md py-1 min-w-40"
          style={{ left: menuPos.x, top: menuPos.y }}
          onClick={(e) => e.stopPropagation()}
        >
          <div className="px-2 py-1 text-[10px] text-muted-foreground uppercase tracking-wider border-b mb-1">
            {node.isDir ? "Folder" : "File"}
          </div>

          <button className="flex items-center gap-2 px-3 py-1.5 text-xs hover:bg-accent w-full text-left"
            onClick={() => { globalOpenFile(node.path); closeMenu(); }}>
            <File className="size-3" /> Open
          </button>

          <button className="flex items-center gap-2 px-3 py-1.5 text-xs hover:bg-accent w-full text-left"
            onClick={handleOpenInFinder}>
            <FolderOpen className="size-3" /> Open in Finder
          </button>

          {node.isDir && (
            <button className="flex items-center gap-2 px-3 py-1.5 text-xs hover:bg-accent w-full text-left"
              onClick={handleCreateFile}>
              <Plus className="size-3" /> New File
            </button>
          )}

          {node.isDir && (
            <button className="flex items-center gap-2 px-3 py-1.5 text-xs hover:bg-accent w-full text-left"
              onClick={() => { onShowModal({ type: "createFolder", dir: node.path }); closeMenu(); }}>
              <Plus className="size-3" /> New Folder
            </button>
          )}

          <div className="border-t my-1" />

          <button className="flex items-center gap-2 px-3 py-1.5 text-xs hover:bg-accent w-full text-left"
            onClick={handleCopyRelative}>
            <ClipboardCopy className="size-3" /> Copy Relative Path
          </button>

          <button className="flex items-center gap-2 px-3 py-1.5 text-xs hover:bg-accent w-full text-left"
            onClick={handleCopyFullPath}>
            <ClipboardCopy className="size-3" /> Copy Full Path
          </button>

          {isText && (
            <button className="flex items-center gap-2 px-3 py-1.5 text-xs hover:bg-accent w-full text-left"
              onClick={handleCopyContent}>
              <ClipboardCopy className="size-3" /> Copy Content
            </button>
          )}

          {isImage && (
            <button className="flex items-center gap-2 px-3 py-1.5 text-xs hover:bg-accent w-full text-left"
              onClick={handleCopyImage}>
              <ClipboardCopy className="size-3" /> Copy Image
            </button>
          )}

          {!node.isDir && (
            <button className="flex items-center gap-2 px-3 py-1.5 text-xs hover:bg-accent w-full text-left"
              onClick={handleCopyFile}>
              <Copy className="size-3" /> Copy File
            </button>
          )}

          <div className="border-t my-1" />

          <button className="flex items-center gap-2 px-3 py-1.5 text-xs hover:bg-accent w-full text-left"
            onClick={handleRename}>
            <Pencil className="size-3" /> Rename
          </button>

          <button className="flex items-center gap-2 px-3 py-1.5 text-xs hover:bg-accent w-full text-left"
            onClick={handleMove}>
            <Move className="size-3" /> Move
          </button>

          <div className="border-t my-1" />

          <button className="flex items-center gap-2 px-3 py-1.5 text-xs hover:bg-accent w-full text-left text-red-400"
            onClick={handleDelete}>
            <Trash2 className="size-3" /> Delete
          </button>
        </div>
      )}

{expanded && children && (
          <div>
            {children
              .filter((child) => showHidden || !child.name.startsWith("."))
              .map((child) => (
                <FileNode
                  key={child.path}
                  node={child}
                  depth={depth + 1}
                  onRefresh={onRefresh}
                  onShowModal={onShowModal}
                  showHidden={showHidden}
                  parentPath={node.path}
                />
              ))}
          </div>
        )}
    </div>
  );
}
