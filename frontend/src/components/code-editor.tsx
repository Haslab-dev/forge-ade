import { useEffect, useRef, useCallback } from "react";
import { EditorView, keymap, placeholder } from "@codemirror/view";
import { EditorState } from "@codemirror/state";
import { defaultKeymap, history, historyKeymap } from "@codemirror/commands";
import { oneDark } from "@codemirror/theme-one-dark";
import { syntaxHighlighting, defaultHighlightStyle } from "@codemirror/language";
import { go } from "@codemirror/lang-go";
import { javascript } from "@codemirror/lang-javascript";
import { rust } from "@codemirror/lang-rust";
import { python } from "@codemirror/lang-python";
import { autocompletion, closeBrackets, closeBracketsKeymap } from "@codemirror/autocomplete";

interface CodeEditorProps {
  value: string;
  path: string;
  onChange?: (value: string) => void;
  onSave?: () => void;
}

function detectLanguage(path: string) {
  const ext = path.split(".").pop()?.toLowerCase();
  switch (ext) {
    case "go":
      return go();
    case "ts":
    case "tsx":
      return javascript({ typescript: true, jsx: true });
    case "js":
    case "jsx":
    case "mjs":
    case "cjs":
      return javascript({ jsx: ext === "jsx" });
    case "rs":
      return rust();
    case "py":
      return python();
    default:
      // Try to detect by filename
      if (path.endsWith("Makefile") || path.endsWith("Dockerfile")) return [];
      // For other files, use plain text (no language plugin)
      return [];
  }
}

export function CodeEditor({ value, path, onChange, onSave }: CodeEditorProps) {
  const editorRef = useRef<HTMLDivElement>(null);
  const viewRef = useRef<EditorView | null>(null);
  const onChangeRef = useRef(onChange);
  const onSaveRef = useRef(onSave);
  const prevValueRef = useRef(value);
  const suppressChangeRef = useRef(false);

  // Keep callback refs up to date
  onChangeRef.current = onChange;
  onSaveRef.current = onSave;

  // Initialize editor once
  useEffect(() => {
    if (!editorRef.current) return;

    const ext = path.split(".").pop()?.toLowerCase();
    const langExtensions = detectLanguage(path);

    const updateListener = EditorView.updateListener.of((update) => {
      if (update.docChanged && !suppressChangeRef.current) {
        const doc = update.state.doc.toString();
        prevValueRef.current = doc;
        onChangeRef.current?.(doc);
      }
    });

    // Save on Cmd+S / Ctrl+S
    const saveKeymap = keymap.of([
      {
        key: "Mod-s",
        run: () => {
          onSaveRef.current?.();
          return true;
        },
      },
    ]);

    const state = EditorState.create({
      doc: value,
      extensions: [
        history(),
        oneDark,
        syntaxHighlighting(defaultHighlightStyle, { fallback: true }),
        langExtensions,
        updateListener,
        saveKeymap,
        keymap.of([...defaultKeymap, ...historyKeymap, ...closeBracketsKeymap]),
        closeBrackets(),
        autocompletion(),
        EditorView.lineWrapping,
        placeholder("Start typing..."),
        EditorView.theme({
          "&": { height: "100%" },
          ".cm-scroller": { overflow: "auto" },
          ".cm-content": { fontFamily: "'JetBrains Mono', 'Fira Code', 'Cascadia Code', monospace", fontSize: "13px", padding: "16px" },
          ".cm-gutters": { borderRight: "1px solid hsl(var(--border))", background: "transparent" },
          ".cm-activeLineGutter": { background: "hsl(var(--accent))" },
          ".cm-activeLine": { background: "hsl(var(--accent) / 0.3)" },
          ".cm-selectionBackground": { background: "hsl(var(--accent))" },
          "&.cm-focused .cm-selectionBackground": { background: "hsl(var(--accent) / 0.7)" },
          ".cm-cursor": { borderLeftColor: "hsl(var(--foreground))" },
          ".cm-matchingBracket": { background: "hsl(var(--accent))", outline: "1px solid hsl(var(--border))" },
        }),
      ],
    });

    const view = new EditorView({
      state,
      parent: editorRef.current,
    });

    viewRef.current = view;

    return () => {
      view.destroy();
      viewRef.current = null;
    };
  }, [path]); // only recreate when path changes (different language)

  // Sync external value changes (e.g., file save, tab switch, external update)
  useEffect(() => {
    const view = viewRef.current;
    if (!view) return;

    const currentDoc = view.state.doc.toString();
    if (value !== currentDoc) {
      suppressChangeRef.current = true;
      view.dispatch({
        changes: {
          from: 0,
          to: currentDoc.length,
          insert: value,
        },
      });
      suppressChangeRef.current = false;
    }
  }, [value]);

  return (
    <div
      ref={editorRef}
      className="flex-1 overflow-hidden"
      style={{ height: "100%" }}
    />
  );
}
