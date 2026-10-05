package main

import (
	"fmt"
	"log"

	"github.com/hasdev/forge-ade/internal/remotefs"
)

// ---------------------------------------------------------------------------
// SSH / Remote SFTP API
// ---------------------------------------------------------------------------

// ConnectSSH establishes a remote SSH/SFTP connection.
func (a *App) ConnectSSH(cfg remotefs.SSHConfig) (*remotefs.ConnectionStatus, error) {
	return a.remoteFS.Connect(cfg)
}

// DisconnectSSH closes an active remote SSH/SFTP session.
func (a *App) DisconnectSSH(connID string) error {
	return a.remoteFS.Disconnect(connID)
}

// ListSSHConnections lists all active remote SSH/SFTP sessions.
func (a *App) ListSSHConnections() []remotefs.ConnectionStatus {
	return a.remoteFS.ListConnections()
}

// OpenSSHWorkspace connects to SSH and sets the active workspace to the remote directory.
func (a *App) OpenSSHWorkspace(cfg remotefs.SSHConfig) (*remotefs.ConnectionStatus, error) {
	status, err := a.remoteFS.Connect(cfg)
	if err != nil {
		return status, err
	}
	remoteURI := fmt.Sprintf("ssh://%s%s", status.ID, status.RemotePath)
	if _, err := a.OpenFolder(remoteURI); err != nil {
		log.Printf("[ssh] connected but failed to open remote workspace %s: %v", remoteURI, err)
	}
	return status, nil
}
