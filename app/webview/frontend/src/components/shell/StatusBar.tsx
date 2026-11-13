import React, { useMemo } from 'react';
import { Settings, Eye, Server } from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { useLayoutStore } from '../../stores/layoutStore';
import { APP_VERSION, APP_BUILD, cn } from '../../lib/utils';

export const StatusBar: React.FC = () => {
  const {
    openSettingsTab,
    activeWorkspacePath,
    openTabs,
    activeTabId,
    selectedFile,
    setIsSSHModalOpen,
    setIsSSHQuickMenuOpen
  } = useWorkspace();
  const isFollowAgentMode = useLayoutStore((s) => s.isFollowAgentMode);
  const setIsFollowAgentMode = useLayoutStore((s) => s.setIsFollowAgentMode);

  const isRemote = activeWorkspacePath.startsWith('ssh://');
  const remoteInfo = useMemo(() => {
    if (!isRemote) return null;
    const rest = activeWorkspacePath.replace(/^ssh:\/\//, '');
    const slashIdx = rest.indexOf('/');
    const hostPart = slashIdx !== -1 ? rest.substring(0, slashIdx) : rest;
    const remotePath = slashIdx !== -1 ? rest.substring(slashIdx) : '/';
    const folder = remotePath.split('/').filter(Boolean).pop() || 'remote-root';
    const cleanHost = hostPart.includes('@')
      ? hostPart.split('@')[1]
      : hostPart.replace(/^ssh-/, '').split('-')[0];
    return {
      host: cleanHost || hostPart,
      remotePath,
      folder,
      rawHost: hostPart
    };
  }, [activeWorkspacePath, isRemote]);

  const folderName = isRemote
    ? remoteInfo?.folder || 'remote'
    : activeWorkspacePath.split('/').pop() || activeWorkspacePath;
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

      {/* Left side: workspace folder or Remote SSH pill */}
      <div className="flex items-center gap-2">
        {isRemote ? (
          <button
            type="button"
            onClick={() => setIsSSHQuickMenuOpen(true)}
            className="flex items-center gap-1.5 px-2 py-0.5 rounded bg-primary/20 hover:bg-primary/30 text-primary font-mono text-[11px] font-semibold transition-colors cursor-pointer border border-primary/30 shadow-xs"
            title={`SSH Remote: ${remoteInfo?.rawHost} (${remoteInfo?.remotePath}) — Click for remote actions`}
          >
            <Server className="w-3 h-3 text-primary shrink-0" />
            <span className="w-1.5 h-1.5 rounded-full bg-success animate-pulse shrink-0" />
            <span className="truncate max-w-[140px]">SSH: {remoteInfo?.host}</span>
            <span className="text-foreground-subtle font-normal truncate max-w-[120px]">
              ({remoteInfo?.folder})
            </span>
          </button>
        ) : (
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={() => setIsSSHModalOpen(true)}
              className="flex items-center gap-1 px-1.5 py-0.5 rounded text-foreground-subtlest hover:text-foreground transition-colors cursor-pointer hover:bg-surface-hover"
              title="Connect to Remote Server (SSH/SFTP)"
            >
              <Server className="w-3 h-3" />
            </button>
            {folderName && (
              <span className="font-mono text-ui-xs" title={activeWorkspacePath}>
                {folderName}
              </span>
            )}
          </div>
        )}
      </div>

      {/* Right side: language of the active file (real) + Version + Settings */}
      <div className="flex items-center gap-3 font-sans text-foreground-subtle">
        <button
          type="button"
          onClick={() => setIsFollowAgentMode(!isFollowAgentMode)}
          className={cn(
            "flex items-center gap-1 transition-colors cursor-pointer px-1.5 py-0.5 rounded",
            isFollowAgentMode ? "text-accent-primary font-medium bg-accent-primary/10" : "text-foreground-subtlest hover:text-foreground"
          )}
          title={isFollowAgentMode ? "Follow Agent Mode: ON (auto-jump editor to agent edits)" : "Follow Agent Mode: OFF"}
        >
          <Eye className="size-3" />
          <span className="text-[10px]">Follow Agent</span>
        </button>

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
