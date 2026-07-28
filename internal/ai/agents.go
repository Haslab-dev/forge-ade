package ai

import (
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hasdev/forge-ade/internal/events"
)

// ProviderType is the type of AI agent provider.
type ProviderType string

const (
	ProviderClaude   ProviderType = "claude"
	ProviderOpencode ProviderType = "opencode"
	ProviderGemini   ProviderType = "gemini"
	ProviderCodex    ProviderType = "codex"
	ProviderAider    ProviderType = "aider"
	ProviderCustom   ProviderType = "custom"
)

// Agent represents a running AI agent process.
type Agent struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Provider  ProviderType `json:"provider"`
	Status    string       `json:"status"` // running, idle, stopped, error
	PID       int          `json:"pid"`
	CreatedAt time.Time    `json:"createdAt"`
	Workspace string       `json:"workspace"`
	cmd       *exec.Cmd      `json:"-"`
	mu        sync.Mutex      `json:"-"`
}

// AgentManager manages AI agent processes.
type AgentManager struct {
	bus      *events.Bus
	mu       sync.RWMutex
	agents   map[string]*Agent
}

// NewAgentManager creates a new AI agent manager.
func NewAgentManager(bus *events.Bus) *AgentManager {
	return &AgentManager{
		bus:    bus,
		agents: make(map[string]*Agent),
	}
}

// Start launches an AI agent process.
func (am *AgentManager) Start(name string, provider ProviderType, workspace string) (*Agent, error) {
	cmd, err := am.buildCommand(provider, workspace)
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start agent: %w", err)
	}

	agent := &Agent{
		ID:        uuid.New().String(),
		Name:      name,
		Provider:  provider,
		Status:    "running",
		PID:       cmd.Process.Pid,
		CreatedAt: time.Now(),
		Workspace: workspace,
		cmd:       cmd,
	}

	am.mu.Lock()
	am.agents[agent.ID] = agent
	am.mu.Unlock()

	am.bus.Publish(events.Event{
		Type: events.AgentStarted,
		Data: map[string]interface{}{
			"id":       agent.ID,
			"name":     agent.Name,
			"provider": string(agent.Provider),
		},
	})

	// Monitor process
	go am.monitor(agent)

	return agent, nil
}

// Stop terminates an AI agent.
func (am *AgentManager) Stop(id string) error {
	am.mu.RLock()
	agent, ok := am.agents[id]
	am.mu.RUnlock()

	if !ok {
		return fmt.Errorf("agent not found: %s", id)
	}

	agent.mu.Lock()
	defer agent.mu.Unlock()

	if agent.Status == "stopped" {
		return nil
	}

	if agent.cmd != nil && agent.cmd.Process != nil {
		_ = agent.cmd.Process.Kill()
	}

	agent.Status = "stopped"

	am.bus.Publish(events.Event{
		Type: events.AgentStopped,
		Data: map[string]interface{}{
			"id": agent.ID,
		},
	})

	return nil
}

// List returns all agents.
func (am *AgentManager) List() []*Agent {
	am.mu.RLock()
	defer am.mu.RUnlock()

	agents := make([]*Agent, 0, len(am.agents))
	for _, a := range am.agents {
		agents = append(agents, a)
	}
	return agents
}

// Get returns an agent by ID.
func (am *AgentManager) Get(id string) (*Agent, bool) {
	am.mu.RLock()
	defer am.mu.RUnlock()
	a, ok := am.agents[id]
	return a, ok
}

// StopAll stops all running agents.
func (am *AgentManager) StopAll() {
	am.mu.RLock()
	ids := make([]string, 0, len(am.agents))
	for id := range am.agents {
		ids = append(ids, id)
	}
	am.mu.RUnlock()

	for _, id := range ids {
		_ = am.Stop(id)
	}
}

func (am *AgentManager) monitor(agent *Agent) {
	err := agent.cmd.Wait()

	agent.mu.Lock()
	if err != nil {
		agent.Status = "error"
	} else {
		agent.Status = "stopped"
	}
	agent.mu.Unlock()

	am.bus.Publish(events.Event{
		Type: events.AgentStopped,
		Data: map[string]interface{}{
			"id":    agent.ID,
			"error": err != nil,
		},
	})
}

func (am *AgentManager) buildCommand(provider ProviderType, workspace string) (*exec.Cmd, error) {
	var cmd *exec.Cmd

	switch provider {
	case ProviderClaude:
		cmd = exec.Command("claude", "-w", workspace)
	case ProviderOpencode:
		cmd = exec.Command("opencode", workspace)
	case ProviderGemini:
		cmd = exec.Command("gemini-cli", workspace)
	case ProviderCodex:
		cmd = exec.Command("codex", workspace)
	case ProviderAider:
		cmd = exec.Command("aider", workspace)
	case ProviderCustom:
		return nil, fmt.Errorf("custom providers require a command path configuration")
	default:
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}

	cmd.Dir = workspace
	return cmd, nil
}
