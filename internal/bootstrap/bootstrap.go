package bootstrap

import (
	"fmt"
	"path/filepath"

	"github.com/Haslab-dev/RakitKode/agent"
	"github.com/Haslab-dev/RakitKode/decisions"
	"github.com/Haslab-dev/RakitKode/events"
	"github.com/Haslab-dev/RakitKode/llm"
	"github.com/Haslab-dev/RakitKode/mcp"
	"github.com/Haslab-dev/RakitKode/memory"
	"github.com/Haslab-dev/RakitKode/plugins"
	"github.com/Haslab-dev/RakitKode/search"
	"github.com/Haslab-dev/RakitKode/skills"
	"github.com/Haslab-dev/RakitKode/store"
	"github.com/Haslab-dev/RakitKode/tools"
	"github.com/hasdev/forge-ade/internal/acp"
	"github.com/hasdev/forge-ade/internal/agentsession"
	"github.com/hasdev/forge-ade/internal/app"
	"github.com/hasdev/forge-ade/internal/automations"
	"github.com/hasdev/forge-ade/internal/board"
	"github.com/hasdev/forge-ade/internal/explorer"
	"github.com/hasdev/forge-ade/internal/git"
	"github.com/hasdev/forge-ade/internal/remotefs"
	"github.com/hasdev/forge-ade/internal/terminal"
	"github.com/hasdev/forge-ade/internal/watcher"
	"github.com/hasdev/forge-ade/internal/workspace"
)

// BootOptions configures application initialization.
type BootOptions struct {
	DataDir string
}

// Boot initializes all domain subsystems, sets up infrastructure, wires
// dependencies, and returns a configured *app.App composition root.
func Boot(opts BootOptions) (*app.App, error) {
	dataDir := opts.DataDir
	if dataDir == "" {
		dataDir = app.GetDataDir()
	}

	bus := events.NewBus()
	store.SetRoot(dataDir)

	wsMgr, err := workspace.NewManager(dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to init workspace manager: %w", err)
	}

	fileWatcher, err := watcher.New(bus)
	if err != nil {
		return nil, fmt.Errorf("failed to init file watcher: %w", err)
	}

	exp := explorer.New(bus)
	sm := terminal.NewManager(bus)
	si := search.NewSearchManager()

	agentCLIConfigs := agentsession.NewConfigStore(dataDir)
	appSettingsStore := agentsession.NewSettingsStore(dataDir)
	agentSessionMgr := agentsession.NewManager(bus, dataDir, agentCLIConfigs)
	terminal.SetDefaultShell(appSettingsStore.Get().Terminal.Shell)

	llmClient := llm.NewLLMClient(dataDir)
	toolReg := tools.NewRegistry(si)
	skillMgr := skills.NewManager(filepath.Join(dataDir, "skills"))
	pluginMgr := plugins.NewManager(dataDir, bus)
	memoryMgr := memory.New(dataDir)
	memoryMgr.SetGlobalDir(dataDir)
	mcpMgr := mcp.NewManager(dataDir)

	app.SeedComputerUse(mcpMgr, skillMgr)
	app.SeedBrowserUse(skillMgr)

	agentMgr := agent.NewManagerWithOptions(agent.ManagerOptions{
		LLMClient:    llmClient,
		ToolRegistry: toolReg,
		Skills:       skillMgr,
		Plugins:      pluginMgr,
		MCP:          mcpMgr,
		Shell:        app.WrapShellManager(sm),
		Bus:          bus,
		DataDir:      dataDir,
	})
	agentMgr.SetMemoryManager(memoryMgr)

	js := appSettingsStore.Get().Jev
	if router := agentMgr.GetJevRouter(); router != nil {
		router.SetEnabled(js.Enabled)
		router.SetAPIKey(js.APIKey)
		if js.Endpoint != "" {
			router.SetEndpoint(js.Endpoint)
		}
	}
	if adapter := agentMgr.GetDecisionAdapter(); adapter != nil {
		mode := decisions.ModeLocalOnly
		if js.Enabled {
			mode = decisions.ModeCascade
		}
		_ = adapter.Configure(decisions.DecisionConfig{
			EnableJev:           js.Enabled,
			JevAPIKey:           js.APIKey,
			JevEndpoint:         js.Endpoint,
			Mode:                mode,
			ConfidenceThreshold: 0.90,
			FallbackToLocal:     true,
		})
	}

	gitEngine := git.NewEngine()
	autoMgr, err := automations.NewManager(dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to init automations manager: %w", err)
	}

	remoteFS := remotefs.NewManager()
	remoteFS.SetEventBus(bus)

	boardMgr := board.NewManager(dataDir)

	return app.NewAppWithDependencies(app.AppDependencies{
		DataDir:         dataDir,
		Bus:             bus,
		WorkspaceMgr:    wsMgr,
		Explorer:        exp,
		RemoteFS:        remoteFS,
		SessionMgr:      sm,
		SearchMgr:       si,
		FileWatcher:     fileWatcher,
		AgentSessions:   agentSessionMgr,
		AgentCLIConfigs: agentCLIConfigs,
		AppSettings:     appSettingsStore,
		ACPMgr:          acp.NewManager(bus),
		LLMClient:       llmClient,
		ToolReg:         toolReg,
		SkillMgr:        skillMgr,
		PluginMgr:       pluginMgr,
		MemoryMgr:       memoryMgr,
		MCPMgr:          mcpMgr,
		AgentMgr:        agentMgr,
		GitEngine:       gitEngine,
		AutoMgr:         autoMgr,
		BoardMgr:        boardMgr,
	})
}
