import { StateField, StateEffect } from '@codemirror/state';
import { EditorView, Decoration, DecorationSet } from '@codemirror/view';

export interface ActiveDiffDecoration {
  fromLine: number;
  toLine: number;
  addedLines: Set<number>;
  deletedLines: Set<number>;
}

export const setActiveDiffEffect = StateEffect.define<ActiveDiffDecoration | null>();

const addedLineDeco = Decoration.line({
  attributes: { class: 'cm-ai-diff-added bg-emerald-500/15 border-l-2 border-emerald-500 pl-1' }
});

const deletedLineDeco = Decoration.line({
  attributes: { class: 'cm-ai-diff-deleted bg-rose-500/15 border-l-2 border-rose-500 line-through opacity-70 pl-1' }
});

export const inlineDiffField = StateField.define<DecorationSet>({
  create() {
    return Decoration.none;
  },
  update(decorations, tr) {
    for (const effect of tr.effects) {
      if (effect.is(setActiveDiffEffect)) {
        if (!effect.value) {
          return Decoration.none;
        }

        const { fromLine, toLine, addedLines, deletedLines } = effect.value;
        const decos: any[] = [];
        const doc = tr.state.doc;

        for (let l = fromLine; l <= Math.min(toLine, doc.lines); l++) {
          const lineObj = doc.line(l);
          if (addedLines.has(l)) {
            decos.push(addedLineDeco.range(lineObj.from));
          } else if (deletedLines.has(l)) {
            decos.push(deletedLineDeco.range(lineObj.from));
          }
        }

        return Decoration.set(decos, true);
      }
    }

    if (tr.docChanged) {
      return decorations.map(tr.changes);
    }

    return decorations;
  },
  provide: (f) => EditorView.decorations.from(f)
});
