export interface SyntaxDiagnostic {
  line: number;
  column: number;
  message: string;
  severity: "error" | "warning";
}

export interface SymbolInfo {
  name: string;
  kind: string;
  containerName?: string;
  location: {
    uri: string;
    range: {
      start: { line: number; character: number };
      end: { line: number; character: number };
    };
  };
}

export class EditorManager {
  public checkSyntax(filePath: string, content: string): SyntaxDiagnostic[] {
    const ext = filePath.split(".").pop()?.toLowerCase() || "";
    const diags: SyntaxDiagnostic[] = [];

    if (ext === "json") {
      try {
        JSON.parse(content);
      } catch (err: any) {
        const msg = err.message || "Invalid JSON";
        const match = msg.match(/position (\d+)/);
        let line = 1;
        let column = 1;
        if (match) {
          const pos = parseInt(match[1], 10);
          const lines = content.slice(0, pos).split("\n");
          line = lines.length;
          column = lines[lines.length - 1].length + 1;
        }
        diags.push({
          line,
          column,
          message: msg,
          severity: "error",
        });
      }
    }

    // Basic brace balancing check for JS/TS/Go/Python
    if (["js", "jsx", "ts", "tsx", "go", "c", "cpp", "zig"].includes(ext)) {
      const stack: { char: string; line: number; col: number }[] = [];
      const lines = content.split("\n");
      for (let l = 0; l < lines.length; l++) {
        const lineStr = lines[l];
        for (let c = 0; c < lineStr.length; c++) {
          const ch = lineStr[c];
          if (ch === "{" || ch === "[" || ch === "(") {
            stack.push({ char: ch, line: l + 1, col: c + 1 });
          } else if (ch === "}" || ch === "]" || ch === ")") {
            const last = stack.pop();
            const expected = ch === "}" ? "{" : ch === "]" ? "[" : "(";
            if (!last || last.char !== expected) {
              diags.push({
                line: l + 1,
                column: c + 1,
                message: `Unmatched closing bracket '${ch}'`,
                severity: "error",
              });
              break;
            }
          }
        }
      }
      if (stack.length > 0) {
        const unclosed = stack[stack.length - 1];
        diags.push({
          line: unclosed.line,
          column: unclosed.col,
          message: `Unclosed bracket '${unclosed.char}'`,
          severity: "warning",
        });
      }
    }

    return diags;
  }

  public formatCode(filePath: string, content: string): string {
    const ext = filePath.split(".").pop()?.toLowerCase() || "";
    if (ext === "json") {
      try {
        return JSON.stringify(JSON.parse(content), null, 2);
      } catch {
        return content;
      }
    }
    return content;
  }

  public getCompletion(prefix: string, filePath: string): any[] {
    const commonSnippets = [
      { Name: "console.log", Kind: "snippet", Detail: "console.log(...)" },
      { Name: "function", Kind: "keyword", Detail: "function declaration" },
      { Name: "return", Kind: "keyword", Detail: "return statement" },
      { Name: "interface", Kind: "keyword", Detail: "interface declaration" },
      { Name: "export", Kind: "keyword", Detail: "export statement" },
      { Name: "import", Kind: "keyword", Detail: "import statement" },
      { Name: "async", Kind: "keyword", Detail: "async function" },
      { Name: "await", Kind: "keyword", Detail: "await promise" },
    ];
    return commonSnippets.filter((s) => s.Name.toLowerCase().startsWith(prefix.toLowerCase()));
  }

  public getMembers(instance: string, filePath: string): any[] {
    return [
      { Name: "length", Kind: "property", Detail: "number" },
      { Name: "toString", Kind: "method", Detail: "(): string" },
      { Name: "map", Kind: "method", Detail: "<U>(fn: (x: T) => U): U[]" },
      { Name: "filter", Kind: "method", Detail: "(fn: (x: T) => boolean): T[]" },
      { Name: "slice", Kind: "method", Detail: "(start?: number, end?: number): T[]" },
      { Name: "forEach", Kind: "method", Detail: "(fn: (x: T) => void): void" },
    ];
  }

  public findSymbol(name: string): SymbolInfo[] {
    return [];
  }

  public searchIndexSymbols(query: string): SymbolInfo[] {
    return [];
  }
}
