import { EditorView } from '@codemirror/view';
import { GhostSuggestion, PredictiveContext } from './ghostTextTypes';

/** Extracts words from the document to suggest local variables, methods, and functions. */
export function extractIdentifiers(docText: string, maxTokens = 60): string[] {
  const words = docText.match(/[a-zA-Z_$][a-zA-Z0-9_$]{2,}/g) || [];
  const freqMap = new Map<string, number>();
  for (const w of words) {
    if (w.length > 2 && !['const', 'function', 'import', 'export', 'return', 'class', 'struct', 'interface', 'package'].includes(w)) {
      freqMap.set(w, (freqMap.get(w) || 0) + 1);
    }
  }
  return Array.from(freqMap.entries())
    .sort((a, b) => b[1] - a[1])
    .slice(0, maxTokens)
    .map(([w]) => w);
}

/** Extracts syntactic and surrounding code context from CodeMirror view. */
export function extractPredictiveContext(
  view: EditorView,
  pos: number,
  fileName: string
): PredictiveContext {
  const doc = view.state.doc;
  const line = doc.lineAt(pos);
  const linePrefix = line.text.slice(0, pos - line.from);
  const lineSuffix = line.text.slice(pos - line.from);

  const startLine = Math.max(1, line.number - 5);
  const endLine = Math.min(doc.lines, line.number + 5);
  const surroundingLines: string[] = [];
  for (let i = startLine; i <= endLine; i++) {
    surroundingLines.push(doc.line(i).text);
  }

  const ext = fileName.toLowerCase().includes('.')
    ? fileName.toLowerCase().split('.').pop() || ''
    : '';

  return {
    prefix: doc.sliceString(Math.max(0, pos - 200), pos),
    linePrefix,
    lineSuffix,
    lineNumber: line.number,
    column: pos - line.from,
    fileName,
    ext,
    surroundingCode: surroundingLines.join('\n'),
    recentIdentifiers: extractIdentifiers(doc.sliceString(Math.max(0, pos - 1500), Math.min(doc.length, pos + 1500)))
  };
}

/** Determines if the cursor is followed by a closing bracket, quote, or semicolon where Tab can jump over. */
export function isTabJumpCandidate(lineSuffix: string): boolean {
  if (!lineSuffix) return false;
  const trimmed = lineSuffix.trimStart();
  if (!trimmed) return false;
  const firstChar = trimmed[0];
  return [')', ']', '}', '"', "'", '`', ';', ',', '>'].includes(firstChar);
}

/** Splits the first word/token chunk from ghost text for word-by-word accept (⌘→ / Alt→). */
export function getNextWordChunk(text: string): { chunk: string; remainder: string } {
  if (!text) return { chunk: '', remainder: '' };

  // Match leading whitespace + first word or punctuation cluster
  const match = text.match(/^(\s*[a-zA-Z0-9_$]+|\s*[^a-zA-Z0-9_$\s]+|\s+)/);
  if (match && match[0]) {
    const chunk = match[0];
    const remainder = text.slice(chunk.length);
    return { chunk, remainder };
  }

  return { chunk: text[0], remainder: text.slice(1) };
}

/** Advances the ghost text if the user typed text matching the beginning of the suggestion. */
export function advanceGhostSuggestion(
  current: GhostSuggestion,
  insertedText: string
): GhostSuggestion | null {
  if (!insertedText) return current;
  if (!current.text.startsWith(insertedText)) {
    return null;
  }

  const nextText = current.text.slice(insertedText.length);
  if (!nextText) return null;

  return {
    ...current,
    text: nextText,
    from: current.from + insertedText.length,
    prefix: current.prefix + insertedText
  };
}

/** Local prediction engine analyzing language idioms and document structure. */
export function predictLocal(ctx: PredictiveContext, cursor: number): GhostSuggestion | null {
  const { linePrefix, lineSuffix, ext, recentIdentifiers } = ctx;

  // Don't autocomplete if user is in the middle of existing non-empty trailing text
  if (lineSuffix.trim().length > 0 && !isTabJumpCandidate(lineSuffix)) {
    return null;
  }

  const trimmedPrefix = linePrefix.trim();
  const leadingSpaces = linePrefix.match(/^\s*/)?.[0] || '';

  // 1. Go Idioms
  if (ext === 'go') {
    if (trimmedPrefix === 'if err != nil {') {
      return makeSuggestion(' return nil, err\n' + leadingSpaces + '}', cursor, linePrefix, 'pattern');
    }
    if (trimmedPrefix === 'if err != nil') {
      return makeSuggestion(' {\n' + leadingSpaces + '\treturn nil, err\n' + leadingSpaces + '}', cursor, linePrefix, 'pattern');
    }
    if (/^func\s+\([a-zA-Z0-9_* ]+\)\s+[a-zA-Z0-9_]+\($/.test(trimmedPrefix)) {
      return makeSuggestion('ctx context.Context) error {', cursor, linePrefix, 'pattern');
    }
    if (/^type\s+[A-Z][a-zA-Z0-9_]*\s+$/.test(trimmedPrefix)) {
      return makeSuggestion('struct {\n' + leadingSpaces + '\tID string `json:"id"`\n' + leadingSpaces + '}', cursor, linePrefix, 'pattern');
    }
    if (trimmedPrefix === 'for _,') {
      return makeSuggestion(' item := range items {', cursor, linePrefix, 'pattern');
    }
    if (trimmedPrefix === 'return nil,') {
      return makeSuggestion(' err', cursor, linePrefix, 'pattern');
    }
  }

  // 2. TypeScript / JavaScript / React Idioms
  if (['ts', 'tsx', 'js', 'jsx', 'mjs'].includes(ext)) {
    if (trimmedPrefix === 'const [') {
      const bestId = recentIdentifiers.find(id => id.length > 2 && /^[a-z]/.test(id)) || 'state';
      const cap = bestId[0].toUpperCase() + bestId.slice(1);
      return makeSuggestion(`${bestId}, set${cap}] = useState();`, cursor, linePrefix, 'pattern');
    }
    if (trimmedPrefix === 'useEffect(() => {') {
      return makeSuggestion('\n' + leadingSpaces + '  return () => {};\n' + leadingSpaces + '}, []);', cursor, linePrefix, 'pattern');
    }
    if (trimmedPrefix === 'try {') {
      return makeSuggestion('\n' + leadingSpaces + '} catch (error) {\n' + leadingSpaces + '  console.error(error);\n' + leadingSpaces + '}', cursor, linePrefix, 'pattern');
    }
    if (trimmedPrefix === 'export const') {
      const bestId = recentIdentifiers.find(id => /^[A-Z]/.test(id)) || 'Component';
      return makeSuggestion(` ${bestId} = () => {\n${leadingSpaces}  return null;\n${leadingSpaces}};`, cursor, linePrefix, 'pattern');
    }
    if (trimmedPrefix === 'import {') {
      return makeSuggestion(' useState, useEffect, useMemo } from \'react\';', cursor, linePrefix, 'pattern');
    }
    if (/^interface\s+[A-Z][a-zA-Z0-9_]*$/.test(trimmedPrefix)) {
      return makeSuggestion(' {\n' + leadingSpaces + '  id: string;\n' + leadingSpaces + '}', cursor, linePrefix, 'pattern');
    }
    if (trimmedPrefix === 'console.') {
      return makeSuggestion('log(', cursor, linePrefix, 'pattern');
    }
  }

  // 3. Python Idioms
  if (['py', 'pyw'].includes(ext)) {
    if (trimmedPrefix === 'if __name__ ==') {
      return makeSuggestion(" '__main__':\n" + leadingSpaces + '    main()', cursor, linePrefix, 'pattern');
    }
    if (trimmedPrefix.startsWith('def ') && trimmedPrefix.endsWith('(')) {
      if (linePrefix.includes('    ')) {
        return makeSuggestion('self):', cursor, linePrefix, 'pattern');
      }
      return makeSuggestion('):', cursor, linePrefix, 'pattern');
    }
    if (trimmedPrefix === 'try:') {
      return makeSuggestion('\n' + leadingSpaces + '    pass\n' + leadingSpaces + 'except Exception as e:\n' + leadingSpaces + '    print(f"Error: {e}")', cursor, linePrefix, 'pattern');
    }
    if (/^class\s+[A-Z][a-zA-Z0-9_]*:?$/.test(trimmedPrefix)) {
      const cls = trimmedPrefix.endsWith(':') ? '' : ':';
      return makeSuggestion(`${cls}\n${leadingSpaces}    def __init__(self):\n${leadingSpaces}        pass`, cursor, linePrefix, 'pattern');
    }
  }

  // 4. Rust Idioms
  if (ext === 'rs') {
    if (trimmedPrefix === 'fn main() {') {
      return makeSuggestion('\n' + leadingSpaces + '    println!("Hello, world!");\n' + leadingSpaces + '}', cursor, linePrefix, 'pattern');
    }
    if (trimmedPrefix.startsWith('impl ') && !trimmedPrefix.includes('{')) {
      return makeSuggestion(' {\n' + leadingSpaces + '    \n' + leadingSpaces + '}', cursor, linePrefix, 'pattern');
    }
    if (trimmedPrefix.startsWith('match ') && !trimmedPrefix.includes('{')) {
      return makeSuggestion(' {\n' + leadingSpaces + '    Some(val) => val,\n' + leadingSpaces + '    None => return,\n' + leadingSpaces + '}', cursor, linePrefix, 'pattern');
    }
    if (trimmedPrefix === 'Ok(') {
      return makeSuggestion('())', cursor, linePrefix, 'pattern');
    }
  }

  // 5. Intelligent Multi-Token Identifier Prediction
  // If the user has typed an incomplete token (3+ chars) matching a common identifier in this file
  const lastTokenMatch = linePrefix.match(/([a-zA-Z_$][a-zA-Z0-9_$]{2,})$/);
  if (lastTokenMatch && lastTokenMatch[1]) {
    const typed = lastTokenMatch[1];
    const match = recentIdentifiers.find(id => id.startsWith(typed) && id !== typed && id.length > typed.length);
    if (match) {
      const remaining = match.slice(typed.length);
      return makeSuggestion(remaining, cursor, linePrefix, 'context');
    }
  }

  return null;
}

function makeSuggestion(
  text: string,
  cursor: number,
  prefix: string,
  source: 'pattern' | 'context' | 'ai'
): GhostSuggestion {
  return {
    id: `ghost_${Date.now()}_${Math.random().toString(36).slice(2, 7)}`,
    text,
    from: cursor,
    to: cursor,
    source,
    prefix
  };
}
