import { create } from 'zustand';
import type { ComposerAttachment } from './composerDraftStore';

export interface PromptStashEntry {
  id: string;
  createdAt: number;
  prompt: string;
  title: string;
  attachments?: ComposerAttachment[];
}

export const MAX_STASH_ENTRIES = 20;
const STORAGE_KEY = 'forge:prompt-stash:v1';

function loadPersistedStash(): PromptStashEntry[] {
  try {
    if (typeof localStorage === 'undefined') return [];
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    if (Array.isArray(parsed)) {
      return parsed as PromptStashEntry[];
    }
  } catch {
    // ignore
  }
  return [];
}

function persistStash(entries: PromptStashEntry[]) {
  try {
    if (typeof localStorage === 'undefined') return;
    localStorage.setItem(STORAGE_KEY, JSON.stringify(entries));
  } catch (e) {
    console.warn('[PROMPT-STASH] Failed to persist prompt stash', e);
  }
}

interface PromptStashStoreState {
  entries: PromptStashEntry[];
  stashPrompt: (prompt: string, attachments?: ComposerAttachment[], customTitle?: string) => PromptStashEntry | null;
  popPrompt: (id?: string) => PromptStashEntry | null;
  removePrompt: (id: string) => void;
  clearStash: () => void;
}

export const usePromptStashStore = create<PromptStashStoreState>((set, get) => ({
  entries: loadPersistedStash(),

  stashPrompt: (prompt: string, attachments?: ComposerAttachment[], customTitle?: string) => {
    const trimmed = prompt.trim();
    if (!trimmed && (!attachments || attachments.length === 0)) {
      return null;
    }

    const title = customTitle || (trimmed ? (trimmed.slice(0, 48) + (trimmed.length > 48 ? '…' : '')) : 'Attached Files');
    const newEntry: PromptStashEntry = {
      id: `stash_${Date.now()}_${Math.random().toString(36).slice(2, 7)}`,
      createdAt: Date.now(),
      prompt,
      title,
      attachments: attachments ? [...attachments] : []
    };

    const nextEntries = [newEntry, ...get().entries].slice(0, MAX_STASH_ENTRIES);
    set({ entries: nextEntries });
    persistStash(nextEntries);
    return newEntry;
  },

  popPrompt: (id?: string) => {
    const entries = get().entries;
    if (entries.length === 0) return null;

    let targetIdx = 0;
    if (id) {
      targetIdx = entries.findIndex(e => e.id === id);
      if (targetIdx === -1) return null;
    }

    const target = entries[targetIdx];
    const nextEntries = entries.filter((_, idx) => idx !== targetIdx);
    set({ entries: nextEntries });
    persistStash(nextEntries);
    return target;
  },

  removePrompt: (id: string) => {
    const nextEntries = get().entries.filter(e => e.id !== id);
    set({ entries: nextEntries });
    persistStash(nextEntries);
  },

  clearStash: () => {
    set({ entries: [] });
    persistStash([]);
  }
}));
