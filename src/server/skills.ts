import fs from "fs";
import path from "path";
import os from "os";

export interface Skill {
  name: string;
  description: string;
  path: string;
}

export class SkillsManager {
  public listSkills(): Skill[] {
    const skills: Skill[] = [];
    const dirs = [
      path.join(os.homedir(), ".agents", "skills"),
      path.join(process.cwd(), ".skills"),
    ];

    for (const d of dirs) {
      try {
        if (fs.existsSync(d)) {
          const entries = fs.readdirSync(d, { withFileTypes: true });
          for (const entry of entries) {
            if (entry.isDirectory()) {
              const skillFile = path.join(d, entry.name, "SKILL.md");
              if (fs.existsSync(skillFile)) {
                const content = fs.readFileSync(skillFile, "utf-8");
                const descMatch = content.match(/description:\s*(.+)/);
                skills.push({
                  name: entry.name,
                  description: descMatch ? descMatch[1].trim() : "Custom agent skill",
                  path: skillFile,
                });
              }
            }
          }
        }
      } catch {}
    }

    return skills;
  }
}
