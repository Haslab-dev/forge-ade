import fs from "fs";
import path from "path";
import { isGitIgnored, loadGitignores, type GitignoreSet } from "./explorer";

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
  /** When false, gitignored files/folders are searched too. Default: respect. */
  respectGitignore?: boolean;
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

interface CacheEntry {
  set: GitignoreSet | null;
  at: number;
}

export class SearchManager {
  // .gitignore sets are re-read on every keystroke-driven search; cache them
  // briefly so rapid consecutive searches don't hammer the FS. Short TTL keeps
  // edits to .gitignore responsive.
  private static GITIGNORE_CACHE_TTL_MS = 5_000;
  private giCache = new Map<string, CacheEntry>();

  /** Cached merged gitignore rules for a folder (null when respect is off). */
  private gitignoreFor(folder: string, respect: boolean): GitignoreSet | null {
    if (!respect) return null;
    const cached = this.giCache.get(folder);
    if (cached && Date.now() - cached.at < SearchManager.GITIGNORE_CACHE_TTL_MS) return cached.set;
    const set = loadGitignores(folder);
    this.giCache.set(folder, { set, at: Date.now() });
    return set;
  }

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
      const gi = this.gitignoreFor(folder, opts.respectGitignore !== false);
      this.walkDir(folder, gi, (filePath, isDir) => {
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
      const gi = this.gitignoreFor(folder, opts.respectGitignore !== false);
      this.walkDir(folder, gi, (filePath, isDir) => {
        if (isDir || !isSearchableFile(filePath)) return true;
        try {
          // Scan the raw buffer line-by-line instead of decoding the whole
          // file and split()ing it — avoids a full-content string plus an
          // array of every line per file (major garbage on large trees).
          const buf = fs.readFileSync(filePath);
          let lineStart = 0;
          let lineNo = 0;
          while (lineStart <= buf.length) {
            let lineEnd = buf.indexOf(10, lineStart); // \n
            if (lineEnd === -1) lineEnd = buf.length;
            lineNo++;
            // Non-global regex: .test() stays stateless across lines.
            const line = buf.toString("utf-8", lineStart, lineEnd);
            if (regex.test(line)) {
              results.push({
                path: filePath,
                name: path.basename(filePath),
                isDir: false,
                score: 1,
                line: lineNo,
                snippet: line.trim(),
              });
              if (results.length >= limit) return false;
            }
            lineStart = lineEnd + 1;
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
      const gi = this.gitignoreFor(folder, opts.respectGitignore !== false);
      this.walkDir(folder, gi, (filePath, isDir) => {
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
  /**
   * Depth-first walk. With a GitignoreSet, ignored entries are pruned at the
   * directory level (an ignored folder is never descended into), matching
   * what the explorer shows. Without one, only .git is skipped.
   */
  private walkDir(
    dir: string,
    gi: GitignoreSet | null,
    onFile: (filePath: string, isDir: boolean) => boolean
  ): void {
    const stack = [dir];

    while (stack.length > 0) {
      const current = stack.pop()!;
      try {
        const entries = fs.readdirSync(current, { withFileTypes: true });
        for (const entry of entries) {
          if (entry.name === ".git") continue;
          const fullPath = path.join(current, entry.name);
          const isDir = entry.isDirectory();
          if (gi && isGitIgnored(gi, fullPath, isDir)) continue;

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
