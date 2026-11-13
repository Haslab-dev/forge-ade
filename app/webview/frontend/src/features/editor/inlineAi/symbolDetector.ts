import { SymbolLensInfo } from './inlineAiTypes';

/**
 * Fast, lightweight regex scanner for symbol declarations across Go, TS/JS, Python, and Rust.
 * Scans the visible document or line slice to generate Code Lenses without heavy AST overhead.
 */
export function detectSymbolsInDoc(docText: string): SymbolLensInfo[] {
  const lines = docText.split('\n');
  const symbols: SymbolLensInfo[] = [];

  let charOffset = 0;
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    const trimmed = line.trim();

    // Skip comments and empty lines
    if (!trimmed || trimmed.startsWith('//') || trimmed.startsWith('#') || trimmed.startsWith('/*') || trimmed.startsWith('*')) {
      charOffset += line.length + 1;
      continue;
    }

    let match: RegExpExecArray | null = null;
    let kind: SymbolLensInfo['kind'] = 'function';
    let name = '';

    // 1. Go methods: func (r *Receiver) MethodName(...)
    if ((match = /^\s*func\s+\([^)]+\)\s+([A-Za-z0-9_]+)\s*\(/i.exec(line))) {
      name = match[1];
      kind = 'method';
    }
    // 2. Go functions: func FunctionName(...)
    else if ((match = /^\s*func\s+([A-Za-z0-9_]+)\s*(?:\[[^\]]*\])?\s*\(/i.exec(line))) {
      name = match[1];
      kind = 'function';
    }
    // 3. Go struct or interface: type Foo struct / type Foo interface
    else if ((match = /^\s*type\s+([A-Za-z0-9_]+)\s+(struct|interface)\b/i.exec(line))) {
      name = match[1];
      kind = match[2] as 'struct' | 'interface';
    }
    // 4. TS / JS function: (export) function foo(...) or (export) async function foo(...)
    else if ((match = /^\s*(?:export\s+)?(?:async\s+)?function\s+([A-Za-z0-9_]+)\s*(?:<[^>]*>)?\s*\(/i.exec(line))) {
      name = match[1];
      kind = 'function';
    }
    // 5. TS / JS arrow function: (export) const foo = (...) => or (export) const foo = async (...) =>
    else if ((match = /^\s*(?:export\s+)?(?:const|let)\s+([A-Za-z0-9_]+)\s*(?::\s*[^=]+)?\s*=\s*(?:async\s*)?(?:\([^)]*\)|[A-Za-z0-9_]+)\s*=>/i.exec(line))) {
      name = match[1];
      kind = 'function';
    }
    // 6. TS / JS / Python class: (export) class Foo
    else if ((match = /^\s*(?:export\s+)?class\s+([A-Za-z0-9_]+)/i.exec(line))) {
      name = match[1];
      kind = 'class';
    }
    // 7. Python def: def foo(...): or async def foo(...):
    else if ((match = /^\s*(?:async\s+)?def\s+([A-Za-z0-9_]+)\s*\(/i.exec(line))) {
      name = match[1];
      kind = 'function';
    }
    // 8. Rust fn: (pub) fn foo(...)
    else if ((match = /^\s*(?:pub(?:\([^)]*\))?\s+)?fn\s+([A-Za-z0-9_]+)\s*\(/i.exec(line))) {
      name = match[1];
      kind = 'function';
    }

    if (name && !['if', 'for', 'switch', 'select', 'while'].includes(name.toLowerCase())) {
      const lineFrom = charOffset;
      const lineTo = charOffset + line.length;
      symbols.push({
        name,
        kind,
        line: i + 1,
        from: lineFrom,
        to: lineTo
      });
    }

    charOffset += line.length + 1;
  }

  return symbols;
}
