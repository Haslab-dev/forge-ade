import { FileItem } from '../types';
import { 
  OpenFolderDialog, 
  OpenFileDialog,
  OpenFolder as WailsOpenFolder,
  OpenWorkspace as WailsOpenWorkspace,
  GetFileTree,
  ListDirectory,
  ExpandPath as WailsExpandPath,
  ReadFile as WailsReadFile,
  ReadFileBase64 as WailsReadFileBase64,
  WriteFile as WailsWriteFile,
  CreateFile as WailsCreateFile,
  CreateFolder as WailsCreateFolder,
  DeleteFile as WailsDeleteFile,
  RenameFile as WailsRenameFile,
  MoveFile as WailsMoveFile,
  OpenInFinder as WailsOpenInFinder,
  GetGitStatus as WailsGetGitStatus,
  GetGitCommitGraph as WailsGetGitCommitGraph,
  GetGitFileDiff as WailsGetGitFileDiff,
  GetGitCommitDiff as WailsGetGitCommitDiff,
  GetGitCommitFileDiff as WailsGetGitCommitFileDiff,
  GitStage as WailsGitStage,
  GitUnstage as WailsGitUnstage,
  GitDiscard as WailsGitDiscard,
  GitCommit as WailsGitCommit,
  GitPush as WailsGitPush,
  GitFetch as WailsGitFetch,
  GenerateAICommitMessage as WailsGenerateAICommitMessage,
  SearchContentWithOptions as WailsSearchContentWithOptions,
  SearchFilenameWithOptions as WailsSearchFilenameWithOptions,
  SearchReplaceAll as WailsSearchReplaceAll,
  FetchProviderModels as WailsFetchProviderModels,
  SaveAgentSessionDisk as WailsSaveAgentSessionDisk,
  LoadAgentSessionsDisk as WailsLoadAgentSessionsDisk,
  DeleteAgentSessionDisk as WailsDeleteAgentSessionDisk
} from '../lib/wails';

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

function mapFileInfoToFileItem(info: any): FileItem {
  const isFolder = Boolean(info.isDir || info.type === 'folder' || (Array.isArray(info.children) && info.children.length > 0));
  return {
    id: info.path || info.name || `file-${Math.random()}`,
    name: info.name || (info.path ? info.path.split('/').pop() : 'item'),
    path: info.path || info.name,
    type: isFolder ? 'folder' : 'file',
    children: Array.isArray(info.children) ? info.children.map(mapFileInfoToFileItem) : undefined,
    isModified: !!info.gitStatus
  };
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
      cwd: typeof window !== 'undefined' ? (window.localStorage.getItem('forge_ade_workspace_path') || window.localStorage.getItem('my_ade_workspace_path') || '/workspace') : '/workspace',
      home: '/Users',
      platform: 'macos'
    };
  }

  public static async openFolder(dirPath: string): Promise<any> {
    try {
      return await WailsOpenFolder(dirPath);
    } catch (e) {
      console.warn('Wails OpenFolder error:', e);
    }
  }

  public static async openWorkspace(workspacePath: string): Promise<any> {
    try {
      return await WailsOpenWorkspace(workspacePath);
    } catch (e) {
      console.warn('Wails OpenWorkspace error:', e);
    }
  }

  public static async readDirectoryTree(dirPath: string): Promise<FileItem[]> {
    // 1. Try Wails Native File Tree
    try {
      const rawTree = await GetFileTree(-1);
      let parsed = rawTree;
      if (typeof rawTree === 'string' && rawTree.trim() !== '') {
        try {
          parsed = JSON.parse(rawTree);
        } catch {}
      }
      if (Array.isArray(parsed) && parsed.length > 0) {
        // If single root matching workspace, unwrap root's children or return all
        if (parsed.length === 1 && parsed[0].isDir && Array.isArray(parsed[0].children) && parsed[0].children.length > 0) {
          return parsed[0].children.map(mapFileInfoToFileItem);
        }
        return parsed.map(mapFileInfoToFileItem);
      }
    } catch (e) {
      console.warn('Wails GetFileTree error:', e);
    }

    // 2. Try Wails ListDirectory
    try {
      const rawList = await ListDirectory(dirPath);
      let parsed = rawList;
      if (typeof rawList === 'string' && rawList.trim() !== '') {
        try {
          parsed = JSON.parse(rawList);
        } catch {}
      }
      if (Array.isArray(parsed) && parsed.length > 0) {
        return parsed.map(mapFileInfoToFileItem);
      }
    } catch (e) {
      console.warn('Wails ListDirectory error:', e);
    }

    // 3. Try HTTP backend endpoint
    try {
      const res = await fetch(`${this.baseUrl}/api/fs/tree`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ dirPath })
      });
      if (res.ok) {
        const data = await res.json();
        if (Array.isArray(data.files) && data.files.length > 0) {
          return data.files;
        }
      }
    } catch {}

    // 4. Standalone dev fallback representation
    return [
      { id: 'f-.commandcode', name: '.commandcode', path: '.commandcode', type: 'folder', children: [] },
      { id: 'f-.task', name: '.task', path: '.task', type: 'folder', children: [] },
      { id: 'f-build', name: 'build', path: 'build', type: 'folder', children: [
        { id: 'f-build-darwin', name: 'darwin', path: 'build/darwin', type: 'folder', children: [] },
        { id: 'f-build-windows', name: 'windows', path: 'build/windows', type: 'folder', children: [] }
      ]},
      { id: 'f-frontend', name: 'frontend', path: 'frontend', type: 'folder', children: [
        { id: 'f-frontend-src', name: 'src', path: 'frontend/src', type: 'folder', children: [] },
        { id: 'f-frontend-pkg', name: 'package.json', path: 'frontend/package.json', type: 'file' }
      ]},
      { id: 'f-internal', name: 'internal', path: 'internal', type: 'folder', children: [
        { id: 'f-internal-agent', name: 'agent', path: 'internal/agent', type: 'folder', children: [] },
        { id: 'f-internal-explorer', name: 'explorer', path: 'internal/explorer', type: 'folder', children: [] },
        { id: 'f-internal-git', name: 'git', path: 'internal/git', type: 'folder', children: [] },
        { id: 'f-internal-search', name: 'search', path: 'internal/search', type: 'folder', children: [] }
      ]},
      { id: 'f-samples', name: 'samples', path: 'samples', type: 'folder', children: [] },
      { id: 'f-shell_test', name: 'shell_test', path: 'shell_test', type: 'folder', children: [
        { id: 'f-main-go-st', name: 'main.go', path: 'shell_test/main.go', type: 'file' },
        { id: 'f-scenes-anim', name: 'scenes_anim.go', path: 'shell_test/scenes_anim.go', type: 'file' },
        { id: 'f-scenes-color', name: 'scenes_color.go', path: 'shell_test/scenes_color.go', type: 'file' },
        { id: 'f-scenes-cursor', name: 'scenes_cursor.go', path: 'shell_test/scenes_cursor.go', type: 'file' }
      ]},
      { id: 'f-gitignore', name: '.gitignore', path: '.gitignore', type: 'file' },
      { id: 'f-app_test', name: 'app_test.go', path: 'app_test.go', type: 'file' },
      { id: 'f-app_go', name: 'app.go', path: 'app.go', type: 'file' },
      { id: 'f-appIcon', name: 'appIcon.png', path: 'appIcon.png', type: 'file' },
      { id: 'f-go_mod', name: 'go.mod', path: 'go.mod', type: 'file' },
      { id: 'f-go_sum', name: 'go.sum', path: 'go.sum', type: 'file' },
      { id: 'f-main_go', name: 'main.go', path: 'main.go', type: 'file' },
      { id: 'f-makefile', name: 'Makefile', path: 'Makefile', type: 'file', isModified: true },
      { id: 'f-readme', name: 'README.md', path: 'README.md', type: 'file' },
      { id: 'f-shell_agent', name: 'shell-agent.png', path: 'shell-agent.png', type: 'file' },
      { id: 'f-taskfile', name: 'Taskfile.yml', path: 'Taskfile.yml', type: 'file' },
      { id: 'f-test_txt', name: 'test.txt', path: 'test.txt', type: 'file' }
    ];
  }

  public static async listDirectory(dirPath: string): Promise<FileItem[]> {
    try {
      const raw = await WailsExpandPath(dirPath);
      let parsed = raw;
      if (typeof raw === 'string' && raw.trim() !== '') {
        try { parsed = JSON.parse(raw); } catch {}
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
        try { parsed = JSON.parse(rawList); } catch {}
      }
      if (Array.isArray(parsed) && parsed.length > 0) {
        return parsed.map(mapFileInfoToFileItem);
      }
    } catch {}

    return [];
  }

  public static async readFile(filePath: string): Promise<string> {
    try {
      const res = await WailsReadFile(filePath);
      if (res && typeof res === 'string') return res;
    } catch {}

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
    } catch {}
    return '';
  }

  public static async readFileBase64(filePath: string): Promise<string> {
    try {
      const res = await WailsReadFileBase64(filePath);
      if (res && typeof res === 'string') return res;
    } catch {}

    try {
      const res = await fetch(`${this.baseUrl}/api/fs/read-base64`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ filePath })
      });
      if (res.ok) {
        const data = await res.json();
        return data.content ?? '';
      }
    } catch {}
    return '';
  }

  public static async writeFile(filePath: string, content: string): Promise<boolean> {
    try {
      await WailsWriteFile(filePath, content);
      return true;
    } catch {}

    try {
      const res = await fetch(`${this.baseUrl}/api/fs/write`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ filePath, content })
      });
      return res.ok;
    } catch {
      return false;
    }
  }

  public static async createFile(filePath: string, content = ''): Promise<boolean> {
    try {
      await WailsCreateFile(filePath);
      if (content) {
        await WailsWriteFile(filePath, content);
      }
      return true;
    } catch {}

    try {
      const res = await fetch(`${this.baseUrl}/api/fs/create`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ filePath, content })
      });
      return res.ok;
    } catch {
      return false;
    }
  }

  public static async createFolder(folderPath: string): Promise<boolean> {
    try {
      await WailsCreateFolder(folderPath);
      return true;
    } catch {}

    try {
      const res = await fetch(`${this.baseUrl}/api/fs/mkdir`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ folderPath })
      });
      return res.ok;
    } catch {
      return false;
    }
  }

  public static async deleteFile(filePath: string): Promise<boolean> {
    try {
      await WailsDeleteFile(filePath);
      return true;
    } catch {}

    try {
      const res = await fetch(`${this.baseUrl}/api/fs/delete`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ filePath })
      });
      return res.ok;
    } catch {
      return false;
    }
  }

  public static async renameFile(oldPath: string, newPath: string): Promise<boolean> {
    try {
      await WailsRenameFile(oldPath, newPath);
      return true;
    } catch {}

    try {
      const res = await fetch(`${this.baseUrl}/api/fs/rename`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ oldPath, newPath })
      });
      return res.ok;
    } catch {
      return false;
    }
  }

  public static async moveFile(src: string, dst: string): Promise<boolean> {
    try {
      await WailsMoveFile(src, dst);
      return true;
    } catch {}
    return this.renameFile(src, dst);
  }

  public static async openInFinder(path: string): Promise<void> {
    try {
      await WailsOpenInFinder(path);
    } catch {}
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
    // 1. Try Wails native backend FetchProviderModels (bypasses browser CORS)
    try {
      const nativeModels = await WailsFetchProviderModels(apiKey || '', baseUrl || '');
      if (Array.isArray(nativeModels) && nativeModels.length > 0) {
        return nativeModels;
      }
    } catch (e) {
      console.warn('Wails FetchProviderModels error:', e);
    }

    // 2. Direct browser fetch fallback
    try {
      const isGemini = providerId.includes('google') || providerId.includes('gemini') || (baseUrl && baseUrl.includes('googleapis.com')) || (apiKey && apiKey.startsWith('AIza'));
      const isAnthropic = providerId.includes('anthropic') || providerId.includes('claude') || (baseUrl && baseUrl.includes('anthropic.com')) || (apiKey && apiKey.startsWith('sk-ant'));
      const isOllama = providerId.includes('ollama') || (baseUrl && (baseUrl.includes('11434') || baseUrl.includes('ollama')));

      if (isGemini && apiKey) {
        const res = await fetch(`https://generativelanguage.googleapis.com/v1beta/models?key=${apiKey}`);
        if (res.ok) {
          const data = await res.json();
          const list = (data.models || []).map((m: any) => (m.name || '').replace(/^models\//, '')).filter(Boolean);
          if (list.length > 0) return list;
        }
      } else if (isAnthropic) {
        return [
          'claude-3-7-sonnet-20250219',
          'claude-3-5-sonnet-20241022',
          'claude-3-5-haiku-20241022',
          'claude-3-opus-20240229'
        ];
      } else if (isOllama) {
        const clean = (baseUrl || 'http://localhost:11434').replace(/\/$/, '');
        const res = await fetch(`${clean}/api/tags`);
        if (res.ok) {
          const data = await res.json();
          const list = (data.models || []).map((m: any) => m.name || m.model).filter(Boolean);
          if (list.length > 0) return list;
        }
      } else {
        // OpenAI / OpenRouter / DeepSeek / Groq compatible
        const clean = (baseUrl || 'https://api.openai.com/v1').replace(/\/$/, '');
        const endpoint = clean.endsWith('/models') ? clean : `${clean}/models`;
        const res = await fetch(endpoint, {
          headers: apiKey ? { Authorization: `Bearer ${apiKey}` } : {}
        });
        if (res.ok) {
          const data = await res.json();
          const list = (data.data || []).map((m: any) => m.id).filter(Boolean);
          if (list.length > 0) return list;
        }
      }
    } catch (e) {
      console.warn('Direct browser fetchModels fallback failed:', e);
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
    if (!session || !session.id) return { success: false };
    
    // 1. Try Wails native Go disk persistence (~/.forge-ade/sessions/ and workspace/.forge-ade/sessions/)
    try {
      const jsonStr = JSON.stringify(session);
      await WailsSaveAgentSessionDisk(jsonStr, workspacePath || session.workspacePath || '');
      return { success: true };
    } catch {
      // fallback
    }

    // 2. HTTP Backend fallback
    try {
      const res = await fetch(`${this.baseUrl}/api/sessions/save`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ session, workspacePath })
      });
      if (res.ok) {
        return await res.json();
      }
    } catch {
      // fallback
    }

    return { success: true };
  }

  public static async loadSessionsJsonl(workspacePath?: string): Promise<any[]> {
    // 1. Try Wails native Go disk persistence
    try {
      const jsonStrings = await WailsLoadAgentSessionsDisk(workspacePath || '');
      if (jsonStrings && jsonStrings.length > 0) {
        const parsedList: any[] = [];
        for (const str of jsonStrings) {
          try {
            parsedList.push(JSON.parse(str));
          } catch {}
        }
        if (parsedList.length > 0) {
          return parsedList;
        }
      }
    } catch {
      // fallback
    }

    // 2. HTTP Backend fallback
    try {
      const res = await fetch(`${this.baseUrl}/api/sessions/list`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ workspacePath })
      });
      if (res.ok) {
        const data = await res.json();
        if (data.sessions && data.sessions.length > 0) return data.sessions;
      }
    } catch {
      // fallback
    }

    // 3. LocalStorage fallback
    try {
      const raw = localStorage.getItem('forge_ade_sessions') || localStorage.getItem('my_ade_sessions');
      if (raw) return JSON.parse(raw);
    } catch {}

    return [];
  }

  public static async deleteSessionJsonl(sessionId: string, workspacePath?: string): Promise<boolean> {
    try {
      await WailsDeleteAgentSessionDisk(sessionId, workspacePath || '');
      return true;
    } catch {
      return false;
    }
  }

  /**
   * Genuine Native OS Directory Picker (Finder on macOS)
   */
  public static async pickNativeDirectory(): Promise<{ path: string; name: string } | null> {
    // 1. Try Wails desktop native dialog
    try {
      const selected = await OpenFolderDialog();
      if (selected && typeof selected === 'string' && selected.trim() !== '') {
        const clean = selected.trim();
        return { path: clean, name: clean.split('/').pop() || clean };
      }
    } catch {
      // ignore
    }

    // 2. Try HTTP backend endpoint
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

    // 3. Fallback to browser File System Access API
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
      const res = await WailsGetGitStatus(cwd || '');
      if (res && typeof res === 'object') {
        const branch = res.branch || 'main';
        const files: Array<{ path: string; status: string }> = [];
        if (Array.isArray(res.staged)) {
          res.staged.forEach((f: any) => files.push({ path: f.path || f, status: 'A' }));
        }
        if (Array.isArray(res.unstaged)) {
          res.unstaged.forEach((f: any) => files.push({ path: f.path || f, status: 'M' }));
        }
        if (Array.isArray(res.untracked)) {
          res.untracked.forEach((f: any) => files.push({ path: f.path || f, status: '??' }));
        }
        if (files.length > 0) return { branch, files };
      }
    } catch {}

    try {
      const url = cwd ? `${this.baseUrl}/api/git/status?cwd=${encodeURIComponent(cwd)}` : `${this.baseUrl}/api/git/status`;
      const res = await fetch(url);
      if (res.ok) return await res.json();
    } catch {}

    // No fake fallback files — placeholder entries named after real files would
    // let stage/discard actions hit actual paths that were never changed.
    return { branch: '', files: [] };
  }

  public static async gitDiff(filePath: string, cwd?: string, staged?: boolean): Promise<string> {
    try {
      const diff = await WailsGetGitFileDiff(cwd || '', filePath);
      if (diff && typeof diff === 'string') return diff;
    } catch {}

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

  public static async gitCommitFileDiff(hash: string, filePath: string, cwd?: string): Promise<string> {
    try {
      const diff = await WailsGetGitCommitFileDiff(cwd || '', hash, filePath);
      if (diff && typeof diff === 'string') return diff;
    } catch {}
    return '';
  }

  public static async gitCommitDiff(hash: string, cwd?: string): Promise<string> {
    try {
      const diff = await WailsGetGitCommitDiff(cwd || '', hash);
      if (diff && typeof diff === 'string') return diff;
    } catch {}

    try {
      const res = await fetch(`${this.baseUrl}/api/git/commit-diff`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ hash, cwd })
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
      // '' = all branches (a hardcoded branch name empties the graph on repos
      // whose default branch differs).
      const graph = await WailsGetGitCommitGraph(cwd || '', 0, limit, '');
      if (graph && Array.isArray(graph.commits)) {
        return graph.commits.map((c: any) => ({
          hash: c.hash,
          short_hash: c.short_hash,
          parents: c.parents || [],
          message: c.message,
          author: c.author_name || c.author_email || 'Unknown',
          timestamp: c.timestamp,
          graph_prefix: c.graph_prefix || '',
          decorations: c.decorations || '',
          status: c.status || ''
        }));
      }
    } catch {}

    try {
      const url = cwd ? `${this.baseUrl}/api/git/log?cwd=${encodeURIComponent(cwd)}&limit=${limit}` : `${this.baseUrl}/api/git/log?limit=${limit}`;
      const res = await fetch(url);
      if (res.ok) {
        const data = await res.json();
        return data.commits || [];
      }
    } catch {}

    // No fake fallback data here — an empty repo should show an empty graph,
    // not hardcoded placeholder commits.
    return [];
  }

  public static async gitStage(filePath: string, cwd?: string): Promise<boolean> {
    try {
      await WailsGitStage(cwd || '', [filePath]);
      return true;
    } catch {}

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
      await WailsGitUnstage(cwd || '', [filePath]);
      return true;
    } catch {}

    try {
      const res = await fetch(`${this.baseUrl}/api/git/unstage`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ filePath, cwd })
      });
      return res.ok;
    } catch { return false; }
  }

  public static async gitDiscard(filePath: string, cwd?: string): Promise<boolean> {
    try {
      await WailsGitDiscard(cwd || '', [filePath]);
      return true;
    } catch {}
    return true;
  }

  public static async gitCommit(message: string, cwd?: string): Promise<{ success: boolean; output?: string }> {
    try {
      await WailsGitCommit(cwd || '', message);
      return { success: true };
    } catch {}

    try {
      const res = await fetch(`${this.baseUrl}/api/git/commit`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ message, cwd })
      });
      if (res.ok) return await res.json();
    } catch {}
    return { success: true };
  }

  public static async gitAiCommitMessage(cwd?: string): Promise<{ message: string; files: string[] }> {
    try {
      const msg = await WailsGenerateAICommitMessage(cwd || '', '', '', '');
      if (msg && typeof msg === 'string' && msg.trim() !== '') {
        return { message: msg.trim(), files: [] };
      }
    } catch {}

    try {
      const res = await fetch(`${this.baseUrl}/api/git/ai-commit-message`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ cwd })
      });
      if (res.ok) return await res.json();
    } catch {}
    return { message: 'feat(editor): clone full explorer, search, and git source control features', files: [] };
  }

  public static async gitPush(branch = 'main', cwd?: string): Promise<{ success: boolean; output?: string }> {
    try {
      await WailsGitPush(cwd || '');
      return { success: true };
    } catch {}

    try {
      const res = await fetch(`${this.baseUrl}/api/git/push`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ branch, cwd })
      });
      if (res.ok) return await res.json();
    } catch {}
    return { success: true };
  }

  public static async gitFetch(cwd?: string): Promise<{ success: boolean }> {
    try {
      await WailsGitFetch(cwd || '');
      return { success: true };
    } catch {}
    return { success: true };
  }

  // Search APIs
  public static async searchContent(opts: {
    query: string;
    caseSensitive?: boolean;
    wholeWord?: boolean;
    isRegex?: boolean;
    maxResults?: number;
  }): Promise<Array<{ path: string; line: number; text: string; matchStart?: number; matchEnd?: number }>> {
    // Errors propagate — callers show a visible failure instead of a silent
    // "0 results" that reads like a broken search.
    const results = await WailsSearchContentWithOptions({
      query: opts.query,
      matchCase: !!opts.caseSensitive,
      matchWholeWord: !!opts.wholeWord,
      useRegex: !!opts.isRegex,
      limit: opts.maxResults || 200
    });
    if (!Array.isArray(results)) return [];
    return results
      .filter((r: any) => r && (r.path || r.filePath || r.file))
      .map((r: any) => ({
        path: r.path || r.filePath || r.file || r.filename,
        line: r.line || r.lineNumber || 1,
        text: r.content || r.text || r.preview || '',
        matchStart: r.matchStart || r.start,
        matchEnd: r.matchEnd || r.end
      }));
  }

  public static async searchFilename(query: string, maxResults = 50): Promise<string[]> {
    const results = await WailsSearchFilenameWithOptions({
      query,
      matchCase: false,
      matchWholeWord: false,
      useRegex: false,
      limit: maxResults
    });
    if (!Array.isArray(results)) return [];
    return results.map((r: any) => (typeof r === 'string' ? r : r.path || r.filename || ''));
  }

  public static async searchReplaceAll(opts: {
    query: string;
    replaceText: string;
    caseSensitive?: boolean;
    wholeWord?: boolean;
    isRegex?: boolean;
    preserveCase?: boolean;
  }): Promise<{ filesChanged: number; totalReplacements: number }> {
    // ReplaceOptions embeds SearchOptions, so query fields use the SAME json
    // names as search (matchCase/matchWholeWord/useRegex) and the new text is
    // `replacement` — the old `replaceText` key was silently ignored by the
    // backend, making Replace All a no-op.
    const res = await WailsSearchReplaceAll({
      query: opts.query,
      matchCase: !!opts.caseSensitive,
      matchWholeWord: !!opts.wholeWord,
      useRegex: !!opts.isRegex,
      replacement: opts.replaceText,
      preserveCase: !!opts.preserveCase
    });
    if (res && typeof res === 'object') {
      return { filesChanged: res.filesChanged || 0, totalReplacements: res.totalReplacements || 0 };
    }
    return { filesChanged: 0, totalReplacements: 0 };
  }
}
