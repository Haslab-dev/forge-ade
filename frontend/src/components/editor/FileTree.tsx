import React, { useState, useRef, useEffect, useCallback, useMemo } from 'react';
import { 
  ChevronRight, 
  ChevronDown, 
  Folder, 
  FolderOpen, 
  FileText, 
  RefreshCw, 
  Search, 
  GitBranch, 
  Check, 
  Trash2, 
  FilePlus, 
  FolderPlus, 
  GitCommit, 
  GitPullRequest, 
  Sparkles, 
  X, 
  MoreHorizontal, 
  Ban, 
  List, 
  FolderTree, 
  ChevronsDownUp, 
  ChevronsUpDown,
  Minimize2, 
  Plus, 
  Undo2, 
  ExternalLink, 
  ArrowUp, 
  Download, 
  Copy,
  Scissors,
  Clipboard,
  Terminal,
  FileCode
} from 'lucide-react';
import { FileItem } from '../../types';
import { useWorkspace } from '../../stores/workspaceStore';
import { ApiBridge } from '../../services/apiBridge';

export const FileTree: React.FC = () => {
  const { 
    files, 
    selectedFile, 
    openFileInEditor, 
    createFile, 
    createFolder, 
    deleteFile, 
    renameFile, 
    refreshFiles, 
    activeWorkspacePath, 
    activeActivity, 
    gitBranch, 
    gitFiles, 
    gitCommits, 
    refreshGitStatus, 
    refreshGitLog,
    openDiffInEditor,
    updateFolderChildren
  } = useWorkspace();

  const [expandedFolders, setExpandedFolders] = useState<Record<string, boolean>>({
    'root': true
  });

  const [isOutlineOpen, setIsOutlineOpen] = useState(false);
  const [isTimelineOpen, setIsTimelineOpen] = useState(false);

  // Search State (Clone Screenshot 1)
  const [searchQuery, setSearchQuery] = useState('');
  const [replaceQuery, setReplaceQuery] = useState('');
  const [showReplace, setShowReplace] = useState(true);
  const [matchCase, setMatchCase] = useState(false);
  const [matchWholeWord, setMatchWholeWord] = useState(false);
  const [useRegex, setUseRegex] = useState(false);
  const [preserveCase, setPreserveCase] = useState(false);
  const [searchViewMode, setSearchViewMode] = useState<'tree' | 'list'>('tree');
  const [expandedSearchFiles, setExpandedSearchFiles] = useState<Record<string, boolean>>({
    'app_test.go': true,
    'app.go': true,
    'go.mod': true
  });
  const [selectedSearchResult, setSelectedSearchResult] = useState<string | null>('app.go-23');
  const [searchResults, setSearchResults] = useState<Array<{
    file: string;
    filePath: string;
    count: number;
    matches: Array<{ id: string; line: number; text: string; match: string; column?: number }>;
  }>>([]);
  const [isSearching, setIsSearching] = useState(false);

  // Git Source Control State (Clone Screenshot 2)
  const [commitMessage, setCommitMessage] = useState('');
  const [isAiGenerating, setIsAiGenerating] = useState(false);
  const [isChangesExpanded, setIsChangesExpanded] = useState(true);
  const [isGraphExpanded, setIsGraphExpanded] = useState(true);
  const [isGitMenuOpen, setIsGitMenuOpen] = useState(false);
  const [selectedCommitHash, setSelectedCommitHash] = useState<string | null>(null);

  // Inline Create / Rename State
  const [isCreatingFile, setIsCreatingFile] = useState(false);
  const [isCreatingFolder, setIsCreatingFolder] = useState(false);
  const [createParentPath, setCreateParentPath] = useState<string>('');
  const [newItemName, setNewItemName] = useState('');
  const [renamingItemId, setRenamingItemId] = useState<string | null>(null);
  const [renameNewName, setRenameNewName] = useState('');

  // Context Menu State
  const [contextMenu, setContextMenu] = useState<{
    x: number;
    y: number;
    item: FileItem | null;
  } | null>(null);

  // Clipboard State for Cut/Copy/Paste
  const [clipboardAction, setClipboardAction] = useState<{ type: 'cut' | 'copy'; path: string } | null>(null);

  // Drag & Drop State
  const [draggedItem, setDraggedItem] = useState<FileItem | null>(null);
  const [dragOverFolderId, setDragOverFolderId] = useState<string | null>(null);

  // Resizable Sidebar width state
  const [sidebarWidth, setSidebarWidth] = useState(260);
  const isResizingRef = useRef(false);

  const workspaceName = activeWorkspacePath ? activeWorkspacePath.split('/').pop() || 'forge-ade' : 'forge-ade';

  // Toggle Folder (supports instant expand + lazy loading children if needed)
  const toggleFolder = async (folderId: string, folderPath?: string) => {
    const isCurrentlyExpanded = Boolean(expandedFolders[folderId] || (folderPath && expandedFolders[folderPath]));
    const nextState = !isCurrentlyExpanded;

    setExpandedFolders(prev => {
      const updated = { ...prev, [folderId]: nextState };
      if (folderPath && folderPath !== folderId) {
        updated[folderPath] = nextState;
      }
      return updated;
    });

    // If opening and folderPath is available, ensure children are loaded
    if (nextState && folderPath && folderPath !== 'root') {
      try {
        const children = await ApiBridge.listDirectory(folderPath);
        if (children && children.length > 0) {
          updateFolderChildren(folderPath, children);
        }
      } catch (err) {
        console.warn('Error expanding folder:', err);
      }
    }
  };

  // Collapse All Folders
  const collapseAllFolders = () => {
    setExpandedFolders({ 'root': true });
  };

  // Expand All Folders
  const expandAllFolders = () => {
    const all: Record<string, boolean> = { 'root': true };
    const mark = (items: FileItem[]) => {
      items.forEach(it => {
        if (it.type === 'folder') {
          all[it.id] = true;
          if (it.children) mark(it.children);
        }
      });
    };
    mark(files);
    setExpandedFolders(all);
  };

  // Close context menu on window click
  useEffect(() => {
    const handleClick = () => setContextMenu(null);
    window.addEventListener('click', handleClick);
    return () => window.removeEventListener('click', handleClick);
  }, []);

  // Sidebar drag resizer
  const handleMouseDownResizer = (e: React.MouseEvent) => {
    e.preventDefault();
    isResizingRef.current = true;
    const startX = e.clientX;
    const startWidth = sidebarWidth;

    const handleMouseMove = (moveEvent: MouseEvent) => {
      if (!isResizingRef.current) return;
      const newWidth = Math.max(180, Math.min(600, startWidth + (moveEvent.clientX - startX)));
      setSidebarWidth(newWidth);
    };

    const handleMouseUp = () => {
      isResizingRef.current = false;
      window.removeEventListener('mousemove', handleMouseMove);
      window.removeEventListener('mouseup', handleMouseUp);
    };

    window.addEventListener('mousemove', handleMouseMove);
    window.addEventListener('mouseup', handleMouseUp);
  };

  // Real Search Execution
  const runSearch = useCallback(async (query: string) => {
    if (!query.trim()) {
      setSearchResults([]);
      return;
    }
    setIsSearching(true);
    try {
      const results = await ApiBridge.searchContent({
        query: query.trim(),
        caseSensitive: matchCase,
        wholeWord: matchWholeWord,
        isRegex: useRegex,
        maxResults: 200
      });

      if (results && results.length > 0) {
        const grouped: Record<string, Array<{ id: string; line: number; text: string; match: string }>> = {};
        for (const res of results) {
          const filePath = res.path;
          if (!grouped[filePath]) grouped[filePath] = [];
          grouped[filePath].push({
            id: `${filePath}-${res.line}-${grouped[filePath].length}`,
            line: res.line,
            text: res.text,
            match: query
          });
        }
        const treeResults = Object.keys(grouped).map(filePath => ({
          file: filePath.split('/').pop() || filePath,
          filePath,
          count: grouped[filePath].length,
          matches: grouped[filePath]
        }));
        setSearchResults(treeResults);
      } else {
        // Search in-memory file tree fallback
        const localMatches: Record<string, Array<{ id: string; line: number; text: string; match: string }>> = {};
        const scan = (items: FileItem[]) => {
          for (const it of items) {
            if (it.type === 'file' && it.content) {
              const lines = it.content.split('\n');
              lines.forEach((lineText, idx) => {
                let matches = false;
                if (useRegex) {
                  try {
                    const regex = new RegExp(query, matchCase ? 'g' : 'gi');
                    matches = regex.test(lineText);
                  } catch {}
                } else if (matchWholeWord) {
                  const regex = new RegExp(`\\b${query}\\b`, matchCase ? 'g' : 'gi');
                  matches = regex.test(lineText);
                } else if (matchCase) {
                  matches = lineText.includes(query);
                } else {
                  matches = lineText.toLowerCase().includes(query.toLowerCase());
                }
                if (matches) {
                  if (!localMatches[it.path]) localMatches[it.path] = [];
                  localMatches[it.path].push({
                    id: `${it.path}-${idx + 1}`,
                    line: idx + 1,
                    text: lineText.trim(),
                    match: query
                  });
                }
              });
            }
            if (it.children) scan(it.children);
          }
        };
        scan(files);
        const treeResults = Object.keys(localMatches).map(filePath => ({
          file: filePath.split('/').pop() || filePath,
          filePath,
          count: localMatches[filePath].length,
          matches: localMatches[filePath]
        }));
        setSearchResults(treeResults);
      }
    } finally {
      setIsSearching(false);
    }
  }, [matchCase, matchWholeWord, useRegex, files]);

  // Debounced search on input change
  useEffect(() => {
    const timer = setTimeout(() => {
      runSearch(searchQuery);
    }, 250);
    return () => clearTimeout(timer);
  }, [searchQuery, runSearch]);

  const totalMatchesCount = useMemo(() => {
    return searchResults.reduce((acc, curr) => acc + curr.count, 0);
  }, [searchResults]);

  // Replace Single Match
  const handleReplaceSingleMatch = async (filePath: string, matchId: string, line: number) => {
    const fileContent = await ApiBridge.readFile(filePath);
    if (!fileContent) return;
    const lines = fileContent.split('\n');
    if (lines.length >= line) {
      lines[line - 1] = lines[line - 1].replace(searchQuery, replaceQuery);
      const newContent = lines.join('\n');
      await ApiBridge.writeFile(filePath, newContent);
      setSearchResults(prev => prev.map(group => {
        if (group.filePath === filePath) {
          const filtered = group.matches.filter(m => m.id !== matchId);
          return { ...group, matches: filtered, count: filtered.length };
        }
        return group;
      }).filter(g => g.count > 0));
    }
  };

  // Replace All Matches
  const handleReplaceAll = async () => {
    await ApiBridge.searchReplaceAll({
      query: searchQuery,
      replaceText: replaceQuery,
      caseSensitive: matchCase,
      wholeWord: matchWholeWord,
      isRegex: useRegex,
      preserveCase
    });
    await refreshFiles();
    runSearch(searchQuery);
  };

  // File Icon Renderer Matching Screenshot 3
  const getFileIcon = (file: FileItem) => {
    if (file.type === 'folder') {
      const isOpen = expandedFolders[file.id];
      const isFrontend = file.name === 'frontend';
      return isOpen ? (
        <FolderOpen className={`w-3.5 h-3.5 shrink-0 ${isFrontend ? 'text-[#f97316]' : 'text-[#6b7280] dark:text-[#9ca3af]'}`} />
      ) : (
        <Folder className={`w-3.5 h-3.5 shrink-0 ${isFrontend ? 'text-[#f97316]' : 'text-[#6b7280] dark:text-[#9ca3af]'}`} />
      );
    }

    const n = file.name.toLowerCase();
    if (n.endsWith('.go')) {
      return <span className="w-3.5 h-3.5 text-[#00add8] font-bold text-[9px] flex items-center justify-center shrink-0 font-mono">GO</span>;
    }
    if (n.endsWith('.md')) {
      return <span className="w-3.5 h-3.5 rounded bg-[#2563eb] text-white text-[8px] font-bold flex items-center justify-center shrink-0 font-mono">M↓</span>;
    }
    if (n.endsWith('.php')) {
      return <span className="w-3.5 h-3.5 text-[#8b5cf6] font-bold text-[9px] flex items-center justify-center shrink-0 font-mono">php</span>;
    }
    if (n.endsWith('.ts') || n.endsWith('.tsx')) {
      return <span className="w-3.5 h-3.5 text-[#3178c6] font-bold text-[9px] flex items-center justify-center shrink-0 font-mono">TS</span>;
    }
    if (n.endsWith('.js') || n.endsWith('.jsx')) {
      return <span className="w-3.5 h-3.5 text-[#eab308] font-bold text-[9px] flex items-center justify-center shrink-0 font-mono">JS</span>;
    }
    if (n.endsWith('.json')) {
      return <span className="text-[#eab308] font-bold text-[10px] font-mono shrink-0">{'{}'}</span>;
    }
    if (n.endsWith('.yml') || n.endsWith('.yaml')) {
      return <span className="text-[#8b5cf6] font-bold text-[9px] font-mono shrink-0">Y</span>;
    }
    if (n === 'makefile') {
      return <span className="text-[#64748b] text-[11px] shrink-0">⚙</span>;
    }
    if (n === '.gitignore') {
      return <span className="text-[#f97316] text-[10px] font-bold shrink-0">⑂</span>;
    }
    if (/\.(png|jpg|jpeg|gif|svg|ico|icns)$/i.test(n)) {
      return <span className="text-[#c084fc] text-[11px] shrink-0">🎨</span>;
    }
    if (n.endsWith('.txt') || n.endsWith('.workspace')) {
      return <FileText className="w-3.5 h-3.5 text-[#9ca3af] shrink-0" />;
    }
    return <FileText className="w-3.5 h-3.5 text-[#9ca3af] shrink-0" />;
  };

  // Drag and drop handlers
  const handleDragStart = (e: React.DragEvent, item: FileItem) => {
    e.stopPropagation();
    setDraggedItem(item);
    e.dataTransfer.setData('text/plain', item.path);
  };

  const handleDragOver = (e: React.DragEvent, folderId: string) => {
    e.preventDefault();
    e.stopPropagation();
    setDragOverFolderId(folderId);
  };

  const handleDrop = async (e: React.DragEvent, targetFolder: FileItem | null) => {
    e.preventDefault();
    e.stopPropagation();
    setDragOverFolderId(null);
    if (!draggedItem) return;

    const targetDir = targetFolder ? targetFolder.path : activeWorkspacePath;
    const newPath = `${targetDir}/${draggedItem.name}`;
    if (draggedItem.path !== newPath) {
      await ApiBridge.moveFile(draggedItem.path, newPath);
      await refreshFiles();
    }
    setDraggedItem(null);
  };

  // Context Menu Trigger
  const handleContextMenu = (e: React.MouseEvent, item: FileItem | null) => {
    e.preventDefault();
    e.stopPropagation();
    setContextMenu({
      x: e.clientX,
      y: e.clientY,
      item
    });
  };

  // Render Explorer Item
  const renderItem = (item: FileItem, depth = 0) => {
    const isFolder = item.type === 'folder';
    const isExpanded = Boolean(expandedFolders[item.id] || (item.path && expandedFolders[item.path]));
    const isSelected = selectedFile?.path === item.path;
    const isRenaming = renamingItemId === item.id;
    const isDragOver = dragOverFolderId === item.id;
    const isFrontend = item.name === 'frontend';
    const isBuild = item.name === 'build';
    const hasModified = item.name === 'Makefile' || gitFiles.some(g => g.path === item.path || g.path.endsWith(item.name));

    return (
      <div 
        key={item.id} 
        className="select-none"
        draggable
        onDragStart={(e) => handleDragStart(e, item)}
        onDragOver={(e) => isFolder && handleDragOver(e, item.id)}
        onDragLeave={() => setDragOverFolderId(null)}
        onDrop={(e) => isFolder && handleDrop(e, item)}
        onContextMenu={(e) => handleContextMenu(e, item)}
      >
        <div
          onClick={() => {
            if (isFolder) {
              toggleFolder(item.id, item.path);
            } else {
              openFileInEditor(item.path);
            }
          }}
          style={{ paddingLeft: `${depth * 12 + 8}px` }}
          className={`flex items-center gap-1.5 py-0.5 pr-2 text-xs cursor-pointer transition-colors group relative ${
            isSelected
              ? 'bg-[#eff6ff] dark:bg-[#1e293b] text-[#2563eb] dark:text-[#60a5fa] font-medium border border-[#3b82f6]/50 rounded-xs'
              : isDragOver
              ? 'bg-[#dbeafe] dark:bg-[#1e3a8a] border border-[#2563eb]'
              : 'text-[#374151] dark:text-[#cccccc] hover:bg-[#f3f4f6] dark:hover:bg-[#252528] hover:text-[#111827] dark:hover:text-white'
          }`}
        >
          {isFolder ? (
            <span className="w-3.5 h-3.5 flex items-center justify-center text-[#9ca3af] group-hover:text-[#111827] dark:group-hover:text-white shrink-0">
              {isExpanded ? <ChevronDown className="w-3 h-3" /> : <ChevronRight className="w-3 h-3" />}
            </span>
          ) : (
            <span className="w-3.5 h-3.5 shrink-0" />
          )}

          {getFileIcon(item)}

          {isRenaming ? (
            <input
              type="text"
              autoFocus
              value={renameNewName}
              onChange={(e) => setRenameNewName(e.target.value)}
              onKeyDown={async (e) => {
                if (e.key === 'Enter' && renameNewName.trim()) {
                  const parentDir = item.path.substring(0, item.path.lastIndexOf('/'));
                  const newPath = parentDir ? `${parentDir}/${renameNewName.trim()}` : renameNewName.trim();
                  await renameFile(item.path, newPath);
                  setRenamingItemId(null);
                  refreshFiles();
                } else if (e.key === 'Escape') {
                  setRenamingItemId(null);
                }
              }}
              onBlur={() => setRenamingItemId(null)}
              className="px-1 py-0.2 bg-white dark:bg-[#1e1e1e] border border-[#2563eb] text-xs font-mono rounded focus:outline-none"
            />
          ) : (
            <span className={`truncate text-[12px] flex-1 ${isFrontend ? 'text-[#f97316] font-medium' : ''}`}>
              {item.name}
            </span>
          )}

          {/* Status Dot / Modified badge */}
          {isBuild && <span className="w-1.5 h-1.5 rounded-full bg-[#9ca3af] shrink-0" />}
          {isFrontend && <span className="w-1.5 h-1.5 rounded-full bg-[#f97316] shrink-0" />}
          {hasModified && <span className="text-[10px] font-mono text-[#d97706] font-semibold shrink-0">M</span>}
        </div>

        {isFolder && isExpanded && item.children && (
          <div>
            {item.children.map(child => renderItem(child, depth + 1))}
          </div>
        )}
      </div>
    );
  };

  // AI Commit message generator
  const handleAiCommit = async () => {
    setIsAiGenerating(true);
    try {
      const res = await ApiBridge.gitAiCommitMessage(activeWorkspacePath);
      setCommitMessage(res.message);
    } finally {
      setIsAiGenerating(false);
    }
  };

  return (
    <div 
      style={{ width: `${sidebarWidth}px`, minWidth: '180px', maxWidth: '600px' }}
      className="bg-white dark:bg-[#181818] text-[#334155] dark:text-[#cccccc] border-r border-[#e5e7eb] dark:border-[#2b2b2b] flex flex-col justify-between h-full select-none overflow-hidden font-sans relative"
    >
      
      {/* ── 1. EXPLORER ACTIVITY ── */}
      {activeActivity === 'explorer' && (
        <div className="flex-1 flex flex-col overflow-hidden">
          
          {/* Header */}
          <div className="h-[35px] min-h-[35px] px-3 flex items-center justify-between border-b border-[#e5e7eb] dark:border-[#282828] text-xs font-semibold text-[#111827] dark:text-white uppercase tracking-wider">
            <span>Explorer</span>
            <button
              type="button"
              className="p-1 rounded hover:bg-[#f3f4f6] dark:hover:bg-[#282828] text-[#6b7280] hover:text-[#111827] dark:hover:text-white transition-colors cursor-pointer"
              title="More Actions..."
            >
              <MoreHorizontal className="w-3.5 h-3.5" />
            </button>
          </div>

          {/* Root Workspace Row with Action Icons */}
          <div className="px-2 py-1.5 flex items-center justify-between text-xs font-bold uppercase text-[#111827] dark:text-[#e5e7eb] hover:bg-[#f3f4f6] dark:hover:bg-[#252528] cursor-pointer group transition-colors">
            <div onClick={() => toggleFolder('root')} className="flex items-center gap-1 min-w-0">
              {expandedFolders['root'] ? (
                <ChevronDown className="w-3.5 h-3.5 text-[#6b7280]" />
              ) : (
                <ChevronRight className="w-3.5 h-3.5 text-[#6b7280]" />
              )}
              <span className="truncate">{workspaceName}</span>
            </div>

            {/* Hover Action Icons */}
            <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
              <button 
                type="button"
                onClick={(e) => {
                  e.stopPropagation();
                  setIsCreatingFile(true);
                  setCreateParentPath(activeWorkspacePath);
                }}
                className="p-0.5 hover:bg-[#e5e7eb] dark:hover:bg-[#333333] rounded text-[#6b7280] hover:text-[#111827] dark:hover:text-white" 
                title="New File"
              >
                <FilePlus className="w-3.5 h-3.5" />
              </button>
              <button 
                type="button"
                onClick={(e) => {
                  e.stopPropagation();
                  setIsCreatingFolder(true);
                  setCreateParentPath(activeWorkspacePath);
                }}
                className="p-0.5 hover:bg-[#e5e7eb] dark:hover:bg-[#333333] rounded text-[#6b7280] hover:text-[#111827] dark:hover:text-white" 
                title="New Folder"
              >
                <FolderPlus className="w-3.5 h-3.5" />
              </button>
              <button 
                type="button"
                onClick={(e) => {
                  e.stopPropagation();
                  refreshFiles();
                }}
                className="p-0.5 hover:bg-[#e5e7eb] dark:hover:bg-[#333333] rounded text-[#6b7280] hover:text-[#111827] dark:hover:text-white" 
                title="Refresh Explorer"
              >
                <RefreshCw className="w-3.5 h-3.5" />
              </button>
              <button 
                type="button"
                onClick={(e) => {
                  e.stopPropagation();
                  collapseAllFolders();
                }}
                className="p-0.5 hover:bg-[#e5e7eb] dark:hover:bg-[#333333] rounded text-[#6b7280] hover:text-[#111827] dark:hover:text-white" 
                title="Collapse Folders in Explorer"
              >
                <Minimize2 className="w-3.5 h-3.5" />
              </button>
            </div>
          </div>

          {/* Inline File / Folder Creation input */}
          {(isCreatingFile || isCreatingFolder) && (
            <div className="px-3 py-1 bg-[#f8fafc] dark:bg-[#1e1e1e] border-y border-[#2563eb] flex items-center gap-1">
              {isCreatingFile ? (
                <FilePlus className="w-3.5 h-3.5 text-[#2563eb] shrink-0" />
              ) : (
                <FolderPlus className="w-3.5 h-3.5 text-[#2563eb] shrink-0" />
              )}
              <input
                type="text"
                autoFocus
                placeholder={isCreatingFile ? 'filename.ext' : 'folder-name'}
                value={newItemName}
                onChange={e => setNewItemName(e.target.value)}
                onKeyDown={async e => {
                  if (e.key === 'Enter' && newItemName.trim()) {
                    const full = createParentPath ? `${createParentPath}/${newItemName.trim()}` : newItemName.trim();
                    if (isCreatingFile) {
                      await createFile(full);
                      openFileInEditor(full);
                    } else {
                      await createFolder(full);
                    }
                    setIsCreatingFile(false);
                    setIsCreatingFolder(false);
                    setNewItemName('');
                    refreshFiles();
                  } else if (e.key === 'Escape') {
                    setIsCreatingFile(false);
                    setIsCreatingFolder(false);
                  }
                }}
                className="flex-1 bg-transparent text-xs font-mono text-[#111827] dark:text-white focus:outline-none p-0"
              />
            </div>
          )}

          {/* File Tree List */}
          {expandedFolders['root'] && (
            <div 
              onContextMenu={(e) => handleContextMenu(e, null)}
              className="flex-1 overflow-y-auto py-0.5"
            >
              {files.map(item => renderItem(item, 0))}
            </div>
          )}

          {/* Bottom Accordions: Outline & Timeline */}
          <div className="border-t border-[#e5e7eb] dark:border-[#282828] bg-white dark:bg-[#181818] text-xs">
            <div 
              onClick={() => setIsOutlineOpen(prev => !prev)} 
              className="px-3 py-1 flex items-center gap-1 text-[11px] font-semibold text-[#6b7280] dark:text-[#9ca3af] uppercase tracking-wider hover:bg-[#f3f4f6] dark:hover:bg-[#252528] cursor-pointer transition-colors"
            >
              {isOutlineOpen ? <ChevronDown className="w-3 h-3" /> : <ChevronRight className="w-3 h-3" />}
              <span>Outline</span>
            </div>
            {isOutlineOpen && (
              <div className="px-5 py-1.5 text-[11px] text-[#6b7280] dark:text-[#9ca3af] space-y-0.5 bg-[#f9fafb] dark:bg-[#1e1e1e]">
                <p className="hover:text-[#2563eb] cursor-pointer font-mono text-[10px]">func init()</p>
                <p className="hover:text-[#2563eb] cursor-pointer font-mono text-[10px]">func sceneSineWave()</p>
              </div>
            )}

            <div 
              onClick={() => setIsTimelineOpen(prev => !prev)} 
              className="px-3 py-1 flex items-center gap-1 text-[11px] font-semibold text-[#6b7280] dark:text-[#9ca3af] uppercase tracking-wider hover:bg-[#f3f4f6] dark:hover:bg-[#252528] cursor-pointer border-t border-[#e5e7eb] dark:border-[#282828] transition-colors"
            >
              {isTimelineOpen ? <ChevronDown className="w-3 h-3" /> : <ChevronRight className="w-3 h-3" />}
              <span>Timeline</span>
            </div>
          </div>

        </div>
      )}

      {/* ── 2. SEARCH ACTIVITY (CLONE SCREENSHOT 1) ── */}
      {activeActivity === 'search' && (
        <div className="flex-1 flex flex-col overflow-hidden">
          {/* Header */}
          <div className="h-[35px] min-h-[35px] px-3 flex items-center justify-between border-b border-[#e5e7eb] dark:border-[#282828] text-xs font-semibold text-[#111827] dark:text-white uppercase tracking-wider">
            <span>Search</span>
            <div className="flex items-center gap-1 text-[#6b7280] dark:text-[#9ca3af]">
              <button 
                type="button" 
                onClick={() => runSearch(searchQuery)}
                className="p-1 rounded hover:bg-[#f3f4f6] dark:hover:bg-[#282828] cursor-pointer" 
                title="Refresh Search (⌘R)"
              >
                <RefreshCw className={`w-3.5 h-3.5 ${isSearching ? 'animate-spin' : ''}`} />
              </button>
              <button 
                type="button" 
                onClick={() => { setSearchQuery(''); setSearchResults([]); }} 
                className="p-1 rounded hover:bg-[#f3f4f6] dark:hover:bg-[#282828] cursor-pointer" 
                title="Clear Search Results"
              >
                <Ban className="w-3.5 h-3.5" />
              </button>
              <button 
                type="button" 
                onClick={() => setSearchViewMode(prev => prev === 'tree' ? 'list' : 'tree')}
                className="p-1 rounded hover:bg-[#f3f4f6] dark:hover:bg-[#282828] cursor-pointer" 
                title="View as Tree / List"
              >
                {searchViewMode === 'tree' ? <FolderTree className="w-3.5 h-3.5" /> : <List className="w-3.5 h-3.5" />}
              </button>
              <button 
                type="button" 
                onClick={() => {
                  const allOpen = Object.keys(expandedSearchFiles).every(k => expandedSearchFiles[k]);
                  const updated: Record<string, boolean> = {};
                  searchResults.forEach(g => { updated[g.file] = !allOpen; });
                  setExpandedSearchFiles(updated);
                }} 
                className="p-1 rounded hover:bg-[#f3f4f6] dark:hover:bg-[#282828] cursor-pointer" 
                title="Toggle Collapse/Expand All"
              >
                <ChevronsDownUp className="w-3.5 h-3.5" />
              </button>
            </div>
          </div>

          {/* Search & Replace Form Box */}
          <div className="p-2.5 space-y-1.5 border-b border-[#e5e7eb] dark:border-[#282828]">
            {/* Search Input Row */}
            <div className="flex items-center gap-1">
              <button
                type="button"
                onClick={() => setShowReplace(prev => !prev)}
                className="p-0.5 text-[#6b7280] hover:text-[#111827] dark:hover:text-white cursor-pointer"
              >
                <ChevronDown className={`w-3.5 h-3.5 transition-transform ${showReplace ? '' : '-rotate-90'}`} />
              </button>
              
              <div className="relative flex-1">
                <input
                  type="text"
                  placeholder="Search"
                  value={searchQuery}
                  onChange={e => setSearchQuery(e.target.value)}
                  onKeyDown={e => { if (e.key === 'Enter') runSearch(searchQuery); }}
                  className="w-full pl-2 pr-16 py-1 bg-white dark:bg-[#1e1e1e] border border-[#d1d5db] dark:border-[#383838] rounded text-xs font-mono text-[#111827] dark:text-white focus:outline-none focus:border-[#2563eb]"
                />
                <div className="absolute right-1.5 top-1 flex items-center gap-1">
                  <button
                    type="button"
                    onClick={() => setMatchCase(prev => !prev)}
                    className={`px-1 py-0.2 rounded text-[10px] font-mono font-bold cursor-pointer ${
                      matchCase ? 'bg-[#2563eb] text-white' : 'text-[#6b7280] hover:bg-[#f3f4f6] dark:hover:bg-[#333]'
                    }`}
                    title="Match Case (⌥⌘C)"
                  >
                    Aa
                  </button>
                  <button
                    type="button"
                    onClick={() => setMatchWholeWord(prev => !prev)}
                    className={`px-1 py-0.2 rounded text-[10px] font-mono font-bold cursor-pointer ${
                      matchWholeWord ? 'bg-[#2563eb] text-white' : 'text-[#6b7280] hover:bg-[#f3f4f6] dark:hover:bg-[#333]'
                    }`}
                    title="Match Whole Word (⌥⌘W)"
                  >
                    ab
                  </button>
                  <button
                    type="button"
                    onClick={() => setUseRegex(prev => !prev)}
                    className={`px-1 py-0.2 rounded text-[10px] font-mono font-bold cursor-pointer ${
                      useRegex ? 'bg-[#2563eb] text-white' : 'text-[#6b7280] hover:bg-[#f3f4f6] dark:hover:bg-[#333]'
                    }`}
                    title="Use Regular Expression (⌥⌘R)"
                  >
                    .*
                  </button>
                </div>
              </div>
            </div>

            {/* Replace Input Row */}
            {showReplace && (
              <div className="flex items-center gap-1 pl-4.5">
                <div className="relative flex-1">
                  <input
                    type="text"
                    placeholder="Replace"
                    value={replaceQuery}
                    onChange={e => setReplaceQuery(e.target.value)}
                    className="w-full pl-2 pr-14 py-1 bg-white dark:bg-[#1e1e1e] border border-[#d1d5db] dark:border-[#383838] rounded text-xs font-mono text-[#111827] dark:text-white focus:outline-none focus:border-[#2563eb]"
                  />
                  <div className="absolute right-1.5 top-1 flex items-center gap-1">
                    <button
                      type="button"
                      onClick={() => setPreserveCase(prev => !prev)}
                      className={`px-1 py-0.2 rounded text-[10px] font-mono font-bold cursor-pointer ${
                        preserveCase ? 'bg-[#2563eb] text-white' : 'text-[#6b7280] hover:bg-[#f3f4f6] dark:hover:bg-[#333]'
                      }`}
                      title="Preserve Case (⌥⌘P)"
                    >
                      AB
                    </button>
                    <button
                      type="button"
                      onClick={handleReplaceAll}
                      className="px-1 py-0.2 text-[10px] font-mono text-[#6b7280] hover:bg-[#f3f4f6] dark:hover:bg-[#333] rounded cursor-pointer"
                      title="Replace All (⌥⌘↵)"
                    >
                      ab→ac
                    </button>
                  </div>
                </div>
              </div>
            )}
          </div>

          {/* Results Summary Bar */}
          <div className="px-3 py-1.5 text-[11px] text-[#6b7280] dark:text-[#9ca3af] flex items-center justify-between border-b border-[#f0f0f2] dark:border-[#262626]">
            <span>{totalMatchesCount} results in {searchResults.length} files</span>
            <button 
              type="button" 
              onClick={() => {
                if (searchResults.length > 0) {
                  openFileInEditor(searchResults[0].filePath, searchResults[0].matches[0]?.line);
                }
              }}
              className="text-[#2563eb] dark:text-[#60a5fa] hover:underline cursor-pointer"
            >
              Open in editor
            </button>
          </div>

          {/* Search Result Tree */}
          <div className="flex-1 overflow-y-auto py-1">
            {searchResults.map(group => {
              const isExp = expandedSearchFiles[group.file] ?? true;
              return (
                <div key={group.file} className="select-none text-xs">
                  {/* File Header */}
                  <div
                    onClick={() => setExpandedSearchFiles(prev => ({ ...prev, [group.file]: !isExp }))}
                    className="flex items-center justify-between px-2 py-1 hover:bg-[#f3f4f6] dark:hover:bg-[#252528] cursor-pointer text-[#111827] dark:text-white font-medium"
                  >
                    <div className="flex items-center gap-1 min-w-0">
                      {isExp ? <ChevronDown className="w-3 h-3 text-[#6b7280]" /> : <ChevronRight className="w-3 h-3 text-[#6b7280]" />}
                      <span className="text-[#00add8] font-bold text-[9px] font-mono">GO</span>
                      <span className="truncate">{group.file}</span>
                    </div>
                    <span className="px-1.5 py-0.2 rounded-full bg-[#2563eb] text-white text-[10px] font-bold">
                      {group.count}
                    </span>
                  </div>

                  {/* Matches List */}
                  {isExp && (
                    <div className="pl-6 space-y-0.5">
                      {group.matches.map(m => {
                        const isSelected = selectedSearchResult === m.id;
                        return (
                          <div
                            key={m.id}
                            onClick={() => {
                              setSelectedSearchResult(m.id);
                              openFileInEditor(group.filePath, m.line);
                            }}
                            className={`flex items-center justify-between px-2 py-0.5 text-[11px] font-mono cursor-pointer rounded-xs group ${
                              isSelected 
                                ? 'bg-[#eff6ff] dark:bg-[#1e293b] text-[#2563eb] dark:text-[#60a5fa] border border-[#2563eb]' 
                                : 'text-[#475569] dark:text-[#cbd5e1] hover:bg-[#f3f4f6] dark:hover:bg-[#282828]'
                            }`}
                          >
                            <span className="truncate flex-1">
                              {m.text}
                            </span>
                            <div className="opacity-0 group-hover:opacity-100 flex items-center gap-1 shrink-0 ml-1">
                              <button 
                                type="button" 
                                onClick={(e) => {
                                  e.stopPropagation();
                                  handleReplaceSingleMatch(group.filePath, m.id, m.line);
                                }}
                                className="p-0.5 hover:bg-[#dbeafe] rounded text-[#2563eb]" 
                                title="Replace This Match"
                              >
                                ab→
                              </button>
                              <button 
                                type="button" 
                                onClick={(e) => {
                                  e.stopPropagation();
                                  setSearchResults(prev => prev.map(g => g.file === group.file ? { ...g, matches: g.matches.filter(match => match.id !== m.id), count: g.matches.length - 1 } : g).filter(g => g.count > 0));
                                }}
                                className="p-0.5 hover:bg-[#dbeafe] rounded text-[#6b7280]" 
                                title="Dismiss"
                              >
                                <X className="w-3 h-3" />
                              </button>
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  )}
                </div>
              );
            })}
          </div>

        </div>
      )}

      {/* ── 3. GIT SOURCE CONTROL ACTIVITY (CLONE SCREENSHOT 2) ── */}
      {activeActivity === 'git' && (
        <div className="flex-1 flex flex-col overflow-hidden">
          
          {/* Header */}
          <div className="h-[35px] min-h-[35px] px-3 flex items-center justify-between border-b border-[#e5e7eb] dark:border-[#282828] text-xs font-semibold text-[#111827] dark:text-white uppercase tracking-wider">
            <span>Source Control</span>
            <div className="flex items-center gap-1">
              <button
                type="button"
                onClick={() => { refreshGitStatus(); refreshGitLog(); }}
                className="p-1 rounded hover:bg-[#f3f4f6] dark:hover:bg-[#282828] text-[#6b7280] hover:text-[#111827] dark:hover:text-white transition-colors cursor-pointer"
                title="Refresh Status & Commits"
              >
                <RefreshCw className="w-3.5 h-3.5" />
              </button>
              <button
                type="button"
                onClick={async () => {
                  let msg = commitMessage.trim();
                  if (!msg) {
                    setIsAiGenerating(true);
                    try {
                      const res = await ApiBridge.gitAiCommitMessage(activeWorkspacePath);
                      msg = res.message;
                      setCommitMessage(msg);
                    } catch {}
                    setIsAiGenerating(false);
                  }
                  if (!msg) return;
                  await ApiBridge.gitCommit(msg, activeWorkspacePath);
                  setCommitMessage('');
                  refreshGitStatus();
                  refreshGitLog();
                  refreshFiles();
                }}
                className="p-1 rounded hover:bg-[#f3f4f6] dark:hover:bg-[#282828] text-[#6b7280] hover:text-[#111827] dark:hover:text-white transition-colors cursor-pointer"
                title="Commit (⌘Enter)"
              >
                <Check className="w-3.5 h-3.5" />
              </button>
              <button
                type="button"
                onClick={async () => {
                  await ApiBridge.gitPush('main', activeWorkspacePath);
                  refreshGitStatus();
                  refreshGitLog();
                }}
                className="p-1 rounded hover:bg-[#f3f4f6] dark:hover:bg-[#282828] text-[#6b7280] hover:text-[#111827] dark:hover:text-white transition-colors cursor-pointer"
                title="Push to Origin"
              >
                <ArrowUp className="w-3.5 h-3.5" />
              </button>
              <div className="relative">
                <button
                  type="button"
                  onClick={() => setIsGitMenuOpen(prev => !prev)}
                  className="p-1 rounded hover:bg-[#f3f4f6] dark:hover:bg-[#282828] text-[#6b7280] hover:text-[#111827] dark:hover:text-white transition-colors cursor-pointer"
                  title="Views and More Actions..."
                >
                  <MoreHorizontal className="w-3.5 h-3.5" />
                </button>

                {/* Git Actions Dropdown Modal */}
                {isGitMenuOpen && (
                  <div className="absolute right-0 top-7 w-48 rounded-xl bg-white dark:bg-[#222224] border border-[#e5e7eb] dark:border-[#383838] shadow-2xl py-1 z-50 text-xs font-normal">
                    <button type="button" onClick={() => { ApiBridge.gitFetch(activeWorkspacePath); refreshGitStatus(); setIsGitMenuOpen(false); }} className="w-full px-3 py-1.5 text-left hover:bg-[#f3f4f6] dark:hover:bg-[#333] flex items-center gap-2">
                      <Download className="w-3.5 h-3.5" /> Fetch from All Remotes
                    </button>
                    <button type="button" onClick={async () => { await ApiBridge.gitPush('main', activeWorkspacePath); refreshGitStatus(); refreshGitLog(); setIsGitMenuOpen(false); }} className="w-full px-3 py-1.5 text-left hover:bg-[#f3f4f6] dark:hover:bg-[#333] flex items-center gap-2">
                      <ArrowUp className="w-3.5 h-3.5" /> Push to Origin
                    </button>
                    <button type="button" onClick={() => { refreshGitStatus(); refreshGitLog(); setIsGitMenuOpen(false); }} className="w-full px-3 py-1.5 text-left hover:bg-[#f3f4f6] dark:hover:bg-[#333] flex items-center gap-2">
                      <RefreshCw className="w-3.5 h-3.5" /> Refresh Status & Log
                    </button>
                  </div>
                )}
              </div>
            </div>
          </div>

          {/* Commit Box */}
          <div className="p-2.5 space-y-2 border-b border-[#e5e7eb] dark:border-[#282828]">
            <div className="relative">
              <textarea
                rows={2}
                placeholder="Message (⌘Enter to commit)"
                value={commitMessage}
                onChange={e => setCommitMessage(e.target.value)}
                onKeyDown={async e => {
                  if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
                    let msg = commitMessage.trim();
                    if (!msg) {
                      setIsAiGenerating(true);
                      try {
                        const res = await ApiBridge.gitAiCommitMessage(activeWorkspacePath);
                        msg = res.message;
                        setCommitMessage(msg);
                      } catch {}
                      setIsAiGenerating(false);
                    }
                    if (msg) {
                      await ApiBridge.gitCommit(msg, activeWorkspacePath);
                      setCommitMessage('');
                      refreshGitStatus();
                      refreshGitLog();
                      refreshFiles();
                    }
                  }
                }}
                className="w-full p-2 pr-20 bg-white dark:bg-[#1e1e1e] border border-[#d1d5db] dark:border-[#383838] rounded-md text-xs font-sans text-[#111827] dark:text-white placeholder-[#9ca3af] focus:outline-none focus:border-[#2563eb] resize-none"
              />
              <button
                type="button"
                disabled={isAiGenerating}
                onClick={handleAiCommit}
                className="absolute right-1.5 bottom-2 px-2 py-1 rounded bg-[#2563eb] hover:bg-[#1d4ed8] text-white text-[10px] font-medium flex items-center gap-1 shadow-2xs cursor-pointer disabled:opacity-50"
              >
                <Sparkles className="w-3 h-3" />
                <span>Generate</span>
              </button>
            </div>

            {/* Big Commit Button */}
            <button
              type="button"
              onClick={async () => {
                let msg = commitMessage.trim();
                if (!msg) {
                  setIsAiGenerating(true);
                  try {
                    const res = await ApiBridge.gitAiCommitMessage(activeWorkspacePath);
                    msg = res.message;
                    setCommitMessage(msg);
                  } catch {}
                  setIsAiGenerating(false);
                }
                if (!msg) return;
                await ApiBridge.gitCommit(msg, activeWorkspacePath);
                setCommitMessage('');
                refreshGitStatus();
                refreshGitLog();
                refreshFiles();
              }}
              className="w-full py-1.5 rounded bg-[#2563eb] hover:bg-[#1d4ed8] text-white font-medium text-xs flex items-center justify-center gap-1 shadow-2xs transition-colors cursor-pointer"
            >
              <Check className="w-3.5 h-3.5" />
              <span>Commit</span>
              <ChevronDown className="w-3.5 h-3.5 ml-1 opacity-70" />
            </button>

            {/* Review Working Changes banner */}
            <button
              type="button"
              onClick={() => {
                // Opens git changes review tab in editor
                openDiffInEditor({
                  id: 'diff-working',
                  filePath: 'Working Tree Changes',
                  fileName: `Git: Changes (${gitFiles.length} files)`,
                  originalContent: '',
                  modifiedContent: '',
                  additions: 0,
                  deletions: 0,
                  status: 'pending',
                  timestamp: new Date().toISOString()
                });
              }}
              className="w-full py-1 px-2 rounded bg-[#f8fafc] dark:bg-[#252528] hover:bg-[#f1f5f9] dark:hover:bg-[#2d2d30] text-[#2563eb] dark:text-[#60a5fa] text-[11px] font-medium flex items-center justify-between transition-colors cursor-pointer"
            >
              <span className="flex items-center gap-1">
                <Sparkles className="w-3 h-3" /> Review Working Changes
              </span>
              <ChevronDown className="w-3 h-3" />
            </button>
          </div>

          {/* Changes Accordion */}
          <div className="flex-1 overflow-y-auto">
            <div className="px-2 py-1 flex items-center justify-between text-xs font-semibold text-[#111827] dark:text-[#e5e7eb] hover:bg-[#f3f4f6] dark:hover:bg-[#252528] cursor-pointer group">
              <div onClick={() => setIsChangesExpanded(prev => !prev)} className="flex items-center gap-1">
                {isChangesExpanded ? <ChevronDown className="w-3 h-3" /> : <ChevronRight className="w-3 h-3" />}
                <span>Changes</span>
                <span className="px-1.5 py-0.2 rounded-full bg-[#2563eb] text-white text-[9px] font-bold">
                  {gitFiles.length}
                </span>
              </div>
              <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100">
                <button 
                  type="button" 
                  onClick={async () => {
                    await ApiBridge.gitStage('', activeWorkspacePath);
                    refreshGitStatus();
                  }} 
                  className="p-0.5 hover:bg-[#e5e7eb] rounded text-[#6b7280]" 
                  title="Stage All Changes"
                >
                  <Plus className="w-3 h-3" />
                </button>
                <button 
                  type="button" 
                  onClick={async () => {
                    await ApiBridge.gitDiscard('', activeWorkspacePath);
                    refreshGitStatus();
                  }} 
                  className="p-0.5 hover:bg-[#e5e7eb] rounded text-[#6b7280]" 
                  title="Discard All Changes"
                >
                  <Undo2 className="w-3 h-3" />
                </button>
              </div>
            </div>

            {/* Changed Files List matching Screenshot 2 */}
            {isChangesExpanded && (
              <div className="space-y-0.5">
                {gitFiles.length === 0 ? (
                  <div className="px-3 py-2 text-[11px] text-[#9ca3af] italic">
                    No changes detected (Working tree clean)
                  </div>
                ) : (
                  gitFiles.map((item, idx) => (
                    <div
                      key={idx}
                      onClick={() => {
                        openDiffInEditor({
                          id: `diff-${item.path}`,
                          filePath: item.path,
                          fileName: `${item.path.split('/').pop() || item.path} (Working Tree)`,
                          originalContent: '',
                          modifiedContent: '',
                          additions: 0,
                          deletions: 0,
                          status: 'pending',
                          timestamp: new Date().toISOString()
                        });
                      }}
                      className="flex items-center justify-between px-3 py-1 hover:bg-[#f3f4f6] dark:hover:bg-[#252528] text-xs cursor-pointer group"
                    >
                      <div className="flex items-center gap-1.5 min-w-0">
                        <span className="text-[#64748b]">⚙</span>
                        <span className="truncate text-[11.5px]">{item.path}</span>
                      </div>
                      <div className="flex items-center gap-1 shrink-0">
                        <div className="opacity-0 group-hover:opacity-100 flex items-center gap-1">
                          <button 
                            type="button" 
                            onClick={(e) => { e.stopPropagation(); openFileInEditor(item.path); }}
                            className="p-0.5 hover:bg-[#e5e7eb] rounded text-[#6b7280]"
                            title="Open File"
                          >
                            <ExternalLink className="w-3 h-3" />
                          </button>
                          <button 
                            type="button" 
                            onClick={async (e) => { 
                              e.stopPropagation(); 
                              await ApiBridge.gitDiscard(item.path, activeWorkspacePath); 
                              refreshGitStatus();
                            }}
                            className="p-0.5 hover:bg-[#e5e7eb] rounded text-[#6b7280]"
                            title="Discard Changes"
                          >
                            <Undo2 className="w-3 h-3" />
                          </button>
                          <button 
                            type="button" 
                            onClick={async (e) => { 
                              e.stopPropagation(); 
                              await ApiBridge.gitStage(item.path, activeWorkspacePath); 
                              refreshGitStatus();
                            }}
                            className="p-0.5 hover:bg-[#e5e7eb] rounded text-[#2563eb]"
                            title="Stage Changes"
                          >
                            <Plus className="w-3 h-3" />
                          </button>
                        </div>
                        <span className="font-mono text-[10px] text-[#d97706] font-semibold">{item.status}</span>
                      </div>
                    </div>
                  ))
                )}
              </div>
            )}

            {/* Git Graph Section */}
            <div className="mt-2 border-t border-[#e5e7eb] dark:border-[#282828]">
              <div 
                onClick={() => setIsGraphExpanded(prev => !prev)}
                className="px-2 py-1 flex items-center justify-between text-xs font-semibold text-[#111827] dark:text-[#e5e7eb] hover:bg-[#f3f4f6] dark:hover:bg-[#252528] cursor-pointer"
              >
                <div className="flex items-center gap-1">
                  {isGraphExpanded ? <ChevronDown className="w-3 h-3" /> : <ChevronRight className="w-3 h-3" />}
                  <span>Graph</span>
                </div>
                <div className="flex items-center gap-1.5 text-[10px] text-[#6b7280]">
                  <span>Auto</span>
                  <RefreshCw className="w-2.5 h-2.5" />
                </div>
              </div>

              {isGraphExpanded && (
                <div className="px-2 py-1 space-y-1">
                  {gitCommits.length === 0 ? (
                    <div className="px-2 py-2 text-[11px] text-[#9ca3af] italic">
                      No commits found in this repository.
                    </div>
                  ) : (
                    gitCommits.map((c, idx) => (
                      <div 
                        key={idx} 
                        onClick={() => {
                          setSelectedCommitHash(c.hash);
                          openDiffInEditor({
                            id: `commit-${c.hash}`,
                            filePath: c.message,
                            fileName: `Commit: ${c.hash?.substring(0, 7)}`,
                            originalContent: '',
                            modifiedContent: '',
                            additions: 10,
                            deletions: 2,
                            status: 'pending',
                            timestamp: new Date().toISOString()
                          });
                        }}
                        className="flex items-start gap-1.5 text-xs group cursor-pointer hover:bg-[#f3f4f6] dark:hover:bg-[#252528] p-1 rounded"
                      >
                        <div className="w-2 h-2 rounded-full bg-[#2563eb] shrink-0 mt-1" />
                        <div className="min-w-0 flex-1">
                          <div className="flex items-center gap-1 flex-wrap">
                            <span className="font-medium text-[#111827] dark:text-white truncate">{c.message}</span>
                            {c.branch && (
                              <span className="px-1.5 py-0.2 rounded-full bg-[#2563eb]/15 text-[#2563eb] text-[9px] font-bold">
                                {c.branch}
                              </span>
                            )}
                          </div>
                          <div className="text-[10px] text-[#9ca3af] flex items-center gap-1.5 font-mono">
                            <span>{c.author}</span>
                            <span>·</span>
                            <span>{c.hash?.substring(0, 7)}</span>
                          </div>
                        </div>
                      </div>
                    ))
                  )}
                </div>
              )}
            </div>
          </div>

        </div>
      )}

      {/* Right-Click Context Menu Modal */}
      {contextMenu && (
        <div 
          style={{ top: `${contextMenu.y}px`, left: `${contextMenu.x}px` }}
          className="fixed z-50 w-52 rounded-xl bg-white dark:bg-[#222224] border border-[#e5e7eb] dark:border-[#383838] shadow-2xl py-1 text-xs select-none font-sans"
        >
          <button
            type="button"
            onClick={() => {
              setIsCreatingFile(true);
              setCreateParentPath(contextMenu.item?.type === 'folder' ? contextMenu.item.path : activeWorkspacePath);
              setContextMenu(null);
            }}
            className="w-full px-3 py-1.5 text-left hover:bg-[#f3f4f6] dark:hover:bg-[#333] flex items-center gap-2 cursor-pointer"
          >
            <FilePlus className="w-3.5 h-3.5 text-[#2563eb]" /> New File...
          </button>
          <button
            type="button"
            onClick={() => {
              setIsCreatingFolder(true);
              setCreateParentPath(contextMenu.item?.type === 'folder' ? contextMenu.item.path : activeWorkspacePath);
              setContextMenu(null);
            }}
            className="w-full px-3 py-1.5 text-left hover:bg-[#f3f4f6] dark:hover:bg-[#333] flex items-center gap-2 cursor-pointer"
          >
            <FolderPlus className="w-3.5 h-3.5 text-[#f97316]" /> New Folder...
          </button>
          
          <div className="my-1 border-t border-[#e5e7eb] dark:border-[#333]" />

          <button
            type="button"
            onClick={() => {
              if (contextMenu.item) ApiBridge.openInFinder(contextMenu.item.path);
              setContextMenu(null);
            }}
            className="w-full px-3 py-1.5 text-left hover:bg-[#f3f4f6] dark:hover:bg-[#333] flex items-center gap-2 cursor-pointer"
          >
            <ExternalLink className="w-3.5 h-3.5" /> Reveal in Finder
          </button>

          <button
            type="button"
            onClick={() => {
              if (contextMenu.item) {
                const targetDir = contextMenu.item.type === 'folder' ? contextMenu.item.path : contextMenu.item.path.substring(0, contextMenu.item.path.lastIndexOf('/'));
                ApiBridge.executeCommand(`cd "${targetDir}"`, activeWorkspacePath);
              }
              setContextMenu(null);
            }}
            className="w-full px-3 py-1.5 text-left hover:bg-[#f3f4f6] dark:hover:bg-[#333] flex items-center gap-2 cursor-pointer"
          >
            <Terminal className="w-3.5 h-3.5" /> Open in Integrated Terminal
          </button>

          <div className="my-1 border-t border-[#e5e7eb] dark:border-[#333]" />

          <button
            type="button"
            onClick={() => {
              if (contextMenu.item) setClipboardAction({ type: 'cut', path: contextMenu.item.path });
              setContextMenu(null);
            }}
            className="w-full px-3 py-1.5 text-left hover:bg-[#f3f4f6] dark:hover:bg-[#333] flex items-center gap-2 cursor-pointer"
          >
            <Scissors className="w-3.5 h-3.5" /> Cut <span className="ml-auto text-[10px] text-[#9ca3af]">⌘X</span>
          </button>

          <button
            type="button"
            onClick={() => {
              if (contextMenu.item) setClipboardAction({ type: 'copy', path: contextMenu.item.path });
              setContextMenu(null);
            }}
            className="w-full px-3 py-1.5 text-left hover:bg-[#f3f4f6] dark:hover:bg-[#333] flex items-center gap-2 cursor-pointer"
          >
            <Copy className="w-3.5 h-3.5" /> Copy <span className="ml-auto text-[10px] text-[#9ca3af]">⌘C</span>
          </button>

          {clipboardAction && (
            <button
              type="button"
              onClick={async () => {
                const targetDir = contextMenu.item?.type === 'folder' ? contextMenu.item.path : activeWorkspacePath;
                const fileName = clipboardAction.path.split('/').pop() || 'item';
                const dest = `${targetDir}/${fileName}`;
                if (clipboardAction.type === 'cut') {
                  await ApiBridge.moveFile(clipboardAction.path, dest);
                  setClipboardAction(null);
                } else {
                  const content = await ApiBridge.readFile(clipboardAction.path);
                  await ApiBridge.createFile(dest, content);
                }
                refreshFiles();
                setContextMenu(null);
              }}
              className="w-full px-3 py-1.5 text-left hover:bg-[#f3f4f6] dark:hover:bg-[#333] flex items-center gap-2 cursor-pointer"
            >
              <Clipboard className="w-3.5 h-3.5" /> Paste <span className="ml-auto text-[10px] text-[#9ca3af]">⌘V</span>
            </button>
          )}

          <div className="my-1 border-t border-[#e5e7eb] dark:border-[#333]" />

          <button
            type="button"
            onClick={() => {
              if (contextMenu.item) navigator.clipboard.writeText(contextMenu.item.path);
              setContextMenu(null);
            }}
            className="w-full px-3 py-1.5 text-left hover:bg-[#f3f4f6] dark:hover:bg-[#333] flex items-center gap-2 cursor-pointer"
          >
            <Copy className="w-3.5 h-3.5" /> Copy Path <span className="ml-auto text-[10px] text-[#9ca3af]">⌥⌘C</span>
          </button>

          <button
            type="button"
            onClick={() => {
              if (contextMenu.item) {
                const rel = contextMenu.item.path.replace(activeWorkspacePath, '').replace(/^\/+/, '');
                navigator.clipboard.writeText(rel);
              }
              setContextMenu(null);
            }}
            className="w-full px-3 py-1.5 text-left hover:bg-[#f3f4f6] dark:hover:bg-[#333] flex items-center gap-2 cursor-pointer"
          >
            <Copy className="w-3.5 h-3.5" /> Copy Relative Path <span className="ml-auto text-[10px] text-[#9ca3af]">⇧⌥⌘C</span>
          </button>

          <button
            type="button"
            onClick={() => {
              if (contextMenu.item) {
                setRenamingItemId(contextMenu.item.id);
                setRenameNewName(contextMenu.item.name);
              }
              setContextMenu(null);
            }}
            className="w-full px-3 py-1.5 text-left hover:bg-[#f3f4f6] dark:hover:bg-[#333] flex items-center gap-2 cursor-pointer"
          >
            <span>Rename...</span> <span className="ml-auto text-[10px] text-[#9ca3af]">Enter</span>
          </button>

          <button
            type="button"
            onClick={() => {
              if (contextMenu.item) deleteFile(contextMenu.item.path);
              setContextMenu(null);
            }}
            className="w-full px-3 py-1.5 text-left hover:bg-[#fee2e2] dark:hover:bg-[#450a0a] text-[#dc2626] flex items-center gap-2 cursor-pointer"
          >
            <Trash2 className="w-3.5 h-3.5" /> Delete <span className="ml-auto text-[10px] text-[#9ca3af]">⌘⌫</span>
          </button>
        </div>
      )}

      {/* Right Drag Resizer Handle */}
      <div
        onMouseDown={handleMouseDownResizer}
        className="absolute top-0 right-0 bottom-0 w-[4px] cursor-col-resize hover:bg-[#2563eb]/50 transition-colors z-20"
      />

    </div>
  );
};
