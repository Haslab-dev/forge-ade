import { create } from 'zustand';
import type { ActivityBarItem, CommandPaletteMode } from '../types';

export const LEFT_SIDEBAR_MIN_WIDTH = 240;
export const LEFT_SIDEBAR_MAX_RATIO = 0.5;
export const RIGHT_PANE_MIN_WIDTH = 360;
export const RIGHT_PANE_MAX_RATIO = 0.6;

export function clampLeftSidebarWidth(width: number): number {
  if (typeof window === 'undefined') return Math.max(LEFT_SIDEBAR_MIN_WIDTH, width);
  const maxWidth = Math.max(LEFT_SIDEBAR_MIN_WIDTH, Math.floor(window.innerWidth * LEFT_SIDEBAR_MAX_RATIO));
  return Math.round(Math.min(Math.max(width, LEFT_SIDEBAR_MIN_WIDTH), maxWidth));
}

export function clampRightPaneWidth(width: number): number {
  if (typeof window === 'undefined') return Math.max(RIGHT_PANE_MIN_WIDTH, width);
  const maxWidth = Math.max(RIGHT_PANE_MIN_WIDTH, Math.floor(window.innerWidth * RIGHT_PANE_MAX_RATIO));
  return Math.round(Math.min(Math.max(width, RIGHT_PANE_MIN_WIDTH), maxWidth));
}

export interface LayoutStoreState {
  activeActivity: ActivityBarItem;
  agentLeftSidebarOpen: boolean;
  editorLeftSidebarOpen: boolean;
  rightActionDrawerOpen: boolean;
  leftSidebarWidth: number;
  rightPaneWidth: number;

  agentViewMode: 'session' | 'board' | 'git';
  leftSidebarView: 'tasks' | 'files';

  isFolderModalOpen: boolean;
  isSSHModalOpen: boolean;
  isRemoteUploadModalOpen: boolean;
  remoteUploadTargetDir: string;
  isCommandPaletteOpen: boolean;
  commandPaletteMode: CommandPaletteMode;
  isAutomationsOpen: boolean;
  isCustomizationsModalOpen: boolean;
  isSettingsModalOpen: boolean;
  settingsActiveSection: string;

  setActiveActivity: (act: ActivityBarItem) => void;
  setAgentLeftSidebarOpen: (val: boolean | ((prev: boolean) => boolean)) => void;
  setEditorLeftSidebarOpen: (val: boolean | ((prev: boolean) => boolean)) => void;
  setRightActionDrawerOpen: (val: boolean | ((prev: boolean) => boolean)) => void;
  setLeftSidebarWidth: (width: number) => void;
  setRightPaneWidth: (width: number) => void;

  setAgentViewMode: (mode: 'session' | 'board' | 'git') => void;
  setLeftSidebarView: (view: 'tasks' | 'files') => void;

  setIsFolderModalOpen: (val: boolean) => void;
  setIsSSHModalOpen: (val: boolean) => void;
  setIsRemoteUploadModalOpen: (val: boolean) => void;
  setRemoteUploadTargetDir: (dir: string) => void;
  setIsCommandPaletteOpen: (val: boolean) => void;
  setCommandPaletteMode: (mode: CommandPaletteMode) => void;
  openCommandPalette: (mode?: CommandPaletteMode) => void;
  setIsAutomationsOpen: (val: boolean | ((prev: boolean) => boolean)) => void;
  setIsCustomizationsModalOpen: (val: boolean) => void;
  setIsSettingsModalOpen: (val: boolean) => void;
  setSettingsActiveSection: (section: string) => void;
}

const getStoredBool = (key: string, fallback: boolean): boolean => {
  if (typeof window === 'undefined') return fallback;
  const val = localStorage.getItem(key);
  return val !== null ? val === 'true' : fallback;
};

const getStoredNumber = (key: string, min: number, fallback: number): number => {
  if (typeof window === 'undefined') return fallback;
  try {
    const val = Number(localStorage.getItem(key));
    if (Number.isFinite(val) && val >= min) return val;
  } catch {}
  return fallback;
};

export const useLayoutStore = create<LayoutStoreState>((set) => ({
  activeActivity: 'explorer',
  agentLeftSidebarOpen: getStoredBool('forge_ade_agent_left_sidebar', true),
  editorLeftSidebarOpen: getStoredBool('forge_ade_editor_left_sidebar', true),
  rightActionDrawerOpen: getStoredBool('forge_ade_right_sidebar', false),
  leftSidebarWidth: getStoredNumber('forge_ade_left_sidebar_width', LEFT_SIDEBAR_MIN_WIDTH, 360),
  rightPaneWidth: getStoredNumber('forge_ade_side_pane_width', RIGHT_PANE_MIN_WIDTH, 540),

  agentViewMode: 'session',
  leftSidebarView: 'tasks',

  isFolderModalOpen: false,
  isSSHModalOpen: false,
  isRemoteUploadModalOpen: false,
  remoteUploadTargetDir: '',
  isCommandPaletteOpen: false,
  commandPaletteMode: 'files',
  isAutomationsOpen: false,
  isCustomizationsModalOpen: false,
  isSettingsModalOpen: false,
  settingsActiveSection: 'general',

  setActiveActivity: (activeActivity) => set({ activeActivity }),

  setAgentLeftSidebarOpen: (val) =>
    set((state) => {
      const next = typeof val === 'function' ? val(state.agentLeftSidebarOpen) : val;
      try { localStorage.setItem('forge_ade_agent_left_sidebar', String(next)); } catch {}
      return { agentLeftSidebarOpen: next };
    }),

  setEditorLeftSidebarOpen: (val) =>
    set((state) => {
      const next = typeof val === 'function' ? val(state.editorLeftSidebarOpen) : val;
      try { localStorage.setItem('forge_ade_editor_left_sidebar', String(next)); } catch {}
      return { editorLeftSidebarOpen: next };
    }),

  setRightActionDrawerOpen: (val) =>
    set((state) => {
      const next = typeof val === 'function' ? val(state.rightActionDrawerOpen) : val;
      try {
        localStorage.setItem('forge_ade_right_sidebar', String(next));
        localStorage.setItem('forge_ade_agent_right_sidebar', String(next));
        localStorage.setItem('forge_ade_editor_right_sidebar', String(next));
      } catch {}
      return { rightActionDrawerOpen: next };
    }),

  setLeftSidebarWidth: (width) => {
    const clamped = clampLeftSidebarWidth(width);
    try { localStorage.setItem('forge_ade_left_sidebar_width', String(clamped)); } catch {}
    set({ leftSidebarWidth: clamped });
  },

  setRightPaneWidth: (width) => {
    const clamped = clampRightPaneWidth(width);
    try { localStorage.setItem('forge_ade_side_pane_width', String(clamped)); } catch {}
    set({ rightPaneWidth: clamped });
  },

  setAgentViewMode: (agentViewMode) => set({ agentViewMode }),
  setLeftSidebarView: (leftSidebarView) => set({ leftSidebarView }),

  setIsFolderModalOpen: (isFolderModalOpen) => set({ isFolderModalOpen }),
  setIsSSHModalOpen: (isSSHModalOpen) => set({ isSSHModalOpen }),
  setIsRemoteUploadModalOpen: (isRemoteUploadModalOpen) => set({ isRemoteUploadModalOpen }),
  setRemoteUploadTargetDir: (remoteUploadTargetDir) => set({ remoteUploadTargetDir }),
  setIsCommandPaletteOpen: (isCommandPaletteOpen) => set({ isCommandPaletteOpen }),
  setCommandPaletteMode: (commandPaletteMode) => set({ commandPaletteMode }),
  openCommandPalette: (mode = 'files') =>
    set({ commandPaletteMode: mode, isCommandPaletteOpen: true }),
  setIsAutomationsOpen: (val) =>
    set((state) => ({ isAutomationsOpen: typeof val === 'function' ? val(state.isAutomationsOpen) : val })),
  setIsCustomizationsModalOpen: (isCustomizationsModalOpen) => set({ isCustomizationsModalOpen }),
  setIsSettingsModalOpen: (isSettingsModalOpen) => set({ isSettingsModalOpen }),
  setSettingsActiveSection: (settingsActiveSection) => set({ settingsActiveSection }),
}));
