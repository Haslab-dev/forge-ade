import React from 'react';
import { GitBranch, AlertCircle, AlertTriangle, Bell, Settings, CheckCircle2 } from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';

export const StatusBar: React.FC = () => {
  const { mode, openSettingsTab, activeAgent, activeWorkspacePath } = useWorkspace();

  const folderName = activeWorkspacePath.split('/').pop() || activeWorkspacePath || 'Workspace';

  if (mode === 'agent') {
    return (
      <footer className="h-[30px] min-h-[30px] bg-white dark:bg-[#181818] border-t border-[#f0f0f2] dark:border-[#2b2b2b] flex items-center justify-between px-4 text-xs text-[#64748b] dark:text-[#9ca3af] select-none z-20">
        <div className="flex items-center gap-2">
          <span className="font-semibold text-[#0f172a] dark:text-[#e5e7eb]">{activeAgent.name}</span>
          <span className="text-[#9ca3af]">•</span>
          <span className="text-[#64748b] dark:text-[#9ca3af] font-mono">{folderName}</span>
        </div>
        <button 
          type="button" 
          onClick={() => openSettingsTab('agents')}
          className="hover:text-[#0f172a] dark:hover:text-white transition-colors flex items-center gap-1.5 cursor-pointer text-xs"
          title="Settings & Theming (⌘,)"
        >
          <Settings className="w-3.5 h-3.5 text-[#3b82f6]" />
          <span>Settings</span>
        </button>
      </footer>
    );
  }

  return (
    <footer className="h-[24px] min-h-[24px] bg-[#f8fafc] dark:bg-[#1e1e1e] border-t border-[#e2e8f0] dark:border-[#2b2b2b] text-[#64748b] dark:text-[#cccccc] flex items-center justify-between px-3 text-[11px] font-mono select-none z-20">
      {/* Left side: Git branch, errors, warnings */}
      <div className="flex items-center gap-3">
        <button 
          type="button"
          className="flex items-center gap-1 hover:bg-[#e2e8f0] dark:hover:bg-[#2d2d2d] px-1.5 py-0.5 rounded transition-colors text-[#0f172a] dark:text-[#cccccc]"
          title="Git branch: main"
        >
          <GitBranch className="w-3 h-3 text-[#3b82f6]" />
          <span>main</span>
        </button>

        <div className="flex items-center gap-2 text-[#9ca3af]">
          <span className="flex items-center gap-0.5 hover:text-[#0f172a] dark:hover:text-white transition-colors cursor-pointer">
            <AlertCircle className="w-3 h-3 text-[#ef4444]" />
            <span>0</span>
          </span>
          <span className="flex items-center gap-0.5 hover:text-[#0f172a] dark:hover:text-white transition-colors cursor-pointer">
            <AlertTriangle className="w-3 h-3 text-[#f59e0b]" />
            <span>0</span>
          </span>
        </div>

        <div className="flex items-center gap-1 text-[#10b981] text-[10px]">
          <CheckCircle2 className="w-2.5 h-2.5" />
          <span>{folderName} Synced</span>
        </div>
      </div>

      {/* Right side: settings, notification */}
      <div className="flex items-center gap-3">
        <button 
          type="button"
          onClick={() => openSettingsTab('agents')}
          className="hover:text-[#0f172a] dark:hover:text-white transition-colors flex items-center gap-1 text-[#64748b] dark:text-[#9ca3af] cursor-pointer"
          title="My-ADE Settings & ACP"
        >
          <Settings className="w-3 h-3 text-[#3b82f6]" />
          <span>Settings</span>
        </button>

        <button 
          type="button"
          className="p-0.5 hover:bg-[#e2e8f0] dark:hover:bg-[#2d2d2d] rounded transition-colors text-[#64748b] dark:text-[#9ca3af] hover:text-[#0f172a] dark:hover:text-white cursor-pointer"
          title="Notifications"
        >
          <Bell className="w-3 h-3" />
        </button>
      </div>
    </footer>
  );
};
