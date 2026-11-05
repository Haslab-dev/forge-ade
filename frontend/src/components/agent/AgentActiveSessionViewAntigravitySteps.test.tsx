import React from 'react';
import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi, beforeAll } from 'vitest';
import { AgentActiveSessionView } from './AgentActiveSessionView';

beforeAll(() => {
  window.HTMLElement.prototype.scrollTo = vi.fn();
  window.HTMLElement.prototype.scrollIntoView = vi.fn();
  (globalThis as any).ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as any;
});

vi.mock('../../lib/wails', async (importOriginal) => {
  const actual = await importOriginal<Record<string, any>>();
  return {
    ...actual,
    ListBackgroundTasks: vi.fn().mockResolvedValue([]),
    ListAgentSchedules: vi.fn().mockResolvedValue([]),
    CreateSession: vi.fn().mockResolvedValue('term-sess-1'),
    CloseSession: vi.fn().mockResolvedValue(undefined),
    ResizePTY: vi.fn().mockResolvedValue(undefined),
    WriteToPTY: vi.fn().mockResolvedValue(undefined),
    GetHomeDir: vi.fn().mockResolvedValue('/home/user'),
    EventsOn: vi.fn().mockReturnValue(() => {}),
    EventsEmit: vi.fn(),
  };
});

vi.mock('../../lib/audioNotifications', () => ({
  playTaskCompletionSound: vi.fn(),
  playInputNeededSound: vi.fn(),
}));

vi.mock('../../stores/workspaceStore', () => ({
  useWorkspace: () => ({
    activeSession: {
      id: 'sess-agy-1',
      agentId: 'agent-agy',
      status: 'completed',
      messages: [
        {
          id: 'user-1',
          role: 'user',
          content: 'Run refactoring task',
          timestamp: new Date().toISOString()
        },
        {
          id: 'agent-1',
          role: 'agent',
          content: 'Completed refactoring cleanly.',
          timestamp: new Date().toISOString(),
          thoughts: [{ id: 'th-1', thoughtText: 'Planning steps', durationSeconds: 3 }],
          toolExecutions: [
            { id: 'agy-tool-1', toolName: 'readFile', status: 'completed' },
            { id: 'agy-tool-2', toolName: 'editFile', status: 'completed' },
            { id: 'agy-tool-3', toolName: 'runCommand', status: 'completed' }
          ],
          totalSteps: 3
        }
      ],
      tasks: [],
      diffs: []
    },
    activeAgent: { id: 'agent-agy', name: 'Google Antigravity', type: 'acp' },
    agents: [{ id: 'agent-agy', name: 'Google Antigravity', type: 'acp', available: true }],
    providers: [],
    files: [],
    skills: [],
    mcps: [],
    activeWorkspacePath: '/data/workspace/forge-ade',
    activeWorkspaces: [{ path: '/data/workspace/forge-ade', name: 'forge-ade', type: 'local', lastActive: Date.now() }],
    recentWorkspaces: [],
    connectedSSHIds: [],
    gitBranch: 'main',
    activeSessionId: 'sess-agy-1',
    isRightActionDrawerOpen: false,
    likedMsgs: {},
    subagents: [],
    backgroundTasks: [],
    schedules: [],
    diffs: []
  })
}));

describe('AgentActiveSessionView - Antigravity Step Runs', () => {
  it('renders "Done in 3 steps" for Google Antigravity completed tasks in worked header and footer badge', () => {
    render(<AgentActiveSessionView />);

    // Worked accordion text includes step count
    const workedButtons = screen.getAllByRole('button');
    const workedBtn = workedButtons.find(b => b.textContent?.includes('Worked for') || b.textContent?.includes('Done in 3 steps'));
    expect(workedBtn).toBeDefined();
    expect(workedBtn?.textContent).toContain('Done in 3 steps');

    // Action footer badge shows step count
    const stepBadge = screen.getByTitle(/Antigravity completed in 3 step runs/i);
    expect(stepBadge).toBeDefined();
    expect(stepBadge.textContent).toContain('Done in 3 steps');
  });
});
