"""forge_ade.workspace — workspace + recents (~/.forge-ade/workspace.json)."""

from __future__ import annotations

from . import bridge
from .util import data_dir, now_sec, read_json, write_json_file

STATE_PATH = data_dir() / "workspace.json"


def _load_state() -> dict:
    state = read_json(STATE_PATH, default={})
    return state if isinstance(state, dict) else {}


@bridge.cmd("services.GetCurrentWorkspace")
def get_workspace(payload: dict):
    state = _load_state()
    return state.get("workspace") or {"name": "", "folders": []}


@bridge.cmd("services.OpenFolder", "services.OpenWorkspace", "services.PinRecent", "services.RemoveRecent")
def open_workspace(payload: dict):
    # The frontend owns workspace/recent state in localStorage and only uses
    # these calls as hints (the Zig handlers did the same bookkeeping).
    return {"ok": True}


@bridge.cmd("services.SaveWorkspace", "services.SaveWorkspaceAs")
def save_workspace(payload: dict):
    workspace = payload.get("workspace") or payload
    state = _load_state()
    state["workspace"] = workspace
    folder = (workspace.get("folders") or [""])[0] if isinstance(workspace, dict) else ""
    if folder:
        recent = [r for r in state.get("recent", []) if r.get("path") != folder]
        recent.insert(0, {"path": folder, "name": folder.rstrip("/").split("/")[-1],
                          "lastOpened": now_sec(), "pinned": False, "favorite": False})
        state["recent"] = recent[:50]
    write_json_file(STATE_PATH, state)
    return {"ok": True}


@bridge.cmd("services.CloseWorkspace")
def close_workspace(payload: dict):
    state = _load_state()
    state["workspace"] = None
    write_json_file(STATE_PATH, state)
    return {"ok": True}


@bridge.cmd("services.GetRecentProjects")
def recent_projects(payload: dict) -> list:
    return _load_state().get("recent", [])
