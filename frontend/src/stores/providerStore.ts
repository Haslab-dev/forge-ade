import { create } from 'zustand';
import { LLMProviderConfig } from '../types';
import { DEFAULT_PROVIDERS } from './agentRegistryStore';
import { ApiBridge } from '../services/apiBridge';
import {
  EventsOn,
  GetProviderProfiles,
  SetActiveModel,
  SyncAgentProviders,
  SaveProviderProfiles,
  DeleteProviderProfile,
  type LocalLLMService
} from '../lib/wails';

export interface ProviderState {
  providers: LLMProviderConfig[];
  deletedProviderIds: string[];
  localServices: LocalLLMService[];
  isScanningLocal: boolean;
  discoverLocalProviders: () => Promise<LocalLLMService[]>;
  setProviders: (
    providersOrFn: LLMProviderConfig[] | ((prev: LLMProviderConfig[]) => LLMProviderConfig[])
  ) => void;
  updateProvider: (id: string, updates: Partial<LLMProviderConfig>) => void;
  addProvider: (provider: LLMProviderConfig) => void;
  deleteProvider: (id: string) => void;
  addModelToProvider: (providerId: string, modelName: string) => void;
  deleteModelFromProvider: (providerId: string, modelName: string) => void;
  toggleModelSelection: (providerId: string, modelName: string) => void;
  selectAllModels: (providerId: string, modelsToSelect?: string[]) => void;
  deselectAllModels: (providerId: string, modelsToDeselect?: string[]) => void;
  fetchProviderModels: (providerId: string) => Promise<void>;
  activateGoModel: (model?: string) => Promise<void>;
  hydrateProvidersFromGo: () => Promise<void>;
  initProviderSync: () => () => void;
}

let goProfilesCache: any[] = [];
let lastSyncedFingerprint = '';
let lastActivatedModel = '';

function loadInitialProviders(): { providers: LLMProviderConfig[]; deletedIds: string[] } {
  let deletedIds: string[] = [];
  try {
    const raw = localStorage.getItem('forge_ade_deleted_provider_ids');
    if (raw) deletedIds = JSON.parse(raw);
  } catch {}

  let initialList: LLMProviderConfig[] = [];
  try {
    const saved = localStorage.getItem('forge_ade_providers') || localStorage.getItem('my_ade_providers');
    if (saved) initialList = JSON.parse(saved);
  } catch {}

  if (!initialList || initialList.length === 0) {
    initialList = DEFAULT_PROVIDERS;
  }
  // Strip out any legacy local placeholders (e.g. ollama) and deleted providers
  initialList = initialList.filter(p => p.id !== 'prov-ollama' && !p.id.toLowerCase().includes('ollama'));
  if (initialList.length === 0) {
    initialList = DEFAULT_PROVIDERS;
  }
  if (Array.isArray(deletedIds) && deletedIds.length > 0) {
    initialList = initialList.filter(p => !deletedIds.includes(p.id));
  }
  return { providers: initialList, deletedIds };
}

export const useProviderStore = create<ProviderState>((set, get) => {
  const initial = loadInitialProviders();

  return {
    providers: initial.providers,
    deletedProviderIds: initial.deletedIds,
    localServices: [],
    isScanningLocal: false,

    discoverLocalProviders: async () => {
      set({ isScanningLocal: true });
      try {
        const services = await ApiBridge.discoverLocalLLMs();
        set({ localServices: services || [] });

        const running = (services || []).filter(
          s => s && s.running && Array.isArray(s.models) && s.models.length > 0
        );
        if (running.length > 0) {
          get().setProviders(prev => {
            const next = [...prev];
            for (const s of running) {
              const existingIdx = next.findIndex(
                p => p.id === s.id || (p.baseUrl && s.baseUrl && p.baseUrl.includes(s.baseUrl))
              );
              if (existingIdx >= 0) {
                const existing = next[existingIdx];
                const mergedModels = Array.from(new Set([...(existing.models || []), ...s.models]));
                const mergedSelected = Array.from(
                  new Set([...(existing.selectedModels || existing.models || []), ...s.models])
                );
                next[existingIdx] = {
                  ...existing,
                  enabled: true,
                  baseUrl: s.baseUrl,
                  models: mergedModels,
                  selectedModels: mergedSelected
                };
              } else {
                next.push({
                  id: s.id,
                  name: s.name,
                  baseUrl: s.baseUrl,
                  apiKey: '',
                  enabled: true,
                  models: s.models,
                  selectedModels: s.models
                });
              }
            }
            return next;
          });
        }
        return services || [];
      } catch (err) {
        console.warn('discoverLocalProviders error:', err);
        return [];
      } finally {
        set({ isScanningLocal: false });
      }
    },

    setProviders: (providersOrFn) => {
      set(state => {
        const next = typeof providersOrFn === 'function' ? providersOrFn(state.providers) : providersOrFn;
        try {
          localStorage.setItem('forge_ade_providers', JSON.stringify(next));
          localStorage.setItem('my_ade_providers', JSON.stringify(next));
        } catch {}
        return { providers: next };
      });
    },

    updateProvider: (id, updates) => {
      get().setProviders(prev => prev.map(p => (p.id === id ? { ...p, ...updates } : p)));
    },

    addProvider: (provider) => {
      get().setProviders(prev => [...prev, provider]);
    },

    deleteProvider: (id) => {
      const deletedIds = Array.from(new Set([...get().deletedProviderIds, id]));
      try {
        localStorage.setItem('forge_ade_deleted_provider_ids', JSON.stringify(deletedIds));
      } catch {}

      const next = get().providers.filter(p => p.id !== id);
      set({ providers: next, deletedProviderIds: deletedIds });
      try {
        localStorage.setItem('forge_ade_providers', JSON.stringify(next));
      } catch {}

      DeleteProviderProfile(id).catch(() => {});

      const payload = next.map(p => ({
        id: p.id,
        name: p.name,
        api_key: p.apiKey || '',
        base_url: p.baseUrl || '',
        enabled: !!p.enabled,
        available_models: p.models || [],
        selected_models: Array.isArray(p.selectedModels) ? p.selectedModels : (p.models || []),
      }));
      SaveProviderProfiles(payload).catch(() => {});
    },

    addModelToProvider: (providerId, modelName) => {
      get().setProviders(prev =>
        prev.map(p => {
          if (p.id === providerId && !p.models.includes(modelName)) {
            const nextModels = [...p.models, modelName];
            const nextSelected = [...(Array.isArray(p.selectedModels) ? p.selectedModels : p.models), modelName];
            return { ...p, models: nextModels, selectedModels: nextSelected };
          }
          return p;
        })
      );
    },

    deleteModelFromProvider: (providerId, modelName) => {
      get().setProviders(prev =>
        prev.map(p => {
          if (p.id === providerId) {
            const nextModels = p.models.filter(m => m !== modelName);
            const nextSelected = (Array.isArray(p.selectedModels) ? p.selectedModels : p.models).filter(
              m => m !== modelName
            );
            return { ...p, models: nextModels, selectedModels: nextSelected };
          }
          return p;
        })
      );
    },

    toggleModelSelection: (providerId, modelName) => {
      get().setProviders(prev =>
        prev.map(p => {
          if (p.id === providerId) {
            const currentSelected = Array.isArray(p.selectedModels) ? p.selectedModels : p.models || [];
            const isSelected = currentSelected.includes(modelName);
            const nextSelected = isSelected
              ? currentSelected.filter(m => m !== modelName)
              : [...currentSelected, modelName];
            return { ...p, selectedModels: nextSelected };
          }
          return p;
        })
      );
    },

    selectAllModels: (providerId, modelsToSelect) => {
      get().setProviders(prev =>
        prev.map(p => {
          if (p.id === providerId) {
            const target = modelsToSelect && modelsToSelect.length > 0 ? modelsToSelect : p.models || [];
            const existing = Array.isArray(p.selectedModels) ? p.selectedModels : [];
            const merged = Array.from(new Set([...existing, ...target]));
            return { ...p, selectedModels: merged };
          }
          return p;
        })
      );
    },

    deselectAllModels: (providerId, modelsToDeselect) => {
      get().setProviders(prev =>
        prev.map(p => {
          if (p.id === providerId) {
            if (!modelsToDeselect || modelsToDeselect.length === 0) {
              return { ...p, selectedModels: [] };
            }
            const existing = Array.isArray(p.selectedModels) ? p.selectedModels : p.models || [];
            const filtered = existing.filter(m => !modelsToDeselect.includes(m));
            return { ...p, selectedModels: filtered };
          }
          return p;
        })
      );
    },

    fetchProviderModels: async (providerId) => {
      const prov = get().providers.find(p => p.id === providerId);
      if (!prov) return;
      const fetched = await ApiBridge.fetchModels(prov.id, prov.baseUrl, prov.apiKey);
      if (fetched.length > 0) {
        get().setProviders(prev =>
          prev.map(p => {
            if (p.id === providerId) {
              const merged = Array.from(new Set([...p.models, ...fetched]));
              return { ...p, models: merged, selectedModels: merged };
            }
            return p;
          })
        );
      }
    },

    activateGoModel: async (model?: string) => {
      const target = (model || '').trim();
      if (!target) return;
      if (lastActivatedModel === target) return;

      let profiles = goProfilesCache;
      if (!profiles || profiles.length === 0) {
        try {
          profiles = await GetProviderProfiles();
          goProfilesCache = Array.isArray(profiles) ? profiles : [];
        } catch {
          return;
        }
      }
      const owner = (profiles || []).find((p: any) => {
        const id = p?.id ?? p?.Id;
        if (!id) return false;
        const selected = p?.selected_models || p?.SelectedModels || [];
        const available = p?.available_models || p?.AvailableModels || [];
        return selected.includes(target) || available.includes(target);
      });
      let ownerId = owner?.id ?? owner?.Id;
      if (!ownerId) {
        const feProvider = get().providers.find(p =>
          (p.selectedModels || []).includes(target) ||
          (p.models || []).includes(target)
        );
        if (feProvider) {
          ownerId = feProvider.id;
        }
      }
      if (!ownerId && target.includes('/')) {
        const prefix = target.split('/')[0];
        const match = (profiles || []).find((p: any) => (p?.id ?? p?.Id) === prefix)
          || get().providers.find(p => p.id === prefix);
        if (match) {
          ownerId = (match as any).id ?? (match as any).Id;
        }
      }
      if (ownerId) {
        try {
          await SetActiveModel(ownerId, target);
          lastActivatedModel = target;
        } catch {}
      }
    },

    hydrateProvidersFromGo: async () => {
      try {
        const profiles = await GetProviderProfiles();
        if (!Array.isArray(profiles) || profiles.length === 0) return;
        goProfilesCache = profiles;

        const deletedIds = get().deletedProviderIds;

        get().setProviders(prev => {
          const next = [...prev];
          for (const gp of profiles) {
            const id = gp?.id ?? gp?.Id;
            const url = String(gp?.base_url ?? gp?.BaseUrl ?? '').replace(/\/+$/, '');
            if (!id && !url) continue;

            if (id && deletedIds.includes(id)) {
              DeleteProviderProfile(id).catch(() => {});
              continue;
            }

            const mapped: LLMProviderConfig = {
              id: id || `go-${url || Math.random().toString(36).slice(2)}`,
              name: gp?.name ?? gp?.Name ?? id ?? 'Provider',
              apiKey: gp?.api_key ?? gp?.ApiKey ?? '',
              baseUrl: gp?.base_url ?? gp?.BaseUrl ?? '',
              enabled: !!(gp?.enabled ?? gp?.Enabled),
              models: gp?.available_models ?? gp?.AvailableModels ?? [],
              selectedModels: Array.isArray(gp?.selected_models) ? gp.selected_models : (gp?.SelectedModels ?? []),
            };
            const idx = next.findIndex(p =>
              (id && p.id === id) || (url && (p.baseUrl || '').replace(/\/+$/, '') === url)
            );
            if (idx >= 0) {
              const merged = { ...next[idx], ...mapped };
              if (JSON.stringify(merged) === JSON.stringify(next[idx])) continue;
              next[idx] = merged;
            } else {
              next.push(mapped);
            }
          }
          return next;
        });
      } catch {}
    },

    initProviderSync: () => {
      const syncWithGo = (providers: LLMProviderConfig[]) => {
        const fingerprint = JSON.stringify(providers);
        if (fingerprint === lastSyncedFingerprint) return;
        lastSyncedFingerprint = fingerprint;
        lastActivatedModel = '';

        const payload = (providers || []).map(p => ({
          id: p.id,
          name: p.name,
          api_key: p.apiKey || '',
          base_url: p.baseUrl || '',
          enabled: !!p.enabled,
          available_models: p.models || [],
          selected_models: Array.isArray(p.selectedModels) ? p.selectedModels : (p.models || []),
        }));

        SyncAgentProviders(payload)
          .then(merged => {
            if (Array.isArray(merged)) goProfilesCache = merged;
          })
          .catch(() => {});
      };

      // Initial sync
      syncWithGo(get().providers);

      // Subscribe to provider changes to re-sync
      const unsubStore = useProviderStore.subscribe(state => {
        syncWithGo(state.providers);
      });

      // Hydrate after boot sync + auto-discover local LLMs
      const t = setTimeout(() => {
        void get().hydrateProvidersFromGo();
        void get().discoverLocalProviders();
      }, 1200);

      const off = EventsOn('agent:config:changed', () => {
        setTimeout(() => {
          void get().hydrateProvidersFromGo();
        }, 300);
      });

      return () => {
        unsubStore();
        clearTimeout(t);
        if (typeof off === 'function') off();
      };
    }
  };
});
