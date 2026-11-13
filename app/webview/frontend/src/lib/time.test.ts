import { describe, expect, it } from "vitest";
import { formatRelativeTime, formatSessionTime, getSessionEpoch } from "./time";

const NOW = Date.now();
const minsAgo = (m: number) => NOW - m * 60_000;

describe("formatRelativeTime", () => {
  it("returns 'just now' for falsy input", () => {
    expect(formatRelativeTime(undefined)).toBe("just now");
    expect(formatRelativeTime(null)).toBe("just now");
    expect(formatRelativeTime("")).toBe("just now");
    expect(formatRelativeTime(0)).toBe("just now");
  });

  it("returns 'just now' for anything under a minute old", () => {
    expect(formatRelativeTime(new Date(NOW - 5_000))).toBe("just now");
    expect(formatRelativeTime(minsAgo(0.9))).toBe("just now");
  });

  it("formats minutes, hours and days", () => {
    expect(formatRelativeTime(new Date(NOW - 61_000).toISOString())).toBe("1m");
    expect(formatRelativeTime(minsAgo(2).toString())).toMatch(/^\d+m$/);
    expect(formatRelativeTime(minsAgo(59.5))).toBe("59m");
    expect(formatRelativeTime(minsAgo(61))).toBe("1h");
    expect(formatRelativeTime(minsAgo(60 * 23))).toBe("23h");
    expect(formatRelativeTime(minsAgo(60 * 25))).toBe("1d");
    expect(formatRelativeTime(minsAgo(60 * 24 * 40))).toBe("40d");
  });

  it("returns 'just now' for future timestamps", () => {
    expect(formatRelativeTime(new Date(NOW + 60_000))).toBe("just now");
    expect(formatRelativeTime(NOW + 3_600_000)).toBe("just now");
  });

  it("passes compact relative strings through unchanged", () => {
    for (const raw of ["5m", "45s", "2h", "3d", "1w", "2M", "1y"]) {
      expect(formatRelativeTime(raw)).toBe(raw);
    }
  });

  it("recovers an epoch embedded in a noisy string", () => {
    // 1700000000000 ≈ Nov 2023 — well in the past, so days-ago output.
    expect(formatRelativeTime("run-1700000000000")).toMatch(/^\d+d$/);
  });

  it("returns unparseable strings as-is unless they mention 'now'/'just'", () => {
    expect(formatRelativeTime("updated 3 days ago")).toBe("updated 3 days ago");
    expect(formatRelativeTime("now here")).toBe("just now");
  });

  it("returns the empty string for whitespace-only input (edge)", () => {
    // trim() collapses it to "", which fails every branch and falls through
    // the "does not mention now" return — documented quirk.
    expect(formatRelativeTime("   ")).toBe("");
  });
});

describe("getSessionEpoch", () => {
  it("parses updatedAt ISO strings", () => {
    const iso = new Date(NOW - 60_000).toISOString();
    expect(getSessionEpoch({ updatedAt: iso })).toBe(Date.parse(iso));
  });

  it("falls back to createdAt", () => {
    const iso = new Date(NOW - 120_000).toISOString();
    expect(getSessionEpoch({ createdAt: iso })).toBe(Date.parse(iso));
  });

  it("skips relative display strings and recovers the epoch from the id", () => {
    expect(getSessionEpoch({ updatedAt: "just now", id: "session-1729000000000" })).toBe(
      1729000000000
    );
  });

  it("rejects id epochs that are too old (12 digits < 1e12)", () => {
    expect(getSessionEpoch({ id: "x999999999999" })).toBe(0);
  });

  it("returns 0 when nothing is parseable", () => {
    expect(getSessionEpoch({})).toBe(0);
    expect(getSessionEpoch({ updatedAt: "garbage", id: "abc" })).toBe(0);
  });
});

describe("formatSessionTime", () => {
  it("reports running sessions before looking at timestamps", () => {
    expect(formatSessionTime({ status: "running", updatedAt: "5m" })).toBe("running");
  });

  it("passes compact relative strings through (trimmed)", () => {
    expect(formatSessionTime({ updatedAt: "5m" })).toBe("5m");
    expect(formatSessionTime({ updatedAt: " 2h " })).toBe("2h");
  });

  it("derives relative time from an id-embedded epoch", () => {
    expect(formatSessionTime({ id: "session-1729000000000" })).toMatch(/^\d+d$/);
  });

  it("falls back to 'just now' for corrupt/absent data (quirk)", () => {
    // Sessions with unparseable timestamps always display as 'just now'.
    expect(formatSessionTime({ id: "abc" })).toBe("just now");
    expect(formatSessionTime({})).toBe("just now");
  });
});
