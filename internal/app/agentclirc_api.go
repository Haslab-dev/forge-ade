package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hasdev/forge-ade/internal/agentclirc"
	"github.com/hasdev/forge-ade/internal/conffmt"
)

// ---------------------------------------------------------------------------
// Agent CLI Runtime Config API
// ---------------------------------------------------------------------------

// ListAgentCliRuntimeConfigs reads the on-disk configuration of every known
// agent CLI (pi, omp, opencode, codex, claude, antigravity, gemini): config
// file paths, providers/models, MCP servers, and skills. Read-only — the UI
// manages entries by opening/revealing the reported files.
func (a *App) ListAgentCliRuntimeConfigs() []agentclirc.RuntimeConfig {
	return agentclirc.List()
}

// EnsureAgentCliPath creates a missing config path so it can be opened right
// away: directories with parents (mkdir -p), files as empty (O_EXCL — an
// existing file is never truncated). Returns true when something was created.
// Local paths only — config locations are always on this machine.
func (a *App) EnsureAgentCliPath(path string, isDir bool) (bool, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return false, fmt.Errorf("empty path")
	}
	if strings.HasPrefix(path, "ssh://") {
		return false, fmt.Errorf("remote config paths cannot be created here")
	}
	if _, err := os.Stat(path); err == nil {
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, err
	}
	if isDir {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return false, err
		}
		return true, nil
	}
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return false, err
		}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, f.Close()
}

// FormatConfigContent formats configuration content based on the path's
// extension: pretty JSON (key order preserved), compact JSONL (unparseable
// lines pass through), and a comment/order-preserving TOML tidy.
func (a *App) FormatConfigContent(path string, content string) (string, error) {
	return conffmt.Format(path, content)
}
