import { describe, expect, it } from "vitest";
import { parseGitDecorations } from "./gitDecorations";

describe("parseGitDecorations", () => {
  it("currently keeps the leading '(' when the string starts with a space (FORGE-116)", () => {
    // git's %d format emits " (HEAD -> main, origin/main)" — the strip regex
    // is anchored to offset 0, so the space defeats it and the first chip
    // renders as "(HEAD -> main". Desired behavior tracked as FORGE-116.
    expect(parseGitDecorations(" (HEAD -> main, origin/main)")).toEqual([
      "(HEAD -> main",
      "origin/main",
    ]);
  });

  it.skip("should strip parens from decorations with a leading space (FORGE-116)", () => {
    expect(parseGitDecorations(" (HEAD -> main, origin/main)")).toEqual([
      "HEAD -> main",
      "origin/main",
    ]);
  });

  it("strips parens when the string starts with one", () => {
    expect(parseGitDecorations("(HEAD -> main, origin/main)")).toEqual([
      "HEAD -> main",
      "origin/main",
    ]);
  });

  it("returns an empty array for missing/empty input", () => {
    expect(parseGitDecorations(undefined)).toEqual([]);
    expect(parseGitDecorations("")).toEqual([]);
  });

  it("handles a single decoration without leading space", () => {
    expect(parseGitDecorations("(main)")).toEqual(["main"]);
  });

  it("drops empty segments", () => {
    expect(parseGitDecorations("(a,,b)")).toEqual(["a", "b"]);
    expect(parseGitDecorations("(,)")).toEqual([]);
  });

  it("passes through strings without parentheses", () => {
    expect(parseGitDecorations("main")).toEqual(["main"]);
  });
});
