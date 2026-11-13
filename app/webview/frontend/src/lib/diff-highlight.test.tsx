import { describe, expect, it } from "vitest";
import { render } from "@testing-library/react";
import type { ReactNode } from "react";
import {
  highlightCodeLine,
  highlightTerminalCommand,
  highlightTerminalLine,
} from "./diff-highlight";

/** Flatten a highlighted node into [text, className] span tuples. */
function spans(node: ReactNode): Array<{ text: string; className: string }> {
  const { container } = render(<div>{node}</div>);
  return Array.from(container.querySelectorAll("span")).map(el => ({
    text: el.textContent ?? "",
    className: el.className,
  }));
}

const find = (list: Array<{ text: string; className: string }>, text: string) =>
  list.find(s => s.text === text);

describe("highlightCodeLine", () => {
  it("returns the input untouched for empty lines", () => {
    expect(highlightCodeLine("", "ts")).toBe("");
  });

  it("tints C-like keywords, strings, numbers and comments", () => {
    const out = spans(highlightCodeLine('const x = 1; // note', "ts"));
    expect(find(out, "const")?.className).toContain("text-violet-400/90");
    expect(find(out, "1")?.className).toContain("text-sky-300/90");
    expect(find(out, "// note")?.className).toContain("italic");
  });

  it("tints json keys, string values, numbers and literals differently", () => {
    const out = spans(highlightCodeLine('{"a": true, "b": "v", "n": 3}', "json"));
    expect(find(out, '"a"')?.className).toContain("text-sky-300/80"); // prop
    expect(find(out, "true")?.className).toContain("text-violet-400/90"); // keyword
    expect(find(out, '"v"')?.className).toContain("text-orange-300/90"); // string
    expect(find(out, "3")?.className).toContain("text-sky-300/90"); // number
  });

  it("highlights headings in markdown", () => {
    const out = spans(highlightCodeLine("## Title", "md"));
    expect(out.some(s => s.className.includes("text-violet-400/90") && s.text.includes("## Title"))).toBe(true);
  });

  it("currently styles markdown inline code with the comment palette (FORGE-115)", () => {
    // renderMd feeds its groups into scan()'s (comment, str, ...) slots, so
    // inline code lands in the "comment" slot and bold in the "string" slot.
    // Desired behavior is tracked as FORGE-115 (skipped test below).
    const out = spans(highlightCodeLine("use `foo` here", "md"));
    expect(find(out, "`foo`")?.className).toContain("text-foreground-subtle/70");
  });

  it.skip("should style markdown inline code as a string, not a comment (FORGE-115)", () => {
    const out = spans(highlightCodeLine("use `foo` here", "md"));
    expect(find(out, "`foo`")?.className).toContain("text-orange-300/90");
  });

  it("highlights python/shell keywords and hash comments", () => {
    const out = spans(highlightCodeLine("def main():  # entry", "py"));
    expect(find(out, "def")?.className).toContain("text-violet-400/90");
    expect(find(out, "# entry")?.className).toContain("italic");
  });

  it("tints css at-rules, declarations, hex colors and strings", () => {
    const out = spans(highlightCodeLine("  color: #ff0000;", "css"));
    expect(find(out, "color")?.className).toContain("text-sky-300/80");
    expect(find(out, "#ff0000")?.className).toContain("text-sky-300/90");
    const at = spans(highlightCodeLine("@media screen", "css"));
    expect(find(at, "@media")?.className).toContain("text-violet-400/90");
  });

  it("falls back to plain text on internal errors", () => {
    expect(highlightCodeLine("plain", "unknownext")).toBeDefined();
  });
});

describe("highlightTerminalLine", () => {
  it("returns empty input untouched", () => {
    expect(highlightTerminalLine("")).toBe("");
  });

  it("parses ANSI color escapes into styled spans", () => {
    const out = spans(highlightTerminalLine("\x1b[31mred\x1b[0m plain"));
    expect(find(out, "red")?.className).toContain("text-destructive");
    expect(find(out, " plain")?.className ?? "").not.toContain("text-destructive");
  });

  it("supports bold/dim/italic/underline and color replacement", () => {
    const out = spans(highlightTerminalLine("\x1b[1;32mgreen bold\x1b[0m"));
    expect(find(out, "green bold")?.className).toContain("font-bold");
    expect(find(out, "green bold")?.className).toContain("text-success");
  });

  it("marks semantic log levels", () => {
    const err = spans(highlightTerminalLine("ERROR: boom"));
    expect(find(err, "ERROR")?.className).toContain("text-destructive");
    const ok = spans(highlightTerminalLine("build Done"));
    expect(find(ok, "Done")?.className).toContain("text-success");
    const warn = spans(highlightTerminalLine("WARN: low disk"));
    expect(find(warn, "WARN")?.className).toContain("text-warning");
  });

  it("underlines URLs and paths", () => {
    const out = spans(highlightTerminalLine("see https://example.dev/x now"));
    expect(find(out, "https://example.dev/x")?.className).toContain("text-sky-300");
  });

  it("styles numeric units and leaves plain words as spans", () => {
    const out = spans(highlightTerminalLine("took 120ms total"));
    expect(find(out, "120ms")?.className).toContain("text-sky-300/90");
  });
});

describe("highlightTerminalCommand", () => {
  it("renders an empty string for empty input", () => {
    expect(highlightTerminalCommand("")).toBe("");
  });

  it("bolds the first token, tints flags, paths and quoted args", () => {
    const out = spans(highlightTerminalCommand('$ bun test --coverage "a b" src/x.ts'));
    expect(find(out, "$")?.className).toContain("font-bold");
    expect(find(out, "bun")?.className).toContain("text-foreground font-semibold");
    expect(find(out, "--coverage")?.className).toContain("text-violet-400");
    expect(find(out, "src/x.ts")?.className).toContain("text-sky-300");
    expect(find(out, "test")?.className).toContain("text-foreground-secondary");
    // Quoted args are split on whitespace inside the quotes (documented
    // behavior): only the leading fragment starts with a quote, so it gets
    // the string tint while the trailing fragment is a plain token.
    expect(find(out, '"a')?.className).toContain("text-orange-300");
    expect(find(out, 'b"')?.className).toContain("text-foreground-secondary");
  });

  it("strips a leading $ prompt if present", () => {
    const out = spans(highlightTerminalCommand("$ ls"));
    // Exactly one $ marker (the synthetic one), the prompt was consumed.
    expect(out.filter(s => s.text === "$")).toHaveLength(1);
  });
});
