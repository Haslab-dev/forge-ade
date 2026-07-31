import { useState, useEffect, useRef } from "react";

interface SimpleModalProps {
  open: boolean;
  title: string;
  defaultValue?: string;
  placeholder?: string;
  onClose: () => void;
  onSubmit: (value: string) => void;
  submitLabel?: string;
  destructive?: boolean;
  error?: string;
  secondaryAction?: {
    label: string;
    onClick: () => void;
    icon?: React.ReactNode;
  };
}

export function SimpleModal({
  open,
  title,
  defaultValue = "",
  placeholder = "",
  onClose,
  onSubmit,
  submitLabel = "Save",
  destructive = false,
  error,
  secondaryAction,
}: SimpleModalProps) {
  const [value, setValue] = useState(defaultValue);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    setValue(defaultValue);
  }, [defaultValue, open]);

  useEffect(() => {
    if (open && inputRef.current) {
      setTimeout(() => {
        inputRef.current?.focus();
        inputRef.current?.select();
      }, 50);
    }
  }, [open]);

  if (!open) return null;

  const handleSubmit = () => {
    const trimmed = value.trim();
    if (trimmed) onSubmit(trimmed);
    onClose();
  };

  const isConfirm = destructive;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 select-none"
      onClick={onClose}
    >
      <div
        className="bg-popover border rounded-lg shadow-lg p-4 w-88 max-w-full"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="text-sm font-medium mb-3">{title}</div>

        {secondaryAction && (
          <div className="mb-3">
            <button
              type="button"
              className="w-full flex items-center justify-center gap-2 px-3 py-2 text-xs font-medium bg-accent hover:bg-accent/80 border border-border/60 rounded-md transition-colors cursor-pointer text-foreground shadow-sm"
              onClick={() => {
                secondaryAction.onClick();
              }}
            >
              {secondaryAction.icon}
              <span>{secondaryAction.label}</span>
            </button>
            <div className="flex items-center my-3">
              <div className="flex-1 border-t border-border/60" />
              <span className="px-2 text-[10px] text-muted-foreground uppercase tracking-wider font-semibold">
                or enter path
              </span>
              <div className="flex-1 border-t border-border/60" />
            </div>
          </div>
        )}

        {isConfirm ? (
          <p className="text-xs text-muted-foreground mb-3">
            Are you sure you want to delete{" "}
            <span className="font-medium text-foreground">{defaultValue || ""}</span>
            ? This action cannot be undone.
          </p>
        ) : (
          <input
            ref={inputRef}
            className="w-full text-sm bg-background border rounded px-2.5 py-1.5 outline-none focus:ring-1 focus:ring-ring mb-3"
            value={value}
            placeholder={placeholder}
            onChange={(e) => setValue(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") handleSubmit();
              if (e.key === "Escape") onClose();
            }}
          />
        )}
        {error && (
          <p className="text-xs text-red-400 mb-3 break-words">{error}</p>
        )}
        <div className="flex justify-end gap-2">
          <button
            className="text-xs px-3 py-1.5 rounded hover:bg-accent cursor-pointer"
            onClick={onClose}
          >
            Cancel
          </button>
          {isConfirm ? (
            <button
              className="text-xs px-3 py-1.5 rounded bg-red-500 text-white hover:bg-red-600 cursor-pointer"
              onClick={() => onSubmit(defaultValue)}
            >
              {submitLabel}
            </button>
          ) : (
            <button
              className="text-xs px-3 py-1.5 rounded bg-primary text-primary-foreground hover:bg-primary/90 cursor-pointer disabled:opacity-50"
              onClick={handleSubmit}
              disabled={!value.trim()}
            >
              {submitLabel}
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
