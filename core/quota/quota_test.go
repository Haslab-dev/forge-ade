package quota

import (
	"testing"
)

func TestFormatAntigravityResetDuration(t *testing.T) {
	if got := FormatAntigravityResetDuration(1.0, "2026-10-22T20:00:00Z"); got != "Quota available" {
		t.Errorf("expected 'Quota available' for 100%% remaining, got %q", got)
	}
	if got := FormatAntigravityResetDuration(0.5, ""); got != "Quota available" {
		t.Errorf("expected 'Quota available' for empty reset time, got %q", got)
	}
	if got := FormatAntigravityResetDuration(0.5, "invalid-time"); got != "Quota available" {
		t.Errorf("expected 'Quota available' for invalid time, got %q", got)
	}
}

func TestFormatDurationMillis(t *testing.T) {
	if got := FormatDurationMillis(0); got != "soon" {
		t.Errorf("expected 'soon' for 0 millis, got %q", got)
	}
	if got := FormatDurationMillis(-100); got != "soon" {
		t.Errorf("expected 'soon' for negative millis, got %q", got)
	}
}

func TestGetAgentQuotaFallback(t *testing.T) {
	res, err := GetAgentQuota("custom-agent", func() map[string]interface{} {
		return map[string]interface{}{
			"totals": map[string]interface{}{
				"promptTokens": 100,
			},
		}
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res["agentId"] != "custom-agent" {
		t.Errorf("expected agentId custom-agent, got %v", res["agentId"])
	}
	totals, ok := res["totals"].(map[string]interface{})
	if !ok || totals["promptTokens"] != 100 {
		t.Errorf("expected promptTokens 100, got %v", res["totals"])
	}
}
