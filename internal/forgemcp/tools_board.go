package forgemcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/hasdev/forge-ade/internal/board"
)

// RegisterBoardTools registers all Kanban board / workflow tools.
func (s *Server) RegisterBoardTools(bm *board.Manager) {
	s.RegisterTool(Tool{
		Name: "forge_board_list_tasks",
		Description: "List Kanban workflow tasks from .forge/board_tasks.json in the current workspace. " +
			"Optionally filter by status (todo, in_progress, in_review, in_testing, done) or priority (low, medium, high, urgent).",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"status": map[string]interface{}{
					"type":        "string",
					"description": "Optional filter: todo | in_progress | in_review | in_testing | done",
				},
				"priority": map[string]interface{}{
					"type":        "string",
					"description": "Optional filter: low | medium | high | urgent",
				},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*ToolCallResult, error) {
		ws := s.resolveWorkspace()
		mgr := s.getBoardMgr(bm, ws)
		tasks, err := mgr.ListTasks(ws)
		if err != nil {
			return nil, fmt.Errorf("failed to list board tasks: %w", err)
		}

		filterStatus := strings.ToLower(getString(args, "status"))
		filterPriority := strings.ToLower(getString(args, "priority"))

		var filtered []board.Task
		for _, t := range tasks {
			if filterStatus != "" && strings.ToLower(t.Status) != filterStatus {
				continue
			}
			if filterPriority != "" && strings.ToLower(t.Priority) != filterPriority {
				continue
			}
			filtered = append(filtered, t)
		}

		data, _ := json.MarshalIndent(filtered, "", "  ")
		summary := fmt.Sprintf("Found %d tasks (total: %d) in workspace %s:\n\n%s", len(filtered), len(tasks), ws, string(data))
		return TextResult(summary), nil
	})

	s.RegisterTool(Tool{
		Name:        "forge_board_get_task",
		Description: "Get complete details of a specific workflow task by its ID or Key (e.g. 'ADE-1' or 'task-1729...').",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"task_id_or_key": map[string]interface{}{
					"type":        "string",
					"description": "The task key (e.g. ADE-1) or internal task ID",
				},
			},
			"required": []string{"task_id_or_key"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*ToolCallResult, error) {
		idOrKey := getString(args, "task_id_or_key")
		if idOrKey == "" {
			return nil, fmt.Errorf("task_id_or_key is required")
		}
		ws := s.resolveWorkspace()
		mgr := s.getBoardMgr(bm, ws)
		task, err := mgr.GetTask(ws, idOrKey)
		if err != nil {
			return nil, err
		}
		data, _ := json.MarshalIndent(task, "", "  ")
		return TextResult(string(data)), nil
	})

	s.RegisterTool(Tool{
		Name: "forge_board_create_task",
		Description: "Create a new workflow task on ForgeADE's Kanban board (.forge/board_tasks.json). " +
			"Automatically generates task key ADE-X and syncs with the desktop board view in real time.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"title": map[string]interface{}{
					"type":        "string",
					"description": "Short, descriptive task title",
				},
				"description": map[string]interface{}{
					"type":        "string",
					"description": "Detailed description, criteria, or context",
				},
				"priority": map[string]interface{}{
					"type":        "string",
					"description": "low | medium | high | urgent (defaults to medium)",
				},
				"status": map[string]interface{}{
					"type":        "string",
					"description": "todo | in_progress | in_review | in_testing | done (defaults to todo)",
				},
				"labels": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Optional tags/labels for categorization",
				},
			},
			"required": []string{"title"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*ToolCallResult, error) {
		title := getString(args, "title")
		if title == "" {
			return nil, fmt.Errorf("title is required")
		}
		ws := s.resolveWorkspace()
		mgr := s.getBoardMgr(bm, ws)

		status := getString(args, "status")
		if status == "" {
			status = "todo"
		}
		priority := getString(args, "priority")
		if priority == "" {
			priority = "medium"
		}

		labels := getStringSlice(args, "labels")

		t := board.Task{
			Title:         title,
			Description:   getString(args, "description"),
			Status:        status,
			Priority:      priority,
			Labels:        labels,
			WorkspacePath: ws,
		}

		saved, err := mgr.SaveTask(ws, t)
		if err != nil {
			return nil, fmt.Errorf("failed to create board task: %w", err)
		}

		data, _ := json.MarshalIndent(saved, "", "  ")
		return TextResult(fmt.Sprintf("Created task [%s] %s:\n%s", saved.Key, saved.Title, string(data))), nil
	})

	s.RegisterTool(Tool{
		Name: "forge_board_update_task",
		Description: "Update a task's status (e.g. moving from 'todo' -> 'in_progress' -> 'done'), " +
			"title, description, or priority on ForgeADE's Kanban board.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"task_id_or_key": map[string]interface{}{
					"type":        "string",
					"description": "Task key (e.g. 'ADE-1') or ID to update",
				},
				"status": map[string]interface{}{
					"type":        "string",
					"description": "New status: todo | in_progress | in_review | in_testing | done",
				},
				"title": map[string]interface{}{
					"type":        "string",
					"description": "Optional updated title",
				},
				"description": map[string]interface{}{
					"type":        "string",
					"description": "Optional updated description",
				},
				"priority": map[string]interface{}{
					"type":        "string",
					"description": "Optional updated priority: low | medium | high | urgent",
				},
			},
			"required": []string{"task_id_or_key"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*ToolCallResult, error) {
		idOrKey := getString(args, "task_id_or_key")
		if idOrKey == "" {
			return nil, fmt.Errorf("task_id_or_key is required")
		}
		ws := s.resolveWorkspace()
		mgr := s.getBoardMgr(bm, ws)

		existing, err := mgr.GetTask(ws, idOrKey)
		if err != nil {
			return nil, err
		}

		if newStatus := getString(args, "status"); newStatus != "" {
			existing.Status = newStatus
		}
		if newTitle := getString(args, "title"); newTitle != "" {
			existing.Title = newTitle
		}
		if newDesc := getString(args, "description"); newDesc != "" {
			existing.Description = newDesc
		}
		if newPriority := getString(args, "priority"); newPriority != "" {
			existing.Priority = newPriority
		}

		saved, err := mgr.SaveTask(ws, existing)
		if err != nil {
			return nil, fmt.Errorf("failed to update task: %w", err)
		}

		data, _ := json.MarshalIndent(saved, "", "  ")
		return TextResult(fmt.Sprintf("Updated task [%s] (status: %s):\n%s", saved.Key, saved.Status, string(data))), nil
	})

	s.RegisterTool(Tool{
		Name:        "forge_board_delete_task",
		Description: "Delete a task from ForgeADE's Kanban board by key or ID.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"task_id_or_key": map[string]interface{}{
					"type":        "string",
					"description": "The task key (e.g. ADE-1) or ID to delete",
				},
			},
			"required": []string{"task_id_or_key"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*ToolCallResult, error) {
		idOrKey := getString(args, "task_id_or_key")
		if idOrKey == "" {
			return nil, fmt.Errorf("task_id_or_key is required")
		}
		ws := s.resolveWorkspace()
		mgr := s.getBoardMgr(bm, ws)
		if err := mgr.DeleteTask(ws, idOrKey); err != nil {
			return nil, err
		}
		return TextResult(fmt.Sprintf("Deleted task %s successfully", idOrKey)), nil
	})
}

func (s *Server) resolveWorkspace() string {
	if s.workspace != "" {
		return s.workspace
	}
	cwd, err := os.Getwd()
	if err == nil && cwd != "" {
		return cwd
	}
	return "."
}

func (s *Server) getBoardMgr(bm *board.Manager, ws string) *board.Manager {
	if bm != nil {
		return bm
	}
	return board.NewManager(ws)
}

func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if s, ok := val.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func getStringSlice(m map[string]interface{}, key string) []string {
	if val, ok := m[key]; ok {
		if arr, ok := val.([]interface{}); ok {
			var out []string
			for _, item := range arr {
				if s, ok := item.(string); ok && s != "" {
					out = append(out, s)
				}
			}
			return out
		}
	}
	return nil
}
