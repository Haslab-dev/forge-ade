// Declarative registry of external agents reachable over ACP (Agent Client
// Protocol). Each entry is spawned as a child process and spoken to with
// JSON-RPC 2.0 over stdio — no per-agent integration code anywhere else.
//
// Commands follow the official ACP ecosystem adapters:
//   omp        → `omp acp`                (Oh-My-Pi native ACP mode)
//   opencode   → `opencode acp`
//   codex      → `codex-acp`              (agentclientprotocol/codex-acp)
//   claude-code→ `claude-code-acp`        (@zed-industries/claude-code-acp)
//   pi         → `pi-acp`
//   gemini     → deprecated; replaced by Antigravity (agy)
//
// Note: `agy` has no native ACP mode — the community `antigravity-acp`
// bridge (github.com/shubzkothekar/antigravity-acp) wraps it.
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
    description: "OpenCode agent via ACP (opencode acp)",
    command: "opencode",
    args: ["acp"],
  },
  {
    id: "codex",
    name: "Codex",
    description: "OpenAI Codex via the codex-acp adapter",
    command: "codex-acp",
    args: [],
  },
  {
    id: "claude-code",
    name: "Claude Code",
    description: "Claude Code via the claude-code-acp adapter",
    command: "claude-code-acp",
    args: [],
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
    description: "Google Antigravity via the antigravity-acp bridge (npm i -g antigravity-acp)",
    command: "antigravity-acp",
    args: [],
  },
];

export function findExternalAgent(id: string): ExternalAgentDef | undefined {
  return EXTERNAL_AGENTS.find((a) => a.id === id);
}
