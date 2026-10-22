import {
  ListAgentCliRuntimeConfigs,
  EnsureAgentCliPath,
  FormatConfigContent,
  CheckSyntax,
  ExecuteCommandSync,
  OpenInTerminal as WailsOpenInTerminal,
  CreateShell as WailsCreateShell,
  WriteSession as WailsWriteSession,
  ResizeSession as WailsResizeSession,
  StopSession as WailsStopSession,
  ListSessions as WailsListSessions,
} from '../../lib/wails';
import type { AgentCliRuntimeConfig, LSPDiagnostic } from '../../types';

export interface CommandExecutionResult {
  stdout: string;
  stderr: string;
  exitCode: number;
}

export class TerminalApi {
  public static async ensureAgentCliPath(path: string, isDir: boolean): Promise<boolean> {
    try {
      return await EnsureAgentCliPath(path, isDir);
    } catch {
      return false;
    }
  }

  public static async formatConfigContent(path: string, content: string): Promise<string> {
    try {
      const res = await FormatConfigContent(path, content);
      return typeof res === 'string' ? res : content;
    } catch {
      return content;
    }
  }

  public static async checkSyntax(path: string, content: string): Promise<LSPDiagnostic[]> {
    try {
      const res = await CheckSyntax(path, content);
      return Array.isArray(res) ? res : [];
    } catch {
      return [];
    }
  }

  public static async listAgentCliRuntimeConfigs(): Promise<AgentCliRuntimeConfig[]> {
    try {
      const configs = await ListAgentCliRuntimeConfigs();
      return Array.isArray(configs) ? configs : [];
    } catch {
      return [];
    }
  }

  public static async openInTerminal(path: string): Promise<void> {
    await WailsOpenInTerminal(path);
  }

  public static async executeCommand(command: string, cwd: string): Promise<CommandExecutionResult> {
    try {
      const res = await ExecuteCommandSync(command, cwd);
      if (res && (res.stdout || res.stderr || res.exitCode === 0)) {
        return res;
      }
      return { stdout: '', stderr: 'Command finished with no output', exitCode: 0 };
    } catch (err) {
      return { stdout: '', stderr: err instanceof Error ? err.message : String(err), exitCode: 1 };
    }
  }

  public static async createShell(name: string, cwd: string = ''): Promise<any> {
    return WailsCreateShell(name, cwd);
  }

  public static async writeSession(id: string, data: string): Promise<void> {
    await WailsWriteSession(id, data);
  }

  public static async resizeSession(id: string, rows: number, cols: number): Promise<void> {
    await WailsResizeSession(id, rows, cols);
  }

  public static async stopSession(id: string): Promise<void> {
    await WailsStopSession(id);
  }

  public static async listSessions(): Promise<any[]> {
    const sessions = await WailsListSessions();
    return Array.isArray(sessions) ? sessions : [];
  }
}
