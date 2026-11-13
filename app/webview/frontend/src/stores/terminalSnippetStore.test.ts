import { describe, expect, it, beforeEach } from 'vitest';
import { useTerminalSnippetStore } from './terminalSnippetStore';

describe('terminalSnippetStore', () => {
  beforeEach(() => {
    localStorage.clear();
    useTerminalSnippetStore.getState().resetDefaultSnippets();
    useTerminalSnippetStore.getState().clearHistory();
    useTerminalSnippetStore.getState().closeSnippetModal();
  });

  it('initializes with rich builtin snippets across multiple categories', () => {
    const snippets = useTerminalSnippetStore.getState().snippets;
    expect(snippets.length).toBeGreaterThan(10);
    const categories = new Set(snippets.map(s => s.category));
    expect(categories.has('remote')).toBe(true);
    expect(categories.has('docker')).toBe(true);
    expect(categories.has('system')).toBe(true);
    expect(categories.has('git')).toBe(true);
  });

  it('adds, updates, and deletes user custom snippets', () => {
    const store = useTerminalSnippetStore.getState();
    const id = store.addSnippet({
      title: 'My Custom SSH Check',
      command: 'ssh -v user@my-server.com',
      category: 'remote',
      description: 'Verbose SSH test'
    });

    expect(id).toBeTruthy();
    let current = useTerminalSnippetStore.getState().snippets.find(s => s.id === id);
    expect(current).toBeDefined();
    expect(current?.title).toBe('My Custom SSH Check');

    store.updateSnippet(id, { command: 'ssh -vvv user@my-server.com' });
    current = useTerminalSnippetStore.getState().snippets.find(s => s.id === id);
    expect(current?.command).toBe('ssh -vvv user@my-server.com');

    store.deleteSnippet(id);
    current = useTerminalSnippetStore.getState().snippets.find(s => s.id === id);
    expect(current).toBeUndefined();
  });

  it('records commands in history and manages favorites', () => {
    const store = useTerminalSnippetStore.getState();
    store.recordCommand('docker compose logs -f');
    store.recordCommand('systemctl restart nginx');

    let history = useTerminalSnippetStore.getState().history;
    expect(history.length).toBe(2);
    expect(history[0].command).toBe('systemctl restart nginx');
    expect(history[1].command).toBe('docker compose logs -f');

    // Toggle favorite
    const itemToFav = history[0];
    store.toggleFavoriteHistory(itemToFav.id);
    history = useTerminalSnippetStore.getState().history;
    expect(history[0].isFavorite).toBe(true);

    // Delete history item
    store.deleteHistoryItem(itemToFav.id);
    history = useTerminalSnippetStore.getState().history;
    expect(history.length).toBe(1);
    expect(history[0].command).toBe('docker compose logs -f');
  });

  it('controls modal open, close, and draft state', () => {
    const store = useTerminalSnippetStore.getState();
    expect(store.isSnippetModalOpen).toBe(false);

    store.openSnippetModal('term-sess-123', 'git stash pop', 'Pop Stash');
    expect(useTerminalSnippetStore.getState().isSnippetModalOpen).toBe(true);
    expect(useTerminalSnippetStore.getState().snippetModalTargetSessionId).toBe('term-sess-123');
    expect(useTerminalSnippetStore.getState().snippetDraftCommand).toBe('git stash pop');
    expect(useTerminalSnippetStore.getState().snippetDraftTitle).toBe('Pop Stash');

    store.closeSnippetModal();
    expect(useTerminalSnippetStore.getState().isSnippetModalOpen).toBe(false);
    expect(useTerminalSnippetStore.getState().snippetModalTargetSessionId).toBeNull();
  });
});
