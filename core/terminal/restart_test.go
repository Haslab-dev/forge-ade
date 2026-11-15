package terminal

import (
	"strings"
	"testing"
)

// TestRebuildForRestartKeepsIdentity verifies a timeout restart rebuilds the
// SAME session kind: a remote/SSH session keeps its executable argv, env and
// isolation flag instead of being replaced by a plain local shell.
func TestRebuildForRestartKeepsIdentity(t *testing.T) {
	orig := NewRemoteShell("deploy", "ssh://conn-1/var/www", "ssh",
		[]string{"-p", "2222", "-o", "StrictHostKeyChecking=accept-new", "user@host", "-t", "cd /var/www"},
		[]string{"SSH_ASKPASS=/tmp/askpass", "SSH_ASKPASS_REQUIRE=force"})
	orig.Isolated = true

	rebuilt := orig.rebuildForRestart()

	if rebuilt.Executable != orig.Executable {
		t.Errorf("executable %q, want %q", rebuilt.Executable, orig.Executable)
	}
	if strings.Join(rebuilt.Args, " ") != strings.Join(orig.Args, " ") {
		t.Errorf("args %v, want %v", rebuilt.Args, orig.Args)
	}
	if strings.Join(rebuilt.Env, " ") != strings.Join(orig.Env, " ") {
		t.Errorf("env %v, want %v", rebuilt.Env, orig.Env)
	}
	if !rebuilt.Isolated {
		t.Error("isolated flag lost on rebuild")
	}
	if rebuilt.Folder != orig.Folder || rebuilt.Name != orig.Name || rebuilt.Type != orig.Type {
		t.Errorf("identity mismatch: got %+v", rebuilt)
	}
	// Deep copies: mutating the rebuild must not touch the original.
	rebuilt.Args[0] = "MUTATED"
	if orig.Args[0] == "MUTATED" {
		t.Error("rebuild shares the args backing array with the original")
	}
}

// TestRebuildForRestartLocalShell keeps local shells working through restarts.
func TestRebuildForRestartLocalShell(t *testing.T) {
	orig := NewShell("ops", "/tmp")
	orig.Command = "watch-mode"

	rebuilt := orig.rebuildForRestart()
	if rebuilt.Executable != "" || rebuilt.Provider != "shell" || rebuilt.Folder != "/tmp" {
		t.Errorf("local shell identity broken: %+v", rebuilt)
	}
	if rebuilt.Command != orig.Command {
		t.Errorf("command override %q, want %q", rebuilt.Command, orig.Command)
	}
}

// TestBuildCommandUsesProcessGroup verifies children are started in their own
// process group so Stop can kill the whole tree, not just the direct PID.
func TestBuildCommandUsesProcessGroup(t *testing.T) {
	sess := NewRemoteShell("r", "ssh://c/x", "echo", []string{"hi"}, nil)
	cmd, err := BuildCommand(sess)
	if err != nil {
		t.Fatalf("BuildCommand: %v", err)
	}
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.Setpgid {
		t.Error("expected SysProcAttr.Setpgid=true so the process runs in its own group")
	}

	local := NewShell("s", "")
	cmd2, err := BuildCommand(local)
	if err != nil {
		t.Fatalf("BuildCommand local: %v", err)
	}
	if cmd2.SysProcAttr == nil || !cmd2.SysProcAttr.Setpgid {
		t.Error("expected SysProcAttr.Setpgid=true for local shells too")
	}
}
