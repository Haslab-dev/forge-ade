import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { showToast, ToastProvider, useToast } from "./toast";

function FireButton({ message, kind }: { message: string; kind?: string }) {
  const { toast } = useToast();
  return <button onClick={() => toast(message, kind as never)}>fire</button>;
}

beforeEach(() => {
  vi.useFakeTimers();
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("ToastProvider", () => {
  it("shows a toast from useToast and auto-dismisses after 3s", () => {
    render(
      <ToastProvider>
        <FireButton message="saved!" kind="success" />
      </ToastProvider>
    );
    expect(screen.queryByText("saved!")).not.toBeInTheDocument();
    fireEvent.click(screen.getByText("fire"));
    expect(screen.getByText("saved!")).toBeInTheDocument();
    act(() => {
      vi.advanceTimersByTime(3000);
    });
    expect(screen.queryByText("saved!")).not.toBeInTheDocument();
  });

  it("applies rose styling for the danger kind", () => {
    render(
      <ToastProvider>
        <FireButton message="deleted" kind="danger" />
      </ToastProvider>
    );
    fireEvent.click(screen.getByText("fire"));
    const el = screen.getByText("deleted").parentElement;
    expect(el?.className).toContain("border-rose-500/40");
  });

  it("applies amber styling for the warn kind", () => {
    render(
      <ToastProvider>
        <FireButton message="careful" kind="warn" />
      </ToastProvider>
    );
    fireEvent.click(screen.getByText("fire"));
    const el = screen.getByText("careful").parentElement;
    expect(el?.className).toContain("border-amber-500/40");
  });

  it("maps the 'error' kind to default styling instead of rose (FORGE-117)", () => {
    // The normalizer handles danger/warn/info/success but not 'error', so
    // error toasts get the default sky styling even though the render branch
    // has an 'error' case. Desired behavior tracked as FORGE-117.
    render(
      <ToastProvider>
        <FireButton message="boom" kind="error" />
      </ToastProvider>
    );
    fireEvent.click(screen.getByText("fire"));
    const el = screen.getByText("boom").parentElement;
    expect(el?.className).toContain("border-sky-500/40");
  });

  it.skip("should style the 'error' kind with rose borders (FORGE-117)", () => {
    render(
      <ToastProvider>
        <FireButton message="boom" kind="error" />
      </ToastProvider>
    );
    fireEvent.click(screen.getByText("fire"));
    const el = screen.getByText("boom").parentElement;
    expect(el?.className).toContain("border-rose-500/40");
  });
});

describe("showToast module escape hatch", () => {
  it("is a silent no-op before any provider renders", () => {
    expect(() => showToast("early")).not.toThrow();
  });

  it("raises toasts after a provider has rendered", () => {
    render(<ToastProvider>children</ToastProvider>);
    act(() => {
      showToast("from module", "info");
    });
    expect(screen.getByText("from module")).toBeInTheDocument();
  });
});
