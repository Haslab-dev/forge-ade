import React, { useState, useEffect } from "react";
import {
  X,
  Plus,
  RefreshCw,
  Eye,
  EyeOff,
  Check,
  Server,
  Key,
  Globe,
  Trash2,
  ChevronDown,
  ChevronRight,
} from "lucide-react";
import { GetProviderProfiles, SaveProviderProfiles, FetchProviderModels } from "../../wailsjs/go/main/App";

export interface ProviderProfile {
  id: string;
  name: string;
  api_key: string;
  base_url: string;
  enabled: boolean;
  available_models: string[];
  selected_models: string[];
}

interface ProviderModalProps {
  open: boolean;
  onClose: () => void;
  onSaved?: () => void;
}

export function ProviderModal({ open, onClose, onSaved }: ProviderModalProps) {
  const [profiles, setProfiles] = useState<ProviderProfile[]>([]);
  const [showKey, setShowKey] = useState<Record<string, boolean>>({});
  const [expandedProvider, setExpandedProvider] = useState<Record<string, boolean>>({});
  const [fetching, setFetching] = useState<Record<string, boolean>>({});

  useEffect(() => {
    if (open) {
      loadProfiles();
    }
  }, [open]);

  async function loadProfiles() {
    try {
      const list = await GetProviderProfiles();
      if (Array.isArray(list) && list.length > 0) {
        setProfiles(list);
      }
    } catch { /* ignore */ }
  }

  async function handleFetchModels(index: number) {
    const prof = profiles[index];
    setFetching((prev) => ({ ...prev, [prof.id]: true }));
    try {
      const models = await FetchProviderModels(prof.api_key, prof.base_url);
      setProfiles((prev) => {
        const copy = [...prev];
        const currentSelected = copy[index].selected_models || [];
        const newSelected = Array.from(new Set([...currentSelected, ...(models.slice(0, 3))]));
        copy[index] = {
          ...copy[index],
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

  function handleToggleModel(profIndex: number, modelName: string) {
    setProfiles((prev) => {
      const copy = [...prev];
      const selected = copy[profIndex].selected_models || [];
      const nextSelected = selected.includes(modelName)
        ? selected.filter((m) => m !== modelName)
        : [...selected, modelName];
      copy[profIndex] = { ...copy[profIndex], selected_models: nextSelected };
      return copy;
    });
  }

  function handleAddProvider() {
    const newProf: ProviderProfile = {
      id: "custom_" + Date.now(),
      name: "Custom OpenAI Compatible",
      api_key: "",
      base_url: "http://localhost:8080/v1",
      enabled: true,
      available_models: [],
      selected_models: [],
    };
    setProfiles((prev) => [...prev, newProf]);
  }

  async function handleSave() {
    try {
      await SaveProviderProfiles(profiles);
      if (onSaved) onSaved();
      onClose();
    } catch (err: any) {
      alert("Failed to save provider profiles: " + err);
    }
  }

  if (!open) return null;

  return (
    <div className="fixed inset-0 bg-black/70 backdrop-blur-sm z-50 flex items-center justify-center p-4">
      <div className="bg-[var(--color-bg-secondary)] border border-[var(--color-border)] rounded-xl w-full max-w-2xl max-h-[85vh] flex flex-col shadow-2xl">
        {/* Header */}
        <div className="flex items-center justify-between px-5 py-3.5 border-b border-[var(--color-border)]">
          <div className="flex items-center space-x-2 font-semibold text-white">
            <Server className="w-5 h-5 text-blue-400" />
            <span>LLM Providers & Model Configuration</span>
          </div>
          <button onClick={onClose} className="p-1 rounded hover:bg-white/10 text-gray-400 hover:text-white">
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Content list */}
        <div className="flex-1 overflow-y-auto p-5 space-y-4 font-sans text-sm">
          {profiles.map((prof, idx) => (
            <div
              key={prof.id}
              className="border border-[var(--color-border)] bg-[var(--color-bg-primary)] rounded-lg p-4 space-y-3"
            >
              {/* Top row: Name & Enabled toggle */}
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
                  className="bg-transparent font-bold text-white text-base focus:outline-none border-b border-transparent focus:border-blue-500"
                />
                <label className="flex items-center space-x-2 cursor-pointer text-xs text-gray-300">
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
                  <span>Enable Provider</span>
                </label>
              </div>

              {/* Endpoint & Key Inputs */}
              <div className="grid grid-cols-1 md:grid-cols-2 gap-3 text-xs">
                <div>
                  <label className="text-gray-400 flex items-center space-x-1 mb-1">
                    <Globe className="w-3.5 h-3.5" />
                    <span>Base URL / Endpoint</span>
                  </label>
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
                    placeholder="https://api.openai.com/v1"
                    className="w-full bg-[var(--color-bg-secondary)] border border-[var(--color-border)] rounded px-2.5 py-1.5 text-gray-200 font-mono focus:outline-none focus:border-blue-500"
                  />
                </div>

                <div>
                  <label className="text-gray-400 flex items-center space-x-1 mb-1">
                    <Key className="w-3.5 h-3.5" />
                    <span>API Key (Encrypted Profile)</span>
                  </label>
                  <div className="relative">
                    <input
                      type={showKey[prof.id] ? "text" : "password"}
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
                      className="w-full bg-[var(--color-bg-secondary)] border border-[var(--color-border)] rounded px-2.5 py-1.5 pr-8 text-gray-200 font-mono focus:outline-none focus:border-blue-500"
                    />
                    <button
                      onClick={() => setShowKey((prev) => ({ ...prev, [prof.id]: !prev[prof.id] }))}
                      className="absolute right-2 top-2 text-gray-400 hover:text-white"
                    >
                      {showKey[prof.id] ? <EyeOff className="w-3.5 h-3.5" /> : <Eye className="w-3.5 h-3.5" />}
                    </button>
                  </div>
                </div>
              </div>

              {/* Fetch Models Bar */}
              <div className="flex items-center justify-between pt-2 border-t border-[var(--color-border)]/50">
                <button
                  onClick={() => handleFetchModels(idx)}
                  disabled={fetching[prof.id]}
                  className="px-3 py-1.5 bg-blue-600/30 hover:bg-blue-600 text-blue-200 hover:text-white border border-blue-500/40 rounded text-xs font-medium flex items-center space-x-1.5 transition-all"
                >
                  <RefreshCw className={`w-3.5 h-3.5 ${fetching[prof.id] ? "animate-spin" : ""}`} />
                  <span>Fetch Models</span>
                </button>

                {prof.available_models && prof.available_models.length > 0 && (
                  <button
                    onClick={() => setExpandedProvider((prev) => ({ ...prev, [prof.id]: !prev[prof.id] }))}
                    className="text-xs text-gray-400 hover:text-white flex items-center space-x-1"
                  >
                    <span>{prof.available_models.length} models fetched ({prof.selected_models?.length || 0} selected)</span>
                    {expandedProvider[prof.id] ? <ChevronDown className="w-3.5 h-3.5" /> : <ChevronRight className="w-3.5 h-3.5" />}
                  </button>
                )}
              </div>

              {/* Models Checkbox Selection Accordion */}
              {expandedProvider[prof.id] && prof.available_models && prof.available_models.length > 0 && (
                <div className="bg-[var(--color-bg-secondary)] border border-[var(--color-border)] rounded-md p-3 max-h-40 overflow-y-auto grid grid-cols-1 md:grid-cols-2 gap-2 text-xs font-mono">
                  {prof.available_models.map((model) => {
                    const isSelected = prof.selected_models?.includes(model);
                    return (
                      <label key={model} className="flex items-center space-x-2 cursor-pointer hover:text-white text-gray-300 truncate">
                        <input
                          type="checkbox"
                          checked={isSelected}
                          onChange={() => handleToggleModel(idx, model)}
                          className="accent-blue-600 rounded"
                        />
                        <span className="truncate">{model}</span>
                      </label>
                    );
                  })}
                </div>
              )}
            </div>
          ))}

          <button
            onClick={handleAddProvider}
            className="w-full py-2 border border-dashed border-[var(--color-border)] hover:border-blue-500/50 rounded-lg text-xs font-medium text-gray-400 hover:text-white flex items-center justify-center space-x-1.5 transition-all"
          >
            <Plus className="w-4 h-4" />
            <span>Add Custom Provider</span>
          </button>
        </div>

        {/* Footer */}
        <div className="flex items-center justify-end space-x-3 px-5 py-3 border-t border-[var(--color-border)] bg-[var(--color-bg-primary)] rounded-b-xl">
          <button onClick={onClose} className="px-4 py-2 text-xs font-medium text-gray-300 hover:text-white">
            Cancel
          </button>
          <button
            onClick={handleSave}
            className="px-5 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-semibold flex items-center space-x-1.5 shadow"
          >
            <Check className="w-4 h-4" />
            <span>Save Provider Configuration</span>
          </button>
        </div>
      </div>
    </div>
  );
}
