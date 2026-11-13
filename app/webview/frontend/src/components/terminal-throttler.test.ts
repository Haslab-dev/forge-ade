import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { Terminal } from "xterm";
import { writeToTerminalThrottled, cancelTerminalThrottled } from "./terminal-view";

describe("writeToTerminalThrottled", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("batches multiple chunks into a single term.write within a single animation frame", () => {
    const term = new Terminal();
    const writeSpy = vi.spyOn(term, "write").mockImplementation((data, cb) => {
      cb?.();
      return term;
    });

    writeToTerminalThrottled(term, "hello ");
    writeToTerminalThrottled(term, "world");
    writeToTerminalThrottled(term, "!");

    // Not yet written before animation frame fires
    expect(writeSpy).not.toHaveBeenCalled();

    // Trigger animation frame callbacks
    vi.runAllTimers();

    expect(writeSpy).toHaveBeenCalledTimes(1);
    expect(writeSpy).toHaveBeenCalledWith("hello world!", expect.any(Function));
  });

  it("invokes flushed callbacks after write finishes", () => {
    const term = new Terminal();
    vi.spyOn(term, "write").mockImplementation((data, cb) => {
      cb?.();
      return term;
    });

    const cb1 = vi.fn();
    const cb2 = vi.fn();

    writeToTerminalThrottled(term, "chunk 1", cb1);
    writeToTerminalThrottled(term, "chunk 2", cb2);

    expect(cb1).not.toHaveBeenCalled();
    expect(cb2).not.toHaveBeenCalled();

    vi.runAllTimers();

    expect(cb1).toHaveBeenCalledTimes(1);
    expect(cb2).toHaveBeenCalledTimes(1);
  });

  it("cancels pending throttled write when cancelTerminalThrottled is called", () => {
    const term = new Terminal();
    const writeSpy = vi.spyOn(term, "write").mockImplementation((data, cb) => {
      cb?.();
      return term;
    });

    writeToTerminalThrottled(term, "will be cancelled");
    cancelTerminalThrottled(term);

    vi.runAllTimers();

    expect(writeSpy).not.toHaveBeenCalled();
  });

  it("keeps queues isolated between different terminal instances", () => {
    const term1 = new Terminal();
    const term2 = new Terminal();

    const writeSpy1 = vi.spyOn(term1, "write").mockImplementation((data, cb) => {
      cb?.();
      return term1;
    });
    const writeSpy2 = vi.spyOn(term2, "write").mockImplementation((data, cb) => {
      cb?.();
      return term2;
    });

    writeToTerminalThrottled(term1, "term1 data");
    writeToTerminalThrottled(term2, "term2 data");

    vi.runAllTimers();

    expect(writeSpy1).toHaveBeenCalledWith("term1 data", expect.any(Function));
    expect(writeSpy2).toHaveBeenCalledWith("term2 data", expect.any(Function));
  });
});
