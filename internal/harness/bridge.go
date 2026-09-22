package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hasdev/forge-ade/internal/llm"
)

// MCPBridge abstracts the existing internal/mcp manager so the harness
// registry can register/dispatch MCP tools without an import cycle.
type MCPBridge interface {
	ListTools(ctx context.Context) ([]llm.MCPTool, error)
	CallTool(ctx context.Context, serverTool string, args map[string]any) (string, error)
}

// SkillsBridge abstracts the existing skills manager catalog.
type SkillsBridge interface {
	Catalog() []SkillEntry
	Load(name string) (string, error)
}

// Bridge wires ForgeADE's existing MCP + skills managers into a harness
// ToolRegistry following harness semantics: MCP tools named
// mcp__<server>__<tool>, Skill tool with the discovery list + content
// envelope.
func Bridge(reg *ToolRegistry, mcp MCPBridge, skills SkillsBridge) error {
	if mcp != nil {
		tools, err := mcp.ListTools(context.Background())
		if err == nil {
			for _, t := range tools {
				name := MCPToolName(t.ServerName, t.Name)
				server, tool := t.ServerName, t.Name
				_ = reg.Register(&ToolEntry{
					Metadata: ToolMetadata{
						Name: name, Description: t.Description,
						ReadOnly: false,
					},
					InputSchema: t.InputSchema,
					Execute: func(ctx context.Context, input map[string]any) (ToolOutput, error) {
						out, err := mcpBridgeCall(ctx, mcp, server, tool, input)
						return ToolOutput{Content: out}, err
					},
				})
			}
		}
	}
	if skills != nil {
		_ = reg.Register(&ToolEntry{
			Metadata: ToolMetadata{
				Name: "Skill",
				Description: "Load a skill's full instructions into context. " +
					"Use the skill names from the available-skills list.",
				ReadOnly: true, TimeoutMs: 30000,
			},
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"skill": map[string]any{"type": "string", "description": "Skill name or qualified plugin:name"},
					"args":  map[string]any{"type": "string", "description": "Optional arguments"},
				},
				"required": []string{"skill"},
			},
			Execute: func(ctx context.Context, input map[string]any) (ToolOutput, error) {
				name, _ := input["skill"].(string)
				if name == "" {
					if n, ok := input["name"].(string); ok {
						name = n
					}
				}
				if name == "" {
					return ToolOutput{IsError: true, Content: "missing required parameter: skill"}, nil
				}
				content, err := skills.Load(name)
				if err != nil {
					return ToolOutput{IsError: true, Content: err.Error()}, nil
				}
				return ToolOutput{Content: content}, nil
			},
		})
	}
	return nil
}

func mcpBridgeCall(ctx context.Context, m MCPBridge, server, tool string, input map[string]any) (string, error) {
	return m.CallTool(ctx, server+"/"+tool, input)
}

// WorkspaceTools registers the core coding tool set on the harness registry,
// delegating execution to ForgeADE's existing tool registry (which already
// implements read/write/edit/bash/search/glob/todo with workspace policy).
type WorkspaceTools interface {
	Execute(ctx context.Context, name string, args map[string]any) (string, bool, error)
	Names() []string
}

// RegisterWorkspaceTools mirrors every tool exposed by ForgeADE's registry
// into the harness registry with the standard metadata defaults.
func RegisterWorkspaceTools(reg *ToolRegistry, wt WorkspaceTools) error {
	for _, name := range wt.Names() {
		n := name
		meta := ToolMetadata{Name: n, Description: "Workspace tool " + n, SideEffectScope: "workspace"}
		switch strings.ToLower(n) {
		case "read", "glob", "grep", "search", "webfetch", "websearch", "todo", "load_skill":
			meta.ReadOnly = true
			meta.SideEffectScope = "none"
		}
		_ = reg.Register(&ToolEntry{
			Metadata: meta,
			InputSchema: map[string]any{"type": "object"},
			Execute: func(ctx context.Context, input map[string]any) (ToolOutput, error) {
				out, isErr, err := wt.Execute(ctx, n, input)
				o := ToolOutput{Content: out, IsError: isErr}
				if err != nil {
					o.Content = err.Error()
					o.IsError = true
				}
				return o, nil
			},
		})
	}
	return nil
}

var _ = fmt.Sprintf
var _ = filepath.Join
var _ = json.Marshal
var _ = os.ReadFile
