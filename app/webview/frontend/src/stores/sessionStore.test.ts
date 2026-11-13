import { describe, expect, it } from "vitest";
import { DEFAULT_APP_SETTINGS, sessionStatusLabel, useSessionStore } from "./sessionStore";
import { DEFAULT_TERMINAL_SETTINGS } from "../lib/terminalSettings";
import type { TerminalSessionRecord } from "../types";

function rec(over: Partial<TerminalSessionRecord> = {}): TerminalSessionRecord {
  return {
    id: "s1",
    agentId: "omp",
    agentName: "omp",
    title: "Test session",
    workingDirectory: "/tmp",
    executable: "/bin/zsh",
    createdAt: 0,
    status: "running",
    ...over,
  };
}

describe("sessionStatusLabel", () => {
  it("maps running/starting to 'Running'", () => {
    expect(sessionStatusLabel(rec({ status: "running" }))).toBe("Running");
    expect(sessionStatusLabel(rec({ status: "starting" }))).toBe("Running");
  });

  it("maps failed to 'Failed'", () => {
    expect(sessionStatusLabel(rec({ status: "failed" }))).toBe("Failed");
  });

  it("uses the plain label without endedAt", () => {
    expect(sessionStatusLabel(rec({ status: "exited", endedAt: undefined }))).toBe("Exited");
    expect(sessionStatusLabel(rec({ status: "terminated", endedAt: undefined }))).toBe(
      "Terminated"
    );
  });

  it("appends relative end time", () => {
    const now = Date.now();
    expect(sessionStatusLabel(rec({ status: "exited", endedAt: now - 10_000 }))).toBe(
      "Exited · now"
    );
    expect(sessionStatusLabel(rec({ status: "exited", endedAt: now - 5 * 60_000 }))).toBe(
      "Exited · 5m"
    );
    expect(sessionStatusLabel(rec({ status: "exited", endedAt: now - 2 * 3_600_000 }))).toBe(
      "Exited · 2h"
    );
    expect(sessionStatusLabel(rec({ status: "exited", endedAt: now - 48 * 3_600_000 }))).toBe(
      "Exited · 2d"
    );
  });
});

describe("DEFAULT_APP_SETTINGS", () => {
  it("defaults to terminal mode with the shared terminal settings", () => {
    expect(DEFAULT_APP_SETTINGS.defaultMode).toBe("terminal");
    expect(DEFAULT_APP_SETTINGS.terminal).toEqual(DEFAULT_TERMINAL_SETTINGS);
  });
});

describe("useSessionStore (initial state)", () => {
  it("starts empty and unloaded", () => {
    const state = useSessionStore.getState();
    expect(state.sessions).toEqual([]);
    expect(state.loaded).toBe(false);
    expect(state.activeSessionId === null || typeof state.activeSessionId === "string").toBe(true);
    expect(state.isNewTaskOpen).toBe(false);
    expect(state.lastCreateError).toBe("");
  });

  it("openNewTask/closeNewTask toggles the modal state", () => {
    const { openNewTask, closeNewTask } = useSessionStore.getState();
    openNewTask("/ws");
    expect(useSessionStore.getState().isNewTaskOpen).toBe(true);
    expect(useSessionStore.getState().newTaskWorkspacePath).toBe("/ws");
    closeNewTask();
    expect(useSessionStore.getState().isNewTaskOpen).toBe(false);
    expect(useSessionStore.getState().newTaskWorkspacePath).toBeUndefined();
  });
});
