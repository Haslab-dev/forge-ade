package usage

import (
	"os"
	"testing"
	"time"
)

func TestUsageManagerRecordAndQuery(t *testing.T) {
	tmpDir := t.TempDir()
	um := NewUsageManager(tmpDir, nil)

	now := time.Now()

	// 1. Record turns for different models and providers at different times
	// Entry 1: 30 minutes ago (should match 1h, 1d, 7d, 1m)
	um.mu.Lock()
	um.entries = append(um.entries, UsageEntry{
		ID:           "test-1",
		Timestamp:    now.Add(-30 * time.Minute),
		SessionID:    "sess-1",
		Model:        "deepseek-v4-1-flash",
		Provider:     "DeepSeek",
		InputTokens:  1000,
		OutputTokens: 200,
		CachedTokens: 500,
		TotalTokens:  1200,
	})
	// Entry 2: 5 hours ago (matches 1d, 7d, 1m, but NOT 1h)
	um.entries = append(um.entries, UsageEntry{
		ID:           "test-2",
		Timestamp:    now.Add(-5 * time.Hour),
		SessionID:    "sess-2",
		Model:        "gemini-2.5-pro",
		Provider:     "Google Gemini",
		InputTokens:  2000,
		OutputTokens: 400,
		CachedTokens: 1000,
		TotalTokens:  2400,
	})
	// Entry 3: 3 days ago (matches 7d, 1m, but NOT 1h or 1d)
	um.entries = append(um.entries, UsageEntry{
		ID:           "test-3",
		Timestamp:    now.Add(-3 * 24 * time.Hour),
		SessionID:    "sess-3",
		Model:        "deepseek-v4-1-flash",
		Provider:     "DeepSeek",
		InputTokens:  3000,
		OutputTokens: 600,
		CachedTokens: 1500,
		TotalTokens:  3600,
	})
	// Entry 4: 20 days ago (matches 1m, but NOT 7d)
	um.entries = append(um.entries, UsageEntry{
		ID:           "test-4",
		Timestamp:    now.Add(-20 * 24 * time.Hour),
		SessionID:    "sess-4",
		Model:        "claude-3-7-sonnet",
		Provider:     "Anthropic",
		InputTokens:  4000,
		OutputTokens: 800,
		CachedTokens: 2000,
		TotalTokens:  4800,
	})
	um.mu.Unlock()

	// 2. Test 1h query
	s1h := um.Query(QueryOptions{TimeRange: TimeRange1H})
	if s1h.TotalTokens != 1200 {
		t.Fatalf("expected 1h total tokens 1200, got %d", s1h.TotalTokens)
	}
	if s1h.TotalInputTokens != 1000 || s1h.TotalOutputTokens != 200 || s1h.TotalCachedTokens != 500 {
		t.Fatalf("unexpected token breakdown: %+v", s1h)
	}
	if len(s1h.ByModel) != 1 || s1h.ByModel[0].Model != "deepseek-v4-1-flash" {
		t.Fatalf("unexpected ByModel for 1h: %+v", s1h.ByModel)
	}
	// Cache hit rate: 500 / 1000 = 50.0%
	if s1h.CacheHitRate < 49.9 || s1h.CacheHitRate > 50.1 {
		t.Fatalf("expected 50%% cache hit rate, got %.2f%%", s1h.CacheHitRate)
	}

	// 3. Test 1d query (includes test-1 and test-2)
	s1d := um.Query(QueryOptions{TimeRange: TimeRange1D})
	if s1d.TotalTokens != 3600 { // 1200 + 2400
		t.Fatalf("expected 1d total tokens 3600, got %d", s1d.TotalTokens)
	}
	if len(s1d.ByModel) != 2 {
		t.Fatalf("expected 2 models in 1d query, got %d", len(s1d.ByModel))
	}
	if len(s1d.ByProvider) != 2 {
		t.Fatalf("expected 2 providers in 1d query, got %d", len(s1d.ByProvider))
	}

	// 4. Test 7d query (includes test-1, test-2, test-3)
	s7d := um.Query(QueryOptions{TimeRange: TimeRange7D})
	if s7d.TotalTokens != 7200 { // 1200 + 2400 + 3600
		t.Fatalf("expected 7d total tokens 7200, got %d", s7d.TotalTokens)
	}

	// 5. Test 1 Month query (includes all 4 entries)
	s1m := um.Query(QueryOptions{TimeRange: TimeRange1Month})
	if s1m.TotalTokens != 12000 { // 1200 + 2400 + 3600 + 4800
		t.Fatalf("expected 1m total tokens 12000, got %d", s1m.TotalTokens)
	}

	// 6. Test Model filter (e.g. only deepseek-v4-1-flash in 1 Month)
	sMdl := um.Query(QueryOptions{TimeRange: TimeRange1Month, FilterModel: "deepseek-v4-1-flash"})
	if sMdl.TotalTokens != 4800 { // test-1 (1200) + test-3 (3600)
		t.Fatalf("expected deepseek total tokens 4800, got %d", sMdl.TotalTokens)
	}
	if len(sMdl.ByModel) != 1 {
		t.Fatalf("expected 1 model, got %d", len(sMdl.ByModel))
	}

	// 7. Test Provider filter (e.g. only Anthropic in 1 Month)
	sProv := um.Query(QueryOptions{TimeRange: TimeRange1Month, FilterProvider: "Anthropic"})
	if sProv.TotalTokens != 4800 {
		t.Fatalf("expected Anthropic total tokens 4800, got %d", sProv.TotalTokens)
	}

	// 8. Test Custom Date Range query
	sCustom := um.Query(QueryOptions{
		TimeRange:   TimeRangeCustom,
		CustomStart: now.Add(-6 * time.Hour),
		CustomEnd:   now.Add(-1 * time.Hour),
	})
	// Only test-2 was 5 hours ago
	if sCustom.TotalTokens != 2400 {
		t.Fatalf("expected custom range total tokens 2400, got %d", sCustom.TotalTokens)
	}
}

func TestUsageLedgerPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	um1 := NewUsageManager(tmpDir, nil)

	um1.RecordTurn("sess-1", "Coding task", "deepseek-v4-1-flash", "DeepSeek", 500, 100, 200, 600, 0.002)

	// Force save directly
	um1.mu.Lock()
	um1.dirty = true
	um1.saveLedgerLocked()
	um1.mu.Unlock()

	// Create new manager instance from same directory and verify loaded
	um2 := NewUsageManager(tmpDir, nil)
	s := um2.Query(QueryOptions{TimeRange: TimeRange1D})
	if s.TotalTokens != 600 {
		t.Fatalf("expected 600 total tokens after reload, got %d", s.TotalTokens)
	}
	if s.TotalInputTokens != 500 || s.TotalOutputTokens != 100 || s.TotalCachedTokens != 200 {
		t.Fatalf("unexpected token fields after reload: %+v", s)
	}
}

func TestBackfillHistoricalSessions(t *testing.T) {
	tmpDir := t.TempDir()
	sessDir := t.TempDir()

	// Write mock session file
	rawJSON := `{
		"id": "session-mock-123",
		"title": "Mock Design Task",
		"model": "glm-5.4",
		"provider": "prov-1794969055943",
		"createdAt": "` + time.Now().Add(-2*time.Hour).Format(time.RFC3339) + `",
		"lastUsage": {
			"inputTokens": 1500,
			"outputTokens": 300,
			"cachedTokens": 600,
			"totalTokens": 1800,
			"costUsd": 0.005
		}
	}`
	mockFilePath := sessDir + "/session-mock-123.json"
	if err := os.WriteFile(mockFilePath, []byte(rawJSON), 0644); err != nil {
		t.Fatalf("failed to write mock session: %v", err)
	}

	um := NewUsageManager(tmpDir, func(model, provID string) string {
		if provID == "prov-1794969055943" {
			return "Zhipu AI (GLM)"
		}
		return ""
	})

	um.BackfillHistoricalSessions(sessDir)

	summary := um.Query(QueryOptions{TimeRange: TimeRange1D})
	if summary.TotalTokens != 1800 {
		t.Fatalf("expected 1800 total tokens from backfill, got %d", summary.TotalTokens)
	}
	if summary.TotalInputTokens != 1500 || summary.TotalOutputTokens != 300 || summary.TotalCachedTokens != 600 {
		t.Fatalf("unexpected token values: %+v", summary)
	}
	if len(summary.ByModel) != 1 || summary.ByModel[0].Model != "glm-5.4" {
		t.Fatalf("unexpected byModel: %+v", summary.ByModel)
	}
	if len(summary.ByProvider) != 1 || summary.ByProvider[0].Provider != "Zhipu AI (GLM)" {
		t.Fatalf("unexpected byProvider: %+v", summary.ByProvider)
	}
}
