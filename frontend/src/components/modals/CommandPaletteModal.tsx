import React, { useEffect, useMemo, useRef, useState } from 'react';
import { Search, FileCode, Sparkles, ArrowRight } from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { useSessionStore } from '../../stores/sessionStore';
import { FileItem } from '../../types';

// Flatten files recursively (module scope — pure, stateless)
const getAllFiles = (items: FileItem[]): FileItem[] => {
  let list: FileItem[] = [];
  for (const item of items) {
    if (item.type === 'file') list.push(item);
    if (item.children) list = list.concat(getAllFiles(item.children));
  }
  return list;
};

export const CommandPaletteModal: React.FC = () => {
  const {
    isCommandPaletteOpen,
    setIsCommandPaletteOpen,
    openFileInEditor,
    openFolder,
    switchSurface,
    setActiveActivity,
    openSettingsTab,
    setIsSSHModalOpen,
    files
  } = useWorkspace();

  const [query, setQuery] = useState('');
  const [selectedIndex, setSelectedIndex] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);

  // Reset per open and move focus into the dialog.
  useEffect(() => {
    if (isCommandPaletteOpen) {
      setQuery('');
      setSelectedIndex(0);
      // Wait for the input to mount before focusing it.
      requestAnimationFrame(() => inputRef.current?.focus());
    }
  }, [isCommandPaletteOpen]);

  const actionItems = useMemo(() => [
    { type: 'action' as const, label: 'New Task', detail: 'ForgeADE agent or an agent CLI (⌘N)', action: () => useSessionStore.getState().openNewTask() },
    { type: 'action' as const, label: 'View: Switch to Agent Mode', detail: 'Tasks — ForgeADE agent chat & CLI terminals (⌘1)', action: () => switchSurface('agent') },
    { type: 'action' as const, label: 'Workspace: Open Folder', detail: 'Pick directory from disk (⌘O)', action: () => openFolder() },
    { type: 'action' as const, label: 'SSH: Connect to Remote Host...', detail: 'Connect to remote server via SSH / SFTP file explorer', action: () => { setIsCommandPaletteOpen(false); setIsSSHModalOpen(true); } },
    { type: 'action' as const, label: 'View: Switch to Editor Mode', detail: 'Open code editor — carries the current folder (⌘2)', action: () => switchSurface('editor') },
    { type: 'action' as const, label: 'Shell: Open Shell Sidebar', detail: 'Integrated shell sessions (Editor)', action: () => { switchSurface('editor'); setActiveActivity('shell'); } },
    { type: 'action' as const, label: 'Preferences: Agent CLIs', detail: 'Configure CLI agents (omp, opencode, …)', action: () => openSettingsTab('agents') },
    { type: 'action' as const, label: 'Preferences: Privacy & Sharing Settings', detail: 'Share terminal activity & user edits', action: () => openSettingsTab('privacy') },
    { type: 'action' as const, label: 'Preferences: Open MCPs & Skills', detail: 'Model Context Protocol servers and skills', action: () => openSettingsTab('mcps') },
    { type: 'action' as const, label: 'Preferences: Open Rules (~/.my-ade/rules)', detail: 'Coding guidelines & instructions', action: () => openSettingsTab('rules') },
  ], [switchSurface, openFolder, setIsCommandPaletteOpen, setIsSSHModalOpen, setActiveActivity, openSettingsTab]);

  // The full-tree walk only runs while the palette is open — previously it
  // re-walked the whole workspace on every store change with the modal closed.
  const items = useMemo(() => {
    if (!isCommandPaletteOpen) return actionItems;
    const fileItems = getAllFiles(files).map(f => ({
      type: 'file' as const,
      label: f.name,
      detail: f.path,
      action: () => openFileInEditor(f.path)
    }));
    return [...fileItems, ...actionItems];
  }, [isCommandPaletteOpen, files, actionItems, openFileInEditor]);

  const filtered = items.filter(it =>
    it.label.toLowerCase().includes(query.toLowerCase()) ||
    it.detail.toLowerCase().includes(query.toLowerCase())
  );

  const clampedSelected = Math.min(selectedIndex, Math.max(0, filtered.length - 1));

  // Keep the highlighted option scrolled into view as arrows move it.
  useEffect(() => {
    const list = listRef.current;
    if (!list || !isCommandPaletteOpen) return;
    const el = list.querySelector<HTMLElement>(`[data-option-index="${clampedSelected}"]`);
    el?.scrollIntoView({ block: 'nearest' });
  }, [clampedSelected, isCommandPaletteOpen]);

  if (!isCommandPaletteOpen) return null;

  const runItem = (index: number) => {
    const item = filtered[index];
    if (!item) return;
    item.action();
    setIsCommandPaletteOpen(false);
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Escape') {
      setIsCommandPaletteOpen(false);
    } else if (e.key === 'ArrowDown') {
      e.preventDefault();
      setSelectedIndex(prev => (prev + 1) % (filtered.length || 1));
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setSelectedIndex(prev => (prev - 1 + (filtered.length || 1)) % (filtered.length || 1));
    } else if (e.key === 'Enter') {
      e.preventDefault();
      runItem(clampedSelected);
    }
  };

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
          <Search className="w-4 h-4 text-foreground-subtle" aria-hidden="true" />
          <input
            ref={inputRef}
            type="text"
            role="combobox"
            aria-expanded="true"
            aria-controls="command-palette-listbox"
            aria-activedescendant={filtered.length > 0 ? `command-palette-option-${clampedSelected}` : undefined}
            aria-label="Search files and commands"
            value={query}
            onChange={e => {
              setQuery(e.target.value);
              setSelectedIndex(0);
            }}
            onKeyDown={handleKeyDown}
            placeholder="Search workspace files or execute action (⌘P)..."
            className="flex-1 bg-transparent border-0 text-sm text-foreground placeholder-foreground-subtle focus:outline-hidden font-sans"
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
          {filtered.map((item, idx) => {
            const isSelected = idx === clampedSelected;
            return (
              <button
                key={idx}
                id={`command-palette-option-${idx}`}
                data-option-index={idx}
                type="button"
                role="option"
                aria-selected={isSelected}
                onClick={() => {
                  item.action();
                  setIsCommandPaletteOpen(false);
                }}
                className={`w-full text-left px-3 py-2 rounded-xl flex items-center justify-between text-xs transition-colors cursor-pointer ${
                  isSelected
                    ? 'bg-primary/10 text-primary'
                    : 'text-foreground-subtle hover:bg-surface-hover dark:hover:bg-surface-hover'
                }`}
              >
                <div className="flex items-center gap-2.5 min-w-0 flex-1 pr-2">
                  {item.type === 'file' ? (
                    <FileCode className="w-4 h-4 text-info shrink-0" aria-hidden="true" />
                  ) : (
                    <Sparkles className="w-4 h-4 text-trajectory-reasoning shrink-0" aria-hidden="true" />
                  )}
                  <div className="min-w-0 flex-1">
                    <p className="font-semibold text-foreground truncate">{item.label}</p>
                    <p className="text-ui-xs text-foreground-subtle truncate">{item.detail}</p>
                  </div>
                </div>

                {isSelected && (
                  <ArrowRight className="w-4 h-4 text-primary dark:text-info shrink-0" aria-hidden="true" />
                )}
              </button>
            );
          })}

          {filtered.length === 0 && (
            <div className="p-8 text-center text-xs text-foreground-subtle" role="status">
              No files or actions matching &quot;{query}&quot;
            </div>
          )}
        </div>
      </div>
    </div>
  );
};
