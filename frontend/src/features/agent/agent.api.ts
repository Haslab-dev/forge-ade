import {
  SaveAgentSessionDisk as WailsSaveAgentSessionDisk,
  LoadAgentSessionsDisk as WailsLoadAgentSessionsDisk,
  DeleteAgentSessionDisk as WailsDeleteAgentSessionDisk,
  ListMemories as WailsListMemories,
  SaveMemory as WailsSaveMemory,
  DeleteMemory as WailsDeleteMemory,
  ReloadMemories as WailsReloadMemories,
  SearchContentWithOptions as WailsSearchContentWithOptions,
  SearchFilenameWithOptions as WailsSearchFilenameWithOptions,
  SearchReplaceAll as WailsSearchReplaceAll,
  IndexStatus as WailsIndexStatus,
  ReindexWorkspace as WailsReindexWorkspace,
  GetOutline as WailsGetOutline,
  SearchIndexSymbols as WailsSearchIndexSymbols,
  GetComputerUseStatus as WailsGetComputerUseStatus,
  OpenComputerUsePermissions as WailsOpenComputerUsePermissions,
  FetchProviderModels as WailsFetchProviderModels,
} from '../../lib/wails';
import type { AgentMemoryEntry, IndexingStatusInfo, WorkspaceSymbol } from '../../types';

const SYMBOL_KIND_NAMES = [
  'function', 'class', 'interface', 'enum', 'struct', 'type',
  'variable', 'constant', 'method', 'package'
] as const;

export function normalizeSymbol(raw: any): WorkspaceSymbol {
  const kind = typeof raw?.Kind === 'number' ? SYMBOL_KIND_NAMES[raw.Kind] : undefined;
  return {
    name: String(raw?.Name ?? ''),
    kind: kind ?? 'unknown',
    file: String(raw?.File ?? ''),
    line: Number(raw?.Line) || 0,
    column: Number(raw?.Column) || 0,
    scope: raw?.Scope || undefined,
    exported: !!raw?.Exported,
  };
}

export class AgentApi {
  public static async saveSessionJsonl(
    session: any,
    workspacePath?: string
  ): Promise<{ success: boolean; filePath?: string }> {
    if (!session || !session.id) return { success: false };
    try {
      const jsonStr = JSON.stringify(session);
      await WailsSaveAgentSessionDisk(jsonStr, workspacePath || session.workspacePath || '');
      return { success: true };
    } catch (err) {
      console.warn('WailsSaveAgentSessionDisk failed, falling back to local storage:', err);
      try {
        localStorage.setItem(`forge_ade_session_${session.id}`, JSON.stringify(session));
        return { success: true };
      } catch {
        return { success: false };
      }
    }
  }

  public static async loadSessionsJsonl(workspacePath?: string): Promise<any[]> {
    try {
      const jsonStrings = await WailsLoadAgentSessionsDisk(workspacePath || '');
      if (jsonStrings && jsonStrings.length > 0) {
        const parsedList: any[] = [];
        for (const str of jsonStrings) {
          try {
            parsedList.push(JSON.parse(str));
          } catch {
            // parse error
          }
        }
        if (parsedList.length > 0) {
          return parsedList;
        }
      }
    } catch {
      // fallback
    }

    try {
      const raw = localStorage.getItem('forge_ade_sessions') || localStorage.getItem('my_ade_sessions');
      if (raw) return JSON.parse(raw);
    } catch {
      // ignore
    }

    return [];
  }

  public static async deleteSessionJsonl(sessionId: string, workspacePath?: string): Promise<boolean> {
    try {
      await WailsDeleteAgentSessionDisk(sessionId, workspacePath || '');
      return true;
    } catch {
      try {
        localStorage.removeItem(`forge_ade_session_${sessionId}`);
        return true;
      } catch {
        return false;
      }
    }
  }

  public static async listMemories(): Promise<AgentMemoryEntry[]> {
    try {
      const res = await WailsListMemories();
      return Array.isArray(res) ? res : [];
    } catch {
      return [];
    }
  }

  public static async saveMemory(entry: AgentMemoryEntry): Promise<void> {
    await WailsSaveMemory(entry);
  }

  public static async deleteMemory(id: string): Promise<void> {
    await WailsDeleteMemory(id);
  }

  public static async reloadMemories(): Promise<AgentMemoryEntry[]> {
    try {
      const res = await WailsReloadMemories();
      return Array.isArray(res) ? res : [];
    } catch {
      return [];
    }
  }

  public static async handshakeACP(_agent: {
    id: string;
    type: string;
    endpoint?: string;
    command?: string;
  }): Promise<{ connected: boolean; error?: string; endpoint?: string }> {
    return { connected: false, error: 'ACP is deprecated and removed' };
  }

  public static async searchContent(opts: {
    query: string;
    caseSensitive?: boolean;
    wholeWord?: boolean;
    isRegex?: boolean;
    maxResults?: number;
  }): Promise<Array<{ path: string; line: number; text: string; matchStart?: number; matchEnd?: number }>> {
    const results = await WailsSearchContentWithOptions({
      query: opts.query,
      matchCase: !!opts.caseSensitive,
      matchWholeWord: !!opts.wholeWord,
      useRegex: !!opts.isRegex,
      limit: opts.maxResults || 200,
    });
    if (!Array.isArray(results)) return [];
    return results
      .filter((r: any) => r && (r.path || r.filePath || r.file))
      .map((r: any) => ({
        path: r.path || r.filePath || r.file || r.filename,
        line: r.line || r.lineNumber || 1,
        text: r.content || r.text || r.preview || '',
        matchStart: r.matchStart || r.start,
        matchEnd: r.matchEnd || r.end,
      }));
  }

  public static async searchFilename(query: string, maxResults = 50): Promise<string[]> {
    const results = await WailsSearchFilenameWithOptions({
      query,
      matchCase: false,
      matchWholeWord: false,
      useRegex: false,
      limit: maxResults,
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
    const res = await WailsSearchReplaceAll({
      query: opts.query,
      matchCase: !!opts.caseSensitive,
      matchWholeWord: !!opts.wholeWord,
      useRegex: !!opts.isRegex,
      replacement: opts.replaceText,
      preserveCase: !!opts.preserveCase,
    });
    if (res && typeof res === 'object') {
      return { filesChanged: res.filesChanged || 0, totalReplacements: res.totalReplacements || 0 };
    }
    return { filesChanged: 0, totalReplacements: 0 };
  }

  public static async getIndexStatus(): Promise<IndexingStatusInfo | null> {
    try {
      const res = await WailsIndexStatus();
      return res || null;
    } catch {
      return null;
    }
  }

  public static async reindexWorkspace(): Promise<{ built: boolean; symbols?: number }> {
    try {
      const res = await WailsReindexWorkspace();
      return res || { built: false };
    } catch (e) {
      console.error('Reindex workspace failed:', e);
      return { built: false };
    }
  }

  public static async getFileOutline(file: string): Promise<WorkspaceSymbol[]> {
    try {
      const res = await WailsGetOutline(file);
      return Array.isArray(res) ? res.map(normalizeSymbol).filter((s) => s.name) : [];
    } catch {
      return [];
    }
  }

  public static async searchWorkspaceSymbols(query: string, limit = 60): Promise<WorkspaceSymbol[]> {
    try {
      const res = await WailsSearchIndexSymbols(query);
      return (Array.isArray(res) ? res : [])
        .slice(0, limit)
        .map(normalizeSymbol)
        .filter((s) => s.name);
    } catch {
      return [];
    }
  }

  public static async getComputerUseStatus(): Promise<any> {
    try {
      return await WailsGetComputerUseStatus();
    } catch {
      return { available: false };
    }
  }

  public static async openComputerUsePermissions(): Promise<void> {
    await WailsOpenComputerUsePermissions();
  }

  public static async fetchModels(providerId: string, baseUrl?: string, apiKey?: string): Promise<string[]> {
    try {
      const nativeModels = await WailsFetchProviderModels(apiKey || '', baseUrl || '');
      if (Array.isArray(nativeModels) && nativeModels.length > 0) {
        return nativeModels;
      }
    } catch (e) {
      console.warn('Wails FetchProviderModels error:', e);
    }

    try {
      const isGemini =
        providerId.includes('google') ||
        providerId.includes('gemini') ||
        (baseUrl && baseUrl.includes('googleapis.com')) ||
        (apiKey && apiKey.startsWith('AIza'));
      if (isGemini && apiKey) {
        const res = await fetch(`https://generativelanguage.googleapis.com/v1beta/models?key=${apiKey}`);
        if (res.ok) {
          const data = await res.json();
          if (Array.isArray(data.models)) {
            return data.models.map((m: any) => m.name.replace(/^models\//, ''));
          }
        }
      }
    } catch {
      // browser fetch fallback
    }

    return [];
  }
}
