package app

import (
	"fmt"

	"github.com/hasdev/forge-ade/internal/acp"
)

// ACPStartSession spawns and starts a new ACP session with the specified agent.
func (a *App) ACPStartSession(opts acp.SessionOpts) (*acp.Session, error) {
	if a.acpMgr == nil {
		return nil, fmt.Errorf("acp manager not initialized")
	}
	return a.acpMgr.StartSession(opts)
}

// ACPPrompt sends a user prompt message to an active ACP session.
func (a *App) ACPPrompt(sessionID string, prompt string) error {
	if a.acpMgr == nil {
		return fmt.Errorf("acp manager not initialized")
	}
	return a.acpMgr.Prompt(sessionID, prompt)
}

// ACPCancel sends an execution cancellation signal to an active ACP session.
func (a *App) ACPCancel(sessionID string) error {
	if a.acpMgr == nil {
		return fmt.Errorf("acp manager not initialized")
	}
	return a.acpMgr.Cancel(sessionID)
}

// ACPRespondPermission responds to a pending tool or terminal permission request.
func (a *App) ACPRespondPermission(sessionID string, requestID string, allow bool) error {
	if a.acpMgr == nil {
		return fmt.Errorf("acp manager not initialized")
	}
	return a.acpMgr.RespondPermission(sessionID, requestID, allow)
}

// ACPRespondPermissionOption responds to a pending tool or terminal permission request with a specific option ID or answer.
func (a *App) ACPRespondPermissionOption(sessionID string, requestID string, allow bool, optionID string) error {
	if a.acpMgr == nil {
		return fmt.Errorf("acp manager not initialized")
	}
	return a.acpMgr.RespondPermissionOption(sessionID, requestID, allow, optionID)
}

// ACPCloseSession terminates and closes an ACP session.
func (a *App) ACPCloseSession(sessionID string) error {
	if a.acpMgr == nil {
		return fmt.Errorf("acp manager not initialized")
	}
	return a.acpMgr.CloseSession(sessionID)
}

// ACPListSessions returns all active ACP sessions.
func (a *App) ACPListSessions() []*acp.Session {
	if a.acpMgr == nil {
		return nil
	}
	return a.acpMgr.ListSessions()
}

// ACPGetAvailableCommands returns the available slash commands for an active ACP session.
func (a *App) ACPGetAvailableCommands(sessionID string) []acp.AvailableCommand {
	if a.acpMgr == nil {
		return nil
	}
	return a.acpMgr.GetAvailableCommands(sessionID)
}
