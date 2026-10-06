package remotefs

import (
	"strings"
	"testing"
)

func TestSSHCommand(t *testing.T) {
	c := &Client{config: SSHConfig{
		ID: "c1", Host: "example.com", Port: 2222, User: "deploy",
		AuthType: "key_file", KeyPath: "/Users/me/.ssh/id_ed25519",
	}}
	exe, args := c.SSHCommand("/var/www/my project")
	if exe != "ssh" {
		t.Fatalf("expected ssh, got %q", exe)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{"-p 2222", "-i /Users/me/.ssh/id_ed25519", "deploy@example.com", "-t", "cd '/var/www/my project' && exec $SHELL -l"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in %q", want, joined)
		}
	}
}

func TestSSHCommandAgentAuth(t *testing.T) {
	c := &Client{config: SSHConfig{
		ID: "c2", Host: "h", Port: 22, User: "u", AuthType: "agent",
	}}
	_, args := c.SSHCommand("/root")
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "-i ") {
		t.Errorf("agent auth should not pass -i: %q", joined)
	}
}

func TestShellQuote(t *testing.T) {
	if got := shellQuote("/a b/c"); got != "'/a b/c'" {
		t.Fatalf("got %q", got)
	}
	if got := shellQuote("/it's"); got != "'/it'\\''s'" {
		t.Fatalf("got %q", got)
	}
}
