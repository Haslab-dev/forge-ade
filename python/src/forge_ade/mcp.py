"""forge_ade.mcp — multi-source MCP discovery + stdio JSON-RPC client.

Port of mcp.zig + mcp-client.zig with the concurrency fixes from the Zig
crash audit applied from the start:
- response values stored in the shared map are deep copies (never aliased to
  a parse arena — Python GC makes this structural, but values are still
  consumed via pop() so ownership is explicit),
- the connection cache key is an owned copy of the discovered name,
- Connection.deinit waits for the reader thread before dropping state.
"""

from __future__ import annotations

import json
import os
import re
import shlex
import subprocess
import threading
import time
from pathlib import Path

from . import bridge
from .util import data_dir, home_dir, read_file_bounded, workspace_root

PROTOCOL_VERSION = "2024-11-05"

_SKIP_JSON = re.compile(r"(?m)(^|(?<=\s))//[^\n]*|/\*.*?\*/", re.S)
_TRAILING_COMMA = re.compile(r",\s*(?=[}\]])")

# ---------------------------------------------------------------------------
# Discovery
# ---------------------------------------------------------------------------

SKIP_DIRS = {".git", "node_modules", "zig-out", ".zig-cache", ".native", "dist", "build",
             "Pods", ".gradle", "DerivedData", ".build", ".swiftpm", "Carthage", ".yarn",
             "vendor", "__pycache__", ".DS_Store", ".idea", ".vscode", ".cache", ".next",
             ".nuxt", ".turbo", "coverage", ".venv", "venv", "target", ".dart_tool"}


def _strip_jsonc(text: str) -> str:
    cleaned = []
    in_str = False
    esc = False
    i = 0
    while i < len(text):
        ch = text[i]
        if in_str:
            cleaned.append(ch)
            if esc:
                esc = False
            elif ch == "\\":
                esc = True
            elif ch == '"':
                in_str = False
            i += 1
            continue
        if ch == '"':
            in_str = True
            cleaned.append(ch)
            i += 1
            continue
        if ch == "/" and i + 1 < len(text) and text[i + 1] == "/":
            while i < len(text) and text[i] != "\n":
                i += 1
            continue
        if ch == "/" and i + 1 < len(text) and text[i + 1] == "*":
            i += 2
            while i + 1 < len(text) and not (text[i] == "*" and text[i + 1] == "/"):
                i += 1
            i += 2
            continue
        cleaned.append(ch)
        i += 1
    return _TRAILING_COMMA.sub("", "".join(cleaned))


def _load_jsonc(path: str):
    try:
        raw = read_file_bounded(path).decode("utf-8", errors="replace")
    except OSError:
        return None
    try:
        return json.loads(_strip_jsonc(raw))
    except json.JSONDecodeError:
        return None


def _expand_env(value: str) -> str:
    return os.path.expandvars(value) if "$" in value else value


def _normalize_server(name: str, raw: dict, source: str) -> dict | None:
    enabled = raw.get("enabled", True)
    if "disabled" in raw:
        enabled = not raw.get("disabled", False)
    url = raw.get("url") or ""
    if url:
        return {"name": name, "url": url, "enabled": enabled, "source": source,
                "command": "", "args": [], "env": None}
    command = raw.get("command") or ""
    if not command:
        return None
    args = raw.get("args") or []
    if isinstance(args, str):
        args = shlex.split(args)
    env_raw = raw.get("env") or raw.get("environment")
    env = None
    if isinstance(env_raw, dict):
        env = {k: _expand_env(v) for k, v in env_raw.items() if isinstance(v, str)}
    # opencode: command is an ARRAY [cmd, arg1, ...]
    if isinstance(command, list):
        if not command:
            return None
        args = [str(c) for c in command[1:]] + args
        command = str(command[0])
    return {"name": name, "url": "", "enabled": enabled, "source": source,
            "command": str(command), "args": [str(a) for a in args], "env": env}


def discover() -> dict[str, dict]:
    home = home_dir()
    cwd = workspace_root()
    json_paths: list[tuple[str, str]] = [
        ("native:project", f"{cwd}/mcp.json"),
        ("native:project", f"{cwd}/.mcp.json"),
        ("native:user", str(data_dir() / "mcp.json")),
        ("native:user-omp", f"{home}/.omp/agent/mcp.json"),
        ("claude:user", f"{home}/.claude.json"),
        ("claude:user-dir", f"{home}/.claude/mcp.json"),
        ("claude:project", f"{cwd}/.claude/mcp.json"),
        ("claude:project-alt", f"{cwd}/.claude/.mcp.json"),
        ("cursor:user", f"{home}/.cursor/mcp.json"),
        ("cursor:project", f"{cwd}/.cursor/mcp.json"),
        ("windsurf:user", f"{home}/.codeium/windsurf/mcp_config.json"),
        ("gemini:user", f"{home}/.gemini/settings.json"),
        ("gemini:project", f"{cwd}/.gemini/settings.json"),
        ("opencode:user", f"{home}/.config/opencode/opencode.jsonc"),
        ("opencode:user-json", f"{home}/.config/opencode/opencode.json"),
        ("opencode:project", f"{cwd}/.opencode/opencode.json"),
    ]
    by_name: dict[str, dict] = {}

    def add(server: dict | None) -> None:
        if server and server["name"] not in by_name:
            by_name[server["name"]] = server

    for source, path in json_paths:
        if not os.path.exists(path):
            continue
        doc = _load_jsonc(path)
        if not isinstance(doc, dict):
            continue
        mapping = doc.get("mcpServers")
        if not isinstance(mapping, dict):
            if source.startswith("opencode:"):
                mapping = doc.get("mcp")
            else:
                mapping = doc if all(isinstance(v, dict) for v in doc.values()) else None
        if isinstance(mapping, dict):
            for name, raw in mapping.items():
                if isinstance(raw, dict):
                    add(_normalize_server(name, raw, source))

    # Codex TOML subset: [mcp_servers.<name>] command/args
    for path in (f"{home}/.codex/config.toml", f"{cwd}/.codex/config.toml"):
        if not os.path.exists(path):
            continue
        try:
            raw = read_file_bounded(path).decode("utf-8", errors="replace")
        except OSError:
            continue
        current = None
        for line in raw.split("\n"):
            line = line.strip()
            if not line or line.startswith("#"):
                continue
            if line.startswith("[mcp_servers."):
                current = line[len("[mcp_servers."):].rstrip("]").strip().strip('"')
                continue
            if line.startswith("["):
                current = None
                continue
            if not current or "=" not in line:
                continue
            key, _, value = line.partition("=")
            key = key.strip()
            value = value.strip().strip('"')
            existing = by_name.get(current)
            if key == "command":
                rec = existing or {"name": current, "url": "", "enabled": True,
                                   "source": "codex", "args": [], "env": None}
                rec["command"] = value
                by_name[current] = rec
            elif key == "args" and existing is not None:
                inner = value.strip("[]")
                existing["args"] = [p.strip().strip("\"'") for p in inner.split(",") if p.strip()]
    return by_name


# ---------------------------------------------------------------------------
# stdio client
# ---------------------------------------------------------------------------


class Connection:
    def __init__(self, name: str, command: str, args: list[str], env: dict | None, cwd: str) -> None:
        self.name = name
        self.proc = subprocess.Popen(
            [command, *args],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            cwd=cwd or None,
            env={**os.environ, **(env or {})} if env else None,
        )
        self._lock = threading.Lock()
        self._responses: dict[int, Any] = {}
        self._errors: dict[int, str] = {}
        self._next_id = 1
        self.alive = True
        self._reader_done = threading.Event()
        threading.Thread(target=self._reader, daemon=True).start()

    def _reader(self) -> None:
        assert self.proc.stdout is not None
        for raw in self.proc.stdout:
            line = raw.decode("utf-8", errors="replace").strip()
            if not line:
                continue
            try:
                frame = json.loads(line)
            except json.JSONDecodeError:
                continue
            if not isinstance(frame, dict):
                continue
            with self._lock:
                if "method" in frame and "id" in frame:
                    self._handle_server_request(frame)
                    continue
                idv = frame.get("id")
                if not isinstance(idv, int):
                    continue
                if "error" in frame:
                    self._errors[idv] = str((frame.get("error") or {}).get("message", "rpc error"))
                elif "result" in frame:
                    self._responses[idv] = frame["result"]
        self.alive = False
        self._reader_done.set()

    def _handle_server_request(self, frame: dict) -> None:
        response = {"jsonrpc": "2.0", "id": frame["id"]}
        if frame.get("method") == "ping":
            response["result"] = {}
        else:
            response["error"] = {"code": -32601, "message": "method not found"}
        self._send(response)

    def _send(self, obj: dict) -> None:
        if not self.alive or self.proc.stdin is None:
            return
        try:
            self.proc.stdin.write((json.dumps(obj) + "\n").encode())
            self.proc.stdin.flush()
        except (BrokenPipeError, OSError):
            self.alive = False

    def request(self, method: str, params: Any, timeout_ms: int) -> Any:
        with self._lock:
            id_ = self._next_id
            self._next_id += 1
        self._send({"jsonrpc": "2.0", "id": id_, "method": method, "params": params})
        deadline = time.time() + timeout_ms / 1000.0
        while time.time() < deadline:
            with self._lock:
                if id_ in self._errors:
                    raise RuntimeError(f"mcp request failed: {self._errors.pop(id_)}")
                if id_ in self._responses:
                    return self._responses.pop(id_)
                alive = self.alive
            if not alive:
                raise RuntimeError("NotConnected")
            time.sleep(0.005)
        raise TimeoutError(f"mcp request timed out: {method}")

    def notify(self, method: str, params: Any) -> None:
        self._send({"jsonrpc": "2.0", "method": method, "params": params})

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
_CONNECTIONS: dict[str, Connection] = {}


def _resolve_command(command: str) -> str:
    if "/" in command and os.path.exists(command):
        return command
    home = home_dir()
    for cand in (f"{home}/homebrew/bin/{command}", "/opt/homebrew/bin/{command}".format(command=command),
                 f"/usr/local/bin/{command}", f"{home}/.bun/bin/{command}",
                 f"{home}/.local/bin/{command}", f"/usr/bin/{command}"):
        if os.path.exists(cand.format(command=command) if "{command}" in cand else cand):
            return cand.format(command=command) if "{command}" in cand else cand
    return command


def ensure_connected(server: dict) -> Connection:
    with _CONN_LOCK:
        existing = _CONNECTIONS.get(server["name"])
        if existing is not None:
            return existing
        if not server.get("command"):
            raise RuntimeError("NoCommand")
        command = _resolve_command(server["command"])
        conn = Connection(server["name"], command, server.get("args") or [],
                          server.get("env"), os.environ.get("PWD") or os.getcwd())
        try:
            init = conn.request("initialize", {
                "protocolVersion": PROTOCOL_VERSION,
                "capabilities": {},
                "clientInfo": {"name": "forge-ade", "version": "1.0.0"},
            }, 25_000)
            conn.notify("notifications/initialized", None)
            del init
        except Exception:
            conn.close()
            raise
        _CONNECTIONS[server["name"]] = conn  # owned key (dict owns its strings)
        return conn


# ---------------------------------------------------------------------------
# Wire handlers
# ---------------------------------------------------------------------------


@bridge.cmd("services.ListMCPServers", "services.RefreshMCP")
def list_servers(payload: dict) -> list:
    return [
        {"name": s["name"], "command": s["command"], "args": s["args"], "env": s["env"],
         "enabled": s["enabled"], "source": s["source"],
         "connected": s["name"] in _CONNECTIONS, "error": None}
        for s in discover().values()
    ]


@bridge.cmd("services.SaveMCPServer")
def save_server(payload: dict):
    server = payload.get("server") or {}
    name = server.get("name")
    if not name:
        raise RuntimeError("name required")
    path = data_dir() / "mcp.json"
    doc = _load_jsonc(str(path)) or {}
    if not isinstance(doc, dict):
        doc = {}
    servers = doc.get("mcpServers") if isinstance(doc.get("mcpServers"), dict) else {}
    servers[name] = {
        "command": server.get("command") or "",
        "args": server.get("args") or [],
        "env": server.get("env") or {},
        "enabled": server.get("enabled", True),
    }
    doc["mcpServers"] = servers
    from .util import write_json_file

    write_json_file(path, doc)
    return {"ok": True}


@bridge.cmd("services.DeleteMCPServer")
def delete_server(payload: dict):
    name = payload.get("name", "")
    path = data_dir() / "mcp.json"
    doc = _load_jsonc(str(path)) or {}
    if isinstance(doc, dict) and isinstance(doc.get("mcpServers"), dict):
        doc["mcpServers"].pop(name, None)
        from .util import write_json_file

        write_json_file(path, doc)
    with _CONN_LOCK:
        conn = _CONNECTIONS.pop(name, None)
    if conn:
        conn.close()
    return {"ok": True}


@bridge.cmd("services.ListMCPTools", "services.ListConnectedMCPTools")
def list_tools(payload: dict) -> list:
    tools = []
    for server in discover().values():
        if not server["enabled"]:
            continue
        try:
            conn = ensure_connected(server)
        except Exception:
            continue
        try:
            result = conn.request("tools/list", {}, 10_000)
        except Exception:
            continue
        for tool in (result or {}).get("tools") or []:
            name = tool.get("name")
            if not name:
                continue
            tools.append({
                "name": f"mcp_{server['name']}_{name}",
                "description": tool.get("description", ""),
                "server": server["name"],
                "parameters": tool.get("inputSchema"),
            })
    return tools


@bridge.cmd("services.ReconnectMCP")
def reconnect(payload: dict):
    connected, failed = [], []
    with _CONN_LOCK:
        conns = list(_CONNECTIONS.values())
        _CONNECTIONS.clear()
    for conn in conns:
        conn.close()
    for server in discover().values():
        if not server["enabled"]:
            continue
        try:
            ensure_connected(server)
            connected.append(server["name"])
        except Exception:
            failed.append(server["name"])
    return {"connected": connected, "failed": failed}


def call_qualified_tool(qualified: str, args: dict) -> str:
    m = re.match(r"mcp_([^_]+)_(.+)", qualified)
    if not m:
        raise RuntimeError("InvalidMcpToolName")
    server_name, tool_name = m.group(1), m.group(2)
    server = discover().get(server_name)
    if server is None:
        raise RuntimeError("McpServerNotFound")
    conn = ensure_connected(server)
    result = conn.request("tools/call", {"name": tool_name, "arguments": args}, 30_000)
    if result.get("isError"):
        raise RuntimeError("McpToolError")
    parts = []
    for block in result.get("content") or []:
        if isinstance(block, dict) and block.get("type") == "text":
            parts.append(block.get("text", ""))
        elif isinstance(block, str):
            parts.append(block)
    return "\n".join(parts) if parts else json.dumps(result)
