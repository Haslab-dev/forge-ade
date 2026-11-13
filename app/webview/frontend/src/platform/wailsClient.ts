import { Events as RuntimeEvents, Clipboard as RuntimeClipboard } from '@wailsio/runtime';
import * as App from '../../bindings/github.com/hasdev/forge-ade/internal/app/app';
import { WailsBridgeError } from '../effect/errors';

/**
 * Canonical Wails client boundary.
 * All IPC calls to Go backend methods and events should pass through this boundary.
 */

export const isDesktopEnvironment = (): boolean => {
  return typeof window !== 'undefined' && typeof (window as any)?._wails !== 'undefined';
};

export const getWailsBindings = (): any => {
  return isDesktopEnvironment() ? App : {};
};

/**
 * Invokes an App RPC method. Throws WailsBridgeError when the call fails or bindings
 * are unavailable. Does not swallow errors or substitute fake success.
 */
export async function invokeWails<T = any>(methodName: string, ...args: any[]): Promise<T> {
  const bindings = getWailsBindings();
  const fn = bindings[methodName];
  if (typeof fn !== 'function') {
    throw new WailsBridgeError({
      method: methodName,
      message: `Method '${methodName}' is not available in the current environment`,
    });
  }

  try {
    return await fn(...args);
  } catch (err: unknown) {
    throw new WailsBridgeError({
      method: methodName,
      message: err instanceof Error ? err.message : String(err),
      cause: err,
    });
  }
}

/**
 * Invokes an App RPC method with an explicit fallback when running in non-desktop environments.
 * If the method is invoked in desktop environment and fails, it throws instead of hiding the error.
 */
export async function invokeWailsWithFallback<T>(methodName: string, fallback: T, ...args: any[]): Promise<T> {
  if (!isDesktopEnvironment()) {
    return fallback;
  }
  return invokeWails<T>(methodName, ...args);
}

/**
 * Subscribes to a Wails runtime event and returns a cleanup function to unsubscribe.
 */
export function onWailsEvent<T = any>(eventName: string, callback: (data: T) => void): () => void {
  return RuntimeEvents.On(eventName, (ev: any) => {
    callback(ev?.data !== undefined ? ev.data : ev);
  });
}

/**
 * Native clipboard operations
 */
export const WailsClipboard = {
  getText: async (): Promise<string> => {
    try {
      return await RuntimeClipboard.Text();
    } catch {
      return '';
    }
  },
  setText: async (text: string): Promise<void> => {
    try {
      await RuntimeClipboard.SetText(text);
    } catch {
      // no-op
    }
  },
};
