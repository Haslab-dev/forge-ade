package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/hasdev/forge-ade/internal/llm"
	"github.com/hasdev/forge-ade/internal/search"
)

type ToolHandler func(ctx context.Context, args map[string]interface{}) (interface{}, error)

type ToolSpec struct {
	Name        string
	Description string
	Parameters  map[string]interface{}
	// Cost is one of "cheap" (1pt), "medium" (3pt), "high" (10pt). Used for
	// the per-turn tool budget the agent sees in its system prompt.
	Cost    string
	Handler ToolHandler
}

type Registry struct {
	tools     map[string]ToolSpec
	mcpCaller MCPCaller
}

func NewRegistry(searchMgr *search.SearchManager) *Registry {
	r := &Registry{
		tools: make(map[string]ToolSpec),
	}

	// Core canonical tool surface (read/write/edit/bash/search/find/glob/todo/ask)
	// is the primary tool set.
	r.registerCoreTools(searchAdapter{sm: searchMgr})

	// Legacy alias names → core tools, so old prompts and habits keep working.
	r.registerLegacyAliases()

	return r
}

// searchAdapter adapts the search manager to the tools package's minimal API.
type searchAdapter struct {
	sm *search.SearchManager
}

func (a searchAdapter) SearchContentWithOptions(opts searchOptions) ([]searchResult, error) {
	if a.sm == nil {
		return nil, nil
	}
	res, err := a.sm.SearchContentWithOptions(search.SearchOptions{
		Query:          opts.Query,
		Limit:          opts.Limit,
		MatchCase:      opts.MatchCase,
		MatchWholeWord: opts.MatchWholeWord,
		UseRegex:       opts.UseRegex,
	})
	if err != nil {
		return nil, err
	}
	out := make([]searchResult, 0, len(res))
	for _, r := range res {
		out = append(out, searchResult{Path: r.Path, Filename: r.Filename, Score: r.Score, Line: r.Line, Content: r.Content})
	}
	return out, nil
}

func (a searchAdapter) SearchFilenameWithOptions(opts searchOptions) []searchResult {
	if a.sm == nil {
		return nil
	}
	res := a.sm.SearchFilenameWithOptions(search.SearchOptions{
		Query: opts.Query,
		Limit: opts.Limit,
	})
	out := make([]searchResult, 0, len(res))
	for _, r := range res {
		out = append(out, searchResult{Path: r.Path, Filename: r.Filename, Score: r.Score})
	}
	return out
}

// registerLegacyAliases maps the old tool names onto the core primary tools.
func (r *Registry) registerLegacyAliases() {
	alias := map[string]string{
		"read_file":        "read",
		"view_file":        "read",
		"cat":              "read",
		"write_file":       "write",
		"create_file":      "write",
		"run_shell":        "bash",
		"exec":             "bash",
		"run_command":      "bash",
		"search_workspace": "search",
		"rg":               "search",
		"grep":             "search",
		"ripgrep":          "search",
		"list_dir":         "read",
		"ls":               "read",
		"git_status":       "git_status",
	}
	for old, new := range alias {
		if spec, ok := r.tools[new]; ok {
			r.tools[old] = spec
		}
	}
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

// RegisterMCPTools registers tools discovered from MCP servers. Each tool's
// name is the full "server/tool" form so routing back to the right server is
// unambiguous. The handler delegates to the MCP manager's CallTool.
func (r *Registry) RegisterMCPTools(tools []llm.MCPTool) {
	for _, t := range tools {
		name := t.ServerName + "/" + t.Name
		handler := func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			return nil, fmt.Errorf("mcp tool %s not wired", name)
		}
		r.tools[name] = ToolSpec{
			Name:        name,
			Description: t.Description,
			Parameters:  t.InputSchema,
			Handler:     handler,
		}
	}
}

// MCPCaller abstracts the MCP manager so the registry can dispatch MCP tool
// calls without importing the mcp package (avoiding an import cycle).
type MCPCaller interface {
	CallTool(ctx context.Context, fullName string, args map[string]any) (string, error)
}

// SetMCPCaller installs the MCP caller used by registered MCP tools.
func (r *Registry) SetMCPCaller(caller MCPCaller) {
	r.mcpCaller = caller
}

// RegisterMCPToolWithCaller registers a single MCP tool and wires its handler
// to the given caller.
func (r *Registry) RegisterMCPToolWithCaller(t llm.MCPTool, caller MCPCaller) {
	name := t.ServerName + "/" + t.Name
	tool := t
	r.tools[name] = ToolSpec{
		Name:        name,
		Description: t.Description,
		Parameters:  t.InputSchema,
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			return caller.CallTool(ctx, tool.ServerName+"/"+tool.Name, args)
		},
	}
}

// UnregisterMCPTools removes previously registered MCP tools (used when MCP
// servers are reconnected or removed).
func (r *Registry) UnregisterMCPTools(serverName string) {
	for name := range r.tools {
		if strings.HasPrefix(name, serverName+"/") {
			delete(r.tools, name)
		}
	}
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

func (r *Registry) Lookup(name string) (ToolSpec, bool) {
	spec, ok := r.tools[name]
	return spec, ok
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
