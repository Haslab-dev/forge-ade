package index

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestStoreBuildAndQuery(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"src/a.ts": `export function parseFile(input: string): number { return 1 }
export class FileBrowser {
  open() {}
}
const PORT = 3000;
import fs from "node:fs";
`,
		"src/b.ts": `export interface User { id: number }
export const readFile = (p: string) => p;
`,
		"src/c.js": `function legacyFn() {}
export default legacyFn;
`,
	})
	s := New(dir)
	if err := s.Build(); err != nil {
		t.Fatal(err)
	}

	syms := s.Symbols()
	if len(syms) != 8 {
		t.Fatalf("got %d symbols, want 7: %v", len(syms), symbolNames(syms))
	}

	// Find (exact)
	if got := s.Find("FileBrowser"); len(got) != 1 || got[0].Name != "FileBrowser" {
		t.Fatalf("Find(FileBrowser) = %+v", got)
	}
	// Definition
	if got := s.Definition("parseFile"); len(got) != 1 || got[0].Line != 1 {
		t.Fatalf("Definition(parseFile) = %+v", got)
	}
	// Completion (prefix)
	comp := s.Completion("parse", "")
	if len(comp) != 1 || comp[0].Name != "parseFile" {
		t.Fatalf("Completion(parse) = %+v", symbolNames(comp))
	}
	// Search: prefix result
	search := s.Search("readFile")
	if len(search) != 1 || search[0].Name != "readFile" {
		t.Fatalf("Search(readFile) = %+v", symbolNames(search))
	}
	// camel case search
	camel := s.Search("FB")
	if len(camel) == 0 || camel[0].Name != "FileBrowser" {
		t.Fatalf("Search(FB) = %+v", symbolNames(camel))
	}

	// Outline
	aPath := filepath.Join(dir, "src/a.ts")
	outline := s.Outline(aPath)
	wantOutline := []string{"parseFile", "FileBrowser", "open", "PORT"}
	if !reflect.DeepEqual(symbolNames(outline), wantOutline) {
		t.Fatalf("Outline = %v, want %v", symbolNames(outline), wantOutline)
	}

	// Imports
	imps := s.Imports(aPath)
	if len(imps) != 1 || imps[0].Path != `"node:fs"` {
		t.Fatalf("Imports = %+v", imps)
	}
	// Exports
	exps := s.Exports(filepath.Join(dir, "src/c.js"))
	if len(exps) != 1 || exps[0].Name != "legacyFn" {
		t.Fatalf("Exports = %+v", exps)
	}
}

func symbolNames(syms []Symbol) []string {
	var out []string
	for _, s := range syms {
		out = append(out, s.Name)
	}
	return out
}

func TestStoreUpdateAndRemove(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.ts")
	writeTree(t, dir, map[string]string{"a.ts": "export function foo() {}\n"})
	s := New(dir)
	if err := s.Build(); err != nil {
		t.Fatal(err)
	}
	if got := s.Find("foo"); len(got) != 1 {
		t.Fatalf("before update: %+v", got)
	}

	// Update: add bar, keep foo
	if err := os.WriteFile(a, []byte("export function foo() {}\nexport function bar() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.Update(a); err != nil {
		t.Fatal(err)
	}
	if len(s.Find("foo")) != 1 || len(s.Find("bar")) != 1 {
		t.Fatalf("after update: foo=%d bar=%d", len(s.Find("foo")), len(s.Find("bar")))
	}

	// Update with identical content — must be skipped (hash check)
	before := len(s.Symbols())
	if err := s.Update(a); err != nil {
		t.Fatal(err)
	}
	if len(s.Symbols()) != before {
		t.Fatal("unchanged update should not reindex")
	}

	// Remove
	if err := s.Remove(a); err != nil {
		t.Fatal(err)
	}
	if got := s.Find("foo"); len(got) != 0 {
		t.Fatalf("after remove: %+v", got)
	}
}

func TestStoreSnapshot(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"a.ts": "export function hello() {}\nexport class Widget {}\n",
	})
	s := New(dir)
	if err := s.Build(); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".workspace", "index.bin")); err != nil {
		t.Fatalf("index.bin missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".workspace", "metadata.json")); err != nil {
		t.Fatalf("metadata.json missing: %v", err)
	}

	// Load into a fresh store
	s2 := New(dir)
	if err := s2.Load(); err != nil {
		t.Fatal(err)
	}
	if got := s2.Find("hello"); len(got) != 1 {
		t.Fatalf("loaded Find(hello) = %+v", got)
	}
	if got := s2.Find("Widget"); len(got) != 1 {
		t.Fatalf("loaded Find(Widget) = %+v", got)
	}
}

func TestStoreSkipsUnsupportedFiles(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"a.ts": "export function a() {}\n",
		"b.rb": "def b\nend\n",
	})
	s := New(dir)
	if err := s.Build(); err != nil {
		t.Fatal(err)
	}
	if got := s.Find("a"); len(got) != 1 {
		t.Fatalf("a not indexed: %+v", got)
	}
	if got := s.Find("b"); len(got) != 0 {
		t.Fatalf("unsupported lang should be skipped: %+v", got)
	}
}
