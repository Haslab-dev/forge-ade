"""forge_ade.fs_cmds — fs.* bridge, command.exec, dialogs, os/clipboard, watcher."""

from __future__ import annotations

import base64
import os
import subprocess
import threading
import time
from pathlib import Path

from . import bridge
from .util import run_shell

SKIP_SCAN_DIRS = {".git", "node_modules", "zig-out", "dist", ".native", ".DS_Store"}

# ---------------------------------------------------------------------------
# fs.* commands
# ---------------------------------------------------------------------------


@bridge.cmd("fs.getCwd")
def fs_getcwd(payload: dict):
    return {"cwd": os.getcwd()}


@bridge.cmd("fs.readDir")
def fs_readdir(payload: dict):
    path = payload.get("path") or os.getcwd()
    entries = []
    try:
        with os.scandir(path) as it:
            for entry in it:
                if entry.name in (".", "..", ".git", "node_modules"):
                    continue
                entries.append(
                    {"name": entry.name, "path": os.path.join(path, entry.name), "isDir": entry.is_dir()}
                )
    except OSError as exc:
        raise RuntimeError(f"Cannot open directory: {exc}") from exc
    entries.sort(key=lambda e: (not e["isDir"], e["name"].lower()))
    return {"path": path, "entries": entries}


@bridge.cmd("fs.readFile")
def fs_readfile(payload: dict):
    path = payload.get("path", "")
    try:
        data = Path(path).read_bytes()
    except OSError as exc:
        raise RuntimeError("File not found or cannot open") from exc
    ext = os.path.splitext(path)[1].lower()
    is_image = ext in {".png", ".jpg", ".jpeg", ".gif", ".webp", ".ico", ".bmp"}
    try:
        text = data.decode("utf-8")
        is_utf8 = True
    except UnicodeDecodeError:
        text, is_utf8 = "", False
    if is_image or not is_utf8:
        return {
            "path": path,
            "content": text,
            "base64": base64.b64encode(data).decode(),
            "isBinary": True,
            "size": len(data),
        }
    return {"path": path, "content": text, "isBinary": False, "size": len(data)}


@bridge.cmd("fs.writeFile")
def fs_writefile(payload: dict):
    path = payload.get("path", "")
    if payload.get("base64"):
        data = base64.b64decode(payload["base64"])
    else:
        data = (payload.get("content") or "").encode("utf-8")
    p = Path(path)
    p.parent.mkdir(parents=True, exist_ok=True)
    p.write_bytes(data)
    return {"ok": True}


@bridge.cmd("fs.createFile")
def fs_createfile(payload: dict):
    path = payload.get("path", "")
    if not path:
        raise RuntimeError("path required")
    p = Path(path)
    p.parent.mkdir(parents=True, exist_ok=True)
    p.touch(exist_ok=True)
    return {"ok": True}


@bridge.cmd("fs.createDir")
def fs_createdir(payload: dict):
    path = payload.get("path", "")
    if not path:
        raise RuntimeError("path required")
    Path(path).mkdir(parents=True, exist_ok=True)
    return {"ok": True}


@bridge.cmd("fs.watch")
def fs_watch(payload: dict):
    _WATCHER.add_path(payload.get("path", ""))
    return {"ok": True}


@bridge.cmd("fs.unwatch")
def fs_unwatch(payload: dict):
    _WATCHER.remove_path(payload.get("path", ""))
    return {"ok": True}


# ---------------------------------------------------------------------------
# Filesystem watcher (poll-scan parity with the Zig fsWatcherThread)
# ---------------------------------------------------------------------------


class _FsWatcher:
    def __init__(self) -> None:
        self._roots: set[str] = set()
        self._known: dict[str, tuple[int, int]] = {}  # path -> (mtime_ns, size)
        self._scanned: set[str] = set()
        self._lock = threading.Lock()
        self._thread: threading.Thread | None = None

    def add_path(self, path: str) -> None:
        if not path:
            return
        with self._lock:
            self._roots.add(path)
            if self._thread is None:
                self._thread = threading.Thread(target=self._loop, daemon=True)
                self._thread.start()

    def remove_path(self, path: str) -> None:
        with self._lock:
            self._roots.discard(path)

    def _loop(self) -> None:
        while True:
            time.sleep(0.4)
            with self._lock:
                roots = list(self._roots)
            if not roots:
                continue
            current: dict[str, tuple[int, int]] = {}
            for root in roots:
                self._scan(root, current, depth=0)
            with self._lock:
                if not self._scanned:
                    self._known = current
                    self._scanned.update(roots)
                    continue
                for path, info in current.items():
                    old = self._known.get(path)
                    if old is None:
                        bridge.emit("fs.change", {"path": path, "kind": "create"})
                    elif old != info:
                        bridge.emit("fs.change", {"path": path, "kind": "modify"})
                for path in self._known:
                    if path not in current:
                        bridge.emit("fs.change", {"path": path, "kind": "delete"})
                self._known = current

    def _scan(self, dir_path: str, out: dict, depth: int) -> None:
        if depth > 5:
            return
        try:
            with os.scandir(dir_path) as it:
                for entry in it:
                    if entry.name in SKIP_SCAN_DIRS:
                        continue
                    full = os.path.join(dir_path, entry.name)
                    try:
                        if entry.is_dir(follow_symlinks=False):
                            self._scan(full, out, depth + 1)
                        else:
                            st = entry.stat(follow_symlinks=False)
                            out[full] = (st.st_mtime_ns, st.st_size)
                    except OSError:
                        continue
        except OSError:
            return


_WATCHER = _FsWatcher()

# ---------------------------------------------------------------------------
# command.exec — shell passthrough (git engine, file ops, agent tooling)
# ---------------------------------------------------------------------------


@bridge.cmd("command.exec")
def command_exec(payload: dict):
    return run_shell(payload.get("command", ""), payload.get("cwd", ""))


# ---------------------------------------------------------------------------
# Dialogs / clipboard / OS open+reveal
# ---------------------------------------------------------------------------


def _dialog_builder(**kwargs):
    """FileDialogBuilder must come from DialogExt.file(app_handle) — the
    builder has no public constructor (metdesk-v2 pattern)."""
    from pytauri_plugins.dialog import DialogExt

    handle = bridge.app_handle()
    if handle is None:
        raise RuntimeError("app not running")
    return DialogExt.file(handle, **kwargs)


def _file_path_str(fp) -> str:
    """Normalizes a pytauri FilePath (pydantic RootModel union) to a path str."""
    if fp is None:
        return ""
    root = getattr(fp, "root", None)
    if isinstance(root, str):
        return root
    if isinstance(root, dict):
        inner = root.get("Path") or root.get("path")
        if isinstance(inner, str):
            return inner
    s = str(fp)
    if s.startswith("/") or s.startswith("~"):
        return s
    try:
        import json as _json

        parsed = _json.loads(fp.model_dump_json())
        if isinstance(parsed, str):
            return parsed
        inner = parsed.get("Path") or parsed.get("path")
        if isinstance(inner, str):
            return inner
    except Exception:
        pass
    return s


@bridge.cmd("native-sdk.dialog.openFile")
def dialog_open(payload: dict):
    kwargs = {}
    if payload.get("title"):
        kwargs["set_title"] = payload["title"]
    builder = _dialog_builder(**kwargs)
    if payload.get("allowDirectories"):
        result = builder.blocking_pick_folder()
    else:
        result = builder.blocking_pick_file()
    return _file_path_str(result)


@bridge.cmd("native-sdk.dialog.saveFile")
def dialog_save(payload: dict):
    kwargs = {}
    if payload.get("title"):
        kwargs["set_title"] = payload["title"]
    if payload.get("defaultName"):
        kwargs["set_file_name"] = payload["defaultName"]
    result = _dialog_builder(**kwargs).blocking_save_file()
    return _file_path_str(result)


@bridge.cmd("native-sdk.clipboard.readText")
def clipboard_read(payload: dict):
    try:
        return subprocess.run(
            ["pbpaste"], capture_output=True, timeout=5, check=False
        ).stdout.decode("utf-8", errors="replace")
    except Exception:
        return ""


@bridge.cmd("native-sdk.os.openUrl")
def os_open_url(payload: dict):
    url = payload.get("url", "")
    if url:
        subprocess.Popen(["open", url])
    return {"ok": True}


@bridge.cmd("native-sdk.os.revealPath")
def os_reveal(payload: dict):
    path = payload.get("path", "")
    if path:
        subprocess.Popen(["open", "-R", path])
    return {"ok": True}
