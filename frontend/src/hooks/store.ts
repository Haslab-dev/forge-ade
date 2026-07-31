import { create } from "zustand";
import type { Workspace, RecentEntry } from "../types";

export type ThemeId =
  | "forge-ade-dark"
  | "forge-ade-light"
  | "vscode-dark"
  | "vscode-light"
  | "codex"
  | "zed"
  | "cursor"
  | "dark"
  | "light";

interface WorkspaceState {
  workspace: Workspace | null;
  recentProjects: RecentEntry[];
  isLoaded: boolean;

  setWorkspace: (ws: Workspace | null) => void;
  setRecentProjects: (entries: RecentEntry[]) => void;
  setIsLoaded: (loaded: boolean) => void;
}

export const useWorkspaceStore = create<WorkspaceState>((set) => ({
  workspace: null,
  recentProjects: [],
  isLoaded: false,

  setWorkspace: (ws) => set({ workspace: ws, isLoaded: true }),
  setRecentProjects: (entries) => set({ recentProjects: entries }),
  setIsLoaded: (loaded) => set({ isLoaded: loaded }),
}));

interface UIState {
  activePanel: string | null;
  showHiddenFiles: boolean;
  theme: string;

  setActivePanel: (panel: string | null) => void;
  setShowHiddenFiles: (show: boolean) => void;
  setTheme: (theme: string) => void;
}

const savedTheme = typeof window !== "undefined" ? localStorage.getItem("forge-ade-theme") || "forge-ade-dark" : "forge-ade-dark";

export const useUIStore = create<UIState>((set) => ({
  activePanel: "explorer",
  showHiddenFiles: true,
  theme: savedTheme,

  setActivePanel: (panel) => set({ activePanel: panel }),
  setShowHiddenFiles: (show) => set({ showHiddenFiles: show }),
  setTheme: (theme) => {
    if (typeof window !== "undefined") localStorage.setItem("forge-ade-theme", theme);
    set({ theme });
  },
}));

export interface OpenFile {
  path: string;
  name: string;
  content: string;
  modified: boolean;
  targetLine?: number;
}

interface EditorState {
  files: OpenFile[];
  activeFileIndex: number;
  setFiles: (files: OpenFile[] | ((prev: OpenFile[]) => OpenFile[])) => void;
  setActiveFileIndex: (index: number | ((prev: number) => number)) => void;
}

export const useEditorStore = create<EditorState>((set) => ({
  files: [],
  activeFileIndex: -1,
  setFiles: (files) =>
    set((state) => ({
      files: typeof files === "function" ? files(state.files) : files,
    })),
  setActiveFileIndex: (index) =>
    set((state) => ({
      activeFileIndex:
        typeof index === "function" ? index(state.activeFileIndex) : index,
    })),
}));

