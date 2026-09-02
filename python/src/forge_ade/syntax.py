"""forge_ade.syntax — syntax check + format.

JSON is validated/reformatted natively. JS/TS/JSX goes through esbuild when
the binary can be found (probing PATH plus the common install locations,
since a bundled app has a minimal PATH). No esbuild → graceful empty result
(the frontend has its own JSON fallback).
"""

from __future__ import annotations

import json
import os
import shutil
import subprocess
from pathlib import Path

from . import bridge
from .util import home_dir


def _find_esbuild() -> str | None:
    found = shutil.which("esbuild")
    if found:
        return found
    home = home_dir()
    for base in (f"{home}/.bun/bin", "/opt/homebrew/bin", "/usr/local/bin",
                 f"{home}/.local/bin", f"{home}/node_modules/.bin"):
        cand = os.path.join(base, "esbuild")
        if os.path.exists(cand):
            return cand
    return None


_LOADER_BY_EXT = {"ts": "ts", "tsx": "tsx", "js": "js", "jsx": "jsx", ".mts": "ts", "mjs": "js", "cts": "ts", "cjs": "js"}


@bridge.cmd("services.CheckSyntax")
def check_syntax(payload: dict) -> list[dict]:
    path = payload.get("path", "")
    content = payload.get("content", "")
    ext = os.path.splitext(path)[1].lstrip(".").lower()

    if ext == "json":
        try:
            json.loads(content or "null")
            return []
        except json.JSONDecodeError as exc:
            line = content.count("\n", 0, exc.pos) + 1 if exc.pos else 1
            column = (exc.pos - content.rfind("\n", 0, exc.pos)) if exc.pos else 1
            return [{"line": line, "column": column, "message": exc.msg, "severity": "error"}]

    loader = _LOADER_BY_EXT.get(ext)
    esbuild = _find_esbuild()
    if loader and esbuild:
        try:
            proc = subprocess.run(
                [esbuild, f"--loader={loader}"], input=content.encode(),
                capture_output=True, timeout=10,
            )
            if proc.returncode != 0:
                stderr = proc.stderr.decode("utf-8", errors="replace")
                message = stderr.strip().split("\n")[0] if stderr else "syntax error"
                line_match = None
                for part in stderr.split():
                    if part.isdigit():
                        line_match = int(part)
                        break
                return [{"line": line_match or 1, "column": 1,
                         "message": message[:500], "severity": "error"}]
            return []
        except (OSError, subprocess.TimeoutExpired):
            return []
    return []


@bridge.cmd("services.FormatCode")
def format_code(payload: dict) -> str:
    path = payload.get("path", "")
    content = payload.get("content", "")
    tab_width = int(payload.get("tabWidth") or 2)
    ext = os.path.splitext(path)[1].lstrip(".").lower()

    if ext == "json":
        try:
            return json.dumps(json.loads(content or "null"), indent=tab_width, ensure_ascii=False)
        except json.JSONDecodeError:
            return content

    loader = _LOADER_BY_EXT.get(ext)
    esbuild = _find_esbuild()
    if loader and esbuild:
        try:
            proc = subprocess.run(
                [esbuild, f"--loader={loader}"], input=content.encode(),
                capture_output=True, timeout=10,
            )
            if proc.returncode == 0:
                return proc.stdout.decode("utf-8", errors="replace")
        except (OSError, subprocess.TimeoutExpired):
            pass
    return content
