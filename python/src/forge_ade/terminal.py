"""forge_ade.terminal — POSIX PTY sessions over the bridge.

Port of the Native SDK's terminal handlers: forkpty per session, a reader
thread per PTY streaming base64 chunks as "terminal.data" events, resize via
TIOCSWINSZ, kill via SIGKILL, and "terminal.exit" on child EOF.
"""

from __future__ import annotations

import base64
import fcntl
import json
import os
import pty
import shlex
import signal
import struct
import termios
import threading
import tty  # noqa: F401  (ensures termios availability on all platforms)

from . import bridge

_LOCK = threading.Lock()
_SESSIONS: dict[int, dict] = {}
_NEXT_ID = [1]


def _reader(session: dict) -> None:
    fd = session["master_fd"]
    buf = bytearray()
    while True:
        try:
            chunk = os.read(fd, 65536)
        except OSError:
            break
        if not chunk:
            break
        buf.clear()
        buf += chunk
        payload = json.dumps(
            {"sessionId": session["id"], "data": base64.b64encode(bytes(buf)).decode()}
        )
        bridge.emit("terminal.data", payload)
    # Child exited — reap and notify.
    with _LOCK:
        if _SESSIONS.get(session["id"]) is session:
            _SESSIONS.pop(session["id"], None)
    try:
        os.waitpid(session["pid"], os.WNOHANG)
    except ChildProcessError:
        pass
    try:
        os.close(fd)
    except OSError:
        pass
    bridge.emit("terminal.exit", json.dumps({"sessionId": session["id"]}))


@bridge.cmd("terminal.spawn")
def terminal_spawn(payload: dict):
    cwd = payload.get("cwd") or None
    cols = int(payload.get("cols") or 80)
    rows = int(payload.get("rows") or 24)
    shell = os.environ.get("SHELL") or "/bin/zsh"

    pid, master_fd = pty.fork()
    if pid == 0:  # child
        os.environ["TERM"] = "xterm-256color"
        os.environ["COLORTERM"] = "truecolor"
        os.environ["LANG"] = "en_US.UTF-8"
        os.environ["LC_ALL"] = "en_US.UTF-8"
        try:
            if cwd:
                os.chdir(cwd)
        except OSError:
            pass
        try:
            os.execvp(shell, [os.path.basename(shell), "-l"])
        except Exception:
            os._exit(127)

    # Parent: size the PTY.
    try:
        fcntl.ioctl(master_fd, termios.TIOCSWINSZ, struct.pack("HHHH", rows, cols, 0, 0))
    except OSError:
        pass

    with _LOCK:
        session_id = _NEXT_ID[0]
        _NEXT_ID[0] += 1
        session = {"id": session_id, "master_fd": master_fd, "pid": pid}
        _SESSIONS[session_id] = session

    threading.Thread(target=_reader, args=(session,), daemon=True).start()
    return {"sessionId": session_id, "pid": pid}


def _get(session_id) -> dict | None:
    try:
        sid = int(session_id)
    except (TypeError, ValueError):
        return None
    with _LOCK:
        return _SESSIONS.get(sid)


@bridge.cmd("terminal.write")
def terminal_write(payload: dict):
    session = _get(payload.get("sessionId"))
    if session is None:
        raise RuntimeError("Session not found")
    data = base64.b64decode(payload.get("data") or "")
    try:
        os.write(session["master_fd"], data)
    except OSError as exc:
        raise RuntimeError("write failed") from exc
    return {"ok": True}


@bridge.cmd("terminal.resize")
def terminal_resize(payload: dict):
    session = _get(payload.get("sessionId"))
    if session is None:
        return {"ok": True}
    cols = max(int(payload.get("cols") or 1), 1)
    rows = max(int(payload.get("rows") or 1), 1)
    try:
        fcntl.ioctl(session["master_fd"], termios.TIOCSWINSZ, struct.pack("HHHH", rows, cols, 0, 0))
    except OSError:
        pass
    return {"ok": True}


@bridge.cmd("terminal.kill")
def terminal_kill(payload: dict):
    session = _get(payload.get("sessionId"))
    if session is None:
        return {"ok": True}
    with _LOCK:
        _SESSIONS.pop(session["id"], None)
    try:
        os.kill(session["pid"], signal.SIGKILL)
    except ProcessLookupError:
        pass
    try:
        os.waitpid(session["pid"], os.WNOHANG)
    except ChildProcessError:
        pass
    try:
        os.close(session["master_fd"])
    except OSError:
        pass
    return {"ok": True}


@bridge.cmd("terminal.list")
def terminal_list(payload: dict):
    with _LOCK:
        sessions = [{"sessionId": s["id"], "pid": s["pid"]} for s in _SESSIONS.values()]
    return {"sessions": sessions}
