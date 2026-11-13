import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { useSteerQueueStore } from '../../stores/steerQueueStore';
import { processNextSteerQueueItem, registerSteerQueueHandlers } from './steerQueueRunner';

describe('steerQueueRunner - processNextSteerQueueItem', () => {
  const mockSendAgentPrompt = vi.fn();
  let mockSessions: any[] = [];

  beforeEach(() => {
    vi.useFakeTimers();
    useSteerQueueStore.setState({ queues: {} });
    mockSendAgentPrompt.mockClear();

    mockSessions = [
      {
        id: 'sess-1',
        title: 'Session 1',
        status: 'idle',
        messages: []
      },
      {
        id: 'sess-running',
        title: 'Running Session',
        status: 'running',
        messages: []
      }
    ];

    registerSteerQueueHandlers({
      sendPrompt: mockSendAgentPrompt,
      findSession: (id) => mockSessions.find(s => s.id === id)
    });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('does nothing and returns false when queue is empty', () => {
    const result = processNextSteerQueueItem('sess-1');
    expect(result).toBe(false);
    expect(mockSendAgentPrompt).not.toHaveBeenCalled();
  });

  it('does not steer while the agent is still running (waits until agent stops)', () => {
    useSteerQueueStore.getState().addToQueue('sess-running', 'queued follow-up');

    const result = processNextSteerQueueItem('sess-running');
    expect(result).toBe(false);
    expect(mockSendAgentPrompt).not.toHaveBeenCalled();
    // Item stays in queue
    expect(useSteerQueueStore.getState().getQueue('sess-running')).toEqual(['queued follow-up']);
  });

  it('automatically pops queue and steers immediately when agent is stopped / completed', () => {
    useSteerQueueStore.getState().addToQueue('sess-1', 'run test suite');
    useSteerQueueStore.getState().addToQueue('sess-1', 'commit code');

    const result = processNextSteerQueueItem('sess-1');
    expect(result).toBe(true);

    // Head was popped immediately
    expect(useSteerQueueStore.getState().getQueue('sess-1')).toEqual(['commit code']);

    // Advance timer to trigger execution
    vi.advanceTimersByTime(100);

    expect(mockSendAgentPrompt).toHaveBeenCalledWith('run test suite', 'sess-1');
  });

  it('prevents concurrent duplicate processing for the same session', () => {
    useSteerQueueStore.getState().addToQueue('sess-1', 'first task');
    useSteerQueueStore.getState().addToQueue('sess-1', 'second task');

    const firstCall = processNextSteerQueueItem('sess-1');
    expect(firstCall).toBe(true);

    // Second immediate call while first is in-flight
    const secondCall = processNextSteerQueueItem('sess-1');
    expect(secondCall).toBe(false);

    // Only first item was popped
    expect(useSteerQueueStore.getState().getQueue('sess-1')).toEqual(['second task']);

    vi.advanceTimersByTime(100);
    expect(mockSendAgentPrompt).toHaveBeenCalledTimes(1);
    expect(mockSendAgentPrompt).toHaveBeenCalledWith('first task', 'sess-1');
  });

  it('dispatches the queued prompt to its own session rather than an active session', () => {
    mockSessions = [
      {
        id: 'sess-project-a',
        title: 'Project A Task',
        status: 'idle',
        workspacePath: '/path/to/project-a',
        messages: []
      },
      {
        id: 'sess-project-b',
        title: 'Project B Task',
        status: 'idle',
        workspacePath: '/path/to/project-b',
        messages: []
      }
    ];

    useSteerQueueStore.getState().addToQueue('sess-project-a', 'Compile Project A');

    // Process queued item for Project A while user is viewing Project B
    const dispatched = processNextSteerQueueItem('sess-project-a');
    expect(dispatched).toBe(true);

    vi.advanceTimersByTime(100);

    // MUST be dispatched with sess-project-a, NOT sess-project-b!
    expect(mockSendAgentPrompt).toHaveBeenCalledWith('Compile Project A', 'sess-project-a');
  });
});
