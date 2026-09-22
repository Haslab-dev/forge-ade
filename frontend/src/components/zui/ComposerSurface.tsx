import { ArrowUp, Loader2, Square } from "lucide-react";
import { useCallback, useRef, useState } from "react";

export interface AttachmentChip {
  id: string;
  label: string;
  onRemove?: () => void;
}

export interface ComposerSurfaceProps {
  onSubmit: (text: string) => void;
  onStop?: () => void;
  pending?: boolean;
  placeholder?: string;
  /** Context header row (workspace picker / branch switcher) in draft state. */
  header?: React.ReactNode;
  attachments?: AttachmentChip[];
  toolbar?: React.ReactNode;
  autoFocus?: boolean;
}

/**
 * Composer surface — port of the ConversationComposer shell: rounded-2xl
 * surface card with shadow, context header row, attachment chips, plain
 * textarea editor (Enter submits, Shift+Enter newline), brand send pill /
 * stop square.
 */
export function ComposerSurface({
  onSubmit, onStop, pending, placeholder = "Ask ForgeADE to do anything…",
  header, attachments = [], toolbar, autoFocus,
}: ComposerSurfaceProps) {
  const [value, setValue] = useState("");
  const ref = useRef<HTMLTextAreaElement>(null);

  const submit = useCallback(() => {
    const text = value.trim();
    if (!text || pending) return;
    onSubmit(text);
    setValue("");
  }, [value, pending, onSubmit]);

  return (
    <div className="z-20 w-full shrink-0">
      <div className={`w-full ${header ? "rounded-2xl bg-surface shadow-xl/5" : ""}`}>
        {header ? <div className="flex min-w-0 flex-wrap items-center gap-0 p-1.5">{header}</div> : null}
        {attachments.length > 0 ? (
          <div className="flex flex-wrap gap-1.5 px-3 pt-2">
            {attachments.map((a) => (
              <span key={a.id} className="relative inline-flex items-center rounded-full bg-surface px-3 py-1.5 text-ui-sm text-foreground-subtle">
                {a.label}
                {a.onRemove ? (
                  <button
                    type="button"
                    onClick={a.onRemove}
                    className="absolute -top-0.5 -right-0.5 size-3.5 rounded-full bg-primary text-[8px] leading-none text-primary-foreground"
                    aria-label={`Remove ${a.label}`}
                  >
                    ×
                  </button>
                ) : null}
              </span>
            ))}
          </div>
        ) : null}
        <div className="flex items-end gap-2 p-2">
          <textarea
            ref={ref}
            autoFocus={autoFocus}
            value={value}
            rows={1}
            onChange={(e) => {
              setValue(e.target.value);
              e.target.style.height = "auto";
              e.target.style.height = `${Math.min(e.target.scrollHeight, 160)}px`;
            }}
            onKeyDown={(e) => {
              if (e.key === "Enter" && !e.shiftKey) {
                e.preventDefault();
                submit();
              }
            }}
            placeholder={placeholder}
            className="max-h-40 min-h-10 flex-1 resize-none overflow-y-auto bg-transparent text-ui-base leading-5 text-foreground outline-none placeholder:text-foreground-subtlest"
          />
          {toolbar}
          {pending && onStop ? (
            <button
              type="button"
              onClick={onStop}
              className="flex cursor-pointer items-center gap-1 rounded-lg bg-brand p-2 text-foreground-inverse hover:bg-brand/80"
              aria-label="Stop"
            >
              <Square className="size-4 fill-current" />
            </button>
          ) : (
            <button
              type="button"
              onClick={submit}
              disabled={!value.trim() || pending}
              className="flex cursor-pointer items-center gap-1 rounded-lg bg-brand p-2 text-foreground-inverse hover:bg-brand/80 disabled:opacity-50"
              aria-label="Send"
            >
              {pending ? <Loader2 className="size-4 animate-spin" /> : <ArrowUp className="size-4" />}
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
