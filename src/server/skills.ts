import fs from "fs";
import path from "path";
import { discoverSkills } from "./discovery/skills";
import type { Skill } from "./discovery/skills";
import type { ConfigStore } from "./config";

export interface SkillInfo {
  name: string;
  description: string;
  path: string;
  source: string;
}

export class SkillsManager {
  private cached: SkillInfo[] | null = null;
  private cachedAt = 0;
  private config?: ConfigStore | undefined;
  private static CACHE_TTL_MS = 60_000;

  constructor(_dataDir?: string, config?: ConfigStore) {
    this.config = config;
  }

  public listSkills(projectFolder?: string): SkillInfo[] {
    if (this.cached && Date.now() - this.cachedAt < SkillsManager.CACHE_TTL_MS) {
      return this.cached;
    }
    const cwd = projectFolder || process.cwd();
    const ignored = this.config?.getSkills().ignored ?? [];
    const { skills, warnings } = discoverSkills(cwd, { ignored });
    if (this.cached === null) {
      // Warn once per daemon run; collisions are static config issues.
      for (const w of warnings) console.warn(`[skills] ${w}`);
    }
    this.cached = skills.map((s) => ({
      name: s.name,
      description: s.description,
      path: s.filePath,
      source: s.source,
    }));
    this.cachedAt = Date.now();
    return this.cached;
  }

  /** Loads a skill's full instruction body for /skill:<name> invocation. */
  public loadSkillBody(name: string, projectFolder?: string): { body: string; baseDir: string } | null {
    const skills = this.cached ?? this.listSkills(projectFolder);
    const found = skills.find((s) => s.name === name);
    if (!found) return null;
    try {
      return {
        body: fs.readFileSync(found.path, "utf-8"),
        baseDir: found.path.slice(0, found.path.lastIndexOf(path.sep)),
      };
    } catch {
      return null;
    }
  }
}

export type { Skill };
