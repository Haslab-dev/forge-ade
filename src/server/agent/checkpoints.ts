// Session file checkpoints for the rewind tool. Shared by core mutating
// handlers (write/edit snapshot before changing) and the extended rewind tool.

import fs from "fs";
import path from "path";
import type { ToolContext } from "./tools";

const CHECKPOINT_MAX_FILE_BYTES = 1_000_000;
const CHECKPOINT_KEEP_PER_SESSION = 400;

export interface CheckpointEntry {
  id: string;
  ts: number;
  path: string;
  before: string;
}

function checkpointFile(dataDir: string, sessionId: string): string {
  const safeId = sessionId.replace(/[^a-zA-Z0-9_-]/g, "_");
  return path.join(dataDir, "checkpoints", `${safeId}.jsonl`);
}

/** Persists pre-mutation content so `rewind` can restore it. Best-effort. */
export function snapshotForRewind(ctx: ToolContext, filePath: string, before: string): void {
  if (!ctx.sessionId || !ctx.dataDir) return;
  if (before.length > CHECKPOINT_MAX_FILE_BYTES) return;
  try {
    const file = checkpointFile(ctx.dataDir, ctx.sessionId);
    fs.mkdirSync(path.dirname(file), { recursive: true });
    const entry =
      JSON.stringify({
        id: `cp-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`,
        ts: Date.now(),
        path: filePath,
        before,
      }) + "\n";
    fs.appendFileSync(file, entry, "utf-8");
    // Bound growth: drop oldest entries beyond the keep window.
    const content = fs.readFileSync(file, "utf-8");
    const lines = content.split("\n").filter(Boolean);
    if (lines.length > CHECKPOINT_KEEP_PER_SESSION) {
      fs.writeFileSync(
        file,
        lines.slice(lines.length - CHECKPOINT_KEEP_PER_SESSION).join("\n") + "\n",
        "utf-8"
      );
    }
  } catch {}
}

export function readCheckpoints(dataDir: string, sessionId: string): CheckpointEntry[] {
  try {
    return fs
      .readFileSync(checkpointFile(dataDir, sessionId), "utf-8")
      .split("\n")
      .filter(Boolean)
      .map((l) => JSON.parse(l) as CheckpointEntry);
  } catch {
    return [];
  }
}
