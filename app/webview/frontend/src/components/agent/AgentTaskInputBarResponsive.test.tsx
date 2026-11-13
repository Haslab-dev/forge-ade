import React from 'react';
import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { AgentTaskInputBar } from './AgentTaskInputBar';

vi.mock('../../stores/workspaceStore', () => ({
  useWorkspace: () => ({
    agents: [{ id: 'agent-internal', name: 'ForgeADE Internal', type: 'internal', available: true, model: 'gpt-4o' }],
    activeAgent: { id: 'agent-internal', name: 'ForgeADE Internal', type: 'internal', model: 'gpt-4o' },
    activeSession: { id: 'sess-resp-1', messages: [], executionMode: 'code' },
    activeWorkspacePath: '/data/workspace/forge-ade',
    activeWorkspaces: [{ path: '/data/workspace/forge-ade', name: 'forge-ade', type: 'local', lastActive: Date.now() }],
    recentWorkspaces: [],
    connectedSSHIds: [],
    currentModel: 'gpt-4o',
    contextUsage: { usedTokens: 50, maxTokens: 100000, percent: 0.05, formattedUsed: '50', formattedMax: '100k' },
    sendAgentPrompt: vi.fn(),
    steerAgentPrompt: vi.fn(),
    compactCurrentSession: vi.fn(),
    gitBranch: 'main',
    providers: [],
    files: [],
    skills: [],
    mcps: [],
  })
}));

describe('AgentTaskInputBar - Responsive Layout', () => {
  it('renders with container query wrapper class on root', () => {
    const { container } = render(<AgentTaskInputBar />);
    const root = container.firstChild as HTMLElement;
    expect(root.className).toContain('@container/composer');
    expect(root.className).toContain('max-w-[var(--app-chat-max-width,48rem)]');
  });

  it('expands to full column width when fillWidth is true', () => {
    const { container } = render(<AgentTaskInputBar fillWidth={true} />);
    const root = container.firstChild as HTMLElement;
    expect(root.className).toContain('@container/composer');
    expect(root.className).toContain('max-w-none');
    expect(root.className).toContain('px-1');
  });

  it('renders model trigger button with flexible max-width and min-w-0 for narrow sidebar view', () => {
    render(<AgentTaskInputBar />);
    const modelBtn = screen.getByTitle(/harness:/i);
    expect(modelBtn).toBeDefined();
    expect(modelBtn.className).toContain('min-w-0');
    expect(modelBtn.className).toContain('max-w-[130px]');
  });

  it('renders permission mode chip with responsive text label', () => {
    render(<AgentTaskInputBar />);
    const modeBtn = screen.getByRole('button', { name: /permission mode/i });
    expect(modeBtn).toBeDefined();
    const modeLabel = modeBtn.querySelector('span');
    expect(modeLabel?.className).toContain('hidden');
    expect(modeLabel?.className).toContain('@[440px]/composer:inline');
  });

  it('renders auto-verify button with responsive label that collapses on narrow width', () => {
    render(<AgentTaskInputBar />);
    const verifyBtn = screen.getByRole('button', { name: /toggle auto-verification loop/i });
    expect(verifyBtn).toBeDefined();
    const verifySpan = verifyBtn.querySelector('span');
    expect(verifySpan?.textContent).toBe('Verify');
    expect(verifySpan?.className).toContain('hidden');
    expect(verifySpan?.className).toContain('@[480px]/composer:inline');
  });
});
