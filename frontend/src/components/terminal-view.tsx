import { useEffect, useRef, useState } from "react";
import { Terminal } from "xterm";
import "xterm/css/xterm.css";
import { FitAddon } from "xterm-addon-fit";
import { WriteSession, ResizeSession, GetHomeDir, OpenInFinder, IsDir } from "../lib/wails";
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
// Backgrounds match the --terminal-background CSS tokens in index.css so the
// terminal blends with the surrounding chrome.
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

const xtermThemes: Record<string, XtermTheme> = {
  zed: {
    background: "#0f1012",
    foreground: "#e3e6ed",
    cursor: "#5b9dff",
    cursorAccent: "#0f1012",
    selectionBackground: "#264f78",
    selectionForeground: "#ffffff",
    selectionInactiveBackground: "#3a3d41",
    black: "#000000",
    red: "#ff5555",
    green: "#50fa7b",
    yellow: "#f1fa8c",
    blue: "#bd93f9",
    magenta: "#ff79c6",
    cyan: "#8be9fd",
    white: "#f8f8f2",
    brightBlack: "#6272a4",
    brightRed: "#ff6e6e",
    brightGreen: "#69ff94",
    brightYellow: "#ffffa5",
    brightBlue: "#d6acff",
    brightMagenta: "#ff92df",
    brightCyan: "#a4ffff",
    brightWhite: "#ffffff",
  },
  dark: {
    background: "#060810",
    foreground: "#dde3f0",
    cursor: "#5b9dff",
    cursorAccent: "#060810",
    selectionBackground: "#264f78",
    selectionForeground: "#ffffff",
    selectionInactiveBackground: "#3a3d41",
    black: "#000000",
    red: "#ff5555",
    green: "#50fa7b",
    yellow: "#f1fa8c",
    blue: "#bd93f9",
    magenta: "#ff79c6",
    cyan: "#8be9fd",
    white: "#f8f8f2",
    brightBlack: "#6272a4",
    brightRed: "#ff6e6e",
    brightGreen: "#69ff94",
    brightYellow: "#ffffa5",
    brightBlue: "#d6acff",
    brightMagenta: "#ff92df",
    brightCyan: "#a4ffff",
    brightWhite: "#ffffff",
  },
  light: {
    background: "#1a1d26",
    foreground: "#dde3f0",
    cursor: "#5b9dff",
    cursorAccent: "#1a1d26",
    selectionBackground: "#264f78",
    selectionForeground: "#ffffff",
    selectionInactiveBackground: "#3a3d41",
    black: "#000000",
    red: "#ff5555",
    green: "#50fa7b",
    yellow: "#f1fa8c",
    blue: "#bd93f9",
    magenta: "#ff79c6",
    cyan: "#8be9fd",
    white: "#f8f8f2",
    brightBlack: "#6272a4",
    brightRed: "#ff6e6e",
    brightGreen: "#69ff94",
    brightYellow: "#ffffa5",
    brightBlue: "#d6acff",
    brightMagenta: "#ff92df",
    brightCyan: "#a4ffff",
    brightWhite: "#ffffff",
  },
  // Basic — clean, minimal, VS Code-style palette
  basic: {
    background: "#0c0c0c",
    foreground: "#cccccc",
    cursor: "#aeafad",
    cursorAccent: "#0c0c0c",
    selectionBackground: "#264f78",
    selectionForeground: "#ffffff",
    selectionInactiveBackground: "#3a3d41",
    black: "#000000",
    red: "#cd3131",
    green: "#0dbc79",
    yellow: "#e5e510",
    blue: "#2472c8",
    magenta: "#bc3fbc",
    cyan: "#11a8cd",
    white: "#e5e5e5",
    brightBlack: "#666666",
    brightRed: "#f14c4c",
    brightGreen: "#23d18b",
    brightYellow: "#f5f543",
    brightBlue: "#3b8eea",
    brightMagenta: "#d670d6",
    brightCyan: "#29b8db",
    brightWhite: "#e5e5e5",
  },
  // Homebrew — warm amber-toned palette
  homebrew: {
    background: "#12121e",
    foreground: "#d8d0c0",
    cursor: "#f5a623",
    cursorAccent: "#12121e",
    selectionBackground: "#f5a623",
    selectionForeground: "#12121e",
    selectionInactiveBackground: "#3a3a5c",
    black: "#1a1a2e",
    red: "#ef4444",
    green: "#4ade80",
    yellow: "#f5a623",
    blue: "#60a5fa",
    magenta: "#c084fc",
    cyan: "#22d3ee",
    white: "#e0d8c0",
    brightBlack: "#5a5a7a",
    brightRed: "#f87171",
    brightGreen: "#4ade80",
    brightYellow: "#fbbf24",
    brightBlue: "#93c5fd",
    brightMagenta: "#d8b4fe",
    brightCyan: "#67e8f9",
    brightWhite: "#f5f0e0",
  },
};

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

  useEffect(() => {
    isActiveRef.current = isActive;
  }, [isActive]);

  useEffect(() => {
    if (!isReady) return;
    ensureGlobalListener();
    const container = containerRef.current;
    if (!container) return;

    let disposed = false;

    // Resolve the app theme name from the root element's class list
    // (the app applies theme names like "zed" | "dark" | "basic" ...).
    const resolveThemeName = (): string => {
      const cls = document.documentElement.className;
      if (cls && xtermThemes[cls]) return cls;
      return "zed";
    };

    const term = new Terminal({
      cursorBlink: true,
      cursorStyle: "block",
      fontSize: 13,
      fontFamily: "'JetBrains Mono', 'Fira Code', Menlo, Monaco, monospace",
      lineHeight: 1.2,
      theme: xtermThemes[resolveThemeName()],
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
        navigator.clipboard.readText().then((text) => {
          if (text) term.paste(text);
        }).catch(() => {});
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
            openInBrowser(url);
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
      const themeObj = xtermThemes[resolveThemeName()];
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
    <div className="h-full w-full overflow-hidden">
      <div
        ref={containerRef}
        className="h-full w-full terminal-view focus:outline-none"
        tabIndex={0}
        style={{ background: "var(--terminal-background, #0c0c0c)", minHeight: 0, minWidth: 0 }}
      />
    </div>
  );
}