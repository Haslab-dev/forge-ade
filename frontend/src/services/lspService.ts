export interface LSPDiagnostic {
  id: string;
  filePath: string;
  line: number;
  column: number;
  message: string;
  severity: 'error' | 'warning' | 'info' | 'hint';
  source: string;
  code?: string;
}

export interface LSPHoverInfo {
  title: string;
  signature?: string;
  documentation: string;
  type?: string;
}

export interface LSPCompletionItem {
  label: string;
  kind: 'function' | 'class' | 'variable' | 'keyword' | 'module' | 'property';
  detail: string;
  documentation: string;
  insertText: string;
}

// Built-in Generic LSP symbol dictionary
const HOVER_DB: Record<string, LSPHoverInfo> = {
  'console.log': {
    title: 'console.log(...data: any[]): void',
    signature: 'console.log(...data: any[]): void',
    documentation: 'Prints to stdout with newline. Multiple arguments can be passed.',
    type: 'built-in function'
  },
  'fetch': {
    title: 'fetch(input: RequestInfo | URL, init?: RequestInit): Promise<Response>',
    signature: 'fetch(input: RequestInfo | URL, init?: RequestInit): Promise<Response>',
    documentation: 'Starts the process of fetching a resource from the network, returning a promise.',
    type: 'built-in function'
  },
  'useState': {
    title: 'React.useState<T>(initialState: T | (() => T)): [T, Dispatch<SetStateAction<T>>]',
    signature: 'function useState<S>(initialState: S | (() => S)): [S, Dispatch<SetStateAction<S>>]',
    documentation: 'Returns a stateful value, and a function to update it.',
    type: 'React Hook'
  },
  'useEffect': {
    title: 'React.useEffect(effect: EffectCallback, deps?: DependencyList): void',
    signature: 'function useEffect(effect: () => void | Destructor, deps?: DependencyList): void',
    documentation: 'Accepts a function that contains imperative, possibly effectful code.',
    type: 'React Hook'
  },
  'useCallback': {
    title: 'React.useCallback<T extends Function>(callback: T, deps: DependencyList): T',
    signature: 'function useCallback<T extends Function>(callback: T, deps: DependencyList): T',
    documentation: 'Returns a memoized callback function.',
    type: 'React Hook'
  }
};

export class LSPService {
  /**
   * Run language diagnostics on a given file content
   */
  public static lintFile(filePath: string, content: string): LSPDiagnostic[] {
    const diagnostics: LSPDiagnostic[] = [];
    const lines = content.split('\n');

    // JSON linter
    if (filePath.endsWith('.json')) {
      try {
        JSON.parse(content);
      } catch (err: unknown) {
        const error = err as Error;
        const match = error.message.match(/position (\d+)/);
        let line = 1;
        let col = 1;
        if (match) {
          const pos = parseInt(match[1], 10);
          const substr = content.slice(0, pos);
          line = substr.split('\n').length;
          col = substr.length - substr.lastIndexOf('\n');
        }
        diagnostics.push({
          id: `diag-json-${Date.now()}`,
          filePath,
          line,
          column: col,
          message: error.message || 'Invalid JSON syntax',
          severity: 'error',
          source: 'json-lsp',
          code: 'JSON001'
        });
      }
      return diagnostics;
    }

    // Markdown linter
    if (filePath.endsWith('.md')) {
      lines.forEach((line, index) => {
        const lineNum = index + 1;
        if (/\[.*?\]\(\s*\)/.test(line)) {
          diagnostics.push({
            id: `diag-mdlink-${lineNum}`,
            filePath,
            line: lineNum,
            column: 1,
            message: 'Markdown link URL is empty',
            severity: 'info',
            source: 'markdown-lint',
            code: 'MD042'
          });
        }
      });
    }

    // Generic JS / TS syntax checks
    if (filePath.endsWith('.ts') || filePath.endsWith('.tsx') || filePath.endsWith('.js') || filePath.endsWith('.jsx')) {
      lines.forEach((line, index) => {
        const lineNum = index + 1;
        if (line.includes('debugger')) {
          diagnostics.push({
            id: `diag-debug-${lineNum}`,
            filePath,
            line: lineNum,
            column: line.indexOf('debugger') + 1,
            message: 'Unexpected "debugger" statement',
            severity: 'warning',
            source: 'ts-lint',
            code: 'NO_DEBUGGER'
          });
        }
      });
    }

    return diagnostics;
  }

  /**
   * Get hover tooltip info for symbol at cursor
   */
  public static getHoverInfo(word: string): LSPHoverInfo | null {
    if (!word) return null;
    const cleanWord = word.trim().replace(/[(),;]/g, '');

    for (const key of Object.keys(HOVER_DB)) {
      if (key.toLowerCase() === cleanWord.toLowerCase() || key.endsWith(cleanWord)) {
        return HOVER_DB[key];
      }
    }

    return {
      title: cleanWord,
      documentation: `Symbol defined in current project scope.`,
      type: 'symbol'
    };
  }

  /**
   * Autocompletions list
   */
  public static getCompletions(prefix: string): LSPCompletionItem[] {
    const list: LSPCompletionItem[] = [
      {
        label: 'useState',
        kind: 'function',
        detail: 'React.useState(initialState)',
        documentation: 'Declare state variable in component',
        insertText: 'const [${1:state}, set${1/(.*)/${1:/capitalize}/}] = useState(${2:initial});'
      },
      {
        label: 'useEffect',
        kind: 'function',
        detail: 'React.useEffect(fn, deps)',
        documentation: 'Execute side effect on dependency update',
        insertText: 'useEffect(() => {\n  $0\n}, [${1:deps}]);'
      },
      {
        label: 'console.log',
        kind: 'function',
        detail: 'console.log(...data)',
        documentation: 'Print output to debug terminal',
        insertText: 'console.log($1);'
      },
      {
        label: 'import React',
        kind: 'keyword',
        detail: 'import React from "react"',
        documentation: 'Import React namespace',
        insertText: 'import React from "react";'
      }
    ];

    if (!prefix) return list;
    return list.filter(item => item.label.toLowerCase().includes(prefix.toLowerCase()));
  }
}
