import { create } from 'zustand';
import type { EditorTab } from '../types';

interface EditorState {
  openTabs: EditorTab[];
  activeTabId: string | null;
  isSplitEditor: boolean;

  openTab: (tab: EditorTab) => void;
  closeTab: (id: string) => void;
  closeOtherTabs: (id: string) => void;
  closeAllTabs: () => void;
  setActiveTabId: (id: string | null) => void;
  updateTab: (id: string, patch: Partial<EditorTab>) => void;
  togglePinTab: (id: string) => void;
  setIsSplitEditor: (val: boolean) => void;
  setOpenTabs: (tabs: EditorTab[] | ((prev: EditorTab[]) => EditorTab[])) => void;
}

export const useEditorStore = create<EditorState>((set) => ({
  openTabs: [],
  activeTabId: null,
  isSplitEditor: false,

  openTab: (tab: EditorTab) =>
    set((state) => {
      const existing = state.openTabs.find((t) => t.id === tab.id || (t.filePath && t.filePath === tab.filePath && t.type === tab.type));
      if (existing) {
        return {
          activeTabId: existing.id,
          openTabs: state.openTabs.map((t) => (t.id === existing.id ? { ...t, ...tab, id: existing.id } : t)),
        };
      }
      return {
        openTabs: [...state.openTabs, tab],
        activeTabId: tab.id,
      };
    }),

  closeTab: (id: string) =>
    set((state) => {
      const idx = state.openTabs.findIndex((t) => t.id === id);
      if (idx === -1) return state;

      const newTabs = state.openTabs.filter((t) => t.id !== id);
      let newActiveId = state.activeTabId;

      if (state.activeTabId === id) {
        if (newTabs.length === 0) {
          newActiveId = null;
        } else if (idx < newTabs.length) {
          newActiveId = newTabs[idx].id;
        } else {
          newActiveId = newTabs[newTabs.length - 1].id;
        }
      }

      return {
        openTabs: newTabs,
        activeTabId: newActiveId,
      };
    }),

  closeOtherTabs: (id: string) =>
    set((state) => ({
      openTabs: state.openTabs.filter((t) => t.id === id || t.isPinned),
      activeTabId: id,
    })),

  closeAllTabs: () =>
    set((state) => {
      const pinned = state.openTabs.filter((t) => t.isPinned);
      return {
        openTabs: pinned,
        activeTabId: pinned.length > 0 ? pinned[0].id : null,
      };
    }),

  setActiveTabId: (id: string | null) => set({ activeTabId: id }),

  updateTab: (id: string, patch: Partial<EditorTab>) =>
    set((state) => ({
      openTabs: state.openTabs.map((t) => (t.id === id ? { ...t, ...patch } : t)),
    })),

  togglePinTab: (id: string) =>
    set((state) => ({
      openTabs: state.openTabs.map((t) => (t.id === id ? { ...t, isPinned: !t.isPinned } : t)),
    })),

  setIsSplitEditor: (val: boolean) => set({ isSplitEditor: val }),

  setOpenTabs: (tabsOrFn) =>
    set((state) => ({
      openTabs: typeof tabsOrFn === 'function' ? tabsOrFn(state.openTabs) : tabsOrFn,
    })),
}));
