// Pure tab-list transitions for the right side pane (AgentRightSidebar).
// Extracted so the lifecycle rules — singleton tabs, file-tab mirroring,
// close-activation, title numbering — are unit-testable without React.

export type PaneTabKind = 'terminal' | 'chat' | 'file' | 'browser' | 'subagent' | 'deck' | 'preview' | 'diffs';

export interface PaneTab {
  id: string;
  kind: PaneTabKind;
  title: string;
  sessionId?: string;
  path?: string;
  taskId?: string;
  content?: string;
  url?: string;
}

/** Source rows mirrored from the workspace store's `sideFileTabs`. */
export interface SideFileRow {
  path: string;
  name: string;
}

/**
 * Append `tab` unless a tab with the same id already exists. Used for the
 * singleton kinds (review, gitdiff, browser, subagent:<id>, sideChat).
 * Returns the input array unchanged when present so React can bail out.
 */
export function upsertSingleton(tabs: PaneTab[], tab: PaneTab): PaneTab[] {
  if (tabs.some(t => t.id === tab.id)) return tabs;
  return [...tabs, tab];
}

/**
 * Reconcile file tabs (opened from the left explorer) with the store's
 * sideFileTabs: non-file tabs keep their order, file tabs are rebuilt
 * wholesale and ordered last. Returns the input array when
 * nothing changed so the mirror effect can bail out of the state update.
 */
export function mirrorFileTabs(prev: PaneTab[], files: SideFileRow[]): PaneTab[] {
  const fileTabs: PaneTab[] = files.map(t => ({
    id: `file:${t.path}`,
    kind: 'file',
    title: t.name,
    path: t.path
  }));
  const nonFile = prev.filter(t => t.kind !== 'file');
  if (
    prev.length === fileTabs.length + nonFile.length &&
    fileTabs.every((t, i) => prev[nonFile.length + i]?.id === t.id)
  ) {
    return prev;
  }
  return [...nonFile, ...fileTabs];
}

/**
 * Selection after closing `closedId`: closing a background tab keeps the
 * current selection untouched; closing the active tab selects the last
 * remaining tab, or nothing when the list empties (the pane returns to its
 * chooser empty state).
 */
export function nextActiveAfterClose(
  remaining: PaneTab[],
  closedId: string,
  activeTab: string | null
): { activeTab: string | null; tabOpen: boolean } {
  if (activeTab !== closedId) return { activeTab, tabOpen: activeTab !== null };
  if (remaining.length > 0) return { activeTab: remaining[remaining.length - 1].id, tabOpen: true };
  return { activeTab: null, tabOpen: false };
}

/**
 * Title for a new terminal tab: `Task: <first word>` when launched for a
 * command, otherwise `zsh N` numbered among existing terminal tabs only.
 */
export function nextTerminalTitle(tabs: PaneTab[], command?: string): string {
  const shortCmd = command ? command.trim().split(/\s+/)[0] : '';
  if (shortCmd) return `Task: ${shortCmd}`;
  return `zsh ${tabs.filter(t => t.kind === 'terminal').length + 1}`;
}

/** Whether the built-in side-chat singleton is already open. */
export function hasBuiltinSideChat(tabs: PaneTab[]): boolean {
  return tabs.some(t => t.id === 'sideChat');
}

/** Title for an additional conversation tab (numbered among chat tabs). */
export function nextConversationTitle(tabs: PaneTab[]): string {
  return `Conversation ${tabs.filter(t => t.kind === 'chat').length + 1}`;
}

/**
 * Find an existing terminal tab to focus instead of spawning a duplicate:
 * by `taskId` when given (one pane per task), else any terminal tab when
 * `focusExisting` (the header/chooser reuse rule). Returns undefined when a
 * new terminal should be created.
 */
export function findTerminalToFocus(
  tabs: PaneTab[],
  opts: { taskId?: string; focusExisting: boolean }
): PaneTab | undefined {
  if (opts.taskId) return tabs.find(t => t.taskId === opts.taskId);
  if (opts.focusExisting) return tabs.find(t => t.kind === 'terminal');
  return undefined;
}
