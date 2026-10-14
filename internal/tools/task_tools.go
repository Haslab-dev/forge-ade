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

// resolveTaskID extracts task_id from common argument keys (task_id, taskId, id, task).
func resolveTaskID(args map[string]any) string {
	for _, key := range []string{"task_id", "taskId", "id", "task"} {
		if v := strings.TrimSpace(argString(args, key)); v != "" {
			return v
		}
	}
	return ""
}

func taskOutputTool() ToolSpec {
	return ToolSpec{
		Name: "task_output",
		Cost: "cheap",
		Description: "Read output from a background task started with bash run_in_background. " +
			"Use block:true to wait for completion (up to `timeout` seconds), or block:false to poll current output.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task_id": map[string]any{"type": "string", "description": "Task id returned by bash run_in_background (or omitted if only one task is running)"},
				"block":   map[string]any{"type": "boolean", "description": "Wait for the task to finish before returning (default false)"},
				"timeout": map[string]any{"type": "integer", "description": "Max seconds to wait when blocking (default 30, max 300)"},
			},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			taskID := resolveTaskID(args)
			if taskID == "" {
				allTasks := ListTasks("")
				if len(allTasks) == 1 {
					taskID = allTasks[0].ID
				} else if len(allTasks) > 1 {
					var running []*BackgroundTask
					for _, t := range allTasks {
						if t.Status == TaskRunning {
							running = append(running, t)
						}
					}
					if len(running) == 1 {
						taskID = running[0].ID
					} else {
						var summaries []map[string]any
						for _, t := range allTasks {
							summaries = append(summaries, map[string]any{
								"task_id": t.ID,
								"command": t.Command,
								"status":  string(t.Status),
							})
						}
						return toolResult(map[string]any{
							"status":       "ambiguous_task_id",
							"message":      "task_id was not provided. Multiple background tasks exist; please specify task_id from active tasks below.",
							"active_tasks": summaries,
						}), nil
					}
				} else {
					return toolResult(map[string]any{
						"status":  "no_tasks",
						"message": "task_id was not provided and no background tasks currently exist. Start one with bash run_in_background: true.",
					}), nil
				}
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
				"task_id": map[string]any{"type": "string", "description": "Task id returned by bash run_in_background (or omitted if only one task is running)"},
			},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			taskID := resolveTaskID(args)
			if taskID == "" {
				allTasks := ListTasks("")
				var running []*BackgroundTask
				for _, t := range allTasks {
					if t.Status == TaskRunning {
						running = append(running, t)
					}
				}
				if len(running) == 1 {
					taskID = running[0].ID
				} else if len(allTasks) == 1 {
					taskID = allTasks[0].ID
				} else {
					return toolResult(map[string]any{
						"status":  "error",
						"message": "task_id is required to stop a background task.",
					}), nil
				}
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

// taskTool implements Maki-style Subagents as "Garbage Collectors".
// When delegating research or search, a subagent is spawned with its own isolated context window.
// The subagent can run dozens of searches, greps, and dead ends.
// When finished, the subagent's full context is discarded ("garbage collected"), returning only
// a concise summary or schema-validated structured output (output_schema) to the parent agent.
// Model tiering: Weak models (Haiku, Flash) handle cheap exploration; Strong models (Opus, Sonnet) retain master coordination.
func taskTool() ToolSpec {
	return ToolSpec{
		Name: "task",
		Cost: "expensive",
		Description: "Delegate research, exploration, or a subtask to an isolated sub-agent ('Subagents as Garbage Collectors'). " +
			"The subagent executes searches, file inspections, and dead ends in its own isolated context window. " +
			"When finished, the subagent's full execution context is discarded ('garbage collected'), returning only a concise summary or schema-validated structured output (output_schema) to you. " +
			"Model tiering: Weak models (Flash, Haiku) handle cheap exploration by default; Strong models (Sonnet, Opus, Pro) retain master coordination.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task": map[string]any{
					"type":        "string",
					"description": "Clear instruction or research goal for the subagent to explore and summarize",
				},
				"subagent_type": map[string]any{
					"type":        "string",
					"description": "Role or specialist to invoke: 'research' (default, read-only exploration), 'search', 'coding', 'planning', or 'auto' (Jev selects).",
				},
				"model_tier": map[string]any{
					"type":        "string",
					"description": "Model tier: 'weak' (default for research/search — uses Flash/Haiku for cheap exploration) or 'strong' (Opus/Sonnet/Pro for master coordination), or an exact model name.",
				},
				"output_schema": map[string]any{
					"description": "Optional JSON schema or description of the structured output expected from the subagent.",
				},
				"block": map[string]any{
					"type":        "boolean",
					"description": "Wait for the subagent to complete before returning results (default true).",
				},
				"timeout": map[string]any{
					"type":        "integer",
					"description": "Max seconds to wait when blocking (default 120, max 600).",
				},
			},
			"required": []string{"task"},
		},
		Handler: handleTaskTool,
	}
}

// invokeSubagentTool allows the agent to spawn an autonomous child agent in the background.
func invokeSubagentTool() ToolSpec {
	return ToolSpec{
		Name:        "invoke_subagent",
		Cost:        "expensive",
		Description: "Spawn an autonomous sub-agent to handle an isolated task, research, or execution. If 'subagent' is omitted or 'auto', Jev dynamically evaluates the task and selects the best matching sub-agent.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"subagent": map[string]any{
					"type":        "string",
					"description": "Name, ID, or role of the sub-agent (e.g. 'research', 'coding', 'planning', or a custom sub-agent name). Set to 'auto' or omit to let Jev automatically analyze the task and pick the ideal sub-agent.",
				},
				"role": map[string]any{
					"type":        "string",
					"description": "Legacy alias for 'subagent'. Role or specialist to invoke ('auto', 'research', 'coding', 'planning').",
				},
				"model": map[string]any{
					"type":        "string",
					"description": "Optional model override (e.g. 'deepseek-v4-flash', 'gpt-6-luna', or provider/model). If omitted, uses the selected sub-agent's configured model.",
				},
				"model_tier": map[string]any{
					"type":        "string",
					"description": "Optional model tier: 'weak' (Flash/Haiku) or 'strong' (Opus/Sonnet/Pro).",
				},
				"task": map[string]any{
					"type":        "string",
					"description": "Clear instruction or prompt for the sub-agent to execute",
				},
				"output_schema": map[string]any{
					"description": "Optional JSON schema or description of structured output expected from the subagent.",
				},
				"block": map[string]any{
					"type":        "boolean",
					"description": "Wait for the sub-agent to complete before returning results (default false for invoke_subagent, true for task)",
				},
				"timeout": map[string]any{
					"type":        "integer",
					"description": "Max seconds to wait when blocking (default 60, max 300)",
				},
			},
			"required": []string{"task"},
		},
		Handler: handleInvokeSubagent,
	}
}

func handleTaskTool(ctx context.Context, args map[string]any) (any, error) {
	return executeSubagentTask(ctx, args, true)
}

func handleInvokeSubagent(ctx context.Context, args map[string]any) (any, error) {
	return executeSubagentTask(ctx, args, false)
}

func executeSubagentTask(ctx context.Context, args map[string]any, defaultBlock bool) (any, error) {
	bridge := SessionBridgeFrom(ctx)
	if bridge == nil {
		return nil, fmt.Errorf("subagent execution is unavailable outside an agent session")
	}
	task := strings.TrimSpace(argString(args, "task"))
	if task == "" {
		task = strings.TrimSpace(argString(args, "prompt"))
	}
	if task == "" {
		task = strings.TrimSpace(argString(args, "description"))
	}
	if task == "" {
		return nil, fmt.Errorf("task parameter is required")
	}

	subagent := strings.TrimSpace(argString(args, "subagent_type"))
	if subagent == "" {
		subagent = strings.TrimSpace(argString(args, "subagent"))
	}
	if subagent == "" {
		subagent = strings.TrimSpace(argString(args, "role"))
	}
	if subagent == "" {
		subagent = "research"
	}

	modelTier := strings.TrimSpace(argString(args, "model_tier"))
	if modelTier == "" {
		modelTier = strings.TrimSpace(argString(args, "model"))
	}
	if modelTier == "" {
		if subagent == "research" || subagent == "search" || subagent == "explore" {
			modelTier = "weak"
		}
	}

	// Output Schema handling: if parent specified output_schema, append instructions
	effectiveTask := task
	var outputSchemaStr string
	if schemaVal, ok := args["output_schema"]; ok && schemaVal != nil {
		switch s := schemaVal.(type) {
		case string:
			outputSchemaStr = s
		default:
			if b, err := json.Marshal(s); err == nil {
				outputSchemaStr = string(b)
			}
		}
	}
	if outputSchemaStr != "" {
		effectiveTask += fmt.Sprintf("\n\n[Structured Output Constraint]:\nYour response MUST strictly conform to this schema/format:\n%s\nReturn ONLY the structured result without conversational commentary or markdown fencing if JSON is expected.", outputSchemaStr)
	}

	subID, err := bridge.SpawnSubagentWithModel(effectiveTask, subagent, modelTier)
	if err != nil {
		return nil, fmt.Errorf("failed to spawn subagent: %w", err)
	}

	// Determine actual role and name assigned to the subagent (e.g. if Jev auto-routed)
	assignedRole := subagent
	assignedName := subagent
	for _, sub := range bridge.GetSubagents() {
		if sub.ID == subID {
			if sub.Role != "" {
				assignedRole = sub.Role
			}
			if sub.Name != "" {
				assignedName = sub.Name
			}
			break
		}
	}

	block := defaultBlock
	if _, exists := args["block"]; exists {
		block = argBool(args, "block", defaultBlock)
	}

	if block {
		timeoutSec := argInt(args, "timeout", 120)
		if timeoutSec <= 0 {
			timeoutSec = 120
		}
		if timeoutSec > 600 {
			timeoutSec = 600
		}
		res, err := bridge.WaitSubagent(ctx, subID, time.Duration(timeoutSec)*time.Second)
		if err != nil {
			return nil, fmt.Errorf("waiting for subagent %s failed: %w", subID, err)
		}
		res["subagent"] = assignedName
		res["role"] = assignedRole
		res["task"] = task
		res["context_garbage_collected"] = true

		// If output_schema was provided and response is valid JSON, parse it into structured_output
		if outputStr, ok := res["output"].(string); ok && outputSchemaStr != "" {
			var parsedJSON any
			cleanJSON := strings.TrimSpace(outputStr)
			if strings.HasPrefix(cleanJSON, "```json") {
				cleanJSON = strings.TrimPrefix(cleanJSON, "```json")
				cleanJSON = strings.TrimSuffix(cleanJSON, "```")
				cleanJSON = strings.TrimSpace(cleanJSON)
			} else if strings.HasPrefix(cleanJSON, "```") {
				cleanJSON = strings.TrimPrefix(cleanJSON, "```")
				cleanJSON = strings.TrimSuffix(cleanJSON, "```")
				cleanJSON = strings.TrimSpace(cleanJSON)
			}
			if err := json.Unmarshal([]byte(cleanJSON), &parsedJSON); err == nil {
				res["structured_output"] = parsedJSON
			}
		}

		return toolResult(res), nil
	}

	return toolResult(map[string]any{
		"status":                    "spawned",
		"subagent_id":               subID,
		"selected_subagent":         assignedName,
		"role":                      assignedRole,
		"task":                      task,
		"context_garbage_collected": true,
		"message":                   fmt.Sprintf("Sub-agent %q (%s) spawned in isolated context. Retrieve garbage-collected output with get_subagent_output.", assignedName, assignedRole),
	}), nil
}

// selectSubagentTool asks Jev to recommend which sub-agent is best for a given task without spawning it yet.
func selectSubagentTool() ToolSpec {
	return ToolSpec{
		Name:        "select_subagent",
		Cost:        "cheap",
		Description: "Ask Jev to analyze a task and recommend the optimal sub-agent role or specialist from all configured sub-agents without spawning it yet.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task": map[string]any{
					"type":        "string",
					"description": "The task or subtask to delegate to a sub-agent",
				},
			},
			"required": []string{"task"},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			bridge := SessionBridgeFrom(ctx)
			if bridge == nil {
				return nil, fmt.Errorf("select_subagent is unavailable outside an agent session")
			}
			task := strings.TrimSpace(argString(args, "task"))
			if task == "" {
				return nil, fmt.Errorf("task parameter is required")
			}
			res, err := bridge.SelectSubagent(task)
			if err != nil {
				return nil, err
			}
			return toolResult(res), nil
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

// backgroundShellTool starts a command in the background (dev server, vite dev, build watcher).
func backgroundShellTool() ToolSpec {
	return ToolSpec{
		Name: "background_shell",
		Cost: "high",
		Description: "Run a shell command as a detached background task (e.g. dev servers like 'vite', 'npm run dev', watchers, background builds, or test suites). " +
			"The task is registered into the user's floating task bar, where they can view live output, stop it, or open an interactive terminal session in the right sidebar. " +
			"Returns task_id immediately. You can check logs with task_output, stop it with task_stop, or schedule a cron check with schedule.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{"type": "string", "description": "Shell command to run in the background (e.g. 'npm run dev', 'vite', 'cargo build', 'go test ./...')"},
				"cwd":     map[string]any{"type": "string", "description": "Working directory (defaults to workspace root)"},
				"label":   map[string]any{"type": "string", "description": "Optional friendly label (e.g. 'Vite Dev Server')"},
			},
			"required": []string{"command"},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			command := argString(args, "command")
			if command == "" {
				command = argCommand(args)
			}
			if command == "" {
				return nil, fmt.Errorf("command is required")
			}
			cwd := argString(args, "cwd")
			if cwd == "" {
				cwd = defaultCwd(ctx)
			}
			sessionID := ""
			if bridge := SessionBridgeFrom(ctx); bridge != nil {
				sessionID = bridge.SessionID()
			}
			task, err := StartBackgroundCommand(sessionID, command, cwd)
			if err != nil {
				return nil, err
			}
			return toolResult(map[string]any{
				"task_id": task.ID,
				"command": task.Command,
				"status":  string(task.Status),
				"message": "Background task started and pinned to floating task bar. User can monitor logs or open a shell in the right sidebar. Use schedule to schedule a follow-up check after X seconds if needed.",
			}), nil
		},
	}
}

// scheduleTool allows the agent to set a one-shot timer or recurring cron check.
func scheduleTool() ToolSpec {
	return ToolSpec{
		Name: "schedule",
		Cost: "cheap",
		Description: "Schedule a one-shot timer or recurring cron check (like Antigravity scheduler). " +
			"For example, if a background build, server startup, or test suite takes time, you can schedule a check (e.g. 60 seconds) with duration_seconds: 60. " +
			"When the duration expires, you will receive an automatic wake-up alert with your prompt and task status, allowing you to check the response with task_output or report status to the user. " +
			"Also supports standard 5-field cron expressions (e.g. '*/1 * * * *').",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"duration_seconds": map[string]any{"type": "integer", "description": "Seconds to wait before triggering the wake-up check (e.g. 30, 60, 120)"},
				"prompt":           map[string]any{"type": "string", "description": "Instruction or reminder for yourself when the timer fires (e.g. 'Check if build task_id has finished using task_output')"},
				"task_id":          map[string]any{"type": "string", "description": "Optional background task_id associated with this schedule"},
				"cron_expression":  map[string]any{"type": "string", "description": "Optional 5-field cron expression for recurring schedule (e.g. '*/2 * * * *')"},
			},
			"required": []string{"prompt"},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			bridge := SessionBridgeFrom(ctx)
			if bridge == nil {
				return nil, fmt.Errorf("schedule is unavailable outside an agent session")
			}
			prompt := argString(args, "prompt")
			if prompt == "" {
				return nil, fmt.Errorf("prompt is required")
			}
			durationSec := argInt(args, "duration_seconds", 0)
			cronExpr := argString(args, "cron_expression")
			taskID := resolveTaskID(args)
			if taskID == "" {
				for _, t := range ListTasks("") {
					if t.Status == TaskRunning {
						taskID = t.ID
						break
					}
				}
			}

			if durationSec <= 0 && cronExpr == "" {
				durationSec = 60
			}

			scheduleID, err := bridge.ScheduleTimer(time.Duration(durationSec)*time.Second, cronExpr, prompt, taskID)
			if err != nil {
				return nil, err
			}
			msg := fmt.Sprintf("Schedule created (id: %s). ", scheduleID)
			if durationSec > 0 {
				msg += fmt.Sprintf("Timer set for %d seconds. You will be notified automatically with your prompt once time elapses.", durationSec)
			} else {
				msg += fmt.Sprintf("Recurring cron set: %s.", cronExpr)
			}
			return toolResult(map[string]any{
				"schedule_id":      scheduleID,
				"duration_seconds": durationSec,
				"cron_expression":  cronExpr,
				"task_id":          taskID,
				"prompt":           prompt,
				"message":          msg,
			}), nil
		},
	}
}

// cancelScheduleTool cancels an active schedule.
func cancelScheduleTool() ToolSpec {
	return ToolSpec{
		Name:        "cancel_schedule",
		Cost:        "cheap",
		Description: "Cancel an active schedule or timer created with schedule.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"schedule_id": map[string]any{"type": "string", "description": "ID of the schedule to cancel"},
			},
			"required": []string{"schedule_id"},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			bridge := SessionBridgeFrom(ctx)
			if bridge == nil {
				return nil, fmt.Errorf("cancel_schedule is unavailable outside an agent session")
			}
			schedID := strings.TrimSpace(argString(args, "schedule_id"))
			if schedID == "" {
				return nil, fmt.Errorf("schedule_id is required")
			}
			ok := bridge.CancelSchedule(schedID)
			return toolResult(map[string]any{
				"schedule_id": schedID,
				"cancelled":   ok,
				"message":     fmt.Sprintf("Schedule %s cancelled: %v", schedID, ok),
			}), nil
		},
	}
}

// listSchedulesTool lists all active schedules and timers.
func listSchedulesTool() ToolSpec {
	return ToolSpec{
		Name:        "list_schedules",
		Cost:        "cheap",
		Description: "List all active schedules and timers for the current session.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			bridge := SessionBridgeFrom(ctx)
			if bridge == nil {
				return nil, fmt.Errorf("list_schedules is unavailable outside an agent session")
			}
			schedules := bridge.ListSchedules()
			return toolResult(map[string]any{
				"schedules": schedules,
				"count":     len(schedules),
			}), nil
		},
	}
}
