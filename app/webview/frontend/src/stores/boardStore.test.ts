import { describe, it, expect, beforeEach, vi } from 'vitest';
import { useBoardStore } from './boardStore';
import { ApiBridge } from '../services/apiBridge';

vi.mock('../services/apiBridge', () => ({
  ApiBridge: {
    listBoardTasks: vi.fn(),
    saveBoardTask: vi.fn(),
    deleteBoardTask: vi.fn(),
    moveBoardTaskStatus: vi.fn()
  }
}));

vi.mock('../lib/wails', () => ({
  EventsOn: vi.fn(() => () => {})
}));

describe('boardStore', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useBoardStore.setState({ boardTasks: [], isLoading: false });
  });

  it('adds a new board task with incremented key ADE-1', () => {
    const store = useBoardStore.getState();
    const created = store.addBoardTask({
      title: 'Setup database',
      status: 'todo',
      priority: 'high',
      assignee: 'Pandu-Micro'
    });

    expect(created.key).toBe('ADE-1');
    expect(created.title).toBe('Setup database');
    expect(useBoardStore.getState().boardTasks).toHaveLength(1);
    expect(ApiBridge.saveBoardTask).toHaveBeenCalledWith(created, '');
  });

  it('updates an existing task and persists it', () => {
    const store = useBoardStore.getState();
    const created = store.addBoardTask({
      title: 'Build UI',
      status: 'todo',
      priority: 'medium'
    });

    store.updateBoardTask(created.id, { title: 'Build UI Component' });
    const current = useBoardStore.getState().boardTasks.find(t => t.id === created.id);
    expect(current?.title).toBe('Build UI Component');
  });

  it('deletes a task by id or key', () => {
    const store = useBoardStore.getState();
    const created = store.addBoardTask({
      title: 'Remove me',
      status: 'todo',
      priority: 'medium'
    });

    expect(useBoardStore.getState().boardTasks).toHaveLength(1);
    store.deleteBoardTask(created.id);
    expect(useBoardStore.getState().boardTasks).toHaveLength(0);
    expect(ApiBridge.deleteBoardTask).toHaveBeenCalledWith(created.id, '');
  });

  it('moves task status and updates timestamp', () => {
    const store = useBoardStore.getState();
    const created = store.addBoardTask({
      title: 'Move me',
      status: 'todo',
      priority: 'medium'
    });

    store.moveBoardTaskStatus(created.id, 'done');
    const updated = useBoardStore.getState().boardTasks.find(t => t.id === created.id);
    expect(updated?.status).toBe('done');
    expect(updated?.completedAt).toBeDefined();
    expect(ApiBridge.moveBoardTaskStatus).toHaveBeenCalledWith(created.id, 'done', '');
  });
});
