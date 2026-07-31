import React, { useEffect, useState, useRef } from "react";
import {
  GetFileTree,
  ToggleHiddenFiles,
  ListSessions,
  ListAgentSessions,
  StopSession,
  DeleteAgentSession,
  CreateFile,
  DeleteFile,
  RenameFile,
  CopyFile,
  MoveFile,
  OpenInFinder,
  ExpandPath,
} from "../lib/wails";
import { getFileIcon } from "../lib/file-icons";
import { useEditorStore } from "../hooks/store";
import { globalOpenFile } from "../panels/editor";
import { cn } from "../lib/utils";
import { GitPanel } from "./git-panel";
import {
  IconFolder,
  IconFolderOpen,
  IconEye,
  IconEyeOff,
  IconTerminal2,
  IconPlus,
  IconRobot,
  IconFiles,
  IconGitBranch,
  IconX,
  IconTrash,
  IconPencil,
  IconCopy,
  IconClipboard,
  IconFile,
} from "@tabler/icons-react";

interface SidebarProps {
  folders: string[];
  onRefreshWorkspace: () => void;
  collapsed: boolean;
  onToggleCollapse: (collapsed: boolean) => void;
  onCreateShell: () => void;
  onCreateAgent: () => void;
}

interface FileNode {
  name: string;
  path: string;
  isDir: boolean;
  children?: FileNode[];
  hidden: boolean;
}

// Helper to collect all folder paths recursively
function collectDirPaths(nodes: FileNode[], paths: Record<string, boolean> = {}) {
  for (const node of nodes) {
    if (node.isDir) {
      paths[node.path] = true;
      if (node.children) {
        collectDirPaths(node.children, paths);
      }
    }
  }
  return paths;
}

function findNodeByPath(nodes: FileNode[], path: string): FileNode | null {
  for (const node of nodes) {
    if (node.path === path) return node;
    if (node.isDir && node.children) {
      const found = findNodeByPath(node.children, path);
      if (found) return found;
    }
  }
  return null;
}

function updateNodeChildren(nodes: FileNode[], path: string, children: FileNode[]): FileNode[] {
  return nodes.map((node) => {
    if (node.path === path) {
      return { ...node, children };
    }
    if (node.isDir && node.children) {
      return {
        ...node,
        children: updateNodeChildren(node.children, path, children),
      };
    }
    return node;
  });
}

export function Sidebar({
  folders,
  onRefreshWorkspace,
  collapsed,
  onToggleCollapse,
  onCreateShell,
  onCreateAgent,
}: SidebarProps) {
  const [activeTab, setActiveTab] = useState<"explorer" | "git">("explorer");
  const [tree, setTree] = useState<FileNode[]>([]);
  const [expandedDirs, setExpandedDirs] = useState<Record<string, boolean>>({});
  const [showHidden, setShowHidden] = useState(true);
  const [sessions, setSessions] = useState<any[]>([]);

  // Vertical resizer state
  const [sessionsHeightPercent, setSessionsHeightPercent] = useState(30);
  const [isDraggingSessions, setIsDraggingSessions] = useState(false);

  // Context Menu State
  const [contextMenu, setContextMenu] = useState<{
    x: number;
    y: number;
    node: FileNode;
  } | null>(null);

  // Copy / Paste File System Clipboard State
  const clipboardPathRef = useRef<string | null>(null);

  // Simple modal prompt state
  const [modalPrompt, setModalPrompt] = useState<{
    type: "createFile" | "createFolder" | "rename";
    dirPath: string;
    oldPath?: string;
    defaultValue?: string;
  } | null>(null);
  const [promptInputValue, setPromptInputValue] = useState("");

  const { files, setFiles, setActiveFileIndex } = useEditorStore();

  useEffect(() => {
    if (folders && folders.length > 0) {
      loadTree();
    }
  }, [folders, showHidden]);

  useEffect(() => {
    loadSessions();
    const timer = setInterval(loadSessions, 3000);
    return () => clearInterval(timer);
  }, []);

  // Closes context menu on click elsewhere
  useEffect(() => {
    const handleWindowClick = () => {
      setContextMenu(null);
    };
    window.addEventListener("click", handleWindowClick);
    return () => window.removeEventListener("click", handleWindowClick);
  }, []);

  async function loadTree() {
    try {
      const dataStr = await GetFileTree(2);
      const data = JSON.parse(dataStr);
      const nodes = Array.isArray(data) ? data : [];
      setTree(nodes);

      // Auto-expand only root-level folders by default on initial tree fetch (preserving user expansion)
      setExpandedDirs((prev) => {
        const next = { ...prev };
        for (const node of nodes) {
          if (node.isDir && next[node.path] === undefined) {
            next[node.path] = true;
          }
        }
        return next;
      });
    } catch (err) {
      console.error("Failed to load file tree:", err);
    }
  }

  async function loadSessions() {
    try {
      const shellList = await ListSessions();
      const agentList = await ListAgentSessions();
      
      const merged = [
        ...shellList.map((s) => ({ ...s, type: "shell" })),
        ...agentList.map((a) => ({ ...a, type: "agent" })),
      ];
      setSessions(merged);
    } catch { /* ignore */ }
  }

  const toggleDir = async (path: string) => {
    const isExpanding = !expandedDirs[path];
    if (isExpanding) {
      const node = findNodeByPath(tree, path);
      if (node && (!node.children || node.children.length === 0)) {
        try {
          const childrenStr = await ExpandPath(path);
          const children = JSON.parse(childrenStr);
          if (Array.isArray(children)) {
            setTree((prevTree) => updateNodeChildren(prevTree, path, children));
          }
        } catch (err) {
          console.error("Failed to expand path:", err);
        }
      }
    }

    setExpandedDirs((prev) => ({
      ...prev,
      [path]: isExpanding,
    }));
  };

  const handleToggleHidden = async () => {
    const nextVal = await ToggleHiddenFiles();
    setShowHidden(nextVal);
  };

  const handleOpenSessionTab = (s: any) => {
    const existingIdx = files.findIndex((f) => f.id === s.id);
    if (existingIdx !== -1) {
      setActiveFileIndex(existingIdx);
      return;
    }

    const newTab = {
      id: s.id,
      name: s.name || (s.type === "shell" ? "Terminal" : "Agent"),
      path: s.id,
      type: s.type as "shell" | "agent",
      content: "",
      modified: false,
    };

    setFiles((prev) => [...prev, newTab]);
    setActiveFileIndex(files.length);
  };

  const handleStopSession = async (id: string, type: string, e: React.MouseEvent) => {
    e.stopPropagation();
    try {
      if (type === "agent") {
        await DeleteAgentSession(id);
      } else {
        await StopSession(id);
      }
      loadSessions();
    } catch (err) {
      console.error("Failed to stop session:", err);
    }
  };

  // Vertical resize handlers
  const handleMouseDownSessions = (e: React.MouseEvent) => {
    e.preventDefault();
    setIsDraggingSessions(true);
  };

  useEffect(() => {
    if (!isDraggingSessions) return;

    const handleMouseMove = (e: MouseEvent) => {
      const container = document.querySelector(".sidebar-panel-container");
      if (container) {
        const rect = container.getBoundingClientRect();
        const nextHeight = rect.bottom - e.clientY;
        const nextPercent = (nextHeight / rect.height) * 100;
        setSessionsHeightPercent(Math.max(15, Math.min(75, nextPercent)));
      }
    };

    const handleMouseUp = () => {
      setIsDraggingSessions(false);
    };

    window.addEventListener("mousemove", handleMouseMove);
    window.addEventListener("mouseup", handleMouseUp);
    return () => {
      window.removeEventListener("mousemove", handleMouseMove);
      window.removeEventListener("mouseup", handleMouseUp);
    };
  }, [isDraggingSessions]);

  // Context Menu handlers
  const handleNodeContextMenu = (e: React.MouseEvent, node: FileNode) => {
    e.preventDefault();
    e.stopPropagation();
    setContextMenu({
      x: e.clientX,
      y: e.clientY,
      node,
    });
  };

  // Filesystem action functions
  const handleCopyPath = (node: FileNode, full: boolean) => {
    navigator.clipboard.writeText(node.path);
  };

  const handleOpenFolderInFinder = (node: FileNode) => {
    OpenInFinder(node.path).catch(console.error);
  };

  const handleCreatePrompt = (type: "createFile" | "createFolder", dirPath: string) => {
    setPromptInputValue("");
    setModalPrompt({ type, dirPath });
  };

  const handleRenamePrompt = (node: FileNode) => {
    setPromptInputValue(node.name);
    setModalPrompt({
      type: "rename",
      dirPath: node.path.split(/[/\\]/).slice(0, -1).join("/"),
      oldPath: node.path,
      defaultValue: node.name,
    });
  };

  const handleCopyFileRef = (node: FileNode) => {
    clipboardPathRef.current = node.path;
  };

  const handlePasteFile = async (targetDir: string) => {
    const src = clipboardPathRef.current;
    if (!src) return;
    const name = src.split(/[/\\]/).pop();
    if (!name) return;
    try {
      await CopyFile(src, targetDir + "/" + name);
      loadTree();
    } catch (err) {
      alert("Paste failed: " + err);
    }
  };

  const handleDeleteNode = async (node: FileNode) => {
    if (!confirm(`Delete ${node.name}? This will permanently delete the file/folder.`)) return;
    try {
      await DeleteFile(node.path);
      loadTree();
    } catch (err) {
      alert("Delete failed: " + err);
    }
  };

  const handlePromptSubmit = async () => {
    if (!modalPrompt || !promptInputValue.trim()) return;
    const val = promptInputValue.trim();
    try {
      if (modalPrompt.type === "createFile") {
        await CreateFile(modalPrompt.dirPath + "/" + val);
      } else if (modalPrompt.type === "createFolder") {
        await CreateFile(modalPrompt.dirPath + "/" + val); // backend handles folder creation
      } else if (modalPrompt.type === "rename" && modalPrompt.oldPath) {
        await RenameFile(modalPrompt.oldPath, modalPrompt.dirPath + "/" + val);
      }
      loadTree();
      setModalPrompt(null);
    } catch (err) {
      alert("FileSystem Action Failed: " + err);
    }
  };

  // Drag and Drop support
  const handleDragStart = (e: React.DragEvent, node: FileNode) => {
    e.dataTransfer.setData("text/plain", node.path);
    e.dataTransfer.effectAllowed = "move";
  };

  const handleDragOver = (e: React.DragEvent, node: FileNode) => {
    if (!node.isDir) return;
    e.preventDefault();
    e.dataTransfer.dropEffect = "move";
  };

  const handleDropNode = async (e: React.DragEvent, targetNode: FileNode) => {
    e.preventDefault();
    if (!targetNode.isDir) return;
    const srcPath = e.dataTransfer.getData("text/plain");
    if (!srcPath || srcPath === targetNode.path) return;
    const name = srcPath.split(/[/\\]/).pop();
    if (!name) return;
    try {
      await MoveFile(srcPath, targetNode.path + "/" + name);
      loadTree();
    } catch (err) {
      alert("Drag-drop move failed: " + err);
    }
  };

  const renderNode = (node: FileNode, depth = 0) => {
    if (node.hidden && !showHidden) return null;

    const isExpanded = !!expandedDirs[node.path];
    const indentStyle = { paddingLeft: `${depth * 8 + 8}px` };

    if (node.isDir) {
      return (
        <div key={node.path}>
          <div
            onClick={() => toggleDir(node.path)}
            onContextMenu={(e) => handleNodeContextMenu(e, node)}
            onDragOver={(e) => handleDragOver(e, node)}
            onDrop={(e) => handleDropNode(e, node)}
            style={indentStyle}
            className="flex items-center gap-1.5 py-0.5 text-xs text-[var(--fg-secondary)] hover:text-[var(--fg-primary)] hover:bg-[var(--bg-surface-hover)] cursor-pointer select-none group"
            draggable
            onDragStart={(e) => handleDragStart(e, node)}
          >
            {isExpanded ? (
              <IconFolderOpen className="size-3.5 text-amber-400 shrink-0" />
            ) : (
              <IconFolder className="size-3.5 text-amber-400 shrink-0" />
            )}
            <span className="truncate">{node.name}</span>
          </div>

          {isExpanded && node.children && (
            <div className="flex flex-col">
              {node.children.map((child) => renderNode(child, depth + 1))}
            </div>
          )}
        </div>
      );
    }

    return (
      <div
        key={node.path}
        onClick={() => {
          const existingIdx = files.findIndex((f) => f.path === node.path);
          if (existingIdx !== -1) {
            setActiveFileIndex(existingIdx);
            return;
          }
          globalOpenFile(node.path);
        }}
        onContextMenu={(e) => handleNodeContextMenu(e, node)}
        style={indentStyle}
        className="flex items-center gap-1.5 py-0.5 text-xs text-[var(--fg-secondary)] hover:text-[var(--fg-primary)] hover:bg-[var(--bg-surface-hover)] cursor-pointer select-none"
        draggable
        onDragStart={(e) => handleDragStart(e, node)}
      >
        {getFileIcon(node.name, "size-3.5 shrink-0")}
        <span className="truncate">{node.name}</span>
      </div>
    );
  };

  if (collapsed) return null;

  return (
    <div className="flex h-full w-[240px] bg-[var(--bg-sidebar)] border-r border-[var(--border-default)] shrink-0 select-none font-sans overflow-hidden relative">
      {/* Icon Switcher Dock */}
      <div className="w-10 border-r border-[var(--border-default)] bg-[var(--bg-panel)] flex flex-col items-center py-2 gap-3 shrink-0">
        <button
          onClick={() => setActiveTab("explorer")}
          className={cn(
            "p-1.5 rounded transition-all cursor-pointer",
            activeTab === "explorer"
              ? "text-[var(--accent-primary)] bg-[var(--bg-surface-active)]"
              : "text-[var(--fg-tertiary)] hover:text-[var(--fg-primary)]"
          )}
          title="File Explorer"
        >
          <IconFiles className="size-5" />
        </button>
        <button
          onClick={() => setActiveTab("git")}
          className={cn(
            "p-1.5 rounded transition-all cursor-pointer",
            activeTab === "git"
              ? "text-[var(--accent-primary)] bg-[var(--bg-surface-active)]"
              : "text-[var(--fg-tertiary)] hover:text-[var(--fg-primary)]"
          )}
          title="Git Control"
        >
          <IconGitBranch className="size-5" />
        </button>
      </div>

      {/* Panel view */}
      <div className="flex-1 flex flex-col min-w-0 sidebar-panel-container">
        {activeTab === "git" ? (
          <GitPanel />
        ) : (
          <div className="flex-1 flex flex-col min-w-0">
            {/* Header */}
            <div className="flex items-center justify-between px-3 py-2 border-b border-[var(--border-default)] shrink-0 bg-[var(--bg-panel)]">
              <span className="text-[10px] font-bold uppercase tracking-wider text-[var(--fg-tertiary)]">Explorer</span>
              <button
                onClick={handleToggleHidden}
                className="p-1 hover:bg-[var(--bg-surface-hover)] rounded text-[var(--fg-tertiary)] hover:text-white cursor-pointer"
                title="Toggle Hidden Files"
              >
                {showHidden ? <IconEye className="size-3.5" /> : <IconEyeOff className="size-3.5" />}
              </button>
            </div>

            {/* File Tree */}
            <div className="flex-1 min-h-0 overflow-y-auto py-1">
              {tree.map((node) => renderNode(node, 0))}
            </div>

            {/* Vertical drag handle */}
            <div
              onMouseDown={handleMouseDownSessions}
              className="h-[3px] bg-[var(--border-default)] hover:bg-[var(--accent-primary)] cursor-ns-resize shrink-0 transition-colors"
              title="Drag to resize Session Manager"
            />

            {/* Sessions Manager container */}
            <div
              style={{ height: `${sessionsHeightPercent}%` }}
              className="shrink-0 flex flex-col bg-[var(--bg-panel)] overflow-hidden"
            >
              <div className="flex items-center justify-between px-3 pt-2 pb-1 select-none">
                <span className="text-[10px] font-bold uppercase tracking-wider text-[var(--fg-tertiary)]">Session Manager</span>
                <div className="flex items-center space-x-1">
                  <button
                    onClick={onCreateShell}
                    className="p-0.5 hover:bg-[var(--bg-surface-hover)] rounded text-[var(--fg-secondary)] hover:text-white cursor-pointer"
                    title="Launch Terminal"
                  >
                    <IconTerminal2 className="size-3.5 text-cyan-400" />
                  </button>
                  <button
                    onClick={onCreateAgent}
                    className="p-0.5 hover:bg-[var(--bg-surface-hover)] rounded text-[var(--fg-secondary)] hover:text-white cursor-pointer"
                    title="Launch AI Agent"
                  >
                    <IconRobot className="size-3.5 text-blue-400" />
                  </button>
                </div>
              </div>

              <div className="flex-1 overflow-y-auto px-3 pb-2 space-y-1">
                {sessions.map((s) => (
                  <div
                    key={s.id}
                    onClick={() => handleOpenSessionTab(s)}
                    className="flex items-center justify-between px-2 py-1 text-xs text-[var(--fg-secondary)] hover:text-[var(--fg-primary)] hover:bg-[var(--bg-surface-hover)] cursor-pointer group rounded"
                  >
                    <div className="flex items-center gap-1.5 min-w-0">
                      {s.type === "shell" ? (
                        <IconTerminal2 className="size-3.5 text-cyan-400 shrink-0" />
                      ) : (
                        <IconRobot className="size-3.5 text-blue-400 shrink-0" />
                      )}
                      <span className="truncate">{s.name}</span>
                    </div>

                    <button
                      onClick={(e) => handleStopSession(s.id, s.type, e)}
                      className="opacity-0 group-hover:opacity-100 hover:bg-[var(--bg-surface-hover)] rounded p-0.5"
                      title="Kill Session"
                    >
                      <IconX className="size-3 text-red-400" />
                    </button>
                  </div>
                ))}
                {sessions.length === 0 && (
                  <div className="text-[10px] text-[var(--fg-tertiary)] italic">No active sessions</div>
                )}
              </div>
            </div>
          </div>
        )}
      </div>

      {/* Right Click Context Menu Overlay */}
      {contextMenu && (
        <div
          style={{
            position: "fixed",
            top: contextMenu.y,
            left: contextMenu.x,
          }}
          className="z-50 bg-[var(--bg-sidebar)] border border-[var(--border-default)] shadow-2xl py-1 min-w-44 text-[11px]"
          onClick={(e) => e.stopPropagation()}
        >
          <div className="px-3 py-1 font-bold text-[9px] text-[var(--fg-tertiary)] uppercase tracking-wider border-b border-[var(--border-default)] mb-1 select-none">
            {contextMenu.node.isDir ? "Folder Action" : "File Action"}
          </div>

          <button
            onClick={() => {
              if (contextMenu.node.isDir) {
                toggleDir(contextMenu.node.path);
              } else {
                globalOpenFile(contextMenu.node.path);
              }
              setContextMenu(null);
            }}
            className="w-full text-left px-3 py-1.5 hover:bg-[var(--bg-panel)] flex items-center space-x-2 text-[var(--fg-primary)] cursor-pointer"
          >
            {contextMenu.node.isDir ? <IconFolder className="size-3.5" /> : <IconFile className="size-3.5" />}
            <span>Open</span>
          </button>

          <button
            onClick={() => {
              handleOpenFolderInFinder(contextMenu.node);
              setContextMenu(null);
            }}
            className="w-full text-left px-3 py-1.5 hover:bg-[var(--bg-panel)] flex items-center space-x-2 text-[var(--fg-primary)] cursor-pointer"
          >
            <IconFolderOpen className="size-3.5 text-amber-400" />
            <span>Open in Finder</span>
          </button>

          {contextMenu.node.isDir && (
            <>
              <button
                onClick={() => {
                  handleCreatePrompt("createFile", contextMenu.node.path);
                  setContextMenu(null);
                }}
                className="w-full text-left px-3 py-1.5 hover:bg-[var(--bg-panel)] flex items-center space-x-2 text-[var(--fg-primary)] cursor-pointer"
              >
                <IconPlus className="size-3.5 text-green-400" />
                <span>New File</span>
              </button>
              <button
                onClick={() => {
                  handleCreatePrompt("createFolder", contextMenu.node.path);
                  setContextMenu(null);
                }}
                className="w-full text-left px-3 py-1.5 hover:bg-[var(--bg-panel)] flex items-center space-x-2 text-[var(--fg-primary)] cursor-pointer"
              >
                <IconPlus className="size-3.5 text-blue-400" />
                <span>New Folder</span>
              </button>
            </>
          )}

          <div className="h-px bg-[var(--border-default)] my-1" />

          <button
            onClick={() => {
              handleCopyFileRef(contextMenu.node);
              setContextMenu(null);
            }}
            className="w-full text-left px-3 py-1.5 hover:bg-[var(--bg-panel)] flex items-center space-x-2 text-[var(--fg-primary)] cursor-pointer"
          >
            <IconCopy className="size-3.5 text-blue-400" />
            <span>Copy File</span>
          </button>

          {contextMenu.node.isDir && clipboardPathRef.current && (
            <button
              onClick={() => {
                handlePasteFile(contextMenu.node.path);
                setContextMenu(null);
              }}
              className="w-full text-left px-3 py-1.5 hover:bg-[var(--bg-panel)] flex items-center space-x-2 text-[var(--fg-primary)] cursor-pointer"
            >
              <IconClipboard className="size-3.5 text-green-400" />
              <span>Paste Into Folder</span>
            </button>
          )}

          <button
            onClick={() => {
              handleCopyPath(contextMenu.node, false);
              setContextMenu(null);
            }}
            className="w-full text-left px-3 py-1.5 hover:bg-[var(--bg-panel)] flex items-center space-x-2 text-[var(--fg-primary)] cursor-pointer"
          >
            <IconClipboard className="size-3.5" />
            <span>Copy Relative Path</span>
          </button>

          <button
            onClick={() => {
              handleCopyPath(contextMenu.node, true);
              setContextMenu(null);
            }}
            className="w-full text-left px-3 py-1.5 hover:bg-[var(--bg-panel)] flex items-center space-x-2 text-[var(--fg-primary)] cursor-pointer"
          >
            <IconClipboard className="size-3.5" />
            <span>Copy Full Path</span>
          </button>

          <div className="h-px bg-[var(--border-default)] my-1" />

          <button
            onClick={() => {
              handleRenamePrompt(contextMenu.node);
              setContextMenu(null);
            }}
            className="w-full text-left px-3 py-1.5 hover:bg-[var(--bg-panel)] flex items-center space-x-2 text-[var(--fg-primary)] cursor-pointer"
          >
            <IconPencil className="size-3.5 text-amber-500" />
            <span>Rename</span>
          </button>

          <button
            onClick={() => {
              handleDeleteNode(contextMenu.node);
              setContextMenu(null);
            }}
            className="w-full text-left px-3 py-1.5 hover:bg-[var(--bg-panel)] flex items-center space-x-2 text-red-400 cursor-pointer"
          >
            <IconTrash className="size-3.5" />
            <span>Delete</span>
          </button>
        </div>
      )}

      {/* Simple Prompt Modal Overlay */}
      {modalPrompt && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className="bg-[var(--bg-sidebar)] border border-[var(--border-default)] w-full max-w-sm p-4 space-y-4 shadow-2xl">
            <div className="flex items-center justify-between pb-2 border-b border-[var(--border-default)]">
              <span className="font-bold text-xs text-[var(--fg-primary)] uppercase tracking-wider">
                {modalPrompt.type === "createFile" && "Create File"}
                {modalPrompt.type === "createFolder" && "Create Folder"}
                {modalPrompt.type === "rename" && "Rename"}
              </span>
              <button
                onClick={() => setModalPrompt(null)}
                className="text-[var(--fg-tertiary)] hover:text-white cursor-pointer"
              >
                <IconX className="size-4" />
              </button>
            </div>

            <div className="space-y-2 text-xs">
              <label className="text-[var(--fg-secondary)] block font-medium">Name</label>
              <input
                type="text"
                value={promptInputValue}
                onChange={(e) => setPromptInputValue(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") handlePromptSubmit();
                }}
                autoFocus
                placeholder={
                  modalPrompt.type === "createFile" ? "filename.txt" :
                  modalPrompt.type === "createFolder" ? "folder_name" :
                  "new_name"
                }
                className="w-full bg-[var(--bg-panel)] border border-[var(--border-default)] px-3 py-1.5 text-[var(--fg-primary)] focus:outline-none focus:border-[var(--accent-primary)] font-mono text-[11px]"
              />
            </div>

            <div className="flex items-center justify-end space-x-2 pt-2 border-t border-[var(--border-default)]">
              <button
                onClick={() => setModalPrompt(null)}
                className="px-3 py-1.5 text-xs text-[var(--fg-secondary)] hover:text-white cursor-pointer"
              >
                Cancel
              </button>
              <button
                onClick={handlePromptSubmit}
                className="px-4 py-1.5 bg-[var(--accent-primary)] hover:bg-[var(--accent-hover)] text-black text-xs font-semibold cursor-pointer"
              >
                Confirm
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
