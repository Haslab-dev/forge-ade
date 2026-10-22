import { AgentSession, ContextUsageInfo } from '../../types';

export function calculateContextUsage(
  activeSession?: AgentSession,
  currentModel?: string
): ContextUsageInfo {
  // Provider-reported usage is authoritative and free — skip the full
  // transcript scan (which re-runs ~16x/sec during streaming otherwise).
  if (activeSession?.contextTokens && activeSession.contextTokens > 0) {
    const reported = activeSession.contextTokens;
    const modelLowerFast = (activeSession.model || currentModel || '').toLowerCase();
    let maxFast = 1000000;
    if (modelLowerFast.includes('2m')) maxFast = 2000000;
    else if (modelLowerFast.includes('200k') || modelLowerFast.includes('claude')) maxFast = 200000;
    else if (
      modelLowerFast.includes('128k') ||
      modelLowerFast.includes('gpt-4') ||
      modelLowerFast.includes('o1') ||
      modelLowerFast.includes('o3') ||
      modelLowerFast.includes('deepseek')
    )
      maxFast = 128000;
    else if (modelLowerFast.includes('64k')) maxFast = 64000;
    else if (modelLowerFast.includes('32k')) maxFast = 32000;

    const pctFast = Math.min(100, Math.max(0, (reported / maxFast) * 100));
    const fmtUsed =
      reported >= 1000000
        ? `${(reported / 1000000).toFixed(2)}M`
        : reported >= 1000
        ? `${(reported / 1000).toFixed(1)}K`
        : `${reported}`;
    const fmtMax =
      maxFast >= 1000000
        ? `${(maxFast / 1000000).toFixed(0)}M`
        : `${Math.round(maxFast / 1000)}K`;
    const catAll = {
      tokens: reported,
      percent: pctFast,
      formattedPercent: pctFast < 0.1 && pctFast > 0 ? '<0.1%' : `${pctFast.toFixed(1)}%`
    };
    const catZero = { tokens: 0, percent: 0, formattedPercent: '0%' };

    return {
      usedTokens: reported,
      maxTokens: maxFast,
      percent: pctFast,
      formattedUsed: fmtUsed,
      formattedMax: fmtMax,
      categories: {
        messages: catAll,
        mcpTools: catZero,
        systemTools: catZero,
        systemPrompt: catZero,
        skills: catZero,
        metaContext: catZero
      }
    };
  }

  // Max tokens based on the selected model
  const modelLower = (activeSession?.model || currentModel || '').toLowerCase();
  let maxTokens = 1000000;
  if (modelLower.includes('2m')) maxTokens = 2000000;
  else if (modelLower.includes('200k') || modelLower.includes('claude')) maxTokens = 200000;
  else if (
    modelLower.includes('128k') ||
    modelLower.includes('gpt-4') ||
    modelLower.includes('o1') ||
    modelLower.includes('o3') ||
    modelLower.includes('deepseek')
  )
    maxTokens = 128000;
  else if (modelLower.includes('64k')) maxTokens = 64000;
  else if (modelLower.includes('32k')) maxTokens = 32000;

  let usedTokens = 0;
  if (activeSession?.contextTokens && activeSession.contextTokens > 0) {
    usedTokens = activeSession.contextTokens;
  } else if (activeSession?.messages) {
    let chars = 0;
    for (const m of activeSession.messages) {
      chars += m.content?.length || 0;
      for (const th of m.thoughts || []) chars += th.thoughtText?.length || 0;
      for (const te of m.toolExecutions || [])
        chars += (te.command?.length || 0) + (te.output?.length || 0);
    }
    for (const d of activeSession.diffs || []) {
      chars += (d.originalContent?.length || 0) + (d.modifiedContent?.length || 0);
    }
    usedTokens = Math.round(chars / 4);
  }

  const percent = Math.min(100, Math.max(0, (usedTokens / maxTokens) * 100));
  const formattedUsed =
    usedTokens >= 1000000
      ? `${(usedTokens / 1000000).toFixed(2)}M`
      : usedTokens >= 1000
      ? `${(usedTokens / 1000).toFixed(1)}K`
      : `${usedTokens}`;
  const formattedMax =
    maxTokens >= 1000000
      ? `${(maxTokens / 1000000).toFixed(0)}M`
      : `${Math.round(maxTokens / 1000)}K`;

  const calcCat = (toks: number) => {
    const pct = usedTokens > 0 ? (toks / usedTokens) * 100 : 0;
    return {
      tokens: toks,
      percent: pct,
      formattedPercent: toks === 0 ? '0%' : pct < 0.1 ? '<0.1%' : `${pct.toFixed(1)}%`
    };
  };

  return {
    usedTokens,
    maxTokens,
    percent,
    formattedUsed,
    formattedMax,
    categories: {
      messages: calcCat(usedTokens),
      mcpTools: calcCat(0),
      systemTools: calcCat(0),
      systemPrompt: calcCat(0),
      skills: calcCat(0),
      metaContext: calcCat(0)
    }
  };
}
