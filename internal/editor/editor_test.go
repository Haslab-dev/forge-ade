package editor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewToolingRoot(t *testing.T) {
	t.Run("empty root falls back to cwd", func(t *testing.T) {
		got := NewTooling("")
		want, err := os.Getwd()
		if err != nil {
			t.Fatalf("os.Getwd() error: %v", err)
		}
		if got.root != want {
			t.Fatalf("NewTooling(\"\").root = %q; want cwd %q", got.root, want)
		}
	})

	t.Run("explicit root is preserved", func(t *testing.T) {
		got := NewTooling("/custom")
		if got.root != "/custom" {
			t.Fatalf("NewTooling(\"/custom\").root = %q; want \"/custom\"", got.root)
		}
	})
}

func TestCheckSyntaxNonJSExtensions(t *testing.T) {
	tool := NewTooling(t.TempDir())
	ctx := context.Background()

	tests := []struct {
		path    string
		content string
	}{
		{"main.go", "package main\nfunc main() { ((( }\n"},
		{"script.py", "def f(:\n"},
		{"style.css", "body { color }\n"},
		{"README", "not code at all\n"},
	}
	for _, tc := range tests {
		diags, err := tool.CheckSyntax(ctx, tc.path, tc.content)
		if err != nil {
			t.Errorf("CheckSyntax(%q) error: %v", tc.path, err)
			continue
		}
		if len(diags) != 0 {
			t.Errorf("CheckSyntax(%q) = %d diags; want 0 (non-JS must short-circuit)", tc.path, len(diags))
		}
	}
}

// TestCheckSyntaxNoOpWithoutEsbuild exercises the "esbuild not installed"
// contract. findEsbuild also probes absolute locations (~/.bun/bin, homebrew,
// ...), so HOME is pointed at a temp dir to hide ~/.bun/bin/esbuild; if a
// probe location still resolves, the test skips rather than flake.
func TestCheckSyntaxNoOpWithoutEsbuild(t *testing.T) {
	empty := t.TempDir()
	t.Setenv("PATH", empty)
	t.Setenv("HOME", empty)

	if p := findEsbuild(); p != "" {
		t.Skipf("esbuild found at %s; cannot test the no-op path hermetically", p)
	}

	diags, err := NewTooling(t.TempDir()).CheckSyntax(context.Background(), "index.ts", "const = ;")
	if err != nil {
		t.Fatalf("CheckSyntax error: %v", err)
	}
	if len(diags) != 0 {
		t.Fatalf("CheckSyntax without esbuild returned %d diags; want 0", len(diags))
	}
}

// installFakeEsbuild puts an executable shell script on PATH that prints a
// realistic esbuild syntax-error report, regardless of the file it is given.
func installFakeEsbuild(t *testing.T) {
	t.Helper()

	binDir := filepath.Join(t.TempDir(), "fakebin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("create fakebin: %v", err)
	}
	script := "#!/bin/sh\n" +
		"cat <<'EOF'\n" +
		"✘ [ERROR] Expected identifier but found \"x\"\n" +
		"    /tmp/proj/index.ts:2:5:\n" +
		"      2 │ const = x;\n" +
		"        ╵     ^\n" +
		"EOF\n"
	bin := filepath.Join(binDir, "esbuild")
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake esbuild: %v", err)
	}
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))
}

func TestCheckSyntaxParsesEsbuildError(t *testing.T) {
	installFakeEsbuild(t)
	tool := NewTooling(t.TempDir())

	diags, err := tool.CheckSyntax(context.Background(), "index.ts", "const = x;\n")
	if err != nil {
		t.Fatalf("CheckSyntax error: %v", err)
	}
	if len(diags) != 1 {
		t.Fatalf("got %d diags (%+v); want 1", len(diags), diags)
	}
	d := diags[0]
	if !strings.Contains(d.Message, "Expected identifier") {
		t.Errorf("Message = %q; want it to contain \"Expected identifier\"", d.Message)
	}
	if d.Line != 2 {
		t.Errorf("Line = %d; want 2", d.Line)
	}

	// FORGE-132: the comment above the parsing loop claims the column is
	// decremented to convert esbuild's report into a 0-based position, but
	// the code assigns the parsed value unchanged. The fake esbuild reports
	// column 5 and that is exactly what surfaces here — the comment is stale
	// or the "-1" was never implemented. Either the comment or the code must
	// change; this assertion pins the current, deterministic parse behavior.
	if d.Column != 5 {
		t.Errorf("Column = %d; want 5 (as reported by esbuild, see FORGE-132)", d.Column)
	}
}

func TestFormatCodeUnsupportedExtensionShortCircuits(t *testing.T) {
	// PATH points at an empty dir: if any binary were invoked it could not
	// resolve, yet the unsupported extension must return before that.
	t.Setenv("PATH", t.TempDir())

	tool := NewTooling(t.TempDir())
	got, err := tool.FormatCode(context.Background(), "main.go", "package main\n")
	if err != nil {
		t.Fatalf("FormatCode(.go) error: %v", err)
	}
	if got != "package main\n" {
		t.Fatalf("FormatCode(.go) = %q; want input unchanged", got)
	}
}

// TestFormatCodeSilentFallbackWithoutPrettier pins the documented behavior of
// returning the original content with a nil error when prettier is missing.
//
// FORGE-133: failures are swallowed silently — callers cannot distinguish
// "formatted" from "formatter unavailable/failed".
func TestFormatCodeSilentFallbackWithoutPrettier(t *testing.T) {
	root := t.TempDir() // no frontend/node_modules, no node_modules
	t.Setenv("PATH", t.TempDir())

	tool := NewTooling(root)
	input := "const  x   =  1\n"
	got, err := tool.FormatCode(context.Background(), "app.ts", input)
	if err != nil {
		t.Fatalf("FormatCode error: %v", err)
	}
	if got != input {
		t.Fatalf("FormatCode without prettier = %q; want original %q", got, input)
	}
}

// installFakePrettier places an executable script at
// <root>/node_modules/.bin/prettier that appends its arguments to
// $FAKE_PRETTIER_LOG and rewrites its input file uppercased.
func installFakePrettier(t *testing.T, root string) string {
	t.Helper()

	logPath := filepath.Join(t.TempDir(), "prettier-args.log")
	t.Setenv("FAKE_PRETTIER_LOG", logPath)

	binDir := filepath.Join(root, "node_modules", ".bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("create node_modules/.bin: %v", err)
	}
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$*\" >> \"$FAKE_PRETTIER_LOG\"\n" +
		// Read fully before truncating: writing back in the same pipeline
		// would clobber the input before cat reads it.
		"printf '%s\\n' \"$(cat \"$1\" | tr 'a-z' 'A-Z')\" > \"$1\"\n"
	bin := filepath.Join(binDir, "prettier")
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake prettier: %v", err)
	}
	return logPath
}

func TestFormatCodeRunsWorkspacePrettier(t *testing.T) {
	root := t.TempDir()
	logPath := installFakePrettier(t, root)

	tool := NewTooling(root)
	got, err := tool.FormatCode(context.Background(), "app.ts", "const x = 1;\n")
	if err != nil {
		t.Fatalf("FormatCode error: %v", err)
	}
	if want := "CONST X = 1;\n"; got != want {
		t.Fatalf("FormatCode = %q; want %q", got, want)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read fake prettier log: %v", err)
	}
	logged := string(logData)
	if !strings.Contains(logged, "--parser typescript") {
		t.Errorf("prettier args %q missing --parser typescript", logged)
	}
	if !strings.Contains(logged, "--write") {
		t.Errorf("prettier args %q missing --write", logged)
	}
	if strings.Contains(logged, "--config") {
		t.Errorf("prettier args %q unexpectedly include --config (no config file exists)", logged)
	}
}

func TestFormatCodePassesDetectedConfig(t *testing.T) {
	t.Run("single .prettierrc", func(t *testing.T) {
		root := t.TempDir()
		cfg := filepath.Join(root, ".prettierrc")
		if err := os.WriteFile(cfg, []byte("{}\n"), 0o644); err != nil {
			t.Fatalf("write .prettierrc: %v", err)
		}
		logPath := installFakePrettier(t, root)

		if _, err := NewTooling(root).FormatCode(context.Background(), "app.ts", "const x = 1;\n"); err != nil {
			t.Fatalf("FormatCode error: %v", err)
		}
		logData, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("read fake prettier log: %v", err)
		}
		if !strings.Contains(string(logData), "--config "+cfg) {
			t.Errorf("prettier args %q missing --config %s", string(logData), cfg)
		}
	})

	t.Run(".prettierrc wins over .prettierrc.json", func(t *testing.T) {
		root := t.TempDir()
		first := filepath.Join(root, ".prettierrc")
		if err := os.WriteFile(first, []byte("{}\n"), 0o644); err != nil {
			t.Fatalf("write .prettierrc: %v", err)
		}
		if err := os.WriteFile(filepath.Join(root, ".prettierrc.json"), []byte("{}\n"), 0o644); err != nil {
			t.Fatalf("write .prettierrc.json: %v", err)
		}
		logPath := installFakePrettier(t, root)

		if _, err := NewTooling(root).FormatCode(context.Background(), "app.ts", "const x = 1;\n"); err != nil {
			t.Fatalf("FormatCode error: %v", err)
		}
		logData, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("read fake prettier log: %v", err)
		}
		logged := string(logData)
		if !strings.Contains(logged, "--config "+first) {
			t.Errorf("prettier args %q missing --config %s (.prettierrc must win)", logged, first)
		}
		if strings.Contains(logged, ".prettierrc.json") {
			t.Errorf("prettier args %q reference .prettierrc.json; only the first match should be used", logged)
		}
	})
}

func TestFindPrettierConfigPriority(t *testing.T) {
	t.Run("only prettier.config.js", func(t *testing.T) {
		root := t.TempDir()
		want := filepath.Join(root, "prettier.config.js")
		if err := os.WriteFile(want, []byte("module.exports = {}\n"), 0o644); err != nil {
			t.Fatalf("write config: %v", err)
		}
		if got := NewTooling(root).findPrettierConfig(); got != want {
			t.Fatalf("findPrettierConfig() = %q; want %q", got, want)
		}
	})

	t.Run(".prettierrc beats .prettierrc.json", func(t *testing.T) {
		root := t.TempDir()
		want := filepath.Join(root, ".prettierrc")
		if err := os.WriteFile(want, []byte("{}\n"), 0o644); err != nil {
			t.Fatalf("write .prettierrc: %v", err)
		}
		if err := os.WriteFile(filepath.Join(root, ".prettierrc.json"), []byte("{}\n"), 0o644); err != nil {
			t.Fatalf("write .prettierrc.json: %v", err)
		}
		if got := NewTooling(root).findPrettierConfig(); got != want {
			t.Fatalf("findPrettierConfig() = %q; want %q", got, want)
		}
	})

	t.Run("no config", func(t *testing.T) {
		if got := NewTooling(t.TempDir()).findPrettierConfig(); got != "" {
			t.Fatalf("findPrettierConfig() = %q; want \"\"", got)
		}
	})
}

func TestFindPrettierPriority(t *testing.T) {
	t.Run("workspace node_modules beats PATH", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		root := t.TempDir()
		want := filepath.Join(root, "node_modules", ".bin", "prettier")
		if err := os.MkdirAll(filepath.Dir(want), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(want, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatalf("write prettier: %v", err)
		}
		if got := NewTooling(root).findPrettier(); got != want {
			t.Fatalf("findPrettier() = %q; want %q", got, want)
		}
	})

	t.Run("frontend install beats root install", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		root := t.TempDir()
		rootBin := filepath.Join(root, "node_modules", ".bin", "prettier")
		frontendBin := filepath.Join(root, "frontend", "node_modules", ".bin", "prettier")
		for _, p := range []string{rootBin, frontendBin} {
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
				t.Fatalf("write prettier: %v", err)
			}
		}
		if got := NewTooling(root).findPrettier(); got != frontendBin {
			t.Fatalf("findPrettier() = %q; want %q", got, frontendBin)
		}
	})
}
