package ui

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/Haslab-dev/RakitKode/events"
	"github.com/Haslab-dev/RakitKode/llm"
	"github.com/hasdev/forge-ade/internal/git"
)

// Engine is the lightweight, CGO-free composition root for ForgeADE Native.
type Engine struct {
	GitEngine *git.Engine
	LLMClient *llm.LLMClient
	Bus       *events.Bus
	DataDir   string
}

// NewEngine creates and initializes the native backend subsystems.
func NewEngine(dataDir string) *Engine {
	if dataDir == "" {
		home, _ := os.UserHomeDir()
		dataDir = filepath.Join(home, ".forge-native")
	}
	_ = os.MkdirAll(dataDir, 0755)

	return &Engine{
		GitEngine: git.NewEngine(),
		LLMClient: llm.NewLLMClient(dataDir),
		Bus:       events.NewBus(),
		DataDir:   dataDir,
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
