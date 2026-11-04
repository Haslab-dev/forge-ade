import {
  EditorView,
  WidgetType,
  Decoration,
  DecorationSet,
  ViewPlugin,
  ViewUpdate,
  KeyBinding,
  keymap
} from '@codemirror/view';
import { StateField, StateEffect, Prec } from '@codemirror/state';
import { GhostSuggestion, GhostState, PredictiveContext } from './ghostTextTypes';
import {
  extractPredictiveContext,
  predictLocal,
  advanceGhostSuggestion,
  getNextWordChunk,
  isTabJumpCandidate
} from './predictiveEngine';

export class GhostTextWidget extends WidgetType {
  constructor(readonly text: string, readonly source: string) {
    super();
  }

  eq(other: GhostTextWidget) {
    return this.text === other.text && this.source === other.source;
  }

  toDOM() {
    const span = document.createElement('span');
    span.className = 'cm-antigravity-ghost opacity-45 italic select-none pointer-events-none font-mono text-foreground';
    span.setAttribute('aria-hidden', 'true');
    span.textContent = this.text;
    return span;
  }
}

export const setGhostSuggestionEffect = StateEffect.define<GhostSuggestion | null>();
export const setGhostEnabledEffect = StateEffect.define<boolean>();

export const ghostTextStateField = StateField.define<GhostState>({
  create() {
    return {
      suggestion: null,
      isFetching: false,
      enabled: true
    };
  },
  update(state, tr) {
    let nextSuggestion = state.suggestion;
    let nextEnabled = state.enabled;

    for (const effect of tr.effects) {
      if (effect.is(setGhostSuggestionEffect)) {
        nextSuggestion = effect.value;
      } else if (effect.is(setGhostEnabledEffect)) {
        nextEnabled = effect.value;
      }
    }

    if (tr.docChanged && nextSuggestion) {
      let matched = false;
      tr.changes.iterChanges((fromA, toA, fromB, toB, inserted) => {
        const text = inserted.toString();
        if (fromA === nextSuggestion!.from && toA === fromA && text.length > 0) {
          const advanced = advanceGhostSuggestion(nextSuggestion!, text);
          if (advanced) {
            nextSuggestion = advanced;
            matched = true;
          }
        }
      });
      if (!matched && !tr.effects.some(e => e.is(setGhostSuggestionEffect))) {
        nextSuggestion = null;
      }
    } else if (tr.selection && nextSuggestion) {
      const head = tr.newSelection.main.head;
      if (head !== nextSuggestion.from) {
        nextSuggestion = null;
      }
    }

    return {
      ...state,
      suggestion: nextSuggestion,
      enabled: nextEnabled
    };
  },
  provide: (field) =>
    EditorView.decorations.from(field, (ghostState) => {
      const { suggestion, enabled } = ghostState;
      if (!enabled || !suggestion || !suggestion.text) {
        return Decoration.none;
      }
      const widget = Decoration.widget({
        widget: new GhostTextWidget(suggestion.text, suggestion.source),
        side: 1
      });
      return Decoration.set([widget.range(suggestion.from)]);
    })
});

export interface GhostTextPluginOptions {
  getFileName: () => string;
  debounceMs?: number;
  fetchAiSuggestion?: (ctx: PredictiveContext, cursor: number, signal: AbortSignal) => Promise<GhostSuggestion | null>;
}

export function createGhostTextViewPlugin(options: GhostTextPluginOptions) {
  return ViewPlugin.fromClass(
    class {
      private timer: ReturnType<typeof setTimeout> | null = null;
      private abortController: AbortController | null = null;
      private debounceMs = options.debounceMs ?? 300;

      constructor(private view: EditorView) {}

      update(update: ViewUpdate) {
        if (!update.docChanged && !update.selectionSet) return;

        if (this.timer) {
          clearTimeout(this.timer);
          this.timer = null;
        }
        if (this.abortController) {
          this.abortController.abort();
          this.abortController = null;
        }

        const state = update.state.field(ghostTextStateField, false);
        if (state && !state.enabled) return;

        // If selection is not a simple collapsed caret, dismiss
        if (!update.state.selection.main.empty) {
          if (state?.suggestion) {
            this.view.dispatch({ effects: setGhostSuggestionEffect.of(null) });
          }
          return;
        }

        this.timer = setTimeout(() => {
          this.computePrediction();
        }, this.debounceMs);
      }

      computePrediction() {
        const state = this.view.state;
        const main = state.selection.main;
        if (!main.empty) return;

        const cursor = main.head;
        const fileName = options.getFileName();
        const ctx = extractPredictiveContext(this.view, cursor, fileName);

        const suggestion = predictLocal(ctx, cursor);
        if (suggestion) {
          this.view.dispatch({
            effects: setGhostSuggestionEffect.of(suggestion)
          });
          return;
        }

        if (options.fetchAiSuggestion) {
          if (this.abortController) {
            this.abortController.abort();
          }
          this.abortController = new AbortController();
          const signal = this.abortController.signal;

          void options.fetchAiSuggestion(ctx, cursor, signal).then((aiSuggestion) => {
            if (signal.aborted || !aiSuggestion) return;
            if (this.view.state.selection.main.head === cursor) {
              this.view.dispatch({
                effects: setGhostSuggestionEffect.of(aiSuggestion)
              });
            }
          }).catch(() => {});
        }
      }

      destroy() {
        if (this.timer) {
          clearTimeout(this.timer);
        }
        if (this.abortController) {
          this.abortController.abort();
        }
      }
    }
  );
}

/** CodeMirror Keymap handler for Antigravity Tab & Ghost Text. */
export function createGhostTextKeymap(): KeyBinding[] {
  return [
    {
      key: 'Tab',
      run: (view: EditorView) => {
        const state = view.state.field(ghostTextStateField, false);
        const suggestion = state?.suggestion;

        // 1. Accept Ghost Text if active
        if (suggestion && suggestion.text) {
          view.dispatch({
            changes: { from: suggestion.from, insert: suggestion.text },
            selection: { anchor: suggestion.from + suggestion.text.length },
            effects: setGhostSuggestionEffect.of(null)
          });
          return true;
        }

        // 2. Tab-to-Jump delimiter
        const head = view.state.selection.main.head;
        const line = view.state.doc.lineAt(head);
        const lineSuffix = line.text.slice(head - line.from);

        if (isTabJumpCandidate(lineSuffix)) {
          const trimmedLead = lineSuffix.match(/^\s*/)?.[0]?.length || 0;
          const jumpDist = trimmedLead + 1;
          view.dispatch({
            selection: { anchor: head + jumpDist }
          });
          return true;
        }

        return false;
      }
    },
    {
      key: 'Mod-ArrowRight',
      run: (view: EditorView) => {
        const state = view.state.field(ghostTextStateField, false);
        const suggestion = state?.suggestion;
        if (!suggestion || !suggestion.text) return false;

        const { chunk, remainder } = getNextWordChunk(suggestion.text);
        if (!chunk) return false;

        const newFrom = suggestion.from + chunk.length;
        const updatedSuggestion: GhostSuggestion | null = remainder
          ? {
              ...suggestion,
              text: remainder,
              from: newFrom,
              prefix: suggestion.prefix + chunk
            }
          : null;

        view.dispatch({
          changes: { from: suggestion.from, insert: chunk },
          selection: { anchor: newFrom },
          effects: setGhostSuggestionEffect.of(updatedSuggestion)
        });
        return true;
      }
    },
    {
      key: 'Alt-ArrowRight',
      run: (view: EditorView) => {
        const state = view.state.field(ghostTextStateField, false);
        const suggestion = state?.suggestion;
        if (!suggestion || !suggestion.text) return false;

        const { chunk, remainder } = getNextWordChunk(suggestion.text);
        if (!chunk) return false;

        const newFrom = suggestion.from + chunk.length;
        const updatedSuggestion: GhostSuggestion | null = remainder
          ? {
              ...suggestion,
              text: remainder,
              from: newFrom,
              prefix: suggestion.prefix + chunk
            }
          : null;

        view.dispatch({
          changes: { from: suggestion.from, insert: chunk },
          selection: { anchor: newFrom },
          effects: setGhostSuggestionEffect.of(updatedSuggestion)
        });
        return true;
      }
    },
    {
      key: 'Escape',
      run: (view: EditorView) => {
        const state = view.state.field(ghostTextStateField, false);
        if (state?.suggestion) {
          view.dispatch({ effects: setGhostSuggestionEffect.of(null) });
          return true;
        }
        return false;
      }
    }
  ];
}

/** Complete extension bundle for Antigravity Tab Autocomplete. */
export function createAntigravityTabExtension(options: GhostTextPluginOptions) {
  return [
    ghostTextStateField,
    createGhostTextViewPlugin(options),
    Prec.highest(keymap.of(createGhostTextKeymap()))
  ];
}
