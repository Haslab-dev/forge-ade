package agent

import (
	"encoding/json"
	"time"
)

// ContentBlock is a single block in an agent message.
// AssistantMessage content blocks. A message may carry multiple blocks of the
// same or different types (text + thinking + tool_call interleaved).
type ContentBlock struct {
	// Type is one of: "text", "thinking", "tool_call", "tool_result".
	Type string `json:"type"`

	// Text content for "text" and "thinking" blocks.
	Text string `json:"text,omitempty"`

	// Tool-call fields (type == "tool_call").
	ToolCallID string         `json:"tool_call_id,omitempty"`
	Name       string         `json:"name,omitempty"`
	Arguments  map[string]any `json:"arguments,omitempty"`

	// Tool-result fields (type == "tool_result").
	IsError bool `json:"is_error,omitempty"`
}

// AgentMessage is a message in an agent session. Content is block-based.
type AgentMessage struct {
	ID        string         `json:"id"`
	Role      string         `json:"role"` // "user", "assistant", "system", "tool"
	Content   []ContentBlock `json:"content"`
	Timestamp time.Time      `json:"timestamp"`
}

// Text returns the concatenated text of all text blocks (used for the LLM
// message payload and for legacy rendering).
func (m *AgentMessage) Text() string {
	var sb []byte
	for _, b := range m.Content {
		if b.Type == "text" {
			sb = append(sb, b.Text...)
		}
	}
	return string(sb)
}

// Thinking returns the concatenated thinking blocks (used for the legacy
// reasoning rendering and for the LLM payload when supported).
func (m *AgentMessage) Thinking() string {
	var sb []byte
	for _, b := range m.Content {
		if b.Type == "thinking" {
			sb = append(sb, b.Text...)
		}
	}
	return string(sb)
}

// ToolCallBlocks returns all tool_call blocks in the message.
func (m *AgentMessage) ToolCallBlocks() []ContentBlock {
	var out []ContentBlock
	for _, b := range m.Content {
		if b.Type == "tool_call" {
			out = append(out, b)
		}
	}
	return out
}

// ToolResultBlocks returns all tool_result blocks in the message.
func (m *AgentMessage) ToolResultBlocks() []ContentBlock {
	var out []ContentBlock
	for _, b := range m.Content {
		if b.Type == "tool_result" {
			out = append(out, b)
		}
	}
	return out
}

func jsonUnmarshalAny(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func jsonMarshalAny(v any) ([]byte, error) {
	return json.Marshal(v)
}
