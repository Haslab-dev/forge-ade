// Package editor provides lightweight, non-LSP editor language tooling:
// syntax diagnostics via esbuild and source formatting via prettier.
//
// Tooling resolves project-local binaries and configs against an explicit
// workspace root — the caller decides what "the project" is — because an
// app launched from Finder has an arbitrary working directory.
package editor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Tooling runs the lightweight syntax check and formatter. root is the
// workspace folder used to locate project-local prettier installs and
// .prettierrc configs (pass "" to fall back to the process working dir).
type Tooling struct {
	root string
}

// NewTooling returns editor tooling rooted at the given workspace folder.
func NewTooling(root string) *Tooling {
	if root == "" {
		if cwd, err := os.Getwd(); err == nil {
			root = cwd
		}
	}
	return &Tooling{root: root}
}

// SyntaxDiagnostic is a single error/warning from the lightweight syntax check.
type SyntaxDiagnostic struct {
	Line      int    `json:"line"`
	Column    int    `json:"column"`
	EndLine   int    `json:"end_line,omitempty"`
	EndColumn int    `json:"end_column,omitempty"`
	Message   string `json:"message"`
}

// findEsbuild resolves the esbuild binary. App-launched processes (from
// Finder/launchd) have a minimal PATH, so also probe common install locations.
func findEsbuild() string {
	if p, err := exec.LookPath("esbuild"); err == nil {
		return p
	}
	home, _ := os.UserHomeDir()
	candidates := []string{
		home + "/.bun/bin/esbuild",
		"/opt/homebrew/bin/esbuild",
		"/usr/local/bin/esbuild",
		"/usr/local/share/npm/bin/esbuild",
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c
		}
	}
	return ""
}

// findBinary probes dir-relative candidates for a binary before falling back
// to PATH. Shared by the esbuild and prettier locators.
func findBinary(dirCandidates []string, name string) string {
	for _, c := range dirCandidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c
		}
	}
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	return ""
}

// CheckSyntax runs esbuild's parser over a JS/TS source string and returns
// syntax diagnostics (missing braces, invalid tokens, etc.). It does not do
// type checking — that requires a language server.
func (t *Tooling) CheckSyntax(ctx context.Context, path, content string) ([]SyntaxDiagnostic, error) {
	// Only JS/TS-family files for now.
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs", ".mts", ".cts":
	default:
		return []SyntaxDiagnostic{}, nil
	}

	esbuildBin := findEsbuild()
	if esbuildBin == "" {
		return []SyntaxDiagnostic{}, nil // esbuild not installed — silently no-op
	}

	tmpFile, err := os.CreateTemp("", "forge-syntax-*"+ext)
	if err != nil {
		return nil, fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)
	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("write temp: %w", err)
	}
	tmpFile.Close()

	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, esbuildBin, tmpPath, "--format=esm", "--log-level=warning")
	out, _ := cmd.CombinedOutput()

	var diags []SyntaxDiagnostic
	// esbuild error shape:
	//   ✘ [ERROR] Expected identifier but found end of file
	//       ../var/folders/.../forge-syntax-123.ts:1:11:
	//         1 │ const x = {
	//           ╵            ^
	// Column in esbuild output is 1-based, but esbuild reports it as 0-based
	// in the "line:col:" marker — we subtract 1 to get the 0-based position.
	lines := strings.Split(string(out), "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if strings.Contains(line, "[ERROR]") {
			msg := strings.TrimSpace(strings.TrimPrefix(line, "✘ [ERROR]"))
			diags = append(diags, SyntaxDiagnostic{Message: msg})
			continue
		}
		if len(diags) == 0 || !strings.Contains(line, ":") {
			continue
		}
		// Location line: "<path>:<line>:<col>:"
		// Match the LAST two colon-separated numbers.
		re := regexp.MustCompile(`:(\d+):(\d+):\s*$`)
		m := re.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		ln, _ := strconv.Atoi(m[1])
		col, _ := strconv.Atoi(m[2])
		if ln > 0 {
			diags[len(diags)-1].Line = ln
			diags[len(diags)-1].Column = col
		}
	}
	return diags, nil
}

// findPrettier resolves the prettier binary. Prefers the workspace's local
// node_modules install, falling back to PATH.
func (t *Tooling) findPrettier() string {
	return findBinary([]string{
		filepath.Join(t.root, "frontend", "node_modules", ".bin", "prettier"),
		filepath.Join(t.root, "node_modules", ".bin", "prettier"),
	}, "prettier")
}

// findPrettierConfig resolves the workspace .prettierrc file.
func (t *Tooling) findPrettierConfig() string {
	for _, name := range []string{".prettierrc", ".prettierrc.js", ".prettierrc.cjs", ".prettierrc.json", ".prettierrc.yaml", "prettier.config.js", "prettier.config.cjs"} {
		p := filepath.Join(t.root, name)
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p
		}
	}
	return ""
}

// FormatCode formats a JS/TS source string using the workspace's prettier
// (with its .prettierrc config when present). Returns the formatted source,
// or the original content if prettier is unavailable.
func (t *Tooling) FormatCode(ctx context.Context, path, content string) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	parser := ""
	switch ext {
	case ".js", ".mjs", ".cjs":
		parser = "babel"
	case ".jsx":
		parser = "babel"
	case ".ts", ".mts", ".cts":
		parser = "typescript"
	case ".tsx":
		parser = "typescript"
	case ".json":
		parser = "json"
	case ".css":
		parser = "css"
	case ".html":
		parser = "html"
	case ".md":
		parser = "markdown"
	default:
		return content, nil
	}

	prettierBin := t.findPrettier()
	if prettierBin == "" {
		return content, nil
	}

	tmpFile, err := os.CreateTemp("", "forge-format-*"+ext)
	if err != nil {
		return content, nil
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)
	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		return content, nil
	}
	tmpFile.Close()

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	args := []string{tmpPath, "--write", "--parser", parser}
	if cfg := t.findPrettierConfig(); cfg != "" {
		args = append(args, "--config", cfg)
	}
	cmd := exec.CommandContext(ctx, prettierBin, args...)
	// Run from the frontend dir (when present) so prettier can resolve
	// plugins installed there (e.g. prettier-plugin-organize-imports).
	frontendDir := filepath.Join(t.root, "frontend")
	if info, err := os.Stat(frontendDir); err == nil && info.IsDir() {
		cmd.Dir = frontendDir
	} else {
		cmd.Dir = t.root
	}
	if err := cmd.Run(); err != nil {
		return content, nil
	}
	formatted, err := os.ReadFile(tmpPath)
	if err != nil {
		return content, nil
	}
	return string(formatted), nil
}
