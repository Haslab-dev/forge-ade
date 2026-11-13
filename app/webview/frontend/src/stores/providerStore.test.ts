import { describe, it, expect, vi, beforeEach } from 'vitest';
import { useProviderStore } from './providerStore';
import { ApiBridge } from '../services/apiBridge';

vi.mock('../services/apiBridge', () => ({
  ApiBridge: {
    fetchModels: vi.fn().mockResolvedValue(['model-alpha', 'model-beta']),
    discoverLocalLLMs: vi.fn().mockResolvedValue([
      {
        id: 'prov-ollama',
        name: 'Ollama (Local Offline)',
        baseUrl: 'http://127.0.0.1:11434',
        running: true,
        models: ['qwen2.5-coder:latest']
      }
    ])
  }
}));

vi.mock('../lib/wails', () => ({
  EventsOn: vi.fn(() => () => {}),
  GetProviderProfiles: vi.fn().mockResolvedValue([]),
  SetActiveModel: vi.fn().mockResolvedValue(undefined),
  SyncAgentProviders: vi.fn().mockResolvedValue([]),
  SaveProviderProfiles: vi.fn().mockResolvedValue(undefined),
  DeleteProviderProfile: vi.fn().mockResolvedValue(undefined)
}));

describe('providerStore', () => {
  beforeEach(() => {
    useProviderStore.setState({
      providers: [
        {
          id: 'prov-1',
          name: 'OpenAI',
          apiKey: 'key-123',
          baseUrl: 'https://api.openai.com/v1',
          enabled: true,
          models: ['gpt-4o', 'gpt-4o-mini'],
          selectedModels: ['gpt-4o']
        }
      ],
      deletedProviderIds: []
    });
  });

  it('adds and updates a provider', () => {
    const store = useProviderStore.getState();
    store.addProvider({
      id: 'prov-2',
      name: 'Anthropic',
      apiKey: 'ant-key',
      baseUrl: 'https://api.anthropic.com',
      enabled: true,
      models: ['claude-3-5-sonnet'],
      selectedModels: ['claude-3-5-sonnet']
    });

    expect(useProviderStore.getState().providers).toHaveLength(2);

    store.updateProvider('prov-2', { name: 'Anthropic Claude' });
    const updated = useProviderStore.getState().providers.find(p => p.id === 'prov-2');
    expect(updated?.name).toBe('Anthropic Claude');
  });

  it('toggles model selection and selects all models', () => {
    const store = useProviderStore.getState();
    store.toggleModelSelection('prov-1', 'gpt-4o-mini');
    let prov = useProviderStore.getState().providers.find(p => p.id === 'prov-1');
    expect(prov?.selectedModels).toContain('gpt-4o-mini');

    store.deselectAllModels('prov-1');
    prov = useProviderStore.getState().providers.find(p => p.id === 'prov-1');
    expect(prov?.selectedModels).toHaveLength(0);

    store.selectAllModels('prov-1');
    prov = useProviderStore.getState().providers.find(p => p.id === 'prov-1');
    expect(prov?.selectedModels).toEqual(['gpt-4o', 'gpt-4o-mini']);
  });

  it('fetches remote models and merges them', async () => {
    const store = useProviderStore.getState();
    await store.fetchProviderModels('prov-1');

    const prov = useProviderStore.getState().providers.find(p => p.id === 'prov-1');
    expect(ApiBridge.fetchModels).toHaveBeenCalledWith('prov-1', 'https://api.openai.com/v1', 'key-123');
    expect(prov?.models).toContain('model-alpha');
    expect(prov?.models).toContain('model-beta');
  });

  it('discovers local LLM providers and merges into store', async () => {
    const store = useProviderStore.getState();
    const services = await store.discoverLocalProviders();

    expect(services).toHaveLength(1);
    expect(services[0].id).toBe('prov-ollama');
    expect(useProviderStore.getState().localServices).toHaveLength(1);

    const ollamaProv = useProviderStore.getState().providers.find(p => p.id === 'prov-ollama');
    expect(ollamaProv).toBeDefined();
    expect(ollamaProv?.models).toContain('qwen2.5-coder:latest');
    expect(ollamaProv?.enabled).toBe(true);
  });
});
