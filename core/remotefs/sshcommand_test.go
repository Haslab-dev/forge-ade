package remotefs

import (
	"os"
	"strings"
	"testing"
)

func TestSSHCommand(t *testing.T) {
	c := &Client{config: SSHConfig{
		ID: "c1", Host: "example.com", Port: 2222, User: "deploy",
		AuthType: "key_file", KeyPath: "/Users/me/.ssh/id_ed25519",
	}}
	exe, args, env := c.SSHCommand("/var/www/my project")
	if exe != "ssh" {
		t.Fatalf("expected ssh, got %q", exe)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{"-p 2222", "-i /Users/me/.ssh/id_ed25519", "deploy@example.com", "-t", "cd '/var/www/my project' && exec $SHELL -l"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in %q", want, joined)
		}
	}
	// Key auth without passphrase must not wire an askpass helper.
	if len(env) != 0 {
		t.Errorf("expected no askpass env for key auth, got %v", env)
	}
}

func TestSSHCommandPasswordAskpass(t *testing.T) {
	c := &Client{config: SSHConfig{
		ID: "c1", Host: "h", Port: 22, User: "u",
		AuthType: "password", Password: "se'cret",
	}}
	_, _, env := c.SSHCommand("/")
	joined := strings.Join(env, " ")
	for _, want := range []string{"SSH_ASKPASS=", "SSH_ASKPASS_REQUIRE=force"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in %q", want, joined)
		}
	}
	// The askpass script must exist, quote the password safely, and clean up.
	script := strings.SplitN(env[0], "=", 2)[1]
	body, err := os.ReadFile(script)
	if err != nil {
		t.Fatalf("askpass script missing: %v", err)
	}
	if !strings.Contains(string(body), `echo 'se'\''cret'`) {
		t.Errorf("password not quoted correctly: %q", string(body))
	}
	if err := c.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, err := os.Stat(script); !os.IsNotExist(err) {
		t.Errorf("askpass script not removed on Close: %v", err)
	}
}

func TestSSHCommandAgentAuth(t *testing.T) {
	c := &Client{config: SSHConfig{
		ID: "c2", Host: "h", Port: 22, User: "u", AuthType: "agent",
	}}
	_, args, _ := c.SSHCommand("/root")
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
