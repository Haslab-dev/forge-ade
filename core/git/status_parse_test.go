package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParsePorcelainV2Line(t *testing.T) {
	const (
		hH = "0000000"
		hI = "1111111"
		mH = "100644"
		mI = "100644"
		mW = "100644"
	)

	cases := []struct {
		name string
		line string
		want *porcelainEntry
	}{
		{
			name: "ordinary entry path without spaces",
			line: "1 .M N... " + mH + " " + mI + " " + mW + " " + hH + " " + hI + " main.txt",
			want: &porcelainEntry{Kind: "1", XY: ".M", Path: "main.txt"},
		},
		{
			name: "ordinary entry path with spaces",
			line: "1 .M N... " + mH + " " + mI + " " + mW + " " + hH + " " + hI + " my docs/annual report 2026 final.md",
			want: &porcelainEntry{Kind: "1", XY: ".M", Path: "my docs/annual report 2026 final.md"},
		},
		{
			name: "ordinary entry staged and unstaged with unicode path",
			line: "1 MM N... " + mH + " " + mI + " " + mW + " " + hH + " " + hI + " données/日本語 ファイル.txt",
			want: &porcelainEntry{Kind: "1", XY: "MM", Path: "données/日本語 ファイル.txt"},
		},
		{
			name: "clean entry parsed (XY filter happens later)",
			line: "1 .. N... " + mH + " " + mI + " " + mW + " " + hH + " " + hI + " clean.txt",
			want: &porcelainEntry{Kind: "1", XY: "..", Path: "clean.txt"},
		},
		{
			name: "rename entry with tab separated origPath",
			line: "2 R. N... " + mH + " " + mI + " " + mW + " " + hH + " " + hI + " R100 new name.txt\told name.txt",
			want: &porcelainEntry{Kind: "2", XY: "R.", Path: "new name.txt", OrigPath: "old name.txt"},
		},
		{
			name: "rename entry unicode with spaces both sides",
			line: "2 R. N... " + mH + " " + mI + " " + mW + " " + hH + " " + hI + " R100 新し い名前.txt\t旧 名前.txt",
			want: &porcelainEntry{Kind: "2", XY: "R.", Path: "新し い名前.txt", OrigPath: "旧 名前.txt"},
		},
		{
			name: "rename without origPath (copy with no tab)",
			line: "2 .D N... " + mH + " " + mI + " " + mW + " " + hH + " " + hI + " C100 gone.txt",
			want: &porcelainEntry{Kind: "2", XY: ".D", Path: "gone.txt"},
		},
		{
			name: "unmerged entry with spaces in path",
			line: "u UU N... AG AG AG AG " + hH + " " + hI + " " + hI + " their file v2.txt",
			want: &porcelainEntry{Kind: "u", XY: "UU", Path: "their file v2.txt"},
		},
		{
			name: "unmerged entry unicode path",
			line: "u AA N... AG AG AG AG " + hH + " " + hI + " " + hI + " résumé final.pdf",
			want: &porcelainEntry{Kind: "u", XY: "AA", Path: "résumé final.pdf"},
		},
		{
			name: "untracked with spaces",
			line: "? scratch pad/new note 1.txt",
			want: &porcelainEntry{Kind: "?", Path: "scratch pad/new note 1.txt"},
		},
		{
			name: "untracked unicode",
			line: "? icons/Logo — neu.png",
			want: &porcelainEntry{Kind: "?", Path: "icons/Logo — neu.png"},
		},
		{
			name: "untracked quoted control-char path unquoted",
			line: "? \"weird\ttab name.txt\"",
			want: &porcelainEntry{Kind: "?", Path: "weird\ttab name.txt"},
		},
		{
			name: "ignored entry skipped",
			line: "! ignored dir/cache.bin",
			want: nil,
		},
		{
			name: "comment header skipped",
			line: "# branch.head main",
			want: nil,
		},
		{
			name: "truncated ordinary line skipped",
			line: "1 .M N... " + mH + " " + hH,
			want: nil,
		},
		{
			name: "truncated unmerged line skipped",
			line: "u UU N... " + mH,
			want: nil,
		},
		{
			name: "malformed question mark skipped",
			line: "?nospace",
			want: nil,
		},
		{
			name: "empty line skipped",
			line: "",
			want: nil,
		},
		{
			name: "unknown marker skipped",
			line: "x .M N... " + mH + " " + hH + " " + hI + " " + mW + " file.txt",
			want: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parsePorcelainV2Line(tc.line)
			if tc.want == nil {
				if ok {
					t.Fatalf("expected ok=false, got entry %+v", got)
				}
				return
			}
			if !ok {
				t.Fatalf("expected ok=true, got ok=false")
			}
			if got.Kind != tc.want.Kind || got.XY != tc.want.XY || got.Path != tc.want.Path || got.OrigPath != tc.want.OrigPath {
				t.Fatalf("parsed %+v, want %+v", got, *tc.want)
			}
		})
	}
}

// TestGetStatusFilenamesWithSpaces runs a real `git status --porcelain=v2`
// against a temp repo containing spaces and unicode filenames and asserts the
// parsed paths are complete (never truncated at the first space).
func TestGetStatusFilenamesWithSpaces(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
			"GIT_TERMINAL_PROMPT=0")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
		}
	}

	run("init", "-q")
	run("config", "user.email", "t@t")
	run("config", "user.name", "t")

	write := func(name, content string) {
		t.Helper()
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Committed baseline files.
	write("keep me.txt", "base\n")
	run("add", "--", "keep me.txt")
	run("commit", "-q", "-m", "init")

	// Modified (staged + unstaged), renamed (with spaces on both sides),
	// untracked set with spaces and unicode. The rename happens without a
	// content change so git detects it (R100).
	write("docs/space file.md", "unstaged\n")
	run("add", "--", "docs/space file.md")
	write("docs/space file.md", "unstaged after staging\n")
	run("mv", "keep me.txt", "renamed file with spaces.txt")
	run("add", "--", "renamed file with spaces.txt")
	write("папка/новый файл.txt", "untracked\n")

	e := NewEngine()
	res, err := e.GetStatus(context.Background(), dir)
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}

	pathsOf := func(list []FileStatus) map[string]string {
		m := map[string]string{}
		for _, f := range list {
			m[f.Path] = f.Status
		}
		return m
	}

	staged := pathsOf(res.Staged)
	unstaged := pathsOf(res.Unstaged)
	untracked := pathsOf(res.Untracked)

	// Rename: staged "R" with the NEW path (orig path carried separately).
	if st, ok := staged["renamed file with spaces.txt"]; !ok || st != "R" {
		t.Errorf("staged: want renamed file with spaces.txt (R), got %v", staged)
	}
	if st, ok := staged["docs/space file.md"]; !ok || st != "A" {
		t.Errorf("staged: want docs/space file.md (A), got %v", staged)
	}
	if st, ok := unstaged["docs/space file.md"]; !ok || st != "M" {
		t.Errorf("unstaged: want docs/space file.md (M), got %v", unstaged)
	}
	if st, ok := untracked["папка/новый файл.txt"]; !ok || st != "?" {
		t.Errorf("untracked: want unicode path, got %v", untracked)
	}
	// Truncation would produce a path like "docs/space" — none may exist.
	for _, m := range []map[string]string{staged, unstaged, untracked} {
		for p := range m {
			if !strings.Contains(p, ".") && strings.Contains(p, " ") {
				t.Errorf("suspicious truncated path %q", p)
			}
		}
	}
}
