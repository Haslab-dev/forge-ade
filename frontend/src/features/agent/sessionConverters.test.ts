import { describe, it, expect } from 'vitest';
import { convertGoSessionToAgentSession } from './sessionConverters';

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
