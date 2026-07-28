import { useEffect, useRef } from "react";
import { Terminal } from "xterm";
import "xterm/css/xterm.css";
import { FitAddon } from "xterm-addon-fit";
import { WebLinksAddon } from "xterm-addon-web-links";
import { WriteSession, ResizeSession, GetHomeDir, OpenInFinder, IsDir } from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime";
import { globalOpenFile } from "../panels/editor";
import { getZoom, setZoom, onZoomChange } from "../lib/zoom";

// Inject terminal link styles once
const styleId = "forge-xterm-link-style";
if (!document.getElementById(styleId)) {
  const style = document.createElement("style");
  style.id = styleId;
  style.textContent = `
    .xterm .xterm-link-layer a, .xterm-link-layer a {
      text-decoration: underline !important;
      text-decoration-color: #4F8CFF !important;
      cursor: pointer !important;
    }
  `;
  document.head.appendChild(style);
}

let homeDir = "";
GetHomeDir().then((h) => { homeDir = h; }).catch(() => {});

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

const termCache = new Map<string, { term: Terminal; fitAddon: FitAddon }>();

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
  const zoomRef = useRef<HTMLDivElement>(null);

  // Zoom in/out with Cmd+= / Cmd+-
  useEffect(() => {
    const el = zoomRef.current;
    if (!el) return;
    el.style.zoom = String(getZoom());
    const unsub = onZoomChange(() => { if (el) el.style.zoom = String(getZoom()); });
    const handler = (e: KeyboardEvent) => {
      if (!e.metaKey && !e.ctrlKey) return;
      if (e.key === "=" || e.key === "+") { e.preventDefault(); setZoom(getZoom() + 0.1); }
      if (e.key === "-") { e.preventDefault(); setZoom(getZoom() - 0.1); }
      if (e.key === "0") { e.preventDefault(); setZoom(1); }
    };
    document.addEventListener("keydown", handler);
    return () => { document.removeEventListener("keydown", handler); unsub(); };
  }, []);

  useEffect(() => {
    ensureGlobalListener();
    if (!containerRef.current) return;

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

      // Custom regex: matches file paths (absolute, relative, ~/, with line:col)
      const filePathRegex = /(\/[^\s:]+(?::\d+(?::\d+)?)?|\.\.?\/[^\s:]+(?::\d+(?::\d+)?)?|~\/[^\s:]+(?::\d+(?::\d+)?)?)/;

      const webLinks = new WebLinksAddon(() => {}, { urlRegex: filePathRegex });
      term.loadAddon(webLinks);

      // Double-click to open file/dir paths
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

      cached = { term, fitAddon };
      termCache.set(sessionId, cached);
    }

    const { term, fitAddon } = cached!;
    term.open(containerRef.current);
    term.focus();
    requestAnimationFrame(() => fitAddon.fit());

    outputHandlers.set(sessionId, (data: string) => {
      term.write(data);
      term.scrollToBottom();
    });

    const disposeInput = term.onData((input) => {
      WriteSession(sessionId, input).catch(() => {});
    });

    const ro = new ResizeObserver(() => {
      try {
        fitAddon.fit();
        const dims = fitAddon.proposeDimensions();
        if (dims) {
          term.scrollToBottom();
          ResizeSession(sessionId, dims.rows, dims.cols).catch(() => {});
        }
      } catch { /* ignore */ }
    });
    ro.observe(containerRef.current);

    return () => {
      outputHandlers.delete(sessionId);
      disposeInput.dispose();
      ro.disconnect();
      if (containerRef.current) containerRef.current.innerHTML = "";
    };
  }, [sessionId]);

  return (
    <div ref={zoomRef} style={{ height: "100%" }}>
      <div
        ref={containerRef}
        className="h-full w-full"
        style={{ background: "#000000", minHeight: 0 }}
      />
    </div>
  );
}
