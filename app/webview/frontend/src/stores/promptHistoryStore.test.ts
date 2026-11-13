import { describe, it, expect, beforeEach } from 'vitest';
import { usePromptHistoryStore, MAX_PROMPT_HISTORY } from './promptHistoryStore';

describe('usePromptHistoryStore', () => {
  beforeEach(() => {
    usePromptHistoryStore.getState().clearHistory();
  });

  it('adds unique prompts to history', () => {
    const store = usePromptHistoryStore.getState();
    store.addHistory('npm test');
    store.addHistory('git status');

    expect(usePromptHistoryStore.getState().history).toEqual(['npm test', 'git status']);
  });

  it('ignores empty or whitespace prompts', () => {
    const store = usePromptHistoryStore.getState();
    store.addHistory('   ');
    expect(usePromptHistoryStore.getState().history).toHaveLength(0);
  });

  it('avoids adjacent duplicate prompts', () => {
    const store = usePromptHistoryStore.getState();
    store.addHistory('npm test');
    store.addHistory('npm test');

    expect(usePromptHistoryStore.getState().history).toEqual(['npm test']);
  });

  it('moves existing prompt to newest position when re-submitted', () => {
    const store = usePromptHistoryStore.getState();
    store.addHistory('first');
    store.addHistory('second');
    store.addHistory('first');

    expect(usePromptHistoryStore.getState().history).toEqual(['second', 'first']);
  });

  it('caps history at MAX_PROMPT_HISTORY', () => {
    const store = usePromptHistoryStore.getState();
    for (let i = 0; i < MAX_PROMPT_HISTORY + 15; i++) {
      store.addHistory(`prompt ${i}`);
    }

    const history = usePromptHistoryStore.getState().history;
    expect(history).toHaveLength(MAX_PROMPT_HISTORY);
    expect(history[history.length - 1]).toBe(`prompt ${MAX_PROMPT_HISTORY + 14}`);
  });
});
