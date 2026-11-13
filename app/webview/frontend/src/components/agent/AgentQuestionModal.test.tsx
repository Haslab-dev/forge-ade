import React from 'react';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { describe, it, expect, vi, afterEach } from 'vitest';
import { AgentQuestionModal, normalizeAgentQuestions } from './AgentQuestionModal';
import { WorkspaceProvider } from '../../stores/workspaceStore';

describe('normalizeAgentQuestions', () => {
  it('handles null, undefined and empty array', () => {
    expect(normalizeAgentQuestions(null)).toEqual([]);
    expect(normalizeAgentQuestions(undefined)).toEqual([]);
    expect(normalizeAgentQuestions([])).toEqual([]);
  });

  it('normalizes string options and recommended tags', () => {
    const raw = [
      {
        header: 'SCOPE',
        question: 'Design "Portfolio Design v3" has 4 screens. Which screens do I build?',
        subtitle: 'Please answer the following questions.',
        options: [
          'Home + Chat (Recommended)',
          'Home only'
        ]
      }
    ];

    const normalized = normalizeAgentQuestions(raw);
    expect(normalized).toHaveLength(1);
    expect(normalized[0].header).toBe('SCOPE');
    expect(normalized[0].question).toContain('Portfolio Design v3');
    expect(normalized[0].options).toHaveLength(2);
    expect(normalized[0].options[0].label).toBe('Home + Chat');
    expect(normalized[0].options[0].isRecommended).toBe(true);
    expect(normalized[0].options[1].label).toBe('Home only');
    expect(normalized[0].options[1].isRecommended).toBe(false);
  });

  it('normalizes object options with explicit recommended and descriptions', () => {
    const raw = [
      {
        id: 'q1',
        header: 'ARCH',
        question: 'Which framework to use?',
        options: [
          { label: 'React 19', description: 'Next-gen compiler', isRecommended: true },
          { label: 'Vue 3', description: 'Options or Composition API' }
        ]
      }
    ];

    const normalized = normalizeAgentQuestions(raw);
    expect(normalized[0].options[0].label).toBe('React 19');
    expect(normalized[0].options[0].description).toBe('Next-gen compiler');
    expect(normalized[0].options[0].isRecommended).toBe(true);
  });

  it('normalizes ACP permission options', () => {
    const raw = [
      {
        id: 'perm-1',
        header: 'PERMISSION',
        question: 'Run bash command?',
        options: [
          { optionId: 'allow_once', name: 'Allow once', kind: 'allow_once' },
          { optionId: 'allow_always', name: 'Allow always', kind: 'allow_always' },
          { optionId: 'deny', name: 'Deny', kind: 'deny' }
        ]
      }
    ];

    const normalized = normalizeAgentQuestions(raw);
    expect(normalized[0].options).toHaveLength(3);
    expect(normalized[0].options[0].label).toBe('Allow once');
    expect(normalized[0].options[0].isRecommended).toBe(true);
  });
});

describe('AgentQuestionModal component', () => {
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  const mockQuestions = [
    {
      id: 'q1',
      header: 'SCOPE',
      question: 'Design "Portfolio Design v3" has 4 screens. Which screens do I build?',
      subtitle: 'Please answer the following questions.',
      options: [
        {
          id: 'opt_1',
          label: 'Home + Chat',
          description: 'Build / (responsive desktop + mobile) and /chat (responsive).',
          isRecommended: true
        },
        {
          id: 'opt_2',
          label: 'Home only',
          description: 'Build / only (responsive). The composer links nowhere for now.'
        }
      ],
      allowCustom: true
    },
    {
      id: 'q2',
      header: 'THEME',
      question: 'Which default theme mode should be configured?',
      options: [
        { id: 'dark', label: 'Dark mode', isRecommended: true },
        { id: 'light', label: 'Light mode' }
      ]
    }
  ];

  it('renders nothing when no questions exist', () => {
    render(
      <WorkspaceProvider>
        <AgentQuestionModal isOpen={true} questions={[]} />
      </WorkspaceProvider>
    );

    expect(screen.queryByText('Agent questions')).not.toBeInTheDocument();
  });

  it('renders question modal dialog with header, segmented bar, scope, and options', () => {
    render(
      <WorkspaceProvider>
        <AgentQuestionModal
          isOpen={true}
          questions={mockQuestions}
        />
      </WorkspaceProvider>
    );

    // Header title
    expect(screen.getByText('Agent questions')).toBeInTheDocument();
    // Stepper counter
    expect(screen.getByText('1 / 2')).toBeInTheDocument();
    // Scope header
    expect(screen.getByText('SCOPE')).toBeInTheDocument();
    // Question text
    expect(screen.getByText(/Portfolio Design v3/i)).toBeInTheDocument();
    // Subtitle
    expect(screen.getByText('Please answer the following questions.')).toBeInTheDocument();
    // Options
    expect(screen.getByText('Home + Chat')).toBeInTheDocument();
    expect(screen.getByText(/responsive desktop \+ mobile/i)).toBeInTheDocument();
    expect(screen.getByText('Recommended')).toBeInTheDocument();
    expect(screen.getByText('Home only')).toBeInTheDocument();

    // Custom input option
    expect(screen.getByPlaceholderText('Type custom answer...')).toBeInTheDocument();

    // Footer keyboard hints
    expect(screen.getByText('move')).toBeInTheDocument();
    expect(screen.getByText('close')).toBeInTheDocument();
    // Action buttons
    expect(screen.getByText('Skip')).toBeInTheDocument();
    expect(screen.getByText('Next')).toBeInTheDocument();
  });

  it('allows selecting an option and advances to next question', () => {
    render(
      <WorkspaceProvider>
        <AgentQuestionModal
          isOpen={true}
          questions={mockQuestions}
        />
      </WorkspaceProvider>
    );

    // Select option 1
    const option1 = screen.getByText('Home + Chat');
    fireEvent.click(option1);

    // Click Next
    const nextBtn = screen.getByText('Next');
    fireEvent.click(nextBtn);

    // Should now be on question 2
    expect(screen.getByText('2 / 2')).toBeInTheDocument();
    expect(screen.getByText('THEME')).toBeInTheDocument();
    expect(screen.getByText('Dark mode')).toBeInTheDocument();
    // Next button should become Submit on the last question
    expect(screen.getByText('Submit')).toBeInTheDocument();
  });

  it('invokes onAnswer callback on submission', async () => {
    const handleAnswer = vi.fn();
    render(
      <WorkspaceProvider>
        <AgentQuestionModal
          isOpen={true}
          questions={[mockQuestions[0]]}
          onAnswer={handleAnswer}
        />
      </WorkspaceProvider>
    );

    // Click option 1
    fireEvent.click(screen.getByText('Home + Chat'));

    // Submit button
    const submitBtn = screen.getByText('Submit');
    fireEvent.click(submitBtn);

    expect(handleAnswer).toHaveBeenCalledWith({
      q1: 'Home + Chat'
    });
  });

  it('invokes onSkip callback when skip is clicked', () => {
    const handleSkip = vi.fn();
    render(
      <WorkspaceProvider>
        <AgentQuestionModal
          isOpen={true}
          questions={mockQuestions}
          onSkip={handleSkip}
        />
      </WorkspaceProvider>
    );

    const skipBtn = screen.getByText('Skip');
    fireEvent.click(skipBtn);

    expect(handleSkip).toHaveBeenCalledTimes(1);
  });
});
