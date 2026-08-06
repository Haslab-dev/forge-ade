package mcp

import (
	"context"
	"encoding/json"
	"fmt"
)

// MCP protocol version we support.
const protocolVersion = "2025-03-26"

// clientInfo identifies us to the MCP server during initialization.
const clientInfoName = "forge-ade"

// serverConnection is a live connection to one MCP server (port of
// a live MCP server connection.
type serverConnection struct {
	name       string
	config     ServerConfig
	transport  *stdioTransport
	serverInfo map[string]any
	capTools   bool
	tools      []Tool
}

// toolDiscovery performs initialize + tools/list on a spawned transport.
func (m *Manager) connectServer(ctx context.Context, cfg ServerConfig) (*serverConnection, error) {
	if cfg.Type != "" && cfg.Type != "stdio" && cfg.Type != "local" {
		return nil, fmt.Errorf("mcp server %q: only stdio (local) servers are supported, got type %q", cfg.Name, cfg.Type)
	}
	command := cfg.Command
	if command == "" {
		return nil, fmt.Errorf("mcp server %q: command is required", cfg.Name)
	}

	transport, err := newStdioTransport(ctx, command, cfg.Args, cfg.Env)
	if err != nil {
		return nil, err
	}

	// initialize
	initParams := map[string]any{
		"protocolVersion": protocolVersion,
		"capabilities": map[string]any{
			"roots": map[string]any{"listChanged": false},
		},
		"clientInfo": map[string]any{
			"name":    clientInfoName,
			"version": "1.0.0",
		},
	}
	res, err := transport.request(ctx, "initialize", initParams)
	if err != nil {
		transport.close()
		return nil, fmt.Errorf("mcp initialize %q: %w", cfg.Name, err)
	}

	var initResult struct {
		ServerInfo   map[string]any `json:"serverInfo"`
		Capabilities map[string]any `json:"capabilities"`
	}
	_ = json.Unmarshal(res, &initResult)

	// Send the initialized notification.
	_ = transport.notify("notifications/initialized", nil)

	conn := &serverConnection{
		name:       cfg.Name,
		config:     cfg,
		transport:  transport,
		serverInfo: initResult.ServerInfo,
	}
	if caps, ok := initResult.Capabilities["tools"].(map[string]any); ok {
		conn.capTools = caps != nil
	}

	// Discover tools.
	if conn.capTools {
		tools, err := m.listTools(ctx, conn)
		if err != nil {
			transport.close()
			return nil, fmt.Errorf("mcp tools/list %q: %w", cfg.Name, err)
		}
		conn.tools = tools
	}

	return conn, nil
}

// listTools paginates through tools/list on a connection.
func (m *Manager) listTools(ctx context.Context, conn *serverConnection) ([]Tool, error) {
	var all []Tool
	cursor := ""
	for {
		params := map[string]any{}
		if cursor != "" {
			params["cursor"] = cursor
		}
		res, err := conn.transport.request(ctx, "tools/list", params)
		if err != nil {
			return nil, err
		}
		var result struct {
			Tools []struct {
				Name        string         `json:"name"`
				Description string         `json:"description"`
				InputSchema map[string]any `json:"inputSchema"`
			} `json:"tools"`
			NextCursor string `json:"nextCursor"`
		}
		if err := json.Unmarshal(res, &result); err != nil {
			return nil, err
		}
		for _, t := range result.Tools {
			all = append(all, Tool{
				ServerName:  conn.name,
				Name:        t.Name,
				Description: t.Description,
				InputSchema: t.InputSchema,
			})
		}
		if result.NextCursor == "" {
			break
		}
		cursor = result.NextCursor
	}
	return all, nil
}

// callTool invokes tools/call on the server that owns the tool name.
func (m *Manager) callTool(ctx context.Context, fullName string, args map[string]any) (string, error) {
	serverName := toolServerName(fullName)
	m.connMu.RLock()
	conn := m.connections[serverName]
	m.connMu.RUnlock()
	if conn == nil {
		return "", fmt.Errorf("no connected MCP server for tool %q", fullName)
	}

	params := map[string]any{
		"name":      toolBaseName(fullName),
		"arguments": args,
	}
	res, err := conn.transport.request(ctx, "tools/call", params)
	if err != nil {
		return "", err
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	_ = json.Unmarshal(res, &result)

	var sb []byte
	for _, c := range result.Content {
		if c.Text != "" {
			sb = append(sb, c.Text...)
		}
	}
	if result.IsError {
		return string(sb), fmt.Errorf("mcp tool %q failed: %s", fullName, string(sb))
	}
	return string(sb), nil
}

// toolServerName extracts the "server" part of a "server/tool" name.
func toolServerName(fullName string) string {
	for i := 0; i < len(fullName); i++ {
		if fullName[i] == '/' {
			return fullName[:i]
		}
	}
	return fullName
}

// toolBaseName strips the "serverName/" prefix from an MCP tool name.
func toolBaseName(fullName string) string {
	for i := len(fullName) - 1; i >= 0; i-- {
		if fullName[i] == '/' {
			return fullName[i+1:]
		}
	}
	return fullName
}

// ---------------------------------------------------------------------------
// Manager extension: live connections + tool discovery
// ---------------------------------------------------------------------------

// connectAll spawns and initializes all enabled MCP servers, discovering their
// tools. Called at startup.
func (m *Manager) connectAll(ctx context.Context) error {
	m.mu.RLock()
	servers := make([]ServerConfig, 0, len(m.servers))
	for _, s := range m.servers {
		if s.Enabled {
			servers = append(servers, s)
		}
	}
	m.mu.RUnlock()

	m.connMu.Lock()
	m.connections = make(map[string]*serverConnection)
	m.connMu.Unlock()

	for _, cfg := range servers {
		conn, err := m.connectServer(ctx, cfg)
		if err != nil {
			// Log and continue; a broken server should not block the 
			m.lastErr.Store(cfg.Name, err)
			continue
		}
		m.connMu.Lock()
		m.connections[cfg.Name] = conn
		m.connMu.Unlock()
	}
	return nil
}

// disconnectAll closes all live MCP connections.
func (m *Manager) disconnectAll() {
	m.connMu.Lock()
	defer m.connMu.Unlock()
	for _, conn := range m.connections {
		conn.transport.close()
	}
	m.connections = make(map[string]*serverConnection)
}

// ListConnectedTools returns the tools discovered from live MCP connections.
// Each tool's Name is prefixed with "serverName/" so the agent can route calls.
func (m *Manager) ListConnectedTools() []Tool {
	m.connMu.RLock()
	defer m.connMu.RUnlock()
	var out []Tool
	for _, conn := range m.connections {
		for _, t := range conn.tools {
			out = append(out, Tool{
				ServerName:  conn.name,
				Name:        conn.name + "/" + t.Name,
				Description: t.Description,
				InputSchema: t.InputSchema,
			})
		}
	}
	return out
}

// ServerConnectionError exposes the most recent connect error for a server.
func (m *Manager) ServerConnectionError(name string) error {
	v, ok := m.lastErr.Load(name)
	if !ok {
		return nil
	}
	err, _ := v.(error)
	return err
}
