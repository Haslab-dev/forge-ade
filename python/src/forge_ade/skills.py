"""forge_ade.skills — multi-source SKILL.md discovery.

Scans the standard agent skill directories (Claude, agents, opencode,
forge-ade's own) plus the project's, dedupes by name (first source wins),
and persists enable/disable state in ~/.forge-ade/skills.json.
"""

from __future__ import annotations

import json
import os
from pathlib import Path

from . import bridge
from .util import data_dir, read_json, write_json_file

SKILLS_STATE = data_dir() / "skills.json"


def _skill_roots() -> list[tuple[str, str]]:
    home = os.path.expanduser("~")
    cwd = os.environ.get("PWD") or os.getcwd()
    return [
        ("native", str(data_dir() / "skills")),
        ("claude", f"{home}/.claude/skills"),
        ("agents", f"{home}/.agents/skills"),
        ("opencode", f"{home}/.config/opencode/skills"),
        ("claude:project", f"{cwd}/.claude/skills"),
        ("agents:project", f"{cwd}/.agents/skills"),
    ]


def _parse_frontmatter(text: str) -> dict:
    out: dict = {}
    if text.startswith("---"):
        end = text.find("---", 3)
        if end != -1:
            for line in text[3:end].split("\n"):
                if ":" in line:
                    key, _, value = line.partition(":")
                    out[key.strip()] = value.strip().strip("\"'")
    return out


def _load_state() -> dict:
    state = read_json(SKILLS_STATE, default={})
    return state if isinstance(state, dict) else {}


def _save_state(state: dict) -> None:
    write_json_file(SKILLS_STATE, state)


def discover() -> list[dict]:
    state = _load_state()
    seen: set[str] = set()
    skills = []
    for source, root in _skill_roots():
        root_path = Path(root)
        if not root_path.is_dir():
            continue
        for entry in sorted(root_path.iterdir()):
            if not entry.is_dir():
                continue
            skill_md = entry / "SKILL.md"
            if not skill_md.exists():
                continue
            try:
                meta = _parse_frontmatter(skill_md.read_text(encoding="utf-8", errors="replace")[:4000])
            except OSError:
                continue
            name = meta.get("name") or entry.name
            if name in seen:
                continue
            seen.add(name)
            skills.append({
                "name": name,
                "description": meta.get("description", ""),
                "path": str(entry),
                "source": source,
                "enabled": state.get(name, True),
            })
    return skills


@bridge.cmd("services.ListSkills")
def list_skills(payload: dict) -> list:
    return [s for s in discover() if s["enabled"]]


@bridge.cmd("services.ListAllSkills", "services.RefreshSkills")
def list_all_skills(payload: dict) -> list:
    return discover()


@bridge.cmd("services.SetSkillEnabled")
def set_skill_enabled(payload: dict):
    name = payload.get("name", "")
    enabled = bool(payload.get("enabled", True))
    state = _load_state()
    state[name] = enabled
    _save_state(state)
    return {"ok": True}


@bridge.cmd("services.SetAllSkillsEnabled")
def set_all_skills_enabled(payload: dict):
    enabled = bool(payload.get("enabled", True))
    state = _load_state()
    for skill in discover():
        state[skill["name"]] = enabled
    _save_state(state)
    return {"ok": True}
