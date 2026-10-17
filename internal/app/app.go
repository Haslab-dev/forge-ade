package app

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unsafe"

	"github.com/Haslab-dev/RakitKode/agent"
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
	"github.com/hasdev/forge-ade/internal/agentsession"
	"github.com/hasdev/forge-ade/internal/automations"
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

// NewApp creates a new App and initializes all subsystems.
func NewApp() *App {
	dataDir := getDataDir()

	bus := events.NewBus()

	// The engine module defaults its store root to ~/.rakit; the desktop app
	// keeps its existing per-project layout under ~/.forge.
	if home, err := os.UserHomeDir(); err == nil {
		store.SetRoot(filepath.Join(home, ".forge"))
	}

	wsMgr, err := workspace.NewManager(dataDir)
	if err != nil {
		log.Fatalf("failed to init workspace manager: %v", err)
	}

	fileWatcher, err := watcher.New(bus)
	if err != nil {
		log.Fatalf("failed to init file watcher: %v", err)
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
	if router := agentMgr.GetJevRouter(); router != nil {
		js := appSettingsStore.Get().Jev
		router.SetAPIKey(js.APIKey)
		if js.Endpoint != "" {
			router.SetEndpoint(js.Endpoint)
		}
	}
	gitEngine := git.NewEngine()
	autoMgr, err := automations.NewManager(dataDir)
	if err != nil {
		log.Fatalf("failed to init automations manager: %v", err)
	}

	remoteFS := remotefs.NewManager()
	remoteFS.SetEventBus(bus)

	app := &App{
		ctx:             context.Background(),
		bus:             bus,
		workspaceMgr:    wsMgr,
		explorer:        exp,
		remoteFS:        remoteFS,
		sessionMgr:      sm,
		searchMgr:       si,
		fileWatcher:     fileWatcher,
		dataDir:         dataDir,
		llmClient:       llmClient,
		toolReg:         toolReg,
		skillMgr:        skillMgr,
		pluginMgr:       pluginMgr,
		memoryMgr:       memoryMgr,
		mcpMgr:          mcpMgr,
		agentMgr:        agentMgr,
		gitEngine:       gitEngine,
		autoMgr:         autoMgr,
		agentSessions:   agentSessionMgr,
		agentCLIConfigs: agentCLIConfigs,
		appSettings:     appSettingsStore,
	}

	// Browser Use: in-app WKWebView engine (needs the main NSWindow handle,
	// provided by main.go once the window exists) + browser_* agent tools.
	app.buMgr = wireBrowserUse(dataDir, bus, toolReg, app.mainWindowHandle)
	app.cuMgr = wireComputerUse(toolReg)

	// Wire up event handlers
	app.setupEventHandlers()

	if ws := wsMgr.Current(); ws != nil {
		app.onWorkspaceOpened(ws)
	} else {
		genDir := app.generalFolder()
		agentMgr.SetDefaultFolder(genDir)
		_, _ = sandbox.Ensure(genDir)
		skillMgr.SetWorkspace(genDir)
		if pluginMgr != nil {
			pluginMgr.SetWorkspace(genDir)
		}
		if memoryMgr != nil {
			memoryMgr.SetWorkspace(genDir)
		}
	}

	return app
}

func (a *App) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	a.ctx = ctx
	log.Println("ForgeADE started")

	// Host scheduler for scheduled automations: fires due cron schedules into
	// new agent sessions for as long as the app is running.
	if a.autoMgr != nil {
		a.autoMgr.StartScheduler(ctx, a.fireAutomation)
	}

	// Connect to enabled MCP servers and register their tools into the tool
	// registry so the agent can call them (connect-on-startup).
	go func() {
		if err := a.mcpMgr.ConnectAll(ctx); err != nil {
			log.Printf("mcp: connect all: %v", err)
		}
		a.refreshMCPTools()
	}()

	// Prune stale scratch from agent sandboxes (tmp/ only — thinking/,
	// coding/ and logs/ are never touched). One sweep per launch; the app
	// restarts daily in practice.
	go sandbox.SweepTemp(7 * 24 * time.Hour)
	return nil
}

// refreshMCPTools re-registers the tools discovered from live MCP connections.
func (a *App) refreshMCPTools() {
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

	a.fileWatcher.Stop()
	a.sessionMgr.StopAll()
	a.agentSessions.Shutdown()
	a.searchMgr.Stop()
	a.mcpMgr.DisconnectAll()
	if a.remoteFS != nil {
		a.remoteFS.CloseAll()
	}
}
