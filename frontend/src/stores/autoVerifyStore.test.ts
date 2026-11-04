import { describe, it, expect, vi, beforeEach } from 'vitest';
import { useAutoVerifyStore } from './autoVerifyStore';
import * as Wails from '../lib/wails';
import * as Toast from '../lib/toast';

vi.mock('../lib/wails', () => ({
  ExecuteCommandSync: vi.fn(),
}));

vi.mock('../lib/toast', () => ({
  showToast: vi.fn(),
}));

describe('autoVerifyStore', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    useAutoVerifyStore.setState({
      enabled: false,
      customCommand: '',
      isVerifying: false,
      lastResult: null,
      retryCounts: {},
    });
  });

  it('toggles enabled state and persists to localStorage', () => {
    const store = useAutoVerifyStore.getState();
    expect(store.enabled).toBe(false);

    store.toggleEnabled();
    expect(useAutoVerifyStore.getState().enabled).toBe(true);
    expect(localStorage.getItem('forge:auto-verify:enabled')).toBe('true');
    expect(Toast.showToast).toHaveBeenCalledWith(
      'Auto-verification enabled. Tests will run after changes.',
      'info'
    );

    useAutoVerifyStore.getState().toggleEnabled();
    expect(useAutoVerifyStore.getState().enabled).toBe(false);
    expect(localStorage.getItem('forge:auto-verify:enabled')).toBe('false');
  });

  it('updates customCommand and persists to localStorage', () => {
    useAutoVerifyStore.getState().setCustomCommand('bun test --run');
    expect(useAutoVerifyStore.getState().customCommand).toBe('bun test --run');
    expect(localStorage.getItem('forge:auto-verify:cmd')).toBe('bun test --run');
  });

  it('detects command via ExecuteCommandSync when customCommand is empty', async () => {
    (Wails.ExecuteCommandSync as any).mockResolvedValueOnce({
      stdout: 'go test ./...',
      stderr: '',
      exitCode: 0,
    });

    const cmd = await useAutoVerifyStore.getState().detectCommand('/dummy/workspace');
    expect(cmd).toBe('go test ./...');
  });

  it('skips verification when disabled', async () => {
    const res = await useAutoVerifyStore.getState().runVerification('session-1', '/dummy');
    expect(res.success).toBe(true);
    expect(Wails.ExecuteCommandSync).not.toHaveBeenCalled();
  });

  it('runs verification and succeeds on exitCode 0', async () => {
    useAutoVerifyStore.setState({ enabled: true, customCommand: 'bun test' });
    (Wails.ExecuteCommandSync as any).mockResolvedValueOnce({
      stdout: 'PASS: 5 tests passed',
      stderr: '',
      exitCode: 0,
    });

    const res = await useAutoVerifyStore.getState().runVerification('session-1', '/dummy');
    expect(res.success).toBe(true);
    expect(Toast.showToast).toHaveBeenCalledWith('✓ Auto-verify passed: bun test', 'success');
    expect(useAutoVerifyStore.getState().lastResult?.exitCode).toBe(0);
  });

  it('triggers onFailure callback on test failure and increments retries', async () => {
    useAutoVerifyStore.setState({ enabled: true, customCommand: 'bun test' });
    (Wails.ExecuteCommandSync as any).mockResolvedValueOnce({
      stdout: '',
      stderr: 'FAIL: 1 test failed: expected true, got false',
      exitCode: 1,
    });

    const onFailure = vi.fn();
    const res = await useAutoVerifyStore.getState().runVerification('session-1', '/dummy', onFailure);
    expect(res.success).toBe(false);
    expect(onFailure).toHaveBeenCalledTimes(1);
    expect(onFailure.mock.calls[0][0]).toContain('⚠️ **Auto-Verification Failed**');
    expect(onFailure.mock.calls[0][0]).toContain('expected true, got false');
    expect(useAutoVerifyStore.getState().retryCounts['session-1']).toBe(1);
  });

  it('halts after 2 retries to prevent infinite loops', async () => {
    useAutoVerifyStore.setState({
      enabled: true,
      customCommand: 'bun test',
      retryCounts: { 'session-1': 2 },
    });

    const onFailure = vi.fn();
    const res = await useAutoVerifyStore.getState().runVerification('session-1', '/dummy', onFailure);
    expect(res.success).toBe(false);
    expect(onFailure).not.toHaveBeenCalled();
    expect(Toast.showToast).toHaveBeenCalledWith(
      'Auto-verification paused: 2 consecutive self-correction attempts reached.',
      'warn'
    );
  });
});
