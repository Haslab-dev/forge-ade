package forgemcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hasdev/forge-ade/internal/board"
)

// RegisterWorkspaceTools registers tools that provide context on the current workspace.
func (s *Server) RegisterWorkspaceTools(bm *board.Manager) {
	s.RegisterTool(Tool{
		Name: "forge_workspace_info",
		Description: "Get environment context about ForgeADE and the current active workspace, " +
			"including working directory, board tasks breakdown, and git status.",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*ToolCallResult, error) {
		ws := s.resolveWorkspace()
		absWs, _ := filepath.Abs(ws)

		mgr := s.getBoardMgr(bm, ws)
		tasks, _ := mgr.ListTasks(ws)

		byStatus := map[string]int{
			"todo":        0,
			"in_progress": 0,
			"in_review":   0,
			"in_testing":  0,
			"done":        0,
		}
		for _, t := range tasks {
			byStatus[t.Status]++
		}

		info := map[string]interface{}{
			"app":             "ForgeADE",
			"version":         ServerVersion,
			"workspace":       absWs,
			"boardTotalTasks": len(tasks),
			"boardByStatus":   byStatus,
			"hasGit":          dirExists(filepath.Join(absWs, ".git")),
			"hasForgeConfig":  dirExists(filepath.Join(absWs, ".forge")),
		}

		data, _ := json.MarshalIndent(info, "", "  ")
		return TextResult(fmt.Sprintf("ForgeADE Workspace Context:\n%s", string(data))), nil
	})
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
