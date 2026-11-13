import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { BranchSelector } from './BranchSelector';
import { ApiBridge } from '../../services/apiBridge';

const mockUseWorkspace = vi.fn();
vi.mock('../../stores/workspaceStore', () => ({
  useWorkspace: () => mockUseWorkspace(),
}));

vi.mock('../../services/apiBridge', () => ({
  ApiBridge: {
    getGitBranches: vi.fn(async () => ['main', 'feature-login', 'feature-auth']),
    gitCheckout: vi.fn(async () => {}),
    gitCreateBranch: vi.fn(async () => {}),
  },
}));

describe('BranchSelector Component', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders current branch name when valid', () => {
    mockUseWorkspace.mockReturnValue({
      activeWorkspacePath: '/data/workspace/forge-ade',
      gitBranch: 'feature-login',
      gitFiles: [],
      setGitBranch: vi.fn(),
      refreshGitStatus: vi.fn(),
      refreshGitLog: vi.fn(),
      refreshFiles: vi.fn(),
    });

    render(<BranchSelector variant="composer" align="end" />);
    expect(screen.getByText('feature-login')).toBeDefined();
    expect(screen.getByTitle('Git branch')).toBeDefined();
  });

  it('does not render "unknown" when gitBranch is "unknown"', () => {
    mockUseWorkspace.mockReturnValue({
      activeWorkspacePath: '/data/workspace/forge-ade',
      gitBranch: 'unknown',
      gitFiles: [],
      setGitBranch: vi.fn(),
      refreshGitStatus: vi.fn(),
      refreshGitLog: vi.fn(),
      refreshFiles: vi.fn(),
    });

    render(<BranchSelector variant="composer" align="end" />);
    expect(screen.queryByText('unknown')).toBeNull();
    expect(screen.getByText('main')).toBeDefined();
  });

  it('switches branch and sets optimistic branch name', async () => {
    const setGitBranch = vi.fn();
    const refreshGitStatus = vi.fn();
    mockUseWorkspace.mockReturnValue({
      activeWorkspacePath: '/data/workspace/forge-ade',
      gitBranch: 'main',
      gitFiles: [],
      setGitBranch,
      refreshGitStatus,
      refreshGitLog: vi.fn(),
      refreshFiles: vi.fn(),
    });

    render(<BranchSelector variant="composer" align="end" />);
    const trigger = screen.getByTitle('Git branch');
    fireEvent.click(trigger);

    await waitFor(() => {
      expect(screen.getByText('feature-login')).toBeDefined();
    });

    const targetBranchBtn = screen.getByText('feature-login');
    fireEvent.click(targetBranchBtn);

    await waitFor(() => {
      expect(ApiBridge.gitCheckout).toHaveBeenCalledWith('/data/workspace/forge-ade', 'feature-login');
      expect(setGitBranch).toHaveBeenCalledWith('feature-login');
      expect(refreshGitStatus).toHaveBeenCalled();
    });
  });
});
