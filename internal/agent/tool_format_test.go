package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hasdev/forge-ade/internal/tools"
)

// executeForFormat runs a real tool through the registry and returns the
// model-facing text exactly the way executeToolCallDetailed does: handler
// result (native Go types) → normalizeToolResult → formatToolModelContent.
func executeForFormat(t *testing.T, reg *tools.Registry, name string, args map[string]any) string {
	t.Helper()
	raw, _ := json.Marshal(args)
	res, err := reg.Execute(context.Background(), name, string(raw))
	if err != nil {
		t.Fatalf("%s failed: %v", name, err)
	}
	return formatToolModelContent(name, normalizeToolResult(res))
}

// TestFormattedToolResultsAreNotEmpty pins the fix for the silent-empty bug:
// tool handlers return native Go types ([]map[string]any, int) while the
// formatter speaks JSON types — without normalization every directory read /
// search / find was rendered as "(0 entries)" / "Found 0 matches" and the
// model reported non-empty folders as empty.
func TestFormattedToolResultsAreNotEmpty(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("alpha"), 0644)
	os.MkdirAll(filepath.Join(dir, "sub"), 0755)
	os.WriteFile(filepath.Join(dir, "sub", "b.txt"), []byte("beta"), 0644)

	reg := tools.NewRegistry(nil)

	// 1. read on a directory must list its entries.
	out := executeForFormat(t, reg, "read", map[string]any{"path": dir})
	if !strings.Contains(out, "2 entries") {
		t.Fatalf("dir read should report 2 entries, got: %q", out)
	}
	if !strings.Contains(out, "a.txt") || !strings.Contains(out, "sub") {
		t.Fatalf("dir read should list names, got: %q", out)
	}

	// 2. find must report the matching files.
	out = executeForFormat(t, reg, "find", map[string]any{"path": "**/*.txt", "cwd": dir})
	if !strings.Contains(out, "Found 2 matches") || !strings.Contains(out, "a.txt") {
		t.Fatalf("find should report matches, got: %q", out)
	}

	// 3. glob must report matches too.
	out = executeForFormat(t, reg, "glob", map[string]any{"pattern": "*.txt", "cwd": dir})
	if !strings.Contains(out, "a.txt") {
		t.Fatalf("glob should report matches, got: %q", out)
	}

	// 4. bash exit codes (native int) must survive formatting.
	text := formatToolModelContent("bash", normalizeToolResult(map[string]any{
		"stdout": "", "stderr": "boom", "exit_code": 1,
	}))
	if !strings.Contains(text, "Exit code: 1") || !strings.Contains(text, "boom") {
		t.Fatalf("bash formatting lost exit code/stderr: %q", text)
	}

	// 5. edit replacement counts (native int) must survive formatting.
	text = formatToolModelContent("edit", normalizeToolResult(map[string]any{
		"path": "/x/y.go", "status": "edited", "replacements": 3, "replace_all": true,
	}))
	if !strings.Contains(text, "All occurrences were successfully replaced") {
		t.Fatalf("edit replace_all note lost: %q", text)
	}
}
