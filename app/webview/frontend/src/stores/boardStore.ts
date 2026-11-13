import { create } from 'zustand';
import { BoardTask, BoardTaskStatus } from '../types';
import { ApiBridge } from '../services/apiBridge';
import { EventsOn } from '../lib/wails';

export interface BoardState {
  boardTasks: BoardTask[];
  isLoading: boolean;
  setBoardTasks: (tasks: BoardTask[] | ((prev: BoardTask[]) => BoardTask[])) => void;
  refreshBoardTasks: (wsFolder?: string) => Promise<void>;
  addBoardTask: (task: Omit<BoardTask, 'id' | 'key' | 'createdAt' | 'updatedAt'>, wsFolder?: string) => BoardTask;
  updateBoardTask: (id: string, patch: Partial<BoardTask>, wsFolder?: string) => void;
  deleteBoardTask: (id: string, wsFolder?: string) => void;
  moveBoardTaskStatus: (id: string, status: BoardTaskStatus, wsFolder?: string) => void;
  initBoardEventListener: (getWs: () => string) => () => void;
}

export const useBoardStore = create<BoardState>((set, get) => ({
  boardTasks: [],
  isLoading: false,

  setBoardTasks: (tasksOrFn) => {
    set(state => ({
      boardTasks: typeof tasksOrFn === 'function' ? tasksOrFn(state.boardTasks) : tasksOrFn
    }));
  },

  refreshBoardTasks: async (wsFolder?: string) => {
    const ws = wsFolder || '';
    set({ isLoading: true });
    try {
      const backendTasks = await ApiBridge.listBoardTasks(ws);
      if (Array.isArray(backendTasks)) {
        set({ boardTasks: backendTasks, isLoading: false });
        return;
      }
    } catch {
      // Fallback to localStorage if in browser environment or backend fails
      try {
        const key = 'forge_ade_board_tasks_' + (ws || 'default');
        const legacyKey = 'forge_ade_jira_tasks_' + (ws || 'default');
        const saved =
          localStorage.getItem(key) ||
          localStorage.getItem('forge_ade_board_tasks_global') ||
          localStorage.getItem(legacyKey) ||
          localStorage.getItem('forge_ade_jira_tasks_global');
        if (saved) {
          const parsed = JSON.parse(saved);
          if (Array.isArray(parsed)) {
            const mockIds = new Set(['task-1', 'task-2', 'task-3', 'task-4', 'task-5']);
            set({ boardTasks: parsed.filter(t => !mockIds.has(t.id)), isLoading: false });
            return;
          }
        }
      } catch {}
    }
    set({ boardTasks: [], isLoading: false });
  },

  addBoardTask: (input, wsFolder?: string) => {
    const tasks = get().boardTasks;
    const id = 'task-' + Date.now();
    const maxNum = tasks.reduce((max, t) => {
      const match = t.key?.match(/^ADE-(\d+)$/);
      if (match) {
        const num = parseInt(match[1], 10);
        return Math.max(max, isNaN(num) ? 0 : num);
      }
      return max;
    }, 0);
    const key = `ADE-${maxNum + 1}`;
    const newTask: BoardTask = {
      ...input,
      id,
      key,
      createdAt: Date.now(),
      updatedAt: Date.now()
    };

    set({ boardTasks: [newTask, ...tasks] });
    const ws = wsFolder || '';
    void ApiBridge.saveBoardTask(newTask, ws);
    return newTask;
  },

  updateBoardTask: (id: string, patch: Partial<BoardTask>, wsFolder?: string) => {
    const updated = get().boardTasks.map(t =>
      t.id === id ? { ...t, ...patch, updatedAt: Date.now() } : t
    );
    set({ boardTasks: updated });

    const target = updated.find(t => t.id === id);
    if (target) {
      const ws = wsFolder || '';
      void ApiBridge.saveBoardTask(target, ws);
    }
  },

  deleteBoardTask: (id: string, wsFolder?: string) => {
    set(state => ({
      boardTasks: state.boardTasks.filter(t => t.id !== id && t.key !== id)
    }));
    const ws = wsFolder || '';
    void ApiBridge.deleteBoardTask(id, ws);
  },

  moveBoardTaskStatus: (id: string, status: BoardTaskStatus, wsFolder?: string) => {
    const now = Date.now();
    const updated = get().boardTasks.map(t =>
      t.id === id
        ? {
            ...t,
            status,
            updatedAt: now,
            completedAt: status === 'done' ? now : undefined
          }
        : t
    );
    set({ boardTasks: updated });

    const ws = wsFolder || '';
    void ApiBridge.moveBoardTaskStatus(id, status, ws);
  },

  initBoardEventListener: (getWs: () => string) => {
    const unsubBoard = EventsOn('board:updated', (tasks: any) => {
      if (Array.isArray(tasks)) {
        set({ boardTasks: tasks });
      } else {
        void get().refreshBoardTasks(getWs());
      }
    });

    const unsubJira = EventsOn('jira:updated', (tasks: any) => {
      if (Array.isArray(tasks)) {
        set({ boardTasks: tasks });
      } else {
        void get().refreshBoardTasks(getWs());
      }
    });

    return () => {
      if (typeof unsubBoard === 'function') unsubBoard();
      if (typeof unsubJira === 'function') unsubJira();
    };
  }
}));
