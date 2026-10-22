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
    id: 'agent-agy',
    name: 'Google Antigravity',
    type: 'acp',
    description: 'Google Antigravity (AGY) official coding agent connected via Agent Client Protocol (ACP).',
    icon: 'antigravity',
    isDefault: false,
    isStarred: true,
    enabled: true,
    status: 'connected',
    provider: 'agy',
    command: 'agy',
    args: ['--input-format', 'stream-json', '--output-format', 'stream-json'],
    model: 'gemini-3.8-flash-high',
    supportedModels: [
      'gemini-3.8-flash-high',
      'gemini-3.8-flash-medium',
      'gemini-3.8-flash-low',
      'claude-sonnet-4-6',
      'claude-opus-4-6-thinking',
      'gpt-oss-120b-medium'
    ],
    supportedEfforts: ['low', 'medium', 'high', 'xhigh', 'max'],
    defaultEffort: 'high',
    commands: [
      { name: 'goal', description: 'Set and track high-level autonomous goal', hint: '<description>' },
      { name: 'plan', description: 'Create and review structured implementation plan', hint: '<objective>' },
      { name: 'compact', description: 'Compact conversation context', hint: '' },
      { name: 'schedule', description: 'Schedule recurring or delayed tasks', hint: '' },
      { name: 'browser', description: 'Open and interact with web applications', hint: '' },
      { name: 'boost', description: 'Deep thinking and rigorous verification mode', hint: '' }
    ]
  },
  {
    id: 'agent-claude',
    name: 'Claude Code',
    type: 'claude',
    description: 'Anthropic Claude Code CLI harness driving local shell, tools, and reasoning over stream-json protocol.',
    icon: 'claude',
    isDefault: false,
    isStarred: true,
    enabled: true,
    status: 'connected',
    provider: 'claude',
    command: 'claude',
    args: ['-p', '--input-format', 'stream-json', '--output-format', 'stream-json', '--verbose', '--include-partial-messages'],
    model: 'sonnet',
    supportedModels: [
      'sonnet',
      'glm-5.3-flash',
      'opus',
      'haiku',
      'claude-opus-5-5',
      'claude-fable-5-1[1m]'
    ],
    supportedEfforts: ['low', 'medium', 'high', 'xhigh'],
    defaultEffort: 'high',
    commands: [
      { name: 'cost', description: 'Show token usage and cost breakdown', hint: '' },
      { name: 'compact', description: 'Compact conversation history', hint: '' },
      { name: 'doctor', description: 'Check health and configuration of environment', hint: '' },
      { name: 'review', description: 'Review code changes and git status', hint: '' },
      { name: 'bug', description: 'Report a bug or unexpected behavior', hint: '' },
      { name: 'pr_comments', description: 'Fetch and address PR review comments', hint: '' }
    ]
  },
  {
    id: 'agent-codex',
    name: 'OpenAI Codex',
    type: 'codex',
    description: 'OpenAI Codex agent harness driven through JSON-RPC 2.0 app-server over stdio.',
    icon: 'codex',
    isDefault: false,
    isStarred: true,
    enabled: true,
    status: 'connected',
    provider: 'codex',
    command: 'codex',
    args: ['app-server', '--listen', 'stdio://'],
    model: 'deepseek-v4-1-flash',
    supportedModels: [
      'deepseek-v4-1-flash',
      'gpt-6.1-sol',
      'gpt-6-luna',
      'gpt-5.6-terra',
      'gpt-5.6-luna',
      'gpt-5.5'
    ],
    supportedEfforts: ['low', 'medium', 'high', 'max'],
    defaultEffort: 'medium',
    commands: [
      { name: 'review', description: 'Review changes and proposed patches', hint: '' },
      { name: 'test', description: 'Generate and execute test suites', hint: '' },
      { name: 'compact', description: 'Compact conversation history', hint: '' }
    ]
  },
  {
    id: 'agent-opencode',
    name: 'OpenCode',
    type: 'opencode',
    description: 'OpenCode autonomous agent connected via Agent Client Protocol (ACP).',
    icon: 'opencode',
    isDefault: false,
    isStarred: true,
    enabled: true,
    status: 'connected',
    provider: 'opencode',
    command: 'opencode',
    args: ['acp'],
    model: 'Kenari/glm-5-3-flash',
    supportedModels: [
      'Kenari/glm-5-3-flash',
      'Kenari/deepseek-v4-1-flash',
      'Kenari/claude-haiku-5-5',
      'Kenari/gpt-6-luna',
      'inadigital/inadigital-flash',
      'inadigital/inadigital-pro',
      'deepseek/deepseek-flash',
      'deepseek/deepseek-v4-pro',
      'opencode/claude-sonnet-5-5',
      'opencode/deepseek-v4.1-flash',
      'opencode/gpt-6-astra'
    ],
    supportedEfforts: ['low', 'medium', 'high'],
    defaultEffort: 'medium',
    commands: [
      { name: 'init', description: 'Initialize OpenCode configuration', hint: '' },
      { name: 'compact', description: 'Compact session context', hint: '' },
      { name: 'review', description: 'Review modified code and diffs', hint: '' }
    ]
  },
  {
    id: 'agent-commandcode',
    name: 'Command Code',
    type: 'commandcode',
    description: 'Command Code autonomous agent with taste learning connected via Agent Client Protocol (ACP).',
    icon: 'commandcode',
    isDefault: false,
    isStarred: true,
    enabled: true,
    status: 'connected',
    provider: 'commandcode',
    command: 'cmd',
    args: ['acp'],
    model: 'deepseek/deepseek-v4.1-flash',
    supportedModels: [
      'deepseek/deepseek-v4.1-flash',
      'deepseek/deepseek-v4-flash',
      'deepseek/deepseek-v4-pro',
      'claude-sonnet-5-5',
      'claude-sonnet-5',
      'claude-opus-5-5',
      'claude-haiku-5-5',
      'gpt-6-astra',
      'gpt-6.1-sol',
      'gpt-6-luna',
      'gpt-5.6-sol',
      'google/gemini-3.8-flash',
      'google/gemini-4-argon',
      'moonshotai/kimi-k3',
      'z-ai/glm-5.3-flash',
      'qwen/qwen3.8-max',
      'qwen/qwen3.8-flash',
      'minimax/minimax-m3.1-flash:free',
      'meta/muse-spark-1.3',
      'xai/grok-4.5',
      'mistral/mistral-large-4'
    ],
    supportedEfforts: ['off', 'low', 'medium', 'high', 'max'],
    defaultEffort: 'medium',
    commands: [
      { name: 'usage', description: 'Check token usage, costs, and context metrics', hint: '' },
      { name: 'compact', description: 'Compress previous conversation context', hint: '' },
      { name: 'init', description: 'Initialize Command Code repository configuration', hint: '' },
      { name: 'cost', description: 'Display estimated session and cumulative cost', hint: '' },
      { name: 'status', description: 'Show agent and workspace status', hint: '' }
    ]
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
    model: 'Kenari/glm-5-3-flash',
    supportedModels: [
      'Kenari/glm-5-3-flash',
      'Kenari/claude-haiku-5-5',
      'Kenari/mimo-v2-5',
      'Kenari/deepseek-v4-1-flash',
      'Kenari/gpt-6-luna',
      'inadigital/inadigital-flash',
      'inadigital/inadigital-pro'
    ],
    supportedEfforts: ['off', 'minimal', 'low', 'medium', 'high'],
    defaultEffort: 'medium',
    commands: [
      { name: 'usage', description: 'Check token usage and context metrics', hint: '' },
      { name: 'compact', description: 'Compact session context', hint: '' },
      { name: 'cost', description: 'Display token cost breakdown', hint: '' }
    ]
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
    model: 'kenari/gpt-6-luna',
    supportedModels: [
      'kenari/gpt-6-luna',
      'kenari/claude-haiku-5-5',
      'kenari/mimo-v2.6-flash',
      'kenari/glm-5.3-flash',
      'kenari/glm-coding/glm-5.3-flash',
      'inadigital/inadigital-flash',
      'inadigital/inadigital-pro',
      'research_kantor/pc-eyay/Qwopus3.6-27B-v2-MTP-NVFP4-GGUF.gguf'
    ],
    supportedEfforts: ['off', 'minimal', 'low', 'medium', 'high'],
    defaultEffort: 'medium',
    commands: [
      { name: 'usage', description: 'Check token usage and context metrics', hint: '' },
      { name: 'compact', description: 'Compact session context', hint: '' },
      { name: 'cost', description: 'Display token cost breakdown', hint: '' }
    ]
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
    supportedModels: [],
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
