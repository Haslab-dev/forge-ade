package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/hasdev/forge-ade/internal/llm"
	"github.com/hasdev/forge-ade/internal/search"
)

type ToolHandler func(ctx context.Context, args map[string]interface{}) (interface{}, error)

type ToolSpec struct {
	Name        string
	Description string
	Parameters  map[string]interface{}
	Handler     ToolHandler
}

type Registry struct {
	tools map[string]ToolSpec
}

func NewRegistry(searchMgr *search.SearchManager) *Registry {
	r := &Registry{
		tools: make(map[string]ToolSpec),
	}

	// 1. read_file / view_file / cat (with start_line and end_line support)
	r.Register(ToolSpec{
		Name:        "read_file",
		Description: "Read contents of a file in workspace, with optional start_line and end_line range filtering.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path":       map[string]interface{}{"type": "string", "description": "Absolute or relative file path"},
				"start_line": map[string]interface{}{"type": "integer", "description": "Optional starting line number (1-based)"},
				"end_line":   map[string]interface{}{"type": "integer", "description": "Optional ending line number (1-based, inclusive)"},
			},
			"required": []string{"path"},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			path, _ := args["path"].(string)
			if path == "" {
				return nil, fmt.Errorf("path is required")
			}

			contentBytes, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("failed to read file %s: %w", path, err)
			}

			startLine := 0
			endLine := 0

			if v, ok := args["start_line"]; ok {
				switch num := v.(type) {
				case float64:
					startLine = int(num)
				case int:
					startLine = num
				}
			}

			if v, ok := args["end_line"]; ok {
				switch num := v.(type) {
				case float64:
					endLine = int(num)
				case int:
					endLine = num
				}
			}

			fullContent := string(contentBytes)
			if startLine > 0 || endLine > 0 {
				lines := strings.Split(fullContent, "\n")
				totalLines := len(lines)
				if startLine < 1 {
					startLine = 1
				}
				if endLine < 1 || endLine > totalLines {
					endLine = totalLines
				}
				if startLine <= endLine && startLine <= totalLines {
					selected := lines[startLine-1 : endLine]
					return map[string]interface{}{
						"path":        path,
						"start_line":  startLine,
						"end_line":    endLine,
						"total_lines": totalLines,
						"content":     strings.Join(selected, "\n"),
					}, nil
				}
			}

			return map[string]interface{}{"path": path, "content": fullContent}, nil
		},
	})

	// Register aliases for read_file
	r.tools["view_file"] = r.tools["read_file"]
	r.tools["cat"] = r.tools["read_file"]

	// 2. write_file / create_file
	r.Register(ToolSpec{
		Name:        "write_file",
		Description: "Write content to a file, creating parent directories if necessary.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path":    map[string]interface{}{"type": "string", "description": "Target file path"},
				"content": map[string]interface{}{"type": "string", "description": "Full file content"},
			},
			"required": []string{"path", "content"},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			path, _ := args["path"].(string)
			content, _ := args["content"].(string)
			if path == "" {
				return nil, fmt.Errorf("path is required")
			}
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return nil, fmt.Errorf("failed to create directory: %w", err)
			}
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				return nil, fmt.Errorf("failed to write file: %w", err)
			}
			return map[string]interface{}{"path": path, "status": "written"}, nil
		},
	})
	r.tools["create_file"] = r.tools["write_file"]

	// 3. search_workspace / rg / grep / ripgrep
	r.Register(ToolSpec{
		Name:        "search_workspace",
		Description: "Search pattern or query in workspace using ripgrep (rg) or grep fallback.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{"type": "string", "description": "Text pattern or regex to search"},
				"path":  map[string]interface{}{"type": "string", "description": "Optional search directory path"},
			},
			"required": []string{"query"},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			query, _ := args["query"].(string)
			searchDir, _ := args["path"].(string)
			if query == "" {
				return nil, fmt.Errorf("query is required")
			}
			if searchDir == "" {
				searchDir, _ = os.Getwd()
			}

			// Try executing rg binary first
			rgPath := findExecutable("rg", []string{
				"/Users/hy4-mac-002/homebrew/bin/rg",
				"/usr/local/bin/rg",
				"/opt/homebrew/bin/rg",
			})

			if rgPath != "" {
				cmdCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
				defer cancel()
				cmd := exec.CommandContext(cmdCtx, rgPath, "-n", "-m", "50", query, searchDir)
				out, err := cmd.CombinedOutput()
				if err == nil || len(out) > 0 {
					return map[string]interface{}{
						"query":  query,
						"engine": "ripgrep",
						"output": string(out),
					}, nil
				}
			}

			// Fallback to grep
			grepCmd := exec.CommandContext(ctx, "grep", "-rn", "-m", "50", query, searchDir)
			out, err := grepCmd.CombinedOutput()
			if err == nil || len(out) > 0 {
				return map[string]interface{}{
					"query":  query,
					"engine": "grep",
					"output": string(out),
				}, nil
			}

			// Fallback to internal search manager
			var results []search.RankedResult
			if searchMgr != nil {
				results, _ = searchMgr.SearchContent(query, 20)
			}
			return map[string]interface{}{"query": query, "engine": "internal", "results": results}, nil
		},
	})
	r.tools["rg"] = r.tools["search_workspace"]
	r.tools["grep"] = r.tools["search_workspace"]
	r.tools["ripgrep"] = r.tools["search_workspace"]
	r.tools["search"] = r.tools["search_workspace"]

	// 4. list_dir / ls / glob
	r.Register(ToolSpec{
		Name:        "list_dir",
		Description: "List files and subdirectories inside a directory.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{"type": "string", "description": "Target directory path"},
			},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			path, _ := args["path"].(string)
			if path == "" {
				path, _ = os.Getwd()
			}
			entries, err := os.ReadDir(path)
			if err != nil {
				return nil, fmt.Errorf("read dir %s: %w", path, err)
			}

			var items []map[string]interface{}
			for _, entry := range entries {
				info, _ := entry.Info()
				size := int64(0)
				if info != nil {
					size = info.Size()
				}
				items = append(items, map[string]interface{}{
					"name":   entry.Name(),
					"is_dir": entry.IsDir(),
					"size":   size,
				})
			}
			return map[string]interface{}{"path": path, "entries": items}, nil
		},
	})
	r.tools["ls"] = r.tools["list_dir"]

	// 4b. glob — real glob pattern matching
	r.Register(ToolSpec{
		Name:        "glob",
		Description: "Find files matching a glob pattern (e.g. **/*.go, src/*.tsx).",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"pattern": map[string]interface{}{"type": "string", "description": "Glob pattern to match"},
				"cwd":     map[string]interface{}{"type": "string", "description": "Base directory (defaults to workspace root)"},
			},
			"required": []string{"pattern"},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			pattern, _ := args["pattern"].(string)
			if pattern == "" {
				return nil, fmt.Errorf("pattern is required")
			}
			cwd, _ := args["cwd"].(string)
			if cwd == "" {
				cwd, _ = os.Getwd()
			}
			full := pattern
			if !strings.HasPrefix(pattern, "/") {
				full = filepath.Join(cwd, pattern)
			}
			matches, err := filepath.Glob(full)
			if err != nil {
				return nil, fmt.Errorf("glob: %w", err)
			}
			var items []map[string]interface{}
			for _, m := range matches {
				info, err := os.Stat(m)
				if err != nil {
					continue
				}
				items = append(items, map[string]interface{}{
					"path":   m,
					"is_dir": info.IsDir(),
					"size":   info.Size(),
				})
			}
			return map[string]interface{}{"pattern": pattern, "matches": items, "count": len(items)}, nil
		},
	})

	// 5. run_shell / bash / exec / run_command
	r.Register(ToolSpec{
		Name:        "run_shell",
		Description: "Run non-interactive shell command and capture stdout/stderr output.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"command": map[string]interface{}{"type": "string", "description": "Shell command to execute"},
				"cwd":     map[string]interface{}{"type": "string", "description": "Working directory"},
			},
			"required": []string{"command"},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			commandStr, _ := args["command"].(string)
			cwd, _ := args["cwd"].(string)
			if commandStr == "" {
				return nil, fmt.Errorf("command is required")
			}

			cmdCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
			defer cancel()

			cmd := exec.CommandContext(cmdCtx, "/bin/zsh", "-l", "-c", commandStr)
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

			return map[string]interface{}{
				"stdout":    stdout.String(),
				"stderr":    stderr.String(),
				"exit_code": exitCode,
			}, nil
		},
	})
	r.tools["bash"] = r.tools["run_shell"]
	r.tools["exec"] = r.tools["run_shell"]
	r.tools["run_command"] = r.tools["run_shell"]

	// 6. git_status
	r.Register(ToolSpec{
		Name:        "git_status",
		Description: "Get git repository status output.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"dir": map[string]interface{}{"type": "string", "description": "Repository directory path"},
			},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			dir, _ := args["dir"].(string)
			if dir == "" {
				dir, _ = os.Getwd()
			}
			cmd := exec.CommandContext(ctx, "git", "status", "--porcelain=v2")
			cmd.Dir = dir
			out, err := cmd.CombinedOutput()
			if err != nil {
				return nil, fmt.Errorf("git status error: %w", err)
			}
			return map[string]interface{}{"status": string(out)}, nil
		},
	})

	return r
}

func findExecutable(name string, candidatePaths []string) string {
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	for _, cp := range candidatePaths {
		if _, err := os.Stat(cp); err == nil {
			return cp
		}
	}
	return ""
}

func (r *Registry) Register(spec ToolSpec) {
	r.tools[spec.Name] = spec
}

func (r *Registry) Definitions() []llm.ToolDefinition {
	defs := make([]llm.ToolDefinition, 0, len(r.tools))
	seen := make(map[string]bool)
	for _, spec := range r.tools {
		if seen[spec.Name] {
			continue
		}
		seen[spec.Name] = true
		defs = append(defs, llm.ToolDefinition{
			Type: "function",
			Function: llm.FunctionSpec{
				Name:        spec.Name,
				Description: spec.Description,
				Parameters:  spec.Parameters,
			},
		})
	}
	return defs
}

func (r *Registry) Execute(ctx context.Context, name string, rawArgs string) (interface{}, error) {
	alias := name
	switch strings.ToLower(name) {
	case "view_file", "cat":
		alias = "read_file"
	case "create_file":
		alias = "write_file"
	case "rg", "grep", "ripgrep", "search":
		alias = "search_workspace"
	case "ls":
		alias = "list_dir"
	case "bash", "exec", "run_command":
		alias = "run_shell"
	}

	spec, ok := r.tools[alias]
	if !ok {
		return nil, fmt.Errorf("unknown tool %s", name)
	}

	var args map[string]interface{}
	if strings.TrimSpace(rawArgs) != "" {
		if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
			return nil, fmt.Errorf("invalid json arguments for %s: %w", name, err)
		}
	} else {
		args = make(map[string]interface{})
	}

	return spec.Handler(ctx, args)
}
