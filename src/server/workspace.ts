import fs from "fs";
import path from "path";
import os from "os";

export interface Workspace {
  name: string;
  folders: string[];
  isTemporary: boolean;
  filePath: string;
  theme: string;
  settings?: Record<string, any>;
}

export interface RecentEntry {
  path: string;
  name: string;
  isWorkspace: boolean;
  lastOpened: number;
  pinned: boolean;
  favorite: boolean;
}

export class WorkspaceManager {
  private dataDir: string;
  private currentWorkspace: Workspace | null = null;
  private recentProjects: RecentEntry[] = [];
  private recentFile: string;

  constructor(dataDir?: string) {
    this.dataDir = dataDir || path.join(os.homedir(), ".forge-ade");
    if (!fs.existsSync(this.dataDir)) {
      try {
        fs.mkdirSync(this.dataDir, { recursive: true });
      } catch {}
    }
    this.recentFile = path.join(this.dataDir, "recent_projects.json");
    this.loadRecent();
  }

  private loadRecent(): void {
    try {
      if (fs.existsSync(this.recentFile)) {
        const raw = fs.readFileSync(this.recentFile, "utf-8");
        this.recentProjects = JSON.parse(raw);
      }
    } catch {
      this.recentProjects = [];
    }
  }

  private saveRecent(): void {
    try {
      fs.writeFileSync(this.recentFile, JSON.stringify(this.recentProjects, null, 2), "utf-8");
    } catch (err) {
      console.error("Failed to save recent projects:", err);
    }
  }

  public openFolder(folderPath: string): Workspace {
    const resolved = path.resolve(folderPath.replace(/^~/, os.homedir()));
    const name = path.basename(resolved) || "Project";
    const ws: Workspace = {
      name,
      folders: [resolved],
      isTemporary: true,
      filePath: "",
      theme: "dark-plus",
    };
    this.currentWorkspace = ws;
    this.addRecent(resolved, name, false);
    return ws;
  }

  public openWorkspace(filePath: string): Workspace {
    const resolved = path.resolve(filePath.replace(/^~/, os.homedir()));
    try {
      if (fs.existsSync(resolved)) {
        const raw = fs.readFileSync(resolved, "utf-8");
        const parsed = JSON.parse(raw);
        const ws: Workspace = {
          name: parsed.name || path.basename(resolved, ".workspace"),
          folders: parsed.folders || [path.dirname(resolved)],
          isTemporary: false,
          filePath: resolved,
          theme: parsed.theme || "dark-plus",
          settings: parsed.settings,
        };
        this.currentWorkspace = ws;
        this.addRecent(resolved, ws.name, true);
        return ws;
      }
    } catch (err) {
      console.error("Error reading workspace file:", err);
    }
    return this.openFolder(path.dirname(resolved));
  }

  public saveWorkspace(): void {
    if (!this.currentWorkspace) return;
    if (this.currentWorkspace.filePath) {
      this.saveWorkspaceAs(this.currentWorkspace.filePath);
    }
  }

  public saveWorkspaceAs(filePath: string): void {
    if (!this.currentWorkspace) return;
    const resolved = path.resolve(filePath.replace(/^~/, os.homedir()));
    const data = {
      name: this.currentWorkspace.name,
      folders: this.currentWorkspace.folders,
      theme: this.currentWorkspace.theme,
      settings: this.currentWorkspace.settings || {},
    };
    try {
      fs.writeFileSync(resolved, JSON.stringify(data, null, 2), "utf-8");
      this.currentWorkspace.filePath = resolved;
      this.currentWorkspace.isTemporary = false;
      this.addRecent(resolved, this.currentWorkspace.name, true);
    } catch (err) {
      console.error("Failed to save workspace file:", err);
      throw err;
    }
  }

  public closeWorkspace(): void {
    this.currentWorkspace = null;
  }

  public addFolderToWorkspace(folderPath: string): void {
    if (!this.currentWorkspace) return;
    const resolved = path.resolve(folderPath.replace(/^~/, os.homedir()));
    if (!this.currentWorkspace.folders.includes(resolved)) {
      this.currentWorkspace.folders.push(resolved);
    }
  }

  public removeFolderFromWorkspace(folderPath: string): void {
    if (!this.currentWorkspace) return;
    const resolved = path.resolve(folderPath.replace(/^~/, os.homedir()));
    this.currentWorkspace.folders = this.currentWorkspace.folders.filter((f) => f !== resolved);
  }

  public getCurrentWorkspace(): Workspace | null {
    return this.currentWorkspace;
  }

  public getRecentProjects(): RecentEntry[] {
    return [...this.recentProjects];
  }

  public pinRecent(targetPath: string, pinned: boolean): void {
    const entry = this.recentProjects.find((r) => r.path === targetPath);
    if (entry) {
      entry.pinned = pinned;
      this.saveRecent();
    }
  }

  public removeRecent(targetPath: string): void {
    this.recentProjects = this.recentProjects.filter((r) => r.path !== targetPath);
    this.saveRecent();
  }

  private addRecent(targetPath: string, name: string, isWorkspace: boolean): void {
    const existingIndex = this.recentProjects.findIndex((r) => r.path === targetPath);
    const pinned = existingIndex >= 0 ? this.recentProjects[existingIndex].pinned : false;
    const favorite = existingIndex >= 0 ? this.recentProjects[existingIndex].favorite : false;

    this.recentProjects = this.recentProjects.filter((r) => r.path !== targetPath);
    this.recentProjects.unshift({
      path: targetPath,
      name,
      isWorkspace,
      lastOpened: Date.now(),
      pinned,
      favorite,
    });
    // Keep max 50 recent projects
    if (this.recentProjects.length > 50) {
      this.recentProjects = this.recentProjects.slice(0, 50);
    }
    this.saveRecent();
  }
}
