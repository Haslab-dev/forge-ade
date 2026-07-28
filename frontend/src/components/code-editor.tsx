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
import { searchKeymap, search } from "@codemirror/search";
import { forgeTheme, forgeHighlight } from "./forge-theme";
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
    case "json":
    case "jsonc":
    case "json5":
      return json();
    case "rs":
      return rust();
    case "py":
      return python();
    case "md":
    case "markdown":
      return markdown();
    case "java":
      return javaLang();
    case "c":
    case "h":
      return StreamLanguage.define(clikeC);
    case "cpp":
    case "cc":
    case "cxx":
    case "hpp":
    case "hxx":
      return cpp();
    case "cs":
      return StreamLanguage.define(csharp);
    case "kt":
    case "kotlin":
    case "kts":
      return StreamLanguage.define(kotlin);
    case "swift":
      return StreamLanguage.define(swift);
    case "sh":
    case "bash":
    case "zsh":
      return StreamLanguage.define(shell);
    default:
      if (path.endsWith("Makefile") || path.endsWith("Dockerfile"))
        return StreamLanguage.define(shell);
      return [];
  }
}

export function CodeEditor({ value, path, onChange, onSave }: CodeEditorProps) {
  const editorRef = useRef<HTMLDivElement>(null);
  const viewRef = useRef<EditorView | null>(null);
  const onChangeRef = useRef(onChange);
  const onSaveRef = useRef(onSave);
  const suppressChangeRef = useRef(false);
  

  onChangeRef.current = onChange;
  onSaveRef.current = onSave;

  // Initialize editor once
  useEffect(() => {
    if (!editorRef.current) return;
    

    const langExtensions = detectLanguage(path);

    const updateListener = EditorView.updateListener.of((update) => {
      if (update.docChanged && !suppressChangeRef.current) {
        const doc = update.state.doc.toString();
        
        onChangeRef.current?.(doc);
        
      }
    });

    const saveKeymapBinding = keymap.of([
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
        lineNumbers(),
        foldGutter(),
        history(),
        forgeTheme,
        forgeHighlight,
        langExtensions,
        updateListener,
        saveKeymapBinding,
        
        keymap.of([
          ...defaultKeymap,
          ...historyKeymap,
          ...closeBracketsKeymap,
          ...searchKeymap,
        ]),
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

    const view = new EditorView({
      state,
      parent: editorRef.current,
    });

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
