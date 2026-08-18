import {
  EditorView,
  hoverTooltip,
  Tooltip,
  keymap,
  Command,
} from "@codemirror/view";
import { EditorState, Extension, Prec } from "@codemirror/state";
import { autocompletion, Completion, CompletionContext, CompletionResult } from "@codemirror/autocomplete";
import {
  linter,
  lintGutter,
  setDiagnostics,
  Diagnostic as CMDeliveryDiagnostic,
} from "@codemirror/lint";
import { marked } from "marked";
import {
  LSPDidOpen,
  LSPDidChange,
  LSPDidSave,
  LSPDidClose,
  LSPGetCompletion,
  LSPGetHover,
  LSPGetDefinition,
  LSPGetDeclaration,
  LSPGetTypeDefinition,
  LSPGetImplementation,
} from "./wails";
import { useLSPStore } from "./lsp-store";
import { getLanguageMeta } from "./languages";

// Map LSP CompletionItemKind (1-25) to CodeMirror completion type string
const LSP_KIND_MAP: Record<number, string> = {
  1: "text",
  2: "method",
  3: "function",
  4: "class", // constructor
  5: "property", // field
  6: "variable",
  7: "class",
  8: "interface",
  9: "namespace", // module
  10: "property",
  11: "keyword", // unit
  12: "constant", // value
  13: "enum",
  14: "keyword",
  15: "snippet",
  16: "color",
  17: "file",
  18: "reference",
  19: "folder",
  20: "enum", // enumMember
  21: "constant",
  22: "type", // struct
  23: "variable", // event
  24: "operator",
  25: "type", // typeParameter
};

// ---------------------------------------------------------------------------
// 1. LSP Autocompletion Provider
// ---------------------------------------------------------------------------

export function createLSPCompletionSource(filePath: string) {
  return async (context: CompletionContext): Promise<CompletionResult | null> => {
    const { state, pos } = context;
    const line = state.doc.lineAt(pos);
    const lineNum0 = line.number - 1;
    const charNum0 = pos - line.from;

    // Match preceding word or trigger characters (., :, >, ", /, @)
    const word = context.matchBefore(/[\w$]+/);
    if (!word && !context.explicit) {
      const prevChar = line.text[charNum0 - 1];
      if (!prevChar || !/[.:>"/@\\]/.test(prevChar)) {
        return null;
      }
    }

    try {
      const items = await LSPGetCompletion(filePath, lineNum0, charNum0);
      if (items && items.length > 0) {
        const options: Completion[] = items.map((item: any) => {
          let documentation: string | undefined;
          if (item.documentation) {
            if (typeof item.documentation === "string") {
              documentation = item.documentation;
            } else if (item.documentation.value) {
              documentation = item.documentation.value;
            }
          }

          const kindNum = typeof item.kind === "number" ? item.kind : 0;
          const typeStr = LSP_KIND_MAP[kindNum] || "variable";

          return {
            label: item.label,
            type: typeStr,
            detail: item.detail,
            info: documentation
              ? () => {
                  const dom = document.createElement("div");
                  dom.className = "cm-lsp-hover-content prose prose-invert text-xs max-w-sm p-2 overflow-y-auto max-h-48 font-sans";
                  try {
                    dom.innerHTML = marked.parse(documentation || "", { async: false }) as string;
                  } catch {
                    dom.textContent = documentation || "";
                  }
                  return dom;
                }
              : undefined,
            apply: item.insertText || item.label,
            boost: item.sortText ? 100 - parseInt(item.sortText.slice(0, 2), 10) || 0 : undefined,
          };
        });

        const from = word ? word.from : pos;
        return {
          from,
          options,
          validFor: /^[\w$]*$/,
        };
      }
    } catch {}

    // Fallback: extract symbols and keywords from active document & language
    if (word && (word.text.length >= 1 || context.explicit)) {
      const prefix = word.text.toLowerCase();
      const seen = new Set<string>();
      const options: Completion[] = [];

      // Collect words from visible lines around cursor
      const docText = state.doc.toString();
      const idRegex = /[A-Za-z_$][A-Za-z0-9_$]{2,}/g;
      let match: RegExpExecArray | null;
      while ((match = idRegex.exec(docText)) !== null) {
        const w = match[0];
        if (w.toLowerCase().startsWith(prefix) && w !== word.text && !seen.has(w)) {
          seen.add(w);
          options.push({
            label: w,
            type: "variable",
          });
          if (options.length >= 40) break;
        }
      }

      if (options.length > 0) {
        return {
          from: word.from,
          options,
          validFor: /^[\w$]*$/,
        };
      }
    }

    return null;
  };
}

// ---------------------------------------------------------------------------
// 2. LSP Hover Tooltip
// ---------------------------------------------------------------------------

export function createLSPHoverExtension(filePath: string): Extension {
  return hoverTooltip(async (view, pos, side) => {
    const { state } = view;
    const line = state.doc.lineAt(pos);
    const lineNum0 = line.number - 1;
    const charNum0 = pos - line.from;

    try {
      const hoverData = await LSPGetHover(filePath, lineNum0, charNum0);
      if (!hoverData || !hoverData.contents) return null;

      let markdownContent = "";
      if (typeof hoverData.contents === "string") {
        markdownContent = hoverData.contents;
      } else if (Array.isArray(hoverData.contents)) {
        markdownContent = hoverData.contents
          .map((c: string | { language?: string; value: string }) =>
            typeof c === "string" ? c : `\`\`\`${c.language || ""}\n${c.value}\n\`\`\``
          )
          .join("\n\n");
      } else if (hoverData.contents && typeof hoverData.contents === "object" && "value" in hoverData.contents && typeof hoverData.contents.value === "string") {
        markdownContent = hoverData.contents.value;
      }

      if (!markdownContent.trim()) return null;

      let startPos = pos;
      let endPos = pos;
      if (hoverData.range) {
        const startLine = state.doc.line(Math.min(hoverData.range.start.line + 1, state.doc.lines));
        startPos = Math.min(startLine.from + hoverData.range.start.character, state.doc.length);
        const endLine = state.doc.line(Math.min(hoverData.range.end.line + 1, state.doc.lines));
        endPos = Math.min(endLine.from + hoverData.range.end.character, state.doc.length);
      }

      return {
        pos: startPos,
        end: endPos > startPos ? endPos : undefined,
        above: true,
        create(view) {
          const dom = document.createElement("div");
          dom.className = "cm-lsp-hover-tooltip border border-[var(--border-default)] bg-[var(--bg-elevated)] shadow-2xl p-2.5 rounded-md text-xs text-[var(--fg-primary)] max-w-md max-h-60 overflow-y-auto font-sans leading-relaxed select-text";
          try {
            dom.innerHTML = marked.parse(markdownContent, { async: false }) as string;
          } catch {
            dom.textContent = markdownContent;
          }
          return { dom };
        },
      };
    } catch {
      return null;
    }
  });
}

// ---------------------------------------------------------------------------
// 3. LSP Diagnostics & Linter Extension
// ---------------------------------------------------------------------------

export function createLSPDiagnosticsLinter(filePath: string): Extension {
  return [
    lintGutter(),
    linter((view) => {
      const doc = view.state.doc;
      const summaries = useLSPStore.getState().diagnostics;
      const fileData = summaries[filePath];
      if (!fileData || !fileData.diagnostics || fileData.diagnostics.length === 0) {
        return [];
      }

      const results: CMDeliveryDiagnostic[] = [];
      for (const diag of fileData.diagnostics) {
        try {
          const startLineNum = Math.max(1, Math.min(diag.range.start.line + 1, doc.lines));
          const startLine = doc.line(startLineNum);
          const from = Math.min(startLine.from + diag.range.start.character, doc.length);

          const endLineNum = Math.max(1, Math.min(diag.range.end.line + 1, doc.lines));
          const endLine = doc.line(endLineNum);
          let to = Math.min(endLine.from + diag.range.end.character, doc.length);

          if (to <= from) {
            to = Math.min(from + 1, doc.length);
          }

          const severityStr: "error" | "warning" | "info" =
            diag.severity === 1 ? "error" : diag.severity === 2 ? "warning" : "info";

          results.push({
            from,
            to,
            severity: severityStr,
            message: diag.message,
            source: diag.source || "LSP",
          });
        } catch {}
      }

      return results;
    }, { delay: 100 }),
  ];
}

export function syncLSPDiagnosticsToView(view: EditorView | null, filePath: string): void {
  if (!view) return;
  const doc = view.state.doc;
  const summaries = useLSPStore.getState().diagnostics;
  const fileData = summaries[filePath];
  const diags = fileData?.diagnostics || [];
  const cmDiags: CMDeliveryDiagnostic[] = [];

  for (const diag of diags) {
    try {
      const startLineNum = Math.max(1, Math.min(diag.range.start.line + 1, doc.lines));
      const startLine = doc.line(startLineNum);
      const from = Math.min(startLine.from + diag.range.start.character, doc.length);

      const endLineNum = Math.max(1, Math.min(diag.range.end.line + 1, doc.lines));
      const endLine = doc.line(endLineNum);
      let to = Math.min(endLine.from + diag.range.end.character, doc.length);

      if (to <= from) {
        to = Math.min(from + 1, doc.length);
      }

      const severityStr: "error" | "warning" | "info" =
        diag.severity === 1 ? "error" : diag.severity === 2 ? "warning" : "info";

      cmDiags.push({
        from,
        to,
        severity: severityStr,
        message: diag.message,
        source: diag.source || "LSP",
      });
    } catch {}
  }

  try {
    view.dispatch(setDiagnostics(view.state, cmDiags));
  } catch {}
}

// ---------------------------------------------------------------------------
// 4. LSP Navigation Commands (Go to Definition / Declaration / Type / Impl)
// ---------------------------------------------------------------------------

export async function executeLSPGoToDefinition(
  filePath: string,
  view: EditorView,
  openFileCallback: (path: string, opts?: { line?: number }) => void
): Promise<boolean> {
  const { state } = view;
  const pos = state.selection.main.head;
  const line = state.doc.lineAt(pos);
  const lineNum0 = line.number - 1;
  const charNum0 = pos - line.from;

  try {
    const locs = await LSPGetDefinition(filePath, lineNum0, charNum0);
    if (!locs || locs.length === 0) return false;

    const target = locs[0];
    openFileCallback(target.uri, { line: target.range.start.line + 1 });
    return true;
  } catch {
    return false;
  }
}

export async function executeLSPGoToDeclaration(
  filePath: string,
  view: EditorView,
  openFileCallback: (path: string, opts?: { line?: number }) => void
): Promise<boolean> {
  const { state } = view;
  const pos = state.selection.main.head;
  const line = state.doc.lineAt(pos);
  const lineNum0 = line.number - 1;
  const charNum0 = pos - line.from;

  try {
    const locs = await LSPGetDeclaration(filePath, lineNum0, charNum0);
    if (!locs || locs.length === 0) return false;

    const target = locs[0];
    openFileCallback(target.uri, { line: target.range.start.line + 1 });
    return true;
  } catch {
    return false;
  }
}

export async function executeLSPGoToTypeDefinition(
  filePath: string,
  view: EditorView,
  openFileCallback: (path: string, opts?: { line?: number }) => void
): Promise<boolean> {
  const { state } = view;
  const pos = state.selection.main.head;
  const line = state.doc.lineAt(pos);
  const lineNum0 = line.number - 1;
  const charNum0 = pos - line.from;

  try {
    const locs = await LSPGetTypeDefinition(filePath, lineNum0, charNum0);
    if (!locs || locs.length === 0) return false;

    const target = locs[0];
    openFileCallback(target.uri, { line: target.range.start.line + 1 });
    return true;
  } catch {
    return false;
  }
}

export async function executeLSPGoToImplementation(
  filePath: string,
  view: EditorView,
  openFileCallback: (path: string, opts?: { line?: number }) => void
): Promise<boolean> {
  const { state } = view;
  const pos = state.selection.main.head;
  const line = state.doc.lineAt(pos);
  const lineNum0 = line.number - 1;
  const charNum0 = pos - line.from;

  try {
    const locs = await LSPGetImplementation(filePath, lineNum0, charNum0);
    if (!locs || locs.length === 0) return false;

    const target = locs[0];
    openFileCallback(target.uri, { line: target.range.start.line + 1 });
    return true;
  } catch {
    return false;
  }
}

// ---------------------------------------------------------------------------
// 5. LSP Keymap (F12, Cmd+F12, etc.)
// ---------------------------------------------------------------------------

export function createLSPNavigationKeymap(
  filePath: string,
  openFileCallback: (path: string, opts?: { line?: number }) => void
): Extension {
  return Prec.high(
    keymap.of([
      {
        key: "F12",
        run: (view) => {
          executeLSPGoToDefinition(filePath, view, openFileCallback);
          return true;
        },
      },
      {
        key: "Mod-F12",
        run: (view) => {
          executeLSPGoToImplementation(filePath, view, openFileCallback);
          return true;
        },
      },
    ])
  );
}
