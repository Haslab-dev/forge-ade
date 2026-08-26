import React, { useState, useEffect, useCallback } from "react";
import { useShortcutsStore, useUIStore } from "../hooks/store";
import { useToast } from "../lib/toast";
import { APP_VERSION } from "../lib/utils";
import {
  IconX,
  IconSettings,
  IconKeyboard,
  IconPalette,
  IconCode,
  IconPlus,
  IconTrash,
  IconRefresh,
  IconRobot,
  IconPlug,
  IconCpu,
  IconCheck,
  IconChevronDown,
  IconChevronRight,
  IconSparkles,
  IconSearch,
  IconTools,
  IconChartBar,
  IconActivity,
  IconKey,
  IconExternalLink,
  IconBrandGoogle,
} from "@tabler/icons-react";
import {
  GetProviderProfiles,
  SaveProviderProfiles,
  FetchProviderModels,
  SetActiveModel,
  SaveLLMProfile,
  ListAgentDefinitions,
  SaveAgentDefinition,
  DeleteAgentDefinition,
  ListMCPServers,
  SaveMCPServer,
  DeleteMCPServer,
  ListConnectedMCPTools,
  ReconnectMCP,
  RefreshMCP,
  RefreshSkills,
  ListLLMProviders,
  ListAllSkills,
  SetSkillEnabled,
  SetAllSkillsEnabled,
  StartOAuthLogin,
  GetOAuthStatus,
  SubmitOAuthManualCode,
  BrowserOpenURL,
  GetUsageSummary,
  GetProviderQuota,
  GetAllProviderQuotas,
  type SkillInfo,
  type MCPServerInfo,
  type MCPToolInfo,
  type UsageSummary,
  type ProviderQuotaReport,
} from "../lib/native";
import { loadEditorSettings, saveEditorSettings, DEFAULT_EDITOR_SETTINGS, type EditorSettings } from "../lib/editor-settings";

interface GlobalSettingsModalProps {
  open: boolean;
  onClose: () => void;
}

type Tab = "shortcuts" | "appearance" | "editor" | "providers" | "agents" | "skills" | "mcp" | "ai-commit" | "usage";

const DEFAULT_ROLES = ["coding", "planning", "research", "custom"];

// Theme palette options — each key matches a theme class in index.css.
const THEME_OPTIONS: { value: string; label: string }[] = [
  { value: "dark-plus", label: "Dark Plus — Blue" },
  { value: "midnight", label: "Midnight — Neutral Blue" },
  { value: "cursor", label: "Cursor — Cyan" },
  { value: "catppuccin", label: "Catppuccin — Mocha" },
  { value: "dracula", label: "Dracula" },
  { value: "nord", label: "Nord" },
  { value: "gruvbox", label: "Gruvbox" },
  { value: "tokyonight", label: "Tokyo Night" },
  { value: "ayu", label: "Ayu" },
  { value: "one-dark", label: "One Dark" },
  { value: "github", label: "GitHub Dark" },
  { value: "light", label: "Light" },
];


export interface ProviderPreset {
  id: string;
  name: string;
  authType: "oauth" | "api_key" | "device";
  baseUrl?: string;
  defaultModels: string[];
  keyUrl?: string;
  description: string;
}

export const PROVIDER_PRESETS: ProviderPreset[] = [
  {
    id: "google-antigravity",
    name: "Google Antigravity (Gemini 3, Claude, GPT-OSS)",
    authType: "oauth",
    baseUrl: "https://daily-cloudcode-pa.googleapis.com",
    defaultModels: ["gemini-3.7-flash-tiered", "gemini-2.5-pro", "gemini-2.5-flash", "claude-3-7-sonnet", "claude-3-5-sonnet"],
    description: "Access Gemini 3 & Claude models with Google Cloud Code Assist OAuth",
  },
  {
    id: "openrouter",
    name: "OpenRouter",
    authType: "api_key",
    baseUrl: "https://openrouter.ai/api/v1",
    keyUrl: "https://openrouter.ai/keys",
    defaultModels: ["anthropic/claude-3.7-sonnet", "openai/gpt-4o", "deepseek/deepseek-r1", "meta-llama/llama-3.3-70b-instruct"],
    description: "Access 200+ models with unified API keys",
  },
  {
    id: "opencode-go",
    name: "OpenCode Go",
    authType: "api_key",
    baseUrl: "https://api.opencode.ai/v1",
    keyUrl: "https://opencode.ai/account",
    defaultModels: ["claude-3-7-sonnet", "claude-3-5-sonnet", "gemini-2.5-pro", "gpt-4o"],
    description: "High-throughput OpenCode coding endpoint",
  },
  {
    id: "opencode-zen",
    name: "OpenCode Zen",
    authType: "api_key",
    baseUrl: "https://zen.opencode.ai/v1",
    keyUrl: "https://opencode.ai/zen",
    defaultModels: ["zen-r1", "zen-coder"],
    description: "OpenCode Zen reasoning models",
  },
  {
    id: "kilo",
    name: "KiloCode",
    authType: "device",
    baseUrl: "https://api.kilo.ai/v1",
    keyUrl: "https://kilo.ai/keys",
    defaultModels: ["kilo-coder", "claude-3.7-sonnet", "deepseek-r1"],
    description: "KiloCode AI device login & API key",
  },
  {
    id: "vercel-ai-gateway",
    name: "Vercel AI Gateway",
    authType: "api_key",
    baseUrl: "https://api.vercel.ai/v1",
    keyUrl: "https://vercel.com/docs/ai/ai-gateway",
    defaultModels: ["anthropic/claude-3.7-sonnet", "openai/gpt-4o"],
    description: "Edge AI gateway proxy",
  },
  {
    id: "openai",
    name: "OpenAI",
    authType: "api_key",
    baseUrl: "https://api.openai.com/v1",
    keyUrl: "https://platform.openai.com/api-keys",
    defaultModels: ["gpt-4o", "gpt-4o-mini", "o1", "o3-mini"],
    description: "Direct OpenAI API",
  },
  {
    id: "anthropic",
    name: "Anthropic",
    authType: "api_key",
    baseUrl: "https://api.anthropic.com/v1",
    keyUrl: "https://console.anthropic.com/settings/keys",
    defaultModels: ["claude-3-7-sonnet-20250219", "claude-3-5-sonnet-20241022", "claude-3-5-haiku-20241022"],
    description: "Direct Anthropic Claude API",
  },
  {
    id: "custom",
    name: "Custom (OpenAI-compatible / Ollama)",
    authType: "api_key",
    baseUrl: "http://localhost:11434/v1",
    defaultModels: ["default-model"],
    description: "Self-hosted Ollama, vLLM, LiteLLM, or LM Studio",
  },
];

export function GlobalSettingsModal({ open, onClose }: GlobalSettingsModalProps) {
  const { keybindings, setKeybindings } = useShortcutsStore();
  const { theme, setTheme } = useUIStore();
  const [editorSettings, setEditorSettings] = useState<EditorSettings>(DEFAULT_EDITOR_SETTINGS);
  useEffect(() => {
    if (open) setEditorSettings(loadEditorSettings());
  }, [open]);
  const updateEditorSetting = <K extends keyof EditorSettings>(key: K, value: EditorSettings[K]) => {
    setEditorSettings((prev) => {
      const next = { ...prev, [key]: value };
      saveEditorSettings(next);
      return next;
    });
  };
  const { toast } = useToast();
  const [activeTab, setActiveTab] = useState<Tab>("shortcuts");
  const [editingId, setEditingId] = useState<string | null>(null);
  const [recordingKeys, setRecordingKeys] = useState<string[]>([]);

  // Skills state
  const [skills, setSkills] = useState<SkillInfo[]>([]);
  const [refreshingSkills, setRefreshingSkills] = useState(false);
  const [skillsSearch, setSkillsSearch] = useState("");

  const loadSkills = useCallback(async () => {
    try {
      const list = await ListAllSkills();
      setSkills(Array.isArray(list) ? list : []);
    } catch { /* ignore */ }
  }, []);

  // Providers state
  const [profiles, setProfiles] = useState<any[]>([]);
  const [selectedPresetId, setSelectedPresetId] = useState("google-antigravity");
  const [oauthFlow, setOauthFlow] = useState<{
    active: boolean;
    provider?: string;
    loginId?: string;
    authUrl?: string;
    userCode?: string;
    instructions?: string;
    status: "idle" | "waiting" | "polling" | "success" | "error";
    error?: string;
  }>({ active: false, status: "idle" });

  // Usage state
  const [usageSummary, setUsageSummary] = useState<UsageSummary | null>(null);
  const [providerQuota, setProviderQuota] = useState<ProviderQuotaReport | null>(null);
  const [allProviderQuotas, setAllProviderQuotas] = useState<ProviderQuotaReport[]>([]);
  const [loadingUsage, setLoadingUsage] = useState(false);
  const [manualOAuthCode, setManualOAuthCode] = useState("");
  const [submittingManualCode, setSubmittingManualCode] = useState(false);
  const [fetchedModelSearch, setFetchedModelSearch] = useState("");
  const [expandedModels, setExpandedModels] = useState<Record<string, boolean>>({});
  const [profileModelSearch, setProfileModelSearch] = useState<Record<string, string>>({});
  const [refreshingModels, setRefreshingModels] = useState<Record<string, boolean>>({});
  const [newProvider, setNewProvider] = useState({ name: "", apiKey: "", baseUrl: "" });
  const [fetchingModels, setFetchingModels] = useState(false);
  const [fetchedModels, setFetchedModels] = useState<string[]>([]);
  const [defaultModels, setDefaultModels] = useState<string[]>([]);

  // Agents state
  const [agentDefs, setAgentDefs] = useState<any[]>([]);
  const [showAgentForm, setShowAgentForm] = useState(false);
  const [agentForm, setAgentForm] = useState({
    name: "",
    description: "",
    role: "coding",
    model: "",
    prompt: "",
    rules: "",
  });

  // MCP state
  const [mcpServers, setMcpServers] = useState<MCPServerInfo[]>([]);
  const [mcpTools, setMcpTools] = useState<MCPToolInfo[]>([]);
  const [expandedMcp, setExpandedMcp] = useState<Record<string, boolean>>({});
  const [showMcpForm, setShowMcpForm] = useState(false);
  const [mcpForm, setMcpForm] = useState<{
    name: string;
    type: "stdio" | "remote";
    command: string;
    args: string;
    url: string;
    env: string;
    headers: string;
  }>({ name: "", type: "stdio", command: "", args: "", url: "", env: "", headers: "" });
  const [mcpReconnecting, setMcpReconnecting] = useState(false);

  // AI Commit generator config (provider + model + prompt), persisted in localStorage
  const DEFAULT_COMMIT_PROMPT =
    "CRITICAL: You are an expert software developer writing a declarative Git commit message adhering strictly to Conventional Commits (e.g. 'feat(settings): add MCP detail configuration and live controls').\\n\\n" +
    "Format rules:\\n" +
    "- Output ONLY the commit message itself. Do NOT include markdown fences, backticks, quotes, explanations, or diffstat numbers (e.g. NEVER include '(file.ts | 100 +-)').\\n" +
    "- First line MUST be: <type>(<scope>): <subject in imperative present tense, lowercase, <=72 chars>.\\n" +
    "- Valid types: feat, fix, refactor, perf, docs, style, test, chore, build, ci.\\n" +
    "- If changes are non-trivial, add an empty line followed by 2-3 concise bullet points with '-' explaining WHAT and WHY.\\n" +
    "- Focus on the declarative semantic intent of the changes.";
  const [commitProvider, setCommitProvider] = useState("");
  const [commitModel, setCommitModel] = useState("");
  const [commitPrompt, setCommitPrompt] = useState(DEFAULT_COMMIT_PROMPT);

  const loadCommitConfig = useCallback(() => {
    try {
      const raw = localStorage.getItem("forge-ade-ai-commit-config");
      if (raw) {
        const cfg = JSON.parse(raw);
        if (cfg.provider) setCommitProvider(cfg.provider);
        if (cfg.model) setCommitModel(cfg.model);
        if (cfg.prompt) setCommitPrompt(cfg.prompt);
      }
    } catch { /* ignore */ }
  }, []);

  const saveCommitConfig = useCallback((provider: string, model: string, prompt: string) => {
    try {
      localStorage.setItem("forge-ade-ai-commit-config", JSON.stringify({ provider, model, prompt }));
    } catch { /* ignore */ }
  }, []);

  // Expanded provider rows
  const [expandedProviders, setExpandedProviders] = useState<Record<string, boolean>>({});

  const loadUsageData = useCallback(async () => {
    setLoadingUsage(true);
    try {
      const [sum, quota, allQuotas] = await Promise.all([
        GetUsageSummary(),
        GetProviderQuota(),
        GetAllProviderQuotas(),
      ]);
      setUsageSummary(sum);
      setProviderQuota(quota);
      setAllProviderQuotas(Array.isArray(allQuotas) ? allQuotas : []);
    } catch (e) {
      console.warn("Failed to load usage data:", e);
    } finally {
      setLoadingUsage(false);
    }
  }, []);

  function formatTimeRemaining(targetIso?: string): string {
    if (!targetIso) return "";
    const targetMs = new Date(targetIso).getTime();
    if (isNaN(targetMs)) return "";
    const diffMs = targetMs - Date.now();
    if (diffMs <= 0) return "(ready)";
    const totalMins = Math.round(diffMs / 60000);
    const hours = Math.floor(totalMins / 60);
    const mins = totalMins % 60;
    if (hours > 0) return `(${hours}h${mins > 0 ? `${mins}m` : ""})`;
    return `(${mins}m)`;
  }

  const handleManualCodeSubmit = async () => {
    if (!manualOAuthCode.trim() || !oauthFlow.loginId) return;
    setSubmittingManualCode(true);
    try {
      const res = await SubmitOAuthManualCode(oauthFlow.loginId, manualOAuthCode.trim());
      if (res.status === "success") {
        toast(`Successfully authenticated with ${oauthFlow.provider || "Google"}!`, "success");
        await loadProfiles();
        setManualOAuthCode("");
        setOauthFlow({ active: false, status: "idle" });
      } else {
        toast(`Authentication failed: ${res.error || "unknown"}`, "danger");
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      toast("Failed to submit code: " + msg, "danger");
    } finally {
      setSubmittingManualCode(false);
    }
  };

  const handleStartOAuth = async (providerId: string) => {
    try {
      const res = await StartOAuthLogin(providerId);
      setOauthFlow({
        active: true,
        provider: providerId,
        loginId: res.loginId,
        authUrl: res.authUrl,
        userCode: res.userCode,
        instructions: res.instructions,
        status: res.method === "device" ? "polling" : "waiting",
      });

      if (res.authUrl) {
        BrowserOpenURL(res.authUrl);
      }

      // Start polling status
      const pollTimer = setInterval(async () => {
        try {
          const st = await GetOAuthStatus(res.loginId);
          if (st?.status === "success") {
            clearInterval(pollTimer);
            setOauthFlow((prev) => ({ ...prev, status: "success" }));
            toast(`Successfully authenticated with ${providerId}!`, "success");
            await loadProfiles();
            setTimeout(() => setOauthFlow({ active: false, status: "idle" }), 1500);
          } else if (st?.status === "error" || st?.status === "cancelled") {
            clearInterval(pollTimer);
            setOauthFlow((prev) => ({ ...prev, status: "error", error: st.error || "Login cancelled" }));
            toast(`Authentication failed: ${st.error || "cancelled"}`, "danger");
          }
        } catch {}
      }, 1500);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      toast("Failed to start OAuth login: " + msg, "danger");
    }
  };

  const loadProfiles = useCallback(async () => {
    try {
      const list = await GetProviderProfiles();
      setProfiles(Array.isArray(list) ? list : []);
      const providers = await ListLLMProviders();
      if (Array.isArray(providers)) {
        const models = providers.flatMap((p: any) => (p.default_model ? [p.default_model] : []));
        setDefaultModels(models);
      }
    } catch { /* ignore */ }
  }, []);

  const loadAgentDefs = useCallback(async () => {
    try {
      const list = await ListAgentDefinitions();
      setAgentDefs(Array.isArray(list) ? list : []);
    } catch { /* ignore */ }
  }, []);

  const loadMcpServers = useCallback(async () => {
    try {
      const [servers, tools] = await Promise.all([
        ListMCPServers(),
        ListConnectedMCPTools(),
      ]);
      setMcpServers(Array.isArray(servers) ? servers : []);
      setMcpTools(Array.isArray(tools) ? tools : []);
    } catch { /* ignore */ }
  }, []);

  useEffect(() => {
    if (!open) return;
    loadProfiles();
    loadAgentDefs();
    loadSkills();
    loadMcpServers();
    loadCommitConfig();
  }, [open, loadProfiles, loadAgentDefs, loadSkills, loadMcpServers, loadCommitConfig]);

  useEffect(() => {
    if (!editingId) return;

    const handleKeyDown = (e: KeyboardEvent) => {
      e.preventDefault();
      e.stopPropagation();

      const parts: string[] = [];
      if (e.metaKey) parts.push("meta");
      if (e.ctrlKey) parts.push("ctrl");
      if (e.altKey) parts.push("alt");
      if (e.shiftKey) parts.push("shift");

      let key = e.key.toLowerCase();
      if (key === " ") key = "space";
      if (!["control", "shift", "alt", "meta"].includes(key)) {
        parts.push(key);
      }

      const combo = parts.join("+");
      // Don't let users bind the OS's reserved editing shortcuts — they
      // must keep working in inputs/editor/terminal.
      const reserved = ["meta+v", "ctrl+v", "meta+c", "ctrl+c", "meta+x", "ctrl+x", "meta+z", "ctrl+z", "meta+shift+z", "ctrl+shift+z", "meta+y", "ctrl+y", "meta+a", "ctrl+a"];
      if (reserved.includes(combo)) {
        setRecordingKeys([]);
        return;
      }

      setRecordingKeys(parts);
    };

    const handleKeyUp = () => {
      if (recordingKeys.length > 0) {
        const combo = recordingKeys.join("+");
        setKeybindings(
          keybindings.map((kb) => (kb.id === editingId ? { ...kb, key: combo } : kb))
        );
        setEditingId(null);
        setRecordingKeys([]);
      }
    };

    window.addEventListener("keydown", handleKeyDown, true);
    window.addEventListener("keyup", handleKeyUp, true);

    return () => {
      window.removeEventListener("keydown", handleKeyDown, true);
      window.removeEventListener("keyup", handleKeyUp, true);
    };
  }, [editingId, recordingKeys, keybindings]);

  async function handleAddProvider() {
    const name = newProvider.name.trim();
    if (!name) return;
    const manual = (newProvider as any)._selectedModels;
    // If the user didn't pick specific models, default to all fetched models;
    // if no models were fetched, fall back to the built-in default models.
    const available = fetchedModels.length > 0 ? fetchedModels : defaultModels;
    const selected = manual && manual.length > 0 ? manual : available;
    const prof = {
      id: name.toLowerCase().replace(/[^a-z0-9]+/g, "-"),
      name,
      api_key: newProvider.apiKey.trim(),
      base_url: newProvider.baseUrl.trim() || "https://api.openai.com/v1",
      enabled: true,
      available_models: available,
      selected_models: selected,
    };
    const next = [...profiles, prof];
    setProfiles(next);
    try {
      await SaveProviderProfiles(next);
      toast("Provider added", "success");
    } catch (err: any) {
      toast("Failed to add provider: " + err, "danger");
    }
    setNewProvider({ name: "", apiKey: "", baseUrl: "" });
    setFetchedModels([]);
  }

  async function handleFetchModels() {
    setFetchingModels(true);
    try {
      const models = await FetchProviderModels(newProvider.apiKey.trim(), newProvider.baseUrl.trim() || "https://api.openai.com/v1");
      setFetchedModels(Array.isArray(models) ? models : []);
      toast(`Models fetched: ${(Array.isArray(models) ? models : []).length}`, "success");
    } catch (err: any) {
      toast("Fetch models failed: " + err, "danger");
    } finally {
      setFetchingModels(false);
    }
  }

  async function handleRefreshProviderModels(providerId: string) {
    const p = profiles.find((x) => (x.id || x.Id) === providerId);
    if (!p) return;
    setRefreshingModels((prev) => ({ ...prev, [providerId]: true }));
    try {
      let fetched: string[] = [];
      const isAntigravity = p.id === "google-antigravity" || p.provider === "google-antigravity" || p.id?.startsWith("google-antigravity");

      if (isAntigravity) {
        const quota = await GetProviderQuota(providerId);
        if (quota?.models && quota.models.length > 0) {
          fetched = quota.models.map((m) => m.model);
        }
      }

      if (fetched.length === 0) {
        const modelsRes = await FetchProviderModels(
          p.apiKey || p.api_key || p.ApiKey || "",
          p.baseURL || p.base_url || p.BaseURL || "https://api.openai.com/v1"
        );
        if (Array.isArray(modelsRes)) fetched = modelsRes;
      }

      if (fetched.length === 0) {
        toast("No models returned by provider endpoint", "warn");
        return;
      }

      const existingModels = toPlainStringArray(p.models);
      const combined = [...new Set([...fetched, ...existingModels])];
      const existingSelected = Array.isArray(p.selected_models) ? toPlainStringArray(p.selected_models) : existingModels;
      const nextSelected = [...new Set([...existingSelected, ...fetched])];

      const next = profiles.map((x) => {
        if ((x.id || x.Id) !== providerId) return x;
        return {
          ...x,
          models: combined.map((id) => ({ id, name: id })),
          selected_models: nextSelected,
        };
      });

      setProfiles(next);
      await SaveProviderProfiles(next);
      toast(`Fetched ${fetched.length} models for ${p.name || providerId}`, "success");
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      toast("Fetch models failed: " + msg, "danger");
    } finally {
      setRefreshingModels((prev) => ({ ...prev, [providerId]: false }));
    }
  }

  async function handleToggleModel(provider: any, model: string) {
    const pid = provider.id || provider.Id;
    const allModels = toPlainStringArray(provider.models);
    const selected = provider.selected_models && Array.isArray(provider.selected_models)
      ? toPlainStringArray(provider.selected_models)
      : allModels;
    const has = selected.includes(model);
    const nextSelected = has ? selected.filter((m: string) => m !== model) : [...selected, model];
    const next = profiles.map((p) => {
      const id = p.id || p.Id;
      if (id !== pid) return p;
      return { ...p, selected_models: nextSelected };
    });
    setProfiles(next);
    try {
      await SaveProviderProfiles(next);
    } catch { /* ignore */ }
  }

  async function handleSelectAllProfileModels(provider: any, selectAll: boolean) {
    const pid = provider.id || provider.Id;
    const allModels = toPlainStringArray(provider.models);
    const nextSelected = selectAll ? allModels : [];
    const next = profiles.map((p) => {
      const id = p.id || p.Id;
      if (id !== pid) return p;
      return { ...p, selected_models: nextSelected };
    });
    setProfiles(next);
    try {
      await SaveProviderProfiles(next);
    } catch { /* ignore */ }
  }

  function toPlainStringArray(arr: any): string[] {
    if (!Array.isArray(arr)) return [];
    return arr.map((x) => (typeof x === "string" ? x : x?.id || String(x))).filter(Boolean);
  }

  async function handleToggleProvider(id: string) {
    const next = profiles.map((p) => {
      const pId = p.id || p.Id;
      if (pId !== id) return p;
      return { ...p, enabled: p.enabled === false ? true : false };
    });
    setProfiles(next);
    try {
      await SaveProviderProfiles(next);
      toast("Provider updated", "success");
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      toast("Failed to update provider: " + msg, "danger");
    }
  }

  async function handleDeleteProvider(id: string) {
    const next = profiles.filter((p) => (p.id || p.Id) !== id);
    setProfiles(next);
    try {
      await SaveProviderProfiles(next);
      toast("Provider removed", "success");
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      toast("Failed to remove provider: " + msg, "danger");
    }
  }

  async function handleSetActiveModel(providerId: string, model: string) {
    try {
      await SetActiveModel(providerId, model);
      const next = profiles.map((p) => {
        if ((p.id || p.Id) === providerId) return { ...p, activeModel: model };
        return p;
      });
      setProfiles(next);
      toast(`Active model set to ${model}`, "success");
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      toast("Failed to set model: " + msg, "danger");
    }
  }

  async function handleSaveNewProvider() {
    const name = newProvider.name.trim();
    if (!name) return;
    const pId = selectedPresetId === "custom" ? name.toLowerCase().replace(/[^a-z0-9]+/g, "-") : selectedPresetId;
    const available = fetchedModels.length > 0 ? fetchedModels : defaultModels;
    const selected = defaultModels.length > 0 ? defaultModels : available;
    const prof = {
      id: pId,
      name,
      apiKey: newProvider.apiKey.trim(),
      api_key: newProvider.apiKey.trim(),
      baseURL: newProvider.baseUrl.trim() || "https://api.openai.com/v1",
      base_url: newProvider.baseUrl.trim() || "https://api.openai.com/v1",
      activeModel: selected[0] || "default-model",
      active_model: selected[0] || "default-model",
      models: available.map((id) => ({ id, name: id })),
      selected_models: selected,
      enabled: true,
    };
    const next = [...profiles.filter((p) => (p.id || p.Id) !== pId), prof];
    setProfiles(next);
    try {
      await SaveProviderProfiles(next);
      toast("Provider saved", "success");
      setNewProvider({ name: "", apiKey: "", baseUrl: "" });
      setDefaultModels([]);
      setFetchedModels([]);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      toast("Failed to save provider: " + msg, "danger");
    }
  }

  async function handleSaveLLMProfile(provider: any) {
    const pid = provider.id || provider.Id;
    const model = provider.activeModel || (provider.selected_models || provider.SelectedModels || [])[0] || (provider.available_models || [])[0] || "";
    try {
      await SaveLLMProfile(pid, provider.api_key || provider.ApiKey || "", provider.base_url || provider.BaseURL || "", model || "");
      if (model) await SetActiveModel(pid, model);
      toast(`Activated ${provider.name || pid} (${model || "default"})`, "success");
    } catch (err: any) {
      toast("Failed to activate provider: " + err, "danger");
    }
  }
  async function handleSaveAgentDef() {
    if (!agentForm.name.trim()) return;
    const def = {
      id: agentForm.name.toLowerCase().replace(/[^a-z0-9]+/g, "-"),
      name: agentForm.name.trim(),
      description: agentForm.description.trim(),
      role_filter: agentForm.role,
      model: agentForm.model.trim(),
      prompt: agentForm.prompt,
      rules: agentForm.rules,
    };
    try {
      await SaveAgentDefinition(def);
      await loadAgentDefs();
      setShowAgentForm(false);
      setAgentForm({ name: "", description: "", role: "coding", model: "", prompt: "", rules: "" });
      toast("Agent saved", "success");
    } catch (err: any) {
      toast("Failed to save agent: " + err, "danger");
    }
  }

  async function handleDeleteAgentDef(id: string) {
    try {
      await DeleteAgentDefinition(id);
      await loadAgentDefs();
      toast("Agent deleted", "success");
    } catch (err: any) {
      toast("Failed to delete agent: " + err, "danger");
    }
  }

  async function handleSaveMcpServer() {
    const name = mcpForm.name.trim();
    if (!name) return;
    let envObj: Record<string, string> | undefined;
    if (mcpForm.env.trim()) {
      try {
        const parsed = JSON.parse(mcpForm.env.trim());
        if (typeof parsed === "object" && parsed !== null) envObj = parsed;
      } catch {
        toast("Environment must be valid JSON", "danger");
        return;
      }
    }
    let headersObj: Record<string, string> | undefined;
    if (mcpForm.headers.trim()) {
      try {
        const parsed = JSON.parse(mcpForm.headers.trim());
        if (typeof parsed === "object" && parsed !== null) headersObj = parsed;
      } catch {
        toast("Headers must be valid JSON", "danger");
        return;
      }
    }
    const server: Partial<MCPServerInfo> = {
      name,
      enabled: true,
      ...(mcpForm.type === "remote"
        ? { url: mcpForm.url.trim(), headers: headersObj }
        : {
            command: mcpForm.command.trim(),
            args: mcpForm.args.trim() ? mcpForm.args.trim().split(/\s+/) : [],
            env: envObj,
          }),
    };
    try {
      await SaveMCPServer(server);
      await loadMcpServers();
      setShowMcpForm(false);
      setMcpForm({ name: "", type: "stdio", command: "", args: "", url: "", env: "", headers: "" });
      toast("MCP server saved", "success");
    } catch (err: any) {
      toast("Failed to save MCP server: " + err, "danger");
    }
  }

  async function handleReconnectMcp() {
    setMcpReconnecting(true);
    try {
      const res = await ReconnectMCP();
      await loadMcpServers();
      toast(`Reconnected: ${res.connected.length} active, ${res.failed.length} failed`, "info");
    } catch (err: any) {
      toast("Reconnect failed: " + err, "danger");
    } finally {
      setMcpReconnecting(false);
    }
  }

  async function handleRefreshSkills() {
    setRefreshingSkills(true);
    try {
      const list = await RefreshSkills();
      await loadSkills();
      toast(`Discovered ${list.length} skills`, "success");
    } catch (err: unknown) {
      toast("Skill refresh failed: " + (err instanceof Error ? err.message : String(err)), "danger");
    } finally {
      setRefreshingSkills(false);
    }
  }

  async function handleRefreshMcp() {
    try {
      const servers = await RefreshMCP();
      await loadMcpServers();
      toast(`Discovered ${servers.length} MCP servers`, "info");
    } catch (err: unknown) {
      toast("MCP refresh failed: " + (err instanceof Error ? err.message : String(err)), "danger");
    }
  }

  async function handleDeleteMcpServer(name: string) {
    try {
      await DeleteMCPServer(name);
      await loadMcpServers();
      toast("MCP server deleted", "success");
    } catch (err: any) {
      toast("Failed to delete MCP server: " + err, "danger");
    }
  }

  if (!open) return null;

  const tabBtn = (t: Tab, icon: React.ReactNode, label: string) => (
    <button
      onClick={() => setActiveTab(t)}
      className={`h-6 gap-1.5 px-2.5 text-[11.5px] flex items-center transition-colors cursor-pointer ${
        activeTab === t
          ? "bg-[var(--bg-app)] text-[var(--fg-primary)] font-medium"
          : "text-[var(--fg-muted)] hover:text-[var(--fg-primary)]"
      }`}
    >
      {icon}
      <span>{label}</span>
    </button>
  );

  return (
    <div className="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex items-center justify-center p-4">
      <div className="bg-[var(--bg-sidebar)] border border-[var(--border-default)] w-full max-w-4xl shadow-2xl flex flex-col h-[680px] max-h-[90vh] rounded-lg overflow-hidden">
        {/* Header — slim: title + close */}
        <header className="flex h-11 shrink-0 items-center gap-2 border-b border-[var(--border-default)] px-4">
          <div className="flex min-w-0 flex-1 items-center gap-2">
            <IconSettings className="size-3.5 text-[var(--accent-primary)]" />
            <span className="text-[12px] font-medium text-[var(--fg-primary)]">Settings</span>
            <span className="rounded border border-[var(--border-default)] px-1.5 py-px text-[10px] font-mono text-[var(--fg-muted)]">v{APP_VERSION}</span>
          </div>
          <button
            onClick={onClose}
            className="flex h-6 w-6 items-center justify-center text-[var(--fg-muted)] hover:bg-[var(--bg-surface)] hover:text-[var(--fg-primary)] cursor-pointer"
            aria-label="Close settings"
          >
            <IconX className="size-3.5" />
          </button>
        </header>

        {/* Centered pill tab bar */}
        <div className="flex shrink-0 items-center gap-1 overflow-x-auto px-4 pt-3 pb-1">
          <div className="bg-[var(--bg-panel)] flex h-7 items-center gap-0.5 p-0.5">
            {tabBtn("shortcuts", <IconKeyboard className="size-3" />, "Shortcuts")}
            {tabBtn("appearance", <IconPalette className="size-3" />, "Appearance")}
            {tabBtn("editor", <IconCode className="size-3" />, "Editor")}
            {tabBtn("providers", <IconCpu className="size-3" />, "Providers")}
            {tabBtn("agents", <IconRobot className="size-3" />, "Agents")}
            {tabBtn("skills", <IconSparkles className="size-3 text-amber-400" />, "Skills")}
            {tabBtn("mcp", <IconPlug className="size-3" />, "MCP")}
            {tabBtn("ai-commit", <IconSparkles className="size-3" />, "AI Commit")}
            {tabBtn("usage", <IconChartBar className="size-3 text-emerald-400" />, "Usage")}
          </div>
        </div>

        {/* Tab Content */}
        <div className="flex-1 overflow-y-auto py-3 px-8 pt-4 pb-7">
          <div className="mx-auto w-full max-w-3xl space-y-3">
          {activeTab === "shortcuts" ? (
            <div className="space-y-3">
              <div className="text-[10px] text-[var(--fg-tertiary)] uppercase font-semibold tracking-wider">
                Click shortcut to remap keybinding
              </div>
              <div className="space-y-1">
                {keybindings.map((kb) => (
                  <div
                    key={kb.id}
                    onClick={() => {
                      setEditingId(kb.id);
                      setRecordingKeys([]);
                    }}
                    className={`flex items-center justify-between p-2 rounded cursor-pointer transition-colors ${
                      editingId === kb.id ? "bg-[var(--bg-surface-active)]" : "hover:bg-[var(--bg-panel)]"
                    }`}
                  >
                    <span className="text-xs text-[var(--fg-primary)] font-medium">{kb.name}</span>
                    <span className="text-xs font-mono px-2 py-0.5 bg-[var(--bg-surface)] border border-[var(--border-default)] text-[var(--fg-primary)] rounded">
                      {editingId === kb.id
                        ? recordingKeys.length > 0
                          ? recordingKeys.join("+")
                          : "Press keys..."
                        : kb.key}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          ) : activeTab === "appearance" ? (
            <div className="space-y-3 text-xs">
              <label className="text-[var(--fg-secondary)] block font-semibold">Theme Palette</label>
              <select
                value={theme}
                onChange={(e) => setTheme(e.target.value)}
                className="w-full bg-[var(--bg-panel)] border border-[var(--border-default)] px-3 py-1.5 text-[var(--fg-primary)] focus:outline-none focus:border-[var(--accent-primary)]"
              >
                {THEME_OPTIONS.map((t) => (
                  <option key={t.value} value={t.value}>{t.label}</option>
                ))}
              </select>
              <div className="flex flex-wrap gap-1.5 pt-1">
                {THEME_OPTIONS.map((t) => (
                  <button
                    key={t.value}
                    onClick={() => setTheme(t.value)}
                    className={`px-2 py-1 border text-[10px] transition-colors cursor-pointer ${
                      theme === t.value
                        ? "border-[var(--accent-primary)] bg-[var(--accent-primary)]/10 text-[var(--fg-primary)] font-semibold"
                        : "border-[var(--border-default)] text-[var(--fg-secondary)] hover:border-[var(--fg-tertiary)]"
                    }`}
                  >
                    {t.label}
                  </button>
                ))}
              </div>
            </div>
          ) : activeTab === "editor" ? (
            <div className="space-y-3 text-xs">
              <div>
                <span className="text-[var(--fg-secondary)] font-semibold">Editor</span>
                <span className="text-[10px] text-[var(--fg-tertiary)] block mt-0.5">
                  Formatting uses the project's own prettier when installed, reading its
                  .prettierrc / prettier.config.* / package.json "prettier" and .editorconfig.
                </span>
              </div>

              <label className="flex items-center gap-2 cursor-pointer select-none w-fit">
                <input
                  type="checkbox"
                  checked={editorSettings.formatOnSave}
                  onChange={(e) => updateEditorSetting("formatOnSave", e.target.checked)}
                  className="size-3.5 accent-[var(--accent-primary)] cursor-pointer"
                />
                <span className="text-[var(--fg-secondary)]">Format on save (ts, js, tsx, jsx, json, css, html, md)</span>
              </label>

              <div className="grid grid-cols-2 gap-3 pt-1">
                <label className="block">
                  <span className="text-[10px] text-[var(--fg-tertiary)] uppercase tracking-wider">Indent width</span>
                  <select
                    value={String(editorSettings.tabWidth)}
                    onChange={(e) =>
                      updateEditorSetting("tabWidth", e.target.value === "auto" ? "auto" : Number(e.target.value))
                    }
                    className="w-full mt-1 bg-[var(--bg-panel)] border border-[var(--border-default)] px-2 py-1 text-[11px] text-[var(--fg-primary)] focus:outline-none focus:border-[var(--accent-primary)]"
                  >
                    <option value="auto">Auto (project config)</option>
                    <option value="1">1</option>
                    <option value="2">2</option>
                    <option value="4">4</option>
                    <option value="8">8</option>
                  </select>
                </label>
                <label className="block">
                  <span className="text-[10px] text-[var(--fg-tertiary)] uppercase tracking-wider">Indent style</span>
                  <select
                    value={editorSettings.useTabs === "auto" ? "auto" : editorSettings.useTabs ? "tabs" : "spaces"}
                    onChange={(e) =>
                      updateEditorSetting(
                        "useTabs",
                        e.target.value === "auto" ? "auto" : e.target.value === "tabs",
                      )
                    }
                    className="w-full mt-1 bg-[var(--bg-panel)] border border-[var(--border-default)] px-2 py-1 text-[11px] text-[var(--fg-primary)] focus:outline-none focus:border-[var(--accent-primary)]"
                  >
                    <option value="auto">Auto (project config)</option>
                    <option value="spaces">Spaces</option>
                    <option value="tabs">Tabs</option>
                  </select>
                </label>
              </div>
              <span className="text-[10px] text-[var(--fg-tertiary)] block">
                Explicit values above override project config; "Auto" defers to it.
              </span>
            </div>
          ) : activeTab === "providers" ? (
            <div className="space-y-3 text-xs">
              <div className="flex items-center justify-between">
                <div>
                  <span className="text-[var(--fg-secondary)] font-semibold">Provider Profiles</span>
                  <span className="text-[10px] text-[var(--fg-tertiary)] block">
                    Connect Google Antigravity, OpenRouter, OpenCode, KiloCode, or custom OpenAI-compatible endpoints.
                  </span>
                </div>
              </div>

              {/* OAuth Active Overlay */}
              {oauthFlow.active && (
                <div className="p-3 bg-[var(--bg-app)] border border-cyan-500/50 rounded-lg space-y-2 font-mono">
                  <div className="flex items-center justify-between">
                    <span className="font-semibold text-cyan-300 flex items-center gap-1.5">
                      <IconRefresh className="size-3.5 animate-spin" />
                      Authenticating with {oauthFlow.provider}...
                    </span>
                    <button
                      onClick={() => setOauthFlow({ active: false, status: "idle" })}
                      className="text-[10px] text-[var(--fg-tertiary)] hover:text-white cursor-pointer"
                    >
                      Cancel
                    </button>
                  </div>
                  {oauthFlow.userCode && (
                    <div className="p-2 bg-[var(--bg-panel)] rounded border border-[var(--border-default)] text-center">
                      <div className="text-[10px] text-[var(--fg-tertiary)]">Confirmation Code:</div>
                      <div className="text-base font-bold text-yellow-400 tracking-widest my-1 select-all">{oauthFlow.userCode}</div>
                      <div className="text-[10px] text-[var(--fg-secondary)]">{oauthFlow.instructions}</div>
                    </div>
                  )}
                  {oauthFlow.authUrl && (
                    <div className="flex items-center justify-between pt-1">
                      <span className="text-[10px] text-[var(--fg-tertiary)]">Browser did not open automatically?</span>
                      <button
                        type="button"
                        onClick={() => BrowserOpenURL(oauthFlow.authUrl || "")}
                        className="px-2.5 py-1 bg-cyan-950/60 border border-cyan-700 text-cyan-300 hover:text-white rounded text-[10.5px] flex items-center gap-1 font-semibold cursor-pointer"
                      >
                        Open Google Login <IconExternalLink className="size-3" />
                      </button>
                    </div>
                  )}

                  {/* Manual Paste Code fallback */}
                  <div className="pt-2 border-t border-[var(--border-default)] space-y-1.5">
                    <div className="text-[10px] text-[var(--fg-tertiary)] flex items-center justify-between">
                      <span>Or paste full redirect URL or code below:</span>
                    </div>
                    <div className="flex items-center gap-1.5">
                      <input
                        value={manualOAuthCode}
                        onChange={(e) => setManualOAuthCode(e.target.value)}
                        placeholder="http://127.0.0.1:51121/oauth-callback?code=4/0A... or raw code"
                        className="flex-1 bg-[var(--bg-panel)] border border-[var(--border-default)] px-2.5 py-1 text-[var(--fg-primary)] font-mono text-[10.5px] rounded focus:outline-none focus:border-[var(--accent-primary)]"
                        onKeyDown={(e) => {
                          if (e.key === "Enter") handleManualCodeSubmit();
                        }}
                      />
                      <button
                        onClick={handleManualCodeSubmit}
                        disabled={!manualOAuthCode.trim() || submittingManualCode}
                        className="px-3 py-1 bg-cyan-600 hover:bg-cyan-500 text-white font-semibold rounded text-[10.5px] cursor-pointer disabled:opacity-50 flex items-center gap-1 shrink-0"
                      >
                        {submittingManualCode ? <IconRefresh className="size-3 animate-spin" /> : null}
                        <span>Authorize</span>
                      </button>
                    </div>
                  </div>
                </div>
              )}

              {/* Add / Configure Provider Card */}
              <div className="border border-[var(--border-default)] p-3 space-y-3 bg-[var(--bg-panel)] rounded-lg">
                <div className="space-y-1">
                  <label className="text-[10px] text-[var(--fg-tertiary)] font-semibold uppercase tracking-wider block">
                    Choose Provider Preset
                  </label>
                  <select
                    value={selectedPresetId}
                    onChange={(e) => {
                      const pId = e.target.value;
                      setSelectedPresetId(pId);
                      const preset = PROVIDER_PRESETS.find((p) => p.id === pId);
                      if (preset) {
                        setNewProvider({
                          name: preset.name.split(" (")[0],
                          apiKey: "",
                          baseUrl: preset.baseUrl || "",
                        });
                        setDefaultModels(preset.defaultModels);
                        setFetchedModels(preset.defaultModels);
                      }
                    }}
                    className="w-full bg-[var(--bg-app)] border border-[var(--border-default)] px-2.5 py-1.5 text-[var(--fg-primary)] font-medium text-[11px] rounded focus:outline-none focus:border-[var(--accent-primary)]"
                  >
                    {PROVIDER_PRESETS.map((preset) => (
                      <option key={preset.id} value={preset.id}>
                        {preset.name}
                      </option>
                    ))}
                  </select>
                </div>

                {(() => {
                  const currentPreset = PROVIDER_PRESETS.find((p) => p.id === selectedPresetId);
                  const isOAuth = currentPreset?.authType === "oauth" || currentPreset?.authType === "device";

                  return (
                    <div className="space-y-2.5 pt-1 border-t border-[var(--border-default)]">
                      {isOAuth ? (
                        <div className="p-3 bg-[var(--bg-sidebar)] border border-[var(--border-default)] rounded flex flex-col md:flex-row items-center justify-between gap-3">
                          <div>
                            <span className="font-semibold text-[11px] text-[var(--fg-primary)] block">
                              {currentPreset?.name}
                            </span>
                            <span className="text-[10px] text-[var(--fg-tertiary)] block mt-0.5">
                              {currentPreset?.description}
                            </span>
                          </div>
                          <button
                            onClick={() => handleStartOAuth(currentPreset!.id)}
                            className="px-4 py-1.5 bg-[var(--accent-primary)] hover:bg-[var(--accent-hover)] text-black font-semibold rounded text-xs cursor-pointer flex items-center gap-1.5 shrink-0 shadow-sm"
                          >
                            <IconBrandGoogle className="size-3.5" />
                            <span>Login via {currentPreset?.id === "kilo" ? "Device Auth" : "Google OAuth"}</span>
                          </button>
                        </div>
                      ) : (
                        <div className="space-y-2">
                          <div className="grid grid-cols-1 md:grid-cols-2 gap-2">
                            <div>
                              <label className="text-[10px] text-[var(--fg-tertiary)] block mb-0.5 font-medium">Provider Name</label>
                              <input
                                value={newProvider.name}
                                onChange={(e) => setNewProvider({ ...newProvider, name: e.target.value })}
                                placeholder="e.g. OpenRouter or Custom"
                                className="w-full bg-[var(--bg-app)] border border-[var(--border-default)] px-2.5 py-1 text-[var(--fg-primary)] focus:outline-none focus:border-[var(--accent-primary)] font-mono text-[11px] rounded"
                              />
                            </div>
                            <div>
                              <label className="text-[10px] text-[var(--fg-tertiary)] block mb-0.5 font-medium">Base URL</label>
                              <input
                                value={newProvider.baseUrl}
                                onChange={(e) => setNewProvider({ ...newProvider, baseUrl: e.target.value })}
                                placeholder="https://api..."
                                className="w-full bg-[var(--bg-app)] border border-[var(--border-default)] px-2.5 py-1 text-[var(--fg-primary)] focus:outline-none focus:border-[var(--accent-primary)] font-mono text-[11px] rounded"
                              />
                            </div>
                          </div>

                          <div>
                            <div className="flex items-center justify-between mb-0.5">
                              <label className="text-[10px] text-[var(--fg-tertiary)] font-medium">API Key</label>
                              {currentPreset?.keyUrl && (
                                <button
                                  type="button"
                                  onClick={() => BrowserOpenURL(currentPreset.keyUrl || "")}
                                  className="text-[10px] text-cyan-400 hover:underline flex items-center gap-0.5 cursor-pointer bg-transparent border-0 p-0"
                                >
                                  Get API Key ↗
                                </button>
                              )}
                            </div>
                            <div className="flex items-center gap-2">
                              <input
                                type="password"
                                value={newProvider.apiKey}
                                onChange={(e) => setNewProvider({ ...newProvider, apiKey: e.target.value })}
                                placeholder="sk-..."
                                className="flex-1 bg-[var(--bg-app)] border border-[var(--border-default)] px-2.5 py-1 text-[var(--fg-primary)] focus:outline-none focus:border-[var(--accent-primary)] font-mono text-[11px] rounded"
                              />
                              <button
                                onClick={handleFetchModels}
                                disabled={fetchingModels || !newProvider.apiKey}
                                className="px-2.5 py-1 bg-[var(--bg-surface-hover)] border border-[var(--border-default)] hover:bg-[var(--bg-surface-active)] text-[var(--fg-primary)] rounded cursor-pointer disabled:opacity-50 flex items-center gap-1 text-[10px]"
                              >
                                <IconRefresh className={fetchingModels ? "size-3 animate-spin" : "size-3"} />
                                Fetch Models
                              </button>
                            </div>
                          </div>
                        </div>
                      )}

                      {fetchedModels.length > 0 && !isOAuth && (
                        <div className="space-y-1.5 pt-1 border-t border-[var(--border-default)]">
                          <div className="flex items-center justify-between">
                            <span className="text-[10px] text-[var(--fg-tertiary)] font-semibold uppercase tracking-wider">
                              Discovered Models ({defaultModels.length}/{fetchedModels.length} selected)
                            </span>
                            <div className="flex items-center gap-2">
                              <button
                                type="button"
                                onClick={() => setDefaultModels([...fetchedModels])}
                                className="text-[10px] text-cyan-400 hover:underline cursor-pointer"
                              >
                                Select All
                              </button>
                              <button
                                type="button"
                                onClick={() => setDefaultModels([])}
                                className="text-[10px] text-[var(--fg-tertiary)] hover:text-white cursor-pointer"
                              >
                                Clear All
                              </button>
                            </div>
                          </div>

                          {/* Search model filter */}
                          <div className="relative">
                            <input
                              value={fetchedModelSearch}
                              onChange={(e) => setFetchedModelSearch(e.target.value)}
                              placeholder="Filter models (e.g. gpt-4o, claude, sonnet)..."
                              className="w-full bg-[var(--bg-app)] border border-[var(--border-default)] px-2 py-1 text-[var(--fg-primary)] font-mono text-[10.5px] rounded focus:outline-none focus:border-[var(--accent-primary)]"
                            />
                            {fetchedModelSearch && (
                              <button
                                type="button"
                                onClick={() => setFetchedModelSearch("")}
                                className="absolute right-1.5 top-1.5 text-[var(--fg-tertiary)] hover:text-white cursor-pointer"
                              >
                                <IconX className="size-3" />
                              </button>
                            )}
                          </div>

                          <div className="max-h-40 overflow-y-auto border border-[var(--border-default)] p-1.5 space-y-0.5 bg-[var(--bg-app)] rounded">
                            {fetchedModels
                              .filter((m) => !fetchedModelSearch.trim() || m.toLowerCase().includes(fetchedModelSearch.trim().toLowerCase()))
                              .map((m) => (
                                <label key={m} className="flex items-center gap-1.5 px-1.5 py-0.5 hover:bg-[var(--bg-surface-hover)] cursor-pointer rounded text-[10.5px]">
                                  <input
                                    type="checkbox"
                                    checked={defaultModels.includes(m)}
                                    onChange={(e) => {
                                      if (e.target.checked) {
                                        setDefaultModels([...defaultModels, m]);
                                      } else {
                                        setDefaultModels(defaultModels.filter((id) => id !== m));
                                      }
                                    }}
                                    className="accent-[var(--accent-primary)] cursor-pointer"
                                  />
                                  <span className="font-mono text-[var(--fg-secondary)]">{m}</span>
                                </label>
                              ))}
                          </div>
                        </div>
                      )}

                      {!isOAuth && (
                        <div className="flex items-center justify-end pt-1">
                          <button
                            onClick={handleSaveNewProvider}
                            disabled={!newProvider.name || !newProvider.apiKey}
                            className="px-3.5 py-1 bg-[var(--accent-primary)] hover:bg-[var(--accent-hover)] text-black font-semibold rounded cursor-pointer disabled:opacity-50 text-[11px]"
                          >
                            Save Provider Profile
                          </button>
                        </div>
                      )}
                    </div>
                  );
                })()}
              </div>

              {/* Configured Profiles List */}
              <div className="space-y-2 pt-1">
                <div className="text-[10px] text-[var(--fg-tertiary)] font-semibold uppercase tracking-wider">
                  Configured Profiles ({profiles.length})
                </div>
                <div className="space-y-2 max-h-[320px] overflow-y-auto pr-1">
                  {profiles.map((p) => {
                    const models = toPlainStringArray(p.models);
                    const hasExplicit = Array.isArray(p.selected_models);
                    const selected = hasExplicit ? toPlainStringArray(p.selected_models) : models;
                    const isAntigravity = p.id === "google-antigravity" || p.provider === "google-antigravity";

                    return (
                      <div
                        key={p.id}
                        className={`border border-[var(--border-default)] p-3 space-y-2 bg-[var(--bg-panel)] rounded-lg transition-colors ${
                          p.enabled ? "" : "opacity-60"
                        }`}
                      >
                        <div className="flex items-center justify-between">
                          <div className="flex items-center gap-2 flex-wrap">
                            <span className="font-mono font-semibold text-[11.5px] text-[var(--fg-primary)]">{p.name || p.id}</span>
                            <span className="text-[9px] px-1.5 py-0.2 bg-[var(--bg-surface)] border border-[var(--border-default)] text-[var(--fg-tertiary)] font-mono rounded">
                              {p.id}
                            </span>
                            {p.accountEmail && (
                              <span className="text-[9px] px-1.5 py-0.2 bg-blue-950/60 border border-blue-800/60 text-blue-300 font-mono rounded">
                                {p.accountEmail}
                              </span>
                            )}
                            {p.enabled ? (
                              <span className="text-[9px] px-1.5 py-0.2 bg-emerald-500/20 text-emerald-400 font-mono rounded">
                                active
                              </span>
                            ) : (
                              <span className="text-[9px] px-1.5 py-0.2 bg-[var(--bg-surface)] text-[var(--fg-tertiary)] font-mono rounded">
                                disabled
                              </span>
                            )}
                          </div>
                          <div className="flex items-center gap-2">
                            {isAntigravity && (
                              <button
                                onClick={() => handleStartOAuth("google-antigravity")}
                                className="px-2 py-0.5 text-[10px] bg-cyan-950/60 border border-cyan-800 text-cyan-300 hover:text-white rounded cursor-pointer"
                                title="Re-authenticate with Google"
                              >
                                Re-login OAuth
                              </button>
                            )}
                            <button
                              onClick={() => handleToggleProvider(p.id)}
                              className={`px-2 py-0.5 text-[10px] rounded cursor-pointer transition-colors ${
                                p.enabled
                                  ? "bg-[var(--accent-primary)] text-black font-semibold"
                                  : "bg-[var(--bg-surface-hover)] text-[var(--fg-secondary)]"
                              }`}
                            >
                              {p.enabled ? "Enabled" : "Enable"}
                            </button>
                            <button
                              onClick={() => handleDeleteProvider(p.id)}
                              className="p-1 hover:bg-[var(--bg-surface-hover)] rounded text-[var(--fg-tertiary)] hover:text-red-400 cursor-pointer"
                              title="Delete provider profile"
                            >
                              <IconTrash className="size-3.5" />
                            </button>
                          </div>
                        </div>

                        <div className="grid grid-cols-2 gap-2 text-[10px] font-mono text-[var(--fg-tertiary)]">
                          <div className="truncate">
                            <span className="text-[var(--fg-muted)]">Base URL:</span> {p.baseURL || "—"}
                          </div>
                          <div className="truncate">
                            <span className="text-[var(--fg-muted)]">API Key:</span> {p.apiKey ? `${p.apiKey.slice(0, 4)}...${p.apiKey.slice(-4)}` : "None"}
                          </div>
                        </div>

                        {/* Active Model Selector */}
                        {models.length > 0 && (
                          <div className="space-y-1.5 pt-1 border-t border-[var(--border-default)]">
                            <div className="flex items-center justify-between">
                              <div className="flex items-center gap-2 flex-1 min-w-0 mr-2">
                                <span className="text-[10px] text-[var(--fg-muted)] shrink-0 font-medium">Active Model:</span>
                                <select
                                  value={p.activeModel}
                                  onChange={(e) => handleSetActiveModel(p.id, e.target.value)}
                                  className="flex-1 bg-[var(--bg-app)] border border-[var(--border-default)] px-2 py-0.5 text-[var(--fg-primary)] font-mono text-[10.5px] rounded focus:outline-none focus:border-[var(--accent-primary)]"
                                >
                                  {models.map((m) => (
                                    <option key={m} value={m}>
                                      {m}
                                    </option>
                                  ))}
                                </select>
                              </div>
                              <button
                                type="button"
                                onClick={() => setExpandedModels((prev) => ({ ...prev, [p.id]: !prev[p.id] }))}
                                className="text-[10px] text-cyan-400 hover:underline flex items-center gap-1 cursor-pointer shrink-0 font-medium"
                              >
                                <span>{expandedModels[p.id] ? "Hide Catalog" : `Manage Models (${selected.length}/${models.length})`}</span>
                                {expandedModels[p.id] ? <IconChevronDown className="size-3" /> : <IconChevronRight className="size-3" />}
                              </button>
                            </div>

                            {/* Expandable Model Catalog with Checkboxes & Search */}
                            {expandedModels[p.id] && (
                              <div className="p-2 bg-[var(--bg-sidebar)] border border-[var(--border-default)] rounded-md space-y-1.5 font-mono text-[10.5px]">
                                <div className="flex items-center justify-between">
                                  <span className="text-[9.5px] text-[var(--fg-tertiary)] uppercase font-semibold">
                                    Enabled in Model Picker ({selected.length || models.length}/{models.length})
                                  </span>
                                  <div className="flex items-center gap-2">
                                    <button
                                      type="button"
                                      onClick={() => handleRefreshProviderModels(p.id)}
                                      disabled={refreshingModels[p.id]}
                                      className="text-[9.5px] text-cyan-400 hover:underline cursor-pointer flex items-center gap-1 disabled:opacity-50 font-medium"
                                      title="Fetch latest available models from provider API"
                                    >
                                      <IconRefresh className={`size-2.5 ${refreshingModels[p.id] ? "animate-spin" : ""}`} />
                                      <span>{refreshingModels[p.id] ? "Fetching..." : "Fetch Models"}</span>
                                    </button>
                                    <span className="text-[var(--fg-tertiary)] text-[9px]">|</span>
                                    <button
                                      type="button"
                                      onClick={() => handleSelectAllProfileModels(p, true)}
                                      className="text-[9.5px] text-cyan-400 hover:underline cursor-pointer"
                                    >
                                      Select All
                                    </button>
                                    <button
                                      type="button"
                                      onClick={() => handleSelectAllProfileModels(p, false)}
                                      className="text-[9.5px] text-[var(--fg-tertiary)] hover:text-white cursor-pointer"
                                    >
                                      Clear All
                                    </button>
                                  </div>
                                </div>

                                {/* Model filter search */}
                                <div className="relative">
                                  <input
                                    value={profileModelSearch[p.id] || ""}
                                    onChange={(e) => setProfileModelSearch((prev) => ({ ...prev, [p.id]: e.target.value }))}
                                    placeholder="Search model name..."
                                    className="w-full bg-[var(--bg-app)] border border-[var(--border-default)] px-2 py-0.8 text-[var(--fg-primary)] font-mono text-[10px] rounded focus:outline-none focus:border-[var(--accent-primary)]"
                                  />
                                  {profileModelSearch[p.id] && (
                                    <button
                                      type="button"
                                      onClick={() => setProfileModelSearch((prev) => ({ ...prev, [p.id]: "" }))}
                                      className="absolute right-1.5 top-1 text-[var(--fg-tertiary)] hover:text-white cursor-pointer"
                                    >
                                      <IconX className="size-3" />
                                    </button>
                                  )}
                                </div>

                                <div className="max-h-40 overflow-y-auto border border-[var(--border-default)] p-1 space-y-0.5 bg-[var(--bg-app)] rounded">
                                  {models
                                    .filter((m) => {
                                      const query = (profileModelSearch[p.id] || "").trim().toLowerCase();
                                      return !query || m.toLowerCase().includes(query);
                                    })
                                    .map((m) => {
                                      const isChecked = selected.includes(m);
                                      const isActive = p.activeModel === m;

                                      return (
                                        <div
                                          key={m}
                                          className={`flex items-center justify-between px-1.5 py-0.5 hover:bg-[var(--bg-surface-hover)] rounded cursor-pointer ${
                                            isActive ? "bg-cyan-950/40 border border-cyan-800/40" : ""
                                          }`}
                                        >
                                          <label className="flex items-center gap-1.5 flex-1 min-w-0 cursor-pointer">
                                            <input
                                              type="checkbox"
                                              checked={isChecked}
                                              onChange={() => handleToggleModel(p, m)}
                                              className="accent-[var(--accent-primary)] cursor-pointer"
                                            />
                                            <span className={`truncate ${isActive ? "text-cyan-300 font-semibold" : "text-[var(--fg-secondary)]"}`}>
                                              {m}
                                            </span>
                                          </label>
                                          <div className="flex items-center gap-1 shrink-0 ml-2">
                                            {isActive ? (
                                              <span className="text-[9px] px-1 py-px bg-cyan-500/20 text-cyan-400 font-mono rounded">
                                                active
                                              </span>
                                            ) : (
                                              <button
                                                type="button"
                                                onClick={() => handleSetActiveModel(p.id, m)}
                                                className="text-[9px] text-[var(--fg-tertiary)] hover:text-cyan-300 cursor-pointer px-1 py-px hover:bg-[var(--bg-panel)] rounded"
                                                title="Set as active model for this provider"
                                              >
                                                set active
                                              </button>
                                            )}
                                          </div>
                                        </div>
                                      );
                                    })}
                                </div>
                              </div>
                            )}
                          </div>
                        )}
                      </div>
                    );
                  })}
                </div>
              </div>
            </div>
          ) : activeTab === "agents" ? (
            <div className="space-y-3 text-xs">
              <div className="flex items-center justify-between">
                <span className="text-[var(--fg-secondary)] font-semibold">Pre-configured Agents</span>
                <button
                  onClick={() => setShowAgentForm(!showAgentForm)}
                  className="px-2 py-1 bg-[var(--bg-surface-hover)] border border-[var(--border-default)] hover:bg-[var(--bg-surface-active)] text-[var(--fg-primary)] rounded cursor-pointer flex items-center gap-1"
                >
                  <IconPlus className="size-3" />
                  New Agent
                </button>
              </div>

              {showAgentForm && (
                <div className="border border-[var(--border-default)] p-2 space-y-2 bg-[var(--bg-panel)]">
                  <input
                    value={agentForm.name}
                    onChange={(e) => setAgentForm({ ...agentForm, name: e.target.value })}
                    placeholder="Agent name (e.g. Code Reviewer)"
                    className="w-full bg-[var(--bg-app)] border border-[var(--border-default)] px-2 py-1 text-[var(--fg-primary)] focus:outline-none focus:border-[var(--accent-primary)] text-[11px]"
                  />
                  <input
                    value={agentForm.description}
                    onChange={(e) => setAgentForm({ ...agentForm, description: e.target.value })}
                    placeholder="Short description"
                    className="w-full bg-[var(--bg-app)] border border-[var(--border-default)] px-2 py-1 text-[var(--fg-primary)] focus:outline-none focus:border-[var(--accent-primary)] text-[11px]"
                  />
                  <div className="grid grid-cols-2 gap-2">
                    <select
                      value={agentForm.role}
                      onChange={(e) => setAgentForm({ ...agentForm, role: e.target.value })}
                      className="bg-[var(--bg-app)] border border-[var(--border-default)] px-2 py-1 text-[var(--fg-primary)] focus:outline-none"
                    >
                      {DEFAULT_ROLES.map((r) => <option key={r} value={r}>{r}</option>)}
                    </select>
                    <input
                      value={agentForm.model}
                      onChange={(e) => setAgentForm({ ...agentForm, model: e.target.value })}
                      placeholder="Model (provider/model)"
                      className="bg-[var(--bg-app)] border border-[var(--border-default)] px-2 py-1 text-[var(--fg-primary)] focus:outline-none focus:border-[var(--accent-primary)] font-mono text-[11px]"
                    />
                  </div>
                  <textarea
                    value={agentForm.prompt}
                    onChange={(e) => setAgentForm({ ...agentForm, prompt: e.target.value })}
                    placeholder="System prompt"
                    rows={3}
                    className="w-full bg-[var(--bg-app)] border border-[var(--border-default)] px-2 py-1 text-[var(--fg-primary)] focus:outline-none focus:border-[var(--accent-primary)] text-[11px] resize-none"
                  />
                  <textarea
                    value={agentForm.rules}
                    onChange={(e) => setAgentForm({ ...agentForm, rules: e.target.value })}
                    placeholder="Rules (one per line)"
                    rows={2}
                    className="w-full bg-[var(--bg-app)] border border-[var(--border-default)] px-2 py-1 text-[var(--fg-primary)] focus:outline-none focus:border-[var(--accent-primary)] text-[11px] resize-none"
                  />
                  <div className="flex items-center justify-end gap-1.5">
                    <button
                      onClick={() => setShowAgentForm(false)}
                      className="px-2 py-1 text-[var(--fg-secondary)] hover:text-white cursor-pointer"
                    >
                      Cancel
                    </button>
                    <button
                      onClick={handleSaveAgentDef}
                      disabled={!agentForm.name.trim()}
                      className="px-3 py-1 bg-[var(--accent-primary)] hover:bg-[var(--accent-hover)] text-black font-semibold rounded cursor-pointer disabled:opacity-50"
                    >
                      Save Agent
                    </button>
                  </div>
                </div>
              )}

              {agentDefs.map((def) => (
                <div key={def.id || def.ID} className="flex items-center justify-between px-2 py-1.5 border border-[var(--border-default)] bg-[var(--bg-panel)]">
                  <div className="flex items-center gap-1.5 min-w-0">
                    <IconRobot className="size-3.5 text-blue-400 shrink-0" />
                    <div className="min-w-0">
                      <div className="font-semibold truncate">{def.name || def.Name}</div>
                      {def.description && <div className="text-[10px] text-[var(--fg-tertiary)] truncate">{def.description}</div>}
                    </div>
                  </div>
                  <div className="flex items-center gap-1 shrink-0">
                    <span className="text-[9px] font-mono uppercase text-[var(--fg-tertiary)] bg-black/30 border border-[var(--border-default)] px-1 py-0.5 rounded">
                      {def.role_filter || def.RoleFilter || ""}
                    </span>
                    {def.model && <span className="text-[9px] font-mono text-purple-400">{def.model}</span>}
                    <button
                      onClick={() => handleDeleteAgentDef(def.id || def.ID)}
                      className="p-1 hover:bg-[var(--bg-surface-hover)] rounded text-red-500 cursor-pointer"
                    >
                      <IconTrash className="size-3.5" />
                    </button>
                  </div>
                </div>
              ))}
            </div>
          ) : activeTab === "skills" ? (
            <div className="space-y-3 text-xs">
              <div className="flex items-center justify-between">
                <div>
                  <div className="flex items-center gap-2">
                    <span className="text-[var(--fg-secondary)] font-semibold">Agent Skills</span>
                    <span className="rounded border border-[var(--border-default)] px-1.5 py-0.2 text-[10px] font-mono text-cyan-300 bg-[var(--bg-panel)]">
                      {skills.filter((s) => s.enabled).length}/{skills.length} enabled
                    </span>
                  </div>
                  <span className="text-[10px] text-[var(--fg-tertiary)] block mt-0.5">
                    Discovered from .agents/skills, ~/.agents/skills, .claude, .codex, opencode, npx skills...
                  </span>
                </div>
                <div className="flex items-center gap-1.5">
                  <button
                    onClick={handleRefreshSkills}
                    disabled={refreshingSkills}
                    className="px-2 py-0.5 text-[10px] bg-[var(--bg-panel)] hover:bg-[var(--bg-surface-hover)] border border-[var(--border-default)] rounded text-[var(--fg-secondary)] hover:text-white cursor-pointer flex items-center gap-1 disabled:opacity-50"
                    title="Re-scan all skill sources (.agents, ~/.agents/skills, .claude, .codex, opencode, npx...) for newly added or updated skills"
                  >
                    <IconRefresh className={`size-3 ${refreshingSkills ? "animate-spin text-cyan-400" : ""}`} />
                    {refreshingSkills ? "Scanning..." : "Refresh"}
                  </button>
                  <button
                    onClick={async () => {
                      await SetAllSkillsEnabled(true);
                      await loadSkills();
                      toast("All skills enabled", "success");
                    }}
                    className="px-2 py-0.5 text-[10px] bg-[var(--bg-panel)] hover:bg-[var(--bg-surface-hover)] border border-[var(--border-default)] rounded text-[var(--fg-secondary)] hover:text-white cursor-pointer"
                  >
                    Enable All
                  </button>
                  <button
                    onClick={async () => {
                      await SetAllSkillsEnabled(false);
                      await loadSkills();
                      toast("All skills disabled", "success");
                    }}
                    className="px-2 py-0.5 text-[10px] bg-[var(--bg-panel)] hover:bg-[var(--bg-surface-hover)] border border-[var(--border-default)] rounded text-[var(--fg-secondary)] hover:text-white cursor-pointer"
                  >
                    Disable All
                  </button>
                </div>
              </div>

              {/* Filter search */}
              <div className="relative">
                <IconSearch className="size-3 absolute left-2.5 top-2.5 text-[var(--fg-tertiary)]" />
                <input
                  type="text"
                  value={skillsSearch}
                  onChange={(e) => setSkillsSearch(e.target.value)}
                  placeholder="Search skills by name, description, or source..."
                  className="w-full bg-[var(--bg-panel)] border border-[var(--border-default)] pl-7 pr-3 py-1.5 text-[11px] text-[var(--fg-primary)] focus:outline-none focus:border-[var(--accent-primary)] font-mono rounded"
                />
              </div>

              {/* Skills list */}
              <div className="space-y-1.5 max-h-[440px] overflow-y-auto pr-1">
                {skills
                  .filter((s) => {
                    if (!skillsSearch.trim()) return true;
                    const q = skillsSearch.toLowerCase();
                    return (
                      s.name.toLowerCase().includes(q) ||
                      s.description.toLowerCase().includes(q) ||
                      s.source.toLowerCase().includes(q)
                    );
                  })
                  .map((skill) => (
                    <div
                      key={skill.name}
                      className="flex items-start gap-2.5 p-2 bg-[var(--bg-panel)] border border-[var(--border-default)] hover:border-[var(--border-hover)] rounded transition-colors"
                    >
                      <input
                        type="checkbox"
                        checked={skill.enabled}
                        onChange={async (e) => {
                          const next = e.target.checked;
                          setSkills((prev) =>
                            prev.map((s) => (s.name === skill.name ? { ...s, enabled: next } : s))
                          );
                          await SetSkillEnabled(skill.name, next);
                          toast(`Skill "${skill.name}" ${next ? "enabled" : "disabled"}`, "info");
                        }}
                        className="mt-0.5 cursor-pointer accent-[var(--accent-primary)] shrink-0"
                      />
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2 flex-wrap">
                          <span className="font-mono font-semibold text-[11.5px] text-cyan-300">
                            {skill.name}
                          </span>
                          <span className="text-[9px] px-1.5 py-0.2 bg-[var(--bg-surface)] border border-[var(--border-default)] text-[var(--fg-tertiary)] font-mono rounded">
                            {skill.source}
                          </span>
                        </div>
                        {skill.description && (
                          <p className="text-[11px] text-[var(--fg-secondary)] mt-0.5 leading-relaxed">
                            {skill.description}
                          </p>
                        )}
                        <p className="text-[9px] text-[var(--fg-tertiary)] font-mono truncate mt-0.5" title={skill.path}>
                          {skill.path}
                        </p>
                      </div>
                    </div>
                  ))}
                {skills.length === 0 && (
                  <div className="py-6 text-center text-[var(--fg-tertiary)] italic text-[11px]">
                    No skills discovered yet. Install skills with <code>npx skills add &lt;name&gt;</code> or place them in <code>.agents/skills/</code>.
                  </div>
                )}
              </div>
            </div>
          ) : activeTab === "ai-commit" ? (
            <div className="space-y-3 text-xs">
              <span className="text-[var(--fg-secondary)] font-semibold block">AI Commit Generator</span>

              <div className="space-y-1.5">
                <label className="text-[var(--fg-tertiary)] block font-medium">Provider</label>
                <select
                  value={commitProvider}
                  onChange={(e) => {
                    const pid = e.target.value;
                    setCommitProvider(pid);
                    const p = profiles.find((x) => (x.id || x.Id) === pid);
                    const models = p?.selected_models || p?.SelectedModels || p?.available_models || p?.AvailableModels || [];
                    if (models.length > 0) {
                      setCommitModel(models[0]);
                      saveCommitConfig(pid, models[0], commitPrompt);
                    } else {
                      saveCommitConfig(pid, commitModel, commitPrompt);
                    }
                  }}
                  className="w-full bg-[var(--bg-app)] border border-[var(--border-default)] px-2 py-1.5 text-[var(--fg-primary)] focus:outline-none focus:border-[var(--accent-primary)] font-mono text-[11px]"
                >
                  <option value="">— Select provider —</option>
                  {profiles.map((p) => (
                    <option key={p.id || p.Id} value={p.id || p.Id}>{p.name || p.Name}</option>
                  ))}
                </select>
              </div>

              <div className="space-y-1.5">
                <label className="text-[var(--fg-tertiary)] block font-medium">Model</label>
                <select
                  value={commitModel}
                  onChange={(e) => {
                    setCommitModel(e.target.value);
                    saveCommitConfig(commitProvider, e.target.value, commitPrompt);
                  }}
                  className="w-full bg-[var(--bg-app)] border border-[var(--border-default)] px-2 py-1.5 text-[var(--fg-primary)] focus:outline-none focus:border-[var(--accent-primary)] font-mono text-[11px]"
                >
                  <option value="">— Select model —</option>
                  {(() => {
                    const p = profiles.find((x) => (x.id || x.Id) === commitProvider);
                    const models = p?.selected_models || p?.SelectedModels || p?.available_models || p?.AvailableModels || [];
                    return models.map((m: string) => <option key={m} value={m}>{m}</option>);
                  })()}
                </select>
              </div>

              <div className="space-y-1.5">
                <div className="flex items-center justify-between">
                  <label className="text-[var(--fg-tertiary)] block font-medium">Prompt</label>
                  <button
                    onClick={() => {
                      setCommitPrompt(DEFAULT_COMMIT_PROMPT);
                      saveCommitConfig(commitProvider, commitModel, DEFAULT_COMMIT_PROMPT);
                    }}
                    className="px-2 py-0.5 text-[10px] text-[var(--fg-secondary)] hover:text-white border border-[var(--border-default)] rounded cursor-pointer"
                  >
                    Reset default
                  </button>
                </div>
                <textarea
                  value={commitPrompt}
                  onChange={(e) => {
                    setCommitPrompt(e.target.value);
                    saveCommitConfig(commitProvider, commitModel, e.target.value);
                  }}
                  rows={6}
                  className="w-full bg-[var(--bg-app)] border border-[var(--border-default)] px-2 py-1.5 text-[var(--fg-primary)] focus:outline-none focus:border-[var(--accent-primary)] text-[11px] resize-none"
                />
              </div>

              <div className="text-[10px] text-[var(--fg-tertiary)] italic">
                Config ini dipakai oleh tombol AI Msg di Git Control untuk generate commit message.
              </div>
            </div>
          ) : activeTab === "usage" ? (
            <div className="space-y-4 text-xs">
              <div className="flex items-center justify-between">
                <div>
                  <span className="text-[var(--fg-secondary)] font-semibold text-sm">Token & Quota Usage</span>
                  <span className="text-[10px] text-[var(--fg-tertiary)] block">
                    Session metrics, workspace consumption breakdown, and live provider quota status.
                  </span>
                </div>
                <button
                  onClick={loadUsageData}
                  disabled={loadingUsage}
                  className="px-2.5 py-1 bg-[var(--bg-panel)] hover:bg-[var(--bg-surface-hover)] border border-[var(--border-default)] rounded text-[var(--fg-secondary)] hover:text-white cursor-pointer flex items-center gap-1 text-[10px]"
                >
                  <IconRefresh className={loadingUsage ? "size-3 animate-spin text-cyan-400" : "size-3"} />
                  Refresh
                </button>
              </div>

              {/* Overview Metric Cards */}
              <div className="grid grid-cols-2 md:grid-cols-4 gap-2">
                <div className="bg-[var(--bg-panel)] border border-[var(--border-default)] p-2.5 rounded-lg space-y-1">
                  <span className="text-[10px] text-[var(--fg-tertiary)] uppercase font-semibold tracking-wider block">Prompt Tokens</span>
                  <span className="text-base font-bold font-mono text-[var(--fg-primary)]">
                    {(usageSummary?.totalPromptTokens ?? 0).toLocaleString()}
                  </span>
                </div>
                <div className="bg-[var(--bg-panel)] border border-[var(--border-default)] p-2.5 rounded-lg space-y-1">
                  <span className="text-[10px] text-[var(--fg-tertiary)] uppercase font-semibold tracking-wider block">Completion Tokens</span>
                  <span className="text-base font-bold font-mono text-cyan-300">
                    {(usageSummary?.totalCompletionTokens ?? 0).toLocaleString()}
                  </span>
                </div>
                <div className="bg-[var(--bg-panel)] border border-[var(--border-default)] p-2.5 rounded-lg space-y-1">
                  <span className="text-[10px] text-[var(--fg-tertiary)] uppercase font-semibold tracking-wider block">Cached Tokens</span>
                  <div className="flex items-baseline gap-1.5">
                    <span className="text-base font-bold font-mono text-emerald-400">
                      {(usageSummary?.totalCachedTokens ?? 0).toLocaleString()}
                    </span>
                    <span className="text-[10px] font-mono text-emerald-500/80">({usageSummary?.cacheHitRate ?? 0}%)</span>
                  </div>
                </div>
                <div className="bg-[var(--bg-panel)] border border-[var(--border-default)] p-2.5 rounded-lg space-y-1">
                  <span className="text-[10px] text-[var(--fg-tertiary)] uppercase font-semibold tracking-wider block">Total Turn Requests</span>
                  <span className="text-base font-bold font-mono text-[var(--fg-primary)]">
                    {(usageSummary?.requestCount ?? 0).toLocaleString()}
                  </span>
                </div>
              </div>

              {/* Multi-Account Antigravity Quotas (Daily 5h, Daily, Weekly) */}
              {allProviderQuotas.length > 0 ? (
                <div className="space-y-3">
                  <div className="flex items-center justify-between">
                    <span className="text-[10px] text-[var(--fg-tertiary)] font-semibold uppercase tracking-wider block">
                      Live Antigravity Quotas ({allProviderQuotas.length} Account{allProviderQuotas.length > 1 ? "s" : ""})
                    </span>
                    <button
                      type="button"
                      onClick={loadUsageData}
                      disabled={loadingUsage}
                      className="text-[10px] text-cyan-400 hover:underline flex items-center gap-1 cursor-pointer disabled:opacity-50 font-mono"
                    >
                      <IconRefresh className={`size-3 ${loadingUsage ? "animate-spin" : ""}`} />
                      <span>Refresh Quotas</span>
                    </button>
                  </div>

                  {/* Family Cards */}
                  {(["Anthropic", "Google", "OpenAI"] as const).map((counter) => {
                    const dailyLimits = allProviderQuotas.map(
                      (r) => r.limits?.find((l) => l.counter === counter && l.windowId === "daily")
                    );
                    const pctOf = (limit: typeof dailyLimits[number]) =>
                      limit?.remainingFraction !== undefined ? limit.remainingFraction * 100 : 100;
                    const pcts = dailyLimits.map(pctOf);
                    // Same aggregation as /usage: [!] when any account is
                    // exhausted/warning (≤10%), "% free" averaged over accounts.
                    const minPct = Math.min(...pcts);
                    const avgPct = pcts.reduce((sum, p) => sum + p, 0) / pcts.length;
                    const isOk = minPct > 10;

                    return (
                      <div key={counter} className="border border-[var(--border-default)] bg-[var(--bg-panel)] p-3 rounded-lg space-y-2.5 font-mono">
                        <div className="flex items-center justify-between flex-wrap gap-2">
                          <div className="flex items-center gap-2">
                            <span className={`text-[10px] font-bold px-1.5 py-0.2 rounded font-mono ${isOk ? "bg-emerald-950/60 text-emerald-400 border border-emerald-800/60" : "bg-red-950/60 text-red-400 border border-red-800/60"}`}>
                              {isOk ? "[ok]" : "[!]"}
                            </span>
                            <span className="text-xs font-bold text-[var(--fg-primary)]">
                              Usage ({counter})
                            </span>
                            <span className="text-[9px] px-1.5 py-0.2 bg-[var(--bg-app)] border border-[var(--border-default)] text-[var(--fg-tertiary)] rounded">
                              Daily
                            </span>
                          </div>
                          <span className={`text-xs font-bold ${isOk ? "text-emerald-400" : "text-amber-400"}`}>
                            {Math.round(avgPct * 10) / 10}% free
                          </span>
                        </div>

                        <div className="grid grid-cols-1 md:grid-cols-3 gap-2">
                          {allProviderQuotas.map((r, idx) => {
                            const dailyLimit = dailyLimits[idx];
                            const pct = pctOf(dailyLimit);
                            const isLow = pct <= 10;
                            const timeFormatted = formatTimeRemaining(dailyLimit?.resetTime);
                            const email = r.accountEmail || `Account ${idx + 1}`;

                            return (
                              <div key={email + idx} className="p-2 bg-[var(--bg-app)] border border-[var(--border-default)] rounded-md space-y-1.5">
                                <div className="flex items-center justify-between text-[10px]">
                                  <span className="font-semibold text-[var(--fg-secondary)] truncate" title={email}>
                                    {email.length > 20 ? email.slice(0, 19) + "…" : email}
                                  </span>
                                  {timeFormatted && (
                                    <span className="text-[9.5px] text-amber-400 font-medium shrink-0 ml-1">{timeFormatted}</span>
                                  )}
                                </div>

                                <div className="space-y-1">
                                  <div className="flex items-center justify-between text-[9px] text-[var(--fg-tertiary)]">
                                    <span className="truncate">{dailyLimit?.tier || "daily quota"}</span>
                                    <span className={`font-bold ${isLow ? "text-red-400" : "text-emerald-400"}`}>
                                      {pct}%
                                    </span>
                                  </div>

                                  <div className="w-full h-1.5 bg-[var(--bg-panel)] rounded-full overflow-hidden">
                                    <div
                                      className={`h-full transition-all ${isLow ? "bg-red-400" : "bg-emerald-400"}`}
                                      style={{ width: `${Math.max(4, Math.min(100, pct))}%` }}
                                    />
                                  </div>
                                </div>
                              </div>
                            );
                          })}
                        </div>
                      </div>
                    );
                  })}
                </div>
              ) : (
                <div className="border border-[var(--border-default)] bg-[var(--bg-panel)] p-3 rounded-lg flex items-center justify-between text-xs font-mono">
                  <div className="flex items-center gap-2">
                    <IconActivity className="size-4 text-cyan-400" />
                    <div>
                      <span className="font-semibold text-[var(--fg-primary)] block">Google Antigravity Quota</span>
                      <span className="text-[10px] text-[var(--fg-tertiary)] block">
                        Login via Google OAuth in the Providers tab to view real-time Antigravity model quotas.
                      </span>
                    </div>
                  </div>
                  <button
                    onClick={() => setActiveTab("providers")}
                    className="px-3 py-1 bg-[var(--bg-surface-hover)] border border-[var(--border-default)] hover:bg-[var(--bg-surface-active)] text-cyan-300 rounded text-[11px] cursor-pointer"
                  >
                    Go to Providers
                  </button>
                </div>
              )}

              {/* Usage Breakdown Tables */}
              <div className="space-y-2">
                <span className="text-[10px] text-[var(--fg-tertiary)] font-semibold uppercase tracking-wider block">
                  Usage Breakdown by Model
                </span>
                <div className="border border-[var(--border-default)] bg-[var(--bg-panel)] rounded-lg overflow-hidden font-mono text-[10.5px]">
                  <table className="w-full text-left">
                    <thead className="bg-[var(--bg-sidebar)] border-b border-[var(--border-default)] text-[9.5px] text-[var(--fg-tertiary)] uppercase">
                      <tr>
                        <th className="py-1.5 px-3">Model</th>
                        <th className="py-1.5 px-3 text-right">Prompt</th>
                        <th className="py-1.5 px-3 text-right">Completion</th>
                        <th className="py-1.5 px-3 text-right">Cached</th>
                        <th className="py-1.5 px-3 text-right">Requests</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-[var(--border-default)]">
                      {usageSummary && Object.entries(usageSummary.byModel).length > 0 ? (
                        Object.entries(usageSummary.byModel).map(([mod, st]) => (
                          <tr key={mod} className="hover:bg-[var(--bg-surface-hover)]">
                            <td className="py-1.5 px-3 text-[var(--fg-primary)] font-semibold">{mod}</td>
                            <td className="py-1.5 px-3 text-right text-[var(--fg-secondary)]">{st.prompt.toLocaleString()}</td>
                            <td className="py-1.5 px-3 text-right text-cyan-300">{st.completion.toLocaleString()}</td>
                            <td className="py-1.5 px-3 text-right text-emerald-400">{st.cached.toLocaleString()}</td>
                            <td className="py-1.5 px-3 text-right text-[var(--fg-tertiary)]">{st.requests}</td>
                          </tr>
                        ))
                      ) : (
                        <tr>
                          <td colSpan={5} className="py-4 text-center text-[var(--fg-tertiary)] italic">
                            No model usage recorded yet.
                          </td>
                        </tr>
                      )}
                    </tbody>
                  </table>
                </div>
              </div>
            </div>

          ) : (
            <div className="space-y-3 text-xs">
              <div className="flex items-center justify-between">
                <div>
                  <div className="flex items-center gap-2">
                    <span className="text-[var(--fg-secondary)] font-semibold">MCP Servers</span>
                    <span className="rounded border border-[var(--border-default)] px-1.5 py-0.2 text-[10px] font-mono text-cyan-300 bg-[var(--bg-panel)]">
                      {mcpServers.filter((s) => s.connected).length}/{mcpServers.length} connected
                    </span>
                    <span className="rounded border border-[var(--border-default)] px-1.5 py-0.2 text-[10px] font-mono text-[var(--fg-muted)] bg-[var(--bg-panel)]">
                      {mcpTools.length} tools available
                    </span>
                  </div>
                  <span className="text-[10px] text-[var(--fg-tertiary)] block mt-0.5">
                    Discovered from native config, opencode, Claude, Codex TOML, Cursor, Gemini...
                  </span>
                </div>
                <div className="flex items-center gap-1.5">
                  <button
                    onClick={handleRefreshMcp}
                    className="px-2 py-0.5 text-[10px] bg-[var(--bg-panel)] hover:bg-[var(--bg-surface-hover)] border border-[var(--border-default)] rounded text-[var(--fg-secondary)] hover:text-white cursor-pointer flex items-center gap-1"
                    title="Re-scan all MCP config sources (native, opencode, Claude, Codex, Cursor, Gemini...) for newly added servers"
                  >
                    <IconRefresh className="size-3" />
                    Refresh
                  </button>
                  <button
                    onClick={handleReconnectMcp}
                    disabled={mcpReconnecting}
                    className="px-2 py-0.5 text-[10px] bg-[var(--bg-panel)] hover:bg-[var(--bg-surface-hover)] border border-[var(--border-default)] rounded text-[var(--fg-secondary)] hover:text-white cursor-pointer flex items-center gap-1 disabled:opacity-50"
                    title="Reconnect all active MCP servers"
                  >
                    <IconRefresh className={`size-3 ${mcpReconnecting ? "animate-spin text-cyan-400" : ""}`} />
                    {mcpReconnecting ? "Connecting..." : "Reconnect All"}
                  </button>
                  <button
                    onClick={() => setShowMcpForm(!showMcpForm)}
                    className="px-2 py-0.5 text-[10px] bg-[var(--bg-surface-hover)] border border-[var(--border-default)] hover:bg-[var(--bg-surface-active)] text-[var(--fg-primary)] rounded cursor-pointer flex items-center gap-1"
                  >
                    <IconPlus className="size-3" />
                    New Server
                  </button>
                </div>
              </div>

              {showMcpForm && (
                <div className="border border-[var(--border-default)] p-3 space-y-2.5 bg-[var(--bg-panel)] rounded">
                  <div className="flex items-center justify-between pb-1 border-b border-[var(--border-default)]">
                    <span className="font-semibold text-[11px] text-[var(--fg-primary)]">Add Custom MCP Server</span>
                    <div className="flex items-center gap-1 bg-[var(--bg-sidebar)] p-0.5 rounded border border-[var(--border-default)] text-[10px]">
                      <button
                        type="button"
                        onClick={() => setMcpForm({ ...mcpForm, type: "stdio" })}
                        className={`px-2 py-0.5 rounded cursor-pointer ${mcpForm.type === "stdio" ? "bg-[var(--bg-app)] font-semibold text-cyan-300" : "text-[var(--fg-muted)]"}`}
                      >
                        stdio (local)
                      </button>
                      <button
                        type="button"
                        onClick={() => setMcpForm({ ...mcpForm, type: "remote" })}
                        className={`px-2 py-0.5 rounded cursor-pointer ${mcpForm.type === "remote" ? "bg-[var(--bg-app)] font-semibold text-cyan-300" : "text-[var(--fg-muted)]"}`}
                      >
                        HTTP (remote)
                      </button>
                    </div>
                  </div>

                  <div className="grid grid-cols-1 md:grid-cols-2 gap-2">
                    <div>
                      <label className="text-[10px] text-[var(--fg-tertiary)] block mb-0.5 font-medium">Server Name *</label>
                      <input
                        value={mcpForm.name}
                        onChange={(e) => setMcpForm({ ...mcpForm, name: e.target.value })}
                        placeholder="e.g. filesystem or exa"
                        className="w-full bg-[var(--bg-app)] border border-[var(--border-default)] px-2.5 py-1 text-[var(--fg-primary)] focus:outline-none focus:border-[var(--accent-primary)] font-mono text-[11px] rounded"
                      />
                    </div>

                    {mcpForm.type === "remote" ? (
                      <div>
                        <label className="text-[10px] text-[var(--fg-tertiary)] block mb-0.5 font-medium">Remote URL *</label>
                        <input
                          value={mcpForm.url}
                          onChange={(e) => setMcpForm({ ...mcpForm, url: e.target.value })}
                          placeholder="https://mcp.exa.ai/mcp..."
                          className="w-full bg-[var(--bg-app)] border border-[var(--border-default)] px-2.5 py-1 text-[var(--fg-primary)] focus:outline-none focus:border-[var(--accent-primary)] font-mono text-[11px] rounded"
                        />
                      </div>
                    ) : (
                      <div>
                        <label className="text-[10px] text-[var(--fg-tertiary)] block mb-0.5 font-medium">Command *</label>
                        <input
                          value={mcpForm.command}
                          onChange={(e) => setMcpForm({ ...mcpForm, command: e.target.value })}
                          placeholder="e.g. npx or node"
                          className="w-full bg-[var(--bg-app)] border border-[var(--border-default)] px-2.5 py-1 text-[var(--fg-primary)] focus:outline-none focus:border-[var(--accent-primary)] font-mono text-[11px] rounded"
                        />
                      </div>
                    )}
                  </div>

                  {mcpForm.type === "stdio" ? (
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-2">
                      <div>
                        <label className="text-[10px] text-[var(--fg-tertiary)] block mb-0.5 font-medium">Arguments (space-separated)</label>
                        <input
                          value={mcpForm.args}
                          onChange={(e) => setMcpForm({ ...mcpForm, args: e.target.value })}
                          placeholder="-y @modelcontextprotocol/server-filesystem /path"
                          className="w-full bg-[var(--bg-app)] border border-[var(--border-default)] px-2.5 py-1 text-[var(--fg-primary)] focus:outline-none focus:border-[var(--accent-primary)] font-mono text-[11px] rounded"
                        />
                      </div>
                      <div>
                        <label className="text-[10px] text-[var(--fg-tertiary)] block mb-0.5 font-medium">Environment (JSON object, optional)</label>
                        <input
                          value={mcpForm.env}
                          onChange={(e) => setMcpForm({ ...mcpForm, env: e.target.value })}
                          placeholder='{"KEY": "VALUE"}'
                          className="w-full bg-[var(--bg-app)] border border-[var(--border-default)] px-2.5 py-1 text-[var(--fg-primary)] focus:outline-none focus:border-[var(--accent-primary)] font-mono text-[11px] rounded"
                        />
                      </div>
                    </div>
                  ) : (
                    <div>
                      <label className="text-[10px] text-[var(--fg-tertiary)] block mb-0.5 font-medium">Headers (JSON object, optional)</label>
                      <input
                        value={mcpForm.headers}
                        onChange={(e) => setMcpForm({ ...mcpForm, headers: e.target.value })}
                        placeholder='{"x-api-key": "..."}'
                        className="w-full bg-[var(--bg-app)] border border-[var(--border-default)] px-2.5 py-1 text-[var(--fg-primary)] focus:outline-none focus:border-[var(--accent-primary)] font-mono text-[11px] rounded"
                      />
                    </div>
                  )}

                  <div className="flex items-center justify-end gap-1.5 pt-1">
                    <button
                      onClick={() => setShowMcpForm(false)}
                      className="px-2.5 py-1 text-[var(--fg-secondary)] hover:text-white cursor-pointer rounded"
                    >
                      Cancel
                    </button>
                    <button
                      onClick={handleSaveMcpServer}
                      disabled={!mcpForm.name.trim() || (mcpForm.type === "remote" ? !mcpForm.url.trim() : !mcpForm.command.trim())}
                      className="px-3 py-1 bg-[var(--accent-primary)] hover:bg-[var(--accent-hover)] text-black font-semibold rounded cursor-pointer disabled:opacity-50 text-[11px]"
                    >
                      Save Server
                    </button>
                  </div>
                </div>
              )}

              {/* Server List */}
              <div className="space-y-2 max-h-[460px] overflow-y-auto pr-1">
                {mcpServers.map((s) => {
                  const sName = s.name;
                  const toolsForServer = mcpTools.filter((t) => t.server === sName);
                  const isExpanded = !!expandedMcp[sName];

                  return (
                    <div
                      key={sName}
                      className="border border-[var(--border-default)] bg-[var(--bg-panel)] rounded overflow-hidden transition-colors"
                    >
                      {/* Header bar */}
                      <div
                        onClick={() => setExpandedMcp((prev) => ({ ...prev, [sName]: !prev[sName] }))}
                        className="flex items-center justify-between p-2.5 hover:bg-[var(--bg-surface-hover)] cursor-pointer select-none transition-colors"
                      >
                        <div className="flex items-center gap-2 min-w-0">
                          <button
                            type="button"
                            className="text-[var(--fg-tertiary)] hover:text-[var(--fg-primary)]"
                            onClick={(e) => {
                              e.stopPropagation();
                              setExpandedMcp((prev) => ({ ...prev, [sName]: !prev[sName] }));
                            }}
                          >
                            {isExpanded ? (
                              <IconChevronDown className="size-3.5" />
                            ) : (
                              <IconChevronRight className="size-3.5" />
                            )}
                          </button>
                          <IconPlug
                            className={`size-3.5 shrink-0 ${
                              s.connected
                                ? "text-emerald-400"
                                : s.enabled === false
                                ? "text-[var(--fg-tertiary)]"
                                : "text-red-400"
                            }`}
                          />
                          <span className="font-mono font-semibold text-[11.5px] text-[var(--fg-primary)] truncate">
                            {sName}
                          </span>
                          <span className="text-[9px] px-1.5 py-0.2 bg-[var(--bg-surface)] border border-[var(--border-default)] text-[var(--fg-tertiary)] font-mono rounded">
                            {s.source}
                          </span>
                          {s.connected ? (
                            <span className="text-[9px] px-1.5 py-0.2 bg-emerald-500/20 text-emerald-400 font-mono rounded flex items-center gap-1">
                              <span className="size-1.5 rounded-full bg-emerald-400" />
                              connected
                            </span>
                          ) : s.enabled === false ? (
                            <span className="text-[9px] px-1.5 py-0.2 bg-[var(--bg-surface)] text-[var(--fg-tertiary)] font-mono rounded">
                              disabled
                            </span>
                          ) : (
                            <span
                              className="text-[9px] px-1.5 py-0.2 bg-red-500/20 text-red-400 font-mono rounded flex items-center gap-1"
                              title={s.error || "Connection failed"}
                            >
                              <span className="size-1.5 rounded-full bg-red-400" />
                              failed
                            </span>
                          )}
                        </div>

                        <div className="flex items-center gap-2 shrink-0" onClick={(e) => e.stopPropagation()}>
                          {toolsForServer.length > 0 && (
                            <span className="text-[10px] font-mono text-cyan-300 bg-cyan-950/40 border border-cyan-800/50 px-1.5 py-0.5 rounded">
                              {toolsForServer.length} tool{toolsForServer.length > 1 ? "s" : ""}
                            </span>
                          )}
                          {s.source?.startsWith("native:") && (
                            <button
                              onClick={() => handleDeleteMcpServer(sName)}
                              className="p-1 text-[var(--fg-tertiary)] hover:text-red-400 hover:bg-[var(--bg-surface)] rounded cursor-pointer transition-colors"
                              title="Delete owned server"
                            >
                              <IconTrash className="size-3.5" />
                            </button>
                          )}
                        </div>
                      </div>

                      {/* Expanded detail card */}
                      {isExpanded && (
                        <div className="p-3 border-t border-[var(--border-default)] bg-[var(--bg-sidebar)] space-y-2.5 text-xs font-mono">
                          {/* Connection error banner if any */}
                          {s.error && (
                            <div className="p-2 bg-red-950/30 border border-red-800/40 rounded text-red-300 text-[10px] leading-relaxed break-all">
                              <span className="font-semibold block mb-0.5 text-red-400">Connection Error:</span>
                              {s.error}
                            </div>
                          )}

                          {/* Transport & Config */}
                          <div className="space-y-1 text-[10px]">
                            <div className="text-[var(--fg-tertiary)] uppercase font-semibold text-[9px] tracking-wider mb-1">
                              Transport & Details
                            </div>
                            {s.url ? (
                              <div className="space-y-1 bg-[var(--bg-panel)] p-2 rounded border border-[var(--border-default)]">
                                <div className="flex items-center gap-2">
                                  <span className="text-[var(--fg-muted)] w-20">Transport:</span>
                                  <span className="text-cyan-300">Streamable HTTP (remote)</span>
                                </div>
                                <div className="flex items-start gap-2">
                                  <span className="text-[var(--fg-muted)] w-20 shrink-0">Endpoint URL:</span>
                                  <span className="text-[var(--fg-primary)] break-all">{s.url}</span>
                                </div>
                                {s.headers && Object.keys(s.headers).length > 0 && (
                                  <div className="pt-1 border-t border-[var(--border-default)] mt-1">
                                    <span className="text-[var(--fg-muted)] block mb-0.5">Headers:</span>
                                    <div className="space-y-0.5 text-[9px]">
                                      {Object.entries(s.headers).map(([k, v]) => (
                                        <div key={k} className="truncate">
                                          <span className="text-[var(--fg-secondary)]">{k}:</span>{" "}
                                          <span className="text-[var(--fg-muted)]">
                                            {k.toLowerCase().includes("key") || k.toLowerCase().includes("auth")
                                              ? `${String(v).slice(0, 8)}...`
                                              : String(v)}
                                          </span>
                                        </div>
                                      ))}
                                    </div>
                                  </div>
                                )}
                              </div>
                            ) : (
                              <div className="space-y-1 bg-[var(--bg-panel)] p-2 rounded border border-[var(--border-default)]">
                                <div className="flex items-center gap-2">
                                  <span className="text-[var(--fg-muted)] w-20">Transport:</span>
                                  <span className="text-emerald-300">stdio (local subprocess)</span>
                                </div>
                                <div className="flex items-center gap-2">
                                  <span className="text-[var(--fg-muted)] w-20">Command:</span>
                                  <span className="text-[var(--fg-primary)] font-semibold">{s.command || "—"}</span>
                                </div>
                                {s.args && s.args.length > 0 && (
                                  <div className="flex items-start gap-2">
                                    <span className="text-[var(--fg-muted)] w-20 shrink-0">Arguments:</span>
                                    <span className="text-[var(--fg-secondary)] break-all">{s.args.join(" ")}</span>
                                  </div>
                                )}
                                {s.env && Object.keys(s.env).length > 0 && (
                                  <div className="pt-1 border-t border-[var(--border-default)] mt-1">
                                    <span className="text-[var(--fg-muted)] block mb-0.5">Environment Variables:</span>
                                    <div className="space-y-0.5 text-[9px] max-h-20 overflow-y-auto">
                                      {Object.entries(s.env).map(([k, v]) => (
                                        <div key={k} className="truncate">
                                          <span className="text-[var(--fg-secondary)]">{k}=</span>
                                          <span className="text-[var(--fg-muted)]">{String(v)}</span>
                                        </div>
                                      ))}
                                    </div>
                                  </div>
                                )}
                              </div>
                            )}
                          </div>

                          {/* Exposed Tools list */}
                          <div className="space-y-1.5 pt-1">
                            <div className="text-[var(--fg-tertiary)] uppercase font-semibold text-[9px] tracking-wider">
                              Discovered Tools ({toolsForServer.length})
                            </div>
                            {toolsForServer.length === 0 ? (
                              <div className="text-[10px] text-[var(--fg-tertiary)] italic py-1 bg-[var(--bg-panel)] p-2 rounded border border-[var(--border-default)]">
                                {s.connected
                                  ? "No tools exported by this server."
                                  : "Server is not connected. Reconnect to discover tools."}
                              </div>
                            ) : (
                              <div className="space-y-1 max-h-48 overflow-y-auto pr-1">
                                {toolsForServer.map((tool) => (
                                  <div
                                    key={tool.name}
                                    className="p-2 bg-[var(--bg-panel)] border border-[var(--border-default)] rounded space-y-1"
                                  >
                                    <div className="flex items-center gap-1.5">
                                      <span className="text-cyan-300 font-semibold text-[11px]">{tool.name}</span>
                                    </div>
                                    {tool.description && (
                                      <div className="text-[10px] text-[var(--fg-secondary)] leading-relaxed font-sans">
                                        {tool.description}
                                      </div>
                                    )}
                                    {tool.parameters &&
                                      typeof tool.parameters === "object" &&
                                      Object.keys(tool.parameters).length > 0 && (
                                        <details className="text-[9px] text-[var(--fg-tertiary)] cursor-pointer">
                                          <summary className="hover:text-[var(--fg-secondary)] font-mono">
                                            Schema definition
                                          </summary>
                                          <pre className="mt-1 p-1.5 bg-[var(--bg-app)] rounded overflow-x-auto text-[9px] text-[var(--fg-muted)] font-mono">
                                            {JSON.stringify(tool.parameters, null, 2)}
                                          </pre>
                                        </details>
                                      )}
                                  </div>
                                ))}
                              </div>
                            )}
                          </div>
                        </div>
                      )}
                    </div>
                  );
                })}

                {mcpServers.length === 0 && (
                  <div className="py-6 text-center text-[var(--fg-tertiary)] italic text-[11px]">
                    No MCP servers discovered. Click "New Server" above or add them to <code>.mcp.json</code> or <code>~/.config/opencode/opencode.json</code>.
                  </div>
                )}
              </div>
            </div>
          )}
          </div>
        </div>

        {/* Footer */}
        <div className="flex items-center justify-between pt-2 border-t border-[var(--border-default)] shrink-0 px-8 pb-2">
          <span className="text-[10px] font-mono text-[var(--fg-tertiary)]">ForgeADE v{APP_VERSION}</span>
          <button
            onClick={onClose}
            className="px-4 py-1.5 bg-[var(--accent-primary)] hover:bg-[var(--accent-hover)] text-white text-xs font-semibold cursor-pointer"
          >
            Done
          </button>
        </div>
      </div>
    </div>
  );
}
