import { describe, it, expect, vi } from 'vitest';
import { runSideConversationPrompt } from './sideConversationRunner';
import { AgentSession } from '../../types';

describe('sideConversationRunner', () => {
  it('streams chunks and thoughts into messages', async () => {
    let messages: any[] = [];
    const updateMessages = (updater: any) => {
      messages = updater(messages);
    };

    const mockEngine = {
      streamSideChat: vi.fn(async (_model, _history, _ctx, handlers) => {
        handlers.onThought({ id: 'th-1', thoughtText: 'Planning...', turn: 1, seq: 1 });
        handlers.onChunk('Hello ');
        handlers.onChunk('world!');
      })
    };

    const session = {
      id: 'sess-1',
      title: 'Test Session',
      sideConversationMessages: []
    } as unknown as AgentSession;

    await runSideConversationPrompt({
      promptText: 'Analyze diff',
      model: 'gpt-4o',
      mode: 'full',
      activeSession: session,
      activeWorkspacePath: '/test/ws',
      diffs: [],
      gitFiles: [],
      gitBranch: 'main',
      engine: mockEngine as any,
      updateMessages
    });

    expect(messages).toHaveLength(2);
    expect(messages[0].role).toBe('user');
    expect(messages[0].content).toBe('Analyze diff');
    expect(messages[1].role).toBe('agent');
    expect(messages[1].content).toBe('Hello world!');
    expect(messages[1].thoughts).toHaveLength(1);
    expect(messages[1].thoughts[0].thoughtText).toBe('Planning...');
  });

  it('handles stream errors gracefully without throwing', async () => {
    let messages: any[] = [];
    const updateMessages = (updater: any) => {
      messages = updater(messages);
    };

    const mockEngine = {
      streamSideChat: vi.fn(async () => {
        throw new Error('LLM rate limit reached');
      })
    };

    const session = {
      id: 'sess-2',
      title: 'Err Session',
      sideConversationMessages: []
    } as unknown as AgentSession;

    await runSideConversationPrompt({
      promptText: 'Prompt causing error',
      model: 'gpt-4o',
      activeSession: session,
      activeWorkspacePath: '/test/ws',
      diffs: [],
      gitFiles: [],
      gitBranch: 'main',
      engine: mockEngine as any,
      updateMessages
    });

    expect(messages).toHaveLength(2);
    expect(messages[1].content).toContain('⚠️ Error: LLM rate limit reached');
    expect(messages[1].isThinking).toBe(false);
  });
});
