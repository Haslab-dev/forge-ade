import { describe, it, expect, vi } from 'vitest';
import { enhanceDiagnosticsWithAi } from './diagnosticAutoFix';
import { Diagnostic } from '@codemirror/lint';
import { EditorView } from '@codemirror/view';

describe('diagnosticAutoFix', () => {
  it('adds "💡 Fix with AI" action to diagnostics', () => {
    const rawDiagnostics: Diagnostic[] = [
      {
        from: 10,
        to: 20,
        severity: 'error',
        message: 'Cannot find name "undeclaredVar"'
      }
    ];

    const onFix = vi.fn();
    const enhanced = enhanceDiagnosticsWithAi(rawDiagnostics, onFix);

    expect(enhanced).toHaveLength(1);
    expect(enhanced[0].actions).toBeDefined();
    expect(enhanced[0].actions?.[0].name).toBe('💡 Fix with AI');

    // Trigger action
    enhanced[0].actions?.[0].apply({} as EditorView, 10, 20);
    expect(onFix).toHaveBeenCalledWith(
      expect.objectContaining({
        from: 10,
        to: 20,
        diagnostic: rawDiagnostics[0]
      })
    );
  });

  it('does not duplicate existing Fix with AI action', () => {
    const rawDiagnostics: Diagnostic[] = [
      {
        from: 5,
        to: 10,
        severity: 'warning',
        message: 'Unused variable',
        actions: [
          {
            name: '💡 Fix with AI',
            apply: vi.fn()
          }
        ]
      }
    ];

    const enhanced = enhanceDiagnosticsWithAi(rawDiagnostics, vi.fn());
    expect(enhanced[0].actions).toHaveLength(1);
  });
});
