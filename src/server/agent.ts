// Agent manager facade — bridges the HTTP/WS surface to the streaming agent
// engine and the per-session JSONL store. Session transcripts live on disk;
// definitions remain in their own config file.

import fs from "fs";
import path from "path";
import os from "os";
import type {
  AgentDefinition,
  Session,
  SessionMeta,
} from "./agent/types";
import { SessionStore } from "./agent/store";
import type { LSPManager } from "./lsp";
import type { EditorManager } from "./editor";
import { AgentEngine } from "./agent/engine";
import { ExternalAgentManager, isExternalRole } from "./acp/manager";
import type { ProviderTarget } from "./agent/llm-client";
import type { McpToolSource, SkillLoader } from "./agent/engine";
import type { LLMManager } from "./llm";
import { executeLocalCommand } from "./slash";

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

export type AgentEventCallback = (eventName: string, payload: unknown) => void;

export class AgentManager {
  private dataDir: string;
  private definitionsFile: string;
  private definitions: AgentDefinition[] = [];
  private store: SessionStore;
  private engine: AgentEngine;
  private external: ExternalAgentManager;
  private onEventCallback: AgentEventCallback | null = null;
  private llmRef?: LLMManager | undefined = undefined;
  private mcpRef?: McpToolSource | undefined = undefined;
  private skillsRef?: SkillLoader | undefined = undefined;

  constructor(
    llm?: LLMManager,
    dataDir?: string,
    deps?: {
      mcp?: McpToolSource;
      skills?: SkillLoader;
      lsp?: LSPManager;
      editor?: EditorManager;
    }
  ) {
    this.dataDir = dataDir || path.join(os.homedir(), ".forge-ade");
    this.definitionsFile = path.join(this.dataDir, "agent_definitions.json");
    this.loadDefinitions();

    this.store = new SessionStore(this.dataDir);
    this.llmRef = llm;
    this.mcpRef = deps?.mcp;
    this.skillsRef = deps?.skills;
    this.engine = new AgentEngine(
      this.store,
      () => activeTarget(llm),
      (eventName, payload) => this.onEventCallback?.(eventName, payload),
      { dataDir: this.dataDir, mcp: deps?.mcp, skills: deps?.skills, lsp: deps?.lsp, editor: deps?.editor },
    );
    this.external = new ExternalAgentManager(
      this.store,
      (eventName, payload) => this.onEventCallback?.(eventName, payload),
      this.dataDir,
    );
  }

  public setOnEvent(callback: AgentEventCallback): void {
    this.onEventCallback = callback;
  }

  /** Human-readable usage summary for /usage. */
  public getUsageSummary(sessionId: string): string {
    const s = this.engine.getSession(sessionId);
    if (!s) return "session not found";
    const t = s.totalUsage;
    const last = s.lastUsage;
    const lines = [
      `session usage: ${t.promptTokens.toLocaleString()} in / ${t.completionTokens.toLocaleString()} out across ${t.requests} LLM calls`,
    ];
    if (last) {
      const tps = (last.completionTokens / Math.max(last.durationMs, 1) * 1000).toFixed(1);
      lines.push(`last call: ${last.promptTokens.toLocaleString()} in / ${last.completionTokens.toLocaleString()} out @ ${tps} tok/s`);
    }
    const ctxPct = ((last?.promptTokens ?? 0) / s.contextWindow * 100).toFixed(1);
    lines.push(`context window: ~${ctxPct}% of ${(s.contextWindow / 1000)}K`);
    return lines.join("\n");
  }

  // -- definitions -----------------------------------------------------------

  private loadDefinitions(): void {
    try {
      if (fs.existsSync(this.definitionsFile)) {
        const parsed: unknown = JSON.parse(fs.readFileSync(this.definitionsFile, "utf-8"));
        if (Array.isArray(parsed)) {
          this.definitions = parsed as AgentDefinition[];
          return;
        }
      }
    } catch {}
    this.definitions = DEFAULT_DEFINITIONS;
  }

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
    this.saveDefinitions();
    this.onEventCallback?.("agent:config:changed", {});
    return def;
  }

  public deleteDefinition(id: string): void {
    this.definitions = this.definitions.filter((d) => d.id !== id);
    this.saveDefinitions();
    this.onEventCallback?.("agent:config:changed", {});
  }

  private saveDefinitions(): void {
    try {
      fs.writeFileSync(this.definitionsFile, JSON.stringify(this.definitions, null, 2), "utf-8");
    } catch (err) {
      console.error("Failed to save agent definitions:", err);
    }
  }
  public createSession(name: string, role: string, projectFolder: string): Session {
    return this.engine.createSession(name, role, projectFolder);
  }

  /** Creates a session backed by an external ACP agent (omp, codex, ...). */
  public createExternalSession(agentKey: string, name: string, projectFolder: string): Promise<Session> {
    return this.external.createExternalSession(agentKey, name, projectFolder);
  }

  public listExternalAgents() {
    return this.external.listAgents();
  }

  /** Config options (model/mode/thinking) + slash commands of an external session. */
  public getExternalState(id: string) {
    return this.external.getExternalState(id);
  }

  public setExternalConfig(id: string, configId: string, value: string | boolean) {
    return this.external.setExternalConfig(id, configId, value);
  }


  public createSessionFromDefinition(defId: string, projectFolder: string): Session {
    const def = this.definitions.find((d) => d.id === defId) || DEFAULT_DEFINITIONS[0];
    const session = this.engine.createSession(def.name, def.role_filter || "coding", projectFolder);
    this.engine.updateSession(session.id, {
      customPrompt: def.prompt,
      customRules: def.rules,
    });
    return this.engine.getSession(session.id) ?? session;
  }

  /** Metadata only — full transcripts come via GetAgentSession. */
  public listSessions(): SessionMeta[] {
    return this.engine.listSessions();
  }

  public listSessionsForFolder(folder: string): SessionMeta[] {
    return this.engine.listSessionsForFolder(folder);
  }
  /** Full session incl. messages. External sessions read from the ACP
   *  manager's live copy — the engine's cache never sees ACP turns. */
  public getSession(id: string): Session | null {
    const external = this.external.getSession(id);
    if (external) return external;
    return this.engine.getSession(id);
  }

  public updateSession(
    id: string,
    name: string,
    role: string,
    customPrompt: string,
    customRules: string
  ): Session | null {
    return this.engine.updateSession(id, {
      name: name || undefined,
      role: role || undefined,
      customPrompt,
      customRules,
    });
  }

  public deleteSession(id: string): void {
    if (this.routeExternal(id) === "external") {
      this.external.deleteSession(id);
      return;
    }
    this.engine.deleteSession(id);
  }

  public clearSession(id: string): void {
    if (this.routeExternal(id) === "external") {
      this.external.clearSession(id);
      return;
    }
    this.engine.clearSession(id);
  }

  public setDialect(id: string, dialect: string): void {
    this.engine.setDialect(id, dialect);
  }

  public setAutoApprove(id: string, enabled: boolean): void {
    this.engine.setAutoApprove(id, enabled);
  }

  public toggleTask(sessionId: string, taskId: string, active: boolean): void {
    this.engine.toggleTask(sessionId, taskId, active);
  }

  /**
   * Single choke point for every chat surface: local slash commands
   * (/usage, /whoami, /login, /logout) execute here — they never reach the
   * model and never start a turn. The notice is persisted into the transcript
   * and broadcast so all clients render it.
   */
  public async sendMessage(
    sessionId: string,
    content: string,
    mentionedFiles: string[] = [],
    attachments: any[] = [],
  ): Promise<void> {
    const routed = this.routeExternal(sessionId);
    if (routed === "external") {
      await this.external.sendMessage(sessionId, content, mentionedFiles, attachments);
      return;
    }
    const local = await executeLocalCommand(
      content,
      { llm: this.llmRef, mcp: this.mcpRef, skills: this.skillsRef },
      { sessionId, sessionUsage: (id) => this.getUsageSummary(id) },
    );
    if (local) {
      // Echo the typed command into the transcript, then the output notice —
      // matching how terminal agents show `/cmd` followed by its result.
      const s = this.engine.getSession(sessionId);
      if (s) {
        const userMsg = {
          id: `msg-${Date.now()}-u`,
          role: "user" as const,
          content: [{ type: "text" as const, text: content }],
          timestamp: new Date().toISOString(),
        };
        s.messages.push(userMsg);
        this.store.appendMessage(sessionId, userMsg);
      }
      this.appendSystemNotice(sessionId, `${content.trim().split(/\s+/)[0]}\n${local.message}`);
      return;
    }
    await this.engine.sendMessage(sessionId, content, mentionedFiles, attachments);
  }

  private appendSystemNotice(sessionId: string, message: string): void {
    const s = this.engine.getSession(sessionId);
    if (!s) return;
    const msg = {
      id: `msg-${Date.now()}-sys`,
      role: "system" as const,
      content: [{ type: "text" as const, text: message }],
      timestamp: new Date().toISOString(),
    };
    s.messages.push(msg);
    this.store.appendMessage(sessionId, msg);
    this.onEventCallback?.("agent:notice", { id: sessionId, message });
    this.onEventCallback?.("agent:updated", { id: sessionId });
  }

  public respondApproval(sessionId: string, approve: boolean, autoAll: boolean): void {
    this.engine.respondApproval(sessionId, approve, autoAll);
  }

  public respondAsk(sessionId: string, answers: Record<string, unknown>): void {
    this.engine.respondAsk(sessionId, answers);
  }

  public stopTurn(sessionId: string): void {
    if (this.routeExternal(sessionId) === "external") {
      this.external.stopTurn(sessionId);
      return;
    }
    this.engine.stopTurn(sessionId);
  }

  /** Routes stop/delete/clear to the external manager for ACP-backed sessions. */
  private routeExternal(id: string): "external" | "internal" | "missing" {
    const ext = this.external.getSession(id);
    if (ext) return "external";
    const s = this.engine.getSession(id);
    if (!s) return "missing";
    return isExternalRole(s.role) ? "external" : "internal";
  }

  public applyDefinitionToSession(sessionId: string, defId: string): void {
    const def = this.definitions.find((d) => d.id === defId);
    if (!def) return;
    this.engine.updateSession(sessionId, {
      role: def.role_filter || undefined,
      customPrompt: def.prompt,
      customRules: def.rules,
    });
  }

  public stopAll(): void {
    try {
      this.external.stopAll();
    } catch {}
  }
}

function extractKeyAndUrl(p: any): { apiKey: string; baseURL: string } {
  if (!p) return { apiKey: "", baseURL: "" };
  const isGoogle = p.id?.startsWith("google-antigravity") || p.provider === "google-antigravity" || p.api === "google-antigravity";
  const apiKey = p.apiKey || p.api_key || (isGoogle && (p.refreshToken || p.refresh_token) ? "oauth" : "");
  const baseURL = p.baseURL || p.base_url || (isGoogle ? "https://daily-cloudcode-pa.googleapis.com" : "");
  return { apiKey, baseURL };
}

/** Resolves the provider profile into a concrete stream target with optional overrides. */
export function resolveTarget(
  llm?: LLMManager,
  providerId?: string,
  model?: string,
): ProviderTarget | null {
  if (!llm) return null;
  const config = llm.getLLMConfig();
  if (!config) return null;

  let chosenProfile: any = null;

  // 1. Try requested providerId if specified and has key/url
  if (providerId) {
    const found = config.profiles?.find(
      (p: any) => p.id === providerId || p.provider === providerId || p.name === providerId
    );
    if (found) {
      const { apiKey, baseURL } = extractKeyAndUrl(found);
      if (apiKey && baseURL) {
        chosenProfile = found;
      }
    }
  }

  // 2. Fall back to activeProfile
  if (!chosenProfile && config.activeProfile) {
    const { apiKey, baseURL } = extractKeyAndUrl(config.activeProfile);
    if (apiKey && baseURL) {
      chosenProfile = config.activeProfile;
    }
  }

  // 3. Fall back to any configured profile with a valid key and url
  if (!chosenProfile && config.profiles?.length) {
    for (const p of config.profiles) {
      if ((p as any).enabled === false) continue;
      const { apiKey, baseURL } = extractKeyAndUrl(p);
      if (apiKey && baseURL) {
        chosenProfile = p;
        break;
      }
    }
  }

  // 4. Final attempt: any enabled profile
  if (!chosenProfile && config.profiles?.length) {
    chosenProfile = config.profiles.find((p: any) => (p as any).enabled !== false) || config.profiles[0];
  }

  if (!chosenProfile) return null;

  const { apiKey, baseURL } = extractKeyAndUrl(chosenProfile);
  if (!apiKey || !baseURL) return null;

  const rawModels = chosenProfile.models || [];
  const firstModel = rawModels[0];
  const fallbackModel = typeof firstModel === "string" ? firstModel : firstModel?.id || "";
  const chosenModel = model || chosenProfile.activeModel || chosenProfile.active_model || fallbackModel || "default-model";

  const isGoogle =
    chosenProfile.id?.startsWith("google-antigravity") ||
    chosenProfile.provider === "google-antigravity" ||
    chosenProfile.api === "google-antigravity";
  const resolvedProviderId = isGoogle
    ? "google-antigravity"
    : chosenProfile.provider === "anthropic" || chosenProfile.id === "anthropic" || chosenProfile.api === "anthropic"
    ? "anthropic"
    : chosenProfile.provider || chosenProfile.api || chosenProfile.id || "openai";

  const contextWindow =
    chosenProfile.contextWindow ??
    chosenProfile.context_window ??
    (typeof firstModel === "object" ? firstModel?.context_window : undefined);
  const maxTokens =
    chosenProfile.maxTokens ??
    chosenProfile.max_tokens ??
    (typeof firstModel === "object" ? firstModel?.max_tokens : undefined);

  return {
    providerId: resolvedProviderId,
    baseURL,
    apiKey,
    model: chosenModel,
    ...(contextWindow !== undefined ? { contextWindow } : {}),
    ...(maxTokens !== undefined ? { maxTokens } : {}),
    ...(chosenProfile.projectId || chosenProfile.project_id ? { projectId: chosenProfile.projectId || chosenProfile.project_id } : {}),
  };
}
function activeTarget(llm?: LLMManager): ProviderTarget | null {
  return resolveTarget(llm);
}
