//go:build !windows

package terminal

import (
	"os/exec"
	"syscall"
)

// setProcessGroup puts the child in its own process group so Stop can kill
// the whole tree: shells spawn grandchildren (scripts, ssh, editors) that a
// direct PID kill leaves running.
func setProcessGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

// killProcessGroup SIGKILLs the child's entire process group.
func killProcessGroup(pid int) error {
	return syscall.Kill(-pid, syscall.SIGKILL)
}

// killDirect SIGKILLs a single PID (fallback when the group kill fails,
// e.g. the process already left its group).
func killDirect(pid int) error {
	return syscall.Kill(pid, syscall.SIGKILL)
}
