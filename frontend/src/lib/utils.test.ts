import { describe, expect, it } from "vitest";
import { cleanPiBanner, cleanRawToolTags, formatDisplayTitle, parseFilePath } from "./utils";

describe("cleanRawToolTags", () => {
  it("returns empty for empty input", () => {
    expect(cleanRawToolTags("")).toBe("");
  });

  it("removes complete toolcall blocks (case-insensitive)", () => {
    expect(cleanRawToolTags("Hello <toolcall>{}</toolcall> world")).toBe("Hello  world");
    expect(cleanRawToolTags("A <TOOL_CALL>x</TOOL_CALL> B")).toBe("A  B");
  });

  it("removes unclosed trailing tool blocks (streaming)", () => {
    expect(cleanRawToolTags("Answer: <tool_call>{\"a\"")).toBe("Answer:");
    expect(cleanRawToolTags("Text <toolcall> more")).toBe("Text");
  });

  it("removes DSML-style calls and parameter blocks", () => {
    expect(cleanRawToolTags("pre<|DSML|calls>body</|DSML|calls>post")).toBe("prepost");
    expect(
      cleanRawToolTags('x<parameter name="file">a.ts</parameter>y')
    ).toBe("xy");
    expect(cleanRawToolTags("x<parameter>unclosed")).toBe("x");
  });

  it("leaves ordinary text (and non-tool tags) alone", () => {
    expect(cleanRawToolTags("The <b>bold</b> tag")).toBe("The <b>bold</b> tag");
    expect(cleanRawToolTags("  plain text  ")).toBe("plain text");
  });
});

describe("cleanPiBanner", () => {
  it("strips the pi version banner including skill/extension lines", () => {
    const text = [
      "pi v0.5.12",
      "---",
      "Skills",
      "- /Users/x/.pi/skills/one.md",
      "Extensions",
      "- /Users/x/.pi/ext/foo.js",
      "",
      "Hello world",
    ].join("\n");
    expect(cleanPiBanner(text)).toBe("Hello world");
  });

  it("passes text through when the banner signature is absent", () => {
    expect(cleanPiBanner("pi v1.2.3 alone")).toBe("pi v1.2.3 alone");
    expect(cleanPiBanner("Skills without pi version")).toBe("Skills without pi version");
  });

  it("keeps content that follows the banner", () => {
    expect(cleanPiBanner("pi v1.0\n---\nSkills\n\nresult: 42")).toBe("result: 42");
  });
});

describe("formatDisplayTitle", () => {
  it("returns empty for falsy input", () => {
    expect(formatDisplayTitle(undefined)).toBe("");
    expect(formatDisplayTitle(null)).toBe("");
    expect(formatDisplayTitle("")).toBe("");
  });

  it("trims wrapping quotes", () => {
    expect(formatDisplayTitle('"Fix the login flow"')).toBe("Fix the login flow");
    expect(formatDisplayTitle("'single'")).toBe("single");
  });

  it("reduces temp paths to a bare filename without extension", () => {
    expect(formatDisplayTitle("/var/folders/zz/T/scratchpad.md")).toBe("scratchpad");
    expect(formatDisplayTitle("/tmp/build-output.log")).toBe("build-output");
    expect(formatDisplayTitle("/TemporaryItems/nstimeout.xyz123")).toBe("nstimeout");
  });

  it("leaves normal titles untouched", () => {
    expect(formatDisplayTitle("src/app.ts")).toBe("src/app.ts");
    expect(formatDisplayTitle("Refactor the store")).toBe("Refactor the store");
  });
});

describe("parseFilePath", () => {
  const ws = "/ws";

  it("returns an all-empty result for non-string/empty input", () => {
    expect(parseFilePath("")).toEqual({
      raw: "",
      cleanPath: "",
      displayName: "",
      isLikelyFilePath: false,
    });
    expect((parseFilePath as (raw: unknown) => ReturnType<typeof parseFilePath>)(42)).toMatchObject({
      isLikelyFilePath: false,
    });
  });

  it("resolves relative paths against the workspace root", () => {
    const r = parseFilePath("src/app.ts", ws);
    expect(r.cleanPath).toBe("/ws/src/app.ts");
    expect(r.displayName).toBe("app.ts");
    expect(r.isLikelyFilePath).toBe(true);
  });

  it("keeps absolute paths as-is", () => {
    const r = parseFilePath("/usr/local/app.go", ws);
    expect(r.cleanPath).toBe("/usr/local/app.go");
    expect(r.displayName).toBe("app.go");
  });

  it("strips quotes, backticks and parens", () => {
    expect(parseFilePath('"src/a.ts"', ws).cleanPath).toBe("/ws/src/a.ts");
    expect(parseFilePath("`src/a.ts`", ws).cleanPath).toBe("/ws/src/a.ts");
    expect(parseFilePath("(src/a.ts)", ws).cleanPath).toBe("/ws/src/a.ts");
  });

  it("strips the file:// protocol", () => {
    expect(parseFilePath("file:///Users/x/y.ts").cleanPath).toBe("/Users/x/y.ts");
  });

  it("parses #L anchors (including ranges)", () => {
    expect(parseFilePath("src/app.ts#L10", ws)).toMatchObject({
      line: 10,
      cleanPath: "/ws/src/app.ts",
    });
    expect(parseFilePath("src/app.ts#L10-L20", ws)).toMatchObject({ line: 10 });
  });

  it("parses :line:col suffixes", () => {
    expect(parseFilePath("src/app.ts:12:5", ws)).toMatchObject({
      line: 12,
      column: 5,
      cleanPath: "/ws/src/app.ts",
    });
    expect(parseFilePath("app.go:42", ws)).toMatchObject({ line: 42 });
  });

  it("gives #L anchors precedence over :line", () => {
    expect(parseFilePath("app.ts:7#L3", ws)).toMatchObject({ line: 3 });
  });

  it("drops the line number when trailing punctuation blocks the colon match (edge)", () => {
    // The colon regex is end-anchored, so "app.ts:12." parses no line; the
    // punctuation strip then leaves "app.ts:12" as the path. Documented edge.
    const r = parseFilePath("src/app.ts:12.", ws);
    expect(r.line).toBeUndefined();
    expect(r.cleanPath).toBe("/ws/src/app.ts:12");
    expect(r.isLikelyFilePath).toBe(false);
  });

  it("strips trailing punctuation from prose references", () => {
    expect(parseFilePath("src/app.ts)", ws).cleanPath).toBe("/ws/src/app.ts");
    expect(parseFilePath("src/app.ts.", ws).cleanPath).toBe("/ws/src/app.ts");
  });

  it("normalizes windows separators to forward slashes", () => {
    expect(parseFilePath("src\\app.ts", ws).cleanPath).toBe("/ws/src/app.ts");
  });

  it("does not prefix ~ paths with the workspace root", () => {
    expect(parseFilePath("~/notes.md", ws).cleanPath).toBe("~/notes.md");
    expect(parseFilePath("~/notes.md", ws).displayName).toBe("notes.md");
  });

  it("classifies prose as not-a-file-path", () => {
    expect(parseFilePath("hello world").isLikelyFilePath).toBe(false);
    expect(parseFilePath("fix the bug").isLikelyFilePath).toBe(false);
  });

  it("classifies extension-less relative/absolute paths with slashes as likely files", () => {
    expect(parseFilePath("./scripts").isLikelyFilePath).toBe(true);
    expect(parseFilePath("~/x").isLikelyFilePath).toBe(true); // any ~/… longer than 2 chars
    expect(parseFilePath("/abs/dir/", ws).isLikelyFilePath).toBe(true);
    expect(parseFilePath("x/y").isLikelyFilePath).toBe(false); // slash but no ext/prefix
    expect(parseFilePath("~/").isLikelyFilePath).toBe(false); // too short
  });

  it("resolves ./ prefixed paths", () => {
    expect(parseFilePath("./app.ts", ws).cleanPath).toBe("/ws/app.ts");
  });

  it("keeps ../ paths under the root without collapsing (edge)", () => {
    // Only a leading "./" is stripped, so "../lib/x.ts" is joined verbatim.
    expect(parseFilePath("../lib/x.ts", ws).cleanPath).toBe("/ws/../lib/x.ts");
  });

  it("falls back to the raw path as displayName when there is no basename", () => {
    expect(parseFilePath("/").displayName).toBe("/");
  });
});
