package app

import (
	"fmt"
	"log"
	"strings"

	"github.com/hasdev/forge-ade/internal/discovery"
	"github.com/hasdev/forge-ade/internal/mcp"
)

// ---------------------------------------------------------------------------
// Multi-source discovery (MCP servers + skills from other agent tools)
// ---------------------------------------------------------------------------

// DiscoverMCPServers aggregates MCP servers from other agent tools' configs
// (Claude, Codex, Cursor, Windsurf, Gemini, opencode, Antigravity) plus our
// own entries (flagged Imported).
func (a *App) DiscoverMCPServers() []discovery.DiscoveredMCPServer {
	return discovery.DiscoverMCPServers(a.mcpMgr.ListServers())
}

// ImportDiscoveredMCPServers imports the named discovered servers into the
// app's own MCP config (enabled) and reconnects.
func (a *App) ImportDiscoveredMCPServers(names []string) error {
	if len(names) == 0 {
		return nil
	}
	found := map[string]discovery.DiscoveredMCPServer{}
	for _, s := range discovery.DiscoverMCPServers(a.mcpMgr.ListServers()) {
		found[strings.ToLower(s.Name)] = s
	}
	imported := 0
	for _, name := range names {
		s, ok := found[strings.ToLower(name)]
		if !ok {
			continue
		}
		if _, err := a.mcpMgr.SaveServer(s.ToServerConfig()); err != nil {
			return fmt.Errorf("import mcp %q: %w", s.Name, err)
		}
		imported++
	}
	if imported > 0 {
		go func() {
			if err := a.mcpMgr.ConnectAll(a.ctx); err != nil {
				log.Printf("mcp: reconnect after import: %v", err)
			}
			a.refreshMCPTools()
			a.emitEvent("agent:config:changed", map[string]interface{}{})
		}()
	}
	log.Printf("[discovery] imported %d MCP servers", imported)
	return nil
}

// DiscoverSkills aggregates skills from other agents' skill directories
// (Claude, AGENTS.md conventions, Antigravity) plus our own (flagged
// Imported).
func (a *App) DiscoverSkills() []discovery.DiscoveredSkill {
	return discovery.DiscoverSkills(a.skillMgr.List())
}

// ImportDiscoveredSkills copies the named discovered skills into the app's
// global skills directory and reloads the skill manager.
func (a *App) ImportDiscoveredSkills(names []string) error {
	if len(names) == 0 {
		return nil
	}
	found := map[string]discovery.DiscoveredSkill{}
	for _, s := range discovery.DiscoverSkills(a.skillMgr.List()) {
		found[strings.ToLower(s.Name)] = s
	}
	imported := 0
	for _, name := range names {
		s, ok := found[strings.ToLower(name)]
		if !ok || s.Path == "" {
			continue
		}
		if _, err := discovery.CopySkill(s.Path); err != nil {
			return fmt.Errorf("import skill %q: %w", s.Name, err)
		}
		imported++
	}
	if imported > 0 {
		a.skillMgr.Reload()
		a.emitEvent("agent:config:changed", map[string]interface{}{})
	}
	log.Printf("[discovery] imported %d skills", imported)
	return nil
}

// ListMCPTools returns loaded MCP tools.
func (a *App) ListMCPTools() []mcp.Tool {
	return a.mcpMgr.ListTools()
}

// ListConnectedMCPTools returns the tools discovered from live MCP connections
// (the actual tools the agent can call).
func (a *App) ListConnectedMCPTools() []mcp.Tool {
	return a.mcpMgr.ListConnectedTools()
}

// ListMCPServers returns configured MCP servers.
func (a *App) ListMCPServers() []mcp.ServerConfig {
	return a.mcpMgr.ListServers()
}

// SaveMCPServer creates or updates an MCP server (GUI-configured).
func (a *App) SaveMCPServer(s mcp.ServerConfig) (mcp.ServerConfig, error) {
	return a.mcpMgr.SaveServer(s)
}

// DeleteMCPServer removes an MCP server by name.
func (a *App) DeleteMCPServer(name string) error {
	return a.mcpMgr.DeleteServer(name)
}

// ReconnectMCP reconnects to all enabled MCP servers and refreshes the tool
// registry. Called after the user edits MCP server config.
func (a *App) ReconnectMCP() error {
	a.mcpMgr.DisconnectAll()
	if err := a.mcpMgr.ConnectAll(a.ctx); err != nil {
		return err
	}
	a.refreshMCPTools()
	return nil
}
