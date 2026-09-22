import React, { useState, useRef, useEffect } from 'react';
import { 
  Folder, 
  ChevronDown, 
  X, 
  Plus, 
  FolderOpen, 
  FileCode, 
  History, 
  Check, 
  Sparkles,
  FilePlus
} from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';

export const EditorProjectDropdown: React.FC = () => {
  const { 
    activeWorkspacePath, 
    openFolder, 
    closeWorkspace, 
    recentWorkspaces,
    openTab,
    gitBranch
  } = useWorkspace();

  const [isOpen, setIsOpen] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);

  const currentProjectName = activeWorkspacePath 
    ? activeWorkspacePath.split('/').filter(Boolean).pop() || 'Untitled Project'
    : null;

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setIsOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const handleCreateScratchFile = () => {
    const scratchId = `scratch-${Date.now()}`;
    openTab({
      id: scratchId,
      fileId: scratchId,
      fileName: 'scratchpad.ts',
      filePath: 'scratchpad.ts',
      type: 'code',
      content: '// Standalone Scratchpad\n// You can write, test, and run code here freely without an active project.\n\nconsole.log("Hello ForgeADE");\n'
    });
    setIsOpen(false);
  };

  const handleSelectRecent = (path: string) => {
    openFolder(path);
    setIsOpen(false);
  };

  return (
    <div className="relative select-none font-sans text-xs" ref={dropdownRef}>
      {/* Trigger Button */}
      <div className="flex items-center rounded-lg border border-border dark:border-border bg-background dark:bg-[#202022] hover:border-border dark:hover:border-[#444448] transition-colors">
        
        {/* Project Name / Status */}
        <button
          type="button"
          onClick={() => setIsOpen(prev => !prev)}
          className="flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium text-foreground-subtle dark:text-[#dddddd] hover:text-black dark:hover:text-white cursor-pointer"
        >
          {currentProjectName ? (
            <>
              <Folder className="w-3.5 h-3.5 text-warning" />
              <span className="font-semibold">{currentProjectName}</span>
              {gitBranch && (
                <span className="text-[10px] text-foreground-subtlest dark:text-[#888888]">({gitBranch})</span>
              )}
            </>
          ) : (
            <>
              <FileCode className="w-3.5 h-3.5 text-info" />
              <span className="text-foreground-subtlest dark:text-[#999999]">Standalone Editor (No Project)</span>
            </>
          )}
          <ChevronDown className="w-3 h-3 text-foreground-subtlest dark:text-[#777777] ml-0.5" />
        </button>

        {/* Seamless Close Project '✕' Button */}
        {currentProjectName && (
          <button
            type="button"
            onClick={(e) => {
              e.stopPropagation();
              closeWorkspace();
            }}
            className="px-1.5 py-1 text-foreground-subtlest hover:text-destructive dark:text-[#777777] dark:hover:text-destructive border-l border-border dark:border-border transition-colors cursor-pointer"
            title="Close Project (Detach project seamlessly)"
          >
            <X className="w-3 h-3" />
          </button>
        )}
      </div>

      {/* Dropdown Menu */}
      {isOpen && (
        <div className="absolute left-0 top-full mt-1 w-72 rounded-2xl bg-white dark:bg-card shadow-2xl border border-border dark:border-border p-2 z-50 animate-in fade-in zoom-in-95 duration-100 text-xs">
          
          {/* Header */}
          <div className="px-2.5 py-1 text-[11px] font-semibold text-foreground-subtlest dark:text-[#777777] uppercase tracking-wider flex items-center justify-between">
            <span>Project Management</span>
            {currentProjectName && (
              <span className="text-[10px] text-success font-mono">Active</span>
            )}
          </div>

          {/* Quick Actions */}
          <div className="space-y-0.5 py-1">
            <button
              type="button"
              onClick={() => {
                setIsOpen(false);
                openFolder();
              }}
              className="w-full text-left px-2.5 py-1.5 rounded-lg hover:bg-background dark:hover:bg-[#28282c] text-foreground-subtle dark:text-foreground-secondary hover:text-black dark:hover:text-white flex items-center gap-2 cursor-pointer font-medium"
            >
              <FolderOpen className="w-3.5 h-3.5 text-info" />
              <span>Open New Folder...</span>
            </button>

            <button
              type="button"
              onClick={handleCreateScratchFile}
              className="w-full text-left px-2.5 py-1.5 rounded-lg hover:bg-background dark:hover:bg-[#28282c] text-foreground-subtle dark:text-foreground-secondary hover:text-black dark:hover:text-white flex items-center gap-2 cursor-pointer"
            >
              <FilePlus className="w-3.5 h-3.5 text-[#10b981]" />
              <span>New Scratchpad Buffer</span>
            </button>

            {currentProjectName && (
              <button
                type="button"
                onClick={() => {
                  setIsOpen(false);
                  closeWorkspace();
                }}
                className="w-full text-left px-2.5 py-1.5 rounded-lg hover:bg-destructive/10 dark:hover:bg-destructive/10 text-destructive flex items-center gap-2 cursor-pointer"
              >
                <X className="w-3.5 h-3.5" />
                <span>Close Current Project</span>
              </button>
            )}
          </div>

          <div className="h-[1px] bg-surface-hover dark:bg-[#2b2b2e] my-1" />

          {/* Recent Opened History Section */}
          <div className="px-2.5 py-1 text-[11px] font-semibold text-foreground-subtlest dark:text-[#777777] uppercase tracking-wider flex items-center gap-1.5">
            <History className="w-3 h-3" />
            <span>Recent History ({recentWorkspaces.length})</span>
          </div>

          <div className="space-y-0.5 max-h-44 overflow-y-auto py-1">
            {recentWorkspaces.length === 0 ? (
              <p className="text-[11px] text-foreground-subtlest italic px-2.5 py-1">No recent workspaces</p>
            ) : (
              recentWorkspaces.map(ws => {
                const name = ws.split('/').filter(Boolean).pop() || ws;
                const isCurrent = ws === activeWorkspacePath;

                return (
                  <button
                    key={ws}
                    type="button"
                    onClick={() => handleSelectRecent(ws)}
                    className={`w-full text-left px-2.5 py-1.5 rounded-lg flex items-center justify-between hover:bg-background dark:hover:bg-[#28282c] cursor-pointer transition-colors ${
                      isCurrent 
                        ? 'bg-primary/10 dark:bg-card text-primary dark:text-[#93c5fd] font-medium' 
                        : 'text-foreground-subtle dark:text-foreground-secondary'
                    }`}
                  >
                    <div className="flex items-center gap-2 truncate min-w-0 pr-2">
                      <Folder className="w-3.5 h-3.5 text-warning shrink-0" />
                      <div className="truncate min-w-0">
                        <p className="truncate text-xs leading-tight font-medium">{name}</p>
                        <p className="text-[10px] text-foreground-subtlest dark:text-[#666666] truncate font-mono">{ws}</p>
                      </div>
                    </div>
                    {isCurrent && <Check className="w-3.5 h-3.5 text-primary dark:text-info shrink-0" />}
                  </button>
                );
              })
            )}
          </div>

        </div>
      )}
    </div>
  );
};
