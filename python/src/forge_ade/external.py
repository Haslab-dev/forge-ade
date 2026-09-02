"""forge_ade.external — external ACP agents (opencode, omp, pi, codex, ...).

Port of acp-client.zig + external-agent.zig: newline-delimited JSON-RPC over
the agent process's stdio, session/new + session/prompt, and the client-side
fs/permission requests the agent sends us.
"""

from __future__ import annotations

import json
import os
import subprocess
import threading
import time
from pathlib import Path

from . import bridge
from .util import home_dir, read_file_bounded, write_file_atomic

PROTOCOL_VERSION = 1

AGENTS = [
    {"id": "omp", "name": "Oh-My-Pi", "description": "Oh-My-Pi coding agent via native ACP mode (omp acp)", "command": "omp", "args": ["acp"]},
    {"id": "opencode", "name": "OpenCode", "description": "OpenCode agent via ACP (runs via npx, no install needed)", "command": "npx", "args": ["-y", "opencode-ai", "acp"]},
    {"id": "codex", "name": "Codex", "description": "OpenAI Codex via the official codex-acp adapter (npx)", "command": "npx", "args": ["-y", "@agentclientprotocol/codex-acp"]},
    {"id": "claude-code", "name": "Claude Code", "description": "Claude Agent SDK via the official claude-agent-acp adapter (npx)", "command": "npx", "args": ["-y", "@agentclientprotocol/claude-agent-acp"]},
    {"id": "pi", "name": "Pi", "description": "Pi coding agent via the pi-acp adapter", "command": "pi-acp", "args": []},
    {"id": "antigravity", "name": "Antigravity", "description": "Google Antigravity via the antigravity-acp bridge (npx; wraps agy)", "command": "npx", "args": ["-y", "antigravity-acp"]},
]


def _find_agent(agent_id: str) -> dict | None:
    return next((a for a in AGENTS if a["id"] == agent_id), None)


class AcpConnection:
    def __init__(self, agent_id: str, command: str, args: list[str]) -> None:
        self.agent_id = agent_id
        self.agent_name = ""
        self.proc = subprocess.Popen(
            [command, *args],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            cwd=os.environ.get("PWD") or os.getcwd(),
        )
        self._lock = threading.Lock()
        self._responses: dict[int, dict] = {}
        self._errors: dict[int, str] = {}
        self._requests: list[dict] = []
        self._next_id = 1
        self.alive = True
        self.sessions: dict[str, dict] = {}
        self._reader_done = threading.Event()
        threading.Thread(target=self._reader, daemon=True).start()

    def _reader(self) -> None:
        assert self.proc.stdout is not None
        for raw in self.proc.stdout:
            try:
                frame = json.loads(raw.decode("utf-8", errors="replace"))
            except json.JSONDecodeError:
                continue
            if not isinstance(frame, dict):
                continue
            with self._lock:
                if "method" in frame and "id" in frame:
                    self._requests.append({
                        "id": frame["id"], "method": frame.get("method", ""),
                        "params": frame.get("params"),
                    })
                    continue
                idv = frame.get("id")
                if isinstance(idv, int):
                    if "error" in frame:
                        self._errors[idv] = str((frame.get("error") or {}).get("message", "rpc error"))
                    elif "result" in frame:
                        self._responses[idv] = frame["result"]
        self.alive = False
        self._reader_done.set()

    def _send(self, obj: dict) -> None:
        if not self.alive or self.proc.stdin is None:
            return
        try:
            self.proc.stdin.write((json.dumps(obj) + "\n").encode())
            self.proc.stdin.flush()
        except (BrokenPipeError, OSError):
            self.alive = False

    def request(self, method: str, params: dict, timeout_ms: int) -> dict:
        with self._lock:
            id_ = self._next_id
            self._next_id += 1
        self._send({"jsonrpc": "2.0", "id": id_, "method": method, "params": params})
        deadline = time.time() + timeout_ms / 1000.0
        while time.time() < deadline:
            with self._lock:
                if id_ in self._errors:
                    raise RuntimeError(f"acp request failed: {self._errors.pop(id_)}")
                if id_ in self._responses:
                    return self._responses.pop(id_)
                alive = self.alive
            if not alive:
                raise RuntimeError("NotConnected")
            time.sleep(0.005)
        raise TimeoutError(f"acp request timed out: {method}")

    def notify(self, method: str, params: dict) -> None:
        self._send({"jsonrpc": "2.0", "method": method, "params": params})

    def drain_requests(self) -> None:
        cwd = os.environ.get("PWD") or os.getcwd()
        while True:
            with self._lock:
                if not self._requests:
                    return
                req = self._requests.pop(0)
            self._handle_request(req, cwd)

    def _handle_request(self, req: dict, cwd: str) -> None:
        method, params = req["method"], req.get("params") or {}
        if method == "fs/read_text_file":
            path = os.path.join(cwd, params.get("path", "")) if not params.get("path", "").startswith("/") else params.get("path", "")
            try:
                content = read_file_bounded(path).decode("utf-8", errors="replace")
                self._send({"jsonrpc": "2.0", "id": req["id"], "result": {"content": content}})
            except OSError:
                self._reply_error(req["id"], "read failed")
        elif method == "fs/write_text_file":
            path = params.get("path", "")
            if not path.startswith("/"):
                path = os.path.join(cwd, path)
            write_file_atomic(path, params.get("content", ""))
            self._send({"jsonrpc": "2.0", "id": req["id"], "result": {}})
        elif method == "session/request_permission":
            chosen = None
            for option in params.get("options") or []:
                kind = option.get("kind", "")
                if kind in ("allow_once", "allow_always"):
                    chosen = option.get("optionId")
                    break
                if chosen is None and kind not in ("reject_once", "reject_always"):
                    chosen = option.get("optionId")
            if chosen:
                self._send({"jsonrpc": "2.0", "id": req["id"],
                            "result": {"outcome": {"outcome": "selected", "optionId": chosen}}})
            else:
                self._send({"jsonrpc": "2.0", "id": req["id"],
                            "result": {"outcome": {"outcome": "cancelled"}}})
        elif method == "session/set_mode":
            self._send({"jsonrpc": "2.0", "id": req["id"], "result": {}})
        else:
            self._reply_error(req["id"], "method not supported")

    def _reply_error(self, id_: int, message: str) -> None:
        self._send({"jsonrpc": "2.0", "id": id_, "error": {"code": -32603, "message": message}})

    def close(self) -> None:
        self.alive = False
        try:
            self.proc.terminate()
        except OSError:
            pass
        self._reader_done.wait(timeout=2.0)
        try:
            self.proc.kill()
        except OSError:
            pass


_CONN_LOCK = threading.Lock()
_CONNECTIONS: dict[str, AcpConnection] = {}


def _resolve_command(command: str) -> str:
    if "/" in command and os.path.exists(command):
        return command
    home = home_dir()
    for base in ("/opt/homebrew/bin", "/usr/local/bin", f"{home}/.bun/bin",
                 f"{home}/.local/bin", "/usr/bin", f"{home}/homebrew/bin"):
        cand = f"{base}/{command}"
        if os.path.exists(cand):
            return cand
    return command


def get_connection(agent: dict) -> AcpConnection:
    with _CONN_LOCK:
        existing = _CONNECTIONS.get(agent["id"])
        if existing is not None:
            return existing
        conn = AcpConnection(agent["id"], _resolve_command(agent["command"]), agent["args"])
        try:
            result = conn.request("initialize", {
                "protocolVersion": PROTOCOL_VERSION,
                "clientCapabilities": {
                    "fs": {"readTextFile": True, "writeTextFile": True},
                    "terminal": False,
                    "session": {"configOptions": {"boolean": {}}},
                },
                "clientInfo": {"name": "ForgeADE", "version": "0.6.0"},
            }, 20_000)
            if isinstance(result.get("agentInfo"), dict):
                conn.agent_name = result["agentInfo"].get("name", "")
        except Exception:
            conn.close()
            raise
        _CONNECTIONS[agent["id"]] = conn
        return conn


# ---------------------------------------------------------------------------
# Wire handlers
# ---------------------------------------------------------------------------


@bridge.cmd("services.ListExternalAgents")
def list_agents(payload: dict) -> list:
    return AGENTS


@bridge.cmd("services.CreateExternalAgentSession")
def create_external_session(payload: dict):
    from .agent import _make_session, _meta_of

    agent = _find_agent(payload.get("agentId", ""))
    if agent is None:
        raise RuntimeError("unknown external agent")
    conn = get_connection(agent)
    acp_session_id = conn.request(
        "session/new", {"cwd": payload.get("projectFolder", ""), "mcpServers": []}, 30_000
    ).get("sessionId", "")
    if not acp_session_id:
        raise RuntimeError("external agent session/new failed")
    conn.drain_requests()

    name = payload.get("name") or agent["name"]
    session_id = f"agent-{int(time.time() * 1000)}"
    session = _make_session(session_id, name, f"external:{agent['id']}", payload.get("projectFolder", ""))
    _store_acp_id(session_id, acp_session_id)
    bridge.emit("session:opened", _meta_of(session))
    bridge.emit_service("agent:updated", {"id": session_id})
    return session


def _acp_id_path(session_id: str) -> Path:
    from .agent import _session_path

    return _session_path(session_id)


def _store_acp_id(session_id: str, acp_id: str) -> None:
    path = _acp_id_path(session_id)
    lines = []
    first = True
    for entry in path.read_text().split("\n") if path.exists() else []:
        if not entry.strip():
            continue
        obj = json.loads(entry)
        if first and obj.get("type") == "session":
            obj["acpSessionId"] = acp_id
        lines.append(json.dumps(obj, ensure_ascii=False))
        first = False
    write_file_atomic(path, "\n".join(lines) + "\n")


def _read_acp_id(session_id: str) -> str | None:
    from .agent import _session_path

    path = _session_path(session_id)
    if not path.exists():
        return None
    for entry in path.read_text().split("\n"):
        if not entry.strip():
            continue
        try:
            obj = json.loads(entry)
        except json.JSONDecodeError:
            continue
        if obj.get("type") == "session":
            return obj.get("acpSessionId")
    return None


@bridge.cmd("services.GetExternalAgentState")
def get_external_state(payload: dict):
    from .agent import _load_session

    session = _load_session(payload.get("id", ""))
    if session is None:
        raise RuntimeError("session not found")
    role = session["role"]
    if not role.startswith("external:"):
        return {"configOptions": [], "availableCommands": []}
    agent = _find_agent(role[len("external:"):])
    if agent is None:
        raise RuntimeError("unknown external agent")
    conn = get_connection(agent)
    acp_id = _read_acp_id(session["id"])
    state = conn.sessions.get(acp_id or "", {})
    return {"configOptions": state.get("configOptions", []), "availableCommands": state.get("availableCommands", [])}


@bridge.cmd("services.SetExternalAgentConfig")
def set_external_config(payload: dict):
    from .agent import _load_session

    session = _load_session(payload.get("id", ""))
    if session is None:
        raise RuntimeError("session not found")
    role = session["role"]
    if not role.startswith("external:"):
        return {"configOptions": [], "availableCommands": []}
    agent = _find_agent(role[len("external:"):])
    if agent is None:
        raise RuntimeError("unknown external agent")
    conn = get_connection(agent)
    acp_id = _read_acp_id(session["id"]) or ""
    result = conn.request("session/set_config_option", {
        "sessionId": acp_id, "configId": payload.get("configId", ""),
        "value": payload.get("value"),
    }, 30_000)
    conn.sessions[acp_id] = {"configOptions": result.get("configOptions", [])}
    return {"configOptions": result.get("configOptions", []), "availableCommands": []}


def send_external_message(session_id: str, text: str) -> None:
    from .agent import _load_session

    session = _load_session(session_id)
    if session is None:
        raise RuntimeError("SessionNotFound")
    role = session["role"]
    if not role.startswith("external:"):
        raise RuntimeError("NotExternalSession")
    agent = _find_agent(role[len("external:"):])
    if agent is None:
        raise RuntimeError("UnknownAgent")
    conn = get_connection(agent)
    acp_id = _read_acp_id(session_id)
    if not acp_id:
        raise RuntimeError("NoAcpSession")
    conn.drain_requests()
    conn.request("session/prompt", {"sessionId": acp_id,
                                    "prompt": [{"type": "text", "text": text}]}, 0)
    conn.drain_requests()
    bridge.emit_service("agent:turn_end", {"id": session_id, "ok": True})
