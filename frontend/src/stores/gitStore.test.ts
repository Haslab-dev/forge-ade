import { describe, it, expect, beforeEach, vi } from 'vitest';
import { useGitStore } from './gitStore';
import { ApiBridge } from '../services/apiBridge';

vi.mock('../services/apiBridge', () => ({
  ApiBridge: {
    gitStatus: vi.fn(async () => ({ branch: 'main', files: [{ path: 'a.ts', status: 'M' }] })),
    gitLog: vi.fn(async () => [{ hash: '1234567', message: 'init' }]),
    writeFile: vi.fn(async () => true),
  },
}));

describe('useGitStore (Zustand)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useGitStore.setState({
      gitBranch: '',
      gitFiles: [],
      gitCommits: [],
      diffs: [],
      activeDiff: null,
      isGitClientOpen: false,
    });
  });

  it('refreshes git status and updates store state', async () => {
    const res = await useGitStore.getState().refreshGitStatus('/workspace');
    expect(res.branch).toBe('main');
    expect(res.files.length).toBe(1);
    expect(useGitStore.getState().gitBranch).toBe('main');
    expect(useGitStore.getState().gitFiles[0].path).toBe('a.ts');
  });

  it('adds, accepts and rejects diffs correctly', async () => {
    const diff = {
      id: 'd1',
      filePath: '/workspace/test.ts',
      fileName: 'test.ts',
      originalContent: 'old',
      modifiedContent: 'new',
      status: 'pending' as const,
      additions: 1,
      deletions: 0,
      timestamp: '123',
    };

    useGitStore.getState().addDiff(diff);
    expect(useGitStore.getState().diffs.length).toBe(1);

    const onWritten = vi.fn(async () => {});
    await useGitStore.getState().acceptDiff('d1', onWritten);
    expect(ApiBridge.writeFile).toHaveBeenCalledWith('/workspace/test.ts', 'new');
    expect(onWritten).toHaveBeenCalled();
    expect(useGitStore.getState().diffs[0].status).toBe('accepted');

    useGitStore.getState().rejectDiff('d1');
    expect(useGitStore.getState().diffs[0].status).toBe('rejected');
  });
});
