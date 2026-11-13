import { describe, it, expect, vi } from 'vitest';
import { createAgentCallbacks } from './agentCallbacks';
import { AgentSession } from '../../types';

vi.mock('../../services/apiBridge', () => ({
  ApiBridge: {
    saveSessionJsonl: vi.fn()
  }
}));

describe('agentCallbacks', () => {
  it('handles text deltas and turn end properly', () => {
    let session: AgentSession = {
      id: 'sess-1',
      title: 'Session 1',
      goSessionId: 'go-1',
      status: 'running',
      messages: [
        { id: 'm-1', role: 'user', content: 'hello' },
        { id: 'm-2', role: 'agent', content: '', isThinking: true }
      ]
    } as unknown as AgentSession;

    const updateSession = (sessionId: string, fn: (s: AgentSession) => AgentSession) => {
      session = fn(session);
    };

    const callbacks = createAgentCallbacks({
      storeSessionId: 'sess-1',
      goSessionId: 'go-1',
      activeWorkspacePath: '/test/ws',
      addDiff: vi.fn(),
      updateSession
    });

    callbacks.onTextDelta('Hello from agent!', 1);
    callbacks.onTurnEnd();

    expect(session.status).toBe('completed');
    expect(session.goState).toBe('idle');
    expect(session.messages[1].content).toBe('Hello from agent!');
    expect(session.messages[1].isThinking).toBe(false);
  });

  it('marks message as stopped by user when cancelled', () => {
    let session: AgentSession = {
      id: 'sess-2',
      title: 'Session 2',
      goSessionId: 'go-2',
      status: 'running',
      messages: [
        { id: 'm-1', role: 'user', content: 'run' },
        { id: 'm-2', role: 'agent', content: 'Partial output', isThinking: true }
      ]
    } as unknown as AgentSession;

    const updateSession = (sessionId: string, fn: (s: AgentSession) => AgentSession) => {
      session = fn(session);
    };

    const callbacks = createAgentCallbacks({
      storeSessionId: 'sess-2',
      goSessionId: 'go-2',
      activeWorkspacePath: '/test/ws',
      addDiff: vi.fn(),
      updateSession
    });

    callbacks.onTurnEnd('context canceled');

    expect(session.messages[1].content).toContain('*(stopped by user)*');
    expect(session.messages[1].isThinking).toBe(false);
  });

  it('notifies onTurnComplete with hasDiffs flag on completion', () => {
    let session: AgentSession = {
      id: 'sess-3',
      title: 'Session 3',
      goSessionId: 'go-3',
      status: 'running',
      messages: [{ id: 'm-1', role: 'agent', content: '' }]
    } as unknown as AgentSession;

    const onTurnComplete = vi.fn();
    const callbacks = createAgentCallbacks({
      storeSessionId: 'sess-3',
      goSessionId: 'go-3',
      activeWorkspacePath: '/test/ws',
      addDiff: vi.fn(),
      updateSession: (id, fn) => { session = fn(session); },
      onTurnComplete
    });

    callbacks.onDiffCreated({
      filePath: 'test.ts',
      originalContent: 'a',
      newContent: 'b'
    } as any);

    callbacks.onTurnEnd();
    expect(onTurnComplete).toHaveBeenCalledWith('sess-3', true);
  });

  it('preserves existing session model onStateChange and prevents snapshot overwrite', () => {
    let session: AgentSession = {
      id: 'sess-model-preserve',
      title: 'Session Model',
      goSessionId: 'go-model-1',
      model: 'deepseek-chat',
      status: 'running',
      messages: []
    } as unknown as AgentSession;

    const callbacks = createAgentCallbacks({
      storeSessionId: 'sess-model-preserve',
      goSessionId: 'go-model-1',
      activeWorkspacePath: '/test/ws',
      addDiff: vi.fn(),
      updateSession: (id, fn) => { session = fn(session); }
    });

    // Go snapshot comes back with default model from Go backend
    callbacks.onStateChange({
      state: 'idle',
      model: 'claude-3-7-sonnet-20250219'
    });

    // Session model MUST NOT be overwritten with claude-3-7-sonnet-20250219
    expect(session.model).toBe('deepseek-chat');
  });

  it('populates model from snapshot onStateChange when session model is initially empty', () => {
    let session: AgentSession = {
      id: 'sess-model-empty',
      title: 'Session Model Empty',
      goSessionId: 'go-model-2',
      model: '',
      status: 'running',
      messages: []
    } as unknown as AgentSession;

    const callbacks = createAgentCallbacks({
      storeSessionId: 'sess-model-empty',
      goSessionId: 'go-model-2',
      activeWorkspacePath: '/test/ws',
      addDiff: vi.fn(),
      updateSession: (id, fn) => { session = fn(session); }
    });

    callbacks.onStateChange({
      state: 'idle',
      model: 'gpt-4o'
    });

    expect(session.model).toBe('gpt-4o');
  });
});
