package llm

import (
	"encoding/json"
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
