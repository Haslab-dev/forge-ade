import { describe, expect, it } from "vitest";
import { extractMentionedPaths, newThoughtStep, reversePatch } from "./goAgentSession";

describe("reversePatch", () => {
  it("reverses a simple replacement hunk", () => {
    const original = "line1\nline2\nline3";
    const modified = "line1\nline2-modified\nline3";
    const patch = [
      "--- a/f.txt",
      "+++ b/f.txt",
      "@@ -1,3 +1,3 @@",
      " line1",
      "-line2",
      "+line2-modified",
      " line3",
    ].join("\n");
    expect(reversePatch(modified, patch)).toBe(original);
  });

  it("reverses a pure addition", () => {
    const original = "a\nc";
    const modified = "a\nb\nc";
    const patch = "@@ -1,2 +1,3 @@\n a\n+b\n c";
    expect(reversePatch(modified, patch)).toBe(original);
  });

  it("reverses a pure deletion", () => {
    const original = "a\nb\nc";
    const modified = "a\nc";
    const patch = "@@ -1,3 +1,2 @@\n a\n-b\n c";
    expect(reversePatch(modified, patch)).toBe(original);
  });

  it("applies multiple hunks bottom-up so offsets stay valid", () => {
    const original = "l1\nl2\nl3\nl4\nl5\nl6";
    const modified = "L1\nl2\nl3\nl4\nl5\nL6";
    const patch = ["@@ -1,2 +1,2 @@", "-l1", "+L1", " l2", "@@ -5,2 +5,2 @@", " l5", "-l6", "+L6"].join("\n");
    expect(reversePatch(modified, patch)).toBe(original);
  });

  it("returns the input unchanged for an empty or header-only patch", () => {
    expect(reversePatch("abc", "")).toBe("abc");
    expect(reversePatch("abc", "--- a/f\n+++ b/f")).toBe("abc");
  });

  it("skips hunks whose start falls outside the modified content", () => {
    const patch = "@@ -100,1 +100,1 @@\n-x\n+y";
    expect(reversePatch("only\nlines", patch)).toBe("only\nlines");
    expect(reversePatch("x", "@@ -0,1 +0,1 @@\n-x\n+y")).toBe("x"); // start<0
  });

  it("handles default hunk counts (no ,n suffix)", () => {
    const original = "a\nb";
    const modified = "a\nB";
    const patch = "@@ -2 +2 @@\n-b\n+B";
    expect(reversePatch(modified, patch)).toBe(original);
  });

  it("ignores '\\ No newline at end of file' markers", () => {
    const patch = "@@ -1,1 +1,1 @@\n-a\n+b\n\\ No newline at end of file";
    expect(reversePatch("b", patch)).toBe("a");
  });

  it.skip("should detect a stale patch whose context no longer matches (FORGE-120)", () => {
    // reversePatch splices without verifying oldText against the content it
    // replaces, so a stale diff silently corrupts the reconstructed
    // "original" shown in the review pane.
    const stale = "@@ -1,3 +1,3 @@\n line1\n-line2\n+line2-modified\n line3";
    expect(() => reversePatch("completely\nunrelated", stale)).toThrow();
  });
});

describe("extractMentionedPaths", () => {
  it("resolves workspace-relative mentions to absolute paths", () => {
    expect(extractMentionedPaths("look at @src/a.ts and @docs/b.md", "/ws")).toEqual([
      "/ws/src/a.ts",
      "/ws/docs/b.md",
    ]);
  });

  it("dedupes by resolved path", () => {
    expect(extractMentionedPaths("@a.ts @a.ts", "/ws")).toEqual(["/ws/a.ts"]);
  });

  it("skips absolute mentions", () => {
    expect(extractMentionedPaths("see @/etc/hosts", "/ws")).toEqual([]);
  });

  it("returns relative mentions verbatim when no workspace is set", () => {
    expect(extractMentionedPaths("@a.ts", "")).toEqual(["a.ts"]);
  });

  it("returns nothing without mentions", () => {
    expect(extractMentionedPaths("no mentions here", "/ws")).toEqual([]);
  });
});

describe("newThoughtStep", () => {
  it("builds a ThoughtStep-shaped accumulator", () => {
    const before = Date.now();
    const step = newThoughtStep("thinking…", 3);
    expect(step.turn).toBe(3);
    expect(step.thoughtText).toBe("thinking…");
    expect(step.durationSeconds).toBe(1);
    expect(step.id).toMatch(/^go-thought-turn-3-\d+$/);
    expect(step.createdAtMs).toBeGreaterThanOrEqual(before);
    expect(step.timestamp).toMatch(/^\d{2}:\d{2}/);
  });

  it("defaults to turn 1", () => {
    expect(newThoughtStep("x").turn).toBe(1);
  });
});
