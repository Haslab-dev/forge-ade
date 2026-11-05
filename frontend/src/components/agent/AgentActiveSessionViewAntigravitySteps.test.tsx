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
            { id: 'agy-tool-1', toolName: 'readFile', command: '{"Path": "src/index.ts"}', output: 'content of index.ts', status: 'completed' },
            { id: 'agy-tool-2', toolName: 'editFile', command: '{"Path": "src/app.ts"}', output: 'edit diff applied', status: 'completed' },
            { id: 'agy-tool-3', toolName: 'runCommand', command: '{"CommandLine": "npm test"}', output: 'tests passed 100%', status: 'completed' }
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

describe('AgentActiveSessionView - Antigravity Step Runs and Accordion Defaults', () => {
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

  it('keeps worked and group tools accordion default open, while individual tools calling are default closed', () => {
    const { container } = render(<AgentActiveSessionView />);

    // 1. Worked accordion is open (shows process items)
    const workedBtn = screen.getAllByRole('button').find(b => b.textContent?.includes('Done in 3 steps'));
    expect(workedBtn).toBeDefined();

    // 2. Group tools accordion (FoldedToolTraces) is default open
    expect(screen.getByText(/Hide 3 steps/i)).toBeInTheDocument();

    // 3. Tool rows are visible inside the group tools accordion
    expect(screen.getByText(/npm test/i)).toBeInTheDocument();

    // 4. Individual tool calling output bodies are default CLOSED
    expect(screen.queryByText('tests passed 100%')).toBeNull();
    expect(screen.queryByText('content of index.ts')).toBeNull();
  });
});
