import React, { useState, useEffect } from "react";
import { useShortcutsStore, useUIStore } from "../hooks/store";
import { IconX, IconSettings, IconKeyboard, IconPalette } from "@tabler/icons-react";

interface GlobalSettingsModalProps {
  open: boolean;
  onClose: () => void;
}

export function GlobalSettingsModal({ open, onClose }: GlobalSettingsModalProps) {
  const { keybindings, setKeybindings } = useShortcutsStore();
  const { theme, setTheme } = useUIStore();
  const [activeTab, setActiveTab] = useState<"shortcuts" | "appearance">("shortcuts");
  const [editingId, setEditingId] = useState<string | null>(null);
  const [recordingKeys, setRecordingKeys] = useState<string[]>([]);

  useEffect(() => {
    if (!editingId) return;

    const handleKeyDown = (e: KeyboardEvent) => {
      e.preventDefault();
      e.stopPropagation();

      const parts: string[] = [];
      if (e.metaKey) parts.push("meta");
      if (e.ctrlKey) parts.push("ctrl");
      if (e.altKey) parts.push("alt");
      if (e.shiftKey) parts.push("shift");

      let key = e.key.toLowerCase();
      if (key === " ") key = "space";
      if (!["control", "shift", "alt", "meta"].includes(key)) {
        parts.push(key);
      }

      setRecordingKeys(parts);
    };

    const handleKeyUp = () => {
      if (recordingKeys.length > 0) {
        const combo = recordingKeys.join("+");
        setKeybindings(
          keybindings.map((kb) => (kb.id === editingId ? { ...kb, key: combo } : kb))
        );
        setEditingId(null);
        setRecordingKeys([]);
      }
    };

    window.addEventListener("keydown", handleKeyDown, true);
    window.addEventListener("keyup", handleKeyUp, true);

    return () => {
      window.removeEventListener("keydown", handleKeyDown, true);
      window.removeEventListener("keyup", handleKeyUp, true);
    };
  }, [editingId, recordingKeys, keybindings]);

  if (!open) return null;

  return (
    <div className="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex items-center justify-center p-4">
      <div className="bg-[var(--bg-sidebar)] border border-[var(--border-default)] w-full max-w-lg shadow-2xl p-4 flex flex-col h-[400px]">
        {/* Header */}
        <div className="flex items-center justify-between pb-2 border-b border-[var(--border-default)] shrink-0">
          <div className="flex items-center gap-1.5 font-bold text-sm text-[var(--fg-primary)]">
            <IconSettings className="size-4 text-[var(--accent-primary)]" />
            <span>Global Settings</span>
          </div>
          <button onClick={onClose} className="text-[var(--fg-tertiary)] hover:text-white cursor-pointer">
            <IconX className="size-4" />
          </button>
        </div>

        {/* Tab Selection */}
        <div className="flex border-b border-[var(--border-default)] text-xs text-[var(--fg-secondary)] shrink-0">
          <button
            onClick={() => setActiveTab("shortcuts")}
            className={`px-3 py-2 flex items-center gap-1 cursor-pointer ${
              activeTab === "shortcuts" ? "border-b-2 border-[var(--accent-primary)] text-white font-semibold" : ""
            }`}
          >
            <IconKeyboard className="size-3.5" />
            <span>Shortcuts</span>
          </button>
          <button
            onClick={() => setActiveTab("appearance")}
            className={`px-3 py-2 flex items-center gap-1 cursor-pointer ${
              activeTab === "appearance" ? "border-b-2 border-[var(--accent-primary)] text-white font-semibold" : ""
            }`}
          >
            <IconPalette className="size-3.5" />
            <span>Appearance</span>
          </button>
        </div>

        {/* Tab Content */}
        <div className="flex-1 overflow-y-auto py-3">
          {activeTab === "shortcuts" ? (
            <div className="space-y-3">
              <div className="text-[10px] text-[var(--fg-tertiary)] uppercase font-semibold tracking-wider">
                Click shortcut to remap keybinding
              </div>
              <div className="space-y-1">
                {keybindings.map((kb) => (
                  <div
                    key={kb.id}
                    onClick={() => {
                      setEditingId(kb.id);
                      setRecordingKeys([]);
                    }}
                    className={`flex items-center justify-between p-2 rounded cursor-pointer transition-colors ${
                      editingId === kb.id ? "bg-[var(--bg-surface-active)]" : "hover:bg-[var(--bg-panel)]"
                    }`}
                  >
                    <span className="text-xs text-[var(--fg-primary)] font-medium">{kb.name}</span>
                    <span className="text-xs font-mono px-2 py-0.5 bg-black/40 border border-[var(--border-default)] text-[var(--accent-primary)] rounded">
                      {editingId === kb.id
                        ? recordingKeys.length > 0
                          ? recordingKeys.join("+")
                          : "Press keys..."
                        : kb.key}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          ) : (
            <div className="space-y-3 text-xs">
              <label className="text-[var(--fg-secondary)] block font-semibold">Theme Palette</label>
              <select
                value={theme}
                onChange={(e) => setTheme(e.target.value)}
                className="w-full bg-[var(--bg-panel)] border border-[var(--border-default)] px-3 py-1.5 text-[var(--fg-primary)] focus:outline-none focus:border-[var(--accent-primary)]"
              >
                <option value="zed">Zed Dark Charcoal (Recommended)</option>
                <option value="dark">Forge Dark</option>
                <option value="light">Forge Light</option>
              </select>
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="flex items-center justify-end pt-2 border-t border-[var(--border-default)] shrink-0">
          <button
            onClick={onClose}
            className="px-4 py-1.5 bg-[var(--accent-primary)] hover:bg-[var(--accent-hover)] text-black text-xs font-semibold cursor-pointer"
          >
            Done
          </button>
        </div>
      </div>
    </div>
  );
}
