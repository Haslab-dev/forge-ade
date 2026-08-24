import fs from "fs";
import path from "path";

export interface FileInfo {
  path: string;
  name: string;
  isDir: boolean;
  size?: number;
  modTime?: number;
  children?: FileInfo[];
  gitStatus?: string;
  hidden?: boolean;
  gitIgnored?: boolean;
}

// Only .git is unconditionally excluded from the explorer tree. Everything
// else (node_modules, zig-out, .native, ...) stays visible — the gitignore
// parser flags it so the UI can render it dimmed instead of hiding it.
const ALWAYS_SKIP = new Set([".git"]);

// ---------------------------------------------------------------------------
// Gitignore parser
// ---------------------------------------------------------------------------

interface IgnoreRule {
  pattern: RegExp;
  negate: boolean;
  dirOnly: boolean;
}

function parseGitignore(filePath: string): IgnoreRule[] {
  try {
    const lines = fs.readFileSync(filePath, "utf-8").split("\n");
    const rules: IgnoreRule[] = [];
    for (let raw of lines) {
      const line = raw.trimEnd();
      if (!line || line.startsWith("#")) continue;
      const negate = line.startsWith("!");
      let pat = negate ? line.slice(1) : line;
      const dirOnly = pat.endsWith("/");
      if (dirOnly) pat = pat.slice(0, -1);

      // Convert gitignore glob to regex
      let re = pat
        .replace(/[.+^${}()|[\]\\]/g, "\\$&") // escape regex special chars (not * ?)
        .replace(/\\\*/g, "*")                  // restore * for further processing
        .replace(/\\\?/g, "?")                  // restore ?
        .replace(/\*\*/g, "\x00")               // placeholder for **
        .replace(/\*/g, "[^/]*")                // * → any non-slash chars
        .replace(/\?/g, "[^/]")                 // ? → single non-slash char
        .replace(/\x00/g, ".*");                // ** → any chars including /

      // Anchored to root if pattern contains /
      if (pat.includes("/") && !pat.startsWith("/")) {
        re = re; // relative to gitignore location
      }
      rules.push({ pattern: new RegExp(`(^|/)${re}($|/)`, "i"), negate, dirOnly });
    }
    return rules;
  } catch {
    return [];
  }
}

// Walk from the git root (or dir) up to find all .gitignore files,
// return merged rules keyed by the directory they came from.
function findGitRoot(start: string): string | null {
  let cur = start;
  while (true) {
    try {
      const stat = fs.statSync(path.join(cur, ".git"));
      if (stat.isDirectory() || stat.isFile()) return cur;
    } catch {}
    const parent = path.dirname(cur);
    if (parent === cur) return null;
    cur = parent;
  }
}

export interface GitignoreSet {
  root: string;
  // map of absolute dir path → rules parsed from that dir's .gitignore
  rulesByDir: Map<string, IgnoreRule[]>;
}

export function loadGitignores(startDir: string): GitignoreSet | null {
  const root = findGitRoot(startDir) ?? startDir;
  const rulesByDir = new Map<string, IgnoreRule[]>();

  // Collect all .gitignore files from root down to startDir
  const collect = (dir: string) => {
    const gi = path.join(dir, ".gitignore");
    if (fs.existsSync(gi)) {
      const rules = parseGitignore(gi);
      if (rules.length) rulesByDir.set(dir, rules);
    }
  };

  collect(root);
  // Walk subdirs from root toward startDir
  const rel = path.relative(root, startDir);
  if (rel && rel !== ".") {
    const parts = rel.split(path.sep);
    let cur = root;
    for (const part of parts) {
      cur = path.join(cur, part);
      collect(cur);
    }
  }

  return { root, rulesByDir };
}

export function isGitIgnored(gi: GitignoreSet, absPath: string, isDir: boolean): boolean {
  const rel = path.relative(gi.root, absPath).replace(/\\/g, "/");
  const base = rel.slice(rel.lastIndexOf("/") + 1);
  let ignored = false;

  // Apply rules from each .gitignore in order (root first, deepest last).
  // Anchored patterns (containing /) test the path relative to their own
  // .gitignore dir; unanchored patterns match any segment's basename — one
  // regex test per rule instead of two.
  for (const [dir, rules] of gi.rulesByDir) {
    const relToDir = path.relative(dir, absPath).replace(/\\/g, "/");
    if (relToDir.startsWith("..")) continue; // file is above this .gitignore
    for (const rule of rules) {
      if (rule.dirOnly && !isDir) continue;
      const anchored = rule.pattern.source.includes("/");
      if (rule.pattern.test(anchored ? relToDir : base)) {
        ignored = !rule.negate;
      }
    }
  }

  return ignored;
}

// ---------------------------------------------------------------------------
// Explorer
// ---------------------------------------------------------------------------

export class ExplorerManager {
  private showHidden: boolean = true;

  public toggleHiddenFiles(): boolean {
    this.showHidden = !this.showHidden;
    return this.showHidden;
  }

  public getFileTree(rootPaths: string[], depth: number = 2): string {
    const trees: FileInfo[] = [];
    for (const rootPath of rootPaths) {
      const resolved = path.resolve(rootPath);
      if (fs.existsSync(resolved)) {
        const gi = loadGitignores(resolved);
        const info = this.scanNode(resolved, 0, depth, gi);
        if (info) trees.push(info);
      }
    }
    return JSON.stringify(trees);
  }

  public listDirectory(dirPath: string): string {
    const resolved = path.resolve(dirPath);
    if (!fs.existsSync(resolved)) return "[]";
    const gi = loadGitignores(resolved);
    const entries = this.readDirEntries(resolved, 0, 1, gi);
    return JSON.stringify(entries);
  }

  public expandPath(targetPath: string): string {
    return this.listDirectory(targetPath);
  }

  private scanNode(
    currentPath: string,
    currentDepth: number,
    maxDepth: number,
    gi: GitignoreSet | null,
    inheritedIgnored: boolean = false,
    preStat?: fs.Stats
  ): FileInfo | null {
    try {
      const stat = preStat ?? fs.statSync(currentPath);
      const name = path.basename(currentPath) || currentPath;
      const isDir = stat.isDirectory();

      if (!this.showHidden && name.startsWith(".") && name !== "." && name !== "..") {
        return null;
      }

      // Git semantics: everything beneath an ignored path is ignored too —
      // descendants inherit the flag so dimming stays consistent when expanded.
      const gitIgnored = inheritedIgnored || (gi ? isGitIgnored(gi, currentPath, isDir) : false);

      const node: FileInfo = {
        path: currentPath,
        name,
        isDir,
        size: stat.size,
        modTime: Math.floor(stat.mtimeMs),
        hidden: name.startsWith("."),
        gitIgnored,
      };

      if (isDir) {
        if (ALWAYS_SKIP.has(name)) {
          node.children = [];
          return node;
        }
        if (currentDepth < maxDepth) {
          node.children = this.readDirEntries(currentPath, currentDepth + 1, maxDepth, gi, gitIgnored);
        } else {
          node.children = [];
        }
      }

      return node;
    } catch {
      return null;
    }
  }

  private readDirEntries(
    dirPath: string,
    currentDepth: number = 0,
    maxDepth: number = 1,
    gi: GitignoreSet | null = null,
    inheritedIgnored: boolean = false,
  ): FileInfo[] {
    const dirNodes: FileInfo[] = [];
    const fileNodes: FileInfo[] = [];
    try {
      const files = fs.readdirSync(dirPath);
      for (const file of files) {
        if (!this.showHidden && file.startsWith(".")) continue;
        if (ALWAYS_SKIP.has(file)) continue;

        const fullPath = path.join(dirPath, file);
        try {
          const stat = fs.statSync(fullPath);
          const childInfo = this.scanNode(fullPath, currentDepth, maxDepth, gi, inheritedIgnored, stat);
          if (childInfo) {
            if (childInfo.isDir) dirNodes.push(childInfo);
            else fileNodes.push(childInfo);
          }
        } catch {}
      }
    } catch {}

    dirNodes.sort((a, b) => a.name.localeCompare(b.name));
    fileNodes.sort((a, b) => a.name.localeCompare(b.name));
    return [...dirNodes, ...fileNodes];
  }
}
