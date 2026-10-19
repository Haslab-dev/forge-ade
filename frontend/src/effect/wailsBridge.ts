import { Effect, Schedule } from 'effect';
import { WailsBridgeError } from './errors';

/**
 * Retries transient Wails IPC calls twice.
 */
export const wailsRetryPolicy = Schedule.recurs(2);

/**
 * Wraps a Wails RPC call or promise-returning function into an Effect with typed WailsBridgeError.
 */
export const fromPromiseCall = <T>(
  methodName: string,
  promiseFn: () => Promise<T>,
  fallback?: T
): Effect.Effect<T, WailsBridgeError> =>
  Effect.tryPromise({
    try: async () => {
      try {
        return await promiseFn();
      } catch (err) {
        if (fallback !== undefined && typeof window !== 'undefined' && !(window as unknown as { _wails?: unknown })._wails) {
          return fallback;
        }
        throw err;
      }
    },
    catch: (err) =>
      new WailsBridgeError({
        method: methodName,
        message: err instanceof Error ? err.message : String(err),
        cause: err,
      }),
  });

/**
 * Invokes an exported Wails v3 App method as an Effect program.
 */
export const callEffect = <T>(
  methodName: string,
  fn: ((...args: any[]) => Promise<T>) | undefined,
  fallback: T,
  ...args: any[]
): Effect.Effect<T, WailsBridgeError> => {
  return Effect.tryPromise({
    try: async () => {
      if (typeof fn !== 'function') {
        return fallback;
      }
      return await fn(...args);
    },
    catch: (err) =>
      new WailsBridgeError({
        method: methodName,
        message: err instanceof Error ? err.message : String(err),
        cause: err,
      }),
  });
};
