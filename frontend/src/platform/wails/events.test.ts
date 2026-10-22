import { describe, it, expect, vi, beforeEach } from 'vitest';
import { onWailsEvent } from './events';
import { executeWailsRpc, WailsRpcError } from './wailsClient';
import { EventsOn } from '../../lib/wails';

vi.mock('../../lib/wails', () => ({
  EventsOn: vi.fn(),
}));

describe('platform/wails IPC', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('onWailsEvent', () => {
    it('registers event handler and catches errors safely', () => {
      let registeredCallback: ((data: unknown) => void) | undefined;
      vi.mocked(EventsOn).mockImplementation((_event, cb) => {
        registeredCallback = cb;
        return vi.fn();
      });

      const handler = vi.fn();
      onWailsEvent('board:updated', handler);

      expect(EventsOn).toHaveBeenCalledWith('board:updated', expect.any(Function));

      // Trigger event
      const payload = { tasks: [], timestamp: 123456 };
      registeredCallback?.(payload);
      expect(handler).toHaveBeenCalledWith(payload);
    });
  });

  describe('executeWailsRpc', () => {
    it('returns success result when RPC succeeds', async () => {
      const res = await executeWailsRpc('testMethod', async () => 'data-ok');
      expect(res).toEqual({ success: true, data: 'data-ok' });
    });

    it('returns error result when RPC fails without swallowing error', async () => {
      const res = await executeWailsRpc('testMethod', async () => {
        throw new Error('connection timeout');
      });
      expect(res.success).toBe(false);
      if (!res.success) {
        expect(res.error).toBeInstanceOf(WailsRpcError);
        expect(res.error.method).toBe('testMethod');
        expect(res.error.message).toContain('connection timeout');
      }
    });
  });
});
