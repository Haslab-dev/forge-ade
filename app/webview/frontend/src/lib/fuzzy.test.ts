import { describe, expect, it } from "vitest";
import { scoreFilePath, scoreText } from "./fuzzy";

describe("scoreText", () => {
  it("returns a neutral match for an empty or whitespace query", () => {
    expect(scoreText("", "anything")).toEqual({ score: 0, positions: [] });
    expect(scoreText("   ", "anything")).toEqual({ score: 0, positions: [] });
  });

  it("returns null when the query is not a subsequence", () => {
    expect(scoreText("zz", "hello")).toBeNull();
    expect(scoreText("oz", "hello")).toBeNull();
  });

  it("matches case-insensitively with ascending positions", () => {
    const m = scoreText("AB", "abc");
    expect(m).not.toBeNull();
    expect(m!.positions).toEqual([0, 1]);
    expect(m!.score).toBeGreaterThan(0);
  });

  it("rewards consecutive matches over gapped ones", () => {
    const consecutive = scoreText("ab", "ab")!;
    const gapped = scoreText("ab", "aXb")!;
    expect(consecutive.score).toBeGreaterThan(gapped.score);
    // Greedy first-match: "ab" in "aXb" matches at 0 and 2, not 2..3.
    expect(gapped.positions).toEqual([0, 2]);
  });

  it("scores an exact 2-char match at base+boundary+consecutive = 35.9", () => {
    // 16 base + 12 (idx 0 boundary) + 8 (consecutive) - 2*0.05 (length).
    expect(scoreText("ab", "ab")!.score).toBeCloseTo(35.9, 10);
  });

  it("rewards camelCase hump boundaries", () => {
    const m = scoreText("cp", "CommandPalette")!;
    expect(m.positions).toEqual([0, 7]);
    // 16 + 12 (idx0) - 7.2 (gap of 6 * 1.2) + 12 (hump) - 14*0.05 (length).
    expect(m.score).toBeCloseTo(32.1, 10);
  });

  it("treats a digit before an uppercase letter as a hump boundary (quirk)", () => {
    // isCamelHump uses prev === prev.toLowerCase(), so digits count as
    // lowercase and "9B" grants the boundary bonus while "9b" does not.
    const hump = scoreText("b", "a9B")!;
    const plain = scoreText("b", "a9b")!;
    expect(hump.score - plain.score).toBeCloseTo(12, 10);
  });

  it("rewards separator boundaries", () => {
    const m = scoreText("read", "docs/readme.md")!;
    expect(m.positions).toEqual([5, 6, 7, 8]);
    expect(m.score).toBeGreaterThan(0);
  });
});

describe("scoreFilePath", () => {
  it("returns a neutral match for an empty query", () => {
    expect(scoreFilePath("", "src/app.ts")).toEqual({ score: 0, positions: [] });
  });

  it("returns null when nothing matches", () => {
    expect(scoreFilePath("qqq", "src/a.ts")).toBeNull();
    expect(scoreFilePath("a", "")).toBeNull();
  });

  it("adds the basename bonus when the query matches the basename", () => {
    const m = scoreFilePath("palette", "src/components/CommandPaletteModal.tsx")!;
    expect(m.score).toBeGreaterThan(100); // base score + BASENAME_BONUS(60)
    // First match is the 'P' of "CommandPaletteModal", 15 (dirname) + 7.
    expect(m.positions[0]).toBe(22);
    expect(m.positions).toEqual([...m.positions].sort((a, b) => a - b));
  });

  it("falls back to the full path when the basename cannot match", () => {
    const m = scoreFilePath("docs", "docs/readme.md")!;
    expect(m.positions).toEqual([0, 1, 2, 3]);
    // 16 + 3*8 (consecutive) + 12 (idx0 boundary) - 14*0.05.
    expect(m.score).toBeCloseTo(51.3, 10);
  });

  it("prefers a basename match over a path match for the same query", () => {
    const basenameHit = scoreFilePath("modal", "src/Modal.tsx")!;
    const pathMatch = scoreFilePath("modal", "modal-notes/zzz.md")!;
    expect(basenameHit.score).toBeGreaterThan(pathMatch.score);
  });

  it("handles windows separators", () => {
    const m = scoreFilePath("app", "src\\app.ts")!;
    expect(m).not.toBeNull();
    expect(m!.positions[0]).toBe(4); // basename "app.ts" starts after the backslash
  });
});
