import React, { useState, useEffect } from 'react';
import { 
  Server, 
  Key, 
  Lock, 
  X, 
  ArrowRight, 
  Folder, 
  Trash2, 
  Loader2, 
  CheckCircle2, 
  AlertCircle,
  Plus
} from 'lucide-react';
import { ApiBridge } from '../../services/apiBridge';
import { useWorkspace } from '../../stores/workspaceStore';
import type { SSHConfig, SSHConnectionStatus } from '../../lib/wails';

interface Props {
  isOpen: boolean;
  onClose: () => void;
}

const STORAGE_KEY = 'forge_saved_ssh_configs';

export const SSHConnectModal: React.FC<Props> = ({ isOpen, onClose }) => {
  const { openFolder } = useWorkspace();

  const [savedConfigs, setSavedConfigs] = useState<SSHConfig[]>([]);
  const [selectedConfigId, setSelectedConfigId] = useState<string | null>(null);

  // Form states
  const [label, setLabel] = useState('');
  const [host, setHost] = useState('');
  const [port, setPort] = useState(22);
  const [user, setUser] = useState('');
  const [authType, setAuthType] = useState<'key_file' | 'password' | 'agent'>('key_file');
  const [keyPath, setKeyPath] = useState('~/.ssh/id_ed25519');
  const [keyPassphrase, setKeyPassphrase] = useState('');
  const [password, setPassword] = useState('');
  const [remotePath, setRemotePath] = useState('/var/www');

  const [isConnecting, setIsConnecting] = useState(false);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);
  const [successStatus, setSuccessStatus] = useState<SSHConnectionStatus | null>(null);

  useEffect(() => {
    if (isOpen) {
      setErrorMsg(null);
      setSuccessStatus(null);
      try {
        const stored = localStorage.getItem(STORAGE_KEY);
        if (stored) {
          const list = JSON.parse(stored);
          if (Array.isArray(list)) {
            setSavedConfigs(list);
            if (list.length > 0 && !selectedConfigId) {
              loadConfigIntoForm(list[0]);
            }
          }
        }
      } catch (err) {
        console.warn('Failed to load saved SSH configs:', err);
      }
    }
  }, [isOpen]);

  const loadConfigIntoForm = (cfg: SSHConfig) => {
    setSelectedConfigId(cfg.id || null);
    setLabel(cfg.label || '');
    setHost(cfg.host || '');
    setPort(cfg.port || 22);
    setUser(cfg.user || '');
    setAuthType(cfg.authType || 'key_file');
    setKeyPath(cfg.keyPath || '~/.ssh/id_ed25519');
    setKeyPassphrase(cfg.keyPassphrase || '');
    setPassword(cfg.password || '');
    setRemotePath(cfg.remotePath || '/var/www');
    setErrorMsg(null);
  };

  const handleResetForm = () => {
    setSelectedConfigId(null);
    setLabel('');
    setHost('');
    setPort(22);
    setUser('');
    setAuthType('key_file');
    setKeyPath('~/.ssh/id_ed25519');
    setKeyPassphrase('');
    setPassword('');
    setRemotePath('/var/www');
    setErrorMsg(null);
  };

  const saveConfig = (newCfg: SSHConfig) => {
    const updated = savedConfigs.filter(c => c.id !== newCfg.id);
    updated.unshift(newCfg);
    setSavedConfigs(updated);
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(updated));
    } catch {}
  };

  const deleteConfig = (id: string, e: React.MouseEvent) => {
    e.stopPropagation();
    const updated = savedConfigs.filter(c => c.id !== id);
    setSavedConfigs(updated);
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(updated));
    } catch {}
    if (selectedConfigId === id) {
      handleResetForm();
    }
  };

  const handleConnect = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!host.trim() || !user.trim()) {
      setErrorMsg('Host and Username are required.');
      return;
    }

    setIsConnecting(true);
    setErrorMsg(null);
    setSuccessStatus(null);

    const configId = selectedConfigId || `ssh-${Date.now()}`;
    const cfg: SSHConfig = {
      id: configId,
      label: label.trim() || `${user.trim()}@${host.trim()}`,
      host: host.trim(),
      port: Number(port) || 22,
      user: user.trim(),
      authType,
      keyPath: authType === 'key_file' ? keyPath.trim() : undefined,
      keyPassphrase: authType === 'key_file' && keyPassphrase ? keyPassphrase : undefined,
      password: authType === 'password' ? password : undefined,
      remotePath: remotePath.trim() || '/'
    };

    try {
      const status = await ApiBridge.openSSHWorkspace(cfg);
      if (status && status.connected) {
        setSuccessStatus(status);
        saveConfig(cfg);
        const remoteUri = `ssh://${status.id}${status.remotePath}`;
        await openFolder(remoteUri);
        setTimeout(() => {
          onClose();
        }, 600);
      } else {
        setErrorMsg(status?.error || 'Failed to connect to remote SSH server.');
      }
    } catch (err: any) {
      setErrorMsg(err?.message || 'SSH connection error.');
    } finally {
      setIsConnecting(false);
    }
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4 animate-in fade-in duration-150">
      <div 
        className="w-full max-w-2xl bg-card border border-border rounded-xl shadow-2xl flex flex-col overflow-hidden max-h-[90vh]"
        onClick={e => e.stopPropagation()}
      >
        {/* Header */}
        <div className="px-5 py-4 border-b border-border flex items-center justify-between bg-surface-subtle/50 dark:bg-background/40">
          <div className="flex items-center gap-2.5">
            <div className="w-8 h-8 rounded-lg bg-primary/10 text-primary flex items-center justify-center font-bold">
              <Server className="w-4 h-4" />
            </div>
            <div>
              <h2 className="text-sm font-semibold text-foreground flex items-center gap-2">
                Connect to Remote Host via SSH
                <span className="text-ui-xs bg-primary/10 text-primary px-1.5 py-0.5 rounded font-mono font-medium">
                  SFTP
                </span>
              </h2>
              <p className="text-xs text-foreground-subtle">
                Browse, edit, and sync remote files natively without local mounting
              </p>
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="p-1 rounded-lg hover:bg-surface-hover dark:hover:bg-surface-hover text-foreground-subtle hover:text-foreground transition-colors cursor-pointer"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Content Body */}
        <div className="flex flex-1 min-h-0 divide-x divide-border dark:divide-border">
          {/* Saved Sessions Sidebar */}
          <div className="w-48 bg-surface-subtle/30 dark:bg-background/20 p-3 flex flex-col gap-1 overflow-y-auto">
            <div className="flex items-center justify-between text-ui-xs font-semibold text-foreground-subtle uppercase tracking-wider px-1 mb-1">
              <span>Saved Hosts</span>
              <button
                type="button"
                onClick={handleResetForm}
                className="p-0.5 hover:bg-surface-hover dark:hover:bg-surface-hover rounded text-primary cursor-pointer"
                title="New Host"
              >
                <Plus className="w-3.5 h-3.5" />
              </button>
            </div>

            {savedConfigs.length === 0 ? (
              <p className="text-xs text-foreground-subtle px-1 py-4 text-center">No saved hosts</p>
            ) : (
              savedConfigs.map(c => (
                <div
                  key={c.id}
                  onClick={() => loadConfigIntoForm(c)}
                  className={`group px-2.5 py-2 rounded-lg text-xs cursor-pointer flex items-center justify-between transition-colors ${
                    selectedConfigId === c.id
                      ? 'bg-primary text-white font-medium shadow-xs'
                      : 'hover:bg-surface-hover dark:hover:bg-surface-hover text-foreground dark:text-foreground-secondary'
                  }`}
                >
                  <div className="truncate min-w-0 pr-1">
                    <div className="truncate font-medium">{c.label || c.host}</div>
                    <div className={`text-ui-xs truncate ${selectedConfigId === c.id ? 'text-white/80' : 'text-foreground-subtle'}`}>
                      {c.user}@{c.host}
                    </div>
                  </div>
                  <button
                    type="button"
                    onClick={(e) => c.id && deleteConfig(c.id, e)}
                    className={`opacity-0 group-hover:opacity-100 p-1 rounded hover:bg-black/10 transition-opacity ${
                      selectedConfigId === c.id ? 'text-white hover:text-white' : 'text-foreground-subtle hover:text-destructive'
                    }`}
                    title="Delete connection profile"
                  >
                    <Trash2 className="w-3 h-3" />
                  </button>
                </div>
              ))
            )}
          </div>

          {/* Form */}
          <form onSubmit={handleConnect} className="flex-1 p-5 overflow-y-auto space-y-4">
            {errorMsg && (
              <div className="p-3 rounded-lg bg-destructive/10 border border-destructive/20 text-destructive text-xs flex items-start gap-2">
                <AlertCircle className="w-4 h-4 shrink-0 mt-0.5" />
                <span className="flex-1 break-words">{errorMsg}</span>
              </div>
            )}

            {successStatus && (
              <div className="p-3 rounded-lg bg-success/10 border border-success/20 text-success text-xs flex items-center gap-2">
                <CheckCircle2 className="w-4 h-4 shrink-0" />
                <span>Connected! Opening remote workspace...</span>
              </div>
            )}

            <div className="grid grid-cols-3 gap-3">
              <div className="col-span-2 space-y-1">
                <label className="text-xs font-medium text-foreground dark:text-foreground-secondary">
                  Host / IP Address *
                </label>
                <input
                  type="text"
                  required
                  placeholder="vps.example.com or 192.168.1.50"
                  value={host}
                  onChange={e => setHost(e.target.value)}
                  className="w-full px-3 py-1.5 bg-background dark:bg-background border border-border rounded-lg text-xs font-mono text-foreground focus:outline-none focus:border-primary"
                />
              </div>

              <div className="space-y-1">
                <label className="text-xs font-medium text-foreground dark:text-foreground-secondary">
                  Port
                </label>
                <input
                  type="number"
                  value={port}
                  onChange={e => setPort(parseInt(e.target.value, 10) || 22)}
                  className="w-full px-3 py-1.5 bg-background dark:bg-background border border-border rounded-lg text-xs font-mono text-foreground focus:outline-none focus:border-primary"
                />
              </div>
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1">
                <label className="text-xs font-medium text-foreground dark:text-foreground-secondary">
                  Username *
                </label>
                <input
                  type="text"
                  required
                  placeholder="ubuntu or root"
                  value={user}
                  onChange={e => setUser(e.target.value)}
                  className="w-full px-3 py-1.5 bg-background dark:bg-background border border-border rounded-lg text-xs font-mono text-foreground focus:outline-none focus:border-primary"
                />
              </div>

              <div className="space-y-1">
                <label className="text-xs font-medium text-foreground dark:text-foreground-secondary">
                  Display Label (Optional)
                </label>
                <input
                  type="text"
                  placeholder="e.g. Production API"
                  value={label}
                  onChange={e => setLabel(e.target.value)}
                  className="w-full px-3 py-1.5 bg-background dark:bg-background border border-border rounded-lg text-xs text-foreground focus:outline-none focus:border-primary"
                />
              </div>
            </div>

            {/* Auth Method Selector */}
            <div className="space-y-1.5 pt-1">
              <label className="text-xs font-medium text-foreground dark:text-foreground-secondary">
                Authentication Method
              </label>
              <div className="flex items-center gap-2">
                {[
                  { id: 'key_file', label: 'Private Key', icon: Key },
                  { id: 'password', label: 'Password', icon: Lock },
                  { id: 'agent', label: 'SSH Agent', icon: Server },
                ].map(item => (
                  <button
                    key={item.id}
                    type="button"
                    onClick={() => setAuthType(item.id as any)}
                    className={`flex-1 py-1.5 px-3 rounded-lg text-xs font-medium border flex items-center justify-center gap-1.5 cursor-pointer transition-colors ${
                      authType === item.id
                        ? 'bg-primary/10 border-primary text-primary dark:text-info'
                        : 'border-border hover:bg-surface-hover dark:hover:bg-surface-hover text-foreground-subtle'
                    }`}
                  >
                    <item.icon className="w-3.5 h-3.5" />
                    {item.label}
                  </button>
                ))}
              </div>
            </div>

            {/* Auth Details */}
            {authType === 'key_file' && (
              <div className="space-y-2 p-3 rounded-lg bg-surface-subtle/50 dark:bg-background/40 border border-border">
                <div className="space-y-1">
                  <label className="text-ui-xs font-medium text-foreground-subtle">
                    Private Key Path
                  </label>
                  <input
                    type="text"
                    required
                    placeholder="~/.ssh/id_ed25519"
                    value={keyPath}
                    onChange={e => setKeyPath(e.target.value)}
                    className="w-full px-2.5 py-1 bg-background dark:bg-background border border-border rounded text-xs font-mono text-foreground focus:outline-none focus:border-primary"
                  />
                </div>
                <div className="space-y-1">
                  <label className="text-ui-xs font-medium text-foreground-subtle">
                    Passphrase (Leave empty if unencrypted)
                  </label>
                  <input
                    type="password"
                    placeholder="Optional passphrase"
                    value={keyPassphrase}
                    onChange={e => setKeyPassphrase(e.target.value)}
                    className="w-full px-2.5 py-1 bg-background dark:bg-background border border-border rounded text-xs font-mono text-foreground focus:outline-none focus:border-primary"
                  />
                </div>
              </div>
            )}

            {authType === 'password' && (
              <div className="space-y-1 p-3 rounded-lg bg-surface-subtle/50 dark:bg-background/40 border border-border">
                <label className="text-ui-xs font-medium text-foreground-subtle">
                  SSH Password
                </label>
                <input
                  type="password"
                  required
                  placeholder="Enter password"
                  value={password}
                  onChange={e => setPassword(e.target.value)}
                  className="w-full px-2.5 py-1 bg-background dark:bg-background border border-border rounded text-xs font-mono text-foreground focus:outline-none focus:border-primary"
                />
              </div>
            )}

            {authType === 'agent' && (
              <div className="p-3 rounded-lg bg-surface-subtle/50 dark:bg-background/40 border border-border text-xs text-foreground-subtle">
                Will authenticate via active local SSH agent (<code className="text-primary font-mono font-medium">$SSH_AUTH_SOCK</code>).
              </div>
            )}

            {/* Remote Folder Path */}
            <div className="space-y-1 pt-1">
              <label className="text-xs font-medium text-foreground dark:text-foreground-secondary flex items-center gap-1.5">
                <Folder className="w-3.5 h-3.5 text-warning" />
                Remote Workspace Folder Path
              </label>
              <input
                type="text"
                required
                placeholder="/var/www/my-project or /home/ubuntu"
                value={remotePath}
                onChange={e => setRemotePath(e.target.value)}
                className="w-full px-3 py-1.5 bg-background dark:bg-background border border-border rounded-lg text-xs font-mono text-foreground focus:outline-none focus:border-primary"
              />
            </div>

            {/* Submit Actions */}
            <div className="pt-3 flex items-center justify-end gap-2 border-t border-border">
              <button
                type="button"
                onClick={onClose}
                className="px-3.5 py-1.5 rounded-lg border border-border hover:bg-surface-hover dark:hover:bg-surface-hover text-xs font-medium text-foreground-subtle transition-colors cursor-pointer"
              >
                Cancel
              </button>
              <button
                type="submit"
                disabled={isConnecting}
                className="px-4 py-1.5 rounded-lg bg-primary hover:bg-accent-hover text-white text-xs font-medium flex items-center gap-1.5 transition-colors cursor-pointer disabled:opacity-50 shadow-xs"
              >
                {isConnecting ? (
                  <>
                    <Loader2 className="w-3.5 h-3.5 animate-spin" />
                    Connecting...
                  </>
                ) : (
                  <>
                    Connect & Open
                    <ArrowRight className="w-3.5 h-3.5" />
                  </>
                )}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  );
};
