// Package harness implements the ForgeADE agent harness in Go: an immutable turn
// state machine, message history, tool registry/scheduler/executor with
// permission gating, MCP, skills, subagents, and plugins.
package harness

import (
	"errors"
	"fmt"
	"time"
)

// Phase is a turn lifecycle phase. The set and transition table mirror
// the reference turn-state design exactly.
type Phase string

const (
	PhaseIdle               Phase = "idle"
	PhaseProcessingInput    Phase = "processing_input"
	PhaseAwaitingModel      Phase = "awaiting_model_response"
	PhaseStreaming          Phase = "streaming"
	PhaseSchedulingTools    Phase = "scheduling_tools"
	PhaseExecutingTools     Phase = "executing_tools"
	PhaseAggregatingResults Phase = "aggregating_results"
	PhaseAwaitingPermission Phase = "awaiting_permission"
	PhaseCompleting         Phase = "completing"
	PhaseError              Phase = "error"
)

func IsTerminalPhase(p Phase) bool {
	return p == PhaseCompleting || p == PhaseError
}

func IsWaitingPhase(p Phase) bool {
	return p == PhaseAwaitingModel || p == PhaseAwaitingPermission || p == PhaseExecutingTools
}

// ErrInvalidPhase is thrown on illegal transitions (CoreErrorType.InvalidTurnPhase).
var ErrInvalidPhase = errors.New("invalid turn phase")

// transitions is the legal transition table; port of turn-state.ts:230-263.
var transitions = map[Phase][]Phase{
	PhaseIdle:               {PhaseProcessingInput},
	PhaseProcessingInput:    {PhaseAwaitingModel, PhaseCompleting},
	PhaseAwaitingModel:      {PhaseStreaming, PhaseCompleting, PhaseError},
	PhaseStreaming:          {PhaseSchedulingTools, PhaseAggregatingResults, PhaseCompleting, PhaseError},
	PhaseSchedulingTools:    {PhaseExecutingTools, PhaseAwaitingPermission, PhaseError},
	PhaseExecutingTools:     {PhaseAggregatingResults, PhaseAwaitingPermission, PhaseError},
	PhaseAggregatingResults: {PhaseAwaitingModel, PhaseSchedulingTools, PhaseCompleting, PhaseError},
	PhaseAwaitingPermission: {PhaseExecutingTools, PhaseError},
	PhaseCompleting:         {PhaseIdle},
	PhaseError:              {PhaseIdle},
}

func canTransition(from, to Phase) bool {
	for _, p := range transitions[from] {
		if p == to {
			return true
		}
	}
	return false
}

// ToolCallStatus is the per-call lifecycle within a turn.
type ToolCallStatus string

const (
	CallScheduled         ToolCallStatus = "scheduled"
	CallWaitingPermission ToolCallStatus = "waiting_permission"
	CallPermissionDenied  ToolCallStatus = "permission_denied"
	CallRunning           ToolCallStatus = "running"
	CallCompleted         ToolCallStatus = "completed"
	CallFailed            ToolCallStatus = "failed"
)

// PermissionDecision mirrors turn-state.ts.
type PermissionDecision string

const (
	PermAllow   PermissionDecision = "allow"
	PermDeny    PermissionDecision = "deny"
	PermEscalate PermissionDecision = "escalate"
	PermModify  PermissionDecision = "modify"
)

// TurnResultType mirrors turn-state.ts.
type TurnResultType string

const (
	ResultSuccess           TurnResultType = "success"
	ResultCancelled         TurnResultType = "cancelled"
	ResultErrorMaxTurns     TurnResultType = "error_max_turns"
	ResultErrorMaxBudget    TurnResultType = "error_max_budget"
	ResultErrorExecution    TurnResultType = "error_during_execution"
	ResultErrorMaxToolCalls TurnResultType = "error_max_tool_calls"
)

// ToolCall is a model-requested tool invocation tracked by the machine.
type ToolCall struct {
	ID     string          `json:"id"`
	Name   string          `json:"name"`
	Input  map[string]any  `json:"input"`
	Status ToolCallStatus  `json:"status"`
}

// ToolResult is the outcome of one executed tool call.
type ToolResult struct {
	CallID     string `json:"call_id"`
	Success    bool   `json:"success"`
	Content    string `json:"content"`
	IsError    bool   `json:"is_error"`
	StopTurn   bool   `json:"stop_turn,omitempty"`
	FollowUpIn string `json:"follow_up_user_input,omitempty"`
}

// PermissionRequest tracks one gating ask.
type PermissionRequest struct {
	ID        string             `json:"id"`
	CallID    string             `json:"call_id"`
	ToolName  string             `json:"tool_name"`
	Input     map[string]any     `json:"input"`
	Decision  PermissionDecision `json:"decision,omitempty"`
	ModifiedInput map[string]any `json:"modified_input,omitempty"`
}

// ModelRequestInfo describes the in-flight model request.
type ModelRequestInfo struct {
	Model       string `json:"model"`
	StopReason  string `json:"stop_reason,omitempty"`
	InputTokens int64  `json:"input_tokens,omitempty"`
	OutputTokens int64 `json:"output_tokens,omitempty"`
}

// TurnState is an immutable value: every mutator returns a copy. Port of
// turn-state.ts TurnState.
type TurnState struct {
	ID                 string             `json:"id"`
	SessionID          string             `json:"session_id"`
	TurnNumber         int                `json:"turn_number"`
	Phase              Phase              `json:"phase"`
	Input              string             `json:"input"`
	ModelRequest       *ModelRequestInfo  `json:"model_request,omitempty"`
	StreamingContent   string             `json:"streaming_content,omitempty"`
	FinalResponse      string             `json:"final_response,omitempty"`
	ToolCalls          []ToolCall         `json:"tool_calls,omitempty"`
	ToolResults        []ToolResult       `json:"tool_results,omitempty"`
	PendingPermissions []PermissionRequest `json:"pending_permissions,omitempty"`
	ResolvedPermissions []PermissionRequest `json:"resolved_permissions,omitempty"`
	ResultType         TurnResultType     `json:"result_type,omitempty"`
	Error              string             `json:"error,omitempty"`
	StartedAt          time.Time          `json:"started_at"`
	CompletedAt        *time.Time         `json:"completed_at,omitempty"`
}

func (s TurnState) clone() TurnState {
	c := s
	c.ToolCalls = append([]ToolCall(nil), s.ToolCalls...)
	c.ToolResults = append([]ToolResult(nil), s.ToolResults...)
	c.PendingPermissions = append([]PermissionRequest(nil), s.PendingPermissions...)
	c.ResolvedPermissions = append([]PermissionRequest(nil), s.ResolvedPermissions...)
	return c
}

// transition moves to `to` if legal, returning an updated copy. Direct sets
// from any phase (fail) bypass this, as in the source.
func (s TurnState) transition(to Phase) (TurnState, error) {
	if !canTransition(s.Phase, to) {
		return s, fmt.Errorf("%w: %s -> %s", ErrInvalidPhase, s.Phase, to)
	}
	c := s.clone()
	c.Phase = to
	return c, nil
}

func (s TurnState) findCall(id string) *ToolCall {
	for i := range s.ToolCalls {
		if s.ToolCalls[i].ID == id {
			return &s.ToolCalls[i]
		}
	}
	return nil
}

// TurnMachine is a pure state machine over TurnState values. Port of
// turn-machine.ts.
type TurnMachine struct {
	state TurnState
}

func NewTurnMachine(sessionID string, turnNumber int, input string) *TurnMachine {
	return &TurnMachine{state: TurnState{
		ID:         newID(),
		SessionID:  sessionID,
		TurnNumber: turnNumber,
		Phase:      PhaseIdle,
		Input:      input,
		StartedAt:  time.Now(),
	}}
}

func (m *TurnMachine) State() TurnState { return m.state }

// start: idle -> processing_input.
func (m *TurnMachine) Start() error {
	st, err := m.state.transition(PhaseProcessingInput)
	if err != nil {
		return err
	}
	m.state = st
	return nil
}

// StartModelRequest: processing_input|aggregating_results -> awaiting_model_response.
// Allowed only from those two phases, checked separately from the table.
func (m *TurnMachine) StartModelRequest(model string) error {
	if m.state.Phase != PhaseProcessingInput && m.state.Phase != PhaseAggregatingResults {
		return fmt.Errorf("%w: startModelRequest from %s", ErrInvalidPhase, m.state.Phase)
	}
	st, err := m.state.transition(PhaseAwaitingModel)
	if err != nil {
		return err
	}
	st.ModelRequest = &ModelRequestInfo{Model: model}
	m.state = st
	return nil
}

// ReceiveModelResponse -> streaming; appends content.
func (m *TurnMachine) ReceiveModelResponse(content string) error {
	st, err := m.state.transition(PhaseStreaming)
	if err != nil {
		return err
	}
	st.StreamingContent += content
	m.state = st
	return nil
}

// AddStreamingContent appends content while already streaming.
func (m *TurnMachine) AddStreamingContent(content string) error {
	if m.state.Phase != PhaseStreaming {
		return fmt.Errorf("%w: addStreamingContent from %s", ErrInvalidPhase, m.state.Phase)
	}
	st := m.state.clone()
	st.StreamingContent += content
	m.state = st
	return nil
}

// ScheduleTools: streaming -> scheduling_tools; every call becomes scheduled.
func (m *TurnMachine) ScheduleTools(calls []ToolCall) error {
	st, err := m.state.transition(PhaseSchedulingTools)
	if err != nil {
		return err
	}
	for i := range calls {
		calls[i].Status = CallScheduled
	}
	st.ToolCalls = append(st.ToolCalls, calls...)
	m.state = st
	return nil
}

// StartToolExecution: -> awaiting_permission if any call waits, else executing_tools.
// Non-permission calls become running.
func (m *TurnMachine) StartToolExecution() error {
	anyWaiting := false
	st := m.state.clone()
	for i := range st.ToolCalls {
		switch st.ToolCalls[i].Status {
		case CallWaitingPermission:
			anyWaiting = true
		case CallScheduled:
			st.ToolCalls[i].Status = CallRunning
		}
	}
	to := PhaseExecutingTools
	if anyWaiting {
		to = PhaseAwaitingPermission
	}
	st2, err := st.transition(to)
	if err != nil {
		return err
	}
	m.state = st2
	return nil
}

// CompleteTool marks a call completed/failed and appends its result. No phase
// transition, mirroring turn-machine.ts completeTool.
func (m *TurnMachine) CompleteTool(callID string, r ToolResult) error {
	st := m.state.clone()
	call := st.findCall(callID)
	if call == nil {
		return fmt.Errorf("unknown tool call %s", callID)
	}
	if r.Success {
		call.Status = CallCompleted
	} else {
		call.Status = CallFailed
		if r.Content == "" {
			r.Content = `{"type":"tool_error","recoverable":true}`
		}
	}
	r.CallID = callID
	st.ToolResults = append(st.ToolResults, r)
	m.state = st
	return nil
}

// RequestPermission: -> awaiting_permission; that call becomes waiting_permission.
func (m *TurnMachine) RequestPermission(req PermissionRequest) error {
	st, err := m.state.transition(PhaseAwaitingPermission)
	if err != nil {
		return err
	}
	call := st.findCall(req.CallID)
	if call == nil {
		return fmt.Errorf("unknown tool call %s", req.CallID)
	}
	call.Status = CallWaitingPermission
	st.PendingPermissions = append(st.PendingPermissions, req)
	m.state = st
	return nil
}

// ResolvePermission: deny => permission_denied; modify may replace input;
// request moves to resolvedPermissions.
func (m *TurnMachine) ResolvePermission(reqID string, decision PermissionDecision, modifiedInput map[string]any) error {
	st := m.state.clone()
	idx := -1
	for i := range st.PendingPermissions {
		if st.PendingPermissions[i].ID == reqID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("unknown permission request %s", reqID)
	}
	req := st.PendingPermissions[idx]
	st.PendingPermissions = append(st.PendingPermissions[:idx], st.PendingPermissions[idx+1:]...)
	req.Decision = decision
	req.ModifiedInput = modifiedInput
	if decision == PermDeny {
		if call := st.findCall(req.CallID); call != nil {
			call.Status = CallPermissionDenied
		}
	} else {
		// allow/escalate/modify: back to scheduled so StartToolExecution
		// picks it up as running.
		if call := st.findCall(req.CallID); call != nil {
			call.Status = CallScheduled
		}
		if decision == PermModify && modifiedInput != nil {
			if call := st.findCall(req.CallID); call != nil {
				call.Input = modifiedInput
			}
		}
	}
	st.ResolvedPermissions = append(st.ResolvedPermissions, req)
	m.state = st
	return nil
}

// AggregateResults: executing_tools -> aggregating_results.
func (m *TurnMachine) AggregateResults() error {
	st, err := m.state.transition(PhaseAggregatingResults)
	if err != nil {
		return err
	}
	m.state = st
	return nil
}

// Complete: -> completing, then may reset to idle.
func (m *TurnMachine) Complete(response string, resultType TurnResultType) error {
	st, err := m.state.transition(PhaseCompleting)
	if err != nil {
		return err
	}
	now := time.Now()
	st.FinalResponse = response
	st.ResultType = resultType
	st.CompletedAt = &now
	m.state = st
	return nil
}

// Fail bypasses the transition table (sets error directly), as in the source.
func (m *TurnMachine) Fail(err error) {
	st := m.state.clone()
	st.Phase = PhaseError
	st.Error = err.Error()
	st.ResultType = ResultErrorExecution
	now := time.Now()
	st.CompletedAt = &now
	m.state = st
}

// Reset: completing|error -> idle.
func (m *TurnMachine) Reset() error {
	st, err := m.state.transition(PhaseIdle)
	if err != nil {
		return err
	}
	m.state = st
	return nil
}

// NextPhase is the next-phase oracle (port of turn-machine.ts:308-339).
func (m *TurnMachine) NextPhase() (Phase, bool) {
	s := m.state
	switch s.Phase {
	case PhaseStreaming:
		if len(s.ToolCalls) > 0 {
			return PhaseSchedulingTools, true
		}
		if s.StreamingContent != "" {
			return PhaseCompleting, true
		}
	case PhaseExecutingTools:
		busy := false
		for _, c := range s.ToolCalls {
			if c.Status == CallRunning || c.Status == CallWaitingPermission {
				busy = true
				break
			}
		}
		if !busy {
			return PhaseAggregatingResults, true
		}
	case PhaseAggregatingResults:
		anyFailed := false
		for _, c := range s.ToolCalls {
			if c.Status == CallFailed || c.Status == CallPermissionDenied {
				anyFailed = true
				break
			}
		}
		if anyFailed {
			return PhaseCompleting, true
		}
		return PhaseAwaitingModel, true
	}
	return "", false
}
