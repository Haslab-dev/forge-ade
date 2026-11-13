package app

import (
	"os"

	"github.com/Haslab-dev/RakitKode/sandbox"
)

// GetSandboxDir provisions (if needed) and returns the agent sandbox root
// for the given project — ~/.forge/sandbox/<slug> with tmp/, thinking/,
// coding/ and logs/ sub directories. An empty projectPath resolves against
// the session default folder.
func (a *App) GetSandboxDir(projectPath string) (string, error) {
	if projectPath == "" {
		projectPath = a.defaultFolder()
	}
	return sandbox.Ensure(projectPath)
}

// ClearSandbox removes the agent sandbox for the given project. Destructive
// and user-initiated (Settings → General); the sandbox is re-provisioned on
// the next agent turn.
func (a *App) ClearSandbox(projectPath string) error {
	if projectPath == "" {
		projectPath = a.defaultFolder()
	}
	root, err := sandbox.Ensure(projectPath)
	if err != nil {
		return err
	}
	return os.RemoveAll(root)
}
