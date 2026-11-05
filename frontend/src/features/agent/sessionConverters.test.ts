import { describe, it, expect } from 'vitest';
import { convertGoSessionToAgentSession, normalizeLoadedSession } from './sessionConverters';

describe('convertGoSessionToAgentSession', () => {
  it('clamps cacheHitRate between 0 and 1', () => {
    // Over 1.0 (e.g. 5.68 from uncached input division) must be clamped to 1
    const overSess = convertGoSessionToAgentSession({
      id: 's1',
      cacheHitRate: 5.68,
      messages: []
    });
    expect(overSess.cacheHitRate).toBe(1);

    // Negative rate must be clamped to 0
    const negSess = convertGoSessionToAgentSession({
      id: 's2',
      cacheHitRate: -0.2,
      messages: []
    });
    expect(negSess.cacheHitRate).toBe(0);

    // Normal valid rate preserved
    const normalSess = convertGoSessionToAgentSession({
      id: 's3',
      cacheHitRate: 0.75,
      messages: []
    });
    expect(normalSess.cacheHitRate).toBe(0.75);
  });
});

describe('normalizeLoadedSession', () => {
  it('resets ephemeral acpSessionId and strips invalid internal session/acp prefixes from remoteSessionId', () => {
    const raw = {
      id: 'session-12345',
      title: 'Previous Session',
      status: 'running',
      acpSessionId: 'acp-99999',
      remoteSessionId: 'session-12345',
      messages: [
        { role: 'user', content: 'hello' },
        { role: 'agent', content: 'thinking...', isThinking: true }
      ]
    };

    const normalized = normalizeLoadedSession(raw);
    expect(normalized).not.toBeNull();
    // ephemeral acpSessionId must be cleared so next prompt cleanly starts/resumes
    expect(normalized?.acpSessionId).toBeUndefined();
    // internal prefix session- in remoteSessionId must be cleared so CLI does not receive invalid session ID
    expect(normalized?.remoteSessionId).toBeUndefined();
    // running status and isThinking flags are sanitized
    expect(normalized?.status).toBe('idle');
    expect(normalized?.messages[1].isThinking).toBe(false);
  });

  it('preserves valid remote agent UUIDs', () => {
    const raw = {
      id: 'session-abcdef',
      remoteSessionId: '16fa2567-5d52-479f-a0f3-f5bd2d53b7c6',
      messages: []
    };

    const normalized = normalizeLoadedSession(raw);
    expect(normalized?.remoteSessionId).toBe('16fa2567-5d52-479f-a0f3-f5bd2d53b7c6');
  });
});
