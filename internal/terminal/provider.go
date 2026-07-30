package terminal

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
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
	shellExe, shellArgs := detectShell()
	return []Provider{
		{
			Name:       "shell",
			Type:       "shell",
			Executable: shellExe,
			Args:       shellArgs,
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
	// Fallback to shell
	shellExe, shellArgs := detectShell()
	return &Provider{
		Name:       "shell",
		Type:       "shell",
		Executable: shellExe,
		Args:       shellArgs,
	}, nil
}

// BuildCommand creates an exec.Cmd for a session based on its provider and folder.
func BuildCommand(session *Session) (*exec.Cmd, error) {
	provider, err := ResolveProvider(session.Provider)
	if err != nil {
		return nil, err
	}

	args := append([]string{}, provider.Args...)
	if session.Command != "" {
		args = append(args, session.Command)
	}

	cmd := exec.Command(provider.Executable, args...)
	cmd.Dir = session.Folder

	// Inherit process environment and set terminal emulation variables
	envMap := make(map[string]string)
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	// Ensure full PATH including Homebrew, Xcode, Node, Cargo, Go binaries for iOS React Native builds
	currentPath := envMap["PATH"]
	homeDir, _ := os.UserHomeDir()
	extraPaths := []string{
		"/opt/homebrew/bin",
		"/opt/homebrew/sbin",
		homeDir + "/homebrew/bin",
		homeDir + "/.cargo/bin",
		homeDir + "/go/bin",
		homeDir + "/.bun/bin",
		"/usr/local/bin",
		"/usr/bin",
		"/bin",
		"/usr/sbin",
		"/sbin",
	}
	for _, p := range extraPaths {
		if p != "" && !strings.Contains(currentPath, p) {
			currentPath = p + ":" + currentPath
		}
	}
	envMap["PATH"] = currentPath
	envMap["TERM"] = "xterm-256color"
	envMap["COLORTERM"] = "truecolor"
	envMap["LANG"] = "en_US.UTF-8"
	envMap["LC_ALL"] = "en_US.UTF-8"

	envList := make([]string, 0, len(envMap))
	for k, v := range envMap {
		envList = append(envList, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = envList

	return cmd, nil
}

func detectShell() (string, []string) {
	if runtime.GOOS == "windows" {
		return "powershell.exe", []string{"-NoLogo"}
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		if _, err := os.Stat("/bin/zsh"); err == nil {
			shell = "/bin/zsh"
		} else {
			shell = "/bin/bash"
		}
	}
	// Pass -l (login shell) like VSCode terminal to source .zshrc / .zprofile
	return shell, []string{"-l"}
}
