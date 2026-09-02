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
  Cpu
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
      className={`border-t border-[#e2e8f0] dark:border-[#2b2b2b] bg-white dark:bg-[#181818] flex flex-col transition-all duration-200 z-20 ${
        isMaximized ? 'h-[80vh]' : 'h-64'
      }`}
    >
      {/* Top Header Tabs: Problems | Output | Debug | Terminal | Ports */}
      <div className="h-9 min-h-[36px] bg-[#f8fafc] dark:bg-[#141416] border-b border-[#e2e8f0] dark:border-[#2b2b2b] flex items-center justify-between px-3 select-none">
        
        {/* Left Side: VS Code style Tab Categories */}
        <div className="flex items-center gap-1 overflow-x-auto text-xs font-medium">
          {terminalTabs.map(tab => {
            const isActive = activeTerminalTabId === tab.id;
            return (
              <button
                key={tab.id}
                type="button"
                onClick={() => setActiveTerminalTabId(tab.id)}
                className={`px-3 py-1 rounded-md transition-colors flex items-center gap-1.5 cursor-pointer ${
                  isActive
                    ? 'bg-white dark:bg-[#202023] text-[#0f172a] dark:text-white shadow-xs font-semibold'
                    : 'text-[#64748b] dark:text-[#9ca3af] hover:text-[#0f172a] dark:hover:text-white'
                }`}
              >
                {tab.type === 'terminal' && <TerminalIcon className="w-3.5 h-3.5 text-[#38bdf8]" />}
                {tab.type === 'problems' && (
                  <div className="flex items-center gap-1">
                    <AlertCircle className="w-3.5 h-3.5 text-[#f59e0b]" />
                    {diagnostics.length > 0 && (
                      <span className="px-1 py-0.2 rounded-full bg-[#fef3c7] dark:bg-[#78350f] text-[#b45309] dark:text-[#fcd34d] text-[10px] font-bold">
                        {diagnostics.length}
                      </span>
                    )}
                  </div>
                )}
                {tab.type === 'output' && <CheckCircle2 className="w-3.5 h-3.5 text-[#10b981]" />}
                {tab.type === 'debug' && <Cpu className="w-3.5 h-3.5 text-[#a855f7]" />}
                <span>{tab.title}</span>
              </button>
            );
          })}
        </div>

        {/* Right Side: Shell tabs + Actions (Add Shell, Clear, Maximize, Close) */}
        <div className="flex items-center gap-2">
          {activeTerminalTabId === 'term-terminal-1' && (
            <div className="flex items-center bg-[#e2e8f0] dark:bg-[#252528] p-0.5 rounded-lg mr-1 max-w-[280px] overflow-x-auto">
              {shells.map(shell => {
                const isActive = activeShellId === shell.id;
                return (
                  <div
                    key={shell.id}
                    onClick={() => setActiveShellId(shell.id)}
                    className={`flex items-center gap-1 px-2 py-0.5 rounded-md text-[11px] font-medium cursor-pointer transition-all ${
                      isActive 
                        ? 'bg-white dark:bg-[#141416] text-[#0f172a] dark:text-white shadow-xs font-semibold' 
                        : 'text-[#64748b] dark:text-[#9ca3af] hover:text-[#0f172a] dark:hover:text-white'
                    }`}
                  >
                    <span className={`w-1.5 h-1.5 rounded-full ${
                      shell.status === 'connected' ? 'bg-[#10b981]' : shell.status === 'connecting' ? 'bg-[#f59e0b] animate-ping' : 'bg-[#ef4444]'
                    }`} />
                    <span className="truncate max-w-[90px]">{shell.name}</span>
                    {shells.length > 1 && (
                      <button
                        type="button"
                        onClick={(e) => handleCloseShell(shell.id, e)}
                        className="hover:text-[#ef4444] rounded p-0.5 transition-colors"
                        title="Kill Shell"
                      >
                        <X className="w-2.5 h-2.5" />
                      </button>
                    )}
                  </div>
                );
              })}

              <button
                type="button"
                onClick={handleAddNewShell}
                className="p-1 hover:bg-[#cbd5e1] dark:hover:bg-[#333336] rounded text-[#64748b] dark:text-[#9ca3af] hover:text-[#0f172a] dark:hover:text-white transition-colors ml-0.5"
                title="New PTY Terminal Session"
              >
                <Plus className="w-3 h-3" />
              </button>
            </div>
          )}

          {activeTerminalTabId === 'term-terminal-1' && (
            <>
              <button
                type="button"
                onClick={handleReconnect}
                className="p-1.5 hover:bg-[#f1f5f9] dark:hover:bg-[#252528] rounded text-[#64748b] dark:text-[#9ca3af] hover:text-[#0f172a] dark:hover:text-white transition-colors"
                title="Reconnect PTY"
              >
                <RefreshCw className="w-3.5 h-3.5" />
              </button>
              <button
                type="button"
                onClick={handleClearTerminal}
                className="p-1.5 hover:bg-[#f1f5f9] dark:hover:bg-[#252528] rounded text-[#64748b] dark:text-[#9ca3af] hover:text-[#0f172a] dark:hover:text-white transition-colors"
                title="Clear Terminal"
              >
                <Trash2 className="w-3.5 h-3.5" />
              </button>
            </>
          )}

          <button
            type="button"
            onClick={() => setIsMaximized(prev => !prev)}
            className="p-1.5 hover:bg-[#f1f5f9] dark:hover:bg-[#252528] rounded text-[#64748b] dark:text-[#9ca3af] hover:text-[#0f172a] dark:hover:text-white transition-colors"
            title={isMaximized ? 'Restore Drawer' : 'Maximize Drawer'}
          >
            {isMaximized ? <Minimize2 className="w-3.5 h-3.5" /> : <Maximize2 className="w-3.5 h-3.5" />}
          </button>

          <button
            type="button"
            onClick={() => setIsTerminalOpen(false)}
            className="p-1.5 hover:bg-[#fee2e2] dark:hover:bg-[#450a0a] rounded text-[#64748b] dark:text-[#9ca3af] hover:text-[#ef4444] transition-colors"
            title="Hide Terminal Drawer (⌘`)"
          >
            <X className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      {/* Main Terminal Viewport (XTerm / Problems / Output) */}
      <div className="flex-1 overflow-hidden relative bg-[#f8fafc] dark:bg-[#141416]">
        {activeTerminalTabId === 'term-terminal-1' ? (
          <div 
            ref={activeShellContainerRef} 
            className="w-full h-full p-2 overflow-hidden"
            style={{ padding: '8px 12px' }}
          />
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
            my-ade development stream running. Output buffer ready.
          </div>
        )}
      </div>

    </div>
  );
};
