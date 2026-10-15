import { useEffect, useRef, useState, useCallback } from "react";
import { Terminal } from "xterm";
import "xterm/css/xterm.css";
import { FitAddon } from "xterm-addon-fit";
import { CanvasAddon } from "@xterm/addon-canvas";
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
import { getTerminalSettings, setTerminalSettings, TERMINAL_SETTINGS_EVENT } from "../lib/terminalSettings";
import { globalOpenFile } from "../lib/editorGlobals";
import { globalOpenSideFile } from "../stores/workspaceStore";
import { openInBrowser } from "../lib/browser";

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
const URL_REGEX = /\b(?:https?:\/\/|www\.|localhost(?::\d+)?|127\.\d+\.\d+\.\d+(?::\d+)?|0\.0\.0\.0(?::\d+)?|\[::1\](?::\d+)?)[^\s<>"')\]]*/gi;

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
// TerminalView registers a clear routine per mounted session so a RESTART
// (same id, new process) can wipe the old screen and refit — otherwise the
// new PTY (spawned at the 80x24 default) renders into a stale, wrongly-sized
// screen and the TUI looks broken / doesn't fill the pane.
const clearHandlers = new Map<string, () => void>();
let globalInitialized = false;

/** Reset a session's terminal for a restarted process: wipe screen +
    scrollback, refit to the pane, and push the fitted size to the new PTY. */
export function ResetTerminalView(sessionId: string) {
  outputBuffers.delete(sessionId);
  clearHandlers.get(sessionId)?.();
}

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
      let totalBytes = buf.reduce((acc, c) => acc + c.length, 0);
      while (buf.length > 50 || totalBytes > 256 * 1024) {
        const removed = buf.shift();
        if (removed) totalBytes -= removed.length;
      }
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
  EventsOn("agentsession:deleted", (payload: any) => {
    const id = typeof payload === "string" ? payload : payload?.id;
    if (id) {
      outputBuffers.delete(id);
      outputHandlers.delete(id);
      clearHandlers.delete(id);
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
    // Selection derives from the accent token so it tracks the active theme.
    selectionBackground: mixHex(p.accent, p.bg, 0.35),
    selectionForeground: p.fg,
    selectionInactiveBackground: mixHex(p.accent, p.bg, 0.75),
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
   * False when the session record exists but has no live process (e.g. a
   * CLI session restored as Terminated after an app restart). Skips all
   * Write/Resize binding calls — they error-spam the backend otherwise.
   */
  ptyAlive?: boolean;
  /**
   * Provides persisted output (Terminal Session history) to replay when this
   * terminal mounts with nothing buffered live — e.g. after an app restart.
   * Called at most once per mount.
   */
  historyProvider?: () => Promise<string | null>;
}

export function TerminalView({ sessionId, isActive = true, ptyAlive = true, historyProvider }: TerminalViewProps) {
  const [isReady] = useState(true);
  const containerRef = useRef<HTMLDivElement>(null);
  const termRef = useRef<Terminal | null>(null);
  const fitAddonRef = useRef<FitAddon | null>(null);
  const isActiveRef = useRef(isActive);
  useEffect(() => {
    isActiveRef.current = isActive;
  }, [isActive]);
  const ptyAliveRef = useRef(ptyAlive);
  useEffect(() => {
    ptyAliveRef.current = ptyAlive;
  }, [ptyAlive]);
  const userScrolledUpRef = useRef(false);

  const [pendingPaste, setPendingPaste] = useState<{ text: string; lineCount: number } | null>(null);
  const [dontAskAgain, setDontAskAgain] = useState(false);
  const pendingPasteRef = useRef(pendingPaste);
  pendingPasteRef.current = pendingPaste;
  const handlePasteTextRef = useRef<(text: string) => void>(() => {});

  const executePaste = useCallback((text: string, mode: 'bracketed' | 'stringify' | 'singleLine', saveAsDefault?: boolean) => {
    if (saveAsDefault) {
      const s = getTerminalSettings();
      setTerminalSettings({ ...s, multilinePasteMode: mode });
    }
    setPendingPaste(null);
    const term = termRef.current;
    if (!term) return;

    if (mode === 'stringify') {
      const stringified = text.replace(/\r\n/g, "\\n").replace(/[\n\r]/g, "\\n");
      term.paste(stringified);
    } else if (mode === 'singleLine') {
      const single = text.replace(/\r\n/g, " ").replace(/[\n\r]/g, " ");
      term.paste(single);
    } else {
      const normalized = text.replace(/\r\n/g, "\n").replace(/\r/g, "\n");
      if (sessionId && ptyAliveRef.current) {
        WriteSession(sessionId, `\x1b[200~${normalized}\x1b[201~`).catch(() => {});
      } else {
        term.paste(normalized);
      }
    }
    try {
      term.focus();
    } catch {}
  }, [sessionId]);

  const handlePasteText = useCallback((text: string) => {
    if (!text) return;
    const hasNewline = text.includes('\n') || text.includes('\r');
    if (!hasNewline) {
      termRef.current?.paste(text);
      return;
    }

    const s = getTerminalSettings();
    const mode = s.multilinePasteMode || 'ask';
    if (mode !== 'ask') {
      executePaste(text, mode);
      return;
    }

    const lines = text.split(/\r\n|\r|\n/);
    setPendingPaste({ text, lineCount: lines.length });
    setDontAskAgain(false);
  }, [executePaste]);

  useEffect(() => {
    handlePasteTextRef.current = handlePasteText;
  }, [handlePasteText]);

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

    try {
      const canvasAddon = new CanvasAddon();
      term.loadAddon(canvasAddon);
    } catch (err) {
      console.warn("[terminal] CanvasAddon load error, falling back to DOM renderer:", err);
    }

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
              handlePasteTextRef.current(text);
              return;
            }
          } catch {}

          // 3. Fallback to web clipboard
          try {
            const text = await navigator.clipboard.readText();
            if (text) {
              handlePasteTextRef.current(text);
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
        URL_REGEX.lastIndex = 0;
        let urlMatch: RegExpExecArray | null;
        while ((urlMatch = URL_REGEX.exec(lineText)) !== null) {
          const rawMatch = urlMatch[0];
          const idx = urlMatch.index;
          let targetUrl = rawMatch.replace(/[),;]+$/, "");
          if (/^0\.0\.0\.0(:\d+)?/i.test(targetUrl)) {
            targetUrl = targetUrl.replace(/^0\.0\.0\.0/i, "localhost");
          }
          if (/^https?:\/\/0\.0\.0\.0(:\d+)?/i.test(targetUrl)) {
            targetUrl = targetUrl.replace(/^(https?:\/\/)0\.0\.0\.0/i, "$1localhost");
          }
          if (/^www\./i.test(targetUrl)) targetUrl = "https://" + targetUrl;
          else if (!/^https?:\/\//i.test(targetUrl)) targetUrl = "http://" + targetUrl;

          links.push({
            range: {
              start: { x: idx + 1, y: bufferLineNumber },
              end: { x: idx + rawMatch.length + 1, y: bufferLineNumber },
            },
            text: rawMatch,
            decorations: { pointerCursor: true, underline: true },
            activate: () => {
              openInBrowser(targetUrl);
            },
          });
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

    const scrollToBottomIfPinned = () => {
      if (disposed) return;
      const t = termRef.current;
      if (!t) return;
      if (!userScrolledUpRef.current) {
        t.scrollToBottom();
        requestAnimationFrame(() => {
          if (!disposed && !userScrolledUpRef.current && termRef.current) {
            termRef.current.scrollToBottom();
          }
        });
        setTimeout(() => {
          if (!disposed && !userScrolledUpRef.current && termRef.current) {
            termRef.current.scrollToBottom();
          }
        }, 60);
      }
    };

    const handleWheel = () => {
      setTimeout(() => {
        const t = termRef.current;
        if (!t) return;
        userScrolledUpRef.current = t.buffer.active.viewportY < t.buffer.active.baseY - 1;
      }, 30);
    };
    container.addEventListener("wheel", handleWheel, { passive: true });

    const resizeObserver = new ResizeObserver((entries) => {
      if (disposed) return;
      for (const entry of entries) {
        const { width, height } = entry.contentRect;
        if (width > 0 && height > 0) {
          fit();
          scrollToBottomIfPinned();
          break;
        }
      }
    });
    resizeObserver.observe(container);

    // Belt-and-suspenders: refit once after fonts/layout settle
    const fitTimer = setTimeout(() => {
      if (disposed) return;
      fit();
      scrollToBottomIfPinned();
      // Push the fitted size explicitly. term.onResize only fires when
      // cols/rows CHANGE from xterm's internal default; if the fitted size
      // equals the 80x24 default, the PTY would never be told about it.
      // Always push once so the child's stty/columns match the real pane.
      const t = termRef.current;
      if (t && t.cols && t.rows && sessionId && ptyAliveRef.current) {
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

    // Live output — batched via requestAnimationFrame to avoid layout thrashing
    // during fast stream / spinner ANSI rewrites.
    let pendingWrite = "";
    let writeRafId: number | null = null;

    const flushWrite = () => {
      writeRafId = null;
      if (!pendingWrite) return;
      const t = termRef.current;
      if (!t) {
        pendingWrite = "";
        return;
      }
      const chunk = pendingWrite;
      pendingWrite = "";
      const isAtBottom = !userScrolledUpRef.current || t.buffer.active.viewportY >= t.buffer.active.baseY - 1;
      t.write(chunk, () => {
        if (isAtBottom && !userScrolledUpRef.current && termRef.current) {
          termRef.current.scrollToBottom();
        }
      });
    };

    outputHandlers.set(sessionId, (data: string) => {
      pendingWrite += data;
      if (writeRafId === null) {
        writeRafId = requestAnimationFrame(flushWrite);
      }
    });

    const disposeScroll = term.onScroll(() => {
      const t = termRef.current;
      if (t && t.buffer.active.viewportY >= t.buffer.active.baseY - 1) {
        userScrolledUpRef.current = false;
      }
    });

    const disposeInput = term.onData((input) => {
      userScrolledUpRef.current = false;
      if (termRef.current) termRef.current.scrollToBottom();
      if (!ptyAliveRef.current) return;
      WriteSession(sessionId, input).catch(() => {});
    });

    const disposeResize = term.onResize(({ cols, rows }) => {
      if (!ptyAliveRef.current) return;
      ResizeSession(sessionId, rows, cols).catch(() => {});
    });

    // Restart routine: full reset (RIS) + scrollback wipe, then refit so the
    // new PTY gets the pane's real size instead of its 80x24 spawn default.
    clearHandlers.set(sessionId, () => {
      pendingWrite = "";
      if (writeRafId !== null) {
        cancelAnimationFrame(writeRafId);
        writeRafId = null;
      }
      const t = termRef.current;
      const fa = fitAddonRef.current;
      if (!t) return;
      t.write("\u001bc");   // RIS — reset screen and modes
      t.write("\u001b[3J"); // ED 3 — clear scrollback
      try {
        fa?.fit();
      } catch { /* ignore */ }
      if (t.cols && t.rows) {
        ResizeSession(sessionId, t.rows, t.cols).catch(() => {});
      }
      t.scrollToBottom();
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
        handlePasteTextRef.current(text);
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

    const handleNativePaste = (e: ClipboardEvent) => {
      if (pendingPasteRef.current) return;
      const text = e.clipboardData?.getData("text/plain");
      if (text) {
        e.preventDefault();
        e.stopPropagation();
        handlePasteTextRef.current(text);
      }
    };

    window.addEventListener("forge:file-drop", handleForgeDrop);
    container.addEventListener("dragover", handleNativeDragOver);
    container.addEventListener("drop", handleNativeDrop);
    container.addEventListener("paste", handleNativePaste);

    // Initial focus for the active pane
    if (isActiveRef.current) {
      try {
        term.focus();
      } catch { /* ignore */ }
    }

    return () => {
      disposed = true;
      if (writeRafId !== null) {
        cancelAnimationFrame(writeRafId);
        writeRafId = null;
      }
      pendingWrite = "";
      clearHandlers.delete(sessionId);
      outputHandlers.delete(sessionId);
      resizeObserver.disconnect();
      themeObserver.disconnect();
      window.removeEventListener(TERMINAL_SETTINGS_EVENT, settingsHandler);
      window.removeEventListener("forge:file-drop", handleForgeDrop);
      container.removeEventListener("dragover", handleNativeDragOver);
      container.removeEventListener("drop", handleNativeDrop);
      container.removeEventListener("paste", handleNativePaste);
      clearTimeout(fitTimer);
      container.removeEventListener("wheel", handleWheel);
      disposeScroll.dispose();
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

  // Refocus the terminal, manage cursor blinking, and scroll to bottom when it becomes the active pane
  useEffect(() => {
    if (!isReady || !isActive) {
      if (termRef.current) {
        try {
          termRef.current.options.cursorBlink = false;
        } catch { /* ignore */ }
      }
      return;
    }
    if (termRef.current) {
      try {
        termRef.current.options.cursorBlink = getTerminalSettings().cursorBlink;
        termRef.current.focus();
        if (!userScrolledUpRef.current) {
          termRef.current.scrollToBottom();
          requestAnimationFrame(() => {
            if (termRef.current && !userScrolledUpRef.current) {
              termRef.current.scrollToBottom();
            }
          });
          setTimeout(() => {
            if (termRef.current && !userScrolledUpRef.current) {
              termRef.current.scrollToBottom();
            }
          }, 60);
        }
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
      {pendingPaste && (
        <div
          role="dialog"
          aria-modal="true"
          aria-label="Multi-line Paste Confirmation"
          className="absolute inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4 animate-in fade-in duration-100"
          onKeyDown={(e) => {
            if (e.key === "Escape") {
              e.preventDefault();
              e.stopPropagation();
              setPendingPaste(null);
              termRef.current?.focus();
            } else if (e.key === "Enter") {
              e.preventDefault();
              e.stopPropagation();
              executePaste(pendingPaste.text, "bracketed", dontAskAgain);
            } else if (e.key.toLowerCase() === "s" && !e.metaKey && !e.ctrlKey) {
              e.preventDefault();
              e.stopPropagation();
              executePaste(pendingPaste.text, "stringify", dontAskAgain);
            } else if (e.key.toLowerCase() === "l" && !e.metaKey && !e.ctrlKey) {
              e.preventDefault();
              e.stopPropagation();
              executePaste(pendingPaste.text, "singleLine", dontAskAgain);
            }
          }}
          tabIndex={0}
          ref={(el) => el?.focus()}
        >
          <div
            className="bg-panel border border-border rounded-lg shadow-2xl max-w-lg w-full p-4 flex flex-col gap-3 text-fg-primary"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center justify-between border-b border-border/50 pb-2">
              <div className="flex items-center gap-2">
                <span className="font-semibold text-ui-base">Paste Multi-Line Content</span>
                <span className="text-ui-xs bg-surface text-fg-secondary px-2 py-0.5 rounded-full border border-border/40 font-mono">
                  {pendingPaste.lineCount} lines • {pendingPaste.text.length} chars
                </span>
              </div>
              <button
                type="button"
                onClick={() => {
                  setPendingPaste(null);
                  termRef.current?.focus();
                }}
                className="text-fg-tertiary hover:text-fg-primary text-ui-sm cursor-pointer p-1"
                title="Cancel (Esc)"
              >
                ✕
              </button>
            </div>

            <p className="text-ui-xs text-fg-secondary leading-relaxed">
              Text contains newlines. In shells (local or SSH), raw newlines can immediately execute commands as <kbd className="px-1 py-0.5 rounded bg-surface border border-border/60 text-ui-xs font-mono">Enter</kbd>. Choose how to paste safely:
            </p>

            <div className="bg-app border border-border/60 rounded p-2.5 max-h-36 overflow-y-auto font-mono text-ui-xs text-fg-secondary whitespace-pre select-text leading-relaxed">
              {pendingPaste.text.slice(0, 1500)}
              {pendingPaste.text.length > 1500 ? "\n... (truncated preview)" : ""}
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-3 gap-2 pt-1">
              <button
                type="button"
                onClick={() => executePaste(pendingPaste.text, "bracketed", dontAskAgain)}
                className="flex flex-col items-center justify-center p-2.5 rounded-md bg-accent-primary text-white hover:brightness-110 cursor-pointer text-center transition-all group"
              >
                <div className="flex items-center gap-1.5 font-medium text-ui-sm">
                  <span>Multiline</span>
                  <kbd className="text-ui-xs opacity-80 px-1 rounded bg-black/20 font-mono">↵</kbd>
                </div>
                <span className="text-ui-xs opacity-80 mt-0.5">Safe (No Enter)</span>
              </button>

              <button
                type="button"
                onClick={() => executePaste(pendingPaste.text, "stringify", dontAskAgain)}
                className="flex flex-col items-center justify-center p-2.5 rounded-md bg-surface hover:bg-surface-hover border border-border text-fg-primary cursor-pointer text-center transition-all group"
              >
                <div className="flex items-center gap-1.5 font-medium text-ui-sm">
                  <span>Stringify</span>
                  <kbd className="text-ui-xs text-fg-tertiary px-1 rounded bg-panel border border-border/40 font-mono">S</kbd>
                </div>
                <span className="text-ui-xs text-fg-secondary mt-0.5">Replace \n as text</span>
              </button>

              <button
                type="button"
                onClick={() => executePaste(pendingPaste.text, "singleLine", dontAskAgain)}
                className="flex flex-col items-center justify-center p-2.5 rounded-md bg-surface hover:bg-surface-hover border border-border text-fg-primary cursor-pointer text-center transition-all group"
              >
                <div className="flex items-center gap-1.5 font-medium text-ui-sm">
                  <span>Single Line</span>
                  <kbd className="text-ui-xs text-fg-tertiary px-1 rounded bg-panel border border-border/40 font-mono">L</kbd>
                </div>
                <span className="text-ui-xs text-fg-secondary mt-0.5">Join with space</span>
              </button>
            </div>

            <div className="flex items-center justify-between pt-2 border-t border-border/50 text-ui-xs text-fg-secondary">
              <label className="flex items-center gap-2 cursor-pointer select-none">
                <input
                  type="checkbox"
                  checked={dontAskAgain}
                  onChange={(e) => setDontAskAgain(e.target.checked)}
                  className="rounded border-border accent-accent-primary cursor-pointer"
                />
                <span>Don't ask again (remember choice)</span>
              </label>
              <button
                type="button"
                onClick={() => {
                  setPendingPaste(null);
                  termRef.current?.focus();
                }}
                className="text-fg-tertiary hover:text-fg-primary cursor-pointer px-2 py-1 rounded"
              >
                Cancel <span className="opacity-60">(Esc)</span>
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}