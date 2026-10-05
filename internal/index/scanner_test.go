package index

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestScannerIgnoresAndFilters(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"src/a.ts":                "export const a = 1;",
		"src/b.js":                "export function b() {}",
		"src/ignored.rb":          "package ignored",
		"node_modules/x/index.js": "export const x = 1;",
		".git/config":             "x",
		"dist/out.js":             "console.log(1)",
		"build/out.ts":            "export const y = 1;",
		"vendor/v.js":             "var v = 1;",
		"src/readme.md":           "# doc",
		"src/data.json":           "{}",
	})
	got, err := (&Scanner{Root: dir}).Scan()
	if err != nil {
		t.Fatal(err)
	}
	rel := []string{}
	for _, p := range got {
		r, _ := filepath.Rel(dir, p)
		rel = append(rel, r)
	}
	want := []string{"src/a.ts", "src/b.js"}
	if len(rel) != len(want) {
		t.Fatalf("got %v want %v", rel, want)
	}
	for i := range want {
		if rel[i] != want[i] {
			t.Fatalf("got %v want %v", rel, want)
		}
	}
}

func TestDetectLanguage(t *testing.T) {
	cases := map[string]Language{
		"a.js": LangJavaScript, "b.mjs": LangJavaScript, "c.cjs": LangJavaScript,
		"d.jsx": LangJSX, "e.ts": LangTypeScript, "f.tsx": LangTSX,
		"g.go": LangGo, "h.kt": LangKotlin, "i.swift": LangSwift,
	}
	for path, want := range cases {
		if got := DetectLanguage(path); got != want {
			t.Errorf("DetectLanguage(%s) = %q, want %q", path, got, want)
		}
	}
	if DetectLanguage("x.txt") != "" {
		t.Error("x.txt should be unsupported")
	}
}
