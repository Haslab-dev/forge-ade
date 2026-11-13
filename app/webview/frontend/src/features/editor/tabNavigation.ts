/**
 * Editor Tab Navigation & History Domain Functions
 * Pure, testable logic decoupled from React lifecycle.
 */

import type { EditorTab } from '../../types';

export interface NavHistoryState {
  stack: EditorTab[];
  index: number;
}

export const MAX_NAV_HISTORY = 50;

/**
 * Pushes a tab to the navigation history stack with browser-like forward truncation.
 * Content is stripped to avoid pinning megabytes of buffer text in history memory.
 */
export function pushNavHistory(
  current: NavHistoryState,
  tab: EditorTab,
  maxLimit: number = MAX_NAV_HISTORY
): NavHistoryState {
  // Diff tabs are transient views, not navigable documents
  if (tab.type === 'diff') return current;

  const stack = current.stack.slice(0, current.index + 1);
  const top = stack[stack.length - 1];
  const snapshot: EditorTab = { ...tab, content: '' };

  if (top && top.id === tab.id) {
    stack[stack.length - 1] = snapshot;
    return { stack, index: stack.length - 1 };
  }

  stack.push(snapshot);
  const capped = stack.length > maxLimit ? stack.slice(stack.length - maxLimit) : stack;
  return { stack: capped, index: capped.length - 1 };
}

/**
 * Navigates back in tab history.
 */
export function getNavBack(current: NavHistoryState): { nextNav: NavHistoryState; targetTab: EditorTab } | null {
  if (current.index <= 0) return null;
  const nextIndex = current.index - 1;
  const targetTab = current.stack[nextIndex];
  return {
    nextNav: { ...current, index: nextIndex },
    targetTab,
  };
}

/**
 * Navigates forward in tab history.
 */
export function getNavForward(current: NavHistoryState): { nextNav: NavHistoryState; targetTab: EditorTab } | null {
  if (current.index >= current.stack.length - 1) return null;
  const nextIndex = current.index + 1;
  const targetTab = current.stack[nextIndex];
  return {
    nextNav: { ...current, index: nextIndex },
    targetTab,
  };
}

/**
 * Cycles to the next or previous tab in list order (Ctrl+Tab / Ctrl+Shift+Tab).
 */
export function cycleActiveTab(
  tabs: EditorTab[],
  activeTabId: string | null,
  direction: 1 | -1
): EditorTab | null {
  if (tabs.length < 2) return null;
  const cur = tabs.findIndex(t => t.id === activeTabId);
  const nextIndex = ((cur === -1 ? 0 : cur) + direction + tabs.length) % tabs.length;
  return tabs[nextIndex] || null;
}

/**
 * Determines the next active tab ID when a tab is closed.
 * Neighbour (right, else left) becomes active; returns null if last tab closed.
 */
export function calculateActiveTabOnClose(
  tabs: EditorTab[],
  closingTabId: string,
  currentActiveId: string | null
): string | null {
  if (currentActiveId !== closingTabId) {
    return currentActiveId;
  }
  const idx = tabs.findIndex(t => t.id === closingTabId);
  if (idx === -1) return currentActiveId;

  const nextTabs = tabs.filter(t => t.id !== closingTabId);
  if (nextTabs.length === 0) return null;

  const neighbor = nextTabs[Math.min(idx, nextTabs.length - 1)];
  return neighbor ? neighbor.id : null;
}

/**
 * Filters out closed tab IDs.
 */
export function filterTabsOnClose(tabs: EditorTab[], idsToClose: string[]): EditorTab[] {
  if (idsToClose.length === 0) return tabs;
  const idSet = new Set(idsToClose);
  return tabs.filter(t => !idSet.has(t.id));
}

/**
 * Updates buffer content for a specific tab in the open tabs list.
 */
export function updateTabBuffer(
  tabs: EditorTab[],
  fileIdOrPath: string,
  content: string
): { updatedTabs: EditorTab[]; changed: boolean } {
  let changed = false;
  const updatedTabs = tabs.map(t => {
    if (t.fileId === fileIdOrPath || t.filePath === fileIdOrPath) {
      if (t.isReadOnly || (t.content === content && t.isModified)) return t;
      changed = true;
      return { ...t, content, isModified: true };
    }
    return t;
  });
  return { updatedTabs: changed ? updatedTabs : tabs, changed };
}

/**
 * Updates open tabs when a file or directory is renamed.
 */
export function updateTabsOnRename(
  tabs: EditorTab[],
  oldPath: string,
  newPath: string
): EditorTab[] {
  const newName = newPath.split('/').pop() || newPath;
  return tabs.map(t => {
    if (t.filePath === oldPath) {
      return { ...t, filePath: newPath, fileName: newName };
    }
    if (t.filePath.startsWith(oldPath + '/')) {
      const sub = t.filePath.slice(oldPath.length);
      return { ...t, filePath: `${newPath}${sub}` };
    }
    return t;
  });
}

/**
 * Removes open tabs when a file or directory is deleted.
 */
export function updateTabsOnDelete(
  tabs: EditorTab[],
  deletedPath: string
): EditorTab[] {
  return tabs.filter(
    t => t.filePath !== deletedPath && !t.filePath.startsWith(deletedPath + '/')
  );
}
