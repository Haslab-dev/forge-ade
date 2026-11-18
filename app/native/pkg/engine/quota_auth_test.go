package engine

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// No stored Agy login: the quota fetch must return the honest unavailable
// payload and must NOT run the live fetch (which would spawn the agy CLI and
// could launch its OAuth login page — the user-reported bug).
func TestComputeAgentQuotaUnavailableWithoutAgyAuth(t *testing.T) {
	fetchCalled := false
	fetch := func() (map[string]interface{}, error) {
		fetchCalled = true
		return map[string]interface{}{"weekly": map[string]interface{}{"remainingPercent": 100.0}}, nil
	}

	res := computeAgentQuota("agent-agy", AgyAuthStatus{}, fetch, nil)

	if fetchCalled {
		t.Fatalf("live quota fetch must not run without an authenticated Agy login")
	}
	if known, _ := res["quotaKnown"].(bool); known {
		t.Fatalf("expected quotaKnown=false, got %v", res)
	}
	if _, hasWeekly := res["weekly"]; hasWeekly {
		t.Fatalf("unavailable payload must carry no window data, got %v", res)
	}
	if acc, _ := res["account"].(string); acc != "" {
		t.Fatalf("unavailable payload must not invent an account, got %q", acc)
	}
}

// Authenticated: the live fetch runs, its result is flagged known, and the
// account is pinned to the snapshot of the credentials the running Agy
// session holds (even when the fetch could not decode one).
func TestComputeAgentQuotaAuthenticatedPinsAccount(t *testing.T) {
	fetchCalls := 0
	fetch := func() (map[string]interface{}, error) {
		fetchCalls++
		return map[string]interface{}{
			"account": "",
			"weekly":  map[string]interface{}{"remainingPercent": 87.5},
		}, nil
	}
	auth := AgyAuthStatus{Authenticated: true, Account: "dev@example.com", Source: "file"}

	res := computeAgentQuota("agent-agy", auth, fetch, nil)

	if fetchCalls != 1 {
		t.Fatalf("expected exactly one live fetch, got %d", fetchCalls)
	}
	if known, _ := res["quotaKnown"].(bool); !known {
		t.Fatalf("expected quotaKnown=true, got %v", res)
	}
	if acc, _ := res["account"].(string); acc != "dev@example.com" {
		t.Fatalf("expected account pinned from auth snapshot, got %q", acc)
	}
	// A decoded account from the fetch itself is never overwritten.
	res = computeAgentQuota("agent-agy", auth, func() (map[string]interface{}, error) {
		return map[string]interface{}{"account": "fetched@example.com"}, nil
	}, nil)
	if acc, _ := res["account"].(string); acc != "fetched@example.com" {
		t.Fatalf("expected fetched account preserved, got %q", acc)
	}
}

// Authenticated but the live fetch fails: still honest — unavailable windows,
// account retained.
func TestComputeAgentQuotaFetchErrorStaysHonest(t *testing.T) {
	auth := AgyAuthStatus{Authenticated: true, Account: "dev@example.com"}
	res := computeAgentQuota("agent-agy", auth, func() (map[string]interface{}, error) {
		return nil, errQuotaFetchFailed{}
	}, nil)
	if known, _ := res["quotaKnown"].(bool); known {
		t.Fatalf("expected quotaKnown=false after a failed fetch, got %v", res)
	}
	if acc, _ := res["account"].(string); acc != "dev@example.com" {
		t.Fatalf("expected known account retained, got %q", acc)
	}
}

type errQuotaFetchFailed struct{}

func (errQuotaFetchFailed) Error() string { return "quota fetch failed" }

// Non-Agy agents keep the original routing: live fetch result, then the ACP
// usage-summary fallback.
func TestComputeAgentQuotaNonAgyRouting(t *testing.T) {
	summaryCalled := false
	summary := func() map[string]interface{} {
		summaryCalled = true
		return map[string]interface{}{"totals": "from-acp"}
	}
	res := computeAgentQuota("agent-internal", AgyAuthStatus{}, func() (map[string]interface{}, error) {
		return map[string]interface{}{"agentName": "Internal Harness"}, nil
	}, summary)
	if res["agentName"] != "Internal Harness" || summaryCalled {
		t.Fatalf("expected fetch passthrough without summary, got %v (summaryCalled=%v)", res, summaryCalled)
	}

	res = computeAgentQuota("agent-internal", AgyAuthStatus{}, func() (map[string]interface{}, error) {
		return nil, errQuotaFetchFailed{}
	}, summary)
	if !summaryCalled || res["totals"] != "from-acp" {
		t.Fatalf("expected summary fallback, got %v", res)
	}
}

func writeAgyFixture(t *testing.T, home string, rel, payload string) {
	t.Helper()
	path := filepath.Join(home, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(payload), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
}

func mustJWT(t *testing.T, email string) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	claims := base64.RawURLEncoding.EncodeToString([]byte(`{"email":"` + email + `"}`))
	return header + "." + claims + ".sig"
}

// The resolver reads the same stores the agy CLI authenticates from and
// reports the account the active session is running under.
func TestAgyAuthFromHomeOAuthFile(t *testing.T) {
	home := t.TempDir()
	writeAgyFixture(t, home, filepath.Join(".gemini", "oauth_creds.json"),
		`{"access_token":"at","refresh_token":"rt","id_token":"`+mustJWT(t, "file@example.com")+`"}`)

	status := agyAuthFromHome(home, func() (string, bool) { return "", false })
	if !status.Authenticated {
		t.Fatalf("expected authenticated with file credentials")
	}
	if status.Account != "file@example.com" {
		t.Fatalf("expected account from ID token, got %q", status.Account)
	}
	if status.Source != "file" {
		t.Fatalf("expected source 'file', got %q", status.Source)
	}
}

func TestAgyAuthFromHomeKeychain(t *testing.T) {
	home := t.TempDir()
	inner := `{"token":{"access_token":"kc-at","refresh_token":"kc-rt","expiry":"2030-01-01T00:00:00Z"},"id_token":"` + mustJWT(t, "keychain@example.com") + `"}`
	encoded := "go-keyring-base64:" + base64.StdEncoding.EncodeToString([]byte(inner))

	status := agyAuthFromHome(home, func() (string, bool) { return encoded, true })
	if !status.Authenticated {
		t.Fatalf("expected authenticated from keychain envelope")
	}
	if status.Account != "keychain@example.com" || status.Source != "keychain" {
		t.Fatalf("unexpected status: %+v", status)
	}
}

// Expired access token with no refresh token cannot be used
// non-interactively: the fetch must treat it as unauthenticated so it never
// forces the login page.
func TestAgyAuthExpiredTokenWithoutRefreshIsUnauthenticated(t *testing.T) {
	home := t.TempDir()
	expired := time.Now().Add(-time.Hour).Format(time.RFC3339)
	writeAgyFixture(t, home, filepath.Join(".gemini", "oauth_creds.json"),
		`{"access_token":"at","id_token":"`+mustJWT(t, "x@example.com")+`","expiry":"`+expired+`"}`)

	status := agyAuthFromHome(home, func() (string, bool) { return "", false })
	if status.Authenticated {
		t.Fatalf("expired token without refresh token must not count as authenticated")
	}
}

// No credentials anywhere: unauthenticated, no invented account.
func TestAgyAuthFromHomeEmpty(t *testing.T) {
	home := t.TempDir()
	status := agyAuthFromHome(home, func() (string, bool) { return "", false })
	if status.Authenticated || status.Account != "" {
		t.Fatalf("expected empty unauthenticated status, got %+v", status)
	}
}

// Active-account file fallback when the ID token carries no email.
func TestAgyAccountEmailFallbackFile(t *testing.T) {
	home := t.TempDir()
	writeAgyFixture(t, home, filepath.Join(".gemini", "google_accounts.json"), `{"active":"fallback@example.com"}`)
	if got := agyAccountEmail(home, ""); got != "fallback@example.com" {
		t.Fatalf("expected fallback account, got %q", got)
	}
	if got := agyAccountEmail(home, mustJWT(t, "claims@example.com")); got != "claims@example.com" {
		t.Fatalf("expected ID token email to win, got %q", got)
	}
}

// unavailableAgyQuota never carries window data (the modal's honesty flag).
func TestUnavailableAgyQuotaShape(t *testing.T) {
	res := unavailableAgyQuota()
	if _, has := res["weekly"]; has {
		t.Fatalf("unavailable payload must not contain weekly data")
	}
	if _, has := res["fiveHour"]; has {
		t.Fatalf("unavailable payload must not contain fiveHour data")
	}
	if !strings.Contains(res["agentName"].(string), "Antigravity") {
		t.Fatalf("expected Antigravity agent name, got %v", res["agentName"])
	}
}
