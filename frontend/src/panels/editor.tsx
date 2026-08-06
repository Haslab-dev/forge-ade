import React, { useEffect, useRef, useState, useMemo, useCallback } from "react";
import { useEditorStore, useUIStore, useWorkspaceTabStore, useWorkspaceStore } from "../hooks/store";
import { EditorFile } from "../types";
import { getFileIcon } from "../lib/file-icons";
import { useToast } from "../lib/toast";
import { cn } from "../lib/utils";
import { ReadFile, ReadFileBase64, WriteFile, ListAgentSessions, EventsOn, CheckSyntax, FormatCode, GetGitFileContentAtCommit, GetGitConflictStageContent, GitResolveConflict, GetGitFileDiffHunks, GetGitFileDiff, RevertGitHunk, GitStage, GetClipboardFiles, CreateShell, CreateAgentSession } from "../lib/wails";
import { TerminalView } from "../components/terminal-view";
import { AgentChatPanel } from "../components/agent-panel";
import { DiffView } from "../components/diff-view";
import { BrowserPanel } from "./browser-panel";
import {
  X,
  Copy,
  Eye,
  FileCode2,
  Image as ImageIcon,
  FileText as FileTextIcon,
  Globe,
  Settings,
  Cpu,
  ChevronDown,
  ChevronUp,
  ChevronRight,
  Brain,
  Shield,
  Check,
  Send,
  Search,
  ArrowDown,
  ArrowUp,
  Zap,
  FileDiff,
  FileText,
  History,
  CirclePlus,
  Undo2,
  Plus,
  Maximize,
  Columns2,
  LayoutGrid,
  Terminal,
  Bot,
  Globe2,
} from "lucide-react";
import { EditorState, Compartment, Extension, RangeSetBuilder, Prec, StateEffect, StateField } from "@codemirror/state";
import { EditorView, keymap, lineNumbers, highlightActiveLine, highlightActiveLineGutter, gutter, GutterMarker, Decoration, DecorationSet, ViewPlugin, ViewUpdate } from "@codemirror/view";
import { defaultKeymap, history, historyKeymap, indentMore, indentLess, indentWithTab, toggleComment, toggleBlockComment } from "@codemirror/commands";
import { search, searchKeymap, openSearchPanel, setSearchQuery, getSearchQuery, highlightSelectionMatches } from "@codemirror/search";
import { bracketMatching, syntaxTree } from "@codemirror/language";
import { closeBrackets, closeBracketsKeymap, autocompletion, Completion, CompletionContext } from "@codemirror/autocomplete";
import { linter, Diagnostic } from "@codemirror/lint";
import { GetCompletion, GetMembers } from "../../wailsjs/go/main/App";
import { javascript } from "@codemirror/lang-javascript";
import { go } from "@codemirror/lang-go";
import { python } from "@codemirror/lang-python";
import { rust } from "@codemirror/lang-rust";
import { json } from "@codemirror/lang-json";
import { html } from "@codemirror/lang-html";
import { markdown } from "@codemirror/lang-markdown";
import { cpp } from "@codemirror/lang-cpp";
import { sql } from "@codemirror/lang-sql";
import { php } from "@codemirror/lang-php";
import { css } from "@codemirror/lang-css";
import { less } from "@codemirror/lang-less";
import { sass } from "@codemirror/lang-sass";
import { oneDark } from "@codemirror/theme-one-dark";
import { marked } from "marked";

let onBeforeOpenFileCallback: (() => void) | null = null;
export function setOnBeforeOpenFile(cb: () => void) {
  onBeforeOpenFileCallback = cb;
}

// Module-level reference to the active editor view, so other modules (e.g.
// format-on-save from App.tsx) can dispatch content into the open editor.
let globalEditorView: EditorView | null = null;
export function setGlobalEditorView(view: EditorView | null) {
  globalEditorView = view;
}
// The freshest live document per open file id. Kept in sync on every keystroke
// (not just the debounce) so save can read the true current content.
const globalLiveContent = new Map<string, string>();
export function setGlobalLiveContent(id: string, content: string) {
  globalLiveContent.set(id, content);
}
export function getGlobalLiveContent(id: string): string | undefined {
  return globalLiveContent.get(id);
}
export function applyFormattedContent(content: string) {
  const view = globalEditorView;
  if (!view || content === view.state.doc.toString()) return;
  // Preserve scroll + cursor across full-doc replace (prettier output).
  const scrollTop = view.scrollDOM.scrollTop;
  const main = view.state.selection.main;
  view.dispatch({
    changes: { from: 0, to: view.state.doc.length, insert: content },
    selection: { anchor: main.anchor, head: main.head },
    scrollIntoView: false,
  });
  view.requestMeasure();
  requestAnimationFrame(() => {
    view.scrollDOM.scrollTop = scrollTop;
  });
}

// ---------------------------------------------------------------------------
// VS Code-style diff gutter: per-line change markers in the gutter. Clicking a
// marker opens a small popover with the hunk preview and actions (revert, stage,
// prev/next change, close).
// ---------------------------------------------------------------------------

export type DiffLineType = "added" | "removed" | "modified";

// new-file line number -> marker type, populated by refreshFileDiff().
let diffLineMap = new Map<number, DiffLineType>();

// Module-level diff compartment so the gutter can be reconfigured without
// recreating the whole editor state.
const diffCompartment = new Compartment();

// ---------------------------------------------------------------------------
// Workspace symbol completion (FWI / RFC-0001). Queries the Go index store.
const workspaceCompletion = (): Extension => autocompletion({
  // debounce the Wails RPC: 150ms instead of the default 100ms
  activateOnTypingDelay: 150,
  override: [
    async (ctx: CompletionContext) => {
      // Member access `obj.` / `obj.pre`: resolve via instance binding.
      const member = memberSource(ctx);
      if (member) {
        const syms = await GetMembers(member.obj, getOpenFilePath() || "");
        const opts = syms
          .filter((s) => s.Name.startsWith(member.pref))
          .map((s) => ({
            label: s.Name,
            type: symbolKindType(s.Kind),
            detail: `${s.File.split("/").pop()}:${s.Line}`,
          }));
        return { from: member.from, options: opts };
      }
      const word = ctx.matchBefore(/[\w$]+/);
      if (!word || (word.from === word.to && !ctx.explicit)) return null;
      // skip tiny prefixes — they return hundreds of matches per keystroke
      if (word.text.length < 2 && !ctx.explicit) return null;
      const syms = await GetCompletion(word.text, getOpenFilePath() || "");
      return {
        from: word.from,
        options: syms.map((s) => {
          const opt: Completion = {
            label: s.Name,
            type: symbolKindType(s.Kind),
            // dependency → auto-import hint; workspace → file:line
            detail: s.Module
              ? `import { ${s.Name} } from "${s.Module}"`
              : `${s.File.split("/").pop()}:${s.Line}`,
          };
          if (s.Module) {
            // custom apply: insert the name, then add the import statement.
            // Wrapped so a failure never blocks the name insertion.
            opt.apply = (view: EditorView, comp: Completion, from: number, to: number) => {
              try {
                view.dispatch({ changes: { from, to, insert: comp.label } });
                ensureImport(view, comp.label, s.Module);
              } catch (e) {
                console.error("[auto-import] failed for", comp.label, e);
              }
            };
          }
          return opt;
        }),
      };
    },
  ],
});

// memberSource detects `obj.` or `obj.pref` at the cursor and returns the
// object name, typed prefix, and completion start position.
function memberSource(ctx: CompletionContext): { obj: string; pref: string; from: number } | null {
  const before = ctx.state.sliceDoc(Math.max(0, ctx.pos - 200), ctx.pos);
  const m = /([\w$]+(?:\.[\w$]+|\[\d+\])*)\.([\w$]*)$/.exec(before);
  if (!m) return null;
  return { obj: m[1], pref: m[2], from: ctx.pos - m[2].length };
}

// ensureImport inserts `import { name } from "module"` at the top of the
// current file unless that module is already imported. Part of the review's
// Auto Import: picking a dependency completion adds its import statement.
function ensureImport(view: EditorView, name: string, module: string): void {
  const doc = view.state.doc.toString();
  // 1. name already imported in any form (named, default, namespace) → done
  const nameRe = new RegExp(`import\\s+[^;]*\\b${escapeRegExp(name)}\\b[^;]*;`, "g");
  if (nameRe.test(doc)) {
    return;
  }
  // 2. module already imported → merge into its `{ ... }` braces.
  // The braces may be several lines above `from` (multi-line imports).
  const modRe = new RegExp(`from\\s+["']${escapeRegExp(module)}["']`);
  const m = modRe.exec(doc);
  if (m) {
    const regionStart = Math.max(0, m.index - 300);
    const brace = /\{([^}]*)\}\s*$/.exec(doc.slice(regionStart, m.index));
    if (brace) {
      const names = brace[1].split(",").map((s) => s.trim()).filter((s) => s && !s.startsWith("type "));
      if (!names.includes(name)) {
        names.push(name);
        const braceFrom = regionStart + brace.index;
        view.dispatch({
          changes: {
            from: braceFrom + 1,
            to: braceFrom + 1 + brace[1].length,
            insert: " " + names.join(", ") + " ",
          },
        });
      }
      return;
    }
    // matched `from "mod"` but no braces (default/namespace import, or a
    // string) → fall through and add a proper named import.
  }
  // 3. new import — after leading comments, grouped after the last import
  let pos = 0;
  for (const line of doc.split("\n")) {
    if (/^\s*(\/\/|\/\*|#!)/.test(line.replace(/^\uFEFF/, ""))) {
      pos += line.length + 1;
    } else {
      break;
    }
  }
  let acc = 0;
  for (const line of doc.split("\n")) {
    if (/^\s*import(\s|\{)/.test(line)) {
      pos = acc + line.length + 1;
    }
    acc += line.length + 1;
  }
  view.dispatch({
    changes: { from: pos, insert: `import { ${name} } from "${module}";\n` },
  });
}

function escapeRegExp(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

// Map index.SymbolKind to a CodeMirror completion type.
function symbolKindType(kind: number): string {
  switch (kind) {
    case 0:
      return "function";
    case 1:
    case 4:
      return "class";
    case 2:
      return "interface";
    case 3:
      return "enum";
    case 5:
      return "type";
    case 7:
      return "constant";
    case 8:
      return "method";
    case 9:
      return "namespace";
    default:
      return "variable";
  }
}

// ---------------------------------------------------------------------------
// Line highlight (jump-to-line from search / path:line). Highlights the target
// line for a few seconds after navigating.
const setLineHighlight = StateEffect.define<{ from: number; to: number } | null>();
const lineHighlightMark = Decoration.mark({ class: "cm-line-highlight" });
const lineHighlightField = StateField.define<DecorationSet>({
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

// ---------------------------------------------------------------------------
// Rainbow brackets: colorize nested bracket pairs by depth using the syntax
// tree, so brackets inside strings/comments/template literals are skipped.
// ---------------------------------------------------------------------------

const RAINBOW_COLORS = [
  "#d91878",
  "#39bae6",
  "#d9e066",
  "#7bc618",
  "#ff9900",
  "#d63bff",
];
const rainbowMarks: Decoration[] = RAINBOW_COLORS.map((color) =>
  Decoration.mark({ attributes: { style: `color: ${color}` } })
);
const openers = new Set(["(", "[", "{"]);
const closers = new Set([")", "]", "}"]);
const pairOf: Record<string, string> = { ")": "(", "]": "[", "}": "{" };

// Lezer literal token names are quoted (e.g. "("); some grammars expose
// unquoted single-char names. Normalize both to a bare bracket char.
function bracketChar(name: string): string | null {
  if (name.length === 1 && (openers.has(name) || closers.has(name))) return name;
  if (name.length === 3 && name[0] === '"' && name[2] === '"') {
    const c = name[1];
    if (openers.has(c) || closers.has(c)) return c;
  }
  return null;
}

// Recomputes rainbow brackets for the visible viewport only. Scanning the
// whole document per keystroke stalls typing on large files (1800+ lines).
// Depth starts at 0 at the viewport edge — colors shift slightly when the
// window scrolls, which is a fair trade for responsive typing.
function computeRainbowBrackets(view: EditorView): DecorationSet {
  const state = view.state;
  const builder = new RangeSetBuilder<Decoration>();
  const stack: string[] = [];
  const { from, to } = view.viewport;
  const cur = syntaxTree(state).cursor();
  if (!cur.moveTo(from)) return builder.finish();
  do {
    if (cur.to <= from) continue; // node fully before the viewport
    if (cur.from >= to) break; // past the viewport
    const ch = bracketChar(cur.name);
    if (!ch) continue;
    if (openers.has(ch)) {
      stack.push(ch);
      const depth = Math.min(stack.length - 1, rainbowMarks.length - 1);
      builder.add(cur.from, cur.to, rainbowMarks[depth]);
    } else if (stack.length > 0 && stack[stack.length - 1] === pairOf[ch]) {
      const depth = Math.min(stack.length - 1, rainbowMarks.length - 1);
      stack.pop();
      builder.add(cur.from, cur.to, rainbowMarks[depth]);
    }
  } while (cur.next());
  return builder.finish();
}

const rainbowBracketsPlugin = ViewPlugin.fromClass(
  class {
    decorations: DecorationSet;
    constructor(view: EditorView) {
      this.decorations = computeRainbowBrackets(view);
    }
    update(update: ViewUpdate) {
      if (update.docChanged || update.viewportChanged) {
        this.decorations = computeRainbowBrackets(update.view);
      }
    }
  },
  { decorations: (v) => v.decorations }
);

let lineHighlightTimer: ReturnType<typeof setTimeout> | null = null;

// Highlights a 1-based line in the active editor for ~4s. No-op when the line
// is out of range or no editor is mounted.
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

// Sorted new-file line numbers that have a diff marker (for next/prev jump).
let diffChangedLines: number[] = [];

// Called from the diff gutter click DOM handler.
let onDiffGutterClick: ((line: number) => void) | null = null;
export function setOnDiffGutterClick(cb: ((line: number) => void) | null) {
  onDiffGutterClick = cb;
}

// Files currently having uncommitted diff hunks (drives tab "open diff" badges).
let diffFilesSet = new Set<string>();
let onDiffFilesChanged: ((files: Set<string>) => void) | null = null;
export function setOnDiffFilesChanged(cb: ((files: Set<string>) => void) | null) {
  onDiffFilesChanged = cb;
}
function updateDiffFiles(path: string, hasDiff: boolean) {
  if (hasDiff) {
    diffFilesSet.add(path);
  } else {
    diffFilesSet.delete(path);
  }
  onDiffFilesChanged?.(new Set(diffFilesSet));
}

// Pushes freshly-fetched hunks into React state (keeps the overview ruler and
// popover in sync after stage/revert refreshes).
let onDiffHunksLoaded: ((hunks: any[]) => void) | null = null;
export function setOnDiffHunksLoaded(cb: ((hunks: any[]) => void) | null) {
  onDiffHunksLoaded = cb;
}

// Icon-like marker drawn in the diff gutter.
class DiffMarker extends GutterMarker {
  type: DiffLineType;
  constructor(type: DiffLineType) {
    super();
    this.type = type;
  }
  eq(other: DiffMarker) {
    return other.type === this.type;
  }
  toDOM() {
    const el = document.createElement("span");
    el.className = "cm-diff-mark cm-diff-" + this.type;
    el.title =
      this.type === "added"
        ? "Added lines"
        : this.type === "removed"
          ? "Removed lines"
          : "Modified lines";
    return el;
  }
}

// Builds the gutter extension from the current diffLineMap. Placed to the left
// of the line-number gutter via Prec.highest (higher priority = leftmost).
// Click handling lives in the gutter's own domEventHandlers because
// EditorView.domEventHandlers only attaches to the content DOM (not the gutter).
function makeDiffGutter() {
  return gutter({
    class: "cm-diff-gutter",
    markers: (view) => {
      const builder = new RangeSetBuilder<GutterMarker>();
      const doc = view.state.doc;
      if (doc.lines > 0) {
        diffLineMap.forEach((type, lineNo) => {
          const clamped = Math.max(1, Math.min(lineNo, doc.lines));
          const line = doc.line(clamped);
          builder.add(line.from, line.from, new DiffMarker(type));
        });
      }
      return builder.finish();
    },
    domEventHandlers: {
      click: (view, line, event) => {
        const target = event.target as HTMLElement;
        if (!target.closest(".cm-diff-mark")) return false;
        const lineNo = view.state.doc.lineAt(line.from).number;
        if (onDiffGutterClick) onDiffGutterClick(lineNo);
        return true;
      },
    },
  });
}

// Fetch the structured diff hunks for a file. Prefers the structured binding;
// falls back to parsing the unified diff text (GetGitFileDiff) so the gutter
// still works even if the hunks binding is unavailable.
async function fetchDiffHunks(path: string): Promise<any[]> {
  try {
    const hunks = await GetGitFileDiffHunks("", path);
    if (Array.isArray(hunks)) return hunks;
  } catch (err) {
    console.error("GetGitFileDiffHunks failed:", err);
  }
  try {
    const text = await GetGitFileDiff("", path);
    return parseUnifiedDiff(text);
  } catch (err) {
    console.error("GetGitFileDiff fallback failed:", err);
    return [];
  }
}

// Minimal unified-diff hunk parser (mirrors internal/git/diff.go).
const HUNK_RE = /^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@/;

function parseUnifiedDiff(text: string): any[] {
  if (!text) return [];
  const hunks: any[] = [];
  let cur: any = null;
  const flush = () => {
    if (cur) {
      hunks.push(cur);
      cur = null;
    }
  };
  for (const line of text.split("\n")) {
    const m = HUNK_RE.exec(line);
    if (m) {
      flush();
      cur = {
        oldStart: parseInt(m[1], 10) || 0,
        oldLines: m[2] ? parseInt(m[2], 10) : 1,
        newStart: parseInt(m[3], 10) || 0,
        newLines: m[4] ? parseInt(m[4], 10) : 1,
        header: line,
        body: [],
      };
      continue;
    }
    if (cur) cur.body.push(line);
  }
  flush();
  return hunks;
}

// Recomputes the diff markers for the given open file path. When a live
// CodeMirror view exists, the gutter compartment is reconfigured in place.
export async function refreshFileDiff(path: string, preloadedHunks?: any[]) {
  const hunks = preloadedHunks ?? (await fetchDiffHunks(path));

  // Always update the tab-badge set so external changes sync even for
  // inactive open tabs.
  updateDiffFiles(path, Array.isArray(hunks) && hunks.length > 0);

  // Only touch the live editor state when this is the currently-open file.
  const view = globalEditorView;
  const isActive =
    !!view && view.state.doc.length > 0 && getOpenFilePath() === path;

  if (isActive) {
    if (onDiffHunksLoaded) onDiffHunksLoaded(hunks);

    const map = new Map<number, DiffLineType>();
    for (const h of Array.isArray(hunks) ? hunks : []) {
      let newLine = h.newStart || 1;
      for (const bodyLine of Array.isArray(h.body) ? h.body : []) {
        const c = bodyLine.charAt(0);
        if (c === "+") {
          map.set(newLine, "added");
          newLine++;
        } else if (c === "-") {
          map.set(newLine, "removed");
        } else {
          newLine++;
        }
      }
    }

    diffLineMap = map;
    diffChangedLines = [...map.keys()].sort((a, b) => a - b);

    if (globalEditorView && view === globalEditorView) {
      const ext = diffLineMap.size > 0 ? Prec.highest(makeDiffGutter()) : [];
      try {
        globalEditorView.dispatch({
          effects: diffCompartment.reconfigure(ext),
        });
      } catch {
        // The active editor may be a FilePane (multi-pane layout) whose state
        // has no diff-gutter Compartment. Diff markers are a single-view
        // feature; silently skip rather than crash the pane.
      }
    }
  }
}

// Derive the currently open file path from the store (best-effort).
function getOpenFilePath(): string | null {
  const st = useEditorStore.getState();
  const f = st.files[st.activeFileIndex];
  return f && f.type === "file" ? f.path : null;
}

// Jump the cursor/scroll to a changed line. dir: 1 = next, -1 = previous.
export function jumpToDiffLine(fromLine: number, dir: 1 | -1) {
  if (diffChangedLines.length === 0) return false;
  let target: number | null = null;
  if (dir > 0) {
    for (const l of diffChangedLines) {
      if (l > fromLine) {
        target = l;
        break;
      }
    }
    if (target === null) target = diffChangedLines[0];
  } else {
    let prev: number | null = null;
    for (const l of diffChangedLines) {
      if (l >= fromLine) break;
      prev = l;
    }
    target = prev !== null ? prev : diffChangedLines[diffChangedLines.length - 1];
  }
  scrollEditorToLine(target);
  return true;
}

// ---------------------------------------------------------------------------

// When a file is opened with a line hint (search / path:line), scroll the
// freshly mounted CodeMirror view to that line after the next render.
let pendingScrollLine: number | null = null;
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

/**
 * Called by the sidebar's fs:changed handler when a file is modified externally.
 * Updates the store entry AND — if the file is currently open in the active tab —
 * directly patches the CodeMirror document so the user sees the change immediately
 * without closing/reopening the tab.
 *
 * @param force  When true, bypasses the "don't overwrite unsaved edits" guard.
 *               Used for `created` events (e.g. nano/vim atomic save via rename+create)
 *               where the old file no longer exists on disk anyway.
 */
export function syncExternalFileChange(path: string, content: string, force = false) {
  const { files, activeFileIndex, setFiles } = useEditorStore.getState();
  const idx = files.findIndex((f) => f.type === "file" && f.path === path);
  if (idx === -1) return; // file not open in any tab

  const file = files[idx];
  // Respect unsaved local edits unless forced (e.g. file was atomically replaced on disk)
  if (file.modified && !force) return;

  // 1. Update the Zustand store and clear the modified flag
  setFiles((prev) => {
    const next = [...prev];
    if (next[idx]) next[idx] = { ...next[idx], content, savedContent: content, modified: false };
    return next;
  });

  // 2. If this is the active tab, push content into the live CodeMirror view
  if (idx === activeFileIndex && globalEditorView) {
    const current = globalEditorView.state.doc.toString();
    if (current !== content) {
      const sel = globalEditorView.state.selection;
      globalEditorView.dispatch({
        changes: { from: 0, to: current.length, insert: content },
        selection: { anchor: Math.min(sel.main.anchor, content.length) },
      });
    }
  }

  // 3. The on-disk content changed — refresh the diff gutter (markers/hunks).
  refreshFileDiff(path);
}

// Called when a file is deleted on disk (internal or external). Closes any
// open tab for it. Returns true when a tab was closed (callers may pair it
// with a follow-up rename).
export function syncExternalDelete(path: string): boolean {
  const { files, setFiles, setActiveFileIndex } = useEditorStore.getState();
  const idx = files.findIndex((f) => f.type === "file" && f.path === path);
  if (idx === -1) return false;
  setFiles((prev) => prev.filter((f) => !(f.type === "file" && f.path === path)));
  setActiveFileIndex((prev) => {
    if (prev === idx) return Math.max(0, prev - 1);
    if (prev > idx) return prev - 1;
    return prev;
  });
  return true;
}

// Called when a file is renamed/moved on disk (internal or external). Updates
// any open tab to the new path/name and reloads its content.
export function syncExternalRename(oldPath: string, newPath: string) {
  const { files, setFiles } = useEditorStore.getState();
  const idx = files.findIndex((f) => f.type === "file" && f.path === oldPath);
  if (idx === -1) {
    // No tab at the old path — typically an atomic save (temp file renamed
    // over the target). Sync the target's content if it is open.
    if (files.some((f) => f.type === "file" && f.path === newPath)) {
      ReadFile(newPath)
        .then((content) => syncExternalFileChange(newPath, content, true))
        .catch(() => {});
    }
    return;
  }
  const name = newPath.split(/[\\/]/).pop() || newPath;
  setFiles((prev) =>
    prev.map((f) =>
      f.type === "file" && f.path === oldPath ? { ...f, path: newPath, name, modified: false } : f
    )
  );
  ReadFile(newPath)
    .then((content) => syncExternalFileChange(newPath, content))
    .catch(() => {});
}

// Render markdown to HTML for chat responses.
function renderMarkdown(src: string): string {
  try {
    return marked.parse(src, { async: false }) as string;
  } catch {
    return src;
  }
}

// Render markdown with clickable task-list checkboxes.
// Returns HTML plus source line index of each `- [ ]` / `- [x]` item.
function renderMarkdownChecklist(src: string): { html: string; taskLines: number[] } {
  const taskLines: number[] = [];
  src.split("\n").forEach((ln, i) => {
    if (/^\s*[-*+]\s+\[[ xX]\](?:\s|$)/.test(ln)) taskLines.push(i);
  });
  let html: string;
  try {
    html = marked.parse(src, { async: false }) as string;
  } catch {
    html = src;
  }
  // marked renders tasks as `<li><input ... disabled="" type="checkbox"> ...`.
  let idx = 0;
  html = html.replace(/<input\s+([^>]*?)type="checkbox"([^>]*?)>/g, (_m, pre, post) => {
    const checked = /checked/.test(pre + post);
    return `<input type="checkbox" data-task-idx="${idx++}"${checked ? " checked" : ""}>`;
  });
  return { html, taskLines };
}

// Token usage breakdown: ↓ input, ↑ output, ⚡ cached.
function TokenUsageBadge({ usage }: { usage: any }) {
  const inTok = usage?.prompt_tokens ?? usage?.PromptTokens ?? 0;
  const outTok = usage?.completion_tokens ?? usage?.CompletionTokens ?? 0;
  const cached = usage?.cached_tokens ?? usage?.CachedTokens ?? 0;
  if (inTok + outTok + cached === 0) return null;
  return (
    <span className="flex items-center gap-2 px-1.5 py-0.5 bg-[var(--bg-panel)] border border-[var(--border-default)] text-[10px] font-mono text-[var(--fg-tertiary)] rounded" title="Token usage: input / output / cached">
      <span className="flex items-center gap-0.5">
        <ArrowDown className="size-2.5" />
        {inTok.toLocaleString()}
      </span>
      <span className="flex items-center gap-0.5">
        <ArrowUp className="size-2.5" />
        {outTok.toLocaleString()}
      </span>
      <span className="flex items-center gap-0.5" title="Cached tokens">
        <Zap className="size-2.5" />
        {cached.toLocaleString()}
      </span>
    </span>
  );
}

export async function globalOpenFile(path: string, opts?: { content?: string; name?: string; id?: string; line?: number }) {
  if (onBeforeOpenFileCallback) onBeforeOpenFileCallback();
  const { files, setFiles, setActiveFileIndex } = useEditorStore.getState();

  const tabId = opts?.id || path;
  const existingIdx = files.findIndex((f) => f.id === tabId);
  if (existingIdx !== -1) {
    setActiveFileIndex(existingIdx);
    if (opts?.line && opts.line > 0) {
      requestAnimationFrame(() => scrollEditorToLine(opts.line ?? null));
    }
    return;
  }

  try {
    const name = opts?.name || path.split(/[/\\]/).pop() || "Untitled";
    const ext = name.split(".").pop()?.toLowerCase();
    const isBinary = ["png", "jpg", "jpeg", "gif", "pdf", "ico", "svg"].includes(ext || "");
    
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
    if (opts?.line && opts.line > 0) {
      requestAnimationFrame(() => scrollEditorToLine(opts.line ?? null));
    }
  } catch (err) {
    console.error("Failed to open file:", err);
  }
}

// Opens a read-only diff tab in the editor. content is the unified diff text.
export function globalOpenDiff(diffPath: string, content: string, opts?: { diffHash?: string; label?: string }) {
  if (onBeforeOpenFileCallback) onBeforeOpenFileCallback();
  const { files, setFiles, setActiveFileIndex } = useEditorStore.getState();

  const hash = opts?.diffHash || "";
  const tabId = `diff:${hash || "worktree"}:${diffPath}`;
  const existingIdx = files.findIndex((f) => f.id === tabId);
  if (existingIdx !== -1) {
    setActiveFileIndex(existingIdx);
    return;
  }

  const base = diffPath.split(/[/\\]/).pop() || "diff";
  const name = hash ? `${base} @ ${hash.slice(0, 7)}` : base;
  const newFile = {
    id: tabId,
    name: opts?.label || name,
    path: diffPath,
    type: "diff" as "diff",
    content,
    modified: false,
    diffPath,
    diffHash: hash || undefined,
  };

  setFiles((prev) => [...prev, newFile]);
  setActiveFileIndex(useEditorStore.getState().files.length - 1);
}

// Opens a conflict-resolution tab for a file with merge conflicts.
export function globalOpenConflict(path: string, status?: string) {
  if (onBeforeOpenFileCallback) onBeforeOpenFileCallback();
  const { files, setFiles, setActiveFileIndex } = useEditorStore.getState();

  const tabId = `conflict:${path}`;
  const existingIdx = files.findIndex((f) => f.id === tabId);
  if (existingIdx !== -1) {
    setActiveFileIndex(existingIdx);
    return;
  }

  const base = path.split(/[/\\]/).pop() || "conflict";
  const newFile = {
    id: tabId,
    name: base,
    path,
    type: "conflict" as "conflict",
    content: "",
    modified: false,
    conflictPath: path,
    conflictStatus: status,
  };

  setFiles((prev) => [...prev, newFile]);
  setActiveFileIndex(useEditorStore.getState().files.length - 1);
}

// Read-only diff tab rendered in the editor, with quick actions to open the
// working file or view the file content at the commit it belongs to.
function DiffTabView({ file }: { file: EditorFile }) {
  const { toast } = useToast();
  const [loadingCommit, setLoadingCommit] = useState(false);
  const diffPath = file.diffPath || file.path;
  const diffHash = file.diffHash;

  const handleOpenFile = () => {
    if (diffPath) globalOpenFile(diffPath);
  };

  const handleViewAtCommit = async () => {
    if (!diffHash || !diffPath) return;
    setLoadingCommit(true);
    try {
      const content = await GetGitFileContentAtCommit("", diffHash, diffPath);
      const base = diffPath.split(/[/\\]/).pop() || "file";
      await globalOpenFile(diffPath, {
        id: `commit:${diffHash}:${diffPath}`,
        name: `${base} @ ${diffHash.slice(0, 7)}`,
        content,
      });
    } catch (err: any) {
      toast("Failed to load file at commit: " + err, "danger");
    } finally {
      setLoadingCommit(false);
    }
  };

  return (
    <div className="flex flex-col h-full overflow-hidden">
      <div className="flex items-center justify-between px-4 py-2 border-b border-[var(--border-default)] bg-[var(--bg-sidebar)] shrink-0 select-none">
        <div className="flex items-center gap-2 text-[var(--fg-secondary)] font-mono text-xs truncate min-w-0">
          <FileDiff className="w-4 h-4 text-purple-400 shrink-0" />
          <span className="truncate">{diffPath}</span>
          {diffHash && <span className="text-[var(--accent-primary)] shrink-0">{diffHash.slice(0, 7)}</span>}
        </div>
        <div className="flex items-center gap-1.5 shrink-0">
          {diffHash && (
            <button
              onClick={handleViewAtCommit}
              disabled={loadingCommit}
              className="px-2 py-1 bg-[var(--bg-panel)] border border-[var(--border-default)] hover:bg-[var(--bg-surface-hover)] text-[var(--fg-primary)] rounded text-[10px] flex items-center gap-1 cursor-pointer disabled:opacity-50"
              title="Open file content as it was in this commit"
            >
              <History className="w-3.5 h-3.5 text-purple-400" />
              <span>{loadingCommit ? "Loading..." : "View at Commit"}</span>
            </button>
          )}
          <button
            onClick={handleOpenFile}
            className="px-2 py-1 bg-[var(--bg-panel)] border border-[var(--border-default)] hover:bg-[var(--bg-surface-hover)] text-[var(--fg-primary)] rounded text-[10px] flex items-center gap-1 cursor-pointer"
            title="Open current working file in the editor"
          >
            <FileText className="w-3.5 h-3.5 text-blue-400" />
            <span>Open File</span>
          </button>
        </div>
      </div>
      <div className="flex-1 overflow-y-auto p-3">
        <DiffView content={file.content ?? ""} emptyText="No changes in this file." />
      </div>
    </div>
  );
}

// Conflict-resolution tab — mirrors VS Code's "Resolve Conflicts" flow:
// preview current/incoming, edit the working file, or accept one side.
function ConflictTabView({ file }: { file: EditorFile }) {
  const { toast } = useToast();
  const conflictPath = file.conflictPath || file.path;
  const conflictStatus = file.conflictStatus || "";

  const [working, setWorking] = useState("");
  const [ours, setOurs] = useState("");
  const [theirs, setTheirs] = useState("");
  const [ancestor, setAncestor] = useState("");
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [work, o, t, base] = await Promise.all([
        ReadFile(conflictPath).catch(() => ""),
        GetGitConflictStageContent("", conflictPath, 2).catch(() => ""),
        GetGitConflictStageContent("", conflictPath, 3).catch(() => ""),
        GetGitConflictStageContent("", conflictPath, 1).catch(() => ""),
      ]);
      setWorking(work);
      setOurs(o);
      setTheirs(t);
      setAncestor(base);
    } catch { /* ignore */ }
    setLoading(false);
  }, [conflictPath]);

  useEffect(() => {
    load();
  }, [load]);

  const acceptSide = async (action: "ours" | "theirs") => {
    if (busy) return;
    setBusy(true);
    try {
      await GitResolveConflict("", conflictPath, action);
      const updated = await ReadFile(conflictPath);
      setWorking(updated);
      const side = action === "ours" ? await GetGitConflictStageContent("", conflictPath, 2) : await GetGitConflictStageContent("", conflictPath, 3);
      if (action === "ours") setOurs(side);
      else setTheirs(side);
      toast(`Conflict resolved — accepted ${action === "ours" ? "current" : "incoming"}.`, "success");
    } catch (err: any) {
      toast("Failed to resolve conflict: " + err, "danger");
    } finally {
      setBusy(false);
    }
  };

  const markResolved = async () => {
    if (busy) return;
    setBusy(true);
    try {
      await WriteFile(conflictPath, working);
      await GitResolveConflict("", conflictPath, "mark");
      toast("Conflict resolved and staged.", "success");
    } catch (err: any) {
      toast("Failed to mark resolved: " + err, "danger");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="flex flex-col h-full overflow-hidden">
      <div className="flex items-center justify-between px-4 py-2 border-b border-[var(--border-default)] bg-[var(--bg-sidebar)] shrink-0 select-none">
        <div className="flex items-center gap-2 text-[var(--fg-secondary)] font-mono text-xs truncate min-w-0">
          <FileDiff className="w-4 h-4 text-amber-400 shrink-0" />
          <span className="truncate">{conflictPath}</span>
          <span className="text-amber-400 bg-amber-500/10 border border-amber-500/40 px-1.5 py-0.5 rounded text-[9px] font-bold shrink-0">
            {conflictStatus || "conflict"}
          </span>
        </div>
        <div className="flex items-center gap-1.5 shrink-0">
          <button
            onClick={() => acceptSide("ours")}
            disabled={busy}
            className="px-2 py-1 bg-emerald-600/80 hover:bg-emerald-600 text-white rounded text-[10px] font-semibold flex items-center gap-1 cursor-pointer disabled:opacity-50"
            title="Accept the current (HEAD) version of the file"
          >
            <Check className="w-3.5 h-3.5" />
            <span>Accept Current</span>
          </button>
          <button
            onClick={() => acceptSide("theirs")}
            disabled={busy}
            className="px-2 py-1 bg-sky-600/80 hover:bg-sky-600 text-white rounded text-[10px] font-semibold flex items-center gap-1 cursor-pointer disabled:opacity-50"
            title="Accept the incoming (merged) version of the file"
          >
            <Check className="w-3.5 h-3.5" />
            <span>Accept Incoming</span>
          </button>
          <button
            onClick={markResolved}
            disabled={busy}
            className="px-2 py-1 bg-[var(--accent-primary)] hover:bg-[var(--accent-hover)] text-black rounded text-[10px] font-semibold flex items-center gap-1 cursor-pointer disabled:opacity-50"
            title="Stage the edited file to mark the conflict resolved"
          >
            <History className="w-3.5 h-3.5" />
            <span>{busy ? "Working..." : "Mark Resolved"}</span>
          </button>
        </div>
      </div>

      {loading ? (
        <div className="flex-1 flex items-center justify-center text-[var(--fg-tertiary)] text-xs animate-pulse">
          Loading conflict data...
        </div>
      ) : (
        <div className="flex-1 min-h-0 flex flex-col">
          {/* Current / Incoming preview panes */}
          <div className="flex flex-row shrink-0 h-40 border-b border-[var(--border-default)]">
            <div className="flex-1 min-w-0 flex flex-col">
              <div className="px-3 py-1 text-[9px] uppercase tracking-wider text-emerald-400 bg-emerald-500/5 border-b border-[var(--border-default)] select-none shrink-0">
                Current (HEAD)
              </div>
              <pre className="flex-1 overflow-auto p-2 text-[11px] font-mono text-[var(--fg-secondary)] whitespace-pre-wrap break-words">{ours || "(no content)"}</pre>
            </div>
            <div className="w-px bg-[var(--border-default)] shrink-0" />
            <div className="flex-1 min-w-0 flex flex-col">
              <div className="px-3 py-1 text-[9px] uppercase tracking-wider text-sky-400 bg-sky-500/5 border-b border-[var(--border-default)] select-none shrink-0">
                Incoming (theirs)
              </div>
              <pre className="flex-1 overflow-auto p-2 text-[11px] font-mono text-[var(--fg-secondary)] whitespace-pre-wrap break-words">{theirs || "(no content)"}</pre>
            </div>
            {ancestor !== "" && (
              <>
                <div className="w-px bg-[var(--border-default)] shrink-0" />
                <div className="flex-1 min-w-0 flex flex-col">
                  <div className="px-3 py-1 text-[9px] uppercase tracking-wider text-[var(--fg-tertiary)] bg-[var(--bg-panel)] border-b border-[var(--border-default)] select-none shrink-0">
                    Ancestor
                  </div>
                  <pre className="flex-1 overflow-auto p-2 text-[11px] font-mono text-[var(--fg-tertiary)] whitespace-pre-wrap break-words">{ancestor || "(no content)"}</pre>
                </div>
              </>
            )}
          </div>

          {/* Editable working file (with conflict markers) */}
          <div className="flex-1 min-h-0 flex flex-col">
            <div className="px-3 py-1 text-[9px] uppercase tracking-wider text-amber-400 bg-amber-500/5 border-b border-[var(--border-default)] select-none shrink-0">
              Working File — edit below, then “Mark Resolved”
            </div>
            <textarea
              value={working}
              onChange={(e) => setWorking(e.target.value)}
              spellCheck={false}
              className="flex-1 w-full resize-none bg-[var(--bg-app)] p-3 text-[12px] font-mono text-[var(--fg-primary)] focus:outline-none"
              placeholder="The file with conflict markers (<<<<<<< ======= >>>>>>>) appears here..."
            />
          </div>
        </div>
      )}
    </div>
  );
}

// Emitted after git mutations (stage/revert) so the explorer + git panel refresh.
function notifyGitStatusChanged() {
  window.dispatchEvent(new CustomEvent("forge:git-status-changed"));
}

// Popover shown when a diff-gutter marker is clicked: hunk preview (red =
// removed, green = added) plus actions (revert, stage, prev/next, close).
function DiffGutterMenu({
  line,
  x,
  y,
  path,
  hunks,
  onClose,
}: {
  line: number;
  x: number;
  y: number;
  path: string;
  hunks: any[];
  onClose: () => void;
}) {
  const { toast } = useToast();
  const [busy, setBusy] = useState(false);

  // New-file line span covered by a hunk, including positions where removed
  // lines are anchored (which can extend beyond newStart+newLines for pure
  // deletions where newLines === 0).
  const hunkMarkerRange = (h: any): { from: number; to: number } => {
    const start = h.newStart || 1;
    let newLine = start;
    let maxMarked = start;
    for (const bl of Array.isArray(h.body) ? h.body : []) {
      const c = bl.charAt(0);
      if (c === "+") {
        maxMarked = Math.max(maxMarked, newLine);
        newLine++;
      } else if (c === "-") {
        maxMarked = Math.max(maxMarked, newLine);
      } else {
        maxMarked = Math.max(maxMarked, newLine);
        newLine++;
      }
    }
    return { from: start, to: Math.max(start + 1, maxMarked + 1) };
  };

  const hunk = hunks.find((h) => {
    const r = hunkMarkerRange(h);
    return line >= r.from && line < r.to;
  });
  const hunkIndex = hunks.findIndex((h) => h === hunk);
  const changedCount = hunks.reduce((n, h) => n + (h.newLines || 0), 0);

  const prevDisabled = changedCount <= 1;
  const nextDisabled = changedCount <= 1;

  const doRevert = async () => {
    if (hunkIndex < 0) return;
    setBusy(true);
    try {
      await RevertGitHunk("", path, hunkIndex);
      toast(`Reverted changes in ${path.split(/[/\\]/).pop()}`, "success");
      notifyGitStatusChanged();
      await refreshFileDiff(path);
      onClose();
    } catch (err: any) {
      toast("Revert failed: " + err, "danger");
    } finally {
      setBusy(false);
    }
  };

  const doStage = async () => {
    setBusy(true);
    try {
      await GitStage("", [path]);
      toast(`Staged ${path.split(/[/\\]/).pop()}`, "success");
      notifyGitStatusChanged();
      await refreshFileDiff(path);
      onClose();
    } catch (err: any) {
      toast("Stage failed: " + err, "danger");
    } finally {
      setBusy(false);
    }
  };

  const W = 340;
  const left = Math.min(x, window.innerWidth - W - 8);
  const top = Math.min(y, window.innerHeight - 240);

  return (
    <>
      <div className="fixed inset-0 z-40" onClick={onClose} onContextMenu={(e) => { e.preventDefault(); onClose(); }} />
      <div
        className="fixed z-50 rounded-lg border border-[var(--border-default)] bg-[var(--bg-sidebar)] shadow-2xl overflow-hidden"
        style={{ left, top, width: W }}
      >
        <div className="flex items-center justify-between px-3 py-1.5 border-b border-[var(--border-default)] bg-[var(--bg-panel)] text-[10px] text-[var(--fg-tertiary)]">
          <span className="font-mono">
            Line {line}
            {hunk ? ` · hunk ${hunkIndex + 1}/${hunks.length}` : ""}
          </span>
          <button
            onClick={onClose}
            className="p-0.5 hover:bg-[var(--bg-surface-hover)] rounded cursor-pointer text-[var(--fg-tertiary)] hover:text-[var(--fg-primary)]"
            title="Close"
          >
            <X className="size-3.5" />
          </button>
        </div>

        <div className="max-h-44 overflow-y-auto font-mono text-[11px] leading-[1.5] py-1 select-text">
          {hunk ? (
            hunk.body.map((l: string, i: number) => {
              const c = l.charAt(0);
              const cls =
                c === "+"
                  ? "text-emerald-400 bg-emerald-500/10"
                  : c === "-"
                    ? "text-red-400 bg-red-500/10"
                    : "text-[var(--fg-tertiary)]";
              return (
                <div key={i} className={`px-3 whitespace-pre ${cls}`}>
                  {l}
                </div>
              );
            })
          ) : (
            <div className="px-3 text-[var(--fg-tertiary)]">No changes on this line.</div>
          )}
        </div>

        <div className="flex items-center gap-1 px-2 py-1.5 border-t border-[var(--border-default)] bg-[var(--bg-panel)]">
          <button
            onClick={doRevert}
            disabled={busy || hunkIndex < 0}
            className="flex items-center gap-1 px-2 py-1 rounded bg-red-500/15 text-red-300 hover:bg-red-500/25 disabled:opacity-40 text-[10px] font-medium cursor-pointer disabled:cursor-default"
            title={hunkIndex < 0 ? "No diff hunk for this line" : "Revert this change (discard hunk)"}
          >
            <Undo2 className="size-3" />
            Revert
          </button>
          <button
            onClick={doStage}
            disabled={busy}
            className="flex items-center gap-1 px-2 py-1 rounded bg-emerald-500/15 text-emerald-300 hover:bg-emerald-500/25 disabled:opacity-40 text-[10px] font-medium cursor-pointer disabled:cursor-default"
            title="Stage this file"
          >
            <CirclePlus className="size-3" />
            Stage
          </button>
          <div className="flex-1" />
          <button
            onClick={() => jumpToDiffLine(line, -1)}
            disabled={busy || prevDisabled}
            className="flex items-center gap-1 px-1.5 py-1 rounded hover:bg-[var(--bg-surface-hover)] text-[var(--fg-secondary)] disabled:opacity-40 cursor-pointer disabled:cursor-default"
            title="Show previous change"
          >
            <ChevronUp className="size-3.5" />
          </button>
          <button
            onClick={() => jumpToDiffLine(line, 1)}
            disabled={busy || nextDisabled}
            className="flex items-center gap-1 px-1.5 py-1 rounded hover:bg-[var(--bg-surface-hover)] text-[var(--fg-secondary)] disabled:opacity-40 cursor-pointer disabled:cursor-default"
            title="Show next change"
          >
            <ChevronDown className="size-3.5" />
          </button>
        </div>
      </div>
    </>
  );
}

// Mini overview ruler on the right edge of the editor: a dot per changed line
// so changes are visible without scrolling. Clicking a dot jumps to the line.
function DiffOverviewRuler({
  changes,
  totalLines,
}: {
  changes: Array<{ line: number; type: DiffLineType }>;
  totalLines: number;
}) {
  if (totalLines <= 0 || changes.length === 0) return null;
  return (
    <div className="diff-overview" aria-hidden>
      {changes.map((c) => {
        const topPct = Math.max(0, Math.min(100, ((c.line - 0.5) / totalLines) * 100));
        const color = c.type === "removed" ? "var(--diff-removed, #f85149)" : "var(--diff-added, #3fb950)";
        return (
          <div
            key={c.line}
            className="diff-overview-dot"
            style={{ top: `${topPct}%`, background: color }}
            title={`Line ${c.line}${c.type === "removed" ? " (removed)" : " (changed)"}`}
            onClick={(e) => {
              e.stopPropagation();
              scrollEditorToLine(c.line);
            }}
          />
        );
      })}
    </div>
  );
}

function getLanguageExtension(path: string) {
  const base = path.split("/").pop() || path;
  const ext = base.includes(".")
    ? base.split(".").pop()?.toLowerCase()
    : base.toLowerCase();
  switch (ext) {
    // Scripting / web (JS-family syntax)
    case "js":
    case "jsx":
    case "ts":
    case "tsx":
    case "mjs":
    case "cjs":
    case "mts":
    case "cts":
      return javascript();
    case "vue":
    case "svelte":
      return javascript();
    case "json":
    case "jsonc":
    case "json5":
    case "geojson":
      return json();
    case "html":
    case "htm":
    case "xml":
    case "svg":
    case "xsl":
    case "xslt":
    case "rss":
    case "xhtml":
    case "dtd":
    case "wsdl":
    case "csproj":
    case "fsproj":
    case "vbproj":
      return html();
    case "md":
    case "markdown":
    case "mdx":
      return markdown();
    case "css":
    case "pcss":
    case "postcss":
      return css();
    case "scss":
    case "sass":
      return sass();
    case "less":
      return less();
    case "styl":
      return less();
    // Backend / systems (C-family syntax)
    case "c":
    case "h":
    case "cpp":
    case "cc":
    case "cxx":
    case "hpp":
    case "hxx":
    case "ino":
    case "cs":
    case "java":
    case "kt":
    case "kts":
    case "m":
    case "mm":
    case "swift":
    case "go":
    case "rs":
    case "dart":
      return ext === "go" ? go() : ext === "rs" ? rust() : cpp();
    case "py":
    case "pyw":
    case "rb":
    case "php":
    case "pl":
    case "pm":
    case "lua":
    case "r":
    case "jl":
      return ext === "py" || ext === "pyw"
        ? python()
        : ext === "rb"
          ? python()
          : ext === "php"
            ? php()
            : python();
    // Data / config (key-value, JSON-ish)
    case "yaml":
    case "yml":
    case "toml":
    case "ini":
    case "cfg":
    case "conf":
    case "properties":
    case "env":
      return json();
    case "sql":
      return sql();
    default:
      return [];
  }
}

export function Editor() {
  const { toast } = useToast();
  const { files, activeFileIndex, setFiles, setActiveFileIndex } = useEditorStore();
  const {
    browserTabs,
    activeBrowserTabId,
    workspaceLayoutMode,
    paneShares,
    setActiveBrowserTab,
    closeBrowserTab,
    openBrowserTab,
    setWorkspaceLayoutMode,
    setPaneShare,
  } = useWorkspaceTabStore();
  const [showCreateModal, setShowCreateModal] = useState(false);
  const editorRef = useRef<HTMLDivElement>(null);
  const splitRef = useRef<HTMLDivElement | null>(null);
  const viewRef = useRef<EditorView | null>(null);
  // Last (content, previewMode) the CodeMirror view was built for, so keystroke
  // echoes don't rebuild the view (would reset undo history + cursor).
  const lastBuildRef = useRef<{ id: string; path: string; content: string; preview: "edit" | "preview" } | null>(null);
  // Per-file live doc buffers, keyed by file id. Each tab keeps its own live
  // content so switching tabs never leaks one file's text into another's store
  // entry (the data-loss bug: a single shared buffer wrote stale or empty text
  // over the previously-active file's content on every tab switch).
  const liveContentRef = useRef<Map<string, string>>(new Map());
  // Pending debounced write timer for the current CodeMirror doc. Cleared on
  // tab switch (after flushing) so it can't fire into a stale target.
  const contentTimerRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  // Most-recently-activated file tab ids (LRU) — eviction keeps these loaded.
  const lruRef = useRef<string[]>([]);
  
  const activeFile = files[activeFileIndex];

  // Markdown preview: render clickable task-list checkboxes (toggle `[ ]` / `[x]`).
  const mdChecklist = useMemo(
    () => (activeFile ? renderMarkdownChecklist(activeFile.content ?? "") : { html: "", taskLines: [] }),
    [activeFile?.path, activeFile?.content]
  );
  const toggleMdTask = useCallback(
    (taskIdx: number) => {
      const lineIdx = mdChecklist.taskLines[taskIdx];
      if (lineIdx === undefined || !activeFile) return;
      const lines = (activeFile.content ?? "").split("\n");
      const ln = lines[lineIdx];
      const next = ln.replace(/^(\s*[-*+]\s+\[)[ xX](\])/, (_m, a) => a + (/\[[xX]\]/.test(ln) ? " " : "x") + "]");
      if (next === ln) return;
      lines[lineIdx] = next;
      const newContent = lines.join("\n");
      setFiles((prev) => {
        const nextArr = [...prev];
        const cur = nextArr[activeFileIndex];
        if (!cur) return prev;
        const saved = cur.savedContent !== undefined ? cur.savedContent : (cur.content ?? "");
        nextArr[activeFileIndex] = { ...cur, content: newContent, savedContent: saved, modified: true };
        return nextArr;
      });
    },
    [mdChecklist.taskLines, activeFile?.content, activeFileIndex, setFiles]
  );

  // Binary/Viewer states
  const [imageBase64, setImageBase64] = useState<string | null>(null);
  const [pdfBase64, setPdfBase64] = useState<string | null>(null);
  const [previewMode, setPreviewMode] = useState<"edit" | "preview">("edit");

  // Diff-gutter state: hunks for the active file + the open popover.
  const [diffHunks, setDiffHunks] = useState<any[]>([]);
  const [diffMenu, setDiffMenu] = useState<{ line: number; x: number; y: number } | null>(null);

  // File paths that currently have uncommitted diff hunks (for tab badges).
  const [diffFiles, setDiffFiles] = useState<Set<string>>(new Set(diffFilesSet));

  // Keep tab badges in sync when any open file's diff changes externally.
  useEffect(() => {
    setOnDiffFilesChanged((files) => setDiffFiles(files));
    return () => setOnDiffFilesChanged(null);
  }, []);

  // Changed new-file lines derived from hunks (line + marker type).
  const diffChanges = useMemo(() => {
    const map = new Map<number, DiffLineType>();
    for (const h of diffHunks) {
      let newLine = h.newStart || 1;
      for (const bl of Array.isArray(h.body) ? h.body : []) {
        const c = bl.charAt(0);
        if (c === "+") {
          map.set(newLine, "added");
          newLine++;
        } else if (c === "-") {
          map.set(newLine, "removed");
        } else {
          newLine++;
        }
      }
    }
    return [...map.entries()]
      .map(([line, type]) => ({ line, type }))
      .sort((a, b) => a.line - b.line);
  }, [diffHunks]);

  // Live line count of the active document (updates as the user types).
  const totalLines = activeFile?.type === "file" && activeFile.content
    ? activeFile.content.split("\n").length
    : 0;

  // Wire the diff gutter click handler up to React state.
  useEffect(() => {
    setOnDiffGutterClick((line) => {
      const view = globalEditorView;
      if (!view) return;
      const rect = view.dom.getBoundingClientRect();
      const lineTop = view.coordsAtPos(view.state.doc.line(line).from);
      setDiffMenu({
        line,
        x: Math.round(rect.left + 8),
        y: lineTop ? Math.round(lineTop.top) : Math.round(rect.top + 8),
      });
    });
    setOnDiffHunksLoaded(setDiffHunks);
    return () => {
      setOnDiffGutterClick(null);
      setOnDiffHunksLoaded(null);
    };
  }, []);

  // Load the diff hunks whenever the active file changes.
  useEffect(() => {
    setDiffMenu(null);
    if (!activeFile || activeFile.type !== "file") {
      diffLineMap = new Map();
      diffChangedLines = [];
      setDiffHunks([]);
      return;
    }
    let cancelled = false;
    (async () => {
      const hunks = await fetchDiffHunks(activeFile.path);
      if (cancelled) return;
      setDiffHunks(Array.isArray(hunks) ? hunks : []);
      refreshFileDiff(activeFile.path, hunks);
    })();
    return () => {
      cancelled = true;
    };
  }, [activeFile?.path, activeFile?.type]);

  // Load binary contents when active tab changes
  useEffect(() => {
    setImageBase64(null);
    setPdfBase64(null);

    if (!activeFile || activeFile.type !== "file") return;

    const ext = activeFile.name.split(".").pop()?.toLowerCase();
    if (["png", "jpg", "jpeg", "gif", "ico", "svg"].includes(ext || "")) {
      ReadFileBase64(activeFile.path).then((data) => {
        setImageBase64(data);
      }).catch(console.error);
    } else if (ext === "pdf") {
      ReadFileBase64(activeFile.path).then((data) => {
        setPdfBase64(data);
      }).catch(console.error);
    }
    
    if (["html", "htm", "md", "markdown", "mdx"].includes(ext || "")) {
      setPreviewMode("edit");
    }
  }, [activeFileIndex, activeFile?.path]);

  // CodeMirror instance mounting
  useEffect(() => {
    if (!editorRef.current || !activeFile || activeFile.type !== "file") {
      if (viewRef.current) {
        viewRef.current.destroy();
        viewRef.current = null;
      }
      return;
    }

    if (activeFile.content === null) {
      // Evicted tab: reload from disk; the view rebuilds when content lands.
      if (viewRef.current) {
        viewRef.current.destroy();
        viewRef.current = null;
      }
      setGlobalEditorView(null);
      const id = activeFile.id;
      let cancelled = false;
      ReadFile(activeFile.path)
        .then((c) => {
          if (cancelled) return;
          const { files, setFiles } = useEditorStore.getState();
          setFiles(files.map((f) => (f.id === id ? { ...f, content: c } : f)));
        })
        .catch(() => {});
      return () => {
        cancelled = true;
      };
    }

    // Flush any pending edits from the file that was just displayed into the
    // store BEFORE switching, keyed by the previous file's id — never by a
    // stale shared buffer or a positional index. This is the fix for the
    // content-loss bug: the old code wrote a single liveContentRef (empty or
    // stale) over the previous file's store entry on every tab switch.
    if (viewRef.current && lastBuildRef.current && lastBuildRef.current.id !== activeFile.id) {
      const prevId = lastBuildRef.current.id;
      const live = liveContentRef.current.get(prevId);
      if (live !== undefined) {
        setFiles((prev) =>
          prev.map((f) =>
            f.id === prevId && f.type === "file" && live !== f.content
              ? { ...f, content: live, modified: live !== f.savedContent }
              : f
          )
        );
      }
      // Clear the timer so a pending debounce for the old file can't fire into
      // the wrong tab later. The flush above already captured its content.
      if (contentTimerRef.current) {
        clearTimeout(contentTimerRef.current);
        contentTimerRef.current = undefined;
      }
    }
    if (
      viewRef.current &&
      viewRef.current.state.doc.toString() === activeFile.content &&
      lastBuildRef.current?.id === activeFile.id &&
      lastBuildRef.current?.preview === previewMode
    ) {
      return;
    }

    const ext = activeFile.name.split(".").pop()?.toLowerCase() || "";
    const isBinary = ["png", "jpg", "jpeg", "gif", "pdf", "ico", "svg"].includes(ext);
    if (isBinary || (["html", "htm", "md", "markdown", "mdx"].includes(ext) && previewMode === "preview")) {
      if (viewRef.current) {
        viewRef.current.destroy();
        viewRef.current = null;
      }
      return;
    }

    // getLanguageExtension is a module-scope function (hoisted below).

    // Mirror the doc into the store on a debounce. Doing it per keystroke
    // re-renders every React subscriber on every keypress, which stalls
    // typing on large files. 250ms is imperceptible for the "modified" dot.
    // The live buffer is stored per file id (liveContentRef), and the write
    // targets the file id — never a captured positional index — so a pending
    // timer can't overwrite a different tab after reorders.
    contentTimerRef.current && clearTimeout(contentTimerRef.current);
    const fileId = activeFile.id;
    const updateListener = EditorView.updateListener.of((update) => {
      if (update.docChanged) {
        const newContent = update.state.doc.toString();
        liveContentRef.current.set(fileId, newContent);
        clearTimeout(contentTimerRef.current);
        contentTimerRef.current = setTimeout(() => {
          setFiles((prev) =>
            prev.map((f) =>
              f.id === fileId && f.type === "file" && newContent !== f.content
                ? { ...f, content: newContent, modified: newContent !== f.savedContent }
                : f
            )
          );
        }, 250);
      }
    });

    // Lightweight syntax diagnostics via esbuild (no LSP) — JS/TS only.
    const syntaxLinter = linter(async (view) => {
      const path = activeFile.path;
      const ext = path.split(".").pop()?.toLowerCase() || "";
      // Skip huge files — per-keystroke esbuild spawn + full-doc IPC is the
      // dominant typing-lag cost on large projects.
      if (view.state.doc.length > 300_000) return [];
      if (!["js", "jsx", "ts", "tsx", "mjs", "cjs", "mts", "cts"].includes(ext)) return [];
      try {
        const diags = await CheckSyntax(path, view.state.doc.toString());
        return (Array.isArray(diags) ? diags : []).map((d: any) => ({
          from: Math.max(0, view.state.doc.line(Math.max(1, d.line)).from + Math.max(0, d.column - 1)),
          to: Math.max(0, view.state.doc.line(Math.max(1, d.line)).to),
          severity: "error" as const,
          message: d.message || "Syntax error",
        }));
      } catch {
        return [];
      }
    }, { delay: 350 });

    // Format via backend (biome/prettier when available) — Cmd+Shift+F.
    const formatKeymap = keymap.of([{
      key: "Mod-Shift-f",
      run: (view) => {
        FormatCode(activeFile.path, view.state.doc.toString()).then((formatted) => {
          if (formatted && formatted !== view.state.doc.toString()) {
            view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: formatted } });
          }
        }).catch(() => {});
        return true;
      },
    }]);

    // Comment toggling — Cmd+/ line comment, Cmd+Shift+/ block comment.
    const commentKeymap = keymap.of([
      { key: "Mod-/", run: toggleComment },
      { key: "Mod-Alt-/", run: toggleBlockComment },
    ]);

    // VSCode-style editing basics: history (undo/redo), Tab indent / Shift-Tab
    // outdent, and bracket indentation (Cmd+] / Cmd+[).
    const basicKeymap = keymap.of([
      indentWithTab,
      { key: "Shift-Tab", run: indentLess },
      { key: "Mod-]", run: indentMore },
      { key: "Mod-[", run: indentLess },
      ...historyKeymap,
    ]);

    const editorSearchKeymap = keymap.of([
      { key: "Mod-f", run: openSearchPanel },
      { key: "Mod-p", run: openSearchPanel },
      { key: "F3", run: openSearchPanel },
      ...searchKeymap,
    ]);

    const state = EditorState.create({
      doc: activeFile.content,
      extensions: [
        history(),
        keymap.of(defaultKeymap),
        search({ top: true, caseSensitive: false, literal: false, regexp: false }),
        editorSearchKeymap,
        getLanguageExtension(activeFile.path),
        workspaceCompletion(),
        highlightSelectionMatches({ highlightWordAroundCursor: true }),
        lineNumbers(),
        highlightActiveLineGutter(),
        highlightActiveLine(),
        diffCompartment.of([]),
        lineHighlightField,
        oneDark,
        updateListener,
        // Paste a file/folder copied from Finder → insert full path instead of
        // filename. Only intercept when the clipboard actually holds files; for
        // normal text we let the native paste run — intercepting it forces a
        // slow Wails↔osascript round-trip that delays/mangles large pastes.
        EditorView.domEventHandlers({
          paste: (event, view) => {
            if (!event.clipboardData) return false;
            const types = Array.from(event.clipboardData.types || []);
            const hasFiles =
              (event.clipboardData.files && event.clipboardData.files.length > 0) ||
              types.includes("Files") ||
              types.some((t) => t === "public.file-url" || t === "text/uri-list");
            if (!hasFiles) return false; // normal text paste → native, instant
            const text = event.clipboardData.getData("text");
            event.preventDefault(); // handle insertion ourselves → no double-paste
            GetClipboardFiles().then((paths) => {
              const insert = paths && paths.length
                ? paths.map((p) => (p.includes(" ") ? `'${p.replace(/'/g, "'\\''")}'` : p)).join(" ")
                : text;
              if (!insert) return;
              view.dispatch({ changes: { from: view.state.selection.main.from, insert } });
            }).catch(() => {
              if (text) view.dispatch({ changes: { from: view.state.selection.main.from, insert: text } });
            });
            return true;
          },
        }),
        EditorView.lineWrapping,
        // Bracket matching + auto-close (syntax-tree aware via language
        // grammar, works for every installed language).
        bracketMatching(),
        closeBrackets(),
        keymap.of(closeBracketsKeymap),
        rainbowBracketsPlugin,
        syntaxLinter,
        formatKeymap,
        commentKeymap,
        basicKeymap,
      ],
    });

    if (viewRef.current) {
      viewRef.current.setState(state);
    } else {
      viewRef.current = new EditorView({
        state,
        parent: editorRef.current,
      });
    }
    lastBuildRef.current = { id: activeFile.id, path: activeFile.path, content: activeFile.content, preview: previewMode };
    setGlobalEditorView(viewRef.current);

    // Jump to a requested line (opened from search results / path:line).
    if (pendingScrollLine && pendingScrollLine > 0) {
      const line = pendingScrollLine;
      pendingScrollLine = null;
      requestAnimationFrame(() => {
        const v = viewRef.current;
        if (!v) return;
        const clamped = Math.max(1, Math.min(line, v.state.doc.lines));
        const info = v.state.doc.line(clamped);
        v.dispatch({
          selection: { anchor: info.from },
          effects: EditorView.scrollIntoView(info.from, { y: "center" }),
        });
        v.focus();
        highlightLine(clamped);
      });
    }
  }, [activeFileIndex, activeFile?.path, activeFile?.content, previewMode]);

  useEffect(() => {
    return () => {
      if (contentTimerRef.current) {
        clearTimeout(contentTimerRef.current);
        contentTimerRef.current = undefined;
      }
      if (viewRef.current) {
        viewRef.current.destroy();
        viewRef.current = null;
      }
      setGlobalEditorView(null);
    };
  }, []);

  // Memory: keep only the active tab + a few recently-visited unmodified tabs
  // loaded in RAM. Evicted tabs reload from disk on activation (see the
  // CodeMirror effect above), so opening many files no longer multiplies
  // memory by every open file's size.
  useEffect(() => {
    const active = files[activeFileIndex];
    if (active?.type === "file" && active.content !== null) {
      lruRef.current = [active.id, ...lruRef.current.filter((id) => id !== active.id)].slice(0, 8);
    }
    const keep = new Set([active?.id, ...lruRef.current.slice(0, 4)].filter(Boolean));
    let changed = false;
    const next = files.map((f) => {
      // Binary tabs keep content "" (base64 is separate) — nothing to evict.
      if (f.type !== "file" || f.modified || f.content === null || f.content === "") return f;
      if (keep.has(f.id)) return f;
      changed = true;
      return { ...f, content: null, savedContent: undefined };
    });
    if (changed) setFiles(next);
  }, [files, activeFileIndex]);

  const closeTab = (idx: number) => {
    setFiles((prev) => {
      const next = [...prev];
      next.splice(idx, 1);
      return next;
    });
    setActiveFileIndex((prev) => {
      if (prev === idx) return Math.max(0, prev - 1);
      if (prev > idx) return prev - 1;
      return prev;
    });
  };

  const closeRight = (idx: number) => {
    setFiles((prev) => prev.slice(0, idx + 1));
    setActiveFileIndex((prev) => Math.min(prev, idx));
  };

  const closeLeft = (idx: number) => {
    setFiles((prev) => prev.slice(idx));
    setActiveFileIndex((prev) => {
      if (prev < idx) return 0;
      return prev - idx;
    });
  };

  const closeOthers = (idx: number) => {
    setFiles((prev) => [prev[idx]]);
    setActiveFileIndex(0);
  };

  const closeAll = () => {
    setFiles([]);
    setActiveFileIndex(-1);
  };

  const moveTab = (fromIdx: number, toIdx: number) => {
    if (fromIdx === toIdx || fromIdx < 0 || toIdx < 0 || fromIdx >= files.length || toIdx >= files.length) return;
    setFiles((prev) => {
      const next = [...prev];
      const [moved] = next.splice(fromIdx, 1);
      next.splice(toIdx, 0, moved);
      return next;
    });
    setActiveFileIndex((prev) => {
      if (prev === fromIdx) return toIdx;
      if (fromIdx < toIdx && prev > fromIdx && prev <= toIdx) return prev - 1;
      if (fromIdx > toIdx && prev >= toIdx && prev < fromIdx) return prev + 1;
      return prev;
    });
  };

  // Tab context menu state
  const [tabMenu, setTabMenu] = useState<{ x: number; y: number; idx: number } | null>(null);

  // Drag-to-reorder state
  const dragTabRef = useRef<number | null>(null);
  const [dragOverIdx, setDragOverIdx] = useState<number | null>(null);

  async function handleOpenDiff(path: string) {
    try {
      const diff = await GetGitFileDiff("", path);
      globalOpenDiff(path, diff || "", { label: `${path.split("/").pop()} (diff)` });
    } catch (err: any) {
      toast("Failed to load diff: " + err, "danger");
    }
  }

  const handleTabDragStart = (e: React.DragEvent, idx: number) => {
    dragTabRef.current = idx;
    e.dataTransfer.effectAllowed = "move";
    // Ghost image: transparent
    const ghost = document.createElement("div");
    ghost.style.opacity = "0";
    document.body.appendChild(ghost);
    e.dataTransfer.setDragImage(ghost, 0, 0);
    setTimeout(() => document.body.removeChild(ghost), 0);
  };

  const handleTabDragOver = (e: React.DragEvent, idx: number) => {
    e.preventDefault();
    e.dataTransfer.dropEffect = "move";
    setDragOverIdx(idx);
  };

  const handleTabDrop = (e: React.DragEvent, targetIdx: number) => {
    e.preventDefault();
    const fromIdx = dragTabRef.current;
    if (fromIdx === null || fromIdx === targetIdx) {
      setDragOverIdx(null);
      dragTabRef.current = null;
      return;
    }
    setFiles((prev) => {
      const next = [...prev];
      const [moved] = next.splice(fromIdx, 1);
      next.splice(targetIdx, 0, moved);
      return next;
    });
    setActiveFileIndex((prev) => {
      if (prev === fromIdx) return targetIdx;
      if (fromIdx < targetIdx) {
        if (prev > fromIdx && prev <= targetIdx) return prev - 1;
      } else {
        if (prev >= targetIdx && prev < fromIdx) return prev + 1;
      }
      return prev;
    });
    setDragOverIdx(null);
    dragTabRef.current = null;
  };

  const handleTabDragEnd = () => {
    setDragOverIdx(null);
    dragTabRef.current = null;
  };

  // Drag divider between side-by-side panes → redistribute flex shares.
  const startPaneResize = (e: React.MouseEvent, leftId: string, rightId: string) => {
    e.preventDefault();
    e.stopPropagation();
    const container = splitRef.current;
    if (!container) return;
    const rect = container.getBoundingClientRect();
    const l0 = paneShares[leftId] || 1;
    const r0 = paneShares[rightId] || 1;
    const total = l0 + r0;
    const startX = e.clientX;
    const onMove = (ev: MouseEvent) => {
      const delta = ((ev.clientX - startX) / rect.width) * total;
      let l = l0 + delta;
      let r = r0 - delta;
      const min = 0.12;
      if (l < min) { r += l - min; l = min; }
      if (r < min) { l += r - min; r = min; }
      setPaneShare(leftId, l);
      setPaneShare(rightId, r);
    };
    const onUp = () => {
      window.removeEventListener("mousemove", onMove);
      window.removeEventListener("mouseup", onUp);
      document.body.style.cursor = "";
    };
    document.body.style.cursor = "col-resize";
    window.addEventListener("mousemove", onMove);
    window.addEventListener("mouseup", onUp);
  };

  // Create a shell/agent/browser tab from the + button.
  const handleCreateTab = async (kind: "shell" | "agent" | "browser") => {
    setShowCreateModal(false);
    if (kind === "browser") {
      openBrowserTab();
      return;
    }
    try {
      const workspace = useWorkspaceStore.getState().workspace;
      const folder = workspace?.folders?.[0] ?? "";
      const created =
        kind === "shell"
          ? await CreateShell("Shell", folder)
          : await CreateAgentSession("Agent", "coding", folder);
      const newTab = {
        id: created.id,
        name: created.name || (kind === "shell" ? "Shell" : "Agent"),
        path: created.id,
        type: kind as "shell" | "agent",
        content: "",
        modified: false,
      };
      setFiles((prev) => [...prev, newTab]);
      setActiveFileIndex(files.length);
    } catch (err) {
      console.error("Failed to create tab:", err);
    }
  };

  // Close context menu on outside click
  useEffect(() => {
    if (!tabMenu) return;
    const close = () => setTabMenu(null);
    window.addEventListener("click", close);
    window.addEventListener("contextmenu", close);
    return () => {
      window.removeEventListener("click", close);
      window.removeEventListener("contextmenu", close);
    };
  }, [tabMenu]);

  return (
    <div className="flex flex-col h-full bg-[var(--bg-app)] overflow-hidden relative">
      {/* Top Tab Bar */}
      <div
        className="flex items-center justify-start bg-[var(--bg-sidebar)] shrink-0 overflow-x-auto select-none relative"
        onDragLeave={() => setDragOverIdx(null)}
      >
        {files.map((file, i) => (
          <div
            key={file.id}
            draggable
            onDragStart={(e) => handleTabDragStart(e, i)}
            onDragOver={(e) => handleTabDragOver(e, i)}
            onDrop={(e) => handleTabDrop(e, i)}
            onDragEnd={handleTabDragEnd}
            onClick={() => setActiveFileIndex(i)}
            onContextMenu={(e) => {
              e.preventDefault();
              e.stopPropagation();
              setTabMenu({ x: e.clientX, y: e.clientY, idx: i });
            }}
            className={`relative flex items-center gap-1.5 px-3 py-1.5 text-xs border-r border-[var(--border-default)] cursor-pointer whitespace-nowrap group shrink-0 transition-colors ${
              i === activeFileIndex
                ? "bg-[var(--bg-app)] text-[var(--fg-primary)] font-semibold border-b-2 border-b-[var(--accent-primary)]"
                : "text-[var(--fg-secondary)] hover:bg-[var(--bg-surface-hover)]"
            }`}
            style={{ userSelect: "none" }}
          >
            {/* Drop indicator line */}
            {dragOverIdx === i && dragTabRef.current !== i && (
              <span
                className="absolute left-0 top-0 bottom-0 w-0.5 bg-[var(--accent-primary)] rounded-full z-10"
                style={{ pointerEvents: "none" }}
              />
            )}
            {file.type === "file" ? (
              getFileIcon(file.name, "size-3.5")
            ) : file.type === "diff" ? (
              <FileDiff className="size-3.5 text-blue-400" />
            ) : file.type === "shell" ? (
              <span className="text-cyan-400 font-mono text-[10px]">$&gt;</span>
            ) : (
              <span className="text-blue-400 font-mono text-[10px]">🤖</span>
            )}
            <span>{file.name}</span>
            {file.modified && <span className="text-amber-400 text-[10px]">●</span>}
            {file.type === "file" && diffFiles.has(file.path) && (
              <button
                title="Open diff"
                onClick={(e) => {
                  e.stopPropagation();
                  handleOpenDiff(file.path);
                }}
                className="p-0.5 hover:bg-[var(--bg-surface-active)] rounded-sm text-blue-400 ml-1 opacity-0 group-hover:opacity-100 transition-opacity"
              >
                <FileDiff className="size-3" />
              </button>
            )}
            <button
              onClick={(e) => {
                e.stopPropagation();
                closeTab(i);
              }}
              className="p-0.5 hover:bg-[var(--bg-surface-active)] rounded-sm ml-1 opacity-0 group-hover:opacity-100 transition-opacity"
            >
              <X className="size-3" />
            </button>
          </div>
        ))}

        {files.length === 0 && browserTabs.length === 0 && (
          <div className="px-4 py-1.5 text-xs text-[var(--fg-tertiary)] italic">
            No tabs open
          </div>
        )}

        {/* Browser tabs */}
        {browserTabs.map((tab) => (
          <div
            key={tab.id}
            onClick={() => setActiveBrowserTab(tab.id)}
            className={`relative flex items-center gap-1.5 px-3 py-1.5 text-xs border-r border-[var(--border-default)] cursor-pointer whitespace-nowrap group shrink-0 transition-colors ${
              activeBrowserTabId === tab.id
                ? "bg-[var(--bg-app)] text-[var(--fg-primary)] font-semibold border-b-2 border-b-[var(--accent-primary)]"
                : "text-[var(--fg-secondary)] hover:bg-[var(--bg-surface-hover)]"
            }`}
            style={{ userSelect: "none" }}
            title={tab.url || "Browser"}
          >
            <Globe2 className="size-3.5 text-cyan-400" />
            <span className="max-w-[120px] truncate">
              {(() => {
                try {
                  return tab.url ? new URL(tab.url).hostname : "Browser";
                } catch {
                  return tab.url || "Browser";
                }
              })()}
            </span>
            <button
              onClick={(e) => {
                e.stopPropagation();
                closeBrowserTab(tab.id);
              }}
              className="p-0.5 hover:bg-[var(--bg-surface-active)] rounded-sm ml-1 opacity-0 group-hover:opacity-100 transition-opacity"
            >
              <X className="size-3" />
            </button>
          </div>
        ))}

        {/* + button: open new shell / agent / browser */}
        <button
          onClick={() => setShowCreateModal(true)}
          title="Open new tab"
          className="px-2 py-1.5 hover:bg-[var(--bg-surface-hover)] rounded text-[var(--fg-secondary)] hover:text-[var(--fg-primary)] flex items-center cursor-pointer shrink-0 ml-1"
        >
          <Plus className="size-4" />
        </button>

        <div className="flex-1" />

        {/* Layout mode controls — apply to all tab types */}
        <div className="flex items-center space-x-0.5 bg-[var(--bg-panel)] p-0.5 border border-[var(--border-default)] text-xs mr-2 shrink-0">
          <button
            onClick={() => setWorkspaceLayoutMode("single")}
            className={cn(
              "p-1 rounded cursor-pointer",
              workspaceLayoutMode === "single"
                ? "bg-[var(--bg-surface-active)] text-[var(--fg-on-active)]"
                : "text-[var(--fg-tertiary)] hover:text-[var(--fg-primary)]"
            )}
            title="Single panel"
          >
            <Maximize className="size-3.5" />
          </button>
          <button
            onClick={() => setWorkspaceLayoutMode("horizontal")}
            className={cn(
              "p-1 rounded cursor-pointer",
              workspaceLayoutMode === "horizontal"
                ? "bg-[var(--bg-surface-active)] text-[var(--fg-on-active)]"
                : "text-[var(--fg-tertiary)] hover:text-[var(--fg-primary)]"
            )}
            title="Split side-by-side"
          >
            <Columns2 className="size-3.5" />
          </button>
          <button
            onClick={() => setWorkspaceLayoutMode("grid")}
            className={cn(
              "p-1 rounded cursor-pointer",
              workspaceLayoutMode === "grid"
                ? "bg-[var(--bg-surface-active)] text-[var(--fg-on-active)]"
                : "text-[var(--fg-tertiary)] hover:text-[var(--fg-primary)]"
            )}
            title="Grid layout"
          >
            <LayoutGrid className="size-3.5" />
          </button>
        </div>

        {activeFile?.type === "file" &&
          ["html", "htm", "md", "markdown", "mdx"].includes(
            activeFile.name.split(".").pop()?.toLowerCase() || "",
          ) && (
            <div className="ml-auto flex items-center gap-1 pr-2 shrink-0">
              <button
                onClick={() => setPreviewMode("edit")}
                className={cn(
                  "px-2 py-0.5 text-[10px] uppercase tracking-wider rounded border transition-colors",
                  previewMode === "edit"
                    ? "bg-[var(--bg-active)] text-[var(--fg)] border-[var(--border-strong)]"
                    : "text-[var(--fg-tertiary)] border-[var(--border-default)] hover:text-[var(--fg)]",
                )}
              >
                Edit
              </button>
              <button
                onClick={() => setPreviewMode("preview")}
                className={cn(
                  "px-2 py-0.5 text-[10px] uppercase tracking-wider rounded border transition-colors",
                  previewMode === "preview"
                    ? "bg-[var(--bg-active)] text-[var(--fg)] border-[var(--border-strong)]"
                    : "text-[var(--fg-tertiary)] border-[var(--border-default)] hover:text-[var(--fg)]",
                )}
              >
                Preview
              </button>
            </div>
          )}

      </div>

      {/* Tab Context Menu */}
      {tabMenu && (
        <div
          className="fixed z-[9999] min-w-[190px] rounded-lg overflow-hidden shadow-2xl border border-[var(--border-default)] bg-[var(--bg-elevated)] text-[var(--fg-primary)] text-xs py-1"
          style={{ left: tabMenu.x, top: tabMenu.y }}
          onClick={(e) => e.stopPropagation()}
        >
          {[
            { label: "Close", icon: "✕", action: () => { closeTab(tabMenu.idx); setTabMenu(null); } },
            { label: "Close Next Tabs", icon: "→", action: () => { closeRight(tabMenu.idx); setTabMenu(null); }, disabled: tabMenu.idx >= files.length - 1 },
            { label: "Close Prev Tabs", icon: "←", action: () => { closeLeft(tabMenu.idx); setTabMenu(null); }, disabled: tabMenu.idx === 0 },
            null, // separator
            { label: "Move Right", icon: "⇥", action: () => { moveTab(tabMenu.idx, Math.min(files.length - 1, tabMenu.idx + 1)); setTabMenu(null); }, disabled: tabMenu.idx >= files.length - 1 },
            { label: "Move Left", icon: "⇤", action: () => { moveTab(tabMenu.idx, Math.max(0, tabMenu.idx - 1)); setTabMenu(null); }, disabled: tabMenu.idx === 0 },
            { label: "Close Others", icon: "◎", action: () => { closeOthers(tabMenu.idx); setTabMenu(null); }, disabled: files.length <= 1 },
            { label: "Close All", icon: "⊗", action: () => { closeAll(); setTabMenu(null); } },
          ].map((item, k) =>
            item === null ? (
              <div key={k} className="my-1 border-t border-[var(--border-default)]" />
            ) : (
              <button
                key={k}
                disabled={item.disabled}
                onClick={item.action}
                className={`w-full flex items-center gap-2.5 px-3 py-1.5 text-left transition-colors ${
                  item.disabled
                    ? "opacity-35 cursor-not-allowed"
                    : "hover:bg-[var(--bg-surface-hover)] cursor-pointer"
                }`}
              >
                <span className="text-[10px] w-3 text-center text-[var(--fg-tertiary)] shrink-0">{item.icon}</span>
                <span>{item.label}</span>
              </button>
            )
          )}
        </div>
      )}

      {/* Active file path location bar */}
      {activeFile && (
        <div
          className="flex items-center gap-1.5 px-3 py-[3px] bg-[var(--bg-sidebar)] border-b border-[var(--border-default)] text-[10px] font-mono text-[var(--fg-tertiary)] select-none cursor-default group/path shrink-0"
          onClick={() => {
            const p = activeFile.type === "file" ? activeFile.path : activeFile.diffPath || activeFile.conflictPath || activeFile.path;
            if (p) navigator.clipboard.writeText(p).then(() => toast("Path copied to clipboard"));
          }}
          title="Click to copy full path"
        >
          {activeFile.type === "file" ? (
            <FileCode2 className="size-3 text-[var(--fg-tertiary)] shrink-0" />
          ) : (
            <FileText className="size-3 text-[var(--fg-tertiary)] shrink-0" />
          )}
          <span className="truncate">
            {activeFile.type === "file"
              ? activeFile.path
              : activeFile.type === "diff"
                ? activeFile.diffPath || activeFile.path
                : activeFile.conflictPath || activeFile.path}
          </span>
          <span className="ml-auto opacity-0 group-hover/path:opacity-100 text-[9px] uppercase tracking-wider shrink-0">copy path</span>
        </div>
      )}

      <div className="flex-1 overflow-hidden relative">
        {(() => {
          // Every open tab in one ordered list (files first, then browsers).
          const allTabs = [
            ...files.map((f) => ({ kind: "file" as const, id: f.id, file: f })),
            ...browserTabs.map((t) => ({ kind: "browser" as const, id: t.id, tab: t })),
          ];
          const activeBrowserTab = browserTabs.find((t) => t.id === activeBrowserTabId);

          if (allTabs.length === 0) {
            return (
              <div className="flex flex-col items-center justify-center h-full text-center p-6 select-none text-[var(--fg-tertiary)]">
                <FileCode2 className="size-16 stroke-[1.2] text-[var(--fg-disabled)] mb-3 animate-pulse" />
                <h3 className="text-sm font-semibold text-[var(--fg-secondary)]">Forge Workspace Tab Panel</h3>
                <p className="text-xs max-w-xs mt-1">
                  Select files, open terminals, or start assistant chats from the Session Manager in the sidebar.
                </p>
              </div>
            );
          }

          const isTabFocused = (t: typeof allTabs[number]) =>
            t.kind === "browser" ? t.id === activeBrowserTabId : t.id === activeFile?.id;

          const activateTab = (t: typeof allTabs[number]) => {
            if (t.kind === "browser") setActiveBrowserTab(t.id);
            else setActiveFileIndex(files.findIndex((f) => f.id === t.id));
          };

          // Renders a tab's content. Every file gets a persistent FilePane
          // (its own CodeMirror instance) so syntax highlighting, scroll, and
          // undo history survive focus changes — panes are never destroyed on
          // switch, only hidden/shown. Shells/agents/browsers also persist.
          const renderTab = (t: typeof allTabs[number], isFocused: boolean) => {
            if (t.kind === "browser") return <BrowserPanel initialUrl={t.tab.url} />;
            if (t.file.type === "shell") {
              return (
                <div className="flex flex-col h-full w-full bg-[var(--terminal-background)]">
                  <div className="flex items-center justify-between px-3 py-1.5 bg-[var(--bg-sidebar)] border-b border-[var(--border-default)] text-xs text-[var(--fg-secondary)] select-none shrink-0">
                    <span className="flex items-center space-x-1.5">
                      <Terminal className="size-3.5 text-cyan-400" />
                      <span className="font-semibold truncate">{t.file.name}</span>
                    </span>
                    <button
                      onClick={(e) => { e.stopPropagation(); closeTab(files.findIndex((f) => f.id === t.file.id)); }}
                      className="hover:text-white cursor-pointer"
                    >
                      <X className="size-3.5" />
                    </button>
                  </div>
                  <div className="flex-1 min-h-0 min-w-0 overflow-hidden">
                    <TerminalView sessionId={t.file.id} isActive={isFocused} />
                  </div>
                </div>
              );
            }
            if (t.file.type === "agent") return <AgentTabCell sessionId={t.file.id} />;
            // Files, diffs, and conflicts all render through FilePane (diff and
            // conflict tabs get their specialized views inside FilePane).
            return (
              <FilePane
                file={t.file}
                isFocused={isFocused}
                onFocus={() => activateTab(t)}
              />
            );
          };

          // ---- Single-pane mode: render ALL panes but hide the inactive ones
          // with CSS. This keeps every CodeMirror instance alive (no rebuild on
          // switch → no scroll reset, no lost highlight), showing only the
          // focused tab.
          if (workspaceLayoutMode === "single" || allTabs.length === 1) {
            return (
              <div className="h-full w-full relative">
                {allTabs.map((t) => {
                  const focused = isTabFocused(t);
                  return (
                    <div
                      key={t.id}
                      className={cn(
                        "absolute inset-0 overflow-hidden",
                        focused ? "z-10" : "hidden"
                      )}
                    >
                      {renderTab(t, focused)}
                    </div>
                  );
                })}
              </div>
            );
          }

          // ---- Horizontal: side-by-side with proportional flex + drag dividers.
          if (workspaceLayoutMode === "horizontal") {
            const list = allTabs.slice(0, 3);
            return (
              <div ref={splitRef} className="flex flex-row h-full w-full overflow-hidden select-none">
                {list.map((t, idx) => {
                  const share = paneShares[t.id] || 1;
                  const isFocused = isTabFocused(t);
                  return (
                    <React.Fragment key={t.id}>
                      <div
                        onClick={() => activateTab(t)}
                        style={{ flex: `${share} 1 0%`, minWidth: 0 }}
                        className={cn(
                          "h-full overflow-hidden",
                          isFocused && "ring-1 ring-[var(--accent-primary)]/60 z-10"
                        )}
                      >
                        {renderTab(t, isFocused)}
                      </div>
                      {idx < list.length - 1 && (
                        <div
                          onMouseDown={(e) => startPaneResize(e, t.id, list[idx + 1].id)}
                          title="Drag to resize"
                          className="w-1 shrink-0 cursor-col-resize bg-[var(--border-default)] hover:bg-[var(--accent-primary)] z-20"
                        />
                      )}
                    </React.Fragment>
                  );
                })}
              </div>
            );
          }

          // ---- Grid: 2×2 layout.
          return (
            <div className="grid grid-cols-2 grid-rows-2 h-full w-full gap-1 p-1 bg-black/20 overflow-hidden">
              {allTabs.slice(0, 4).map((t) => {
                const isFocused = isTabFocused(t);
                return (
                  <div
                    key={t.id}
                    className={cn(
                      "h-full w-full overflow-hidden border border-[var(--border-default)]",
                      isFocused && "ring-1 ring-[var(--accent-primary)]/60 z-10"
                    )}
                    onClick={() => activateTab(t)}
                  >
                    {renderTab(t, isFocused)}
                  </div>
                );
              })}
            </div>
          );
        })()}
      </div>

      {diffMenu && activeFile?.type === "file" && (
        <DiffGutterMenu
          line={diffMenu.line}
          x={diffMenu.x}
          y={diffMenu.y}
          path={activeFile.path}
          hunks={diffHunks}
          onClose={() => setDiffMenu(null)}
        />
      )}

      {/* Open-new-tab modal: shell / agent / browser */}
      {showCreateModal && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className="bg-[var(--bg-sidebar)] border border-[var(--border-default)] w-full max-w-sm p-4 space-y-3">
            <div className="flex items-center justify-between pb-2 border-b border-[var(--border-default)]">
              <span className="font-bold text-sm text-[var(--fg-primary)]">Open New</span>
              <button
                onClick={() => setShowCreateModal(false)}
                className="text-[var(--fg-tertiary)] hover:text-white cursor-pointer"
              >
                <X className="size-4" />
              </button>
            </div>
            <div className="grid grid-cols-3 gap-2 text-xs font-semibold">
              <button
                onClick={() => handleCreateTab("shell")}
                className="p-3 border border-[var(--border-default)] bg-[var(--bg-panel)] flex flex-col items-center justify-center space-y-1 transition-all hover:border-[var(--accent-primary)] cursor-pointer"
              >
                <Terminal className="size-6 text-cyan-400" />
                <span>Shell</span>
              </button>
              <button
                onClick={() => handleCreateTab("agent")}
                className="p-3 border border-[var(--border-default)] bg-[var(--bg-panel)] flex flex-col items-center justify-center space-y-1 transition-all hover:border-[var(--accent-primary)] cursor-pointer"
              >
                <Bot className="size-6 text-blue-400" />
                <span>Agent</span>
              </button>
              <button
                onClick={() => handleCreateTab("browser")}
                className="p-3 border border-[var(--border-default)] bg-[var(--bg-panel)] flex flex-col items-center justify-center space-y-1 transition-all hover:border-[var(--accent-primary)] cursor-pointer"
              >
                <Globe2 className="size-6 text-cyan-400" />
                <span>Browser</span>
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
// Embedded AI Agent chat client inside tabs — shared AgentChatPanel
function AgentTabCell({ sessionId }: { sessionId: string }) {
  const [sessions, setSessions] = useState<any[]>([]);

  const session = useMemo(() => {
    return sessions.find((s) => s.id === sessionId) || null;
  }, [sessions, sessionId]);

  useEffect(() => {
    loadAgent();
    const unsubscribe = EventsOn("agent:updated", () => loadAgent());
    return () => {
      if (typeof unsubscribe === "function") unsubscribe();
    };
  }, [sessionId]);

  async function loadAgent() {
    try {
      const list = await ListAgentSessions();
      setSessions(Array.isArray(list) ? list : []);
    } catch { /* ignore */ }
  }

  if (!session) return <div className="h-full w-full bg-[var(--bg-app)]" />;
  return <AgentChatPanel session={session} onClose={() => {}} />;
}

// ---------------------------------------------------------------------------
// FilePane — a persistent CodeMirror editor for a single file tab. Each file
// gets its OWN CodeMirror instance that lives as long as the tab is open. It is
// never destroyed on focus change (single mode hides it with CSS, multi-pane
// shows it), so scroll position, syntax highlighting, and undo history persist.
//
// No diff-gutter Compartment is used here: a module-level Compartment can only
// belong to ONE editor state, so sharing it across panes crashed the editor.
// The diff gutter is intentionally a single-view feature only.
// ---------------------------------------------------------------------------
function FilePane({ file, isFocused, onFocus }: {
  file: EditorFile;
  isFocused: boolean;
  onFocus: () => void;
}) {
  const { setFiles } = useEditorStore();
  const paneRef = useRef<HTMLDivElement>(null);
  const paneViewRef = useRef<EditorView | null>(null);

  // Reload evicted content from disk when the store entry was nulled by LRU.
  useEffect(() => {
    if (file.content === null && file.type === "file") {
      ReadFile(file.path)
        .then((c) => {
          const { files, setFiles } = useEditorStore.getState();
          setFiles(files.map((f) => (f.id === file.id ? { ...f, content: c } : f)));
        })
        .catch(() => {});
    }
  }, [file.content, file.path, file.id]);

  // Mount / rebuild CodeMirror for this pane.
  useEffect(() => {
    if (!paneRef.current || file.type !== "file") return;
    const ext = file.name.split(".").pop()?.toLowerCase() || "";
    const isBinary = ["png", "jpg", "jpeg", "gif", "pdf", "ico", "svg"].includes(ext);
    if (file.content === null || isBinary) {
      if (paneViewRef.current) {
        paneViewRef.current.destroy();
        paneViewRef.current = null;
      }
      return;
    }

    // Skip rebuild when the live doc already matches — prevents feedback loops
    // from the debounced store write (which would reset scroll/cursor).
    if (paneViewRef.current && paneViewRef.current.state.doc.toString() === file.content) {
      if (isFocused) setGlobalEditorView(paneViewRef.current);
      return;
    }

    const paneFileId = file.id;
    let contentTimer: ReturnType<typeof setTimeout> | undefined;
    const updateListener = EditorView.updateListener.of((update) => {
      if (update.docChanged) {
        const newContent = update.state.doc.toString();
        setGlobalLiveContent(paneFileId, newContent);
        clearTimeout(contentTimer);
        contentTimer = setTimeout(() => {
          setFiles((prev) =>
            prev.map((f) =>
              f.id === paneFileId && newContent !== f.content
                ? { ...f, content: newContent, modified: newContent !== f.savedContent }
                : f
            )
          );
        }, 250);
      }
    });

    const state = EditorState.create({
      doc: file.content,
      extensions: [
        history(),
        keymap.of(defaultKeymap),
        getLanguageExtension(file.path),
        highlightSelectionMatches({ highlightWordAroundCursor: true }),
        lineNumbers(),
        highlightActiveLineGutter(),
        highlightActiveLine(),
        lineHighlightField,
        oneDark,
        rainbowBracketsPlugin,
        bracketMatching(),
        closeBrackets(),
        keymap.of(closeBracketsKeymap),
        EditorView.lineWrapping,
        updateListener,
      ],
    });

    if (paneViewRef.current) {
      paneViewRef.current.setState(state);
    } else {
      paneViewRef.current = new EditorView({ state, parent: paneRef.current });
    }
    if (isFocused) setGlobalEditorView(paneViewRef.current);

    return () => {
      if (contentTimer) clearTimeout(contentTimer);
    };
  }, [file.id, file.path, file.content, isFocused, setFiles]);

  // Cleanup on unmount (tab closed).
  useEffect(() => {
    return () => {
      if (paneViewRef.current) {
        paneViewRef.current.destroy();
        paneViewRef.current = null;
      }
    };
  }, []);

  return (
    <div
      className="flex flex-col h-full w-full bg-[var(--bg-app)] overflow-hidden"
      onClick={() => isFocused || onFocus()}
    >
      {file.type === "diff" ? (
        <DiffTabView file={file} />
      ) : file.type === "conflict" ? (
        <ConflictTabView file={file} />
      ) : (
        <>
          <div className="flex items-center justify-between px-3 py-1.5 bg-[var(--bg-sidebar)] border-b border-[var(--border-default)] text-[10px] text-[var(--fg-tertiary)] select-none shrink-0">
            <span className="flex items-center gap-1.5 truncate">
              <FileCode2 className="size-3 shrink-0" />
              <span className="truncate font-mono">{file.path}</span>
            </span>
          </div>
          <div ref={paneRef} className="flex-1 min-h-0 min-w-0" />
        </>
      )}
    </div>
  );
}