package engine

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/Haslab-dev/RakitKode/agent"
	"github.com/Haslab-dev/RakitKode/events"
	"github.com/Haslab-dev/RakitKode/llm"
	"github.com/Haslab-dev/RakitKode/mcp"
	"github.com/Haslab-dev/RakitKode/memory"
	"github.com/Haslab-dev/RakitKode/plugins"
	"github.com/Haslab-dev/RakitKode/search"
	"github.com/Haslab-dev/RakitKode/skills"
	"github.com/Haslab-dev/RakitKode/tools"
	"github.com/hasdev/forge-ade/core/acp"
	"github.com/hasdev/forge-ade/core/automations"
	"github.com/hasdev/forge-ade/core/commitmsg"
	"github.com/hasdev/forge-ade/core/git"
	"github.com/hasdev/forge-ade/core/quota"
	"github.com/hasdev/forge-ade/core/remotefs"
	"github.com/hasdev/forge-ade/core/terminal"
	"github.com/hasdev/forge-ade/core/watcher"
)

// Engine is the lightweight, CGO-free composition root for ForgeADE Native.
type Engine struct {
	GitEngine       *git.Engine
	LLMClient       *llm.LLMClient
	CommitGenerator *commitmsg.Generator
	Watcher         *watcher.Watcher
	ACPMgr          *acp.Manager
	RemoteFS        *remotefs.Manager
	Bus             *events.Bus
	DataDir         string

	// RakitKode managers shared with the webview app (search, tools, skills,
	// plugins, memory, MCP, PTY shells, and the full agent engine).
	SearchMgr *search.SearchManager
	ToolReg   *tools.Registry
	SkillMgr  *skills.Manager
	PluginMgr *plugins.Manager
	MemoryMgr *memory.Manager
	MCPMgr    *mcp.Manager
	ShellMgr  *terminal.Manager
	AgentMgr  *agent.Manager
	AutoMgr   *automations.Manager

	watcherMu         sync.Mutex
	watcherSubscribed bool
	watcherOnChange   func()
	watcherTimer      *time.Timer
	watcherRoot       string // currently watched root dir ("" = none)

	// Automation scheduler lifecycle + UI fire handler.
	autoMu      sync.Mutex
	autoCancel  context.CancelFunc
	autoHandler func(automations.Automation)
}

// automationFireEvent is published on the engine bus when a scheduled
// automation becomes due. Defined here because the shared RakitKode events
// package predates automations.
const automationFireEvent events.EventType = "automations:fire"

// NewEngine creates and initializes the native backend subsystems.
func NewEngine(dataDir string) *Engine {
	if dataDir == "" {
		home, _ := os.UserHomeDir()
		// ForgeADE data directory holds sessions, profiles, providers, and settings.
		dataDir = filepath.Join(home, ".forge-ade")
		if _, err := os.Stat(dataDir); os.IsNotExist(err) {
			legacy := filepath.Join(home, ".forge-native")
			if _, lerr := os.Stat(legacy); lerr == nil {
				dataDir = legacy
			}
		}
	}
	_ = os.MkdirAll(dataDir, 0755)

	bus := events.NewBus()
	acpMgr := acp.NewManager(bus)
	acpMgr.SetDataDir(dataDir)

	remoteMgr := remotefs.NewManager(dataDir)
	remoteMgr.SetEventBus(bus)

	gitEngine := git.NewEngine()
	llmClient := llm.NewLLMClient(dataDir)
	commitGen := &commitmsg.Generator{
		Git: gitEngine,
		LLM: llmClient,
	}

	w, _ := watcher.New(bus)

	mgrs := wireManagers(dataDir, bus, llmClient)
	autoMgr, autoErr := automations.NewManager(dataDir)
	if autoErr != nil {
		// A corrupt automations.json was quarantined; the manager still works
		// on a fresh schedule, so the engine keeps it and logs the cause.
		log.Printf("engine: automations: %v", autoErr)
	}

	e := &Engine{
		GitEngine:       gitEngine,
		LLMClient:       llmClient,
		CommitGenerator: commitGen,
		Watcher:         w,
		ACPMgr:          acpMgr,
		RemoteFS:        remoteMgr,
		Bus:             bus,
		DataDir:         dataDir,
		SearchMgr:       mgrs.Search,
		ToolReg:         mgrs.Tools,
		SkillMgr:        mgrs.Skills,
		PluginMgr:       mgrs.Plugin,
		MemoryMgr:       mgrs.Memory,
		MCPMgr:          mgrs.MCP,
		ShellMgr:        mgrs.Shells,
		AgentMgr:        mgrs.Agent,
		AutoMgr:         autoMgr,
	}

	// Run the cron scheduler for the engine's lifetime: it advances and
	// persists NextRunAt/LastRunAt and delivers due fires via the bus and
	// the handler registered with SetAutomationFireHandler.
	ctx, cancel := context.WithCancel(context.Background())
	e.autoCancel = cancel
	autoMgr.StartScheduler(ctx, e.dispatchAutomationFire)

	// Connect MCP servers and register aliases in background so engine startup is non-blocking.
	if e.MCPMgr != nil && len(e.MCPMgr.ListServers()) > 0 {
		go func() {
			// A panic in background MCP discovery must not kill the app
			// (P1-6); startup simply continues without MCP tools.
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[engine] MCP background sync panicked: %v\n%s", r, debug.Stack())
				}
			}()
			_ = e.SyncMCP(nil)
		}()
	}

	return e
}

// SyncMCP connects to all enabled MCP servers, discovers their tools, registers canonical
// aliases (such as Figma tools), and publishes an mcp:servers_updated event on the engine bus.
func (e *Engine) SyncMCP(ctx context.Context) error {
	if e.MCPMgr == nil || e.ToolReg == nil {
		return nil
	}
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
	}
	if err := e.MCPMgr.ConnectAll(ctx); err != nil {
		log.Printf("engine: MCP ConnectAll: %v", err)
	}
	connected := e.MCPMgr.ListConnectedTools()
	for _, t := range connected {
		e.ToolReg.RegisterMCPToolWithCaller(llm.MCPTool{
			ServerName:  t.ServerName,
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
			Title:       t.Title,
			ReadOnly:    t.ReadOnly,
			Destructive: t.Destructive,
		}, e.MCPMgr)

		// Enrich Figma tools with canonical standard aliases so models looking
		// for "figma_*" or "mcp__figma__*" find them immediately.
		if strings.Contains(strings.ToLower(t.ServerName), "figma") {
			base := t.Name
			if strings.Contains(base, "/") {
				base = strings.SplitN(base, "/", 2)[1]
			}
			aliases := []string{
				"figma_" + strings.TrimPrefix(base, "figma_"),
				"mcp__figma__" + strings.TrimPrefix(base, "figma_"),
			}
			if base == "get_figma_data" {
				aliases = append(aliases, "figma_get_data", "figma_get_file_data", "mcp__figma__get_data")
			} else if base == "download_figma_images" {
				aliases = append(aliases, "figma_download_images", "mcp__figma__download_images")
			}
			for _, alias := range aliases {
				e.ToolReg.RegisterMCPToolWithCaller(llm.MCPTool{
					ServerName:  t.ServerName,
					Name:        alias,
					Description: t.Description,
					InputSchema: t.InputSchema,
					Title:       t.Title,
					ReadOnly:    t.ReadOnly,
					Destructive: t.Destructive,
				}, e.MCPMgr)
			}
		}
	}
	if e.Bus != nil {
		e.Bus.Publish(events.Event{
			Type: "mcp:servers_updated",
			Data: map[string]interface{}{"count": len(connected)},
		})
	}
	return nil
}

// dispatchAutomationFire delivers a due automation to the UI layer: a bus
// event plus the registered fire handler. core stays UI-free — the fire
// carries only the automation payload; the UI decides how to run it.
func (e *Engine) dispatchAutomationFire(a automations.Automation) {
	if e.Bus != nil {
		e.Bus.Publish(events.Event{
			Type: automationFireEvent,
			Data: map[string]interface{}{
				"id":        a.ID,
				"name":      a.Name,
				"prompt":    a.Prompt,
				"workspace": a.Workspace,
				"cronExpr":  a.CronExpr,
			},
		})
	}
	e.autoMu.Lock()
	fn := e.autoHandler
	e.autoMu.Unlock()
	if fn != nil {
		fn(a)
	}
}

// SetAutomationFireHandler registers the callback invoked when a scheduled
// automation fires (with its full payload). Pass nil to clear it.
func (e *Engine) SetAutomationFireHandler(fn func(automations.Automation)) {
	e.autoMu.Lock()
	e.autoHandler = fn
	e.autoMu.Unlock()
}

// ConnectSSH establishes a connection to a remote SSH/SFTP server.
func (e *Engine) ConnectSSH(cfg remotefs.SSHConfig) (*remotefs.ConnectionStatus, error) {
	if e.RemoteFS == nil {
		return nil, fmt.Errorf("remote fs manager not initialized")
	}
	return e.RemoteFS.Connect(cfg)
}

// DisconnectSSH closes an active remote SSH/SFTP connection.
func (e *Engine) DisconnectSSH(connID string) error {
	if e.RemoteFS == nil {
		return nil
	}
	return e.RemoteFS.Disconnect(connID)
}

// ReconnectSSH attempts to re-establish an existing remote SSH session.
func (e *Engine) ReconnectSSH(connID string) (*remotefs.ConnectionStatus, error) {
	if e.RemoteFS == nil {
		return nil, fmt.Errorf("remote fs manager not initialized")
	}
	return e.RemoteFS.Reconnect(connID)
}

// IsSSHConnected reports whether remote connection connID is active.
func (e *Engine) IsSSHConnected(connID string) bool {
	if e.RemoteFS == nil {
		return false
	}
	return e.RemoteFS.IsConnected(connID)
}

// ListSSHConnections lists all registered remote SSH sessions.
func (e *Engine) ListSSHConnections() []remotefs.ConnectionStatus {
	if e.RemoteFS == nil {
		return nil
	}
	return e.RemoteFS.ListConnections()
}

// GetGitStatus queries porcelain v2 status through the native Git engine.
func (e *Engine) GetGitStatus(repoPath string) (*git.GitStatusResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return e.GitEngine.GetStatus(ctx, repoPath)
}

// GitStage stages the specified files.
func (e *Engine) GitStage(repoPath string, paths []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return e.GitEngine.Stage(ctx, repoPath, paths)
}

// GitUnstage unstages the specified files.
func (e *Engine) GitUnstage(repoPath string, paths []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return e.GitEngine.Unstage(ctx, repoPath, paths)
}

// GitCommit creates a git commit.
// GitDiscard reverts unstaged changes of the given paths to HEAD, as
// forge-ade's GitDiscard binding does (git restore --worktree, plus clean
// for untracked paths).
func (e *Engine) GitDiscard(repoPath string, paths []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return e.GitEngine.Discard(ctx, repoPath, paths)
}

func (e *Engine) GitCommit(repoPath string, message string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return e.GitEngine.Commit(ctx, repoPath, message)
}

// GetGitBranches retrieves repository branches.
func (e *Engine) GetGitBranches(repoPath string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return e.GitEngine.GetBranches(ctx, repoPath)
}

// validateBranchName rejects branch arguments that could be misread as
// command-line flags. Note `--` cannot separate it the way Merge does: after
// `--`, git checkout parses arguments as PATHSPECS, so `git checkout -- main`
// fails with "pathspec did not match" instead of switching branches (verified
// against git). exec.Command passes argv directly — no shell — so a leading
// dash is the only injection vector, and it is blocked here.
func validateBranchName(branch string) error {
	name := strings.TrimSpace(branch)
	if name == "" {
		return fmt.Errorf("branch name cannot be empty")
	}
	if strings.HasPrefix(name, "-") {
		return fmt.Errorf("invalid branch name %q: must not start with '-'", branch)
	}
	return nil
}

// GitCheckout switches branch in repo.
func (e *Engine) GitCheckout(repoPath, branch string) error {
	if err := validateBranchName(branch); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "checkout", branch)
	cmd.Dir = repoPath
	err := cmd.Run()
	if err == nil && e.GitEngine != nil {
		e.GitEngine.Invalidate(repoPath)
	}
	return err
}

// GitCreateBranch creates and checks out a new branch.
func (e *Engine) GitCreateBranch(repoPath, branch string) error {
	if err := validateBranchName(branch); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "checkout", "-b", branch)
	cmd.Dir = repoPath
	err := cmd.Run()
	if err == nil && e.GitEngine != nil {
		e.GitEngine.Invalidate(repoPath)
	}
	return err
}

// GetGitDiff returns the unified diff for a single file against HEAD.
func (e *Engine) GetGitDiff(repoPath, path string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	diff, err := e.GitEngine.GetFileDiff(ctx, repoPath, path)
	if err != nil {
		return ""
	}
	return diff
}

// GitPush pushes committed changes to the upstream branch.
func (e *Engine) GitPush(repoPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return e.GitEngine.Push(ctx, repoPath)
}

// GitFetch fetches latest references from remotes.
func (e *Engine) GitFetch(repoPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_, err := e.GitEngine.Fetch(ctx, repoPath)
	return err
}

// GitPull fetches and merges from the tracking branch.
func (e *Engine) GitPull(repoPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "pull")
	cmd.Dir = repoPath
	err := cmd.Run()
	if err == nil && e.GitEngine != nil {
		e.GitEngine.Invalidate(repoPath)
	}
	return err
}

// GetCommitDiff returns the diff of a specific commit hash.
func (e *Engine) GetCommitDiff(repoPath, hash string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	diff, err := e.GitEngine.GetCommitDiff(ctx, repoPath, hash)
	if err != nil {
		return ""
	}
	return diff
}

// GetGitCommitGraph returns commit history log.
func (e *Engine) GetGitCommitGraph(repoPath string, limit int) (*git.CommitGraphResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return e.GitEngine.GetCommitGraph(ctx, repoPath, 0, limit, "")
}

// GenerateAICommit generates an AI commit message using the staged diff and LLMClient.
func (e *Engine) GenerateAICommit(ctx context.Context, repoPath, instruction string) (string, error) {
	if e.CommitGenerator == nil {
		return "", nil
	}
	cfg := e.GetLLMConfig()
	return e.CommitGenerator.Generate(ctx, repoPath, cfg.ProviderID, cfg.Model, instruction)
}

// GetAgentQuota returns real account/tier/quota usage for an agent (e.g.
// agent-agy), falling back to ACP usage totals, as forge-ade's binding does.
//
// Antigravity quotas are read-only over the credentials the active Agy ACP
// session already holds: when no stored Agy login exists the fetch never
// spawns the agy CLI or hits its APIs (spawning it unauthenticated launches
// the OAuth login page — user-reported) and the honest "unavailable" payload
// is returned instead.
func (e *Engine) GetAgentQuota(agentID string) map[string]interface{} {
	summary := func() map[string]interface{} { return e.ACPMgr.GetUsageSummary() }
	return computeAgentQuota(agentID, agyAuthLookup(), func() (map[string]interface{}, error) {
		return quota.GetAgentQuota(agentID, summary)
	}, summary)
}

// computeAgentQuota is the testable core of GetAgentQuota: pure routing over
// an auth snapshot, a live fetch, and the ACP usage-summary fallback.
func computeAgentQuota(agentID string, auth AgyAuthStatus, fetch func() (map[string]interface{}, error), summary func() map[string]interface{}) map[string]interface{} {
	norm := strings.ToLower(agentID)
	if strings.Contains(norm, "agy") || strings.Contains(norm, "antigravity") {
		if !auth.Authenticated {
			// No Agy login the fetch could use non-interactively: report the
			// honest unavailable state — never a fabricated 100% and never a
			// login trigger.
			return unavailableAgyQuota()
		}
		res, err := fetch()
		if err != nil || res == nil {
			res = unavailableAgyQuota()
		} else if known, ok := res["quotaKnown"].(bool); ok && !known {
			// Every quota source failed (CLI missing, LS unreachable, API
			// non-200): strip the placeholder windows so the modal renders
			// Unavailable instead of fabricated green bars.
			stripped := unavailableAgyQuota()
			if acc, _ := res["account"].(string); strings.TrimSpace(acc) != "" {
				stripped["account"] = acc
			}
			res = stripped
		} else {
			res["quotaKnown"] = true
		}
		// Pin the account to the snapshot of the credentials the running Agy
		// session authenticates with, so the modal always names the account
		// that actually owns these quota numbers.
		if acc, _ := res["account"].(string); strings.TrimSpace(acc) == "" {
			res["account"] = auth.Account
		}
		return res
	}
	if res, err := fetch(); err == nil && res != nil {
		return res
	}
	if summary != nil {
		if s := summary(); s != nil {
			return s
		}
	}
	return map[string]interface{}{}
}

// unavailableAgyQuota is the honest "no Antigravity account connected"
// payload: no window data at all, so the modal renders Unavailable instead
// of a fake full quota.
func unavailableAgyQuota() map[string]interface{} {
	return map[string]interface{}{
		"agentId":    "agent-agy",
		"agentName":  "Google Antigravity",
		"account":    "",
		"quotaKnown": false,
	}
}

// StartWatcher attaches file system monitoring to the root directory and calls onChange when changes occur.
func (e *Engine) StartWatcher(rootDir string, onChange func()) {
	if e.Watcher == nil || e.Bus == nil {
		return
	}
	// The General home workspace is never recursively watched: a recursive
	// watch over the user's home exhausts the per-process file-descriptor
	// budget (macOS GUI apps start at 256) and floods the bus with noise
	// from unrelated tools writing into ~.
	if home, err := os.UserHomeDir(); err == nil && (rootDir == home || rootDir == "" || rootDir == "/") {
		return
	}
	e.watcherMu.Lock()
	prevRoot := e.watcherRoot
	e.watcherRoot = rootDir
	e.watcherMu.Unlock()

	// A workspace switch must release the previous root: every WatchDir adds
	// the whole subtree to the fsnotify watch list, so switching without
	// unwatching accumulates stale handles (and stale event sources) forever.
	if prevRoot != "" && prevRoot != rootDir {
		_ = e.Watcher.UnwatchDir(prevRoot)
	}

	e.Watcher.Start()
	go func(root string) {
		_ = e.Watcher.WatchDir(root)
	}(rootDir)

	e.watcherMu.Lock()
	e.watcherOnChange = onChange
	if e.watcherTimer != nil {
		e.watcherTimer.Stop()
		e.watcherTimer = nil
	}
	subscribed := e.watcherSubscribed
	e.watcherSubscribed = true
	e.watcherMu.Unlock()

	if !subscribed {
		e.Bus.Subscribe(events.FileChanged, func(ev events.Event) {
			e.watcherMu.Lock()
			if e.watcherTimer != nil {
				e.watcherTimer.Stop()
			}
			cb := e.watcherOnChange
			if cb != nil {
				// The debounce callback runs on its own timer goroutine: a
				// panicking handler must be logged, not fatal (P1-6).
				e.watcherTimer = time.AfterFunc(150*time.Millisecond, func() {
					defer func() {
						if r := recover(); r != nil {
							log.Printf("[engine] watcher debounce callback panicked: %v\n%s", r, debug.Stack())
						}
					}()
					cb()
				})
			}
			e.watcherMu.Unlock()
		})
	}
}

// StopWatcher cancels any pending debounce timer and stops the filesystem watcher.
func (e *Engine) StopWatcher() {
	if e == nil {
		return
	}
	e.watcherMu.Lock()
	e.watcherOnChange = nil
	if e.watcherTimer != nil {
		e.watcherTimer.Stop()
		e.watcherTimer = nil
	}
	e.watcherRoot = ""
	e.watcherMu.Unlock()
	if e.Watcher != nil {
		e.Watcher.Stop()
	}
}

// GetLLMConfig returns the persisted LLM profile.
func (e *Engine) GetLLMConfig() llm.Profile {
	if e.LLMClient == nil {
		return llm.Profile{}
	}
	return e.LLMClient.GetConfig()
}

// SaveLLMProfile saves LLM provider credentials and model choices.
func (e *Engine) SaveLLMProfile(providerID, apiKey, baseURL, model string) error {
	if e.LLMClient == nil {
		return nil
	}
	return e.LLMClient.SaveProfile(providerID, apiKey, baseURL, model)
}
