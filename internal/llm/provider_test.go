package llm

import (
	"context"
	"encoding/json"
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
