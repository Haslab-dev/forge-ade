import React, { useState, useEffect, useRef, useCallback } from 'react';
import {
  ArrowLeft,
  ArrowRight,
  RotateCw,
  Globe,
  Loader2,
  MousePointerClick,
  X,
  Plus,
  ExternalLink,
  MonitorSmartphone,
  ChevronDown,
  Check,
  RotateCcw
} from 'lucide-react';
import { cn } from '../../lib/utils';
import {
  BrowserOpenURL,
  BrowserUseCancelPick,
  BrowserUseClickAt,
  BrowserUseEngine,
  BrowserUseHistory,
  BrowserUseNavigate,
  BrowserUsePickElement,
  BrowserUseSetRect,
  BrowserUseSetZoom,
  BrowserUseStart,
  BrowserUseStatus,
  EventsOn
} from '../../lib/wails';
import { getLastOpenedUrl, clearLastOpenedUrl } from '../../lib/browser';

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
  const containerRef = useRef<HTMLDivElement>(null);
  const zoomMenuRef = useRef<HTMLDivElement>(null);

  const [engine, setEngine] = useState<string>('');
  const [urlInput, setUrlInput] = useState('');
  const [url, setUrl] = useState('');
  const [title, setTitle] = useState('');
  const [running, setRunning] = useState(false);
  const [loading, setLoading] = useState(false);
  const [picking, setPicking] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [frame, setFrame] = useState<{ data: string; w: number; h: number } | null>(null);

  // ── Viewport size & zoom state ───────────────────────────────────────────
  const [isFitToWindow, setIsFitToWindow] = useState(true);
  const [customWidth, setCustomWidth] = useState<number | null>(null);
  const [customHeight, setCustomHeight] = useState<number | null>(null);
  const [containerSize, setContainerSize] = useState<{ w: number; h: number }>({ w: 800, h: 600 });
  const [widthInput, setWidthInput] = useState('800');
  const [heightInput, setHeightInput] = useState('600');
  const [zoom, setZoom] = useState(1.0);
  const [zoomMenuOpen, setZoomMenuOpen] = useState(false);
  const [showViewportBar, setShowViewportBar] = useState(true);

  // ── Actions ───────────────────────────────────────────────────────────────
  const guard = useCallback(async (fn: () => Promise<any>) => {
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
  }, []);

  // ── Status + frame subscriptions ──────────────────────────────────────────
  useEffect(() => {
    let mounted = true;
    (async () => {
      const eng = await BrowserUseEngine();
      if (!mounted) return;
      setEngine(eng);
      if (eng === 'in-app') {
        await BrowserUseStart().catch(() => {});
      }

      const pending = getLastOpenedUrl();
      if (pending) {
        clearLastOpenedUrl();
        setUrl(pending);
        setUrlInput(pending);
        void guard(() => BrowserUseNavigate(pending));
        return;
      }

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
  }, [guard]);

  // Listen for open/navigate requests from terminal or elsewhere
  useEffect(() => {
    const handleOpen = (e: any) => {
      const targetUrl = e?.detail?.url;
      if (targetUrl) {
        setUrl(targetUrl);
        setUrlInput(targetUrl);
        setError(null);
        void guard(() => BrowserUseNavigate(targetUrl));
      }
    };
    window.addEventListener('forge:open-sidepane-browser', handleOpen);
    return () => {
      window.removeEventListener('forge:open-sidepane-browser', handleOpen);
    };
  }, [guard]);

  // ── Viewport size & dimension helpers ────────────────────────────────────
  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;
    const ro = new ResizeObserver(entries => {
      for (const entry of entries) {
        const w = Math.round(entry.contentRect.width);
        const h = Math.round(entry.contentRect.height);
        if (w > 0 && h > 0) {
          setContainerSize({ w, h });
          if (isFitToWindow) {
            setWidthInput(String(w));
            setHeightInput(String(h));
          }
        }
      }
      void syncRect(true);
    });
    ro.observe(el);
    return () => ro.disconnect();
  }, [isFitToWindow]);

  useEffect(() => {
    if (!zoomMenuOpen) return;
    const onMouseDown = (e: MouseEvent) => {
      if (zoomMenuRef.current && !zoomMenuRef.current.contains(e.target as Node)) {
        setZoomMenuOpen(false);
      }
    };
    document.addEventListener('mousedown', onMouseDown);
    return () => document.removeEventListener('mousedown', onMouseDown);
  }, [zoomMenuOpen]);

  const applyCustomDimensions = () => {
    const parsedW = parseInt(widthInput, 10);
    const parsedH = parseInt(heightInput, 10);
    if (!isNaN(parsedW) && parsedW > 100 && !isNaN(parsedH) && parsedH > 100) {
      setCustomWidth(parsedW);
      setCustomHeight(parsedH);
      setIsFitToWindow(false);
      setTimeout(() => void syncRect(true), 0);
    } else {
      setWidthInput(String(containerSize.w));
      setHeightInput(String(containerSize.h));
    }
  };

  const handleSwapDimensions = () => {
    const w = customWidth || containerSize.w;
    const h = customHeight || containerSize.h;
    setCustomWidth(h);
    setCustomHeight(w);
    setWidthInput(String(h));
    setHeightInput(String(w));
    setIsFitToWindow(false);
    setTimeout(() => void syncRect(true), 0);
  };

  const handleSelectPreset = (w: number, h: number) => {
    setCustomWidth(w);
    setCustomHeight(h);
    setWidthInput(String(w));
    setHeightInput(String(h));
    setIsFitToWindow(false);
    setZoomMenuOpen(false);
    setTimeout(() => void syncRect(true), 0);
  };

  const handleFitToWindow = () => {
    setIsFitToWindow(true);
    setCustomWidth(null);
    setCustomHeight(null);
    setWidthInput(String(containerSize.w));
    setHeightInput(String(containerSize.h));
    setZoom(1.0);
    void BrowserUseSetZoom(1.0);
    setZoomMenuOpen(false);
    setTimeout(() => void syncRect(true), 0);
  };

  const handleSetZoom = (newZoom: number) => {
    setZoom(newZoom);
    void BrowserUseSetZoom(newZoom);
    setZoomMenuOpen(false);
  };

  // ── Keep the native view positioned over the host div ────────────────────
  const lastRectRef = useRef<{ x: number; y: number; w: number; h: number } | null>(null);
  const syncRect = useCallback(async (visible: boolean) => {
    const el = hostRef.current;
    const container = containerRef.current;
    if (el && container) {
      const r = el.getBoundingClientRect();
      const cr = container.getBoundingClientRect();

      // Strict intersection with the container bounding box so the native
      // browser view NEVER bleeds outside the sidebar canvas into the chat
      // column, editor, header bar, or out of window bounds.
      const left = Math.max(cr.left, r.left);
      const top = Math.max(cr.top, r.top);
      const right = Math.min(cr.right, r.right);
      const bottom = Math.min(cr.bottom, r.bottom);
      const w = Math.max(0, right - left);
      const h = Math.max(0, bottom - top);

      if (w > 20 && h > 20) {
        lastRectRef.current = { x: left, y: top, w, h };
      } else {
        lastRectRef.current = null;
      }
    } else {
      lastRectRef.current = null;
    }

    const lr = lastRectRef.current;
    const shouldBeVisible = Boolean(visible && lr && lr.w > 20 && lr.h > 20 && !zoomMenuOpen);
    try {
      if (shouldBeVisible && lr) {
        await BrowserUseSetRect(lr.x, lr.y, lr.w, lr.h, true);
      } else if (!visible || zoomMenuOpen) {
        await BrowserUseSetRect(0, 0, 0, 0, false);
      }
    } catch {
      // bindings not ready (plain vite dev) — ignore
    }
  }, [zoomMenuOpen]);

  // Hide the native view ONLY when BrowserViewer completely unmounts.
  useEffect(() => {
    return () => {
      void BrowserUseSetRect(0, 0, 0, 0, false);
    };
  }, []);

  useEffect(() => {
    void syncRect(true);
    const el = hostRef.current;
    const container = containerRef.current;
    const ro = new ResizeObserver(() => void syncRect(true));
    if (el) ro.observe(el);
    if (container) ro.observe(container);
    const onWinResize = () => {
      void syncRect(true);
    };
    window.addEventListener('resize', onWinResize);
    return () => {
      ro.disconnect();
      window.removeEventListener('resize', onWinResize);
    };
  }, [syncRect, isFitToWindow, customWidth, customHeight]);

  useEffect(() => {
    void syncRect(true);
  }, [zoomMenuOpen, syncRect]);

  const go = () => {
    let target = urlInput.trim();
    if (!target) return;
    if (!/^https?:\/\//i.test(target) && !target.startsWith('about:') && !target.startsWith('data:')) {
      if (/^(localhost|127\.\d+\.\d+\.\d+|0\.0\.0\.0|\[::1\])/i.test(target) || target.includes(':')) {
        target = 'http://' + target;
      } else if (target.includes('.')) {
        target = 'https://' + target;
      }
    }
    void guard(() => BrowserUseNavigate(target));
  };

  const cancelPick = async () => {
    setPicking(false);
    try {
      await BrowserUseCancelPick();
    } catch {}
  };

  const pick = async () => {
    if (picking) {
      void cancelPick();
      return;
    }
    setPicking(true);
    setError(null);
    try {
      const res = await BrowserUsePickElement();
      const picked = res?.picked || {};
      if (picked.cancelled) return;
      const label = [
        `@browser [element: ${picked.selector || picked.tag || 'unknown'}`,
        picked.text ? ` — "${picked.text}"` : '',
        ']'
      ].join('');
      window.dispatchEvent(new CustomEvent('forge:composer-insert', { detail: label }));
    } catch (e: any) {
      const msg = String(e?.message || e);
      if (!msg.includes('timed out') && !msg.includes('canceled') && !msg.includes('cancelled')) {
        setError(msg);
      }
    } finally {
      setPicking(false);
    }
  };

  useEffect(() => {
    if (!picking) return;
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        void cancelPick();
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [picking]);

  const isCDP = engine === 'cdp';

  return (
    <div className="flex h-full flex-col bg-sidebar">
      {/* Chrome: URL bar + navigation + quick tools */}
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

        {/* Toggle viewport dimension bar */}
        <button
          type="button"
          title="Toggle viewport size controls"
          onClick={() => setShowViewportBar(v => !v)}
          className={cn(
            'flex size-7 items-center justify-center rounded-lg transition-colors',
            showViewportBar
              ? 'bg-surface-hover text-foreground'
              : 'text-foreground-subtle hover:bg-surface-hover hover:text-foreground'
          )}
        >
          <MonitorSmartphone className="size-3.5" />
        </button>

        {/* Open in external browser */}
        <button
          type="button"
          title="Open in default browser"
          disabled={!url}
          onClick={() => {
            if (url) BrowserOpenURL(url).catch(() => {});
          }}
          className="flex size-7 items-center justify-center rounded-lg text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground disabled:pointer-events-none disabled:opacity-30"
        >
          <ExternalLink className="size-3.5" />
        </button>

        {/* Select element or cancel */}
        <button
          type="button"
          title={picking ? 'Cancel element selection (Esc)' : 'Select element for chat'}
          onClick={() => void pick()}
          className={cn(
            'flex size-7 items-center justify-center rounded-lg transition-colors',
            picking
              ? 'bg-accent-primary/20 text-accent-primary ring-1 ring-accent-primary/50'
              : 'text-foreground-subtle hover:bg-surface-hover hover:text-foreground'
          )}
        >
          {picking ? <X className="size-3.5" /> : <MousePointerClick className="size-3.5" />}
        </button>
      </div>

      {/* Subheader: Viewport dimension controls & zoom dropdown */}
      {showViewportBar && (
        <div className="flex shrink-0 items-center justify-center gap-2 border-b border-border bg-sidebar px-3 py-1 text-ui-xs">
          <div className="flex items-center gap-1 font-mono">
            <input
              type="text"
              value={widthInput}
              onChange={e => setWidthInput(e.target.value)}
              onKeyDown={e => {
                if (e.key === 'Enter') applyCustomDimensions();
              }}
              onBlur={applyCustomDimensions}
              title="Width in pixels"
              className="w-12 rounded border border-transparent bg-surface px-1 py-0.5 text-center font-mono text-ui-xs text-foreground transition-colors hover:border-border focus:border-accent-primary focus:bg-surface-hover focus:outline-none"
            />
            <span className="select-none text-foreground-subtlest">×</span>
            <input
              type="text"
              value={heightInput}
              onChange={e => setHeightInput(e.target.value)}
              onKeyDown={e => {
                if (e.key === 'Enter') applyCustomDimensions();
              }}
              onBlur={applyCustomDimensions}
              title="Height in pixels"
              className="w-12 rounded border border-transparent bg-surface px-1 py-0.5 text-center font-mono text-ui-xs text-foreground transition-colors hover:border-border focus:border-accent-primary focus:bg-surface-hover focus:outline-none"
            />
          </div>

          <button
            type="button"
            title="Swap width & height"
            onClick={handleSwapDimensions}
            className="flex size-6 items-center justify-center rounded text-foreground-subtlest transition-colors hover:bg-surface-hover hover:text-foreground"
          >
            <RotateCcw className="size-3" />
          </button>

          {/* Dropdown Menu for Zoom & Presets */}
          <div className="relative" ref={zoomMenuRef}>
            <button
              type="button"
              onClick={() => setZoomMenuOpen(v => !v)}
              className="flex items-center gap-1 rounded border border-border/40 bg-surface px-2 py-0.5 text-ui-xs text-foreground transition-colors hover:bg-surface-hover"
            >
              <span>{isFitToWindow ? 'Fit to window' : `${Math.round(zoom * 100)}%`}</span>
              <ChevronDown className="size-3 text-foreground-subtlest" />
            </button>

            {zoomMenuOpen && (
              // Opens DOWNWARD ("to below not top") like standard menus.
              // While this menu is open, the native view is temporarily hidden
              // so this HTML overlay renders cleanly in front of the browser.
              <div className="absolute right-0 top-full z-50 mt-1 min-w-[150px] rounded-lg border border-border bg-surface p-1 shadow-2xl backdrop-blur-sm">
                <button
                  type="button"
                  onClick={handleFitToWindow}
                  className="flex w-full items-center justify-between rounded px-2 py-1 text-left text-ui-xs text-foreground hover:bg-surface-hover"
                >
                  <span>Fit to window</span>
                  {isFitToWindow && <Check className="size-3 text-accent-primary" />}
                </button>

                <div className="my-1 border-t border-border" />

                {[0.5, 0.75, 1.0, 1.25, 1.5, 2.0].map(z => (
                  <button
                    key={z}
                    type="button"
                    onClick={() => handleSetZoom(z)}
                    className="flex w-full items-center justify-between rounded px-2 py-1 text-left text-ui-xs text-foreground hover:bg-surface-hover"
                  >
                    <span>{Math.round(z * 100)}%</span>
                    {zoom === z && !isFitToWindow && <Check className="size-3 text-accent-primary" />}
                  </button>
                ))}

                <div className="my-1 border-t border-border" />
                <div className="px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wider text-foreground-subtlest">
                  Devices
                </div>

                {[
                  { name: 'Desktop', w: 1280, h: 800 },
                  { name: 'Laptop', w: 1024, h: 768 },
                  { name: 'Tablet', w: 768, h: 1024 },
                  { name: 'Mobile', w: 375, h: 667 },
                  { name: 'Mobile Landscape', w: 667, h: 375 },
                ].map(p => (
                  <button
                    key={p.name}
                    type="button"
                    onClick={() => handleSelectPreset(p.w, p.h)}
                    className="flex w-full items-center justify-between rounded px-2 py-1 text-left text-ui-xs text-foreground hover:bg-surface-hover"
                  >
                    <span>{p.name}</span>
                    <span className="font-mono text-[10px] text-foreground-subtlest">{p.w}×{p.h}</span>
                  </button>
                ))}
              </div>
            )}
          </div>
        </div>
      )}

      {error && (
        <div className="shrink-0 border-b border-border bg-[var(--color-diff-removed)]/10 px-3 py-1.5 text-ui-xs text-foreground-secondary">
          {error}
        </div>
      )}

      {/* Page host canvas (centered preview with dark backdrop) */}
      <div
        ref={containerRef}
        onScroll={() => void syncRect(true)}
        className="relative flex min-h-0 flex-1 overflow-auto bg-[#18181b] p-2"
      >
        <div
          ref={hostRef}
          style={
            isFitToWindow
              ? { width: '100%', height: '100%' }
              : {
                  width: customWidth ? `${customWidth}px` : '100%',
                  height: customHeight ? `${customHeight}px` : '100%'
                }
          }
          className={cn(
            'relative m-auto',
            !isFitToWindow && 'shrink-0 overflow-hidden rounded border border-border/40 shadow-2xl'
          )}
        >
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
            running || url ? (
              <div className="relative h-full w-full bg-white/95 dark:bg-[#1e1f22]">
                {loading && (
                  <div className="absolute inset-0 flex flex-col items-center justify-center gap-2 bg-[#1e1f22]/70 text-foreground-subtle z-10 pointer-events-none">
                    <Loader2 className="size-5 animate-spin text-accent-primary" />
                    <span className="text-ui-xs font-mono">Loading…</span>
                  </div>
                )}
              </div>
            ) : (
              <EmptyState engine={engine} running={running} onLaunch={() => void guard(() => BrowserUseStart())} />
            )
          ) : (
            <EmptyState engine={engine} running={running} onLaunch={() => void guard(() => BrowserUseStart())} />
          )}
        </div>
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
