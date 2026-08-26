// LLM manager — thin adapter over ConfigStore's models.json. All providers,
// API keys (auth), and the model catalog with per-model metadata live there;
// this class only shapes them for the bridge and the agent engine.

import { ConfigStore } from "./config";
import type { ProviderAuth, ModelMeta } from "./config";

export interface ProviderProfile {
  id: string;
  name: string;
  provider: string;
  apiKey: string;
  baseURL: string;
  activeModel: string;
  models: ModelMeta[];
  /** Plain model ids, kept for settings-modal compatibility. */
  selected_models: string[];
  enabled: boolean;
}

export interface ProviderPreset {
  id: string;
  name: string;
  baseURL: string;
  models: string[];
  authType?: "oauth" | "api_key" | "device";
  keyUrl?: string;
  description?: string;
}

export const DEFAULT_PROVIDERS: ProviderPreset[] = [
  {
    id: "google-antigravity",
    name: "Google Antigravity",
    baseURL: "https://daily-cloudcode-pa.googleapis.com",
    models: [
      "gemini-3.7-flash-tiered",
      "gemini-2.5-pro",
      "gemini-2.5-flash",
      "claude-3-7-sonnet",
      "claude-3-5-sonnet",
    ],
    authType: "oauth",
    description: "Gemini 3, Claude, GPT-OSS via Google Cloud Code Assist OAuth",
  },
  {
    id: "openrouter",
    name: "OpenRouter",
    baseURL: "https://openrouter.ai/api/v1",
    models: [
      "anthropic/claude-3.7-sonnet",
      "openai/gpt-4o",
      "deepseek/deepseek-r1",
      "meta-llama/llama-3.3-70b-instruct",
    ],
    authType: "api_key",
    keyUrl: "https://openrouter.ai/keys",
    description: "Multi-provider AI routing endpoint",
  },
  {
    id: "opencode-go",
    name: "OpenCode Go",
    baseURL: "https://api.opencode.ai/v1",
    models: [
      "claude-3-7-sonnet",
      "claude-3-5-sonnet",
      "gemini-2.5-pro",
      "gpt-4o",
    ],
    authType: "api_key",
    keyUrl: "https://opencode.ai/account",
    description: "OpenCode high-throughput coding models",
  },
  {
    id: "opencode-zen",
    name: "OpenCode Zen",
    baseURL: "https://zen.opencode.ai/v1",
    models: ["zen-r1", "zen-coder"],
    authType: "api_key",
    keyUrl: "https://opencode.ai/zen",
    description: "OpenCode Zen reasoning models",
  },
  {
    id: "kilo",
    name: "KiloCode",
    baseURL: "https://api.kilo.ai/v1",
    models: ["kilo-coder", "claude-3.7-sonnet", "deepseek-r1", "gpt-4o"],
    authType: "device",
    keyUrl: "https://kilo.ai/keys",
    description: "KiloCode AI device login & API key",
  },
  {
    id: "vercel-ai-gateway",
    name: "Vercel AI Gateway",
    baseURL: "https://api.vercel.ai/v1",
    models: ["anthropic/claude-3.7-sonnet", "openai/gpt-4o"],
    authType: "api_key",
    keyUrl: "https://vercel.com/docs/ai/ai-gateway",
    description: "Edge AI gateway from Vercel",
  },
  {
    id: "openai",
    name: "OpenAI",
    baseURL: "https://api.openai.com/v1",
    models: ["gpt-4o", "gpt-4o-mini", "o1", "o3-mini"],
    authType: "api_key",
    keyUrl: "https://platform.openai.com/api-keys",
  },
  {
    id: "anthropic",
    name: "Anthropic",
    baseURL: "https://api.anthropic.com/v1",
    models: [
      "claude-3-7-sonnet-20250219",
      "claude-3-5-sonnet-20241022",
      "claude-3-5-haiku-20241022",
    ],
    authType: "api_key",
    keyUrl: "https://console.anthropic.com/settings/keys",
  },
  {
    id: "custom",
    name: "Custom (OpenAI-compatible)",
    baseURL: "http://localhost:11434/v1",
    models: ["default-model"],
    authType: "api_key",
    description: "Self-hosted vLLM, Ollama, LiteLLM, or LM Studio",
  },
];

function toProfile(p: ProviderAuth): ProviderProfile {
  const allIds = p.models.map((m) => m.id);
  const hasExplicit = Array.isArray(p.selected_models);
  const selection = hasExplicit
    ? p.selected_models!.filter((id) => allIds.includes(id))
    : allIds;
  return {
    id: p.id,
    name: p.name,
    provider: p.api,
    apiKey: p.api_key,
    baseURL: p.base_url,
    activeModel: p.active_model,
    models: p.models,
    selected_models: selection,
    enabled: p.enabled !== false,
    ...((p as any).projectId ? { projectId: (p as any).projectId } : {}),
    ...((p as any).accountEmail
      ? { accountEmail: (p as any).accountEmail }
      : {}),
    ...((p as any).refreshToken
      ? { refreshToken: (p as any).refreshToken }
      : {}),
  };
}

/** Normalizes a mixed model list into plain id strings. */
function toIdList(value: unknown): string[] {
  if (!Array.isArray(value)) return [];
  return value
    .map((m) => {
      if (typeof m === "string") return m;
      if (
        m &&
        typeof m === "object" &&
        typeof (m as { id?: unknown }).id === "string"
      ) {
        return (m as { id: string }).id;
      }
      return "";
    })
    .filter((s): s is string => s.length > 0);
}

export class LLMManager {
  private store: ConfigStore;

  constructor(store: ConfigStore) {
    this.store = store;
  }

  public getProviderProfiles(): ProviderProfile[] {
    return Object.values(this.store.getProviders()).map(toProfile);
  }

  public saveProviderProfiles(profiles: unknown[]): void {
    this.store.saveModels((models) => {
      const next: Record<string, ProviderAuth> = {};
      for (const raw of profiles) {
        if (!raw || typeof raw !== "object") continue;
        const rec = raw as Record<string, unknown>;
        const pick = (...keys: string[]): string => {
          for (const k of keys) {
            const v = rec[k];
            if (typeof v === "string" && v.length > 0) return v;
          }
          return "";
        };
        const id = pick("id", "provider_id", "name");
        if (!id) continue;
        const existing = models.providers[id];
        const apiKey = pick("apiKey", "api_key", "ApiKey");
        const baseURL = pick("baseURL", "base_url", "BaseURL");
        const activeModel =
          pick("activeModel", "active_model") || existing?.active_model || "";

        // Catalog = every model the provider offers. The settings modal sends
        // available_models/selected_models WITHOUT a "models" field (new
        // providers have no stored catalog yet), so derive the catalog from
        // the union of all three lists instead of falling back to empty.
        const rawCatalog: unknown[] = Array.isArray(rec.models)
          ? rec.models
          : [];
        const hasExplicitSelection = Array.isArray(rec.selected_models);
        const rawSelection: unknown[] = hasExplicitSelection
          ? (rec.selected_models as unknown[])
          : Array.isArray(rec.available_models)
            ? rec.available_models
            : [];
        const rawAvailable: unknown[] = Array.isArray(rec.available_models)
          ? rec.available_models
          : [];
        const catalogSource = [
          ...rawCatalog,
          ...rawAvailable,
          ...rawSelection,
          ...(existing?.models ?? []),
        ];
        const catalog: ModelMeta[] = [];
        const seen = new Set<string>();
        for (const m of catalogSource) {
          if (typeof m === "string") {
            if (seen.has(m)) continue;
            seen.add(m);
            catalog.push(existing?.models.find((x) => x.id === m) ?? { id: m });
            continue;
          }
          if (
            !m ||
            typeof m !== "object" ||
            !("id" in m) ||
            typeof m.id !== "string"
          )
            continue;
          const id = m.id;
          if (seen.has(id)) continue;
          seen.add(id);
          const prior = existing?.models.find((x) => x.id === id);
          const out: ModelMeta = { id };
          const name = "name" in m ? m.name : prior?.name;
          if (typeof name === "string") out.name = name;
          const reasoning = "reasoning" in m ? m.reasoning : prior?.reasoning;
          if (typeof reasoning === "boolean") out.reasoning = reasoning;
          // Provider payload shape — structurally identical to ModelMeta minus
          // the required id; each optional field is individually validated.
          const src = m as {
            id: string;
            name?: unknown;
            reasoning?: unknown;
            context_window?: unknown;
            max_tokens?: unknown;
          };
          if (typeof src.name === "string") out.name = src.name;
          if (typeof src.reasoning === "boolean") out.reasoning = src.reasoning;
          if (typeof src.context_window === "number")
            out.context_window = src.context_window;
          else if (prior && typeof prior.context_window === "number")
            out.context_window = prior.context_window;
          if (typeof src.max_tokens === "number")
            out.max_tokens = src.max_tokens;
          else if (prior && typeof prior.max_tokens === "number")
            out.max_tokens = prior.max_tokens;
          catalog.push(out);
        }

        const selectionIds = toIdList(rawSelection);
        const selectedModels =
          hasExplicitSelection && selectionIds.length > 0
            ? selectionIds.filter((sid) => catalog.some((m) => m.id === sid))
            : undefined; // no explicit selection → picker shows the full catalog

        const enabledRaw = rec.enabled;
        next[id] = {
          id,
          name: pick("name") || existing?.name || id,
          api:
            id.startsWith("google-antigravity") ||
            pick("provider") === "google-antigravity"
              ? "google-antigravity"
              : pick("provider") === "anthropic"
                ? "anthropic"
                : pick("provider") || existing?.api || "openai-completions",
          base_url: baseURL || existing?.base_url || "",
          api_key: apiKey || existing?.api_key || "",
          auth: "apiKey",
          active_model:
            activeModel || catalog[0]?.id || existing?.active_model || "",
          models: catalog.length > 0 ? catalog : (existing?.models ?? []),
          ...(hasExplicitSelection
            ? { selected_models: selectedModels || [] }
            : {}),
          ...(enabledRaw === false ? { enabled: false } : {}),
          ...(rec.projectId || (existing as any)?.projectId
            ? {
                projectId: (rec.projectId ||
                  (existing as any)?.projectId) as string,
              }
            : {}),
          ...(rec.accountEmail || (existing as any)?.accountEmail
            ? {
                accountEmail: (rec.accountEmail ||
                  (existing as any)?.accountEmail) as string,
              }
            : {}),
          ...(rec.refreshToken || (existing as any)?.refreshToken
            ? {
                refreshToken: (rec.refreshToken ||
                  (existing as any)?.refreshToken) as string,
              }
            : {}),
        };
      }

      if (profiles.length === 1 && Object.keys(models.providers).length > 0) {
        for (const [k, v] of Object.entries(next)) {
          models.providers[k] = v;
        }
      } else {
        models.providers = next;
      }
      // A keyless entry sharing an endpoint with a keyed one inherits the key
      // (e.g. the settings modal re-saving an opencode-imported router).
      const knownKeys = this.store.knownKeysByUrl();
      for (const p of Object.values(models.providers)) {
        if (p.api_key) continue;
        const donor = Object.values(models.providers).find(
          (other) => other.api_key && other.base_url === p.base_url,
        );
        if (donor) {
          p.api_key = donor.api_key;
          continue;
        }
        const known = knownKeys.get(p.base_url);
        if (known) {
          console.log(
            `[llm] restored key for "${p.id}" from config history (${p.base_url})`,
          );
          p.api_key = known;
        }
      }

      // Keep a usable default: key + model first, then model catalog, then keyed.
      const ranked = Object.values(models.providers).filter(
        (p) => p.enabled !== false,
      );
      const ready = ranked.find(
        (p) => p.api_key.length > 0 && p.active_model.length > 0,
      );
      const withModel = ranked.find((p) => p.models.length > 0);
      const keyed = ranked.find((p) => p.api_key.length > 0);
      const best =
        ready?.id ?? withModel?.id ?? keyed?.id ?? Object.keys(next)[0] ?? "";
      if (
        !next[models.default_provider] ||
        next[models.default_provider]?.enabled === false
      ) {
        models.default_provider = best;
      } else if (!next[models.default_provider].api_key) {
        // A keyless default is a broken chat; hand the crown to a usable one.
        models.default_provider = best;
      }
    });
  }

  public setActiveModel(providerId: string, model: string): void {
    this.store.saveModels((models) => {
      const provider = models.providers[providerId];
      if (!provider) return;
      provider.active_model = model;
      if (!provider.models.some((m) => m.id === model))
        provider.models.push({ id: model });
      models.default_provider = providerId;
    });
  }

  public saveLLMProfile(
    providerId: string,
    apiKey: string,
    baseURL: string,
    model: string,
  ): void {
    this.store.saveModels((models) => {
      let provider = models.providers[providerId];
      if (!provider) {
        const known = DEFAULT_PROVIDERS.find((d) => d.id === providerId);
        provider = {
          id: providerId,
          name: known?.name ?? providerId,
          api: providerId === "anthropic" ? "anthropic" : "openai-completions",
          base_url: known?.baseURL ?? "",
          api_key: "",
          auth: "apiKey",
          active_model: known?.models[0] ?? "",
          models: (known?.models ?? []).map((m) => ({ id: m })),
        };
        models.providers[providerId] = provider;
      }
      if (apiKey) provider.api_key = apiKey;
      if (baseURL) provider.base_url = baseURL;
      if (model) {
        provider.active_model = model;
        if (!provider.models.some((m) => m.id === model))
          provider.models.push({ id: model });
      }
    });
  }

  public getLLMConfig(): {
    activeProfile: {
      id: string;
      name: string;
      provider: string;
      apiKey: string;
      baseURL: string;
      activeModel: string;
      models: ModelMeta[];
      contextWindow?: number | undefined;
      maxTokens?: number | undefined;
      projectId?: string | undefined;
      accountEmail?: string | undefined;
    } | null;
    profiles: ProviderProfile[];
  } {
    const provider = this.store.defaultProvider();
    const allProfiles = this.getProviderProfiles();
    if (!provider) return { activeProfile: null, profiles: allProfiles };
    const meta = provider.models.find((m) => m.id === provider.active_model);
    return {
      activeProfile: {
        id: provider.id,
        name: provider.name,
        provider: provider.api,
        apiKey: provider.api_key,
        baseURL: provider.base_url,
        activeModel: provider.active_model,
        models: provider.models,
        ...(meta?.context_window !== undefined
          ? { contextWindow: meta.context_window }
          : {}),
        ...(meta?.max_tokens !== undefined
          ? { maxTokens: meta.max_tokens }
          : {}),
        ...((provider as any).projectId
          ? { projectId: (provider as any).projectId }
          : {}),
        ...((provider as any).accountEmail
          ? { accountEmail: (provider as any).accountEmail }
          : {}),
      },
      profiles: allProfiles,
    };
  }

  public listLLMProviders(): Array<{
    id: string;
    name: string;
    baseURL: string;
    models: string[];
  }> {
    return DEFAULT_PROVIDERS.map(({ id, name, baseURL, models }) => ({
      id,
      name,
      baseURL,
      models,
    }));
  }

  /** Fetches the model catalog from an OpenAI-compatible endpoint. */
  public async fetchProviderModels(
    apiKey: string,
    baseURL: string,
  ): Promise<string[]> {
    try {
      const url = `${baseURL.replace(/\/+$/, "")}/models`;
      const res = await fetch(url, {
        headers: apiKey ? { Authorization: `Bearer ${apiKey}` } : {},
      });
      if (!res.ok) return [];
      const data: unknown = await res.json();
      if (data && typeof data === "object" && "data" in data) {
        const rows = (data as { data?: unknown }).data;
        if (Array.isArray(rows)) {
          return rows
            .map((m) =>
              m && typeof m === "object" && "id" in m
                ? String((m as { id: unknown }).id)
                : "",
            )
            .filter(Boolean);
        }
      }
      return [];
    } catch {
      return [];
    }
  }
}
