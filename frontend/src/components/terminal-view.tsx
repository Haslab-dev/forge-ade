import { useEffect, useRef, useState } from "react";
import { Terminal } from "xterm";
import "xterm/css/xterm.css";
import { FitAddon } from "xterm-addon-fit";
import { WriteSession, ResizeSession, GetHomeDir, OpenInFinder, IsDir } from "../lib/wails";
import { EventsOn } from "../lib/wails";
import { globalOpenFile } from "../panels/editor";

// Inject terminal link and padding styles once
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

type OutputHandler = (data: string) => void;
const outputHandlers = new Map<string, OutputHandler>();
// Buffer output per session so terminals created after output starts or
// remounted keep full data.
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

const theme = {
  background: "#0f1012",
  foreground: "#e3e6ed",
  cursor: "#5b9dff",
  cursorAccent: "#0f1012",
  selectionBackground: "#5b9dff33",
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
};

// No WASM initialization needed for xterm.js

interface TerminalViewProps {
  sessionId: string;
  isActive?: boolean;
}

export function TerminalView({ sessionId, isActive = true }: TerminalViewProps) {
  const [isReady, setIsReady] = useState(true); // Always ready for xterm.js
  const containerRef = useRef<HTMLDivElement>(null);
  const termRef = useRef<Terminal | null>(null);
  const fitAddonRef = useRef<FitAddon | null>(null);
  const roRef = useRef<ResizeObserver | null>(null);
  const ioRef = useRef<IntersectionObserver | null>(null);
  const termIdRef = useRef<string | null>(null);

  const doFit = () => {
    if (
      containerRef.current &&
      containerRef.current.clientWidth > 0 &&
      fitAddonRef.current
    ) {
      try {
        fitAddonRef.current.fit();
      } catch { /* ignore */ }
    }
  };

  useEffect(() => {
    if (isReady && isActive) {
      doFit();
      const raf = requestAnimationFrame(doFit);
      const t1 = setTimeout(doFit, 50);
      const t2 = setTimeout(doFit, 150);
      const t3 = setTimeout(doFit, 300);
      const t4 = setTimeout(doFit, 600);
      const t5 = setTimeout(doFit, 1200);
      // Refit once web fonts finish loading — char metrics affect fit().
      let fontsCanceled = false;
      if (document.fonts?.ready) {
        document.fonts.ready.then(() => {
          if (!fontsCanceled) doFit();
        }).catch(() => {});
      }
      return () => {
        fontsCanceled = true;
        cancelAnimationFrame(raf);
        clearTimeout(t1);
        clearTimeout(t2);
        clearTimeout(t3);
        clearTimeout(t4);
        clearTimeout(t5);
      };
    }
  }, [isReady, isActive, sessionId]);

  useEffect(() => {
    if (!isReady) return;
    ensureGlobalListener();
    if (!containerRef.current) return;

    const term = new Terminal({
      cursorBlink: true,
      cursorStyle: "bar",
      fontSize: 13,
      fontFamily: "'JetBrains Mono', 'Fira Code', Menlo, Monaco, monospace",
      theme,
      allowTransparency: false,
      scrollback: 10000,
    });

    const fitAddon = new FitAddon();
    term.loadAddon(fitAddon);

    term.attachCustomKeyEventHandler((e: KeyboardEvent) => {
      if (e.type === "keydown" && (e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        term.clear();
        return false; // block default key handling in xterm
      }
      if (e.metaKey && !e.ctrlKey && e.key === "c") {
        e.preventDefault();
        const sel = term.getSelection();
        if (sel) navigator.clipboard.writeText(sel).catch(() => {});
        return false; // block default key handling in xterm
      }
      if (e.metaKey && !e.ctrlKey && e.key === "v") {
        e.preventDefault();
        navigator.clipboard.readText().then((text) => {
          if (text) term.paste(text);
        }).catch(() => {});
        return false; // block default key handling in xterm
      }
      return true; // allow other keys (typing) to be processed by xterm
    });

    termRef.current = term;
    fitAddonRef.current = fitAddon;
    termIdRef.current = sessionId;

    term.open(containerRef.current);

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

    const handleFocusClick = () => {
      try {
        term.focus();
        const ta = containerRef.current?.querySelector("textarea");
        if (ta) {
          ta.focus({ preventScroll: true });
        }
      } catch { /* ignore */ }
    };
    containerRef.current?.addEventListener("click", handleFocusClick);

    const handleFocusEvent = (e: FocusEvent) => {
      const ta = containerRef.current?.querySelector("textarea");
      if (ta && e.target !== ta) {
        try {
          ta.focus({ preventScroll: true });
        } catch { /* ignore */ }
      }
    };
    containerRef.current?.addEventListener("focus", handleFocusEvent);

    doFit();
    try {
      term.focus();
      const ta = containerRef.current?.querySelector("textarea");
      if (ta) {
        ta.focus({ preventScroll: true });
      }
    } catch { /* ignore */ }

    requestAnimationFrame(doFit);
    setTimeout(doFit, 50);
    setTimeout(doFit, 150);

    const buf = outputBuffers.get(sessionId);
    if (buf && buf.length > 0) {
      term.write(buf.join(""));
      term.scrollToBottom();
    }

    outputHandlers.set(sessionId, (data: string) => {
      const isAtBottom = term.buffer.active.viewportY === term.buffer.active.baseY;
      term.write(data);
      if (isAtBottom) {
        term.scrollToBottom();
      }
    });

    const disposeInput = term.onData((input) => {
      WriteSession(sessionId, input).catch(() => {});
    });

    // Keep the PTY size in lockstep with xterm's real rendered geometry.
    // xterm fires this whenever fit() changes its cols/rows — the same
    // source of truth the PTY backend must use, so a CLI redrawing via
    // ESC[2K/ESC[1A never wraps against a mismatched width.
    const disposeResize = term.onResize(({ cols, rows }) => {
      ResizeSession(sessionId, rows, cols).catch(() => {});
    });

    // Drive fit() off container layout — fires after layout, no manual
    // proposeDimensions needed.
    const ro = new ResizeObserver(() => {
      if (containerRef.current && containerRef.current.clientWidth > 0) {
        fitAddon.fit();
      }
    });
    ro.observe(containerRef.current);
    roRef.current = ro;

    const io = new IntersectionObserver((entries) => {
      for (const entry of entries) {
        if (entry.isIntersecting) {
          fitAddon.fit();
          requestAnimationFrame(() => fitAddon.fit());
          setTimeout(() => fitAddon.fit(), 50);
          setTimeout(() => fitAddon.fit(), 150);
        }
      }
    }, { threshold: 0.01 });
    io.observe(containerRef.current);
    ioRef.current = io;

    const handleWindowResize = () => {
      fitAddon.fit();
    };
    window.addEventListener("resize", handleWindowResize);

    return () => {
      outputHandlers.delete(sessionId);
      disposeInput.dispose();
      disposeResize.dispose();
      ro.disconnect();
      io.disconnect();
      window.removeEventListener("resize", handleWindowResize);
      term.element?.removeEventListener("dblclick", dblclickHandler);
      containerRef.current?.removeEventListener("click", handleFocusClick);
      containerRef.current?.removeEventListener("focus", handleFocusEvent);
      term.dispose();
      termRef.current = null;
      fitAddonRef.current = null;
      roRef.current = null;
      ioRef.current = null;
    };
  }, [isReady, sessionId]);

  useEffect(() => {
    if (isReady && termRef.current && termIdRef.current === sessionId && isActive) {
      const focus = () => {
        try {
          termRef.current?.focus();
          const ta = containerRef.current?.querySelector("textarea");
          if (ta) {
            ta.focus({ preventScroll: true });
          }
        } catch { /* ignore */ }
      };
      focus();
      const t1 = setTimeout(focus, 50);
      const t2 = setTimeout(focus, 150);
      return () => {
        clearTimeout(t1);
        clearTimeout(t2);
      };
    }
  }, [isReady, sessionId, isActive]);

  useEffect(() => {
    if (!isReady || !isActive || !containerRef.current) return;

    const handleWindowKeyDown = (e: KeyboardEvent) => {
      // Ignore key events with system modifiers (Cmd, Alt, etc.) to keep shortcuts working
      if (e.metaKey || e.altKey || e.ctrlKey) return;

      const activeEl = document.activeElement;
      if (
        activeEl &&
        (activeEl.tagName === "INPUT" ||
          activeEl.tagName === "TEXTAREA" ||
          activeEl.closest(".cm-editor") ||
          activeEl.closest(".selectable-text"))
      ) {
        return;
      }
      
      const ta = containerRef.current?.querySelector("textarea");
      if (ta && document.activeElement !== ta) {
        try {
          ta.focus({ preventScroll: true });
        } catch { /* ignore */ }
      }
    };

    window.addEventListener("keydown", handleWindowKeyDown, true);
    return () => {
      window.removeEventListener("keydown", handleWindowKeyDown, true);
    };
  }, [isReady, isActive, sessionId]);

  return (
    <div className="h-full w-full overflow-hidden">
      <div
        ref={containerRef}
        className="h-full w-full terminal-view focus:outline-none"
        tabIndex={0}
        style={{ background: "#0f1012", minHeight: 0, minWidth: 0 }}
      />
    </div>
  );
}
