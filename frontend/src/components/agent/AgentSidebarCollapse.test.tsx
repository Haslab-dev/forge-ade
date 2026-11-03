import React from 'react';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
import { AgentSidebar } from './AgentSidebar';
import { useWorkspace } from '../../stores/workspaceStore';
import { useSessionStore } from '../../stores/sessionStore';

describe('AgentSidebar project collapse behavior', () => {
  beforeEach(() => {
    useWorkspace.setState({
      agentWorkspacePath: '/data/workspace/forge-ade',
      activeWorkspacePath: '/data/workspace/forge-ade',
      activeSessionId: 'sess-1',
      activeTaskKind: 'forge',
      activeWorkspaces: [
        { path: '/data/workspace/forge-ade', name: 'forge-ade', type: 'local', lastActive: Date.now() },
        { path: '/data/workspace/other-running-project', name: 'other-running-project', type: 'local', lastActive: Date.now() },
        { path: '/data/workspace/idle-project', name: 'idle-project', type: 'local', lastActive: Date.now() }
      ],
      sessions: [
        {
          id: 'sess-1',
          title: 'Task in forge-ade',
          status: 'idle',
          createdAt: String(Date.now()),
          updatedAt: String(Date.now()),
          workspacePath: '/data/workspace/forge-ade',
          messages: [],
          model: 'default',
          agentId: 'forge'
        },
        {
          id: 'sess-2',
          title: 'Active Task in other-running-project',
          status: 'running',
          createdAt: String(Date.now()),
          updatedAt: String(Date.now()),
          workspacePath: '/data/workspace/other-running-project',
          messages: [],
          model: 'default',
          agentId: 'forge'
        },
        {
          id: 'sess-3',
          title: 'Finished Task in idle-project',
          status: 'completed',
          createdAt: String(Date.now()),
          updatedAt: String(Date.now()),
          workspacePath: '/data/workspace/idle-project',
          messages: [],
          model: 'default',
          agentId: 'forge'
        },
        {
          id: 'sess-4',
          title: 'Idle Task in General',
          status: 'completed',
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

  it('collapses idle other projects while keeping the opened project and running agent projects open', () => {
    render(<AgentSidebar />);

    // All project tasks are initially rendered
    expect(screen.getByText('Task in forge-ade')).toBeInTheDocument();
    expect(screen.getByText('Active Task in other-running-project')).toBeInTheDocument();
    expect(screen.getByText('Finished Task in idle-project')).toBeInTheDocument();
    expect(screen.getByText('Idle Task in General')).toBeInTheDocument();

    // Find the collapse icon button
    const collapseBtn = screen.getByTitle(/Collapse inactive projects/i);
    expect(collapseBtn).toBeInTheDocument();

    // Click collapse icon
    fireEvent.click(collapseBtn);

    // 1. Opened project (forge-ade) tasks remain open and visible
    expect(screen.getByText('Task in forge-ade')).toBeInTheDocument();

    // 2. Project with running agent (other-running-project) remains open and visible!
    expect(screen.getByText('Active Task in other-running-project')).toBeInTheDocument();

    // 3. Inactive projects (idle-project and General) tasks are collapsed and hidden
    expect(screen.queryByText('Finished Task in idle-project')).not.toBeInTheDocument();
    expect(screen.queryByText('Idle Task in General')).not.toBeInTheDocument();

    // Button title now indicates expanding all projects
    expect(screen.getByTitle(/Expand all projects/i)).toBeInTheDocument();

    // Click again to expand all
    fireEvent.click(screen.getByTitle(/Expand all projects/i));
    expect(screen.getByText('Finished Task in idle-project')).toBeInTheDocument();
    expect(screen.getByText('Idle Task in General')).toBeInTheDocument();
  });

  it('keeps projects with running terminal CLI agents open when collapsing inactive projects', () => {
    useSessionStore.setState({
      sessions: [
        {
          id: 'term-sess-1',
          agentId: 'claude-code',
          agentName: 'Claude Code CLI',
          status: 'running',
          title: 'CLI running task in cli-project',
          workspacePath: '/data/workspace/cli-project',
          workingDirectory: '/data/workspace/cli-project',
          executable: '/bin/bash',
          createdAt: Date.now()
        }
      ]
    });

    render(<AgentSidebar />);

    expect(screen.getByText('CLI running task in cli-project')).toBeInTheDocument();

    const collapseBtn = screen.getByTitle(/Collapse inactive projects/i);
    fireEvent.click(collapseBtn);

    // Opened project (forge-ade) stays open
    expect(screen.getByText('Task in forge-ade')).toBeInTheDocument();
    // Project with running CLI agent stays open
    expect(screen.getByText('CLI running task in cli-project')).toBeInTheDocument();
    // Idle project is collapsed
    expect(screen.queryByText('Finished Task in idle-project')).not.toBeInTheDocument();
  });

  it('hides inactive sessions within open project when collapsed, only opening active session', () => {
    useWorkspace.setState({
      agentWorkspacePath: '/data/workspace/forge-ade',
      activeWorkspacePath: '/data/workspace/forge-ade',
      activeSessionId: 'sess-active',
      activeTaskKind: 'forge',
      activeWorkspaces: [
        { path: '/data/workspace/forge-ade', name: 'forge-ade', type: 'local', lastActive: Date.now() }
      ],
      sessions: [
        {
          id: 'sess-active',
          title: 'Active Working Session',
          status: 'idle',
          createdAt: String(Date.now()),
          updatedAt: String(Date.now()),
          workspacePath: '/data/workspace/forge-ade',
          messages: [],
          model: 'default',
          agentId: 'forge'
        },
        {
          id: 'sess-inactive',
          title: 'Older Inactive Session',
          status: 'completed',
          createdAt: String(Date.now() - 100000),
          updatedAt: String(Date.now() - 100000),
          workspacePath: '/data/workspace/forge-ade',
          messages: [],
          model: 'default',
          agentId: 'forge'
        }
      ]
    });

    render(<AgentSidebar />);

    // Initially both are visible
    expect(screen.getByText('Active Working Session')).toBeInTheDocument();
    expect(screen.getByText('Older Inactive Session')).toBeInTheDocument();

    // Click collapse
    const collapseBtn = screen.getByTitle(/Collapse inactive projects/i);
    fireEvent.click(collapseBtn);

    // Active session remains visible
    expect(screen.getByText('Active Working Session')).toBeInTheDocument();

    // Inactive session is closed / hidden
    expect(screen.queryByText('Older Inactive Session')).not.toBeInTheDocument();

    // Click expand all
    fireEvent.click(screen.getByTitle(/Expand all projects/i));
    expect(screen.getByText('Older Inactive Session')).toBeInTheDocument();
  });
});
