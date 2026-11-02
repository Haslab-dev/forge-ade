import React from 'react';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
import { SidebarFileExplorer } from './SidebarFileExplorer';
import { useWorkspace } from '../../stores/workspaceStore';
import { FileItem } from '../../types';

describe('SidebarFileExplorer folder default collapse behavior', () => {
  const mockFiles: FileItem[] = [
    {
      id: 'src',
      name: 'src',
      path: '/data/workspace/forge-ade/src',
      type: 'folder',
      children: [
        {
          id: 'src/main.ts',
          name: 'main.ts',
          path: '/data/workspace/forge-ade/src/main.ts',
          type: 'file'
        }
      ]
    },
    {
      id: 'docs',
      name: 'docs',
      path: '/data/workspace/forge-ade/docs',
      type: 'folder',
      children: [
        {
          id: 'docs/README.md',
          name: 'README.md',
          path: '/data/workspace/forge-ade/docs/README.md',
          type: 'file'
        }
      ]
    },
    {
      id: 'package.json',
      name: 'package.json',
      path: '/data/workspace/forge-ade/package.json',
      type: 'file'
    }
  ];

  beforeEach(() => {
    useWorkspace.setState({
      activeWorkspacePath: '/data/workspace/forge-ade',
      files: mockFiles,
      gitFiles: []
    });
  });

  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it('defaults all folders to collapsed when opening project files', () => {
    const onBack = vi.fn();
    render(<SidebarFileExplorer onBack={onBack} />);

    // Root folder names and root files are displayed
    expect(screen.getByText('src')).toBeInTheDocument();
    expect(screen.getByText('docs')).toBeInTheDocument();
    expect(screen.getByText('package.json')).toBeInTheDocument();

    // Nested children inside folders must NOT be rendered by default (collapsed)
    expect(screen.queryByText('main.ts')).not.toBeInTheDocument();
    expect(screen.queryByText('README.md')).not.toBeInTheDocument();
  });

  it('allows clicking folder to expand and collapse, and provides collapse-all button', () => {
    const onBack = vi.fn();
    render(<SidebarFileExplorer onBack={onBack} />);

    // Initially collapsed
    expect(screen.queryByText('main.ts')).not.toBeInTheDocument();

    // Click 'src' folder to expand
    fireEvent.click(screen.getByText('src'));
    expect(screen.getByText('main.ts')).toBeInTheDocument();

    // Click collapse all folders button
    const collapseAllBtn = screen.getByTitle(/Collapse all folders/i);
    expect(collapseAllBtn).toBeInTheDocument();
    fireEvent.click(collapseAllBtn);

    // Now 'main.ts' is collapsed again
    expect(screen.queryByText('main.ts')).not.toBeInTheDocument();
  });
});
