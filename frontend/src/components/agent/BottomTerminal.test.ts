import { describe, it, expect, beforeEach, vi } from 'vitest';
import { autoResumeSshTerminal } from './BottomTerminal';
import { useTerminalStore } from '../../stores/terminalStore';
import { CreateShell, StopSession } from '../../lib/wails';

vi.mock('../../lib/wails', async (importOriginal) => {
  const actual: any = await importOriginal();
  return {
    ...actual,
    CreateShell: vi.fn(async (title: string, cwd: string) => ({ id: 'new-ssh-session-999', title, cwd })),
    StopSession: vi.fn(async () => true),
    WriteSession: vi.fn(async () => true),
    GetAgentTerminalSessionOutput: vi.fn(async () => ''),
  };
});

vi.mock('../../lib/toast', () => ({
  showToast: vi.fn(),
}));

describe('BottomTerminal SSH continuity', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useTerminalStore.setState({
      isBottomTerminalOpen: false,
      activeTerminalId: null,
    });
  });

  it('restores remote shell terminal and auto-opens dock when SSH reconnects', async () => {
    // Simulate terminal was open before SSH disconnect
    useTerminalStore.getState().setIsBottomTerminalOpen(true);
    window.dispatchEvent(new CustomEvent('forge:ssh-disconnected', { detail: { id: 'conn-1' } }));

    // Now autoResumeSshTerminal is triggered upon reconnect
    const session = await autoResumeSshTerminal('ssh://conn-1/workspace/my-app', true);

    expect(CreateShell).toHaveBeenCalledWith('my-app', 'ssh://conn-1/workspace/my-app');
    expect(session).not.toBeNull();
    expect(session?.id).toBe('new-ssh-session-999');
    expect(useTerminalStore.getState().isBottomTerminalOpen).toBe(true);
    expect(useTerminalStore.getState().activeTerminalId).toBe('new-ssh-session-999');
  });

  it('ignores non-ssh workspace paths', async () => {
    const session = await autoResumeSshTerminal('/local/path/to/project', false);
    expect(session).toBeNull();
    expect(CreateShell).not.toHaveBeenCalled();
  });
});
