import { useEffect, useRef } from "react";
import { Terminal } from "xterm";
import "xterm/css/xterm.css";
import { FitAddon } from "xterm-addon-fit";
import { WriteSession, ResizeSession } from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";

// ── Global output dispatcher (single EventsOn for all sessions) ────
type OutputHandler = (data: string) => void;
const outputHandlers = new Map<string, OutputHandler>();
let globalInitialized = false;

function ensureGlobalListener() {
  if (globalInitialized) return;
  globalInitialized = true;
  EventsOn("session:output", (payload: any) => {
    if (payload && payload.id && payload.data) {
      const handler = outputHandlers.get(payload.id);
      if (handler) handler(payload.data);
    }
  });
}

// ── Persist xterm instances per session (survives tab switches) ────
const termCache = new Map<
  string,
  { term: Terminal; fitAddon: FitAddon }
>();

// ── Theme ─────────────────────────────────────────────────────────
const theme = {
  background: "#000000",
  foreground: "#33ff00",
  cursor: "#33ff00",
  cursorAccent: "#000000",
  selectionBackground: "#33ff0033",
  black: "#000000",
  red: "#cc0000",
  green: "#33ff00",
  yellow: "#ffff00",
  blue: "#0066ff",
  magenta: "#cc00ff",
  cyan: "#00ffff",
  white: "#d0d0d0",
  brightBlack: "#808080",
  brightRed: "#ff0000",
  brightGreen: "#33ff00",
  brightYellow: "#ffff00",
  brightBlue: "#0066ff",
  brightMagenta: "#cc00ff",
  brightCyan: "#00ffff",
  brightWhite: "#ffffff",
};

interface TerminalViewProps {
  sessionId: string;
}

export function TerminalView({ sessionId }: TerminalViewProps) {
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    ensureGlobalListener();
    if (!containerRef.current) return;

    // Get or create cached xterm
    let cached = termCache.get(sessionId);
    if (!cached) {
      const term = new Terminal({
        cursorBlink: true,
        cursorStyle: "bar",
        fontSize: 14,
        fontFamily: "'JetBrains Mono', 'Fira Code', 'Cascadia Code', 'Menlo', monospace",
        theme,
        allowTransparency: false,
        cols: 80,
        rows: 24,
        scrollback: 10000,
        convertEol: true,
      });
      const fitAddon = new FitAddon();
      term.loadAddon(fitAddon);
      cached = { term, fitAddon };
      termCache.set(sessionId, cached);
    }

    const { term, fitAddon } = cached;

    // Mount to DOM
    term.open(containerRef.current);
    term.focus();
    requestAnimationFrame(() => fitAddon.fit());

    // Register output handler for THIS session
    outputHandlers.set(sessionId, (data: string) => {
      term.write(data);
      term.scrollToBottom();
    });

    // User input → PTY
    const disposeInput = term.onData((input) => {
      WriteSession(sessionId, input).catch(() => {});
    });

    // Resize observer
    const ro = new ResizeObserver(() => {
      try {
        fitAddon.fit();
        const dims = fitAddon.proposeDimensions();
        if (dims) {
          term.scrollToBottom();
          ResizeSession(sessionId, dims.rows, dims.cols).catch(() => {});
        }
      } catch {
        /* ignore */
      }
    });
    ro.observe(containerRef.current);

    return () => {
      // Cleanup: unregister handler, detach DOM, keep xterm cached
      outputHandlers.delete(sessionId);
      disposeInput.dispose();
      ro.disconnect();
      if (containerRef.current) {
        containerRef.current.innerHTML = "";
      }
    };
  }, [sessionId]);

  return (
    <div
      ref={containerRef}
      className="h-full w-full"
      style={{ background: "#000000", minHeight: 0 }}
    />
  );
}
