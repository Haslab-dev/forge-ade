import React, { useState, useEffect, useRef, useCallback } from 'react';
import { ArrowLeft, ArrowRight, RotateCw, Globe, Loader2, MousePointerClick, X, Plus } from 'lucide-react';
import { cn } from '../../lib/utils';
import {
  BrowserUseClickAt,
  BrowserUseEngine,
  BrowserUseHistory,
  BrowserUseNavigate,
  BrowserUsePickElement,
  BrowserUseSetRect,
  BrowserUseStart,
  BrowserUseStatus,
  EventsOn
} from '../../lib/wails';

/**
 * BrowserViewer — the right-sidebar browser panel.
 *
 * In-app engine (macOS default): the page is a real WKWebView hosted by the
 * Go layer as a native subview of the window; this component renders the
 * chrome (URL bar, back/forward/reload, element picker) and keeps the native
 * view positioned over `hostRef` via BrowserUseSetRect.
 *
 * CDP engine (fallback): the panel renders screencast frames from the
 * headless browser and relays clicks/wheel.
 */

interface StatusEvent {
  state?: string;
  engine?: string;
  url?: string;
  title?: string;
  message?: string;
}

export const BrowserViewer: React.FC = () => {
  const hostRef = useRef<HTMLDivElement>(null);
  const [engine, setEngine] = useState<string>('');
  const [urlInput, setUrlInput] = useState('');
  const [url, setUrl] = useState('');
  const [title, setTitle] = useState('');
  const [running, setRunning] = useState(false);
  const [loading, setLoading] = useState(false);
  const [picking, setPicking] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [frame, setFrame] = useState<{ data: string; w: number; h: number } | null>(null);

  // ── Status + frame subscriptions ──────────────────────────────────────────
  useEffect(() => {
    let mounted = true;
    (async () => {
      const eng = await BrowserUseEngine();
      if (!mounted) return;
      setEngine(eng);
      const st = await BrowserUseStatus();
      if (!mounted) return;
      setRunning(!!st?.running);
      setUrl(st?.url || '');
      setUrlInput(st?.url || '');
      setTitle(st?.title || '');
    })();
    const offStatus = EventsOn('browser:status', (s: StatusEvent) => {
      setRunning(s?.state === 'running' || s?.state === 'navigated' || !!s?.url);
      if (s?.url !== undefined) {
        setUrl(s.url);
        setUrlInput(s.url);
      }
      if (s?.title !== undefined) setTitle(s.title);
      if (s?.state === 'navigated') setLoading(false);
    });
    const offFrame = EventsOn('browser:frame', (f: any) => {
      if (f?.data) setFrame({ data: f.data, w: f.width || 0, h: f.height || 0 });
    });
    return () => {
      mounted = false;
      offStatus();
      offFrame();
    };
  }, []);

  // ── Keep the native view positioned over the host div ────────────────────
  // The last measured rect is cached because React nulls refs before unmount
  // cleanups run — reading hostRef there would early-out and leave the
  // native view floating over the app when the tab closes.
  const lastRectRef = useRef<{ x: number; y: number; w: number; h: number } | null>(null);
  const syncRect = useCallback(async (visible: boolean) => {
    const el = hostRef.current;
    if (el) {
      const r = el.getBoundingClientRect();
      if (r.width > 0 && r.height > 0) {
        lastRectRef.current = { x: r.left, y: r.top, w: r.width, h: r.height };
      }
    }
    const lr = lastRectRef.current;
    if (!lr) return;
    try {
      await BrowserUseSetRect(lr.x, lr.y, lr.w, lr.h, visible && lr.w > 20 && lr.h > 20);
    } catch {
      // bindings not ready (plain vite dev) — ignore
    }
  }, []);

  useEffect(() => {
    syncRect(true);
    const el = hostRef.current;
    const ro = new ResizeObserver(() => syncRect(true));
    if (el) ro.observe(el);
    window.addEventListener('resize', onWinResize);
    function onWinResize() {
      syncRect(true);
    }
    // The ResizeObserver + window resize listener cover layout changes; the
    // old 500ms poll hammered the backend with redundant SetRect IPC.
    return () => {
      ro.disconnect();
      window.removeEventListener('resize', onWinResize);
      syncRect(false); // hide the native view when the panel unmounts
    };
  }, [syncRect]);

  // ── Actions ───────────────────────────────────────────────────────────────
  const guard = async (fn: () => Promise<any>) => {
    setError(null);
    setLoading(true);
    try {
      const res = await fn();
      if (res?.url) {
        setUrl(res.url);
        setUrlInput(res.url);
      }
      if (res?.title) setTitle(res.title);
      if (res?.snapshot_error) setError(String(res.snapshot_error));
    } catch (e: any) {
      setError(String(e?.message || e));
    } finally {
      setLoading(false);
      // Status poll loop also refreshes url/title for the in-app engine.
      BrowserUseStatus().then(st => {
        if (st?.url) {
          setUrl(st.url);
          setUrlInput(st.url);
        }
        if (st?.title) setTitle(st.title);
      }).catch(() => {});
    }
  };

  const go = () => {
    const target = urlInput.trim();
    if (!target) return;
    void guard(() => BrowserUseNavigate(target));
  };

  const pick = async () => {
    if (picking) return;
    setPicking(true);
    setError(null);
    try {
      const res = await BrowserUsePickElement();
      const picked = res?.picked || {};
      const label = [
        `@browser [element: ${picked.selector || picked.tag || 'unknown'}`,
        picked.text ? ` — "${picked.text}"` : '',
        ']'
      ].join('');
      window.dispatchEvent(new CustomEvent('forge:composer-insert', { detail: label }));
    } catch (e: any) {
      const msg = String(e?.message || e);
      if (!msg.includes('timed out')) setError(msg);
    } finally {
      setPicking(false);
    }
  };

  const isCDP = engine === 'cdp';

  return (
    <div className="flex h-full flex-col bg-sidebar">
      {/* Chrome: URL bar + navigation */}
      <div className="flex shrink-0 items-center gap-1 border-b border-border px-2 py-1.5">
        <button
          type="button"
          title="Back"
          onClick={() => void guard(() => BrowserUseHistory('back'))}
          className="flex size-7 items-center justify-center rounded-lg text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground"
        >
          <ArrowLeft className="size-3.5" />
        </button>
        <button
          type="button"
          title="Forward"
          onClick={() => void guard(() => BrowserUseHistory('forward'))}
          className="flex size-7 items-center justify-center rounded-lg text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground"
        >
          <ArrowRight className="size-3.5" />
        </button>
        <button
          type="button"
          title="Reload"
          onClick={() => void guard(() => BrowserUseHistory('reload'))}
          className="flex size-7 items-center justify-center rounded-lg text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground"
        >
          {loading ? <Loader2 className="size-3.5 animate-spin" /> : <RotateCw className="size-3.5" />}
        </button>
        <div className="flex h-7 min-w-0 flex-1 items-center gap-1.5 rounded-lg border border-border bg-surface px-2">
          <Globe className="size-3 shrink-0 text-foreground-subtlest" />
          <input
            value={urlInput}
            onChange={e => setUrlInput(e.target.value)}
            onKeyDown={e => {
              if (e.key === 'Enter') go();
            }}
            placeholder="Enter a URL for the browser…"
            spellCheck={false}
            className="min-w-0 flex-1 bg-transparent text-ui-xs text-foreground outline-none placeholder:text-foreground-subtlest"
          />
        </div>
        <button
          type="button"
          title="Select element for chat"
          onClick={() => void pick()}
          className={cn(
            'flex size-7 items-center justify-center rounded-lg transition-colors',
            picking
              ? 'bg-accent-primary/20 text-accent-primary'
              : 'text-foreground-subtle hover:bg-surface-hover hover:text-foreground'
          )}
        >
          {picking ? <X className="size-3.5" /> : <MousePointerClick className="size-3.5" />}
        </button>
      </div>

      {error && (
        <div className="shrink-0 border-b border-border bg-[var(--color-diff-removed)]/10 px-3 py-1.5 text-ui-xs text-foreground-secondary">
          {error}
        </div>
      )}

      {/* Page host */}
      <div ref={hostRef} className="relative min-h-0 flex-1">
        {isCDP ? (
          frame ? (
            <img
              src={`data:image/jpeg;base64,${frame.data}`}
              alt={title || 'browser page'}
              className="h-full w-full object-contain"
              onClick={e => {
                // Scale display coords to frame pixels for the CDP backend.
                const rect = (e.target as HTMLImageElement).getBoundingClientRect();
                const fx = ((e.clientX - rect.left) / rect.width) * frame.w;
                const fy = ((e.clientY - rect.top) / rect.height) * frame.h;
                void BrowserUseClickAt(fx, fy);
              }}
            />
          ) : (
            <EmptyState engine={engine} running={running} onLaunch={() => void guard(() => BrowserUseStart())} />
          )
        ) : engine === 'in-app' ? (
          // The native WKWebView is layered over this exact rect by the Go
          // layer — nothing to render here. Keep a subtle page backdrop.
          <div className="h-full w-full bg-white/95 dark:bg-[#1e1f22]" />
        ) : (
          <EmptyState engine={engine} running={running} onLaunch={() => void guard(() => BrowserUseStart())} />
        )}
      </div>

      {/* Status line */}
      <div className="flex shrink-0 items-center gap-2 border-t border-border px-3 py-1 text-ui-xs text-foreground-subtlest">
        <span className="truncate">{title || url || (engine === 'in-app' ? 'In-app browser' : engine ? `${engine} browser` : 'Browser')}</span>
        {url && <span className="ml-auto shrink-0 font-mono opacity-70">{new URL(url, 'http://x').host}</span>}
      </div>
    </div>
  );
};

const EmptyState: React.FC<{ engine: string; running: boolean; onLaunch: () => void }> = ({ engine, running, onLaunch }) => (
  <div className="flex h-full flex-col items-center justify-center gap-3 text-center">
    <div className="flex size-11 items-center justify-center rounded-2xl border border-border bg-surface">
      <Globe className="size-5 text-foreground-subtle" />
    </div>
    <div className="max-w-[240px] space-y-1">
      <div className="text-ui-sm font-medium text-foreground">Browser viewer</div>
      <div className="text-ui-xs text-foreground-subtle">
        {engine === 'cdp'
          ? 'The agent drives a headless browser; frames stream here.'
          : 'Open a URL to browse inside ForgeADE — the agent can drive it too.'}
      </div>
    </div>
    {!running && (
      <button
        type="button"
        onClick={onLaunch}
        className="flex items-center gap-1.5 rounded-lg border border-border bg-surface px-3 py-1.5 text-ui-xs text-foreground transition-colors hover:bg-surface-hover"
      >
        <Plus className="size-3" />
        Launch browser
      </button>
    )}
  </div>
);
