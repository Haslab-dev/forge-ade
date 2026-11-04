import { describe, expect, it, beforeEach } from 'vitest';
import { useWorkspace } from './workspaceStore';

describe('Editor Agent Tooling (Forge Internal Agent routing & model config)', () => {
  beforeEach(() => {
    localStorage.clear();
    useWorkspace.setState({
      activeAgentId: 'agent-agy',
      currentModel: 'claude-3-7-sonnet-20250219',
      editorToolingModel: '',
      mode: 'editor',
      sessions: []
    });
  });

  it('updates editorToolingModel and persists to localStorage', () => {
    const store = useWorkspace.getState();
    expect(store.editorToolingModel).toBe('');

    store.setEditorToolingModel('qwen2.5-coder:latest');
    expect(useWorkspace.getState().editorToolingModel).toBe('qwen2.5-coder:latest');
    expect(localStorage.getItem('forge_ade_editor_tooling_model')).toBe('qwen2.5-coder:latest');

    // Setting empty string clears it
    store.setEditorToolingModel('');
    expect(useWorkspace.getState().editorToolingModel).toBe('');
    expect(localStorage.getItem('forge_ade_editor_tooling_model')).toBeNull();
  });

  it('launchEditorAgentTooling always routes to internal agent and sets mode to agent', () => {
    const store = useWorkspace.getState();
    expect(store.activeAgentId).toBe('agent-agy');
    expect(store.mode).toBe('editor');

    const sessionId = store.launchEditorAgentTooling('ask', 'How does this function work?');

    const updated = useWorkspace.getState();
    // Switched mode to agent
    expect(updated.mode).toBe('agent');
    // Active agent is routed to agent-internal
    expect(updated.activeAgentId).toBe('agent-internal');

    // Check created session
    const createdSession = updated.sessions.find((s) => s.id === sessionId);
    expect(createdSession).toBeDefined();
    expect(createdSession?.agentId).toBe('agent-internal');
    expect(createdSession?.messages[0]?.content).toBe('How does this function work?');
  });

  it('uses editorToolingModel when configured, or customModel override', () => {
    const store = useWorkspace.getState();
    store.setEditorToolingModel('gemini-2.0-flash');

    // Default tooling launch picks editorToolingModel
    const session1Id = store.launchEditorAgentTooling('debug', 'Debug panic stacktrace');
    const session1 = useWorkspace.getState().sessions.find((s) => s.id === session1Id);
    expect(session1?.model).toBe('gemini-2.0-flash');
    expect(session1?.agentId).toBe('agent-internal');

    // Custom model override takes precedence
    const session2Id = store.launchEditorAgentTooling('fix', 'Fix lint error', 'deepseek-r1');
    const session2 = useWorkspace.getState().sessions.find((s) => s.id === session2Id);
    expect(session2?.model).toBe('deepseek-r1');
    expect(session2?.agentId).toBe('agent-internal');
  });
});
