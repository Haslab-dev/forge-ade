"""forge_ade.misc_cmds — editor helpers, file ops, slash commands, git helpers."""

from __future__ import annotations

import base64
import os
import shlex
from pathlib import Path

from . import bridge
from .util import run_shell, workspace_root


# ---------------------------------------------------------------------------
# Editor helpers
# ---------------------------------------------------------------------------

_COMPLETION_ITEMS = [
    {"Name": "console.log", "Kind": "snippet", "Detail": "console.log(...)"},
    {"Name": "function", "Kind": "keyword", "Detail": "function declaration"},
    {"Name": "import", "Kind": "keyword", "Detail": "import statement"},
    {"Name": "export", "Kind": "keyword", "Detail": "export statement"},
    {"Name": "interface", "Kind": "keyword", "Detail": "interface declaration"},
    {"Name": "const", "Kind": "keyword", "Detail": "const declaration"},
    {"Name": "let", "Kind": "keyword", "Detail": "let declaration"},
    {"Name": "return", "Kind": "keyword", "Detail": "return statement"},
]


@bridge.cmd("services.GetCompletion")
def get_completion(payload: dict) -> list:
    prefix = (payload.get("prefix") or "").lower()
    return [item for item in _COMPLETION_ITEMS if item["Name"].lower().startswith(prefix)]


@bridge.cmd("services.GetMembers")
def get_members(payload: dict) -> list:
    return [
        {"Name": "length", "Kind": "property", "Detail": "number"},
        {"Name": "toString", "Kind": "method", "Detail": "(): string"},
        {"Name": "map", "Kind": "method", "Detail": "(fn) => []"},
        {"Name": "filter", "Kind": "method", "Detail": "(fn) => []"},
        {"Name": "forEach", "Kind": "method", "Detail": "(fn) => void"},
        {"Name": "find", "Kind": "method", "Detail": "(fn) => T | undefined"},
    ]


# ---------------------------------------------------------------------------
# File ops
# ---------------------------------------------------------------------------


@bridge.cmd("services.ListDirectory")
def list_directory(payload: dict) -> list[dict]:
    directory = payload.get("path", "")
    nodes = []
    try:
        with os.scandir(directory) as it:
            for entry in it:
                if entry.name in (".", ".."):
                    continue
                nodes.append({
                    "path": os.path.join(directory, entry.name),
                    "name": entry.name,
                    "isDir": entry.is_dir(follow_symlinks=False),
                    "size": entry.stat(follow_symlinks=False).st_size,
                    "modTime": int(entry.stat(follow_symlinks=False).st_mtime * 1000),
                    "gitIgnored": False,
                    "hidden": entry.name.startswith("."),
                })
    except OSError:
        return []
    return nodes


@bridge.cmd("services.ReadFileBase64")
def read_file_base64(payload: dict) -> str:
    path = payload.get("path", "")
    try:
        data = Path(path).read_bytes()
    except OSError as exc:
        raise RuntimeError("read failed") from exc
    return base64.b64encode(data).decode()


@bridge.cmd("services.CreateFile")
def create_file(payload: dict):
    path = payload.get("path", "")
    if not path:
        raise RuntimeError("path required")
    p = Path(path)
    p.parent.mkdir(parents=True, exist_ok=True)
    p.touch(exist_ok=True)
    return {"ok": True}


@bridge.cmd("services.CreateFolder")
def create_folder(payload: dict):
    path = payload.get("path", "")
    if not path:
        raise RuntimeError("path required")
    Path(path).mkdir(parents=True, exist_ok=True)
    return {"ok": True}


# ---------------------------------------------------------------------------
# Slash commands
# ---------------------------------------------------------------------------


@bridge.cmd("services.ListSlashCommands")
def list_slash_commands(payload: dict) -> list[dict]:
    return [
        {"name": "yolo", "description": "Auto-approve all tool calls for this turn", "kind": "toggle"},
        {"name": "clear", "description": "Clear the current session transcript", "kind": "action"},
        {"name": "compact", "description": "Summarize older messages to save context", "kind": "action"},
    ]


@bridge.cmd("services.ExecuteSlashCommand")
def execute_slash_command(payload: dict) -> dict:
    text = payload.get("text", "")
    if text.startswith("/clear"):
        return {"handled": True, "message": "cleared"}
    if text.startswith("/yolo"):
        return {"handled": True, "message": "yolo mode enabled"}
    return {"handled": False}


@bridge.cmd("services.OpenExternalURL")
def open_external_url(payload: dict):
    return {"ok": True}


@bridge.cmd("services.RespondAgentAsk")
def respond_agent_ask(payload: dict):
    return {"ok": True}


# ---------------------------------------------------------------------------
# Git helpers
# ---------------------------------------------------------------------------


def _repo(payload: dict) -> str:
    return payload.get("repoPath") or workspace_root()


@bridge.cmd("services.GitCommit")
def git_commit(payload: dict):
    repo = _repo(payload)
    amend = " --amend" if payload.get("amend") else ""
    res = run_shell(f"git commit{amend} -m {shlex.quote(payload.get('message', ''))}", repo)
    if not res["success"]:
        raise RuntimeError("git commit failed")
    return "ok"


@bridge.cmd("services.GitPush")
def git_push(payload: dict) -> str:
    repo = _repo(payload)
    force = " --force" if payload.get("force") else ""
    res = run_shell(f"git push{force}", repo)
    if not res["success"]:
        raise RuntimeError("git push failed")
    return res["output"]


@bridge.cmd("services.GitFetch")
def git_fetch(payload: dict) -> str:
    res = run_shell("git fetch", _repo(payload))
    if not res["success"]:
        raise RuntimeError("git fetch failed")
    return res["output"]


@bridge.cmd("services.GitMerge")
def git_merge(payload: dict) -> str:
    repo = _repo(payload)
    flags = (" --no-ff" if payload.get("noFF") else "") + (" --squash" if payload.get("squash") else "")
    res = run_shell(f"git merge{flags} {payload.get('source', '')}", repo)
    if not res["success"]:
        raise RuntimeError("git merge failed")
    return res["output"]
