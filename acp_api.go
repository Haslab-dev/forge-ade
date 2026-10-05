package main

import (
	"context"

	"github.com/hasdev/forge-ade/internal/acp"
	"github.com/hasdev/forge-ade/internal/mcp"
)

// ---------------------------------------------------------------------------
// ACP API (Agent Client Protocol for Pi, OMP, and external agents)
// ---------------------------------------------------------------------------

// AcpListAgents returns all configured ACP agents.
func (a *App) AcpListAgents() []acp.AgentConfig {
	return a.acpMgr.ListAgents()
}

// AcpSaveAgent creates or updates an external ACP agent config.
func (a *App) AcpSaveAgent(cfg acp.AgentConfig) (acp.AgentConfig, error) {
	return a.acpMgr.SaveAgent(cfg)
}

// AcpDeleteAgent removes an external ACP agent config.
func (a *App) AcpDeleteAgent(id string) error {
	return a.acpMgr.DeleteAgent(id)
}

// AcpToggleAgent enables or disables an external ACP agent.
func (a *App) AcpToggleAgent(id string, enabled bool) error {
	return a.acpMgr.ToggleAgent(id, enabled)
}

// AcpCheckAgentBinary checks if the given executable is found in system PATH.
func (a *App) AcpCheckAgentBinary(command string) (bool, string) {
	return a.acpMgr.CheckBinary(command)
}

// AcpListSessions returns all active ACP sessions.
func (a *App) AcpListSessions() []acp.ACPSession {
	return a.acpMgr.ListSessions()
}

// AcpGetSession returns a single ACP session by ID.
func (a *App) AcpGetSession(id string) (acp.ACPSession, bool) {
	return a.acpMgr.GetSession(id)
}

// AcpCreateSession opens a new session with an external ACP agent.
func (a *App) AcpCreateSession(agentID, name, folder string) (acp.ACPSession, error) {
	if folder == "" {
		if ws := a.workspaceMgr.Current(); ws != nil && len(ws.GetFolders()) > 0 {
			folder = ws.GetFolders()[0]
		}
	}
	return a.acpMgr.CreateSession(context.Background(), agentID, name, folder)
}

// AcpPrompt sends a user message to an active ACP session with optional mentioned files.
func (a *App) AcpPrompt(sessionID, text string, mentionedFiles []string) error {
	return a.acpMgr.Send(context.Background(), sessionID, text, mentionedFiles)
}

// AcpCancel aborts the current in-flight prompt for an ACP session.
func (a *App) AcpCancel(sessionID string) {
	a.acpMgr.Cancel(sessionID)
}

// AcpRespondPermission responds to an outstanding permission request from an ACP agent.
func (a *App) AcpRespondPermission(sessionID, optionID string, cancel bool) error {
	return a.acpMgr.RespondPermission(sessionID, optionID, cancel)
}

// AcpCloseSession cancels the session's active turn, keeping the session
// alive. Kept as a separate binding for frontend API stability; ACP sessions
// close on process exit (there is no per-session teardown in the manager).
func (a *App) AcpCloseSession(sessionID string) {
	a.acpMgr.Cancel(sessionID)
}

// AcpSetSessionModel configures the active model for an ACP session.
func (a *App) AcpSetSessionModel(sessionID, model string) error {
	return a.acpMgr.SetSessionModel(context.Background(), sessionID, model)
}

// AcpGetAgentModels returns available models for a given agent.
func (a *App) AcpGetAgentModels(agentID string) []string {
	return a.acpMgr.GetAgentModels(agentID)
}

// AcpGetSlashCommands returns slash commands and discovered skills for a given agent.
func (a *App) AcpGetSlashCommands(agentID string) []acp.SlashCommandItem {
	return a.acpMgr.GetSlashCommandsAndSkills(agentID)
}

type acpMCPAdapter struct{ m *mcp.Manager }

func (a acpMCPAdapter) ListServers() []acp.MCPServerInfo {
	out := []acp.MCPServerInfo{}
	for _, s := range a.m.ListServers() {
		out = append(out, acp.MCPServerInfo{Name: s.Name})
	}
	return out
}

func (a acpMCPAdapter) ListTools() []acp.MCPToolInfo {
	out := []acp.MCPToolInfo{}
	for _, t := range a.m.ListTools() {
		out = append(out, acp.MCPToolInfo{ServerName: t.ServerName, Name: t.Name, Description: t.Description})
	}
	return out
}
