import React from 'react';
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { RulesManagerModal } from './RulesManagerModal';
import { useWorkspace } from '../../stores/workspaceStore';
import * as wails from '../../lib/wails';

describe('RulesManagerModal', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    useWorkspace.setState({
      agentWorkspacePath: '/data/workspace/forge-ade',
      activeWorkspacePath: '/data/workspace/forge-ade'
    });
    vi.spyOn(wails, 'GetWorkspaceRules').mockResolvedValue({
      activeRules: [
        { path: 'AGENTS.md', format: 'agents_md', exists: true, content: 'Rule 1: Always test', updatedAt: 1000 },
        { path: '.cursorrules', format: 'cursorrules', exists: false, content: '', updatedAt: 0 }
      ],
      effectivePrompt: 'Rule 1: Always test'
    });
    vi.spyOn(wails, 'SaveWorkspaceRule').mockResolvedValue(undefined);
    vi.spyOn(wails, 'SyncWorkspaceRule').mockResolvedValue(undefined);
  });

  afterEach(() => {
    cleanup();
  });

  it('renders modal header and content', async () => {
    const onClose = vi.fn();
    render(<RulesManagerModal isOpen={true} onClose={onClose} />);

    expect(screen.getByText('Unified Project Rules Manager')).toBeInTheDocument();

    await waitFor(() => {
      const textarea = screen.getByRole('textbox') as HTMLTextAreaElement;
      expect(textarea.value).toBe('Rule 1: Always test');
    }, { timeout: 3000 });
  });

  it('saves rule content when clicking save', async () => {
    const onClose = vi.fn();
    render(<RulesManagerModal isOpen={true} onClose={onClose} />);

    await waitFor(() => {
      const textarea = screen.getByRole('textbox') as HTMLTextAreaElement;
      expect(textarea.value).toBe('Rule 1: Always test');
    });

    const textarea = screen.getByRole('textbox');
    fireEvent.change(textarea, { target: { value: 'Updated test rules' } });

    const saveBtn = screen.getByRole('button', { name: /save rule/i });
    fireEvent.click(saveBtn);

    await waitFor(() => {
      expect(wails.SaveWorkspaceRule).toHaveBeenCalledWith(
        '/data/workspace/forge-ade',
        'AGENTS.md',
        'Updated test rules'
      );
    });
  });
});
