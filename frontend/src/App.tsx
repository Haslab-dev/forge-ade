import { useEffect, useCallback, useState } from "react";
import "./style.css";
import { useWorkspaceStore, useUIStore } from "./hooks/store";
import { Welcome } from "./panels/welcome";
import { Sidebar } from "./components/sidebar";
import { SessionsBar } from "./components/sessions-bar";
import { Editor, globalOpenFile, setOnBeforeOpenFile } from "./panels/editor";
import { ShellScreen } from "./panels/shell-screen";
import { AgentScreen } from "./panels/agent-screen";
import { GitGraphPanel } from "./panels/git-graph-panel";
import { ResizableSplit } from "./components/resizable-split";
import {
  IconFileCode,
  IconTerminal2,
  IconRobot,
  IconGitBranch,
  IconFolder,
  IconFileText,
  IconFile,
  IconDeviceFloppy,
  IconX,
} from "@tabler/icons-react";
import { SimpleModal } from "./components/simple-modal";
import { cn } from "./lib/utils";
import type { RecentEntry, Workspace } from "./types";
import { ClipboardGetText } from "../wailsjs/runtime";
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
  ListSessions,
  StopSession,
  RenameSession,
  CreateShell,
} from "../wailsjs/go/main/App";
import { terminal } from "../wailsjs/go/models";
import { FolderOpen, FileText, File, Save, Download, FileCode2, Shell, Bot, GitBranch, SquareArrowOutUpRight } from "lucide-react";

function toWorkspace(ws: any): Workspace {
  return {
    name: ws.name ?? ws.Name ?? "Untitled",
    folders: ws.folders ?? ws.Folders ?? [],
    isTemporary: ws.isTemporary ?? ws.IsTemporary ?? true,
    filePath: ws.filePath ?? ws.FilePath ?? "",
    theme: ws.settings?.theme ?? ws.Settings?.Theme ?? "dark",
  };
}

function App() {
  const { workspace, recentProjects, setWorkspace, setRecentProjects } =
    useWorkspaceStore();
  const { theme } = useUIStore();
  const [activeScreen, setActiveScreen] = useState<"editor" | "shell" | "agent" | "git-graph">("editor");
  const [activeSessionId, setActiveSessionId] = useState<string | null>(null);
  const [sessions, setSessions] = useState<terminal.Session[]>([]);
  const [openSessionIds, setOpenSessionIds] = useState<string[]>([]);
  const [shellSessionIds, setShellSessionIds] = useState<string[]>([]);
  const [showShellNameModal, setShowShellNameModal] = useState(false);
  const [showOpenPathModal, setShowOpenPathModal] = useState(false);
  const [openPathValue, setOpenPathValue] = useState("");

  // Sync design token theme to root HTML element
  useEffect(() => {
    document.documentElement.className = theme;
    const isLight = theme.includes("light");
    document.documentElement.classList.toggle("dark", !isLight);
    document.documentElement.classList.toggle("light", isLight);
  }, [theme]);

  // Load recent projects on mount
  useEffect(() => {
    loadRecentProjects();
  }, []);

  // Auto-switch to Editor when a file is opened
  useEffect(() => {
    setOnBeforeOpenFile(() => setActiveScreen("editor"));
  }, []);

  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);

  // Auto-load active sessions
  useEffect(() => {
    const load = async () => {
      try {
        const list: terminal.Session[] = await ListSessions();
        setSessions(Array.isArray(list) ? list : []);
      } catch { /* ignore */ }
    };
    load();
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
    } catch { /* ignore */ }
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
    [setWorkspace]
  );

  const handlePinRecent = useCallback(
    async (path: string, pinned: boolean) => {
      try {
        await PinRecent(path, pinned);
        loadRecentProjects();
      } catch (err) {
        console.error("Failed to pin recent:", err);
      }
    },
    []
  );

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
      setSessions([]);
      setOpenSessionIds([]);
      setShellSessionIds([]);
      setActiveSessionId(null);
    } catch (err) {
      console.error("Failed to close workspace:", err);
    }
  }, [setWorkspace]);

  const handleSelectSession = useCallback((id: string | null) => {
    setActiveSessionId(id);
    if (id) {
      setOpenSessionIds((prev) => (prev.includes(id) ? prev : [...prev, id]));
      setShellSessionIds((prev) => (prev.includes(id) ? prev : [...prev, id]));
      setActiveScreen("shell");
    }
  }, []);

  const handleCloseSessionTab = useCallback(
    (id: string) => {
      setOpenSessionIds((prev) => prev.filter((s) => s !== id));
      if (activeSessionId === id) {
        const remaining = openSessionIds.filter((s) => s !== id);
        setActiveSessionId(remaining[remaining.length - 1] ?? null);
      }
    },
    [activeSessionId, openSessionIds]
  );

  const handleCloseShellSession = useCallback(
    (id: string) => {
      // Close tab view ONLY; DO NOT kill background process!
      setOpenSessionIds((prev) => prev.filter((s) => s !== id));
      if (activeSessionId === id) {
        const remaining = openSessionIds.filter((s) => s !== id);
        setActiveSessionId(remaining[remaining.length - 1] ?? null);
      }
    },
    [activeSessionId, openSessionIds]
  );

  const handleStopSession = useCallback(
    async (id: string) => {
      try {
        await StopSession(id);
        setSessions((prev) => prev.filter((s) => s.id !== id));
        setShellSessionIds((prev) => prev.filter((s) => s !== id));
        setOpenSessionIds((prev) => prev.filter((s) => s !== id));
        if (activeSessionId === id) {
          const remaining = shellSessionIds.filter((s) => s !== id);
          setActiveSessionId(remaining[remaining.length - 1] ?? null);
        }
      } catch (err) {
        console.error("Failed to stop session:", err);
      }
    },
    [activeSessionId, shellSessionIds]
  );

  const handleRenameSession = useCallback(async (id: string, name: string) => {
    try {
      await RenameSession(id, name);
      const list: terminal.Session[] = await ListSessions();
      setSessions(Array.isArray(list) ? list : []);
    } catch (err) {
      console.error("Failed to rename session:", err);
    }
  }, []);

  const handleCreateShell = useCallback(
    async (name?: string) => {
      const shellName = name?.trim() || "Shell";
      try {
        const s = await CreateShell(shellName, workspace?.folders[0] ?? "");
        setSessions((prev) => [...prev, s]);
        setShellSessionIds((prev) => [...prev, s.id]);
        setActiveSessionId(s.id);
        setActiveScreen("shell");
      } catch (err) {
        console.error("Failed to create shell:", err);
      }
    },
    [workspace]
  );

  const handleRequestCreateShell = useCallback(() => {
    setShowShellNameModal(true);
  }, []);

  const handleRefreshWorkspace = useCallback(async () => {
    try {
      const ws = await GetCurrentWorkspace();
      if (ws) setWorkspace(toWorkspace(ws));
    } catch { /* ignore */ }
  }, [setWorkspace]);

  // ---------- Welcome Screen ----------
  if (!workspace) {
    return (
      <div className="dark h-screen w-screen bg-background text-foreground">
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
    <div className="dark h-screen w-screen bg-background text-foreground flex flex-col overflow-hidden">
      {/* App Bar */}
      <header className="safe-area-top titlebar-drag flex items-center h-10 px-3 border-b bg-muted/30 text-sm shrink-0 gap-1">
        <span className="font-semibold tracking-tight mr-2">ForgeADE</span>
        <span className="text-muted-foreground mx-1">/</span>
        <span className="font-medium truncate max-w-48">{workspace.name}</span>
        {workspace.isTemporary && (
          <span className="ml-1.5 text-[10px] text-muted-foreground bg-muted px-1.5 py-0.5 rounded">temp</span>
        )}
        <div className="w-px h-4 bg-border mx-2" />

        <div className="flex items-center gap-0.5 titlebar-no-drag">
          <button
            className={cn(
              "inline-flex items-center gap-1 px-2 py-1 text-xs rounded transition-colors cursor-pointer",
              activeScreen === "editor"
                ? "bg-accent text-foreground"
                : "text-muted-foreground hover:text-foreground hover:bg-accent/50"
            )}
            onClick={() => setActiveScreen("editor")}
            title="Editor"
          >
            <IconFileCode className="size-3.5" />
            <span className="hidden sm:inline">Editor</span>
          </button>
          <button
            className={cn(
              "inline-flex items-center gap-1 px-2 py-1 text-xs rounded transition-colors cursor-pointer border border-transparent",
              activeScreen === "shell" || activeScreen === "agent"
                ? "bg-[var(--bg-surface-active)] text-white border-[var(--border-default)] font-semibold shadow-sm"
                : "text-muted-foreground hover:text-foreground hover:bg-accent/50"
            )}
            onClick={() => setActiveScreen("shell")}
            title="Sessions Orchestrator"
          >
            <IconTerminal2 className="size-3.5" />
            <span className="hidden sm:inline">Session</span>
          </button>
          <button
            className={cn(
              "inline-flex items-center gap-1 px-2 py-1 text-xs rounded transition-colors cursor-pointer",
              activeScreen === "git-graph"
                ? "bg-accent text-foreground"
                : "text-muted-foreground hover:text-foreground hover:bg-accent/50"
            )}
            onClick={() => setActiveScreen("git-graph")}
            title="Git Graph"
          >
            <IconGitBranch className="size-3.5 text-purple-400" />
            <span className="hidden sm:inline">Git Graph</span>
          </button>
        </div>
        <div className="flex-1" />
        <div className="flex items-center gap-0.5 titlebar-no-drag">
          <button className="inline-flex items-center gap-1 px-2 py-1 text-xs text-muted-foreground hover:text-foreground hover:bg-accent rounded" onClick={handleOpenFolder} title="Open Project">
            <IconFolder className="size-3.5" />
            <span className="hidden sm:inline">Open Project</span>
          </button>
          <button className="inline-flex items-center gap-1 px-2 py-1 text-xs text-muted-foreground hover:text-foreground hover:bg-accent rounded" onClick={handleOpenWorkspace} title="Open Workspace">
            <IconFileText className="size-3.5" />
            <span className="hidden sm:inline">Open Workspace</span>
          </button>
          <button className="inline-flex items-center gap-1 px-2 py-1 text-xs text-muted-foreground hover:text-foreground hover:bg-accent rounded" onClick={handleOpenFile} title="Open File">
            <IconFile className="size-3.5" />
            <span className="hidden sm:inline">Open File</span>
          </button>
          <button className="inline-flex items-center gap-1 px-2 py-1 text-xs text-muted-foreground hover:text-foreground hover:bg-accent rounded" onClick={handleNewWindow} title="Open New Window">
            <SquareArrowOutUpRight className="size-3.5" />
            <span className="hidden sm:inline">New Window</span>
          </button>
          <div className="w-px h-4 bg-border mx-1" />
          <button className="inline-flex items-center gap-1 px-2 py-1 text-xs text-muted-foreground hover:text-foreground hover:bg-accent rounded" onClick={handleSaveWorkspace} title="Save Workspace">
            <IconDeviceFloppy className="size-3.5" />
            <span className="hidden sm:inline">Save</span>
          </button>
          <button className="inline-flex items-center gap-1 px-2 py-1 text-xs text-muted-foreground hover:text-foreground hover:bg-accent rounded" onClick={handleClose} title="Close Workspace">
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
              onOpenSession={handleSelectSession}
              sessions={sessions}
              onRefreshSessions={async () => {
                try {
                  const list: terminal.Session[] = await ListSessions();
                  setSessions(Array.isArray(list) ? list : []);
                } catch { /* ignore */ }
              }}
              cwd={workspace.folders[0]}
              onCreateShell={handleRequestCreateShell}
              onRefreshWorkspace={handleRefreshWorkspace}
              collapsed={sidebarCollapsed}
              onToggleCollapse={setSidebarCollapsed}
            />
          }
          right={
            <main className="flex-1 h-full overflow-hidden">
              {activeScreen === "editor" ? (
                <Editor
                  sessionTabs={sessions.filter((s) => openSessionIds.includes(s.id))}
                  activeSessionId={activeSessionId}
                  onSelectSession={handleSelectSession}
                  onCloseSession={handleCloseSessionTab}
                  onRenameSession={handleRenameSession}
                  onCreateShell={handleRequestCreateShell}
                />
              ) : activeScreen === "shell" ? (
                <ShellScreen
                  sessions={sessions.filter((s) => shellSessionIds.includes(s.id))}
                  onCreateShell={handleRequestCreateShell}
                  onCloseSession={handleCloseShellSession}
                  onStopSession={handleStopSession}
                  onRenameSession={handleRenameSession}
                  initialSessionId={activeSessionId}
                />
              ) : activeScreen === "agent" ? (
                <AgentScreen />
              ) : (
                <GitGraphPanel />
              )}
            </main>
          }
          initialLeftWidth={280}
          minLeftWidth={180}
          maxLeftWidth={600}
        />
      </div>

      {/* Sessions Bar (bottom) — compact list, click to open tab */}
      <SessionsBar onSelectSession={handleSelectSession} cwd={workspace.folders[0]} onCreateShell={handleRequestCreateShell} />

      {/* Shell name modal */}
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

      {/* Open File modal with options for Finder/Explorer and Input Path */}
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
    </div>
  );
}

export default App;
