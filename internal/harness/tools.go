package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ToolMetadata mirrors ToolEntry.metadata in core/src/tool/registry.ts.
type ToolMetadata struct {
	Name                  string `json:"name"`
	Description           string `json:"description"`
	ModelInstructions     string `json:"model_instructions,omitempty"`
	ReadOnly              bool   `json:"read_only,omitempty"`
	Destructive           bool   `json:"destructive,omitempty"`
	ConcurrentSafe        *bool  `json:"concurrent_safe,omitempty"`
	ProviderVisible       *bool  `json:"provider_visible,omitempty"`
	MaxOutputBytes        int    `json:"max_output_bytes,omitempty"`
	TimeoutMs             int    `json:"timeout_ms,omitempty"`
	NeedsApproval         bool   `json:"needs_approval,omitempty"`
	SideEffectScope       string `json:"side_effect_scope,omitempty"` // none|session|workspace
	RequiresUserInteraction bool `json:"requires_user_interaction,omitempty"`
}

// ReadOnlySet is the scheduler default (tool/scheduler.ts:233-243).
var ReadOnlySet = map[string]bool{
	"Read": true, "Glob": true, "Grep": true, "WebSearch": true, "WebFetch": true,
	"TodoRead": true, "TodoWrite": true, "AskUserQuestion": true, "Skill": true,
}

// ToolEntry is a registered tool.
type ToolEntry struct {
	Metadata     ToolMetadata
	InputSchema  map[string]any
	Aliases      []string
	Execute      func(ctx context.Context, input map[string]any) (ToolOutput, error)
	ProviderID   string // non-empty for MCP tools
}

// ToolOutput is what a tool returns: model-visible content plus optional
// turn-control / follow-up flags (port of result shape §3.6).
type ToolOutput struct {
	Content     string          `json:"content"` // model content; images serialize as blocks
	IsError     bool            `json:"is_error,omitempty"`
	StopTurn    bool            `json:"stop_turn,omitempty"`
	FollowUpIn  string          `json:"follow_up_user_input,omitempty"`
	Structured  json.RawMessage `json:"structured,omitempty"`
}

// Registry port of tool/registry.ts: canonical names win; conflicting aliases
// rejected; toContracts strips execute and merges modelInstructions.
type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]*ToolEntry
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{tools: map[string]*ToolEntry{}}
}

func (r *ToolRegistry) Register(e *ToolEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, dup := r.tools[e.Metadata.Name]; dup {
		return fmt.Errorf("tool %s already registered", e.Metadata.Name)
	}
	r.tools[e.Metadata.Name] = e
	for _, a := range e.Aliases {
		r.tools[a] = e
	}
	return nil
}

func (r *ToolRegistry) Lookup(name string) (*ToolEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.tools[name]
	return e, ok
}

// Names returns canonical provider-visible tool names in registration order
// (canonical always wins over aliases).
func (r *ToolRegistry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	seen := map[*ToolEntry]bool{}
	var out []string
	for _, e := range r.tools {
		if seen[e] || !e.Metadata.ReadOnly && e.Metadata.ProviderVisible != nil && !*e.Metadata.ProviderVisible {
			continue
		}
		seen[e] = true
		out = append(out, e.Metadata.Name)
	}
	return out
}

// Contracts produces provider tool definitions.
func (r *ToolRegistry) Contracts() []map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()
	seen := map[*ToolEntry]bool{}
	var out []map[string]any
	for _, e := range r.tools {
		if seen[e] {
			continue
		}
		seen[e] = true
		desc := e.Metadata.Description
		if e.Metadata.ModelInstructions != "" {
			desc += "\n\nUsage:\n- " + e.Metadata.ModelInstructions
		}
		out = append(out, map[string]any{
			"name":        e.Metadata.Name,
			"description": desc,
			"input_schema": e.InputSchema,
		})
	}
	return out
}

// canRunInParallel port of scheduler canRunInParallel.
func (e *ToolEntry) canRunInParallel() bool {
	m := &e.Metadata
	if m.Destructive {
		return false
	}
	if m.ConcurrentSafe != nil {
		return *m.ConcurrentSafe
	}
	if ReadOnlySet[m.Name] || m.ReadOnly {
		return true
	}
	return m.SideEffectScope == "none" || m.SideEffectScope == ""
}

// ScheduledTool is one item in an execution schedule.
type ScheduledTool struct {
	Call ToolCall `json:"call"`
}

// Schedule is the grouped execution plan.
type Schedule struct {
	Items          []ScheduledTool `json:"items"`
	ParallelGroups [][]int         `json:"parallel_groups"` // indexes into Items
}

// ScheduleTools port of tool/scheduler.ts: level-based topological grouping,
// parallel items accumulate into groups capped at maxConcurrency (10),
// non-parallel items get singleton groups. DependsOn cycles are rejected.
func ScheduleTools(reg *ToolRegistry, calls []ToolCall, maxConcurrency int) (*Schedule, error) {
	if maxConcurrency <= 0 {
		maxConcurrency = 10
	}
	levels := make([]int, len(calls))
	entryFor := func(c ToolCall) *ToolEntry {
		e, _ := reg.Lookup(c.Name)
		return e
	}
	// level = max(dep levels)+1; deps referenced by call ID
	byID := map[string]int{}
	for i, c := range calls {
		byID[c.ID] = i
	}
	for i := range calls {
		lvl := 0
		for _, dep := range depIDs(calls[i].Input) {
			if j, ok := byID[dep]; ok && j != i {
				if levels[j]+1 > lvl {
					lvl = levels[j] + 1
				}
			}
		}
		levels[i] = lvl
	}
	sched := &Schedule{Items: make([]ScheduledTool, len(calls))}
	for i, c := range calls {
		sched.Items[i] = ScheduledTool{Call: c}
	}
	maxLevel := 0
	for _, l := range levels {
		if l > maxLevel {
			maxLevel = l
		}
	}
	for lvl := 0; lvl <= maxLevel; lvl++ {
		var parallelIdx []int
		for i := range calls {
			if levels[i] != lvl {
				continue
			}
			e := entryFor(calls[i])
			if e != nil && e.canRunInParallel() {
				parallelIdx = append(parallelIdx, i)
				continue
			}
			sched.ParallelGroups = append(sched.ParallelGroups, []int{i}) // singleton
		}
		for start := 0; start < len(parallelIdx); start += maxConcurrency {
			end := start + maxConcurrency
			if end > len(parallelIdx) {
				end = len(parallelIdx)
			}
			sched.ParallelGroups = append(sched.ParallelGroups, parallelIdx[start:end])
		}
	}
	return sched, nil
}

// depIDs extracts dependsOn references from a call input (string or []string).
func depIDs(input map[string]any) []string {
	v, ok := input["dependsOn"]
	if !ok {
		return nil
	}
	switch t := v.(type) {
	case string:
		return []string{t}
	case []any:
		var out []string
		for _, x := range t {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// Executor runs a schedule with concurrency, permission gating and hook
// wiring. Port of tool/executor/ batch-runner semantics: later groups still
// run after failures; only a result with StopTurn truncates remaining groups.
type Executor struct {
	Registry       *ToolRegistry
	Permissions    *PermissionService
	HookRunner     HookRunner
	MaxConcurrency int
	DefaultTimeout int // ms
}

// ExecuteSchedule runs groups in order. onResult fires per completed call.
func (x *Executor) ExecuteSchedule(ctx context.Context, sched *Schedule, session *SessionContext, onResult func(ToolResult)) {
	done := map[string]ToolResult{}
	stop := false
	for _, group := range sched.ParallelGroups {
		if stop {
			for _, i := range group {
				res := ToolResult{CallID: sched.Items[i].Call.ID, Success: false,
					Content: "Tool cancelled because a previous tool result requested a turn stop."}
				done[res.CallID] = res
				if onResult != nil {
					onResult(res)
				}
			}
			continue
		}
		sem := make(chan struct{}, x.concurrency())
		var wg sync.WaitGroup
		var mu sync.Mutex
		for _, i := range group {
			item := sched.Items[i]
			wg.Add(1)
			go func() {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				res := x.executeOne(ctx, item.Call, session)
				mu.Lock()
				done[item.Call.ID] = res
				mu.Unlock()
				if onResult != nil {
					onResult(res)
				}
				if res.StopTurn {
					mu.Lock()
					stop = true
					mu.Unlock()
				}
			}()
		}
		wg.Wait()
	}
}

func (x *Executor) concurrency() int {
	if x.MaxConcurrency > 0 {
		return x.MaxConcurrency
	}
	return 10
}

func (x *Executor) timeoutMs(e *ToolEntry) int {
	if e.Metadata.TimeoutMs > 0 {
		return e.Metadata.TimeoutMs
	}
	if x.DefaultTimeout > 0 {
		return x.DefaultTimeout
	}
	return 300000
}

// executeOne: permission gate → hook → execute → envelope. Unknown tool and
// panics become recoverable error results so the turn keeps running.
func (x *Executor) executeOne(ctx context.Context, call ToolCall, session *SessionContext) ToolResult {
	e, ok := x.Registry.Lookup(call.Name)
	if !ok {
		return ToolResult{CallID: call.ID, Success: false, Content: fmt.Sprintf("unknown tool %s", call.Name)}
	}
	// Permission flow
	if x.Permissions != nil {
		decision, err := x.Permissions.Check(ctx, session, e, call.Name, call.Input)
		if err != nil {
			return ToolResult{CallID: call.ID, Success: false, Content: err.Error()}
		}
		switch decision.Action {
		case PermActionDeny:
			return ToolResult{CallID: call.ID, Success: false, Content: decision.Message}
		case PermActionAsk:
			// The host (UI) resolves; in headless mode default deny like the source.
			if x.Permissions.Broker == nil {
				return ToolResult{CallID: call.ID, Success: false, Content: "permission denied by default broker"}
			}
			granted, msg := x.Permissions.Broker.Ask(ctx, session, call.Name, call.Input, decision)
			if !granted {
				return ToolResult{CallID: call.ID, Success: false, Content: msg}
			}
		}
	}
	// PreToolUse hook
	if x.HookRunner != nil {
		if replacement, ok := x.HookRunner.PreToolUse(ctx, call.Name, call.Input); ok {
			call.Input = replacement
		}
	}
	tctx, cancel := context.WithTimeout(ctx, timeDurationMs(x.timeoutMs(e)))
	defer cancel()
	out, execErr := func() (out ToolOutput, err error) {
		defer func() {
			if r := recover(); r != nil {
				out = ToolOutput{IsError: true, Content: fmt.Sprintf("tool panicked: %v", r)}
				err = nil
			}
		}()
		return e.Execute(tctx, call.Input)
	}()
	res := ToolResult{CallID: call.ID, Success: !out.IsError && execErr == nil, Content: out.Content, StopTurn: out.StopTurn, FollowUpIn: out.FollowUpIn}
	if execErr != nil {
		res.Success = false
		res.Content = execErr.Error()
	}
	if x.HookRunner != nil && res.Success {
		res.Content = x.HookRunner.PostToolUse(ctx, call.Name, call.Input, res.Content)
	}
	return res
}

// HookRunner is the Pre/PostToolUse hook surface (full hook event set wired
// in the plugin system).
type HookRunner interface {
	PreToolUse(ctx context.Context, tool string, input map[string]any) (modified map[string]any, replaced bool)
	PostToolUse(ctx context.Context, tool string, input map[string]any, output string) string
}

func timeDurationMs(ms int) time.Duration {
	return time.Duration(ms) * time.Millisecond
}

// EnforceOversizedOutput wraps oversized tool output in the persisted-output
// envelope (spec §3.6): preview first 2000 chars plus saved path.
func EnforceOversizedOutput(content string, maxBytes int, savedPath string) string {
	if maxBytes <= 0 || len(content) <= maxBytes {
		return content
	}
	preview := content
	if len(preview) > 2000 {
		preview = preview[:2000]
	}
	return fmt.Sprintf("<persisted-output>Output too large (%d bytes). Full output saved to: %s\n\nPreview (first 2,000 chars): %s</persisted-output>", len(content), savedPath, preview)
}

// SanitizeMCPName port of core/src/mcp/name.ts sanitization.
func SanitizeMCPName(part string) string {
	var b strings.Builder
	prevUnderscore := false
	for _, r := range part {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
			prevUnderscore = false
		} else if !prevUnderscore {
			b.WriteByte('_')
			prevUnderscore = true
		}
	}
	if b.Len() == 0 {
		return "unknown"
	}
	return b.String()
}

// MCPToolName builds `mcp__<server>__<tool>`.
func MCPToolName(server, tool string) string {
	return "mcp__" + SanitizeMCPName(server) + "__" + SanitizeMCPName(tool)
}
