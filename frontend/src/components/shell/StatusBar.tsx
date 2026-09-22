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
      <footer className="h-[26px] min-h-[26px] bg-background border-t border-border flex items-center justify-between px-3 text-xs text-foreground-subtle select-none z-20 font-sans transition-colors">
        <div className="flex items-center gap-2">
          <span className="font-semibold text-foreground">{activeAgent.name}</span>
          {folderName && (
            <>
              <span className="text-foreground-subtlest">•</span>
              <span className="text-foreground-subtle font-mono text-[11px]">{folderName}</span>
            </>
          )}
        </div>
        <button
          type="button"
          onClick={() => openSettingsTab('agents')}
          className="hover:text-foreground transition-colors flex items-center gap-1.5 cursor-pointer text-xs"
          title="Settings (⌘,)"
        >
          <Settings className="w-3 h-3 text-success" />
          <span>Settings</span>
        </button>
      </footer>
    );
  }

  return (
    <footer className="h-[24px] min-h-[24px] bg-background border-t border-border text-foreground-subtle flex items-center justify-between px-3 text-[11px] font-sans select-none z-20 transition-colors">

      {/* Left side: workspace folder (real value only) */}
      <div className="flex items-center gap-3">
        {folderName && (
          <span className="font-mono text-[11px]" title={activeWorkspacePath}>
            {folderName}
          </span>
        )}
      </div>

      {/* Right side: language of the active file (real) + Settings */}
      <div className="flex items-center gap-3 font-sans text-foreground-subtle">
        {curName && (
          <span className="flex items-center gap-1 hover:text-foreground transition-colors">
            <span className="text-foreground-subtlest font-mono font-bold text-[10px]">{'{}'}</span>
            <span>{getLanguageLabel()}</span>
          </span>
        )}
        <button
          type="button"
          onClick={() => openSettingsTab('agents')}
          className="hover:text-foreground transition-colors cursor-pointer flex items-center gap-1.5"
          title="Settings (⌘,)"
        >
          <Settings className="w-3 h-3 text-success" />
          <span>Settings</span>
        </button>
      </div>

    </footer>
  );
};
