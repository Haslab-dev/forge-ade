import React from 'react';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { describe, it, expect, vi, afterEach } from 'vitest';
import { ToolExecutionCard, extractShellCommand, parseToolArgs, cleanArgPath } from './ToolBlockViews';
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

  it('parses Antigravity PascalCase tool parameters accurately', () => {
    const rawRead = '{"Path": "/Users/hy4-mac-002/hasdev/works/inagov-ios/inagov-ios/Modules/Common/View/SwiftUIAppHeader.swift"}';
    const parsedRead = parseToolArgs(rawRead, 'read_file');
    expect(parsedRead.path).toBe('/Users/hy4-mac-002/hasdev/works/inagov-ios/inagov-ios/Modules/Common/View/SwiftUIAppHeader.swift');

    const rawEdit = '{"Path": "/app/index.ts", "Target": "const a = 1;", "Replacement": "const a = 2;"}';
    const parsedEdit = parseToolArgs(rawEdit, 'edit_file');
    expect(parsedEdit.path).toBe('/app/index.ts');
    expect(parsedEdit.oldString).toBe('const a = 1;');
    expect(parsedEdit.newString).toBe('const a = 2;');

    const rawRun = '{"CommandLine": "npm test", "WaitMsBeforeAsync": 5000}';
    const parsedRun = parseToolArgs(rawRun, 'run_command');
    expect(parsedRun.cmd).toBe('npm test');

    const rawWrite = '{"Path": "/app/test.txt", "Content": "hello world"}';
    const parsedWrite = parseToolArgs(rawWrite, 'write_file');
    expect(parsedWrite.path).toBe('/app/test.txt');
    expect(parsedWrite.content).toBe('hello world');
  });

  it('cleans malformed Path prefixes without corrupting absolute paths', () => {
    expect(cleanArgPath('Path":"/Users/user/file.swift')).toBe('/Users/user/file.swift');
    expect(cleanArgPath('{"Path": "/Users/user/file.swift"}')).toBe('/Users/user/file.swift');
    expect(cleanArgPath('filepath: "src/main.ts"')).toBe('src/main.ts');
  });

  it('renders shell icon, command line, and accordion toggle', () => {
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

    // Row acts as the accordion button
    const rowButton = screen.getByRole('button');
    expect(rowButton).toBeInTheDocument();

    // Clicking row triggers onToggle
    fireEvent.click(rowButton);
    expect(handleToggle).toHaveBeenCalledTimes(1);

    // When isOpen is true, terminal card tool is revealed with output
    rerender(
      <ToolExecutionCard
        tool={tool}
        isOpen={true}
        onToggle={handleToggle}
      />
    );

    expect(screen.getByText(/CONTAINER ID/i)).toBeInTheDocument();
  });
});
