import React from 'react';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
import { AgentSidebar } from './AgentSidebar';
import { WorkspaceProvider, useWorkspace } from '../../stores/workspaceStore';
import { useSessionStore } from '../../stores/sessionStore';

describe('AgentSidebar project collapse behavior', () => {
  beforeEach(() => {
    useWorkspace.setState({
      agentWorkspacePath: '/data/workspace/forge-ade',
      activeWorkspacePath: '/data/workspace/forge-ade',
      activeWorkspaces: [
        { path: '/data/workspace/forge-ade', name: 'forge-ade', type: 'local', lastActive: Date.now() },
        { path: '/data/workspace/other-project', name: 'other-project', type: 'local', lastActive: Date.now() }
      ],
      sessions: [
        {
          id: 'sess-1',
          title: 'Task in forge-ade',
          status: 'running',
          createdAt: String(Date.now()),
          updatedAt: String(Date.now()),
          workspacePath: '/data/workspace/forge-ade',
          messages: [],
          model: 'default',
          agentId: 'forge'
        },
        {
          id: 'sess-2',
          title: 'Task in General',
          status: 'running',
          createdAt: String(Date.now()),
          updatedAt: String(Date.now()),
          workspacePath: 'general',
          messages: [],
          model: 'default',
          agentId: 'forge'
        }
      ]
    });
  });

  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it('collapses other projects while keeping the opened project open', () => {
    render(<AgentSidebar />);

    // Both project tasks are initially rendered
    expect(screen.getByText('Task in forge-ade')).toBeInTheDocument();
    expect(screen.getByText('Task in General')).toBeInTheDocument();

    // Find the collapse icon button
    const collapseBtn = screen.getByTitle(/Collapse other projects/i);
    expect(collapseBtn).toBeInTheDocument();

    // Click collapse icon
    fireEvent.click(collapseBtn);

    // Opened project (forge-ade) tasks remain open and visible
    expect(screen.getByText('Task in forge-ade')).toBeInTheDocument();

    // Other project (General) tasks are now collapsed
    expect(screen.queryByText('Task in General')).not.toBeInTheDocument();

    // Button title now indicates expanding all projects
    expect(screen.getByTitle(/Expand all projects/i)).toBeInTheDocument();

    // Click again to expand
    fireEvent.click(screen.getByTitle(/Expand all projects/i));
    expect(screen.getByText('Task in General')).toBeInTheDocument();
  });
});
