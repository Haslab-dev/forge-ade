import React from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { InlineAiBar } from './InlineAiBar';
import { InlineAiSessionState } from './inlineAiTypes';

describe('InlineAiBar', () => {
  const baseState: InlineAiSessionState = {
    isOpen: true,
    status: 'idle',
    prompt: '',
    from: 0,
    to: 10,
    startLine: 1,
    endLine: 2,
    originalText: 'func Hello() {}',
    generatedText: ''
  };

  it('renders nothing when isOpen is false', () => {
    const { container } = render(
      <InlineAiBar
        state={{ ...baseState, isOpen: false }}
        activeModel="default"
        onPromptChange={vi.fn()}
        onSubmit={vi.fn()}
        onStop={vi.fn()}
        onAccept={vi.fn()}
        onReject={vi.fn()}
        onClose={vi.fn()}
      />
    );
    expect(container.firstChild).toBeNull();
  });

  it('renders input and quick action chips when idle', () => {
    const onSubmit = vi.fn();
    render(
      <InlineAiBar
        state={baseState}
        activeModel="default"
        onPromptChange={vi.fn()}
        onSubmit={onSubmit}
        onStop={vi.fn()}
        onAccept={vi.fn()}
        onReject={vi.fn()}
        onClose={vi.fn()}
      />
    );

    expect(screen.getByRole('dialog', { name: /inline ai assistant/i })).toBeInTheDocument();
    expect(screen.getByText('✨ Refactor')).toBeInTheDocument();
    expect(screen.getByText('🧪 Unit Tests')).toBeInTheDocument();

    fireEvent.click(screen.getByText('✨ Refactor'));
    expect(onSubmit).toHaveBeenCalledWith('Refactor this to be more clean, idiomatic, and readable');
  });

  it('renders Accept and Reject buttons when in diff status', () => {
    const onAccept = vi.fn();
    const onReject = vi.fn();

    render(
      <InlineAiBar
        state={{
          ...baseState,
          status: 'diff',
          generatedText: 'func Hello() { println("Hi") }'
        }}
        activeModel="default"
        onPromptChange={vi.fn()}
        onSubmit={vi.fn()}
        onStop={vi.fn()}
        onAccept={onAccept}
        onReject={onReject}
        onClose={vi.fn()}
      />
    );

    expect(screen.getByText(/Proposed Changes/i)).toBeInTheDocument();
    const acceptBtn = screen.getByRole('button', { name: /accept/i });
    const rejectBtn = screen.getByRole('button', { name: /reject/i });

    expect(acceptBtn).toBeInTheDocument();
    expect(rejectBtn).toBeInTheDocument();

    fireEvent.click(acceptBtn);
    expect(onAccept).toHaveBeenCalled();

    fireEvent.click(rejectBtn);
    expect(onReject).toHaveBeenCalled();
  });
});
