import React from 'react';
import { Settings } from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { APP_VERSION, APP_BUILD } from '../../lib/utils';

export const StatusBar: React.FC = () => {
  const { openSettingsTab, activeWorkspacePath, openTabs, activeTabId, selectedFile } = useWorkspace();

  const folderName = activeWorkspacePath.split('/').pop() || activeWorkspacePath;
  const activeTab = openTabs.find(t => t.id === activeTabId);
  const curName = activeTab?.fileName || selectedFile?.name || '';

  const getLanguageLabel = () => {
    if (curName.endsWith('.sqlite') || curName.endsWith('.sqlite3') || curName.endsWith('.db') || curName.endsWith('.db3')) return 'SQLite Database';
    if (curName.endsWith('.duckdb')) return 'DuckDB Database';
    if (curName.endsWith('.rocksdb') || curName.endsWith('.ldb') || curName.endsWith('.sst')) return 'RocksDB Store';
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

  return (
    <footer className="h-[24px] min-h-[24px] bg-background border-t border-border text-foreground-subtle flex items-center justify-between px-3 text-ui-xs font-sans select-none z-20 transition-colors">

      {/* Left side: workspace folder (real value only) */}
      <div className="flex items-center gap-3">
        {folderName && (
          <span className="font-mono text-ui-xs" title={activeWorkspacePath}>
            {folderName}
          </span>
        )}
      </div>

      {/* Right side: language of the active file (real) + Version + Settings */}
      <div className="flex items-center gap-3 font-sans text-foreground-subtle">
        {curName && (
          <span className="flex items-center gap-1 hover:text-foreground transition-colors">
            <span className="text-foreground-subtlest font-mono font-bold text-ui-xs">{'{}'}</span>
            <span>{getLanguageLabel()}</span>
          </span>
        )}
        <span
          title={`ForgeADE v${APP_VERSION} (Build ${APP_BUILD})`}
          className="font-mono text-ui-xs text-foreground-subtle hover:text-foreground transition-colors cursor-default select-none font-medium"
        >
          v{APP_VERSION}
          <span className="text-[10px] text-foreground-subtle/70 font-normal ml-1">#{APP_BUILD}</span>
        </span>
        <button
          type="button"
          onClick={() => openSettingsTab('general')}
          className="hover:text-foreground transition-colors cursor-pointer flex items-center gap-1.5"
          title="Settings (⌘,)"
          aria-label="Open settings"
        >
          <Settings className="w-3 h-3 text-success" />
          <span>Settings</span>
        </button>
      </div>

    </footer>
  );
};
