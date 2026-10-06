package main

import (
	"fmt"
	"strings"

	"github.com/hasdev/forge-ade/internal/terminal"
)

// ---------------------------------------------------------------------------
// Session API (Unified — Shell + AI Agents + Docker, etc.)
// ---------------------------------------------------------------------------

// CreateShell creates a new shell session. An ssh:// folder opens a remote
// shell (a local ssh process under the PTY) instead of silently falling back
// to a local one.
func (a *App) CreateShell(name, folder string) (*terminal.Session, error) {
	if strings.HasPrefix(folder, "ssh://") {
		client, remotePath, isRemote := a.remoteFS.ParseRemotePath(folder)
		if !isRemote || client == nil {
			return nil, fmt.Errorf("ssh connection lost — reconnect via the SSH dialog before opening a remote shell")
		}
		exe, args := client.SSHCommand(remotePath)
		return a.sessionMgr.CreateRemoteShell(name, folder, exe, args)
	}
	if folder == "" {
		folder = a.defaultFolder()
	}
	return a.sessionMgr.CreateShell(name, folder)
}

// CreateAIAgent creates a new AI agent session.
func (a *App) CreateAIAgent(name, provider, folder string) (*terminal.Session, error) {
	if folder == "" {
		folder = a.defaultFolder()
	}
	return a.sessionMgr.CreateAIAgent(name, provider, folder)
}

// WriteSession writes data to a session's stdin (shell or agent session).
func (a *App) WriteSession(id string, data string) error {
	_, err := a.sessionMgr.Write(id, []byte(data))
	if err != nil && a.writeAgentSession(id, data) {
		return nil
	}
	return err
}

// ResizeSession resizes a session's PTY (shell or agent session).
func (a *App) ResizeSession(id string, rows, cols uint16) error {
	err := a.sessionMgr.Resize(id, rows, cols)
	if err == nil {
		return nil
	}
	if _, ok := a.agentSessions.Get(id); ok {
		return a.agentSessions.Resize(id, rows, cols)
	}
	return err
}

// StopSession terminates a session (shell or agent session).
func (a *App) StopSession(id string) error {
	err := a.sessionMgr.Stop(id)
	if err != nil && a.stopAgentSession(id) {
		return nil
	}
	return err
}

// RenameSession renames a session.
func (a *App) RenameSession(id, name string) error {
	return a.sessionMgr.Rename(id, name)
}

// ListSessions returns all active sessions.
func (a *App) ListSessions() []*terminal.Session {
	return a.sessionMgr.List()
}

// ListShells returns only shell sessions.
func (a *App) ListShells() []*terminal.Session {
	return a.sessionMgr.ListByType(terminal.SessionShell)
}

// ListAIAgents returns only AI agent sessions.
func (a *App) ListAIAgents() []*terminal.Session {
	return a.sessionMgr.ListByType(terminal.SessionAI)
}
