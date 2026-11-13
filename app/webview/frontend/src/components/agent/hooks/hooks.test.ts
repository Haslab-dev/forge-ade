import { describe, it, expect, vi } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useAgentExecution } from './useAgentExecution';
import { useAgentThreadSearch } from './useAgentThreadSearch';

describe('useAgentExecution', () => {
  it('initializes elapsed time to 0 when not working', () => {
    const { result } = renderHook(() => useAgentExecution(false));
    expect(result.current.liveElapsed).toBe(0);
  });
});

describe('useAgentThreadSearch', () => {
  it('opens and closes find bar state', () => {
    const { result } = renderHook(() => useAgentThreadSearch());
    expect(result.current.findOpen).toBe(false);

    act(() => {
      result.current.openFind();
    });
    expect(result.current.findOpen).toBe(true);

    act(() => {
      result.current.closeFind();
    });
    expect(result.current.findOpen).toBe(false);
  });

  it('triggers find bar on meta+F keydown', () => {
    const { result } = renderHook(() => useAgentThreadSearch());
    expect(result.current.findOpen).toBe(false);

    act(() => {
      window.dispatchEvent(new KeyboardEvent('keydown', { key: 'f', metaKey: true }));
    });
    expect(result.current.findOpen).toBe(true);
  });
});
