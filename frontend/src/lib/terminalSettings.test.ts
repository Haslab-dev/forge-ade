import { beforeEach, describe, expect, it, vi } from "vitest";

// terminalSettings keeps a module-level cache, so every test gets a fresh
// module instance and a clean localStorage.
beforeEach(() => {
  vi.resetModules();
  localStorage.clear();
});

async function load() {
  return await import("./terminalSettings");
}

describe("terminalSettings", () => {
  it("returns defaults when nothing is stored", async () => {
    const mod = await load();
    expect(mod.getTerminalSettings()).toEqual(mod.DEFAULT_TERMINAL_SETTINGS);
  });

  it("persists settings and notifies listeners", async () => {
    const mod = await load();
    const events: unknown[] = [];
    window.addEventListener(mod.TERMINAL_SETTINGS_EVENT, e => events.push(e));

    mod.setTerminalSettings({ ...mod.DEFAULT_TERMINAL_SETTINGS, fontSize: 20 });

    expect(events).toHaveLength(1);
    const stored = JSON.parse(localStorage.getItem("forge_ade_terminal_settings")!);
    expect(stored.fontSize).toBe(20);
    expect(mod.getTerminalSettings().fontSize).toBe(20);
  });

  it("merges a partial settings object over defaults", async () => {
    const mod = await load();
    // Simulate a partial payload written by an older version.
    localStorage.setItem(
      "forge_ade_terminal_settings",
      JSON.stringify({ fontSize: 16.5 })
    );
    const s = mod.getTerminalSettings();
    expect(s.fontSize).toBe(16.5);
    // Unspecified fields come from defaults.
    expect(s.cursorStyle).toBe(mod.DEFAULT_TERMINAL_SETTINGS.cursorStyle);
    expect(s.scrollback).toBe(mod.DEFAULT_TERMINAL_SETTINGS.scrollback);
    expect(s.fontFamily).toBe(mod.DEFAULT_TERMINAL_SETTINGS.fontFamily);
  });

  it("falls back to defaults on corrupt JSON", async () => {
    const mod = await load();
    localStorage.setItem("forge_ade_terminal_settings", "{not json");
    expect(mod.getTerminalSettings()).toEqual(mod.DEFAULT_TERMINAL_SETTINGS);
  });

  it("setTerminalSettings survives a throwing localStorage", async () => {
    const mod = await load();
    const setItem = Storage.prototype.setItem;
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new Error("quota");
    });
    expect(() =>
      mod.setTerminalSettings({ ...mod.DEFAULT_TERMINAL_SETTINGS, fontSize: 22 })
    ).not.toThrow();
    // Cache is still updated even though persistence failed.
    expect(mod.getTerminalSettings().fontSize).toBe(22);
    Storage.prototype.setItem = setItem;
  });
});
