import { useEffect, useRef, useState } from "react";
import { Terminal } from "xterm";
import "xterm/css/xterm.css";
import { FitAddon } from "xterm-addon-fit";
import { IconArrowUp, IconArrowDown } from "@tabler/icons-react";
import { WriteSession, ResizeSession, GetHomeDir, OpenInFinder, IsDir, BrowserOpenURL, GetClipboardFiles } from "../lib/wails";
import { EventsOn } from "../lib/wails";
import { globalOpenFile } from "../panels/editor";
import { openInBrowser } from "../panels/browser-panel";

// Inject terminal link and sizing styles once. xterm's own CSS positions
// the screen/viewport absolutely; forcing them to fill the container keeps
// fit() and scroll behaviour correct inside the panel.
const styleId = "forge-xterm-link-style";
if (typeof document !== "undefined" && !document.getElementById(styleId)) {
  const style = document.createElement("style");
  style.id = styleId;
  style.textContent = `
    .xterm .xterm-link-layer a, .xterm-link-layer a {
      text-decoration: underline !important;
      text-decoration-color: #4F8CFF !important;
      cursor: pointer !important;
    }
    .xterm {
      padding: 6px 8px !important;
      height: 100% !important;
      width: 100% !important;
      box-sizing: border-box !important;
    }
    .xterm-screen {
      height: 100% !important;
      width: 100% !important;
    }
    .xterm-viewport {
      height: 100% !important;
      width: 100% !important;
    }
  `;
  document.head.appendChild(style);
}

let homeDir = "";
GetHomeDir().then((h) => { homeDir = h; }).catch(() => {});

// URL detection for terminal links. Matches http/https URLs plus bare hosts
// like localhost:3000 / 127.0.0.1:8080 (dev-server links often print without
// a scheme). Groups: 1 = full URL.
const URL_REGEX = /\b(?:https?:\/\/|www\.|localhost:\d+|127\.\d+\.\d+\.\d+:\d+)[^\s<>"')\]]*/gi;

function findUrl(text: string): string | null {
  URL_REGEX.lastIndex = 0;
  const m = URL_REGEX.exec(text);
  if (!m || !m[0]) return null;
  let url = m[0];
  // Normalize: add scheme to bare localhost / IP hosts, strip trailing
  // punctuation like ')' or ','.
  url = url.replace(/[),;]+$/, "");
  if (/^www\./i.test(url)) url = "https://" + url;
  else if (!/^https?:\/\//i.test(url)) url = "http://" + url;
  return url;
}

// ============================================================================
// Global output listener + per-session buffers.
//
// Output arrives as raw byte chunks over the Wails event bus. We buffer them
// per session so a terminal that mounts AFTER output started can replay the
// stream. The buffer and the live handler both feed xterm raw — xterm.js
// parses the byte stream itself (it handles \r in-place redraws, ANSI cursor
// moves, split escape sequences, etc.). Never split on lines or hold chunks
// back: that breaks input echo, live agent output, and spinners.
// ============================================================================

type OutputHandler = (data: string) => void;
const outputHandlers = new Map<string, OutputHandler>();
const outputBuffers = new Map<string, string[]>();
let globalInitialized = false;

function ensureGlobalListener() {
  if (globalInitialized) return;
  globalInitialized = true;
  EventsOn("session:output", (payload: any) => {
    if (payload && payload.id && payload.data) {
      let buf = outputBuffers.get(payload.id);
      if (!buf) {
        buf = [];
        outputBuffers.set(payload.id, buf);
      }
      buf.push(payload.data);
      if (buf.length > 10000) buf.splice(0, buf.length - 10000);
      const handler = outputHandlers.get(payload.id);
      if (handler) handler(payload.data);
    }
  });
  EventsOn("session:closed", (payload: any) => {
    if (payload && payload.id) {
      outputBuffers.delete(payload.id);
      outputHandlers.delete(payload.id);
    }
  });
}

ensureGlobalListener();

// ============================================================================
// xterm.js themes — one per app theme so the terminal adapts dynamically.
// Instead of a hardcoded map, each theme's palette is READ from the active CSS
// variables (--bg / --fg / --accent / status colors) at runtime, so the
// terminal always matches the palette in index.css — including new palettes
// added later with zero code changes.
// ============================================================================

interface XtermTheme {
  background: string;
  foreground: string;
  cursor: string;
  cursorAccent: string;
  selectionBackground: string;
  selectionForeground: string;
  selectionInactiveBackground: string;
  black: string;
  red: string;
  green: string;
  yellow: string;
  blue: string;
  magenta: string;
  cyan: string;
  white: string;
  brightBlack: string;
  brightRed: string;
  brightGreen: string;
  brightYellow: string;
  brightBlue: string;
  brightMagenta: string;
  brightCyan: string;
  brightWhite: string;
}

/** Mix two hex colors, `weight` = amount of `b` (0..1). */
function mixHex(a: string, b: string, weight: number): string {
  const pa = parseInt(a.slice(1), 16);
  const pb = parseInt(b.slice(1), 16);
  const r = Math.round(((pa >> 16) & 255) * (1 - weight) + ((pb >> 16) & 255) * weight);
  const g = Math.round(((pa >> 8) & 255) * (1 - weight) + ((pb >> 8) & 255) * weight);
  const bl = Math.round((pa & 255) * (1 - weight) + (pb & 255) * weight);
  return "#" + ((r << 16) | (g << 8) | bl).toString(16).padStart(6, "0");
}

/** Lighten (w>0) or darken (w<0) a hex color against white/black. */
function shadeHex(hex: string, w: number): string {
  return w >= 0 ? mixHex(hex, "#ffffff", w) : mixHex(hex, "#000000", -w);
}

/** Resolve the active palette from the root element's CSS variables. */
function paletteFromCss(): { bg: string; fg: string; accent: string; success: string; warning: string; danger: string; info: string } {
  const s = getComputedStyle(document.documentElement);
  const read = (name: string, fallback: string) => {
    const v = s.getPropertyValue(name).trim();
    return /^#[0-9a-f]{6}$/i.test(v) ? v : fallback;
  };
  return {
    bg: read("--bg", "#1a1a1a"),
    fg: read("--fg", "#cccccc"),
    accent: read("--accent", "#0057fe"),
    success: read("--success", "#4ade80"),
    warning: read("--warning", "#facc15"),
    danger: read("--danger", "#f87171"),
    info: read("--info", "#38bdf8"),
  };
}

/** Build the full xterm theme from a resolved palette. */
function buildXtermTheme(p: ReturnType<typeof paletteFromCss>): XtermTheme {
  const isLight = false; // all bundled palettes are dark; derive by luminance
  return {
    background: p.bg,
    foreground: p.fg,
    cursor: p.accent,
    cursorAccent: p.bg,
    selectionBackground: "#2563eb",
    selectionForeground: "#ffffff",
    selectionInactiveBackground: "rgba(37, 99, 235, 0.25)",
    black: shadeHex(p.bg, -0.08),
    red: p.danger,
    green: p.success,
    yellow: p.warning,
    blue: p.accent,
    magenta: p.accent,
    cyan: p.info,
    white: p.fg,
    brightBlack: mixHex(p.fg, p.bg, 0.55),
    brightRed: shadeHex(p.danger, 0.15),
    brightGreen: shadeHex(p.success, 0.15),
    brightYellow: shadeHex(p.warning, 0.15),
    brightBlue: shadeHex(p.accent, 0.15),
    brightMagenta: shadeHex(p.accent, 0.15),
    brightCyan: shadeHex(p.info, 0.15),
    brightWhite: shadeHex(p.fg, 0.1),
  };
}

/** Get the xterm theme for the current app theme name. */
function xtermThemeFor(): XtermTheme {
  const p = paletteFromCss();
  return buildXtermTheme(p);
}

// ============================================================================
// React component — faithful port of the agent-terminal XTermTerminal:
// raw writes, ResizeObserver-driven fit, no focus stealing, no burst timers.
// ============================================================================

interface TerminalViewProps {
  sessionId: string;
  isActive?: boolean;
}

export function TerminalView({ sessionId, isActive = true }: TerminalViewProps) {
  const [isReady] = useState(true);
  const containerRef = useRef<HTMLDivElement>(null);
  const termRef = useRef<Terminal | null>(null);
  const fitAddonRef = useRef<FitAddon | null>(null);
  const isActiveRef = useRef(isActive);
  // Scroll state — shows the up/down buttons when the buffer has scrolled
  // beyond the viewport (e.g. an agent wrote a huge amount of output).
  const [canScroll, setCanScroll] = useState(false);
  const [atBottom, setAtBottom] = useState(true);

  // Update scroll indicator whenever the terminal scrolls / renders.
  // termRef is set inside the mount effect below (which runs after this),
  // so retry until the terminal exists — deps never change once mounted.
  const scrollDispRef = useRef<{ dispose: () => void } | null>(null);
  useEffect(() => {
    let raf = 0;
    const trySubscribe = () => {
      const t = termRef.current;
      if (!t) {
        raf = requestAnimationFrame(trySubscribe);
        return;
      }
      const updateScrollState = () => {
        const term = termRef.current;
        if (!term) return;
        const can = term.buffer.active.viewportY > 0 || term.buffer.active.baseY > 0;
        const bottom = term.buffer.active.viewportY === term.buffer.active.baseY;
        setCanScroll(can);
        setAtBottom(bottom);
      };
      const disp1 = t.onScroll(updateScrollState);
      const disp2 = t.onRender(() => updateScrollState());
      scrollDispRef.current = {
        dispose: () => {
          disp1.dispose();
          disp2.dispose();
        },
      };
      updateScrollState();
    };
    trySubscribe();
    return () => {
      cancelAnimationFrame(raf);
      scrollDispRef.current?.dispose();
      scrollDispRef.current = null;
    };
  }, [isReady, sessionId]);

  const scrollUp = () => {
    const t = termRef.current;
    if (!t) return;
    // Scroll up by ~80% of the viewport height.
    const lines = Math.max(1, Math.floor(t.rows * 0.8));
    t.scrollLines(-lines);
  };

  const scrollDown = () => {
    const t = termRef.current;
    if (!t) return;
    t.scrollToBottom();
  };

  useEffect(() => {
    isActiveRef.current = isActive;
  }, [isActive]);

  useEffect(() => {
    if (!isReady) return;
    ensureGlobalListener();
    const container = containerRef.current;
    if (!container) return;

    let disposed = false;

    const term = new Terminal({
      cursorBlink: true,
      cursorStyle: "block",
      fontSize: 13,
      fontFamily: "'JetBrains Mono', 'Fira Code', Menlo, Monaco, monospace",
      lineHeight: 1.2,
      theme: xtermThemeFor(),
      allowTransparency: false,
      scrollback: 5000,
    });

    const fitAddon = new FitAddon();
    term.loadAddon(fitAddon);

    term.attachCustomKeyEventHandler((e: KeyboardEvent) => {
      // Cmd/Ctrl+K clears the terminal
      if (e.type === "keydown" && (e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        term.clear();
        return false;
      }
      // Cmd+C copies the selection
      if (e.metaKey && !e.ctrlKey && e.key === "c") {
        e.preventDefault();
        const sel = term.getSelection();
        if (sel) navigator.clipboard.writeText(sel).catch(() => {});
        return false;
      }
      // Cmd+V pastes from the clipboard
      if (e.metaKey && !e.ctrlKey && e.key === "v") {
        e.preventDefault();
        GetClipboardFiles().then((paths) => {
          if (paths && paths.length) {
            // Files copied from Finder → paste full paths (shell-quoted).
            const quoted = paths.map((p) => p.includes(" ") ? `'${p.replace(/'/g, "'\\''")}'` : p).join(" ");
            term.paste(quoted);
          } else {
            navigator.clipboard.readText().then((text) => {
              if (text) term.paste(text);
            }).catch(() => {});
          }
        }).catch(() => {
          navigator.clipboard.readText().then((text) => {
            if (text) term.paste(text);
          }).catch(() => {});
        });
        return false;
      }
      return true;
    });

    termRef.current = term;
    fitAddonRef.current = fitAddon;

    term.open(container);

    // Register a link provider so URLs in output (e.g. dev-server "Server
    // running at http://localhost:3000") become clickable links: blue
    // underline on hover + pointer cursor, click opens the internal browser.
    const linkProvider = {
      provideLinks(bufferLineNumber: number, callback: (links: any[] | undefined) => void) {
        const t = termRef.current;
        if (!t) { callback(undefined); return; }
        const line = t.buffer.active.getLine(bufferLineNumber - 1);
        if (!line) { callback(undefined); return; }
        const lineText = line.translateToString(true);
        const url = findUrl(lineText);
        if (!url) { callback(undefined); return; }
        // Re-find the actual match offset in the raw line text.
        URL_REGEX.lastIndex = 0;
        const m = URL_REGEX.exec(lineText);
        if (!m || !m[0]) { callback(undefined); return; }
        const rawMatch = m[0];
        const idx = m.index;
        callback([{
          range: {
            start: { x: idx + 1, y: bufferLineNumber },
            end: { x: idx + rawMatch.length + 1, y: bufferLineNumber },
          },
          text: rawMatch,
          decorations: { pointerCursor: true, underline: true },
          activate: () => {
            // localhost → in-app browser; other hosts → system default browser.
            let host = "";
            try { host = new URL(url).hostname; } catch { host = ""; }
            if (/^(localhost|127\.0\.0\.1|0\.0\.0\.0|\[::1\])$/i.test(host)) {
              openInBrowser(url);
            } else {
              BrowserOpenURL(url);
            }
          },
        }]);
      },
    };
    const disposeLinks = term.registerLinkProvider(linkProvider as any);

    // Double-click a path → open it in the editor / Finder
    const dblclickHandler = () => {
      const selection = term.getSelection().trim();
      if (!selection) return;
      const resolved = selection.startsWith("~/") ? (homeDir || "") + selection.slice(1) : selection;
      const cleanPath = resolved.replace(/:(\d+)(:\d+)?$/, "").trim();
      if (!cleanPath) return;
      IsDir(cleanPath).then((isDir) => {
        if (isDir) OpenInFinder(cleanPath).catch(() => {});
        else globalOpenFile(cleanPath);
      }).catch(() => globalOpenFile(cleanPath));
    };
    term.element?.addEventListener("dblclick", dblclickHandler);

    // Click inside the terminal → focus it (xterm manages its own textarea;
    // do not poke at it — that steals scroll position mid-input)
    const handleFocusClick = () => {
      try {
        term.focus();
      } catch { /* ignore */ }
    };
    container.addEventListener("click", handleFocusClick);

    // Drive fit() off container layout. fit() fires term.onResize with the
    // new cols/rows, which resizes the PTY — single source of truth, no
    // manual ResizeSession calls, no burst timers.
    const fit = () => {
      try {
        fitAddon.fit();
      } catch { /* ignore */ }
    };
    const resizeObserver = new ResizeObserver((entries) => {
      if (disposed) return;
      for (const entry of entries) {
        const { width, height } = entry.contentRect;
        if (width > 0 && height > 0) {
          fit();
          break;
        }
      }
    });
    resizeObserver.observe(container);

    // Belt-and-suspenders: refit once after fonts/layout settle
    const fitTimer = setTimeout(() => {
      if (disposed) return;
      fit();
      // Push the fitted size explicitly. term.onResize only fires when
      // cols/rows CHANGE from xterm's internal default; if the fitted size
      // equals the 80x24 default, the PTY would never be told about it.
      // Always push once so the child's stty/columns match the real pane.
      const t = termRef.current;
      if (t && t.cols && t.rows && sessionId) {
        ResizeSession(sessionId, t.rows, t.cols).catch(() => {});
      }
    }, 50);

    // Apply theme changes at runtime (Global Settings → Appearance)
    const themeObserver = new MutationObserver(() => {
      if (disposed) return;
      const themeObj = xtermThemeFor();
      if (themeObj && termRef.current) {
        termRef.current.options.theme = themeObj;
        termRef.current.refresh(0, termRef.current.rows - 1);
      }
    });
    themeObserver.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["class"],
    });

    // Replay any output that arrived before this terminal mounted — raw,
    // exactly as it was received.
    const buf = outputBuffers.get(sessionId);
    if (buf && buf.length > 0) {
      term.write(buf.join(""));
      term.scrollToBottom();
    }

    // Live output — raw passthrough. xterm parses \r redraws (spinners),
    // ANSI cursor moves, and split escape sequences natively.
    outputHandlers.set(sessionId, (data: string) => {
      const t = termRef.current;
      if (!t) return;
      const isAtBottom = t.buffer.active.viewportY === t.buffer.active.baseY;
      t.write(data);
      // Pin to bottom only while the user is already at the bottom —
      // never yank them back down if they scrolled up to read.
      if (isAtBottom) t.scrollToBottom();
    });

    const disposeInput = term.onData((input) => {
      WriteSession(sessionId, input).catch(() => {});
    });

    const disposeResize = term.onResize(({ cols, rows }) => {
      ResizeSession(sessionId, rows, cols).catch(() => {});
    });

    // Initial focus for the active pane
    if (isActiveRef.current) {
      try {
        term.focus();
      } catch { /* ignore */ }
    }

    return () => {
      disposed = true;
      outputHandlers.delete(sessionId);
      resizeObserver.disconnect();
      themeObserver.disconnect();
      clearTimeout(fitTimer);
      disposeInput.dispose();
      disposeResize.dispose();
      disposeLinks.dispose();
      term.element?.removeEventListener("dblclick", dblclickHandler);
      container.removeEventListener("click", handleFocusClick);
      term.dispose();
      termRef.current = null;
      fitAddonRef.current = null;
    };
  }, [isReady, sessionId]);

  // Refocus the terminal when it becomes the active pane
  useEffect(() => {
    if (!isReady || !isActive) return;
    if (termRef.current) {
      try {
        termRef.current.focus();
      } catch { /* ignore */ }
    }
  }, [isReady, isActive, sessionId]);

  return (
    <div className="relative h-full w-full overflow-hidden">
      <div
        ref={containerRef}
        className="h-full w-full terminal-view focus:outline-none"
        tabIndex={0}
        style={{ background: "var(--terminal-background, #0c0c0c)", minHeight: 0, minWidth: 0 }}
      />
      {/* Scroll controls — top-right, only when the buffer has overflowed */}
      {canScroll && (
        <div className="absolute top-2 right-2 flex flex-col gap-1 select-none z-10">
          <button
            onClick={scrollUp}
            title="Scroll up"
            className="p-1.5 rounded bg-black/60 border border-[var(--border-default)] text-white hover:text-white hover:bg-black/80 cursor-pointer backdrop-blur-sm"
          >
            <IconArrowUp className="size-3.5" />
          </button>
          <button
            onClick={scrollDown}
            title="Scroll to bottom"
            disabled={atBottom}
            className="p-1.5 rounded bg-black/60 border border-[var(--border-default)] text-white hover:text-white hover:bg-black/80 cursor-pointer backdrop-blur-sm disabled:opacity-40 disabled:cursor-default"
          >
            <IconArrowDown className="size-3.5" />
          </button>
        </div>
      )}
    </div>
  );
}