import React, { useState } from 'react';
import { Search, FileCode, Sparkles, FolderOpen, Terminal, Layers, Settings, ArrowRight } from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { FileItem } from '../../types';

export const CommandPaletteModal: React.FC = () => {
  const { 
    isCommandPaletteOpen, 
    setIsCommandPaletteOpen, 
    openFileInEditor, 
    openFolder,
    setMode, 
    createNewSession, 
    setIsTerminalOpen,
    isTerminalOpen,
    openSettingsTab,
    files,
    activeWorkspacePath
  } = useWorkspace();

  const [query, setQuery] = useState('');
  const [selectedIndex, setSelectedIndex] = useState(0);

  if (!isCommandPaletteOpen) return null;

  // Flatten files recursively
  const getAllFiles = (items: FileItem[]): FileItem[] => {
    let list: FileItem[] = [];
    for (const item of items) {
      if (item.type === 'file') list.push(item);
      if (item.children) list = list.concat(getAllFiles(item.children));
    }
    return list;
  };

  const dynamicFileItems = getAllFiles(files).map(f => ({
    type: 'file' as const,
    label: f.name,
    detail: f.path,
    action: () => openFileInEditor(f.path)
  }));

  const actionItems = [
    { type: 'action' as const, label: 'Workspace: Open Folder', detail: 'Pick directory from disk (⌘O)', action: () => openFolder() },
    { type: 'action' as const, label: 'Agent: New Space Session', detail: 'Start new reasoning session (⌥T)', action: () => { setMode('agent'); createNewSession(); } },
    { type: 'action' as const, label: 'View: Switch to Editor Mode', detail: 'Open code editor & terminal (⌘2)', action: () => setMode('editor') },
    { type: 'action' as const, label: 'View: Switch to Agent Mode', detail: 'Open reasoning & telemetry stream (⌘1)', action: () => setMode('agent') },
    { type: 'action' as const, label: 'Terminal: Toggle Integrated Terminal Drawer', detail: 'Ghostty Native Terminal', action: () => setIsTerminalOpen(!isTerminalOpen) },
    { type: 'action' as const, label: 'Preferences: Open Agent Registry & ACP', detail: 'Manage My-ADE Internal, Pi, OhMyPi, OpenCode', action: () => openSettingsTab('agents') },
    { type: 'action' as const, label: 'Preferences: Privacy & Sharing Settings', detail: 'Share terminal activity & user edits', action: () => openSettingsTab('privacy') },
    { type: 'action' as const, label: 'Preferences: Open MCPs & Skills', detail: 'Model Context Protocol servers and skills', action: () => openSettingsTab('mcps') },
    { type: 'action' as const, label: 'Preferences: Open Rules (~/.my-ade/rules)', detail: 'Coding guidelines & instructions', action: () => openSettingsTab('rules') },
    { type: 'action' as const, label: 'Preferences: Open Sub-Agents', detail: 'Sub-agent task delegation', action: () => openSettingsTab('subagents') }
  ];

  const items = [...dynamicFileItems, ...actionItems];

  const filtered = items.filter(it => 
    it.label.toLowerCase().includes(query.toLowerCase()) || 
    it.detail.toLowerCase().includes(query.toLowerCase())
  );

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
      if (filtered[selectedIndex]) {
        filtered[selectedIndex].action();
        setIsCommandPaletteOpen(false);
      }
    }
  };

  return (
    <div 
      className="fixed inset-0 bg-black/50 backdrop-blur-xs flex items-start justify-center pt-20 z-50 p-4"
      onClick={() => setIsCommandPaletteOpen(false)}
    >
      <div 
        className="w-full max-w-xl bg-white dark:bg-[#1f1f22] rounded-2xl shadow-2xl border border-[#e5e7eb] dark:border-[#383838] overflow-hidden animate-in fade-in zoom-in-95 duration-100"
        onClick={e => e.stopPropagation()}
      >
        {/* Search input bar */}
        <div className="p-3.5 border-b border-[#f3f4f6] dark:border-[#2f2f31] flex items-center gap-2.5">
          <Search className="w-4 h-4 text-[#9ca3af]" />
          <input
            type="text"
            autoFocus
            value={query}
            onChange={e => {
              setQuery(e.target.value);
              setSelectedIndex(0);
            }}
            onKeyDown={handleKeyDown}
            placeholder="Search workspace files or execute action (⌘P)..."
            className="flex-1 bg-transparent border-0 text-sm text-[#111827] dark:text-white placeholder-[#9ca3af] focus:outline-hidden font-sans"
          />
          <kbd className="text-[10px] font-mono px-1.5 py-0.5 rounded bg-[#f3f4f6] dark:bg-[#2c2c2f] text-[#6b7280] dark:text-[#9ca3af]">
            ESC
          </kbd>
        </div>

        {/* Results list */}
        <div className="max-h-80 overflow-y-auto p-1.5 space-y-0.5">
          {filtered.map((item, idx) => {
            const isSelected = idx === selectedIndex;
            return (
              <button
                key={idx}
                type="button"
                onClick={() => {
                  item.action();
                  setIsCommandPaletteOpen(false);
                }}
                className={`w-full text-left px-3 py-2 rounded-xl flex items-center justify-between text-xs transition-colors cursor-pointer ${
                  isSelected
                    ? 'bg-[#eff6ff] dark:bg-[#282d3b] text-[#1e40af] dark:text-[#93c5fd]'
                    : 'text-[#374151] dark:text-[#d1d5db] hover:bg-[#f3f4f6] dark:hover:bg-[#2a2a2d]'
                }`}
              >
                <div className="flex items-center gap-2.5 min-w-0 flex-1 pr-2">
                  {item.type === 'file' ? (
                    <FileCode className="w-4 h-4 text-[#3b82f6] shrink-0" />
                  ) : (
                    <Sparkles className="w-4 h-4 text-[#a855f7] shrink-0" />
                  )}
                  <div className="min-w-0 flex-1">
                    <p className="font-semibold text-[#111827] dark:text-white truncate">{item.label}</p>
                    <p className="text-[11px] text-[#6b7280] dark:text-[#9ca3af] truncate">{item.detail}</p>
                  </div>
                </div>

                {isSelected && (
                  <ArrowRight className="w-4 h-4 text-[#2563eb] dark:text-[#60a5fa] shrink-0" />
                )}
              </button>
            );
          })}

          {filtered.length === 0 && (
            <div className="p-8 text-center text-xs text-[#9ca3af]">
              No files or actions matching &quot;{query}&quot;
            </div>
          )}
        </div>
      </div>
    </div>
  );
};
