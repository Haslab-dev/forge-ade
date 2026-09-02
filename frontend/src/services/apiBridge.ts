import { FileItem } from '../types';

export interface CommandExecutionResult {
  stdout: string;
  stderr: string;
  exitCode: number;
}

export interface WorkspaceInfo {
  cwd: string;
  home: string;
  platform: string;
}

export class ApiBridge {
  private static baseUrl = '';

  public static async getWorkspaceInfo(): Promise<WorkspaceInfo> {
    try {
      const res = await fetch(`${this.baseUrl}/api/workspace/info`);
      if (res.ok) {
        return await res.json();
      }
    } catch {
      // Fallback
    }
    return {
      cwd: typeof window !== 'undefined' ? (window.localStorage.getItem('my_ade_workspace_path') || window.localStorage.getItem('devin_workspace_path') || '/workspace') : '/workspace',
      home: '/Users',
      platform: 'macos'
    };
  }

  public static async readDirectoryTree(dirPath: string): Promise<FileItem[]> {
    try {
      const res = await fetch(`${this.baseUrl}/api/fs/tree`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ dirPath })
      });
      if (res.ok) {
        const data = await res.json();
        return data.files || [];
      }
    } catch (e) {
      console.warn('Backend fs/tree failed, falling back to local state', e);
    }
    return [];
  }

  public static async readFile(filePath: string): Promise<string> {
    try {
      const res = await fetch(`${this.baseUrl}/api/fs/read`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ filePath })
      });
      if (res.ok) {
        const data = await res.json();
        return data.content ?? '';
      }
    } catch (e) {
      console.warn('Backend fs/read failed', e);
    }
    return '';
  }

  public static async writeFile(filePath: string, content: string): Promise<boolean> {
    try {
      const res = await fetch(`${this.baseUrl}/api/fs/write`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ filePath, content })
      });
      return res.ok;
    } catch (e) {
      console.warn('Backend fs/write failed', e);
      return false;
    }
  }

  public static async createFile(filePath: string, content = ''): Promise<boolean> {
    try {
      const res = await fetch(`${this.baseUrl}/api/fs/create`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ filePath, content })
      });
      return res.ok;
    } catch (e) {
      console.warn('Backend fs/create failed', e);
      return false;
    }
  }

  public static async deleteFile(filePath: string): Promise<boolean> {
    try {
      const res = await fetch(`${this.baseUrl}/api/fs/delete`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ filePath })
      });
      return res.ok;
    } catch (e) {
      console.warn('Backend fs/delete failed', e);
      return false;
    }
  }

  public static async renameFile(oldPath: string, newPath: string): Promise<boolean> {
    try {
      const res = await fetch(`${this.baseUrl}/api/fs/rename`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ oldPath, newPath })
      });
      return res.ok;
    } catch (e) {
      console.warn('Backend fs/rename failed', e);
      return false;
    }
  }

  public static async executeCommand(command: string, cwd: string): Promise<CommandExecutionResult> {
    try {
      const res = await fetch(`${this.baseUrl}/api/terminal/exec`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ command, cwd })
      });
      if (res.ok) {
        return await res.json();
      }
      const errText = await res.text();
      return { stdout: '', stderr: errText, exitCode: 1 };
    } catch (e: any) {
      return { stdout: '', stderr: e.message || 'Execution failed', exitCode: 1 };
    }
  }

  public static async fetchModels(providerId: string, baseUrl?: string, apiKey?: string): Promise<string[]> {
    try {
      const res = await fetch(`${this.baseUrl}/api/models/fetch`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ providerId, baseUrl, apiKey })
      });
      if (res.ok) {
        const data = await res.json();
        return data.models || [];
      }
    } catch (e) {
      console.warn('fetchModels failed', e);
    }
    return [];
  }

  public static async discoverMcps(): Promise<any[]> {
    try {
      const res = await fetch(`${this.baseUrl}/api/mcp/discover`);
      if (res.ok) {
        const data = await res.json();
        return data.discoveredMcps || [];
      }
    } catch (e) {
      console.warn('discoverMcps failed', e);
    }
    return [];
  }

  public static async discoverSkills(): Promise<any[]> {
    try {
      const res = await fetch(`${this.baseUrl}/api/skills/discover`);
      if (res.ok) {
        const data = await res.json();
        return data.discoveredSkills || [];
      }
    } catch (e) {
      console.warn('discoverSkills failed', e);
    }
    return [];
  }

  public static async handshakeACP(agent: { id: string; type: string; endpoint?: string }): Promise<{ connected: boolean; error?: string; endpoint?: string }> {
    try {
      const res = await fetch(`${this.baseUrl}/api/acp/handshake`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ agentId: agent.id, type: agent.type, endpoint: agent.endpoint })
      });
      if (res.ok) {
        return await res.json();
      }
    } catch (e: any) {
      return { connected: false, error: e.message || 'ACP connection timeout' };
    }
    return { connected: false, error: 'ACP handshake failed' };
  }

  public static async saveSessionJsonl(session: any, workspacePath?: string): Promise<{ success: boolean; filePath?: string }> {
    try {
      const res = await fetch(`${this.baseUrl}/api/sessions/save`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ session, workspacePath })
      });
      if (res.ok) {
        return await res.json();
      }
    } catch (e) {
      console.warn('saveSessionJsonl failed', e);
    }
    return { success: false };
  }

  public static async loadSessionsJsonl(workspacePath?: string): Promise<any[]> {
    try {
      const res = await fetch(`${this.baseUrl}/api/sessions/list`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ workspacePath })
      });
      if (res.ok) {
        const data = await res.json();
        return data.sessions || [];
      }
    } catch (e) {
      console.warn('loadSessionsJsonl failed', e);
    }
    return [];
  }

  /**
   * Genuine Native OS Directory Picker (Finder on macOS)
   */
  public static async pickNativeDirectory(): Promise<{ path: string; name: string } | null> {
    try {
      const res = await fetch(`${this.baseUrl}/api/workspace/pick-directory`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' }
      });
      if (res.ok) {
        const data = await res.json();
        if (!data.canceled && data.path) {
          return { path: data.path, name: data.name || data.path.split('/').pop() || 'workspace' };
        }
      }
    } catch (e) {
      console.warn('pickNativeDirectory failed, falling back to browser picker', e);
    }

    // Fallback to browser File System Access API
    const browserPicked = await this.pickDirectoryInBrowser();
    if (browserPicked) {
      return { path: browserPicked.name, name: browserPicked.name };
    }
    return null;
  }

  /**
   * Genuine Native OS File Picker (Finder on macOS)
   */
  public static async pickNativeFiles(): Promise<Array<{ path: string; name: string; content: string }>> {
    try {
      const res = await fetch(`${this.baseUrl}/api/workspace/pick-files`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' }
      });
      if (res.ok) {
        const data = await res.json();
        if (!data.canceled && Array.isArray(data.files)) {
          return data.files;
        }
      }
    } catch (e) {
      console.warn('pickNativeFiles failed', e);
    }
    return [];
  }

  /**
   * Browser Native File System Access API picker
   */
  public static async pickDirectoryInBrowser(): Promise<{ name: string; handle: any; files: FileItem[] } | null> {
    if (typeof window !== 'undefined' && 'showDirectoryPicker' in window) {
      try {
        const dirHandle = await (window as any).showDirectoryPicker({
          mode: 'readwrite'
        });
        const files: FileItem[] = await this.readEntriesFromHandle(dirHandle, dirHandle.name);
        return { name: dirHandle.name, handle: dirHandle, files };
      } catch (err: any) {
        if (err.name === 'AbortError') return null;
        console.error('showDirectoryPicker error:', err);
      }
    }
    return null;
  }

  private static async readEntriesFromHandle(dirHandle: any, currentPath: string): Promise<FileItem[]> {
    const items: FileItem[] = [];
    for await (const entry of dirHandle.values()) {
      const itemPath = `${currentPath}/${entry.name}`;
      if (entry.kind === 'file') {
        let content = '';
        try {
          const file = await entry.getFile();
          content = await file.text();
        } catch {
          // binary or unreadable
        }
        items.push({
          id: `f-${itemPath}`,
          name: entry.name,
          path: itemPath,
          type: 'file',
          content,
          isModified: false
        });
      } else if (entry.kind === 'directory') {
        const children = await this.readEntriesFromHandle(entry, itemPath);
        items.push({
          id: `d-${itemPath}`,
          name: entry.name,
          path: itemPath,
          type: 'folder',
          children
        });
      }
    }
    return items.sort((a, b) => {
      if (a.type === 'folder' && b.type !== 'folder') return -1;
      if (a.type !== 'folder' && b.type === 'folder') return 1;
      return a.name.localeCompare(b.name);
    });
  }

  // Git APIs
  public static async gitStatus(cwd?: string): Promise<{ branch: string; files: Array<{ path: string; status: string }> }> {
    try {
      const url = cwd ? `${this.baseUrl}/api/git/status?cwd=${encodeURIComponent(cwd)}` : `${this.baseUrl}/api/git/status`;
      const res = await fetch(url);
      if (res.ok) return await res.json();
    } catch {}
    return { branch: 'main', files: [] };
  }

  public static async gitDiff(filePath: string, cwd?: string, staged?: boolean): Promise<string> {
    try {
      const res = await fetch(`${this.baseUrl}/api/git/diff`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ filePath, cwd, staged })
      });
      if (res.ok) {
        const data = await res.json();
        return data.diff || '';
      }
    } catch {}
    return '';
  }

  public static async gitLog(cwd?: string, limit = 50): Promise<any[]> {
    try {
      const url = cwd ? `${this.baseUrl}/api/git/log?cwd=${encodeURIComponent(cwd)}&limit=${limit}` : `${this.baseUrl}/api/git/log?limit=${limit}`;
      const res = await fetch(url);
      if (res.ok) {
        const data = await res.json();
        return data.commits || [];
      }
    } catch {}
    return [];
  }

  public static async gitStage(filePath: string, cwd?: string): Promise<boolean> {
    try {
      const res = await fetch(`${this.baseUrl}/api/git/stage`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ filePath, cwd })
      });
      return res.ok;
    } catch { return false; }
  }

  public static async gitUnstage(filePath: string, cwd?: string): Promise<boolean> {
    try {
      const res = await fetch(`${this.baseUrl}/api/git/unstage`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ filePath, cwd })
      });
      return res.ok;
    } catch { return false; }
  }

  public static async gitCommit(message: string, cwd?: string): Promise<{ success: boolean; output?: string }> {
    try {
      const res = await fetch(`${this.baseUrl}/api/git/commit`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ message, cwd })
      });
      if (res.ok) return await res.json();
    } catch {}
    return { success: false };
  }

  public static async gitAiCommitMessage(cwd?: string): Promise<{ message: string; files: string[] }> {
    try {
      const res = await fetch(`${this.baseUrl}/api/git/ai-commit-message`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ cwd })
      });
      if (res.ok) return await res.json();
    } catch {}
    return { message: 'chore: update files', files: [] };
  }

  public static async gitPush(branch = 'main', cwd?: string): Promise<{ success: boolean; output?: string }> {
    try {
      const res = await fetch(`${this.baseUrl}/api/git/push`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ branch, cwd })
      });
      if (res.ok) return await res.json();
    } catch {}
    return { success: false };
  }
}
