import { spawnSync } from "child_process";
import path from "path";
import fs from "fs";

export interface FileStatus {
  path: string;
  dir: string;
  staging: string;
  status: string;
}

export interface GitStatusResult {
  branch: string;
  ahead: number;
  behind: number;
  staged: FileStatus[];
  unstaged: FileStatus[];
  untracked: FileStatus[];
  conflicts: FileStatus[];
}

export interface CommitNode {
  hash: string;
  short_hash: string;
  parents: string[];
  author_name: string;
  author_email: string;
  timestamp: string;
  message: string;
  graph_prefix: string;
  decorations: string;
  status?: string;
}

export interface CommitGraphResult {
  commits: CommitNode[];
  total_count: number;
  has_more: boolean;
  offset: number;
  limit: number;
}

export interface DiffHunk {
  oldStart: number;
  oldLines: number;
  newStart: number;
  newLines: number;
  header: string;
  lines: string[];
}

export class GitManager {
  private runGit(repoPath: string, args: string[]): string {
    const cwd = repoPath ? path.resolve(repoPath) : process.cwd();
    const res = spawnSync("git", args, {
      cwd,
      encoding: "utf-8",
      maxBuffer: 20 * 1024 * 1024,
    });
    if (res.error) throw res.error;
    return res.stdout || "";
  }

  public getGitStatus(repoPath: string): GitStatusResult {
    try {
      const branchOut = this.runGit(repoPath, ["rev-parse", "--abbrev-ref", "HEAD"]).trim();
      const statusOut = this.runGit(repoPath, ["status", "--porcelain=v1", "-uall"]);

      const staged: FileStatus[] = [];
      const unstaged: FileStatus[] = [];
      const untracked: FileStatus[] = [];
      const conflicts: FileStatus[] = [];

      const lines = statusOut.split("\n").filter(Boolean);
      for (const line of lines) {
        if (line.length < 3) continue;
        const x = line[0];
        const y = line[1];
        let filePath = line.slice(3).trim();
        if (filePath.startsWith('"') && filePath.endsWith('"')) {
          filePath = filePath.slice(1, -1);
        }

        const dir = path.dirname(filePath);
        const dirFormatted = dir === "." ? "" : dir;

        // Check conflicts
        if (x === "U" || y === "U" || (x === "A" && y === "A") || (x === "D" && y === "D")) {
          conflicts.push({
            path: filePath,
            dir: dirFormatted,
            staging: "conflicts",
            status: `${x}${y}`,
          });
          continue;
        }

        if (x === "?" && y === "?") {
          untracked.push({
            path: filePath,
            dir: dirFormatted,
            staging: "untracked",
            status: "U",
          });
        } else {
          if (x !== " " && x !== "?") {
            staged.push({
              path: filePath,
              dir: dirFormatted,
              staging: "staged",
              status: x,
            });
          }
          if (y !== " " && y !== "?") {
            unstaged.push({
              path: filePath,
              dir: dirFormatted,
              staging: "unstaged",
              status: y,
            });
          }
        }
      }

      return {
        branch: branchOut || "main",
        ahead: 0,
        behind: 0,
        staged,
        unstaged,
        untracked,
        conflicts,
      };
    } catch {
      return {
        branch: "main",
        ahead: 0,
        behind: 0,
        staged: [],
        unstaged: [],
        untracked: [],
        conflicts: [],
      };
    }
  }

  public getGitBranches(repoPath: string): string[] {
    try {
      const out = this.runGit(repoPath, ["branch", "--list", "--format=%(refname:short)"]);
      return out.split("\n").map((b) => b.trim()).filter(Boolean);
    } catch {
      return [];
    }
  }

  public getGitCommitGraph(repoPath: string, offset: number = 0, limit: number = 50, branch: string = ""): CommitGraphResult {
    try {
      const args = [
        "log",
        "--graph",
        `--skip=${offset}`,
        `-n`,
        `${limit}`,
        "--format=format:COMMIT_ROW|%h|%H|%an|%ae|%aI|%p|%D|%s",
      ];
      if (branch) {
        args.push(branch);
      } else {
        args.push("--all");
      }

      const out = this.runGit(repoPath, args);
      const commits: CommitNode[] = [];
      const lines = out.split("\n");

      for (const line of lines) {
        const markerIdx = line.indexOf("COMMIT_ROW|");
        if (markerIdx === -1) continue;

        const graphPrefix = line.slice(0, markerIdx).trimEnd();
        const rawPayload = line.slice(markerIdx + "COMMIT_ROW|".length);
        const parts = rawPayload.split("|");
        if (parts.length < 8) continue;

        const short_hash = parts[0];
        const hash = parts[1];
        const author_name = parts[2];
        const author_email = parts[3];
        const timestamp = parts[4];
        const parents = parts[5] ? parts[5].split(" ").filter(Boolean) : [];
        const decorations = parts[6] || "";
        const message = parts.slice(7).join("|");

        commits.push({
          hash,
          short_hash,
          parents,
          author_name,
          author_email,
          timestamp,
          message,
          graph_prefix: graphPrefix || "*",
          decorations,
        });
      }

      return {
        commits,
        total_count: commits.length,
        has_more: commits.length >= limit,
        offset,
        limit,
      };
    } catch {
      return {
        commits: [],
        total_count: 0,
        has_more: false,
        offset,
        limit,
      };
    }
  }

  public getGitCommitDiff(repoPath: string, hash: string): string {
    return this.runGit(repoPath, ["show", hash]);
  }

  public getGitCommitBody(repoPath: string, hash: string): string {
    return this.runGit(repoPath, ["log", "-1", "--format=%B", hash]);
  }

  public getGitFileDiff(repoPath: string, filePath: string): string {
    return this.runGit(repoPath, ["diff", "HEAD", "--", filePath]);
  }

  public getGitCommitFileDiff(repoPath: string, hash: string, filePath: string): string {
    return this.runGit(repoPath, ["show", `${hash}`, "--", filePath]);
  }

  public getGitFileContentAtCommit(repoPath: string, hash: string, filePath: string): string {
    return this.runGit(repoPath, ["show", `${hash}:${filePath}`]);
  }

  public getGitFileDiffHunks(repoPath: string, filePath: string): DiffHunk[] {
    const diff = this.getGitFileDiff(repoPath, filePath);
    const hunks: DiffHunk[] = [];
    const lines = diff.split("\n");

    let currentHunk: DiffHunk | null = null;

    for (const line of lines) {
      if (line.startsWith("@@")) {
        if (currentHunk) hunks.push(currentHunk);
        const match = line.match(/@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@(.*)/);
        if (match) {
          currentHunk = {
            oldStart: parseInt(match[1], 10),
            oldLines: match[2] ? parseInt(match[2], 10) : 1,
            newStart: parseInt(match[3], 10),
            newLines: match[4] ? parseInt(match[4], 10) : 1,
            header: line,
            lines: [],
          };
        }
      } else if (currentHunk) {
        currentHunk.lines.push(line);
      }
    }
    if (currentHunk) hunks.push(currentHunk);
    return hunks;
  }

  public revertGitHunk(repoPath: string, filePath: string, hunkIndex: number): void {
    this.runGit(repoPath, ["checkout", "HEAD", "--", filePath]);
  }

  public gitStage(repoPath: string, paths: string[]): void {
    this.runGit(repoPath, ["add", "--", ...paths]);
  }

  public gitUnstage(repoPath: string, paths: string[]): void {
    this.runGit(repoPath, ["reset", "HEAD", "--", ...paths]);
  }

  public gitDiscard(repoPath: string, paths: string[]): void {
    try {
      this.runGit(repoPath, ["checkout", "HEAD", "--", ...paths]);
    } catch {}
    try {
      this.runGit(repoPath, ["clean", "-f", "--", ...paths]);
    } catch {}
  }

  public getGitConflictStageContent(repoPath: string, filePath: string, stage: number): string {
    return this.runGit(repoPath, ["show", `:${stage}:${filePath}`]);
  }

  public gitResolveConflict(repoPath: string, filePath: string, action: string): void {
    if (action === "ours") {
      this.runGit(repoPath, ["checkout", "--ours", "--", filePath]);
      this.gitStage(repoPath, [filePath]);
    } else if (action === "theirs") {
      this.runGit(repoPath, ["checkout", "--theirs", "--", filePath]);
      this.gitStage(repoPath, [filePath]);
    } else if (action === "mark") {
      this.gitStage(repoPath, [filePath]);
    }
  }

  public gitCommit(repoPath: string, message: string): void {
    this.runGit(repoPath, ["commit", "-m", message]);
  }

  public gitPush(repoPath: string): void {
    this.runGit(repoPath, ["push"]);
  }

  public gitFetch(repoPath: string): string {
    return this.runGit(repoPath, ["fetch"]);
  }

  public gitMerge(repoPath: string, source: string, noFF: boolean, squash: boolean): string {
    const args = ["merge"];
    if (noFF) args.push("--no-ff");
    if (squash) args.push("--squash");
    args.push(source);
    return this.runGit(repoPath, args);
  }

  public generateAICommitMessage(repoPath: string, providerId: string, model: string, instruction?: string): string {
    const diff = this.runGit(repoPath, ["diff", "--cached", "--stat"]);
    return `feat: update project files (${diff.split("\n")[0]?.trim() || "staged changes"})`;
  }
}
