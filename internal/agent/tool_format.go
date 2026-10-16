package agent

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Haslab-dev/RakitKode/llm"
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

	case "load_skill", "skill", "call_skill", "run_skill", "invoke_skill":
		name := strAny(m["name"])
		desc := strAny(m["description"])
		dir := strAny(m["base_dir"])
		body := strAny(m["body"])
		var sb strings.Builder
		fmt.Fprintf(&sb, "Skill %q loaded successfully.\nDescription: %s\nDirectory: %s\n\nInstructions (SKILL.md):\n%s", name, desc, dir, body)
		return sb.String()

	case "python_eval", "python", "python_repl":
		out := strAny(m["stdout"])
		errOut := strAny(m["stderr"])
		resVal := strAny(m["result"])
		var sb strings.Builder
		if strings.TrimSpace(out) != "" {
			sb.WriteString(strings.TrimRight(out, "\n"))
		}
		if strings.TrimSpace(resVal) != "" && resVal != out && resVal != "(evaluated to None)" {
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(strings.TrimRight(resVal, "\n"))
		}
		if strings.TrimSpace(errOut) != "" {
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString("STDERR:\n" + strings.TrimRight(errOut, "\n"))
		}
		if sb.Len() == 0 {
			sb.WriteString("(evaluated to None / no output)")
		}
		return sb.String()

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

// FlattenToolDefinitions normalizes all tool parameter schemas in-place for model tolerance.
func FlattenToolDefinitions(defs []llm.ToolDefinition) []llm.ToolDefinition {
	if len(defs) == 0 {
		return defs
	}
	out := make([]llm.ToolDefinition, len(defs))
	for i, d := range defs {
		out[i] = d
		if d.Function.Parameters != nil {
			out[i].Function.Parameters = FlattenToolSchema(d.Function.Parameters)
		}
	}
	return out
}

// FlattenToolSchema inlines $defs, resolves local $ref pointers, and flattens allOf
// structures so that models (Anthropic, DeepSeek, Ollama, etc.) receive clean, flattened
// JSON schemas without complex indirection.
func FlattenToolSchema(schema map[string]any) map[string]any {
	if schema == nil {
		return nil
	}
	copied := deepCopyMap(schema)
	defs := extractDefs(copied)
	flattenMap(copied, defs, make(map[string]bool))
	if !hasDanglingRefs(copied) {
		delete(copied, "$defs")
		delete(copied, "definitions")
	}
	return copied
}

func extractDefs(m map[string]any) map[string]any {
	defs := make(map[string]any)
	if d, ok := m["$defs"].(map[string]any); ok {
		for k, v := range d {
			defs[k] = v
		}
	}
	if d, ok := m["definitions"].(map[string]any); ok {
		for k, v := range d {
			defs[k] = v
		}
	}
	return defs
}

func deepCopyMap(m map[string]any) map[string]any {
	data, err := json.Marshal(m)
	if err != nil {
		return m
	}
	var c map[string]any
	if err := json.Unmarshal(data, &c); err != nil {
		return m
	}
	return c
}

func flattenMap(m map[string]any, defs map[string]any, seenRefs map[string]bool) {
	if m == nil {
		return
	}

	// 1. Handle allOf
	if allOf, ok := m["allOf"].([]any); ok {
		props, _ := m["properties"].(map[string]any)
		if props == nil {
			props = make(map[string]any)
			m["properties"] = props
		}
		var reqList []string
		if r, ok := m["required"].([]any); ok {
			for _, item := range r {
				if s, ok := item.(string); ok {
					reqList = append(reqList, s)
				}
			}
		}

		for _, item := range allOf {
			if subMap, ok := item.(map[string]any); ok {
				flattenMap(subMap, defs, seenRefs)
				if subProps, ok := subMap["properties"].(map[string]any); ok {
					for pk, pv := range subProps {
						props[pk] = pv
					}
				}
				if subReq, ok := subMap["required"].([]any); ok {
					for _, reqItem := range subReq {
						if s, ok := reqItem.(string); ok {
							reqList = append(reqList, s)
						}
					}
				}
			}
		}
		delete(m, "allOf")
		if len(reqList) > 0 {
			m["required"] = uniqueStrings(reqList)
		}
	}

	// 2. Inline $ref if local
	if refVal, ok := m["$ref"].(string); ok {
		refName := parseLocalRef(refVal)
		if refName != "" && defs != nil && !seenRefs[refName] {
			if target, ok := defs[refName].(map[string]any); ok {
				seenRefs[refName] = true
				targetCopy := deepCopyMap(target)
				flattenMap(targetCopy, defs, seenRefs)
				delete(m, "$ref")
				for k, v := range targetCopy {
					if _, exists := m[k]; !exists {
						m[k] = v
					}
				}
			}
		}
	}

	// 3. Recurse into properties
	if props, ok := m["properties"].(map[string]any); ok {
		for _, propVal := range props {
			if propMap, ok := propVal.(map[string]any); ok {
				flattenMap(propMap, defs, seenRefs)
			}
		}
	}

	// 4. Recurse into items (for array types)
	if items, ok := m["items"].(map[string]any); ok {
		flattenMap(items, defs, seenRefs)
	}
}

func parseLocalRef(ref string) string {
	if strings.HasPrefix(ref, "#/$defs/") {
		return strings.TrimPrefix(ref, "#/$defs/")
	}
	if strings.HasPrefix(ref, "#/definitions/") {
		return strings.TrimPrefix(ref, "#/definitions/")
	}
	return ""
}

func hasDanglingRefs(val any) bool {
	switch v := val.(type) {
	case map[string]any:
		if ref, ok := v["$ref"].(string); ok && strings.HasPrefix(ref, "#/") {
			return true
		}
		for _, sub := range v {
			if hasDanglingRefs(sub) {
				return true
			}
		}
	case []any:
		for _, sub := range v {
			if hasDanglingRefs(sub) {
				return true
			}
		}
	}
	return false
}

func uniqueStrings(strs []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, s := range strs {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
