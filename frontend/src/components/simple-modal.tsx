import React, { useState, useEffect } from "react";
import { IconX } from "@tabler/icons-react";

interface SimpleModalProps {
  open: boolean;
  title: string;
  defaultValue?: string;
  placeholder?: string;
  submitLabel?: string;
  onClose: () => void;
  onSubmit: (value: string) => void;
  secondaryAction?: {
    label: string;
    icon: React.ReactNode;
    onClick: () => void;
  };
}

export function SimpleModal({
  open,
  title,
  defaultValue = "",
  placeholder = "",
  submitLabel = "Submit",
  onClose,
  onSubmit,
  secondaryAction,
}: SimpleModalProps) {
  const [value, setValue] = useState(defaultValue);

  useEffect(() => {
    if (open) {
      setValue(defaultValue);
    }
  }, [open, defaultValue]);

  if (!open) return null;

  return (
    <div className="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex items-center justify-center p-4">
      <div className="bg-[var(--bg-sidebar)] border border-[var(--border-default)] w-full max-w-md shadow-2xl p-4 flex flex-col gap-3">
        {/* Header */}
        <div className="flex items-center justify-between pb-2 border-b border-[var(--border-default)]">
          <span className="font-bold text-sm text-[var(--fg-primary)]">{title}</span>
          <button
            onClick={onClose}
            className="text-[var(--fg-tertiary)] hover:text-[var(--fg-primary)] cursor-pointer"
          >
            <IconX className="size-4" />
          </button>
        </div>

        {/* Input */}
        <input
          type="text"
          value={value}
          onChange={(e) => setValue(e.target.value)}
          placeholder={placeholder}
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              onSubmit(value);
            }
          }}
          className="w-full bg-[var(--bg-panel)] border border-[var(--border-default)] px-3 py-1.5 text-xs text-[var(--fg-primary)] focus:outline-none focus:border-[var(--accent-primary)]"
          autoFocus
        />

        {/* Actions */}
        <div className="flex items-center justify-between mt-2 pt-2 border-t border-[var(--border-default)]">
          {secondaryAction ? (
            <button
              onClick={secondaryAction.onClick}
              className="flex items-center gap-1.5 px-3 py-1.5 text-xs text-[var(--fg-secondary)] hover:text-[var(--fg-primary)] bg-[var(--bg-panel)] border border-[var(--border-default)] hover:bg-[var(--bg-surface)] cursor-pointer"
            >
              {secondaryAction.icon}
              <span>{secondaryAction.label}</span>
            </button>
          ) : (
            <div />
          )}

          <div className="flex items-center gap-2">
            <button
              onClick={onClose}
              className="px-3 py-1.5 text-xs text-[var(--fg-secondary)] hover:text-[var(--fg-primary)] cursor-pointer"
            >
              Cancel
            </button>
            <button
              onClick={() => onSubmit(value)}
              className="px-4 py-1.5 text-xs font-semibold bg-[var(--accent-primary)] hover:bg-[var(--accent-hover)] text-black cursor-pointer"
            >
              {submitLabel}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
