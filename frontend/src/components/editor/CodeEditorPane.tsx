import React, { useState, useRef, useEffect, useMemo, useCallback } from 'react';
import {
  X,
  ChevronRight,
  Columns2,
  Rows2,
  BookOpen,
  MoreHorizontal,
  FileCode,
  FilePlus2,
  Terminal as TerminalIcon,
  ChevronDown,
  Split,
  Wand2,
  Loader2,
  Pin, PinOff, Eye, EyeOff, PanelLeft,
  Copy, ExternalLink
} from 'lucide-react';
import { EditorState, Compartment, StateEffect, StateField } from '@codemirror/state';
import {
  EditorView,
  keymap,
  gutter,
  GutterMarker,
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
import { TerminalView } from '../terminal-view';
import { loadLanguage, themeExtensions } from './cmSetup';
import { EditorTab } from '../../types';
import { ApiBridge } from '../../services/apiBridge';
import { FormatCode } from '../../lib/wails';

interface CodeEditorPaneProps {
  tabId?: string;
  paneTabs?: EditorTab[];
  // Split panes pass this so clicking a tab switches THAT pane's file
  // instead of only the global active tab.
  onTabSelect?: (tab: EditorTab) => void;
  onClosePaneTab?: (tabId: string) => void;
  /** Bulk closes for the tab context menu (pinned tabs survive in EditorView). */
  onCloseOthers?: (keepTabId: string) => void;
  onCloseLeft?: (keepTabId: string) => void;
  onCloseRight?: (keepTabId: string) => void;
  onCloseClean?: () => void;
  onCloseAll?: () => void;
  /** Drag & drop: this pane's id and the cross-pane tab move handler. */
  paneId?: string;
  onMoveTab?: (tabId: string, fromPaneId: string, toPaneId: string, insertIndex?: number) => void;
  /** Zed-style pane focus: 'focused' shows the accent underline, 'unfocused'
      dims the tab chrome, 'solo' (single pane) looks like before splits. */
  paneFocus?: 'solo' | 'focused' | 'unfocused';
  onSplitRight?: () => void;
  onSplitLeft?: () => void;
  onSplitDown?: () => void;
  onSplitUp?: () => void;
  onTogglePreview?: () => void;
  isPreview?: boolean;
}

// Static LSP suggestions exposed as a CodeMirror completion source. Placeholder
// templates (${1:...}, $0) are flattened to plain text — CM apply has no
// tab-stop support and the old popup did the same conversion.
function stripSnippetPlaceholders(text: string): string {
  return text.replace(/\$\{\d+:?([^}]*)\}/g, '$1').replace(/\$0/g, '');
}

type LineChangeKind = 'added' | 'modified' | 'removed';

/** Line-level diff of saved vs current buffer. Common prefix/suffix is
    trimmed first; the remainder is resolved with an LCS table when small
    enough, otherwise marked wholesale as modified (VS Code-style bars). */
function diffLineStatuses(saved: string[], current: string[]): Map<number, LineChangeKind> {
  const marks = new Map<number, LineChangeKind>();
  let pre = 0;
  while (pre < saved.length && pre < current.length && saved[pre] === current[pre]) pre++;
  let sEnd = saved.length, cEnd = current.length;
  while (sEnd > pre && cEnd > pre && saved[sEnd - 1] === current[cEnd - 1]) { sEnd--; cEnd--; }
  const a = saved.slice(pre, sEnd);
  const b = current.slice(pre, cEnd);
  if (a.length === 0 && b.length === 0) return marks;

  const markRange = (kind: LineChangeKind, from0: number, to0: number) => {
    for (let i = from0; i < to0; i++) marks.set(pre + i + 1, kind);
  };

  if (a.length === 0) { markRange('added', 0, b.length); return marks; }
  if (b.length === 0) {
    marks.set(Math.min(pre + 1, Math.max(current.length, 1)), 'removed');
    return marks;
  }
  if (a.length * b.length > 1_500_000) { markRange('modified', 0, b.length); return marks; }

  const n = a.length, m = b.length;
  const dp = new Int32Array((n + 1) * (m + 1));
  for (let i = n - 1; i >= 0; i--) {
    for (let j = m - 1; j >= 0; j--) {
      dp[i * (m + 1) + j] = a[i] === b[j]
        ? dp[(i + 1) * (m + 1) + j + 1] + 1
        : Math.max(dp[(i + 1) * (m + 1) + j], dp[i * (m + 1) + j + 1]);
    }
  }
  let i = 0, j = 0;
  while (i < n && j < m) {
    if (a[i] === b[j]) { i++; j++; continue; }
    if (dp[(i + 1) * (m + 1) + j] >= dp[i * (m + 1) + j + 1]) {
      const line = Math.min(pre + j + 1, Math.max(current.length, 1));
      marks.set(line, marks.get(line) ?? 'removed');
      i++;
    } else {
      marks.set(pre + j + 1, marks.get(pre + j + 1) ?? 'added');
      j++;
    }
  }
  if (i < n) {
    const line = Math.min(pre + j + 1, Math.max(current.length, 1));
    marks.set(line, marks.get(line) ?? 'removed');
  }
  if (j < m) markRange('added', j, m);
  return marks;
}

const changeEffect = StateEffect.define<ReadonlyMap<number, LineChangeKind>>();
const changedLinesField = StateField.define<ReadonlyMap<number, LineChangeKind>>({
  create: () => new Map(),
  update(value, tr) {
    for (const e of tr.effects) if (e.is(changeEffect)) return e.value;
    return value;
  },
});

class ChangeMarker extends GutterMarker {
  constructor(private kind: LineChangeKind) { super(); }
  toDOM() {
    const el = document.createElement('div');
    el.className = `cm-change-marker cm-change-${this.kind}`;
    return el;
  }
}
const addedMarker = () => new ChangeMarker('added');
const modifiedMarker = () => new ChangeMarker('modified');
const removedMarker = () => new ChangeMarker('removed');

// Pointer-based tab dragging. HTML5 drag-and-drop is unreliable in
// WKWebView (drag sessions frequently never start, especially cross-pane),
// so tabs are dragged with raw pointer events: press a tab, move >4px, and
// the drop target pane + insert index are hit-tested from registered bars.
interface PaneTabDrag {
  tabId: string;
  fromPaneId: string;
  startX: number;
  startY: number;
  active: boolean;
}
let paneTabDrag: PaneTabDrag | null = null;
// paneId -> tab bar element, so a drag started in one pane can hit-test
// every pane's bar (cross-pane moves).
const paneBarRegistry = new Map<string, HTMLElement>();

/** Insert index within `bar` for a drop at clientX (tab midpoints). */
function dropIndexForBar(bar: HTMLElement, clientX: number): number {
  const tabs = Array.from(bar.querySelectorAll<HTMLElement>('[data-tab-id]'));
  for (let i = 0; i < tabs.length; i++) {
    const r = tabs[i].getBoundingClientRect();
    if (clientX < r.left + r.width / 2) return i;
  }
  return tabs.length;
}

/** Pane + insert index under a pointer position, or null when the pointer
    is outside every registered bar (drop outside a bar cancels the move). */
function hitTestPaneBars(clientX: number, clientY: number): { paneId: string; index: number } | null {
  for (const [paneId, bar] of paneBarRegistry) {
    const r = bar.getBoundingClientRect();
    if (clientY >= r.top - 6 && clientY <= r.bottom + 6 && clientX >= r.left - 8 && clientX <= r.right + 8) {
      return { paneId, index: dropIndexForBar(bar, clientX) };
    }
  }
  return null;
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
  paneTabs,
  onTabSelect,
  onClosePaneTab,
  onCloseOthers,
  onCloseLeft,
  onCloseRight,
  onCloseClean,
  onCloseAll,
  paneId,
  onMoveTab,
  paneFocus = 'solo',
  onSplitRight,
  onSplitLeft,
  onSplitDown,
  onSplitUp,
  onTogglePreview,
  isPreview = false
}) => {
  const {
    openTabs,
    activeTabId,
    closeTab,
    openTab,
    openTerminalTab,
    selectedFile,
    updateFileContent,
    setIsSplitEditor,
    activeWorkspacePath,
    diagnostics,
    setTabBuffer,
    saveTabToDisk,
    setDiagnostics,
    toggleTabPin,
    toggleTabReadOnly,
    setActiveActivity,
    setIsLeftSidebarOpen
  } = useWorkspace();

  // The value is unread; the state slot exists so theme flips re-render the
  // pane (CodeMirror theme re-apply reads the DOM classes directly).
  const [, setIsDark] = useState(() => document.documentElement.classList.contains('dark'));
  const [scrollTop, setScrollTop] = useState(0);
  const [scrollHeight, setScrollHeight] = useState(1);
  const [clientHeight, setClientHeight] = useState(1);
  const [isSplitMenuOpen, setIsSplitMenuOpen] = useState(false);
  const [isFormatting, setIsFormatting] = useState(false);
  // Zed-style right-click menu on a tab.
  const [tabMenu, setTabMenu] = useState<{ x: number; y: number; tab: EditorTab } | null>(null);
  // Insert position (tab index) while dragging a pane tab over this bar,
  // and the tab currently being dragged (for visual feedback).
  const [tabDropIndex, setTabDropIndex] = useState<number | null>(null);
  const [draggingTabId, setDraggingTabId] = useState<string | null>(null);
  const tabBarRef = useRef<HTMLDivElement | null>(null);
  // Clicks right after a finished drag are drags released over the same tab —
  // don't open anything.
  const suppressClickRef = useRef(false);

  const cmHostRef = useRef<HTMLDivElement>(null);
  const viewRef = useRef<EditorView | null>(null);
  const minimapRef = useRef<HTMLDivElement>(null);
  const applyingRef = useRef(false);

  // Compartments let language / theme / lint reconfigure without rebuilding the view.
  const languageCompartment = useRef(new Compartment());
  const themeCompartment = useRef(new Compartment());
  const lintCompartment = useRef(new Compartment());
  const editableCompartment = useRef(new Compartment());

  // Latest store values for callbacks captured once at view creation.
  const currentContentRef = useRef('');
  const currentFileNameRef = useRef('');
  const diagsRef = useRef(diagnostics);
  const bufferRef = useRef(setTabBuffer);
  const saveRef = useRef(saveTabToDisk);
  const savedContentRef = useRef<Map<string, string>>(new Map());
  const flushTimerRef = useRef<number | null>(null);
  const minimapRafRef = useRef<number>(0);
  const activeTabRef = useRef<{ fileId?: string } | null>(null);
  // Last seen active tab id — lets the content-sync effect tell a real tab
  // switch (must swap) from a same-tab typing echo (must not clobber).
  const prevActiveTabIdRef = useRef<string | undefined>(undefined);

  const displayedTabs = paneTabs || openTabs;
  const effectiveTabId = tabId || activeTabId;
  const activeTab = displayedTabs.find(t => t.id === effectiveTabId) || (displayedTabs.length > 0 ? displayedTabs[0] : undefined);
  const isTerminalTab = activeTab?.type === 'terminal';
  const currentContent = activeTab?.content ?? '';
  const currentFileName = activeTab?.fileName || '';
  const workspaceName = activeWorkspacePath ? activeWorkspacePath.split('/').pop() || '' : '';

  // Directory segments between the workspace root and the active file —
  // test-app/src/App.css renders as test-app › src › App.css. Works over
  // ssh:// too since both the workspace root and tab paths share the prefix.
  const breadcrumbDirs = useMemo(() => {
    const p = activeTab?.filePath;
    if (!p || !activeWorkspacePath || !p.startsWith(activeWorkspacePath)) return [];
    const rel = p.slice(activeWorkspacePath.length).replace(/^\/+/, '');
    return rel.split('/').filter(Boolean).slice(0, -1);
  }, [activeTab?.filePath, activeWorkspacePath]);
  const isImageFile = useMemo(() => /\.(png|jpg|jpeg|gif|webp|ico|icns|bmp|svg)$/i.test(currentFileName), [currentFileName]);

  const fileDiags = useMemo(
    () => diagnostics.filter(d => d.filePath === activeTab?.filePath),
    [diagnostics, activeTab?.filePath]
  );

  currentContentRef.current = currentContent;
  currentFileNameRef.current = currentFileName;
  diagsRef.current = fileDiags;
  bufferRef.current = setTabBuffer;
  saveRef.current = saveTabToDisk;
  activeTabRef.current = activeTab ?? null;

  // Formattable: config formats through the backend (JSON pretty with key
  // order kept, JSONL compact, TOML tidy — comments survive), everything
  // else through prettier. Remote (ssh://) paths are read via SFTP too.
  const isFormattableFile = (fileName: string): boolean =>
    /\.(json|jsonl|toml|tml|js|mjs|cjs|jsx|ts|tsx|css|scss|less|html|md|markdown)$/i.test(fileName);

  const formatDocument = useCallback(async () => {
    const view = viewRef.current;
    const tab = activeTabRef.current as EditorTab | null;
    if (!view || !tab?.filePath || isFormatting) return;
    const path = tab.filePath;
    if (!isFormattableFile(tab.fileName || path)) return;
    const content = view.state.doc.toString();
    setIsFormatting(true);
    try {
      const formatted = /\.(json|jsonl|toml|tml)$/i.test(path)
        ? await ApiBridge.formatConfigContent(path, content)
        : await FormatCode(path, content);
      if (formatted && formatted !== content && tab.fileId) {
        // Buffer update marks the tab dirty and pushes the new content into
        // the view through the external-changes effect. Disk write happens
        // on save.
        bufferRef.current(tab.fileId, formatted);
        computeChangeMarks();
      }
    } catch (e: any) {
      console.warn('format failed:', e?.message || e);
    } finally {
      setIsFormatting(false);
    }
  }, [isFormatting]);
  const formatRef = useRef(formatDocument);
  formatRef.current = formatDocument;

  // Flush the live buffer into the store (dirty marker + right-pane previews)
  // without touching the disk — the write happens on explicit save.
  const flushBuffer = useCallback(() => {
    const view = viewRef.current;
    const tab = activeTabRef.current as EditorTab | null;
    if (!view || !tab?.fileId || applyingRef.current) return;
    bufferRef.current(tab.fileId, view.state.doc.toString());
  }, []);

  // VS Code-style gutter markers: diff the buffer against the saved baseline.
  const computeChangeMarks = useCallback(() => {
    const view = viewRef.current;
    const tab = activeTabRef.current as EditorTab | null;
    if (!view || !tab) return;
    const saved = savedContentRef.current.get(tab.id);
    const marks = saved === undefined
      ? new Map()
      : diffLineStatuses(saved.split('\n'), view.state.doc.toString().split('\n'));
    view.dispatch({ effects: changeEffect.of(marks) });
  }, []);

  const loadSavedBaseline = useCallback(async (tab: EditorTab) => {
    if (tab.type !== 'code' || !tab.filePath) return;
    try {
      const disk = await ApiBridge.readFile(tab.filePath);
      savedContentRef.current.set(tab.id, disk);
    } catch {
      savedContentRef.current.set(tab.id, tab.content ?? '');
    }
    computeChangeMarks();
  }, [computeChangeMarks]);

  const saveDocument = useCallback(async () => {
    const view = viewRef.current;
    const tab = activeTabRef.current as EditorTab | null;
    if (!view || !tab?.filePath || tab.type !== 'code') return;
    // Cancel the pending buffer flush — the save below supersedes it.
    if (flushTimerRef.current) {
      window.clearTimeout(flushTimerRef.current);
      flushTimerRef.current = null;
    }
    let content = view.state.doc.toString();
    // Format on save (per request), then lint runs on the dispatched change.
    if (isFormattableFile(tab.fileName || tab.filePath)) {
      try {
        const formatted = /\.(json|jsonl|toml|tml)$/i.test(tab.filePath)
          ? await ApiBridge.formatConfigContent(tab.filePath, content)
          : await FormatCode(tab.filePath, content);
        if (formatted && formatted !== content) {
          view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: formatted } });
          content = formatted;
        }
      } catch (e: any) {
        console.warn('format on save failed:', e?.message || e);
      }
    }
    if (flushTimerRef.current) {
      window.clearTimeout(flushTimerRef.current);
      flushTimerRef.current = null;
    }
    await saveRef.current(tab.fileId, content);
    savedContentRef.current.set(tab.id, content);
    computeChangeMarks();
  }, []);
  const saveDocRef = useRef(saveDocument);
  saveDocRef.current = saveDocument;

  const syncMinimap = () => {
    // rAF coalescing — scroll events fired per frame re-rendered the pane.
    if (minimapRafRef.current) return;
    minimapRafRef.current = requestAnimationFrame(() => {
      minimapRafRef.current = 0;
      const sd = viewRef.current?.scrollDOM;
      if (!sd) return;
      setScrollTop(sd.scrollTop);
      setScrollHeight(sd.scrollHeight || 1);
      setClientHeight(sd.clientHeight || 1);
    });
  };

  const lintExtensions = () => [
    lintGutter(),
    linter(async (view): Promise<Diagnostic[]> => {
      const path = currentFileNameRef.current;
      const content = view.state.doc.toString();
      if (!path) return [];
      const ext = path.toLowerCase().split('.').pop() || '';

      // JS/TS family: esbuild parser through the backend binding.
      if (['js', 'mjs', 'cjs', 'jsx', 'ts', 'mts', 'cts', 'tsx'].includes(ext)) {
        try {
          const res = await ApiBridge.checkSyntax(path, content);
          return (res || []).map((d: any) => {
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
        } catch {
          return [];
        }
      }

      // JSON: local parse (no backend roundtrip).
      if (ext === 'json') {
        try {
          JSON.parse(content);
          return [];
        } catch (e: any) {
          const msg = String(e?.message || 'invalid JSON');
          const m = /position (\d+)/.exec(msg);
          const pos = m ? Math.min(Number(m[1]), content.length) : 0;
          const line = view.state.doc.lineAt(pos);
          return [{
            from: line.from,
            to: Math.max(line.from, Math.min(line.to, line.from + 1)),
            severity: 'error' as const,
            message: msg.split(' (')[0],
          }];
        }
      }

      // Merge store diagnostics for this file (LSP etc.).
      return (diagsRef.current || [])
        .filter((d: any) => d.filePath === path)
        .map((d: any) => {
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
    }, { delay: 600 }),
  ];

  // The CodeMirror host only renders for a non-image active tab; create the
  // view when the host appears and destroy it when it goes away. A mount-once
  // effect here left the pane permanently blank when the pane first mounted
  // with no tabs open (fresh start) or while an image tab was active.
  const hostMounted = !!activeTab && !isImageFile && !isTerminalTab;

  // Create the CodeMirror view when the host surface is present.
  useEffect(() => {
    if (!hostMounted || !cmHostRef.current || viewRef.current) return;

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
            {
              key: 'Mod-s',
              run: () => {
                void saveDocRef.current();
                return true;
              },
            },
            {
              key: 'Shift-Alt-f',
              run: () => {
                void formatRef.current();
                return true;
              },
            },
            ...closeBracketsKeymap,
            ...defaultKeymap,
            ...searchKeymap,
            ...historyKeymap,
            ...completionKeymap,
            indentWithTab,
          ]),
          search({ top: true }),
          changedLinesField,
          gutter({
            class: 'cm-change-gutter',
            lineMarker(view, line) {
              const marks = view.state.field(changedLinesField, false);
              if (!marks || marks.size === 0) return null;
              const kind = marks.get(view.state.doc.lineAt(line.from).number);
              if (kind === 'added') return addedMarker();
              if (kind === 'modified') return modifiedMarker();
              if (kind === 'removed') return removedMarker();
              return null;
            },
          }),
          languageCompartment.current.of([]),
          themeCompartment.current.of(themeExtensions(document.documentElement.classList.contains('dark'))),
          lintCompartment.current.of(lintExtensions()),
          editableCompartment.current.of([
            EditorView.editable.of(!activeTab?.isReadOnly),
            EditorState.readOnly.of(!!activeTab?.isReadOnly)
          ]),
          EditorView.updateListener.of((update) => {
            if (applyingRef.current) return;
            if (update.docChanged) {
              const tab = activeTabRef.current;
              if (tab?.fileId) {
                // Debounced buffer flush — typing used to hit the disk on
                // every keystroke.
                if (flushTimerRef.current) window.clearTimeout(flushTimerRef.current);
                flushTimerRef.current = window.setTimeout(() => {
                  flushTimerRef.current = null;
                  flushBuffer();
                  computeChangeMarks();
                }, 350);
              }
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
      if (minimapRafRef.current) cancelAnimationFrame(minimapRafRef.current);
      view.destroy();
      viewRef.current = null;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [hostMounted]);

  // Push external content changes (tab switch, format, agent edit) into the view.
  useEffect(() => {
    const view = viewRef.current;
    if (!view) return;
    const tabSwitched = prevActiveTabIdRef.current !== activeTab?.id;
    prevActiveTabIdRef.current = activeTab?.id;
    if (view.state.doc.toString() === currentContent) return;
    // The buffer is the source of truth only for the SAME tab while the user
    // is typing — a stale store value must never clobber in-progress edits.
    // A genuine tab switch always swaps: explorer clicks don't blur
    // CodeMirror, so view.hasFocus alone must not block the switch.
    if (!tabSwitched && view.hasFocus) return;
    applyingRef.current = true;
    view.dispatch({
      changes: { from: 0, to: view.state.doc.length, insert: currentContent },
    });
    applyingRef.current = false;
    syncMinimap();
    computeChangeMarks();
  }, [currentContent, activeTab?.id, computeChangeMarks]);

  // Evict on-disk baselines for closed tabs — the map holds a full file copy
  // per tab ever opened and would otherwise grow unbounded for the session.
  useEffect(() => {
    if (savedContentRef.current.size === 0) return;
    const live = new Set(displayedTabs.map(t => t.id));
    for (const id of Array.from(savedContentRef.current.keys())) {
      if (!live.has(id)) savedContentRef.current.delete(id);
    }
  }, [displayedTabs]);

  // Baseline for gutter change markers: load the on-disk content once per tab.
  useEffect(() => {
    const tab = activeTab;
    if (!tab || tab.type !== 'code' || isImageFile) return;
    if (savedContentRef.current.get(tab.id) === undefined) {
      void loadSavedBaseline(tab);
    } else {
      computeChangeMarks();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeTab?.id, activeTab?.filePath]);

  // Flush pending edits when the editor loses focus (tab click, pane switch).
  useEffect(() => {
    const view = viewRef.current;
    if (!view) return;
    const el = view.contentDOM;
    const onBlur = () => flushBuffer();
    el.addEventListener('blur', onBlur);
    return () => el.removeEventListener('blur', onBlur);
  }, []);

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

  // Apply read-only toggles (tab context menu) without rebuilding the view.
  useEffect(() => {
    const view = viewRef.current;
    if (!view) return;
    view.dispatch({
      effects: editableCompartment.current.reconfigure([
        EditorView.editable.of(!activeTab?.isReadOnly),
        EditorState.readOnly.of(!!activeTab?.isReadOnly)
      ]),
    });
  }, [activeTab?.id, activeTab?.isReadOnly]);

  // Pointer-based tab dragging (HTML5 DnD is unreliable in WKWebView): the
  // source pane activates the drag after a 4px threshold and commits the move
  // on release; every pane tracks the pointer so its own bar can show the
  // insert marker when the drag is over it. Drop index + target pane come
  // from a module-level hit-test over all registered bars.
  useEffect(() => {
    if (!paneId || !onMoveTab) return;

    const handleMove = (e: PointerEvent) => {
      if (!paneTabDrag) return;
      if (paneTabDrag.fromPaneId === paneId && !paneTabDrag.active) {
        if (Math.abs(e.clientX - paneTabDrag.startX) < 4 && Math.abs(e.clientY - paneTabDrag.startY) < 4) return;
        paneTabDrag.active = true;
        setDraggingTabId(paneTabDrag.tabId);
      }
      if (!paneTabDrag.active) return;
      const hit = hitTestPaneBars(e.clientX, e.clientY);
      setTabDropIndex(hit && hit.paneId === paneId ? hit.index : null);
    };

    const handleUp = (e: PointerEvent) => {
      if (!paneTabDrag || paneTabDrag.fromPaneId !== paneId) return;
      const drag = paneTabDrag;
      paneTabDrag = null;
      setDraggingTabId(null);
      setTabDropIndex(null);
      if (!drag.active) return;

      const hit = hitTestPaneBars(e.clientX, e.clientY);
      if (!hit) return; // released outside every tab bar — cancel
      // With pointer capture the release fires a click on the source tab —
      // don't let it re-open/activate anything.
      suppressClickRef.current = true;
      window.setTimeout(() => { suppressClickRef.current = false; }, 0);
      onMoveTab(drag.tabId, drag.fromPaneId, hit.paneId, hit.index);
    };

    const handleCancel = () => {
      if (!paneTabDrag || paneTabDrag.fromPaneId !== paneId) return;
      paneTabDrag = null;
      setDraggingTabId(null);
      setTabDropIndex(null);
    };

    window.addEventListener('pointermove', handleMove);
    window.addEventListener('pointerup', handleUp);
    window.addEventListener('pointercancel', handleCancel);
    return () => {
      window.removeEventListener('pointermove', handleMove);
      window.removeEventListener('pointerup', handleUp);
      window.removeEventListener('pointercancel', handleCancel);
    };
  }, [paneId, onMoveTab]);

  // Keep the active tab visible: opening a file (or switching/reordering
  // tabs) scrolls the bar so the active pill is in view. Runs on every
  // render — already-visible tabs are a cheap no-op — because opening an
  // already-active-but-scrolled-away tab changes no prop this effect could
  // otherwise key on. Rect deltas (not offsetLeft) so the math is correct
  // regardless of which ancestor is the offsetParent.
  useEffect(() => {
    const bar = tabBarRef.current;
    if (!bar || !activeTab?.id) return;
    const el = bar.querySelector<HTMLElement>(`[data-tab-id="${CSS.escape(activeTab.id)}"]`);
    if (!el) return;
    const barRect = bar.getBoundingClientRect();
    const elRect = el.getBoundingClientRect();
    const left = elRect.left - barRect.left + bar.scrollLeft;
    const right = left + elRect.width;
    if (left < bar.scrollLeft + 8) bar.scrollLeft = Math.max(0, left - 8);
    else if (right > bar.scrollLeft + bar.clientWidth - 8) bar.scrollLeft = right - bar.clientWidth + 8;
  });

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

  const getTabFileIcon = (fileName: string, tabType?: string) => {
    if (tabType === 'terminal' || fileName.toLowerCase() === 'terminal') {
      return <TerminalIcon className="w-3.5 h-3.5 text-success shrink-0" />;
    }
    if (fileName.endsWith('.md')) {
      return <span className="w-3.5 h-3.5 rounded bg-primary text-white text-ui-xs font-bold flex items-center justify-center shrink-0 font-mono shadow-2xs">M↓</span>;
    }
    if (fileName.endsWith('.php')) {
      return <span className="w-3.5 h-3.5 text-primary font-bold text-ui-xs flex items-center justify-center shrink-0 font-mono">php</span>;
    }
    if (fileName.endsWith('.json')) {
      return <span className="text-warning font-bold text-ui-xs font-mono shrink-0">{'{}'}</span>;
    }
    return <FileCode className="w-3.5 h-3.5 text-info shrink-0" />;
  };

  // Breadcrumbs symbol — the first markdown heading if any; no fake crumbs.
  const breadcrumbSymbol = useMemo(() => {
    if (!currentFileName.endsWith('.md')) return null;
    const match = currentContent.match(/^#{1,6}\s+(.+)$/m);
    return match ? match[1].trim() : null;
  }, [currentContent, currentFileName]);

  // Minimap renders at most ~100 bars — avoid splitting multi-MB files on
  // every keystroke just to throw most lines away.
  const lines: string[] = useMemo(() => {
    if (!currentContent) return [];
    const out: string[] = [];
    let start = 0;
    for (let i = 0; i <= 100; i++) {
      const nl = currentContent.indexOf('\n', start);
      if (nl === -1) {
        out.push(currentContent.slice(start));
        break;
      }
      out.push(currentContent.slice(start, nl));
      start = nl + 1;
    }
    return out;
  }, [currentContent]);

  const minimapViewportRatio = clientHeight / (scrollHeight || 1);
  const minimapTopRatio = scrollTop / (scrollHeight || 1);

  return (
    <div
      className="flex-1 flex flex-col h-full overflow-hidden bg-background border-r border-border select-none font-sans"
      // A finished drag fires a trailing click on the source tab (pointer
      // capture retargets pointerup). Kill it in the capture phase so it
      // can't bubble to EditorView's pane wrapper — that onClick would
      // re-focus the SOURCE pane right after the move focused the target.
      onClickCapture={(e) => {
        if (suppressClickRef.current) e.stopPropagation();
      }}
    >

      {/* Pane Tab Header Bar — accent underline marks the focused pane */}
      <div
        className={`h-[35px] min-h-[35px] bg-surface dark:bg-background border-b flex items-center justify-between px-2 ${
          paneFocus === 'focused' ? 'border-primary/50' : 'border-border'
        }`}
      >
        {/* Open tabs — one pill per opened document */}
        <div
          ref={(el) => {
            tabBarRef.current = el;
            if (paneId && el) paneBarRegistry.set(paneId, el);
            else if (el === null && paneId) paneBarRegistry.delete(paneId);
          }}
          role="tablist"
          aria-label="Open editors"
          className="flex items-center h-full overflow-x-auto min-w-0 flex-1"
        >
          {displayedTabs.map((tab, tabIndex) => (
            <React.Fragment key={tab.id}>
            {tabDropIndex === tabIndex && (
              <div className="w-0.5 self-stretch bg-primary shrink-0 pointer-events-none" aria-hidden="true" />
            )}
            {(() => {
            const isActive = tab.id === activeTab?.id;
            return (
              <div
                key={tab.id}
                role="tab"
                aria-selected={isActive}
                tabIndex={isActive ? 0 : -1}
                data-tab-id={tab.id}
                onPointerDown={(e) => {
                  if (e.button !== 0 || !paneId || !onMoveTab) return;
                  // Don't start drags from the embedded close/pin buttons.
                  if ((e.target as HTMLElement).closest('button')) return;
                  paneTabDrag = { tabId: tab.id, fromPaneId: paneId, startX: e.clientX, startY: e.clientY, active: false };
                  // Capture keeps move/up events flowing to the window even
                  // when the pointer leaves the tab (or the pane).
                  try { e.currentTarget.setPointerCapture(e.pointerId); } catch { /* non-critical */ }
                }}
                onClick={(e) => {
                  if (suppressClickRef.current) {
                    suppressClickRef.current = false;
                    e.preventDefault();
                    return;
                  }
                  if (onTabSelect) onTabSelect(tab); else openTab(tab);
                }}
                onContextMenu={(e) => {
                  e.preventDefault();
                  setTabMenu({ x: e.clientX, y: e.clientY, tab });
                }}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' || e.key === ' ') {
                    e.preventDefault();
                    if (onTabSelect) onTabSelect(tab); else openTab(tab);
                  } else if (e.key === 'ArrowRight' || e.key === 'ArrowLeft') {
                    e.preventDefault();
                    const idx = displayedTabs.findIndex(t => t.id === tab.id);
                    const next = displayedTabs[(idx + (e.key === 'ArrowRight' ? 1 : -1) + displayedTabs.length) % displayedTabs.length];
                    if (next) {
                      if (onTabSelect) onTabSelect(next); else openTab(next);
                    }
                  }
                }}
                title={tab.filePath}
                className={`h-full px-3 flex items-center gap-2 text-xs font-medium cursor-pointer border-r border-border whitespace-nowrap shrink-0 transition-colors ${
                  isActive
                    ? paneFocus === 'unfocused'
                      ? 'bg-card/60 text-foreground-subtle'
                      : 'bg-card text-foreground shadow-2xs'
                    : 'text-foreground-subtle hover:text-foreground hover:bg-surface-hover dark:hover:bg-[#222224]'
                } ${draggingTabId === tab.id ? 'opacity-40' : ''}`}
              >
                {getTabFileIcon(tab.fileName, tab.type)}
                <span className={`max-w-[160px] truncate ${tab.isReadOnly ? 'italic opacity-80' : ''}`}>{tab.fileName}</span>
                {tab.isModified && (
                  <span className="size-1.5 shrink-0 rounded-full bg-warning" aria-hidden="true" />
                )}
                {tab.isPinned && (
                  <Pin className="w-3 h-3 text-primary shrink-0 fill-primary" aria-label="Pinned" />
                )}
                <button
                  type="button"
                  onClick={(e) => {
                    e.stopPropagation();
                    if (onClosePaneTab) {
                      onClosePaneTab(tab.id);
                    } else {
                      closeTab(tab.id);
                    }
                  }}
                  aria-label={`Close ${tab.fileName}`}
                  className="p-0.5 rounded hover:bg-surface-hover dark:hover:bg-surface-hover text-foreground-subtle hover:text-foreground transition-colors cursor-pointer"
                  title="Close"
                >
                  <X className="w-3 h-3" aria-hidden="true" />
                </button>
              </div>
            );
            })()}
            {tabDropIndex === displayedTabs.length && (
              <div className="w-0.5 self-stretch bg-primary shrink-0 pointer-events-none" aria-hidden="true" />
            )}
            </React.Fragment>
          ))}
        </div>

        {/* Right Action Icons */}
        <div className="flex items-center gap-1 text-foreground-subtle relative">
          {/* Add Shell/Terminal in this Editor Pane */}
          <button
            type="button"
            onClick={() => openTerminalTab()}
            aria-label="Open Shell in Editor Pane"
            className="p-1 rounded hover:bg-surface-hover dark:hover:bg-surface-hover hover:text-foreground transition-colors cursor-pointer"
            title="Open Shell in Editor Pane"
          >
            <TerminalIcon className="w-3.5 h-3.5" aria-hidden="true" />
          </button>

          {currentFileName.endsWith('.md') && (
            <button
              type="button"
              onClick={onTogglePreview}
              aria-label="Toggle Markdown Preview"
              aria-pressed={isPreview}
              className={`p-1 rounded hover:bg-surface-hover dark:hover:bg-surface-hover transition-colors cursor-pointer ${
                isPreview ? 'text-primary bg-primary/10 dark:bg-card' : ''
              }`}
              title="Toggle Markdown Preview"
            >
              <BookOpen className="w-3.5 h-3.5" aria-hidden="true" />
            </button>
          )}

          {/* Format Document */}
          {!isImageFile && !isTerminalTab && activeTab?.filePath && isFormattableFile(currentFileName) && (
            <button
              type="button"
              onClick={() => void formatDocument()}
              disabled={isFormatting}
              aria-label="Format Document"
              className="p-1 rounded hover:bg-surface-hover dark:hover:bg-surface-hover hover:text-foreground transition-colors cursor-pointer disabled:opacity-50"
              title="Format Document (Shift+Alt+F)"
            >
              {isFormatting ? (
                <Loader2 className="w-3.5 h-3.5 animate-spin" aria-hidden="true" />
              ) : (
                <Wand2 className="w-3.5 h-3.5" aria-hidden="true" />
              )}
            </button>
          )}

          {/* Directional Split Pane Actions (Zed / VSCode style) */}
          <button
            type="button"
            onClick={onSplitRight || (() => setIsSplitEditor(prev => !prev))}
            aria-label="Split Right"
            className="p-1 rounded hover:bg-surface-hover dark:hover:bg-surface-hover hover:text-foreground transition-colors cursor-pointer"
            title="Split Right"
          >
            <Columns2 className="w-3.5 h-3.5" aria-hidden="true" />
          </button>

          <button
            type="button"
            onClick={onSplitDown || (() => setIsSplitEditor(prev => !prev))}
            aria-label="Split Down"
            className="p-1 rounded hover:bg-surface-hover dark:hover:bg-surface-hover hover:text-foreground transition-colors cursor-pointer"
            title="Split Down"
          >
            <Rows2 className="w-3.5 h-3.5" aria-hidden="true" />
          </button>

          {/* More Split Direction Options */}
          <div
            className="relative"
            onKeyDown={(e) => { if (e.key === 'Escape') setIsSplitMenuOpen(false); }}
          >
            <button
              type="button"
              onClick={() => setIsSplitMenuOpen((prev: boolean) => !prev)}
              aria-haspopup="menu"
              aria-expanded={isSplitMenuOpen}
              aria-label="Split Options"
              className="p-1 rounded hover:bg-surface-hover dark:hover:bg-surface-hover hover:text-foreground transition-colors cursor-pointer"
              title="Split Options..."
            >
              <MoreHorizontal className="w-3.5 h-3.5" aria-hidden="true" />
            </button>

            {isSplitMenuOpen && (
              <div
                role="menu"
                aria-label="Split options"
                className="absolute right-0 top-full mt-1 w-44 rounded-xl bg-popover border border-border shadow-2xl py-1 text-xs select-none z-50 font-sans"
                onMouseLeave={() => setIsSplitMenuOpen(false)}
              >
                <button
                  type="button"
                  role="menuitem"
                  onClick={() => {
                    if (onSplitRight) onSplitRight();
                    else setIsSplitEditor(true);
                    setIsSplitMenuOpen(false);
                  }}
                  className="w-full px-3 py-1.5 text-left hover:bg-surface-hover dark:hover:bg-card flex items-center justify-between cursor-pointer"
                >
                  <span className="flex items-center gap-2">
                    <Columns2 className="w-3.5 h-3.5 text-primary" aria-hidden="true" /> Split Right
                  </span>
                </button>
                <button
                  type="button"
                  role="menuitem"
                  onClick={() => {
                    if (onSplitDown) onSplitDown();
                    else setIsSplitEditor(true);
                    setIsSplitMenuOpen(false);
                  }}
                  className="w-full px-3 py-1.5 text-left hover:bg-surface-hover dark:hover:bg-card flex items-center justify-between cursor-pointer"
                >
                  <span className="flex items-center gap-2">
                    <Rows2 className="w-3.5 h-3.5 text-success" aria-hidden="true" /> Split Down
                  </span>
                </button>
                <button
                  type="button"
                  role="menuitem"
                  onClick={() => {
                    if (onSplitLeft) onSplitLeft();
                    else setIsSplitEditor(true);
                    setIsSplitMenuOpen(false);
                  }}
                  className="w-full px-3 py-1.5 text-left hover:bg-surface-hover dark:hover:bg-card flex items-center justify-between cursor-pointer"
                >
                  <span className="flex items-center gap-2">
                    <Columns2 className="w-3.5 h-3.5 text-warning scale-x-[-1]" aria-hidden="true" /> Split Left
                  </span>
                </button>
                <button
                  type="button"
                  role="menuitem"
                  onClick={() => {
                    if (onSplitUp) onSplitUp();
                    else setIsSplitEditor(true);
                    setIsSplitMenuOpen(false);
                  }}
                  className="w-full px-3 py-1.5 text-left hover:bg-surface-hover dark:hover:bg-card flex items-center justify-between cursor-pointer"
                >
                  <span className="flex items-center gap-2">
                    <Rows2 className="w-3.5 h-3.5 text-primary scale-y-[-1]" aria-hidden="true" /> Split Up
                  </span>
                </button>
                <div className="my-1 border-t border-border" />
                <button
                  type="button"
                  role="menuitem"
                  onClick={() => {
                    openTerminalTab();
                    setIsSplitMenuOpen(false);
                  }}
                  className="w-full px-3 py-1.5 text-left hover:bg-surface-hover dark:hover:bg-card flex items-center justify-between cursor-pointer"
                >
                  <span className="flex items-center gap-2">
                    <TerminalIcon className="w-3.5 h-3.5 text-success" aria-hidden="true" /> New Terminal Tab
                  </span>
                </button>
              </div>
            )}
          </div>
        </div>
      </div>

      {activeTab ? (
        <>
          {/* Breadcrumbs Row */}
          <div className="h-[22px] min-h-[22px] bg-background border-b border-border dark:border-surface px-3 flex items-center gap-1.5 text-ui-xs text-foreground-subtle select-none font-sans overflow-x-auto">
            {workspaceName && (
              <>
                <span>{workspaceName}</span>
                <ChevronRight className="w-3 h-3 text-foreground-subtle" aria-hidden="true" />
              </>
            )}
            {breadcrumbDirs.map((dir, i) => (
              <React.Fragment key={`${dir}-${i}`}>
                <span>{dir}</span>
                <ChevronRight className="w-3 h-3 text-foreground-subtle" aria-hidden="true" />
              </React.Fragment>
            ))}
            <div className="flex items-center gap-1">
              {getTabFileIcon(currentFileName, activeTab.type)}
              <span className="font-medium text-foreground dark:text-foreground-secondary">{currentFileName}</span>
            </div>
            {breadcrumbSymbol && (
              <>
                <ChevronRight className="w-3 h-3 text-foreground-subtle" aria-hidden="true" />
                <span className="text-foreground-subtle truncate">{breadcrumbSymbol}</span>
              </>
            )}
          </div>
        </>
      ) : null}

      {/* Empty state — no tabs open, no phantom file titles */}
      {!activeTab && (
        <div className="flex-1 flex flex-col items-center justify-center gap-2 text-foreground-subtle select-none">
          <FilePlus2 className="w-8 h-8" />
          <div className="text-sm font-medium text-foreground-subtle">No file open</div>
          <div className="text-xs">Open a file from the Explorer or the Search panel.</div>
        </div>
      )}

      {/* Tab content rendering */}
      {activeTab && (isTerminalTab ? (
        <div className="flex-1 flex flex-col h-full bg-background overflow-hidden">
          {activeTab.terminalSessionId ? (
            <TerminalView sessionId={activeTab.terminalSessionId} isActive={true} />
          ) : (
            <div className="flex-1 flex items-center justify-center text-xs text-foreground-subtle">
              Terminal session initializing...
            </div>
          )}
        </div>
      ) : isImageFile ? (
        <ImagePreview
          filePath={activeTab?.filePath || selectedFile?.path || ''}
          fileName={currentFileName}
          rawContent={currentContent}
        />
      ) : (
        /* CodeMirror Editor Surface + Minimap */
        <div className="flex-1 flex overflow-hidden relative bg-background">
          <div ref={cmHostRef} className="flex-1 min-w-0 h-full overflow-hidden" />

          {/* Minimap (Right side) */}
          <div
            ref={minimapRef}
            onClick={handleMinimapClick}
            className="w-[60px] min-w-[60px] h-full bg-panel border-l border-border dark:border-surface overflow-hidden select-none relative cursor-pointer hidden md:block"
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
                        ? 'bg-primary dark:bg-[#60a5fa]'
                        : isComment
                        ? 'bg-fg-tertiary'
                        : 'bg-fg-tertiary'
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
              className="absolute left-0 right-0 bg-primary/10 dark:bg-white/10 border-y border-primary/30 dark:border-white/20 transition-all pointer-events-none"
            />
          </div>
        </div>
      ))}

      {/* Zed-style tab context menu */}
      {tabMenu && (() => {
        const t = tabMenu.tab;
        const isCodeTab = t.type === 'code';
        const hasFile = !!t.filePath && !t.filePath.startsWith('terminal');
        const dir = hasFile && t.filePath.includes('/')
          ? t.filePath.substring(0, t.filePath.lastIndexOf('/'))
          : activeWorkspacePath;
        const itemCls = "w-full px-3 py-1.5 text-left hover:bg-surface-hover dark:hover:bg-card flex items-center gap-2 cursor-pointer text-foreground-subtle hover:text-foreground transition-colors";
        const sep = <div key="sep" className="my-1 border-t border-border" />;
        const closeThis = () => { if (onClosePaneTab) onClosePaneTab(t.id); else closeTab(t.id); };
        return (
          <div
            className="fixed inset-0 z-[70]"
            onClick={() => setTabMenu(null)}
            onContextMenu={(e) => { e.preventDefault(); setTabMenu(null); }}
          >
            <div
              role="menu"
              aria-label="Tab actions"
              style={{
                left: Math.max(8, Math.min(tabMenu.x, window.innerWidth - 260)),
                top: Math.max(8, Math.min(tabMenu.y, window.innerHeight - 460))
              }}
              className="absolute w-60 rounded-xl border border-border bg-popover p-1.5 shadow-xl text-ui-sm select-none"
              onClick={(e) => e.stopPropagation()}
            >
              <button type="button" role="menuitem" className={itemCls} onClick={() => { closeThis(); setTabMenu(null); }}>
                <X className="w-3.5 h-3.5" /> Close
              </button>
              <button type="button" role="menuitem" className={itemCls} onClick={() => { onCloseOthers?.(t.id); setTabMenu(null); }}>
                <X className="w-3.5 h-3.5" /> Close Others
              </button>
              <button type="button" role="menuitem" className={itemCls} onClick={() => { onCloseLeft?.(t.id); setTabMenu(null); }}>
                <X className="w-3.5 h-3.5" /> Close Left
              </button>
              <button type="button" role="menuitem" className={itemCls} onClick={() => { onCloseRight?.(t.id); setTabMenu(null); }}>
                <X className="w-3.5 h-3.5" /> Close Right
              </button>

              {sep}

              <button type="button" role="menuitem" className={itemCls} onClick={() => { onCloseClean?.(); setTabMenu(null); }}>
                <X className="w-3.5 h-3.5" /> Close Clean
              </button>
              <button type="button" role="menuitem" className={itemCls} onClick={() => { onCloseAll?.(); setTabMenu(null); }}>
                <X className="w-3.5 h-3.5" /> Close All
              </button>

              {isCodeTab && (<> {sep}
                <button type="button" role="menuitem" className={itemCls} onClick={() => { toggleTabReadOnly(t.id); setTabMenu(null); }}>
                  {t.isReadOnly ? <Eye className="w-3.5 h-3.5" /> : <EyeOff className="w-3.5 h-3.5" />}
                  {t.isReadOnly ? 'Make Tab Editable' : 'Make Tab Read-Only'}
                </button>
              </>)}

              {hasFile && (<>
                {sep}
                <button type="button" role="menuitem" className={itemCls} onClick={() => { void navigator.clipboard.writeText(t.filePath); setTabMenu(null); }}>
                  <Copy className="w-3.5 h-3.5" /> Copy Path
                </button>
                <button type="button" role="menuitem" className={itemCls} onClick={() => {
                  const rel = t.filePath.replace(activeWorkspacePath, '').replace(/^\/+/, '');
                  void navigator.clipboard.writeText(rel);
                  setTabMenu(null);
                }}>
                  <Copy className="w-3.5 h-3.5" /> Copy Relative Path
                </button>
              </>)}

              {hasFile && (<>
                {sep}
                <button type="button" role="menuitem" className={itemCls} onClick={() => { void ApiBridge.openInFinder(t.filePath); setTabMenu(null); }}>
                  <ExternalLink className="w-3.5 h-3.5" /> Reveal in Finder
                </button>
              </>)}

              {sep}
              <button type="button" role="menuitem" className={itemCls} onClick={() => { toggleTabPin(t.id); setTabMenu(null); }}>
                {t.isPinned ? <PinOff className="w-3.5 h-3.5" /> : <Pin className="w-3.5 h-3.5" />}
                {t.isPinned ? 'Unpin Tab' : 'Pin Tab'}
              </button>
              <button type="button" role="menuitem" className={itemCls} onClick={() => { setActiveActivity('explorer'); setIsLeftSidebarOpen(true); setTabMenu(null); }}>
                <PanelLeft className="w-3.5 h-3.5" /> Reveal In Project Panel
              </button>
              <button type="button" role="menuitem" className={itemCls} onClick={() => { void openTerminalTab(undefined, dir || ''); setTabMenu(null); }}>
                <TerminalIcon className="w-3.5 h-3.5" /> Open in Terminal
              </button>
            </div>
          </div>
        );
      })()}

    </div>
  );
};
