import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import React from 'react';
import { OpenUIBlock } from './OpenUIBlock';
import { useWorkspace } from '../../stores/workspaceStore';

vi.mock('../../stores/workspaceStore', () => ({
  useWorkspace: {
    getState: vi.fn(() => ({
      activeSessionId: 'test-session-123',
      sendAgentPrompt: vi.fn(),
    })),
  },
}));

describe('OpenUIBlock Component', () => {
  it('renders an opui program with Card, Metric, Badge, and Button', () => {
    const code = `
root = Card([stat, badge, note, act], "Performance Summary", "System Metrics")
stat = Metric("CPU Usage", "12%", "down")
badge = Badge("Healthy", "success")
note = Text("All systems running smoothly.")
act = Button("Refresh", @send_prompt("Refresh status"))
`;

    const { container } = render(<OpenUIBlock code={code} streaming={false} />);

    // Check title and note
    expect(screen.getByText('Performance Summary')).toBeInTheDocument();
    expect(screen.getByText('System Metrics')).toBeInTheDocument();
    expect(screen.getByText('CPU Usage')).toBeInTheDocument();
    expect(screen.getByText('12%')).toBeInTheDocument();
    expect(screen.getByText('Healthy')).toBeInTheDocument();
    expect(screen.getByText('All systems running smoothly.')).toBeInTheDocument();
    expect(screen.getByText('Refresh')).toBeInTheDocument();

    // Verify white background is applied
    const whiteBlock = container.querySelector('.bg-white');
    expect(whiteBlock).toBeInTheDocument();
  });

  it('triggers sendAgentPrompt when an action button is clicked', () => {
    const mockSendAgentPrompt = vi.fn();
    (useWorkspace.getState as any).mockReturnValue({
      activeSessionId: 'test-session-123',
      sendAgentPrompt: mockSendAgentPrompt,
    });

    const code = `
root = Card([act], "Action Test")
act = Button("Click Me", @send_prompt("Triggered Action"))
`;

    render(<OpenUIBlock code={code} streaming={false} />);

    const button = screen.getByText('Click Me');
    fireEvent.click(button);

    expect(mockSendAgentPrompt).toHaveBeenCalledWith('Triggered Action');
  });

  it('renders smoke test snippet with neutral metric and list', () => {
    const code = `
root = Card([header, metrics, note, items, act], "OpenUI Smoke Test", "Rendering check for the e-voting workspace")
header = Heading("Generative UI is live", 2)
metrics = Stack([m1, m2, m3], "horizontal")
m1 = Metric("Components", "6", "up")
m2 = Metric("Render time", "42ms", "neutral")
m3 = Metric("Errors", "0", "down")
note = Text("This card exercises headings, metrics, badges, lists, and actions.")
items = List(["Card and Stack layout", "Metric and Badge primitives", "Interactive send_prompt button"])
act = Button("Run another test", @send_prompt("Render a second OpenUI test with a table"))
`;
    render(<OpenUIBlock code={code} streaming={false} />);
    expect(screen.getByText('OpenUI Smoke Test')).toBeInTheDocument();
    expect(screen.queryByText(/OpenUI block could not be rendered/i)).toBeNull();
  });

  it('renders error fallback on unrecoverable syntax error when not streaming', () => {
    const malformed = `invalid %%% syntax ???`;
    render(<OpenUIBlock code={malformed} streaming={false} />);

    expect(screen.getByText(/OpenUI block could not be rendered/i)).toBeInTheDocument();
  });
});
