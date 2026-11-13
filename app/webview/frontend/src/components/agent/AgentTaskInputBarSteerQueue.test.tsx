import React from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, beforeEach, vi } from 'vitest';
import { AgentTaskInputBar, parseSteerQueueItem } from './AgentTaskInputBar';
import { useSteerQueueStore } from '../../stores/steerQueueStore';
import { useComposerDraftStore } from '../../stores/composerDraftStore';

const mockSteerAgentPrompt = vi.fn();
const mockSendAgentPrompt = vi.fn();
let mockSessionStatus = 'idle';

vi.mock('../../stores/workspaceStore', () => ({
  useWorkspace: (selector?: (s: any) => any) => {
    const state = {
      agents: [{ id: 'agent-agy', name: 'Google Antigravity', type: 'acp', available: true }],
      activeAgent: { id: 'agent-agy', name: 'Google Antigravity', type: 'acp' },
      activeSession: { id: 'sess-test', status: mockSessionStatus, messages: [] },
      activeWorkspacePath: '/data/workspace/forge-ade',
      activeWorkspaces: [{ path: '/data/workspace/forge-ade', name: 'forge-ade', type: 'local', lastActive: Date.now() }],
      recentWorkspaces: [],
      connectedSSHIds: [],
      currentModel: 'gemini-3.8-flash',
      contextUsage: { usedTokens: 100, maxTokens: 1000000, percent: 0.01, formattedUsed: '100', formattedMax: '1M' },
      sendAgentPrompt: mockSendAgentPrompt,
      steerAgentPrompt: mockSteerAgentPrompt,
      compactCurrentSession: vi.fn(),
      gitBranch: 'main',
      providers: [],
      files: [],
      skills: [],
      mcps: [],
    };
    return typeof selector === 'function' ? selector(state) : state;
  }
}));

describe('parseSteerQueueItem', () => {
  it('parses prompt text and single image attachment correctly', () => {
    const raw = 'Please fix this visual bug\n\nAttached files:\n@.forge/attachments/shot.png';
    const parsed = parseSteerQueueItem(raw);

    expect(parsed.promptText).toBe('Please fix this visual bug');
    expect(parsed.attachments).toHaveLength(1);
    expect(parsed.attachments[0]).toEqual({
      path: '.forge/attachments/shot.png',
      name: 'shot.png',
      isImage: true,
    });
    expect(parsed.rawAttachmentsBlock).toBe('Attached files:\n@.forge/attachments/shot.png');
  });

  it('parses multiple attachments with mixed image and non-image files', () => {
    const raw = 'Analyze these files\n\nAttached files:\n@.forge/attachments/diagram.svg\n@src/index.ts';
    const parsed = parseSteerQueueItem(raw);

    expect(parsed.promptText).toBe('Analyze these files');
    expect(parsed.attachments).toHaveLength(2);
    expect(parsed.attachments[0]).toEqual({
      path: '.forge/attachments/diagram.svg',
      name: 'diagram.svg',
      isImage: true,
    });
    expect(parsed.attachments[1]).toEqual({
      path: 'src/index.ts',
      name: 'index.ts',
      isImage: false,
    });
  });

  it('parses queue item with only image and empty prompt text', () => {
    const raw = 'Attached files:\n@.forge/attachments/error.png';
    const parsed = parseSteerQueueItem(raw);

    expect(parsed.promptText).toBe('');
    expect(parsed.attachments).toHaveLength(1);
    expect(parsed.attachments[0].name).toBe('error.png');
    expect(parsed.attachments[0].isImage).toBe(true);
  });

  it('handles legacy "Please review the following attached files:" heading', () => {
    const raw = 'Please review the following attached files:\n@.forge/attachments/legacy.png';
    const parsed = parseSteerQueueItem(raw);

    expect(parsed.promptText).toBe('');
    expect(parsed.attachments).toHaveLength(1);
    expect(parsed.attachments[0].name).toBe('legacy.png');
    expect(parsed.attachments[0].isImage).toBe(true);
  });

  it('handles plain text queue item with no attachments', () => {
    const raw = 'Write 5 more unit tests';
    const parsed = parseSteerQueueItem(raw);

    expect(parsed.promptText).toBe('Write 5 more unit tests');
    expect(parsed.attachments).toHaveLength(0);
    expect(parsed.rawAttachmentsBlock).toBe('');
  });
});

describe('AgentTaskInputBar - Steer Queue with Attached Images', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockSessionStatus = 'running';
    useSteerQueueStore.setState({ queues: {} });
    useComposerDraftStore.getState().clearDraft('session:sess-test');
  });

  it('renders queued steer items with image badge in the stacked tray', () => {
    const sessionId = 'sess-test';
    useSteerQueueStore.getState().addToQueue(
      sessionId,
      'Fix button alignment\n\nAttached files:\n@.forge/attachments/btn_shot.png'
    );

    render(<AgentTaskInputBar />);

    // Queue tray header
    expect(screen.getByText(/Queued \(1\)/i)).toBeDefined();
    // Instruction text
    expect(screen.getByText('Fix button alignment')).toBeDefined();
    // Attached image filename badge
    expect(screen.getByText('btn_shot.png')).toBeDefined();
  });

  it('clicking "Steer" on a queued item with an image passes the full instruction with image to steerAgentPrompt', () => {
    const sessionId = 'sess-test';
    const fullInstruction = 'Fix button alignment\n\nAttached files:\n@.forge/attachments/btn_shot.png';
    useSteerQueueStore.getState().addToQueue(sessionId, fullInstruction);

    render(<AgentTaskInputBar />);

    const steerButtons = screen.getAllByRole('button', { name: /steer/i });
    // First steer button inside the queued item row
    const itemSteerBtn = steerButtons[0];
    fireEvent.click(itemSteerBtn);

    expect(mockSteerAgentPrompt).toHaveBeenCalledWith(fullInstruction);
    // Item was removed from queue after steering
    expect(useSteerQueueStore.getState().getQueue(sessionId)).toEqual([]);
  });

  it('inline editing a queued steer item updates text while preserving attached image references', () => {
    const sessionId = 'sess-test';
    useSteerQueueStore.getState().addToQueue(
      sessionId,
      'Initial typo\n\nAttached files:\n@.forge/attachments/mock.png'
    );

    render(<AgentTaskInputBar />);

    const editBtn = screen.getByTitle('Edit instruction');
    fireEvent.click(editBtn);

    // Editing input should only contain promptText ('Initial typo'), not the attachment string
    const editInput = screen.getByPlaceholderText('Type instruction...') as HTMLInputElement;
    expect(editInput.value).toBe('Initial typo');

    // Change instruction
    fireEvent.change(editInput, { target: { value: 'Corrected instructions' } });
    fireEvent.keyDown(editInput, { key: 'Enter' });

    // The queued item in store must retain the image attachment
    const updatedQueue = useSteerQueueStore.getState().getQueue(sessionId);
    expect(updatedQueue).toEqual([
      'Corrected instructions\n\nAttached files:\n@.forge/attachments/mock.png'
    ]);
  });

  it('shows Queue and Steer buttons when running and input has attached files even if text is empty', () => {
    useComposerDraftStore.getState().setDraft('session:sess-test', {
      prompt: '',
      attachedFiles: [
        {
          id: 'att-1',
          name: 'pasted_image.png',
          path: '/data/workspace/forge-ade/.forge/attachments/img_123_pasted.png',
          isImage: true,
          size: 1024,
        }
      ]
    });

    render(<AgentTaskInputBar />);

    // Queue button and Steer button should be visible
    expect(screen.getByRole('button', { name: /queue steering instruction/i })).toBeDefined();
    expect(screen.getByRole('button', { name: /steer agent now/i })).toBeDefined();
  });

  it('pressing Enter while running with an attached image queues prompt with image reference and clears attachments', () => {
    useComposerDraftStore.getState().setDraft('session:sess-test', {
      prompt: 'Check this out',
      attachedFiles: [
        {
          id: 'att-2',
          name: 'screenshot.png',
          path: '/data/workspace/forge-ade/.forge/attachments/img_screenshot.png',
          isImage: true,
          size: 2048,
        }
      ]
    });

    render(<AgentTaskInputBar />);

    const textarea = screen.getByRole('textbox') as HTMLTextAreaElement;
    expect(textarea.value).toBe('Check this out');

    // Press Enter to queue steer
    fireEvent.keyDown(textarea, { key: 'Enter', shiftKey: false });

    // Queued prompt in store must contain prompt text + attachment reference
    const queue = useSteerQueueStore.getState().getQueue('sess-test');
    expect(queue).toHaveLength(1);
    expect(queue[0]).toBe('Check this out\n\nAttached files:\n@.forge/attachments/img_screenshot.png');

    // Input textarea must be cleared
    expect(textarea.value).toBe('');
  });

  it('immediate steer button passes prompt with attached image and clears composer draft', () => {
    useComposerDraftStore.getState().setDraft('session:sess-test', {
      prompt: 'Steer right away',
      attachedFiles: [
        {
          id: 'att-3',
          name: 'urgent.png',
          path: '/data/workspace/forge-ade/.forge/attachments/urgent.png',
          isImage: true,
          size: 512,
        }
      ]
    });

    render(<AgentTaskInputBar />);

    const steerNowBtn = screen.getByRole('button', { name: /steer agent now/i });
    fireEvent.click(steerNowBtn);

    expect(mockSteerAgentPrompt).toHaveBeenCalledWith(
      'Steer right away\n\nAttached files:\n@.forge/attachments/urgent.png'
    );
  });
});
