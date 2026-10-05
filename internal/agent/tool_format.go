package agent

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Tool results are returned to the model as plain, purpose-shaped text (the
// harness formatModelContent concept) instead of raw JSON blobs: a successful
// edit reads as confirmation ("file state is current…"), a bash result reads
// as terminal output. The structured map still travels to the UI via the
// tool_end event's display payload.

// maxToolModelBytes head-truncates any formatted tool result fed to the model.
const maxToolModelBytes = 100 * 1024

// normalizeToolResult converts a handler's result map to its JSON shape.
// Tool handlers return native Go types ([]map[string]any, int, …) while the
// formatter (and the frontend) speak JSON types ([]any, float64) — without
// this round-trip every failed type assertion silently renders a real result
// as empty, which the model then reports as "the folder is empty".
func normalizeToolResult(res any) any {
	b, err := json.Marshal(res)
	if err != nil {
		return res
	}
	var m map[string]any
	if json.Unmarshal(b, &m) != nil || m == nil {
		return res
	}
	return m
}

// formatToolModelContent renders a successful tool result as model-facing
// text. The raw map is returned unchanged for the UI display payload.
func formatToolModelContent(name string, res any) string {
	m, ok := res.(map[string]any)
	if !ok {
		return formatFallbackModelContent(res)
	}

	switch name {
	case "edit", "edit_file", "replace_file_content":
		path := strAny(m["path"])
		repl, _ := m["replacements"].(float64)
		var sb strings.Builder
		fmt.Fprintf(&sb, "The file %s has been updated successfully.", path)
		if b, _ := m["replace_all"].(bool); b && repl > 1 {
			sb.WriteString(" All occurrences were successfully replaced.")
		}
		sb.WriteString(" (file state is current in your context — no need to Read it back)")
		return sb.String()

	case "write", "write_file", "create_file":
		path := strAny(m["path"])
		if strAny(m["status"]) == "written" {
			return fmt.Sprintf("File created successfully at: %s (file state is current in your context — no need to Read it back)", path)
		}
		return fmt.Sprintf("The file %s has been updated successfully. (file state is current in your context — no need to Read it back)", path)

	case "read", "read_file", "view_file", "cat":
		if typ := strAny(m["type"]); typ == "dir" {
			return formatDirEntries(m)
		}
		content := strAny(m["content"])
		total, _ := m["total_lines"].(float64)
		header := fmt.Sprintf("Contents of %s", strAny(m["path"]))
		if total > 0 {
			header = fmt.Sprintf("%s (%d total lines)", header, int(total))
		}
		return header + ":\n" + content

	case "read_multiple", "read_directory_files":
		files, _ := m["files"].(map[string]any)
		var sb strings.Builder
		for p, c := range files {
			sb.WriteString(fmt.Sprintf("=== %s ===\n%s\n\n", p, strAny(c)))
		}
		return strings.TrimSpace(sb.String())

	case "bash", "run_shell", "exec", "run_command":
		out := strAny(m["stdout"])
		errOut := strAny(m["stderr"])
		code, _ := m["exit_code"].(float64)
		var sb strings.Builder
		if strings.TrimSpace(out) != "" {
			sb.WriteString(strings.TrimRight(out, "\n"))
		}
		if strings.TrimSpace(errOut) != "" {
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString("STDERR:\n" + strings.TrimRight(errOut, "\n"))
		}
		if sb.Len() == 0 {
			sb.WriteString("(no output)")
		}
		if code != 0 {
			fmt.Fprintf(&sb, "\nExit code: %d", int(code))
		}
		return sb.String()

	case "search", "search_workspace", "rg", "grep":
		matches, _ := m["matches"].([]any)
		var sb strings.Builder
		fmt.Fprintf(&sb, "Found %d matches:\n", len(matches))
		for _, mt := range matches {
			mm, _ := mt.(map[string]any)
			fmt.Fprintf(&sb, "%s:%v: %s\n", strAny(mm["path"]), mm["line"], strings.TrimRight(strAny(mm["content"]), "\n"))
		}
		return strings.TrimRight(sb.String(), "\n")

	case "find", "glob":
		matches, _ := m["matches"].([]any)
		var sb strings.Builder
		fmt.Fprintf(&sb, "Found %d matches:\n", len(matches))
		for _, mt := range matches {
			mm, _ := mt.(map[string]any)
			sb.WriteString(strAny(mm["path"]) + "\n")
		}
		return strings.TrimRight(sb.String(), "\n")

	case "git_status":
		return strAny(m["status"])

	default:
		return formatFallbackModelContent(res)
	}
}

func formatDirEntries(m map[string]any) string {
	entries, _ := m["entries"].([]any)
	var sb strings.Builder
	fmt.Fprintf(&sb, "Directory %s (%d entries):\n", strAny(m["path"]), len(entries))
	for _, e := range entries {
		em, _ := e.(map[string]any)
		name := strAny(em["name"])
		if b, _ := em["is_dir"].(bool); b {
			name += "/"
		}
		sb.WriteString("  " + name + "\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

func formatFallbackModelContent(res any) string {
	b, err := json.Marshal(res)
	if err != nil {
		return fmt.Sprintf("%v", res)
	}
	return string(b)
}

func strAny(v any) string {
	s, _ := v.(string)
	return s
}

// truncateModelText head-truncates oversized tool output with an explicit note.
func truncateModelText(s string) string {
	if len(s) <= maxToolModelBytes {
		return s
	}
	return s[:maxToolModelBytes] + fmt.Sprintf("\n\n[Tool output truncated: originalBytes=%d, maxModelBytes=%d, strategy=truncate]", len(s), maxToolModelBytes)
}
