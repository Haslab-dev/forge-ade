import { create } from 'zustand';

export interface SteerQueueState {
  queues: Record<string, string[]>;
  getQueue: (sessionId: string) => string[];
  setQueue: (sessionId: string, queue: string[]) => void;
  addToQueue: (sessionId: string, item: string) => void;
  removeFromQueue: (sessionId: string, index: number) => void;
  moveInQueue: (sessionId: string, fromIndex: number, toIndex: number) => void;
  updateQueueItem: (sessionId: string, index: number, text: string) => void;
  clearQueue: (sessionId: string) => void;
}

const STORAGE_KEY = 'forge:steer-queues:v1';

function loadPersistedQueues(): Record<string, string[]> {
  try {
    if (typeof localStorage === 'undefined') return {};
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return {};
    const parsed = JSON.parse(raw);
    if (parsed && typeof parsed === 'object') {
      return parsed as Record<string, string[]>;
    }
  } catch {}
  return {};
}

let saveTimer: ReturnType<typeof setTimeout> | null = null;
function persistQueues(queues: Record<string, string[]>) {
  if (typeof localStorage === 'undefined') return;
  if (saveTimer) clearTimeout(saveTimer);
  saveTimer = setTimeout(() => {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(queues));
    } catch (e) {
      console.warn('[STEER-QUEUE] Failed to persist steer queues to localStorage', e);
    }
  }, 200);
}

export const useSteerQueueStore = create<SteerQueueState>((set, get) => ({
  queues: loadPersistedQueues(),

  getQueue: (sessionId: string) => {
    if (!sessionId) return [];
    return get().queues[sessionId] || [];
  },

  setQueue: (sessionId: string, queue: string[]) => {
    if (!sessionId) return;
    set(state => {
      const next = { ...state.queues, [sessionId]: queue };
      persistQueues(next);
      return { queues: next };
    });
  },

  addToQueue: (sessionId: string, item: string) => {
    const trimmed = item.trim();
    if (!sessionId || !trimmed) return;
    set(state => {
      const current = state.queues[sessionId] || [];
      const next = { ...state.queues, [sessionId]: [...current, trimmed] };
      persistQueues(next);
      return { queues: next };
    });
  },

  removeFromQueue: (sessionId: string, index: number) => {
    if (!sessionId) return;
    set(state => {
      const current = state.queues[sessionId] || [];
      if (index < 0 || index >= current.length) return state;
      const filtered = current.filter((_, i) => i !== index);
      const next = { ...state.queues, [sessionId]: filtered };
      persistQueues(next);
      return { queues: next };
    });
  },

  moveInQueue: (sessionId: string, fromIndex: number, toIndex: number) => {
    if (!sessionId) return;
    set(state => {
      const current = state.queues[sessionId] || [];
      if (
        fromIndex === toIndex ||
        fromIndex < 0 ||
        fromIndex >= current.length ||
        toIndex < 0 ||
        toIndex >= current.length
      ) {
        return state;
      }
      const updated = [...current];
      const [moved] = updated.splice(fromIndex, 1);
      updated.splice(toIndex, 0, moved);
      const next = { ...state.queues, [sessionId]: updated };
      persistQueues(next);
      return { queues: next };
    });
  },

  updateQueueItem: (sessionId: string, index: number, text: string) => {
    const trimmed = text.trim();
    if (!sessionId || !trimmed) return;
    set(state => {
      const current = state.queues[sessionId] || [];
      if (index < 0 || index >= current.length) return state;
      const updated = current.map((item, i) => (i === index ? trimmed : item));
      const next = { ...state.queues, [sessionId]: updated };
      persistQueues(next);
      return { queues: next };
    });
  },

  clearQueue: (sessionId: string) => {
    if (!sessionId) return;
    set(state => {
      if (!state.queues[sessionId]) return state;
      const next = { ...state.queues };
      delete next[sessionId];
      persistQueues(next);
      return { queues: next };
    });
  }
}));
