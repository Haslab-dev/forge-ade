import { useEffect, useRef, useState } from "react";
import { Terminal } from "xterm";
import "xterm/css/xterm.css";
import { FitAddon } from "xterm-addon-fit";
import {
  WriteSession,
  ResizeSession,
  GetHomeDir,
  OpenInFinder,
  IsDir,
  BrowserOpenURL,
  GetClipboardFiles,
  ClipboardGetText,
  ClipboardSetText,
  EventsOn,
} from "../lib/wails";
import { getTerminalSettings, TERMINAL_SETTINGS_EVENT } from "../lib/terminalSettings";
import { globalOpenFile } from "../panels/editor";
import { globalOpenSideFile } from "../stores/workspaceStore";
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
      if (buf.length > 500) buf.splice(0, buf.length - 500);
      const handler = outputHandlers.get(payload.id);
      if (handler) handler(payload.data);
    }
  });
  EventsOn("session:closed", (payload: any) => {
    if (payload && payload.id) {
      // Drop only the live buffer — the registered handler stays so a
      // RESTARTED session (new process, same id) keeps streaming into the
      // mounted terminal without a remount. Handlers are removed when their
      // component unmounts.
      outputBuffers.delete(payload.id);
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
  /**
   * Provides persisted output (Terminal Session history) to replay when this
   * terminal mounts with nothing buffered live — e.g. after an app restart.
   * Called at most once per mount.
   */
  historyProvider?: () => Promise<string | null>;
}

export function TerminalView({ sessionId, isActive = true, historyProvider }: TerminalViewProps) {
  const [isReady] = useState(true);
  const containerRef = useRef<HTMLDivElement>(null);
  const termRef = useRef<Terminal | null>(null);
  const fitAddonRef = useRef<FitAddon | null>(null);
  const isActiveRef = useRef(isActive);
  useEffect(() => {
    isActiveRef.current = isActive;
  }, [isActive]);

  useEffect(() => {
    if (!isReady) return;
    ensureGlobalListener();
    const container = containerRef.current;
    if (!container) return;

    let disposed = false;

    const settings = getTerminalSettings();
    const term = new Terminal({
      cursorBlink: settings.cursorBlink,
      cursorStyle: settings.cursorStyle,
      fontSize: settings.fontSize,
      fontFamily: settings.fontFamily,
      lineHeight: 1.2,
      theme: xtermThemeFor(),
      allowTransparency: false,
      scrollback: settings.scrollback,
    });

    const fitAddon = new FitAddon();
    term.loadAddon(fitAddon);

    term.attachCustomKeyEventHandler((e: KeyboardEvent) => {
      // Ignore keyup events so we don't double-trigger or cancel key releases
      if (e.type !== "keydown") return true;

      // Cmd/Ctrl+K clears the terminal
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        term.clear();
        return false;
      }

      // Cmd+C / Ctrl+Shift+C: copy selection
      const isCopy =
        (e.metaKey && !e.ctrlKey && e.key.toLowerCase() === "c") ||
        (e.ctrlKey && e.shiftKey && e.key.toLowerCase() === "c");
      if (isCopy) {
        const sel = term.getSelection();
        if (sel) {
          e.preventDefault();
          ClipboardSetText(sel).catch(() => {});
          navigator.clipboard?.writeText?.(sel)?.catch?.(() => {});
          return false;
        }
        if (e.metaKey) {
          return false;
        }
      }

      // Cmd+V / Ctrl+Shift+V / Ctrl+V: paste text or files
      const isPaste =
        (e.metaKey && !e.ctrlKey && e.key.toLowerCase() === "v") ||
        (e.ctrlKey && e.shiftKey && e.key.toLowerCase() === "v");
      if (isPaste) {
        e.preventDefault();
        (async () => {
          // 1. Files copied from Finder / file manager
          try {
            const files = await GetClipboardFiles();
            if (files && files.length > 0) {
              const quoted = files
                .map((p) => (p.includes(" ") ? `'${p.replace(/'/g, "'\\''")}'` : p))
                .join(" ") + " ";
              term.paste(quoted);
              return;
            }
          } catch {}

          // 2. Native text from Wails runtime (NSPasteboard on macOS)
          try {
            const text = await ClipboardGetText();
            if (text) {
              term.paste(text);
              return;
            }
          } catch {}

          // 3. Fallback to web clipboard
          try {
            const text = await navigator.clipboard.readText();
            if (text) {
              term.paste(text);
            }
          } catch {}
        })();
        return false;
      }

      return true;
    });

    termRef.current = term;
    fitAddonRef.current = fitAddon;

    term.open(container);

    // Register a link provider so URLs in output (e.g. dev-server "Server
    // running at http://localhost:3000") become clickable links: blue
    // Register link provider for Web URLs and File Paths in terminal output
    const linkProvider = {
      provideLinks(bufferLineNumber: number, callback: (links: any[] | undefined) => void) {
        const t = termRef.current;
        if (!t) { callback(undefined); return; }
        const line = t.buffer.active.getLine(bufferLineNumber - 1);
        if (!line) { callback(undefined); return; }
        const lineText = line.translateToString(true);

        const links: any[] = [];
        
        // 1. Web URLs
        const url = findUrl(lineText);
        if (url) {
          URL_REGEX.lastIndex = 0;
          const m = URL_REGEX.exec(lineText);
          if (m && m[0]) {
            const rawMatch = m[0];
            const idx = m.index;
            links.push({
              range: {
                start: { x: idx + 1, y: bufferLineNumber },
                end: { x: idx + rawMatch.length + 1, y: bufferLineNumber },
              },
              text: rawMatch,
              decorations: { pointerCursor: true, underline: true },
              activate: () => {
                let host = "";
                try { host = new URL(url).hostname; } catch { host = ""; }
                if (/^(localhost|127\.0\.0\.1|0\.0\.0\.0|\[::1\])$/i.test(host)) {
                  openInBrowser(url);
                } else {
                  BrowserOpenURL(url);
                }
              },
            });
          }
        }

        // 2. File Paths e.g. /abs/path/file.ts, relative/file.js:10, samples/sample.ts
        const filePathRegex = /(?:\/|~|\.\/|\.\.\/|[\w\-.]+\/)(?:[\w\-.]+\/)*[\w\-.]+\.(?:[a-zA-Z0-9]+)(?::\d+)*(?::\d+)*/gi;
        let fileMatch: RegExpExecArray | null;
        while ((fileMatch = filePathRegex.exec(lineText)) !== null) {
          const rawMatch = fileMatch[0];
          const idx = fileMatch.index;
          // Avoid duplicate bounds if overlapping web url
          if (links.some((l) => idx >= l.range.start.x - 1 && idx < l.range.end.x - 1)) continue;

          links.push({
            range: {
              start: { x: idx + 1, y: bufferLineNumber },
              end: { x: idx + rawMatch.length + 1, y: bufferLineNumber },
            },
            text: rawMatch,
            decorations: { pointerCursor: true, underline: true },
            activate: () => {
              const cleanPath = rawMatch.replace(/^['"]|['"]$/g, "");
              globalOpenSideFile(cleanPath);
            },
          });
        }

        callback(links.length > 0 ? links : undefined);
      },
    };
    const disposeLinks = term.registerLinkProvider(linkProvider as any);

    // Double-click a path → open it in the right sidebar / Finder
    const dblclickHandler = () => {
      const selection = term.getSelection().trim();
      if (!selection) return;
      const resolved = selection.startsWith("~/") ? (homeDir || "") + selection.slice(1) : selection;
      const cleanPath = resolved.replace(/:(\d+)(:\d+)?$/, "").replace(/^['"]|['"]$/g, "").trim();
      if (!cleanPath) return;
      IsDir(cleanPath).then((isDir) => {
        if (isDir) OpenInFinder(cleanPath).catch(() => {});
        else globalOpenSideFile(cleanPath);
      }).catch(() => globalOpenSideFile(cleanPath));
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

    // Apply terminal settings changes at runtime (Settings → Terminal). Font
    // metrics change the fit, so refit after applying.
    const settingsHandler = () => {
      if (disposed || !termRef.current) return;
      const s = getTerminalSettings();
      termRef.current.options.fontSize = s.fontSize;
      termRef.current.options.fontFamily = s.fontFamily;
      termRef.current.options.cursorStyle = s.cursorStyle;
      termRef.current.options.cursorBlink = s.cursorBlink;
      try {
        fitAddon.fit();
      } catch { /* ignore */ }
    };
    window.addEventListener(TERMINAL_SETTINGS_EVENT, settingsHandler);

    // Replay any output that arrived before this terminal mounted — raw,
    // exactly as it was received. With nothing buffered live (fresh app
    // start on an old session), fall back to the persisted history provider.
    const buf = outputBuffers.get(sessionId);
    if (buf && buf.length > 0) {
      term.write(buf.join(""));
      term.scrollToBottom();
    } else if (historyProvider) {
      historyProvider()
        .then((history) => {
          if (disposed || !history) return;
          // A live listener may have raced us — live chunks are appended
          // after the history write, so ordering stays correct.
          term.write(history);
          term.scrollToBottom();
        })
        .catch(() => {});
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

    // Drag & drop file / image support:
    // Pastes quoted absolute path into shell, exactly like Antigravity / macOS Terminal
    const handleForgeDrop = (ev: Event) => {
      const detail = (ev as CustomEvent)?.detail;
      if (!detail?.files || detail.files.length === 0) return;
      const el = containerRef.current;
      if (!el) return;
      const rect = el.getBoundingClientRect();
      const inside =
        detail.x >= rect.left &&
        detail.x <= rect.right &&
        detail.y >= rect.top &&
        detail.y <= rect.bottom;
      if (!inside && !isActiveRef.current) return;

      ev.preventDefault();
      ev.stopPropagation();
      const quoted = detail.files
        .map((p: string) => (p.includes(" ") ? `'${p.replace(/'/g, "'\\''")}'` : p))
        .join(" ") + " ";
      term.paste(quoted);
      try {
        term.focus();
      } catch {}
    };

    const handleNativeDragOver = (e: DragEvent) => {
      e.preventDefault();
      if (e.dataTransfer) {
        e.dataTransfer.dropEffect = "copy";
      }
    };

    const handleNativeDrop = (e: DragEvent) => {
      e.preventDefault();
      e.stopPropagation();
      if (!e.dataTransfer) return;
      const text = e.dataTransfer.getData("text/plain");
      if (text) {
        term.paste(text);
        return;
      }
      if (e.dataTransfer.files && e.dataTransfer.files.length > 0) {
        const paths: string[] = [];
        for (let i = 0; i < e.dataTransfer.files.length; i++) {
          const f = e.dataTransfer.files[i] as any;
          if (f.path) paths.push(f.path);
        }
        if (paths.length > 0) {
          const quoted = paths
            .map((p) => (p.includes(" ") ? `'${p.replace(/'/g, "'\\''")}'` : p))
            .join(" ") + " ";
          term.paste(quoted);
        }
      }
    };

    window.addEventListener("forge:file-drop", handleForgeDrop);
    container.addEventListener("dragover", handleNativeDragOver);
    container.addEventListener("drop", handleNativeDrop);

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
      window.removeEventListener(TERMINAL_SETTINGS_EVENT, settingsHandler);
      window.removeEventListener("forge:file-drop", handleForgeDrop);
      container.removeEventListener("dragover", handleNativeDragOver);
      container.removeEventListener("drop", handleNativeDrop);
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
        role="application"
        aria-label="Terminal"
        data-file-drop-target="true"
        className="h-full w-full terminal-view focus-visible:outline focus-visible:outline-1 focus-visible:outline-[var(--accent-primary,#5b9dff)]"
        tabIndex={0}
        style={{ background: "var(--terminal-background, #0c0c0c)", minHeight: 0, minWidth: 0 }}
      />
    </div>
  );
}