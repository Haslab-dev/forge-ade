import { useEffect, useRef } from "react";
import { Terminal } from "xterm";
import "xterm/css/xterm.css";
import { FitAddon } from "xterm-addon-fit";
import { WriteSession, ResizeSession } from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";

// ── Global output dispatcher ──────────────────────────────────────
// Single EventsOn listener dispatches to all terminal instances.
// This avoids duplicate listeners from re-mounting components.

type OutputHandler = (data: string) => void;
const outputHandlers = new Map<string, OutputHandler>();

let globalListenerInitialized = false;

function ensureGlobalListener() {
  if (globalListenerInitialized) return;
  globalListenerInitialized = true;
  EventsOn("session:output", (payload: any) => {
    if (payload && payload.id && payload.data) {
      const handler = outputHandlers.get(payload.id);
      if (handler) handler(payload.data);
    }
  });
}

// ── Persist xterm instances across re-renders ─────────────────────
const termInstances = new Map<string, {
  term: Terminal;
  fitAddon: FitAddon;
  initialized: boolean;
}>();

// ── Theme ─────────────────────────────────────────────────────────
const homebrewTheme = {
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
  sessionName: string;
}

export function TerminalView({ sessionId, sessionName }: TerminalViewProps) {
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    // Ensure the global output listener exists (only once ever)
    ensureGlobalListener();

    if (!containerRef.current) return;

    // Create or retrieve cached xterm instance
    let instance = termInstances.get(sessionId);
    if (!instance) {
      const term = new Terminal({
        cursorBlink: true,
        cursorStyle: "bar",
        fontSize: 14,
        fontFamily: "'JetBrains Mono', 'Fira Code', 'Cascadia Code', 'Menlo', monospace",
        theme: homebrewTheme,
        allowTransparency: false,
        cols: 80,
        rows: 24,
        scrollback: 10000,
        convertEol: true,
      });
      const fitAddon = new FitAddon();
      term.loadAddon(fitAddon);
      instance = { term, fitAddon, initialized: false };
      termInstances.set(sessionId, instance);
    }

    const { term, fitAddon } = instance;

    // Mount to DOM
    term.open(containerRef.current);
    term.focus();
    requestAnimationFrame(() => fitAddon.fit());

    // Write header only once per session lifetime
    if (!instance.initialized) {
      instance.initialized = true;
      term.writeln("\x1b[32m━━━ ForgeADE Session: " + sessionName + " ━━━\x1b[0m");
      term.writeln("");
    }

    // Register output handler for this session
    outputHandlers.set(sessionId, (data: string) => {
      term.write(data);
      term.scrollToBottom();
    });

    // Handle user input → send to PTY
    const disposeData = term.onData((input) => {
      WriteSession(sessionId, input).catch(() => {});
    });

    // Handle resize
    const resizeObserver = new ResizeObserver(() => {
      try {
        fitAddon.fit();
        const dims = fitAddon.proposeDimensions();
        if (dims) {
          term.scrollToBottom();
          ResizeSession(sessionId, dims.rows, dims.cols).catch(() => {});
        }
      } catch { /* ignore */ }
    });
    resizeObserver.observe(containerRef.current);

    return () => {
      // Cleanup: remove handler, detach from DOM, keep xterm instance
      outputHandlers.delete(sessionId);
      disposeData.dispose();
      resizeObserver.disconnect();
      if (containerRef.current) {
        containerRef.current.innerHTML = "";
      }
    };
  }, [sessionId, sessionName]);

  return (
    <div
      ref={containerRef}
      className="h-full w-full"
      style={{ background: "#000000", minHeight: 0 }}
    />
  );
}
