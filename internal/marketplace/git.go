package marketplace

import (
	"os/exec"
)

// execGit runs a git command and returns stdout+stderr combined.
func execGit(args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	return cmd.CombinedOutput()
}
