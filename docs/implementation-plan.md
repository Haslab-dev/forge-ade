# Implementation Plan - RakitKode Go Porting & ForgeADE Agent Workspace 2.0

Port the RakitKode agent runtime from Bun/TS to native Go, integrate it into ForgeADE's backend architecture, and implement a modern KiloCode / Codex inspired Agent UI with multi-session, multi-role filters, tool/MCP/skill capabilities, @-mentions context autocomplete, and a memory-safe, lightweight Git graph.

---

## User Review Required

> [!IMPORTANT]
> **LLM Provider API Keys & Settings**: The ported Go LLM Router (`internal/llm`) will support 9+ providers (OpenAI, DeepSeek, Gemini, Anthropic, Ollama local, Groq, OpenRouter, Azure, Mistral) matching RakitKode's profile system (`~/.forge-ade/profiles.json`).

> [!WARNING]
> **Memory Leak Prevention Strategy**:
> - **Git Graph**: We will execute streaming native `git log --graph --format=...` commands lazily (pagination by 100 commits) rather than holding `go-git` full object trees in RAM. Diff blobs will be fetched on-demand.
> - **Search Indexing**: Standardize on bounded trie nodes and explicit ring buffers for terminal output to ensure RAM usage remains lightweight (<200MB instead of mult-gigabyte leaks).

---

## Open Questions

> [!NOTE]
> 1. **Default Agent Model**: Should local Ollama auto-detection be enabled by default when no cloud API keys are provided?
> 2. **Custom Agent System Prompts**: Would you like custom agent roles to be saved in workspace files (e.g. `.forge/agents/custom.json`) so they persist across team projects?

---

## Proposed Changes

### Backend (Go Engine & RakitKode Port)

#### [NEW] [llm/router.go](file:///Users/hy4-mac-002/hasdev/personal/forge-ade/internal/llm/router.go)
- Multi-provider LLM client in Go supporting streaming completions, tool calling, token usage tracking, and profile storage.
- Adapters for OpenAI, DeepSeek, Gemini, Anthropic, Ollama, Groq, OpenRouter, Azure, and Mistral.

#### [NEW] [agent/agent.go](file:///Users/hy4-mac-002/hasdev/personal/forge-ade/internal/agent/agent.go)
- Agent runtime orchestrating planning, reasoning, tool calls, and human-in-the-loop approvals (`y`/`n`/`yy`/`nn`).
- Role filter configurations:
  - **Coding**: Code generation, file editing, test execution, bug fixing.
  - **Planning**: Step-by-step task breakdown, to-do list creation, non-modifying research.
  - **Research**: Repository exploration, web/documentation retrieval, architecture synthesis.
  - **Custom**: User-defined system prompts, capabilities, and temperature settings.

#### [NEW] [agent/tools.go](file:///Users/hy4-mac-002/hasdev/personal/forge-ade/internal/agent/tools.go)
- Native Tool Registry (`read_file`, `write_file`, `edit_file`, `search_workspace`, `search_symbols`, `run_shell`, `git_status`, `git_diff`).

#### [NEW] [mcp/client.go](file:///Users/hy4-mac-002/hasdev/personal/forge-ade/internal/mcp/client.go)
- Model Context Protocol (MCP) client manager to register external STDIO/SSE MCP servers and expose tools to agents.

#### [NEW] [skills/skills.go](file:///Users/hy4-mac-002/hasdev/personal/forge-ade/internal/skills/skills.go)
- Skill loader discovering `SKILL.md` instructions from global (`~/.forge-ade/skills`) and workspace (`.agents/skills`) paths.

#### [NEW] [git/graph.go](file:///Users/hy4-mac-002/hasdev/personal/forge-ade/internal/git/graph.go)
- Lightweight Git service using native `git` CLI streaming.
- Lazy log pagination (`GetCommitGraph(offset, limit)`).
- Zero object graph materialization to maintain low memory footprint.

#### [MODIFY] [terminal/manager.go](file:///Users/hy4-mac-002/hasdev/personal/forge-ade/internal/terminal/manager.go)
- Extend session management to seamlessly track both interactive PTY Shell sessions and background AI Agent sessions.

#### [MODIFY] [app.go](file:///Users/hy4-mac-002/hasdev/personal/forge-ade/app.go)
- Expose Wails bindings for Agent Runtime, LLM Profiles, Skill discovery, MCP management, Git graph streaming, and @-mention file search context.

---

### Frontend (React & KiloCode-Inspired UI)

#### [NEW] [agent-screen.tsx](file:///Users/hy4-mac-002/hasdev/personal/forge-ade/frontend/src/panels/agent-screen.tsx)
- KiloCode / Codex inspired agent interface:
  - **Header & Stats**: Session header with live token usage meter (`Tokens ⬆️ 207.2K ⬆️ cache 588.0K ⬇️ 6.9K`).
  - **Task Checklist**: Dynamic To-dos list with progress bar.
  - **Reasoning Dropdown**: Collapsible internal thinking box (`Reasoning ∨`).
  - **Message Stream**: Markdown formatting, tool call execution banners, patch diff previews, clickable file links.
  - **Input Box**:
    - `@-Mention Autocomplete`: Typing `@` triggers popover fuzzy-searching workspace files and folders.
    - Role Filter Selector (`Coding`, `Planning`, `Research`, `Custom`).
    - Model & Provider selector (`OpenAI / gpt-4o`, `DeepSeek / deepseek-chat`, `Ollama / llama3.1`, etc.).
    - Action buttons for memory/tools/approval modes.

#### [NEW] [git-graph-panel.tsx](file:///Users/hy4-mac-002/hasdev/personal/forge-ade/frontend/src/panels/git-graph-panel.tsx)
- Lightweight Git Graph panel:
  - Visual branch commit nodes and commit list table.
  - Infinite scroll / lazy loading.
  - Quick commit detail and diff drawer.

#### [MODIFY] [sidebar.tsx](file:///Users/hy4-mac-002/hasdev/personal/forge-ade/frontend/src/components/sidebar.tsx)
- Add navigation buttons for **Agents Workspace** and **Git Graph**.

#### [MODIFY] [App.tsx](file:///Users/hy4-mac-002/hasdev/personal/forge-ade/frontend/src/App.tsx)
- Integrate multi-session shell and multi-session agent tab bar and routing.

---

## Verification Plan

### Automated Tests
- Run Go backend tests for agent tools, LLM router parsing, skill loading, and git graph pagination:
  ```bash
  go test ./internal/...
  ```
- Build frontend to verify TypeScript types and React component compilation:
  ```bash
  cd frontend && npm run build
  ```

### Manual Verification
- Test opening multiple shell sessions and multiple agent sessions simultaneously.
- Verify role filter selection (`Coding`, `Planning`, `Research`, `Custom`).
- Test `@` mention file context injection into agent prompts.
- Test tool execution and human-in-the-loop approval (`y`/`n`).
- Verify lightweight Git Graph rendering with zero RAM memory spikes on large repositories.
