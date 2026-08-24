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

/**
 * Compiles the query into a NON-global RegExp (global flags make .test()
 * stateful via lastIndex, which silently skipped every other line).
 */
function buildSearchRegex(opts: SearchOptions): RegExp | null {
  if (!opts.query) return null;
  let source = opts.isRegex ? opts.query : opts.query.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  if (opts.wholeWord) source = `\\b(?:${source})\\b`;
  try {
    return new RegExp(source, opts.caseSensitive ? "" : "i");
  } catch {
    return null;
  }
}

export interface ReplaceOptions extends SearchOptions {
  replacement: string;
}

export interface ReplaceResult {
  filesChanged: number;
  totalReplacements: number;
  files: string[];
}

// Content search skips obvious binaries and oversized files.
const BINARY_EXTS = new Set([
  "png", "jpg", "jpeg", "gif", "webp", "svg", "ico", "avif", "icns",
  "zip", "gz", "tgz", "xz", "zst", "7z", "rar", "tar", "bz2",
  "wasm", "woff", "woff2", "ttf", "otf", "eot", "mp4", "mp3", "mov",
  "pdf", "exe", "dll", "dylib", "so", "bin",
]);
const MAX_SEARCHABLE_BYTES = 1_000_000;

function isSearchableFile(filePath: string): boolean {
  const ext = path.extname(filePath).slice(1).toLowerCase();
  if (BINARY_EXTS.has(ext)) return false;
  try {
    return fs.statSync(filePath).size <= MAX_SEARCHABLE_BYTES;
  } catch {
    return false;
  }
}

export class SearchManager {
  public searchFilename(query: string, folderPaths: string[], limit: number = 50): RankedResult[] {
    return this.searchFilenameWithOptions({ query, limit }, folderPaths);
  }

  /**
   * Filename / folder-name search. Matches files AND directories so the
   * explorer can offer "open file" or "reveal folder" on a name hit.
   */
  public searchFilenameWithOptions(opts: SearchOptions, folderPaths: string[]): RankedResult[] {
    const results: RankedResult[] = [];
    const limit = opts.limit || 50;
    const regex = buildSearchRegex(opts);
    if (!regex) return results;

    for (const folder of folderPaths) {
      if (!fs.existsSync(folder)) continue;
      this.walkDir(folder, (filePath, isDir) => {
        const base = path.basename(filePath);
        if (regex.test(base)) {
          const exact = !isDir && base.toLowerCase() === opts.query.toLowerCase();
          results.push({
            path: filePath,
            name: base,
            isDir,
            score: exact ? 100 : isDir ? 60 : 50,
          });
        }
        return results.length < limit;
      });
      if (results.length >= limit) break;
    }

    results.sort((a, b) => b.score - a.score || a.path.localeCompare(b.path));
    return results;
  }

  public searchContent(opts: SearchOptions, folderPaths: string[]): RankedResult[] {
    const results: RankedResult[] = [];
    const limit = opts.limit || 100;
    const regex = buildSearchRegex(opts);
    if (!regex) return results;

    for (const folder of folderPaths) {
      if (!fs.existsSync(folder)) continue;
      this.walkDir(folder, (filePath, isDir) => {
        if (isDir || !isSearchableFile(filePath)) return true;
        try {
          const content = fs.readFileSync(filePath, "utf-8");
          const lines = content.split("\n");
          for (let i = 0; i < lines.length; i++) {
            // Non-global regex: .test() stays stateless across lines.
            if (regex.test(lines[i])) {
              results.push({
                path: filePath,
                name: path.basename(filePath),
                isDir: false,
                score: 1,
                line: i + 1,
                snippet: lines[i].trim(),
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
    const base = buildSearchRegex(opts);
    if (!base) {
      return { filesChanged: 0, totalReplacements: 0, files: [] };
    }
    const regex = new RegExp(base.source, base.flags.includes("g") ? base.flags : base.flags + "g");

    for (const folder of folderPaths) {
      if (!fs.existsSync(folder)) continue;
      this.walkDir(folder, (filePath, isDir) => {
        if (isDir || !isSearchableFile(filePath)) return true;
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
