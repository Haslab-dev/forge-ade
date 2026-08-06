import {
  useEffect,
  useLayoutEffect,
  useCallback,
  useRef,
  useState,
} from "react";
import "./index.css";
import {
  useWorkspaceStore,
  useUIStore,
  useShortcutsStore,
  useEditorStore,
  openBrowserTab,
} from "./hooks/store";
import { Welcome } from "./panels/welcome";
import { Sidebar } from "./components/sidebar";
import { SessionsBar } from "./components/sessions-bar";
import { Editor, globalOpenFile, setOnBeforeOpenFile } from "./panels/editor";
import { GitGraphPanel } from "./panels/git-graph-panel";
import { setOnOpenInBrowser } from "./panels/browser-panel";
import { ResizableSplit } from "./components/resizable-split";
import {
  IconFileCode,
  IconGitBranch,
  IconFolder,
  IconFileText,
  IconFile,
  IconDeviceFloppy,
  IconX,
  IconSettings,
  IconArrowUpRight,
  IconMenu2,
} from "@tabler/icons-react";
import { SimpleModal } from "./components/simple-modal";
import { GlobalSettingsModal } from "./components/global-settings-modal";
import { cn } from "./lib/utils";
import type { RecentEntry, Workspace } from "./types";
import { ClipboardGetText } from "./lib/wails";
import {
  GetRecentProjects,
  OpenFolder,
  OpenWorkspace,
  CloseWorkspace,
  GetCurrentWorkspace,
  SaveWorkspace,
  SaveWorkspaceAs,
  SaveWorkspaceDialog,
  PinRecent,
  RemoveRecent,
  OpenFolderDialog,
  OpenWorkspaceDialog,
  OpenFileDialog,
  OpenNewWindow,
  CreateShell,
  CreateAgentSession,
  WriteFile,
  FormatCode,
} from "./lib/wails";
import { applyFormattedContent, getGlobalLiveContent } from "./panels/editor";
import { FolderOpen } from "lucide-react";

function toWorkspace(ws: any): Workspace {
  return {
    name: ws.name ?? ws.Name ?? "Untitled",
    folders: ws.folders ?? ws.Folders ?? [],
    isTemporary: ws.isTemporary ?? ws.IsTemporary ?? true,
    filePath: ws.filePath ?? ws.FilePath ?? "",
    theme: ws.settings?.theme ?? ws.Settings?.Theme ?? "dark-plus",
  };
}

function App() {
  const { workspace, recentProjects, setWorkspace, setRecentProjects } =
    useWorkspaceStore();
  const { theme } = useUIStore();
  const ok = "test";
  const [activeScreen, setActiveScreen] = useState<"editor" | "git-graph">(
    "editor",
  );

  // WKWebView keeps the unmounted CodeMirror editor's composited layer (ghost
  // text) painted over the next screen until a repaint is forced — the same
  // thing the user got by collapsing/reopening the sidebar. Toggling the main
  // panel's rendering rebuilds its layer tree so the stale layer is dropped.
  const mainRef = useRef<HTMLElement>(null);
  useLayoutEffect(() => {
    const el = mainRef.current;
    if (!el) return;
    el.style.display = "none";
    void el.offsetHeight; // forced reflow: discard stale composited layers
    el.style.display = "";
  }, [activeScreen]);
  const [showShellNameModal, setShowShellNameModal] = useState(false);
  const [showAgentCreateModal, setShowAgentCreateModal] = useState(false);
  const [newAgentRole, setNewAgentRole] = useState<
    "coding" | "planning" | "research" | "custom"
  >("coding");
  const [newAgentName, setNewAgentName] = useState("");
  const [showOpenPathModal, setShowOpenPathModal] = useState(false);
  const [openPathValue, setOpenPathValue] = useState("");
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const [showSettingsModal, setShowSettingsModal] = useState(false);
  const [menuOpen, setMenuOpen] = useState(false);

  const { files, activeFileIndex, setFiles, setActiveFileIndex } =
    useEditorStore();

  // Sync design token theme to root HTML element
  useEffect(() => {
    document.documentElement.className = theme;
  }, [theme]);

  // Load recent projects on mount
  useEffect(() => {
    loadRecentProjects();
  }, []);

  // Auto-switch to Editor when a file is opened
  useEffect(() => {
    setOnBeforeOpenFile(() => setActiveScreen("editor"));
  }, []);

  // Global "open in internal browser" handler: opens the browser tab inside the
  // Workspace tab panel. Used by terminal links (dev-server URLs etc.).
  useEffect(() => {
    setOnOpenInBrowser((url) => {
      openBrowserTab(url);
    });
    return () => setOnOpenInBrowser(null);
  }, []);

  async function loadRecentProjects() {
    try {
      const entries: any[] = await GetRecentProjects();
      const mapped: RecentEntry[] = entries.map((e: any) => ({
        path: e.Path ?? e.path,
        name: e.Name ?? e.name,
        isWorkspace: e.IsWorkspace ?? e.isWorkspace,
        lastOpened: e.LastOpened ?? e.lastOpened,
        pinned: e.Pinned ?? e.pinned,
        favorite: e.Favorite ?? e.favorite,
      }));
      setRecentProjects(mapped);
    } catch (err) {
      console.error("Failed to load recent projects:", err);
    }
  }

  const handleOpenFolder = useCallback(async () => {
    const path = await OpenFolderDialog();
    if (!path) return;
    try {
      const ws = await OpenFolder(path);
      setWorkspace(toWorkspace(ws));
      loadRecentProjects();
    } catch (err) {
      console.error("Failed to open folder:", err);
    }
  }, [setWorkspace]);

  const handleOpenWorkspace = useCallback(async () => {
    const path = await OpenWorkspaceDialog();
    if (!path) return;
    try {
      const ws = await OpenWorkspace(path);
      setWorkspace(toWorkspace(ws));
      loadRecentProjects();
    } catch (err) {
      console.error("Failed to open workspace:", err);
    }
  }, [setWorkspace]);

  const handleOpenFile = useCallback(async () => {
    let defaultPath = "";
    try {
      const clipText = await ClipboardGetText();
      if (clipText && clipText.trim()) {
        const trimmed = clipText.trim();
        if (
          trimmed.startsWith("/") ||
          trimmed.startsWith("~") ||
          /^[A-Za-z]:[\\/]/.test(trimmed)
        ) {
          defaultPath = trimmed;
        }
      }
    } catch {
      /* ignore */
    }
    setOpenPathValue(defaultPath);
    setShowOpenPathModal(true);
  }, []);

  const handleNewWindow = useCallback(async () => {
    try {
      await OpenNewWindow("");
    } catch (err) {
      console.error("Failed to open new window:", err);
    }
  }, []);

  const handleSaveWorkspace = useCallback(async () => {
    if (!workspace) return;
    if (workspace.isTemporary || !workspace.filePath) {
      const path = await SaveWorkspaceDialog();
      if (!path) return;
      try {
        await SaveWorkspaceAs(path);
        setWorkspace({ ...workspace, filePath: path, isTemporary: false });
        loadRecentProjects();
      } catch (err) {
        console.error("Failed to save workspace:", err);
      }
    } else {
      try {
        await SaveWorkspace();
      } catch (err) {
        console.error("Failed to save workspace:", err);
      }
    }
  }, [workspace, setWorkspace]);

  const handleOpenRecent = useCallback(
    async (entry: RecentEntry) => {
      try {
        let ws: any;
        if (entry.isWorkspace) {
          ws = await OpenWorkspace(entry.path);
        } else {
          ws = await OpenFolder(entry.path);
        }
        setWorkspace(toWorkspace(ws));
        loadRecentProjects();
      } catch (err) {
        console.error("Failed to open recent:", err);
      }
    },
    [setWorkspace],
  );

  const handlePinRecent = useCallback(async (path: string, pinned: boolean) => {
    try {
      await PinRecent(path, pinned);
      loadRecentProjects();
    } catch (err) {
      console.error("Failed to pin recent:", err);
    }
  }, []);

  const handleRemoveRecent = useCallback(async (path: string) => {
    try {
      await RemoveRecent(path);
      loadRecentProjects();
    } catch (err) {
      console.error("Failed to remove recent:", err);
    }
  }, []);

  const handleClose = useCallback(async () => {
    try {
      await CloseWorkspace();
      setWorkspace(null);
      setFiles([]);
      setActiveFileIndex(-1);
    } catch (err) {
      console.error("Failed to close workspace:", err);
    }
  }, [setWorkspace, setFiles, setActiveFileIndex]);

  const handleCreateShell = useCallback(
    async (name?: string) => {
      const shellName = name?.trim() || "Shell";
      try {
        const s = await CreateShell(shellName, workspace?.folders[0] ?? "");

        const newTab = {
          id: s.id,
          name: s.name || "Shell",
          path: s.id,
          type: "shell" as "shell",
          content: "",
          modified: false,
        };

        setFiles((prev) => [...prev, newTab]);
        setActiveFileIndex(files.length);
        // The sidebar/session list self-refreshes via the session:opened event.
        setActiveScreen("editor");
      } catch (err) {
        console.error("Failed to create shell:", err);
      }
    },
    [workspace, files.length, setFiles, setActiveFileIndex],
  );

  const handleCreateAgent = useCallback(async () => {
    const name = newAgentName.trim() || `Agent (${newAgentRole})`;
    try {
      const a = await CreateAgentSession(
        name,
        newAgentRole,
        workspace?.folders[0] ?? "",
      );

      const newTab = {
        id: a.id,
        name: a.name || "Agent",
        path: a.id,
        type: "agent" as "agent",
        content: "",
        modified: false,
      };

      setFiles((prev) => [...prev, newTab]);
      setActiveFileIndex(files.length);
      // The sidebar/session list self-refreshes via the agent:updated event.
      setShowAgentCreateModal(false);
      setNewAgentName("");
      setActiveScreen("editor");
    } catch (err) {
      console.error("Failed to create agent session:", err);
    }
  }, [
    workspace,
    files.length,
    newAgentName,
    newAgentRole,
    setFiles,
    setActiveFileIndex,
  ]);

  const handleRequestCreateShell = useCallback(() => {
    setActiveScreen("editor");
    setShowShellNameModal(true);
  }, []);

  const handleRequestCreateAgent = useCallback(() => {
    setActiveScreen("editor");
    setShowAgentCreateModal(true);
  }, []);

  // Stable callbacks so React.memo(Sidebar) can skip re-renders on App churn.
  const handleOpenSession = useCallback(() => setActiveScreen("editor"), []);
  const handleOpenSettings = useCallback(() => setShowSettingsModal(true), []);

  const handleRefreshWorkspace = useCallback(async () => {
    try {
      const ws = await GetCurrentWorkspace();
      if (ws) setWorkspace(toWorkspace(ws));
    } catch {
      /* ignore */
    }
  }, [setWorkspace]);

  // Hoisted keybindings and listener hook
  const { keybindings } = useShortcutsStore();

  const handleSaveActiveFile = useCallback(async () => {
    const file = files[activeFileIndex];
    if (!file || file.type !== "file") return;
    try {
      // Use the freshest live doc (mirrored on every keystroke) so saving
      // never writes a debounce-stale copy to disk.
      const live = getGlobalLiveContent(file.id);
      let content = live !== undefined ? live : (file.content ?? "");
      const ext = file.path.split(".").pop()?.toLowerCase() || "";
      if (
        [
          "js",
          "jsx",
          "ts",
          "tsx",
          "mjs",
          "cjs",
          "mts",
          "cts",
          "json",
          "css",
          "html",
          "md",
        ].includes(ext)
      ) {
        const formatted = await FormatCode(file.path, file.content ?? "");
        if (formatted) {
          content = formatted;
          applyFormattedContent(formatted);
        }
      }
      await WriteFile(file.path, content);
      setFiles((prev) => {
        const next = [...prev];
        next[activeFileIndex] = {
          ...next[activeFileIndex],
          content,
          savedContent: content,
          modified: false,
        };
        return next;
      });
    } catch (err) {
      console.error("Shortcut failed to save file:", err);
    }
  }, [files, activeFileIndex, setFiles]);

  const handleCloseActiveFile = useCallback(() => {
    if (activeFileIndex < 0 || activeFileIndex >= files.length) return;
    setFiles((prev) => {
      const next = [...prev];
      next.splice(activeFileIndex, 1);
      return next;
    });
    setActiveFileIndex((prev) => {
      if (prev === activeFileIndex) return Math.max(0, prev - 1);
      if (prev > activeFileIndex) return prev - 1;
      return prev;
    });
  }, [files.length, activeFileIndex, setFiles, setActiveFileIndex]);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement;
      // Do not block normal inputs on terminal and text inputs unless command shortcuts are pressed
      const isInput =
        target &&
        (target.tagName === "INPUT" ||
          target.tagName === "TEXTAREA" ||
          target.closest(".cm-editor") ||
          target.closest("canvas"));

      const parts: string[] = [];
      if (e.metaKey) parts.push("meta");
      if (e.ctrlKey) parts.push("ctrl");
      if (e.altKey) parts.push("alt");
      if (e.shiftKey) parts.push("shift");

      let key = e.key.toLowerCase();
      if (key === " ") key = "space";
      if (["control", "shift", "alt", "meta"].includes(key)) {
        return;
      }
      parts.push(key);
      const pressedShortcut = parts.join("+");

      // Never intercept the OS's reserved editing shortcuts (clipboard,
      // undo/redo, select-all). These belong to the user's normal workflow
      // and must always reach the focused input/editor/terminal.
      const reserved = [
        "meta+v",
        "ctrl+v",
        "meta+c",
        "ctrl+c",
        "meta+x",
        "ctrl+x",
        "meta+z",
        "ctrl+z",
        "meta+shift+z",
        "ctrl+shift+z",
        "meta+y",
        "ctrl+y",
        "meta+a",
        "ctrl+a",
      ];
      if (reserved.includes(pressedShortcut)) {
        return;
      }

      const matched = keybindings.find((kb) => kb.key === pressedShortcut);
      if (!matched) return;

      // If in input and the matched shortcut doesn't have modifiers, do not intercept
      if (isInput && !e.metaKey && !e.ctrlKey) {
        return;
      }

      e.preventDefault();
      e.stopPropagation();

      switch (matched.id) {
        case "save-file":
          handleSaveActiveFile();
          break;
        case "close-file":
          handleCloseActiveFile();
          break;
        case "toggle-sidebar":
          setSidebarCollapsed((prev) => !prev);
          break;
        case "toggle-terminal":
          // Open terminal shell tab directly
          handleCreateShell();
          break;
        case "toggle-agent":
          // Open agent chat tab directly
          handleRequestCreateAgent();
          break;
        case "open-file":
          handleOpenFile();
          break;
        case "new-terminal":
          handleRequestCreateShell();
          break;
        default:
          break;
      }
    };

    window.addEventListener("keydown", handleKeyDown, true);
    return () => window.removeEventListener("keydown", handleKeyDown, true);
  }, [
    keybindings,
    handleSaveActiveFile,
    handleCloseActiveFile,
    handleOpenFile,
    handleRequestCreateShell,
    handleCreateShell,
    handleRequestCreateAgent,
  ]);

  // ---------- Welcome Screen ----------
  if (!workspace) {
    return (
      <div
        className={`${theme} h-screen w-screen bg-background text-foreground`}
      >
        <Welcome
          recentProjects={recentProjects}
          onOpenFolder={handleOpenFolder}
          onOpenWorkspace={handleOpenWorkspace}
          onOpenRecent={handleOpenRecent}
          onPinRecent={handlePinRecent}
          onRemoveRecent={handleRemoveRecent}
        />
      </div>
    );
  }

  // ---------- Workspace Layout ----------
  return (
    <div
      className={`${theme} h-screen w-screen bg-background text-foreground flex flex-col overflow-hidden`}
    >
      {/* App Bar (Simplified, sleek, premium borderless design) */}
      <header className="safe-area-top titlebar-drag flex items-center h-10 px-3 border-b border-[var(--border-default)] bg-[var(--bg-panel)] text-xs shrink-0 gap-1 select-none">
        {/* File/Workspace menu — hamburger (3-line) button */}
        <div className="relative titlebar-no-drag">
          <button
            onClick={() => setMenuOpen((v) => !v)}
            className="inline-flex items-center justify-center p-1.5 rounded transition-colors cursor-pointer text-[var(--fg-secondary)] hover:text-white hover:bg-[var(--bg-surface-hover)]"
            title="Menu"
            aria-label="Menu"
          >
            <IconMenu2 className="size-4" />
          </button>
          {menuOpen && (
            <>
              <div
                className="fixed inset-0 z-40"
                onClick={() => setMenuOpen(false)}
              />
              <div className="absolute left-0 top-full mt-1 z-50 w-56 bg-[var(--bg-sidebar)] border border-[var(--border-default)] shadow-2xl py-1 text-[11px]">
                <button
                  onClick={() => {
                    setMenuOpen(false);
                    handleOpenFolder();
                  }}
                  className="w-full text-left px-3 py-1.5 hover:bg-[var(--bg-panel)] flex items-center gap-2 text-[var(--fg-primary)] cursor-pointer"
                >
                  <IconFolder className="size-3.5 text-amber-400" />
                  <span>Open Folder</span>
                </button>
                <button
                  onClick={() => {
                    setMenuOpen(false);
                    handleOpenWorkspace();
                  }}
                  className="w-full text-left px-3 py-1.5 hover:bg-[var(--bg-panel)] flex items-center gap-2 text-[var(--fg-primary)] cursor-pointer"
                >
                  <IconFileText className="size-3.5 text-blue-400" />
                  <span>Open Workspace</span>
                </button>
                <button
                  onClick={() => {
                    setMenuOpen(false);
                    handleOpenFile();
                  }}
                  className="w-full text-left px-3 py-1.5 hover:bg-[var(--bg-panel)] flex items-center gap-2 text-[var(--fg-primary)] cursor-pointer"
                >
                  <IconFile className="size-3.5 text-cyan-400" />
                  <span>Open File</span>
                </button>
                <div className="h-px bg-[var(--border-default)] my-1" />
                <button
                  onClick={() => {
                    setMenuOpen(false);
                    handleNewWindow();
                  }}
                  className="w-full text-left px-3 py-1.5 hover:bg-[var(--bg-panel)] flex items-center gap-2 text-[var(--fg-primary)] cursor-pointer"
                >
                  <IconArrowUpRight className="size-3.5 text-purple-400" />
                  <span>Open New Window</span>
                </button>
                <button
                  onClick={() => {
                    setMenuOpen(false);
                    handleSaveWorkspace();
                  }}
                  className="w-full text-left px-3 py-1.5 hover:bg-[var(--bg-panel)] flex items-center gap-2 text-[var(--fg-primary)] cursor-pointer"
                >
                  <IconDeviceFloppy className="size-3.5 text-green-400" />
                  <span>Save Workspace</span>
                </button>
                <div className="h-px bg-[var(--border-default)] my-1" />
                <button
                  onClick={() => {
                    setMenuOpen(false);
                    setShowSettingsModal(true);
                  }}
                  className="w-full text-left px-3 py-1.5 hover:bg-[var(--bg-panel)] flex items-center gap-2 text-[var(--fg-primary)] cursor-pointer"
                >
                  <IconSettings className="size-3.5 text-slate-400" />
                  <span>Global Settings</span>
                </button>
              </div>
            </>
          )}
        </div>

        <span className="hidden sm:inline font-bold tracking-tight text-[var(--accent-primary)]">
          ForgeADE
        </span>
        <span className="hidden sm:inline text-muted-foreground/30 mx-1">
          /
        </span>
        <span className="font-medium text-[var(--fg-secondary)] truncate max-w-16 md:max-w-48">
          {workspace.name}
        </span>
        {workspace.isTemporary && (
          <span className="ml-1.5 text-[9px] text-[var(--fg-tertiary)] bg-[var(--bg-surface-active)]/70 border border-[var(--border-default)] px-1 py-0.2 rounded font-mono">
            temp
          </span>
        )}
        <div className="w-px h-3.5 bg-[var(--border-default)] mx-2" />

        <div className="flex items-center gap-1 titlebar-no-drag">
          <button
            className={cn(
              "inline-flex items-center gap-1.5 px-2.5 py-1 rounded transition-colors cursor-pointer font-semibold",
              activeScreen === "editor"
                ? "bg-[var(--bg-surface-active)] text-[var(--fg-on-active)]"
                : "text-[var(--fg-secondary)] hover:text-white hover:bg-[var(--bg-surface-hover)]",
            )}
            onClick={() => setActiveScreen("editor")}
            title="Workspace Editor"
          >
            <IconFileCode className="size-3.5" />
            <span className="hidden md:inline">Workspace</span>
          </button>

          <button
            className={cn(
              "inline-flex items-center gap-1.5 px-2.5 py-1 rounded transition-colors cursor-pointer font-semibold",
              activeScreen === "git-graph"
                ? "bg-[var(--bg-surface-active)] text-[var(--fg-on-active)]"
                : "text-[var(--fg-secondary)] hover:text-white hover:bg-[var(--bg-surface-hover)]",
            )}
            onClick={() => setActiveScreen("git-graph")}
            title="Git Graph"
          >
            <IconGitBranch className="size-3.5 text-purple-400" />
            <span className="hidden md:inline">Git Graph</span>
          </button>
        </div>

        <div className="flex-1" />

        <div className="flex items-center gap-0.5 titlebar-no-drag">
          <button
            className="inline-flex items-center gap-1 px-2 py-1 text-muted-foreground hover:text-foreground hover:bg-accent rounded"
            onClick={handleClose}
            title="Close Workspace"
          >
            <IconX className="size-3.5" />
            <span className="hidden sm:inline">Close</span>
          </button>
        </div>
      </header>

      {/* Main Content */}
      <div className="flex flex-1 overflow-hidden">
        <ResizableSplit
          collapsed={sidebarCollapsed}
          left={
            <Sidebar
              folders={workspace.folders}
              onRefreshWorkspace={handleRefreshWorkspace}
              collapsed={sidebarCollapsed}
              onToggleCollapse={setSidebarCollapsed}
              onCreateShell={handleRequestCreateShell}
              onCreateAgent={handleRequestCreateAgent}
              onOpenSession={handleOpenSession}
              onOpenSettings={handleOpenSettings}
            />
          }
          right={
            <main
              ref={mainRef}
              className="flex-1 h-full overflow-hidden bg-[var(--bg-app)]"
            >
              {activeScreen === "git-graph" ? (
                <GitGraphPanel />
              ) : (
                <Editor />
              )}
            </main>
          }
          initialLeftWidth={240}
          minLeftWidth={150}
          maxLeftWidth={500}
          collapsedWidth={40}
        />
      </div>

      {/* Status Bar */}
      <SessionsBar
        onSelectSession={() => {}}
        cwd={workspace.folders[0]}
        onCreateShell={handleRequestCreateShell}
      />

      {/* Shell Name Modal */}
      <SimpleModal
        open={showShellNameModal}
        title="New Shell"
        defaultValue="Shell"
        placeholder="Shell name"
        submitLabel="Create"
        onClose={() => setShowShellNameModal(false)}
        onSubmit={(name) => {
          setShowShellNameModal(false);
          handleCreateShell(name);
        }}
      />

      {/* Launch Agent Modal */}
      {showAgentCreateModal && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className="bg-[var(--bg-sidebar)] border border-[var(--border-default)] w-full max-w-sm p-4 space-y-4">
            <div className="flex items-center justify-between pb-2 border-b border-[var(--border-default)]">
              <span className="font-bold text-sm text-[var(--fg-primary)]">
                Launch AI Agent
              </span>
              <button
                onClick={() => setShowAgentCreateModal(false)}
                className="text-[var(--fg-tertiary)] hover:text-white cursor-pointer"
              >
                <IconX className="size-4" />
              </button>
            </div>

            <div className="space-y-2 text-xs">
              <label className="text-[var(--fg-secondary)] block font-medium">
                Agent Role Filter
              </label>
              <select
                value={newAgentRole}
                onChange={(e: any) => setNewAgentRole(e.target.value)}
                className="w-full bg-[var(--bg-panel)] border border-[var(--border-default)] px-3 py-1.5 text-[var(--fg-primary)] focus:outline-none"
              >
                <option value="coding">Coding Agent</option>
                <option value="planning">Planning Agent</option>
                <option value="research">Research Agent</option>
                <option value="custom">Custom Agent</option>
              </select>

              <label className="text-[var(--fg-secondary)] block font-medium pt-1">
                Session Name (Optional)
              </label>
              <input
                type="text"
                value={newAgentName}
                onChange={(e) => setNewAgentName(e.target.value)}
                placeholder="Coding Agent Session"
                className="w-full bg-[var(--bg-panel)] border border-[var(--border-default)] px-3 py-1.5 text-[var(--fg-primary)] focus:outline-none focus:border-[var(--accent-primary)]"
              />
            </div>

            <div className="flex items-center justify-end space-x-2 pt-2 border-t border-[var(--border-default)]">
              <button
                onClick={() => setShowAgentCreateModal(false)}
                className="px-3 py-1.5 text-xs text-[var(--fg-secondary)] hover:text-white cursor-pointer"
              >
                Cancel
              </button>
              <button
                onClick={handleCreateAgent}
                className="px-4 py-1.5 bg-[var(--accent-primary)] hover:bg-[var(--accent-hover)] text-black text-xs font-semibold cursor-pointer"
              >
                Launch
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Open File Modal */}
      <SimpleModal
        open={showOpenPathModal}
        title="Open File"
        defaultValue={openPathValue}
        placeholder="/path/to/file"
        submitLabel="Open Path"
        onClose={() => setShowOpenPathModal(false)}
        onSubmit={(path) => {
          setShowOpenPathModal(false);
          globalOpenFile(path);
        }}
        secondaryAction={{
          label: "Browse Finder / File Explorer",
          icon: <FolderOpen className="size-4 text-amber-400" />,
          onClick: async () => {
            try {
              const path = await OpenFileDialog();
              if (path) {
                setShowOpenPathModal(false);
                globalOpenFile(path);
              }
            } catch (err) {
              console.error("Failed to pick file:", err);
            }
          },
        }}
      />

      {/* Global Settings Modal */}
      <GlobalSettingsModal
        open={showSettingsModal}
        onClose={() => setShowSettingsModal(false)}
      />
    </div>
  );
}

export default App;
