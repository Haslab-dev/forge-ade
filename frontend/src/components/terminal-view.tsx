import { useEffect, useRef, useLayoutEffect } from "react";
import { Terminal } from "xterm";
import "xterm/css/xterm.css";
import { FitAddon } from "xterm-addon-fit";
import { WebLinksAddon } from "xterm-addon-web-links";
import { WriteSession, ResizeSession, GetHomeDir, OpenInFinder, IsDir } from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime";
import { globalOpenFile } from "../panels/editor";
import { getZoom, setZoom, onZoomChange } from "../lib/zoom";

// Inject terminal link and padding styles once
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
// Buffer output per session so terminals created after output starts or remounted keep full data
const outputBuffers = new Map<string, string[]>();
let globalInitialized = false;

function ensureGlobalListener() {
  if (globalInitialized) return;
  globalInitialized = true;
  EventsOn("session:output", (payload: any) => {
    if (payload && payload.id && payload.data) {
      // Always buffer
      let buf = outputBuffers.get(payload.id);
      if (!buf) {
        buf = [];
        outputBuffers.set(payload.id, buf);
      }
      buf.push(payload.data);
      // Keep buffer under 10k chunks
      if (buf.length > 10000) buf.splice(0, buf.length - 10000);
      // Also write to live handler if present
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
  isActive?: boolean;
}

export function TerminalView({ sessionId, isActive = true }: TerminalViewProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const zoomRef = useRef<HTMLDivElement>(null);
  const termRef = useRef<Terminal | null>(null);
  const fitAddonRef = useRef<FitAddon | null>(null);
  const roRef = useRef<ResizeObserver | null>(null);
  const ioRef = useRef<IntersectionObserver | null>(null);
  const termIdRef = useRef<string | null>(null);

  const safeFit = () => {
    if (
      containerRef.current &&
      containerRef.current.clientWidth > 0 &&
      containerRef.current.clientHeight > 0 &&
      fitAddonRef.current &&
      termRef.current
    ) {
      try {
        fitAddonRef.current.fit();
        termRef.current.refresh(0, -1);
        const dims = fitAddonRef.current.proposeDimensions();
        if (dims && dims.rows > 0 && dims.cols > 0) {
          ResizeSession(sessionId, dims.rows, dims.cols).catch(() => {});
        }
      } catch { /* ignore */ }
    }
  };

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

  // Trigger fit whenever active state changes or sessionId updates
  useEffect(() => {
    if (isActive) {
      safeFit();
      const raf = requestAnimationFrame(safeFit);
      const t1 = setTimeout(safeFit, 50);
      const t2 = setTimeout(safeFit, 150);
      return () => {
        cancelAnimationFrame(raf);
        clearTimeout(t1);
        clearTimeout(t2);
      };
    }
  }, [isActive, sessionId]);

  useEffect(() => {
    ensureGlobalListener();
    if (!containerRef.current) return;

    // Create fresh terminal instance for mount
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
    });

    const fitAddon = new FitAddon();
    term.loadAddon(fitAddon);

    // Custom regex: matches file paths (absolute, relative, ~/, with line:col)
    const filePathRegex = /(\/[^\s:]+(?::\d+(?::\d+)?)?|\.\.?\/[^\s:]+(?::\d+(?::\d+)?)?|~\/[^\s:]+(?::\d+(?::\d+)?)?)/;
    const webLinks = new WebLinksAddon(() => {}, { urlRegex: filePathRegex });
    term.loadAddon(webLinks);

    // Cmd+C to copy, Cmd+V to paste, Cmd+K to clear terminal
    term.attachCustomKeyEventHandler((e: KeyboardEvent) => {
      if (e.type === "keydown" && (e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        term.clear();
        return false;
      }
      if (e.metaKey && !e.ctrlKey && e.key === "c") {
        e.preventDefault();
        const sel = term.getSelection();
        if (sel) navigator.clipboard.writeText(sel).catch(() => {});
        return false;
      }
      if (e.metaKey && !e.ctrlKey && e.key === "v") {
        e.preventDefault();
        navigator.clipboard.readText().then((text) => {
          if (text) term.paste(text);
        }).catch(() => {});
        return false;
      }
      return true;
    });

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

    termRef.current = term;
    fitAddonRef.current = fitAddon;
    termIdRef.current = sessionId;

    // Open in DOM
    term.open(containerRef.current);

    // Synchronously fit and notify PTY of exact container dimensions immediately
    safeFit();
    term.focus();

    // Backup fits for animation / layout stabilization
    requestAnimationFrame(safeFit);
    setTimeout(safeFit, 50);
    setTimeout(safeFit, 150);

    // Snapshot current buffered output for this session (retains full history across mounts)
    const buf = outputBuffers.get(sessionId);
    if (buf && buf.length > 0) {
      term.write(buf.join(""));
      term.scrollToBottom();
    }

    let isAtBottom = true;
    const disposeScroll = term.onScroll(() => {
      isAtBottom = term.buffer.active.viewportY === term.buffer.active.baseY;
    });

    // Register live output handler
    outputHandlers.set(sessionId, (data: string) => {
      term.write(data);
      if (isAtBottom) {
        term.scrollToBottom();
      }
    });

    // Input handler
    const disposeInput = term.onData((input) => {
      WriteSession(sessionId, input).catch(() => {});
    });

    // Resize observer for element bounds change
    const ro = new ResizeObserver(() => {
      safeFit();
    });
    ro.observe(containerRef.current);
    roRef.current = ro;

    // Intersection observer for visibility change (e.g. tab becoming unhidden)
    const io = new IntersectionObserver((entries) => {
      for (const entry of entries) {
        if (entry.isIntersecting) {
          safeFit();
          requestAnimationFrame(safeFit);
          setTimeout(safeFit, 50);
          setTimeout(safeFit, 150);
        }
      }
    }, { threshold: 0.01 });
    io.observe(containerRef.current);
    ioRef.current = io;

    // Window resize handler
    const handleWindowResize = () => {
      safeFit();
    };
    window.addEventListener("resize", handleWindowResize);

    return () => {
      outputHandlers.delete(sessionId);
      disposeInput.dispose();
      disposeScroll.dispose();
      ro.disconnect();
      io.disconnect();
      window.removeEventListener("resize", handleWindowResize);
      term.dispose();
      termRef.current = null;
      fitAddonRef.current = null;
      roRef.current = null;
      ioRef.current = null;
    };
  }, [sessionId]);

  useLayoutEffect(() => {
    if (termRef.current && termIdRef.current === sessionId && isActive) {
      termRef.current.focus();
    }
  }, [sessionId, isActive]);

  return (
    <div ref={zoomRef} className="h-full w-full overflow-hidden" style={{ height: "100%", width: "100%" }}>
      <div
        ref={containerRef}
        className="h-full w-full"
        style={{ background: "#000000", minHeight: 0, minWidth: 0 }}
      />
    </div>
  );
}




