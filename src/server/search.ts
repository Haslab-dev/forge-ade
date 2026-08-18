import fs from "fs";
import path from "path";

export interface RankedResult {
  path: string;
  name: string;
  isDir: boolean;
  score: number;
  line?: number;
  snippet?: string;
}

export interface SearchOptions {
  query: string;
  folder?: string;
  limit?: number;
  caseSensitive?: boolean;
  wholeWord?: boolean;
  isRegex?: boolean;
  includePattern?: string;
  excludePattern?: string;
}

export interface ReplaceOptions extends SearchOptions {
  replacement: string;
}

export interface ReplaceResult {
  filesChanged: number;
  totalReplacements: number;
  files: string[];
}

export class SearchManager {
  public searchFilename(query: string, folderPaths: string[], limit: number = 50): RankedResult[] {
    const results: RankedResult[] = [];
    const qLower = query.toLowerCase();

    for (const folder of folderPaths) {
      if (!fs.existsSync(folder)) continue;
      this.walkDir(folder, (filePath, isDir) => {
        const base = path.basename(filePath);
        if (base.toLowerCase().includes(qLower)) {
          results.push({
            path: filePath,
            name: base,
            isDir,
            score: base.toLowerCase() === qLower ? 100 : 50,
          });
        }
        return results.length < limit;
      });
      if (results.length >= limit) break;
    }

    return results;
  }

  public searchContent(opts: SearchOptions, folderPaths: string[]): RankedResult[] {
    const results: RankedResult[] = [];
    const limit = opts.limit || 100;
    const query = opts.query;
    if (!query) return [];

    let regex: RegExp;
    try {
      regex = new RegExp(opts.isRegex ? query : query.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"), opts.caseSensitive ? "g" : "gi");
    } catch {
      return [];
    }

    for (const folder of folderPaths) {
      if (!fs.existsSync(folder)) continue;
      this.walkDir(folder, (filePath, isDir) => {
        if (isDir) return true;
        try {
          const content = fs.readFileSync(filePath, "utf-8");
          const lines = content.split("\n");
          for (let i = 0; i < lines.length; i++) {
            const line = lines[i];
            if (regex.test(line)) {
              results.push({
                path: filePath,
                name: path.basename(filePath),
                isDir: false,
                score: 1,
                line: i + 1,
                snippet: line.trim(),
              });
              if (results.length >= limit) return false;
            }
          }
        } catch {}
        return results.length < limit;
      });
      if (results.length >= limit) break;
    }

    return results;
  }

  public searchReplaceAll(opts: ReplaceOptions, folderPaths: string[]): ReplaceResult {
    const matchedFiles = new Set<string>();
    let totalReplacements = 0;

    let regex: RegExp;
    try {
      regex = new RegExp(opts.isRegex ? opts.query : opts.query.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"), opts.caseSensitive ? "g" : "gi");
    } catch {
      return { filesChanged: 0, totalReplacements: 0, files: [] };
    }

    for (const folder of folderPaths) {
      if (!fs.existsSync(folder)) continue;
      this.walkDir(folder, (filePath, isDir) => {
        if (isDir) return true;
        try {
          const content = fs.readFileSync(filePath, "utf-8");
          const replaced = content.replace(regex, opts.replacement);
          if (replaced !== content) {
            fs.writeFileSync(filePath, replaced, "utf-8");
            matchedFiles.add(filePath);
            const matches = content.match(regex);
            totalReplacements += matches ? matches.length : 1;
          }
        } catch {}
        return true;
      });
    }

    return {
      filesChanged: matchedFiles.size,
      totalReplacements,
      files: Array.from(matchedFiles),
    };
  }

  private walkDir(dir: string, onFile: (filePath: string, isDir: boolean) => boolean): void {
    const stack = [dir];
    const ignored = new Set([".git", "node_modules", ".zig-cache", "zig-out", "dist", ".native"]);

    while (stack.length > 0) {
      const current = stack.pop()!;
      try {
        const entries = fs.readdirSync(current, { withFileTypes: true });
        for (const entry of entries) {
          if (ignored.has(entry.name)) continue;
          const fullPath = path.join(current, entry.name);
          const isDir = entry.isDirectory();

          const shouldContinue = onFile(fullPath, isDir);
          if (!shouldContinue) return;

          if (isDir) {
            stack.push(fullPath);
          }
        }
      } catch {}
    }
  }
}
