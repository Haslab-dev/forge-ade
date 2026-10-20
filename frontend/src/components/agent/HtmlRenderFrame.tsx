import React, { useState, useEffect, useRef, useLayoutEffect, useCallback } from 'react';
import { 
  Eye, 
  Code2, 
  ExternalLink, 
  Maximize2, 
  Minimize2, 
  Copy, 
  Check, 
  RefreshCw 
} from 'lucide-react';
import { cn } from '../../lib/utils';
import { getActiveHtmlRenderTheme } from '../../lib/themeEngine';
import { BrowserOpenURL } from '../../lib/wails';

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
  initialMode?: 'preview' | 'code';
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
  const [mode, setMode] = useState<'preview' | 'code'>(initialMode);
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [copied, setCopied] = useState(false);
  const [frameHeight, setFrameHeight] = useState<number | undefined>(undefined);
  const iframeRef = useRef<HTMLIFrameElement>(null);
  const [reloadKey, setReloadKey] = useState(0);

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
    const textToCopy = htmlContent || src || '';
    if (!textToCopy) return;
    try {
      await navigator.clipboard.writeText(textToCopy);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // ignore
    }
  }, [htmlContent, src]);

  const handleOpenExternal = useCallback(() => {
    if (src) {
      void BrowserOpenURL(src).catch(() => {});
    } else if (htmlContent) {
      const blob = new Blob([injectBootstrap(htmlContent, theme)], { type: 'text/html' });
      const url = URL.createObjectURL(blob);
      void BrowserOpenURL(url).catch(() => {});
    }
  }, [src, htmlContent, theme]);

  const processedHtml = useRef('');
  useLayoutEffect(() => {
    if (htmlContent) {
      processedHtml.current = injectBootstrap(htmlContent, theme);
    }
  }, [htmlContent, theme]);

  const contentLines = htmlContent.split('\n');

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
        <div className="flex h-9 shrink-0 items-center justify-between border-b border-border bg-panel px-3 text-ui-xs">
          <div className="flex items-center gap-2 font-mono text-foreground-subtle min-w-0">
            <span className="flex size-2 rounded-full bg-accent" />
            <span className="truncate font-medium text-foreground">{title}</span>
            <span className="rounded bg-surface-hover px-1.5 py-0.5 text-[10px] uppercase font-semibold tracking-wider text-foreground-subtlest">
              HTML Preview
            </span>
          </div>

          <div className="flex items-center gap-1 shrink-0">
            {/* Preview / Code switcher */}
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
            </div>

            <div className="h-4 w-px bg-border/60 mx-1" />

            {/* Reload button */}
            <button
              type="button"
              onClick={() => setReloadKey(k => k + 1)}
              className="flex size-6 items-center justify-center rounded text-foreground-subtle hover:bg-surface-hover hover:text-foreground transition-colors cursor-pointer"
              title="Reload preview"
            >
              <RefreshCw className="size-3.5" />
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

      {/* Body */}
      <div className="flex-1 min-h-0 relative overflow-hidden bg-background">
        {mode === 'preview' ? (
          <iframe
            key={reloadKey}
            ref={iframeRef}
            src={src}
            srcDoc={src ? undefined : (processedHtml.current || injectBootstrap(htmlContent, theme))}
            title={title}
            sandbox="allow-scripts allow-forms allow-popups allow-modals allow-downloads"
            className="size-full border-0 bg-transparent"
          />
        ) : (
          <div className="size-full overflow-auto font-mono text-ui-xs leading-[18px] p-3 select-text bg-background">
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
      </div>
    </div>
  );
};
