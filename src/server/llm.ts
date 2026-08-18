import fs from "fs";
import path from "path";
import os from "os";

export interface ProviderProfile {
  id: string;
  name: string;
  provider: string;
  apiKey: string;
  baseURL: string;
  activeModel: string;
  models: string[];
  enabled: boolean;
}

export interface ProviderConfig {
  id: string;
  name: string;
  defaultBaseURL: string;
  models: string[];
  docURL?: string;
}

const DEFAULT_PROVIDERS: ProviderConfig[] = [
  {
    id: "openai",
    name: "OpenAI",
    defaultBaseURL: "https://api.openai.com/v1",
    models: ["gpt-4o", "gpt-4o-mini", "o1", "o3-mini"],
  },
  {
    id: "anthropic",
    name: "Anthropic",
    defaultBaseURL: "https://api.anthropic.com/v1",
    models: ["claude-3-7-sonnet-20250219", "claude-3-5-sonnet-20241022", "claude-3-5-haiku-20241022"],
  },
  {
    id: "openrouter",
    name: "OpenRouter",
    defaultBaseURL: "https://openrouter.ai/api/v1",
    models: ["anthropic/claude-3.7-sonnet", "openai/gpt-4o", "deepseek/deepseek-r1"],
  },
  {
    id: "ollama",
    name: "Ollama (Local)",
    defaultBaseURL: "http://127.0.0.1:11434/v1",
    models: ["llama3.3", "qwen2.5-coder", "deepseek-r1"],
  },
  {
    id: "deepseek",
    name: "DeepSeek",
    defaultBaseURL: "https://api.deepseek.com/v1",
    models: ["deepseek-chat", "deepseek-reasoner"],
  },
  {
    id: "groq",
    name: "Groq",
    defaultBaseURL: "https://api.groq.com/openai/v1",
    models: ["llama-3.3-70b-versatile", "mixtral-8x7b-32768"],
  },
];

export class LLMManager {
  private dataDir: string;
  private configFile: string;
  private profiles: ProviderProfile[] = [];
  private activeProfileId: string = "openai";

  constructor(dataDir?: string) {
    this.dataDir = dataDir || path.join(os.homedir(), ".forge-ade");
    this.configFile = path.join(this.dataDir, "llm_config.json");
    this.loadConfig();
  }

  private loadConfig(): void {
    try {
      if (fs.existsSync(this.configFile)) {
        const raw = fs.readFileSync(this.configFile, "utf-8");
        const parsed = JSON.parse(raw);
        this.profiles = parsed.profiles || [];
        this.activeProfileId = parsed.activeProfileId || "openai";
      } else {
        this.profiles = DEFAULT_PROVIDERS.map((p) => ({
          id: p.id,
          name: p.name,
          provider: p.id,
          apiKey: "",
          baseURL: p.defaultBaseURL,
          activeModel: p.models[0] || "",
          models: p.models,
          enabled: true,
        }));
      }
    } catch {
      this.profiles = [];
    }
  }

  private saveConfig(): void {
    try {
      fs.writeFileSync(
        this.configFile,
        JSON.stringify(
          {
            profiles: this.profiles,
            activeProfileId: this.activeProfileId,
          },
          null,
          2
        ),
        "utf-8"
      );
    } catch (err) {
      console.error("Failed to save LLM config:", err);
    }
  }

  public getProviderProfiles(): ProviderProfile[] {
    return [...this.profiles];
  }

  public saveProviderProfiles(profiles: ProviderProfile[]): void {
    this.profiles = profiles;
    this.saveConfig();
  }

  public async fetchProviderModels(apiKey: string, baseURL: string): Promise<string[]> {
    const url = `${baseURL.replace(/\/+$/, "")}/models`;
    try {
      const res = await fetch(url, {
        headers: {
          Authorization: `Bearer ${apiKey}`,
        },
      });
      if (res.ok) {
        const data = await res.json() as { data?: Array<{ id: string }> };
        if (Array.isArray(data.data)) {
          return data.data.map((m) => m.id);
        }
      }
    } catch (err) {
      console.warn("Failed to fetch models:", err);
    }
    return [];
  }

  public setActiveModel(providerId: string, model: string): void {
    const profile = this.profiles.find((p) => p.id === providerId);
    if (profile) {
      profile.activeModel = model;
      this.activeProfileId = providerId;
      this.saveConfig();
    }
  }

  public saveLLMProfile(providerId: string, apiKey: string, baseURL: string, model: string): void {
    let profile = this.profiles.find((p) => p.id === providerId);
    if (profile) {
      profile.apiKey = apiKey;
      profile.baseURL = baseURL;
      profile.activeModel = model;
    } else {
      profile = {
        id: providerId,
        name: providerId,
        provider: providerId,
        apiKey,
        baseURL,
        activeModel: model,
        models: [model],
        enabled: true,
      };
      this.profiles.push(profile);
    }
    this.saveConfig();
  }

  public getLLMConfig(): any {
    const active = this.profiles.find((p) => p.id === this.activeProfileId) || this.profiles[0] || null;
    return {
      activeProfile: active,
      profiles: this.profiles,
    };
  }

  public listLLMProviders(): ProviderConfig[] {
    return DEFAULT_PROVIDERS;
  }
}
