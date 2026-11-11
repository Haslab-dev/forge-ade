package ui

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/Haslab-dev/RakitKode/events"
	"github.com/Haslab-dev/RakitKode/llm"
	"github.com/hasdev/forge-ade/internal/acp"
	"github.com/hasdev/forge-ade/internal/commitmsg"
	"github.com/hasdev/forge-ade/internal/git"
	"github.com/hasdev/forge-ade/internal/watcher"
)

// Engine is the lightweight, CGO-free composition root for ForgeADE Native.
type Engine struct {
	GitEngine       *git.Engine
	LLMClient       *llm.LLMClient
	CommitGenerator *commitmsg.Generator
	Watcher         *watcher.Watcher
	ACPMgr          *acp.Manager
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
		Bus:             bus,
		DataDir:         dataDir,
	}
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

// StartWatcher attaches file system monitoring to the root directory and calls onChange when changes occur.
func (e *Engine) StartWatcher(rootDir string, onChange func()) {
	if e.Watcher == nil || e.Bus == nil {
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
