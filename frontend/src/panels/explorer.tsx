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
  GetRepoStatus,
} from "../../wailsjs/go/main/App";
import type { FileInfo, GitStatusEntry } from "../types";

interface ExplorerProps {
  roots: string[];
  onRefresh?: () => void;
}

export function Explorer({ roots, onRefresh }: ExplorerProps) {
  const [refreshKey, setRefreshKey] = useState(0);
  const [gitStatus, setGitStatus] = useState<Map<string, string>>(new Map());
  const [dirGitStatus, setDirGitStatus] = useState<Map<string, string>>(new Map());
  const [modal, setModal] = useState<{
    type: "createFile" | "createFolder" | "delete" | "rename" | "copy" | "move";
    dir: string;
    oldName?: string;
    oldPath?: string;
  } | null>(null);

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

  const refreshGit = useCallback(async () => {
    try {
      const statusMap: Record<string, GitStatusEntry[]> = await GetRepoStatus();
      const fileStatus = new Map<string, string>();
      const dirStatus = new Map<string, string>();
      for (const entries of Object.values(statusMap)) {
        for (const s of entries) {
          const code = s.staging && s.staging !== " " ? s.staging : s.worktree;
          if (code && code !== " ") {
            fileStatus.set(s.path, code);
            // Bubble up to parent directories
            const parts = s.path.split("/");
            for (let i = 1; i < parts.length; i++) {
              const parent = parts.slice(0, i).join("/");
              const existing = dirStatus.get(parent);
              if (!existing || priority(code) < priority(existing)) {
                dirStatus.set(parent, code);
              }
            }
          }
        }
      }
      setGitStatus(fileStatus);
      setDirGitStatus(dirStatus);
    } catch { }
  }, []);

  useEffect(() => {
    refreshGit();
  }, [refreshGit, refreshKey]);

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
            gitStatus={gitStatus}
            dirGitStatus={dirGitStatus}
            onRefresh={triggerRefresh}
            onShowModal={setModal}
          />
        ))}
      </div>

      <SimpleModal
        open={modal !== null}
        title={
          modal?.type === "createFile" ? "Create File" :
          modal?.type === "createFolder" ? "Create Folder" :
          modal?.type === "rename" ? "Rename" :
          modal?.type === "copy" ? "Copy As" :
          modal?.type === "move" ? "Move To" : ""
        }
        placeholder={
          modal?.type === "createFile" ? "filename.ts" :
          modal?.type === "createFolder" ? "folder-name" :
          modal?.type === "rename" && modal?.oldName ? modal.oldName :
          modal?.type === "copy" ? "copy-filename.ts" :
          "filename"
        }
        onClose={() => setModal(null)}
        onSubmit={handleModalSubmit}
        submitLabel={
          modal?.type === "rename" ? "Rename" :
          modal?.type === "copy" ? "Copy" :
          modal?.type === "move" ? "Move" :
          "Create"
        }
      />
    </ScrollArea>
  );
}

function FolderTree({
  rootPath,
  gitStatus,
  dirGitStatus,
  onRefresh,
  onShowModal,
}: {
  rootPath: string;
  gitStatus: Map<string, string>;
  dirGitStatus: Map<string, string>;
  onRefresh: () => void;
  onShowModal: (m: { type: "createFile" | "createFolder" | "rename" | "copy" | "move"; dir: string; oldName?: string; oldPath?: string } | null) => void;
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
      } catch { }
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
            gitStatus={gitStatus}
            dirGitStatus={dirGitStatus}
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
  gitStatus,
  dirGitStatus,
  onRefresh,
  onShowModal,
  parentPath,
}: {
  node: FileInfo;
  depth: number;
  gitStatus: Map<string, string>;
  dirGitStatus: Map<string, string>;
  onRefresh: () => void;
  onShowModal: (m: { type: "createFile" | "createFolder" | "rename" | "copy" | "move"; dir: string; oldName?: string; oldPath?: string } | null) => void;
  parentPath?: string;
}) {
  const [expanded, setExpanded] = useState(false);
  const [children, setChildren] = useState<FileInfo[] | undefined>(node.children);
  const [showMenu, setShowMenu] = useState(false);
  const [menuPos, setMenuPos] = useState({ x: 0, y: 0 });
  const [dragOver, setDragOver] = useState(false);
  const nodeRef = useRef<HTMLDivElement>(null);

  // For directories, use dirGitStatus (bubbled up from children); for files use direct gitStatus
  const gitCode = node.isDir
    ? getDirGitCode(node.path, dirGitStatus, gitStatus)
    : getGitCode(node.path, gitStatus);

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

  const handleDelete = async () => {
    if (!confirm(`Delete "${node.name}"?`)) return;
    try {
      await DeleteFile(node.path);
      onRefresh();
    } catch (err) {
      console.error(err);
    }
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

  if (node.hidden) return null;

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

        {/* Directory git indicator: colored dot */}
        {gitCode && node.isDir && (
          <span className={cn(
            "w-2 h-2 rounded-full shrink-0 mr-1 inline-block",
            gitCode === "M" && "bg-blue-400",
            gitCode === "A" && "bg-green-400",
            gitCode === "D" && "bg-red-400",
            gitCode === "R" && "bg-purple-400",
            gitCode === "?" && "bg-green-400/60",
          )} />
        )}

        {/* File git indicator: letter */}
        {gitCode && !node.isDir && (
          <span className={cn(
            "text-[10px] font-bold shrink-0 mr-1",
            gitCode === "M" && "text-blue-400",
            gitCode === "A" && "text-green-400",
            gitCode === "D" && "text-red-400",
            gitCode === "R" && "text-purple-400",
            gitCode === "C" && "text-orange-400",
            gitCode === "U" && "text-orange-400",
            gitCode === "?" && "text-green-400/60",
          )}>
            {gitCode === "?" ? "U" : gitCode}
          </span>
        )}

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
          {children.map((child) => (
            <FileNode
              key={child.path}
              node={child}
              depth={depth + 1}
              gitStatus={gitStatus}
              dirGitStatus={dirGitStatus}
              onRefresh={onRefresh}
              onShowModal={onShowModal}
              parentPath={node.path}
            />
          ))}
        </div>
      )}
    </div>
  );
}

function getGitCode(path: string, status: Map<string, string>): string {
  for (const [p, code] of status.entries()) {
    if (path.endsWith("/" + p) || path === p) return code;
  }
  return "";
}

function getDirGitCode(path: string, dirStatus: Map<string, string>, fileStatus: Map<string, string>): string {
  for (const [p, code] of dirStatus.entries()) {
    if (path.endsWith("/" + p) || path === p) return code;
  }
  for (const [p, code] of fileStatus.entries()) {
    if (path.endsWith("/" + p.split("/")[0])) return code;
  }
  return "";
}

function priority(code: string): number {
  switch (code) {
    case "D": return 0;
    case "M": return 1;
    case "A": return 2;
    case "R": return 3;
    case "?": return 4;
    default: return 5;
  }
}
