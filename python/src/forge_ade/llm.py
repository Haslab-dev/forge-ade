"""forge_ade.llm — provider profiles + active model (~/.forge-ade/models.json).

Port of llm.zig + the LLM parts of misc.zig. All config mutation rewrites the
file atomically from fully-owned Python dicts — the cross-allocator map
mutation crashes of the Zig port don't exist here.
"""

from __future__ import annotations

import json
from typing import Any

from . import bridge
from .llm_client import stream_chat
from .util import data_dir, read_json, run_shell, write_json_file, workspace_root

MODELS_PATH = data_dir() / "models.json"


def _load_models() -> dict:
    doc = read_json(MODELS_PATH, default={})
    return doc if isinstance(doc, dict) else {}


def _save_models(doc: dict) -> None:
    write_json_file(MODELS_PATH, doc)


def _providers(doc: dict) -> dict:
    providers = doc.get("providers")
    return providers if isinstance(providers, dict) else {}


def _profile_shape(pid: str, rec: dict) -> dict:
    models = rec.get("models") or []
    catalog_ids = [m if isinstance(m, str) else (m or {}).get("id", "") for m in models]
    selected = rec.get("selected_models") or []
    all_ids = list(dict.fromkeys([i for i in catalog_ids if i] + [s for s in selected if s]))
    return {
        "id": pid,
        "name": rec.get("name") or pid,
        "provider": rec.get("api") or "openai-completions",
        "apiKey": rec.get("api_key") or "",
        "baseURL": rec.get("base_url") or "",
        "activeModel": rec.get("active_model") or "",
        "models": models,
        "selected_models": selected or all_ids,
        "enabled": rec.get("enabled", True),
        "projectId": rec.get("projectId"),
        "accountEmail": rec.get("accountEmail"),
        "refreshToken": rec.get("refreshToken"),
    }


@bridge.cmd("services.GetProviderProfiles", "services.ListProviderProfiles", "services.ListLLMProviders")
def list_provider_profiles(payload: dict) -> list:
    doc = _load_models()
    return [_profile_shape(pid, rec) for pid, rec in _providers(doc).items()]


@bridge.cmd("services.GetLLMConfig")
def get_llm_config(payload: dict) -> dict:
    doc = _load_models()
    providers = _providers(doc)
    default_id = doc.get("default_provider") or ""
    rec = providers.get(default_id) if default_id else None
    if rec is None and providers:
        default_id, rec = next(iter(providers.items()))
    if rec is None:
        return {
            "provider_id": "", "api_key": "", "base_url": "", "model": "",
            "activeProfile": None, "profiles": [],
        }
    profile = _profile_shape(default_id, rec)
    return {
        "provider_id": profile["id"],
        "api_key": profile["apiKey"],
        "base_url": profile["baseURL"],
        "model": profile["activeModel"],
        "activeProfile": profile,
        "profiles": [],
    }


@bridge.cmd("services.SetActiveModel")
def set_active_model(payload: dict):
    doc = _load_models()
    pid = payload.get("providerId", "")
    model = payload.get("model", "")
    doc["default_provider"] = pid
    rec = _providers(doc).get(pid)
    if isinstance(rec, dict):
        rec["active_model"] = model
    _save_models(doc)
    return {"ok": True}


@bridge.cmd("services.SaveProviderProfiles")
def save_provider_profiles(payload: dict):
    doc = _load_models()
    providers = _providers(doc)
    for profile in payload.get("profiles") or []:
        pid = profile.get("id") or profile.get("provider_id")
        if not pid:
            continue
        rec: dict = {}
        for src_key, dst_key in (
            ("name", "name"), ("api_key", "api_key"), ("base_url", "base_url"),
            ("active_model", "active_model"), ("enabled", "enabled"),
            ("projectId", "projectId"), ("accountEmail", "accountEmail"),
            ("refreshToken", "refreshToken"),
        ):
            value = profile.get(src_key)
            if value is None and src_key in ("api_key", "base_url"):
                value = profile.get("apiKey" if src_key == "api_key" else "baseURL")
            if value is not None:
                rec[dst_key] = value
        api = profile.get("api") or profile.get("provider")
        if api:
            rec["api"] = api
        models = profile.get("models")
        existing = providers.get(pid) or {}
        if models:
            rec["models"] = models
        elif "models" in existing:
            rec["models"] = existing["models"]
        if profile.get("selected_models") is not None:
            rec["selected_models"] = profile["selected_models"]
        elif "selected_models" in existing:
            rec["selected_models"] = existing["selected_models"]
        if "active_model" not in rec and "active_model" in existing:
            rec["active_model"] = existing["active_model"]
        rec.setdefault("enabled", True)
        providers[pid] = rec
    doc["providers"] = providers
    _save_models(doc)
    return {"ok": True}


@bridge.cmd("services.SaveLLMProfile")
def save_llm_profile(payload: dict):
    pid = payload.get("providerId", "")
    if not pid:
        raise RuntimeError("providerId required")
    doc = _load_models()
    providers = _providers(doc)
    rec = providers.get(pid) or {}
    if payload.get("apiKey"):
        rec["api_key"] = payload["apiKey"]
    if payload.get("baseURL"):
        rec["base_url"] = payload["baseURL"]
    if payload.get("model"):
        rec["active_model"] = payload["model"]
    rec.setdefault("name", pid)
    rec.setdefault("api", "openai-completions")
    rec.setdefault("enabled", True)
    providers[pid] = rec
    doc["providers"] = providers
    doc["default_provider"] = pid
    _save_models(doc)
    return {"ok": True}


def load_active_target() -> dict | None:
    """Resolves {providerId, baseURL, apiKey, model} for the active provider."""
    doc = _load_models()
    providers = _providers(doc)
    default_id = doc.get("default_provider") or ""
    rec = providers.get(default_id) if default_id else None
    if rec is None and providers:
        _, rec = next(iter(providers.items()))
    if not isinstance(rec, dict):
        return None
    base_url = rec.get("base_url") or ""
    api_key = rec.get("api_key") or ""
    model = rec.get("active_model") or ""
    if not (base_url and api_key and model):
        return None
    return {
        "providerId": default_id or rec.get("name") or "",
        "baseURL": base_url,
        "apiKey": api_key,
        "model": model,
    }


@bridge.cmd("services.GenerateAICommitMessage")
def generate_ai_commit_message(payload: dict) -> str:
    repo = payload.get("repoPath") or workspace_root()
    diff = run_shell("git diff --cached --stat", repo)["output"]
    status = run_shell("git status --porcelain=v1", repo)["output"]

    target = load_active_target()
    if target is None:
        raise RuntimeError("no active LLM provider configured")
    if payload.get("model"):
        target["model"] = payload["model"]

    instruction = payload.get("instruction") or "Follow conventional commits (feat/fix/refactor/etc)."
    messages = [
        {
            "role": "system",
            "content": "You write concise conventional git commit messages. Output ONLY the commit message, no explanation, no quotes.",
        },
        {
            "role": "user",
            "content": f"Write a commit message for:\n{diff}\n\nWorking tree:\n{status}\n\nInstruction: {instruction}",
        },
    ]
    result = stream_chat(target, messages, tools=[])
    content = (result.get("content") or "").strip().strip("\"'` \r\n\t")
    if not content:
        raise RuntimeError("AI commit generation returned an empty response — the provider may have failed silently")
    return content


# OAuth / quotas — not migrated yet (bootstrap parity with the Zig layer).
@bridge.cmd(
    "services.StartOAuthLogin", "services.GetOAuthStatus", "services.SubmitOAuthManualCode",
    "services.GetProviderQuota", "services.GetAllProviderQuotas", "services.FetchProviderModels",
)
def not_migrated(payload: dict):
    raise RuntimeError("not yet migrated to the pytauri backend")
