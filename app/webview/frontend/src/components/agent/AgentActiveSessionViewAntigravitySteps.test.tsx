import React from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
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

let mockActiveSession: any = {
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
};

vi.mock('../../stores/workspaceStore', () => ({
  useWorkspace: () => ({
    activeSession: mockActiveSession,
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

  it('keeps worked accordion closed when task is done, and when opened shows group tools open with individual tools closed', () => {
    const { container } = render(<AgentActiveSessionView />);

    // 1. Worked accordion is closed by default when task is done
    const workedBtn = screen.getAllByRole('button').find(b => b.textContent?.includes('Done in 3 steps'));
    expect(workedBtn).toBeDefined();
    expect(workedBtn?.getAttribute('aria-expanded')).toBe('false');

    // Group tools should not be visible while worked accordion is closed
    expect(screen.queryByText(/Hide 3 steps/i)).toBeNull();

    // 2. Click to open worked accordion
    fireEvent.click(workedBtn!);
    expect(workedBtn?.getAttribute('aria-expanded')).toBe('true');

    // 3. Group tools accordion (FoldedToolTraces) is default open inside
    expect(screen.getByText(/Hide 3 steps/i)).toBeInTheDocument();

    // 4. Tool rows are visible inside the group tools accordion
    expect(screen.getByText(/npm test/i)).toBeInTheDocument();

    // 5. Individual tool calling output bodies are default CLOSED
    expect(screen.queryByText('tests passed 100%')).toBeNull();
    expect(screen.queryByText('content of index.ts')).toBeNull();
  });

  it('does NOT render "Done in ..." while task is currently running', () => {
    mockActiveSession = {
      id: 'sess-agy-running',
      agentId: 'agent-agy',
      status: 'running',
      messages: [
        { id: 'u1', role: 'user', content: 'Do task', timestamp: new Date().toISOString() },
        {
          id: 'a1',
          role: 'agent',
          content: 'Working on it...',
          isThinking: true,
          timestamp: new Date().toISOString(),
          thoughts: [{ id: 'th1', thoughtText: 'Thinking...', durationSeconds: 2 }],
          toolExecutions: [
            { id: 'agy-tool-1', toolName: 'readFile', status: 'running' }
          ]
        }
      ],
      tasks: [],
      diffs: []
    };

    render(<AgentActiveSessionView />);

    // Should NOT show "Done in" while running
    expect(screen.queryByText(/Done in/i)).toBeNull();
    // Action footer badge should NOT be present
    expect(screen.queryByTitle(/Antigravity completed/i)).toBeNull();

    // Worked accordion MUST be OPEN while in progress
    const runningWorkedBtn = screen.getAllByRole('button').find(b => b.textContent?.includes('Working'));
    expect(runningWorkedBtn).toBeDefined();
    expect(runningWorkedBtn?.getAttribute('aria-expanded')).toBe('true');
  });

  it('renders per-task step count for second turn instead of cumulative session step count', () => {
    mockActiveSession = {
      id: 'sess-agy-multiturn',
      agentId: 'agent-agy',
      status: 'completed',
      messages: [
        // Turn 1: 3 steps
        { id: 'u1', role: 'user', content: 'Task 1', timestamp: '5m ago' },
        {
          id: 'a1',
          role: 'agent',
          content: 'Task 1 done',
          timestamp: '4m ago',
          toolExecutions: [
            { id: 'agy-tool-1', toolName: 'read', status: 'completed' },
            { id: 'agy-tool-2', toolName: 'edit', status: 'completed' },
            { id: 'agy-tool-3', toolName: 'test', status: 'completed' }
          ],
          totalSteps: 3
        },
        // Turn 2: only 2 steps (even if global agy tool IDs were 4 and 5)
        { id: 'u2', role: 'user', content: 'Task 2', timestamp: '2m ago' },
        {
          id: 'a2',
          role: 'agent',
          content: 'Task 2 done',
          timestamp: '1m ago',
          toolExecutions: [
            { id: 'agy-tool-4', toolName: 'read', status: 'completed' },
            { id: 'agy-tool-5', toolName: 'edit', status: 'completed' }
          ],
          totalSteps: 2
        }
      ],
      tasks: [],
      diffs: []
    };

    render(<AgentActiveSessionView />);

    // Turn 1 shows "Done in 3 steps"
    expect(screen.getByTitle(/Antigravity completed in 3 step runs/i)).toBeInTheDocument();
    // Turn 2 shows "Done in 2 steps" (NOT cumulative 5 steps!)
    expect(screen.getByTitle(/Antigravity completed in 2 step runs/i)).toBeInTheDocument();
    expect(screen.queryByTitle(/Antigravity completed in 5 step runs/i)).toBeNull();
  });

  it('renders "Done in X steps" by default for other agents (Claude, Codex, Internal) when task is completed', () => {
    mockActiveSession = {
      id: 'sess-claude-1',
      agentId: 'agent-claude',
      status: 'completed',
      messages: [
        { id: 'u1', role: 'user', content: 'Design architecture', timestamp: '3m ago' },
        {
          id: 'a1',
          role: 'agent',
          content: 'Architecture proposed',
          timestamp: '2m ago',
          thoughts: [{ id: 'th1', thoughtText: 'Analyzing dependencies', durationSeconds: 5 }],
          toolExecutions: [
            { id: 'tool-1', toolName: 'readFile', status: 'completed' },
            { id: 'tool-2', toolName: 'writeFile', status: 'completed' }
          ],
          totalSteps: 2
        }
      ],
      tasks: [],
      diffs: []
    };

    render(<AgentActiveSessionView />);

    // Worked accordion shows "Done in 2 steps"
    const workedButtons = screen.getAllByRole('button');
    const workedBtn = workedButtons.find(b => b.textContent?.includes('Done in 2 steps'));
    expect(workedBtn).toBeDefined();

    // Both worked header and footer badge show "Done in 2 steps"
    expect(screen.getAllByText(/Done in 2 steps/i)).toHaveLength(2);
  });
});
