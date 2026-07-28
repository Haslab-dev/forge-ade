import { useState, useEffect, useRef } from "react";

interface SimpleModalProps {
  open: boolean;
  title: string;
  defaultValue?: string;
  placeholder?: string;
  onClose: () => void;
  onSubmit: (value: string) => void;
  submitLabel?: string;
}

export function SimpleModal({
  open,
  title,
  defaultValue = "",
  placeholder = "",
  onClose,
  onSubmit,
  submitLabel = "Save",
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

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
      onClick={onClose}
    >
      <div
        className="bg-popover border rounded-lg shadow-lg p-4 w-80"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="text-sm font-medium mb-3">{title}</div>
        <input
          ref={inputRef}
          className="w-full text-sm bg-background border rounded px-2 py-1.5 outline-none focus:ring-1 focus:ring-ring mb-3"
          value={value}
          placeholder={placeholder}
          onChange={(e) => setValue(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") handleSubmit();
            if (e.key === "Escape") onClose();
          }}
        />
        <div className="flex justify-end gap-2">
          <button
            className="text-xs px-3 py-1.5 rounded hover:bg-accent cursor-pointer"
            onClick={onClose}
          >
            Cancel
          </button>
          <button
            className="text-xs px-3 py-1.5 rounded bg-primary text-primary-foreground hover:bg-primary/90 cursor-pointer disabled:opacity-50"
            onClick={handleSubmit}
            disabled={!value.trim()}
          >
            {submitLabel}
          </button>
        </div>
      </div>
    </div>
  );
}
