package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runEdit executes the edit tool the way the agent loop does (JSON args).
func runEdit(t *testing.T, ctx context.Context, reg *Registry, args map[string]any) (any, error) {
	t.Helper()
	raw, _ := json.Marshal(args)
	return reg.Execute(ctx, "edit", string(raw))
}

// TestEditUniqueMatchReplaces: a unique match is replaced; per the harness
// rule a repeated old_string without replace_all is ambiguous (never a silent
// replace-first).
func TestEditUniqueMatchReplaces(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	os.WriteFile(path, []byte("alpha\nbeta\n"), 0644)
	reg := NewRegistry(nil)

	if _, err := runEdit(t, ctx, reg, map[string]any{"path": path, "old_string": "alpha", "new_string": "GAMMA"}); err != nil {
		t.Fatalf("edit failed: %v", err)
	}
	b, _ := os.ReadFile(path)
	got := string(b)
	want := "GAMMA\nbeta\n"
	if got != want {
		t.Fatalf("unique replace: got %q want %q", got, want)
	}
}

// TestEditAmbiguousErrors: multiple matches without replace_all must fail.
func TestEditAmbiguousErrors(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	os.WriteFile(path, []byte("dup\nmid\ndup\n"), 0644)
	reg := NewRegistry(nil)

	_, err := runEdit(t, ctx, reg, map[string]any{"path": path, "old_string": "dup", "new_string": "x"})
	if err == nil {
		t.Fatal("expected ambiguity error")
	}
	if !strings.Contains(err.Error(), "Found 2 matches") || !strings.Contains(err.Error(), "replace_all") {
		t.Fatalf("ambiguous error should suggest replace_all: %v", err)
	}
	// File must be untouched.
	b, _ := os.ReadFile(path)
	if string(b) != "dup\nmid\ndup\n" {
		t.Fatalf("ambiguous edit mutated file: %q", string(b))
	}
}

// TestEditReplaceAll: with replace_all every occurrence is replaced.
func TestEditReplaceAll(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	os.WriteFile(path, []byte("dup\nmid\ndup\n"), 0644)
	reg := NewRegistry(nil)

	if _, err := runEdit(t, ctx, reg, map[string]any{"path": path, "old_string": "dup", "new_string": "x", "replace_all": true}); err != nil {
		t.Fatalf("edit failed: %v", err)
	}
	b, _ := os.ReadFile(path)
	if string(b) != "x\nmid\nx\n" {
		t.Fatalf("replace_all result: %q", string(b))
	}
}

// TestEditNotFoundAndSameStrings: helpful errors for missing anchor and no-op.
func TestEditNotFoundAndSameStrings(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	os.WriteFile(path, []byte("hello\n"), 0644)
	reg := NewRegistry(nil)

	if _, err := runEdit(t, ctx, reg, map[string]any{"path": path, "old_string": "nope", "new_string": "x"}); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("want not-found error, got %v", err)
	}
	if _, err := runEdit(t, ctx, reg, map[string]any{"path": path, "old_string": "hello", "new_string": "hello"}); err == nil || !strings.Contains(err.Error(), "exactly the same") {
		t.Fatalf("want no-change error, got %v", err)
	}
}

// TestEditLineTrimmedFallback: whitespace drift per line still matches.
func TestEditLineTrimmedFallback(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	os.WriteFile(path, []byte("function a() {\n    return 1;\n}\n"), 0644)
	reg := NewRegistry(nil)

	oldStr := "function a() {\n\treturn 1;\n}"
	if _, err := runEdit(t, ctx, reg, map[string]any{"path": path, "old_string": oldStr, "new_string": "function a() {\n\treturn 2;\n}"}); err != nil {
		t.Fatalf("line-trimmed edit failed: %v", err)
	}
	b, _ := os.ReadFile(path)
	if !strings.Contains(string(b), "return 2;") {
		t.Fatalf("line-trimmed edit not applied: %q", string(b))
	}
}

// TestEditLineNumberPrefixFallback: old_string copied from numbered Read output.
func TestEditLineNumberPrefixFallback(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	os.WriteFile(path, []byte("first\nsecond\nthird\n"), 0644)
	reg := NewRegistry(nil)

	oldStr := "1\tfirst\n2\tsecond"
	if _, err := runEdit(t, ctx, reg, map[string]any{"path": path, "old_string": oldStr, "new_string": "first\nREPLACED"}); err != nil {
		t.Fatalf("line-number edit failed: %v", err)
	}
	b, _ := os.ReadFile(path)
	if !strings.Contains(string(b), "REPLACED\nthird") {
		t.Fatalf("line-number edit not applied: %q", string(b))
	}
}

// TestEditCreateAndDeleteSemantics: empty old_string creates; empty new_string deletes a line.
func TestEditCreateAndDeleteSemantics(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	reg := NewRegistry(nil)

	created := filepath.Join(dir, "sub", "new.txt")
	if _, err := runEdit(t, ctx, reg, map[string]any{"path": created, "old_string": "", "new_string": "brand new"}); err != nil {
		t.Fatalf("create-mode edit failed: %v", err)
	}
	if b, _ := os.ReadFile(created); string(b) != "brand new" {
		t.Fatalf("create-mode content: %q", string(b))
	}

	dele := filepath.Join(dir, "del.txt")
	os.WriteFile(dele, []byte("keep\nploog\nkeep2\n"), 0644)
	if _, err := runEdit(t, ctx, reg, map[string]any{"path": dele, "old_string": "ploog", "new_string": ""}); err != nil {
		t.Fatalf("delete edit failed: %v", err)
	}
	b, _ := os.ReadFile(dele)
	if string(b) != "keep\nkeep2\n" {
		t.Fatalf("delete edit result: %q", string(b))
	}
}

// TestEditCRLFPreserved: editing a CRLF file keeps CRLF line endings.
func TestEditCRLFPreserved(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "win.txt")
	os.WriteFile(path, []byte("one\r\ntwo\r\n"), 0644)
	reg := NewRegistry(nil)

	if _, err := runEdit(t, ctx, reg, map[string]any{"path": path, "old_string": "two", "new_string": "DOS"}); err != nil {
		t.Fatalf("crlf edit failed: %v", err)
	}
	b, _ := os.ReadFile(path)
	if !strings.Contains(string(b), "one\r\nDOS\r\n") {
		t.Fatalf("crlf not preserved: %q", string(b))
	}
}

// TestReadBeforeEditGate: with a session tracker, edit requires a prior read,
// and write requires one for existing files. New files are never gated.
func TestReadBeforeEditGate(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	reg := NewRegistry(nil)
	tracker := NewFileStateTracker()
	ctx = WithFileState(ctx, tracker)

	path := filepath.Join(dir, "gate.txt")
	os.WriteFile(path, []byte("secret\n"), 0644)

	_, err := runEdit(t, ctx, reg, map[string]any{"path": path, "old_string": "secret", "new_string": "x"})
	if err == nil || !strings.Contains(err.Error(), "has not been read yet") {
		t.Fatalf("expected read-before-edit gate, got %v", err)
	}

	if _, err := reg.Execute(ctx, "write", `{"path":"`+path+`","content":"nope"}`); err == nil || !strings.Contains(err.Error(), "has not been read yet") {
		t.Fatalf("expected read-before-write gate, got %v", err)
	}

	// After a read, edit passes.
	if _, err := reg.Execute(ctx, "read", `{"path":"`+path+`"}`); err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if _, err := runEdit(t, ctx, reg, map[string]any{"path": path, "old_string": "secret", "new_string": "edited"}); err != nil {
		t.Fatalf("edit after read failed: %v", err)
	}

	// External modification invalidates the snapshot.
	os.WriteFile(path, []byte("tampered\n"), 0644)
	if _, err := runEdit(t, ctx, reg, map[string]any{"path": path, "old_string": "tampered", "new_string": "x"}); err == nil || !strings.Contains(err.Error(), "modified since read") {
		t.Fatalf("expected staleness error, got %v", err)
	}
}
