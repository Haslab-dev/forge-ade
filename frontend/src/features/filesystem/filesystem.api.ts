import {
  GetFileTree,
  GetDirectoryTree,
  ListDirectory,
  ExpandPath as WailsExpandPath,
  ReadFile as WailsReadFile,
  ReadFileBase64 as WailsReadFileBase64,
  WriteFile as WailsWriteFile,
  WriteFileBase64 as WailsWriteFileBase64,
  CreateFile as WailsCreateFile,
  CreateFolder as WailsCreateFolder,
  DeleteFile as WailsDeleteFile,
  RenameFile as WailsRenameFile,
  MoveFile as WailsMoveFile,
  CopyPath as WailsCopyPath,
  OpenInFinder as WailsOpenInFinder,
  GetSandboxDir as WailsGetSandboxDir,
  ClearSandbox as WailsClearSandbox,
  IsDir as WailsIsDir,
} from '../../lib/wails';
import type { FileItem } from '../../types';

export function mapFileInfoToFileItem(info: any): FileItem {
  const isFolder = Boolean(info.isDir || info.type === 'folder' || (Array.isArray(info.children) && info.children.length > 0));
  return {
    id: info.path || info.name || `file-${Math.random()}`,
    name: info.name || (info.path ? info.path.split(/[/\\]/).pop() : 'item'),
    path: info.path || info.name,
    type: isFolder ? 'folder' : 'file',
    children: Array.isArray(info.children) ? info.children.map(mapFileInfoToFileItem) : undefined,
    isModified: !!info.gitStatus,
  };
}

export class FilesystemApi {
  public static async readDirectoryTree(dirPath: string): Promise<FileItem[]> {
    if (dirPath) {
      try {
        const rawTree = await GetDirectoryTree(dirPath, -1);
        let parsed = rawTree;
        if (typeof rawTree === 'string' && rawTree.trim() !== '') {
          try {
            parsed = JSON.parse(rawTree);
          } catch {
            // parse error
          }
        }
        if (Array.isArray(parsed) && parsed.length > 0) {
          if (parsed.length === 1 && parsed[0].isDir && Array.isArray(parsed[0].children) && parsed[0].children.length > 0) {
            return parsed[0].children.map(mapFileInfoToFileItem);
          }
          return parsed.map(mapFileInfoToFileItem);
        }
      } catch (e) {
        console.warn('Wails GetDirectoryTree error:', e);
      }
    }

    try {
      const rawTree = await GetFileTree(-1);
      let parsed = rawTree;
      if (typeof rawTree === 'string' && rawTree.trim() !== '') {
        try {
          parsed = JSON.parse(rawTree);
        } catch {
          // parse error
        }
      }
      if (Array.isArray(parsed) && parsed.length > 0) {
        if (parsed.length === 1 && parsed[0].isDir && Array.isArray(parsed[0].children) && parsed[0].children.length > 0) {
          return parsed[0].children.map(mapFileInfoToFileItem);
        }
        return parsed.map(mapFileInfoToFileItem);
      }
    } catch (e) {
      console.warn('Wails GetFileTree error:', e);
    }

    try {
      const rawList = await ListDirectory(dirPath);
      let parsed = rawList;
      if (typeof rawList === 'string' && rawList.trim() !== '') {
        try {
          parsed = JSON.parse(rawList);
        } catch {
          // parse error
        }
      }
      if (Array.isArray(parsed) && parsed.length > 0) {
        return parsed.map(mapFileInfoToFileItem);
      }
    } catch (e) {
      console.warn('Wails ListDirectory error:', e);
    }

    return [];
  }

  public static async listDirectory(dirPath: string): Promise<FileItem[]> {
    try {
      const raw = await WailsExpandPath(dirPath);
      let parsed = raw;
      if (typeof raw === 'string' && raw.trim() !== '') {
        try {
          parsed = JSON.parse(raw);
        } catch {
          // parse error
        }
      }
      if (Array.isArray(parsed) && parsed.length > 0) {
        return parsed.map(mapFileInfoToFileItem);
      }
    } catch (e) {
      console.warn('Wails ExpandPath error:', e);
    }

    try {
      const rawList = await ListDirectory(dirPath);
      let parsed = rawList;
      if (typeof rawList === 'string' && rawList.trim() !== '') {
        try {
          parsed = JSON.parse(rawList);
        } catch {
          // parse error
        }
      }
      if (Array.isArray(parsed) && parsed.length > 0) {
        return parsed.map(mapFileInfoToFileItem);
      }
    } catch {
      // ignore
    }

    return [];
  }

  public static async readFile(filePath: string): Promise<string> {
    return WailsReadFile(filePath);
  }

  public static async readFileBase64(filePath: string): Promise<string> {
    return WailsReadFileBase64(filePath);
  }

  public static async writeFile(filePath: string, content: string): Promise<boolean> {
    await WailsWriteFile(filePath, content);
    return true;
  }

  public static async writeFileBase64(filePath: string, base64Content: string): Promise<boolean> {
    await WailsWriteFileBase64(filePath, base64Content);
    return true;
  }

  public static async saveAttachment(filename: string, base64Content: string, projectPath: string = ''): Promise<string> {
    const sandboxDir = await WailsGetSandboxDir(projectPath);
    const filePath = `${sandboxDir}/${Date.now()}-${filename}`;
    await WailsWriteFileBase64(filePath, base64Content);
    return filePath;
  }

  public static async createFile(filePath: string, content = ''): Promise<boolean> {
    await WailsCreateFile(filePath);
    if (content) {
      await WailsWriteFile(filePath, content);
    }
    return true;
  }

  public static async createFolder(folderPath: string): Promise<boolean> {
    await WailsCreateFolder(folderPath);
    return true;
  }

  public static async deleteFile(filePath: string): Promise<boolean> {
    await WailsDeleteFile(filePath);
    return true;
  }

  public static async renameFile(oldPath: string, newPath: string): Promise<boolean> {
    await WailsRenameFile(oldPath, newPath);
    return true;
  }

  public static async moveFile(sourcePath: string, targetPath: string): Promise<boolean> {
    await WailsMoveFile(sourcePath, targetPath);
    return true;
  }

  public static async copyPath(src: string, dst: string): Promise<void> {
    await WailsCopyPath(src, dst);
  }

  public static async openInFinder(path: string): Promise<void> {
    await WailsOpenInFinder(path);
  }

  public static async isDir(path: string): Promise<boolean> {
    return WailsIsDir(path);
  }

  public static async fileSize(path: string): Promise<number> {
    try {
      const content = await WailsReadFile(path);
      return typeof content === 'string' ? new Blob([content]).size : -1;
    } catch {
      return -1;
    }
  }

  public static async expandPath(path: string): Promise<string> {
    return WailsExpandPath(path);
  }

  public static async getSandboxDir(projectPath: string = ''): Promise<string> {
    return WailsGetSandboxDir(projectPath);
  }

  public static async clearSandbox(projectPath: string = ''): Promise<void> {
    await WailsClearSandbox(projectPath);
  }
}
