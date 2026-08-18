import fs from "fs";
import path from "path";
import os from "os";

export interface ContentBlock {
  type: "text" | "thinking" | "tool_call" | "tool_result";
  text?: string;
  tool_call_id?: string;
  name?: string;
  arguments?: Record<string, any>;
  is_error?: boolean;
}

export interface AgentMessage {
  id: string;
  role: "user" | "assistant" | "system" | "tool";
  content: ContentBlock[];
  timestamp: string;
}

export interface AgentSession {
  id: string;
  name: string;
  role: string;
  projectFolder: string;
  messages: AgentMessage[];
  customPrompt?: string;
  customRules?: string;
  dialect?: string;
  autoApprove?: boolean;
  tasks?: { id: string; title: string; completed: boolean }[];
  createdAt: number;
}

export interface AgentDefinition {
  id: string;
  name: string;
  role_filter?: string;
  description: string;
  prompt: string;
  rules: string;
  model?: string;
  color?: string;
}

const DEFAULT_DEFINITIONS: AgentDefinition[] = [
  {
    id: "coder",
    name: "Full-Stack Engineer",
    role_filter: "coding",
    description: "Builds features, fixes bugs, and runs refactors with tool access.",
    prompt: "You are an expert full-stack engineer. Write clean, idiomatic code.",
    rules: "1. Read files before editing.\n2. Verify changes with tests.",
    model: "claude-3-7-sonnet-20250219",
  },
  {
    id: "planner",
    name: "Architect & Planner",
    role_filter: "planning",
    description: "Designs system architectures and breaks down complex phases.",
    prompt: "You are a software architect. Create crisp, structured plans.",
    rules: "1. List constraints.\n2. Break down into discrete phases.",
    model: "claude-3-7-sonnet-20250219",
  },
  {
    id: "researcher",
    name: "Research Scout",
    role_filter: "research",
    description: "Investigates APIs, repos, and documentation.",
    prompt: "You are a research scout. Gather exact facts from sources.",
    rules: "1. Be evidence-first.\n2. Cite exact files and symbols.",
    model: "claude-3-5-haiku-20241022",
  },
];

export type AgentEventCallback = (eventName: string, payload: any) => void;

export class AgentManager {
  private dataDir: string;
  private sessionsFile: string;
  private definitionsFile: string;
  private sessions: AgentSession[] = [];
  private definitions: AgentDefinition[] = [];
  private onEventCallback: AgentEventCallback | null = null;

  constructor(dataDir?: string) {
    this.dataDir = dataDir || path.join(os.homedir(), ".forge-ade");
    this.sessionsFile = path.join(this.dataDir, "agent_sessions.json");
    this.definitionsFile = path.join(this.dataDir, "agent_definitions.json");
    this.loadState();
  }

  public setOnEvent(callback: AgentEventCallback): void {
    this.onEventCallback = callback;
  }

  private loadState(): void {
    try {
      if (fs.existsSync(this.sessionsFile)) {
        this.sessions = JSON.parse(fs.readFileSync(this.sessionsFile, "utf-8"));
      }
    } catch {
      this.sessions = [];
    }

    try {
      if (fs.existsSync(this.definitionsFile)) {
        this.definitions = JSON.parse(fs.readFileSync(this.definitionsFile, "utf-8"));
      } else {
        this.definitions = DEFAULT_DEFINITIONS;
      }
    } catch {
      this.definitions = DEFAULT_DEFINITIONS;
    }
  }

  private saveState(): void {
    try {
      fs.writeFileSync(this.sessionsFile, JSON.stringify(this.sessions, null, 2), "utf-8");
      fs.writeFileSync(this.definitionsFile, JSON.stringify(this.definitions, null, 2), "utf-8");
    } catch (err) {
      console.error("Failed to save agent state:", err);
    }
  }

  public createSession(name: string, role: string, projectFolder: string): AgentSession {
    const id = `agent-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`;
    const session: AgentSession = {
      id,
      name: name || "Agent Session",
      role: role || "coding",
      projectFolder: projectFolder || process.cwd(),
      messages: [],
      createdAt: Date.now(),
    };
    this.sessions.unshift(session);
    this.saveState();
    if (this.onEventCallback) {
      this.onEventCallback("session:opened", session);
    }
    return session;
  }

  public createSessionFromDefinition(defId: string, projectFolder: string): AgentSession {
    const def = this.definitions.find((d) => d.id === defId) || DEFAULT_DEFINITIONS[0];
    const session = this.createSession(def.name, def.role_filter || "coding", projectFolder);
    session.customPrompt = def.prompt;
    session.customRules = def.rules;
    this.saveState();
    return session;
  }

  public listSessions(): AgentSession[] {
    return [...this.sessions];
  }

  public listSessionsForFolder(folder: string): AgentSession[] {
    const norm = path.resolve(folder || process.cwd());
    return this.sessions.filter((s) => path.resolve(s.projectFolder) === norm);
  }

  public getSession(id: string): AgentSession | null {
    return this.sessions.find((s) => s.id === id) || null;
  }

  public updateSession(
    id: string,
    name: string,
    role: string,
    customPrompt: string,
    customRules: string
  ): AgentSession | null {
    const session = this.getSession(id);
    if (session) {
      if (name) session.name = name;
      if (role) session.role = role;
      if (customPrompt !== undefined) session.customPrompt = customPrompt;
      if (customRules !== undefined) session.customRules = customRules;
      this.saveState();
      if (this.onEventCallback) {
        this.onEventCallback("agent:updated", { id });
      }
    }
    return session;
  }

  public deleteSession(id: string): void {
    this.sessions = this.sessions.filter((s) => s.id !== id);
    this.saveState();
    if (this.onEventCallback) {
      this.onEventCallback("session:closed", { id });
    }
  }

  public clearSession(id: string): void {
    const session = this.getSession(id);
    if (session) {
      session.messages = [];
      this.saveState();
      if (this.onEventCallback) {
        this.onEventCallback("agent:updated", { id });
      }
    }
  }

  public setDialect(id: string, dialect: string): void {
    const session = this.getSession(id);
    if (session) {
      session.dialect = dialect;
      this.saveState();
    }
  }

  public setAutoApprove(id: string, enabled: boolean): void {
    const session = this.getSession(id);
    if (session) {
      session.autoApprove = enabled;
      this.saveState();
    }
  }

  public toggleTask(sessionId: string, taskId: string, completed: boolean): void {
    const session = this.getSession(sessionId);
    if (session?.tasks) {
      const task = session.tasks.find((t) => t.id === taskId);
      if (task) {
        task.completed = completed;
        this.saveState();
      }
    }
  }

  public async sendMessage(sessionId: string, content: string, mentionedFiles: string[] = []): Promise<void> {
    const session = this.getSession(sessionId);
    if (!session) return;

    const userMsg: AgentMessage = {
      id: `msg-${Date.now()}`,
      role: "user",
      content: [{ type: "text", text: content }],
      timestamp: new Date().toISOString(),
    };
    session.messages.push(userMsg);
    this.saveState();

    if (this.onEventCallback) {
      this.onEventCallback("agent:updated", { id: sessionId });
    }

    // Assistant response with thinking and text blocks
    setTimeout(() => {
      const responseId = `msg-${Date.now() + 100}`;
      const assistantMsg: AgentMessage = {
        id: responseId,
        role: "assistant",
        content: [
          {
            type: "thinking",
            text: `Analyzing query in workspace: ${session.projectFolder}\nContext: ${mentionedFiles.join(", ") || "none"}`,
          },
          {
            type: "text",
            text: `I'm ready to assist with your workspace at \`${session.projectFolder}\`.\n\nYou can ask me to inspect code, run terminal commands, manage git changes, or scaffold new components.`,
          },
        ],
        timestamp: new Date().toISOString(),
      };
      session.messages.push(assistantMsg);
      this.saveState();

      if (this.onEventCallback) {
        this.onEventCallback("agent:updated", { id: sessionId });
      }
    }, 200);
  }

  public respondApproval(sessionId: string, approve: boolean, autoAll: boolean): void {
    if (autoAll) {
      this.setAutoApprove(sessionId, true);
    }
  }

  public respondAsk(sessionId: string, answers: any): void {}

  public stopTurn(sessionId: string): void {}

  public listDefinitions(): AgentDefinition[] {
    return [...this.definitions];
  }

  public saveDefinition(def: AgentDefinition): AgentDefinition {
    const existingIndex = this.definitions.findIndex((d) => d.id === def.id);
    if (existingIndex >= 0) {
      this.definitions[existingIndex] = def;
    } else {
      this.definitions.push(def);
    }
    this.saveState();
    if (this.onEventCallback) {
      this.onEventCallback("agent:config:changed", {});
    }
    return def;
  }

  public deleteDefinition(id: string): void {
    this.definitions = this.definitions.filter((d) => d.id !== id);
    this.saveState();
    if (this.onEventCallback) {
      this.onEventCallback("agent:config:changed", {});
    }
  }

  public applyDefinitionToSession(sessionId: string, defId: string): void {
    const session = this.getSession(sessionId);
    const def = this.definitions.find((d) => d.id === defId);
    if (session && def) {
      session.role = def.role_filter || "coding";
      session.customPrompt = def.prompt;
      session.customRules = def.rules;
      this.saveState();
      if (this.onEventCallback) {
        this.onEventCallback("agent:updated", { id: sessionId });
      }
    }
  }
}
