import { describe, it, expect } from "vitest";
import { getChronologicalTimeline } from "../../lib/timeline";
import type { AgentMessage } from "../../types";

describe("getChronologicalTimeline", () => {
  it("interleaves think -> tool -> response -> think -> tool -> final response using seq", () => {
    const msg: AgentMessage = {
      id: "msg-1",
      role: "agent",
      content: "Full text content",
      timestamp: "10:30",
      thoughts: [
        {
          id: "th-1",
          thoughtText: "First thought",
          durationSeconds: 2,
          timestamp: "10:30",
          turn: 1,
          seq: 1,
        },
        {
          id: "th-2",
          thoughtText: "Second thought after tool",
          durationSeconds: 1,
          timestamp: "10:30",
          turn: 1,
          seq: 4,
        },
      ],
      toolExecutions: [
        {
          id: "tool-1",
          toolName: "read_file",
          command: '{"file_path":"test.txt"}',
          output: "hello world",
          status: "completed",
          turn: 1,
          seq: 2,
        },
        {
          id: "tool-2",
          toolName: "bash",
          command: 'echo done',
          output: "done",
          status: "completed",
          turn: 1,
          seq: 5,
        },
      ],
      textBlocks: [
        {
          id: "tb-1",
          text: "I read the file and here is what I found.",
          turn: 1,
          seq: 3,
        },
        {
          id: "tb-2",
          text: "Final response: everything is complete!",
          turn: 1,
          seq: 6,
        },
      ],
    };

    const timeline = getChronologicalTimeline(msg);
    expect(timeline.length).toBe(6);
    expect(timeline.map(item => item.kind)).toEqual([
      "thought",
      "tool",
      "text",
      "thought",
      "tool",
      "text",
    ]);

    expect(timeline[0].thought?.thoughtText).toBe("First thought");
    expect(timeline[1].tool?.toolName).toBe("read_file");
    expect(timeline[2].text).toBe("I read the file and here is what I found.");
    expect(timeline[3].thought?.thoughtText).toBe("Second thought after tool");
    expect(timeline[4].tool?.toolName).toBe("bash");
    expect(timeline[5].text).toBe("Final response: everything is complete!");
  });

  it("falls back to legacy turn-based order when no textBlocks or seq are present", () => {
    const msg: AgentMessage = {
      id: "msg-legacy",
      role: "agent",
      content: "Legacy final response",
      timestamp: "10:00",
      turnTexts: {
        1: "Legacy final response",
      },
      thoughts: [
        {
          id: "th-legacy",
          thoughtText: "Old thought",
          durationSeconds: 3,
          timestamp: "10:00",
          turn: 1,
        },
      ],
      toolExecutions: [
        {
          id: "tool-legacy",
          toolName: "read_file",
          command: "file.txt",
          output: "content",
          status: "completed",
          turn: 1,
        },
      ],
    };

    const timeline = getChronologicalTimeline(msg);
    expect(timeline.length).toBe(3);
    expect(timeline[0].kind).toBe("thought");
    expect(timeline[1].kind).toBe("text");
    expect(timeline[2].kind).toBe("tool");
  });

  it("sorts by createdAtMs when seq is not present but createdAtMs is available", () => {
    const t0 = 1700000000000;
    const msg: AgentMessage = {
      id: "msg-time",
      role: "agent",
      content: "All text",
      timestamp: "10:00",
      thoughts: [
        {
          id: "th-1",
          thoughtText: "First think",
          durationSeconds: 1,
          timestamp: "10:00",
          createdAtMs: t0 + 10,
        },
      ],
      toolExecutions: [
        {
          id: "tool-1",
          toolName: "read_file",
          command: "file.txt",
          output: "content",
          status: "completed",
          createdAtMs: t0 + 20,
        },
      ],
      textBlocks: [
        {
          id: "tb-1",
          text: "Intermediate text",
          createdAtMs: t0 + 30,
        },
      ],
    };

    const timeline = getChronologicalTimeline(msg);
    expect(timeline.length).toBe(3);
    expect(timeline.map(i => i.kind)).toEqual(["thought", "tool", "text"]);
    expect(timeline[0].thought?.thoughtText).toBe("First think");
    expect(timeline[1].tool?.toolName).toBe("read_file");
    expect(timeline[2].text).toBe("Intermediate text");
  });
});
