import React, { useEffect, useMemo, useState } from "react";
import {
  IconActivity,
  IconBolt,
  IconClock,
  IconCurrencyDollar,
  IconGauge,
  IconList,
  IconRefresh,
  IconCheck,
  IconX,
} from "@tabler/icons-react";
import { cn } from "../lib/utils";
import {
  GetUsageOverview,
  GetUsageTimeSeries,
  GetUsageRequests,
  GetUsageBuckets,
  GetUsageFilterOptions,
} from "../lib/native";

const DATE_FILTERS = [
  { id: "today", label: "Today" },
  { id: "7d", label: "7 Days" },
  { id: "30d", label: "30 Days" },
  { id: "month", label: "Month" },
  { id: "year", label: "Year" },
  { id: "all", label: "All" },
];

function fmtNum(n: number): string {
  if (n >= 1e9) return (n / 1e9).toFixed(2) + "B";
  if (n >= 1e6) return (n / 1e6).toFixed(1) + "M";
  if (n >= 1e3) return (n / 1e3).toFixed(0) + "K";
  return String(n || 0);
}

function fmtCost(n: number): string {
  if (!n) return "$0";
  if (n >= 1) return "$" + n.toFixed(2);
  if (n >= 0.01) return "$" + n.toFixed(3);
  return "$" + n.toFixed(4);
}

function fmtDuration(ms: number): string {
  if (ms >= 1000) return (ms / 1000).toFixed(1) + "s";
  return ms + "ms";
}

// A single KPI card.
function KpiCard({ label, value, sub, icon }: { label: string; value: string; sub?: string; icon: React.ReactNode }) {
  return (
    <div className="flex flex-col gap-1 bg-[var(--bg-panel)] border border-[var(--border-default)] rounded p-3 min-w-0">
      <div className="flex items-center gap-1.5 text-[10px] font-semibold uppercase tracking-wider text-[var(--fg-tertiary)]">
        {icon}
        {label}
      </div>
      <div className="text-lg font-bold text-[var(--fg-primary)] font-mono truncate">{value}</div>
      {sub && <div className="text-[10px] text-[var(--fg-tertiary)] truncate">{sub}</div>}
    </div>
  );
}

// Stacked token bar chart (Input / Cached / Output) rendered as SVG.
function TokenChart({ points }: { points: any[] }) {
  const max = useMemo(() => {
    let m = 1;
    for (const p of points) m = Math.max(m, (p.input_tokens || 0) + (p.cached_tokens || 0) + (p.output_tokens || 0));
    return m;
  }, [points]);

  if (points.length === 0) {
    return <div className="text-[10px] text-[var(--fg-tertiary)] italic">No data in this range</div>;
  }

  const W = 720;
  const H = 180;
  const barW = Math.max(4, (W / points.length) - 3);

  return (
    <svg viewBox={`0 0 ${W} ${H}`} className="w-full h-44">
      {points.map((p, i) => {
        const x = i * (W / points.length) + 1.5;
        const inH = (p.input_tokens || 0) / max * (H - 20);
        const cacheH = (p.cached_tokens || 0) / max * (H - 20);
        const outH = (p.output_tokens || 0) / max * (H - 20);
        const y0 = H - 10;
        return (
          <g key={p.date}>
            <title>{`${p.date}: in ${fmtNum(p.input_tokens)} / cached ${fmtNum(p.cached_tokens)} / out ${fmtNum(p.output_tokens)}`}</title>
            {/* Input */}
            <rect x={x} y={y0 - inH} width={barW} height={inH} fill="#4f8cff" />
            {/* Cached */}
            <rect x={x} y={y0 - inH - cacheH} width={barW} height={cacheH} fill="#22c55e" />
            {/* Output */}
            <rect x={x} y={y0 - inH - cacheH - outH} width={barW} height={outH} fill="#a78bfa" />
          </g>
        );
      })}
    </svg>
  );
}

function BucketsTable({ dimension, filter }: { dimension: string; filter: string }) {
  const [buckets, setBuckets] = useState<any[]>([]);

  useEffect(() => {
    let cancelled = false;
    GetUsageBuckets(dimension, filter).then((rows) => {
      if (!cancelled) setBuckets(Array.isArray(rows) ? rows : []);
    });
    return () => { cancelled = true; };
  }, [dimension, filter]);

  if (buckets.length === 0) {
    return <div className="text-[10px] text-[var(--fg-tertiary)] italic">No data</div>;
  }

  return (
    <div className="flex flex-col gap-0.5 min-w-0">
      {buckets.slice(0, 15).map((b) => (
        <div
          key={b.key}
          className="flex items-center gap-2 px-2 py-1 text-[11px] font-mono rounded hover:bg-[var(--bg-surface-hover)]"
        >
          <span className="w-36 truncate text-[var(--fg-primary)] font-medium shrink-0" title={b.key}>{b.key}</span>
          <span className="w-16 text-right text-[var(--fg-tertiary)] shrink-0">{fmtNum(b.requests)} req</span>
          <span className="w-16 text-right text-[var(--fg-secondary)] shrink-0" title="Input tokens">in {fmtNum(b.input_tokens)}</span>
          <span className="w-16 text-right text-[var(--fg-secondary)] shrink-0" title="Output tokens">out {fmtNum(b.output_tokens)}</span>
          <span className="w-16 text-right text-emerald-400 shrink-0" title="Cached tokens">cache {fmtNum(b.cached_tokens)}</span>
          <span className="w-12 text-right text-sky-400 shrink-0" title="Cache hit rate">{(b.cache_hit_rate || 0).toFixed(0)}%</span>
          <span className="flex-1" />
          <span className="w-16 text-right text-[var(--fg-tertiary)] shrink-0" title="Estimated cost">{fmtCost(b.cost_usd)}</span>
        </div>
      ))}
    </div>
  );
}

export function UsagePanel() {
  const [filter, setFilter] = useState("30d");
  const [overview, setOverview] = useState<any>(null);
  const [points, setPoints] = useState<any[]>([]);
  const [requests, setRequests] = useState<any[]>([]);
  const [filterOptions, setFilterOptions] = useState<any>(null);
  const [loading, setLoading] = useState(false);
  const [tab, setTab] = useState<"overview" | "requests" | "workspaces" | "agents" | "providers" | "models">("overview");

  const reload = async () => {
    setLoading(true);
    try {
      const [ov, ts, reqs, opts] = await Promise.all([
        GetUsageOverview(filter),
        GetUsageTimeSeries(filter),
        GetUsageRequests(filter, 100),
        GetUsageFilterOptions(),
      ]);
      setOverview(ov);
      setPoints(Array.isArray(ts) ? ts : []);
      setRequests(Array.isArray(reqs) ? reqs : []);
      setFilterOptions(opts);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { reload(); }, [filter]);

  const cacheHit = overview?.cache_hit_rate ?? 0;

  return (
    <div className="flex flex-col h-full w-full bg-[var(--bg-app)] overflow-hidden">
      {/* Header */}
      <div className="flex items-center justify-between px-3 py-2 bg-[var(--bg-sidebar)] border-b border-[var(--border-default)] shrink-0">
        <div className="flex items-center gap-2 font-bold text-xs text-[var(--fg-primary)]">
          <IconActivity className="size-4 text-sky-400" />
          Usage &amp; Agent Observability
        </div>
        <div className="flex items-center gap-1">
          {/* Date filter */}
          <div className="flex items-center gap-0.5 bg-[var(--bg-panel)] border border-[var(--border-default)] rounded p-0.5">
            {DATE_FILTERS.map((f) => (
              <button
                key={f.id}
                onClick={() => setFilter(f.id)}
                className={cn(
                  "px-1.5 py-0.5 text-[10px] rounded cursor-pointer",
                  filter === f.id
                    ? "bg-[var(--accent-primary)] text-black font-semibold"
                    : "text-[var(--fg-tertiary)] hover:text-[var(--fg-primary)]"
                )}
              >
                {f.label}
              </button>
            ))}
          </div>
          <button
            onClick={reload}
            className="p-1 rounded hover:bg-[var(--bg-surface-hover)] text-[var(--fg-tertiary)] hover:text-white cursor-pointer"
            title="Refresh"
          >
            <IconRefresh className={cn("size-3.5", loading && "animate-spin")} />
          </button>
        </div>
      </div>

      {/* Sub-tabs */}
      <div className="flex items-center gap-0.5 px-3 py-1.5 border-b border-[var(--border-default)] shrink-0 bg-[var(--bg-panel)] overflow-x-auto">
        {([
          ["overview", "Overview"],
          ["requests", "Requests"],
          ["workspaces", "Workspaces"],
          ["agents", "Agents"],
          ["providers", "Providers"],
          ["models", "Models"],
        ] as const).map(([id, label]) => (
          <button
            key={id}
            onClick={() => setTab(id)}
            className={cn(
              "px-2 py-0.5 text-[10px] font-semibold rounded cursor-pointer",
              tab === id ? "bg-[var(--bg-surface-active)] text-[var(--fg-on-active)]" : "text-[var(--fg-tertiary)] hover:text-[var(--fg-primary)]"
            )}
          >
            {label}
          </button>
        ))}
      </div>

      <div className="flex-1 overflow-y-auto p-3 space-y-4">
        {/* Overview KPIs — 2x2 grid on narrow, 4-col on wide */}
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-2">
          <KpiCard label="Requests" value={fmtNum(overview?.requests)} icon={<IconList className="size-3" />} />
          <KpiCard label="Input Tokens" value={fmtNum(overview?.input_tokens)} icon={<IconGauge className="size-3 text-sky-400" />} />
          <KpiCard label="Output Tokens" value={fmtNum(overview?.output_tokens)} icon={<IconGauge className="size-3 text-purple-400" />} />
          <KpiCard label="Cached Tokens" value={fmtNum(overview?.cached_tokens)} icon={<IconBolt className="size-3 text-emerald-400" />} />
          <KpiCard label="Generation Speed" value={overview?.avg_speed_tps ? `${overview.avg_speed_tps} tok/s` : "—"} icon={<IconActivity className="size-3 text-cyan-400" />} />
          <KpiCard label="Latency (P95)" value={overview?.latency_p95_ms ? fmtDuration(overview.latency_p95_ms) : "—"} icon={<IconClock className="size-3" />} />
          <KpiCard label="Avg Tool Calls" value={overview?.avg_tool_calls ? overview.avg_tool_calls.toFixed(1) : "—"} icon={<IconGauge className="size-3 text-amber-400" />} />
          <KpiCard label="Estimated Cost" value={fmtCost(overview?.cost_usd)} icon={<IconCurrencyDollar className="size-3 text-emerald-400" />} />
        </div>

        {/* Chart */}
        <div className="bg-[var(--bg-panel)] border border-[var(--border-default)] rounded p-3">
          <div className="flex items-center justify-between mb-2">
            <span className="text-[10px] font-bold uppercase tracking-wider text-[var(--fg-tertiary)]">Tokens per day</span>
            <div className="flex items-center gap-2 text-[9px] font-mono text-[var(--fg-tertiary)]">
              <span className="flex items-center gap-1"><span className="size-2 rounded-sm bg-[#4f8cff]" /> Input</span>
              <span className="flex items-center gap-1"><span className="size-2 rounded-sm bg-[#22c55e]" /> Cached</span>
              <span className="flex items-center gap-1"><span className="size-2 rounded-sm bg-[#a78bfa]" /> Output</span>
            </div>
          </div>
          <TokenChart points={points} />
        </div>

        {/* Tab content */}
        {tab === "overview" && (
          <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
            <div className="bg-[var(--bg-panel)] border border-[var(--border-default)] rounded p-3">
              <div className="text-[10px] font-bold uppercase tracking-wider text-[var(--fg-tertiary)] mb-2">Workspaces</div>
              <BucketsTable dimension="workspace" filter={filter} />
            </div>
            <div className="bg-[var(--bg-panel)] border border-[var(--border-default)] rounded p-3">
              <div className="text-[10px] font-bold uppercase tracking-wider text-[var(--fg-tertiary)] mb-2">Providers</div>
              <BucketsTable dimension="provider" filter={filter} />
            </div>
            <div className="bg-[var(--bg-panel)] border border-[var(--border-default)] rounded p-3">
              <div className="text-[10px] font-bold uppercase tracking-wider text-[var(--fg-tertiary)] mb-2">Models</div>
              <BucketsTable dimension="model" filter={filter} />
            </div>
          </div>
        )}

        {tab === "workspaces" && (
          <div className="bg-[var(--bg-panel)] border border-[var(--border-default)] rounded p-3">
            <div className="text-[10px] font-bold uppercase tracking-wider text-[var(--fg-tertiary)] mb-2">By Workspace</div>
            <BucketsTable dimension="workspace" filter={filter} />
          </div>
        )}

        {tab === "agents" && (
          <div className="bg-[var(--bg-panel)] border border-[var(--border-default)] rounded p-3">
            <div className="text-[10px] font-bold uppercase tracking-wider text-[var(--fg-tertiary)] mb-2">By Agent</div>
            <BucketsTable dimension="agent" filter={filter} />
          </div>
        )}

        {tab === "providers" && (
          <div className="bg-[var(--bg-panel)] border border-[var(--border-default)] rounded p-3">
            <div className="text-[10px] font-bold uppercase tracking-wider text-[var(--fg-tertiary)] mb-2">By Provider</div>
            <BucketsTable dimension="provider" filter={filter} />
          </div>
        )}

        {tab === "models" && (
          <div className="bg-[var(--bg-panel)] border border-[var(--border-default)] rounded p-3">
            <div className="text-[10px] font-bold uppercase tracking-wider text-[var(--fg-tertiary)] mb-2">By Model</div>
            <BucketsTable dimension="model" filter={filter} />
          </div>
        )}

        {/* Request table */}
        {tab === "requests" && (
          <div className="bg-[var(--bg-panel)] border border-[var(--border-default)] rounded overflow-x-auto">
            <table className="w-full text-[11px] font-mono">
              <thead>
                <tr className="text-left text-[9px] uppercase tracking-wider text-[var(--fg-tertiary)] border-b border-[var(--border-default)]">
                  <th className="px-2 py-1.5">Time</th>
                  <th className="px-2 py-1.5">Workspace</th>
                  <th className="px-2 py-1.5">Provider</th>
                  <th className="px-2 py-1.5">Model</th>
                  <th className="px-2 py-1.5 text-right">In</th>
                  <th className="px-2 py-1.5 text-right">Out</th>
                  <th className="px-2 py-1.5 text-right">Cache</th>
                  <th className="px-2 py-1.5 text-right">Speed</th>
                  <th className="px-2 py-1.5 text-right">Latency</th>
                  <th className="px-2 py-1.5 text-right">Tools</th>
                  <th className="px-2 py-1.5 text-right">Cost</th>
                  <th className="px-2 py-1.5 text-center">Status</th>
                </tr>
              </thead>
              <tbody>
                {requests.map((r) => (
                  <tr key={r.id} className="border-b border-[var(--border-default)]/50 hover:bg-[var(--bg-surface-hover)]">
                    <td className="px-2 py-1 text-[var(--fg-tertiary)]">
                      {new Date(r.timestamp).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}
                    </td>
                    <td className="px-2 py-1 text-[var(--fg-secondary)] truncate max-w-28">{r.workspace || "—"}</td>
                    <td className="px-2 py-1 text-[var(--fg-secondary)] truncate max-w-24">{r.provider || "—"}</td>
                    <td className="px-2 py-1 text-[var(--fg-secondary)] truncate max-w-36">{r.model || "—"}</td>
                    <td className="px-2 py-1 text-right text-[var(--fg-tertiary)]">{fmtNum(r.input_tokens)}</td>
                    <td className="px-2 py-1 text-right text-[var(--fg-tertiary)]">{fmtNum(r.output_tokens)}</td>
                    <td className="px-2 py-1 text-right text-emerald-400">{fmtNum(r.cached_tokens)}</td>
                    <td className="px-2 py-1 text-right text-cyan-400 font-mono">{r.speed_tps ? `${r.speed_tps} tps` : "—"}</td>
                    <td className="px-2 py-1 text-right text-[var(--fg-tertiary)]">{fmtDuration(r.latency_ms)}</td>
                    <td className="px-2 py-1 text-right text-[var(--fg-tertiary)]">{r.tool_calls}</td>
                    <td className="px-2 py-1 text-right text-emerald-400 font-mono">{fmtCost(r.cost_usd)}</td>
                    <td className="px-2 py-1 text-center">
                      {r.success ? (
                        <IconCheck className="size-3.5 text-emerald-400 inline" />
                      ) : (
                        <IconX className="size-3.5 text-red-400 inline" />
                      )}
                    </td>
                  </tr>
                ))}
                {requests.length === 0 && (
                  <tr><td colSpan={11} className="px-2 py-3 text-[10px] text-[var(--fg-tertiary)] italic">No requests in this range</td></tr>
                )}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}
