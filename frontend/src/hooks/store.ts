import { create } from "zustand";
import type { Workspace, RecentEntry } from "../types";

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
  theme: "dark" | "light";

  setActivePanel: (panel: string | null) => void;
  setShowHiddenFiles: (show: boolean) => void;
  setTheme: (theme: "dark" | "light") => void;
}

export const useUIStore = create<UIState>((set) => ({
  activePanel: "explorer",
  showHiddenFiles: false,
  theme: "dark",

  setActivePanel: (panel) => set({ activePanel: panel }),
  setShowHiddenFiles: (show) => set({ showHiddenFiles: show }),
  setTheme: (theme) => set({ theme }),
}));
