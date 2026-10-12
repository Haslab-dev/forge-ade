package llm

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

// TestTokenStatsDeepSeekCache verifies DeepSeek's cache accounting fields
// (prompt_cache_hit_tokens / prompt_cache_miss_tokens) decode correctly — they
// are what the agent header's cache-hit % reads from.
func TestTokenStatsDeepSeekCache(t *testing.T) {
	payload := `{"prompt_tokens":100,"completion_tokens":20,"prompt_cache_hit_tokens":80,"prompt_cache_miss_tokens":20,"total_tokens":120}`
	var s TokenStats
	if err := json.Unmarshal([]byte(payload), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s.PromptTokens != 100 || s.PromptCacheHitTokens != 80 || s.PromptCacheMissTokens != 20 {
		t.Fatalf("bad decode: %+v", s)
	}
	// Round-trip: persisted session usage must keep hit/miss.
	out, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var s2 TokenStats
	if err := json.Unmarshal(out, &s2); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	if s2.PromptCacheHitTokens != 80 {
		t.Fatalf("hit tokens lost in round-trip: %+v", s2)
	}
}

// TestParseSSEStreamNonStreamingFallback verifies that a provider which replies
// in SSE format even when asked for a non-streaming response is parsed
// correctly — this is what AI commit generation hit as "unmarshal response:
// unexpected json input".
func TestParseSSEStreamNonStreamingFallback(t *testing.T) {
	body := "data: {\"choices\":[{\"delta\":{\"content\":\"docs\"}}]}\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\"(readme):\"}}]}\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\" update docs\"}}]}\n" +
		"data: {\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":5,\"total_tokens\":15}}\n" +
		"data: [DONE]\n"

	resp, err := parseSSEStream(context.Background(), strings.NewReader(body), nil, nil)
	if err != nil {
		t.Fatalf("parseSSEStream: %v", err)
	}
	if resp.Content != "docs(readme): update docs" {
		t.Fatalf("bad content: %q", resp.Content)
	}
	if resp.TokenUsage.TotalTokens != 15 {
		t.Fatalf("bad usage: %+v", resp.TokenUsage)
	}
}

// TestSyncProviderProfiles pins the merge semantics that keep the internal
// agent authenticated: frontend (localStorage) profiles merge into the
// settings-configured list by ID first, then base URL, without clobbering
// existing keys or IDs.
func TestSyncProviderProfiles(t *testing.T) {
	dir := t.TempDir()
	c := NewLLMClient(dir)

	// Settings-configured profile (its own ID, has a key).
	c.profiles = []ProviderProfile{{
		ID:             "openai",
		Name:           "OpenAI",
		APIKey:         "sk-settings",
		BaseURL:        "https://api.openai.com/v1",
		Enabled:        true,
		SelectedModels: []string{"gpt-4o"},
	}}

	// Frontend list: same base URL under a different ID, now with a key.
	merged, err := c.SyncProviderProfiles([]ProviderProfile{{
		ID:             "prov-openai",
		Name:           "OpenAI",
		APIKey:         "sk-frontend",
		BaseURL:        "https://api.openai.com/v1",
		Enabled:        true,
		SelectedModels: []string{"gpt-4o", "gpt-4o-mini"},
	}})
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if len(merged) != 1 {
		t.Fatalf("expected merge by base URL (1 profile), got %d: %+v", len(merged), merged)
	}
	p := merged[0]
	if p.ID != "openai" {
		t.Fatalf("matched profile must keep its settings ID, got %q", p.ID)
	}
	if p.APIKey != "sk-frontend" {
		t.Fatalf("frontend key should win, got %q", p.APIKey)
	}
	if len(p.SelectedModels) != 2 {
		t.Fatalf("selected models not updated: %+v", p.SelectedModels)
	}

	// A keyless incoming entry must not wipe an existing key.
	merged, err = c.SyncProviderProfiles([]ProviderProfile{{
		ID:      "openai",
		BaseURL: "https://api.openai.com/v1",
		Enabled: true,
	}})
	if err != nil {
		t.Fatalf("sync 2: %v", err)
	}
	if merged[0].APIKey != "sk-frontend" {
		t.Fatalf("keyless sync wiped existing key: %q", merged[0].APIKey)
	}

	// Unknown provider (no ID/URL match) is appended.
	merged, err = c.SyncProviderProfiles([]ProviderProfile{{
		ID: "prov-custom", Name: "Custom", APIKey: "k", BaseURL: "https://api.custom.example/v1", Enabled: true,
		SelectedModels: []string{"m1"},
	}})
	if err != nil {
		t.Fatalf("sync 3: %v", err)
	}
	if len(merged) != 2 {
		t.Fatalf("expected appended profile, got %d", len(merged))
	}
	if merged[1].ID != "prov-custom" || merged[1].APIKey != "k" {
		t.Fatalf("appended profile mismatch: %+v", merged[1])
	}
}

// TestSyncProviderProfilesHealsKeylessDefault: a fresh client (default openai
// profile, no key) adopts the first enabled keyful profile after a sync
// instead of staying on the keyless default that produced opaque 401s.
func TestSyncProviderProfilesHealsKeylessDefault(t *testing.T) {
	dir := t.TempDir()
	c := NewLLMClient(dir) // active: openai default, empty key

	_, err := c.SyncProviderProfiles([]ProviderProfile{{
		ID: "prov-ds", Name: "DeepSeek", APIKey: "ds-key",
		BaseURL: "https://api.deepseek.com/v1", Enabled: true,
		SelectedModels: []string{"deepseek-chat"},
	}})
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if c.apiKey != "ds-key" || c.providerID != "prov-ds" {
		t.Fatalf("keyless default not healed: provider=%q key=%q", c.providerID, c.apiKey)
	}
	if c.baseURL != "https://api.deepseek.com/v1" || c.model != "deepseek-chat" {
		t.Fatalf("active view not adopted: %q %q", c.baseURL, c.model)
	}
}

// TestParseSSEStreamChunkCallbacks verifies that chunks are delivered incrementally
// to the onChunk callback as they arrive through the reader.
func TestParseSSEStreamChunkCallbacks(t *testing.T) {
	pr, pw := io.Pipe()
	var chunks []string
	done := make(chan struct{})

	go func() {
		defer close(done)
		_, _ = parseSSEStream(context.Background(), pr, func(deltaContent string, deltaReasoning string) {
			if deltaContent != "" {
				chunks = append(chunks, deltaContent)
			}
		}, nil)
	}()

	// Write first chunk
	_, _ = pw.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"Hello \"}}]}\n\n"))
	// Write second chunk
	_, _ = pw.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"world!\"}}]}\n\n"))
	// Write done
	_, _ = pw.Write([]byte("data: [DONE]\n\n"))
	_ = pw.Close()

	<-done

	if len(chunks) != 2 || chunks[0] != "Hello " || chunks[1] != "world!" {
		t.Fatalf("expected 2 incremental chunks ['Hello ', 'world!'], got: %+v", chunks)
	}
}

func TestSessionIDContext(t *testing.T) {
	ctx := context.Background()
	if id := SessionIDFrom(ctx); id != "" {
		t.Errorf("expected empty session ID, got %q", id)
	}

	withID := WithSessionID(ctx, "session-xyz-123")
	if id := SessionIDFrom(withID); id != "session-xyz-123" {
		t.Errorf("expected 'session-xyz-123', got %q", id)
	}
}

func TestPreparePayloadWithCache(t *testing.T) {
	messages := []LLMMessage{
		{Role: RoleSystem, Content: "You are an AI assistant."},
		{Role: RoleUser, Content: "Hello world"},
	}
	tools := []ToolDefinition{
		{
			Type: "function",
			Function: FunctionSpec{
				Name:        "read_file",
				Description: "Read file",
			},
		},
		{
			Type: "function",
			Function: FunctionSpec{
				Name:        "write_file",
				Description: "Write file",
			},
		},
	}

	// 1. Non-caching provider (e.g. OpenAI) should return plain structs without cache_control
	resMsgs, resTools := PreparePayloadWithCache("openai", "gpt-4o", messages, tools)
	if _, ok := resTools.([]ToolDefinition); !ok {
		t.Fatalf("expected untouched ToolDefinition slice for openai")
	}
	if _, ok := resMsgs.([]LLMMessage); !ok {
		t.Fatalf("expected untouched LLMMessage slice for openai")
	}

	// 2. Anthropic / OpenRouter claude should inject cache_control breakpoints
	cMsgs, cTools := PreparePayloadWithCache("openrouter", "anthropic/claude-3.5-sonnet", messages, tools)
	toolSlice, ok := cTools.([]map[string]any)
	if !ok {
		t.Fatalf("expected []map[string]any for tools, got %T", cTools)
	}
	if len(toolSlice) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(toolSlice))
	}
	// Breakpoint 1: Last tool should have cache_control
	lastTool := toolSlice[1]
	if cc, has := lastTool["cache_control"]; !has || cc == nil {
		t.Errorf("expected last tool to have cache_control, got %+v", lastTool)
	}
	firstTool := toolSlice[0]
	if _, has := firstTool["cache_control"]; has {
		t.Errorf("expected first tool NOT to have cache_control")
	}

	// Breakpoint 2 & 4: System and conversation tail should have cache_control
	msgSlice, ok := cMsgs.([]map[string]any)
	if !ok {
		t.Fatalf("expected []map[string]any for messages, got %T", cMsgs)
	}
	if len(msgSlice) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgSlice))
	}
	// System message content parts
	sysParts, ok := msgSlice[0]["content"].([]map[string]any)
	if !ok || len(sysParts) == 0 || sysParts[0]["cache_control"] == nil {
		t.Errorf("expected system message to have cache_control content part, got %+v", msgSlice[0])
	}
	// User message content parts (conversation tail)
	userParts, ok := msgSlice[1]["content"].([]map[string]any)
	if !ok || len(userParts) == 0 || userParts[0]["cache_control"] == nil {
		t.Errorf("expected user message tail to have cache_control content part, got %+v", msgSlice[1])
	}
}
