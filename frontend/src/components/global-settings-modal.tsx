import React, { useState, useEffect, useCallback } from "react";
import { useShortcutsStore } from "../hooks/store";
import { useWorkspace } from "../stores/workspaceStore";
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
  IconCompass,
  IconDownload,
  IconSun,
  IconMoon,
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
  ListLLMProviders,
  DiscoverMCPServers,
  ImportDiscoveredMCPServers,
  DiscoverSkills,
  ImportDiscoveredSkills,
} from "../lib/wails";

import { ThemeMode } from "../types";

interface GlobalSettingsModalProps {
  open: boolean;
  onClose: () => void;
}

type Tab = "shortcuts" | "appearance" | "providers" | "agents" | "mcp" | "discover" | "ai-commit";

const DEFAULT_ROLES = ["coding", "planning", "research", "custom"];

// Theme options — Dark and Light mode only.
const THEME_OPTIONS: { value: ThemeMode; label: string; desc: string }[] = [
  { value: "dark", label: "Dark Mode", desc: "Minimalist dark palette with high contrast" },
  { value: "light", label: "Light Mode", desc: "Crisp, clean bright appearance" },
];

export function GlobalSettingsModal({ open, onClose }: GlobalSettingsModalProps) {
  const { keybindings, setKeybindings } = useShortcutsStore();
  const { theme, setTheme } = useWorkspace();
  const { toast } = useToast();
  const [activeTab, setActiveTab] = useState<Tab>("shortcuts");
  const [editingId, setEditingId] = useState<string | null>(null);
  const [recordingKeys, setRecordingKeys] = useState<string[]>([]);

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
  const [mcpServers, setMcpServers] = useState<any[]>([]);
  const [showMcpForm, setShowMcpForm] = useState(false);
  const [mcpForm, setMcpForm] = useState({ name: "", command: "", args: "", url: "" });

  // Multi-source Discovery state (MCPs & Skills across other agent tools)
  const [discoveredMCPs, setDiscoveredMCPs] = useState<any[]>([]);
  const [discoveredSkills, setDiscoveredSkills] = useState<any[]>([]);
  const [discoveringMCPs, setDiscoveringMCPs] = useState(false);
  const [discoveringSkills, setDiscoveringSkills] = useState(false);
  const [importingMCPs, setImportingMCPs] = useState<Record<string, boolean>>({});
  const [importingSkills, setImportingSkills] = useState<Record<string, boolean>>({});
  const [discoverSubTab, setDiscoverSubTab] = useState<"all" | "mcp" | "skills">("all");

  // AI Commit generator config (provider + model + prompt), persisted in localStorage
  const DEFAULT_COMMIT_PROMPT =
    "CRITICAL: You are a Git commit message generator. Your output MUST be ONLY a concise 1 to 2 line Git commit message following conventional commits format (e.g., 'docs(readme): rewrite architecture guide and update tech stack'). DO NOT include any analysis, section headings, Markdown tables, or explanations. ONLY output the raw commit message text.";
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
      const list = await ListMCPServers();
      setMcpServers(Array.isArray(list) ? list : []);
    } catch { /* ignore */ }
  }, []);

  const loadDiscovered = useCallback(async () => {
    setDiscoveringMCPs(true);
    setDiscoveringSkills(true);
    try {
      const mcps = await DiscoverMCPServers();
      setDiscoveredMCPs(Array.isArray(mcps) ? mcps : []);
    } catch {
      setDiscoveredMCPs([]);
    } finally {
      setDiscoveringMCPs(false);
    }
    try {
      const sks = await DiscoverSkills();
      setDiscoveredSkills(Array.isArray(sks) ? sks : []);
    } catch {
      setDiscoveredSkills([]);
    } finally {
      setDiscoveringSkills(false);
    }
  }, []);

  useEffect(() => {
    if (!open) return;
    loadProfiles();
    loadAgentDefs();
    loadMcpServers();
    loadCommitConfig();
    loadDiscovered();
  }, [open, loadProfiles, loadAgentDefs, loadMcpServers, loadCommitConfig, loadDiscovered]);

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
    const model = (provider.selected_models || provider.SelectedModels || [])[0];
    try {
      await SaveLLMProfile(pid, provider.api_key || provider.ApiKey || "", provider.base_url || provider.BaseURL || "", model || "");
      if (model) await SetActiveModel(pid, model);
    } catch { /* ignore */ }
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
    if (!mcpForm.name.trim()) return;
    const server = {
      name: mcpForm.name.trim(),
      command: mcpForm.command.trim(),
      args: mcpForm.args.trim() ? mcpForm.args.trim().split(/\s+/) : [],
      url: mcpForm.url.trim() || undefined,
      type: mcpForm.url.trim() ? "remote" : "local",
      enabled: true,
    };
    try {
      await SaveMCPServer(server);
      await loadMcpServers();
      setShowMcpForm(false);
      setMcpForm({ name: "", command: "", args: "", url: "" });
      toast("MCP server saved", "success");
    } catch (err: any) {
      toast("Failed to save MCP server: " + err, "danger");
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

  async function handleImportMCPServer(name: string) {
    setImportingMCPs((prev) => ({ ...prev, [name]: true }));
    try {
      await ImportDiscoveredMCPServers([name]);
      toast(`Imported MCP server: ${name}`, "success");
      await loadMcpServers();
      await loadDiscovered();
    } catch (err: any) {
      toast(`Failed to import MCP: ${err}`, "danger");
    } finally {
      setImportingMCPs((prev) => ({ ...prev, [name]: false }));
    }
  }

  async function handleImportSkill(name: string) {
    setImportingSkills((prev) => ({ ...prev, [name]: true }));
    try {
      await ImportDiscoveredSkills([name]);
      toast(`Imported skill: ${name}`, "success");
      await loadDiscovered();
    } catch (err: any) {
      toast(`Failed to import skill: ${err}`, "danger");
    } finally {
      setImportingSkills((prev) => ({ ...prev, [name]: false }));
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
      <div className="bg-[var(--bg-sidebar)] border border-[var(--border-default)] w-full max-w-2xl shadow-2xl flex flex-col h-[560px]">
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
            {tabBtn("mcp", <IconPlug className="size-3" />, "MCP")}
            {tabBtn("discover", <IconCompass className="size-3 text-cyan-400" />, "Discover")}
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
            <div className="space-y-4 text-xs">
              <div>
                <label className="text-[var(--fg-primary)] font-semibold block mb-1">Color Theme</label>
                <p className="text-[var(--fg-secondary)] text-[11px] mb-3">
                  Choose your preferred appearance mode.
                </p>
              </div>

              <div className="grid grid-cols-2 gap-3">
                {THEME_OPTIONS.map((t) => {
                  const isSelected = theme === t.value;
                  const Icon = t.value === "dark" ? IconMoon : IconSun;
                  return (
                    <button
                      key={t.value}
                      type="button"
                      onClick={() => setTheme(t.value)}
                      className={`p-4 rounded-xl border text-left transition-all cursor-pointer flex flex-col justify-between gap-3 ${
                        isSelected
                          ? "border-[var(--accent-primary)] bg-[var(--accent-primary)]/10 shadow-xs ring-1 ring-[var(--accent-primary)]"
                          : "border-[var(--border-default)] bg-[var(--bg-panel)] hover:border-[var(--fg-tertiary)]"
                      }`}
                    >
                      <div className="flex items-center justify-between">
                        <div className={`p-2 rounded-lg ${isSelected ? "bg-[var(--accent-primary)] text-white" : "bg-[var(--bg-surface)] text-[var(--fg-secondary)]"}`}>
                          <Icon className="w-5 h-5" />
                        </div>
                        {isSelected && (
                          <span className="text-[10px] font-semibold text-[var(--accent-primary)] px-2 py-0.5 rounded-full bg-[var(--accent-primary)]/20">
                            Active
                          </span>
                        )}
                      </div>
                      <div>
                        <div className="font-semibold text-sm text-[var(--fg-primary)]">{t.label}</div>
                        <div className="text-[11px] text-[var(--fg-secondary)] mt-0.5 leading-relaxed">{t.desc}</div>
                      </div>
                    </button>
                  );
                })}
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
                  <div className="max-h-32 overflow-y-auto border border-[var(--border-default)] p-1 space-y-0.5">
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
                      <div className="border-t border-[var(--border-default)] p-2 space-y-1">
                        {models.length === 0 && (
                          <div className="text-[10px] text-[var(--fg-tertiary)] italic">No models fetched</div>
                        )}
                        {models.map((m: string) => (
                          <label key={m} className="flex items-center gap-1.5 px-1 py-0.5 hover:bg-[var(--bg-surface-hover)] cursor-pointer rounded">
                            <input
                              type="checkbox"
                              checked={selected.includes(m)}
                              onChange={() => handleToggleModel(p, m)}
                            />
                            <span className="font-mono text-[11px]">{m}</span>
                          </label>
                        ))}
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
          ) : activeTab === "mcp" ? (
            <div className="space-y-3 text-xs">
              <div className="flex items-center justify-between">
                <span className="text-[var(--fg-secondary)] font-semibold">MCP Servers</span>
                <button
                  onClick={() => setShowMcpForm(!showMcpForm)}
                  className="px-2 py-1 bg-[var(--bg-surface-hover)] border border-[var(--border-default)] hover:bg-[var(--bg-surface-active)] text-[var(--fg-primary)] rounded cursor-pointer flex items-center gap-1"
                >
                  <IconPlus className="size-3" />
                  New Server
                </button>
              </div>

              {showMcpForm && (
                <div className="border border-[var(--border-default)] p-2 space-y-2 bg-[var(--bg-panel)]">
                  <input
                    value={mcpForm.name}
                    onChange={(e) => setMcpForm({ ...mcpForm, name: e.target.value })}
                    placeholder="Server name (e.g. filesystem)"
                    className="w-full bg-[var(--bg-app)] border border-[var(--border-default)] px-2 py-1 text-[var(--fg-primary)] focus:outline-none focus:border-[var(--accent-primary)] font-mono text-[11px]"
                  />
                  <input
                    value={mcpForm.command}
                    onChange={(e) => setMcpForm({ ...mcpForm, command: e.target.value })}
                    placeholder="Command (e.g. npx)"
                    className="w-full bg-[var(--bg-app)] border border-[var(--border-default)] px-2 py-1 text-[var(--fg-primary)] focus:outline-none focus:border-[var(--accent-primary)] font-mono text-[11px]"
                  />
                  <input
                    value={mcpForm.args}
                    onChange={(e) => setMcpForm({ ...mcpForm, args: e.target.value })}
                    placeholder="Args (space-separated)"
                    className="w-full bg-[var(--bg-app)] border border-[var(--border-default)] px-2 py-1 text-[var(--fg-primary)] focus:outline-none focus:border-[var(--accent-primary)] font-mono text-[11px]"
                  />
                  <input
                    value={mcpForm.url}
                    onChange={(e) => setMcpForm({ ...mcpForm, url: e.target.value })}
                    placeholder="Remote URL (optional, for remote servers)"
                    className="w-full bg-[var(--bg-app)] border border-[var(--border-default)] px-2 py-1 text-[var(--fg-primary)] focus:outline-none focus:border-[var(--accent-primary)] font-mono text-[11px]"
                  />
                  <div className="flex items-center justify-end gap-1.5">
                    <button
                      onClick={() => setShowMcpForm(false)}
                      className="px-2 py-1 text-[var(--fg-secondary)] hover:text-white cursor-pointer"
                    >
                      Cancel
                    </button>
                    <button
                      onClick={handleSaveMcpServer}
                      disabled={!mcpForm.name.trim()}
                      className="px-3 py-1 bg-[var(--accent-primary)] hover:bg-[var(--accent-hover)] text-black font-semibold rounded cursor-pointer disabled:opacity-50"
                    >
                      Save Server
                    </button>
                  </div>
                </div>
              )}

              {mcpServers.length === 0 && (
                <div className="text-[10px] text-[var(--fg-tertiary)] italic">No MCP servers configured</div>
              )}
              {mcpServers.map((s) => (
                <div key={s.name || s.Name} className="flex items-center justify-between px-2 py-1.5 border border-[var(--border-default)] bg-[var(--bg-panel)]">
                  <div className="flex items-center gap-1.5 min-w-0">
                    <IconPlug className="size-3.5 text-green-400 shrink-0" />
                    <div className="min-w-0">
                      <div className="font-semibold truncate">{s.name || s.Name}</div>
                      <div className="text-[10px] text-[var(--fg-tertiary)] font-mono truncate">
                        {s.command || s.Command || ""} {s.type === "remote" ? `(${s.url})` : ""}
                      </div>
                    </div>
                  </div>
                  <button
                    onClick={() => handleDeleteMcpServer(s.name || s.Name)}
                    className="p-1 hover:bg-[var(--bg-surface-hover)] rounded text-red-500 cursor-pointer shrink-0"
                  >
                    <IconTrash className="size-3.5" />
                  </button>
                </div>
              ))}
            </div>
          ) : activeTab === "discover" ? (
            <div className="space-y-3 text-xs">
              <div className="flex items-center justify-between">
                <div>
                  <span className="text-[var(--fg-secondary)] font-semibold block">Discovered Agent Skills & MCPs</span>
                  <span className="text-[10px] text-[var(--fg-tertiary)]">
                    Auto-scanned from Claude, Codex, Cursor, Opencode, Pi, Antigravity, and global configs.
                  </span>
                </div>
                <button
                  onClick={loadDiscovered}
                  disabled={discoveringMCPs || discoveringSkills}
                  className="px-2 py-1 bg-[var(--bg-surface-hover)] border border-[var(--border-default)] hover:bg-[var(--bg-surface-active)] text-[var(--fg-primary)] rounded cursor-pointer flex items-center gap-1 text-[11px] disabled:opacity-50"
                >
                  <IconRefresh className={discoveringMCPs || discoveringSkills ? "size-3 animate-spin" : "size-3"} />
                  Rescan
                </button>
              </div>

              {/* Filter subtabs */}
              <div className="flex items-center gap-1 border-b border-[var(--border-default)] pb-1.5 pt-1">
                <button
                  onClick={() => setDiscoverSubTab("all")}
                  className={`px-2 py-0.5 rounded text-[11px] font-medium transition-colors cursor-pointer ${
                    discoverSubTab === "all"
                      ? "bg-[var(--accent-primary)] text-black"
                      : "text-[var(--fg-secondary)] hover:text-white"
                  }`}
                >
                  All ({discoveredMCPs.length + discoveredSkills.length})
                </button>
                <button
                  onClick={() => setDiscoverSubTab("mcp")}
                  className={`px-2 py-0.5 rounded text-[11px] font-medium transition-colors cursor-pointer ${
                    discoverSubTab === "mcp"
                      ? "bg-[var(--accent-primary)] text-black"
                      : "text-[var(--fg-secondary)] hover:text-white"
                  }`}
                >
                  MCP Servers ({discoveredMCPs.length})
                </button>
                <button
                  onClick={() => setDiscoverSubTab("skills")}
                  className={`px-2 py-0.5 rounded text-[11px] font-medium transition-colors cursor-pointer ${
                    discoverSubTab === "skills"
                      ? "bg-[var(--accent-primary)] text-black"
                      : "text-[var(--fg-secondary)] hover:text-white"
                  }`}
                >
                  Skills ({discoveredSkills.length})
                </button>
              </div>

              {/* MCP Discovery Section */}
              {(discoverSubTab === "all" || discoverSubTab === "mcp") && (
                <div className="space-y-1.5">
                  <div className="text-[10px] uppercase font-bold tracking-wider text-[var(--fg-tertiary)] flex items-center gap-1.5 pt-1">
                    <IconPlug className="size-3 text-green-400" />
                    <span>Discovered MCP Servers ({discoveredMCPs.length})</span>
                  </div>

                  {discoveredMCPs.length === 0 ? (
                    <div className="text-[11px] text-[var(--fg-tertiary)] italic p-2 border border-[var(--border-default)] bg-[var(--bg-panel)]">
                      {discoveringMCPs ? "Scanning for MCP configs..." : "No external MCP servers detected."}
                    </div>
                  ) : (
                    discoveredMCPs.map((m) => (
                      <div
                        key={`${m.origin}-${m.name}`}
                        className="flex items-center justify-between p-2 border border-[var(--border-default)] bg-[var(--bg-panel)] hover:border-[var(--border-focus)] transition-colors"
                      >
                        <div className="min-w-0 pr-2">
                          <div className="flex items-center gap-1.5">
                            <span className="font-semibold text-[var(--fg-primary)] truncate">{m.name}</span>
                            <span className="text-[9px] px-1.5 py-px rounded bg-blue-500/10 text-blue-400 border border-blue-500/30 font-mono">
                              {m.originLabel || m.origin}
                            </span>
                            {m.imported && (
                              <span className="text-[9px] px-1.5 py-px rounded bg-green-500/10 text-green-400 border border-green-500/30 font-mono">
                                Imported
                              </span>
                            )}
                          </div>
                          <div className="text-[10px] text-[var(--fg-tertiary)] font-mono truncate mt-0.5">
                            {m.command || m.url} {Array.isArray(m.args) ? m.args.join(" ") : ""}
                          </div>
                        </div>

                        <button
                          onClick={() => handleImportMCPServer(m.name)}
                          disabled={m.imported || importingMCPs[m.name]}
                          className="px-2.5 py-1 bg-[var(--bg-surface-hover)] border border-[var(--border-default)] hover:bg-[var(--accent-primary)] hover:text-black rounded text-[11px] font-semibold text-[var(--fg-primary)] cursor-pointer disabled:opacity-40 disabled:hover:bg-[var(--bg-surface-hover)] disabled:hover:text-[var(--fg-primary)] shrink-0 flex items-center gap-1"
                        >
                          {importingMCPs[m.name] ? (
                            <IconRefresh className="size-3 animate-spin" />
                          ) : m.imported ? (
                            <IconCheck className="size-3 text-green-400" />
                          ) : (
                            <IconDownload className="size-3 text-cyan-400" />
                          )}
                          <span>{m.imported ? "Installed" : "Import"}</span>
                        </button>
                      </div>
                    ))
                  )}
                </div>
              )}

              {/* Skills Discovery Section */}
              {(discoverSubTab === "all" || discoverSubTab === "skills") && (
                <div className="space-y-1.5 pt-2">
                  <div className="text-[10px] uppercase font-bold tracking-wider text-[var(--fg-tertiary)] flex items-center gap-1.5">
                    <IconSparkles className="size-3 text-purple-400" />
                    <span>Discovered Agent Skills ({discoveredSkills.length})</span>
                  </div>

                  {discoveredSkills.length === 0 ? (
                    <div className="text-[11px] text-[var(--fg-tertiary)] italic p-2 border border-[var(--border-default)] bg-[var(--bg-panel)]">
                      {discoveringSkills ? "Scanning for agent skills..." : "No external agent skills detected."}
                    </div>
                  ) : (
                    discoveredSkills.map((s) => (
                      <div
                        key={`${s.origin}-${s.name}`}
                        className="flex items-center justify-between p-2 border border-[var(--border-default)] bg-[var(--bg-panel)] hover:border-[var(--border-focus)] transition-colors"
                      >
                        <div className="min-w-0 pr-2">
                          <div className="flex items-center gap-1.5">
                            <span className="font-semibold text-[var(--fg-primary)] truncate">{s.name}</span>
                            <span className="text-[9px] px-1.5 py-px rounded bg-purple-500/10 text-purple-400 border border-purple-500/30 font-mono">
                              {s.originLabel || s.origin}
                            </span>
                            {s.imported && (
                              <span className="text-[9px] px-1.5 py-px rounded bg-green-500/10 text-green-400 border border-green-500/30 font-mono">
                                Imported
                              </span>
                            )}
                          </div>
                          {s.description && (
                            <div className="text-[10px] text-[var(--fg-secondary)] truncate mt-0.5">
                              {s.description}
                            </div>
                          )}
                          {s.path && (
                            <div className="text-[9px] text-[var(--fg-tertiary)] font-mono truncate mt-0.5">
                              {s.path}
                            </div>
                          )}
                        </div>

                        <button
                          onClick={() => handleImportSkill(s.name)}
                          disabled={s.imported || importingSkills[s.name]}
                          className="px-2.5 py-1 bg-[var(--bg-surface-hover)] border border-[var(--border-default)] hover:bg-[var(--accent-primary)] hover:text-black rounded text-[11px] font-semibold text-[var(--fg-primary)] cursor-pointer disabled:opacity-40 disabled:hover:bg-[var(--bg-surface-hover)] disabled:hover:text-[var(--fg-primary)] shrink-0 flex items-center gap-1"
                        >
                          {importingSkills[s.name] ? (
                            <IconRefresh className="size-3 animate-spin" />
                          ) : s.imported ? (
                            <IconCheck className="size-3 text-green-400" />
                          ) : (
                            <IconDownload className="size-3 text-purple-400" />
                          )}
                          <span>{s.imported ? "Installed" : "Import"}</span>
                        </button>
                      </div>
                    ))
                  )}
                </div>
              )}
            </div>
          ) : null}
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
