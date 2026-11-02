import { Diagnostic, Action } from '@codemirror/lint';
import { EditorView } from '@codemirror/view';

export interface FixDiagnosticPayload {
  diagnostic: Diagnostic;
  view: EditorView;
  from: number;
  to: number;
}

/** Wraps CodeMirror diagnostics with an interactive '💡 Fix with AI' action. */
export function enhanceDiagnosticsWithAi(
  diagnostics: Diagnostic[],
  onFix: (payload: FixDiagnosticPayload) => void
): Diagnostic[] {
  return diagnostics.map((d) => {
    const existingActions = d.actions || [];
    const hasFixAction = existingActions.some((a) => a.name.includes('Fix with AI'));

    if (hasFixAction) {
      return d;
    }

    const aiAction: Action = {
      name: '💡 Fix with AI',
      apply: (view: EditorView, from: number, to: number) => {
        onFix({ diagnostic: d, view, from, to });
      }
    };

    return {
      ...d,
      actions: [aiAction, ...existingActions]
    };
  });
}
