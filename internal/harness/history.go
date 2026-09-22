package harness

import (
	"time"

	"github.com/google/uuid"
)

func newID() string { return uuid.NewString() }

// EntryKind distinguishes canonical messages from attachments (system
// reminders injected into the request only).
type EntryKind string

const (
	EntryKindMessage    EntryKind = "message"
	EntryKindAttachment EntryKind = "attachment"
)

// EntrySource tracks provenance: real_user, legacy_synthetic, shared_context,
// plus system-reminder sources (runtime_mode, todo_reminder, ...).
type EntrySource string

// Message roles.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
	RoleSystem    Role = "system"
)

// ToolCall as serialized on an assistant message.
type MessageToolCall struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	ArgsJSON string         `json:"arguments"`
}

// ModelInputMessage is the provider-facing message shape.
type ModelInputMessage struct {
	Role        Role             `json:"role"`
	Content     string           `json:"content"`
	ToolCalls   []MessageToolCall `json:"tool_calls,omitempty"`
	ToolCallID  string           `json:"tool_call_id,omitempty"`
	ToolName    string           `json:"tool_name,omitempty"`
	IsError     bool             `json:"is_error,omitempty"`
	ProviderID  string           `json:"provider_id,omitempty"`
	ModelID     string           `json:"model_id,omitempty"`
	Tokens      int64            `json:"tokens,omitempty"`
	CacheControl string          `json:"cache_control,omitempty"`
}

// HistoryEntry is one entry in the message history.
type HistoryEntry struct {
	Kind      EntryKind         `json:"kind"`
	Message   *ModelInputMessage `json:"message,omitempty"`
	// Attachment carries <system-reminder> content injected per-request or
	// persisted (source decides).
	Attachment string           `json:"attachment,omitempty"`
	Source    EntrySource       `json:"source"`
	Tokens    int64             `json:"tokens,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
}

// MessageHistory is the canonical conversation log. Port of
// core/src/agent/message-history.ts: addUser/addAssistant/addToolResult/
// addAttachment/replaceMessages.
type MessageHistory struct {
	entries []HistoryEntry
}

func NewMessageHistory() *MessageHistory { return &MessageHistory{} }

func (h *MessageHistory) Entries() []HistoryEntry {
	return append([]HistoryEntry(nil), h.entries...)
}

func (h *MessageHistory) Len() int { return len(h.entries) }

func (h *MessageHistory) add(e HistoryEntry) {
	e.CreatedAt = time.Now()
	h.entries = append(h.entries, e)
}

func (h *MessageHistory) AddUser(content string, source EntrySource) {
	h.add(HistoryEntry{Kind: EntryKindMessage, Source: source, Message: &ModelInputMessage{Role: RoleUser, Content: content}})
}

// AddAttachment injects a system-reminder style entry. Persisted sources
// survive resume; per-request sources are stripped before persistence by the
// caller.
func (h *MessageHistory) AddAttachment(content string, source EntrySource, persisted bool) {
	h.add(HistoryEntry{Kind: EntryKindAttachment, Source: source, Attachment: content})
}

func (h *MessageHistory) AddAssistant(content string, toolCalls []MessageToolCall, modelID string, tokens int64) {
	h.add(HistoryEntry{Kind: EntryKindMessage, Source: "model", Message: &ModelInputMessage{
		Role: RoleAssistant, Content: content, ToolCalls: toolCalls, ModelID: modelID, Tokens: tokens,
	}})
}

func (h *MessageHistory) AddToolResult(callID, toolName, content string, success bool) {
	h.add(HistoryEntry{Kind: EntryKindMessage, Source: "tool", Message: &ModelInputMessage{
		Role: RoleTool, Content: content, ToolCallID: callID, ToolName: toolName, IsError: !success,
	}})
}

// ReplaceMessages swaps canonical history (compact/rewind).
func (h *MessageHistory) ReplaceMessages(entries []HistoryEntry) {
	h.entries = append([]HistoryEntry(nil), entries...)
}

// TruncateTo drops everything after the given number of entries (rewind).
func (h *MessageHistory) TruncateTo(n int) {
	if n >= 0 && n <= len(h.entries) {
		h.entries = h.entries[:n]
	}
}

// TokenEstimate is a cheap char/4 estimate used when provider usage is absent.
func (h *MessageHistory) TokenEstimate() int64 {
	var total int64
	for _, e := range h.entries {
		switch {
		case e.Message != nil:
			total += int64(len(e.Message.Content))/4 + e.Message.Tokens
		case e.Attachment != "":
			total += int64(len(e.Attachment)) / 4
		}
	}
	return total
}

// ProviderMessages projects history to the wire format, honoring cache-control
// anchors applied AFTER user reordering (see spec §2.2).
type ProviderMessage struct {
	Role       Role              `json:"role"`
	Content    string            `json:"content"`
	ToolCalls  []MessageToolCall `json:"tool_calls,omitempty"`
	ToolCallID string            `json:"tool_call_id,omitempty"`
	IsError    bool              `json:"is_error,omitempty"`
}

func (h *MessageHistory) ProviderMessages() []ProviderMessage {
	out := make([]ProviderMessage, 0, len(h.entries))
	for _, e := range h.entries {
		if e.Kind != EntryKindMessage || e.Message == nil {
			// attachments render as user-role system-reminder wrappers
			if e.Attachment != "" {
				out = append(out, ProviderMessage{Role: RoleUser, Content: systemReminder(e.Attachment)})
			}
			continue
		}
		m := e.Message
		out = append(out, ProviderMessage{
			Role: m.Role, Content: m.Content, ToolCalls: m.ToolCalls,
			ToolCallID: m.ToolCallID, IsError: m.IsError,
		})
	}
	return out
}

func systemReminder(content string) string {
	return "<system-reminder>" + content + "</system-reminder>"
}
