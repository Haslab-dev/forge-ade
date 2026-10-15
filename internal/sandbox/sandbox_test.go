package sandbox

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func home(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

func TestEnsureCreatesSandboxWithSubDirs(t *testing.T) {
	home(t)
	root, err := Ensure("/ws/proj")
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	for _, sub := range subDirs {
		info, err := os.Stat(filepath.Join(root, sub))
		if err != nil {
			t.Fatalf("sub dir %s missing: %v", sub, err)
		}
		if !info.IsDir() {
			t.Fatalf("%s is not a directory", sub)
		}
	}
	// Under ~/.forge/sandbox/<slug>
	if got := filepath.Base(filepath.Dir(root)); got != "sandbox" {
		t.Fatalf("sandbox root parent = %q, want sandbox", got)
	}
	if filepath.IsAbs(filepath.Base(root)) || filepath.Base(root) == "" {
		t.Fatalf("unexpected slug %q", filepath.Base(root))
	}
}

func TestEnsureIdempotent(t *testing.T) {
	home(t)
	first, err := Ensure("/ws/proj")
	if err != nil {
		t.Fatalf("first Ensure: %v", err)
	}
	if err := os.WriteFile(filepath.Join(first, DirTmp, "keep.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	second, err := Ensure("/ws/proj")
	if err != nil {
		t.Fatalf("second Ensure: %v", err)
	}
	if second != first {
		t.Fatalf("root changed: %q != %q", second, first)
	}
	if _, err := os.Stat(filepath.Join(first, DirTmp, "keep.txt")); err != nil {
		t.Fatalf("Ensure clobbered existing files: %v", err)
	}
}

func TestEnsureSeparatesSameNamedProjects(t *testing.T) {
	home(t)
	a, err := Ensure("/ws/alpha/proj")
	if err != nil {
		t.Fatal(err)
	}
	b, err := Ensure("/ws/beta/proj")
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatalf("same-name projects share a sandbox: %q", a)
	}
	// Both share the leaf prefix, differing in the hash suffix.
	if filepath.Base(a)[:4] != filepath.Base(b)[:4] {
		t.Fatalf("slugs %q and %q do not share the leaf prefix", a, b)
	}
}

func TestSubReturnsProvisionedSubDir(t *testing.T) {
	home(t)
	sub, err := Sub("/ws/proj", DirThinking)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(sub)
	if err != nil || !info.IsDir() {
		t.Fatalf("thinking dir not provisioned: %v", err)
	}
	if filepath.Base(sub) != DirThinking {
		t.Fatalf("sub = %q", sub)
	}
}

func TestUnder(t *testing.T) {
	home(t)
	root, err := Ensure("/ws/proj")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		path string
		want bool
	}{
		{filepath.Join(root, "tmp", "x.txt"), true},
		{root, true},
		{filepath.Join(root, "thinking"), true},
		{root + "-sibling/x", false},
		{"/etc/passwd", false},
		{"/ws/proj", false}, // workspace itself is outside the sandbox
	}
	for _, tc := range cases {
		if got := Under("/ws/proj", tc.path); got != tc.want {
			t.Errorf("Under(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
	// Unprovisionable project (HOME broken) fails closed.
	t.Setenv("HOME", "")
	if _, err := os.UserHomeDir(); err != nil {
		if Under("/ws/proj", filepath.Join(root, "tmp", "x")) {
			t.Error("Under must fail closed when the sandbox root cannot be resolved")
		}
	}
}

func TestSweepTempRemovesOnlyOldTmpEntries(t *testing.T) {
	home(t)
	root, err := Ensure("/ws/proj")
	if err != nil {
		t.Fatal(err)
	}

	oldFile := filepath.Join(root, DirTmp, "old.txt")
	if err := os.WriteFile(oldFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(oldFile, past, past); err != nil {
		t.Fatal(err)
	}
	oldDir := filepath.Join(root, DirTmp, "olddir")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(oldDir, past, past); err != nil {
		t.Fatal(err)
	}

	freshFile := filepath.Join(root, DirTmp, "fresh.txt")
	if err := os.WriteFile(freshFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	keepThinking := filepath.Join(root, DirThinking, "note.md")
	if err := os.WriteFile(keepThinking, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A thinking entry older than the cutoff must survive the sweep.
	if err := os.Chtimes(keepThinking, past, past); err != nil {
		t.Fatal(err)
	}

	SweepTemp(time.Hour)

	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Errorf("old tmp file survived sweep (err=%v)", err)
	}
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Errorf("old tmp dir survived sweep (err=%v)", err)
	}
	if _, err := os.Stat(freshFile); err != nil {
		t.Errorf("fresh tmp file was swept: %v", err)
	}
	if _, err := os.Stat(keepThinking); err != nil {
		t.Errorf("thinking entry was swept: %v", err)
	}
}

func TestSweepTempAcrossAllSandboxes(t *testing.T) {
	home(t)
	rootA, err := Ensure("/ws/a")
	if err != nil {
		t.Fatal(err)
	}
	rootB, err := Ensure("/ws/b")
	if err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{rootA, rootB} {
		old := filepath.Join(root, DirTmp, "stale.txt")
		if err := os.WriteFile(old, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		past := time.Now().Add(-48 * time.Hour)
		if err := os.Chtimes(old, past, past); err != nil {
			t.Fatal(err)
		}
	}
	SweepTemp(time.Hour)
	for _, root := range []string{rootA, rootB} {
		if _, err := os.Stat(filepath.Join(root, DirTmp, "stale.txt")); !os.IsNotExist(err) {
			t.Errorf("stale entry in %s survived sweep", root)
		}
	}
}
