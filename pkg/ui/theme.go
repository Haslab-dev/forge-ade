package ui

import (
	"fmt"
	"strings"
)

// Workspace Surface Modes (ForgeADE Dual-Surface architecture)
const (
	ModeAgent  = 0 // ✨ Autonomous Agent Mode
	ModeEditor = 1 // 💻 Code Editor Mode
)

// Right Side Drawer Tabs
const (
	RightTabGit      = 0
	RightTabDatabase = 1
	RightTabTasks    = 2
	RightTabAgent    = 3
)

// Editor Activity Bar Tabs
const (
	ActivityExplorer = 0
	ActivityGit      = 1
	ActivityDatabase = 2
)

// UI Tab Constants (kept for backward compatibility)
const (
	TabEditor   = 0
	TabDiff     = 1
	TabTerminal = 2
	TabAgent    = 3
	TabGit      = 4
	TabDatabase = 5
	TabSplit    = 6
)

// TabNames defines user-facing labels for the navigation bar.
var TabNames = []string{
	"Editor",
	"Diff",
	"Terminal",
	"Agent AI",
	"Source Control",
	"Database",
	"Split View",
}

// FormatLineCol returns a formatted string like "Ln 12, Col 4"
func FormatLineCol(line, col int) string {
	return fmt.Sprintf("Ln %d, Col %d", line+1, col+1)
}

// ShortenPath abbreviates a long absolute path for display in status bars.
func ShortenPath(fullPath, rootDir string) string {
	if strings.HasPrefix(fullPath, rootDir) {
		rel := strings.TrimPrefix(fullPath, rootDir)
		rel = strings.TrimPrefix(rel, "/")
		if rel == "" {
			return "."
		}
		return rel
	}
	return fullPath
}
