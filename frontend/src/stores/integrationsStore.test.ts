import { describe, it, expect, beforeEach, vi } from 'vitest';
import { useIntegrationsStore } from './integrationsStore';
import { ApiBridge } from '../services/apiBridge';

vi.mock('../services/apiBridge', () => ({
  ApiBridge: {
    listMCPServers: vi.fn(async () => []),
    listConnectedMCPTools: vi.fn(async () => []),
    listSkills: vi.fn(async () => []),
    listPlugins: vi.fn(async () => []),
    listMemories: vi.fn(async () => []),
    listAutomations: vi.fn(async () => []),
    saveMemory: vi.fn(),
    deleteMemory: vi.fn(),
    reloadMemories: vi.fn(async () => [])
  }
}));

vi.mock('../lib/wails', () => ({
  SaveMCPServer: vi.fn(),
  DeleteMCPServer: vi.fn(),
  EventsOn: vi.fn(() => () => {})
}));

describe('integrationsStore', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('initializes with default skills and commands', () => {
    const state = useIntegrationsStore.getState();
    expect(state.skills.length).toBeGreaterThan(0);
    expect(state.customCommands.length).toBeGreaterThan(0);
    expect(state.browserSettings.enabled).toBe(true);
  });

  it('adds and deletes a skill', () => {
    const store = useIntegrationsStore.getState();
    const newSkill = {
      id: 'test-skill',
      name: 'test-skill',
      category: 'Testing',
      description: 'Testing skill',
      trigger: 'test-skill',
      enabled: true
    };

    store.addSkill(newSkill);
    expect(useIntegrationsStore.getState().skills.some(s => s.id === 'test-skill')).toBe(true);

    store.deleteSkill('test-skill');
    expect(useIntegrationsStore.getState().skills.some(s => s.id === 'test-skill')).toBe(false);
  });

  it('updates browser and computer settings', () => {
    const store = useIntegrationsStore.getState();
    store.updateBrowserSettings({ headless: false });
    expect(useIntegrationsStore.getState().browserSettings.headless).toBe(false);

    store.updateComputerSettings({ commandTimeoutSeconds: 60 });
    expect(useIntegrationsStore.getState().computerSettings.commandTimeoutSeconds).toBe(60);
  });
});
