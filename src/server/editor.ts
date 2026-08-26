import fs from "fs";
import path from "path";
import { formatCode as runPrettier } from "./formatter";
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


  /**
   * Formats via the project's own prettier + config files. JSON keeps a
   * dependency-free fallback when no prettier is resolvable. Returns the
   * original content unchanged when formatting is unavailable or fails.
   */
  public async formatCode(filePath: string, content: string, overrides?: { tabWidth?: number; useTabs?: boolean }): Promise<string> {
    const formatted = await runPrettier(filePath, content, overrides);
    if (formatted !== null) return formatted;
    const ext = filePath.split(".").pop()?.toLowerCase() || "";
    if (ext === "json") {
      try {
        return JSON.stringify(JSON.parse(content), null, overrides?.tabWidth ?? 2);
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

  public findSymbol(name: string, folderPaths: string[] = [process.cwd()]): SymbolInfo[] {
    return this.searchIndexSymbols(name, folderPaths);
  }

  public searchIndexSymbols(query: string, folderPaths: string[] = [process.cwd()]): SymbolInfo[] {
    const symbols: SymbolInfo[] = [];
    const qLower = query.toLowerCase();

    const patterns = [
      { regex: /(?:export\s+)?(?:default\s+)?(?:async\s+)?function\s+([A-Za-z0-9_$]+)/, kind: "Function" },
      { regex: /(?:export\s+)?(?:class|struct|enum|interface|type)\s+([A-Za-z0-9_$]+)/, kind: "Class" },
      { regex: /(?:export\s+)?(?:const|let|var|val)\s+([A-Za-z0-9_$]+)\s*[:=]/, kind: "Variable" },
      { regex: /^\s*fn\s+([A-Za-z0-9_$]+)/, kind: "Function" },
      { regex: /^\s*pub\s+fn\s+([A-Za-z0-9_$]+)/, kind: "Function" },
      { regex: /^\s*def\s+([A-Za-z0-9_$]+)/, kind: "Function" },
      { regex: /^\s*func\s+(?:\([^)]+\)\s*)?([A-Za-z0-9_$]+)/, kind: "Function" },
    ];

    const validExts = new Set([
      ".ts", ".tsx", ".js", ".jsx", ".go", ".rs", ".py", ".c", ".cpp", ".h", ".hpp", ".java", ".zig", ".dart", ".kt", ".swift", ".php"
    ]);

    const walk = (dir: string, depth = 0) => {
      if (depth > 6) return;
      let entries: fs.Dirent[] = [];
      try {
        entries = fs.readdirSync(dir, { withFileTypes: true });
      } catch {
        return;
      }

      for (const entry of entries) {
        if (symbols.length >= 100) return;
        const name = entry.name;
        if (
          name.startsWith(".") ||
          name === "node_modules" ||
          name === "dist" ||
          name === "target" ||
          name === "build" ||
          name === "zig-out" ||
          name === "zig-cache" ||
          name === "vendor" ||
          name === ".git" ||
          name === ".next" ||
          name === ".turbo" ||
          name === "__pycache__" ||
          name === "coverage"
        ) {
          continue;
        }
        const fullPath = path.join(dir, entry.name);
        if (entry.isDirectory()) {
          walk(fullPath, depth + 1);
        } else if (entry.isFile()) {
          const ext = path.extname(entry.name).toLowerCase();
          if (!validExts.has(ext)) continue;

          try {
            const content = fs.readFileSync(fullPath, "utf-8");
            const lines = content.split("\n");
            for (let i = 0; i < Math.min(lines.length, 2000); i++) {
              const line = lines[i];
              for (const p of patterns) {
                const m = line.match(p.regex);
                if (m && m[1]) {
                  const symName = m[1];
                  if (!query || symName.toLowerCase().includes(qLower)) {
                    const col = line.indexOf(symName);
                    symbols.push({
                      name: symName,
                      kind: p.kind,
                      containerName: path.basename(fullPath),
                      location: {
                        uri: fullPath,
                        range: {
                          start: { line: i + 1, character: col >= 0 ? col : 0 },
                          end: { line: i + 1, character: col >= 0 ? col + symName.length : symName.length },
                        },
                      },
                    });
                    if (symbols.length >= 100) return;
                  }
                  break;
                }
              }
            }
          } catch {
            // Ignore unreadable files
          }
        }
      }
    };

    for (const folder of folderPaths) {
      walk(folder);
      if (symbols.length >= 100) break;
    }

    return symbols;
  }
}
