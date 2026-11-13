import React, { useState, useEffect } from 'react';
import {
  Server,
  Terminal,
  FolderOpen,
  Upload,
  RefreshCw,
  Zap,
  Unplug,
  X,
  ArrowRight,
  Folder,
  Globe,
  Loader2,
  CheckCircle2,
  AlertCircle
} from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { ApiBridge } from '../../services/apiBridge';
import { showToast } from '../../lib/toast';
import type { SSHConfig, SSHConnectionStatus } from '../../lib/wails';

interface Props {
  isOpen: boolean;
  onClose: () => void;
}

const STORAGE_KEY = 'forge_saved_ssh_configs';

export const SSHQuickMenuModal: React.FC<Props> = ({ isOpen, onClose }) => {
  const {
    activeWorkspacePath,
    openFolder,
    refreshFiles,
    openTerminalTab,
    openRemoteUploader,
    disconnectSSH,
    setIsSSHModalOpen
  } = useWorkspace();

  const [activeConn, setActiveConn] = useState<SSHConnectionStatus | null>(null);
  const [isChangingDir, setIsChangingDir] = useState(false);
  const [newDirPath, setNewDirPath] = useState('');
  const [isReconnecting, setIsReconnecting] = useState(false);

  const isRemote = activeWorkspacePath.startsWith('ssh://');
  const rest = isRemote ? activeWorkspacePath.replace(/^ssh:\/\//, '') : '';
  const slashIdx = rest.indexOf('/');
  const connId = slashIdx !== -1 ? rest.substring(0, slashIdx) : rest;
  const currentRemotePath = slashIdx !== -1 ? rest.substring(slashIdx) : '/';

  // Load active connection status
  useEffect(() => {
    if (!isOpen || !isRemote) return;
    setIsChangingDir(false);
    setNewDirPath(currentRemotePath);

    ApiBridge.listSSHConnections()
      .then((conns) => {
        const found = conns.find((c) => c.id === connId) || (conns.length > 0 ? conns[0] : null);
        setActiveConn(found || null);
      })
      .catch((err) => {
        console.warn('Failed to fetch SSH connections:', err);
      });
  }, [isOpen, isRemote, connId, currentRemotePath]);

  if (!isOpen || !isRemote) return null;

  const hostDisplay = activeConn ? `${activeConn.user}@${activeConn.host}:${activeConn.port}` : connId;
  const labelDisplay = activeConn?.label || hostDisplay;

  const handleOpenTerminal = () => {
    onClose();
    void openTerminalTab(undefined, activeWorkspacePath);
  };

  const handleOpenUploader = () => {
    onClose();
    openRemoteUploader(activeWorkspacePath);
  };

  const handleRefresh = async () => {
    onClose();
    try {
      await refreshFiles();
      showToast('Remote files refreshed', 'success');
    } catch (err: any) {
      showToast(`Refresh failed: ${err?.message || 'network error'}`, 'error');
    }
  };

  const handleChangeDir = async (e: React.FormEvent) => {
    e.preventDefault();
    const clean = newDirPath.trim();
    if (!clean) return;
    const targetDir = clean.startsWith('/') ? clean : `/${clean}`;
    const newUri = `ssh://${connId}${targetDir}`;
    onClose();
    try {
      await openFolder(newUri);
      showToast(`Switched remote directory to ${targetDir}`, 'success');
    } catch (err: any) {
      showToast(`Failed to switch directory: ${err?.message || 'error'}`, 'error');
    }
  };

  const handleQuickDir = (path: string) => {
    setNewDirPath(path);
  };

  const handleReconnect = async () => {
    setIsReconnecting(true);
    try {
      let cfg: SSHConfig | null = null;
      try {
        const stored = localStorage.getItem(STORAGE_KEY);
        if (stored) {
          const list: SSHConfig[] = JSON.parse(stored);
          cfg = list.find((c) => c.id === connId || (activeConn && c.host === activeConn.host && c.user === activeConn.user)) || null;
        }
      } catch {}

      if (!cfg && activeConn) {
        cfg = {
          id: activeConn.id,
          label: activeConn.label,
          host: activeConn.host,
          port: activeConn.port,
          user: activeConn.user,
          authType: 'agent',
          remotePath: currentRemotePath
        };
      }

      if (cfg) {
        const res = await ApiBridge.openSSHWorkspace({ ...cfg, remotePath: currentRemotePath });
        if (res && res.connected) {
          showToast(`Reconnected to ${cfg.host}`, 'success');
          await refreshFiles();
          onClose();
        } else {
          showToast(`Reconnection failed: ${res?.error || 'unknown error'}`, 'error');
        }
      } else {
        showToast('Saved config not found. Opening connection modal.', 'info');
        onClose();
        setIsSSHModalOpen(true);
      }
    } catch (err: any) {
      showToast(`Reconnection error: ${err?.message || 'failed'}`, 'error');
    } finally {
      setIsReconnecting(false);
    }
  };

  const handleDisconnect = async () => {
    onClose();
    await disconnectSSH();
    showToast(`Disconnected from ${hostDisplay}`, 'info');
  };

  const handleSwitchHost = () => {
    onClose();
    setIsSSHModalOpen(true);
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4 animate-in fade-in duration-150 select-none"
      onClick={onClose}
    >
      <div
        className="w-full max-w-lg bg-card border border-border rounded-xl shadow-2xl flex flex-col overflow-hidden text-foreground font-sans animate-in zoom-in-95 duration-150"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="px-5 py-4 border-b border-border bg-surface-subtle/40 flex items-center justify-between">
          <div className="flex items-center gap-3 min-w-0">
            <div className="w-9 h-9 rounded-lg bg-primary/15 text-primary flex items-center justify-center shrink-0">
              <Server className="w-5 h-5" />
            </div>
            <div className="min-w-0">
              <div className="flex items-center gap-2">
                <h3 className="text-sm font-semibold truncate">{labelDisplay}</h3>
                <span className="flex items-center gap-1 text-[10px] font-mono font-medium px-1.5 py-0.5 rounded bg-success/15 text-success">
                  <span className="w-1.5 h-1.5 rounded-full bg-success animate-pulse" />
                  Connected
                </span>
              </div>
              <p className="text-xs text-foreground-subtle font-mono truncate" title={currentRemotePath}>
                {hostDisplay} &bull; {currentRemotePath}
              </p>
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="p-1.5 rounded-lg hover:bg-surface-hover text-foreground-subtle hover:text-foreground transition-colors cursor-pointer"
            aria-label="Close"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Change Directory Sub-panel */}
        {isChangingDir ? (
          <form onSubmit={handleChangeDir} className="p-4 border-b border-border bg-surface-subtle/20 space-y-3">
            <div className="flex items-center justify-between">
              <span className="text-xs font-semibold text-foreground flex items-center gap-1.5">
                <Folder className="w-3.5 h-3.5 text-primary" /> Change Remote Directory
              </span>
              <button
                type="button"
                onClick={() => setIsChangingDir(false)}
                className="text-xs text-foreground-subtle hover:text-foreground"
              >
                Cancel
              </button>
            </div>
            <div className="flex items-center gap-2">
              <input
                type="text"
                value={newDirPath}
                onChange={(e) => setNewDirPath(e.target.value)}
                placeholder="/var/www or /home/user/project"
                className="flex-1 bg-background border border-border rounded-lg px-3 py-1.5 text-xs font-mono text-foreground focus:outline-none focus:border-primary"
                autoFocus
              />
              <button
                type="submit"
                className="px-3 py-1.5 bg-primary text-primary-foreground text-xs font-medium rounded-lg hover:bg-primary/90 flex items-center gap-1 cursor-pointer"
              >
                Switch <ArrowRight className="w-3.5 h-3.5" />
              </button>
            </div>
            {/* Quick folder shortcuts */}
            <div className="flex items-center gap-1.5 pt-1 overflow-x-auto text-[11px]">
              <span className="text-foreground-subtle text-ui-xs">Quick jump:</span>
              {['/var/www', '/home', '/etc', '/opt', '/root', '/'].map((p) => (
                <button
                  key={p}
                  type="button"
                  onClick={() => handleQuickDir(p)}
                  className="px-2 py-0.5 rounded bg-surface-subtle hover:bg-surface-hover text-foreground-subtle hover:text-foreground font-mono text-[10px] cursor-pointer"
                >
                  {p}
                </button>
              ))}
            </div>
          </form>
        ) : null}

        {/* Action List */}
        <div className="p-2 divide-y divide-border/50 overflow-y-auto max-h-[60vh]">
          <div className="py-1">
            {/* Open Terminal */}
            <button
              type="button"
              onClick={handleOpenTerminal}
              className="w-full px-3 py-2.5 rounded-lg hover:bg-surface-hover flex items-center justify-between text-left transition-colors cursor-pointer group"
            >
              <div className="flex items-center gap-3">
                <div className="w-8 h-8 rounded-lg bg-surface-subtle flex items-center justify-center text-primary group-hover:bg-primary/15 transition-colors">
                  <Terminal className="w-4 h-4" />
                </div>
                <div>
                  <div className="text-xs font-medium text-foreground">Open Remote Terminal</div>
                  <div className="text-[11px] text-foreground-subtle">
                    Launch an interactive SSH shell in this directory
                  </div>
                </div>
              </div>
              <span className="text-ui-xs text-foreground-subtler font-mono">⌘`</span>
            </button>

            {/* Change Remote Directory */}
            <button
              type="button"
              onClick={() => setIsChangingDir(!isChangingDir)}
              className="w-full px-3 py-2.5 rounded-lg hover:bg-surface-hover flex items-center justify-between text-left transition-colors cursor-pointer group"
            >
              <div className="flex items-center gap-3">
                <div className="w-8 h-8 rounded-lg bg-surface-subtle flex items-center justify-center text-primary group-hover:bg-primary/15 transition-colors">
                  <FolderOpen className="w-4 h-4" />
                </div>
                <div>
                  <div className="text-xs font-medium text-foreground">Change Remote Directory</div>
                  <div className="text-[11px] text-foreground-subtle">
                    Switch to another path on {activeConn?.host || 'this server'}
                  </div>
                </div>
              </div>
              <ArrowRight className="w-3.5 h-3.5 text-foreground-subtle group-hover:translate-x-0.5 transition-transform" />
            </button>

            {/* Upload Files */}
            <button
              type="button"
              onClick={handleOpenUploader}
              className="w-full px-3 py-2.5 rounded-lg hover:bg-surface-hover flex items-center justify-between text-left transition-colors cursor-pointer group"
            >
              <div className="flex items-center gap-3">
                <div className="w-8 h-8 rounded-lg bg-surface-subtle flex items-center justify-center text-primary group-hover:bg-primary/15 transition-colors">
                  <Upload className="w-4 h-4" />
                </div>
                <div>
                  <div className="text-xs font-medium text-foreground">Upload Files to Remote</div>
                  <div className="text-[11px] text-foreground-subtle">
                    Upload local files or directories over SFTP
                  </div>
                </div>
              </div>
            </button>

            {/* Refresh Files */}
            <button
              type="button"
              onClick={handleRefresh}
              className="w-full px-3 py-2.5 rounded-lg hover:bg-surface-hover flex items-center justify-between text-left transition-colors cursor-pointer group"
            >
              <div className="flex items-center gap-3">
                <div className="w-8 h-8 rounded-lg bg-surface-subtle flex items-center justify-center text-foreground-subtle group-hover:text-foreground transition-colors">
                  <RefreshCw className="w-4 h-4" />
                </div>
                <div>
                  <div className="text-xs font-medium text-foreground">Refresh Remote Explorer</div>
                  <div className="text-[11px] text-foreground-subtle">
                    Reload file and directory tree from server
                  </div>
                </div>
              </div>
            </button>
          </div>

          <div className="py-1">
            {/* Reconnect Session */}
            <button
              type="button"
              disabled={isReconnecting}
              onClick={handleReconnect}
              className="w-full px-3 py-2.5 rounded-lg hover:bg-surface-hover flex items-center justify-between text-left transition-colors cursor-pointer group disabled:opacity-50"
            >
              <div className="flex items-center gap-3">
                <div className="w-8 h-8 rounded-lg bg-surface-subtle flex items-center justify-center text-warning group-hover:bg-warning/15 transition-colors">
                  {isReconnecting ? <Loader2 className="w-4 h-4 animate-spin" /> : <Zap className="w-4 h-4" />}
                </div>
                <div>
                  <div className="text-xs font-medium text-foreground">Reconnect Session</div>
                  <div className="text-[11px] text-foreground-subtle">
                    Re-dial SSH/SFTP connection if connection is laggy
                  </div>
                </div>
              </div>
            </button>

            {/* Switch SSH Host */}
            <button
              type="button"
              onClick={handleSwitchHost}
              className="w-full px-3 py-2.5 rounded-lg hover:bg-surface-hover flex items-center justify-between text-left transition-colors cursor-pointer group"
            >
              <div className="flex items-center gap-3">
                <div className="w-8 h-8 rounded-lg bg-surface-subtle flex items-center justify-center text-foreground-subtle group-hover:text-foreground transition-colors">
                  <Globe className="w-4 h-4" />
                </div>
                <div>
                  <div className="text-xs font-medium text-foreground">Connect to Another Host...</div>
                  <div className="text-[11px] text-foreground-subtle">
                    Open saved SSH hosts or connect to a new server
                  </div>
                </div>
              </div>
            </button>

            {/* Disconnect SSH */}
            <button
              type="button"
              onClick={handleDisconnect}
              className="w-full px-3 py-2.5 rounded-lg hover:bg-destructive/10 flex items-center justify-between text-left transition-colors cursor-pointer group"
            >
              <div className="flex items-center gap-3">
                <div className="w-8 h-8 rounded-lg bg-surface-subtle flex items-center justify-center text-destructive group-hover:bg-destructive/20 transition-colors">
                  <Unplug className="w-4 h-4" />
                </div>
                <div>
                  <div className="text-xs font-medium text-destructive">Disconnect SSH Remote</div>
                  <div className="text-[11px] text-foreground-subtle">
                    Close remote workspace and return to local files
                  </div>
                </div>
              </div>
            </button>
          </div>
        </div>

        {/* Footer */}
        <div className="px-4 py-2 bg-surface-subtle/30 border-t border-border flex items-center justify-between text-ui-xs text-foreground-subtle">
          <span>ForgeADE Remote SSH SFTP</span>
          <span className="font-mono">Press Esc to close</span>
        </div>
      </div>
    </div>
  );
};
