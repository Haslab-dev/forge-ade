import React from 'react';
import { Settings } from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';

export const StatusBar: React.FC = () => {
  const { mode, openSettingsTab, activeAgent, activeWorkspacePath, openTabs, activeTabId, selectedFile } = useWorkspace();

  const folderName = activeWorkspacePath.split('/').pop() || activeWorkspacePath;
  const activeTab = openTabs.find(t => t.id === activeTabId);
  const curName = activeTab?.fileName || selectedFile?.name || '';

  const getLanguageLabel = () => {
    if (curName.endsWith('.php')) return 'PHP';
    if (curName.endsWith('.md')) return 'Markdown';
    if (curName.endsWith('.ts') || curName.endsWith('.tsx')) return 'TypeScript';
    if (curName.endsWith('.js') || curName.endsWith('.jsx')) return 'JavaScript';
    if (curName.endsWith('.json')) return 'JSON';
    if (curName.endsWith('.html')) return 'HTML';
    if (curName.endsWith('.css')) return 'CSS';
    if (curName.endsWith('.go')) return 'Go';
    if (curName.endsWith('.py')) return 'Python';
    if (curName.endsWith('.rs')) return 'Rust';
    return 'Plain Text';
  };

  if (mode === 'agent') {
    return (
      <footer className="h-[26px] min-h-[26px] bg-white dark:bg-[#181818] border-t border-[#e5e7eb] dark:border-[#2b2b2b] flex items-center justify-between px-3 text-xs text-[#6b7280] dark:text-[#9ca3af] select-none z-20 font-sans">
        <div className="flex items-center gap-2">
          <span className="font-semibold text-[#111827] dark:text-[#e5e7eb]">{activeAgent.name}</span>
          {folderName && (
            <>
              <span className="text-[#9ca3af]">•</span>
              <span className="text-[#6b7280] dark:text-[#9ca3af] font-mono text-[11px]">{folderName}</span>
            </>
          )}
        </div>
        <button
          type="button"
          onClick={() => openSettingsTab('agents')}
          className="hover:text-[#111827] dark:hover:text-white transition-colors flex items-center gap-1.5 cursor-pointer text-xs"
          title="Settings (⌘,)"
        >
          <Settings className="w-3 h-3 text-[#3b82f6]" />
          <span>Settings</span>
        </button>
      </footer>
    );
  }

  return (
    <footer className="h-[24px] min-h-[24px] bg-[#ffffff] dark:bg-[#181818] border-t border-[#e5e7eb] dark:border-[#2b2b2b] text-[#475569] dark:text-[#9ca3af] flex items-center justify-between px-3 text-[11px] font-sans select-none z-20">

      {/* Left side: workspace folder (real value only) */}
      <div className="flex items-center gap-3">
        {folderName && (
          <span className="font-mono text-[11px]" title={activeWorkspacePath}>
            {folderName}
          </span>
        )}
      </div>

      {/* Right side: language of the active file (real) + Settings */}
      <div className="flex items-center gap-3 font-sans text-[#64748b] dark:text-[#94a3b8]">
        {curName && (
          <span className="flex items-center gap-1 hover:text-[#111827] dark:hover:text-white transition-colors">
            <span className="text-[#9ca3af] font-mono font-bold text-[10px]">{'{}'}</span>
            <span>{getLanguageLabel()}</span>
          </span>
        )}
        <button
          type="button"
          onClick={() => openSettingsTab('agents')}
          className="hover:text-[#111827] dark:hover:text-white transition-colors cursor-pointer flex items-center gap-1.5"
          title="Settings (⌘,)"
        >
          <Settings className="w-3 h-3" />
          <span>Settings</span>
        </button>
      </div>

    </footer>
  );
};
