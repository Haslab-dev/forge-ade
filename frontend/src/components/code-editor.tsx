import { useEffect, useRef } from "react";
import { EditorView, keymap, placeholder, lineNumbers } from "@codemirror/view";
import { EditorState } from "@codemirror/state";
import { defaultKeymap, history, historyKeymap } from "@codemirror/commands";
import {
  StreamLanguage,
  indentOnInput,
  bracketMatching,
  foldGutter,
  indentUnit,
} from "@codemirror/language";
import { searchKeymap, search, openSearchPanel } from "@codemirror/search";
import { forgeTheme, forgeHighlight } from "./forge-theme";
import { getZoom, setZoom, onZoomChange } from "../lib/zoom";
import { go } from "@codemirror/lang-go";
import { javascript } from "@codemirror/lang-javascript";
import { rust } from "@codemirror/lang-rust";
import { python } from "@codemirror/lang-python";
import { markdown } from "@codemirror/lang-markdown";
import { html } from "@codemirror/lang-html";
import { vue } from "@codemirror/lang-vue";
import { php } from "@codemirror/lang-php";
import { json } from "@codemirror/lang-json";
import { java as javaLang } from "@codemirror/lang-java";
import { cpp } from "@codemirror/lang-cpp";
import { autocompletion, closeBrackets, closeBracketsKeymap } from "@codemirror/autocomplete";
import { shell } from "@codemirror/legacy-modes/mode/shell";
import { swift } from "@codemirror/legacy-modes/mode/swift";
import { csharp, kotlin, c as clikeC } from "@codemirror/legacy-modes/mode/clike";
import { groovy } from "@codemirror/legacy-modes/mode/groovy";
import { properties } from "@codemirror/legacy-modes/mode/properties";
import { prolog } from "codemirror-lang-prolog";

interface CodeEditorProps {
  value: string;
  path: string;
  scrollToLine?: number;
  onChange?: (value: string) => void;
  onSave?: () => void;
}

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
    case "html": case "htm": case "astro": return html();
    case "vue": return vue();
    case "php": case "phtml": case "php3": case "php4": case "php5": case "phps": return php();
    case "java": return javaLang();
    case "mod":
    case "sum":
    case "c": case "h": return StreamLanguage.define(clikeC);
    case "cpp": case "cc": case "cxx": case "hpp": case "hxx": return cpp();
    case "cs": return StreamLanguage.define(csharp);
    case "kt": case "kotlin": case "kts": return StreamLanguage.define(kotlin);
    case "swift": return StreamLanguage.define(swift);
    case "gradle": return StreamLanguage.define(groovy);
    case "bat": return StreamLanguage.define(shell);
    case "properties": return StreamLanguage.define(properties);
    case "keystore": return StreamLanguage.define(properties);
    case "pro": return prolog();
    case "sh": case "bash": case "zsh": return StreamLanguage.define(shell);
    default:
      if (path.endsWith("Makefile") || path.endsWith("Dockerfile")) return StreamLanguage.define(shell);
      return [];
  }
}

export function CodeEditor({ value, path, scrollToLine, onChange, onSave }: CodeEditorProps) {
  const editorRef = useRef<HTMLDivElement>(null);
  const viewRef = useRef<EditorView | null>(null);
  const onChangeRef = useRef(onChange);
  const onSaveRef = useRef(onSave);
  const suppressChangeRef = useRef(false);
  const zoomRef = useRef<HTMLDivElement>(null);

  onChangeRef.current = onChange;
  onSaveRef.current = onSave;

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

    const searchBindings = keymap.of([
      { key: "Mod-s", run: () => { onSaveRef.current?.(); return true; } },
      { key: "Mod-p", run: openSearchPanel },
      { key: "Mod-f", run: openSearchPanel },
    ]);

    const state = EditorState.create({
      doc: value,
      extensions: [
        lineNumbers(),
        foldGutter(),
        history(),
        forgeTheme,
        forgeHighlight,
        langExtensions,
        updateListener,
        searchBindings,
        keymap.of([...defaultKeymap, ...historyKeymap, ...closeBracketsKeymap, ...searchKeymap]),
        bracketMatching(),
        closeBrackets(),
        autocompletion(),
        indentOnInput(),
        indentUnit.of("  "),
        EditorView.lineWrapping,
        placeholder("Start typing..."),
        search({ top: true }),
      ],
    });

    const view = new EditorView({ state, parent: editorRef.current });
    viewRef.current = view;

    return () => {
      view.destroy();
      viewRef.current = null;
    };
  }, [path]);

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

  // Scroll and center target line when scrollToLine changes
  useEffect(() => {
    const view = viewRef.current;
    if (!view || !scrollToLine || scrollToLine <= 0) return;
    try {
      const lineCount = view.state.doc.lines;
      const targetLine = Math.min(scrollToLine, lineCount);
      const lineObj = view.state.doc.line(targetLine);
      view.dispatch({
        selection: { anchor: lineObj.from, head: lineObj.from },
        effects: EditorView.scrollIntoView(lineObj.from, { y: "center" }),
      });
      view.focus();
    } catch { /* ignore */ }
  }, [scrollToLine, path]);

  return (
    <div ref={zoomRef} style={{ height: "100%", position: "relative" }}>
      <div ref={editorRef} className="flex-1 overflow-hidden" style={{ height: "100%" }} />
    </div>
  );
}
