import React, { useState, useRef, useEffect, useCallback, useMemo } from 'react';
import { 
  X, 
  ChevronRight, 
  ChevronDown, 
  ChevronUp, 
  Columns2, 
  BookOpen, 
  MoreHorizontal, 
  Folder, 
  FileCode, 
  Code2
} from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { LSPService, LSPCompletionItem } from '../../services/lspService';
import { ImagePreview } from './ImagePreview';

interface CodeEditorPaneProps {
  tabId?: string;
  onSplitRight?: () => void;
  onTogglePreview?: () => void;
  isPreview?: boolean;
}

export const CodeEditorPane: React.FC<CodeEditorPaneProps> = ({
  tabId,
  onSplitRight,
  onTogglePreview,
  isPreview = false
}) => {
  const { 
    openTabs, 
    activeTabId, 
    closeTab, 
    selectedFile, 
    updateFileContent, 
    setIsSplitEditor, 
    activeWorkspacePath, 
    diagnostics
  } = useWorkspace();

  const [cursorLine, setCursorLine] = useState(1);
  const [cursorCol, setCursorCol] = useState(1);
  const [scrollTop, setScrollTop] = useState(0);
  const [scrollHeight, setScrollHeight] = useState(1);
  const [clientHeight, setClientHeight] = useState(1);

  // LSP Autocomplete State
  const [completions, setCompletions] = useState<LSPCompletionItem[]>([]);
  const [selectedCompletionIdx, setSelectedCompletionIdx] = useState(0);
  const [completionPos, setCompletionPos] = useState<{ x: number; y: number } | null>(null);
  const [activeWord, setActiveWord] = useState('');

  // Find & Replace State
  const [isFindOpen, setIsFindOpen] = useState(false);
  const [showReplace, setShowReplace] = useState(false);
  const [findQuery, setFindQuery] = useState('');
  const [replaceQuery, setReplaceQuery] = useState('');
  const [matchCase, setMatchCase] = useState(false);
  const [currentMatchIndex, setCurrentMatchIndex] = useState(0);

  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const gutterRef = useRef<HTMLDivElement>(null);
  const minimapRef = useRef<HTMLDivElement>(null);
  const findInputRef = useRef<HTMLInputElement>(null);

  const effectiveTabId = tabId || activeTabId;
  const activeTab = openTabs.find(t => t.id === effectiveTabId) || openTabs[0];
  const currentContent = activeTab?.content ?? selectedFile?.content ?? '';
  const currentFileName = activeTab?.fileName || selectedFile?.name || 'Readme.md';
  const workspaceName = activeWorkspacePath ? activeWorkspacePath.split('/').pop() || 'HasPHP' : 'HasPHP';
  const isImageFile = useMemo(() => /\.(png|jpg|jpeg|gif|webp|ico|icns|bmp|svg)$/i.test(currentFileName), [currentFileName]);

  // Synchronize Line Gutter Scroll with Textarea
  const handleScroll = (e: React.UIEvent<HTMLTextAreaElement>) => {
    const target = e.currentTarget;
    setScrollTop(target.scrollTop);
    setScrollHeight(target.scrollHeight);
    setClientHeight(target.clientHeight);
    if (gutterRef.current) {
      gutterRef.current.scrollTop = target.scrollTop;
    }
  };

  // Jump to specific line on tab activation / search match click
  useEffect(() => {
    if (activeTab?.line && textareaRef.current) {
      const lineNum = activeTab.line;
      const lines = currentContent.split('\n');
      let charPos = 0;
      for (let i = 0; i < Math.min(lineNum - 1, lines.length); i++) {
        charPos += lines[i].length + 1;
      }
      textareaRef.current.focus();
      textareaRef.current.setSelectionRange(charPos, charPos + (lines[lineNum - 1]?.length || 0));
      const targetScroll = Math.max(0, (lineNum - 5) * 20);
      textareaRef.current.scrollTop = targetScroll;
      setScrollTop(targetScroll);
      if (gutterRef.current) {
        gutterRef.current.scrollTop = targetScroll;
      }
      setCursorLine(lineNum);
    }
  }, [activeTab?.id, activeTab?.line, currentContent]);

  // Keyboard shortcuts (Cmd+F / Cmd+H)
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'f') {
        e.preventDefault();
        setIsFindOpen(true);
        setTimeout(() => findInputRef.current?.focus(), 50);
      } else if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'h') {
        e.preventDefault();
        setIsFindOpen(true);
        setShowReplace(true);
        setTimeout(() => findInputRef.current?.focus(), 50);
      } else if (e.key === 'Escape' && isFindOpen) {
        setIsFindOpen(false);
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [isFindOpen]);

  const matches = useMemo(() => {
    if (!findQuery) return [];
    const results: number[] = [];
    const contentToSearch = matchCase ? currentContent : currentContent.toLowerCase();
    const query = matchCase ? findQuery : findQuery.toLowerCase();
    let pos = 0;
    while ((pos = contentToSearch.indexOf(query, pos)) !== -1) {
      results.push(pos);
      pos += query.length;
    }
    return results;
  }, [findQuery, currentContent, matchCase]);

  const handleReplaceOne = () => {
    if (!findQuery || matches.length === 0 || !activeTab) return;
    const pos = matches[currentMatchIndex] || matches[0];
    const newText = currentContent.slice(0, pos) + replaceQuery + currentContent.slice(pos + findQuery.length);
    updateFileContent(activeTab.fileId, newText);
  };

  const handleReplaceAll = () => {
    if (!findQuery || !activeTab) return;
    const regex = new RegExp(findQuery.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), matchCase ? 'g' : 'gi');
    const newText = currentContent.replace(regex, replaceQuery);
    updateFileContent(activeTab.fileId, newText);
  };

  // Cursor and Autocomplete Tracker
  const updateCursorAndLsp = useCallback((textarea: HTMLTextAreaElement) => {
    const selStart = textarea.selectionStart;
    const textBefore = textarea.value.substring(0, selStart);
    const lineList = textBefore.split('\n');
    const line = lineList.length;
    const col = lineList[lineList.length - 1].length + 1;
    setCursorLine(line);
    setCursorCol(col);

    const match = textBefore.match(/([a-zA-Z0-9_$]+)$/);
    if (match && match[1].length >= 2) {
      const word = match[1];
      setActiveWord(word);
      const items = LSPService.getCompletions(word);
      if (items.length > 0) {
        setCompletions(items);
        setSelectedCompletionIdx(0);
        const top = Math.min(textarea.clientHeight - 120, (line - 1) * 22 + 30 - textarea.scrollTop);
        const left = Math.min(textarea.clientWidth - 200, col * 7.5 + 40);
        setCompletionPos({ x: left, y: top });
        return;
      }
    }
    setCompletions([]);
    setCompletionPos(null);
  }, []);

  const handleApplyCompletion = (item: LSPCompletionItem) => {
    if (!textareaRef.current || !activeTab) return;
    const textarea = textareaRef.current;
    const selStart = textarea.selectionStart;
    const textBefore = textarea.value.substring(0, selStart - activeWord.length);
    const textAfter = textarea.value.substring(selStart);
    const snippetText = item.insertText.replace(/\$\{\d+:?([^}]*)\}/g, '$1').replace(/\$0/g, '');
    const newContent = textBefore + snippetText + textAfter;

    updateFileContent(activeTab.fileId, newContent);
    setCompletions([]);
    setCompletionPos(null);

    setTimeout(() => {
      textarea.focus();
      const newPos = textBefore.length + snippetText.length;
      textarea.setSelectionRange(newPos, newPos);
    }, 10);
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (completions.length > 0) {
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        setSelectedCompletionIdx(prev => (prev + 1) % completions.length);
        return;
      } else if (e.key === 'ArrowUp') {
        e.preventDefault();
        setSelectedCompletionIdx(prev => (prev - 1 + completions.length) % completions.length);
        return;
      } else if (e.key === 'Enter' || e.key === 'Tab') {
        e.preventDefault();
        handleApplyCompletion(completions[selectedCompletionIdx]);
        return;
      } else if (e.key === 'Escape') {
        setCompletions([]);
        setCompletionPos(null);
        return;
      }
    }

    if (e.key === 'Tab') {
      e.preventDefault();
      const textarea = e.currentTarget;
      const start = textarea.selectionStart;
      const end = textarea.selectionEnd;
      const val = textarea.value;
      const newVal = val.substring(0, start) + '    ' + val.substring(end);
      if (activeTab) {
        updateFileContent(activeTab.fileId, newVal);
      }
      setTimeout(() => {
        textarea.selectionStart = textarea.selectionEnd = start + 4;
      }, 0);
    }
  };

  // File Icon Helper
  const getTabFileIcon = (fileName: string) => {
    if (fileName.endsWith('.md')) {
      return <span className="w-3.5 h-3.5 rounded bg-[#2563eb] text-white text-[8px] font-bold flex items-center justify-center shrink-0 font-mono shadow-2xs">M↓</span>;
    }
    if (fileName.endsWith('.php')) {
      return <span className="w-3.5 h-3.5 text-[#8b5cf6] font-bold text-[9px] flex items-center justify-center shrink-0 font-mono">php</span>;
    }
    if (fileName.endsWith('.json')) {
      return <span className="text-[#eab308] font-bold text-[10px] font-mono shrink-0">{'{}'}</span>;
    }
    return <FileCode className="w-3.5 h-3.5 text-[#3b82f6] shrink-0" />;
  };

  // Breadcrumbs symbol resolution
  const breadcrumbSymbol = useMemo(() => {
    if (currentFileName.endsWith('.md')) {
      const match = currentContent.match(/^#\s+(.*)$/m) || currentContent.match(/^##\s+(.*)$/m);
      return match ? `abc # ⚡ ${match[1].replace(/^[#\s]+/, '')}` : 'abc # ⚡ HasPHP Framework';
    }
    if (currentFileName.endsWith('.php')) {
      return currentFileName;
    }
    return currentFileName;
  }, [currentContent, currentFileName]);

  const lines = useMemo(() => currentContent.split('\n'), [currentContent]);
  const fileDiags = diagnostics.filter(d => d.filePath === activeTab?.filePath);

  // Minimap Viewport calculations
  const minimapViewportRatio = clientHeight / (scrollHeight || 1);
  const minimapTopRatio = scrollTop / (scrollHeight || 1);

  return (
    <div className="flex-1 flex flex-col h-full overflow-hidden bg-white dark:bg-[#181818] border-r border-[#e5e7eb] dark:border-[#282828] select-none font-sans">
      
      {/* Pane Tab Header Bar */}
      <div className="h-[35px] min-h-[35px] bg-[#f9fafb] dark:bg-[#181818] border-b border-[#e5e7eb] dark:border-[#282828] flex items-center justify-between px-2">
        {/* Left Tab Pill */}
        <div className="flex items-center gap-1.5 h-full">
          <div className="h-full px-3 bg-white dark:bg-[#1e1e1e] border-r border-[#e5e7eb] dark:border-[#282828] flex items-center gap-2 text-xs font-medium text-[#111827] dark:text-white cursor-pointer shadow-2xs">
            {getTabFileIcon(currentFileName)}
            <span>{currentFileName}</span>
            <button
              type="button"
              onClick={(e) => {
                e.stopPropagation();
                if (activeTab) closeTab(activeTab.id);
              }}
              className="p-0.5 rounded hover:bg-[#e5e7eb] dark:hover:bg-[#333333] text-[#9ca3af] hover:text-[#111827] dark:hover:text-white transition-colors cursor-pointer"
            >
              <X className="w-3 h-3" />
            </button>
          </div>
        </div>

        {/* Right Action Icons */}
        <div className="flex items-center gap-1 text-[#6b7280] dark:text-[#9ca3af]">
          {currentFileName.endsWith('.md') && (
            <button
              type="button"
              onClick={onTogglePreview}
              className={`p-1 rounded hover:bg-[#e5e7eb] dark:hover:bg-[#282828] transition-colors cursor-pointer ${
                isPreview ? 'text-[#2563eb] bg-[#eff6ff] dark:bg-[#1e293b]' : ''
              }`}
              title="Toggle Markdown Preview"
            >
              <BookOpen className="w-3.5 h-3.5" />
            </button>
          )}

          <button
            type="button"
            onClick={onSplitRight || (() => setIsSplitEditor(prev => !prev))}
            className="p-1 rounded hover:bg-[#e5e7eb] dark:hover:bg-[#282828] hover:text-[#111827] dark:hover:text-white transition-colors cursor-pointer"
            title="Split Editor Right"
          >
            <Columns2 className="w-3.5 h-3.5" />
          </button>

          <button
            type="button"
            className="p-1 rounded hover:bg-[#e5e7eb] dark:hover:bg-[#282828] hover:text-[#111827] dark:hover:text-white transition-colors cursor-pointer"
            title="More Actions..."
          >
            <MoreHorizontal className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      {/* Breadcrumbs Row */}
      <div className="h-[22px] min-h-[22px] bg-white dark:bg-[#181818] border-b border-[#f0f0f2] dark:border-[#262626] px-3 flex items-center gap-1.5 text-[11px] text-[#6b7280] dark:text-[#9ca3af] select-none font-sans overflow-x-auto">
        <span className="hover:text-[#111827] dark:hover:text-white cursor-pointer">{workspaceName}</span>
        <ChevronRight className="w-3 h-3 text-[#9ca3af]" />
        <div className="flex items-center gap-1 hover:text-[#111827] dark:hover:text-white cursor-pointer">
          {getTabFileIcon(currentFileName)}
          <span className="font-medium text-[#111827] dark:text-[#e2e8f0]">{currentFileName}</span>
        </div>
        {currentFileName.endsWith('.md') && (
          <>
            <ChevronRight className="w-3 h-3 text-[#9ca3af]" />
            <span className="text-[#6b7280] dark:text-[#9ca3af] truncate">{breadcrumbSymbol}</span>
          </>
        )}
      </div>

      {/* Find & Replace Floating Widget */}
      {isFindOpen && (
        <div className="absolute top-16 right-8 z-30 p-2.5 rounded-xl bg-white dark:bg-[#222224] border border-[#e5e7eb] dark:border-[#383838] shadow-2xl font-sans text-xs space-y-2 animate-in fade-in zoom-in-95 duration-100 min-w-[320px]">
          <div className="flex items-center gap-1.5">
            <button
              type="button"
              onClick={() => setShowReplace(prev => !prev)}
              className="p-1 rounded text-[#6b7280] hover:bg-[#f3f4f6] dark:hover:bg-[#333333] cursor-pointer"
              title="Toggle Replace"
            >
              <ChevronRight className={`w-3.5 h-3.5 transition-transform ${showReplace ? 'rotate-90' : ''}`} />
            </button>
            <div className="relative flex-1">
              <input
                ref={findInputRef}
                type="text"
                placeholder="Find"
                value={findQuery}
                onChange={e => {
                  setFindQuery(e.target.value);
                  setCurrentMatchIndex(0);
                }}
                className="w-full pl-2 pr-14 py-1 rounded-md bg-[#f8fafc] dark:bg-[#181818] border border-[#e2e8f0] dark:border-[#383838] text-xs font-mono text-[#0f172a] dark:text-white focus:outline-none focus:ring-1 focus:ring-[#2563eb]"
              />
              <span className="absolute right-2 top-1 text-[10px] text-[#9ca3af] font-mono select-none">
                {matches.length > 0 ? `${currentMatchIndex + 1}/${matches.length}` : findQuery ? 'No match' : ''}
              </span>
            </div>
            <button
              type="button"
              onClick={() => setMatchCase(prev => !prev)}
              className={`px-1.5 py-0.5 rounded font-mono text-[10px] font-bold cursor-pointer ${
                matchCase ? 'bg-[#2563eb] text-white' : 'text-[#6b7280] hover:bg-[#f3f4f6] dark:hover:bg-[#333333]'
              }`}
              title="Match Case"
            >
              Aa
            </button>
            <button
              type="button"
              disabled={matches.length === 0}
              onClick={() => setCurrentMatchIndex(prev => (prev > 0 ? prev - 1 : matches.length - 1))}
              className="p-1 rounded hover:bg-[#f3f4f6] dark:hover:bg-[#333333] disabled:opacity-30 cursor-pointer"
              title="Previous Match"
            >
              <ChevronUp className="w-3.5 h-3.5" />
            </button>
            <button
              type="button"
              disabled={matches.length === 0}
              onClick={() => setCurrentMatchIndex(prev => (prev < matches.length - 1 ? prev + 1 : 0))}
              className="p-1 rounded hover:bg-[#f3f4f6] dark:hover:bg-[#333333] disabled:opacity-30 cursor-pointer"
              title="Next Match"
            >
              <ChevronDown className="w-3.5 h-3.5" />
            </button>
            <button
              type="button"
              onClick={() => setIsFindOpen(false)}
              className="p-1 rounded hover:bg-[#f3f4f6] dark:hover:bg-[#333333] cursor-pointer"
              title="Close (Esc)"
            >
              <X className="w-3.5 h-3.5" />
            </button>
          </div>

          {showReplace && (
            <div className="flex items-center gap-1.5 pl-6">
              <input
                type="text"
                placeholder="Replace"
                value={replaceQuery}
                onChange={e => setReplaceQuery(e.target.value)}
                className="flex-1 px-2 py-1 rounded-md bg-[#f8fafc] dark:bg-[#181818] border border-[#e2e8f0] dark:border-[#383838] text-xs font-mono text-[#0f172a] dark:text-white focus:outline-none focus:ring-1 focus:ring-[#2563eb]"
              />
              <button
                type="button"
                onClick={handleReplaceOne}
                disabled={matches.length === 0}
                className="px-2 py-1 rounded bg-[#f3f4f6] dark:bg-[#333333] hover:bg-[#e5e7eb] text-[#111827] dark:text-white text-[11px] font-medium disabled:opacity-30 cursor-pointer"
              >
                Replace
              </button>
              <button
                type="button"
                onClick={handleReplaceAll}
                disabled={matches.length === 0}
                className="px-2 py-1 rounded bg-[#f3f4f6] dark:bg-[#333333] hover:bg-[#e5e7eb] text-[#111827] dark:text-white text-[11px] font-medium disabled:opacity-30 cursor-pointer"
              >
                All
              </button>
            </div>
          )}
        </div>
      )}

      {/* If file is an image (PNG, JPG, SVG, ICO, ICNS, WEBP, GIF, BMP) */}
      {isImageFile ? (
        <ImagePreview 
          filePath={activeTab?.filePath || selectedFile?.path || ''} 
          fileName={currentFileName} 
          rawContent={currentContent} 
        />
      ) : (
        /* Editor Main Surface: Gutter + Code Editor + Minimap */
        <div className="flex-1 flex overflow-hidden relative bg-white dark:bg-[#181818]">
          
          {/* Line Numbers Gutter */}
          <div 
            ref={gutterRef}
            className="w-[45px] min-w-[45px] bg-white dark:bg-[#181818] text-[#9ca3af] dark:text-[#555555] text-right pr-3 py-2.5 select-none text-[12px] leading-[21px] font-mono overflow-hidden"
          >
            {lines.map((_, i) => {
              const lineNum = i + 1;
              const isCurrent = lineNum === cursorLine;
              const hasError = fileDiags.some(d => d.line === lineNum && d.severity === 'error');
              const hasWarning = fileDiags.some(d => d.line === lineNum && d.severity === 'warning');

              return (
                <div 
                  key={i} 
                  className={`h-[21px] flex items-center justify-end gap-1 transition-colors ${
                    isCurrent ? 'text-[#111827] dark:text-white font-semibold' : ''
                  }`}
                >
                  {hasError ? (
                    <span className="w-1.5 h-1.5 rounded-full bg-[#ef4444]" />
                  ) : hasWarning ? (
                    <span className="w-1.5 h-1.5 rounded-full bg-[#f59e0b]" />
                  ) : null}
                  <span>{lineNum}</span>
                </div>
              );
            })}
          </div>

          {/* Textarea Code Container */}
          <div className="flex-1 relative h-full overflow-hidden">
            <textarea
              ref={textareaRef}
              value={currentContent}
              onChange={(e) => {
                if (activeTab) updateFileContent(activeTab.fileId, e.target.value);
                updateCursorAndLsp(e.target);
              }}
              onScroll={handleScroll}
              onSelect={(e) => updateCursorAndLsp(e.currentTarget)}
              onKeyUp={(e) => updateCursorAndLsp(e.currentTarget)}
              onClick={(e) => updateCursorAndLsp(e.currentTarget)}
              onKeyDown={handleKeyDown}
              spellCheck={false}
              className="w-full h-full p-2.5 bg-transparent text-[#111827] dark:text-[#e2e8f0] caret-[#2563eb] dark:caret-[#38bdf8] font-mono text-[12px] leading-[21px] resize-none focus:outline-none selection:bg-[#bfdbfe]/70 dark:selection:bg-[#264f78]/70 overflow-y-auto overflow-x-auto whitespace-pre tab-4"
            />

            {/* Autocomplete Popup (LSP) */}
            {completionPos && completions.length > 0 && (
              <div 
                style={{ left: `${completionPos.x}px`, top: `${completionPos.y}px` }}
                className="absolute z-40 w-64 rounded-xl bg-white dark:bg-[#222224] border border-[#e5e7eb] dark:border-[#383838] shadow-2xl overflow-hidden animate-in fade-in zoom-in-95 duration-100 font-sans"
              >
                <div className="px-2.5 py-1 bg-[#f8fafc] dark:bg-[#1f1f20] border-b border-[#e2e8f0] dark:border-[#333333] flex items-center justify-between text-[10px] text-[#6b7280] dark:text-[#9ca3af] select-none">
                  <span className="font-semibold uppercase tracking-wider">Suggestions</span>
                  <span>Tab / Enter</span>
                </div>
                <div className="max-h-48 overflow-y-auto p-1 space-y-0.5">
                  {completions.map((item, idx) => {
                    const isSelected = idx === selectedCompletionIdx;
                    return (
                      <div
                        key={idx}
                        onMouseDown={(e) => {
                          e.preventDefault();
                          handleApplyCompletion(item);
                        }}
                        className={`px-2 py-1.5 rounded-lg flex items-center justify-between text-xs cursor-pointer transition-colors ${
                          isSelected 
                            ? 'bg-[#2563eb] text-white' 
                            : 'text-[#374151] dark:text-[#d1d5db] hover:bg-[#f1f5f9] dark:hover:bg-[#333333]'
                        }`}
                      >
                        <div className="flex items-center gap-1.5 min-w-0">
                          <Code2 className="w-3.5 h-3.5 opacity-80 shrink-0" />
                          <span className="font-mono font-semibold truncate">{item.label}</span>
                        </div>
                        <span className={`text-[10px] opacity-70 truncate max-w-[90px] font-mono ${isSelected ? 'text-white' : 'text-[#6b7280]'}`}>
                          {item.kind}
                        </span>
                      </div>
                    );
                  })}
                </div>
              </div>
            )}
          </div>

          {/* Minimap (Right side) */}
          <div 
            ref={minimapRef}
            onClick={(e) => {
              if (!textareaRef.current || !minimapRef.current) return;
              const rect = minimapRef.current.getBoundingClientRect();
              const clickY = e.clientY - rect.top;
              const targetRatio = clickY / rect.height;
              textareaRef.current.scrollTop = targetRatio * textareaRef.current.scrollHeight;
            }}
            className="w-[60px] min-w-[60px] h-full bg-[#fafafa] dark:bg-[#161616] border-l border-[#f0f0f2] dark:border-[#262626] overflow-hidden select-none relative cursor-pointer hidden md:block"
            title="Minimap"
          >
            {/* Visual Mini Line Blocks */}
            <div className="p-1 space-y-[2px] opacity-70 pointer-events-none scale-90 origin-top-left">
              {lines.slice(0, 100).map((l, i) => {
                const trimmed = l.trim();
                if (!trimmed) return <div key={i} className="h-[2px]" />;
                const indent = l.search(/\S/) >= 0 ? l.search(/\S/) : 0;
                const width = Math.min(100, Math.max(15, trimmed.length * 2.5));
                const isComment = trimmed.startsWith('//') || trimmed.startsWith('/*') || trimmed.startsWith('*');
                const isHeader = trimmed.startsWith('#');
                return (
                  <div 
                    key={i} 
                    style={{ marginLeft: `${indent * 2}px`, width: `${width}%` }}
                    className={`h-[2px] rounded-xs ${
                      isHeader 
                        ? 'bg-[#2563eb] dark:bg-[#60a5fa]' 
                        : isComment 
                        ? 'bg-[#94a3b8] dark:bg-[#555]' 
                        : 'bg-[#64748b] dark:bg-[#777]'
                    }`}
                  />
                );
              })}
            </div>

            {/* Viewport Overlay Box */}
            <div 
              style={{ 
                top: `${minimapTopRatio * 100}%`, 
                height: `${Math.max(15, minimapViewportRatio * 100)}%` 
              }}
              className="absolute left-0 right-0 bg-[#2563eb]/10 dark:bg-white/10 border-y border-[#2563eb]/30 dark:border-white/20 transition-all pointer-events-none"
            />
          </div>

        </div>
      )}

    </div>
  );
};
