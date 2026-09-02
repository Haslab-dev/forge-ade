import React, { useState, useRef, useEffect, useCallback } from 'react';
import { 
  X, 
  ChevronRight, 
  Copy, 
  Check, 
  SplitSquareVertical, 
  Sparkles, 
  FileCode, 
  CheckCircle2, 
  Folder, 
  Search, 
  Replace, 
  ChevronDown, 
  ChevronUp,
  Code2
} from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { LSPService, LSPHoverInfo, LSPCompletionItem } from '../../services/lspService';

export const CodeEditorPane: React.FC = () => {
  const { 
    openTabs, 
    activeTabId, 
    setActiveTabId, 
    closeTab, 
    selectedFile, 
    updateFileContent, 
    setIsSplitEditor, 
    activeWorkspacePath, 
    diagnostics, 
    createNewSession 
  } = useWorkspace();

  const [copied, setCopied] = useState(false);
  const [cursorLine, setCursorLine] = useState(1);
  const [cursorCol, setCursorCol] = useState(1);

  // LSP Autocomplete State
  const [completions, setCompletions] = useState<LSPCompletionItem[]>([]);
  const [selectedCompletionIdx, setSelectedCompletionIdx] = useState(0);
  const [completionPos, setCompletionPos] = useState<{ x: number; y: number } | null>(null);
  const [activeWord, setActiveWord] = useState('');

  // LSP Hover Tooltip State
  const [hoverInfo, setHoverInfo] = useState<LSPHoverInfo | null>(null);
  const [hoverCoords, setHoverCoords] = useState<{ x: number; y: number } | null>(null);

  // Find & Replace State
  const [isFindOpen, setIsFindOpen] = useState(false);
  const [showReplace, setShowReplace] = useState(false);
  const [findQuery, setFindQuery] = useState('');
  const [replaceQuery, setReplaceQuery] = useState('');
  const [matchCase, setMatchCase] = useState(false);
  const [currentMatchIndex, setCurrentMatchIndex] = useState(0);

  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const gutterRef = useRef<HTMLDivElement>(null);
  const findInputRef = useRef<HTMLInputElement>(null);

  const activeTab = openTabs.find(t => t.id === activeTabId && t.type === 'code') || openTabs.find(t => t.type === 'code') || openTabs[0];
  const currentContent = activeTab?.content ?? selectedFile?.content ?? '';

  // Synchronize Line Gutter Scroll with Textarea
  const handleScroll = (e: React.UIEvent<HTMLTextAreaElement>) => {
    if (gutterRef.current) {
      gutterRef.current.scrollTop = e.currentTarget.scrollTop;
    }
  };

  // Listen for Cmd+F / Cmd+H
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

  const matches = React.useMemo(() => {
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

    // Extract current word before cursor for Autocompletion
    const match = textBefore.match(/([a-zA-Z0-9_$]+)$/);
    if (match && match[1].length >= 2) {
      const word = match[1];
      setActiveWord(word);
      const items = LSPService.getCompletions(word);
      if (items.length > 0) {
        setCompletions(items);
        setSelectedCompletionIdx(0);
        // Approximate position
        const top = Math.min(textarea.clientHeight - 120, (line - 1) * 22 + 30 - textarea.scrollTop);
        const left = Math.min(textarea.clientWidth - 200, col * 7.5 + 40);
        setCompletionPos({ x: left, y: top });
        return;
      }
    }
    setCompletions([]);
    setCompletionPos(null);
  }, []);

  const handleTextChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    if (activeTab) {
      updateFileContent(activeTab.fileId, e.target.value);
    }
    updateCursorAndLsp(e.target);
  };

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
    // Autocomplete Navigation
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

    // Tab key inserts 2 spaces
    if (e.key === 'Tab') {
      e.preventDefault();
      const textarea = e.currentTarget;
      const start = textarea.selectionStart;
      const end = textarea.selectionEnd;
      const val = textarea.value;
      const next = val.substring(0, start) + '  ' + val.substring(end);
      if (activeTab) {
        updateFileContent(activeTab.fileId, next);
      }
      setTimeout(() => {
        textarea.selectionStart = textarea.selectionEnd = start + 2;
        updateCursorAndLsp(textarea);
      }, 0);
    }
  };

  const handleCopy = () => {
    navigator.clipboard.writeText(currentContent);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };

  const lines = currentContent.split('\n');
  const fileDiags = diagnostics.filter(d => activeTab?.filePath?.endsWith(d.filePath) || d.filePath?.endsWith(activeTab?.fileName || ''));

  return (
    <div className="flex-1 flex flex-col h-full bg-white dark:bg-[#1e1e1e] text-[#1e293b] dark:text-[#d4d4d4] font-mono text-[13px] overflow-hidden border-r border-[#e2e8f0] dark:border-[#2b2b2b] relative transition-colors duration-150">
      
      {/* Top Tab Bar */}
      <div className="h-[35px] min-h-[35px] bg-[#f8fafc] dark:bg-[#252526] flex items-center justify-between border-b border-[#e2e8f0] dark:border-[#1e1e1e] overflow-x-auto select-none">
        <div className="flex items-center h-full">
          {openTabs.filter(t => t.type === 'code').map(tab => {
            const isActive = tab.id === activeTab?.id;
            return (
              <div
                key={tab.id}
                onClick={() => setActiveTabId(tab.id)}
                className={`h-full flex items-center gap-2 px-3 text-xs border-r border-[#e2e8f0] dark:border-[#1e1e1e] cursor-pointer transition-colors ${
                  isActive
                    ? 'bg-white dark:bg-[#1e1e1e] text-[#0f172a] dark:text-white font-medium border-t-2 border-t-[#2563eb] dark:border-t-[#007acc]'
                    : 'text-[#64748b] dark:text-[#969696] hover:bg-[#e2e8f0] dark:hover:bg-[#2a2d2e] hover:text-[#0f172a] dark:hover:text-white'
                }`}
              >
                {tab.fileName.endsWith('.md') ? (
                  <span className="text-[9px] px-1 py-0.2 rounded bg-[#3b82f6] text-white font-bold">M↓</span>
                ) : tab.fileName.endsWith('.ts') || tab.fileName.endsWith('.tsx') ? (
                  <span className="text-[#3178c6] font-bold text-[10px] font-mono">TS</span>
                ) : tab.fileName.endsWith('.js') || tab.fileName.endsWith('.jsx') ? (
                  <span className="text-[#f7df1e] font-bold text-[10px] font-mono">JS</span>
                ) : tab.fileName.endsWith('.zig') ? (
                  <span className="text-[#f7a41d] font-bold text-[10px] font-mono">⚡</span>
                ) : (
                  <FileCode className="w-3.5 h-3.5 text-[#3b82f6]" />
                )}
                <span className="truncate max-w-[130px]">{tab.fileName}</span>
                <button
                  type="button"
                  onClick={(e) => {
                    e.stopPropagation();
                    closeTab(tab.id);
                  }}
                  className="p-0.5 hover:bg-[#e2e8f0] dark:hover:bg-[#333333] rounded text-[#64748b] dark:text-[#858585] hover:text-[#0f172a] dark:hover:text-white"
                >
                  <X className="w-3 h-3" />
                </button>
              </div>
            );
          })}
        </div>

        {/* Tab actions right */}
        <div className="flex items-center gap-1 px-2 text-[#64748b] dark:text-[#858585]">
          <button
            type="button"
            onClick={() => createNewSession(`Review and refactor ${activeTab?.fileName}`)}
            className="flex items-center gap-1 px-2 py-0.5 rounded bg-[#eff6ff] hover:bg-[#dbeafe] text-[#2563eb] dark:bg-[#252526] dark:hover:bg-[#333336] dark:text-[#38bdf8] text-xs transition-colors font-medium"
            title="Ask Agent to edit this file"
          >
            <Sparkles className="w-3 h-3" />
            <span className="text-[11px] font-sans">Ask Agent</span>
          </button>
          <button
            type="button"
            onClick={handleCopy}
            className="p-1 hover:bg-[#e2e8f0] dark:hover:bg-[#333333] hover:text-[#0f172a] dark:hover:text-white rounded transition-colors"
            title="Copy content"
          >
            {copied ? <Check className="w-3.5 h-3.5 text-[#10b981]" /> : <Copy className="w-3.5 h-3.5" />}
          </button>
          <button
            type="button"
            onClick={() => setIsSplitEditor(prev => !prev)}
            className="p-1 hover:bg-[#e2e8f0] dark:hover:bg-[#333333] hover:text-[#0f172a] dark:hover:text-white rounded transition-colors"
            title="Toggle Split View"
          >
            <SplitSquareVertical className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      {/* Breadcrumb Bar */}
      <div className="h-[24px] min-h-[24px] bg-white dark:bg-[#1e1e1e] border-b border-[#e2e8f0] dark:border-[#282828] px-3 flex items-center gap-1.5 text-[11px] text-[#64748b] dark:text-[#858585] select-none font-sans">
        <Folder className="w-3 h-3 text-[#2563eb] dark:text-[#38bdf8]" />
        <span className="hover:text-[#0f172a] dark:hover:text-white cursor-pointer">{activeWorkspacePath || 'workspace'}</span>
        <ChevronRight className="w-3 h-3 text-[#94a3b8] dark:text-[#555555]" />
        <span className="hover:text-[#0f172a] dark:hover:text-white cursor-pointer text-[#0f172a] dark:text-[#cccccc] font-medium">{activeTab?.fileName || 'file'}</span>
        <ChevronRight className="w-3 h-3 text-[#94a3b8] dark:text-[#555555]" />
        <span className="text-[#64748b] dark:text-[#858585] font-mono">Ln {cursorLine}, Col {cursorCol}</span>
      </div>

      {/* Find & Replace Floating Widget */}
      {isFindOpen && (
        <div className="absolute top-12 right-6 z-30 p-2.5 rounded-xl bg-white dark:bg-[#252526] border border-[#cbd5e1] dark:border-[#383838] shadow-2xl font-sans text-xs space-y-2 animate-in fade-in zoom-in-95 duration-100 min-w-[320px]">
          <div className="flex items-center gap-1.5">
            <button
              type="button"
              onClick={() => setShowReplace(prev => !prev)}
              className="p-1 rounded text-[#64748b] hover:bg-[#f1f5f9] dark:hover:bg-[#333333]"
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
                className="w-full pl-2 pr-14 py-1 rounded bg-[#f8fafc] dark:bg-[#1e1e1e] border border-[#cbd5e1] dark:border-[#383838] text-xs font-mono text-[#0f172a] dark:text-white focus:outline-hidden focus:ring-1 focus:ring-[#2563eb]"
              />
              <span className="absolute right-2 top-1 text-[10px] text-[#94a3b8] font-mono select-none">
                {matches.length > 0 ? `${currentMatchIndex + 1}/${matches.length}` : findQuery ? 'No match' : ''}
              </span>
            </div>
            <button
              type="button"
              onClick={() => setMatchCase(prev => !prev)}
              className={`px-1.5 py-0.5 rounded font-mono text-[10px] font-bold ${
                matchCase ? 'bg-[#2563eb] text-white' : 'text-[#64748b] hover:bg-[#f1f5f9] dark:hover:bg-[#333333]'
              }`}
              title="Match Case"
            >
              Aa
            </button>
            <button
              type="button"
              disabled={matches.length === 0}
              onClick={() => setCurrentMatchIndex(prev => (prev > 0 ? prev - 1 : matches.length - 1))}
              className="p-1 rounded hover:bg-[#f1f5f9] dark:hover:bg-[#333333] disabled:opacity-30"
              title="Previous Match"
            >
              <ChevronUp className="w-3.5 h-3.5" />
            </button>
            <button
              type="button"
              disabled={matches.length === 0}
              onClick={() => setCurrentMatchIndex(prev => (prev < matches.length - 1 ? prev + 1 : 0))}
              className="p-1 rounded hover:bg-[#f1f5f9] dark:hover:bg-[#333333] disabled:opacity-30"
              title="Next Match"
            >
              <ChevronDown className="w-3.5 h-3.5" />
            </button>
            <button
              type="button"
              onClick={() => setIsFindOpen(false)}
              className="p-1 rounded hover:bg-[#f1f5f9] dark:hover:bg-[#333333]"
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
                className="flex-1 px-2 py-1 rounded bg-[#f8fafc] dark:bg-[#1e1e1e] border border-[#cbd5e1] dark:border-[#383838] text-xs font-mono text-[#0f172a] dark:text-white focus:outline-hidden focus:ring-1 focus:ring-[#2563eb]"
              />
              <button
                type="button"
                onClick={handleReplaceOne}
                disabled={matches.length === 0}
                className="px-2 py-1 rounded bg-[#f1f5f9] dark:bg-[#333333] hover:bg-[#e2e8f0] text-[#0f172a] dark:text-white text-[11px] font-medium disabled:opacity-30"
              >
                Replace
              </button>
              <button
                type="button"
                onClick={handleReplaceAll}
                disabled={matches.length === 0}
                className="px-2 py-1 rounded bg-[#f1f5f9] dark:bg-[#333333] hover:bg-[#e2e8f0] text-[#0f172a] dark:text-white text-[11px] font-medium disabled:opacity-30"
              >
                All
              </button>
            </div>
          )}
        </div>
      )}

      {/* Editor Body: Line Numbers + Direct Interactive Textarea */}
      <div className="flex-1 flex overflow-hidden relative bg-white dark:bg-[#1e1e1e]">
        
        {/* Line Numbers Gutter (Scroll synchronized) */}
        <div 
          ref={gutterRef}
          className="w-[50px] min-w-[50px] bg-[#f8fafc] dark:bg-[#1e1e1e] text-[#94a3b8] dark:text-[#5c5c5c] text-right pr-3.5 py-3 select-none text-[12.5px] leading-[22px] font-mono border-r border-[#e2e8f0] dark:border-[#262626] overflow-hidden"
        >
          {lines.map((_, i) => {
            const lineNum = i + 1;
            const hasError = fileDiags.some(d => d.line === lineNum && d.severity === 'error');
            const hasWarning = fileDiags.some(d => d.line === lineNum && d.severity === 'warning');
            const isCurrent = lineNum === cursorLine;

            return (
              <div 
                key={i} 
                className={`flex items-center justify-end gap-1 h-[22px] transition-colors ${
                  isCurrent ? 'text-[#0f172a] dark:text-white font-bold' : 'hover:text-[#334155] dark:hover:text-[#999999]'
                }`}
              >
                {hasError ? (
                  <span className="w-1.5 h-1.5 rounded-full bg-[#ef4444]" title="LSP Error" />
                ) : hasWarning ? (
                  <span className="w-1.5 h-1.5 rounded-full bg-[#f59e0b]" title="LSP Warning" />
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
            onChange={handleTextChange}
            onScroll={handleScroll}
            onSelect={(e) => updateCursorAndLsp(e.currentTarget)}
            onKeyUp={(e) => updateCursorAndLsp(e.currentTarget)}
            onClick={(e) => updateCursorAndLsp(e.currentTarget)}
            onKeyDown={handleKeyDown}
            spellCheck={false}
            className="w-full h-full p-3 bg-transparent text-[#0f172a] dark:text-[#e2e8f0] caret-[#2563eb] dark:caret-[#38bdf8] font-mono text-[12.5px] leading-[22px] resize-none focus:outline-hidden selection:bg-[#bfdbfe]/60 dark:selection:bg-[#264f78]/60 overflow-y-auto overflow-x-auto whitespace-pre tab-2"
          />

          {/* Autocomplete Popup (LSP) */}
          {completionPos && completions.length > 0 && (
            <div 
              style={{ left: `${completionPos.x}px`, top: `${completionPos.y}px` }}
              className="absolute z-40 w-64 rounded-xl bg-white dark:bg-[#252526] border border-[#cbd5e1] dark:border-[#383838] shadow-2xl overflow-hidden animate-in fade-in zoom-in-95 duration-100 font-sans"
            >
              <div className="px-2.5 py-1 bg-[#f1f5f9] dark:bg-[#1f1f20] border-b border-[#e2e8f0] dark:border-[#333333] flex items-center justify-between text-[10px] text-[#64748b] dark:text-[#94a3b8] select-none">
                <span className="font-semibold uppercase tracking-wider">LSP Suggestions</span>
                <span>Tab/Enter to insert</span>
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
                          : 'text-[#334155] dark:text-[#d1d5db] hover:bg-[#f1f5f9] dark:hover:bg-[#333333]'
                      }`}
                    >
                      <div className="flex items-center gap-1.5 min-w-0">
                        <Code2 className="w-3.5 h-3.5 opacity-80 shrink-0" />
                        <span className="font-mono font-semibold truncate">{item.label}</span>
                      </div>
                      <span className={`text-[10px] opacity-70 truncate max-w-[90px] font-mono ${isSelected ? 'text-white' : 'text-[#64748b]'}`}>
                        {item.kind}
                      </span>
                    </div>
                  );
                })}
              </div>
            </div>
          )}
        </div>

      </div>

      {/* Bottom Status Bar */}
      <div className="h-[22px] min-h-[22px] bg-[#f1f5f9] dark:bg-[#181818] border-t border-[#e2e8f0] dark:border-[#282828] px-3 flex items-center justify-between text-[11px] text-[#64748b] dark:text-[#94a3b8] select-none font-sans">
        <div className="flex items-center gap-3">
          <span className="font-mono">Ln {cursorLine}, Col {cursorCol}</span>
          <span>Spaces: 2</span>
          <span>UTF-8</span>
          <span>
            {activeTab?.fileName?.endsWith('.ts') || activeTab?.fileName?.endsWith('.tsx') ? 'TypeScript' :
             activeTab?.fileName?.endsWith('.js') || activeTab?.fileName?.endsWith('.jsx') ? 'JavaScript' :
             activeTab?.fileName?.endsWith('.json') ? 'JSON' :
             activeTab?.fileName?.endsWith('.md') ? 'Markdown' :
             activeTab?.fileName?.endsWith('.zig') ? 'Zig' :
             activeTab?.fileName?.endsWith('.py') ? 'Python' :
             activeTab?.fileName?.endsWith('.rs') ? 'Rust' : 'Plain Text'}
          </span>
        </div>
        <div className="flex items-center gap-3">
          <span className="flex items-center gap-1 text-[#10b981]">
            <CheckCircle2 className="w-3 h-3" />
            <span>LSP Online</span>
          </span>
          <span>{activeWorkspacePath.split('/').pop() || 'workspace'}</span>
        </div>
      </div>

    </div>
  );
};
