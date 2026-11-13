import { describe, expect, it } from 'vitest';
import { useWorkspace } from './workspaceStore';
import { convertGoSessionToAgentSession } from '../features/agent/sessionConverters';
import type { AgentSession } from '../types';

describe('Session Provider & Model Stickiness', () => {
  it('convertGoSessionToAgentSession sets agentId to agent-internal, not session UUID', () => {
    const snap = {
      id: 'sess-uuid-12345',
      name: 'Go Task',
      model: 'gemini-2.5-pro',
      messages: []
    };
    const converted = convertGoSessionToAgentSession(snap);
    expect(converted.agentId).toBe('agent-internal');
    expect(converted.goSessionId).toBe('sess-uuid-12345');
    expect(converted.model).toBe('gemini-2.5-pro');
  });

  it('restores agentId, model, executionMode, and reasoningLevel when switching sessions', () => {
    const store = useWorkspace.getState();

    const sessionA: AgentSession = {
      id: 'session-a',
      title: 'Session A (ForgeADE)',
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
      id: 'session-b',
      title: 'Session B (Antigravity ACP)',
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

    store.setSessions([sessionA, sessionB]);

    // Select Session B
    store.setActiveSessionId('session-b');
    const stateB = useWorkspace.getState();
    expect(stateB.activeSessionId).toBe('session-b');
    expect(stateB.activeAgentId).toBe('agent-agy');
    expect(stateB.currentModel).toBe('claude-3-7-sonnet');
    expect(stateB.agentExecutionMode).toBe('bypass');
    expect(stateB.reasoningLevel).toBe('high');

    // Switch back to Session A: should stick to Session A's properties!
    store.setActiveSessionId('session-a');
    const stateA = useWorkspace.getState();
    expect(stateA.activeSessionId).toBe('session-a');
    expect(stateA.activeAgentId).toBe('agent-internal');
    expect(stateA.currentModel).toBe('gemini-2.5-pro');
    expect(stateA.agentExecutionMode).toBe('code');
    expect(stateA.reasoningLevel).toBe('medium');
  });

  it('updating agentId or model while on a session updates that specific session', () => {
    const store = useWorkspace.getState();
    const sessionC: AgentSession = {
      id: 'session-c',
      title: 'Session C',
      status: 'idle',
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
      agentId: 'agent-internal',
      model: 'gemini-2.5-flash',
      workspacePath: '/data/workspace/forge-ade',
      messages: []
    };

    store.setSessions([sessionC]);
    store.setActiveSessionId('session-c');

    // Change model on session C
    store.setCurrentModel('gpt-4o');
    expect(useWorkspace.getState().currentModel).toBe('gpt-4o');
    const updatedSess = useWorkspace.getState().sessions.find(s => s.id === 'session-c');
    expect(updatedSess?.model).toBe('gpt-4o');
  });

  it('handoffSession records a role: handoff message with provider details', () => {
    const store = useWorkspace.getState();
    const sessionH: AgentSession = {
      id: 'session-handoff-test',
      title: 'Handoff Test',
      status: 'idle',
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
      agentId: 'agent-internal',
      model: 'gemini-2.5-flash',
      workspacePath: '/data/workspace/forge-ade',
      messages: [
        {
          id: 'msg-1',
          role: 'user',
          content: 'Hello agent',
          timestamp: new Date().toISOString()
        }
      ]
    };

    store.setSessions([sessionH]);
    store.setActiveSessionId('session-handoff-test');

    store.handoffSession('agent-agy', 'Switching to Antigravity ACP engine');

    const updatedSession = useWorkspace.getState().sessions.find(s => s.id === 'session-handoff-test');
    expect(updatedSession).toBeDefined();
    expect(updatedSession?.agentId).toBe('agent-agy');
    expect(updatedSession?.messages.length).toBe(2);

    const handoffMsg = updatedSession?.messages[1];
    expect(handoffMsg?.role).toBe('handoff');
    expect(handoffMsg?.handoff?.fromProvider).toBe('agent-internal');
    expect(handoffMsg?.handoff?.toProvider).toBe('agent-agy');
    expect(handoffMsg?.handoff?.summary).toContain('Antigravity ACP');
  });

  it('forkSession creates a new session containing a fork handoff notice and cloned messages', () => {
    const store = useWorkspace.getState();
    const origSession: AgentSession = {
      id: 'session-orig',
      title: 'Original Task',
      status: 'idle',
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
      agentId: 'agent-internal',
      model: 'gemini-2.5-flash',
      workspacePath: '/data/workspace/forge-ade',
      messages: [
        {
          id: 'msg-orig-1',
          role: 'user',
          content: 'Task instruction',
          timestamp: new Date().toISOString()
        }
      ]
    };

    store.setSessions([origSession]);
    const forkedId = store.forkSession('session-orig', 'Forked Task');

    expect(forkedId).toBeTruthy();
    const forkedSession = useWorkspace.getState().sessions.find(s => s.id === forkedId);
    expect(forkedSession).toBeDefined();
    expect(forkedSession?.title).toBe('Forked Task');
    expect(forkedSession?.messages.length).toBe(2);
    expect(forkedSession?.messages[0].content).toBe('Task instruction');
    expect(forkedSession?.messages[1].role).toBe('handoff');
    expect(forkedSession?.messages[1].handoff?.summary).toContain('Forked conversation');
  });

  it('createNewSession with agent-internal respects selected currentModel and does not revert', () => {
    const store = useWorkspace.getState();
    store.setCurrentModel('deepseek-chat');
    expect(useWorkspace.getState().currentModel).toBe('deepseek-chat');

    const newId = store.createNewSession('Fix internal agent model bug', 'agent-internal');
    const created = useWorkspace.getState().sessions.find(s => s.id === newId);
    expect(created?.model).toBe('deepseek-chat');
    expect(useWorkspace.getState().currentModel).toBe('deepseek-chat');
  });
});
