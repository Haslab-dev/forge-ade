package harness

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/hasdev/forge-ade/internal/llm"
)

// StreamHandler receives UI-bound streaming events.
type StreamHandler interface {
	OnTextDelta(delta string)
	OnReasoningDelta(delta string)
	OnToolStatus(callID, toolName, status, detail string)
	OnTurnComplete(resultType TurnResultType, response string)
}

// Runtime is the agentic turn loop: port of runtime/methods/turn-loop.ts +
// turn-model-step.ts + turn-tools.ts, simplified to the Go single-process
// model while preserving the machine transitions, tool batching, permission
// gating, compaction thresholds, and turn-control semantics.
type Runtime struct {
	LLM         *llm.LLMClient
	Registry    *ToolRegistry
	Executor    *Executor
	History     *MessageHistory
	System      string
	Stream      StreamHandler
	// Compaction thresholds (spec §2.4)
	ContextWindow    int64 // default 200000
	AutoCompactBuffer int64 // 13000
	PreflightReserve int64 // 21000
	// MaxModelSteps caps runaway loops in the native port (the source relies
	// on no-tool-call exit; we keep an explicit circuit breaker).
	MaxModelSteps int

	usageTokens     int64
	lastPromptTokens int64
}

func NewRuntime(llmClient *llm.LLMClient, reg *ToolRegistry, perms *PermissionService, stream StreamHandler) *Runtime {
	return &Runtime{
		LLM: llmClient, Registry: reg,
		Executor: &Executor{Registry: reg, Permissions: perms},
		History:  NewMessageHistory(),
		ContextWindow: 200000, AutoCompactBuffer: 13000, PreflightReserve: 21000,
		MaxModelSteps: 100,
		Stream:        stream,
	}
}

// SessionContext for the current turn (modes/rules).
type TurnContext struct {
	Session *SessionContext
	TurnNo  int
}

// ExecuteTurn runs one full user turn: the agentic loop until the model
// produces no tool calls, the turn is stopped, or a circuit breaker trips.
func (r *Runtime) ExecuteTurn(ctx context.Context, userContent string, tctx TurnContext) (TurnResultType, string) {
	machine := NewTurnMachine(tctx.Session.SessionID, tctx.TurnNo, userContent)
	_ = machine.Start()
	r.History.AddUser(userContent, "real_user")

	r.usageTokens = r.History.TokenEstimate()
	modelSteps := 0
	for {
		if err := ctx.Err(); err != nil {
			machine.Complete("", ResultCancelled)
			r.emitComplete(ResultCancelled)
			return ResultCancelled, ""
		}
		if modelSteps >= r.MaxModelSteps {
			machine.Complete("", ResultErrorMaxTurns)
			r.emitComplete(ResultErrorMaxTurns)
			return ResultErrorMaxTurns, ""
		}
		// Mid-turn auto/micro compaction check (spec §2.4).
		r.maybeCompact()

		if err := machine.StartModelRequest(""); err != nil {
			machine.Fail(err)
			r.emitComplete(ResultErrorExecution)
			return ResultErrorExecution, ""
		}
		// Cancellable model request.
		mctx, cancel := context.WithCancel(ctx)
		defer cancel()

		resp, err := r.LLM.ChatWithStreamDetailed(mctx, r.providerMessages(), r.providerTools(),
			func(deltaContent, deltaReasoning string) {
				if deltaContent != "" {
					_ = machine.AddStreamingContent(deltaContent)
					if r.Stream != nil {
						r.Stream.OnTextDelta(deltaContent)
					}
				}
				if deltaReasoning != "" && r.Stream != nil {
					r.Stream.OnReasoningDelta(deltaReasoning)
				}
			}, nil)
		if err != nil {
			cancel()
			if ctx.Err() != nil {
				// cancellation is a normal result: commit partial text
				if machine.State().StreamingContent != "" {
					r.History.AddAssistant(machine.State().StreamingContent, nil, "", 0)
				}
				machine.Complete(machine.State().StreamingContent, ResultCancelled)
				r.emitComplete(ResultCancelled)
				return ResultCancelled, machine.State().StreamingContent
			}
			machine.Fail(err)
			r.emitComplete(ResultErrorExecution)
			return ResultErrorExecution, ""
		}
		modelSteps++
		r.recordUsage(resp)

		if len(resp.ToolCalls) == 0 {
			// Output-token continuation: finishReason length with no tools
			// appends a continuation entry and re-requests (bounded).
			if resp.FinishReason == "length" && modelSteps < r.MaxModelSteps {
				r.History.AddAssistant(resp.Content, nil, "", 0)
				r.History.AddUser("[continue]", "legacy_synthetic")
				continue
			}
			r.History.AddAssistant(resp.Content, nil, "", 0)
			if err := machine.Complete(resp.Content, ResultSuccess); err != nil {
				machine.Fail(err)
			}
			r.emitComplete(ResultSuccess)
			return ResultSuccess, resp.Content
		}

		// Commit assistant tool_use to history.
		calls := make([]MessageToolCall, 0, len(resp.ToolCalls))
		coreCalls := make([]ToolCall, 0, len(resp.ToolCalls))
		for _, tc := range resp.ToolCalls {
			calls = append(calls, MessageToolCall{ID: tc.ID, Name: tc.Function.Name, ArgsJSON: tc.Function.Arguments})
			input := map[string]any{}
			if strings.TrimSpace(tc.Function.Arguments) != "" {
				_ = json.Unmarshal([]byte(tc.Function.Arguments), &input)
			}
			coreCalls = append(coreCalls, ToolCall{ID: tc.ID, Name: tc.Function.Name, Input: input})
		}
		r.History.AddAssistant(resp.Content, calls, "", 0)
		if err := machine.ScheduleTools(coreCalls); err != nil {
			machine.Fail(err)
			r.emitComplete(ResultErrorExecution)
			return ResultErrorExecution, ""
		}
		if err := machine.StartToolExecution(); err != nil {
			machine.Fail(err)
			r.emitComplete(ResultErrorExecution)
			return ResultErrorExecution, ""
		}

		sched, err := ScheduleTools(r.Registry, coreCalls, 10)
		if err != nil {
			machine.Fail(err)
			r.emitComplete(ResultErrorExecution)
			return ResultErrorExecution, ""
		}
		var firstStop TurnResultType
		r.Executor.ExecuteSchedule(ctx, sched, tctx.Session, func(res ToolResult) {
			_ = machine.CompleteTool(res.CallID, res)
			status := "completed"
			if !res.Success {
				status = "failed"
			}
			if r.Stream != nil {
				name := ""
				for _, c := range coreCalls {
					if c.ID == res.CallID {
						name = c.Name
					}
				}
				r.Stream.OnToolStatus(res.CallID, name, status, res.Content)
			}
			r.History.AddToolResult(res.CallID, "", res.Content, res.Success)
			if res.StopTurn && firstStop == "" {
				firstStop = ResultSuccess
			}
		})
		if err := machine.AggregateResults(); err != nil {
			machine.Fail(err)
			r.emitComplete(ResultErrorExecution)
			return ResultErrorExecution, ""
		}
		if firstStop != "" {
			// turnControl.stopTurnAfterResult: complete the turn after the
			// batch, without another model step.
			machine.Complete("", firstStop)
			r.emitComplete(firstStop)
			return firstStop, machine.State().FinalResponse
		}
		// aggregating_results -> awaiting_model_response (the agentic loop)
	}
}

func (r *Runtime) emitComplete(rt TurnResultType) {
	if r.Stream != nil {
		r.Stream.OnTurnComplete(rt, "")
	}
}

// usage tracking fields (single-turn execution per Runtime instance).
var _ = struct{}{}

func (r *Runtime) maybeCompact() {
	threshold := r.ContextWindow - r.PreflightReserve - r.AutoCompactBuffer
	if r.usageTokens >= threshold {
		r.microcompact()
	}
}

// microcompact clears all but the 5 most recent tool results, replacing them
// with the standard stub (spec §2.4).
func (r *Runtime) microcompact() {
	const keepRecent = 5
	entries := r.History.Entries()
	toolIdx := []int{}
	for i, e := range entries {
		if e.Kind == EntryKindMessage && e.Message != nil && e.Message.Role == RoleTool {
			toolIdx = append(toolIdx, i)
		}
	}
	if len(toolIdx) <= keepRecent {
		return
	}
	for _, i := range toolIdx[:len(toolIdx)-keepRecent] {
		entries[i].Message.Content = "[Old tool result content cleared]"
	}
	r.History.ReplaceMessages(entries)
	r.usageTokens = r.History.TokenEstimate()
}

func (r *Runtime) providerMessages() []llm.LLMMessage {
	pm := r.History.ProviderMessages()
	out := make([]llm.LLMMessage, 0, len(pm)+1)
	out = append(out, llm.LLMMessage{Role: llm.RoleSystem, Content: r.System})
	for _, m := range pm {
		lm := llm.LLMMessage{Role: llm.Role(m.Role), Content: m.Content, ToolCallID: m.ToolCallID}
		for _, tc := range m.ToolCalls {
			lm.ToolCalls = append(lm.ToolCalls, llm.ToolCall{
				ID: tc.ID, Type: "function",
				Function: llm.ToolFunction{Name: tc.Name, Arguments: tc.ArgsJSON},
			})
		}
		out = append(out, lm)
	}
	return out
}

func (r *Runtime) providerTools() []llm.ToolDefinition {
	contracts := r.Registry.Contracts()
	out := make([]llm.ToolDefinition, 0, len(contracts))
	for _, c := range contracts {
		name, _ := c["name"].(string)
		desc, _ := c["description"].(string)
		schema, _ := c["input_schema"].(map[string]any)
		out = append(out, llm.ToolDefinition{
			Type: "function",
			Function: llm.FunctionSpec{Name: name, Description: desc, Parameters: schema},
		})
	}
	return out
}

// recordUsage folds provider token stats into the rolling estimate.
func (r *Runtime) recordUsage(resp *llm.LLMResponse) {
	if resp == nil {
		return
	}
	r.lastPromptTokens = int64(resp.TokenUsage.PromptTokens)
	if r.lastPromptTokens > 0 {
		r.usageTokens = r.lastPromptTokens + int64(resp.TokenUsage.CompletionTokens)
	}
}

// usage tracking fields (guarded by single-turn execution per runtime).
