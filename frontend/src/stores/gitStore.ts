import { create } from 'zustand';
import type { FileDiff, GitFileStatus } from '../types';
import { ApiBridge } from '../services/apiBridge';

export interface GitStoreState {
  gitBranch: string;
  gitFiles: GitFileStatus[];
  gitCommits: any[];
  diffs: FileDiff[];
  activeDiff: FileDiff | null;
  isGitClientOpen: boolean;

  setGitBranch: (branch: string | ((prev: string) => string)) => void;
  setGitFiles: (files: GitFileStatus[] | ((prev: GitFileStatus[]) => GitFileStatus[])) => void;
  setGitCommits: (commits: any[] | ((prev: any[]) => any[])) => void;
  setIsGitClientOpen: (open: boolean | ((prev: boolean) => boolean)) => void;
  setDiffs: (diffs: FileDiff[] | ((prev: FileDiff[]) => FileDiff[])) => void;
  setActiveDiff: (diff: FileDiff | null) => void;
  addDiff: (diff: FileDiff) => void;
  acceptDiff: (diffId: string, onFileWritten?: () => Promise<void>) => Promise<void>;
  rejectDiff: (diffId: string) => void;
  refreshGitStatus: (wsPath: string) => Promise<{ branch: string; files: GitFileStatus[] }>;
  refreshGitLog: (wsPath: string) => Promise<any[]>;
}

export const useGitStore = create<GitStoreState>((set, get) => ({
  gitBranch: '',
  gitFiles: [],
  gitCommits: [],
  diffs: [],
  activeDiff: null,
  isGitClientOpen: false,

  setGitBranch: (gitBranch) =>
    set((state) => ({ gitBranch: typeof gitBranch === 'function' ? gitBranch(state.gitBranch) : gitBranch })),
  setGitFiles: (gitFiles) =>
    set((state) => ({ gitFiles: typeof gitFiles === 'function' ? gitFiles(state.gitFiles) : gitFiles })),
  setGitCommits: (gitCommits) =>
    set((state) => ({ gitCommits: typeof gitCommits === 'function' ? gitCommits(state.gitCommits) : gitCommits })),
  setIsGitClientOpen: (val) =>
    set((state) => ({ isGitClientOpen: typeof val === 'function' ? val(state.isGitClientOpen) : val })),
  setActiveDiff: (activeDiff) => set({ activeDiff }),

  setDiffs: (diffsOrFn) =>
    set((state) => ({
      diffs: typeof diffsOrFn === 'function' ? diffsOrFn(state.diffs) : diffsOrFn,
    })),

  addDiff: (diff: FileDiff) =>
    set((state) => ({
      diffs: [diff, ...state.diffs.filter((d) => d.filePath !== diff.filePath)],
    })),

  acceptDiff: async (diffId: string, onFileWritten?: () => Promise<void>) => {
    const diff = get().diffs.find((d) => d.id === diffId);
    if (!diff || !diff.modifiedContent) return;

    await ApiBridge.writeFile(diff.filePath, diff.modifiedContent);
    if (onFileWritten) {
      await onFileWritten();
    }
    set((state) => ({
      diffs: state.diffs.map((d) => (d.id === diffId ? { ...d, status: 'accepted' as const } : d)),
    }));
  },

  rejectDiff: (diffId: string) =>
    set((state) => ({
      diffs: state.diffs.map((d) => (d.id === diffId ? { ...d, status: 'rejected' as const } : d)),
    })),

  refreshGitStatus: async (wsPath: string) => {
    try {
      const result = await ApiBridge.gitStatus(wsPath);
      const branch = result?.branch || '';
      const files = result?.files || [];
      set({ gitBranch: branch, gitFiles: files });
      return { branch, files };
    } catch (err) {
      console.warn('Failed to refresh git status:', err);
      return { branch: '', files: [] };
    }
  },

  refreshGitLog: async (wsPath: string) => {
    try {
      const commits = await ApiBridge.gitLog(wsPath);
      const safeCommits = Array.isArray(commits) ? commits : [];
      set({ gitCommits: safeCommits });
      return safeCommits;
    } catch (err) {
      console.warn('Failed to refresh git log:', err);
      return [];
    }
  },
}));
