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
    ],
    supportedEfforts: ['low', 'medium', 'high', 'max'],
    defaultEffort: 'medium'
  },
  {
    id: 'agent-claude',
    name: 'Claude Code',
    type: 'claude',
    description: 'Anthropic Claude Code CLI harness driving local shell, tools, and reasoning over NDJSON stdio protocol.',
    icon: 'claude',
    isDefault: false,
    isStarred: true,
    enabled: true,
    status: 'connected',
    provider: 'claude',
    command: 'claude',
    args: [],
    supportedModels: [
      'claude-3-7-sonnet-20250219',
      'claude-3-5-sonnet-20241022',
      'claude-3-5-haiku-20241022',
      'claude-3-opus-20240229'
    ],
    supportedEfforts: ['low', 'medium', 'high', 'xhigh'],
    defaultEffort: 'high'
  },
  {
    id: 'agent-codex',
    name: 'Codex',
    type: 'codex',
    description: 'OpenAI Codex agent harness driven through JSON-RPC 2.0 app-server over stdio.',
    icon: 'codex',
    isDefault: false,
    isStarred: true,
    enabled: true,
    status: 'connected',
    provider: 'codex',
    command: 'codex',
    args: ['app-server'],
    supportedModels: [
      'gpt-5.6-sol',
      'gpt-5.5',
      'o3',
      'o3-mini',
      'gpt-4o'
    ],
    supportedEfforts: ['low', 'medium', 'high', 'max'],
    defaultEffort: 'medium'
  },
  {
    id: 'agent-opencode',
    name: 'OpenCode',
    type: 'opencode',
    description: 'OpenCode autonomous agent connected via local HTTP/SSE server or Agent Client Protocol.',
    icon: 'opencode',
    isDefault: false,
    isStarred: true,
    enabled: true,
    status: 'connected',
    provider: 'opencode',
    command: 'opencode',
    args: ['acp'],
    supportedModels: [
      'deepseek/deepseek-flash',
      'qwen2.5-coder:latest',
      'deepseek-r1:latest',
      'claude-3-7-sonnet',
      'gpt-4o'
    ],
    supportedEfforts: ['low', 'medium', 'high'],
    defaultEffort: 'medium'
  },
  {
    id: 'agent-pi',
    name: 'Pi Agent',
    type: 'pi',
    description: 'Autonomous core agent implementing the Pi protocol with codebase research and live terminal orchestration.',
    icon: 'pi',
    isDefault: false,
    isStarred: true,
    enabled: true,
    status: 'connected',
    provider: 'pi',
    command: 'pi-acp',
    args: [],
    supportedModels: [
      'sumopod/gpt-5.6-luna',
      'cerebras/qwen-3.8-27b',
      'kenari/glm-5-3-flash'
    ],
    supportedEfforts: ['off', 'minimal', 'low', 'medium', 'high'],
    defaultEffort: 'medium'
  },
  {
    id: 'agent-ohmypi',
    name: 'Oh My Pi (omp)',
    type: 'ohmypi',
    description: 'High-throughput agent fork with multi-turn reasoning and parallel tool execution.',
    icon: 'ohmypi',
    isDefault: false,
    isStarred: true,
    enabled: true,
    status: 'connected',
    provider: 'ohmypi',
    command: 'omp',
    args: ['acp'],
    supportedModels: [
      'myairouter/kenari/glm-5-3-flash',
      'myairouter/cerebras/qwen-3.8-27b',
      'myairouter/sumopod/gpt-5.6-luna',
      'ina/inadigital-flash',
      'ina/inadigital-pro'
    ],
    supportedEfforts: ['off', 'minimal', 'low', 'medium', 'high'],
    defaultEffort: 'medium'
  },
  {
    id: 'agent-cursor',
    name: 'Cursor CLI',
    type: 'cursor',
    description: 'Cursor agent harness communicating over Agent Client Protocol (ACP) stdio transport.',
    icon: 'cursor',
    isDefault: false,
    isStarred: true,
    enabled: true,
    status: 'connected',
    provider: 'cursor',
    command: 'agent',
    args: ['acp'],
    supportedModels: [
      'auto',
      'gpt-5.4-high'
    ],
    supportedEfforts: ['low', 'medium', 'high'],
    defaultEffort: 'medium'
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
