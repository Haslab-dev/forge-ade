package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/Haslab-dev/RakitKode/agent"
	"github.com/Haslab-dev/RakitKode/llm"
	"github.com/Haslab-dev/RakitKode/tools"
)

// BoardToolNames defines the tools registered for workflow board manipulation.
var BoardToolNames = []string{
	"board_create_task",
	"board_list_tasks",
	"board_get_task",
	"board_update_task",
	"board_delete_task",
	"board",
}

// BoardSystemPromptGuidance instructs the LLM on how and when to use the workflow board tools.
const BoardSystemPromptGuidance = `
## Workflow Kanban Board Integration
Forge ADE includes an integrated project Workflow Kanban Board for tracking tasks, bugs, features, and backlogs.
You have native board management tools:
- board_create_task: Create a task on the Kanban board (title, description, priority: "low"|"medium"|"high"|"urgent", status: "todo"|"in_progress"|"in_review"|"in_testing"|"done", assignee, labels).
- board_list_tasks: List and search existing tasks on the board (supports filtering by status or priority, and keyword search).
- board_get_task: Get complete details of a task by ID or key (e.g. ADE-1).
- board_update_task: Move tasks across statuses, change priority, or edit title/description/assignee/labels.
- board_delete_task: Delete a task by ID or key.

MANDATORY RULES:
1. When the user asks to create, add, or log a task or issue (e.g. "tolong buatkan saya task baru dengan priority high", "create a bug task for ADE crash"), you MUST immediately call board_create_task! Never say you lack task manager integration; these board tools are your native task manager integration.
2. When the user asks about tasks on the board, backlog, or status of work, call board_list_tasks or board_get_task.
3. When the user asks to change task status (e.g. mark done, in progress), call board_update_task.
`

// resolveToolWorkspace identifies the workspace folder for a tool call.
func resolveToolWorkspace(ctx context.Context, app *App, args map[string]any) string {
	if ws := argString(args, "workspace"); ws != "" {
		return ws
	}
	if ws := argString(args, "workspace_path"); ws != "" {
		return ws
	}
	if app != nil && app.agentMgr != nil {
		if sessionID := llm.SessionIDFrom(ctx); sessionID != "" {
			if sess, ok := app.agentMgr.GetSession(sessionID); ok && sess != nil && sess.Folder != "" {
				return sess.Folder
			}
		}
	}
	if app != nil {
		return app.defaultFolder()
	}
	return ""
}

// ensureSessionBoardTools guarantees that an agent session has board tools in its whitelist
// and includes the Workflow Kanban Board guidance in its system prompt.
func (a *App) ensureSessionBoardTools(sess *agent.Session) {
	if sess == nil {
		return
	}

	// 1. Ensure board tool names are in sess.Tools whitelist
	existingTools := make(map[string]bool, len(sess.Tools))
	for _, t := range sess.Tools {
		existingTools[strings.ToLower(strings.TrimSpace(t))] = true
	}
	for _, bt := range BoardToolNames {
		if !existingTools[bt] {
			sess.Tools = append(sess.Tools, bt)
			existingTools[bt] = true
		}
	}

	// 2. Ensure BoardSystemPromptGuidance is in sess.SystemPrompt
	if !strings.Contains(sess.SystemPrompt, "board_create_task") {
		sess.SystemPrompt = strings.TrimRight(sess.SystemPrompt, "\n") + "\n\n" + strings.TrimSpace(BoardSystemPromptGuidance) + "\n"
	}
}

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

	// Ensure all existing agent definitions include board tools
	if app.agentMgr != nil {
		for _, def := range app.agentMgr.ListAgentDefinitions() {
			hasBoard := false
			for _, t := range def.Tools {
				if t == "board_create_task" {
					hasBoard = true
					break
				}
			}
			if !hasBoard {
				def.Tools = append(def.Tools, BoardToolNames...)
				_, _ = app.agentMgr.SaveAgentDefinition(def)
			}
		}
		// Also patch any active sessions in memory
		for _, sess := range app.agentMgr.ListSessions() {
			app.ensureSessionBoardTools(sess)
		}
	}
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
			ws := resolveToolWorkspace(ctx, app, args)
			task := BoardTask{
				Title:         title,
				Description:   argString(args, "description"),
				Status:        argString(args, "status"),
				Priority:      argString(args, "priority"),
				Assignee:      argString(args, "assignee"),
				Labels:        argStringSlice(args, "labels"),
				WorkspacePath: ws,
			}
			created, err := app.SaveBoardTask(ws, task)
			if err != nil {
				return nil, err
			}
			return map[string]any{
				"success": true,
				"task":    created,
				"message": fmt.Sprintf("Created board task %s: %s (Status: %s, Priority: %s)", created.Key, created.Title, created.Status, created.Priority),
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
			ws := resolveToolWorkspace(ctx, app, args)
			tasks, err := app.ListBoardTasks(ws)
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
		Name:        "board_get_task",
		Cost:        "cheap",
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
			ws := resolveToolWorkspace(ctx, app, args)
			task, err := app.GetBoardTask(ws, idOrKey)
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
			ws := resolveToolWorkspace(ctx, app, args)
			existing, err := app.GetBoardTask(ws, idOrKey)
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
			saved, err := app.SaveBoardTask(ws, existing)
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
		Name:        "board_delete_task",
		Cost:        "cheap",
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
			ws := resolveToolWorkspace(ctx, app, args)
			if err := app.DeleteBoardTask(ws, idOrKey); err != nil {
				return nil, err
			}
			return map[string]any{
				"success": true,
				"message": fmt.Sprintf("Deleted task %s from workflow board", idOrKey),
			}, nil
		},
	}
}
