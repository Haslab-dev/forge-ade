import React from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { MultiFileDiffReviewBar } from './MultiFileDiffReviewBar';

describe('MultiFileDiffReviewBar', () => {
  const mockFiles = [
    { path: 'src/main.ts', additions: 15, deletions: 3 },
    { path: 'src/api.ts', additions: 8, deletions: 0 }
  ];

  it('renders modified files summary and chips', () => {
    const onSelect = vi.fn();
    render(
      <MultiFileDiffReviewBar
        files={mockFiles}
        onSelectFile={onSelect}
      />
    );

    expect(screen.getByText('2 files changed')).toBeInTheDocument();
    expect(screen.getByText('+23')).toBeInTheDocument();
    expect(screen.getAllByText('-3').length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText('main.ts')).toBeInTheDocument();
    expect(screen.getByText('api.ts')).toBeInTheDocument();

    fireEvent.click(screen.getByText('main.ts'));
    expect(onSelect).toHaveBeenCalledWith('src/main.ts');
  });

  it('triggers Accept All and Reject All callbacks', () => {
    const onAccept = vi.fn();
    const onReject = vi.fn();
    render(
      <MultiFileDiffReviewBar
        files={mockFiles}
        onSelectFile={vi.fn()}
        onAcceptAll={onAccept}
        onRejectAll={onReject}
      />
    );

    fireEvent.click(screen.getByText('Accept All'));
    expect(onAccept).toHaveBeenCalled();

    fireEvent.click(screen.getByText('Reject All'));
    expect(onReject).toHaveBeenCalled();
  });
});
