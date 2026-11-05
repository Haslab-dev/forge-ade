import { describe, expect, it, beforeEach } from 'vitest';
import { useWorkspace } from './workspaceStore';
import { useSteerQueueStore } from './steerQueueStore';
import type { AgentSession } from '../types';

describe('Strict Session Configuration & Steer Queue Isolation', () => {
  beforeEach(() => {
    useSteerQueueStore.setState({ queues: {} });
  });

  it('prevents permission mode, reasoning level, and model leakage when switching sessions', () => {
    const store = useWorkspace.getState();

    const sessionA: AgentSession = {
      id: 'sess-iso-a',
      title: 'Session A',
      status: 'idle',
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
      agentId: 'agent-internal',
      model: 'gemini-2.5-pro',
      provider: 'google',
      workspacePath: '/data/workspace/forge-ade',
      messages: [],
      executionMode: 'code',
      reasoningLevel: 'medium'
    };

    const sessionB: AgentSession = {
      id: 'sess-iso-b',
      title: 'Session B',
      status: 'idle',
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
      agentId: 'agent-agy',
      model: 'claude-3-7-sonnet',
      provider: 'custom',
      workspacePath: '/data/workspace/forge-ade',
      messages: [],
      executionMode: 'bypass',
      reasoningLevel: 'high'
    };

    // Legacy or unconfigured session: executionMode and reasoningLevel are undefined
    const sessionC: AgentSession = {
      id: 'sess-iso-c',
      title: 'Session C (Legacy)',
      status: 'idle',
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
      agentId: 'agent-codex',
      model: 'gpt-4o',
      provider: 'openai',
      workspacePath: '/data/workspace/forge-ade',
      messages: []
    };

    store.setSessions([sessionA, sessionB, sessionC]);

    // 1. Switch to Session B (bypass, high)
    store.setActiveSessionId('sess-iso-b');
    let state = useWorkspace.getState();
    expect(state.activeSessionId).toBe('sess-iso-b');
    expect(state.activeAgentId).toBe('agent-agy');
    expect(state.currentModel).toBe('claude-3-7-sonnet');
    expect(state.agentExecutionMode).toBe('bypass');
    expect(state.reasoningLevel).toBe('high');

    // 2. Switch to Session C: MUST NOT inherit bypass or high from Session B!
    // It should guard and fall back to 'code' and 'medium'.
    store.setActiveSessionId('sess-iso-c');
    state = useWorkspace.getState();
    expect(state.activeSessionId).toBe('sess-iso-c');
    expect(state.activeAgentId).toBe('agent-codex');
    expect(state.currentModel).toBe('gpt-4o');
    expect(state.agentExecutionMode).toBe('code');
    expect(state.reasoningLevel).toBe('medium');

    // 3. Switch to Session A: restores Session A configs
    store.setActiveSessionId('sess-iso-a');
    state = useWorkspace.getState();
    expect(state.activeSessionId).toBe('sess-iso-a');
    expect(state.activeAgentId).toBe('agent-internal');
    expect(state.currentModel).toBe('gemini-2.5-pro');
    expect(state.agentExecutionMode).toBe('code');
    expect(state.reasoningLevel).toBe('medium');
  });

  it('mutating config on active session only affects that session and does not corrupt others', () => {
    const store = useWorkspace.getState();

    const session1: AgentSession = {
      id: 'sess-guard-1',
      title: 'Session 1',
      status: 'idle',
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
      agentId: 'agent-internal',
      model: 'gemini-2.5-flash',
      workspacePath: '/data/workspace/forge-ade',
      messages: [],
      executionMode: 'code',
      reasoningLevel: 'medium'
    };

    const session2: AgentSession = {
      id: 'sess-guard-2',
      title: 'Session 2',
      status: 'idle',
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
      agentId: 'agent-agy',
      model: 'claude-3-5-haiku',
      workspacePath: '/data/workspace/forge-ade',
      messages: [],
      executionMode: 'plan',
      reasoningLevel: 'low'
    };

    store.setSessions([session1, session2]);

    // Activate session 2
    store.setActiveSessionId('sess-guard-2');

    // Change model, execution mode, and reasoning level on session 2
    store.setCurrentModel('claude-3-7-sonnet');
    store.setAgentExecutionMode('bypass');
    store.setReasoningLevel('high');

    const curSessions = useWorkspace.getState().sessions;
    const updatedSess2 = curSessions.find(s => s.id === 'sess-guard-2');
    const untouchedSess1 = curSessions.find(s => s.id === 'sess-guard-1');

    // Session 2 is updated
    expect(updatedSess2?.model).toBe('claude-3-7-sonnet');
    expect(updatedSess2?.executionMode).toBe('bypass');
    expect(updatedSess2?.reasoningLevel).toBe('high');

    // Session 1 remains completely untouched!
    expect(untouchedSess1?.model).toBe('gemini-2.5-flash');
    expect(untouchedSess1?.executionMode).toBe('code');
    expect(untouchedSess1?.reasoningLevel).toBe('medium');

    // Switching back to Session 1 cleanly loads Session 1's config
    store.setActiveSessionId('sess-guard-1');
    const state = useWorkspace.getState();
    expect(state.currentModel).toBe('gemini-2.5-flash');
    expect(state.agentExecutionMode).toBe('code');
    expect(state.reasoningLevel).toBe('medium');
  });

  it('isolates steer queue between sessions', () => {
    const steerStore = useSteerQueueStore.getState();

    // Session A queues follow-up instructions
    steerStore.addToQueue('session-steer-a', 'Fix app bar chevron');
    steerStore.addToQueue('session-steer-a', 'Add unit tests');

    expect(steerStore.getQueue('session-steer-a')).toEqual([
      'Fix app bar chevron',
      'Add unit tests'
    ]);

    // Session B queue should initially be empty
    expect(steerStore.getQueue('session-steer-b')).toEqual([]);

    // Queue instruction into Session B
    steerStore.addToQueue('session-steer-b', 'Implement dark mode');
    expect(steerStore.getQueue('session-steer-b')).toEqual(['Implement dark mode']);

    // Session A queue remains untouched
    expect(steerStore.getQueue('session-steer-a')).toEqual([
      'Fix app bar chevron',
      'Add unit tests'
    ]);

    // Reorder and edit in Session A
    steerStore.moveInQueue('session-steer-a', 0, 1);
    expect(steerStore.getQueue('session-steer-a')).toEqual([
      'Add unit tests',
      'Fix app bar chevron'
    ]);

    steerStore.updateQueueItem('session-steer-a', 0, 'Add comprehensive unit tests');
    expect(steerStore.getQueue('session-steer-a')).toEqual([
      'Add comprehensive unit tests',
      'Fix app bar chevron'
    ]);

    // Session B queue still unchanged
    expect(steerStore.getQueue('session-steer-b')).toEqual(['Implement dark mode']);

    // Remove from Session A
    steerStore.removeFromQueue('session-steer-a', 1);
    expect(steerStore.getQueue('session-steer-a')).toEqual(['Add comprehensive unit tests']);
    expect(steerStore.getQueue('session-steer-b')).toEqual(['Implement dark mode']);
  });

  it('hydrates session from savedSessions into sessions on setActiveSessionId and accepts prompts', () => {
    const store = useWorkspace.getState();

    const historicalSession: AgentSession = {
      id: 'sess-historical-1',
      title: 'Historical Session',
      status: 'idle',
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
      agentId: 'agent-agy',
      model: 'gemini-2.5-pro',
      workspacePath: '/data/workspace/forge-ade',
      messages: [
        { id: 'm1', role: 'user', content: 'initial task', timestamp: '1m ago' }
      ]
    };

    // Initialize with session only in savedSessions and not in active sessions
    useWorkspace.setState({
      sessions: [],
      savedSessions: [historicalSession],
      activeSessionId: null
    });

    // Selecting historical session
    useWorkspace.getState().setActiveSessionId('sess-historical-1');

    // Must be hydrated into active sessions array
    const stateAfterSelect = useWorkspace.getState();
    expect(stateAfterSelect.activeSessionId).toBe('sess-historical-1');
    expect(stateAfterSelect.sessions.some(s => s.id === 'sess-historical-1')).toBe(true);

    // Submitting a new prompt to this resumed session
    useWorkspace.getState().sendAgentPrompt('continue next step');

    const stateAfterPrompt = useWorkspace.getState();
    const active = stateAfterPrompt.sessions.find(s => s.id === 'sess-historical-1');
    expect(active).toBeDefined();
    expect(active?.status).toBe('running');
    expect(active?.messages.some(m => m.content === 'continue next step')).toBe(true);
  });
});
