package terminal

import (
	"os"
	"strings"
	"testing"
)

// A session with an explicit Executable override runs that argv locally
// (remote shells) instead of the provider shell.
func TestBuildCommandOverride(t *testing.T) {
	s := NewRemoteShell("ssh:host", "ssh://c1/var/www", "ssh",
		[]string{"-p", "22", "deploy@host", "-t", "cd '/var/www' && exec $SHELL -l"}, nil)
	cmd, err := BuildCommand(s)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if cmd.Path == "" || cmd.Args[0] != "ssh" {
		t.Fatalf("expected ssh argv, got %v", cmd.Args)
	}
	if cmd.Dir == "" || cmd.Dir == "ssh://c1/var/www" {
		t.Fatalf("local cwd expected, got %q", cmd.Dir)
	}
}

// Extra env (SSH_ASKPASS wiring) is appended to the inherited environment.
func TestBuildCommandOverrideEnv(t *testing.T) {
	s := NewRemoteShell("ssh:host", "ssh://c1/www", "ssh",
		[]string{"deploy@host"}, []string{"SSH_ASKPASS=/tmp/askpass.sh", "SSH_ASKPASS_REQUIRE=force"})
	cmd, err := BuildCommand(s)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if cmd.Env == nil {
		t.Fatal("expected env to be set")
	}
	joined := strings.Join(cmd.Env, "\n")
	if !strings.Contains(joined, "SSH_ASKPASS=/tmp/askpass.sh") {
		t.Errorf("askpass env missing: %q", joined)
	}
	if !strings.Contains(joined, "PATH=") {
		t.Errorf("inherited environment missing (PATH): %q", joined)
	}
	_ = os.Unsetenv("SSH_ASKPASS")
}
