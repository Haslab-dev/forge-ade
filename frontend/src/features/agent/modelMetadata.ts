export interface ModelMeta {
  id: string;
  displayName: string;
  provider: string;
  providerKey: string;
  contextWindow?: string;
  isReasoning?: boolean;
  isFast?: boolean;
  isFlagship?: boolean;
  isVision?: boolean;
  agentId?: string;
  agentName?: string;
}

export function parseModelMetadata(modelId: string, providerHint?: string, agentHint?: string): ModelMeta {
  const raw = modelId || '';
  const lower = raw.toLowerCase();

  let displayName = raw;
  let provider = providerHint || 'Custom';
  let providerKey = (providerHint || 'custom').toLowerCase();
  let contextWindow = '128k';
  let isReasoning = false;
  let isFast = false;
  let isFlagship = false;
  let isVision = false;

  // Anthropic Claude
  if (lower.includes('claude-3-7') || lower.includes('3.7-sonnet') || lower.includes('sonnet-3.7')) {
    displayName = 'Claude 3.7 Sonnet';
    provider = 'Anthropic';
    providerKey = 'claude';
    contextWindow = '200k';
    isFlagship = true;
    isReasoning = true;
    isVision = true;
  } else if (lower.includes('claude-3-5-sonnet') || lower.includes('sonnet-3.5') || lower === 'sonnet') {
    displayName = 'Claude 3.5 Sonnet';
    provider = 'Anthropic';
    providerKey = 'claude';
    contextWindow = '200k';
    isFlagship = true;
    isVision = true;
  } else if (lower.includes('claude-3-5-haiku') || lower.includes('haiku')) {
    displayName = 'Claude 3.5 Haiku';
    provider = 'Anthropic';
    providerKey = 'claude';
    contextWindow = '200k';
    isFast = true;
  } else if (lower.includes('opus')) {
    displayName = 'Claude 3 Opus';
    provider = 'Anthropic';
    providerKey = 'claude';
    contextWindow = '200k';
    isFlagship = true;
  }
  // OpenAI
  else if (lower === 'gpt-4o' || lower.includes('gpt-4o-2024')) {
    displayName = 'GPT-4o (Omni)';
    provider = 'OpenAI';
    providerKey = 'codex';
    contextWindow = '128k';
    isFlagship = true;
    isVision = true;
  } else if (lower.includes('gpt-4o-mini')) {
    displayName = 'GPT-4o Mini';
    provider = 'OpenAI';
    providerKey = 'codex';
    contextWindow = '128k';
    isFast = true;
  } else if (lower.includes('o3-mini')) {
    displayName = 'o3-mini (Reasoning)';
    provider = 'OpenAI';
    providerKey = 'codex';
    contextWindow = '200k';
    isReasoning = true;
    isFast = true;
  } else if (lower === 'o1' || lower.includes('o1-preview')) {
    displayName = 'o1 (Deep Reasoning)';
    provider = 'OpenAI';
    providerKey = 'codex';
    contextWindow = '200k';
    isReasoning = true;
    isFlagship = true;
  }
  // Google Gemini
  else if (lower.includes('gemini-2.5-pro') || lower.includes('gemini-1.5-pro')) {
    displayName = 'Gemini 2.5 Pro';
    provider = 'Google Gemini';
    providerKey = 'gemini';
    contextWindow = '1M';
    isFlagship = true;
    isReasoning = true;
    isVision = true;
  } else if (lower.includes('gemini-2.0-flash') || lower.includes('gemini-2.5-flash') || lower.includes('gemini-3.8-flash')) {
    displayName = lower.includes('high') ? 'Gemini 3.8 Flash (High)' : 'Gemini 2.5 Flash';
    provider = 'Google Gemini';
    providerKey = 'gemini';
    contextWindow = '1M';
    isFast = true;
    isVision = true;
  }
  // DeepSeek
  else if (lower.includes('deepseek-r1') || lower.includes('deepseek-reasoner')) {
    displayName = 'DeepSeek R1 (Reasoning)';
    provider = 'DeepSeek';
    providerKey = 'deepseek';
    contextWindow = '64k';
    isReasoning = true;
    isFlagship = true;
  } else if (lower.includes('deepseek-v3') || lower.includes('deepseek-chat')) {
    displayName = 'DeepSeek V3';
    provider = 'DeepSeek';
    providerKey = 'deepseek';
    contextWindow = '64k';
    isFast = true;
  }
  // GLM / Kimi / Qwen
  else if (lower.includes('glm')) {
    displayName = raw.split('/').pop() || raw;
    provider = 'Zhipu GLM';
    providerKey = 'claude';
  } else if (lower.includes('qwen') || lower.includes('qwopus')) {
    displayName = raw.split('/').pop() || raw;
    provider = 'Qwen';
    providerKey = 'custom';
  } else {
    // Clean slash prefixes
    const parts = raw.split('/');
    if (parts.length > 1) {
      provider = parts[0].toUpperCase();
      displayName = parts.slice(1).join('/');
    }
  }

  return {
    id: raw,
    displayName,
    provider,
    providerKey,
    contextWindow,
    isReasoning,
    isFast,
    isFlagship,
    isVision,
    agentId: agentHint,
  };
}
