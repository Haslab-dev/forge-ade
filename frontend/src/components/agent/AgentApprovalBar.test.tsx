import { describe, expect, it, vi } from 'vitest';
import React from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
import { AgentApprovalBar } from './AgentApprovalBar';

// Mock Wails call
vi.mock('../../lib/wails', () => ({
  RespondAgentApproval: vi.fn().mockResolvedValue(undefined)
}));

describe('AgentApprovalBar', () => {
  it('renders nothing when tools array is empty', () => {
    const { container } = render(<AgentApprovalBar tools={[]} goSessionId="sess-1" />);
    expect(container.firstChild).toBeNull();
  });

  it('renders single approval bar with tool summary and approve button', async () => {
    const { RespondAgentApproval } = await import('../../lib/wails');
    render(
      <AgentApprovalBar
        goSessionId="sess-1"
        tools={[
          {
            id: 'call-1',
            name: 'bash',
            arguments: JSON.stringify({ command: 'npm run test' })
          }
        ]}
      />
    );

    expect(screen.getByText(/Approval Needed:/i)).toBeInTheDocument();
    expect(screen.getByText(/Run command: npm run test/i)).toBeInTheDocument();

    const approveBtn = screen.getByRole('button', { name: /Approve/i });
    fireEvent.click(approveBtn);

    expect(RespondAgentApproval).toHaveBeenCalledWith('sess-1', true, false);
  });

  it('renders grouped approval bar when multiple tools are pending', async () => {
    const { RespondAgentApproval } = await import('../../lib/wails');
    render(
      <AgentApprovalBar
        goSessionId="sess-1"
        tools={[
          {
            id: 'call-1',
            name: 'edit_file',
            arguments: JSON.stringify({ path: 'src/main.ts' })
          },
          {
            id: 'call-2',
            name: 'write_file',
            arguments: JSON.stringify({ path: 'src/test.ts' })
          }
        ]}
      />
    );

    expect(screen.getByText(/2 actions require approval:/i)).toBeInTheDocument();
    const approveAllBtn = screen.getByRole('button', { name: /Approve all/i });
    fireEvent.click(approveAllBtn);

    expect(RespondAgentApproval).toHaveBeenCalledWith('sess-1', true, false);
  });
});
