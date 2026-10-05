import type { TerminalSettings } from '../types';

/**
 * Terminal rendering settings shared by every terminal surface (session
 * terminals, editor tabs, bottom dock). Backed by the Go app settings but
 * mirrored into localStorage so terminals read them synchronously at
 * construction time; a window event notifies live surfaces of changes.
 */

const LS_KEY = 'forge_ade_terminal_settings';
export const TERMINAL_SETTINGS_EVENT = 'forge:terminal-settings-changed';

export const DEFAULT_TERMINAL_SETTINGS: TerminalSettings = {
  shell: '',
  // Mirrors the app's --font-mono token (style system) so terminal and UI
  // mono text share one stack.
  fontFamily: "'JetBrains Mono', ui-monospace, SFMono-Regular, Menlo, monospace",
  fontSize: 13,
  cursorStyle: 'block',
  cursorBlink: true,
  scrollback: 5000
};

let cache: TerminalSettings | null = null;

export function getTerminalSettings(): TerminalSettings {
  if (cache) return cache;
  try {
    const saved = localStorage.getItem(LS_KEY);
    if (saved) {
      const parsed = JSON.parse(saved);
      const merged: TerminalSettings = { ...DEFAULT_TERMINAL_SETTINGS, ...parsed };
      cache = merged;
      return merged;
    }
  } catch {
    // ignore
  }
  const defaults: TerminalSettings = { ...DEFAULT_TERMINAL_SETTINGS };
  cache = defaults;
  return defaults;
}

/** Mirror settings into localStorage and notify mounted terminals. */
export function setTerminalSettings(settings: TerminalSettings): void {
  cache = { ...DEFAULT_TERMINAL_SETTINGS, ...settings };
  try {
    localStorage.setItem(LS_KEY, JSON.stringify(cache));
  } catch {
    // ignore
  }
  window.dispatchEvent(new CustomEvent(TERMINAL_SETTINGS_EVENT, { detail: cache }));
}
