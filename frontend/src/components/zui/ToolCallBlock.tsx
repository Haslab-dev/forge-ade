import { ChevronRight } from "lucide-react";
import { useState } from "react";

export type ToolCallStatus = "scheduled" | "waiting_permission" | "permission_denied" | "running" | "completed" | "failed";

export interface ToolCallBlockProps {
  icon: React.ReactNode;
  /** Kind label, e.g. "Bash", "Read", "Edit". Shimmers while running. */
  kind: string;
  /** Short detail after the label, e.g. the command or file name. */
  kindDetail?: string;
  sourceBadge?: string;
  primaryText?: string;
  secondaryText?: string;
  status?: ToolCallStatus;
  statusText?: string;
  errorTooltip?: string;
  diffCount?: { added: number; removed: number };
  children?: React.ReactNode;
  defaultOpen?: boolean;
}

function StatusWord({ status, statusText, errorTooltip }: { status?: ToolCallStatus; statusText?: string; errorTooltip?: string }) {
  if (!status || status === "completed") return null;
  const word = statusText ?? status.replace(/_/g, " ");
  if (status === "failed" || status === "permission_denied") {
    return (
      <span
        className="max-w-96 cursor-help break-words text-foreground-subtlest underline decoration-dotted underline-offset-2"
        title={errorTooltip}
      >
        {word}
      </span>
    );
  }
  return <span className="text-foreground-subtlest">{word}</span>;
}

/**
 * Borderless single-line collapsible tool block — port of ToolLayout.tsx
 * chrome. Icon stays static while running; the kind label shimmers via
 * .animated-gradient-text; chevron appears on hover and rotates when open.
 */
export function ToolCallBlock({
  icon, kind, kindDetail, sourceBadge, primaryText, secondaryText,
  status, statusText, errorTooltip, diffCount, children, defaultOpen,
}: ToolCallBlockProps) {
  const [open, setOpen] = useState(!!defaultOpen);
  const running = status === "running" || status === "waiting_permission";
  return (
    <div className="flex w-full flex-col" data-testid={`tool-block:${kind}`}>
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="group/tool-summary inline-flex max-w-full cursor-pointer items-start gap-2 self-start text-left text-ui-base"
      >
        <span className="mt-0.5 shrink-0 text-foreground-subtlest [&_svg]:size-4">{icon}</span>
        <span className={`whitespace-nowrap font-medium ${running ? "animated-gradient-text" : "text-foreground-subtlest"}`}>
          {kind}
        </span>
        {kindDetail ? <span className="truncate text-foreground-subtlest">{kindDetail}</span> : null}
        {sourceBadge ? (
          <span className="rounded border border-border bg-background-alt px-1.5 py-0.5 text-ui-xs leading-none text-foreground-subtlest">
            {sourceBadge}
          </span>
        ) : null}
        {primaryText ? <span className="truncate text-foreground-subtlest">{primaryText}</span> : null}
        {secondaryText ? <span className="truncate text-foreground-subtlest">{secondaryText}</span> : null}
        {diffCount ? (
          <span className="shrink-0 font-mono text-ui-xs leading-none">
            <span className="text-diff-added">+{diffCount.added}</span>{" "}
            <span className="text-diff-removed">-{diffCount.removed}</span>
          </span>
        ) : null}
        <StatusWord status={status} statusText={statusText} errorTooltip={errorTooltip} />
        <ChevronRight
          className={`size-4 shrink-0 text-foreground-subtlest opacity-0 transition-transform group-hover/tool-summary:opacity-100 ${
            open ? "rotate-90" : ""
          }`}
        />
      </button>
      {open && children ? <div className="pt-2">{children}</div> : null}
    </div>
  );
}

/** Nested tool call children container (ml-2 border-l). */
export function ToolCallChildren({ children }: { children: React.ReactNode }) {
  return <div className="ml-2 space-y-2 border-l border-border pl-3.5">{children}</div>;
}
