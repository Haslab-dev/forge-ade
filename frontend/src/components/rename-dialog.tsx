import { useState, useEffect, useRef } from "react";
import * as Dialog from "@radix-ui/react-dialog";
import { cn } from "../lib/utils";

interface RenameDialogProps {
  open: boolean;
  currentName: string;
  onClose: () => void;
  onRename: (newName: string) => void;
}

export function RenameDialog({ open, currentName, onClose, onRename }: RenameDialogProps) {
  const [name, setName] = useState(currentName);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    setName(currentName);
  }, [currentName]);

  useEffect(() => {
    if (open && inputRef.current) {
      inputRef.current.focus();
      inputRef.current.select();
    }
  }, [open]);

  const handleSubmit = () => {
    const trimmed = name.trim();
    if (trimmed && trimmed !== currentName) {
      onRename(trimmed);
    }
    onClose();
  };

  return (
    <Dialog.Root open={open} onOpenChange={(open) => { if (!open) onClose(); }}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 bg-black/50 z-50" />
        <Dialog.Content className="fixed top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 z-50 bg-popover border rounded-lg shadow-lg p-4 w-80">
          <Dialog.Title className="text-sm font-medium mb-3">
            Rename Session
          </Dialog.Title>
          <input
            ref={inputRef}
            className="w-full text-sm bg-background border rounded px-2 py-1.5 outline-none focus:ring-1 focus:ring-ring mb-3"
            value={name}
            onChange={(e) => setName(e.target.value)}
            onKeyDown={(e) => { if (e.key === "Enter") handleSubmit(); if (e.key === "Escape") onClose(); }}
          />
          <div className="flex justify-end gap-2">
            <button
              className="text-xs px-3 py-1.5 rounded hover:bg-accent cursor-pointer"
              onClick={onClose}
            >
              Cancel
            </button>
            <button
              className={cn(
                "text-xs px-3 py-1.5 rounded cursor-pointer",
                name.trim() && name.trim() !== currentName
                  ? "bg-primary text-primary-foreground hover:bg-primary/90"
                  : "bg-muted text-muted-foreground"
              )}
              onClick={handleSubmit}
              disabled={!name.trim() || name.trim() === currentName}
            >
              Save
            </button>
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
