"""forge_ade.lsp — LSP client + per-language manager.

Port of lsp.zig + lsp-manager.zig. Concurrency from the crash audit is baked
in: ONE manager lock guards the clients map AND client use (the Zig version's
unused mutex let three concurrent LSPDidOpen calls race the map and
double-spawn servers), and responses are consumed (pop) not aliased.
"""

from __future__ import annotations

import json
import os
import re
import subprocess
import threading
import time
from pathlib import Path

from . import bridge
from .util import home_dir

LANG_BY_EXT = {
    ".ts": "typescript", ".tsx": "typescriptreact", ".js": "javascript",
    ".jsx": "javascriptreact", ".mts": "typescript", ".cts": "typescript",
    ".mjs": "javascript", ".cjs": "javascript", ".go": "go", ".py": "python",
    ".rs": "rust", ".zig": "zig", ".c": "c", ".h": "c", ".cpp": "cpp",
    ".cc": "cpp", ".hpp": "cpp", ".swift": "swift", ".html": "html", ".htm": "html",
    ".css": "css", ".scss": "css", ".less": "css", ".json": "json",
    ".java": "java", ".kt": "kotlin", ".kts": "kotlin",
}

SERVER_CANDIDATES = {
    "typescript": [("typescript-language-server", ["--stdio"]), ("vtsls", ["--stdio"]),
                   ("bunx", ["-y", "typescript-language-server", "--stdio"]),
                   ("npx", ["-y", "typescript-language-server", "--stdio"])],
    "javascript": [("typescript-language-server", ["--stdio"]), ("vtsls", ["--stdio"]),
                   ("bunx", ["-y", "typescript-language-server", "--stdio"]),
                   ("npx", ["-y", "typescript-language-server", "--stdio"])],
    "typescriptreact": [("typescript-language-server", ["--stdio"]), ("vtsls", ["--stdio"]),
                        ("bunx", ["-y", "typescript-language-server", "--stdio"]),
                        ("npx", ["-y", "typescript-language-server", "--stdio"])],
    "javascriptreact": [("typescript-language-server", ["--stdio"]), ("vtsls", ["--stdio"]),
                        ("bunx", ["-y", "typescript-language-server", "--stdio"]),
                        ("npx", ["-y", "typescript-language-server", "--stdio"])],
    "go": [("gopls", [])],
    "python": [("pyright-langserver", ["--stdio"]), ("pylsp", []),
               ("basedpyright-langserver", ["--stdio"]), ("npx", ["-y", "pyright", "--stdio"])],
    "rust": [("rust-analyzer", [])],
    "zig": [("zls", [])],
    "c": [("clangd", ["--background-index", "--clang-tidy"])],
    "cpp": [("clangd", ["--background-index", "--clang-tidy"])],
    "swift": [("sourcekit-lsp", []), ("xcrun", ["sourcekit-lsp"])],
    "html": [("vscode-html-language-server", ["--stdio"])],
    "css": [("vscode-css-language-server", ["--stdio"])],
    "json": [("vscode-json-language-server", ["--stdio"])],
}


def language_id_from_path(path: str) -> str:
    return LANG_BY_EXT.get(os.path.splitext(path)[1].lower(), "")


def _file_uri(path: str) -> str:
    if path.startswith("file://"):
        return path
    return "file://" + path.replace(" ", "%20")


def _resolve_command(command: str) -> str:
    if "/" in command and os.path.exists(command):
        return command
    home = home_dir()
    for base in ("/opt/homebrew/bin", "/usr/local/bin", f"{home}/.bun/bin",
                 f"{home}/.local/bin", f"{home}/go/bin", f"{home}/.cargo/bin",
                 f"{home}/homebrew/bin", "/usr/bin"):
        cand = f"{base}/{command}"
        if os.path.exists(cand):
            return cand
    return command


class Client:
    def __init__(self, language_id: str, workspace_root: str, command: str, args: list[str]) -> None:
        self.language_id = language_id
        self.workspace_root = workspace_root
        self.command = command
        self.status = "starting"
        self.proc = subprocess.Popen(
            [command, *args], stdin=subprocess.PIPE, stdout=subprocess.PIPE,
            cwd=workspace_root or None,
        )
        self._lock = threading.Lock()
        self._responses: dict[int, dict] = {}
        self._errors: dict[int, str] = {}
        self._diagnostics: dict[str, str] = {}
        self._open_docs: set[str] = set()
        self._next_id = 1
        self.alive = True
        self.initialized = False
        self._reader_done = threading.Event()
        threading.Thread(target=self._reader, daemon=True).start()

    # -- wire ---------------------------------------------------------------

    def _reader(self) -> None:
        assert self.proc.stdout is not None
        recv = b""
        while True:
            try:
                chunk = self.proc.stdout.read(65536)
            except OSError:
                break
            if not chunk:
                break
            recv += chunk
            while True:
                header_end = recv.find(b"\r\n\r\n")
                if header_end == -1:
                    break
                header = recv[:header_end].decode("utf-8", errors="replace")
                match = re.search(r"Content-Length:\s*(\d+)", header)
                if not match:
                    recv = recv[header_end + 4:]
                    continue
                total = header_end + 4 + int(match.group(1))
                if len(recv) < total:
                    break
                body = recv[header_end + 4:total]
                recv = recv[total:]
                try:
                    self._handle_frame(json.loads(body.decode("utf-8", errors="replace")))
                except json.JSONDecodeError:
                    continue
        self.alive = False
        self._reader_done.set()

    def _handle_frame(self, frame: dict) -> None:
        if "id" in frame:
            with self._lock:
                idv = frame["id"]
                if "error" in frame:
                    self._errors[idv] = str((frame.get("error") or {}).get("message", "lsp error"))
                elif "result" in frame:
                    self._responses[idv] = frame["result"]
            return
        method = frame.get("method", "")
        if method == "textDocument/publishDiagnostics":
            params = frame.get("params") or {}
            uri = params.get("uri", "")
            if uri:
                path = uri[len("file://"):] if uri.startswith("file://") else uri
                diags_json = json.dumps(params.get("diagnostics", []))
                with self._lock:
                    self._diagnostics[path] = diags_json
                self._emit_diagnostics(path)

    def _emit_diagnostics(self, path: str) -> None:
        with self._lock:
            diags_json = self._diagnostics.get(path, "[]")
        try:
            diags = json.loads(diags_json)
        except json.JSONDecodeError:
            diags = []
        errors = sum(1 for d in diags if d.get("severity") == 1)
        warnings = sum(1 for d in diags if d.get("severity") == 2)
        bridge.emit("services.agent", {
            "event": "lsp:diagnostics",
            "payload": {"filePath": path, "errors": errors,
                        "warnings": warnings, "diagnostics": diags},
        })

    def _send(self, obj: dict) -> None:
        if not self.alive or self.proc.stdin is None:
            raise RuntimeError("NotConnected")
        body = json.dumps(obj).encode()
        try:
            self.proc.stdin.write(f"Content-Length: {len(body)}\r\n\r\n".encode() + body)
            self.proc.stdin.flush()
        except (BrokenPipeError, OSError) as exc:
            raise RuntimeError("NotConnected") from exc

    def request(self, method: str, params: dict, timeout_ms: int) -> dict:
        with self._lock:
            id_ = self._next_id
            self._next_id += 1
        self._send({"jsonrpc": "2.0", "id": id_, "method": method, "params": params})
        deadline = time.time() + timeout_ms / 1000.0
        while time.time() < deadline:
            with self._lock:
                if id_ in self._errors:
                    raise RuntimeError(f"lsp request failed: {self._errors.pop(id_)}")
                if id_ in self._responses:
                    return self._responses.pop(id_)
                alive = self.alive
            if not alive:
                raise RuntimeError("NotConnected")
            time.sleep(0.005)
        raise TimeoutError(f"lsp request timed out: {method}")

    def notify(self, method: str, params: dict) -> None:
        self._send({"jsonrpc": "2.0", "method": method, "params": params})

    # -- document ops -------------------------------------------------------

    def did_open(self, path: str, content: str) -> None:
        self.notify("textDocument/didOpen", {
            "textDocument": {"uri": _file_uri(path), "languageId": self.language_id,
                             "version": 1, "text": content},
        })
        with self._lock:
            self._open_docs.add(path)

    def did_change(self, path: str, content: str) -> None:
        self.notify("textDocument/didChange", {
            "textDocument": {"uri": _file_uri(path), "version": 2},
            "contentChanges": [{"text": content}],
        })

    def did_save(self, path: str, content: str) -> None:
        self.notify("textDocument/didSave", {"textDocument": {"uri": _file_uri(path)}, "text": content})

    def did_close(self, path: str) -> None:
        self.notify("textDocument/didClose", {"textDocument": {"uri": _file_uri(path)}})

    def completion(self, path: str, line: int, character: int) -> dict:
        return self.request("textDocument/completion",
                            {"textDocument": {"uri": _file_uri(path)},
                             "position": {"line": line, "character": character}}, 8_000)

    def hover(self, path: str, line: int, character: int) -> dict | None:
        return self.request("textDocument/hover",
                            {"textDocument": {"uri": _file_uri(path)},
                             "position": {"line": line, "character": character}}, 8_000)

    def locations(self, method: str, path: str, line: int, character: int) -> dict:
        return self.request(method,
                            {"textDocument": {"uri": _file_uri(path)},
                             "position": {"line": line, "character": character}}, 8_000)

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


# ---------------------------------------------------------------------------
# Manager (single lock over the clients map — the crash-audit fix)
# ---------------------------------------------------------------------------

_MGR_LOCK = threading.RLock()
_CLIENTS: dict[str, Client] = {}


def _workspace_root() -> str:
    return os.environ.get("PWD") or os.getcwd()


def _get_client(language_id: str) -> Client:
    with _MGR_LOCK:
        client = _CLIENTS.get(language_id)
        if client is not None and client.alive:
            return client
        candidates = SERVER_CANDIDATES.get(language_id) or []
        for command, args in candidates:
            resolved = _resolve_command(command)
            if resolved == command and "/" not in command:
                continue
            try:
                client = Client(language_id, _workspace_root(), resolved, args)
            except OSError:
                continue
            try:
                client.request("initialize", {
                    "processId": None,
                    "clientInfo": {"name": "forge-ade", "version": "0.6.0"},
                    "rootUri": "file://" + _workspace_root(),
                    "capabilities": {
                        "textDocument": {
                            "completion": {"completionItem": {"snippetSupport": True}},
                            "hover": {"contentFormat": ["markdown"]},
                            "definition": {}, "declaration": {},
                            "typeDefinition": {}, "implementation": {},
                        },
                        "workspace": {"workspaceFolders": {}},
                    },
                    "workspaceFolders": [],
                }, 15_000)
                client.notify("initialized", {})
                client.initialized = True
                client.status = "running"
            except Exception:
                client.close()
                continue
            _CLIENTS[language_id] = client
            return client
        raise RuntimeError("lsp server not available")


# ---------------------------------------------------------------------------
# Wire handlers
# ---------------------------------------------------------------------------


def _normalize_items(res) -> list:
    if isinstance(res, list):
        return res
    if isinstance(res, dict) and isinstance(res.get("items"), list):
        return res["items"]
    return []


@bridge.cmd("services.LSPDidOpen")
def lsp_did_open(payload: dict):
    lang = language_id_from_path(payload.get("path", ""))
    if not lang:
        return {"ok": True}
    with _MGR_LOCK:
        client = _get_client(lang)
        client.did_open(payload.get("path", ""), payload.get("content", ""))
    return {"ok": True}


@bridge.cmd("services.LSPDidChange")
def lsp_did_change(payload: dict):
    lang = language_id_from_path(payload.get("path", ""))
    with _MGR_LOCK:
        client = _CLIENTS.get(lang)
        if client:
            client.did_change(payload.get("path", ""), payload.get("content", ""))
    return {"ok": True}


@bridge.cmd("services.LSPDidSave")
def lsp_did_save(payload: dict):
    lang = language_id_from_path(payload.get("path", ""))
    with _MGR_LOCK:
        client = _CLIENTS.get(lang)
        if client:
            client.did_save(payload.get("path", ""), payload.get("content") or "")
    return {"ok": True}


@bridge.cmd("services.LSPDidClose")
def lsp_did_close(payload: dict):
    lang = language_id_from_path(payload.get("path", ""))
    with _MGR_LOCK:
        client = _CLIENTS.get(lang)
        if client:
            client.did_close(payload.get("path", ""))
    return {"ok": True}


@bridge.cmd("services.LSPGetCompletion")
def lsp_completion(payload: dict) -> list:
    lang = language_id_from_path(payload.get("path", ""))
    if not lang:
        raise RuntimeError("unsupported language")
    with _MGR_LOCK:
        client = _get_client(lang)
        return _normalize_items(client.completion(
            payload.get("path", ""), int(payload.get("line") or 0), int(payload.get("character") or 0)))


@bridge.cmd("services.LSPGetHover")
def lsp_hover(payload: dict):
    lang = language_id_from_path(payload.get("path", ""))
    if not lang:
        return None
    with _MGR_LOCK:
        client = _get_client(lang)
        return client.hover(payload.get("path", ""), int(payload.get("line") or 0),
                            int(payload.get("character") or 0))


def _locations(lsp_method: str, payload: dict) -> list:
    lang = language_id_from_path(payload.get("path", ""))
    if not lang:
        return []
    with _MGR_LOCK:
        client = _get_client(lang)
        res = client.locations(lsp_method, payload.get("path", ""),
                               int(payload.get("line") or 0), int(payload.get("character") or 0))
    if isinstance(res, list):
        return res
    if isinstance(res, dict):
        return [res]
    return []


@bridge.cmd("services.LSPGetDefinition")
def lsp_definition(payload: dict) -> list:
    return _locations("textDocument/definition", payload)


@bridge.cmd("services.LSPGetDeclaration")
def lsp_declaration(payload: dict) -> list:
    return _locations("textDocument/declaration", payload)


@bridge.cmd("services.LSPGetTypeDefinition")
def lsp_type_definition(payload: dict) -> list:
    return _locations("textDocument/typeDefinition", payload)


@bridge.cmd("services.LSPGetImplementation")
def lsp_implementation(payload: dict) -> list:
    return _locations("textDocument/implementation", payload)


@bridge.cmd("services.LSPGetDiagnostics")
def lsp_diagnostics(payload: dict) -> dict:
    out = {}
    with _MGR_LOCK:
        for lang, client in _CLIENTS.items():
            with client._lock:
                for path, diags_json in client._diagnostics.items():
                    try:
                        diags = json.loads(diags_json)
                    except json.JSONDecodeError:
                        continue
                    out[path] = {
                        "errors": sum(1 for d in diags if d.get("severity") == 1),
                        "warnings": sum(1 for d in diags if d.get("severity") == 2),
                        "diagnostics": diags,
                    }
    return out


@bridge.cmd("services.LSPListServers")
def lsp_list_servers(payload: dict) -> list:
    with _MGR_LOCK:
        return [{
            "languageId": c.language_id, "name": c.command, "command": c.command,
            "args": [], "status": c.status, "workspaceRoot": c.workspace_root,
            "openDocumentsCount": len(c._open_docs),
            "errorsCount": 0, "warningsCount": 0,
        } for c in _CLIENTS.values()]


def _stop(language_id: str) -> None:
    with _MGR_LOCK:
        client = _CLIENTS.pop(language_id, None)
    if client:
        client.close()


@bridge.cmd("services.LSPRestartServer", "services.LSPStopServer")
def lsp_stop_server(payload: dict):
    lang = payload.get("languageId") or payload.get("id") or ""
    if lang:
        _stop(lang)
    return {"ok": True}


@bridge.cmd("services.LSPRestartAll", "services.LSPStopAll")
def lsp_stop_all(payload: dict) -> dict:
    with _MGR_LOCK:
        langs = list(_CLIENTS.keys())
    for lang in langs:
        _stop(lang)
    return {lang: True for lang in langs}


@bridge.cmd("services.LSPGetServerLogs")
def lsp_server_logs(payload: dict) -> list:
    return []
