package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// ServerConfig describes an MCP server configured via the GUI.
type ServerConfig struct {
	Name    string            `json:"name"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
	Type    string            `json:"type,omitempty"` // local (default) or remote (url)
	URL     string            `json:"url,omitempty"`
	Enabled bool              `json:"enabled"`
}

type Tool struct {
	ServerName  string                 `json:"server_name"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

type Manager struct {
	mu        sync.RWMutex
	servers   map[string]ServerConfig
	tools     map[string]Tool
	storePath string

	connMu      sync.RWMutex
	connections map[string]*serverConnection // keyed by "server/tool"
	lastErr     sync.Map                     // server name -> error
}

// NewManager creates an MCP manager that persists servers to
// <dataDir>/mcp_servers.json.
func NewManager(dataDir string) *Manager {
	m := &Manager{
		servers:     make(map[string]ServerConfig),
		tools:       make(map[string]Tool),
		storePath:   filepath.Join(dataDir, "mcp_servers.json"),
		connections: make(map[string]*serverConnection),
	}
	m.load()
	return m
}

func (m *Manager) load() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.storePath == "" {
		return
	}
	data, err := os.ReadFile(m.storePath)
	if err != nil {
		return
	}
	var list []ServerConfig
	if json.Unmarshal(data, &list) != nil {
		return
	}
	m.servers = make(map[string]ServerConfig, len(list))
	for _, s := range list {
		if s.Name != "" {
			m.servers[s.Name] = s
		}
	}
}

func (m *Manager) saveLocked() {
	if m.storePath == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(m.storePath), 0755)
	list := make([]ServerConfig, 0, len(m.servers))
	for _, s := range m.servers {
		list = append(list, s)
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err == nil {
		_ = os.WriteFile(m.storePath, data, 0644)
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

// ListServers returns all configured MCP servers.
func (m *Manager) ListServers() []ServerConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]ServerConfig, 0, len(m.servers))
	for _, s := range m.servers {
		list = append(list, s)
	}
	return list
}

// SaveServer creates or updates an MCP server (keyed by name).
func (m *Manager) SaveServer(s ServerConfig) (ServerConfig, error) {
	if s.Name == "" {
		return s, os.ErrInvalid
	}
	if s.Env == nil {
		s.Env = make(map[string]string)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.servers[s.Name] = s
	m.saveLocked()
	return s, nil
}

// DeleteServer removes an MCP server by name.
func (m *Manager) DeleteServer(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.servers, name)
	m.saveLocked()
	return nil
}

// CallTool invokes an MCP tool by its full "server/tool" name with the given
// arguments. Returns the tool's text content (or an error when the tool failed).
func (m *Manager) CallTool(ctx context.Context, fullName string, args map[string]any) (string, error) {
	return m.callTool(ctx, fullName, args)
}

// ConnectAll connects to all enabled MCP servers and discovers their tools.
func (m *Manager) ConnectAll(ctx context.Context) error {
	return m.connectAll(ctx)
}

// DisconnectAll closes all live MCP connections.
func (m *Manager) DisconnectAll() {
	m.disconnectAll()
}
