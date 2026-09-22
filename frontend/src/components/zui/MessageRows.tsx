import { Copy, Pencil, ThumbsDown, ThumbsUp } from "lucide-react";
import { useState } from "react";

export interface UserMessageRowProps {
  content: string;
  attachments?: string[];
  onEdit?: (newText: string) => void;
}

/**
 * User message — port of UserInputRowView: right-aligned bubble with the
 * rounded-tr-xs corner accent, hover copy/edit actions.
 */
export function UserMessageRow({ content, attachments = [], onEdit }: UserMessageRowProps) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(content);
  const [copied, setCopied] = useState(false);

  return (
    <div className="group/user-row flex flex-col items-end">
      {attachments.length > 0 ? (
        <div className="mb-1.5 flex flex-wrap justify-end gap-1.5">
          {attachments.map((a, i) => (
            <span key={i} className="rounded-full bg-surface px-3 py-1.5 text-ui-sm text-foreground-subtle">
              {a}
            </span>
          ))}
        </div>
      ) : null}
      {editing ? (
        <div className="flex w-full max-w-xl flex-col gap-2">
          <textarea
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            className="min-h-32 w-full resize-none rounded-xl border border-border bg-surface p-3 text-ui-base text-foreground outline-none"
          />
          <div className="flex justify-end gap-2">
            <button
              type="button"
              onClick={() => { setEditing(false); setDraft(content); }}
              className="rounded-lg border border-border px-3 py-1.5 text-ui-sm text-foreground-subtle hover:bg-surface-hover"
            >
              Cancel
            </button>
            <button
              type="button"
              onClick={() => { setEditing(false); onEdit?.(draft.trim()); }}
              className="rounded-lg bg-brand px-3 py-1.5 text-ui-sm font-medium text-foreground-inverse hover:bg-brand/80"
            >
              Send
            </button>
          </div>
        </div>
      ) : (
        <div className="flex max-w-full flex-col gap-2 rounded-xl rounded-tr-xs border border-border bg-surface px-4 py-3 text-ui-base text-foreground md:max-w-xl">
          <p className="break-words whitespace-pre-wrap">{content}</p>
        </div>
      )}
      {!editing ? (
        <div className="mt-1 flex gap-1 opacity-0 transition-opacity group-hover/user-row:opacity-100">
          <button
            type="button"
            aria-label="Copy"
            onClick={() => {
              navigator.clipboard?.writeText(content);
              setCopied(true);
              setTimeout(() => setCopied(false), 1200);
            }}
            className="rounded p-1 text-foreground-subtlest hover:bg-surface-hover hover:text-foreground"
          >
            <Copy className={`size-3.5 ${copied ? "text-success" : ""}`} />
          </button>
          {onEdit ? (
            <button
              type="button"
              aria-label="Edit"
              onClick={() => setEditing(true)}
              className="rounded p-1 text-foreground-subtlest hover:bg-surface-hover hover:text-foreground"
            >
              <Pencil className="size-3.5" />
            </button>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}

export interface AssistantMessageRowProps {
  content: string;
  streaming?: boolean;
  timestamp?: string;
  renderMarkdown: (content: string) => React.ReactNode;
  onFork?: () => void;
}

/**
 * Assistant message — port of AssistantTextRowView: full-width, no bubble,
 * markdown body, hover actions (copy / like / dislike / fork / timestamp).
 */
export function AssistantMessageRow({ content, streaming, timestamp, renderMarkdown, onFork }: AssistantMessageRowProps) {
  const [copied, setCopied] = useState(false);
  const [reaction, setReaction] = useState<"up" | "down" | null>(null);

  return (
    <div className="group/assistant-row w-full text-ui-base" data-conversation-selectable>
      <div className={streaming ? "forge-stream-text-in" : undefined}>
        {renderMarkdown(content)}
      </div>
      {!streaming ? (
        <div className="mt-1 flex items-center gap-1.5 opacity-0 transition-opacity group-hover/assistant-row:opacity-100">
          <button
            type="button"
            aria-label="Copy"
            onClick={() => {
              navigator.clipboard?.writeText(content);
              setCopied(true);
              setTimeout(() => setCopied(false), 1200);
            }}
            className="rounded p-1 text-foreground-subtlest hover:bg-surface-hover hover:text-foreground"
          >
            <Copy className={`size-3.5 ${copied ? "text-success" : ""}`} />
          </button>
          <button
            type="button"
            aria-label="Good response"
            onClick={() => setReaction(reaction === "up" ? null : "up")}
            className={`rounded p-1 text-foreground-subtlest hover:bg-surface-hover ${
              reaction === "up" ? "bg-success/10 text-success" : ""
            }`}
          >
            <ThumbsUp className="size-3.5" />
          </button>
          <button
            type="button"
            aria-label="Bad response"
            onClick={() => setReaction(reaction === "down" ? null : "down")}
            className={`rounded p-1 text-foreground-subtlest hover:bg-surface-hover ${
              reaction === "down" ? "bg-warning/10 text-warning" : ""
            }`}
          >
            <ThumbsDown className="size-3.5" />
          </button>
          {onFork ? (
            <button
              type="button"
              aria-label="Fork conversation"
              onClick={onFork}
              className="rounded p-1 text-foreground-subtlest hover:bg-surface-hover hover:text-foreground"
            >
              <svg viewBox="0 0 24 24" className="size-3.5" fill="none" stroke="currentColor" strokeWidth="2">
                <path d="M6 3v12a3 3 0 0 0 3 3h6" />
                <circle cx="6" cy="3" r="1" />
                <path d="M18 9a3 3 0 1 0 0-6 3 3 0 0 0 0 6Zm0 0v1a3 3 0 0 1-3 3H9" />
              </svg>
            </button>
          ) : null}
          {timestamp ? <span className="text-ui-sm text-foreground-subtlest">{timestamp}</span> : null}
        </div>
      ) : null}
    </div>
  );
}
