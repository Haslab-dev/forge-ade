import { describe, expect, it, vi, beforeEach } from 'vitest';
import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { ModelProviderPaletteModal } from './ModelProviderPaletteModal';
import { useWorkspace } from '../../stores/workspaceStore';

vi.mock('../../lib/wails', async (importOriginal) => {
  const actual = await importOriginal<Record<string, any>>();
  return {
    ...actual,
    SetActiveModel: vi.fn().mockResolvedValue(undefined),
  };
});

describe('ModelProviderPaletteModal', () => {
  beforeEach(() => {
    useWorkspace.setState({
      currentModel: 'claude-3-7-sonnet-20250219',
      activeAgentId: 'agent-internal',
      providers: [
        {
          id: 'prov-anthropic',
          name: 'Anthropic Claude',
          baseUrl: 'https://api.anthropic.com/v1',
          enabled: true,
          models: ['claude-3-7-sonnet-20250219', 'claude-3-5-haiku-20241022']
        },
        {
          id: 'prov-openai',
          name: 'OpenAI',
          baseUrl: 'https://api.openai.com/v1',
          enabled: true,
          models: ['gpt-4o', 'o3-mini']
        }
      ],
      agents: [
        {
          id: 'agent-internal',
          name: 'ForgeADE Internal',
          type: 'internal',
          description: 'ForgeADE Internal Agent',
          icon: 'forge-ade',
          enabled: true,
          status: 'connected',
          provider: 'custom',
          supportedModels: ['claude-3-7-sonnet-20250219', 'gpt-4o']
        }
      ]
    });
  });

  it('renders nothing when isOpen is false', () => {
    const { container } = render(<ModelProviderPaletteModal isOpen={false} onClose={() => {}} />);
    expect(container.firstChild).toBeNull();
  });

  it('renders search input and models when isOpen is true', () => {
    render(<ModelProviderPaletteModal isOpen={true} onClose={() => {}} />);
    expect(screen.getByPlaceholderText(/Search models/i)).toBeInTheDocument();
    expect(screen.getByText('Claude 3.7 Sonnet')).toBeInTheDocument();
    expect(screen.getByText('GPT-4o (Omni)')).toBeInTheDocument();
  });

  it('filters models by search query', () => {
    render(<ModelProviderPaletteModal isOpen={true} onClose={() => {}} />);
    const input = screen.getByPlaceholderText(/Search models/i);
    fireEvent.change(input, { target: { value: 'gpt' } });

    expect(screen.getByText('GPT-4o (Omni)')).toBeInTheDocument();
    expect(screen.queryByText('Claude 3.7 Sonnet')).not.toBeInTheDocument();
  });

  it('selects model and triggers onClose', async () => {
    const onClose = vi.fn();
    render(<ModelProviderPaletteModal isOpen={true} onClose={onClose} />);

    const gptRow = screen.getByText('GPT-4o (Omni)');
    fireEvent.click(gptRow);

    await waitFor(() => {
      expect(useWorkspace.getState().currentModel).toBe('gpt-4o');
      expect(onClose).toHaveBeenCalled();
    });
  });
});
