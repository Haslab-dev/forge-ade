package app

import (
	"github.com/hasdev/forge-ade/internal/agentsession"
	"github.com/hasdev/forge-ade/internal/terminal"
)

// ---------------------------------------------------------------------------
// Terminal Session mode API — sessions, agent CLI configs, app settings.
//
// Sessions own their process/PTY; ForgeADE never interprets agent output.
// ---------------------------------------------------------------------------

// GetAppSettings returns the persisted application settings (default mode,
// terminal rendering).
func (a *App) GetAppSettings() agentsession.AppSettings {
	return a.appSettings.Get()
}

// SaveAppSettings validates and persists application settings. Terminal
// settings apply to newly created sessions/shells immediately.
func (a *App) SaveAppSettings(settings agentsession.AppSettings) (agentsession.AppSettings, error) {
	saved, err := a.appSettings.Save(settings)
	if err != nil {
		return saved, err
	}
	terminal.SetDefaultShell(saved.Terminal.Shell)
	return saved, nil
}

// ListAgentCLIConfigs returns all configured agent CLIs.
func (a *App) ListAgentCLIConfigs() []agentsession.AgentCLIConfig {
	return a.agentCLIConfigs.List()
}

// SaveAgentCLIConfig upserts one agent CLI configuration.
func (a *App) SaveAgentCLIConfig(cfg agentsession.AgentCLIConfig) (agentsession.AgentCLIConfig, error) {
	return a.agentCLIConfigs.Save(cfg)
}

// ResetAgentCLIConfig restores one agent CLI to its built-in defaults.
func (a *App) ResetAgentCLIConfig(id string) (agentsession.AgentCLIConfig, error) {
	return a.agentCLIConfigs.Reset(id)
}

// DetectAgentExecutable reports whether an executable resolves on this
// machine (Settings → Agent CLIs → Detect).
func (a *App) DetectAgentExecutable(executable string) bool {
	return agentsession.DetectExecutable(executable)
}

// CreateAgentTerminalSession spawns an agent CLI in a dedicated PTY session.
func (a *App) CreateAgentTerminalSession(agentID, workspacePath, title string) (agentsession.Session, error) {
	return a.agentSessions.Create(agentID, workspacePath, title)
}

// ListAgentTerminalSessions returns all session records (running + history).
func (a *App) ListAgentTerminalSessions() []agentsession.Session {
	return a.agentSessions.List()
}

// GetAgentTerminalSessionOutput returns the session's persisted terminal
// output for replay into a terminal renderer.
func (a *App) GetAgentTerminalSessionOutput(id string) string {
	return a.agentSessions.OutputTail(id)
}

// RestartAgentTerminalSession relaunches the session's agent CLI within the
// same session record.
func (a *App) RestartAgentTerminalSession(id string) (agentsession.Session, error) {
	return a.agentSessions.Restart(id)
}

// DeleteAgentTerminalSession removes a session and its history (stopping it
// first when running).
func (a *App) DeleteAgentTerminalSession(id string) error {
	return a.agentSessions.Delete(id)
}

// The generic Write/Stop bindings fall back to the agent session manager when
// the id is not a plain shell session, so the shared terminal renderer drives
// both session kinds through one API. (Resize fallback lives inline in
// terminal_api.go, which needs the underlying error.)
func (a *App) writeAgentSession(id string, data string) bool {
	if _, ok := a.agentSessions.Get(id); !ok {
		return false
	}
	_, err := a.agentSessions.Write(id, []byte(data))
	return err == nil
}

func (a *App) stopAgentSession(id string) bool {
	if _, ok := a.agentSessions.Get(id); !ok {
		return false
	}
	return a.agentSessions.Stop(id) == nil
}
