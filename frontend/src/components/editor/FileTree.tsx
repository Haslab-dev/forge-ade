import React, { useState, useMemo } from 'react';
import { 
  ChevronRight, 
  ChevronDown, 
  Folder, 
  FolderOpen, 
  FileText, 
  FileJson, 
  Tag, 
  Plus,
  RefreshCw,
  Search,
  GitBranch,
  Check,
  Download,
  Trash2,
  FilePlus,
  FolderPlus,
  GitCommit,
  GitCompare,
  GitPullRequest,
  Sparkles,
  X,
  ArrowUpRight,
  Eye,
  Filter
} from 'lucide-react';
import { FileItem } from '../../types';
import { useWorkspace } from '../../stores/workspaceStore';
import { ApiBridge } from '../../services/apiBridge';

export const FileTree: React.FC = () => {
  const { 
    files, 
    selectedFile, 
    openFileInEditor, 
    openDiffInEditor,
    diffs,
    createFile,
    createFolder,
    deleteFile,
    renameFile,
    openFolder,
    refreshFiles,
    activeWorkspacePath, 
    activeActivity,
    setActiveActivity,
    diagnostics,
    gitBranch,
    gitFiles,
    gitCommits,
    refreshGitStatus,
    refreshGitLog
  } = useWorkspace();

  const [expandedFolders, setExpandedFolders] = useState<Record<string, boolean>>({
    'root': true
  });

  const [isOutlineOpen, setIsOutlineOpen] = useState(false);
  const [isTimelineOpen, setIsTimelineOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [searchMode, setSearchMode] = useState<'files' | 'content'>('files');
  const [commitMessage, setCommitMessage] = useState('');
  const [isCommitted, setIsCommitted] = useState(false);
  const [isAiGenerating, setIsAiGenerating] = useState(false);
  const [gitView, setGitView] = useState<'changes' | 'graph'>('changes');
  const [selectedGitFile, setSelectedGitFile] = useState<string | null>(null);
  const [gitFileDiff, setGitFileDiff] = useState('');

  // New file/folder modal inline state
  const [isCreatingFile, setIsCreatingFile] = useState(false);
  const [newFileName, setNewFileName] = useState('');

  const toggleFolder = (folderId: string) => {
    setExpandedFolders(prev => ({
      ...prev,
      [folderId]: !prev[folderId]
    }));
  };

  const getFileIcon = (file: FileItem) => {
    if (file.type === 'folder') {
      const isOpen = expandedFolders[file.id];
      return isOpen ? (
        <FolderOpen className="w-4 h-4 text-[#dcb67a] shrink-0" />
      ) : (
        <Folder className="w-4 h-4 text-[#dcb67a] shrink-0" />
      );
    }

    if (file.name.endsWith('.md')) {
      return <span className="w-4 h-4 rounded bg-[#2563eb] text-white text-[9px] font-bold flex items-center justify-center shrink-0 font-mono">M↓</span>;
    }
    if (file.name.endsWith('.ts') || file.name.endsWith('.tsx')) {
      return <span className="w-4 h-4 text-[#3178c6] flex items-center justify-center shrink-0 font-bold text-[10px] font-mono">TS</span>;
    }
    if (file.name.endsWith('.js') || file.name.endsWith('.jsx')) {
      return <span className="w-4 h-4 text-[#f7df1e] flex items-center justify-center shrink-0 font-bold text-[10px] font-mono">JS</span>;
    }
    if (file.name.endsWith('.zig')) {
      return <span className="w-4 h-4 text-[#f7a41d] flex items-center justify-center shrink-0 font-bold text-[10px] font-mono">⚡</span>;
    }
    if (file.name.endsWith('.json')) {
      return <FileJson className="w-4 h-4 text-[#eab308] shrink-0" />;
    }
    if (file.name.includes('.lock') || file.name.endsWith('.phar')) {
      return <span className="w-4 h-4 text-[#9ca3af] flex items-center justify-center shrink-0">🔒</span>;
    }
    if (file.name === '.gitignore') {
      return <Tag className="w-4 h-4 text-[#ef4444] shrink-0" />;
    }
    if (/\.(png|jpg|jpeg|gif|svg|bmp|webp|ico)$/i.test(file.name)) {
      return <Eye className="w-4 h-4 text-[#a855f7] shrink-0" />;
    }
    if (file.name.endsWith('.pdf')) {
      return <FileText className="w-4 h-4 text-[#ef4444] shrink-0" />;
    }
    return <FileText className="w-4 h-4 text-[#9ca3af] shrink-0" />;
  };

  const getGitStatusIcon = (filePath: string) => {
    const gitFile = gitFiles.find(g => g.path === filePath || filePath.endsWith(g.path));
    if (!gitFile) return null;
    const s = gitFile.status.trim();
    if (s === '??') return <span className="text-[10px] font-bold text-[#16a34a]">U</span>;
    if (s.includes('M')) return <span className="text-[10px] font-bold text-[#d97706]">M</span>;
    if (s.includes('A')) return <span className="text-[10px] font-bold text-[#2563eb]">A</span>;
    if (s.includes('D')) return <span className="text-[10px] font-bold text-[#dc2626]">D</span>;
    if (s.includes('R')) return <span className="text-[10px] font-bold text-[#7c3aed]">R</span>;
    return <span className="text-[10px] font-bold text-[#9ca3af]">?</span>;
  };

  const renderItem = (item: FileItem, depth = 0) => {
    const isFolder = item.type === 'folder';
    const isExpanded = expandedFolders[item.id] ?? false;
    const isSelected = selectedFile?.path === item.path;
    const gitStatus = getGitStatusIcon(item.path);

    return (
      <div key={item.id} className="select-none">
        <div
          onClick={() => {
            if (isFolder) {
              toggleFolder(item.id);
            } else {
              openFileInEditor(item.path);
            }
          }}
          style={{ paddingLeft: `${depth * 14 + 10}px` }}
          className={`flex items-center gap-1.5 py-1 pr-2 text-xs cursor-pointer transition-colors group ${
            isSelected
              ? 'bg-[#e2e8f0] dark:bg-[#37373d] text-[#0f172a] dark:text-white font-medium'
              : 'text-[#334155] dark:text-[#cccccc] hover:bg-[#e2e8f0] dark:hover:bg-[#2a2d2e] hover:text-[#0f172a] dark:hover:text-white'
          }`}
        >
          {isFolder ? (
            <span className="w-3.5 h-3.5 flex items-center justify-center text-[#64748b] dark:text-[#858585] group-hover:text-[#0f172a] dark:group-hover:text-white">
              {isExpanded ? <ChevronDown className="w-3.5 h-3.5" /> : <ChevronRight className="w-3.5 h-3.5" />}
            </span>
          ) : (
            <span className="w-3.5 h-3.5" />
          )}
          {getFileIcon(item)}
          <span className="truncate text-[12px] flex-1">{item.name}</span>
          {gitStatus && <span className="shrink-0 ml-1">{gitStatus}</span>}
        </div>
        {isFolder && isExpanded && item.children && (
          <div>
            {item.children.map(child => renderItem(child, depth + 1))}
          </div>
        )}
      </div>
    );
  };

  const getAllFiles = (items: FileItem[]): FileItem[] => {
    let result: FileItem[] = [];
    for (const item of items) {
      if (item.type === 'file') result.push(item);
      if (item.children) result = result.concat(getAllFiles(item.children));
    }
    return result;
  };

  const allFilesList = useMemo(() => getAllFiles(files), [files]);

  const searchResults = useMemo(() => {
    if (!searchQuery.trim()) return [];
    const q = searchQuery.toLowerCase();
    if (searchMode === 'files') {
      return allFilesList.filter(f => 
        f.name.toLowerCase().includes(q) || f.path.toLowerCase().includes(q)
      );
    } else {
      const results: Array<{ file: FileItem; line: number; text: string }> = [];
      for (const file of allFilesList) {
        if (file.content) {
          const lines = file.content.split('\n');
          for (let i = 0; i < lines.length; i++) {
            if (lines[i].toLowerCase().includes(q)) {
              results.push({ file, line: i + 1, text: lines[i].trim() });
            }
          }
        }
      }
      return results;
    }
  }, [searchQuery, searchMode, allFilesList]);

  const handleAiCommit = async () => {
    setIsAiGenerating(true);
    try {
      const result = await ApiBridge.gitAiCommitMessage(activeWorkspacePath);
      setCommitMessage(result.message);
    } finally {
      setIsAiGenerating(false);
    }
  };

  const handleCommit = async () => {
    if (!commitMessage.trim()) return;
    const result = await ApiBridge.gitCommit(commitMessage, activeWorkspacePath);
    if (result.success) {
      setIsCommitted(true);
      setCommitMessage('');
      await refreshGitStatus();
      await refreshGitLog();
      setTimeout(() => setIsCommitted(false), 2000);
    }
  };

  const handleViewDiff = async (filePath: string) => {
    setSelectedGitFile(filePath);
    const diff = await ApiBridge.gitDiff(filePath, activeWorkspacePath);
    setGitFileDiff(diff);
  };

  const handleStage = async (filePath: string) => {
    await ApiBridge.gitStage(filePath, activeWorkspacePath);
    await refreshGitStatus();
  };

  const handleUnstage = async (filePath: string) => {
    await ApiBridge.gitUnstage(filePath, activeWorkspacePath);
    await refreshGitStatus();
  };

  // Render git graph as simple visual
  const renderGitGraph = () => {
    if (gitCommits.length === 0) {
      return <p className="text-[11px] text-[#9ca3af] italic py-4 text-center">No commits found</p>;
    }
    return (
      <div className="space-y-0">
        {gitCommits.slice(0, 30).map((commit, idx) => {
          const date = new Date(commit.timestamp * 1000);
          const timeAgo = getTimeAgo(date);
          const isMerge = commit.parents && commit.parents.length > 1;
          const shortHash = commit.hash.substring(0, 7);
          return (
            <div key={commit.hash} className="flex items-start gap-0 group">
              {/* Graph line */}
              <div className="flex flex-col items-center w-6 shrink-0">
                <div className={`w-2.5 h-2.5 rounded-full border-2 shrink-0 ${
                  idx === 0 ? 'bg-[#2563eb] border-[#2563eb]' : 'bg-transparent border-[#94a3b8] dark:border-[#555]'
                }`} />
                {idx < gitCommits.length - 1 && (
                  <div className="w-[1px] h-6 bg-[#cbd5e1] dark:bg-[#444]" />
                )}
              </div>
              {/* Commit info */}
              <div className="flex-1 py-1 pr-2 min-w-0">
                <div className="flex items-center gap-1.5">
                  <span className="text-[11px] font-medium text-[#0f172a] dark:text-[#e5e7eb] truncate">
                    {commit.message}
                  </span>
                  {isMerge && <GitPullRequest className="w-3 h-3 text-[#7c3aed] shrink-0" />}
                </div>
                <div className="flex items-center gap-2 mt-0.5">
                  <span className="text-[10px] font-mono text-[#2563eb] dark:text-[#60a5fa]">{shortHash}</span>
                  <span className="text-[10px] text-[#9ca3af]">{commit.author}</span>
                  <span className="text-[10px] text-[#9ca3af]">·</span>
                  <span className="text-[10px] text-[#9ca3af]">{timeAgo}</span>
                </div>
              </div>
            </div>
          );
        })}
      </div>
    );
  };

  const getTimeAgo = (date: Date) => {
    const seconds = Math.floor((Date.now() - date.getTime()) / 1000);
    if (seconds < 60) return 'just now';
    if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`;
    if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`;
    if (seconds < 604800) return `${Math.floor(seconds / 86400)}d ago`;
    return date.toLocaleDateString();
  };

  const getStatusLabel = (status: string) => {
    const s = status.trim();
    if (s === '??') return 'Untracked';
    if (s === 'M ' || s === ' M') return 'Modified';
    if (s === 'A ') return 'Added';
    if (s === 'D ') return 'Deleted';
    if (s === 'R ') return 'Renamed';
    if (s.includes('M')) return 'Modified';
    return s;
  };

  const getStatusColor = (status: string) => {
    const s = status.trim();
    if (s === '??') return 'text-[#16a34a]';
    if (s.includes('M')) return 'text-[#d97706]';
    if (s.includes('A')) return 'text-[#2563eb]';
    if (s.includes('D')) return 'text-[#dc2626]';
    return 'text-[#9ca3af]';
  };

  return (
    <div className="w-[240px] min-w-[220px] max-w-[320px] bg-[#f8fafc] dark:bg-[#252526] text-[#334155] dark:text-[#cccccc] border-r border-[#e2e8f0] dark:border-[#1e1e1e] flex flex-col justify-between h-full select-none overflow-hidden font-sans transition-colors duration-150">
      
      {/* Activity: Explorer */}
      {activeActivity === 'explorer' && (
        <div className="flex-1 flex flex-col overflow-hidden">
          <div className="px-3 py-2 text-[11px] font-semibold text-[#64748b] dark:text-[#858585] uppercase tracking-wider flex items-center justify-between border-b border-[#e2e8f0] dark:border-[#1e1e1e]">
            <span>Explorer</span>
            <div className="flex items-center gap-1">
              <button type="button" onClick={() => setIsCreatingFile(true)} className="p-0.5 hover:bg-[#e2e8f0] dark:hover:bg-[#333333] rounded text-[#64748b] dark:text-[#858585] hover:text-[#0f172a] dark:hover:text-white transition-colors" title="New File">
                <Plus className="w-3 h-3" />
              </button>
              <button type="button" onClick={() => refreshFiles()} className="p-0.5 hover:bg-[#e2e8f0] dark:hover:bg-[#333333] rounded text-[#64748b] dark:text-[#858585] hover:text-[#0f172a] dark:hover:text-white transition-colors" title="Refresh Explorer">
                <RefreshCw className="w-3 h-3" />
              </button>
              <button type="button" onClick={() => openFolder()} className="p-0.5 hover:bg-[#e2e8f0] dark:hover:bg-[#333333] rounded text-[#64748b] dark:text-[#858585] hover:text-[#0f172a] dark:hover:text-white transition-colors" title="Open Workspace Folder">
                <FolderOpen className="w-3.5 h-3.5 text-[#2563eb] dark:text-[#38bdf8]" />
              </button>
            </div>
          </div>

          {isCreatingFile && (
            <div className="p-2 bg-white dark:bg-[#1e1e1e] border-b border-[#e2e8f0] dark:border-[#333] flex items-center gap-1">
              <FilePlus className="w-3.5 h-3.5 text-[#2563eb] dark:text-[#38bdf8] shrink-0" />
              <input
                type="text" autoFocus placeholder="filename.ext" value={newFileName}
                onChange={e => setNewFileName(e.target.value)}
                onKeyDown={async e => {
                  if (e.key === 'Enter' && newFileName.trim()) {
                    await createFile(newFileName.trim());
                    setIsCreatingFile(false);
                    setNewFileName('');
                  } else if (e.key === 'Escape') {
                    setIsCreatingFile(false);
                  }
                }}
                className="flex-1 bg-transparent border-0 text-[#0f172a] dark:text-white font-mono text-xs focus:outline-none p-0 placeholder-[#94a3b8]"
              />
            </div>
          )}

          <div onClick={() => toggleFolder('root')} className="px-2 py-1.5 flex items-center gap-1 text-[11px] font-bold uppercase text-[#0f172a] dark:text-[#cccccc] hover:bg-[#e2e8f0] dark:hover:bg-[#2a2d2e] cursor-pointer tracking-wider transition-colors">
            {expandedFolders['root'] ? <ChevronDown className="w-3.5 h-3.5 text-[#64748b] dark:text-[#858585]" /> : <ChevronRight className="w-3.5 h-3.5 text-[#64748b] dark:text-[#858585]" />}
            <span>{activeWorkspacePath.split('/').pop()}</span>
          </div>

          {expandedFolders['root'] && (
            <div className="flex-1 overflow-y-auto py-0.5">
              {files.map(item => renderItem(item, 0))}
            </div>
          )}

          <div className="border-t border-[#e2e8f0] dark:border-[#1e1e1e] bg-[#f8fafc] dark:bg-[#252526] text-xs">
            <div onClick={() => setIsOutlineOpen(prev => !prev)} className="px-3 py-1.5 flex items-center gap-1 text-[11px] font-semibold text-[#64748b] dark:text-[#858585] uppercase tracking-wider hover:bg-[#e2e8f0] dark:hover:bg-[#2a2d2e] cursor-pointer transition-colors">
              {isOutlineOpen ? <ChevronDown className="w-3.5 h-3.5" /> : <ChevronRight className="w-3.5 h-3.5" />}
              <span>Outline</span>
            </div>
            {isOutlineOpen && (
              <div className="px-5 py-2 text-[11px] text-[#64748b] dark:text-[#9ca3af] space-y-1 bg-white/70 dark:bg-[#1e1e1e]/40">
                {selectedFile?.content ? (
                  selectedFile.content.split('\n')
                    .filter(l => l.startsWith('#') || l.startsWith('export ') || l.startsWith('function ') || l.startsWith('class ') || l.startsWith('pub fn'))
                    .slice(0, 8)
                    .map((item, idx) => (
                      <p key={idx} className="hover:text-[#0f172a] dark:hover:text-white cursor-pointer truncate font-mono text-[10px]">{item.trim()}</p>
                    ))
                ) : (
                  <p className="italic text-[10px] text-[#9ca3af]">Select a file to inspect outline symbols</p>
                )}
              </div>
            )}
            <div onClick={() => setIsTimelineOpen(prev => !prev)} className="px-3 py-1.5 flex items-center gap-1 text-[11px] font-semibold text-[#64748b] dark:text-[#858585] uppercase tracking-wider hover:bg-[#e2e8f0] dark:hover:bg-[#2a2d2e] cursor-pointer border-t border-[#e2e8f0] dark:border-[#1e1e1e] transition-colors">
              {isTimelineOpen ? <ChevronDown className="w-3.5 h-3.5" /> : <ChevronRight className="w-3.5 h-3.5" />}
              <span>Timeline</span>
            </div>
            {isTimelineOpen && (
              <div className="px-5 py-2 text-[11px] text-[#64748b] dark:text-[#9ca3af] bg-white/70 dark:bg-[#1e1e1e]/40">
                <p>Git: Initial Commit (2 hours ago)</p>
              </div>
            )}
          </div>
        </div>
      )}

      {/* Activity: Search (⌘⇧F) - with content search and line numbers */}
      {activeActivity === 'search' && (
        <div className="flex-1 flex flex-col overflow-hidden">
          <div className="p-3 space-y-2 border-b border-[#e2e8f0] dark:border-[#1e1e1e]">
            <div className="text-[11px] font-semibold text-[#64748b] dark:text-[#858585] uppercase tracking-wider">
              Search
            </div>
            <div className="relative">
              <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-[#94a3b8]" />
              <input
                type="text" autoFocus value={searchQuery}
                onChange={e => setSearchQuery(e.target.value)}
                placeholder={searchMode === 'files' ? 'Search file names...' : 'Search file contents...'}
                className="w-full pl-8 pr-2.5 py-1.5 bg-white dark:bg-[#1e1e1e] border border-[#cbd5e1] dark:border-[#3c3c3c] focus:border-[#2563eb] dark:focus:border-[#007acc] rounded-lg text-xs text-[#0f172a] dark:text-white placeholder-[#94a3b8] dark:placeholder-[#737373] focus:outline-none"
              />
            </div>
            <div className="flex items-center gap-1">
              <button
                type="button" onClick={() => setSearchMode('files')}
                className={`px-2 py-0.5 rounded text-[10px] font-medium transition-colors ${searchMode === 'files' ? 'bg-[#2563eb] text-white' : 'text-[#64748b] dark:text-[#858585] hover:bg-[#e2e8f0] dark:hover:bg-[#333]'}`}
              >
                Files
              </button>
              <button
                type="button" onClick={() => setSearchMode('content')}
                className={`px-2 py-0.5 rounded text-[10px] font-medium transition-colors ${searchMode === 'content' ? 'bg-[#2563eb] text-white' : 'text-[#64748b] dark:text-[#858585] hover:bg-[#e2e8f0] dark:hover:bg-[#333]'}`}
              >
                Content
              </button>
              {searchQuery && (
                <span className="text-[10px] text-[#9ca3af] ml-auto">
                  {searchMode === 'content' ? `${(searchResults as any[]).length} matches` : `${(searchResults as FileItem[]).length} results`}
                </span>
              )}
            </div>
          </div>

          <div className="flex-1 overflow-y-auto p-2 space-y-1">
            {searchMode === 'files' ? (
              (searchResults as FileItem[]).map(file => (
                <div
                  key={file.id}
                  onClick={() => openFileInEditor(file.path)}
                  className="p-2 rounded-lg hover:bg-[#e2e8f0] dark:hover:bg-[#2e2e32] cursor-pointer text-xs space-y-0.5 transition-colors"
                >
                  <div className="flex items-center gap-1.5 font-medium text-[#0f172a] dark:text-white">
                    {getFileIcon(file)}
                    <span>{file.name}</span>
                  </div>
                  <div className="text-[10px] text-[#64748b] dark:text-[#858585] font-mono truncate pl-5.5">
                    {file.path}
                  </div>
                </div>
              ))
            ) : (
              (searchResults as Array<{ file: FileItem; line: number; text: string }>).map((result, idx) => (
                <div
                  key={idx}
                  onClick={() => openFileInEditor(result.file.path)}
                  className="p-2 rounded-lg hover:bg-[#e2e8f0] dark:hover:bg-[#2e2e32] cursor-pointer text-xs transition-colors"
                >
                  <div className="flex items-center gap-1.5 text-[#0f172a] dark:text-white">
                    {getFileIcon(result.file)}
                    <span className="font-medium truncate">{result.file.name}</span>
                    <span className="text-[10px] text-[#2563eb] dark:text-[#60a5fa] font-mono shrink-0">:{result.line}</span>
                  </div>
                  <div className="text-[10px] text-[#64748b] dark:text-[#858585] font-mono truncate pl-5.5 mt-0.5">
                    {result.text.substring(0, 80)}
                  </div>
                </div>
              ))
            )}
            {searchQuery && (searchResults as any[]).length === 0 && (
              <p className="text-[11px] text-[#9ca3af] italic py-4 text-center">No results found</p>
            )}
          </div>
        </div>
      )}

      {/* Activity: Source Control (Git) - with diff, graph, AI commit */}
      {activeActivity === 'git' && (
        <div className="flex-1 flex flex-col overflow-hidden">
          {/* Git Header */}
          <div className="p-3 border-b border-[#e2e8f0] dark:border-[#1e1e1e] space-y-2">
            <div className="flex items-center justify-between text-[11px] font-semibold text-[#64748b] dark:text-[#858585] uppercase tracking-wider">
              <span className="flex items-center gap-1.5">
                <GitBranch className="w-3.5 h-3.5 text-[#2563eb]" />
                Source Control
              </span>
              <div className="flex items-center gap-1">
                <span className="text-[10px] px-1.5 py-0.2 rounded bg-[#eff6ff] dark:bg-[#1e293b] text-[#2563eb] dark:text-[#38bdf8] font-mono">
                  {gitBranch}
                </span>
                <button onClick={() => { refreshGitStatus(); refreshGitLog(); }} className="p-0.5 hover:bg-[#e2e8f0] dark:hover:bg-[#333] rounded transition-colors" title="Refresh">
                  <RefreshCw className="w-3 h-3" />
                </button>
              </div>
            </div>

            {/* View Toggle: Changes | Graph */}
            <div className="flex bg-[#e2e8f0] dark:bg-[#1e1e1e] p-0.5 rounded-lg">
              <button
                type="button" onClick={() => setGitView('changes')}
                className={`flex-1 px-2 py-0.5 rounded text-[10px] font-medium transition-colors flex items-center justify-center gap-1 ${gitView === 'changes' ? 'bg-white dark:bg-[#333] text-[#0f172a] dark:text-white shadow-xs' : 'text-[#64748b] dark:text-[#858585]'}`}
              >
                <GitCompare className="w-3 h-3" />
                Changes
                {gitFiles.length > 0 && <span className="ml-0.5 px-1 py-0 rounded-full bg-[#2563eb] text-white text-[9px]">{gitFiles.length}</span>}
              </button>
              <button
                type="button" onClick={() => setGitView('graph')}
                className={`flex-1 px-2 py-0.5 rounded text-[10px] font-medium transition-colors flex items-center justify-center gap-1 ${gitView === 'graph' ? 'bg-white dark:bg-[#333] text-[#0f172a] dark:text-white shadow-xs' : 'text-[#64748b] dark:text-[#858585]'}`}
              >
                <GitCommit className="w-3 h-3" />
                Graph
              </button>
            </div>
          </div>

          <div className="flex-1 overflow-y-auto">
            {gitView === 'changes' ? (
              <div className="p-3 space-y-3">
                {/* AI Commit Section */}
                <div className="space-y-2">
                  <div className="flex items-center gap-1.5">
                    <textarea
                      rows={2} value={commitMessage}
                      onChange={e => setCommitMessage(e.target.value)}
                      placeholder="Commit message..."
                      className="flex-1 bg-white dark:bg-[#1e1e1e] border border-[#cbd5e1] dark:border-[#3c3c3c] rounded-lg p-2 text-xs text-[#0f172a] dark:text-white placeholder-[#94a3b8] dark:placeholder-[#737373] focus:outline-none resize-none"
                      onKeyDown={e => { if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') handleCommit(); }}
                    />
                  </div>
                  <div className="flex gap-1.5">
                    <button
                      type="button" onClick={handleAiCommit}
                      disabled={isAiGenerating || gitFiles.length === 0}
                      className="flex-1 py-1.5 rounded-lg bg-gradient-to-r from-[#7c3aed] to-[#2563eb] hover:from-[#6d28d9] hover:to-[#1d4ed8] text-white text-xs font-semibold flex items-center justify-center gap-1.5 transition-all disabled:opacity-50 cursor-pointer"
                    >
                      {isAiGenerating ? (
                        <RefreshCw className="w-3.5 h-3.5 animate-spin" />
                      ) : (
                        <Sparkles className="w-3.5 h-3.5" />
                      )}
                      <span>{isAiGenerating ? 'Generating...' : 'AI Commit'}</span>
                    </button>
                    <button
                      type="button" onClick={handleCommit}
                      disabled={!commitMessage.trim() || gitFiles.length === 0}
                      className="flex-1 py-1.5 rounded-lg bg-[#16a34a] hover:bg-[#15803d] text-white text-xs font-semibold flex items-center justify-center gap-1.5 transition-colors disabled:opacity-50 cursor-pointer"
                    >
                      {isCommitted ? <Check className="w-3.5 h-3.5" /> : <GitCommit className="w-3.5 h-3.5" />}
                      <span>{isCommitted ? 'Committed!' : 'Commit'}</span>
                    </button>
                  </div>
                </div>

                {/* Changed Files List */}
                <div className="space-y-1">
                  <div className="text-[11px] font-semibold text-[#64748b] dark:text-[#858585] uppercase flex items-center justify-between">
                    <span>Changes</span>
                    <span className="text-[10px] px-1.5 rounded-full bg-[#e2e8f0] dark:bg-[#333] text-[#0f172a] dark:text-white font-mono">
                      {gitFiles.length}
                    </span>
                  </div>
                  {gitFiles.length === 0 ? (
                    <p className="text-[11px] text-[#9ca3af] italic py-3 text-center">Working tree clean</p>
                  ) : (
                    gitFiles.map((gf, idx) => (
                      <div key={idx} className="space-y-1">
                        <div
                          onClick={() => handleViewDiff(gf.path)}
                          className="p-1.5 rounded-lg hover:bg-[#e2e8f0] dark:hover:bg-[#1e1e1e] cursor-pointer flex items-center justify-between text-xs transition-colors group"
                        >
                          <div className="flex items-center gap-1.5 min-w-0 flex-1">
                            <FileText className="w-3.5 h-3.5 text-[#2563eb] dark:text-[#3b82f6] shrink-0" />
                            <span className="truncate max-w-[130px] font-mono text-[#334155] dark:text-[#d1d5db]">{gf.path.split('/').pop()}</span>
                            <span className={`text-[10px] font-bold ${getStatusColor(gf.status)}`}>{getStatusLabel(gf.status)}</span>
                          </div>
                          <div className="flex items-center gap-1 shrink-0 opacity-0 group-hover:opacity-100 transition-opacity">
                            <button onClick={(e) => { e.stopPropagation(); handleStage(gf.path); }} className="p-0.5 hover:bg-[#dcfce7] rounded text-[#16a34a]" title="Stage">
                              <Plus className="w-3 h-3" />
                            </button>
                            <Eye className="w-3 h-3 text-[#64748b]" />
                          </div>
                        </div>
                        {selectedGitFile === gf.path && gitFileDiff && (
                          <div className="ml-4 p-2 bg-[#f8fafc] dark:bg-[#1a1a1a] rounded-lg border border-[#e2e8f0] dark:border-[#333] text-[10px] font-mono overflow-x-auto max-h-32 overflow-y-auto">
                            <pre className="whitespace-pre-wrap text-[#334155] dark:text-[#d1d5db]">
                              {gitFileDiff.split('\n').map((line, i) => {
                                const color = line.startsWith('+') ? 'text-[#16a34a]' : line.startsWith('-') ? 'text-[#dc2626]' : 'text-[#64748b]';
                                return <div key={i} className={color}>{line || ' '}</div>;
                              })}
                            </pre>
                          </div>
                        )}
                      </div>
                    ))
                  )}
                </div>
              </div>
            ) : (
              /* Git Graph View */
              <div className="p-3">
                <div className="text-[11px] font-semibold text-[#64748b] dark:text-[#858585] uppercase mb-2">Commit Graph</div>
                {renderGitGraph()}
              </div>
            )}
          </div>
        </div>
      )}

    </div>
  );
};
