import { Effect, Exit, Fiber } from 'effect';

/**
 * Executes an Effect program, returning a standard Promise.
 * If the effect fails with a typed error, the promise rejects with that typed error.
 */
export const runPromise = <A, E>(effect: Effect.Effect<A, E>): Promise<A> =>
  Effect.runPromise(effect);

/**
 * Executes an Effect program, returning an Exit structure (Success or Failure).
 * Never throws/rejects, making it ideal for boundary error recovery.
 */
export const runPromiseExit = <A, E>(effect: Effect.Effect<A, E>): Promise<Exit.Exit<A, E>> =>
  Effect.runPromiseExit(effect);

/**
 * Executes a synchronous Effect program directly.
 */
export const runSync = <A, E>(effect: Effect.Effect<A, E>): A =>
  Effect.runSync(effect);

/**
 * Spawns an Effect in a background Fiber with cancellation support.
 */
export const runFork = <A, E>(effect: Effect.Effect<A, E>): Fiber.Fiber<A, E> =>
  Effect.runFork(effect);

/**
 * Safely cancels a running Fiber.
 */
export const interruptFiber = <A, E>(fiber: Fiber.Fiber<A, E>): Promise<Exit.Exit<void, never>> =>
  Effect.runPromiseExit(Fiber.interrupt(fiber));
