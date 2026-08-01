import React, { createContext, useCallback, useContext, useMemo, useState } from "react";

type ToastKind = "default" | "info" | "success" | "danger";

type ToastFn = (message: string, kind?: ToastKind) => void;

const ToastContext = createContext<{ toast: ToastFn } | null>(null);

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [queue, setQueue] = useState<Array<{ id: number; message: string; kind: ToastKind }>>([]);

  const toast = useCallback<ToastFn>((message, kind = "default") => {
    const id = Date.now() + Math.random();
    setQueue((prev) => [...prev, { id, message, kind }]);
    window.setTimeout(() => {
      setQueue((prev) => prev.filter((item) => item.id !== id));
    }, 3000);
  }, []);

  const value = useMemo(() => ({ toast }), [toast]);

  return (
    <ToastContext.Provider value={value}>
      {children}
      <div className="fixed bottom-4 right-4 z-[9999] flex max-w-sm flex-col gap-2 pointer-events-none">
        {queue.map((item) => (
          <div
            key={item.id}
            className="pointer-events-auto rounded-lg border border-[var(--border-default)] bg-[var(--bg-elevated)] px-3 py-2 text-xs text-[var(--fg-primary)] shadow-xl backdrop-blur"
          >
            <div className="font-medium capitalize">{item.kind}</div>
            <div className="text-[var(--fg-secondary)]">{item.message}</div>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}

export function useToast() {
  const ctx = useContext(ToastContext);
  if (!ctx) {
    return {
      toast: (() => {}) as ToastFn,
    };
  }
  return ctx;
}
