// Global editor hooks shared across feature modules.
//
// These module-level singletons let any surface (terminal output links, git
// panels, agent chat) open files or scroll the active editor without a React
// context. The hook registry survives whether or not an editor surface is
// currently mounted — when none is, the hooks simply no-op.

import { IsDir, ReadFile } from "./wails";
import { useEditorStore, useWorkspaceStore, useWorkspaceTabStore } from "../hooks/store";
import { Decoration, type DecorationSet, EditorView } from "@codemirror/view";
import { StateEffect, StateField } from "@codemirror/state";

// ---------------------------------------------------------------------------
// Open-file lifecycle hook
// ---------------------------------------------------------------------------

let onBeforeOpenFileCallback: (() => void) | null = null;

/** Registers a callback fired before any globalOpenFile/globalOpenDiff (e.g. to switch to the editor mode). */
export function setOnBeforeOpenFile(cb: () => void) {
  onBeforeOpenFileCallback = cb;
}

// ---------------------------------------------------------------------------
// Active editor view registry + scroll-to-line / line highlight
// ---------------------------------------------------------------------------

// Module-level reference to the active editor view, so other modules (e.g.
// format-on-save) can dispatch content into the open editor.
let globalEditorView: EditorView | null = null;

export function setGlobalEditorView(view: EditorView | null) {
  globalEditorView = view;
}

const setLineHighlight = StateEffect.define<{ from: number; to: number } | null>();
const lineHighlightMark = Decoration.mark({ class: "cm-line-highlight" });

/** StateField an editor must include for highlightLine() to show the marker. */
export const lineHighlightField = StateField.define<DecorationSet>({
  create: () => Decoration.none,
  update(deco, tr) {
    deco = deco.map(tr.changes);
    for (const e of tr.effects) {
      if (e.is(setLineHighlight)) {
        deco = e.value
          ? Decoration.set([lineHighlightMark.range(e.value.from, e.value.to)])
          : Decoration.none;
      }
    }
    return deco;
  },
  provide: (f) => EditorView.decorations.from(f),
});

let lineHighlightTimer: ReturnType<typeof setTimeout> | null = null;

/** Highlights a 1-based line in the active editor for ~4s. No-op when the line is out of range or no editor is mounted. */
export function highlightLine(line: number) {
  const v = globalEditorView;
  if (!v || !line || line < 1 || line > v.state.doc.lines) return;
  const info = v.state.doc.line(line);
  v.dispatch({ effects: setLineHighlight.of({ from: info.from, to: info.to }) });
  if (lineHighlightTimer) clearTimeout(lineHighlightTimer);
  lineHighlightTimer = setTimeout(() => {
    v.dispatch({ effects: setLineHighlight.of(null) });
  }, 4000);
}

// Line requested before an editor was mounted (or before its document
// finished loading); the editor consumes it once it can scroll.
let pendingScrollLine: number | null = null;

/** Consumes the pending scroll target, if any. Call after an editor mounts/loads a document. */
export function takePendingScrollLine(): number | null {
  const line = pendingScrollLine;
  pendingScrollLine = null;
  return line;
}

export function scrollEditorToLine(line: number | null) {
  pendingScrollLine = line;
  const v = globalEditorView;
  if (v && line && line > 0 && line <= v.state.doc.lines) {
    const info = v.state.doc.line(line);
    v.dispatch({ selection: { anchor: info.from }, effects: EditorView.scrollIntoView(info.from, { y: "center" }) });
    v.focus();
    pendingScrollLine = null;
    highlightLine(line);
  }
}

// ---------------------------------------------------------------------------
// globalOpenFile — open a file tab from anywhere in the app
// ---------------------------------------------------------------------------

export async function globalOpenFile(rawPath: string, opts?: { content?: string; name?: string; id?: string; line?: number }) {
  if (onBeforeOpenFileCallback) onBeforeOpenFileCallback();

  // Strip line numbers if present e.g. "path/file.go:123:45"
  let lineFromPath: number | undefined = opts?.line;
  let path = rawPath.trim();
  const lineMatch = /:(\d+)(?::\d+)?$/.exec(path);
  if (lineMatch) {
    if (!lineFromPath) lineFromPath = parseInt(lineMatch[1], 10);
    path = path.slice(0, lineMatch.index);
  }

  // Resolve relative paths against current workspace folder if needed
  const workspace = useWorkspaceStore.getState().workspace;
  const rootFolder = workspace?.folders?.[0];
  if (rootFolder && !path.startsWith("/") && !path.startsWith("~")) {
    const cleanRel = path.replace(/^\.\//, "");
    path = `${rootFolder}/${cleanRel}`;
  }

  const name = opts?.name || path.split(/[/\\]/).pop() || "Untitled";
  const ext = name.split(".").pop()?.toLowerCase() || "";

  // Guard: Do not open archive formats or directories in the editor
  const archiveExts = ["zip", "gz", "tar", "tgz", "bz2", "xz", "7z", "rar", "zst"];
  if (archiveExts.includes(ext)) {
    console.warn("Ignoring archive file for editor:", path);
    return;
  }

  try {
    const isDirectory = await IsDir(path);
    if (isDirectory) {
      console.warn("Ignoring directory for editor open:", path);
      return;
    }
  } catch { /* proceed if stat fails or path is synthetic */ }

  // Opening a file should show the file tab, not a browser pane.
  useWorkspaceTabStore.getState().setActiveBrowserTab(null);
  const { files, setFiles, setActiveFileIndex } = useEditorStore.getState();

  const tabId = opts?.id || path;
  const existingIdx = files.findIndex((f) => f.id === tabId);
  if (existingIdx !== -1) {
    setActiveFileIndex(existingIdx);
    if (lineFromPath && lineFromPath > 0) {
      requestAnimationFrame(() => scrollEditorToLine(lineFromPath ?? null));
    }
    return;
  }

  try {
    const isBinary = ["png", "jpg", "jpeg", "gif", "pdf", "ico", "svg"].includes(ext);

    let content = "";
    if (!isBinary) {
      content = opts?.content !== undefined ? opts.content : await ReadFile(path);
    }

    const newFile = {
      id: tabId,
      name,
      path,
      type: "file" as "file",
      content,
      savedContent: content,
      modified: false,
    };

    setFiles((prev) => [...prev, newFile]);
    // Read fresh state after setFiles to get the correct new index
    setActiveFileIndex(useEditorStore.getState().files.length - 1);
    if (lineFromPath && lineFromPath > 0) {
      requestAnimationFrame(() => scrollEditorToLine(lineFromPath ?? null));
    }
  } catch (err) {
    console.error("Failed to open file:", err);
  }
}
