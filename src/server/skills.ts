// Skills manager — multi-source discovery over every supported agent tool's
// skill locations, with frontmatter parsing and name-collision resolution.

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
  enabled: boolean;
}

export class SkillsManager {
  private cached: SkillInfo[] | null = null;
  private cachedAt = 0;
  private config?: ConfigStore | undefined;
  private static CACHE_TTL_MS = 60_000;

  constructor(_dataDir?: string, config?: ConfigStore) {
    this.config = config;
  }

  public listAllSkills(projectFolder?: string): SkillInfo[] {
    const cwd = projectFolder || process.cwd();
    const ignored = new Set(this.config?.getSkills().ignored ?? []);
    const extraDirs = this.config?.getSkills().extra_dirs ?? [];
    const { skills, warnings } = discoverSkills(cwd, { extraDirs, includeDisabled: true });
    if (this.cached === null) {
      for (const w of warnings) console.warn(`[skills] ${w}`);
    }
    return skills.map((s) => ({
      name: s.name,
      description: s.description,
      path: s.filePath,
      source: s.source,
      enabled: !ignored.has(s.name),
    }));
  }

  public listSkills(projectFolder?: string): SkillInfo[] {
    if (this.cached && Date.now() - this.cachedAt < SkillsManager.CACHE_TTL_MS) {
      return this.cached;
    }
    const all = this.listAllSkills(projectFolder);
    this.cached = all.filter((s) => s.enabled);
    this.cachedAt = Date.now();
    return this.cached;
  }

  public setSkillEnabled(name: string, enabled: boolean): void {
    if (!this.config) return;
    this.config.saveSkills((skills) => {
      const ignored = new Set(skills.ignored ?? []);
      if (enabled) {
        ignored.delete(name);
      } else {
        ignored.add(name);
      }
      skills.ignored = Array.from(ignored);
    });
    this.cached = null;
  }

  public setAllSkillsEnabled(enabled: boolean, projectFolder?: string): void {
    if (!this.config) return;
    if (enabled) {
      this.config.saveSkills((skills) => {
        skills.ignored = [];
      });
    } else {
      const all = this.listAllSkills(projectFolder);
      this.config.saveSkills((skills) => {
        skills.ignored = all.map((s) => s.name);
      });
    }
    this.cached = null;
  }

  /** Loads a skill's full instruction body for /skill:<name> invocation. */
  public loadSkillBody(name: string, projectFolder?: string): { body: string; baseDir: string } | null {
    const all = this.listAllSkills(projectFolder);
    const found = all.find((s) => s.name === name);
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
