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
});
