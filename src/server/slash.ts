import type { LLMManager } from "./llm";
import type { McpToolSource, SkillLoader } from "./agent/engine";
import { getAggregatedUsage, fetchAntigravityQuota, getAllAntigravityQuotas, formatMultiAccountAsciiQuota } from "./auth/quota";
export interface SlashCommand {
  name: string;
  description: string;
  kind: "builtin" | "skill" | "auth" | "stats";
}

export interface SlashCommandDeps {
  skills?: SkillLoader | undefined;
  mcp?: McpToolSource | undefined;
  llm?: LLMManager | undefined;
}

export interface LocalCommandContext {
  sessionId?: string | undefined;
  sessionUsage?: ((sessionId: string) => string) | undefined;
}
export interface CommandExecutionResult {
  handled: true;
  message: string;
}

const BUILTIN_COMMANDS: SlashCommand[] = [
  { name: "/help", description: "List available commands and tools", kind: "builtin" },
  { name: "/clear", description: "Clear this conversation", kind: "builtin" },
  { name: "/commit", description: "Generate an AI commit message from staged changes", kind: "builtin" },
  { name: "/plan", description: "Ask the agent for a step-by-step plan", kind: "builtin" },
  { name: "/review", description: "Security and quality review of modified files", kind: "builtin" },
  { name: "/model", description: "Switch active model/provider", kind: "builtin" },
  { name: "/usage", description: "Token usage for this session and totals", kind: "stats" },
  { name: "/mcp", description: "List MCP servers, connection state and tools", kind: "builtin" },
  { name: "/whoami", description: "Show active provider, model and account state", kind: "auth" },
  { name: "/login", description: "Add or update a provider API key (/login <provider> <key>)", kind: "auth" },
  { name: "/logout", description: "Remove the stored API key for a provider (/logout [provider])", kind: "auth" },
];

/** All commands, optionally filtered by what the user has typed so far. */
export function listSlashCommands(deps: SlashCommandDeps, query?: string, projectFolder?: string): SlashCommand[] {
  const all: SlashCommand[] = [...BUILTIN_COMMANDS];
  if (deps.skills) {
    for (const skill of deps.skills.listSkills(projectFolder)) {
      all.push({ name: `/skill:${skill.name}`, description: skill.description || "Skill", kind: "skill" });
    }
  }
  const q = (query ?? "").trim().toLowerCase();
  if (!q.startsWith("/")) return [];
  return all.filter((c) => c.name.toLowerCase().startsWith(q));
}

function maskKey(key: string): string {
  if (key.length <= 8) return "***";
  return `${key.slice(0, 4)}…${key.slice(-4)}`;
}

interface ProfileLike {
  id?: string;
  name?: string;
  provider?: string;
  apiKey?: string;
  baseURL?: string;
  activeModel?: string;
}

function llmProfiles(llm: LLMManager): { profiles: ProfileLike[]; activeId: string | undefined } {
  const profiles: ProfileLike[] = llm.getProviderProfiles().map((p) => ({
    id: p.id,
    name: p.name,
    provider: p.provider,
    apiKey: p.apiKey,
    baseURL: p.baseURL,
    activeModel: p.activeModel,
  }));
  const config = llm.getLLMConfig() as { activeProfile?: { id?: string } | null };
  return { profiles, activeId: config.activeProfile?.id };
}
/**
 * Executes local (non-agent) commands. Returns null when the text is not a
 * local command so it flows to the agent instead.
 */
export async function executeLocalCommand(
  text: string,
  deps: SlashCommandDeps,
  context?: LocalCommandContext,
): Promise<CommandExecutionResult | null> {
  const trimmed = text.trim();
  if (!trimmed.startsWith("/")) return null;

  const spaceIdx = trimmed.indexOf(" ");
  const cmd = (spaceIdx === -1 ? trimmed : trimmed.slice(0, spaceIdx)).toLowerCase();
  const rest = spaceIdx === -1 ? "" : trimmed.slice(spaceIdx + 1).trim();

  switch (cmd) {
    case "/whoami": {
      if (!deps.llm) return { handled: true, message: "LLM manager unavailable" };
      const { profiles, activeId } = llmProfiles(deps.llm);
      const lines: string[] = [];
      for (const p of profiles) {
        const marker = p.id === activeId ? "*" : " ";
        lines.push(`${marker} ${p.id} — model ${p.activeModel || "(unset)"}, key ${p.apiKey ? maskKey(p.apiKey) : "not set"}, ${p.baseURL || "(no url)"}`);
      }
      if (lines.length === 0) lines.push("(no providers configured)");
      if (deps.mcp) {
        const servers = deps.mcp.listServers();
        lines.push(`mcp: ${servers.filter((s: any) => s.connected).length}/${servers.length} servers connected`);
      }
      return { handled: true, message: `* = active\n${lines.join("\n")}` };
    }


    case "/mcp": {
      if (!deps.mcp) return { handled: true, message: "MCP manager unavailable" };
      const servers = deps.mcp.listServers();
      if (servers.length === 0) return { handled: true, message: "no MCP servers discovered — add one to ~/.forge-ade/mcp.json or settings" };
      const lines = servers.map((s) => {
        const state = s.enabled === false
          ? "[disabled]"
          : s.connected
            ? "connected"
            : `failed (${s.error ?? "unknown"})`;
        return `${s.connected ? "●" : "○"} ${s.name} — ${state} [${s.source}]`;
      });
      lines.push(`tools available to agents: ${deps.mcp.listConnectedTools().length}`);
      return { handled: true, message: lines.join("\n") };
    }

    case "/login": {
      if (!deps.llm) return { handled: true, message: "LLM manager unavailable" };
      const parts = rest.split(/\s+/).filter(Boolean);
      if (parts.length < 2) {
        return {
          handled: true,
          message:
            "usage: /login <providerId> <apiKey>\nthe key is stored locally in ~/.forge-ade/llm_config.json; base URL/model are preserved",
        };
      }
      const [providerId, apiKey] = parts;
      const existing = deps.llm.getProviderProfiles().find((p) => p.id === providerId);
      if (!existing) {
        return { handled: true, message: `unknown provider "${providerId}" — configure it in settings first` };
      }
      // Patch only the key; keep base URL and model so the profile stays valid.
      deps.llm.saveLLMProfile(providerId, apiKey, existing.baseURL, existing.activeModel);
      deps.llm.setActiveModel(providerId, existing.activeModel);
      return { handled: true, message: `stored API key for "${providerId}" (${maskKey(apiKey)})` };
    }

    case "/logout": {
      if (!deps.llm) return { handled: true, message: "LLM manager unavailable" };
      const { profiles } = llmProfiles(deps.llm);
      const targetId = rest || profiles.find((p) => p.apiKey)?.id || "";
      const profile = profiles.find((p) => p.id === targetId);
      if (!profile) return { handled: true, message: "nothing to log out of" };
      if (!profile.apiKey) return { handled: true, message: `"${targetId}" has no stored key` };
      deps.llm.saveLLMProfile(targetId, "", profile.baseURL ?? "", profile.activeModel ?? "");
      return { handled: true, message: `removed stored credentials for "${targetId}"` };
    }

    case "/usage": {
      const summary = getAggregatedUsage();
      const cfg = deps.llm?.getLLMConfig() as any;
      const active = cfg?.activeProfile;

      const lines: string[] = [];

      // Check for Google Antigravity quotas across all connected accounts
      const antigravityReports = deps.llm ? await getAllAntigravityQuotas(deps.llm) : [];
      if (antigravityReports.length > 0) {
        lines.push("```text");
        lines.push(formatMultiAccountAsciiQuota(antigravityReports));
        lines.push("```");
        lines.push("");
      }

      lines.push(`• **Active Provider:** \`${active?.name || active?.id || "None"}\``);
      lines.push(`• **Active Model:** \`${active?.activeModel || "None"}\``);
      lines.push("");

      if (context?.sessionId && context.sessionUsage) {
        const sessionReport = context.sessionUsage(context.sessionId);
        if (sessionReport && sessionReport.trim()) {
          lines.push("### Current Session");
          lines.push(sessionReport.trim());
          lines.push("");
        }
      }

      lines.push("### Workspace Totals");
      lines.push(`- **Prompt Tokens:** ${summary.totalPromptTokens.toLocaleString()}`);
      lines.push(`- **Completion Tokens:** ${summary.totalCompletionTokens.toLocaleString()}`);
      lines.push(`- **Cached Tokens:** ${summary.totalCachedTokens.toLocaleString()} (${summary.cacheHitRate}% cache hit)`);
      lines.push(`- **Total Tokens:** ${summary.totalTokens.toLocaleString()}`);
      lines.push(`- **Total Requests:** ${summary.requestCount}`);

      return { handled: true, message: lines.join("\n") };
    }
    default:
      return null; // not a local command — let it flow to the agent
  }
}
