import React, { useState } from 'react';
import { Plus, FileCode, Folder, MessageSquare, GitBranch, X, Search } from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { FileItem } from '../../types';

interface Props {
  isOpen: boolean;
  onClose: () => void;
  onSelectContext: (item: string) => void;
}

export const ContextSelectorModal: React.FC<Props> = ({ isOpen, onClose, onSelectContext }) => {
  const { files, sessions, activeWorkspacePath } = useWorkspace();
  const [query, setQuery] = useState('');

  if (!isOpen) return null;

  // Flatten files and folders dynamically
  const collectItems = (items: FileItem[]): { type: 'file' | 'folder'; label: string; path: string; desc: string }[] => {
    let list: { type: 'file' | 'folder'; label: string; path: string; desc: string }[] = [];
    for (const item of items) {
      list.push({
        type: item.type === 'folder' ? 'folder' : 'file',
        label: item.name,
        path: item.path,
        desc: item.path
      });
      if (item.children) {
        list = list.concat(collectItems(item.children));
      }
    }
    return list;
  };

  const dynamicFileContexts = collectItems(files);

  const dynamicSessionContexts = sessions.map(s => ({
    type: 'chat' as const,
    label: s.title,
    path: s.id,
    desc: `Agent session (${s.model})`
  }));

  const gitContext = [
    { type: 'file' as const, label: 'git:diff', path: 'git:diff', desc: 'Current workspace git diff changes' },
    { type: 'file' as const, label: 'git:status', path: 'git:status', desc: 'Git status and branch state' }
  ];

  const allContexts = [...gitContext, ...dynamicFileContexts, ...dynamicSessionContexts];

  const filtered = allContexts.filter(c => 
    c.label.toLowerCase().includes(query.toLowerCase()) || 
    c.desc.toLowerCase().includes(query.toLowerCase())
  );

  return (
    <div className="fixed inset-0 bg-black/40 backdrop-blur-xs flex items-center justify-center z-50 p-4">
      <div className="bg-white dark:bg-[#202022] w-full max-w-md rounded-2xl shadow-2xl border border-[#e5e7eb] dark:border-[#383838] overflow-hidden animate-in fade-in zoom-in-95 duration-150">
        {/* Header */}
        <div className="p-3.5 border-b border-[#f3f4f6] dark:border-[#2f2f31] flex items-center justify-between">
          <div className="flex items-center gap-2">
            <div className="w-6 h-6 rounded-lg bg-[#eff6ff] dark:bg-[#1e293b] flex items-center justify-center text-[#2563eb] dark:text-[#60a5fa]">
              <Plus className="w-3.5 h-3.5" />
            </div>
            <h3 className="text-xs font-semibold text-[#111827] dark:text-white">
              Add Context to Agent ({activeWorkspacePath || 'Workspace'})
            </h3>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="p-1 text-[#9ca3af] hover:text-[#4b5563] dark:hover:text-white rounded-md transition-colors"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Search */}
        <div className="p-3 border-b border-[#f3f4f6] dark:border-[#2f2f31] bg-[#f9fafb] dark:bg-[#1a1a1c]">
          <div className="relative">
            <Search className="w-3.5 h-3.5 text-[#9ca3af] absolute left-3 top-2.5" />
            <input
              type="text"
              autoFocus
              placeholder="Search files, folders, or @chats..."
              value={query}
              onChange={e => setQuery(e.target.value)}
              className="w-full pl-9 pr-3 py-1.5 text-xs bg-white dark:bg-[#262628] border border-[#e5e7eb] dark:border-[#383838] rounded-xl focus:outline-hidden focus:ring-1 focus:ring-[#2563eb] text-[#111827] dark:text-white placeholder-[#9ca3af]"
            />
          </div>
        </div>

        {/* Context List */}
        <div className="p-2 max-h-64 overflow-y-auto space-y-1">
          {filtered.length === 0 ? (
            <div className="p-4 text-center text-xs text-[#9ca3af]">No matching files or context items</div>
          ) : (
            filtered.map((item, idx) => (
              <button
                key={idx}
                type="button"
                onClick={() => {
                  onSelectContext(`@${item.label}`);
                  onClose();
                }}
                className="w-full text-left p-2 rounded-xl hover:bg-[#f3f4f6] dark:hover:bg-[#2a2a2c] flex items-center justify-between text-xs transition-colors group"
              >
                <div className="flex items-center gap-2.5 min-w-0 flex-1 pr-2">
                  {item.type === 'file' && <FileCode className="w-4 h-4 text-[#3b82f6] shrink-0" />}
                  {item.type === 'folder' && <Folder className="w-4 h-4 text-[#f59e0b] shrink-0" />}
                  {item.type === 'chat' && <MessageSquare className="w-4 h-4 text-[#10b981] shrink-0" />}
                  <div className="min-w-0 flex-1">
                    <p className="font-medium text-[#111827] dark:text-white truncate">{item.label}</p>
                    <p className="text-[11px] text-[#6b7280] dark:text-[#9ca3af] truncate">{item.desc}</p>
                  </div>
                </div>
                <span className="text-[10px] px-2 py-0.5 rounded bg-[#f3f4f6] dark:bg-[#333336] text-[#6b7280] dark:text-[#9ca3af] group-hover:bg-[#2563eb] group-hover:text-white transition-colors shrink-0">
                  Insert
                </span>
              </button>
            ))
          )}
        </div>
      </div>
    </div>
  );
};
