import {
  OpenFolder as WailsOpenFolder,
  OpenWorkspace as WailsOpenWorkspace,
  CloseWorkspace as WailsCloseWorkspace,
  GetRecentProjects as WailsGetRecentProjects,
  RemoveRecent as WailsRemoveRecent,
  OpenFolderDialog,
  OpenFileDialog,
} from '../../lib/wails';
import type { FileItem } from '../../types';

export interface WorkspaceInfo {
  cwd?: string;
  home?: string;
  platform?: string;
  currentFolder?: string;
  folders?: string[];
  recentProjects?: any[];
}

export class WorkspaceApi {
  public static async getWorkspaceInfo(): Promise<WorkspaceInfo> {
    const recents = await WailsGetRecentProjects();
    return {
      cwd: '',
      home: '',
      platform: '',
      currentFolder: '',
      folders: [],
      recentProjects: Array.isArray(recents) ? recents : [],
    };
  }

  public static async openFolder(dirPath: string): Promise<any> {
    return WailsOpenFolder(dirPath);
  }

  public static async openWorkspace(workspacePath: string): Promise<any> {
    return WailsOpenWorkspace(workspacePath);
  }

  public static async closeWorkspace(): Promise<void> {
    await WailsCloseWorkspace();
  }

  public static async getRecentProjects(): Promise<any[]> {
    const list = await WailsGetRecentProjects();
    return Array.isArray(list) ? list : [];
  }

  public static async removeRecent(path: string): Promise<void> {
    await WailsRemoveRecent(path);
  }

  public static async pickNativeDirectory(): Promise<{ path: string; name: string } | null> {
    const selected = await OpenFolderDialog();
    if (!selected || typeof selected !== 'string' || selected.trim() === '') {
      return null;
    }
    const cleanPath = selected.replace(/\/+$/, '');
    const parts = cleanPath.split(/[/\\]/);
    const name = parts[parts.length - 1] || cleanPath;
    return { path: selected, name };
  }

  public static async pickNativeFiles(): Promise<Array<{ path: string; name: string; content: string }>> {
    const selected = await OpenFileDialog();
    if (!selected) return [];
    const paths = Array.isArray(selected) ? selected : [selected];
    return paths.filter(Boolean).map((p) => {
      const parts = p.split(/[/\\]/);
      return { path: p, name: parts[parts.length - 1] || p, content: '' };
    });
  }

  public static async pickDirectoryInBrowser(): Promise<{ name: string; handle: any; files: FileItem[] } | null> {
    if (typeof window === 'undefined' || !(window as any).showDirectoryPicker) {
      return null;
    }
    try {
      const dirHandle = await (window as any).showDirectoryPicker();
      return {
        name: dirHandle.name,
        handle: dirHandle,
        files: [],
      };
    } catch {
      return null;
    }
  }
}
