"""forge_ade.search — gitignore-aware filename/content search + replace.

Port of search.zig (SKIP_DIRS, binary-extension skip, 1MB cap, simple
gitignore rules). Handles both payload shapes: {opts: {...}} (the frontend's
SearchFilenameWithOptions) and the flat option object.
"""

from __future__ import annotations

import fnmatch
import os
import re
from pathlib import Path

from . import bridge
from .util import home_dir

SKIP_DIRS = {".git", "node_modules", "zig-out", ".zig-cache", ".native", "dist", "build",
             "Pods", ".gradle", "DerivedData", ".build", ".swiftpm", "Carthage", ".yarn",
             "vendor", "__pycache__", ".DS_Store", ".idea", ".vscode", ".cache", ".next",
             ".nuxt", ".turbo", "coverage", ".venv", "venv", "target", ".dart_tool"}

BIN_EXTS = {"png", "jpg", "jpeg", "gif", "webp", "svg", "ico", "avif", "icns",
            "zip", "gz", "tgz", "xz", "zst", "7z", "rar", "tar", "bz2",
            "wasm", "woff", "woff2", "ttf", "otf", "eot", "mp4", "mp3", "mov",
            "pdf", "exe", "dll", "dylib", "so", "bin"}


def _opts(payload: dict) -> dict:
    value = payload.get("opts")
    return value if isinstance(value, dict) else payload


def _parse_gitignore(root: str) -> list[tuple[str, bool]]:
    rules: list[tuple[str, bool]] = []
    try:
        text = (Path(root) / ".gitignore").read_text(encoding="utf-8", errors="replace")
    except OSError:
        return rules
    for line in text.split("\n"):
        line = line.strip()
        if not line or line.startswith("#") or line.startswith("!"):
            continue
        dir_only = line.endswith("/")
        if dir_only:
            line = line[:-1]
        if line.startswith("**/"):
            line = line[3:]
        rules.append((line, dir_only))
    return rules


def _rule_matches(pattern: str, rel: str, is_dir: bool, dir_only: bool) -> bool:
    if dir_only and not is_dir:
        return False
    if pattern.endswith("/*"):
        prefix = pattern[:-2]
        return rel.startswith(prefix) and len(rel) > len(prefix)
    if "/" not in pattern:
        base = os.path.basename(rel)
        return base == pattern or rel == pattern
    return fnmatch.fnmatch(rel, pattern) or fnmatch.fnmatch(rel, f"**/{pattern}")


def _is_ignored(rel: str, is_dir: bool, rules: list[tuple[str, bool]]) -> bool:
    parts = rel.split("/")
    for i in range(len(parts)):
        sub = "/".join(parts[: i + 1])
        for pattern, dir_only in rules:
            if _rule_matches(pattern, sub, is_dir or i < len(parts) - 1, dir_only):
                return True
    return False


def _walk(folder: str, rules: list[tuple[str, bool]], on_entry) -> None:
    def walk_dir(directory: str, prefix: str) -> bool:
        try:
            with os.scandir(directory) as it:
                entries = sorted(it, key=lambda e: e.name)
        except OSError:
            return True
        for entry in entries:
            name = entry.name
            if name in SKIP_DIRS:
                continue
            is_dir = entry.is_dir(follow_symlinks=False)
            rel = f"{prefix}{name}" if prefix else name
            if rules and _is_ignored(rel, is_dir, rules):
                continue
            full = os.path.join(directory, name)
            if not on_entry(full, rel, is_dir):
                return False
            if is_dir and walk_dir(full, rel + "/") is False:
                return False
        return True

    walk_dir(folder, "")


def _substring(haystack: str, needle: str, case_sensitive: bool) -> bool:
    if not needle:
        return False
    if case_sensitive:
        return needle in haystack
    return needle.lower() in haystack.lower()


@bridge.cmd("services.SearchFilenameWithOptions", "services.SearchFilename")
def search_filename(payload: dict) -> list[dict]:
    opts = _opts(payload)
    query = opts.get("query", "")
    folder = opts.get("folder") or home_dir()
    limit = int(opts.get("limit") or 50)
    case_sensitive = bool(opts.get("caseSensitive"))
    rules = _parse_gitignore(folder) if opts.get("respectGitignore", True) else []
    results: list[dict] = []

    def on_entry(full: str, rel: str, is_dir: bool) -> bool:
        base = os.path.basename(full)
        if not _substring(base, query, case_sensitive):
            return True
        exact = not is_dir and base.lower() == query.lower()
        results.append({
            "path": full, "name": base, "isDir": is_dir,
            "score": 100 if exact else (60 if is_dir else 50),
        })
        return len(results) < limit

    _walk(folder, rules, on_entry)
    results.sort(key=lambda r: (-r["score"], r["path"]))
    return results


def _searchable(path: str) -> bool:
    ext = os.path.splitext(path)[1].lstrip(".").lower()
    if ext in BIN_EXTS:
        return False
    try:
        return os.path.getsize(path) <= 1_000_000
    except OSError:
        return False


@bridge.cmd("services.SearchContentWithOptions", "services.FindSymbol", "services.SearchIndexSymbols")
def search_content(payload: dict) -> list[dict]:
    opts = _opts(payload)
    query = opts.get("query") or payload.get("name") or ""
    folder = opts.get("folder") or home_dir()
    limit = int(opts.get("limit") or 100)
    case_sensitive = bool(opts.get("caseSensitive"))
    rules = _parse_gitignore(folder) if opts.get("respectGitignore", True) else []
    results: list[dict] = []
    if not query:
        return results

    def on_entry(full: str, rel: str, is_dir: bool) -> bool:
        if is_dir or not _searchable(full):
            return True
        try:
            text = Path(full).read_text(encoding="utf-8", errors="replace")
        except OSError:
            return True
        for line_no, line in enumerate(text.split("\n"), start=1):
            if _substring(line, query, case_sensitive):
                results.append({
                    "path": full, "name": os.path.basename(full), "isDir": False,
                    "score": 1, "line": line_no, "snippet": line.strip()[:500],
                })
                if len(results) >= limit:
                    return False
        return True

    _walk(folder, rules, on_entry)
    return results


@bridge.cmd("services.SearchReplaceAll")
def search_replace_all(payload: dict) -> dict:
    opts = _opts(payload)
    query = opts.get("query", "")
    replacement = opts.get("replacement", "")
    folder = opts.get("folder") or home_dir()
    case_sensitive = bool(opts.get("caseSensitive"))
    rules = _parse_gitignore(folder) if opts.get("respectGitignore", True) else []
    files_changed = 0
    total_replacements = 0
    files: list[str] = []
    if not query:
        return {"filesChanged": 0, "totalReplacements": 0, "files": []}

    pattern = re.compile(re.escape(query), 0 if case_sensitive else re.IGNORECASE)

    def on_entry(full: str, rel: str, is_dir: bool) -> bool:
        nonlocal files_changed, total_replacements
        if is_dir or not _searchable(full):
            return True
        try:
            raw = Path(full).read_text(encoding="utf-8", errors="strict")
        except (OSError, UnicodeDecodeError):
            return True
        new_text, count = pattern.subn(replacement.replace("\\", "\\\\"), raw)
        if count == 0 or new_text == raw:
            return True
        try:
            Path(full).write_text(new_text, encoding="utf-8")
        except OSError:
            return True
        files_changed += 1
        total_replacements += count
        files.append(full)
        return True

    _walk(folder, rules, on_entry)
    return {"filesChanged": files_changed, "totalReplacements": total_replacements, "files": files}
