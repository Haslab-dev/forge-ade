import { describe, expect, it } from 'vitest';
import React from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
import { FoldedToolTraces, summarizeToolsList } from './FoldedToolTraces';
import type { ToolExecution } from '../../types';

describe('FoldedToolTraces', () => {
  it('summarizes multiple tool kinds accurately', () => {
    const tools: ToolExecution[] = [
      { id: '1', toolName: 'bash', command: 'ls', output: '', status: 'completed' },
      { id: '2', toolName: 'read_file', command: 'cat a.txt', output: '', status: 'completed' },
      { id: '3', toolName: 'read_file', command: 'cat b.txt', output: '', status: 'completed' },
      { id: '4', toolName: 'edit_file', command: 'edit', output: '', status: 'completed' },
      { id: '5', toolName: 'grep', command: 'search', output: '', status: 'completed' }
    ];

    const result = summarizeToolsList(tools);
    expect(result.summary).toBe('Ran 1 command · Read 2 files · Edited 1 file · Ran 1 search');
    expect(result.hasRunning).toBe(false);
    expect(result.hasFailed).toBe(false);
  });

  it('renders summary bar and toggles children on click (default open)', () => {
    const tools: ToolExecution[] = [
      { id: '1', toolName: 'bash', command: 'ls', output: '', status: 'completed' },
      { id: '2', toolName: 'bash', command: 'pwd', output: '', status: 'completed' }
    ];

    render(
      <FoldedToolTraces tools={tools}>
        <div data-testid="tool-child-1">Tool 1</div>
        <div data-testid="tool-child-2">Tool 2</div>
      </FoldedToolTraces>
    );

    expect(screen.getByText(/Ran 2 commands/i)).toBeInTheDocument();
    // Default open
    expect(screen.getByTestId('tool-child-1')).toBeInTheDocument();
    expect(screen.getByTestId('tool-child-2')).toBeInTheDocument();

    // Click toggle to close
    fireEvent.click(screen.getByRole('button', { name: /Ran 2 commands/i }));
    expect(screen.queryByTestId('tool-child-1')).toBeNull();
    expect(screen.queryByTestId('tool-child-2')).toBeNull();
  });

  it('respects defaultExpanded=false when explicitly passed', () => {
    const tools: ToolExecution[] = [
      { id: '1', toolName: 'bash', command: 'ls', output: '', status: 'completed' },
      { id: '2', toolName: 'bash', command: 'pwd', output: '', status: 'completed' }
    ];

    render(
      <FoldedToolTraces tools={tools} defaultExpanded={false}>
        <div data-testid="tool-child-1">Tool 1</div>
      </FoldedToolTraces>
    );

    expect(screen.queryByTestId('tool-child-1')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: /Ran 2 commands/i }));
    expect(screen.getByTestId('tool-child-1')).toBeInTheDocument();
  });
});
