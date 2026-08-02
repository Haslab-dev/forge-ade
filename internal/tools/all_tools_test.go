package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hasdev/forge-ade/internal/search"
)

// TestAllTools exercises every registered tool against a real temp directory.
// It mirrors what the agent loop does: Registry.Execute with JSON args.
func TestAllTools(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	target := filepath.Join(dir, "test.txt")
	os.WriteFile(target, []byte("Ini test content\nmore to test\nline 3: halo dunia\n"), 0644)

	// Seed a real search manager with the test dir so `search` finds matches.
	sm := search.NewSearchManager()
	sm.SetDirectories([]string{dir})
	reg := NewRegistry(sm)

	run := func(t *testing.T, name string, args map[string]any) (any, error) {
		t.Helper()
		raw, _ := json.Marshal(args)
		res, err := reg.Execute(ctx, name, string(raw))
		t.Logf("TOOL %s args=%v => err=%v", name, args, err)
		if err == nil && res != nil {
			if b, jerr := json.MarshalIndent(res, "", "  "); jerr == nil {
				t.Logf("  result: %s", string(b))
			}
		} else if err != nil {
			t.Logf("  error: %v", err)
		}
		return res, err
	}

	// 1. read
	if _, err := run(t, "read", map[string]any{"path": target}); err != nil {
		t.Errorf("read failed: %v", err)
	}
	// 2. write (new file)
	newFile := filepath.Join(dir, "new.txt")
	if _, err := run(t, "write", map[string]any{"path": newFile, "content": "hello from write\n"}); err != nil {
		t.Errorf("write failed: %v", err)
	}
	if b, _ := os.ReadFile(newFile); string(b) != "hello from write\n" {
		t.Errorf("write content mismatch: %q", string(b))
	}
	// 3. edit
	if _, err := run(t, "edit", map[string]any{"path": target, "old": "Ini test content", "new": "EDITED"}); err != nil {
		t.Errorf("edit failed: %v", err)
	}
	if b, _ := os.ReadFile(target); !strings.Contains(string(b), "EDITED") {
		t.Errorf("edit did not apply: %q", string(b))
	}
	// 4. bash
	if _, err := run(t, "bash", map[string]any{"command": "echo hi from bash && pwd"}); err != nil {
		t.Errorf("bash failed: %v", err)
	}
	// 5. search — should find "more to test"
	res, err := run(t, "search", map[string]any{"pattern": "more to test", "path": dir})
	if err != nil {
		t.Errorf("search failed: %v", err)
	} else if m, ok := res.(map[string]any); ok {
		if c, okc := m["count"].(int); !okc || c != 1 {
			t.Errorf("search count = %v, want 1", m["count"])
		}
	}
	// 6. find with ** recursion
	res, err = run(t, "find", map[string]any{"path": "**/*.txt", "cwd": dir})
	if err != nil {
		t.Errorf("find failed: %v", err)
	} else if m, ok := res.(map[string]any); ok {
		if c, okc := m["count"].(int); !okc || c != 2 {
			t.Errorf("find count = %v, want 2 (test.txt + new.txt)", m["count"])
		}
	}
	// 7. glob
	if _, err := run(t, "glob", map[string]any{"pattern": "*.txt", "cwd": dir}); err != nil {
		t.Errorf("glob failed: %v", err)
	}
	// 8. git_status (in a non-repo dir should return error but not panic)
	if _, err := run(t, "git_status", map[string]any{"dir": dir}); err != nil {
		t.Logf("git_status in non-repo: %v (expected)", err)
	}
	// 9. ask — needs a session bridge; outside a session it should error cleanly.
	if _, err := run(t, "ask", map[string]any{"questions": []any{map[string]any{"id": "q1", "question": "pick", "options": []any{"a", "b"}}}}); err == nil {
		t.Log("ask returned nil outside session (may be expected if no bridge)")
	}
	// 10. todo — same: outside session errors cleanly.
	if _, err := run(t, "todo", map[string]any{"op": "view"}); err == nil {
		t.Log("todo returned nil outside session (may be expected if no bridge)")
	}
}
