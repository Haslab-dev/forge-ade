import { useEffect, useCallback, useState } from "react";
import "./style.css";
import { useWorkspaceStore, useUIStore } from "./hooks/store";
import { Welcome } from "./panels/welcome";
import { Sidebar } from "./components/sidebar";
import { SessionsBar } from "./components/sessions-bar";
import { Editor } from "./panels/editor";
import type { RecentEntry, Workspace } from "./types";
import {
  GetRecentProjects,
  OpenFolder,
  OpenWorkspace,
  CloseWorkspace,
  SaveWorkspace,
  SaveWorkspaceAs,
  PinRecent,
  RemoveRecent,
  OpenFolderDialog,
  OpenWorkspaceDialog,
  ListSessions,
  StopSession,
} from "../wailsjs/go/main/App";
import { terminal } from "../wailsjs/go/models";
import { FolderOpen, FileText, Save, Download } from "lucide-react";

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
  const [activeSessionId, setActiveSessionId] = useState<string | null>(null);
  const [sessions, setSessions] = useState<terminal.Session[]>([]);
  const [openSessionIds, setOpenSessionIds] = useState<string[]>([]);

  // Sync theme
  useEffect(() => {
    document.documentElement.classList.toggle("dark", theme === "dark");
  }, [theme]);

  // Load recent projects on mount
  useEffect(() => {
    loadRecentProjects();
  }, []);

  // Poll sessions
  useEffect(() => {
    const poll = async () => {
      try {
        const list: terminal.Session[] = await ListSessions();
        setSessions(Array.isArray(list) ? list : []);
      } catch { /* ignore */ }
    };
    poll();
    const interval = setInterval(poll, 2000);
    return () => clearInterval(interval);
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

  const handleSaveWorkspace = useCallback(async () => {
    if (!workspace) return;
    if (workspace.isTemporary || !workspace.filePath) {
      const path = await OpenWorkspaceDialog();
      if (!path) return;
      try {
        await SaveWorkspaceAs(path);
        setWorkspace({ ...workspace, filePath: path, isTemporary: false });
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
        const ws = entry.isWorkspace
          ? await OpenWorkspace(entry.path)
          : await OpenFolder(entry.path);
        setWorkspace(toWorkspace(ws));
        loadRecentProjects();
      } catch (err) {
        console.error("Failed to open:", err);
      }
    },
    [setWorkspace]
  );

  const handlePinRecent = useCallback(async (path: string, pinned: boolean) => {
    try {
      await PinRecent(path, pinned);
      loadRecentProjects();
    } catch (err) {
      console.error(err);
    }
  }, []);

  const handleRemoveRecent = useCallback(async (path: string) => {
    try {
      await RemoveRecent(path);
      loadRecentProjects();
    } catch (err) {
      console.error(err);
    }
  }, []);

  const handleClose = useCallback(async () => {
    try {
      await CloseWorkspace();
      setWorkspace(null);
    } catch (err) {
      console.error(err);
    }
  }, [setWorkspace]);

  const handleSelectSession = useCallback((id: string | null) => {
    setActiveSessionId(id);
    // Add to open tabs if not already there
    if (id) {
      setOpenSessionIds((prev) => prev.includes(id) ? prev : [...prev, id]);
    }
  }, []);

  const handleCloseSessionTab = useCallback((id: string) => {
    // Remove tab from bar — session keeps running
    setOpenSessionIds((prev) => prev.filter((sid) => sid !== id));
    setActiveSessionId((prev) => prev === id ? null : prev);
  }, []);

  const handleRenameSession = useCallback(async (_id: string, _name: string) => {
    // Refresh session list immediately after rename
    try {
      const list: terminal.Session[] = await ListSessions();
      setSessions(Array.isArray(list) ? list : []);
    } catch { /* ignore */ }
  }, []);

  const handleStopSession = useCallback(async (id: string) => {
    // Kill the process — session stays in list as "stopped"
    try {
      await StopSession(id);
      // Refresh list to show updated status
      const list: terminal.Session[] = await ListSessions();
      setSessions(Array.isArray(list) ? list : []);
      if (activeSessionId === id) setActiveSessionId(null);
    } catch (err) {
      console.error(err);
    }
  }, [activeSessionId]);

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
        <div className="flex-1" />
        <div className="flex items-center gap-0.5 titlebar-no-drag">
          <button className="inline-flex items-center gap-1 px-2 py-1 text-xs text-muted-foreground hover:text-foreground hover:bg-accent rounded" onClick={handleOpenFolder} title="Open Project">
            <FolderOpen className="size-3.5" />
            <span className="hidden sm:inline">Open Project</span>
          </button>
          <button className="inline-flex items-center gap-1 px-2 py-1 text-xs text-muted-foreground hover:text-foreground hover:bg-accent rounded" onClick={handleOpenWorkspace} title="Open Workspace">
            <FileText className="size-3.5" />
            <span className="hidden sm:inline">Open Workspace</span>
          </button>
          <div className="w-px h-4 bg-border mx-1" />
          <button className="inline-flex items-center gap-1 px-2 py-1 text-xs text-muted-foreground hover:text-foreground hover:bg-accent rounded" onClick={handleSaveWorkspace} title="Save Workspace">
            <Save className="size-3.5" />
            <span className="hidden sm:inline">Save</span>
          </button>
          <button className="inline-flex items-center gap-1 px-2 py-1 text-xs text-muted-foreground hover:text-foreground hover:bg-accent rounded" onClick={handleClose} title="Close Workspace">
            <Download className="size-3.5" />
            <span className="hidden sm:inline">Close</span>
          </button>
        </div>
      </header>

      {/* Main Content — Editor manages both file tabs + session tabs */}
      <div className="flex flex-1 overflow-hidden">
        <Sidebar folders={workspace.folders} onOpenSession={handleSelectSession} />
        <main className="flex-1 overflow-hidden">
          <Editor
            sessionTabs={sessions.filter((s) => openSessionIds.includes(s.id))}
            activeSessionId={activeSessionId}
            onSelectSession={handleSelectSession}
            onCloseSession={handleCloseSessionTab}
            onRenameSession={handleRenameSession}
          />
        </main>
      </div>

      {/* Sessions Bar (bottom) — compact list, click to open tab */}
      <SessionsBar onSelectSession={handleSelectSession} />
    </div>
  );
}

export default App;
