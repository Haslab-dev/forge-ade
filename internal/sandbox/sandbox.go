// Package sandbox provisions a per-project scratch workspace under
// ~/.forge/sandbox/<slug> so the built-in agent has somewhere to edit temp
// files, work through thinking, prototype code and write logs without
// touching the user's workspace. Layout:
//
//	~/.forge/sandbox/<project-slug>/
//	  tmp/        throwaway scratch, pruned by SweepTemp
//	  thinking/   scratch notes and working-through problems
//	  coding/     temp code experiments
//	  logs/       agent-written session logs
package sandbox

import (
	"os"
	"path/filepath"
	"time"

	"github.com/hasdev/forge-ade/internal/globalstore"
)

// Sub directories created inside every sandbox.
const (
	DirTmp      = "tmp"
	DirThinking = "thinking"
	DirCoding   = "coding"
	DirLogs     = "logs"
)

var subDirs = []string{DirTmp, DirThinking, DirCoding, DirLogs}

// Ensure provisions the sandbox for projectPath (creating all sub dirs) and
// returns its root. Safe to call repeatedly — every step is idempotent and
// cheap, so per-turn callers can rely on it.
func Ensure(projectPath string) (string, error) {
	root, err := globalstore.ProjectDir("sandbox", projectPath)
	if err != nil {
		return "", err
	}
	for _, sub := range subDirs {
		if err := os.MkdirAll(filepath.Join(root, sub), 0o755); err != nil {
			return "", err
		}
	}
	return root, nil
}

// Sub returns the path of one sandbox sub directory for projectPath,
// provisioning the sandbox first.
func Sub(projectPath, sub string) (string, error) {
	root, err := Ensure(projectPath)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, sub), nil
}

// Under reports whether path is inside projectPath's sandbox root.
func Under(projectPath, path string) bool {
	root, err := Ensure(projectPath)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && filepath.IsLocal(rel)
}

// SweepTemp removes files and directories older than maxAge from every
// sandbox's tmp/ directory. Other sub directories are never touched. It
// scans all sandboxes (every slug under ~/.forge/sandbox), so it can run
// once at startup without knowing which projects exist.
func SweepTemp(maxAge time.Duration) {
	home, err := globalstore.Root()
	if err != nil {
		return
	}
	root := filepath.Join(home, "sandbox")
	slugs, err := os.ReadDir(root)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-maxAge)
	for _, slug := range slugs {
		if !slug.IsDir() {
			continue
		}
		sweepTmpDir(filepath.Join(root, slug.Name(), DirTmp), cutoff)
	}
}

func sweepTmpDir(dir string, cutoff time.Time) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		modTime := info.ModTime()
		if !modTime.Before(cutoff) {
			continue
		}
		_ = os.RemoveAll(filepath.Join(dir, e.Name()))
	}
}
