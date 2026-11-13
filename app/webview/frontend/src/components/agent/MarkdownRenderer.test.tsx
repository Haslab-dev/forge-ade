import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import React from 'react';
import { MarkdownRenderer } from './MarkdownRenderer';
import { useWorkspace } from '../../stores/workspaceStore';

vi.mock('../../stores/workspaceStore', () => {
  const openSideFile = vi.fn();
  return {
    useWorkspace: () => ({
      openSideFile,
      activeWorkspacePath: '/Users/hy4-mac-002/hasdev/works/inagov-ios',
    }),
  };
});

describe('MarkdownRenderer file path formatting', () => {
  it('renders clean file link for [`SwiftUIAppHeader.swift`](file:///...) without raw path', () => {
    const content = 'Changes Applied:\n[`SwiftUIAppHeader.swift`](file:///Users/hy4-mac-002/hasdev/works/inagov-ios/inagov-ios/Modules/Common/View/SwiftUIAppHeader.swift):';
    render(<MarkdownRenderer content={content} />);

    // Label should be the clean display name, not raw path
    const button = screen.getByText('SwiftUIAppHeader.swift');
    expect(button).toBeDefined();

    // Raw file:///... path should not be displayed in text
    expect(screen.queryByText(/file:\/\/\//)).toBeNull();

    // Clicking should invoke openSideFile
    fireEvent.click(button);
    expect(useWorkspace().openSideFile).toHaveBeenCalledWith(
      '/Users/hy4-mac-002/hasdev/works/inagov-ios/inagov-ios/Modules/Common/View/SwiftUIAppHeader.swift'
    );
  });

  it('renders standalone filename like RiwayatListView.swift as a clickable chip', () => {
    const content = 'Profile Sub-screens:\nRiwayatListView.swift';
    render(<MarkdownRenderer content={content} />);

    const chip = screen.getByText('RiwayatListView.swift');
    expect(chip).toBeDefined();

    fireEvent.click(chip);
    expect(useWorkspace().openSideFile).toHaveBeenCalledWith(
      expect.stringContaining('RiwayatListView.swift')
    );
  });

  it('renders absolute path cleanly relative to active workspace path', () => {
    const content = 'Updated file: /Users/hy4-mac-002/hasdev/works/inagov-ios/inagov-ios/Modules/Common/View/ScaffoldView.swift';
    render(<MarkdownRenderer content={content} />);

    // Should display relative path: inagov-ios/Modules/Common/View/ScaffoldView.swift
    const relText = screen.getByText('inagov-ios/Modules/Common/View/ScaffoldView.swift');
    expect(relText).toBeDefined();

    // Should not show the full prefix /Users/hy4-mac-002/hasdev/works/inagov-ios/inagov-ios...
    expect(screen.queryByText('/Users/hy4-mac-002/hasdev/works/inagov-ios/inagov-ios/Modules/Common/View/ScaffoldView.swift')).toBeNull();
  });

  it('renders bold-wrapped links like **[`build/`](file:///...)** as clickable chips without raw url', () => {
    const content = '- **[`build/`](file:///Users/hy4-mac-002/hasdev/works/inagov-ios/build)** — Wails packaging';
    render(<MarkdownRenderer content={content} />);

    const chip = screen.getByText('build/');
    expect(chip).toBeDefined();
    expect(screen.queryByText(/file:\/\/\//)).toBeNull();
  });
});
