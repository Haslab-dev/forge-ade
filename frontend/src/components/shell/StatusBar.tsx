import React from 'react';
import { 
  GitBranch, 
  AlertCircle, 
  AlertTriangle, 
  Bell, 
  Settings, 
  CheckCircle2, 
  RefreshCw,
  Code2
} from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';

export const StatusBar: React.FC = () => {
  const { mode, openSettingsTab, activeAgent, activeWorkspacePath, openTabs, activeTabId, selectedFile } = useWorkspace();

  const folderName = activeWorkspacePath.split('/').pop() || activeWorkspacePath || 'HasPHP';
  const activeTab = openTabs.find(t => t.id === activeTabId);
  const curName = activeTab?.fileName || selectedFile?.name || 'Readme.md';

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
          <span className="text-[#9ca3af]">•</span>
          <span className="text-[#6b7280] dark:text-[#9ca3af] font-mono text-[11px]">{folderName}</span>
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
      
      {/* Left side: Remote + Git branch + Diagnostics */}
      <div className="flex items-center gap-3">
        {/* Remote badge icon */}
        <button
          type="button"
          className="w-5 h-4 rounded bg-[#2563eb] text-white flex items-center justify-center text-[10px] font-mono font-bold hover:opacity-90 transition-opacity cursor-pointer"
          title="Remote connection"
        >
          &gt;&lt;
        </button>

        {/* Git Branch & Sync */}
        <div className="flex items-center gap-1 text-[#334155] dark:text-[#cbd5e1] font-mono hover:text-[#2563eb] transition-colors cursor-pointer">
          <GitBranch className="w-3 h-3 text-[#3b82f6]" />
          <span>main</span>
          <RefreshCw className="w-2.5 h-2.5 ml-0.5 text-[#9ca3af] hover:rotate-180 transition-transform" />
        </div>

        {/* Diagnostics: errors and warnings */}
        <div className="flex items-center gap-2 font-mono text-[#64748b] dark:text-[#94a3b8]">
          <span className="flex items-center gap-1 hover:text-[#ef4444] transition-colors cursor-pointer">
            <span className="w-3 h-3 rounded-full border border-current flex items-center justify-center text-[8px] font-bold">×</span>
            <span>0</span>
          </span>
          <span className="flex items-center gap-1 hover:text-[#f59e0b] transition-colors cursor-pointer">
            <AlertTriangle className="w-3 h-3 text-[#f59e0b]" />
            <span>0</span>
          </span>
        </div>
      </div>

      {/* Right side: Cursor + Indent + Encoding + Language + Tier + Forge Settings + Bell */}
      <div className="flex items-center gap-3 font-sans text-[#64748b] dark:text-[#94a3b8]">
        <span className="hover:text-[#111827] dark:hover:text-white transition-colors cursor-pointer">
          Ln 1, Col 1
        </span>
        <span className="hover:text-[#111827] dark:hover:text-white transition-colors cursor-pointer">
          Spaces: 4
        </span>
        <span className="hover:text-[#111827] dark:hover:text-white transition-colors cursor-pointer">
          UTF-8
        </span>
        <span className="hover:text-[#111827] dark:hover:text-white transition-colors cursor-pointer">
          LF
        </span>
        <span className="flex items-center gap-1 hover:text-[#111827] dark:hover:text-white transition-colors cursor-pointer">
          <span className="text-[#9ca3af] font-mono font-bold text-[10px]">{'{}'}</span>
          <span>{getLanguageLabel()}</span>
        </span>
        <button
          type="button"
          onClick={() => openSettingsTab('providers')}
          className="hover:text-[#2563eb] dark:hover:text-[#60a5fa] transition-colors cursor-pointer"
        >
          Free - Upgrade Now
        </button>
        <button
          type="button"
          onClick={() => openSettingsTab('agents')}
          className="hover:text-[#111827] dark:hover:text-white transition-colors cursor-pointer"
        >
          Forge - Settings
        </button>
        <button
          type="button"
          className="hover:text-[#111827] dark:hover:text-white transition-colors cursor-pointer p-0.5"
          title="Notifications"
        >
          <Bell className="w-3 h-3" />
        </button>
      </div>

    </footer>
  );
};
