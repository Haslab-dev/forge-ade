import { create } from "zustand";
import { Workspace, RecentEntry, EditorFile, ShortcutKeybinding } from "../types";

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

const savedTheme = typeof window !== "undefined" ? localStorage.getItem("forge-ade-theme") || "dark-plus" : "dark-plus";

export const useUIStore = create<UIState>((set) => ({
  theme: savedTheme,
  setTheme: (theme) => {
    if (typeof window !== "undefined") localStorage.setItem("forge-ade-theme", theme);
    set({ theme });
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

// Shortcuts Store
interface ShortcutsState {
  keybindings: ShortcutKeybinding[];
  setKeybindings: (kb: ShortcutKeybinding[]) => void;
}

const defaultKeybindings: ShortcutKeybinding[] = [
  { id: "save-file", name: "Save Active File", key: "meta+s" },
  { id: "close-file", name: "Close Active File", key: "meta+w" },
  { id: "toggle-sidebar", name: "Toggle Sidebar Explorer", key: "meta+b" },
  { id: "toggle-terminal", name: "Toggle Terminal Panel", key: "ctrl+`" },
  { id: "toggle-agent", name: "Toggle Agent Chat Panel", key: "meta+i" },
  { id: "open-file", name: "Open File Path", key: "meta+p" },
  { id: "new-terminal", name: "Launch New Terminal Shell", key: "ctrl+shift+t" },
];

const savedKeybindings = typeof window !== "undefined" 
  ? localStorage.getItem("forge-ade-keybindings") 
  : null;

export const useShortcutsStore = create<ShortcutsState>((set) => ({
  keybindings: savedKeybindings ? JSON.parse(savedKeybindings) : defaultKeybindings,
  setKeybindings: (keybindings) => {
    if (typeof window !== "undefined") {
      localStorage.setItem("forge-ade-keybindings", JSON.stringify(keybindings));
    }
    set({ keybindings });
  },
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
// App.tsx's global open-in-browser handler (terminal links etc.).
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

// Session Layout Store for Shell/Agent Screen
interface SessionLayoutState {
  layoutMode: "single" | "horizontal" | "grid";
  closedViewSessionIds: string[];
  selectedSessionId: string | null;
  panelShares: Record<string, number>;
  setLayoutMode: (mode: "single" | "horizontal" | "grid") => void;
  closeView: (id: string) => void;
  reopenView: (id: string) => void;
  setSelectedSessionId: (id: string | null) => void;
  setPanelShare: (id: string, share: number) => void;
}

export const useSessionLayoutStore = create<SessionLayoutState>((set) => ({
  layoutMode: "single",
  closedViewSessionIds: [],
  selectedSessionId: null,
  panelShares: {},
  setLayoutMode: (layoutMode) => set({ layoutMode }),
  closeView: (id) => set((state) => ({
    closedViewSessionIds: state.closedViewSessionIds.includes(id) 
      ? state.closedViewSessionIds 
      : [...state.closedViewSessionIds, id],
  })),
  reopenView: (id) => set((state) => ({
    closedViewSessionIds: state.closedViewSessionIds.filter((x) => x !== id),
  })),
  setSelectedSessionId: (selectedSessionId) => set({ selectedSessionId }),
  setPanelShare: (id, share) => set((state) => ({
    panelShares: { ...state.panelShares, [id]: share },
  })),
}));
