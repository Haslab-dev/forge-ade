/**
 * Strongly-typed event subscription adapter for Contract-Driven IPC.
 */

import { EventsOn } from '../../lib/wails';
import type { WailsEventPayloadMap } from './contracts';

export type EventUnsubscribe = () => void;

/**
 * Subscribes to a typed backend Wails event with compile-time payload checking.
 */
export function onWailsEvent<K extends keyof WailsEventPayloadMap>(
  event: K,
  handler: (payload: WailsEventPayloadMap[K]) => void
): EventUnsubscribe {
  return EventsOn(event as string, (data: unknown) => {
    try {
      handler(data as WailsEventPayloadMap[K]);
    } catch (err) {
      console.error(`[IPC] Error handling event "${String(event)}":`, err);
    }
  });
}
