import {
  EditorState,
  EditorSelection,
  SelectionRange,
  StateField,
  StateEffect,
  Prec,
} from "@codemirror/state";
import { EditorView, Command, keymap } from "@codemirror/view";

// ---------------------------------------------------------------------------
// Selection History Stack for "Undo Cursor" (Ctrl+U / Meta+U)
// ---------------------------------------------------------------------------

const pushSelectionHistoryEffect = StateEffect.define<EditorSelection>();
const popSelectionHistoryEffect = StateEffect.define<void>();

export const selectionHistoryField = StateField.define<EditorSelection[]>({
  create() {
    return [];
  },
  update(history, tr) {
    for (const effect of tr.effects) {
      if (effect.is(pushSelectionHistoryEffect)) {
        // Keep a maximum of 50 history entries
        const next = [...history, effect.value];
        if (next.length > 50) next.shift();
        return next;
      }
      if (effect.is(popSelectionHistoryEffect)) {
        if (history.length > 0) {
          return history.slice(0, history.length - 1);
        }
      }
    }
    return history;
  },
});

function recordSelection(view: EditorView) {
  view.dispatch({
    effects: pushSelectionHistoryEffect.of(view.state.selection),
  });
}

// ---------------------------------------------------------------------------
// 1. Adding a Line (Add Cursor Above / Below)
// macOS: Meta+Alt+ArrowUp / Meta+Alt+ArrowDown
// Win/Linux: Ctrl+Alt+ArrowUp / Ctrl+Alt+ArrowDown
// ---------------------------------------------------------------------------

export const addCursorAbove: Command = (view: EditorView) => {
  const { state } = view;
  recordSelection(view);

  const newRanges: SelectionRange[] = [...state.selection.ranges];
  let added = false;

  for (const range of state.selection.ranges) {
    const headLine = state.doc.lineAt(range.head);
    if (headLine.number <= 1) continue;

    const col = range.head - headLine.from;
    const targetLine = state.doc.line(headLine.number - 1);
    const targetPos = Math.min(targetLine.from + col, targetLine.to);

    const anchorCol = range.anchor - headLine.from;
    const targetAnchor = range.empty
      ? targetPos
      : Math.min(targetLine.from + anchorCol, targetLine.to);

    const newRange = EditorSelection.range(targetAnchor, targetPos);
    // Only add if not already covered
    if (!newRanges.some((r) => r.from === newRange.from && r.to === newRange.to)) {
      newRanges.push(newRange);
      added = true;
    }
  }

  if (!added) return false;

  view.dispatch({
    selection: EditorSelection.create(newRanges, newRanges.length - 1),
    scrollIntoView: true,
  });
  return true;
};

export const addCursorBelow: Command = (view: EditorView) => {
  const { state } = view;
  recordSelection(view);

  const newRanges: SelectionRange[] = [...state.selection.ranges];
  let added = false;

  for (const range of state.selection.ranges) {
    const headLine = state.doc.lineAt(range.head);
    if (headLine.number >= state.doc.lines) continue;

    const col = range.head - headLine.from;
    const targetLine = state.doc.line(headLine.number + 1);
    const targetPos = Math.min(targetLine.from + col, targetLine.to);

    const anchorCol = range.anchor - headLine.from;
    const targetAnchor = range.empty
      ? targetPos
      : Math.min(targetLine.from + anchorCol, targetLine.to);

    const newRange = EditorSelection.range(targetAnchor, targetPos);
    if (!newRanges.some((r) => r.from === newRange.from && r.to === newRange.to)) {
      newRanges.push(newRange);
      added = true;
    }
  }

  if (!added) return false;

  view.dispatch({
    selection: EditorSelection.create(newRanges, newRanges.length - 1),
    scrollIntoView: true,
  });
  return true;
};

// ---------------------------------------------------------------------------
// 2. Insert Cursors at End of Line (Alt+Shift+I)
// ---------------------------------------------------------------------------

export const insertCursorsAtEndOfLine: Command = (view: EditorView) => {
  const { state } = view;
  recordSelection(view);

  const newRanges: SelectionRange[] = [];
  const visitedLines = new Set<number>();

  for (const range of state.selection.ranges) {
    const startLine = state.doc.lineAt(range.from);
    const endLine = state.doc.lineAt(range.to);

    for (let l = startLine.number; l <= endLine.number; l++) {
      if (visitedLines.has(l)) continue;
      visitedLines.add(l);
      const line = state.doc.line(l);
      newRanges.push(EditorSelection.cursor(line.to));
    }
  }

  if (newRanges.length === 0) return false;

  view.dispatch({
    selection: EditorSelection.create(newRanges, newRanges.length - 1),
    scrollIntoView: true,
  });
  return true;
};

// ---------------------------------------------------------------------------
// 3. Select Current Line (Ctrl+L / Meta+L)
// ---------------------------------------------------------------------------

export const selectCurrentLine: Command = (view: EditorView) => {
  const { state } = view;
  recordSelection(view);

  const newRanges: SelectionRange[] = [];

  for (const range of state.selection.ranges) {
    const startLine = state.doc.lineAt(range.from);
    const endLine = state.doc.lineAt(range.to);

    // If the full line (including newline) is already selected, expand to next line
    const isFullLineSelected =
      range.from === startLine.from &&
      (range.to === endLine.to + 1 || (endLine.number === state.doc.lines && range.to === endLine.to));

    if (isFullLineSelected && endLine.number < state.doc.lines) {
      const nextLine = state.doc.line(endLine.number + 1);
      const toPos = nextLine.number < state.doc.lines ? nextLine.to + 1 : nextLine.to;
      newRanges.push(EditorSelection.range(startLine.from, toPos));
    } else {
      const toPos = endLine.number < state.doc.lines ? endLine.to + 1 : endLine.to;
      newRanges.push(EditorSelection.range(startLine.from, toPos));
    }
  }

  if (newRanges.length === 0) return false;

  view.dispatch({
    selection: EditorSelection.create(newRanges, 0),
    scrollIntoView: true,
  });
  return true;
};

// ---------------------------------------------------------------------------
// Helper: get word or selection text at cursor
// ---------------------------------------------------------------------------

function getTargetText(state: EditorState, range: SelectionRange): { text: string; from: number; to: number } | null {
  if (!range.empty) {
    return {
      text: state.sliceDoc(range.from, range.to),
      from: range.from,
      to: range.to,
    };
  }

  // Find word boundaries around cursor
  const line = state.doc.lineAt(range.head);
  const offset = range.head - line.from;
  const text = line.text;

  const isWordChar = (c: string) => /[\w$]/.test(c);
  if (!text[offset] && !text[offset - 1]) return null;

  let start = offset;
  let end = offset;

  if (start > 0 && !isWordChar(text[start]) && isWordChar(text[start - 1])) {
    start--;
  }

  while (start > 0 && isWordChar(text[start - 1])) start--;
  while (end < text.length && isWordChar(text[end])) end++;

  if (start === end) return null;

  return {
    text: text.slice(start, end),
    from: line.from + start,
    to: line.from + end,
  };
}

// ---------------------------------------------------------------------------
// 4. Select All Occurrences at Cursor (Ctrl+Shift+L / Meta+Shift+L)
// ---------------------------------------------------------------------------

export const selectAllOccurrences: Command = (view: EditorView) => {
  const { state } = view;
  const main = state.selection.main;
  const target = getTargetText(state, main);
  if (!target || !target.text) return false;

  recordSelection(view);

  const docText = state.doc.toString();
  const searchText = target.text;
  const newRanges: SelectionRange[] = [];

  let idx = docText.indexOf(searchText);
  while (idx !== -1) {
    newRanges.push(EditorSelection.range(idx, idx + searchText.length));
    idx = docText.indexOf(searchText, idx + searchText.length);
  }

  if (newRanges.length === 0) return false;

  view.dispatch({
    selection: EditorSelection.create(newRanges, 0),
    scrollIntoView: true,
  });
  return true;
};

// ---------------------------------------------------------------------------
// 5. Add Next Occurrence (Ctrl+D / Meta+D)
// ---------------------------------------------------------------------------

export const selectNextOccurrence: Command = (view: EditorView) => {
  const { state } = view;
  const main = state.selection.main;

  // If collapsed, first select the word under cursor
  if (main.empty) {
    const target = getTargetText(state, main);
    if (!target || !target.text) return false;
    recordSelection(view);
    view.dispatch({
      selection: EditorSelection.single(target.from, target.to),
      scrollIntoView: true,
    });
    return true;
  }

  const searchText = state.sliceDoc(main.from, main.to);
  if (!searchText) return false;

  recordSelection(view);

  const docText = state.doc.toString();
  const existingRanges = state.selection.ranges;
  const lastRange = existingRanges[existingRanges.length - 1];

  // Search forward from the end of the latest selection
  let searchStart = lastRange.to;
  let nextIdx = docText.indexOf(searchText, searchStart);

  // Wrap around if not found
  if (nextIdx === -1) {
    nextIdx = docText.indexOf(searchText, 0);
  }

  if (nextIdx === -1) return false;

  const nextRange = EditorSelection.range(nextIdx, nextIdx + searchText.length);

  // Check if this range is already selected
  const alreadySelected = existingRanges.some(
    (r) => r.from === nextRange.from && r.to === nextRange.to
  );

  if (alreadySelected && existingRanges.length > 1) {
    // If all are selected, no-op or find first unselected
    let idx = 0;
    while (idx < docText.length) {
      const match = docText.indexOf(searchText, idx);
      if (match === -1) break;
      const r = EditorSelection.range(match, match + searchText.length);
      if (!existingRanges.some((ex) => ex.from === r.from && ex.to === r.to)) {
        const combined = [...existingRanges, r];
        view.dispatch({
          selection: EditorSelection.create(combined, combined.length - 1),
          scrollIntoView: true,
        });
        return true;
      }
      idx = match + searchText.length;
    }
    return false;
  }

  const newRanges = [...existingRanges, nextRange];
  view.dispatch({
    selection: EditorSelection.create(newRanges, newRanges.length - 1),
    scrollIntoView: true,
  });
  return true;
};

// ---------------------------------------------------------------------------
// 6. Skip Next Occurrence (Ctrl+K Ctrl+D / Meta+K Meta+D)
// ---------------------------------------------------------------------------


export const selectSkipOccurrence: Command = (view: EditorView) => {
  const { state } = view;
  const ranges = state.selection.ranges;
  if (ranges.length === 0) return false;

  const main = state.selection.main;
  const searchText = state.sliceDoc(main.from, main.to);
  if (!searchText) return false;

  recordSelection(view);

  const docText = state.doc.toString();
  const lastRange = ranges[ranges.length - 1];

  let nextIdx = docText.indexOf(searchText, lastRange.to);
  if (nextIdx === -1) {
    nextIdx = docText.indexOf(searchText, 0);
  }

  if (nextIdx === -1) return false;

  const nextRange = EditorSelection.range(nextIdx, nextIdx + searchText.length);

  // Replace the last range with nextRange instead of appending
  const newRanges = [...ranges.slice(0, ranges.length - 1), nextRange];
  view.dispatch({
    selection: EditorSelection.create(newRanges, newRanges.length - 1),
    scrollIntoView: true,
  });
  return true;
};

// ---------------------------------------------------------------------------
// 7. Undo Cursor (Ctrl+U / Meta+U)
// ---------------------------------------------------------------------------

export const undoCursor: Command = (view: EditorView) => {
  const history = view.state.field(selectionHistoryField, false);
  if (!history || history.length === 0) {
    // If no custom history, collapse multiple selections to single main selection
    if (view.state.selection.ranges.length > 1) {
      view.dispatch({
        selection: EditorSelection.single(
          view.state.selection.main.anchor,
          view.state.selection.main.head
        ),
        scrollIntoView: true,
      });
      return true;
    }
    return false;
  }

  const prevSelection = history[history.length - 1];
  view.dispatch({
    selection: prevSelection,
    effects: popSelectionHistoryEffect.of(),
    scrollIntoView: true,
  });
  return true;
};

// ---------------------------------------------------------------------------
// Multi-Cursor Keymaps for CodeMirror 6
// ---------------------------------------------------------------------------

export const multiCursorKeymap = Prec.highest(
  keymap.of([
    // Adding a line above / below
    { key: "Alt-ArrowUp", mac: "Alt-Meta-ArrowUp", win: "Ctrl-Alt-ArrowUp", linux: "Ctrl-Alt-ArrowUp", run: addCursorAbove },
    { key: "Alt-ArrowDown", mac: "Alt-Meta-ArrowDown", win: "Ctrl-Alt-ArrowDown", linux: "Ctrl-Alt-ArrowDown", run: addCursorBelow },

    // Insert cursors at end of line
    { key: "Alt-Shift-i", run: insertCursorsAtEndOfLine },
    { key: "Alt-Shift-I", run: insertCursorsAtEndOfLine },

    // Select current line
    { key: "Mod-l", run: selectCurrentLine },
    { key: "Mod-L", run: selectCurrentLine },

    // Select all occurrences
    { key: "Mod-Shift-l", run: selectAllOccurrences },
    { key: "Mod-Shift-L", run: selectAllOccurrences },

    // Add next occurrence
    { key: "Mod-d", run: selectNextOccurrence },
    { key: "Mod-D", run: selectNextOccurrence },

    // Undo cursor
    { key: "Mod-u", run: undoCursor },
    { key: "Mod-U", run: undoCursor },
  ])
);

export function executeOnActiveEditor(command: Command, editorView: EditorView | null): boolean {
  if (!editorView) return false;
  return command(editorView);
}
