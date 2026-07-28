import { useEffect, useRef } from "react";
import { Terminal } from "xterm";
import "xterm/css/xterm.css";
import { FitAddon } from "xterm-addon-fit";
import { WriteSession, ResizeSession } from "../../wailsjs/go/main/App";
import { EventsOn, EventsOff } from "../../wailsjs/runtime/runtime";

// Persist xterm instances across re-renders (keyed by sessionId)
const termInstances = new Map<string, {
  term: Terminal;
  fitAddon: FitAddon;
  initialized: boolean;
}>();

// Cleanup on full unmount
window.addEventListener("beforeunload", () => {
  termInstances.forEach(({ term }) => term.dispose());
  termInstances.clear();
});

interface TerminalViewProps {
  sessionId: string;
  sessionName: string;
}

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

export function TerminalView({ sessionId, sessionName }: TerminalViewProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const initializedRef = useRef(false);

  useEffect(() => {
    if (!containerRef.current) return;

    // Check if we already have an instance for this session
    let instance = termInstances.get(sessionId);

    if (!instance) {
      // Create new xterm instance
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

    // Write header only once per session lifetime
    if (!instance.initialized) {
      instance.initialized = true;
      term.writeln("\x1b[32m━━━ ForgeADE Session: " + sessionName + " ━━━\x1b[0m");
      term.writeln("");
    }

    // Focus and fit
    term.focus();
    requestAnimationFrame(() => fitAddon.fit());

    // Listen for output events from Go backend
    const dispose = EventsOn("session:output", (data: any) => {
      if (data && data.id === sessionId && data.data) {
        term.write(data.data);
        // Auto-scroll to bottom
        term.scrollToBottom();
      }
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
      } catch {
        // ignore
      }
    });
    resizeObserver.observe(containerRef.current);

    return () => {
      // Cleanup listeners but KEEP the xterm instance for reuse
      if (dispose && typeof dispose === "function") dispose();
      EventsOff("session:output");
      disposeData.dispose();
      resizeObserver.disconnect();

      // Detach from DOM but don't destroy the terminal
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
