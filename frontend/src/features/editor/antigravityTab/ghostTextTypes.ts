export type GhostSource = 'pattern' | 'context' | 'ai';

export interface GhostSuggestion {
  id: string;
  text: string;
  from: number;
  to: number;
  source: GhostSource;
  prefix: string;
}

export interface GhostState {
  suggestion: GhostSuggestion | null;
  isFetching: boolean;
  enabled: boolean;
}

export interface PredictiveContext {
  prefix: string;
  linePrefix: string;
  lineSuffix: string;
  lineNumber: number;
  column: number;
  fileName: string;
  ext: string;
  surroundingCode: string;
  recentIdentifiers: string[];
}
