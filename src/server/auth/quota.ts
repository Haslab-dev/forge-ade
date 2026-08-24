// Live quota & usage fetchers for providers (Google Antigravity, OpenCode, Kilo)
// and aggregate workspace token statistics.

import fs from "fs";
import path from "path";
import os from "os";
import { refreshGoogleToken } from "./oauth";

export interface QuotaWindowLimit {
  /** Backend counter family from the API: "Anthropic" | "Google" | "OpenAI". */
  counter?: string | undefined;
  windowId: string;
  windowLabel: string;
  remainingFraction?: number | undefined;
  resetTime?: string | undefined;
  tier?: string | undefined;
}

export interface ModelQuota {
  model: string;
  displayName: string;
  counter?: string | undefined;
  /** Deduped quota windows for this model, worst (lowest remaining) first. */
  windows: QuotaWindowLimit[];
  // Flat worst-across-windows view kept for existing consumers.
  remainingFraction?: number | undefined;
  percentageLeft?: number | undefined;
  resetTime?: string | undefined;
  tier?: string | undefined;
}

export interface ProviderQuotaReport {
  provider: string;
  accountEmail?: string | undefined;
  projectId?: string | undefined;
  tier?: string | undefined;
  fetchedAt: number;
  models: ModelQuota[];
  /** Deduped per-counter/per-window limits shared across models, worst first. */
  limits: QuotaWindowLimit[];
}

export interface UsageSummary {
  totalPromptTokens: number;
  totalCompletionTokens: number;
  totalCachedTokens: number;
  totalTokens: number;
  totalCost?: number | undefined;
  cacheHitRate: number;
  requestCount: number;
  byProvider: Record<string, { prompt: number; completion: number; cached: number; requests: number }>;
  byModel: Record<string, { prompt: number; completion: number; cached: number; requests: number }>;
  byWorkspace: Record<string, { prompt: number; completion: number; cached: number; requests: number }>;
  liveQuota?: ProviderQuotaReport | null | undefined;
}
let cachedAntigravityVersion = "2.9.1";

export function getAntigravityUserAgent(): string {
  const os = process.platform === "win32" ? "windows" : process.platform;
  const arch = process.arch === "x64" ? "x64" : "arm64";
  return `antigravity/hub/${cachedAntigravityVersion} (aidev_client; os_type=${os}; arch=${arch}; cl=963137146)`;
}

// Update version from Google manifest in background
try {
  fetch("https://antigravity-hub-auto-updater-974169037036.us-central1.run.app/manifest/latest-arm64-mac.yml", {
    signal: AbortSignal.timeout(4000),
  })
    .then((r) => r.text())
    .then((txt) => {
      const m = /^\s*version\s*:\s*([^\s#]+)/m.exec(txt);
      if (m?.[1]) cachedAntigravityVersion = m[1].replace(/['"]/g, "").trim();
    })
    .catch(() => {});
} catch {}

const ANTIGRAVITY_ENDPOINT = "https://daily-cloudcode-pa.googleapis.com";

const DAY_MS = 24 * 60 * 60 * 1000;
const WEEK_MS = 7 * DAY_MS;

interface RawQuotaInfo {
  remainingFraction?: number;
  resetTime?: string;
  tier?: string;
  windowId?: string;
  windowLabel?: string;
  apiProvider?: string;
  modelProvider?: string;
}

function clampQuotaFraction(value: number | undefined): number | undefined {
  if (value === undefined || !Number.isFinite(value)) return undefined;
  return Math.min(Math.max(value, 0), 1);
}

function classifyWindow(id: string | undefined, label: string | undefined): { id: string; label: string } | undefined {
  const source = `${id ?? ""} ${label ?? ""}`.toLowerCase();
  if (source.includes("week") || source.includes("7d") || /7[\s_-]*day/.test(source)) {
    return { id: "weekly", label: "Weekly" };
  }
  if (source.includes("day") || source.includes("daily") || source.includes("24h")) {
    return { id: "daily", label: "Daily" };
  }
  if (id || label) return { id: id ?? label ?? "default", label: label ?? id ?? "Default" };
  return undefined;
}

function inferWindowFromReset(resetAt: number | undefined, nowMs: number): { id: string; label: string } {
  if (resetAt !== undefined && resetAt - nowMs > DAY_MS) return { id: "weekly", label: "Weekly" };
  return { id: "daily", label: "Daily" };
}

function quotaCounterName(info: RawQuotaInfo): string | undefined {
  switch (info.modelProvider ?? info.apiProvider) {
    case "MODEL_PROVIDER_ANTHROPIC":
    case "API_PROVIDER_ANTHROPIC_VERTEX":
      return "Anthropic";
    case "MODEL_PROVIDER_GOOGLE":
    case "API_PROVIDER_GOOGLE_GEMINI":
      return "Google";
    case "MODEL_PROVIDER_OPENAI":
    case "API_PROVIDER_OPENAI_VERTEX":
      return "OpenAI";
    default:
      return undefined;
  }
}

type QuotaInfoValue = RawQuotaInfo | RawQuotaInfo[];

interface RawModelInfo {
  displayName?: string;
  apiProvider?: string;
  modelProvider?: string;
  quotaInfo?: QuotaInfoValue;
  quotaInfos?: QuotaInfoValue;
  dailyQuotaInfo?: QuotaInfoValue;
  dailyQuotaInfos?: QuotaInfoValue;
  weeklyQuotaInfo?: QuotaInfoValue;
  weeklyQuotaInfos?: QuotaInfoValue;
  quotaInfoByTier?: Record<string, QuotaInfoValue>;
  quotaInfoByWindow?: Record<string, QuotaInfoValue>;
  quotaInfosByWindow?: Record<string, QuotaInfoValue>;
}

/** Flattens every quota field shape the fetchAvailableModels response uses into a list. */
function normalizeQuotaInfos(info: RawModelInfo): RawQuotaInfo[] {
  const results: RawQuotaInfo[] = [];
  const source = {
    ...(info.apiProvider ? { apiProvider: info.apiProvider } : {}),
    ...(info.modelProvider ? { modelProvider: info.modelProvider } : {}),
  };
  const addValue = (
    value: QuotaInfoValue | undefined,
    tier?: string,
    windowDescriptor?: { id: string; label: string }
  ) => {
    if (!value) return;
    const entries = Array.isArray(value) ? value : [value];
    for (const entry of entries) {
      results.push({
        ...source,
        ...entry,
        windowId: entry.windowId ?? windowDescriptor?.id,
        windowLabel: entry.windowLabel ?? windowDescriptor?.label,
        ...(tier ? { tier } : {}),
      });
    }
  };

  addValue(info.quotaInfo);
  addValue(info.quotaInfos);
  addValue(info.dailyQuotaInfo, undefined, classifyWindow("daily", "Daily"));
  addValue(info.dailyQuotaInfos, undefined, classifyWindow("daily", "Daily"));
  addValue(info.weeklyQuotaInfo, undefined, classifyWindow("weekly", "Weekly"));
  addValue(info.weeklyQuotaInfos, undefined, classifyWindow("weekly", "Weekly"));

  if (info.quotaInfoByTier) {
    for (const [tier, value] of Object.entries(info.quotaInfoByTier)) {
      addValue(value, tier);
    }
  }

  const addWindowMap = (values?: Record<string, QuotaInfoValue>) => {
    if (!values) return;
    for (const [windowId, value] of Object.entries(values)) {
      addValue(value, undefined, classifyWindow(windowId, undefined));
    }
  };
  addWindowMap(info.quotaInfoByWindow);
  addWindowMap(info.quotaInfosByWindow);

  return results;
}

/**
 * Quota entries often carry no explicit window id; entries sharing a backend
 * counter + tier split into daily/weekly windows whose latest reset time is
 * further out than a day. Mirrors oh-my-pi's window inference.
 */
function inferWindowDescriptors(
  quotaInfos: RawQuotaInfo[],
  nowMs: number
): WeakMap<RawQuotaInfo, { id: string; label: string }> {
  const descriptors = new WeakMap<RawQuotaInfo, { id: string; label: string }>();
  const groups = new Map<string, { info: RawQuotaInfo; resetAt: number | undefined }[]>();

  for (const info of quotaInfos) {
    const explicit = classifyWindow(info.windowId, info.windowLabel);
    if (explicit) {
      descriptors.set(info, explicit);
      continue;
    }
    const key = [info.modelProvider ?? "", info.apiProvider ?? "", info.tier ?? ""].join("|");
    const group = groups.get(key) ?? [];
    const t = info.resetTime ? new Date(info.resetTime).getTime() : NaN;
    group.push({ info, resetAt: Number.isNaN(t) ? undefined : t });
    groups.set(key, group);
  }

  for (const group of groups.values()) {
    const resetTimes = [...new Set(group.map((e) => e.resetAt).filter((t) => t !== undefined))].sort((a, b) => a - b);
    const latestReset = resetTimes.length > 1 ? resetTimes[resetTimes.length - 1] : undefined;
    for (const entry of group) {
      descriptors.set(
        entry.info,
        latestReset !== undefined && entry.resetAt === latestReset
          ? { id: "weekly", label: "Weekly" }
          : inferWindowFromReset(entry.resetAt, nowMs)
      );
    }
  }

  return descriptors;
}

/**
 * Dedupes per counter|tier|window keeping the entry with fraction data for
 * the bar and the lowest remaining fraction (worst case), while preserving
 * any reset time so "(resets in …)" survives. Same merge rules as oh-my-pi.
 */
function mergeQuotaLimit(map: Map<string, QuotaWindowLimit>, key: string, next: QuotaWindowLimit): void {
  const existing = map.get(key);
  if (!existing) {
    map.set(key, next);
    return;
  }
  let best = existing;
  if (existing.remainingFraction === undefined && next.remainingFraction !== undefined) {
    best = next;
  } else if (
    existing.remainingFraction !== undefined &&
    next.remainingFraction !== undefined &&
    next.remainingFraction < existing.remainingFraction
  ) {
    best = next;
  }
  const merged: QuotaWindowLimit = {
    ...best,
    resetTime: best.resetTime ?? next.resetTime,
    tier: best.tier ?? next.tier,
  };
  map.set(key, merged);
}

/** Fetches available models and quota from Google Antigravity. */
export async function fetchAntigravityQuota(
  accessToken: string,
  projectId?: string,
  refreshToken?: string,
  onTokenRefreshed?: (newToken: string) => void,
): Promise<ProviderQuotaReport | null> {
  const endpoints = [
    "https://daily-cloudcode-pa.googleapis.com",
    "https://daily-cloudcode-pa.sandbox.googleapis.com",
  ];

  let res: Response | null = null;
  let token = accessToken;

  for (const ep of endpoints) {
    try {
      let r = await fetch(`${ep}/v1internal:fetchAvailableModels`, {
        method: "POST",
        headers: {
          Authorization: `Bearer ${token}`,
          "Content-Type": "application/json",
          "User-Agent": getAntigravityUserAgent(),
        },
        body: JSON.stringify({
          project: projectId || undefined,
        }),
        signal: AbortSignal.timeout(15_000),
      });

      if (r.status === 401 && refreshToken) {
        try {
          const refreshed = await refreshGoogleToken(refreshToken);
          token = refreshed.access_token;
          onTokenRefreshed?.(token);

          r = await fetch(`${ep}/v1internal:fetchAvailableModels`, {
            method: "POST",
            headers: {
              Authorization: `Bearer ${token}`,
              "Content-Type": "application/json",
              "User-Agent": getAntigravityUserAgent(),
            },
            body: JSON.stringify({
              project: projectId || undefined,
            }),
            signal: AbortSignal.timeout(15_000),
          });
        } catch {}
      }

      if (r.ok) {
        res = r;
        break;
      }
    } catch {}
  }

  if (!res || !res.ok) {
    console.warn("[antigravity-quota] fetchAvailableModels failed across endpoints");
    return null;
  }

  try {
    const data = (await res.json()) as { models?: Record<string, RawModelInfo> };
    if (!data.models) return null;

    const nowMs = Date.now();
    const models: ModelQuota[] = [];
    const allLimits = new Map<string, QuotaWindowLimit>();
    let detectedTier = "free-tier";

    for (const [modelId, info] of Object.entries(data.models)) {
      const rawInfos = normalizeQuotaInfos(info);
      const descriptors = inferWindowDescriptors(rawInfos, nowMs);

      // Quota is shared across models within the same backend counter, tier,
      // and reset window; keep counters separate so an exhausted Gemini pool
      // cannot hide behind a healthy Claude one (mirrors oh-my-pi).
      const modelLimits = new Map<string, QuotaWindowLimit>();
      for (const q of rawInfos) {
        const descriptor = descriptors.get(q);
        const classified = classifyWindow(q.windowId, q.windowLabel);
        const entry: QuotaWindowLimit = {
          counter: quotaCounterName(q),
          windowId: descriptor?.id ?? q.windowId ?? classified?.id ?? "default",
          windowLabel: descriptor?.label ?? classified?.label ?? "Default",
          // Exhausted Google/Gemini counters omit remainingFraction and keep
          // only resetTime — treat that shape as fully used.
          remainingFraction: clampQuotaFraction(q.remainingFraction) ?? (q.resetTime ? 0 : undefined),
          resetTime: q.resetTime,
          tier: q.tier,
        };
        if (q.tier) detectedTier = q.tier;

        const key = `${entry.counter?.toLowerCase() ?? "default"}|${(q.tier ?? "default").toLowerCase()}|${entry.windowId}`;
        mergeQuotaLimit(modelLimits, key, entry);
        mergeQuotaLimit(allLimits, key, entry);
      }

      const windows = [...modelLimits.values()].sort(
        (a, b) => (a.remainingFraction ?? 1) - (b.remainingFraction ?? 1)
      );
      const worst = windows[0];
      models.push({
        model: modelId,
        displayName: info.displayName || modelId,
        counter: worst?.counter,
        windows,
        remainingFraction: worst?.remainingFraction,
        percentageLeft:
          worst?.remainingFraction !== undefined ? Math.round(worst.remainingFraction * 1000) / 10 : undefined,
        resetTime: worst?.resetTime,
        tier: worst?.tier,
      });
    }

    const limits = [...allLimits.values()].sort(
      (a, b) => (a.remainingFraction ?? 1) - (b.remainingFraction ?? 1)
    );

    return {
      provider: "google-antigravity",
      projectId,
      tier: detectedTier,
      fetchedAt: Date.now(),
      models,
      limits,
    };
  } catch (err) {
    console.warn("[antigravity-quota] Failed to parse quota:", err);
    return null;
  }
}

/** Aggregation cache: reparse usage.jsonl only when its mtime changes (P5). */
interface UsageAggregateCache {
  key: string;
  mtimeMs: number;
  summary: UsageSummary;
}
let usageAggregateCache: UsageAggregateCache | null = null;

/** Computes aggregated token usage across sessions and global usage records. */
export function getAggregatedUsage(dataDir?: string): UsageSummary {
  const baseDir = dataDir || path.join(os.homedir(), ".forge-ade");
  const usageFile = path.join(baseDir, "usage.jsonl");

  let mtimeMs = 0;
  try {
    mtimeMs = fs.statSync(usageFile).mtimeMs;
  } catch {}
  const cacheKey = `${dataDir || ""}`;
  if (usageAggregateCache && usageAggregateCache.key === cacheKey && usageAggregateCache.mtimeMs === mtimeMs) {
    return usageAggregateCache.summary;
  }

  let totalPrompt = 0;
  let totalCompletion = 0;
  let totalCached = 0;
  let requestCount = 0;

  const byProvider: Record<string, { prompt: number; completion: number; cached: number; requests: number }> = {};
  const byModel: Record<string, { prompt: number; completion: number; cached: number; requests: number }> = {};
  const byWorkspace: Record<string, { prompt: number; completion: number; cached: number; requests: number }> = {};

  if (mtimeMs > 0) {
    try {
      const content = fs.readFileSync(usageFile, "utf-8");
      const lines = content.split("\n").map((l) => l.trim()).filter(Boolean);

      for (const line of lines) {
        try {
          const rec = JSON.parse(line);
          const p = Number(rec.prompt_tokens) || Number(rec.input_tokens) || 0;
          const c = Number(rec.completion_tokens) || Number(rec.output_tokens) || 0;
          const k = Number(rec.cached_tokens) || 0;

          totalPrompt += p;
          totalCompletion += c;
          totalCached += k;
          requestCount++;

          const prov = rec.provider || "unknown";
          if (!byProvider[prov]) byProvider[prov] = { prompt: 0, completion: 0, cached: 0, requests: 0 };
          byProvider[prov].prompt += p;
          byProvider[prov].completion += c;
          byProvider[prov].cached += k;
          byProvider[prov].requests++;

          const mod = rec.model || "unknown";
          if (!byModel[mod]) byModel[mod] = { prompt: 0, completion: 0, cached: 0, requests: 0 };
          byModel[mod].prompt += p;
          byModel[mod].completion += c;
          byModel[mod].cached += k;
          byModel[mod].requests++;

          const ws = rec.workspace || "Default";
          if (!byWorkspace[ws]) byWorkspace[ws] = { prompt: 0, completion: 0, cached: 0, requests: 0 };
          byWorkspace[ws].prompt += p;
          byWorkspace[ws].completion += c;
          byWorkspace[ws].cached += k;
          byWorkspace[ws].requests++;
        } catch {}
      }
    } catch {}
  }

  const totalTokens = totalPrompt + totalCompletion + totalCached;
  const cacheHitRate = totalPrompt + totalCached > 0 ? (totalCached / (totalPrompt + totalCached)) * 100 : 0;

  const summary: UsageSummary = {
    totalPromptTokens: totalPrompt,
    totalCompletionTokens: totalCompletion,
    totalCachedTokens: totalCached,
    totalTokens,
    cacheHitRate: Math.round(cacheHitRate * 10) / 10,
    requestCount,
    byProvider,
    byModel,
    byWorkspace,
  };
  usageAggregateCache = { key: cacheKey, mtimeMs, summary };
  return summary;
}

/** Formats coarse time remaining from ISO timestamp, e.g. "(4h53m)" or "(2h26m)" */
export function formatTimeUntil(targetIso: string | undefined): string {
  if (!targetIso) return "";
  const targetMs = new Date(targetIso).getTime();
  const diffMs = targetMs - Date.now();
  if (diffMs <= 0) return "(ready)";
  // Floor instead of round: a window 299.9 minutes out renders "4h59m",
  // never a premature "5h".
  const totalMins = Math.floor(diffMs / 60000);
  const hours = Math.floor(totalMins / 60);
  const mins = totalMins % 60;
  if (hours > 0) return `(${hours}h${mins > 0 ? `${mins}m` : ""})`;
  return `(${mins}m)`;
}

/**
 * Renders a 24-character ASCII usage bar (filled = used quota), e.g.
 * ████████████░░░░░░░░░░░░ with ▒/▓ partial cells like oh-my-pi's renderer.
 */
export function renderAsciiQuotaBar(fractionLeft: number | undefined, width = 24): string {
  if (fractionLeft === undefined) return "·".repeat(width);
  const clamped = Math.min(Math.max(fractionLeft, 0), 1);
  const exact = (1 - clamped) * width;
  const fullCells = Math.floor(exact);
  const remainder = exact - fullCells;
  let partial = "";
  if (remainder >= 2 / 3) partial = "▓";
  else if (remainder >= 1 / 3) partial = "▒";
  return `${"█".repeat(fullCells)}${partial}${"░".repeat(Math.max(0, width - fullCells - (partial ? 1 : 0)))}`;
}

function truncateLabel(label: string, maxLen: number): string {
  if (!label) return "account";
  if (label.length <= maxLen) return label;
  return label.slice(0, maxLen - 1) + "…";
}

/** Same thresholds as oh-my-pi: exhausted at 0, warning at ≤10% remaining. */
function quotaLimitStatus(remaining: number | undefined): "ok" | "warning" | "exhausted" {
  if (remaining === undefined) return "ok";
  if (remaining <= 0) return "exhausted";
  if (remaining <= 0.1) return "warning";
  return "ok";
}

/**
 * Formats multi-account Antigravity quotas in the same layout as oh-my-pi's
 * /usage view: one section per backend counter and window ("Usage (Google)
 * (Daily)"), account columns sorted worst-first and kept in that order across
 * sections, bars filled by used quota, trailing "% free" averaged over the
 * accounts in the section.
 */
export function formatMultiAccountAsciiQuota(reports: ProviderQuotaReport[]): string {
  if (!reports || reports.length === 0) {
    return "No Antigravity accounts connected.";
  }

  const BAR_WIDTH = 24;
  const COLUMN_WIDTH = 30;

  // Worst-first column order, computed once per provider so an account keeps
  // its position in every section (oh-my-pi issue #6067 fix).
  const rankOfReport = new Map<ProviderQuotaReport, number>();
  reports.forEach((report, index) => {
    const worstUsed = report.limits.reduce((max, limit) => {
      const used = limit.remainingFraction === undefined ? -1 : 1 - limit.remainingFraction;
      return used > max ? used : max;
    }, -1);
    rankOfReport.set(report, -worstUsed * 1000 + index);
  });

  interface Column {
    report: ProviderQuotaReport;
    limit: QuotaWindowLimit;
  }
  const groups = new Map<string, { label: string; windowLabel: string; columns: Column[] }>();
  for (const report of reports) {
    for (const limit of report.limits) {
      const label = limit.counter ? `Usage (${limit.counter})` : "Usage";
      const key = `${label}|${limit.windowId}`;
      const group = groups.get(key) ?? { label, windowLabel: limit.windowLabel, columns: [] };
      group.columns.push({ report, limit });
      groups.set(key, group);
    }
  }

  const sortedGroups = [...groups.values()].sort((a, b) => {
    const aWorst = Math.min(...a.columns.map((c) => c.limit.remainingFraction ?? 1));
    const bWorst = Math.min(...b.columns.map((c) => c.limit.remainingFraction ?? 1));
    if (aWorst !== bWorst) return aWorst - bWorst;
    return a.label.localeCompare(b.label);
  });

  const lines: string[] = [];
  for (const group of sortedGroups) {
    const columns = [...group.columns].sort(
      (a, b) => (rankOfReport.get(a.report) ?? 0) - (rankOfReport.get(b.report) ?? 0)
    );

    // Worst-of aggregation like oh-my-pi: any non-ok account flags the section.
    const tag = columns.some((c) => quotaLimitStatus(c.limit.remainingFraction) !== "ok") ? "[!]" : "[ok]";
    lines.push(`${tag} ${group.label} (${group.windowLabel})`);

    const suffixes = columns.map((c) => formatTimeUntil(c.limit.resetTime));
    const maxSuffix = suffixes.reduce((max, s) => Math.max(max, s.length), 0);
    const gap = maxSuffix > 0 ? 1 : 0;
    const prefixBudget = COLUMN_WIDTH - maxSuffix - gap;

    const headerCells = columns.map((c, i) => {
      const email = c.report.accountEmail || "Primary";
      if (prefixBudget < 2) {
        return truncateLabel(`${email} ${suffixes[i]}`.trim(), COLUMN_WIDTH).padEnd(COLUMN_WIDTH);
      }
      const prefix = truncateLabel(email, prefixBudget).padEnd(prefixBudget);
      const suffixPad = " ".repeat(maxSuffix - suffixes[i].length);
      return `${prefix} ${suffixPad}${suffixes[i]}`.padEnd(COLUMN_WIDTH);
    });
    lines.push(`  ${headerCells.join(" ").trimEnd()}`);

    const barCells = columns.map((c) => renderAsciiQuotaBar(c.limit.remainingFraction, BAR_WIDTH).padEnd(COLUMN_WIDTH));
    const fractions = columns.map((c) => c.limit.remainingFraction).filter((f): f is number => f !== undefined);
    const freeText =
      fractions.length > 0
        ? `${Math.round((fractions.reduce((sum, f) => sum + f, 0) / fractions.length) * 1000) / 10}% free`
        : "";
    lines.push(`  ${barCells.join(" ").trimEnd()}${freeText ? ` ${freeText}` : ""}`.trimEnd());
    lines.push("");
  }

  return lines.join("\n").trimEnd();
}

/** Fetches quota for all configured Google Antigravity profiles */
export async function getAllAntigravityQuotas(llm: any): Promise<ProviderQuotaReport[]> {
  const profiles: any[] = llm?.getProviderProfiles() || [];
  const antigravityProfiles = profiles.filter(
    (p) => p.id === "google-antigravity" || p.provider === "google-antigravity" || p.id.startsWith("google-antigravity")
  );

  if (antigravityProfiles.length === 0) return [];

  const reports = await Promise.all(
    antigravityProfiles.map(async (p) => {
      const quota = await fetchAntigravityQuota(
        p.apiKey,
        p.projectId,
        p.refreshToken,
        (newToken) => {
          p.apiKey = newToken;
          llm.saveProviderProfiles([p]);
        }
      );
      if (quota) {
        quota.accountEmail = p.accountEmail;
      }
      return quota;
    })
  );

  return reports.filter((r): r is ProviderQuotaReport => r !== null);
}

