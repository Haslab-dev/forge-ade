package quota

import (
	"encoding/json"
	"testing"
)

func TestNormalizeSubscriptionTier(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"free-tier", "FREE"},
		{"Antigravity Starter Quota", "FREE"},
		{"standard-tier", "FREE"},
		{"g1-pro-tier", "PRO"},
		{"Google AI Pro", "PRO"},
		{"pro", "PRO"},
		{"g1-ultra-tier", "ULTRA"},
		{"Google AI Ultra", "ULTRA"},
		{"helium", "ULTRA"},
		{"ultra", "ULTRA"},
		{"enterprise-tier", "ENTERPRISE"},
		{"", "FREE"},
	}

	for _, tc := range tests {
		got := NormalizeSubscriptionTier(tc.input)
		if got != tc.expected {
			t.Errorf("NormalizeSubscriptionTier(%q) = %q, expected %q", tc.input, got, tc.expected)
		}
	}
}

func TestParseAccountInput(t *testing.T) {
	// Raw token
	ref, acc, idTok, err := ParseAccountInput("1//04test_refresh_token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref != "1//04test_refresh_token" || acc != "" || idTok != "" {
		t.Errorf("unexpected parsed values: ref=%q, acc=%q, idTok=%q", ref, acc, idTok)
	}

	// JSON credentials format 1
	json1 := `{"refresh_token": "1//json_ref", "access_token": "ya29.json_acc", "id_token": "jwt.header.payload"}`
	ref, acc, idTok, err = ParseAccountInput(json1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref != "1//json_ref" || acc != "ya29.json_acc" || idTok != "jwt.header.payload" {
		t.Errorf("unexpected parsed values: ref=%q, acc=%q, idTok=%q", ref, acc, idTok)
	}

	// JSON credentials format 2 (nested token object)
	json2 := `{"token": {"refresh_token": "1//nested_ref", "access_token": "ya29.nested_acc"}, "id_token": "jwt.nested"}`
	ref, acc, idTok, err = ParseAccountInput(json2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref != "1//nested_ref" || acc != "ya29.nested_acc" || idTok != "jwt.nested" {
		t.Errorf("unexpected parsed values: ref=%q, acc=%q, idTok=%q", ref, acc, idTok)
	}

	// Empty input
	_, _, _, err = ParseAccountInput("   ")
	if err == nil {
		t.Errorf("expected error for empty input")
	}
}

func TestAccountManagerCRUD(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewAccountManager(tmpDir)

	if len(mgr.Accounts()) != 0 {
		t.Fatalf("expected 0 accounts initially, got %d", len(mgr.Accounts()))
	}

	// Add an account manually
	acc1 := &AntigravityAccount{
		ID:           "test1@example.com",
		Email:        "test1@example.com",
		Label:        "Work",
		RefreshToken: "1//refresh_1",
		Tier:         "PRO",
		Status:       "ok",
		Quota: &AntigravityAccountQuota{
			Gemini5hPercent:     85.5,
			Gemini5hReset:       "Resets in 2h 10m",
			GeminiWeeklyPercent: 92.0,
			GeminiWeeklyReset:   "Quota available",
			Claude5hPercent:     70.0,
			Claude5hReset:       "Resets in 1h 30m",
			ClaudeWeeklyPercent: 88.0,
			ClaudeWeeklyReset:   "Quota available",
			QuotaKnown:          true,
		},
	}

	mgr.mu.Lock()
	mgr.accounts = append(mgr.accounts, acc1)
	if err := mgr.saveLocked(); err != nil {
		mgr.mu.Unlock()
		t.Fatalf("failed to save: %v", err)
	}
	mgr.mu.Unlock()

	// Verify retrieval
	got, ok := mgr.GetAccount("test1@example.com")
	if !ok || got.Email != "test1@example.com" {
		t.Fatalf("expected to get account test1@example.com, got %v", got)
	}
	if got.Tier != "PRO" || got.Quota == nil || got.Quota.Gemini5hPercent != 85.5 {
		t.Errorf("unexpected account data: %+v", got)
	}

	// Update label
	if err := mgr.UpdateLabel("test1@example.com", "Work Account"); err != nil {
		t.Fatalf("failed to update label: %v", err)
	}
	got, _ = mgr.GetAccount("test1@example.com")
	if got.Label != "Work Account" {
		t.Errorf("expected label 'Work Account', got %q", got.Label)
	}

	// Add second account
	acc2 := &AntigravityAccount{
		ID:           "test2@example.com",
		Email:        "test2@example.com",
		Label:        "Personal",
		RefreshToken: "1//refresh_2",
		Tier:         "ULTRA",
		Status:       "ok",
	}
	mgr.mu.Lock()
	mgr.accounts = append(mgr.accounts, acc2)
	_ = mgr.saveLocked()
	mgr.mu.Unlock()

	if len(mgr.Accounts()) != 2 {
		t.Errorf("expected 2 accounts, got %d", len(mgr.Accounts()))
	}

	// Set active
	if err := mgr.SetActive("test2@example.com"); err != nil {
		t.Fatalf("failed to set active: %v", err)
	}
	a1, _ := mgr.GetAccount("test1@example.com")
	a2, _ := mgr.GetAccount("test2@example.com")
	if a1.IsActive || !a2.IsActive {
		t.Errorf("expected acc2 active, got a1.IsActive=%v, a2.IsActive=%v", a1.IsActive, a2.IsActive)
	}

	// Reload from new manager instance to verify disk persistence
	mgr2 := NewAccountManager(tmpDir)
	if len(mgr2.Accounts()) != 2 {
		t.Fatalf("expected 2 accounts after reload, got %d", len(mgr2.Accounts()))
	}
	reloaded, _ := mgr2.GetAccount("test2@example.com")
	if reloaded.Tier != "ULTRA" || !reloaded.IsActive {
		t.Errorf("unexpected reloaded account: %+v", reloaded)
	}

	// Remove account
	if err := mgr2.RemoveAccount("test1@example.com"); err != nil {
		t.Fatalf("failed to remove account: %v", err)
	}
	if len(mgr2.Accounts()) != 1 {
		t.Errorf("expected 1 account after delete, got %d", len(mgr2.Accounts()))
	}
}

func TestAntigravityQuotaPayloads(t *testing.T) {
	// Verify JSON structure serialization
	q := &AntigravityAccountQuota{
		Gemini5hPercent:     90.0,
		Gemini5hReset:       "Quota available",
		GeminiWeeklyPercent: 95.0,
		GeminiWeeklyReset:   "Quota available",
		Claude5hPercent:     80.0,
		Claude5hReset:       "Resets in 3h",
		ClaudeWeeklyPercent: 85.0,
		ClaudeWeeklyReset:   "Quota available",
		QuotaKnown:          true,
		Models: []ModelQuotaItem{
			{Name: "gemini-2.5-pro", Percentage: 95.0, ResetTime: "2026-11-20T00:00:00Z"},
			{Name: "claude-3-7-sonnet", Percentage: 80.0, ResetTime: "2026-11-20T00:00:00Z"},
		},
	}

	data, err := json.Marshal(q)
	if err != nil {
		t.Fatalf("failed to marshal quota: %v", err)
	}

	var parsed AntigravityAccountQuota
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal quota: %v", err)
	}

	if parsed.Gemini5hPercent != 90.0 || len(parsed.Models) != 2 {
		t.Errorf("unexpected unmarshaled quota: %+v", parsed)
	}
}

func TestRefreshAccountSkipsInactiveWithoutNetwork(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewAccountManager(tmpDir)

	mgr.mu.Lock()
	mgr.accounts = []*AntigravityAccount{
		{ID: "inactive@example.com", Email: "inactive@example.com", IsActive: false, Status: "ok"},
	}
	_ = mgr.saveLocked()
	mgr.mu.Unlock()

	acc, err := mgr.RefreshAccount("inactive@example.com")
	if err != nil {
		t.Fatalf("unexpected error refreshing inactive account: %v", err)
	}
	if acc.Quota != nil {
		t.Errorf("expected inactive account refresh to skip network/CLI probe, got quota=%+v", acc.Quota)
	}
}

func TestSupportsArgonAndModelSorting(t *testing.T) {
	accWithoutArgon := &AntigravityAccount{
		Email: "free@example.com",
		Tier:  "FREE",
		Quota: &AntigravityAccountQuota{
			Gemini5hPercent: 100,
		},
	}
	if accWithoutArgon.SupportsArgon() {
		t.Errorf("expected accWithoutArgon.SupportsArgon() = false")
	}

	accWithArgon := &AntigravityAccount{
		Email: "work@example.com",
		Tier:  "PRO",
		Quota: &AntigravityAccountQuota{
			ArgonWeeklyPercent: 100,
			ArgonWeeklyReset:   "Quota available",
		},
	}
	if !accWithArgon.SupportsArgon() {
		t.Errorf("expected accWithArgon.SupportsArgon() = true")
	}

	models := []string{
		"gpt-oss-120b-medium",
		"gemini-3.8-flash-high",
		"gemini-4-argon-high",
		"claude-opus-5-5-high",
		"gemini-4-argon-medium",
		"gemini-4-neon-high",
	}
	sortAntigravityModels(models)
	if models[0] != "gemini-4-argon-high" || models[1] != "gemini-4-argon-medium" {
		t.Errorf("expected Argon models at the top, got %v", models)
	}
}
