import { useEffect, useRef, useCallback } from "react";
import { Terminal } from "xterm";
import "xterm/css/xterm.css";
import { FitAddon } from "xterm-addon-fit";
import { WriteSession, ResizeSession } from "../../wailsjs/go/main/App";
import { EventsOn, EventsOff } from "../../wailsjs/runtime/runtime";

interface TerminalViewProps {
  sessionId: string;
  sessionName: string;
  visible: boolean;
}

export function TerminalView({ sessionId, sessionName, visible }: TerminalViewProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const terminalRef = useRef<Terminal | null>(null);
  const fitAddonRef = useRef<FitAddon | null>(null);

  // Initialize xterm
  useEffect(() => {
    if (!containerRef.current || !visible) return;

    const term = new Terminal({
      cursorBlink: true,
      cursorStyle: "bar",
      fontSize: 13,
      fontFamily: "'JetBrains Mono', 'Fira Code', 'Cascadia Code', monospace",
      theme: {
        background: "#1e1e2e",
        foreground: "#cdd6f4",
        cursor: "#f5e0dc",
        selectionBackground: "#585b70",
        black: "#45475a",
        red: "#f38ba8",
        green: "#a6e3a1",
        yellow: "#f9e2af",
        blue: "#89b4fa",
        magenta: "#f5c2e7",
        cyan: "#94e2d5",
        white: "#bac2de",
        brightBlack: "#585b70",
        brightRed: "#f38ba8",
        brightGreen: "#a6e3a1",
        brightYellow: "#f9e2af",
        brightBlue: "#89b4fa",
        brightMagenta: "#f5c2e7",
        brightCyan: "#94e2d5",
        brightWhite: "#a6adc8",
      },
      allowTransparency: true,
      cols: 80,
      rows: 24,
    });

    const fitAddon = new FitAddon();
    term.loadAddon(fitAddon);
    fitAddonRef.current = fitAddon;

    term.open(containerRef.current);
    term.focus();

    // Fit to container after mount
    requestAnimationFrame(() => fitAddon.fit());

    terminalRef.current = term;

    // Write header
    term.writeln(`\x1b[36m━━━ ForgeADE Session: ${sessionName} ━━━\x1b[0m`);

    // Listen for output events from Go backend
    const eventName = "session:output";
    const dispose = EventsOn(eventName, (data: any) => {
      if (data && data.id === sessionId && data.data) {
        term.write(data.data);
      }
    });

    // Handle user input → send to PTY
    term.onData((input) => {
      WriteSession(sessionId, input).catch(() => {});
    });

    // Handle resize
    const resizeObserver = new ResizeObserver(() => {
      try {
        fitAddon.fit();
        const dims = fitAddon.proposeDimensions();
        if (dims) {
          ResizeSession(sessionId, dims.rows, dims.cols).catch(() => {});
        }
      } catch {
        // ignore
      }
    });

    if (containerRef.current) {
      resizeObserver.observe(containerRef.current);
    }

    return () => {
      if (dispose && typeof dispose === "function") dispose();
      EventsOff(eventName);
      resizeObserver.disconnect();
      term.dispose();
      terminalRef.current = null;
    };
  }, [sessionId, visible]);

  // Re-fit when visibility changes
  useEffect(() => {
    if (visible && fitAddonRef.current) {
      requestAnimationFrame(() => fitAddonRef.current?.fit());
    }
  }, [visible]);

  return (
    <div
      ref={containerRef}
      className="h-full w-full"
      style={{ background: "#1e1e2e", display: visible ? "block" : "none" }}
    />
  );
}
