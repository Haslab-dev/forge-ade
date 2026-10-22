/**
 * Canonical Wails IPC Client with strict error semantics.
 */

import { IpcResult } from './contracts';

export class WailsRpcError extends Error {
  public readonly method: string;
  public readonly causeDetails?: unknown;

  constructor(method: string, message: string, causeDetails?: unknown) {
    super(`[WailsRPC:${method}] ${message}`);
    this.name = 'WailsRpcError';
    this.method = method;
    this.causeDetails = causeDetails;
  }
}

/**
 * Executes a backend RPC method ensuring errors are preserved and typed.
 */
export async function executeWailsRpc<T>(
  methodName: string,
  fn: () => Promise<T>
): Promise<IpcResult<T, WailsRpcError>> {
  try {
    const result = await fn();
    return { success: true, data: result };
  } catch (err: unknown) {
    const message = err instanceof Error ? err.message : String(err);
    const rpcError = new WailsRpcError(methodName, message, err);
    console.error(rpcError.message, err);
    return { success: false, error: rpcError };
  }
}
