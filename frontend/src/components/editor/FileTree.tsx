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
  Network, 
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
  Server,
  Unplug,
  FileCode
} from 'lucide-react';
import { FileItem } from '../../types';
import { parseGitDecorations } from '../../lib/gitDecorations';
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
    moveFile,
    copyFile,
    revalidateFileMutations,
    refreshFiles, 
    activeWorkspacePath, 
    activeActivity, 
    gitBranch, 
    gitFiles, 
    gitCommits, 
    openGitGraphPane,
    refreshGitStatus, 
    refreshGitLog,
    openDiffInEditor,
    updateFolderChildren,
    openFolder,
    openTab,
    isSSHModalOpen,
    setIsSSHModalOpen,
    disconnectSSH,
    recentWorkspaces
  } = useWorkspace();

  const [expandedFolders, setExpandedFolders] = useState<Record<string, boolean>>({
    'root': true
  });

  const [isOutlineOpen, setIsOutlineOpen] = useState(false);
  const [isTimelineOpen, setIsTimelineOpen] = useState(false);

  // Search State
  const [searchQuery, setSearchQuery] = useState('');
  const [replaceQuery, setReplaceQuery] = useState('');
  const [showReplace, setShowReplace] = useState(true);
  const [matchCase, setMatchCase] = useState(false);
  const [matchWholeWord, setMatchWholeWord] = useState(false);
  const [useRegex, setUseRegex] = useState(false);
  const [preserveCase, setPreserveCase] = useState(false);
  const [searchViewMode, setSearchViewMode] = useState<'tree' | 'list'>('tree');
  const [expandedSearchFiles, setExpandedSearchFiles] = useState<Record<string, boolean>>({});
  const [selectedSearchResult, setSelectedSearchResult] = useState<string | null>(null);
  const [searchResults, setSearchResults] = useState<Array<{
    file: string;
    filePath: string;
    count: number;
    matches: Array<{ id: string; line: number; text: string; match: string }>;
  }>>([]);
  const [isSearching, setIsSearching] = useState(false);
  const [searchError, setSearchError] = useState<string | null>(null);
  const [replaceFeedback, setReplaceFeedback] = useState('');

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
  const [selectedTreeItem, setSelectedTreeItem] = useState<FileItem | null>(null);

  const handlePasteAction = async (targetItem: FileItem | null) => {
    if (!clipboardAction) return;
    const targetDir = targetItem?.type === 'folder'
      ? targetItem.path
      : (targetItem ? targetItem.path.substring(0, targetItem.path.lastIndexOf('/')) : activeWorkspacePath);
    const fileName = clipboardAction.path.split('/').pop() || 'item';
    const dest = `${targetDir}/${fileName}`;
    if (clipboardAction.type === 'cut') {
      await moveFile(clipboardAction.path, dest);
      setClipboardAction(null);
    } else {
      await copyFile(clipboardAction.path, dest);
    }
  };

  const handleDeleteAction = async (targetItem: FileItem | null) => {
    if (!targetItem) return;
    await deleteFile(targetItem.path);
    if (selectedTreeItem?.path === targetItem.path) {
      setSelectedTreeItem(null);
    }
  };

  // Drag & Drop State
  const [draggedItem, setDraggedItem] = useState<FileItem | null>(null);
  const [dragOverFolderId, setDragOverFolderId] = useState<string | null>(null);

  // Resizable Sidebar width state
  const [sidebarWidth, setSidebarWidth] = useState(260);
  const isResizingRef = useRef(false);

  const isSSHRemote = Boolean(activeWorkspacePath && activeWorkspacePath.startsWith('ssh://'));
  const workspaceName = isSSHRemote
    ? `[SSH] ${activeWorkspacePath.split('/').filter(Boolean).pop() || 'remote'}`
    : (activeWorkspacePath ? activeWorkspacePath.split('/').filter(Boolean).pop() || 'forge-ade' : 'forge-ade');

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

  // Close context menu on window click or Escape
  useEffect(() => {
    const handleClick = () => setContextMenu(null);
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setContextMenu(null);
    };
    window.addEventListener('click', handleClick);
    window.addEventListener('keydown', handleKey);
    return () => {
      window.removeEventListener('click', handleClick);
      window.removeEventListener('keydown', handleKey);
    };
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

  // Keyboard resizing (WCAG 2.1.1): arrows move the sidebar edge by 24px.
  const handleResizerKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'ArrowLeft') {
      e.preventDefault();
      setSidebarWidth(w => Math.max(180, w - 24));
    } else if (e.key === 'ArrowRight') {
      e.preventDefault();
      setSidebarWidth(w => Math.min(600, w + 24));
    }
  };

  // Real Search Execution — content matches from the Go backend (ripgrep with
  // a pure-Go fallback) plus filename matches for files with no content hits.
  // Backend failures surface as a visible error instead of a silent "0 results".
  const runSearch = useCallback(async (query: string) => {
    const q = query.trim();
    if (!q) {
      setSearchResults([]);
      setSearchError(null);
      return;
    }
    setIsSearching(true);
    setSearchError(null);
    try {
      const [contentHits, nameHits] = await Promise.all([
        ApiBridge.searchContent({
          query: q,
          caseSensitive: matchCase,
          wholeWord: matchWholeWord,
          isRegex: useRegex,
          maxResults: 1000
        }),
        ApiBridge.searchFilename(q, 200).catch(() => [] as string[])
      ]);

      const grouped: Record<string, Array<{ id: string; line: number; text: string; match: string }>> = {};
      for (const res of contentHits) {
        const filePath = res.path;
        if (!filePath) continue;
        if (!grouped[filePath]) grouped[filePath] = [];
        grouped[filePath].push({
          id: `${filePath}:${res.line}:${grouped[filePath].length}`,
          line: res.line,
          text: res.text,
          match: q
        });
      }
      // Filename-only matches (files that matched by name but had no content hits).
      for (const filePath of nameHits) {
        if (!filePath || grouped[filePath]) continue;
        const base = filePath.split('/').pop() || filePath;
        const hit = matchCase ? base.includes(q) : base.toLowerCase().includes(q.toLowerCase());
        if (hit) grouped[filePath] = [];
      }

      const treeResults = Object.entries(grouped).map(([filePath, matches]) => ({
        file: filePath.split('/').pop() || filePath,
        filePath,
        count: matches.length || 1,
        matches
      }));
      // Files with content hits first (most matches first), then by path.
      treeResults.sort((a, b) =>
        b.matches.length - a.matches.length || a.filePath.localeCompare(b.filePath)
      );
      setSearchResults(treeResults);
    } catch (err: any) {
      setSearchResults([]);
      setSearchError(
        typeof err?.message === 'string' && err.message
          ? `Search failed: ${err.message}`
          : 'Search failed — backend unavailable'
      );
    } finally {
      setIsSearching(false);
    }
  }, [matchCase, matchWholeWord, useRegex]);

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
    if (!searchQuery.trim()) return;
    try {
      const res = await ApiBridge.searchReplaceAll({
        query: searchQuery,
        replaceText: replaceQuery,
        caseSensitive: matchCase,
        wholeWord: matchWholeWord,
        isRegex: useRegex,
        preserveCase
      });
      setReplaceFeedback(
        res.totalReplacements > 0
          ? `${res.totalReplacements} replacement${res.totalReplacements === 1 ? '' : 's'} in ${res.filesChanged} file${res.filesChanged === 1 ? '' : 's'}`
          : 'No matches to replace'
      );
      await refreshFiles();
      runSearch(searchQuery);
    } catch (err: any) {
      setReplaceFeedback(
        typeof err?.message === 'string' && err.message ? `Replace failed: ${err.message}` : 'Replace failed'
      );
    }
  };

  // File Icon Renderer Matching Screenshot 3
  const getFileIcon = (file: FileItem) => {
    if (file.type === 'folder') {
      const isOpen = expandedFolders[file.id];
      const isFrontend = file.name === 'frontend';
      return isOpen ? (
        <FolderOpen className={`w-3.5 h-3.5 shrink-0 ${isFrontend ? 'text-full-access' : 'text-foreground-subtle'}`} />
      ) : (
        <Folder className={`w-3.5 h-3.5 shrink-0 ${isFrontend ? 'text-full-access' : 'text-foreground-subtle'}`} />
      );
    }

    const n = file.name.toLowerCase();
    if (n.endsWith('.go')) {
      return <span className="w-3.5 h-3.5 text-[#00add8] font-bold text-ui-xs flex items-center justify-center shrink-0 font-mono">GO</span>;
    }
    if (n.endsWith('.md')) {
      return <span className="w-3.5 h-3.5 rounded bg-primary text-white text-ui-xs font-bold flex items-center justify-center shrink-0 font-mono">M↓</span>;
    }
    if (n.endsWith('.php')) {
      return <span className="w-3.5 h-3.5 text-primary font-bold text-ui-xs flex items-center justify-center shrink-0 font-mono">php</span>;
    }
    if (n.endsWith('.ts') || n.endsWith('.tsx')) {
      return <span className="w-3.5 h-3.5 text-[#3178c6] font-bold text-ui-xs flex items-center justify-center shrink-0 font-mono">TS</span>;
    }
    if (n.endsWith('.js') || n.endsWith('.jsx')) {
      return <span className="w-3.5 h-3.5 text-warning font-bold text-ui-xs flex items-center justify-center shrink-0 font-mono">JS</span>;
    }
    if (n.endsWith('.json')) {
      return <span className="text-warning font-bold text-ui-xs font-mono shrink-0">{'{}'}</span>;
    }
    if (n.endsWith('.yml') || n.endsWith('.yaml')) {
      return <span className="text-primary font-bold text-ui-xs font-mono shrink-0">Y</span>;
    }
    if (n === 'makefile') {
      return <span className="text-foreground-subtlest text-ui-xs shrink-0">⚙</span>;
    }
    if (n === '.gitignore') {
      return <span className="text-full-access text-ui-xs font-bold shrink-0">⑂</span>;
    }
    if (/\.(png|jpg|jpeg|gif|svg|ico|icns)$/i.test(n)) {
      return <span className="text-primary text-ui-xs shrink-0">🎨</span>;
    }
    if (n.endsWith('.txt') || n.endsWith('.workspace')) {
      return <FileText className="w-3.5 h-3.5 text-foreground-subtle shrink-0" />;
    }
    return <FileText className="w-3.5 h-3.5 text-foreground-subtle shrink-0" />;
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
      await moveFile(draggedItem.path, newPath);
    }
    setDraggedItem(null);
  };

  // Context Menu Trigger
  const handleContextMenu = (e: React.MouseEvent, item: FileItem | null) => {
    e.preventDefault();
    e.stopPropagation();
    if (item) setSelectedTreeItem(item);
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
    const isSelected = selectedTreeItem ? selectedTreeItem.path === item.path : (selectedFile?.path === item.path);
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
          role="treeitem"
          aria-level={depth + 1}
          aria-expanded={isFolder ? isExpanded : undefined}
          aria-selected={isSelected}
          tabIndex={0}
          onClick={() => {
            setSelectedTreeItem(item);
            if (isFolder) {
              toggleFolder(item.id, item.path);
            } else {
              openFileInEditor(item.path);
            }
          }}
          onKeyDown={(e) => {
            if (e.key === 'Enter' || e.key === ' ') {
              e.preventDefault();
              if (isFolder) {
                toggleFolder(item.id, item.path);
              } else {
                openFileInEditor(item.path);
              }
            } else if (e.key === 'ArrowRight' && isFolder && !isExpanded) {
              e.preventDefault();
              toggleFolder(item.id, item.path);
            } else if (e.key === 'ArrowLeft' && isFolder && isExpanded) {
              e.preventDefault();
              toggleFolder(item.id, item.path);
            }
          }}
          style={{ paddingLeft: `${depth * 12 + 8}px` }}
          className={`flex items-center gap-1.5 py-0.5 pr-2 text-xs cursor-pointer transition-colors group relative outline-none focus-visible:outline focus-visible:outline-1 focus-visible:outline-[var(--accent-primary,#5b9dff)] ${
            isSelected
              ? 'bg-primary/10 dark:bg-card text-primary dark:text-info font-medium border border-[#3b82f6]/50 rounded-xs'
              : isDragOver
              ? 'bg-primary/10 dark:bg-[#1e3a8a] border border-primary'
              : 'text-foreground-subtle hover:bg-surface-hover dark:hover:bg-surface-hover hover:text-foreground'
          }`}
        >
          {isFolder ? (
            <span className="w-3.5 h-3.5 flex items-center justify-center text-foreground-subtle group-hover:text-foreground dark:group-hover:text-white shrink-0">
              {isExpanded ? <ChevronDown className="w-3 h-3" aria-hidden="true" /> : <ChevronRight className="w-3 h-3" aria-hidden="true" />}
            </span>
          ) : (
            <span className="w-3.5 h-3.5 shrink-0" />
          )}

          {getFileIcon(item)}

          {isRenaming ? (
            <input
              type="text"
              autoFocus
              aria-label={`Rename ${item.name}`}
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
              className="px-1 py-0.2 bg-card border border-primary text-xs font-mono rounded focus:outline-none"
            />
          ) : (
            <span className={`truncate text-ui-sm flex-1 ${isFrontend ? 'text-full-access font-medium' : ''}`}>
              {item.name}
            </span>
          )}

          {/* Status Dot / Modified badge */}
          {isBuild && <span className="w-1.5 h-1.5 rounded-full bg-[#9ca3af] shrink-0" aria-hidden="true" />}
          {isFrontend && <span className="w-1.5 h-1.5 rounded-full bg-[#f97316] shrink-0" aria-hidden="true" />}
          {hasModified && (
            <span className="text-ui-xs font-mono text-warning font-semibold shrink-0" aria-label="Modified">
              M
            </span>
          )}
        </div>

        {isFolder && isExpanded && item.children && (
          <div role="group">
            {item.children.map(child => renderItem(child, depth + 1))}
          </div>
        )}
      </div>
    );
  };

  return (
    <div 
      style={{ width: `${sidebarWidth}px`, minWidth: '180px', maxWidth: '600px' }}
      className="bg-background text-foreground-subtle border-r border-border flex flex-col justify-between h-full select-none overflow-hidden font-sans relative"
    >
      
      {/* ── 1. EXPLORER ACTIVITY ── */}
      {activeActivity === 'explorer' && (
        <div className="flex-1 flex flex-col overflow-hidden">
          
          {/* Header */}
          <div className="h-[35px] min-h-[35px] px-3 flex items-center justify-between border-b border-border text-xs font-semibold text-foreground uppercase tracking-wider">
            <span>Explorer</span>
            <button
              type="button"
              className="p-1 rounded hover:bg-surface-hover dark:hover:bg-surface-hover text-foreground-subtle hover:text-foreground transition-colors cursor-pointer"
              title="More Actions..."
            >
              <MoreHorizontal className="w-3.5 h-3.5" />
            </button>
          </div>

          {/* Root Workspace Row with Action Icons */}
          <div className="px-2 py-1.5 flex items-center justify-between text-xs font-bold uppercase text-foreground dark:text-[#e5e7eb] hover:bg-surface-hover dark:hover:bg-surface-hover cursor-pointer group transition-colors">
            <div onClick={() => toggleFolder('root')} className="flex items-center gap-1.5 min-w-0 flex-1">
              {expandedFolders['root'] ? (
                <ChevronDown className="w-3.5 h-3.5 text-foreground-subtle shrink-0" />
              ) : (
                <ChevronRight className="w-3.5 h-3.5 text-foreground-subtle shrink-0" />
              )}
              {isSSHRemote && <Server className="w-3.5 h-3.5 text-primary shrink-0" />}
              <span className="truncate">{workspaceName}</span>
            </div>

            {/* If SSH, show Disconnect button directly */}
            {isSSHRemote && (
              <button
                type="button"
                onClick={async (e) => {
                  e.stopPropagation();
                  await disconnectSSH();
                }}
                className="p-1 hover:bg-destructive/20 rounded text-destructive shrink-0 transition-colors mr-1"
                title="Disconnect SSH Remote (Close SSH)"
              >
                <Unplug className="w-3.5 h-3.5" />
              </button>
            )}

            {/* Hover Action Icons */}
            <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
              <button 
                type="button"
                onClick={(e) => {
                  e.stopPropagation();
                  setIsCreatingFile(true);
                  setCreateParentPath(activeWorkspacePath);
                }}
                className="p-0.5 hover:bg-surface-hover dark:hover:bg-surface-hover rounded text-foreground-subtle hover:text-foreground" 
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
                className="p-0.5 hover:bg-surface-hover dark:hover:bg-surface-hover rounded text-foreground-subtle hover:text-foreground" 
                title="New Folder"
              >
                <FolderPlus className="w-3.5 h-3.5" />
              </button>
              <button 
                type="button"
                onClick={(e) => {
                  e.stopPropagation();
                  revalidateFileMutations();
                }}
                className="p-0.5 hover:bg-surface-hover dark:hover:bg-surface-hover rounded text-foreground-subtle hover:text-foreground" 
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
                className="p-0.5 hover:bg-surface-hover dark:hover:bg-surface-hover rounded text-foreground-subtle hover:text-foreground" 
                title="Collapse Folders in Explorer"
              >
                <Minimize2 className="w-3.5 h-3.5" />
              </button>
            </div>
          </div>

          {/* Inline File / Folder Creation input */}
          {(isCreatingFile || isCreatingFolder) && (
            <div className="px-3 py-1 bg-background dark:bg-card border-y border-primary flex items-center gap-1">
              {isCreatingFile ? (
                <FilePlus className="w-3.5 h-3.5 text-primary shrink-0" />
              ) : (
                <FolderPlus className="w-3.5 h-3.5 text-primary shrink-0" />
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
                  } else if (e.key === 'Escape') {
                    setIsCreatingFile(false);
                    setIsCreatingFolder(false);
                  }
                }}
                className="flex-1 bg-transparent text-xs font-mono text-foreground focus:outline-none p-0"
              />
            </div>
          )}

          {/* File Tree List */}
          {expandedFolders['root'] && (
            <div
              role="tree"
              tabIndex={0}
              aria-label="Workspace files"
              onContextMenu={(e) => handleContextMenu(e, null)}
              onKeyDown={async (e) => {
                if ((e.target as HTMLElement).tagName === 'INPUT') return;

                if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'c' && !e.shiftKey && !e.altKey) {
                  if (selectedTreeItem) {
                    e.preventDefault();
                    setClipboardAction({ type: 'copy', path: selectedTreeItem.path });
                  }
                } else if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'x') {
                  if (selectedTreeItem) {
                    e.preventDefault();
                    setClipboardAction({ type: 'cut', path: selectedTreeItem.path });
                  }
                } else if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'v') {
                  if (clipboardAction) {
                    e.preventDefault();
                    await handlePasteAction(selectedTreeItem);
                  }
                } else if ((e.key === 'Backspace' && e.metaKey) || e.key === 'Delete') {
                  if (selectedTreeItem) {
                    e.preventDefault();
                    await handleDeleteAction(selectedTreeItem);
                  }
                } else if (e.key === 'F2') {
                  if (selectedTreeItem) {
                    e.preventDefault();
                    setRenamingItemId(selectedTreeItem.id);
                    setRenameNewName(selectedTreeItem.name);
                  }
                }
              }}
              className="flex-1 overflow-y-auto py-0.5 outline-none focus-visible:ring-1 focus-visible:ring-primary/40"
            >
              {files.length === 0 ? (
                <div className="p-4 space-y-3 text-center select-none">
                  <div className="py-2">
                    <p className="text-xs font-semibold text-foreground">No Folder Opened</p>
                    <p className="text-ui-xs text-foreground-subtle mt-0.5">
                      Standalone Code Editor Mode
                    </p>
                  </div>

                  <div className="space-y-1.5">
                    <button
                      type="button"
                      onClick={() => openFolder()}
                      className="w-full py-1.5 px-3 rounded-lg bg-primary text-white text-xs font-medium hover:bg-accent-hover transition-colors cursor-pointer shadow-xs"
                    >
                      Open Folder
                    </button>

                    <button
                      type="button"
                      onClick={() => setIsSSHModalOpen(true)}
                      className="w-full py-1.5 px-3 rounded-lg border border-border hover:bg-surface-hover dark:hover:bg-card text-foreground text-xs font-medium transition-colors cursor-pointer flex items-center justify-center gap-1.5"
                    >
                      <Server className="w-3.5 h-3.5 text-primary" />
                      Connect via SSH Remote
                    </button>

                    <button
                      type="button"
                      onClick={() => {
                        const scratchId = `scratch-${Date.now()}`;
                        openTab({
                          id: scratchId,
                          fileId: scratchId,
                          fileName: 'scratchpad.ts',
                          filePath: 'scratchpad.ts',
                          type: 'code',
                          content: '// Standalone Scratchpad\n// Edit freely without active project\n'
                        });
                      }}
                      className="w-full py-1.5 px-3 rounded-lg bg-surface-hover dark:bg-surface-hover text-foreground-subtle hover:bg-surface-hover dark:hover:bg-[#303034] text-xs font-medium transition-colors cursor-pointer"
                    >
                      New Scratch File
                    </button>
                  </div>

                  {recentWorkspaces.length > 0 && (
                    <div className="pt-2 text-left border-t border-border">
                      <span className="text-ui-xs font-semibold uppercase text-foreground-subtle tracking-wider">
                        Recent Projects
                      </span>
                      <div className="mt-1 space-y-0.5">
                        {recentWorkspaces.slice(0, 5).map(ws => {
                          const name = ws.split('/').filter(Boolean).pop() || ws;
                          return (
                            <button
                              key={ws}
                              type="button"
                              onClick={() => openFolder(ws)}
                              className="w-full text-left px-2 py-1 rounded hover:bg-surface-hover dark:hover:bg-surface-hover text-xs text-foreground-subtle hover:text-black dark:hover:text-white truncate cursor-pointer flex items-center gap-1.5"
                            >
                              <Folder className="w-3 h-3 text-warning shrink-0" />
                              <span className="truncate">{name}</span>
                            </button>
                          );
                        })}
                      </div>
                    </div>
                  )}
                </div>
              ) : (
                files.map(item => renderItem(item, 0))
              )}
            </div>
          )}

          {/* Bottom Accordions: Outline & Timeline */}
          <div className="border-t border-border bg-background text-xs">
            <button
              type="button"
              aria-expanded={isOutlineOpen}
              onClick={() => setIsOutlineOpen(prev => !prev)}
              className="w-full px-3 py-1 flex items-center gap-1 text-ui-xs font-semibold text-foreground-subtle uppercase tracking-wider hover:bg-surface-hover dark:hover:bg-surface-hover cursor-pointer transition-colors text-left"
            >
              {isOutlineOpen ? <ChevronDown className="w-3 h-3" aria-hidden="true" /> : <ChevronRight className="w-3 h-3" aria-hidden="true" />}
              <span>Outline</span>
            </button>
            {isOutlineOpen && (
              <div className="px-5 py-1.5 text-ui-xs text-foreground-subtle space-y-0.5 bg-surface dark:bg-card">
                <p className="hover:text-primary cursor-pointer font-mono text-ui-xs">func init()</p>
                <p className="hover:text-primary cursor-pointer font-mono text-ui-xs">func sceneSineWave()</p>
              </div>
            )}

            <button
              type="button"
              aria-expanded={isTimelineOpen}
              onClick={() => setIsTimelineOpen(prev => !prev)}
              className="w-full px-3 py-1 flex items-center gap-1 text-ui-xs font-semibold text-foreground-subtle uppercase tracking-wider hover:bg-surface-hover dark:hover:bg-surface-hover cursor-pointer border-t border-border transition-colors text-left"
            >
              {isTimelineOpen ? <ChevronDown className="w-3 h-3" aria-hidden="true" /> : <ChevronRight className="w-3 h-3" aria-hidden="true" />}
              <span>Timeline</span>
            </button>
          </div>

        </div>
      )}

      {/* ── 2. SEARCH ACTIVITY (CLONE SCREENSHOT 1) ── */}
      {activeActivity === 'search' && (
        <div className="flex-1 flex flex-col overflow-hidden">
          {/* Header */}
          <div className="h-[35px] min-h-[35px] px-3 flex items-center justify-between border-b border-border text-xs font-semibold text-foreground uppercase tracking-wider">
            <span>Search</span>
            <div className="flex items-center gap-1 text-foreground-subtle">
              <button
                type="button"
                onClick={() => runSearch(searchQuery)}
                aria-label="Refresh Search"
                className="p-1 rounded hover:bg-surface-hover dark:hover:bg-surface-hover cursor-pointer"
                title="Refresh Search (⌘R)"
              >
                <RefreshCw className={`w-3.5 h-3.5 ${isSearching ? 'animate-spin' : ''}`} aria-hidden="true" />
              </button>
              <button
                type="button"
                onClick={() => { setSearchQuery(''); setSearchResults([]); }}
                aria-label="Clear Search Results"
                className="p-1 rounded hover:bg-surface-hover dark:hover:bg-surface-hover cursor-pointer"
                title="Clear Search Results"
              >
                <Ban className="w-3.5 h-3.5" />
              </button>
              <button
                type="button"
                onClick={() => setSearchViewMode(prev => prev === 'tree' ? 'list' : 'tree')}
                aria-label="View as Tree / List"
                className="p-1 rounded hover:bg-surface-hover dark:hover:bg-surface-hover cursor-pointer"
                title="View as Tree / List"
              >
                {searchViewMode === 'tree' ? <FolderTree className="w-3.5 h-3.5" aria-hidden="true" /> : <List className="w-3.5 h-3.5" aria-hidden="true" />}
              </button>
              <button
                type="button"
                onClick={() => {
                  const allOpen = Object.keys(expandedSearchFiles).every(k => expandedSearchFiles[k]);
                  const updated: Record<string, boolean> = {};
                  searchResults.forEach(g => { updated[g.file] = !allOpen; });
                  setExpandedSearchFiles(updated);
                }}
                aria-label="Toggle Collapse/Expand All"
                className="p-1 rounded hover:bg-surface-hover dark:hover:bg-surface-hover cursor-pointer"
                title="Toggle Collapse/Expand All"
              >
                <ChevronsDownUp className="w-3.5 h-3.5" aria-hidden="true" />
              </button>
            </div>
          </div>

          {/* Search & Replace Form Box */}
          <div className="p-2.5 space-y-1.5 border-b border-border">
            {/* Search Input Row */}
            <div className="flex items-center gap-1">
              <button
                type="button"
                onClick={() => setShowReplace(prev => !prev)}
                className="p-0.5 text-foreground-subtle hover:text-foreground cursor-pointer"
              >
                <ChevronDown className={`w-3.5 h-3.5 transition-transform ${showReplace ? '' : '-rotate-90'}`} />
              </button>
              
              <div className="relative flex-1">
                <input
                  type="text"
                  placeholder="Search"
                  aria-label="Search"
                  value={searchQuery}
                  onChange={e => setSearchQuery(e.target.value)}
                  onKeyDown={e => { if (e.key === 'Enter') runSearch(searchQuery); }}
                  className="w-full pl-2 pr-16 py-1 bg-card border border-[#d1d5db] dark:border-border rounded text-xs font-mono text-foreground focus:outline-none focus:border-primary"
                />
                <div className="absolute right-1.5 top-1 flex items-center gap-1">
                  <button
                    type="button"
                    onClick={() => setMatchCase(prev => !prev)}
                    aria-pressed={matchCase}
                    className={`px-1 py-0.2 rounded text-ui-xs font-mono font-bold cursor-pointer ${
                      matchCase ? 'bg-primary text-white' : 'text-foreground-subtle hover:bg-surface-hover dark:hover:bg-card'
                    }`}
                    title="Match Case (⌥⌘C)"
                  >
                    Aa
                  </button>
                  <button
                    type="button"
                    onClick={() => setMatchWholeWord(prev => !prev)}
                    aria-pressed={matchWholeWord}
                    className={`px-1 py-0.2 rounded text-ui-xs font-mono font-bold cursor-pointer ${
                      matchWholeWord ? 'bg-primary text-white' : 'text-foreground-subtle hover:bg-surface-hover dark:hover:bg-card'
                    }`}
                    title="Match Whole Word (⌥⌘W)"
                  >
                    ab
                  </button>
                  <button
                    type="button"
                    onClick={() => setUseRegex(prev => !prev)}
                    aria-pressed={useRegex}
                    className={`px-1 py-0.2 rounded text-ui-xs font-mono font-bold cursor-pointer ${
                      useRegex ? 'bg-primary text-white' : 'text-foreground-subtle hover:bg-surface-hover dark:hover:bg-card'
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
                    aria-label="Replace"
                    value={replaceQuery}
                    onChange={e => setReplaceQuery(e.target.value)}
                    className="w-full pl-2 pr-14 py-1 bg-card border border-[#d1d5db] dark:border-border rounded text-xs font-mono text-foreground focus:outline-none focus:border-primary"
                  />
                  <div className="absolute right-1.5 top-1 flex items-center gap-1">
                    <button
                      type="button"
                      onClick={() => setPreserveCase(prev => !prev)}
                      aria-pressed={preserveCase}
                      className={`px-1 py-0.2 rounded text-ui-xs font-mono font-bold cursor-pointer ${
                        preserveCase ? 'bg-primary text-white' : 'text-foreground-subtle hover:bg-surface-hover dark:hover:bg-card'
                      }`}
                      title="Preserve Case (⌥⌘P)"
                    >
                      AB
                    </button>
                    <button
                      type="button"
                      onClick={handleReplaceAll}
                      className="px-1 py-0.2 text-ui-xs font-mono text-foreground-subtle hover:bg-surface-hover dark:hover:bg-card rounded cursor-pointer"
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
          <div className="px-3 py-1.5 text-ui-xs text-foreground-subtle flex items-center justify-between border-b border-border dark:border-surface">
            <span>
              {searchQuery.trim()
                ? `${totalMatchesCount} result${totalMatchesCount === 1 ? '' : 's'} in ${searchResults.length} file${searchResults.length === 1 ? '' : 's'}`
                : 'Type to search across the workspace'}
            </span>
            {searchResults.length > 0 && (
              <button
                type="button"
                onClick={() => {
                  if (searchResults.length > 0) {
                    openFileInEditor(searchResults[0].filePath, searchResults[0].matches[0]?.line);
                  }
                }}
                className="text-primary dark:text-info hover:underline cursor-pointer"
              >
                Open in editor
              </button>
            )}
          </div>

          {/* Replace feedback / search error */}
          {(replaceFeedback || searchError) && (
            <div
              className={`px-3 py-1 text-ui-xs border-b border-border dark:border-surface ${
                searchError ? 'text-destructive' : 'text-foreground-subtle'
              }`}
            >
              {searchError || replaceFeedback}
            </div>
          )}

          {/* Search Result Tree */}
          <div className="flex-1 overflow-y-auto py-1">
            {searchQuery.trim() && !isSearching && searchResults.length === 0 && !searchError && (
              <div className="px-3 py-2 text-ui-xs text-foreground-subtle italic">
                No results found.
              </div>
            )}
            {searchResults.map(group => {
              const isExp = expandedSearchFiles[group.file] ?? true;
              return (
                <div key={group.file} className="select-none text-xs">
                  {/* File Header */}
                  <div
                    onClick={() => setExpandedSearchFiles(prev => ({ ...prev, [group.file]: !isExp }))}
                    className="flex items-center justify-between px-2 py-1 hover:bg-surface-hover dark:hover:bg-surface-hover cursor-pointer text-foreground font-medium"
                  >
                    <div className="flex items-center gap-1 min-w-0">
                      {isExp ? <ChevronDown className="w-3 h-3 text-foreground-subtle" /> : <ChevronRight className="w-3 h-3 text-foreground-subtle" />}
                      <FileText className="w-3.5 h-3.5 text-foreground-subtle shrink-0" />
                      <span className="truncate" title={group.filePath}>{group.file}</span>
                    </div>
                    <span className="px-1.5 py-0.2 rounded-full bg-primary text-white text-ui-xs font-bold">
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
                            className={`flex items-center justify-between px-2 py-0.5 text-ui-xs font-mono cursor-pointer rounded-xs group ${
                              isSelected 
                                ? 'bg-primary/10 dark:bg-card text-primary dark:text-info border border-primary' 
                                : 'text-foreground-subtle dark:text-foreground-secondary hover:bg-surface-hover dark:hover:bg-surface-hover'
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
                                className="p-0.5 hover:bg-primary/10 rounded text-primary" 
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
                                className="p-0.5 hover:bg-primary/10 rounded text-foreground-subtle" 
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

      {/* Right-Click Context Menu Modal */}
      {contextMenu && (
        <div
          role="menu"
          aria-label="File actions"
          style={{ top: `${contextMenu.y}px`, left: `${contextMenu.x}px` }}
          className="fixed z-50 w-52 rounded-xl bg-popover border border-border shadow-2xl py-1 text-xs select-none font-sans"
        >
          <button
            type="button"
            onClick={() => {
              setIsCreatingFile(true);
              setCreateParentPath(contextMenu.item?.type === 'folder' ? contextMenu.item.path : activeWorkspacePath);
              setContextMenu(null);
            }}
            className="w-full px-3 py-1.5 text-left hover:bg-surface-hover dark:hover:bg-card flex items-center gap-2 cursor-pointer"
          >
            <FilePlus className="w-3.5 h-3.5 text-primary" /> New File...
          </button>
          <button
            type="button"
            onClick={() => {
              setIsCreatingFolder(true);
              setCreateParentPath(contextMenu.item?.type === 'folder' ? contextMenu.item.path : activeWorkspacePath);
              setContextMenu(null);
            }}
            className="w-full px-3 py-1.5 text-left hover:bg-surface-hover dark:hover:bg-card flex items-center gap-2 cursor-pointer"
          >
            <FolderPlus className="w-3.5 h-3.5 text-full-access" /> New Folder...
          </button>
          
          <div className="my-1 border-t border-border" />

          <button
            type="button"
            onClick={() => {
              if (contextMenu.item) ApiBridge.openInFinder(contextMenu.item.path);
              setContextMenu(null);
            }}
            className="w-full px-3 py-1.5 text-left hover:bg-surface-hover dark:hover:bg-card flex items-center gap-2 cursor-pointer"
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
            className="w-full px-3 py-1.5 text-left hover:bg-surface-hover dark:hover:bg-card flex items-center gap-2 cursor-pointer"
          >
            <Terminal className="w-3.5 h-3.5" /> Open in Integrated Terminal
          </button>

          <div className="my-1 border-t border-border" />

          <button
            type="button"
            onClick={() => {
              if (contextMenu.item) setClipboardAction({ type: 'cut', path: contextMenu.item.path });
              setContextMenu(null);
            }}
            className="w-full px-3 py-1.5 text-left hover:bg-surface-hover dark:hover:bg-card flex items-center gap-2 cursor-pointer"
          >
            <Scissors className="w-3.5 h-3.5" /> Cut <span className="ml-auto text-ui-xs text-foreground-subtle">⌘X</span>
          </button>

          <button
            type="button"
            onClick={() => {
              if (contextMenu.item) setClipboardAction({ type: 'copy', path: contextMenu.item.path });
              setContextMenu(null);
            }}
            className="w-full px-3 py-1.5 text-left hover:bg-surface-hover dark:hover:bg-card flex items-center gap-2 cursor-pointer"
          >
            <Copy className="w-3.5 h-3.5" /> Copy <span className="ml-auto text-ui-xs text-foreground-subtle">⌘C</span>
          </button>

          {clipboardAction && (
            <button
              type="button"
              onClick={async () => {
                await handlePasteAction(contextMenu.item);
                setContextMenu(null);
              }}
              className="w-full px-3 py-1.5 text-left hover:bg-surface-hover dark:hover:bg-card flex items-center gap-2 cursor-pointer"
            >
              <Clipboard className="w-3.5 h-3.5" /> Paste <span className="ml-auto text-ui-xs text-foreground-subtle">⌘V</span>
            </button>
          )}

          <div className="my-1 border-t border-border" />

          <button
            type="button"
            onClick={() => {
              if (contextMenu.item) navigator.clipboard.writeText(contextMenu.item.path);
              setContextMenu(null);
            }}
            className="w-full px-3 py-1.5 text-left hover:bg-surface-hover dark:hover:bg-card flex items-center gap-2 cursor-pointer"
          >
            <Copy className="w-3.5 h-3.5" /> Copy Path <span className="ml-auto text-ui-xs text-foreground-subtle">⌥⌘C</span>
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
            className="w-full px-3 py-1.5 text-left hover:bg-surface-hover dark:hover:bg-card flex items-center gap-2 cursor-pointer"
          >
            <Copy className="w-3.5 h-3.5" /> Copy Relative Path <span className="ml-auto text-ui-xs text-foreground-subtle">⇧⌥⌘C</span>
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
            className="w-full px-3 py-1.5 text-left hover:bg-surface-hover dark:hover:bg-card flex items-center gap-2 cursor-pointer"
          >
            <span>Rename...</span> <span className="ml-auto text-ui-xs text-foreground-subtle">Enter</span>
          </button>

          <button
            type="button"
            onClick={async () => {
              if (contextMenu.item) await handleDeleteAction(contextMenu.item);
              setContextMenu(null);
            }}
            className="w-full px-3 py-1.5 text-left hover:bg-destructive/10 dark:hover:bg-destructive/10 text-destructive flex items-center gap-2 cursor-pointer"
          >
            <Trash2 className="w-3.5 h-3.5" /> Delete <span className="ml-auto text-ui-xs text-foreground-subtle">⌘⌫</span>
          </button>
        </div>
      )}

      {/* Right Drag Resizer Handle */}
      <div
        role="separator"
        aria-orientation="vertical"
        aria-label="Resize sidebar"
        tabIndex={0}
        onKeyDown={handleResizerKeyDown}
        onMouseDown={handleMouseDownResizer}
        className="absolute top-0 right-0 bottom-0 w-[4px] cursor-col-resize hover:bg-primary/50 focus-visible:bg-primary/70 transition-colors z-20 outline-none focus-visible:outline focus-visible:outline-1 focus-visible:outline-[var(--accent-primary,#5b9dff)]"
      />

    </div>
  );
};
