// Declarative registry of external agents reachable over ACP (Agent Client
// Protocol). Each entry is spawned as a child process and spoken to with
// JSON-RPC 2.0 over stdio — no per-agent integration code anywhere else.
//
// Launch commands (verified against upstream docs):
//   omp         → `omp acp`                       native ACP mode
//   opencode    → `npx -y opencode-ai acp`        https://opencode.ai/docs/acp/
//   codex       → `npx -y @agentclientprotocol/codex-acp`
//   claude-code → `npx -y @agentclientprotocol/claude-agent-acp`
//                 (official Claude Agent SDK adapter)
//   pi          → `pi-acp`
//   antigravity → `npx -y antigravity-acp`        community bridge; `agy` has
//                                                 no native ACP mode

export interface ExternalAgentDef {
  id: string;
  name: string;
  description: string;
  command: string;
  args: string[];
}

export const EXTERNAL_AGENTS: ExternalAgentDef[] = [
  {
    id: "omp",
    name: "Oh-My-Pi",
    description: "Oh-My-Pi coding agent via native ACP mode (omp acp)",
    command: "omp",
    args: ["acp"],
  },
  {
    id: "opencode",
    name: "OpenCode",
    description: "OpenCode agent via ACP (runs via npx, no install needed)",
    command: "npx",
    args: ["-y", "opencode-ai", "acp"],
  },
  {
    id: "codex",
    name: "Codex",
    description: "OpenAI Codex via the official codex-acp adapter (npx)",
    command: "npx",
    args: ["-y", "@agentclientprotocol/codex-acp"],
  },
  {
    id: "claude-code",
    name: "Claude Code",
    description: "Claude Agent SDK via the official claude-agent-acp adapter (npx)",
    command: "npx",
    args: ["-y", "@agentclientprotocol/claude-agent-acp"],
  },
  {
    id: "pi",
    name: "Pi",
    description: "Pi coding agent via the pi-acp adapter",
    command: "pi-acp",
    args: [],
  },
  {
    id: "antigravity",
    name: "Antigravity",
    description: "Google Antigravity via the antigravity-acp bridge (npx; wraps agy)",
    command: "npx",
    args: ["-y", "antigravity-acp"],
  },
];

export function findExternalAgent(id: string): ExternalAgentDef | undefined {
  return EXTERNAL_AGENTS.find((a) => a.id === id);
}
