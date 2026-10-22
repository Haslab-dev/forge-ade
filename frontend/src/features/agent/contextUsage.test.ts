import { describe, it, expect } from 'vitest';
import { calculateContextUsage } from './contextUsage';
import { AgentSession } from '../../types';

describe('calculateContextUsage', () => {
  it('calculates usage from reported contextTokens fast path', () => {
    const session = {
      id: 's1',
      model: 'claude-3-5-sonnet-20241022',
      contextTokens: 50000,
      messages: []
    } as unknown as AgentSession;

    const usage = calculateContextUsage(session, session.model);
    expect(usage.usedTokens).toBe(50000);
    expect(usage.maxTokens).toBe(200000);
    expect(usage.percent).toBe(25);
    expect(usage.formattedUsed).toBe('50.0K');
    expect(usage.formattedMax).toBe('200K');
  });

  it('estimates usage from message characters when contextTokens is missing', () => {
    const session = {
      id: 's2',
      model: 'gpt-4o',
      messages: [
        { id: 'm1', role: 'user', content: 'a'.repeat(400) },
        { id: 'm2', role: 'agent', content: 'b'.repeat(400) }
      ]
    } as unknown as AgentSession;

    const usage = calculateContextUsage(session, session.model);
    expect(usage.usedTokens).toBe(200); // 800 chars / 4
    expect(usage.maxTokens).toBe(128000);
  });

  it('handles empty session with fallback defaults', () => {
    const usage = calculateContextUsage(undefined, 'custom-model');
    expect(usage.usedTokens).toBe(0);
    expect(usage.maxTokens).toBe(1000000);
    expect(usage.percent).toBe(0);
  });
});
