import { useEffect, useRef, useState, useCallback } from "react";
import { EditorView, keymap, placeholder, lineNumbers, Decoration, DecorationSet, ViewPlugin, ViewUpdate, WidgetType, hoverTooltip, Tooltip } from "@codemirror/view";
import { EditorState, StateField, StateEffect, RangeSetBuilder } from "@codemirror/state";
import { defaultKeymap, history, historyKeymap } from "@codemirror/commands";
import {
  StreamLanguage,
  indentOnInput,
  bracketMatching,
  foldGutter,
  indentUnit,
} from "@codemirror/language";
import { searchKeymap, search } from "@codemirror/search";
import { forgeTheme, forgeHighlight } from "./forge-theme";
import { getZoom, setZoom, onZoomChange } from "../lib/zoom";
import { go } from "@codemirror/lang-go";
import { javascript } from "@codemirror/lang-javascript";
import { rust } from "@codemirror/lang-rust";
import { python } from "@codemirror/lang-python";
import { markdown } from "@codemirror/lang-markdown";
import { json } from "@codemirror/lang-json";
import { java as javaLang } from "@codemirror/lang-java";
import { cpp } from "@codemirror/lang-cpp";
import { autocompletion, closeBrackets, closeBracketsKeymap } from "@codemirror/autocomplete";
import { shell } from "@codemirror/legacy-modes/mode/shell";
import { swift } from "@codemirror/legacy-modes/mode/swift";
import { csharp, kotlin, c as clikeC } from "@codemirror/legacy-modes/mode/clike";
import { cn } from "../lib/utils";
import { GetFileDiff, GetRepoRoot, GetRelPath, StageDiffHunk } from "../../wailsjs/go/main/App";
import { git } from "../../wailsjs/go/models";

interface CodeEditorProps {
  value: string;
  path: string;
  onChange?: (value: string) => void;
  onSave?: () => void;
}

//test

// Git decoration styling
const gitAddedTheme = EditorView.theme({
  ".cm-gitAdded": { backgroundColor: "rgba(34,197,94,.15)" },
  ".cm-gitAddedGutter": {
    backgroundColor: "rgba(34,197,94,.15)",
    borderLeft: "3px solid #22C55E",
    boxSizing: "border-box",
  },
  ".cm-gitModified": { backgroundColor: "rgba(59,130,246,.15)" },
  ".cm-gitModifiedGutter": {
    backgroundColor: "rgba(59,130,246,.15)",
    borderLeft: "3px solid #3B82F6",
    boxSizing: "border-box",
  },
  ".cm-gitDeleted": { backgroundColor: "rgba(239,68,68,.15)" },
  ".cm-gitDeletedGutter": {
    backgroundColor: "rgba(239,68,68,.15)",
    borderLeft: "3px solid #EF4444",
    boxSizing: "border-box",
  },
  ".cm-gitDeletedLine": {
    backgroundColor: "rgba(239,68,68,.15)",
    textDecoration: "line-through",
    opacity: 0.5,
  },
});

function detectLanguage(path: string) {
  const ext = path.split(".").pop()?.toLowerCase();
  switch (ext) {
    case "go": return go();
    case "ts": case "tsx": return javascript({ typescript: true, jsx: true });
    case "js": case "jsx": case "mjs": case "cjs": return javascript({ jsx: ext === "jsx" });
    case "json": case "jsonc": case "json5": return json();
    case "rs": return rust();
    case "py": return python();
    case "md": case "markdown": return markdown();
    case "java": return javaLang();
    case "c": case "h": return StreamLanguage.define(clikeC);
    case "cpp": case "cc": case "cxx": case "hpp": case "hxx": return cpp();
    case "cs": return StreamLanguage.define(csharp);
    case "kt": case "kotlin": case "kts": return StreamLanguage.define(kotlin);
    case "swift": return StreamLanguage.define(swift);
    case "sh": case "bash": case "zsh": return StreamLanguage.define(shell);
    default:
      if (path.endsWith("Makefile") || path.endsWith("Dockerfile")) return StreamLanguage.define(shell);
      return [];
  }
}

export function CodeEditor({ value, path, onChange, onSave }: CodeEditorProps) {
  const editorRef = useRef<HTMLDivElement>(null);
  const viewRef = useRef<EditorView | null>(null);
  const onChangeRef = useRef(onChange);
  const onSaveRef = useRef(onSave);
  const suppressChangeRef = useRef(false);
  const zoomRef = useRef<HTMLDivElement>(null);
  const [diffData, setDiffData] = useState<git.FileDiff | null>(null);
  const [peekHunk, setPeekHunk] = useState<git.DiffHunk | null>(null);
  const [loadingDiff, setLoadingDiff] = useState(false);

  onChangeRef.current = onChange;
  onSaveRef.current = onSave;

  // Load diff data
  useEffect(() => {
    let cancelled = false;
    async function loadDiff() {
      setLoadingDiff(true);
      setDiffData(null);
      setPeekHunk(null);
      try {
        const repoPath = await GetRepoRoot(path);
        if (!repoPath) { setLoadingDiff(false); return; }
        const relPath = await GetRelPath(path);
        const fd = await GetFileDiff(repoPath, relPath);
        if (!cancelled && fd && fd.hunks && fd.hunks.length > 0) {
          setDiffData(fd);
        }
      } catch { }
      if (!cancelled) setLoadingDiff(false);
    }
    loadDiff();
    return () => { cancelled = true; };
  }, [path]);

  // Zoom
  useEffect(() => {
    const el = zoomRef.current;
    if (!el) return;
    el.style.zoom = String(getZoom());
    const unsub = onZoomChange(() => { if (el) el.style.zoom = String(getZoom()); });
    const handler = (e: KeyboardEvent) => {
      if (!e.metaKey && !e.ctrlKey) return;
      if (e.key === "=" || e.key === "+") { e.preventDefault(); setZoom(getZoom() + 0.1); }
      if (e.key === "-") { e.preventDefault(); setZoom(getZoom() - 0.1); }
      if (e.key === "0") { e.preventDefault(); setZoom(1); }
    };
    document.addEventListener("keydown", handler);
    return () => { document.removeEventListener("keydown", handler); unsub(); };
  }, []);

  // Build line decorations from diff
  const gitDecorations = useCallback((state: EditorState) => {
    const builder = new RangeSetBuilder<Decoration>();
    if (!diffData) return builder.finish();

    const lineMap: Record<number, { type: string }> = {};
    for (const hunk of diffData.hunks) {
      for (const line of hunk.lines) {
        if (line.newLine > 0 && line.type !== "deleted") {
          lineMap[line.newLine] = { type: line.type };
        }
      }
    }

    for (let i = 1; i <= state.doc.lines; i++) {
      const line = state.doc.line(i);
      const info = lineMap[i];
      if (!info) continue;

      const gutterClass = info.type === "added" ? "cm-gitAddedGutter" :
        info.type === "modified" ? "cm-gitModifiedGutter" : "";

      const lineClass = info.type === "added" ? "cm-gitAdded" :
        info.type === "modified" ? "cm-gitModified" :
        info.type === "deleted" ? "cm-gitDeletedLine" : "";

      builder.add(
        line.from,
        line.from,
        Decoration.line({ class: lineClass })
      );

      builder.add(
        line.from,
        line.from,
        Decoration.widget({
          widget: new GutterWidget(gutterClass),
          side: -1,
          block: false,
        })
      );
    }
    return builder.finish();
  }, [diffData]);

  // Initialize editor
  useEffect(() => {
    if (!editorRef.current) return;

    const langExtensions = detectLanguage(path);

    const updateListener = EditorView.updateListener.of((update) => {
      if (update.docChanged && !suppressChangeRef.current) {
        const doc = update.state.doc.toString();
        onChangeRef.current?.(doc);
      }
    });

    const gitDecorationsField = StateField.define<DecorationSet>({
      create(state) { return Decoration.none; },
      update(decorations, tr) {
        if (tr.docChanged || tr.selection || !diffData) return decorations;
        const builder = new RangeSetBuilder<Decoration>();
        const lineMap: Record<number, { type: string }> = {};
        for (const hunk of diffData.hunks) {
          for (const line of hunk.lines) {
            if (line.newLine > 0 && line.type !== "deleted") {
              lineMap[line.newLine] = { type: line.type };
            }
          }
        }
        for (let i = 1; i <= tr.state.doc.lines; i++) {
          const line = tr.state.doc.line(i);
          const info = lineMap[i];
          if (!info) continue;
          const lineClass = info.type === "added" ? "cm-gitAdded" :
            info.type === "modified" ? "cm-gitModified" : "";
          if (lineClass) {
            builder.add(line.from, line.from, Decoration.line({ class: lineClass }));
          }
        }
        return builder.finish();
      },
      provide: (f) => EditorView.decorations.from(f),
    });

    const saveKeymapBinding = keymap.of([
      {
        key: "Mod-s",
        run: () => { onSaveRef.current?.(); return true; },
      },
    ]);

    const state = EditorState.create({
      doc: value,
      extensions: [
        lineNumbers(),
        foldGutter(),
        history(),
        forgeTheme,
        forgeHighlight,
        gitAddedTheme,
        gitDecorationsField,
        langExtensions,
        updateListener,
        saveKeymapBinding,
        keymap.of([...defaultKeymap, ...historyKeymap, ...closeBracketsKeymap, ...searchKeymap]),
        bracketMatching(),
        closeBrackets(),
        autocompletion(),
        indentOnInput(),
        indentUnit.of("  "),
        EditorView.lineWrapping,
        placeholder("Start typing..."),
        search({ top: true }),
        hoverTooltip((view, pos) => {
          const line = view.state.doc.lineAt(pos);
          if (!diffData) return null;
          for (const hunk of diffData.hunks) {
            for (const dl of hunk.lines) {
              if (dl.newLine === line.number && dl.type !== "context") {
                return {
                  pos: line.from,
                  end: line.to,
                  above: true,
                  create() {
                    const dom = document.createElement("div");
                    dom.className = "px-2 py-1 text-xs font-mono";
                    dom.style.cssText = "background:#1a1a2e; border:1px solid #333; border-radius:4px; max-width:400px;";
                    dom.textContent = dl.type === "added" ? "Added" : dl.type === "deleted" ? "Removed" : "Modified";
                    return { dom };
                  },
                } as Tooltip;
              }
            }
          }
          return null;
        }),
      ],
    });

    const view = new EditorView({ state, parent: editorRef.current });
    viewRef.current = view;

    return () => {
      view.destroy();
      viewRef.current = null;
    };
  }, [path, diffData]);

  // Sync external value changes
  useEffect(() => {
    const view = viewRef.current;
    if (!view) return;
    const currentDoc = view.state.doc.toString();
    if (value !== currentDoc) {
      suppressChangeRef.current = true;
      view.dispatch({
        changes: { from: 0, to: currentDoc.length, insert: value },
      });
      suppressChangeRef.current = false;
    }
  }, [value]);

  const handlePeekClose = () => setPeekHunk(null);

  return (
    <div ref={zoomRef} style={{ height: "100%", position: "relative" }}>
      <div ref={editorRef} className="flex-1 overflow-hidden" style={{ height: "100%" }} />

      {/* Diff Peek overlay */}
      {peekHunk && (
        <div className="absolute bottom-0 left-0 right-0 z-40 border-t bg-[#0d1117] max-h-48 overflow-auto">
          <div className="flex items-center justify-between px-3 py-1 text-[10px] text-muted-foreground border-b bg-muted/20">
            <span>Diff Peek</span>
            <button className="p-0.5 hover:bg-accent rounded" onClick={handlePeekClose}>
              <span className="text-xs">✕</span>
            </button>
          </div>
          <div className="font-mono text-xs">
            {peekHunk.lines.map((line, i) => (
              <div key={i} className={cn(
                "px-4 py-0.5 leading-5 flex",
                line.type === "added" && "bg-[rgba(34,197,94,.1)] text-green-400",
                line.type === "deleted" && "bg-[rgba(239,68,68,.1)] text-red-400",
                line.type === "context" && "text-gray-400",
              )}>
                <span className="w-8 shrink-0 text-right mr-2 opacity-40">
                  {line.oldLine > 0 ? line.oldLine : ""}
                </span>
                <span className="w-8 shrink-0 text-right mr-2 opacity-40">
                  {line.newLine > 0 ? line.newLine : ""}
                </span>
                <span className="w-4 shrink-0">
                  {line.type === "added" ? "+" : line.type === "deleted" ? "-" : " "}
                </span>
                <span className="flex-1 whitespace-pre">{line.content}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Hunk stage button */}
      {diffData && diffData.hunks.length > 0 && (
        <div className="absolute top-1 right-2 z-40 flex gap-1">
          {diffData.hunks.slice(0, 3).map((_, i) => (
            <button
              key={i}
              className="text-[10px] px-1.5 py-0.5 bg-muted hover:bg-accent rounded"
              onClick={async () => {
                try {
                  const repoPath = await GetRepoRoot(path);
                  const relPath = await GetRelPath(path);
                  await StageDiffHunk(repoPath, relPath, i);
                } catch (e) { console.error(e); }
              }}
              title={`Stage hunk ${i + 1}`}
            >
              H{i + 1}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

class GutterWidget extends WidgetType {
  cls: string;
  constructor(cls: string) { super(); this.cls = cls; }
  eq(other: GutterWidget) { return other.cls === this.cls; }
  toDOM() {
    const span = document.createElement("span");
    span.className = this.cls;
    span.style.cssText = "display:inline-block;width:0;height:100%;";
    return span;
  }
}
