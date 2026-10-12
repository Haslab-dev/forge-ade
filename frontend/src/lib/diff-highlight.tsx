import React from "react";

/**
 * Lightweight per-line syntax tinting for diff viewers — ZCode-style palette:
 * keywords violet, strings orange, numbers sky, comments muted. Deliberately
 * stateless per line (multi-line constructs degrade gracefully).
 */

const CLIKE_KEYWORDS = new Set(
  ("const let var function return if else for while do import from export default class extends new await async " +
    "interface type enum implements public private protected static readonly package func struct map chan go defer " +
    "switch case break continue try catch finally throw typeof instanceof in of this super null undefined nil true false " +
    "string int int64 bool float float64 byte rune error any uintptr select range defer goto fallthrough defer")
    .split(" ")
    .filter(Boolean)
);

const HASH_KEYWORDS = new Set(
  "def class return if elif else for while import from as with try except finally raise lambda pass break continue and or not in is None True False self echo then fi esac do done function local export".split(
    " "
  )
);

const COLOR = {
  comment: "text-foreground-subtle/70 italic",
  string: "text-orange-300/90",
  number: "text-sky-300/90",
  keyword: "text-violet-400/90",
  prop: "text-sky-300/80"
};

function pushText(out: React.ReactNode[], text: string) {
  if (text) out.push(<span key={out.length}>{text}</span>);
}

function pushTok(out: React.ReactNode[], text: string, cls: string) {
  if (text) out.push(
    <span key={out.length} className={cls}>
      {text}
    </span>
  );
}

/** Generic scanner: alternates plain text with matched token classes. */
function scan(text: string, re: RegExp): React.ReactNode[] {
  const out: React.ReactNode[] = [];
  let last = 0;
  re.lastIndex = 0;
  let m: RegExpExecArray | null;
  while ((m = re.exec(text)) !== null) {
    pushText(out, text.slice(last, m.index));
    const [full, comment, str, num, kw] = m;
    if (comment) pushTok(out, full, COLOR.comment);
    else if (str) pushTok(out, full, COLOR.string);
    else if (num) pushTok(out, full, COLOR.number);
    else if (kw) pushTok(out, full, COLOR.keyword);
    else pushText(out, full);
    last = m.index + full.length;
    if (full.length === 0) re.lastIndex++;
  }
  pushText(out, text.slice(last));
  return out;
}

function renderCLike(text: string): React.ReactNode[] {
  const kw = Array.from(CLIKE_KEYWORDS).join("|");
  return scan(
    text,
    new RegExp(
      `(\\/\\/.*$|#.*$|\\/\\*.*?(?:\\*\\/|$))|("(?:[^"\\\\]|\\\\.)*"|'(?:[^'\\\\]|\\\\.)*'|\`(?:[^\`\\\\]|\\\\.)*\`)|\\b(\\d+(?:\\.\\d+)?)\\b|\\b(${kw})\\b`,
      "g"
    )
  );
}

function renderHash(text: string): React.ReactNode[] {
  const kw = Array.from(HASH_KEYWORDS).join("|");
  return scan(
    text,
    new RegExp(
      `(#.*$)|("(?:[^"\\\\]|\\\\.)*"|'(?:[^'\\\\]|\\\\.)*')|\\b(\\d+(?:\\.\\d+)?)\\b|\\b(${kw})\\b`,
      "g"
    )
  );
}

function renderJson(text: string): React.ReactNode[] {
  const out: React.ReactNode[] = [];
  const re = /("(?:[^"\\]|\\.)*")(\s*:)?|(-?\d+(?:\.\d+)?)|\b(true|false|null)\b/g;
  let last = 0;
  let m: RegExpExecArray | null;
  while ((m = re.exec(text)) !== null) {
    pushText(out, text.slice(last, m.index));
    if (m[1] && m[2]) {
      // object key
      pushTok(out, m[1], COLOR.prop);
      pushText(out, m[2]);
    } else if (m[1]) {
      pushTok(out, m[1], COLOR.string);
    } else if (m[3]) {
      pushTok(out, m[3], COLOR.number);
    } else {
      pushTok(out, m[0], COLOR.keyword);
    }
    last = m.index + m[0].length;
  }
  pushText(out, text.slice(last));
  return out;
}

function renderCss(text: string): React.ReactNode[] {
  const out: React.ReactNode[] = [];
  const trimmed = text.trimStart();
  // Comment lines
  if (/^\s*\/\*/.test(text)) {
    pushTok(out, text, COLOR.comment);
    return out;
  }
  // At-rules and selectors: keep neutral, tint at-rule keyword
  const atRule = trimmed.startsWith("@");
  const isDecl = /^\s*[\w-]+\s*:/.test(text) && !trimmed.endsWith("{");
  if (atRule) {
    const m = text.match(/^(\s*)(@[\w-]+)/);
    if (m) {
      pushText(out, m[1]);
      pushTok(out, m[2], COLOR.keyword);
      pushText(out, text.slice(m[0].length));
      return out;
    }
  }
  if (isDecl) {
    const m = text.match(/^(\s*)([\w-]+)(\s*:\s*)(.*)$/);
    if (m) {
      pushText(out, m[1]);
      pushTok(out, m[2], COLOR.prop);
      pushText(out, m[3]);
      // Value: tint hex colors and numbers/units, strings too
      const vRe = /(#[0-9a-fA-F]{3,8}\b)|(-?\d+(?:\.\d+)?[a-z%]*)|("[^"]*"|'[^']*')/g;
      let last = 0;
      let vm: RegExpExecArray | null;
      while ((vm = vRe.exec(m[4])) !== null) {
        pushText(out, m[4].slice(last, vm.index));
        if (vm[1]) pushTok(out, vm[1], COLOR.number);
        else if (vm[2]) pushTok(out, vm[2], COLOR.number);
        else pushTok(out, vm[3], COLOR.string);
        last = vm.index + vm[0].length;
      }
      pushText(out, m[4].slice(last));
      return out;
    }
  }
  pushText(out, text);
  return out;
}

function renderMd(text: string): React.ReactNode[] {
  const heading = text.match(/^(\s*)(#{1,6}\s)(.*)$/);
  if (heading) {
    return [
      <span key="h" className="text-violet-400/90 font-semibold">
        {text}
      </span>
    ];
  }
  return scan(
    text,
    /(`[^`]*`)|(\*\*[^*]+\*\*)/g,
  );
}

export function highlightCodeLine(text: string, ext?: string): React.ReactNode {
  if (!text) return text;
  const e = (ext || "").toLowerCase();
  try {
    if (e === "css" || e === "scss" || e === "less") return renderCss(text);
    if (e === "json") return renderJson(text);
    if (e === "md" || e === "markdown") return renderMd(text);
    if (["py", "sh", "bash", "zsh", "yaml", "yml", "toml"].includes(e)) return renderHash(text);
    return renderCLike(text);
  } catch {
    return text;
  }
}

const ANSI_COLOR_MAP: Record<number, string> = {
  30: "text-foreground-subtle",
  31: "text-destructive",
  32: "text-success",
  33: "text-warning",
  34: "text-primary",
  35: "text-violet-400",
  36: "text-sky-300",
  37: "text-foreground",
  90: "text-foreground-subtle",
  91: "text-destructive",
  92: "text-success",
  93: "text-warning",
  94: "text-primary",
  95: "text-violet-400",
  96: "text-sky-300",
  97: "text-foreground",
};

/**
 * Parses ANSI escape sequences into React elements with Tailwind color/style classes.
 */
function renderAnsiLine(text: string): React.ReactNode[] {
  const parts: React.ReactNode[] = [];
  const ansiRegex = /\x1b\[([0-9;]*)m/g;
  let lastIndex = 0;
  let currentClasses = new Set<string>();
  let match: RegExpExecArray | null;

  while ((match = ansiRegex.exec(text)) !== null) {
    if (match.index > lastIndex) {
      const chunk = text.slice(lastIndex, match.index);
      const cls = Array.from(currentClasses).join(" ");
      parts.push(
        cls ? (
          <span key={parts.length} className={cls}>
            {chunk}
          </span>
        ) : (
          <span key={parts.length}>{chunk}</span>
        )
      );
    }

    const codeStr = match[1] || "0";
    const codes = codeStr.split(";").map((c) => parseInt(c, 10) || 0);

    for (const code of codes) {
      if (code === 0) {
        currentClasses.clear();
      } else if (code === 1) {
        currentClasses.add("font-bold");
      } else if (code === 2) {
        currentClasses.add("opacity-70");
      } else if (code === 3) {
        currentClasses.add("italic");
      } else if (code === 4) {
        currentClasses.add("underline");
      } else if (ANSI_COLOR_MAP[code]) {
        // Clear previous fg colors
        for (const c of Array.from(currentClasses)) {
          if (c.startsWith("text-")) currentClasses.delete(c);
        }
        currentClasses.add(ANSI_COLOR_MAP[code]);
      }
    }

    lastIndex = match.index + match[0].length;
  }

  if (lastIndex < text.length) {
    const chunk = text.slice(lastIndex);
    const cls = Array.from(currentClasses).join(" ");
    parts.push(
      cls ? (
        <span key={parts.length} className={cls}>
          {chunk}
        </span>
      ) : (
        <span key={parts.length}>{chunk}</span>
      )
    );
  }

  return parts;
}

/**
 * Highlights a shell/terminal output line with log levels, errors, paths, URLs, flags, and numbers.
 */
export function highlightTerminalLine(text: string): React.ReactNode {
  if (!text) return text;

  // 1. If ANSI escape sequences exist, parse them directly
  if (/\x1b\[[0-9;]*m/.test(text)) {
    return <>{renderAnsiLine(text)}</>;
  }

  // 2. High-signal semantic regex scan
  const re = /(https?:\/\/[^\s]+|(?:\/[\w.-]+)+\b|(?:\.\.?\/[\w.-]+)+(?::\d+(?::\d+)?)?)|(\b(?:ERROR|FATAL|FAIL|FAILED|FAILURE|Exception|Err!|error)\b)|(\b(?:WARN|WARNING|WARN!|warning|caution)\b)|(\b(?:SUCCESS|PASSED|PASS|Done|done|compiled)\b|[✓✔])|(--?[a-zA-Z0-9_-]+(?:=[^\s]+)?)|("(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*')|(\b\d+(?:\.\d+)?(?:ms|s|m|h|b|kb|mb|gb|%)?\b)/g;

  const out: React.ReactNode[] = [];
  let last = 0;
  let m: RegExpExecArray | null;

  while ((m = re.exec(text)) !== null) {
    if (m.index > last) {
      out.push(<span key={out.length}>{text.slice(last, m.index)}</span>);
    }
    const [full, urlOrPath, err, warn, succ, flag, str, num] = m;
    if (err) {
      pushTok(out, full, "text-destructive font-semibold");
    } else if (warn) {
      pushTok(out, full, "text-warning font-semibold");
    } else if (succ) {
      pushTok(out, full, "text-success font-semibold");
    } else if (urlOrPath) {
      pushTok(out, full, "text-sky-300 underline underline-offset-2 decoration-sky-300/40");
    } else if (flag) {
      pushTok(out, full, "text-violet-400 font-medium");
    } else if (str) {
      pushTok(out, full, COLOR.string);
    } else if (num) {
      pushTok(out, full, COLOR.number);
    } else {
      pushText(out, full);
    }
    last = m.index + full.length;
  }

  if (last < text.length) {
    out.push(<span key={out.length}>{text.slice(last)}</span>);
  }

  return out.length > 0 ? <>{out}</> : text;
}

/**
 * Highlights a shell command invocation like `$ bun test --coverage`
 */
export function highlightTerminalCommand(cmd: string): React.ReactNode {
  if (!cmd) return "";
  const trimmed = cmd.trim();
  const cleanCmd = trimmed.startsWith("$ ") ? trimmed.slice(2) : trimmed;

  const parts = cleanCmd.split(/(\s+)/);
  let isFirstToken = true;

  return (
    <>
      <span className="text-foreground-subtle select-none font-bold mr-1.5">$</span>
      {parts.map((p, idx) => {
        if (!p) return null;
        if (/^\s+$/.test(p)) return <span key={idx}>{p}</span>;

        if (isFirstToken) {
          isFirstToken = false;
          return (
            <span key={idx} className="text-foreground font-semibold">
              {p}
            </span>
          );
        }

        if (p.startsWith("-")) {
          return (
            <span key={idx} className="text-violet-400 font-medium">
              {p}
            </span>
          );
        }

        if (p.startsWith('"') || p.startsWith("'")) {
          return (
            <span key={idx} className="text-orange-300">
              {p}
            </span>
          );
        }

        if (p.includes("/") || p.includes(".")) {
          return (
            <span key={idx} className="text-sky-300">
              {p}
            </span>
          );
        }

        return (
          <span key={idx} className="text-foreground-secondary">
            {p}
          </span>
        );
      })}
    </>
  );
}
