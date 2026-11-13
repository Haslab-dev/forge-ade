import { useState, useEffect } from 'react';

export interface UseAgentThreadSearchResult {
  findOpen: boolean;
  setFindOpen: (open: boolean) => void;
  openFind: () => void;
  closeFind: () => void;
}

/**
 * Manages in-thread find (⌘F / Ctrl+F) lifecycle and keyboard shortcuts.
 */
export function useAgentThreadSearch(): UseAgentThreadSearchResult {
  const [findOpen, setFindOpen] = useState(false);

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'f') {
        e.preventDefault();
        setFindOpen(true);
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, []);

  return {
    findOpen,
    setFindOpen,
    openFind: () => setFindOpen(true),
    closeFind: () => setFindOpen(false),
  };
}
