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
}

const DEFAULT_IGNORED = new Set([
  ".git",
  "node_modules",
  ".zig-cache",
  "zig-out",
  ".native",
  "dist",
  ".DS_Store",
]);

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
        const info = this.scanNode(resolved, 0, depth);
        if (info) trees.push(info);
      }
    }
    return JSON.stringify(trees);
  }

  public listDirectory(dirPath: string): string {
    const resolved = path.resolve(dirPath);
    if (!fs.existsSync(resolved)) return "[]";
    const entries = this.readDirEntries(resolved);
    return JSON.stringify(entries);
  }

  public expandPath(targetPath: string): string {
    return this.listDirectory(targetPath);
  }

  private scanNode(currentPath: string, currentDepth: number, maxDepth: number): FileInfo | null {
    try {
      const stat = fs.statSync(currentPath);
      const name = path.basename(currentPath) || currentPath;
      const isDir = stat.isDirectory();

      if (!this.showHidden && name.startsWith(".") && name !== "." && name !== "..") {
        return null;
      }

      const node: FileInfo = {
        path: currentPath,
        name,
        isDir,
        size: stat.size,
        modTime: Math.floor(stat.mtimeMs),
      };

      if (isDir) {
        if (DEFAULT_IGNORED.has(name)) {
          node.children = [];
          return node;
        }

        if (currentDepth < maxDepth) {
          node.children = this.readDirEntries(currentPath, currentDepth + 1, maxDepth);
        } else {
          node.children = [];
        }
      }

      return node;
    } catch {
      return null;
    }
  }

  private readDirEntries(dirPath: string, currentDepth: number = 0, maxDepth: number = 1): FileInfo[] {
    const results: FileInfo[] = [];
    try {
      const files = fs.readdirSync(dirPath);
      // Sort directories first, then alphabetical
      const dirNodes: FileInfo[] = [];
      const fileNodes: FileInfo[] = [];

      for (const file of files) {
        if (!this.showHidden && file.startsWith(".")) continue;
        if (DEFAULT_IGNORED.has(file)) continue;

        const fullPath = path.join(dirPath, file);
        try {
          const stat = fs.statSync(fullPath);
          const isDir = stat.isDirectory();
          const node: FileInfo = {
            path: fullPath,
            name: file,
            isDir,
            size: stat.size,
            modTime: Math.floor(stat.mtimeMs),
          };

          if (isDir) {
            if (currentDepth < maxDepth) {
              node.children = this.readDirEntries(fullPath, currentDepth + 1, maxDepth);
            } else {
              node.children = [];
            }
            dirNodes.push(node);
          } else {
            fileNodes.push(node);
          }
        } catch {}
      }

      dirNodes.sort((a, b) => a.name.localeCompare(b.name));
      fileNodes.sort((a, b) => a.name.localeCompare(b.name));
      return [...dirNodes, ...fileNodes];
    } catch {
      return [];
    }
  }
}
