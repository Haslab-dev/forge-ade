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

  it('recognizes context windows for modern models (agnes, deepseek-v4, luna, glm)', () => {
    const agnesUsage = calculateContextUsage({ id: 's3', model: 'agnes-2-5-flash:free', contextTokens: 146 } as unknown as AgentSession);
    expect(agnesUsage.maxTokens).toBe(512000);

    const lunaUsage = calculateContextUsage({ id: 's4', model: 'gpt-6-luna', contextTokens: 53900 } as unknown as AgentSession);
    expect(lunaUsage.maxTokens).toBe(128000);

    const dsUsage = calculateContextUsage({ id: 's5', model: 'deepseek-v4-1-flash', contextTokens: 11200 } as unknown as AgentSession);
    expect(dsUsage.maxTokens).toBe(1000000);

    const glmUsage = calculateContextUsage({ id: 's6', model: 'glm-5-3-flash', contextTokens: 185 } as unknown as AgentSession);
    expect(glmUsage.maxTokens).toBe(128000);
  });
});
