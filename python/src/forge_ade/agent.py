"""forge_ade.agent — agent sessions + turn orchestration.

Port of agent.zig: JSONL session files under ~/.forge-ade/agent-sessions/,
an LLM turn loop with tool execution, and the agent:* event stream
(agent:turn_start / message_delta / message_end / tool_start / tool_end /
turn_end / updated / error) wrapped on the services.agent channel.
"""

from __future__ import annotations

import json
import os
import re
import threading
import time
import uuid
from pathlib import Path

from . import bridge
from .llm import load_active_target
from .llm_client import stream_chat
from .mcp import call_qualified_tool
from .util import data_dir, now_ms, read_json, run_shell, write_file_atomic, write_json_file

SESSIONS_DIR = data_dir() / "agent-sessions"
ABORTS: dict[str, threading.Event] = {}
ABORTS_LOCK = threading.Lock()

DEFAULT_DEFINITIONS = [
    {"id": "coder", "name": "Full-Stack Engineer", "role_filter": "coding",
     "description": "Builds features, fixes bugs, and runs refactors with tool access.",
     "prompt": "You are an expert full-stack engineer. Write clean, idiomatic code.",
     "rules": "1. Read files before editing.\n2. Verify changes with tests.",
     "model": "claude-3-7-sonnet-20250219"},
    {"id": "planner", "name": "Architect & Planner", "role_filter": "planning",
     "description": "Designs system architectures and breaks down complex phases.",
     "prompt": "You are a software architect. Create crisp, structured plans.",
     "rules": "1. List constraints.\n2. Break down into discrete phases.",
     "model": "claude-3-7-sonnet-20250219"},
    {"id": "researcher", "name": "Research Scout", "role_filter": "research",
     "description": "Investigates APIs, repos, and documentation.",
     "prompt": "You are a research scout. Gather exact facts from sources.",
     "rules": "1. Be evidence-first.\n2. Cite exact files and symbols.",
     "model": "claude-3-5-haiku-20241022"},
]

TOOL_DEFS = [
    {"name": "bash", "description": "Run a shell command in the project directory and return its stdout.",
     "parameters": '{"type":"object","properties":{"command":{"type":"string","description":"The shell command to run"}},"required":["command"]}'},
    {"name": "read_file", "description": "Read a file's contents from the project.",
     "parameters": '{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}'},
    {"name": "write_file", "description": "Write content to a file (creates parent dirs).",
     "parameters": '{"type":"object","properties":{"path":{"type":"string"},"content":{"type":"string"}},"required":["path","content"]}'},
    {"name": "list_dir", "description": "List the files and directories at a path.",
     "parameters": '{"type":"object","properties":{"path":{"type":"string"}},"required":[]}'},
    {"name": "glob", "description": "Find files matching a glob pattern under the project.",
     "parameters": '{"type":"object","properties":{"pattern":{"type":"string"}},"required":["pattern"]}'},
    {"name": "grep", "description": "Search file contents for a query string.",
     "parameters": '{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}'},
]

SYSTEM_PROMPT = (
    "You are ForgeADE, an expert coding agent inside the user's IDE. Work in the "
    "current project. Read files before editing. Verify changes. Be concise but "
    "complete. You have tools: read_file(path), write_file(path, content), "
    "bash(command), glob(pattern), grep(query), list_dir(path)."
)


def _session_path(session_id: str) -> Path:
    safe = re.sub(r"[^A-Za-z0-9_.-]", "_", session_id)
    return SESSIONS_DIR / f"{safe}.jsonl"


def _definitions_path() -> Path:
    return data_dir() / "agent_definitions.json"


# ---------------------------------------------------------------------------
# Session persistence (JSONL)
# ---------------------------------------------------------------------------


def _read_lines(path: Path) -> list[dict]:
    out = []
    try:
        raw = path.read_text(encoding="utf-8", errors="replace")
    except OSError:
        return out
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


def _append_line(path: Path, obj: dict) -> None:
    existing = path.read_bytes() if path.exists() else b""
    line = json.dumps(obj, ensure_ascii=False) + "\n"
    write_file_atomic(path, existing + line.encode("utf-8"))


def _load_session(session_id: str) -> dict | None:
    path = _session_path(session_id)
    lines = _read_lines(path)
    meta = None
    messages = []
    for entry in lines:
        if entry.get("type") == "session":
            meta = entry
        elif entry.get("type") == "message":
            msg = entry.get("message") or {}
            messages.append(msg)
    if meta is None:
        return None
    preview = ""
    if messages:
        blocks = messages[-1].get("content") or []
        if blocks:
            preview = blocks[0].get("text") or ""
    return {
        "id": meta.get("id") or session_id,
        "name": meta.get("name") or "Agent",
        "role": meta.get("role") or "coding",
        "projectFolder": meta.get("projectFolder") or "",
        "acpSessionId": meta.get("acpSessionId"),
        "createdAt": meta.get("createdAt") or 0,
        "updatedAt": meta.get("createdAt") or 0,
        "messageCount": len(messages),
        "lastMessagePreview": preview,
        "state": "idle",
        "contextWindow": 128_000,
        "messages": messages,
    }


def _meta_of(session: dict) -> dict:
    return {k: session[k] for k in (
        "id", "name", "role", "projectFolder", "createdAt", "updatedAt",
        "messageCount", "lastMessagePreview", "state", "contextWindow",
    )}


def _rewrite_header(session_id: str, updates: dict) -> None:
    path = _session_path(session_id)
    lines = _read_lines(path)
    out: list[str] = []
    first = True
    for entry in lines:
        if first and entry.get("type") == "session":
            entry = {**entry, **updates}
        out.append(json.dumps(entry, ensure_ascii=False))
        first = False
    write_file_atomic(path, ("\n".join(out) + "\n").encode("utf-8"))


# ---------------------------------------------------------------------------
# Wire handlers
# ---------------------------------------------------------------------------


@bridge.cmd("services.ListAgentSessions", "services.ListAgentSessionsForFolder")
def list_sessions(payload: dict) -> list:
    SESSIONS_DIR.mkdir(parents=True, exist_ok=True)
    metas = []
    for file in SESSIONS_DIR.glob("*.jsonl"):
        session = _load_session(file.stem)
        if session:
            metas.append(_meta_of(session))
    metas.sort(key=lambda m: m.get("updatedAt") or 0, reverse=True)
    return metas


@bridge.cmd("services.GetAgentSession")
def get_session(payload: dict):
    session = _load_session(payload.get("id", ""))
    if session is None:
        raise RuntimeError("session not found")
    return session


def _make_session(session_id: str, name: str, role: str, project_folder: str) -> dict:
    SESSIONS_DIR.mkdir(parents=True, exist_ok=True)
    now = now_ms() // 1000
    header = {
        "type": "session", "version": 1, "id": session_id, "name": name,
        "role": role, "projectFolder": project_folder, "createdAt": now,
    }
    _append_line(_session_path(session_id), header)
    return {
        "id": session_id, "name": name, "role": role, "projectFolder": project_folder,
        "createdAt": now, "updatedAt": now, "messageCount": 0,
        "lastMessagePreview": "", "state": "idle", "contextWindow": 128_000, "messages": [],
    }


@bridge.cmd("services.CreateAgentSession", "services.CreateAgentSessionFromDefinition")
def create_session(payload: dict):
    def_id = payload.get("defId")
    if def_id:
        definition = next((d for d in _load_definitions() if d["id"] == def_id), None)
        if definition is None:
            raise RuntimeError("definition not found")
        name, role = definition["name"], definition["role_filter"]
    else:
        name, role = payload.get("name") or "Agent", payload.get("role") or "coding"
    project = payload.get("projectFolder") or ""
    session_id = f"agent-{now_ms()}-{uuid.uuid4().hex[:6]}"
    session = _make_session(session_id, name, role, project)
    bridge.emit("session:opened", _meta_of(session))
    bridge.emit_service("agent:updated", {"id": session_id})
    return session


@bridge.cmd("services.UpdateAgentSession", "services.SetAgentAutoApprove", "services.SetAgentDialect", "services.ToggleAgentTask")
def update_session(payload: dict):
    session_id = payload.get("id", "")
    if _load_session(session_id) is None:
        raise RuntimeError("session not found")
    updates: dict = {"updatedAt": now_ms() // 1000}
    for key in ("name", "role", "autoApprove", "dialect", "customPrompt", "customRules"):
        if key in payload and payload[key] != "":
            updates[key] = payload[key]
    _rewrite_header(session_id, updates)
    bridge.emit_service("agent:updated", {"id": session_id})
    session = _load_session(session_id)
    return _meta_of(session) if session else {"ok": True}


@bridge.cmd("services.DeleteAgentSession", "services.ClearAgentSession")
def delete_session(payload: dict):
    path = _session_path(payload.get("id", ""))
    try:
        path.unlink()
    except OSError:
        pass
    bridge.emit("session:closed", {"id": payload.get("id", "")})
    return {"ok": True}


@bridge.cmd("services.ListAgentDefinitions")
def list_definitions(payload: dict) -> list:
    return _load_definitions()


def _load_definitions() -> list[dict]:
    doc = read_json(_definitions_path(), default=None)
    if isinstance(doc, list) and doc:
        return doc
    return list(DEFAULT_DEFINITIONS)


@bridge.cmd("services.SaveAgentDefinition")
def save_definition(payload: dict):
    definition = payload.get("def") or {}
    if not definition.get("id"):
        definition["id"] = f"def-{now_ms()}"
    defs = _load_definitions()
    defs = [d for d in defs if d.get("id") != definition["id"]]
    defs.append(definition)
    write_json_file(_definitions_path(), defs)
    bridge.emit_service("agent:config:changed", {})
    return definition


@bridge.cmd("services.DeleteAgentDefinition", "services.ApplyAgentDefinitionToSession")
def delete_definition(payload: dict):
    def_id = payload.get("id") or payload.get("defId")
    if def_id and payload.get("id") and payload.get("defId"):
        # ApplyAgentDefinitionToSession: apply config to the session in place.
        definition = next((d for d in _load_definitions() if d["id"] == payload.get("defId")), None)
        if definition is None:
            raise RuntimeError("definition not found")
        session_id = payload["id"]
        _rewrite_header(session_id, {
            "name": definition["name"], "role": definition["role_filter"],
            "customPrompt": definition.get("prompt", ""), "customRules": definition.get("rules", ""),
            "updatedAt": now_ms() // 1000,
        })
        bridge.emit_service("agent:updated", {"id": session_id})
        return {"ok": True}
    if def_id:
        defs = [d for d in _load_definitions() if d.get("id") != def_id]
        write_json_file(_definitions_path(), defs)
        bridge.emit_service("agent:config:changed", {})
    return {"ok": True}


@bridge.cmd("services.StopAgentTurn")
def stop_turn(payload: dict):
    session_id = payload.get("id", "")
    with ABORTS_LOCK:
        flag = ABORTS.get(session_id)
    if flag:
        flag.set()
    bridge.emit_service("agent:turn_end", {"id": session_id, "ok": False, "stopped": True})
    return {"ok": True}


@bridge.cmd("services.RespondAgentApproval")
def respond_approval(payload: dict):
    return {"ok": True}


# ---------------------------------------------------------------------------
# Tool execution
# ---------------------------------------------------------------------------


def _execute_tool(name: str, args_json: str, cwd: str) -> str:
    try:
        args = json.loads(args_json) if args_json else {}
    except json.JSONDecodeError:
        args = {}
    if not isinstance(args, dict):
        args = {}
    if name.startswith("mcp_"):
        try:
            return call_qualified_tool(name, args)
        except Exception as exc:  # noqa: BLE001
            return f"mcp call failed: {exc}"
    if name in ("bash", "shell", "run"):
        return run_shell(args.get("command") or args.get("description") or "", cwd)["output"]
    if name in ("read_file", "read"):
        path = os.path.expanduser(args.get("path", ""))
        try:
            return Path(path).read_text(encoding="utf-8", errors="replace")[:200_000]
        except OSError as exc:
            return f"error: {exc}"
    if name in ("write_file", "write"):
        path = os.path.expanduser(args.get("path", ""))
        content = args.get("content") or ""
        try:
            p = Path(path)
            p.parent.mkdir(parents=True, exist_ok=True)
            p.write_text(content, encoding="utf-8")
            return f"wrote {len(content)} bytes to {path}"
        except OSError as exc:
            return f"error: {exc}"
    if name in ("glob", "find"):
        pattern = args.get("pattern", "")
        return run_shell(f"find . -path '*/{pattern}' 2>/dev/null | head -100", cwd)["output"]
    if name in ("grep", "search"):
        query = args.get("query", "")
        return run_shell(f"grep -rn --include='*' -m 5 {shlex_quote(query)} . 2>/dev/null | head -50", cwd)["output"]
    if name == "list_dir":
        path = args.get("path") or "."
        return run_shell(f"ls -la {shlex_quote(path)}", cwd)["output"]
    return "error: unknown tool"


def shlex_quote(value: str) -> str:
    import shlex

    return shlex.quote(value)


def _extract_text_tool_calls(content: str) -> list[dict]:
    calls = []
    pos = 0
    while True:
        start = content.find("<tool_call>", pos)
        if start == -1:
            break
        body_start = start + len("<tool_call>")
        end = content.find("</tool_call>", body_start)
        if end == -1:
            break
        body = content[body_start:end]
        pos = end + len("</tool_call>")
        try:
            obj = json.loads(body)
        except json.JSONDecodeError:
            continue
        name = obj.get("name")
        if not name:
            continue
        args_value = obj.get("arguments")
        if isinstance(args_value, dict):
            args_json = json.dumps(args_value)
        elif isinstance(args_value, str) and args_value:
            args_json = args_value
        else:
            args_json = "{}"
        calls.append({
            "id": f"call-{now_ms()}-{len(calls)}",
            "name": name,
            "arguments": args_json,
        })
    return calls


# ---------------------------------------------------------------------------
# The turn loop
# ---------------------------------------------------------------------------


def _session_to_messages(path: Path, tail: int = 30) -> list[dict]:
    messages = []
    for entry in _read_lines(path):
        if entry.get("type") != "message":
            continue
        msg = entry.get("message") or {}
        role = msg.get("role", "")
        blocks = msg.get("content") or []
        if role == "tool":
            text = blocks[0].get("text", "") if blocks else ""
            messages.append({"role": "tool", "content": text,
                             "tool_call_id": msg.get("tool_call_id") or (blocks[0].get("tool_call_id") if blocks else "")})
            continue
        text = blocks[0].get("text", "") if blocks else ""
        message: dict = {"role": role, "content": text}
        tool_calls = msg.get("tool_calls")
        if not tool_calls and role == "assistant" and blocks:
            tool_calls = _build_tool_calls_json(blocks)
        if tool_calls:
            message["tool_calls"] = tool_calls
        messages.append(message)
    if tail > 0 and len(messages) > tail:
        messages = messages[-tail:]
    return messages


def _build_tool_calls_json(blocks: list) -> str | None:
    calls = []
    for block in blocks:
        if block.get("type") != "tool_call":
            continue
        args = block.get("arguments") or "{}"
        try:
            json.loads(args)
        except (json.JSONDecodeError, TypeError):
            args = json.dumps(args)
        calls.append(
            '{"id":%s,"type":"function","function":{"name":%s,"arguments":%s}}'
            % (json.dumps(block.get("tool_call_id") or ""),
               json.dumps(block.get("name") or ""),
               args if isinstance(args, str) else json.dumps(args))
        )
    return "[" + ",".join(calls) + "]" if calls else None


def _persist_message(path: Path, obj: dict) -> None:
    _append_line(path, {"type": "message", **obj})


@bridge.cmd("services.SendAgentMessage")
def send_message(payload: dict):
    session_id = payload.get("id", "")
    text = payload.get("message") or payload.get("content") or ""
    if not session_id or not text:
        raise RuntimeError("id and message required")

    session = _load_session(session_id)
    if session is None:
        raise RuntimeError("session not found")
    role = session["role"]

    path = _session_path(session_id)
    msg_id = f"msg-{now_ms()}"
    ts = str(now_ms())
    _persist_message(path, {
        "id": msg_id, "timestamp": ts,
        "message": {"id": msg_id, "role": "user",
                    "content": [{"type": "text", "text": text}], "timestamp": ts},
    })
    bridge.emit_service("agent:updated", {"id": session_id})
    bridge.emit_service("agent:turn_start", {"id": session_id})

    if role.startswith("external:"):
        from .external import send_external_message

        try:
            send_external_message(session_id, text)
        except Exception as exc:  # noqa: BLE001
            bridge.emit_service("agent:error", {"id": session_id, "message": f"external agent failed: {exc}"})
            bridge.emit_service("agent:turn_end", {"id": session_id, "ok": False})
        return {"ok": True}

    target = load_active_target()
    if target is None:
        bridge.emit_service("agent:error", {"id": session_id, "message": "no active LLM provider configured — add a provider profile in Settings"})
        bridge.emit_service("agent:turn_end", {"id": session_id, "ok": False})
        return {"ok": True}

    a_msg_id = f"msg-{now_ms()}-a"
    bridge.emit_service("agent:message_start", {"id": session_id, "messageId": a_msg_id})

    def on_chunk(delta_content: str, delta_reasoning: str) -> None:
        if delta_content:
            bridge.emit_service("agent:message_delta",
                                {"id": session_id, "kind": "text", "delta": delta_content})
        if delta_reasoning:
            bridge.emit_service("agent:message_delta",
                                {"id": session_id, "kind": "thinking", "delta": delta_reasoning})

    def on_tool(index: int, t_id: str, t_name: str, t_args: str) -> None:
        bridge.emit_service("agent:tool_start",
                            {"id": session_id, "index": index, "name": t_name, "toolCallId": t_id})
        if t_args:
            bridge.emit_service("agent:tool_delta",
                                {"id": session_id, "index": index, "args": t_args})

    messages = [{"role": "system", "content": SYSTEM_PROMPT}]
    messages += _session_to_messages(path)
    messages.append({"role": "user", "content": text})

    abort = threading.Event()
    with ABORTS_LOCK:
        ABORTS[session_id] = abort
    try:
        try:
            result = stream_chat(target, messages, TOOL_DEFS, on_chunk=on_chunk,
                                 on_tool_delta=on_tool, abort_flag=abort)
        except Exception as exc:  # noqa: BLE001
            bridge.emit_service("agent:error", {"id": session_id, "message": f"LLM call failed: {exc}"})
            bridge.emit_service("agent:message_end",
                                {"id": session_id, "messageId": a_msg_id,
                                 "message": {"id": a_msg_id, "role": "assistant", "content": [],
                                             "timestamp": ts, "state": "done"}})
            bridge.emit_service("agent:turn_end", {"id": session_id, "ok": False})
            return {"ok": True}

        content = result["content"]
        tool_calls = list(result["toolCalls"])
        blocks: list[dict] = []
        if content:
            blocks.append({"type": "text", "text": content})
        for tc in tool_calls:
            blocks.append({"type": "tool_call", "tool_call_id": tc["id"],
                           "name": tc["name"], "arguments": tc["arguments"]})
        _persist_message(path, {
            "id": a_msg_id, "timestamp": ts,
            "message": {"id": a_msg_id, "role": "assistant", "timestamp": ts,
                        "content": blocks, "tool_calls": _build_tool_calls_json(blocks),
                        "state": "done"},
        })
        bridge.emit_service("agent:message_end", {
            "id": session_id, "messageId": a_msg_id,
            "message": {"id": a_msg_id, "role": "assistant",
                        "content": [{"type": "text", "text": content}] if content else [],
                        "timestamp": ts, "state": "done"},
        })

        tool_calls += _extract_text_tool_calls(content)
        cwd = os.environ.get("PWD") or os.getcwd()
        for index, tc in enumerate(tool_calls):
            if abort.is_set():
                break
            bridge.emit_service("agent:tool_start",
                                {"id": session_id, "index": index, "name": tc["name"],
                                 "toolCallId": tc["id"]})
            output = _execute_tool(tc["name"], tc["arguments"], cwd)
            tool_msg_id = f"msg-{now_ms()}-r"
            _persist_message(path, {
                "id": tool_msg_id, "timestamp": str(now_ms()),
                "message": {"id": tool_msg_id, "role": "tool", "timestamp": str(now_ms()),
                            "content": [{"type": "tool_result", "tool_call_id": tc["id"],
                                         "name": tc["name"], "text": output, "is_error": False}],
                            "tool_call_id": tc["id"], "state": "done"},
            })
            bridge.emit_service("agent:tool_end", {
                "id": session_id, "index": index, "toolCallId": tc["id"],
                "name": tc["name"], "result": output, "isError": False,
            })

        bridge.emit_service("agent:turn_end", {
            "id": session_id, "ok": True,
            "usage": {"at": now_ms(), "promptTokens": result["promptTokens"],
                      "completionTokens": result["completionTokens"],
                      "cachedTokens": result["cachedTokens"], "durationMs": 0},
            "contextWindow": 128_000,
        })
        return {"ok": True}
    finally:
        with ABORTS_LOCK:
            ABORTS.pop(session_id, None)


@bridge.cmd("services.RespondAgentAsk")
def respond_agent_ask(payload: dict):
    return {"ok": True}
