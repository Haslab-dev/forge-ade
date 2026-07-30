package mcp

import (
	"sync"
)

type ServerConfig struct {
	Name    string            `json:"name"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
}

type Tool struct {
	ServerName  string                 `json:"server_name"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

type Manager struct {
	mu      sync.RWMutex
	servers map[string]ServerConfig
	tools   map[string]Tool
}

func NewManager() *Manager {
	return &Manager{
		servers: make(map[string]ServerConfig),
		tools:   make(map[string]Tool),
	}
}

func (m *Manager) ListTools() []Tool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]Tool, 0, len(m.tools))
	for _, t := range m.tools {
		list = append(list, t)
	}
	return list
}
