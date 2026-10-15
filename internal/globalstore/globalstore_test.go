package globalstore

import (
	"crypto/sha1"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// BUG-009 note: ProjectSlug hashes filepath.Abs(workspacePath), so relative
// inputs are resolved against the process working directory. The tests below
// therefore pin behavior for absolute paths (stable, cwd-independent) and use
// t.Chdir when exercising relative inputs.

func TestProjectSlug(t *testing.T) {
	base := t.TempDir()
	proj := filepath.Join(base, "proj")

	t.Run("stable across calls", func(t *testing.T) {
		first := ProjectSlug(proj)
		if first != ProjectSlug(proj) {
			t.Fatalf("slug not stable for %s: %q vs %q", proj, first, ProjectSlug(proj))
		}

		t.Chdir(base)
		relFirst := ProjectSlug("nested")
		if relFirst != ProjectSlug("nested") {
			t.Fatalf("slug not stable for relative path: %q vs %q", relFirst, ProjectSlug("nested"))
		}
	})

	t.Run("distinct paths get distinct slugs", func(t *testing.T) {
		a := ProjectSlug(filepath.Join(base, "alpha"))
		b := ProjectSlug(filepath.Join(base, "beta"))
		if a == b {
			t.Fatalf("distinct paths produced identical slug %q", a)
		}
	})

	t.Run("same leaf in different dirs shares leaf prefix but differs in hash", func(t *testing.T) {
		a := ProjectSlug(filepath.Join(base, "clients", "app"))
		b := ProjectSlug(filepath.Join(base, "servers", "app"))
		if !strings.HasPrefix(a, "app-") || !strings.HasPrefix(b, "app-") {
			t.Fatalf("expected slugs with 'app-' prefix, got %q and %q", a, b)
		}
		if a == b {
			t.Fatalf("same leaf name in different dirs collapsed to one slug %q", a)
		}
	})

	t.Run("trailing separators trimmed", func(t *testing.T) {
		if got := ProjectSlug(proj + "/"); got != ProjectSlug(proj) {
			t.Fatalf("trailing slash not trimmed: %q vs %q", got, ProjectSlug(proj))
		}
		if got := ProjectSlug(proj + "\\"); got != ProjectSlug(proj) {
			t.Fatalf("trailing backslash not trimmed: %q vs %q", got, ProjectSlug(proj))
		}
	})

	t.Run("empty and dot fall back to default", func(t *testing.T) {
		for _, in := range []string{"", ".", "./", "/", "\\", "   "} {
			if got := ProjectSlug(in); got != "default" {
				t.Errorf("ProjectSlug(%q) = %q; want \"default\"", in, got)
			}
		}
	})

	t.Run("leaf extracted from both separators", func(t *testing.T) {
		tests := []struct {
			in   string
			leaf string
		}{
			{filepath.Join(base, "a", "b", "widget"), "widget"},
			{`C:\Users\dev\widget`, "widget"},
			{proj, "proj"},
		}
		for _, tc := range tests {
			if got := ProjectSlug(tc.in); !strings.HasPrefix(got, tc.leaf+"-") {
				t.Errorf("ProjectSlug(%q) = %q; want prefix %q-", tc.in, got, tc.leaf)
			}
		}
	})

	t.Run("hash suffix is 8 hex chars of the abs path", func(t *testing.T) {
		got := ProjectSlug(proj)
		idx := strings.LastIndex(got, "-")
		if idx < 0 {
			t.Fatalf("slug %q has no '-' separator", got)
		}
		suffix := got[idx+1:]
		if len(suffix) != 8 {
			t.Fatalf("hash suffix %q is %d chars; want 8", suffix, len(suffix))
		}
		if _, err := hex.DecodeString(suffix); err != nil {
			t.Fatalf("hash suffix %q is not hex: %v", suffix, err)
		}
		// proj comes from t.TempDir() so it is already absolute.
		sum := sha1.Sum([]byte(proj))
		if want := hex.EncodeToString(sum[:])[:8]; suffix != want {
			t.Fatalf("hash suffix %q; want sha1(abs path)[:8] = %q", suffix, want)
		}
	})

	t.Run("slug is filesystem safe", func(t *testing.T) {
		for _, in := range []string{proj, `C:\Users\dev\widget`, "/a/b/c", "plain"} {
			if got := ProjectSlug(in); strings.ContainsAny(got, `/\`) {
				t.Errorf("ProjectSlug(%q) = %q contains a path separator", in, got)
			}
		}
	})
}

func TestRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	root, err := Root()
	if err != nil {
		t.Fatalf("Root() error: %v", err)
	}
	if root != filepath.Join(home, ".forge") {
		t.Fatalf("Root() = %q; want %q", root, filepath.Join(home, ".forge"))
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		t.Fatalf("Root() did not create %s: (stat err: %v)", root, err)
	}
}

func TestProjectDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	wsA := filepath.Join(home, "code", "alpha")
	wsB := filepath.Join(home, "other", "alpha")

	t.Run("creates ~/.forge/<kind>/<slug>", func(t *testing.T) {
		dir, err := ProjectDir("sessions", wsA)
		if err != nil {
			t.Fatalf("ProjectDir() error: %v", err)
		}
		if want := filepath.Join(home, ".forge", "sessions", ProjectSlug(wsA)); dir != want {
			t.Fatalf("ProjectDir() = %q; want %q", dir, want)
		}
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			t.Fatalf("ProjectDir() did not create %s: (stat err: %v)", dir, err)
		}
	})

	t.Run("repeated call is idempotent", func(t *testing.T) {
		first, err := ProjectDir("sessions", wsA)
		if err != nil {
			t.Fatalf("ProjectDir() error: %v", err)
		}
		second, err := ProjectDir("sessions", wsA)
		if err != nil {
			t.Fatalf("ProjectDir() error: %v", err)
		}
		if first != second {
			t.Fatalf("repeated ProjectDir() = %q; want %q", second, first)
		}
		if _, err := os.Stat(second); err != nil {
			t.Fatalf("expected dir to still exist: %v", err)
		}
	})

	t.Run("same leaf name gets separate dirs", func(t *testing.T) {
		dirA, err := ProjectDir("memory", wsA)
		if err != nil {
			t.Fatalf("ProjectDir() error: %v", err)
		}
		dirB, err := ProjectDir("memory", wsB)
		if err != nil {
			t.Fatalf("ProjectDir() error: %v", err)
		}
		if dirA == dirB {
			t.Fatalf("two same-leaf workspaces share dir %q", dirA)
		}
		if filepath.Dir(dirA) != filepath.Dir(dirB) {
			t.Fatalf("expected both dirs under the same kind dir: %q vs %q", dirA, dirB)
		}
		if !strings.HasPrefix(filepath.Base(dirB), "alpha-") {
			t.Fatalf("dir base %q missing 'alpha-' leaf prefix", filepath.Base(dirB))
		}
		for _, d := range []string{dirA, dirB} {
			if _, err := os.Stat(d); err != nil {
				t.Fatalf("expected %s to exist: %v", d, err)
			}
		}
	})
}
