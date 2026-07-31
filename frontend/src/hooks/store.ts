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

const savedTheme = typeof window !== "undefined" ? localStorage.getItem("forge-ade-theme") || "zed" : "zed";

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
