"""forge_ade.llm_client — streaming OpenAI-compatible chat client.

Port of llm-client.zig: streams via `curl -sN` (macOS ships curl; avoids
argv limits by POSTing the body from a temp file), parses SSE `data:` lines
incrementally, and reports content/reasoning/tool-call deltas through
callbacks. All slices are fully-owned Python strings — the use-after-free
class from the Zig port can't exist here.
"""

from __future__ import annotations

import json
import os
import subprocess
import tempfile
import threading
from typing import Any, Callable

from .util import now_ms, write_file_atomic


def _json_escape(s: str) -> str:
    return json.dumps(s, ensure_ascii=False)


def build_body(target: dict, messages: list[dict], tools: list[dict]) -> str:
    parts = [f'"model":{_json_escape(target.get("model", ""))}']
    parts.append('"stream":true')
    parts.append('"stream_options":{"include_usage":true}')
    msgs = []
    for m in messages:
        entry = f'"role":{_json_escape(m.get("role", ""))},"content":{_json_escape(m.get("content", ""))}'
        if m.get("tool_call_id"):
            entry += f',"tool_call_id":{_json_escape(m["tool_call_id"])}'
        if m.get("tool_calls"):
            entry += f',"tool_calls":{m["tool_calls"]}'
        msgs.append("{" + entry + "}")
    parts.append(f'"messages":[{",".join(msgs)}]')
    if tools:
        tdefs = []
        for t in tools:
            entry = f'"type":"function","function":{{"name":{_json_escape(t["name"])}'
            if t.get("description"):
                entry += f',"description":{_json_escape(t["description"])}'
            entry += f',"parameters":{t.get("parameters") or '{"type":"object","properties":{}}'}}}' + "}"
            tdefs.append("{" + entry + "}")
        parts.append(f'"tools":[{",".join(tdefs)}]')
    return "{" + ",".join(parts) + "}"


class ToolAccum:
    __slots__ = ("id", "name", "args")

    def __init__(self) -> None:
        self.id = ""
        self.name = ""
        self.args = ""

    def append(self, id_: str, name: str, args: str) -> None:
        if id_ and not self.id:
            self.id = id_
        if name:
            self.name += name
        if args:
            self.args += args


def stream_chat(
    target: dict,
    messages: list[dict],
    tools: list[dict],
    on_chunk: Callable[[str, str], None] | None = None,
    on_tool_delta: Callable[[int, str, str, str], None] | None = None,
    abort_flag: threading.Event | None = None,
) -> dict:
    """Returns {content, reasoning, toolCalls:[{id,name,arguments}], usage:{...}, stopReason}."""
    base = (target.get("baseURL") or "").rstrip("/")
    url = f"{base}/chat/completions"
    body = build_body(target, messages, tools)

    fd, tmp_path = tempfile.mkstemp(prefix="forge-llm-body-", suffix=".json")
    try:
        os.close(fd)
        write_file_atomic(tmp_path, body)
        proc = subprocess.Popen(
            [
                "curl", "-sN", "-X", "POST",
                "-H", "Content-Type: application/json",
                "-H", f"Authorization: Bearer {target.get('apiKey', '')}",
                "--data-binary", f"@{tmp_path}",
                url,
            ],
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
        )
    except Exception:
        if os.path.exists(tmp_path):
            os.unlink(tmp_path)
        raise

    acc_content: list[str] = []
    acc_reasoning: list[str] = []
    tool_map: dict[int, ToolAccum] = {}
    tool_order: list[int] = []
    usage = {"promptTokens": 0, "completionTokens": 0, "cachedTokens": 0}
    stop_reason = "stop"

    assert proc.stdout is not None
    buf = ""
    while not (abort_flag and abort_flag.is_set()):
        chunk = proc.stdout.readline()
        if not chunk:
            break
        try:
            buf += chunk.decode("utf-8", errors="replace")
        except Exception:
            continue
        while "\n" in buf:
            line, buf = buf.split("\n", 1)
            line = line.strip()
            if not line.startswith("data:"):
                continue
            data = line[len("data:"):].strip()
            if not data or data == "[DONE]":
                continue
            try:
                obj = json.loads(data)
            except json.JSONDecodeError:
                continue
            if isinstance(obj, dict) and isinstance(obj.get("error"), dict):
                msg = obj["error"].get("message", "provider error")
                proc.kill()
                raise RuntimeError(f"provider error: {msg}")
            choices = obj.get("choices") or []
            if choices:
                choice = choices[0]
                fr = choice.get("finish_reason")
                if fr:
                    stop_reason = "tool_use" if fr == "tool_calls" else fr
                delta = choice.get("delta") or {}
                content = delta.get("content")
                if content:
                    acc_content.append(content)
                    if on_chunk:
                        on_chunk(content, "")
                reasoning = delta.get("reasoning_content") or delta.get("reasoning") or ""
                if reasoning:
                    acc_reasoning.append(reasoning)
                    if on_chunk:
                        on_chunk("", reasoning)
                for tc in delta.get("tool_calls") or []:
                    if not isinstance(tc, dict):
                        continue
                    index = int(tc.get("index") or 0)
                    fn = tc.get("function") or {}
                    id_, name, args = tc.get("id") or "", fn.get("name") or "", fn.get("arguments") or ""
                    if index not in tool_map:
                        tool_map[index] = ToolAccum()
                        tool_order.append(index)
                    tool_map[index].append(id_, name, args)
                    if on_tool_delta:
                        on_tool_delta(index, id_, name, args)
            usage_obj = obj.get("usage")
            if isinstance(usage_obj, dict):
                usage["promptTokens"] = int(usage_obj.get("prompt_tokens") or 0)
                usage["completionTokens"] = int(usage_obj.get("completion_tokens") or 0)
                cached = (usage_obj.get("prompt_tokens_details") or {}).get("cached_tokens")
                usage["cachedTokens"] = int(cached or 0)

    proc.wait()
    try:
        os.unlink(tmp_path)
    except OSError:
        pass

    tool_calls = [
        {"id": tool_map[i].id, "name": tool_map[i].name, "arguments": tool_map[i].args or "{}"}
        for i in tool_order
        if tool_map[i].name
    ]
    return {
        "content": "".join(acc_content),
        "reasoning": "".join(acc_reasoning),
        "toolCalls": tool_calls,
        "stopReason": stop_reason,
        "startedAt": now_ms(),
        **usage,
    }
