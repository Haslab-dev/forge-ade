import { describe, it, expect } from 'vitest';
import {
  extractIdentifiers,
  getNextWordChunk,
  advanceGhostSuggestion,
  isTabJumpCandidate,
  predictLocal
} from './predictiveEngine';
import { PredictiveContext } from './ghostTextTypes';

describe('predictiveEngine', () => {
  it('extracts unique identifiers from code', () => {
    const code = `
      function calculateMetrics(workspaceRoot, activeTabId) {
        const workspaceData = getWorkspace(workspaceRoot);
        return workspaceData.calculateMetrics();
      }
    `;
    const ids = extractIdentifiers(code);
    expect(ids).toContain('calculateMetrics');
    expect(ids).toContain('workspaceRoot');
    expect(ids).toContain('workspaceData');
  });

  it('splits next word chunk properly', () => {
    const { chunk, remainder } = getNextWordChunk('const value = 42;');
    expect(chunk).toBe('const');
    expect(remainder).toBe(' value = 42;');

    const next = getNextWordChunk(' value = 42;');
    expect(next.chunk).toBe(' value');
    expect(next.remainder).toBe(' = 42;');
  });

  it('advances ghost suggestion when user types matching prefix', () => {
    const initial = {
      id: 'g1',
      text: 'return nil, err',
      from: 10,
      to: 10,
      source: 'pattern' as const,
      prefix: 'if err != nil {\n'
    };

    const advanced = advanceGhostSuggestion(initial, 'ret');
    expect(advanced).not.toBeNull();
    expect(advanced?.text).toBe('urn nil, err');
    expect(advanced?.from).toBe(13);

    // Mismatched typing should clear suggestion
    const mismatched = advanceGhostSuggestion(initial, 'xyz');
    expect(mismatched).toBeNull();
  });

  it('detects Tab-jump candidate characters', () => {
    expect(isTabJumpCandidate(');')).toBe(true);
    expect(isTabJumpCandidate('];')).toBe(true);
    expect(isTabJumpCandidate('}')).toBe(true);
    expect(isTabJumpCandidate('";')).toBe(true);
    expect(isTabJumpCandidate('  ;')).toBe(true);
    expect(isTabJumpCandidate('someIdentifier()')).toBe(false);
  });

  it('predicts Go error return pattern', () => {
    const ctx: PredictiveContext = {
      prefix: 'if err != nil {',
      linePrefix: 'if err != nil {',
      lineSuffix: '',
      lineNumber: 12,
      column: 15,
      fileName: 'main.go',
      ext: 'go',
      surroundingCode: 'if err != nil {',
      recentIdentifiers: []
    };

    const pred = predictLocal(ctx, 15);
    expect(pred).not.toBeNull();
    expect(pred?.text).toContain('return nil, err');
  });

  it('predicts React useState hook pattern in TypeScript', () => {
    const ctx: PredictiveContext = {
      prefix: 'const [',
      linePrefix: '  const [',
      lineSuffix: '',
      lineNumber: 5,
      column: 9,
      fileName: 'App.tsx',
      ext: 'tsx',
      surroundingCode: 'function Component() {\n  const [',
      recentIdentifiers: ['query', 'activeTab']
    };

    const pred = predictLocal(ctx, 9);
    expect(pred).not.toBeNull();
    expect(pred?.text).toContain('setQuery] = useState();');
  });

  it('predicts multi-token identifier completion when typing prefix', () => {
    const ctx: PredictiveContext = {
      prefix: 'active',
      linePrefix: '  const current = active',
      lineSuffix: '',
      lineNumber: 8,
      column: 24,
      fileName: 'workspace.ts',
      ext: 'ts',
      surroundingCode: '',
      recentIdentifiers: ['activeWorkspacePath', 'terminalSessions']
    };

    const pred = predictLocal(ctx, 24);
    expect(pred).not.toBeNull();
    expect(pred?.text).toBe('WorkspacePath');
  });
});
