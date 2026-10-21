import { create } from 'zustand';
import type { AgentExecutionMode, AgentReasoningLevel } from '../types';

export interface ComposerAttachment {
  id: string;
  name: string;
  path?: string;
  isImage?: boolean;
  previewUrl?: string;
  size?: number;
}

export interface ComposerDraft {
  prompt: string;
  attachedFiles: ComposerAttachment[];
  executionMode?: AgentExecutionMode;
  reasoningLevel?: AgentReasoningLevel;
  updatedAt: number;
}

interface ComposerDraftStoreState {
  drafts: Record<string, ComposerDraft>;
  getDraft: (draftKey: string) => ComposerDraft;
  setDraft: (draftKey: string, patch: Partial<ComposerDraft>) => void;
  clearDraft: (draftKey: string) => void;
}

const STORAGE_KEY = 'forge:composer-drafts:v1';

function loadPersistedDrafts(): Record<string, ComposerDraft> {
  try {
    if (typeof localStorage === 'undefined') return {};
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return {};
    const parsed = JSON.parse(raw);
    if (parsed && typeof parsed === 'object') {
      return parsed as Record<string, ComposerDraft>;
    }
  } catch {
    // ignore parse/storage errors
  }
  return {};
}

let saveTimer: ReturnType<typeof setTimeout> | null = null;
function persistDrafts(drafts: Record<string, ComposerDraft>) {
  if (typeof localStorage === 'undefined') return;
  if (saveTimer) clearTimeout(saveTimer);
  saveTimer = setTimeout(() => {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(drafts));
    } catch (e) {
      console.warn('[COMPOSER-DRAFT] Failed to persist drafts to localStorage', e);
    }
  }, 250);
}

export const useComposerDraftStore = create<ComposerDraftStoreState>((set, get) => ({
  drafts: loadPersistedDrafts(),

  getDraft: (draftKey: string) => {
    const existing = get().drafts[draftKey];
    if (existing) return existing;
    return {
      prompt: '',
      attachedFiles: [],
      updatedAt: Date.now()
    };
  },

  setDraft: (draftKey: string, patch: Partial<ComposerDraft>) => {
    set(state => {
      const current = state.drafts[draftKey] || {
        prompt: '',
        attachedFiles: [],
        updatedAt: Date.now()
      };
      const updated: ComposerDraft = {
        ...current,
        ...patch,
        updatedAt: Date.now()
      };
      const nextDrafts = { ...state.drafts, [draftKey]: updated };
      persistDrafts(nextDrafts);
      return { drafts: nextDrafts };
    });
  },

  clearDraft: (draftKey: string) => {
    set(state => {
      if (!state.drafts[draftKey]) return state;
      const nextDrafts = { ...state.drafts };
      delete nextDrafts[draftKey];
      persistDrafts(nextDrafts);
      return { drafts: nextDrafts };
    });
  }
}));
