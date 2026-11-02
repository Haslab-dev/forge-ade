export type InlineAiStatus = 'idle' | 'generating' | 'diff' | 'error';

export type CodeLensAction = 'refactor' | 'test' | 'explain' | 'optimize' | 'fix';

export interface InlineAiSessionState {
  isOpen: boolean;
  status: InlineAiStatus;
  prompt: string;
  from: number;
  to: number;
  startLine: number;
  endLine: number;
  originalText: string;
  generatedText: string;
  error?: string;
  actionKind?: CodeLensAction | 'custom';
  targetSymbol?: string;
}

export interface DiffHunkLine {
  type: 'added' | 'deleted' | 'unchanged';
  text: string;
}

export interface SymbolLensInfo {
  name: string;
  kind: 'function' | 'method' | 'class' | 'struct' | 'interface';
  line: number;
  from: number;
  to: number;
}
