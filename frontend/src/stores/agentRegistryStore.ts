import { ACPAgent, PrivacySettings, LLMProviderConfig } from '../types';

export const DEFAULT_AGENTS: ACPAgent[] = [
  {
    id: 'agent-internal',
    name: 'ForgeADE Internal',
    type: 'internal',
    description: 'Internal autonomous agent engine supporting multi-turn planning, real terminal execution, file editing, and direct LLM provider reasoning.',
    icon: 'forge-ade',
    isDefault: true,
    isStarred: true,
    enabled: true,
    status: 'connected',
    provider: 'custom',
    supportedModels: [
      'claude-3-7-sonnet-20250219',
      'claude-3-5-sonnet-20241022',
      'gpt-4o',
      'gemini-2.0-flash',
      'deepseek-r1',
      'qwen2.5-coder:latest'
    ]
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

export const DEFAULT_PROVIDERS: LLMProviderConfig[] = [
  {
    id: 'prov-ollama',
    name: 'Ollama (Local)',
    baseUrl: 'http://localhost:11434',
    enabled: true,
    models: [
      'qwen2.5-coder:latest',
      'deepseek-r1:latest',
      'llama3.3:latest',
      'mistral:latest',
      'codellama:latest'
    ],
    selectedModels: [
      'qwen2.5-coder:latest',
      'deepseek-r1:latest',
      'llama3.3:latest'
    ]
  },
  {
    id: 'prov-anthropic',
    name: 'Anthropic Claude',
    baseUrl: 'https://api.anthropic.com/v1',
    enabled: true,
    models: [
      'claude-3-7-sonnet-20250219',
      'claude-3-5-sonnet-20241022',
      'claude-3-5-haiku-20241022',
      'claude-3-opus-20240229'
    ],
    selectedModels: [
      'claude-3-7-sonnet-20250219',
      'claude-3-5-sonnet-20241022'
    ]
  },
  {
    id: 'prov-openai',
    name: 'OpenAI',
    baseUrl: 'https://api.openai.com/v1',
    enabled: true,
    models: [
      'gpt-4o',
      'gpt-4o-mini',
      'o3-mini',
      'o1',
      'o1-preview'
    ],
    selectedModels: [
      'gpt-4o',
      'gpt-4o-mini',
      'o3-mini'
    ]
  },
  {
    id: 'prov-google',
    name: 'Google Gemini',
    baseUrl: 'https://generativelanguage.googleapis.com',
    enabled: true,
    models: [
      'gemini-2.0-flash',
      'gemini-2.0-flash-thinking-exp',
      'gemini-1.5-pro',
      'gemini-1.5-flash'
    ],
    selectedModels: [
      'gemini-2.0-flash',
      'gemini-1.5-pro'
    ]
  },
  {
    id: 'prov-openrouter',
    name: 'OpenRouter',
    baseUrl: 'https://openrouter.ai/api/v1',
    enabled: true,
    models: [
      'anthropic/claude-3.7-sonnet',
      'deepseek/deepseek-r1',
      'openai/gpt-4o',
      'qwen/qwen-2.5-coder-32b-instruct',
      'meta-llama/llama-3.3-70b-instruct'
    ],
    selectedModels: [
      'anthropic/claude-3.7-sonnet',
      'deepseek/deepseek-r1'
    ]
  },
  {
    id: 'prov-deepseek',
    name: 'DeepSeek',
    baseUrl: 'https://api.deepseek.com/v1',
    enabled: true,
    models: [
      'deepseek-chat',
      'deepseek-reasoner'
    ],
    selectedModels: [
      'deepseek-chat',
      'deepseek-reasoner'
    ]
  }
];

export const DEFAULT_PRIVACY: PrivacySettings = {
  shareTerminalActivity: true,
  shareUserEdits: true
};
