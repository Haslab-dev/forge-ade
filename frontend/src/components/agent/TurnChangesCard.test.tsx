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

  it('extracts modified files from Antigravity PascalCase parameters without Path prefix corruption', () => {
    const tools: ToolExecution[] = [
      {
        id: 'agy-tool-1',
        toolName: 'edit_file',
        command: '{"Path": "/Users/hy4-mac-002/hasdev/works/inagov-ios/inagov-ios/Modules/Common/View/SwiftUIAppHeader.swift", "Target": "let old = 1", "Replacement": "let new = 2"}',
        status: 'completed',
        output: '@@ -10,3 +10,3 @@\n-let old = 1\n+let new = 2\n'
      }
    ];

    const changes = extractTurnChanges(tools, '/Users/hy4-mac-002/hasdev/works/inagov-ios');
    expect(changes).toHaveLength(1);
    expect(changes[0].filePath).toBe('inagov-ios/Modules/Common/View/SwiftUIAppHeader.swift');
    expect(changes[0].fileName).toBe('SwiftUIAppHeader.swift');
    expect(changes[0].additions).toBe(1);
    expect(changes[0].deletions).toBe(1);
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

  it('renders Revert Turn button when originalContent is available and restores file on click', async () => {
    const { ApiBridge } = await import('../../services/apiBridge');
    const writeFileSpy = vi.spyOn(ApiBridge, 'writeFile').mockResolvedValue(true as any);

    const mockDiff: FileDiff = {
      id: 'diff-revert-1',
      filePath: 'src/config.ts',
      fileName: 'config.ts',
      originalContent: 'export const port = 3000;',
      modifiedContent: 'export const port = 8080;',
      additions: 1,
      deletions: 1,
      status: 'pending',
      timestamp: '2026-11-03T00:00:00Z'
    };

    const tools: ToolExecution[] = [
      {
        id: 'tool-rev-1',
        toolName: 'edit_file',
        command: '{"path": "src/config.ts"}',
        status: 'completed',
        diff: mockDiff
      }
    ];

    render(
      <TurnChangesCard
        tools={tools}
        workspacePath="/data/workspace/forge-ade"
      />
    );

    const revertBtn = screen.getByTitle(/Revert all file modifications/i);
    expect(revertBtn).toBeInTheDocument();
    expect(revertBtn).toHaveTextContent('Revert Turn');

    fireEvent.click(revertBtn);

    expect(writeFileSpy).toHaveBeenCalledWith(
      '/data/workspace/forge-ade/src/config.ts',
      'export const port = 3000;'
    );
  });
});
