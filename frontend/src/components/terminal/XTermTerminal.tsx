import React, { useState, useRef, useEffect, useCallback } from 'react';
import { 
  Plus, 
  Trash2, 
  Maximize2, 
  Minimize2, 
  X, 
  Terminal as TerminalIcon,
  CheckCircle2,
  AlertCircle,
  RefreshCw,
  Cpu,
  ChevronDown,
  Columns2,
  MoreHorizontal
} from 'lucide-react';
import { Terminal } from 'xterm';
import { FitAddon } from 'xterm-addon-fit';
import 'xterm/css/xterm.css';
import { useWorkspace } from '../../stores/workspaceStore';

interface ShellTab {
  id: string;
  name: string;
  cwd: string;
  status: 'connected' | 'connecting' | 'disconnected';
}

export const XTermTerminal: React.FC = () => {
  const { 
    mode,
    terminalTabs, 
    activeTerminalTabId, 
    setActiveTerminalTabId, 
    setIsTerminalOpen,
    isTerminalOpen,
    activeWorkspacePath,
    diagnostics,
    theme
  } = useWorkspace();

  const [shells, setShells] = useState<ShellTab[]>([
    {
      id: 'shell-1',
      name: 'zsh (1)',
      cwd: activeWorkspacePath || '',
      status: 'connecting'
    }
  ]);
  const [activeShellId, setActiveShellId] = useState<string>('shell-1');
  const [isMaximized, setIsMaximized] = useState(false);
  const [terminalHeight, setTerminalHeight] = useState(240);
  const isResizingRef = useRef(false);

  const handleMouseDownResize = (e: React.MouseEvent) => {
    e.preventDefault();
    isResizingRef.current = true;
    const startY = e.clientY;
    const startHeight = terminalHeight;

    const handleMouseMove = (moveEvent: MouseEvent) => {
      if (!isResizingRef.current) return;
      const newHeight = Math.max(120, Math.min(window.innerHeight * 0.8, startHeight + (startY - moveEvent.clientY)));
      setTerminalHeight(newHeight);
      const currentActive = terminalMapRef.current.get(activeShellId);
      if (currentActive) {
        currentActive.fitAddon.fit();
      }
    };

    const handleMouseUp = () => {
      isResizingRef.current = false;
      window.removeEventListener('mousemove', handleMouseMove);
      window.removeEventListener('mouseup', handleMouseUp);
    };

    window.addEventListener('mousemove', handleMouseMove);
    window.addEventListener('mouseup', handleMouseUp);
  };

  // References to hold Terminal instances, FitAddons, and WebSockets per shell ID
  const terminalMapRef = useRef<Map<string, {
    term: Terminal;
    fitAddon: FitAddon;
    ws: WebSocket;
    container: HTMLDivElement;
  }>>(new Map());

  const activeShellContainerRef = useRef<HTMLDivElement>(null);

  // Get terminal colors based on current theme
  const getTerminalTheme = useCallback(() => {
    const isDark = theme === 'dark';
    return {
      background: isDark ? '#141416' : '#f8fafc',
      foreground: isDark ? '#e2e8f0' : '#0f172a',
      cursor: isDark ? '#38bdf8' : '#0284c7',
      cursorAccent: isDark ? '#0f172a' : '#ffffff',
      selectionBackground: isDark ? '#334155' : '#cbd5e1',
      black: isDark ? '#1e293b' : '#0f172a',
      red: '#ef4444',
      green: '#10b981',
      yellow: '#f59e0b',
      blue: '#3b82f6',
      magenta: '#a855f7',
      cyan: '#06b6d4',
      white: isDark ? '#f8fafc' : '#ffffff',
      brightBlack: '#64748b',
      brightRed: '#f87171',
      brightGreen: '#34d399',
      brightYellow: '#fbbf24',
      brightBlue: '#60a5fa',
      brightMagenta: '#c084fc',
      brightCyan: '#22d3ee',
      brightWhite: '#ffffff'
    };
  }, [theme]);

  // Connect or initialize a terminal instance for a shell ID
  const initTerminalForShell = useCallback((shell: ShellTab, container: HTMLDivElement) => {
    // If instance already exists, attach container and refit
    const existing = terminalMapRef.current.get(shell.id);
    if (existing) {
      if (existing.container !== container) {
        container.innerHTML = '';
        existing.term.open(container);
        existing.container = container;
      }
      existing.term.options.theme = getTerminalTheme();
      setTimeout(() => existing.fitAddon.fit(), 50);
      return;
    }

    container.innerHTML = '';

    const term = new Terminal({
      cursorBlink: true,
      cursorStyle: 'bar',
      fontFamily: "'Fira Code', Menlo, Monaco, 'Courier New', monospace",
      fontSize: 13,
      lineHeight: 1.25,
      letterSpacing: 0,
      theme: getTerminalTheme(),
      allowProposedApi: true
    });

    const fitAddon = new FitAddon();
    term.loadAddon(fitAddon);
    term.open(container);

    // Initial fit
    setTimeout(() => {
      try { fitAddon.fit(); } catch {}
    }, 50);

    // Setup WebSocket connection to PTY backend
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.host;
    const wsUrl = `${protocol}//${host}/pty?cwd=${encodeURIComponent(shell.cwd || activeWorkspacePath || '')}`;

    term.writeln('\x1b[38;5;39m⚡ my-ade Native PTY Session (Login Shell)\x1b[0m');
    term.writeln('\x1b[90mConnected to interactive zsh pseudo-terminal.\x1b[0m\r\n');

    let ws: WebSocket;
    try {
      ws = new WebSocket(wsUrl);
    } catch (err: any) {
      term.writeln(`\r\n\x1b[31mFailed to connect WebSocket: ${err.message}\x1b[0m\r\n`);
      return;
    }

    ws.onopen = () => {
      setShells(prev => prev.map(s => s.id === shell.id ? { ...s, status: 'connected' } : s));
      try {
        fitAddon.fit();
        ws.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }));
      } catch {}
    };

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data);
        if (msg.type === 'output' && msg.data) {
          term.write(msg.data);
        } else if (msg.type === 'exit') {
          term.writeln(`\r\n\x1b[33m[Process completed with exit code ${msg.code}]\x1b[0m\r\n`);
          setShells(prev => prev.map(s => s.id === shell.id ? { ...s, status: 'disconnected' } : s));
        }
      } catch {
        term.write(event.data);
      }
    };

    ws.onerror = () => {
      setShells(prev => prev.map(s => s.id === shell.id ? { ...s, status: 'disconnected' } : s));
      term.writeln('\r\n\x1b[31m⚠️ PTY WebSocket Connection Error\x1b[0m\r\n');
    };

    ws.onclose = () => {
      setShells(prev => prev.map(s => s.id === shell.id ? { ...s, status: 'disconnected' } : s));
    };

    // Forward keyboard input from xterm to PTY
    const onDataDisposable = term.onData((data: string) => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 'input', data }));
      }
    });

    // Forward terminal resize events
    const onResizeDisposable = term.onResize(({ cols, rows }: { cols: number; rows: number }) => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 'resize', cols, rows }));
      }
    });

    terminalMapRef.current.set(shell.id, {
      term,
      fitAddon,
      ws,
      container
    });
  }, [activeWorkspacePath, getTerminalTheme]);

  // Mount active shell terminal when active tab / active shell changes
  useEffect(() => {
    if (!isTerminalOpen || activeTerminalTabId !== 'term-terminal-1') return;

    const currentShell = shells.find(s => s.id === activeShellId) || shells[0];
    if (currentShell && activeShellContainerRef.current) {
      initTerminalForShell(currentShell, activeShellContainerRef.current);
    }
  }, [activeShellId, activeTerminalTabId, isTerminalOpen, shells, initTerminalForShell]);

  // Update theme of all terminals on theme toggle
  useEffect(() => {
    const nextTheme = getTerminalTheme();
    terminalMapRef.current.forEach(({ term }) => {
      term.options.theme = nextTheme;
    });
  }, [theme, getTerminalTheme]);

  // Handle window resize to refit terminal
  useEffect(() => {
    const handleResize = () => {
      terminalMapRef.current.forEach(({ fitAddon, ws, term }) => {
        try {
          fitAddon.fit();
          if (ws.readyState === WebSocket.OPEN) {
            ws.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }));
          }
        } catch {}
      });
    };

    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, []);

  // Fit active terminal after mode switch (Agent <-> Editor), maximize toggle, or drawer transition
  useEffect(() => {
    if (mode !== 'editor' || !isTerminalOpen) return;

    const timer = setTimeout(() => {
      const activeObj = terminalMapRef.current.get(activeShellId);
      if (activeObj) {
        try {
          activeObj.fitAddon.fit();
          activeObj.term.focus();
          if (activeObj.ws.readyState === WebSocket.OPEN) {
            activeObj.ws.send(JSON.stringify({ type: 'resize', cols: activeObj.term.cols, rows: activeObj.term.rows }));
          }
        } catch {}
      }
    }, 50);
    return () => clearTimeout(timer);
  }, [mode, isMaximized, isTerminalOpen, activeShellId]);

  const handleAddNewShell = () => {
    const nextNum = shells.length + 1;
    const newId = `shell-${Date.now()}`;
    const newShell: ShellTab = {
      id: newId,
      name: `zsh (${nextNum})`,
      cwd: activeWorkspacePath,
      status: 'connecting'
    };
    setShells(prev => [...prev, newShell]);
    setActiveShellId(newId);
  };

  const handleCloseShell = (id: string, e: React.MouseEvent) => {
    e.stopPropagation();
    if (shells.length <= 1) return;

    const instance = terminalMapRef.current.get(id);
    if (instance) {
      try {
        instance.ws.close();
        instance.term.dispose();
      } catch {}
      terminalMapRef.current.delete(id);
    }

    setShells(prev => {
      const next = prev.filter(s => s.id !== id);
      if (activeShellId === id) {
        setActiveShellId(next[0].id);
      }
      return next;
    });
  };

  const handleClearTerminal = () => {
    const activeObj = terminalMapRef.current.get(activeShellId);
    if (activeObj) {
      activeObj.term.clear();
      activeObj.term.focus();
    }
  };

  const handleReconnect = () => {
    const currentShell = shells.find(s => s.id === activeShellId);
    if (!currentShell) return;

    const existing = terminalMapRef.current.get(activeShellId);
    if (existing) {
      try {
        existing.ws.close();
        existing.term.dispose();
      } catch {}
      terminalMapRef.current.delete(activeShellId);
    }

    if (activeShellContainerRef.current) {
      initTerminalForShell(currentShell, activeShellContainerRef.current);
    }
  };

  if (!isTerminalOpen) return null;

  const currentShell = shells.find(s => s.id === activeShellId) || shells[0];

  return (
    <div 
      style={{ height: isMaximized ? '80vh' : `${terminalHeight}px` }}
      className="border-t border-[#e2e8f0] dark:border-[#2b2b2b] bg-white dark:bg-[#181818] flex flex-col z-20 relative select-none"
    >
      {/* Top Drag Resizer Handle */}
      <div 
        onMouseDown={handleMouseDownResize}
        className="absolute top-0 left-0 right-0 h-[4px] cursor-row-resize hover:bg-[#2563eb]/50 transition-colors z-30"
      />

      {/* Top Header Tabs matching Screenshot */}
      <div className="h-9 min-h-[36px] bg-white dark:bg-[#181818] border-b border-[#e2e8f0] dark:border-[#2b2b2b] flex items-center justify-between px-3 select-none">
        
        {/* Left Side: Problems | Output | Debug Console | Terminal | Ports */}
        <div className="flex items-center gap-2 overflow-x-auto text-xs">
          {/* Problems */}
          <button
            type="button"
            onClick={() => setActiveTerminalTabId('term-problems')}
            className={`px-2 py-1 rounded-md transition-colors flex items-center gap-1.5 cursor-pointer ${
              activeTerminalTabId === 'term-problems'
                ? 'bg-[#e5e7eb] dark:bg-[#28282b] text-[#0f172a] dark:text-white font-medium'
                : 'text-[#64748b] dark:text-[#9ca3af] hover:text-[#0f172a] dark:hover:text-white'
            }`}
          >
            <span>Problems</span>
            <span className="px-1.5 py-0.2 rounded-full bg-[#2563eb] text-white text-[10px] font-bold">
              {diagnostics.length || 5}
            </span>
          </button>

          {/* Output */}
          <button
            type="button"
            onClick={() => setActiveTerminalTabId('term-output')}
            className={`px-2 py-1 rounded-md transition-colors cursor-pointer ${
              activeTerminalTabId === 'term-output'
                ? 'bg-[#e5e7eb] dark:bg-[#28282b] text-[#0f172a] dark:text-white font-medium'
                : 'text-[#64748b] dark:text-[#9ca3af] hover:text-[#0f172a] dark:hover:text-white'
            }`}
          >
            Output
          </button>

          {/* Debug Console */}
          <button
            type="button"
            onClick={() => setActiveTerminalTabId('term-debug')}
            className={`px-2 py-1 rounded-md transition-colors cursor-pointer ${
              activeTerminalTabId === 'term-debug'
                ? 'bg-[#e5e7eb] dark:bg-[#28282b] text-[#0f172a] dark:text-white font-medium'
                : 'text-[#64748b] dark:text-[#9ca3af] hover:text-[#0f172a] dark:hover:text-white'
            }`}
          >
            Debug Console
          </button>

          {/* Terminal */}
          <button
            type="button"
            onClick={() => setActiveTerminalTabId('term-terminal-1')}
            className={`px-2.5 py-1 rounded-md transition-colors cursor-pointer ${
              activeTerminalTabId === 'term-terminal-1'
                ? 'bg-[#e5e7eb] dark:bg-[#28282b] text-[#0f172a] dark:text-white font-semibold'
                : 'text-[#64748b] dark:text-[#9ca3af] hover:text-[#0f172a] dark:hover:text-white'
            }`}
          >
            Terminal
          </button>

          {/* Ports */}
          <button
            type="button"
            onClick={() => setActiveTerminalTabId('term-ports')}
            className={`px-2 py-1 rounded-md transition-colors cursor-pointer ${
              activeTerminalTabId === 'term-ports'
                ? 'bg-[#e5e7eb] dark:bg-[#28282b] text-[#0f172a] dark:text-white font-medium'
                : 'text-[#64748b] dark:text-[#9ca3af] hover:text-[#0f172a] dark:hover:text-white'
            }`}
          >
            Ports
          </button>
        </div>

        {/* Right Side Actions matching Screenshot */}
        <div className="flex items-center gap-1.5 text-[#64748b] dark:text-[#9ca3af]">
          {/* Shell name / dropdown */}
          <div className="flex items-center gap-1 hover:text-[#0f172a] dark:hover:text-white px-1.5 py-0.5 rounded cursor-pointer transition-colors text-xs font-mono">
            <TerminalIcon className="w-3.5 h-3.5" />
            <span>zsh</span>
          </div>

          <button
            type="button"
            onClick={handleAddNewShell}
            className="flex items-center gap-0.5 p-1 hover:bg-[#f1f5f9] dark:hover:bg-[#252528] rounded hover:text-[#0f172a] dark:hover:text-white cursor-pointer transition-colors"
            title="New Terminal"
          >
            <Plus className="w-3.5 h-3.5" />
            <ChevronDown className="w-2.5 h-2.5" />
          </button>

          {/* AI command icon (@) */}
          <button
            type="button"
            className="p-1 hover:bg-[#f1f5f9] dark:hover:bg-[#252528] rounded hover:text-[#0f172a] dark:hover:text-white cursor-pointer transition-colors font-mono text-xs font-bold px-1.5"
            title="Generate Terminal Command (⌘I)"
          >
            @
          </button>

          {/* Split Terminal */}
          <button
            type="button"
            className="p-1 hover:bg-[#f1f5f9] dark:hover:bg-[#252528] rounded hover:text-[#0f172a] dark:hover:text-white cursor-pointer transition-colors"
            title="Split Terminal (⌘\)"
          >
            <Columns2 className="w-3.5 h-3.5" />
          </button>

          {/* Kill / Trash */}
          <button
            type="button"
            onClick={handleClearTerminal}
            className="p-1 hover:bg-[#f1f5f9] dark:hover:bg-[#252528] rounded hover:text-[#0f172a] dark:hover:text-white cursor-pointer transition-colors"
            title="Kill Terminal Session"
          >
            <Trash2 className="w-3.5 h-3.5" />
          </button>

          {/* More actions (...) */}
          <button
            type="button"
            className="p-1 hover:bg-[#f1f5f9] dark:hover:bg-[#252528] rounded hover:text-[#0f172a] dark:hover:text-white cursor-pointer transition-colors"
            title="More Terminal Actions"
          >
            <MoreHorizontal className="w-3.5 h-3.5" />
          </button>

          {/* Separator */}
          <div className="w-[1px] h-3.5 bg-[#d1d5db] dark:bg-[#383838] mx-0.5" />

          {/* Maximize / Restore */}
          <button
            type="button"
            onClick={() => setIsMaximized(prev => !prev)}
            className="p-1 hover:bg-[#f1f5f9] dark:hover:bg-[#252528] rounded hover:text-[#0f172a] dark:hover:text-white cursor-pointer transition-colors"
            title={isMaximized ? 'Restore Panel Size' : 'Maximize Panel Size'}
          >
            {isMaximized ? <Minimize2 className="w-3.5 h-3.5" /> : <Maximize2 className="w-3.5 h-3.5" />}
          </button>

          {/* Close panel */}
          <button
            type="button"
            onClick={() => setIsTerminalOpen(false)}
            className="p-1 hover:bg-[#fee2e2] dark:hover:bg-[#450a0a] rounded hover:text-[#ef4444] cursor-pointer transition-colors"
            title="Close Panel (⌘`)"
          >
            <X className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      {/* Main Terminal Viewport (XTerm / Problems / Output) */}
      <div className="flex-1 overflow-hidden relative bg-white dark:bg-[#141416] flex flex-col justify-between">
        {activeTerminalTabId === 'term-terminal-1' ? (
          <>
            <div 
              ref={activeShellContainerRef} 
              className="w-full flex-1 p-2 overflow-hidden"
              style={{ padding: '8px 12px' }}
            />
            {/* Subtle bottom center hint matching screenshot */}
            <div className="text-center py-1 text-[11px] font-mono text-[#9ca3af] dark:text-[#666666] select-none pointer-events-none">
              ⌘I to generate a command.
            </div>
          </>
        ) : activeTerminalTabId === 'term-problems' ? (
          <div className="p-4 overflow-y-auto h-full text-xs font-sans space-y-2">
            <h3 className="font-bold text-[#0f172a] dark:text-white flex items-center gap-2">
              <AlertCircle className="w-4 h-4 text-[#f59e0b]" />
              Workspace Diagnostics & Lints ({diagnostics.length})
            </h3>
            {diagnostics.length === 0 ? (
              <p className="text-[#64748b] dark:text-[#94a3b8]">No problems detected in the workspace.</p>
            ) : (
              <div className="space-y-1.5">
                {diagnostics.map((diag, i) => (
                  <div key={i} className="p-2 rounded-lg bg-white dark:bg-[#1e1e1e] border border-[#e2e8f0] dark:border-[#2b2b2b] flex items-center justify-between">
                    <span className="font-mono text-[#ef4444]">{diag.message}</span>
                    <span className="text-[#94a3b8] text-[10px]">Line {diag.line}</span>
                  </div>
                ))}
              </div>
            )}
          </div>
        ) : (
          <div className="p-4 text-xs font-mono text-[#64748b] dark:text-[#94a3b8]">
            forge-ade development stream running. Output buffer ready.
          </div>
        )}
      </div>

    </div>
  );
};
