import React from 'react';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { describe, it, expect, vi, afterEach } from 'vitest';
import { ToolExecutionCard, extractShellCommand } from './ToolBlockViews';
import type { ToolExecution } from '../../types';

describe('ToolExecutionCard - Shell calls parity', () => {
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it('extracts shell command from various tool representations', () => {
    const tool1: ToolExecution = {
      id: 't-1',
      toolName: 'bash',
      command: '{"command": "df -h | grep /dev"}',
      output: '/dev/root 16G 4.7G',
      status: 'completed'
    };
    expect(extractShellCommand(tool1)).toBe('df -h | grep /dev');

    const tool2: ToolExecution = {
      id: 't-2',
      toolName: 'exec_command',
      command: '/bin/bash -c "systemctl status nginx"',
      output: 'Active: active (running)',
      status: 'completed'
    };
    expect(extractShellCommand(tool2)).toBe('systemctl status nginx');

    const tool3: ToolExecution = {
      id: 't-3',
      toolName: 'run_terminal_cmd',
      command: 'git status -s\ngit log -n 1',
      output: 'clean',
      status: 'completed'
    };
    expect(extractShellCommand(tool3)).toBe('git status -s');
  });

  it('renders shell icon, command line, and accordion show more button', () => {
    const tool: ToolExecution = {
      id: 't-shell-1',
      toolName: 'bash',
      command: '{"command": "docker ps -a --format table"}',
      output: 'CONTAINER ID   IMAGE\n12345   redis',
      status: 'completed'
    };

    const handleToggle = vi.fn();

    const { rerender } = render(
      <ToolExecutionCard
        tool={tool}
        isOpen={false}
        onToggle={handleToggle}
      />
    );

    // Command text is rendered in the row header
    expect(screen.getByText('docker ps -a --format table')).toBeInTheDocument();

    // Accordion button "Show more" is present
    const toggleBtn = screen.getByTestId('show-more-toggle');
    expect(toggleBtn).toBeInTheDocument();
    expect(toggleBtn).toHaveTextContent('Show more');

    // Clicking row triggers onToggle
    fireEvent.click(toggleBtn);
    expect(handleToggle).toHaveBeenCalledTimes(1);

    // When isOpen is true, terminal card tool is revealed with output and "Hide" button
    rerender(
      <ToolExecutionCard
        tool={tool}
        isOpen={true}
        onToggle={handleToggle}
      />
    );

    expect(screen.getByTestId('show-more-toggle')).toHaveTextContent('Hide');
    expect(screen.getByText(/CONTAINER ID/i)).toBeInTheDocument();
  });
});
