import { Info, Wrench } from "lucide-react";

export interface PermissionOption {
  id: string;
  label: string;
  hint?: string;
}

export interface PermissionCardProps {
  title: string;
  toolName: string;
  /** Rendered parameter lines, e.g. `command: npm test`. */
  paramLines?: string[];
  reason?: string;
  running?: boolean;
  options: PermissionOption[];
  selectedId: string;
  onSelect: (id: string) => void;
  onConfirm: () => void;
  error?: string;
  footerHint?: string;
}

/**
 * Permission prompt — port of PermissionDialog.tsx docked card: bottom-dock
 * (above the composer), rounded-2xl popover card, radio option rows, brand
 * confirm button. Not a modal dialog.
 */
export function PermissionCard({
  title, toolName, paramLines, reason, running, options, selectedId,
  onSelect, onConfirm, error, footerHint,
}: PermissionCardProps) {
  return (
    <div className="relative z-1 w-full shrink-0">
      <div className="w-full overflow-hidden rounded-2xl border border-border bg-popover shadow-xs">
        <div className="flex flex-col gap-3 p-3">
          <p className="text-ui-base font-medium leading-tight text-foreground-subtle">{title}</p>
          <div className="flex items-center gap-2">
            <Wrench className="size-4 text-foreground-subtle" />
            <span className="text-ui-base font-medium text-foreground">{toolName}</span>
          </div>
          {paramLines?.map((line, i) => (
            <p key={i} className="break-all font-mono text-ui-base leading-5 text-foreground-subtle">
              {line}
            </p>
          ))}
          {reason ? <p className="text-ui-base leading-5 text-foreground">{reason}</p> : null}
          {running ? (
            <p className="animated-gradient-text font-medium text-ui-base">Waiting for permission…</p>
          ) : null}
          <div className="space-y-1">
            {options.map((opt) => (
              <button
                key={opt.id}
                type="button"
                onClick={() => onSelect(opt.id)}
                className={`flex w-full items-center gap-2 rounded-lg px-2 py-1.5 text-left ${
                  selectedId === opt.id ? "bg-selected" : "hover:bg-surface-hover"
                }`}
              >
                <span className="text-ui-base font-medium text-foreground">{opt.label}</span>
                {opt.hint ? <span className="text-ui-base leading-4 text-foreground-subtle">{opt.hint}</span> : null}
              </button>
            ))}
          </div>
          {error ? (
            <p role="alert" className="text-ui-base text-destructive">
              {error}
            </p>
          ) : null}
          <div className="flex items-center justify-between gap-2 px-1">
            <span className="flex items-center gap-1.5 text-ui-base text-foreground-subtle">
              <Info className="size-4" />
              {footerHint ?? "Review the requested action before allowing it."}
            </span>
            <button
              type="button"
              onClick={onConfirm}
              className="rounded-lg bg-brand px-3 py-1.5 text-ui-base font-medium text-foreground-inverse hover:bg-brand/80"
            >
              Allow
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
