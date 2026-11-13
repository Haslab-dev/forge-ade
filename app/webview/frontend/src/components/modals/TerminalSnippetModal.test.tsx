import { describe, expect, it, beforeEach, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import React from 'react';
import { TerminalSnippetModal } from './TerminalSnippetModal';
import { useTerminalSnippetStore } from '../../stores/terminalSnippetStore';

vi.mock('../../lib/wails', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../lib/wails')>();
  return {
    ...actual,
    WriteSession: vi.fn().mockResolvedValue(undefined),
    GetHomeDir: vi.fn().mockResolvedValue('/home/user')
  };
});

describe('TerminalSnippetModal', () => {
  beforeEach(() => {
    localStorage.clear();
    useTerminalSnippetStore.getState().resetDefaultSnippets();
    useTerminalSnippetStore.getState().clearHistory();
    useTerminalSnippetStore.getState().closeSnippetModal();
  });

  it('does not render when modal is closed', () => {
    const { container } = render(<TerminalSnippetModal />);
    expect(container.firstChild).toBeNull();
  });

  it('renders modal with tabs and snippets when open', () => {
    useTerminalSnippetStore.getState().openSnippetModal('term-sess-1');
    render(<TerminalSnippetModal />);

    expect(screen.getByText('Terminal Snippets & Command History')).toBeDefined();
    expect(screen.getByPlaceholderText(/search snippets/i)).toBeDefined();
    expect(screen.getByRole('button', { name: /New Snippet/i })).toBeDefined();

    // Check category pills
    expect(screen.getByRole('button', { name: /Remote \/ SSH/i })).toBeDefined();
    expect(screen.getByRole('button', { name: /Docker/i })).toBeDefined();
  });

  it('filters snippets by search query', () => {
    useTerminalSnippetStore.getState().openSnippetModal('term-sess-1');
    render(<TerminalSnippetModal />);

    const searchInput = screen.getByPlaceholderText(/search snippets/i);
    fireEvent.change(searchInput, { target: { value: 'nginx' } });

    expect(screen.getByText('Tail Nginx Error Log')).toBeDefined();
    expect(screen.queryByText('Docker Compose Up')).toBeNull();
  });

  it('switches to Remembered History tab and shows items', () => {
    useTerminalSnippetStore.getState().recordCommand('ssh root@192.168.1.100');
    useTerminalSnippetStore.getState().openSnippetModal('term-sess-1');
    render(<TerminalSnippetModal />);

    const historyTab = screen.getByRole('button', { name: /Remembered History/i });
    fireEvent.click(historyTab);

    expect(screen.getByText('ssh root@192.168.1.100')).toBeDefined();
  });
});
