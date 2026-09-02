import React, { useState, useEffect, useRef } from 'react';
import { Folder, FolderOpen, X, ArrowRight, HardDrive, RefreshCw, Clock, Trash2, Search, ChevronRight } from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { ApiBridge } from '../../services/apiBridge';

interface Props {
  isOpen: boolean;
  onClose: () => void;
}

export const OpenFolderModal: React.FC<Props> = ({ isOpen, onClose }) => {
  const { activeWorkspacePath, openFolder, recentWorkspaces } = useWorkspace();
  const [folderPathInput, setFolderPathInput] = useState(activeWorkspacePath || '');
  const [systemCwd, setSystemCwd] = useState<string>('');
  const [systemHome, setSystemHome] = useState<string>('');
  const [loading, setLoading] = useState(false);
  const [filterQuery, setFilterQuery] = useState('');
  const fileInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (isOpen) {
      setFolderPathInput(activeWorkspacePath);
      setFilterQuery('');
      ApiBridge.getWorkspaceInfo().then(info => {
        if (info?.cwd) setSystemCwd(info.cwd);
        if (info?.home) setSystemHome(info.home);
      });
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
      console.warn('Native folder browse failed, falling back to input', err);
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

  const quickFolders = [
    { label: 'Home Directory', path: systemHome || '~', icon: '🏠' },
    { label: 'Current Project', path: systemCwd || '.', icon: '📁' }
  ];

  return (
    <div className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50 p-4">
      <div className="bg-white dark:bg-[#1e1e1e] w-full max-w-xl rounded-2xl shadow-2xl border border-[#e5e7eb] dark:border-[#383838] overflow-hidden animate-in fade-in zoom-in-95 duration-150 font-sans">
        
        {/* Header */}
        <div className="p-4 border-b border-[#f3f4f6] dark:border-[#2f2f31] flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="w-9 h-9 rounded-xl bg-gradient-to-br from-[#2563eb] to-[#7c3aed] flex items-center justify-center text-white shadow-md">
              <FolderOpen className="w-4.5 h-4.5" />
            </div>
            <div>
              <h3 className="text-sm font-bold text-[#0f172a] dark:text-white">
                Open Workspace
              </h3>
              <p className="text-[11px] text-[#64748b] dark:text-[#94a3b8]">
                Pick a folder to edit files and run commands
              </p>
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="p-1.5 text-[#9ca3af] hover:text-[#0f172a] dark:hover:text-white hover:bg-[#f1f5f9] dark:hover:bg-[#2a2d2e] rounded-lg transition-colors cursor-pointer"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Body */}
        <div className="p-4 space-y-4 max-h-[70vh] overflow-y-auto">
          
          {/* Path input */}
          <div className="space-y-1.5">
            <label className="text-[11px] font-semibold text-[#64748b] dark:text-[#94a3b8] uppercase tracking-wider">
              Path
            </label>
            <div className="flex items-center gap-2">
              <div className="flex-1 relative">
                <Folder className="absolute left-3 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-[#94a3b8]" />
                <input
                  type="text"
                  autoFocus
                  placeholder="/path/to/your/project"
                  value={folderPathInput}
                  onChange={e => setFolderPathInput(e.target.value)}
                  onKeyDown={e => {
                    if (e.key === 'Enter') handleOpenDirectory(folderPathInput);
                  }}
                  className="w-full pl-9 pr-3 py-2.5 text-xs font-mono bg-[#f8fafc] dark:bg-[#252526] border border-[#e2e8f0] dark:border-[#383838] rounded-xl text-[#0f172a] dark:text-white focus:outline-none focus:ring-2 focus:ring-[#2563eb] focus:border-transparent transition-all placeholder-[#94a3b8]"
                />
              </div>
              <button
                type="button"
                onClick={() => handleOpenDirectory(folderPathInput)}
                disabled={loading || !folderPathInput.trim()}
                className="px-4 py-2.5 rounded-xl bg-[#2563eb] hover:bg-[#1d4ed8] text-white text-xs font-semibold flex items-center gap-1.5 transition-all disabled:opacity-50 cursor-pointer shadow-sm hover:shadow-md"
              >
                {loading ? <RefreshCw className="w-3.5 h-3.5 animate-spin" /> : <ArrowRight className="w-3.5 h-3.5" />}
                <span>Open</span>
              </button>
            </div>
          </div>

          {/* Native Browse Button */}
          <button
            type="button"
            onClick={handleNativeBrowse}
            className="w-full py-3 px-4 rounded-xl border-2 border-dashed border-[#cbd5e1] dark:border-[#444444] bg-[#f8fafc] dark:bg-[#252526] hover:bg-[#f1f5f9] dark:hover:bg-[#2e2e2e] hover:border-[#2563eb] dark:hover:border-[#38bdf8] text-xs text-[#334155] dark:text-[#d1d5db] font-medium flex items-center justify-center gap-2 transition-all cursor-pointer group"
          >
            <HardDrive className="w-4 h-4 text-[#2563eb] dark:text-[#38bdf8] group-hover:scale-110 transition-transform" />
            <span>Browse with System File Chooser</span>
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

          {/* Recent Workspaces */}
          {recentWorkspaces.length > 0 && (
            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <span className="text-[11px] font-semibold text-[#64748b] dark:text-[#94a3b8] uppercase tracking-wider flex items-center gap-1.5">
                  <Clock className="w-3 h-3" />
                  Recent Workspaces
                </span>
                {recentWorkspaces.length > 3 && (
                  <div className="relative">
                    <Search className="absolute left-2 top-1/2 -translate-y-1/2 w-3 h-3 text-[#94a3b8]" />
                    <input
                      type="text"
                      placeholder="Filter..."
                      value={filterQuery}
                      onChange={e => setFilterQuery(e.target.value)}
                      className="pl-7 pr-2 py-1 text-[11px] bg-[#f1f5f9] dark:bg-[#252526] border border-[#e2e8f0] dark:border-[#383838] rounded-lg text-[#334155] dark:text-[#d1d5db] focus:outline-none focus:ring-1 focus:ring-[#2563eb] w-32 placeholder-[#94a3b8]"
                    />
                  </div>
                )}
              </div>
              <div className="space-y-1">
                {filteredRecent.map((recentPath, idx) => {
                  const folderName = recentPath.split('/').pop() || recentPath;
                  const isActive = recentPath === activeWorkspacePath;
                  return (
                    <button
                      key={idx}
                      type="button"
                      onClick={() => handleOpenDirectory(recentPath)}
                      className={`w-full p-2.5 rounded-xl flex items-center justify-between text-xs transition-all cursor-pointer text-left group ${
                        isActive
                          ? 'bg-[#eff6ff] dark:bg-[#1e293b] border border-[#bfdbfe] dark:border-[#1e3a5f]'
                          : 'hover:bg-[#f3f4f6] dark:hover:bg-[#2a2d2e] border border-transparent'
                      }`}
                    >
                      <div className="flex items-center gap-2.5 min-w-0 flex-1 pr-2">
                        <div className={`w-8 h-8 rounded-lg flex items-center justify-center shrink-0 ${
                          isActive
                            ? 'bg-[#2563eb] text-white'
                            : 'bg-[#f1f5f9] dark:bg-[#252526] text-[#64748b] dark:text-[#94a3b8] group-hover:bg-[#2563eb] group-hover:text-white transition-colors'
                        }`}>
                          <Folder className="w-4 h-4" />
                        </div>
                        <div className="min-w-0 flex-1">
                          <p className={`font-semibold truncate ${isActive ? 'text-[#2563eb] dark:text-[#60a5fa]' : 'text-[#0f172a] dark:text-white'}`}>
                            {folderName}
                          </p>
                          <p className="text-[11px] text-[#64748b] dark:text-[#94a3b8] font-mono truncate">
                            {recentPath}
                          </p>
                        </div>
                      </div>
                      <div className="flex items-center gap-1.5 shrink-0">
                        {isActive && (
                          <span className="text-[9px] px-1.5 py-0.5 rounded-full bg-[#2563eb] text-white font-semibold">
                            Active
                          </span>
                        )}
                        <ChevronRight className="w-3.5 h-3.5 text-[#94a3b8] group-hover:text-[#2563eb] dark:group-hover:text-[#60a5fa] transition-colors" />
                      </div>
                    </button>
                  );
                })}
              </div>
            </div>
          )}

          {/* Quick Suggestions */}
          <div className="space-y-1.5 pt-2 border-t border-[#f3f4f6] dark:border-[#2f2f31]">
            <span className="text-[11px] font-semibold text-[#64748b] dark:text-[#94a3b8] uppercase tracking-wider">
              Quick Access
            </span>
            <div className="grid grid-cols-2 gap-1.5">
              {quickFolders.map((q, idx) => (
                <button
                  key={idx}
                  type="button"
                  onClick={() => handleOpenDirectory(q.path)}
                  className="p-2.5 rounded-xl hover:bg-[#f3f4f6] dark:hover:bg-[#282828] flex items-center gap-2 text-xs transition-all cursor-pointer text-left group border border-transparent hover:border-[#e2e8f0] dark:hover:border-[#383838]"
                >
                  <span className="text-base">{q.icon}</span>
                  <div className="min-w-0 flex-1">
                    <p className="font-semibold text-[#0f172a] dark:text-white truncate text-[11px]">{q.label}</p>
                    <p className="text-[10px] text-[#64748b] dark:text-[#94a3b8] font-mono truncate">{q.path}</p>
                  </div>
                </button>
              ))}
            </div>
          </div>

        </div>

        {/* Footer */}
        <div className="p-3 bg-[#f8fafc] dark:bg-[#181818] border-t border-[#f3f4f6] dark:border-[#2f2f31] flex items-center justify-between text-[11px] text-[#64748b] dark:text-[#858585]">
          <span className="truncate max-w-[300px]">
            Current: <strong className="font-mono text-[#0f172a] dark:text-white">{activeWorkspacePath || 'None'}</strong>
          </span>
          <button
            type="button"
            onClick={onClose}
            className="px-3 py-1 rounded-lg hover:bg-[#e2e8f0] dark:hover:bg-[#282828] text-[#0f172a] dark:text-white transition-colors cursor-pointer font-medium"
          >
            Cancel
          </button>
        </div>

      </div>
    </div>
  );
};
