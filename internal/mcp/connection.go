package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode"
)

// MCP protocol version we support.
const protocolVersion = "2024-11-05"

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
	if ctx == nil {
		ctx = context.Background()
	}
	if cfg.Type != "" && cfg.Type != "stdio" && cfg.Type != "local" {
		return nil, fmt.Errorf("mcp server %q: only stdio (local) servers are supported, got type %q", cfg.Name, cfg.Type)
	}
	command := cfg.Command
	if command == "" {
		return nil, fmt.Errorf("mcp server %q: command is required", cfg.Name)
	}
	args := cfg.Args
	if len(args) == 0 && strings.Contains(command, " ") {
		cmd, parsedArgs := splitCommandLine(command)
		command = cmd
		args = parsedArgs
	}

	env := cfg.Env
	if env == nil {
		env = make(map[string]string)
	}

	// Smart normalization for known MCP servers
	// 1. Figma: ensure --figma-api-key flag is present in args if FIGMA_API_KEY is in Env
	if figmaKey := env["FIGMA_API_KEY"]; figmaKey != "" {
		hasFlag := false
		for _, a := range args {
			if strings.HasPrefix(a, "--figma-api-key=") {
				hasFlag = true
				break
			}
		}
		if !hasFlag {
			args = append(args, "--figma-api-key="+figmaKey)
		}
	}

	// 2. Jira & Atlassian: sync URL, Email/Username, and Token aliases across MCP conventions
	if jiraURL := env["JIRA_URL"]; jiraURL != "" && env["JIRA_BASE_URL"] == "" {
		env["JIRA_BASE_URL"] = jiraURL
	} else if jiraBase := env["JIRA_BASE_URL"]; jiraBase != "" && env["JIRA_URL"] == "" {
		env["JIRA_URL"] = jiraBase
	}
	if jiraEmail := env["JIRA_EMAIL"]; jiraEmail != "" && env["JIRA_USERNAME"] == "" {
		env["JIRA_USERNAME"] = jiraEmail
	} else if jiraUser := env["JIRA_USERNAME"]; jiraUser != "" && env["JIRA_EMAIL"] == "" {
		env["JIRA_EMAIL"] = jiraUser
	}
	if jiraToken := env["JIRA_API_TOKEN"]; jiraToken != "" && env["JIRA_TOKEN"] == "" {
		env["JIRA_TOKEN"] = jiraToken
	} else if jToken := env["JIRA_TOKEN"]; jToken != "" && env["JIRA_API_TOKEN"] == "" {
		env["JIRA_API_TOKEN"] = jToken
	}

	// 3. Confluence: sync URL, Email/Username, and Token aliases
	if confURL := env["CONFLUENCE_URL"]; confURL != "" && env["CONFLUENCE_BASE_URL"] == "" {
		env["CONFLUENCE_BASE_URL"] = confURL
	} else if confBase := env["CONFLUENCE_BASE_URL"]; confBase != "" && env["CONFLUENCE_URL"] == "" {
		env["CONFLUENCE_URL"] = confBase
	}
	if confEmail := env["CONFLUENCE_EMAIL"]; confEmail != "" && env["CONFLUENCE_USERNAME"] == "" {
		env["CONFLUENCE_USERNAME"] = confEmail
	} else if confUser := env["CONFLUENCE_USERNAME"]; confUser != "" && env["CONFLUENCE_EMAIL"] == "" {
		env["CONFLUENCE_EMAIL"] = confUser
	}
	if confToken := env["CONFLUENCE_API_TOKEN"]; confToken != "" && env["CONFLUENCE_TOKEN"] == "" {
		env["CONFLUENCE_TOKEN"] = confToken
	} else if cToken := env["CONFLUENCE_TOKEN"]; cToken != "" && env["CONFLUENCE_API_TOKEN"] == "" {
		env["CONFLUENCE_API_TOKEN"] = cToken
	}

	// Inherit Confluence credentials from Jira if Confluence URL is active
	if env["CONFLUENCE_URL"] != "" {
		if env["CONFLUENCE_USERNAME"] == "" && env["JIRA_USERNAME"] != "" {
			env["CONFLUENCE_USERNAME"] = env["JIRA_USERNAME"]
			env["CONFLUENCE_EMAIL"] = env["JIRA_USERNAME"]
		}
		if env["CONFLUENCE_API_TOKEN"] == "" && env["JIRA_API_TOKEN"] != "" {
			env["CONFLUENCE_API_TOKEN"] = env["JIRA_API_TOKEN"]
			env["CONFLUENCE_TOKEN"] = env["JIRA_API_TOKEN"]
		}
	}

	transport, err := newStdioTransport(ctx, command, args, env)
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
	// The subprocess lives on ctx, so only the handshake gets the deadline —
	// a wedged server fails connectServer instead of blocking the pass.
	initCtx, cancelInit := context.WithTimeout(ctx, 30*time.Second)
	res, err := transport.request(initCtx, "initialize", initParams)
	cancelInit()
	if err != nil {
		stderrMsg := strings.TrimSpace(transport.stderrOutput())
		transport.close()
		if stderrMsg != "" {
			return nil, fmt.Errorf("mcp initialize %q: %w (stderr: %s)", cfg.Name, err, stderrMsg)
		}
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
	if ctx == nil {
		ctx = context.Background()
	}
	m.mu.RLock()
	servers := make([]ServerConfig, 0, len(m.servers))
	for _, s := range m.servers {
		if s.Enabled {
			servers = append(servers, s)
		}
	}
	m.mu.RUnlock()

	// Close any transports from a previous run first — replacing the map
	// without closing leaked the old MCP subprocesses and their pipes.
	m.connMu.Lock()
	for _, conn := range m.connections {
		conn.transport.close()
	}
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

// splitCommandLine splits a command string into the command name and arguments,
// preserving single and double quoted segments.
func splitCommandLine(s string) (string, []string) {
	var parts []string
	var current strings.Builder
	inQuote := false
	var quoteChar rune

	for _, r := range strings.TrimSpace(s) {
		switch {
		case inQuote:
			if r == quoteChar {
				inQuote = false
			} else {
				current.WriteRune(r)
			}
		case r == '\'' || r == '"':
			inQuote = true
			quoteChar = r
		case unicode.IsSpace(r):
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	if len(parts) == 0 {
		return "", nil
	}
	return parts[0], parts[1:]
}
