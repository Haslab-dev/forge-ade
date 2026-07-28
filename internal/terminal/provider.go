package terminal

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// Provider defines how to launch a session type.
type Provider struct {
	Name       string   `yaml:"name" json:"name"`
	Type       string   `yaml:"type" json:"type"` // shell, ai, docker, etc.
	Executable string   `yaml:"executable" json:"executable"`
	Args       []string `yaml:"args" json:"args"`
	Env        []string `yaml:"env,omitempty" json:"env,omitempty"`
}

// DefaultProviders returns the built-in provider definitions.
func DefaultProviders() []Provider {
	return []Provider{
		{
			Name:       "shell",
			Type:       "shell",
			Executable: detectShell(),
			Args:       []string{},
		},
		{
			Name:       "claude",
			Type:       "ai",
			Executable: "claude",
			Args:       []string{},
		},
		{
			Name:       "opencode",
			Type:       "ai",
			Executable: "opencode",
			Args:       []string{},
		},
		{
			Name:       "gemini",
			Type:       "ai",
			Executable: "gemini-cli",
			Args:       []string{},
		},
		{
			Name:       "codex",
			Type:       "ai",
			Executable: "codex",
			Args:       []string{},
		},
		{
			Name:       "aider",
			Type:       "ai",
			Executable: "aider",
			Args:       []string{},
		},
		{
			Name:       "kilo",
			Type:       "ai",
			Executable: "kilo",
			Args:       []string{},
		},
	}
}

// ResolveProvider finds a provider by name.
func ResolveProvider(name string) (*Provider, error) {
	for _, p := range DefaultProviders() {
		if p.Name == name {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("unknown provider: %s", name)
}

// BuildCommand creates an exec.Cmd for a session based on its provider and folder.
func BuildCommand(session *Session) (*exec.Cmd, error) {
	provider, err := ResolveProvider(session.Provider)
	if err != nil {
		return nil, err
	}

	args := append(provider.Args, session.Command)
	if args[len(args)-1] == "" {
		args = args[:len(args)-1]
	}

	cmd := exec.Command(provider.Executable, args...)
	cmd.Dir = session.Folder
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("TERM=xterm-256color"))

	return cmd, nil
}

func detectShell() string {
	if runtime.GOOS == "windows" {
		return "cmd.exe"
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		return "/bin/bash"
	}
	return shell
}
