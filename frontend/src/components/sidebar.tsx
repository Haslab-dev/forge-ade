import React, { memo, useEffect, useState, useRef, useCallback, useMemo } from "react";
import {
  GetFileTree,
  ToggleHiddenFiles,
  ListSessions,
  ListAgentSessions,
  ListAgentSessionsForFolder,
  RenameSession,
  RenameAgentSession,
  StopSession,
  DeleteAgentSession,
  CreateFile,
  CreateFolder,
  DeleteFile,
  RenameFile,
  CopyFile,
  CopyPath,
  GetClipboardFiles,
  GetFsChangeCount,
  MoveFile,
  OpenInFinder,
  ExpandPath,
  EventsOn,
  ReadFile,
  SearchContentWithOptions,
  SearchFilenameWithOptions,
  SearchReplaceAll,
} from "../lib/wails";
import { getFileIcon } from "../lib/file-icons";
import { useVirtualizer } from "@tanstack/react-virtual";
import { useEditorStore } from "../hooks/store";
import {
  globalOpenFile,
  syncExternalFileChange,
  syncExternalDelete,
  syncExternalRename,
} from "../panels/editor";
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
  IconRefresh,
  IconLayoutSidebarLeftCollapse,
  IconSearch,
  IconReplace,
  IconChevronRight,
  IconSettings,
} from "@tabler/icons-react";

interface SidebarProps {
  folders: string[];
  onRefreshWorkspace: () => void;
  collapsed: boolean;
  onToggleCollapse: (collapsed: boolean) => void;
  onCreateShell: () => void;
  onCreateAgent: () => void;
  onOpenSession?: () => void;
  onOpenSettings?: () => void;
}

interface FileNode {
  name: string;
  path: string;
  isDir: boolean;
  children?: FileNode[];
  hidden: boolean;
  gitIgnored?: boolean;
  gitStatus?: string;
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

// Compare two children arrays by path — used to decide whether a dir node
// needs a fresh children list without breaking row identity on every refresh.
function sameChildren(a?: FileNode[], b?: FileNode[]): boolean {
  if (!a || !b) return false;
  if (a.length !== b.length) return false;
  for (let i = 0; i < a.length; i++) {
    if (a[i].path !== b[i].path) return false;
  }
  return true;
}

// Merge a freshly fetched tree into the current one, REUSING existing node
// objects for unchanged paths. Keeps row identity stable so React skips
// re-rendering untouched rows and children arrays stay intact (no height
// collapse / scroll jump / flicker on refresh). Type changes (dir<->file at
// same path) fall through to the new node.
// Metadata changes (git badge / ignored) clone the node with fresh fields so
// refresh actually updates badges.
function mergeTrees(prev: FileNode[], next: FileNode[]): FileNode[] {
  const byPath = new Map(prev.map((n) => [n.path, n]));
  return next.map((n) => {
    const p = byPath.get(n.path);
    if (!p || p.isDir !== n.isDir) return n;
    if (
      p.name === n.name &&
      p.gitStatus === n.gitStatus &&
      p.gitIgnored === n.gitIgnored &&
      p.hidden === n.hidden
    ) {
      // Dir with fresh children in this fetch: swap them in so
      // renamed/created/deleted entries actually appear/disappear, but keep
      // the old object when unchanged to preserve row identity.
      if (p.isDir && n.children && !sameChildren(p.children, n.children)) {
        return { ...p, children: n.children };
      }
      return p;
    }
    // Metadata changed: clone with fresh fields, keep cached children when
    // the fetch didn't include them (deep dirs below the depth limit).
    return { ...p, ...n, children: n.children ?? p.children };
  });
}

// Same ordering as internal/explorer: dirs first, then case-sensitive byte order.
function compareTreeNodes(a: FileNode, b: FileNode): number {
  if (a.isDir !== b.isDir) return a.isDir ? -1 : 1;
  return a.name < b.name ? -1 : a.name > b.name ? 1 : 0;
}

// Optimistic in-place mutations so the explorer updates instantly, before the
// (now fast) full refetch reconciles.
function insertNode(nodes: FileNode[], parentPath: string, newNode: FileNode): FileNode[] {
  return nodes.map((node) => {
    if (node.path === parentPath && node.isDir) {
      return { ...node, children: [...(node.children ?? []), newNode].sort(compareTreeNodes) };
    }
    if (node.isDir && node.children) {
      return { ...node, children: insertNode(node.children, parentPath, newNode) };
    }
    return node;
  });
}

function removeNode(nodes: FileNode[], path: string): FileNode[] {
  return nodes
    .filter((n) => n.path !== path)
    .map((n) => (n.isDir && n.children ? { ...n, children: removeNode(n.children, path) } : n));
}

// Rename a node — and, for dirs, every descendant path (oldPath/foo → newPath/foo).
function renameNode(nodes: FileNode[], oldPath: string, newPath: string, name: string): FileNode[] {
  return nodes.map((node) => {
    if (node.path === oldPath || node.path.startsWith(oldPath + "/")) {
      const suffix = node.path.slice(oldPath.length);
      return {
        ...node,
        name: node.path === oldPath ? name : node.name,
        path: newPath + suffix,
        children: node.children ? renameNode(node.children, oldPath, newPath, name) : node.children,
      };
    }
    if (node.isDir && node.children) {
      return { ...node, children: renameNode(node.children, oldPath, newPath, name) };
    }
    return node;
  });
}

// Paths that should NOT trigger a tree refresh (bulk install / build ops)
const SKIP_REFRESH_SEGMENTS = new Set([
  "node_modules", ".git", "pods", ".gradle", "gradle",
  ".dart_tool", ".pub-cache", ".pub", "__pycache__",
  "target", "vendor", "dist", "build", ".next",
  ".xcworkspace", ".xcodeproj", "xcbuilddata", "deriveddata",
  ".idea", ".vscode", ".build", ".swiftpm",
]);

function isSkippedPath(p: string): boolean {
  return p.split(/[\/\\]/).some((seg) => SKIP_REFRESH_SEGMENTS.has(seg.toLowerCase()));
}

// Git badge shown next to file rows: U = untracked/added (green), M = modified
// (blue), D = deleted (red). Mirrors VS Code source-control decorations.
function renderGitFileBadge(status?: string) {
  if (!status) return null;
  const cfg: Record<string, { cls: string; label: string }> = {
    U: { cls: "text-emerald-400 border-emerald-500/40", label: "U" },
    M: { cls: "text-sky-400 border-sky-500/40", label: "M" },
    D: { cls: "text-red-400 border-red-500/40", label: "D" },
  };
  const c = cfg[status];
  if (!c) return null;
  return (
    <span
      className={`text-[9px] font-bold border rounded-sm px-0.5 leading-tight shrink-0 ${c.cls}`}
      title={
        status === "U"
          ? "Untracked / added"
          : status === "M"
            ? "Modified"
            : "Deleted"
      }
    >
      {c.label}
    </span>
  );
}

export const Sidebar = memo(function Sidebar({
  folders,
  onRefreshWorkspace,
  collapsed,
  onToggleCollapse,
  onCreateShell,
  onCreateAgent,
  onOpenSession,
  onOpenSettings,
}: SidebarProps) {
  const [activeTab, setActiveTab] = useState<"explorer" | "search" | "git">("explorer");
  const searchInputRef = useRef<HTMLInputElement>(null);
  // Focus the search input whenever the search tab is opened.
  useEffect(() => {
    if (activeTab !== "search") return;
    const t = setTimeout(() => searchInputRef.current?.focus(), 80);
    return () => clearTimeout(t);
  }, [activeTab]);
  const [tree, setTree] = useState<FileNode[]>([]);
  const [renamingPath, setRenamingPath] = useState<string | null>(null);
  const [expandedDirs, setExpandedDirs] = useState<Record<string, boolean>>({});
  const expandedDirsRef = useRef(expandedDirs);
  useEffect(() => { expandedDirsRef.current = expandedDirs; }, [expandedDirs]);
  const [showHidden, setShowHidden] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [sessions, setSessions] = useState<any[]>([]);
  const [searchQuery, setSearchQuery] = useState("");
  const [searchMatchCase, setSearchMatchCase] = useState(false);
  const [searchWholeWord, setSearchWholeWord] = useState(false);
  const [searchRegex, setSearchRegex] = useState(true);
  const [searchScope, setSearchScope] = useState<"name" | "content" | "both">("both");
  const [searchIncludeFolder, setSearchIncludeFolder] = useState("");
  const [searchExcludeFolder, setSearchExcludeFolder] = useState("");
  const [searchReplace, setSearchReplace] = useState("");
  const [searchPreserveCase, setSearchPreserveCase] = useState(false);
  const [replaceFeedback, setReplaceFeedback] = useState("");
  const [searchExpanded, setSearchExpanded] = useState<Set<string>>(new Set());
  const [searchStats, setSearchStats] = useState({ totalMatches: 0, totalFiles: 0 });
  const [searchNonce, setSearchNonce] = useState(0);
  const [searchResults, setSearchResults] = useState<Array<{ path: string; name: string; line?: number; preview?: string }>>([]);
  const searchTokenRef = useRef(0);

  // Kill-session confirmation modal state
  const [killConfirm, setKillConfirm] = useState<{ id: string; name: string; type: string } | null>(null);
  const [renameSessionTarget, setRenameSessionTarget] = useState<{ id: string; name: string; type: string } | null>(null);
  const [renameSessionValue, setRenameSessionValue] = useState("");

  // Vertical resizer state — session manager uses fixed pixel height so the
  // explorer keeps the rest and the session manager never collapses out of view.
  const [sessionsHeight, setSessionsHeight] = useState(200);
  const [isDraggingSessions, setIsDraggingSessions] = useState(false);

  // Context Menu State
  const [contextMenu, setContextMenu] = useState<{
    x: number;
    y: number;
    node: FileNode;
  } | null>(null);

  // Delete confirmation state
  const [deleteConfirm, setDeleteConfirm] = useState<FileNode | null>(null);

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

  const { files, activeFileIndex, setFiles, setActiveFileIndex } = useEditorStore();

  // Ref to hold the debounce timer for tree refresh
  const treeRefreshTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  // Deferred tab closes for deleted paths. A delete may be the old half of a
  // rename/move (delete old path + create new path arrive as two events); the
  // tab's content is kept so a create with matching content can be detected as
  // a rename and the tab updated in place instead of closed.
  const pendingDeleteRef = useRef<
    Map<string, { timer: ReturnType<typeof setTimeout>; content: string | null }>
  >(new Map());
  // Recently created paths (path -> timestamp). macOS reports a rename as
  // create-then-delete; this buffer lets a delete find its rename target.
  const recentCreateRef = useRef<Map<string, number>>(new Map());

  const loadTree = useCallback(async () => {
    try {
      const dataStr = await GetFileTree(2);
      const data = JSON.parse(dataStr);
      const nodes = Array.isArray(data) ? data : [];

      // Build the refreshed tree entirely in a local variable first, then
      // commit it in ONE render pass. Never swap in a half-fetched tree, so
      // expanded dirs never collapse mid-refresh (no flicker).
      let cur = nodes;

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
      // Restore children for EVERY expanded dir (deep ones too) so a refresh
      // never collapses sub-sub-folders. ExpandPath returns one level at a time,
      // so walk the whole tree until all expanded dirs have children again.
      // Fetch each level's missing children in parallel — one sequential IPC
      // round-trip per dir used to make every refresh crawl on wide trees.
      let level = nodes;
      while (level.length) {
        // Always re-fetch EVERY expanded dir's children — even ones that
        // already have cached children — so renamed/created/deleted files
        // inside expanded subfolders never stay stale across refreshes.
        const missing = level.filter(
          (n) => n.isDir && expandedDirsRef.current[n.path]
        );
        const results = await Promise.all(
          missing.map((n) => ExpandPath(n.path).then(JSON.parse).catch(() => null))
        );
        const next: FileNode[] = [];
        for (let i = 0; i < missing.length; i++) {
          if (Array.isArray(results[i])) next.push(...results[i]);
        }
        // Merge this level into the local tree (no render yet).
        for (let i = 0; i < missing.length; i++) {
          if (Array.isArray(results[i])) {
            cur = updateNodeChildren(cur, missing[i].path, results[i]);
          }
        }
        // Descend into dirs that already have children (expanded or not) so
        // deeper expanded dirs stay open too.
        const fetched = new Set(missing.map((n) => n.path));
        for (const n of level) {
          if (n.isDir && !fetched.has(n.path)) next.push(...(n.children ?? []));
        }
        level = next;
      }
      // Single commit. Reuse unchanged nodes -> rows keep DOM, scroll stays put.
      setTree((prev) => (prev.length ? mergeTrees(prev, cur) : cur));
    } catch (err) {
      console.error("Failed to load file tree:", err);
    }
  }, []);

  // Debounced tree refresh — coalesces bursts (npm install, git pulls) but
  // still feels instant for single file create/delete ops.
  const scheduledTreeRefresh = useCallback(() => {
    if (treeRefreshTimer.current) clearTimeout(treeRefreshTimer.current);
    treeRefreshTimer.current = setTimeout(() => {
      loadTree();
    }, 200);
  }, [loadTree]);

  const handleRefreshClick = useCallback(async () => {
    setRefreshing(true);
    try {
      onRefreshWorkspace();
      await loadTree();
    } finally {
      setRefreshing(false);
    }
  }, [onRefreshWorkspace, loadTree]);

  useEffect(() => {
    if (folders && folders.length > 0) {
      loadTree();
    }
  }, [folders, showHidden, loadTree]);

  // Subscribe to backend fs:changed events for real-time sync
  useEffect(() => {
    const unsub = EventsOn(
      "fs:changed",
      (data: { type: string; path: string; oldPath?: string }) => {
        if (!data?.path) return;

        // Skip bulk-install paths entirely to avoid flicker
        if (isSkippedPath(data.path)) return;

        // Explicit rename (paired old+new, emitted by the backend): follow any
        // open tab from the old path, and cancel deferred closes on both sides.
        if (data.type === "renamed" && data.oldPath && data.oldPath !== data.path) {
          const pending = pendingDeleteRef.current;
          for (const p of [data.oldPath, data.path]) {
            const t = pending.get(p);
            if (t) {
              clearTimeout(t.timer);
              pending.delete(p);
            }
          }
          syncExternalRename(data.oldPath, data.path);
          scheduledTreeRefresh();
          return;
        }

        // Create: detect rename/move by content-matching against recently
        // deleted tabs (Linux reports rename as delete + create). Also record
        // the path so a later delete (macOS create-then-delete order) can find
        // its rename target. Then sync content if the path is open.
        if (data.type === "created") {
          scheduledTreeRefresh();
          recentCreateRef.current.set(data.path, Date.now());
          const openFiles = useEditorStore.getState().files;
          const isOpen = openFiles.some((f) => f.path === data.path);
          const pending = pendingDeleteRef.current;
          const candidates = [...pending.entries()].filter(
            ([, info]) => info.content !== null
          );
          if (!isOpen && candidates.length === 0) return;
          ReadFile(data.path)
            .then((content) => {
              for (const [oldPath, info] of candidates) {
                if (info.content === content && oldPath !== data.path) {
                  clearTimeout(info.timer);
                  pending.delete(oldPath);
                  syncExternalRename(oldPath, data.path);
                  return;
                }
              }
              if (isOpen) syncExternalFileChange(data.path, content, true);
            })
            .catch(() => {});
          return;
        }

        // Delete: a delete may be the old half of a rename/move. On macOS the
        // create arrives first, so first try to match against a recent create
        // with identical content; otherwise defer the tab close briefly so a
        // follow-up create (Linux order) can update the tab in place.
        if (data.type === "deleted") {
          scheduledTreeRefresh();
          const tab = useEditorStore
            .getState()
            .files.find((f) => f.type === "file" && f.path === data.path);
          const tabContent = tab?.content ?? null;

          // create-first order: find a recent create with the same content.
          if (tabContent !== null) {
            const now = Date.now();
            for (const [createPath, ts] of recentCreateRef.current) {
              if (now - ts > 2000) {
                recentCreateRef.current.delete(createPath);
                continue;
              }
              recentCreateRef.current.delete(createPath);
              const p = createPath;
              ReadFile(p)
                .then((content) => {
                  if (content === tabContent && p !== data.path) {
                    syncExternalRename(data.path, p);
                  } else {
                    syncExternalDelete(data.path);
                  }
                })
                .catch(() => syncExternalDelete(data.path));
              return;
            }
          }

          // delete-first order: defer the close; a create with matching
          // content (handled in the created branch) updates the tab instead.
          const pending = pendingDeleteRef.current;
          if (!pending.has(data.path)) {
            const info = {
              content: tabContent,
              timer: setTimeout(() => {
                pending.delete(data.path);
                syncExternalDelete(data.path);
              }, 700),
            };
            pending.set(data.path, info);
          }
          return;
        }

        // Refresh the file tree (debounced)
        scheduledTreeRefresh();

        // Sync content for modified events. `created` is handled above with
        // force=true to cover atomic saves (nano/vim rename+create cycle).
        if (data.type === "modified") {
          // Only fetch content if the file is actually open — avoids reading
          // the full content of every file the build/watcher touches.
          const openFiles = useEditorStore.getState().files;
          if (!openFiles.some((f) => f.path === data.path)) return;
          ReadFile(data.path)
            .then((content) => syncExternalFileChange(data.path, content, false))
            .catch(() => {});
        }
      }
    );
    return () => {
      unsub?.();
      pendingDeleteRef.current.forEach((info) => clearTimeout(info.timer));
      pendingDeleteRef.current.clear();
      recentCreateRef.current.clear();
    };
  }, [scheduledTreeRefresh]);

  // Safety net: the push-based fs:changed events are the primary sync path.
  // If any are dropped (event bridge hiccup), poll the backend change counter
  // so the explorer + open tabs still catch up on internal/external changes.
  useEffect(() => {
    let cancelled = false;
    let timer: ReturnType<typeof setTimeout>;
    let last = -1;
    const tick = async () => {
      if (cancelled) return;
      // Trim stale create records used for rename detection.
      const now = Date.now();
      for (const [p, ts] of recentCreateRef.current) {
        if (now - ts > 3000) recentCreateRef.current.delete(p);
      }
      try {
        const cur = await GetFsChangeCount();
        if (last !== -1 && cur !== last) {
          scheduledTreeRefresh();
          const openFiles = useEditorStore
            .getState()
            .files.filter((f) => f.type === "file");
          for (const f of openFiles) {
            ReadFile(f.path)
              .then((content) => syncExternalFileChange(f.path, content, false))
              .catch(() => {});
          }
        }
        last = cur;
      } catch {
        /* backend unavailable — try again next tick */
      }
      timer = window.setTimeout(tick, 2000);
    };
    timer = window.setTimeout(tick, 2000);
    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [scheduledTreeRefresh]);

  // Refresh the tree (and thus git badges) after git mutations like stage/revert.
  useEffect(() => {
    const handler = () => scheduledTreeRefresh();
    window.addEventListener("forge:git-status-changed", handler);
    return () => window.removeEventListener("forge:git-status-changed", handler);
  }, [scheduledTreeRefresh]);

  useEffect(() => {
    loadSessions();
    // Reload when the workspace changes (project-scoped session history).
    // No polling — agent + terminal events keep it fresh.
    const unsubs = [
      "agent:updated",
      "agent:turn_start",
      "agent:turn_end",
      "agent:message_start",
      "agent:message_end",
      "agent:tool_end",
      "agent:ask",
      "session:opened",
      "session:closed",
    ].map((ev) => EventsOn(ev, loadSessions));
    return () => {
      unsubs.forEach((u) => typeof u === "function" && u());
    };
  }, [folders]);

  // Closes context menu on click elsewhere
  useEffect(() => {
    const handleWindowClick = () => {
      setContextMenu(null);
    };
    window.addEventListener("click", handleWindowClick);
    return () => window.removeEventListener("click", handleWindowClick);
  }, []);

  async function loadSessions() {
    try {
      const shellList = await ListSessions();
      // Agent sessions are project-scoped: only show history for the current
      // workspace folder (so sessions from other projects stay hidden).
      const projectFolder = folders?.[0] ?? "";
      const agentList = projectFolder
        ? await ListAgentSessionsForFolder(projectFolder)
        : await ListAgentSessions();

      const merged = [];
      const seen = new Set();
      for (const s of shellList) {
        merged.push({ ...s, type: "shell" });
        seen.add(s.id);
      }
      for (const a of agentList) {
        if (seen.has(a.id)) continue;
        merged.push({ ...a, type: "agent" });
        seen.add(a.id);
      }
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
    onOpenSession?.();
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

  const requestKillSession = (s: any, e: React.MouseEvent) => {
    e.stopPropagation();
    setKillConfirm({ id: s.id, name: s.name || s.id, type: s.type });
  };

  const requestRenameSession = (s: any, e: React.MouseEvent) => {
    e.stopPropagation();
    setRenameSessionTarget({ id: s.id, name: s.name || s.id, type: s.type });
    setRenameSessionValue(s.name || "");
  };

  const confirmRenameSession = async () => {
    if (!renameSessionTarget) return;
    const name = renameSessionValue.trim();
    if (!name) return;
    try {
      if (renameSessionTarget.type === "agent") {
        await RenameAgentSession(renameSessionTarget.id, name);
      } else {
        await RenameSession(renameSessionTarget.id, name);
      }
      setRenameSessionTarget(null);
      setRenameSessionValue("");
      loadSessions();
    } catch (err) {
      console.error("Failed to rename session:", err);
    }
  };

  const confirmKillSession = async () => {
    if (!killConfirm) return;
    const { id, type } = killConfirm;
    setKillConfirm(null);
    try {
      if (type === "agent") {
        await DeleteAgentSession(id);
      } else {
        await StopSession(id);
      }
      // Close any editor/panel tab hosting the killed session.
      const store = useEditorStore.getState();
      const kept = store.files.filter((f: any) => !(f.id === id && (f.type === "shell" || f.type === "agent")));
      if (kept.length !== store.files.length) {
        store.setFiles(kept);
        if (store.activeFileIndex >= kept.length) {
          store.setActiveFileIndex(Math.max(0, kept.length - 1));
        }
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
        setSessionsHeight(Math.max(120, Math.min(rect.height - 120, nextHeight)));
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

  // Inline rename (no modal): edit the name right in the tree row.
  const startInlineRename = (node: FileNode) => {
    setRenamingPath(node.path);
  };

  const commitInlineRename = async (node: FileNode, newName: string) => {
    const name = newName.trim();
    setRenamingPath(null);
    if (!name || name === node.name) return;
    try {
      const dir = node.path.split(/[/\\]/).slice(0, -1).join("/");
      const newPath = (dir ? dir + "/" : "/") + name;
      await RenameFile(node.path, newPath);
      // Keep the renamed dir (and its expanded children) open across the refresh.
      setExpandedDirs((prev) => {
        const next: Record<string, boolean> = {};
        for (const [path, val] of Object.entries(prev)) {
          const key = path === node.path || path.startsWith(node.path + "/")
            ? newPath + path.slice(node.path.length)
            : path;
          next[key] = val;
        }
        return next;
      });
      // Optimistic: rename in place instantly, then reconcile with the refetch.
      setTree((prev) => renameNode(prev, node.path, newPath, name));
      loadTree();
    } catch (err) {
      console.error("Rename failed:", err);
    }
  };

  const handleCopyFileRef = (node: FileNode) => {
    clipboardPathRef.current = node.path;
  };

  const handlePasteFile = async (targetDir: string) => {
    try {
      // 1. System clipboard (files copied from Finder / outside workspace).
      const clipFiles = await GetClipboardFiles();
      if (clipFiles.length) {
        for (const src of clipFiles) {
          const name = src.split(/[/\\]/).pop();
          if (!name) continue;
          await CopyPath(src, targetDir + "/" + name);
        }
        loadTree();
        return;
      }
      // 2. Fallback: in-app copy (Copy File context action).
      const src = clipboardPathRef.current;
      if (!src) return;
      const name = src.split(/[/\\]/).pop();
      if (!name) return;
      await CopyPath(src, targetDir + "/" + name);
      loadTree();
    } catch (err) {
      alert("Paste failed: " + err);
    }
  };

  const handleDeleteNode = async (node: FileNode) => {
    setDeleteConfirm(node);
  };

  const handleDeleteConfirmed = async () => {
    const node = deleteConfirm;
    if (!node) return;
    setDeleteConfirm(null);
    try {
      await DeleteFile(node.path);
      // Close any editor tab whose path is under the deleted node.
      const prefix = node.isDir ? node.path + "/" : node.path;
      const files = useEditorStore.getState().files;
      const kept = files.filter((f: any) => {
        const p = f.path || f.Path || "";
        return !(p === node.path || p.startsWith(prefix));
      });
      if (kept.length !== files.length) {
        useEditorStore.getState().setFiles(kept);
        const idx = useEditorStore.getState().activeFileIndex;
        if (idx >= kept.length) useEditorStore.getState().setActiveFileIndex(Math.max(0, kept.length - 1));
      }
      // Optimistic: drop the node instantly, then reconcile with the refetch.
      setTree((prev) => removeNode(prev, node.path));
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
        await CreateFolder(modalPrompt.dirPath + "/" + val);
      } else if (modalPrompt.type === "rename" && modalPrompt.oldPath) {
        await RenameFile(modalPrompt.oldPath, modalPrompt.dirPath + "/" + val);
      }
      // Optimistic: show the new/renamed node instantly, then reconcile.
      if (modalPrompt.type === "createFile" || modalPrompt.type === "createFolder") {
        if (!(val.startsWith(".") && !showHidden)) {
          setTree((prev) =>
            insertNode(prev, modalPrompt.dirPath, {
              name: val,
              path: modalPrompt.dirPath + "/" + val,
              isDir: modalPrompt.type === "createFolder",
              children: modalPrompt.type === "createFolder" ? [] : undefined,
              hidden: val.startsWith("."),
            })
          );
        }
      } else if (modalPrompt.type === "rename" && modalPrompt.oldPath) {
        const oldPath = modalPrompt.oldPath;
        setTree((prev) => renameNode(prev, oldPath, modalPrompt.dirPath + "/" + val, val));
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

  // Derive the active file path from the editor store for highlighting
  const activeFilePath = files[activeFileIndex]?.path ?? null;

  // Flatten visible tree into rows (dir children expanded per expandedDirs) so
  // the tree can be virtualized — huge projects were rendering thousands of DOM
  // nodes on every refresh/expand.
  const visibleRows = useMemo(() => {
    const rows: { node: FileNode; depth: number }[] = [];
    const walk = (nodes: FileNode[], depth: number) => {
      for (const node of nodes) {
        if (node.hidden && !showHidden) continue;
        rows.push({ node, depth });
        if (node.isDir && expandedDirs[node.path] && node.children) {
          walk(node.children, depth + 1);
        }
      }
    };
    walk(tree, 0);
    return rows;
  }, [tree, expandedDirs, showHidden]);

  const treeScrollRef = useRef<HTMLDivElement | null>(null);
  const virtualizer = useVirtualizer({
    count: visibleRows.length,
    getScrollElement: () => treeScrollRef.current,
    estimateSize: () => 22,
    overscan: 12,
    getItemKey: (i) => visibleRows[i].node.path,
  });

  const renderRow = ({ node, depth }: { node: FileNode; depth: number }) => {
    const isExpanded = !!expandedDirs[node.path];
    const indentStyle = { paddingLeft: `${depth * 8 + 8}px` };
    const ignoredDim = node.gitIgnored ? " opacity-50" : "";

    if (node.isDir) {
      return (
        <div
          key={node.path}
          data-file-row
          onClick={() => toggleDir(node.path)}
          onContextMenu={(e) => handleNodeContextMenu(e, node)}
          onDragOver={(e) => handleDragOver(e, node)}
          onDrop={(e) => handleDropNode(e, node)}
          style={indentStyle}
          className={"flex items-center gap-1.5 py-0.5 text-xs text-[var(--fg-secondary)] hover:text-[var(--fg-primary)] hover:bg-[var(--bg-surface-hover)] cursor-pointer select-none group" + ignoredDim}
          draggable
          onDragStart={(e) => handleDragStart(e, node)}
        >
          {isExpanded ? (
            <IconFolderOpen className="size-3.5 text-amber-400 shrink-0" />
          ) : (
            <IconFolder className="size-3.5 text-amber-400 shrink-0" />
          )}
          {node.path === renamingPath ? (
            <input
              autoFocus
              defaultValue={node.name}
              className="w-full min-w-0 bg-[var(--bg-panel)] border border-[var(--accent)] rounded px-1 py-0 text-xs text-[var(--fg-primary)] focus:outline-none"
              draggable={false}
              onClick={(e) => e.stopPropagation()}
              onKeyDown={(e) => {
                e.stopPropagation();
                if (e.key === "Enter") commitInlineRename(node, e.currentTarget.value);
                else if (e.key === "Escape") setRenamingPath(null);
              }}
              onBlur={() => setRenamingPath(null)}
            />
          ) : (
            <span className="truncate">{node.name}</span>
          )}
          {node.gitStatus && (
            <span
              className="inline-block size-1.5 rounded-full bg-emerald-400 shrink-0"
              title="Contains uncommitted changes"
            />
          )}
          {node.gitIgnored && (
            <span className="text-[9px] uppercase tracking-wide text-[var(--fg-tertiary)] opacity-60 shrink-0" title="Gitignored">
              gitignored
            </span>
          )}
        </div>
      );
    }

    const isActive = activeFilePath === node.path;

    return (
      <div
        key={node.path}
        data-file-row
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
        className={cn(
          "flex items-center gap-1.5 py-0.5 text-xs cursor-pointer select-none transition-colors",
          isActive
            ? "explorer-file-active"
            : "text-[var(--fg-secondary)] hover:text-[var(--fg-primary)] hover:bg-[var(--bg-surface-hover)]",
          ignoredDim
        )}
        draggable
        onDragStart={(e) => handleDragStart(e, node)}
      >
        {getFileIcon(node.name, "size-3.5 shrink-0")}
        {node.path === renamingPath ? (
          <input
            autoFocus
            defaultValue={node.name}
            className="w-full min-w-0 bg-[var(--bg-panel)] border border-[var(--accent)] rounded px-1 py-0 text-xs text-[var(--fg-primary)] focus:outline-none"
            draggable={false}
            onClick={(e) => e.stopPropagation()}
            onKeyDown={(e) => {
              e.stopPropagation();
              if (e.key === "Enter") commitInlineRename(node, e.currentTarget.value);
              else if (e.key === "Escape") setRenamingPath(null);
            }}
            onBlur={() => setRenamingPath(null)}
          />
        ) : (
          <span className="truncate">{node.name}</span>
        )}
        {renderGitFileBadge(node.gitStatus)}
        {node.gitIgnored && (
          <span className="text-[9px] uppercase tracking-wide text-[var(--fg-tertiary)] shrink-0" title="Gitignored">
            gitignored
          </span>
        )}
      </div>
    );
  };

  const collectFileNodes = (nodes: FileNode[], out: FileNode[] = []) => {
    for (const node of nodes) {
      if (node.hidden && !showHidden) continue;
      if (node.isDir) {
        if (node.children) collectFileNodes(node.children, out);
      } else {
        out.push(node);
      }
    }
    return out;
  };

  const pathMatchesFolderRule = (path: string, pattern: string, regexMode: boolean) => {
    const value = pattern.trim();
    if (!value) return true;
    const rel = path.replace(/\\/g, "/");
    try {
      const re = new RegExp(value, "i");
      return regexMode ? re.test(rel) : rel.toLowerCase().includes(value.toLowerCase());
    } catch {
      return false;
    }
  };

  useEffect(() => {
    const q = searchQuery.trim();
    const token = ++searchTokenRef.current;
    if (!q) {
      setSearchResults([]);
      setSearchStats({ totalMatches: 0, totalFiles: 0 });
      return;
    }

    const delay = window.setTimeout(async () => {
      let results: Array<{ path: string; name: string; line?: number; preview?: string }> = [];
      try {
        const opts = {
          query: q,
          matchCase: searchMatchCase,
          matchWholeWord: searchWholeWord,
          useRegex: searchRegex,
          limit: 5000,
        };
        if (searchScope !== "content") {
          const nameResults = await SearchFilenameWithOptions(opts as any);
          results.push(
            ...nameResults
              .map((r: any) => ({
                path: r.path ?? r.Path,
                name: r.filename ?? r.Filename ?? (r.path ?? r.Path)?.split("/").pop() ?? "file",
              }))
              .filter((r: any) => r.path && pathMatchesFolderRule(r.path, searchIncludeFolder, false) && (!searchExcludeFolder.trim() || !pathMatchesFolderRule(r.path, searchExcludeFolder, true)))
          );
        }
        if (searchScope !== "name") {
          const contentResults = await SearchContentWithOptions(opts as any);
          results.push(
            ...contentResults
              .map((r: any) => ({
                path: r.path ?? r.Path,
                name: r.filename ?? r.Filename ?? (r.path ?? r.Path)?.split("/").pop() ?? "file",
                line: r.line ?? r.Line,
                preview: r.content ?? r.Content ?? "",
              }))
              .filter((r: any) => r.path && pathMatchesFolderRule(r.path, searchIncludeFolder, false) && (!searchExcludeFolder.trim() || !pathMatchesFolderRule(r.path, searchExcludeFolder, true)))
          );
        }
      } catch {
        results = [];
      }
      // Keep every distinct line match (same path + line collapses duplicates)
      // so the accordion can show all hits per file with a count.
      const seen = new Set<string>();
      results = results.filter((r) => {
        const key = r.path + ":" + (r.line ?? 0);
        if (seen.has(key)) return false;
        seen.add(key);
        return true;
      });
      results.sort((a, b) => a.path.localeCompare(b.path) || (a.line ?? 0) - (b.line ?? 0));
      if (results.length > 5000) results = results.slice(0, 5000);
      if (token === searchTokenRef.current) {
        setSearchResults(results);
        const fileSet = new Set(results.map((r) => r.path));
        setSearchStats({
          totalMatches: results.length,
          totalFiles: fileSet.size,
        });
        // Expand every file accordion by default for a fresh search.
        setSearchExpanded(fileSet);
      }
    }, 250);

    return () => window.clearTimeout(delay);
  }, [searchQuery, searchMatchCase, searchWholeWord, searchRegex, searchScope, searchIncludeFolder, searchExcludeFolder, searchNonce, tree, showHidden]);

  // Replace All: replace every match of the current query across all files.
  const handleReplaceAll = async () => {
    const q = searchQuery.trim();
    if (!q) return;
    try {
      setReplaceFeedback("Replacing…");
      const res = await SearchReplaceAll({
        query: q,
        matchCase: searchMatchCase,
        matchWholeWord: searchWholeWord,
        useRegex: searchRegex,
        replacement: searchReplace,
        preserveCase: searchPreserveCase,
      } as any);
      setReplaceFeedback(
        res?.totalReplacements
          ? `${res.totalReplacements} replacement${res.totalReplacements === 1 ? "" : "s"} in ${res.filesChanged} file${res.filesChanged === 1 ? "" : "s"}`
          : "No matches to replace"
      );
      setSearchNonce((v) => v + 1); // re-run search so results reflect the new content
    } catch {
      setReplaceFeedback("Replace failed");
    }
  };

  const toggleSearchExpanded = (path: string) => {
    setSearchExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(path)) next.delete(path);
      else next.add(path);
      return next;
    });
  };

  // Group search results per file for the accordion view.
  const searchGroups = useMemo(() => {
    const m = new Map<
      string,
      { path: string; name: string; lines: Array<{ line: number; preview?: string }>; nameMatch: boolean }
    >();
    for (const r of searchResults) {
      let g = m.get(r.path);
      if (!g) {
        g = { path: r.path, name: r.name, lines: [], nameMatch: false };
        m.set(r.path, g);
      }
      if (r.line !== undefined) g.lines.push({ line: r.line, preview: r.preview });
      else g.nameMatch = true;
    }
    return Array.from(m.values());
  }, [searchResults]);

  // The sidebar icon dock is always visible (40px). When collapsed, clicking
  // an icon expands the panel. When expanded, double-clicking an icon collapses it.

  return (
    <div className="flex h-full w-full bg-[var(--bg-sidebar)] border-r border-[var(--border-default)] shrink-0 select-none font-sans overflow-hidden relative">
      {/* Icon Switcher Dock */}
      <div className="w-10 border-r border-[var(--border-default)] bg-[var(--bg-panel)] flex flex-col items-center py-2 gap-3 shrink-0 overflow-y-auto">
        <button
          onClick={() => {
            if (collapsed) {
              onToggleCollapse(false);
              setActiveTab("explorer");
            } else {
              setActiveTab("explorer");
            }
          }}
          onDoubleClick={() => { if (!collapsed) onToggleCollapse(true); }}
          className={cn(
            "p-1.5 rounded transition-all cursor-pointer",
            activeTab === "explorer" && !collapsed
              ? "text-[var(--accent-primary)] bg-[var(--bg-surface-active)]"
              : "text-[var(--fg-tertiary)] hover:text-[var(--fg-primary)]"
          )}
          title={collapsed ? "Expand Sidebar" : "File Explorer (double-click to hide)"}
        >
          <IconFiles className="size-5" />
        </button>
        <button
          onClick={() => {
            if (collapsed) {
              onToggleCollapse(false);
              setActiveTab("search");
            } else {
              setActiveTab("search");
            }
          }}
          onDoubleClick={() => { if (!collapsed) onToggleCollapse(true); }}
          className={cn(
            "p-1.5 rounded transition-all cursor-pointer",
            activeTab === "search" && !collapsed
              ? "text-[var(--accent-primary)] bg-[var(--bg-surface-active)]"
              : "text-[var(--fg-tertiary)] hover:text-[var(--fg-primary)]"
          )}
          title={collapsed ? "Expand Sidebar" : "Search Panel"}
        >
          <IconSearch className="size-5" />
        </button>
        <button
          onClick={() => {
            if (collapsed) {
              onToggleCollapse(false);
              setActiveTab("git");
            } else {
              setActiveTab("git");
            }
          }}
          onDoubleClick={() => { if (!collapsed) onToggleCollapse(true); }}
          className={cn(
            "p-1.5 rounded transition-all cursor-pointer",
            activeTab === "git" && !collapsed
              ? "text-[var(--accent-primary)] bg-[var(--bg-surface-active)]"
              : "text-[var(--fg-tertiary)] hover:text-[var(--fg-primary)]"
          )}
          title={collapsed ? "Expand Sidebar" : "Git Control (double-click to hide)"}
        >
          <IconGitBranch className="size-5" />
        </button>

        {/* Collapse / Expand sidebar button */}
        <div className="flex-1" />
        <button
          onClick={() => onOpenSettings?.()}
          className="p-1.5 rounded transition-all cursor-pointer text-[var(--fg-tertiary)] hover:text-white hover:bg-[var(--bg-surface-hover)]"
          title="Global Settings"
        >
          <IconSettings className="size-5" />
        </button>
        <button
          onClick={() => onToggleCollapse(!collapsed)}
          className="p-1.5 rounded transition-all cursor-pointer text-[var(--fg-tertiary)] hover:text-white hover:bg-[var(--bg-surface-hover)]"
          title={collapsed ? "Expand Sidebar" : "Hide Sidebar"}
        >
          <IconLayoutSidebarLeftCollapse
            className="size-4"
            style={{ transform: collapsed ? "scaleX(-1)" : undefined }}
          />
        </button>
      </div>

      {/* Panel view */}
      <div
        className="flex-1 flex flex-col min-w-0 min-h-0 sidebar-panel-container"
        onDoubleClick={(e) => {
          // Double-click on empty space hides the sidebar. Ignore double-clicks
          // on interactive elements (buttons, inputs, file rows).
          const target = e.target as HTMLElement;
          if (target.closest("button, input, select, textarea, a")) return;
          if (target.closest("[data-file-row]")) return;
          onToggleCollapse(!collapsed);
        }}
      >
        {activeTab === "git" ? (
          <GitPanel />
        ) : activeTab === "search" ? (
          <div className="flex-1 flex flex-col min-w-0 min-h-0">
            <div className="flex flex-col gap-1.5 px-3 py-2 border-b border-[var(--border-default)] shrink-0 bg-[var(--bg-panel)]">
              <div className="flex items-center justify-between">
                <span className="text-[10px] font-bold uppercase tracking-wider text-[var(--fg-tertiary)]">
                  Search
                </span>
                <button
                  onClick={() => {
                    setSearchQuery("");
                    setSearchResults([]);
                    setSearchStats({ totalMatches: 0, totalFiles: 0 });
                  }}
                  className="p-1 hover:bg-[var(--bg-surface-hover)] rounded text-[var(--fg-tertiary)] hover:text-white cursor-pointer"
                  title="Clear Search"
                >
                  <IconX className="size-3.5" />
                </button>
              </div>

              {/* Search input with inline clear */}
              <div className="flex items-center gap-1.5 bg-[var(--bg-surface)] border border-[var(--border-default)] px-2 py-1 rounded focus-within:border-[var(--accent-primary)]">
                <IconSearch className="size-3.5 text-[var(--fg-tertiary)] shrink-0" />
                <input
                  ref={searchInputRef}
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Escape") setSearchQuery("");
                  }}
                  placeholder="Search"
                  className="flex-1 bg-transparent text-[11px] text-[var(--fg-primary)] font-mono outline-none placeholder:text-[var(--fg-tertiary)]"
                />
              </div>

              {/* Match options: case / whole word / regex (default on) / scope */}
              <div className="flex items-center gap-1.5">
                <button
                  onClick={() => setSearchMatchCase((v) => !v)}
                  title="Match Case"
                  className={`px-1.5 py-0.5 rounded border text-[10px] cursor-pointer ${
                    searchMatchCase
                      ? "bg-[var(--accent-primary)]/20 border-[var(--accent-primary)] text-[var(--accent-primary)]"
                      : "border-[var(--border-default)] text-[var(--fg-tertiary)] hover:text-white"
                  }`}
                >
                  Aa
                </button>
                <button
                  onClick={() => setSearchWholeWord((v) => !v)}
                  title="Match Whole Word"
                  className={`px-1.5 py-0.5 rounded border text-[10px] cursor-pointer ${
                    searchWholeWord
                      ? "bg-[var(--accent-primary)]/20 border-[var(--accent-primary)] text-[var(--accent-primary)]"
                      : "border-[var(--border-default)] text-[var(--fg-tertiary)] hover:text-white"
                  }`}
                >
                  ab
                </button>
                <button
                  onClick={() => setSearchRegex((v) => !v)}
                  title="Use Regular Expression"
                  className={`px-1.5 py-0.5 rounded border text-[10px] cursor-pointer ${
                    searchRegex
                      ? "bg-[var(--accent-primary)]/20 border-[var(--accent-primary)] text-[var(--accent-primary)]"
                      : "border-[var(--border-default)] text-[var(--fg-tertiary)] hover:text-white"
                  }`}
                >
                  .*
                </button>
                <select
                  value={searchScope}
                  onChange={(e) => setSearchScope(e.target.value as any)}
                  className="ml-auto bg-[var(--bg-surface)] border border-[var(--border-default)] rounded px-1.5 py-0.5 text-[10px] text-[var(--fg-primary)] outline-none"
                >
                  <option value="both">Name + content</option>
                  <option value="name">File names</option>
                  <option value="content">Content only</option>
                </select>
              </div>

              {/* Replace: input + preserve case + replace all */}
              <div className="flex items-center gap-1.5 bg-[var(--bg-surface)] border border-[var(--border-default)] px-2 py-1 rounded focus-within:border-[var(--accent-primary)]">
                <IconReplace className="size-3.5 text-[var(--fg-tertiary)] shrink-0" />
                <input
                  value={searchReplace}
                  onChange={(e) => setSearchReplace(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Escape") setSearchReplace("");
                    if (e.key === "Enter") handleReplaceAll();
                  }}
                  placeholder="Replace"
                  className="flex-1 bg-transparent text-[11px] text-[var(--fg-primary)] font-mono outline-none placeholder:text-[var(--fg-tertiary)]"
                />
                <button
                  onClick={() => setSearchPreserveCase((v) => !v)}
                  title="Preserve Case"
                  className={`px-1.5 py-0.5 rounded border text-[10px] cursor-pointer ${
                    searchPreserveCase
                      ? "bg-[var(--accent-primary)]/20 border-[var(--accent-primary)] text-[var(--accent-primary)]"
                      : "border-[var(--border-default)] text-[var(--fg-tertiary)] hover:text-white"
                  }`}
                >
                  AB
                </button>
                <button
                  onClick={handleReplaceAll}
                  disabled={!searchQuery.trim() || searchResults.length === 0}
                  className="p-1 rounded bg-[var(--accent-primary)] text-white hover:opacity-90 cursor-pointer disabled:opacity-40 disabled:cursor-default"
                  title="Replace All"
                >
                  <IconReplace className="size-3.5" />
                </button>
              </div>
              {replaceFeedback && (
                <div className="text-[10px] text-[var(--fg-secondary)]">{replaceFeedback}</div>
              )}

              {/* Files to include / exclude */}
              <div className="grid grid-cols-2 gap-2">
                <input
                  value={searchIncludeFolder}
                  onChange={(e) => setSearchIncludeFolder(e.target.value)}
                  placeholder="files to include"
                  className="w-full bg-[var(--bg-surface)] border border-[var(--border-default)] px-2 py-1 rounded text-[10px] text-[var(--fg-primary)] font-mono outline-none focus:border-[var(--accent-primary)]"
                />
                <input
                  value={searchExcludeFolder}
                  onChange={(e) => setSearchExcludeFolder(e.target.value)}
                  placeholder="files to exclude"
                  className="w-full bg-[var(--bg-surface)] border border-[var(--border-default)] px-2 py-1 rounded text-[10px] text-[var(--fg-primary)] font-mono outline-none focus:border-[var(--accent-primary)]"
                />
              </div>
            </div>

            {/* Results */}
            <div className="shrink-0 px-3 py-1.5 text-[11px] text-[var(--fg-secondary)] border-b border-[var(--border-subtle)] bg-[var(--bg-panel)]">
              {searchQuery.trim() &&
                (searchStats.totalFiles > 0
                  ? `${searchStats.totalMatches} result${searchStats.totalMatches === 1 ? "" : "s"} in ${searchStats.totalFiles} file${searchStats.totalFiles === 1 ? "" : "s"}`
                  : "No results")}
            </div>
            <div className="flex-1 min-h-0 overflow-y-auto py-1">
              {!searchQuery.trim() ? (
                <div className="px-3 py-2 text-[11px] text-[var(--fg-tertiary)]">Type to search</div>
              ) : (
                <>
                  {searchGroups.map((g) => {
                    const isOpen = searchExpanded.has(g.path);
                    const count = g.lines.length || (g.nameMatch ? 1 : 0);
                    return (
                      <div key={g.path}>
                        <button
                          onClick={() => toggleSearchExpanded(g.path)}
                          className="w-full flex items-center gap-1.5 px-2 py-1 hover:bg-[var(--bg-surface-hover)] cursor-pointer group"
                        >
                          <IconChevronRight
                            className={`size-3 text-[var(--fg-tertiary)] shrink-0 transition-transform ${isOpen ? "rotate-90" : ""}`}
                          />
                          {getFileIcon(g.name, "size-3.5 shrink-0")}
                          <span className="truncate text-[12px] text-[var(--fg-primary)]">{g.name}</span>
                          <span className="ml-auto shrink-0 text-[10px] text-[var(--fg-tertiary)]">
                            {count}
                          </span>
                        </button>
                        <div className="text-[10px] text-[var(--fg-tertiary)] font-mono px-6 truncate pb-0.5">
                          {g.path}
                        </div>
                        {isOpen &&
                          g.lines.map((l) => (
                            <button
                              key={`${g.path}:${l.line}`}
                              onClick={() => globalOpenFile(g.path, { line: l.line })}
                              className="w-full text-left flex items-center gap-2 px-6 py-0.5 hover:bg-[var(--bg-surface-hover)] cursor-pointer"
                              title={`${g.path}:${l.line}`}
                            >
                              <span className="shrink-0 w-6 text-right text-[10px] text-[var(--fg-tertiary)]/50 font-mono select-none">
                                {l.line}
                              </span>
                              <span className="truncate text-[11px] text-[var(--fg-secondary)] font-mono">
                                {l.preview}
                              </span>
                            </button>
                          ))}
                      </div>
                    );
                  })}
                </>
              )}
            </div>
          </div>
        ) : (
          <div className="flex-1 flex flex-col min-w-0 min-h-0">
            {/* Header */}
            <div className="flex flex-col gap-2 px-3 py-2 border-b border-[var(--border-default)] shrink-0 bg-[var(--bg-panel)]">
              <div className="flex items-center justify-between">
                <span className="text-[10px] font-bold uppercase tracking-wider text-[var(--fg-tertiary)]">Explorer</span>
                <div className="flex items-center gap-1">
                  <button
                    onClick={handleRefreshClick}
                    disabled={refreshing}
                    className="p-1 hover:bg-[var(--bg-surface-hover)] rounded text-[var(--fg-tertiary)] hover:text-white cursor-pointer disabled:opacity-60"
                    title="Refresh Workspace"
                  >
                    <IconRefresh className={`size-3.5 ${refreshing ? "animate-spin" : ""}`} />
                  </button>
                  <button
                    onClick={handleToggleHidden}
                    className="p-1 hover:bg-[var(--bg-surface-hover)] rounded text-[var(--fg-tertiary)] hover:text-white cursor-pointer"
                    title="Toggle Hidden Files"
                  >
                    {showHidden ? <IconEye className="size-3.5" /> : <IconEyeOff className="size-3.5" />}
                  </button>

                </div>
              </div>
            </div>

            {/* File Tree */}
            <div ref={treeScrollRef} className="flex-1 min-h-0 overflow-y-auto py-1">
              <div className="relative" style={{ height: `${virtualizer.getTotalSize()}px` }}>
                {virtualizer.getVirtualItems().map((vi) => (
                  <div
                    key={vi.key}
                    data-index={vi.index}
                    ref={virtualizer.measureElement}
                    className="absolute top-0 left-0 w-full"
                    style={{ transform: `translateY(${vi.start}px)` }}
                  >
                    {renderRow(visibleRows[vi.index])}
                  </div>
                ))}
              </div>
            </div>

            {/* Vertical drag handle */}
            <div
              onMouseDown={handleMouseDownSessions}
              className="h-[3px] bg-[var(--border-default)] hover:bg-[var(--accent-primary)] cursor-ns-resize shrink-0 transition-colors"
              title="Drag to resize Session Manager"
            />

            {/* Sessions Manager container */}
            <div
              style={{ height: `${sessionsHeight}px` }}
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
                      <div className="min-w-0 leading-tight">
                        <div className="truncate">{s.name}</div>
                        {s.type === "shell" && s.pid ? (
                          <div className="text-[9px] font-mono text-[var(--fg-tertiary)] truncate">PID {s.pid}</div>
                        ) : null}
                      </div>
                    </div>

                    <div className="flex items-center space-x-0.5 shrink-0">
                      <button
                        onClick={(e) => requestRenameSession(s, e)}
                        className="opacity-0 group-hover:opacity-100 hover:bg-[var(--bg-surface-hover)] rounded p-0.5"
                        title="Rename Session"
                      >
                        <IconPencil className="size-3 text-[var(--fg-tertiary)] hover:text-white" />
                      </button>
                      <button
                        onClick={(e) => requestKillSession(s, e)}
                        className="opacity-0 group-hover:opacity-100 hover:bg-[var(--bg-surface-hover)] rounded p-0.5"
                        title="Kill Session"
                      >
                        <IconX className="size-3 text-red-400" />
                      </button>
                    </div>
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
              startInlineRename(contextMenu.node);
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

      {/* Rename Session Modal Overlay */}
      {renameSessionTarget && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className="bg-[var(--bg-sidebar)] border border-[var(--border-default)] w-full max-w-sm p-4 space-y-3 shadow-2xl">
            <div className="flex items-center justify-between pb-2 border-b border-[var(--border-default)]">
              <span className="font-bold text-xs text-[var(--fg-primary)] uppercase tracking-wider">Rename Session</span>
              <button onClick={() => setRenameSessionTarget(null)} className="text-[var(--fg-tertiary)] hover:text-white cursor-pointer">
                <IconX className="size-4" />
              </button>
            </div>
            <input
              autoFocus
              value={renameSessionValue}
              onChange={(e) => setRenameSessionValue(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") confirmRenameSession();
                if (e.key === "Escape") setRenameSessionTarget(null);
              }}
              placeholder="Session name"
              className="w-full bg-[var(--bg-panel)] border border-[var(--border-default)] p-2 text-xs text-[var(--fg-primary)] focus:outline-none focus:border-[var(--accent-primary)]"
            />
            <div className="flex items-center justify-end space-x-2 pt-1">
              <button
                onClick={() => setRenameSessionTarget(null)}
                className="px-3 py-1 text-xs text-[var(--fg-secondary)] hover:text-white border border-[var(--border-default)] rounded cursor-pointer"
              >
                Cancel
              </button>
              <button
                onClick={confirmRenameSession}
                className="px-3 py-1 text-xs font-semibold bg-[var(--accent-primary)] hover:bg-[var(--accent-hover)] text-black rounded cursor-pointer"
              >
                Rename
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Kill Session Confirmation Modal Overlay */}
      {killConfirm && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className="bg-[var(--bg-sidebar)] border border-[var(--border-default)] w-full max-w-sm p-4 space-y-4 shadow-2xl">
            <div className="flex items-center justify-between pb-2 border-b border-[var(--border-default)]">
              <span className="font-bold text-xs text-[var(--fg-primary)] uppercase tracking-wider text-red-400">
                Kill Session
              </span>
              <button
                onClick={() => setKillConfirm(null)}
                className="text-[var(--fg-tertiary)] hover:text-white cursor-pointer"
              >
                <IconX className="size-4" />
              </button>
            </div>

            <div className="text-xs text-[var(--fg-secondary)] break-all">
              Kill <span className="font-mono text-[var(--fg-primary)]">{killConfirm.name}</span>? The process
              {killConfirm.type === "shell" ? " and its PID" : ""} will be terminated and its open panel tab will be closed.
            </div>

            <div className="flex items-center justify-end space-x-2 pt-2 border-t border-[var(--border-default)]">
              <button
                onClick={() => setKillConfirm(null)}
                className="px-3 py-1.5 text-xs text-[var(--fg-secondary)] hover:text-white cursor-pointer"
              >
                Cancel
              </button>
              <button
                onClick={confirmKillSession}
                className="px-4 py-1.5 bg-red-600 hover:bg-red-700 text-white text-xs font-semibold cursor-pointer"
              >
                Kill Session
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Delete Confirmation Modal Overlay */}
      {deleteConfirm && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className="bg-[var(--bg-sidebar)] border border-[var(--border-default)] w-full max-w-sm p-4 space-y-4 shadow-2xl">
            <div className="flex items-center justify-between pb-2 border-b border-[var(--border-default)]">
              <span className="font-bold text-xs text-[var(--fg-primary)] uppercase tracking-wider text-red-400">
                Delete
              </span>
              <button
                onClick={() => setDeleteConfirm(null)}
                className="text-[var(--fg-tertiary)] hover:text-white cursor-pointer"
              >
                <IconX className="size-4" />
              </button>
            </div>

            <div className="text-xs text-[var(--fg-secondary)] break-all">
              Delete <span className="font-mono text-[var(--fg-primary)]">{deleteConfirm.name}</span>? This will permanently delete the file/folder.
            </div>

            <div className="flex items-center justify-end space-x-2 pt-2 border-t border-[var(--border-default)]">
              <button
                onClick={() => setDeleteConfirm(null)}
                className="px-3 py-1.5 text-xs text-[var(--fg-secondary)] hover:text-white cursor-pointer"
              >
                Cancel
              </button>
              <button
                onClick={handleDeleteConfirmed}
                className="px-4 py-1.5 bg-red-600 hover:bg-red-700 text-white text-xs font-semibold cursor-pointer"
              >
                Delete
              </button>
            </div>
          </div>
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
});
