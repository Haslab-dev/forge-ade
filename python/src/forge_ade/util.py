"""forge_ade.util — shared filesystem/shell helpers."""

from __future__ import annotations

import json
import os
import subprocess
import tempfile
import time
from pathlib import Path
from typing import Any

HOME = Path.home()
DATA_DIR = HOME / ".forge-ade"


def data_dir() -> Path:
    DATA_DIR.mkdir(parents=True, exist_ok=True)
    return DATA_DIR


def now_ms() -> int:
    return int(time.time() * 1000)


def now_sec() -> int:
    return int(time.time())


def read_file_bounded(path: str | os.PathLike, limit: int = 25 * 1024 * 1024) -> bytes:
    with open(path, "rb") as f:
        data = f.read(limit)
    return data


def read_json(path: str | os.PathLike, default: Any = None) -> Any:
    try:
        return json.loads(read_file_bounded(path))
    except Exception:
        return default


def write_file_atomic(path: str | os.PathLike, data: bytes | str) -> None:
    p = Path(path)
    p.parent.mkdir(parents=True, exist_ok=True)
    if isinstance(data, str):
        data = data.encode("utf-8")
    fd, tmp = tempfile.mkstemp(dir=str(p.parent), prefix=p.name + ".", suffix=".tmp")
    try:
        with os.fdopen(fd, "wb") as f:
            f.write(data)
        os.replace(tmp, p)
    except Exception:
        try:
            os.unlink(tmp)
        except OSError:
            pass
        raise


def write_json_file(path: str | os.PathLike, value: Any) -> None:
    write_file_atomic(path, json.dumps(value, indent=2, ensure_ascii=False))


def run_shell(command: str, cwd: str = "", timeout: float = 120.0) -> dict:
    """Run a shell command, merging stdout+stderr (the Zig bridge did the
    same via dup2). Returns {output, exitCode, success} — the exact shape
    the frontend's execCommand consumes."""
    try:
        proc = subprocess.run(
            ["/bin/sh", "-c", command],
            cwd=cwd or None,
            capture_output=True,
            timeout=timeout,
        )
        output = (proc.stdout + proc.stderr).decode("utf-8", errors="replace")
        code = proc.returncode
    except subprocess.TimeoutExpired as exc:
        output = (exc.stdout or b"") + (exc.stderr or b"")
        return {
            "output": output.decode("utf-8", errors="replace") + f"\n[timeout after {timeout}s]",
            "exitCode": 124,
            "success": False,
        }
    except Exception as exc:  # noqa: BLE001
        return {"output": str(exc), "exitCode": -1, "success": False}
    return {"output": output, "exitCode": code, "success": code == 0}


def home_dir() -> str:
    return str(HOME)


def workspace_root() -> str:
    return os.environ.get("PWD") or os.getcwd()
