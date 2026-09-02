import React, { useState, useEffect, useRef } from 'react';
import { 
  Folder, 
  FolderOpen, 
  X, 
  ArrowRight, 
  HardDrive, 
  RefreshCw, 
  Clock, 
  Trash2, 
  Search, 
  ChevronRight
} from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { ApiBridge } from '../../services/apiBridge';

interface Props {
  isOpen: boolean;
  onClose: () => void;
}

export const OpenFolderModal: React.FC<Props> = ({ isOpen, onClose }) => {
  const { 
    activeWorkspacePath, 
    openFolder, 
    recentWorkspaces,
    removeRecentWorkspace,
    clearRecentWorkspaces
  } = useWorkspace();

  const [folderPathInput, setFolderPathInput] = useState(activeWorkspacePath || '');
  const [loading, setLoading] = useState(false);
  const [filterQuery, setFilterQuery] = useState('');
  const fileInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (isOpen) {
      setFolderPathInput(activeWorkspacePath);
      setFilterQuery('');
    }
  }, [isOpen, activeWorkspacePath]);

  if (!isOpen) return null;

  const handleOpenDirectory = async (path: string) => {
    if (!path.trim()) return;
    setLoading(true);
    try {
      await openFolder(path.trim());
      onClose();
    } finally {
      setLoading(false);
    }
  };

  const handleNativeBrowse = async () => {
    setLoading(true);
    try {
      const picked = await ApiBridge.pickNativeDirectory();
      if (picked && picked.path) {
        await openFolder(picked.path);
        onClose();
        return;
      }
    } catch (err) {
      console.warn('Native folder browse fallback', err);
      fileInputRef.current?.click();
    } finally {
      setLoading(false);
    }
  };

  const handleFileInputChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = e.target.files;
    if (files && files.length > 0) {
      const firstFile = files[0];
      const relPath = firstFile.webkitRelativePath;
      const rootDirName = relPath.split('/')[0] || 'workspace';
      await openFolder(rootDirName);
      onClose();
    }
  };

  const filteredRecent = recentWorkspaces.filter(p =>
    p.toLowerCase().includes(filterQuery.toLowerCase())
  );

  return (
    <div className="fixed inset-0 bg-black/60 backdrop-blur-xs flex items-center justify-center z-50 p-4 animate-in fade-in duration-100">
      <div 
        className="bg-white dark:bg-[#1e1e1e] w-full max-w-lg rounded-2xl shadow-2xl border border-[#e5e7eb] dark:border-[#333333] overflow-hidden flex flex-col font-sans"
        onClick={e => e.stopPropagation()}
      >
        {/* Header */}
        <div className="px-5 py-4 border-b border-[#f3f4f6] dark:border-[#2b2b2b] flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="w-9 h-9 rounded-xl bg-[#2563eb] flex items-center justify-center text-white shadow-xs">
              <FolderOpen className="w-5 h-5" />
            </div>
            <div>
              <h3 className="text-sm font-bold text-[#111827] dark:text-white">
                Open Workspace Folder
              </h3>
              <p className="text-[11px] text-[#6b7280] dark:text-[#9ca3af]">
                Pick a folder on your computer to start editing and chatting
              </p>
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="p-1.5 text-[#9ca3af] hover:text-[#111827] dark:hover:text-white hover:bg-[#f3f4f6] dark:hover:bg-[#282828] rounded-lg transition-colors cursor-pointer"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Body */}
        <div className="p-5 space-y-4 max-h-[65vh] overflow-y-auto">
          
          {/* Main 1-Click Browse Action */}
          <button
            type="button"
            onClick={handleNativeBrowse}
            disabled={loading}
            className="w-full py-3.5 px-4 rounded-xl bg-[#2563eb] hover:bg-[#1d4ed8] text-white text-xs font-semibold flex items-center justify-center gap-2.5 shadow-sm hover:shadow-md transition-all cursor-pointer group"
          >
            {loading ? (
              <RefreshCw className="w-4 h-4 animate-spin" />
            ) : (
              <HardDrive className="w-4 h-4 group-hover:scale-110 transition-transform" />
            )}
            <span>Choose Folder from Computer...</span>
          </button>

          <input
            type="file"
            ref={fileInputRef}
            onChange={handleFileInputChange}
            // @ts-ignore
            webkitdirectory=""
            directory=""
            multiple
            className="hidden"
          />

          {/* Direct Path Input */}
          <div className="space-y-1.5 pt-1">
            <label className="text-[11px] font-semibold text-[#6b7280] dark:text-[#9ca3af] uppercase tracking-wider block">
              Or specify path
            </label>
            <div className="flex items-center gap-2">
              <div className="flex-1 relative">
                <Folder className="absolute left-3 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-[#9ca3af]" />
                <input
                  type="text"
                  placeholder="/Users/name/projects/my-app"
                  value={folderPathInput}
                  onChange={e => setFolderPathInput(e.target.value)}
                  onKeyDown={e => {
                    if (e.key === 'Enter') handleOpenDirectory(folderPathInput);
                    if (e.key === 'Escape') onClose();
                  }}
                  className="w-full pl-9 pr-3 py-2 text-xs font-mono bg-[#f9fafb] dark:bg-[#252526] border border-[#e5e7eb] dark:border-[#383838] rounded-xl text-[#111827] dark:text-white focus:outline-none focus:ring-2 focus:ring-[#2563eb] focus:border-transparent transition-all placeholder-[#9ca3af]"
                />
              </div>
              <button
                type="button"
                onClick={() => handleOpenDirectory(folderPathInput)}
                disabled={loading || !folderPathInput.trim()}
                className="px-3.5 py-2 rounded-xl bg-[#f3f4f6] dark:bg-[#2e2e30] hover:bg-[#e5e7eb] dark:hover:bg-[#3a3a3d] text-[#111827] dark:text-white text-xs font-semibold flex items-center gap-1.5 transition-all disabled:opacity-40 cursor-pointer border border-[#e5e7eb] dark:border-[#3e3e42]"
              >
                <span>Open</span>
                <ArrowRight className="w-3.5 h-3.5" />
              </button>
            </div>
          </div>

          {/* Recent Workspaces History */}
          <div className="pt-2 border-t border-[#f3f4f6] dark:border-[#2b2b2b] space-y-2">
            <div className="flex items-center justify-between">
              <span className="text-[11px] font-semibold text-[#6b7280] dark:text-[#9ca3af] uppercase tracking-wider flex items-center gap-1.5">
                <Clock className="w-3.5 h-3.5" />
                <span>Recent Workspaces</span>
                {recentWorkspaces.length > 0 && (
                  <span className="text-[10px] px-1.5 py-0.2 rounded-full bg-[#f1f5f9] dark:bg-[#2d2d30] text-[#64748b] dark:text-[#94a3b8] font-mono">
                    {recentWorkspaces.length}
                  </span>
                )}
              </span>

              {recentWorkspaces.length > 0 && (
                <div className="flex items-center gap-2">
                  {recentWorkspaces.length > 3 && (
                    <div className="relative">
                      <Search className="absolute left-2 top-1/2 -translate-y-1/2 w-3 h-3 text-[#9ca3af]" />
                      <input
                        type="text"
                        placeholder="Search..."
                        value={filterQuery}
                        onChange={e => setFilterQuery(e.target.value)}
                        className="pl-6.5 pr-2 py-0.5 text-[11px] bg-[#f9fafb] dark:bg-[#252526] border border-[#e5e7eb] dark:border-[#383838] rounded-md text-[#111827] dark:text-white focus:outline-none w-28 placeholder-[#9ca3af]"
                      />
                    </div>
                  )}
                  <button
                    type="button"
                    onClick={clearRecentWorkspaces}
                    className="text-[10px] text-[#9ca3af] hover:text-[#ef4444] transition-colors cursor-pointer"
                  >
                    Clear All
                  </button>
                </div>
              )}
            </div>

            {filteredRecent.length === 0 ? (
              <div className="py-6 text-center text-xs text-[#9ca3af]">
                {filterQuery ? 'No matching recent workspaces' : 'No previous workspaces opened yet'}
              </div>
            ) : (
              <div className="space-y-1 max-h-48 overflow-y-auto pr-1">
                {filteredRecent.map((recentPath, idx) => {
                  const folderName = recentPath.split('/').pop() || recentPath;
                  const isActive = recentPath === activeWorkspacePath;
                  return (
                    <div
                      key={idx}
                      className={`w-full p-2.5 rounded-xl flex items-center justify-between text-xs transition-all group ${
                        isActive
                          ? 'bg-[#eff6ff] dark:bg-[#1e293b]/70 border border-[#bfdbfe] dark:border-[#1e3a5f]'
                          : 'hover:bg-[#f3f4f6] dark:hover:bg-[#252528] border border-transparent'
                      }`}
                    >
                      <button
                        type="button"
                        onClick={() => handleOpenDirectory(recentPath)}
                        className="flex items-center gap-2.5 min-w-0 flex-1 text-left cursor-pointer"
                      >
                        <div className={`w-7 h-7 rounded-lg flex items-center justify-center shrink-0 ${
                          isActive
                            ? 'bg-[#2563eb] text-white'
                            : 'bg-[#f3f4f6] dark:bg-[#2a2a2c] text-[#6b7280] dark:text-[#9ca3af] group-hover:text-[#2563eb] transition-colors'
                        }`}>
                          <Folder className="w-3.5 h-3.5" />
                        </div>
                        <div className="min-w-0 flex-1">
                          <div className="flex items-center gap-2">
                            <span className={`font-semibold truncate ${isActive ? 'text-[#2563eb] dark:text-[#60a5fa]' : 'text-[#111827] dark:text-white'}`}>
                              {folderName}
                            </span>
                            {isActive && (
                              <span className="text-[9px] px-1.5 py-0.2 rounded-full bg-[#2563eb] text-white font-medium">
                                Active
                              </span>
                            )}
                          </div>
                          <p className="text-[11px] text-[#6b7280] dark:text-[#9ca3af] font-mono truncate mt-0.5">
                            {recentPath}
                          </p>
                        </div>
                      </button>

                      <div className="flex items-center gap-1 shrink-0 ml-2">
                        <button
                          type="button"
                          onClick={(e) => {
                            e.stopPropagation();
                            removeRecentWorkspace(recentPath);
                          }}
                          className="p-1 text-[#9ca3af] hover:text-[#ef4444] rounded opacity-0 group-hover:opacity-100 transition-opacity cursor-pointer"
                          title="Remove from recent history"
                        >
                          <Trash2 className="w-3.5 h-3.5" />
                        </button>
                        <ChevronRight className="w-3.5 h-3.5 text-[#9ca3af] group-hover:text-[#2563eb] dark:group-hover:text-[#60a5fa] transition-colors" />
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </div>

        </div>

        {/* Footer */}
        <div className="px-5 py-3 bg-[#f9fafb] dark:bg-[#18181a] border-t border-[#f3f4f6] dark:border-[#2b2b2b] flex items-center justify-between text-xs text-[#6b7280] dark:text-[#9ca3af]">
          <span className="truncate max-w-[280px]">
            Current: <strong className="font-mono text-[#111827] dark:text-white">{activeWorkspacePath ? activeWorkspacePath.split('/').pop() : 'None'}</strong>
          </span>
          <button
            type="button"
            onClick={onClose}
            className="px-3 py-1 rounded-lg hover:bg-[#e5e7eb] dark:hover:bg-[#282828] text-[#111827] dark:text-white transition-colors cursor-pointer font-medium"
          >
            Cancel
          </button>
        </div>

      </div>
    </div>
  );
};
