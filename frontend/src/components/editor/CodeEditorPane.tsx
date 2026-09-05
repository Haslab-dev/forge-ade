import React, { useState, useRef, useEffect, useMemo } from 'react';
import {
  X,
  ChevronRight,
  Columns2,
  BookOpen,
  MoreHorizontal,
  FileCode,
  FilePlus2
} from 'lucide-react';
import { EditorState, Compartment } from '@codemirror/state';
import {
  EditorView,
  keymap,
  drawSelection,
  dropCursor,
  rectangularSelection,
  crosshairCursor,
  lineNumbers,
  highlightActiveLine,
  highlightActiveLineGutter,
  highlightSpecialChars
} from '@codemirror/view';
import {
  indentOnInput,
  indentUnit,
  bracketMatching
} from '@codemirror/language';
import {
  defaultKeymap,
  historyKeymap,
  indentWithTab,
  history
} from '@codemirror/commands';
import { searchKeymap, highlightSelectionMatches, search } from '@codemirror/search';
import { autocompletion, closeBrackets, closeBracketsKeymap, completionKeymap, type CompletionSource } from '@codemirror/autocomplete';
import { linter, lintGutter, type Diagnostic } from '@codemirror/lint';
import { useWorkspace } from '../../stores/workspaceStore';
import { LSPService, LSPCompletionItem } from '../../services/lspService';
import { ImagePreview } from './ImagePreview';
import { loadLanguage, themeExtensions } from './cmSetup';

interface CodeEditorPaneProps {
  tabId?: string;
  onSplitRight?: () => void;
  onTogglePreview?: () => void;
  isPreview?: boolean;
}

// Static LSP suggestions exposed as a CodeMirror completion source. Placeholder
// templates (${1:...}, $0) are flattened to plain text — CM apply has no
// tab-stop support and the old popup did the same conversion.
function stripSnippetPlaceholders(text: string): string {
  return text.replace(/\$\{\d+:?([^}]*)\}/g, '$1').replace(/\$0/g, '');
}

const lspCompletionSource: CompletionSource = (context) => {
  const word = context.matchBefore(/[a-zA-Z0-9_$]+/);
  if (!word || (word.from === word.to && !context.explicit)) return null;
  if (word.text.length < 2 && !context.explicit) return null;
  const items: LSPCompletionItem[] = LSPService.getCompletions(word.text);
  if (items.length === 0) return null;
  return {
    from: word.from,
    options: items.map(item => ({
      label: item.label,
      detail: item.detail,
      type: item.kind === 'function' ? 'function' : item.kind === 'keyword' ? 'keyword' : 'variable',
      apply: stripSnippetPlaceholders(item.insertText || item.label),
    })),
  };
};

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
    openTab,
    selectedFile,
    updateFileContent,
    setIsSplitEditor,
    activeWorkspacePath,
    diagnostics
  } = useWorkspace();

  const [isDark, setIsDark] = useState(() => document.documentElement.classList.contains('dark'));
  const [scrollTop, setScrollTop] = useState(0);
  const [scrollHeight, setScrollHeight] = useState(1);
  const [clientHeight, setClientHeight] = useState(1);

  const cmHostRef = useRef<HTMLDivElement>(null);
  const viewRef = useRef<EditorView | null>(null);
  const minimapRef = useRef<HTMLDivElement>(null);
  const applyingRef = useRef(false);

  // Compartments let language / theme / lint reconfigure without rebuilding the view.
  const languageCompartment = useRef(new Compartment());
  const themeCompartment = useRef(new Compartment());
  const lintCompartment = useRef(new Compartment());

  // Latest store values for callbacks captured once at view creation.
  const currentContentRef = useRef('');
  const currentFileNameRef = useRef('');
  const diagsRef = useRef(diagnostics);
  const writeRef = useRef(updateFileContent);
  const activeTabRef = useRef<{ fileId?: string } | null>(null);

  const effectiveTabId = tabId || activeTabId;
  const activeTab = openTabs.find(t => t.id === effectiveTabId);
  const currentContent = activeTab?.content ?? '';
  const currentFileName = activeTab?.fileName || '';
  const workspaceName = activeWorkspacePath ? activeWorkspacePath.split('/').pop() || '' : '';
  const isImageFile = useMemo(() => /\.(png|jpg|jpeg|gif|webp|ico|icns|bmp|svg)$/i.test(currentFileName), [currentFileName]);

  const fileDiags = useMemo(
    () => diagnostics.filter(d => d.filePath === activeTab?.filePath),
    [diagnostics, activeTab?.filePath]
  );

  currentContentRef.current = currentContent;
  currentFileNameRef.current = currentFileName;
  diagsRef.current = fileDiags;
  writeRef.current = updateFileContent;
  activeTabRef.current = activeTab ?? null;

  const syncMinimap = () => {
    const sd = viewRef.current?.scrollDOM;
    if (!sd) return;
    setScrollTop(sd.scrollTop);
    setScrollHeight(sd.scrollHeight || 1);
    setClientHeight(sd.clientHeight || 1);
  };

  const lintExtensions = () => [
    lintGutter(),
    linter((view): Diagnostic[] => {
      const path = currentFileNameRef.current;
      return (diagsRef.current || [])
        .filter(d => d.filePath === path || !d.filePath)
        .map(d => {
          const lineNo = Math.min(Math.max(1, d.line || 1), view.state.doc.lines);
          const line = view.state.doc.line(lineNo);
          const from = Math.min(line.from + Math.max(0, (d.column || 1) - 1), line.to);
          return {
            from,
            to: Math.max(from, Math.min(line.to, from + 1)),
            severity: d.severity === 'warning' ? 'warning' : d.severity === 'info' ? 'info' : 'error',
            message: d.message || '',
          } as Diagnostic;
        });
    }),
  ];

  // Create the CodeMirror view once for the lifetime of the pane.
  useEffect(() => {
    if (!cmHostRef.current || viewRef.current) return;

    const view = new EditorView({
      state: EditorState.create({
        doc: currentContentRef.current,
        extensions: [
          lineNumbers(),
          highlightActiveLineGutter(),
          highlightSpecialChars(),
          highlightActiveLine(),
          history(),
          drawSelection(),
          dropCursor(),
          EditorState.allowMultipleSelections.of(true),
          indentOnInput(),
          indentUnit.of('    '),
          bracketMatching(),
          closeBrackets(),
          autocompletion({ override: [lspCompletionSource] }),
          rectangularSelection(),
          crosshairCursor(),
          highlightSelectionMatches(),
          keymap.of([
            ...closeBracketsKeymap,
            ...defaultKeymap,
            ...searchKeymap,
            ...historyKeymap,
            ...completionKeymap,
            indentWithTab,
          ]),
          search({ top: true }),
          languageCompartment.current.of([]),
          themeCompartment.current.of(themeExtensions(document.documentElement.classList.contains('dark'))),
          lintCompartment.current.of(lintExtensions()),
          EditorView.updateListener.of((update) => {
            if (applyingRef.current) return;
            if (update.docChanged) {
              const tab = activeTabRef.current;
              if (tab?.fileId) writeRef.current(tab.fileId, update.state.doc.toString());
            }
            if (update.docChanged || update.geometryChanged) syncMinimap();
          }),
        ],
      }),
      parent: cmHostRef.current,
    });

    view.scrollDOM.addEventListener('scroll', syncMinimap);
    viewRef.current = view;
    syncMinimap();

    return () => {
      view.scrollDOM.removeEventListener('scroll', syncMinimap);
      view.destroy();
      viewRef.current = null;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Push external content changes (tab switch, format, agent edit) into the view.
  useEffect(() => {
    const view = viewRef.current;
    if (!view) return;
    if (view.state.doc.toString() !== currentContent) {
      applyingRef.current = true;
      view.dispatch({
        changes: { from: 0, to: view.state.doc.length, insert: currentContent },
      });
      applyingRef.current = false;
      syncMinimap();
    }
  }, [currentContent, activeTab?.id]);

  // Reconfigure the language when the active file type changes. Languages load
  // lazily, so dispatch happens when the package chunk resolves.
  useEffect(() => {
    let cancelled = false;
    loadLanguage(currentFileName).then(lang => {
      if (cancelled) return;
      viewRef.current?.dispatch({
        effects: languageCompartment.current.reconfigure(lang ?? []),
      });
    });
    return () => { cancelled = true; };
  }, [currentFileName]);

  // Reconfigure lint when the active file or its diagnostics change.
  useEffect(() => {
    const view = viewRef.current;
    if (!view) return;
    view.dispatch({
      effects: lintCompartment.current.reconfigure(lintExtensions()),
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [fileDiags, activeTab?.filePath]);

  // Follow the app's light/dark class so tokens and chrome stay in sync.
  useEffect(() => {
    const el = document.documentElement;
    const observer = new MutationObserver(() => {
      const dark = el.classList.contains('dark');
      setIsDark(dark);
      viewRef.current?.dispatch({
        effects: themeCompartment.current.reconfigure(themeExtensions(dark)),
      });
    });
    observer.observe(el, { attributes: true, attributeFilter: ['class'] });
    return () => observer.disconnect();
  }, []);

  // Jump to a specific line on tab activation / search match click.
  useEffect(() => {
    const view = viewRef.current;
    if (!view || !activeTab?.line) return;
    const lineNo = Math.min(Math.max(1, activeTab.line), view.state.doc.lines);
    const line = view.state.doc.line(lineNo);
    view.dispatch({ selection: { anchor: line.from }, scrollIntoView: true });
    view.focus();
  }, [activeTab?.id, activeTab?.line]);

  const handleMinimapClick = (e: React.MouseEvent<HTMLDivElement>) => {
    const view = viewRef.current;
    if (!view || !minimapRef.current) return;
    const rect = minimapRef.current.getBoundingClientRect();
    const clickY = e.clientY - rect.top;
    const targetRatio = clickY / rect.height;
    view.scrollDOM.scrollTop = targetRatio * view.scrollDOM.scrollHeight;
  };

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

  // Breadcrumbs symbol — the first markdown heading if any; no fake crumbs.
  const breadcrumbSymbol = useMemo(() => {
    if (!currentFileName.endsWith('.md')) return null;
    const match = currentContent.match(/^#{1,6}\s+(.+)$/m);
    return match ? match[1].trim() : null;
  }, [currentContent, currentFileName]);

  const lines = useMemo(() => currentContent.split('\n'), [currentContent]);

  const minimapViewportRatio = clientHeight / (scrollHeight || 1);
  const minimapTopRatio = scrollTop / (scrollHeight || 1);

  return (
    <div className="flex-1 flex flex-col h-full overflow-hidden bg-white dark:bg-[#181818] border-r border-[#e5e7eb] dark:border-[#282828] select-none font-sans">

      {/* Pane Tab Header Bar */}
      <div className="h-[35px] min-h-[35px] bg-[#f9fafb] dark:bg-[#181818] border-b border-[#e5e7eb] dark:border-[#282828] flex items-center justify-between px-2">
        {/* Open tabs — one pill per opened document */}
        <div className="flex items-center h-full overflow-x-auto min-w-0 flex-1">
          {openTabs.map(tab => {
            const isActive = tab.id === activeTab?.id;
            return (
              <div
                key={tab.id}
                onClick={() => openTab(tab)}
                title={tab.filePath}
                className={`h-full px-3 flex items-center gap-2 text-xs font-medium cursor-pointer border-r border-[#e5e7eb] dark:border-[#282828] whitespace-nowrap transition-colors ${
                  isActive
                    ? 'bg-white dark:bg-[#1e1e1e] text-[#111827] dark:text-white shadow-2xs'
                    : 'text-[#6b7280] dark:text-[#9ca3af] hover:text-[#111827] dark:hover:text-white hover:bg-[#f3f4f6] dark:hover:bg-[#222224]'
                }`}
              >
                {getTabFileIcon(tab.fileName)}
                <span className="max-w-[160px] truncate">{tab.fileName}</span>
                <button
                  type="button"
                  onClick={(e) => { e.stopPropagation(); closeTab(tab.id); }}
                  className="p-0.5 rounded hover:bg-[#e5e7eb] dark:hover:bg-[#333333] text-[#9ca3af] hover:text-[#111827] dark:hover:text-white transition-colors cursor-pointer"
                  title="Close"
                >
                  <X className="w-3 h-3" />
                </button>
              </div>
            );
          })}
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

      {activeTab ? (
        <>
          {/* Breadcrumbs Row */}
          <div className="h-[22px] min-h-[22px] bg-white dark:bg-[#181818] border-b border-[#f0f0f2] dark:border-[#262626] px-3 flex items-center gap-1.5 text-[11px] text-[#6b7280] dark:text-[#9ca3af] select-none font-sans overflow-x-auto">
            {workspaceName && (
              <>
                <span className="hover:text-[#111827] dark:hover:text-white cursor-pointer">{workspaceName}</span>
                <ChevronRight className="w-3 h-3 text-[#9ca3af]" />
              </>
            )}
            <div className="flex items-center gap-1 hover:text-[#111827] dark:hover:text-white cursor-pointer">
              {getTabFileIcon(currentFileName)}
              <span className="font-medium text-[#111827] dark:text-[#e2e8f0]">{currentFileName}</span>
            </div>
            {breadcrumbSymbol && (
              <>
                <ChevronRight className="w-3 h-3 text-[#9ca3af]" />
                <span className="text-[#6b7280] dark:text-[#9ca3af] truncate">{breadcrumbSymbol}</span>
              </>
            )}
          </div>
        </>
      ) : null}

      {/* Empty state — no tabs open, no phantom file titles */}
      {!activeTab && (
        <div className="flex-1 flex flex-col items-center justify-center gap-2 text-[#9ca3af] select-none">
          <FilePlus2 className="w-8 h-8" />
          <div className="text-sm font-medium text-[#6b7280] dark:text-[#9ca3af]">No file open</div>
          <div className="text-xs">Open a file from the Explorer or the Search panel.</div>
        </div>
      )}

      {/* If file is an image (PNG, JPG, SVG, ICO, ICNS, WEBP, GIF, BMP) */}
      {activeTab && (isImageFile ? (
        <ImagePreview
          filePath={activeTab?.filePath || selectedFile?.path || ''}
          fileName={currentFileName}
          rawContent={currentContent}
        />
      ) : (
        /* CodeMirror Editor Surface + Minimap */
        <div className="flex-1 flex overflow-hidden relative bg-white dark:bg-[#181818]">
          <div ref={cmHostRef} className="flex-1 min-w-0 h-full overflow-hidden" />

          {/* Minimap (Right side) */}
          <div
            ref={minimapRef}
            onClick={handleMinimapClick}
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
                const isComment = trimmed.startsWith('//') || trimmed.startsWith('/*') || trimmed.startsWith('*') || trimmed.startsWith('#');
                const isHeader = trimmed.startsWith('#') && (currentFileName.endsWith('.md') || currentFileName.endsWith('.py') || currentFileName.endsWith('.yml') || currentFileName.endsWith('.yaml'));
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

          {/* Editor mode badge — reflects the active CodeMirror language */}
          <div className="absolute bottom-2 right-[70px] px-2 py-0.5 rounded-full bg-[#f1f5f9]/90 dark:bg-[#222224]/90 border border-[#e5e7eb] dark:border-[#383838] text-[9px] font-mono font-semibold text-[#6b7280] dark:text-[#9ca3af] select-none pointer-events-none">
            {currentFileName.split('.').pop()?.toUpperCase() || 'TXT'}{isDark ? ' · DARK' : ''}
          </div>
        </div>
      ))}

    </div>
  );
};
