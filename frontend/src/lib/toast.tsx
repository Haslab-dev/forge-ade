import React, { createContext, useCallback, useContext, useMemo, useState } from "react";

type ToastKind = "info" | "warn" | "error" | "success" | "default" | "danger";

type ToastFn = (message: string, kind?: ToastKind) => void;

const ToastContext = createContext<{ toast: ToastFn } | null>(null);

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [queue, setQueue] = useState<Array<{ id: number; message: string; kind: ToastKind }>>([]);

  const toast = useCallback<ToastFn>((message, kind = "default") => {
    const normalized: ToastKind =
      kind === "danger" ? "danger" :
      kind === "warn" ? "warn" :
      kind === "info" ? "info" :
      kind === "success" ? "success" :
      "default";
    const id = Date.now() + Math.random();
    setQueue((prev) => [...prev, { id, message, kind: normalized }]);
    window.setTimeout(() => {
      setQueue((prev) => prev.filter((item) => item.id !== id));
    }, 3000);
  }, []);

  const value = useMemo(() => ({ toast }), [toast]);

  return (
    <ToastContext.Provider value={value}>
      {children}
      <div className="fixed top-4 left-1/2 -translate-x-1/2 z-[9999] flex max-w-lg flex-col items-center gap-2 pointer-events-none">
        {queue.map((item) => (
          <div
            key={item.id}
            className={`pointer-events-auto min-w-[280px] rounded-lg border px-3 py-2 text-xs text-[var(--fg-primary)] shadow-xl backdrop-blur ${
              item.kind === "success"
                ? "border-emerald-500/40 bg-emerald-500/12"
                : item.kind === "error" || item.kind === "danger"
                  ? "border-rose-500/40 bg-rose-500/12"
                  : item.kind === "warn"
                    ? "border-amber-500/40 bg-amber-500/12"
                    : "border-sky-500/40 bg-sky-500/12"
            }`}
          >
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
