import React, { useState, useRef, useCallback, useEffect } from "react";
import {
  IconArrowLeft,
  IconArrowRight,
  IconRefresh,
  IconExternalLink,
  IconWorld,
  IconX,
  IconSearch,
  IconPlugConnected,
  IconFileCode,
  IconHome,
} from "@tabler/icons-react";
import { cn } from "../lib/utils";
import { BrowserOpenURL } from "../lib/wails";

// No default URL — the panel starts on a welcome screen; navigation happens
// when the user types a URL, searches, or clicks a link from the terminal.
const DEFAULT_URL = "";

// If the user types something that isn't a URL (no scheme, no dot-host),
// treat it as a search query.
function toUrl(input: string): string {
  const s = input.trim();
  if (!s) return "";
  if (/^https?:\/\//i.test(s)) return s;
  // Looks like a domain: google.com, localhost:8080, 127.0.0.1:3000, api.site/path
  if (/^[\w-]+(\.[\w-]+)+([/?#][^\s]*)?$/.test(s) || /^localhost(:\d+)?(\/.*)?$/i.test(s) || /^\d{1,3}(\.\d{1,3}){3}(:\d+)?(\/.*)?$/.test(s)) {
    return "https://" + s;
  }
  // Otherwise search
  return "https://www.google.com/search?q=" + encodeURIComponent(s);
}

interface HistoryEntry {
  url: string;
}

// Global "open a URL in the internal browser panel" registry. The terminal
// (or anything else) can call openInBrowser(url) to navigate the browser
// panel even when it isn't the active screen. App.tsx registers the real
// handler (switch screen + navigate) on mount.
type BrowserOpenHandler = (url: string) => void;
let openBrowserHandler: BrowserOpenHandler | null = null;

// The panel registers its live navigate() here so the global handler can
// drive navigation from outside.
let navigateRef: ((raw: string) => void) | null = null;

// When a link is clicked before the panel has mounted (App switches screen,
// React renders, THEN the panel registers navigateRef), hold the URL here so
// the panel can consume it on mount instead of falling back to the external
// browser.
let pendingUrl: string | null = null;

export function setOnOpenInBrowser(cb: BrowserOpenHandler | null) {
  openBrowserHandler = cb;
}

// The panel registers its live navigate() so App.tsx's global handler can
// drive navigation from outside.
export function setNavigateRef(cb: ((raw: string) => void) | null) {
  navigateRef = cb;
  // If a URL was queued while the panel was unmounted, deliver it now.
  if (cb && pendingUrl) {
    const url = pendingUrl;
    pendingUrl = null;
    cb(url);
  }
}

// navigateBrowser drives the live panel directly (no recursion) — used by the
// App-level handler after switching to the browser screen.
export function navigateBrowser(raw: string) {
  if (navigateRef) {
    navigateRef(raw);
  } else {
    // Panel not mounted yet — queue it; setNavigateRef delivers on mount.
    pendingUrl = raw;
  }
}

export function openInBrowser(url: string) {
  if (openBrowserHandler) {
    openBrowserHandler(url);
  } else if (navigateRef) {
    navigateRef(url);
  } else {
    // Fallback: open in the system browser if the panel isn't wired up.
    BrowserOpenURL(url).catch(() => {});
  }
}

export function BrowserPanel({ initialUrl }: { initialUrl?: string }) {
  const [urlInput, setUrlInput] = useState("");
  const [currentUrl, setCurrentUrl] = useState("");
  const [history, setHistory] = useState<HistoryEntry[]>([]);
  const [historyIndex, setHistoryIndex] = useState(-1);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [useExternal, setUseExternal] = useState(false);
  const iframeKeyRef = useRef(0);

  // Navigate to the tab's initial URL once it mounts (workspace browser tabs).
  useEffect(() => {
    if (initialUrl) {
      setUrlInput(initialUrl);
      setCurrentUrl(initialUrl);
      setError(null);
      setHistory([{ url: initialUrl }]);
      setHistoryIndex(0);
      iframeKeyRef.current += 1;
      setLoading(true);
      startBlockTimer();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Blocked-embed detection: WebKit fires iframe onLoad even for pages the
  // server refused via X-Frame-Options / CSP — it renders an empty doc. So we
  // can't trust onLoad; instead start a timer each navigation and if no
  // "real" load arrives within the window, show a blocked overlay.
  const [blocked, setBlocked] = useState(false);
  const blockTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    return () => {
      if (blockTimerRef.current) clearTimeout(blockTimerRef.current);
    };
  }, []);

  const startBlockTimer = useCallback(() => {
    if (blockTimerRef.current) clearTimeout(blockTimerRef.current);
    setBlocked(false);
    blockTimerRef.current = setTimeout(() => {
      // Still loading after the window → likely blocked (blank frame).
      setLoading(false);
      setBlocked(true);
    }, 1800);
  }, []);

  const handleFrameLoad = useCallback(() => {
    setLoading(false);
    setBlocked(false);
    if (blockTimerRef.current) clearTimeout(blockTimerRef.current);
  }, []);

  const navigate = useCallback((raw: string) => {
    const url = toUrl(raw);
    setUrlInput(url);
    setCurrentUrl(url);
    setError(null);
    // Forward nav clears the redo tail; push current onto history.
    setHistory((prev) => {
      const base = prev.slice(0, historyIndex + 1);
      return [...base, { url }];
    });
    setHistoryIndex((idx) => idx + 1);
    iframeKeyRef.current += 1; // force iframe reload even if url unchanged
    setLoading(true);
    startBlockTimer();
  }, [historyIndex, startBlockTimer]);
  // Register live navigate so openInBrowser() can drive this panel from
  // outside (e.g. clicking a link in the terminal).
  useEffect(() => {
    setNavigateRef(navigate);
    return () => setNavigateRef(null);
  }, [navigate]);

  const goBack = useCallback(() => {
    setHistoryIndex((idx) => {
      if (idx <= 0) return idx;
      const newIdx = idx - 1;
      const entry = history[newIdx];
      if (entry) {
        setUrlInput(entry.url);
        setCurrentUrl(entry.url);
        setError(null);
        iframeKeyRef.current += 1;
        startBlockTimer();
      }
      return newIdx;
    });
  }, [history, startBlockTimer]);

  const goForward = useCallback(() => {
    setHistoryIndex((idx) => {
      if (idx >= history.length - 1) return idx;
      const newIdx = idx + 1;
      const entry = history[newIdx];
      if (entry) {
        setUrlInput(entry.url);
        setCurrentUrl(entry.url);
        setError(null);
        iframeKeyRef.current += 1;
        startBlockTimer();
      }
      return newIdx;
    });
  }, [history, startBlockTimer]);

  const reload = useCallback(() => {
    iframeKeyRef.current += 1;
    setLoading(true);
    startBlockTimer();
  }, [startBlockTimer]);

  // Return to the welcome screen (empty URL).
  const goHome = useCallback(() => {
    setUrlInput("");
    setCurrentUrl("");
    setError(null);
    setBlocked(false);
    setLoading(false);
    setHistory([]);
    setHistoryIndex(-1);
    if (blockTimerRef.current) clearTimeout(blockTimerRef.current);
  }, []);

  const openExternal = useCallback(() => {
    BrowserOpenURL(currentUrl).catch(() => {});
    setUseExternal(true);
  }, [currentUrl]);

  return (
    <div className="flex flex-col h-full bg-[var(--bg-sidebar)] select-none text-xs font-sans min-w-0 min-h-0">
      {/* Header */}
      <div className="flex items-center gap-1 px-2 py-1.5 border-b border-[var(--border-default)] shrink-0 bg-[var(--bg-panel)]">
        <span className="flex items-center gap-1 font-bold text-[10px] uppercase tracking-wider text-[var(--fg-tertiary)] shrink-0">
          <IconWorld className="size-3.5 text-cyan-400" />
          <span>Browser</span>
        </span>
      </div>

      {/* Navigation bar */}
      <div className="flex items-center gap-1 px-2 py-1.5 border-b border-[var(--border-default)] shrink-0 bg-[var(--bg-panel)]">
        <button
          onClick={goBack}
          disabled={historyIndex <= 0}
          className="p-1 rounded hover:bg-[var(--bg-surface-hover)] text-[var(--fg-secondary)] hover:text-white disabled:opacity-30 cursor-pointer"
          title="Back"
        >
          <IconArrowLeft className="size-3.5" />
        </button>
        <button
          onClick={goForward}
          disabled={historyIndex >= history.length - 1}
          className="p-1 rounded hover:bg-[var(--bg-surface-hover)] text-[var(--fg-secondary)] hover:text-white disabled:opacity-30 cursor-pointer"
          title="Forward"
        >
          <IconArrowRight className="size-3.5" />
        </button>
        <button
          onClick={reload}
          className="p-1 rounded hover:bg-[var(--bg-surface-hover)] text-[var(--fg-secondary)] hover:text-white cursor-pointer"
          title="Reload"
        >
          <IconRefresh className={cn("size-3.5", loading && "animate-spin")} />
        </button>
        <button
          onClick={goHome}
          className={cn(
            "p-1 rounded hover:bg-[var(--bg-surface-hover)] hover:text-white cursor-pointer",
            !currentUrl ? "text-[var(--accent-primary)]" : "text-[var(--fg-secondary)]"
          )}
          title="Home"
        >
          <IconHome className="size-3.5" />
        </button>

        {/* URL bar — Enter navigates, typed text searches */}
        <form
          className="flex-1 min-w-0"
          onSubmit={(e) => {
            e.preventDefault();
            navigate(urlInput);
          }}
        >
          <input
            value={urlInput}
            onChange={(e) => setUrlInput(e.target.value)}
            placeholder="Enter URL or search…"
            spellCheck={false}
            className="w-full bg-[var(--bg-app)] border border-[var(--border-default)] px-2 py-1 text-[11px] text-[var(--fg-primary)] focus:outline-none focus:border-[var(--accent-primary)] font-mono"
          />
        </form>

        {/* Open in external system browser */}
        <button
          onClick={openExternal}
          className="p-1 rounded hover:bg-[var(--bg-surface-hover)] text-[var(--fg-secondary)] hover:text-white cursor-pointer"
          title="Open in system browser"
        >
          <IconExternalLink className="size-3.5" />
        </button>
      </div>

      {/* Error banner (many sites block iframe embedding) */}
      {error && (
        <div className="flex items-center gap-1.5 px-2 py-1 border-b border-red-500/30 bg-red-500/10 text-red-300 text-[10px] shrink-0">
          <IconX className="size-3" />
          <span className="flex-1 truncate">{error}</span>
          <button
            onClick={() => setError(null)}
            className="hover:text-white cursor-pointer"
          >
            <IconX className="size-3" />
          </button>
        </div>
      )}

      {/* Frame / content */}
      <div className="flex-1 min-h-0 min-w-0 relative bg-[var(--bg-app)]">
        {!currentUrl ? (
          /* Welcome screen — no URL loaded yet */
          <div className="flex flex-col items-center justify-center h-full text-center p-8 space-y-5 select-none">
            <IconWorld className="size-14 text-[var(--fg-disabled)]" />
            <h2 className="text-base font-semibold text-[var(--fg-primary)]">Browser</h2>
            <p className="text-xs text-[var(--fg-secondary)] max-w-sm leading-relaxed">
              Preview APIs, docs, Swagger UI, or any URL — just type an address
              above or click a link from the terminal. Sites that block iframe
              embedding can be opened externally with the ↗ button.
            </p>
            <div className="w-full max-w-sm space-y-1.5">
              {[
                { icon: <IconSearch className="size-3.5" />, label: "Type a URL or search query", hint: "localhost:3000 · google.com · swagger docs" },
                { icon: <IconPlugConnected className="size-3.5" />, label: "Preview local dev servers", hint: "Vite, Next.js, API servers" },
                { icon: <IconFileCode className="size-3.5" />, label: "Inspect API docs & Swagger UI", hint: "FastAPI, Swagger, OpenAPI" },
              ].map((it, i) => (
                <div
                  key={i}
                  className="flex items-center gap-2.5 px-3 py-2 rounded border border-[var(--border-default)] bg-[var(--bg-panel)] text-left"
                >
                  <span className="text-[var(--accent-primary)] shrink-0">{it.icon}</span>
                  <div className="min-w-0">
                    <div className="text-[11px] font-medium text-[var(--fg-primary)]">{it.label}</div>
                    <div className="text-[10px] text-[var(--fg-tertiary)] font-mono truncate">{it.hint}</div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        ) : currentUrl.startsWith("http") ? (
          <>
            <iframe
              key={iframeKeyRef.current}
              src={currentUrl}
              className="w-full h-full border-0"
              sandbox="allow-scripts allow-same-origin allow-forms allow-popups allow-downloads"
              title="browser preview"
              onLoad={handleFrameLoad}
              onError={() => {
                setLoading(false);
                setBlocked(true);
                if (blockTimerRef.current) clearTimeout(blockTimerRef.current);
              }}
            />
            {blocked && (
              <div className="absolute inset-0 flex flex-col items-center justify-center text-center p-8 space-y-3 bg-[var(--bg-app)]">
                <IconWorld className="size-12 text-[var(--fg-disabled)]" />
                <h3 className="text-sm font-semibold text-[var(--fg-primary)]">This site can't be embedded</h3>
                <p className="text-xs text-[var(--fg-secondary)] max-w-sm leading-relaxed">
                  <span className="font-mono text-[var(--fg-primary)] break-all">{currentUrl}</span>{" "}
                  refuses to load inside a frame (X-Frame-Options / CSP), so it
                  can't be previewed here. Open it in your system browser
                  instead — local APIs, Swagger UI, and docs that allow
                  embedding will preview inline.
                </p>
                <button
                  onClick={openExternal}
                  className="inline-flex items-center gap-1.5 px-4 py-1.5 bg-[var(--accent-primary)] hover:bg-[var(--accent-hover)] text-black text-xs font-semibold rounded cursor-pointer"
                >
                  <IconExternalLink className="size-3.5" />
                  Open in browser
                </button>
                <button
                  onClick={() => navigate(urlInput)}
                  className="px-3 py-1 text-[10px] text-[var(--fg-tertiary)] hover:text-white cursor-pointer"
                >
                  Retry
                </button>
              </div>
            )}
          </>
        ) : (
          <div className="flex flex-col items-center justify-center h-full text-center p-6 space-y-3 text-[var(--fg-tertiary)]">
            <IconWorld className="size-12 text-[var(--fg-disabled)]" />
            <p className="text-xs max-w-xs">
              Preview APIs, docs, Swagger UI, or any URL here. Sites that block
              iframe embedding (X-Frame-Options) can be opened externally with
              the ↗ button.
            </p>
          </div>
        )}
      </div>

      {/* External-open notice */}
      {useExternal && (
        <div className="flex items-center gap-1.5 px-2 py-1 border-t border-[var(--border-default)] bg-[var(--bg-panel)] text-[10px] text-[var(--fg-tertiary)] shrink-0">
          <IconExternalLink className="size-3" />
          <span className="flex-1 truncate">Opened {currentUrl} in your default browser.</span>
          <button
            onClick={() => setUseExternal(false)}
            className="hover:text-white cursor-pointer"
          >
            <IconX className="size-3" />
          </button>
        </div>
      )}
    </div>
  );
}
