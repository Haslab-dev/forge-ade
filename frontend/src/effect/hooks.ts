import { useState, useEffect, useRef, useCallback } from 'react';
import { Effect, Fiber, Exit, Cause } from 'effect';

export interface UseRunEffectResult<A, E> {
  data: A | null;
  error: E | null;
  loading: boolean;
  execute: (overrideEffect?: Effect.Effect<A, E>) => Promise<Exit.Exit<A, E>>;
  abort: () => void;
}

/**
 * React 19 hook to run an Effect-TS program with automatic lifecycle & fiber cancellation.
 * If the component unmounts while the Effect is in-flight, the fiber is automatically interrupted.
 */
export function useRunEffect<A, E>(
  effectOrFactory: Effect.Effect<A, E> | (() => Effect.Effect<A, E>),
  deps: unknown[] = [],
  autoRun = false
): UseRunEffectResult<A, E> {
  const [data, setData] = useState<A | null>(null);
  const [error, setError] = useState<E | null>(null);
  const [loading, setLoading] = useState<boolean>(autoRun);
  const fiberRef = useRef<Fiber.Fiber<A, E> | null>(null);
  const factoryRef = useRef(effectOrFactory);
  factoryRef.current = effectOrFactory;

  const abort = useCallback(() => {
    if (fiberRef.current) {
      Effect.runFork(Fiber.interrupt(fiberRef.current));
      fiberRef.current = null;
      setLoading(false);
    }
  }, []);

  const execute = useCallback(async (overrideEffect?: Effect.Effect<A, E>): Promise<Exit.Exit<A, E>> => {
    abort();
    setLoading(true);
    setError(null);

    const program = overrideEffect ?? (typeof factoryRef.current === 'function' ? factoryRef.current() : factoryRef.current);

    return new Promise<Exit.Exit<A, E>>((resolve) => {
      const fiber = Effect.runFork(
        Effect.onExit(program, (exit) => {
          if (exit._tag === 'Success') {
            setData(exit.value);
            setError(null);
          } else {
            const errResult = Cause.findError(exit.cause);
            const err = errResult._tag === 'Success' ? ((errResult as any).success ?? (errResult as any).value) : exit.cause;
            setError(err as E);
          }
          setLoading(false);
          fiberRef.current = null;
          resolve(exit);
          return Effect.void;
        })
      );
      fiberRef.current = fiber;
    });
  }, [abort]);

  useEffect(() => {
    if (autoRun) {
      execute();
    }
    return () => {
      abort();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps);

  return {
    data,
    error,
    loading,
    execute,
    abort,
  };
}

/**
 * Runs an Effect once or when dependencies change, handling cleanup automatically.
 */
export function useAsyncEffect<A, E>(
  effect: Effect.Effect<A, E>,
  onSuccess?: (data: A) => void,
  onError?: (err: E) => void,
  deps: unknown[] = []
): void {
  useEffect(() => {
    const fiber = Effect.runFork(
      Effect.onExit(effect, (exit) => {
        if (exit._tag === 'Success') {
          onSuccess?.(exit.value);
        } else {
          const errResult = Cause.findError(exit.cause);
          const err = errResult._tag === 'Success' ? ((errResult as any).success ?? (errResult as any).value) : exit.cause;
          onError?.(err as E);
        }
        return Effect.void;
      })
    );

    return () => {
      Effect.runFork(Fiber.interrupt(fiber));
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps);
}
