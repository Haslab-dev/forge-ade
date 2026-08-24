// Live quota & usage fetchers for providers (Google Antigravity, OpenCode, Kilo)
// and aggregate workspace token statistics.

import fs from "fs";
import path from "path";
import os from "os";
import { refreshGoogleToken } from "./oauth";

export interface ModelQuota {
  model: string;
  displayName: string;
  remainingFraction?: number | undefined;
  percentageLeft?: number | undefined;
  resetTime?: string | undefined;
  tier?: string | undefined;
  dailyQuota?: {
    remainingFraction?: number | undefined;
    resetTime?: string | undefined;
  } | undefined;
  weeklyQuota?: {
    remainingFraction?: number | undefined;
    resetTime?: string | undefined;
  } | undefined;
}

export interface ProviderQuotaReport {
  provider: string;
  accountEmail?: string | undefined;
  projectId?: string | undefined;
  tier?: string | undefined;
  fetchedAt: number;
  models: ModelQuota[];
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
    const data = (await res.json()) as {
      models?: Record<
        string,
        {
          displayName?: string;
          quotaInfo?: any;
          quotaInfos?: any[];
          dailyQuotaInfo?: any;
          weeklyQuotaInfo?: any;
        }
      >;
    };

    if (!data.models) return null;

    const models: ModelQuota[] = [];
    let detectedTier = "free-tier";

    for (const [modelId, info] of Object.entries(data.models)) {
      const q = info.quotaInfo || info.quotaInfos?.[0] || info.dailyQuotaInfo;
      const rem = typeof q?.remainingFraction === "number" ? q.remainingFraction : undefined;
      if (q?.tier) detectedTier = q.tier;

      models.push({
        model: modelId,
        displayName: info.displayName || modelId,
        remainingFraction: rem,
        percentageLeft: rem !== undefined ? Math.round(rem * 1000) / 10 : undefined,
        resetTime: q?.resetTime,
        tier: q?.tier,
        dailyQuota: info.dailyQuotaInfo
          ? {
              remainingFraction: info.dailyQuotaInfo.remainingFraction,
              resetTime: info.dailyQuotaInfo.resetTime,
            }
          : undefined,
        weeklyQuota: info.weeklyQuotaInfo
          ? {
              remainingFraction: info.weeklyQuotaInfo.remainingFraction,
              resetTime: info.weeklyQuotaInfo.resetTime,
            }
          : undefined,
      });
    }

    return {
      provider: "google-antigravity",
      projectId,
      tier: detectedTier,
      fetchedAt: Date.now(),
      models,
    };
  } catch (err) {
    console.warn("[antigravity-quota] Failed to parse quota:", err);
    return null;
  }
}

/** Computes aggregated token usage across sessions and global usage records. */
export function getAggregatedUsage(dataDir?: string): UsageSummary {
  const baseDir = dataDir || path.join(os.homedir(), ".forge-ade");
  const usageFile = path.join(baseDir, "usage.jsonl");

  let totalPrompt = 0;
  let totalCompletion = 0;
  let totalCached = 0;
  let requestCount = 0;

  const byProvider: Record<string, { prompt: number; completion: number; cached: number; requests: number }> = {};
  const byModel: Record<string, { prompt: number; completion: number; cached: number; requests: number }> = {};
  const byWorkspace: Record<string, { prompt: number; completion: number; cached: number; requests: number }> = {};

  if (fs.existsSync(usageFile)) {
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

  return {
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
}

/** Formats coarse time remaining from ISO timestamp, e.g. "(4h53m)" or "(2h26m)" */
export function formatTimeUntil(targetIso: string | undefined): string {
  if (!targetIso) return "";
  const targetMs = new Date(targetIso).getTime();
  if (isNaN(targetMs)) return "";
  const diffMs = targetMs - Date.now();
  if (diffMs <= 0) return "(ready)";
  const totalMins = Math.round(diffMs / 60000);
  const hours = Math.floor(totalMins / 60);
  const mins = totalMins % 60;
  if (hours > 0) return `(${hours}h${mins > 0 ? `${mins}m` : ""})`;
  return `(${mins}m)`;
}

/** Renders a 24-character ASCII progress bar, e.g. ████████████░░░░░░░░░░░░ */
export function renderAsciiQuotaBar(fractionLeft: number | undefined, width = 24): string {
  if (fractionLeft === undefined) return "·".repeat(width);
  const clamped = Math.min(Math.max(fractionLeft, 0), 1);
  const used = 1 - clamped;
  const filled = Math.round(used * width);
  return `${"█".repeat(filled)}${"░".repeat(Math.max(0, width - filled))}`;
}

/** Truncates email cleanly to fit column */
function truncateEmail(email: string, maxLen = 26): string {
  if (!email) return "account";
  if (email.length <= maxLen) return email;
  return email.slice(0, maxLen - 1) + "…";
}

/** Formats multi-account daily & weekly quotas with ASCII bars matching oh-my-pi */
export function formatMultiAccountAsciiQuota(reports: ProviderQuotaReport[]): string {
  if (!reports || reports.length === 0) {
    return "No Antigravity accounts connected.";
  }

  const lines: string[] = [];

  interface FamilyConfig {
    name: string;
    match: (m: string) => boolean;
  }

  const families: FamilyConfig[] = [
    {
      name: "Anthropic",
      match: (m) => m.includes("claude") || m.includes("sonnet") || m.includes("opus") || m.includes("haiku"),
    },
    {
      name: "Google",
      match: (m) => m.includes("gemini") || m.includes("flash") || m.includes("pro"),
    },
    {
      name: "OpenAI",
      match: (m) => m.includes("gpt") || m.includes("o1") || m.includes("o3") || m.includes("oss"),
    },
  ];

  for (const fam of families) {
    // Find the primary representative model for this family in each account report
    const accountEntries = reports.map((r) => {
      const email = r.accountEmail || "Primary";
      const matchedModel = r.models.find((m) => fam.match(m.model.toLowerCase())) || r.models[0];
      const pct = matchedModel?.percentageLeft ?? 100;
      const frac = matchedModel?.remainingFraction ?? 1.0;
      const resetTime = matchedModel?.resetTime || matchedModel?.dailyQuota?.resetTime;
      const timeLabel = formatTimeUntil(resetTime);

      return {
        email,
        pct,
        frac,
        timeLabel,
      };
    });

    if (accountEntries.length === 0) continue;

    const minPct = Math.min(...accountEntries.map((e) => e.pct));
    const statusTag = minPct < 20 ? "[!]" : "[ok]";

    lines.push(`${statusTag} Usage (${fam.name}) (Daily)`);

    // Header row with email and (time)
    const headerParts = accountEntries.map((e) => {
      const label = `${truncateEmail(e.email, 22)} ${e.timeLabel}`;
      return label.padEnd(28, " ");
    });
    lines.push(`   ${headerParts.join(" ")}`);

    // Bar row with ASCII bars and average % free
    const barParts = accountEntries.map((e) => {
      const bar = renderAsciiQuotaBar(e.frac, 24);
      return bar.padEnd(28, " ");
    });
    lines.push(`   ${barParts.join(" ")} ${minPct}% free`);
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

