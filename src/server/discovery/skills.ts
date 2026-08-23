// Multi-source skill discovery — ported from the reference discovery layer.
// Skills are SKILL.md files (YAML frontmatter: name/description; body:
// instructions) discovered from every major agent tool's conventional
// location, deduplicated by name with source priority.

import fs from "fs";
import path from "path";
import os from "os";

export interface Skill {
  name: string;
  description: string;
  filePath: string;
  baseDir: string;
  /** Where it came from, e.g. "claude:user", "agents:project". */
  source: string;
}

export interface LoadSkillsResult {
  skills: Skill[];
  warnings: string[];
}

/** Provider scan roots. Order = priority when names collide (first wins). */
interface SkillSource {
  provider: string;
  level: "user" | "project";
  dir: string;
}

function home(): string {
  return os.homedir();
}

/** Walks up from cwd to the filesystem root collecting ancestor dirs (closest first). */
function ancestorsFrom(cwd: string): string[] {
  const out: string[] = [];
  let current = path.resolve(cwd);
  while (true) {
    out.push(current);
    const parent = path.dirname(current);
    if (parent === current) break;
    current = parent;
  }
  return out;
}

function projectSkillSources(cwd: string): SkillSource[] {
  const sources: SkillSource[] = [];
  for (const dir of ancestorsFrom(cwd)) {
    for (const base of [".agents", ".agent"]) {
      sources.push({ provider: "agents", level: "project", dir: path.join(dir, base, "skills") });
    }
    sources.push({ provider: "claude", level: "project", dir: path.join(dir, ".claude", "skills") });
    sources.push({ provider: "codex", level: "project", dir: path.join(dir, ".codex", "skills") });
    sources.push({ provider: "gemini", level: "project", dir: path.join(dir, ".gemini", "skills") });
    sources.push({ provider: "opencode", level: "project", dir: path.join(dir, ".opencode", "skills") });
    sources.push({ provider: "github", level: "project", dir: path.join(dir, ".github", "skills") });
    sources.push({ provider: "native", level: "project", dir: path.join(dir, ".omp", "skills") });
    // Stop the walk at the repo boundary to avoid scanning unrelated trees.
    if (fs.existsSync(path.join(dir, ".git"))) break;
  }
  return sources;
}

function userSkillSources(): SkillSource[] {
  return [
    { provider: "native", level: "user", dir: path.join(home(), ".forge-ade", "skills") },
    { provider: "agents", level: "user", dir: path.join(home(), ".agents", "skills") },
    { provider: "claude", level: "user", dir: path.join(home(), ".claude", "skills") },
    { provider: "codex", level: "user", dir: path.join(home(), ".codex", "skills") },
    { provider: "gemini", level: "user", dir: path.join(home(), ".gemini", "skills") },
    { provider: "opencode", level: "user", dir: path.join(home(), ".config", "opencode", "skills") },
  ];
}

const FRONTMATTER_RE = /^---\r?\n([\s\S]*?)\r?\n---\r?\n?/;

/** Minimal YAML frontmatter reader: top-level `key: value` pairs only. */
export function parseFrontmatter(raw: string): Record<string, string> {
  const match = FRONTMATTER_RE.exec(raw);
  const fields: Record<string, string> = {};
  if (!match) return fields;
  for (const line of match[1].split("\n")) {
    const idx = line.indexOf(":");
    if (idx <= 0) continue;
    const key = line.slice(0, idx).trim();
    const value = line.slice(idx + 1).trim().replace(/^["']|["']$/g, "");
    if (key && !(key in fields)) fields[key] = value;
  }
  return fields;
}

interface ScannedSkill {
  skill: Skill;
  realPath: string;
}

function scanDir(source: SkillSource, warnings: string[]): ScannedSkill[] {
  let entries: fs.Dirent[];
  try {
    entries = fs.readdirSync(source.dir, { withFileTypes: true });
  } catch {
    return [];
  }
  const found: ScannedSkill[] = [];
  for (const entry of entries) {
    // Layout A: <dir>/<name>/SKILL.md   Layout B: <dir>/<name>.md
    const skillDir = entry.isDirectory() ? path.join(source.dir, entry.name) : null;
    const file = skillDir ? path.join(skillDir, "SKILL.md") : path.join(source.dir, entry.name);
    if (!skillDir && !entry.isFile()) continue;
    if (!skillDir && path.extname(entry.name) !== ".md") continue;
    let raw: string;
    try {
      raw = fs.readFileSync(file, "utf-8");
    } catch {
      continue;
    }
    const fields = parseFrontmatter(raw);
    const name = fields.name || (skillDir ? path.basename(skillDir) : path.basename(entry.name, ".md"));
    if (!name) continue;
    let realPath = file;
    try {
      realPath = fs.realpathSync(file);
    } catch {}
    found.push({
      realPath,
      skill: {
        name,
        description: fields.description || "",
        filePath: file,
        baseDir: skillDir ?? source.dir,
        source: `${source.provider}:${source.level}`,
      },
    });
    void warnings;
  }
  return found;
}

/**
 * Loads skills from every supported source. Name collisions resolve to the
 * higher-priority source (earlier in the scan order); identical files reached
 * through symlinks load once.
 */
export function discoverSkills(cwd: string, options?: { ignored?: string[] }): LoadSkillsResult {
  const warnings: string[] = [];
  const sources = [...projectSkillSources(cwd), ...userSkillSources()];
  const byName = new Map<string, Skill>();
  const seenRealPaths = new Set<string>();
  const ignored = new Set(options?.ignored ?? []);

  for (const source of sources) {
    for (const { skill, realPath } of scanDir(source, warnings)) {
      if (seenRealPaths.has(realPath)) continue;
      seenRealPaths.add(realPath);
      if (ignored.has(skill.name)) continue;
      const existing = byName.get(skill.name);
      if (existing) {
        if (existing.filePath !== skill.filePath) {
          warnings.push(
            `skill "${skill.name}" collision: keeping ${existing.source} (${existing.filePath}), skipping ${skill.source}`,
          );
        }
        continue;
      }
      byName.set(skill.name, skill);
    }
  }

  const skills = [...byName.values()].sort((a, b) => a.name.localeCompare(b.name));
  return { skills, warnings };
}
