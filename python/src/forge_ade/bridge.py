"""forge_ade.bridge — the wire bridge.

One Tauri command (`bridge`) receives {command, payload-json} and dispatches
to a Python handler registry keyed by the EXACT command names the frontend
used over the Native SDK bridge (`fs.readDir`, `terminal.spawn`,
`services.ListAgentSessions`, ...). Handlers return JSON-serializable values;
the response is always a JSON string:

    {"ok": true, "result": <value>}
    {"ok": false, "error": {"message": "..."}}

Streaming events (terminal data, fs changes, agent turns, LSP diagnostics)
are emitted to the main window with the SAME event names the Native SDK used:
"terminal.data", "terminal.exit", "fs.change", "services.agent", "lsp.data".
"""

from __future__ import annotations

import json
import re
import threading
from typing import Any, Callable

from pydantic import BaseModel

from pytauri import AppHandle, Commands

# Tauri only allows alphanumeric, `-`, `/`, `:` and `_` in event names —
# the Native SDK names use dots (terminal.data, fs.change, services.agent),
# so both this side and the frontend shim map invalid chars to `_`.
_EVENT_NAME_OK = re.compile(r"[^a-zA-Z0-9\-/:_]")


def _safe_event_name(event: str) -> str:
    return _EVENT_NAME_OK.sub("_", event)

commands = Commands()

HANDLERS: dict[str, Callable[[dict], Any]] = {}


def cmd(*names: str) -> Callable:
    """Register a handler under one or more bridge command names."""

    def deco(fn: Callable[[dict], Any]) -> Callable[[dict], Any]:
        for name in names:
            HANDLERS[name] = fn
        return fn

    return deco


class BridgeArgs(BaseModel):
    command: str
    payload: str = "{}"


def dispatch(args: BridgeArgs) -> str:
    print(f"[bridge] << {args.command}", flush=True)
    handler = HANDLERS.get(args.command)
    if handler is None:
        print(f"[bridge] !! unknown command {args.command}", flush=True)
        return json.dumps(
            {"ok": False, "error": {"message": f"unknown command: {args.command}"}}
        )
    try:
        payload = json.loads(args.payload) if args.payload else {}
    except json.JSONDecodeError:
        payload = {}
    if not isinstance(payload, dict):
        payload = {"value": payload}
    try:
        result = handler(payload)
        return json.dumps({"ok": True, "result": result}, default=str)
    except Exception as exc:  # noqa: BLE001 — the wire contract requires a message
        import traceback

        print(f"[bridge] !! {args.command}: {exc}\n{traceback.format_exc()}", flush=True)
        return json.dumps({"ok": False, "error": {"message": str(exc) or exc.__class__.__name__}})


@commands.command()
async def bridge(body: BridgeArgs) -> str:
    """Single entry point for every frontend bridge call."""
    import anyio

    return await anyio.to_thread.run_sync(dispatch, body)


# ---------------------------------------------------------------------------
# Event emission (background threads → main window)
# ---------------------------------------------------------------------------

_app_handle: AppHandle | None = None
_emit_lock = threading.Lock()


def set_app_handle(handle: AppHandle) -> None:
    global _app_handle
    _app_handle = handle


def app_handle() -> AppHandle | None:
    return _app_handle


def emit_str(event: str, payload: str) -> None:
    """Emit a raw JSON-string payload to the main window (Native SDK parity)."""
    handle = _app_handle
    if handle is None:
        return
    safe_name = _safe_event_name(event)
    with _emit_lock:
        try:
            handle.emit_str_to("main", safe_name, payload)
        except Exception:
            try:
                handle.emit_str(safe_name, payload)
            except Exception:
                pass


def emit(event: str, payload: Any) -> None:
    try:
        emit_str(event, json.dumps(payload, default=str))
    except Exception:
        pass


def emit_service(event: str, payload: Any) -> None:
    """Emit a service event wrapped as {event, payload} on the services.agent
    channel — the frontend's zero.on("services.agent") handler dispatches by
    the inner event name."""
    emit("services.agent", {"event": event, "payload": payload})
