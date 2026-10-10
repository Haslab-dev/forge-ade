import { create } from "zustand";
import { Workspace, RecentEntry, EditorFile } from "../types";

// Workspace Store
interface WorkspaceState {
  workspace: Workspace | null;
  recentProjects: RecentEntry[];
  setWorkspace: (ws: Workspace | null) => void;
  setRecentProjects: (projects: RecentEntry[]) => void;
}

export const useWorkspaceStore = create<WorkspaceState>((set) => ({
  workspace: null,
  recentProjects: [],
  setWorkspace: (workspace) => set({ workspace }),
  setRecentProjects: (recentProjects) => set({ recentProjects }),
}));

// UI Store
interface UIState {
  theme: string;
  setTheme: (theme: string) => void;
}

const savedTheme = typeof window !== "undefined" 
  ? (localStorage.getItem("forge-ade-theme") || "dark") 
  : "dark";

export const useUIStore = create<UIState>((set) => ({
  theme: savedTheme === "light" ? "light" : "dark",
  setTheme: (theme) => {
    const validTheme = theme === "light" ? "light" : "dark";
    if (typeof window !== "undefined") {
      localStorage.setItem("forge-ade-theme", validTheme);
      if (validTheme === "dark") {
        document.documentElement.classList.add("dark");
        document.documentElement.classList.remove("light");
        document.body.classList.add("dark");
        document.body.classList.remove("light");
      } else {
        document.documentElement.classList.remove("dark");
        document.documentElement.classList.add("light");
        document.body.classList.remove("dark");
        document.body.classList.add("light");
      }
    }
    set({ theme: validTheme });
  },
}));

// Editor Store
interface EditorState {
  files: EditorFile[];
  activeFileIndex: number;
  previewFile: string | null;
  setFiles: (files: EditorFile[] | ((prev: EditorFile[]) => EditorFile[])) => void;
  setActiveFileIndex: (index: number | ((prev: number) => number)) => void;
  setPreviewFile: (path: string | null) => void;
}

export const useEditorStore = create<EditorState>((set) => ({
  files: [],
  // NOTE: `files` entries pin full file content; add an eviction path if this
  // store ever becomes the primary open-file holder again (it currently only
  // feeds legacy global hooks).
  activeFileIndex: -1,
  previewFile: null,
  setFiles: (update) => set((state) => ({
    files: typeof update === "function" ? update(state.files) : update,
  })),
  setActiveFileIndex: (update) => set((state) => ({
    activeFileIndex: typeof update === "function" ? update(state.activeFileIndex) : update,
  })),
  setPreviewFile: (previewFile) => set({ previewFile }),
}));

// Workspace tab-panel store: browser tab + layout mode for the unified viewer.
// The browser is a tab in the same tab bar as files/shells/agents.
interface BrowserTab {
  id: string; // stable id, e.g. "browser:1"
  name: string;
  url: string;
}
interface WorkspaceTabState {
  browserTabs: BrowserTab[];
  activeBrowserTabId: string | null;
  workspaceLayoutMode: "single" | "horizontal" | "grid";
  paneShares: Record<string, number>; // tab id -> flex share in horizontal layout
  openBrowserTab: (url?: string) => string;
  closeBrowserTab: (id: string) => void;
  setActiveBrowserTab: (id: string | null) => void;
  setWorkspaceLayoutMode: (mode: "single" | "horizontal" | "grid") => void;
  setPaneShare: (id: string, share: number) => void;
}

let browserCounter = 0;
const browserTabId = () => `browser:${++browserCounter}`;

// Opens (or activates) a browser tab in the workspace tab panel. Used by
// tsx's global open-in-browser handler (terminal links etc.).
export function openBrowserTab(url = "") {
  const st = useWorkspaceTabStore.getState();
  const existing = st.browserTabs.find((t) => t.url === url && url !== "");
  if (existing) {
    st.setActiveBrowserTab(existing.id);
    return existing.id;
  }
  return st.openBrowserTab(url);
}

export const useWorkspaceTabStore = create<WorkspaceTabState>((set) => ({
  browserTabs: [],
  activeBrowserTabId: null,
  workspaceLayoutMode: "single",
  paneShares: {},
  openBrowserTab: (url = "") => {
    const id = browserTabId();
    set((state) => ({
      browserTabs: [...state.browserTabs, { id, name: "Browser", url }],
      activeBrowserTabId: id,
    }));
    return id;
  },
  closeBrowserTab: (id) => set((state) => {
    const tabs = state.browserTabs.filter((t) => t.id !== id);
    const active = state.activeBrowserTabId === id ? (tabs.length ? tabs[tabs.length - 1].id : null) : state.activeBrowserTabId;
    return { browserTabs: tabs, activeBrowserTabId: active };
  }),
  setActiveBrowserTab: (id) => set({ activeBrowserTabId: id }),
  setWorkspaceLayoutMode: (workspaceLayoutMode) => set({ workspaceLayoutMode }),
  setPaneShare: (id, share) => set((state) => ({
    paneShares: { ...state.paneShares, [id]: share },
  })),
}));
