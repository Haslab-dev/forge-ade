import { describe, it, expect } from 'vitest';
import { computeLineDiff } from './inlineDiffUtils';

describe('computeLineDiff', () => {
  it('correctly identifies added and deleted lines', () => {
    const original = `func add(a, b int) int {
    return a + b
}`;
    const generated = `func add(a, b int) int {
    // Add two numbers
    return a + b
}`;

    const diff = computeLineDiff(original, generated);
    expect(diff).toEqual([
      { type: 'unchanged', text: 'func add(a, b int) int {' },
      { type: 'added', text: '    // Add two numbers' },
      { type: 'unchanged', text: '    return a + b' },
      { type: 'unchanged', text: '}' }
    ]);
  });

  it('handles completely replaced blocks', () => {
    const original = 'const x = 1;';
    const generated = 'const y = 2;';
    const diff = computeLineDiff(original, generated);
    expect(diff).toEqual([
      { type: 'added', text: 'const y = 2;' },
      { type: 'deleted', text: 'const x = 1;' }
    ]);
  });
});
