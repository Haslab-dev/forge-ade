package engine

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
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

	return e
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
func (e *Engine) GetAgentQuota(agentID string) map[string]interface{} {
	summary := func() map[string]interface{} { return e.ACPMgr.GetUsageSummary() }
	if res, err := quota.GetAgentQuota(agentID, summary); err == nil {
		return res
	}
	return summary()
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
				e.watcherTimer = time.AfterFunc(150*time.Millisecond, cb)
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
