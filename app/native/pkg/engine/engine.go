package engine

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/Haslab-dev/RakitKode/events"
	"github.com/Haslab-dev/RakitKode/llm"
	"github.com/hasdev/forge-ade/core/acp"
	"github.com/hasdev/forge-ade/core/commitmsg"
	"github.com/hasdev/forge-ade/core/git"
	"github.com/hasdev/forge-ade/core/quota"
	"github.com/hasdev/forge-ade/core/remotefs"
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
}

// NewEngine creates and initializes the native backend subsystems.
func NewEngine(dataDir string) *Engine {
	if dataDir == "" {
		home, _ := os.UserHomeDir()
		// Prefer existing ForgeADE data directory so all real sessions,
		// profiles, providers, and settings are seamlessly shared!
		dataDir = filepath.Join(home, ".forge-ade")
		if _, err := os.Stat(dataDir); os.IsNotExist(err) {
			dataDir = filepath.Join(home, ".forge-native")
		}
	}
	_ = os.MkdirAll(dataDir, 0755)

	bus := events.NewBus()
	acpMgr := acp.NewManager(bus)
	acpMgr.SetDataDir(dataDir)

	remoteMgr := remotefs.NewManager()
	remoteMgr.SetEventBus(bus)

	gitEngine := git.NewEngine()
	llmClient := llm.NewLLMClient(dataDir)
	commitGen := &commitmsg.Generator{
		Git: gitEngine,
		LLM: llmClient,
	}

	w, _ := watcher.New(bus)

	return &Engine{
		GitEngine:       gitEngine,
		LLMClient:       llmClient,
		CommitGenerator: commitGen,
		Watcher:         w,
		ACPMgr:          acpMgr,
		RemoteFS:        remoteMgr,
		Bus:             bus,
		DataDir:         dataDir,
	}
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

// GitCheckout switches branch in repo.
func (e *Engine) GitCheckout(repoPath, branch string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "checkout", branch)
	cmd.Dir = repoPath
	return cmd.Run()
}

// GitCreateBranch creates and checks out a new branch.
func (e *Engine) GitCreateBranch(repoPath, branch string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "checkout", "-b", branch)
	cmd.Dir = repoPath
	return cmd.Run()
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
	return cmd.Run()
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
	e.Watcher.Start()
	_ = e.Watcher.WatchDir(rootDir)

	e.Bus.Subscribe(events.FileChanged, func(ev events.Event) {
		if onChange != nil {
			onChange()
		}
	})
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
