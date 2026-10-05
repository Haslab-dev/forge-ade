package tools

import (
	"context"
	"fmt"
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
			taskID := argString(args, "task_id")
			if taskID == "" {
				return nil, fmt.Errorf("task_id is required")
			}
			if _, ok := GetTask(taskID); !ok {
				return nil, fmt.Errorf("unknown task_id %s (it may have already been consumed; check task_list)", taskID)
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
				return nil, fmt.Errorf("unknown task_id %s", taskID)
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
			taskID := argString(args, "task_id")
			if taskID == "" {
				return nil, fmt.Errorf("task_id is required")
			}
			task, ok := GetTask(taskID)
			if !ok {
				return nil, fmt.Errorf("unknown task_id %s", taskID)
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
