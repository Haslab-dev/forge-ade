"""forge_ade.usage — token-usage journal (~/.forge-ade/usage/usage.jsonl)."""

from __future__ import annotations

import json
import time
from pathlib import Path

from . import bridge
from .util import data_dir, now_ms

USAGE_PATH = data_dir() / "usage" / "usage.jsonl"


def _read_records() -> list[dict]:
    try:
        raw = USAGE_PATH.read_text(encoding="utf-8", errors="replace")
    except OSError:
        return []
    out = []
    for line in raw.split("\n"):
        line = line.strip()
        if not line:
            continue
        try:
            obj = json.loads(line)
        except json.JSONDecodeError:
            continue
        if isinstance(obj, dict):
            out.append(obj)
    return out


@bridge.cmd("services.GetAllUsageRecords")
def get_all_records(payload: dict) -> list[dict]:
    return [
        {"ts": r.get("ts"), "provider": r.get("provider", ""), "model": r.get("model", ""),
         "workspace": r.get("workspace", ""), "sessionId": r.get("sessionId", ""),
         "inputTokens": r.get("inputTokens", 0), "outputTokens": r.get("outputTokens", 0),
         "cachedTokens": r.get("cachedTokens", 0), "latencyMs": r.get("latencyMs", 0)}
        for r in _read_records()
    ]


def _summarize(records: list[dict]) -> dict:
    by_provider: dict[str, dict] = {}
    by_model: dict[str, dict] = {}
    by_workspace: dict[str, dict] = {}
    total_prompt = total_completion = total_cached = 0
    for r in records:
        p = r.get("promptTokens", 0)
        c = r.get("completionTokens", 0)
        ca = r.get("cachedTokens", 0)
        total_prompt += p
        total_completion += c
        total_cached += ca
        for bucket, key in ((by_provider, r.get("provider", "")),
                            (by_model, r.get("model", "")),
                            (by_workspace, r.get("workspace", ""))):
            b = bucket.setdefault(key, {"prompt": 0, "completion": 0, "cached": 0, "requests": 0})
            b["prompt"] += p
            b["completion"] += c
            b["cached"] += ca
            b["requests"] += 1
    denom = total_prompt + total_cached
    return {
        "totalPromptTokens": total_prompt,
        "totalCompletionTokens": total_completion,
        "totalCachedTokens": total_cached,
        "totalTokens": total_prompt + total_completion,
        "cacheHitRate": round((total_cached / denom) * 100, 1) if denom else 0,
        "requestCount": len(records),
        "byProvider": by_provider,
        "byModel": by_model,
        "byWorkspace": by_workspace,
    }


@bridge.cmd("services.GetUsageSummary")
def get_usage_summary(payload: dict) -> dict:
    records = [
        {"promptTokens": r.get("inputTokens", 0), "completionTokens": r.get("outputTokens", 0),
         "cachedTokens": r.get("cachedTokens", 0), "provider": r.get("provider", ""),
         "model": r.get("model", ""), "workspace": r.get("workspace", "")}
        for r in _read_records()
    ]
    return _summarize(records)


@bridge.cmd("services.GetUsageOverview")
def get_usage_overview(payload: dict) -> dict:
    summary = get_usage_summary({})
    return {
        "totalTokens": summary["totalTokens"],
        "inputTokens": summary["totalPromptTokens"],
        "outputTokens": summary["totalCompletionTokens"],
        "cachedTokens": summary["totalCachedTokens"],
        "totalCost": 0,
        "requestCount": summary["requestCount"],
        "avgLatencyMs": 0,
    }


def record_usage(provider: str, model: str, workspace: str, session_id: str,
                 input_tokens: int, output_tokens: int, cached_tokens: int,
                 latency_ms: int) -> None:
    """Appends a journal row (called from the agent turn loop)."""
    row = {
        "ts": now_ms(), "provider": provider, "model": model, "workspace": workspace,
        "sessionId": session_id, "inputTokens": input_tokens,
        "outputTokens": output_tokens, "cachedTokens": cached_tokens,
        "latencyMs": latency_ms,
    }
    USAGE_PATH.parent.mkdir(parents=True, exist_ok=True)
    existing = USAGE_PATH.read_bytes() if USAGE_PATH.exists() else b""
    with open(USAGE_PATH, "ab") as f:
        f.write(existing + (json.dumps(row) + "\n").encode())
