import React, { useState, useEffect, useCallback } from "react";
import { useShortcutsStore, useUIStore } from "../hooks/store";
import { useToast } from "../lib/toast";
import { APP_VERSION } from "../lib/utils";
import {
  IconX,
  IconSettings,
  IconKeyboard,
  IconPalette,
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
  ListLLMProviders,
  ListAllSkills,
  SetSkillEnabled,
  SetAllSkillsEnabled,
  type SkillInfo,
  type MCPServerInfo,
  type MCPToolInfo,
} from "../lib/native";

interface GlobalSettingsModalProps {
  open: boolean;
  onClose: () => void;
}

type Tab = "shortcuts" | "appearance" | "providers" | "agents" | "skills" | "mcp" | "ai-commit";

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

export function GlobalSettingsModal({ open, onClose }: GlobalSettingsModalProps) {
  const { keybindings, setKeybindings } = useShortcutsStore();
  const { theme, setTheme } = useUIStore();
  const { toast } = useToast();
  const [activeTab, setActiveTab] = useState<Tab>("shortcuts");
  const [editingId, setEditingId] = useState<string | null>(null);
  const [recordingKeys, setRecordingKeys] = useState<string[]>([]);

  // Skills state
  const [skills, setSkills] = useState<SkillInfo[]>([]);
  const [skillsSearch, setSkillsSearch] = useState("");

  const loadSkills = useCallback(async () => {
    try {
      const list = await ListAllSkills();
      setSkills(Array.isArray(list) ? list : []);
    } catch { /* ignore */ }
  }, []);

  // Providers state
  const [profiles, setProfiles] = useState<any[]>([]);
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
    try {
      const models = await FetchProviderModels(p.api_key || p.ApiKey || "", p.base_url || p.BaseURL || "https://api.openai.com/v1");
      if (!Array.isArray(models)) return;
      const next = profiles.map((x) => {
        if ((x.id || x.Id) !== providerId) return x;
        return { ...x, available_models: models, selected_models: (x.selected_models || x.SelectedModels || []).filter((m: string) => models.includes(m)) };
      });
      setProfiles(next);
      await SaveProviderProfiles(next);
      toast(`Models refreshed: ${models.length}`, "success");
    } catch (err: any) {
      toast("Refresh models failed: " + err, "danger");
    }
  }

  async function handleToggleModel(provider: any, model: string) {
    const pid = provider.id || provider.Id;
    const selected = provider.selected_models || provider.SelectedModels || [];
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
            {tabBtn("providers", <IconCpu className="size-3" />, "Providers")}
            {tabBtn("agents", <IconRobot className="size-3" />, "Agents")}
            {tabBtn("skills", <IconSparkles className="size-3 text-amber-400" />, "Skills")}
            {tabBtn("mcp", <IconPlug className="size-3" />, "MCP")}
            {tabBtn("ai-commit", <IconSparkles className="size-3" />, "AI Commit")}
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
          ) : activeTab === "providers" ? (
            <div className="space-y-3 text-xs">
              <div className="flex items-center justify-between">
                <span className="text-[var(--fg-secondary)] font-semibold">Provider Profiles</span>
              </div>

              {/* New provider form */}
              <div className="border border-[var(--border-default)] p-2 space-y-2 bg-[var(--bg-panel)]">
                <div className="grid grid-cols-2 gap-2">
                  <input
                    value={newProvider.name}
                    onChange={(e) => setNewProvider({ ...newProvider, name: e.target.value })}
                    placeholder="Provider name (e.g. my-llm)"
                    className="w-full bg-[var(--bg-app)] border border-[var(--border-default)] px-2 py-1 text-[var(--fg-primary)] focus:outline-none focus:border-[var(--accent-primary)] font-mono text-[11px]"
                  />
                  <input
                    value={newProvider.baseUrl}
                    onChange={(e) => setNewProvider({ ...newProvider, baseUrl: e.target.value })}
                    placeholder="Base URL (default OpenAI)"
                    className="w-full bg-[var(--bg-app)] border border-[var(--border-default)] px-2 py-1 text-[var(--fg-primary)] focus:outline-none focus:border-[var(--accent-primary)] font-mono text-[11px]"
                  />
                </div>
                <div className="flex items-center gap-2">
                  <input
                    type="password"
                    value={newProvider.apiKey}
                    onChange={(e) => setNewProvider({ ...newProvider, apiKey: e.target.value })}
                    placeholder="API Key"
                    className="flex-1 bg-[var(--bg-app)] border border-[var(--border-default)] px-2 py-1 text-[var(--fg-primary)] focus:outline-none focus:border-[var(--accent-primary)] font-mono text-[11px]"
                  />
                  <button
                    onClick={handleFetchModels}
                    disabled={fetchingModels || !newProvider.apiKey}
                    className="px-2 py-1 bg-[var(--bg-surface-hover)] border border-[var(--border-default)] hover:bg-[var(--bg-surface-active)] text-[var(--fg-primary)] rounded cursor-pointer disabled:opacity-50 flex items-center gap-1"
                  >
                    <IconRefresh className={fetchingModels ? "size-3 animate-spin" : "size-3"} />
                    Fetch Models
                  </button>
                </div>
                {fetchedModels.length > 0 && (
                  <div className="max-h-56 overflow-y-auto border border-[var(--border-default)] p-1 space-y-0.5">
                    {fetchedModels.map((m) => (
                      <label key={m} className="flex items-center gap-1.5 px-1 py-0.5 hover:bg-[var(--bg-surface-hover)] cursor-pointer">
                        <input
                          type="checkbox"
                          checked={(newProvider as any)._selectedModels?.includes(m) || false}
                          onChange={() => {
                            const current = (newProvider as any)._selectedModels || [];
                            const next = current.includes(m)
                              ? current.filter((x: string) => x !== m)
                              : [...current, m];
                            setNewProvider({ ...newProvider, _selectedModels: next } as any);
                          }}
                        />
                        <span className="font-mono text-[11px]">{m}</span>
                      </label>
                    ))}
                  </div>
                )}
                <div className="flex items-center justify-end gap-1.5">
                  <button
                    onClick={() => {
                      const current = (newProvider as any)._selectedModels || [];
                      const next = current.length === fetchedModels.length && current.length > 0
                        ? []
                        : fetchedModels.slice();
                      setNewProvider({ ...newProvider, _selectedModels: next } as any);
                    }}
                    className="px-2 py-1 text-[10px] text-[var(--fg-secondary)] hover:text-white cursor-pointer"
                  >
                    {((newProvider as any)._selectedModels?.length ?? 0) === fetchedModels.length && fetchedModels.length > 0
                      ? "Clear all"
                      : "Select all"}
                  </button>
                  <button
                    onClick={handleAddProvider}
                    disabled={!newProvider.name.trim()}
                    className="px-3 py-1 bg-[var(--accent-primary)] hover:bg-[var(--accent-hover)] text-black font-semibold rounded cursor-pointer disabled:opacity-50"
                  >
                    Add Provider
                  </button>
                </div>
              </div>

              {/* Provider list */}
              {profiles.map((p) => {
                const pid = p.id || p.Id;
                const models = p.available_models || p.AvailableModels || [];
                const selected = p.selected_models || p.SelectedModels || [];
                const expanded = !!expandedProviders[pid];
                return (
                  <div key={pid} className="border border-[var(--border-default)] bg-[var(--bg-panel)]">
                    <div className="flex items-center justify-between px-2 py-1.5">
                      <div className="flex items-center gap-1.5 min-w-0">
                        <button
                          onClick={() => setExpandedProviders((prev) => ({ ...prev, [pid]: !prev[pid] }))}
                          className="text-[var(--fg-tertiary)] hover:text-white cursor-pointer"
                        >
                          {expanded ? <IconChevronDown className="size-3.5" /> : <IconChevronRight className="size-3.5" />}
                        </button>
                        <IconCpu className="size-3.5 text-purple-400 shrink-0" />
                        <span className="font-semibold truncate">{p.name || p.Name}</span>
                        <span className="text-[10px] text-[var(--fg-tertiary)] font-mono">{p.base_url || p.BaseURL || ""}</span>
                      </div>
                      <div className="flex items-center gap-1">
                        <button
                          onClick={() => handleSaveLLMProfile(p)}
                          className="p-1 hover:bg-[var(--bg-surface-hover)] rounded text-green-500 cursor-pointer"
                          title="Set as active"
                        >
                          <IconCheck className="size-3.5" />
                        </button>
                        <button
                          onClick={() => handleRefreshProviderModels(pid)}
                          className="p-1 hover:bg-[var(--bg-surface-hover)] rounded text-blue-500 cursor-pointer"
                          title="Refresh models from provider"
                        >
                          <IconRefresh className="size-3.5" />
                        </button>
                        <button
                          onClick={() => {
                            const next = profiles.filter((x) => (x.id || x.Id) !== pid);
                            setProfiles(next);
                            SaveProviderProfiles(next).then(() => toast("Provider removed", "success")).catch(() => toast("Failed to remove provider", "danger"));
                          }}
                          className="p-1 hover:bg-[var(--bg-surface-hover)] rounded text-red-500 cursor-pointer"
                          title="Remove provider"
                        >
                          <IconTrash className="size-3.5" />
                        </button>
                      </div>
                    </div>
                    {expanded && (
                      <div className="border-t border-[var(--border-default)] p-2.5 space-y-2 bg-[var(--bg-app)]">
                        <div className="grid grid-cols-2 gap-2">
                          <div>
                            <label className="text-[10px] text-[var(--fg-tertiary)] block mb-0.5 font-medium">Base URL</label>
                            <input
                              value={p.base_url || p.baseURL || ""}
                              onChange={(e) => {
                                const next = profiles.map((x) => {
                                  if ((x.id || x.Id) === pid) {
                                    return { ...x, base_url: e.target.value, baseURL: e.target.value };
                                  }
                                  return x;
                                });
                                setProfiles(next);
                              }}
                              className="w-full bg-[var(--bg-panel)] border border-[var(--border-default)] px-2 py-1 text-[var(--fg-primary)] focus:outline-none focus:border-[var(--accent-primary)] font-mono text-[11px] rounded"
                            />
                          </div>
                          <div>
                            <label className="text-[10px] text-[var(--fg-tertiary)] block mb-0.5 font-medium">API Key</label>
                            <input
                              type="password"
                              value={p.api_key || p.apiKey || ""}
                              onChange={(e) => {
                                const next = profiles.map((x) => {
                                  if ((x.id || x.Id) === pid) {
                                    return { ...x, api_key: e.target.value, apiKey: e.target.value };
                                  }
                                  return x;
                                });
                                setProfiles(next);
                              }}
                              placeholder="sk-..."
                              className="w-full bg-[var(--bg-panel)] border border-[var(--border-default)] px-2 py-1 text-[var(--fg-primary)] focus:outline-none focus:border-[var(--accent-primary)] font-mono text-[11px] rounded"
                            />
                          </div>
                        </div>
                        <div className="flex items-center justify-between pt-1">
                          <span className="text-[10px] text-[var(--fg-tertiary)] font-semibold uppercase tracking-wider">Models ({models.length})</span>
                          <div className="flex items-center gap-1.5">
                            <button
                              onClick={() => handleRefreshProviderModels(pid)}
                              className="px-2 py-0.5 text-[10px] bg-[var(--bg-panel)] border border-[var(--border-default)] rounded hover:bg-[var(--bg-surface-hover)] text-[var(--fg-secondary)] flex items-center gap-1 cursor-pointer"
                            >
                              <IconRefresh className="size-3" />
                              Fetch from API
                            </button>
                            <button
                              onClick={async () => {
                                try {
                                  await SaveProviderProfiles(profiles);
                                  const model = p.activeModel || selected[0] || models[0] || "";
                                  if (model) await SetActiveModel(pid, model);
                                  toast("Provider settings saved to disk", "success");
                                } catch (err: any) {
                                  toast("Save failed: " + err, "danger");
                                }
                              }}
                              className="px-2.5 py-0.5 text-[10px] bg-[var(--accent-primary)] hover:bg-[var(--accent-hover)] text-black font-semibold rounded cursor-pointer"
                            >
                              Save Changes
                            </button>
                          </div>
                        </div>
                        {models.length === 0 && (
                          <div className="text-[10px] text-[var(--fg-tertiary)] italic py-1">No models fetched yet. Click "Fetch from API" above or select models.</div>
                        )}
                        {models.length > 0 && (
                          <div className="max-h-60 overflow-y-auto border border-[var(--border-default)] p-1 space-y-0.5 bg-[var(--bg-panel)] rounded">
                            {models.map((m: string) => (
                              <label key={m} className="flex items-center gap-1.5 px-1 py-0.5 hover:bg-[var(--bg-surface-hover)] cursor-pointer rounded">
                                <input
                                  type="checkbox"
                                  checked={selected.includes(m)}
                                  onChange={() => handleToggleModel(p, m)}
                                />
                                <span className="font-mono text-[11px] flex-1 truncate">{m}</span>
                                {p.activeModel === m && (
                                  <span className="text-[9px] px-1 py-0.2 bg-emerald-500/20 text-emerald-400 rounded font-mono">active</span>
                                )}
                              </label>
                            ))}
                          </div>
                        )}
                      </div>
                    )}
                  </div>
                );
              })}
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
