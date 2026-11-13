import { describe, it, expect, beforeEach, vi } from 'vitest';
import { useTerminalStore } from './terminalStore';
import { CreateShell, StopSession } from '../lib/wails';

vi.mock('../lib/wails', () => ({
  CreateShell: vi.fn(async (title: string, cwd: string) => ({ id: 'sh-123', title, cwd })),
  StopSession: vi.fn(async () => true),
}));

vi.mock('../lib/toast', () => ({
  showToast: vi.fn(),
}));

describe('useTerminalStore (Zustand)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useTerminalStore.setState({
      isBottomTerminalOpen: false,
      activeTerminalId: null,
    });
  });

  it('toggles bottom terminal open status with boolean or updater function', () => {
    useTerminalStore.getState().setIsBottomTerminalOpen(true);
    expect(useTerminalStore.getState().isBottomTerminalOpen).toBe(true);

    useTerminalStore.getState().setIsBottomTerminalOpen(prev => !prev);
    expect(useTerminalStore.getState().isBottomTerminalOpen).toBe(false);
  });

  it('creates shell session and stores active session id', async () => {
    const id = await useTerminalStore.getState().createShellSession('zsh', '/my/project');
    expect(CreateShell).toHaveBeenCalledWith('zsh', '/my/project');
    expect(id).toBe('sh-123');
    expect(useTerminalStore.getState().activeTerminalId).toBe('sh-123');
  });

  it('stops shell session properly', async () => {
    await useTerminalStore.getState().stopShellSession('sh-123');
    expect(StopSession).toHaveBeenCalledWith('sh-123');
  });
});
