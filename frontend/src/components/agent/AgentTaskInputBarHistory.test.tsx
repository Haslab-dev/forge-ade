import React from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, beforeEach, vi } from 'vitest';
import { AgentTaskInputBar } from './AgentTaskInputBar';
import { usePromptHistoryStore } from '../../stores/promptHistoryStore';
import { useAutoVerifyStore } from '../../stores/autoVerifyStore';

vi.mock('../../stores/workspaceStore', () => ({
  useWorkspace: () => ({
    agents: [{ id: 'agent-agy', name: 'Google Antigravity', type: 'acp', available: true }],
    activeAgent: { id: 'agent-agy', name: 'Google Antigravity', type: 'acp' },
    activeSession: { id: 'sess-1', messages: [] },
    activeWorkspacePath: '/data/workspace/forge-ade',
    activeWorkspaces: [{ path: '/data/workspace/forge-ade', name: 'forge-ade', type: 'local', lastActive: Date.now() }],
    recentWorkspaces: [],
    connectedSSHIds: [],
    currentModel: 'gemini-3.8-flash',
    contextUsage: { usedTokens: 100, maxTokens: 1000000, percent: 0.01, formattedUsed: '100', formattedMax: '1M' },
    sendAgentPrompt: vi.fn(),
    steerAgentPrompt: vi.fn(),
    compactCurrentSession: vi.fn(),
  })
}));

describe('AgentTaskInputBar - Prompt History Arrow Up/Down', () => {
  beforeEach(() => {
    usePromptHistoryStore.getState().clearHistory();
    usePromptHistoryStore.getState().addHistory('first prompt');
    usePromptHistoryStore.getState().addHistory('second prompt');
  });

  it('cycles backwards through prompt history on ArrowUp and restores draft on ArrowDown', () => {
    render(<AgentTaskInputBar />);

    const textarea = screen.getByRole('textbox') as HTMLTextAreaElement;
    expect(textarea.value).toBe('');

    // Press ArrowUp: should recall most recent prompt ("second prompt")
    fireEvent.keyDown(textarea, { key: 'ArrowUp' });
    expect(textarea.value).toBe('second prompt');

    // Press ArrowUp again: should recall older prompt ("first prompt")
    fireEvent.keyDown(textarea, { key: 'ArrowUp' });
    expect(textarea.value).toBe('first prompt');

    // Press ArrowDown: should move forward to "second prompt"
    fireEvent.keyDown(textarea, { key: 'ArrowDown' });
    expect(textarea.value).toBe('second prompt');

    // Press ArrowDown again: should restore original empty draft
    fireEvent.keyDown(textarea, { key: 'ArrowDown' });
    expect(textarea.value).toBe('');
  });

  it('preserves existing draft while navigating history and restores it at the bottom', () => {
    render(<AgentTaskInputBar />);

    const textarea = screen.getByRole('textbox') as HTMLTextAreaElement;

    // Type a draft
    fireEvent.change(textarea, { target: { value: 'my unfinished idea' } });
    expect(textarea.value).toBe('my unfinished idea');

    // Set cursor at beginning and press ArrowUp
    textarea.selectionStart = 0;
    textarea.selectionEnd = 0;
    fireEvent.keyDown(textarea, { key: 'ArrowUp' });
    expect(textarea.value).toBe('second prompt');

    // Press ArrowDown to return to the draft
    fireEvent.keyDown(textarea, { key: 'ArrowDown' });
    expect(textarea.value).toBe('my unfinished idea');
  });

  it('toggles auto-verification when Verify button is clicked', () => {
    useAutoVerifyStore.setState({ enabled: false });
    render(<AgentTaskInputBar />);

    const verifyBtn = screen.getByRole('button', { name: /toggle auto-verification loop/i });
    expect(verifyBtn).toBeDefined();

    expect(useAutoVerifyStore.getState().enabled).toBe(false);
    fireEvent.click(verifyBtn);
    expect(useAutoVerifyStore.getState().enabled).toBe(true);

    fireEvent.click(verifyBtn);
    expect(useAutoVerifyStore.getState().enabled).toBe(false);
  });

  it('dispatches forge:chat-submitted event when pressing Enter to send prompt', () => {
    const chatSubmittedSpy = vi.fn();
    window.addEventListener('forge:chat-submitted', chatSubmittedSpy);

    render(<AgentTaskInputBar />);
    const textarea = screen.getByRole('textbox') as HTMLTextAreaElement;

    fireEvent.change(textarea, { target: { value: 'Fix auto scroll issue' } });
    fireEvent.keyDown(textarea, { key: 'Enter', shiftKey: false });

    expect(chatSubmittedSpy).toHaveBeenCalled();
    window.removeEventListener('forge:chat-submitted', chatSubmittedSpy);
  });
});
