import { describe, expect, it, vi } from 'vitest';
import React from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
import { TurnChangesCard, extractTurnChanges } from './TurnChangesCard';
import type { ToolExecution, FileDiff } from '../../types';

describe('TurnChangesCard', () => {
  it('extracts modified files from tool diffs and commands correctly', () => {
    const mockDiff: FileDiff = {
      id: 'diff-1',
      filePath: 'src/main.go',
      fileName: 'main.go',
      originalContent: '',
      modifiedContent: '',
      additions: 12,
      deletions: 3,
      status: 'pending',
      timestamp: '2026-11-03T00:00:00Z'
    };

    const tools: ToolExecution[] = [
      {
        id: 'tool-1',
        toolName: 'edit_file',
        command: '{"path": "src/main.go"}',
        status: 'completed',
        diff: mockDiff
      },
      {
        id: 'tool-2',
        toolName: 'write_file',
        command: '{"path": "src/utils.go", "content": "package main\\nfunc helper() {}\\n"}',
        status: 'completed'
      },
      {
        id: 'tool-3',
        toolName: 'read_file',
        command: '{"path": "README.md"}',
        status: 'completed'
      }
    ];

    const changes = extractTurnChanges(tools);
    expect(changes).toHaveLength(2);

    const mainChange = changes.find(c => c.filePath === 'src/main.go');
    expect(mainChange).toBeDefined();
    expect(mainChange?.additions).toBe(12);
    expect(mainChange?.deletions).toBe(3);

    const utilsChange = changes.find(c => c.filePath === 'src/utils.go');
    expect(utilsChange).toBeDefined();
    expect(utilsChange?.additions).toBe(3);
    expect(utilsChange?.deletions).toBe(0);
  });

  it('renders null when there are no file modifications', () => {
    const tools: ToolExecution[] = [
      {
        id: 'tool-1',
        toolName: 'read_file',
        command: '{"path": "README.md"}',
        status: 'completed'
      }
    ];

    const { container } = render(<TurnChangesCard tools={tools} />);
    expect(container.firstChild).toBeNull();
  });

  it('toggles expansion and triggers onOpenDiff callback', () => {
    const mockDiff: FileDiff = {
      id: 'diff-1',
      filePath: 'src/server.go',
      fileName: 'server.go',
      originalContent: '',
      modifiedContent: '',
      additions: 5,
      deletions: 1,
      status: 'pending',
      timestamp: '2026-11-03T00:00:00Z'
    };

    const tools: ToolExecution[] = [
      {
        id: 'tool-1',
        toolName: 'edit_file',
        command: '{"path": "src/server.go"}',
        status: 'completed',
        diff: mockDiff
      }
    ];

    const onOpenDiff = vi.fn();
    const onOpenFile = vi.fn();

    render(
      <TurnChangesCard
        tools={tools}
        onOpenDiff={onOpenDiff}
        onOpenFile={onOpenFile}
      />
    );

    expect(screen.getByText(/Modified 1 file in this turn/i)).toBeInTheDocument();
    expect(screen.getByText('+5')).toBeInTheDocument();
    expect(screen.getByText('-1')).toBeInTheDocument();

    // Expand
    fireEvent.click(screen.getByText(/Modified 1 file in this turn/i));
    expect(screen.getByText('server.go')).toBeInTheDocument();

    // Click file to open diff
    fireEvent.click(screen.getByText('server.go'));
    expect(onOpenDiff).toHaveBeenCalledWith(mockDiff);
  });
});
