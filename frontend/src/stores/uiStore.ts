import { create } from 'zustand';

export type AppSurface = 'workspace' | 'editor' | 'automations' | 'settings';

interface UIState {
  surface: AppSurface;
  isCommandPaletteOpen: boolean;
  isSettingsOpen: boolean;
  isAutomationsOpen: boolean;
  isSSHModalOpen: boolean;
  isRemoteUploadOpen: boolean;
  isBottomTerminalOpen: boolean;
  isRightSidebarOpen: boolean;

  setSurface: (surface: AppSurface) => void;
  setCommandPaletteOpen: (open: boolean) => void;
  setSettingsOpen: (open: boolean) => void;
  setAutomationsOpen: (open: boolean) => void;
  setSSHModalOpen: (open: boolean) => void;
  setRemoteUploadOpen: (open: boolean) => void;
  setBottomTerminalOpen: (open: boolean) => void;
  setRightSidebarOpen: (open: boolean) => void;
  toggleBottomTerminal: () => void;
  toggleRightSidebar: () => void;
}

export const useUIStore = create<UIState>((set) => ({
  surface: 'workspace',
  isCommandPaletteOpen: false,
  isSettingsOpen: false,
  isAutomationsOpen: false,
  isSSHModalOpen: false,
  isRemoteUploadOpen: false,
  isBottomTerminalOpen: false,
  isRightSidebarOpen: true,

  setSurface: (surface) => set({ surface }),
  setCommandPaletteOpen: (open) => set({ isCommandPaletteOpen: open }),
  setSettingsOpen: (open) => set({ isSettingsOpen: open }),
  setAutomationsOpen: (open) => set({ isAutomationsOpen: open }),
  setSSHModalOpen: (open) => set({ isSSHModalOpen: open }),
  setRemoteUploadOpen: (open) => set({ isRemoteUploadOpen: open }),
  setBottomTerminalOpen: (open) => set({ isBottomTerminalOpen: open }),
  setRightSidebarOpen: (open) => set({ isRightSidebarOpen: open }),
  toggleBottomTerminal: () => set((state) => ({ isBottomTerminalOpen: !state.isBottomTerminalOpen })),
  toggleRightSidebar: () => set((state) => ({ isRightSidebarOpen: !state.isRightSidebarOpen })),
}));
