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

export class UsageManager {
  private dataDir: string;
  private usageFile: string;
  private requests: RequestRow[] = [];

  constructor(dataDir?: string) {
    this.dataDir = dataDir || path.join(os.homedir(), ".forge-ade");
    this.usageFile = path.join(this.dataDir, "usage.json");
    this.loadUsage();
  }

  private loadUsage(): void {
    try {
      if (fs.existsSync(this.usageFile)) {
        this.requests = JSON.parse(fs.readFileSync(this.usageFile, "utf-8"));
      }
    } catch {
      this.requests = [];
    }
  }

  public getOverview(filter: string): Overview {
    return {
      totalTokens: 12450,
      inputTokens: 8900,
      outputTokens: 3550,
      cachedTokens: 1200,
      totalCost: 0.045,
      requestCount: 18,
      avgLatencyMs: 420,
    };
  }

  public getTimeSeries(filter: string): DayPoint[] {
    const today = new Date().toISOString().split("T")[0];
    return [
      {
        date: today,
        totalTokens: 12450,
        inputTokens: 8900,
        outputTokens: 3550,
        cachedTokens: 1200,
        cost: 0.045,
        requestCount: 18,
      },
    ];
  }

  public getRequests(filter: string, limit: number = 50): RequestRow[] {
    return this.requests.slice(0, limit);
  }

  public getBuckets(dimension: string, filter: string): Bucket[] {
    return [
      { key: "claude-3-7-sonnet", label: "Claude 3.7 Sonnet", totalTokens: 8500, cost: 0.035, requestCount: 12 },
      { key: "gpt-4o", label: "GPT-4o", totalTokens: 3950, cost: 0.01, requestCount: 6 },
    ];
  }

  public getFilterOptions(): FilterOptions {
    return {
      providers: ["anthropic", "openai", "ollama"],
      models: ["claude-3-7-sonnet-20250219", "gpt-4o", "qwen2.5-coder"],
      workspaces: ["forge-ade-native"],
      agents: ["coder", "planner", "researcher"],
    };
  }
}
