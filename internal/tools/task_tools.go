package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// task_output / task_stop / task_list — the long-running-task control surface
// (port of ZCode's TaskOutput / TaskStop tools). Registered from
// Registry.registerCoreTools.

func taskOutputTool() ToolSpec {
	return ToolSpec{
		Name: "task_output",
		Cost: "cheap",
		Description: "Read output from a background task started with bash run_in_background. " +
			"Use block:true to wait for completion (up to `timeout` seconds), or block:false to poll current output.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task_id": map[string]any{"type": "string", "description": "Task id returned by bash run_in_background"},
				"block":   map[string]any{"type": "boolean", "description": "Wait for the task to finish before returning (default false)"},
				"timeout": map[string]any{"type": "integer", "description": "Max seconds to wait when blocking (default 30, max 300)"},
			},
			"required": []string{"task_id"},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			taskID := strings.TrimSpace(argString(args, "task_id"))
			if taskID == "" {
				return nil, fmt.Errorf("task_id is required")
			}
			if _, ok := GetTask(taskID); !ok {
				return toolResult(map[string]any{
					"task_id": taskID,
					"status":  "not_found",
					"message": fmt.Sprintf("task %s not found (it may have completed or session restarted; check task_list)", taskID),
				}), nil
			}
			timeoutSec := argInt(args, "timeout", 30)
			if timeoutSec < 0 {
				timeoutSec = 30
			}
			if timeoutSec > 300 {
				timeoutSec = 300
			}
			if argBool(args, "block", false) {
				task, _ := WaitTask(ctx, taskID, time.Duration(timeoutSec)*time.Second)
				if task != nil {
					return toolResult(task.taskSnapshot()), nil
				}
			}
			task, ok := GetTask(taskID)
			if !ok {
				return toolResult(map[string]any{
					"task_id": taskID,
					"status":  "not_found",
					"message": fmt.Sprintf("task %s not found", taskID),
				}), nil
			}
			return toolResult(task.taskSnapshot()), nil
		},
	}
}

func taskStopTool() ToolSpec {
	return ToolSpec{
		Name: "task_stop",
		Cost: "cheap",
		Description: "Stop a running background task (kills its whole process group). " +
			"Returns the task's final state.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task_id": map[string]any{"type": "string", "description": "Task id returned by bash run_in_background"},
			},
			"required": []string{"task_id"},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			taskID := strings.TrimSpace(argString(args, "task_id"))
			if taskID == "" {
				return nil, fmt.Errorf("task_id is required")
			}
			task, ok := GetTask(taskID)
			if !ok {
				return toolResult(map[string]any{
					"task_id": taskID,
					"status":  "stopped",
					"message": fmt.Sprintf("task %s was not found or is already terminated", taskID),
				}), nil
			}
			if task.Status != TaskRunning {
				return toolResult(map[string]any{
					"task_id": taskID,
					"status":  string(task.Status),
					"message": fmt.Sprintf("task is not running (status: %s)", task.Status),
				}), nil
			}
			StopTask(taskID)
			// Give the process a moment to exit so the returned snapshot is terminal.
			final, _ := WaitTask(context.Background(), taskID, 2*time.Second)
			if final == nil {
				final = task
			}
			return toolResult(map[string]any{
				"task_id": taskID,
				"status":  string(final.Status),
				"message": "task stopped",
			}), nil
		},
	}
}

func taskListTool() ToolSpec {
	return ToolSpec{
		Name:        "task_list",
		Cost:        "cheap",
		Description: "List this session's background tasks with their status (running/completed/failed/stopped).",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			sessionID := ""
			if bridge := SessionBridgeFrom(ctx); bridge != nil {
				sessionID = bridge.SessionID()
			}
			tasks := ListTasks(sessionID)
			out := make([]map[string]any, 0, len(tasks))
			for _, t := range tasks {
				out = append(out, t.taskSnapshot())
			}
			return toolResult(map[string]any{"tasks": out, "count": len(out)}), nil
		},
	}
}

// updateTasksTool allows the agent to update its task checklist with structured statuses
// (pending, in_progress, completed) and active continuous forms ("Running unit tests...").
func updateTasksTool() ToolSpec {
	return ToolSpec{
		Name: "update_tasks",
		Cost: "cheap",
		Description: "Update, replace, or clear the task checklist for the session. " +
			"Calling this replaces the entire active checklist with the provided tasks. " +
			"When all prior tasks are completed and new user instructions arrive, or when the objective pivots, " +
			"call update_tasks with a fresh task breakdown to reset the checklist. Pass tasks: [] to clear the checklist completely.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"tasks": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"id":          map[string]any{"type": "string", "description": "Task sequence or ID (e.g. '1', '2')"},
							"title":       map[string]any{"type": "string", "description": "Task title (imperative form)"},
							"status":      map[string]any{"type": "string", "enum": []string{"pending", "in_progress", "completed"}, "description": "Task status"},
							"active_form": map[string]any{"type": "string", "description": "Present continuous form when in progress (e.g. 'Running tests')"},
						},
						"required": []string{"title", "status"},
					},
					"description": "The updated list of tasks (or empty array [] to clear/reset)",
				},
			},
			"required": []string{"tasks"},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			bridge := SessionBridgeFrom(ctx)
			if bridge == nil {
				return nil, fmt.Errorf("update_tasks is unavailable outside an agent session")
			}
			var rawList []any
			if l, ok := args["tasks"].([]any); ok {
				rawList = l
			} else if s, ok := args["tasks"].(string); ok {
				var parsed []any
				if err := json.Unmarshal([]byte(s), &parsed); err == nil {
					rawList = parsed
				} else {
					return nil, fmt.Errorf("tasks parameter must be an array")
				}
			} else {
				return nil, fmt.Errorf("tasks parameter must be an array")
			}

			if len(rawList) == 0 {
				bridge.SetTodos([]TodoItem{})
				return toolResult(map[string]any{
					"summary":        "Checklist cleared and reset for new objective.",
					"completed":      0,
					"in_progress":    0,
					"pending":        0,
					"total":          0,
					"just_completed": []string{},
					"current_focus":  "",
					"tasks":          []TodoItem{},
				}), nil
			}

			oldTodos := bridge.GetTodos()
			oldStatus := make(map[string]string)
			for _, t := range oldTodos {
				oldStatus[t.Title] = t.Status
			}

			newTodos := make([]TodoItem, 0, len(rawList))
			var justCompleted []string
			var currentFocus string
			completedCount, inProgressCount, pendingCount := 0, 0, 0

			for i, item := range rawList {
				m, ok := item.(map[string]any)
				if !ok {
					continue
				}
				title := strings.TrimSpace(argString(m, "title"))
				if title == "" {
					continue
				}
				id := strings.TrimSpace(argString(m, "id"))
				if id == "" {
					id = fmt.Sprintf("t%d", i+1)
				}
				rawStatus := strings.ToLower(strings.TrimSpace(argString(m, "status")))
				var status string
				switch rawStatus {
				case "completed", "complete", "done", "finished":
					status = "completed"
				case "in_progress", "in-progress", "running", "active", "working":
					status = "in_progress"
				default:
					status = "pending"
				}
				activeForm := strings.TrimSpace(argString(m, "active_form"))

				switch status {
				case "completed":
					completedCount++
					if prev, exists := oldStatus[title]; !exists || prev != "completed" {
						justCompleted = append(justCompleted, title)
					}
				case "in_progress":
					inProgressCount++
					if activeForm != "" {
						currentFocus = activeForm
					} else {
						currentFocus = title
					}
				case "pending":
					pendingCount++
				}

				newTodos = append(newTodos, TodoItem{
					ID:         id,
					Title:      title,
					Status:     status,
					ActiveForm: activeForm,
				})
			}

			bridge.SetTodos(newTodos)

			total := len(newTodos)
			resp := fmt.Sprintf("Progress: %d/%d tasks completed.\nStatus: %d completed, %d in progress, %d pending.\n",
				completedCount, total, completedCount, inProgressCount, pendingCount)
			if len(justCompleted) > 0 {
				resp += fmt.Sprintf("Just completed: %s\n", strings.Join(justCompleted, ", "))
			}
			if currentFocus != "" {
				resp += fmt.Sprintf("Current focus: %s\n", currentFocus)
			}
			if completedCount == total && total > 0 {
				resp += "All tasks completed! If the user provides new instructions or next steps, call update_tasks with a fresh checklist to reset.\n"
			}

			return toolResult(map[string]any{
				"summary":        resp,
				"completed":      completedCount,
				"in_progress":    inProgressCount,
				"pending":        pendingCount,
				"total":          total,
				"just_completed": justCompleted,
				"current_focus":  currentFocus,
				"tasks":          newTodos,
			}), nil
		},
	}
}

// invokeSubagentTool allows the agent to spawn an autonomous child agent in the background.
func invokeSubagentTool() ToolSpec {
	return ToolSpec{
		Name:        "invoke_subagent",
		Cost:        "expensive",
		Description: "Spawn an autonomous sub-agent to handle an isolated task, research, or execution. The sub-agent runs independently and reports results.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"role": map[string]any{
					"type":        "string",
					"description": "Role of the sub-agent (e.g. 'research', 'coding', 'planning', 'custom')",
				},
				"task": map[string]any{
					"type":        "string",
					"description": "Clear instruction or prompt for the sub-agent to execute",
				},
				"block": map[string]any{
					"type":        "boolean",
					"description": "Wait for the sub-agent to complete before returning results (default false)",
				},
				"timeout": map[string]any{
					"type":        "integer",
					"description": "Max seconds to wait when blocking (default 60, max 300)",
				},
			},
			"required": []string{"task"},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			bridge := SessionBridgeFrom(ctx)
			if bridge == nil {
				return nil, fmt.Errorf("invoke_subagent is unavailable outside an agent session")
			}
			task := strings.TrimSpace(argString(args, "task"))
			if task == "" {
				return nil, fmt.Errorf("task parameter is required")
			}
			role := strings.TrimSpace(argString(args, "role"))
			if role == "" {
				role = "coding"
			}
			subID, err := bridge.SpawnSubagent(task, role)
			if err != nil {
				return nil, fmt.Errorf("failed to spawn subagent: %w", err)
			}

			if argBool(args, "block", false) {
				timeoutSec := argInt(args, "timeout", 60)
				if timeoutSec <= 0 {
					timeoutSec = 60
				}
				if timeoutSec > 300 {
					timeoutSec = 300
				}
				res, err := bridge.WaitSubagent(ctx, subID, time.Duration(timeoutSec)*time.Second)
				if err != nil {
					return nil, fmt.Errorf("waiting for subagent %s failed: %w", subID, err)
				}
				res["role"] = role
				res["task"] = task
				return toolResult(res), nil
			}

			return toolResult(map[string]any{
				"status":      "spawned",
				"subagent_id": subID,
				"role":        role,
				"task":        task,
				"message":     fmt.Sprintf("Spawned sub-agent (%s) with session ID %s. It is executing in the background. Use get_subagent_output to check results.", role, subID),
			}), nil
		},
	}
}

// getSubagentOutputTool reads current or final output from a child agent session.
func getSubagentOutputTool() ToolSpec {
	return ToolSpec{
		Name: "get_subagent_output",
		Cost: "cheap",
		Description: "Read output from a spawned sub-agent. " +
			"Use block:true to wait for completion (up to `timeout` seconds), or block:false to check current status.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"subagent_id": map[string]any{"type": "string", "description": "Subagent ID returned by invoke_subagent"},
				"block":       map[string]any{"type": "boolean", "description": "Wait for the subagent to finish (default false)"},
				"timeout":     map[string]any{"type": "integer", "description": "Max seconds to wait when blocking (default 30, max 300)"},
			},
			"required": []string{"subagent_id"},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			bridge := SessionBridgeFrom(ctx)
			if bridge == nil {
				return nil, fmt.Errorf("get_subagent_output is unavailable outside an agent session")
			}
			subID := strings.TrimSpace(argString(args, "subagent_id"))
			if subID == "" {
				return nil, fmt.Errorf("subagent_id is required")
			}
			timeoutSec := argInt(args, "timeout", 30)
			if timeoutSec < 0 {
				timeoutSec = 30
			}
			if timeoutSec > 300 {
				timeoutSec = 300
			}
			if !argBool(args, "block", false) {
				timeoutSec = 0
			}
			res, err := bridge.WaitSubagent(ctx, subID, time.Duration(timeoutSec)*time.Second)
			if err != nil {
				return nil, err
			}
			return toolResult(res), nil
		},
	}
}

// listSubagentsTool lists all sub-agents spawned by the current session.
func listSubagentsTool() ToolSpec {
	return ToolSpec{
		Name:        "list_subagents",
		Cost:        "cheap",
		Description: "List all child sub-agents spawned by this session, including their IDs, roles, states, and outputs.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			bridge := SessionBridgeFrom(ctx)
			if bridge == nil {
				return nil, fmt.Errorf("list_subagents is unavailable outside an agent session")
			}
			subs := bridge.GetSubagents()
			return toolResult(map[string]any{
				"subagents": subs,
				"count":     len(subs),
			}), nil
		},
	}
}
