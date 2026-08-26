import {
  useEffect,
  useLayoutEffect,
  useCallback,
  useReducer,
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
import { UsagePanel } from "./panels/usage-panel";
import { setOnOpenInBrowser } from "./panels/browser-panel";
import { ResizableSplit } from "./components/resizable-split";
import {
  IconArrowLeft,
  IconArrowRight,
  IconFileCode,
  IconFolder,
  IconFolderPlus,
  IconFolderOpen,
  IconFileText,
  IconFile,
  IconDeviceFloppy,
  IconX,
  IconSettings,
  IconArrowUpRight,
  IconMenu2,
  IconSearch,
  IconCommand,
} from "@tabler/icons-react";
import {
  recordNav,
  goBack as navGoBack,
  goForward as navGoForward,
  canGoBack,
  canGoForward,
  subscribeNav,
  setNavSilent,
  type NavEntry,
} from "./lib/nav-history";
import { CommandPalette, PaletteMode } from "./components/command-palette";
import { SimpleModal } from "./components/simple-modal";
import { GlobalSettingsModal } from "./components/global-settings-modal";
import { LSPModal } from "./components/lsp-modal";
import { cn } from "./lib/utils";
import type { RecentEntry, Workspace } from "./types";
import { ClipboardGetText } from "./lib/native";
import {
  GetRecentProjects,
  OpenFolder,
  AddFolderToWorkspace,
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
  ListSessions,
  ListAgentSessions,
  WriteFile,
  FormatCode,
} from "./lib/native";
import { applyFormattedContent, getGlobalLiveContent } from "./panels/editor";
import { formatWithSettings, loadEditorSettings } from "./lib/editor-settings";

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
  const [activeScreen, setActiveScreen] = useState<"editor" | "git-graph" | "usage">(
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
  const [showOpenPathModal, setShowOpenPathModal] = useState(false);
  const [openPathValue, setOpenPathValue] = useState("");
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const [showSettingsModal, setShowSettingsModal] = useState(false);
  const [menuOpen, setMenuOpen] = useState(false);
  const [showCommandPalette, setShowCommandPalette] = useState(false);
  const [paletteMode, setPaletteMode] = useState<PaletteMode>("command");
  const [showLSPModal, setShowLSPModal] = useState(false);
  const [lspModalPos, setLspModalPos] = useState<{ x: number; y: number } | undefined>(undefined);

  const { files, activeFileIndex, setFiles, setActiveFileIndex } =
    useEditorStore();

  // Auto-switch to Editor when a file is opened
  useEffect(() => {
    setOnBeforeOpenFile(() => setActiveScreen("editor"));
  }, []);

  // ---------- Navigation history (back/forward) ----------
  // Record every activated tab (file/shell/agent/browser) and every main
  // screen switch so ← / → replays the exact visit order.
  const [, forceNavRender] = useReducer((c: number) => c + 1, 0);
  useEffect(() => subscribeNav(forceNavRender), []);

  const activeTabId = files[activeFileIndex]?.id;
  const activeTabPath = files[activeFileIndex]?.type === "file" ? files[activeFileIndex]?.path : undefined;
  useEffect(() => {
    if (activeTabId === undefined) return;
    recordNav(
      activeTabPath
        ? { kind: "tab", id: activeTabId, path: activeTabPath }
        : { kind: "tab", id: activeTabId }
    );
  }, [activeTabId]);

  useEffect(() => {
    recordNav({ kind: "screen", screen: activeScreen });
  }, [activeScreen]);

  const applyNavEntry = useCallback((entry: NavEntry) => {
    setNavSilent(true);
    if (entry.kind === "screen") {
      setActiveScreen(entry.screen);
      setTimeout(() => setNavSilent(false), 0);
      return;
    }
    const st = useEditorStore.getState();
    const idx = st.files.findIndex((f) => f.id === entry.id);
    if (idx >= 0) {
      st.setActiveFileIndex(idx);
      setActiveScreen("editor");
      setTimeout(() => setNavSilent(false), 0);
      return;
    }
    if (entry.path) {
      void globalOpenFile(entry.path, entry.line ? { line: entry.line } : undefined).then(() => {
        setActiveScreen("editor");
        setTimeout(() => setNavSilent(false), 0);
      });
      return;
    }
    setNavSilent(false);
  }, []);

  const handleNavBack = useCallback(() => {
    const entry = navGoBack();
    if (entry) applyNavEntry(entry);
  }, [applyNavEntry]);
  const handleNavForward = useCallback(() => {
    const entry = navGoForward();
    if (entry) applyNavEntry(entry);
  }, [applyNavEntry]);

  // Alt+← / Alt+→ walk the history like VSCode's go back/forward.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (!e.altKey || e.ctrlKey || e.metaKey) return;
      if (e.key === "ArrowLeft") {
        e.preventDefault();
        handleNavBack();
      } else if (e.key === "ArrowRight") {
        e.preventDefault();
        handleNavForward();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [handleNavBack, handleNavForward]);
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

  const resetViewPane = useCallback(() => {
    setFiles([]);
    setActiveFileIndex(-1);
  }, [setFiles, setActiveFileIndex]);

  const handleOpenFolder = useCallback(async () => {
    const path = await OpenFolderDialog();
    if (!path) return;
    try {
      const ws = await OpenFolder(path);
      resetViewPane();
      setWorkspace(toWorkspace(ws));
      loadRecentProjects();
    } catch (err) {
      console.error("Failed to open folder:", err);
    }
  }, [setWorkspace, resetViewPane]);

  const handleAddFolderToWorkspace = useCallback(async () => {
    const path = await OpenFolderDialog();
    if (!path) return;
    try {
      if (!workspace) {
        const ws = await OpenFolder(path);
        resetViewPane();
        setWorkspace(toWorkspace(ws));
      } else {
        await AddFolderToWorkspace(path);
        const ws = await GetCurrentWorkspace();
        if (ws) setWorkspace(toWorkspace(ws));
      }
      loadRecentProjects();
    } catch (err) {
      console.error("Failed to add folder to workspace:", err);
    }
  }, [workspace, setWorkspace, resetViewPane]);

  const handleOpenWorkspace = useCallback(async () => {
    const path = await OpenWorkspaceDialog();
    if (!path) return;
    try {
      const ws = await OpenWorkspace(path);
      resetViewPane();
      setWorkspace(toWorkspace(ws));
      loadRecentProjects();
    } catch (err) {
      console.error("Failed to open workspace:", err);
    }
  }, [setWorkspace, resetViewPane]);

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
        resetViewPane();
        setWorkspace(toWorkspace(ws));
        loadRecentProjects();
      } catch (err) {
        console.error("Failed to open recent:", err);
      }
    },
    [setWorkspace, resetViewPane],
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
      let shellName = name?.trim();
      if (!shellName || shellName === "Shell" || shellName === "Terminal") {
        try {
          const existing = await ListSessions();
          const names = new Set(existing.map((s: any) => (s.name || s.Name || "").trim()));
          if (!names.has("Shell")) {
            shellName = "Shell";
          } else {
            let num = 2;
            while (names.has(`Shell ${num}`)) {
              num++;
            }
            shellName = `Shell ${num}`;
          }
        } catch {
          shellName = "Shell";
        }
      }
      try {
        const s = await CreateShell(shellName, workspace?.folders[0] ?? "");

        const newTab = {
          id: s.id,
          name: s.name || shellName,
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

    // Creates an agent immediately with the default "coding" role and an
    // auto-numbered name — no confirmation modal.
    const handleCreateAgent = useCallback(async () => {
      const baseName = "Agent (coding)";
    let name = baseName;
    try {
      const existing = await ListAgentSessions();
      const names = new Set(existing.map((a: any) => (a.name || a.Name || "").trim()));
      if (names.has(baseName)) {
        let num = 2;
        while (names.has(`${baseName} ${num}`)) num++;
        name = `${baseName} ${num}`;
      }
    } catch {
      name = baseName;
    }
    try {
      const a = await CreateAgentSession(name, "coding", workspace?.folders[0] ?? "");
      if (!a) throw new Error("Agent session creation failed");

      const newTab = {
        id: a.id,
        name: a.name || name,
        path: a.id,
        type: "agent" as "agent",
        content: "",
        modified: false,
      };

      setFiles((prev) => [...prev, newTab]);
      setActiveFileIndex(files.length);
      // The sidebar/session list self-refreshes via the agent:updated event.
      setActiveScreen("editor");
    } catch (err) {
      console.error("Failed to create agent session:", err);
    }
  }, [
    workspace,
    files.length,
    setFiles,
    setActiveFileIndex,
  ]);

  const handleRequestCreateShell = useCallback(() => {
    setActiveScreen("editor");
    setShowShellNameModal(true);
  }, []);

  const handleRequestCreateAgent = useCallback(() => {
    setActiveScreen("editor");
    void handleCreateAgent();
  }, [handleCreateAgent]);

  // Stable callbacks so React.memo(Sidebar) can skip re-renders on App churn.
  const handleOpenSession = useCallback(() => setActiveScreen("editor"), []);
  const handleOpenSettings = useCallback(() => setShowSettingsModal(true), []);
  const handleOpenUsage = useCallback(() => setActiveScreen("usage"), []);

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
      let content = getGlobalLiveContent(file.id) ?? file.content ?? "";
      const settings = loadEditorSettings();
      if (settings.formatOnSave) {
        // Format the LIVE buffer — the previous code formatted the stale
        // `file.content` and silently dropped unsaved keystrokes.
        const formatted = await formatWithSettings(file.path, content);
        if (formatted && formatted !== content) {
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

      // Direct Command Palette & Quick Open Shortcuts (RFC)
      if (
        pressedShortcut === "meta+shift+p" ||
        pressedShortcut === "ctrl+shift+p" ||
        pressedShortcut === "f1"
      ) {
        setShowCommandPalette(true);
        setPaletteMode("command");
        return;
      }
      if (pressedShortcut === "meta+p" || pressedShortcut === "ctrl+p") {
        setShowCommandPalette(true);
        setPaletteMode("file");
        return;
      }
      if (pressedShortcut === "meta+shift+o" || pressedShortcut === "ctrl+shift+o") {
        setShowCommandPalette(true);
        setPaletteMode("doc-symbol");
        return;
      }
      if (pressedShortcut === "ctrl+g") {
        setShowCommandPalette(true);
        setPaletteMode("line");
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
          setShowCommandPalette(true);
          setPaletteMode("file");
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
                    handleAddFolderToWorkspace();
                  }}
                  className="w-full text-left px-3 py-1.5 hover:bg-[var(--bg-panel)] flex items-center gap-2 text-[var(--fg-primary)] cursor-pointer"
                >
                  <IconFolderPlus className="size-3.5 text-emerald-400" />
                  <span>Add Folder to Workspace</span>
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
                    handleClose();
                  }}
                  className="w-full text-left px-3 py-1.5 hover:bg-[var(--bg-panel)] flex items-center gap-2 text-[var(--fg-primary)] cursor-pointer"
                >
                  <IconX className="size-3.5 text-red-400" />
                  <span>Close Workspace</span>
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
        </div>
        {/* Navigation history: back / forward across tabs and screens */}
        <div className="flex items-center gap-0.5 titlebar-no-drag">
          <button
            onClick={handleNavBack}
            disabled={!canGoBack()}
            className="inline-flex items-center justify-center p-1.5 rounded transition-colors cursor-pointer text-[var(--fg-secondary)] hover:text-white hover:bg-[var(--bg-surface-hover)] disabled:opacity-30 disabled:cursor-default disabled:hover:bg-transparent"
            title="Go Back (Alt+←)"
            aria-label="Go Back"
          >
            <IconArrowLeft className="size-4" />
          </button>
          <button
            onClick={handleNavForward}
            disabled={!canGoForward()}
            className="inline-flex items-center justify-center p-1.5 rounded transition-colors cursor-pointer text-[var(--fg-secondary)] hover:text-white hover:bg-[var(--bg-surface-hover)] disabled:opacity-30 disabled:cursor-default disabled:hover:bg-transparent"
            title="Go Forward (Alt+→)"
            aria-label="Go Forward"
          >
            <IconArrowRight className="size-4" />
          </button>
        </div>
        <div className="w-px h-3.5 bg-[var(--border-default)] mx-2" />
        {/* Center Quick Search / Command Palette Bar */}
        <div className="flex-1 max-w-sm mx-auto flex items-center justify-center titlebar-no-drag">
          <button
            onClick={() => {
              setShowCommandPalette(true);
              setPaletteMode("command");
            }}
            className="w-full flex items-center justify-between px-2.5 py-1 bg-[var(--bg-app)]/70 hover:bg-[var(--bg-app)] border border-[var(--border-default)] rounded text-[11px] text-[var(--fg-tertiary)] hover:text-[var(--fg-primary)] transition-all cursor-pointer shadow-sm group"
            title="Open Command Palette (⌘P or ⇧⌘P)"
          >
            <div className="flex items-center gap-1.5 truncate">
              <IconSearch className="size-3.5 text-[var(--fg-tertiary)] group-hover:text-[var(--accent-primary)]" />
              <span className="truncate">Search commands, files, or symbols...</span>
            </div>
            <span className="font-mono text-[9px] px-1 py-0.2 rounded bg-black/20 border border-[var(--border-default)] shrink-0 ml-2">
              ⌘P
            </span>
          </button>
        </div>

        <div className="flex-1" />

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
              onOpenUsage={handleOpenUsage}
            />
          }
          right={
            <main
              ref={mainRef}
              className="flex-1 h-full overflow-hidden bg-[var(--bg-app)]"
            >
              {activeScreen === "git-graph" ? (
                <GitGraphPanel />
              ) : activeScreen === "usage" ? (
                <UsagePanel />
              ) : (
                <Editor />
              )}
            </main>
          }
          initialLeftWidth={300}
          minLeftWidth={200}
          maxLeftWidth={700}
          collapsedWidth={40}
        />
      </div>

      {/* Status Bar */}
      <SessionsBar
        onSelectSession={() => {}}
        cwd={workspace.folders[0]}
        onCreateShell={handleRequestCreateShell}
        onOpenGitGraph={() => setActiveScreen("git-graph")}
        onOpenLSPModal={(pos) => {
          setLspModalPos(pos);
          setShowLSPModal(true);
        }}
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
          icon: <IconFolderOpen className="size-4 text-amber-400" />,
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
      {/* Lapce-Style Command Palette (RFC Compliant) */}
      <CommandPalette
        open={showCommandPalette}
        initialMode={paletteMode}
        onClose={() => setShowCommandPalette(false)}
        onOpenFolder={handleOpenFolder}
        onOpenWorkspace={handleOpenWorkspace}
        onSaveFile={handleSaveActiveFile}
        onCloseFile={handleCloseActiveFile}
        onCreateShell={() => handleCreateShell()}
        onCreateAgent={() => handleRequestCreateAgent()}
        onOpenSettings={() => setShowSettingsModal(true)}
        onOpenUsage={() => setActiveScreen("usage")}
        onSwitchScreen={setActiveScreen}
        onFormatCode={() => {
          const f = files[activeFileIndex];
          if (f && f.type === "file") {
            // Prefer the live buffer so unsaved keystrokes are formatted too.
            const live = getGlobalLiveContent(f.id);
            const content = live !== undefined ? live : (f.content ?? "");
            formatWithSettings(f.path, content).then((formatted) => {
              if (formatted && formatted !== content) applyFormattedContent(formatted);
            });
          }
        }}
        onToggleSidebar={() => setSidebarCollapsed((prev) => !prev)}
        onOpenFileByPath={globalOpenFile}
        onOpenLSPModal={() => setShowLSPModal(true)}
      />
      {/* LSP Server Management Popover matching Image 1 */}
      <LSPModal
        open={showLSPModal}
        onClose={() => setShowLSPModal(false)}
        anchorPosition={lspModalPos}
      />
    </div>
  );
}

export default App;
