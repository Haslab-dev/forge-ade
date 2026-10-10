import React, { useEffect, useMemo, useRef, useState } from 'react';
import {
  Search, FileCode, ArrowRight, Command as CommandIcon, AtSign, Hash,
  Clock, CornerDownLeft, Braces, Box, Shapes, ListOrdered, Type, Variable,
  Package, Replace, Loader2
} from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { useSessionStore } from '../../stores/sessionStore';
import { FileItem, WorkspaceSymbol, CommandPaletteMode, EditorTab } from '../../types';
import { ApiBridge } from '../../services/apiBridge';
import { scoreFilePath, scoreText, FuzzyMatch } from '../../lib/fuzzy';
import { getFileIcon } from '../../lib/file-icons';
import { showToast } from '../../lib/toast';

// Flatten files recursively (module scope — pure, stateless)
const getAllFiles = (items: FileItem[]): FileItem[] => {
  let list: FileItem[] = [];
  for (const item of items) {
    if (item.type === 'file') list.push(item);
    if (item.children) list = list.concat(getAllFiles(item.children));
  }
  return list;
};

const MODE_PREFIX: Record<CommandPaletteMode, string> = {
  files: '',
  commands: '>',
  symbols: '@',
  line: ':'
};

const MODE_PLACEHOLDER: Record<CommandPaletteMode, string> = {
  files: 'Search files by name (append > for commands, @ for symbols, : for line)...',
  commands: 'Type a command name...',
  symbols: 'Go to symbol in the active editor...',
  line: 'Go to line (e.g. 42 or 42:7)...'
};

const kindIcon = (kind: WorkspaceSymbol['kind'], className = 'w-4 h-4'): React.ReactNode => {
  const cls = `${className} shrink-0`;
  switch (kind) {
    case 'function':
    case 'method':
      return <Braces className={`${cls} text-trajectory-reasoning`} aria-hidden="true" />;
    case 'class':
    case 'struct':
      return <Box className={`${cls} text-warning`} aria-hidden="true" />;
    case 'interface':
      return <Shapes className={`${cls} text-trajectory-tool-result`} aria-hidden="true" />;
    case 'enum':
      return <ListOrdered className={`${cls} text-warning`} aria-hidden="true" />;
    case 'type':
      return <Type className={`${cls} text-trajectory-tool-result`} aria-hidden="true" />;
    case 'variable':
      return <Variable className={`${cls} text-trajectory-assistant`} aria-hidden="true" />;
    case 'package':
      return <Package className={`${cls} text-foreground-subtle`} aria-hidden="true" />;
    default:
      return <Hash className={`${cls} text-foreground-subtle`} aria-hidden="true" />;
  }
};

/** Renders `text` with fuzzy-match positions highlighted. */
const Highlighted: React.FC<{ text: string; positions?: number[] }> = ({ text, positions }) => {
  if (!positions || positions.length === 0) return <>{text}</>;
  const set = new Set(positions);
  return (
    <>
      {text.split('').map((ch, i) =>
        set.has(i) ? (
          <span key={i} className="text-accent-primary">{ch}</span>
        ) : (
          <React.Fragment key={i}>{ch}</React.Fragment>
        )
      )}
    </>
  );
};

interface PaletteItem {
  icon: React.ReactNode;
  title: string;
  titlePositions?: number[];
  detail?: string;
  kbd?: string;
  run: () => void;
}

type Row = { kind: 'header'; text: string } | { kind: 'hint'; text: string; spin?: boolean } | { kind: 'item'; item: PaletteItem };

const header = (text: string): Row => ({ kind: 'header', text });
const hint = (text: string, spin = false): Row => ({ kind: 'hint', text, spin });

export const CommandPaletteModal: React.FC = () => {
  const {
    isCommandPaletteOpen,
    setIsCommandPaletteOpen,
    commandPaletteMode,
    openFileInEditor,
    openFolder,
    switchSurface,
    setActiveActivity,
    openSettingsTab,
    setIsSSHModalOpen,
    setIsLeftSidebarOpen,
    setIsBottomTerminalOpen,
    goBack,
    goForward,
    canGoBack,
    canGoForward,
    openTabs,
    activeTabId,
    saveTabToDisk,
    files,
    recentFiles,
    activeWorkspacePath
  } = useWorkspace();

  const [query, setQuery] = useState('');
  const [selectedIndex, setSelectedIndex] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);

  // Prefix-driven mode (VS Code semantics): typing the prefix inside the input
  // switches modes; the store's commandPaletteMode only seeds the initial query.
  const mode: CommandPaletteMode = query.startsWith('>')
    ? 'commands'
    : query.startsWith('@')
      ? 'symbols'
      : query.startsWith(':')
        ? 'line'
        : 'files';
  const term = query.replace(/^[>@:]\s*/, '');

  const activeTab: EditorTab | null = useMemo(
    () => openTabs.find(t => t.id === activeTabId) || null,
    [openTabs, activeTabId]
  );
  const activeCodePath = activeTab && activeTab.type === 'code' && activeTab.filePath
    ? activeTab.filePath
    : null;

  // Reset per open (and when a shortcut re-opens with a different mode).
  useEffect(() => {
    if (isCommandPaletteOpen) {
      setQuery(MODE_PREFIX[commandPaletteMode] ?? '');
      setSelectedIndex(0);
      // Wait for the input to mount before focusing it.
      requestAnimationFrame(() => inputRef.current?.focus());
    }
  }, [isCommandPaletteOpen, commandPaletteMode]);

  const openFile = (path: string, line?: number, column?: number) => {
    openFileInEditor(path, line, column).catch(err => {
      showToast(err instanceof Error ? err.message : String(err), 'error');
    });
  };

  const saveActiveTab = () => {
    if (!activeTab || activeTab.type !== 'code') {
      showToast('No active code editor to save', 'error');
      return;
    }
    void saveTabToDisk(activeTab.fileId, activeTab.content ?? '');
  };

  const openFindInFiles = () => {
    switchSurface('editor');
    setIsLeftSidebarOpen(true);
    setActiveActivity('search');
  };

  // ── Data sources ────────────────────────────────────────────────────────────

  // The full-tree walk only runs while the palette is open — previously it
  // re-walked the whole workspace on every store change with the modal closed.
  const allFiles = useMemo(
    () => (isCommandPaletteOpen ? getAllFiles(files) : []),
    [isCommandPaletteOpen, files]
  );
  const filePathSet = useMemo(() => new Set(allFiles.map(f => f.path)), [allFiles]);

  // Recent files scoped to the active workspace, newest first.
  const wsRecents = useMemo(() => {
    if (!isCommandPaletteOpen || !activeWorkspacePath) return [];
    return recentFiles.filter(r => r.path.startsWith(activeWorkspacePath));
  }, [isCommandPaletteOpen, recentFiles, activeWorkspacePath]);

  // Backend filename search (respects ignore rules; covers files the local
  // tree may have missed). Debounced so fast typing doesn't hammer the IPC.
  const [remotePaths, setRemotePaths] = useState<string[]>([]);
  useEffect(() => {
    if (!isCommandPaletteOpen || mode !== 'files' || !term.trim()) {
      setRemotePaths([]);
      return;
    }
    let cancelled = false;
    const id = window.setTimeout(() => {
      ApiBridge.searchFilename(term.trim(), 100)
        .then(paths => { if (!cancelled) setRemotePaths((paths || []).filter(Boolean)); })
        .catch(() => { if (!cancelled) setRemotePaths([]); });
    }, 120);
    return () => {
      cancelled = true;
      window.clearTimeout(id);
    };
  }, [isCommandPaletteOpen, mode, term]);

  // @ mode, per-file outline of the active editor (workspace index).
  const [outline, setOutline] = useState<WorkspaceSymbol[]>([]);
  const [outlineLoading, setOutlineLoading] = useState(false);
  useEffect(() => {
    if (!isCommandPaletteOpen || mode !== 'symbols' || !activeCodePath) {
      setOutline([]);
      return;
    }
    let cancelled = false;
    setOutlineLoading(true);
    ApiBridge.getFileOutline(activeCodePath)
      .then(syms => { if (!cancelled) setOutline(syms); })
      .catch(() => { if (!cancelled) setOutline([]); })
      .finally(() => { if (!cancelled) setOutlineLoading(false); });
    return () => { cancelled = true; };
  }, [isCommandPaletteOpen, mode, activeCodePath]);

  // @ mode fallback: no active editor (or the file has no indexed symbols)
  // → search symbols across the whole workspace index.
  const [wsSymbols, setWsSymbols] = useState<WorkspaceSymbol[]>([]);
  const [wsSymbolsLoading, setWsSymbolsLoading] = useState(false);
  const useWorkspaceSymbols = mode === 'symbols' && (!activeCodePath || outline.length === 0);
  useEffect(() => {
    if (!isCommandPaletteOpen || !useWorkspaceSymbols || !term.trim()) {
      setWsSymbols([]);
      return;
    }
    let cancelled = false;
    setWsSymbolsLoading(true);
    const id = window.setTimeout(() => {
      ApiBridge.searchWorkspaceSymbols(term.trim(), 60)
        .then(syms => { if (!cancelled) setWsSymbols(syms); })
        .catch(() => { if (!cancelled) setWsSymbols([]); })
        .finally(() => { if (!cancelled) setWsSymbolsLoading(false); });
    }, 140);
    return () => {
      cancelled = true;
      window.clearTimeout(id);
      setWsSymbolsLoading(false);
    };
  }, [isCommandPaletteOpen, useWorkspaceSymbols, term]);

  // ── Commands (> mode) ───────────────────────────────────────────────────────
  const commands = useMemo(() => [
    { label: 'Go to File...', detail: 'Quick open by name (⌘P)', action: () => setQuery('') },
    { label: 'Go to Symbol in Editor...', detail: 'Outline of the active file (@)', action: () => setQuery('@') },
    { label: 'Go to Line/Column...', detail: 'Jump to a line in the active file (:)', action: () => setQuery(':') },
    { label: 'Search: Find in Files', detail: 'Project-wide content search (⌘⇧F)', action: openFindInFiles },
    { label: 'File: Save Active Editor', detail: 'Write the buffer to disk (⌘S in editor)', action: saveActiveTab },
    ...(canGoBack ? [{ label: 'Editor: Go Back', detail: 'Previous location', action: goBack }] : []),
    ...(canGoForward ? [{ label: 'Editor: Go Forward', detail: 'Next location', action: goForward }] : []),
    { label: 'View: Toggle Bottom Terminal', detail: 'Docked terminal panel', action: () => setIsBottomTerminalOpen(v => !v) },
    { label: 'View: Toggle Primary Sidebar', detail: 'Explorer / search / git sidebar (⌘B)', action: () => setIsLeftSidebarOpen(v => !v) },
    { label: 'New Task', detail: 'ForgeADE agent or an agent CLI (⌘N)', action: () => useSessionStore.getState().openNewTask() },
    { label: 'View: Switch to Agent Mode', detail: 'Tasks — ForgeADE agent chat & CLI terminals (⌘1)', action: () => switchSurface('agent') },
    { label: 'Workspace: Open Folder', detail: 'Pick directory from disk (⌘O)', action: () => openFolder() },
    { label: 'SSH: Connect to Remote Host...', detail: 'Connect to remote server via SSH / SFTP file explorer', action: () => { setIsSSHModalOpen(true); } },
    { label: 'View: Switch to Editor Mode', detail: 'Open code editor — carries the current folder (⌘2)', action: () => switchSurface('editor') },
    { label: 'Shell: Open Shell Sidebar', detail: 'Integrated shell sessions (Editor)', action: () => { switchSurface('editor'); setActiveActivity('shell'); } },
    { label: 'Workspace: Rebuild Symbol Index', detail: 'Re-index for symbol search & outlines', action: () => {
      void ApiBridge.reindexWorkspace().then(res => {
        showToast(res.built ? `Index rebuilt — ${res.symbols ?? 0} symbols` : 'Indexing unavailable for this workspace', res.built ? 'success' : 'error');
      });
    } },
    { label: 'Preferences: Agent CLIs', detail: 'Configure CLI agents (omp, opencode, …)', action: () => openSettingsTab('agents') },
    { label: 'Preferences: Privacy & Sharing Settings', detail: 'Share terminal activity & user edits', action: () => openSettingsTab('privacy') },
    { label: 'Preferences: Open MCPs & Skills', detail: 'Model Context Protocol servers and skills', action: () => openSettingsTab('mcps') },
    { label: 'Preferences: Open Rules (~/.my-ade/rules)', detail: 'Coding guidelines & instructions', action: () => openSettingsTab('rules') },
    // eslint-disable-next-line react-hooks/exhaustive-deps
  ], [canGoBack, canGoForward, goBack, goForward, setIsBottomTerminalOpen, setIsLeftSidebarOpen, switchSurface, openFolder, setIsSSHModalOpen, setActiveActivity, openSettingsTab, activeTab, saveTabToDisk]);

  // ── Row building ────────────────────────────────────────────────────────────
  const rows = useMemo<Row[]>(() => {
    if (mode === 'commands') {
      const q = term.trim();
      const scored = commands
        .map(c => ({ c, m: scoreText(q, c.label) }))
        .filter((x): x is { c: (typeof commands)[number]; m: FuzzyMatch } => !!x.m);
      scored.sort((a, b) => b.m.score - a.m.score);
      if (scored.length === 0) return [hint(`No commands matching "${q}"`)];
      return [
        header('Commands'),
        ...scored.map(({ c, m }) => ({
          kind: 'item' as const,
          item: {
            icon: <CommandIcon className="w-4 h-4 text-trajectory-reasoning shrink-0" aria-hidden="true" />,
            title: c.label,
            titlePositions: q ? m.positions : undefined,
            detail: c.detail,
            run: c.action
          }
        }))
      ];
    }

    if (mode === 'symbols') {
      const q = term.trim();
      if (outline.length > 0) {
        const scored = outline
          .map(s => ({ s, m: scoreText(q, s.name) }))
          .filter((x): x is { s: WorkspaceSymbol; m: FuzzyMatch } => !!x.m);
        scored.sort((a, b) => b.m.score - a.m.score || a.s.line - b.s.line);
        const rowsOut: Row[] = [header(`Symbols in ${activeCodePath?.split('/').pop() ?? 'editor'}`)];
        scored.forEach(({ s, m }) => {
          rowsOut.push({
            kind: 'item',
            item: {
              icon: kindIcon(s.kind),
              title: s.name,
              titlePositions: q ? m.positions : undefined,
              detail: [s.scope, `line ${s.line}`].filter(Boolean).join(' · '),
              run: () => openFile(activeCodePath!, s.line, s.column)
            }
          });
        });
        return rowsOut;
      }
      if (wsSymbolsLoading) return [hint('Searching workspace symbols...', true)];
      if (wsSymbols.length > 0) {
        const rowsOut: Row[] = [header('Workspace symbols')];
        wsSymbols.forEach(s => {
          rowsOut.push({
            kind: 'item',
            item: {
              icon: kindIcon(s.kind),
              title: s.name,
              detail: `${s.file.split('/').pop() ?? s.file} · line ${s.line}`,
              run: () => openFile(s.file, s.line, s.column)
            }
          });
        });
        return rowsOut;
      }
      if (outlineLoading) return [hint('Reading outline...', true)];
      return [hint(
        activeCodePath
          ? 'No symbols in this file — type to search the whole workspace'
          : 'Open a file to see its symbols, or type to search the workspace'
      )];
    }

    if (mode === 'line') {
      const m = term.match(/^(\d+)(?:[:,](\d+))?$/);
      if (!activeCodePath) return [hint('Open a file first, then use : to jump to a line')];
      if (!m) return [hint('Type a line number (e.g. 42 or 42:7)')];
      const line = parseInt(m[1], 10);
      const column = m[2] ? parseInt(m[2], 10) : undefined;
      return [{
        kind: 'item',
        item: {
          icon: <Replace className="w-4 h-4 text-trajectory-tool-result shrink-0" aria-hidden="true" />,
          title: `Go to line ${line}${column ? `, column ${column}` : ''}`,
          detail: activeCodePath,
          kbd: '↵',
          run: () => openFile(activeCodePath, line, column)
        }
      }];
    }

    // files (quick open)
    const q = term.trim();
    const out: Row[] = [];
    const pathSet = filePathSet;

    const recents = wsRecents
      .filter(r => (pathSet.size === 0 ? true : pathSet.has(r.path)))
      .filter(r => !q || scoreFilePath(q, r.path))
      .slice(0, 6);
    if (recents.length > 0) {
      out.push(header('Recently opened'));
      recents.forEach(r => {
        const m = q ? scoreFilePath(q, r.path) : null;
        const name = r.path.split('/').pop() || r.path;
        const nameOffset = r.path.length - name.length;
        out.push({
          kind: 'item',
          item: {
            icon: <Clock className="w-4 h-4 text-foreground-subtle shrink-0" aria-hidden="true" />,
            title: name,
            titlePositions: m ? m.positions.map(p => p - nameOffset) : undefined,
            detail: r.path.slice(0, Math.max(0, nameOffset - 1)),
            run: () => openFile(r.path)
          }
        });
      });
    }

    if (q) {
      const scored = allFiles
        .map(f => ({ f, m: scoreFilePath(q, f.path) }))
        .filter((x): x is { f: FileItem; m: FuzzyMatch } => !!x.m);
      scored.sort((a, b) => b.m.score - a.m.score);
      const seen = new Set(recents.map(r => r.path));
      const results = scored.filter(x => !seen.has(x.f.path)).slice(0, 40);
      const extraRemote = remotePaths.filter(
        p => !seen.has(p) && !pathSet.has(p) && !results.some(x => x.f.path === p)
      ).slice(0, 20);

      if (results.length > 0 || extraRemote.length > 0) {
        out.push(header('File results'));
        results.forEach(({ f, m }) => {
          const name = f.name;
          const nameOffset = f.path.length - name.length;
          out.push({
            kind: 'item',
            item: {
              icon: getFileIcon(name, 'w-4 h-4'),
              title: name,
              titlePositions: m.positions.map(p => p - nameOffset).filter(p => p >= 0),
              detail: f.path.slice(0, Math.max(0, nameOffset - 1)),
              run: () => openFile(f.path)
            }
          });
        });
        extraRemote.forEach(p => {
          const name = p.split('/').pop() || p;
          out.push({
            kind: 'item',
            item: {
              icon: getFileIcon(name, 'w-4 h-4'),
              title: name,
              detail: p.slice(0, Math.max(0, p.length - name.length - 1)),
              run: () => openFile(p)
            }
          });
        });
      }
    } else if (out.length === 0) {
      const top = allFiles.slice(0, 15);
      if (top.length > 0) {
        out.push(header('Workspace files'));
        top.forEach(f => {
          out.push({
            kind: 'item',
            item: {
              icon: getFileIcon(f.name, 'w-4 h-4'),
              title: f.name,
              detail: f.path.slice(0, Math.max(0, f.path.length - f.name.length - 1)),
              run: () => openFile(f.path)
            }
          });
        });
      }
    }

    if (out.length === 0) return [hint(`No files matching "${q}"`)];
    return out;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [mode, term, commands, allFiles, filePathSet, wsRecents, remotePaths, outline, outlineLoading, wsSymbols, wsSymbolsLoading, activeCodePath]);

  const selectableCount = rows.filter(r => r.kind === 'item').length;
  const clampedSelected = Math.min(selectedIndex, Math.max(0, selectableCount - 1));

  // Keep the highlighted option scrolled into view as arrows move it.
  useEffect(() => {
    const list = listRef.current;
    if (!list || !isCommandPaletteOpen) return;
    const el = list.querySelector<HTMLElement>(`[data-option-index="${clampedSelected}"]`);
    el?.scrollIntoView({ block: 'nearest' });
  }, [clampedSelected, isCommandPaletteOpen]);

  if (!isCommandPaletteOpen) return null;

  const runSelectedItem = () => {
    let seen = -1;
    for (const row of rows) {
      if (row.kind !== 'item') continue;
      seen++;
      if (seen === clampedSelected) {
        row.item.run();
        setIsCommandPaletteOpen(false);
        return;
      }
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Escape') {
      setIsCommandPaletteOpen(false);
    } else if (e.key === 'ArrowDown') {
      e.preventDefault();
      setSelectedIndex(prev => (prev + 1) % (selectableCount || 1));
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setSelectedIndex(prev => (prev - 1 + (selectableCount || 1)) % (selectableCount || 1));
    } else if (e.key === 'Enter') {
      e.preventDefault();
      runSelectedItem();
    }
  };

  const ModeIcon = mode === 'commands' ? CommandIcon : mode === 'symbols' ? AtSign : mode === 'line' ? Hash : Search;

  return (
    <div
      className="fixed inset-0 bg-black/50 backdrop-blur-xs flex items-start justify-center pt-20 z-50 p-4"
      onClick={() => setIsCommandPaletteOpen(false)}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Command palette"
        className="w-full max-w-xl bg-popover rounded-2xl shadow-2xl border border-border overflow-hidden animate-in fade-in zoom-in-95 duration-100"
        onClick={e => e.stopPropagation()}
      >
        {/* Search input bar */}
        <div className="p-3.5 border-b border-border flex items-center gap-2.5">
          <ModeIcon className="w-4 h-4 text-foreground-subtle" aria-hidden="true" />
          <input
            ref={inputRef}
            type="text"
            role="combobox"
            aria-expanded="true"
            aria-controls="command-palette-listbox"
            aria-activedescendant={selectableCount > 0 ? `command-palette-option-${clampedSelected}` : undefined}
            aria-label="Search files, symbols, or execute commands"
            value={query}
            onChange={e => {
              setQuery(e.target.value);
              setSelectedIndex(0);
            }}
            onKeyDown={handleKeyDown}
            placeholder={MODE_PLACEHOLDER[mode]}
            className="flex-1 bg-transparent border-0 text-sm text-foreground placeholder-foreground-subtle focus:outline-hidden font-sans"
            spellCheck={false}
            autoComplete="off"
          />
          <kbd className="text-ui-xs font-mono px-1.5 py-0.5 rounded bg-surface-hover text-foreground-subtle">
            ESC
          </kbd>
        </div>

        {/* Results list */}
        <div
          id="command-palette-listbox"
          ref={listRef}
          role="listbox"
          aria-label="Commands and files"
          className="max-h-80 overflow-y-auto p-1.5 space-y-0.5"
        >
          {rows.map((row, idx) => {
            if (row.kind === 'header') {
              return (
                <div
                  key={`h-${idx}`}
                  className="px-3 pt-2 pb-1 text-ui-xs font-semibold uppercase tracking-wide text-foreground-subtle"
                >
                  {row.text}
                </div>
              );
            }
            if (row.kind === 'hint') {
              return (
                <div key={`hint-${idx}`} className="p-6 text-center text-xs text-foreground-subtle flex items-center justify-center gap-2" role="status">
                  {row.spin && <Loader2 className="w-3.5 h-3.5 animate-spin" aria-hidden="true" />}
                  {row.text}
                </div>
              );
            }

            const itemIdx = rows.slice(0, idx + 1).filter(r => r.kind === 'item').length - 1;
            const isSelected = itemIdx === clampedSelected;
            return (
              <button
                key={`item-${idx}`}
                id={`command-palette-option-${itemIdx}`}
                data-option-index={itemIdx}
                type="button"
                role="option"
                aria-selected={isSelected}
                onClick={() => {
                  row.item.run();
                  setIsCommandPaletteOpen(false);
                }}
                className={`w-full text-left px-3 py-2 rounded-xl flex items-center justify-between text-xs transition-colors cursor-pointer ${
                  isSelected
                    ? 'bg-primary/10 text-primary'
                    : 'text-foreground-subtle hover:bg-surface-hover dark:hover:bg-surface-hover'
                }`}
              >
                <div className="flex items-center gap-2.5 min-w-0 flex-1 pr-2">
                  {row.item.icon ?? <FileCode className="w-4 h-4 text-trajectory-tool-result shrink-0" aria-hidden="true" />}
                  <div className="min-w-0 flex-1">
                    <p className="font-semibold text-foreground truncate">
                      <Highlighted text={row.item.title} positions={row.item.titlePositions} />
                    </p>
                    {row.item.detail && (
                      <p className="text-ui-xs text-foreground-subtle truncate">{row.item.detail}</p>
                    )}
                  </div>
                </div>

                {isSelected ? (
                  <CornerDownLeft className="w-4 h-4 text-primary dark:text-trajectory-tool-result shrink-0" aria-hidden="true" />
                ) : (
                  row.item.kbd && (
                    <kbd className="text-ui-xs font-mono px-1.5 py-0.5 rounded bg-surface-hover text-foreground-subtle shrink-0">
                      {row.item.kbd}
                    </kbd>
                  )
                )}
              </button>
            );
          })}
        </div>

        {/* Footer: navigation hints + mode switcher */}
        <div className="border-t border-border px-3 py-2 flex items-center gap-3 text-ui-xs text-foreground-subtle">
          <span className="flex items-center gap-1"><kbd className="font-mono">↑↓</kbd> navigate</span>
          <span className="flex items-center gap-1"><kbd className="font-mono">↵</kbd> open</span>
          <span className="hidden sm:flex items-center gap-1"><ArrowRight className="w-3 h-3" aria-hidden="true" /> switch mode:</span>
          <div className="ml-auto flex items-center gap-1">
            {([
              ['files', 'files'],
              ['commands', '>'],
              ['symbols', '@'],
              ['line', ':']
            ] as Array<[CommandPaletteMode, string]>).map(([m, label]) => (
              <button
                key={m}
                type="button"
                onClick={() => {
                  setQuery(MODE_PREFIX[m]);
                  setSelectedIndex(0);
                  requestAnimationFrame(() => inputRef.current?.focus());
                }}
                className={`px-1.5 py-0.5 rounded font-mono transition-colors cursor-pointer ${
                  mode === m
                    ? 'bg-primary/10 text-primary'
                    : 'hover:bg-surface-hover text-foreground-subtle'
                }`}
              >
                {label}
              </button>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
};
