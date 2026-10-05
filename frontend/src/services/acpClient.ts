import { ACPAgent, FileDiff } from '../types';

export interface ACPMessage {
  jsonrpc: '2.0';
  id?: string | number;
  method?: string;
  params?: Record<string, unknown>;
  result?: unknown;
  error?: { code: number; message: string; data?: unknown };
}

export interface ACPSessionOptions {
  agentId: string;
  workspacePath: string;
  port?: number;
  transport?: 'websocket' | 'stdio' | 'pipe';
  autoReconnect?: boolean;
}

export class ACPClient {
  private status: 'connected' | 'connecting' | 'disconnected' = 'connected';
  private agent: ACPAgent | null = null;
  private listeners: Map<string, Array<(data: unknown) => void>> = new Map();

  constructor(agent?: ACPAgent) {
    if (agent) {
      this.agent = agent;
    }
  }

  public getStatus(): 'connected' | 'connecting' | 'disconnected' {
    return this.status;
  }

  public on(event: string, callback: (data: unknown) => void) {
    if (!this.listeners.has(event)) {
      this.listeners.set(event, []);
    }
    this.listeners.get(event)?.push(callback);
  }

  public off(event: string, callback: (data: unknown) => void) {
    const arr = this.listeners.get(event) || [];
    this.listeners.set(event, arr.filter(cb => cb !== callback));
  }

  private emit(event: string, data: unknown) {
    const callbacks = this.listeners.get(event) || [];
    callbacks.forEach(cb => cb(data));
  }

  public async connect(): Promise<boolean> {
    this.status = 'connecting';
    this.emit('statusChange', this.status);

    await new Promise(r => setTimeout(r, 200));
    this.status = 'connected';
    this.emit('statusChange', this.status);
    return true;
  }

  public async sendPrompt(prompt: string, context?: Record<string, unknown>): Promise<void> {
    this.emit('promptSent', { prompt, context, timestamp: new Date().toISOString() });
  }

  public async applyDiff(diff: FileDiff): Promise<boolean> {
    this.emit('diffApplied', { diffId: diff.id, filePath: diff.filePath });
    return true;
  }

  public disconnect() {
    this.status = 'disconnected';
    this.emit('statusChange', this.status);
  }
}

export const globalACPClient = new ACPClient();
