package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/Haslab-dev/RakitKode/tools"
)

// RegisterBoardTools registers autonomous workflow board tools into RakitKode's tool registry.
// These tools allow agents to create, inspect, update, move, and delete tasks on the project's
// workflow board without colliding with external Atlassian Jira MCP tools.
func RegisterBoardTools(r *tools.Registry, app *App) {
	if r == nil || app == nil {
		return
	}
	r.Register(boardCreateTaskTool(app))
	r.Register(boardListTasksTool(app))
	r.Register(boardGetTaskTool(app))
	r.Register(boardUpdateTaskTool(app))
	r.Register(boardDeleteTaskTool(app))
}

func boardCreateTaskTool(app *App) tools.ToolSpec {
	return tools.ToolSpec{
		Name: "board_create_task",
		Cost: "cheap",
		Description: "Create a new task on the project's internal workflow Kanban board. " +
			"Use this when you or the user want to record, track, or queue a task, bug, feature, or review item.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"title": map[string]any{
					"type":        "string",
					"description": "Concise summary / title of the task",
				},
				"description": map[string]any{
					"type":        "string",
					"description": "Optional detailed specifications, steps, acceptance criteria, or code context",
				},
				"status": map[string]any{
					"type":        "string",
					"enum":        []string{"todo", "in_progress", "in_review", "in_testing", "done"},
					"description": "Initial status (default: todo)",
				},
				"priority": map[string]any{
					"type":        "string",
					"enum":        []string{"low", "medium", "high", "urgent"},
					"description": "Priority level (default: medium)",
				},
				"assignee": map[string]any{
					"type":        "string",
					"description": "Assignee name (e.g. Pandu-Micro, Forge Agent, or developer name)",
				},
				"labels": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "string",
					},
					"description": "Optional tags/labels (e.g. ['frontend', 'bug', 'git'])",
				},
			},
			"required": []string{"title"},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			title := argString(args, "title")
			if title == "" {
				return nil, errRequired("title")
			}
			task := BoardTask{
				Title:       title,
				Description: argString(args, "description"),
				Status:      argString(args, "status"),
				Priority:    argString(args, "priority"),
				Assignee:    argString(args, "assignee"),
				Labels:      argStringSlice(args, "labels"),
			}
			created, err := app.SaveBoardTask("", task)
			if err != nil {
				return nil, err
			}
			return map[string]any{
				"success": true,
				"task":    created,
				"message": fmt.Sprintf("Created board task %s: %s (Status: %s)", created.Key, created.Title, created.Status),
			}, nil
		},
	}
}

func boardListTasksTool(app *App) tools.ToolSpec {
	return tools.ToolSpec{
		Name: "board_list_tasks",
		Cost: "cheap",
		Description: "List and inspect tasks on the project's internal workflow Kanban board. " +
			"Can filter by status (todo, in_progress, in_review, in_testing, done, all) or priority.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"status": map[string]any{
					"type":        "string",
					"description": "Filter by status: all, todo, in_progress, in_review, in_testing, or done (default: all)",
				},
				"priority": map[string]any{
					"type":        "string",
					"description": "Filter by priority: all, low, medium, high, or urgent (default: all)",
				},
				"query": map[string]any{
					"type":        "string",
					"description": "Optional search keyword to match against task title, key, or description",
				},
			},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			tasks, err := app.ListBoardTasks("")
			if err != nil {
				return nil, err
			}
			statusFilter := strings.ToLower(argString(args, "status"))
			priorityFilter := strings.ToLower(argString(args, "priority"))
			query := strings.ToLower(argString(args, "query"))

			filtered := make([]BoardTask, 0, len(tasks))
			for _, t := range tasks {
				if statusFilter != "" && statusFilter != "all" && !strings.EqualFold(t.Status, statusFilter) {
					continue
				}
				if priorityFilter != "" && priorityFilter != "all" && !strings.EqualFold(t.Priority, priorityFilter) {
					continue
				}
				if query != "" {
					match := strings.Contains(strings.ToLower(t.Key), query) ||
						strings.Contains(strings.ToLower(t.Title), query) ||
						strings.Contains(strings.ToLower(t.Description), query)
					if !match {
						continue
					}
				}
				filtered = append(filtered, t)
			}
			return map[string]any{
				"total": len(filtered),
				"tasks": filtered,
			}, nil
		},
	}
}

func boardGetTaskTool(app *App) tools.ToolSpec {
	return tools.ToolSpec{
		Name: "board_get_task",
		Cost: "cheap",
		Description: "Get complete details of a specific task on the workflow board by its key (e.g. 'ADE-1') or ID.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task_id_or_key": map[string]any{
					"type":        "string",
					"description": "The task key (e.g. 'ADE-1') or ID",
				},
			},
			"required": []string{"task_id_or_key"},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			idOrKey := argString(args, "task_id_or_key")
			if idOrKey == "" {
				return nil, errRequired("task_id_or_key")
			}
			task, err := app.GetBoardTask("", idOrKey)
			if err != nil {
				return nil, err
			}
			return task, nil
		},
	}
}

func boardUpdateTaskTool(app *App) tools.ToolSpec {
	return tools.ToolSpec{
		Name: "board_update_task",
		Cost: "cheap",
		Description: "Update fields of an existing task on the workflow board (status, title, description, priority, assignee, labels). " +
			"Use this to move tasks across statuses (todo -> in_progress -> in_review -> in_testing -> done) or modify task content.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task_id_or_key": map[string]any{
					"type":        "string",
					"description": "The task key (e.g. 'ADE-1') or ID to update",
				},
				"status": map[string]any{
					"type":        "string",
					"enum":        []string{"todo", "in_progress", "in_review", "in_testing", "done"},
					"description": "New status for the task",
				},
				"title": map[string]any{
					"type":        "string",
					"description": "New title",
				},
				"description": map[string]any{
					"type":        "string",
					"description": "New description",
				},
				"priority": map[string]any{
					"type":        "string",
					"enum":        []string{"low", "medium", "high", "urgent"},
					"description": "New priority",
				},
				"assignee": map[string]any{
					"type":        "string",
					"description": "New assignee",
				},
				"labels": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "string",
					},
					"description": "Updated tags/labels",
				},
			},
			"required": []string{"task_id_or_key"},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			idOrKey := argString(args, "task_id_or_key")
			if idOrKey == "" {
				return nil, errRequired("task_id_or_key")
			}
			existing, err := app.GetBoardTask("", idOrKey)
			if err != nil {
				return nil, err
			}
			if t := argString(args, "title"); t != "" {
				existing.Title = t
			}
			if d, ok := args["description"].(string); ok {
				existing.Description = d
			}
			if s := argString(args, "status"); s != "" {
				existing.Status = s
			}
			if p := argString(args, "priority"); p != "" {
				existing.Priority = p
			}
			if a := argString(args, "assignee"); a != "" {
				existing.Assignee = a
			}
			if l := argStringSlice(args, "labels"); l != nil {
				existing.Labels = l
			}
			saved, err := app.SaveBoardTask("", existing)
			if err != nil {
				return nil, err
			}
			return map[string]any{
				"success": true,
				"task":    saved,
				"message": fmt.Sprintf("Updated board task %s (Status: %s, Priority: %s)", saved.Key, saved.Status, saved.Priority),
			}, nil
		},
	}
}

func boardDeleteTaskTool(app *App) tools.ToolSpec {
	return tools.ToolSpec{
		Name: "board_delete_task",
		Cost: "cheap",
		Description: "Delete a task from the workflow board by its key (e.g. 'ADE-1') or ID.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task_id_or_key": map[string]any{
					"type":        "string",
					"description": "The task key (e.g. 'ADE-1') or ID to delete",
				},
			},
			"required": []string{"task_id_or_key"},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			idOrKey := argString(args, "task_id_or_key")
			if idOrKey == "" {
				return nil, errRequired("task_id_or_key")
			}
			if err := app.DeleteBoardTask("", idOrKey); err != nil {
				return nil, err
			}
			return map[string]any{
				"success": true,
				"message": fmt.Sprintf("Deleted task %s from workflow board", idOrKey),
			}, nil
		},
	}
}
