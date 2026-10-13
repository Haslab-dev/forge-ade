package terminal

import (
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
	// Explicit override (remote shells): run the given argv locally, under
	// the user's home dir — the remote side provides its own working dir.
	if session.Executable != "" {
		cmd := exec.Command(session.Executable, session.Args...)
		envMap := ptyEnv()
		// Connection-specific env (SSH_ASKPASS wiring) wins over the base.
		for _, e := range session.Env {
			if k, v, ok := strings.Cut(e, "="); ok {
				envMap[k] = v
			}
		}
		cmd.Env = envList(envMap)
		if home, err := os.UserHomeDir(); err == nil {
			cmd.Dir = home
		}
		return cmd, nil
	}

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
	// An empty folder, remote URI (ssh://), or non-existent path would cause
	// pty.StartWithSize to fail with "chdir: no such file or directory" — drop
	// shells at the user's home directory instead.
	if cmd.Dir == "" || strings.HasPrefix(cmd.Dir, "ssh://") {
		if home, err := os.UserHomeDir(); err == nil {
			cmd.Dir = home
		}
	} else if stat, err := os.Stat(cmd.Dir); err != nil || !stat.IsDir() {
		if home, err := os.UserHomeDir(); err == nil {
			cmd.Dir = home
		}
	}

	// Inherit process environment and set terminal emulation variables
	cmd.Env = envList(ptyEnv())

	return cmd, nil
}

// ptyEnv builds the base environment for a PTY child: the app's environment
// with a full PATH plus explicit terminal/locale variables. Remote shells need
// this as much as local ones — ssh forwards TERM to the remote host in its
// pty request, and a GUI-launched app carries no TERM of its own, which left
// remote shells with a dumb terminal: no colors, `clear` broken.
func ptyEnv() map[string]string {
	envMap := make(map[string]string)
	for _, e := range os.Environ() {
		if k, v, ok := strings.Cut(e, "="); ok {
			envMap[k] = v
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

	return envMap
}

// envList flattens an environment map into NAME=VALUE entries.
func envList(envMap map[string]string) []string {
	list := make([]string, 0, len(envMap))
	for k, v := range envMap {
		list = append(list, k+"="+v)
	}
	return list
}

// defaultShellOverride, when non-empty, replaces the auto-detected login
// shell (Settings → Terminal → Shell). Set via SetDefaultShell.
var defaultShellOverride string

// SetDefaultShell overrides the shell used for new shell sessions (empty
// restores auto-detection from $SHELL).
func SetDefaultShell(shell string) {
	defaultShellOverride = shell
}

func detectShell() (string, []string) {
	if runtime.GOOS == "windows" {
		return "powershell.exe", []string{"-NoLogo"}
	}
	shell := defaultShellOverride
	if shell == "" {
		shell = os.Getenv("SHELL")
	}
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
