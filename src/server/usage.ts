// Usage manager — real token-usage accounting from a global JSONL journal.
// Every LLM call the agent engine makes appends one line; this module
// aggregates them for the Usage tab. Journal is global across all projects.

import fs from "fs";
import path from "path";
import os from "os";

export interface Overview {
  totalTokens: number;
  inputTokens: number;
  outputTokens: number;
  cachedTokens: number;
  totalCost: number;
  requestCount: number;
  avgLatencyMs: number;
}

export interface DayPoint {
  date: string;
  totalTokens: number;
  inputTokens: number;
  outputTokens: number;
  cachedTokens: number;
  cost: number;
  requestCount: number;
}

export interface RequestRow {
  id: string;
  timestamp: number;
  provider: string;
  model: string;
  workspace: string;
  inputTokens: number;
  outputTokens: number;
  cost: number;
  latencyMs: number;
  status: string;
}

export interface Bucket {
  key: string;
  label: string;
  totalTokens: number;
  cost: number;
  requestCount: number;
}

export interface FilterOptions {
  providers: string[];
  models: string[];
  workspaces: string[];
  agents: string[];
}

/** One appended line per LLM call (written by the agent engine). */
export interface UsageRecord {
  ts: number;
  provider: string;
  model: string;
  workspace: string;
  sessionId: string;
  inputTokens: number;
  outputTokens: number;
  cachedTokens: number;
  latencyMs: number;
}

const FILTER_MS: Record<string, number> = {
  today: 1,
  "24h": 1,
  "7d": 7,
  "30d": 30,
};

function filterCutoff(filter: string): number {
  if (filter === "all" || filter === "") return 0;
  const days = FILTER_MS[filter];
  if (!days) return 0;
  if (days === 1 && (filter === "today")) {
    const d = new Date();
    d.setHours(0, 0, 0, 0);
    return d.getTime();
  }
  return Date.now() - days * 86_400_000;
}

function dayKey(ts: number): string {
  const d = new Date(ts);
  const pad = (v: number) => String(v).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

function basenameOf(folder: string): string {
  const trimmed = folder.replace(/\/+$/, "");
  return trimmed.slice(trimmed.lastIndexOf("/") + 1) || trimmed;
}

/** Appends one usage record; called by the agent engine after each LLM call. */
export function recordUsage(dataDir: string, record: UsageRecord): void {
  try {
    const dir = path.join(dataDir, "usage");
    fs.mkdirSync(dir, { recursive: true });
    fs.appendFileSync(path.join(dir, "usage.jsonl"), JSON.stringify(record) + "\n", "utf-8");
  } catch (err) {
    console.error("[usage] journal append failed:", err);
  }
}

export class UsageManager {
  private dataDir: string;
  private usageFile: string;
  private cache: { records: UsageRecord[]; mtimeMs: number } | null = null;

  constructor(dataDir?: string) {
    this.dataDir = dataDir || path.join(os.homedir(), ".forge-ade");
    this.usageFile = path.join(this.dataDir, "usage", "usage.jsonl");
  }

  /** Reads the journal, re-parsing only when the file changed. */
  private records(): UsageRecord[] {
    let mtimeMs = 0;
    try {
      mtimeMs = fs.statSync(this.usageFile).mtimeMs;
    } catch {
      return [];
    }
    if (this.cache && this.cache.mtimeMs === mtimeMs) return this.cache.records;
    const records: UsageRecord[] = [];
    try {
      const lines = fs.readFileSync(this.usageFile, "utf-8").split("\n");
      for (const line of lines) {
        if (!line) continue;
        try {
          const parsed = JSON.parse(line) as Partial<UsageRecord>;
          if (
            typeof parsed.ts === "number" &&
            typeof parsed.inputTokens === "number" &&
            typeof parsed.outputTokens === "number"
          ) {
            records.push({
              ts: parsed.ts,
              provider: String(parsed.provider ?? "unknown"),
              model: String(parsed.model ?? "unknown"),
              workspace: String(parsed.workspace ?? ""),
              sessionId: String(parsed.sessionId ?? ""),
              inputTokens: parsed.inputTokens,
              outputTokens: parsed.outputTokens,
              cachedTokens: typeof parsed.cachedTokens === "number" ? parsed.cachedTokens : 0,
              latencyMs: typeof parsed.latencyMs === "number" ? parsed.latencyMs : 0,
            });
          }
        } catch {}
      }
    } catch {}
    this.cache = { records, mtimeMs };
    return records;
  }

  private inWindow(filter: string): UsageRecord[] {
    const cutoff = filterCutoff(filter);
    return this.records().filter((r) => r.ts >= cutoff);
  }

  /** Raw journal rows (all time) for the frontend aggregator. */
  public getAllRecords(): Array<{
    ts: number;
    provider: string;
    model: string;
    workspace: string;
    sessionId: string;
    inputTokens: number;
    outputTokens: number;
    cachedTokens: number;
    latencyMs: number;
  }> {
    return this.inWindow("all").map((r) => ({
      ts: r.ts,
      provider: r.provider,
      model: r.model,
      workspace: r.workspace,
      sessionId: r.sessionId,
      inputTokens: r.inputTokens,
      outputTokens: r.outputTokens,
      cachedTokens: r.cachedTokens,
      latencyMs: r.latencyMs,
    }));
  }

  public getOverview(filter: string): Overview {
    const rows = this.inWindow(filter);
    const inputTokens = rows.reduce((n, r) => n + r.inputTokens, 0);
    const outputTokens = rows.reduce((n, r) => n + r.outputTokens, 0);
    const cachedTokens = rows.reduce((n, r) => n + r.cachedTokens, 0);
    const latency = rows.reduce((n, r) => n + r.latencyMs, 0);
    return {
      totalTokens: inputTokens + outputTokens,
      inputTokens,
      outputTokens,
      cachedTokens,
      totalCost: 0,
      requestCount: rows.length,
      avgLatencyMs: rows.length > 0 ? Math.round(latency / rows.length) : 0,
    };
  }

  public getTimeSeries(filter: string): DayPoint[] {
    const byDay = new Map<string, DayPoint>();
    for (const r of this.inWindow(filter)) {
      const key = dayKey(r.ts);
      let point = byDay.get(key);
      if (!point) {
        point = { date: key, totalTokens: 0, inputTokens: 0, outputTokens: 0, cachedTokens: 0, cost: 0, requestCount: 0 };
        byDay.set(key, point);
      }
      point.inputTokens += r.inputTokens;
      point.outputTokens += r.outputTokens;
      point.cachedTokens += r.cachedTokens;
      point.totalTokens += r.inputTokens + r.outputTokens;
      point.requestCount += 1;
    }
    return [...byDay.values()].sort((a, b) => a.date.localeCompare(b.date));
  }

  public getRequests(filter: string, limit: number = 50): RequestRow[] {
    return this.inWindow(filter)
      .sort((a, b) => b.ts - a.ts)
      .slice(0, limit)
      .map((r, i) => ({
        id: `${r.sessionId}-${i}`,
        timestamp: r.ts,
        provider: r.provider,
        model: r.model,
        workspace: basenameOf(r.workspace),
        inputTokens: r.inputTokens,
        outputTokens: r.outputTokens,
        cost: 0,
        latencyMs: r.latencyMs,
        status: "ok",
      }));
  }

  public getBuckets(dimension: string, filter: string): Bucket[] {
    type Acc = { label: string; tokens: number; requests: number };
    const byKey = new Map<string, Acc>();
    for (const r of this.inWindow(filter)) {
      let key: string;
      let label: string;
      switch (dimension) {
        case "provider":
          key = r.provider;
          label = r.provider;
          break;
        case "model":
          key = r.model;
          label = r.model;
          break;
        case "workspace":
          key = r.workspace || "(none)";
          label = basenameOf(r.workspace) || "(none)";
          break;
        case "agent":
          key = r.sessionId;
          label = r.sessionId.slice(0, 18);
          break;
        default:
          return [];
      }
      let acc = byKey.get(key);
      if (!acc) {
        acc = { label, tokens: 0, requests: 0 };
        byKey.set(key, acc);
      }
      acc.tokens += r.inputTokens + r.outputTokens;
      acc.requests += 1;
    }
    return [...byKey.entries()]
      .map(([key, acc]) => ({ key, label: acc.label, totalTokens: acc.tokens, cost: 0, requestCount: acc.requests }))
      .sort((a, b) => b.totalTokens - a.totalTokens);
  }

  public getFilterOptions(): FilterOptions {
    const all = this.records();
    const uniq = (values: string[]): string[] => [...new Set(values.filter(Boolean))].sort();
    return {
      providers: uniq(all.map((r) => r.provider)),
      models: uniq(all.map((r) => r.model)),
      workspaces: uniq(all.map((r) => basenameOf(r.workspace))),
      agents: uniq(all.map((r) => r.sessionId)),
    };
  }
}
