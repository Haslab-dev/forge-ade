package agent

import (
	"fmt"
	"strings"

	"github.com/hasdev/forge-ade/internal/tools"
)

// notificationTailCap bounds each task's output tail injected into the model
// context; the full stream stays on disk (task.output_file).
const notificationTailCap = 2000

// formatTaskNotifications renders drained background-task completions as a
// <task-notification> block (port of ZCode's notification envelope).
func formatTaskNotifications(notes []tools.TaskNotification) string {
	var sb strings.Builder
	sb.WriteString("<task-notification>")
	for _, n := range notes {
		status := string(n.Status)
		detail := fmt.Sprintf("\n- Task %s finished: status=%s", n.TaskID, status)
		if n.Status != tools.TaskStopped {
			detail += fmt.Sprintf(", exit_code=%d", n.ExitCode)
		}
		detail += "\n  command: " + n.Command
		if tail := strings.TrimSpace(n.Tail); tail != "" {
			if len(tail) > notificationTailCap {
				tail = tail[len(tail)-notificationTailCap:]
			}
			detail += "\n  output tail:\n    " + strings.ReplaceAll(tail, "\n", "\n    ")
		}
		detail += "\n  (use task_output with this task_id for the full output if you need it)"
		sb.WriteString(detail)
	}
	sb.WriteString("\n</task-notification>")
	return sb.String()
}
