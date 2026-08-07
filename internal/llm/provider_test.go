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
