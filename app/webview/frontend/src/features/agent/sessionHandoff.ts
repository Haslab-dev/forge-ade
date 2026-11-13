import { AgentSession, TerminalSessionRecord } from '../../types';

export interface CliHandoffSeed {
  workingDirectory: string;
  workspacePath?: string;
  initialPrompt?: string;
  agentName: string;
}

export interface GuiHandoffResult {
  session: AgentSession;
  summary: string;
}

/**
 * Converts a Terminal CLI session (e.g. claude, codex, opencode, agy) into a structured ForgeADE GUI Agent Session.
 */
export function handoffCliToGui(
  terminalSession: TerminalSessionRecord,
  summaryPrompt?: string
): GuiHandoffResult {
  const sessionId = `handoff-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`;
  const title = terminalSession.title || `${terminalSession.agentName} Session`;
  const prompt = summaryPrompt || `Context handed off from terminal session "${terminalSession.agentName}". Ready to continue development.`;
  const now = new Date().toISOString();

  const session: AgentSession = {
    id: sessionId,
    title: `${title} (GUI)`,
    model: 'default',
    agentId: 'forge-agent',
    workspacePath: terminalSession.workspacePath || terminalSession.workingDirectory,
    status: 'idle',
    createdAt: now,
    updatedAt: now,
    messages: [
      {
        id: `msg-${Date.now()}-1`,
        role: 'user',
        content: prompt,
        timestamp: now
      }
    ],
    tasks: [],
    progress: {
      current_goal: `Continue work from ${terminalSession.agentName} terminal session`
    }
  };

  return {
    session,
    summary: prompt
  };
}

/**
 * Extracts context and goal from a GUI Agent Session to seed an external CLI agent inside a PTY terminal.
 */
export function handoffGuiToCli(
  agentSession: AgentSession,
  targetCliName = 'claude'
): CliHandoffSeed {
  const goal = agentSession.progress?.current_goal;
  const lastUserMsg = agentSession.messages
    .filter(m => m.role === 'user')
    .pop()?.content;

  const initialPrompt = goal || lastUserMsg || `Continue engineering tasks in ${agentSession.title}`;
  const workingDirectory = agentSession.workspacePath || '.';

  return {
    workingDirectory,
    workspacePath: agentSession.workspacePath,
    initialPrompt,
    agentName: targetCliName
  };
}
