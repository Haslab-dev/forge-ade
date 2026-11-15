//go:build windows

package terminal

import (
	"os/exec"
)

// setProcessGroup is a no-op on Windows: Job objects would be required for
// true tree kills, and the direct Process.Kill fallback applies there.
func setProcessGroup(cmd *exec.Cmd) {}

// killProcessGroup has no direct Windows equivalent here; the caller falls
// back to the direct kill.
func killProcessGroup(pid int) error { return nil }

// killDirect is handled by cmd.Process.Kill at the call site on Windows.
func killDirect(pid int) error { _ = pid; return nil }
