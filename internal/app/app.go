package app

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"runtime/debug"
	"sync"
	"time"
	"unsafe"

	"github.com/Haslab-dev/RakitKode/agent"
	"github.com/Haslab-dev/RakitKode/decisions"
	"github.com/Haslab-dev/RakitKode/events"
	"github.com/Haslab-dev/RakitKode/index"
	"github.com/Haslab-dev/RakitKode/llm"
	"github.com/Haslab-dev/RakitKode/mcp"
	"github.com/Haslab-dev/RakitKode/memory"
	"github.com/Haslab-dev/RakitKode/plugins"
	"github.com/Haslab-dev/RakitKode/sandbox"
	"github.com/Haslab-dev/RakitKode/search"
	"github.com/Haslab-dev/RakitKode/skills"
	"github.com/Haslab-dev/RakitKode/store"
	"github.com/Haslab-dev/RakitKode/tools"
	"github.com/hasdev/forge-ade/internal/acp"
	"github.com/hasdev/forge-ade/internal/agentsession"
	"github.com/hasdev/forge-ade/internal/automations"
	"github.com/hasdev/forge-ade/internal/board"
	"github.com/hasdev/forge-ade/internal/browseruse"
	"github.com/hasdev/forge-ade/internal/computeruse"
	"github.com/hasdev/forge-ade/internal/explorer"
	"github.com/hasdev/forge-ade/internal/git"
	"github.com/hasdev/forge-ade/internal/remotefs"
	"github.com/hasdev/forge-ade/internal/terminal"
	"github.com/hasdev/forge-ade/internal/watcher"
	"github.com/hasdev/forge-ade/internal/workspace"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// App is the main application struct that exposes methods to the frontend via Wails.
type App struct {
	ctx          context.Context
	appCancel    context.CancelFunc
	bus          *events.Bus
	workspaceMgr *workspace.Manager
	explorer     *explorer.Explorer
	remoteFS     *remotefs.Manager
	sessionMgr   *terminal.Manager
	searchMgr    *search.SearchManager
	fileWatcher  *watcher.Watcher
	indexStore   *index.Store
	indexUnsub   func()
	indexCancel  context.CancelFunc
	dataDir      string

	// Terminal Session mode (the default experience): agent CLI sessions,
	// their launch configs, and application settings.
	agentSessions   *agentsession.Manager
	agentCLIConfigs *agentsession.ConfigStore
	appSettings     *agentsession.SettingsStore

	// ACP Engine (Agent Client Protocol)
	acpMgr *acp.Manager

	llmClient *llm.LLMClient
	toolReg   *tools.Registry
	skillMgr  *skills.Manager
	pluginMgr *plugins.Manager
	memoryMgr *memory.Manager
	mcpMgr    *mcp.Manager
	agentMgr  *agent.Manager
	gitEngine *git.Engine
	autoMgr   *automations.Manager
	buMgr     *browseruse.Manager
	cuMgr     *computeruse.Manager
	boardMgr  *board.Manager

	boardWatcher *BoardWatcher

	// watchedRoots tracks the workspace folders currently watched by the file
	// watcher so opening a new workspace can unwatch the previous one instead
	// of accumulating watches (and exhausting the watcher's dir budget).
	watchedRoots []string

	// workspaceMu serializes onWorkspaceOpened (called from NewApp's restore
	// path and from the open-folder/workspace bindings, which can overlap at
	// startup).
	workspaceMu sync.Mutex

	// mainWinMu guards mainWin (set from main before Run, read from any
	// goroutine via mainWindowHandle).
	mainWinMu sync.Mutex
	mainWin   *application.WebviewWindow
}

// SetMainWindow records the primary Wails window (called from main.go right
// after window creation).
func (a *App) SetMainWindow(win *application.WebviewWindow) {
	a.mainWinMu.Lock()
	a.mainWin = win
	a.mainWinMu.Unlock()
}

// mainWindowHandle returns the NSWindow handle of the main window, or nil
// before it exists.
func (a *App) mainWindowHandle() unsafe.Pointer {
	a.mainWinMu.Lock()
	win := a.mainWin
	a.mainWinMu.Unlock()
	if win == nil {
		return nil
	}
	return win.NativeWindow()
}

// AppDependencies defines the explicit subsystem dependencies injected into App.
type AppDependencies struct {
	DataDir         string
	Bus             *events.Bus
	WorkspaceMgr    *workspace.Manager
	Explorer        *explorer.Explorer
	RemoteFS        *remotefs.Manager
	SessionMgr      *terminal.Manager
	SearchMgr       *search.SearchManager
	FileWatcher     *watcher.Watcher
	AgentSessions   *agentsession.Manager
	AgentCLIConfigs *agentsession.ConfigStore
	AppSettings     *agentsession.SettingsStore
	ACPMgr          *acp.Manager
	LLMClient       *llm.LLMClient
	ToolReg         *tools.Registry
	SkillMgr        *skills.Manager
	PluginMgr       *plugins.Manager
	MemoryMgr       *memory.Manager
	MCPMgr          *mcp.Manager
	AgentMgr        *agent.Manager
	GitEngine       *git.Engine
	AutoMgr         *automations.Manager
	BUMgr           *browseruse.Manager
	CUMgr           *computeruse.Manager
	BoardMgr        *board.Manager
	BoardWatcher    *BoardWatcher
}

// NewAppWithDependencies creates an App instance with fully injected dependencies.
func NewAppWithDependencies(deps AppDependencies) (*App, error) {
	ctx, cancel := context.WithCancel(context.Background())

	a := &App{
		ctx:             ctx,
		appCancel:       cancel,
		bus:             deps.Bus,
		workspaceMgr:    deps.WorkspaceMgr,
		explorer:        deps.Explorer,
		remoteFS:        deps.RemoteFS,
		sessionMgr:      deps.SessionMgr,
		searchMgr:       deps.SearchMgr,
		fileWatcher:     deps.FileWatcher,
		dataDir:         deps.DataDir,
		llmClient:       deps.LLMClient,
		toolReg:         deps.ToolReg,
		skillMgr:        deps.SkillMgr,
		pluginMgr:       deps.PluginMgr,
		memoryMgr:       deps.MemoryMgr,
		mcpMgr:          deps.MCPMgr,
		agentMgr:        deps.AgentMgr,
		gitEngine:       deps.GitEngine,
		autoMgr:         deps.AutoMgr,
		buMgr:           deps.BUMgr,
		cuMgr:           deps.CUMgr,
		boardMgr:        deps.BoardMgr,
		boardWatcher:    deps.BoardWatcher,
		agentSessions:   deps.AgentSessions,
		agentCLIConfigs: deps.AgentCLIConfigs,
		appSettings:     deps.AppSettings,
		acpMgr:          deps.ACPMgr,
	}

	if a.acpMgr != nil && a.dataDir != "" {
		a.acpMgr.SetDataDir(a.dataDir)
	}

	if a.toolReg != nil {
		if a.buMgr == nil {
			a.buMgr = wireBrowserUse(a.dataDir, a.bus, a.toolReg, a.mainWindowHandle)
		}
		if a.cuMgr == nil {
			a.cuMgr = wireComputerUse(a.toolReg)
		}
		RegisterBoardTools(a.toolReg, a)
	}

	if a.boardWatcher == nil && a.boardMgr != nil {
		bw, err := newBoardWatcher(a)
		if err == nil {
			a.boardWatcher = bw
		}
	}

	// Wire board updates
	if a.boardMgr != nil {
		a.boardMgr.SetOnUpdated(func(tasks []board.Task, rawBytes []byte) {
			if a.boardWatcher != nil {
				a.boardWatcher.UpdateLastHash(rawBytes)
			}
			a.emitEvent(EventBoardUpdated, tasks)
		})
	}

	// Wire up event handlers
	a.setupEventHandlers()

	if a.workspaceMgr != nil {
		if ws := a.workspaceMgr.Current(); ws != nil {
			a.onWorkspaceOpened(ws)
		} else {
			genDir := a.generalFolder()
			if a.agentMgr != nil {
				a.agentMgr.SetDefaultFolder(genDir)
			}
			_, _ = sandbox.Ensure(genDir)
			if a.skillMgr != nil {
				a.skillMgr.SetWorkspace(genDir)
			}
			if a.pluginMgr != nil {
				a.pluginMgr.SetWorkspace(genDir)
			}
			if a.memoryMgr != nil {
				a.memoryMgr.SetWorkspace(genDir)
			}
			if a.boardWatcher != nil {
				a.boardWatcher.WatchWorkspace([]string{genDir})
			}
		}
	}

	return a, nil
}

// NewApp creates a new App and initializes all subsystems.
func NewApp() (*App, error) {
	dataDir := getDataDir()

	bus := events.NewBus()

	// Engine storage nests under the app data dir (matching KendaliAI's
	// ~/.kendaliai/rakit pattern): one home for config, credentials and
	// engine store buckets.
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

	// Terminal Session mode: agent CLI launch configs + app settings
	// (default mode "terminal") + the session manager itself.
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

	// Computer Use (official MCP server from the ZCode plugin cache) and
	// Browser Use (in-app WKWebView engine + browser_* tools) extend the agent.
	seedComputerUse(mcpMgr, skillMgr)
	seedBrowserUse(skillMgr)

	agentMgr := agent.NewManagerWithOptions(agent.ManagerOptions{
		LLMClient:    llmClient,
		ToolRegistry: toolReg,
		Skills:       skillMgr,
		Plugins:      pluginMgr,
		MCP:          mcpMgr,
		Shell:        wrapShellManager(sm),
		Bus:          bus,
		DataDir:      dataDir,
	})
	agentMgr.SetMemoryManager(memoryMgr)

	// Jev tool-routing credentials come from app settings (Settings UI /
	// app-settings.json) — no .env fallback. Empty key = local heuristic
	// routing only.
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

	return NewAppWithDependencies(AppDependencies{
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

// MustNewApp creates a new App and panics if initialization fails.
func MustNewApp() *App {
	a, err := NewApp()
	if err != nil {
		panic(err)
	}
	return a
}

func (a *App) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	if a.appCancel != nil {
		a.appCancel()
	}
	a.ctx, a.appCancel = context.WithCancel(ctx)
	log.Println("ForgeADE started")

	// Host scheduler for scheduled automations: fires due cron schedules into
	// new agent sessions for as long as the app is running.
	if a.autoMgr != nil {
		a.autoMgr.StartScheduler(a.ctx, a.fireAutomation)
	}

	// Connect to enabled MCP servers and register their tools into the tool
	// registry so the agent can call them (connect-on-startup).
	go func() {
		if a.mcpMgr != nil {
			if err := a.mcpMgr.ConnectAll(a.ctx); err != nil {
				log.Printf("mcp: connect all: %v", err)
			}
			a.refreshMCPTools()
			// Reclaim one-time startup allocations from MCP handshake
			debug.FreeOSMemory()
		}
	}()

	// Prune stale scratch from agent sandboxes (tmp/ only — thinking/,
	// coding/ and logs/ are never touched). One sweep per launch; the app
	// restarts daily in practice.
	go sandbox.SweepTemp(7 * 24 * time.Hour)

	// Periodic idle memory scavenger: releases free heap pages back to macOS kernel
	// via madvise(MADV_FREE / MADV_DONTNEED). Go runtime retains virtual arenas by default;
	// FreeOSMemory returns unused physical memory when the app is idle.
	go func() {
		ticker := time.NewTicker(2 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-a.ctx.Done():
				return
			case <-ticker.C:
				debug.FreeOSMemory()
			}
		}
	}()

	return nil
}

// refreshMCPTools re-registers the tools discovered from live MCP connections.
func (a *App) refreshMCPTools() {
	if a.mcpMgr == nil || a.toolReg == nil {
		return
	}
	for _, t := range a.mcpMgr.ListConnectedTools() {
		a.toolReg.RegisterMCPToolWithCaller(llm.MCPTool{
			ServerName:  t.ServerName,
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		}, a.mcpMgr)
	}
}

// ServiceShutdown is called by Wails v3 when the application shuts down.
func (a *App) ServiceShutdown() {
	if a.appCancel != nil {
		a.appCancel()
	}

	// A browser spawned for the browser-use tools is our own child process
	// (own process group) — close it explicitly or it outlives the app.
	if a.buMgr != nil {
		a.buMgr.Close()
	}

	if a.indexCancel != nil {
		a.indexCancel()
		a.indexCancel = nil
	}
	if a.indexUnsub != nil {
		a.indexUnsub()
		a.indexUnsub = nil
	}

	if a.fileWatcher != nil {
		a.fileWatcher.Stop()
	}
	if a.boardWatcher != nil {
		a.boardWatcher.Stop()
	}
	if a.sessionMgr != nil {
		a.sessionMgr.StopAll()
	}
	if a.agentSessions != nil {
		a.agentSessions.Shutdown()
	}
	if a.searchMgr != nil {
		a.searchMgr.Stop()
	}
	if a.mcpMgr != nil {
		a.mcpMgr.DisconnectAll()
	}
	if a.remoteFS != nil {
		a.remoteFS.CloseAll()
	}
}
