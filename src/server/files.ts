import fs from "fs";
import path from "path";
import os from "os";

export class FileManager {
  public resolvePath(targetPath: string): string {
    if (!targetPath) return "";
    if (targetPath.startsWith("~")) {
      return path.resolve(os.homedir(), targetPath.slice(1).replace(/^[/\\]/, ""));
    }
    return path.resolve(targetPath);
  }

  public getHomeDir(): string {
    return os.homedir();
  }

  public isDir(targetPath: string): boolean {
    try {
      const resolved = this.resolvePath(targetPath);
      return fs.statSync(resolved).isDirectory();
    } catch {
      return false;
    }
  }

  public readFile(targetPath: string): string {
    const resolved = this.resolvePath(targetPath);
    return fs.readFileSync(resolved, "utf-8");
  }

  public readFileBase64(targetPath: string): string {
    const resolved = this.resolvePath(targetPath);
    return fs.readFileSync(resolved).toString("base64");
  }

  public writeFile(targetPath: string, content: string): void {
    const resolved = this.resolvePath(targetPath);
    const dir = path.dirname(resolved);
    if (!fs.existsSync(dir)) {
      fs.mkdirSync(dir, { recursive: true });
    }
    fs.writeFileSync(resolved, content, "utf-8");
  }

  public createFile(targetPath: string): void {
    const resolved = this.resolvePath(targetPath);
    const dir = path.dirname(resolved);
    if (!fs.existsSync(dir)) {
      fs.mkdirSync(dir, { recursive: true });
    }
    if (!fs.existsSync(resolved)) {
      fs.writeFileSync(resolved, "", "utf-8");
    }
  }

  public createFolder(targetPath: string): void {
    const resolved = this.resolvePath(targetPath);
    fs.mkdirSync(resolved, { recursive: true });
  }

  public deleteFile(targetPath: string): void {
    const resolved = this.resolvePath(targetPath);
    if (fs.existsSync(resolved)) {
      const stat = fs.statSync(resolved);
      if (stat.isDirectory()) {
        fs.rmSync(resolved, { recursive: true, force: true });
      } else {
        fs.unlinkSync(resolved);
      }
    }
  }

  public renameFile(oldPath: string, newPath: string): void {
    const src = this.resolvePath(oldPath);
    const dst = this.resolvePath(newPath);
    const dstDir = path.dirname(dst);
    if (!fs.existsSync(dstDir)) {
      fs.mkdirSync(dstDir, { recursive: true });
    }
    fs.renameSync(src, dst);
  }

  public copyFile(srcPath: string, dstPath: string): void {
    const src = this.resolvePath(srcPath);
    const dst = this.resolvePath(dstPath);
    const dstDir = path.dirname(dst);
    if (!fs.existsSync(dstDir)) {
      fs.mkdirSync(dstDir, { recursive: true });
    }
    fs.copyFileSync(src, dst);
  }

  public copyPath(srcPath: string, dstPath: string): void {
    const src = this.resolvePath(srcPath);
    const dst = this.resolvePath(dstPath);
    fs.cpSync(src, dst, { recursive: true });
  }

  public moveFile(srcPath: string, dstPath: string): void {
    this.renameFile(srcPath, dstPath);
  }

  public getClipboardFiles(): string[] {
    // Return empty array if not supported by OS clipboard
    return [];
  }
}
