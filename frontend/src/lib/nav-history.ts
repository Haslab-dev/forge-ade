/**
 * Global navigation history for the workspace (VSCode-style back/forward).
 * Records every activated editor tab (file/shell/agent/browser) and every
 * main-screen switch, so ← / → walks the exact visit order.
 */

export type NavEntry =
  | { kind: "tab"; id: string; path?: string; line?: number }
  | { kind: "screen"; screen: "editor" | "git-graph" | "usage" };

const MAX_HISTORY = 50;

let backStack: NavEntry[] = [];
let forwardStack: NavEntry[] = [];
let current: NavEntry | null = null;
let silent = false;

const listeners = new Set<() => void>();

function sameEntry(a: NavEntry | null, b: NavEntry): boolean {
  if (!a || a.kind !== b.kind) return false;
  if (a.kind === "screen" && b.kind === "screen") return a.screen === b.screen;
  if (a.kind === "tab" && b.kind === "tab") return a.id === b.id;
  return false;
}

function emit(): void {
  listeners.forEach((l) => l());
}

/** Record a navigation target. Ignored while a restore is in flight. */
export function recordNav(entry: NavEntry): void {
  if (silent) return;
  if (sameEntry(current, entry)) return;
  if (current) backStack.push(current);
  forwardStack = [];
  if (backStack.length > MAX_HISTORY) backStack.shift();
  current = entry;
  emit();
}

/** Suppress recording while programmatically restoring an entry. */
export function setNavSilent(value: boolean): void {
  silent = value;
}

export function canGoBack(): boolean {
  return backStack.length > 0;
}

export function canGoForward(): boolean {
  return forwardStack.length > 0;
}

export function goBack(): NavEntry | null {
  const prev = backStack.pop();
  if (!prev) return null;
  if (current) forwardStack.push(current);
  current = prev;
  emit();
  return prev;
}

export function goForward(): NavEntry | null {
  const next = forwardStack.pop();
  if (!next) return null;
  if (current) backStack.push(current);
  current = next;
  emit();
  return next;
}

export function subscribeNav(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}
