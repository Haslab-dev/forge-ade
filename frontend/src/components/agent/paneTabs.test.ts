import { describe, expect, it } from "vitest";
import {
  findTerminalToFocus,
  hasBuiltinSideChat,
  mirrorFileTabs,
  nextActiveAfterClose,
  closeTabsLeft,
  closeTabsRight,
  closeTabsOthers,
  nextActiveAfterBulkClose,
  nextConversationTitle,
  nextTerminalTitle,
  upsertSingleton,
  type PaneTab,
} from "./paneTabs";

const tab = (id: string, kind: PaneTab["kind"] = "terminal", extra: Partial<PaneTab> = {}): PaneTab => ({
  id,
  kind,
  title: id,
  ...extra,
});

describe("upsertSingleton", () => {
  it("appends to an empty list", () => {
    const result = upsertSingleton([], tab("browser", "browser", { title: "Browser" }));
    expect(result.map(t => t.id)).toEqual(["browser"]);
  });

  it("appends after existing tabs, preserving order", () => {
    const prev = [tab("browser", "browser"), tab("term-1")];
    const result = upsertSingleton(prev, tab("sideChat", "chat"));
    expect(result.map(t => t.id)).toEqual(["browser", "term-1", "sideChat"]);
  });

  it("is a no-op (same reference) when the id already exists", () => {
    const prev = [tab("browser", "browser")];
    expect(upsertSingleton(prev, tab("browser", "browser", { title: "Different" }))).toBe(prev);
  });

  it("keeps the original entry when a duplicate id arrives with new content", () => {
    const prev = [tab("browser", "browser", { title: "Browser" })];
    const result = upsertSingleton(prev, tab("browser", "browser", { title: "Changed" }));
    expect(result[0].title).toBe("Browser");
  });

  it("does not mutate the input array", () => {
    const prev = [tab("a")];
    upsertSingleton(prev, tab("b"));
    expect(prev.map(t => t.id)).toEqual(["a"]);
  });
});

describe("mirrorFileTabs", () => {
  const file = (path: string, name = path) => ({ path, name });

  it("maps store rows to file:* tabs with name as title", () => {
    // The store pre-computes `name` (basename); mirror uses it verbatim.
    const result = mirrorFileTabs([], [{ path: "src/a.ts", name: "a.ts" }, file("README.md")]);
    expect(result).toEqual([
      { id: "file:src/a.ts", kind: "file", title: "a.ts", path: "src/a.ts" },
      { id: "file:README.md", kind: "file", title: "README.md", path: "README.md" },
    ]);
  });

  it("orders non-file tabs first and file tabs last", () => {
    const prev = [tab("browser", "browser"), tab("file:old.ts", "file", { path: "old.ts" })];
    const result = mirrorFileTabs(prev, [file("new.ts")]);
    expect(result.map(t => t.id)).toEqual(["browser", "file:new.ts"]);
  });

  it("adds newly opened files without disturbing non-file tabs", () => {
    const prev = [tab("browser", "browser"), tab("file:a.ts", "file", { path: "a.ts" })];
    const result = mirrorFileTabs(prev, [file("a.ts"), file("b.ts")]);
    expect(result.map(t => t.id)).toEqual(["browser", "file:a.ts", "file:b.ts"]);
  });

  it("drops file tabs removed from the store", () => {
    const prev = [tab("file:a.ts", "file", { path: "a.ts" }), tab("file:b.ts", "file", { path: "b.ts" })];
    const result = mirrorFileTabs(prev, [file("b.ts")]);
    expect(result.map(t => t.id)).toEqual(["file:b.ts"]);
  });

  it("reorders file tabs when the store order changes", () => {
    const prev = [tab("file:a.ts", "file", { path: "a.ts" }), tab("file:b.ts", "file", { path: "b.ts" })];
    const result = mirrorFileTabs(prev, [file("b.ts"), file("a.ts")]);
    expect(result.map(t => t.id)).toEqual(["file:b.ts", "file:a.ts"]);
  });

  it("returns the same reference when nothing changed (React bail-out)", () => {
    const prev = [tab("browser", "browser"), tab("file:a.ts", "file", { path: "a.ts" })];
    expect(mirrorFileTabs(prev, [file("a.ts")])).toBe(prev);
    expect(mirrorFileTabs([], [])).toHaveLength(0);
  });

  it("clears all file tabs when the store list empties, keeping others", () => {
    const prev = [tab("browser", "browser"), tab("file:a.ts", "file", { path: "a.ts" })];
    const result = mirrorFileTabs(prev, []);
    expect(result.map(t => t.id)).toEqual(["browser"]);
  });

  it("normalizes interleaved file tabs to non-file-first ordering", () => {
    const prev = [tab("file:a.ts", "file", { path: "a.ts" }), tab("browser", "browser")];
    const result = mirrorFileTabs(prev, [file("a.ts")]);
    expect(result.map(t => t.id)).toEqual(["browser", "file:a.ts"]);
  });
});

describe("nextActiveAfterClose", () => {
  const remaining = [tab("browser", "browser"), tab("term-1")];

  it("keeps the selection when a background tab is closed", () => {
    expect(nextActiveAfterClose(remaining, "some-other", "term-1")).toEqual({
      activeTab: "term-1",
      tabOpen: true,
    });
  });

  it("selects the LAST remaining tab when the active tab is closed", () => {
    // The component passes the already-filtered list, mirror of closePaneTab.
    const rest = remaining.filter(t => t.id !== "term-1");
    expect(nextActiveAfterClose(rest, "term-1", "term-1")).toEqual({
      activeTab: "browser",
      tabOpen: true,
    });
    // closing the first of many selects the new last
    const many = [tab("a"), tab("b"), tab("c")];
    expect(nextActiveAfterClose(many.filter(t => t.id !== "a"), "a", "a").activeTab).toBe("c");
  });

  it("closes the pane (null/false) when the last tab is closed", () => {
    expect(nextActiveAfterClose([], "browser", "browser")).toEqual({
      activeTab: null,
      tabOpen: false,
    });
  });

  it("stays closed when there was no selection", () => {
    expect(nextActiveAfterClose(remaining, "browser", null)).toEqual({
      activeTab: null,
      tabOpen: false,
    });
  });

  it("applies the rule even when closedId is already gone from remaining", () => {
    expect(nextActiveAfterClose([], "ghost", "ghost")).toEqual({ activeTab: null, tabOpen: false });
  });
});

describe("nextTerminalTitle", () => {
  it("numbers from existing terminal tabs only", () => {
    const tabs = [tab("browser", "browser"), tab("term-1", "terminal"), tab("sideChat", "chat")];
    expect(nextTerminalTitle(tabs)).toBe("zsh 2");
    expect(nextTerminalTitle([])).toBe("zsh 1");
  });

  it("derives 'Task: <first word>' from a command", () => {
    expect(nextTerminalTitle([], "git status --porcelain")).toBe("Task: git");
    expect(nextTerminalTitle([], "  bun   test ")).toBe("Task: bun");
  });

  it("falls back to numbering for empty/whitespace commands", () => {
    expect(nextTerminalTitle([tab("t1", "terminal")], "")).toBe("zsh 2");
    expect(nextTerminalTitle([], "   ")).toBe("zsh 1");
  });
});

describe("hasBuiltinSideChat / nextConversationTitle", () => {
  it("detects the builtin side-chat singleton anywhere in the list", () => {
    expect(hasBuiltinSideChat([])).toBe(false);
    expect(hasBuiltinSideChat([tab("browser", "browser")])).toBe(false);
    expect(hasBuiltinSideChat([tab("browser", "browser"), tab("sideChat", "chat")])).toBe(true);
  });

  it("numbers additional conversations after the builtin side chat", () => {
    expect(nextConversationTitle([])).toBe("Conversation 1");
    expect(nextConversationTitle([tab("sideChat", "chat")])).toBe("Conversation 2");
    expect(nextConversationTitle([tab("sideChat", "chat"), tab("chat-9", "chat")])).toBe("Conversation 3");
  });
});

describe("findTerminalToFocus", () => {
  const tabs = [tab("term-1", "terminal", { taskId: "task-a" }), tab("term-2", "terminal")];

  it("focuses by taskId when given", () => {
    expect(findTerminalToFocus(tabs, { taskId: "task-a", focusExisting: false })?.id).toBe("term-1");
  });

  it("returns undefined when the taskId has no tab (spawn a new one)", () => {
    expect(findTerminalToFocus(tabs, { taskId: "task-z", focusExisting: false })).toBeUndefined();
  });

  it("focuses any existing terminal under the reuse rule", () => {
    expect(findTerminalToFocus(tabs, { focusExisting: true })?.id).toBe("term-1");
    expect(findTerminalToFocus([], { focusExisting: true })).toBeUndefined();
  });

  it("taskId takes precedence over the reuse rule", () => {
    expect(findTerminalToFocus(tabs, { taskId: "task-z", focusExisting: true })).toBeUndefined();
  });

  it("returns undefined without taskId/focusExisting (always create)", () => {
    expect(findTerminalToFocus(tabs, { focusExisting: false })).toBeUndefined();
  });
});

describe("bulk close helpers", () => {
  const tabs = [tab("t1"), tab("t2"), tab("t3"), tab("t4")];

  it("closeTabsLeft removes all tabs before target", () => {
    expect(closeTabsLeft(tabs, "t3").map(t => t.id)).toEqual(["t3", "t4"]);
    expect(closeTabsLeft(tabs, "t1").map(t => t.id)).toEqual(["t1", "t2", "t3", "t4"]);
  });

  it("closeTabsRight removes all tabs after target", () => {
    expect(closeTabsRight(tabs, "t2").map(t => t.id)).toEqual(["t1", "t2"]);
    expect(closeTabsRight(tabs, "t4").map(t => t.id)).toEqual(["t1", "t2", "t3", "t4"]);
  });

  it("closeTabsOthers keeps only the target tab", () => {
    expect(closeTabsOthers(tabs, "t2").map(t => t.id)).toEqual(["t2"]);
  });

  it("nextActiveAfterBulkClose preserves current active if still present", () => {
    const res = nextActiveAfterBulkClose([tab("t2"), tab("t3")], "t2", "t3");
    expect(res).toEqual({ activeTab: "t3", tabOpen: true });
  });

  it("nextActiveAfterBulkClose falls back to target if current active was closed", () => {
    const res = nextActiveAfterBulkClose([tab("t2"), tab("t3")], "t2", "t1");
    expect(res).toEqual({ activeTab: "t2", tabOpen: true });
  });

  it("nextActiveAfterBulkClose returns closed state if remaining is empty", () => {
    const res = nextActiveAfterBulkClose([], "t1", "t1");
    expect(res).toEqual({ activeTab: null, tabOpen: false });
  });
});
