import { create } from 'zustand';
import { CreateShell, StopSession } from '../lib/wails';
import { showToast } from '../lib/toast';

export interface TerminalStoreState {
  isBottomTerminalOpen: boolean;
  activeTerminalId: string | null;

  setIsBottomTerminalOpen: (val: boolean | ((prev: boolean) => boolean)) => void;
  setActiveTerminalId: (id: string | null) => void;
  createShellSession: (title: string, cwd: string) => Promise<string | null>;
  stopShellSession: (sessionId: string) => Promise<void>;
}

export const useTerminalStore = create<TerminalStoreState>((set) => ({
  isBottomTerminalOpen: false,
  activeTerminalId: null,

  setIsBottomTerminalOpen: (val) =>
    set((state) => ({
      isBottomTerminalOpen: typeof val === 'function' ? val(state.isBottomTerminalOpen) : val,
    })),

  setActiveTerminalId: (activeTerminalId) => set({ activeTerminalId }),

  createShellSession: async (title: string, cwd: string) => {
    try {
      const sess = await CreateShell(title, cwd);
      if (sess?.id) {
        set({ activeTerminalId: sess.id });
        return sess.id;
      }
      return null;
    } catch (err) {
      console.warn('Failed to create shell session:', err);
      showToast(err instanceof Error ? err.message : String(err), 'error');
      return null;
    }
  },

  stopShellSession: async (sessionId: string) => {
    try {
      await StopSession(sessionId);
    } catch (err) {
      console.warn('Failed to stop shell session:', err);
    }
  },
}));
