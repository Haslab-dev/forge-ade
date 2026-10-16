package app

import (
	"github.com/Haslab-dev/RakitKode/agent"
	"github.com/hasdev/forge-ade/internal/terminal"
)

// agentShellManager adapts the desktop PTY terminal manager to the engine's
// ShellManager interface — CreateShell returns the concrete terminal session,
// which satisfies agent.ShellSession via GetID.
type agentShellManager struct {
	*terminal.Manager
}

func (a agentShellManager) CreateShell(name, folder string) (agent.ShellSession, error) {
	return a.Manager.CreateShell(name, folder)
}

// wrapShellManager returns nil for a nil manager so the engine falls back to
// one-shot shells instead of holding a non-nil wrapper around nil.
func wrapShellManager(m *terminal.Manager) agent.ShellManager {
	if m == nil {
		return nil
	}
	return agentShellManager{Manager: m}
}
