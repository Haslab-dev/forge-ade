// Package globalstore resolves per-project storage under ~/.forge so no
// helper state (.forge/, .workspace/) leaks into user workspaces. Each kind
// (sessions, memory, skills, plugins, index) gets ~/.forge/<kind>/<slug>/,
// where <slug> is the workspace folder name plus a short hash of its
// absolute path — same-named projects in different locations stay separate.
package globalstore

import (
	"crypto/sha1"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

// Root returns ~/.forge, creating it if needed.
func Root() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".forge")
	return dir, os.MkdirAll(dir, 0o755)
}

// ProjectSlug derives a stable, filesystem-safe per-project name from a
// workspace path: "<leaf>-<8-char hash of the absolute path>".
func ProjectSlug(workspacePath string) string {
	clean := strings.TrimRight(strings.TrimSpace(workspacePath), "/\\")
	if clean == "" || clean == "." {
		return "default"
	}
	leaf := clean
	if i := strings.LastIndexAny(clean, "/\\"); i >= 0 {
		leaf = clean[i+1:]
	}
	if leaf == "" {
		leaf = "default"
	}
	abs, err := filepath.Abs(clean)
	if err != nil {
		abs = clean
	}
	sum := sha1.Sum([]byte(abs))
	return leaf + "-" + hex.EncodeToString(sum[:])[:8]
}

// ProjectDir returns ~/.forge/<kind>/<slug>, creating it if needed.
func ProjectDir(kind, workspacePath string) (string, error) {
	root, err := Root()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(root, kind, ProjectSlug(workspacePath))
	return dir, os.MkdirAll(dir, 0o755)
}
