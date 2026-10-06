package terminal

import "testing"

// A session with an explicit Executable override runs that argv locally
// (remote shells) instead of the provider shell.
func TestBuildCommandOverride(t *testing.T) {
	s := NewRemoteShell("ssh:host", "ssh://c1/var/www", "ssh",
		[]string{"-p", "22", "deploy@host", "-t", "cd '/var/www' && exec $SHELL -l"})
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
