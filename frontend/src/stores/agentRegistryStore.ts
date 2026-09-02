import { ACPAgent, PrivacySettings, LLMProviderConfig } from '../types';

export const DEFAULT_AGENTS: ACPAgent[] = [
  {
    id: 'agent-internal',
    name: 'My-ADE Internal',
    type: 'internal',
    description: 'Internal autonomous agent engine supporting multi-turn planning, real terminal execution, file editing, and direct LLM provider reasoning.',
    icon: 'my-ade',
    isDefault: true,
    isStarred: true,
    enabled: true,
    status: 'connected',
    provider: 'custom',
    supportedModels: []
  },
  {
    id: 'agent-pi',
    name: 'Pi Agent',
    type: 'pi',
    description: 'Autonomous core agent implementing the Pi protocol with comprehensive codebase research, live terminal orchestration, and adaptive tools.',
    icon: 'pi',
    isDefault: false,
    isStarred: true,
    enabled: true,
    status: 'disconnected',
    provider: 'pi',
    endpoint: 'stdio://pi',
    supportedModels: []
  },
  {
    id: 'agent-ohmypi',
    name: 'OhMyPi (omp)',
    type: 'ohmypi',
    description: 'High-throughput ACP agent with multi-turn reasoning and parallel tool execution for fast iteration.',
    icon: 'ohmypi',
    isDefault: false,
    isStarred: true,
    enabled: true,
    provider: 'ohmypi',
    endpoint: 'ws://127.0.0.1:3001/acp',
    supportedModels: []
  },
  {
    id: 'agent-opencode',
    name: 'OpenCode Agent',
    type: 'opencode',
    description: 'Open-source coding agent connected via Agent Client Protocol (ACP) supporting local and cloud LLM execution.',
    icon: 'opencode',
    isDefault: false,
    isStarred: false,
    enabled: true,
    provider: 'opencode',
    endpoint: 'ws://127.0.0.1:3002/acp',
    supportedModels: []
  }
];

// No hardcoded default providers - all providers are added by the user
export const DEFAULT_PROVIDERS: LLMProviderConfig[] = [];

export const DEFAULT_PRIVACY: PrivacySettings = {
  shareTerminalActivity: true,
  shareUserEdits: true
};
