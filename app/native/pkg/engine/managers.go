package engine

import (
	"path/filepath"

	"github.com/Haslab-dev/RakitKode/agent"
	"github.com/Haslab-dev/RakitKode/events"
	"github.com/Haslab-dev/RakitKode/llm"
	"github.com/Haslab-dev/RakitKode/mcp"
	"github.com/Haslab-dev/RakitKode/memory"
	"github.com/Haslab-dev/RakitKode/plugins"
	"github.com/Haslab-dev/RakitKode/search"
	"github.com/Haslab-dev/RakitKode/skills"
	"github.com/Haslab-dev/RakitKode/store"
	"github.com/Haslab-dev/RakitKode/tools"
	"github.com/hasdev/forge-ade/core/terminal"
)

// managers bundles the RakitKode managers wired for the native app.
type managers struct {
	Search *search.SearchManager
	Tools  *tools.Registry
	Skills *skills.Manager
	Plugin *plugins.Manager
	Memory *memory.Manager
	MCP    *mcp.Manager
	Shells *terminal.Manager
	Agent  *agent.Manager
}

// agentShellManager adapts the core PTY terminal manager to the engine's
// ShellManager interface — CreateShell returns the concrete terminal session,
// which satisfies agent.ShellSession via GetID. Mirrors the webview app's
// internal/app/shellmanager.go adapter.
type agentShellManager struct {
	*terminal.Manager
}

func (a agentShellManager) CreateShell(name, folder string) (agent.ShellSession, error) {
	return a.Manager.CreateShell(name, folder)
}

// WrapShellManager wraps a terminal.Manager as an agent.ShellManager; nil for
// a nil manager so the agent falls back to one-shot shells.
func WrapShellManager(m *terminal.Manager) agent.ShellManager {
	if m == nil {
		return nil
	}
	return agentShellManager{Manager: m}
}

// wireManagers builds every RakitKode manager the webview bootstrap wires,
// sharing one event bus so agent:* topics flow to the UI bridges.
func wireManagers(dataDir string, bus *events.Bus, llmClient *llm.LLMClient) *managers {
	store.SetRoot(dataDir)

	si := search.NewSearchManager()
	toolReg := tools.NewRegistry(si)
	skillMgr := skills.NewManager(filepath.Join(dataDir, "skills"))
	pluginMgr := plugins.NewManager(dataDir, bus)
	memoryMgr := memory.New(dataDir)
	memoryMgr.SetGlobalDir(dataDir)
	mcpMgr := mcp.NewManager(dataDir)
	shellMgr := terminal.NewManager(bus)

	agentMgr := agent.NewManagerWithOptions(agent.ManagerOptions{
		LLMClient:    llmClient,
		ToolRegistry: toolReg,
		Skills:       skillMgr,
		Plugins:      pluginMgr,
		MCP:          mcpMgr,
		Shell:        WrapShellManager(shellMgr),
		Bus:          bus,
		DataDir:      dataDir,
	})
	agentMgr.SetMemoryManager(memoryMgr)

	return &managers{
		Search: si,
		Tools:  toolReg,
		Skills: skillMgr,
		Plugin: pluginMgr,
		Memory: memoryMgr,
		MCP:    mcpMgr,
		Shells: shellMgr,
		Agent:  agentMgr,
	}
}
