import { spawn, ChildProcess } from "child_process";
import os from "os";

export interface TerminalSession {
  id: string;
  name: string;
  type: "shell" | "ai";
  folder?: string;
  status: string;
  createdAt: number;
}

export type TerminalEventCallback = (eventName: string, payload: any) => void;

export class TerminalManager {
  private sessions: Map<string, { info: TerminalSession; process: ChildProcess | null }> = new Map();
  private onEventCallback: TerminalEventCallback | null = null;

  public setOnEvent(callback: TerminalEventCallback): void {
    this.onEventCallback = callback;
  }

  public createShell(name: string, cwd?: string): TerminalSession {
    const id = `shell-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`;
    const shell = process.env.SHELL || (os.platform() === "win32" ? "powershell.exe" : "/bin/zsh");
    const workingDir = cwd || os.homedir();

    const info: TerminalSession = {
      id,
      name: name || "Shell",
      type: "shell",
      folder: workingDir,
      status: "running",
      createdAt: Date.now(),
    };

    try {
      const child = spawn(shell, ["-l", "-i"], {
        cwd: workingDir,
        env: {
          ...process.env,
          TERM: "xterm-256color",
          COLORTERM: "truecolor",
        },
        stdio: ["pipe", "pipe", "pipe"],
      });

      child.stdout?.on("data", (chunk: Buffer) => {
        const text = chunk.toString("utf-8");
        if (this.onEventCallback) {
          this.onEventCallback("session:output", { id, data: text });
        }
      });

      child.stderr?.on("data", (chunk: Buffer) => {
        const text = chunk.toString("utf-8");
        if (this.onEventCallback) {
          this.onEventCallback("session:output", { id, data: text });
        }
      });

      child.on("exit", () => {
        this.sessions.delete(id);
        if (this.onEventCallback) {
          this.onEventCallback("session:closed", { id });
        }
      });

      this.sessions.set(id, { info, process: child });

      if (this.onEventCallback) {
        this.onEventCallback("session:opened", info);
      }
    } catch (err) {
      console.error("Failed to spawn shell:", err);
      this.sessions.set(id, { info, process: null });
    }

    return info;
  }

  public createAIAgent(name: string, provider: string, folder?: string): TerminalSession {
    const id = `agent-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`;
    const info: TerminalSession = {
      id,
      name: name || "AI Agent",
      type: "ai",
      folder: folder || os.homedir(),
      status: "running",
      createdAt: Date.now(),
    };
    this.sessions.set(id, { info, process: null });
    if (this.onEventCallback) {
      this.onEventCallback("session:opened", info);
    }
    return info;
  }

  public writeSession(id: string, data: string): void {
    const session = this.sessions.get(id);
    if (session?.process?.stdin && !session.process.stdin.destroyed) {
      session.process.stdin.write(data);
    }
  }

  public resizeSession(id: string, rows: number, cols: number): void {
    // Terminal resize signal
  }

  public stopSession(id: string): void {
    const session = this.sessions.get(id);
    if (session?.process) {
      session.process.kill("SIGTERM");
    }
    this.sessions.delete(id);
    if (this.onEventCallback) {
      this.onEventCallback("session:closed", { id });
    }
  }

  public renameSession(id: string, name: string): void {
    const session = this.sessions.get(id);
    if (session) {
      session.info.name = name;
      if (this.onEventCallback) {
        this.onEventCallback("session:renamed", { id, name });
      }
    }
  }

  public listSessions(): TerminalSession[] {
    return Array.from(this.sessions.values()).map((s) => s.info);
  }

  public listByType(type: "shell" | "ai"): TerminalSession[] {
    return this.listSessions().filter((s) => s.type === type);
  }
}
