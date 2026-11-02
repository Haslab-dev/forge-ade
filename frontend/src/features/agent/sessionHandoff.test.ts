import { describe, it, expect } from 'vitest';
import { handoffCliToGui, handoffGuiToCli } from './sessionHandoff';
import { TerminalSessionRecord, AgentSession } from '../../types';

describe('sessionHandoff', () => {
  it('converts CLI terminal session to GUI session', () => {
    const termSession: TerminalSessionRecord = {
      id: 'term-123',
      agentId: 'claude-cli',
      agentName: 'Claude Code',
      title: 'Fix issue #42',
      workingDirectory: '/data/workspace/project-x',
      workspacePath: '/data/workspace/project-x',
      executable: 'claude',
      createdAt: Date.now(),
      status: 'running'
    };

    const { session, summary } = handoffCliToGui(termSession);
    expect(session.id).toMatch(/^handoff-/);
    expect(session.title).toContain('Fix issue #42');
    expect(session.workspacePath).toBe('/data/workspace/project-x');
    expect(session.messages).toHaveLength(1);
    expect(summary).toContain('Claude Code');
  });

  it('converts GUI session to CLI seed with active goal', () => {
    const guiSession: AgentSession = {
      id: 'sess-456',
      title: 'Refactor Auth Pipeline',
      model: 'default',
      agentId: 'forge-agent',
      workspacePath: '/data/workspace/forge-ade',
      status: 'idle',
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
      messages: [
        {
          id: 'm1',
          role: 'user',
          content: 'Please refactor authentication tokens',
          timestamp: new Date().toISOString()
        }
      ],
      tasks: [],
      progress: {
        current_goal: 'Complete JWT migration to purego'
      }
    };

    const seed = handoffGuiToCli(guiSession, 'opencode');
    expect(seed.agentName).toBe('opencode');
    expect(seed.workingDirectory).toBe('/data/workspace/forge-ade');
    expect(seed.initialPrompt).toBe('Complete JWT migration to purego');
  });
});
