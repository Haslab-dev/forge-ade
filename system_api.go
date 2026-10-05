package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func getDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".forge-ade")
	}
	dir := filepath.Join(home, ".forge-ade")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return filepath.Join(".", ".forge-ade")
	}
	return dir
}

// ResolvePath expands a leading ~ and resolves relative paths against the
// current working directory, returning a cleaned absolute path.
func ResolvePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("empty path")
	}
	if strings.HasPrefix(path, "~/") || path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if path == "~" {
			return home, nil
		}
		path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
	}
	return filepath.Abs(path)
}

// OpenNewWindow launches a new ForgeADE window as a separate OS process.
// Optional workspacePath opens the given folder or .workspace file in it.
func (a *App) OpenNewWindow(workspacePath string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("new window: resolve executable: %w", err)
	}

	args := []string{}
	if workspacePath != "" {
		resolved, err := ResolvePath(workspacePath)
		if err != nil {
			return fmt.Errorf("new window: resolve path: %w", err)
		}
		args = append(args, resolved)
	}

	cmd := exec.Command(exe, args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("new window: launch: %w", err)
	}
	// Detach from the parent so the new process outlives this one.
	_ = cmd.Process.Release()
	return nil
}

type CommandResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exitCode"`
}

// ExecuteCommandSync executes a command with /dev/null stdin so CLI agents don't hang.
func (a *App) ExecuteCommandSync(command, cwd string) CommandResult {
	if cwd == "" {
		cwd = a.defaultFolder()
	}
	cmd := exec.Command("bash", "-c", command)
	cmd.Dir = cwd
	cmd.Stdin = nil // no interactive stdin

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}
	return CommandResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}
}

// acpMCPAdapter adapts the MCP manager to the acp.MCPLister interface.
