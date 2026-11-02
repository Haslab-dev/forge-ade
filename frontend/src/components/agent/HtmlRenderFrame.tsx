import React, { useState, useEffect, useRef, useLayoutEffect, useCallback, useMemo } from 'react';
import { 
  Eye, 
  Code2, 
  Columns, 
  ExternalLink, 
  Maximize2, 
  Minimize2, 
  Copy, 
  Check, 
  RefreshCw, 
  Download,
  Smartphone,
  Tablet,
  Monitor,
  ChevronLeft,
  ChevronRight,
  Layers
} from 'lucide-react';
import { cn } from '../../lib/utils';
import { getActiveHtmlRenderTheme } from '../../lib/themeEngine';
import { BrowserOpenURL, OpenDetachedWindow } from '../../lib/wails';
import {
  detectSlides,
  generateArtifactFilename,
  ViewportMode,
  VIEWPORT_WIDTHS,
  ArtifactVersion
} from './artifactsUtils';

const HOST_CONTEXT_CHANGED_METHOD = 'ui/notifications/host-context-changed';
const OPEN_LINK_METHOD = 'ui/open-link';
const SIZE_CHANGED_METHOD = 'ui/notifications/size-changed';

interface HtmlRenderFrameProps {
  htmlContent?: string;
  src?: string;
  title?: string;
  className?: string;
  fitHeight?: boolean;
  showControls?: boolean;
  initialMode?: 'preview' | 'code' | 'split';
  onFullscreenChange?: (fullscreen: boolean) => void;
}

const BASE_CSS = `
html {
  background: var(--background);
  color: var(--foreground);
  font-family: var(--font-sans);
  font-size: 14px;
  line-height: 1.5;
  -webkit-font-smoothing: antialiased;
  scrollbar-width: thin;
  scrollbar-color: var(--border) transparent;
}
body {
  margin: 0;
  padding: 16px;
  box-sizing: border-box;
}
code, kbd, pre, samp {
  font-family: var(--font-mono);
}
:where(:focus-visible) {
  outline: 2px solid var(--ring);
}
`;

function buildBootstrap(theme: { appearance: string; variables: Record<string, string> }): string {
  const vars = Object.entries(theme.variables)
    .map(([k, v]) => `${k}:${v};`)
    .join('');
  const rootStyle = `:root { color-scheme: ${theme.appearance}; ${vars} } ${BASE_CSS}`;

  const script = `
(function() {
  var styleEl = document.getElementById("forge-html-theme");
  if (!styleEl) {
    styleEl = document.createElement("style");
    styleEl.id = "forge-html-theme";
    document.head.insertBefore(styleEl, document.head.firstChild);
  }
  function updateTheme(t) {
    if (!t || !t.variables) return;
    var css = ":root { color-scheme: " + (t.appearance === "light" ? "light" : "dark") + "; ";
    for (var k in t.variables) {
      if (/^--[a-z0-9-]+$/.test(k)) {
        css += k + ":" + String(t.variables[k]).replace(/[;{}<>]/g, "") + ";";
      }
    }
    styleEl.textContent = css + "} " + ${JSON.stringify(BASE_CSS)};
  }

  window.addEventListener("message", function(e) {
    var d = e.data;
    if (d && d.jsonrpc === "2.0" && d.method === ${JSON.stringify(HOST_CONTEXT_CHANGED_METHOD)}) {
      if (d.params && d.params.styles) {
        updateTheme({ appearance: d.params.theme, variables: d.params.styles.variables });
      }
    }
  });

  document.addEventListener("click", function(e) {
    var target = e.target;
    var anchor = target && target.closest ? target.closest("a[href]") : null;
    if (!anchor) return;
    var href = anchor.getAttribute("href");
    if (!href || href.startsWith("#") || href.startsWith("javascript:")) return;
    e.preventDefault();
    if (window.parent !== window) {
      window.parent.postMessage({
        jsonrpc: "2.0",
        method: ${JSON.stringify(OPEN_LINK_METHOD)},
        params: { url: anchor.href }
      }, "*");
    }
  }, true);

  if (window.parent !== window) {
    var lastH = 0;
    var reportSize = function() {
      var r = document.documentElement;
      var h = Math.ceil(Math.max(r.scrollHeight, r.offsetHeight, document.body ? document.body.scrollHeight : 0));
      if (h !== lastH && h > 0) {
        lastH = h;
        window.parent.postMessage({
          jsonrpc: "2.0",
          method: ${JSON.stringify(SIZE_CHANGED_METHOD)},
          params: { height: h }
        }, "*");
      }
    };
    if (window.ResizeObserver) {
      new ResizeObserver(reportSize).observe(document.documentElement);
    }
    window.addEventListener("DOMContentLoaded", reportSize);
    window.addEventListener("load", reportSize);
  }
})();
`;

  return `<style id="forge-html-theme">${rootStyle}</style><script>${script}</script>`;
}

export function injectBootstrap(html: string, theme: { appearance: string; variables: Record<string, string> }): string {
  const bootstrap = buildBootstrap(theme);
  const headMatch = /<head(?:\s[^>]*)?>/i.exec(html);
  if (headMatch) {
    const at = headMatch.index + headMatch[0].length;
    return html.slice(0, at) + bootstrap + html.slice(at);
  }
  const htmlMatch = /<html(?:\s[^>]*)?>/i.exec(html);
  if (htmlMatch) {
    const at = htmlMatch.index + htmlMatch[0].length;
    return `${html.slice(0, at)}<head>${bootstrap}</head>${html.slice(at)}`;
  }
  const doctype = /^\s*<!doctype[^>]*>/i.exec(html);
  if (doctype) {
    const at = doctype[0].length;
    return `${html.slice(0, at)}<head>${bootstrap}</head>${html.slice(at)}`;
  }
  return `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">${bootstrap}</head><body>${html}</body></html>`;
}

export const HtmlRenderFrame: React.FC<HtmlRenderFrameProps> = ({
  htmlContent = '',
  src,
  title = 'HTML Preview',
  className,
  fitHeight = false,
  showControls = true,
  initialMode = 'preview',
  onFullscreenChange
}) => {
  const [mode, setMode] = useState<'preview' | 'code' | 'split'>(initialMode);
  const [viewport, setViewport] = useState<ViewportMode>('responsive');
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [copied, setCopied] = useState(false);
  const [frameHeight, setFrameHeight] = useState<number | undefined>(undefined);
  const iframeRef = useRef<HTMLIFrameElement>(null);
  const [reloadKey, setReloadKey] = useState(0);

  // ── Versioning History ───────────────────────────────────────────────────────
  const [versions, setVersions] = useState<ArtifactVersion[]>([
    { version: 1, content: htmlContent, timestamp: Date.now() }
  ]);
  const [activeVersionIdx, setActiveVersionIdx] = useState(0);

  useEffect(() => {
    if (!htmlContent) return;
    setVersions(prev => {
      const last = prev[prev.length - 1];
      if (last && last.content === htmlContent) return prev;
      const nextVer: ArtifactVersion = {
        version: prev.length + 1,
        content: htmlContent,
        timestamp: Date.now()
      };
      const updated = [...prev, nextVer];
      setActiveVersionIdx(updated.length - 1);
      return updated;
    });
  }, [htmlContent]);

  const activeContent = versions[activeVersionIdx]?.content ?? htmlContent;

  // ── Carousel Slides Detection ───────────────────────────────────────────────
  const slides = useMemo(() => detectSlides(activeContent), [activeContent]);
  const [currentSlideIdx, setCurrentSlideIdx] = useState(0);

  useEffect(() => {
    setCurrentSlideIdx(0);
  }, [activeContent]);

  const currentSlideHtml = slides.length > 1 ? slides[currentSlideIdx] || activeContent : activeContent;

  const [theme, setTheme] = useState(getActiveHtmlRenderTheme);

  // Sync theme updates
  useEffect(() => {
    const onThemeChanged = () => {
      const next = getActiveHtmlRenderTheme();
      setTheme(next);
      if (iframeRef.current?.contentWindow) {
        iframeRef.current.contentWindow.postMessage({
          jsonrpc: '2.0',
          method: HOST_CONTEXT_CHANGED_METHOD,
          params: {
            theme: next.appearance,
            styles: { variables: next.variables }
          }
        }, '*');
      }
    };
    window.addEventListener('forge:theme-changed', onThemeChanged);
    return () => window.removeEventListener('forge:theme-changed', onThemeChanged);
  }, []);

  // Listen to iframe postMessages
  useEffect(() => {
    const onMessage = (event: MessageEvent) => {
      const data = event.data;
      if (!data || typeof data !== 'object' || data.jsonrpc !== '2.0') return;

      if (data.method === OPEN_LINK_METHOD && data.params?.url) {
        void BrowserOpenURL(data.params.url).catch(() => {});
      } else if (data.method === SIZE_CHANGED_METHOD && fitHeight && typeof data.params?.height === 'number') {
        setFrameHeight(data.params.height);
      }
    };

    window.addEventListener('message', onMessage);
    return () => window.removeEventListener('message', onMessage);
  }, [fitHeight]);

  const toggleFullscreen = useCallback(() => {
    setIsFullscreen(prev => {
      const next = !prev;
      onFullscreenChange?.(next);
      return next;
    });
  }, [onFullscreenChange]);

  const handleCopy = useCallback(async () => {
    const textToCopy = activeContent || src || '';
    if (!textToCopy) return;
    try {
      await navigator.clipboard.writeText(textToCopy);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // ignore
    }
  }, [activeContent, src]);

  const handleDownload = useCallback(() => {
    const text = activeContent || '';
    if (!text) return;
    const blob = new Blob([text], { type: 'text/html' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = generateArtifactFilename(title, activeVersionIdx + 1, 'html');
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  }, [activeContent, title, activeVersionIdx]);

  const handleDetachWindow = useCallback(() => {
    const winTitle = `${title} - ForgeADE Artifact`;
    const targetUrl = src || '/';
    void OpenDetachedWindow(winTitle, targetUrl, 1024, 768).catch(() => {});
  }, [title, src]);

  const handleOpenExternal = useCallback(() => {
    if (src) {
      void BrowserOpenURL(src).catch(() => {});
    } else if (activeContent) {
      const blob = new Blob([injectBootstrap(activeContent, theme)], { type: 'text/html' });
      const url = URL.createObjectURL(blob);
      void BrowserOpenURL(url).catch(() => {});
    }
  }, [src, activeContent, theme]);

  const processedHtml = useRef('');
  useLayoutEffect(() => {
    if (currentSlideHtml) {
      processedHtml.current = injectBootstrap(currentSlideHtml, theme);
    }
  }, [currentSlideHtml, theme]);

  const contentLines = activeContent.split('\n');
  const totalVersions = versions.length;
  const currentVersionNumber = activeVersionIdx + 1;

  return (
    <div
      className={cn(
        "flex flex-col bg-surface border border-border rounded-lg overflow-hidden transition-all",
        isFullscreen ? "fixed inset-4 z-50 shadow-2xl border-border/80 bg-background" : "size-full",
        className
      )}
      style={fitHeight && !isFullscreen && frameHeight ? { height: frameHeight } : undefined}
    >
      {showControls && (
        <div className="flex h-10 shrink-0 items-center justify-between border-b border-border bg-panel px-3 text-ui-xs gap-2 select-none">
          {/* Left Title & Version Controls */}
          <div className="flex items-center gap-2 font-mono text-foreground-subtle min-w-0">
            <span className="flex size-2 rounded-full bg-accent" />
            <span className="truncate font-semibold text-foreground">{title}</span>
            <span className="rounded bg-surface-hover px-1.5 py-0.5 text-[10px] uppercase font-semibold tracking-wider text-foreground-subtlest">
              Artifact
            </span>

            {/* Version stepper */}
            {totalVersions > 1 && (
              <div className="flex items-center gap-0.5 ml-1 px-1 py-0.5 rounded border border-border/70 bg-surface/50 text-[11px]">
                <button
                  type="button"
                  disabled={activeVersionIdx <= 0}
                  onClick={() => setActiveVersionIdx(i => Math.max(0, i - 1))}
                  className="p-0.5 hover:text-foreground disabled:opacity-30 cursor-pointer"
                  title="Previous version"
                >
                  <ChevronLeft className="size-3" />
                </button>
                <span className="font-semibold text-foreground px-1">
                  v{currentVersionNumber}/{totalVersions}
                </span>
                <button
                  type="button"
                  disabled={activeVersionIdx >= totalVersions - 1}
                  onClick={() => setActiveVersionIdx(i => Math.min(totalVersions - 1, i + 1))}
                  className="p-0.5 hover:text-foreground disabled:opacity-30 cursor-pointer"
                  title="Next version"
                >
                  <ChevronRight className="size-3" />
                </button>
              </div>
            )}
          </div>

          {/* Right Controls */}
          <div className="flex items-center gap-1 shrink-0">
            {/* Viewport Switcher */}
            <div className="flex items-center rounded-md bg-surface-hover/80 p-0.5 border border-border/50">
              <button
                type="button"
                onClick={() => setViewport('desktop')}
                className={cn(
                  "p-1 rounded transition-colors cursor-pointer",
                  viewport === 'desktop' || viewport === 'responsive'
                    ? "bg-panel text-foreground shadow-xs"
                    : "text-foreground-subtle hover:text-foreground"
                )}
                title="Desktop viewport (100%)"
              >
                <Monitor className="size-3" />
              </button>
              <button
                type="button"
                onClick={() => setViewport('tablet')}
                className={cn(
                  "p-1 rounded transition-colors cursor-pointer",
                  viewport === 'tablet'
                    ? "bg-panel text-foreground shadow-xs"
                    : "text-foreground-subtle hover:text-foreground"
                )}
                title="Tablet viewport (768px)"
              >
                <Tablet className="size-3" />
              </button>
              <button
                type="button"
                onClick={() => setViewport('mobile')}
                className={cn(
                  "p-1 rounded transition-colors cursor-pointer",
                  viewport === 'mobile'
                    ? "bg-panel text-foreground shadow-xs"
                    : "text-foreground-subtle hover:text-foreground"
                )}
                title="Mobile viewport (375px)"
              >
                <Smartphone className="size-3" />
              </button>
            </div>

            <div className="h-4 w-px bg-border/60 mx-0.5" />

            {/* Preview / Code / Split Switcher */}
            <div className="flex items-center rounded-md bg-surface-hover/80 p-0.5 border border-border/50">
              <button
                type="button"
                onClick={() => setMode('preview')}
                className={cn(
                  "flex items-center gap-1 rounded px-2 py-0.5 font-medium transition-colors cursor-pointer",
                  mode === 'preview'
                    ? "bg-panel text-foreground shadow-xs"
                    : "text-foreground-subtle hover:text-foreground"
                )}
                title="Rendered visual preview"
              >
                <Eye className="size-3" />
                <span>Preview</span>
              </button>
              <button
                type="button"
                onClick={() => setMode('code')}
                className={cn(
                  "flex items-center gap-1 rounded px-2 py-0.5 font-medium transition-colors cursor-pointer",
                  mode === 'code'
                    ? "bg-panel text-foreground shadow-xs"
                    : "text-foreground-subtle hover:text-foreground"
                )}
                title="Source code"
              >
                <Code2 className="size-3" />
                <span>Code</span>
              </button>
              <button
                type="button"
                onClick={() => setMode('split')}
                className={cn(
                  "flex items-center gap-1 rounded px-2 py-0.5 font-medium transition-colors cursor-pointer",
                  mode === 'split'
                    ? "bg-panel text-foreground shadow-xs"
                    : "text-foreground-subtle hover:text-foreground"
                )}
                title="Side-by-side Split View"
              >
                <Columns className="size-3" />
                <span>Split</span>
              </button>
            </div>

            <div className="h-4 w-px bg-border/60 mx-0.5" />

            {/* Reload button */}
            <button
              type="button"
              onClick={() => setReloadKey(k => k + 1)}
              className="flex size-6 items-center justify-center rounded text-foreground-subtle hover:bg-surface-hover hover:text-foreground transition-colors cursor-pointer"
              title="Reload preview"
            >
              <RefreshCw className="size-3.5" />
            </button>

            {/* Download button */}
            <button
              type="button"
              onClick={handleDownload}
              className="flex size-6 items-center justify-center rounded text-foreground-subtle hover:bg-surface-hover hover:text-foreground transition-colors cursor-pointer"
              title="Download artifact (.html)"
            >
              <Download className="size-3.5" />
            </button>

            {/* Copy button */}
            <button
              type="button"
              onClick={handleCopy}
              className="flex size-6 items-center justify-center rounded text-foreground-subtle hover:bg-surface-hover hover:text-foreground transition-colors cursor-pointer"
              title="Copy code"
            >
              {copied ? <Check className="size-3.5 text-success" /> : <Copy className="size-3.5" />}
            </button>

            {/* Pop out into detached window */}
            <button
              type="button"
              onClick={handleDetachWindow}
              className="flex size-6 items-center justify-center rounded text-foreground-subtle hover:bg-surface-hover hover:text-foreground transition-colors cursor-pointer"
              title="Detach into native secondary window"
            >
              <ExternalLink className="size-3.5 text-primary" />
            </button>

            {/* Open in external browser */}
            <button
              type="button"
              onClick={handleOpenExternal}
              className="flex size-6 items-center justify-center rounded text-foreground-subtle hover:bg-surface-hover hover:text-foreground transition-colors cursor-pointer"
              title="Open in external browser"
            >
              <ExternalLink className="size-3.5" />
            </button>

            {/* Fullscreen toggle */}
            <button
              type="button"
              onClick={toggleFullscreen}
              className="flex size-6 items-center justify-center rounded text-foreground-subtle hover:bg-surface-hover hover:text-foreground transition-colors cursor-pointer"
              title={isFullscreen ? "Exit full screen" : "Full screen"}
            >
              {isFullscreen ? <Minimize2 className="size-3.5" /> : <Maximize2 className="size-3.5" />}
            </button>
          </div>
        </div>
      )}

      {/* Multi-Slide Carousel Header (if slides detected) */}
      {slides.length > 1 && (
        <div className="flex h-7 items-center justify-between px-3 border-b border-border/60 bg-surface/80 text-ui-xs font-mono select-none">
          <div className="flex items-center gap-1.5 text-foreground-subtle">
            <Layers className="size-3 text-primary" />
            <span>Slide {currentSlideIdx + 1} of {slides.length}</span>
          </div>
          <div className="flex items-center gap-1">
            <button
              type="button"
              disabled={currentSlideIdx <= 0}
              onClick={() => setCurrentSlideIdx(i => Math.max(0, i - 1))}
              className="p-1 rounded hover:bg-surface-hover disabled:opacity-30 cursor-pointer"
            >
              <ChevronLeft className="size-3" />
            </button>
            <div className="flex items-center gap-1 px-1">
              {slides.map((_, idx) => (
                <button
                  key={idx}
                  type="button"
                  onClick={() => setCurrentSlideIdx(idx)}
                  className={cn(
                    "size-1.5 rounded-full transition-all cursor-pointer",
                    idx === currentSlideIdx ? "bg-primary w-3" : "bg-border hover:bg-foreground-subtle"
                  )}
                  title={`Go to slide ${idx + 1}`}
                />
              ))}
            </div>
            <button
              type="button"
              disabled={currentSlideIdx >= slides.length - 1}
              onClick={() => setCurrentSlideIdx(i => Math.min(slides.length - 1, i + 1))}
              className="p-1 rounded hover:bg-surface-hover disabled:opacity-30 cursor-pointer"
            >
              <ChevronRight className="size-3" />
            </button>
          </div>
        </div>
      )}

      {/* Body Canvas */}
      <div className="flex-1 min-h-0 relative overflow-hidden bg-background flex flex-col md:flex-row">
        {/* Code Pane (for 'code' or 'split') */}
        {(mode === 'code' || mode === 'split') && (
          <div className={cn(
            "overflow-auto font-mono text-ui-xs leading-[18px] p-3 select-text bg-background border-r border-border/60",
            mode === 'split' ? "w-full md:w-1/2 h-1/2 md:h-full shrink-0" : "size-full"
          )}>
            {contentLines.map((line, idx) => (
              <div key={idx} className="flex hover:bg-surface-hover/30 transition-colors">
                <span className="w-10 shrink-0 text-right pr-3 select-none text-foreground-subtlest">
                  {idx + 1}
                </span>
                <span className="flex-1 whitespace-pre text-foreground/90 font-mono">
                  {line}
                </span>
              </div>
            ))}
          </div>
        )}

        {/* Live Preview Pane (for 'preview' or 'split') */}
        {(mode === 'preview' || mode === 'split') && (
          <div className={cn(
            "relative flex items-center justify-center overflow-auto p-2 bg-neutral-900/10 dark:bg-black/20",
            mode === 'split' ? "w-full md:w-1/2 h-1/2 md:h-full" : "size-full"
          )}>
            <div
              className={cn(
                "h-full transition-all flex flex-col overflow-hidden bg-background",
                viewport === 'tablet' && "w-[768px] shadow-xl border border-border/80 rounded-lg",
                viewport === 'mobile' && "w-[375px] shadow-2xl border-2 border-border rounded-2xl",
                (viewport === 'desktop' || viewport === 'responsive') && "size-full"
              )}
            >
              <iframe
                key={reloadKey + currentSlideIdx}
                ref={iframeRef}
                src={src}
                srcDoc={src ? undefined : (processedHtml.current || injectBootstrap(currentSlideHtml, theme))}
                title={title}
                sandbox="allow-scripts allow-forms allow-popups allow-modals allow-downloads"
                className="size-full border-0 bg-transparent"
              />
            </div>
          </div>
        )}
      </div>
    </div>
  );
};
