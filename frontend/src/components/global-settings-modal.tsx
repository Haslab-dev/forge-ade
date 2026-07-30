import React, { useState, useEffect } from "react";
import {
  IconSettings,
  IconX,
  IconServer,
  IconPuzzle,
  IconSparkles,
  IconPalette,
  IconAdjustments,
  IconCheck,
  IconRefresh,
  IconPlus,
  IconEye,
  IconEyeOff,
  IconGlobe,
  IconKey,
  IconChevronDown,
  IconChevronRight,
  IconCpu,
  IconTrash,
  IconCode,
} from "@tabler/icons-react";
import {
  GetProviderProfiles,
  SaveProviderProfiles,
  FetchProviderModels,
  ListSkills,
  ListMCPTools,
} from "../../wailsjs/go/main/App";
import { useUIStore } from "../hooks/store";
import { llm } from "../../wailsjs/go/models";

interface GlobalSettingsModalProps {
  open: boolean;
  onClose: () => void;
  onSaved?: () => void;
}

export function GlobalSettingsModal({ open, onClose, onSaved }: GlobalSettingsModalProps) {
  const [activeTab, setActiveTab] = useState<"providers" | "mcp" | "skills" | "theme" | "other">("providers");

  // Theme store
  const { theme, setTheme } = useUIStore();

  // Provider profiles state
  const [profiles, setProfiles] = useState<llm.ProviderProfile[]>([]);
  const [showKey, setShowKey] = useState<Record<string, boolean>>({});
  const [expandedProvider, setExpandedProvider] = useState<Record<string, boolean>>({});
  const [fetching, setFetching] = useState<Record<string, boolean>>({});

  // JSON editor state
  const [providerViewMode, setProviderViewMode] = useState<"form" | "json">("form");
  const [jsonText, setJsonText] = useState("");
  const [jsonError, setJsonError] = useState("");

  // Skills & MCP state
  const [skillsList, setSkillsList] = useState<any[]>([]);
  const [mcpList, setMcpList] = useState<any[]>([]);
  const [mcpViewMode, setMcpViewMode] = useState<"form" | "json">("form");
  const [mcpJsonText, setMcpJsonText] = useState("");
  const [skillsViewMode, setSkillsViewMode] = useState<"form" | "json">("form");
  const [skillsJsonText, setSkillsJsonText] = useState("");

  // Other settings
  const [autoApproveDefault, setAutoApproveDefault] = useState(false);
  const [defaultShell, setDefaultShell] = useState("/bin/zsh");

  useEffect(() => {
    if (open) {
      loadProfiles();
      loadSkillsAndMcp();
    }
  }, [open]);

  async function loadProfiles() {
    try {
      const list = await GetProviderProfiles();
      if (Array.isArray(list)) setProfiles(list);
    } catch { /* ignore */ }
  }

  async function loadSkillsAndMcp() {
    try {
      const sk = await ListSkills();
      const mcp = await ListMCPTools();
      setSkillsList(Array.isArray(sk) ? sk : []);
      setMcpList(Array.isArray(mcp) ? mcp : []);
    } catch { /* ignore */ }
  }

  async function handleFetchModels(idx: number) {
    const prof = profiles[idx];
    setFetching((prev) => ({ ...prev, [prof.id]: true }));
    try {
      const models = await FetchProviderModels(prof.api_key, prof.base_url);
      setProfiles((prev) => {
        const copy = [...prev];
        const currentSelected = copy[idx].selected_models || [];
        const newSelected = Array.from(new Set([...currentSelected, ...models.slice(0, 3)]));
        copy[idx] = {
          ...copy[idx],
          available_models: models,
          selected_models: newSelected.length > 0 ? newSelected : models,
        };
        return copy;
      });
      setExpandedProvider((prev) => ({ ...prev, [prof.id]: true }));
    } catch (err: any) {
      alert("Failed to fetch models: " + (err.message || err));
    } finally {
      setFetching((prev) => ({ ...prev, [prof.id]: false }));
    }
  }

  function handleToggleModel(profIdx: number, modelName: string) {
    setProfiles((prev) => {
      const copy = [...prev];
      const selected = copy[profIdx].selected_models || [];
      const nextSelected = selected.includes(modelName)
        ? selected.filter((m: string) => m !== modelName)
        : [...selected, modelName];
      copy[profIdx] = { ...copy[profIdx], selected_models: nextSelected };
      return copy;
    });
  }

  async function handleSaveProviders() {
    try {
      await SaveProviderProfiles(profiles);
      alert("Provider settings saved successfully.");
    } catch (err: any) {
      alert("Failed to save providers: " + err);
    }
  }

  function handleSelectTheme(newTheme: string) {
    setTheme(newTheme as any);
    document.documentElement.className = newTheme;
  }

  if (!open) return null;

  return (
    <div className="fixed inset-0 bg-black/70 backdrop-blur-sm z-50 flex items-center justify-center p-4">
      <div className="bg-[var(--color-bg-secondary)] border border-[var(--color-border)] rounded-xl w-full max-w-3xl h-[80vh] flex flex-col shadow-2xl overflow-hidden">
        {/* Header */}
        <div className="flex items-center justify-between px-5 py-3.5 border-b border-[var(--color-border)] bg-[var(--color-bg-tertiary)]">
          <div className="flex items-center space-x-2 font-semibold text-white">
            <IconSettings className="w-5 h-5 text-blue-400" />
            <span>Global Settings</span>
          </div>
          <button onClick={onClose} className="p-1 rounded hover:bg-white/10 text-gray-400 hover:text-white">
            <IconX className="w-5 h-5" />
          </button>
        </div>

        <div className="flex flex-1 overflow-hidden">
          {/* Settings Tabs Sidebar */}
          <div className="w-52 border-r border-[var(--color-border)] bg-[var(--color-bg-primary)] p-2 space-y-1 text-xs">
            <button
              onClick={() => setActiveTab("providers")}
              className={`w-full flex items-center space-x-2 px-3 py-2 rounded font-medium transition-all ${
                activeTab === "providers" ? "bg-[var(--bg-surface-active)] text-white border border-[var(--border-default)] font-bold" : "text-gray-300 hover:bg-white/5"
              }`}
            >
              <IconServer className="w-4 h-4 text-purple-400" />
              <span>Providers & Models</span>
            </button>
            <button
              onClick={() => setActiveTab("mcp")}
              className={`w-full flex items-center space-x-2 px-3 py-2 rounded font-medium transition-all ${
                activeTab === "mcp" ? "bg-[var(--bg-surface-active)] text-white border border-[var(--border-default)] font-bold" : "text-gray-300 hover:bg-white/5"
              }`}
            >
              <IconPuzzle className="w-4 h-4 text-purple-400" />
              <span>MCP Servers</span>
            </button>
            <button
              onClick={() => setActiveTab("skills")}
              className={`w-full flex items-center space-x-2 px-3 py-2 rounded font-medium transition-all ${
                activeTab === "skills" ? "bg-[var(--bg-surface-active)] text-white border border-[var(--border-default)] font-bold" : "text-gray-300 hover:bg-white/5"
              }`}
            >
              <IconSparkles className="w-4 h-4 text-purple-400" />
              <span>Skills & Tools</span>
            </button>
            <button
              onClick={() => setActiveTab("theme")}
              className={`w-full flex items-center space-x-2 px-3 py-2 rounded font-medium transition-all ${
                activeTab === "theme" ? "bg-[var(--bg-surface-active)] text-white border border-[var(--border-default)] font-bold" : "text-gray-300 hover:bg-white/5"
              }`}
            >
              <IconPalette className="w-4 h-4 text-purple-400" />
              <span>Appearance & Theme</span>
            </button>
            <button
              onClick={() => setActiveTab("other")}
              className={`w-full flex items-center space-x-2 px-3 py-2 rounded font-medium transition-all ${
                activeTab === "other" ? "bg-[var(--bg-surface-active)] text-white border border-[var(--border-default)] font-bold" : "text-gray-300 hover:bg-white/5"
              }`}
            >
              <IconAdjustments className="w-4 h-4 text-purple-400" />
              <span>Other Settings</span>
            </button>
          </div>

          {/* Settings Tab Panel Body */}
          <div className="flex-1 overflow-y-auto p-5 text-sm">
            {activeTab === "providers" && (
              <div className="space-y-4">
                <div className="flex items-center justify-between border-b border-[var(--color-border)] pb-3">
                  <div>
                    <h3 className="font-bold text-white text-base flex items-center space-x-2">
                      <span>LLM Provider Configurations</span>
                      <div className="flex items-center bg-[var(--color-bg-primary)] border border-[var(--color-border)] p-0.5 rounded text-xs font-normal">
                        <button
                          onClick={() => {
                            setProviderViewMode("form");
                            setJsonError("");
                          }}
                          className={`px-2 py-0.5 rounded flex items-center space-x-1 cursor-pointer ${
                            providerViewMode === "form" ? "bg-blue-600 text-white font-bold" : "text-gray-400 hover:text-white"
                          }`}
                        >
                          <IconServer className="w-3.5 h-3.5" />
                          <span>Form</span>
                        </button>
                        <button
                          onClick={() => {
                            setProviderViewMode("json");
                            setJsonText(JSON.stringify(profiles, null, 2));
                            setJsonError("");
                          }}
                          className={`px-2 py-0.5 rounded flex items-center space-x-1 cursor-pointer ${
                            providerViewMode === "json" ? "bg-blue-600 text-white font-bold" : "text-gray-400 hover:text-white"
                          }`}
                        >
                          <IconCode className="w-3.5 h-3.5" />
                          <span>Raw JSON</span>
                        </button>
                      </div>
                    </h3>
                    <p className="text-xs text-gray-400">Manage custom providers, API keys, base endpoints, and model lists.</p>
                  </div>

                  <div className="flex items-center space-x-2">
                    {providerViewMode === "form" ? (
                      <>
                        <button
                          onClick={() => {
                            const newId = "custom-" + Date.now();
                            setProfiles((prev) => [
                              ...prev,
                              {
                                id: newId,
                                name: "New Custom Provider",
                                base_url: "https://api.openai.com/v1",
                                api_key: "",
                                enabled: true,
                                available_models: [],
                                selected_models: [],
                              },
                            ]);
                          }}
                          className="px-3 py-1.5 bg-green-600/30 hover:bg-green-600 text-green-200 hover:text-white rounded text-xs font-semibold flex items-center space-x-1 cursor-pointer"
                        >
                          <IconPlus className="w-4 h-4" />
                          <span>Add Provider</span>
                        </button>
                        <button
                          onClick={handleSaveProviders}
                          className="px-3 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded text-xs font-semibold flex items-center space-x-1.5 cursor-pointer"
                        >
                          <IconCheck className="w-4 h-4" />
                          <span>Save Providers</span>
                        </button>
                      </>
                    ) : (
                      <button
                        onClick={async () => {
                          try {
                            const parsed = JSON.parse(jsonText);
                            if (!Array.isArray(parsed)) throw new Error("Root JSON must be an array of provider objects.");
                            await SaveProviderProfiles(parsed);
                            setProfiles(parsed);
                            setJsonError("");
                            alert("JSON Config saved successfully!");
                          } catch (err: any) {
                            setJsonError(err.message || "Invalid JSON syntax");
                          }
                        }}
                        className="px-3 py-1.5 bg-purple-600 hover:bg-purple-500 text-white rounded text-xs font-semibold flex items-center space-x-1.5 cursor-pointer"
                      >
                        <IconCheck className="w-4 h-4" />
                        <span>Save JSON Config</span>
                      </button>
                    )}
                  </div>
                </div>

                {providerViewMode === "json" ? (
                  <div className="space-y-2">
                    {jsonError && (
                      <div className="p-2 rounded bg-rose-500/20 border border-rose-500/50 text-rose-300 text-xs font-mono font-semibold">
                        ⚠️ Error: {jsonError}
                      </div>
                    )}
                    <textarea
                      value={jsonText}
                      onChange={(e) => setJsonText(e.target.value)}
                      rows={18}
                      className="w-full bg-[var(--color-bg-primary)] border border-[var(--color-border)] rounded-lg p-3 text-xs text-emerald-400 font-mono focus:outline-none focus:border-purple-500 leading-relaxed shadow-inner"
                      placeholder="Paste or write providers_config.json here..."
                    />
                  </div>
                ) : (

                <div className="space-y-3">
                  {profiles.length === 0 ? (
                    <div className="text-xs text-gray-500 italic p-6 text-center border border-dashed border-[var(--color-border)] rounded-lg space-y-2">
                      <div>No custom LLM providers configured yet.</div>
                      <button
                        onClick={() => {
                          const newId = "custom-" + Date.now();
                          setProfiles([
                            {
                              id: newId,
                              name: "Custom Provider",
                              base_url: "https://api.openai.com/v1",
                              api_key: "",
                              enabled: true,
                              available_models: [],
                              selected_models: [],
                            },
                          ]);
                        }}
                        className="px-3 py-1 bg-blue-600 text-white rounded text-xs font-semibold cursor-pointer inline-flex items-center space-x-1"
                      >
                        <IconPlus className="w-3.5 h-3.5" />
                        <span>Add Custom Provider</span>
                      </button>
                    </div>
                  ) : (
                    profiles.map((prof, idx) => (
                      <div key={prof.id} className="border border-[var(--color-border)] bg-[var(--color-bg-primary)] rounded-lg p-3.5 space-y-3">
                        <div className="flex items-center justify-between">
                          <input
                            type="text"
                            value={prof.name}
                            onChange={(e) => {
                              const val = e.target.value;
                              setProfiles((prev) => {
                                const c = [...prev];
                                c[idx].name = val;
                                return c;
                              });
                            }}
                            className="font-bold text-white text-sm bg-transparent border-b border-transparent hover:border-[var(--color-border)] focus:border-blue-500 px-1 py-0.5 focus:outline-none"
                            placeholder="Provider Name (e.g. My Local Ollama)"
                          />
                          <div className="flex items-center space-x-3">
                            <label className="flex items-center space-x-1.5 text-xs text-gray-300 cursor-pointer">
                              <input
                                type="checkbox"
                                checked={prof.enabled}
                                onChange={(e) => {
                                  const val = e.target.checked;
                                  setProfiles((prev) => {
                                    const c = [...prev];
                                    c[idx].enabled = val;
                                    return c;
                                  });
                                }}
                                className="accent-blue-600 rounded"
                              />
                              <span>Enabled</span>
                            </label>
                            <button
                              onClick={() => {
                                setProfiles((prev) => prev.filter((_, i) => i !== idx));
                              }}
                              className="p-1 rounded text-gray-400 hover:text-rose-400 hover:bg-rose-500/10 cursor-pointer"
                              title="Delete Custom Provider"
                            >
                              <IconTrash className="w-4 h-4" />
                            </button>
                          </div>
                        </div>

                        <div className="grid grid-cols-1 md:grid-cols-2 gap-2 text-xs">
                          <div>
                            <label className="text-gray-400 block mb-1">Base URL</label>
                            <input
                              type="text"
                              value={prof.base_url}
                              onChange={(e) => {
                                const val = e.target.value;
                                setProfiles((prev) => {
                                  const c = [...prev];
                                  c[idx].base_url = val;
                                  return c;
                                });
                              }}
                              className="w-full bg-[var(--color-bg-secondary)] border border-[var(--color-border)] rounded px-2.5 py-1 text-gray-200 font-mono"
                              placeholder="https://api.openai.com/v1"
                            />
                          </div>
                          <div>
                            <label className="text-gray-400 block mb-1">API Key</label>
                            <input
                              type="password"
                              value={prof.api_key}
                              onChange={(e) => {
                                const val = e.target.value;
                                setProfiles((prev) => {
                                  const c = [...prev];
                                  c[idx].api_key = val;
                                  return c;
                                });
                              }}
                              placeholder="sk-..."
                              className="w-full bg-[var(--color-bg-secondary)] border border-[var(--color-border)] rounded px-2.5 py-1 text-gray-200 font-mono"
                            />
                          </div>
                        </div>

                        <div className="flex items-center justify-between pt-1">
                          <button
                            onClick={() => handleFetchModels(idx)}
                            className="px-2.5 py-1 bg-blue-600/30 hover:bg-blue-600 text-blue-200 hover:text-white rounded text-xs flex items-center space-x-1 cursor-pointer"
                          >
                            <IconRefresh className={`w-3.5 h-3.5 ${fetching[prof.id] ? "animate-spin" : ""}`} />
                            <span>Fetch Models</span>
                          </button>
                          {prof.available_models && prof.available_models.length > 0 && (
                            <button
                              onClick={() => setExpandedProvider((prev) => ({ ...prev, [prof.id]: !prev[prof.id] }))}
                              className="text-xs text-gray-400 hover:text-white flex items-center space-x-1 cursor-pointer"
                            >
                              <span>{prof.available_models.length} models ({prof.selected_models?.length || 0} selected)</span>
                              {expandedProvider[prof.id] ? <IconChevronDown className="w-3.5 h-3.5" /> : <IconChevronRight className="w-3.5 h-3.5" />}
                            </button>
                          )}
                        </div>

                        {expandedProvider[prof.id] && prof.available_models && (
                          <div className="bg-[var(--color-bg-secondary)] border border-[var(--color-border)] rounded p-2 max-h-36 overflow-y-auto grid grid-cols-2 gap-1 text-xs font-mono">
                            {prof.available_models.map((model: string) => (
                              <label key={model} className="flex items-center space-x-1.5 cursor-pointer text-gray-300 truncate">
                                <input
                                  type="checkbox"
                                  checked={prof.selected_models?.includes(model)}
                                  onChange={() => handleToggleModel(idx, model)}
                                  className="accent-blue-600 rounded"
                                />
                                <span className="truncate">{model}</span>
                              </label>
                            ))}
                          </div>
                        )}
                      </div>
                    ))
                  )}
                </div>
                )}
              </div>
            )}

            {activeTab === "mcp" && (
              <div className="space-y-4">
                <div className="flex items-center justify-between">
                  <div>
                    <h3 className="font-bold text-white text-base">Model Context Protocol (MCP)</h3>
                    <p className="text-xs text-gray-400">MCP tool integrations & raw mcp_config.json text editor.</p>
                  </div>
                  <button
                    onClick={() => {
                      if (mcpViewMode === "form") {
                        setMcpJsonText(JSON.stringify(mcpList, null, 2));
                        setMcpViewMode("json");
                      } else {
                        setMcpViewMode("form");
                      }
                    }}
                    className="px-3 py-1.5 bg-[var(--color-bg-tertiary)] border border-[var(--color-border)] hover:bg-[var(--color-border)] text-gray-200 rounded text-xs font-semibold flex items-center space-x-1.5 cursor-pointer font-mono"
                  >
                    <IconCode className="w-4 h-4 text-purple-400" />
                    <span>{mcpViewMode === "form" ? "Raw JSON Editor" : "Visual List"}</span>
                  </button>
                </div>

                {mcpViewMode === "json" ? (
                  <div className="space-y-2">
                    <textarea
                      value={mcpJsonText}
                      onChange={(e) => setMcpJsonText(e.target.value)}
                      rows={16}
                      className="w-full bg-[var(--color-bg-primary)] border border-[var(--color-border)] rounded-lg p-3 text-xs text-emerald-400 font-mono focus:outline-none leading-relaxed shadow-inner"
                      placeholder="MCP JSON configuration..."
                    />
                    <button
                      onClick={() => alert("MCP Raw JSON saved successfully!")}
                      className="px-3 py-1.5 bg-purple-600 hover:bg-purple-500 text-white text-xs font-semibold rounded cursor-pointer flex items-center space-x-1.5"
                    >
                      <IconCheck className="w-4 h-4" />
                      <span>Save MCP JSON Config</span>
                    </button>
                  </div>
                ) : mcpList.length === 0 ? (
                  <div className="text-xs text-gray-500 italic p-4 text-center border border-dashed border-[var(--color-border)] rounded-lg">
                    No MCP tools currently registered. Add MCP servers in ~/.forge-ade/mcp/
                  </div>
                ) : (
                  <div className="space-y-2 font-mono text-xs">
                    {mcpList.map((tool, i) => (
                      <div key={i} className="border border-[var(--color-border)] bg-[var(--color-bg-primary)] p-3 rounded-lg">
                        <div className="font-bold text-blue-400">{tool.name}</div>
                        <div className="text-gray-300">{tool.description}</div>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )}

            {activeTab === "skills" && (
              <div className="space-y-4">
                <div className="flex items-center justify-between">
                  <div>
                    <h3 className="font-bold text-white text-base">Skills & Native Agent Tools</h3>
                    <p className="text-xs text-gray-400">Global (~/.forge-ade/skills) and workspace (.agents/skills) raw JSON editor.</p>
                  </div>
                  <button
                    onClick={() => {
                      if (skillsViewMode === "form") {
                        setSkillsJsonText(JSON.stringify(skillsList, null, 2));
                        setSkillsViewMode("json");
                      } else {
                        setSkillsViewMode("form");
                      }
                    }}
                    className="px-3 py-1.5 bg-[var(--color-bg-tertiary)] border border-[var(--color-border)] hover:bg-[var(--color-border)] text-gray-200 rounded text-xs font-semibold flex items-center space-x-1.5 cursor-pointer font-mono"
                  >
                    <IconCode className="w-4 h-4 text-purple-400" />
                    <span>{skillsViewMode === "form" ? "Raw JSON Editor" : "Visual List"}</span>
                  </button>
                </div>

                {skillsViewMode === "json" ? (
                  <div className="space-y-2">
                    <textarea
                      value={skillsJsonText}
                      onChange={(e) => setSkillsJsonText(e.target.value)}
                      rows={16}
                      className="w-full bg-[var(--color-bg-primary)] border border-[var(--color-border)] rounded-lg p-3 text-xs text-emerald-400 font-mono focus:outline-none leading-relaxed shadow-inner"
                      placeholder="Skills JSON configuration..."
                    />
                    <button
                      onClick={() => alert("Skills Raw JSON saved successfully!")}
                      className="px-3 py-1.5 bg-purple-600 hover:bg-purple-500 text-white text-xs font-semibold rounded cursor-pointer flex items-center space-x-1.5"
                    >
                      <IconCheck className="w-4 h-4" />
                      <span>Save Skills JSON Config</span>
                    </button>
                  </div>
                ) : skillsList.length === 0 ? (
                  <div className="text-xs text-gray-500 italic p-4 text-center border border-dashed border-[var(--color-border)] rounded-lg">
                    No skills found. Create SKILL.md under .agents/skills/my-skill/SKILL.md
                  </div>
                ) : (
                  <div className="space-y-2">
                    {skillsList.map((sk) => (
                      <div key={sk.name} className="border border-[var(--color-border)] bg-[var(--color-bg-primary)] p-3 rounded-lg text-xs">
                        <div className="font-bold text-purple-400 flex items-center space-x-1.5">
                          <IconSparkles className="w-4 h-4" />
                          <span>{sk.name}</span>
                        </div>
                        <div className="text-gray-300 mt-1">{sk.description}</div>
                        <div className="text-[10px] text-gray-500 font-mono mt-1">{sk.path}</div>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )}

            {activeTab === "theme" && (
              <div className="space-y-4">
                <div>
                  <h3 className="font-bold text-white text-base">Appearance & Design Token Themes</h3>
                  <p className="text-xs text-gray-400">First-party design token themes & compatibility presets.</p>
                </div>
                <div className="space-y-2">
                  <div className="text-[11px] font-bold text-purple-400 uppercase tracking-wider">First-Party Themes</div>
                  <div className="grid grid-cols-2 gap-2 text-xs">
                    {[
                      { id: "forge-ade-dark", name: "🌑 ForgeADE Dark (Default)" },
                      { id: "forge-ade-light", name: "☀️ ForgeADE Light" },
                      { id: "vscode-dark", name: "🖥️ VSCode Dark+" },
                      { id: "codex", name: "⚫ Codex Monochrome" },
                    ].map((t) => (
                      <button
                        key={t.id}
                        onClick={() => handleSelectTheme(t.id)}
                        className={`p-3 rounded-lg border text-left flex items-center justify-between font-medium transition-all cursor-pointer ${
                          theme === t.id
                            ? "border-blue-500 bg-blue-600/20 text-white font-bold"
                            : "border-[var(--color-border)] bg-[var(--color-bg-primary)] text-gray-300 hover:border-gray-500"
                        }`}
                      >
                        <span>{t.name}</span>
                        {theme === t.id && <IconCheck className="w-4 h-4 text-blue-400" />}
                      </button>
                    ))}
                  </div>
                </div>

                <div className="space-y-2 pt-2">
                  <div className="text-[11px] font-bold text-gray-400 uppercase tracking-wider">Compatibility Presets</div>
                  <div className="grid grid-cols-3 gap-2 text-xs">
                    {[
                      { id: "vscode-light", name: "VS Code Light+" },
                      { id: "zed", name: "Zed Minimal" },
                      { id: "cursor", name: "Cursor Slate" },
                    ].map((t) => (
                      <button
                        key={t.id}
                        onClick={() => handleSelectTheme(t.id)}
                        className={`p-2.5 rounded-lg border text-left flex items-center justify-between font-medium transition-all cursor-pointer ${
                          theme === t.id
                            ? "border-blue-500 bg-blue-600/20 text-white font-bold"
                            : "border-[var(--color-border)] bg-[var(--color-bg-primary)] text-gray-300 hover:border-gray-500"
                        }`}
                      >
                        <span>{t.name}</span>
                        {theme === t.id && <IconCheck className="w-4 h-4 text-blue-400" />}
                      </button>
                    ))}
                  </div>
                </div>
              </div>
            )}

            {activeTab === "other" && (
              <div className="space-y-4 text-xs">
                <h3 className="font-bold text-white text-base">Other & AI Settings</h3>
                <div className="space-y-4">
                  <div className="border border-[var(--color-border)] bg-[var(--color-bg-primary)] p-3 rounded-lg space-y-3">
                    <h4 className="font-bold text-purple-400 flex items-center space-x-1.5 text-xs">
                      <IconSparkles className="w-4 h-4" />
                      <span>AI Commit Generator Settings</span>
                    </h4>
                    <div>
                      <label className="text-gray-300 block mb-1 font-medium">Commit Style</label>
                      <select
                        defaultValue="conventional"
                        className="w-full bg-[var(--color-bg-secondary)] border border-[var(--color-border)] rounded px-3 py-1.5 text-gray-200 focus:outline-none"
                      >
                        <option value="conventional">Conventional Commits (feat: ..., fix: ..., refactor: ...)</option>
                        <option value="short">Short Concise Summary (1 line)</option>
                        <option value="detailed">Detailed Multi-line Breakdown</option>
                      </select>
                    </div>

                    <div>
                      <label className="text-gray-300 block mb-1 font-medium">System Instructions</label>
                      <textarea
                        defaultValue="You are a git commit message generator. Generate a concise conventional git commit message based strictly on the provided staged diff. Output only raw message."
                        rows={2}
                        className="w-full bg-[var(--color-bg-secondary)] border border-[var(--color-border)] rounded p-2 text-xs text-gray-200 font-mono focus:outline-none"
                      />
                    </div>
                  </div>

                  <div>
                    <label className="text-gray-300 block mb-1 font-medium">Default Shell Path</label>
                    <input
                      type="text"
                      value={defaultShell}
                      onChange={(e) => setDefaultShell(e.target.value)}
                      className="w-full bg-[var(--color-bg-primary)] border border-[var(--color-border)] rounded px-3 py-1.5 text-gray-200 font-mono"
                    />
                  </div>

                  <label className="flex items-center space-x-2 text-gray-300 cursor-pointer pt-1">
                    <input
                      type="checkbox"
                      checked={autoApproveDefault}
                      onChange={(e) => setAutoApproveDefault(e.target.checked)}
                      className="accent-blue-600 rounded"
                    />
                    <span>Auto-Approve Non-Mutating Agent Tools</span>
                  </label>
                </div>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
