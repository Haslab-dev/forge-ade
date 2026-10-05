package main

import (
	"github.com/hasdev/forge-ade/internal/agentsession"
)

// Disk persistence for Terminal Session records. The storage layouts
// (~/.forge/sessions/<project>/, workspace .forge/.forge-ade dirs, and the
// legacy ~/.forge-ade/sessions mirror) live in internal/agentsession/disk.go;
// these bindings stay on App so the frontend API is unchanged.

// SaveAgentSessionDisk persists an agent session JSON to the per-project
// session directory (and workspace/legacy mirrors).
func (a *App) SaveAgentSessionDisk(sessionJSON string, workspacePath string) error {
	return agentsession.SaveSessionDisk(sessionJSON, workspacePath)
}

// LoadAgentSessionsDisk retrieves all agent sessions from the global,
// workspace, and legacy storage locations (migrating legacy files on read).
func (a *App) LoadAgentSessionsDisk(workspacePath string) ([]string, error) {
	return agentsession.LoadSessionsDisk(workspacePath)
}

// DeleteAgentSessionDisk removes a session JSON from all storage locations.
func (a *App) DeleteAgentSessionDisk(sessionID string, workspacePath string) error {
	return agentsession.DeleteSessionDisk(sessionID, workspacePath)
}
