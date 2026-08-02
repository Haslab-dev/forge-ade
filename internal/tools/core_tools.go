package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Session bridge — lets tools like `todo` and `ask` touch agent session state
// without importing the agent package (avoids an import cycle).
// ---------------------------------------------------------------------------

// TodoItem mirrors the agent session's task list in a tools-friendly shape.
type TodoItem struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Status  string `json:"status"` // pending | in_progress | completed | blocked | abandoned
	Phase   string `json:"phase,omitempty"`
	Blocker string `json:"blocker,omitempty"`
}

// AskQuestion is a structured question the `ask` tool can pause the agent for.
type AskQuestion struct {
	ID          string   `json:"id"`
	Question    string   `json:"question"`
	Header      string   `json:"header,omitempty"`
	Options     []string `json:"options"`
	Multi       bool     `json:"multi,omitempty"`
	Recommended int      `json:"recommended,omitempty"`
}

// SessionBridge is implemented by the agent manager and handed to tools via
// the context so `todo` / `ask` can interact with the live session.
type SessionBridge interface {
	GetTodos() []TodoItem
	SetTodos(items []TodoItem)
	Ask(questions []AskQuestion) error
}

type bridgeCtxKey int

const (
	bridgeKey bridgeCtxKey = iota
)

// WithSessionBridge attaches a session bridge to the context.
func WithSessionBridge(ctx context.Context, bridge SessionBridge) context.Context {
	return context.WithValue(ctx, bridgeKey, bridge)
}

// SessionBridgeFrom returns the session bridge from the context, if any.
func SessionBridgeFrom(ctx context.Context) SessionBridge {
	if ctx == nil {
		return nil
	}
	b, _ := ctx.Value(bridgeKey).(SessionBridge)
	return b
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

// toolResult formats a tool response for the LLM.
func toolResult(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{"result": v}
}

func argString(args map[string]any, key string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}

func argInt(args map[string]any, key string, def int) int {
	switch v := args[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return def
}

func argBool(args map[string]any, key string, def bool) bool {
	if v, ok := args[key].(bool); ok {
		return v
	}
	return def
}

// ---------------------------------------------------------------------------
// Core tool registrations
// ---------------------------------------------------------------------------

// doublestarMatch matches a pattern containing `**` against a relative path.
// `**` matches any number of path segments (including zero); other glob
// segments use filepath.Match semantics. Converted to a regex under the hood
// so recursion and zero-length `**/` prefixes behave like real doublestar.
func doublestarMatch(pattern, rel string) (bool, error) {
	pattern = filepath.ToSlash(pattern)
	rel = filepath.ToSlash(rel)
	if !strings.Contains(pattern, "**") {
		return filepath.Match(pattern, rel)
	}
	var sb strings.Builder
	sb.WriteString("^")
	parts := strings.Split(pattern, "**")
	for i, part := range parts {
		if i > 0 {
			sb.WriteString("(?:.*/)?") // ** spans zero or more segments
		}
		if part == "" || part == "/" {
			continue
		}
		seg := strings.Trim(part, "/")
		re, err := globSegmentToRegex(seg)
		if err != nil {
			return false, err
		}
		sb.WriteString(re)
	}
	sb.WriteString("$")
	matched, err := regexp.MatchString(sb.String(), rel)
	return matched, err
}

// globSegmentToRegex converts a glob segment (may contain `*`, `?`, `[...]`)
// into a regex fragment that matches within a path.
func globSegmentToRegex(seg string) (string, error) {
	var sb strings.Builder
	for i := 0; i < len(seg); i++ {
		c := seg[i]
		switch c {
		case '*':
			sb.WriteString("[^/]*")
		case '?':
			sb.WriteString("[^/]")
		case '[':
			// Copy the character class verbatim (handle leading !/^).
			j := i + 1
			if j < len(seg) && (seg[j] == '!' || seg[j] == '^') {
				j++
			}
			if j < len(seg) && seg[j] == ']' {
				j++
			}
			for j < len(seg) && seg[j] != ']' {
				j++
			}
			if j >= len(seg) {
				return "", fmt.Errorf("malformed character class")
			}
			cls := seg[i : j+1]
			sb.WriteString(cls)
			i = j
		case '.', '(', ')', '+', '|', '^', '$', '{', '}', '\\':
			sb.WriteByte('\\')
			sb.WriteByte(c)
		default:
			sb.WriteByte(c)
		}
	}
	return sb.String(), nil
}

// git_status — get the repository status (kept from the old surface).
func gitStatusTool() ToolSpec {
	return ToolSpec{
		Name:        "git_status",
		Description: "Get git repository status output (porcelain v2).",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"dir": map[string]any{"type": "string", "description": "Repository directory path"},
			},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			dir := argString(args, "dir")
			if dir == "" {
				dir, _ = os.Getwd()
			}
			cmd := exec.CommandContext(ctx, "git", "status", "--porcelain=v2")
			cmd.Dir = dir
			out, err := cmd.CombinedOutput()
			if err != nil {
				return nil, fmt.Errorf("git status error: %w", err)
			}
			return toolResult(map[string]any{"status": string(out)}), nil
		},
	}
}

// registerCoreTools registers the canonical tool surface as the primary tool
// names: read, write, edit, bash, search, find, glob, todo, ask.
func (r *Registry) registerCoreTools(searchMgr searchAPI) {
	r.Register(readTool())
	r.Register(writeTool())
	r.Register(editTool())
	r.Register(bashTool())
	r.Register(searchTool(searchMgr))
	r.Register(findTool())
	r.Register(globTool())
	r.Register(todoTool())
	r.Register(askTool())
	r.Register(gitStatusTool())
}

// read — files, directories, and globs through one path.
func readTool() ToolSpec {
	return ToolSpec{
		Name:        "read",
		Description: "Read a file, directory, or glob match. Use start_line/end_line to read a range, or omit to read the whole file. Directories return their entries.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":       map[string]any{"type": "string", "description": "File path, directory path, or glob pattern"},
				"start_line": map[string]any{"type": "integer", "description": "1-based start line (optional)"},
				"end_line":   map[string]any{"type": "integer", "description": "1-based end line, inclusive (optional)"},
			},
			"required": []string{"path"},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			path := argString(args, "path")
			if path == "" {
				return nil, fmt.Errorf("path is required")
			}
			info, err := os.Stat(path)
			if err == nil && info.IsDir() {
				entries, err := os.ReadDir(path)
				if err != nil {
					return nil, fmt.Errorf("read dir %s: %w", path, err)
				}
				var items []map[string]any
				for _, e := range entries {
					ei, _ := e.Info()
					sz := int64(0)
					if ei != nil {
						sz = ei.Size()
					}
					items = append(items, map[string]any{"name": e.Name(), "is_dir": e.IsDir(), "size": sz})
				}
				return toolResult(map[string]any{"path": path, "type": "dir", "entries": items, "count": len(items)}), nil
			}

			data, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("failed to read %s: %w", path, err)
			}
			content := string(data)
			totalLines := 1
			if strings.Count(content, "\n") > 0 {
				totalLines = strings.Count(content, "\n") + (1 - boolInt(strings.HasSuffix(content, "\n")))
			}
			start := argInt(args, "start_line", 0)
			end := argInt(args, "end_line", 0)
			selected := content
			if start > 0 || end > 0 {
				lines := strings.Split(content, "\n")
				if start < 1 {
					start = 1
				}
				if end < 1 || end > len(lines) {
					end = len(lines)
				}
				if start <= end {
					selected = strings.Join(lines[start-1:end], "\n")
				}
			}
			return toolResult(map[string]any{
				"path":        path,
				"type":        "file",
				"total_lines": totalLines,
				"content":     selected,
			}), nil
		},
	}
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// write — create or overwrite a file.
func writeTool() ToolSpec {
	return ToolSpec{
		Name:        "write",
		Description: "Create or overwrite a file with the given content, creating parent directories if needed.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":    map[string]any{"type": "string", "description": "Target file path"},
				"content": map[string]any{"type": "string", "description": "Full file content"},
			},
			"required": []string{"path", "content"},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			path := argString(args, "path")
			content := argString(args, "content")
			if path == "" {
				return nil, fmt.Errorf("path is required")
			}
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return nil, fmt.Errorf("failed to create directory: %w", err)
			}
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				return nil, fmt.Errorf("failed to write file: %w", err)
			}
			return toolResult(map[string]any{"path": path, "status": "written"}), nil
		},
	}
}

// edit — targeted string replacement in a file
// hashline `edit`: replace an exact anchor with new text, report the count).
func editTool() ToolSpec {
	return ToolSpec{
		Name:        "edit",
		Description: "Make a targeted edit to a file: replace the exact `old` string with `new`. `old` must appear in the file or the edit fails (no silent no-op).",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string", "description": "File to edit"},
				"old":  map[string]any{"type": "string", "description": "Exact text to find (must match)"},
				"new":  map[string]any{"type": "string", "description": "Replacement text"},
			},
			"required": []string{"path", "old", "new"},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			path := argString(args, "path")
			old := argString(args, "old")
			new := argString(args, "new")
			if path == "" || old == "" {
				return nil, fmt.Errorf("path and old are required")
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("failed to read %s: %w", path, err)
			}
			content := string(data)
			count := strings.Count(content, old)
			if count == 0 {
				return nil, fmt.Errorf("edit failed: the anchor text was not found in %s", path)
			}
			updated := strings.ReplaceAll(content, old, new)
			if err := os.WriteFile(path, []byte(updated), 0644); err != nil {
				return nil, fmt.Errorf("failed to write %s: %w", path, err)
			}
			return toolResult(map[string]any{"path": path, "replacements": count, "status": "edited"}), nil
		},
	}
}

// bash — run a workspace shell command.
func bashTool() ToolSpec {
	return ToolSpec{
		Name:        "bash",
		Description: "Run a shell command in the workspace and capture stdout/stderr/exit code.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{"type": "string", "description": "Shell command to run"},
				"cwd":     map[string]any{"type": "string", "description": "Working directory (defaults to workspace root)"},
				"timeout": map[string]any{"type": "integer", "description": "Timeout in seconds (default 45)"},
			},
			"required": []string{"command"},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			command := argString(args, "command")
			cwd := argString(args, "cwd")
			if command == "" {
				return nil, fmt.Errorf("command is required")
			}
			timeoutSec := argInt(args, "timeout", 45)
			if timeoutSec <= 0 {
				timeoutSec = 45
			}
			cmdCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
			defer cancel()

			cmd := exec.CommandContext(cmdCtx, "/bin/zsh", "-l", "-c", command)
			if cwd != "" {
				cmd.Dir = cwd
			}
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			err := cmd.Run()
			exitCode := 0
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					exitCode = exitErr.ExitCode()
				} else {
					exitCode = 1
				}
			}
			return toolResult(map[string]any{
				"stdout":    stdout.String(),
				"stderr":    stderr.String(),
				"exit_code": exitCode,
			}), nil
		},
	}
}

// searchAPI is the subset of the search manager the search tool needs.
type searchAPI interface {
	SearchContentWithOptions(opts searchOptions) ([]searchResult, error)
	SearchFilenameWithOptions(opts searchOptions) []searchResult
}

// searchOptions / searchResult mirror the search package's public types so the
// tools package does not need to import it.
type searchOptions struct {
	Query         string
	Limit         int
	MatchCase     bool
	MatchWholeWord bool
	UseRegex      bool
	Path          string
}

type searchResult struct {
	Path     string
	Filename string
	Score    float64
	Line     int
	Content  string
}

// search — regex over files.
func searchTool(sm searchAPI) ToolSpec {
	return ToolSpec{
		Name:        "search",
		Description: "Search file contents for a pattern (regex or plain text) across the workspace, returning file:line matches.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": map[string]any{"type": "string", "description": "Regex or text pattern to search"},
				"path":    map[string]any{"type": "string", "description": "Optional directory to scope the search to"},
				"regex":   map[string]any{"type": "boolean", "description": "Treat pattern as regex (default true)"},
				"case_sensitive": map[string]any{"type": "boolean", "description": "Case-sensitive match (default false)"},
				"limit":   map[string]any{"type": "integer", "description": "Max results (default 50)"},
			},
			"required": []string{"pattern"},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			pattern := argString(args, "pattern")
			if pattern == "" {
				return nil, fmt.Errorf("pattern is required")
			}
			if sm == nil {
				return nil, fmt.Errorf("search is unavailable")
			}
			res, err := sm.SearchContentWithOptions(searchOptions{
				Query:         pattern,
				Limit:         argInt(args, "limit", 50),
				MatchCase:     argBool(args, "case_sensitive", false),
				UseRegex:      argBool(args, "regex", true),
				Path:          argString(args, "path"),
			})
			if err != nil {
				return nil, err
			}
			var matches []map[string]any
			for _, r := range res {
				matches = append(matches, map[string]any{
					"path":    r.Path,
					"line":    r.Line,
					"content": r.Content,
				})
			}
			return toolResult(map[string]any{"pattern": pattern, "matches": matches, "count": len(matches)}), nil
		},
	}
}

// find — glob-based path lookup.
func findTool() ToolSpec {
	return ToolSpec{
		Name:        "find",
		Description: "Find files and directories matching a glob pattern (e.g. src/**/*.ts, **/*_test.go). Returns matching paths.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":   map[string]any{"type": "string", "description": "Glob pattern to match"},
				"cwd":    map[string]any{"type": "string", "description": "Base directory (defaults to workspace root)"},
				"limit":  map[string]any{"type": "integer", "description": "Max results (default 200)"},
			},
			"required": []string{"path"},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			pattern := argString(args, "path")
			if pattern == "" {
				return nil, fmt.Errorf("path is required")
			}
			cwd := argString(args, "cwd")
			if cwd == "" {
				cwd, _ = os.Getwd()
			}
			limit := argInt(args, "limit", 200)

			// Go's filepath.Glob does not support `**` recursion; walk the tree
			// and match the pattern against relative paths so `**/*.txt` works.
			var matches []string
			if strings.Contains(pattern, "**") {
				_ = filepath.WalkDir(cwd, func(path string, d os.DirEntry, err error) error {
					if err != nil {
						return nil
					}
					rel, rerr := filepath.Rel(cwd, path)
					if rerr != nil {
						return nil
					}
					if rel == "." {
						return nil
					}
					ok, merr := doublestarMatch(pattern, rel)
					if merr == nil && ok {
						matches = append(matches, path)
					}
					if len(matches) >= limit {
						return filepath.SkipAll
					}
					return nil
				})
			} else {
				full := pattern
				if !filepath.IsAbs(pattern) {
					full = filepath.Join(cwd, pattern)
				}
				m, err := filepath.Glob(full)
				if err != nil {
					return nil, fmt.Errorf("glob: %w", err)
				}
				matches = m
			}
			sort.Strings(matches)
			if len(matches) > limit {
				matches = matches[:limit]
			}
			var items []map[string]any
			for _, m := range matches {
				info, err := os.Stat(m)
				if err != nil {
					continue
				}
				items = append(items, map[string]any{"path": m, "is_dir": info.IsDir()})
			}
			return toolResult(map[string]any{"pattern": pattern, "matches": items, "count": len(items)}), nil
		},
	}
}

// glob — real glob pattern matching (kept from the old surface, now canonical).
func globTool() ToolSpec {
	return ToolSpec{
		Name:        "glob",
		Description: "Find files matching a glob pattern (e.g. **/*.go, src/*.tsx).",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": map[string]any{"type": "string", "description": "Glob pattern to match"},
				"cwd":     map[string]any{"type": "string", "description": "Base directory (defaults to workspace root)"},
			},
			"required": []string{"pattern"},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			pattern := argString(args, "pattern")
			if pattern == "" {
				return nil, fmt.Errorf("pattern is required")
			}
			cwd := argString(args, "cwd")
			if cwd == "" {
				cwd, _ = os.Getwd()
			}
			full := pattern
			if !filepath.IsAbs(pattern) {
				full = filepath.Join(cwd, pattern)
			}
			matches, err := filepath.Glob(full)
			if err != nil {
				return nil, fmt.Errorf("glob: %w", err)
			}
			var items []map[string]any
			for _, m := range matches {
				info, err := os.Stat(m)
				if err != nil {
					continue
				}
				items = append(items, map[string]any{"path": m, "is_dir": info.IsDir(), "size": info.Size()})
			}
			return toolResult(map[string]any{"pattern": pattern, "matches": items, "count": len(items)}), nil
		},
	}
}

// todo — ordered mutations over the session todo list.
// Ops: init, start, done, drop, block, unblock, rm, append, view.
func todoTool() ToolSpec {
	return ToolSpec{
		Name:        "todo",
		Description: "Manage the session todo list. Ops: init {list:[...]}, append {phase, items:[...]}, start {task|phase}, done {task|phase}, drop, block {task, reason}, unblock {task}, rm {task|phase}, view. Tasks carry phase + status (pending/in_progress/completed/blocked/abandoned).",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"op":     map[string]any{"type": "string", "description": "init | append | start | done | drop | block | unblock | rm | view"},
				"list":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "For init: initial task titles"},
				"items":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "For append: task titles"},
				"phase":  map[string]any{"type": "string", "description": "Phase name"},
				"task":   map[string]any{"type": "string", "description": "Task title to target"},
				"reason": map[string]any{"type": "string", "description": "Blocker reason (block op)"},
			},
			"required": []string{"op"},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			bridge := SessionBridgeFrom(ctx)
			if bridge == nil {
				return nil, fmt.Errorf("todo is unavailable outside an agent session")
			}
			op := argString(args, "op")
			todos := bridge.GetTodos()
			errors := []string{}

			switch op {
			case "init":
				list, _ := args["list"].([]any)
				var titles []string
				for _, l := range list {
					if s, ok := l.(string); ok {
						titles = append(titles, s)
					}
				}
				todos = todos[:0]
				for i, t := range titles {
					todos = append(todos, TodoItem{ID: fmt.Sprintf("t%d", i+1), Title: t, Status: "pending"})
				}
			case "append":
				phase := argString(args, "phase")
				items, _ := args["items"].([]any)
				if phase == "" {
					errors = append(errors, "append requires a phase")
				}
				if len(items) == 0 {
					errors = append(errors, "append requires items")
				}
				nextID := len(todos) + 1
				for _, it := range items {
					if s, ok := it.(string); ok {
						todos = append(todos, TodoItem{ID: fmt.Sprintf("t%d", nextID), Title: s, Status: "pending", Phase: phase})
						nextID++
					}
				}
			case "start":
				target := argString(args, "task")
				updated := false
				for i := range todos {
					if todos[i].Title == target || todos[i].ID == target {
						// clear other in_progress
						for j := range todos {
							if todos[j].Status == "in_progress" {
								todos[j].Status = "pending"
							}
						}
						todos[i].Status = "in_progress"
						updated = true
						break
					}
				}
				if !updated {
					errors = append(errors, "task not found: "+target)
				}
			case "done":
				target := argString(args, "task")
				phase := argString(args, "phase")
				for i := range todos {
					if (target != "" && (todos[i].Title == target || todos[i].ID == target)) ||
						(phase != "" && todos[i].Phase == phase) {
						todos[i].Status = "completed"
					}
				}
			case "drop":
				target := argString(args, "task")
				for i := range todos {
					if todos[i].Title == target || todos[i].ID == target {
						todos[i].Status = "abandoned"
					}
				}
			case "block":
				target := argString(args, "task")
				reason := argString(args, "reason")
				for i := range todos {
					if todos[i].Title == target || todos[i].ID == target {
						if todos[i].Status == "pending" || todos[i].Status == "in_progress" {
							todos[i].Status = "blocked"
							todos[i].Blocker = reason
						}
					}
				}
			case "unblock":
				target := argString(args, "task")
				for i := range todos {
					if (todos[i].Title == target || todos[i].ID == target) && todos[i].Status == "blocked" {
						todos[i].Status = "pending"
						todos[i].Blocker = ""
					}
				}
			case "rm":
				target := argString(args, "task")
				phase := argString(args, "phase")
				var kept []TodoItem
				for _, t := range todos {
					if target != "" && (t.Title == target || t.ID == target) {
						continue
					}
					if phase != "" && t.Phase == phase {
						continue
					}
					kept = append(kept, t)
				}
				todos = kept
			case "view":
				// no mutation
			default:
				errors = append(errors, "unknown op: "+op)
			}

			if len(errors) > 0 {
				bridge.SetTodos(todos)
				return toolResult(map[string]any{"errors": errors, "todos": todos}), nil
			}
			bridge.SetTodos(todos)
			return toolResult(map[string]any{"todos": todos, "count": len(todos)}), nil
		},
	}
}

// ask — structured follow-up questions. The agent pauses
// and the user picks from options; the answer is injected back as a tool result.
func askTool() ToolSpec {
	return ToolSpec{
		Name:        "ask",
		Description: "Ask the user structured follow-up questions. Each question has an id, text, and options; the user picks one (or more if multi). Use this when a task is ambiguous instead of guessing.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"questions": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"id":          map[string]any{"type": "string"},
							"question":    map[string]any{"type": "string"},
							"header":      map[string]any{"type": "string"},
							"options":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
							"multi":       map[string]any{"type": "boolean"},
							"recommended": map[string]any{"type": "integer"},
						},
						"required": []string{"id", "question", "options"},
					},
				},
			},
			"required": []string{"questions"},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			bridge := SessionBridgeFrom(ctx)
			if bridge == nil {
				return nil, fmt.Errorf("ask is unavailable outside an agent session")
			}
			raw, _ := args["questions"].([]any)
			if len(raw) == 0 {
				return nil, fmt.Errorf("questions must not be empty")
			}
			var qs []AskQuestion
			for _, r := range raw {
				m, _ := r.(map[string]any)
				if m == nil {
					continue
				}
				var opts []string
				if o, ok := m["options"].([]any); ok {
					for _, oo := range o {
						if s, ok := oo.(string); ok {
							opts = append(opts, s)
						}
					}
				}
				if len(opts) == 0 {
					continue
				}
				qs = append(qs, AskQuestion{
					ID:          argString(m, "id"),
					Question:    argString(m, "question"),
					Header:      argString(m, "header"),
					Options:     opts,
					Multi:       argBool(m, "multi", false),
					Recommended: argInt(m, "recommended", -1),
				})
			}
			if len(qs) == 0 {
				return nil, fmt.Errorf("questions must have options")
			}
			if err := bridge.Ask(qs); err != nil {
				return nil, err
			}
			// Return a marker result; the agent loop notices the session is in
			// the awaiting_input state and pauses the turn.
			return toolResult(map[string]any{"status": "awaiting_input", "questions": qs}), nil
		},
	}
}

// Ensure json is referenced (used by callers of this file's helpers).
var _ = json.Marshal
