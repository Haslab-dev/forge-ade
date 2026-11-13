import { create } from 'zustand';

export const MAX_PROMPT_HISTORY = 100;
const STORAGE_KEY = 'forge:prompt-history:v1';

function loadPersistedHistory(): string[] {
  try {
    if (typeof localStorage === 'undefined') return [];
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    if (Array.isArray(parsed)) {
      return parsed.filter((item): item is string => typeof item === 'string' && item.trim().length > 0);
    }
  } catch {
    // ignore
  }
  return [];
}

function persistHistory(history: string[]) {
  try {
    if (typeof localStorage === 'undefined') return;
    localStorage.setItem(STORAGE_KEY, JSON.stringify(history));
  } catch (e) {
    console.warn('[PROMPT-HISTORY] Failed to persist prompt history', e);
  }
}

interface PromptHistoryStoreState {
  history: string[]; // Oldest to newest: [oldest, ..., newest]
  addHistory: (prompt: string) => void;
  clearHistory: () => void;
}

export const usePromptHistoryStore = create<PromptHistoryStoreState>((set, get) => ({
  history: loadPersistedHistory(),

  addHistory: (prompt: string) => {
    const trimmed = prompt.trim();
    if (!trimmed) return;

    const current = get().history;
    // Avoid adjacent duplicate
    if (current.length > 0 && current[current.length - 1] === trimmed) {
      return;
    }

    // Filter out previous exact duplicate if any and append to end (most recent)
    const filtered = current.filter(item => item !== trimmed);
    const next = [...filtered, trimmed].slice(-MAX_PROMPT_HISTORY);

    set({ history: next });
    persistHistory(next);
  },

  clearHistory: () => {
    set({ history: [] });
    persistHistory([]);
  }
}));
