import React, { useState, useEffect, useCallback } from "react";
import { useShortcutsStore, useUIStore } from "../hooks/store";
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
} from "../lib/wails";

interface GlobalSettingsModalProps {
  open: boolean;
  onClose: () => void;
}

type Tab = "shortcuts" | "appearance" | "providers" | "agents" | "mcp";

const DEFAULT_ROLES = ["coding", "planning", "research", "custom"];

export function GlobalSettingsModal({ open, onClose }: GlobalSettingsModalProps) {
  const { keybindings, setKeybindings } = useShortcutsStore();
  const { theme, setTheme } = useUIStore();
  const [activeTab, setActiveTab] = useState<Tab>("shortcuts");
  const [editingId, setEditingId] = useState<string | null>(null);
  const [recordingKeys, setRecordingKeys] = useState<string[]>([]);

  // Providers state
  const [profiles, setProfiles] = useState<any[]>([]);
  const [newProvider, setNewProvider] = useState({ name: "", apiKey: "", baseUrl: "" });
  const [fetchingModels, setFetchingModels] = useState(false);
  const [fetchedModels, setFetchedModels] = useState<string[]>([]);

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

  // Expanded provider rows
  const [expandedProviders, setExpandedProviders] = useState<Record<string, boolean>>({});

  const loadProfiles = useCallback(async () => {
    try {
      const list = await GetProviderProfiles();
      setProfiles(Array.isArray(list) ? list : []);
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

  useEffect(() => {
    if (!open) return;
    loadProfiles();
    loadAgentDefs();
    loadMcpServers();
  }, [open, loadProfiles, loadAgentDefs, loadMcpServers]);

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
    const selected = (newProvider as any)._selectedModels || fetchedModels;
    const prof = {
      id: name.toLowerCase().replace(/[^a-z0-9]+/g, "-"),
      name,
      api_key: newProvider.apiKey.trim(),
      base_url: newProvider.baseUrl.trim() || "https://api.openai.com/v1",
      enabled: true,
      available_models: fetchedModels,
      selected_models: selected,
    };
    const next = [...profiles, prof];
    setProfiles(next);
    try {
      await SaveProviderProfiles(next);
    } catch { /* ignore */ }
    setNewProvider({ name: "", apiKey: "", baseUrl: "" });
    setFetchedModels([]);
  }

  async function handleFetchModels() {
    setFetchingModels(true);
    try {
      const models = await FetchProviderModels(newProvider.apiKey.trim(), newProvider.baseUrl.trim() || "https://api.openai.com/v1");
      setFetchedModels(Array.isArray(models) ? models : []);
    } catch (err: any) {
      alert("Fetch models failed: " + err);
    } finally {
      setFetchingModels(false);
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
    } catch { /* ignore */ }
  }

  async function handleDeleteAgentDef(id: string) {
    try {
      await DeleteAgentDefinition(id);
      await loadAgentDefs();
    } catch { /* ignore */ }
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
    } catch { /* ignore */ }
  }

  async function handleDeleteMcpServer(name: string) {
    try {
      await DeleteMCPServer(name);
      await loadMcpServers();
    } catch { /* ignore */ }
  }

  if (!open) return null;

  const tabBtn = (t: Tab, icon: React.ReactNode, label: string) => (
    <button
      onClick={() => setActiveTab(t)}
      className={`px-3 py-2 flex items-center gap-1 cursor-pointer ${
        activeTab === t ? "border-b-2 border-[var(--accent-primary)] text-white font-semibold" : ""
      }`}
    >
      {icon}
      <span>{label}</span>
    </button>
  );

  return (
    <div className="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex items-center justify-center p-4">
      <div className="bg-[var(--bg-sidebar)] border border-[var(--border-default)] w-full max-w-xl shadow-2xl p-4 flex flex-col h-[520px]">
        {/* Header */}
        <div className="flex items-center justify-between pb-2 border-b border-[var(--border-default)] shrink-0">
          <div className="flex items-center gap-1.5 font-bold text-sm text-[var(--fg-primary)]">
            <IconSettings className="size-4 text-[var(--accent-primary)]" />
            <span>Global Settings</span>
          </div>
          <button onClick={onClose} className="text-[var(--fg-tertiary)] hover:text-white cursor-pointer">
            <IconX className="size-4" />
          </button>
        </div>

        {/* Tab Selection */}
        <div className="flex border-b border-[var(--border-default)] text-xs text-[var(--fg-secondary)] shrink-0 overflow-x-auto">
          {tabBtn("shortcuts", <IconKeyboard className="size-3.5" />, "Shortcuts")}
          {tabBtn("appearance", <IconPalette className="size-3.5" />, "Appearance")}
          {tabBtn("providers", <IconCpu className="size-3.5" />, "Providers")}
          {tabBtn("agents", <IconRobot className="size-3.5" />, "Agents")}
          {tabBtn("mcp", <IconPlug className="size-3.5" />, "MCP")}
        </div>

        {/* Tab Content */}
        <div className="flex-1 overflow-y-auto py-3">
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
                    <span className="text-xs font-mono px-2 py-0.5 bg-black/40 border border-[var(--border-default)] text-[var(--accent-primary)] rounded">
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
                <option value="zed">Zed Dark Charcoal (Recommended)</option>
                <option value="dark">Forge Dark</option>
                <option value="light">Forge Light</option>
              </select>
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
                          onClick={() => {
                            const next = profiles.filter((x) => (x.id || x.Id) !== pid);
                            setProfiles(next);
                            SaveProviderProfiles(next);
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
          ) : (
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
                    placeholder="Args (space-separated, e.g. -y @modelcontextprotocol/server-filesystem)"
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
          )}
        </div>

        {/* Footer */}
        <div className="flex items-center justify-end pt-2 border-t border-[var(--border-default)] shrink-0">
          <button
            onClick={onClose}
            className="px-4 py-1.5 bg-[var(--accent-primary)] hover:bg-[var(--accent-hover)] text-black text-xs font-semibold cursor-pointer"
          >
            Done
          </button>
        </div>
      </div>
    </div>
  );
}
