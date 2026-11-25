//go:build windows

package browseruse

import (
	"os/exec"
)

func setBrowserProcessGroup(_ *exec.Cmd) {}

func killBrowserProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
}
